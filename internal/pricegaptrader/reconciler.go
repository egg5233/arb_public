// Package pricegaptrader — reconciler.go owns the Phase 14 PG-LIVE-03 daily
// reconcile path: aggregate closed Strategy 4 positions for a UTC date, write
// pg:reconcile:daily:{date}, surface the clean-day signal that drives the
// Plan 14-03 RampController.
//
// Decisions implemented here:
//   - D-03 triple-fail: 3 retry attempts at delays 5s/15s/30s. The retry loop
//     IMITATES internal/engine/exit.go:1119 — D-15 module-isolation rule
//     prohibits importing internal/engine.
//   - D-04 byte-identical idempotency: typed-struct schema + sorted positions
//     (by ExchangeClosedAt, ID) + sorted FlaggedIDs + frozen ComputedAt via
//     nowFn. TestReconcile_Idempotency_ByteEqual locks this contract.
//   - D-05 NetClean signal: realized_pnl_usdt >= 0 AND positions_closed >= 1.
//   - D-07 paper-mode parity: reconciler does NOT branch on PriceGapLiveCapital
//     — clean-day signal accumulates regardless of capital mode.
//   - D-09 anomaly threshold: |RealizedSlipBps| > cfg.PriceGapAnomalySlippageBps
//     fires AnomalyHighSlippage; ID surfaces in FlaggedIDs.
//   - D-10 missing close timestamp: ExchangeClosedAt zero -> fall back to
//     ClosedAt (local clock); position remains in aggregation; flag
//     anomaly_missing_close_ts.
//   - D-15 module isolation: this file imports context, encoding/json, errors,
//     fmt, sort, time, arb/internal/config, arb/internal/models, arb/pkg/utils.
//     NO internal/engine, NO internal/api, NO internal/database, NO
//     internal/notify.
//
// Concurrency: RunForDate is invoked by the daemon's single goroutine
// (Plan 14-04). The Reconciler holds no shared mutable state.
package pricegaptrader

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"arb/internal/config"
	"arb/internal/models"
	"arb/pkg/utils"
)

// ReconcileStore — narrow interface for Reconciler dependencies. Production
// binds *database.Client (which already provides all four methods); tests
// bind fakeReconcileStore (reconciler_test.go).
//
// D-15 boundary: declared HERE (not in internal/models) because the
// Reconciler is an internal pricegaptrader concern; widening
// internal/models with a reconcile-specific interface would invite
// cross-module coupling. *database.Client satisfies this interface
// implicitly via the matching method set on the existing
// models.PriceGapStore extension.
type ReconcileStore interface {
	GetPriceGapClosedPositionsForDate(date string) ([]string, error)
	LoadPriceGapPosition(id string) (*models.PriceGapPosition, bool, error)
	SavePriceGapReconcileDaily(date string, payload []byte) error
	LoadPriceGapReconcileDaily(date string) ([]byte, bool, error)
}

// ReconcileNotifier — narrow interface for the digest + failure dispatch
// surface used by the Reconciler. Plan 14-04 ships *notify.TelegramNotifier
// which satisfies the broader PriceGapNotifier interface declared in
// notify.go (this interface is a subset).
//
// fakeReconcileNotifier (reconciler_test.go) implements this directly.
type ReconcileNotifier interface {
	NotifyPriceGapDailyDigest(date string, record DailyReconcileRecord, ramp models.RampState)
	NotifyPriceGapReconcileFailure(date string, err error)
}

// reconcileRetryDelays — the 3-retry timing pattern. IMITATES (does NOT import)
// internal/engine/exit.go:1119 per D-15. The loop in RunForDate sleeps before
// attempts 1 and 2 (i.e. between attempts 0->1 and 1->2 by indexing
// reconcileRetryDelays[attempt] when attempt > 0); the literal pinned by the
// acceptance grep "5 \* time.Second, 15 \* time.Second, 30 \* time.Second"
// must remain on a single line in this declaration.
var reconcileRetryDelays = []time.Duration{5 * time.Second, 15 * time.Second, 30 * time.Second}

// Reconciler aggregates closed Strategy 4 positions for a UTC date and writes
// pg:reconcile:daily:{date}. Constructed via NewReconciler; tests can swap
// the time + sleep sources via SetNowFunc / SetSleepFunc to make the 3-retry
// path testable without paying the 50s real-time cost.
type Reconciler struct {
	store    ReconcileStore
	notifier ReconcileNotifier
	cfg      *config.Config
	log      *utils.Logger
	nowFn    func() time.Time
	sleepFn  func(time.Duration)
}

// NewReconciler wires the Reconciler with production defaults (time.Now and
// time.Sleep). Tests construct the same struct then call SetNowFunc /
// SetSleepFunc to swap the clock and sleeper sources.
func NewReconciler(store ReconcileStore, notifier ReconcileNotifier, cfg *config.Config, log *utils.Logger) *Reconciler {
	return &Reconciler{
		store:    store,
		notifier: notifier,
		cfg:      cfg,
		log:      log,
		nowFn:    time.Now,
		sleepFn:  time.Sleep,
	}
}

