// Package binance — unit tests for ws_private.go ORDER_TRADE_UPDATE and ALGO_UPDATE
// parsing, covering the H3B.1 field-tag fix and H3B.2 algo-remap callback.
package binance

import (
	"encoding/json"
	"strconv"
	"testing"

	"arb/pkg/exchange"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// buildOrderTradeUpdateMsg builds a minimal ORDER_TRADE_UPDATE JSON payload
// matching the Binance USDT-M futures private stream format.
// orderIDInt is the numeric order ID (field "i" is int64 in the adapter struct).
func buildOrderTradeUpdateMsg(t *testing.T, symbol string, orderIDInt int64, status string,
	filledQty float64, reduceOnly, closePosition bool, origOrderType string) []byte {
	t.Helper()
	msg := map[string]interface{}{
		"e": "ORDER_TRADE_UPDATE",
		"E": int64(1713312000000),
		"o": map[string]interface{}{
			"s":  symbol,
			"c":  "client-oid-1",
			"S":  "SELL",
			"o":  origOrderType,
			"X":  status,
			"i":  orderIDInt, // int64 so JSON unmarshal works
			"ap": "1.5900",
			"z":  strconv.FormatFloat(filledQty, 'f', -1, 64),
			"R":  reduceOnly, // field "R" — was "ro" (bug fixed in H3B.1)
			"q":  "189",
			"cp": closePosition, // NEW field added in H3B.1
			"ot": origOrderType, // NEW field added in H3B.1
		},
	}
	b, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("buildOrderTradeUpdateMsg marshal: %v", err)
	}
	return b
}

// captureOrderUpdate wires SetOrderCallback and handlePrivateMessage, then
// returns the most recently stored OrderUpdate for the given orderID.
func captureOrderUpdate(t *testing.T, adapter *Adapter, msg []byte, orderID string) (exchange.OrderUpdate, bool) {
	t.Helper()
	var got exchange.OrderUpdate
	var gotOK bool
	adapter.SetOrderCallback(func(upd exchange.OrderUpdate) {
		// Only capture completed fills (status=filled).
		if upd.OrderID == orderID {
			got = upd
			gotOK = true
		}
	})
	adapter.handlePrivateMessage(msg)
	// Also check the order store directly.
	if stored, ok := adapter.GetOrderUpdate(orderID); ok {
		return stored, true
	}
	return got, gotOK
}

// newBareAdapter builds an Adapter with no live connections — sufficient for
// unit-testing handlePrivateMessage.
func newBareAdapter() *Adapter {
	return &Adapter{
		priceSyms: make(map[string]bool),
		depthSyms: make(map[string]bool),
	}
}

// ---------------------------------------------------------------------------
// H3B.1 — TestBinanceWSClosePositionDetection
//
// Payload: R=false, cp=true, ot=TAKE_PROFIT_MARKET
// Expected: upd.ReduceOnly = true  (cp=true triggers the narrow heuristic)
// ---------------------------------------------------------------------------

func TestBinanceWSClosePositionDetection(t *testing.T) {
	adapter := newBareAdapter()

	// Binance TP fired via closePosition=true (PlaceStopLoss path uses cp=true).
	// R=false because the algo order itself was not marked reduceOnly at placement.
	msg := buildOrderTradeUpdateMsg(t, "SIRENUSDT", 3561813146, "FILLED", 189.0,
		false, /* R */
		true,  /* cp */
		"TAKE_PROFIT_MARKET" /* ot */)

	upd, ok := captureOrderUpdate(t, adapter, msg, "3561813146")
	if !ok {
		t.Fatal("ORDER_TRADE_UPDATE not captured in order store")
	}
	if !upd.ReduceOnly {
		t.Errorf("ReduceOnly = false, want true (cp=true + ot=TAKE_PROFIT_MARKET should set ReduceOnly)")
	}
}

