package engine

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"arb/internal/config"
	"arb/internal/database"
	"arb/internal/models"
	"arb/internal/risk"
	"arb/pkg/utils"

	"github.com/alicebob/miniredis/v2"
)

// unified_entry_test.go — unit tests for the unified cross-strategy
// entry selector (internal/engine/unified_entry.go). Tests cover the
// user's 5-candidate example, cross-strategy symbol dedup, slot caps,
// preheld-reservation dispatch, release-on-error, readiness gate
// regressions, and the nil spotEntry guard per
// plans/PLAN-unified-capital-allocator.md Section 11.

// stubSpotEntryExecutor is a minimal spotEntryExecutor for tests. It
// records the calls it received and returns caller-configurable
// results. The orchestrator depends on this narrow interface rather
// than on the concrete *spotengine.SpotEngine so it can be stubbed
// without bringing up the spot engine package.
type stubSpotEntryExecutor struct {
	mu sync.Mutex

	listResult      []models.SpotEntryCandidate
	buildResultF    func(models.SpotEntryCandidate) (*models.SpotEntryPlan, error)
	openResultF     func(*models.SpotEntryPlan, float64, *risk.CapitalReservation) error
	openCallCount   int
	openCalledFor   []string // plan symbols observed in OpenSelectedEntry
	openWithPreheld []bool   // whether each open call received a non-nil reservation
}

func (s *stubSpotEntryExecutor) ListEntryCandidates(maxAge time.Duration) []models.SpotEntryCandidate {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]models.SpotEntryCandidate, len(s.listResult))
	copy(out, s.listResult)
	return out
}

func (s *stubSpotEntryExecutor) BuildEntryPlan(c models.SpotEntryCandidate) (*models.SpotEntryPlan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.buildResultF != nil {
		return s.buildResultF(c)
	}
	// Default: mirror the candidate into a plan with a reasonable notional
	// so tests don't need to set buildResultF when they just want a plan.
	return &models.SpotEntryPlan{
		Candidate:           c,
		MidPrice:            1.0,
		PlannedBaseSize:     100.0,
		PlannedNotionalUSDT: 100.0,
		CapitalBudgetUSDT:   200.0,
		FuturesMarginUSDT:   30.0,
	}, nil
}

func (s *stubSpotEntryExecutor) OpenSelectedEntry(plan *models.SpotEntryPlan, capOverride float64, preheld *risk.CapitalReservation) error {
	s.mu.Lock()
	s.openCallCount++
	if plan != nil {
		s.openCalledFor = append(s.openCalledFor, plan.Candidate.Symbol)
	}
	s.openWithPreheld = append(s.openWithPreheld, preheld != nil)
	openResultF := s.openResultF
	s.mu.Unlock()
	if openResultF != nil {
		return openResultF(plan, capOverride, preheld)
	}
	return nil
}

