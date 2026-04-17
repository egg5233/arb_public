# Plan: SIRENUSDT 3-bug fix — retrySecondLeg VWAP + reconcile guard + SL/TP detection
Version: v13
Date: 2026-04-17
Status: ALL PASS — READY TO IMPLEMENT

## Incident Summary

Position `sirenusdt-1776365702568` (2026-04-16 18:55 UTC):
- Real exchange PnL = **-$1.39**, stored realized_pnl = **+$2385.98**
- Root: `retrySecondLeg` MKT fill recorded synthetic BBO-mid price (~$41.56 due to bad BingX BBO snapshot) → ShortEntry VWAP inflated to $14.28 (real ~$1.58) → reconcile guard then rejected the correct -$2387 diff because it "exceeds notional"

Position `sirenusdt-1776268502636` (2026-04-15 15:55 → 2026-04-16 15:29 UTC):
- Price rose $0.78 → $1.02 (+31%), binance TP + bingx SL fired simultaneously
- Binance TP fill: orderID=3561813146 (≠ stored long_tp_order_id=3000001251209638), reduceOnly=false → both detection methods miss
- Bingx WS emitted zero events at SL fire time → private WS delivery loss (not callback drop)
- Consolidator cleaned up 10s later

## Changes

### Fix H1A — retrySecondLeg uses real avg fill price via existing GetOrderUpdate path
**Rewritten per codex v1 finding: do NOT invent `GetOrderAvgPrice`. Use existing `GetOrderUpdate` + REST fallback pattern already proven in `confirmFill()` at `internal/engine/engine.go:4320-4371`.**

- `internal/engine/engine.go:2763-2774` (IOC retry branch): after `GetOrderFilledQty`, additionally call `exch.GetOrderUpdate(orderID)` to read `AvgPrice` from the order store. If AvgPrice > 0, use it; if AvgPrice is 0 (no WS update yet), sleep 200ms and retry `GetOrderUpdate`; if still 0, fall back to `orderPrice` (limit price, reasonable approximation for filled IOC).
- `internal/engine/engine.go:2811-2821` (MKT retry branch): same GetOrderUpdate path; if AvgPrice unavailable after retries, fall back to `refPrice` (last known reference price) — NOT `(bbo.Bid + bbo.Ask) / 2`.

### Fix H1B — retrySecondLeg BBO sanity clamp against refPrice
- `internal/engine/engine.go:2723-2738`: after `exch.GetBBO(symbol)`, reject BBO if any of: `bid <= 0`, `ask <= 0`, `bid > refPrice * 1.20`, `ask < refPrice * 0.80`, `ask > refPrice * 1.20`, `bid < refPrice * 0.80` (symmetric 20% envelope on BOTH legs). On reject, log warning and fall back to `exchange.BBO{Bid: refPrice, Ask: refPrice}` immediately (no BBO-refetch loop — simpler, since refPrice fallback is safe).
- Prevents absurd retry prices like 57.01 / 25.04 / 1.08.

### Fix H1C — DEFERRED as secondary hardening
**Per codex v1 finding: underspecified. Drop from this plan; file as separate follow-up.**

Follow-up spec for a future plan: in `pkg/exchange/bingx/ws.go:219-242`, before overwriting the priceStore BBO entry, read the prior value from `priceStore`. Reject the incoming update if `new.Bid > prior.Bid * 10 OR new.Bid < prior.Bid * 0.1 OR new.Ask > prior.Ask * 10 OR new.Ask < prior.Ask * 0.1` (symmetric order-of-magnitude guard on both sides). On reject, log warning and keep prior BBO.

### Fix H2 — reconcile guard via LongCloseSize/ShortCloseSize (intended-close-size) fields
**Rewritten per v9 codex finding: `InitialSize` with "immutable-from-birth" invariant is wrong because partial-close revert, rotation with partial fill, and startup duplicate merge all legitimately change the active position size. Also, the pre-migration fallback to `longOK && shortOK` is too weak for `closePositionWithMode` path which reconciles with non-zero live sizes. v10 uses `LongCloseSize`/`ShortCloseSize` = the position's current intended full close size, updated on every legitimate size-changing active path.**

#### H2.1 — Add LongCloseSize / ShortCloseSize fields
- `internal/models/position.go`: add two fields to `ArbitragePosition`:
  ```go
  LongCloseSize  float64 `json:"long_close_size"`   // IMMUTABLE across depth-exit zeroing; mutable on legitimate size-changing paths
  ShortCloseSize float64 `json:"short_close_size"`
  ```
- Semantics: the size the position intends to have at its next final close. Updated whenever the active position size legitimately changes (entry, revert, rotation accepting partial, startup merge). Not modified by depth-exit's zeroing.

#### H2.2 — Set/update CloseSize at all size-changing active paths
Per codex v10 exact citations:

1. **Active entry activation** (`internal/engine/engine.go:3122-3142` and `internal/engine/engine.go:4265-4286`):
   - Set `LongCloseSize = confirmedLong`, `ShortCloseSize = confirmedShort` when the position is first persisted as Active.
2. **Partial-close revert** (`internal/engine/exit.go:973-980`): when depth-exit goes back to Active with remainder sizes, update `LongCloseSize = longRemainder`, `ShortCloseSize = shortRemainder`.
3. **Accepted rotation resize / partial swap persistence** (`internal/engine/exit.go:2776-2779`, `2907-2918`, `2968-2986`): when rotation persists `openFilled < target` or otherwise resizes a leg, update the affected leg's `LongCloseSize` or `ShortCloseSize` to the new live size.
4. **Startup duplicate merge survivor** (`internal/engine/engine.go:2591-2603`): when merge grows the survivor's live size, update the survivor's CloseSize to the merged size.
5. **Depth-exit close path** (`internal/engine/exit.go:960-998`): DO NOT zero `LongCloseSize`/`ShortCloseSize`. Keep them equal to the size the position had going into the final close. Reconcile needs them preserved to gate against the raw exchange-aggregated CloseSize.
6. **Normal close path** `closePositionWithMode` (`internal/engine/exit.go:2008-2079`): Status=closed but LongSize/ShortSize remain non-zero; CloseSize was set at entry and stays in sync — no new write needed here.

#### H2.3 — Reconcile gate: Two-phase flow (per codex v11 exact spec)
`internal/engine/exit.go:1137-1189` restructure as TWO phases — Tier 1 pre-split completeness, then split, then Tier 2/3 post-split:

```go
// Existing longOK && shortOK check at :1149-1152 stays as-is.

const sizeEpsilon = 1e-6

// -------- Phase 1: pre-split Tier 1 completeness gate --------
longSiblings := e.siblingsFor(pos, "long")
shortSiblings := e.siblingsFor(pos, "short")
useTier1 := pos.LongCloseSize > 0 && pos.ShortCloseSize > 0 &&
    allSiblingsHaveCloseSize(longSiblings, "long") &&
    allSiblingsHaveCloseSize(shortSiblings, "short")

if useTier1 {
    longExpected := pos.LongCloseSize + sumSiblingCloseSize(longSiblings, "long")
    shortExpected := pos.ShortCloseSize + sumSiblingCloseSize(shortSiblings, "short")
    if longAgg.CloseSize < longExpected-sizeEpsilon || shortAgg.CloseSize < shortExpected-sizeEpsilon {
        e.log.Warn("reconcile %s [attempt %d]: incomplete close data (longRawClose=%.6f/%.6f shortRawClose=%.6f/%.6f), retrying",
            pos.ID, attempt, longAgg.CloseSize, longExpected, shortAgg.CloseSize, shortExpected)
        return false
    }
}

// -------- Phase 2: split + diff calculation (existing code) --------
longAgg = e.splitSharedPnL(longAgg, pos, "long")
shortAgg = e.splitSharedPnL(shortAgg, pos, "short")
reconciledPnL := longAgg.NetPnL + shortAgg.NetPnL + pos.RotationPnL
diff := reconciledPnL - pos.RealizedPnL

// -------- Phase 3: post-split Tier 2 / Tier 3 (only if not Tier 1) --------
if !useTier1 {
    if pos.LongSize > 0 || pos.ShortSize > 0 {
        // Tier 2: pre-migration with non-zero live size (closePositionWithMode path).
        // Retain old abs(diff) > notional guard for current position only; NO sibling reconstruction.
        longNotional := pos.LongEntry * pos.LongSize
        shortNotional := pos.ShortEntry * pos.ShortSize
        notional := math.Max(longNotional, shortNotional)
        if notional > 0 && math.Abs(diff) > notional {
            e.log.Warn("reconcile %s [attempt %d]: pre-migration diff %.4f exceeds notional %.4f, retrying",
                pos.ID, attempt, diff, notional)
            return false
        }
    } else {
        // Tier 3: pre-migration depth-exit (both CloseSize and LongSize zero).
        // Rely on longOK && shortOK (already checked at :1149-1152).
        e.log.Warn("reconcile %s [attempt %d]: pre-migration depth-exit, no size info — relying on longOK && shortOK",
            pos.ID, attempt)
    }
}

// REMOVE old guard block at :1177-1189 (replaced by tier logic above).
// Informational variance log AFTER tiers pass:
if pos.EntryNotional > 0 && math.Abs(longAgg.NetPnL+shortAgg.NetPnL) > pos.EntryNotional*0.5 {
    e.log.Warn("reconcile %s [attempt %d]: large delta-neutral variance long+short=%.4f vs entryNotional %.4f (informational, proceeding)",
        pos.ID, attempt, longAgg.NetPnL+shortAgg.NetPnL, pos.EntryNotional)
}
// Fall through to existing needsPnLUpdate / needsFundingUpdate / write path.
```

**Key v12 changes per codex v11:**
- Tier 1 is a PRE-SPLIT gate — runs before `splitSharedPnL` modifies CloseSize.
- Tier 2/3 are POST-SPLIT — they need `diff` which is only computed after split+reconciledPnL calculation.
- Single `useTier1` flag tracks which path was taken so we don't double-gate.
- Tier 2 keeps only the current-position `abs(diff) > notional` guard; NO sibling reconstruction from mixed history.

#### H2.4 — Helper extraction (v11 narrowed)
```go
// siblingsFor — extract from splitSharedPnL's filter loop (shared helper).
func (e *Engine) siblingsFor(pos *models.ArbitragePosition, side string) []*models.ArbitragePosition { ... }

// allSiblingsHaveCloseSize returns true iff every sibling has LongCloseSize (or ShortCloseSize) > 0.
func allSiblingsHaveCloseSize(siblings []*models.ArbitragePosition, side string) bool { ... }

// sumSiblingCloseSize returns sum of LongCloseSize/ShortCloseSize across siblings (Tier 1 only).
func sumSiblingCloseSize(siblings []*models.ArbitragePosition, side string) float64 { ... }

// NOTE: no sumSiblingLongSize helper — Tier 2 intentionally does not reconstruct
// sibling totals from history per codex v10 finding (mixed/zeroed history unreliable).
```

Refactor `splitSharedPnL` to use `siblingsFor` for consistent sibling discovery.

#### H2 unit tests (v13)
- `TestReconcileAcceptsDepthExitWithCloseSizeMatch` (Tier 1 H1 replay) — depth-exit zeros LongSize, CloseSize=189 preserved, rawCloseSize=189, local=+2385, exchange=-1.39 → correction applied.
- `TestReconcileRetriesWhenAggregationFails` — one side empty close records (longOK=false) → retry (pre-existing).
- `TestReconcileRetriesWhenCloseSizePartial` (Tier 1) — no siblings, CloseSize=385, rawCloseSize=100 → retry.
- `TestReconcileAcceptsSharedPositionCompleteClose` (Tier 1 shared) — 2 siblings each CloseSize=100, rawCloseSize=200 → passes.
- `TestReconcileRetriesSharedPositionPartial` (Tier 1 shared partial) — 2 siblings each CloseSize=100, rawCloseSize=50 → retry.
- `TestReconcileMixedHistoryFallsThroughToTier2` — pos CloseSize=100, one sibling CloseSize=0 → Tier 1 precondition fails, falls to Tier 2 current-position notional guard only (no sibling reconstruction).
- `TestReconcilePreMigrationNormalCloseRetainsNotionalGuard` (Tier 2) — CloseSize=0, LongSize=385, diff within notional → accepted; diff > notional → retry.
- `TestReconcileDepthExitPreMigrationFallback` (Tier 3) — CloseSize=0 AND LongSize=0 → accepts via longOK && shortOK only.
- `TestCloseSizeImmutableThroughDepthExit` — depth-exit zeros LongSize but CloseSize preserved in both position record and history entry.
- `TestReconcilePartialCloseRevertUpdatesCloseSize` — revert sets LongSize=remainder AND LongCloseSize=remainder.
- `TestReconcileRotationAcceptingPartialUpdatesCloseSize` — rotation `openFilled < target` sets CloseSize to `openFilled`.
- `TestReconcileStartupMergeUpdatesCloseSize` — duplicate merge survivor updates CloseSize to merged size.

### Fix H3A — widen slIndex to include TP IDs (full callsite update)
**Scope expanded per codex v1+v2 findings: include `slEntry` type, `rebuildSLIndex`, and `exit.go` rotation handler (which DOES replace TP per exit.go:3183-3208).**

