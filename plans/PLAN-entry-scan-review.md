# PLAN: :40 EntryScan Review Fixes

**Version**: v8
**Status**: ALL PASS ✓
**Review rounds**: v1→v2→v3→v4→v5→v6→v7→v8
**Source**: Codex xhigh review of EntryScan logic (2026-04-08)
**Scope**: 4 findings — 1 HIGH, 2 MEDIUM, 1 LOW

### Codex Review History
- v1: B PASS. A/C/D NEEDS-REVISION.
- v2: B PASS. A/C/D NEEDS-REVISION.
- v3: B PASS. A/C/D NEEDS-REVISION.
  - A: recordScanHistory only records, doesn't run filters on alts. DB spread history also needs alts.
  - C: GetSpotOppsCount lacks 20min freshness check. spot:opps:cache has all dashboard rows not just entryable.
  - D: config.go:120 ScanMinutes, :122 ExitScanMinute comments also wrong. ARCHITECTURE.md:197 ExitScan(:25) wrong. :323 ScanMinutes row wrong.
- v4: A/B/D PASS. C NEEDS-REVISION.
  - C: TTL<10min hardcoded staleness wrong — SpotFuturesScanIntervalMin is configurable.
    Should derive freshness from scan interval + slack, not fixed cutoff.
- v5: B PASS. A/C/D NEEDS-REVISION (xhigh review).
  - A: backtest prefetch only warms primary opps — alt backtest may fail-open.
    Also: alt temporary Opportunity needs OIRank for isPersistent() low-OI gate.
  - C: CA-04 is bidirectional but plan only threads capOverride for perp entry.
    Need to explicitly scope as perp-only or add spot side.
  - D: RebalanceScanMinute comment says default 20 but Load() sets 10.
- v6: A/B PASS. C/D NEEDS-REVISION.
  - C: Cold start — empty spot cache → spotHasOpps=false → frees cap wrongly. Need "unknown" state.
  - D: ARCHITECTURE.md:87 scan list has :00 which doesn't exist in actual defaults.
- v7: C PASS. D NEEDS-REVISION.
  - D: Changing Load() RebalanceScanMinute from 10→20 would break tests.
    Code default is 10, comment is wrong. Fix comment, not code.

---

## Finding A (HIGH): Allocator override bypasses scanner verification

### Problem
`applyAllocatorOverrides()` (engine.go:882) can replace the primary verified pair with
an `AlternativePair` from the ranker. Alternatives never pass through verifier, persistence,
volatility, or backtest filters. `oppKey` is pair-level (`symbol|long|short`).

### Fix Design (v4 — complete scanner-side verification + filtering)

**Architecture**: Alternatives are verified in `verifyOpportunity()`, recorded in both
in-memory history AND Redis spread history, and then the engine's override path checks
`alt.Verified` before using them. The scanner's filter chain itself doesn't need changes
because persistence/volatility are checked by the engine at override time via a new
exported `CheckPairFilters()` method.

**Why expose a scanner method**: The scanner owns `scanHistory` (in-memory) and
`isPersistent()`/`isSpreadVolatile()`/`backtestFundingHistory()`. The engine cannot
replicate this without duplicating state. A single exported method is the clean seam.

**Changes**:

1. **`models/opportunity.go`**: Add `NextFunding time.Time` and `Verified bool` to
   `AlternativePair`.

2. **`discovery/verifier.go`** — verify alternatives in `verifyOpportunity()`:
   After primary pair passes verification, iterate `opp.Alternatives`. For each alt,
   build a temporary `Opportunity` with alt's exchanges/rates and call `verifyOpportunity()`
   recursively (with empty Alternatives to avoid infinite recursion). Keep only passing
   alts, populate their `NextFunding` (exchange-verified) and `Verified = true`.

   **Performance note**: With `TopPairsPerSymbol=3` and ~20 symbols, worst case is
   20 * 2 * 2 = 80 extra `GetFundingRate()` calls, parallelized within each symbol's
   goroutine. May need to increase `verifyTimeout` from 10s to 15s.

   **OIRank**: Alt `Opportunity` must inherit parent's `OIRank` — `isPersistent()` uses
   it for the low-OI stability gate (scanner.go:580+).

3. **`discovery/scanner.go`** — `recordScanHistory()`:
   - Also record verified alternatives in in-memory `scanHistory` (their own `oppKey`).
   - Also include verified alternatives in `AddSpreadHistoryBatch()` by converting
     them to temporary `Opportunity` objects so the DB layer can persist their spread
     history to Redis. This ensures pair-keyed persistence is durable across restarts.

