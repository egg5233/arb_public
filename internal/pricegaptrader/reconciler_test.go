package pricegaptrader

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"sort"
	"testing"
	"time"

	"arb/internal/config"
	"arb/internal/models"
	"arb/pkg/utils"
)

// fakeReconcileStore — narrow test fake satisfying the ReconcileStore interface
// declared in reconciler.go. Each method is hand-rolled (no reflection) so the
// fake is deterministic. listErrCount lets a single test simulate the
// triple-fail scenario by returning listErr on the first N calls and switching
// to success on the (N+1)-th.
type fakeReconcileStore struct {
	idsByDate     map[string][]string
	positions     map[string]*models.PriceGapPosition
	savedPayloads map[string][]byte
	listErr       error
	listErrCount  int // number of times listErr is returned before falling through to idsByDate
	listCalls     int
	loadErr       error
}

func newFakeReconcileStore() *fakeReconcileStore {
	return &fakeReconcileStore{
		idsByDate:     map[string][]string{},
		positions:     map[string]*models.PriceGapPosition{},
		savedPayloads: map[string][]byte{},
	}
}

func (f *fakeReconcileStore) GetPriceGapClosedPositionsForDate(date string) ([]string, error) {
	f.listCalls++
	if f.listErrCount > 0 {
		f.listErrCount--
		return nil, f.listErr
	}
	ids, ok := f.idsByDate[date]
	if !ok {
		return []string{}, nil
	}
	// Return a copy so callers cannot mutate fixture state inadvertently.
	out := make([]string, len(ids))
	copy(out, ids)
	return out, nil
}

func (f *fakeReconcileStore) LoadPriceGapPosition(id string) (*models.PriceGapPosition, bool, error) {
	if f.loadErr != nil {
		return nil, false, f.loadErr
	}
	pos, ok := f.positions[id]
	if !ok {
		return nil, false, nil
	}
	return pos, true, nil
}

func (f *fakeReconcileStore) SavePriceGapReconcileDaily(date string, payload []byte) error {
	cp := make([]byte, len(payload))
	copy(cp, payload)
	f.savedPayloads[date] = cp
	return nil
}

func (f *fakeReconcileStore) LoadPriceGapReconcileDaily(date string) ([]byte, bool, error) {
	p, ok := f.savedPayloads[date]
	if !ok {
		return nil, false, nil
	}
	cp := make([]byte, len(p))
	copy(cp, p)
	return cp, true, nil
}

// reconcileFailureArgs / reconcileDigestArgs — captured arguments for assert.
type reconcileFailureArgs struct {
	date string
	err  error
}

type reconcileDigestArgs struct {
	date   string
	record DailyReconcileRecord
	ramp   models.RampState
}

// fakeReconcileNotifier — captures Digest + Failure calls. Plan 14-04 ships
// the real Telegram impl; this fake is sufficient for Plan 14-02 tests.
type fakeReconcileNotifier struct {
	digestCalls  []reconcileDigestArgs
	failureCalls []reconcileFailureArgs
	// Embed the package's noop notifier so adding more PriceGapNotifier methods
	// in later plans does not break this fake. Reconciler only depends on the
	// narrow ReconcileNotifier interface — which this fake satisfies directly.
}

func (f *fakeReconcileNotifier) NotifyPriceGapDailyDigest(date string, record DailyReconcileRecord, ramp models.RampState) {
	f.digestCalls = append(f.digestCalls, reconcileDigestArgs{date: date, record: record, ramp: ramp})
}

func (f *fakeReconcileNotifier) NotifyPriceGapReconcileFailure(date string, err error) {
	f.failureCalls = append(f.failureCalls, reconcileFailureArgs{date: date, err: err})
}

// fakeSleeper records every call to Sleep without actually waiting. Tests
// verify exactly which delays were used.
type fakeSleeper struct {
	slept []time.Duration
}

func (f *fakeSleeper) Sleep(d time.Duration) {
	f.slept = append(f.slept, d)
}

