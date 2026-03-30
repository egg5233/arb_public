# Bybit API Annotations

## Gotchas & Implementation Notes

### POST Body Type Coercion
Bybit's v5 API documentation specifies boolean/integer types for fields like `reduceOnly`, `positionIdx`, `triggerDirection`, and `mode`. However, the API accepts string representations (e.g., `"true"` for boolean, `"0"` for integer). Our client uses `map[string]string` for all POST bodies, which serializes everything as JSON strings. This works in practice but is technically non-conformant.

### LoadAllContracts Pagination
`GET /v5/market/instruments-info` defaults to `limit=500`. Bybit currently has ~450 linear perpetual pairs. Our adapter does not implement cursor-based pagination — if the count exceeds 500, new instruments will be silently missing. Monitor and add pagination if needed.

### Funding Interval Unit
`fundingInterval` in instruments-info returns **minutes** (e.g., `480` = 8 hours), NOT hours. This is different from the ticker stream's `fundingIntervalHour` field which returns **hours** as a string (e.g., `"8"`). Be careful not to confuse these two fields.

### Unified Trading Account (UTA)
All Bybit accounts are now UTA. `TransferToFutures` is a no-op because the unified account already has trading access. Only `TransferToSpot` (UNIFIED -> FUND) is needed for withdrawals.

### Error Code 170213
Used alongside 110001 for idempotent cancel handling. This code is not in the official error code list but appears in practice for certain cancel scenarios.

### Orderbook Depth Level
Code comments reference "top-5" depth but the actual subscription is `orderbook.50` (50 levels). The subscription depth and comment are mismatched — the 50-level subscription is correct for the use case.

### switch-isolated vs switch-mode
- `POST /v5/position/switch-isolated` — switches margin mode (cross/isolated) for a symbol
- `POST /v5/position/switch-mode` — switches position mode (one-way/hedge)
These are two different endpoints. Don't confuse them.

### availableToWithdraw Fallback
In unified accounts, `availableToWithdraw` in wallet-balance may report **empty string `""`** (not `"0"`) even when funds are available for trading. ParseFloat on `""` returns 0, triggering the adapter's fallback to `equity - locked`. This is correct behavior.

---

## Live Validation Results (2026-03-26)

All 10 READ-ONLY endpoints were called with real API keys. Results below.

### Endpoint 1: GET /v5/market/instruments-info (LoadAllContracts)
**Status**: ✅ Working — all fields used by adapter are present
**Extra fields in response not in doc**:
- `copyTrading` — copy trading flag
- `deliveryFeeRate` — delivery fee rate (string, empty for perpetuals)
- `displayName` — human-readable symbol display name
- `forbidUplWithdrawal` — boolean, UPL withdrawal restriction flag
- `riskParameters` — object with additional risk limit parameters
- `symbolType` — symbol type classification
**Adapter impact**: None — adapter only reads `symbol`, `status`, `lotSizeFilter`, `priceFilter`, `fundingInterval`, `upperFundingRate`, `lowerFundingRate`. All present.

### Endpoint 2: GET /v5/market/tickers (GetFundingRate)
**Status**: ✅ Working — all fields used by adapter are present
**Extra fields in response not in doc**:
- `basis` — basis value
- `basisRate` — basis rate
- `basisRateYear` — annualized basis rate
- `curPreListingPhase` — pre-listing phase indicator
- `deliveryFeeRate` — delivery fee rate
- `deliveryTime` — delivery time
- `fundingCap` — funding rate cap (string, e.g., `"0.005"`)
- `fundingIntervalHour` — funding interval in **hours** as string (e.g., `"8"`)
- `preOpenPrice` — pre-open price
- `preQty` — pre-listing quantity
- `predictedDeliveryPrice` — predicted delivery price
**Adapter impact**: None — adapter only reads `symbol`, `fundingRate`, `nextFundingTime`. All present.
**Note**: `fundingCap` and `fundingIntervalHour` are available here but adapter fetches caps from instruments-info instead. Could simplify by reading from tickers.

### Endpoint 3: GET /v5/account/wallet-balance (GetFuturesBalance)
**Status**: ✅ Working — all fields used by adapter are present
**Extra fields at account level**:
- `accountIMRateByMp` — IM rate by mark price
- `accountLTV` — loan-to-value ratio
- `accountMMRateByMp` — MM rate by mark price
- `totalInitialMarginByMp` — total IM by mark price
- `totalMaintenanceMarginByMp` — total MM by mark price
**Extra fields at coin level**:
- `accruedInterest` — accrued interest for borrowing
- `availableToBorrow` — available borrowing amount
- `availableToWithdraw` — available to withdraw (returns `""` for USDT in unified account!)
- `spotHedgingQty` — spot hedging quantity
**Adapter impact**: `availableToWithdraw` returns empty string `""` for USDT, not `"0"`. ParseFloat(`""`) returns 0, correctly triggering the fallback to `equity - locked` at adapter.go:619-621. This is working as designed.

### Endpoint 4: GET /v5/position/list (GetPosition)
**Status**: ✅ Working — all fields used by adapter are present
**Extra fields in response not in doc**:
- `adlRankIndicator` — ADL rank indicator (integer)
- `autoAddMargin` — auto add margin flag (integer)
- `breakEvenPrice` — break-even price
- `bustPrice` — bankruptcy price
- `leverageSysUpdatedTime` — leverage system update timestamp
- `mmrSysUpdatedTime` — MMR system update timestamp
- `positionBalance` — position balance
- `positionIM` — initial margin for position
- `positionIMByMp` — IM by mark price
- `positionMM` — maintenance margin for position
- `positionMMByMp` — MM by mark price
- `positionStatus` — position status (e.g., `"Normal"`)
- `sessionAvgPrice` — session average price
- `tpslMode` — TP/SL mode
- `tradeMode` — trade mode (0=cross, 1=isolated) — **used by adapter for margin mode detection**
- `trailingStop` — trailing stop value
**Adapter impact**: `tradeMode` field is critical for margin mode detection (adapter.go:314,338-339) and IS present in the response. Doc should document this field.

