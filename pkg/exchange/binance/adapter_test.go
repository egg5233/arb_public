package binance

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestGetClosePnL_Binance_CloseSizeUnknown verifies that Binance's income-API-
// based close-PnL aggregation flags CloseSizeUnknown=true. The Binance income
// endpoint does not expose trade size, so callers must not size-gate on the
// returned record.
func TestGetClosePnL_Binance_CloseSizeUnknown(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/fapi/v1/income" {
			http.NotFound(w, r)
			return
		}
		switch r.URL.Query().Get("incomeType") {
		case "REALIZED_PNL":
			json.NewEncoder(w).Encode([]map[string]interface{}{{"income": "12.34"}})
		case "COMMISSION":
			json.NewEncoder(w).Encode([]map[string]interface{}{{"income": "-0.50"}})
		case "FUNDING_FEE":
			json.NewEncoder(w).Encode([]map[string]interface{}{{"income": "0.25"}})
		default:
			json.NewEncoder(w).Encode([]map[string]interface{}{})
		}
	}))
	defer srv.Close()

	adapter := &Adapter{
		client:    NewClient("testkey", "testsecret").WithBaseURL(srv.URL),
		apiKey:    "testkey",
		secretKey: "testsecret",
	}

	pnls, err := adapter.GetClosePnL("BTCUSDT", time.Unix(0, 0))
	if err != nil {
		t.Fatalf("GetClosePnL: %v", err)
	}
	if len(pnls) != 1 {
		t.Fatalf("expected 1 aggregated record, got %d", len(pnls))
	}
	if !pnls[0].CloseSizeUnknown {
		t.Errorf("CloseSizeUnknown = false, want true (Binance income API has no size)")
	}
	if pnls[0].CloseSize != 0 {
		t.Errorf("CloseSize = %.4f, want 0 (not derivable)", pnls[0].CloseSize)
	}
}

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

// TestLoadAllContracts_Binance_DeliveryDateParsing verifies that
// LoadAllContracts populates ContractInfo.DeliveryDate from the
// fapi/v1/exchangeInfo deliveryDate field for true perpetuals being delisted,
// while leaving live perpetuals (year-2100 sentinel) with a zero DeliveryDate.
//
// Regression: 2026-04-08 batch delist (RLSUSDT, OLUSDT, HIPPOUSDT, PUFFERUSDT)
// — the article-scraper missed the announcement, but Binance had populated
// deliveryDate at announcement time. This test pins that signal.
func TestLoadAllContracts_Binance_DeliveryDateParsing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/fapi/v1/exchangeInfo":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"symbols": []map[string]interface{}{
					{
						// Normal live perpetual: year-2100 sentinel.
						"symbol":       "BTCUSDT",
						"status":       "TRADING",
						"contractType": "PERPETUAL",
						"deliveryDate": int64(4133404800000), // 2100-12-25 sentinel
						"filters": []map[string]interface{}{
							{"filterType": "LOT_SIZE", "minQty": "0.001", "maxQty": "1000", "stepSize": "0.001"},
							{"filterType": "PRICE_FILTER", "tickSize": "0.10"},
						},
					},
					{
						// Delisting perpetual still in TRADING (the critical case).
						// 1775725200000 = 2026-04-09T09:00:00Z
						"symbol":       "WIFUSDT",
						"status":       "TRADING",
						"contractType": "PERPETUAL",
						"deliveryDate": int64(1775725200000),
						"filters": []map[string]interface{}{
							{"filterType": "LOT_SIZE", "minQty": "1", "maxQty": "1000000", "stepSize": "1"},
							{"filterType": "PRICE_FILTER", "tickSize": "0.0001"},
						},
					},
					{
						// Dated quarterly: contractType != PERPETUAL → must NOT
						// be flagged as a delist even with a real deliveryDate.
						"symbol":       "BTCUSDT_240329",
						"status":       "TRADING",
						"contractType": "CURRENT_QUARTER",
						"deliveryDate": int64(1711699200000), // 2024-03-29
						"filters": []map[string]interface{}{
							{"filterType": "LOT_SIZE", "minQty": "0.001", "maxQty": "1000", "stepSize": "0.001"},
							{"filterType": "PRICE_FILTER", "tickSize": "0.10"},
						},
					},
				},
			})
		case "/fapi/v1/leverageBracket":
			// Auth-protected endpoint; we don't care about it for this test —
			// loadMaintenanceRates will swallow the error and continue.
			w.WriteHeader(401)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"code": -2015,
				"msg":  "Invalid API-key.",
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

	contracts, err := adapter.LoadAllContracts()
	if err != nil {
		t.Fatalf("LoadAllContracts: %v", err)
	}

	btc, ok := contracts["BTCUSDT"]
	if !ok {
		t.Fatal("BTCUSDT not found")
	}
	if !btc.DeliveryDate.IsZero() {
		t.Errorf("BTCUSDT (live perpetual) DeliveryDate = %v, want zero (year-2100 sentinel must be ignored)", btc.DeliveryDate)
	}

	wif, ok := contracts["WIFUSDT"]
	if !ok {
		t.Fatal("WIFUSDT not found")
	}
	if wif.DeliveryDate.IsZero() {
		t.Error("WIFUSDT (delisting perpetual) DeliveryDate is zero, want populated")
	}
	// Sanity-check the parsed time matches the millisecond input.
	wantUnixMs := int64(1775725200000)
	if got := wif.DeliveryDate.UnixMilli(); got != wantUnixMs {
		t.Errorf("WIFUSDT DeliveryDate = %d ms, want %d ms", got, wantUnixMs)
	}

	quarterly, ok := contracts["BTCUSDT_240329"]
	if !ok {
		t.Fatal("BTCUSDT_240329 not found")
	}
	if !quarterly.DeliveryDate.IsZero() {
		t.Errorf("BTCUSDT_240329 (quarterly) DeliveryDate = %v, want zero (only PERPETUAL contracts should be flagged)", quarterly.DeliveryDate)
	}
}
