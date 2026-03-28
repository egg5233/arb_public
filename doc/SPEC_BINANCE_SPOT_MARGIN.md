# Binance Spot Margin Borrow-and-Sell — API Specification

## Purpose

Synthetic short via spot margin: borrow an asset on cross margin, sell it on spot, later buy it back, repay the loan. This creates a delta-neutral short leg as an alternative to futures shorting, useful when futures funding rates are unfavorable or when spot margin borrowing rates are cheaper.

## Base URLs

| Scope | URL |
|-------|-----|
| Spot / Margin / Wallet (`/sapi/`, `/api/`) | `https://api.binance.com` |
| USDS-M Futures (`/fapi/`) | `https://fapi.binance.com` |

All margin endpoints use `https://api.binance.com` — the same base URL as the existing `SpotPost()` and `SpotGet()` methods in `client.go`.

## Authentication

Same as futures: HMAC-SHA256 signing. Every signed request needs:
- `timestamp` (ms) in params
- `signature` = HMAC-SHA256(secretKey, sorted query string)
- Header: `X-MBX-APIKEY: <apiKey>`

The existing `client.buildQuery()` and `SpotPost()`/`SpotGet()` methods handle this.

## API Key Permissions Required

The API key must have these permissions enabled (visible at `GET /sapi/v1/account/apiRestrictions`):

| Permission Field | Required | Purpose |
|-----------------|----------|---------|
| `enableReading` | YES | Query account, balances, orders |
| `enableSpotAndMarginTrading` | YES | Place margin orders |
| `enableMargin` | YES | Borrow/repay operations |
| `permitsUniversalTransfer` | YES | Transfer between futures and margin accounts |

Check at startup alongside existing futures permission checks.

## Cross Margin vs Isolated Margin

**Recommendation: Cross Margin** for this use case.

| Factor | Cross Margin | Isolated Margin |
|--------|-------------|-----------------|
| Collateral | Shared across all pairs | Per-pair isolated |
| Setup | One account, works for all pairs | Must enable per symbol pair |
| Leverage | 3x or 5x (Classic), 10x (Pro) | Up to 10x per pair |
| Liquidation | Shared — one big pool absorbs losses | Per-pair — one pair can liquidate alone |
| Complexity | Simpler — single account | More complex — manage per pair |
| Best for | Multi-pair arb bot | Single concentrated position |

Cross margin is simpler, supports all pairs from one account, and the shared collateral pool is fine because positions are delta-neutral (the long futures leg hedges the short margin leg).

---

## Complete Operation Flow

```
┌─────────────────────────────────────────────────────────────┐
│  ENTRY: Create synthetic short                               │
│                                                              │
│  1. Transfer USDT: Futures → Cross Margin                    │
│     POST /sapi/v1/asset/transfer  type=UMFUTURE_MARGIN      │
│                                                              │
│  2. Check max borrowable amount                              │
│     GET /sapi/v1/margin/maxBorrowable                        │
│                                                              │
│  3. Borrow asset (e.g., BTC)                                 │
│     POST /sapi/v1/margin/borrow-repay  type=BORROW           │
│                                                              │
│  4. Sell borrowed asset for USDT                             │
│     POST /sapi/v1/margin/order  side=SELL                    │
│                                                              │
│  (Alternative to steps 3+4: single order with                │
│   sideEffectType=AUTO_BORROW_REPAY, side=SELL)               │
│                                                              │
├─────────────────────────────────────────────────────────────┤
│  HOLD: Monitor position                                      │
│                                                              │
│  5. Query margin account details                             │
│     GET /sapi/v1/margin/account                              │
│                                                              │
│  6. Check hourly interest rate                               │
│     GET /sapi/v1/margin/next-hourly-interest-rate            │
│                                                              │
│  7. Query outstanding loans                                  │
│     GET /sapi/v1/margin/borrow-repay  type=BORROW            │
│                                                              │
├─────────────────────────────────────────────────────────────┤
│  EXIT: Close synthetic short                                 │
│                                                              │
│  8. Buy back asset                                           │
│     POST /sapi/v1/margin/order  side=BUY                     │
│                                                              │
│  9. Repay loan                                               │
│     POST /sapi/v1/margin/borrow-repay  type=REPAY            │
│                                                              │
│  (Alternative to steps 8+9: single order with                │
│   sideEffectType=AUTO_REPAY, side=BUY)                       │
│                                                              │
│  10. Transfer USDT profit: Cross Margin → Futures            │
│      POST /sapi/v1/asset/transfer  type=MARGIN_UMFUTURE     │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

---

## Endpoint Reference

### 1. Universal Transfer (Futures <-> Margin)

Move USDT between futures and cross margin accounts.

**Endpoint**: `POST /sapi/v1/asset/transfer`
**Weight**: 900 (UID)
**Security**: USER_DATA (signed)
**Base URL**: `https://api.binance.com`
**Prerequisite**: API key must have "Permits Universal Transfer" enabled.

