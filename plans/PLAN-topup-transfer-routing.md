# Top-Up Transfer Routing Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Route rebalance Pass-1 approvals that the risk simulator only approved by borrowing from `cache.TransferablePerExchange` into case (b) rescue-candidate path so a real transfer gets scheduled, instead of silently dropping them into case (a) where no transfer fires and entry-scan then rejects on real balance.

**Architecture:** Thread a new `TopUpApplied map[string]float64` field through `models.RiskApproval`. The risk simulator populates it whenever cross-exchange top-up inflated `pair.bal.Available`. Rebalance Pass-1 switches on `len(approval.TopUpApplied) > 0` to treat approved-with-topup identically to `RejectionKindCapital` rescue candidates. Executor path (`Approve` / `ApproveWithReserved`) never populates the field, so non-simulator callers are unaffected. Post-transfer replay (`postTransferReplayFilter`) already clears `TransferablePerExchange` in its replay cache, so after rescue, the replay correctly refuses if real balance still cannot cover.

**Tech Stack:** Go 1.26, `internal/models`, `internal/risk`, `internal/engine`. Go stdlib `testing` package.

---

**Version:** v2
**Date:** 2026-04-22
**Status:** REVIEWING

## Context — Observed incident

2026-04-22 11:35–11:45 UTC, METUSDT gateio/bitget:

- 11:35:01 rebalance snapshot: `gateio futures=76.27 usageRatio=0.7968 maxTransferOut=0.00`, `bitget futures=53.95 usageRatio=0.7945 maxTransferOut=53.95`
- 11:35:02 allocator transferable pool: `bitget:291.74 gateio:292.07`
- 11:35:03 `approval dryRun METUSDT: added spot 0.00 to bitget (avail now=53.67)` — then simulator applied cross-exchange top-up silently
- 11:35:03 `rebalance: pass1 case(a) approved METUSDT gateio/bitget reserved long=199.93 short=199.71`
- 11:35 → 11:45: **no transfer/withdraw log for METUSDT or gateio/bitget**
- 11:45:01 real `Approve` saw `gateio=73.08 bitget=50.38` and rejected with `post-trade margin ratio would reach 0.87 on gateio (L4 threshold: 0.80)`

Codex independent review (dispatch-mcp task `baa9d706`) confirmed bug exists.

## File Structure

- **Modify** `internal/models/interfaces.go` — add `TopUpApplied` field to `RiskApproval` struct.
- **Modify** `internal/risk/manager.go` — track top-up per leg inside `approveInternal` dryRun branch; attach to final approval at the single terminal `return &models.RiskApproval{...}` site (line 756).
- **Modify** `internal/engine/rebalance.go` — change Pass-1 switch at line 210 so approved-with-topup falls through to the same rescue-candidate path as `RejectionKindCapital`. Update `hasPricedApprovalFields` if needed for approved-path priced fields.
- **Modify** `internal/engine/allocator.go` — verify `SimulateApprovalForPair` callers at line 430 (pool allocator reval) and line 568 (sequential rank-first) handle `TopUpApplied` correctly or are documented as exempt.
- **Create** `internal/risk/manager_topup_test.go` — regression test for simulator top-up metadata.
- **Create** `internal/engine/rebalance_topup_test.go` — regression test for Pass-1 routing.

---

## Task 1: Add `TopUpApplied` field to `RiskApproval`

**Files:**
- Modify: `internal/models/interfaces.go:83-97`

- [ ] **Step 1: Add field to struct**

Modify `internal/models/interfaces.go:83-97` from:

```go
// RiskApproval holds the result of a RiskChecker.Approve call.
type RiskApproval struct {
    Approved          bool          `json:"approved"`
    Kind              RejectionKind `json:"kind"`                  // rejection classification
    Size              float64       `json:"size"`
    Reason            string        `json:"reason"`
    Price             float64       `json:"price"`
    GapBPS            float64       `json:"gap_bps"`               // cross-exchange price gap at approval time
    // RequiredMargin is max(LongMarginNeeded, ShortMarginNeeded) with the
    // safety buffer already applied (for reservation purposes).
    // IMPORTANT: this value already includes the safety buffer — do NOT
    // multiply by MarginSafetyMultiplier again at call sites.
    RequiredMargin    float64       `json:"required_margin"`       // max(long,short) margin with safety buffer (for reservation)
    LongMarginNeeded  float64       `json:"long_margin_needed"`    // per-leg margin needed on long exchange (with buffer)
    ShortMarginNeeded float64       `json:"short_margin_needed"`   // per-leg margin needed on short exchange (with buffer)
}
```

to:

```go
// RiskApproval holds the result of a RiskChecker.Approve call.
type RiskApproval struct {
    Approved          bool          `json:"approved"`
    Kind              RejectionKind `json:"kind"`                  // rejection classification
    Size              float64       `json:"size"`
    Reason            string        `json:"reason"`
    Price             float64       `json:"price"`
    GapBPS            float64       `json:"gap_bps"`               // cross-exchange price gap at approval time
    // RequiredMargin is max(LongMarginNeeded, ShortMarginNeeded) with the
    // safety buffer already applied (for reservation purposes).
    // IMPORTANT: this value already includes the safety buffer — do NOT
    // multiply by MarginSafetyMultiplier again at call sites.
    RequiredMargin    float64       `json:"required_margin"`       // max(long,short) margin with safety buffer (for reservation)
    LongMarginNeeded  float64       `json:"long_margin_needed"`    // per-leg margin needed on long exchange (with buffer)
    ShortMarginNeeded float64       `json:"short_margin_needed"`   // per-leg margin needed on short exchange (with buffer)
    // TopUpApplied is populated only by SimulateApproval / SimulateApprovalForPair.
    // Key = exchange name, Value = USDT amount the simulator borrowed from
    // cache.TransferablePerExchange to inflate pair.bal.Available so that margin
    // checks passed. A non-empty map means the approval is feasible ONLY IF a
    // real cross-exchange transfer of the same size lands before the executor
    // runs. Rebalance Pass-1 MUST route such approvals into the rescue-candidate
    // path (case b) instead of case (a), otherwise no transfer is scheduled and
    // the executor will reject on real balance.
    // Real Approve / ApproveWithReserved (dryRun=false) never populates this.
    TopUpApplied      map[string]float64 `json:"top_up_applied,omitempty"`
}
```

