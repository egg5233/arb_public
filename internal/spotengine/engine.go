package spotengine

import (
	"sync"
	"time"

	"arb/internal/api"
	"arb/internal/config"
	"arb/internal/database"
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
	log        *utils.Logger
	stopCh     chan struct{}
	wg         sync.WaitGroup

	// latestOpps caches the most recent discovery scan results for ManualOpen lookups.
	oppsMu     sync.RWMutex
	latestOpps []SpotArbOpportunity
}

// NewSpotEngine creates a new SpotEngine with all required dependencies.
// It extracts SpotMarginExchange implementations from the exchanges map.
func NewSpotEngine(
	exchanges map[string]exchange.Exchange,
	db *database.Client,
	apiSrv *api.Server,
	cfg *config.Config,
) *SpotEngine {
	sm := make(map[string]exchange.SpotMarginExchange)
	for name, exc := range exchanges {
		if m, ok := exc.(exchange.SpotMarginExchange); ok {
			sm[name] = m
		}
	}

	return &SpotEngine{
		exchanges:  exchanges,
		spotMargin: sm,
		db:         db,
		api:        apiSrv,
		cfg:        cfg,
		log:        utils.NewLogger("spot-engine"),
		stopCh:     make(chan struct{}),
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
	e.log.Info("Spot-futures engine stopped")
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

	// Initial scan on startup.
	e.log.Info("spot-futures discovery scan (interval: %s)", scanInterval)
	opps := e.runDiscoveryScan()
	e.logDiscoveryResults(opps)
	e.pushOppsToAPI(opps)

	for {
		select {
		case <-e.stopCh:
			return
		case <-ticker.C:
			e.log.Info("spot-futures discovery scan")
			opps := e.runDiscoveryScan()
			e.logDiscoveryResults(opps)
			e.pushOppsToAPI(opps)
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
}

// getLatestOpps returns a copy of the latest discovery scan results (thread-safe).
func (e *SpotEngine) getLatestOpps() []SpotArbOpportunity {
	e.oppsMu.RLock()
	defer e.oppsMu.RUnlock()
	out := make([]SpotArbOpportunity, len(e.latestOpps))
	copy(out, e.latestOpps)
	return out
}