- `internal/engine/engine.go:131-135`: rename `slEntry` → `stopOrderEntry`, add `Kind string` field (`"sl"` or `"tp"`).
- `internal/engine/engine.go:1479-1491`: rename `registerSLOrders` → `registerStopOrders`. Register all 4 ID fields (LongSLOrderID, ShortSLOrderID, LongTPOrderID, ShortTPOrderID) into `slIndex` with appropriate `Kind`.
- `internal/engine/engine.go:1493-1503`: rename `unregisterSLOrders` → `unregisterStopOrders`. Delete all 4 ID keys.
- `internal/engine/engine.go:1505-1520` (`handleSLFill`): after method 1 lookup hits, call `triggerEmergencyClose` regardless of `Kind` (both SL and TP fills should trigger close semantics).
- `internal/engine/engine.go:1661-1675` (`rebuildSLIndex`): rename to `rebuildStopIndex`, calls `registerStopOrders`. Log message updated.
- `internal/engine/engine.go:4600`: update call site to `registerStopOrders`.
- `internal/engine/exit.go:3229-3233` (rotation direct `slIndex` write): the rotation path at exit.go:3183-3208 places a new TP with `tpOID`. Current code only registers `oid` (new SL). v3 MUST register BOTH new SL and new TP IDs (if non-empty) under the widened `stopOrderEntry` schema.

### Fix H3B — Binance close-fill detection: field-tag fix + narrow heuristic + ALGO_UPDATE remap
**Rewritten per codex v1+v2 findings: current narrow heuristic does NOT catch SIRENUSDT incident because `PlaceTakeProfit` uses `closePosition=false` (adapter.go:1192). Must add ALGO_UPDATE handling to scope to resolve the actual incident path.**

#### H3B.1 — Fix Binance field tags
- `pkg/exchange/binance/ws_private.go:102-115`: fix the incorrect JSON tag. Binance docs (`doc/binance/binance-usds-futures-api-docs.md:11052-11098`) specify the reduce-only field as `R` in the `o` sub-object, but the current code uses `json:"ro"`. Change to `json:"R"`. Also add: `ClosePosition bool \`json:"cp"\`` and `OrigOrderType string \`json:"ot"\``.
- `pkg/exchange/binance/ws_private.go:128-136`: set `upd.ReduceOnly` using a narrow rule:
  ```go
  // ReduceOnly is true when the exchange explicitly marks the fill as reducing position,
  // OR when it results from a TRIGGERED conditional order with closePosition=true.
  // Do NOT broadly classify by orderType alone (STOP_MARKET without cp=true could be opening).
  upd.ReduceOnly = o.ReduceOnly || (o.ClosePosition && (o.OrigOrderType == "STOP_MARKET" || o.OrigOrderType == "TAKE_PROFIT_MARKET"))
  ```
- Fix log line to show `R` not `ro`.
- This covers binance SL paths (`PlaceStopLoss` uses `closePosition=true` at adapter.go:1166) but NOT TP (uses `closePosition=false` at adapter.go:1192). The next sub-fix handles TP.

#### H3B.2 — Handle Binance ALGO_UPDATE event for algoID → matching-engine orderID remap
**This is the actual fix for the SIRENUSDT incident path.**

Binance algo order flow (`doc/binance/binance-usds-futures-api-docs.md:10659-10713`):
1. `PlaceTakeProfit` / `PlaceStopLoss` returns `algoId` — stored as `LongTPOrderID` / `LongSLOrderID` etc.
2. When trigger price hits, Binance fires an ALGO_UPDATE with `X=TRIGGERED` and `ai` populated (matching-engine order ID).
3. Then Binance fires ORDER_TRADE_UPDATE with OrderID = that `ai`.

Current engine has NO ALGO_UPDATE handler (`grep ALGO_UPDATE pkg/exchange/binance` returns 0 matches).

##### H3B.2a — Define named interface in `pkg/exchange/types.go`
Following the existing pattern for `WSMetricsCallbackSetter` (types.go:26-29) and `OrderMetricsCallbackSetter` (types.go:50-53):

```go
// AlgoRemap carries an exchange's mapping from a previously-stored algo/conditional order ID
// to the matching-engine order ID that appears when the algo is triggered.
type AlgoRemap struct {
    AlgoID string // the ID we stored at placement time (e.g. Binance algoId)
    RealID string // the matching-engine order ID now active (e.g. Binance order i)
    Symbol string // symbol, for defensive filtering
}

// AlgoRemapCallback consumes ID-remap events so callers can alias stored IDs to live IDs.
type AlgoRemapCallback func(AlgoRemap)

// AlgoRemapCallbackSetter is implemented by adapters that expose algo-trigger remap events
// (currently Binance; other exchanges either use same ID on trigger or N/A).
type AlgoRemapCallbackSetter interface {
    SetAlgoRemapCallback(fn AlgoRemapCallback)
}
```

##### H3B.2b — Binance adapter implementation
- `pkg/exchange/binance/adapter.go`: add private field `algoRemapCallback exchange.AlgoRemapCallback` (guarded by `sync.Mutex algoRemapMu` or atomic pointer). Implement `SetAlgoRemapCallback(fn)`.
- `pkg/exchange/binance/ws_private.go`: in `handlePrivateMessage`, add branch `if base.EventType == "ALGO_UPDATE"` that parses:
  ```go
  var algo struct {
      Order struct {
          AlgoID        int64  `json:"aid"`
          RealID        string `json:"ai"`
          Symbol        string `json:"s"`
          AlgoStatus    string `json:"X"`
      } `json:"o"`
  }
  ```
  When `algo.Order.AlgoStatus == "TRIGGERED"` and `algo.Order.RealID != ""`, invoke the adapter's stored `algoRemapCallback` with `AlgoRemap{AlgoID: strconv.FormatInt(algo.Order.AlgoID, 10), RealID: algo.Order.RealID, Symbol: algo.Order.Symbol}`.

##### H3B.2c — Engine-side wiring (initial + reload)
Engine `Start()` path (in `internal/engine/engine.go` initialization sequence, runs once at startup):

```go
// After adapters are constructed but before entering main loop:
for name, exch := range e.exchanges {
    if setter, ok := exch.(exchange.AlgoRemapCallbackSetter); ok {
        exchName := name
        setter.SetAlgoRemapCallback(func(remap exchange.AlgoRemap) {
            e.handleAlgoRemap(exchName, remap)
        })
    }
}
```

