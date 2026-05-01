package api

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"arb/internal/database"
)

const sessionTTL = 7 * 24 * time.Hour

// authStore holds active sessions.
type authStore struct {
	mu       sync.RWMutex
	db       *database.Client
	sessions map[string]time.Time // token -> expiry
	done     chan struct{}
}

func newAuthStore(db *database.Client) *authStore {
	return &authStore{
		db:       db,
		sessions: make(map[string]time.Time),
		done:     make(chan struct{}),
	}
}

func authSessionKey(token string) string {
	return "arb:auth:session:" + token
}

// cleanupLoop periodically removes expired sessions.
func (a *authStore) cleanupLoop() {
	if a.db != nil {
		<-a.done
		return
	}
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			a.purgeExpired()
		case <-a.done:
			return
		}
	}
}

func (a *authStore) stop() {
	select {
	case <-a.done:
	default:
		close(a.done)
	}
}

func (a *authStore) purgeExpired() {
	a.mu.Lock()
	defer a.mu.Unlock()
	now := time.Now()
	for tok, exp := range a.sessions {
		if now.After(exp) {
			delete(a.sessions, tok)
		}
	}
}

// createSession generates a random token and stores it.
func (a *authStore) createSession() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token := hex.EncodeToString(b)
	if a.db != nil {
		if err := a.db.SetWithTTL(authSessionKey(token), "1", sessionTTL); err != nil {
			return "", err
		}
		return token, nil
	}
	a.mu.Lock()
	a.sessions[token] = time.Now().Add(sessionTTL)
	a.mu.Unlock()
	return token, nil
}

// validate checks whether the token exists and is not expired.
func (a *authStore) validate(token string) bool {
	if a.db != nil {
		_, err := a.db.Get(authSessionKey(token))
		return err == nil
	}
	a.mu.RLock()
	exp, ok := a.sessions[token]
	a.mu.RUnlock()
	if !ok {
		return false
	}
	if time.Now().After(exp) {
		a.mu.Lock()
		delete(a.sessions, token)
		a.mu.Unlock()
		return false
	}
	return true
}

// authMiddleware rejects requests that do not carry a valid Bearer token.
// When no dashboard password is configured, auth is bypassed for read-only
// endpoints. Write endpoints (close, open, transfer) always require auth.
func (s *Server) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Skip auth when no password is configured (development/testing mode)
		// but never skip for state-changing endpoints.
		if s.cfg.DashboardPassword == "" && !s.isMutatingEndpoint(r) {
			next(w, r)
			return
		}
		if s.cfg.DashboardPassword == "" {
			writeJSON(w, http.StatusForbidden, Response{Error: "dashboard password must be configured to use this endpoint"})
			return
		}

		header := r.Header.Get("Authorization")
		if header == "" || !strings.HasPrefix(header, "Bearer ") {
			writeJSON(w, http.StatusUnauthorized, Response{Error: "missing or invalid authorization header"})
			return
		}
		token := strings.TrimPrefix(header, "Bearer ")
		if !s.auth.validate(token) {
			writeJSON(w, http.StatusUnauthorized, Response{Error: "invalid or expired token"})
			return
		}
		next(w, r)
	}
}

// isMutatingEndpoint returns true for endpoints that change state (trades, transfers).
func (s *Server) isMutatingEndpoint(r *http.Request) bool {
	if r.Method == http.MethodGet || r.Method == http.MethodOptions {
		return false
	}
	path := r.URL.Path
	return strings.HasPrefix(path, "/api/positions/") ||
		path == "/api/transfer" ||
		path == "/api/spot/open" ||
		strings.HasPrefix(path, "/api/pricegap/candidate/") ||
		path == "/api/pg/breaker/recover" ||
		path == "/api/pg/breaker/test-fire"
}

// loginRequest is the expected body for POST /api/login.
type loginRequest struct {
	Password string `json:"password"`
}

// loginResponse is the response for a successful login.
type loginResponse struct {
	Token string `json:"token"`
}

// handleLogin authenticates the user and returns a session token.
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, Response{Error: "invalid request body"})
		return
	}

	expected := s.cfg.DashboardPassword
	if expected == "" {
		// No password configured — issue a token for any request
		token, err := s.auth.createSession()
		if err != nil {
			s.log.Error("failed to create session: %v", err)
			writeJSON(w, http.StatusInternalServerError, Response{Error: "internal error"})
			return
		}
		writeJSON(w, http.StatusOK, Response{OK: true, Data: loginResponse{Token: token}})
		return
	}

	if subtle.ConstantTimeCompare([]byte(req.Password), []byte(expected)) != 1 {
		writeJSON(w, http.StatusUnauthorized, Response{Error: "invalid password"})
		return
	}

	token, err := s.auth.createSession()
	if err != nil {
		s.log.Error("failed to create session: %v", err)
		writeJSON(w, http.StatusInternalServerError, Response{Error: "internal error"})
		return
	}

	writeJSON(w, http.StatusOK, Response{OK: true, Data: loginResponse{Token: token}})
}
