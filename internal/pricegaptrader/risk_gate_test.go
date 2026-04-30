package pricegaptrader

import (
	"errors"
	"testing"
	"time"

	"arb/internal/config"
	"arb/internal/models"
	"arb/pkg/exchange"
)

// ---- Fakes ------------------------------------------------------------------

// fakeStore satisfies models.PriceGapStore — only methods used by preEntry get
// real bodies. All others return zero values.
type fakeStore struct {
	disabled map[string]string // symbol -> reason ("" means not disabled; presence => disabled)
	active   []*models.PriceGapPosition

	// Plan 05: record saved positions + simulate per-resource lock ownership.
	saved        []*models.PriceGapPosition
	heldLocks    map[string]string // resource -> token (presence means held)
	lockCounter  int
	forceLockBusy bool // tests set true to simulate contention
	// PG-DIR-01: per-call lock-resource history (FIFO append on every
	// AcquirePriceGapLock invocation, regardless of held/forceLockBusy outcome).
	// Used by TestOpenPair_LockKeyUnchangedByDirection to assert forward and
	// inverse fires acquire the SAME lock key (configured tuple).
	lockHistory []string

	// Plan 06: rolling slippage window + disabled-flag write recorder.
	slipWindow      map[string][]models.SlippageSample // candidateID -> samples
	disabledSetWith map[string]string                  // symbol -> reason passed to SetCandidateDisabled
	history         []*models.PriceGapPosition         // AddPriceGapHistory recorder
	removedActive   []string                           // RemoveActivePriceGapPosition recorder
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		disabled:        map[string]string{},
		heldLocks:       map[string]string{},
		slipWindow:      map[string][]models.SlippageSample{},
		disabledSetWith: map[string]string{},
	}
}

func (f *fakeStore) IsCandidateDisabled(symbol string) (bool, string, int64, error) {
	r, ok := f.disabled[symbol]
	return ok, r, 0, nil
}
func (f *fakeStore) GetActivePriceGapPositions() ([]*models.PriceGapPosition, error) {
	return f.active, nil
}

// ---- zero-value PriceGapStore method stubs ---------------------------------
func (f *fakeStore) SavePriceGapPosition(p *models.PriceGapPosition) error {
	f.saved = append(f.saved, p)
	return nil
}
func (f *fakeStore) GetPriceGapPosition(id string) (*models.PriceGapPosition, error) {
	return nil, nil
}
func (f *fakeStore) AddPriceGapHistory(p *models.PriceGapPosition) error {
	f.history = append(f.history, p)
	return nil
}
func (f *fakeStore) RemoveActivePriceGapPosition(id string) error {
	f.removedActive = append(f.removedActive, id)
	return nil
}
func (f *fakeStore) SetCandidateDisabled(symbol, reason string) error {
	f.disabledSetWith[symbol] = reason
	f.disabled[symbol] = reason
	return nil
}
func (f *fakeStore) ClearCandidateDisabled(symbol string) error {
	delete(f.disabled, symbol)
	delete(f.disabledSetWith, symbol)
	return nil
}
func (f *fakeStore) AppendSlippageSample(candidateID string, sample models.SlippageSample) error {
	f.slipWindow[candidateID] = append(f.slipWindow[candidateID], sample)
	return nil
}
// GetSlippageWindow returns up to the most recent n samples, mimicking Redis LTRIM semantics.
func (f *fakeStore) GetSlippageWindow(candidateID string, n int) ([]models.SlippageSample, error) {
	all := f.slipWindow[candidateID]
	if n <= 0 || len(all) <= n {
		out := make([]models.SlippageSample, len(all))
		copy(out, all)
		return out, nil
	}
	out := make([]models.SlippageSample, n)
	copy(out, all[len(all)-n:])
	return out, nil
}

