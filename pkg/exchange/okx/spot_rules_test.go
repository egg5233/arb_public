package okx

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestSpotOrderRulesOKXGolden(t *testing.T) {
	// OKX client unwraps the envelope and returns Data directly.
	// Wrap in the OKX response envelope for the test server.
	fixture, err := os.ReadFile("testdata/instruments_GWEI_USDT.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v5/public/instruments" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("instType") != "SPOT" {
			t.Errorf("expected instType=SPOT, got %q", r.URL.Query().Get("instType"))
		}
		if r.URL.Query().Get("instId") != "GWEI-USDT" {
			t.Errorf("expected instId=GWEI-USDT, got %q", r.URL.Query().Get("instId"))
		}
		w.Header().Set("Content-Type", "application/json")
		// Wrap fixture in OKX envelope.
		w.Write([]byte(`{"code":"0","msg":"","data":`))
		w.Write(fixture)
		w.Write([]byte(`}`))
	}))
	defer srv.Close()

	adapter := &Adapter{
		client:    NewClientWithBase(srv.URL),
		priceSyms: make(map[string]bool),
		depthSyms: make(map[string]bool),
	}

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
		t.Errorf("QtyStep = %v, want 1.0", rules.QtyStep)
	}
	// minNotional is empty string in fixture → should be 0.
	if rules.MinNotional != 0.0 {
		t.Errorf("MinNotional = %v, want 0.0 (absent field)", rules.MinNotional)
	}
}

func TestSpotOrderRulesOKXCached(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"code":"0","msg":"","data":[{"minSz":"0.001","lotSz":"0.001","minNotional":""}]}`))
	}))
	defer srv.Close()

	adapter := &Adapter{
		client:    NewClientWithBase(srv.URL),
		priceSyms: make(map[string]bool),
		depthSyms: make(map[string]bool),
	}
	const sym = "ETHUSDT_okx_cache"

	spotRulesCacheMu.Lock()
	delete(spotRulesCache, sym)
	spotRulesCacheMu.Unlock()

	if _, err := adapter.SpotOrderRules(sym); err != nil {
		t.Fatalf("first call: %v", err)
	}
	if _, err := adapter.SpotOrderRules(sym); err != nil {
		t.Fatalf("second call: %v", err)
	}
	if calls != 1 {
		t.Errorf("expected 1 HTTP call (cache hit on second), got %d", calls)
	}
}

func TestSpotOrderRulesOKXCacheExpiry(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"code":"0","msg":"","data":[{"minSz":"1","lotSz":"1","minNotional":""}]}`))
	}))
	defer srv.Close()

	adapter := &Adapter{
		client:    NewClientWithBase(srv.URL),
		priceSyms: make(map[string]bool),
		depthSyms: make(map[string]bool),
	}
	const sym = "GWEI_okx_expired"

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

func TestSpotOrderRulesOKXEmptyData(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"code":"0","msg":"","data":[]}`))
	}))
	defer srv.Close()

	adapter := &Adapter{
		client:    NewClientWithBase(srv.URL),
		priceSyms: make(map[string]bool),
		depthSyms: make(map[string]bool),
	}
	const sym = "NOSUCHOKX"

	spotRulesCacheMu.Lock()
	delete(spotRulesCache, sym)
	spotRulesCacheMu.Unlock()

	_, err := adapter.SpotOrderRules(sym)
	if err == nil {
		t.Fatal("expected error for empty data array")
	}
}
