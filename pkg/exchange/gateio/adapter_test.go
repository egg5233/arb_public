package gateio

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"arb/pkg/exchange"
)

// TestQuantoMultiplierRoundTrip verifies that Gate.io position sizes are
// returned in base asset units (not raw contracts), so that passing the
// size back to PlaceOrder produces the correct contract count.
//
// Gate.io IRUSDT has quanto_multiplier=10, meaning 1 contract = 10 IR.
// GetPosition should return 880 (base units) for 88 contracts.
// PlaceOrder should convert 880 back to 88 contracts.
func TestQuantoMultiplierRoundTrip(t *testing.T) {
	const (
		quantoMult  = 10.0
		contracts   = int64(88)
		baseUnits   = float64(contracts) * quantoMult // 880
		symbol      = "IR_USDT"
		internalSym = "IRUSDT"
	)

	// Track what PlaceOrder sends to the API.
	var placedSize int64
	var mu sync.Mutex

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v4/futures/usdt/contracts":
			// LoadAllContracts response
			json.NewEncoder(w).Encode([]map[string]interface{}{{
				"name":              symbol,
				"quanto_multiplier": strconv.FormatFloat(quantoMult, 'f', -1, 64),
				"order_size_min":    1,
				"order_size_max":    1000000,
				"order_price_round": "0.0001",
			}})

		case r.URL.Path == "/api/v4/futures/usdt/positions/"+symbol:
			// GetPosition response — returns raw contract count
			json.NewEncoder(w).Encode(map[string]interface{}{
				"contract":             symbol,
				"size":                 -contracts, // negative = short
				"entry_price":          "0.0400",
				"unrealised_pnl":       "-0.50",
				"leverage":             "5",
				"cross_leverage_limit": 0,
				"mode":                 "single",
				"liq_price":            "0.0500",
				"mark_price":           "0.0410",
			})

		case r.URL.Path == "/api/v4/futures/usdt/orders" && r.Method == "POST":
			// PlaceOrder — capture the size sent
			var req struct {
				Size int64 `json:"size"`
			}
			json.NewDecoder(r.Body).Decode(&req)
			mu.Lock()
			placedSize = req.Size
			mu.Unlock()
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id":   12345,
				"text": "",
			})

		case r.URL.Path == "/api/v4/futures/usdt/orders/12345":
			// GetOrderFilledQty response — filled all contracts
			json.NewEncoder(w).Encode(map[string]interface{}{
				"size":       contracts,
				"left":       0,
				"fill_price": "0.0400",
			})

		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	adapter := &Adapter{
		client:    NewClientWithBase(srv.URL + "/api/v4"),
		priceSyms: make(map[string]bool),
	}

	// Step 1: Load contracts to populate contractMult
	_, err := adapter.LoadAllContracts()
	if err != nil {
		t.Fatalf("LoadAllContracts: %v", err)
	}

	// Verify multiplier was loaded
	mult, ok := adapter.contractMult[internalSym]
	if !ok || mult != quantoMult {
		t.Fatalf("expected contractMult[%s] = %v, got %v (ok=%v)", internalSym, quantoMult, mult, ok)
	}

	// Step 2: GetPosition should return base asset units, not contracts
	positions, err := adapter.GetPosition(internalSym)
	if err != nil {
		t.Fatalf("GetPosition: %v", err)
	}
	if len(positions) != 1 {
		t.Fatalf("expected 1 position, got %d", len(positions))
	}

	posSize, _ := strconv.ParseFloat(positions[0].Total, 64)
	if posSize != baseUnits {
		t.Errorf("GetPosition size: expected %.0f base units, got %.0f (raw contracts=%d × mult=%.0f)",
			baseUnits, posSize, contracts, quantoMult)
	}
	if positions[0].HoldSide != "short" {
		t.Errorf("expected holdSide=short, got %s", positions[0].HoldSide)
	}

	// Step 3: PlaceOrder with the position size should send correct contracts
	_, err = adapter.PlaceOrder(exchange.PlaceOrderParams{
		Symbol:     internalSym,
		Side:       exchange.SideBuy,
		OrderType:  "market",
		Size:       positions[0].Total, // "880" base units
		Force:      "ioc",
		ReduceOnly: true,
		Price:      "0",
	})
	if err != nil {
		t.Fatalf("PlaceOrder: %v", err)
	}

	mu.Lock()
	gotSize := placedSize
	mu.Unlock()

	if gotSize != contracts {
		t.Errorf("PlaceOrder sent %d contracts, expected %d (base units %.0f / mult %.0f)",
			gotSize, contracts, baseUnits, quantoMult)
	}

	// Step 4: GetOrderFilledQty should return base asset units
	filled, err := adapter.GetOrderFilledQty("12345", internalSym)
	if err != nil {
		t.Fatalf("GetOrderFilledQty: %v", err)
	}
	if filled != baseUnits {
		t.Errorf("GetOrderFilledQty: expected %.0f base units, got %.0f", baseUnits, filled)
	}
}

