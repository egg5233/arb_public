package okx

import (
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"

	"arb/pkg/exchange"
)

// TestCtValRoundTrip verifies that OKX position sizes are returned in base
// asset units (not raw contracts), and that passing the size back to PlaceOrder
// produces the correct contract count.
//
// OKX PIPPINUSDT has ctVal=10, meaning 1 contract = 10 PIPPIN.
// GetPosition should return 480 (base units) for 48 contracts.
// PlaceOrder should convert 480 back to 48 contracts.
// GetOrderFilledQty should return 480 base units for 48 contracts filled.
// WS order updates should also convert to base units.
func TestCtValRoundTrip(t *testing.T) {
	const (
		ctVal       = 10.0
		contracts   = 48
		baseUnits   = float64(contracts) * ctVal // 480
		instID      = "PIPPIN-USDT-SWAP"
		internalSym = "PIPPINUSDT"
	)

	var placedSz string
	var mu sync.Mutex

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// All OKX responses use envelope: {"code":"0","msg":"","data":[...]}
		respond := func(data interface{}) {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"code": "0",
				"msg":  "",
				"data": data,
			})
		}

		switch {
		case r.URL.Path == "/api/v5/public/instruments":
			respond([]map[string]interface{}{{
				"instId":    instID,
				"tickSz":    "0.00001",
				"lotSz":     "1",
				"minSz":     "1",
				"ctVal":     strconv.FormatFloat(ctVal, 'f', -1, 64),
				"state":     "live",
				"settleCcy": "USDT",
			}})

		case r.URL.Path == "/api/v5/account/positions":
			respond([]map[string]interface{}{{
				"instId":   instID,
				"posSide":  "net",
				"pos":      strconv.Itoa(-contracts), // negative = short (net mode)
				"availPos": strconv.Itoa(contracts),
				"avgPx":    "0.17500",
				"upl":      "-0.50",
				"lever":    "2",
				"mgnMode":  "cross",
				"liqPx":    "0.25000",
				"markPx":   "0.17600",
			}})

		case r.URL.Path == "/api/v5/trade/order" && r.Method == "POST":
			var req struct {
				Sz string `json:"sz"`
			}
			json.NewDecoder(r.Body).Decode(&req)
			mu.Lock()
			placedSz = req.Sz
			mu.Unlock()
			respond([]map[string]interface{}{{
				"ordId":   "123456",
				"clOrdId": "",
				"sCode":   "0",
				"sMsg":    "",
			}})

		case r.URL.Path == "/api/v5/trade/order" && r.Method == "GET":
			respond([]map[string]interface{}{{
				"fillSz":    strconv.Itoa(contracts),
				"accFillSz": strconv.Itoa(contracts),
			}})

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

	// Step 1: Load contracts to populate ctValCache
	contracts_info, err := adapter.LoadAllContracts()
	if err != nil {
		t.Fatalf("LoadAllContracts: %v", err)
	}
	if _, ok := contracts_info[internalSym]; !ok {
		t.Fatalf("expected contract info for %s", internalSym)
	}

	// Verify ctVal was cached
	gotCtVal := adapter.getCtVal(internalSym)
	if gotCtVal != ctVal {
		t.Fatalf("expected ctVal=%v, got %v", ctVal, gotCtVal)
	}

	// Step 2: GetPosition should return base asset units
	positions, err := adapter.GetPosition(internalSym)
	if err != nil {
		t.Fatalf("GetPosition: %v", err)
	}
	if len(positions) != 1 {
		t.Fatalf("expected 1 position, got %d", len(positions))
	}

	posSize, _ := strconv.ParseFloat(positions[0].Total, 64)
	if posSize != baseUnits {
		t.Errorf("GetPosition size: expected %.0f base units, got %.0f (contracts=%d × ctVal=%.0f)",
			baseUnits, posSize, contracts, ctVal)
	}
	if positions[0].HoldSide != "short" {
		t.Errorf("expected holdSide=short, got %s", positions[0].HoldSide)
	}

	// Step 3: GetAllPositions should also return base units
	allPos, err := adapter.GetAllPositions()
	if err != nil {
		t.Fatalf("GetAllPositions: %v", err)
	}
	if len(allPos) != 1 {
		t.Fatalf("expected 1 position, got %d", len(allPos))
	}
	allPosSize, _ := strconv.ParseFloat(allPos[0].Total, 64)
	if allPosSize != baseUnits {
		t.Errorf("GetAllPositions size: expected %.0f, got %.0f", baseUnits, allPosSize)
	}

	// Step 4: PlaceOrder with base units should send correct contract count
	_, err = adapter.PlaceOrder(exchange.PlaceOrderParams{
		Symbol:     internalSym,
		Side:       exchange.SideBuy,
		OrderType:  "market",
		Size:       positions[0].Total, // "480" base units
		Force:      "ioc",
		ReduceOnly: true,
	})
	if err != nil {
		t.Fatalf("PlaceOrder: %v", err)
	}

	mu.Lock()
	gotSz := placedSz
	mu.Unlock()

	expectedContracts := strconv.Itoa(contracts)
	if gotSz != expectedContracts {
		t.Errorf("PlaceOrder sent sz=%s, expected %s (base=%.0f / ctVal=%.0f)",
			gotSz, expectedContracts, baseUnits, ctVal)
	}

	// Step 5: GetOrderFilledQty should return base units
	filled, err := adapter.GetOrderFilledQty("123456", internalSym)
	if err != nil {
		t.Fatalf("GetOrderFilledQty: %v", err)
	}
	if filled != baseUnits {
		t.Errorf("GetOrderFilledQty: expected %.0f base units, got %.0f", baseUnits, filled)
	}

	// Step 6: WS order update should convert to base units
	wsMsg := []byte(`{
		"arg": {"channel": "orders", "instType": "SWAP"},
		"data": [{
			"instId": "` + instID + `",
			"ordId": "789",
			"clOrdId": "",
			"state": "filled",
			"fillSz": "` + strconv.Itoa(contracts) + `",
			"accFillSz": "` + strconv.Itoa(contracts) + `",
			"avgPx": "0.17500"
		}]
	}`)
	adapter.orderStore = sync.Map{}
	adapter.handleOrderMessage(wsMsg)

	upd, ok := adapter.orderStore.Load("789")
	if !ok {
		t.Fatal("WS order update not stored")
	}
	wsUpdate := upd.(exchange.OrderUpdate)
	if math.Abs(wsUpdate.FilledVolume-baseUnits) > 0.01 {
		t.Errorf("WS FilledVolume: expected %.0f base units, got %.0f", baseUnits, wsUpdate.FilledVolume)
	}
}

