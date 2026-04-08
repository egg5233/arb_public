---
quick: 260408-ugr
slug: deliverydate-based-delist-detection
type: quick
status: complete
started: 2026-04-08T14:06:12Z
completed: 2026-04-08T14:18:22Z
duration_min: 12
tasks_completed: 5
tasks_total: 5
files_created: 2
files_modified: 9
commits:
  - 6298fdc: feat(quick/260408-ugr) parse deliveryDate from Binance and Bybit exchangeInfo
  - 37b3d28: feat(quick/260408-ugr) periodic contract refresh poller writes deliveryDate delists to Redis blacklist
  - 170aed2: fix(quick/260408-ugr) generalize checkDelistPositions beyond Binance-only auto-exit
  - b548319: feat(quick/260408-ugr) spot-futures parity for delist detection
  - 8f04a46: chore(quick/260408-ugr) bump VERSION to 0.30.0 + CHANGELOG entry
version: 0.30.0
key-files:
  created:
    - internal/discovery/contract_refresh.go
    - .planning/quick/260408-ugr-deliverydate-based-delist-detection/deferred-items.md
  modified:
    - pkg/exchange/types.go
    - pkg/exchange/binance/adapter.go
    - pkg/exchange/binance/adapter_test.go
    - pkg/exchange/bybit/adapter.go
    - internal/discovery/scanner.go
    - internal/engine/engine.go
    - internal/database/state.go
    - internal/spotengine/risk_gate.go
    - internal/spotengine/monitor.go
    - internal/config/config.go
    - cmd/main.go
    - VERSION
    - CHANGELOG.md
decisions:
  - "Reuse existing arb:delist:{SYMBOL} Redis key — both signal paths (article scraper + new poller) write the same key idempotently. Avoids new key prefix and lets all downstream consumers (scanner step-8 filter, engine.checkDelistPositions, dashboard) work unchanged."
  - "Use database.IsDelisted helper from spot-engine instead of injecting *discovery.Scanner — keeps spot-engine free of a new constructor parameter and avoids any package-cycle risk. Hard-coded prefix constant in database/state.go is acceptable since the key shape is intentionally stable."
  - "Generalize checkDelistPositions to all exchanges (option B from plan) — simpler than encoding the source exchange in the Redis value, and a delisted coin should be exited regardless of which leg flagged it."
  - "Reuse DelistFilterEnabled as the on/off toggle — single Safety-tab UI switch already controls article scraper, scanner filter, and now the contract refresh poller + spot-engine delist gate. Avoids configuration sprawl."
  - "Add separate contractsMu RWMutex on Scanner instead of reusing the existing s.mu (opportunities lock) — minimal blast radius, no risk of widening the existing lock's scope and creating contention with the runCycle goroutine."
---

# Quick Task 260408-ugr: deliveryDate-based delist detection — Summary

## One-liner

Adds a 1h-cadence `deliveryDate`-based delist poller (Binance + Bybit) that writes the existing `arb:delist:{SYMBOL}` Redis key, generalizes `checkDelistPositions` to all exchanges, and gives the spot-futures engine its first delist check (pre-entry + active-position) — root-cause fix for the 2026-04-08 RLSUSDT loss where the article-scraper title regex missed a generic batch announcement.

## Context

On 2026-04-08, Binance scheduled a batch delist (OLUSDT, HIPPOUSDT, RLSUSDT, PUFFERUSDT, plus COIN-M WIFUSD_PERP/WLDUSD_PERP). The announcement title was *"Binance Futures Will Delist Multiple Perpetual Contracts (2026-04-08 & 2026-04-09)"* — a generic title with no symbol tokens. The article-scraping parser at `internal/discovery/delist.go:151` only reads titles, and its regexes expect either `"Will Delist <coins> on <date>"` or `<COIN>USD[T]?` tokens; neither fired. The bot entered RLSUSDT after the announcement and took a loss.