// reconcilerFixture builds a Reconciler bound to fresh fakes + a fixed-clock
// nowFn. Tests further configure the fakes and call RunForDate.
type reconcilerFixture struct {
	store    *fakeReconcileStore
	notifier *fakeReconcileNotifier
	sleeper  *fakeSleeper
	cfg      *config.Config
	rec      *Reconciler
	now      time.Time
}

func newReconcilerFixture(t *testing.T) *reconcilerFixture {
	t.Helper()
	store := newFakeReconcileStore()
	notifier := &fakeReconcileNotifier{}
	sleeper := &fakeSleeper{}
	cfg := &config.Config{
		PriceGapAnomalySlippageBps: 50,
	}
	now := time.Date(2026, 4, 30, 0, 30, 0, 0, time.UTC)
	rec := NewReconciler(store, notifier, cfg, utils.NewLogger("reconcile-test"))
	rec.SetNowFunc(func() time.Time { return now })
	rec.SetSleepFunc(sleeper.Sleep)
	return &reconcilerFixture{
		store:    store,
		notifier: notifier,
		sleeper:  sleeper,
		cfg:      cfg,
		rec:      rec,
		now:      now,
	}
}

// makePos builds a closed PriceGapPosition fixture for a given date.
// exchangeClosedAt zero -> reconciler MUST fall back to localClosedAt (D-10).
func makePos(id string, version int, exchangeClosedAt, localClosedAt time.Time, pnl, slipBps float64) *models.PriceGapPosition {
	return &models.PriceGapPosition{
		ID:               id,
		Symbol:           "TESTUSDT",
		LongExchange:     "binance",
		ShortExchange:    "bybit",
		Status:           models.PriceGapStatusClosed,
		Mode:             models.PriceGapModeLive,
		Version:          version,
		ExchangeClosedAt: exchangeClosedAt,
		ClosedAt:         localClosedAt,
		RealizedPnL:      pnl,
		RealizedSlipBps:  slipBps,
	}
}