// AcquirePriceGapLock simulates a mutex: returns (token, true, nil) on free,
// (_, false, nil) on already-held or when forceLockBusy is set.
func (f *fakeStore) AcquirePriceGapLock(resource string, ttl time.Duration) (string, bool, error) {
	f.lockHistory = append(f.lockHistory, resource)
	if f.forceLockBusy {
		return "", false, nil
	}
	if _, held := f.heldLocks[resource]; held {
		return "", false, nil
	}
	f.lockCounter++
	tok := "tok-" + resource
	f.heldLocks[resource] = tok
	return tok, true, nil
}

func (f *fakeStore) ReleasePriceGapLock(resource, token string) error {
	if cur, ok := f.heldLocks[resource]; ok && cur == token {
		delete(f.heldLocks, resource)
	}
	return nil
}

// fakeDelistChecker satisfies models.DelistChecker.
type fakeDelistChecker struct {
	delisted map[string]bool
}

func newFakeDelistChecker() *fakeDelistChecker {
	return &fakeDelistChecker{delisted: map[string]bool{}}
}

func (f *fakeDelistChecker) IsDelisted(symbol string) bool {
	return f.delisted[symbol]
}

// Compile-time guarantees.
var _ models.PriceGapStore = (*fakeStore)(nil)
var _ models.DelistChecker = (*fakeDelistChecker)(nil)

// ---- Helpers ----------------------------------------------------------------

func defaultGateConfig() *config.Config {
	return &config.Config{
		PriceGapEnabled:              true,
		PriceGapBudget:               5000,
		PriceGapMaxConcurrent:        3,
		PriceGapGateConcentrationPct: 0.5,
		PriceGapKlineStalenessSec:    90,
	}
}

func newGateTestTracker(t *testing.T, store *fakeStore, delist *fakeDelistChecker, cfg *config.Config) *Tracker {
	t.Helper()
	exch := map[string]exchange.Exchange{
		"binance": newStubExchange("binance"),
		"gate":    newStubExchange("gate"),
		"bybit":   newStubExchange("bybit"),
		"bitget":  newStubExchange("bitget"),
	}
	return NewTracker(exch, store, delist, cfg)
}

func binanceBybitCand() models.PriceGapCandidate {
	return models.PriceGapCandidate{
		Symbol:             "SOON",
		LongExch:           "binance",
		ShortExch:          "bybit",
		ThresholdBps:       200,
		MaxPositionUSDT:    5000,
		ModeledSlippageBps: 47.9,
	}
}

func binanceGateCand() models.PriceGapCandidate {
	return models.PriceGapCandidate{
		Symbol:             "SOON",
		LongExch:           "binance",
		ShortExch:          "gate",
		ThresholdBps:       200,
		MaxPositionUSDT:    5000,
		ModeledSlippageBps: 47.9,
	}
}

func gateBinanceActivePos(notional float64) *models.PriceGapPosition {
	return &models.PriceGapPosition{
		ID:            "X",
		Symbol:        "ABC",
		LongExchange:  "gate",
		ShortExchange: "binance",
		NotionalUSDT:  notional,
		Status:        models.PriceGapStatusOpen,
	}
}

func binanceBybitActivePos(notional float64) *models.PriceGapPosition {
	return &models.PriceGapPosition{
		ID:            "Y",
		Symbol:        "ABC",
		LongExchange:  "binance",
		ShortExchange: "bybit",
		NotionalUSDT:  notional,
		Status:        models.PriceGapStatusOpen,
	}
}

func freshDetection() DetectionResult {
	return DetectionResult{Fired: true, SpreadBps: 250, StalenessSec: 30}
}

// ---- Tests ------------------------------------------------------------------

func TestRiskGate_DisabledCandidate_BlocksFirst(t *testing.T) {
	store := newFakeStore()
	store.disabled["SOON"] = "exec_quality:breach"
	tr := newGateTestTracker(t, store, newFakeDelistChecker(), defaultGateConfig())

	res := tr.preEntry(binanceBybitCand(), 1000, freshDetection(), nil)
	if res.Approved {
		t.Fatalf("disabled candidate must be blocked")
	}
	if !errors.Is(res.Err, ErrPriceGapCandidateDisabled) {
		t.Fatalf("err=%v, want ErrPriceGapCandidateDisabled", res.Err)
	}
}