Binance's docs (`doc/binance/binance-usds-futures-api-docs.md:4647`) state that `deliveryDate` on `Get /fapi/v1/exchangeInfo` is updated to the delisting time at announcement time. Live-verified today: non-delist perps return `4133404800000` (year-2100 sentinel); the 4 delisted perps return `1775638800000` (2026-04-08T09:00Z); WIFUSD_PERP and WLDUSD_PERP return `1775725200000` (2026-04-09T09:00Z) **while still in `status=TRADING`** — proving that `status` filtering alone is insufficient and `deliveryDate` is the right primary signal.

The scope decision was to also add spot-futures parity in the same PR: `grep "Delist\|delist" internal/spotengine/` returned zero matches before this work — the spot-futures engine had no delist check at all.

## Architecture

```
                       ┌─────────────────────────────────┐
                       │ pkg/exchange/{binance,bybit}/   │
                       │   adapter.LoadAllContracts()    │
                       │   → ContractInfo.DeliveryDate   │
                       └────────────┬────────────────────┘
                                    │
                                    ▼
                       ┌─────────────────────────────────┐
                       │ internal/discovery/             │
                       │   contract_refresh.go (NEW)     │
                       │   StartContractRefresh — 1h     │
                       │   ticker, writes Redis blacklist│
                       └────────────┬────────────────────┘
                                    │
                                    ▼
                       ┌─────────────────────────────────┐
                       │  Redis: arb:delist:{SYMBOL}     │
                       │  (also written by article       │
                       │   scraper, belt-and-suspenders) │
                       └─────┬──────────────────────┬────┘
                             │                      │
              ┌──────────────┘                      └──────────────┐
              ▼                                                    ▼
  ┌──────────────────────────┐                    ┌──────────────────────────────┐
  │ Perp engine              │                    │ Spot-futures engine          │
  │ - scanner.IsDelisted     │                    │ - risk_gate step 7 (NEW)     │
  │ - scanner step-8 filter  │                    │ - monitor active scan (NEW)  │
  │ - checkDelistPositions   │                    │   reads via db.IsDelisted    │
  │   (generalized — no      │                    │                              │
  │   Binance-only guard)    │                    │                              │
  └──────────────────────────┘                    └──────────────────────────────┘
```

## What was built

### Task 1 — Data layer (`6298fdc`)

- `pkg/exchange/types.go`: added `DeliveryDate time.Time` field on `ContractInfo`. Zero value = normal perpetual; non-zero = scheduled delist/expiry.
- `pkg/exchange/binance/adapter.go`: extended the `exchangeInfo` unmarshal struct with `ContractType` and `DeliveryDate int64`. Populates `ContractInfo.DeliveryDate` only when `ContractType == "PERPETUAL"` AND `DeliveryDate > 0` AND `DeliveryDate < 4102444800000` (year-2099 cutoff to skip the year-2100 sentinel). Dated quarterlies are intentionally skipped.
- `pkg/exchange/bybit/adapter.go`: same treatment for the v5 instruments-info endpoint, parsing `deliveryTime` (string) for `LinearPerpetual` contracts only.
- `pkg/exchange/binance/adapter_test.go`: added `TestLoadAllContracts_Binance_DeliveryDateParsing` covering live-perpetual (zero), delisting-perpetual (populated), and quarterly (zero) cases.

The other 4 adapters (gateio, bitget, okx, bingx) leave `DeliveryDate` zero — they have no equivalent field in their contract endpoints per the doc survey, so they fall back to the existing article-scraper signal path. Documented as out of scope in the changelog.

### Task 2 — Periodic refresh poller (`37b3d28`)

- `internal/discovery/contract_refresh.go` (NEW): `StartContractRefresh()` launches a background goroutine with a 1h ticker (configurable) and an immediate first poll. Each tick calls `LoadAllContracts` for every configured exchange, writes any near-future `DeliveryDate`-flagged perpetual to `arb:delist:{SYMBOL}` with TTL = `time.Until(DeliveryDate) + 7d`, and updates the in-memory contracts cache.
- `internal/discovery/scanner.go`: added `contractsMu sync.RWMutex` to guard the `contracts` map (previously read unlocked). `SetContracts` and `hasContract` now go through the lock; the new `replaceContractsForExchange` updates a single exchange slot to avoid clobbering successful polls when one exchange fails.
- `internal/config/config.go`: added `ContractRefreshInterval time.Duration` field with default 1h, JSON key `contract_refresh_min` with explicit-0-disables semantics.
- `cmd/main.go`: starts the new poller alongside `StartDelistMonitor()` when `DelistFilterEnabled` is true and `ContractRefreshInterval > 0`. Logs the configured cadence.