- [ ] **Step 2: Compile check**

Run: `go build ./internal/models/`
Expected: success, no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/models/interfaces.go
git commit -m "models: add RiskApproval.TopUpApplied for simulator rescue routing"
```

---

## Task 2: Populate `TopUpApplied` in simulator

**Files:**
- Modify: `internal/risk/manager.go:334-410` (dryRun top-up block)
- Modify: `internal/risk/manager.go:756` (final approval return)
- Test: `internal/risk/manager_topup_test.go`

- [ ] **Step 1: Write failing test**

Create `internal/risk/manager_topup_test.go`:

```go
package risk

import (
    "testing"

    "arb/internal/models"
    "arb/pkg/exchange"
)

// TestSimulator_TopUpAppliedPopulated verifies that when cross-exchange top-up
// is used to inflate balance so a dryRun approval passes, TopUpApplied carries
// the per-exchange amount for the rebalance router to detect.
func TestSimulator_TopUpAppliedPopulated(t *testing.T) {
    // Setup: two exchanges each with futures=50 USDT (insufficient for size
    // needing ~150 margin), spot=0, cross-exchange transferable pool provides
    // 200 per leg. Simulator should approve via top-up AND record the top-up.
    m := newTestManagerWithBalances(t, map[string]*exchange.Balance{
        "gateio":  {Available: 50, Total: 50},
        "bitget":  {Available: 50, Total: 50},
    })
    cache := &PrefetchCache{
        Balances: map[string]*exchange.Balance{
            "gateio":  {Available: 50, Total: 50},
            "bitget":  {Available: 50, Total: 50},
        },
        Orderbooks: map[string]*exchange.Orderbook{
            "gateio:METUSDT": newTestOrderbook(0.188),
            "bitget:METUSDT": newTestOrderbook(0.188),
        },
        TransferablePerExchange: map[string]float64{
            "gateio": 200,
            "bitget": 200,
        },
        ActivePositions: nil,
    }
    opp := models.Opportunity{
        Symbol:        "METUSDT",
        LongExchange:  "gateio",
        ShortExchange: "bitget",
        Spread:        3.87,
        IntervalHours: 4,
    }

    approval, err := m.SimulateApprovalForPair(opp, "gateio", "bitget", nil, nil, cache)
    if err != nil {
        t.Fatalf("SimulateApprovalForPair: %v", err)
    }
    if !approval.Approved {
        t.Fatalf("expected approved with top-up, got rejected: %s", approval.Reason)
    }
    if len(approval.TopUpApplied) == 0 {
        t.Fatal("TopUpApplied empty — rebalance router cannot detect top-up dependency")
    }
    if approval.TopUpApplied["gateio"] <= 0 {
        t.Errorf("TopUpApplied[gateio] = %.2f, expected >0", approval.TopUpApplied["gateio"])
    }
    if approval.TopUpApplied["bitget"] <= 0 {
        t.Errorf("TopUpApplied[bitget] = %.2f, expected >0", approval.TopUpApplied["bitget"])
    }
}

// TestSimulator_TopUpAppliedEmptyWhenSufficient verifies that when real balance
// is sufficient, TopUpApplied stays empty (approval is honest).
func TestSimulator_TopUpAppliedEmptyWhenSufficient(t *testing.T) {
    m := newTestManagerWithBalances(t, map[string]*exchange.Balance{
        "gateio": {Available: 500, Total: 500},
        "bitget": {Available: 500, Total: 500},
    })
    cache := &PrefetchCache{
        Balances: map[string]*exchange.Balance{
            "gateio": {Available: 500, Total: 500},
            "bitget": {Available: 500, Total: 500},
        },
        Orderbooks: map[string]*exchange.Orderbook{
            "gateio:METUSDT": newTestOrderbook(0.188),
            "bitget:METUSDT": newTestOrderbook(0.188),
        },
        TransferablePerExchange: map[string]float64{
            "gateio": 200,
            "bitget": 200,
        },
        ActivePositions: nil,
    }
    opp := models.Opportunity{
        Symbol:        "METUSDT",
        LongExchange:  "gateio",
        ShortExchange: "bitget",
        Spread:        3.87,
        IntervalHours: 4,
    }

    approval, err := m.SimulateApprovalForPair(opp, "gateio", "bitget", nil, nil, cache)
    if err != nil {
        t.Fatalf("SimulateApprovalForPair: %v", err)
    }
    if !approval.Approved {
        t.Fatalf("expected approved (real balance sufficient): %s", approval.Reason)
    }
    if len(approval.TopUpApplied) != 0 {
        t.Errorf("TopUpApplied non-empty when real balance sufficient: %v", approval.TopUpApplied)
    }
}