// TestReconcile_Idempotency_ByteEqual — D-04 lock test.
// Running RunForDate twice on the same fixture writes byte-identical payloads.
func TestReconcile_Idempotency_ByteEqual(t *testing.T) {
	f := newReconcilerFixture(t)
	date := "2026-04-29"

	// 3 closed positions with deterministic timestamps. All slippage under
	// the 50 bps threshold so no anomaly flags fire (clean baseline for
	// idempotency proof).
	posA := makePos("POS_A", 1,
		time.Date(2026, 4, 29, 8, 0, 0, 0, time.UTC),
		time.Date(2026, 4, 29, 8, 0, 1, 0, time.UTC),
		5.5, 10)
	posB := makePos("POS_B", 1,
		time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC),
		time.Date(2026, 4, 29, 12, 0, 1, 0, time.UTC),
		-3.0, 12)
	posC := makePos("POS_C", 1,
		time.Date(2026, 4, 29, 16, 0, 0, 0, time.UTC),
		time.Date(2026, 4, 29, 16, 0, 1, 0, time.UTC),
		8.0, 8)
	f.store.positions[posA.ID] = posA
	f.store.positions[posB.ID] = posB
	f.store.positions[posC.ID] = posC
	// Intentionally UNSORTED — proves the Reconciler sorts before writing.
	f.store.idsByDate[date] = []string{"POS_C", "POS_A", "POS_B"}

	ctx := context.Background()
	if err := f.rec.RunForDate(ctx, date); err != nil {
		t.Fatalf("RunForDate first call: unexpected err: %v", err)
	}
	first, ok := f.store.savedPayloads[date]
	if !ok {
		t.Fatalf("expected payload saved after first RunForDate")
	}
	firstCopy := make([]byte, len(first))
	copy(firstCopy, first)

	if err := f.rec.RunForDate(ctx, date); err != nil {
		t.Fatalf("RunForDate second call: unexpected err: %v", err)
	}
	second := f.store.savedPayloads[date]
	if !bytes.Equal(firstCopy, second) {
		t.Fatalf("D-04 idempotency violation: payload differs across runs\nfirst:  %s\nsecond: %s",
			string(firstCopy), string(second))
	}

	// Decode + assert ordering and frozen ComputedAt.
	var rec DailyReconcileRecord
	if err := json.Unmarshal(firstCopy, &rec); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if rec.SchemaVersion != reconcileSchemaVersion {
		t.Errorf("schema_version: got %d, want %d", rec.SchemaVersion, reconcileSchemaVersion)
	}
	if !rec.ComputedAt.Equal(f.now) {
		t.Errorf("ComputedAt: got %v, want frozen %v (proves nowFn used, not time.Now)", rec.ComputedAt, f.now)
	}
	gotIDs := []string{}
	for _, p := range rec.Positions {
		gotIDs = append(gotIDs, p.ID)
	}
	wantIDs := []string{"POS_A", "POS_B", "POS_C"}
	if !equalStrings(gotIDs, wantIDs) {
		t.Errorf("position order: got %v, want %v (sort by (ExchangeClosedAt,ID) ASC)", gotIDs, wantIDs)
	}
	if rec.Totals.PositionsClosed != 3 {
		t.Errorf("PositionsClosed: got %d, want 3", rec.Totals.PositionsClosed)
	}
	wantPnL := 5.5 + (-3.0) + 8.0
	if rec.Totals.RealizedPnLUSDT != wantPnL {
		t.Errorf("RealizedPnLUSDT: got %v, want %v", rec.Totals.RealizedPnLUSDT, wantPnL)
	}
	if rec.Totals.Wins != 2 || rec.Totals.Losses != 1 {
		t.Errorf("wins/losses: got %d/%d, want 2/1", rec.Totals.Wins, rec.Totals.Losses)
	}
	if !rec.Totals.NetClean {
		t.Errorf("NetClean: got false, want true (positive PnL with >=1 closed)")
	}
	if rec.Anomalies.HighSlippageCount != 0 || rec.Anomalies.MissingCloseTsCount != 0 {
		t.Errorf("anomalies: got high=%d missing=%d, want both 0",
			rec.Anomalies.HighSlippageCount, rec.Anomalies.MissingCloseTsCount)
	}
	// FlaggedIDs MUST be empty slice (not nil) so JSON output is "[]" not "null"
	// — preserves byte-equality with future no-anomaly days.
	if rec.Anomalies.FlaggedIDs == nil {
		t.Errorf("FlaggedIDs: nil; expected non-nil empty slice for byte-equality")
	}
	if len(rec.Anomalies.FlaggedIDs) != 0 {
		t.Errorf("FlaggedIDs: got %v, want empty", rec.Anomalies.FlaggedIDs)
	}
}

