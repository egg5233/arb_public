# PLAN — Stage 2 DB-Failure Salvage (Finding #1 from codex d1ac2563)

**Author:** zxcnny930 + Claude
**Date:** 2026-04-21
**Status:** v5 — addresses independent codex 2c8bf062 (3 MEDIUM findings)
**Review log:**
- independent codex 2c8bf062 NEEDS-REVISION → v5 fixes 3:
  1. §5 Test harness nil-deref: `buildRankFirstEngine` in `engine_rank_first_test.go` builds Engine via struct literal, NOT `NewEngine()`. After adding `activePosSource`, any mirrored test builder leaves the field nil → `getActivePositionsWithRetry` nil-derefs. Plan now explicitly states: test harness MUST set `e.activePosSource` (either to the miniredis-backed `e.db`, or to the failing stub). Added explicit builder guidance in §5.
  2. §5 Missing retry-success test: T1-T7 either pure-success or pure-failure. Added **T2b** = "first GetActivePositions attempt fails, second succeeds" → proves the retry helper works, not just the salvage fallback. T2 renamed T2a.
  3. §5 Missing mixed-validity test: no scenario covers "same cycle, some overrides valid + some stale". Added **T8** = override `{A, B}`, `applyAllocatorOverrides` returns `[A_patched]` with B stale-skipped. Stage 2 execs `[A]`, `stage2Attempted={A}`, salvage rescans `{B}`. Proves the `stage2Attempted + openedByUs` logic handles mixed-validity input.
  Other probed edges confirmed OK by 2c8bf062: snapshot-before-drain race, StatusClosed filter, RescanSymbols constraints, single-goroutine sleep is acceptable.
- codex 62d8e2fb NEEDS-REVISION → v4 fixes 1:
  1. §3.4 `getActivePositionsWithRetry` helper snippet now uses `e.activePosSource.GetActivePositions()` (matches §5 design). Previous v3 had `e.db.GetActivePositions()` directly, which bypassed the new injection hook and contradicted §5. W2a/W2b/W2c/W3/W4/W5 all PASS per codex 62d8e2fb.

- codex 9582ebaa NEEDS-REVISION → v3 fixes 2:
  1. §5 DB failure injection: remove the speculative "preferred if a StateStore/wrapper already exists" branch. `Engine` currently holds a concrete `*database.Client`, so `activePositionsGetter` interface injection is the single explicit plan.
  2. §5 T6 expected outcome: drop the hard-coded "B fails re-scan" assertion. Rewrite as "salvage calls `RescanSymbols({B})`; test controls rescan stub outcome deterministically." Pick one path for the actual test case. §6 R4 no longer tied to T6's specific outcome.
- codex 07728398 NEEDS-REVISION → v2 fixes 4:
  1. §3.2/§3.4: `db.GetActivePositions()` returns `[]*models.ArbitragePosition`, not `[]*models.Position` (verified at `internal/database/state.go:164`, `internal/models/position.go:6`)
  2. §5 test matrix: add T6 (all-stale `patched=nil, hadOverrides=true`) and T7 (multi-symbol partial, override={A,B}, Stage 1 opens A, Stage 2 opens B, salvage noop)
  3. §5 failure injection: drop `miniredis.SetError(...)` speculation; use a thin failing `StateStore` stub (no such method exists in current engine tests — verified)
  4. §3.4/§8: retry latency math corrected — one retry = one `time.Sleep(backoff)` per call site, so worst-case added backoff is `~300ms × 2 call sites = ~600ms` plus DB call time, not `600ms × 2 = 1.2s`
**Branch:** `fix/allocator-unified` (continues existing branch; follow-up commits on top of 32bdb810)
**Origin:** codex independent review `d1ac2563` verdict NEEDS-REVISION Finding #1 (HIGH).

---

## 1. Goal

Close the single-point-of-failure in `internal/engine/engine.go:888-1018` where a transient
`db.GetActivePositions()` failure between Stage 1 and Stage 2 causes `applyAllocatorOverrides`
to drain `e.allocOverrides` **without executing any Stage 2 opps**, and the existing salvage
block (`:950-987`) silently excludes those funded-but-un-opened symbols because they are
`inScan[sym]`.

