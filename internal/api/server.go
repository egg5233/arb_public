package api

import (
	"context"
	"net/http"
	"sync/atomic"
	"time"

	"arb/internal/analytics"
	"arb/internal/config"
	"arb/internal/database"
	"arb/internal/models"
	"arb/internal/risk"
	"arb/pkg/exchange"
	"arb/pkg/utils"
)

// CandidateRegistry is the chokepoint contract Server holds onto for
// cfg.PriceGapCandidates mutations (Plan 11-03 / PG-DISC-04). Production wires
// this to *pricegaptrader.Registry; tests inject a fake that captures Replace
// calls. Methods mirror the four mutators on *pricegaptrader.Registry plus
// the RegistryReader pair so handlers can read snapshot state without
// touching cfg directly.
//
// Defined in the api package (rather than imported from pricegaptrader)
// because tests in this package need a fake double — moving the interface
// here keeps that fake at api-package scope without an extra import-cycle.
type CandidateRegistry interface {
	Add(ctx context.Context, source string, c models.PriceGapCandidate) error
	Update(ctx context.Context, source string, idx int, c models.PriceGapCandidate) error
	Delete(ctx context.Context, source string, idx int) error
	Replace(ctx context.Context, source string, next []models.PriceGapCandidate) error
	Get(idx int) (models.PriceGapCandidate, bool)
	List() []models.PriceGapCandidate
}

// Server is the Dashboard HTTP/WebSocket server.
type Server struct {
	db                     *database.Client
	cfg                    *config.Config
	hub                    *Hub
	log                    *utils.Logger
	srv                    *http.Server
	opps                   []models.Opportunity
	auth                   *authStore
	exchanges              map[string]exchange.Exchange
	closePosition          func(posID string) error                                                       // registered by engine
	openPosition           func(symbol, longExchange, shortExchange string, opts ManualOpenOptions) error // registered by engine
	logSub                 chan utils.LogEntry
	rejStore               *models.RejectionStore
	permissions            map[string]exchange.PermissionResult
	scorer                 *risk.ExchangeScorer
	spotOpps               atomic.Value // []interface{}
	spotOpenPosition       func(symbol, exchange, direction string, opts ManualOpenOptions) error
	spotClosePosition      func(positionID string) error
	spotInjectTestOpp      func(symbol, exchange string)
	spotMaintenanceWarning func(symbol, exchange string) string
	spotRunBacktest        func(ctx context.Context, symbol, exchange, direction string, days int) (interface{}, error)
	configNotifier         *config.ConfigNotifier
	analyticsStore         *analytics.Store
	allocator              *risk.CapitalAllocator
	// registry is the chokepoint for cfg.PriceGapCandidates mutations
	// (Plan 11-03 / PG-DISC-04). Wired in cmd/main.go via SetRegistry from
	// the single *pricegaptrader.Registry instance shared with the tracker.
	// Tests use the fake CandidateRegistry to capture Replace calls.
	registry CandidateRegistry
}

// ManualOpenOptions carries dashboard-only manual entry overrides.
type ManualOpenOptions struct {
	Force bool
}

// NewServer creates a new Dashboard server.
func NewServer(db *database.Client, cfg *config.Config, exchanges map[string]exchange.Exchange) *Server {
	s := &Server{
		db:             db,
		cfg:            cfg,
		hub:            NewHub(),
		log:            utils.NewLogger("api"),
		auth:           newAuthStore(db),
		exchanges:      exchanges,
		configNotifier: config.NewConfigNotifier(),
	}
	return s
}

