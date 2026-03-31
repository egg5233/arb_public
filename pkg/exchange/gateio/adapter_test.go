package gateio

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"

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

func TestEnsureOneWayMode_UsesJSONBody(t *testing.T) {
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
	if gotQuery != "" {
		t.Fatalf("expected no query string, got %q", gotQuery)
	}

	var payload struct {
		DualMode bool `json:"dual_mode"`
	}
	if err := json.Unmarshal(gotBody, &payload); err != nil {
		t.Fatalf("unmarshal request body: %v", err)
	}
	if payload.DualMode {
		t.Fatalf("expected dual_mode=false, got true")
	}
}