After this plan:
1. A Stage 2 `GetActivePositions()` failure is retried once with backoff before falling through.
2. If Stage 2 still fails to execute, the salvage block picks up **all** unopened override
   symbols (in-scan AND out-of-scan), not just the out-of-scan subset.
3. The failure class is covered by targeted tests.

---

## 2. Problem Analysis (from codex d1ac2563)

> "A single `GetActivePositions()` failure after `applyAllocatorOverrides()` can still drop
> funded overrides for symbols that ARE in the current scan, causing under-trade / possible
> fund stranding. … The later salvage block only rescans override symbols NOT in the current
> scan (`engine.go:950-978` uses `inScan[sym]` as an exclusion), so any funded override
> symbol that was present in `result.Opps` but never reached `executeArbitrage(stage2Opps, ...)`
> is now lost for the cycle."

### 2.1 Concrete sequence leading to the bug

1. `:20` rebalance funds override for symbol `SYM_A` with pair `BinanceLong/BybitShort` →
   `e.allocOverrides["SYM_A"] = {Binance, Bybit}`.
2. `:40` entry scan. `result.Opps` contains `SYM_A` (possibly with a different pair).
3. Stage 1 rank-based `executeArbitrage(result.Opps, ...)` runs — `SYM_A` is not opened
   (lower rank than other opps filling MaxPositions budget, or risk rejection).
4. `overrideSnapshot := {"SYM_A": ...}` captured. ✓
5. `applyAllocatorOverrides(result.Opps)` drains `e.allocOverrides = nil`, returns
   `patched = [SYM_A_patched]`, `hadOverrides = true`.
6. `activeAfterS1, err := e.db.GetActivePositions()` — **Redis transient error**.
7. Line 909: `e.log.Warn(... "skipping stage-2 ...")`. No further Stage 2 work.
8. Salvage block at `:950`:
   - `inScan["SYM_A"] = true` (it was in `result.Opps`).
   - Loop body at `:966-975`: `if inScan[sym] || openedByUs[sym] { continue }` → **skip**.
   - `SYM_A` is NOT rescanned. Funds stranded until next rebalance cycle.

### 2.2 Why `inScan[sym]` was used

Historical reason: the Bug-7 salvage block was written to handle the case
**"rebalance funded a symbol that DROPPED OUT of the current scan"**. The `inScan`
exclusion assumed Stage 2 always had a successful chance on in-scan symbols. When the
Stage 2 DB read fails, that assumption breaks.

---

## 3. Fix Design

### 3.1 Two complementary fixes

**Fix A — Retry Stage 2 `GetActivePositions()` once with a short backoff.**
Reduces the probability of the failure class. Does not eliminate it.

**Fix B — Replace `inScan[sym]` exclusion with `stage2Attempted[sym]` tracking.**
Eliminates the blind-spot even when both retries fail.

Both are in-scope for this plan. Fix B is load-bearing; Fix A is insurance.

### 3.2 Real API signatures (no invented APIs)

```go
// Existing in internal/database/state.go:164:
func (db *Client) GetActivePositions() ([]*models.ArbitragePosition, error)

// Existing in pkg/utils/logging.go:
type Logger struct { ... }
func (l *Logger) Warn(format string, args ...interface{})
func (l *Logger) Info(format string, args ...interface{})

// Existing in internal/engine/engine.go:
func (e *Engine) executeArbitrage(opps []models.Opportunity, perpCap int)
func (e *Engine) applyAllocatorOverrides(opps []models.Opportunity) ([]models.Opportunity, bool)

// Existing in internal/discovery/scanner.go:
func (s *Scanner) RescanSymbols(pairs []models.SymbolPair) []models.Opportunity
```

No new package imports. `time.Sleep` already imported in engine.go.

### 3.3 Pseudocode (replaces engine.go:888-988)

