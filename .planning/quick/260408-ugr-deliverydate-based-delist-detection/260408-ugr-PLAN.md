---
phase: 260408-ugr
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - pkg/exchange/types.go
  - pkg/exchange/binance/adapter.go
  - pkg/exchange/bybit/adapter.go
  - pkg/exchange/binance/adapter_test.go
  - internal/discovery/contract_refresh.go
  - internal/discovery/scanner.go
  - internal/config/config.go
  - cmd/main.go
  - internal/engine/engine.go
  - internal/database/state.go
  - internal/spotengine/risk_gate.go
  - internal/spotengine/monitor.go
  - CHANGELOG.md
  - VERSION
autonomous: true
requirements:
  - SAFETY-DELIST-01
user_setup: []

must_haves:
  truths:
    - "Binance perpetuals with a near-future deliveryDate are written to arb:delist:{SYMBOL} within ContractRefreshInterval of bot startup"
    - "Bybit linear perpetuals with a near-future deliveryTime are written to arb:delist:{SYMBOL} within ContractRefreshInterval of bot startup"
    - "Normal perpetuals (year-2100 sentinel deliveryDate) are NOT added to the delist blacklist"
    - "Scanner step-8 filter blocks delisted symbols from entry opportunities on any exchange (existing behavior preserved via key reuse)"
    - "Engine checkDelistPositions auto-exits an active position for a delisted symbol regardless of which exchange the leg is on"
    - "Spot-futures risk_gate rejects any entry for a delisted symbol with reason delist_{symbol}"
    - "Spot-futures monitor triggers an exit on an active position whose symbol has been flagged delist since entry"
    - "Article-scraping delist monitor still runs as a secondary signal (no regression)"
    - "Existing perp-perp trading is not broken — build passes, adapter tests pass"
  artifacts:
    - path: "pkg/exchange/types.go"
      provides: "ContractInfo.DeliveryDate time.Time field"
      contains: "DeliveryDate time.Time"
    - path: "pkg/exchange/binance/adapter.go"
      provides: "Binance LoadAllContracts parses deliveryDate + contractType into ContractInfo.DeliveryDate"
      contains: "DeliveryDate"
    - path: "pkg/exchange/bybit/adapter.go"
      provides: "Bybit LoadAllContracts parses deliveryTime + contractType into ContractInfo.DeliveryDate"
      contains: "DeliveryDate"
    - path: "internal/discovery/contract_refresh.go"
      provides: "StartContractRefresh periodic poller writing arb:delist:{SYMBOL} from DeliveryDate"
      contains: "StartContractRefresh"
      min_lines: 70
    - path: "internal/config/config.go"
      provides: "ContractRefreshInterval duration field, default 1h"
      contains: "ContractRefreshInterval"
    - path: "cmd/main.go"
      provides: "StartContractRefresh wired alongside StartDelistMonitor under DelistFilterEnabled"
      contains: "StartContractRefresh"
    - path: "internal/engine/engine.go"
      provides: "checkDelistPositions no longer gated on Binance-only leg"
      contains: "checkDelistPositions"
    - path: "internal/database/state.go"
      provides: "IsDelisted(symbol) helper reading arb:delist:{symbol}"
      contains: "func (c *Client) IsDelisted"
    - path: "internal/spotengine/risk_gate.go"
      provides: "Delist check as new pre-entry step before dry-run short-circuit"
      contains: "IsDelisted"
    - path: "internal/spotengine/monitor.go"
      provides: "Active-position delist scan invoking launchExit for flagged symbols"
      contains: "IsDelisted"
    - path: "CHANGELOG.md"
      provides: "New 0.30.0 entry describing the deliveryDate delist detection fix"
      contains: "deliveryDate"
    - path: "VERSION"
      provides: "Version bumped to 0.30.0"
      contains: "0.30.0"
  key_links:
    - from: "internal/discovery/contract_refresh.go"
      to: "pkg/exchange/{binance,bybit}/adapter.go LoadAllContracts"
      via: "refreshContractsAndFlagDelists iterates s.exchanges and calls LoadAllContracts"
      pattern: "LoadAllContracts"
    - from: "internal/discovery/contract_refresh.go"
      to: "Redis key arb:delist:{SYMBOL}"
      via: "s.db.SetWithTTL with delistRedisPrefix + symbol (or SetDelistCooldown)"
      pattern: "delistRedisPrefix|arb:delist:"
    - from: "internal/spotengine/risk_gate.go"
      to: "internal/database/state.go IsDelisted"
      via: "e.db.IsDelisted(symbol) call in step 7"
      pattern: "e\\.db\\.IsDelisted"
    - from: "internal/spotengine/monitor.go"
      to: "internal/spotengine/engine.go launchExit"
      via: "e.launchExit(pos, \"delist\", true) when e.db.IsDelisted(pos.Symbol)"
      pattern: "launchExit.*delist"
    - from: "cmd/main.go"
      to: "internal/discovery/contract_refresh.go StartContractRefresh"
      via: "scanner.StartContractRefresh() after StartDelistMonitor inside DelistFilterEnabled block"
      pattern: "StartContractRefresh"
---

<objective>
Replace the brittle article-scraping delist parser with a deliveryDate-based safety net
that polls Binance `/fapi/v1/exchangeInfo` and Bybit `/v5/market/instruments-info` on a
periodic ticker, writes flagged symbols to the existing `arb:delist:{SYMBOL}` Redis key,
and adds spot-futures parity so both engines refuse entries and auto-exit active
positions when a coin is scheduled for delisting.

Purpose: On 2026-04-08 Binance delisted OLUSDT, HIPPOUSDT, RLSUSDT, PUFFERUSDT with a
generic-title announcement that bypassed the regex parser — the bot entered RLSUSDT and
took a loss. `deliveryDate` is machine-readable, set at announcement time, and already
populated on endpoints we call. The article scraper is kept as a belt-and-suspenders
second signal. Spot-futures has NO delist awareness today (`grep -n "Delist\|delist"
internal/spotengine/` returns zero matches) — same PR closes that gap.

