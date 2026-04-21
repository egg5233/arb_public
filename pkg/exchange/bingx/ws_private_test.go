// Package bingx — unit tests for ws_private.go ORDER_TRADE_UPDATE parsing,
// covering the H3C fix: TAKE_PROFIT_MARKET order type sets ReduceOnly=true.
package bingx

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"sync"
	"testing"

	"arb/pkg/exchange"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// gzipEncode compresses data as the real BingX private WS does.
func gzipEncode(t *testing.T, data []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if _, err := w.Write(data); err != nil {
		t.Fatalf("gzip write: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	return buf.Bytes()
}

// newTestPrivateWS creates a PrivateWS wired to a local order store and fill
// callback — no network connections.
func newTestPrivateWS() (*PrivateWS, *sync.Map, *[]exchange.OrderUpdate) {
	store := &sync.Map{}
	var fills []exchange.OrderUpdate
	var mu sync.Mutex
	onFillFn := func(upd exchange.OrderUpdate) {
		mu.Lock()
		fills = append(fills, upd)
		mu.Unlock()
	}
	ws := NewPrivateWS(nil, store, &onFillFn)
	return ws, store, &fills
}

// buildBingXOrderMsg builds a BingX ORDER_TRADE_UPDATE JSON payload.
func buildBingXOrderMsg(t *testing.T, orderID, status, orderType string,
	filledQty float64, reduceOnly bool) []byte {
	t.Helper()
	msg := map[string]interface{}{
		"e": "ORDER_TRADE_UPDATE",
		"o": map[string]interface{}{
			"s":  "SIREN-USDT",
			"S":  "SELL",
			"X":  status,
			"i":  orderID,
			"c":  "client-" + orderID,
			"q":  "189",
			"ap": "1.5900",
			"z":  "189",
			"ro": reduceOnly,
			"o":  orderType,
		},
	}
	b, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("buildBingXOrderMsg marshal: %v", err)
	}
	return b
}

// ---------------------------------------------------------------------------
// H3C — TestBingxWSTakeProfitMarketIsReduce
//
// BingX payload OrderType=TAKE_PROFIT_MARKET → verify reduceOnly=true
// even when the explicit ro flag is false.
// ---------------------------------------------------------------------------

func TestBingxWSTakeProfitMarketIsReduce(t *testing.T) {
	ws, store, _ := newTestPrivateWS()

	raw := buildBingXOrderMsg(t, "bingx-tp-111", "FILLED", "TAKE_PROFIT_MARKET", 189.0, false)
	// handleMessage accepts raw (uncompressed) bytes directly.
	ws.handleMessage(raw)

	val, ok := store.Load("bingx-tp-111")
	if !ok {
		t.Fatal("order update not stored in orderStore")
	}
	upd := val.(exchange.OrderUpdate)
	if !upd.ReduceOnly {
		t.Errorf("ReduceOnly = false, want true for TAKE_PROFIT_MARKET order type")
	}
}

// TestBingxWSStopMarketIsReduce verifies the existing STOP_MARKET path still works.
func TestBingxWSStopMarketIsReduce(t *testing.T) {
	ws, store, _ := newTestPrivateWS()

	raw := buildBingXOrderMsg(t, "bingx-sl-222", "FILLED", "STOP_MARKET", 189.0, false)
	ws.handleMessage(raw)

	val, ok := store.Load("bingx-sl-222")
	if !ok {
		t.Fatal("order update not stored for STOP_MARKET")
	}
	upd := val.(exchange.OrderUpdate)
	if !upd.ReduceOnly {
		t.Errorf("ReduceOnly = false, want true for STOP_MARKET order type")
	}
}

// TestBingxWSExplicitReduceOnlyFlagHonoured verifies that when ro=true is set
// explicitly (on any order type), ReduceOnly is true.
func TestBingxWSExplicitReduceOnlyFlagHonoured(t *testing.T) {
	ws, store, _ := newTestPrivateWS()

	raw := buildBingXOrderMsg(t, "bingx-ro-333", "FILLED", "LIMIT", 100.0, true)
	ws.handleMessage(raw)

	val, ok := store.Load("bingx-ro-333")
	if !ok {
		t.Fatal("order update not stored")
	}
	upd := val.(exchange.OrderUpdate)
	if !upd.ReduceOnly {
		t.Errorf("ReduceOnly = false, want true when ro=true")
	}
}

// TestBingxWSNormalLimitFillNotReduce verifies that a normal LIMIT fill with
// ro=false does NOT get classified as reduce-only (no false positive).
func TestBingxWSNormalLimitFillNotReduce(t *testing.T) {
	ws, store, _ := newTestPrivateWS()

	raw := buildBingXOrderMsg(t, "bingx-limit-444", "FILLED", "LIMIT", 50.0, false)
	ws.handleMessage(raw)

	val, ok := store.Load("bingx-limit-444")
	if !ok {
		t.Fatal("order update not stored")
	}
	upd := val.(exchange.OrderUpdate)
	if upd.ReduceOnly {
		t.Errorf("ReduceOnly = true, want false for normal LIMIT fill")
	}
}
