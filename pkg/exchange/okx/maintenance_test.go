package okx

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestMaintenanceRateNormalization_OKX verifies that OKX mmr is parsed correctly
// from position-tiers (already decimal string: "0.03" = 3%).
func TestMaintenanceRateNormalization_OKX(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		respond := func(data interface{}) {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"code": "0",
				"msg":  "",
				"data": data,
			})
		}
		switch r.URL.Path {
		case "/api/v5/public/instruments":
			respond([]map[string]interface{}{{
				"instId":    "BTC-USDT-SWAP",
				"tickSz":    "0.1",
				"lotSz":     "1",
				"minSz":     "1",
				"ctVal":     "0.01",
				"state":     "live",
				"settleCcy": "USDT",
			}})
		case "/api/v5/account/set-position-mode":
			respond(nil)
		case "/api/v5/public/position-tiers":
			instFamily := r.URL.Query().Get("instFamily")
			if instFamily != "BTC-USDT" {
				t.Errorf("expected instFamily=BTC-USDT, got %s", instFamily)
			}
			respond([]map[string]interface{}{
				{
					"tier":     "1",
					"minSz":    "0",
					"maxSz":    "15000",
					"mmr":      "0.01",
					"maxLever": "125",
				},
				{
					"tier":     "2",
					"minSz":    "15000",
					"maxSz":    "50000",
					"mmr":      "0.02",
					"maxLever": "50",
				},
				{
					"tier":     "3",
					"minSz":    "50000",
					"maxSz":    "200000",
					"mmr":      "0.03",
					"maxLever": "20",
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	adapter := &Adapter{
		client:    NewClientWithBase(srv.URL),
		priceSyms: make(map[string]bool),
		depthSyms: make(map[string]bool),
	}

	// Load contracts to populate ctValCache
	adapter.LoadAllContracts()

	// Test GetMaintenanceRate with small notional (tier 1)
	// OKX tiers are in contract counts. ctVal=0.01, so notional 100 USDT at price 50000
	// means ~0.002 BTC = 0.2 contracts. Should match tier 1.
	rate, err := adapter.GetMaintenanceRate("BTCUSDT", 100)
	if err != nil {
		t.Fatalf("GetMaintenanceRate: %v", err)
	}
	if rate != 0.01 {
		t.Errorf("tier 1 rate = %v, want 0.01", rate)
	}
}

// TestGetMaintenanceRate_OKX_TierMatching verifies OKX tier matching
// by contract count boundaries.
func TestGetMaintenanceRate_OKX_TierMatching(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		respond := func(data interface{}) {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"code": "0",
				"msg":  "",
				"data": data,
			})
		}
		switch r.URL.Path {
		case "/api/v5/public/instruments":
			respond([]map[string]interface{}{{
				"instId":    "ETH-USDT-SWAP",
				"tickSz":    "0.01",
				"lotSz":     "1",
				"minSz":     "1",
				"ctVal":     "0.1",
				"state":     "live",
				"settleCcy": "USDT",
			}})
		case "/api/v5/account/set-position-mode":
			respond(nil)
		case "/api/v5/public/position-tiers":
			respond([]map[string]interface{}{
				{
					"tier":     "1",
					"minSz":    "0",
					"maxSz":    "500",
					"mmr":      "0.02",
					"maxLever": "100",
				},
				{
					"tier":     "2",
					"minSz":    "500",
					"maxSz":    "5000",
					"mmr":      "0.04",
					"maxLever": "50",
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	adapter := &Adapter{
		client:    NewClientWithBase(srv.URL),
		priceSyms: make(map[string]bool),
		depthSyms: make(map[string]bool),
	}

	adapter.LoadAllContracts()

	// Notional 0 should return tier 1 (0.02)
	rate0, err := adapter.GetMaintenanceRate("ETHUSDT", 0)
	if err != nil {
		t.Fatalf("GetMaintenanceRate: %v", err)
	}
	if rate0 != 0.02 {
		t.Errorf("notional=0 rate = %v, want 0.02 (tier 1)", rate0)
	}
}