// TestExecutor_TopUpAppliedNeverPopulated verifies real Approve (dryRun=false)
// never populates TopUpApplied, to keep non-simulator callers insulated.
func TestExecutor_TopUpAppliedNeverPopulated(t *testing.T) {
    m := newTestManagerWithBalances(t, map[string]*exchange.Balance{
        "gateio": {Available: 500, Total: 500},
        "bitget": {Available: 500, Total: 500},
    })
    opp := models.Opportunity{
        Symbol:        "METUSDT",
        LongExchange:  "gateio",
        ShortExchange: "bitget",
        Spread:        3.87,
        IntervalHours: 4,
    }

    approval, err := m.Approve(opp)
    if err != nil {
        t.Fatalf("Approve: %v", err)
    }
    if len(approval.TopUpApplied) != 0 {
        t.Errorf("Executor path populated TopUpApplied: %v", approval.TopUpApplied)
    }
}
```

Note: `newTestManagerWithBalances` and `newTestOrderbook` — reuse existing test helpers in the `risk` package if available; otherwise add minimal fakes in a new `manager_topup_helpers_test.go` that stub `exchange.Exchange` with an in-memory `Balance` and `Orderbook` source. Use the same pattern `internal/risk/manager_test.go` uses today.

- [ ] **Step 2: Run tests, verify they fail**

Run: `go test ./internal/risk/ -run 'Test(Simulator|Executor)_TopUpApplied' -v`
Expected: FAIL with one of:
- `approval.TopUpApplied empty` (before populating the field)
- compilation error if helpers missing

- [ ] **Step 3: Implement — track top-up inside dryRun block**

Modify `internal/risk/manager.go:334-410`. Codex-supplied verbatim BEFORE/AFTER:

BEFORE:

```go
if dryRun && needed > 0 {
	longClone := *longBal
	shortClone := *shortBal
	longBal = &longClone
	shortBal = &shortClone
	for _, pair := range []struct {
		name string
		exch exchange.Exchange
		bal  *exchange.Balance
	}{
		{opp.LongExchange, longExch, longBal},
		{opp.ShortExchange, shortExch, shortBal},
	} {
		bufferedNeed := needed * m.cfg.MarginSafetyMultiplier
		// For split-account exchanges, unconditionally add spot balance so
		// dryRun sees the same avail as rebalanceAvailable (futures+spot).
		// Same-exchange long+short is structurally impossible in perp-perp,
		// so per-leg spot augmentation cannot double-count.
		isUnified := false
		if uc, ok := pair.exch.(interface{ IsUnified() bool }); ok && uc.IsUnified() {
			isUnified = true
		}
		if !isUnified {
			if spotBal, err := pair.exch.GetSpotBalance(); err == nil && spotBal.Available > 0 {
				pair.bal.Available += spotBal.Available
				pair.bal.Total += spotBal.Available
				m.log.Debug("approval dryRun %s: added spot %.2f to %s (avail now=%.2f)", opp.Symbol, spotBal.Available, pair.name, pair.bal.Available)
			}
		}
		// Add cross-exchange transferable surplus (allocator planning only).
		// Trigger when EITHER margin buffer is insufficient OR post-trade
		// ratio would exceed L4. Only inflate by the deficit needed.
		if cache != nil && cache.TransferablePerExchange != nil {
			// Check if post-trade ratio would exceed L4 even with sufficient margin buffer.
			margin := needed
			wouldExceedL4 := false
			if pair.bal.Total > 0 {
				postAvail := pair.bal.Available - margin
				if postAvail < 0 {
					postAvail = 0
				}
				postRatio := 1 - postAvail/pair.bal.Total
				wouldExceedL4 = postRatio >= m.cfg.MarginL4Threshold
			}

			if pair.bal.Available < bufferedNeed || wouldExceedL4 {
				if topUp, ok := cache.TransferablePerExchange[pair.name]; ok && topUp > 0 {
					// Compute margin buffer deficit.
					deficit := bufferedNeed - pair.bal.Available
					if deficit < 0 {
						deficit = 0
					}

					// Also compute the post-trade ratio deficit: how much extra
					// is needed so that post-trade ratio stays below L4.
					targetRatio := m.cfg.MarginL4Threshold - 0.005 // marginEpsilon: avoid hitting L4 boundary
					freeTarget := 1.0 - targetRatio
					if freeTarget > 0 && pair.bal.Total > 0 {
						ratioDeficit := (freeTarget*pair.bal.Total - pair.bal.Available + margin) / targetRatio
						if ratioDeficit > deficit {
							deficit = ratioDeficit
						}
					}

					actualTopUp := deficit
					if actualTopUp > topUp {
						actualTopUp = topUp
					}
					if actualTopUp > 0 {
						pair.bal.Available += actualTopUp
						pair.bal.Total += actualTopUp
					}
				}
			}
		}
	}
}
```

AFTER:

```go
var approvalTopUp map[string]float64
if dryRun && needed > 0 {
	longClone := *longBal
	shortClone := *shortBal
	longBal = &longClone
	shortBal = &shortClone
	approvalTopUp = map[string]float64{}
	for _, pair := range []struct {
		name string
		exch exchange.Exchange
		bal  *exchange.Balance
	}{
		{opp.LongExchange, longExch, longBal},
		{opp.ShortExchange, shortExch, shortBal},
	} {
		bufferedNeed := needed * m.cfg.MarginSafetyMultiplier
		// For split-account exchanges, unconditionally add spot balance so
		// dryRun sees the same avail as rebalanceAvailable (futures+spot).
		// Same-exchange long+short is structurally impossible in perp-perp,
		// so per-leg spot augmentation cannot double-count.
		isUnified := false
		if uc, ok := pair.exch.(interface{ IsUnified() bool }); ok && uc.IsUnified() {
			isUnified = true
		}
		if !isUnified {
			if spotBal, err := pair.exch.GetSpotBalance(); err == nil && spotBal.Available > 0 {
				pair.bal.Available += spotBal.Available
				pair.bal.Total += spotBal.Available
				m.log.Debug("approval dryRun %s: added spot %.2f to %s (avail now=%.2f)", opp.Symbol, spotBal.Available, pair.name, pair.bal.Available)
			}
		}
		// Add cross-exchange transferable surplus (allocator planning only).
		// Trigger when EITHER margin buffer is insufficient OR post-trade
		// ratio would exceed L4. Only inflate by the deficit needed.
		if cache != nil && cache.TransferablePerExchange != nil {
			margin := needed
			wouldExceedL4 := false
			if pair.bal.Total > 0 {
				postAvail := pair.bal.Available - margin
				if postAvail < 0 {
					postAvail = 0
				}
				postRatio := 1 - postAvail/pair.bal.Total
				wouldExceedL4 = postRatio >= m.cfg.MarginL4Threshold
			}

			if pair.bal.Available < bufferedNeed || wouldExceedL4 {
				if topUp, ok := cache.TransferablePerExchange[pair.name]; ok && topUp > 0 {
					deficit := bufferedNeed - pair.bal.Available
					if deficit < 0 {
						deficit = 0
					}

					targetRatio := m.cfg.MarginL4Threshold - 0.005
					freeTarget := 1.0 - targetRatio
					if freeTarget > 0 && pair.bal.Total > 0 {
						ratioDeficit := (freeTarget*pair.bal.Total - pair.bal.Available + margin) / targetRatio
						if ratioDeficit > deficit {
							deficit = ratioDeficit
						}
					}

					actualTopUp := deficit
					if actualTopUp > topUp {
						actualTopUp = topUp
					}
					if actualTopUp > 0 {
						pair.bal.Available += actualTopUp
						pair.bal.Total += actualTopUp
						approvalTopUp[pair.name] += actualTopUp
					}
				}
			}
		}
	}
	if len(approvalTopUp) == 0 {
		approvalTopUp = nil
	}
}
```

- [ ] **Step 4: Implement — attach `approvalTopUp` to final approval return**

Modify `internal/risk/manager.go:756-765`. Codex-supplied verbatim BEFORE/AFTER:

BEFORE:

```go
return &models.RiskApproval{
	Approved:          true,
	Size:              size,
	Reason:            "all pre-trade checks passed",
	Price:             midPrice,
	GapBPS:            gapBps,
	RequiredMargin:    requiredWithBuffer,
	LongMarginNeeded:  longMarginWithBuffer,
	ShortMarginNeeded: shortMarginWithBuffer,
}, nil
```

AFTER:

```go
return &models.RiskApproval{
	Approved:          true,
	Size:              size,
	Reason:            "all pre-trade checks passed",
	Price:             midPrice,
	GapBPS:            gapBps,
	RequiredMargin:    requiredWithBuffer,
	LongMarginNeeded:  longMarginWithBuffer,
	ShortMarginNeeded: shortMarginWithBuffer,
	TopUpApplied:      approvalTopUp,
}, nil
```

- [ ] **Step 5: Run tests, verify they pass**

Run: `go test ./internal/risk/ -run 'Test(Simulator|Executor)_TopUpApplied' -v`
Expected: PASS all three tests.

Note: repeated `-run` flags only honor the last pattern — use a single regex as shown. Same correction applies to Step 2 above (`go test ./internal/risk/ -run 'Test(Simulator|Executor)_TopUpApplied' -v`).

- [ ] **Step 6: Run full risk package tests to catch regressions**

Run: `go test ./internal/risk/ -v`
Expected: all existing tests still PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/risk/manager.go internal/risk/manager_topup_test.go
git commit -m "risk(manager): track cross-exchange top-up in RiskApproval.TopUpApplied"
```

