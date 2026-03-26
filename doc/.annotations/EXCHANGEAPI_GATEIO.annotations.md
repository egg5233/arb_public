# Gate.io API Annotations

> Gotchas and implementation notes discovered during adapter audit (2026-03-26)
> Updated with live API validation results (2026-03-26)

## Live Validation Summary (2026-03-26)

All 9 READ-ONLY endpoints called successfully. Key discrepancies between doc and live responses noted below.

---

## CRITICAL: order_size_min / order_size_max Type Mismatch

**Doc says**: string (`"order_size_min": "1"`)
**Actual live response**: **number** (`"order_size_min": 1`)

Both `order_size_min` and `order_size_max` are returned as JSON **number** (not string) in the live API response from `/futures/usdt/contracts`. Example from BTC_USDT:
```
order_size_min → float64 = 1
order_size_max → float64 = 15000000
```

**Adapter status**: Uses `json.Number` for both fields — this correctly handles both number and string types. No fix needed.

**Doc correction**: The EXCHANGEAPI_GATEIO.md sample JSON shows `"order_size_min": "1"` (string) — should be `"order_size_min": 1` (number).

## CRITICAL: Orderbook `s` (size) Field Type Mismatch

**Doc says**: string (`"s": "100"`)
**Actual live response**: **number** (`"s": 13730`)

The orderbook `asks[].s` and `bids[].s` fields are JSON **number**, not string.

**Adapter status**: Parses `s` as `int64` — this works correctly with the actual number type.

**Doc correction**: The EXCHANGEAPI_GATEIO.md orderbook sample should show `"s": 100` (number), not `"s": "100"` (string). The `p` (price) field is correctly documented as string.

## Orderbook Response: Missing `id` Field

**Doc says**: Response includes `"id": 123456`
**Actual live response**: No `id` field present (only `current`, `update`, `asks`, `bids`)

The orderbook response only contains:
```json
{"current": 1774516313.15, "update": 1774516313.145, "asks": [...], "bids": [...]}
```

Adapter does not use `id` so no impact.

## Position `size` Field Type

**Doc Key Position Fields note says**: `size` is "string type"
**Actual live response**: **number** (`"size": 217` or `"size": 0`)

Position `size` comes back as JSON number. The adapter correctly parses it as `int64`.

## Margin Mode Detection

Gate.io uses two fields to determine margin mode:
- `leverage = "0"` → **cross margin** mode
- `leverage > 0` → **isolated margin** mode
- `cross_leverage_limit` → leverage cap within cross mode (NOT an indicator of isolated mode)

The `pos_margin_mode` response field (`"isolated"` or `"cross"`) is the most reliable source.

**Live confirmation**: Position with `leverage: "0"` has `pos_margin_mode: "cross"` — consistent.

**Gotcha**: Setting cross mode via `leverage=0, cross_leverage_limit=5` results in `cross_leverage_limit > 0` but the position is still in CROSS mode.

## Position Fields: `lever` vs `leverage`

Live responses include BOTH `leverage` (string) AND `lever` (string) on positions. Both contain the same value. The `lever` field is undocumented. Adapter uses `leverage` — correct.

## Size Field Types (v4.106.0+)

Since API v4.106.0, size-related fields (`size`, `left`, `order_size_min`, `order_size_max`) may be returned as **string type** in JSON when `X-Gate-Size-Decimal: 1` header is sent. Without that header (our case), they come as **number**.

Implementations should use `json.Number` for `order_size_min`/`order_size_max` (adapter does this) or `int64` for `size`/`left` (adapter does this).

## Contract Sizing (quanto_multiplier)

All sizing in Gate.io is in **contracts**, not base currency:
- `order_size_min`/`order_size_max`: in contracts
- Position `size`: in contracts (positive=long, negative=short)
- Order `size`/`left`: in contracts

Use `quanto_multiplier` to convert: `base_amount = contracts * quanto_multiplier`

**Gotcha**: When exposing MinSize/MaxSize to the engine (which works in base units), multiply by `quanto_multiplier`.

## Accounts Endpoint: New Fields Not in Doc

The `/futures/usdt/accounts` response includes many fields not documented:
- `maintenance_margin` (string) — account-level maintenance margin
- `cross_available` (string) — available balance in cross mode
- `cross_order_margin` (string)
- `cross_maintenance_margin` (string)
- `cross_initial_margin` (string)
- `cross_unrealised_pnl` (string)
- `margin_mode_name` (string) — e.g. `"single_currency"`
- `position_mode` (string) — e.g. `"single"`
- `margin_mode` (number) — numeric margin mode indicator
- `isolated_position_margin` (string)
- `enable_tiered_mm` (boolean)
- `enable_evolved_classic` (boolean)
- `position_initial_margin` (string)

Adapter only reads `total`, `available`, `currency` — sufficient for balance tracking.

## Contracts: Undocumented Fields