#### Parameters

| Name | Type | Required | Description |
|------|------|----------|-------------|
| type | ENUM | YES | Transfer type code (see below) |
| asset | STRING | YES | e.g., "USDT" |
| amount | DECIMAL | YES | Amount to transfer |
| recvWindow | LONG | NO | Max 60000 |
| timestamp | LONG | YES | |

#### Transfer Type Codes (relevant subset)

| Code | Direction |
|------|-----------|
| `UMFUTURE_MARGIN` | USDS-M Futures -> Cross Margin |
| `MARGIN_UMFUTURE` | Cross Margin -> USDS-M Futures |
| `UMFUTURE_MAIN` | USDS-M Futures -> Spot |
| `MAIN_MARGIN` | Spot -> Cross Margin |
| `MARGIN_MAIN` | Cross Margin -> Spot |
| `MAIN_UMFUTURE` | Spot -> USDS-M Futures |

**Direct `UMFUTURE_MARGIN` is supported** — no need to hop through spot.

#### Response
```json
{"tranId": 13526853623}
```

#### Go Implementation Pattern
```go
func (b *Adapter) TransferFuturesToMargin(asset string, amount string) error {
    params := map[string]string{
        "type":   "UMFUTURE_MARGIN",
        "asset":  asset,
        "amount": amount,
    }
    _, err := b.client.SpotPost("/sapi/v1/asset/transfer", params)
    return err
}

func (b *Adapter) TransferMarginToFutures(asset string, amount string) error {
    params := map[string]string{
        "type":   "MARGIN_UMFUTURE",
        "asset":  asset,
        "amount": amount,
    }
    _, err := b.client.SpotPost("/sapi/v1/asset/transfer", params)
    return err
}
```

---

### 2. Query Max Borrowable

Check how much of an asset can be borrowed before attempting.

**Endpoint**: `GET /sapi/v1/margin/maxBorrowable`
**Weight**: 50 (IP)
**Security**: USER_DATA (signed)
**Base URL**: `https://api.binance.com`

#### Parameters

| Name | Type | Required | Description |
|------|------|----------|-------------|
| asset | STRING | YES | e.g., "BTC", "ETH" |
| isolatedSymbol | STRING | NO | Omit for cross margin |
| recvWindow | LONG | NO | Max 60000 |
| timestamp | LONG | YES | |

#### Response
```json
{
    "amount": "1.69248805",
    "borrowLimit": "60"
}
```

| Field | Description |
|-------|-------------|
| `amount` | Max borrowable right now (considers collateral + system availability) |
| `borrowLimit` | Account-tier max borrow limit for this asset |

#### Go Implementation Pattern
```go
type MaxBorrowable struct {
    Amount      float64
    BorrowLimit float64
}

func (b *Adapter) GetMaxBorrowable(asset string) (*MaxBorrowable, error) {
    params := map[string]string{"asset": asset}
    body, err := b.client.SpotGet("/sapi/v1/margin/maxBorrowable", params)
    if err != nil {
        return nil, fmt.Errorf("GetMaxBorrowable: %w", err)
    }
    var resp struct {
        Amount      string `json:"amount"`
        BorrowLimit string `json:"borrowLimit"`
    }
    if err := json.Unmarshal(body, &resp); err != nil {
        return nil, err
    }
    amt, _ := strconv.ParseFloat(resp.Amount, 64)
    limit, _ := strconv.ParseFloat(resp.BorrowLimit, 64)
    return &MaxBorrowable{Amount: amt, BorrowLimit: limit}, nil
}
```

---

### 3. Borrow Asset

Borrow an asset on cross margin. Must have sufficient USDT collateral already in the margin account.

**Endpoint**: `POST /sapi/v1/margin/borrow-repay`
**Weight**: 1500
**Security**: USER_DATA (signed)
**Base URL**: `https://api.binance.com`

#### Parameters

| Name | Type | Required | Description |
|------|------|----------|-------------|
| asset | STRING | YES | Asset to borrow, e.g., "BTC" |
| isIsolated | STRING | YES | `"FALSE"` for cross margin |
| symbol | STRING | Conditional | Required only for isolated margin; omit for cross |
| amount | STRING | YES | Amount to borrow |
| type | STRING | YES | `"BORROW"` |
| recvWindow | LONG | NO | Max 60000 |
| timestamp | LONG | YES | |

#### Response
```json
{"tranId": 100000001}
```

#### Important Notes
- Requests must be spaced at least **100ms apart** (sequential processing).
- Will fail if: amount exceeds `maxBorrowable`, asset is not borrowable, insufficient collateral.
- The `tranId` can be used to query the borrow status via the GET endpoint.