---

## Task 3: Route top-up approvals into case (b) in Pass-1

**Files:**
- Modify: `internal/engine/rebalance.go:210-258` (switch block)
- Test: `internal/engine/rebalance_topup_test.go`

- [ ] **Step 1: Write failing test**

Create `internal/engine/rebalance_topup_test.go`:

```go
package engine

import (
    "testing"

    "arb/internal/models"
)

// TestPass1_TopUpApprovedRoutesToCaseB verifies that an approved approval with
// non-empty TopUpApplied is treated as a rescue candidate (case b), not case (a).
// Baseline bug: METUSDT was routed to case (a), no transfer planned, executor rejected.
func TestPass1_TopUpApprovedRoutesToCaseB(t *testing.T) {
    eng := newTestEngineWithPass1Harness(t)

    opps := []models.Opportunity{
        {Symbol: "METUSDT", LongExchange: "gateio", ShortExchange: "bitget", Spread: 3.87, IntervalHours: 4},
    }

    // Fake SimulateApprovalForPair returns approved-with-topup.
    eng.risk.(*fakeRisk).nextApproval = &models.RiskApproval{
        Approved:          true,
        Size:              1596,
        Price:             0.188,
        RequiredMargin:    200,
        LongMarginNeeded:  199.93,
        ShortMarginNeeded: 199.71,
        TopUpApplied: map[string]float64{
            "gateio": 124,
            "bitget": 246,
        },
    }

    kept, pass2, reserved := eng.runPass1ForTest(opps)
    _ = kept
    _ = pass2
    _ = reserved

    if len(eng.pass1Candidates) != 1 {
        t.Fatalf("expected 1 rescue candidate, got %d (would-be case-a slipped through)", len(eng.pass1Candidates))
    }
    cand := eng.pass1Candidates[0]
    if cand.symbol != "METUSDT" {
        t.Errorf("candidate symbol = %s, want METUSDT", cand.symbol)
    }
}

// TestPass1_PlainApprovedStaysCaseA verifies approved WITHOUT top-up still goes case (a).
func TestPass1_PlainApprovedStaysCaseA(t *testing.T) {
    eng := newTestEngineWithPass1Harness(t)

    opps := []models.Opportunity{
        {Symbol: "ABCUSDT", LongExchange: "binance", ShortExchange: "okx", Spread: 3.5, IntervalHours: 4},
    }
    eng.risk.(*fakeRisk).nextApproval = &models.RiskApproval{
        Approved:          true,
        Size:              1000,
        Price:             0.5,
        RequiredMargin:    150,
        LongMarginNeeded:  150,
        ShortMarginNeeded: 150,
        // TopUpApplied intentionally nil — real balance sufficient.
    }

    kept, _, reserved := eng.runPass1ForTest(opps)
    _ = kept

    if len(eng.pass1Candidates) != 0 {
        t.Errorf("plain approved should NOT create rescue candidate, got %d", len(eng.pass1Candidates))
    }
    if reserved["binance"] <= 0 || reserved["okx"] <= 0 {
        t.Errorf("plain approved should still reserve: reserved=%v", reserved)
    }
}
```