4. **`discovery/scanner.go`** — new exported `CheckPairFilters()` method:
   ```go
   // CheckPairFilters runs pair-keyed filters (persistence, volatility, backtest)
   // on a single opportunity. Returns empty string if OK, or rejection reason.
   func (s *Scanner) CheckPairFilters(opp models.Opportunity) string {
       if reason := s.isPersistent(opp); reason != "" {
           return reason
       }
       if reason := s.isSpreadVolatile(opp); reason != "" {
           return reason
       }
       if s.cfg.BacktestDays > 0 {
           if pass, reason := s.backtestFundingHistory(opp); !pass {
               return reason
           }
       }
       return ""
   }
   ```

5. **`discovery/scanner.go`** — `prefetchBacktestData()`:
   Include verified alternatives so their backtest cache is warm for override decisions.
   In the non-EntryScan prefetch path (scanner.go:981), build expanded list including
   alt pairs as synthetic Opportunity objects.

6. **`engine/engine.go`** — `applyAllocatorOverrides()`:
   After matching an alternative:
   - Check `alt.Verified` — skip unverified.
   - Build temporary `Opportunity` with alt's fields + parent's `OIRank`.
   - Call `scanner.CheckPairFilters(altOpp)` — skip if rejected.
   - Patch `o.NextFunding = alt.NextFunding`.
   - Keep interval + funding window as defense-in-depth.

   **Dependency**: Engine needs scanner reference. Check if it already has one.

7. **`discovery/scanner.go`** — `CheckPairFilters()`:
   Use `backtestFundingHistoryRequireCached(opp, true)` instead of `backtestFundingHistory()`.
   This does an inline fetch on cache miss rather than failing open, ensuring alternatives
   get a real backtest decision even if prefetch didn't warm their cache yet.

### Files Modified
- `internal/models/opportunity.go` — add NextFunding, Verified to AlternativePair
- `internal/discovery/verifier.go` — verify alternatives
- `internal/discovery/scanner.go` — record alt history (memory + Redis), CheckPairFilters()
- `internal/engine/engine.go` — check alt.Verified + CheckPairFilters in override path

### Pseudo-code

