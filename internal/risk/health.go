package risk

import (
	"fmt"
	"sort"
	"time"

	"arb/internal/config"
	"arb/internal/database"
	"arb/pkg/exchange"
	"arb/internal/models"
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
	PnL         float64 // sum of unrealized PnL across all positions on this exchange
	Balance     *exchange.Balance
	Positions   []string // position IDs with legs on this exchange
}

// HealthAction represents a protective action to be taken by the engine.
type HealthAction struct {
	Type      string                      // "transfer", "reduce", "close"
	Exchange  string                      // affected exchange
	Positions []*models.ArbitragePosition // positions to act on
	Fraction  float64                     // for reduce (e.g. 0.5)
	DonorExch string                      // for transfer: source exchange
	Amount    float64                     // for transfer: USDT amount
}

// HealthMonitor checks per-exchange margin health and emits protective actions.
type HealthMonitor struct {
	exchanges map[string]exchange.Exchange
	db        *database.Client
	cfg       *config.Config
	log       *utils.Logger
	states    map[string]*ExchangeHealth
	actionCh  chan HealthAction
	stopCh    chan struct{}
}

// NewHealthMonitor creates a new HealthMonitor.
func NewHealthMonitor(exchanges map[string]exchange.Exchange, db *database.Client, cfg *config.Config) *HealthMonitor {
	return &HealthMonitor{
		exchanges: exchanges,
		db:        db,
		cfg:       cfg,
		log:       utils.NewLogger("health-monitor"),
		states:    make(map[string]*ExchangeHealth),
		actionCh:  make(chan HealthAction, 20),
		stopCh:    make(chan struct{}),
	}
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

	for {
		select {
		case <-ticker.C:
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
	level := h.computeLevel(bal, totalPnL, len(exchPositions))

	prevState := h.states[name]
	state := &ExchangeHealth{
		Exchange:    name,
		Level:       level,
		MarginRatio: bal.MarginRatio,
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
}

func (h *HealthMonitor) computeLevel(bal *exchange.Balance, pnl float64, posCount int) HealthLevel {
	if posCount == 0 {
		return L0None
	}

	marginRatio := bal.MarginRatio

	// Use hybrid fallback if margin ratio unavailable
	if marginRatio <= 0 && bal.Total > 0 {
		marginRatio = 1.0 - (bal.Available / bal.Total)
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

	// Sort positions by PnL ascending (worst first) using the leg on this exchange
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
		Fraction:  h.cfg.L4ReduceFraction,
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

// FormatStates returns a summary string of all exchange health states.
func (h *HealthMonitor) FormatStates() string {
	var s string
	for name, state := range h.states {
		s += fmt.Sprintf("%s=%s(mr=%.3f) ", name, state.Level, state.MarginRatio)
	}
	return s
}
