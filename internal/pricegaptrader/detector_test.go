package pricegaptrader

import (
	"testing"
	"time"

	"arb/internal/config"
	"arb/internal/models"
	"arb/pkg/exchange"
)

// Shared setup: returns tracker with one candidate (+fresh config defaults).
func newDetectorTestTracker(_ *testing.T, cand models.PriceGapCandidate) (*Tracker, *stubExchange, *stubExchange) {
	longEx := newStubExchange("binance")
	shortEx := newStubExchange("gate")
	exch := map[string]exchange.Exchange{"binance": longEx, "gate": shortEx}
	cfg := &config.Config{
		PriceGapEnabled:           true,
		PriceGapBarPersistence:    4,
		PriceGapKlineStalenessSec: 90,
		PriceGapCandidates:        []models.PriceGapCandidate{cand},
	}
	tr := NewTracker(exch, nil, nil, cfg)
	return tr, longEx, shortEx
}

func defaultCandidate() models.PriceGapCandidate {
	return models.PriceGapCandidate{
		Symbol:             "SOON",
		LongExch:           "binance",
		ShortExch:          "gate",
		ThresholdBps:       200,
		MaxPositionUSDT:    5000,
		ModeledSlippageBps: 47.9,
	}
}

// setBBOs at a price level that produces target spread (bps) on both legs.
// Uses pxLong / pxShort around mid=100; diff = spreadBps/10000 * 100 * 2 / 2.
// Simpler: pass explicit long/short mid prices.
func setMids(longEx, shortEx *stubExchange, symbol string, pxLong, pxShort float64, ts time.Time) {
	// Use tight bid/ask so mid == pxLong/pxShort exactly.
	longEx.setBBO(symbol, pxLong, pxLong, ts)
	shortEx.setBBO(symbol, pxShort, pxShort, ts)
}

// ---- barRing unit tests -----------------------------------------------------

func TestBarRing_PushDedup(t *testing.T) {
	var r barRing
	minute := int64(1000)
	if !r.push(minute, 100) {
		t.Fatalf("first push should succeed")
	}
	if r.push(minute, 150) {
		t.Fatalf("second push at same minute should be deduped")
	}
	// Different minute should accept.
	if !r.push(minute+1, 200) {
		t.Fatalf("push at new minute should succeed")
	}
}

func TestBarRing_AllExceed_Empty(t *testing.T) {
	var r barRing
	if r.allExceed(100) {
		t.Fatalf("empty ring must not report allExceed")
	}
}

func TestBarRing_AllExceed_BelowThreshold(t *testing.T) {
	var r barRing
	for i := int64(0); i < 4; i++ {
		r.push(i, 150) // below T=200
	}
	if r.allExceed(200) {
		t.Fatalf("all bars at 150 should not exceed T=200")
	}
}

func TestBarRing_AllExceed_MixedSigns(t *testing.T) {
	var r barRing
	// +250, +250, -250, +250 — same |v| but mixed sign, must NOT fire (T-08-11).
	r.push(1, +250)
	r.push(2, +250)
	r.push(3, -250)
	r.push(4, +250)
	if r.allExceed(200) {
		t.Fatalf("mixed-sign bars must not fire allExceed (same-sign invariant)")
	}
}

func TestBarRing_AllExceed_Pass(t *testing.T) {
	var r barRing
	for i := int64(1); i <= 4; i++ {
		r.push(i, +250)
	}
	if !r.allExceed(200) {
		t.Fatalf("four same-sign bars above threshold must fire")
	}
}

// ---- detectOnce integration tests ------------------------------------------

// setupFireSequence: advances clock by 60s, sets BBOs at a +250 bps spread,
// calls detectOnce. Repeats `bars` times starting from start.
func pushBarsAt250Bps(t *testing.T, tr *Tracker, longEx, shortEx *stubExchange, cand models.PriceGapCandidate, start time.Time, bars int) (lastResult DetectionResult, lastAt time.Time) {
	t.Helper()
	clk := newFakeClock(start)
	for i := 0; i < bars; i++ {
		// Spread = +250 bps → mid=100, diff=2.5 → pxLong=101.25, pxShort=98.75.
		setMids(longEx, shortEx, cand.Symbol, 101.25, 98.75, clk.Now())
		lastResult = tr.detectOnce(cand, clk.Now())
		lastAt = clk.Now()
		clk.Advance(60 * time.Second)
	}
	return lastResult, lastAt
}

