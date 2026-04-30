// Package pricegaptrader — Phase 12 PromotionController (PG-DISC-02).
//
// PromotionController is the auto-promotion / auto-demotion controller invoked
// synchronously at the end of Scanner.RunCycle (D-16). It tracks per-candidate
// streak counters across cycles and calls Registry.Add / Registry.Delete (the
// Phase 11 chokepoint) when the streak threshold is met. Promote and demote
// events are surfaced via PromoteEventSink (Redis LIST + WS broadcast — Plan 02
// concrete impl), TelemetrySink (cap-full skip counter), and PromoteNotifier
// (Telegram with per-event cooldown key).
//
// Decisions implemented here:
//   - D-01 promote at >=6 consecutive cycles with Score >= cfg.PriceGapAutoPromoteScore.
//   - D-02 strict consecutive: any non-accepted cycle resets the counter to 0.
//   - D-03 streak storage in-memory; cold restart resets.
//   - D-04 symmetric demote streak: >=6 cycles below threshold or absent.
//   - D-05 active-position guard blocks demote; streak is HELD (not reset);
//     fail-safe → on guard read error treat as blocked.
//   - D-06 auto-demote mandatory (no manual-only mode).
//   - D-08 cap-full silent skip + IncCapFullSkip(symbol) + HOLD streak at 6.
//   - D-09 no auto-displacement.
//   - D-10 PromoteEvent JSON keys (lowercase snake_case).
//   - D-11 PromoteEventSink fans out to Redis LIST + WS hub (Plan 02).
//   - D-13 Telegram cooldown key built from action + tuple + direction.
//   - D-15 module-boundary preserved via narrow interfaces declared here; this
//     file imports only context, errors, fmt, arb/internal/config,
//     arb/internal/models, and arb/pkg/utils. No internal/api,
//     no internal/database, no internal/notify.
//   - D-16 Apply is called synchronously by Scanner.RunCycle (Plan 03 wiring);
//     controller does NOT spawn goroutines.
//   - D-17 the read-only RegistryReader interface stays in registry_reader.go
//     for future read-only consumers (e.g. Phase 16 dashboard render);
//     PromotionController takes the broader RegistryWriter interface
//     declared below — implemented by *Registry in registry.go.
//   - D-18 Direction is pinned to the literal "bidirectional" everywhere this
//     file references it. CycleRecord (scanner.go:89) has NO Direction
//     field; controller MUST NEVER read rec.Direction. Today's scanner
//     is bidirectional-only; a successor phase that introduces
//     long_only/short_only universes adds a CycleRecord.Direction field
//     and replaces D-18.
//
// Concurrency: Apply is called from Scanner.RunCycle's single goroutine.
// streak maps are owned exclusively by the controller and need no mutex
// because they are touched only inside Apply.
package pricegaptrader

import (
	"context"
	"errors"
	"fmt"

	"arb/internal/config"
	"arb/internal/models"
	"arb/pkg/utils"
)

// promoteEventDirection is the literal direction string the controller pins
// onto every Registry.Add / Registry.Delete / PromoteEvent / candidateKey /
// Telegram cooldown key. Per D-18 this is constant for all of Phase 12.
// Mirrors models.PriceGapDirectionBidirectional (kept here as a local literal
// to avoid coupling Plan 01 to a one-string import; the value is identical).
const promoteEventDirection = "bidirectional"

// promoteStreakThreshold and demoteStreakThreshold — D-01 and D-04. The
// streak counter increments by 1 per accepted cycle and fires the
// promote/demote action exactly when it crosses 6 (>= 6, not > 6).
const (
	promoteStreakThreshold = 6
	demoteStreakThreshold  = 6
)

// promoteSourcePromote and promoteSourceDemote — registry audit-trail tags
// (CONTEXT.md line 116). The Registry writes these into pg:registry:audit so
// operators can see WHICH writer mutated cfg.PriceGapCandidates.
const (
	promoteSourcePromote = "scanner-promote"
	promoteSourceDemote  = "scanner-demote"
)

// promoteReasonAbove and promoteReasonBelow — D-10 PromoteEvent.Reason values.
const (
	promoteReasonAbove = "score_threshold_met"
	promoteReasonBelow = "score_below_threshold"
)

