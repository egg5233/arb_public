// Package notify — test_hook.go exposes test-only hooks for cross-package
// tests (Phase 14 Plan 14-04: pricegaptrader notify_test.go uses these to
// retarget telegram API base to httptest.Server).
//
// These hooks are intentionally unexported through normal API surface;
// callers outside notify use SetTelegramAPIBase to swap the base URL and
// the returned prior value to restore. Production code never calls these.
package notify

// SetTelegramAPIBase swaps the package-level telegramAPIBase URL and returns
// the prior value so the caller can restore it. Used by sibling-package
// tests that need to retarget Telegram API calls to an httptest.Server.
func SetTelegramAPIBase(newBase string) string {
	prior := telegramAPIBase
	telegramAPIBase = newBase
	return prior
}
