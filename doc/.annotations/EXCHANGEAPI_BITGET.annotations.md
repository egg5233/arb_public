# Bitget API Annotations

## Live Validation (2026-03-26)

All 9 READ-ONLY endpoints called successfully against live Bitget API. Results compared against `doc/EXCHANGEAPI_BITGET.md` and `pkg/exchange/bitget/adapter.go`.

---

### BUG: GetFuturesBalance — `frozen` vs `locked`
- **Severity**: Medium (currently benign, becomes critical when funds are locked)
- **Location**: `adapter.go:566` — `Frozen string \`json:"frozen"\``
- **Live API returns**: `"locked": "0"` — there is NO `frozen` field in the response
- **Impact**: `Frozen` always parses as 0/empty, even when funds are actually locked
- **Fix**: Change JSON tag from `json:"frozen"` to `json:"locked"`
- **Doc status**: Doc at line 369 correctly shows `"locked": "0"` — adapter is wrong

### CONFIRMED: Position field `openPriceAvg`
- **Live API returns**: `"openPriceAvg": "2.2628375"` ✅
- **Adapter** (`adapter.go:261`): `AverageOpenPrice string \`json:"openPriceAvg"\`` ✅ Correct
- Previous annotation concern was unfounded — the JSON tag IS `openPriceAvg`, field naming is just Go convention

### GetFundingRate — Missing fields in live response
- **Adapter expects** (`adapter.go:456-457`): `nextFundRate`, `fundingTime`
- **Live API returns**: Only `symbol`, `fundingRate`, `fundingRateInterval`, `nextUpdate`, `minFundingRate`, `maxFundingRate`
- **`nextFundRate`**: Not returned → NextRate always 0. Low impact (not used for decisions).
- **`fundingTime`**: Not returned → Adapter falls back to `nextUpdate` which IS present. No impact.
- **Doc status**: Doc correctly lists only the fields that appear in live response

### Orderbook entries — numbers not strings
- **Doc shows**: `"asks": [["26347.5", "0.25"], ...]` (string arrays)
- **Live returns**: `"asks": [[70041.7, 2.5366], ...]` (number arrays)
- **Impact**: None — adapter uses `json.RawMessage` + `parseRawFloat` which handles both
- **Doc needs update**: Response example should show numbers, not strings

### GetUserTrades — `tradeSide` values in one_way_mode
- **Doc says**: `tradeSide` is `"open"` or `"close"` (hedge mode)
- **Live returns**: `"tradeSide": "buy_single"` (one_way_mode)
- **Possible values in one_way_mode**: `buy_single`, `sell_single`
- **Impact**: Adapter doesn't use `tradeSide`, so no code impact
- **`totalDeductionFee`**: Can be `null` (not always a string). Adapter doesn't parse it, so no impact.

### GetFundingFees (account/bill) — Additional fields
- **Live response fields**: `billId`, `symbol`, `amount`, `fee`, `feeByCoupon`, `businessType`, `coin`, `balance`, `cTime`
- **Adapter parses**: `amount`, `cTime` only — sufficient for current use
- **`symbol` field available**: Could be used to filter by symbol if needed

### GetClosePnL (history-position) — Field names confirmed
- All adapter JSON tags match live response ✅
- `utime` (lowercase) confirmed — `adapter.go:1042`
- Additional live fields not in adapter: `positionId`, `marginCoin`, `marginMode`, `openTotalPos`, `posMode`, `ctime`

### GetPendingOrders — null vs empty array
- **When no orders**: `"entrustedList": null` (not `[]`)
- **Impact**: Go handles `null` → nil slice, `len(nil) == 0`. No issue.
- **`endId`**: Also `null` when empty

### LoadAllContracts — `fundInterval` confirmed
- **Live API returns**: `"fundInterval": "8"` (string) ✅
- **Adapter** (`adapter.go:529`): `FundingInterval string \`json:"fundInterval"\`` ✅
- **Additional fields not in adapter/doc**: `buyLimitPriceRatio`, `sellLimitPriceRatio`, `feeRateUpRatio`, `openCostUpRatio`, `maxSymbolOrderNum`, `maxProductOrderNum`, `maxPositionNum`, `offTime`, `limitOpenTime`, `deliveryTime`, `deliveryStartTime`, `deliveryPeriod`, `launchTime`, `posLimit`, `maintainTime`, `openTime`, `isRwa`
- **Doc example inaccuracy**: Doc shows `"volumePlace": "2"` for BTCUSDT, live returns `"4"`. This is per-symbol, not a bug.
- **`maxLever`**: Doc says `"125"`, live returns `"150"` for BTCUSDT. Also per-symbol/time dependent.

### GetFuturesBalance — Additional account fields
- **Live but not in adapter**: `grant`, `isolatedMargin` (null), `crossedMargin` (null)
- **`crossedMarginLeverage`**: Returns as number (float64), not string — unusual for Bitget
- **`isolatedLongLever`/`isolatedShortLever`**: Also numbers, not strings

### reduceOnly Case Sensitivity
- API expects uppercase: `"YES"` or `"NO"`
- Code at `adapter.go:94` sends `"YES"` ✅ (previous annotation was incorrect)

### Account Bill businessType Values
- Funding fees: `contract_settle_fee` ✅ confirmed in live data
- Other values: `open_long`, `open_short`, `close_long`, `close_short`

### productType Case
- Canonical value is uppercase: `USDT-FUTURES` ✅ confirmed working

---

## Action Items

| Priority | Issue | File | Line | Fix |
|----------|-------|------|------|-----|
| **Medium** | `frozen` → `locked` in balance | adapter.go | 566 | Change `json:"frozen"` to `json:"locked"` |
| **Low** | Orderbook doc shows strings, API returns numbers | EXCHANGEAPI_BITGET.md | 248 | Update example |
| **Info** | `nextFundRate` field absent from API | adapter.go | 456 | No fix needed, graceful fallback |
| **Info** | `fundingTime` field absent from API | adapter.go | 457 | No fix needed, `nextUpdate` used |
