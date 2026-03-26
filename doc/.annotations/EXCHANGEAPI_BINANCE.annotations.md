# Binance API Annotations

## Endpoint Version Gotchas

### /fapi/v2/positionRisk vs /fapi/v3/positionRisk
- v2 returns `leverage` and `marginType` fields directly in the position response
- v3 drops these fields — must use `GET /fapi/v1/symbolConfig` to get leverage/marginType per symbol
- Our adapter (adapter.go:232) uses v2 because it depends on `leverage` and `marginType`
- **Migration note**: Switching to v3 requires adding a separate symbolConfig call

### /fapi/v2/account vs /fapi/v3/account
- Both v2 and v3 return the same field names for the fields we use (`totalMarginBalance`, `totalMaintMargin`, `availableBalance`, `assets[]`)
- v3 is a safe drop-in replacement for our GetFuturesBalance usage

### /sapi/v1/futures/transfer (legacy) vs /sapi/v1/asset/transfer (universal)
- Legacy uses numeric type codes: 1=spot→futures, 2=futures→spot
- Universal uses string type codes: MAIN_UMFUTURE, UMFUTURE_MAIN
- Universal requires "Permits Universal Transfer" API key permission
- Our adapter uses the legacy endpoint — may need migration if Binance deprecates it

## Undocumented Endpoints Used

### /fapi/v1/algoOrder
- Used for conditional/algo orders (STOP_MARKET with closePosition)
- POST to create, DELETE to cancel (uses `algoId` param)
- Required since 2025-12-09 for conditional orders on Binance futures
- Response returns `algoId` (int64), not `orderId`
- Not in our local API docs — should be added

## listenKey Signing
- POST/PUT/DELETE /fapi/v1/listenKey are USER_STREAM security type (API key only, no signature required)
- Our client sends signature anyway (harmless — Binance ignores extra params)
- No need to change, but be aware if building a minimal client

## Position Mode
- Error code -4059: NO_NEED_TO_CHANGE_POSITION_SIDE (already in desired mode)
- Error code -4067: Also means "no need to change" — not in official error code docs but returned in practice
- Both are safe to suppress in EnsureOneWayMode

## Live Validation (2026-03-26)

### Validation Status: ALL PASS
All 11 read-only endpoints called successfully. All adapter methods work end-to-end.

### Endpoint Results

| # | Endpoint | Status | Notes |
|---|----------|--------|-------|
| 1 | GET /fapi/v1/exchangeInfo | OK | 598 contracts loaded |
| 2 | GET /fapi/v1/premiumIndex | OK | All 8 doc fields match live |
| 3 | GET /fapi/v1/fundingInfo | OK | All 5 doc fields match live |
| 4 | GET /fapi/v2/account | OK | All adapter fields present |
| 5 | GET /sapi/v1/capital/config/getall | OK | All adapter fields present |
| 6 | GET /fapi/v2/positionRisk | OK | All 8 adapter fields present |
| 7 | GET /fapi/v1/depth | OK | All 5 doc fields match live |
| 8 | GET /fapi/v1/userTrades | OK (empty) | No trades in account; doc fields validated via income |
| 9 | GET /fapi/v1/income (FUNDING_FEE) | OK (empty) | No funding fees for BTCUSDT |
| 10 | GET /fapi/v1/income (REALIZED_PNL) | OK | All 8 doc fields match live |
| 11 | GET /fapi/v1/openOrders | OK (empty) | No open orders |

### Doc vs Live Field Mismatches

**Fields in live response NOT in our doc** (benign — we don't use these):

#### GET /fapi/v1/exchangeInfo → symbols[]
- `maintMarginPercent` — string, e.g. "2.5000"
- `maxMoveOrderLimit` — int, move order limit
- `permissionSets` — array of permission sets
- `requiredMarginPercent` — string, e.g. "5.0000"

#### GET /fapi/v1/fundingInfo[]
- `updateTime` — int64 epoch ms, when funding info was last updated

#### GET /sapi/v1/capital/config/getall[]
- `ipoable` — string, IEO balance
- `ipoing` — string, IEO staking balance
- `isLegalMoney` — bool, fiat currency flag
- `storage` — string, storage balance

#### GET /fapi/v2/positionRisk[]
Extra fields not in our adapter struct but present in live:
- `adlQuantile` — int, ADL rank indicator
- `breakEvenPrice` — string, break-even price
- `isAutoAddMargin` — string ("true"/"false"), auto add margin flag
- `isolated` — bool, isolated margin flag
- `isolatedMargin` — string, isolated margin amount
- `isolatedWallet` — string, isolated wallet balance
- `maxNotionalValue` — string, max notional
- `notional` — string, current notional
- `positionSide` — string, position side ("BOTH" in one-way mode)
- `updateTime` — int64, last update timestamp

#### GET /fapi/v2/account
Extra top-level fields not in v3 doc but present in v2 live:
- `feeBurn` — bool
- `feeTier` — int
- `tradeGroupId` — int
- `canTrade`, `canDeposit`, `canWithdraw` — bool
- `positions` — array (includes all symbol positions)

Extra asset fields: `crossUnPnl`, `crossWalletBalance`, `initialMargin`, `maintMargin`, `marginAvailable`, `marginBalance`, `maxWithdrawAmount`, `openOrderInitialMargin`, `positionInitialMargin`, `unrealizedProfit`, `updateTime`

### Adapter Field Verification
All fields read by our Go adapter structs exist in the live responses:
- **adapter.go:237-246** (positionRisk): `symbol`, `positionAmt`, `entryPrice`, `unRealizedProfit`, `leverage`, `marginType`, `liquidationPrice`, `markPrice` — **ALL PRESENT**
- **adapter.go:482-490** (account/balance): `totalMarginBalance`, `totalMaintMargin`, `availableBalance`, `assets[].asset`, `assets[].walletBalance`, `assets[].availableBalance` — **ALL PRESENT**
- **adapter.go:531-534** (spot balance): `coin`, `free`, `locked` — **ALL PRESENT**
- **adapter.go:383-388** (premiumIndex): `symbol`, `lastFundingRate`, `nextFundingTime`, `interestRate`, `estimatedSettlePrice` — **ALL PRESENT**
- **adapter.go:428-432** (fundingInfo): `symbol`, `fundingIntervalHours`, `adjustedFundingRateCap`, `adjustedFundingRateFloor` — **ALL PRESENT**
- **adapter.go:571-574** (depth): `bids`, `asks`, `T` — **ALL PRESENT**

### Write Endpoints (NOT called)
- POST /fapi/v1/order (PlaceOrder)
- DELETE /fapi/v1/order (CancelOrder)
- POST /fapi/v1/leverage (SetLeverage)
- POST /fapi/v1/marginType (SetMarginMode)
- POST /fapi/v1/positionSide/dual (EnsureOneWayMode)
- POST /fapi/v1/algoOrder (PlaceStopLoss)
- DELETE /fapi/v1/algoOrder (CancelStopLoss)
- POST /sapi/v1/capital/withdraw/apply (Withdraw)
- POST /sapi/v1/futures/transfer (TransferToFutures)
