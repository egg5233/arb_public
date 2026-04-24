package bingx

import (
	"sync"
	"testing"

	"arb/pkg/exchange"
)

// TestHandleBookTicker_UppercaseDoesNotOverwritePrice is a regression test for
// the 2026-04-24 bug where case-insensitive JSON matching on the BingX public
// bookTicker payload caused `B` (bid qty) and `A` (ask qty) to overwrite
// `b` (bid price) and `a` (ask price). The handler claimed only tags "b" and
// "a", but Go's encoding/json falls back to case-insensitive matching, so the
// later-emitted uppercase quantity fields clobbered the earlier-emitted
// lowercase price fields.
//
// Symptom: pricegap BBO liveness logs showed SOON@bingx bid=24565 ask=28345
// (quantities in native tokens) instead of SOON's real bid/ask around $0.18.
func TestHandleBookTicker_UppercaseDoesNotOverwritePrice(t *testing.T) {
	ws := &PublicWS{priceStore: &sync.Map{}}

	// Real BingX payload shape. Ordering matters — uppercase fields come AFTER
	// lowercase, so case-insensitive overwrites reproduce the original bug.
	payload := []byte(`{"code":0,"dataType":"SOON-USDT@bookTicker","data":{"e":"bookTicker","u":1,"E":1,"T":1,"s":"SOON-USDT","b":"0.18140","B":"10128.78","a":"0.18160","A":"79448.90"}}`)

	ws.handleBookTickerMessage(payload, "SOON-USDT@bookTicker")

	val, ok := ws.priceStore.Load("SOONUSDT")
	if !ok {
		t.Fatalf("priceStore has no SOONUSDT entry")
	}
	bbo, ok := val.(exchange.BBO)
	if !ok {
		t.Fatalf("priceStore entry is not exchange.BBO: %T", val)
	}

	if bbo.Bid != 0.18140 {
		t.Fatalf("Bid: got %v, want 0.18140 (price, not quantity)", bbo.Bid)
	}
	if bbo.Ask != 0.18160 {
		t.Fatalf("Ask: got %v, want 0.18160 (price, not quantity)", bbo.Ask)
	}
	if bbo.Ask <= bbo.Bid {
		t.Fatalf("crossed book: bid=%v ask=%v", bbo.Bid, bbo.Ask)
	}
}
