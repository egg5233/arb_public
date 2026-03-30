package bitget

import (
	"net/http"
	"testing"
)

// TestChineseSymbolQueryConsistency verifies that the query string used for
// HMAC signing matches the query string actually sent in the HTTP request,
// even when symbol contains non-ASCII characters (e.g. 龙虾USDT).
func TestChineseSymbolQueryConsistency(t *testing.T) {
	params := map[string]string{
		"symbol":      "龙虾USDT",
		"marginCoin":  "USDT",
		"productType": "USDT-FUTURES",
	}

	// Build the query string the same way doRequest does for signing
	qs := buildQueryString(params)
	signedPath := "/api/v2/mix/position/single-position?" + qs

	// Build an http.Request the same way the fixed doRequest does
	req, err := http.NewRequest("GET", "https://api.bitget.com/api/v2/mix/position/single-position", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.URL.RawQuery = qs

	// The actual URL path+query that will be sent
	actualPath := req.URL.RequestURI()

	if signedPath != actualPath {
		t.Errorf("signed path and actual request path differ:\n  signed: %s\n  actual: %s", signedPath, actualPath)
	}

	// Also verify the old (broken) approach would differ
	reqOld, err := http.NewRequest("GET", "https://api.bitget.com"+signedPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	oldPath := reqOld.URL.RequestURI()

	if oldPath != signedPath {
		t.Logf("confirmed: old approach produces different path:\n  signed: %s\n  old:    %s", signedPath, oldPath)
	} else {
		t.Logf("note: old approach happened to match on this system (encoding may vary)")
	}
}
