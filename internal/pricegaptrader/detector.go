package pricegaptrader

import (
	"fmt"
	"math"
	"time"

	"arb/internal/models"
	"arb/pkg/exchange"
)

// candidateBars — per-candidate in-memory rolling bar state.
// Key in Tracker.bars map = PriceGapCandidate.ID(). Resets on process restart (D-07).
type candidateBars struct {
	ring         barRing
	lastSampleAt time.Time
}

// barRing — 4-slot circular buffer of 1m spread closes (D-07, PG-01).
// Fixed-size array (T-08-10 mitigation: no unbounded growth).
type barRing struct {
	closes  [4]float64 // spread bps
	valid   [4]bool
	idx     int
	lastMin int64 // unix minute of most recent completed bar (dedup within same minute)
}

// push records a new 1m close; returns true if the minute was new (actually stored).
// Dedup within the same minute: if unixMinute equals the last-stored minute, no-op.
func (b *barRing) push(unixMinute int64, spreadBps float64) bool {
	if unixMinute == b.lastMin && b.valid[(b.idx+3)%4] {
		return false
	}
	b.closes[b.idx] = spreadBps
	b.valid[b.idx] = true
	b.idx = (b.idx + 1) % 4
	b.lastMin = unixMinute
	return true
}

// allExceedDirected returns true only when ALL 4 bars are populated, each
// |s|≥T, all same sign, AND the sign matches the candidate's direction policy.
//
//   - "pinned" (or empty/default): only positive-sign continuity fires
//     (configured long_exch cheaper than short_exch by ≥ T, i.e. spread > 0).
//     Closes the latent Phase-8 bug where allExceed used math.Abs and
//     silently fired any sign — pinned candidates that fired on inverse
//     spread were placing wrong-side trades (Pitfall 2 / PG-DIR-01).
//   - "bidirectional": either positive or negative same-sign continuity
//     fires; the executor swaps wire-side leg roles for negative fires.
//
// Same-sign check (T-08-11 mitigation) prevents a threshold crossing followed
// by a sign reversion from firing a false event in BOTH modes.
func (b *barRing) allExceedDirected(T float64, direction string) bool {
	var firstSign float64
	for i, ok := range b.valid {
		if !ok {
			return false
		}
		s := b.closes[i]
		if math.Abs(s) < T {
			return false
		}
		if i == 0 {
			firstSign = s
		} else if s*firstSign < 0 {
			return false
		}
	}
	// Direction filter — pinned (default/empty) requires positive-sign
	// continuity; bidirectional accepts either.
	if direction != models.PriceGapDirectionBidirectional {
		if firstSign < 0 {
			return false
		}
	}
	return true
}

// barsAllValidAndExceed reports whether the ring has 4 valid bars with same
// sign and magnitude ≥ T, ignoring direction policy. Used by detectOnce to
// distinguish "pinned_wrong_sign" (sign-only rejection) from
// "insufficient_persistence" (anything else) for log diagnostics — does NOT
// affect fire semantics. PG-DIR-01.
func (b *barRing) barsAllValidAndExceed(T float64) bool {
	var firstSign float64
	for i, ok := range b.valid {
		if !ok {
			return false
		}
		s := b.closes[i]
		if math.Abs(s) < T {
			return false
		}
		if i == 0 {
			firstSign = s
		} else if s*firstSign < 0 {
			return false
		}
	}
	return true
}

// allExceed — backward-compat wrapper preserving the pre-PG-DIR-01 signature
// for any external callers (currently none in production; retained for tests
// that still call the unparameterized form).
func (b *barRing) allExceed(T float64) bool {
	return b.allExceedDirected(T, models.PriceGapDirectionPinned)
}

// computeSpreadBps — D-06: (pxLong - pxShort) / mid × 10_000 bps.
// mid = (pxLong + pxShort) / 2. Returns 0 on non-positive mid (guards against
// adapter bug that could publish a zero or negative quote).
func computeSpreadBps(pxLong, pxShort float64) float64 {
	mid := (pxLong + pxShort) / 2.0
	if mid <= 0 {
		return 0
	}
	return (pxLong - pxShort) / mid * 10_000.0
}