`handleAlgoRemap` (new method on Engine):
```go
func (e *Engine) handleAlgoRemap(exchName string, remap exchange.AlgoRemap) {
    e.slIndexMu.Lock()
    if entry, ok := e.slIndex[exchName+":"+remap.AlgoID]; ok {
        // Alias: add the matching-engine ID as a second key pointing at the same entry.
        // Keep the original algoID entry so unregisterStopOrders still cleans up on close.
        e.slIndex[exchName+":"+remap.RealID] = entry
        e.log.Info("algoRemap %s: %s → %s (leg=%s kind=%s)",
            exchName, remap.AlgoID, remap.RealID, entry.Leg, entry.Kind)
    }
    e.slIndexMu.Unlock()
}
```

##### H3B.2d — Reload path re-attachment (exchange_manager.go) — split by ownership
**Critical: per codex v3 finding, `internal/engine/exchange_manager.go:85-154` Reload() rebuilds adapters WITHOUT re-running cmd/main.go wiring code. This is a latent bug affecting all existing `*CallbackSetter` patterns — this plan addresses it for BOTH existing and new callbacks, split by actual ownership:**

**Ownership map (current architecture, verified against code):**
- `cmd/main.go:181-196` owns: `SetMetricsCallback` (risk scorer latency recording), `SetWSMetricsCallback` (WS health events → scorer), `SetOrderMetricsCallback` (order fill events → scorer).
- `internal/engine/engine.go:1647-1658` (`setupSLCallbacks`) owns: `SetOrderCallback` (SL fill events → slFillCh).
- **New**: engine will own `SetAlgoRemapCallback` (algo-trigger events → slIndex alias).

**Implementation:**

1. `internal/engine/exchange_manager.go`: add a dispatch mechanism that invokes multiple reload-handler callbacks, NOT just one:
   ```go
   // ExchangeManager struct gains:
   reloadHandlersMu sync.Mutex
   reloadHandlers   []func(name string, adapter exchange.Exchange)
   
   // New public method:
   func (m *ExchangeManager) AddReloadHandler(fn func(name string, adapter exchange.Exchange)) {
       m.reloadHandlersMu.Lock()
       m.reloadHandlers = append(m.reloadHandlers, fn)
       m.reloadHandlersMu.Unlock()
   }
   
   // Inside Reload(), after `m.exchanges[name] = adapter` (line 151):
   m.reloadHandlersMu.Lock()
   handlers := append([]func(string, exchange.Exchange){}, m.reloadHandlers...)
   m.reloadHandlersMu.Unlock()
   for _, h := range handlers {
       h(name, adapter)
   }
   ```
   Invoke handlers OUTSIDE `m.mu` to avoid deadlock if a handler calls back into ExchangeManager. (Verified: existing callbacks in main/engine do not re-enter ExchangeManager.)

2. `cmd/main.go`: register ONE reload handler that re-runs the existing scorer wiring (lines 182-196 equivalent):
   ```go
   em.AddReloadHandler(func(name string, exch exchange.Exchange) {
       exchangeName := name
       exch.SetMetricsCallback(func(endpoint string, latency time.Duration, err error) {
           scorer.RecordLatency(exchangeName, endpoint, latency, err)
       })
       if setter, ok := exch.(exchange.WSMetricsCallbackSetter); ok {
           setter.SetWSMetricsCallback(func(event exchange.WSEvent) {
               scorer.RecordWSEvent(exchangeName, event)
           })
       }
       if setter, ok := exch.(exchange.OrderMetricsCallbackSetter); ok {
           setter.SetOrderMetricsCallback(func(event exchange.OrderMetricEvent) {
               scorer.RecordOrderEvent(exchangeName, event)
           })
       }
   })
   ```
   The existing startup wiring (cmd/main.go:182-196) and this reload handler MUST call the same body; refactor to a shared closure/function `attachScorerCallbacks(name, exch)` to avoid drift.

3. `internal/engine/engine.go`: register a separate reload handler for engine-owned callbacks:
   ```go
   // New method on Engine:
   func (e *Engine) attachAdapterCallbacks(name string, exch exchange.Exchange) {
       exchName := name
       // Re-attach SL fill callback (was in setupSLCallbacks)
       exch.SetOrderCallback(func(upd exchange.OrderUpdate) {
           select {
           case e.slFillCh <- slFillEvent{Exchange: exchName, Update: upd}:
           default:
               e.log.Warn("SL fill channel full, dropping event for %s on %s", upd.OrderID, exchName)
           }
       })
       // Re-attach algo-remap callback (H3B.2c)
       if setter, ok := exch.(exchange.AlgoRemapCallbackSetter); ok {
           setter.SetAlgoRemapCallback(func(remap exchange.AlgoRemap) {
               e.handleAlgoRemap(exchName, remap)
           })
       }
   }
   
   // Called in Engine initialization (replaces direct setupSLCallbacks loop body with per-exchange call):
   func (e *Engine) setupSLCallbacks() {
       for name, exch := range e.exchanges {
           e.attachAdapterCallbacks(name, exch)
       }
   }
   
   // Engine init also registers reload handler:
   em.AddReloadHandler(e.attachAdapterCallbacks)
   ```

This ensures:
- Initial startup uses `setupSLCallbacks` + cmd/main.go lines 182-196 (unchanged behavior).
- Reload rebuilds trigger both reload handlers, which restore full callback state.
- Ownership stays clean: scorer wiring in main, engine wiring in engine.
- `SetOrderCallback` (SL fill) is now correctly re-attached on rebuild — a latent bug that v4 did not address.

##### H3B.2e — Event ordering: ALGO_UPDATE vs ORDER_TRADE_UPDATE
Per Binance docs (`doc/binance/binance-usds-futures-api-docs.md:10659-10713`), ALGO_UPDATE reaches `X=TRIGGERED` when "the order has been successfully placed into the matching engine" — i.e. the matching-engine order is created before it can fill. Under normal conditions, ALGO_UPDATE TRIGGERED arrives strictly before any ORDER_TRADE_UPDATE fill for that orderID.

**Risk**: WebSocket event ordering is not strictly guaranteed. If ORDER_TRADE_UPDATE wins the race and arrives before ALGO_UPDATE TRIGGERED, `handleSLFill` method 1 lookup fails (no alias yet), and method 2 falls back to narrow `cp && (STOP||TP)_MARKET` heuristic — which fails for TP (`cp=false`).

**Fallback**: consolidator catches `exchange-flat (both sizes=0) but Active` after 10s (this is what caught `sirenusdt-1776268502636` today). So the system is not catastrophically exposed — the fallback is a late (10s) reconciliation instead of instant detection. That is still a meaningful improvement over current code which ALSO relies on consolidator.

For tighter coverage, a future follow-up can add a "pending fill" cache: when an unknown orderID fill with `ot=TAKE_PROFIT_MARKET|STOP_MARKET` arrives, park it briefly and link if ALGO_UPDATE arrives within e.g. 2 seconds. Out of scope for v4.