func TestRiskGate_MaxConcurrent_AtLimit(t *testing.T) {
	store := newFakeStore()
	tr := newGateTestTracker(t, store, newFakeDelistChecker(), defaultGateConfig())
	active := []*models.PriceGapPosition{
		binanceBybitActivePos(100),
		binanceBybitActivePos(100),
		binanceBybitActivePos(100),
	}
	res := tr.preEntry(binanceBybitCand(), 500, freshDetection(), active)
	if res.Approved {
		t.Fatalf("3 active + MaxConcurrent=3 must block")
	}
	if !errors.Is(res.Err, ErrPriceGapMaxConcurrent) {
		t.Fatalf("err=%v, want ErrPriceGapMaxConcurrent", res.Err)
	}
}

func TestRiskGate_MaxConcurrent_BelowLimit(t *testing.T) {
	store := newFakeStore()
	tr := newGateTestTracker(t, store, newFakeDelistChecker(), defaultGateConfig())
	active := []*models.PriceGapPosition{
		binanceBybitActivePos(100),
		binanceBybitActivePos(100),
	}
	res := tr.preEntry(binanceBybitCand(), 500, freshDetection(), active)
	if !res.Approved {
		t.Fatalf("2 active + MaxConcurrent=3 must pass this gate; err=%v reason=%s", res.Err, res.Reason)
	}
}

func TestRiskGate_PerPositionCap_Boundary(t *testing.T) {
	// requested == MaxPositionUSDT => PASS (uses strict >, not ≥)
	store := newFakeStore()
	tr := newGateTestTracker(t, store, newFakeDelistChecker(), defaultGateConfig())
	res := tr.preEntry(binanceBybitCand(), 5000, freshDetection(), nil)
	if !res.Approved {
		t.Fatalf("requested==MaxPositionUSDT boundary must pass; err=%v reason=%s", res.Err, res.Reason)
	}
}

func TestRiskGate_PerPositionCap_Over(t *testing.T) {
	store := newFakeStore()
	tr := newGateTestTracker(t, store, newFakeDelistChecker(), defaultGateConfig())
	res := tr.preEntry(binanceBybitCand(), 5001, freshDetection(), nil)
	if res.Approved {
		t.Fatalf("requested=5001 > cap=5000 must block")
	}
	if !errors.Is(res.Err, ErrPriceGapPerPositionCap) {
		t.Fatalf("err=%v, want ErrPriceGapPerPositionCap", res.Err)
	}
}

func TestRiskGate_Budget_JustFits(t *testing.T) {
	store := newFakeStore()
	tr := newGateTestTracker(t, store, newFakeDelistChecker(), defaultGateConfig())
	active := []*models.PriceGapPosition{binanceBybitActivePos(2500)}
	res := tr.preEntry(binanceBybitCand(), 2500, freshDetection(), active)
	if !res.Approved {
		t.Fatalf("2500+2500 == budget 5000 must pass; err=%v reason=%s", res.Err, res.Reason)
	}
}

func TestRiskGate_Budget_Overflows(t *testing.T) {
	store := newFakeStore()
	tr := newGateTestTracker(t, store, newFakeDelistChecker(), defaultGateConfig())
	active := []*models.PriceGapPosition{binanceBybitActivePos(2500)}
	res := tr.preEntry(binanceBybitCand(), 2501, freshDetection(), active)
	if res.Approved {
		t.Fatalf("2500+2501=5001 > budget=5000 must block")
	}
	if !errors.Is(res.Err, ErrPriceGapBudgetExceeded) {
		t.Fatalf("err=%v, want ErrPriceGapBudgetExceeded", res.Err)
	}
}

func TestRiskGate_GateConcentration_Uncaps_NoGateLeg(t *testing.T) {
	// Candidate is binance/bybit (no Gate leg). Even if active notional on
	// gate-touching positions is high, the cap doesn't constrain THIS request.
	store := newFakeStore()
	tr := newGateTestTracker(t, store, newFakeDelistChecker(), defaultGateConfig())
	active := []*models.PriceGapPosition{gateBinanceActivePos(3000)}
	res := tr.preEntry(binanceBybitCand(), 1000, freshDetection(), active)
	if !res.Approved {
		t.Fatalf("binance-bybit candidate must not trigger gate cap; err=%v reason=%s", res.Err, res.Reason)
	}
}

