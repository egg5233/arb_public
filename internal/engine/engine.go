package engine

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"arb/internal/api"
	"arb/internal/config"
	"arb/internal/database"
	"arb/internal/discovery"
	"arb/internal/models"
	"arb/internal/notify"
	"arb/internal/risk"
	"arb/pkg/exchange"
	"arb/pkg/utils"
)

var errPartialEntry = errors.New("partial entry: consolidator will reconcile")

// isAlreadySetError detects harmless "already set" responses from exchanges
// when setting leverage or margin mode to the current value.
func isAlreadySetError(err error) bool {
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "no need") || strings.Contains(s, "not modified") ||
		strings.Contains(s, "already") || strings.Contains(s, "same") ||
		strings.Contains(s, "not changed")
}

// isMarginError detects insufficient margin/balance errors across exchanges.
// Case-insensitive to handle varying error message formats.
func isMarginError(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "insufficient") ||
		strings.Contains(msg, "not enough") ||
		strings.Contains(msg, "exceeds the balance") ||
		strings.Contains(msg, "exceeds balance") ||
		strings.Contains(msg, "margin") && strings.Contains(msg, "available")
}

// Engine is the core arbitrage engine. It orchestrates discovery, risk
// approval, trade execution, position management and exit logic.
type Engine struct {
	exchanges     map[string]exchange.Exchange
	contracts     map[string]map[string]exchange.ContractInfo // exchange -> symbol -> info
	discovery     *discovery.Scanner
	risk          *risk.Manager
	monitor       *risk.Monitor
	healthMonitor *risk.HealthMonitor
	allocator     *risk.CapitalAllocator
	db            *database.Client
	api           *api.Server
	cfg           *config.Config
	log           *utils.Logger
	stopCh        chan struct{}

	// Exit goroutine tracking for L4/L5 preemption.
	exitMu      sync.Mutex
	exitCancels map[string]context.CancelFunc // posID → cancel running exit goroutine
	exitActive  map[string]bool               // posID → true while exit goroutine is running
	exitDone    map[string]chan struct{}      // posID → signalled when exit goroutine finishes

	// Pre-settlement timer dedup.
	preSettleMu     sync.Mutex
	preSettleActive map[string]bool // posID → true while timer is pending

	// Entry tracking: prevents consolidator from treating mid-fill positions as orphans.
	entryMu     sync.Mutex
	entryActive map[string]string // "exchange:symbol" → posID while depth fill is running

	// Global capacity lock for manual open to prevent concurrent over-subscription.
	capacityMu sync.Mutex

	// Mutex to serialize PnL reconciliation (reconcilePnL + reconcileRotationPnL).
	pnlReconcileMu sync.Mutex

	// Rejection tracking for dashboard display.
	rejStore *models.RejectionStore

	// Telegram notifier for critical event alerts (SL, emergency close, API errors).
	telegram *notify.TelegramNotifier

	// Rolling window loss limiter -- independent from L0-L5 risk tiers (D-08).
	lossLimiter *risk.LossLimitChecker

	// Analytics snapshot writer for recording position close events.
	snapshotWriter interface {
		RecordPerpClose(pos *models.ArbitragePosition)
	}

	// Per-exchange consecutive API error counter.
	apiErrMu     sync.Mutex
	apiErrCounts map[string]int // exchange name -> consecutive failure count

	// SL fill detection: reverse index from (exchange:orderID) → (posID, leg).
	slIndexMu sync.RWMutex
	slIndex   map[string]slEntry // "exchange:orderID" → posID + leg
	slFillCh  chan slFillEvent   // buffered channel for non-blocking WS callbacks

	// ownOrders tracks order IDs placed by the engine itself.
	// Used by handleSLFill method 2 to avoid false-triggering on our own
	// reduce-only fills (exits, L4 reductions, rotation closes, consolidator trims).
	ownOrders sync.Map // "exchange:orderID" → struct{}{}

	// consolidateRetries tracks PnL query retry counts per position in markPositionClosed.
	// key: posID → consecutive consolidation cycles where GetClosePnL failed.
	consolidateRetries map[string]int

	// allocOverrides stores the allocator's chosen exchange pairs from the most
	// recent rebalance scan, keyed by symbol. The entry scan applies these to
	// patch opportunities so the executed pair matches the rebalanced pair.
	allocOverrideMu sync.Mutex
	allocOverrides  map[string]allocatorChoice // symbol → chosen pair

	// spotCloseCallback dispatches spot-futures health actions to SpotEngine.
	// Set via SetSpotCloseCallback after both engines are initialized.
	spotCloseCallback func(pos *models.SpotFuturesPosition, reason string, isEmergency bool) error
}

// slEntry maps a stop-loss order to its position and leg.
type slEntry struct {
	PosID string
	Leg   string // "long" or "short"
}

// NewEngine creates a new Engine with all required dependencies.
func NewEngine(
	exchanges map[string]exchange.Exchange,
	disc *discovery.Scanner,
	riskMgr *risk.Manager,
	riskMon *risk.Monitor,
	healthMon *risk.HealthMonitor,
	db *database.Client,
	apiSrv *api.Server,
	cfg *config.Config,
	allocator *risk.CapitalAllocator,
) *Engine {
	e := &Engine{
		exchanges:       exchanges,
		discovery:       disc,
		risk:            riskMgr,
		monitor:         riskMon,
		healthMonitor:   healthMon,
		allocator:       allocator,
		db:              db,
		api:             apiSrv,
		cfg:             cfg,
		log:             utils.NewLogger("engine"),
		stopCh:          make(chan struct{}),
		exitCancels:     make(map[string]context.CancelFunc),
		exitActive:      make(map[string]bool),
		exitDone:        make(map[string]chan struct{}),
		preSettleActive: make(map[string]bool),
		entryActive:     make(map[string]string),
		slIndex:         make(map[string]slEntry),
		slFillCh:        make(chan slFillEvent, 64),
		apiErrCounts:       make(map[string]int),
		consolidateRetries: make(map[string]int),
	}
	return e
}

// SetTelegram injects the shared Telegram notifier for perp-perp alerts.
func (e *Engine) SetTelegram(tg *notify.TelegramNotifier) {
	e.telegram = tg
}

// SetLossLimiter injects the loss limit checker for pre-entry gating.
func (e *Engine) SetLossLimiter(ll *risk.LossLimitChecker) {
	e.lossLimiter = ll
}

// SetSnapshotWriter injects the analytics snapshot writer for recording
// position close events. The writer is optional; nil means analytics disabled.
func (e *Engine) SetSnapshotWriter(sw interface{ RecordPerpClose(pos *models.ArbitragePosition) }) {
	e.snapshotWriter = sw
}

// SetSpotCloseCallback registers the SpotEngine's ClosePosition method for
// cross-engine dispatch. When the health monitor detects L4/L5 conditions,
// spot-futures positions are dispatched via this callback instead of the
// perp-perp close path.
func (e *Engine) SetSpotCloseCallback(fn func(pos *models.SpotFuturesPosition, reason string, isEmergency bool) error) {
	e.spotCloseCallback = fn
}

// recordAPIError increments the consecutive error counter for an exchange.
// Triggers Telegram notification at exactly 3 consecutive failures (per D-02).
func (e *Engine) recordAPIError(exchName string, err error) {
	if !e.cfg.EnablePerpTelegram {
		return
	}
	e.apiErrMu.Lock()
	e.apiErrCounts[exchName]++
	count := e.apiErrCounts[exchName]
	e.apiErrMu.Unlock()

	if count == 3 {
		e.telegram.NotifyConsecutiveAPIErrors(exchName, count, err)
	}
}

// recordAPISuccess resets the consecutive error counter for an exchange.
func (e *Engine) recordAPISuccess(exchName string) {
	e.apiErrMu.Lock()
	e.apiErrCounts[exchName] = 0
	e.apiErrMu.Unlock()
}

// ManualClose initiates an exit for the given position from the dashboard.
// It atomically transitions active → exiting before spawning the exit goroutine.
func (e *Engine) ManualClose(posID string) error {
	pos, err := e.db.GetPosition(posID)
	if err != nil {
		return fmt.Errorf("position %s not found", posID)
	}
	if pos.Status != models.StatusActive {
		return fmt.Errorf("position %s is not active (status=%s)", posID, pos.Status)
	}
	// Atomic: only proceed if we win the active → exiting transition.
	err = e.db.UpdatePositionFields(posID, func(fresh *models.ArbitragePosition) bool {
		if fresh.Status != models.StatusActive {
			return false
		}
		fresh.Status = models.StatusExiting
		return true
	})
	if err != nil {
		return fmt.Errorf("position %s already being handled", posID)
	}
	// Re-read to confirm we won the transition.
	pos, _ = e.db.GetPosition(posID)
	if pos.Status != models.StatusExiting {
		return fmt.Errorf("position %s already being handled (status=%s)", posID, pos.Status)
	}
	e.log.Info("manual close requested for %s (%s)", posID, pos.Symbol)
	e.spawnExitGoroutine(pos, "manual close from dashboard")
	return nil
}

// ManualOpen opens a position for a cached opportunity, running risk checks
// synchronously and executing the depth fill asynchronously.
// If force is true, risk approval is skipped (user override from dashboard).
func (e *Engine) ManualOpen(symbol, longExchange, shortExchange string, force bool) error {
	// 1. Find opportunity in scanner cache (thread-safe)
	opps := e.discovery.GetOpportunities()
	var opp *models.Opportunity
	for i := range opps {
		if opps[i].Symbol == symbol && opps[i].LongExchange == longExchange && opps[i].ShortExchange == shortExchange {
			opp = &opps[i]
			break
		}
	}
	if opp == nil {
		return fmt.Errorf("opportunity not found in latest scan")
	}

	// 2. Hold global capacity lock to prevent concurrent requests from
	//    over-subscribing MaxPositions. Released before async execution.
	e.log.Info("ManualOpen %s: acquiring capacity lock...", symbol)
	e.capacityMu.Lock()
	e.log.Info("ManualOpen %s: capacity lock acquired", symbol)

	active, err := e.db.GetActivePositions()
	if err != nil {
		e.capacityMu.Unlock()
		return fmt.Errorf("failed to check active positions")
	}
	for _, pos := range active {
		if pos.Symbol == symbol {
			e.capacityMu.Unlock()
			return fmt.Errorf("position for %s already open", symbol)
		}
	}

	// 3. Check capacity
	slots := e.cfg.MaxPositions - len(active)
	if slots <= 0 {
		e.capacityMu.Unlock()
		return fmt.Errorf("at max capacity (%d/%d)", len(active), e.cfg.MaxPositions)
	}

	// 4. Acquire execution lock
	lockResource := fmt.Sprintf("execute:%s", symbol)
	lock, acquired, err := e.db.AcquireOwnedLock(lockResource, 5*time.Minute)
	if err != nil {
		e.capacityMu.Unlock()
		return fmt.Errorf("failed to acquire lock for %s", symbol)
	}
	if !acquired {
		e.capacityMu.Unlock()
		return fmt.Errorf("execution already in progress for %s", symbol)
	}

	// 5. Risk approval (synchronous, under capacity lock to prevent races)
	approval, err := e.risk.Approve(*opp)
	if err != nil {
		e.capacityMu.Unlock()
		lock.Release()
		return fmt.Errorf("risk check failed: %v", err)
	}
	if !approval.Approved {
		if force {
			e.log.Info("ManualOpen %s: force=true, overriding risk rejection: %s", symbol, approval.Reason)
		} else {
			e.capacityMu.Unlock()
			lock.Release()
			return fmt.Errorf("risk rejected: %s", approval.Reason)
		}
	}

	// 6. Dry run check
	if e.cfg.DryRun {
		e.capacityMu.Unlock()
		lock.Release()
		return fmt.Errorf("dry run mode — trade not executed")
	}

	reservation, err := e.reservePerpCapital(*opp, approval, 0)
	if err != nil {
		e.capacityMu.Unlock()
		lock.Release()
		return fmt.Errorf("capital allocator rejected: %v", err)
	}

	// 7. Reserve a slot by persisting a pending position *after* risk approval
	//    but *before* releasing the capacity mutex, so concurrent requests
	//    see the occupied slot without the reservation self-rejecting.
	pendingPos := e.createPendingPosition(*opp)
	if err := e.db.SavePosition(pendingPos); err != nil {
		e.releasePerpReservation(reservation)
		e.capacityMu.Unlock()
		lock.Release()
		return fmt.Errorf("failed to reserve position slot: %v", err)
	}
	e.api.BroadcastPositionUpdate(pendingPos)

	// Slot reserved — release capacity mutex now.
	e.capacityMu.Unlock()

	// 8. Async execution (depth fill is slow — return 202 to user)
	e.log.Info("manual open: executing trade for %s (L:%s S:%s) size=%.6f price=%.5f",
		symbol, longExchange, shortExchange, approval.Size, approval.Price)
	go func() {
		defer lock.Release()
		err := e.executeTradeV2WithPos(*opp, pendingPos, approval.Size, approval.Price, approval.GapBPS)
		if errors.Is(err, errPartialEntry) {
			if cErr := e.commitPerpCapital(reservation, pendingPos.ID); cErr != nil {
				e.log.Error("manual open: capital commit for partial %s: %v", symbol, cErr)
				if e.allocator != nil && e.allocator.Enabled() {
					_ = e.allocator.Reconcile()
				}
			}
			return
		}
		if err != nil {
			e.releasePerpReservation(reservation)
			e.log.Error("manual open: trade execution failed for %s: %v", symbol, err)
			e.removePendingPosition(pendingPos)
			return
		}
		if cErr := e.commitPerpCapital(reservation, pendingPos.ID); cErr != nil {
			e.log.Error("manual open: capital commit failed for %s: %v — triggering allocator reconcile", symbol, cErr)
			if e.allocator != nil && e.allocator.Enabled() {
				_ = e.allocator.Reconcile()
			}
		}
	}()
	return nil
}

// SetContracts stores loaded contract info for tick-size-aware price formatting.
func (e *Engine) SetContracts(contracts map[string]map[string]exchange.ContractInfo) {
	e.contracts = contracts
}

// SetRejectionStore sets the shared rejection store for recording filtered opportunities.
func (e *Engine) SetRejectionStore(store *models.RejectionStore) {
	e.rejStore = store
}

// formatPrice rounds a price to the exchange's tick size for the given symbol.
func (e *Engine) formatPrice(exchName, symbol string, price float64) string {
	if e.contracts != nil {
		if exContracts, ok := e.contracts[exchName]; ok {
			if ci, ok := exContracts[symbol]; ok && ci.PriceStep > 0 {
				rounded := utils.RoundToStep(price, ci.PriceStep)
				return utils.FormatPrice(rounded, ci.PriceDecimals)
			}
		}
	}
	return utils.FormatPrice(price, 8)
}

// Start launches all engine goroutines: the main loop, scheduler, exit
// manager, funding tracker and alert forwarder.
func (e *Engine) Start() {
	e.log.Info("engine starting")

	// One-time seed: add default blacklist entries only if the seed hasn't run before.
	if seeded, _ := e.db.Get("arb:blacklist:seeded"); seeded == "" {
		for _, sym := range []string{"DRIFTUSDT"} {
			_ = e.db.AddToBlacklist(sym)
		}
		_ = e.db.SetWithTTL("arb:blacklist:seeded", "1", 0)
	}

	// Subscribe BBO for any existing active positions (survives restart).
	if active, err := e.db.GetActivePositions(); err == nil {
		for _, pos := range active {
			if longExch, ok := e.exchanges[pos.LongExchange]; ok {
				longExch.SubscribeSymbol(pos.Symbol)
			}
			if shortExch, ok := e.exchanges[pos.ShortExchange]; ok {
				shortExch.SubscribeSymbol(pos.Symbol)
			}
		}
	}

	// Set up SL fill detection callbacks on all exchanges.
	e.setupSLCallbacks()
	e.rebuildSLIndex()
	go e.consumeSLFills()

	go e.run()
	go e.forwardAlerts()
	go e.trackFunding()
	go e.consumeHealthActions()
	e.StartConsolidator()

	e.log.Info("engine started")
}