#### Go Implementation Pattern
```go
func (b *Adapter) MarginBorrow(asset string, amount string) (int64, error) {
    params := map[string]string{
        "asset":      asset,
        "isIsolated": "FALSE",
        "amount":     amount,
        "type":       "BORROW",
    }
    body, err := b.client.SpotPost("/sapi/v1/margin/borrow-repay", params)
    if err != nil {
        return 0, fmt.Errorf("MarginBorrow: %w", err)
    }
    var resp struct {
        TranID int64 `json:"tranId"`
    }
    if err := json.Unmarshal(body, &resp); err != nil {
        return 0, err
    }
    return resp.TranID, nil
}
```

---

### 4. Sell Borrowed Asset (Margin Order)

Place a sell order on the margin account. This sells the borrowed asset for USDT.

**Endpoint**: `POST /sapi/v1/margin/order`
**Weight**: 6 (UID)
**Security**: TRADE (signed)
**Base URL**: `https://api.binance.com`

#### Parameters

| Name | Type | Required | Description |
|------|------|----------|-------------|
| symbol | STRING | YES | e.g., "BTCUSDT" |
| isIsolated | STRING | NO | `"FALSE"` for cross margin (default) |
| side | ENUM | YES | `"SELL"` to sell borrowed asset |
| type | ENUM | YES | `"MARKET"` or `"LIMIT"` |
| quantity | DECIMAL | YES* | Required for MARKET sell, LIMIT |
| quoteOrderQty | DECIMAL | NO | Alternative: specify USDT amount for MARKET |
| price | DECIMAL | YES* | Required for LIMIT orders |
| timeInForce | ENUM | NO | `"GTC"`, `"IOC"`, `"FOK"` (required for LIMIT) |
| sideEffectType | ENUM | NO | See below |
| autoRepayAtCancel | BOOLEAN | NO | Auto-repay on cancel; default true |
| newClientOrderId | STRING | NO | Custom order ID |
| newOrderRespType | ENUM | NO | `"ACK"`, `"RESULT"`, `"FULL"` |
| recvWindow | LONG | NO | Max 60000 |
| timestamp | LONG | YES | |

#### sideEffectType Values

| Value | Behavior |
|-------|----------|
| `NO_SIDE_EFFECT` | Default. Normal order, no auto-borrow/repay. |
| `MARGIN_BUY` | Auto-borrow if insufficient balance (for BUY orders). |
| `AUTO_REPAY` | Auto-repay liability after order fills (for BUY orders buying back). |
| `AUTO_BORROW_REPAY` | **Combined**: auto-borrow on SELL, auto-repay on BUY. **This is the simplest approach.** |

**Key insight**: Using `sideEffectType=AUTO_BORROW_REPAY` with `side=SELL` combines steps 3+4 into a single API call. It borrows the asset and immediately sells it. Similarly, `sideEffectType=AUTO_BORROW_REPAY` with `side=BUY` buys back and auto-repays.

#### Response (FULL type)
```json
{
    "symbol": "BTCUSDT",
    "orderId": 26769564559,
    "clientOrderId": "E156O3KP4gOif65bjuUK5V",
    "transactTime": 1713873075893,
    "price": "0.00000000",
    "origQty": "0.001",
    "executedQty": "0.001",
    "cummulativeQuoteQty": "65.98253000",
    "status": "FILLED",
    "timeInForce": "GTC",
    "type": "MARKET",
    "side": "SELL",
    "fills": [
        {
            "price": "65982.53",
            "qty": "0.001",
            "commission": "0.06598253",
            "commissionAsset": "USDT"
        }
    ],
    "marginBuyBorrowAmount": 0.001,
    "marginBuyBorrowAsset": "BTC"
}
```

The `marginBuyBorrowAmount` and `marginBuyBorrowAsset` fields appear only when auto-borrow occurs.

#### Go Implementation Pattern
```go
// MarginSell places a market sell on cross margin (with auto-borrow).
// Returns the order ID and actual filled quantity.
func (b *Adapter) MarginSell(symbol, asset string, qty string) (string, float64, error) {
    params := map[string]string{
        "symbol":           symbol,
        "isIsolated":       "FALSE",
        "side":             "SELL",
        "type":             "MARKET",
        "quantity":         qty,
        "sideEffectType":   "AUTO_BORROW_REPAY",
        "newOrderRespType": "FULL",
    }
    body, err := b.client.SpotPost("/sapi/v1/margin/order", params)
    if err != nil {
        return "", 0, fmt.Errorf("MarginSell: %w", err)
    }
    var resp struct {
        OrderID     int64  `json:"orderId"`
        ExecutedQty string `json:"executedQty"`
        Status      string `json:"status"`
    }
    if err := json.Unmarshal(body, &resp); err != nil {
        return "", 0, err
    }
    filled, _ := strconv.ParseFloat(resp.ExecutedQty, 64)
    return strconv.FormatInt(resp.OrderID, 10), filled, nil
}
```