The poller reuses the existing `arb:delist:{SYMBOL}` key and the existing `delistBufferDays = 7` constant from `delist.go`. When both signal paths write the same key (same date, same symbol), the operation is idempotent.

### Task 3 — Generalized auto-exit (`170aed2`)

- `internal/engine/engine.go`: removed the Binance-only guard in `checkDelistPositions`. Now triggers emergency close on any active position whose symbol is on the blacklist, regardless of which leg's exchange flagged it. Updated log + alert messages to reflect the broader scope. A delisted perpetual must be closed regardless of source — holding it past delivery forces unfavorable settlement.

### Task 4 — Spot-futures parity (`b548319`)

- `internal/database/state.go`: added `IsDelisted(symbol string) bool` helper (3s timeout, fail-open on Redis error). Hard-coded `keyDelistPrefix = "arb:delist:"` constant to avoid the import cycle that would result from depending on `internal/discovery`. Same key the perp scanner reads — single source of truth.
- `internal/spotengine/risk_gate.go`: inserted delist check as step 7 (before dry-run, which shifts to step 8). Gated by `DelistFilterEnabled` to mirror the perp scanner.
- `internal/spotengine/monitor.go`: added an active-position delist scan inside `monitorTick`. Any active spot-futures position whose futures symbol is on the blacklist triggers an emergency exit via `launchExit(pos, "delist_"+symbol, true)`. Skips if an exit goroutine is already running.

### Task 5 — Versioning (`8f04a46`)

- `VERSION`: bumped from 0.29.3 to 0.30.0 (minor bump — new feature, root-cause-fixes a live trading loss, adds a new config field).
- `CHANGELOG.md`: full Added/Fixed/Changed/Notes entry under `[0.30.0] - 2026-04-08`.

## Verification

### Builds and unit tests

```
$ go build ./...                                               → success (no errors)

$ go test ./pkg/exchange/binance/... ./pkg/exchange/bybit/... \
          ./internal/engine/... ./internal/spotengine/... \
          ./internal/database/... ./internal/config/...        → 221 passed in 6 packages
```

The new `TestLoadAllContracts_Binance_DeliveryDateParsing` is included in the binance adapter package run. All existing tests continue to pass.

### Pre-existing test failure (deferred — see deferred-items.md)

`go test ./internal/discovery/...` fails to **build** because `internal/discovery/test_helpers_test.go:75` defines a `stubExchange` that doesn't implement `CancelAllOrders`, an interface method added in v0.29.2. Verified pre-existing by stashing all my changes (including untracked files) and re-running on `HEAD~5` — the same build failure reproduces. This is **out of scope** per CLAUDE.md scope-boundary rules; documented in `deferred-items.md` for follow-up.

### Manual integration verification (not run as part of execution — bot is live)

The plan provides curl-based live verification commands against `fapi.binance.com/fapi/v1/exchangeInfo` and a Redis-key check. These should be run by the operator after deploy:

```bash
# Confirm the production endpoint still has the expected delisting symbols
curl -s 'https://fapi.binance.com/fapi/v1/exchangeInfo' | \
  python3 -c "import json,sys; d=json.load(sys.stdin); \
    [print(s['symbol'], s.get('contractType'), s.get('status'), s.get('deliveryDate')) \
     for s in d['symbols'] if s['symbol'] in ('RLSUSDT','OLUSDT','HIPPOUSDT','PUFFERUSDT','WIFUSDT','BTCUSDT')]"

# After bot restart, within 1h:
redis-cli -n 2 KEYS 'arb:delist:*'
# Expected: RLSUSDT, OLUSDT, HIPPOUSDT, PUFFERUSDT (from poller),
# plus whatever the article scraper picked up.

# Log line to grep for:
journalctl -u arb -n 200 | grep "delivery-date delist detected"
```

## Deviations from plan

