package bingx

import (
	"testing"
	"time"
)

func TestBingXRateLimitRules(t *testing.T) {
	tests := []struct {
		name         string
		method       string
		path         string
		wantInterval time.Duration
	}{
		{
			name:         "fund balance uses five per second limit",
			method:       "GET",
			path:         "/openApi/fund/v1/account/balance",
			wantInterval: bingxFivePerSecondInterval,
		},
		{
			name:         "futures balance uses five per second limit",
			method:       "GET",
			path:         "/openApi/swap/v3/user/balance",
			wantInterval: bingxFivePerSecondInterval,
		},
		{
			name:         "asset transfer uses two per second limit",
			method:       "POST",
			path:         "/openApi/api/v3/post/asset/transfer",
			wantInterval: bingxTwoPerSecondInterval,
		},
		{
			name:         "withdraw uses two per second limit",
			method:       "POST",
			path:         "/openApi/wallets/v1/capital/withdraw/apply",
			wantInterval: bingxTwoPerSecondInterval,
		},
		{
			name:         "swap order placement uses ten per second limit",
			method:       "POST",
			path:         "/openApi/swap/v2/trade/order",
			wantInterval: bingxTenPerSecondInterval,
		},
		{
			name:         "swap order query uses five per second limit",
			method:       "GET",
			path:         "/openApi/swap/v2/trade/order",
			wantInterval: bingxFivePerSecondInterval,
		},
		{
			name:         "unknown signed endpoint keeps default limit",
			method:       "GET",
			path:         "/openApi/unknown",
			wantInterval: bingxDefaultSignedInterval,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, got := bingxRateLimitRule(tt.method, tt.path)
			if got != tt.wantInterval {
				t.Fatalf("interval = %s, want %s", got, tt.wantInterval)
			}
		})
	}
}

func TestBingXRateLimitUsesMethodSpecificBuckets(t *testing.T) {
	getBucket, getInterval := bingxRateLimitRule("GET", "/openApi/swap/v2/trade/order")
	postBucket, postInterval := bingxRateLimitRule("POST", "/openApi/swap/v2/trade/order")

	if getBucket == postBucket {
		t.Fatalf("GET and POST buckets are both %q, want separate buckets", getBucket)
	}
	if getInterval != bingxFivePerSecondInterval {
		t.Fatalf("GET interval = %s, want %s", getInterval, bingxFivePerSecondInterval)
	}
	if postInterval != bingxTenPerSecondInterval {
		t.Fatalf("POST interval = %s, want %s", postInterval, bingxTenPerSecondInterval)
	}
}
