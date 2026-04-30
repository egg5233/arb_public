// Package pricegaptrader — Auto-discovery scanner core (Phase 11 / PG-DISC-01).
//
// Scanner walks an operator-curated universe (≤20 symbols, validated upstream
// by Plan 01) across the cross-product of supported exchanges and emits a
// per-(symbol×ordered-pair) cycle record. Six hard gates are evaluated in a
// short-circuit cheapest-first order; only when ALL gates pass does the
// scorer compute a non-zero magnitude in [1,100] (D-01 gate-then-magnitude).
//
// Decisions implemented here:
//   - D-01 gate-then-magnitude: ALL gates must pass before non-zero score.
//   - D-02 integer 0-100 score.
//   - D-03 sub-scores recorded on every CycleRecord.
//   - D-04 rejected candidates captured with reason code (every pair writes).
//   - D-08 silent skip for singletons (only one listing).
//   - D-11 default OFF: with PriceGapDiscoveryEnabled=false, only an
//     enabled-flag stamp is emitted via TelemetryWriter.
//   - D-13 compile-time read-only: Scanner accepts RegistryReader, never
//     the concrete registry mutator type. Enforced by scanner_static_test.go.
//
// Scope: Plan 04 ships Scanner + interfaces ONLY. Plan 05 implements
// TelemetryWriter (concrete Telemetry struct), wires scanLoop into
// Tracker.Start, adds REST handlers, and constructs Scanner in cmd/main.go.
//
// Module-boundary contract (T-11-07): this file imports only models, config,
// pkg/exchange, and pkg/utils. The live trading engine packages are not
// reachable from this scanner — see CLAUDE.md "Module boundaries".
package pricegaptrader

import (
	"context"
	"math"
	"strings"
	"sync"
	"time"

	"arb/internal/config"
	"arb/internal/models"
	"arb/pkg/exchange"
	"arb/pkg/utils"
)

// ScanReason is the discriminator on CycleRecord.WhyRejected. The empty
// string (ReasonAccepted) signals the candidate passed every gate. The
// remaining 8 values cover the documented rejection paths (D-04).
type ScanReason string

const (
	// ReasonAccepted (empty string) — all gates passed.
	ReasonAccepted ScanReason = ""

	// ReasonInsufficientPersistence — fewer than 4 same-sign bars at or
	// above the configured threshold (PG-01 / Pitfall 1 antidote).
	ReasonInsufficientPersistence ScanReason = "insufficient_persistence"

	// ReasonStaleBBO — wall-clock gap since last sample for either leg
	// exceeded PriceGapKlineStalenessSec; bar ring was reset.
	ReasonStaleBBO ScanReason = "stale_bbo"

	// ReasonInsufficientDepth — minimum clearable USDT across the two legs
	// at threshold-bps spread fell below PriceGapDiscoveryMinDepthUSDT.
	ReasonInsufficientDepth ScanReason = "insufficient_depth"

	// ReasonDenylist — symbol or symbol@exchange tuple matched
	// PriceGapDiscoveryDenylist (D-07).
	ReasonDenylist ScanReason = "denylist"

	// ReasonBybitBlackout — wall-clock minute fell inside the :04..:05:30
	// window and either leg was bybit. Hardcoded per CLAUDE.local.md scan
	// schedule.
	ReasonBybitBlackout ScanReason = "bybit_blackout"

	// ReasonSymbolNotListedLong — long-leg adapter returned ok=false on
	// GetBBO for the symbol; recorded but does not increment the cross-pair
	// error counter (the singleton case is silent-skipped at the caller).
	ReasonSymbolNotListedLong ScanReason = "symbol_not_listed_long"

	// ReasonSymbolNotListedShort — short-leg adapter returned ok=false on
	// GetBBO for the symbol; same caveats as ReasonSymbolNotListedLong.
	ReasonSymbolNotListedShort ScanReason = "symbol_not_listed_short"

	// ReasonSampleError — adapter returned an error during BBO/Depth read.
	// Increments the consecutive-error budget; 5 consecutive trips abort
	// the cycle with TelemetryWriter.WriteCycleFailed.
	ReasonSampleError ScanReason = "sample_error"
)

