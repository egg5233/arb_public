// Package notify — pricegap_reconcile.go ships Phase 14 PG-LIVE-03 stub
// methods on *TelegramNotifier that satisfy the widened
// pricegaptrader.PriceGapNotifier interface.
//
// These methods are stubs in Plan 14-02 — they exist so the compile-time
// conformance assertion in pricegap_assert.go (var _ pricegaptrader.
// PriceGapNotifier = (*TelegramNotifier)(nil)) keeps building. Plan 14-04
// replaces these stubs with the real Telegram dispatch (digest formatting,
// critical-path rate-limit override, cooldown keys) on the existing
// t.send + t.checkCooldown plumbing in telegram.go.
//
// Lives in a separate file (not telegram.go) so the pricegaptrader import
// stays isolated to the assertion + reconcile-stub files. telegram.go must
// remain pricegaptrader-import-free per the existing module-graph layout.
package notify

import (
	"arb/internal/models"
	"arb/internal/pricegaptrader"
)

// NotifyPriceGapDailyDigest is a Plan 14-02 stub. Plan 14-04 will format the
// reconcile record + ramp state into a routine UTC-00:30 Telegram digest
// and dispatch via t.send (cooldown key "pg_digest:{date}", non-critical
// path). Until then the method is a nil-receiver-safe no-op so the
// PriceGapNotifier interface is satisfied at compile time.
func (t *TelegramNotifier) NotifyPriceGapDailyDigest(_ string, _ pricegaptrader.DailyReconcileRecord, _ models.RampState) {
	if t == nil {
		return
	}
	// Plan 14-04 replaces this body.
}

// NotifyPriceGapReconcileFailure is a Plan 14-02 stub. Plan 14-04 will
// dispatch a critical-path Telegram alert (cooldown override, prefix
// "PRICE-GAP RECONCILE FAILED"). Until then the method is a
// nil-receiver-safe no-op so the PriceGapNotifier interface is satisfied
// at compile time.
func (t *TelegramNotifier) NotifyPriceGapReconcileFailure(_ string, _ error) {
	if t == nil {
		return
	}
	// Plan 14-04 replaces this body.
}
