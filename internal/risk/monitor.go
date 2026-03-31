package risk

import (
	"fmt"
	"math"
	"time"

	"arb/internal/config"
	"arb/internal/database"
	"arb/pkg/exchange"
	"arb/internal/models"
	"arb/pkg/utils"
)

// Alert severity constants.
const (
	SeverityInfo     = "info"
	SeverityWarning  = "warning"
	SeverityCritical = "critical"
)

// Alert represents a risk monitoring alert for an active position.
type Alert struct {
	Type       string `json:"type"`
	PositionID string `json:"position_id"`
	Message    string `json:"message"`
	Severity   string `json:"severity"`
}

// Monitor continuously checks active positions for risk conditions.
type Monitor struct {
	exchanges map[string]exchange.Exchange
	db        *database.Client
	cfg       *config.Config
	log       *utils.Logger
	alertChan chan Alert
	stopChan  chan struct{}
}

// NewMonitor creates a new position Monitor.
func NewMonitor(exchanges map[string]exchange.Exchange, db *database.Client, cfg *config.Config) *Monitor {
	return &Monitor{
		exchanges: exchanges,
		db:        db,
		cfg:       cfg,
		log:       utils.NewLogger("risk-monitor"),
		alertChan: make(chan Alert, 100),
		stopChan:  make(chan struct{}),
	}
}

// Start begins the background monitoring loop, checking every 30 seconds.
func (m *Monitor) Start() {
	go m.run()
}

// Stop signals the monitoring loop to exit.
func (m *Monitor) Stop() {
	close(m.stopChan)
}

// AlertChan returns a read-only channel for consuming alerts.
func (m *Monitor) AlertChan() <-chan Alert {
	return m.alertChan
}

func (m *Monitor) run() {
	interval := time.Duration(m.cfg.RiskMonitorIntervalSec) * time.Second
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Run an initial check immediately.
	m.checkAll()

	for {
		select {
		case <-ticker.C:
			m.checkAll()
		case <-m.stopChan:
			m.log.Info("monitor stopped")
			return
		}
	}
}

func (m *Monitor) checkAll() {
	positions, err := m.db.GetActivePositions()
	if err != nil {
		m.log.Error("failed to get active positions: %v", err)
		return
	}

	// Prefetch positions from all exchanges to reduce API calls.
	// One GetAllPositions() call per exchange instead of one GetPosition() per (exchange,symbol) per check.
	allPositions := map[string][]exchange.Position{} // exchange name -> all positions
	allPosErrors := map[string]error{}               // exchange name -> error (if any)
	fetchedExchanges := map[string]bool{}

	for _, pos := range positions {
		if pos.Status != models.StatusActive {
			continue
		}
		for _, exchName := range []string{pos.LongExchange, pos.ShortExchange} {
			if fetchedExchanges[exchName] {
				continue
			}
			fetchedExchanges[exchName] = true
			exch, ok := m.exchanges[exchName]
			if !ok {
				continue
			}
			ps, err := exch.GetAllPositions()
			if err != nil {
				allPosErrors[exchName] = err
			} else {
				allPositions[exchName] = ps
			}
		}
	}

	for _, pos := range positions {
		if pos.Status != models.StatusActive {
			continue
		}
		m.checkPosition(pos, allPositions, allPosErrors)
	}
}

func (m *Monitor) checkPosition(pos *models.ArbitragePosition, allPositions map[string][]exchange.Position, allPosErrors map[string]error) {
	m.checkFundingRate(pos)
	m.checkPositionBalance(pos, allPositions, allPosErrors)
	m.checkUnrealizedPnL(pos, allPositions, allPosErrors)
	m.checkLiquidationDistance(pos, allPositions, allPosErrors)
}

// getCachedPositions returns positions for a symbol from the prefetched cache.
// Falls back to GetPosition() if the exchange was not prefetched.
func getCachedPositions(
	exchName string, symbol string,
	exch exchange.Exchange,
	allPositions map[string][]exchange.Position,
	allPosErrors map[string]error,
) ([]exchange.Position, error) {
	if err, hasErr := allPosErrors[exchName]; hasErr && err != nil {
		return nil, err
	}
	if all, ok := allPositions[exchName]; ok {
		var result []exchange.Position
		for _, p := range all {
			if p.Symbol == symbol {
				result = append(result, p)
			}
		}
		return result, nil
	}
	// Fallback: exchange was not prefetched
	return exch.GetPosition(symbol)
}