```go
// models/opportunity.go
type AlternativePair struct {
    // ... existing fields ...
    NextFunding time.Time `json:"next_funding"`
    Verified    bool      `json:"verified"`
}

// verifier.go — inside verifyOpportunity(), after primary passes all checks:
if len(opp.Alternatives) > 0 {
    var verifiedAlts []models.AlternativePair
    for _, alt := range opp.Alternatives {
        altOpp := models.Opportunity{
            Symbol:        opp.Symbol,
            LongExchange:  alt.LongExchange,
            ShortExchange: alt.ShortExchange,
            LongRate:      alt.LongRate,
            ShortRate:     alt.ShortRate,
            IntervalHours: alt.IntervalHours,
            OIRank:        opp.OIRank, // inherit parent's OIRank for isPersistent() low-OI gate
            Source:        opp.Source,
        }
        // Empty Alternatives to avoid recursion
        pass, enriched, _ := s.verifyOpportunity(altOpp)
        if pass {
            alt.NextFunding = enriched.NextFunding
            alt.Verified = true
            verifiedAlts = append(verifiedAlts, alt)
        }
    }
    opp.Alternatives = verifiedAlts
}

// scanner.go recordScanHistory() — record alts too
func (s *Scanner) recordScanHistory(opps []models.Opportunity) {
    now := time.Now()
    s.scanHistoryMu.Lock()
    for _, opp := range opps {
        key := oppKey(opp)
        s.scanHistory[key] = append(s.scanHistory[key], scanRecord{Time: now, Spread: opp.Spread})
        for _, alt := range opp.Alternatives {
            if !alt.Verified { continue }
            altKey := opp.Symbol + "|" + alt.LongExchange + "|" + alt.ShortExchange
            s.scanHistory[altKey] = append(s.scanHistory[altKey], scanRecord{Time: now, Spread: alt.Spread})
        }
    }
    // ... prune logic (unchanged) ...
    s.scanHistoryMu.Unlock()

    // Persist to Redis — include verified alternatives as synthetic opps
    var allOpps []models.Opportunity
    allOpps = append(allOpps, opps...)
    for _, opp := range opps {
        for _, alt := range opp.Alternatives {
            if !alt.Verified { continue }
            allOpps = append(allOpps, models.Opportunity{
                Symbol:        opp.Symbol,
                LongExchange:  alt.LongExchange,
                ShortExchange: alt.ShortExchange,
                Spread:        alt.Spread,
                IntervalHours: alt.IntervalHours,
            })
        }
    }
    if err := s.db.AddSpreadHistoryBatch(allOpps, now); err != nil {
        s.log.Error("failed to persist spread history: %v", err)
    }
}

// scanner.go CheckPairFilters — exported for engine use
// Uses backtestFundingHistoryRequireCached with inline fetch enabled,
// so alt pairs get a real backtest decision even on cache miss.
func (s *Scanner) CheckPairFilters(opp models.Opportunity) string {
    if reason := s.isPersistent(opp); reason != "" {
        return reason
    }
    if reason := s.isSpreadVolatile(opp); reason != "" {
        return reason
    }
    if s.cfg.BacktestDays > 0 {
        if pass, reason := s.backtestFundingHistoryRequireCached(opp, true); !pass {
            return reason
        }
    }
    return ""
}

// engine.go applyAllocatorOverrides — after matching alt:
if !alt.Verified {
    e.log.Warn("entry: override %s alt %s/%s not verified, skipping", o.Symbol, alt.LongExchange, alt.ShortExchange)
    stale++
    continue
}
// Pair-level filter check via scanner
altOpp := models.Opportunity{
    Symbol:        o.Symbol,
    LongExchange:  alt.LongExchange,
    ShortExchange: alt.ShortExchange,
    Spread:        alt.Spread,
    IntervalHours: alt.IntervalHours,
    OIRank:        o.OIRank, // inherit for isPersistent() low-OI gate
}
if reason := e.scanner.CheckPairFilters(altOpp); reason != "" {
    e.log.Warn("entry: override %s alt %s/%s rejected by pair filter: %s", o.Symbol, alt.LongExchange, alt.ShortExchange, reason)
    stale++
    continue
}
// Patch NextFunding from verified alt
if !alt.NextFunding.IsZero() {
    o.NextFunding = alt.NextFunding
}
```

---

## Finding B (MEDIUM): NextFunding semantic mismatch — PASS

Use `opp.NextFunding` in `createPendingPosition()` with fallback to `computeNextFunding()`.

### Files Modified
- `internal/engine/capital.go`

### Pseudo-code

```go
func (e *Engine) createPendingPosition(opp models.Opportunity) *models.ArbitragePosition {
    nf := opp.NextFunding
    if nf.IsZero() {
        nf = e.computeNextFunding(opp.Symbol, opp.LongExchange, opp.ShortExchange)
    }
    now := time.Now().UTC()
    return &models.ArbitragePosition{
        // ... existing fields ...
        NextFunding: nf,
        // ...
    }
}
```

---

## Finding C (MEDIUM): dynamicStrategyPct computed but not enforced

### Fix Design (v4 — revised per Codex feedback)

**Scope**: Perp-only in this plan. CA-04 is bidirectional (perp↔spot), but spot engine
has its own discovery loop and doesn't use the perp EntryScan path. Spot-side dynamic
shifting is a separate future enhancement — the spot engine would need its own
`perpHasOpps` signal and `dynamicCap` threading. Explicitly out of scope for this fix.

**Approach**: `ReserveWithCap(strategy, exposures, capOverride float64)` — pure arg.

**v4 corrections from v3**:

1. **`GetSpotEntryableOppsCount` freshness**: Derive staleness from configurable scan interval,
   not hardcoded TTL cutoff. The cache key has 30min TTL, but `SpotFuturesScanIntervalMin`
   is configurable (default 10min, could be 20+min).
   - Staleness rule: `remaining TTL < (30min - 2*scanInterval)`. This means if scanInterval=10min,
     stale when TTL<10min (written >20min ago). If scanInterval=20min, stale when TTL<-10min
     (never stale within TTL — correct, since one scan per 20min is expected).
   - **Simpler approach (chosen)**: Pass `scanInterval` to the DB method. Stale if key age >
     `2 * scanInterval`. Key age = `30min - remaining_TTL`. So: stale if `TTL < 30min - 2*scanInterval`.
     If scanInterval is 0 or TTL check fails, default to stale (conservative).
   - Also: only count entries where `FilterStatus == ""` (entryable, not dashboard display rows).