// PromoteEvent is the JSON-serialized payload for D-10 (Redis pg:promote:events
// LIST entries) and D-11 (WS hub broadcast). The struct is LOCKED by Plan 02's
// RedisWSPromoteSink and Plan 04's frontend timeline component — do not rename
// fields or change JSON tags.
//
// Direction is always the literal "bidirectional" per D-18. A successor phase
// that introduces long_only/short_only universes will need a corresponding
// CycleRecord.Direction field and a Direction-derivation logic change here.
type PromoteEvent struct {
	TS           int64  `json:"ts"`            // unix milliseconds at fire time
	Action       string `json:"action"`        // "promote" | "demote"
	Symbol       string `json:"symbol"`
	LongExch     string `json:"long_exch"`
	ShortExch    string `json:"short_exch"`
	Direction    string `json:"direction"`     // pinned "bidirectional" (D-18)
	Score        int    `json:"score"`         // last cycle's CycleRecord.Score
	StreakCycles int    `json:"streak_cycles"` // streak length at fire time
	Reason       string `json:"reason"`        // promoteReasonAbove | promoteReasonBelow
}

// PromoteEventSink — D-11 narrow interface. Implemented in Plan 02 by
// *RedisWSPromoteSink (Redis RPush + LTrim 1000 + hub.Broadcast). Tests
// substitute a fake.
type PromoteEventSink interface {
	Emit(ctx context.Context, ev PromoteEvent) error
}

// TelemetrySink — D-08 narrow interface. Implemented in Plan 02 by
// *Telemetry's IncCapFullSkip method (HASH HINCRBY on
// pg:scan:metrics field cap_full_skips:{symbol}). symbol parameter
// per CONTEXT discretion + Plan 02 line 75.
type TelemetrySink interface {
	IncCapFullSkip(ctx context.Context, symbol string) error
}

// PromoteNotifier — D-13/D-14 narrow interface. Implemented in Plan 02 by
// (*notify.TelegramNotifier).NotifyPromoteEvent (built on the existing
// Send + checkCooldownAt(eventKey, now) primitive at telegram.go:206).
//
// The cooldown key is constructed by the implementer using the same field
// values passed here, so the cooldown contract lives in Plan 02. This
// interface intentionally accepts no error — Telegram failure is logged
// best-effort by the notifier impl, never bubbled into Apply.
type PromoteNotifier interface {
	NotifyPromoteEvent(action, symbol, longExch, shortExch, direction string, score, streak int)
}

// ActivePositionChecker — D-05 narrow interface. Implemented in Plan 03 by
// *DBActivePositionChecker against database.GetActivePriceGapPositions.
//
// Returns (true, nil) when an active position matches the candidate's
// (Symbol, LongExch, ShortExch, Direction) tuple — controller MUST skip the
// demote and HOLD the demote streak.
//
// On any read error returns (true, err) — fail-safe to "blocked" so a
// transient Redis hiccup does not delete a candidate that may have an open
// position. Controller treats both (true, nil) and (_, err) the same.
type ActivePositionChecker interface {
	IsActiveForCandidate(c models.PriceGapCandidate) (bool, error)
}

// RegistryWriter — D-17 narrow interface. Implemented by *Registry in
// registry.go. Plan 01 deliberately declares the WRITER interface (not just
// RegistryReader) because the controller is the only legitimate auto-mutation
// writer in Phase 12; the read-only RegistryReader interface in
// registry_reader.go remains in the codebase for read-only consumers
// (e.g. Phase 16 dashboard render path) that must NOT see the writer methods.
//
// List is included so the controller can look up the slice index needed for
// Delete by matching the (Symbol, LongExch, ShortExch, "bidirectional") tuple.
type RegistryWriter interface {
	Add(ctx context.Context, source string, c models.PriceGapCandidate) error
	Delete(ctx context.Context, source string, idx int) error
	List() []models.PriceGapCandidate
}

// nowFunc is the injectable clock source for unit tests. Real callers use
// time.Now via the default constructor; tests inject a fake to control TS.
type nowFunc func() int64 // unix milliseconds