func TestRiskGate_GateConcentration_WithGateLeg_Under(t *testing.T) {
	// Cap = 0.5 * 5000 = 2500. Active gate=1000 + request=1000 = 2000 ≤ 2500 → PASS.
	store := newFakeStore()
	tr := newGateTestTracker(t, store, newFakeDelistChecker(), defaultGateConfig())
	active := []*models.PriceGapPosition{gateBinanceActivePos(1000)}
	res := tr.preEntry(binanceGateCand(), 1000, freshDetection(), active)
	if !res.Approved {
		t.Fatalf("2000 under 2500 cap must pass; err=%v reason=%s", res.Err, res.Reason)
	}
}

func TestRiskGate_GateConcentration_WithGateLeg_Over(t *testing.T) {
	// Cap = 2500. Active gate=2000 + request=1000 = 3000 > 2500 → BLOCK.
	store := newFakeStore()
	tr := newGateTestTracker(t, store, newFakeDelistChecker(), defaultGateConfig())
	active := []*models.PriceGapPosition{gateBinanceActivePos(2000)}
	res := tr.preEntry(binanceGateCand(), 1000, freshDetection(), active)
	if res.Approved {
		t.Fatalf("3000 over 2500 cap must block")
	}
	if !errors.Is(res.Err, ErrPriceGapGateConcentrationCap) {
		t.Fatalf("err=%v, want ErrPriceGapGateConcentrationCap", res.Err)
	}
}

func TestRiskGate_Delisted(t *testing.T) {
	store := newFakeStore()
	delist := newFakeDelistChecker()
	delist.delisted["SOON"] = true
	tr := newGateTestTracker(t, store, delist, defaultGateConfig())

	res := tr.preEntry(binanceBybitCand(), 1000, freshDetection(), nil)
	if res.Approved {
		t.Fatalf("delisted symbol must block")
	}
	if !errors.Is(res.Err, ErrPriceGapDelistedLeg) {
		t.Fatalf("err=%v, want ErrPriceGapDelistedLeg", res.Err)
	}
}

func TestRiskGate_StaleBBO(t *testing.T) {
	store := newFakeStore()
	tr := newGateTestTracker(t, store, newFakeDelistChecker(), defaultGateConfig())
	det := DetectionResult{Fired: true, SpreadBps: 250, StalenessSec: 95} // > 90s limit

	res := tr.preEntry(binanceBybitCand(), 1000, det, nil)
	if res.Approved {
		t.Fatalf("staleness 95s > limit 90s must block")
	}
	if !errors.Is(res.Err, ErrPriceGapStaleBBO) {
		t.Fatalf("err=%v, want ErrPriceGapStaleBBO", res.Err)
	}
}

func TestRiskGate_AllPass(t *testing.T) {
	store := newFakeStore()
	tr := newGateTestTracker(t, store, newFakeDelistChecker(), defaultGateConfig())
	active := []*models.PriceGapPosition{binanceBybitActivePos(1000)}
	res := tr.preEntry(binanceBybitCand(), 1000, freshDetection(), active)
	if !res.Approved {
		t.Fatalf("happy path must approve; err=%v reason=%s", res.Err, res.Reason)
	}
	if res.Err != nil {
		t.Fatalf("approved decision must carry nil Err; got %v", res.Err)
	}
}

