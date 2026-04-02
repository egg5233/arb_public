package risk

import (
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"arb/internal/config"
	"arb/internal/database"
	"arb/internal/models"
	"arb/pkg/exchange"
	"arb/pkg/utils"
)

// HealthLevel represents the margin health tier for an exchange.
type HealthLevel int

const (
	L0None     HealthLevel = iota // Has balance, 0 positions
	L1Safe                        // Has positions, sum PnL >= 0
	L2Low                         // Has positions, sum PnL < 0, marginRatio < L3
	L3Medium                      // PnL < 0, marginRatio >= L3 threshold
	L4High                        // PnL < 0, marginRatio >= L4 threshold
	L5Critical                    // marginRatio >= L5 threshold
)

// HealthLevelName returns a human-readable name for the health level.
func (l HealthLevel) String() string {
	switch l {
	case L0None:
		return "L0-None"
	case L1Safe:
		return "L1-Safe"
	case L2Low:
		return "L2-Low"
	case L3Medium:
		return "L3-Medium"
	case L4High:
		return "L4-High"
	case L5Critical:
		return "L5-Critical"
	default:
		return "Unknown"
	}
}

// ExchangeHealth holds the current health state for a single exchange.
type ExchangeHealth struct {
	Exchange    string
	Level       HealthLevel
	MarginRatio float64
	TrendState  LiqTrendState
	TrendSlope  float64
	TrendRatio  float64
	PnL         float64 // sum of unrealized PnL across all positions on this exchange
	Balance     *exchange.Balance
	Positions   []string // position IDs with legs on this exchange
}

// HealthAction represents a protective action to be taken by the engine.
type HealthAction struct {
	Type           string                      // "transfer", "reduce", "close", "close_orphan", "close_orphan_dust"
	Exchange       string                      // affected exchange
	Positions      []*models.ArbitragePosition // positions to act on
	Fraction       float64                     // for reduce (e.g. 0.5)
	DonorExch      string                      // for transfer: source exchange
	Amount         float64                     // for transfer: USDT amount
	OrphanSide     string                      // for orphan: "long" or "short"
	OrphanExchange string                      // for orphan: which exchange has the orphan leg
}

// HealthMonitor checks per-exchange margin health and emits protective actions.
type HealthMonitor struct {
	exchanges     map[string]exchange.Exchange
	db            *database.Client
	cfg           *config.Config
	log           *utils.Logger
	states        map[string]*ExchangeHealth
	liqTrend      *LiqTrendTracker
	actionCh      chan HealthAction
	stopCh        chan struct{}
	configChanged <-chan struct{}
	orphanMu      sync.Mutex
	orphanCounts  map[string]int // position ID -> consecutive orphan detection count
}

// NewHealthMonitor creates a new HealthMonitor.
func NewHealthMonitor(exchanges map[string]exchange.Exchange, db *database.Client, cfg *config.Config) *HealthMonitor {
	return &HealthMonitor{
		exchanges:    exchanges,
		db:           db,
		cfg:          cfg,
		log:          utils.NewLogger("health-monitor"),
		states:       make(map[string]*ExchangeHealth),
		liqTrend:     NewLiqTrendTracker(cfg),
		actionCh:     make(chan HealthAction, 20),
		stopCh:       make(chan struct{}),
		orphanCounts: make(map[string]int),
	}
}

// SetConfigNotify registers a channel that signals when health monitor config has changed.
func (h *HealthMonitor) SetConfigNotify(ch <-chan struct{}) {
	h.configChanged = ch
}

// Start begins the health monitoring loop.
func (h *HealthMonitor) Start() {
	go h.run()
}

// Stop signals the health monitor to exit.
func (h *HealthMonitor) Stop() {
	close(h.stopCh)
}

// ActionChan returns a read-only channel for consuming health actions.
func (h *HealthMonitor) ActionChan() <-chan HealthAction {
	return h.actionCh
}

// States returns the current health states for all exchanges.
func (h *HealthMonitor) States() map[string]*ExchangeHealth {
	return h.states
}

