package bybit

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestMaintenanceRateNormalization_Bybit verifies that Bybit maintenanceMargin
// is correctly divided by 100 (Bybit returns percentage: "0.5" = 0.5% = 0.005 decimal).
func TestMaintenanceRateNormalization_Bybit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v5/market/instruments-info":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"retCode": 0,
				"retMsg":  "OK",
				"result": map[string]interface{}{
					"list": []map[string]interface{}{
						{
							"symbol": "BTCUSDT",
							"status": "Trading",
							"lotSizeFilter": map[string]string{
								"minOrderQty": "0.001",
								"maxOrderQty": "100",
								"qtyStep":     "0.001",
							},
							"priceFilter": map[string]string{
								"tickSize": "0.1",
							},
						},
					},
				},
			})
		case "/v5/market/risk-limit":
			// Bybit returns maintenanceMargin as percentage: "0.5" means 0.5%
			symbol := r.URL.Query().Get("symbol")
			if symbol == "" {
				// Bulk request (used by LoadAllContracts)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"retCode": 0,
					"retMsg":  "OK",
					"result": map[string]interface{}{
						"list": []map[string]interface{}{
							{
								"symbol":            "BTCUSDT",
								"riskLimitValue":    "2000000",
								"maintenanceMargin": "0.5",
								"isLowestRisk":      1,
							},
							{
								"symbol":            "BTCUSDT",
								"riskLimitValue":    "4000000",
								"maintenanceMargin": "1.0",
								"isLowestRisk":      0,
							},
						},
						"nextPageCursor": "",
					},
				})
			} else {
				// Per-symbol request (used by GetMaintenanceRate)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"retCode": 0,
					"retMsg":  "OK",
					"result": map[string]interface{}{
						"list": []map[string]interface{}{
							{
								"symbol":            "BTCUSDT",
								"riskLimitValue":    "2000000",
								"maintenanceMargin": "0.5",
								"isLowestRisk":      1,
							},
							{
								"symbol":            "BTCUSDT",
								"riskLimitValue":    "4000000",
								"maintenanceMargin": "1.0",
								"isLowestRisk":      0,
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
		client: &Client{
			baseURL:    srv.URL,
			apiKey:     "test",
			secretKey:  "test",
			recvWindow: "5000",
			httpClient: http.DefaultClient,
		},
	}

	contracts, err := adapter.LoadAllContracts()
	if err != nil {
		t.Fatalf("LoadAllContracts: %v", err)
	}

	btc, ok := contracts["BTCUSDT"]
	if !ok {
		t.Fatal("BTCUSDT not found")
	}
	// "0.5" percentage / 100 = 0.005 decimal
	if btc.MaintenanceRate != 0.005 {
		t.Errorf("BTCUSDT MaintenanceRate = %v, want 0.005 (0.5%% / 100)", btc.MaintenanceRate)
	}
}

// TestGetMaintenanceRate_Bybit_TierMatching verifies tier-matched rate from
// Bybit risk-limit endpoint.
func TestGetMaintenanceRate_Bybit_TierMatching(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v5/market/risk-limit":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"retCode": 0,
				"retMsg":  "OK",
				"result": map[string]interface{}{
					"list": []map[string]interface{}{
						{
							"symbol":            "BTCUSDT",
							"riskLimitValue":    "2000000",
							"maintenanceMargin": "0.5",
							"isLowestRisk":      1,
						},
						{
							"symbol":            "BTCUSDT",
							"riskLimitValue":    "4000000",
							"maintenanceMargin": "1.0",
							"isLowestRisk":      0,
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
		client: &Client{
			baseURL:    srv.URL,
			apiKey:     "test",
			secretKey:  "test",
			recvWindow: "5000",
			httpClient: http.DefaultClient,
		},
	}

	// Notional 1M should match tier 1 (0.5% -> 0.005)
	rate, err := adapter.GetMaintenanceRate("BTCUSDT", 1000000)
	if err != nil {
		t.Fatalf("GetMaintenanceRate: %v", err)
	}
	if rate != 0.005 {
		t.Errorf("tier 1 rate = %v, want 0.005", rate)
	}

	// Notional 3M should match tier 2 (1.0% -> 0.01)
	rate2, err := adapter.GetMaintenanceRate("BTCUSDT", 3000000)
	if err != nil {
		t.Fatalf("GetMaintenanceRate: %v", err)
	}
	if rate2 != 0.01 {
		t.Errorf("tier 2 rate = %v, want 0.01", rate2)
	}

	// Notional 0 should return tier 1
	rate0, err := adapter.GetMaintenanceRate("BTCUSDT", 0)
	if err != nil {
		t.Fatalf("GetMaintenanceRate: %v", err)
	}
	if rate0 != 0.005 {
		t.Errorf("notional=0 rate = %v, want 0.005 (tier 1)", rate0)
	}
}