func TestRiskGate_OrderingInvariant(t *testing.T) {
	// Multiple gates would fail simultaneously:
	//   - disable flag SET (gate 1)
	//   - budget overflow (gate 4)
	// Gate 1 is first in D-17 order, so its error must be returned (not gate 4).
	// This test locks the ordering — any refactor that reorders gates breaks it.
	store := newFakeStore()
	store.disabled["SOON"] = "exec_quality:breach"
	tr := newGateTestTracker(t, store, newFakeDelistChecker(), defaultGateConfig())
	active := []*models.PriceGapPosition{binanceBybitActivePos(4999)}
	// request 9999 → would also trip per-position cap + budget — but gate 1 must fire first.
	res := tr.preEntry(binanceBybitCand(), 9999, freshDetection(), active)
	if res.Approved {
		t.Fatalf("multiple failures — must still block")
	}
	if !errors.Is(res.Err, ErrPriceGapCandidateDisabled) {
		t.Fatalf("D-17 ordering violated: got %v, want ErrPriceGapCandidateDisabled (gate 1 first)", res.Err)
	}
}

// TestRiskGate_DuplicateCandidate_Blocks — regression test for the
// 2026-04-24 UAT incident where SOONUSDT opened at 21:25:04 and again at
// 21:25:34 under the same (symbol, long_exch, short_exch) tuple. Gate 0
// (duplicate-candidate re-entry) must block a second entry while the
// first is still active, independent of MaxConcurrent or Budget.
func TestRiskGate_DuplicateCandidate_Blocks(t *testing.T) {
	store := newFakeStore()
	tr := newGateTestTracker(t, store, newFakeDelistChecker(), defaultGateConfig())

	cand := binanceBybitCand() // Symbol="SOON", long=binance, short=bybit
	active := []*models.PriceGapPosition{{
		ID:            "pg_SOON_binance_bybit_1",
		Symbol:        cand.Symbol,
		LongExchange:  cand.LongExch,
		ShortExchange: cand.ShortExch,
		NotionalUSDT:  1000,
		Status:        models.PriceGapStatusOpen,
	}}

	res := tr.preEntry(cand, 1000, freshDetection(), active)
	if res.Approved {
		t.Fatalf("duplicate candidate must be blocked")
	}
	if !errors.Is(res.Err, ErrPriceGapDuplicateCandidate) {
		t.Fatalf("err=%v, want ErrPriceGapDuplicateCandidate", res.Err)
	}
	if res.Reason != "duplicate_candidate" {
		t.Fatalf("reason=%q, want duplicate_candidate", res.Reason)
	}
}

// TestRiskGate_DuplicateCandidate_DifferentSymbol_Passes — an active
// position with the same exchange pair but a different symbol must NOT
// trigger Gate 0. Uniqueness is on the full (symbol, long_exch, short_exch)
// tuple, not just the exchange pair.
func TestRiskGate_DuplicateCandidate_DifferentSymbol_Passes(t *testing.T) {
	store := newFakeStore()
	tr := newGateTestTracker(t, store, newFakeDelistChecker(), defaultGateConfig())

	cand := binanceBybitCand() // Symbol="SOON"
	// Active position on same exchanges but different symbol.
	active := []*models.PriceGapPosition{{
		ID:            "pg_OTHER_binance_bybit_1",
		Symbol:        "OTHER",
		LongExchange:  cand.LongExch,
		ShortExchange: cand.ShortExch,
		NotionalUSDT:  1000,
		Status:        models.PriceGapStatusOpen,
	}}

	res := tr.preEntry(cand, 1000, freshDetection(), active)
	if !res.Approved {
		t.Fatalf("different-symbol same-exchange-pair must pass gate 0; err=%v reason=%s", res.Err, res.Reason)
	}
}

// Phase 14 Gate 6 (live ramp budget) stubs — bodies land in plan 14-03.
// Ramp-stage budget cap, no-op when live capital OFF, fail-closed posture
// when ramp state unavailable.

func TestRiskGate_Gate6_Ramp_OverBudget_Rejects(t *testing.T) {
	t.Skip("Wave 0 stub: implementation lands in plan 14-03")
}

func TestRiskGate_Gate6_Ramp_NoOpWhenLiveCapitalOff(t *testing.T) {
	t.Skip("Wave 0 stub: implementation lands in plan 14-03")
}

func TestRiskGate_Gate6_Ramp_FailClosedOnStateUnavailable(t *testing.T) {
	t.Skip("Wave 0 stub: implementation lands in plan 14-03")
}
