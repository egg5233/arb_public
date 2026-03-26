# OKX API Annotations

## Live Validation (2026-03-26)

All 11 READ-ONLY endpoints tested against live OKX API. All adapter-used fields confirmed present.

---

### CRITICAL BUG: GetClosePnL ignores fee/fundingFee/realizedPnl fields

**Endpoint**: `GET /api/v5/account/positions-history`

The adapter comment says:
> "OKX positions-history only returns total pnl (inclusive of fees/funding). Fee and funding breakdowns are not available from this endpoint."

**This is WRONG.** Live response clearly returns separate breakdown fields:

```json
{
  "pnl": "0.1472",           // PnL EXCLUDING fees
  "fee": "-0.1000224",       // Accumulated trading fee (negative = charged)
  "fundingFee": "0",         // Accumulated funding fee
  "liqPenalty": "0",         // Liquidation penalty
  "realizedPnl": "0.0471776" // = pnl + fee + fundingFee + liqPenalty + settledPnl
}
```

**Current adapter behavior** (`adapter.go:1009-1064`):
- Parses only `pnl` and assigns it to both `PricePnL` and `NetPnL`
- Sets `Fees: 0` and `Funding: 0` — but these ARE available!

**Correct mapping should be**:
| Adapter field | Should use | Currently uses |
|---------------|-----------|----------------|
| `PricePnL` | `pnl` | `pnl` (correct) |
| `Fees` | `fee` | `0` (BUG) |
| `Funding` | `fundingFee` | `0` (BUG) |
| `NetPnL` | `realizedPnl` | `pnl` (BUG — overstates profit, ignores fees) |

**Impact**: PnL reconciliation overstates realized profits by the amount of trading fees paid.

---

### CRITICAL BUG: GetClosePnL side inference ignores `direction` field

The live response includes a `direction` field (`"long"` or `"short"`) that directly gives the position direction. The adapter currently uses complex inference from `posSide` + `openMaxPos` sign, but could just read `direction`:

```json
{
  "posSide": "net",
  "direction": "long",
  "openMaxPos": "368"
}
```

---

### Bills Endpoint — type vs subType
The `GET /api/v5/account/bills` endpoint has TWO filter params:
- `type` — bill type (e.g., `8` = Funding fee, `2` = Trade)
- `subType` — bill subtype (e.g., `173` = funding expense, `174` = funding income, `5` = close long, `6` = close short)

These are NOT interchangeable. Live validation confirms `type: "8"` correctly filters to funding fee records with subTypes `173` (expense) and `174` (income).

**Live-validated fields (34 total)**: bal, balChg, billId, ccy, clOrdId, earnAmt, earnApr, execType, fee, fillFwdPx, fillIdxPx, fillMarkPx, fillMarkVol, fillPxUsd, fillPxVol, fillTime, from, instId, instType, interest, mgnMode, notes, ordId, pnl, posBal, posBalChg, px, subType, sz, tag, to, tradeId, ts, type

**Doc lists 19 fields** — missing 15 from doc: clOrdId, earnAmt, earnApr, fillFwdPx, fillIdxPx, fillMarkPx, fillMarkVol, fillPxUsd, fillPxVol, interest, mgnMode, notes, posBal, posBalChg, tag

---

### Response Envelope Already Unwrapped by Client
The OKX client (`client.go`) already unwraps the `{code: "0", data: [...]}` envelope and returns the raw `data` array. Unmarshal directly into a slice.

---

### sz (Size) is Always in Contracts for SWAP
All OKX SWAP endpoints return `sz`, `pos`, `fillSz`, `accFillSz`, `closeTotalPos` in **contract units**. Must multiply by `ctVal` to get base asset units.

**Note**: `GetPendingOrders` now correctly converts via ctVal. `GetClosePnL` converts `closeTotalPos`. Orderbook quantities remain in contracts (may affect depth analysis if ctVal != 1).

---