func (h *HealthMonitor) run() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Initial check
	h.checkAll()
	h.checkOrphanLegs()

	for {
		select {
		case <-ticker.C:
			h.checkAll()
			h.checkOrphanLegs()
		case <-h.configChanged:
			h.liqTrend = NewLiqTrendTracker(h.cfg)
			h.log.Info("health monitor: config updated, liq trend tracker rebuilt")
			h.checkAll()
		case <-h.stopCh:
			h.log.Info("health monitor stopped")
			return
		}
	}
}

func (h *HealthMonitor) checkAll() {
	positions, err := h.db.GetActivePositions()
	if err != nil {
		h.log.Error("failed to get active positions: %v", err)
		return
	}

	for name := range h.exchanges {
		h.checkExchangeHealth(name, positions)
	}
}

func (h *HealthMonitor) checkExchangeHealth(name string, positions []*models.ArbitragePosition) {
	exch, ok := h.exchanges[name]
	if !ok {
		return
	}

	bal, err := exch.GetFuturesBalance()
	if err != nil {
		h.log.Error("failed to get balance for %s: %v", name, err)
		return
	}

	// Find positions with legs on this exchange
	var exchPositions []*models.ArbitragePosition
	var posIDs []string
	var totalPnL float64

	for _, pos := range positions {
		if pos.Status != models.StatusActive {
			continue
		}
		if pos.LongExchange == name || pos.ShortExchange == name {
			exchPositions = append(exchPositions, pos)
			posIDs = append(posIDs, pos.ID)

			// Get unrealized PnL for the leg on this exchange
			pnl := h.getLegPnL(exch, pos, name)
			totalPnL += pnl
		}
	}

	// Determine health level
	marginRatio := h.normalizeMarginRatio(bal)
	level := h.computeLevel(marginRatio, totalPnL, len(exchPositions))
	trend := h.liqTrend.Sample(name, marginRatio, time.Now(), h.cfg.MarginL3Threshold, h.cfg.MarginL4Threshold)

	prevState := h.states[name]
	state := &ExchangeHealth{
		Exchange:    name,
		Level:       level,
		MarginRatio: marginRatio,
		TrendState:  trend.State,
		TrendSlope:  trend.SlopePerMinute,
		TrendRatio:  trend.ProjectedRatio,
		PnL:         totalPnL,
		Balance:     bal,
		Positions:   posIDs,
	}
	h.states[name] = state

	// Log level transitions
	if prevState == nil || prevState.Level != level {
		spotStr := ""
		if spotBal, err := exch.GetSpotBalance(); err == nil && spotBal.Available > 0 {
			spotStr = fmt.Sprintf(" spot=%.2f", spotBal.Available)
		}
		h.log.Info("%s health: %s (marginRatio=%.4f pnl=%.4f positions=%d futBal=%.2f (avail=%.2f|frozen=%.2f)%s)",
			name, level, bal.MarginRatio, totalPnL, len(exchPositions),
			bal.Total, bal.Available, bal.Frozen, spotStr)
	}

	// Emit actions based on level
	switch level {
	case L3Medium:
		h.handleL3(name, state, exchPositions)
	case L4High:
		h.handleL4(name, state, exchPositions)
	case L5Critical:
		h.handleL5(name, state, exchPositions)
	}
	h.handleTrend(name, state, exchPositions, trend)
}

func (h *HealthMonitor) normalizeMarginRatio(bal *exchange.Balance) float64 {
	if bal == nil {
		return 0
	}
	marginRatio := bal.MarginRatio
	if marginRatio <= 0 && bal.Total > 0 && bal.Available > 0 {
		return 1.0 - (bal.Available / bal.Total)
	}
	return marginRatio
}