Note: `newTestEngineWithPass1Harness`, `fakeRisk`, `runPass1ForTest`, `pass1Candidates` — these are test-scaffolding names. Two options:
1. If `internal/engine/rebalance_test.go` already has similar harness helpers, reuse them.
2. Otherwise, add a small test helper file `internal/engine/rebalance_topup_helpers_test.go` that minimally exercises the Pass-1 switch by extracting a package-private wrapper (e.g., a thin test-only func that accepts injected opps + a stubbed `risk` dependency and returns `(candidates, pass2Input, reserved)`). Follow whatever stub pattern the existing `internal/engine/rebalance_argmax_test.go` uses.

- [ ] **Step 2: Run tests, verify they fail**

Run: `go test ./internal/engine/ -run TestPass1_TopUp -run TestPass1_PlainApproved -v`
Expected: FAIL — `TestPass1_TopUpApprovedRoutesToCaseB` will show 0 candidates (current buggy behavior routes to case a).

- [ ] **Step 3: Implement — modify Pass-1 switch**

Modify `internal/engine/rebalance.go:210-258`. Current switch:

```go
switch {
case approval.Approved:
    // Case (a): capital sufficient — SKIP. Entry scan will open by
    // rank. Reserve per-leg margin so later Pass 1 iterations cannot
    // over-commit.
    longNeed := approval.LongMarginNeeded
    if longNeed <= 0 {
        longNeed = approval.RequiredMargin
    }
    shortNeed := approval.ShortMarginNeeded
    if shortNeed <= 0 {
        shortNeed = approval.RequiredMargin
    }
    if longNeed <= 0 && shortNeed <= 0 {
        delete(remaining, opp.Symbol)
        continue
    }
    reserved[opp.LongExchange] += longNeed
    reserved[opp.ShortExchange] += shortNeed
    approvedCaseA[opp.LongExchange] += longNeed
    approvedCaseA[opp.ShortExchange] += shortNeed
    delete(remaining, opp.Symbol)
    pass1UsedSlots++
    e.log.Debug("rebalance: pass1 case(a) approved %s %s/%s reserved long=%.2f short=%.2f",
        opp.Symbol, opp.LongExchange, opp.ShortExchange, longNeed, shortNeed)

case approval.Kind == models.RejectionKindCapital && hasPricedApprovalFields(approval):
    // Case (b): capital-short rescue candidate.
    choice := rescueChoiceFromApproval(opp, approval, e.cfg.CapitalPerLeg*e.cfg.MarginSafetyMultiplier)
    candidates = append(candidates, choice)
    rescueSymbols[opp.Symbol] = true
    longNeed, shortNeed := choice.marginNeeds()
    reserved[opp.LongExchange] += longNeed
    reserved[opp.ShortExchange] += shortNeed
    pass1UsedSlots++
    e.log.Debug("rebalance: pass1 case(b) rescue-candidate %s %s/%s margin long=%.2f short=%.2f",
        opp.Symbol, opp.LongExchange, opp.ShortExchange, longNeed, shortNeed)

default:
    e.log.Debug("rebalance: pass1 case(c) reject %s kind=%s reason=%s",
        opp.Symbol, approval.Kind.String(), approval.Reason)
}
```

Change to:

```go
switch {
case approval.Approved && len(approval.TopUpApplied) == 0:
    // Case (a): capital genuinely sufficient (no simulator top-up) — SKIP.
    // Entry scan will open by rank. Reserve per-leg margin so later Pass 1
    // iterations cannot over-commit.
    longNeed := approval.LongMarginNeeded
    if longNeed <= 0 {
        longNeed = approval.RequiredMargin
    }
    shortNeed := approval.ShortMarginNeeded
    if shortNeed <= 0 {
        shortNeed = approval.RequiredMargin
    }
    if longNeed <= 0 && shortNeed <= 0 {
        delete(remaining, opp.Symbol)
        continue
    }
    reserved[opp.LongExchange] += longNeed
    reserved[opp.ShortExchange] += shortNeed
    approvedCaseA[opp.LongExchange] += longNeed
    approvedCaseA[opp.ShortExchange] += shortNeed
    delete(remaining, opp.Symbol)
    pass1UsedSlots++
    e.log.Debug("rebalance: pass1 case(a) approved %s %s/%s reserved long=%.2f short=%.2f",
        opp.Symbol, opp.LongExchange, opp.ShortExchange, longNeed, shortNeed)

case (approval.Approved && len(approval.TopUpApplied) > 0) ||
    (approval.Kind == models.RejectionKindCapital && hasPricedApprovalFields(approval)):
    // Case (b): capital-short rescue candidate. Merged branch — either
    //   (i) simulator returned RejectionKindCapital with priced fields, OR
    //   (ii) simulator approved ONLY because cross-exchange top-up inflated
    //        pair.bal.Available; TopUpApplied records per-exchange borrow amounts.
    // Both cases need a real transfer scheduled before entry can open.
    // rescueChoiceFromApproval already handles priced-approved input (Size/Price
    // populated); it reads margin from approval.Long/ShortMarginNeeded.
    choice := rescueChoiceFromApproval(opp, approval, e.cfg.CapitalPerLeg*e.cfg.MarginSafetyMultiplier)
    candidates = append(candidates, choice)
    rescueSymbols[opp.Symbol] = true
    longNeed, shortNeed := choice.marginNeeds()
    reserved[opp.LongExchange] += longNeed
    reserved[opp.ShortExchange] += shortNeed
    pass1UsedSlots++
    if len(approval.TopUpApplied) > 0 {
        e.log.Debug("rebalance: pass1 case(b) top-up rescue-candidate %s %s/%s topUp=%v margin long=%.2f short=%.2f",
            opp.Symbol, opp.LongExchange, opp.ShortExchange, approval.TopUpApplied, longNeed, shortNeed)
    } else {
        e.log.Debug("rebalance: pass1 case(b) rescue-candidate %s %s/%s margin long=%.2f short=%.2f",
            opp.Symbol, opp.LongExchange, opp.ShortExchange, longNeed, shortNeed)
    }

default:
    e.log.Debug("rebalance: pass1 case(c) reject %s kind=%s reason=%s",
        opp.Symbol, approval.Kind.String(), approval.Reason)
}
```

- [ ] **Step 4: Run tests, verify they pass**

Run: `go test ./internal/engine/ -run TestPass1_TopUp -run TestPass1_PlainApproved -v`
Expected: PASS both tests.

- [ ] **Step 5: Run full engine package tests**

Run: `go test ./internal/engine/ -v`
Expected: all existing tests still PASS. Pay attention to existing `rebalance_test.go`, `rebalance_argmax_test.go`, `engine_rank_first_test.go`, `allocator_transferable_parity_test.go` — none should regress.

- [ ] **Step 6: Commit**

```bash
git add internal/engine/rebalance.go internal/engine/rebalance_topup_test.go
git commit -m "engine(rebalance): route top-up approvals into case(b) rescue path"
```

---

## Task 4: Verify `rescueChoiceFromApproval` handles approved input

**Status: DONE (audit-only, no code change needed)**

**Files:**
- Read: `internal/engine/rebalance.go` (find `rescueChoiceFromApproval`)
- Modify (if needed): `internal/engine/rebalance.go`

- [x] **Step 1: Read the function**

`rescueChoiceFromApproval` (lines 679–715) reads `a.Size`, `a.Price`, `a.LongMarginNeeded`,
`a.ShortMarginNeeded`, `a.RequiredMargin`. It does NOT branch on `approval.Approved` anywhere.
No `if a.Approved` check exists in the function body. Audit passed.

- [x] **Step 2: No branch on Approved=false found**

The function is field-access only. It accepts any `*models.RiskApproval` regardless of the
`Approved` boolean — no change required.

- [x] **Step 3: Pass-1 tests confirm no regression**

```
--- PASS: TestPass1_TopUpApprovedRoutesToCaseB (0.03s)
--- PASS: TestPass1_PlainApprovedStaysCaseA (0.01s)
PASS
ok  arb/internal/engine  0.161s
```

- [x] **Step 4: Empty commit with M6 resolution documented**

### M6 Resolution

Reviewer concern: case(b) branch(i) guard `(approval.Approved && len(approval.TopUpApplied) > 0)`
does not check `hasPricedApprovalFields`. Could an `Approved=true, TopUpApplied non-empty,
Size=0/Price=0` input reach `rescueChoiceFromApproval`?

**Finding: NOT reachable in practice.** Traced through `approveInternal` in `manager.go`:

1. `approvalTopUp` is only initialised when `dryRun && needed > 0` (line 335).
2. After balance inflation, `size` is recomputed via `calculateSizeWithPrice` (line 469).
   If `size <= 0`, the function returns `Approved=false, Kind=RejectionKindCapital` immediately
   (line 472) — never reaches the terminal `Approved=true` return.
3. `midPrice` comes from `longOB.Bids[0]/Asks[0]`. An empty orderbook returns
   `Approved=false, Kind=RejectionKindMarket` (line 463) before price is used.
4. The terminal return (lines 762–772) sets `Size: size > 0`, `Price: midPrice > 0`,
   `RequiredMargin/LongMarginNeeded/ShortMarginNeeded` all derived from
   `size * midPrice / leverage * safetyMultiplier > 0`.

Therefore: **`Approved=true` with `TopUpApplied non-empty` structurally implies all pricing
fields are non-zero.** The M6 scenario (unpriced approved input) cannot occur at runtime.

**Resolution A** confirmed: `rescueChoiceFromApproval` also has a defensive `fallbackMargin`
path (lines 692–700) that handles zero-margin as a last resort, making it doubly safe.
Adding `hasPricedApprovalFields` to the case(b)(i) guard would be redundant. No change needed.

```bash
git commit --allow-empty -m "engine(rebalance): audit rescueChoiceFromApproval handles approved input (no change)"
```

---

## Task 5: Verify `postTransferReplayFilter` still works end-to-end

**Files:**
- Read: `internal/engine/rebalance.go:357-450` (`postTransferReplayFilter`)
- Test: `internal/engine/rebalance_topup_test.go` (extend)

- [ ] **Step 1: Read postTransferReplayFilter**