### Endpoint 5: GET /v5/market/orderbook (GetOrderbook)
**Status**: ✅ All fields match doc
**Fields**: `s`, `a`, `b`, `ts`, `u`, `seq`, `cts`

### Endpoint 6: GET /v5/execution/list (GetUserTrades)
**Status**: ✅ Working — all fields used by adapter are present
**Extra fields in response not in doc**:
- `blockTradeId` — block trade identifier
- `createType` — order creation type (e.g., `"CreateByUser"`)
- `execFeeV2` — execution fee v2 (negative = maker rebate)
- `extraFees` — extra fees info
- `indexPrice` — index price at execution time
- `markIv` — mark implied volatility (options)
- `marketUnit` — market unit type
- `stopOrderType` — stop order type (e.g., `"UNKNOWN"`)
- `tradeIv` — trade implied volatility (options)
- `underlyingPrice` — underlying price (options)
**Adapter impact**: None — adapter reads `execId`, `orderId`, `symbol`, `side`, `execPrice`, `execQty`, `execFee`, `feeCurrency`, `execTime`. All present.

### Endpoint 7: GET /v5/account/transaction-log (GetFundingFees)
**Status**: ✅ All fields match doc
**Note**: Adapter uses `type: "SETTLEMENT"` (string), NOT numeric type `24`. The doc correctly shows string type values.
**Fields confirmed**: `funding`, `transactionTime`, `symbol`, `side`, `feeRate`, etc.

### Endpoint 8: GET /v5/position/closed-pnl (GetClosePnL)
**Status**: ✅ Working — all fields used by adapter are present
**Extra fields in response not in doc**:
- `cumEntryValue` — cumulative entry value — **used by adapter for pricePnL calculation**
- `cumExitValue` — cumulative exit value — **used by adapter for pricePnL calculation**
**Adapter impact**: Both `cumEntryValue` and `cumExitValue` are read by the adapter (adapter.go:978-979,1007-1008) and ARE present in the real response. Doc should add these fields.

### Endpoint 9: GET /v5/order/realtime (GetPendingOrders)
**Status**: ✅ Working (empty list — no open orders)
**Note**: Requires at least one of `symbol`, `settleCoin`, or `baseCoin`. Adapter always passes `symbol`, which is correct. My initial test failed because it passed only `category` — this is a parameter requirement not clearly stated in the doc.
**Doc correction needed**: The doc says symbol is optional for linear, but in practice `symbol OR settleCoin OR baseCoin` is **mandatory** (API returns error code 10001 otherwise).

### Endpoint 10: GET /v5/user/query-api (CheckPermissions)
**Status**: ✅ All fields match doc
**Key observations from real response**:
- `readOnly: 0` — read/write key
- `permissions.ContractTrade: ["Order", "Position"]` — futures trading enabled
- `permissions.Wallet: ["AccountTransfer", "Withdraw"]` — transfer and withdraw enabled
- `ips: ["114.32.59.106"]` — IP-restricted (good security)

---

## Doc Corrections Needed

### 1. closed-pnl: Missing `cumEntryValue` and `cumExitValue`
The doc lists response fields but omits `cumEntryValue` and `cumExitValue`. These are present in real responses and used by our adapter for price PnL calculation. Add to doc response fields table.

### 2. order/realtime: Parameter requirement clarification
Doc says `symbol` is optional, but API returns error 10001 if none of `symbol`, `settleCoin`, or `baseCoin` is provided. Should document: "For linear: at least one of symbol, baseCoin, or settleCoin is required."

### 3. position/list: Missing `tradeMode` field
Doc doesn't document `tradeMode` (0=cross, 1=isolated). Our adapter uses this field for margin mode detection. Should be added to doc.

### 4. wallet-balance coin level: `availableToWithdraw` returns empty string
Doc shows `availableToWithdraw` but doesn't mention it returns `""` (empty string) instead of `"0"` in some unified account configurations. This is a known Bybit behavior.

### 5. instruments-info: Multiple undocumented fields
6 fields present in real response but not in doc: `copyTrading`, `deliveryFeeRate`, `displayName`, `forbidUplWithdrawal`, `riskParameters`, `symbolType`. None affect our adapter but should be documented for completeness.

### 6. tickers: `fundingCap` and `fundingIntervalHour` available
These fields are available in the tickers response and could be used instead of making a separate instruments-info call. Currently adapter makes an extra API call to instruments-info for rate caps — could be optimized.

### 7. crypto-loan-common/loanable-data: Undocumented in main Bybit API docs site
`GET /v5/crypto-loan-common/loanable-data` is a public endpoint (no auth) used by the spot-futures borrow-rate adapter. It returns flexible/fixed loan parameters per coin. This endpoint is from Bybit's Crypto Loan product API, separate from the UTA spot margin endpoints. The response fields documented in `EXCHANGEAPI_BYBIT_MARGIN.md` are based on code usage — the full response may contain additional fields not yet documented. Needs live-validation scrape to confirm complete schema.
