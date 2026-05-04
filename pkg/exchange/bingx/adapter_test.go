package bingx

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"arb/pkg/exchange"
)

func TestTestOrderPostsNonMarketableIOCToLiveOrderEndpoint(t *testing.T) {
	var seen bool
	var deleteSeen bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			deleteSeen = true
			if r.URL.Path != "/openApi/swap/v2/trade/order" {
				t.Fatalf("delete path = %s, want /openApi/swap/v2/trade/order", r.URL.Path)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":109421,"msg":"order not exist","data":{}}`))
			return
		}

		seen = true

		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/openApi/swap/v2/trade/order" {
			t.Fatalf("path = %s, want /openApi/swap/v2/trade/order", r.URL.Path)
		}
		if got := r.Header.Get("X-BX-APIKEY"); got != "test-key" {
			t.Fatalf("X-BX-APIKEY = %q, want test-key", got)
		}
		if got := r.Header.Get("Content-Type"); !strings.HasPrefix(got, "application/x-www-form-urlencoded") {
			t.Fatalf("Content-Type = %q, want application/x-www-form-urlencoded", got)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm: %v", err)
		}

		want := map[string]string{
			"symbol":       "SIREN-USDT",
			"type":         "LIMIT",
			"side":         "BUY",
			"quantity":     "79",
			"price":        "0.0068447",
			"timeInForce":  "IOC",
			"positionSide": "BOTH",
		}
		for key, value := range want {
			if got := r.Form.Get(key); got != value {
				t.Fatalf("%s = %q, want %q", key, got, value)
			}
		}
		if got := r.Form.Get("signature"); got == "" {
			t.Fatal("signature is empty")
		}
		if got := r.Form.Get("timestamp"); got == "" {
			t.Fatal("timestamp is empty")
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"msg":"","data":{"order":{"orderId":123456789}}}`))
	}))
	defer server.Close()

	adapter := NewAdapter(exchange.ExchangeConfig{
		ApiKey:    "test-key",
		SecretKey: "test-secret",
	})
	adapter.client.baseURL = server.URL
	adapter.client.httpClient = server.Client()

	err := adapter.TestOrder(exchange.PlaceOrderParams{
		Symbol:    "SIRENUSDT",
		Side:      exchange.SideBuy,
		OrderType: "limit",
		Price:     "0.0068447",
		Size:      "79",
		Force:     "ioc",
	})
	if err != nil {
		t.Fatalf("TestOrder returned error: %v", err)
	}
	if !seen {
		t.Fatal("test server did not receive request")
	}
	if !deleteSeen {
		t.Fatal("test server did not receive probe cancel request")
	}
}

func TestTestOrderReturnsAPIOrdersDisabledError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/openApi/swap/v2/trade/order" {
			t.Fatalf("path = %s, want /openApi/swap/v2/trade/order", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":109400,"msg":"Reminder: Due to the large market fluctuations, in order to reduce the risk of liquidation, API orders are temporarily disabled.","data":{}}`))
	}))
	defer server.Close()

	adapter := NewAdapter(exchange.ExchangeConfig{
		ApiKey:    "test-key",
		SecretKey: "test-secret",
	})
	adapter.client.baseURL = server.URL
	adapter.client.httpClient = server.Client()

	err := adapter.TestOrder(exchange.PlaceOrderParams{
		Symbol:    "SPORTFUNUSDT",
		Side:      exchange.SideBuy,
		OrderType: "limit",
		Price:     "0.00049415",
		Size:      "1000",
		Force:     "ioc",
	})
	if err == nil {
		t.Fatal("TestOrder returned nil error, want BingX API error")
	}
	if !strings.Contains(err.Error(), "API orders are temporarily disabled") {
		t.Fatalf("error = %q, want API orders disabled message", err.Error())
	}
}