// TestReconcile_MissingExchangeCloseTs_FallsBackToLocal — D-10 contract.
// Position with zero ExchangeClosedAt is INCLUDED in aggregation; its
// FallbackLocalCloseAt+AnomalyMissingCloseTs flags fire; ID surfaces in
// FlaggedIDs.
func TestReconcile_MissingExchangeCloseTs_FallsBackToLocal(t *testing.T) {
	f := newReconcilerFixture(t)
	date := "2026-04-29"

	posA := makePos("POS_A", 1,
		time.Date(2026, 4, 29, 8, 0, 0, 0, time.UTC),
		time.Date(2026, 4, 29, 8, 0, 1, 0, time.UTC),
		5.5, 10)
	// POS_B: ExchangeClosedAt is the zero time -> Reconciler MUST fall back
	// to ClosedAt and stamp the anomaly flag.
	posB := makePos("POS_B", 1,
		time.Time{},
		time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC),
		-3.0, 12)
	posC := makePos("POS_C", 1,
		time.Date(2026, 4, 29, 16, 0, 0, 0, time.UTC),
		time.Date(2026, 4, 29, 16, 0, 1, 0, time.UTC),
		8.0, 8)
	f.store.positions[posA.ID] = posA
	f.store.positions[posB.ID] = posB
	f.store.positions[posC.ID] = posC
	f.store.idsByDate[date] = []string{"POS_A", "POS_B", "POS_C"}

	if err := f.rec.RunForDate(context.Background(), date); err != nil {
		t.Fatalf("RunForDate: unexpected err: %v", err)
	}
	payload := f.store.savedPayloads[date]
	if payload == nil {
		t.Fatalf("expected payload saved")
	}
	var rec DailyReconcileRecord
	if err := json.Unmarshal(payload, &rec); err != nil {
		t.Fatalf("decode: %v", err)
	}
	// All 3 positions still in record (D-10: do NOT drop, flag instead).
	if rec.Totals.PositionsClosed != 3 {
		t.Errorf("PositionsClosed: got %d, want 3 (missing close ts must NOT exclude position)",
			rec.Totals.PositionsClosed)
	}
	if rec.Anomalies.MissingCloseTsCount != 1 {
		t.Errorf("MissingCloseTsCount: got %d, want 1", rec.Anomalies.MissingCloseTsCount)
	}
	// Find POS_B in the record and assert flags.
	var posBRec *DailyReconcilePosition
	for i := range rec.Positions {
		if rec.Positions[i].ID == "POS_B" {
			posBRec = &rec.Positions[i]
			break
		}
	}
	if posBRec == nil {
		t.Fatalf("POS_B missing from record positions")
	}
	if !posBRec.FallbackLocalCloseAt {
		t.Errorf("POS_B.FallbackLocalCloseAt: got false, want true")
	}
	if !posBRec.AnomalyMissingCloseTs {
		t.Errorf("POS_B.AnomalyMissingCloseTs: got false, want true")
	}
	// ExchangeClosedAt should equal ClosedAt (the fallback source).
	if !posBRec.ExchangeClosedAt.Equal(posB.ClosedAt) {
		t.Errorf("POS_B.ExchangeClosedAt: got %v, want fallback to ClosedAt %v",
			posBRec.ExchangeClosedAt, posB.ClosedAt)
	}
	// FlaggedIDs contains POS_B.
	if !containsString(rec.Anomalies.FlaggedIDs, "POS_B") {
		t.Errorf("FlaggedIDs: got %v, want to contain POS_B", rec.Anomalies.FlaggedIDs)
	}
}

// TestReconcile_TripleFail_SkipsDay — D-03 contract + Q1 retry shape.
// 3 consecutive errors -> RunForDate returns the error, never writes payload,
// fires Notifier.NotifyPriceGapReconcileFailure exactly once.
//
// Q1 retry shape:
//
//	delays := []time.Duration{5*time.Second, 15*time.Second, 30*time.Second}
//	for attempt, d := range delays {
//	    if attempt > 0 {
//	        sleepFn(d)
//	    }
//	    err := aggregateAndWrite(ctx, date)
//	    if err == nil { return nil }
//	}
//
// Attempt 0: no sleep, fails -> attempt 1: sleep delays[1]=15s, fails ->
// attempt 2: sleep delays[2]=30s, fails -> return err.
// fakeSleeper.slept MUST be [15s, 30s] — 2 entries.
func TestReconcile_TripleFail_SkipsDay(t *testing.T) {
	f := newReconcilerFixture(t)
	date := "2026-04-29"

	listErr := errors.New("redis down")
	f.store.listErr = listErr
	f.store.listErrCount = 3 // fail every attempt

	err := f.rec.RunForDate(context.Background(), date)
	if err == nil {
		t.Fatalf("expected error after triple-fail, got nil")
	}
	if !errors.Is(err, listErr) {
		t.Errorf("returned err: got %v, want wrapping listErr", err)
	}

	if len(f.store.savedPayloads) != 0 {
		t.Errorf("savedPayloads: got %d entries, want 0 (triple-fail must not write)", len(f.store.savedPayloads))
	}
	if len(f.notifier.failureCalls) != 1 {
		t.Fatalf("failureCalls: got %d, want 1", len(f.notifier.failureCalls))
	}
	fc := f.notifier.failureCalls[0]
	if fc.date != date {
		t.Errorf("failure.date: got %q, want %q", fc.date, date)
	}
	if !errors.Is(fc.err, listErr) {
		t.Errorf("failure.err: got %v, want wrapping listErr", fc.err)
	}
	// Q1 retry shape: 2 sleeps applied (between attempts 1->2 and 2->3),
	// not before attempt 1, not after attempt 3. Values are delays[1] and
	// delays[2] = 15s and 30s respectively.
	wantSleeps := []time.Duration{15 * time.Second, 30 * time.Second}
	if !equalDurations(f.sleeper.slept, wantSleeps) {
		t.Errorf("sleep durations: got %v, want %v", f.sleeper.slept, wantSleeps)
	}
	// store.GetPriceGapClosedPositionsForDate should have been called exactly 3 times.
	if f.store.listCalls != 3 {
		t.Errorf("list calls: got %d, want 3", f.store.listCalls)
	}
}