// Start begins serving HTTP requests and the WebSocket hub. It blocks until
// the server is stopped or encounters a fatal error.
func (s *Server) Start() {
	go s.hub.Run()
	go s.auth.cleanupLoop()
	s.startBinaryDriftMonitor()

	// Subscribe to log stream and broadcast to WS clients.
	s.logSub = utils.Subscribe()
	go func() {
		for entry := range s.logSub {
			s.hub.Broadcast("log", entry)
		}
	}()

	mux := http.NewServeMux()

	// Auth
	mux.HandleFunc("/api/login", s.cors(s.handleLogin))

	// Protected API routes
	mux.HandleFunc("/api/positions", s.cors(s.authMiddleware(s.handleGetPositions)))
	mux.HandleFunc("/api/history", s.cors(s.authMiddleware(s.handleGetHistory)))
	mux.HandleFunc("/api/opportunities", s.cors(s.authMiddleware(s.handleGetOpportunities)))
	mux.HandleFunc("/api/stats", s.cors(s.authMiddleware(s.handleGetStats)))
	mux.HandleFunc("/api/config", s.cors(s.authMiddleware(s.handleConfig)))
	mux.HandleFunc("/api/exchanges", s.cors(s.authMiddleware(s.handleGetExchanges)))
	mux.HandleFunc("GET /api/exchanges/health", s.cors(s.authMiddleware(s.handleGetExchangeHealth)))

	// Position actions
	mux.HandleFunc("/api/positions/close", s.cors(s.authMiddleware(s.handleClosePosition)))
	mux.HandleFunc("/api/positions/open", s.cors(s.authMiddleware(s.handleOpenPosition)))
	mux.HandleFunc("GET /api/positions/{id}/funding", s.cors(s.authMiddleware(s.handleGetPositionFunding)))

	// Blacklist
	mux.HandleFunc("/api/blacklist", s.cors(s.authMiddleware(s.handleBlacklist)))

	// Transfer routes
	mux.HandleFunc("/api/transfer", s.cors(s.authMiddleware(s.handleTransfer)))
	mux.HandleFunc("/api/transfers", s.cors(s.authMiddleware(s.handleGetTransfers)))
	mux.HandleFunc("/api/addresses", s.cors(s.authMiddleware(s.handleAddresses)))

	// Logs
	mux.HandleFunc("/api/logs", s.cors(s.authMiddleware(s.handleGetLogs)))

	// Rejections
	mux.HandleFunc("/api/rejections", s.cors(s.authMiddleware(s.handleGetRejections)))

	// AI Diagnose
	mux.HandleFunc("/api/diagnose", s.cors(s.authMiddleware(s.handleDiagnose)))

	// Permissions
	mux.HandleFunc("/api/permissions", s.cors(s.authMiddleware(s.handleGetPermissions)))

	// TradFi-Perps agreement
	mux.HandleFunc("/api/tradfi-status", s.cors(s.authMiddleware(s.handleTradFiStatus)))
	mux.HandleFunc("/api/sign-tradfi", s.cors(s.authMiddleware(s.handleSignTradFi)))

	// Spot-futures routes
	mux.HandleFunc("/api/spot/positions", s.cors(s.authMiddleware(s.handleGetSpotPositions)))
	mux.HandleFunc("/api/spot/history", s.cors(s.authMiddleware(s.handleGetSpotHistory)))
	mux.HandleFunc("/api/spot/stats", s.cors(s.authMiddleware(s.handleGetSpotStats)))
	mux.HandleFunc("/api/spot/opportunities", s.cors(s.authMiddleware(s.handleGetSpotOpportunities)))
	mux.HandleFunc("/api/spot/open", s.cors(s.authMiddleware(s.handleSpotManualOpen)))
	mux.HandleFunc("/api/spot/check-price-gap", s.cors(s.authMiddleware(s.handleSpotCheckPriceGap)))
	mux.HandleFunc("/api/spot/batch-check-gap", s.cors(s.authMiddleware(s.handleSpotBatchCheckGap)))
	mux.HandleFunc("/api/spot/batch-check-borrowable", s.cors(s.authMiddleware(s.handleSpotBatchCheckBorrowable)))
	mux.HandleFunc("/api/spot/close", s.cors(s.authMiddleware(s.handleSpotManualClose)))
	mux.HandleFunc("GET /api/spot/positions/{id}/health", s.cors(s.authMiddleware(s.handleSpotPositionHealth)))
	mux.HandleFunc("/api/spot/config/auto", s.cors(s.authMiddleware(s.handleSpotAutoConfig)))
	mux.HandleFunc("/api/spot/test-inject", s.cors(s.authMiddleware(s.handleSpotTestInject)))
	mux.HandleFunc("/api/spot/test-lifecycle", s.cors(s.authMiddleware(s.handleSpotTestLifecycle)))
	mux.HandleFunc("POST /api/spot/backtest", s.cors(s.authMiddleware(s.handleSpotBacktest)))

	// Analytics
	mux.HandleFunc("/api/analytics/pnl-history", s.cors(s.authMiddleware(s.handleGetAnalyticsPnLHistory)))
	mux.HandleFunc("/api/analytics/summary", s.cors(s.authMiddleware(s.handleGetAnalyticsSummary)))

	// Price-gap tracker (Phase 9 dashboard)
	mux.HandleFunc("GET /api/pricegap/state", s.cors(s.authMiddleware(s.handlePriceGapState)))
	mux.HandleFunc("GET /api/pricegap/candidates", s.cors(s.authMiddleware(s.handlePriceGapCandidates)))
	mux.HandleFunc("GET /api/pricegap/positions", s.cors(s.authMiddleware(s.handlePriceGapPositions)))
	mux.HandleFunc("GET /api/pricegap/closed", s.cors(s.authMiddleware(s.handlePriceGapClosed)))
	mux.HandleFunc("GET /api/pricegap/metrics", s.cors(s.authMiddleware(s.handlePriceGapMetrics)))
	mux.HandleFunc("POST /api/pricegap/candidate/{symbol}/disable", s.cors(s.authMiddleware(s.handlePriceGapCandidateDisable)))
	mux.HandleFunc("POST /api/pricegap/candidate/{symbol}/enable", s.cors(s.authMiddleware(s.handlePriceGapCandidateEnable)))

	// Capital allocation
	mux.HandleFunc("/api/allocation", s.cors(s.authMiddleware(s.handleGetAllocation)))

	// System update
	mux.HandleFunc("/api/check-update", s.cors(s.authMiddleware(s.handleCheckUpdate)))
	mux.HandleFunc("/api/update", s.cors(s.authMiddleware(s.handleUpdate)))

	// WebSocket
	mux.HandleFunc("/ws", s.hub.HandleWS)

	// Embedded React frontend (SPA fallback for all non-API paths)
	mux.Handle("/", frontendHandler())

	addr := s.cfg.DashboardAddr
	if addr == "" {
		addr = ":8080"
	}

	s.srv = &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 180 * time.Second, // AI diagnose needs up to 120s
		IdleTimeout:  60 * time.Second,
	}

	s.log.Info("dashboard listening on %s", addr)
	if err := s.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		s.log.Error("server error: %v", err)
	}
}