func TestEnsureOneWayMode_UsesQueryParam(t *testing.T) {
	var gotPath string
	var gotQuery string
	var gotBody []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}

		var err error
		gotBody, err = io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery

		json.NewEncoder(w).Encode(map[string]interface{}{
			"in_dual_mode": false,
		})
	}))
	defer srv.Close()

	adapter := &Adapter{
		client: NewClientWithBase(srv.URL + "/api/v4"),
	}

	if err := adapter.EnsureOneWayMode(); err != nil {
		t.Fatalf("EnsureOneWayMode: %v", err)
	}

	if gotPath != "/api/v4/futures/usdt/dual_mode" {
		t.Fatalf("expected path /api/v4/futures/usdt/dual_mode, got %s", gotPath)
	}
	if gotQuery != "dual_mode=false" {
		t.Fatalf("expected query dual_mode=false, got %q", gotQuery)
	}
	if len(gotBody) != 0 {
		t.Fatalf("expected empty body, got %d bytes: %s", len(gotBody), string(gotBody))
	}
}

func TestGetSpotMarginOrder_ClosedIOCPartialIsNotMarkedFilled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4/spot/orders/1852454420" {
			http.NotFound(w, r)
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":             "1852454420",
			"status":         "closed",
			"finish_as":      "ioc",
			"currency_pair":  "BTC_USDT",
			"filled_amount":  "0.4",
			"avg_deal_price": "101.5",
		})
	}))
	defer srv.Close()

	adapter := &Adapter{
		client: NewClientWithBase(srv.URL + "/api/v4"),
	}

	status, err := adapter.GetSpotMarginOrder("1852454420", "BTCUSDT")
	if err != nil {
		t.Fatalf("GetSpotMarginOrder: %v", err)
	}
	if status == nil {
		t.Fatal("expected order status")
	}
	if status.Status != "cancelled" {
		t.Fatalf("status = %q, want cancelled", status.Status)
	}
	if status.FilledQty != 0.4 {
		t.Fatalf("filled qty = %.2f, want 0.4", status.FilledQty)
	}
	if status.AvgPrice != 101.5 {
		t.Fatalf("avg price = %.2f, want 101.5", status.AvgPrice)
	}
}