Read `internal/engine/rebalance.go:357-450`. Confirm it builds `replayCache` with `TransferablePerExchange = nil` (or explicitly cleared) so the second simulation pass does NOT inflate balance again. Per Codex independent review (dispatch baa9d706), the current implementation at lines 407-413 already clears `TransferablePerExchange`.

- [ ] **Step 2: Extend rebalance_topup_test.go with replay scenario**

Append to `internal/engine/rebalance_topup_test.go`:

```go
// TestPass1_TopUpReplayRejectsIfRealBalanceStillInsufficient verifies the
// post-transfer replay correctly refuses a top-up rescue when the simulated
// transfer would not land enough capital. Protects against the case where
// top-up amount > actual donor capacity.
func TestPass1_TopUpReplayRejectsIfRealBalanceStillInsufficient(t *testing.T) {
    eng := newTestEngineWithPass1Harness(t)

    opps := []models.Opportunity{
        {Symbol: "METUSDT", LongExchange: "gateio", ShortExchange: "bitget", Spread: 3.87, IntervalHours: 4},
    }

    // First call: approved-with-topup (150 per leg).
    // Replay call (after fake transfer): simulator should reject because
    // fakeTransferPlan only moves 50 per leg — insufficient.
    fr := eng.risk.(*fakeRisk)
    fr.nextApproval = &models.RiskApproval{
        Approved: true,
        Size: 1596, Price: 0.188,
        RequiredMargin: 200, LongMarginNeeded: 199.93, ShortMarginNeeded: 199.71,
        TopUpApplied: map[string]float64{"gateio": 150, "bitget": 150},
    }
    fr.replayApproval = &models.RiskApproval{
        Approved: false,
        Kind:     models.RejectionKindCapital,
        Reason:   "post-trade margin ratio would reach 0.91",
    }

    kept, _, _ := eng.runPass1ForTest(opps)
    if len(kept) != 0 {
        t.Errorf("replay should have dropped top-up rescue when transfer insufficient, kept=%d", len(kept))
    }
}
```

- [ ] **Step 3: Run replay test**

Run: `go test ./internal/engine/ -run TestPass1_TopUpReplay -v`
Expected: PASS — kept is empty.

- [ ] **Step 4: Commit**

```bash
git add internal/engine/rebalance_topup_test.go
git commit -m "engine(rebalance): add replay-rejection test for insufficient top-up rescue"
```

---

## Task 6: Audit other `SimulateApprovalForPair` callers

**Files:**
- Read: `internal/engine/allocator.go:430` (pool allocator reval)
- Read: `internal/engine/allocator.go:568` (sequential rank-first)

- [ ] **Step 1: Read allocator.go:430 context (runPoolAllocator reval loop)**

Read `internal/engine/allocator.go` around line 430. Look for how `approval` is consumed after the call. The question: does any code path treat `approval.Approved == true` as "immediately executable" and act on it without checking `TopUpApplied`?

Specifically, check:
- Does pool allocator schedule a transfer independently based on `dryRunTransferPlan`, or does it rely on `approval.Approved == true` to mean "no transfer needed"?
- If the latter, the same bug exists in the pool allocator path.

- [ ] **Step 2: Read allocator.go:568 context (sequential rank-first)**

Similar audit for line 568.

- [ ] **Step 3: Document findings in plan + tests**

Add to the END of `plans/PLAN-topup-transfer-routing.md` (this file):

```markdown
## Caller Audit Results (Task 6)

- `allocator.go:430` (pool allocator reval): [SAFE | BUG — describe]
- `allocator.go:568` (sequential rank-first): [SAFE | BUG — describe]
```

If either is a BUG: add a Task 6b to this plan extending the fix to that caller using the same `TopUpApplied` field. Do not implement until the plan extension has been reviewed (this means dispatching Codex review again).

If both are SAFE: commit the audit result as a no-op commit with explanation.

```bash
git commit --allow-empty -m "engine(rebalance): audit allocator.go top-up handling — both SAFE (see plan)"
```

---

## Task 7: Integration verification — real compile + run

**Files:** None modified. Verify the whole package builds and tests pass.

Note: This repo uses `go:embed` to embed `web/dist/` into the Go binary, so the frontend MUST build before `go build ./...` or the embed directive fails / serves stale JS.

- [ ] **Step 1: Frontend build**

Run: `cd web && npm run build && cd ..`
Expected: success, `web/dist/` populated.

- [ ] **Step 2: Full repo build**

Run: `go build ./...`
Expected: success, no errors.

- [ ] **Step 3: Full repo test**

Run: `go test ./...`
Expected: all tests PASS. If pre-existing failures exist at `de724e1a` baseline (unrelated to this change), document them but do not block on them — only fail on NEW failures introduced by Tasks 1–6.

- [ ] **Step 4: Lint (go vet)**

Run: `go vet ./...`
Expected: no NEW warnings introduced by this change. Pre-existing vet warnings at `de724e1a` are acceptable if they also exist on the unchanged baseline.

- [ ] **Step 5: Commit verification**

```bash
git commit --allow-empty -m "chore: verify full-repo build + test after top-up routing fix"
```

---

## Task 8: Version bump + CHANGELOG

**Files:**
- Modify: `VERSION`
- Modify: `CHANGELOG.md`

- [ ] **Step 1: Bump patch version**

Read current `VERSION`. Increment patch (e.g., `0.33.0` → `0.33.1`) and write it back.

- [ ] **Step 2: Add CHANGELOG entry**

Prepend to `CHANGELOG.md` under a new version header:

```markdown
## v0.33.1 — 2026-04-22

- **Rebalance top-up routing bug** — simulator's cross-exchange top-up inflated `pair.bal.Available` to pass margin/L4 checks, but the resulting `approval.Approved=true` was routed into Pass-1 case (a) which schedules NO transfer. Subsequent entry-scan executor rejected on real (unrestocked) balance. 2026-04-22 METUSDT gateio/bitget incident: 11:35 rebalance reserved 199.93/199.71, 11:45 entry rejected with post-trade margin ratio 0.87 on gateio (>L4 0.80). Fix:
  - Added `RiskApproval.TopUpApplied map[string]float64` recording per-exchange simulator borrows (`internal/models/interfaces.go`).
  - `approveInternal` populates the field only when `dryRun=true` (`internal/risk/manager.go`).
  - Pass-1 switch at `internal/engine/rebalance.go:210` routes `Approved && len(TopUpApplied) > 0` into the case-(b) rescue-candidate path, unifying with existing `RejectionKindCapital` handling so a real transfer is planned and the post-transfer replay re-validates against `TransferablePerExchange=nil`.
  - Regression tests in `internal/risk/manager_topup_test.go` and `internal/engine/rebalance_topup_test.go`.
  - Independent review via dispatch-mcp (task `baa9d706`) confirmed the bug before fix.
```

- [ ] **Step 3: Commit**

```bash
git add VERSION CHANGELOG.md
git commit -m "chore: bump v0.33.1 for top-up routing fix"
```

---

## Risk / Side Effects

| Risk | Mitigation |
|------|-----------|
| Other callers of `SimulateApprovalForPair` in `allocator.go` (pool allocator reval, sequential rank-first) may also treat approved-with-topup as executable | Task 6 audits both; add extension if bugs found |
| `rescueChoiceFromApproval` may not expect `Approved=true` input | Task 4 audits and fixes if needed |
| `postTransferReplayFilter` must see `TopUpApplied` cleared by replay cache (no re-inflation) | Task 5 adds explicit regression test; Codex confirmed line 407-413 clears `TransferablePerExchange` |
| JSON serialization of `RiskApproval` over dashboard websocket | `omitempty` tag on `TopUpApplied` keeps existing payloads identical when empty |
| `hasPricedApprovalFields` returns true for approved path — verified (approved return at manager.go:756 sets `Size`, `Price`, `RequiredMargin`, margins) | No change to helper needed |
| Executor path inadvertently setting `TopUpApplied` | Test 3 in Task 2 pins this — `Approve()` path must leave it nil |

## Out of Scope

- Changes to pool allocator transfer semantics (Tier-1/Tier-2 inversion, argmax selector) — unrelated
- Changes to `PrefetchCache.TransferablePerExchange` computation — unrelated
- Changes to `keepFundedChoices` / post-transfer replay filter mechanics — already correct per Codex audit, just extended with a regression test
- **Rebalance snapshot drift** (the 10-minute gap between rebalance at :35 balance snapshot and executor at :45 live re-read). Codex v1 independent review (dispatch `baa9d706`) flagged this as a separate concern that widens divergence even without the top-up bug. This plan does NOT address drift; a follow-up plan should consider either re-snapshotting closer to entry or deadlining case-(a) reservations.

## Review History

- v1 2026-04-22: DRAFT — initial plan
- v2 2026-04-22: codex (dispatch `f6d78e6f`) — NEEDS-REVISION: Task 2 scope ambiguity + `-run` flag duplication; Task 7 missing frontend build (`go:embed` order); fixes applied verbatim per Codex's BEFORE/AFTER

---

## Caller Audit Results (Task 6)

- `allocator.go:430` (pool allocator reval): **SAFE** — The re-validation loop (lines 421–457) uses `approval.Approved` only as a pass/fail gate to include the choice in `validated[]`. After validation, `dryRunTransferPlan(selected, balances, feeCache)` is called unconditionally on real `balances` (not the inflated simulator cache) to compute actual transfer deficits and feasibility. `TopUpApplied` is never consulted; if a top-up was needed, `dryRunTransferPlan` independently recomputes the deficit from real balance state and either plans a real transfer or returns `Feasible: false`. There is no code path where `approval.Approved == true` causes a transfer to be skipped.

- `allocator.go:568` (sequential rank-first / `buildAllocatorCandidates`): **SAFE** — The call is inside `appendChoice`, a candidate-builder closure. `approval.Approved == true` is used only as a gate to include a pair as an `allocatorChoice` struct candidate for the solver. The `TopUpApplied` field is never read here. Transfer planning happens entirely downstream via `solveAllocator` → `dryRunTransferPlan` on real balance data. An approval that was only reachable via top-up inflation may produce a candidate that `dryRunTransferPlan` later rejects as infeasible if real donors cannot cover the deficit — this is the correct fail-safe behavior.

**Both callers are SAFE.** Neither treats `approval.Approved == true` as "no transfer needed." Both route through `dryRunTransferPlan` which independently computes transfer plans from real balances, making `TopUpApplied` irrelevant to their correctness. The Codex v2 review finding (`2efe0b06`) is confirmed correct.

Commit: see commit immediately following Task 5

---

## Self-Review (Writer)

**Spec coverage:** All 3 change points from `CLAIM` (simulator, approval struct, rebalance switch) have tasks (Tasks 1/2/3). Post-transfer replay behavior covered by Task 5. Other callers covered by Task 6. Version/changelog Task 8.

**Placeholder scan:** No TBD/TODO/placeholder patterns. All Go code blocks are concrete. Some test helper names (`newTestEngineWithPass1Harness`, `fakeRisk`) are flagged as requiring the executor to reuse existing test patterns — this is acceptable because the skill permits "reuse existing helpers if present; otherwise follow `rebalance_argmax_test.go` pattern" as a concrete instruction, not a placeholder.

**Type consistency:** `TopUpApplied map[string]float64` consistent across Tasks 1-3. `approval.Approved` and `approval.TopUpApplied` names match Go struct conventions.