func TestDetectOnce_InsufficientBars(t *testing.T) {
	cand := defaultCandidate()
	tr, longEx, shortEx := newDetectorTestTracker(t, cand)
	start := time.Date(2026, 4, 21, 12, 0, 0, 0, time.UTC)
	res, _ := pushBarsAt250Bps(t, tr, longEx, shortEx, cand, start, 3)
	if res.Fired {
		t.Fatalf("3 bars should not fire; got Fired=true")
	}
	if res.Reason != "insufficient_persistence" {
		t.Fatalf("reason=%q, want insufficient_persistence", res.Reason)
	}
	if res.SpreadBps < 240 || res.SpreadBps > 260 {
		t.Fatalf("spread=%.2f, expected ~250", res.SpreadBps)
	}
}

func TestDetectOnce_StalBBO(t *testing.T) {
	cand := defaultCandidate()
	tr, longEx, shortEx := newDetectorTestTracker(t, cand)
	clk := newFakeClock(time.Date(2026, 4, 21, 12, 0, 0, 0, time.UTC))

	// Seed first sample.
	setMids(longEx, shortEx, cand.Symbol, 101.25, 98.75, clk.Now())
	first := tr.detectOnce(cand, clk.Now())
	if first.Fired {
		t.Fatalf("first sample should never fire")
	}

	// Advance past staleness limit (90s). Next detectOnce should reset the ring
	// and report stale_bbo.
	clk.Advance(120 * time.Second)
	setMids(longEx, shortEx, cand.Symbol, 101.25, 98.75, clk.Now())
	res := tr.detectOnce(cand, clk.Now())
	if res.Fired {
		t.Fatalf("stale BBO must not fire")
	}
	if res.Reason != "stale_bbo" {
		t.Fatalf("reason=%q, want stale_bbo", res.Reason)
	}
}

// ---- PG-DIR-01: direction-aware allExceedDirected tests --------------------

// Helper: push 4 bars with the given spread sign at fresh per-minute timestamps.
// Sets BBOs to produce target spread (positive: pxLong > pxShort).
func pushBarsAtSpread(t *testing.T, tr *Tracker, longEx, shortEx *stubExchange, cand models.PriceGapCandidate, start time.Time, bars int, signedBps float64) DetectionResult {
	t.Helper()
	clk := newFakeClock(start)
	mid := 100.0
	// spread = (pxLong - pxShort) / mid * 1e4, with mid = (pxLong+pxShort)/2.
	// Set pxLong = mid + half, pxShort = mid - half → spread = 2*half/mid * 1e4.
	// Solve: half = signedBps / 2 / 1e4 * mid.
	half := signedBps / 2.0 / 10_000.0 * mid
	pxLong := mid + half
	pxShort := mid - half
	var res DetectionResult
	for i := 0; i < bars; i++ {
		setMids(longEx, shortEx, cand.Symbol, pxLong, pxShort, clk.Now())
		res = tr.detectOnce(cand, clk.Now())
		clk.Advance(60 * time.Second)
	}
	return res
}

// TestDirection_PinnedFiresPositive — pinned + 4 same-sign positive bars at
// |spread|≥T → fires (configured baseline).
func TestDirection_PinnedFiresPositive(t *testing.T) {
	cand := defaultCandidate()
	cand.Direction = models.PriceGapDirectionPinned
	tr, longEx, shortEx := newDetectorTestTracker(t, cand)
	start := time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC)
	res := pushBarsAtSpread(t, tr, longEx, shortEx, cand, start, 4, +250)
	if !res.Fired {
		t.Fatalf("pinned + positive 4-bar must fire: reason=%q spread=%.2f", res.Reason, res.SpreadBps)
	}
}

// TestDirection_PinnedSignFilter — pinned + 4 same-sign NEGATIVE bars at
// |spread|≥T → does NOT fire. Closes the latent Phase-8 bug where allExceed
// used math.Abs and silently fired any sign.
func TestDirection_PinnedSignFilter(t *testing.T) {
	cand := defaultCandidate()
	cand.Direction = models.PriceGapDirectionPinned
	tr, longEx, shortEx := newDetectorTestTracker(t, cand)
	start := time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC)
	res := pushBarsAtSpread(t, tr, longEx, shortEx, cand, start, 4, -250)
	if res.Fired {
		t.Fatalf("pinned + negative 4-bar must NOT fire (sign filter); spread=%.2f", res.SpreadBps)
	}
	if res.Reason != "pinned_wrong_sign" {
		t.Fatalf("reason=%q, want pinned_wrong_sign", res.Reason)
	}
}

