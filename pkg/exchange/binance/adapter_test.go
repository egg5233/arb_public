package binance

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestMaintenanceRateNormalization_Binance verifies that Binance maintMarginRatio
// is used directly (already decimal: 0.0065 = 0.65%).
func TestMaintenanceRateNormalization_Binance(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/fapi/v1/exchangeInfo":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"symbols": []map[string]interface{}{
					{
						"symbol": "BTCUSDT",
						"status": "TRADING",
						"filters": []map[string]interface{}{
							{
								"filterType": "LOT_SIZE",
								"minQty":     "0.001",
								"maxQty":     "1000",
								"stepSize":   "0.001",
							},
							{
								"filterType": "PRICE_FILTER",
								"tickSize":   "0.10",
							},
						},
					},
				},
			})
		case "/fapi/v1/leverageBracket":
			// Authenticated endpoint. Check for API key header.
			if r.Header.Get("X-MBX-APIKEY") == "" {
				w.WriteHeader(401)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"code": -2015,
					"msg":  "Invalid API-key, IP, or permissions for action.",
				})
				return
			}
			symbol := r.URL.Query().Get("symbol")
			if symbol != "" {
				// Per-symbol response
				json.NewEncoder(w).Encode([]map[string]interface{}{
					{
						"symbol": "BTCUSDT",
						"brackets": []map[string]interface{}{
							{
								"bracket":          1,
								"notionalCap":      50000,
								"notionalFloor":    0,
								"maintMarginRatio": 0.004,
								"cum":              0,
							},
							{
								"bracket":          2,
								"notionalCap":      250000,
								"notionalFloor":    50000,
								"maintMarginRatio": 0.005,
								"cum":              50,
							},
							{
								"bracket":          3,
								"notionalCap":      1000000,
								"notionalFloor":    250000,
								"maintMarginRatio": 0.01,
								"cum":              1300,
							},
						},
					},
				})
			} else {
				// Bulk response (all symbols)
				json.NewEncoder(w).Encode([]map[string]interface{}{
					{
						"symbol": "BTCUSDT",
						"brackets": []map[string]interface{}{
							{
								"bracket":          1,
								"notionalCap":      50000,
								"notionalFloor":    0,
								"maintMarginRatio": 0.004,
								"cum":              0,
							},
							{
								"bracket":          2,
								"notionalCap":      250000,
								"notionalFloor":    50000,
								"maintMarginRatio": 0.005,
								"cum":              50,
							},
						},
					},
				})
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	adapter := &Adapter{
		client:    NewClient("testkey", "testsecret").WithBaseURL(srv.URL),
		apiKey:    "testkey",
		secretKey: "testsecret",
		priceSyms: make(map[string]bool),
		depthSyms: make(map[string]bool),
	}

	contracts, err := adapter.LoadAllContracts()
	if err != nil {
		t.Fatalf("LoadAllContracts: %v", err)
	}

	btc, ok := contracts["BTCUSDT"]
	if !ok {
		t.Fatal("BTCUSDT not found")
	}
	// First bracket maintMarginRatio = 0.004 (already decimal)
	if btc.MaintenanceRate != 0.004 {
		t.Errorf("BTCUSDT MaintenanceRate = %v, want 0.004", btc.MaintenanceRate)
	}
}

// TestGetMaintenanceRate_Binance_TierMatching verifies tier-matched rate from
// Binance leverageBracket endpoint (authenticated).
func TestGetMaintenanceRate_Binance_TierMatching(t *testing.T) {
	var authChecked bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/fapi/v1/leverageBracket":
			// Verify authentication
			if r.Header.Get("X-MBX-APIKEY") != "" {
				authChecked = true
			}
			json.NewEncoder(w).Encode([]map[string]interface{}{
				{
					"symbol": "ETHUSDT",
					"brackets": []map[string]interface{}{
						{
							"bracket":          1,
							"notionalCap":      50000,
							"notionalFloor":    0,
							"maintMarginRatio": 0.0065,
							"cum":              0,
						},
						{
							"bracket":          2,
							"notionalCap":      250000,
							"notionalFloor":    50000,
							"maintMarginRatio": 0.01,
							"cum":              175,
						},
					},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	adapter := &Adapter{
		client:    NewClient("testkey", "testsecret").WithBaseURL(srv.URL),
		apiKey:    "testkey",
		secretKey: "testsecret",
		priceSyms: make(map[string]bool),
		depthSyms: make(map[string]bool),
	}

	// Notional 10k should match bracket 1 (0.0065)
	rate, err := adapter.GetMaintenanceRate("ETHUSDT", 10000)
	if err != nil {
		t.Fatalf("GetMaintenanceRate: %v", err)
	}
	if rate != 0.0065 {
		t.Errorf("bracket 1 rate = %v, want 0.0065", rate)
	}

	// Verify authentication was used
	if !authChecked {
		t.Error("leverageBracket should use authenticated client (X-MBX-APIKEY header)")
	}

	// Notional 100k should match bracket 2 (0.01)
	rate2, err := adapter.GetMaintenanceRate("ETHUSDT", 100000)
	if err != nil {
		t.Fatalf("GetMaintenanceRate: %v", err)
	}
	if rate2 != 0.01 {
		t.Errorf("bracket 2 rate = %v, want 0.01", rate2)
	}

	// Notional 0 should return bracket 1
	rate0, err := adapter.GetMaintenanceRate("ETHUSDT", 0)
	if err != nil {
		t.Fatalf("GetMaintenanceRate: %v", err)
	}
	if rate0 != 0.0065 {
		t.Errorf("notional=0 rate = %v, want 0.0065 (bracket 1)", rate0)
	}
}
