// Package notify — pricegap_reconcile.go ships Phase 14 PG-LIVE-03 reconcile
// + PG-LIVE-01 ramp Telegram surfaces on *TelegramNotifier that satisfy the
// widened pricegaptrader.PriceGapNotifier interface.
//
// Lives in a separate file (not telegram.go) so the pricegaptrader import
// stays isolated to the assertion + reconcile-stub files. telegram.go must
// remain pricegaptrader-import-free per the existing module-graph layout.
//
// Plan 14-04 ships these as REAL Telegram dispatch on the existing t.send +
// t.checkCooldown plumbing. Cooldown keys:
//   - daily digest: "pg_digest:{date}" — non-critical, one-shot per UTC day
//     (cooldown is informational since the daemon only fires once per date)
//   - reconcile failure: "pg_reconcile_failure:{date}" — CRITICAL path
//   - ramp demote: "pg_ramp_demote:{prior}->{next}" — CRITICAL (capital
//     reduction event)
//   - ramp force op: "pg_ramp_force_op:{action}:{prior}->{next}" —
//     CRITICAL (manual operator override)
//
// All methods are nil-receiver-safe per project convention.
package notify

import (
	"fmt"
	"strings"

	"arb/internal/models"
	"arb/internal/pricegaptrader"
)

// NotifyPriceGapDailyDigest dispatches the routine UTC-00:30 reconcile digest
// (PG-LIVE-03, D-11). Format mirrors NotifyPriceGapEntry/Exit shape: header
// line + key-value rows; no Markdown special chars at field boundaries so the
// dashboard timeline regex remains stable. Anomalies and skipped-positions
// rows render only when non-zero. Mode is LIVE when PriceGapLiveCapital is
// implied by ramp.CurrentStage >= 1 with non-zero LastEvalTs (we cannot read
// cfg from notify package without pricegaptrader leakage; the ramp snapshot
// supplies the same signal — a non-bootstrapped ramp implies paper).
func (t *TelegramNotifier) NotifyPriceGapDailyDigest(date string, rec pricegaptrader.DailyReconcileRecord, ramp models.RampState) {
	if t == nil {
		return
	}
	// One-shot per UTC date — cooldown protects against test/spurious double-fire.
	if !t.checkCooldown("pg_digest:" + date) {
		return
	}
	mode := "PAPER"
	// Heuristic: if the ramp has been evaluated at least once (LastEvalTs non-zero)
	// the operator is running the live-capital pipeline; daemon-side wiring sets
	// LastEvalTs on the first Eval call regardless of capital mode (D-07 paper-mode
	// parity), so we additionally guard on DemoteCount or stage > 1 to disambiguate.
	if !ramp.LastEvalTs.IsZero() && (ramp.CurrentStage > 1 || ramp.DemoteCount > 0) {
		mode = "LIVE"
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "PRICE-GAP DAILY DIGEST [%s] %s\n", mode, date)
	fmt.Fprintf(&sb, "PnL: $%.2f USDT  Closed: %d  Wins/Losses: %d/%d\n",
		rec.Totals.RealizedPnLUSDT,
		rec.Totals.PositionsClosed,
		rec.Totals.Wins,
		rec.Totals.Losses,
	)
	fmt.Fprintf(&sb, "Ramp: stage %d  clean-days %d  demotes %d\n",
		ramp.CurrentStage,
		ramp.CleanDayCounter,
		ramp.DemoteCount,
	)
	if rec.Totals.SkippedCount > 0 {
		fmt.Fprintf(&sb, "Skipped: %d positions failed to load\n", rec.Totals.SkippedCount)
	}
	if len(rec.Anomalies.FlaggedIDs) > 0 {
		fmt.Fprintf(&sb, "Anomalies: %d high-slip + %d missing-ts\nFlagged: %s\n",
			rec.Anomalies.HighSlippageCount,
			rec.Anomalies.MissingCloseTsCount,
			sanitizeForTelegram(strings.Join(rec.Anomalies.FlaggedIDs, ", "), 512),
		)
	}
	go t.send(sanitizeForTelegram(sb.String(), 4000))
}

// NotifyPriceGapReconcileFailure is the CRITICAL Telegram path triggered by
// the reconciler's 3-retry triple-fail (PG-LIVE-03, D-03). Cooldown keyed per
// date so triple-fail on consecutive days each surfaces.
func (t *TelegramNotifier) NotifyPriceGapReconcileFailure(date string, err error) {
	if t == nil {
		return
	}
	if !t.checkCooldown("pg_reconcile_failure:" + date) {
		return
	}
	errMsg := "<nil>"
	if err != nil {
		errMsg = err.Error()
	}
	text := sanitizeForTelegram(fmt.Sprintf(
		"PRICE-GAP RECONCILE FAILED\nDate: %s\n3-retry triple-fail; clean-day signal blocked\nError: %s",
		date, errMsg,
	), 4000)
	go t.send(text)
}

// NotifyPriceGapRampDemote is fired by RampController.Eval when a loss-day
// triggers an asymmetric-ratchet demote (PG-LIVE-01, D-15 #2 — capital
// reduction event). CRITICAL path.
func (t *TelegramNotifier) NotifyPriceGapRampDemote(prior, next int, reason string) {
	if t == nil {
		return
	}
	if !t.checkCooldown(fmt.Sprintf("pg_ramp_demote:%d->%d", prior, next)) {
		return
	}
	text := sanitizeForTelegram(fmt.Sprintf(
		"PRICE-GAP RAMP DEMOTE\nStage %d -> %d\nReason: %s",
		prior, next, reason,
	), 500)
	go t.send(text)
}

// NotifyPriceGapRampForceOp is fired by pg-admin force-promote / force-demote
// / reset (PG-LIVE-01, D-15 #3 + #4 — manual operator override). CRITICAL
// path; surfaces full operator + reason for audit visibility.
func (t *TelegramNotifier) NotifyPriceGapRampForceOp(action string, prior, next int, operator, reason string) {
	if t == nil {
		return
	}
	if !t.checkCooldown(fmt.Sprintf("pg_ramp_force_op:%s:%d->%d", action, prior, next)) {
		return
	}
	text := sanitizeForTelegram(fmt.Sprintf(
		"PRICE-GAP RAMP %s\nStage %d -> %d\nOperator: %s\nReason: %s",
		strings.ToUpper(action), prior, next, operator, reason,
	), 500)
	go t.send(text)
}