// sampleLegs fetches BBOs on both legs. Returns (midLong, midShort, bboOK, err).
// bboOK is false if either GetBBO returned !ok — this is the primary freshness
// signal because pkg/exchange.BBO carries no timestamp in the current adapter
// surface. The per-candidate wall-clock staleness check lives in detectOnce.
func sampleLegs(
	exchanges map[string]exchange.Exchange,
	cand models.PriceGapCandidate,
) (midLong, midShort float64, bboOK bool, err error) {
	longEx, ok := exchanges[cand.LongExch]
	if !ok {
		return 0, 0, false, fmt.Errorf("unknown long exchange: %s", cand.LongExch)
	}
	shortEx, ok := exchanges[cand.ShortExch]
	if !ok {
		return 0, 0, false, fmt.Errorf("unknown short exchange: %s", cand.ShortExch)
	}

	bboL, okL := longEx.GetBBO(cand.Symbol)
	if !okL {
		return 0, 0, false, fmt.Errorf("long GetBBO not populated: %s", cand.Symbol)
	}
	bboS, okS := shortEx.GetBBO(cand.Symbol)
	if !okS {
		return 0, 0, false, fmt.Errorf("short GetBBO not populated: %s", cand.Symbol)
	}

	midLong = (bboL.Bid + bboL.Ask) / 2.0
	midShort = (bboS.Bid + bboS.Ask) / 2.0
	bboOK = true
	return
}

// DetectionResult — output of detectOnce.
type DetectionResult struct {
	Fired        bool
	SpreadBps    float64
	MidLong      float64
	MidShort     float64
	StalenessSec float64 // wall-clock seconds since prior sample for this candidate
	Reason       string  // populated when Fired=false, for log diagnostics
}

// detectOnce runs a single-tick detection for one candidate. Pure function apart from
// BBO fetch + bar state mutation; unit-testable with stubExchange.
//
// Freshness gate (T-08-09 mitigation; D-22 PriceGapKlineStalenessSec):
// pkg/exchange.BBO carries no publish timestamp, so the detector measures
// the wall-clock interval between successive successful samples for a given
// candidate. If the gap exceeds PriceGapKlineStalenessSec the sample is
// rejected as stale. The first sample after startup (or after a long absence)
// is always kept (no staleness to compare against), but the 4-bar persistence
// gate prevents any fire until 4 consecutive bars have accumulated.
func (t *Tracker) detectOnce(cand models.PriceGapCandidate, now time.Time) DetectionResult {
	midL, midS, _, err := sampleLegs(t.exchanges, cand)
	if err != nil {
		return DetectionResult{Reason: "sample_error: " + err.Error()}
	}

	spread := computeSpreadBps(midL, midS)
	stalenessLimit := time.Duration(t.cfg.PriceGapKlineStalenessSec) * time.Second

	t.barsMu.Lock()
	cb := t.bars[cand.ID()]
	if cb == nil {
		cb = &candidateBars{}
		t.bars[cand.ID()] = cb
	}
	// Wall-clock staleness: if the gap since the last successful sample exceeds
	// the configured limit, reset the ring — a stale chain can't count toward
	// 4-bar persistence.
	var stalenessSec float64
	stale := false
	if !cb.lastSampleAt.IsZero() {
		gap := now.Sub(cb.lastSampleAt)
		stalenessSec = gap.Seconds()
		if gap >= stalenessLimit {
			cb.ring = barRing{}
			stale = true
		}
	}
	cb.ring.push(now.Unix()/60, spread)
	cb.lastSampleAt = now
	fired := !stale && cb.ring.allExceedDirected(cand.ThresholdBps, cand.Direction)
	// Snapshot post-fire state for diagnostic Reason classification while still
	// under barsMu (allExceed reads the same closes/valid arrays push() just
	// mutated; we must NOT release the lock between these reads).
	pinnedWrongSign := false
	if !fired && !stale && cand.Direction != models.PriceGapDirectionBidirectional {
		pinnedWrongSign = cb.ring.barsAllValidAndExceed(cand.ThresholdBps)
	}
	t.barsMu.Unlock()

	reason := ""
	if !fired {
		switch {
		case stale:
			reason = "stale_bbo"
		case pinnedWrongSign:
			// 4 same-sign bars with |s|≥T but sign was negative on a pinned
			// candidate — PG-DIR-01 sign filter rejected. Distinct reason for
			// observability so Phase 9 dashboards can surface latent-bug
			// closures without changing fire semantics.
			reason = "pinned_wrong_sign"
		default:
			reason = "insufficient_persistence"
		}
	}
	return DetectionResult{
		Fired:        fired,
		SpreadBps:    spread,
		MidLong:      midL,
		MidShort:     midS,
		StalenessSec: stalenessSec,
		Reason:       reason,
	}
}
