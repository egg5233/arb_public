package spotengine

import (
	"sync"
	"time"

	"arb/internal/api"
	"arb/internal/config"
	"arb/internal/database"
	"arb/internal/models"
	"arb/internal/notify"
	"arb/internal/risk"
	"arb/pkg/exchange"
	"arb/pkg/utils"
)

// SpotEngine orchestrates spot-futures arbitrage: discovery, entry, monitoring
// and exit of delta-neutral positions using spot margin + futures hedging.
type SpotEngine struct {
	exchanges  map[string]exchange.Exchange
	spotMargin map[string]exchange.SpotMarginExchange
	db         *database.Client
	api        *api.Server
	cfg        *config.Config
	allocator  *risk.CapitalAllocator
	log        *utils.Logger
	stopCh     chan struct{}
	wg         sync.WaitGroup
	exitWG     sync.WaitGroup

	// latestOpps caches the most recent discovery scan results for ManualOpen lookups.
	oppsMu     sync.RWMutex
	latestOpps []SpotArbOpportunity

	// exitMu protects exitState from concurrent access.
	exitMu    sync.Mutex
	exitState exitState

	// lastSeen tracks which symbols were present in the previous scan
	// so we can delete Redis persistence counters for symbols that disappeared.
	lastSeenMu sync.RWMutex
	lastSeen   map[string]bool

	// posMu provides per-position locking for read-modify-write operations.
	posMu sync.Map // posID → *sync.Mutex

	// borrowVelocity tracks recent borrow APR samples per position for spike detection.
	borrowVelocity *RateVelocityDetector

	// telegram sends trade lifecycle alerts. Nil if unconfigured.
	telegram *notify.TelegramNotifier
}

// NewSpotEngine creates a new SpotEngine with all required dependencies.
// It extracts SpotMarginExchange implementations from the exchanges map.
func NewSpotEngine(
	exchanges map[string]exchange.Exchange,
	db *database.Client,
	apiSrv *api.Server,
	cfg *config.Config,
	allocator *risk.CapitalAllocator,
) *SpotEngine {
	sm := make(map[string]exchange.SpotMarginExchange)
	for name, exc := range exchanges {
		if m, ok := exc.(exchange.SpotMarginExchange); ok {
			sm[name] = m
		}
	}

	return &SpotEngine{
		exchanges:      exchanges,
		spotMargin:     sm,
		db:             db,
		api:            apiSrv,
		cfg:            cfg,
		allocator:      allocator,
		log:            utils.NewLogger("spot-engine"),
		stopCh:         make(chan struct{}),
		exitState:      exitState{exiting: make(map[string]bool)},
		lastSeen:       make(map[string]bool),
		borrowVelocity: NewRateVelocityDetector(),
		telegram:       notify.NewTelegram(cfg.TelegramBotToken, cfg.TelegramChatID),
	}
}

// Start launches the spot-futures engine goroutines.
func (e *SpotEngine) Start() {
	e.log.Info("Spot-futures engine starting (monitor interval: %ds, max positions: %d)",
		e.cfg.SpotFuturesMonitorIntervalSec, e.cfg.SpotFuturesMaxPositions)
	e.log.Info("Spot margin exchanges available: %d", len(e.spotMargin))

	e.wg.Add(2)
	go e.discoveryLoop()
	go e.monitorLoop()
}

// Stop signals all goroutines to exit and waits for them to finish.
func (e *SpotEngine) Stop() {
	e.log.Info("Spot-futures engine stopping...")
	close(e.stopCh)
	e.wg.Wait()
	e.exitWG.Wait()
	e.log.Info("Spot-futures engine stopped")
}

// launchExit runs an automated exit in the background and tracks it so Stop
// waits for in-flight close sequences to finish before returning.
func (e *SpotEngine) launchExit(pos *models.SpotFuturesPosition, reason string, isEmergency bool) {
	e.exitWG.Add(1)
	go func() {
		defer e.exitWG.Done()
		e.initiateExit(pos, reason, isEmergency)
	}()
}

// discoveryLoop periodically scans for spot-futures arbitrage opportunities.
func (e *SpotEngine) discoveryLoop() {
	defer e.wg.Done()

	scanInterval := time.Duration(e.cfg.SpotFuturesScanIntervalMin) * time.Minute
	if scanInterval < time.Minute {
		scanInterval = 10 * time.Minute
	}
	ticker := time.NewTicker(scanInterval)
	defer ticker.Stop()

	// Seed lastSeen from Redis so the first scan correctly cleans up
	// persistence counters for symbols that disappeared during downtime.
	if syms, err := e.db.ListSpotPersistenceSymbols(); err != nil {
		e.log.Error("failed to seed lastSeen from Redis: %v", err)
	} else if len(syms) > 0 {
		e.lastSeenMu.Lock()
		for _, s := range syms {
			e.lastSeen[s] = true
		}
		e.lastSeenMu.Unlock()
		e.log.Info("seeded lastSeen with %d symbols from Redis persistence keys", len(syms))
	}

	// Initial scan on startup.
	e.log.Info("spot-futures discovery scan (interval: %s)", scanInterval)
	opps := e.runDiscoveryScan()
	passed := filterPassed(opps)
	e.logDiscoveryResults(passed)
	e.pushOppsToAPI(opps)
	e.updatePersistenceCounts(passed)
	e.attemptAutoEntries(passed)

	for {
		select {
		case <-e.stopCh:
			return
		case <-ticker.C:
			e.log.Info("spot-futures discovery scan")
			opps := e.runDiscoveryScan()
			passed := filterPassed(opps)
			e.logDiscoveryResults(passed)
			e.pushOppsToAPI(opps)
			e.updatePersistenceCounts(passed)
			e.attemptAutoEntries(passed)
		}
	}
}