// rebalanceFunds analyzes upcoming opportunities and ensures each exchange
// has enough margin. Performs same-exchange spot→futures transfers first,
// then cross-exchange withdrawals for remaining deficits.
func (e *Engine) rebalanceFunds(passedOpps ...[]models.Opportunity) {
	// Clear any stale allocator overrides from the previous cycle.
	// If the prior entry scan had 0 opportunities, applyAllocatorOverrides
	// was never called and the old overrides would leak into the next cycle.
	e.allocOverrideMu.Lock()
	if e.allocOverrides != nil {
		e.log.Info("rebalance: clearing %d stale allocator overrides from previous cycle", len(e.allocOverrides))
		e.allocOverrides = nil
	}
	e.allocOverrideMu.Unlock()

	var opps []models.Opportunity
	if len(passedOpps) > 0 && len(passedOpps[0]) > 0 {
		opps = passedOpps[0]
	} else {
		opps = e.discovery.GetOpportunities()
	}
	if len(opps) == 0 {
		e.log.Info("rebalance: no opportunities, skipping")
		return
	}

	// Determine capital needs per exchange by simulating the full entry
	// approval filter chain (matching risk/manager.go approveInternal)
	// without locks, orders, or side effects.
	active, err := e.db.GetActivePositions()
	if err != nil {
		e.log.Error("rebalance: failed to get active positions: %v", err)
		return
	}
	remainingSlots := e.cfg.MaxPositions - len(active)
	if remainingSlots <= 0 {
		e.log.Info("rebalance: at max capacity (%d/%d), skipping", len(active), e.cfg.MaxPositions)
		return
	}

	// Build occupied symbols set (same logic as executeArbitrage)
	activeSymbols := make(map[string]bool)
	for _, p := range active {
		if p.Status != models.StatusClosed {
			activeSymbols[p.Symbol] = true
		}
	}

	// Pre-filter: remove opps whose symbol already has an active position or is blacklisted.
	var newOpps []models.Opportunity
	for _, opp := range opps {
		if activeSymbols[opp.Symbol] {
			continue
		}
		if blocked, err := e.db.IsBlacklisted(opp.Symbol); err == nil && blocked {
			continue
		}
		newOpps = append(newOpps, opp)
	}
	if len(newOpps) == 0 {
		e.log.Info("rebalance: all %d opps already have active positions, skipping", len(opps))
		return
	}
	if len(newOpps) < len(opps) {
		e.log.Info("rebalance: filtered %d/%d opps (active symbols removed)", len(newOpps), len(opps))
	}
	opps = newOpps

	// Build set of exchanges that have active position legs (used to cap
	// donor transfers so we don't drain margin from exchanges holding positions).
	exchWithPositions := make(map[string]bool)
	for _, p := range active {
		if p.Status == models.StatusClosed {
			continue
		}
		exchWithPositions[p.LongExchange] = true
		exchWithPositions[p.ShortExchange] = true
	}

	// Query all exchange balances (futures + spot) BEFORE calculating needs
	// so we can do budget-aware allocation.
	balances := map[string]rebalanceBalanceInfo{}
	for name, exch := range e.exchanges {
		var bi rebalanceBalanceInfo
		if futBal, err := exch.GetFuturesBalance(); err == nil {
			bi.futures = futBal.Available
			bi.futuresTotal = futBal.Total
			bi.marginRatio = futBal.MarginRatio
			bi.maxTransferOut = futBal.MaxTransferOut
		}
		if spotBal, err := exch.GetSpotBalance(); err == nil {
			bi.spot = spotBal.Available
		}
		bi.hasPositions = exchWithPositions[name]
		balances[name] = bi
		e.log.Info("rebalance: %s futures=%.2f spot=%.2f futuresTotal=%.2f marginRatio=%.4f maxTransferOut=%.2f hasPos=%v", name, bi.futures, bi.spot, bi.futuresTotal, bi.marginRatio, bi.maxTransferOut, bi.hasPositions)
	}

	// ---------------------------------------------------------------------------
	// Try pool allocator first (if enabled).
	// ---------------------------------------------------------------------------
	if e.cfg.EnablePoolAllocator {
		allocSel, allocErr := e.runPoolAllocator(opps, balances, remainingSlots)
		if allocErr != nil {
			e.log.Warn("rebalance: pool allocator failed, falling back to sequential: %v", allocErr)
		} else if allocSel != nil && allocSel.feasible {
			e.log.Info("rebalance: pool allocator selected %d opps (value=%.4f, choices=%s)", len(allocSel.choices), allocSel.totalBaseValue, e.formatAllocatorSummary(allocSel))
			e.executeRebalanceFundingPlan(allocSel.needs, balances, nil)

			// Store allocator choices so the upcoming entry scan can patch
			// opportunities to use the same exchange pairs that funds were rebalanced for.
			e.allocOverrideMu.Lock()
			e.allocOverrides = make(map[string]allocatorChoice, len(allocSel.choices))
			for _, c := range allocSel.choices {
				e.allocOverrides[c.symbol] = c
			}
			e.allocOverrideMu.Unlock()
			e.log.Info("rebalance: stored %d allocator overrides for entry scan", len(allocSel.choices))

			e.log.Info("rebalance: complete")
			return
		} else {
			e.log.Warn("rebalance: pool allocator infeasible, skipping rebalance")
			return
		}
	}

	// ---------------------------------------------------------------------------
	// Sequential allocation fallback (original path).
	// Budget-aware: iterate opps by score and only count needs for positions
	// the exchanges can actually afford.
	// ---------------------------------------------------------------------------
	available := map[string]float64{}
	for name, bal := range balances {
		total := bal.futures + bal.spot
		// Unified accounts: spot and futures share the same pool, don't double-count.
		type unifiedChecker interface{ IsUnified() bool }
		if name == "okx" || name == "bybit" {
			total = bal.futures
		} else if uc, ok := e.exchanges[name].(unifiedChecker); ok && uc.IsUnified() {
			total = bal.futures
		}
		available[name] = total
	}

	needs := map[string]float64{}
	selectedSymbols := make(map[string]bool)     // prevent same-batch duplicates
	reserved := map[string]float64{}             // cumulative margin reserved per exchange
	plannedTransfers := map[string]float64{}     // cross-exchange transfer plan: recipient → amount
	selected := 0

	for _, opp := range opps {
		if selected >= remainingSlots {
			break
		}
		if activeSymbols[opp.Symbol] || selectedSymbols[opp.Symbol] {
			continue
		}

		// Run full approval simulation (same checks as risk.Approve, no side effects)
		approval, err := e.risk.SimulateApproval(opp, reserved)
		if err != nil {
			e.log.Info("rebalance: skip %s — simulate error: %v", opp.Symbol, err)
			continue
		}
		if !approval.Approved {
			// Check if rejection is margin-related and cross-exchange transfer could help.
			isMarginRejection := strings.Contains(approval.Reason, "insufficient margin buffer") ||
				strings.Contains(approval.Reason, "post-trade margin ratio") ||
				strings.Contains(approval.Reason, "insufficient capital")
			if !isMarginRejection {
				e.log.Info("rebalance: skip %s — %s", opp.Symbol, approval.Reason)
				continue
			}

			// Margin-related rejection: check if cross-exchange transfer from a donor could help.
			estMargin := e.effectiveCapitalPerLeg() * e.cfg.MarginSafetyMultiplier
			if estMargin <= 0 {
				if approval.RequiredMargin > 0 {
					estMargin = approval.RequiredMargin
				} else {
					estMargin = 50 * e.cfg.MarginSafetyMultiplier
				}
			}
			canRescue := true
			for _, legExch := range []string{opp.LongExchange, opp.ShortExchange} {
				bal := balances[legExch]
				effectiveAvail := bal.futures - reserved[legExch]
				if effectiveAvail >= estMargin {
					continue
				}
				deficit := estMargin - effectiveAvail
				foundDonor := false
				for donorName, donorBal := range balances {
					if donorName == legExch {
						continue
					}
					if balances[donorName].marginRatio >= e.cfg.MarginL3Threshold {
						continue
					}
					donorSurplus := donorBal.futures + donorBal.spot - reserved[donorName] - plannedTransfers[donorName+"_out"]
					type unifiedChecker interface{ IsUnified() bool }
					if donorName == "okx" || donorName == "bybit" {
						donorSurplus = donorBal.futures - reserved[donorName] - plannedTransfers[donorName+"_out"]
					} else if uc, ok := e.exchanges[donorName].(unifiedChecker); ok && uc.IsUnified() {
						donorSurplus = donorBal.futures - reserved[donorName] - plannedTransfers[donorName+"_out"]
					}
					// Cap donor surplus by margin health to prevent draining exchanges with active positions.
					if balances[donorName].hasPositions && balances[donorName].marginRatio > 0 && balances[donorName].futuresTotal > 0 && e.cfg.MarginL3Threshold > 0 {
						maint := balances[donorName].marginRatio * balances[donorName].futuresTotal
						healthCap := balances[donorName].futuresTotal - maint/e.cfg.MarginL3Threshold
						if healthCap < donorSurplus {
							donorSurplus = healthCap
						}
					}
					if donorSurplus > 10 {
						plannedTransfers[legExch] += deficit
						plannedTransfers[donorName+"_out"] += deficit
						foundDonor = true
						e.log.Info("rebalance: %s needs %.2f transfer from %s for %s (margin rescue)",
							legExch, deficit, donorName, opp.Symbol)
						break
					}
				}
				if !foundDonor {
					canRescue = false
					break
				}
			}
			if !canRescue {
				e.log.Info("rebalance: skip %s — %s (no donor available)", opp.Symbol, approval.Reason)
				continue
			}

			needs[opp.LongExchange] += estMargin
			needs[opp.ShortExchange] += estMargin
			reserved[opp.LongExchange] += estMargin
			reserved[opp.ShortExchange] += estMargin
			selectedSymbols[opp.Symbol] = true
			selected++
			e.log.Info("rebalance: selected %s via cross-exchange rescue (margin=%.2f per leg)", opp.Symbol, estMargin)
			continue
		}

		// Approval passed. Check if exchanges need cross-exchange transfers.
		requiredMargin := approval.RequiredMargin
		if requiredMargin <= 0 {
			requiredMargin = e.effectiveCapitalPerLeg() * e.cfg.MarginSafetyMultiplier
		}

		needsTransfer := false
		for _, leg := range []struct {
			exchange string
		}{
			{opp.LongExchange},
			{opp.ShortExchange},
		} {
			bal := balances[leg.exchange]
			effectiveAvail := bal.futures - reserved[leg.exchange]
			if effectiveAvail < requiredMargin {
				deficit := requiredMargin - effectiveAvail
				foundDonor := false
				for donorName, donorBal := range balances {
					if donorName == leg.exchange {
						continue
					}
					donorSurplus := donorBal.futures + donorBal.spot - reserved[donorName] - plannedTransfers[donorName+"_out"]
					type unifiedChecker interface{ IsUnified() bool }
					if donorName == "okx" || donorName == "bybit" {
						donorSurplus = donorBal.futures - reserved[donorName] - plannedTransfers[donorName+"_out"]
					} else if uc, ok := e.exchanges[donorName].(unifiedChecker); ok && uc.IsUnified() {
						donorSurplus = donorBal.futures - reserved[donorName] - plannedTransfers[donorName+"_out"]
					}
					if balances[donorName].marginRatio >= e.cfg.MarginL3Threshold {
						continue
					}
					// Cap donor surplus by margin health to prevent draining exchanges with active positions.
					if balances[donorName].hasPositions && balances[donorName].marginRatio > 0 && balances[donorName].futuresTotal > 0 && e.cfg.MarginL3Threshold > 0 {
						maint := balances[donorName].marginRatio * balances[donorName].futuresTotal
						healthCap := balances[donorName].futuresTotal - maint/e.cfg.MarginL3Threshold
						if healthCap < donorSurplus {
							donorSurplus = healthCap
						}
					}
					if donorSurplus > 10 {
						plannedTransfers[leg.exchange] += deficit
						plannedTransfers[donorName+"_out"] += deficit
						foundDonor = true
						e.log.Debug("rebalance: %s needs %.2f transfer from %s for %s",
							leg.exchange, deficit, donorName, opp.Symbol)
						break
					}
				}
				if !foundDonor {
					e.log.Debug("rebalance: skip %s — no donor for %s deficit=%.2f",
						opp.Symbol, leg.exchange, deficit)
					needsTransfer = true
					break
				}
			}
		}
		if needsTransfer {
			e.log.Debug("rebalance: sequential skip %s: needsTransfer=true", opp.Symbol)
			continue
		}

		needs[opp.LongExchange] += requiredMargin
		needs[opp.ShortExchange] += requiredMargin
		reserved[opp.LongExchange] += requiredMargin
		reserved[opp.ShortExchange] += requiredMargin
		selectedSymbols[opp.Symbol] = true
		selected++
		e.log.Info("rebalance: selected %s (margin=%.2f per leg)", opp.Symbol, requiredMargin)
	}

	e.log.Info("rebalance: analyzed %d opportunities, selected %d, needs: %v, plannedTransfers: %v", len(opps), selected, needs, plannedTransfers)

	// Phase 1: Same-exchange spot→futures transfers (instant).
	type seqDeficit struct {
		exchange string
		amount   float64
	}
	var crossDeficits []seqDeficit

	for name, need := range needs {
		bal := balances[name]
		targetFreeRatio := 1 - e.cfg.MarginL4Threshold
		if targetFreeRatio <= 0 {
			targetFreeRatio = 0.20
		}

		if bal.futures >= need {
			if bal.spot > 0 && bal.futuresTotal > 0 {
				projectedAvail := bal.futures - need
				if projectedAvail < 0 {
					projectedAvail = 0
				}
				projectedRatio := 1 - projectedAvail/bal.futuresTotal
				if projectedRatio >= e.cfg.MarginL4Threshold {
					extra := (need - bal.futuresTotal*targetFreeRatio) / targetFreeRatio
					if extra > bal.spot {
						extra = bal.spot
					}
					if extra >= 1.0 {
						amtStr := fmt.Sprintf("%.4f", extra)
						postRatio := 1 - (bal.futures+extra-need)/(bal.futuresTotal+extra)
						e.log.Info("rebalance: %s spot→futures %s USDT (margin ratio relief, projected=%.2f post=%.2f L4=%.2f)",
							name, amtStr, projectedRatio, postRatio, e.cfg.MarginL4Threshold)
						if !e.cfg.DryRun {
							if err := e.exchanges[name].TransferToFutures("USDT", amtStr); err != nil {
								e.log.Error("rebalance: %s spot→futures failed: %v", name, err)
							} else {
								bi := balances[name]
								bi.futures += extra
								bi.spot -= extra
								balances[name] = bi
							}
						}
					}
				}
			}
			continue
		}

		marginDeficit := need - bal.futures
		if marginDeficit < 0 {
			marginDeficit = 0
		}
		actualMargin := need / e.cfg.MarginSafetyMultiplier
		if actualMargin <= 0 {
			actualMargin = need
		}
		targetRatio := e.cfg.MarginL4Threshold - marginEpsilon
		freeTarget := 1.0 - targetRatio
		var ratioDeficit float64
		if freeTarget > 0 && bal.futuresTotal > 0 {
			ratioDeficit = (freeTarget*bal.futuresTotal - bal.futures + actualMargin) / targetRatio
			if ratioDeficit < 0 {
				ratioDeficit = 0
			}
		}
		transferAmt := marginDeficit
		if ratioDeficit > transferAmt {
			transferAmt = ratioDeficit
		}
		e.log.Debug("rebalance: seq deficit %s: need=%.2f futures=%.2f marginDef=%.2f ratioDef=%.2f transferAmt=%.2f", name, need, bal.futures, marginDeficit, ratioDeficit, transferAmt)
		if bal.spot > 0 {
			actualTransfer := transferAmt
			if actualTransfer > bal.spot {
				actualTransfer = bal.spot
			}
			if actualTransfer < 1.0 {
				e.log.Debug("rebalance: %s spot→futures skip (%.4f USDT below minimum)", name, actualTransfer)
				if transferAmt > 10 {
					crossDeficits = append(crossDeficits, seqDeficit{name, transferAmt})
				}
				continue
			}
			postTotal := bal.futuresTotal + actualTransfer
			postRatio := 1 - (bal.futures+actualTransfer-need)/postTotal
			amtStr := fmt.Sprintf("%.4f", actualTransfer)
			e.log.Info("rebalance: %s spot→futures %s USDT (same-exchange, instant, post-ratio=%.2f L4=%.2f)", name, amtStr, postRatio, e.cfg.MarginL4Threshold)
			if !e.cfg.DryRun {
				if err := e.exchanges[name].TransferToFutures("USDT", amtStr); err != nil {
					e.log.Error("rebalance: %s spot→futures failed: %v", name, err)
				} else {
					transferAmt -= actualTransfer
					bi := balances[name]
					bi.futures += actualTransfer
					bi.spot -= actualTransfer
					balances[name] = bi
				}
			}
		}
		if transferAmt > 10 {
			crossDeficits = append(crossDeficits, seqDeficit{name, transferAmt})
		}
	}

	if len(crossDeficits) == 0 {
		e.log.Info("rebalance: all exchanges funded, no cross-exchange transfers needed")
		return
	}

	// Convert seqDeficits to rebalanceDeficit and delegate to the existing
	// cross-exchange executor (same path the pool allocator uses).
	// Pass full 'needs' map so the lower-half can correctly calculate donor surplus.
	precomputed := make([]rebalanceDeficit, len(crossDeficits))
	for i, cd := range crossDeficits {
		precomputed[i] = rebalanceDeficit{exchange: cd.exchange, amount: cd.amount}
	}
	e.log.Info("rebalance: %d exchanges need cross-exchange funding, delegating to allocator executor", len(precomputed))
	e.executeRebalanceFundingPlan(needs, balances, precomputed)

	e.log.Info("rebalance: complete")
}

// applyAllocatorOverrides consumes stored allocator choices and uses them to
// both FILTER and PATCH the opportunity list.  When overrides are present
// (allocator ran this cycle), only symbols the allocator selected are kept;
// non-selected symbols are dropped so they cannot consume an entry slot and
// invalidate the allocator's capital plan.  When overrides are empty (allocator
// didn't run, or TopPairsPerSymbol==1 with no alternatives), all opps pass
// through unchanged for backwards compatibility.
func (e *Engine) applyAllocatorOverrides(opps []models.Opportunity) ([]models.Opportunity, bool) {
	e.allocOverrideMu.Lock()
	overrides := e.allocOverrides
	e.allocOverrides = nil
	e.allocOverrideMu.Unlock()

	if len(overrides) == 0 {
		return nil, false // no allocator guidance
	}

	// Filter to only allocator-selected symbols, and patch pairs.
	// IMPORTANT: When the override wants a different exchange pair, we validate
	// that the pair still exists in the CURRENT scan's alternatives and use the
	// fresh data from the current scan — never stale data from the rebalance scan.
	var filtered []models.Opportunity
	patched := 0
	skipped := 0
	stale := 0
	for _, opp := range opps {
		choice, ok := overrides[opp.Symbol]
		if !ok {
			skipped++
			continue // allocator didn't select this symbol, drop it
		}
		// Copy to avoid mutating the shared scanner slice.
		o := opp
		if choice.longExchange == o.LongExchange && choice.shortExchange == o.ShortExchange {
			// Primary pair already matches the override — use current fresh data as-is.
		} else {
			// Override wants a different pair — validate it still exists in
			// the current scan's alternatives so we never apply stale data.
			found := false
			rejected := false
			for _, alt := range o.Alternatives {
				if alt.LongExchange == choice.longExchange && alt.ShortExchange == choice.shortExchange {
					// Reject unverified alternatives — they never passed exchange API checks.
					if !alt.Verified {
						e.log.Warn("entry: override %s alt %s/%s not verified, skipping",
							o.Symbol, alt.LongExchange, alt.ShortExchange)
						stale++
						rejected = true
						break
					}
					// Run pair-level filter checks (persistence, volatility, backtest, interval, funding window).
					altOpp := models.Opportunity{
						Symbol:        o.Symbol,
						LongExchange:  alt.LongExchange,
						ShortExchange: alt.ShortExchange,
						Spread:        alt.Spread,
						IntervalHours: alt.IntervalHours,
						NextFunding:   alt.NextFunding,
						OIRank:        o.OIRank, // inherit for isPersistent() low-OI gate
					}
					if reason := e.discovery.CheckPairFilters(altOpp); reason != "" {
						e.log.Warn("entry: override %s alt %s/%s rejected by pair filter: %s",
							o.Symbol, alt.LongExchange, alt.ShortExchange, reason)
						stale++
						rejected = true
						break
					}
					e.log.Info("allocator override %s: %s/%s → %s/%s (spread %.4f→%.4f bps/h)",
						o.Symbol, o.LongExchange, o.ShortExchange,
						choice.longExchange, choice.shortExchange,
						o.Spread, alt.Spread)
					o.LongExchange = alt.LongExchange
					o.ShortExchange = alt.ShortExchange
					o.LongRate = alt.LongRate
					o.ShortRate = alt.ShortRate
					o.Spread = alt.Spread
					// IntervalHours: use current alt if available, otherwise keep opp-level value.
					if alt.IntervalHours > 0 {
						o.IntervalHours = alt.IntervalHours
					}
					// Patch NextFunding from verified alt.
					if !alt.NextFunding.IsZero() {
						o.NextFunding = alt.NextFunding
					}
					found = true
					patched++
					break
				}
			}
			if rejected {
				continue // already counted in stale
			}
			if !found {
				e.log.Warn("entry: allocator override for %s pair %s/%s no longer in current scan, skipping",
					o.Symbol, choice.longExchange, choice.shortExchange)
				stale++
				continue // skip this opp entirely — pair is stale
			}
		}
		filtered = append(filtered, o)
	}

	e.log.Info("allocator filter: %d opps kept, %d dropped, %d patched, %d stale-skipped (from %d total)",
		len(filtered), skipped, patched, stale, len(opps))

	if len(filtered) == 0 {
		e.log.Warn("entry: all allocator overrides stale, no opps passed filter")
		return nil, true // overrides existed but all stale
	}
	return filtered, true
}


// recordTransfer saves a transfer record to the database for dashboard display.
func (e *Engine) recordTransfer(from, to, coin, chain, amount, fee, txID, status, reason string) {
	now := time.Now().UTC()
	id := fmt.Sprintf("xfer-%d", now.UnixMilli())
	label := reason
	if label != "" {
		label = " [" + label + "]"
	}
	record := &database.TransferRecord{
		ID:        id,
		From:      from + label,
		To:        to,
		Coin:      coin,
		Chain:     chain,
		Amount:    amount,
		Fee:       fee,
		TxID:      txID,
		Status:    status,
		CreatedAt: now.Format(time.RFC3339),
	}
	if err := e.db.SaveTransfer(record); err != nil {
		e.log.Error("recordTransfer: failed to save: %v", err)
	}
}

// Stop signals all goroutines to exit.
func (e *Engine) Stop() {
	e.log.Info("engine stopping")
	close(e.stopCh)
	e.log.Info("engine stopped")
}

// run is the main event loop. It listens for new opportunity batches from
// discovery and dispatches actions based on scan type:
//   - RebalanceScan (:10 default, production :20) → rebalance funds across exchanges
//   - ExitScan      (:30) → check exit conditions
//   - RotateScan    (:35) → check leg rotations
//   - EntryScan     (:40) → execute new arb positions
func (e *Engine) run() {
	oppCh := e.discovery.OpportunityChan()

	for {
		select {
		case result := <-oppCh:
			e.log.Info("run loop: received %d opportunities (type=%s), dispatching...",
				len(result.Opps), result.Type)

			// Forward to dashboard only from normal scans — filtered scan types
			// (rebalance, entry, exit, rotate) apply heavy filters that can produce
			// 0 results and would wipe the dashboard opportunity list.
			if result.Type == discovery.NormalScan {
				e.api.SetOpportunities(result.Opps)
				e.api.BroadcastOpportunities(result.Opps)
			}

			// Check for delisted coins in active positions.
			if e.cfg.DelistFilterEnabled {
				e.checkDelistPositions()
			}

			switch result.Type {
			case discovery.RebalanceScan:
				e.rebalanceFunds()
				e.log.Info("run loop: rebalanceScan handler done")
			case discovery.ExitScan:
				e.checkIntervalChanges()
				e.checkExitsV2()
				e.log.Info("run loop: exitScan handler done")
			case discovery.RotateScan:
				e.log.Info("rotateScan: starting checkRotations")
				e.checkRotations()
				e.log.Info("rotateScan: checkRotations done")
				if e.cfg.EnablePoolAllocator {
					e.log.Info("rotateScan: starting rebalanceFunds")
					e.rebalanceFunds()
					e.log.Info("rotateScan: rebalanceFunds done")
				}
				e.log.Info("run loop: rotateScan handler done")
			case discovery.EntryScan:
				// Update performance-weighted allocation before entry decisions (CA-03/CA-04).
				e.updateAllocation()

				// Determine opportunity presence for dynamic shifting.
				perpHasOpps := len(result.Opps) > 0
				spotHasOpps := e.cfg.SpotFuturesEnabled // conservative default: assume spot has opps if engine is enabled
				if e.cfg.SpotFuturesEnabled {
					scanInterval := time.Duration(e.cfg.SpotFuturesScanIntervalMin) * time.Minute
					if scanInterval < time.Minute {
						scanInterval = 10 * time.Minute
					}
					maxAge := 2 * scanInterval
					count, err := e.db.GetSpotEntryableOppsCount(maxAge)
					if err == nil && count >= 0 {
						// Fresh cache: count == 0 means confirmed no opps → free cap for perp.
						// count > 0 means spot has opps → keep cap reserved.
						spotHasOpps = count > 0
					}
					// else: cache missing/stale/error → keep conservative default (spotHasOpps=true)
				}
				perpCap := e.dynamicStrategyPct(risk.StrategyPerpPerp, perpHasOpps, spotHasOpps)
				if perpCap > 0 {
					e.log.Info("dynamic strategy cap: perp=%.1f%% (perpOpps=%v, spotOpps=%v)", perpCap*100, perpHasOpps, spotHasOpps)
				}

				if len(result.Opps) > 0 {
					e.log.Info("entry scan complete, triggering trade execution")

					var entryOpps []models.Opportunity
					tier := "none"

					// Tier 2: Fall back to :35 allocator overrides.
					if len(entryOpps) == 0 {
						patched, hadOverrides := e.applyAllocatorOverrides(result.Opps)
						if len(patched) > 0 {
							tier = "tier-2-rebalance-overrides"
							e.log.Info("entry: %s using %d opps", tier, len(patched))
							entryOpps = patched
						} else if hadOverrides {
							// Allocator ran but all overrides stale — block tier-3 fallback
							tier = "blocked-stale-overrides"
							e.log.Warn("entry: allocator ran but overrides stale, skipping tier-3 to avoid unfunded entries")
						}
					}

					// Tier 3: rank-based — only if allocator did NOT run
					if len(entryOpps) == 0 && tier == "none" {
						tier = "tier-3-rank-based"
						e.log.Info("entry: %s with %d opps", tier, len(result.Opps))
						entryOpps = result.Opps
					}

					e.executeArbitrage(entryOpps, perpCap)
					e.log.Info("run loop: entryScan handler done (via %s)", tier)
				}
			}

		case <-e.stopCh:
			e.log.Info("main loop exiting")
			return
		}
	}
}

