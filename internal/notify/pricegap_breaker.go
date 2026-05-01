// Package notify — pricegap_breaker.go ships Phase 15 Plan 15-04 (PG-LIVE-02)
// drawdown circuit-breaker Telegram surfaces on *TelegramNotifier that
// satisfy the widened pricegaptrader.PriceGapNotifier interface.
//
// Lives in a separate file (mirror of pricegap_reconcile.go layout) so the
// Plan 15-03 stubs can be cleanly replaced and the module-graph stays tidy.
//
// Critical bucket: both methods dispatch via sendCritical, which bypasses the
// per-event-type cooldown allowlist used by routine alerts. Per CONTEXT D-17
// the breaker trip + recovery are fire-now operator alerts — they MUST surface
// regardless of cooldown state. Same precedent as NotifyPriceGapReconcileFailure
// for triple-fail reconcile (Phase 14 Plan 14-04) which also bypasses the
// regular cooldown layer for its critical-path dispatch.
//
// Timezone: alert payloads render timestamps in Asia/Taipei (UTC+8) per
// project convention (CLAUDE.local.md / dashboard times). Falls back to UTC
// if the timezone database is unavailable on the host.
//
// Nil-receiver-safe per project convention.
package notify

import (
	"fmt"
	"strings"
	"time"

	"arb/internal/models"
)

// breakerLocation memoizes the Asia/Taipei zone lookup. time.LoadLocation is
// cheap on Linux but the result is invariant for the process lifetime.
var breakerLocation = func() *time.Location {
	loc, err := time.LoadLocation("Asia/Taipei")
	if err != nil || loc == nil {
		return time.UTC
	}
	return loc
}()

// sendCritical is the allowlist-bypass dispatch used by Phase 15 breaker
// alerts. Mirrors the bare `go t.send(text)` shape that NotifyPriceGapDailyDigest
// + NotifyPriceGapReconcileFailure already use post-cooldown — sendCritical
// codifies the contract: NO cooldown check, NO allowlist filter, fire now.
//
// Nil-receiver-safe. Best-effort: errors land in t.log via the underlying
// send path; the caller never blocks.
func (t *TelegramNotifier) sendCritical(text string) {
	if t == nil {
		return
	}
	go t.send(text)
}

// formatBreakerTime renders a unix-ms timestamp in Asia/Taipei. Returns
// "(never)" for zero/MaxInt64 sentinels so the dashboard / Telegram payload
// never shows nonsense epochs.
func formatBreakerTime(unixMs int64) string {
	if unixMs <= 0 {
		return "(never)"
	}
	// MaxInt64 sentinel (sticky-until-operator) renders as a stable label
	// rather than year 292,277,026,596 — operator clarity > arithmetic purity.
	if unixMs == int64(^uint64(0)>>1) {
		return "(sticky-until-operator)"
	}
	return time.UnixMilli(unixMs).In(breakerLocation).Format("2006-01-02 15:04:05 MST")
}

// NotifyPriceGapBreakerTrip dispatches the CRITICAL trip alert per CONTEXT
// D-17. The message contains:
//   - source label (live | test_fire | test_fire_dry_run)
//   - 24h realized PnL that triggered the trip (USDT, 2 decimals)
//   - configured threshold (USDT, 2 decimals)
//   - ramp stage at trip
//   - paused candidate count
//   - trip time in Asia/Taipei
//   - recovery instruction line (operator action required)
//
// Bypasses cooldown (sendCritical) so a re-trip after operator recovery
// always surfaces. Nil-receiver-safe.
func (t *TelegramNotifier) NotifyPriceGapBreakerTrip(record models.BreakerTripRecord) error {
	if t == nil {
		return nil
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "PRICE-GAP BREAKER TRIPPED [%s]\n", strings.ToUpper(record.Source))
	fmt.Fprintf(&sb, "24h Realized PnL: $%.2f USDT\n", record.TripPnLUSDT)
	fmt.Fprintf(&sb, "Threshold: $%.2f USDT\n", record.Threshold)
	fmt.Fprintf(&sb, "Ramp Stage: %d\n", record.RampStage)
	fmt.Fprintf(&sb, "Paused: %d candidate(s)\n", record.PausedCandidateCount)
	fmt.Fprintf(&sb, "Trip Time: %s\n", formatBreakerTime(record.TripTs))
	fmt.Fprintf(&sb, "Recover via: pg-admin breaker recover --confirm or dashboard Recover button")
	t.sendCritical(sanitizeForTelegram(sb.String(), 4000))
	return nil
}

// NotifyPriceGapBreakerRecovery dispatches the CRITICAL recovery alert when
// an operator clears the sticky paper-mode flag. Includes operator name,
// recovery timestamp (Asia/Taipei), original trip PnL + threshold, and a
// "resumed" closing line. Nil-receiver-safe.
func (t *TelegramNotifier) NotifyPriceGapBreakerRecovery(record models.BreakerTripRecord, operator string) error {
	if t == nil {
		return nil
	}
	op := strings.TrimSpace(operator)
	if op == "" {
		op = "(unknown)"
	}
	recoveryTs := time.Now().UnixMilli()
	if record.RecoveryTs != nil && *record.RecoveryTs > 0 {
		recoveryTs = *record.RecoveryTs
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "PRICE-GAP BREAKER RECOVERED\n")
	fmt.Fprintf(&sb, "Operator: %s\n", op)
	fmt.Fprintf(&sb, "Recovery Time: %s\n", formatBreakerTime(recoveryTs))
	fmt.Fprintf(&sb, "Original Trip: $%.2f USDT (threshold $%.2f)\n",
		record.TripPnLUSDT, record.Threshold)
	fmt.Fprintf(&sb, "Engine has resumed normal mode")
	t.sendCritical(sanitizeForTelegram(sb.String(), 4000))
	return nil
}