// PromotionController owns per-candidate streak counters and dispatches
// promote/demote actions through the Phase 11 Registry chokepoint. All
// dependencies are interfaces (D-15) so unit tests can substitute fakes
// without touching Redis, Telegram, or *Registry's persistence path.
type PromotionController struct {
	cfg       *config.Config
	registry  RegistryWriter
	guard     ActivePositionChecker
	sink      PromoteEventSink
	notifier  PromoteNotifier
	telemetry TelemetrySink
	log       *utils.Logger
	now       nowFunc

	// promoteStreaks — keyed by candidateKey(symbol, long, short, "bidirectional").
	// Incremented by 1 per accepted cycle (Score >= threshold AND in summary.Records).
	// Reset to 0 on miss/below-threshold cycle (D-02).
	// HELD at threshold on cap-full / dedupe / non-fatal skip (D-08).
	promoteStreaks map[string]int

	// demoteStreaks — keyed by candidateKey. Incremented when an
	// already-promoted candidate is missing from summary.Records OR has
	// Score < threshold (D-04). HELD on guard-blocked demote (D-05).
	demoteStreaks map[string]int
}

// NewPromotionController wires the controller. log MAY be nil (the controller
// guards every log call). All other dependencies MUST be non-nil; a nil
// dependency is a programmer error and would crash on first use, so the
// constructor returns an error rather than panicking later.
func NewPromotionController(
	cfg *config.Config,
	registry RegistryWriter,
	guard ActivePositionChecker,
	sink PromoteEventSink,
	notifier PromoteNotifier,
	telemetry TelemetrySink,
	log *utils.Logger,
) (*PromotionController, error) {
	if cfg == nil {
		return nil, errors.New("promotion: cfg is nil")
	}
	if registry == nil {
		return nil, errors.New("promotion: registry is nil")
	}
	if guard == nil {
		return nil, errors.New("promotion: guard is nil")
	}
	if sink == nil {
		return nil, errors.New("promotion: sink is nil")
	}
	if notifier == nil {
		return nil, errors.New("promotion: notifier is nil")
	}
	if telemetry == nil {
		return nil, errors.New("promotion: telemetry is nil")
	}
	return &PromotionController{
		cfg:            cfg,
		registry:       registry,
		guard:          guard,
		sink:           sink,
		notifier:       notifier,
		telemetry:      telemetry,
		log:            log,
		now:            defaultNowMs,
		promoteStreaks: make(map[string]int),
		demoteStreaks:  make(map[string]int),
	}, nil
}

// candidateKey builds the in-memory streak counter key. The fourth element
// is the pinned "bidirectional" literal per D-18 — this function MUST NOT
// accept a Direction parameter from a CycleRecord, because CycleRecord has
// no Direction field. Plan 01 audit fix: callers pass promoteEventDirection
// directly.
func candidateKey(symbol, longExch, shortExch, direction string) string {
	return symbol + "|" + longExch + "|" + shortExch + "|" + direction
}