// TestDirection_DefaultEmptyTreatedAsPinned — Direction="" treated as pinned;
// negative-direction bars do NOT fire.
func TestDirection_DefaultEmptyTreatedAsPinned(t *testing.T) {
	cand := defaultCandidate()
	cand.Direction = "" // legacy / pre-Phase-999.1
	tr, longEx, shortEx := newDetectorTestTracker(t, cand)
	start := time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC)
	res := pushBarsAtSpread(t, tr, longEx, shortEx, cand, start, 4, -250)
	if res.Fired {
		t.Fatalf("default-empty + negative 4-bar must NOT fire (treated as pinned)")
	}
}

// TestDirection_BidirectionalFiresEitherSign_Positive — bidirectional + 4
// positive bars → fires.
func TestDirection_BidirectionalFiresEitherSign_Positive(t *testing.T) {
	cand := defaultCandidate()
	cand.Direction = models.PriceGapDirectionBidirectional
	tr, longEx, shortEx := newDetectorTestTracker(t, cand)
	start := time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC)
	res := pushBarsAtSpread(t, tr, longEx, shortEx, cand, start, 4, +250)
	if !res.Fired {
		t.Fatalf("bidirectional + positive 4-bar must fire: reason=%q", res.Reason)
	}
	if res.SpreadBps < 240 || res.SpreadBps > 260 {
		t.Fatalf("spread=%.2f, expected ~+250", res.SpreadBps)
	}
}

// TestDirection_BidirectionalFiresEitherSign_Negative — bidirectional + 4
// negative bars → fires; SpreadBps preserves the negative sign for the executor.
func TestDirection_BidirectionalFiresEitherSign_Negative(t *testing.T) {
	cand := defaultCandidate()
	cand.Direction = models.PriceGapDirectionBidirectional
	tr, longEx, shortEx := newDetectorTestTracker(t, cand)
	start := time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC)
	res := pushBarsAtSpread(t, tr, longEx, shortEx, cand, start, 4, -250)
	if !res.Fired {
		t.Fatalf("bidirectional + negative 4-bar must fire: reason=%q", res.Reason)
	}
	if res.SpreadBps > -240 || res.SpreadBps < -260 {
		t.Fatalf("spread=%.2f, expected ~-250 (sign preserved for executor swap)", res.SpreadBps)
	}
}

// TestDirection_BidirectionalRejectsMixedSigns — bidirectional + mixed-sign
// bars do NOT fire (T-08-11 same-sign continuity preserved for both modes).
func TestDirection_BidirectionalRejectsMixedSigns(t *testing.T) {
	cand := defaultCandidate()
	cand.Direction = models.PriceGapDirectionBidirectional
	tr, longEx, shortEx := newDetectorTestTracker(t, cand)
	start := time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC)
	clk := newFakeClock(start)
	// 4 bars: +250, +250, -250, -250 — magnitude OK on each, sign-flipped → must not fire.
	for i, signedBps := range []float64{+250, +250, -250, -250} {
		mid := 100.0
		half := signedBps / 2.0 / 10_000.0 * mid
		pxLong := mid + half
		pxShort := mid - half
		setMids(longEx, shortEx, cand.Symbol, pxLong, pxShort, clk.Now())
		res := tr.detectOnce(cand, clk.Now())
		clk.Advance(60 * time.Second)
		if i == 3 && res.Fired {
			t.Fatalf("bidirectional + mixed-sign must NOT fire (same-sign invariant T-08-11)")
		}
	}
}

func TestDetectOnce_FiresOnFourthBar(t *testing.T) {
	cand := defaultCandidate()
	tr, longEx, shortEx := newDetectorTestTracker(t, cand)
	start := time.Date(2026, 4, 21, 12, 0, 0, 0, time.UTC)

	// 4 consecutive same-sign bars above T=200 with fresh samples (60s apart,
	// well under 90s staleness limit).
	res, _ := pushBarsAt250Bps(t, tr, longEx, shortEx, cand, start, 4)
	if !res.Fired {
		t.Fatalf("4 consecutive +250 bps bars must fire; reason=%q spread=%.2f", res.Reason, res.SpreadBps)
	}
	if res.SpreadBps < 240 || res.SpreadBps > 260 {
		t.Fatalf("spread=%.2f expected ~250", res.SpreadBps)
	}
	if res.MidLong != 101.25 || res.MidShort != 98.75 {
		t.Fatalf("mids: long=%.2f short=%.2f", res.MidLong, res.MidShort)
	}
}
