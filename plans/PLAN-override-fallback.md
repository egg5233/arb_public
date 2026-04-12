# Plan: Override Fallback When Discovery Returns 0 (v6)

## Problem

:45 allocator 選了機會並轉帳，但 :55 discovery 回 0 → override 沒被用 → 錢白轉。

## v5 Rejection (7 findings)

1. CRITICAL: `e.discovery` 是 `models.Discoverer` interface，沒有 `RescanSymbols`
2. CRITICAL: `SymbolPair` 放 discovery 會 import cycle
3. HIGH: 不能呼叫 `runCycleInternal`（會 double-send oppChan deadlock）
4. HIGH: 漏了 `recordScanHistory`（persistence filter 需要）
5. MEDIUM: TopN 截斷可能丟掉 override symbol
6. MEDIUM: `Poll()` 回 `*LorisResponse` 不是 `[]Opportunity`

## v6 Approach

1. 在 `models` 定義 `SymbolPair` + `SymbolRescanner` interface
2. Scanner 實作 `RescanSymbols`，內部直接呼叫 sub-steps（不走 `runCycleInternal`）
3. 提取 `applyEntryFilters` helper（persistence/volatility/cooldown/interval/funding/backtest/delist）讓 `runCycleInternal` 和 `RescanSymbols` 共用
4. RescanSymbols 明確呼叫 `recordScanHistory`
5. Symbol filter 在 rank 後、topN 前套用
6. Engine 用 type-assert 取得 `SymbolRescanner`

## Changes

### Step 1: models — 定義 types + interface

**File**: `internal/models/interfaces.go`

```go
// SymbolPair identifies a specific exchange pair for re-scanning.
type SymbolPair struct {
    Symbol        string
    LongExchange  string
    ShortExchange string
}

// SymbolRescanner can re-scan specific symbols through the full pipeline.
// Implemented by Scanner when the allocator override fallback needs fresh data.
type SymbolRescanner interface {
    RescanSymbols(pairs []SymbolPair) []Opportunity
}
```

### Step 2: scanner — 提取 `applyEntryFilters`

**File**: `internal/discovery/scanner.go`

從 `runCycleInternal` 的 filter chain（~line 894-1035）提取為獨立方法：

```go
// applyEntryFilters runs the full entry filter chain on verified opps.
// Used by both runCycleInternal and RescanSymbols.
func (s *Scanner) applyEntryFilters(verified []models.Opportunity, scanType ScanType) []models.Opportunity {
    // persistence filter
    // volatility filter  
    // cooldown check (isSymbolCoolingDown)
    // interval check
    // funding window filter (for EntryScan/RebalanceScan/RotateScan)
    // backtest filter
    // delist check
    return filtered
}
```

`runCycleInternal` 改為呼叫 `s.applyEntryFilters(verified, scanType)`。

### Step 3: scanner — 實作 `RescanSymbols`

**File**: `internal/discovery/scanner.go`