// CycleRecord is the per-(symbol × ordered cross-pair) emission shape.
// Plan 05 Telemetry serializes the parent CycleSummary to pg:scan:cycles.
type CycleRecord struct {
	// Identity
	Symbol    string `json:"symbol"`
	LongExch  string `json:"long_exch"`
	ShortExch string `json:"short_exch"`

	// Top-line score (D-02 integer 0-100). 0 when WhyRejected is non-empty.
	Score int `json:"score"`

	// Sub-scores (D-03). Always populated even for rejected records so
	// dashboards can plot "would have been" magnitude alongside reason.
	SpreadBps       float64 `json:"spread_bps"`
	PersistenceBars int     `json:"persistence_bars"`
	DepthScore      float64 `json:"depth_score"`
	FreshnessAgeSec int     `json:"freshness_age_s"`
	FundingBpsPerHr float64 `json:"funding_bps_h"`

	// Per-record gate trace.
	GatesPassed []string `json:"gates_passed"`
	GatesFailed []string `json:"gates_failed"`

	// Rejection metadata (D-04). Empty WhyRejected = accepted.
	WhyRejected     ScanReason `json:"why_rejected,omitempty"`
	AlreadyPromoted bool       `json:"already_promoted"`

	Timestamp int64 `json:"ts"`
}

// CycleSummary is the per-cycle envelope. WhyRejected is a per-reason
// histogram so dashboards can plot rejection-category trends without
// re-scanning Records.
type CycleSummary struct {
	StartedAt      int64              `json:"started_at"`
	CompletedAt    int64              `json:"completed_at"`
	DurationMs     int64              `json:"duration_ms"`
	CandidatesSeen int                `json:"candidates_seen"`
	Accepted       int                `json:"accepted"`
	Rejected       int                `json:"rejected"`
	Errors         int                `json:"errors"`
	WhyRejected    map[ScanReason]int `json:"why_rejected_histogram"`
	Records        []CycleRecord      `json:"records"`
}

// TelemetryWriter is the output sink for the scanner. Plan 04 declares the
// interface; Plan 05 implements the concrete Telemetry struct (Redis-backed)
// and injects it into NewScanner.
//
// Each method MUST be safe to call from a single goroutine (the scanner's
// RunCycle owner). Plan 05's writer adds its own internal serialization for
// concurrent observers.
type TelemetryWriter interface {
	// WriteCycle persists the cycle envelope (records + histogram + counts).
	WriteCycle(ctx context.Context, summary CycleSummary) error
	// WriteEnabledFlag stamps the master switch state. Called once per
	// RunCycle invocation, before any other work.
	WriteEnabledFlag(ctx context.Context, enabled bool) error
	// WriteSymbolNoCrossPair increments the singleton-skip counter for a
	// symbol whose universe row had <2 listings (D-08 silent skip).
	WriteSymbolNoCrossPair(ctx context.Context) error
	// WriteCycleFailed marks the current cycle as aborted because the
	// consecutive-error budget tripped (RESEARCH §1).
	WriteCycleFailed(ctx context.Context) error
}

// barRingExceedThreshold is the local equivalent of detector.go's
// barRing.allExceedDirected with a fixed Bidirectional policy. The scanner
// uses bidirectional-only because at discovery time it does not yet know
// which leg the operator will pin long; sign-pinning is a Plan 05/12 concern
// at promotion time.
//
// The function is a pure read — caller must hold the mutex protecting the
// ring; the scanner owns the per-key bar ring under s.mu.
func barRingExceedThreshold(b *barRing, T float64) bool {
	return b.allExceedDirected(T, models.PriceGapDirectionBidirectional)
}

// barRingPopulatedCount returns how many of the 4 ring slots are valid.
// Useful for the PersistenceBars sub-score (D-03).
func barRingPopulatedCount(b *barRing) int {
	c := 0
	for _, ok := range b.valid {
		if ok {
			c++
		}
	}
	return c
}

// legSample is the per-leg read result aggregated by sampleLeg.
type legSample struct {
	bid            float64
	ask            float64
	mid            float64
	depthClearable float64
	freshnessSec   int
	listingPresent bool
	depthPresent   bool
}

