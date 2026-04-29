package bybit

import (
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

func TestGetFuturesBalanceUsesAccountLevelFields(t *testing.T) {
	tests := []struct {
		name                string
		account             map[string]string
		coinEquity          string
		availableToWithdraw string
		withdrawal          string
		withdrawalOK        bool
		wantTotal           float64
		wantAvailable       float64
		wantFrozen          float64
		wantMarginRatio     float64
		wantMaxTransferOut  float64
		wantAuthoritative   bool
	}{
		{
			name: "zero totalAvailableBalance does not fallback to coin equity or availableToWithdraw zero",
			account: map[string]string{
				"accountMMRate":          "0.20",
				"totalAvailableBalance":  "0",
				"totalMarginBalance":     "500",
				"totalInitialMargin":     "125",
				"totalMaintenanceMargin": "25",
				"totalEquity":            "600",
			},
			coinEquity:          "400",
			availableToWithdraw: "0",
			withdrawalOK:        false,
			wantTotal:           500,
			wantAvailable:       0,
			wantFrozen:          125,
			wantMarginRatio:     0.20,
			wantMaxTransferOut:  0,
			wantAuthoritative:   false,
		},
		{
			name: "zero totalAvailableBalance does not fallback to coin equity when availableToWithdraw empty",
			account: map[string]string{
				"accountMMRate":          "0.25",
				"totalAvailableBalance":  "0",
				"totalMarginBalance":     "800",
				"totalInitialMargin":     "200",
				"totalMaintenanceMargin": "40",
				"totalEquity":            "900",
			},
			coinEquity:          "700",
			availableToWithdraw: "",
			withdrawalOK:        false,
			wantTotal:           800,
			wantAvailable:       0,
			wantFrozen:          200,
			wantMarginRatio:     0.25,
			wantMaxTransferOut:  0,
			wantAuthoritative:   false,
		},
		{
			name: "available uses totalAvailableBalance even when coin equity is zero",
			account: map[string]string{
				"accountMMRate":          "",
				"totalAvailableBalance":  "77.25",
				"totalMarginBalance":     "88",
				"totalInitialMargin":     "11",
				"totalMaintenanceMargin": "4.4",
				"totalEquity":            "99",
			},
			coinEquity:          "0",
			availableToWithdraw: "0",
			withdrawal:          "12.5",
			withdrawalOK:        true,
			wantTotal:           88,
			wantAvailable:       77.25,
			wantFrozen:          11,
			wantMarginRatio:     0.05,
			wantMaxTransferOut:  12.5,
			wantAuthoritative:   true,
		},
		{
			name: "total falls back to totalEquity only when totalMarginBalance is missing",
			account: map[string]string{
				"accountMMRate":          "0.09",
				"totalAvailableBalance":  "7",
				"totalMarginBalance":     "",
				"totalInitialMargin":     "3",
				"totalMaintenanceMargin": "2",
				"totalEquity":            "99.5",
			},
			coinEquity:          "1000",
			availableToWithdraw: "1000",
			withdrawalOK:        false,
			wantTotal:           99.5,
			wantAvailable:       7,
			wantFrozen:          3,
			wantMarginRatio:     0.09,
			wantMaxTransferOut:  0,
			wantAuthoritative:   false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/v5/account/wallet-balance":
					if r.URL.Query().Get("accountType") != "UNIFIED" {
						t.Fatalf("accountType = %q, want UNIFIED", r.URL.Query().Get("accountType"))
					}
					if r.URL.Query().Get("coin") != "USDT" {
						t.Fatalf("coin = %q, want USDT", r.URL.Query().Get("coin"))
					}

					account := map[string]interface{}{
						"accountMMRate":          tc.account["accountMMRate"],
						"totalAvailableBalance":  tc.account["totalAvailableBalance"],
						"totalMarginBalance":     tc.account["totalMarginBalance"],
						"totalInitialMargin":     tc.account["totalInitialMargin"],
						"totalMaintenanceMargin": tc.account["totalMaintenanceMargin"],
						"totalEquity":            tc.account["totalEquity"],
						"coin": []map[string]string{{
							"coin":                "USDT",
							"equity":              tc.coinEquity,
							"availableToWithdraw": tc.availableToWithdraw,
							"locked":              "999",
						}},
					}
					json.NewEncoder(w).Encode(map[string]interface{}{
						"retCode": 0,
						"retMsg":  "OK",
						"result": map[string]interface{}{
							"list": []map[string]interface{}{account},
						},
					})
				case "/v5/account/withdrawal":
					if !tc.withdrawalOK {
						json.NewEncoder(w).Encode(map[string]interface{}{
							"retCode": 10001,
							"retMsg":  "withdrawal unavailable",
							"result":  map[string]interface{}{},
						})
						return
					}
					json.NewEncoder(w).Encode(map[string]interface{}{
						"retCode": 0,
						"retMsg":  "OK",
						"result": map[string]interface{}{
							"availableWithdrawal": tc.withdrawal,
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

			got, err := adapter.GetFuturesBalance()
			if err != nil {
				t.Fatalf("GetFuturesBalance: %v", err)
			}

			assertFloatEqual(t, "Total", got.Total, tc.wantTotal)
			assertFloatEqual(t, "Available", got.Available, tc.wantAvailable)
			assertFloatEqual(t, "Frozen", got.Frozen, tc.wantFrozen)
			assertFloatEqual(t, "MarginRatio", got.MarginRatio, tc.wantMarginRatio)
			assertFloatEqual(t, "MaxTransferOut", got.MaxTransferOut, tc.wantMaxTransferOut)
			if got.Currency != "USDT" {
				t.Fatalf("Currency = %q, want USDT", got.Currency)
			}
			if got.MaxTransferOutAuthoritative != tc.wantAuthoritative {
				t.Fatalf("MaxTransferOutAuthoritative = %v, want %v", got.MaxTransferOutAuthoritative, tc.wantAuthoritative)
			}
		})
	}
}

func assertFloatEqual(t *testing.T, name string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-12 {
		t.Fatalf("%s = %v, want %v", name, got, want)
	}
}

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

	fees, err := adapter.GetFundingFees("BTCUSDT", time.Now().UTC().Add(-1*time.Hour))
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

func TestGetFundingFeesWalksTransactionLog24hWindows(t *testing.T) {
	var calls []string
	now := time.Now().UTC()
	since := now.Add(-49 * time.Hour)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v5/account/transaction-log":
			startMs, _ := strconv.ParseInt(r.URL.Query().Get("startTime"), 10, 64)
			endMs, _ := strconv.ParseInt(r.URL.Query().Get("endTime"), 10, 64)
			start := time.UnixMilli(startMs).UTC()
			end := time.UnixMilli(endMs).UTC()
			calls = append(calls, start.Format(time.RFC3339)+"->"+end.Format(time.RFC3339))

			var list []map[string]string
			switch len(calls) {
			case 1:
				list = []map[string]string{
					{"funding": "1.00", "transactionTime": strconv.FormatInt(start.Add(1*time.Hour).UnixMilli(), 10)},
					{"funding": "1.25", "transactionTime": strconv.FormatInt(start.Add(2*time.Hour).UnixMilli(), 10)},
				}
			case 2:
				list = []map[string]string{
					{"funding": "2.00", "transactionTime": strconv.FormatInt(start.Add(1*time.Hour).UnixMilli(), 10)},
					{"funding": "2.25", "transactionTime": strconv.FormatInt(start.Add(2*time.Hour).UnixMilli(), 10)},
				}
			case 3:
				list = []map[string]string{
					{"funding": "3.00", "transactionTime": strconv.FormatInt(start.Add(1*time.Hour).UnixMilli(), 10)},
				}
			default:
				t.Fatalf("unexpected extra window request %v", calls)
			}

			json.NewEncoder(w).Encode(map[string]interface{}{
				"retCode": 0,
				"retMsg":  "OK",
				"result": map[string]interface{}{
					"list":           list,
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

	fees, err := adapter.GetFundingFees("BTCUSDT", since)
	if err != nil {
		t.Fatalf("GetFundingFees: %v", err)
	}

	if len(calls) != 3 {
		t.Fatalf("window calls = %d, want 3 (%v)", len(calls), calls)
	}
	if len(fees) != 5 {
		t.Fatalf("funding payments = %d, want 5", len(fees))
	}
	if fees[0].Amount != 1.00 || fees[2].Amount != 2.00 || fees[4].Amount != 3.00 {
		t.Fatalf("unexpected funding sequence: %+v", fees)
	}
}