// Stop gracefully shuts down the server.
func (s *Server) Stop() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s.auth.stop()

	if s.logSub != nil {
		utils.Unsubscribe(s.logSub)
	}

	if s.srv != nil {
		if err := s.srv.Shutdown(ctx); err != nil {
			s.log.Error("shutdown error: %v", err)
		}
	}
	s.log.Info("dashboard stopped")
}

// cors wraps a handler to add CORS headers for local development.
func (s *Server) cors(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next(w, r)
	}
}

// handleConfig routes GET and POST/PUT to the appropriate handler.
func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleGetConfig(w, r)
	case http.MethodPost, http.MethodPut:
		s.handlePostConfig(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// BroadcastPositionUpdate sends a position update to all WebSocket clients.
// It broadcasts the full active positions list so the frontend can replace state.
func (s *Server) BroadcastPositionUpdate(pos *models.ArbitragePosition) {
	// Send individual update for fine-grained handling.
	s.hub.Broadcast("position_update", pos)

	// Also send the full active positions list so the frontend can replace state.
	positions, err := s.db.GetActivePositions()
	if err == nil {
		s.hub.Broadcast("positions", positions)
	}
}

// BroadcastOpportunities sends opportunity data to all WebSocket clients.
func (s *Server) BroadcastOpportunities(opps []models.Opportunity) {
	s.hub.Broadcast("opportunities", opps)
}

// BroadcastStats sends stats data to all WebSocket clients.
func (s *Server) BroadcastStats() {
	stats, err := s.db.GetStats()
	if err == nil {
		s.hub.Broadcast("stats", stats)
	}
}

// BroadcastAlert sends an alert to all WebSocket clients.
func (s *Server) BroadcastAlert(alert interface{}) {
	s.hub.Broadcast("alert", alert)
}

// BroadcastLossLimits sends loss limit status to all WebSocket clients.
func (s *Server) BroadcastLossLimits(status interface{}) {
	s.hub.Broadcast("loss_limits", status)
}

func (s *Server) SetExchangeScorer(scorer *risk.ExchangeScorer) {
	s.scorer = scorer
}

// BroadcastRejection sends a rejected opportunity to all WebSocket clients.
func (s *Server) BroadcastRejection(r models.RejectedOpportunity) {
	s.hub.Broadcast("rejection", r)
}

// SetRejectionStore registers the shared rejection store for the GET endpoint.
func (s *Server) SetRejectionStore(store *models.RejectionStore) {
	s.rejStore = store
}

// SetCloseHandler registers the engine's manual close callback.
func (s *Server) SetCloseHandler(fn func(posID string) error) {
	s.closePosition = fn
}

// SetOpenHandler registers the engine's manual open callback.
func (s *Server) SetOpenHandler(fn func(symbol, longExchange, shortExchange string, force bool) error) {
	if fn == nil {
		s.openPosition = nil
		return
	}
	s.openPosition = func(symbol, longExchange, shortExchange string, opts ManualOpenOptions) error {
		return fn(symbol, longExchange, shortExchange, opts.Force)
	}
}

// SetOpenHandlerWithOptions registers a manual open callback that accepts the
// full API options payload.
func (s *Server) SetOpenHandlerWithOptions(fn func(symbol, longExchange, shortExchange string, opts ManualOpenOptions) error) {
	s.openPosition = fn
}

// SetOpportunities updates the cached opportunities slice for the GET endpoint.
func (s *Server) SetOpportunities(opps []models.Opportunity) {
	s.opps = opps
}

// SetSpotOpenHandler registers the spot-futures engine's manual open callback.
func (s *Server) SetSpotOpenHandler(fn func(symbol, exchange, direction string) error) {
	if fn == nil {
		s.spotOpenPosition = nil
		return
	}
	s.spotOpenPosition = func(symbol, exchange, direction string, opts ManualOpenOptions) error {
		return fn(symbol, exchange, direction)
	}
}

// SetSpotOpenHandlerWithOptions registers a spot-futures manual open callback
// that accepts the full API options payload.
func (s *Server) SetSpotOpenHandlerWithOptions(fn func(symbol, exchange, direction string, opts ManualOpenOptions) error) {
	s.spotOpenPosition = fn
}

// SetSpotCloseHandler registers the spot-futures engine's manual close callback.
func (s *Server) SetSpotCloseHandler(fn func(positionID string) error) {
	s.spotClosePosition = fn
}

// SetSpotTestInjectHandler registers the spot-futures engine's test inject callback.
func (s *Server) SetSpotTestInjectHandler(fn func(symbol, exchange string)) {
	s.spotInjectTestOpp = fn
}

// SetSpotMaintenanceWarning registers the callback that checks whether a
// symbol has an elevated maintenance rate. Returns a warning string or "".
func (s *Server) SetSpotMaintenanceWarning(fn func(symbol, exchange string) string) {
	s.spotMaintenanceWarning = fn
}

// SetSpotBacktestHandler registers the spot engine's on-demand backtest callback.
func (s *Server) SetSpotBacktestHandler(fn func(ctx context.Context, symbol, exchange, direction string, days int) (interface{}, error)) {
	s.spotRunBacktest = fn
}

// SetPermissions stores the startup permission check results.
func (s *Server) SetPermissions(perms map[string]exchange.PermissionResult) {
	s.permissions = perms
}

// SetAnalyticsStore injects the analytics SQLite store for dashboard API endpoints.
func (s *Server) SetAnalyticsStore(store *analytics.Store) {
	s.analyticsStore = store
}

// SetCapitalAllocator injects the capital allocator for the allocation API endpoint.
func (s *Server) SetCapitalAllocator(allocator *risk.CapitalAllocator) {
	s.allocator = allocator
}

// SetRegistry injects the *pricegaptrader.Registry chokepoint that owns all
// cfg.PriceGapCandidates mutations (Plan 11-03 / PG-DISC-04). cmd/main.go
// constructs exactly one Registry instance and shares it with the tracker;
// the api server forwards POST /api/config candidate updates here via
// CandidateRegistry.Replace with source="dashboard-handler".
func (s *Server) SetRegistry(reg CandidateRegistry) {
	s.registry = reg
}

// ConfigNotifier returns the shared ConfigNotifier so that other
// components (engine, scanner, etc.) can subscribe to config changes.
func (s *Server) ConfigNotifier() *config.ConfigNotifier {
	return s.configNotifier
}