---

### 5. Query Next Hourly Interest Rate

Get the upcoming hourly interest rate for borrowed assets. Use to calculate ongoing cost.

**Endpoint**: `GET /sapi/v1/margin/next-hourly-interest-rate`
**Weight**: 100 (IP)
**Security**: USER_DATA (signed)
**Base URL**: `https://api.binance.com`

#### Parameters

| Name | Type | Required | Description |
|------|------|----------|-------------|
| assets | STRING | YES | Comma-separated, up to 20. e.g., "BTC,ETH,SOL" |
| isIsolated | BOOLEAN | YES | `false` for cross margin |

#### Response
```json
[
    {
        "asset": "BTC",
        "nextHourlyInterestRate": "0.00000571"
    },
    {
        "asset": "ETH",
        "nextHourlyInterestRate": "0.00000578"
    }
]
```

The rate is **per hour** as a decimal. To get annual rate: `rate * 24 * 365`.
Example: `0.00000571 * 24 * 365 = 0.050 = 5.0% annual`.

#### Go Implementation Pattern
```go
type HourlyInterestRate struct {
    Asset  string
    Rate   float64 // per-hour decimal
}

func (b *Adapter) GetNextHourlyInterestRate(assets []string) ([]HourlyInterestRate, error) {
    params := map[string]string{
        "assets":     strings.Join(assets, ","),
        "isIsolated": "false",
    }
    body, err := b.client.SpotGet("/sapi/v1/margin/next-hourly-interest-rate", params)
    if err != nil {
        return nil, fmt.Errorf("GetNextHourlyInterestRate: %w", err)
    }
    var resp []struct {
        Asset                  string `json:"asset"`
        NextHourlyInterestRate string `json:"nextHourlyInterestRate"`
    }
    if err := json.Unmarshal(body, &resp); err != nil {
        return nil, err
    }
    out := make([]HourlyInterestRate, len(resp))
    for i, r := range resp {
        rate, _ := strconv.ParseFloat(r.NextHourlyInterestRate, 64)
        out[i] = HourlyInterestRate{Asset: r.Asset, Rate: rate}
    }
    return out, nil
}
```

---

### 6. Query Cross Margin Account Details

Get full account state: balances, borrowed amounts, interest, margin level.

**Endpoint**: `GET /sapi/v1/margin/account`
**Weight**: 10 (IP)
**Security**: USER_DATA (signed)
**Base URL**: `https://api.binance.com`

#### Parameters

| Name | Type | Required | Description |
|------|------|----------|-------------|
| recvWindow | LONG | NO | Max 60000 |
| timestamp | LONG | YES | |

#### Response
```json
{
    "created": true,
    "borrowEnabled": true,
    "marginLevel": "11.64405625",
    "collateralMarginLevel": "3.2",
    "totalAssetOfBtc": "6.82728457",
    "totalLiabilityOfBtc": "0.58633215",
    "totalNetAssetOfBtc": "6.24095242",
    "TotalCollateralValueInUSDT": "5.82728457",
    "totalOpenOrderLossInUSDT": "582.728457",
    "tradeEnabled": true,
    "transferInEnabled": true,
    "transferOutEnabled": true,
    "accountType": "MARGIN_1",
    "userAssets": [
        {
            "asset": "BTC",
            "borrowed": "0.00000000",
            "free": "0.00499500",
            "interest": "0.00000000",
            "locked": "0.00000000",
            "netAsset": "0.00499500"
        },
        {
            "asset": "USDT",
            "borrowed": "0.00000000",
            "free": "1000.00000000",
            "interest": "0.00000000",
            "locked": "0.00000000",
            "netAsset": "1000.00000000"
        }
    ]
}
```

#### Key Fields

| Field | Description |
|-------|-------------|
| `created` | Whether margin account exists |
| `borrowEnabled` | Whether borrowing is currently allowed |
| `marginLevel` | Current margin level (equity / liability). Liquidation at 1.1 for 3x, 1.05 for 5x |
| `accountType` | `MARGIN_1` = Cross Margin Classic, `MARGIN_2` = Cross Margin Pro |
| `userAssets[].borrowed` | Outstanding borrowed amount per asset |
| `userAssets[].interest` | Accrued unpaid interest per asset |
| `userAssets[].free` | Available balance (can be sold or transferred out) |
| `userAssets[].netAsset` | free + locked - borrowed - interest |

**This is the primary endpoint for checking outstanding loans.** Look at `userAssets` where `borrowed > 0`.

