package gateio

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"

	"arb/pkg/exchange"
)

// TestPlaceOrder_MarketDefaultsTifIOC is a regression test for the
// 2026-04-24 pricegap closeLegMarket failure on Gate.io:
//
//	gateio API error label=INVALID_ARGUMENT
//	message=market order without IOC or FOK
//
// Gate.io rejects futures market orders (price "0") unless tif is "ioc"
// or "fok". The adapter previously defaulted tif="gtc" for every order
// regardless of OrderType. When pricegap's closeLegMarket submitted
// OrderType="market" with Force="" the adapter passed tif=gtc, and Gate.io
// rejected. Caller's explicit Force still wins; this test only pins the
// default-for-market behavior.
func TestPlaceOrder_MarketDefaultsTifIOC(t *testing.T) {
	const (
		symbol      = "SOON_USDT"
		internalSym = "SOONUSDT"
	)

	var capturedTif string
	var capturedPrice string
	var mu sync.Mutex

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v4/futures/usdt/contracts":
			json.NewEncoder(w).Encode([]map[string]interface{}{{
				"name":              symbol,
				"quanto_multiplier": "1",
				"order_size_min":    1,
				"order_size_max":    1000000,
				"order_price_round": "0.00001",
			}})

		case r.URL.Path == "/api/v4/futures/usdt/orders" && r.Method == "POST":
			var body struct {
				Tif   string `json:"tif"`
				Price string `json:"price"`
			}
			json.NewDecoder(r.Body).Decode(&body)
			mu.Lock()
			capturedTif = body.Tif
			capturedPrice = body.Price
			mu.Unlock()
			json.NewEncoder(w).Encode(map[string]interface{}{"id": 99, "text": ""})

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

	// PRICE CASE: closeLegMarket path — market order, no Force, no Price.
	_, err := adapter.PlaceOrder(exchange.PlaceOrderParams{
		Symbol:     internalSym,
		Side:       exchange.SideSell,
		OrderType:  "market",
		Size:       strconv.FormatFloat(100, 'f', -1, 64),
		ReduceOnly: true,
	})
	if err != nil {
		t.Fatalf("PlaceOrder: %v", err)
	}

	mu.Lock()
	tif := capturedTif
	price := capturedPrice
	mu.Unlock()

	if tif != "ioc" {
		t.Fatalf("market order default tif: got %q, want %q (Gate.io requires ioc/fok for market)", tif, "ioc")
	}
	if price != "0" {
		t.Fatalf("market order price: got %q, want %q", price, "0")
	}
}

// TestPlaceOrder_ExplicitForceStillWins guards against over-correction:
// callers that explicitly set Force="fok" or Force="gtc" (on a limit order)
// must still see their choice preserved.
func TestPlaceOrder_ExplicitForceStillWins(t *testing.T) {
	const (
		symbol      = "SOON_USDT"
		internalSym = "SOONUSDT"
	)

	var capturedTif string
	var mu sync.Mutex

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v4/futures/usdt/contracts":
			json.NewEncoder(w).Encode([]map[string]interface{}{{
				"name":              symbol,
				"quanto_multiplier": "1",
				"order_size_min":    1,
				"order_size_max":    1000000,
				"order_price_round": "0.00001",
			}})

		case r.URL.Path == "/api/v4/futures/usdt/orders" && r.Method == "POST":
			var body struct {
				Tif string `json:"tif"`
			}
			json.NewDecoder(r.Body).Decode(&body)
			mu.Lock()
			capturedTif = body.Tif
			mu.Unlock()
			json.NewEncoder(w).Encode(map[string]interface{}{"id": 99})

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

	// Market order with explicit Force="fok" → must send fok, not the
	// ioc default the regression fix introduced.
	_, err := adapter.PlaceOrder(exchange.PlaceOrderParams{
		Symbol:    internalSym,
		Side:      exchange.SideSell,
		OrderType: "market",
		Size:      "100",
		Force:     "fok",
	})
	if err != nil {
		t.Fatalf("PlaceOrder: %v", err)
	}

	mu.Lock()
	tif := capturedTif
	mu.Unlock()

	if tif != "fok" {
		t.Fatalf("explicit Force=fok should win: got tif=%q, want %q", tif, "fok")
	}
}