// ---------------------------------------------------------------------------
// H3B.1 — TestBinanceWSNormalFillStaysNonReduce
//
// Payload: R=false, cp=false, ot=LIMIT
// Expected: upd.ReduceOnly = false  (no false positive for normal limit fill)
// ---------------------------------------------------------------------------

func TestBinanceWSNormalFillStaysNonReduce(t *testing.T) {
	adapter := newBareAdapter()

	msg := map[string]interface{}{
		"e": "ORDER_TRADE_UPDATE",
		"E": int64(1713312000000),
		"o": map[string]interface{}{
			"s":  "BTCUSDT",
			"c":  "client-oid-2",
			"S":  "BUY",
			"o":  "LIMIT",
			"X":  "FILLED",
			"i":  int64(12345678), // must be int64 so strconv.FormatInt works correctly
			"ap": "65000.00",
			"z":  "0.001",
			"R":  false, // R=false
			"q":  "0.001",
			"cp": false,   // cp=false — normal entry order
			"ot": "LIMIT", // ot=LIMIT
		},
	}
	b, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	adapter.handlePrivateMessage(b)
	upd, ok := adapter.GetOrderUpdate("12345678")
	if !ok {
		t.Fatal("ORDER_TRADE_UPDATE not captured in order store")
	}
	if upd.ReduceOnly {
		t.Errorf("ReduceOnly = true, want false (normal LIMIT fill must not be classified as reduce-only)")
	}
}

// ---------------------------------------------------------------------------
// H3B.2 — TestBinanceALGOUPDATETriggersRemap
//
// Feed an ALGO_UPDATE message with AlgoStatus=TRIGGERED + RealID populated.
// Verify the registered AlgoRemapCallback receives the correct AlgoRemap struct.
// ---------------------------------------------------------------------------

func TestBinanceALGOUPDATETriggersRemap(t *testing.T) {
	adapter := newBareAdapter()

	var got exchange.AlgoRemap
	var gotCalled bool
	adapter.SetAlgoRemapCallback(func(remap exchange.AlgoRemap) {
		got = remap
		gotCalled = true
	})

	msg := map[string]interface{}{
		"e": "ALGO_UPDATE",
		"E": 1713312000000,
		"o": map[string]interface{}{
			"aid": int64(3000001251209638),
			"ai":  "3561813146",
			"s":   "SIRENUSDT",
			"X":   "TRIGGERED",
		},
	}
	b, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	adapter.handlePrivateMessage(b)

	if !gotCalled {
		t.Fatal("AlgoRemapCallback was not called for ALGO_UPDATE TRIGGERED")
	}
	if got.RealID != "3561813146" {
		t.Errorf("RealID = %q, want %q", got.RealID, "3561813146")
	}
	if got.Symbol != "SIRENUSDT" {
		t.Errorf("Symbol = %q, want %q", got.Symbol, "SIRENUSDT")
	}
	// AlgoID should be the string representation of aid=3000001251209638.
	if got.AlgoID == "" {
		t.Error("AlgoID is empty, want non-empty string from aid field")
	}
}

// TestBinanceALGOUPDATENotTriggeredNoCallback verifies that ALGO_UPDATE with
// AlgoStatus != TRIGGERED does NOT fire the remap callback.
func TestBinanceALGOUPDATENotTriggeredNoCallback(t *testing.T) {
	adapter := newBareAdapter()

	called := false
	adapter.SetAlgoRemapCallback(func(remap exchange.AlgoRemap) {
		called = true
	})

	msg := map[string]interface{}{
		"e": "ALGO_UPDATE",
		"E": 1713312000000,
		"o": map[string]interface{}{
			"aid": int64(123456),
			"ai":  "",
			"s":   "BTCUSDT",
			"X":   "NEW", // not TRIGGERED
		},
	}
	b, _ := json.Marshal(msg)
	adapter.handlePrivateMessage(b)

	if called {
		t.Error("AlgoRemapCallback should NOT fire for non-TRIGGERED ALGO_UPDATE")
	}
}