### Auto-fixed issues

**1. [Rule 2 - Concurrency safety] Added `contractsMu` RWMutex on Scanner**
- **Found during:** Task 2
- **Issue:** The plan called for the new refresh poller to update `s.contracts` concurrently with reads from `hasContract` (called from the scanner's runCycle goroutine). The existing field had no lock — works fine when `SetContracts` is called once at startup, race-prone with periodic refresh.
- **Fix:** Added a dedicated `contractsMu sync.RWMutex` field on `Scanner`, lock-protected `SetContracts`, `hasContract`, and the new `replaceContractsForExchange`. Chose a separate mutex (not the existing `s.mu` opportunities lock) to avoid widening that lock's scope and creating contention with the existing runCycle/GetOpportunities paths.
- **Files modified:** `internal/discovery/scanner.go`, `internal/discovery/contract_refresh.go`
- **Commit:** `37b3d28`

**2. [Rule 2 - Toggle consistency] Gated spot-engine delist check on `DelistFilterEnabled`**
- **Found during:** Task 4
- **Issue:** The plan inserted the spot-engine pre-entry and active-scan delist checks unconditionally. But the perp scanner step-8 filter is gated on `DelistFilterEnabled`, and the user-facing toggle in the Safety tab controls that flag. If the operator disables delist filtering for any reason (debugging, manual override), the spot engine would still enforce it — creating an inconsistency.
- **Fix:** Wrapped both spot-engine checks in `if e.cfg.DelistFilterEnabled { ... }` to mirror the perp scanner. Single toggle controls the whole subsystem.
- **Files modified:** `internal/spotengine/risk_gate.go`, `internal/spotengine/monitor.go`
- **Commit:** `b548319`

### No architectural changes

No Rule 4 escalations were needed. The plan was clean and the implementation followed it directly.

## Known Stubs

None. The new code is fully wired end-to-end:
- `LoadAllContracts` populates `DeliveryDate` from real exchange responses.
- `refreshContractsAndFlagDelists` reads it and writes Redis with real TTLs.
- `IsDelisted` reads the same Redis key from both engines.
- `checkDelistPositions` and the spot monitor force-exit on real flag.
- All paths run on the live bot's first bot restart after deploy.

No placeholder values, no TODOs, no "coming soon" markers introduced.

## Threat Flags

None. This work removes attack surface (a delist-detection blind spot) rather than adding any. No new network endpoints, no new auth paths, no new file access patterns. The new poller calls the same `LoadAllContracts` REST endpoints the bot already calls at startup; the new code only writes to a Redis key prefix that already exists.

## Deferred items

See `deferred-items.md` in this directory for the pre-existing `stubExchange` test build failure that blocks `go test ./internal/discovery/...` (unrelated to this PR — added in v0.29.2 when `CancelAllOrders` was added to the Exchange interface).

## Self-Check: PASSED

Files verified to exist:
- FOUND: internal/discovery/contract_refresh.go
- FOUND: pkg/exchange/types.go (DeliveryDate field)
- FOUND: pkg/exchange/binance/adapter.go (extended unmarshal)
- FOUND: pkg/exchange/binance/adapter_test.go (TestLoadAllContracts_Binance_DeliveryDateParsing)
- FOUND: pkg/exchange/bybit/adapter.go (deliveryTime parsing)
- FOUND: internal/discovery/scanner.go (contractsMu)
- FOUND: internal/engine/engine.go (generalized checkDelistPositions)
- FOUND: internal/database/state.go (IsDelisted helper)
- FOUND: internal/spotengine/risk_gate.go (delist step 7)
- FOUND: internal/spotengine/monitor.go (delist scan)
- FOUND: internal/config/config.go (ContractRefreshInterval)
- FOUND: cmd/main.go (StartContractRefresh wiring)
- FOUND: VERSION (0.30.0)
- FOUND: CHANGELOG.md (0.30.0 entry)

Commits verified:
- FOUND: 6298fdc (Task 1)
- FOUND: 37b3d28 (Task 2)
- FOUND: 170aed2 (Task 3)
- FOUND: b548319 (Task 4)
- FOUND: 8f04a46 (Task 5)