#### Go Implementation Pattern
```go
type MarginAccountAsset struct {
    Asset    string
    Free     float64
    Locked   float64
    Borrowed float64
    Interest float64
    NetAsset float64
}

type MarginAccountInfo struct {
    Created       bool
    BorrowEnabled bool
    MarginLevel   float64
    Assets        []MarginAccountAsset
}

func (b *Adapter) GetMarginAccount() (*MarginAccountInfo, error) {
    body, err := b.client.SpotGet("/sapi/v1/margin/account", nil)
    if err != nil {
        return nil, fmt.Errorf("GetMarginAccount: %w", err)
    }
    var resp struct {
        Created       bool   `json:"created"`
        BorrowEnabled bool   `json:"borrowEnabled"`
        MarginLevel   string `json:"marginLevel"`
        UserAssets    []struct {
            Asset    string `json:"asset"`
            Borrowed string `json:"borrowed"`
            Free     string `json:"free"`
            Interest string `json:"interest"`
            Locked   string `json:"locked"`
            NetAsset string `json:"netAsset"`
        } `json:"userAssets"`
    }
    if err := json.Unmarshal(body, &resp); err != nil {
        return nil, err
    }
    info := &MarginAccountInfo{
        Created:       resp.Created,
        BorrowEnabled: resp.BorrowEnabled,
    }
    info.MarginLevel, _ = strconv.ParseFloat(resp.MarginLevel, 64)
    for _, a := range resp.UserAssets {
        borrowed, _ := strconv.ParseFloat(a.Borrowed, 64)
        interest, _ := strconv.ParseFloat(a.Interest, 64)
        if borrowed > 0 || interest > 0 || a.Asset == "USDT" {
            free, _ := strconv.ParseFloat(a.Free, 64)
            locked, _ := strconv.ParseFloat(a.Locked, 64)
            netAsset, _ := strconv.ParseFloat(a.NetAsset, 64)
            info.Assets = append(info.Assets, MarginAccountAsset{
                Asset: a.Asset, Free: free, Locked: locked,
                Borrowed: borrowed, Interest: interest, NetAsset: netAsset,
            })
        }
    }
    return info, nil
}
```

---

### 7. Query Borrow/Repay Records

Check status and history of borrow/repay transactions.

**Endpoint**: `GET /sapi/v1/margin/borrow-repay`
**Weight**: 10 (IP)
**Security**: USER_DATA (signed)
**Base URL**: `https://api.binance.com`

#### Parameters

| Name | Type | Required | Description |
|------|------|----------|-------------|
| type | STRING | YES | `"BORROW"` or `"REPAY"` |
| asset | STRING | NO | Filter by asset |
| isolatedSymbol | STRING | NO | Omit for cross margin |
| txId | LONG | NO | Specific transaction ID |
| startTime | LONG | NO | |
| endTime | LONG | NO | |
| current | LONG | NO | Page number, starts at 1 |
| size | LONG | NO | Default 10, max 100 |
| recvWindow | LONG | NO | Max 60000 |
| timestamp | LONG | YES | |

**Note**: Either `txId` or `startTime` must be provided. Without asset, returns last 7 days. With asset, 30 days before endTime.

#### Response
```json
{
    "rows": [
        {
            "type": "BORROW",
            "isolatedSymbol": "",
            "amount": "0.10000000",
            "asset": "BTC",
            "interest": "0.00000833",
            "principal": "0.09999167",
            "status": "CONFIRMED",
            "timestamp": 1563438204000,
            "txId": 2970933056
        }
    ],
    "total": 1
}
```

| Field | Description |
|-------|-------------|
| `status` | `PENDING`, `CONFIRMED`, or `FAILED` |
| `interest` | Interest charged at time of borrow (for type=BORROW) |
| `principal` | Net principal amount |

---

### 8. Buy Back Asset (Close Short)

Buy back the borrowed asset to prepare for repayment.

Uses the same `POST /sapi/v1/margin/order` endpoint as step 4, with `side=BUY`.

#### Parameters (key differences from sell)

| Name | Value | Notes |
|------|-------|-------|
| side | `"BUY"` | Buying back the asset |
| sideEffectType | `"AUTO_REPAY"` or `"AUTO_BORROW_REPAY"` | Auto-repay after fill |
| type | `"MARKET"` | For immediate execution |
| quantity | (string) | Amount of asset to buy back |

**With `sideEffectType=AUTO_REPAY`**: After the buy fills, the purchased asset automatically repays the outstanding loan. This combines steps 8+9 into one call.

#### Go Implementation Pattern
```go
// MarginBuy places a market buy on cross margin (with auto-repay).
func (b *Adapter) MarginBuy(symbol string, qty string) (string, float64, error) {
    params := map[string]string{
        "symbol":           symbol,
        "isIsolated":       "FALSE",
        "side":             "BUY",
        "type":             "MARKET",
        "quantity":         qty,
        "sideEffectType":   "AUTO_REPAY",
        "newOrderRespType": "FULL",
    }
    body, err := b.client.SpotPost("/sapi/v1/margin/order", params)
    if err != nil {
        return "", 0, fmt.Errorf("MarginBuy: %w", err)
    }
    var resp struct {
        OrderID     int64  `json:"orderId"`
        ExecutedQty string `json:"executedQty"`
        Status      string `json:"status"`
    }
    if err := json.Unmarshal(body, &resp); err != nil {
        return "", 0, err
    }
    filled, _ := strconv.ParseFloat(resp.ExecutedQty, 64)
    return strconv.FormatInt(resp.OrderID, 10), filled, nil
}
```

