package utils

import "testing"

func TestIsValidBaseSymbol(t *testing.T) {
	valid := []string{"BTC", "ETH", "1000SATS", "1MBABYDOGE", "0G", "1INCH", "10000000AIDOGE"}
	invalid := []string{"龙虾", "币安人生", "BTC/ETH", "BTC_USDT", "", "BTC USDT", "café"}
	for _, s := range valid {
		if !IsValidBaseSymbol(s) {
			t.Fatalf("IsValidBaseSymbol(%q) = false, want true", s)
		}
	}
	for _, s := range invalid {
		if IsValidBaseSymbol(s) {
			t.Fatalf("IsValidBaseSymbol(%q) = true, want false", s)
		}
	}
}