Output: Single Go binary that within one `ContractRefreshInterval` of startup has
written `arb:delist:{SYMBOL}` entries for every Binance/Bybit perpetual with a
near-future `deliveryDate`, blocking new entries in both engines and auto-exiting any
active position.
</objective>

<execution_context>
@/var/solana/data/arb/.claude/get-shit-done/workflows/execute-plan.md
@/var/solana/data/arb/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/STATE.md
@/home/solana/.claude/plans/radiant-spinning-quokka.md
@pkg/exchange/types.go
@pkg/exchange/binance/adapter.go
@pkg/exchange/bybit/adapter.go
@internal/discovery/delist.go
@internal/discovery/scanner.go
@internal/engine/engine.go
@internal/spotengine/risk_gate.go
@internal/spotengine/monitor.go
@internal/spotengine/engine.go
@internal/database/state.go
@internal/database/redis.go
@internal/database/spot_state.go
@internal/config/config.go
@cmd/main.go
@CHANGELOG.md
@VERSION

<interfaces>
<!-- Key types and signatures the executor needs. Extracted from codebase. -->
<!-- Use these directly — no extra exploration required. -->

From pkg/exchange/types.go (ContractInfo at line 103):
```go
type ContractInfo struct {
    Symbol          string
    MinSize         float64
    StepSize        float64
    MaxSize         float64
    SizeDecimals    int
    PriceStep       float64
    PriceDecimals   int
    MaintenanceRate float64 // tier-1 maintenance margin rate as decimal (0.005 = 0.5%). 0 = unknown.
    // NEW: DeliveryDate time.Time — zero = normal perpetual, non-zero = scheduled delist/expiry
}
```
(No JSON tags — ContractInfo is in-memory only, additive change.)

From pkg/exchange/binance/adapter.go:398 (LoadAllContracts, anonymous unmarshal struct at line 404):
```go
var resp struct {
    Symbols []struct {
        Symbol  string `json:"symbol"`
        Status  string `json:"status"`
        Filters []struct { ... } `json:"filters"`
        // NEW: ContractType string `json:"contractType"`
        // NEW: DeliveryDate int64  `json:"deliveryDate"`
    } `json:"symbols"`
}
```
Loop at line 422 has `if sym.Status != "TRADING" { continue }` and builds
`ci := exchange.ContractInfo{Symbol: sym.Symbol}`. Add DeliveryDate population
after the filter loop, before `result[sym.Symbol] = ci`.

From pkg/exchange/bybit/adapter.go:457 (LoadAllContracts, anonymous struct at line 466):
```go
var resp struct {
    List []struct {
        Symbol          string      `json:"symbol"`
        Status          string      `json:"status"`
        LotSizeFilter   struct{...} `json:"lotSizeFilter"`
        PriceFilter     struct{...} `json:"priceFilter"`
        FundingInterval json.Number `json:"fundingInterval"`
        // NEW: ContractType string      `json:"contractType"`
        // NEW: DeliveryTime json.Number `json:"deliveryTime"` (string-encoded ms)
    } `json:"list"`
}
```
Build loop at line 486 uses status == "Trading" (title case). Bybit returns
`deliveryTime` as a string — parse via `strconv.ParseInt` (import already present).
Bybit linear perpetual contractType value is `"LinearPerpetual"`.

From internal/discovery/scanner.go:57 (Scanner struct):
```go
type Scanner struct {
    exchanges map[string]exchange.Exchange                  // line 58 — use this in refresh loop
    contracts map[string]map[string]exchange.ContractInfo   // line 59 — exchange → symbol → info
    db        *database.Client
    cfg       *config.Config
    log       *utils.Logger
    client    *http.Client
    // ...
    stopCh    chan struct{}
}
```

