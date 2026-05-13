package bybit

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"arb/pkg/exchange"
)

func TestTestOrderPostsIOCProbeAndCancels(t *testing.T) {
	var createSeen bool
	var cancelSeen bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v5/order/create":
			createSeen = true
			if r.Method != http.MethodPost {
				t.Fatalf("create method = %s, want POST", r.Method)
			}
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode create body: %v", err)
			}
			want := map[string]string{
				"category":    "linear",
				"symbol":      "HOODUSDT",
				"side":        "Sell",
				"orderType":   "Limit",
				"qty":         "1.00",
				"price":       "116.40",
				"timeInForce": "IOC",
				"orderLinkId": "probe-1",
			}
			for key, value := range want {
				if got := body[key]; got != value {
					t.Fatalf("create %s = %q, want %q (body=%v)", key, got, value, body)
				}
			}
			if _, ok := body["reduceOnly"]; ok {
				t.Fatalf("create body contains reduceOnly: %v", body)
			}
			_, _ = w.Write([]byte(`{"retCode":0,"retMsg":"OK","result":{"orderId":"probe-order","orderLinkId":"probe-1"}}`))
		case "/v5/order/cancel":
			cancelSeen = true
			if r.Method != http.MethodPost {
				t.Fatalf("cancel method = %s, want POST", r.Method)
			}
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode cancel body: %v", err)
			}
			if body["category"] != "linear" || body["symbol"] != "HOODUSDT" || body["orderId"] != "probe-order" {
				t.Fatalf("cancel body = %v", body)
			}
			_, _ = w.Write([]byte(`{"retCode":0,"retMsg":"OK","result":{}}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	adapter := NewAdapter(exchange.ExchangeConfig{ApiKey: "key", SecretKey: "secret"})
	adapter.client.baseURL = server.URL
	adapter.client.httpClient = server.Client()

	err := adapter.TestOrder(exchange.PlaceOrderParams{
		Symbol:    "HOODUSDT",
		Side:      exchange.SideSell,
		OrderType: "limit",
		Price:     "116.40",
		Size:      "1.00",
		Force:     "ioc",
		ClientOid: "probe-1",
	})
	if err != nil {
		t.Fatalf("TestOrder returned error: %v", err)
	}
	if !createSeen || !cancelSeen {
		t.Fatalf("createSeen=%v cancelSeen=%v, want both true", createSeen, cancelSeen)
	}
}

func TestTestOrderReturnsAgreementError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"retCode":110126,"retMsg":"You must sign the required agreement before trading this contract.","result":{}}`))
	}))
	defer server.Close()

	adapter := NewAdapter(exchange.ExchangeConfig{ApiKey: "key", SecretKey: "secret"})
	adapter.client.baseURL = server.URL
	adapter.client.httpClient = server.Client()

	err := adapter.TestOrder(exchange.PlaceOrderParams{Symbol: "HOODUSDT", Side: exchange.SideSell, OrderType: "limit", Price: "116.40", Size: "1.00", Force: "ioc"})
	if err == nil {
		t.Fatal("TestOrder returned nil error, want agreement error")
	}
	if !strings.Contains(err.Error(), "110126") || !strings.Contains(err.Error(), "required agreement") {
		t.Fatalf("TestOrder error = %v, want 110126 agreement", err)
	}
}

func TestTestOrderReturnsErrorWhenProbeOrderIDMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"retCode":0,"retMsg":"OK","result":{"orderLinkId":"probe-1"}}`))
	}))
	defer server.Close()

	adapter := NewAdapter(exchange.ExchangeConfig{ApiKey: "key", SecretKey: "secret"})
	adapter.client.baseURL = server.URL
	adapter.client.httpClient = server.Client()

	err := adapter.TestOrder(exchange.PlaceOrderParams{Symbol: "HOODUSDT", Side: exchange.SideSell, OrderType: "limit", Price: "116.40", Size: "1.00", Force: "ioc"})
	if err == nil || !strings.Contains(err.Error(), "missing probe order id") {
		t.Fatalf("TestOrder error = %v, want missing probe order id", err)
	}
}

func TestTestOrderRejectsUnsafeProbeParamsWithoutSending(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("unexpected request to %s", r.URL.Path)
	}))
	defer server.Close()

	adapter := NewAdapter(exchange.ExchangeConfig{ApiKey: "key", SecretKey: "secret"})
	adapter.client.baseURL = server.URL
	adapter.client.httpClient = server.Client()

	tests := []exchange.PlaceOrderParams{
		{Symbol: "HOODUSDT", Side: exchange.SideSell, OrderType: "market", Price: "116.40", Size: "1.00", Force: "ioc"},
		{Symbol: "HOODUSDT", Side: exchange.SideSell, OrderType: "limit", Price: "116.40", Size: "1.00", Force: "gtc"},
		{Symbol: "HOODUSDT", Side: exchange.SideSell, OrderType: "limit", Price: "0", Size: "1.00", Force: "ioc"},
		{Symbol: "HOODUSDT", Side: exchange.SideSell, OrderType: "limit", Price: "116.40", Size: "0", Force: "ioc"},
		{Symbol: "HOODUSDT", Side: exchange.SideSell, OrderType: "limit", Price: "116.40", Size: "1.00", Force: "ioc", ReduceOnly: true},
	}
	for _, tc := range tests {
		if err := adapter.TestOrder(tc); err == nil {
			t.Fatalf("TestOrder(%+v) returned nil error, want unsafe probe rejection", tc)
		}
	}
}
