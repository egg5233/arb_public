package bitget

import (
	"arb/pkg/exchange"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestMaintenanceRateNormalization_Bitget verifies that Bitget keepMarginRate
// is parsed correctly (already decimal string: "0.004" = 0.4%).
func TestMaintenanceRateNormalization_Bitget(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/mix/market/query-position-lever":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"code": "00000",
				"msg":  "success",
				"data": []map[string]interface{}{
					{
						"level":          "1",
						"startUnit":      "0",
						"endUnit":        "150000",
						"leverage":       "125",
						"keepMarginRate": "0.004",
					},
					{
						"level":          "2",
						"startUnit":      "150000",
						"endUnit":        "600000",
						"leverage":       "100",
						"keepMarginRate": "0.005",
					},
					{
						"level":          "3",
						"startUnit":      "600000",
						"endUnit":        "2000000",
						"leverage":       "75",
						"keepMarginRate": "0.01",
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

	// Notional 100k should match tier 1 (0.004)
	rate, err := adapter.GetMaintenanceRate("BTCUSDT", 100000)
	if err != nil {
		t.Fatalf("GetMaintenanceRate: %v", err)
	}
	if rate != 0.004 {
		t.Errorf("tier 1 rate = %v, want 0.004", rate)
	}

	// Notional 200k should match tier 2 (0.005)
	rate2, err := adapter.GetMaintenanceRate("BTCUSDT", 200000)
	if err != nil {
		t.Fatalf("GetMaintenanceRate: %v", err)
	}
	if rate2 != 0.005 {
		t.Errorf("tier 2 rate = %v, want 0.005", rate2)
	}

	// Notional 1M should match tier 3 (0.01)
	rate3, err := adapter.GetMaintenanceRate("BTCUSDT", 1000000)
	if err != nil {
		t.Fatalf("GetMaintenanceRate: %v", err)
	}
	if rate3 != 0.01 {
		t.Errorf("tier 3 rate = %v, want 0.01", rate3)
	}

	// Notional 0 should return tier 1 (lowest)
	rate0, err := adapter.GetMaintenanceRate("BTCUSDT", 0)
	if err != nil {
		t.Fatalf("GetMaintenanceRate: %v", err)
	}
	if rate0 != 0.004 {
		t.Errorf("notional=0 rate = %v, want 0.004 (tier 1)", rate0)
	}
}

// TestMaintenanceRate_Bitget_BoundsCheck verifies that invalid rates are rejected.
func TestMaintenanceRate_Bitget_BoundsCheck(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"code": "00000",
			"msg":  "success",
			"data": []map[string]interface{}{
				{
					"level":          "1",
					"startUnit":      "0",
					"endUnit":        "150000",
					"leverage":       "125",
					"keepMarginRate": "-0.5",
				},
			},
		})
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

	rate, err := adapter.GetMaintenanceRate("BTCUSDT", 100000)
	if err != nil {
		t.Fatalf("GetMaintenanceRate: %v", err)
	}
	if rate != 0 {
		t.Errorf("negative rate should return 0, got %v", rate)
	}
}

func TestGetFuturesBalance_PreservesZeroCrossedMaxAvailable(t *testing.T) {
	bal := getBitgetFuturesBalanceForTest(t, map[string]interface{}{
		"accountEquity":       "100",
		"available":           "75",
		"crossedMaxAvailable": "0",
		"locked":              "3",
		"crossedRiskRate":     "0.12",
		"maxTransferOut":      "60",
	})

	if bal.Available != 0 {
		t.Fatalf("Available = %v, want 0; crossedMaxAvailable=\"0\" must not fall back to available", bal.Available)
	}
	if bal.Total != 100 {
		t.Fatalf("Total = %v, want 100", bal.Total)
	}
	if bal.Frozen != 3 {
		t.Fatalf("Frozen = %v, want 3", bal.Frozen)
	}
}

func TestGetFuturesBalance_FallsBackWhenCrossedMaxAvailableMissingOrEmpty(t *testing.T) {
	tests := []struct {
		name string
		data map[string]interface{}
		want float64
	}{
		{
			name: "empty crossedMaxAvailable",
			data: map[string]interface{}{
				"accountEquity":       "100",
				"available":           "75",
				"crossedMaxAvailable": "",
				"locked":              "3",
			},
			want: 75,
		},
		{
			name: "missing crossedMaxAvailable",
			data: map[string]interface{}{
				"accountEquity": "100",
				"available":     "76",
				"locked":        "3",
			},
			want: 76,
		},
		{
			name: "missing crossedMaxAvailable preserves zero available",
			data: map[string]interface{}{
				"accountEquity": "100",
				"available":     "0",
				"locked":        "3",
			},
			want: 0,
		},
		{
			name: "empty crossedMaxAvailable preserves zero available",
			data: map[string]interface{}{
				"accountEquity":       "100",
				"available":           "0",
				"crossedMaxAvailable": "",
				"locked":              "3",
			},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bal := getBitgetFuturesBalanceForTest(t, tt.data)
			if bal.Available != tt.want {
				t.Fatalf("Available = %v, want %v", bal.Available, tt.want)
			}
		})
	}
}

func getBitgetFuturesBalanceForTest(t *testing.T, data map[string]interface{}) *exchange.Balance {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/mix/account/account" {
			http.NotFound(w, r)
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"code": "00000",
			"msg":  "success",
			"data": data,
		})
	}))
	defer srv.Close()

	adapter := &Adapter{
		client: &Client{
			apiKey:     "test",
			secretKey:  "test",
			passphrase: "test",
			baseURL:    srv.URL,
			httpClient: srv.Client(),
		},
	}

	bal, err := adapter.GetFuturesBalance()
	if err != nil {
		t.Fatalf("GetFuturesBalance: %v", err)
	}
	return bal
}