```go
e.log.Info("entry stage-1: rank-based executeArbitrage with %d opps", len(result.Opps))
e.executeArbitrage(result.Opps, perpCap)

// Snapshot BEFORE drain (Bug 7 invariant preserved).
e.allocOverrideMu.Lock()
overrideSnapshot := make(map[string]allocatorChoice, len(e.allocOverrides))
for k, v := range e.allocOverrides {
    overrideSnapshot[k] = v
}
e.allocOverrideMu.Unlock()

patched, hadOverrides := e.applyAllocatorOverrides(result.Opps)

// Track which override symbols Stage 2 actually got to try.
// Default: empty. Populated only when Stage 2 DB read succeeds AND executeArbitrage runs.
stage2Attempted := map[string]bool{}

if len(patched) > 0 {
    // Finding #1 Fix A: retry DB read once on transient failure.
    activeAfterS1, err := e.getActivePositionsWithRetry(2, 300*time.Millisecond)
    if err != nil {
        e.log.Warn("entry stage-2: GetActivePositions failed after retry: %v — deferring to salvage", err)
        // Intentionally do NOT early-return: fall through to salvage with
        // stage2Attempted empty, so salvage covers in-scan symbols too.
    } else {
        openedS1 := make(map[string]bool, len(activeAfterS1))
        for _, p := range activeAfterS1 {
            if p.Status != models.StatusClosed {
                openedS1[p.Symbol] = true
            }
        }
        var stage2Opps []models.Opportunity
        dropped := 0
        for _, o := range patched {
            // Mark attempted regardless of duplicate-filter outcome.
            // openedS1 symbols are already active — salvage would be a noop anyway.
            stage2Attempted[o.Symbol] = true
            if openedS1[o.Symbol] {
                dropped++
                continue
            }
            stage2Opps = append(stage2Opps, o)
        }
        if dropped > 0 {
            e.log.Info("entry stage-2: %d override(s) skipped (already opened by stage-1)", dropped)
        }
        if len(stage2Opps) > 0 {
            e.log.Info("entry stage-2: executing %d override opportunities (tier-2-rebalance-overrides)", len(stage2Opps))
            e.executeArbitrage(stage2Opps, perpCap)
        } else {
            e.log.Info("entry stage-2: no override opportunities remain after stage-1 filter")
        }
    }
} else if hadOverrides {
    e.log.Warn("entry stage-2: allocator overrides present but all stale after filter")
    // NOTE: stage2Attempted stays empty. Salvage will consider these symbols.
    // applyAllocatorOverrides already logged per-symbol reasons. Symbols that
    // were legitimately stale-rejected will fail RescanSymbols too, so salvage
    // is a noop for them — no extra cost.
}

// --- Salvage block ---
if len(overrideSnapshot) > 0 {
    activeForSalvage, err := e.getActivePositionsWithRetry(2, 300*time.Millisecond)
    if err != nil {
        e.log.Warn("entry stage-2 salvage: GetActivePositions failed after retry: %v — transfers may be stranded until next cycle", err)
    } else {
        openedByUs := make(map[string]bool, len(activeForSalvage))
        for _, p := range activeForSalvage {
            if p.Status != models.StatusClosed {
                openedByUs[p.Symbol] = true
            }
        }
        var missingPairs []models.SymbolPair
        for sym, choice := range overrideSnapshot {
            // Finding #1 Fix B: the exclusion is now `stage2Attempted` (what we
            // actually tried) + `openedByUs` (what's actually active).
            // Dropping `inScan[sym]` — that was an over-approximation that
            // excluded symbols Stage 2 never got to attempt.
            if stage2Attempted[sym] || openedByUs[sym] {
                continue
            }
            missingPairs = append(missingPairs, models.SymbolPair{
                Symbol:        sym,
                LongExchange:  choice.longExchange,
                ShortExchange: choice.shortExchange,
            })
        }
        if len(missingPairs) > 0 {
            e.log.Info("entry stage-2 salvage: %d funded override(s) need rescan (stage2-skipped or out-of-scan), re-scanning", len(missingPairs))
            salvageOpps := e.discovery.RescanSymbols(missingPairs)
            if len(salvageOpps) > 0 {
                e.log.Info("entry stage-2 salvage: %d/%d passed re-scan, executing", len(salvageOpps), len(missingPairs))
                e.executeArbitrage(salvageOpps, perpCap)
            } else {
                e.log.Warn("entry stage-2 salvage: all %d symbols failed re-scan — transfers may be stranded until next cycle", len(missingPairs))
            }
        }
    }
}
e.log.Info("run loop: entryScan handler done (stage-1 + stage-2 + salvage)")
```

### 3.4 New helper — `getActivePositionsWithRetry`

Lives in `internal/engine/engine.go` near existing helpers (private method on `*Engine`).