### Fix H3C — BingX TAKE_PROFIT_MARKET close semantics
- `pkg/exchange/bingx/ws_private.go:254-255`: extend existing inference:
  ```go
  reduceOnly := o.ReduceOnly ||
                o.OrderType == "STOP_MARKET" ||
                o.OrderType == "TAKE_PROFIT_MARKET"
  ```

## Why

### H1 (retrySecondLeg)
- Market order adapters return real `avgPrice` via `GetOrderUpdate` (WS) and `GetOrderFilledQty`+`orderStore` (REST populated). Verified: `pkg/exchange/bingx/adapter.go:303-343`, `pkg/exchange/bingx/ws_private.go:229-269`, and `confirmFill()` at `internal/engine/engine.go:4320-4371` is the blueprint.
- But `retrySecondLeg` at `engine.go:2820` synthesizes `fillPrice := (bbo.Bid + bbo.Ask) / 2` and uses that instead.
- When `GetBBO()` returns garbage, synthetic fillPrice is garbage → VWAP inflated → all downstream PnL/SL calculations corrupted.

### H2 (reconcile guard)
- Current guard `exit.go:1185-1189` assumes `abs(diff) > notional` means "exchange data incomplete".
- But if stored local PnL was contaminated at entry (like H1), local can be grossly wrong while exchange is complete and consistent.
- Guard refuses to correct exactly when correction is most needed → bogus PnL permanently enters stats.

