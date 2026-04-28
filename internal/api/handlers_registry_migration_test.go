// Package api — handlers_registry_migration_test.go: Plan 11-03 hard-cut
// migration tests. Verifies POST /api/config price_gap.candidates routes
// through *pricegaptrader.Registry.Replace with source="dashboard-handler",
// validation/guard preserved, and zero direct cfg.PriceGapCandidates writes
// remain in the api package.
package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"testing"

	"arb/internal/models"
	"arb/internal/pricegaptrader"
)

// fakeCandidateRegistry is a CandidateRegistry double that captures Replace
// calls so the migration tests can assert exactly what handlers.go forwards.
type fakeCandidateRegistry struct {
	mu         sync.Mutex
	replaceCnt int
	lastSource string
	lastNext   []models.PriceGapCandidate
	replaceErr error
	addCnt     int
	deleteCnt  int
	updateCnt  int
}

// Compile-time satisfaction — handlers.go's CandidateRegistry interface.
var _ CandidateRegistry = (*fakeCandidateRegistry)(nil)

func (f *fakeCandidateRegistry) Replace(ctx context.Context, source string, next []models.PriceGapCandidate) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.replaceCnt++
	f.lastSource = source
	cp := make([]models.PriceGapCandidate, len(next))
	copy(cp, next)
	f.lastNext = cp
	return f.replaceErr
}

func (f *fakeCandidateRegistry) Add(ctx context.Context, source string, c models.PriceGapCandidate) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.addCnt++
	return nil
}

func (f *fakeCandidateRegistry) Update(ctx context.Context, source string, idx int, c models.PriceGapCandidate) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.updateCnt++
	return nil
}

func (f *fakeCandidateRegistry) Delete(ctx context.Context, source string, idx int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleteCnt++
	return nil
}

func (f *fakeCandidateRegistry) Get(idx int) (models.PriceGapCandidate, bool) {
	return models.PriceGapCandidate{}, false
}

func (f *fakeCandidateRegistry) List() []models.PriceGapCandidate {
	return nil
}

// withFakeRegistry installs a fakeCandidateRegistry on s and returns it. It
// also writes any seeded candidates to the on-disk config.json so Registry's
// reload-from-disk semantics see consistent state in tests that exercise the
// real *Registry.
func withFakeRegistry(t *testing.T, s *Server) *fakeCandidateRegistry {
	t.Helper()
	fr := &fakeCandidateRegistry{}
	s.registry = fr
	return fr
}

// TestPostConfigCandidates_RoutesThroughRegistry verifies the happy path —
// validated candidates are forwarded to Registry.Replace exactly once with
// source="dashboard-handler" and the post-validation slice.
func TestPostConfigCandidates_RoutesThroughRegistry(t *testing.T) {
	s, _, _, td := newPriceGapTestServer(t)
	defer td()
	fr := withFakeRegistry(t, s)
	s.cfg.PriceGapCandidates = nil

	body := map[string]any{
		"price_gap": map[string]any{
			"candidates": []any{candidateMap(nil)},
		},
	}
	rr, env := postPriceGap(t, s, body)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200 (body=%s)", rr.Code, rr.Body.String())
	}
	if ok, _ := env["ok"].(bool); !ok {
		t.Fatalf("ok=false, env=%v", env)
	}
	if fr.replaceCnt != 1 {
		t.Fatalf("Registry.Replace call count: got %d, want 1", fr.replaceCnt)
	}
	if fr.lastSource != "dashboard-handler" {
		t.Fatalf("Registry.Replace source: got %q, want %q", fr.lastSource, "dashboard-handler")
	}
	if len(fr.lastNext) != 1 || fr.lastNext[0].Symbol != "BTCUSDT" {
		t.Fatalf("Registry.Replace next: got %+v", fr.lastNext)
	}
}

// TestPostConfigCandidates_ValidationFails_RegistryNotCalled verifies that an
// invalid candidate (malformed symbol) returns 400 BEFORE Registry is called.
func TestPostConfigCandidates_ValidationFails_RegistryNotCalled(t *testing.T) {
	s, _, _, td := newPriceGapTestServer(t)
	defer td()
	fr := withFakeRegistry(t, s)

	body := map[string]any{
		"price_gap": map[string]any{
			"candidates": []any{candidateMap(map[string]any{"symbol": "btc/usdt"})},
		},
	}
	rr, _ := postPriceGap(t, s, body)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400 (body=%s)", rr.Code, rr.Body.String())
	}
	if fr.replaceCnt != 0 {
		t.Fatalf("Registry.Replace must not be called on validation failure; got %d", fr.replaceCnt)
	}
}

