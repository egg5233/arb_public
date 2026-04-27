// Package pricegaptrader — Phase 999.1 Plan 05 end-to-end regression coverage
// for the bidirectional candidate flow.
//
// These tests drive the FULL tracker pipeline (detectOnce → 4-bar persistence →
// runTick → risk gate → openPair → wire-side leg-role swap → store persist) so
// they catch regressions across Plan 01 (detector + executor swap), Plan 02
// (validator), and Plan 04 (modal toggle) at one assertion site. Unlike the
// unit-level TestOpenPair_Bidirectional* tests in execution_test.go (which
// inject DetectionResult directly), these tests start from BBO state and
// exercise the same code paths that production serves.
//
// Harness reuses the Phase 7 e2e helpers: miniredis-backed *database.Client,
// stubExchange BBO injection, feedFiringSpread for 4-bar priming.
package pricegaptrader

import (
	"testing"
	"time"

	"arb/internal/models"
	"arb/pkg/exchange"
)

// feedFiringSpreadInverse — like feedFiringSpread but produces a NEGATIVE
// (inverse) spread of ~-250 bps. mid_long ≈ 0.975, mid_short ≈ 1.000 →
// (0.975 - 1.000) / 0.9875 * 1e4 ≈ -253 bps.
func feedFiringSpreadInverse(t *testing.T, tr *Tracker, longEx, shortEx *stubExchange, cand models.PriceGapCandidate, start time.Time) time.Time {
	t.Helper()
	longEx.setBBO(cand.Symbol, 0.974, 0.976, start)
	shortEx.setBBO(cand.Symbol, 0.999, 1.001, start)
	now := start
	for i := 0; i < 4; i++ {
		tr.detectOnce(cand, now)
		now = now.Add(time.Minute)
	}
	return now
}

// TestE2E_BidirectionalInverseFire_PaperMode — bidirectional candidate with a
// sustained NEGATIVE spread must (a) fire, (b) swap wire-side leg roles
// (BUY hits configured-short_exch, SELL hits configured-long_exch), (c)
// preserve the configured tuple on CandidateLongExch/CandidateShortExch
// for Phase 10 D-11 active-position guard matching, and (d) record
// FiredDirection="inverse". Driven via runTick in paper mode so no real
// PlaceOrder fires on either leg.
func TestE2E_BidirectionalInverseFire_PaperMode(t *testing.T) {
	db, _ := newE2EClient(t)
	bin := newStubExchange("binance")
	byb := newStubExchange("bybit")
	exch := map[string]exchange.Exchange{"binance": bin, "bybit": byb}

	cand := e2eCandidate()
	cand.Direction = models.PriceGapDirectionBidirectional
	cfg := e2eCfg([]models.PriceGapCandidate{cand})
	cfg.PriceGapPaperMode = true
	tr := NewTracker(exch, db, newFakeDelistChecker(), cfg)

	// Prime 4 bars of -250 bps (configured-short cheaper than configured-long).
	start := time.Now()
	lastTick := feedFiringSpreadInverse(t, tr, bin, byb, cand, start)

	// Paper mode: no PlaceOrder calls expected, but runTick still drives
	// detector → gate → openPair → store.
	tr.runTick(lastTick)

	// Assert paper mode honored — zero PlaceOrder calls on either leg.
	if n := len(bin.placedOrders()); n != 0 {
		t.Errorf("paper mode: expected 0 PlaceOrder on binance, got %d", n)
	}
	if n := len(byb.placedOrders()); n != 0 {
		t.Errorf("paper mode: expected 0 PlaceOrder on bybit, got %d", n)
	}

	active, err := db.GetActivePriceGapPositions()
	if err != nil {
		t.Fatalf("GetActivePriceGapPositions: %v", err)
	}
	if len(active) != 1 {
		t.Fatalf("expected 1 active position, got %d", len(active))
	}
	pos := active[0]

	// Wire-side roles MUST be swapped vs configured.
	// Configured: long=binance, short=bybit. Inverse fire swaps to
	// wire-long=bybit, wire-short=binance.
	if pos.LongExchange != "bybit" {
		t.Errorf("inverse fire: expected wire LongExchange=bybit, got %q", pos.LongExchange)
	}
	if pos.ShortExchange != "binance" {
		t.Errorf("inverse fire: expected wire ShortExchange=binance, got %q", pos.ShortExchange)
	}

	// Configured tuple PRESERVED on observability fields (Plan 10 D-11).
	if pos.CandidateLongExch != "binance" {
		t.Errorf("expected CandidateLongExch=binance, got %q", pos.CandidateLongExch)
	}
	if pos.CandidateShortExch != "bybit" {
		t.Errorf("expected CandidateShortExch=bybit, got %q", pos.CandidateShortExch)
	}

	// Fired direction recorded.
	if pos.FiredDirection != models.PriceGapFiredInverse {
		t.Errorf("expected FiredDirection=%q, got %q", models.PriceGapFiredInverse, pos.FiredDirection)
	}

	// Paper mode stamped on the position (immutable through lifecycle).
	if pos.Mode != models.PriceGapModePaper {
		t.Errorf("expected pos.Mode=%q, got %q", models.PriceGapModePaper, pos.Mode)
	}

	// Status must be open.
	if pos.Status != models.PriceGapStatusOpen {
		t.Errorf("expected pos.Status=open, got %q", pos.Status)
	}
}