```go
// getActivePositionsWithRetry wraps the injected activePositionsGetter with a
// simple retry to survive transient Redis hiccups during the critical
// stage-1 → stage-2 transition. Used only by the entry-scan path where a
// dropped read leads to fund stranding (see PLAN-stage2-db-failure-salvage.md).
//
// The call goes through e.activePosSource (NOT e.db directly) so that tests
// can inject a failingActivePosGetter to exercise the retry + salvage branches.
// e.activePosSource is wired to e.db in NewEngine; see §5.
func (e *Engine) getActivePositionsWithRetry(maxAttempts int, backoff time.Duration) ([]*models.ArbitragePosition, error) {
    if maxAttempts < 1 {
        maxAttempts = 1
    }
    var lastErr error
    for i := 0; i < maxAttempts; i++ {
        positions, err := e.activePosSource.GetActivePositions()
        if err == nil {
            return positions, nil
        }
        lastErr = err
        if i < maxAttempts-1 {
            time.Sleep(backoff)
        }
    }
    return nil, lastErr
}
```

- `maxAttempts=2` at call sites → 1 retry (total 2 attempts), matches Fix A.
- `backoff=300ms` → bounded latency impact on the entry-scan loop.
- Short, intentionally dumb; we do NOT want exponential escalation here because
  the entry scan window is short and the salvage path is the real safety net.
- **Latency math**: one retry = one `time.Sleep(backoff)` per call site. With 2 call
  sites (Stage 2 filter + salvage filter), worst case is `~300ms × 2 = ~600ms` of
  added sleep, plus the DB round-trip time on each failed attempt. Negligible
  relative to `executeArbitrage` which already runs synchronously on this goroutine
  for >1s routinely.

---

## 4. Affected Files

| File | Change | Lines |
|---|---|---|
| `internal/engine/engine.go` | New interface `activePositionsGetter` + `activePosSource` field + constructor wire-up. | +10 |
| `internal/engine/engine.go` | New helper `getActivePositionsWithRetry` (uses `e.activePosSource`). | +15 |
| `internal/engine/engine.go` | Rewrite of Stage 2 + salvage block (`:888-988`). | ~100 replaced |
| `internal/engine/engine_stage2_salvage_test.go` (new) | 9 scenarios T1, T2a, T2b, T3, T4, T5, T6, T7, T8. `failingActivePosGetter` used by T2a, T2b, T4. | +~420 |

No changes outside `internal/engine/`. No config, no API, no frontend, no Redis schema.

---

## 5. Tests (new file: `engine_stage2_salvage_test.go`)

Goal: cover the three paths that currently lack tests (codex Finding #2, MEDIUM).

### Test harness builder

**IMPORTANT** (2c8bf062 Finding 1): the existing `buildRankFirstEngine` in
`engine_rank_first_test.go` constructs `Engine` via a **struct literal**, not
`NewEngine()`. After Step 0 adds `activePosSource`, a mirrored builder will
leave `activePosSource` nil — `getActivePositionsWithRetry` would nil-deref
before exercising the retry path.

The new test builder in `engine_stage2_salvage_test.go` MUST explicitly
initialize `activePosSource`:

```go
func buildStage2SalvageEngine(t *testing.T, overrides ...option) (*Engine, *miniredis.Miniredis) {
    mr, db := newMiniredisClient(t)  // mirrors existing helper
    e := &Engine{
        db: db,
        // ...other existing fields mirrored from buildRankFirstEngine...
    }
    e.activePosSource = e.db  // REQUIRED: mirror NewEngine's constructor line.
    for _, opt := range overrides {
        opt(e)  // tests that need failure injection override activePosSource here.
    }
    return e, mr
}

type option func(*Engine)

func withFailingActivePosGetter(failN int) option {
    return func(e *Engine) {
        e.activePosSource = &failingActivePosGetter{real: e.db, failN: failN}
    }
}
```

Tests that don't need failure injection use the default builder (get `e.db`
behavior). Tests T2a/T2b/T4 pass `withFailingActivePosGetter(N)` to swap the
seam. This must be a single-threaded swap BEFORE the handler is invoked — no
concurrent re-assignment after the run loop starts.

### Assertions each test makes

