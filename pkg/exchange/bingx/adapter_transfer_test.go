package bingx

import (
	"arb/pkg/exchange"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// TestTransferToSpot_UsesDocumentedV1Endpoint verifies that TransferToSpot
// POSTs to the documented /openApi/api/asset/v1/transfer endpoint with
// fromAccount=USDTMPerp and toAccount=fund (not the legacy v3 endpoint with
// type=PFUTURES_FUND). Regression guard for the v5 100437 incident.
func TestTransferToSpot_UsesDocumentedV1Endpoint(t *testing.T) {
	gotPath, gotParams := captureTransferRequest(t, func(a *Adapter) error {
		return a.TransferToSpot("USDT", "14.04")
	})

	if gotPath != "/openApi/api/asset/v1/transfer" {
		t.Errorf("TransferToSpot path = %q, want /openApi/api/asset/v1/transfer", gotPath)
	}
	if gotParams.Get("fromAccount") != "USDTMPerp" {
		t.Errorf("fromAccount = %q, want USDTMPerp", gotParams.Get("fromAccount"))
	}
	if gotParams.Get("toAccount") != "fund" {
		t.Errorf("toAccount = %q, want fund", gotParams.Get("toAccount"))
	}
	if gotParams.Get("asset") != "USDT" {
		t.Errorf("asset = %q, want USDT", gotParams.Get("asset"))
	}
	if gotParams.Get("amount") != "14.04" {
		t.Errorf("amount = %q, want 14.04", gotParams.Get("amount"))
	}
	// Legacy type must not be sent.
	if gotParams.Get("type") != "" {
		t.Errorf("legacy type param should not be sent, got %q", gotParams.Get("type"))
	}
}

// TestTransferToFutures_UsesDocumentedV1Endpoint verifies the symmetric
// reverse direction.
func TestTransferToFutures_UsesDocumentedV1Endpoint(t *testing.T) {
	gotPath, gotParams := captureTransferRequest(t, func(a *Adapter) error {
		return a.TransferToFutures("USDT", "30.00")
	})

	if gotPath != "/openApi/api/asset/v1/transfer" {
		t.Errorf("TransferToFutures path = %q, want /openApi/api/asset/v1/transfer", gotPath)
	}
	if gotParams.Get("fromAccount") != "fund" {
		t.Errorf("fromAccount = %q, want fund", gotParams.Get("fromAccount"))
	}
	if gotParams.Get("toAccount") != "USDTMPerp" {
		t.Errorf("toAccount = %q, want USDTMPerp", gotParams.Get("toAccount"))
	}
	if gotParams.Get("asset") != "USDT" {
		t.Errorf("asset = %q, want USDT", gotParams.Get("asset"))
	}
	if gotParams.Get("amount") != "30.00" {
		t.Errorf("amount = %q, want 30.00", gotParams.Get("amount"))
	}
}

// captureTransferRequest spins up a local httptest server, points the adapter
// at it, runs fn, and returns the path + parsed form params of the request.
func captureTransferRequest(t *testing.T, fn func(*Adapter) error) (string, url.Values) {
	t.Helper()
	var capturedPath string
	var capturedParams url.Values

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		parsed, _ := url.ParseQuery(string(body))
		// Params may also be in the URL query string depending on how signature is appended.
		if capturedParams == nil {
			capturedParams = url.Values{}
		}
		for k, vs := range parsed {
			for _, v := range vs {
				capturedParams.Add(k, v)
			}
		}
		for k, vs := range r.URL.Query() {
			for _, v := range vs {
				// Avoid duplicating if body already had it.
				if capturedParams.Get(k) == "" {
					capturedParams.Add(k, v)
				}
			}
		}
		// Strip trailing signature-like params if they leaked in.
		_ = strings.TrimSpace
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"msg":"","data":{"tranId":"test"}}`))
	}))
	t.Cleanup(server.Close)

	adapter := NewAdapter(exchange.ExchangeConfig{
		Exchange:  "bingx",
		ApiKey:    "test-key",
		SecretKey: "test-secret",
	})
	// Redirect the client's baseURL to the local test server (same-package access).
	adapter.client.baseURL = server.URL

	if err := fn(adapter); err != nil {
		t.Fatalf("transfer returned error: %v", err)
	}
	return capturedPath, capturedParams
}