// TestMaintenanceRateNormalization_GateIO verifies that Gate.io maintenance_rate
// is parsed correctly from the contracts endpoint (already decimal: "0.005" = 0.5%).
func TestMaintenanceRateNormalization_GateIO(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v4/futures/usdt/contracts" {
			json.NewEncoder(w).Encode([]map[string]interface{}{
				{
					"name":              "BTC_USDT",
					"quanto_multiplier": "0.0001",
					"order_size_min":    1,
					"order_size_max":    1000000,
					"order_price_round": "0.1",
					"maintenance_rate":  "0.005",
				},
				{
					"name":              "GUA_USDT",
					"quanto_multiplier": "1",
					"order_size_min":    1,
					"order_size_max":    100000,
					"order_price_round": "0.0001",
					"maintenance_rate":  "0.3",
				},
			})
		} else {
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	adapter := &Adapter{
		client:    NewClientWithBase(srv.URL + "/api/v4"),
		priceSyms: make(map[string]bool),
	}

	contracts, err := adapter.LoadAllContracts()
	if err != nil {
		t.Fatalf("LoadAllContracts: %v", err)
	}

	// BTC: maintenance_rate "0.005" should be 0.005 (0.5%)
	btc, ok := contracts["BTCUSDT"]
	if !ok {
		t.Fatal("BTCUSDT not found")
	}
	if btc.MaintenanceRate != 0.005 {
		t.Errorf("BTCUSDT MaintenanceRate = %v, want 0.005", btc.MaintenanceRate)
	}

	// GUA: maintenance_rate "0.3" should be 0.3 (30%)
	gua, ok := contracts["GUAUSDT"]
	if !ok {
		t.Fatal("GUAUSDT not found")
	}
	if gua.MaintenanceRate != 0.3 {
		t.Errorf("GUAUSDT MaintenanceRate = %v, want 0.3", gua.MaintenanceRate)
	}
}

