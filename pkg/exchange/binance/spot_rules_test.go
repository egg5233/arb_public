package binance

import (
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestSpotOrderRulesBinanceGolden(t *testing.T) {
	fixture, err := os.ReadFile("testdata/exchangeInfo_BTCUSDT.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/exchangeInfo" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("symbol") != "BTCUSDT" {
			t.Errorf("expected symbol=BTCUSDT, got %q", r.URL.Query().Get("symbol"))
		}
		// Spot exchangeInfo is a public endpoint — must NOT have timestamp/signature.
		if r.URL.Query().Get("timestamp") != "" {
			t.Errorf("unexpected timestamp param on public endpoint")
		}
		if r.URL.Query().Get("signature") != "" {
			t.Errorf("unexpected signature param on public endpoint")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(fixture)
	}))
	defer srv.Close()

	adapter := &Adapter{client: NewClient("", "").WithBaseURL(srv.URL)}

	spotRulesCacheMu.Lock()
	delete(spotRulesCache, "BTCUSDT")
	spotRulesCacheMu.Unlock()

	rules, err := adapter.SpotOrderRules("BTCUSDT")
	if err != nil {
		t.Fatalf("SpotOrderRules: %v", err)
	}
	if math.Abs(rules.MinBaseQty-0.00001) > 1e-10 {
		t.Errorf("MinBaseQty = %v, want 0.00001", rules.MinBaseQty)
	}
	if math.Abs(rules.QtyStep-0.00001) > 1e-10 {
		t.Errorf("QtyStep = %v, want 0.00001", rules.QtyStep)
	}
	if math.Abs(rules.MinNotional-5.0) > 1e-9 {
		t.Errorf("MinNotional = %v, want 5.0", rules.MinNotional)
	}
}

func TestSpotOrderRulesBinanceCached(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"symbols":[{"symbol":"ETHUSDT","filters":[{"filterType":"LOT_SIZE","minQty":"0.0001","stepSize":"0.0001"},{"filterType":"MIN_NOTIONAL","minNotional":"10"}]}]}`))
	}))
	defer srv.Close()

	adapter := &Adapter{client: NewClient("", "").WithBaseURL(srv.URL)}
	const sym = "ETHUSDT"

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

func TestSpotOrderRulesBinanceCacheExpiry(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"symbols":[{"symbol":"XYZUSDT","filters":[{"filterType":"LOT_SIZE","minQty":"1","stepSize":"1"}]}]}`))
	}))
	defer srv.Close()

	adapter := &Adapter{client: NewClient("", "").WithBaseURL(srv.URL)}
	const sym = "XYZUSDT"

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

func TestSpotOrderRulesBinanceNoSymbol(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"symbols":[]}`))
	}))
	defer srv.Close()

	adapter := &Adapter{client: NewClient("", "").WithBaseURL(srv.URL)}
	const sym = "NOSUCHUSDT"

	spotRulesCacheMu.Lock()
	delete(spotRulesCache, sym)
	spotRulesCacheMu.Unlock()

	_, err := adapter.SpotOrderRules(sym)
	if err == nil {
		t.Fatal("expected error for empty symbols list")
	}
}