// Scanner is the read-only auto-discovery cycle engine.
//
// Concurrency: a single goroutine (Plan 05's scanLoop) owns RunCycle. The
// per-key bar ring map and lastSample map are guarded by s.mu so external
// observers (none today; reserved for future debug introspection) can read
// state safely. consecErr is incremented inside scanPair while s.mu is held.
//
// Phase 12 D-17: registry was widened from RegistryReader to *Registry so the
// PromotionController can call Add/Delete via the chokepoint. Direct mutator
// calls from scanner.go itself remain forbidden by scanner_static_test.go's
// `cfg.PriceGapCandidates =` regex; the chokepoint discipline is enforced by
// PromotionController not having a *config.Config field of its own (it only
// reads thresholds), so the scanner→controller→registry path is the ONLY
// auto-mutation route in Phase 12.
type Scanner struct {
	cfg       *config.Config
	registry  *Registry
	exchanges map[string]exchange.Exchange
	telemetry TelemetryWriter
	promotion *PromotionController // Phase 12 (D-16); nil-safe — RunCycle checks
	log       *utils.Logger
	nowFunc   func() time.Time

	mu         sync.Mutex
	bars       map[string]*barRing  // key = "sym:long:short"
	lastSample map[string]time.Time // key = "sym:exch"
	consecErr  int                  // resets on next successful sample
}

// NewScanner constructs a Scanner. log is optional; when nil the scanner
// emits no logs (callers in Plan 05 will pass utils.NewLogger("scanner")).
//
// Phase 12 D-17 swap: registry is now *Registry (concrete) so the
// PromotionController can call Add/Delete via this same instance. The
// promotion parameter is nil-safe — RunCycle checks for nil before calling
// Apply, so unit tests that only exercise scanner gating continue to pass
// nil here.
func NewScanner(
	cfg *config.Config,
	registry *Registry,
	exchanges map[string]exchange.Exchange,
	telemetry TelemetryWriter,
	promotion *PromotionController,
	log *utils.Logger,
) *Scanner {
	return &Scanner{
		cfg:        cfg,
		registry:   registry,
		exchanges:  exchanges,
		telemetry:  telemetry,
		promotion:  promotion,
		log:        log,
		nowFunc:    time.Now,
		bars:       make(map[string]*barRing),
		lastSample: make(map[string]time.Time),
	}
}

// RunCycle performs one full discovery cycle. Default-OFF emits only the
// enabled-flag stamp. The cycle is bounded by the consecutive-error budget
// (5 sample errors abort and short-circuit remaining work).
func (s *Scanner) RunCycle(ctx context.Context, now time.Time) {
	if !s.cfg.PriceGapDiscoveryEnabled {
		_ = s.telemetry.WriteEnabledFlag(ctx, false)
		return
	}
	_ = s.telemetry.WriteEnabledFlag(ctx, true)

	// Snapshot config state under cfg.RLock to avoid torn reads if a
	// dashboard config-write lands mid-cycle (the locker is reused across
	// the whole codebase via cfg.RLock/RUnlock).
	s.cfg.RLock()
	universe := append([]string(nil), s.cfg.PriceGapDiscoveryUniverse...)
	denylist := append([]string(nil), s.cfg.PriceGapDiscoveryDenylist...)
	threshold := float64(s.cfg.PriceGapDiscoveryThresholdBps)
	depthFloor := float64(s.cfg.PriceGapDiscoveryMinDepthUSDT)
	stalenessSec := s.cfg.PriceGapKlineStalenessSec
	if stalenessSec <= 0 {
		stalenessSec = 90
	}
	s.cfg.RUnlock()

	promoted := s.snapshotPromoted()

	startedAt := now.Unix()
	startedAtNanos := s.nowFunc()
	summary := CycleSummary{
		StartedAt:   startedAt,
		WhyRejected: map[ScanReason]int{},
		Records:     []CycleRecord{},
	}

	cycleAborted := false

UniverseLoop:
	for _, sym := range universe {
		if cycleAborted {
			break UniverseLoop
		}

		listings := s.listingExchanges(sym)
		if len(listings) < 2 {
			_ = s.telemetry.WriteSymbolNoCrossPair(ctx)
			continue
		}

		summary.CandidatesSeen += len(listings) * (len(listings) - 1)

		for _, longID := range listings {
			for _, shortID := range listings {
				if longID == shortID {
					continue
				}
				s.mu.Lock()
				if s.consecErr >= 5 {
					s.mu.Unlock()
					summary.Errors++
					_ = s.telemetry.WriteCycleFailed(ctx)
					cycleAborted = true
					break
				}
				s.mu.Unlock()

				rec := s.scanPair(sym, longID, shortID, denylist, threshold, depthFloor, stalenessSec, now, promoted)
				if rec.WhyRejected == ReasonAccepted {
					summary.Accepted++
				} else {
					summary.Rejected++
					summary.WhyRejected[rec.WhyRejected]++
					if rec.WhyRejected == ReasonSampleError {
						summary.Errors++
					}
				}
				summary.Records = append(summary.Records, rec)
			}
			if cycleAborted {
				break
			}
		}
	}

	completedAt := s.nowFunc()
	summary.CompletedAt = completedAt.Unix()
	summary.DurationMs = completedAt.Sub(startedAtNanos).Milliseconds()
	_ = s.telemetry.WriteCycle(ctx, summary)

	// Phase 12 (D-16) — synchronous auto-promotion. Apply runs in this same
	// goroutine right after the telemetry write so streak counters see the
	// authoritative cycle outcome. promotion is nil when:
	//   - the operator has set PriceGapDiscoveryEnabled=false (cmd/main.go
	//     never constructs a controller in that case — rest of RunCycle is
	//     also gated on the same flag at the top of this function)
	//   - the scanner is under unit test and the test passed nil
	// Pitfall 6: never abort a cycle on a controller hiccup — Apply errors
	// are logged best-effort and the loop continues.
	if s.promotion != nil {
		if err := s.promotion.Apply(ctx, summary); err != nil && s.log != nil {
			s.log.Warn("[phase-12] promotion.Apply failed: %v", err)
		}
	}
}