```go
func (s *Scanner) RescanSymbols(pairs []models.SymbolPair) []models.Opportunity {
    // 1. Build requested symbol set
    requested := make(map[string]models.SymbolPair)
    for _, p := range pairs {
        requested[p.Symbol] = p
    }

    // 2. Poll fresh rates (returns *LorisResponse)
    loris, err := s.Poll()
    if err != nil {
        s.log.Error("RescanSymbols: poll failed: %v", err)
        return nil
    }

    // 2b. Update interval cache (same as runCycleInternal:829-834)
    if loris.FundingIntervals != nil {
        s.intervalsMu.Lock()
        s.lastLorisIntervals = loris.FundingIntervals
        s.intervalsMu.Unlock()
    }

    // NOTE: This poll is Loris-only. CoinGlass-sourced symbols will not
    // be found and will gracefully return nil with a log message. This is
    // acceptable degradation — CoinGlass overrides are rare and the log
    // provides visibility.

    // 3. Rank (returns []models.Opportunity with OIRank, Source, metadata)
    ranked := s.RankOpportunities(loris)

    // 4. Filter to requested symbols BEFORE topN cut
    var targetOpps []models.Opportunity
    for _, opp := range ranked {
        if _, ok := requested[opp.Symbol]; ok {
            targetOpps = append(targetOpps, opp)
        }
    }
    if len(targetOpps) == 0 {
        s.log.Info("RescanSymbols: none of %d requested symbols found in poll", len(pairs))
        return nil
    }

    // 5. Verify rates (sign, magnitude, divergence)
    verified := s.VerifyRates(targetOpps)

    // 6. Record scan history (needed for persistence filter)
    s.recordScanHistory(verified)

    // 7. Apply full entry filter chain
    filtered := s.applyEntryFilters(verified, EntryScan)

    // 8. Post-filter to specific exchange pairs from overrides
    var result []models.Opportunity
    for _, opp := range filtered {
        p, ok := requested[opp.Symbol]
        if !ok {
            continue
        }
        // Check primary pair
        if opp.LongExchange == p.LongExchange && opp.ShortExchange == p.ShortExchange {
            result = append(result, opp)
            continue
        }
        // Check alternatives
        for _, alt := range opp.Alternatives {
            if alt.LongExchange == p.LongExchange && alt.ShortExchange == p.ShortExchange && alt.Verified {
                // Patch to use alt pair (same as applyAllocatorOverrides)
                opp.LongExchange = alt.LongExchange
                opp.ShortExchange = alt.ShortExchange
                opp.LongRate = alt.LongRate
                opp.ShortRate = alt.ShortRate
                opp.Spread = alt.Spread
                if alt.IntervalHours > 0 {
                    opp.IntervalHours = alt.IntervalHours
                }
                if !alt.NextFunding.IsZero() {
                    opp.NextFunding = alt.NextFunding
                }
                // Run CheckPairFilters on the alt pair
                altOpp := opp
                if reason := s.CheckPairFilters(altOpp); reason != "" {
                    s.log.Info("RescanSymbols: %s alt %s/%s rejected: %s", opp.Symbol, alt.LongExchange, alt.ShortExchange, reason)
                    break
                }
                result = append(result, opp)
                break
            }
        }
    }

    s.log.Info("RescanSymbols: %d/%d symbols passed re-scan", len(result), len(pairs))
    return result
}
```

注意：不呼叫 `runCycleInternal`，不 send oppChan，不 deadlock。

### Step 4: engine — fallback 路徑

**File**: `internal/engine/engine.go` — ~line 1221

```go
} else {
    e.allocOverrideMu.Lock()
    overrides := e.allocOverrides
    e.allocOverrides = nil
    e.allocOverrideMu.Unlock()

    if len(overrides) > 0 {
        e.log.Info("entry: discovery returned 0 opps but %d allocator overrides exist, re-scanning", len(overrides))

        // Type-assert for RescanSymbols capability
        rescanner, ok := e.discovery.(models.SymbolRescanner)
        if !ok {
            e.log.Warn("entry: discovery does not support RescanSymbols, skipping override fallback")
        } else {
            var pairs []models.SymbolPair
            for symbol, choice := range overrides {
                pairs = append(pairs, models.SymbolPair{
                    Symbol:        symbol,
                    LongExchange:  choice.longExchange,
                    ShortExchange: choice.shortExchange,
                })
            }

            fallbackOpps := rescanner.RescanSymbols(pairs)

            if len(fallbackOpps) > 0 {
                e.log.Info("entry: override fallback: %d/%d passed, executing", len(fallbackOpps), len(pairs))
                e.executeArbitrage(fallbackOpps, perpCap)
            } else {
                e.log.Info("entry: override fallback: all %d symbols failed re-scan", len(pairs))
            }
        }
    }
}
```

## Files Changed

| File | Change |
|------|--------|
| `internal/models/interfaces.go` | 新增 `SymbolPair` + `SymbolRescanner` interface |
| `internal/discovery/scanner.go` | 提取 `applyEntryFilters`；新增 `RescanSymbols` |
| `internal/engine/engine.go` | ~line 1221: else 分支 type-assert + RescanSymbols |

## All v5 Issues Resolved

| # | Issue | Resolution |
|---|-------|-----------|
| 1 | Interface gap | `SymbolRescanner` interface in models, type-assert in engine |
| 2 | Import cycle | `SymbolPair` in models (not discovery) |
| 3 | oppChan deadlock | 直接呼叫 sub-steps，不走 runCycleInternal |
| 4 | recordScanHistory | 明確呼叫，在 VerifyRates 後 |
| 5 | TopN truncation | Symbol filter 在 rank 後、topN 前（不經過 topN） |
| 6 | Poll type mismatch | `s.Poll()` → `s.RankOpportunities(loris)` → filter slice |

## Known Limitations

1. **CoinGlass-sourced symbols**: `RescanSymbols` only polls Loris. If the override symbol was originally from CoinGlass, the re-scan will not find it and returns nil gracefully with a log message. This is rare and acceptable.
2. **lastLorisIntervals cache**: Updated in `RescanSymbols` after `Poll()` (same as `runCycleInternal:829-834`) to keep interval data fresh.