func (h *HealthMonitor) computeLevel(marginRatio float64, pnl float64, posCount int) HealthLevel {
	if posCount == 0 {
		return L0None
	}

	// L5: Critical (regardless of PnL)
	if marginRatio >= h.cfg.MarginL5Threshold {
		return L5Critical
	}

	// L4: High
	if pnl < 0 && marginRatio >= h.cfg.MarginL4Threshold {
		return L4High
	}

	// L3: Medium
	if pnl < 0 && marginRatio >= h.cfg.MarginL3Threshold {
		return L3Medium
	}

	// L2: Low (has positions, PnL < 0, but margin is ok)
	if pnl < 0 {
		return L2Low
	}

	// L1: Safe
	return L1Safe
}

func (h *HealthMonitor) getLegPnL(exch exchange.Exchange, pos *models.ArbitragePosition, exchName string) float64 {
	var side string
	if pos.LongExchange == exchName {
		side = "long"
	} else {
		side = "short"
	}

	positions, err := exch.GetPosition(pos.Symbol)
	if err != nil {
		return 0
	}

	var pnl float64
	for _, p := range positions {
		if p.HoldSide == side {
			v, _ := utils.ParseFloat(p.UnrealizedPL)
			pnl += v
		}
	}
	return pnl
}

func (h *HealthMonitor) handleTrend(name string, state *ExchangeHealth, positions []*models.ArbitragePosition, trend LiqTrendResult) {
	if trend.SampleCount < h.cfg.LiqMinSamples || trend.State == LiqTrendStable {
		return
	}
	if trend.State == LiqTrendWarning {
		h.log.Warn("liq-trend %s: warning slope=%.4f/min projected=%.4f current=%.4f",
			name, trend.SlopePerMinute, trend.ProjectedRatio, trend.CurrentRatio)
		return
	}
	if !h.cfg.EnableLiqTrendTracking || state.Level >= L4High {
		return
	}
	if len(positions) == 0 {
		return
	}
	h.log.Warn("liq-trend %s: projected margin ratio %.4f crosses L4 with slope %.4f/min, requesting pre-emptive reduce",
		name, trend.ProjectedRatio, trend.SlopePerMinute)
	h.queueReduceAction(name, positions, h.cfg.L4ReduceFraction)
}

// handleL3 attempts to transfer funds from a donor exchange to the at-risk exchange.
func (h *HealthMonitor) handleL3(name string, state *ExchangeHealth, positions []*models.ArbitragePosition) {
	// Find a donor: prefer L0, then L1
	var donorName string
	var donorAvailable float64

	// Sort donors by preference: L0 first, then L1
	type donor struct {
		name      string
		level     HealthLevel
		available float64
	}
	var donors []donor

	for otherName, otherState := range h.states {
		if otherName == name {
			continue
		}
		if otherState.Level == L0None || otherState.Level == L1Safe {
			donors = append(donors, donor{
				name:      otherName,
				level:     otherState.Level,
				available: otherState.Balance.Available,
			})
		}
	}

	if len(donors) == 0 {
		h.log.Warn("L3 %s: no donor exchange available for fund transfer", name)
		return
	}

	// Prefer L0 over L1, then highest available balance
	sort.Slice(donors, func(i, j int) bool {
		if donors[i].level != donors[j].level {
			return donors[i].level < donors[j].level
		}
		return donors[i].available > donors[j].available
	})

	donorName = donors[0].name
	donorAvailable = donors[0].available

	// Calculate how much to transfer: enough to bring margin ratio back below L3
	// Rough estimate: we need to increase equity so that marginRatio < L3 threshold
	// marginRatio = maintMargin / equity => need equity = maintMargin / targetRatio
	// deficit = needed_equity - current_equity
	if state.Balance.Total <= 0 {
		return
	}

	// Target: bring ratio to half of L3 threshold (safe buffer)
	targetRatio := h.cfg.MarginL3Threshold * 0.5
	if targetRatio <= 0 {
		targetRatio = 0.25
	}

	// Current maintMargin = marginRatio * equity
	maintMargin := state.MarginRatio * state.Balance.Total
	neededEquity := maintMargin / targetRatio
	deficit := neededEquity - state.Balance.Total

	if deficit <= 0 {
		return
	}

	// Cap by donor's available balance (leave 10% buffer for donor)
	amount := deficit
	maxFromDonor := donorAvailable * 0.9
	if amount > maxFromDonor {
		amount = maxFromDonor
	}

	if amount < 1.0 { // minimum transfer amount
		return
	}

	h.log.Info("L3 %s: requesting transfer of %.2f USDT from %s", name, amount, donorName)

	select {
	case h.actionCh <- HealthAction{
		Type:      "transfer",
		Exchange:  name,
		Positions: positions,
		DonorExch: donorName,
		Amount:    amount,
	}:
	default:
		h.log.Warn("L3 %s: action channel full, dropping transfer request", name)
	}
}