// TestE2E_PinnedRejectsInverseSign_BackwardCompat — legacy candidate with
// Direction="" (or the default "pinned") must NOT fire on a sustained
// NEGATIVE spread. This pins the Plan 01 latent-bug fix: pre-Phase-999.1
// detector used math.Abs and would fire either sign for pinned candidates.
// A regression here would re-introduce wrong-side trades on inverse spreads.
func TestE2E_PinnedRejectsInverseSign_BackwardCompat(t *testing.T) {
	db, _ := newE2EClient(t)
	bin := newStubExchange("binance")
	byb := newStubExchange("bybit")
	exch := map[string]exchange.Exchange{"binance": bin, "bybit": byb}

	cand := e2eCandidate() // Direction="" → legacy / treated as pinned
	cfg := e2eCfg([]models.PriceGapCandidate{cand})
	tr := NewTracker(exch, db, newFakeDelistChecker(), cfg)

	start := time.Now()
	lastTick := feedFiringSpreadInverse(t, tr, bin, byb, cand, start)

	tr.runTick(lastTick)

	// No orders, no positions — pinned mode rejects the inverse-sign fire.
	if n := len(bin.placedOrders()); n != 0 {
		t.Errorf("legacy pinned: expected 0 orders on binance, got %d", n)
	}
	if n := len(byb.placedOrders()); n != 0 {
		t.Errorf("legacy pinned: expected 0 orders on bybit, got %d", n)
	}
	active, err := db.GetActivePriceGapPositions()
	if err != nil {
		t.Fatalf("GetActivePriceGapPositions: %v", err)
	}
	if len(active) != 0 {
		t.Fatalf("legacy pinned candidate must not fire on negative spread, got %d positions", len(active))
	}
}

// TestE2E_LegacyPositionDecode_BackwardCompat — pre-Phase-999.1 PriceGapPosition
// JSON in Redis (no FiredDirection / CandidateLongExch / CandidateShortExch
// fields) must decode cleanly via the store contract. Asserts zero-value
// defaults on the new fields and that legacy positions still appear in the
// active set as Open.
func TestE2E_LegacyPositionDecode_BackwardCompat(t *testing.T) {
	db, mr := newE2EClient(t)

	// Hand-craft a legacy JSON payload that omits the Phase 999.1 fields.
	// Mirrors Phase 8 D-15 schema. Redis keys: pg:positions hash + pg:positions:active set.
	const posID = "pg-legacy-1"
	const legacyJSON = `{
		"id": "pg-legacy-1",
		"symbol": "SOON",
		"long_exchange": "binance",
		"short_exchange": "bybit",
		"long_size": 1000,
		"short_size": 1000,
		"long_fill_price": 1.000,
		"short_fill_price": 1.025,
		"status": "open",
		"opened_at": "2026-04-01T00:00:00Z",
		"mode": "live"
	}`
	mr.HSet("pg:positions", posID, legacyJSON)
	mr.SAdd("pg:positions:active", posID)

	active, err := db.GetActivePriceGapPositions()
	if err != nil {
		t.Fatalf("GetActivePriceGapPositions: %v", err)
	}
	if len(active) != 1 {
		t.Fatalf("expected 1 legacy position, got %d", len(active))
	}
	pos := active[0]

	// Phase 999.1 fields default to zero values on legacy decode.
	if pos.FiredDirection != "" {
		t.Errorf("legacy FiredDirection should default to \"\", got %q", pos.FiredDirection)
	}
	if pos.CandidateLongExch != "" {
		t.Errorf("legacy CandidateLongExch should default to \"\", got %q", pos.CandidateLongExch)
	}
	if pos.CandidateShortExch != "" {
		t.Errorf("legacy CandidateShortExch should default to \"\", got %q", pos.CandidateShortExch)
	}

	// Existing fields decoded normally — close path reads these face-value.
	if pos.LongExchange != "binance" || pos.ShortExchange != "bybit" {
		t.Errorf("legacy wire-side roles = (%q, %q), want (binance, bybit)",
			pos.LongExchange, pos.ShortExchange)
	}
	if pos.Status != models.PriceGapStatusOpen {
		t.Errorf("legacy Status = %q, want open", pos.Status)
	}
}