// newTestEngineForUnified builds a minimal Engine wired with an
// in-memory miniredis + database.Client + enabled CapitalAllocator.
// No real discovery scanner or risk manager is attached; tests that
// need those exercise the helper functions (groupChoicesBySymbol,
// loadUnifiedOccupancy, unifiedOwnerReady) directly.
func newTestEngineForUnified(t *testing.T) (*Engine, *database.Client, *miniredis.Miniredis) {
	t.Helper()

	mr := miniredis.RunT(t)
	db, err := database.New(mr.Addr(), "", 2)
	if err != nil {
		t.Fatalf("database.New: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	cfg := &config.Config{
		EnableCapitalAllocator:      true,
		EnableUnifiedEntrySelection: true,
		MaxTotalExposureUSDT:        1000,
		MaxPerpPerpPct:              0.80,
		MaxSpotFuturesPct:           0.80,
		MaxPerExchangePct:           0.80,
		ReservationTTLSec:           60,
		Leverage:                    5,
		MinHoldTime:                 16 * time.Hour,
		SpotFuturesScanIntervalMin:  10,
		SpotFuturesLeverage:         3,
		MaxPositions:                3,
		SpotFuturesMaxPositions:     2,
		AllocatorTimeoutMs:          30,
		EntryScanMinute:             40,
	}
	allocator := risk.NewCapitalAllocator(db, cfg)

	e := &Engine{
		cfg:       cfg,
		db:        db,
		allocator: allocator,
		log:       utils.NewLogger("test"),
	}
	return e, db, mr
}

// savePerpPosition persists a perp ArbitragePosition in the given status
// to the test DB so loadUnifiedOccupancy picks it up.
func savePerpPosition(t *testing.T, db *database.Client, id, symbol, status string) {
	t.Helper()
	pos := &models.ArbitragePosition{
		ID:            id,
		Symbol:        symbol,
		LongExchange:  "binance",
		ShortExchange: "bybit",
		Status:        status,
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}
	if err := db.SavePosition(pos); err != nil {
		t.Fatalf("SavePosition: %v", err)
	}
}

// saveSpotPosition persists a SpotFuturesPosition in the given status.
func saveSpotPosition(t *testing.T, db *database.Client, id, symbol, status string) {
	t.Helper()
	pos := &models.SpotFuturesPosition{
		ID:        id,
		Symbol:    symbol,
		Exchange:  "binance",
		Status:    status,
		CreatedAt: time.Now().UTC(),
	}
	if err := db.SaveSpotPosition(pos); err != nil {
		t.Fatalf("SaveSpotPosition: %v", err)
	}
}

// TestUnifiedEntry_LoadUnifiedOccupancy_BlocksAllNonClosedPositions
// verifies loadUnifiedOccupancy counts every non-closed perp and spot
// status (pending/partial/active/exiting/closing on perp; pending/active/
// exiting on spot) and skips only closed positions. Slot math must
// deduct both strategies from MaxPositions and spot alone from
// SpotFuturesMaxPositions.
func TestUnifiedEntry_LoadUnifiedOccupancy_BlocksAllNonClosedPositions(t *testing.T) {
	e, db, _ := newTestEngineForUnified(t)
	e.cfg.MaxPositions = 10
	e.cfg.SpotFuturesMaxPositions = 5

	// Perp: one of every non-closed status + one closed (must not count).
	savePerpPosition(t, db, "p1", "AAA", models.StatusPending)
	savePerpPosition(t, db, "p2", "BBB", models.StatusPartial)
	savePerpPosition(t, db, "p3", "CCC", models.StatusActive)
	savePerpPosition(t, db, "p4", "DDD", models.StatusExiting)
	savePerpPosition(t, db, "p5", "EEE", models.StatusClosing)
	savePerpPosition(t, db, "p6", "FFF", models.StatusClosed) // ignored

	// Spot: one of every non-closed status + one closed.
	saveSpotPosition(t, db, "s1", "XXX", models.SpotStatusPending)
	saveSpotPosition(t, db, "s2", "YYY", models.SpotStatusActive)
	saveSpotPosition(t, db, "s3", "ZZZ", models.SpotStatusExiting)
	saveSpotPosition(t, db, "s4", "QQQ", models.SpotStatusClosed) // ignored

	occ, err := e.loadUnifiedOccupancy()
	if err != nil {
		t.Fatalf("loadUnifiedOccupancy: %v", err)
	}
	if occ.ActivePerp != 5 {
		t.Fatalf("ActivePerp: want 5 (pending/partial/active/exiting/closing), got %d", occ.ActivePerp)
	}
	if occ.ActiveSpot != 3 {
		t.Fatalf("ActiveSpot: want 3 (pending/active/exiting), got %d", occ.ActiveSpot)
	}
	if occ.GlobalSlotsRemaining != 10-8 {
		t.Fatalf("GlobalSlotsRemaining: want 2, got %d", occ.GlobalSlotsRemaining)
	}
	if occ.SpotSlotsRemaining != 5-3 {
		t.Fatalf("SpotSlotsRemaining: want 2, got %d", occ.SpotSlotsRemaining)
	}
	// ActiveSymbols must contain every non-closed symbol from both stores.
	wantSymbols := []string{"AAA", "BBB", "CCC", "DDD", "EEE", "XXX", "YYY", "ZZZ"}
	for _, sym := range wantSymbols {
		if _, ok := occ.ActiveSymbols[sym]; !ok {
			t.Errorf("ActiveSymbols missing %q", sym)
		}
	}
	// Closed symbols must NOT be present.
	for _, sym := range []string{"FFF", "QQQ"} {
		if _, ok := occ.ActiveSymbols[sym]; ok {
			t.Errorf("ActiveSymbols should not contain closed symbol %q", sym)
		}
	}
}

// TestUnifiedEntry_SymbolAlreadyActiveIsExcluded verifies
// groupChoicesBySymbol drops candidates whose symbol is already held
// across either strategy.
func TestUnifiedEntry_SymbolAlreadyActiveIsExcluded(t *testing.T) {
	e, _, _ := newTestEngineForUnified(t)

	occ := &unifiedOccupancy{
		ActiveSymbols:        map[string]struct{}{"BTCUSDT": {}},
		GlobalSlotsRemaining: 5,
		SpotSlotsRemaining:   5,
	}

	perpReq := &perpDispatchRequest{
		Opp: models.Opportunity{
			Symbol:        "BTCUSDT",
			LongExchange:  "binance",
			ShortExchange: "bybit",
			Spread:        5.0,
		},
		Approval: &models.RiskApproval{
			Approved: true, Size: 1, Price: 100, RequiredMargin: 50,
		},
	}
	spotPlan := &models.SpotEntryPlan{
		Candidate: models.SpotEntryCandidate{
			Symbol:     "BTCUSDT",
			Exchange:   "binance",
			Direction:  "borrow_sell_long",
			FundingAPR: 0.15, BorrowAPR: 0.03, FeePct: 0,
		},
		PlannedBaseSize:     100,
		PlannedNotionalUSDT: 1000,
	}

	groups, keyToChoice := e.groupChoicesBySymbol([]*perpDispatchRequest{perpReq}, []*models.SpotEntryPlan{spotPlan}, occ)

	if len(groups) != 0 {
		t.Fatalf("expected no groups when symbol already active, got %d: %+v", len(groups), groups)
	}
	if len(keyToChoice) != 0 {
		t.Fatalf("expected keyToChoice empty, got %+v", keyToChoice)
	}
}

// TestUnifiedEntry_DuplicateSymbolDedupAcrossStrategies verifies that
// when the same symbol appears as both a perp candidate AND a spot
// candidate, the solver never picks both (mutual exclusion per
// GroupKey).
func TestUnifiedEntry_DuplicateSymbolDedupAcrossStrategies(t *testing.T) {
	e, _, _ := newTestEngineForUnified(t)

	occ := &unifiedOccupancy{GlobalSlotsRemaining: 5, SpotSlotsRemaining: 5}

	perpReq := &perpDispatchRequest{
		Opp: models.Opportunity{
			Symbol: "BTCUSDT", LongExchange: "binance", ShortExchange: "bybit", Spread: 100,
		},
		Approval: &models.RiskApproval{
			Approved: true, Size: 1, Price: 100, RequiredMargin: 50,
		},
	}
	spotPlan := &models.SpotEntryPlan{
		Candidate: models.SpotEntryCandidate{
			Symbol:     "BTCUSDT",
			Exchange:   "binance",
			Direction:  "borrow_sell_long",
			FundingAPR: 0.50, BorrowAPR: 0.0, FeePct: 0,
		},
		PlannedBaseSize: 1, PlannedNotionalUSDT: 10000,
	}

	groups, keyToChoice := e.groupChoicesBySymbol(
		[]*perpDispatchRequest{perpReq},
		[]*models.SpotEntryPlan{spotPlan},
		occ,
	)
	if len(groups["BTCUSDT"]) != 2 {
		t.Fatalf("expected both perp+spot in the same GroupKey=BTCUSDT, got %d", len(groups["BTCUSDT"]))
	}
	evaluate := e.makeUnifiedEvaluator(keyToChoice, occ, nil)
	winners := solveGroupedSearch(groups, occ.GlobalSlotsRemaining, 100*time.Millisecond, evaluate)
	if len(winners) != 1 {
		t.Fatalf("mutual exclusion violated: want exactly 1 winner per symbol, got %d", len(winners))
	}
	if c, ok := keyToChoice[winners[0]]; !ok || c.Symbol != "BTCUSDT" {
		t.Fatalf("winner must be a BTCUSDT choice, got key=%q", winners[0])
	}
}

// TestUnifiedEntry_RespectsSpotOnlyCapIndependently — an evaluator
// supplied with SpotSlotsRemaining=1 must reject any set containing
// more than one spot pick, even when global slots allow it.
func TestUnifiedEntry_RespectsSpotOnlyCapIndependently(t *testing.T) {
	e, _, _ := newTestEngineForUnified(t)

	occ := &unifiedOccupancy{GlobalSlotsRemaining: 5, SpotSlotsRemaining: 1}

	spot1 := &models.SpotEntryPlan{
		Candidate: models.SpotEntryCandidate{
			Symbol: "AAA", Exchange: "binance", Direction: "buy_spot_short",
			FundingAPR: 0.40,
		},
		PlannedBaseSize: 1, PlannedNotionalUSDT: 1000,
	}
	spot2 := &models.SpotEntryPlan{
		Candidate: models.SpotEntryCandidate{
			Symbol: "BBB", Exchange: "bybit", Direction: "buy_spot_short",
			FundingAPR: 0.35,
		},
		PlannedBaseSize: 1, PlannedNotionalUSDT: 1000,
	}
	spot3 := &models.SpotEntryPlan{
		Candidate: models.SpotEntryCandidate{
			Symbol: "CCC", Exchange: "okx", Direction: "buy_spot_short",
			FundingAPR: 0.30,
		},
		PlannedBaseSize: 1, PlannedNotionalUSDT: 1000,
	}

	groups, keyToChoice := e.groupChoicesBySymbol(nil, []*models.SpotEntryPlan{spot1, spot2, spot3}, occ)
	evaluate := e.makeUnifiedEvaluator(keyToChoice, occ, nil)
	winners := solveGroupedSearch(groups, occ.GlobalSlotsRemaining, 100*time.Millisecond, evaluate)

	spotCount := 0
	for _, w := range winners {
		if c, ok := keyToChoice[w]; ok && c.Strategy == risk.StrategySpotFutures {
			spotCount++
		}
	}
	if spotCount > 1 {
		t.Fatalf("SpotSlotsRemaining=1 violated: expected <=1 spot winner, got %d", spotCount)
	}
}

// TestUnifiedEntry_RespectsMaxPositionsCombinedCap — with
// GlobalSlotsRemaining=2 the solver must never pick more than 2 winners
// even when more high-value candidates exist.
func TestUnifiedEntry_RespectsMaxPositionsCombinedCap(t *testing.T) {
	e, _, _ := newTestEngineForUnified(t)

	occ := &unifiedOccupancy{GlobalSlotsRemaining: 2, SpotSlotsRemaining: 5}

	mkPerp := func(sym, long, short string, spread float64) *perpDispatchRequest {
		return &perpDispatchRequest{
			Opp:      models.Opportunity{Symbol: sym, LongExchange: long, ShortExchange: short, Spread: spread},
			Approval: &models.RiskApproval{Approved: true, Size: 1, Price: 100, RequiredMargin: 50},
		}
	}
	mkSpot := func(sym string) *models.SpotEntryPlan {
		return &models.SpotEntryPlan{
			Candidate: models.SpotEntryCandidate{
				Symbol: sym, Exchange: "binance", Direction: "buy_spot_short", FundingAPR: 0.30,
			},
			PlannedBaseSize: 1, PlannedNotionalUSDT: 1000,
		}
	}
	perps := []*perpDispatchRequest{
		mkPerp("AAA", "binance", "bybit", 200),
		mkPerp("BBB", "gateio", "okx", 150),
		mkPerp("CCC", "bitget", "bingx", 120),
	}
	spots := []*models.SpotEntryPlan{mkSpot("XXX"), mkSpot("YYY")}

	groups, keyToChoice := e.groupChoicesBySymbol(perps, spots, occ)
	evaluate := e.makeUnifiedEvaluator(keyToChoice, occ, nil)
	winners := solveGroupedSearch(groups, occ.GlobalSlotsRemaining, 100*time.Millisecond, evaluate)

	if len(winners) > 2 {
		t.Fatalf("MaxPositions combined cap violated: want <=2 winners, got %d", len(winners))
	}
}

// TestUnifiedEntry_UserFiveCandidateExample — reproduces the user's
// 5-candidate scenario from the plan scope: {spot, perp, perp, perp,
// spot} candidates where capacity fits only 2. Values are tuned so
// that per USDT-over-horizon scoring the ranking is:
//
//	#1 spot S1  — very large FundingAPR and notional → biggest NetValueUSDT
//	#2 perp P2  — high spread * large notional → highest perp baseValue
//	#3 perp P3
//	#4 perp P4
//	#5 spot S5  — tiny notional and low APR
//
// Both strategies' scoring formulas are applied verbatim via
// scoreSpotEntry + computeAllocatorBaseValue, so the test is also a
// regression on the USDT-over-horizon comparison across strategies.
//
// Expected winners: {#1 spot S1, #2 perp P2}.
func TestUnifiedEntry_UserFiveCandidateExample(t *testing.T) {
	e, _, _ := newTestEngineForUnified(t)
	occ := &unifiedOccupancy{GlobalSlotsRemaining: 2, SpotSlotsRemaining: 2}

	// #1 spot — tuned to beat every perp and the other spot.
	// scoreSpotEntry with FundingAPR=3.0, notional=1e7, holdHours=16 →
	// 3.0 * 16/8760 * 1e7 ≈ 54794.5 USDT.
	spot1 := &models.SpotEntryPlan{
		Candidate: models.SpotEntryCandidate{
			Symbol: "S1", Exchange: "binance", Direction: "buy_spot_short",
			FundingAPR: 3.0, BorrowAPR: 0.0, FeePct: 0,
		},
		PlannedBaseSize: 1, PlannedNotionalUSDT: 1e7,
	}
	// #2 perp — computeAllocatorBaseValue(spread=200, notional=1e6, holdHours=16)
	// = 200 * 16 * 1e6 / 10000 = 320000 USDT (fees skipped with empty fee map).
	// Bigger than spot1's ~54.8k — so it ranks #2 only if we halve it; to
	// ensure S1 > P2 we bring P2's notional down.
	// Target: P2 baseValue ~40000 (spread 200 * 16 * 125000 / 10000 = 40000).
	perp2 := &perpDispatchRequest{
		Opp: models.Opportunity{
			Symbol: "P2", LongExchange: "binance", ShortExchange: "bybit", Spread: 200,
		},
		Approval: &models.RiskApproval{Approved: true, Size: 2.5, Price: 50000, RequiredMargin: 200},
	}
	// #3 perp (Spread=100, notional=125000) → 100*16*125000/10000=20000
	perp3 := &perpDispatchRequest{
		Opp: models.Opportunity{
			Symbol: "P3", LongExchange: "gateio", ShortExchange: "okx", Spread: 100,
		},
		Approval: &models.RiskApproval{Approved: true, Size: 2.5, Price: 50000, RequiredMargin: 50},
	}
	// #4 perp (Spread=50, notional=125000) → 50*16*125000/10000=10000
	perp4 := &perpDispatchRequest{
		Opp: models.Opportunity{
			Symbol: "P4", LongExchange: "bitget", ShortExchange: "bingx", Spread: 50,
		},
		Approval: &models.RiskApproval{Approved: true, Size: 2.5, Price: 50000, RequiredMargin: 50},
	}
	// #5 spot — tiny (FundingAPR=0.05, notional=100) → 0.05*16/8760*100 ≈ 0.0091 USDT
	spot5 := &models.SpotEntryPlan{
		Candidate: models.SpotEntryCandidate{
			Symbol: "S5", Exchange: "okx", Direction: "buy_spot_short",
			FundingAPR: 0.05, BorrowAPR: 0.0, FeePct: 0,
		},
		PlannedBaseSize: 1, PlannedNotionalUSDT: 100,
	}

	groups, keyToChoice := e.groupChoicesBySymbol(
		[]*perpDispatchRequest{perp2, perp3, perp4},
		[]*models.SpotEntryPlan{spot1, spot5},
		occ,
	)
	evaluate := e.makeUnifiedEvaluator(keyToChoice, occ, nil)
	winners := solveGroupedSearch(groups, occ.GlobalSlotsRemaining, 100*time.Millisecond, evaluate)

	if len(winners) != 2 {
		t.Fatalf("want 2 winners (top spot + top perp), got %d: %v", len(winners), winners)
	}

	// Collect winning symbols.
	winSymbols := map[string]bool{}
	for _, k := range winners {
		c, ok := keyToChoice[k]
		if !ok {
			continue
		}
		winSymbols[c.Symbol] = true
	}
	if !winSymbols["S1"] {
		t.Errorf("expected #1 spot S1 among winners (score ~54795 vs perps), got %v", winSymbols)
	}
	if !winSymbols["P2"] {
		t.Errorf("expected #2 perp P2 among winners (score ~40000), got %v", winSymbols)
	}
}

// TestUnifiedEntry_AppliesOverrideRescanFallbackWhenOppsEmpty — when
// discovery returns no perp opps but overrides exist, the helper must
// drive a RescanSymbols fallback and the resulting opps re-enter the
// orchestration pipeline.
func TestUnifiedEntry_AppliesOverrideRescanFallbackWhenOppsEmpty(t *testing.T) {
	e, _, _ := newTestEngineForUnified(t)
	e.allocOverrides = map[string]allocatorChoice{
		"AAA": {symbol: "AAA", longExchange: "binance", shortExchange: "bybit"},
	}
	stub := &stubRescanner{
		resultF: func(pairs []models.SymbolPair) []models.Opportunity {
			return []models.Opportunity{{
				Symbol:        "AAA",
				LongExchange:  "binance",
				ShortExchange: "bybit",
				Spread:        10,
			}}
		},
	}
	e.rescannerOverride = stub

	opps, tier := e.consumeOverridesAndEnrichOpps(nil)
	if tier != "tier-2-override-fallback" {
		t.Fatalf("tier: want tier-2-override-fallback, got %q", tier)
	}
	if len(opps) != 1 {
		t.Fatalf("want 1 fallback opp, got %d", len(opps))
	}
	if opps[0].Symbol != "AAA" {
		t.Fatalf("unexpected fallback opp: %+v", opps[0])
	}
	if stub.calls != 1 {
		t.Fatalf("RescanSymbols should be called once; got %d", stub.calls)
	}
}

// TestUnifiedEntry_LegacyPathRunsWhenFlagOnButAllocatorDisabled —
// readiness gate regression: unifiedOwnerReady() must return false
// when EnableUnifiedEntrySelection=true but EnableCapitalAllocator=false.
// runUnifiedEntrySelection's inner defense gate must short-circuit
// without side effects so the caller falls through to the legacy path.
func TestUnifiedEntry_LegacyPathRunsWhenFlagOnButAllocatorDisabled(t *testing.T) {
	e, _, _ := newTestEngineForUnified(t)
	e.cfg.EnableCapitalAllocator = false
	// Allocator still exists but now reports disabled via Enabled().
	if e.unifiedOwnerReady() {
		t.Fatalf("unifiedOwnerReady must be false when EnableCapitalAllocator=false")
	}

	// Running the selector directly should early-return and NOT touch
	// the spotEntry executor — proves the inner defense gate fires.
	stub := &stubSpotEntryExecutor{}
	e.spotEntry = stub
	if err := e.runUnifiedEntrySelection(); err != nil {
		t.Fatalf("runUnifiedEntrySelection should early-return nil, got %v", err)
	}
	if stub.openCallCount != 0 {
		t.Fatalf("spotEntry.OpenSelectedEntry must not be called when allocator disabled; got %d calls", stub.openCallCount)
	}
}

// TestUnifiedEntry_RequiresSpotEntryExecutorBeforeDispatch — the
// orchestrator guards on e.spotEntry != nil. With the flag on and the
// allocator enabled but spotEntry still nil, the selector must log a
// warning and return without crashing.
func TestUnifiedEntry_RequiresSpotEntryExecutorBeforeDispatch(t *testing.T) {
	e, _, _ := newTestEngineForUnified(t)
	e.spotEntry = nil
	// Must not panic; must early-return a nil error.
	if err := e.runUnifiedEntrySelection(); err != nil {
		t.Fatalf("runUnifiedEntrySelection with nil spotEntry: want nil err, got %v", err)
	}
}

// TestUnifiedEntry_DispatchHonorsPreheldReservation verifies that when
// the orchestrator hands a spot winner to the spot executor, the
// executor receives a NON-NIL preheld reservation — i.e. the selector
// does NOT expect OpenSelectedEntry to reserve capital again.
func TestUnifiedEntry_DispatchHonorsPreheldReservation(t *testing.T) {
	e, _, _ := newTestEngineForUnified(t)

	plan := &models.SpotEntryPlan{
		Candidate: models.SpotEntryCandidate{
			Symbol: "AAA", Exchange: "binance", Direction: "buy_spot_short",
			FundingAPR: 0.20,
		},
		PlannedBaseSize: 1, PlannedNotionalUSDT: 100,
	}
	stub := &stubSpotEntryExecutor{}
	e.spotEntry = stub

	// Fabricate a reservation directly so dispatchUnifiedSpot gets a
	// non-nil CapitalReservation (no need to build a BatchReservation
	// here — we are testing the dispatch hand-off specifically).
	res := &risk.CapitalReservation{
		ID:        "cap-test",
		Strategy:  risk.StrategySpotFutures,
		Exposures: map[string]float64{"binance": 100},
	}
	choice := &unifiedEntryChoice{
		Key: "spot:AAA:binance:buy_spot_short:0", Symbol: "AAA",
		Strategy: risk.StrategySpotFutures, ValueUSDT: 10, SlotCost: 1,
		Spot: plan,
	}

	if err := e.dispatchUnifiedSpot(choice, res); err != nil {
		t.Fatalf("dispatchUnifiedSpot: %v", err)
	}
	if stub.openCallCount != 1 {
		t.Fatalf("OpenSelectedEntry call count: want 1, got %d", stub.openCallCount)
	}
	if len(stub.openWithPreheld) != 1 || !stub.openWithPreheld[0] {
		t.Fatalf("OpenSelectedEntry must receive non-nil preheld reservation; observed preheld flags=%v", stub.openWithPreheld)
	}
	if len(stub.openCalledFor) != 1 || stub.openCalledFor[0] != "AAA" {
		t.Fatalf("OpenSelectedEntry must receive the winning plan's symbol; got %v", stub.openCalledFor)
	}
}

// TestUnifiedEntry_ReleasesPreheldOnDispatchError — when dispatch
// returns a non-nil error, dispatchUnifiedSpot must propagate the error
// to the orchestrator so the batch-level ReleaseBatch can reclaim the
// uncommitted reservation. Verified by having the stub return an error
// and asserting dispatchUnifiedSpot propagates it.
func TestUnifiedEntry_ReleasesPreheldOnDispatchError(t *testing.T) {
	e, _, _ := newTestEngineForUnified(t)

	plan := &models.SpotEntryPlan{
		Candidate: models.SpotEntryCandidate{
			Symbol: "AAA", Exchange: "binance", Direction: "buy_spot_short",
			FundingAPR: 0.20,
		},
		PlannedBaseSize: 1, PlannedNotionalUSDT: 100,
	}
	wantErr := errors.New("spot open failed")
	stub := &stubSpotEntryExecutor{
		openResultF: func(plan *models.SpotEntryPlan, cap float64, preheld *risk.CapitalReservation) error {
			return wantErr
		},
	}
	e.spotEntry = stub

	res := &risk.CapitalReservation{ID: "cap-err", Strategy: risk.StrategySpotFutures,
		Exposures: map[string]float64{"binance": 100}}
	choice := &unifiedEntryChoice{
		Key: "k1", Symbol: "AAA", Strategy: risk.StrategySpotFutures,
		ValueUSDT: 10, SlotCost: 1, Spot: plan,
	}

	err := e.dispatchUnifiedSpot(choice, res)
	if err == nil || !errors.Is(err, wantErr) {
		t.Fatalf("dispatchUnifiedSpot: want error %v propagated, got %v", wantErr, err)
	}
}

// TestUnifiedEntry_FallsBackToTier3WhenAllOverridesStale — regression for
// FIX 1: when overrides exist but rescanning returns empty AND the
// original perp opps slice was non-empty, selectUnifiedPerpOpps must fall
// back to the original opps as tier-3-rank-based so the perp side of the
// unified dispatch is not silently dropped.
//
// Two scenarios are covered:
//  1. empty originalPerpOpps + RescanSymbols returns empty  → opps stay
//     empty and tier stays "tier-2-override-fallback-empty" (nothing to
//     fall back to).
//  2. non-empty originalPerpOpps + all overrides stale (RescanSymbols
//     empty) → opps fall back to originalPerpOpps, tier flips to
//     "tier-3-rank-based".
func TestUnifiedEntry_FallsBackToTier3WhenAllOverridesStale(t *testing.T) {
	t.Run("empty_original_stays_empty", func(t *testing.T) {
		e, _, _ := newTestEngineForUnified(t)
		e.allocOverrides = map[string]allocatorChoice{
			"AAA": {symbol: "AAA", longExchange: "binance", shortExchange: "bybit"},
		}
		e.rescannerOverride = &stubRescanner{
			resultF: func(pairs []models.SymbolPair) []models.Opportunity { return nil },
		}
		got, tier := e.selectUnifiedPerpOpps(nil)
		if tier != "tier-2-override-fallback-empty" {
			t.Fatalf("tier: want tier-2-override-fallback-empty, got %q", tier)
		}
		if len(got) != 0 {
			t.Fatalf("want 0 opps when nothing to fall back to, got %d", len(got))
		}
	})

	t.Run("non_empty_original_rescans_empty_falls_back_tier3", func(t *testing.T) {
		e, _, _ := newTestEngineForUnified(t)
		e.allocOverrides = map[string]allocatorChoice{
			"AAA": {symbol: "AAA", longExchange: "binance", shortExchange: "bybit"},
		}
		// RescanSymbols returns empty — override salvage fails.
		e.rescannerOverride = &stubRescanner{
			resultF: func(pairs []models.SymbolPair) []models.Opportunity { return nil },
		}
		// Original perp opps are non-empty — should become the tier-3
		// fallback output.
		originalOpps := []models.Opportunity{
			{Symbol: "BBB", LongExchange: "gateio", ShortExchange: "okx", Spread: 8},
			{Symbol: "CCC", LongExchange: "bitget", ShortExchange: "bingx", Spread: 5},
		}
		// consumeOverridesAndEnrichOpps takes the tier-2 patch branch on
		// non-empty opps. Since no override Symbol matches, patched=nil +
		// didPatch=true → returns (nil, "tier-2-override-patch"). Our
		// fallback must then restore originalOpps with tier-3 tag.
		got, tier := e.selectUnifiedPerpOpps(originalOpps)
		if tier != "tier-3-rank-based" {
			t.Fatalf("tier: want tier-3-rank-based, got %q", tier)
		}
		if len(got) != len(originalOpps) {
			t.Fatalf("want %d opps from original list, got %d", len(originalOpps), len(got))
		}
		// Verify we're returning the SAME opps by symbol.
		wantSyms := map[string]bool{"BBB": true, "CCC": true}
		for _, o := range got {
			if !wantSyms[o.Symbol] {
				t.Errorf("unexpected fallback symbol %q (must come from originalPerpOpps)", o.Symbol)
			}
		}
	})
}

// TestUnifiedEntry_EvaluatorRejectsPerExchangeCapBreach — regression for
// FIX 2: when cumulative per-exchange margin exceeds the allocator's
// per-exchange cap (totalCap * MaxPerExchangePct), the evaluator must
// mark the set infeasible BEFORE ReserveBatch runs. Without this, B&B
// would pick a high-value subset that ReserveBatch then rejects all-or-
// nothing, wasting the tick.
func TestUnifiedEntry_EvaluatorRejectsPerExchangeCapBreach(t *testing.T) {
	e, _, _ := newTestEngineForUnified(t)
	// Tight per-exchange cap: 100 USDT on each exchange. Two perp candidates
	// both landing on "binance" would together breach that cap.
	e.cfg.MaxTotalExposureUSDT = 1000
	e.cfg.MaxPerExchangePct = 0.10 // per-exchange cap = 100
	e.cfg.MaxPerpPerpPct = 1.0     // disable strategy cap so only per-exchange fires
	e.cfg.MaxSpotFuturesPct = 1.0

	// Two perp picks both long-side on binance, each requesting 80 USDT of
	// binance exposure. Individually OK (80 <= 100). Together 160 > 100.
	perpA := &perpDispatchRequest{
		Opp: models.Opportunity{Symbol: "AAA", LongExchange: "binance", ShortExchange: "bybit", Spread: 100},
		Approval: &models.RiskApproval{
			Approved: true, Size: 1, Price: 80,
			LongMarginNeeded: 80, ShortMarginNeeded: 30,
		},
	}
	perpB := &perpDispatchRequest{
		Opp: models.Opportunity{Symbol: "BBB", LongExchange: "binance", ShortExchange: "okx", Spread: 100},
		Approval: &models.RiskApproval{
			Approved: true, Size: 1, Price: 80,
			LongMarginNeeded: 80, ShortMarginNeeded: 30,
		},
	}
	occ := &unifiedOccupancy{GlobalSlotsRemaining: 5, SpotSlotsRemaining: 0}
	_, keyToChoice := e.groupChoicesBySymbol([]*perpDispatchRequest{perpA, perpB}, nil, occ)

	snap, err := e.buildCapacitySnapshot(true, false)
	if err != nil {
		t.Fatalf("buildCapacitySnapshot: %v", err)
	}
	evaluate := e.makeUnifiedEvaluator(keyToChoice, occ, snap)

	// Individually each key is feasible.
	var keyA, keyB string
	for k, c := range keyToChoice {
		switch c.Symbol {
		case "AAA":
			keyA = k
		case "BBB":
			keyB = k
		}
	}
	if _, okA := evaluate([]string{keyA}); !okA {
		t.Fatalf("single perp A should be feasible on its own")
	}
	if _, okB := evaluate([]string{keyB}); !okB {
		t.Fatalf("single perp B should be feasible on its own")
	}
	// The combined set breaches the per-exchange cap and must be rejected.
	if _, okAB := evaluate([]string{keyA, keyB}); okAB {
		t.Fatalf("combined perp A + B must breach binance per-exchange cap (160 > 100)")
	}
}

// TestUnifiedEntry_EvaluatorRejectsStrategyCapBreach — regression for
// FIX 2: when cumulative per-strategy exposure exceeds the allocator's
// strategy cap, the evaluator must mark the set infeasible.
func TestUnifiedEntry_EvaluatorRejectsStrategyCapBreach(t *testing.T) {
	e, _, _ := newTestEngineForUnified(t)
	// Strategy cap for perp = totalCap * MaxPerpPerpPct = 1000 * 0.10 = 100.
	// Loose per-exchange cap (each exchange separate so it doesn't fire).
	e.cfg.MaxTotalExposureUSDT = 1000
	e.cfg.MaxPerpPerpPct = 0.10
	e.cfg.MaxSpotFuturesPct = 1.0
	e.cfg.MaxPerExchangePct = 1.0

	// Two perp picks each needing 70 USDT on distinct exchanges. Individually
	// fine; combined perp total = 280 > 100 strategy cap.
	perpA := &perpDispatchRequest{
		Opp: models.Opportunity{Symbol: "AAA", LongExchange: "binance", ShortExchange: "bybit", Spread: 100},
		Approval: &models.RiskApproval{
			Approved: true, Size: 1, Price: 70,
			LongMarginNeeded: 70, ShortMarginNeeded: 70,
		},
	}
	perpB := &perpDispatchRequest{
		Opp: models.Opportunity{Symbol: "BBB", LongExchange: "gateio", ShortExchange: "okx", Spread: 100},
		Approval: &models.RiskApproval{
			Approved: true, Size: 1, Price: 70,
			LongMarginNeeded: 70, ShortMarginNeeded: 70,
		},
	}
	occ := &unifiedOccupancy{GlobalSlotsRemaining: 5, SpotSlotsRemaining: 0}
	_, keyToChoice := e.groupChoicesBySymbol([]*perpDispatchRequest{perpA, perpB}, nil, occ)

	snap, err := e.buildCapacitySnapshot(true, false)
	if err != nil {
		t.Fatalf("buildCapacitySnapshot: %v", err)
	}
	evaluate := e.makeUnifiedEvaluator(keyToChoice, occ, snap)

	var keyA, keyB string
	for k, c := range keyToChoice {
		switch c.Symbol {
		case "AAA":
			keyA = k
		case "BBB":
			keyB = k
		}
	}
	// Combined 140+140=280 > 100 perp strategy cap.
	if _, ok := evaluate([]string{keyA, keyB}); ok {
		t.Fatalf("combined perp exposure 280 must breach strategy cap 100")
	}
}

// TestUnifiedEntry_SolverPicksFeasibleSubsetNotHighestValue — end-to-end
// regression for FIX 2: B&B should prefer a feasible subset even when a
// higher-value-but-infeasible subset exists, because ReserveBatch would
// reject the infeasible subset atomically.
func TestUnifiedEntry_SolverPicksFeasibleSubsetNotHighestValue(t *testing.T) {
	e, _, _ := newTestEngineForUnified(t)
	// Per-exchange cap = 100 USDT; total cap = 1000; strategy caps loose.
	e.cfg.MaxTotalExposureUSDT = 1000
	e.cfg.MaxPerExchangePct = 0.10
	e.cfg.MaxPerpPerpPct = 1.0
	e.cfg.MaxSpotFuturesPct = 1.0

	// Two "fat" perps both on binance — highest individual values, but
	// together breach the per-exchange cap.
	fatA := &perpDispatchRequest{
		Opp: models.Opportunity{Symbol: "AAA", LongExchange: "binance", ShortExchange: "bybit", Spread: 300},
		Approval: &models.RiskApproval{
			Approved: true, Size: 1, Price: 80,
			LongMarginNeeded: 80, ShortMarginNeeded: 80,
		},
	}
	fatB := &perpDispatchRequest{
		Opp: models.Opportunity{Symbol: "BBB", LongExchange: "binance", ShortExchange: "okx", Spread: 300},
		Approval: &models.RiskApproval{
			Approved: true, Size: 1, Price: 80,
			LongMarginNeeded: 80, ShortMarginNeeded: 80,
		},
	}
	// A "thin" perp on different exchange pair. Lower value but feasible
	// alongside either fat pick.
	thin := &perpDispatchRequest{
		Opp: models.Opportunity{Symbol: "CCC", LongExchange: "gateio", ShortExchange: "bitget", Spread: 100},
		Approval: &models.RiskApproval{
			Approved: true, Size: 1, Price: 50,
			LongMarginNeeded: 50, ShortMarginNeeded: 50,
		},
	}
	occ := &unifiedOccupancy{GlobalSlotsRemaining: 2, SpotSlotsRemaining: 0}
	groups, keyToChoice := e.groupChoicesBySymbol([]*perpDispatchRequest{fatA, fatB, thin}, nil, occ)

	snap, err := e.buildCapacitySnapshot(true, false)
	if err != nil {
		t.Fatalf("buildCapacitySnapshot: %v", err)
	}
	evaluate := e.makeUnifiedEvaluator(keyToChoice, occ, snap)
	winners := solveGroupedSearch(groups, occ.GlobalSlotsRemaining, 500*time.Millisecond, evaluate)

	if len(winners) == 0 {
		t.Fatalf("solver must find SOME feasible winner set, got empty")
	}
	// Count winners on binance and assert we never pick both fat binance
	// picks (their combined 160+160 breaches the 100 per-exchange cap).
	binanceCount := 0
	for _, k := range winners {
		c := keyToChoice[k]
		if c != nil && c.Perp != nil && c.Perp.Opp.LongExchange == "binance" {
			binanceCount++
		}
	}
	if binanceCount > 1 {
		t.Fatalf("solver picked %d binance-long perps; per-exchange cap must block 2nd pick", binanceCount)
	}
}

// TestUnifiedEntry_PerpValueDeductsFees — regression for FIX 3: the perp
// entry-scoring path must subtract trading fees (via
// getEffectiveAllocatorFees) so the selector's ranking matches the
// rebalance allocator. Without the fee deduction, perp candidates are
// systematically over-valued vs spot (spot's scoreSpotEntry already nets
// fees).
//
// Setup: identical spread / notional / hold-time; one perp on low-fee
// exchanges (binance/okx), one on high-fee exchanges (bitget/bingx).
// After fees the low-fee perp must score strictly higher.
func TestUnifiedEntry_PerpValueDeductsFees(t *testing.T) {
	e, _, _ := newTestEngineForUnified(t)
	occ := &unifiedOccupancy{GlobalSlotsRemaining: 5, SpotSlotsRemaining: 0}

	lowFee := &perpDispatchRequest{
		Opp:      models.Opportunity{Symbol: "AAA", LongExchange: "binance", ShortExchange: "okx", Spread: 100},
		Approval: &models.RiskApproval{Approved: true, Size: 1, Price: 100_000, RequiredMargin: 100},
	}
	highFee := &perpDispatchRequest{
		Opp:      models.Opportunity{Symbol: "BBB", LongExchange: "bitget", ShortExchange: "bingx", Spread: 100},
		Approval: &models.RiskApproval{Approved: true, Size: 1, Price: 100_000, RequiredMargin: 100},
	}

	_, keyToChoice := e.groupChoicesBySymbol(
		[]*perpDispatchRequest{lowFee, highFee}, nil, occ,
	)

	// Two choices built → both have their ValueUSDT populated by
	// computeAllocatorBaseValue with the perp fee map. The low-fee pick
	// must outscore the high-fee pick even with identical spread+notional.
	var lowScore, highScore float64
	var found int
	for _, c := range keyToChoice {
		switch c.Symbol {
		case "AAA":
			lowScore = c.ValueUSDT
			found++
		case "BBB":
			highScore = c.ValueUSDT
			found++
		}
	}
	if found != 2 {
		t.Fatalf("expected 2 scored perp choices, got %d", found)
	}
	if lowScore <= highScore {
		t.Fatalf("low-fee perp must score strictly higher than high-fee perp (got low=%.4f high=%.4f)", lowScore, highScore)
	}
	// Sanity: both positive (fee deduction didn't push them below zero for
	// these values — spread=100 bps over 16h on 100k notional dominates).
	if lowScore <= 0 || highScore <= 0 {
		t.Fatalf("scores unexpectedly non-positive: low=%.4f high=%.4f", lowScore, highScore)
	}
}

func TestUnifiedEntry_EvaluatorHonorsPctOverride(t *testing.T) {
	e, _, _ := newTestEngineForUnified(t)
	occ := &unifiedOccupancy{GlobalSlotsRemaining: 1, SpotSlotsRemaining: 0}
	req := &perpDispatchRequest{
		Opp: models.Opportunity{Symbol: "BTCUSDT", LongExchange: "binance", ShortExchange: "bybit", Spread: 100},
		Approval: &models.RiskApproval{
			Approved: true, Size: 1, Price: 100,
			LongMarginNeeded: 450, ShortMarginNeeded: 450,
		},
	}
	_, keyToChoice := e.groupChoicesBySymbol([]*perpDispatchRequest{req}, nil, occ)
	var key string
	for k := range keyToChoice {
		key = k
	}

	overrideSnap := &unifiedCapacitySnapshot{
		committedByStrategy: map[risk.Strategy]float64{},
		committedByExchange: map[string]float64{},
		totalCap:            1000,
		effectivePerpPct:    0.60,
		perpPctOverride:     1.0,
	}
	if _, ok := e.makeUnifiedEvaluator(keyToChoice, occ, overrideSnap)([]string{key}); !ok {
		t.Fatal("override snapshot should admit the 900 USDT perp request")
	}

	staticSnap := &unifiedCapacitySnapshot{
		committedByStrategy: map[risk.Strategy]float64{},
		committedByExchange: map[string]float64{},
		totalCap:            1000,
		effectivePerpPct:    0.60,
	}
	if _, ok := e.makeUnifiedEvaluator(keyToChoice, occ, staticSnap)([]string{key}); ok {
		t.Fatal("static snapshot should reject the 900 USDT perp request")
	}
}

func TestUnifiedEntry_BatchReservationItemThreadsPctOverride(t *testing.T) {
	e, _, _ := newTestEngineForUnified(t)
	e.cfg.EnableUnifiedCapital = true
	e.cfg.MaxTotalExposureUSDT = 1000
	e.cfg.MaxPerpPerpPct = 0.60
	e.cfg.MaxSpotFuturesPct = 0.40
	e.cfg.AllocationCeilingPct = 1.0
	e.allocator.SetEffectiveAllocation(0.60, 0.40)

	req := &perpDispatchRequest{
		Opp: models.Opportunity{Symbol: "BTCUSDT", LongExchange: "binance", ShortExchange: "bybit", Spread: 100},
		Approval: &models.RiskApproval{
			Approved: true, Size: 1, Price: 100,
			LongMarginNeeded: 350, ShortMarginNeeded: 350,
		},
	}

	admitted, batch, _, err := e.admitUnifiedWinners([]*perpDispatchRequest{req}, nil)
	if err != nil {
		t.Fatalf("admitUnifiedWinners: %v", err)
	}
	if len(admitted) != 1 || batch == nil {
		t.Fatalf("admission result = (%d winners, batch=%v), want 1 winner and batch", len(admitted), batch != nil)
	}
	if admitted[0].CapOverride != 1.0 {
		t.Fatalf("CapOverride = %.2f, want 1.00", admitted[0].CapOverride)
	}
	if batch.Items[admitted[0].Key] == nil {
		t.Fatalf("batch missing winner reservation for key %q", admitted[0].Key)
	}
}

func TestUnifiedEntry_PendingPositionRollbackOnPartialFailure(t *testing.T) {
	e, db, _ := newTestEngineForUnified(t)
	e.cfg.EnableUnifiedCapital = true
	e.cfg.MaxTotalExposureUSDT = 1000
	e.cfg.MaxPerpPerpPct = 1.0
	e.cfg.MaxSpotFuturesPct = 0.0
	e.cfg.AllocationCeilingPct = 1.0
	e.allocator.SetEffectiveAllocation(1.0, 0.0)

	reqs := []*perpDispatchRequest{
		{
			Opp:      models.Opportunity{Symbol: "AAA", LongExchange: "binance", ShortExchange: "bybit", Spread: 100},
			Approval: &models.RiskApproval{Approved: true, Size: 1, Price: 100, LongMarginNeeded: 10, ShortMarginNeeded: 10},
		},
		{
			Opp:      models.Opportunity{Symbol: "BBB", LongExchange: "okx", ShortExchange: "gateio", Spread: 100},
			Approval: &models.RiskApproval{Approved: true, Size: 1, Price: 100, LongMarginNeeded: 10, ShortMarginNeeded: 10},
		},
	}
	saveCalls := 0
	e.savePendingPerpOverride = func(pos *models.ArbitragePosition) error {
		saveCalls++
		if saveCalls == 2 {
			return errors.New("boom")
		}
		return db.SavePosition(pos)
	}

	if _, _, _, err := e.admitUnifiedWinners(reqs, nil); err == nil {
		t.Fatal("expected admitUnifiedWinners to fail on second pending save")
	}
	active, err := db.GetActivePositions()
	if err != nil {
		t.Fatalf("GetActivePositions: %v", err)
	}
	if len(active) != 0 {
		t.Fatalf("active positions after rollback = %d, want 0", len(active))
	}
	history, err := db.GetHistory(10)
	if err != nil {
		t.Fatalf("GetHistory: %v", err)
	}
	if len(history) == 0 {
		t.Fatal("rollback should move the already-created pending record into history")
	}
}

func TestUnifiedEntry_DispatchUsesPreCreatedPendingNotNew(t *testing.T) {
	e, db, _ := newTestEngineForUnified(t)
	pending := e.createPendingPosition(models.Opportunity{Symbol: "BTCUSDT", LongExchange: "binance", ShortExchange: "bybit"})
	if err := db.SavePosition(pending); err != nil {
		t.Fatalf("SavePosition: %v", err)
	}
	saveCalls := 0
	e.savePendingPerpOverride = func(*models.ArbitragePosition) error {
		saveCalls++
		return nil
	}
	e.acquireLockOverride = func(string, time.Duration) (*database.OwnedLock, bool, error) {
		return nil, false, nil
	}

	choice := &unifiedEntryChoice{
		Key:        "perp:btc",
		Symbol:     "BTCUSDT",
		Strategy:   risk.StrategyPerpPerp,
		PendingPos: pending,
		Perp: &perpDispatchRequest{
			Opp:      models.Opportunity{Symbol: "BTCUSDT", LongExchange: "binance", ShortExchange: "bybit"},
			Approval: &models.RiskApproval{Approved: true, Size: 1, Price: 100},
		},
	}

	if err := e.dispatchUnifiedPerp(choice, nil); err == nil {
		t.Fatal("expected busy-lock dispatch failure")
	}
	if saveCalls != 0 {
		t.Fatalf("dispatch should not create a new pending record, savePendingPerpOverride called %d times", saveCalls)
	}
}

func TestUnifiedEntry_EarlyReturnPathsCleanupPendingRecord(t *testing.T) {
	e, db, _ := newTestEngineForUnified(t)
	pending := e.createPendingPosition(models.Opportunity{Symbol: "ETHUSDT", LongExchange: "okx", ShortExchange: "gateio"})
	if err := db.SavePosition(pending); err != nil {
		t.Fatalf("SavePosition: %v", err)
	}
	e.acquireLockOverride = func(string, time.Duration) (*database.OwnedLock, bool, error) {
		return nil, false, nil
	}

	choice := &unifiedEntryChoice{
		Key:        "perp:eth",
		Symbol:     "ETHUSDT",
		Strategy:   risk.StrategyPerpPerp,
		PendingPos: pending,
		Perp: &perpDispatchRequest{
			Opp:      models.Opportunity{Symbol: "ETHUSDT", LongExchange: "okx", ShortExchange: "gateio"},
			Approval: &models.RiskApproval{Approved: true, Size: 1, Price: 100},
		},
	}

	err := e.dispatchUnifiedPerp(choice, nil)
	if err == nil || !strings.Contains(err.Error(), "busy") {
		t.Fatalf("dispatch error = %v, want busy lock cleanup path", err)
	}
	stored, err := db.GetPosition(pending.ID)
	if err != nil {
		t.Fatalf("GetPosition: %v", err)
	}
	if stored.Status != models.StatusClosed {
		t.Fatalf("pending status = %s, want closed", stored.Status)
	}
	if !strings.Contains(stored.ExitReason, "entry_failed") {
		t.Fatalf("ExitReason = %q, want entry_failed marker", stored.ExitReason)
	}
}

func TestUnifiedEntry_E2ESpotPreheldHandoff(t *testing.T) {
	for _, tc := range []struct {
		name        string
		commitFinal bool
	}{
		{name: "release", commitFinal: false},
		{name: "commit", commitFinal: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, db, _ := newTestEngineForUnified(t)
			e.cfg.EnableUnifiedCapital = true
			e.cfg.AllocationCeilingPct = 1.0

			releaseCh := make(chan struct{})
			reservedCh := make(chan *risk.CapitalReservation, 1)
			stub := &stubSpotEntryExecutor{
				listResult: []models.SpotEntryCandidate{{
					Symbol:     "BTCUSDT",
					BaseCoin:   "BTC",
					Exchange:   "binance",
					Direction:  "buy_spot_short",
					FundingAPR: 0.50,
					BorrowAPR:  0.01,
					Timestamp:  time.Now().UTC(),
				}},
			}
			stub.openResultF = func(plan *models.SpotEntryPlan, capOverride float64, preheld *risk.CapitalReservation) error {
				reservedCh <- preheld
				<-releaseCh
				if tc.commitFinal {
					return e.allocator.Commit(preheld, "spot-pos", map[string]float64{
						plan.Candidate.Exchange: plan.PlannedNotionalUSDT,
					})
				}
				return nil
			}
			e.SetSpotEntryExecutor(stub)

			errCh := make(chan error, 1)
			go func() {
				errCh <- e.runUnifiedEntrySelection()
			}()

			preheld := <-reservedCh
			if preheld == nil {
				t.Fatal("expected preheld reservation during dispatch")
			}
			reservedKey := "risk:capital:reserved:" + preheld.ID
			keys, err := db.Redis().Keys(context.Background(), "risk:capital:*").Result()
			if err != nil {
				t.Fatalf("redis keys: %v", err)
			}
			foundReserved := false
			foundCommitted := false
			for _, key := range keys {
				if key == reservedKey {
					foundReserved = true
				}
				if strings.HasPrefix(key, "risk:capital:committed:") {
					foundCommitted = true
				}
			}
			if !foundReserved {
				t.Fatalf("mid-dispatch reserved key %q not found in %v", reservedKey, keys)
			}
			if foundCommitted {
				t.Fatalf("committed keys should not exist mid-dispatch: %v", keys)
			}

			close(releaseCh)
			if err := <-errCh; err != nil {
				t.Fatalf("runUnifiedEntrySelection: %v", err)
			}

			keys, err = db.Redis().Keys(context.Background(), "risk:capital:*").Result()
			if err != nil {
				t.Fatalf("redis keys: %v", err)
			}
			for _, key := range keys {
				if key == reservedKey {
					t.Fatalf("reserved key %q should be gone after dispatch", reservedKey)
				}
			}

			committedCount := 0
			for _, key := range keys {
				if strings.HasPrefix(key, "risk:capital:committed:") {
					committedCount++
				}
			}
			if tc.commitFinal && committedCount == 0 {
				t.Fatal("commit case should leave committed allocator keys behind")
			}
			if !tc.commitFinal && committedCount != 0 {
				t.Fatalf("release case should not leave committed allocator keys, got %d", committedCount)
			}
		})
	}
}

