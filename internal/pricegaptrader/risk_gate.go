package pricegaptrader

import (
	"fmt"
	"strings"

	"arb/internal/config"
	"arb/internal/models"
)

// GateDecision — struct result for logging/telemetry; err non-nil = blocked.
type GateDecision struct {
	Approved bool
	Err      error
	Reason   string // human-readable tag for log + exit_reason stamping
}

// preEntry composes the 8 deterministic gates (gate 0 + 7) in the order
// specified by CONTEXT §D-17 + Phase 14 D-22. Returns first failure; runs no
// side effects.
//
// Gates (in order):
//  0. Duplicate-candidate re-entry lockout (per-candidate slot)
//  1. Exec-quality disable flag (D-19) [reads Redis via PriceGapStore]
//  2. Max concurrent (PG-RISK-04) [reads active set cardinality]
//  3. Per-position notional cap (PG-RISK-05) [pure math]
//  4. Budget remaining (sum requested + this <= PriceGapBudget) [pure math]
//  5. Gate concentration cap 50% (PG-RISK-01) [pure math, conditional on Gate-leg presence]
//  6. Live-capital ramp (Phase 14 PG-LIVE-01) [pure math, conditional on PriceGapLiveCapital]
//  7. Delist/halt/staleness (PG-RISK-02) [t.delist.IsDelisted via DelistChecker + det.StalenessSec]
//
// Phase 14 inserts Gate 6 (ramp) AFTER Gate 5 (concentration); the previous
// Gate 6 (delist+staleness) is now Gate 7. Gate 6 is a no-op when
// PriceGapLiveCapital=false (D-07 paper-mode pass-through).
//
// Order is documented here because any reorder would change blame-attribution in
// logs and exit_reason stamping. TestRiskGate_OrderingInvariant locks D-17.
//
// Delist lookup uses the injected models.DelistChecker interface (D-02
// interface-driven DI). Production passes *discovery.Scanner (which has
// func (s *Scanner) IsDelisted(symbol string) bool — satisfies the
// interface directly). Tests pass a fakeDelistChecker.
//
// Phase 9 Plan 06 — Telegram risk-block hook: on each denial we invoke
// t.notifier.NotifyPriceGapRiskBlock(symbol, gate, detail) using gate names
// that match the Plan 04 allowlist (concentration, max_concurrent,
// kline_stale, delist, budget, exec_quality). Gates outside the allowlist
// (per_position_cap, stale-less-than-threshold) invoke nothing — the
// notifier's allowlist would silently drop unknown gates anyway, so the
// call is elided to make intent explicit at the call site.
func (t *Tracker) preEntry(
	cand models.PriceGapCandidate,
	requestedNotionalUSDT float64,
	det DetectionResult,
	activePositions []*models.PriceGapPosition,
) GateDecision {

	// Gate 0: per-candidate re-entry lockout. A PriceGapCandidate is a slot
	// defined by (symbol, long_exch, short_exch) — one active position per
	// slot. Without this gate, the detector re-fires on the same candidate
	// every tick while a prior position is still open, double-counting
	// budget and compounding capital exposure. Observed in the 2026-04-24
	// UAT attempt: SOONUSDT opened at 21:25:04 then again at 21:25:34.
	// No Telegram alert — this fires on sustained market signal, not an
	// operational event worth paging.
	for _, p := range activePositions {
		if p.Symbol == cand.Symbol &&
			p.LongExchange == cand.LongExch &&
			p.ShortExchange == cand.ShortExch {
			return GateDecision{Err: ErrPriceGapDuplicateCandidate, Reason: "duplicate_candidate"}
		}
	}

	// Gate 1: exec-quality disabled
	disabled, reason, _, err := t.db.IsCandidateDisabled(cand.Symbol)
	if err != nil {
		// Fail-open on Redis error — matches existing project pattern (Phase 03-02)
		t.log.Warn("pricegap: disable-flag read failed, fail-open: %v", err)
	} else if disabled {
		t.notifier.NotifyPriceGapRiskBlock(cand.Symbol, "exec_quality", reason)
		return GateDecision{Err: ErrPriceGapCandidateDisabled, Reason: "exec_quality: " + reason}
	}

	// Gate 2: max concurrent (PG-RISK-04)
	if len(activePositions) >= t.cfg.PriceGapMaxConcurrent {
		t.notifier.NotifyPriceGapRiskBlock(cand.Symbol, "max_concurrent",
			fmt.Sprintf("active=%d limit=%d", len(activePositions), t.cfg.PriceGapMaxConcurrent))
		return GateDecision{Err: ErrPriceGapMaxConcurrent, Reason: "max_concurrent"}
	}

	// Gate 3: per-position notional cap (PG-RISK-05)
	// No Telegram alert: this gate fires on caller-side sizing logic bugs,
	// not on market or concentration signals operators need to act on.
	if requestedNotionalUSDT > cand.MaxPositionUSDT {
		return GateDecision{Err: ErrPriceGapPerPositionCap, Reason: "per_position_cap"}
	}

	// Gate 4: budget (sum of active notionals + this request <= PriceGapBudget)
	var activeNotional float64
	for _, p := range activePositions {
		activeNotional += p.NotionalUSDT
	}
	if activeNotional+requestedNotionalUSDT > t.cfg.PriceGapBudget {
		t.notifier.NotifyPriceGapRiskBlock(cand.Symbol, "budget",
			fmt.Sprintf("active=$%.0f requested=$%.0f cap=$%.0f",
				activeNotional, requestedNotionalUSDT, t.cfg.PriceGapBudget))
		return GateDecision{Err: ErrPriceGapBudgetExceeded, Reason: "budget"}
	}

	// Gate 5: Gate-concentration cap 50% (PG-RISK-01). Only fires when THIS
	// candidate touches Gate.io — otherwise the request doesn't add to the
	// gate bucket, so the cap only constrains existing gate positions
	// which already passed on their own entry.
	if candHasGate(cand) {
		var gateNotional float64
		for _, p := range activePositions {
			if positionHasGate(p) {
				gateNotional += p.NotionalUSDT
			}
		}
		cap := t.cfg.PriceGapBudget * t.cfg.PriceGapGateConcentrationPct
		if gateNotional+requestedNotionalUSDT > cap {
			t.notifier.NotifyPriceGapRiskBlock(cand.Symbol, "concentration",
				fmt.Sprintf("gate=$%.0f requested=$%.0f cap=$%.0f",
					gateNotional, requestedNotionalUSDT, cap))
			return GateDecision{Err: ErrPriceGapGateConcentrationCap, Reason: "concentration"}
		}
	}

	// Gate 6: ramp (Phase 14 — first gate that touches live capital).
	// D-07: NO-OP when PriceGapLiveCapital=false (paper-mode pass-through).
	// D-22: defense in depth — this gate enforces min(stage_size, hard_ceiling)
	//   independently of the Sizer at the call site (sizer.go). Either alone
	//   is sufficient; both must agree.
	// T-14-12 + T-14-07: fail-closed when ramp controller is missing or returns
	//   stage 0 (corruption / pre-bootstrap state).
	if t.cfg.PriceGapLiveCapital {
		if t.ramp == nil {
			t.notifier.NotifyPriceGapRiskBlock(cand.Symbol, "ramp", "ramp controller unavailable")
			return GateDecision{Err: ErrPriceGapRampStateUnavailable, Reason: "ramp"}
		}
		snap := t.ramp.Snapshot()
		stageSize := stageSizeForCfg(snap.CurrentStage, t.cfg)
		if stageSize <= 0 {
			// Stage out of [1,3] → fail-closed.
			t.notifier.NotifyPriceGapRiskBlock(cand.Symbol, "ramp",
				fmt.Sprintf("invalid stage=%d", snap.CurrentStage))
			return GateDecision{Err: ErrPriceGapRampStateUnavailable, Reason: "ramp"}
		}
		cap := stageSize
		if t.cfg.PriceGapHardCeilingUSDT > 0 && t.cfg.PriceGapHardCeilingUSDT < cap {
			// min(stage_size, hard_ceiling).
			cap = t.cfg.PriceGapHardCeilingUSDT
		}
		if requestedNotionalUSDT > cap {
			t.notifier.NotifyPriceGapRiskBlock(cand.Symbol, "ramp",
				fmt.Sprintf("requested=$%.0f stage=%d stage_size=$%.0f ceiling=$%.0f cap=$%.0f",
					requestedNotionalUSDT, snap.CurrentStage, stageSize,
					t.cfg.PriceGapHardCeilingUSDT, cap))
			return GateDecision{Err: ErrPriceGapRampExceeded, Reason: "ramp"}
		}
	}

	// Gate 7: delist / halt / staleness (PG-RISK-02). [Phase 14 renumbered
	// from Gate 6 — the ramp gate above is now Gate 6.]
	// t.delist is the injected models.DelistChecker (Plan 01); production
	// wires *discovery.Scanner, tests wire a fakeDelistChecker.
	if t.delist != nil && t.delist.IsDelisted(cand.Symbol) {
		t.notifier.NotifyPriceGapRiskBlock(cand.Symbol, "delist", "delist/halt detected")
		return GateDecision{Err: ErrPriceGapDelistedLeg, Reason: "delist"}
	}
	if det.StalenessSec >= float64(t.cfg.PriceGapKlineStalenessSec) {
		t.notifier.NotifyPriceGapRiskBlock(cand.Symbol, "kline_stale",
			fmt.Sprintf("stalenessSec=%.1f limit=%d", det.StalenessSec, t.cfg.PriceGapKlineStalenessSec))
		return GateDecision{Err: ErrPriceGapStaleBBO, Reason: "kline_stale"}
	}

	return GateDecision{Approved: true}
}