// forwardAlerts relays risk monitor alerts to the dashboard WebSocket.
func (e *Engine) forwardAlerts() {
	alertCh := e.monitor.AlertChan()
	for {
		select {
		case alert := <-alertCh:
			e.api.BroadcastAlert(alert)
		case <-e.stopCh:
			return
		}
	}
}

// consumeHealthActions processes protective actions from the health monitor.
func (e *Engine) consumeHealthActions() {
	actionCh := e.healthMonitor.ActionChan()
	for {
		select {
		case action := <-actionCh:
			switch action.Type {
			case "transfer":
				e.handleTransfer(action)
			case "reduce":
				e.handleReduce(action)
				// Dispatch spot-futures position reduction to SpotEngine (per D-10).
				e.dispatchSpotHealthAction(action.SpotPositions, "health_reduce", false)
			case "close":
				e.handleEmergencyClose(action)
				// Dispatch spot-futures emergency close to SpotEngine (per D-10).
				e.dispatchSpotHealthAction(action.SpotPositions, "health_emergency", true)
			case "close_orphan", "close_orphan_dust":
				e.handleOrphanClose(action)
			default:
				e.log.Warn("unknown health action type: %s", action.Type)
			}
		case <-e.stopCh:
			return
		}
	}
}

// dispatchSpotHealthAction sends spot-futures positions to the SpotEngine via
// the registered callback. Each position is closed in a separate goroutine to
// avoid blocking the health action consumer.
func (e *Engine) dispatchSpotHealthAction(spotPositions []*models.SpotFuturesPosition, reason string, isEmergency bool) {
	if len(spotPositions) == 0 || e.spotCloseCallback == nil {
		return
	}
	for _, sp := range spotPositions {
		go func(pos *models.SpotFuturesPosition) {
			if err := e.spotCloseCallback(pos, reason, isEmergency); err != nil {
				e.log.Error("health %s: spot close %s failed: %v", reason, pos.ID, err)
			}
		}(sp)
	}
}

func (e *Engine) handleTransfer(action risk.HealthAction) {
	if e.cfg.DryRun {
		e.log.Info("[DRY RUN] L3 would transfer %.2f USDT from %s to %s",
			action.Amount, action.DonorExch, action.Exchange)
		return
	}

	donorExch, ok := e.exchanges[action.DonorExch]
	if !ok {
		e.log.Error("L3 transfer: donor exchange %s not found", action.DonorExch)
		return
	}

	targetExch, ok := e.exchanges[action.Exchange]
	if !ok {
		e.log.Error("L3 transfer: target exchange %s not found", action.Exchange)
		return
	}

	amountStr := fmt.Sprintf("%.4f", action.Amount)

	// NOTE: L3 cross-exchange transfer is NOT fully implemented. The code below
	// only moves funds within each exchange's own accounts (futures→spot on donor,
	// spot→futures on target). There is NO actual withdrawal/deposit between
	// exchanges — that requires withdrawal API, deposit address lookup, on-chain
	// confirmation, and is complex/risky to automate. For real cross-exchange
	// rebalancing, rely on L4 (position reduction) or L5 (emergency close) instead.
	if action.DonorExch != action.Exchange {
		e.log.Error("L3 transfer: cross-exchange transfer from %s to %s is NOT possible — "+
			"no withdrawal/deposit implemented. Skipping. L4/L5 will handle this.",
			action.DonorExch, action.Exchange)
		return
	}

	// Intra-exchange transfer: donor futures → spot, then spot → futures.
	if err := donorExch.TransferToSpot("USDT", amountStr); err != nil {
		e.log.Error("L3 transfer: donor %s futures->spot failed: %v", action.DonorExch, err)
		return
	}

	// Transfer into target's futures account
	if err := targetExch.TransferToFutures("USDT", amountStr); err != nil {
		e.log.Error("L3 transfer: target %s spot->futures failed: %v", action.Exchange, err)
		return
	}

	e.log.Info("L3 transfer complete: %.2f USDT from %s to %s",
		action.Amount, action.DonorExch, action.Exchange)
	e.recordTransfer(action.DonorExch, action.Exchange, "USDT", "internal", amountStr, "0", "", "completed", "L3-health")
}

// cancelExitGoroutine cancels a running exit goroutine for the given position
// and waits briefly for it to clean up.
func (e *Engine) cancelExitGoroutine(posID string) {
	e.exitMu.Lock()
	cancel, ok := e.exitCancels[posID]
	done := e.exitDone[posID]
	e.exitMu.Unlock()
	if ok {
		e.log.Info("cancelling exit goroutine for %s (preempted by L4/L5)", posID)
		cancel()
		if done != nil {
			select {
			case <-done:
				e.log.Info("exit goroutine for %s confirmed stopped", posID)
			case <-time.After(5 * time.Second):
				e.log.Error("exit goroutine for %s did not stop within 5s", posID)
			}
		}
	}
}

// checkDelistPositions checks if any active positions hold coins being delisted
// on Binance. If so, preempts any exit goroutine and triggers emergency close.
func (e *Engine) checkDelistPositions() {
	positions, err := e.db.GetActivePositions()
	if err != nil {
		return
	}
	for _, pos := range positions {
		if !e.discovery.IsDelisted(pos.Symbol) {
			continue
		}
		// Only auto-exit if Binance is one of the legs.
		if pos.LongExchange != "binance" && pos.ShortExchange != "binance" {
			e.log.Warn("DELIST WARNING: %s delisting but no Binance leg (%s/%s), skipping auto-exit",
				pos.Symbol, pos.LongExchange, pos.ShortExchange)
			continue
		}
		// Skip if already closing.
		if pos.Status == models.StatusClosing || pos.Status == models.StatusClosed {
			continue
		}
		e.log.Warn("DELIST ALERT: %s is being delisted on Binance, emergency closing %s (%s/%s)",
			pos.Symbol, pos.ID, pos.LongExchange, pos.ShortExchange)

		// Preempt any running exit goroutine.
		e.cancelExitGoroutine(pos.ID)

		// Broadcast alert to dashboard.
		e.api.BroadcastAlert(map[string]string{
			"type":    "delist",
			"symbol":  pos.Symbol,
			"message": fmt.Sprintf("Binance delisting %s — emergency closing %s", pos.Symbol, pos.ID),
		})

		go e.closePositionEmergency(pos)
	}
}

// registerSLOrders adds stop-loss order IDs to the SL index for instant fill detection.
func (e *Engine) registerSLOrders(pos *models.ArbitragePosition) {
	e.slIndexMu.Lock()
	defer e.slIndexMu.Unlock()
	if pos.LongSLOrderID != "" {
		key := pos.LongExchange + ":" + pos.LongSLOrderID
		e.slIndex[key] = slEntry{PosID: pos.ID, Leg: "long"}
	}
	if pos.ShortSLOrderID != "" {
		key := pos.ShortExchange + ":" + pos.ShortSLOrderID
		e.slIndex[key] = slEntry{PosID: pos.ID, Leg: "short"}
	}
}

// unregisterSLOrders removes stop-loss order IDs from the SL index.
func (e *Engine) unregisterSLOrders(pos *models.ArbitragePosition) {
	e.slIndexMu.Lock()
	defer e.slIndexMu.Unlock()
	if pos.LongSLOrderID != "" {
		delete(e.slIndex, pos.LongExchange+":"+pos.LongSLOrderID)
	}
	if pos.ShortSLOrderID != "" {
		delete(e.slIndex, pos.ShortExchange+":"+pos.ShortSLOrderID)
	}
}

// handleSLFill processes an SL fill event from the slFillCh channel.
// Two detection methods:
//  1. slIndex lookup — matches plan/algo order IDs (may miss if exchange issues new ID on trigger)
//  2. ReduceOnly detection — if a close fill arrives for a symbol we hold, and we didn't initiate it,
//     verify the leg is actually gone on the exchange before triggering emergency close.
func (e *Engine) handleSLFill(exchName string, upd exchange.OrderUpdate) {
	// Method 1: Try slIndex match (original approach).
	key := exchName + ":" + upd.OrderID
	e.slIndexMu.RLock()
	entry, ok := e.slIndex[key]
	e.slIndexMu.RUnlock()

	if ok {
		e.triggerEmergencyClose(exchName, entry.PosID, entry.Leg, upd)
		return
	}

	// Method 2: Detect unexpected close fills via ReduceOnly flag.
	// Works for all exchanges regardless of whether they set reduceOnly on SL fills
	// (Binance uses closePosition=true which may not set ro).
	if !upd.ReduceOnly || upd.Symbol == "" {
		return
	}

	// Skip if this is a known bot-initiated order.
	ownKey := exchName + ":" + upd.OrderID
	if _, isOwn := e.ownOrders.LoadAndDelete(ownKey); isOwn {
		return
	}

	// Find an active position on this exchange+symbol.
	positions, err := e.db.GetActivePositions()
	if err != nil {
		return
	}

	for _, pos := range positions {
		if pos.Status != models.StatusActive {
			continue
		}
		// Match exchange + symbol.
		var leg string
		if pos.LongExchange == exchName && strings.EqualFold(pos.Symbol, upd.Symbol) {
			leg = "long"
		} else if pos.ShortExchange == exchName && strings.EqualFold(pos.Symbol, upd.Symbol) {
			leg = "short"
		} else {
			continue
		}

		// Skip if we're actively exiting or entering this position.
		e.exitMu.Lock()
		exiting := e.exitActive[pos.ID]
		e.exitMu.Unlock()
		if exiting {
			continue
		}
		e.entryMu.Lock()
		entering := e.entryActive[exchName+":"+upd.Symbol] != ""
		e.entryMu.Unlock()
		if entering {
			continue
		}

		// Verify: confirm the leg is actually gone on the exchange.
		// This prevents false triggers from our own partial exits, L4 trims,
		// rotation closes, and consolidator trims which also use reduceOnly.
		exch, ok := e.exchanges[exchName]
		if !ok {
			continue
		}
		remaining, err := getExchangePositionSize(exch, pos.Symbol, leg)
		if err != nil {
			continue // can't verify, skip
		}
		if remaining > 0 {
			// Leg still has size — this was a partial close (trim, L4 reduction, etc.), not SL.
			continue
		}

		e.log.Warn("SL/LIQUIDATION DETECTED: %s on %s %s filled=%.6f avg=%.8f — leg confirmed flat (pos=%s leg=%s)",
			upd.OrderID, exchName, upd.Symbol, upd.FilledVolume, upd.AvgPrice, pos.ID, leg)
		e.triggerEmergencyClose(exchName, pos.ID, leg, upd)
		return
	}
}

// triggerEmergencyClose handles a detected SL/TP/liquidation fill on one leg.
func (e *Engine) triggerEmergencyClose(exchName, posID, leg string, upd exchange.OrderUpdate) {
	pos, err := e.db.GetPosition(posID)
	if err != nil || pos == nil {
		e.log.Error("SL fill: failed to load position %s: %v", posID, err)
		return
	}
	if pos.Status == models.StatusClosed || pos.Status == models.StatusClosing {
		return
	}

	e.unregisterSLOrders(pos)
	e.cancelExitGoroutine(pos.ID)

	e.log.Warn("SL TRIGGERED: order %s on %s filled (pos=%s leg=%s size=%.6f)",
		upd.OrderID, exchName, posID, leg, upd.FilledVolume)

	e.api.BroadcastAlert(map[string]string{
		"type":    "sl_triggered",
		"symbol":  pos.Symbol,
		"message": fmt.Sprintf("Stop-loss triggered on %s (%s leg) — emergency closing %s", exchName, leg, posID),
	})

	if e.cfg.EnablePerpTelegram {
		e.telegram.NotifySLTriggered(pos, leg, exchName)
	}

	go e.closePositionEmergency(pos)
}

// slFillEvent is an order fill event from a WS callback, queued for processing.
type slFillEvent struct {
	Exchange string
	Update   exchange.OrderUpdate
}

// consumeSLFills processes SL fill events from the channel.
func (e *Engine) consumeSLFills() {
	for {
		select {
		case ev := <-e.slFillCh:
			e.handleSLFill(ev.Exchange, ev.Update)
		case <-e.stopCh:
			return
		}
	}
}

// setupSLCallbacks registers non-blocking SL fill callbacks on all exchange adapters.
// Sends events to slFillCh which is consumed by consumeSLFills goroutine.
func (e *Engine) setupSLCallbacks() {
	for name, exch := range e.exchanges {
		exchName := name // capture for closure
		exch.SetOrderCallback(func(upd exchange.OrderUpdate) {
			// Non-blocking send — drop if channel full (consumeSLFills will catch up).
			select {
			case e.slFillCh <- slFillEvent{Exchange: exchName, Update: upd}:
			default:
				e.log.Warn("SL fill channel full, dropping event for %s on %s", upd.OrderID, exchName)
			}
		})
	}
}

// rebuildSLIndex loads all active positions and registers their SL orders.
func (e *Engine) rebuildSLIndex() {
	positions, err := e.db.GetActivePositions()
	if err != nil {
		e.log.Error("rebuildSLIndex: %v", err)
		return
	}
	for _, pos := range positions {
		e.registerSLOrders(pos)
	}
	e.slIndexMu.RLock()
	count := len(e.slIndex)
	e.slIndexMu.RUnlock()
	e.log.Info("SL index rebuilt: %d entries from %d positions", count, len(positions))
}

func (e *Engine) handleReduce(action risk.HealthAction) {
	if e.cfg.DryRun {
		e.log.Info("[DRY RUN] L4 would reduce %d positions on %s by %.0f%%",
			len(action.Positions), action.Exchange, action.Fraction*100)
		return
	}

	if e.cfg.EnablePerpTelegram {
		e.telegram.NotifyEmergencyClosePerp(action.Exchange, "L4", len(action.Positions))
	}

	for _, pos := range action.Positions {
		e.cancelExitGoroutine(pos.ID)

		e.log.Info("L4 reducing position %s (exchange=%s fraction=%.2f)",
			pos.ID, action.Exchange, action.Fraction)

		if err := e.reducePosition(pos, action.Fraction); err != nil {
			e.log.Error("L4 reduce position %s failed: %v", pos.ID, err)
		}

		// Re-check margin ratio after each reduction
		exch, ok := e.exchanges[action.Exchange]
		if !ok {
			continue
		}
		bal, err := exch.GetFuturesBalance()
		if err != nil {
			continue
		}
		if bal.MarginRatio < e.cfg.MarginL4Threshold {
			e.log.Info("L4 margin ratio %.4f now below threshold %.4f, stopping reductions",
				bal.MarginRatio, e.cfg.MarginL4Threshold)
			break
		}
	}
}

func (e *Engine) handleEmergencyClose(action risk.HealthAction) {
	if e.cfg.DryRun {
		e.log.Info("[DRY RUN] L5 would emergency close %d positions on %s",
			len(action.Positions), action.Exchange)
		return
	}

	if e.cfg.EnablePerpTelegram {
		e.telegram.NotifyEmergencyClosePerp(action.Exchange, "L5", len(action.Positions))
	}

	for _, pos := range action.Positions {
		e.cancelExitGoroutine(pos.ID)

		e.log.Error("L5 EMERGENCY CLOSE: position %s (exchange=%s)", pos.ID, action.Exchange)
		reason := fmt.Sprintf("L5 emergency close: %s margin critical", action.Exchange)
		_ = e.db.UpdatePositionFields(pos.ID, func(fresh *models.ArbitragePosition) bool {
			fresh.ExitReason = reason
			return true
		})
		pos.ExitReason = reason
		if err := e.closePositionEmergency(pos); err != nil {
			e.log.Error("L5 emergency close %s failed: %v", pos.ID, err)
		}
	}

	// Transfer remaining futures balance to spot for safety
	exch, ok := e.exchanges[action.Exchange]
	if !ok {
		return
	}
	bal, err := exch.GetFuturesBalance()
	if err != nil || bal.Available <= 0 {
		return
	}
	amountStr := fmt.Sprintf("%.4f", bal.Available)
	if err := exch.TransferToSpot("USDT", amountStr); err != nil {
		e.log.Error("L5 safety transfer on %s failed: %v", action.Exchange, err)
	} else {
		e.log.Info("L5 safety transfer: moved %.2f USDT to spot on %s", bal.Available, action.Exchange)
		e.recordTransfer(action.Exchange, action.Exchange+" spot", "USDT", "internal", amountStr, "0", "", "completed", "L5-safety")
	}
}

