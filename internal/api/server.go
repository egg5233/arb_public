package api

import (
	"context"
	"net/http"
	"sync/atomic"
	"time"

	"arb/internal/config"
	"arb/internal/database"
	"arb/internal/models"
	"arb/internal/risk"
	"arb/pkg/exchange"
	"arb/pkg/utils"
)

// Server is the Dashboard HTTP/WebSocket server.
type Server struct {
	db                *database.Client
	cfg               *config.Config
	hub               *Hub
	log               *utils.Logger
	srv               *http.Server
	opps              []models.Opportunity
	auth              *authStore
	exchanges         map[string]exchange.Exchange
	closePosition     func(posID string) error                                           // registered by engine
	openPosition      func(symbol, longExchange, shortExchange string, force bool) error // registered by engine
	logSub            chan utils.LogEntry
	rejStore          *models.RejectionStore
	permissions       map[string]exchange.PermissionResult
	scorer            *risk.ExchangeScorer
	spotOpps          atomic.Value // []interface{}
	spotOpenPosition   func(symbol, exchange, direction string) error
	spotClosePosition  func(positionID string) error
	spotInjectTestOpp  func(symbol, exchange string)
}

// NewServer creates a new Dashboard server.
func NewServer(db *database.Client, cfg *config.Config, exchanges map[string]exchange.Exchange) *Server {
	s := &Server{
		db:        db,
		cfg:       cfg,
		hub:       NewHub(),
		log:       utils.NewLogger("api"),
		auth:      newAuthStore(),
		exchanges: exchanges,
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

	// Spot-futures routes
	mux.HandleFunc("/api/spot/positions", s.cors(s.authMiddleware(s.handleGetSpotPositions)))
	mux.HandleFunc("/api/spot/history", s.cors(s.authMiddleware(s.handleGetSpotHistory)))
	mux.HandleFunc("/api/spot/stats", s.cors(s.authMiddleware(s.handleGetSpotStats)))
	mux.HandleFunc("/api/spot/opportunities", s.cors(s.authMiddleware(s.handleGetSpotOpportunities)))
	mux.HandleFunc("/api/spot/open", s.cors(s.authMiddleware(s.handleSpotManualOpen)))
	mux.HandleFunc("/api/spot/close", s.cors(s.authMiddleware(s.handleSpotManualClose)))
	mux.HandleFunc("GET /api/spot/positions/{id}/health", s.cors(s.authMiddleware(s.handleSpotPositionHealth)))
	mux.HandleFunc("/api/spot/config/auto", s.cors(s.authMiddleware(s.handleSpotAutoConfig)))
	mux.HandleFunc("/api/spot/test-inject", s.cors(s.authMiddleware(s.handleSpotTestInject)))

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
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, OPTIONS")
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
	s.openPosition = fn
}

// SetOpportunities updates the cached opportunities slice for the GET endpoint.
func (s *Server) SetOpportunities(opps []models.Opportunity) {
	s.opps = opps
}

// SetSpotOpenHandler registers the spot-futures engine's manual open callback.
func (s *Server) SetSpotOpenHandler(fn func(symbol, exchange, direction string) error) {
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

// SetPermissions stores the startup permission check results.
func (s *Server) SetPermissions(perms map[string]exchange.PermissionResult) {
	s.permissions = perms
}