// TestGetMaintenanceRate_GateIO_TierMatching verifies tier-matched rate lookup
// from Gate.io risk_limit_tiers endpoint.
func TestGetMaintenanceRate_GateIO_TierMatching(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v4/futures/usdt/contracts":
			json.NewEncoder(w).Encode([]map[string]interface{}{{
				"name":              "BTC_USDT",
				"quanto_multiplier": "0.0001",
				"order_size_min":    1,
				"order_size_max":    1000000,
				"order_price_round": "0.1",
				"maintenance_rate":  "0.005",
			}})
		case r.URL.Path == "/api/v4/futures/usdt/risk_limit_tiers":
			// Tiered response: tier 1 up to 500k, tier 2 up to 2M
			json.NewEncoder(w).Encode([]map[string]interface{}{
				{
					"risk_limit":       500000,
					"maintenance_rate": "0.005",
				},
				{
					"risk_limit":       2000000,
					"maintenance_rate": "0.01",
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	adapter := &Adapter{
		client:    NewClientWithBase(srv.URL + "/api/v4"),
		priceSyms: make(map[string]bool),
	}
	// Load contracts first (to populate contractMult)
	adapter.LoadAllContracts()

	// Notional 100k should match tier 1 (0.005)
	rate, err := adapter.GetMaintenanceRate("BTCUSDT", 100000)
	if err != nil {
		t.Fatalf("GetMaintenanceRate: %v", err)
	}
	if rate != 0.005 {
		t.Errorf("tier 1 rate = %v, want 0.005", rate)
	}

	// Notional 600k should match tier 2 (0.01)
	rate2, err := adapter.GetMaintenanceRate("BTCUSDT", 600000)
	if err != nil {
		t.Fatalf("GetMaintenanceRate: %v", err)
	}
	if rate2 != 0.01 {
		t.Errorf("tier 2 rate = %v, want 0.01", rate2)
	}

	// Notional 0 should return tier 1 (lowest)
	rate0, err := adapter.GetMaintenanceRate("BTCUSDT", 0)
	if err != nil {
		t.Fatalf("GetMaintenanceRate: %v", err)
	}
	if rate0 != 0.005 {
		t.Errorf("notional=0 rate = %v, want 0.005 (tier 1)", rate0)
	}
}

// TestGetClosePnL_GateIO_QuantoMultiplierAppliedToCloseSize verifies that
// Gate.io's accum_size (raw contract count) is converted to base-asset units
// using the per-symbol quanto_multiplier before being returned in ClosePnL.
// This mirrors the OKX ctVal handling and is required so the reconcile
// Tier-1 completeness gate sees apples-to-apples (token) sizes.
func TestGetClosePnL_GateIO_QuantoMultiplierAppliedToCloseSize(t *testing.T) {
	const (
		quantoMult  = 10.0
		symbol      = "IR_USDT"
		internalSym = "IRUSDT"
		// Exchange reports raw contracts; adapter must multiply by 10 → 880.
		rawContracts = 88
		baseUnits    = float64(rawContracts) * quantoMult
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v4/futures/usdt/contracts":
			json.NewEncoder(w).Encode([]map[string]interface{}{{
				"name":              symbol,
				"quanto_multiplier": strconv.FormatFloat(quantoMult, 'f', -1, 64),
				"order_size_min":    1,
				"order_size_max":    1000000,
				"order_price_round": "0.0001",
			}})
		case "/api/v4/futures/usdt/position_close":
			json.NewEncoder(w).Encode([]map[string]interface{}{{
				"pnl":         "1.2345",
				"pnl_pnl":     "1.0",
				"pnl_fund":    "0.3",
				"pnl_fee":     "-0.0655",
				"side":        "long",
				"long_price":  "0.0400",
				"short_price": "0.0410",
				"accum_size":  strconv.Itoa(rawContracts),
				"time":        1700000000.0,
			}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	adapter := &Adapter{
		client:    NewClientWithBase(srv.URL + "/api/v4"),
		priceSyms: make(map[string]bool),
	}
	if _, err := adapter.LoadAllContracts(); err != nil {
		t.Fatalf("LoadAllContracts: %v", err)
	}

	pnls, err := adapter.GetClosePnL(internalSym, time.Unix(0, 0))
	if err != nil {
		t.Fatalf("GetClosePnL: %v", err)
	}
	if len(pnls) != 1 {
		t.Fatalf("expected 1 ClosePnL record, got %d", len(pnls))
	}
	if pnls[0].CloseSize != baseUnits {
		t.Errorf("CloseSize = %.4f, want %.4f (raw %d × quanto %v)",
			pnls[0].CloseSize, baseUnits, rawContracts, quantoMult)
	}
	if pnls[0].CloseSizeUnknown {
		t.Errorf("CloseSizeUnknown = true, want false (size was derivable)")
	}
}

func TestGetUnifiedBalanceSingleCurrencyPreservesZeroAvailableMargin(t *testing.T) {
	bal := getGateUnifiedBalanceForTest(t, map[string]interface{}{
		"mode":                            "single_currency",
		"unified_account_total_equity":    "",
		"total_available_margin":          "",
		"total_maintenance_margin":        "0",
		"unified_account_total_liability": "0",
		"balances": map[string]interface{}{
			"USDT": map[string]interface{}{
				"equity":           "100",
				"available":        "75",
				"available_margin": "0",
				"mm":               "5",
				"margin_balance":   "100",
			},
		},
	})

	if bal.Total != 100 {
		t.Fatalf("Total = %v, want 100", bal.Total)
	}
	if bal.Available != 0 {
		t.Fatalf("Available = %v, want 0; available_margin=\"0\" must not fall back to available", bal.Available)
	}
	if bal.MarginRatio != 0.05 {
		t.Fatalf("MarginRatio = %v, want 0.05", bal.MarginRatio)
	}
}

func TestGetUnifiedBalanceMultiCurrencyPreservesZeroTotalAvailableMargin(t *testing.T) {
	for _, mode := range []string{"multi_currency", "portfolio"} {
		t.Run(mode, func(t *testing.T) {
			bal := getGateUnifiedBalanceForTest(t, map[string]interface{}{
				"mode":                            mode,
				"unified_account_total_equity":    "100",
				"total_available_margin":          "0",
				"total_maintenance_margin":        "10",
				"unified_account_total_liability": "0",
				"balances": map[string]interface{}{
					"USDT": map[string]interface{}{
						"equity":    "100",
						"available": "80",
					},
				},
			})

			if bal.Available != 0 {
				t.Fatalf("Available = %v, want 0; total_available_margin=\"0\" must not fall back to balances.USDT.available", bal.Available)
			}
			if bal.MarginRatio != 0.1 {
				t.Fatalf("MarginRatio = %v, want 0.1", bal.MarginRatio)
			}
		})
	}
}

func TestGetUnifiedBalanceFallsBackOnlyWhenPrimaryAvailableFieldMissingOrEmpty(t *testing.T) {
	tests := []struct {
		name string
		body map[string]interface{}
		want float64
	}{
		{
			name: "single empty available_margin",
			body: map[string]interface{}{
				"mode":                         "single_currency",
				"unified_account_total_equity": "",
				"total_available_margin":       "",
				"balances": map[string]interface{}{
					"USDT": map[string]interface{}{
						"equity":           "100",
						"available":        "75",
						"available_margin": "",
					},
				},
			},
			want: 75,
		},
		{
			name: "single missing available_margin",
			body: map[string]interface{}{
				"mode":                         "single_currency",
				"unified_account_total_equity": "",
				"total_available_margin":       "",
				"balances": map[string]interface{}{
					"USDT": map[string]interface{}{
						"equity":    "100",
						"available": "76",
					},
				},
			},
			want: 76,
		},
		{
			name: "multi empty total_available_margin",
			body: map[string]interface{}{
				"mode":                         "multi_currency",
				"unified_account_total_equity": "100",
				"total_available_margin":       "",
				"balances": map[string]interface{}{
					"USDT": map[string]interface{}{
						"available": "80",
					},
				},
			},
			want: 80,
		},
		{
			name: "multi missing total_available_margin",
			body: map[string]interface{}{
				"mode":                         "multi_currency",
				"unified_account_total_equity": "100",
				"balances": map[string]interface{}{
					"USDT": map[string]interface{}{
						"available": "81",
					},
				},
			},
			want: 81,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bal := getGateUnifiedBalanceForTest(t, tt.body)
			if bal.Available != tt.want {
				t.Fatalf("Available = %v, want %v", bal.Available, tt.want)
			}
		})
	}
}

func getGateUnifiedBalanceForTest(t *testing.T, body map[string]interface{}) *exchange.Balance {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4/unified/accounts" {
			http.NotFound(w, r)
			return
		}
		json.NewEncoder(w).Encode(body)
	}))
	defer srv.Close()

	adapter := &Adapter{
		client: NewClientWithBase(srv.URL + "/api/v4"),
	}
	bal, err := adapter.getUnifiedBalance()
	if err != nil {
		t.Fatalf("getUnifiedBalance: %v", err)
	}
	return bal
}

// TestMaintenanceRate_GateIO_BoundsCheck verifies that invalid rates are rejected.
func TestMaintenanceRate_GateIO_BoundsCheck(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v4/futures/usdt/contracts":
			json.NewEncoder(w).Encode([]map[string]interface{}{{
				"name":              "BAD_USDT",
				"quanto_multiplier": "1",
				"order_size_min":    1,
				"order_size_max":    100000,
				"order_price_round": "0.01",
				"maintenance_rate":  "-0.5",
			}})
		case r.URL.Path == "/api/v4/futures/usdt/risk_limit_tiers":
			json.NewEncoder(w).Encode([]map[string]interface{}{
				{
					"risk_limit":       500000,
					"maintenance_rate": "1.5",
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	adapter := &Adapter{
		client:    NewClientWithBase(srv.URL + "/api/v4"),
		priceSyms: make(map[string]bool),
	}

	contracts, err := adapter.LoadAllContracts()
	if err != nil {
		t.Fatalf("LoadAllContracts: %v", err)
	}

	// Negative rate should be clamped to 0
	bad := contracts["BADUSDT"]
	if bad.MaintenanceRate != 0 {
		t.Errorf("negative rate should be 0, got %v", bad.MaintenanceRate)
	}

	// GetMaintenanceRate with rate >= 1.0 should return 0
	rate, err := adapter.GetMaintenanceRate("BADUSDT", 100000)
	if err != nil {
		t.Fatalf("GetMaintenanceRate: %v", err)
	}
	if rate != 0 {
		t.Errorf("rate >= 1.0 should return 0, got %v", rate)
	}
}