### H3 (SL/TP detection)
- Codex confirmed (dispatch #62283a03): binance TP ID never indexed, plus binance remaps algo→matching-engine ID on trigger.
- Real fill from SIRENUSDT trade had orderID=3561813146 ≠ stored 3000001251209638, reduceOnly=false.
- Both detection methods missed; only consolidator (10s delay) saved the position.

## How (key snippets)

### Fix H1A
```go
// Helper added near confirmFill in engine.go:
func (e *Engine) queryAvgFillPrice(exch exchange.Exchange, orderID string) float64 {
    // 1st try: WS order-store
    if upd, ok := exch.GetOrderUpdate(orderID); ok && upd.AvgPrice > 0 {
        return upd.AvgPrice
    }
    // Retry after short sleep (WS may lag slightly behind REST GetOrderFilledQty)
    time.Sleep(200 * time.Millisecond)
    if upd, ok := exch.GetOrderUpdate(orderID); ok && upd.AvgPrice > 0 {
        return upd.AvgPrice
    }
    return 0 // caller decides fallback
}

// engine.go:2769 (IOC retry), rewritten:
if filledQty > 0 {
    avgPrice := e.queryAvgFillPrice(exch, orderID)
    if avgPrice <= 0 {
        avgPrice = orderPrice // limit price is a reasonable lower bound for filled IOC
        e.log.Warn("retrySecondLeg[IOC-%d] %s: avgPrice unavailable, using orderPrice=%.6f", attempt, exchName, orderPrice)
    }
    totalFilled += filledQty
    totalNotional += filledQty * avgPrice
    remainingSize -= filledQty
}

// engine.go:2817 (MKT retry), rewritten:
if filledQty > 0 {
    avgPrice := e.queryAvgFillPrice(exch, orderID)
    if avgPrice <= 0 {
        avgPrice = refPrice // refPrice is last known good price; safer than BBO-mid
        e.log.Warn("retrySecondLeg[MKT-%d] %s: avgPrice unavailable, using refPrice=%.6f", mktAttempt, exchName, refPrice)
    }
    totalFilled += filledQty
    totalNotional += filledQty * avgPrice
    remainingSize -= filledQty
}
```

### Fix H1B
```go
// engine.go:2723
bbo, ok := exch.GetBBO(symbol)
if !ok || bbo.Bid <= 0 || bbo.Ask <= 0 ||
    bbo.Bid > refPrice*1.20 || bbo.Bid < refPrice*0.80 ||
    bbo.Ask > refPrice*1.20 || bbo.Ask < refPrice*0.80 {
    e.log.Warn("retrySecondLeg: BBO sanity failed (bid=%.6f ask=%.6f ref=%.6f) — fallback to refPrice", bbo.Bid, bbo.Ask, refPrice)
    bbo = exchange.BBO{Bid: refPrice, Ask: refPrice}
}
```

### Fix H2 (pseudo-code — v12 canonical, supersedes earlier versions)
```go
// models/position.go — add fields:
type ArbitragePosition struct {
    // ... existing fields ...
    LongSize       float64 `json:"long_size"`
    ShortSize      float64 `json:"short_size"`
    LongCloseSize  float64 `json:"long_close_size"`   // NEW — current intended full close size (see H2.2 for update sites)
    ShortCloseSize float64 `json:"short_close_size"`  // NEW
    // ... rest of fields ...
}

// engine.go:3122-3142 (and :4265-4286) — active-entry activation sites:
pos := &models.ArbitragePosition{
    // ... existing fields ...
    LongSize:       confirmedLong,
    ShortSize:      confirmedShort,
    LongCloseSize:  confirmedLong,
    ShortCloseSize: confirmedShort,
}

// exit.go:973-980 — partial-close revert:
fresh.LongSize = longRemainder
fresh.ShortSize = shortRemainder
fresh.LongCloseSize = longRemainder
fresh.ShortCloseSize = shortRemainder

// exit.go:2776-2779 / :2907-2918 / :2968-2986 — rotation partial/resize:
// When a leg's live size changes, update its CloseSize to match.

// engine.go:2591-2603 — startup duplicate merge survivor:
// After merging sizes, also set CloseSize to match.

// exit.go:960-998 — depth-exit close: DO NOT touch LongCloseSize/ShortCloseSize.
// fresh.LongSize = 0  and  fresh.ShortSize = 0  continue as before.

// ======== tryReconcilePnL body — replace :1177-1189 block with two-phase flow: ========

// Existing aggregation + longOK && shortOK check at :1137-1152 stays as-is.

const sizeEpsilon = 1e-6

// Phase 1: pre-split Tier 1 completeness gate
longSiblings := e.siblingsFor(pos, "long")
shortSiblings := e.siblingsFor(pos, "short")
useTier1 := pos.LongCloseSize > 0 && pos.ShortCloseSize > 0 &&
    allSiblingsHaveCloseSize(longSiblings, "long") &&
    allSiblingsHaveCloseSize(shortSiblings, "short")

if useTier1 {
    longExpected := pos.LongCloseSize + sumSiblingCloseSize(longSiblings, "long")
    shortExpected := pos.ShortCloseSize + sumSiblingCloseSize(shortSiblings, "short")
    if longAgg.CloseSize < longExpected-sizeEpsilon || shortAgg.CloseSize < shortExpected-sizeEpsilon {
        e.log.Warn("reconcile %s [attempt %d]: incomplete close data (longRawClose=%.6f/%.6f shortRawClose=%.6f/%.6f), retrying",
            pos.ID, attempt, longAgg.CloseSize, longExpected, shortAgg.CloseSize, shortExpected)
        return false
    }
}

// Phase 2: split + diff calculation (existing code)
longAgg = e.splitSharedPnL(longAgg, pos, "long")
shortAgg = e.splitSharedPnL(shortAgg, pos, "short")
reconciledPnL := longAgg.NetPnL + shortAgg.NetPnL + pos.RotationPnL
diff := reconciledPnL - pos.RealizedPnL

// Phase 3: post-split Tier 2 / Tier 3 (only if not Tier 1)
if !useTier1 {
    if pos.LongSize > 0 || pos.ShortSize > 0 {
        longNotional := pos.LongEntry * pos.LongSize
        shortNotional := pos.ShortEntry * pos.ShortSize
        notional := math.Max(longNotional, shortNotional)
        if notional > 0 && math.Abs(diff) > notional {
            e.log.Warn("reconcile %s [attempt %d]: pre-migration diff %.4f exceeds notional %.4f, retrying",
                pos.ID, attempt, diff, notional)
            return false
        }
    } else {
        e.log.Warn("reconcile %s [attempt %d]: pre-migration depth-exit, no size info — relying on longOK && shortOK",
            pos.ID, attempt)
    }
}

// Informational variance log:
if pos.EntryNotional > 0 && math.Abs(longAgg.NetPnL+shortAgg.NetPnL) > pos.EntryNotional*0.5 {
    e.log.Warn("reconcile %s [attempt %d]: large delta-neutral variance long+short=%.4f vs entryNotional %.4f (informational, proceeding)",
        pos.ID, attempt, longAgg.NetPnL+shortAgg.NetPnL, pos.EntryNotional)
}
// Fall through to existing needsPnLUpdate / needsFundingUpdate / write path.
```

### Fix H3A
```go
// engine.go:131-135
type stopOrderEntry struct {
    PosID string
    Leg   string // "long" or "short"
    Kind  string // "sl" or "tp"
}

// engine.go:1479-1491
func (e *Engine) registerStopOrders(pos *models.ArbitragePosition) {
    e.slIndexMu.Lock()
    defer e.slIndexMu.Unlock()
    regs := []struct{
        ex, oid, leg, kind string
    }{
        {pos.LongExchange,  pos.LongSLOrderID,  "long",  "sl"},
        {pos.ShortExchange, pos.ShortSLOrderID, "short", "sl"},
        {pos.LongExchange,  pos.LongTPOrderID,  "long",  "tp"},
        {pos.ShortExchange, pos.ShortTPOrderID, "short", "tp"},
    }
    for _, r := range regs {
        if r.oid != "" {
            e.slIndex[r.ex+":"+r.oid] = stopOrderEntry{
                PosID: pos.ID,
                Leg:   r.leg,
                Kind:  r.kind,
            }
        }
    }
}

// exit.go:3229-3233 rotation path replacement:
e.slIndexMu.Lock()
e.slIndex[newExchName+":"+oid] = stopOrderEntry{
    PosID: pos.ID, Leg: legSide, Kind: "sl",
}
if tpOID != "" {
    e.slIndex[newExchName+":"+tpOID] = stopOrderEntry{
        PosID: pos.ID, Leg: legSide, Kind: "tp",
    }
}
e.slIndexMu.Unlock()
```
(plus matching unregisterStopOrders and rebuildStopIndex rename)

### Fix H3B
```go
// binance/ws_private.go:102-115
Order struct {
    Symbol        string `json:"s"`
    ClientOrderID string `json:"c"`
    Side          string `json:"S"`
    OrderType     string `json:"o"`
    OrderStatus   string `json:"X"`
    OrderID       int64  `json:"i"`
    AvgPrice      string `json:"ap"`
    FilledQty     string `json:"z"`
    ReduceOnly    bool   `json:"R"`   // FIXED: was "ro"
    OrigQty       string `json:"q"`
    ClosePosition bool   `json:"cp"`  // ADDED
    OrigOrderType string `json:"ot"`  // ADDED
} `json:"o"`

// binance/ws_private.go:128-136
isCloseFill := o.ReduceOnly ||
    (o.ClosePosition && (o.OrigOrderType == "STOP_MARKET" || o.OrigOrderType == "TAKE_PROFIT_MARKET"))
upd := exchange.OrderUpdate{
    OrderID:      oid,
    ClientOID:    o.ClientOrderID,
    Status:       strings.ToLower(o.OrderStatus),
    FilledVolume: filledVol,
    AvgPrice:     avgPrice,
    Symbol:       o.Symbol,
    ReduceOnly:   isCloseFill,
}
wsPrivLog.Info("order update: %s %s %s status=%s filled=%.6f avg=%.8f R=%v cp=%v ot=%s isClose=%v",
    o.Symbol, o.Side, oid, o.OrderStatus, filledVol, avgPrice, o.ReduceOnly, o.ClosePosition, o.OrigOrderType, isCloseFill)
```

## Risk

### H1A risk
- `GetOrderUpdate` is standard across adapters (part of `Exchange` interface). No new surface needed.
- Fallback chain: WS → REST-populated store → orderPrice (IOC) / refPrice (MKT). Always produces a finite, sane value.
- If all 6 adapter stores lag on WS, IOC fallback to orderPrice is already the current behavior for similar paths. MKT fallback to refPrice is safer than current BBO-mid.

### H1B risk
- 20% deviation envelope may be too tight for high-vol tokens (meme coins can move 20% in short windows). If false positives observed, raise to 30% in a future tuning commit. But blocking bad BBO from entering VWAP is the priority.

### H2 risk
- Schema change: 2 new fields on `ArbitragePosition`. JSON backward-compat is free (missing fields decode as zero).
- CloseSize discipline: updated on ALL legitimate size-changing active paths (entry, partial-close revert, rotation partial, startup merge). Any NEW code path that changes `LongSize`/`ShortSize` while position is Active must also update CloseSize. Doc comment emphasizes this invariant.
- Depth-exit zeros LongSize but NOT CloseSize — this is the key property that makes reconcile gate work.
- Three-tier fallback protects existing in-flight positions:
  - **Tier 1** (fully-migrated): pos AND all siblings have CloseSize > 0 → sibling-aware exact CloseSize gate PRE-SPLIT.
  - **Tier 2** (pre-migration normal close): pos.LongSize/ShortSize > 0 but CloseSize cohort incomplete → keep only the current-position `abs(diff) > notional` guard; do NOT reconstruct sibling totals from history (live code already treats history as non-exact — normal-close siblings keep sizes, depth-exit siblings are zeroed, so sibling-size sum can undercount and false-pass).
  - **Tier 3** (pre-migration depth-exit): both CloseSize=0 AND LongSize=0 → rely on longOK && shortOK (equivalent to old `notional<=0` fallback).
- Rotation DB write at exit.go:3210-3227 doesn't change leg total size, only exchange. CloseSize unchanged here. Partial-accepting rotation at :2776-2779 / :2907-2918 / :2968-2986 DOES change size — CloseSize MUST be updated there.
- Retry loop (`pnlReconcileAttempts`) still handles eventual consistency during exchange API lag.
- For H1 incident (post-fix): CloseSize=189 preserved through depth-exit zeroing, raw CloseSize 189 == expected 189 → Tier 1 gate passes attempt 1, correction from +$2385 → -$1.39 applies.

### H3A risk
- `slEntry` → `stopOrderEntry` touches 6 files/locations. Mitigation: search `slEntry` occurrences before change, update all, compile to verify.

### H3B risk
- **CRITICAL**: Fixing `json:"ro"` → `json:"R"` is a latent-bug fix with broad effect. Any code relying on the old (broken) ReduceOnly=false always-for-Binance behavior will now see ReduceOnly=true for genuine reduce-only orders. Mitigation: `handleSLFill` method 2 already ownOrder-filters to skip bot-initiated orders, so this change shouldn't trigger false emergency closes for regular engine activity. Test on live before deploy.
- Narrow heuristic (`cp=true && (STOP_MARKET || TAKE_PROFIT_MARKET)`) avoids false positives on opening conditional orders.

### H3C risk
- Low. Symmetric extension of existing STOP_MARKET pattern. No callers to update.

## Verification plan

Before deploy:
1. `go build ./...` — compile check
2. `go test ./internal/engine/... ./pkg/exchange/binance/... ./pkg/exchange/bingx/...`
3. Add unit tests:
   - `TestRetrySecondLegUsesGetOrderUpdateAvgPrice` — mock adapter with populated order store; verify totalNotional uses WS AvgPrice not BBO-mid.
   - `TestRetrySecondLegFallbackWhenAvgPriceZero` — mock GetOrderUpdate returns AvgPrice=0; verify IOC falls back to orderPrice, MKT falls back to refPrice.
   - `TestRetrySecondLegBBOSanityFallback` — mock BBO returns bid=100*refPrice; verify logs sanity warning and uses refPrice.
   - `TestReconcileAcceptsDepthExitWithCloseSizeMatch` (Tier 1 H1 replay) — depth-exit zeros LongSize, CloseSize=189 preserved, rawCloseSize=189, local=+2385, exchange=-1.39 → correction applied.
   - `TestReconcileRetriesWhenAggregationFails` — one side empty close records (longOK=false) → retry (pre-existing).
   - `TestReconcileRetriesWhenCloseSizePartial` (Tier 1) — no siblings, CloseSize=385, rawCloseSize=100 → retry.
   - `TestReconcileAcceptsSharedPositionCompleteClose` (Tier 1 shared) — 2 siblings each CloseSize=100, rawCloseSize=200 → passes.
   - `TestReconcileRetriesSharedPositionPartial` (Tier 1 shared partial) — 2 siblings each CloseSize=100, rawCloseSize=50 → retry.
   - `TestReconcileMixedHistoryFallsThroughToTier2` — pos CloseSize=100, one sibling CloseSize=0 → Tier 1 precondition fails, falls to Tier 2 current-position notional guard only (no sibling reconstruction).
   - `TestReconcilePreMigrationNormalCloseRetainsNotionalGuard` (Tier 2) — CloseSize=0, LongSize=385, diff within notional → accepted; diff > notional → retry.
   - `TestReconcileDepthExitPreMigrationFallback` (Tier 3) — CloseSize=0 AND LongSize=0 → accepts via longOK && shortOK only.
   - `TestCloseSizeImmutableThroughDepthExit` — depth-exit zeros LongSize but CloseSize preserved in both position record and history entry.
   - `TestReconcilePartialCloseRevertUpdatesCloseSize` — revert sets LongSize=remainder AND LongCloseSize=remainder.
   - `TestReconcileRotationAcceptingPartialUpdatesCloseSize` — rotation openFilled < target sets CloseSize to openFilled.
   - `TestReconcileStartupMergeUpdatesCloseSize` — duplicate merge survivor: CloseSize updated to merged size.
   - `TestStopIndexMatchesTPID` — register TP ID, receive WS update with that ID → verify triggerEmergencyClose called.
   - `TestBinanceWSClosePositionDetection` — parse payload `R=false cp=true ot=TAKE_PROFIT_MARKET` → verify upd.ReduceOnly=true.
   - `TestBinanceWSNormalFillStaysNonReduce` — parse payload `R=false cp=false ot=LIMIT` → verify upd.ReduceOnly=false (no false positive).
   - `TestBingxWSTakeProfitMarketIsReduce` — parse BingX payload OrderType=TAKE_PROFIT_MARKET → verify reduceOnly=true.
4. Livetest smoke on testnet: place + cancel a reduce-only order on Binance, verify upd.ReduceOnly arrives correctly.

After deploy:
1. Monitor for log `reconcile ... diff ... exceeds notional` — should drop to zero.
2. Monitor for new log `retrySecondLeg: BBO sanity failed` — expected rare, any occurrence indicates bad BBO feed.
3. Monitor for unexpected `handleSLFill` method 2 triggers — verify they really are exchange-side SL/TP and not legitimate engine activity mis-classified.
4. Dashboard stats: look for any `abs(realized_pnl) > 10 * abs(entry_notional)` positions (sanity).

## Out of scope (tracked as follow-ups)

1. **H1C** — BingX priceStore BBO order-of-magnitude guard (needs prior-BBO lookup spec).
2. **BingX private WS resilience** — listenKey refresh telemetry and private-WS gap detection. Not a code bug in our handlers; an observability gap.

## Review History
- v1: codex (dispatch #529bf3e1) — NEEDS-REVISION — 5 items: H1A wrong interface, H1C underspecified, H2 inconsistent, H3A undercounted callers, H3B too broad + field-tag bug
- v2: codex (dispatch #b9f347b7) — NEEDS-REVISION — 3 items: H2 still 50% not completeness, H3A pseudo-code bug + rotation TP not registered, H3B narrow heuristic doesn't catch actual incident (TP uses cp=false)
- v3: codex (dispatch #7ecd7abd) — NEEDS-REVISION — 2 items: H2 Risk text stale (still says 50%), H3B.2 underspecified wiring (callback interface + reload path + event ordering)
- v4: codex (dispatch #b608e191) — NEEDS-REVISION — 1 item: H3B.2d reload ownership confusion (wrong component owning scorer callbacks; missed `SetOrderCallback` reattachment)
- v5: addresses v4 finding:
  - H3B.2d rewritten to split reload reattachment by actual ownership:
    - `ExchangeManager.AddReloadHandler` supports multiple registered handlers
    - `cmd/main.go` registers handler for scorer callbacks (`SetMetricsCallback`, `SetWSMetricsCallback`, `SetOrderMetricsCallback`)
    - `Engine` registers separate handler for engine-owned callbacks (`SetOrderCallback` for SL fills AND new `SetAlgoRemapCallback`)
  - Both startup paths and reload path now consistently call the same per-exchange attachment bodies (shared closures)
  - Fixes latent `SetOrderCallback` reload-drop bug that v4 omitted
- v5: codex (dispatch #30676c4a) — **ALL PASS** — All 8 sub-fixes (H1A, H1B, H1C, H2, H3A, H3B.1, H3B.2, H3C) cleared.
- v6: user-directed simplification of H2. The 95% threshold was over-engineered; v6 removes both the old `abs(diff) > notional` gate AND the new 95% CloseSize gate. Trust exchange aggregated data unconditionally when `longOK && shortOK` succeed (pre-existing check at exit.go:1149-1152). Delta-neutral variance becomes informational log only.
- v6: codex (dispatch #4a3bdac7) — NEEDS-REVISION — 1 item: H2 over-simplification. `longOK && shortOK` does not prove completeness; `aggregateClosePnLBySide` returns `found=true` on any ≥1 record per side. Partial aggregation could pass on attempt 1 and block later attempts from overwriting with full data.
- v7: user-corrected H2 rewrite. USDT-margined perp fees deduct from USDT, not contract size, so CloseSize should match entry size exactly (not 95%). H2 uses exact comparison `longAgg.CloseSize >= pos.LongSize - 1e-6` (float epsilon only) when pos.Size > 0, falls back to `longOK && shortOK` for depth-exit path where sizes are zeroed. Added 2 unit tests for partial-reporting and depth-exit paths.
- v7: codex (dispatch #92a3594c re-dispatched from stuck #5eaa2bfd) — NEEDS-REVISION — 1 item: exact CloseSize match on POST-SPLIT aggregate breaks for shared/merged exchange positions. `splitSharedPnL` (exit.go:1418-1431) scales CloseSize by `1/(siblings+1)`, so a fully-reported exchange close produces post-split CloseSize < pos.Size when siblings exist.
- v8: H2 restructured. Completeness check moves BEFORE `splitSharedPnL`, compares RAW aggregates against sibling-aware expected total (pos.Size + sum of sibling sizes in same 5-min window + same exchange+symbol+side filter). Extracted sibling-discovery logic into shared helpers `countSiblings`/`expectedCloseSize`/`siblingsFor`. Added 2 more unit tests for shared-position complete/partial scenarios.
- v8: codex (dispatch #eba88dd7) — NEEDS-REVISION — 1 item: `expectedCloseSize` relies on `pos.Size + sibling sizes` but depth-exit zeros `pos.LongSize/ShortSize` at exit.go:991-992 BEFORE reconcilePnL runs, and history entries from depth-exit/rollback paths also have zeroed sizes. So sibling sum undercounts, gate false-passes.
- v9: H2 adds immutable `LongInitialSize`/`ShortInitialSize` fields to position model. Set at entry-fill completion, never mutated by rotation/close/zero-out. Reconcile gate uses these + sibling sum of InitialSize. Old pre-migration positions (InitialSize=0) fall back to `longOK && shortOK` — no regression. Added 2 more unit tests for InitialSize preservation and pre-migration fallback.
- v9: codex (dispatch #956c3594) — NEEDS-REVISION — 2 items: (1) "immutable from birth" wrong — legitimate size-changing paths exist (partial revert, rotation partial, startup merge); (2) pre-migration fallback too weak for `closePositionWithMode` path which reconciles with non-zero live sizes.
- v10: H2 replaces `InitialSize` with `LongCloseSize`/`ShortCloseSize` (current intended close size). Updated on all size-changing active paths (entry, revert, rotation partial, startup merge) but NOT by depth-exit zeroing. Three-tier gate: (1) CloseSize when available, (2) LongSize + sibling LongSize when pre-migration normal close has non-zero live size, (3) longOK && shortOK as last resort for depth-exit pre-migration. Added 8 unit tests covering all tiers and size-changing paths.
- v10: codex (dispatch #3c6e3a66) — NEEDS-REVISION — 1 item: Tier 2 still reconstructs sibling LongSize/ShortSize from mixed history (normal-close siblings keep non-zero sizes, depth-exit siblings zeroed). `splitSharedPnL` already treats history as non-exact; sibling-size sum can undercount and false-pass partial exchange data. Codex provided exact fix.
- v11: H2 applies codex v10's exact fix: Tier 1 requires pos AND all siblings to have CloseSize populated (stricter precondition); Tier 2 retains the old `abs(diff) > notional` guard for the current position only, NO sibling reconstruction; Tier 3 unchanged. Deleted `sumSiblingLongSize`/`sumSiblingShortSize` helpers. Updated entry-site citations to codex-exact (engine.go:3122-3142, :4265-4286, :2591-2603; exit.go:973-980, 2776-2779, 2907-2918, 2968-2986). Added `TestReconcileMixedHistoryFallsThroughToTier2` for mixed-cohort regression.
- v11: codex (dispatch #8b4c2837) — NEEDS-REVISION — 2 plan-text consistency issues: (1) Tier 2 referenced `diff` before reconciledPnL/diff are computed (order-of-operations bug in pseudo-code); (2) stale text in "How → Fix H2" still showed deleted LongInitialSize design, stale "Risk → H2" still said Tier 2 reconstructs sibling LongSize. Codex provided exact two-phase flow.
- v12: applies codex v11 exact two-phase flow: Phase 1 (pre-split Tier 1 gate), Phase 2 (split + diff calculation), Phase 3 (post-split Tier 2/3 using computed diff). Unified `useTier1` flag prevents double-gating. Overwrote stale "How" and "Risk" sections with v12 canonical text. Expanded Verification plan's unit test list to match H2 section (12 tests).
- v12: codex (dispatch #dfb673ec) — NEEDS-REVISION — 1 documentation mismatch: H2 unit-test subsection still labeled "v11" with 10 tests, Verification plan has 12 tests. Two inventories not identical.
- v13: renamed H2 unit-test subsection to "v13" and replaced with the exact 12-test inventory (same as Verification plan). Plan now has single canonical H2 test list.
- v13: codex (dispatch #c27e5c3c) — **ALL PASS** — All 8 sub-fixes (H1A, H1B, H1C, H2, H3A, H3B.1, H3B.2, H3C) cleared. Ready for implementation.