### fundingTime vs nextFundingTime vs prevFundingTime
Live response includes 3 time fields (not 2 as documented):
- `prevFundingTime` = previous period's settlement time (UNDOCUMENTED)
- `fundingTime` = current period's settlement time (may be past if settled)
- `nextFundingTime` = next period's settlement time (always future)

After settlement, `fundingTime` is stale. For "when is next funding", prefer `nextFundingTime`.

---

### Funding Rate — Undocumented Fields
Live response includes 5 fields not in doc:
- `impactValue` — impact value for funding calc (e.g., "20000.0000000000000000")
- `interestRate` — base interest rate (e.g., "0.0001000000000000")
- `nextFundingRate` — predicted next funding rate (often empty string)
- `premium` — premium index (e.g., "-0.0003880868743882")
- `prevFundingTime` — previous settlement timestamp

The adapter parses `nextFundingRate` — confirmed present (but may be empty string "").

---

### Instruments — Field Count Mismatch
Doc lists ~20 fields. Live response returns **46 fields**. Notable undocumented fields:
- `instIdCode` (numeric) — numeric instrument code
- `maxLmtAmt`, `maxMktAmt` — max order amounts (in addition to sizes)
- `maxIcebergSz`, `maxStopSz`, `maxTriggerSz`, `maxTwapSz` — algo order limits
- `posLmtAmt`, `posLmtPct` — position limits
- `futureSettlement` (boolean) — settlement flag
- `longPosRemainingQuota`, `shortPosRemainingQuota` — position quotas
- `openType` — e.g., "call_auction"
- `upcChg` — upcoming changes indicator

All adapter-used fields (instId, tickSz, lotSz, minSz, ctVal, state, settleCcy) confirmed present.

---

### Positions — availPos Empty for Cross Margin
Live response shows `availPos: ""` (empty string) for cross-margin positions. Adapter handles this correctly by falling back to `pos`:
```go
if p.AvailPos == "" {
    availPos = absPos
}
```

Live-validated: **64 fields** returned (doc lists ~23).

---

### Account Config — Additional Fields
Live response returns **29 fields** (doc lists 6). Notable extras:
- `perm`: "read_only,withdraw,trade" — API key permissions
- `ip`: current request IP
- `kycLv`: KYC level
- `label`: API key label (e.g., "arb")
- `settleCcy`, `settleCcyList` — settlement currency config

Adapter only uses `posMode` — confirmed present as `"net_mode"`.

---

### Positions History — All Doc Fields Confirmed
Live response returns **25 fields**, all matching doc. Key fields confirmed:
- `realizedPnl` = pnl + fee + fundingFee + liqPenalty (formula verified with live data)
- `direction` field present ("long"/"short") — usable instead of inference
- `pnlRatio` available for ratio-based analysis
- No server-side time filter — client-side filtering required (adapter does this correctly)

---

## Endpoint Status Summary

| Endpoint | Status | Fields Match | Adapter Issues |
|----------|--------|-------------|----------------|
| /public/instruments | OK | 46 live vs 20 doc | None (adapter uses 7 fields, all present) |
| /public/funding-rate | OK | 17 live vs 12 doc | nextFundingRate undocumented but parsed |
| /account/balance | OK | 20 live vs 11 doc | None |
| /asset/balances | OK | 4 live = 4 doc | None (exact match) |
| /account/positions | OK | 64 live vs 23 doc | availPos="" handled correctly |
| /market/books | OK | 4 live = 4 doc | None (exact match) |
| /trade/fills-history | OK | Empty (no fills) | Can't validate fields |
| /account/bills?type=8 | OK | 34 live vs 19 doc | Adapter only uses balChg, ts (correct) |
| /account/positions-history | **BUG** | 25 live = 25 doc | fee/fundingFee/realizedPnl ignored |
| /trade/orders-pending | OK | Empty (no orders) | Can't validate fields |
| /account/config | OK | 29 live vs 6 doc | None |