From internal/discovery/delist.go (reuse these constants/methods):
```go
const (
    delistPollInterval = 6 * time.Hour
    delistRedisPrefix  = "arb:delist:"   // line 15 — same package, unexported, usable directly
    delistBufferDays   = 7                // line 16 — 7-day TTL buffer after delist date
)

// Existing methods — call these from the new poller:
func (s *Scanner) SetDelistCooldown(symbol string, ttl time.Duration)  // line 228
func (s *Scanner) IsDelisted(symbol string) bool                        // line 72
func (s *Scanner) GetDelistDate(symbol string) string                   // line 234
```
`SetDelistCooldown` currently writes value `"manual"`. The new poller needs a
date string instead, so write directly via `s.db.SetWithTTL(delistRedisPrefix+symbol, dateStr, ttl)`
(or extend `SetDelistCooldown` with a value parameter — executor's choice, prefer direct call to keep call sites stable).

From internal/engine/engine.go:1225 (checkDelistPositions — the Binance-only guard at 1235-1239 must be removed):
```go
for _, pos := range positions {
    if !e.discovery.IsDelisted(pos.Symbol) { continue }
    // REMOVE THIS BLOCK (lines 1235-1239):
    if pos.LongExchange != "binance" && pos.ShortExchange != "binance" {
        e.log.Warn("DELIST WARNING: %s delisting but no Binance leg ...", ...)
        continue
    }
    // Skip if already closing, then preempt exit goroutine + BroadcastAlert + go closePositionEmergency(pos)
}
```

From internal/database/redis.go:62 (use for IsDelisted helper):
```go
func (c *Client) Get(key string) (string, error)          // line 62
func (c *Client) SetWithTTL(key, value string, ttl time.Duration) error  // line 69
```
`state.go` is in `package database` — add IsDelisted there. Redis key prefix
`"arb:delist:"` must be hard-coded inside `database` (cannot import `discovery` —
discovery imports database, not the other way).

From internal/spotengine/risk_gate.go:20 (checkRiskGate — current checks 1-6, dry-run is step 7 at line 107-112):
```go
// Current layout:
//   1. Capacity
//   2. Duplicate
//   3. Cooldown (HasSpotCooldown)
//   4. Persistence
//   5. Price gap gate
//   6. Maintenance rate gate
//   7. Dry-run short-circuit  ← MUST BECOME STEP 8
// New step 7 (inserted before dry-run):
if e.db.IsDelisted(symbol) {
    return RiskGateResult{Allowed: false, Reason: fmt.Sprintf("delist_%s", symbol)}
}
```

From internal/spotengine/engine.go:148 (exit entry point):
```go
func (e *SpotEngine) launchExit(pos *models.SpotFuturesPosition, reason string, isEmergency bool)
```
Call with `e.launchExit(pos, "delist", true)` from monitorTick active-scan branch,
gated by `!e.isExiting(pos.ID)` to avoid double-triggering.

From internal/spotengine/monitor.go:46 (monitorTick — active loop at line 56):
Active positions flow through `monitorPosition` at line 92. Insert the delist
check BEFORE `if pos.Status != models.SpotStatusActive { continue }` at line 89
so it applies to active and exiting states (but only trigger launchExit if
`pos.Status == SpotStatusActive` and not already exiting).

From internal/config/config.go:117 (risk filters block):
```go
DelistFilterEnabled bool  // line 117 — reuse as gate for BOTH article scraper and deliveryDate poller
// NEW: ContractRefreshInterval time.Duration  // default 1h; 0 disables the deliveryDate poller
```
Defaults block starts at line 558. Add `ContractRefreshInterval: time.Hour` alongside `DelistFilterEnabled: true` at line 563.

From cmd/main.go:383 (delist monitor wiring):
```go
if cfg.DelistFilterEnabled {
    scanner.StartDelistMonitor()
    log.Info("Binance delist monitor enabled")
    // NEW: scanner.StartContractRefresh()
    // NEW: log.Info("deliveryDate contract refresh enabled (interval: %s)", cfg.ContractRefreshInterval)
} else {
    log.Info("Binance delist monitor disabled by config")
}
```
</interfaces>
</context>

<tasks>

<task type="auto" tdd="true">
  <name>Task 1: Data layer — ContractInfo.DeliveryDate + Binance/Bybit parsing + unit test</name>
  <files>pkg/exchange/types.go, pkg/exchange/binance/adapter.go, pkg/exchange/bybit/adapter.go, pkg/exchange/binance/adapter_test.go</files>
  <behavior>
    - `ContractInfo` gains a `DeliveryDate time.Time` field; zero value = normal perpetual, non-zero = scheduled delist/expiry.
    - Binance `LoadAllContracts` populates `DeliveryDate` from `sym.DeliveryDate` (milliseconds) ONLY when `sym.ContractType == "PERPETUAL"` AND `sym.DeliveryDate > 0` AND `sym.DeliveryDate < 4102444800000` (year-2099 cutoff; skips the year-2100 sentinel `4133404800000`). Dated quarterlies (`ContractType != "PERPETUAL"`) and the sentinel leave `DeliveryDate` zero. The existing `status != "TRADING"` skip stays as-is (applies BEFORE the ContractInfo build).
    - Bybit `LoadAllContracts` populates `DeliveryDate` from `inst.DeliveryTime` (string-encoded ms, parse via `strconv.ParseInt(string(inst.DeliveryTime), 10, 64)`) ONLY when `inst.ContractType == "LinearPerpetual"` AND parsed value > 0 AND parsed value < 4102444800000. Existing `inst.Status != "Trading"` skip stays as-is.
    - A new unit test `TestLoadAllContracts_DeliveryDate` in `pkg/exchange/binance/adapter_test.go` exercises the Binance parser with three fixtures:
        (a) PERPETUAL with `deliveryDate=4133404800000` (sentinel) → `DeliveryDate.IsZero() == true`
        (b) PERPETUAL with `deliveryDate=1775638800000` (2026-04-08 09:00Z) → `DeliveryDate.UnixMilli() == 1775638800000`
        (c) non-PERPETUAL (e.g. `CURRENT_QUARTER`) with a real `deliveryDate` → `DeliveryDate.IsZero() == true`
      Test MUST stand up a `httptest.Server` that returns the fixture JSON for `/fapi/v1/exchangeInfo` and point the adapter at it. Follow the existing `adapter_test.go` pattern (check the file for the established mock-server style before writing).
  </behavior>
  <action>
    1. **pkg/exchange/types.go** — in the `ContractInfo` struct at line 103, add a new field:
       ```go
       DeliveryDate time.Time // zero = normal perpetual, non-zero = scheduled delist/expiry (in UTC)
       ```
       Verify `time` package is already imported (it is, per the existing `FundingRate.NextFunding time.Time` field). No other changes in this file.

    2. **pkg/exchange/binance/adapter.go** — in `LoadAllContracts` at line 398:
       - Extend the anonymous unmarshal struct at line 404 to add two fields inside the `Symbols []struct { ... }` block:
         ```go
         ContractType string `json:"contractType"`
         DeliveryDate int64  `json:"deliveryDate"`
         ```
       - Inside the `for _, sym := range resp.Symbols` loop, AFTER the existing filter loop populates `ci.MinSize/StepSize/PriceStep/etc.`, and BEFORE `result[sym.Symbol] = ci`, add:
         ```go
         const sentinelCutoff int64 = 4102444800000 // year-2099 in ms; skips Binance's 4133404800000 (year-2100) sentinel
         if sym.ContractType == "PERPETUAL" && sym.DeliveryDate > 0 && sym.DeliveryDate < sentinelCutoff {
             ci.DeliveryDate = time.UnixMilli(sym.DeliveryDate).UTC()
         }
         ```
       - Confirm `time` is already imported (it is — used by `ensureTimeOffset` etc.).
       - Do NOT touch the `sym.Status != "TRADING"` filter at line 423 — it stays. The new DeliveryDate check is additive: a `SETTLING`/`CLOSE` symbol is still skipped by the old guard, and a TRADING symbol with a future deliveryDate gets flagged via the new field.

    3. **pkg/exchange/bybit/adapter.go** — in `LoadAllContracts` at line 457:
       - Extend the anonymous struct at line 466 (`resp.List`) to add two fields:
         ```go
         ContractType string      `json:"contractType"`
         DeliveryTime json.Number `json:"deliveryTime"` // string-encoded ms on Bybit
         ```
       - Inside the build loop at line 486, after constructing the `contracts[inst.Symbol] = exchange.ContractInfo{...}` literal, replace the direct assignment with a local variable so we can populate `DeliveryDate` before storing:
         ```go
         ci := exchange.ContractInfo{
             Symbol:        inst.Symbol,
             MinSize:       minSize,
             StepSize:      stepSize,
             MaxSize:       maxSize,
             SizeDecimals:  countDecimals(stepSize),
             PriceStep:     priceStep,
             PriceDecimals: countDecimals(priceStep),
         }
         if inst.ContractType == "LinearPerpetual" {
             if dtMs, err := strconv.ParseInt(string(inst.DeliveryTime), 10, 64); err == nil {
                 const sentinelCutoff int64 = 4102444800000
                 if dtMs > 0 && dtMs < sentinelCutoff {
                     ci.DeliveryDate = time.UnixMilli(dtMs).UTC()
                 }
             }
         }
         contracts[inst.Symbol] = ci
         ```
       - Confirm `strconv` and `time` are already imported (strconv is used for the existing ParseFloat calls; time should be added if missing — check top of file).

    4. **pkg/exchange/binance/adapter_test.go** — add `TestLoadAllContracts_DeliveryDate`:
       - First READ the existing file to match its mock-server style (look for `httptest.NewServer` or similar; use the same client-construction pattern the existing tests use to point an Adapter at the mock URL).
       - Fixture JSON for `/fapi/v1/exchangeInfo` response body:
         ```json
         {
           "symbols": [
             {"symbol":"BTCUSDT","status":"TRADING","contractType":"PERPETUAL","deliveryDate":4133404800000,"filters":[{"filterType":"LOT_SIZE","minQty":"0.001","maxQty":"1000","stepSize":"0.001"},{"filterType":"PRICE_FILTER","tickSize":"0.1"}]},
             {"symbol":"RLSUSDT","status":"TRADING","contractType":"PERPETUAL","deliveryDate":1775638800000,"filters":[{"filterType":"LOT_SIZE","minQty":"1","maxQty":"1000000","stepSize":"1"},{"filterType":"PRICE_FILTER","tickSize":"0.0001"}]},
             {"symbol":"BTCUSDT_260926","status":"TRADING","contractType":"CURRENT_QUARTER","deliveryDate":1790000000000,"filters":[{"filterType":"LOT_SIZE","minQty":"1","maxQty":"1000","stepSize":"1"},{"filterType":"PRICE_FILTER","tickSize":"0.1"}]}
           ]
         }
         ```
         The mock server must ALSO respond to `/fapi/v1/leverageBracket` with `[]` (empty JSON array) so `loadMaintenanceRates` doesn't blow up when called at the end of `LoadAllContracts`.
       - Assertions:
         - `result["BTCUSDT"].DeliveryDate.IsZero()` is `true` (sentinel filtered)
         - `result["RLSUSDT"].DeliveryDate.IsZero()` is `false` AND `result["RLSUSDT"].DeliveryDate.UnixMilli() == 1775638800000`
         - `result["BTCUSDT_260926"].DeliveryDate.IsZero()` is `true` (non-PERPETUAL filtered)

    **Avoid**: Do NOT remove the existing `Status != "TRADING"` skip. Do NOT use `time.Unix(ms/1000, 0)` — use `time.UnixMilli(ms).UTC()` for consistency and UTC anchoring. Do NOT touch gateio/bitget/okx/bingx adapters — they don't expose equivalent fields per the approved plan and are out of scope.

    **Why**: Binance is the only exchange that delisted today, but Bybit linear perpetuals use the same pattern (`deliveryTime` on `/v5/market/instruments-info`). Supporting both now covers the two exchanges where live delist incidents have already occurred.
  </action>
  <verify>
    <automated>cd /var/solana/data/arb &amp;&amp; go build ./pkg/exchange/... &amp;&amp; go test ./pkg/exchange/binance/... -run TestLoadAllContracts_DeliveryDate -v &amp;&amp; go test ./pkg/exchange/bybit/... -count=1</automated>
  </verify>
  <done>
    - `ContractInfo` has `DeliveryDate time.Time` field; package compiles across pkg/exchange/...
    - `TestLoadAllContracts_DeliveryDate` passes: sentinel deliveryDate → zero, 2026-04-08 deliveryDate → populated, CURRENT_QUARTER → zero
    - All existing binance + bybit adapter tests still pass (`go test -count=1`)
    - Imports clean (goimports/gofmt), no unused variables
  </done>
</task>

<task type="auto">
  <name>Task 2: Periodic contract-refresh poller — new discovery/contract_refresh.go</name>
  <files>internal/discovery/contract_refresh.go, internal/discovery/scanner.go</files>
  <action>
    Create a new file `internal/discovery/contract_refresh.go` that adds two methods to `*Scanner` (same package as `delist.go`, so `delistRedisPrefix` and `delistBufferDays` are usable directly):

    1. **`func (s *Scanner) StartContractRefresh()`** — goroutine launcher modeled on `StartDelistMonitor` at `delist.go:52-68`:
       - Read interval from `s.cfg.ContractRefreshInterval`. If zero or negative, log `s.log.Info("contract refresh disabled (ContractRefreshInterval=0)")` and return without starting the goroutine.
       - Log `s.log.Info("contract refresh started (interval: %s)", interval)` before launching.
       - Launch a goroutine that:
         - Calls `s.refreshContractsAndFlagDelists()` immediately on startup.
         - Creates `ticker := time.NewTicker(interval)`, defers `ticker.Stop()`.
         - Loops on `select { case <-ticker.C: s.refreshContractsAndFlagDelists(); case <-s.stopCh: s.log.Info("contract refresh stopped"); return }`.

    2. **`func (s *Scanner) refreshContractsAndFlagDelists()`** — body:
       - Capture `now := time.Now().UTC()` and `lookahead := now.Add(365 * 24 * time.Hour)` (only flag deliveryDates within the next year).
       - Track counters: `newFlags := 0`, `totalFlags := 0`, `errExchanges := 0`.
       - Iterate `for exchName, exch := range s.exchanges` (the map is already on Scanner at line 58):
         - Call `contracts, err := exch.LoadAllContracts()`. On error, log `s.log.Warn("contract refresh: %s LoadAllContracts failed: %v", exchName, err)` and `errExchanges++`; continue.
         - For each `symbol, ci := range contracts`:
           - Skip if `ci.DeliveryDate.IsZero()`.
           - Skip if `ci.DeliveryDate.After(lookahead)` (defensive — should not happen because adapters apply the year-2099 cutoff, but cheap).
           - `totalFlags++`
           - Compute `ttl := time.Until(ci.DeliveryDate) + time.Duration(delistBufferDays)*24*time.Hour`. Clamp to `>= time.Hour` to match `pollDelistAnnouncements` semantics.
           - Check existing key: `existing, _ := s.db.Get(delistRedisPrefix + symbol)`. If `existing == ""` this is a NEW flag — emit `s.log.Warn("contract refresh: delivery-date delist detected: %s on %s scheduled for %s (in %s)", symbol, exchName, ci.DeliveryDate.Format("2006-01-02"), time.Until(ci.DeliveryDate).Round(time.Hour))` and `newFlags++`.
           - Write: `dateStr := ci.DeliveryDate.Format("2006-01-02")`; `if err := s.db.SetWithTTL(delistRedisPrefix+symbol, dateStr, ttl); err != nil { s.log.Error("contract refresh: failed to write %s: %v", delistRedisPrefix+symbol, err); continue }`. (Idempotent overwrite is fine — same date wins.)
       - Also refresh scanner's in-memory contracts cache with the latest map so step-8 filter sees fresh data on the next scan:
         - Build `updated := make(map[string]map[string]exchange.ContractInfo, len(s.exchanges))` as you iterate (store each successful `contracts` result into `updated[exchName]`).
         - After the loop, call `s.SetContracts(updated)` (exists at `scanner.go:128`).
       - Final summary log: `s.log.Info("contract refresh: scanned %d exchanges, flagged %d symbols (%d new), %d exchange errors", len(s.exchanges), totalFlags, newFlags, errExchanges)`.

    **Imports for the new file:**
    ```go
    package discovery

    import (
        "time"

        "arb/pkg/exchange"
    )
    ```

    **Do NOT modify scanner.go** in this task EXCEPT to verify `SetContracts` signature at line 128 matches `map[string]map[string]exchange.ContractInfo` (it does per interfaces block). If it doesn't, adjust the `updated` build step accordingly — do not change the scanner.go signature.

    **Why this design:**
    - Key reuse with `delistRedisPrefix` means `Scanner.IsDelisted()`, scanner step-8 filter at `scanner.go:954`, and engine `checkDelistPositions` at `engine.go:1225` all work unchanged.
    - Cadence is configurable (next task adds the config field). Default 1h balances API weight against maximum exposure window from announcement.
    - The article scraper at `delist.go` continues running — this is belt-and-suspenders. When both writers target the same key with the same date, writes are idempotent.
    - gateio/bitget/okx/bingx adapters return `ci.DeliveryDate.IsZero() == true` (they don't populate the field), so they're harmless to iterate — they just contribute no flags. Scanner contract cache still gets refreshed for them, which is a free win.

    **Avoid**: Do NOT write to the Redis key if `DeliveryDate.IsZero()` — would clobber entries from the article scraper. Do NOT use `SetDelistCooldown` (it writes value `"manual"`, losing the date). Do NOT gate the poller on exchange name — it MUST iterate every entry in `s.exchanges` so gate/bitget/okx/bingx contracts get refreshed in-memory (flags simply won't fire for them because DeliveryDate is zero).
  </action>
  <verify>
    <automated>cd /var/solana/data/arb &amp;&amp; go build ./internal/discovery/... &amp;&amp; go vet ./internal/discovery/...</automated>
  </verify>
  <done>
    - `internal/discovery/contract_refresh.go` exists with `StartContractRefresh` and `refreshContractsAndFlagDelists` methods on `*Scanner`
    - Package `internal/discovery` compiles and vets clean
    - Loop iterates `s.exchanges`, writes `arb:delist:{SYMBOL}` via `s.db.SetWithTTL` with a date string value, and refreshes `s.SetContracts` with the fresh map
    - New-flag log line uses WARN level and includes symbol + exchange + date + time-until
    - Zero-interval config gracefully disables the poller
  </done>
</task>

<task type="auto">
  <name>Task 3: Config field + main.go wiring under DelistFilterEnabled</name>
  <files>internal/config/config.go, cmd/main.go</files>
  <action>
    1. **internal/config/config.go** — add the new tuning field:
       - At line 117 (immediately after `DelistFilterEnabled bool`), add:
         ```go
         ContractRefreshInterval         time.Duration // deliveryDate-based delist poller cadence (default 1h, 0 disables)
         ```
         Verify `time` package is already imported (it is — used by `ScanMinutes` neighborhood).
       - In the defaults block at line 558 (`Default()` or equivalent — the block that sets `DelistFilterEnabled: true` at line 563), add right after `DelistFilterEnabled: true,`:
         ```go
         ContractRefreshInterval: time.Hour,
         ```
       - **Do NOT add a JSON parse/serialize path unless the surrounding struct already does so for other Duration fields.** The field is read directly from the Go struct at startup; dashboard tuning is out of scope for this safety fix. If other duration-typed fields in the risk block ARE serialized via `applyJSON`/`toJSON` (search for `DelistFilter` usage pattern at lines 786 and 1365), mirror the same pattern so the field roundtrips cleanly. If not, leave it as a code-default only.
       - **CRITICAL**: Do NOT touch `config.json`. The default lives in Go; any runtime override happens via env var or a future dashboard addition, out of scope here.

    2. **cmd/main.go** — wire the new poller at the existing delist-monitor block at line 383:
       - Locate:
         ```go
         if cfg.DelistFilterEnabled {
             scanner.StartDelistMonitor()
             log.Info("Binance delist monitor enabled")
         } else {
             log.Info("Binance delist monitor disabled by config")
         }
         ```
       - Inside the `if` branch, immediately after the existing `log.Info("Binance delist monitor enabled")`, add:
         ```go
         scanner.StartContractRefresh()
         if cfg.ContractRefreshInterval > 0 {
             log.Info("deliveryDate contract refresh enabled (interval: %s)", cfg.ContractRefreshInterval)
         } else {
             log.Info("deliveryDate contract refresh disabled (ContractRefreshInterval=0)")
         }
         ```
       - The poller's own startup logs from Task 2 will also fire; the extra main.go log line makes the wiring obvious at boot.

    **Why reuse `DelistFilterEnabled`:**
    The existing dashboard Safety-tab toggle already gates delist detection. Reusing it ties the new mechanism to the same user-visible switch, avoiding a dual-toggle UX. Per the approved plan this is a replacement/augmentation of an already-gated feature, so `feedback_risk_configurable_switch` (which requires a new toggle for net-new risk features) does NOT apply here — both the scraper and the new poller are the same feature.

    **Avoid**: Do NOT create a new config toggle for the refresher. Do NOT add a dashboard JSON field unless other similar Duration fields in the same config section are already JSON-serialized (verify first). Do NOT touch `config.json`.
  </action>
  <verify>
    <automated>cd /var/solana/data/arb &amp;&amp; go build ./... &amp;&amp; go vet ./internal/config/... ./cmd/...</automated>
  </verify>
  <done>
    - `Config.ContractRefreshInterval time.Duration` field exists with default `time.Hour`
    - `cmd/main.go` calls `scanner.StartContractRefresh()` inside the `DelistFilterEnabled` block, right after `StartDelistMonitor`
    - Full repo builds (`go build ./...`)
    - `config.json` is unchanged
  </done>
</task>

<task type="auto">
  <name>Task 4: Generalize perp-perp delist exit + spot-futures parity (IsDelisted helper, risk_gate step 7, monitor active scan)</name>
  <files>internal/database/state.go, internal/engine/engine.go, internal/spotengine/risk_gate.go, internal/spotengine/monitor.go</files>
  <action>
    1. **internal/database/state.go** — add a new helper method at the end of the file:
       ```go
       // IsDelisted returns true if the given symbol is on the shared delist blacklist
       // at key arb:delist:{symbol}. Mirrors discovery.Scanner.IsDelisted for callers
       // (e.g. spotengine) that don't hold a Scanner reference. Fail-open: Redis errors
       // return false so a transient outage does not block entries.
       func (c *Client) IsDelisted(symbol string) bool {
           val, err := c.Get("arb:delist:" + symbol)
           return err == nil && val != ""
       }
       ```
       **Important**: Hard-code the `"arb:delist:"` prefix — the `delistRedisPrefix` constant is unexported in `package discovery`, and `database` cannot import `discovery` (would create a cycle: discovery already imports database). Add a comment referencing the source-of-truth constant: `// NOTE: must match internal/discovery/delist.go delistRedisPrefix`.

    2. **internal/engine/engine.go** — generalize `checkDelistPositions` at line 1225. Remove the Binance-only guard at lines 1235-1239:
       ```go
       // DELETE these five lines:
       // Only auto-exit if Binance is one of the legs.
       if pos.LongExchange != "binance" && pos.ShortExchange != "binance" {
           e.log.Warn("DELIST WARNING: %s delisting but no Binance leg (%s/%s), skipping auto-exit",
               pos.Symbol, pos.LongExchange, pos.ShortExchange)
           continue
       }
       ```
       After removal, the loop immediately proceeds to the status check and emergency close. Update the log message at line 1244 from `"Binance delisting %s"` to `"delisting %s"` (it will now fire for Bybit-only positions too):
       ```go
       e.log.Warn("DELIST ALERT: %s is being delisted, emergency closing %s (%s/%s)",
           pos.Symbol, pos.ID, pos.LongExchange, pos.ShortExchange)
       ```
       And update the `BroadcastAlert` message string at line 1254 from `"Binance delisting %s — ..."` to `"Delisting %s — ..."` for consistency.
       Leave everything else in the function unchanged — `cancelExitGoroutine`, `BroadcastAlert`, `go closePositionEmergency(pos)` all stay.

    3. **internal/spotengine/risk_gate.go** — insert a new check between step 6 (maintenance rate) at line 105 and step 7 (dry-run) at line 107. The new check becomes step 7, and dry-run shifts to step 8:
       ```go
       // 7. Delist check: reject if the symbol is on the shared delist blacklist.
       //    Reuses the arb:delist:{SYMBOL} key maintained by discovery.Scanner
       //    (article scraper + deliveryDate poller). Fail-open on Redis errors
       //    via database.Client.IsDelisted.
       if e.db.IsDelisted(symbol) {
           return RiskGateResult{Allowed: false, Reason: fmt.Sprintf("delist_%s", symbol)}
       }

       // 8. Dry-run: all real checks passed — log the would-be entry and skip execution.
       if e.cfg.SpotFuturesDryRun {
           ...
       }
       ```
       Update the comment numbering on step 8 (dry-run) to reflect its new position. No other changes in this file.

    4. **internal/spotengine/monitor.go** — add an active-position delist scan inside `monitorTick` at line 46. Insert the check inside the `for _, pos := range positions` loop at line 56, BEFORE the existing status branches (pending/exiting/active), so it fires for Active positions only but is evaluated on every tick:
       ```go
       for _, pos := range positions {
           // Delist scan: flagged symbols get a forced emergency exit regardless
           // of existing exit triggers. Fail-open (IsDelisted returns false on Redis errors).
           if pos.Status == models.SpotStatusActive && e.db.IsDelisted(pos.Symbol) {
               if !e.isExiting(pos.ID) {
                   e.log.Warn("monitor: %s on %s scheduled for delist — triggering emergency exit", pos.Symbol, pos.Exchange)
                   e.launchExit(pos, "delist", true)
               }
               continue
           }

           if pos.Status == models.SpotStatusPending {
               ...existing logic...
           }
           ...
       }
       ```
       `e.isExiting(pos.ID)` is an existing guard used at line 74 — reuse verbatim. `e.launchExit` signature is `launchExit(pos, reason string, isEmergency bool)` at `engine.go:148`. Passing `isEmergency=true` ensures the exit takes the fastest path.

    **Dependency wiring rationale:**
    The approved plan explicitly recommends Option B (database helper) over Option A (inject Scanner into SpotEngine) for minimal blast radius. `SpotEngine` already holds `e.db *database.Client` — no constructor changes needed.

    **Avoid**:
    - Do NOT store `"exchange"` metadata in the delist Redis value. Keeping value = date string preserves compatibility with `Scanner.GetDelistDate`.
    - Do NOT call `e.launchExit` for Pending or Exiting spot-futures positions — they're already in a terminal workflow. The `pos.Status == SpotStatusActive` guard prevents double-triggering.
    - Do NOT import `internal/discovery` from `internal/database` — that creates an import cycle (discovery imports database). The hard-coded `"arb:delist:"` prefix is intentional.
  </action>
  <verify>
    <automated>cd /var/solana/data/arb &amp;&amp; go build ./... &amp;&amp; go vet ./internal/engine/... ./internal/spotengine/... ./internal/database/... &amp;&amp; go test ./internal/database/... ./internal/engine/... ./internal/spotengine/... -count=1</automated>
  </verify>
  <done>
    - `database.Client.IsDelisted(symbol)` exists and reads `arb:delist:{symbol}`
    - `engine.checkDelistPositions` no longer has the `pos.LongExchange != "binance" && pos.ShortExchange != "binance"` guard; log messages say "delisting" not "Binance delisting"
    - `spotengine.checkRiskGate` has a new step 7 that returns `delist_{symbol}` reason when `e.db.IsDelisted(symbol)` is true, BEFORE the dry-run short-circuit
    - `spotengine.monitorTick` active-position loop triggers `e.launchExit(pos, "delist", true)` for Active positions flagged as delisted, gated on `!e.isExiting(pos.ID)`
    - Full repo builds and vets clean
    - Existing engine + spotengine + database tests still pass
  </done>
</task>

<task type="auto">
  <name>Task 5: Bump CHANGELOG.md + VERSION, final full build</name>
  <files>CHANGELOG.md, VERSION</files>
  <action>
    1. **VERSION** — overwrite file contents with:
       ```
       0.30.0
       ```
       (single line, trailing newline). Current value is `0.29.3`. Minor bump (`0.29.x → 0.30.0`) reflects the new risk-safety feature across data layer + two engines.

    2. **CHANGELOG.md** — add a new `## [0.30.0] - 2026-04-08` section at the top, immediately after the `# Changelog` header block and BEFORE the existing `## [0.29.3] - 2026-04-07` entry:
       ```markdown
       ## [0.30.0] - 2026-04-08

       ### Fixed
       - **Delist detection missed Binance 2026-04-08 batch** — article-scraping parser at `internal/discovery/delist.go` only reads titles and the generic "Multiple Perpetual Contracts" announcement bypassed both regex patterns, letting the bot enter RLSUSDT for a loss. Replaced with a deliveryDate-based poller that reads `/fapi/v1/exchangeInfo` and `/v5/market/instruments-info` directly.

       ### Added
       - **deliveryDate-based delist poller** — new `internal/discovery/contract_refresh.go` periodically calls `LoadAllContracts()` on every configured exchange and writes `arb:delist:{SYMBOL}` for any perpetual whose `DeliveryDate` is set to a near-future value. Reuses the existing Redis key so scanner step-8 filter and engine `checkDelistPositions` work unchanged. Default cadence 1h (configurable via `ContractRefreshInterval`).
       - **ContractInfo.DeliveryDate field** — `pkg/exchange/types.go` gains `DeliveryDate time.Time`; populated by Binance adapter (from `deliveryDate` + `contractType=="PERPETUAL"`) and Bybit adapter (from `deliveryTime` + `contractType=="LinearPerpetual"`), both with a year-2099 sentinel cutoff to skip Binance's year-2100 default. Gateio/bitget/okx/bingx leave the field zero — out of scope this PR.
       - **Spot-futures delist parity** — `SpotEngine.checkRiskGate` now has a delist check as step 7 (before dry-run) that returns `delist_{symbol}` for flagged futures symbols. `SpotEngine.monitorTick` scans active positions every tick and triggers `launchExit(pos, "delist", true)` if a held symbol gets flagged after entry. Closes the gap where spot-futures had zero delist awareness.
       - **`database.Client.IsDelisted(symbol)` helper** — lets `internal/spotengine` query the shared `arb:delist:` blacklist without importing `internal/discovery` (which would create an import cycle). Fail-open on Redis errors.
       - **`config.ContractRefreshInterval`** — new duration tuning knob, default `1h`, `0` disables the poller. Gated by the existing `DelistFilterEnabled` toggle.

       ### Changed
       - **`engine.checkDelistPositions` no longer Binance-only** — the old guard skipped auto-exit unless a leg was on Binance. With Bybit `deliveryTime` parity, any flagged symbol now auto-exits regardless of which exchange is the leg. Log message changed from "Binance delisting" to "delisting".
       ```

    3. **Final full build** — as a done-criteria check, run `make build` once to confirm the frontend is still present (go:embed requirement) AND the Go binary compiles with all changes integrated. If `web/dist/` is missing, fall back to `go build ./...` and note in the completion summary that a frontend rebuild is needed before deployment — DO NOT run `npm install` under any circumstances (axios lockdown per CLAUDE.md); `npm ci` is the only permitted install path and should not be needed because lockfile + dist are present.

    **Avoid**:
    - Do NOT run `npm install` / `npm update` / `npx` / `pnpm install` in `web/`. Use `npm ci` ONLY if strictly required; the frontend is unchanged by this PR so no rebuild should be needed in the common case.
    - Do NOT touch `config.json`.
    - Do NOT skip the CHANGELOG update. Per CLAUDE.md `feedback_changelog_versioning`, every commit must update both `CHANGELOG.md` and `VERSION`.
  </action>
  <verify>
    <automated>cd /var/solana/data/arb &amp;&amp; grep -q "^0.30.0$" VERSION &amp;&amp; grep -q "## \[0.30.0\] - 2026-04-08" CHANGELOG.md &amp;&amp; grep -q "deliveryDate" CHANGELOG.md &amp;&amp; go build ./...</automated>
  </verify>
  <done>
    - `VERSION` contains `0.30.0`
    - `CHANGELOG.md` has a `## [0.30.0] - 2026-04-08` section with Fixed/Added/Changed subsections describing the deliveryDate poller, ContractInfo.DeliveryDate, spot-futures parity, IsDelisted helper, ContractRefreshInterval, and the Binance-only guard removal
    - Full repo builds (`go build ./...`) with all task 1-5 changes integrated
    - No `config.json` modifications
  </done>
</task>

</tasks>

<verification>
**Build-level**
- `go build ./...` succeeds with all tasks integrated
- `go vet ./...` clean
- `go test ./pkg/exchange/binance/... ./pkg/exchange/bybit/... ./internal/discovery/... ./internal/engine/... ./internal/spotengine/... ./internal/database/... -count=1` passes
- Full `make build` succeeds (frontend already built; `web/dist/` present)

**Integration smoke (post-deploy, live read-only probe per approved plan)**
Live fixtures from the approved plan are still valid as of 2026-04-08:
```bash
curl -s 'https://fapi.binance.com/fapi/v1/exchangeInfo' | \
  python3 -c "import json,sys; d=json.load(sys.stdin); \
    [print(s['symbol'], s.get('contractType'), s.get('status'), s.get('deliveryDate')) \
     for s in d['symbols'] if s['symbol'] in ('RLSUSDT','OLUSDT','HIPPOUSDT','PUFFERUSDT','BTCUSDT')]"
```
Expected: RLSUSDT / OLUSDT / HIPPOUSDT / PUFFERUSDT have `deliveryDate=1775638800000` (2026-04-08 09:00Z); BTCUSDT has `4133404800000` (sentinel).

After bot restart with the fix, within 1h:
- `redis-cli -n 2 KEYS 'arb:delist:*'` shows entries for the four delisted symbols
- Engine log contains `"contract refresh: delivery-date delist detected: RLSUSDT on binance ..."` WARN line
- Scanner step-8 filter log fires for any opportunity that surfaces for those symbols
- If any position is still open for a flagged symbol, `checkDelistPositions` triggers emergency close on the next scan tick (no longer blocked by the Binance-only guard)
- Spot-futures risk_gate logs `delist_{symbol}` rejection for any spot-futures opportunity on a flagged symbol

**Regression checks**
- Existing `StartDelistMonitor` article scraper still runs (log line on startup)
- Normal perpetuals (BTC, ETH, SOL) are NOT added to the blacklist — their sentinel deliveryDate is filtered by the `< sentinelCutoff` guard
- Quarterly contracts (if any exist in `/fapi/v1/exchangeInfo`) are NOT flagged — `contractType != "PERPETUAL"` filters them out
- Perp-perp live trading continues uninterrupted
</verification>

<success_criteria>
1. All 5 tasks complete; `go build ./...` passes
2. `pkg/exchange/binance/adapter_test.go::TestLoadAllContracts_DeliveryDate` passes
3. `internal/discovery/contract_refresh.go` exists with `StartContractRefresh` + `refreshContractsAndFlagDelists` methods on `*Scanner`
4. `internal/discovery/scanner.go` exchanges map (line 58) is iterated by the new refresh loop (via method receiver, no scanner.go struct changes)
5. `Config.ContractRefreshInterval time.Duration` field exists with default `time.Hour`; `cmd/main.go` calls `scanner.StartContractRefresh()` under `DelistFilterEnabled`
6. `engine.checkDelistPositions` no longer has the Binance-only leg guard (lines 1235-1239 removed)
7. `database.Client.IsDelisted(symbol) bool` helper exists in `internal/database/state.go`
8. `spotengine.checkRiskGate` rejects flagged symbols at new step 7 (before dry-run)
9. `spotengine.monitorTick` triggers `launchExit(pos, "delist", true)` for Active positions with flagged symbols
10. `VERSION` = `0.30.0`, `CHANGELOG.md` has `## [0.30.0] - 2026-04-08` section
11. `config.json` UNCHANGED (verify via `git status config.json` — should show no changes)
12. No npm/pnpm install run; `web/` unchanged (`git status web/` empty)
</success_criteria>

<output>
After completion, create `.planning/quick/260408-ugr-deliverydate-based-delist-detection/260408-ugr-01-SUMMARY.md` capturing:
- Files touched with one-line description per file
- Exact line numbers of key edits (Binance adapter DeliveryDate parse, Bybit adapter DeliveryDate parse, engine guard removal, risk_gate step 7, monitor active scan)
- Build + test results
- VERSION bump
- Any deviations from the plan (e.g. if JSON serialization for ContractRefreshInterval was added or skipped based on surrounding patterns)
- Next steps: deploy + monitor `contract refresh` WARN log lines in engine log within first hour of restart
</output>