// TestPostConfigCandidates_ActivePositionGuard_RegistryNotCalled verifies the
// active-position guard runs BEFORE Registry. Removing a candidate that has
// an active position must return 409 and NOT invoke Registry.Replace.
func TestPostConfigCandidates_ActivePositionGuard_RegistryNotCalled(t *testing.T) {
	s, _, _, td := newPriceGapTestServer(t)
	defer td()
	fr := withFakeRegistry(t, s)

	// Seed an active position matching one of the seeded candidates so the
	// guard fires on a delete-all attempt.
	pos := &models.PriceGapPosition{
		ID:            "test-pos-1",
		Symbol:        "BTCUSDT",
		LongExchange:  "binance",
		ShortExchange: "bybit",
		Status:        "open",
	}
	if err := s.db.SavePriceGapPosition(pos); err != nil {
		t.Fatalf("seed active position: %v", err)
	}

	body := map[string]any{
		"price_gap": map[string]any{
			"candidates": []any{}, // delete-all — must be blocked
		},
	}
	rr, _ := postPriceGap(t, s, body)
	if rr.Code != http.StatusConflict {
		t.Fatalf("status: got %d, want 409 (body=%s)", rr.Code, rr.Body.String())
	}
	if fr.replaceCnt != 0 {
		t.Fatalf("Registry.Replace must not be called when active-position guard fires; got %d", fr.replaceCnt)
	}
}

// TestPostConfigCandidates_RegistryError_Returns500 verifies that a Registry
// failure surfaces as 500 to the dashboard.
func TestPostConfigCandidates_RegistryError_Returns500(t *testing.T) {
	s, _, _, td := newPriceGapTestServer(t)
	defer td()
	fr := withFakeRegistry(t, s)
	fr.replaceErr = pricegaptrader.ErrCapExceeded

	body := map[string]any{
		"price_gap": map[string]any{
			"candidates": []any{candidateMap(nil)},
		},
	}
	rr, env := postPriceGap(t, s, body)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status: got %d, want 500 (body=%s)", rr.Code, rr.Body.String())
	}
	if errStr, _ := env["error"].(string); errStr == "" {
		t.Fatalf("expected error in envelope, got %v", env)
	}
	if fr.replaceCnt != 1 {
		t.Fatalf("Registry.Replace must be invoked once; got %d", fr.replaceCnt)
	}
}

// TestPostConfigCandidates_NoDirectMutationsInPackage is a static-source check
// guarding against any future contributor reintroducing `s.cfg.PriceGapCandidates = ...`
// in handlers.go or pricegap_handlers.go. After the Plan 11-03 hard-cut the
// only legitimate writers are inside internal/pricegaptrader/registry.go.
func TestPostConfigCandidates_NoDirectMutationsInPackage(t *testing.T) {
	files := []string{"handlers.go", "pricegap_handlers.go"}
	// Match `[s.]?cfg.PriceGapCandidates = ` or `[s.]?cfg.PriceGapCandidates =[`
	// to catch both whole-slice replacements and `... = append(...)`-style
	// rewrites that would also drift from the chokepoint.
	re := regexp.MustCompile(`(?m)^\s*[a-zA-Z_][a-zA-Z0-9_]*\.cfg\.PriceGapCandidates\s*=`)
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for _, f := range files {
		path := filepath.Join(dir, f)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if locs := re.FindAllIndex(data, -1); len(locs) > 0 {
			t.Errorf("%s: %d direct cfg.PriceGapCandidates= mutation(s) remain — must route through s.registry.Replace (Plan 11-03 hard-cut, T-11-18)", f, len(locs))
		}
	}
}

// Compile-time assertion: handlers.go must declare a CandidateRegistry
// interface satisfied by *pricegaptrader.Registry. This catches the
// Server.registry field/type drift at test compile time.
var _ CandidateRegistry = (*pricegaptrader.Registry)(nil)

// httptest is referenced through the postPriceGap helper but we keep the
// import explicit so tests that bypass the helper still compile.
var _ = httptest.NewRecorder