Each test drives the same code path as production: sets `e.allocOverrides`,
populates miniredis with known active positions (or none), invokes the Stage 2 +
salvage block via the same entry scan entry point used in
`engine_rank_first_test.go`, and asserts on:
- The set of symbols passed to `executeArbitrage` (captured via a stub
  `executeArbitrage` or an `onArbRun` hook — choose whichever the existing test
  harness already uses; mirror `engine_rank_first_test.go`).
- For T2a/T2b/T4: the `failingActivePosGetter.calls` counter (proves retry was
  actually exercised, not just bypassed).
- `e.allocOverrides` is drained regardless.
- Log assertions are NOT required — behavior assertions only.

### Test matrix

| # | Scenario | Stage 1 opps | Override set | S1 opens | S2 DB | Salvage DB | Expected |
|---|---|---|---|---|---|---|---|
| T1 | Happy path: Stage 2 executes normally | `[A, B]` | `{B}` | `{A}` | OK | OK | Stage 2 execs `[B]`; salvage noop |
| T2a | **Regression for Finding #1**: Stage 2 DB fails both attempts, in-scan override recovered by salvage | `[A, B]` | `{B}` | `{A}` | FAIL (failN=2) | OK | Stage 2 execs nothing; salvage rescans `{B}` and execs it. `failingActivePosGetter.calls == 2` asserted. |
| T2b | **Retry success**: Stage 2 DB fails first attempt, succeeds second attempt (proves retry helper works, not just salvage fallback) | `[A, B]` | `{B}` | `{A}` | FAIL then OK (failN=1) | OK | Stage 2 executes `[B]` via retry success; salvage is a noop because `stage2Attempted={B}` covers it. `failingActivePosGetter.calls == 2` asserted (1 fail + 1 success), `time.Sleep(300ms)` path exercised once. |
| T3 | Out-of-scan override recovered by salvage (Bug-7 existing behavior preserved) | `[A]` | `{B}` | `{A}` | OK | OK | Stage 2 execs nothing (`B` filtered out by `applyAllocatorOverrides`); salvage rescans `{B}` |
| T4 | Both DB reads fail → clean skip, no crash, no duplicate | `[A, B]` | `{B}` | `{A}` | FAIL | FAIL | No Stage 2 exec, no salvage exec, warn logged, `e.allocOverrides` drained |
| T5 | Stage 1 already opened override symbol → Stage 2 + salvage both skip | `[A, B]` | `{B}` | `{A, B}` | OK | OK | Stage 2 drops B as duplicate; salvage drops B via `openedByUs` (no duplicate open) |
| T6 | **All-stale** case: `applyAllocatorOverrides` returns `(nil, true)` | `[A]` | `{B}` with stale alt | `{A}` | n/a (no patched) | OK | Stage 2 logs "all stale after filter", `stage2Attempted` empty. Salvage **must call** `RescanSymbols({B})` (assertion: rescan was invoked with exactly the `{B}` pair). Test controls the stubbed rescan outcome. We pick the **empty-result** branch for T6's concrete assertion: rescan returns `[]`, so no `executeArbitrage` call is made; the "all … failed re-scan" warn path is exercised. A second test variant T6b using the **non-empty** branch is out of scope for this PR but noted as a natural extension. |
| T7 | **Multi-symbol partial**: Stage 1 opens A, Stage 2 opens B, salvage true noop | `[A, B]` | `{A, B}` | `{A}` | OK | OK | `applyAllocatorOverrides` returns `[A_patched, B_patched]`. Stage 2: `openedS1={A}` → drops A as duplicate, execs `[B]`. `stage2Attempted={A, B}`. Salvage: both excluded (A via `stage2Attempted`, B via `stage2Attempted` AND `openedByUs`). No salvage call. |
| T8 | **Mixed validity**: override set has BOTH valid and stale entries in same cycle | `[A, B]` | `{A, B}` — B's alt is stale, A's alt is valid | `{}` (S1 opens nothing) | OK | OK | `applyAllocatorOverrides` returns `[A_patched]` (B stale-skipped internally). Stage 2: `openedS1={}` → execs `[A]`, `stage2Attempted={A}`. Salvage: iterates `overrideSnapshot={A,B}`. A is in `stage2Attempted` → skip. B is NOT in `stage2Attempted` and NOT in `openedByUs` → add to `missingPairs`. Salvage calls `RescanSymbols({B})` — test stubs it to return empty (or a fresh opp; either is acceptable per §6 R4). Proves the new logic handles mixed-validity correctly: no duplicate open on A, B gets its rescan chance. |