2. **ManualOpen**: Passes `dynamicCap=0` to `reservePerpCapital()`.

### Changes

1. **`risk/allocator.go`**:
   - `ReserveWithCap(strategy, exposures, capOverride float64)`.
   - `checkCapsWithOverride(state, strategy, exposures, capOverride float64)`.
   - `Reserve()` wraps `ReserveWithCap(..., 0)`.

2. **`engine/capital.go`**: `reservePerpCapital(opp, approval, dynamicCap)`.

3. **`engine/engine.go`**: `executeArbitrage(opps, dynamicCap)`. ManualOpen passes 0.

4. **`database/spot_state.go`**: `GetSpotEntryableOppsCount(maxAge time.Duration) (int, error)`:
   - Read `spot:opps:cache`.
   - Check remaining TTL. Compute key age = `30min - TTL`. If age > maxAge, return 0.
   - Unmarshal JSON array, count entries where `filter_status` is empty string.
   - Return count.

5. **`engine/engine.go` EntryScan handler**:
   Three states: `spotHasOpps=true` (real opps), `false` (confirmed no opps), 
   `unknown` (cold start / stale cache → keep static split, don't free cap).
   ```go
   spotHasOpps := e.cfg.SpotFuturesEnabled // default: conservative (unchanged from current)
   if e.cfg.SpotFuturesEnabled {
       scanInterval := time.Duration(e.cfg.SpotFuturesScanIntervalMin) * time.Minute
       if scanInterval < time.Minute {
           scanInterval = 10 * time.Minute
       }
       maxAge := 2 * scanInterval
       count, err := e.db.GetSpotEntryableOppsCount(maxAge)
       if err == nil && count >= 0 {
           // count >= 0 means cache exists and is fresh (not stale/missing)
           // count == 0 means fresh cache with no entryable opps → free cap
           // count > 0 means spot has opps → keep cap reserved
           spotHasOpps = count > 0
       }
       // else: cache missing/stale/error → keep conservative default (spotHasOpps=true)
   }
   ```
   
   **Key insight**: On cold start, `spot:opps:cache` doesn't exist → `GetSpotEntryableOppsCount`
   returns error/negative → we keep `spotHasOpps = true` (conservative, same as current behavior).
   Only when we have a fresh cache showing 0 entryable opps do we set `spotHasOpps = false`.

### Files Modified
- `internal/risk/allocator.go` — ReserveWithCap, checkCapsWithOverride
- `internal/engine/engine.go` — thread dynamicCap, fix spotHasOpps
- `internal/engine/capital.go` — reservePerpCapital accepts dynamicCap
- `internal/database/spot_state.go` — GetSpotEntryableOppsCount

### Pseudo-code

```go
// allocator.go
func (a *CapitalAllocator) ReserveWithCap(strategy Strategy, exposures map[string]float64, capOverride float64) (*CapitalReservation, error) {
    // Same as Reserve but uses checkCapsWithOverride
}

func (a *CapitalAllocator) Reserve(strategy Strategy, exposures map[string]float64) (*CapitalReservation, error) {
    return a.ReserveWithCap(strategy, exposures, 0)
}

func (a *CapitalAllocator) checkCapsWithOverride(state allocatorState, strategy Strategy, exposures map[string]float64, capOverride float64) error {
    totalCap := a.cfg.MaxTotalExposureUSDT
    if totalCap <= 0 { return nil }
    requestTotal := 0.0
    for _, amount := range exposures { requestTotal += amount }
    if requestTotal <= 0 { return nil }
    if state.total+requestTotal > totalCap {
        return fmt.Errorf("total capital cap exceeded: ...")
    }
    pct := a.strategyPct(strategy)
    if capOverride > 0 && capOverride > pct {
        pct = capOverride
    }
    strategyCap := totalCap * pct
    if strategyCap > 0 && state.byStrategy[strategy]+requestTotal > strategyCap {
        return fmt.Errorf("%s cap exceeded: ...", strategy)
    }
    // exchange cap check unchanged
}

// database/spot_state.go
const spotOppsCacheTTL = 30 * time.Minute // must match spotengine SetWithTTL value

// GetSpotEntryableOppsCount returns the count of entryable spot opportunities.
// Returns (count, nil) when cache is fresh — count may be 0 (confirmed no opps).
// Returns (-1, err) when cache is missing, expired, or stale — caller should treat
// as "unknown" and keep conservative defaults.
func (c *Client) GetSpotEntryableOppsCount(maxAge time.Duration) (int, error) {
    ctx := context.Background()
    ttl, err := c.rdb.TTL(ctx, "spot:opps:cache").Result()
    if err != nil || ttl <= 0 {
        return -1, fmt.Errorf("spot cache missing or expired") // unknown state
    }
    age := spotOppsCacheTTL - ttl
    if maxAge > 0 && age > maxAge {
        return -1, fmt.Errorf("spot cache stale (age=%v > max=%v)", age, maxAge) // unknown
    }
    raw, err := c.rdb.Get(ctx, "spot:opps:cache").Result()
    if err != nil { return -1, err }
    var opps []struct {
        FilterStatus string `json:"filter_status"`
    }
    if err := json.Unmarshal([]byte(raw), &opps); err != nil {
        return -1, err
    }
    count := 0
    for _, o := range opps {
        if o.FilterStatus == "" {
            count++
        }
    }
    return count, nil // fresh cache, confirmed count (may be 0)
}
```

---

## Finding D (LOW): Schedule comment/config mismatch

### All locations to fix (v4 — complete sweep)

Actual defaults (config.go:513-567):
- `ScanMinutes: [10, 20, 30, 35, 40, 45, 50]`
- `RebalanceScanMinute: 10` (Load() default, but comment says 20 — MISMATCH)
- `ExitScanMinute: 30`
- `EntryScanMinute: 40`
- `RotateScanMinute: 35`

**RebalanceScanMinute decision**: Comment says "default 20", but `Load()` defaults to 10
and tests verify 10 (`config_test.go:14,38`). The live `config.json` overrides to 20 via
`Fund.RebalanceScanMinute`. The code default of 10 is intentional as base; the comment is
wrong. Fix: change comment from "default 20" to "default 10". Do NOT change `Load()` default
— that would break tests and the fallback semantics.

| File | Line | Current | Fix to |
|------|------|---------|--------|
| config.go | 120 | `default [5,15,25,35,45,55]` | `default [10,20,30,35,40,45,50]` |
| config.go | 121 | `default 35` (entry) | `default 40` |
| config.go | 122 | `default 25` (exit) | `default 30` |
| config.go | 123 | `default 45` (rotate) | `default 35` |
| config.go | 36 | comment `default 20` | comment `default 10` (matches Load() and tests) |
| ARCHITECTURE.md | 87 | `:00, :10, :20, :30, :35, :40, :50` | `:10, :20, :30, :35, :40, :45, :50` (remove :00, add :45) |
| ARCHITECTURE.md | 197 | `ExitScan (:25) and EntryScan (:35)` | `ExitScan (:30) and EntryScan (:40)` |
| ARCHITECTURE.md | 323 | `ScanMinutes [5,15,25,35,45,55]` | `ScanMinutes [10,20,30,35,40,45,50]` |
| ARCHITECTURE.md | 325 | `EntryScanMinute 35` | `EntryScanMinute 40` |
| ARCHITECTURE.md | 326 | `ExitScanMinute 25` | `ExitScanMinute 30` |
| ARCHITECTURE.md | 327 | `RotateScanMinute 45` | `RotateScanMinute 35` |
| ARCHITECTURE.md | 430 | `:20 rebalance, :30 exit, :35 rotate, :40 entry` | already correct! |
| CLAUDE.md | schedule table | `:25 exit, :35 entry, :45 rotate` | `:30 exit, :35 rotate, :40 entry` |

### Files Modified
- `internal/config/config.go` — fix comments lines 120-123
- `ARCHITECTURE.md` — fix lines 197, 323, 325, 326, 327
- `CLAUDE.md` — fix schedule table

---

## Implementation Order

1. **D** (LOW) — comment/doc fix, zero risk
2. **B** (MEDIUM) — NextFunding pass-through, low risk
3. **A** (HIGH) — scanner-side alt verification + CheckPairFilters, medium risk
4. **C** (MEDIUM) — ReserveWithCap + spotHasOpps, highest complexity

## Pre-implementation Checklist
- [ ] Plan v8 reviewed by Codex (ALL PASS)?
- [ ] Engine has scanner reference for CheckPairFilters call?
- [ ] Allocator tests cover ReserveWithCap path?
- [ ] Verifier tests cover alternative verification?
- [ ] `go build` passes after each fix?