---

### 9. Repay Loan (Manual)

If not using auto-repay on order, manually repay the borrowed amount + interest.

**Endpoint**: `POST /sapi/v1/margin/borrow-repay`
**Weight**: 1500
**Security**: USER_DATA (signed)
**Base URL**: `https://api.binance.com`

#### Parameters

| Name | Type | Required | Description |
|------|------|----------|-------------|
| asset | STRING | YES | Asset to repay, e.g., "BTC" |
| isIsolated | STRING | YES | `"FALSE"` for cross margin |
| symbol | STRING | Conditional | Required only for isolated margin |
| amount | STRING | YES | Amount to repay (should include interest) |
| type | STRING | YES | `"REPAY"` |
| recvWindow | LONG | NO | Max 60000 |
| timestamp | LONG | YES | |

#### Response
```json
{"tranId": 100000002}
```

#### Important Notes
- Repay amount should cover `borrowed + interest` (query via margin account endpoint).
- Cannot repay more than what's owed (error: "Repayment amount exceeds liability").
- Cannot leave remaining debt below minimum threshold.
- Space requests 100ms apart from other borrow/repay calls.

#### Go Implementation Pattern
```go
func (b *Adapter) MarginRepay(asset string, amount string) (int64, error) {
    params := map[string]string{
        "asset":      asset,
        "isIsolated": "FALSE",
        "amount":     amount,
        "type":       "REPAY",
    }
    body, err := b.client.SpotPost("/sapi/v1/margin/borrow-repay", params)
    if err != nil {
        return 0, fmt.Errorf("MarginRepay: %w", err)
    }
    var resp struct {
        TranID int64 `json:"tranId"`
    }
    if err := json.Unmarshal(body, &resp); err != nil {
        return 0, err
    }
    return resp.TranID, nil
}
```

---

### 10. Additional Query Endpoints

#### Query Margin Order Status

**Endpoint**: `GET /sapi/v1/margin/order`
**Weight**: 10 (IP)

| Name | Type | Required | Description |
|------|------|----------|-------------|
| symbol | STRING | YES | |
| isIsolated | STRING | NO | `"FALSE"` for cross margin |
| orderId | LONG | NO | Either orderId or origClientOrderId |
| origClientOrderId | STRING | NO | |
| recvWindow | LONG | NO | |
| timestamp | LONG | YES | |

**Response**:
```json
{
    "clientOrderId": "ZwfQzuDIGpceVhKW5DvCmO",
    "cummulativeQuoteQty": "65.98253000",
    "executedQty": "0.001",
    "orderId": 213205622,
    "origQty": "0.001",
    "price": "0.00000000",
    "side": "SELL",
    "status": "FILLED",
    "symbol": "BTCUSDT",
    "isIsolated": false,
    "time": 1562133008725,
    "timeInForce": "GTC",
    "type": "MARKET",
    "updateTime": 1562133008725
}
```

#### Query Open Margin Orders

**Endpoint**: `GET /sapi/v1/margin/openOrders`
**Weight**: 10 (IP)

| Name | Type | Required | Description |
|------|------|----------|-------------|
| symbol | STRING | NO | |
| isIsolated | STRING | NO | `"FALSE"` for cross margin |
| recvWindow | LONG | NO | |
| timestamp | LONG | YES | |

#### Cancel Margin Order

**Endpoint**: `DELETE /sapi/v1/margin/order`
**Weight**: 10 (IP)

| Name | Type | Required | Description |
|------|------|----------|-------------|
| symbol | STRING | YES | |
| isIsolated | STRING | NO | `"FALSE"` for cross margin |
| orderId | LONG | NO | Either orderId or origClientOrderId |
| origClientOrderId | STRING | NO | |
| recvWindow | LONG | NO | |
| timestamp | LONG | YES | |

#### Query Margin Trades

**Endpoint**: `GET /sapi/v1/margin/myTrades`
**Weight**: 10 (IP)

| Name | Type | Required | Description |
|------|------|----------|-------------|
| symbol | STRING | YES | |
| isIsolated | STRING | NO | `"FALSE"` for cross margin |
| orderId | LONG | NO | |
| startTime | LONG | NO | |
| endTime | LONG | NO | |
| fromId | LONG | NO | |
| limit | INT | NO | Default 500, max 1000 |
| recvWindow | LONG | NO | |
| timestamp | LONG | YES | |