### DB failure injection

Current engine wiring holds a **concrete** `*database.Client` — no `StateStore` /
wrapper interface exists (verified: `grep -n "e.db " internal/engine/engine.go`
shows direct field access; no interface indirection). No `miniredis.SetError(...)`
helper is used in the current engine tests (verified via `rg -n "SetError" internal/`).

**The approach** (single explicit path, no branches):

Add a minimal `activePositionsGetter` interface on `*Engine` plus a new field:

```go
// internal/engine/engine.go
type activePositionsGetter interface {
    GetActivePositions() ([]*models.ArbitragePosition, error)
}

type Engine struct {
    ...existing fields...
    // activePosSource is used ONLY by the staged entry-scan path's retry wrapper.
    // Production: set to e.db in the constructor. Tests: override to a failing
    // stub to exercise the retry + salvage branches.
    activePosSource activePositionsGetter
}
```

Constructor change — a single line:
```go
e.activePosSource = e.db  // in NewEngine after e.db is assigned
```

Call-site change in `getActivePositionsWithRetry`:
```go
positions, err := e.activePosSource.GetActivePositions()
```

The existing 14 call sites of `e.db.GetActivePositions()` outside the staged entry
scan remain unchanged (zero-blast-radius scope). Only the two call sites inside the
Stage 2 + salvage block are routed through `e.activePosSource` via the retry helper.

Test stub:
```go
type failingActivePosGetter struct {
    real     activePositionsGetter // delegates to miniredis-backed *database.Client
    failN    int                    // fail the first N calls, then delegate
    calls    int
}

func (f *failingActivePosGetter) GetActivePositions() ([]*models.ArbitragePosition, error) {
    f.calls++
    if f.calls <= f.failN {
        return nil, errors.New("forced: db unavailable")
    }
    return f.real.GetActivePositions()
}
```

**Explicit non-goals:**
- No package-level globals.
- No monkey-patching.
- No interface on other unrelated `*database.Client` methods — we are not
  refactoring the engine-db seam, only adding the minimum hook for this failure
  path.
- Tests that do NOT need failure injection (T1, T3, T5, T6, T7) use the default
  `e.activePosSource = e.db` path; they must not see any behavior difference vs.
  the current code.

### Coverage target

- `getActivePositionsWithRetry`:
  - all-fail path hit in T2a/T4 (failN=2, both attempts fail).
  - **retry-recovery path hit in T2b** (failN=1, first fails, second succeeds) — THIS is the test that proves the retry helper itself works, vs just the salvage fallback.
  - success-on-first-try path hit in T1/T3/T5/T6/T7/T8.
- Stage 2 + salvage branches: all 9 matrix cells hit at least one unique code path.
- `stage2Attempted` decision branches:
  - `stage2Attempted=empty` + in-scan symbol → salvage rescans (T2a).
  - `stage2Attempted={sym}` + salvage loop sees sym → skip (T1, T7 for B, T2b).
  - `stage2Attempted` populated even when `openedS1` dropped the symbol (T5, T7 for A).
  - **Mixed-validity**: `stage2Attempted` is a PARTIAL subset of `overrideSnapshot`; salvage correctly handles the remainder (T8).

---

## 6. Risks & Mitigations