Live `/futures/usdt/contracts` returns additional fields not in doc:
- `funding_cap_ratio` (string) — e.g. `"0.75"` — max funding rate as ratio of maintenance margin rate
- `cross_leverage_default` (string) — default cross leverage
- `launch_time` (number) — contract launch timestamp
- `is_pre_market` (boolean)
- `market_order_size_max` (string) — max market order size
- `config_change_time` (number)
- `risk_limit_max` (string)

## Funding Rate Fields Confirmed

All funding fields from `/futures/usdt/contracts/{contract}` match adapter expectations:
- `funding_rate` → string ✓
- `funding_interval` → number (seconds) ✓
- `funding_next_apply` → number (unix timestamp) ✓
- `funding_rate_limit` → string ✓
- `funding_rate_indicative` → string ✓
- `funding_impact_value` → string
- `funding_offset` → number

## Account Book (Funding Fees) Fields

Live response from `/futures/usdt/account_book?type=fund`:
- `change` → string ✓ (matches adapter)
- `time` → number (float64) ✓ (matches adapter)
- Additional fields: `balance` (string), `text` (string = contract name), `contract` (string), `type` (string), `trade_id` (string), `id` (string), `update_id` (string), `biz_info` (string)

Note: `id` and `update_id` are returned as **string**, not number.

## Position Close (GetClosePnL) Fields

Live response confirms all adapter-parsed fields:
- `pnl` → string ✓
- `pnl_pnl` → string ✓
- `pnl_fund` → string ✓
- `pnl_fee` → string ✓
- `side` → string ("long" or "short") ✓
- `long_price` → string ✓
- `short_price` → string ✓
- `accum_size` → string ✓
- `time` → number (float64) ✓

Additional undocumented fields: `first_open_time` (number), `time_us` (number — microsecond timestamp), `max_size` (string), `text` (string), `leverage` (string), `lever` (string), `margin_mode` (string), `voucher_id` (number)

## User Trades (my_trades) Fields

Live response field types:
- `id` → number (float64) — adapter uses `int64` ✓
- `order_id` → **string** — adapter uses `string` ✓
- `contract` → string ✓
- `size` → number (float64) — adapter uses `int64` ✓
- `price` → string ✓
- `fee` → string ✓
- `create_time` → number (float64, with ms precision like `1.7745138030739e+09`) ✓
- `close_size` → number — not in adapter (not needed)
- `role` → string ("taker"/"maker") — not in adapter
- `text` → string
- `point_fee` → string
- `amend_text` → string
- `biz_info` → string

## Order Fields (finished orders)

Live response field types:
- `id` → number (float64) — adapter uses `int64` ✓
- `size` → number (float64) — adapter uses `int64` ✓
- `left` → number (float64) — adapter uses `int64` ✓
- `price` → string ✓
- `fill_price` → string ✓
- `status` → string ✓
- `tif` → string ✓
- `text` → string ✓
- `contract` → string ✓
- `is_reduce_only` → boolean ✓
- `is_close` → boolean
- `is_liq` → boolean
- `finish_as` → string ✓
- `finish_time` → number (float64)
- `create_time` → number (float64)
- `iceberg` → number (float64, not string as doc implies)
- `pos_margin_mode` → string
- `pnl` → string (realized pnl of the order)
- `pnl_margin` → string
- `mkfr` → string (maker fee rate)
- `tkfr` → string (taker fee rate)
- `stp_id` → number
- `stp_act` → string
- `update_id` → number
- `bbo` → string

Note: `iceberg` is number in live response but doc example shows string `"0"`.

## IOC Order finish_as Values

IOC orders that **partially fill** get `finish_as: "ioc"` (NOT `"filled"`). Only fully filled orders get `finish_as: "filled"`. The full set of finish_as values:
- `filled` — fully filled
- `cancelled` — user cancelled
- `ioc` — IOC order expired (may have partial fill)
- `liquidated`, `auto_deleveraged`, `reduce_only`, `position_closed`, `stp`, `unknown`

## Dual Mode (One-Way vs Hedge)

The `POST /futures/{settle}/dual_mode` endpoint expects a **JSON body** `{"dual_mode": false}`, not a query parameter.

## WebSocket order_book Subscribe Format

The `futures.order_book` channel expects parameters in the payload array:
```json
{"payload": ["BTC_USDT", "5", "0"]}  // [contract, level, interval_ms]
```

Not as top-level fields like `"accuracy"` or `"limit"`.

## WebSocket book_ticker Field Names

The actual WebSocket book_ticker push uses abbreviated field names (not matching REST docs):
- `"s"` = contract/symbol (docs show `"contract"`)
- `"b"` = best bid price, `"B"` = best bid size
- `"a"` = best ask price, `"A"` = best ask size

Go's case-insensitive JSON would collide `b`/`B` and `a`/`A` — use raw map parsing.