// scanPair evaluates the 6 hard gates in cheapest-first order. On any
// rejection it emits a CycleRecord with WhyRejected populated and Score=0.
// The PersistenceBars / SpreadBps / DepthScore / FreshnessAgeSec sub-scores
// are populated as far as the gate sequence reached (D-03).
func (s *Scanner) scanPair(
	sym, longID, shortID string,
	denylist []string,
	threshold, depthFloor float64,
	stalenessSec int,
	now time.Time,
	promoted map[string]bool,
) CycleRecord {
	rec := CycleRecord{
		Symbol:      sym,
		LongExch:    longID,
		ShortExch:   shortID,
		Timestamp:   now.Unix(),
		GatesPassed: []string{},
		GatesFailed: []string{},
	}
	rec.AlreadyPromoted = promoted[sym+":"+longID+":"+shortID]

	// Gate 1 — Denylist (cheapest: pure string check).
	if matchesDenylist(sym, longID, denylist) || matchesDenylist(sym, shortID, denylist) {
		rec.WhyRejected = ReasonDenylist
		rec.GatesFailed = append(rec.GatesFailed, "denylist")
		return rec
	}
	rec.GatesPassed = append(rec.GatesPassed, "denylist")

	// Gate 2 — Bybit blackout (wall-clock check).
	if (longID == "bybit" || shortID == "bybit") && inBybitBlackout(now) {
		rec.WhyRejected = ReasonBybitBlackout
		rec.GatesFailed = append(rec.GatesFailed, "bybit_blackout")
		return rec
	}
	rec.GatesPassed = append(rec.GatesPassed, "bybit_blackout")

	// Gate 3 — BBO sample on both legs.
	legL, errL := s.sampleLeg(sym, longID, threshold, now, stalenessSec)
	legR, errR := s.sampleLeg(sym, shortID, threshold, now, stalenessSec)
	if errL != nil || errR != nil || !legL.listingPresent || !legR.listingPresent || !legL.depthPresent || !legR.depthPresent {
		s.mu.Lock()
		s.consecErr++
		s.mu.Unlock()
		rec.WhyRejected = ReasonSampleError
		rec.GatesFailed = append(rec.GatesFailed, "sample")
		return rec
	}
	// Successful read — reset the consecutive-error counter.
	s.mu.Lock()
	s.consecErr = 0
	s.mu.Unlock()
	rec.GatesPassed = append(rec.GatesPassed, "sample")
	rec.FreshnessAgeSec = maxInt(legL.freshnessSec, legR.freshnessSec)
	rec.SpreadBps = computeSpreadBps(legL.mid, legR.mid)

	// Gate 4 — BBO freshness (<= stalenessSec).
	if legL.freshnessSec > stalenessSec || legR.freshnessSec > stalenessSec {
		rec.WhyRejected = ReasonStaleBBO
		rec.GatesFailed = append(rec.GatesFailed, "freshness")
		s.mu.Lock()
		key := sym + ":" + longID + ":" + shortID
		s.bars[key] = &barRing{}
		s.mu.Unlock()
		return rec
	}
	rec.GatesPassed = append(rec.GatesPassed, "freshness")

	// Gate 5 — Depth probe.
	clearable := math.Min(legL.depthClearable, legR.depthClearable)
	if clearable < depthFloor {
		rec.WhyRejected = ReasonInsufficientDepth
		rec.GatesFailed = append(rec.GatesFailed, "depth")
		// DepthScore is the ratio of clearable to floor (capped at 1.0)
		// so dashboards can show "how close to passing".
		rec.DepthScore = clamp01(clearable / safeDiv(depthFloor))
		return rec
	}
	rec.GatesPassed = append(rec.GatesPassed, "depth")
	// Above-floor depth: log-style score in [0,1] saturating at 5x floor.
	rec.DepthScore = clamp01(clearable / (5 * safeDiv(depthFloor)))

	// Gate 6 — ≥4-bar persistence. Push current bar then check.
	key := sym + ":" + longID + ":" + shortID
	s.mu.Lock()
	br := s.bars[key]
	if br == nil {
		br = &barRing{}
		s.bars[key] = br
	}
	br.push(now.Unix()/60, rec.SpreadBps)
	rec.PersistenceBars = barRingPopulatedCount(br)
	persists := barRingExceedThreshold(br, threshold)
	s.mu.Unlock()

	if !persists {
		rec.WhyRejected = ReasonInsufficientPersistence
		rec.GatesFailed = append(rec.GatesFailed, "persistence")
		return rec
	}
	rec.GatesPassed = append(rec.GatesPassed, "persistence")

	// All 6 gates passed → magnitude.
	rec.FundingBpsPerHr = s.fundingAlignment(sym, longID, shortID)
	spreadExcess := math.Abs(rec.SpreadBps) / threshold // ≥1 when persisting
	spreadNorm := clamp01((spreadExcess - 1.0) / 4.0)   // saturates at 5×T
	fundingComponent := math.Max(rec.FundingBpsPerHr/10.0, 0)
	mag := 0.5*spreadNorm + 0.3*rec.DepthScore + 0.2*clamp01(fundingComponent)
	score := int(math.Round(clamp01(mag) * 100))
	if score < 1 {
		// Floor at 1 so callers can distinguish "all gates passed" from
		// "rejected" without consulting WhyRejected (D-01 + D-02).
		score = 1
	}
	rec.Score = score
	rec.WhyRejected = ReasonAccepted
	return rec
}