// SetNowFunc replaces the clock source. Tests inject a fixed-instant lambda
// so DailyReconcileRecord.ComputedAt is deterministic and the byte-equality
// lock test can pass.
func (r *Reconciler) SetNowFunc(fn func() time.Time) {
	if fn == nil {
		return
	}
	r.nowFn = fn
}

// SetSleepFunc replaces the sleep source. Tests inject a recording lambda so
// the 3-retry loop can be exercised without paying the 50s real-time cost.
func (r *Reconciler) SetSleepFunc(fn func(time.Duration)) {
	if fn == nil {
		return
	}
	r.sleepFn = fn
}

// RunForDate aggregates closed Strategy 4 positions for the given UTC date
// and writes pg:reconcile:daily:{date}. 3-retry on failure (D-03).
//
// IMITATES the retry shape from internal/engine/exit.go:1119 (5s/15s/30s) —
// DO NOT IMPORT internal/engine here (D-15). Sleep happens BEFORE attempts
// 1 and 2 (not before attempt 0, not after attempt 2): this mirrors the
// engine's PnL reconcile pattern where the first attempt is immediate.
//
// On triple-fail: the Reconciler logs an error, calls
// Notifier.NotifyPriceGapReconcileFailure, and returns the last error.
// pg:reconcile:daily:{date} is NOT written for failed days — RampController
// (Plan 14-03) treats unreconciled days as ambiguous (no promote, no demote,
// fail-safe per T-14-03).
//
// ctx cancellation: respected between retries; an in-flight aggregateAndWrite
// is left to complete its underlying store operation but the loop exits as
// soon as the in-flight call returns.
func (r *Reconciler) RunForDate(ctx context.Context, date string) error {
	var lastErr error
	for attempt, d := range reconcileRetryDelays {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			r.sleepFn(d)
		}
		err := r.aggregateAndWrite(ctx, date)
		if err == nil {
			return nil
		}
		lastErr = err
		if r.log != nil {
			r.log.Warn("[pricegap-reconcile] %s attempt %d/%d failed: %v",
				date, attempt+1, len(reconcileRetryDelays), err)
		}
	}
	// Triple-fail (D-03). Critical path: log error + Telegram dispatch +
	// surface error to caller. Plan 14-03 RampController will refuse to
	// promote on absent reconcile records — fail-safe.
	if r.log != nil {
		r.log.Error("[pricegap-reconcile] %s FAILED after %d attempts: %v",
			date, len(reconcileRetryDelays), lastErr)
	}
	r.notifier.NotifyPriceGapReconcileFailure(date, lastErr)
	return lastErr
}

// aggregateAndWrite runs one full aggregate+marshal+save cycle. Returns an
// error to drive the retry loop in RunForDate; nil on success.
//
// Per-position errors (HGET fail, decode fail) are LOGGED and counted into
// totals.SkippedCount per T-14-11 — a single corrupt position must not break
// reconcile for the entire day.
func (r *Reconciler) aggregateAndWrite(ctx context.Context, date string) error {
	_ = ctx // ctx accepted for forward-compat; current store impls don't take ctx.

	ids, err := r.store.GetPriceGapClosedPositionsForDate(date)
	if err != nil {
		return fmt.Errorf("list closed positions for %s: %w", date, err)
	}

	positions := make([]DailyReconcilePosition, 0, len(ids))
	skipped := 0
	for _, id := range ids {
		pos, exists, e := r.store.LoadPriceGapPosition(id)
		if e != nil {
			skipped++
			if r.log != nil {
				r.log.Warn("[pricegap-reconcile] %s skipping %s (load err): %v", date, id, e)
			}
			continue
		}
		if !exists || pos == nil {
			skipped++
			if r.log != nil {
				r.log.Warn("[pricegap-reconcile] %s skipping %s (not found in pg:pos:*)", date, id)
			}
			continue
		}
		positions = append(positions, r.buildPositionRecord(pos))
	}

	// T-14-06 mitigation: deterministic position ordering. Sort by
	// ExchangeClosedAt ASC; tie-break by ID ASC so two positions closed in
	// the same second still produce a stable order.
	sort.SliceStable(positions, func(i, j int) bool {
		ti, tj := positions[i].ExchangeClosedAt, positions[j].ExchangeClosedAt
		if !ti.Equal(tj) {
			return ti.Before(tj)
		}
		return positions[i].ID < positions[j].ID
	})

	totals, anomalies := r.computeAggregates(positions, skipped)

	// T-14-06 mitigation: sort flagged IDs ASC for determinism. The slice
	// was pre-allocated to []string{} (never nil) by computeAggregates so a
	// no-anomaly day produces "[]" not "null" in the JSON.
	sort.Strings(anomalies.FlaggedIDs)

	record := DailyReconcileRecord{
		SchemaVersion: reconcileSchemaVersion,
		Date:          date,
		ComputedAt:    r.nowFn().UTC(),
		Totals:        totals,
		Positions:     positions,
		Anomalies:     anomalies,
	}

	payload, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal reconcile record for %s: %w", date, err)
	}
	if err := r.store.SavePriceGapReconcileDaily(date, payload); err != nil {
		return fmt.Errorf("save pg:reconcile:daily:%s: %w", date, err)
	}
	return nil
}