**Response**:
```json
[
    {
        "commission": "0.06598253",
        "commissionAsset": "USDT",
        "id": 34,
        "isBuyer": false,
        "isMaker": false,
        "orderId": 39324,
        "price": "65982.53",
        "qty": "0.001",
        "symbol": "BTCUSDT",
        "isIsolated": false,
        "time": 1561973357171
    }
]
```

#### Query Interest Rate History

**Endpoint**: `GET /sapi/v1/margin/interestRateHistory`
**Weight**: 1 (IP)

| Name | Type | Required | Description |
|------|------|----------|-------------|
| asset | STRING | YES | e.g., "BTC" |
| vipLevel | INT | NO | Defaults to your level |
| startTime | LONG | NO | Defaults to 7 days ago |
| endTime | LONG | NO | Max 1 month range |
| recvWindow | LONG | NO | |
| timestamp | LONG | YES | |

**Response**:
```json
[{"asset": "BTC", "dailyInterestRate": "0.00025000", "timestamp": 1611544731000, "vipLevel": 1}]
```

**Note**: Returns `dailyInterestRate` (not hourly). Divide by 24 for hourly equivalent.

#### Query Interest History (Charged Interest)

**Endpoint**: `GET /sapi/v1/margin/interestHistory`
**Weight**: 1 (IP)

| Name | Type | Required | Description |
|------|------|----------|-------------|
| asset | STRING | NO | |
| isolatedSymbol | STRING | NO | |
| startTime | LONG | NO | |
| endTime | LONG | NO | Max 30-day range |
| current | LONG | NO | Page, default 1 |
| size | LONG | NO | Default 10, max 100 |
| recvWindow | LONG | NO | |
| timestamp | LONG | YES | |

**Response**:
```json
{
    "rows": [
        {
            "txId": 1352286576452864727,
            "interestAccuredTime": 1672160400000,
            "asset": "USDT",
            "rawAsset": "USDT",
            "principal": "45.3313",
            "interest": "0.00024995",
            "interestRate": "0.00013233",
            "type": "ON_BORROW",
            "isolatedSymbol": ""
        }
    ],
    "total": 1
}
```

Interest types: `PERIODIC`, `ON_BORROW`, `PERIODIC_CONVERTED`, `ON_BORROW_CONVERTED`, `PORTFOLIO`.

---

### 11. Margin Asset / Pair Discovery

#### Get All Cross Margin Pairs

**Endpoint**: `GET /sapi/v1/margin/allPairs`
**Weight**: 1 (IP)
**Security**: NONE (no signing needed)

**Response**:
```json
[
    {
        "base": "BNB",
        "id": 351637150141315861,
        "isBuyAllowed": true,
        "isMarginTrade": true,
        "isSellAllowed": true,
        "quote": "BTC",
        "symbol": "BNBBTC"
    }
]
```

**Note**: Cross margin pairs use spot symbol format (e.g., `BTCUSDT`), same as futures. Check `isMarginTrade == true` before attempting to borrow/trade.

#### Get All Margin Assets

**Endpoint**: `GET /sapi/v1/margin/allAssets`
**Weight**: 1 (IP)
**Security**: NONE

**Response**:
```json
[
    {
        "assetFullName": "Bitcoin",
        "assetName": "BTC",
        "isBorrowable": true,
        "isMortgageable": true,
        "userMinBorrow": "0.00010000",
        "userMinRepay": "0.00010000"
    }
]
```

Check `isBorrowable == true` before attempting to borrow.

---

## Simplified Flow (Recommended for Bot)

Using `sideEffectType=AUTO_BORROW_REPAY`, the flow reduces to just **5 API calls**:

### Entry (3 calls)
1. **Transfer USDT to margin**: `POST /sapi/v1/asset/transfer` with `type=UMFUTURE_MARGIN`
2. **Check borrowable**: `GET /sapi/v1/margin/maxBorrowable`
3. **Borrow + Sell in one shot**: `POST /sapi/v1/margin/order` with `side=SELL`, `sideEffectType=AUTO_BORROW_REPAY`, `type=MARKET`

### Exit (2 calls)
4. **Buy back + Repay in one shot**: `POST /sapi/v1/margin/order` with `side=BUY`, `sideEffectType=AUTO_REPAY`, `type=MARKET`
5. **Transfer USDT back**: `POST /sapi/v1/asset/transfer` with `type=MARGIN_UMFUTURE`

### Monitoring (during hold)
- **Account state**: `GET /sapi/v1/margin/account` (borrowed, interest, margin level)
- **Interest rate**: `GET /sapi/v1/margin/next-hourly-interest-rate` (upcoming cost)

---

## Prerequisites / Account Setup

### 1. Margin Account Must Exist
The `GET /sapi/v1/margin/account` response has `"created": true` if the margin account exists. If `false`, the user must enable margin trading through the Binance web UI first — there is **no API endpoint to create a new cross margin account**. This is a one-time manual step.