// checkFundingRate verifies the funding rate spread has not reversed or narrowed significantly.
func (m *Monitor) checkFundingRate(pos *models.ArbitragePosition) {
	longExch, ok := m.exchanges[pos.LongExchange]
	if !ok {
		return
	}
	shortExch, ok := m.exchanges[pos.ShortExchange]
	if !ok {
		return
	}

	longRate, err := longExch.GetFundingRate(pos.Symbol)
	if err != nil {
		m.log.Error("funding rate check failed for %s on %s: %v", pos.Symbol, pos.LongExchange, err)
		return
	}
	shortRate, err := shortExch.GetFundingRate(pos.Symbol)
	if err != nil {
		m.log.Error("funding rate check failed for %s on %s: %v", pos.Symbol, pos.ShortExchange, err)
		return
	}

	// Normalize both rates to bps/hour to match EntrySpread units.
	// Exchanges have different funding intervals (e.g. Bybit 1h, Bitget 8h).
	longIntervalH := longRate.Interval.Hours()
	if longIntervalH <= 0 {
		longIntervalH = 8
	}
	shortIntervalH := shortRate.Interval.Hours()
	if shortIntervalH <= 0 {
		shortIntervalH = 8
	}
	longBpsH := longRate.Rate * 10000 / longIntervalH
	shortBpsH := shortRate.Rate * 10000 / shortIntervalH
	currentSpreadBpsH := shortBpsH - longBpsH

	// Always persist current spread for dashboard visibility.
	_ = m.db.UpdatePositionFields(pos.ID, func(fresh *models.ArbitragePosition) bool {
		if fresh.Status != models.StatusActive {
			return false
		}
		fresh.CurrentSpread = currentSpreadBpsH
		return true
	})

	// Spread reversed — was positive at entry, now negative
	if pos.EntrySpread > 0 && currentSpreadBpsH < 0 {
		m.sendAlert(Alert{
			Type:       "funding_rate",
			PositionID: pos.ID,
			Message:    fmt.Sprintf("Spread reversed: entry=%.4f bps/h current=%.4f bps/h", pos.EntrySpread, currentSpreadBpsH),
			Severity:   SeverityCritical,
		})
		return
	}

	// Spread narrowed more than 50%
	if pos.EntrySpread > 0 && currentSpreadBpsH < pos.EntrySpread*0.5 {
		m.sendAlert(Alert{
			Type:       "funding_rate",
			PositionID: pos.ID,
			Message:    fmt.Sprintf("Spread narrowing: entry=%.4f bps/h current=%.4f bps/h (>50%% reduction)", pos.EntrySpread, currentSpreadBpsH),
			Severity:   SeverityWarning,
		})
	}
}

// checkPositionBalance verifies long and short leg sizes are balanced.
func (m *Monitor) checkPositionBalance(pos *models.ArbitragePosition, allPositions map[string][]exchange.Position, allPosErrors map[string]error) {
	longExch, ok := m.exchanges[pos.LongExchange]
	if !ok {
		return
	}
	shortExch, ok := m.exchanges[pos.ShortExchange]
	if !ok {
		return
	}

	longPositions, err := getCachedPositions(pos.LongExchange, pos.Symbol, longExch, allPositions, allPosErrors)
	if err != nil {
		m.log.Error("position check failed for %s on %s: %v", pos.Symbol, pos.LongExchange, err)
		return
	}
	shortPositions, err := getCachedPositions(pos.ShortExchange, pos.Symbol, shortExch, allPositions, allPosErrors)
	if err != nil {
		m.log.Error("position check failed for %s on %s: %v", pos.Symbol, pos.ShortExchange, err)
		return
	}

	var longSize, shortSize float64
	for _, p := range longPositions {
		if p.HoldSide == "long" {
			s, _ := utils.ParseFloat(p.Total)
			longSize += s
		}
	}
	for _, p := range shortPositions {
		if p.HoldSide == "short" {
			s, _ := utils.ParseFloat(p.Total)
			shortSize += s
		}
	}

	if longSize <= 0 || shortSize <= 0 {
		return
	}

	imbalance := math.Abs(longSize-shortSize) / math.Max(longSize, shortSize)
	if imbalance > 0.05 {
		m.sendAlert(Alert{
			Type:       "position_balance",
			PositionID: pos.ID,
			Message:    fmt.Sprintf("Position imbalance detected: long=%.6f short=%.6f (%.1f%%)", longSize, shortSize, imbalance*100),
			Severity:   SeverityWarning,
		})
	}
}

