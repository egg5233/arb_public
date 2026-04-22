package bingx

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"arb/pkg/exchange"
)

func TestGetPendingOrdersAndFilledQty_PrefixNormalize(t *testing.T) {
	var openOrdersSymbol string
	var detailSymbol string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/openApi/swap/v2/quote/contracts":
			json.NewEncoder(w).Encode(map[string]any{
				"code": 0,
				"data": []map[string]any{
					{
						"symbol":            "1000PEPE-USDT",
						"asset":             "1000PEPE",
						"size":              "1",
						"tradeMinQuantity":  "1",
						"pricePrecision":    4,
						"quantityPrecision": 0,
						"status":            1,
					},
				},
			})
		case "/openApi/swap/v2/trade/openOrders":
			openOrdersSymbol = r.URL.Query().Get("symbol")
			json.NewEncoder(w).Encode(map[string]any{
				"code": 0,
				"data": map[string]any{
					"orders": []map[string]any{
						{
							"orderId":       123,
							"clientOrderId": "cid-1",
							"symbol":        "1000PEPE-USDT",
							"side":          "BUY",
							"type":          "LIMIT",
							"price":         "0.003",
							"origQty":       "2",
							"status":        "PENDING",
						},
					},
				},
			})
		case "/openApi/swap/v2/trade/order":
			detailSymbol = r.URL.Query().Get("symbol")
			json.NewEncoder(w).Encode(map[string]any{
				"code": 0,
				"data": map[string]any{
					"order": map[string]any{
						"executedQty": "2",
						"avgPrice":    "0.003",
						"status":      "FILLED",
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
			httpClient: http.DefaultClient,
		},
	}

	orders, err := adapter.GetPendingOrders("PEPEUSDT")
	if err != nil {
		t.Fatalf("GetPendingOrders: %v", err)
	}
	if openOrdersSymbol != "1000PEPE-USDT" {
		t.Fatalf("open orders symbol = %q, want 1000PEPE-USDT", openOrdersSymbol)
	}
	if len(orders) != 1 {
		t.Fatalf("orders len = %d, want 1", len(orders))
	}
	if orders[0].Symbol != "PEPEUSDT" {
		t.Fatalf("order symbol = %q, want PEPEUSDT", orders[0].Symbol)
	}
	if orders[0].Price != "0.000003" {
		t.Fatalf("order price = %q, want 0.000003", orders[0].Price)
	}
	if orders[0].Size != "2000" {
		t.Fatalf("order size = %q, want 2000", orders[0].Size)
	}

	qty, err := adapter.GetOrderFilledQty("123", "PEPEUSDT")
	if err != nil {
		t.Fatalf("GetOrderFilledQty: %v", err)
	}
	if detailSymbol != "1000PEPE-USDT" {
		t.Fatalf("detail symbol = %q, want 1000PEPE-USDT", detailSymbol)
	}
	if qty != 2000 {
		t.Fatalf("filled qty = %v, want 2000", qty)
	}

	val, ok := adapter.orderStore.Load("123")
	if !ok {
		t.Fatal("orderStore missing entry")
	}
	update := val.(exchange.OrderUpdate)
	if update.FilledVolume != 2000 {
		t.Fatalf("stored filled volume = %v, want 2000", update.FilledVolume)
	}
	if update.AvgPrice != 0.000003 {
		t.Fatalf("stored avg price = %v, want 0.000003", update.AvgPrice)
	}
}