// Apply is the synchronous controller entrypoint invoked by Scanner.RunCycle
// at the end of every cycle (D-16). It walks summary.Records, classifies each
// (symbol, long, short) tuple as accepted-above-threshold or not, updates the
// promote/demote streak maps, and fires Registry.Add / Registry.Delete when
// thresholds are crossed.
//
// Errors returned by individual steps (sink.Emit, telemetry.IncCapFullSkip,
// registry.Add for non-cap/non-dedupe reasons) are LOGGED and the cycle
// continues — Apply returns nil for transient failures. Apply returns a
// non-nil error only if a programmer-error condition is detected (e.g. cfg
// snapshot fails). Pitfall 6: never abort the scanner cycle on a controller
// hiccup.
func (p *PromotionController) Apply(ctx context.Context, summary CycleSummary) error {
	// Snapshot config under cfg.RLock so a dashboard config write mid-cycle
	// does not produce torn reads of PriceGapAutoPromoteScore. Mirrors the
	// scanner.go RLock/RUnlock pattern.
	p.cfg.RLock()
	threshold := p.cfg.PriceGapAutoPromoteScore
	maxCands := p.cfg.PriceGapMaxCandidates
	p.cfg.RUnlock()

	// 1. Build the accepted set: candidates in this cycle that BOTH appear in
	//    summary.Records AND have Score >= threshold. Per D-18 Direction is
	//    derived from the pinned literal, NOT from rec.Direction.
	accepted := make(map[string]CycleRecord, len(summary.Records))
	for _, rec := range summary.Records {
		// D-02: only fully-accepted records count. WhyRejected != "" means
		// the record is in summary.Records BUT was rejected by a gate;
		// such records should not increment the streak.
		if rec.WhyRejected != ReasonAccepted {
			continue
		}
		if rec.Score < threshold {
			continue
		}
		key := candidateKey(rec.Symbol, rec.LongExch, rec.ShortExch, promoteEventDirection)
		accepted[key] = rec
	}

	// 2. Walk the accepted set: increment promote streaks; reset demote
	//    streaks (a candidate that's accepted-above-threshold cannot be on a
	//    demote trajectory). Promote when threshold met AND not already
	//    promoted.
	currentList := p.registry.List()
	currentTuples := make(map[string]int, len(currentList)) // candidateKey → idx in cfg.PriceGapCandidates
	for idx, c := range currentList {
		k := candidateKey(c.Symbol, c.LongExch, c.ShortExch, promoteEventDirection)
		// Only index candidates whose Direction is "bidirectional" (or empty,
		// which Phase 11 normalizes to "pinned" — controller does not own
		// pinned candidates so they are NOT in our streak universe). Compare
		// against the pinned literal to avoid coupling.
		if c.Direction == promoteEventDirection {
			currentTuples[k] = idx
		}
	}

	// First pass: reset promote streaks for any candidate that is in the
	// universe but did NOT make the accepted set this cycle (D-02 strict
	// consecutive: any miss resets the counter to 0). We only reset, not
	// delete, so the absence-test below for cycle N+1 still works.
	for key := range p.promoteStreaks {
		if _, ok := accepted[key]; !ok {
			delete(p.promoteStreaks, key)
		}
	}

	for key, rec := range accepted {
		// Reset demote streak — accepted means not on a demote trajectory.
		delete(p.demoteStreaks, key)

		// Increment promote streak.
		p.promoteStreaks[key]++

		// Fire promote when threshold reached AND not already in registry.
		if _, alreadyPromoted := currentTuples[key]; alreadyPromoted {
			// Already promoted; nothing to do. Streak increment is benign
			// (it cannot overflow within a session) and would be reset on
			// the next miss anyway.
			continue
		}

		if p.promoteStreaks[key] < promoteStreakThreshold {
			continue
		}

		// Build the candidate to add. D-18 literal direction.
		cand := models.PriceGapCandidate{
			Symbol:    rec.Symbol,
			LongExch:  rec.LongExch,
			ShortExch: rec.ShortExch,
			Direction: promoteEventDirection,
			// ThresholdBps / MaxPositionUSDT / ModeledSlippageBps fall back
			// to the operator-curated defaults persisted in config.json by
			// the dashboard "Add candidate" path — Phase 12 does NOT auto-set
			// trade economics; that's a Phase 14 capital-ramp concern. The
			// zero values here are normalized by the existing config.Load
			// validators on the next reload, and are immediately overridable
			// from the dashboard.
		}

		// Cap-full guard (D-08): if the registry currently holds the cap, the
		// controller skips the Add silently, increments the cap_full_skips
		// HASH counter, and HOLDS the streak at threshold so the next cycle
		// retries the moment a slot frees.
		if len(currentList) >= maxCands {
			if err := p.telemetry.IncCapFullSkip(ctx, rec.Symbol); err != nil && p.log != nil {
				p.log.Warn("[phase-12] IncCapFullSkip(%s) failed: %v", rec.Symbol, err)
			}
			// HOLD streak at threshold (do not reset; do not increment past).
			if p.promoteStreaks[key] > promoteStreakThreshold {
				p.promoteStreaks[key] = promoteStreakThreshold
			}
			continue
		}

		// Add via the chokepoint. ErrCapExceeded covers a race where another
		// writer (dashboard / pg-admin) added a candidate between our List()
		// snapshot and Add — same handling as the cap-full guard. ErrDuplicate
		// covers a race where the same controller instance double-fires
		// (should not happen because Apply is single-goroutine). Any other
		// error is logged and the streak is HELD at threshold so retry on
		// next cycle.
		if err := p.registry.Add(ctx, promoteSourcePromote, cand); err != nil {
			switch {
			case errors.Is(err, ErrCapExceeded):
				if e := p.telemetry.IncCapFullSkip(ctx, rec.Symbol); e != nil && p.log != nil {
					p.log.Warn("[phase-12] IncCapFullSkip(%s) failed: %v", rec.Symbol, e)
				}
				if p.promoteStreaks[key] > promoteStreakThreshold {
					p.promoteStreaks[key] = promoteStreakThreshold
				}
			case errors.Is(err, ErrDuplicateCandidate):
				// Defensive: should never happen because the controller is
				// the only source of "scanner-promote" tuples and our List()
				// snapshot already detected existing tuples. Hold the streak
				// so we do not loop on the same candidate.
				if p.promoteStreaks[key] > promoteStreakThreshold {
					p.promoteStreaks[key] = promoteStreakThreshold
				}
				if p.log != nil {
					p.log.Warn("[phase-12] unexpected duplicate on promote %s: %v", key, err)
				}
			default:
				if p.log != nil {
					p.log.Warn("[phase-12] registry.Add(%s) failed: %v", key, err)
				}
				if p.promoteStreaks[key] > promoteStreakThreshold {
					p.promoteStreaks[key] = promoteStreakThreshold
				}
			}
			continue
		}

		// Add succeeded → emit event, fire Telegram, reset streak. The streak
		// reset prevents an immediate second promote if the same candidate
		// somehow re-appears at threshold next cycle (it would have to demote
		// first, then re-promote on a fresh streak).
		ev := PromoteEvent{
			TS:           p.now(),
			Action:       "promote",
			Symbol:       rec.Symbol,
			LongExch:     rec.LongExch,
			ShortExch:    rec.ShortExch,
			Direction:    promoteEventDirection,
			Score:        rec.Score,
			StreakCycles: p.promoteStreaks[key],
			Reason:       promoteReasonAbove,
		}
		p.fireEvent(ctx, ev)
		// Update the local tuple index map so subsequent demote logic sees
		// the new candidate.
		currentList = append(currentList, cand)
		currentTuples[key] = len(currentList) - 1
		// Reset promote streak — candidate is now in the registry.
		delete(p.promoteStreaks, key)
	}

	// 3. Walk the currently-promoted candidates: any that are NOT in the
	//    accepted set this cycle either fell below threshold or are absent
	//    entirely. Increment their demote streak; reset their promote streak.
	//    Fire demote when threshold met AND guard does not block.
	for key, idx := range currentTuples {
		if _, isAccepted := accepted[key]; isAccepted {
			// Still accepted-above-threshold; demote streak already deleted
			// in the accepted-walk above. Reset just to be safe.
			delete(p.demoteStreaks, key)
			continue
		}

		// Reset promote streak — candidate is currently below the bar.
		delete(p.promoteStreaks, key)

		// Increment demote streak.
		p.demoteStreaks[key]++

		if p.demoteStreaks[key] < demoteStreakThreshold {
			continue
		}

		// Look up the candidate to consult the active-position guard. We
		// already have the idx from currentTuples; defensively re-read to
		// guard against an out-of-range write (shouldn't happen but free
		// defense).
		if idx < 0 || idx >= len(currentList) {
			if p.log != nil {
				p.log.Warn("[phase-12] demote idx %d out of range for %s", idx, key)
			}
			continue
		}
		cand := currentList[idx]

		// D-05 active-position guard. Both (true, nil) and (_, err) → BLOCK.
		blocked, err := p.guard.IsActiveForCandidate(cand)
		if err != nil {
			if p.log != nil {
				p.log.Warn("[phase-12] active-position guard read failed for %s (treating as blocked): %v", key, err)
			}
			// HOLD demote streak at threshold; retry next cycle.
			if p.demoteStreaks[key] > demoteStreakThreshold {
				p.demoteStreaks[key] = demoteStreakThreshold
			}
			continue
		}
		if blocked {
			// HOLD demote streak; no event; no Telegram (D-05).
			if p.demoteStreaks[key] > demoteStreakThreshold {
				p.demoteStreaks[key] = demoteStreakThreshold
			}
			continue
		}

		// Last-known score for the candidate. Because the candidate is NOT
		// in summary.Records (or is below threshold), the most useful score
		// is 0 (absent) or the rejected record's score. Walk records for a
		// matching tuple to find the actual score for the event payload.
		lastScore := 0
		for _, rec := range summary.Records {
			if rec.Symbol == cand.Symbol &&
				rec.LongExch == cand.LongExch &&
				rec.ShortExch == cand.ShortExch {
				lastScore = rec.Score
				break
			}
		}

		if err := p.registry.Delete(ctx, promoteSourceDemote, idx); err != nil {
			if errors.Is(err, ErrIndexOutOfRange) {
				// Race: someone else (dashboard/pg-admin) deleted the
				// candidate between List() and Delete. Reset both streaks —
				// the candidate is gone.
				delete(p.demoteStreaks, key)
				delete(p.promoteStreaks, key)
				continue
			}
			if p.log != nil {
				p.log.Warn("[phase-12] registry.Delete(%s) failed: %v", key, err)
			}
			// HOLD streak; retry next cycle.
			if p.demoteStreaks[key] > demoteStreakThreshold {
				p.demoteStreaks[key] = demoteStreakThreshold
			}
			continue
		}

		// Delete succeeded → emit event, fire Telegram, reset both streaks
		// (candidate is no longer in registry; streaks would be irrelevant
		// even if they persisted).
		ev := PromoteEvent{
			TS:           p.now(),
			Action:       "demote",
			Symbol:       cand.Symbol,
			LongExch:     cand.LongExch,
			ShortExch:    cand.ShortExch,
			Direction:    promoteEventDirection,
			Score:        lastScore,
			StreakCycles: p.demoteStreaks[key],
			Reason:       promoteReasonBelow,
		}
		p.fireEvent(ctx, ev)
		delete(p.demoteStreaks, key)
		delete(p.promoteStreaks, key)
	}

	return nil
}