// handleOrphanClose processes orphan leg cleanup actions from the health monitor.
// "close_orphan" closes the surviving leg via market order; "close_orphan_dust"
// skips the close (too small) and just marks the position closed.
func (e *Engine) handleOrphanClose(action risk.HealthAction) {
	for _, pos := range action.Positions {
		// Skip if another goroutine is already closing this position.
		e.exitMu.Lock()
		exiting := e.exitActive[pos.ID]
		if !exiting {
			e.exitActive[pos.ID] = true
		}
		e.exitMu.Unlock()
		if exiting {
			e.log.Info("[orphan-cleanup] position %s already being exited, skipping", pos.ID)
			continue
		}

		// Claim position with CAS — set StatusClosing atomically.
		claimed := false
		_ = e.db.UpdatePositionFields(pos.ID, func(fresh *models.ArbitragePosition) bool {
			if fresh.Status != models.StatusActive {
				return false // already closing/closed by another path
			}
			fresh.Status = models.StatusClosing
			fresh.ExitReason = fmt.Sprintf("orphan %s leg on %s", action.OrphanSide, action.OrphanExchange)
			fresh.UpdatedAt = time.Now().UTC()
			claimed = true
			return true
		})
		if !claimed {
			e.exitMu.Lock()
			delete(e.exitActive, pos.ID)
			e.exitMu.Unlock()
			e.log.Info("[orphan-cleanup] position %s status changed, skipping", pos.ID)
			continue
		}

		exch, ok := e.exchanges[action.OrphanExchange]
		if !ok {
			e.log.Error("[orphan-cleanup] exchange %s not found for position %s", action.OrphanExchange, pos.ID)
			e.exitMu.Lock()
			delete(e.exitActive, pos.ID)
			e.exitMu.Unlock()
			continue
		}

		reason := fmt.Sprintf("orphan %s leg on %s (%s)", action.OrphanSide, action.OrphanExchange, action.Type)

		if action.Type == "close_orphan" {
			var side exchange.Side
			var size float64
			if action.OrphanSide == "long" {
				side = exchange.SideSell
				size = pos.LongSize
			} else {
				side = exchange.SideBuy
				size = pos.ShortSize
			}
			e.log.Info("[orphan-cleanup] closing %s %s leg on %s (size=%.6f)", pos.ID, action.OrphanSide, action.OrphanExchange, size)
			rem := e.closeFullyWithRetry(exch, pos.Symbol, side, size)
			if rem > 0 {
				e.log.Error("ORPHAN EXPOSURE: %s %s %.6f on %s — manual intervention needed", pos.Symbol, side, rem, exch.Name())
			}

			// Verify flat after close.
			time.Sleep(500 * time.Millisecond)
			remaining, verifyErr := getExchangePositionSize(exch, pos.Symbol, action.OrphanSide)
			if verifyErr != nil || remaining > 0 {
				e.log.Error("[orphan-cleanup] position %s NOT flat after close (remaining=%.6f, err=%v) — reverting to active",
					pos.ID, remaining, verifyErr)
				_ = e.db.UpdatePositionFields(pos.ID, func(fresh *models.ArbitragePosition) bool {
					fresh.Status = models.StatusActive
					if action.OrphanSide == "long" {
						fresh.LongSize = remaining
					} else {
						fresh.ShortSize = remaining
					}
					fresh.UpdatedAt = time.Now().UTC()
					return true
				})
				// Reset orphan sentinel so health monitor can re-detect this position.
				e.healthMonitor.ResetOrphanCount(pos.ID)
				e.exitMu.Lock()
				delete(e.exitActive, pos.ID)
				e.exitMu.Unlock()
				continue
			}
		}
		// "close_orphan_dust": skip market close — too small to trade.

		// Mark position as closed with proper bookkeeping.
		_ = e.db.UpdatePositionFields(pos.ID, func(fresh *models.ArbitragePosition) bool {
			fresh.Status = models.StatusClosed
			fresh.ExitReason = reason
			fresh.UpdatedAt = time.Now().UTC()
			return true
		})
		pos.Status = models.StatusClosed
		pos.ExitReason = reason

		if err := e.db.AddToHistory(pos); err != nil {
			e.log.Error("[orphan-cleanup] failed to add %s to history: %v", pos.ID, err)
		}
		// Release allocator slot.
		e.releasePerpPosition(pos.ID)
		e.api.BroadcastPositionUpdate(pos)

		e.exitMu.Lock()
		delete(e.exitActive, pos.ID)
		e.exitMu.Unlock()

		e.log.Info("[orphan-cleanup] position %s closed: %s", pos.ID, reason)
	}
}

// trackFunding updates FundingCollected for all active positions.
// Runs once on startup, then every hour at HH:10:00.
func (e *Engine) trackFunding() {
	// Run immediately on startup.
	e.updateFundingCollected()

	for {
		now := time.Now().UTC()
		// Next HH:10:00.
		next := now.Truncate(time.Hour).Add(10 * time.Minute)
		if !next.After(now) {
			next = next.Add(time.Hour)
		}
		timer := time.NewTimer(time.Until(next))

		select {
		case <-timer.C:
			e.updateFundingCollected()
		case <-e.stopCh:
			timer.Stop()
			return
		}
	}
}

// updateFundingCollected queries exchange positions for realized funding and
// updates each active position's FundingCollected field.
func (e *Engine) updateFundingCollected() {
	positions, err := e.db.GetActivePositions()
	if err != nil {
		e.log.Error("trackFunding: failed to get active positions: %v", err)
		return
	}

	for _, pos := range positions {
		if pos.Status != models.StatusActive {
			continue
		}

		longExch, ok := e.exchanges[pos.LongExchange]
		if !ok {
			continue
		}
		shortExch, ok := e.exchanges[pos.ShortExchange]
		if !ok {
			continue
		}

		// Fetch actual funding fees from each exchange independently.
		// Use position FundingFee field if available (Bitget, OKX, Gate.io),
		// otherwise fall back to GetFundingFees history (Binance, Bybit).
		var fundingAccrued float64
		var gotLong, gotShort bool

		longPos, err1 := longExch.GetPosition(pos.Symbol)
		if err1 == nil {
			for _, p := range longPos {
				if p.HoldSide == "long" && p.FundingFee != "" {
					f, _ := strconv.ParseFloat(p.FundingFee, 64)
					fundingAccrued += f
					gotLong = true
				}
			}
		}
		if !gotLong {
			if fees, err := longExch.GetFundingFees(pos.Symbol, pos.CreatedAt); err == nil {
				for _, f := range fees {
					fundingAccrued += f.Amount
				}
				gotLong = true
			}
		}

		shortPos, err2 := shortExch.GetPosition(pos.Symbol)
		if err2 == nil {
			for _, p := range shortPos {
				if p.HoldSide == "short" && p.FundingFee != "" {
					f, _ := strconv.ParseFloat(p.FundingFee, 64)
					fundingAccrued += f
					gotShort = true
				}
			}
		}
		if !gotShort {
			if fees, err := shortExch.GetFundingFees(pos.Symbol, pos.CreatedAt); err == nil {
				for _, f := range fees {
					fundingAccrued += f.Amount
				}
				gotShort = true
			}
		}

		if !gotLong || !gotShort {
			continue // skip if either side failed
		}

		// Refresh unrealized PnL per leg from position data we already fetched.
		// Only update if GetPosition succeeded; preserve last known value on failure.
		longUPL := pos.LongUnrealizedPnL
		shortUPL := pos.ShortUnrealizedPnL
		if err1 == nil {
			var acc float64
			for _, p := range longPos {
				if p.HoldSide == "long" && p.UnrealizedPL != "" {
					upl, _ := strconv.ParseFloat(p.UnrealizedPL, 64)
					acc += upl
				}
			}
			longUPL = acc
		}
		if err2 == nil {
			var acc float64
			for _, p := range shortPos {
				if p.HoldSide == "short" && p.UnrealizedPL != "" {
					upl, _ := strconv.ParseFloat(p.UnrealizedPL, 64)
					acc += upl
				}
			}
			shortUPL = acc
		}

		uplChanged := longUPL != pos.LongUnrealizedPnL || shortUPL != pos.ShortUnrealizedPnL
		fundingChanged := fundingAccrued != 0 && fundingAccrued != pos.FundingCollected

		if fundingChanged || uplChanged {
			nextFunding := e.computeNextFunding(pos.Symbol, pos.LongExchange, pos.ShortExchange)

			// Re-read position from Redis and update atomically to avoid
			// overwriting status changes made by closePosition or reducePosition.
			if err := e.db.UpdatePositionFields(pos.ID, func(fresh *models.ArbitragePosition) bool {
				if fresh.Status != models.StatusActive {
					return false // position is closing/closed, skip update
				}
				if fundingChanged {
					fresh.FundingCollected = fundingAccrued
					if !fresh.NextFunding.IsZero() && time.Now().UTC().After(fresh.NextFunding) {
						fresh.NextFunding = nextFunding
					}
				}
				fresh.LongUnrealizedPnL = longUPL
				fresh.ShortUnrealizedPnL = shortUPL
				return true
			}); err != nil {
				e.log.Error("trackFunding: failed to save position %s: %v", pos.ID, err)
			} else {
				e.api.BroadcastPositionUpdate(pos)
			}
		}
	}
}

// nextStandardSnapshot returns the next standard hourly funding snapshot at or after t.
func nextStandardSnapshot(t time.Time) time.Time {
	next := time.Date(t.Year(), t.Month(), t.Day(), t.Hour()+1, 0, 0, 0, time.UTC)
	return next
}

// computeNextFunding determines the next funding time for a symbol by querying
// both exchanges and returning the earliest next funding snapshot.
func (e *Engine) computeNextFunding(symbol, longExchName, shortExchName string) time.Time {
	var earliest time.Time

	if longExch, ok := e.exchanges[longExchName]; ok {
		if fr, err := longExch.GetFundingRate(symbol); err == nil && !fr.NextFunding.IsZero() {
			earliest = fr.NextFunding
		}
	}
	if shortExch, ok := e.exchanges[shortExchName]; ok {
		if fr, err := shortExch.GetFundingRate(symbol); err == nil && !fr.NextFunding.IsZero() {
			if earliest.IsZero() || fr.NextFunding.Before(earliest) {
				earliest = fr.NextFunding
			}
		}
	}

	// Fallback: next standard 4h snapshot.
	if earliest.IsZero() {
		earliest = nextStandardSnapshot(time.Now().UTC())
	}

	return earliest
}

// SimExecuteTradeV2 is a public wrapper around executeTradeV2 for use by
// the simtrade CLI tool.
func (e *Engine) SimExecuteTradeV2(opp models.Opportunity, size, price, gapBPS float64) error {
	return e.executeTradeV2(opp, size, price, gapBPS)
}

// executeArbitrage attempts to open arbitrage positions for a batch of
// opportunities. Phase 1 sequentially filters and approves candidates,
// Phase 2 executes approved trades in parallel goroutines.
// dynamicCap, when > 0, overrides the strategy cap for capital reservations (CA-04).
func (e *Engine) executeArbitrage(opps []models.Opportunity, dynamicCap float64) {
	e.log.Info("executeArbitrage: acquiring capacity lock...")
	e.capacityMu.Lock()
	e.log.Info("executeArbitrage: capacity lock acquired")

	// Loss limit pre-entry gate (per D-07: halt new entries only, existing positions continue)
	if e.lossLimiter != nil && e.cfg.EnableLossLimits {
		blocked, status := e.lossLimiter.CheckLimits()
		if blocked {
			e.log.Warn("loss limit breached (%s): daily=%.2f/%.2f weekly=%.2f/%.2f -- halting new entries",
				status.BreachType, status.DailyLoss, status.DailyLimit,
				status.WeeklyLoss, status.WeeklyLimit)
			e.api.BroadcastLossLimits(status)
			if e.cfg.EnablePerpTelegram {
				e.telegram.NotifyLossLimitBreached(status.BreachType,
					status.DailyLoss, status.DailyLimit,
					status.WeeklyLoss, status.WeeklyLimit)
			}
			e.capacityMu.Unlock()
			return
		}
	}

	active, err := e.db.GetActivePositions()
	if err != nil {
		e.capacityMu.Unlock()
		e.log.Error("failed to get active positions: %v", err)
		return
	}
	e.log.Info("executeArbitrage: %d active positions, MaxPositions=%d", len(active), e.cfg.MaxPositions)
	slots := e.cfg.MaxPositions - len(active)

	// Build set of occupied symbols to block duplicate entries.
	// Any non-closed position (active, exiting, partial, pending, closing)
	// blocks the symbol from new entry to prevent overlapping positions.
	activeSymbols := make(map[string]bool)
	for _, p := range active {
		if p.Status != models.StatusClosed {
			activeSymbols[p.Symbol] = true
		}
	}

	if slots <= 0 {
		e.capacityMu.Unlock()
		e.log.Info("at max capacity (%d/%d), skipping", len(active), e.cfg.MaxPositions)
		return
	}

	// ---------------------------------------------------------------------------
	// Pre-fetch phase: collect all unique exchanges and (exchange, symbol) pairs
	// from the opportunity batch, then fetch balances and orderbooks in parallel.
	// This avoids 4+ serial API calls per opportunity during the approval loop.
	// ---------------------------------------------------------------------------
	type prefetchKey struct{ exchange, symbol string }
	exchangeSet := map[string]bool{}
	symbolPairs := map[prefetchKey]bool{}
	for _, opp := range opps {
		if activeSymbols[opp.Symbol] {
			continue
		}
		exchangeSet[opp.LongExchange] = true
		exchangeSet[opp.ShortExchange] = true
		symbolPairs[prefetchKey{opp.LongExchange, opp.Symbol}] = true
		symbolPairs[prefetchKey{opp.ShortExchange, opp.Symbol}] = true
	}

	prefetchStart := time.Now()
	var pfWg sync.WaitGroup
	balanceSyncMap := sync.Map{}
	orderbookSyncMap := sync.Map{}
	spotBalanceSyncMap := sync.Map{}

	for exch := range exchangeSet {
		pfWg.Add(1)
		go func(name string) {
			defer pfWg.Done()
			if adapter, ok := e.exchanges[name]; ok {
				if bal, err := adapter.GetFuturesBalance(); err == nil {
					balanceSyncMap.Store(name, bal)
				} else {
					e.log.Warn("prefetch: GetFuturesBalance(%s) failed: %v", name, err)
				}
			}
		}(exch)
	}
	for exch := range exchangeSet {
		pfWg.Add(1)
		go func(name string) {
			defer pfWg.Done()
			if adapter, ok := e.exchanges[name]; ok {
				if sb, err := adapter.GetSpotBalance(); err == nil {
					spotBalanceSyncMap.Store(name, sb)
				}
			}
		}(exch)
	}
	// Collect BingX symbols separately — BingX has strict rate limits,
	// so its orderbook requests run sequentially in one goroutine.
	var bingxSymbols []string
	for key := range symbolPairs {
		if key.exchange == "bingx" {
			bingxSymbols = append(bingxSymbols, key.symbol)
		} else {
			pfWg.Add(1)
			go func(k prefetchKey) {
				defer pfWg.Done()
				if adapter, ok := e.exchanges[k.exchange]; ok {
					if ob, ok := adapter.GetDepth(k.symbol); ok {
						if ob.Time.IsZero() || time.Since(ob.Time) < 5*time.Second {
							orderbookSyncMap.Store(k.exchange+":"+k.symbol, ob)
						} else {
							e.log.Debug("prefetch: %s:%s orderbook stale (%.1fs)", k.exchange, k.symbol, time.Since(ob.Time).Seconds())
							if freshOb, err := adapter.GetOrderbook(k.symbol, 20); err == nil {
								orderbookSyncMap.Store(k.exchange+":"+k.symbol, freshOb)
							}
						}
					} else if ob, err := adapter.GetOrderbook(k.symbol, 20); err == nil {
						orderbookSyncMap.Store(k.exchange+":"+k.symbol, ob)
					} else {
						e.log.Warn("prefetch: GetOrderbook(%s:%s) failed: %v", k.exchange, k.symbol, err)
					}
				}
			}(key)
		}
	}
	if len(bingxSymbols) > 0 {
		pfWg.Add(1)
		go func() {
			defer pfWg.Done()
			adapter, ok := e.exchanges["bingx"]
			if !ok {
				return
			}
			for _, sym := range bingxSymbols {
				if ob, ok := adapter.GetDepth(sym); ok {
					if ob.Time.IsZero() || time.Since(ob.Time) < 5*time.Second {
						orderbookSyncMap.Store("bingx:"+sym, ob)
					} else {
						e.log.Debug("prefetch: bingx:%s orderbook stale (%.1fs)", sym, time.Since(ob.Time).Seconds())
						if freshOb, err := adapter.GetOrderbook(sym, 20); err == nil {
							orderbookSyncMap.Store("bingx:"+sym, freshOb)
						}
					}
				} else if ob, err := adapter.GetOrderbook(sym, 20); err == nil {
					orderbookSyncMap.Store("bingx:"+sym, ob)
				} else {
					e.log.Warn("prefetch: GetOrderbook(bingx:%s) failed: %v", sym, err)
				}
			}
		}()
	}
	pfWg.Wait()

	// Convert sync.Maps to regular maps for the cache.
	prefetchCache := &risk.PrefetchCache{
		Balances:   make(map[string]*exchange.Balance),
		Orderbooks: make(map[string]*exchange.Orderbook),
	}
	balanceSyncMap.Range(func(k, v interface{}) bool {
		prefetchCache.Balances[k.(string)] = v.(*exchange.Balance)
		return true
	})
	orderbookSyncMap.Range(func(k, v interface{}) bool {
		prefetchCache.Orderbooks[k.(string)] = v.(*exchange.Orderbook)
		return true
	})
	e.log.Info("executeArbitrage: pre-fetched %d balances + %d orderbooks in %v",
		len(prefetchCache.Balances), len(prefetchCache.Orderbooks), time.Since(prefetchStart).Round(time.Millisecond))
	// Log balance summary — uses prefetched data only (no extra API calls post-prefetch).
	{
		var summary string
		for name, bal := range prefetchCache.Balances {
			spotStr := ""
			if v, ok := spotBalanceSyncMap.Load(name); ok {
				sb := v.(*exchange.Balance)
				if sb.Available > 0.01 {
					spotStr = fmt.Sprintf(" spot=%.2f", sb.Available)
				}
			}
			if summary != "" {
				summary += " | "
			}
			summary += fmt.Sprintf("%s: total=%.2f frozen=%.2f avail=%.2f%s", name, bal.Total, bal.Frozen, bal.Available, spotStr)
		}
		e.log.Info("[entry] balances: %s", summary)
	}

	// Phase 1: Sequential pre-filter — approve up to `slots` candidates.
	type candidate struct {
		opp         models.Opportunity
		size        float64
		price       float64
		gapBPS      float64
		lockKey     string
		lock        *database.OwnedLock
		pos         *models.ArbitragePosition
		reservation *risk.CapitalReservation
	}
	var candidates []candidate
	newSlotCandidates := 0
	reserved := map[string]float64{} // tracks margin committed by prior approvals in this batch

	for _, opp := range opps {
		if activeSymbols[opp.Symbol] {
			continue // skip — symbol already has an active position
		}
		// Check blacklist
		if blocked, err := e.db.IsBlacklisted(opp.Symbol); err == nil && blocked {
			if e.rejStore != nil {
				e.rejStore.AddOpp(opp, "engine", "symbol blacklisted")
			}
			continue
		}
		if newSlotCandidates >= slots {
			continue
		}

		lockResource := fmt.Sprintf("execute:%s", opp.Symbol)
		symLock, acquired, err := e.db.AcquireOwnedLock(lockResource, 5*time.Minute)
		if err != nil {
			e.log.Error("failed to acquire lock for %s: %v", opp.Symbol, err)
			continue
		}
		if !acquired {
			e.log.Info("lock busy for %s, skipping", opp.Symbol)
			continue
		}

		e.log.Info("executeArbitrage: risk.Approve %s...", opp.Symbol)
		approval, err := e.risk.ApproveWithReservedCached(opp, reserved, prefetchCache)
		if err != nil {
			e.log.Error("risk approval error for %s: %v", opp.Symbol, err)
			if e.rejStore != nil {
				e.rejStore.AddOpp(opp, "risk", fmt.Sprintf("error: %v", err))
			}
			symLock.Release()
			continue
		}
		if !approval.Approved {
			e.log.Info("risk rejected %s: %s", opp.Symbol, approval.Reason)
			if e.rejStore != nil {
				e.rejStore.AddOpp(opp, "risk", approval.Reason)
			}
			symLock.Release()
			continue
		}
		e.log.Info("executeArbitrage: risk approved %s size=%.6f", opp.Symbol, approval.Size)

		if e.cfg.DryRun {
			e.log.Info("[DRY RUN] would execute trade for %s: size=%.6f price=%.2f spread=%.2f bps/h", opp.Symbol, approval.Size, approval.Price, opp.Spread)
			symLock.Release()
			continue
		}

		reservation, err := e.reservePerpCapital(opp, approval, dynamicCap)
		if err != nil {
			e.log.Info("capital allocator rejected %s: %v", opp.Symbol, err)
			if e.rejStore != nil {
				e.rejStore.AddOpp(opp, "risk", fmt.Sprintf("capital allocator: %v", err))
			}
			symLock.Release()
			continue
		}

		pendingPos := e.createPendingPosition(opp)
		if err := e.db.SavePosition(pendingPos); err != nil {
			e.releasePerpReservation(reservation)
			e.log.Error("failed to save pending position for %s: %v", opp.Symbol, err)
			symLock.Release()
			continue
		}
		e.api.BroadcastPositionUpdate(pendingPos)

		candidates = append(candidates, candidate{
			opp:         opp,
			size:        approval.Size,
			price:       approval.Price,
			gapBPS:      approval.GapBPS,
			lockKey:     lockResource,
			lock:        symLock,
			pos:         pendingPos,
			reservation: reservation,
		})
		reserved[opp.LongExchange] += approval.RequiredMargin
		reserved[opp.ShortExchange] += approval.RequiredMargin
		newSlotCandidates++
	}

	// Release capacity lock — candidates hold per-symbol locks and will
	// create pending positions inside executeTradeV2.
	e.log.Info("executeArbitrage: releasing capacity lock, %d candidates approved", len(candidates))
	e.capacityMu.Unlock()

	if len(candidates) == 0 {
		return
	}

	// Phase 2: Parallel execution — launch all candidates simultaneously.
	e.log.Info("parallel execution: %d candidates approved, launching", len(candidates))

	var wg sync.WaitGroup
	for _, c := range candidates {
		wg.Add(1)
		go func(c candidate) {
			defer wg.Done()
			defer c.lock.Release()

			e.log.Info("executing trade for %s: size=%.6f price=%.5f gapBPS=%.1f", c.opp.Symbol, c.size, c.price, c.gapBPS)
			err := e.executeTradeV2WithPos(c.opp, c.pos, c.size, c.price, c.gapBPS)
			if errors.Is(err, errPartialEntry) {
				if cErr := e.commitPerpCapital(c.reservation, c.pos.ID); cErr != nil {
					e.log.Error("capital commit for partial %s: %v", c.opp.Symbol, cErr)
					if e.allocator != nil && e.allocator.Enabled() {
						_ = e.allocator.Reconcile()
					}
				}
				return
			}
			if err != nil {
				e.releasePerpReservation(c.reservation)
				e.log.Error("trade execution failed for %s: %v", c.opp.Symbol, err)
				e.cleanupFailedPosition(c.opp.Symbol, err.Error())
				return
			}
			if cErr := e.commitPerpCapital(c.reservation, c.pos.ID); cErr != nil {
				e.log.Error("capital commit failed for %s: %v — triggering allocator reconcile", c.opp.Symbol, cErr)
				if e.allocator != nil && e.allocator.Enabled() {
					_ = e.allocator.Reconcile()
				}
			}
		}(c)
	}
	e.log.Info("executeArbitrage: waiting for %d goroutines to complete...", len(candidates))
	wg.Wait()
	e.log.Info("executeArbitrage: all goroutines complete")
}

