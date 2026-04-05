package bybit

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
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

func TestGetFundingFeesPaginatesTransactionLog(t *testing.T) {
	var firstPageCalls, secondPageCalls int

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v5/account/transaction-log":
			cursor := r.URL.Query().Get("cursor")
			if cursor == "" {
				firstPageCalls++
				if r.URL.Query().Get("startTime") == "" {
					t.Fatal("expected startTime query param on first page")
				}

				list := make([]map[string]string, 0, 50)
				for i := 0; i < 50; i++ {
					list = append(list, map[string]string{
						"funding":         "1.25",
						"transactionTime": strconv.FormatInt(time.Date(2026, 4, 4, i%24, 0, 0, 0, time.UTC).Add(time.Duration(i/24)*24*time.Hour).UnixMilli(), 10),
					})
				}
				json.NewEncoder(w).Encode(map[string]interface{}{
					"retCode": 0,
					"retMsg":  "OK",
					"result": map[string]interface{}{
						"list":           list,
						"nextPageCursor": "page-2",
					},
				})
				return
			}

			if cursor != "page-2" {
				t.Fatalf("unexpected cursor %q", cursor)
			}
			secondPageCalls++
			json.NewEncoder(w).Encode(map[string]interface{}{
				"retCode": 0,
				"retMsg":  "OK",
				"result": map[string]interface{}{
					"list": []map[string]string{
						{"funding": "2.50", "transactionTime": strconv.FormatInt(time.Date(2026, 4, 6, 2, 0, 0, 0, time.UTC).UnixMilli(), 10)},
						{"funding": "2.75", "transactionTime": strconv.FormatInt(time.Date(2026, 4, 6, 3, 0, 0, 0, time.UTC).UnixMilli(), 10)},
						{"funding": "3.00", "transactionTime": strconv.FormatInt(time.Date(2026, 4, 6, 4, 0, 0, 0, time.UTC).UnixMilli(), 10)},
					},
					"nextPageCursor": "",
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

	fees, err := adapter.GetFundingFees("BTCUSDT", time.Date(2026, 4, 4, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("GetFundingFees: %v", err)
	}

	if firstPageCalls != 1 {
		t.Fatalf("first page calls = %d, want 1", firstPageCalls)
	}
	if secondPageCalls != 1 {
		t.Fatalf("second page calls = %d, want 1", secondPageCalls)
	}
	if len(fees) != 53 {
		t.Fatalf("funding payments = %d, want 53", len(fees))
	}
	if fees[len(fees)-1].Amount != 3.00 {
		t.Fatalf("last funding amount = %.2f, want 3.00", fees[len(fees)-1].Amount)
	}
	if !fees[len(fees)-1].Time.Equal(time.Date(2026, 4, 6, 4, 0, 0, 0, time.UTC)) {
		t.Fatalf("last funding time = %s, want 2026-04-06 04:00 UTC", fees[len(fees)-1].Time.UTC().Format(time.RFC3339))
	}
}