// listingExchanges returns the exchange IDs (in deterministic insertion
// order across the config's adapter map; we sort lexicographically so the
// test universe-walk asserts on stable counts) where GetBBO(sym) returns
// ok=true.
func (s *Scanner) listingExchanges(sym string) []string {
	keys := make([]string, 0, len(s.exchanges))
	for k := range s.exchanges {
		keys = append(keys, k)
	}
	// Stable lex order so tests can pin pair identities.
	sortStrings(keys)
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		ex := s.exchanges[k]
		if _, ok := ex.GetBBO(sym); ok {
			out = append(out, k)
		}
	}
	return out
}

// sampleLeg reads BBO + depth for one leg and updates the per-leg lastSample
// timestamp used by the freshness gate. If GetBBO returns ok=false the leg
// is reported as not-listing (caller treats as ReasonSampleError given the
// scanner only enters scanPair once listingExchanges has confirmed both
// legs list the symbol — a race here is rare but possible mid-cycle).
func (s *Scanner) sampleLeg(sym, exchID string, threshold float64, now time.Time, stalenessSec int) (legSample, error) {
	out := legSample{}
	ex, ok := s.exchanges[exchID]
	if !ok {
		return out, nil
	}
	bbo, ok := ex.GetBBO(sym)
	if !ok {
		return out, nil
	}
	out.bid = bbo.Bid
	out.ask = bbo.Ask
	out.mid = (bbo.Bid + bbo.Ask) / 2.0
	out.listingPresent = true

	ob, ok := ex.GetDepth(sym)
	if !ok || ob == nil {
		return out, nil
	}
	out.depthPresent = true
	out.depthClearable = clearableUSDT(ob, threshold)

	// Freshness book-keeping: last successful sample wall-clock time per
	// (sym, exch) key. Returns the gap in seconds since the previous tick.
	key := sym + ":" + exchID
	s.mu.Lock()
	prev, hadPrev := s.lastSample[key]
	if hadPrev {
		gap := int(now.Sub(prev).Seconds())
		if gap < 0 {
			gap = 0
		}
		out.freshnessSec = gap
	}
	s.lastSample[key] = now
	s.mu.Unlock()
	_ = stalenessSec // gate evaluated by caller against freshnessSec
	return out, nil
}