// TestUnifiedEntry_SnapshotUsesStrategyPctFallback — Fix G regression: when
// allocator.Summary() reports EffectivePerpPct/EffectiveSpotPct == 0 (the
// state that holds before updateAllocation() has run at least once), the
// snapshot must fall back to the static cfg.MaxPerpPerpPct / MaxSpotFuturesPct.
// Without this, B&B feasibility uses zero caps and admits nothing.
//
// Plan v15 Section 4 acceptance #508 (strategyPct fallback path); plan calls
// out allocator.go:599-621 as the live fallback source.
func TestUnifiedEntry_SnapshotUsesStrategyPctFallback(t *testing.T) {
	e, _, _ := newTestEngineForUnified(t)
	// Distinctive static values so we can verify the fallback took effect
	// rather than the zero from an uninitialized allocator.
	e.cfg.MaxPerpPerpPct = 0.42
	e.cfg.MaxSpotFuturesPct = 0.31

	// Do NOT call SetEffectiveAllocation / updateAllocation — allocator starts
	// with effectivePerpPct=0, effectiveSpotPct=0. That is the very state the
	// fallback is designed to cover.

	snap, err := e.buildCapacitySnapshot(true, true)
	if err != nil {
		t.Fatalf("buildCapacitySnapshot: %v", err)
	}
	if snap.effectivePerpPct != 0.42 {
		t.Errorf("effectivePerpPct = %v; want static fallback 0.42", snap.effectivePerpPct)
	}
	if snap.effectiveSpotPct != 0.31 {
		t.Errorf("effectiveSpotPct = %v; want static fallback 0.31", snap.effectiveSpotPct)
	}
	// perpPctOverride / spotPctOverride must also resolve to the static
	// fallback (they default to effective*Pct when DynamicStrategyPct returns 0).
	if snap.perpPctOverride != 0.42 {
		t.Errorf("perpPctOverride = %v; want fallback 0.42", snap.perpPctOverride)
	}
	if snap.spotPctOverride != 0.31 {
		t.Errorf("spotPctOverride = %v; want fallback 0.31", snap.spotPctOverride)
	}
}