// handleL4 requests position reduction on the at-risk exchange.
func (h *HealthMonitor) handleL4(name string, state *ExchangeHealth, positions []*models.ArbitragePosition) {
	if len(positions) == 0 {
		return
	}

	h.log.Warn("L4 %s: marginRatio=%.4f, requesting position reduction (%.0f%%)",
		name, state.MarginRatio, h.cfg.L4ReduceFraction*100)
	h.queueReduceAction(name, positions, h.cfg.L4ReduceFraction)
}

func (h *HealthMonitor) queueReduceAction(name string, positions []*models.ArbitragePosition, fraction float64) {
	if len(positions) == 0 {
		return
	}

	// Sort positions by PnL ascending (worst first) using the leg on this exchange.
	type posPnL struct {
		pos *models.ArbitragePosition
		pnl float64
	}
	var sorted []posPnL
	exch := h.exchanges[name]
	for _, pos := range positions {
		pnl := h.getLegPnL(exch, pos, name)
		sorted = append(sorted, posPnL{pos: pos, pnl: pnl})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].pnl < sorted[j].pnl
	})

	sortedPositions := make([]*models.ArbitragePosition, len(sorted))
	for i, sp := range sorted {
		sortedPositions[i] = sp.pos
	}

	select {
	case h.actionCh <- HealthAction{
		Type:      "reduce",
		Exchange:  name,
		Positions: sortedPositions,
		Fraction:  fraction,
	}:
	default:
		h.log.Warn("L4 %s: action channel full, dropping reduce request", name)
	}
}

// handleL5 requests emergency close of all positions on the at-risk exchange.
func (h *HealthMonitor) handleL5(name string, state *ExchangeHealth, positions []*models.ArbitragePosition) {
	if len(positions) == 0 {
		return
	}

	h.log.Error("L5 %s: marginRatio=%.4f CRITICAL — requesting emergency close of %d positions",
		name, state.MarginRatio, len(positions))

	select {
	case h.actionCh <- HealthAction{
		Type:      "close",
		Exchange:  name,
		Positions: positions,
	}:
	default:
		h.log.Error("L5 %s: action channel full, dropping emergency close!", name)
	}
}