// inBybitBlackout returns true for wall-clock minutes 4..5 inclusive of
// second-window 5:00..5:30. The window is the documented Bybit funding
// snapshot blackout (CLAUDE.local.md scan schedule).
func inBybitBlackout(now time.Time) bool {
	m := now.Minute() % 60
	if m == 4 {
		return true
	}
	if m == 5 && now.Second() < 30 {
		return true
	}
	return false
}

// matchesDenylist reports whether sym or sym@exchID matches any entry of
// the denylist (case-insensitive on both sides for operator-typo tolerance).
func matchesDenylist(sym, exchID string, denylist []string) bool {
	tuple := sym + "@" + exchID
	for _, e := range denylist {
		if strings.EqualFold(e, sym) || strings.EqualFold(e, tuple) {
			return true
		}
	}
	return false
}

// snapshotPromoted reads the registry once per cycle and builds a set keyed
// by "sym:long:short:direction" so AlreadyPromoted on each record is a
// constant-time lookup. Read-only — never mutates the registry.
func (s *Scanner) snapshotPromoted() map[string]bool {
	out := map[string]bool{}
	if s.registry == nil {
		return out
	}
	for _, c := range s.registry.List() {
		// Two keys: with and without direction. Plan 04 records carry the
		// flag without a direction discriminator so callers can detect any
		// configured pair regardless of direction policy.
		out[c.Symbol+":"+c.LongExch+":"+c.ShortExch] = true
		out[c.Symbol+":"+c.LongExch+":"+c.ShortExch+":"+c.Direction] = true
	}
	return out
}

// fundingAlignment is the funding-rate component of the magnitude score.
// Plan 04 has no funding-rate cache wired (Plan 05 will add it); for now
// return 0. Sign and magnitude semantics: positive = funding favors trade.
func (s *Scanner) fundingAlignment(sym, longID, shortID string) float64 {
	// Reserved for Plan 05 wiring. Loris ÷8 normalization and sign-detection
	// will live here. Returning 0 makes the funding component a no-op in
	// the magnitude formula until Plan 05.
	_ = sym
	_ = longID
	_ = shortID
	return 0
}

// computeSpreadBps mirrors detector.go's helper for the cross-exchange case
// (long_mid - short_mid) / mid × 10_000. Returned in bps; sign indicates
// direction (positive = long leg cheaper).
//
// NOTE: detector.go declares a helper of the same name; that one is the
// public-package exported helper used by the existing detector flow. To
// avoid shadowing it we use a slightly different signature here and inline
// the math.
func computeSpreadBpsLegs(midL, midR float64) float64 {
	mid := (midL + midR) / 2.0
	if mid <= 0 {
		return 0
	}
	return (midL - midR) / mid * 10_000.0
}

// clearableUSDT estimates how much USDT can be transacted on the leg's
// orderbook before price moves more than `threshold` bps from the midpoint.
// Walks the deeper of bids/asks (whichever side has size) and accumulates
// price × qty until the cumulative price movement exceeds the cap.
//
// This is a conservative single-side estimate; Plan 05 may refine it to
// take both sides into account. For the discovery gate the floor of $1k is
// generous enough that the simple approach suffices.
func clearableUSDT(ob *exchange.Orderbook, thresholdBps float64) float64 {
	if ob == nil {
		return 0
	}
	bids := ob.Bids
	asks := ob.Asks
	if len(bids) == 0 && len(asks) == 0 {
		return 0
	}

	// Prefer the ask side when both populated — buying liquidity is the
	// usual constraint for the long leg of a discovery candidate.
	side := asks
	if len(side) == 0 {
		side = bids
	}
	if len(side) == 0 {
		return 0
	}
	mid := side[0].Price
	if mid <= 0 {
		return 0
	}
	maxDelta := mid * thresholdBps / 10_000.0
	cleared := 0.0
	for _, lvl := range side {
		if math.Abs(lvl.Price-mid) > maxDelta {
			break
		}
		cleared += lvl.Price * lvl.Quantity
	}
	return cleared
}

// clamp01 clips x into [0,1]. Useful for sub-score normalization.
func clamp01(x float64) float64 {
	if x < 0 {
		return 0
	}
	if x > 1 {
		return 1
	}
	return x
}

// safeDiv returns x for x>0 else 1, so callers can divide without a NaN
// when the floor is configured at 0.
func safeDiv(x float64) float64 {
	if x <= 0 {
		return 1
	}
	return x
}

// maxInt is the obvious helper.
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// sortStrings is a tiny in-place insertion sort to avoid a dependency on
// sort.Strings — keeps the import surface minimal in this file.
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1] > s[j]; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}