// PG-DIR-01 verified 2026-04-27: Gate-concentration check (Gate 5 above) is
// already role-blind — both candHasGate and positionHasGate match Gate.io on
// EITHER leg via OR. Bidirectional inverse fires (where Gate may end up on
// the wire-side LONG leg even though configured as the SHORT leg) are
// correctly counted. No code change required for A5; this comment is the
// audit-trail marker.
func candHasGate(c models.PriceGapCandidate) bool {
	return strings.EqualFold(c.LongExch, "gate") || strings.EqualFold(c.ShortExch, "gate")
}

func positionHasGate(p *models.PriceGapPosition) bool {
	return strings.EqualFold(p.LongExchange, "gate") || strings.EqualFold(p.ShortExchange, "gate")
}

// stageSizeForCfg returns the configured per-leg notional for a ramp stage.
// Mirrors Sizer.stageSize (sizer.go) — kept here as a small helper so
// risk_gate.go does not need to depend on the *Sizer concrete type. Returns
// 0 for stages outside [1,3] which the caller treats as fail-closed.
//
// Phase 14 D-22 defense-in-depth: this and Sizer.stageSize must stay in sync.
// If you change one, change both — and consider whether the duplication is
// still worth the decoupling.
func stageSizeForCfg(stage int, cfg *config.Config) float64 {
	if cfg == nil {
		return 0
	}
	switch stage {
	case 1:
		return cfg.PriceGapStage1SizeUSDT
	case 2:
		return cfg.PriceGapStage2SizeUSDT
	case 3:
		return cfg.PriceGapStage3SizeUSDT
	default:
		return 0
	}
}
