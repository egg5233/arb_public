package engine

import (
	"errors"
	"testing"

	"arb/pkg/exchange"
)

// TestIsBenignBybit131212 covers the Bybit 131212 benign-error handler used in
// the rebalance deposit-confirmation path. Scenario: user's Bybit account has
// Set Deposit Account routing deposits directly into UNIFIED, so when the
// allocator calls TransferToFutures(FUND→UNIFIED), Bybit returns 131212
// "insufficient balance" (FUND is empty). If UNIFIED already holds the
// expected post-deposit amount, the error is benign.
func TestIsBenignBybit131212(t *testing.T) {
	const startBal = 46.69
	const totalPending = 79.09
	const threshold = startBal + totalPending*0.9 // 117.87

	cases := []struct {
		name       string
		err        error
		balance    *exchange.Balance
		balErr     error
		wantBenign bool
	}{
		{
			name:       "nil error → not benign",
			err:        nil,
			balance:    &exchange.Balance{Available: 125.79},
			wantBenign: false,
		},
		{
			name:       "non-131212 error → not benign",
			err:        errors.New("bybit API error code=131213 msg=some other error"),
			balance:    &exchange.Balance{Available: 125.79},
			wantBenign: false,
		},
		{
			name:       "131212 + unified at/above threshold → benign (incident case)",
			err:        errors.New("TransferToFutures: bybit API error code=131212 msg=user insufficient balance:"),
			balance:    &exchange.Balance{Available: 125.79}, // matches production log
			wantBenign: true,
		},
		{
			name:       "131212 + unified just above 90% threshold → benign",
			err:        errors.New("bybit API error code=131212 msg=user insufficient balance"),
			balance:    &exchange.Balance{Available: threshold + 0.001}, // avoid float-compare edge
			wantBenign: true,
		},
		{
			name:       "131212 + unified below threshold → NOT benign (real shortfall)",
			err:        errors.New("bybit API error code=131212 msg=user insufficient balance"),
			balance:    &exchange.Balance{Available: 50.00}, // unified didn't gain enough
			wantBenign: false,
		},
		{
			name:       "131212 + GetFuturesBalance error → NOT benign (cannot verify)",
			err:        errors.New("bybit API error code=131212"),
			balErr:     errors.New("network error"),
			wantBenign: false,
		},
		{
			name:       "131212 + nil balance → NOT benign",
			err:        errors.New("bybit API error code=131212"),
			balance:    nil,
			wantBenign: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			getBal := func() (*exchange.Balance, error) {
				return tc.balance, tc.balErr
			}
			got := isBenignBybit131212(tc.err, getBal, startBal, totalPending)
			if got != tc.wantBenign {
				t.Errorf("isBenignBybit131212() = %v, want %v", got, tc.wantBenign)
			}
		})
	}
}
