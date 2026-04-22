package pricegaptrader

import (
	"fmt"
	"time"

	"arb/internal/models"
)

// execQualityWindow is the rolling-sample size used to evaluate realized-vs-
// modeled slippage. PG-RISK-03 / D-19 fix this at exactly 10 trades.
const execQualityWindow = 10

// execQualityMultiplier — threshold multiplier over mean modeled slippage.
// D-19: disable when mean(realized) > 2 * mean(modeled).
const execQualityMultiplier = 2.0

// recordSlippageAndMaybeDisable appends the closed trade's (realized, modeled)
// slippage sample and evaluates the 10-trade rolling exec-quality rule (D-19).
//
// Rule: after at least execQualityWindow samples, if
//   mean(realized) > execQualityMultiplier * mean(modeled)
// then pg:candidate:disabled:<symbol> is set so subsequent preEntry Gate 1
// blocks with ErrPriceGapCandidateDisabled until human re-enables via
// cmd/pg-admin (future work).
//
// Guards:
//   - Fewer than 10 samples → no-op.
//   - Mean modeled ≤ 0       → no-op (divide-by-zero guard; also prevents the
//     pathological "modeled=0, realized>0 → auto-disable everything" trap).
//   - Redis read error        → logged and returned (fail-open).
func (t *Tracker) recordSlippageAndMaybeDisable(pos *models.PriceGapPosition) {
	candID := pos.Symbol + "_" + pos.LongExchange + "_" + pos.ShortExchange
	sample := models.SlippageSample{
		PositionID: pos.ID,
		Realized:   pos.RealizedSlipBps,
		Modeled:    pos.ModeledSlipBps,
		Timestamp:  time.Now(),
	}
	if err := t.db.AppendSlippageSample(candID, sample); err != nil {
		t.log.Warn("pricegap: AppendSlippageSample %s: %v", candID, err)
		return
	}

	samples, err := t.db.GetSlippageWindow(candID, execQualityWindow)
	if err != nil {
		t.log.Warn("pricegap: GetSlippageWindow %s: %v", candID, err)
		return
	}
	if len(samples) < execQualityWindow {
		return
	}

	var realSum, modSum float64
	for _, s := range samples {
		realSum += s.Realized
		modSum += s.Modeled
	}
	meanReal := realSum / float64(execQualityWindow)
	meanMod := modSum / float64(execQualityWindow)

	// Divide-by-zero + nonsense-data guard. If there's no modeled signal we
	// can't say realized is "too high" relative to expectation.
	if meanMod <= 0 {
		return
	}

	if meanReal > execQualityMultiplier*meanMod {
		reason := fmt.Sprintf("exec_quality_auto_disable: meanReal=%.1f > 2.0*meanMod=%.1f at %s",
			meanReal, meanMod, time.Now().UTC().Format(time.RFC3339))
		if err := t.db.SetCandidateDisabled(pos.Symbol, reason); err != nil {
			t.log.Warn("pricegap: SetCandidateDisabled %s: %v", pos.Symbol, err)
			return
		}
		t.log.Warn("pricegap: AUTO-DISABLED %s (%s)", pos.Symbol, reason)

		// Phase 9 Plan 06 auto-disable hook — push a pg_event + a
		// pg_candidate_update so the UI flips the candidate row to disabled
		// without a full state refetch, and alert the operator via Telegram
		// (gate="exec_quality" is on the Plan 04 allowlist).
		disabledAt := time.Now().Unix()
		t.broadcaster.BroadcastPriceGapEvent(PriceGapEvent{
			Type:   "auto_disable",
			Symbol: pos.Symbol,
			Reason: "exec_quality",
		})
		t.broadcaster.BroadcastPriceGapCandidateUpdate(PriceGapCandidateUpdate{
			Symbol:     pos.Symbol,
			Disabled:   true,
			Reason:     reason,
			DisabledAt: disabledAt,
		})
		t.notifier.NotifyPriceGapRiskBlock(pos.Symbol, "exec_quality", reason)
	}
}