func TestTestOrderReturnsErrorWhenProbeOrderIDMissing(t *testing.T) {
	var deleteSeen bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			deleteSeen = true
			t.Fatalf("delete should not be sent without a probe order id")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"msg":"","data":{"order":{}}}`))
	}))
	defer server.Close()

	adapter := NewAdapter(exchange.ExchangeConfig{
		ApiKey:    "test-key",
		SecretKey: "test-secret",
	})
	adapter.client.baseURL = server.URL
	adapter.client.httpClient = server.Client()

	err := adapter.TestOrder(exchange.PlaceOrderParams{
		Symbol:    "SPORTFUNUSDT",
		Side:      exchange.SideSell,
		OrderType: "limit",
		Price:     "0.09842540",
		Size:      "1000",
		Force:     "ioc",
	})
	if err == nil {
		t.Fatal("TestOrder returned nil error, want missing order id failure")
	}
	if !strings.Contains(err.Error(), "missing probe order id") {
		t.Fatalf("error = %q, want missing probe order id context", err.Error())
	}
	if deleteSeen {
		t.Fatal("delete was sent without a probe order id")
	}
}

func TestTestOrderReturnsProbeCancelError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodDelete {
			_, _ = w.Write([]byte(`{"code":100500,"msg":"cancel failed","data":{}}`))
			return
		}
		_, _ = w.Write([]byte(`{"code":0,"msg":"","data":{"order":{"orderId":123456789}}}`))
	}))
	defer server.Close()

	adapter := NewAdapter(exchange.ExchangeConfig{
		ApiKey:    "test-key",
		SecretKey: "test-secret",
	})
	adapter.client.baseURL = server.URL
	adapter.client.httpClient = server.Client()

	err := adapter.TestOrder(exchange.PlaceOrderParams{
		Symbol:    "SPORTFUNUSDT",
		Side:      exchange.SideBuy,
		OrderType: "limit",
		Price:     "0.00049415",
		Size:      "1000",
		Force:     "ioc",
	})
	if err == nil {
		t.Fatal("TestOrder returned nil error, want cancel failure")
	}
	if !strings.Contains(err.Error(), "cancel probe order") {
		t.Fatalf("error = %q, want cancel probe order context", err.Error())
	}
}

func TestTestOrderTreatsCancelOrderNotExistMessageAsSuccess(t *testing.T) {
	var deleteSeen bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodDelete {
			deleteSeen = true
			_, _ = w.Write([]byte(`{"code":109400,"msg":"order not exist","data":{}}`))
			return
		}
		_, _ = w.Write([]byte(`{"code":0,"msg":"","data":{"order":{"orderId":123456789}}}`))
	}))
	defer server.Close()

	adapter := NewAdapter(exchange.ExchangeConfig{
		ApiKey:    "test-key",
		SecretKey: "test-secret",
	})
	adapter.client.baseURL = server.URL
	adapter.client.httpClient = server.Client()

	err := adapter.TestOrder(exchange.PlaceOrderParams{
		Symbol:    "GUNUSDT",
		Side:      exchange.SideSell,
		OrderType: "limit",
		Price:     "0.02823213",
		Size:      "17414",
		Force:     "ioc",
	})
	if err != nil {
		t.Fatalf("TestOrder returned error for cancel order-not-exist message: %v", err)
	}
	if !deleteSeen {
		t.Fatal("test server did not receive probe cancel request")
	}
}

func TestTestOrderDoesNotTreatCancelAPIOrdersDisabledAsSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodDelete {
			_, _ = w.Write([]byte(`{"code":109400,"msg":"Reminder: Due to the large market fluctuations, in order to reduce the risk of liquidation, API orders are temporarily disabled.","data":{}}`))
			return
		}
		_, _ = w.Write([]byte(`{"code":0,"msg":"","data":{"order":{"orderId":123456789}}}`))
	}))
	defer server.Close()

	adapter := NewAdapter(exchange.ExchangeConfig{
		ApiKey:    "test-key",
		SecretKey: "test-secret",
	})
	adapter.client.baseURL = server.URL
	adapter.client.httpClient = server.Client()

	err := adapter.TestOrder(exchange.PlaceOrderParams{
		Symbol:    "GUNUSDT",
		Side:      exchange.SideSell,
		OrderType: "limit",
		Price:     "0.02823213",
		Size:      "17414",
		Force:     "ioc",
	})
	if err == nil {
		t.Fatal("TestOrder returned nil error, want cancel API-disabled failure")
	}
	if !strings.Contains(err.Error(), "API orders are temporarily disabled") {
		t.Fatalf("error = %q, want API disabled message", err.Error())
	}
}

func TestTestOrderReturnsErrorWhenProbeFillsBeforeCancel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodDelete {
			_, _ = w.Write([]byte(`{"code":80018,"msg":"order already filled","data":{}}`))
			return
		}
		_, _ = w.Write([]byte(`{"code":0,"msg":"","data":{"order":{"orderId":123456789}}}`))
	}))
	defer server.Close()

	adapter := NewAdapter(exchange.ExchangeConfig{
		ApiKey:    "test-key",
		SecretKey: "test-secret",
	})
	adapter.client.baseURL = server.URL
	adapter.client.httpClient = server.Client()

	err := adapter.TestOrder(exchange.PlaceOrderParams{
		Symbol:    "SPORTFUNUSDT",
		Side:      exchange.SideSell,
		OrderType: "limit",
		Price:     "0.09842540",
		Size:      "1000",
		Force:     "ioc",
	})
	if err == nil {
		t.Fatal("TestOrder returned nil error, want filled probe failure")
	}
	if !strings.Contains(err.Error(), "probe order filled before cancel") {
		t.Fatalf("error = %q, want filled probe context", err.Error())
	}
}

func TestTestOrderRejectsUnsafeProbeParamsWithoutSending(t *testing.T) {
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		t.Fatalf("server should not receive unsafe probe request")
	}))
	defer server.Close()

	adapter := NewAdapter(exchange.ExchangeConfig{
		ApiKey:    "test-key",
		SecretKey: "test-secret",
	})
	adapter.client.baseURL = server.URL
	adapter.client.httpClient = server.Client()

	tests := []exchange.PlaceOrderParams{
		{Symbol: "SPORTFUNUSDT", Side: exchange.SideBuy, OrderType: "market", Size: "100", Force: "ioc"},
		{Symbol: "SPORTFUNUSDT", Side: exchange.SideBuy, OrderType: "limit", Price: "0.1", Size: "100", Force: "gtc"},
		{Symbol: "SPORTFUNUSDT", Side: exchange.SideBuy, OrderType: "limit", Price: "", Size: "100", Force: "ioc"},
		{Symbol: "SPORTFUNUSDT", Side: exchange.SideBuy, OrderType: "limit", Price: "0.1", Size: "0", Force: "ioc"},
		{Symbol: "SPORTFUNUSDT", Side: exchange.SideBuy, OrderType: "limit", Price: "0.1", Size: "100", Force: "ioc", ReduceOnly: true},
		{Symbol: "SPORTFUNUSDT", Side: exchange.Side("close"), OrderType: "limit", Price: "0.1", Size: "100", Force: "ioc"},
	}
	for _, tc := range tests {
		err := adapter.TestOrder(tc)
		if err == nil {
			t.Fatalf("TestOrder(%+v) returned nil error, want unsafe probe rejection", tc)
		}
		if !strings.Contains(err.Error(), "unsafe probe") {
			t.Fatalf("error = %q, want unsafe probe context", err.Error())
		}
	}
	if requests != 0 {
		t.Fatalf("unsafe probes sent %d requests, want 0", requests)
	}
}

func TestGetFuturesBalancePreservesZeroAvailableMarginAndParsesUsedFreezed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/openApi/swap/v3/user/balance" {
			t.Fatalf("path = %s, want /openApi/swap/v3/user/balance", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"code": 0,
			"msg": "",
			"data": [
				{
					"asset": "USDT",
					"balance": "100",
					"equity": "150",
					"availableMargin": "0",
					"usedMargin": "12.5",
					"freezedMargin": "7.25"
				}
			]
		}`))
	}))
	defer server.Close()

	adapter := NewAdapter(exchange.ExchangeConfig{
		ApiKey:    "test-key",
		SecretKey: "test-secret",
	})
	adapter.client.baseURL = server.URL
	adapter.client.httpClient = server.Client()

	bal, err := adapter.GetFuturesBalance()
	if err != nil {
		t.Fatalf("GetFuturesBalance: %v", err)
	}

	if bal.Available != 0 {
		t.Fatalf("Available = %v, want 0; availableMargin=\"0\" must not fall back", bal.Available)
	}
	if bal.Frozen != 12.5 {
		t.Fatalf("Frozen = %v, want usedMargin 12.5", bal.Frozen)
	}
	if bal.MaxTransferOut != 80.25 {
		t.Fatalf("MaxTransferOut = %v, want balance-used-freezed = 80.25", bal.MaxTransferOut)
	}
	if bal.MarginRatio != 1 {
		t.Fatalf("MarginRatio = %v, want 1 when availableMargin is 0 and equity is positive", bal.MarginRatio)
	}
	if !bal.MarginRatioUnavailable {
		t.Fatal("MarginRatioUnavailable = false, want true")
	}
}
