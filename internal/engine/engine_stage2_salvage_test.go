// Package engine — unit tests for the Stage 2 DB-failure salvage plan.
//
// Covers 9 scenarios (T1, T2a, T2b, T3, T4, T5, T6, T7, T8) described in
// plans/PLAN-stage2-db-failure-salvage.md §5 (v6 — current).
//
// Architecture notes:
//   - Engine is built via struct literal (mirrors buildRankFirstEngine pattern).
//   - activePosSource is explicitly set to e.db (REQUIRED — would nil-deref otherwise).
//   - Tests drive runStage2AndSalvage() directly — no full run loop needed.
//   - executeArbitrage calls are captured via testExecuteArbitrageHook.
//   - RescanSymbols calls are captured via testRescanHook.
//   - DB failure injection uses failingActivePosGetter which wraps the real
//     miniredis-backed *database.Client and fails the first N calls.
//   - failN/calls counts are TOTAL across both Stage 2 and salvage call sites
//     because they share the same e.activePosSource seam.
package engine

import (
	"context"
	"errors"
	"testing"

	"arb/internal/config"
	"arb/internal/database"
	"arb/internal/discovery"
	"arb/internal/models"
	"arb/internal/risk"
	"arb/pkg/exchange"
	"arb/pkg/utils"

	"github.com/alicebob/miniredis/v2"
)

// ---------------------------------------------------------------------------
// DB failure stub
// ---------------------------------------------------------------------------

// failingActivePosGetter wraps a real activePositionsGetter and returns an
// error for the first failN calls, then delegates to the real implementation.
// calls is a total-call counter across ALL invocations (Stage 2 + salvage share
// the same seam), not a per-phase counter.
type failingActivePosGetter struct {
	real  activePositionsGetter
	failN int
	calls int
}

func (f *failingActivePosGetter) GetActivePositions() ([]*models.ArbitragePosition, error) {
	f.calls++
	if f.calls <= f.failN {
		return nil, errors.New("forced: db unavailable")
	}
	return f.real.GetActivePositions()
}

// ---------------------------------------------------------------------------
// Functional option type
// ---------------------------------------------------------------------------

type option func(*Engine)

// withFailingActivePosGetter swaps e.activePosSource for a failingActivePosGetter
// that fails the first failN calls, then delegates to the real e.db.
// Must be applied AFTER the engine is built (so e.db is already set).
func withFailingActivePosGetter(failN int) option {
	return func(e *Engine) {
		e.activePosSource = &failingActivePosGetter{real: e.db, failN: failN}
	}
}

// ---------------------------------------------------------------------------
// Test harness builder
// ---------------------------------------------------------------------------