// removePendingPosition marks a specific pending position as closed and broadcasts
// the update, so the frontend immediately removes the row.
func (e *Engine) removePendingPosition(pos *models.ArbitragePosition) {
	pos.Status = models.StatusClosed
	pos.UpdatedAt = time.Now().UTC()
	if err := e.db.SavePosition(pos); err != nil {
		e.log.Error("removePendingPosition: failed to close %s: %v", pos.ID, err)
	}
	if e.api != nil {
		e.api.BroadcastPositionUpdate(pos)
	}
}

// findSiblingPosition returns an existing active position for the same symbol
// and exchange pair, excluding the given position ID.
func (e *Engine) findSiblingPosition(symbol, longExch, shortExch, excludeID string) *models.ArbitragePosition {
	positions, err := e.db.GetActivePositions()
	if err != nil {
		return nil
	}
	for _, p := range positions {
		if p.ID == excludeID {
			continue
		}
		if p.Symbol == symbol && p.LongExchange == longExch && p.ShortExchange == shortExch && p.Status == models.StatusActive {
			return p
		}
	}
	return nil
}

// mergeIntoPosition merges new fills into an existing active position using
// weighted average entry prices. Cancels old SLs and places new ones.
// Returns true if merge succeeded, false if the sibling changed state and
// merge was skipped (caller should keep the pending position as-is).
func (e *Engine) mergeIntoPosition(existing, pending *models.ArbitragePosition,
	addLong, addShort, longPrice, shortPrice float64, spread float64) bool {

	merged := false
	err := e.db.UpdatePositionFields(existing.ID, func(fresh *models.ArbitragePosition) bool {
		// Guard: only merge into active positions with matching exchanges
		if fresh.Status != models.StatusActive {
			return false
		}
		if fresh.LongExchange != pending.LongExchange || fresh.ShortExchange != pending.ShortExchange {
			return false
		}

		// Weighted average entry prices
		oldLong := fresh.LongSize
		oldShort := fresh.ShortSize
		totalLong := oldLong + addLong
		totalShort := oldShort + addShort
		if totalLong > 0 {
			fresh.LongEntry = (oldLong*fresh.LongEntry + addLong*longPrice) / totalLong
		}
		if totalShort > 0 {
			fresh.ShortEntry = (oldShort*fresh.ShortEntry + addShort*shortPrice) / totalShort
		}
		fresh.LongSize = totalLong
		fresh.ShortSize = totalShort
		fresh.EntryNotional = math.Max(fresh.LongEntry*fresh.LongSize, fresh.ShortEntry*fresh.ShortSize)

		// Weighted average entry spread (feeds exit logic)
		if totalLong > 0 {
			fresh.EntrySpread = (oldLong*fresh.EntrySpread + addLong*spread) / totalLong
		}

		fresh.UpdatedAt = time.Now().UTC()
		merged = true
		return true
	})
	if err != nil {
		e.log.Error("mergeIntoPosition: failed to update %s: %v", existing.ID, err)
		return false
	}
	if !merged {
		e.log.Warn("mergeIntoPosition: sibling %s changed state, skipping merge", existing.ID)
		return false
	}

	// Cancel old SLs and place new ones with combined size
	e.cancelStopLosses(existing)

	// Re-read for accurate state (after SL cancel, before SL attach)
	updated, err := e.db.GetPosition(existing.ID)
	if err != nil {
		e.log.Error("mergeIntoPosition: failed to re-read %s, attaching SLs with stale data: %v", existing.ID, err)
		e.attachStopLosses(existing) // fallback: use stale pointer so position isn't left unprotected
		return true
	}

	e.attachStopLosses(updated)

	// Broadcast after SL attachment so UI has correct SL IDs
	e.api.BroadcastPositionUpdate(updated)

	e.log.Info("merge complete: %s now size=%.6f long=%.6f@%.6f short=%.6f@%.6f",
		updated.ID, updated.LongSize, updated.LongSize, updated.LongEntry, updated.ShortSize, updated.ShortEntry)
	return true
}

// MergeExistingDuplicates finds and merges any active positions that share the
// same symbol + exchange pair. Called once on startup to clean up pre-existing duplicates.
func (e *Engine) MergeExistingDuplicates() {
	positions, err := e.db.GetActivePositions()
	if err != nil {
		e.log.Error("MergeExistingDuplicates: failed to get positions: %v", err)
		return
	}

	// Group by (symbol, longExchange, shortExchange)
	type key struct{ symbol, long, short string }
	groups := make(map[key][]*models.ArbitragePosition)
	for _, p := range positions {
		if p.Status != models.StatusActive {
			continue
		}
		k := key{p.Symbol, p.LongExchange, p.ShortExchange}
		groups[k] = append(groups[k], p)
	}

	for k, group := range groups {
		if len(group) < 2 {
			continue
		}

		// Keep the earliest position as survivor
		survivor := group[0]
		for _, p := range group[1:] {
			if p.CreatedAt.Before(survivor.CreatedAt) {
				survivor = p
			}
		}

		// Merge all others into survivor
		for _, p := range group {
			if p.ID == survivor.ID {
				continue
			}
			e.log.Info("startup merge: absorbing %s into %s (%s %s→%s)",
				p.ID, survivor.ID, k.symbol, k.long, k.short)

			// Cancel SLs on the absorbed position
			e.cancelStopLosses(p)

			// Merge sizes and funding into survivor
			_ = e.db.UpdatePositionFields(survivor.ID, func(fresh *models.ArbitragePosition) bool {
				oldLong := fresh.LongSize
				oldShort := fresh.ShortSize
				totalLong := oldLong + p.LongSize
				totalShort := oldShort + p.ShortSize
				if totalLong > 0 {
					fresh.LongEntry = (oldLong*fresh.LongEntry + p.LongSize*p.LongEntry) / totalLong
				}
				if totalShort > 0 {
					fresh.ShortEntry = (oldShort*fresh.ShortEntry + p.ShortSize*p.ShortEntry) / totalShort
				}
				fresh.LongSize = totalLong
				fresh.ShortSize = totalShort
				fresh.FundingCollected += p.FundingCollected
				fresh.RotationPnL += p.RotationPnL
				fresh.EntryFees += p.EntryFees
				// Always recompute EntryNotional after merge since entry/size changed.
				fresh.EntryNotional = math.Max(fresh.LongEntry*fresh.LongSize, fresh.ShortEntry*fresh.ShortSize)
				if totalLong > 0 {
					fresh.EntrySpread = (oldLong*fresh.EntrySpread + p.LongSize*p.EntrySpread) / totalLong
				}
				// Merge AllExchanges
				for _, ex := range p.AllExchanges {
					found := false
					for _, e := range fresh.AllExchanges {
						if e == ex {
							found = true
							break
						}
					}
					if !found {
						fresh.AllExchanges = append(fresh.AllExchanges, ex)
					}
				}
				fresh.UpdatedAt = time.Now().UTC()
				return true
			})

			// Mark absorbed position as closed
			e.removePendingPosition(p)
		}

		// Re-read survivor, replace SLs with combined size
		updated, _ := e.db.GetPosition(survivor.ID)
		if updated != nil {
			e.cancelStopLosses(updated)
			e.attachStopLosses(updated)
			e.api.BroadcastPositionUpdate(updated)
			e.log.Info("startup merge complete: %s size=%.6f long@%.6f short@%.6f",
				updated.ID, updated.LongSize, updated.LongEntry, updated.ShortEntry)
		}
	}
}

// cleanupFailedPosition finds pending or partial positions for the given symbol
// with zero fills on both legs and marks them as closed so they don't block
// MAX_POSITIONS capacity. The reason parameter is stored as failure metadata
// and the position is added to history for post-mortem analysis.
func (e *Engine) cleanupFailedPosition(symbol string, reason string) {
	positions, err := e.db.GetActivePositions()
	if err != nil {
		e.log.Error("cleanupFailedPosition: failed to get positions: %v", err)
		return
	}

	for _, pos := range positions {
		if pos.Symbol != symbol {
			continue
		}
		if pos.Status != models.StatusPending && pos.Status != models.StatusPartial {
			continue
		}
		if pos.LongSize > 0 || pos.ShortSize > 0 {
			continue
		}
		e.log.Info("cleaning up orphaned %s position %s (status=%s reason=%s)", symbol, pos.ID, pos.Status, reason)
		pos.Status = models.StatusClosed
		pos.FailureReason = reason
		pos.FailureStage = deriveFailureStage(reason)
		pos.ExitReason = "entry_failed: " + reason
		pos.UpdatedAt = time.Now().UTC()
		// Persist to history before saving state so the record is not lost.
		if err := e.db.AddToHistory(pos); err != nil {
			e.log.Error("cleanupFailedPosition: AddToHistory failed for %s: %v", pos.ID, err)
		}
		if err := e.db.SavePosition(pos); err != nil {
			e.log.Error("cleanupFailedPosition: failed to close %s: %v", pos.ID, err)
		}
		// Broadcast removal so the frontend drops the phantom row immediately.
		if e.api != nil {
			e.api.BroadcastPositionUpdate(pos)
		}
	}
}

// deriveFailureStage maps an error message to a human-readable failure stage.
func deriveFailureStage(reason string) string {
	r := strings.ToLower(reason)
	switch {
	case strings.Contains(r, "depth data not available"), strings.Contains(r, "depth subscribe"):
		return "depth_subscribe"
	case strings.Contains(r, "depth fill"), strings.Contains(r, "below minimum"):
		return "depth_fill"
	case strings.Contains(r, "circuit breaker"), strings.Contains(r, "consecutive fail"):
		return "circuit_breaker"
	case strings.Contains(r, "margin"), strings.Contains(r, "insufficient"):
		return "margin"
	case strings.Contains(r, "place order"), strings.Contains(r, "placeorder"):
		return "order_placement"
	case strings.Contains(r, "leverage"), strings.Contains(r, "margin mode"):
		return "setup"
	default:
		return "unknown"
	}
}

// retrySecondLeg retries the second leg of a trade with escalating slippage.
// Attempts 0-1 use normal slippage, 2-3 use double slippage. If IOC attempts
// fail, switches to market orders and retries until filled or margin error.
// The goal: once leg 1 is filled, leg 2 MUST be filled to match.
func (e *Engine) retrySecondLeg(exch exchange.Exchange, exchName string, symbol string, side exchange.Side, remainingSize float64, refPrice float64) (filled float64, avgPrice float64) {
	baseSlippage := e.cfg.SlippageBPS / 10000.0
	var totalFilled, totalNotional float64

	// Phase 1: IOC with escalating slippage (attempts 0-3).
	for attempt := 0; attempt < 4 && remainingSize > 0; attempt++ {
		if attempt > 0 {
			time.Sleep(200 * time.Millisecond)
		}

		bbo, ok := exch.GetBBO(symbol)
		if !ok {
			bbo = exchange.BBO{Bid: refPrice, Ask: refPrice}
		}

		slippage := baseSlippage
		if attempt >= 2 {
			slippage = baseSlippage * 2
		}

		var orderPrice float64
		if side == exchange.SideBuy {
			orderPrice = bbo.Ask * (1 + slippage)
		} else {
			orderPrice = bbo.Bid * (1 - slippage)
		}

		sizeStr := e.formatSize(exchName, symbol, remainingSize)
		params := exchange.PlaceOrderParams{
			Symbol:    symbol,
			Side:      side,
			OrderType: "limit",
			Price:     e.formatPrice(exchName, symbol, orderPrice),
			Size:      sizeStr,
			Force:     "ioc",
		}
		e.log.Info("retrySecondLeg[IOC-%d] %s %s: price=%s size=%s slippage=%.4f",
			attempt, exchName, symbol, params.Price, sizeStr, slippage)

		orderID, err := exch.PlaceOrder(params)
		if err != nil {
			e.log.Warn("retrySecondLeg[IOC-%d] %s %s: PlaceOrder failed: %v", attempt, exchName, symbol, err)
			if isMarginError(err) {
				e.log.Error("retrySecondLeg: margin error on %s, aborting", exchName)
				goto done
			}
			continue
		}
		e.ownOrders.Store(exchName+":"+orderID, struct{}{})

		time.Sleep(500 * time.Millisecond)
		filledQty, qErr := exch.GetOrderFilledQty(orderID, symbol)
		if qErr != nil {
			e.log.Warn("retrySecondLeg[IOC-%d] %s %s: GetOrderFilledQty failed: %v", attempt, exchName, symbol, qErr)
			continue
		}
		if filledQty > 0 {
			totalFilled += filledQty
			totalNotional += filledQty * orderPrice
			remainingSize -= filledQty
			e.log.Info("retrySecondLeg[IOC-%d] %s %s: filled=%.6f remaining=%.6f",
				attempt, exchName, symbol, filledQty, remainingSize)
		} else {
			e.log.Info("retrySecondLeg[IOC-%d] %s %s: no fill", attempt, exchName, symbol)
		}
	}

	// Phase 2: Market order — keep retrying until fully filled.
	// Leg 1 is already open, we MUST match it.
	for mktAttempt := 0; remainingSize > 0 && mktAttempt < 10; mktAttempt++ {
		if mktAttempt > 0 {
			time.Sleep(500 * time.Millisecond)
		}

		bbo, ok := exch.GetBBO(symbol)
		if !ok {
			bbo = exchange.BBO{Bid: refPrice, Ask: refPrice}
		}

		sizeStr := e.formatSize(exchName, symbol, remainingSize)
		e.log.Info("retrySecondLeg[MKT-%d] %s %s: MARKET size=%s", mktAttempt, exchName, symbol, sizeStr)

		orderID, err := exch.PlaceOrder(exchange.PlaceOrderParams{
			Symbol:    symbol,
			Side:      side,
			OrderType: "market",
			Size:      sizeStr,
		})
		if err != nil {
			e.log.Warn("retrySecondLeg[MKT-%d] %s %s: PlaceOrder failed: %v", mktAttempt, exchName, symbol, err)
			if isMarginError(err) {
				e.log.Error("retrySecondLeg: margin error on %s, aborting market retries", exchName)
				break
			}
			continue
		}
		e.ownOrders.Store(exchName+":"+orderID, struct{}{})

		time.Sleep(500 * time.Millisecond)
		filledQty, qErr := exch.GetOrderFilledQty(orderID, symbol)
		if qErr != nil {
			e.log.Warn("retrySecondLeg[MKT-%d] %s %s: GetOrderFilledQty failed: %v", mktAttempt, exchName, symbol, qErr)
			continue
		}
		if filledQty > 0 {
			fillPrice := (bbo.Bid + bbo.Ask) / 2
			totalFilled += filledQty
			totalNotional += filledQty * fillPrice
			remainingSize -= filledQty
			e.log.Info("retrySecondLeg[MKT-%d] %s %s: filled=%.6f remaining=%.6f",
				mktAttempt, exchName, symbol, filledQty, remainingSize)
		} else {
			e.log.Info("retrySecondLeg[MKT-%d] %s %s: no fill", mktAttempt, exchName, symbol)
		}
	}

done:
	if totalFilled > 0 {
		avgPrice = totalNotional / totalFilled
	}
	return totalFilled, avgPrice
}

