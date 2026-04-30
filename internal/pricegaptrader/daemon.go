// Package pricegaptrader — daemon.go ships the Phase 14 Plan 14-04 reconcile
// daemon: fires UTC 00:30 daily, runs Reconciler.RunForDate(yesterday) →
// RampController.Eval → NotifyPriceGapDailyDigest synchronously, with
// boot-time catchup if started after 01:00 UTC and yesterday's reconcile
// is missing.
//
// Module isolation (D-15): same as tracker.go — context, fmt, time only.
// Notifier is invoked through the existing Tracker.notifier field
// (PriceGapNotifier) which carries the digest method (Plan 14-02 widened
// the interface). Telegram dispatch happens via *TelegramNotifier
// (Task 1 of this plan).
package pricegaptrader

import (
	"context"
	"time"
)

// reconcileDaemonRunTimeout bounds a single reconcile attempt to prevent a
// hung Reconciler from blocking the daemon goroutine and stop signal
// (T-14-14). 10 minutes is generous — the 3-retry triple-fail caps real
// wallclock latency at ~50s, plus aggregation, plus overhead.
const reconcileDaemonRunTimeout = 10 * time.Minute

// reconcileLoop fires UTC 00:30 daily. Synchronously:
//  1. Reconciler.RunForDate(date) → 3-retry on failure (D-03).
//  2. Reconciler.LoadRecord(date) → fetch the just-written aggregate.
//  3. RampController.Eval(date, record) → drive ramp transitions.
//  4. notifier.NotifyPriceGapDailyDigest(date, record, ramp.Snapshot()).
//
// Boot-time catchup (RESEARCH Q14): if Tracker starts AFTER 01:00 UTC and
// yesterday's reconcile is missing, run-immediately on boot, then settle
// into the normal cadence (next fire at the next UTC 00:30).
//
// Failure handling: triple-fail on RunForDate already dispatches
// NotifyPriceGapReconcileFailure inside the Reconciler. The loop logs the
// error and skips the day — RampController.Eval is NOT called, so the
// asymmetric ratchet does not falsely demote.
//
// ctx cancellation: each per-day RunForDate gets a 10-minute timeout
// (T-14-14); the loop respects t.stopCh between fires.
func (t *Tracker) reconcileLoop() {
	defer t.wg.Done()

	if t.reconciler == nil || t.rampDaemon == nil {
		t.log.Info("pricegap: reconcile daemon disabled (no Reconciler/Ramp wired)")
		return
	}

	runOnce := func(date string) {
		ctx, cancel := context.WithTimeout(context.Background(), reconcileDaemonRunTimeout)
		defer cancel()
		if err := t.reconciler.RunForDate(ctx, date); err != nil {
			// Triple-fail already dispatched NotifyPriceGapReconcileFailure
			// from within the Reconciler. Skip Eval/digest for this day.
			t.log.Error("pricegap: reconcile %s daemon-run failed: %v", date, err)
			return
		}
		rec, exists, lerr := t.reconciler.LoadRecord(ctx, date)
		if lerr != nil || !exists {
			t.log.Error("pricegap: reconcile %s record not loadable post-run: err=%v exists=%v",
				date, lerr, exists)
			return
		}
		t.rampDaemon.Eval(ctx, date, rec)
		if t.notifier != nil {
			t.notifier.NotifyPriceGapDailyDigest(date, rec, t.rampDaemon.Snapshot())
		}
	}

	// Boot-time catchup (RESEARCH Q14): if past 01:00 UTC and yesterday is
	// not yet reconciled → run-immediately. We use Hour() >= 1 as the
	// trigger so a system that booted at 00:45 UTC waits for the regular
	// 00:30 fire (which it already missed), then catches up on the NEXT
	// boot if needed. The 1-hour threshold bounds the worst-case skip
	// window to ~01:00–01:30 UTC on a slow boot.
	nowUTC := time.Now().UTC()
	if nowUTC.Hour() >= 1 {
		yesterday := nowUTC.AddDate(0, 0, -1).Format("2006-01-02")
		if _, exists, _ := t.reconciler.LoadRecord(context.Background(), yesterday); !exists {
			t.log.Info("pricegap: boot-catchup running reconcile for %s", yesterday)
			runOnce(yesterday)
		}
	}

	// Settle into normal cadence — fire every UTC 00:30 forever.
	for {
		next := nextUTCFireTime(time.Now())
		select {
		case <-t.stopCh:
			return
		case <-time.After(time.Until(next)):
			date := time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")
			runOnce(date)
		}
	}
}

// nextUTCFireTime returns the next UTC 00:30 instant strictly after `now`.
// The "strictly after" semantic prevents zero-duration sleeps when called
// at exactly 00:30:00.000 UTC — the next fire becomes the following day.
//
// Pure function; safe to test without any Tracker fixture.
func nextUTCFireTime(now time.Time) time.Time {
	u := now.UTC()
	target := time.Date(u.Year(), u.Month(), u.Day(), 0, 30, 0, 0, time.UTC)
	if !target.After(u) {
		target = target.AddDate(0, 0, 1)
	}
	return target
}