// checkUnrealizedPnL monitors combined unrealized PnL against position value.
func (m *Monitor) checkUnrealizedPnL(pos *models.ArbitragePosition, allPositions map[string][]exchange.Position, allPosErrors map[string]error) {
	longExch, ok := m.exchanges[pos.LongExchange]
	if !ok {
		return
	}
	shortExch, ok := m.exchanges[pos.ShortExchange]
	if !ok {
		return
	}

	longPositions, err := getCachedPositions(pos.LongExchange, pos.Symbol, longExch, allPositions, allPosErrors)
	if err != nil {
		return
	}
	shortPositions, err := getCachedPositions(pos.ShortExchange, pos.Symbol, shortExch, allPositions, allPosErrors)
	if err != nil {
		return
	}

	var unrealizedPnL float64
	for _, p := range longPositions {
		if p.HoldSide == "long" {
			pnl, _ := utils.ParseFloat(p.UnrealizedPL)
			unrealizedPnL += pnl
		}
	}
	for _, p := range shortPositions {
		if p.HoldSide == "short" {
			pnl, _ := utils.ParseFloat(p.UnrealizedPL)
			unrealizedPnL += pnl
		}
	}

	// Calculate position value from entry prices and sizes
	positionValue := (pos.LongSize * pos.LongEntry) + (pos.ShortSize * pos.ShortEntry)
	if positionValue <= 0 {
		return
	}

	lossPct := -unrealizedPnL / positionValue
	if lossPct > 0.05 {
		m.sendAlert(Alert{
			Type:       "unrealized_pnl",
			PositionID: pos.ID,
			Message:    fmt.Sprintf("Unrealized loss %.2f%% of position value (PnL=%.2f, Value=%.2f)", lossPct*100, unrealizedPnL, positionValue),
			Severity:   SeverityCritical,
		})
	} else if lossPct > 0.02 {
		m.sendAlert(Alert{
			Type:       "unrealized_pnl",
			PositionID: pos.ID,
			Message:    fmt.Sprintf("Unrealized loss %.2f%% of position value (PnL=%.2f, Value=%.2f)", lossPct*100, unrealizedPnL, positionValue),
			Severity:   SeverityWarning,
		})
	}
}

// checkLiquidationDistance verifies that each leg has sufficient distance to
// its liquidation price.  When the exchange provides a liquidation price we use
// it directly; otherwise we estimate from leverage.
//
// Alert thresholds:
//   - Warning  if mark price is within 15% of liquidation distance
//   - Critical if mark price is within 10% of liquidation distance
func (m *Monitor) checkLiquidationDistance(pos *models.ArbitragePosition, allPositions map[string][]exchange.Position, allPosErrors map[string]error) {
	type legInfo struct {
		exchange string
		side     string
	}
	legs := []legInfo{
		{exchange: pos.LongExchange, side: "long"},
		{exchange: pos.ShortExchange, side: "short"},
	}

	for _, leg := range legs {
		exch, ok := m.exchanges[leg.exchange]
		if !ok {
			continue
		}
		positions, err := getCachedPositions(leg.exchange, pos.Symbol, exch, allPositions, allPosErrors)
		if err != nil {
			continue
		}

		for _, p := range positions {
			if p.HoldSide != leg.side {
				continue
			}

			markPrice, _ := utils.ParseFloat(p.MarkPrice)
			liqPrice, _ := utils.ParseFloat(p.LiquidationPrice)
			entryPrice, _ := utils.ParseFloat(p.AverageOpenPrice)
			leverage, _ := utils.ParseFloat(p.Leverage)

			// If mark price is unknown, skip — we cannot assess distance.
			if markPrice <= 0 {
				continue
			}

			// Estimate liquidation price from leverage if the exchange didn't
			// provide one (liqPrice == 0).
			if liqPrice <= 0 && entryPrice > 0 && leverage > 0 {
				// Rough isolated-margin estimate (cross margin is more
				// forgiving, so this is conservative):
				//   long:  liq = entry * (1 - 1/leverage)
				//   short: liq = entry * (1 + 1/leverage)
				if leg.side == "long" {
					liqPrice = entryPrice * (1.0 - 1.0/leverage)
				} else {
					liqPrice = entryPrice * (1.0 + 1.0/leverage)
				}
			}

			if liqPrice <= 0 {
				continue
			}

			// Distance as fraction of mark price.
			distance := math.Abs(markPrice-liqPrice) / markPrice

			if distance < 0.10 {
				m.sendAlert(Alert{
					Type:       "liquidation_distance",
					PositionID: pos.ID,
					Message:    fmt.Sprintf("CRITICAL liquidation proximity on %s %s leg: mark=%.4f liq=%.4f distance=%.1f%%", leg.exchange, leg.side, markPrice, liqPrice, distance*100),
					Severity:   SeverityCritical,
				})
			} else if distance < 0.15 {
				m.sendAlert(Alert{
					Type:       "liquidation_distance",
					PositionID: pos.ID,
					Message:    fmt.Sprintf("Liquidation approaching on %s %s leg: mark=%.4f liq=%.4f distance=%.1f%%", leg.exchange, leg.side, markPrice, liqPrice, distance*100),
					Severity:   SeverityWarning,
				})
			}
		}
	}
}


func (m *Monitor) sendAlert(alert Alert) {
	m.log.Warn("[%s] %s: %s (severity=%s)", alert.Type, alert.PositionID, alert.Message, alert.Severity)
	select {
	case m.alertChan <- alert:
	default:
		m.log.Error("alert channel full, dropping alert for %s", alert.PositionID)
	}
}