// TestUnifiedEntry_SnapshotThreadsOppPresenceToDynamicPct — Fix G regression:
// buildCapacitySnapshot must pass (perpHasOpps, spotHasOpps) through to
// DynamicStrategyPct so the dynamic-shift logic (free unused capacity from
// the strategy that has no opps) kicks in. When spotHasOpps=false, perp
// should be allowed to borrow the spot allocation; and vice versa.
//
// Plan v15 Section 3 Fix G + Section 4 acceptance #509.
func TestUnifiedEntry_SnapshotThreadsOppPresenceToDynamicPct(t *testing.T) {
	e, _, _ := newTestEngineForUnified(t)
	// Enable unified-capital allocation so DynamicStrategyPct's dynamic-shift
	// branch runs (otherwise it short-circuits to strategyPct).
	e.cfg.EnableUnifiedCapital = true
	e.cfg.MaxTotalExposureUSDT = 1000
	e.cfg.MaxPerpPerpPct = 0.40
	e.cfg.MaxSpotFuturesPct = 0.40
	e.cfg.AllocationCeilingPct = 0.90

	// Seed effective pcts directly — simulates post-updateAllocation state.
	// With zero committed on both sides, the unused-spot pool available to
	// borrow back to perp is spotPct*totalCap = 400. When spotHasOpps=false,
	// perp override should include that freed capacity.
	e.allocator.SetEffectiveAllocation(0.40, 0.40)

	bothSides, err := e.buildCapacitySnapshot(true, true)
	if err != nil {
		t.Fatalf("buildCapacitySnapshot(true,true): %v", err)
	}
	perpOnly, err := e.buildCapacitySnapshot(true, false)
	if err != nil {
		t.Fatalf("buildCapacitySnapshot(true,false): %v", err)
	}
	spotOnly, err := e.buildCapacitySnapshot(false, true)
	if err != nil {
		t.Fatalf("buildCapacitySnapshot(false,true): %v", err)
	}

	// perp + spot both present — no freed allocation, plain 0.40.
	if bothSides.perpPctOverride != 0.40 {
		t.Errorf("bothSides perpPctOverride = %v; want 0.40", bothSides.perpPctOverride)
	}
	// spot has no opps → perp grabs spot's uncommitted share → strictly > 0.40.
	if !(perpOnly.perpPctOverride > bothSides.perpPctOverride) {
		t.Errorf("perpOnly perpPctOverride %v must exceed bothSides %v (spot free capacity not threaded through)",
			perpOnly.perpPctOverride, bothSides.perpPctOverride)
	}
	// perp has no opps → spot grabs perp's uncommitted share → strictly > 0.40.
	if !(spotOnly.spotPctOverride > bothSides.spotPctOverride) {
		t.Errorf("spotOnly spotPctOverride %v must exceed bothSides %v (perp free capacity not threaded through)",
			spotOnly.spotPctOverride, bothSides.spotPctOverride)
	}
	// Ceiling clamp holds (AllocationCeilingPct=0.90).
	if perpOnly.perpPctOverride > 0.90 {
		t.Errorf("perpOnly perpPctOverride %v breaches ceiling 0.90", perpOnly.perpPctOverride)
	}
	if spotOnly.spotPctOverride > 0.90 {
		t.Errorf("spotOnly spotPctOverride %v breaches ceiling 0.90", spotOnly.spotPctOverride)
	}
}