| Risk | Severity | Mitigation |
|---|---|---|
| **R1**: Duplicate open — salvage rescans `B`, but Stage 2 also executed `B` if DB partially succeeded | Medium | `stage2Attempted[B]` is set unconditionally once DB read succeeds, BEFORE executeArbitrage. Salvage checks `stage2Attempted` → skips. Verified by T1. |
| **R2**: Retry amplifies Redis load | Low | `maxAttempts=2`, `backoff=300ms`. Entry scan runs once per minute at :40. Worst-case 600ms added latency. No thundering herd because single-goroutine. |
| **R3**: Salvage's own DB read also fails (T4) | Low | Same as current behavior — log warn, defer to next cycle. No regression. |
| **R4**: `applyAllocatorOverrides` filters out a symbol as stale → salvage retries it | Low | `RescanSymbols` applies fresh freshness checks independently. Two outcomes: (a) symbol is still stale → rescan returns empty → "failed re-scan" warn path, no trade opened; or (b) pair data changed between applyAllocatorOverrides and salvage → rescan passes → a trade may open with fresh data. Both outcomes are acceptable and desired — this is the whole point of salvage. Wasted work in case (a): a few HTTP calls to exchange REST. No state corruption in either case. |
| **R5**: A symbol Stage 1 *attempted* but rejected (risk/slippage) — salvage rescans it | Medium | Salvage only iterates `overrideSnapshot`, not all of `result.Opps`. So only allocator-selected symbols are rescanned. `RescanSymbols` re-checks risk/slippage; a symbol rejected by Stage 1 on deterministic grounds will be rejected again. Non-deterministic rejections (e.g., orderbook slippage) may succeed the second time — this is actually desirable for fund-stranded symbols. Accept as current behavior. |
| **R6**: `time.Sleep(300ms)` inside the engine main loop blocks other handlers | Low | The main loop already performs serial handler work inside the `select` case. Adding 300ms × at most 2 reads = ≤600ms is negligible relative to `executeArbitrage` itself, which routinely exceeds 1s. Verified by log grep: `entry stage-\d:.*executing` timings in production. |

---

## 7. Deletion / cleanup

No deletions. This is an additive fix within an existing block. The `inScan`
**variable name** is kept in scope but repurposed would be confusing, so the
rewrite simply drops it and uses `stage2Attempted` from the start. No cross-file
refs to `inScan` exist (it's a local).

Verify with: `rg -n "inScan" internal/engine/ | rg -v _test.go` → after edit,
should return 0 matches inside `engine.go` (safe because `inScan` only appears
in this block).

---

## 8. Implementation Steps

1. **Step 0** — Add `activePositionsGetter` interface + `activePosSource` field +
   constructor wire-up (`e.activePosSource = e.db`). Verify `go build ./...` still
   green — no behavior change yet.
2. **Step 1** — Add `getActivePositionsWithRetry` helper near `applyAllocatorOverrides`
   (`internal/engine/engine.go:~760`). Uses `e.activePosSource`.
3. **Step 2** — Rewrite Stage 2 + salvage block (`internal/engine/engine.go:888-988`)
   per pseudocode in §3.3. Single commit, diff stays local to this 100-line region.
4. **Step 3** — `go build ./...` + existing `go test ./internal/engine/...` → must stay green.
5. **Step 4** — Create `internal/engine/engine_stage2_salvage_test.go` with tests T1, T2a, T2b, T3, T4, T5, T6, T7, T8 (9 tests). Include `buildStage2SalvageEngine` builder that explicitly sets `e.activePosSource = e.db` (see §5).
6. **Step 5** — `go test ./internal/engine/ -run 'Stage2Salvage' -v` → 9/9 green.
7. **Step 6** — Full `go test ./...` + full `go vet ./...` green.
8. **Step 7** — `rg -n "inScan" internal/engine/engine.go` → 0 matches.
9. **Step 8** — Post-implementation review: `/codex:review` (follow
   feedback_review_flow: normal → PASS → independent).
10. **Step 9** — Commit with message pattern matching repo convention:
    `fix(allocator): salvage Stage 2 DB-failure in-scan overrides (codex d1ac2563 #1)`.

Not in scope (out-of-plan):
- Finding #2 MEDIUM test coverage for `applyAllocatorOverrides` itself —
  the new test file covers the end-to-end seam, which subsumes most of the
  applyAllocatorOverrides paths via T1/T3/T5. If codex still asks for a direct
  unit test on applyAllocatorOverrides, follow up with an additive patch.

---

## 9. Verification checklist (before "ALL PASS")

- [ ] Plan written
- [ ] Plan reviewed by codex (normal review) — PASS
- [ ] Plan reviewed by codex (independent review) — PASS
- [ ] Only then: implementation
- [ ] Post-implementation review — PASS
- [ ] `go build ./...` green
- [ ] `go test ./...` green
- [ ] `go vet ./...` green
- [ ] `rg -i argmax internal cmd web/src` = 0 (sanity check — this plan touches
      allocator-adjacent code but should NOT re-introduce argmax)