// pushOppsToAPI sends discovery results to the API server for /api/spot/opportunities
// and caches them locally for ManualOpen lookups.
func (e *SpotEngine) pushOppsToAPI(opps []SpotArbOpportunity) {
	// Cache locally for ManualOpen.
	e.oppsMu.Lock()
	e.latestOpps = opps
	e.oppsMu.Unlock()

	items := make([]interface{}, len(opps))
	for i, o := range opps {
		items[i] = o
	}
	e.api.SetSpotOpportunities(items)
	e.api.BroadcastSpotOpportunities(items)
}

// filterPassed returns only opportunities that passed all entry filters.
func filterPassed(opps []SpotArbOpportunity) []SpotArbOpportunity {
	var out []SpotArbOpportunity
	for _, o := range opps {
		if o.FilterStatus == "" {
			out = append(out, o)
		}
	}
	return out
}

// getLatestOpps returns a copy of the latest discovery scan results (thread-safe).
func (e *SpotEngine) getLatestOpps() []SpotArbOpportunity {
	e.oppsMu.RLock()
	defer e.oppsMu.RUnlock()
	out := make([]SpotArbOpportunity, len(e.latestOpps))
	copy(out, e.latestOpps)
	return out
}

// isExiting returns true if the given position is currently being exited.
func (e *SpotEngine) isExiting(posID string) bool {
	e.exitMu.Lock()
	defer e.exitMu.Unlock()
	return e.exitState.exiting[posID]
}

// updatePersistenceCounts increments Redis persistence counters for symbols
// present in the latest scan and deletes counters for symbols that disappeared.
func (e *SpotEngine) updatePersistenceCounts(opps []SpotArbOpportunity) {
	seen := make(map[string]bool, len(opps))
	for _, opp := range opps {
		key := opp.Symbol
		if seen[key] {
			continue // already incremented this symbol this scan
		}
		seen[key] = true
		if _, err := e.db.IncrSpotPersistence(opp.Symbol); err != nil {
			e.log.Error("persist incr %s: %v", key, err)
		}
	}

	// Delete counters for symbols that disappeared since last scan.
	e.lastSeenMu.Lock()
	prevSeen := e.lastSeen
	e.lastSeen = seen
	e.lastSeenMu.Unlock()

	for key := range prevSeen {
		if !seen[key] {
			if err := e.db.DeleteSpotPersistence(key); err != nil {
				e.log.Error("persist del %s: %v", key, err)
			}
		}
	}
}

// getPersistenceCount returns how many consecutive scans a symbol has appeared in.
// Returns 0 on Redis error (fail-closed: denies entry if Redis is down).
func (e *SpotEngine) getPersistenceCount(symbol string) int {
	count, err := e.db.GetSpotPersistence(symbol)
	if err != nil {
		e.log.Error("persist get %s: %v", symbol, err)
		return 0
	}
	return count
}

// posLock returns a per-position mutex, creating one if needed.
func (e *SpotEngine) posLock(posID string) *sync.Mutex {
	v, _ := e.posMu.LoadOrStore(posID, &sync.Mutex{})
	return v.(*sync.Mutex)
}

// isSeparateAccount returns true for exchanges with separate spot and futures wallets.
func isSeparateAccount(exchName string) bool {
	switch exchName {
	case "binance", "bitget":
		return true
	default:
		return false
	}
}

// capitalForExchange returns the position capital limit appropriate for the
// exchange's account type. Separate-account exchanges (Binance, Bitget) use a
// lower default since cross-margin collateral is not shared.
func (e *SpotEngine) capitalForExchange(exchName string) float64 {
	if isSeparateAccount(exchName) {
		cap := e.cfg.SpotFuturesSeparateAcctMaxUSDT
		if cap <= 0 {
			cap = 200
		}
		return cap
	}
	cap := e.cfg.SpotFuturesUnifiedAcctMaxUSDT
	if cap <= 0 {
		cap = 500
	}
	return cap
}

// findActivePosition returns the most recently created active position for a
// symbol+exchange pair, or nil if none exists.
func (e *SpotEngine) findActivePosition(symbol, exchName string) *models.SpotFuturesPosition {
	positions, err := e.db.GetActiveSpotPositions()
	if err != nil {
		return nil
	}
	for _, p := range positions {
		if p.Symbol == symbol && p.Exchange == exchName && p.Status == models.SpotStatusActive {
			return p
		}
	}
	return nil
}

// lockedUpdatePosition wraps db.UpdateSpotPositionFields with per-position
// locking, preventing lost-update races between concurrent callers (monitor
// tick and exit goroutine) that operate on the same position.
func (e *SpotEngine) lockedUpdatePosition(posID string, mutate func(pos *models.SpotFuturesPosition) bool) error {
	mu := e.posLock(posID)
	mu.Lock()
	defer mu.Unlock()
	return e.db.UpdateSpotPositionFields(posID, mutate)
}