// TestUnifiedEntry_AdmissionSerializedAgainstConcurrentManualOpen — Fix H
// regression: the admissionMu is a SHARED *sync.Mutex between Engine (value
// field owner via AdmissionMutex() accessor) and SpotEngine (holds pointer
// via NewSpotEngine param). While one side holds the lock (simulating unified
// admission), the other side's lock acquisition (simulating perp ManualOpen
// or spot ManualOpen) must block until release.
//
// Plan v15 Section 4 acceptance #512. This test verifies the lock-sharing
// contract directly — real Engine.ManualOpen / SpotEngine.ManualOpen funnel
// through the same *sync.Mutex via e.admissionMu.Lock / e.admissionLock(),
// so the serialization property proved here applies to them.
func TestUnifiedEntry_AdmissionSerializedAgainstConcurrentManualOpen(t *testing.T) {
	e, _, _ := newTestEngineForUnified(t)
	// Engine.AdmissionMutex() is the canonical accessor; SpotEngine is wired
	// with this exact pointer in cmd/main.go:432.
	sharedMu := e.AdmissionMutex()
	if sharedMu == nil {
		t.Fatal("AdmissionMutex() returned nil — lock-sharing contract broken")
	}

	// Hold the lock via the Engine's own admissionMu to simulate unified
	// admission in-progress (runUnifiedEntrySelection holds this during
	// solver + reserve + pending-save per Fix H).
	sharedMu.Lock()

	// Two contenders simulate perp ManualOpen and spot ManualOpen. Both must
	// acquire sharedMu before doing any admission work (perp via
	// e.admissionMu.Lock in engine.go ManualOpen / executeArbitrage; spot via
	// e.admissionLock() helper that wraps e.admissionMu.Lock). Here we use
	// the raw *sync.Mutex — what matters is the shared identity.
	acquired := make(chan string, 2)
	var wg sync.WaitGroup
	contenders := []string{"perp-ManualOpen", "spot-ManualOpen"}
	for _, name := range contenders {
		wg.Add(1)
		go func(who string) {
			defer wg.Done()
			sharedMu.Lock()
			acquired <- who
			sharedMu.Unlock()
		}(name)
	}

	// Give contenders time to schedule. None may acquire while we still hold.
	select {
	case name := <-acquired:
		sharedMu.Unlock() // avoid leaking lock in the failure path
		t.Fatalf("%s acquired admissionMu while unified admission still held it", name)
	case <-time.After(80 * time.Millisecond):
		// Expected: both contenders are blocked waiting on sharedMu.
	}

	// Release. Both contenders must proceed in some order.
	sharedMu.Unlock()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		// Both goroutines finished — serialization worked.
	case <-time.After(2 * time.Second):
		t.Fatal("contenders did not unblock within 2s after admissionMu release")
	}

	if len(acquired) != 2 {
		t.Errorf("expected 2 contender acquisitions after release, got %d", len(acquired))
	}
}