// newMiniredisDB creates a miniredis-backed *database.Client and returns both
// so tests can inspect the miniredis state or control it.
func newMiniredisDB(t *testing.T) (*miniredis.Miniredis, *database.Client) {
	t.Helper()
	mr := miniredis.RunT(t)
	db, err := database.New(mr.Addr(), "", 2)
	if err != nil {
		t.Fatalf("database.New: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return mr, db
}

// buildStage2SalvageEngine creates an Engine wired with real miniredis DB.
// It mirrors the buildRankFirstEngine struct-literal pattern and EXPLICITLY
// sets e.activePosSource = e.db (required: zero-value nil would nil-deref in
// getActivePositionsWithRetry). Options are applied after the field is set so
// tests that need failure injection can override activePosSource.
func buildStage2SalvageEngine(t *testing.T, opts ...option) (*Engine, *miniredis.Miniredis) {
	t.Helper()
	mr, db := newMiniredisDB(t)
	cfg := &config.Config{
		MaxPositions:            5,
		CapitalPerLeg:           100,
		Leverage:                2,
		MarginSafetyMultiplier:  2.0,
		MarginL4Threshold:       0.80,
		EnablePoolAllocator:     true,
	}
	exchanges := map[string]exchange.Exchange{
		"binance": newRankFirstStub(300, 1000, 0),
		"bybit":   newRankFirstStub(300, 1000, 0),
	}
	riskMgr := risk.NewManager(exchanges, db, cfg, nil)
	scanner := discovery.NewScanner(exchanges, db, cfg)
	e := &Engine{
		cfg:             cfg,
		log:             utils.NewLogger("test"),
		db:              db,
		risk:            riskMgr,
		discovery:       scanner,
		exchanges:       exchanges,
		allocOverrides:  nil,
		slIndex:         make(map[string]stopOrderEntry),
		slFillCh:        make(chan slFillEvent, 16),
		exitCancels:     make(map[string]context.CancelFunc),
		exitActive:      make(map[string]bool),
		exitDone:        make(map[string]chan struct{}),
		preSettleActive: make(map[string]bool),
		entryActive:     make(map[string]string),
		apiErrCounts:    make(map[string]int),
		depthRefs:       make(map[string]*depthRef),
	}
	// REQUIRED: mirror NewEngine constructor wire-up. Without this,
	// getActivePositionsWithRetry nil-derefs on the zero-value interface field.
	e.activePosSource = e.db
	for _, opt := range opts {
		opt(e)
	}
	return e, mr
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// captureArb installs a testExecuteArbitrageHook and returns a pointer to the
// accumulated list of symbol slices passed to executeArbitrage across all calls.
func captureArb(e *Engine) *[][]string {
	var calls [][]string
	e.testExecuteArbitrageHook = func(opps []models.Opportunity) {
		syms := make([]string, len(opps))
		for i, o := range opps {
			syms[i] = o.Symbol
		}
		calls = append(calls, syms)
	}
	return &calls
}

// captureRescan installs a testRescanHook that records which symbol sets were
// requested and returns the provided stub result.
func captureRescan(e *Engine, result []models.Opportunity) *[][]string {
	var calls [][]string
	e.testRescanHook = func(pairs []models.SymbolPair) []models.Opportunity {
		syms := make([]string, len(pairs))
		for i, p := range pairs {
			syms[i] = p.Symbol
		}
		calls = append(calls, syms)
		return result
	}
	return &calls
}

// seedPosition writes an active (non-closed) position for the given symbol to
// the miniredis-backed DB, simulating Stage 1 having opened it.
func seedPosition(t *testing.T, db *database.Client, symbol string) {
	t.Helper()
	pos := &models.ArbitragePosition{
		ID:            "pos-" + symbol,
		Symbol:        symbol,
		Status:        models.StatusActive,
		LongExchange:  "binance",
		ShortExchange: "bybit",
	}
	if err := db.SavePosition(pos); err != nil {
		t.Fatalf("seedPosition %s: %v", symbol, err)
	}
}

// setOverride sets a single allocator override on the engine.
func setOverride(e *Engine, sym, long, short string) {
	e.allocOverrideMu.Lock()
	if e.allocOverrides == nil {
		e.allocOverrides = make(map[string]allocatorChoice)
	}
	e.allocOverrides[sym] = allocatorChoice{symbol: sym, longExchange: long, shortExchange: short}
	e.allocOverrideMu.Unlock()
}

// opp returns a minimal opportunity with matching primary pair so that
// applyAllocatorOverrides keeps it (primary pair == override pair).
func opp(sym, long, short string) models.Opportunity {
	return models.Opportunity{
		Symbol:        sym,
		LongExchange:  long,
		ShortExchange: short,
		Spread:        5.0,
		Score:         10,
	}
}

// sortedSyms returns a sorted copy of a []string (for deterministic comparison
// when order may vary due to map iteration).
func sortedSyms(ss []string) []string {
	out := make([]string, len(ss))
	copy(out, ss)
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[i] > out[j] {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

// containsSymbol returns true if sym is in the slice.
func containsSymbol(syms []string, sym string) bool {
	for _, s := range syms {
		if s == sym {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// T1: Happy path — Stage 2 executes [B], salvage noop
// Override {B}. Stage 1 opps = [A, B]. S1 opens {A}.
// Stage 2 DB OK: openedS1={A}. stage2Attempted={B}. Execs [B].
// Salvage: openedByUs={A,B} after Stage 2 → B excluded. No salvage exec.
// ---------------------------------------------------------------------------

func TestStage2Salvage_T1_HappyPath(t *testing.T) {
	e, _ := buildStage2SalvageEngine(t)
	arbCalls := captureArb(e)
	captureRescan(e, nil) // salvage should not call rescan

	setOverride(e, "BTOKEN", "binance", "bybit")

	// Simulate Stage 1 opened ATOKEN.
	seedPosition(t, e.db, "ATOKEN")

	opps := []models.Opportunity{
		opp("ATOKEN", "binance", "bybit"),
		opp("BTOKEN", "binance", "bybit"),
	}

	e.runStage2AndSalvage(opps, 0)

	if len(*arbCalls) != 1 {
		t.Fatalf("T1: want 1 executeArbitrage call (Stage 2), got %d: %v", len(*arbCalls), *arbCalls)
	}
	if !containsSymbol((*arbCalls)[0], "BTOKEN") {
		t.Errorf("T1: Stage 2 call should contain BTOKEN, got %v", (*arbCalls)[0])
	}
}

// ---------------------------------------------------------------------------
// T2a: REGRESSION for Finding #1 — Stage 2 DB fails both attempts.
// In-scan override recovered by salvage.
// Override {B}. Stage 1 opps=[A,B]. S1 opens {A}.
// failN=2: Stage 2 calls 1+2 fail → Stage 2 bails (no executeArbitrage for patched).
// Salvage: call 3 succeeds → salvage rescans {B} and execs it.
// Total calls == 3 asserted.
// ---------------------------------------------------------------------------

func TestStage2Salvage_T2a_Stage2DBFailBothAttemptsInScanRecoveredBySalvage(t *testing.T) {
	stub := &failingActivePosGetter{failN: 2}
	e, _ := buildStage2SalvageEngine(t, func(eng *Engine) {
		stub.real = eng.db
		eng.activePosSource = stub
	})
	arbCalls := captureArb(e)
	// Salvage rescan returns one opp so executeArbitrage is called once via salvage.
	captureRescan(e, []models.Opportunity{opp("BTOKEN", "binance", "bybit")})

	setOverride(e, "BTOKEN", "binance", "bybit")
	seedPosition(t, e.db, "ATOKEN")

	opps := []models.Opportunity{
		opp("ATOKEN", "binance", "bybit"),
		opp("BTOKEN", "binance", "bybit"),
	}

	e.runStage2AndSalvage(opps, 0)

	// Stage 2 DB failed twice → stage2Attempted empty → salvage rescans B.
	// Salvage exec returns opp B → executeArbitrage called once (salvage path).
	if len(*arbCalls) != 1 {
		t.Fatalf("T2a: want 1 executeArbitrage call (salvage), got %d: %v", len(*arbCalls), *arbCalls)
	}
	if !containsSymbol((*arbCalls)[0], "BTOKEN") {
		t.Errorf("T2a: salvage executeArbitrage should contain BTOKEN, got %v", (*arbCalls)[0])
	}
	if stub.calls != 3 {
		t.Errorf("T2a: want 3 total GetActivePositions calls (2 Stage2 fail + 1 salvage success), got %d", stub.calls)
	}
}

// ---------------------------------------------------------------------------
// T2b: Retry success — Stage 2 DB fails first attempt, succeeds second.
// Override {B}. Stage 1 opps=[A,B]. S1 opens {A}.
// failN=1: Stage 2 call 1 fails, call 2 succeeds → Stage 2 executes [B].
// Salvage: call 3 succeeds → stage2Attempted={B} → salvage skips B (noop).
// Total calls == 3 asserted.
// ---------------------------------------------------------------------------

func TestStage2Salvage_T2b_RetrySuccess(t *testing.T) {
	stub := &failingActivePosGetter{failN: 1}
	e, _ := buildStage2SalvageEngine(t, func(eng *Engine) {
		stub.real = eng.db
		eng.activePosSource = stub
	})
	arbCalls := captureArb(e)
	captureRescan(e, nil) // salvage should not rescan

	setOverride(e, "BTOKEN", "binance", "bybit")
	seedPosition(t, e.db, "ATOKEN")

	opps := []models.Opportunity{
		opp("ATOKEN", "binance", "bybit"),
		opp("BTOKEN", "binance", "bybit"),
	}

	e.runStage2AndSalvage(opps, 0)

	// Stage 2 retry succeeded → stage2Attempted={BTOKEN}.
	// Salvage: BTOKEN excluded via stage2Attempted. No salvage exec.
	if len(*arbCalls) != 1 {
		t.Fatalf("T2b: want 1 executeArbitrage call (Stage 2), got %d: %v", len(*arbCalls), *arbCalls)
	}
	if !containsSymbol((*arbCalls)[0], "BTOKEN") {
		t.Errorf("T2b: Stage 2 call should contain BTOKEN, got %v", (*arbCalls)[0])
	}
	if stub.calls != 3 {
		t.Errorf("T2b: want 3 total GetActivePositions calls (1 Stage2 fail + 1 Stage2 success + 1 salvage), got %d", stub.calls)
	}
}

// ---------------------------------------------------------------------------
// T3: Out-of-scan override recovered by salvage (Bug-7 existing behavior preserved).
// Stage 1 opps=[A]. Override {B} (not in scan). S1 opens {A}.
// Stage 2: applyAllocatorOverrides iterates [A], B not in opps → filtered=nil, hadOverrides=true.
// stage2Attempted empty. Salvage: B not in stage2Attempted, not in openedByUs → rescan.
// ---------------------------------------------------------------------------

func TestStage2Salvage_T3_OutOfScanOverrideRecoveredBySalvage(t *testing.T) {
	e, _ := buildStage2SalvageEngine(t)
	arbCalls := captureArb(e)
	rescanCalls := captureRescan(e, []models.Opportunity{opp("BTOKEN", "binance", "bybit")})

	setOverride(e, "BTOKEN", "binance", "bybit")
	seedPosition(t, e.db, "ATOKEN")

	// BTOKEN is NOT in opps — it dropped out of the current scan.
	opps := []models.Opportunity{
		opp("ATOKEN", "binance", "bybit"),
	}

	e.runStage2AndSalvage(opps, 0)

	// Stage 2: patched=nil (B not in opps). stage2Attempted empty.
	// Salvage: B rescanned, returns opp → executeArbitrage called.
	if len(*rescanCalls) != 1 {
		t.Fatalf("T3: want 1 rescan call, got %d: %v", len(*rescanCalls), *rescanCalls)
	}
	if !containsSymbol((*rescanCalls)[0], "BTOKEN") {
		t.Errorf("T3: rescan should include BTOKEN, got %v", (*rescanCalls)[0])
	}
	if len(*arbCalls) != 1 {
		t.Fatalf("T3: want 1 executeArbitrage call (salvage), got %d", len(*arbCalls))
	}
	if !containsSymbol((*arbCalls)[0], "BTOKEN") {
		t.Errorf("T3: salvage exec should contain BTOKEN, got %v", (*arbCalls)[0])
	}
}

// ---------------------------------------------------------------------------
// T4: Both DB reads fail → clean skip, no crash, no duplicate.
// Override {B}. Stage 1 opps=[A,B]. failN=4.
// Stage 2: calls 1+2 fail → stage2Attempted empty.
// Salvage: calls 3+4 fail → warn logged, no exec.
// Total calls == 4 asserted.
// ---------------------------------------------------------------------------

func TestStage2Salvage_T4_BothDBReadsFail(t *testing.T) {
	stub := &failingActivePosGetter{failN: 4}
	e, _ := buildStage2SalvageEngine(t, func(eng *Engine) {
		stub.real = eng.db
		eng.activePosSource = stub
	})
	arbCalls := captureArb(e)
	captureRescan(e, nil)

	setOverride(e, "BTOKEN", "binance", "bybit")
	seedPosition(t, e.db, "ATOKEN")

	opps := []models.Opportunity{
		opp("ATOKEN", "binance", "bybit"),
		opp("BTOKEN", "binance", "bybit"),
	}

	e.runStage2AndSalvage(opps, 0)

	// No executeArbitrage calls — both Stage 2 and salvage DB reads failed.
	if len(*arbCalls) != 0 {
		t.Errorf("T4: want 0 executeArbitrage calls, got %d: %v", len(*arbCalls), *arbCalls)
	}
	if stub.calls != 4 {
		t.Errorf("T4: want 4 total GetActivePositions calls (2 Stage2 fail + 2 salvage fail), got %d", stub.calls)
	}
}

// ---------------------------------------------------------------------------
// T5: Stage 1 already opened override symbol → Stage 2 + salvage both skip.
// Override {B}. Stage 1 opps=[A,B]. S1 opens {A, B}.
// Stage 2: openedS1={A,B} → B dropped as duplicate (stage2Attempted={B} still set).
// Salvage: B excluded via stage2Attempted (AND openedByUs). No dup open.
// ---------------------------------------------------------------------------

func TestStage2Salvage_T5_Stage1AlreadyOpenedOverrideSymbol(t *testing.T) {
	e, _ := buildStage2SalvageEngine(t)
	arbCalls := captureArb(e)
	captureRescan(e, nil)

	setOverride(e, "BTOKEN", "binance", "bybit")

	// Stage 1 opened both A and B.
	seedPosition(t, e.db, "ATOKEN")
	seedPosition(t, e.db, "BTOKEN")

	opps := []models.Opportunity{
		opp("ATOKEN", "binance", "bybit"),
		opp("BTOKEN", "binance", "bybit"),
	}

	e.runStage2AndSalvage(opps, 0)

	// Stage 2: B dropped (openedS1), no stage2Opps → no executeArbitrage.
	// Salvage: B in stage2Attempted → skipped. No duplicate open.
	if len(*arbCalls) != 0 {
		t.Errorf("T5: want 0 executeArbitrage calls (B already opened by S1), got %d: %v", len(*arbCalls), *arbCalls)
	}
}

// ---------------------------------------------------------------------------
// T6: All-stale — applyAllocatorOverrides returns (nil, true).
// Override {B} with stale alt (override pair ≠ primary, not in alternatives).
// Stage 1 opps=[A]. Stage 2: patched=nil, hadOverrides=true.
// stage2Attempted empty. Salvage MUST call RescanSymbols({B}).
// Test controls stub: rescan returns empty → no executeArbitrage call.
// ---------------------------------------------------------------------------

func TestStage2Salvage_T6_AllStale(t *testing.T) {
	e, _ := buildStage2SalvageEngine(t)
	arbCalls := captureArb(e)
	rescanCalls := captureRescan(e, nil) // rescan returns empty

	// Override B with a pair that won't match primary or alternatives.
	setOverride(e, "BTOKEN", "okx", "gateio") // stale pair — not primary (binance/bybit) and no alternatives

	seedPosition(t, e.db, "ATOKEN")

	// opps contains B but with a DIFFERENT primary pair than the override wants.
	// applyAllocatorOverrides: B found in overrides, but override wants okx/gateio
	// while opp says binance/bybit. It checks alternatives — none provided →
	// "no longer in current scan, skipping" → stale. filtered=nil → returns (nil, true).
	opps := []models.Opportunity{
		opp("ATOKEN", "binance", "bybit"),
		opp("BTOKEN", "binance", "bybit"), // primary doesn't match override pair; no alternatives
	}

	e.runStage2AndSalvage(opps, 0)

	// Stage 2: patched=nil, hadOverrides=true → warn "all stale". stage2Attempted empty.
	// Salvage: B in overrideSnapshot, not in stage2Attempted, not in openedByUs
	// → missingPairs includes B → rescan called.
	if len(*rescanCalls) != 1 {
		t.Fatalf("T6: want 1 rescan call (salvage must rescan B), got %d: %v", len(*rescanCalls), *rescanCalls)
	}
	if !containsSymbol((*rescanCalls)[0], "BTOKEN") {
		t.Errorf("T6: rescan should include BTOKEN, got %v", (*rescanCalls)[0])
	}
	// Rescan returned empty → no executeArbitrage call.
	if len(*arbCalls) != 0 {
		t.Errorf("T6: want 0 executeArbitrage calls (rescan returned empty), got %d: %v", len(*arbCalls), *arbCalls)
	}
}

// ---------------------------------------------------------------------------
// T7: Multi-symbol partial — Stage 1 opens A, Stage 2 opens B, salvage noop.
// Override {A, B}. Stage 1 opps=[A,B]. S1 opens {A}.
// applyAllocatorOverrides returns [A_patched, B_patched].
// Stage 2: openedS1={A} → drops A as duplicate, execs [B].
// stage2Attempted={A, B}. Salvage: both excluded via stage2Attempted. No salvage call.
// ---------------------------------------------------------------------------

func TestStage2Salvage_T7_MultiSymbolPartial(t *testing.T) {
	e, _ := buildStage2SalvageEngine(t)
	arbCalls := captureArb(e)
	rescanCalls := captureRescan(e, nil)

	setOverride(e, "ATOKEN", "binance", "bybit")
	setOverride(e, "BTOKEN", "binance", "bybit")

	// Stage 1 opened ATOKEN.
	seedPosition(t, e.db, "ATOKEN")

	opps := []models.Opportunity{
		opp("ATOKEN", "binance", "bybit"),
		opp("BTOKEN", "binance", "bybit"),
	}

	e.runStage2AndSalvage(opps, 0)

	// Stage 2 should execute exactly [B] (A dropped as duplicate).
	if len(*arbCalls) != 1 {
		t.Fatalf("T7: want 1 executeArbitrage call (Stage 2 with B), got %d: %v", len(*arbCalls), *arbCalls)
	}
	if !containsSymbol((*arbCalls)[0], "BTOKEN") {
		t.Errorf("T7: Stage 2 call should contain BTOKEN, got %v", (*arbCalls)[0])
	}
	if containsSymbol((*arbCalls)[0], "ATOKEN") {
		t.Errorf("T7: Stage 2 call must NOT contain ATOKEN (already opened by S1), got %v", (*arbCalls)[0])
	}
	// Salvage: both A and B in stage2Attempted → no rescan.
	if len(*rescanCalls) != 0 {
		t.Errorf("T7: want 0 rescan calls (both in stage2Attempted), got %d: %v", len(*rescanCalls), *rescanCalls)
	}
}

// ---------------------------------------------------------------------------
// T8: Mixed validity — override {A, B} with B stale alt, A valid alt.
// Stage 1 opps=[A,B] with both. S1 opens nothing.
// applyAllocatorOverrides: A primary matches override → kept. B override pair
// not in alternatives → stale-skipped. Returns [A_patched].
// Stage 2: openedS1={} → execs [A]. stage2Attempted={A}.
// Salvage: iterates overrideSnapshot={A,B}. A in stage2Attempted → skip.
// B NOT in stage2Attempted, NOT in openedByUs → missingPairs={B} → rescan called.
// Test stubs rescan to return empty.
// ---------------------------------------------------------------------------

func TestStage2Salvage_T8_MixedValidity(t *testing.T) {
	e, _ := buildStage2SalvageEngine(t)
	arbCalls := captureArb(e)
	rescanCalls := captureRescan(e, nil) // rescan B returns empty

	// A: override pair matches primary (binance/bybit) → valid.
	setOverride(e, "ATOKEN", "binance", "bybit")
	// B: override wants okx/gateio but opp primary is binance/bybit with no alternatives → stale.
	setOverride(e, "BTOKEN", "okx", "gateio")

	// Stage 1 opens nothing.
	opps := []models.Opportunity{
		opp("ATOKEN", "binance", "bybit"),
		opp("BTOKEN", "binance", "bybit"), // primary ≠ override pair (okx/gateio), no alternatives
	}

	e.runStage2AndSalvage(opps, 0)

	// Stage 2: patched=[A] (B was stale-filtered). stage2Attempted={A}. Execs [A].
	if len(*arbCalls) != 1 {
		t.Fatalf("T8: want 1 executeArbitrage call (Stage 2 with A), got %d: %v", len(*arbCalls), *arbCalls)
	}
	if !containsSymbol((*arbCalls)[0], "ATOKEN") {
		t.Errorf("T8: Stage 2 call should contain ATOKEN, got %v", (*arbCalls)[0])
	}
	if containsSymbol((*arbCalls)[0], "BTOKEN") {
		t.Errorf("T8: Stage 2 call must NOT contain BTOKEN (stale-filtered), got %v", (*arbCalls)[0])
	}

	// Salvage: B not in stage2Attempted → rescan called for B.
	if len(*rescanCalls) != 1 {
		t.Fatalf("T8: want 1 rescan call for B, got %d: %v", len(*rescanCalls), *rescanCalls)
	}
	if !containsSymbol((*rescanCalls)[0], "BTOKEN") {
		t.Errorf("T8: rescan should include BTOKEN, got %v", (*rescanCalls)[0])
	}
	// A should NOT be in rescan (it was in stage2Attempted).
	if containsSymbol((*rescanCalls)[0], "ATOKEN") {
		t.Errorf("T8: rescan must NOT include ATOKEN (it was in stage2Attempted), got %v", (*rescanCalls)[0])
	}
}
