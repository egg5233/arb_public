package notify

import "testing"

// Wave 0 stubs for breaker Telegram dispatch. Plan 15-04 implements.

// TestNotifyPriceGapBreakerTrip_CriticalBucket — D-17: trip alert dispatches
// via the critical bucket (bypasses allowlist + cooldown). Same shape as
// NotifyPriceGapReconcileFailure precedent (Phase 14).
func TestNotifyPriceGapBreakerTrip_CriticalBucket(t *testing.T) {
	t.Skip("Wave 0 stub — Plan 15-04 implements NotifyPriceGapBreakerTrip")
}

// TestNotifyPriceGapBreakerRecovery_CriticalBucket — recovery alert is also
// critical-bucket: operators must see the resume-trading event regardless
// of cooldown state.
func TestNotifyPriceGapBreakerRecovery_CriticalBucket(t *testing.T) {
	t.Skip("Wave 0 stub — Plan 15-04 implements NotifyPriceGapBreakerRecovery")
}
