package engine

import (
	"errors"
	"testing"
	"time"

	"arb/pkg/exchange"
)

// TestPollBingXFundVisibility covers the localized visibility gate applied after
// a BingX donor futures→spot transfer. Scenario: TransferToSpot via the v1
// asset/transfer API may have a short delay before Withdraw(walletType=1) can
// see the balance. The gate polls until visible or times out, in which case
// the caller rolls back and skips the donor.
func TestPollBingXFundVisibility(t *testing.T) {
	const baseline = 100.0
	const moved = 14.04
	const required = moved * 0.99 // 13.8996

	t.Run("visible immediately → returns true fast", func(t *testing.T) {
		calls := 0
		getBal := func() (*exchange.Balance, error) {
			calls++
			// Simulate balance already reflecting the move on first poll.
			return &exchange.Balance{Available: baseline + moved}, nil
		}
		start := time.Now()
		visible := pollBingXFundVisibility(getBal, baseline, moved, 15*time.Second, 10*time.Millisecond)
		elapsed := time.Since(start)
		if !visible {
			t.Errorf("expected visible=true on immediate-match, got false")
		}
		if elapsed > 200*time.Millisecond {
			t.Errorf("expected fast return (<200ms), got %v", elapsed)
		}
		if calls < 1 {
			t.Errorf("expected at least 1 poll, got %d", calls)
		}
	})

	t.Run("becomes visible after a few polls → returns true", func(t *testing.T) {
		calls := 0
		getBal := func() (*exchange.Balance, error) {
			calls++
			// First two polls show no delta; third shows enough.
			if calls < 3 {
				return &exchange.Balance{Available: baseline}, nil
			}
			return &exchange.Balance{Available: baseline + required}, nil
		}
		visible := pollBingXFundVisibility(getBal, baseline, moved, 5*time.Second, 10*time.Millisecond)
		if !visible {
			t.Errorf("expected visible=true after delay, got false")
		}
		if calls < 3 {
			t.Errorf("expected at least 3 polls, got %d", calls)
		}
	})

	t.Run("never visible within timeout → returns false", func(t *testing.T) {
		getBal := func() (*exchange.Balance, error) {
			return &exchange.Balance{Available: baseline}, nil // no delta
		}
		start := time.Now()
		visible := pollBingXFundVisibility(getBal, baseline, moved, 100*time.Millisecond, 20*time.Millisecond)
		elapsed := time.Since(start)
		if visible {
			t.Errorf("expected visible=false on timeout, got true")
		}
		if elapsed < 80*time.Millisecond {
			t.Errorf("expected poll to run until timeout (~100ms), elapsed=%v", elapsed)
		}
	})

	t.Run("GetSpotBalance errors throughout → returns false", func(t *testing.T) {
		getBal := func() (*exchange.Balance, error) {
			return nil, errors.New("network error")
		}
		visible := pollBingXFundVisibility(getBal, baseline, moved, 80*time.Millisecond, 20*time.Millisecond)
		if visible {
			t.Errorf("expected visible=false when balance queries fail, got true")
		}
	})

	t.Run("moved=0 short-circuits to true", func(t *testing.T) {
		getBal := func() (*exchange.Balance, error) {
			t.Errorf("should not poll when moved=0")
			return nil, nil
		}
		visible := pollBingXFundVisibility(getBal, baseline, 0, 1*time.Second, 10*time.Millisecond)
		if !visible {
			t.Errorf("expected visible=true for moved=0, got false")
		}
	})
}
