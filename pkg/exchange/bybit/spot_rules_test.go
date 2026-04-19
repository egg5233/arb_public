package bybit

import (
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestSpotOrderRulesBybitGolden(t *testing.T) {
	fixture, err := os.ReadFile("testdata/instruments_info_GWEI_USDT.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v5/market/instruments-info" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("category") != "spot" {
			t.Errorf("expected category=spot, got %q", r.URL.Query().Get("category"))
		}
		if r.URL.Query().Get("symbol") != "GWEIUSDT" {
			t.Errorf("expected symbol=GWEIUSDT, got %q", r.URL.Query().Get("symbol"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(fixture)
	}))
	defer srv.Close()

	adapter := &Adapter{client: NewClient("key", "secret")}
	adapter.client.baseURL = srv.URL
	adapter.client.httpClient = srv.Client()

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
		t.Errorf("QtyStep = %v, want 1.0 (basePrecision=1)", rules.QtyStep)
	}
	if rules.MinNotional != 1.0 {
		t.Errorf("MinNotional = %v, want 1.0", rules.MinNotional)
	}
}

func TestSpotOrderRulesBybitCached(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		fmt.Fprint(w, `{"retCode":0,"retMsg":"OK","result":{"category":"spot","list":[{"lotSizeFilter":{"minOrderQty":"0.001","basePrecision":"0.001"},"quoteFilter":{"minOrderAmt":"1"}}]}}`)
	}))
	defer srv.Close()

	adapter := &Adapter{client: NewClient("key", "secret")}
	adapter.client.baseURL = srv.URL
	adapter.client.httpClient = srv.Client()
	const sym = "ETHUSDT_bybit_cache"

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

func TestSpotOrderRulesBybitCacheExpiry(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		fmt.Fprint(w, `{"retCode":0,"retMsg":"OK","result":{"category":"spot","list":[{"lotSizeFilter":{"minOrderQty":"1","basePrecision":"1"},"quoteFilter":{"minOrderAmt":"1"}}]}}`)
	}))
	defer srv.Close()

	adapter := &Adapter{client: NewClient("key", "secret")}
	adapter.client.baseURL = srv.URL
	adapter.client.httpClient = srv.Client()
	const sym = "GWEI_bybit_expired"

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

func TestSpotOrderRulesBybitEmptyList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"retCode":0,"retMsg":"OK","result":{"category":"spot","list":[]}}`)
	}))
	defer srv.Close()

	adapter := &Adapter{client: NewClient("key", "secret")}
	adapter.client.baseURL = srv.URL
	adapter.client.httpClient = srv.Client()
	const sym = "NOSUCHBYBIT"

	spotRulesCacheMu.Lock()
	delete(spotRulesCache, sym)
	spotRulesCacheMu.Unlock()

	_, err := adapter.SpotOrderRules(sym)
	if err == nil {
		t.Fatal("expected error for empty instrument list")
	}
}

func TestSpotOrderRulesBybitPrecisionStep(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"retCode":0,"retMsg":"OK","result":{"category":"spot","list":[{"lotSizeFilter":{"minOrderQty":"0.0001","basePrecision":"0.0001"},"quoteFilter":{"minOrderAmt":"5"}}]}}`)
	}))
	defer srv.Close()

	adapter := &Adapter{client: NewClient("key", "secret")}
	adapter.client.baseURL = srv.URL
	adapter.client.httpClient = srv.Client()
	const sym = "BTCUSDT_bybit_step"

	spotRulesCacheMu.Lock()
	delete(spotRulesCache, sym)
	spotRulesCacheMu.Unlock()

	rules, err := adapter.SpotOrderRules(sym)
	if err != nil {
		t.Fatalf("SpotOrderRules: %v", err)
	}
	if math.Abs(rules.QtyStep-0.0001) > 1e-12 {
		t.Errorf("QtyStep = %v, want 0.0001", rules.QtyStep)
	}
	if math.Abs(rules.MinBaseQty-0.0001) > 1e-12 {
		t.Errorf("MinBaseQty = %v, want 0.0001", rules.MinBaseQty)
	}
	if rules.MinNotional != 5.0 {
		t.Errorf("MinNotional = %v, want 5.0", rules.MinNotional)
	}
}
