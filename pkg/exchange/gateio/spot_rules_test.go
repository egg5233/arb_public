package gateio

import (
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestSpotOrderRulesGateioGolden(t *testing.T) {
	fixture, err := os.ReadFile("testdata/spot_currency_pair_GWEI_USDT.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/spot/currency_pairs/GWEI_USDT" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(fixture)
	}))
	defer srv.Close()

	adapter := &Adapter{client: NewClientWithBase(srv.URL)}
	// Clear cache to force live fetch.
	spotRulesCacheMu.Lock()
	delete(spotRulesCache, "GWEIUSDT")
	spotRulesCacheMu.Unlock()

	rules, err := adapter.SpotOrderRules("GWEIUSDT")
	if err != nil {
		t.Fatalf("SpotOrderRules: %v", err)
	}
	if rules.MinBaseQty != 1.0 {
		t.Errorf("MinBaseQty = %v, want 1.0", rules.MinBaseQty)
	}
	if rules.QtyStep != 1.0 {
		t.Errorf("QtyStep = %v, want 1.0 (precision=0 → integer-only)", rules.QtyStep)
	}
	if rules.MinNotional != 1.0 {
		t.Errorf("MinNotional = %v, want 1.0", rules.MinNotional)
	}
}

func TestSpotOrderRulesGateioCached(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"min_base_amount":"0.001","amount_precision":3,"min_quote_amount":"2"}`))
	}))
	defer srv.Close()

	adapter := &Adapter{client: NewClientWithBase(srv.URL)}
	const sym = "BTCUSDT_cachetest"

	spotRulesCacheMu.Lock()
	delete(spotRulesCache, sym)
	spotRulesCacheMu.Unlock()

	// First call: hits server.
	if _, err := adapter.SpotOrderRules(sym); err != nil {
		t.Fatalf("first call: %v", err)
	}
	// Second call within TTL: must use cache.
	if _, err := adapter.SpotOrderRules(sym); err != nil {
		t.Fatalf("second call: %v", err)
	}
	if calls != 1 {
		t.Errorf("expected 1 HTTP call (cache hit on second), got %d", calls)
	}
}

func TestSpotOrderRulesGateioCacheExpiry(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"min_base_amount":"1","amount_precision":0,"min_quote_amount":"1"}`))
	}))
	defer srv.Close()

	adapter := &Adapter{client: NewClientWithBase(srv.URL)}
	const sym = "GWEI_expired"

	// Plant an already-expired cache entry.
	spotRulesCacheMu.Lock()
	spotRulesCache[sym] = spotRulesCacheEntry{
		rules:     nil,
		expiresAt: time.Now().Add(-1 * time.Second),
	}
	spotRulesCacheMu.Unlock()

	if _, err := adapter.SpotOrderRules(sym); err != nil {
		t.Fatalf("SpotOrderRules after expiry: %v", err)
	}
	if calls != 1 {
		t.Errorf("expected 1 HTTP call after cache expiry, got %d", calls)
	}
}

func TestSpotOrderRulesGateioPrecisionStep(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// amount_precision=3 → step=0.001
		w.Write([]byte(`{"min_base_amount":"0.001","amount_precision":3,"min_quote_amount":"0"}`))
	}))
	defer srv.Close()

	adapter := &Adapter{client: NewClientWithBase(srv.URL)}
	const sym = "ETHUSDT_step"

	spotRulesCacheMu.Lock()
	delete(spotRulesCache, sym)
	spotRulesCacheMu.Unlock()

	rules, err := adapter.SpotOrderRules(sym)
	if err != nil {
		t.Fatalf("SpotOrderRules: %v", err)
	}
	if math.Abs(rules.QtyStep-0.001) > 1e-10 {
		t.Errorf("QtyStep = %v, want 0.001 (precision=3)", rules.QtyStep)
	}
	if math.Abs(rules.MinBaseQty-0.001) > 1e-10 {
		t.Errorf("MinBaseQty = %v, want 0.001", rules.MinBaseQty)
	}
}
