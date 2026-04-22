package bitget

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestGetFundingRateAndInterval_PrefixNormalize(t *testing.T) {
	var fundingSymbol string
	var intervalSymbol string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/mix/market/contracts":
			symbol := r.URL.Query().Get("symbol")
			if symbol != "" {
				intervalSymbol = symbol
			}
			json.NewEncoder(w).Encode(map[string]any{
				"code": "00000",
				"data": []map[string]any{
					{
						"symbol":         "1000PEPEUSDT",
						"baseCoin":       "1000PEPE",
						"minTradeNum":    "1",
						"volumePlace":    "0",
						"sizeMultiplier": "1",
						"maxOrderQty":    "100000",
						"pricePlace":     "4",
						"priceEndStep":   "1",
						"fundInterval":   "4",
					},
				},
			})
		case "/api/v2/mix/market/current-fund-rate":
			fundingSymbol = r.URL.Query().Get("symbol")
			json.NewEncoder(w).Encode(map[string]any{
				"code": "00000",
				"data": []map[string]any{
					{
						"symbol":              "1000PEPEUSDT",
						"fundingRate":         "0.001",
						"nextFundRate":        "0.002",
						"nextUpdate":          "1713744000000",
						"fundingRateInterval": "",
						"maxFundingRate":      "0.01",
						"minFundingRate":      "-0.01",
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
			apiKey:     "test",
			secretKey:  "test",
			passphrase: "test",
			baseURL:    srv.URL,
			httpClient: http.DefaultClient,
		},
	}

	fr, err := adapter.GetFundingRate("PEPEUSDT")
	if err != nil {
		t.Fatalf("GetFundingRate: %v", err)
	}
	if fundingSymbol != "1000PEPEUSDT" {
		t.Fatalf("funding symbol = %q, want 1000PEPEUSDT", fundingSymbol)
	}
	if intervalSymbol != "1000PEPEUSDT" {
		t.Fatalf("interval symbol = %q, want 1000PEPEUSDT", intervalSymbol)
	}
	if fr.Symbol != "PEPEUSDT" {
		t.Fatalf("funding symbol result = %q, want PEPEUSDT", fr.Symbol)
	}
	if fr.Interval != 4*time.Hour {
		t.Fatalf("funding interval = %v, want 4h", fr.Interval)
	}
}
