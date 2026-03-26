 use playwright skills (headless=true) and check the link:                                                                                                                            
    Binance Futures:https://developers.binance.com/docs/derivatives/usds-margined-futures
    Binance Spot/Wallet:https://developers.binance.com/docs/wallet
    Bitget Contract:https://www.bitget.com/api-doc/contract/intro
    Bitget Spot/Wallet:https://www.bitget.com/api-doc/spot/intro
    Okx:https://www.okx.com/docs-v5/en/                                                                                                                                                
    Gate.io:https://www.gate.com/docs/developers/apiv4/en/                                                                                                                             
    Bybit:https://bybit-exchange.github.io/docs/v5/guide                                                                                                                               
    BingX:https://bingx-api.github.io/docs-v3/#/en/info . 
Document it into CODEX_EXCHANGEAPI_{EXCHANGE_NAME}.md . Keep in mind that the format should be readable and usable for further AI modl to    
complete any exchange-api related task. 

example:
```
## REST API Endpoints

### Perpetual Swap — Market Data

#### Get Contract List (USDT-M Perp Futures Symbols)
- **Endpoint**: `GET /openApi/swap/v2/quote/contracts`
- **Rate Limit**: 500/10s (IP)
- **Auth**: No signature required
- **Parameters**:

| Name | Type | Required | Description |
|---|---|---|---|
| symbol | string | No | Trading pair, e.g. BTC-USDT |
| timestamp | int64 | No | Request timestamp (ms) |
| recvWindow | int64 | No | Request valid window (ms) |

- **Response Fields**:

| Field | Type | Description |
|---|---|---|
| contractId | string | Contract ID |
| symbol | string | Trading pair (e.g. BTC-USDT) |
| quantityPrecision | int64 | Quantity decimal places |
| pricePrecision | int64 | Price decimal places |
| makerFeeRate | float64 | Maker fee rate |
| takerFeeRate | float64 | Taker fee rate |
| tradeMinQuantity | float64 | Min trade quantity (COIN) |
| tradeMinUSDT | float64 | Min trade quantity (USDT) |
| maxLongLeverage | int64 | Max long leverage |
| maxShortLeverage | int64 | Max short leverage |
| currency | string | Settlement currency (e.g. USDT) |
| asset | string | Trading asset (e.g. BTC) |
| status | int64 | 1=online, 25=no-open, 5=pre-online, 0=offline |
| apiStateOpen | string | Can API open positions ("true"/"false") |
| apiStateClose | string | Can API close positions ("true"/"false") |
| launchTime | long | Listing time (ms) |
| maintainTime | long | No-open start time (ms) |
| offTime | long | Offline time (ms) |

- **Response Example**:
```json
{
  "code": 0,
  "msg": "",
  "data": [
    {
      "contractId": "100",
      "symbol": "BTC-USDT",
      "size": "0",
      "quantityPrecision": 4,
      "pricePrecision": 1,
      "feeRate": 0.0005,
      "makerFeeRate": 0.0002,
      "takerFeeRate": 0.0005,
      "tradeMinLimit": 0,
      "tradeMinQuantity": 0.0001,
      "tradeMinUSDT": 2,
      "maxLongLeverage": 125,
      "maxShortLeverage": 125,
      "currency": "USDT",
      "asset": "BTC",
      "status": 1,
      "apiStateOpen": "true",
      "apiStateClose": "true",
      "launchTime": 1586275200000,
      "maintainTime": 0,
      "offTime": 0
    }
  ]
}
```