func TestGetFuturesBalance_OKX_TrustsZeroAvailEq(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		respond := func(data interface{}) {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"code": "0",
				"msg":  "",
				"data": data,
			})
		}

		switch r.URL.Path {
		case "/api/v5/account/balance":
			respond([]map[string]interface{}{{
				"mgnRatio": "",
				"details": []map[string]interface{}{
					{
						"ccy":       "USDT",
						"eq":        "100",
						"availEq":   "0",
						"availBal":  "77",
						"frozenBal": "0",
						"mmr":       "5",
					},
				},
			}})
		case "/api/v5/account/max-withdrawal":
			respond([]map[string]interface{}{})
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

	bal, err := adapter.GetFuturesBalance()
	if err != nil {
		t.Fatalf("GetFuturesBalance: %v", err)
	}
	if bal.Available != 0 {
		t.Fatalf("Available = %v, want 0", bal.Available)
	}
	if bal.Total != 100 {
		t.Fatalf("Total = %v, want 100", bal.Total)
	}
}

func TestGetFuturesBalance_OKX_FallsBackToAvailBalWhenAvailEqEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		respond := func(data interface{}) {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"code": "0",
				"msg":  "",
				"data": data,
			})
		}

		switch r.URL.Path {
		case "/api/v5/account/balance":
			respond([]map[string]interface{}{{
				"mgnRatio": "",
				"details": []map[string]interface{}{
					{
						"ccy":       "USDT",
						"eq":        "100",
						"availEq":   "",
						"availBal":  "42.25",
						"frozenBal": "0",
						"mmr":       "5",
					},
				},
			}})
		case "/api/v5/account/max-withdrawal":
			respond([]map[string]interface{}{})
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

	bal, err := adapter.GetFuturesBalance()
	if err != nil {
		t.Fatalf("GetFuturesBalance: %v", err)
	}
	if math.Abs(bal.Available-42.25) > 1e-9 {
		t.Fatalf("Available = %v, want 42.25", bal.Available)
	}
}