// buildPositionRecord lifts a PriceGapPosition into the on-disk record shape.
// D-10 missing-close-ts handling lives here: if the source ExchangeClosedAt
// is zero, fall back to ClosedAt and stamp the anomaly flag.
func (r *Reconciler) buildPositionRecord(pos *models.PriceGapPosition) DailyReconcilePosition {
	dp := DailyReconcilePosition{
		ID:                  pos.ID,
		Version:             pos.Version,
		RealizedPnLUSDT:     pos.RealizedPnL,
		RealizedSlippageBps: pos.RealizedSlipBps,
	}
	// D-10 fallback: zero ExchangeClosedAt -> use ClosedAt; flag anomaly.
	if pos.ExchangeClosedAt.IsZero() {
		dp.ExchangeClosedAt = pos.ClosedAt
		dp.FallbackLocalCloseAt = true
		dp.AnomalyMissingCloseTs = true
	} else {
		dp.ExchangeClosedAt = pos.ExchangeClosedAt
	}
	// D-09 anomaly threshold: |RealizedSlipBps| > threshold -> flag.
	// cfg may be nil in unit tests that don't configure it; default to no
	// flag in that case (the threshold is a config-driven knob, not a
	// hard-coded invariant).
	if r.cfg != nil && r.cfg.PriceGapAnomalySlippageBps > 0 {
		if absFloat(pos.RealizedSlipBps) > r.cfg.PriceGapAnomalySlippageBps {
			dp.AnomalyHighSlippage = true
		}
	}
	return dp
}

// computeAggregates produces the totals + anomalies summary from the
// already-built per-position records. anomalies.FlaggedIDs is initialized to
// an empty (non-nil) slice so JSON output is "[]" rather than "null" — this
// is a byte-equality requirement for D-04.
func (r *Reconciler) computeAggregates(positions []DailyReconcilePosition, skipped int) (DailyReconcileTotals, DailyReconcileAnomalies) {
	totals := DailyReconcileTotals{
		SkippedCount: skipped,
	}
	anomalies := DailyReconcileAnomalies{
		FlaggedIDs: []string{}, // non-nil; preserves byte-equality on no-anomaly days
	}
	for _, p := range positions {
		totals.RealizedPnLUSDT += p.RealizedPnLUSDT
		totals.PositionsClosed++
		if p.RealizedPnLUSDT >= 0 {
			totals.Wins++
		} else {
			totals.Losses++
		}
		flagged := false
		if p.AnomalyHighSlippage {
			anomalies.HighSlippageCount++
			flagged = true
		}
		if p.AnomalyMissingCloseTs {
			anomalies.MissingCloseTsCount++
			flagged = true
		}
		if flagged && !containsReconcileFlag(anomalies.FlaggedIDs, p.ID) {
			anomalies.FlaggedIDs = append(anomalies.FlaggedIDs, p.ID)
		}
	}
	// D-05 clean signal: realized PnL >= 0 AND >= 1 closed position.
	totals.NetClean = totals.PositionsClosed >= 1 && totals.RealizedPnLUSDT >= 0
	return totals, anomalies
}

// LoadRecord exposes pg:reconcile:daily:{date} parsed for downstream
// consumers (RampController in Plan 14-03). Returns (record, false, nil)
// when the day has not been reconciled — RampController treats this as
// ambiguous and refuses to promote (fail-safe per T-14-03).
func (r *Reconciler) LoadRecord(_ context.Context, date string) (DailyReconcileRecord, bool, error) {
	payload, exists, err := r.store.LoadPriceGapReconcileDaily(date)
	if err != nil {
		return DailyReconcileRecord{}, false, fmt.Errorf("load pg:reconcile:daily:%s: %w", date, err)
	}
	if !exists {
		return DailyReconcileRecord{}, false, nil
	}
	var rec DailyReconcileRecord
	if err := json.Unmarshal(payload, &rec); err != nil {
		return DailyReconcileRecord{}, false, fmt.Errorf("unmarshal reconcile record for %s: %w", date, err)
	}
	return rec, true, nil
}

func absFloat(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// containsReconcileFlag — local helper (avoids name collision with other
// package-private helpers). Linear scan is fine for the small N we expect
// (single-day flagged set).
func containsReconcileFlag(s []string, t string) bool {
	for _, v := range s {
		if v == t {
			return true
		}
	}
	return false
}