// checkOrphanLegs detects positions where one leg has been fully closed (size=0)
// while the other leg remains open. After 3 consecutive detections, emits an action
// to close the orphaned leg.
func (h *HealthMonitor) checkOrphanLegs() {
	positions, err := h.db.GetActivePositions()
	if err != nil {
		h.log.Error("[orphan-leg] failed to get active positions: %v", err)
		return
	}

	const epsilon = 1e-8
	const requiredConsecutive = 10 // 10 × 30s = 5 minutes of consecutive detection

	// Track which position IDs we see this cycle (to clean up stale counts).
	seen := make(map[string]bool)

	for _, pos := range positions {
		if pos.Status != models.StatusActive {
			continue
		}
		seen[pos.ID] = true

		// Get exchange position for long leg.
		longExch, longOk := h.exchanges[pos.LongExchange]
		if !longOk {
			continue
		}
		longPositions, err := longExch.GetAllPositions()
		if err != nil {
			// API error — skip, don't count as orphan.
			continue
		}

		// Get exchange position for short leg.
		shortExch, shortOk := h.exchanges[pos.ShortExchange]
		if !shortOk {
			continue
		}
		shortPositions, err := shortExch.GetAllPositions()
		if err != nil {
			continue
		}

		// Find the actual size of each leg on the exchange.
		longSize := h.findLegSize(longPositions, pos.Symbol, "long")
		shortSize := h.findLegSize(shortPositions, pos.Symbol, "short")

		longZero := math.Abs(longSize) < epsilon
		shortZero := math.Abs(shortSize) < epsilon

		if (longZero && !shortZero) || (!longZero && shortZero) {
			// One leg is missing — potential orphan.
			h.orphanMu.Lock()
			count := h.orphanCounts[pos.ID]
			if count < 0 {
				// Already dispatched — waiting for engine to handle.
				h.orphanMu.Unlock()
				continue
			}
			count++
			h.orphanCounts[pos.ID] = count
			h.orphanMu.Unlock()

			if count < requiredConsecutive {
				h.log.Info("[orphan-leg] position %s: detection %d/%d", pos.ID, count, requiredConsecutive)
				continue
			}

			// Determine which side survives.
			var orphanSide, orphanExchange string
			var survivingSize, price float64
			if longZero {
				// Long leg is gone, short leg survives.
				orphanSide = "short"
				orphanExchange = pos.ShortExchange
				survivingSize = shortSize
				price = pos.ShortEntry
			} else {
				// Short leg is gone, long leg survives.
				orphanSide = "long"
				orphanExchange = pos.LongExchange
				survivingSize = longSize
				price = pos.LongEntry
			}

			notional := math.Abs(survivingSize) * price

			h.log.Warn("[orphan-leg] position %s: %s leg on %s is orphaned (size=%.6f, notional=%.2f, other leg=0) — dispatching close",
				pos.ID, orphanSide, orphanExchange, survivingSize, notional)

			actionType := "close_orphan"
			if notional < 5.0 {
				actionType = "close_orphan_dust"
			}

			select {
			case h.actionCh <- HealthAction{
				Type:           actionType,
				Exchange:       orphanExchange,
				Positions:      []*models.ArbitragePosition{pos},
				OrphanSide:     orphanSide,
				OrphanExchange: orphanExchange,
			}:
				// Mark as dispatched (negative sentinel) to prevent re-enqueue.
				h.orphanMu.Lock()
				h.orphanCounts[pos.ID] = -1
				h.orphanMu.Unlock()
			default:
				h.log.Warn("[orphan-leg] action channel full, dropping orphan action for %s", pos.ID)
			}
		} else {
			// Both legs present (or both zero) — reset counter.
			h.orphanMu.Lock()
			delete(h.orphanCounts, pos.ID)
			h.orphanMu.Unlock()
		}
	}

	// Clean up counts for positions no longer active.
	h.orphanMu.Lock()
	for id := range h.orphanCounts {
		if !seen[id] {
			delete(h.orphanCounts, id)
		}
	}
	h.orphanMu.Unlock()
}

// ResetOrphanCount clears the orphan detection sentinel for a position,
// allowing it to be re-detected if a close attempt failed and the position
// was reverted to active.
func (h *HealthMonitor) ResetOrphanCount(posID string) {
	h.orphanMu.Lock()
	delete(h.orphanCounts, posID)
	h.orphanMu.Unlock()
}

// findLegSize finds the total size for a given symbol and side from exchange positions.
func (h *HealthMonitor) findLegSize(positions []exchange.Position, symbol, side string) float64 {
	var total float64
	for _, p := range positions {
		if p.Symbol == symbol && p.HoldSide == side {
			v, err := utils.ParseFloat(p.Total)
			if err == nil {
				total += v
			}
		}
	}
	return total
}

// FormatStates returns a summary string of all exchange health states.
func (h *HealthMonitor) FormatStates() string {
	var s string
	for name, state := range h.states {
		s += fmt.Sprintf("%s=%s(mr=%.3f) ", name, state.Level, state.MarginRatio)
	}
	return s
}
