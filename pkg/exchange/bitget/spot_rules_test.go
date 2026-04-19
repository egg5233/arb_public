package bitget

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

// newBitgetTestAdapter creates a Bitget adapter pointing at a test server URL.
func newBitgetTestAdapter(url string) *Adapter {
	return &Adapter{client: &Client{
		baseURL:    url,
		httpClient: &http.Client{},
	}}
}

func TestSpotOrderRulesBitgetGolden(t *testing.T) {
	fixture, err := os.ReadFile("testdata/spot_symbols_GWEIUSDT.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/spot/public/symbols" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("symbol") != "GWEIUSDT" {
			t.Errorf("expected symbol=GWEIUSDT, got %q", r.URL.Query().Get("symbol"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(fixture)
	}))
	defer srv.Close()

	adapter := newBitgetTestAdapter(srv.URL)

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
	// quantityScale=0 → step=1 (integer-only)
	if rules.QtyStep != 1.0 {
		t.Errorf("QtyStep = %v, want 1.0 (quantityScale=0)", rules.QtyStep)
	}
	if rules.MinNotional != 1.0 {
		t.Errorf("MinNotional = %v, want 1.0", rules.MinNotional)
	}
}

func TestSpotOrderRulesBitgetCached(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		fmt.Fprint(w, `{"code":"00000","msg":"success","data":[{"minTradeAmount":"0.01","quantityScale":"2","minTradeUSDT":"5"}]}`)
	}))
	defer srv.Close()

	adapter := newBitgetTestAdapter(srv.URL)
	const sym = "ETHUSDT_bitget_cache"

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

func TestSpotOrderRulesBitgetCacheExpiry(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		fmt.Fprint(w, `{"code":"00000","msg":"success","data":[{"minTradeAmount":"1","quantityScale":"0","minTradeUSDT":"1"}]}`)
	}))
	defer srv.Close()

	adapter := newBitgetTestAdapter(srv.URL)
	const sym = "GWEI_bitget_expired"

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

func TestSpotOrderRulesBitgetAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"code":"40200","msg":"Invalid symbol","data":[]}`)
	}))
	defer srv.Close()

	adapter := newBitgetTestAdapter(srv.URL)
	const sym = "INVALIDSYM"

	spotRulesCacheMu.Lock()
	delete(spotRulesCache, sym)
	spotRulesCacheMu.Unlock()

	_, err := adapter.SpotOrderRules(sym)
	if err == nil {
		t.Fatal("expected error for non-00000 code")
	}
}

func TestSpotOrderRulesBitgetQuantityScale(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// quantityScale=3 → step=0.001
		fmt.Fprint(w, `{"code":"00000","msg":"success","data":[{"minTradeAmount":"0.001","quantityScale":"3","minTradeUSDT":"1"}]}`)
	}))
	defer srv.Close()

	adapter := newBitgetTestAdapter(srv.URL)
	const sym = "BTCUSDT_bitget_scale"

	spotRulesCacheMu.Lock()
	delete(spotRulesCache, sym)
	spotRulesCacheMu.Unlock()

	rules, err := adapter.SpotOrderRules(sym)
	if err != nil {
		t.Fatalf("SpotOrderRules: %v", err)
	}
	const wantStep = 0.001
	if rules.QtyStep < wantStep*0.9999 || rules.QtyStep > wantStep*1.0001 {
		t.Errorf("QtyStep = %v, want ~%v (quantityScale=3)", rules.QtyStep, wantStep)
	}
}