// executeTrade opens a delta-neutral position across two exchanges using
// simultaneous IOC limit orders with price protection ceilings. Both legs
// fire concurrently for near-simultaneous execution, then partial fills
// are trimmed to the smaller leg.
func (e *Engine) executeTrade(opp models.Opportunity, size float64, price float64) error {
	now := time.Now().UTC()
	posID := utils.GenerateID(opp.Symbol, now.UnixMilli())

	// Use verifier-enriched NextFunding when available, otherwise compute from exchange APIs.
	nextFunding := opp.NextFunding
	if nextFunding.IsZero() {
		nextFunding = e.computeNextFunding(opp.Symbol, opp.LongExchange, opp.ShortExchange)
	}

	pos := &models.ArbitragePosition{
		ID:            posID,
		Symbol:        opp.Symbol,
		LongExchange:  opp.LongExchange,
		ShortExchange: opp.ShortExchange,
		Status:        models.StatusPending,
		EntrySpread:   opp.Spread,
		AllExchanges:  []string{opp.LongExchange, opp.ShortExchange},
		NextFunding:   nextFunding,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if err := e.db.SavePosition(pos); err != nil {
		return fmt.Errorf("save pending position: %w", err)
	}
	e.api.BroadcastPositionUpdate(pos)

	longExch, ok := e.exchanges[opp.LongExchange]
	if !ok {
		return fmt.Errorf("long exchange %s not found", opp.LongExchange)
	}
	shortExch, ok := e.exchanges[opp.ShortExchange]
	if !ok {
		return fmt.Errorf("short exchange %s not found", opp.ShortExchange)
	}

	// Set leverage and margin mode on both exchanges.
	leverage := strconv.Itoa(e.cfg.Leverage)
	for _, setup := range []struct {
		exch exchange.Exchange
		name string
		side string
	}{
		{longExch, opp.LongExchange, "long"},
		{shortExch, opp.ShortExchange, "short"},
	} {
		if err := setup.exch.SetLeverage(opp.Symbol, leverage, setup.side); err != nil {
			if isAlreadySetError(err) {
				e.log.Debug("set leverage on %s: %v (already set)", setup.name, err)
			} else {
				return fmt.Errorf("set leverage on %s: %w", setup.name, err)
			}
		}
		if err := setup.exch.SetMarginMode(opp.Symbol, "cross"); err != nil {
			if isAlreadySetError(err) {
				e.log.Debug("set margin mode on %s: %v (already set)", setup.name, err)
			} else {
				return fmt.Errorf("set margin mode on %s: %w", setup.name, err)
			}
		}
	}

	sizeStr := utils.FormatSize(size, 6)

	// ---------------------------------------------------------------
	// BBO snapshot → compute IOC limit prices with slippage ceiling
	// ---------------------------------------------------------------
	longBBO, ok := longExch.GetBBO(opp.Symbol)
	if !ok {
		longBBO = exchange.BBO{Bid: price, Ask: price}
	}
	shortBBO, ok := shortExch.GetBBO(opp.Symbol)
	if !ok {
		shortBBO = exchange.BBO{Bid: price, Ask: price}
	}

	slippage := e.cfg.SlippageBPS / 10000.0
	longCeiling := longBBO.Ask * (1 + slippage)
	shortFloor := shortBBO.Bid * (1 - slippage)

	e.log.Info("IOC entry for %s: long=%s ask=%.6f ceiling=%.6f | short=%s bid=%.6f floor=%.6f | size=%s",
		opp.Symbol, opp.LongExchange, longBBO.Ask, longCeiling,
		opp.ShortExchange, shortBBO.Bid, shortFloor, sizeStr)

	// ---------------------------------------------------------------
	// Fire both IOC limit orders concurrently
	// ---------------------------------------------------------------
	type orderResult struct {
		orderID string
		err     error
	}
	longCh := make(chan orderResult, 1)
	shortCh := make(chan orderResult, 1)

	go func() {
		oid, err := longExch.PlaceOrder(exchange.PlaceOrderParams{
			Symbol:    opp.Symbol,
			Side:      exchange.SideBuy,
			OrderType: "limit",
			Price:     e.formatPrice(opp.LongExchange, opp.Symbol, longCeiling),
			Size:      sizeStr,
			Force:     "ioc",
		})
		longCh <- orderResult{oid, err}
	}()
	go func() {
		oid, err := shortExch.PlaceOrder(exchange.PlaceOrderParams{
			Symbol:    opp.Symbol,
			Side:      exchange.SideSell,
			OrderType: "limit",
			Price:     e.formatPrice(opp.ShortExchange, opp.Symbol, shortFloor),
			Size:      sizeStr,
			Force:     "ioc",
		})
		shortCh <- orderResult{oid, err}
	}()

	longRes := <-longCh
	shortRes := <-shortCh

	// Handle placement failures.
	if longRes.err != nil && shortRes.err != nil {
		e.log.Error("both IOC orders failed: long=%v short=%v", longRes.err, shortRes.err)
		pos.Status = models.StatusClosed
		pos.UpdatedAt = time.Now().UTC()
		_ = e.db.SavePosition(pos)
		return fmt.Errorf("both IOC orders failed")
	}
	if longRes.err != nil {
		e.log.Error("long IOC failed: %v, closing short leg", longRes.err)
		rem := e.closeFullyWithRetry(shortExch, opp.Symbol, exchange.SideBuy, size) // buy to close short
		if rem > 0 {
			e.log.Error("ORPHAN EXPOSURE: %s %s %.6f on %s — manual intervention needed", opp.Symbol, exchange.SideBuy, rem, shortExch.Name())
		}
		pos.Status = models.StatusClosed
		pos.UpdatedAt = time.Now().UTC()
		_ = e.db.SavePosition(pos)
		return fmt.Errorf("long IOC failed: %w", longRes.err)
	}
	if shortRes.err != nil {
		e.log.Error("short IOC failed: %v, closing long leg", shortRes.err)
		rem := e.closeFullyWithRetry(longExch, opp.Symbol, exchange.SideSell, size) // sell to close long
		if rem > 0 {
			e.log.Error("ORPHAN EXPOSURE: %s %s %.6f on %s — manual intervention needed", opp.Symbol, exchange.SideSell, rem, longExch.Name())
		}
		pos.Status = models.StatusClosed
		pos.UpdatedAt = time.Now().UTC()
		_ = e.db.SavePosition(pos)
		return fmt.Errorf("short IOC failed: %w", shortRes.err)
	}

	pos.LongOrderID = longRes.orderID
	pos.ShortOrderID = shortRes.orderID
	pos.Status = models.StatusPartial
	pos.UpdatedAt = time.Now().UTC()
	_ = e.db.SavePosition(pos)

	e.log.Info("IOC pair fired: long=%s short=%s", longRes.orderID, shortRes.orderID)

	// ---------------------------------------------------------------
	// Confirm fills (WS + REST fallback, 5s timeout)
	// ---------------------------------------------------------------
	longFilled, longAvg, longCFErr := e.confirmFillSafe(longExch, longRes.orderID, opp.Symbol)
	shortFilled, shortAvg, shortCFErr := e.confirmFillSafe(shortExch, shortRes.orderID, opp.Symbol)
	if longCFErr != nil {
		e.log.Error("IOC fill state unknown for long %s: %v", longRes.orderID, longCFErr)
	}
	if shortCFErr != nil {
		e.log.Error("IOC fill state unknown for short %s: %v", shortRes.orderID, shortCFErr)
	}
	// If one side filled but the other is unknown, we have orphan risk
	if longFilled > 0 && shortCFErr != nil {
		e.log.Error("ORPHAN WARNING: %s long filled %.6f but short fill unknown — saving as partial", posID, longFilled)
		pos.LongSize = longFilled
		pos.ShortSize = 0
		pos.Status = models.StatusPartial
		pos.FailureReason = "short_fill_unknown"
		pos.UpdatedAt = time.Now().UTC()
		_ = e.db.SavePosition(pos)
		return errPartialEntry
	}
	if shortFilled > 0 && longCFErr != nil {
		e.log.Error("ORPHAN WARNING: %s short filled %.6f but long fill unknown — saving as partial", posID, shortFilled)
		pos.LongSize = 0
		pos.ShortSize = shortFilled
		pos.Status = models.StatusPartial
		pos.FailureReason = "long_fill_unknown"
		pos.UpdatedAt = time.Now().UTC()
		_ = e.db.SavePosition(pos)
		return errPartialEntry
	}

	e.log.Info("IOC fills for %s: long=%.6f@%.6f short=%.6f@%.6f",
		opp.Symbol, longFilled, longAvg, shortFilled, shortAvg)

	// ---------------------------------------------------------------
	// Trim to smaller leg
	// ---------------------------------------------------------------
	minFill := math.Min(longFilled, shortFilled)
	const minPositionUSDT = 10.0

	if minFill*price < minPositionUSDT {
		// Too small to keep — close whatever filled.
		e.log.Warn("IOC fills too small for %s (min=%.2f USDT), aborting", posID, minFill*price)
		if longFilled > 0 {
			rem := e.closeFullyWithRetry(longExch, opp.Symbol, exchange.SideSell, longFilled)
			if rem > 0 {
				e.log.Error("ORPHAN EXPOSURE: %s %s %.6f on %s — manual intervention needed", opp.Symbol, exchange.SideSell, rem, longExch.Name())
			}
		}
		if shortFilled > 0 {
			rem := e.closeFullyWithRetry(shortExch, opp.Symbol, exchange.SideBuy, shortFilled)
			if rem > 0 {
				e.log.Error("ORPHAN EXPOSURE: %s %s %.6f on %s — manual intervention needed", opp.Symbol, exchange.SideBuy, rem, shortExch.Name())
			}
		}
		pos.Status = models.StatusClosed
		pos.UpdatedAt = time.Now().UTC()
		_ = e.db.SavePosition(pos)
		return fmt.Errorf("IOC fills below minimum (%s: long=%.6f short=%.6f)", opp.Symbol, longFilled, shortFilled)
	}

	// Trim excess from the larger leg.
	if longFilled > minFill {
		excess := longFilled - minFill
		e.log.Info("trimming long excess %.6f on %s", excess, opp.LongExchange)
		rem := e.closeFullyWithRetry(longExch, opp.Symbol, exchange.SideSell, excess)
		if rem > 0 {
			e.log.Warn("trim incomplete: %s %s %.6f remaining on %s", opp.Symbol, exchange.SideSell, rem, longExch.Name())
		}
	}
	if shortFilled > minFill {
		excess := shortFilled - minFill
		e.log.Info("trimming short excess %.6f on %s", excess, opp.ShortExchange)
		rem := e.closeFullyWithRetry(shortExch, opp.Symbol, exchange.SideBuy, excess)
		if rem > 0 {
			e.log.Warn("trim incomplete: %s %s %.6f remaining on %s", opp.Symbol, exchange.SideBuy, rem, shortExch.Name())
		}
	}

	// ---------------------------------------------------------------
	// Merge into existing sibling or activate as new position
	// ---------------------------------------------------------------
	finalLongEntry := longAvg
	finalShortEntry := shortAvg
	if finalLongEntry <= 0 {
		finalLongEntry = longBBO.Ask
	}
	if finalShortEntry <= 0 {
		finalShortEntry = shortBBO.Bid
	}

	// Activate as new position.
	pos.LongSize = minFill
	pos.ShortSize = minFill
	pos.LongEntry = finalLongEntry
	pos.ShortEntry = finalShortEntry

	// BBO slippage estimation (best-effort, non-blocking).
	// Buy (long entry): positive = worse than ask. Sell (short entry): positive = worse than bid.
	var entrySlippage float64
	if longBBO.Ask > 0 && pos.LongEntry > 0 {
		entrySlippage += pos.LongEntry - longBBO.Ask
	}
	if shortBBO.Bid > 0 && pos.ShortEntry > 0 {
		entrySlippage += shortBBO.Bid - pos.ShortEntry
	}
	pos.Slippage = entrySlippage
	pos.EntryNotional = math.Max(pos.LongEntry*pos.LongSize, pos.ShortEntry*pos.ShortSize)

	pos.Status = models.StatusActive
	pos.UpdatedAt = time.Now().UTC()
	if err := e.db.SavePosition(pos); err != nil {
		return fmt.Errorf("save active position: %w", err)
	}
	e.api.BroadcastPositionUpdate(pos)

	e.log.Info("position %s active: size=%.6f long=%.6f@%.6f(%s) short=%.6f@%.6f(%s) nextFunding=%s",
		posID, minFill, pos.LongSize, pos.LongEntry, opp.LongExchange,
		pos.ShortSize, pos.ShortEntry, opp.ShortExchange,
		pos.NextFunding.Format("15:04:05 UTC"))

	// Query entry fees asynchronously.
	posCopy := *pos
	go e.queryEntryFees(&posCopy)

	return nil
}

// formatSize rounds a size to the contract step size for a given exchange/symbol.
func (e *Engine) formatSize(exchName, symbol string, size float64) string {
	if e.contracts != nil {
		if exContracts, ok := e.contracts[exchName]; ok {
			if ci, ok := exContracts[symbol]; ok && ci.StepSize > 0 {
				rounded := utils.RoundToStep(size, ci.StepSize)
				return utils.FormatSize(rounded, ci.SizeDecimals)
			}
		}
	}
	return utils.FormatSize(size, 6)
}

// executeTradeV2 opens a delta-neutral position using depth-driven sequential
// IOC orders. It reads the live orderbook depth on each tick, sizes to available
// liquidity, and places the less-liquid leg first. If the first leg fails, it
// aborts for free (no position to unwind).
func (e *Engine) executeTradeV2(opp models.Opportunity, targetSize float64, price float64, gapBPS float64) error {
	now := time.Now().UTC()
	posID := utils.GenerateID(opp.Symbol, now.UnixMilli())
	// Use verifier-enriched NextFunding when available, otherwise compute from exchange APIs.
	nextFunding := opp.NextFunding
	if nextFunding.IsZero() {
		nextFunding = e.computeNextFunding(opp.Symbol, opp.LongExchange, opp.ShortExchange)
	}

	pos := &models.ArbitragePosition{
		ID:            posID,
		Symbol:        opp.Symbol,
		LongExchange:  opp.LongExchange,
		ShortExchange: opp.ShortExchange,
		Status:        models.StatusPending,
		EntrySpread:   opp.Spread,
		AllExchanges:  []string{opp.LongExchange, opp.ShortExchange},
		NextFunding:   nextFunding,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if err := e.db.SavePosition(pos); err != nil {
		return fmt.Errorf("save pending position: %w", err)
	}
	e.api.BroadcastPositionUpdate(pos)

	return e.executeTradeV2WithPos(opp, pos, targetSize, price, gapBPS)
}

// executeTradeV2WithPos executes a trade using a pre-created pending position.
func (e *Engine) executeTradeV2WithPos(opp models.Opportunity, pos *models.ArbitragePosition, targetSize float64, price float64, gapBPS float64) error {
	posID := pos.ID

	longExch, ok := e.exchanges[opp.LongExchange]
	if !ok {
		return fmt.Errorf("long exchange %s not found", opp.LongExchange)
	}
	shortExch, ok := e.exchanges[opp.ShortExchange]
	if !ok {
		return fmt.Errorf("short exchange %s not found", opp.ShortExchange)
	}

	// Set leverage and margin mode on both exchanges.
	leverage := strconv.Itoa(e.cfg.Leverage)
	for _, setup := range []struct {
		exch exchange.Exchange
		name string
		side string
	}{
		{longExch, opp.LongExchange, "long"},
		{shortExch, opp.ShortExchange, "short"},
	} {
		if err := setup.exch.SetLeverage(opp.Symbol, leverage, setup.side); err != nil {
			if isAlreadySetError(err) {
				e.log.Debug("set leverage on %s: %v (already set)", setup.name, err)
			} else {
				return fmt.Errorf("set leverage on %s: %w", setup.name, err)
			}
		}
		if err := setup.exch.SetMarginMode(opp.Symbol, "cross"); err != nil {
			if isAlreadySetError(err) {
				e.log.Debug("set margin mode on %s: %v (already set)", setup.name, err)
			} else {
				return fmt.Errorf("set margin mode on %s: %w", setup.name, err)
			}
		}
	}

	// ---------------------------------------------------------------
	// Subscribe to depth on both exchanges
	// ---------------------------------------------------------------
	longExch.SubscribeDepth(opp.Symbol)
	shortExch.SubscribeDepth(opp.Symbol)
	defer longExch.UnsubscribeDepth(opp.Symbol)
	defer shortExch.UnsubscribeDepth(opp.Symbol)

	// Wait up to 8s for depth data to arrive with retry.
	// Some exchanges (BingX) have unstable WS — re-subscribe if first attempt fails.
	depthReady := false
	for attempt := 0; attempt < 2; attempt++ {
		for i := 0; i < 40; i++ { // 4s per attempt
			_, lok := longExch.GetDepth(opp.Symbol)
			_, sok := shortExch.GetDepth(opp.Symbol)
			if lok && sok {
				depthReady = true
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
		if depthReady {
			break
		}
		if attempt == 0 {
			// First attempt failed — force re-subscribe (handles stale WS state)
			e.log.Warn("depth subscribe retry for %s: re-subscribing both legs", opp.Symbol)
			longExch.UnsubscribeDepth(opp.Symbol)
			shortExch.UnsubscribeDepth(opp.Symbol)
			time.Sleep(200 * time.Millisecond)
			longExch.SubscribeDepth(opp.Symbol)
			shortExch.SubscribeDepth(opp.Symbol)
		}
	}
	if !depthReady {
		_, lok := longExch.GetDepth(opp.Symbol)
		_, sok := shortExch.GetDepth(opp.Symbol)
		reason := fmt.Sprintf("depth data not available after 8s for %s (long=%v short=%v)", opp.Symbol, lok, sok)
		pos.Status = models.StatusClosed
		pos.FailureReason = reason
		pos.FailureStage = "depth_subscribe"
		pos.ExitReason = "entry_failed: " + reason
		pos.UpdatedAt = time.Now().UTC()
		_ = e.db.AddToHistory(pos)
		_ = e.db.SavePosition(pos)
		return fmt.Errorf("%s", reason)
	}

	// ---------------------------------------------------------------
	// Depth-driven fill loop
	// ---------------------------------------------------------------
	var confirmedLong, confirmedShort float64
	var longVWAP, shortVWAP float64
	var lastSavedFill float64 // for debounced position saves
	var longConsecFails, shortConsecFails int
	const maxConsecFails = 5 // abort if an exchange fails this many times in a row
	var spreadRejected bool  // only log spread rejection on state change
	deadline := time.Now().Add(time.Duration(e.cfg.EntryTimeoutSec) * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	// Mark both exchange:symbol pairs as mid-entry so the consolidator skips them.
	longKey := opp.LongExchange + ":" + opp.Symbol
	shortKey := opp.ShortExchange + ":" + opp.Symbol
	e.entryMu.Lock()
	e.entryActive[longKey] = posID
	e.entryActive[shortKey] = posID
	e.entryMu.Unlock()
	defer func() {
		e.entryMu.Lock()
		delete(e.entryActive, longKey)
		delete(e.entryActive, shortKey)
		e.entryMu.Unlock()
	}()

	e.log.Info("depth fill loop starting for %s: target=%.6f timeout=%ds",
		opp.Symbol, targetSize, e.cfg.EntryTimeoutSec)

	// Look up step size + min size for the "close enough" check.
	var stepSize, minSize float64
	if e.contracts != nil {
		if exContracts, ok := e.contracts[opp.ShortExchange]; ok {
			if ci, ok := exContracts[opp.Symbol]; ok {
				stepSize = ci.StepSize
				minSize = ci.MinSize
			}
		}
	}

	// Cached balance for margin pre-check (refreshed every 5s, not every 100ms tick)
	var cachedLongBal, cachedShortBal *exchange.Balance
	var lastBalCheck time.Time

fillLoop:
	for {
		filled := math.Min(confirmedLong, confirmedShort)
		remaining := targetSize - filled
		// Done if: exact target, within 0.5% tolerance, or remaining < step/min size (unfillable).
		closeEnough := remaining <= 0 ||
			(filled > 0 && remaining/targetSize < 0.005) ||
			(filled > 0 && stepSize > 0 && remaining < stepSize) ||
			(filled > 0 && minSize > 0 && remaining < minSize)
		if closeEnough {
			if remaining > 0 {
				e.log.Info("depth fill loop: target ~reached for %s (%.1f%% filled, remaining %.6f < step/min)",
					opp.Symbol, filled/targetSize*100, remaining)
			} else {
				e.log.Info("depth fill loop: target reached for %s", opp.Symbol)
			}
			break
		}
		if time.Now().After(deadline) {
			e.log.Warn("depth fill loop: timeout for %s (long=%.6f short=%.6f)",
				opp.Symbol, confirmedLong, confirmedShort)
			break
		}
		if longConsecFails >= maxConsecFails || shortConsecFails >= maxConsecFails {
			e.log.Error("depth fill loop: circuit breaker for %s — %s has %d consecutive failures, aborting",
				opp.Symbol, func() string {
					if longConsecFails >= maxConsecFails {
						return opp.LongExchange
					}
					return opp.ShortExchange
				}(), max(longConsecFails, shortConsecFails))
			break
		}

		select {
		case <-ticker.C:
		case <-e.stopCh:
			e.log.Info("depth fill loop: engine stopped")
			break fillLoop
		}

		// Read depth from both exchanges
		shortDepth, sok := shortExch.GetDepth(opp.Symbol)
		longDepth, lok := longExch.GetDepth(opp.Symbol)
		if !sok || !lok || len(shortDepth.Bids) == 0 || len(longDepth.Asks) == 0 {
			if time.Now().After(deadline.Add(-290 * time.Second)) { // log once near start
				e.log.Debug("depth fill %s: no depth (short=%v long=%v)", opp.Symbol, sok, lok)
			}
			continue
		}

		// Staleness check: skip if depth is older than 5 seconds
		shortAge := time.Since(shortDepth.Time)
		longAge := time.Since(longDepth.Time)
		if shortAge > 5*time.Second || longAge > 5*time.Second {
			if time.Now().After(deadline.Add(-290 * time.Second)) { // log once near start
				e.log.Debug("depth fill %s: stale (short=%.1fs long=%.1fs)", opp.Symbol, shortAge.Seconds(), longAge.Seconds())
			}
			continue
		}

		if len(shortDepth.Bids) == 0 || len(longDepth.Asks) == 0 {
			continue
		}

		bestBid := shortDepth.Bids[0].Price
		bestAsk := longDepth.Asks[0].Price
		if bestBid <= 0 || bestAsk <= 0 {
			continue
		}

		// Use the price gap measured at risk approval as the allowed spread.
		// The risk manager already validated this gap is recoverable.
		maxSpread := gapBPS

		// Top-of-book spread check
		spreadBPS := (bestAsk/bestBid - 1) * 10000
		if spreadBPS > maxSpread {
			if !spreadRejected {
				e.log.Debug("depth tick: %s spread=%.1fbps > max=%.1fbps (ask=%.6f bid=%.6f), waiting...",
					opp.Symbol, spreadBPS, maxSpread, bestAsk, bestBid)
				spreadRejected = true
			}
			continue
		}
		if spreadRejected {
			e.log.Debug("depth tick: %s spread=%.1fbps recovered (max=%.1fbps), resuming",
				opp.Symbol, spreadBPS, maxSpread)
			spreadRejected = false
		}

		// Aggregate depth across all levels within the spread threshold.
		// This captures liquidity at deeper levels (e.g. OKX fine-grained
		// ticks with small qty each but large aggregate volume).
		var bidQty, askQty float64
		var bidPrice, askPrice float64 // worst price we'd fill at
		for _, lvl := range shortDepth.Bids {
			levelSpread := (bestAsk/lvl.Price - 1) * 10000
			if levelSpread > maxSpread {
				break
			}
			bidQty += lvl.Quantity
			bidPrice = lvl.Price // worst (lowest) bid we'd accept
		}
		for _, lvl := range longDepth.Asks {
			levelSpread := (lvl.Price/bestBid - 1) * 10000
			if levelSpread > maxSpread {
				break
			}
			askQty += lvl.Quantity
			askPrice = lvl.Price // worst (highest) ask we'd pay
		}

		if bidQty <= 0 || askQty <= 0 {
			continue
		}

		// Size to available aggregated liquidity
		size := math.Min(remaining, math.Min(bidQty, askQty))

		// Round to contract step size (use short exchange as reference for step)
		if e.contracts != nil {
			if exContracts, ok := e.contracts[opp.ShortExchange]; ok {
				if ci, ok := exContracts[opp.Symbol]; ok && ci.StepSize > 0 {
					size = utils.RoundToStep(size, ci.StepSize)
					if size < ci.MinSize {
						continue
					}
				}
			}
		}

		// Min notional check
		if size*bidPrice < e.cfg.MinChunkUSDT {
			continue
		}

		// Determine less-liquid leg (higher consumption ratio = riskier → goes first)
		shortFirst := (size / bidQty) >= (size / askQty)

		// Format size per-leg: each exchange may have different StepSize/precision.
		shortSizeStr := e.formatSize(opp.ShortExchange, opp.Symbol, size)
		longSizeStr := e.formatSize(opp.LongExchange, opp.Symbol, size)
		shortPriceStr := e.formatPrice(opp.ShortExchange, opp.Symbol, bidPrice)
		longPriceStr := e.formatPrice(opp.LongExchange, opp.Symbol, askPrice)

		// Margin pre-check: cap size to what both exchanges can actually afford.
		// Cache balance for 5s to avoid hammering REST APIs every 100ms tick.
		leverage := float64(e.cfg.Leverage)
		if leverage <= 0 {
			leverage = 1
		}

		if time.Since(lastBalCheck) > 5*time.Second {
			if lb, err := longExch.GetFuturesBalance(); err == nil {
				cachedLongBal = lb
			} else {
				cachedLongBal = nil // fail-closed: clear stale cache
				e.log.Warn("depth tick: long balance refresh failed: %v", err)
			}
			if sb, err := shortExch.GetFuturesBalance(); err == nil {
				cachedShortBal = sb
			} else {
				cachedShortBal = nil // fail-closed: clear stale cache
				e.log.Warn("depth tick: short balance refresh failed: %v", err)
			}
			lastBalCheck = time.Now()
		}
		// Fail-closed: if we have no cached balance, skip this tick (don't place orders blind)
		if cachedLongBal == nil || cachedShortBal == nil {
			e.log.Warn("depth tick: no cached balance for %s, skipping tick", opp.Symbol)
			continue
		}
		{
			// Max affordable size per leg = (available * leverage) / price
			maxLongSize := (cachedLongBal.Available * leverage) / askPrice
			maxShortSize := (cachedShortBal.Available * leverage) / bidPrice
			affordableSize := math.Min(maxLongSize, maxShortSize)

			if affordableSize < size {
				// Round down to step size
				if e.contracts != nil {
					if exContracts, ok := e.contracts[opp.ShortExchange]; ok {
						if ci, ok := exContracts[opp.Symbol]; ok && ci.StepSize > 0 {
							affordableSize = utils.RoundToStep(affordableSize, ci.StepSize)
						}
					}
				}
				if affordableSize > 0 && affordableSize*bidPrice >= e.cfg.MinChunkUSDT {
					e.log.Info("depth tick: margin cap %s size %.6f → %.6f (longAvail=%.2f shortAvail=%.2f)",
						opp.Symbol, size, affordableSize, cachedLongBal.Available, cachedShortBal.Available)
					size = affordableSize
					// Re-format sizes after capping
					shortSizeStr = e.formatSize(opp.ShortExchange, opp.Symbol, size)
					longSizeStr = e.formatSize(opp.LongExchange, opp.Symbol, size)
				} else {
					e.log.Warn("depth tick: insufficient margin for %s min chunk (longAvail=%.2f shortAvail=%.2f), skipping tick",
						opp.Symbol, cachedLongBal.Available, cachedShortBal.Available)
					continue
				}
			}
		}

		// On INSUFFICIENT errors, invalidate cache to force fresh balance next tick
		var longFilled, longAvg, shortFilled, shortAvg float64

		if shortFirst {
			e.log.Info("depth tick: short-first %s size=%s bid=%.2f ask=%.2f spread=%.1fbps",
				opp.Symbol, shortSizeStr, bidPrice, askPrice, spreadBPS)

			// Short leg first (sell into bids)
			e.log.Info("depth-fill %s: placing SELL order on %s size=%s price=%s (IOC)", opp.Symbol, opp.ShortExchange, shortSizeStr, shortPriceStr)
			shortOID, err := shortExch.PlaceOrder(exchange.PlaceOrderParams{
				Symbol:    opp.Symbol,
				Side:      exchange.SideSell,
				OrderType: "limit",
				Price:     shortPriceStr,
				Size:      shortSizeStr,
				Force:     "ioc",
			})
			if err != nil {
				shortConsecFails++
				e.recordAPIError(opp.ShortExchange, err)
				// On margin error, refresh balance to get accurate state
				if isMarginError(err) {
					e.log.Warn("depth tick: short IOC margin insufficient for %s, invalidating cache", opp.Symbol)
					lastBalCheck = time.Time{} // force immediate refresh next tick
				}
				e.log.Warn("depth tick: short IOC failed (%d/%d): %v", shortConsecFails, maxConsecFails, err)
				continue
			}
			e.recordAPISuccess(opp.ShortExchange)
			var shortCFErr error
			shortFilled, shortAvg, shortCFErr = e.confirmFillSafe(shortExch, shortOID, opp.Symbol)
			if shortCFErr != nil {
				// First-leg fill state unknown — cancel and freeze this tick.
				// Do NOT continue loop blindly; the order may have filled.
				e.log.Error("depth tick: short fill state unknown for %s: %v — freezing tick", shortOID, shortCFErr)
				_ = shortExch.CancelOrder(opp.Symbol, shortOID)
				shortConsecFails++
				// Break out of depth loop to trigger post-loop reconciliation
				// which will use the force checkpoint to persist actual state.
				break
			}
			if shortFilled == 0 {
				shortConsecFails++
				e.log.Info("depth tick: short leg got 0 fill (%d/%d)", shortConsecFails, maxConsecFails)
				continue
			}
			shortConsecFails = 0

			// Long leg second (buy from asks) — only fill what short filled
			longSizeStr := e.formatSize(opp.LongExchange, opp.Symbol, shortFilled)
			e.log.Info("depth-fill %s: placing BUY order on %s size=%s price=%s (IOC)", opp.Symbol, opp.LongExchange, longSizeStr, longPriceStr)
			longOID, err := longExch.PlaceOrder(exchange.PlaceOrderParams{
				Symbol:    opp.Symbol,
				Side:      exchange.SideBuy,
				OrderType: "limit",
				Price:     longPriceStr,
				Size:      longSizeStr,
				Force:     "ioc",
			})
			if err != nil {
				e.recordAPIError(opp.LongExchange, err)
				if isMarginError(err) {
					e.log.Warn("depth tick: long IOC margin insufficient for %s after short filled, invalidating cache", opp.Symbol)
					lastBalCheck = time.Time{}
				}
				e.log.Warn("depth tick: long IOC failed after short filled %.6f, retrying second leg: %v", shortFilled, err)
				retryFilled, retryAvg := e.retrySecondLeg(longExch, opp.LongExchange, opp.Symbol, exchange.SideBuy, shortFilled, askPrice)
				if retryFilled > 0 {
					longFilled = retryFilled
					longAvg = retryAvg
					longConsecFails = 0
				} else {
					rem := e.closeFullyWithRetry(shortExch, opp.Symbol, exchange.SideBuy, shortFilled)
					if rem > 0 {
						e.log.Error("ORPHAN EXPOSURE: %s %s %.6f on %s — manual intervention needed", opp.Symbol, exchange.SideBuy, rem, shortExch.Name())
					}
					longConsecFails++
					continue
				}
			} else {
				e.recordAPISuccess(opp.LongExchange)
				var longCFErr error
				longFilled, longAvg, longCFErr = e.confirmFillSafe(longExch, longOID, opp.Symbol)
				if longCFErr != nil {
					e.log.Error("ORPHAN WARNING: %s long fill unknown after short filled %.6f: %v", posID, shortFilled, longCFErr)
					pos.LongSize = confirmedLong
					pos.ShortSize = confirmedShort + shortFilled
					pos.LongEntry = longVWAP
					pos.ShortEntry = shortVWAP
					pos.EntryNotional = math.Max(longVWAP*pos.LongSize, shortVWAP*pos.ShortSize)
					pos.Status = models.StatusPartial
					pos.FailureReason = "second_leg_fill_unknown"
					pos.UpdatedAt = time.Now().UTC()
					_ = e.db.SavePosition(pos)
					return errPartialEntry
				}
				if longFilled == 0 {
					e.log.Warn("depth tick: long confirmFill got 0 after short filled %.6f, retrying second leg", shortFilled)
					retryFilled, retryAvg := e.retrySecondLeg(longExch, opp.LongExchange, opp.Symbol, exchange.SideBuy, shortFilled, askPrice)
					if retryFilled > 0 {
						longFilled = retryFilled
						longAvg = retryAvg
						longConsecFails = 0
					} else {
						rem := e.closeFullyWithRetry(shortExch, opp.Symbol, exchange.SideBuy, shortFilled)
						if rem > 0 {
							e.log.Error("ORPHAN EXPOSURE: %s %s %.6f on %s — manual intervention needed", opp.Symbol, exchange.SideBuy, rem, shortExch.Name())
						}
						longConsecFails++
						continue
					}
				}
			}
			if longFilled > 0 && longFilled < shortFilled {
				excess := shortFilled - longFilled
				e.log.Info("depth tick: trimming short excess %.6f", excess)
				rem := e.closeFullyWithRetry(shortExch, opp.Symbol, exchange.SideBuy, excess)
				if rem > 0 {
					e.log.Warn("trim incomplete: %s %s %.6f remaining on %s", opp.Symbol, exchange.SideBuy, rem, shortExch.Name())
				}
				shortFilled = longFilled // only count matched portion
			}
		} else {
			e.log.Info("depth tick: long-first %s size=%s bid=%.2f ask=%.2f spread=%.1fbps",
				opp.Symbol, longSizeStr, bidPrice, askPrice, spreadBPS)

			// Long leg first (buy from asks)
			e.log.Info("depth-fill %s: placing BUY order on %s size=%s price=%s (IOC)", opp.Symbol, opp.LongExchange, longSizeStr, longPriceStr)
			longOID, err := longExch.PlaceOrder(exchange.PlaceOrderParams{
				Symbol:    opp.Symbol,
				Side:      exchange.SideBuy,
				OrderType: "limit",
				Price:     longPriceStr,
				Size:      longSizeStr,
				Force:     "ioc",
			})
			if err != nil {
				longConsecFails++
				e.recordAPIError(opp.LongExchange, err)
				// On margin error, refresh balance to get accurate state
				if isMarginError(err) {
					e.log.Warn("depth tick: long IOC margin insufficient for %s, invalidating cache", opp.Symbol)
					lastBalCheck = time.Time{} // force immediate refresh next tick
				}
				e.log.Warn("depth tick: long IOC failed (%d/%d): %v", longConsecFails, maxConsecFails, err)
				continue
			}
			e.recordAPISuccess(opp.LongExchange)
			var longCFErr error
			longFilled, longAvg, longCFErr = e.confirmFillSafe(longExch, longOID, opp.Symbol)
			if longCFErr != nil {
				e.log.Error("depth tick: long fill state unknown for %s: %v — freezing tick", longOID, longCFErr)
				_ = longExch.CancelOrder(opp.Symbol, longOID)
				longConsecFails++
				break // freeze depth loop — post-loop reconciliation handles
			}
			if longFilled == 0 {
				longConsecFails++
				e.log.Info("depth tick: long leg got 0 fill (%d/%d)", longConsecFails, maxConsecFails)
				continue
			}
			longConsecFails = 0

			// Short leg second — only fill what long filled
			shortSizeStr := e.formatSize(opp.ShortExchange, opp.Symbol, longFilled)
			e.log.Info("depth-fill %s: placing SELL order on %s size=%s price=%s (IOC)", opp.Symbol, opp.ShortExchange, shortSizeStr, shortPriceStr)
			shortOID, err := shortExch.PlaceOrder(exchange.PlaceOrderParams{
				Symbol:    opp.Symbol,
				Side:      exchange.SideSell,
				OrderType: "limit",
				Price:     shortPriceStr,
				Size:      shortSizeStr,
				Force:     "ioc",
			})
			if err != nil {
				e.recordAPIError(opp.ShortExchange, err)
				if isMarginError(err) {
					e.log.Warn("depth tick: short IOC margin insufficient for %s after long filled, invalidating cache", opp.Symbol)
					lastBalCheck = time.Time{}
				}
				e.log.Warn("depth tick: short IOC failed after long filled %.6f, retrying second leg: %v", longFilled, err)
				retryFilled, retryAvg := e.retrySecondLeg(shortExch, opp.ShortExchange, opp.Symbol, exchange.SideSell, longFilled, bidPrice)
				if retryFilled > 0 {
					shortFilled = retryFilled
					shortAvg = retryAvg
					shortConsecFails = 0
				} else {
					rem := e.closeFullyWithRetry(longExch, opp.Symbol, exchange.SideSell, longFilled)
					if rem > 0 {
						e.log.Error("ORPHAN EXPOSURE: %s %s %.6f on %s — manual intervention needed", opp.Symbol, exchange.SideSell, rem, longExch.Name())
					}
					shortConsecFails++
					continue
				}
			} else {
				e.recordAPISuccess(opp.ShortExchange)
				var shortCFErr error
				shortFilled, shortAvg, shortCFErr = e.confirmFillSafe(shortExch, shortOID, opp.Symbol)
				if shortCFErr != nil {
					e.log.Error("ORPHAN WARNING: %s short fill unknown after long filled %.6f: %v", posID, longFilled, shortCFErr)
					pos.LongSize = confirmedLong + longFilled
					pos.ShortSize = confirmedShort
					pos.LongEntry = longVWAP
					pos.ShortEntry = shortVWAP
					pos.EntryNotional = math.Max(longVWAP*pos.LongSize, shortVWAP*pos.ShortSize)
					pos.Status = models.StatusPartial
					pos.FailureReason = "second_leg_fill_unknown"
					pos.UpdatedAt = time.Now().UTC()
					_ = e.db.SavePosition(pos)
					return errPartialEntry
				}
				if shortFilled == 0 {
					e.log.Warn("depth tick: short confirmFill got 0 after long filled %.6f, retrying second leg", longFilled)
					retryFilled, retryAvg := e.retrySecondLeg(shortExch, opp.ShortExchange, opp.Symbol, exchange.SideSell, longFilled, bidPrice)
					if retryFilled > 0 {
						shortFilled = retryFilled
						shortAvg = retryAvg
						shortConsecFails = 0
					} else {
						rem := e.closeFullyWithRetry(longExch, opp.Symbol, exchange.SideSell, longFilled)
						if rem > 0 {
							e.log.Error("ORPHAN EXPOSURE: %s %s %.6f on %s — manual intervention needed", opp.Symbol, exchange.SideSell, rem, longExch.Name())
						}
						shortConsecFails++
						continue
					}
				}
			}
			if shortFilled > 0 && shortFilled < longFilled {
				excess := longFilled - shortFilled
				e.log.Info("depth tick: trimming long excess %.6f", excess)
				rem := e.closeFullyWithRetry(longExch, opp.Symbol, exchange.SideSell, excess)
				if rem > 0 {
					e.log.Warn("trim incomplete: %s %s %.6f remaining on %s", opp.Symbol, exchange.SideSell, rem, longExch.Name())
				}
				longFilled = shortFilled // only count matched portion
			}
		}

		// Accumulate fills with VWAP
		if longFilled > 0 {
			if longAvg <= 0 {
				longAvg = askPrice // fallback to IOC limit price
			}
			if confirmedLong > 0 {
				longVWAP = (longVWAP*confirmedLong + longAvg*longFilled) / (confirmedLong + longFilled)
			} else {
				longVWAP = longAvg
			}
			confirmedLong += longFilled
		}
		if shortFilled > 0 {
			if shortAvg <= 0 {
				shortAvg = bidPrice
			}
			if confirmedShort > 0 {
				shortVWAP = (shortVWAP*confirmedShort + shortAvg*shortFilled) / (confirmedShort + shortFilled)
			} else {
				shortVWAP = shortAvg
			}
			confirmedShort += shortFilled
		}

		minSoFar := math.Min(confirmedLong, confirmedShort)
		e.log.Info("depth fill: %s cumulative long=%.6f short=%.6f (%.1f%% of target)",
			opp.Symbol, confirmedLong, confirmedShort, minSoFar/targetSize*100)

		// Debounced position save: update on ≥10% fill increment
		if minSoFar-lastSavedFill >= targetSize*0.10 || minSoFar >= targetSize {
			pos.LongSize = confirmedLong
			pos.ShortSize = confirmedShort
			pos.Status = models.StatusPartial
			pos.UpdatedAt = time.Now().UTC()
			_ = e.db.SavePosition(pos)
			e.api.BroadcastPositionUpdate(pos)
			lastSavedFill = minSoFar
		}
	}

	// ---------------------------------------------------------------
	// Post-loop reconciliation
	// ---------------------------------------------------------------
	minFill := math.Min(confirmedLong, confirmedShort)
	const minPositionUSDT = 10.0

	if minFill*price < minPositionUSDT {
		e.log.Warn("depth fill too small for %s (%.2f USDT), aborting", posID, minFill*price)
		if confirmedLong > 0 {
			rem := e.closeFullyWithRetry(longExch, opp.Symbol, exchange.SideSell, confirmedLong)
			if rem > 0 {
				e.log.Error("ORPHAN EXPOSURE: %s %s %.6f on %s — manual intervention needed", opp.Symbol, exchange.SideSell, rem, longExch.Name())
			}
		}
		if confirmedShort > 0 {
			rem := e.closeFullyWithRetry(shortExch, opp.Symbol, exchange.SideBuy, confirmedShort)
			if rem > 0 {
				e.log.Error("ORPHAN EXPOSURE: %s %s %.6f on %s — manual intervention needed", opp.Symbol, exchange.SideBuy, rem, shortExch.Name())
			}
		}
		reason := fmt.Sprintf("depth fills below minimum (%s: long=%.6f short=%.6f)", opp.Symbol, confirmedLong, confirmedShort)
		pos.Status = models.StatusClosed
		pos.FailureReason = reason
		pos.FailureStage = "depth_fill"
		pos.ExitReason = "entry_failed: " + reason
		pos.UpdatedAt = time.Now().UTC()
		_ = e.db.AddToHistory(pos)
		_ = e.db.SavePosition(pos)
		return fmt.Errorf("%s", reason)
	}

	// Force checkpoint before trim — ensures DB has current fill state
	pos.LongSize = confirmedLong
	pos.ShortSize = confirmedShort
	pos.LongEntry = longVWAP
	pos.ShortEntry = shortVWAP
	pos.EntryNotional = math.Max(longVWAP*confirmedLong, shortVWAP*confirmedShort)
	pos.Status = models.StatusPartial
	pos.UpdatedAt = time.Now().UTC()
	var ckErr error
	for attempt := 1; attempt <= 3; attempt++ {
		ckErr = e.db.SavePosition(pos)
		if ckErr == nil {
			break
		}
		e.log.Error("force checkpoint %s attempt %d/3: %v", posID, attempt, ckErr)
		time.Sleep(time.Duration(attempt) * 500 * time.Millisecond)
	}
	if ckErr != nil {
		e.log.Error("CRITICAL: %s checkpoint failed, closing both legs to go flat", posID)
		var longRem, shortRem float64
		if confirmedLong > 0 {
			longRem = e.closeFullyWithRetry(longExch, opp.Symbol, exchange.SideSell, confirmedLong)
		}
		if confirmedShort > 0 {
			shortRem = e.closeFullyWithRetry(shortExch, opp.Symbol, exchange.SideBuy, confirmedShort)
		}
		if longRem > 0 || shortRem > 0 {
			e.log.Error("ORPHAN EXPOSURE: %s checkpoint failed + close incomplete (longRem=%.6f shortRem=%.6f)", posID, longRem, shortRem)
			pos.FailureReason = "checkpoint_failed_orphan"
			_ = e.db.SavePosition(pos)
			return errPartialEntry
		}
		pos.LongSize = 0
		pos.ShortSize = 0
		pos.Status = models.StatusClosed
		pos.FailureReason = "checkpoint_failed_rollback"
		pos.ExitReason = "entry_failed: DB unreachable, rolled back to flat"
		pos.UpdatedAt = time.Now().UTC()
		_ = e.db.SavePosition(pos)
		_ = e.db.AddToHistory(pos)
		e.api.BroadcastPositionUpdate(pos)
		return fmt.Errorf("checkpoint save failed, rolled back to flat: %w", ckErr)
	}

	// Trim excess from the larger leg — track actual post-trim sizes
	trimFailed := false
	actualLong := confirmedLong
	actualShort := confirmedShort
	if confirmedLong > minFill {
		excess := confirmedLong - minFill
		e.log.Info("trimming long excess %.6f on %s", excess, opp.LongExchange)
		rem := e.closeFullyWithRetry(longExch, opp.Symbol, exchange.SideSell, excess)
		actualLong = minFill + rem // what's actually left on exchange
		if rem > 0 {
			e.log.Error("TRIM FAILED: %s long excess %.6f on %s", opp.Symbol, rem, opp.LongExchange)
			trimFailed = true
		}
	}
	if confirmedShort > minFill {
		excess := confirmedShort - minFill
		e.log.Info("trimming short excess %.6f on %s", excess, opp.ShortExchange)
		rem := e.closeFullyWithRetry(shortExch, opp.Symbol, exchange.SideBuy, excess)
		actualShort = minFill + rem // what's actually left on exchange
		if rem > 0 {
			e.log.Error("TRIM FAILED: %s short excess %.6f on %s", opp.Symbol, rem, opp.ShortExchange)
			trimFailed = true
		}
	}

	if trimFailed {
		// Save actual post-trim sizes, not pre-trim values
		pos.LongSize = actualLong
		pos.ShortSize = actualShort
		pos.FailureReason = "trim_incomplete"
		pos.FailureStage = "post_fill_trim"
		pos.UpdatedAt = time.Now().UTC()
		_ = e.db.SavePosition(pos)
		e.api.BroadcastPositionUpdate(pos)
		return errPartialEntry
	}

	// Finalize entry prices
	finalLongEntry := longVWAP
	finalShortEntry := shortVWAP
	if finalLongEntry <= 0 {
		bbo, _ := longExch.GetBBO(opp.Symbol)
		finalLongEntry = bbo.Ask
	}
	if finalShortEntry <= 0 {
		bbo, _ := shortExch.GetBBO(opp.Symbol)
		finalShortEntry = bbo.Bid
	}

	// Activate position
	pos.LongSize = minFill
	pos.ShortSize = minFill
	pos.LongEntry = finalLongEntry
	pos.ShortEntry = finalShortEntry
	pos.EntryNotional = math.Max(pos.LongEntry*pos.LongSize, pos.ShortEntry*pos.ShortSize)

	// BBO slippage estimation (best-effort, non-blocking).
	var slippageV2 float64
	if bbo, ok := longExch.GetBBO(opp.Symbol); ok && bbo.Ask > 0 && pos.LongEntry > 0 {
		slippageV2 += pos.LongEntry - bbo.Ask
	}
	if bbo, ok := shortExch.GetBBO(opp.Symbol); ok && bbo.Bid > 0 && pos.ShortEntry > 0 {
		slippageV2 += bbo.Bid - pos.ShortEntry
	}
	pos.Slippage = slippageV2

	pos.Status = models.StatusActive
	pos.UpdatedAt = time.Now().UTC()
	var saveErr error
	for attempt := 1; attempt <= 3; attempt++ {
		saveErr = e.db.SavePosition(pos)
		if saveErr == nil {
			break
		}
		e.log.Error("save active position %s attempt %d/3: %v", posID, attempt, saveErr)
		time.Sleep(time.Duration(attempt) * time.Second)
	}
	if saveErr != nil {
		e.log.Error("CRITICAL: %s filled but save failed after 3 attempts — left as StatusPartial", posID)
		return errPartialEntry
	}
	e.api.BroadcastPositionUpdate(pos)

	// Subscribe both exchanges to the symbol's BBO price stream so that
	// exit close (Smart Close) can use IOC limit orders instead of falling
	// back to market orders when BBO is unavailable.
	longExch.SubscribeSymbol(opp.Symbol)
	shortExch.SubscribeSymbol(opp.Symbol)

	e.log.Info("position %s active (depth-v2): size=%.6f long=%.6f@%.6f(%s) short=%.6f@%.6f(%s) nextFunding=%s",
		posID, minFill, pos.LongSize, pos.LongEntry, opp.LongExchange,
		pos.ShortSize, pos.ShortEntry, opp.ShortExchange,
		pos.NextFunding.Format("15:04:05 UTC"))

	// Attach protective stop-loss orders on both legs.
	e.attachStopLosses(pos)

	// Query entry fees asynchronously.
	posCopy := *pos
	go e.queryEntryFees(&posCopy)

	return nil
}

// confirmFill checks WS then REST to get fill quantity and average price for
// an IOC order. IOC orders complete within the API round-trip, so this is
// just confirmation with a short timeout.
func (e *Engine) confirmFill(exch exchange.Exchange, orderID, symbol string) (filledQty, avgPrice float64) {
	deadline := time.Now().Add(5 * time.Second)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		// Try WS first — wait for terminal state (filled/cancelled) so we
		// get the final fill quantity, not a partial mid-fill snapshot.
		if upd, ok := exch.GetOrderUpdate(orderID); ok {
			if upd.Status == "filled" || upd.Status == "cancelled" {
				return upd.FilledVolume, upd.AvgPrice
			}
		}
		if time.Now().After(deadline) {
			// Check if WS already has a terminal state.
			if upd, ok := exch.GetOrderUpdate(orderID); ok {
				if upd.Status == "filled" || upd.Status == "cancelled" {
					e.log.Info("confirmFill: WS terminal %s on %s: status=%s filled=%.6f avg=%.8f",
						orderID, exch.Name(), upd.Status, upd.FilledVolume, upd.AvgPrice)
					return upd.FilledVolume, upd.AvgPrice
				}
				e.log.Warn("confirmFill: timeout %s on %s: WS status=%s filled=%.6f (non-terminal)",
					orderID, exch.Name(), upd.Status, upd.FilledVolume)
			} else {
				e.log.Warn("confirmFill: timeout %s on %s: no WS update at all", orderID, exch.Name())
			}
			// Cancel any resting/partial order to prevent untracked fills.
			if err := exch.CancelOrder(symbol, orderID); err != nil {
				e.log.Warn("confirmFill: cancel %s on %s: %v", orderID, exch.Name(), err)
			}
			// Re-query after cancel to get the true terminal fill state.
			time.Sleep(200 * time.Millisecond)
			restFilled, restErr := exch.GetOrderFilledQty(orderID, symbol)
			if restErr != nil {
				e.log.Warn("confirmFill: REST query %s on %s failed: %v", orderID, exch.Name(), restErr)
				return 0, 0
			}
			e.log.Info("confirmFill: REST query %s on %s: filled=%.6f", orderID, exch.Name(), restFilled)
			if restFilled > 0 {
				if upd, ok := exch.GetOrderUpdate(orderID); ok && upd.AvgPrice > 0 {
					e.log.Info("confirmFill: REST+WS %s on %s: filled=%.6f avg=%.8f",
						orderID, exch.Name(), restFilled, upd.AvgPrice)
					return restFilled, upd.AvgPrice
				}
				e.log.Warn("confirmFill: REST filled but no avg price for %s on %s", orderID, exch.Name())
				return restFilled, 0
			}
			return 0, 0
		}
		select {
		case <-ticker.C:
		case <-e.stopCh:
			return 0, 0
		}
	}
}

// confirmFillSafe wraps confirmFill with error awareness for the entry path.
// Returns an error when fill state is unknown (REST query failed), so callers
// can distinguish "confirmed zero" from "unknown".
func (e *Engine) confirmFillSafe(exch exchange.Exchange, orderID, symbol string) (float64, float64, error) {
	filled, avg := e.confirmFill(exch, orderID, symbol)
	if filled == 0 && avg == 0 {
		restFilled, restErr := exch.GetOrderFilledQty(orderID, symbol)
		if restErr != nil {
			return 0, 0, fmt.Errorf("fill state unknown for %s on %s: %w", orderID, exch.Name(), restErr)
		}
		if restFilled > 0 {
			return restFilled, 0, nil
		}
		return 0, 0, nil
	}
	return filled, avg, nil
}

// closeLeg places a reduce-only market IOC order to close or trim a position leg.
func (e *Engine) closeLeg(exch exchange.Exchange, symbol string, side exchange.Side, size string) {
	oid, err := exch.PlaceOrder(exchange.PlaceOrderParams{
		Symbol:     symbol,
		Side:       side,
		OrderType:  "market",
		Size:       size,
		Force:      "ioc",
		ReduceOnly: true,
	})
	if err != nil {
		e.log.Error("closeLeg %s %s %s: %v", exch.Name(), symbol, side, err)
		return
	}
	e.ownOrders.Store(exch.Name()+":"+oid, struct{}{})
	e.log.Info("closeLeg %s %s %s order=%s", exch.Name(), symbol, side, oid)
}

// queryEntryFees queries both exchanges for trades since position creation and
// sums the fees. Runs asynchronously after entry activation. Updates pos.EntryFees
// in Redis if fees > 0.
func (e *Engine) queryEntryFees(pos *models.ArbitragePosition) {
	since := pos.CreatedAt.Add(-1 * time.Minute)
	longExch, lok := e.exchanges[pos.LongExchange]
	shortExch, sok := e.exchanges[pos.ShortExchange]

	// Retry up to 3 times with increasing delays — some exchanges (BingX)
	// are slow to make fill records available via REST after WS confirms.
	delays := []time.Duration{5 * time.Second, 10 * time.Second, 20 * time.Second}
	var longFees, shortFees float64

	for attempt, delay := range delays {
		time.Sleep(delay)
		longFees, shortFees = 0, 0

		if lok {
			trades, err := longExch.GetUserTrades(pos.Symbol, since, 50)
			if err != nil {
				e.log.Warn("queryEntryFees %s: %s GetUserTrades failed: %v", pos.ID, pos.LongExchange, err)
			} else {
				for _, t := range trades {
					longFees += t.Fee
				}
			}
		}
		if sok {
			trades, err := shortExch.GetUserTrades(pos.Symbol, since, 50)
			if err != nil {
				e.log.Warn("queryEntryFees %s: %s GetUserTrades failed: %v", pos.ID, pos.ShortExchange, err)
			} else {
				for _, t := range trades {
					shortFees += t.Fee
				}
			}
		}

		// Both legs returned fees — done.
		if longFees > 0 && shortFees > 0 {
			break
		}
		if attempt < len(delays)-1 {
			e.log.Debug("queryEntryFees %s: attempt %d — long=%.4f short=%.4f, retrying...",
				pos.ID, attempt+1, longFees, shortFees)
		}
	}

	totalFees := longFees + shortFees
	if totalFees <= 0 {
		return
	}

	if err := e.db.UpdatePositionFields(pos.ID, func(fresh *models.ArbitragePosition) bool {
		fresh.EntryFees = totalFees
		return true
	}); err != nil {
		e.log.Error("queryEntryFees %s: failed to update: %v", pos.ID, err)
		return
	}
	e.log.Info("queryEntryFees %s: entry fees=%.4f (long=%s:%.4f short=%s:%.4f)",
		pos.ID, totalFees, pos.LongExchange, longFees, pos.ShortExchange, shortFees)
}

// attachStopLosses places protective stop-loss and take-profit orders on both legs.
// SL distance = 90% / leverage (e.g. 3x → 30%).
// TP on each leg is set at the other leg's SL trigger price, so both legs close
// when price moves significantly in either direction (prevents orphan positions).
// Logs errors but does not fail — position is already open.
func (e *Engine) attachStopLosses(pos *models.ArbitragePosition) {
	leverage := float64(e.cfg.Leverage)
	if leverage <= 0 {
		leverage = 3
	}
	distance := 0.9 / leverage

	longExch, lok := e.exchanges[pos.LongExchange]
	shortExch, sok := e.exchanges[pos.ShortExchange]

	var longSLID, shortSLID string
	var longTPID, shortTPID string

	// Compute SL trigger prices for both legs.
	longSLTrigger := pos.LongEntry * (1 - distance)   // price drops → long loses
	shortSLTrigger := pos.ShortEntry * (1 + distance)  // price rises → short loses

	// Long SL: trigger when price drops → sell to close.
	if lok && pos.LongEntry > 0 {
		tp := e.formatPrice(pos.LongExchange, pos.Symbol, longSLTrigger)
		oid, err := longExch.PlaceStopLoss(exchange.StopLossParams{
			Symbol:       pos.Symbol,
			Side:         exchange.SideSell,
			Size:         e.formatSize(pos.LongExchange, pos.Symbol, pos.LongSize),
			TriggerPrice: tp,
		})
		if err != nil {
			e.log.Error("SL placement failed on %s %s (long): %v", pos.LongExchange, pos.Symbol, err)
		} else {
			longSLID = oid
			e.log.Info("SL placed on %s %s: sell trigger=%s (long entry=%.4f, %.1f%% distance)",
				pos.LongExchange, pos.Symbol, tp, pos.LongEntry, distance*100)
		}
	}

	// Short SL: trigger when price rises → buy to close.
	if sok && pos.ShortEntry > 0 {
		tp := e.formatPrice(pos.ShortExchange, pos.Symbol, shortSLTrigger)
		oid, err := shortExch.PlaceStopLoss(exchange.StopLossParams{
			Symbol:       pos.Symbol,
			Side:         exchange.SideBuy,
			Size:         e.formatSize(pos.ShortExchange, pos.Symbol, pos.ShortSize),
			TriggerPrice: tp,
		})
		if err != nil {
			e.log.Error("SL placement failed on %s %s (short): %v", pos.ShortExchange, pos.Symbol, err)
		} else {
			shortSLID = oid
			e.log.Info("SL placed on %s %s: buy trigger=%s (short entry=%.4f, %.1f%% distance)",
				pos.ShortExchange, pos.Symbol, tp, pos.ShortEntry, distance*100)
		}
	}

	// Long TP: trigger when price rises to short's SL level → sell to close.
	// This ensures the long leg closes when the short leg's SL fires.
	if lok && pos.LongEntry > 0 && pos.ShortEntry > 0 {
		tp := e.formatPrice(pos.LongExchange, pos.Symbol, shortSLTrigger)
		oid, err := longExch.PlaceTakeProfit(exchange.TakeProfitParams{
			Symbol:       pos.Symbol,
			Side:         exchange.SideSell,
			Size:         e.formatSize(pos.LongExchange, pos.Symbol, pos.LongSize),
			TriggerPrice: tp,
		})
		if err != nil {
			e.log.Error("TP placement failed on %s %s (long): %v", pos.LongExchange, pos.Symbol, err)
		} else {
			longTPID = oid
			e.log.Info("TP placed on %s %s: sell trigger=%s (= short SL level, long entry=%.4f)",
				pos.LongExchange, pos.Symbol, tp, pos.LongEntry)
		}
	}

	// Short TP: trigger when price drops to long's SL level → buy to close.
	// This ensures the short leg closes when the long leg's SL fires.
	if sok && pos.ShortEntry > 0 && pos.LongEntry > 0 {
		tp := e.formatPrice(pos.ShortExchange, pos.Symbol, longSLTrigger)
		oid, err := shortExch.PlaceTakeProfit(exchange.TakeProfitParams{
			Symbol:       pos.Symbol,
			Side:         exchange.SideBuy,
			Size:         e.formatSize(pos.ShortExchange, pos.Symbol, pos.ShortSize),
			TriggerPrice: tp,
		})
		if err != nil {
			e.log.Error("TP placement failed on %s %s (short): %v", pos.ShortExchange, pos.Symbol, err)
		} else {
			shortTPID = oid
			e.log.Info("TP placed on %s %s: buy trigger=%s (= long SL level, short entry=%.4f)",
				pos.ShortExchange, pos.Symbol, tp, pos.ShortEntry)
		}
	}

	// Persist SL and TP order IDs.
	if longSLID != "" || shortSLID != "" || longTPID != "" || shortTPID != "" {
		_ = e.db.UpdatePositionFields(pos.ID, func(fresh *models.ArbitragePosition) bool {
			if longSLID != "" {
				fresh.LongSLOrderID = longSLID
			}
			if shortSLID != "" {
				fresh.ShortSLOrderID = shortSLID
			}
			if longTPID != "" {
				fresh.LongTPOrderID = longTPID
			}
			if shortTPID != "" {
				fresh.ShortTPOrderID = shortTPID
			}
			return true
		})
		pos.LongSLOrderID = longSLID
		pos.ShortSLOrderID = shortSLID
		pos.LongTPOrderID = longTPID
		pos.ShortTPOrderID = shortTPID
	}

	// Register in SL index for instant fill detection.
	e.registerSLOrders(pos)
}

// cancelStopLosses cancels any active stop-loss and take-profit orders on both legs.
// Logs errors but does not block exit.
func (e *Engine) cancelStopLosses(pos *models.ArbitragePosition) {
	// Unregister from SL index first to prevent stale triggers.
	e.unregisterSLOrders(pos)

	if pos.LongSLOrderID != "" {
		if exch, ok := e.exchanges[pos.LongExchange]; ok {
			if err := exch.CancelStopLoss(pos.Symbol, pos.LongSLOrderID); err != nil {
				e.log.Warn("cancel long SL %s on %s: %v", pos.LongSLOrderID, pos.LongExchange, err)
			} else {
				e.log.Info("cancelled long SL %s on %s", pos.LongSLOrderID, pos.LongExchange)
			}
		}
	}
	if pos.ShortSLOrderID != "" {
		if exch, ok := e.exchanges[pos.ShortExchange]; ok {
			if err := exch.CancelStopLoss(pos.Symbol, pos.ShortSLOrderID); err != nil {
				e.log.Warn("cancel short SL %s on %s: %v", pos.ShortSLOrderID, pos.ShortExchange, err)
			} else {
				e.log.Info("cancelled short SL %s on %s", pos.ShortSLOrderID, pos.ShortExchange)
			}
		}
	}

	// Cancel take-profit orders.
	if pos.LongTPOrderID != "" {
		if exch, ok := e.exchanges[pos.LongExchange]; ok {
			if err := exch.CancelTakeProfit(pos.Symbol, pos.LongTPOrderID); err != nil {
				e.log.Warn("cancel long TP %s on %s: %v", pos.LongTPOrderID, pos.LongExchange, err)
			} else {
				e.log.Info("cancelled long TP %s on %s", pos.LongTPOrderID, pos.LongExchange)
			}
		}
	}
	if pos.ShortTPOrderID != "" {
		if exch, ok := e.exchanges[pos.ShortExchange]; ok {
			if err := exch.CancelTakeProfit(pos.Symbol, pos.ShortTPOrderID); err != nil {
				e.log.Warn("cancel short TP %s on %s: %v", pos.ShortTPOrderID, pos.ShortExchange, err)
			} else {
				e.log.Info("cancelled short TP %s on %s", pos.ShortTPOrderID, pos.ShortExchange)
			}
		}
	}
}

// waitForFill polls a single order until it is filled or the deadline passes.
// Used by exit.go for market order fill confirmation.
func (e *Engine) waitForFill(exch exchange.Exchange, orderID, symbol string, deadline time.Time) (float64, error) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if upd, ok := exch.GetOrderUpdate(orderID); ok {
				if upd.Status == "filled" || upd.FilledVolume > 0 {
					return upd.FilledVolume, nil
				}
			}
			// Fallback to REST.
			filled, err := exch.GetOrderFilledQty(orderID, symbol)
			if err == nil && filled > 0 {
				return filled, nil
			}
			if time.Now().After(deadline) {
				return 0, fmt.Errorf("fill timeout for order %s", orderID)
			}
		case <-e.stopCh:
			return 0, fmt.Errorf("engine stopped")
		}
	}
}

// getExchangePositionSize returns the actual position size for a given symbol
// and side from an exchange.
func getExchangePositionSize(exch exchange.Exchange, symbol, side string) (float64, error) {
	positions, err := exch.GetPosition(symbol)
	if err != nil {
		return 0, err
	}
	var total float64
	for _, p := range positions {
		if p.HoldSide == side {
			s, _ := utils.ParseFloat(p.Total)
			total += s
		}
	}
	return total, nil
}

// getUnrealizedPnL returns the combined unrealized PnL for a position's two
// legs by querying each exchange.
func getUnrealizedPnL(longExch, shortExch exchange.Exchange, symbol string) float64 {
	var pnl float64

	longPositions, err := longExch.GetPosition(symbol)
	if err == nil {
		for _, p := range longPositions {
			if p.HoldSide == "long" {
				v, _ := utils.ParseFloat(p.UnrealizedPL)
				pnl += v
			}
		}
	}

	shortPositions, err := shortExch.GetPosition(symbol)
	if err == nil {
		for _, p := range shortPositions {
			if p.HoldSide == "short" {
				v, _ := utils.ParseFloat(p.UnrealizedPL)
				pnl += v
			}
		}
	}

	return pnl
}

// abs returns the absolute value of a float64.
func abs(v float64) float64 {
	return math.Abs(v)
}

// effectiveAdvanceMin scales the order advance window based on the opportunity's
// funding interval. For short intervals (< 8h), it uses 20% of the interval
// to avoid consuming too much of the cycle. Returns cfgAdvance for standard
// or unknown intervals.
func effectiveAdvanceMin(cfgAdvance int, intervalHours float64) int {
	if intervalHours <= 0 || intervalHours >= 8 {
		return cfgAdvance
	}
	scaled := int(intervalHours * 60 * 0.20) // 20% of interval
	if scaled < 3 {
		scaled = 3 // minimum 3 minutes
	}
	if scaled > cfgAdvance {
		return cfgAdvance
	}
	return scaled
}