### 2. API Key Permissions
Ensure the API key has `enableMargin`, `enableSpotAndMarginTrading`, and `permitsUniversalTransfer` all set to `true`. Check via `GET /sapi/v1/account/apiRestrictions`.

### 3. Cross Margin Leverage
Default leverage is 3x. Can adjust to 5x (Classic) or 10x (Pro) via:
`POST /sapi/v1/margin/max-leverage` with `maxLeverage=3|5|10`
Weight: 3000 (UID), rate limit: 1/min/IP.

For the arb bot, **3x is sufficient** — we only need to borrow ~1x worth of the base asset against USDT collateral.

### 4. Verify Pair Is Borrowable
Before entering a position, verify:
- `GET /sapi/v1/margin/allPairs` — check `isMarginTrade == true` for the symbol
- `GET /sapi/v1/margin/allAssets` — check `isBorrowable == true` for the base asset
- `GET /sapi/v1/margin/maxBorrowable` — check `amount > 0` for the actual borrowable quantity

---

## Error Codes (Margin-Specific)

| Code | Message | Meaning |
|------|---------|---------|
| -3001 | Margin account does not exist | Need to enable margin trading in Binance UI |
| -3003 | Insufficient margin balance | Not enough collateral |
| -3005 | Transfer out amount exceeds max | Exceeds withdrawable amount |
| -3006 | Borrow amount exceeds max | Exceeds maxBorrowable |
| -3007 | Insufficient balance to repay | Not enough asset to repay |
| -3008 | Margin account is not activated | Margin account not created |
| -3015 | Not a valid margin asset | Asset can't be used in margin |
| -3022 | Repay exceeds liability | Trying to repay more than owed |
| -3041 | Margin account does not exist | Cross margin not enabled |
| -3045 | Asset not borrowable | Asset borrowing suspended |

---

## Rate Limit Summary

| Endpoint | Weight | Type |
|----------|--------|------|
| POST /sapi/v1/asset/transfer | 900 | UID |
| POST /sapi/v1/margin/borrow-repay | 1500 | UID (100ms spacing) |
| POST /sapi/v1/margin/order | 6 | UID |
| DELETE /sapi/v1/margin/order | 10 | IP |
| GET /sapi/v1/margin/maxBorrowable | 50 | IP |
| GET /sapi/v1/margin/account | 10 | IP |
| GET /sapi/v1/margin/order | 10 | IP |
| GET /sapi/v1/margin/openOrders | 10 | IP |
| GET /sapi/v1/margin/myTrades | 10 | IP |
| GET /sapi/v1/margin/borrow-repay | 10 | IP |
| GET /sapi/v1/margin/next-hourly-interest-rate | 100 | IP |
| GET /sapi/v1/margin/interestRateHistory | 1 | IP |
| GET /sapi/v1/margin/interestHistory | 1 | IP |
| GET /sapi/v1/margin/allPairs | 1 | IP |
| GET /sapi/v1/margin/allAssets | 1 | IP |
| POST /sapi/v1/margin/max-leverage | 3000 | UID (1/min) |

---

## Integration Notes for Existing Codebase

### Client Methods Already Available
- `client.SpotPost(path, params)` — signed POST to `api.binance.com` (used for transfers, withdrawals)
- `client.SpotGet(path, params)` — signed GET to `api.binance.com` (used for spot balance)
- `client.buildQuery(params)` — adds timestamp + HMAC signature
- All methods use the same API key and secret as futures

### Client Methods Needed
A `SpotDelete(path, params)` method is needed for `DELETE /sapi/v1/margin/order`. Pattern:
```go
func (c *Client) SpotDelete(path string, params map[string]string) ([]byte, error) {
    qs := c.buildQuery(params)
    reqURL := "https://api.binance.com" + path + "?" + qs
    req, err := http.NewRequest(http.MethodDelete, reqURL, nil)
    if err != nil {
        return nil, err
    }
    req.Header.Set("X-MBX-APIKEY", c.apiKey)
    return c.doRequest(req)
}
```

### Symbol Format
Margin uses the same symbol format as spot: `BTCUSDT`, `ETHUSDT`, etc. This is the same format used by futures, so no conversion needed.

### Spot Orderbook
For IOC/limit margin orders, you may need spot orderbook data. The spot orderbook endpoint is:
`GET /api/v3/depth?symbol=BTCUSDT&limit=20` (no signing needed, weight 5).
This differs from the futures orderbook at `/fapi/v1/depth`.

### Spot Contract/Pair Info
Spot trading filters (LOT_SIZE, PRICE_FILTER, MIN_NOTIONAL) come from:
`GET /api/v3/exchangeInfo?symbol=BTCUSDT` (no signing, weight 20).
The filter structure is similar to futures but with different values.