// TestReconcile_AnomalyHighSlippage_Flagged — D-09 anomaly threshold.
// Position with abs(RealizedSlipBps) > cfg.PriceGapAnomalySlippageBps fires
// AnomalyHighSlippage; ID surfaces in FlaggedIDs; counter increments.
func TestReconcile_AnomalyHighSlippage_Flagged(t *testing.T) {
	f := newReconcilerFixture(t)
	date := "2026-04-29"

	// POS_A slip 75 bps > 50 threshold -> flagged.
	posA := makePos("POS_A", 1,
		time.Date(2026, 4, 29, 8, 0, 0, 0, time.UTC),
		time.Date(2026, 4, 29, 8, 0, 1, 0, time.UTC),
		5.5, 75)
	posB := makePos("POS_B", 1,
		time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC),
		time.Date(2026, 4, 29, 12, 0, 1, 0, time.UTC),
		-3.0, 12)
	f.store.positions[posA.ID] = posA
	f.store.positions[posB.ID] = posB
	f.store.idsByDate[date] = []string{"POS_A", "POS_B"}

	if err := f.rec.RunForDate(context.Background(), date); err != nil {
		t.Fatalf("RunForDate: %v", err)
	}
	var rec DailyReconcileRecord
	if err := json.Unmarshal(f.store.savedPayloads[date], &rec); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if rec.Anomalies.HighSlippageCount != 1 {
		t.Errorf("HighSlippageCount: got %d, want 1", rec.Anomalies.HighSlippageCount)
	}
	var posARec *DailyReconcilePosition
	for i := range rec.Positions {
		if rec.Positions[i].ID == "POS_A" {
			posARec = &rec.Positions[i]
			break
		}
	}
	if posARec == nil {
		t.Fatalf("POS_A missing from record positions")
	}
	if !posARec.AnomalyHighSlippage {
		t.Errorf("POS_A.AnomalyHighSlippage: got false, want true (75 > 50)")
	}
	if !containsString(rec.Anomalies.FlaggedIDs, "POS_A") {
		t.Errorf("FlaggedIDs: got %v, want to contain POS_A", rec.Anomalies.FlaggedIDs)
	}
	// FlaggedIDs MUST be sorted ASC for determinism.
	sortedCopy := append([]string{}, rec.Anomalies.FlaggedIDs...)
	sort.Strings(sortedCopy)
	if !equalStrings(rec.Anomalies.FlaggedIDs, sortedCopy) {
		t.Errorf("FlaggedIDs not sorted ASC: got %v, sorted would be %v",
			rec.Anomalies.FlaggedIDs, sortedCopy)
	}
}

// equalStrings — cheap slice comparison without importing reflect.
func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func equalDurations(a, b []time.Duration) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func containsString(s []string, t string) bool {
	for _, v := range s {
		if v == t {
			return true
		}
	}
	return false
}
