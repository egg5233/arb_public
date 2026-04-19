package bitget

import (
	"context"
	"errors"
	"testing"
	"time"

	"arb/pkg/exchange"
)

func TestGetMarginInterestRateHistoryBitgetNotSupported(t *testing.T) {
	adapter := new(Adapter)
	_, err := adapter.GetMarginInterestRateHistory(context.Background(), "BTC",
		time.Now().Add(-24*time.Hour), time.Now())
	if !errors.Is(err, exchange.ErrHistoricalBorrowNotSupported) {
		t.Fatalf("expected ErrHistoricalBorrowNotSupported, got %v", err)
	}
}