// fireEvent emits the PromoteEvent to the sink AND fires the Telegram
// notifier. Sink errors are logged best-effort; notifier is intentionally
// fire-and-forget (interface returns no error). The event timestamp is
// captured by the caller before this function is called so Apply controls
// monotonicity within a single cycle.
func (p *PromotionController) fireEvent(ctx context.Context, ev PromoteEvent) {
	if err := p.sink.Emit(ctx, ev); err != nil && p.log != nil {
		p.log.Warn("[phase-12] sink.Emit(%s %s) failed: %v", ev.Action, ev.Symbol, err)
	}
	p.notifier.NotifyPromoteEvent(
		ev.Action,
		ev.Symbol,
		ev.LongExch,
		ev.ShortExch,
		ev.Direction,
		ev.Score,
		ev.StreakCycles,
	)
}

// defaultNowMs returns the current unix-millisecond timestamp. Tests inject
// a fake via the unexported now field on PromotionController.
//
// Plan 01 keeps the time import out by routing through a package-level
// variable defaultNowMsImpl. Production callers (Plan 03) call SetNowFunc
// with the real time.Now-based source after construction.
func defaultNowMs() int64 {
	return defaultNowMsImpl()
}

// defaultNowMsImpl is split out so promotion_test.go can swap it via the
// nowFunc injection point without import-time fanout. Default returns 0;
// Plan 03 overrides via SetNowFunc on bootstrap.
var defaultNowMsImpl = func() int64 {
	return 0
}

// SetNowFunc replaces the timestamp source. Plan 03 calls this in
// cmd/main.go bootstrap with `func() int64 { return time.Now().UnixMilli() }`.
// Tests use this to deterministically set TS values in PromoteEvent
// fixtures.
func (p *PromotionController) SetNowFunc(fn func() int64) {
	if fn == nil {
		return
	}
	p.now = fn
}

// String — debug diagnostic used inside log.Warn calls above.
func (p *PromotionController) String() string {
	return fmt.Sprintf("PromotionController(promote_streaks=%d, demote_streaks=%d)",
		len(p.promoteStreaks), len(p.demoteStreaks))
}
