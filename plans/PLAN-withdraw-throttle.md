# Plan: Cross-Exchange Withdraw Throttle
Version: v5
Date: 2026-04-18
Status: REVIEWING

## Incident

At 2026-04-18 12:45:10 UTC on v0.32.24, rebalance scheduled two back-to-back withdraws from gateio (1 ms apart):

```
12:45:07.907 batched withdraw gateio→binance 48.41 USDT (sent)
12:45:10.026 gateio withdraw txid=w95036027 confirmed
12:45:10.027 batched withdraw gateio→bingx 134.54 USDT (1ms later)
12:45:10.353 ERROR: gateio API error label=TOO_FAST message=Withdrawal frequency is limited to 10s
```

Gate.io enforces ≥10 seconds between withdraws. Bybit documents similar 10s-per-coin/chain secondary limit. Bot's batched-withdraw loop had no per-donor throttle.

Net effect: bingx's leg never funded → override dropped → entry tier-3 → no position.

## Root cause (Codex-verified)

- `batchedWds` map keyed by `donor+"->"+recipient` at `internal/engine/allocator.go:1775` → Gate.io produced two separate batches.
- Executor loop at `internal/engine/allocator.go:1820` dispatches all batches sequentially with no inter-request delay.
- Existing code has NO generic per-exchange withdraw timing tracker. (BingX client-side 150ms limiter at `pkg/exchange/bingx/client.go:55` is a different, unrelated limiter.)

## Change (Codex-supplied concrete code)

### 1. `internal/engine/allocator.go` — throttle loop before each Withdraw call

BEFORE (around `:1820`):
```go
for _, bk := range batchKeys {
	bw := batchedWds[bk]
	withdrawAmtForAPI := bw.netTotal
	if bw.isGross {
		withdrawAmtForAPI = bw.netTotal + bw.fee
	}
	amtStr := fmt.Sprintf("%.4f", withdrawAmtForAPI)
	e.log.Info("rebalance: batched withdraw %s->%s net=%.2f fee=%.4f amount=%.2f via %s",
		bw.donor, bw.recipient, bw.netTotal, bw.fee, withdrawAmtForAPI, bw.chain)

	wdResult, err := e.exchanges[bw.donor].Withdraw(exchange.WithdrawParams{
		Coin:    "USDT",
		Chain:   bw.chain,
		Address: bw.destAddr,
		Amount:  amtStr,
	})
```

AFTER:
```go
lastWithdrawAt := make(map[string]time.Time, len(batchedWds))
for _, bk := range batchKeys {
	bw := batchedWds[bk]
	withdrawAmtForAPI := bw.netTotal
	if bw.isGross {
		withdrawAmtForAPI = bw.netTotal + bw.fee
	}
	amtStr := fmt.Sprintf("%.4f", withdrawAmtForAPI)

	if minMs := e.cfg.WithdrawMinIntervalMs; minMs > 0 {
		if last, ok := lastWithdrawAt[bw.donor]; ok {
			wait := time.Duration(minMs)*time.Millisecond - time.Since(last)
			if wait > 0 {
				e.log.Info("rebalance: withdraw throttle donor=%s wait=%v", bw.donor, wait.Round(time.Millisecond))
				time.Sleep(wait)
			}
		}
		lastWithdrawAt[bw.donor] = time.Now()
	}

	e.log.Info("rebalance: batched withdraw %s->%s net=%.2f fee=%.4f amount=%.2f via %s",
		bw.donor, bw.recipient, bw.netTotal, bw.fee, withdrawAmtForAPI, bw.chain)

	wdResult, err := e.exchanges[bw.donor].Withdraw(exchange.WithdrawParams{
		Coin:    "USDT",
		Chain:   bw.chain,
		Address: bw.destAddr,
		Amount:  amtStr,
	})
```

Timing is anchored at request dispatch (updated to `time.Now()` BEFORE the actual `Withdraw` API call completes) — correct semantics since rate limit is measured from request moment, not response.

### 2. `internal/config/config.go` — add `WithdrawMinIntervalMs`

Add to `Config` struct:
```go
WithdrawMinIntervalMs int // min interval (ms) between withdraws from same donor in batched rebalance (default: 11000; 0 = disabled)
```

Add to `jsonRisk` struct:
```go
WithdrawMinIntervalMs *int `json:"withdraw_min_interval_ms"`
```

Add to defaults block:
```go
WithdrawMinIntervalMs: 11000,
```

Add to `applyJSON`:
```go
if rk.WithdrawMinIntervalMs != nil && *rk.WithdrawMinIntervalMs >= 0 {
    c.WithdrawMinIntervalMs = *rk.WithdrawMinIntervalMs
}
```

Add to `SaveJSON` risk section (at `internal/config/config.go:~1429`):

BEFORE:
```go
risk := getMap(raw, "risk")
risk["margin_l3_threshold"] = c.MarginL3Threshold
risk["margin_l4_threshold"] = c.MarginL4Threshold
risk["margin_l4_headroom"] = c.MarginL4Headroom
risk["margin_l5_threshold"] = c.MarginL5Threshold
risk["l4_reduce_fraction"] = c.L4ReduceFraction
risk["margin_safety_multiplier"] = c.MarginSafetyMultiplier
risk["entry_margin_headroom"] = c.EntryMarginHeadroom
risk["risk_monitor_interval_sec"] = c.RiskMonitorIntervalSec
```

AFTER (add one line):
```go
risk := getMap(raw, "risk")
risk["margin_l3_threshold"] = c.MarginL3Threshold
risk["margin_l4_threshold"] = c.MarginL4Threshold
risk["margin_l4_headroom"] = c.MarginL4Headroom
risk["margin_l5_threshold"] = c.MarginL5Threshold
risk["l4_reduce_fraction"] = c.L4ReduceFraction
risk["margin_safety_multiplier"] = c.MarginSafetyMultiplier
risk["entry_margin_headroom"] = c.EntryMarginHeadroom
risk["withdraw_min_interval_ms"] = c.WithdrawMinIntervalMs
risk["risk_monitor_interval_sec"] = c.RiskMonitorIntervalSec
```

### 3. `internal/api/handlers.go` — API surface

#### 3a. `configRiskResponse` struct — add field:
```go
type configRiskResponse struct {
    MarginL3Threshold           float64 `json:"margin_l3_threshold"`
    MarginL4Threshold           float64 `json:"margin_l4_threshold"`
    MarginL5Threshold           float64 `json:"margin_l5_threshold"`
    L4ReduceFraction            float64 `json:"l4_reduce_fraction"`
    MarginSafetyMultiplier      float64 `json:"margin_safety_multiplier"`
    WithdrawMinIntervalMs       int     `json:"withdraw_min_interval_ms"`
    EntryMarginHeadroom         float64 `json:"entry_margin_headroom"`
    RiskMonitorIntervalSec      int     `json:"risk_monitor_interval_sec"`
    // ...remainder unchanged
}
```

#### 3b. `buildConfigResponse` populate:
```go
Risk: configRiskResponse{
    MarginL3Threshold:           s.cfg.MarginL3Threshold,
    MarginL4Threshold:           s.cfg.MarginL4Threshold,
    MarginL5Threshold:           s.cfg.MarginL5Threshold,
    L4ReduceFraction:            s.cfg.L4ReduceFraction,
    MarginSafetyMultiplier:      s.cfg.MarginSafetyMultiplier,
    WithdrawMinIntervalMs:       s.cfg.WithdrawMinIntervalMs,
    EntryMarginHeadroom:         s.cfg.EntryMarginHeadroom,
    RiskMonitorIntervalSec:      s.cfg.RiskMonitorIntervalSec,
    // ...
},
```

#### 3c. `riskUpdate` struct — add pointer field:
```go
type riskUpdate struct {
    MarginL3Threshold           *float64 `json:"margin_l3_threshold"`
    MarginL4Threshold           *float64 `json:"margin_l4_threshold"`
    MarginL5Threshold           *float64 `json:"margin_l5_threshold"`
    L4ReduceFraction            *float64 `json:"l4_reduce_fraction"`
    MarginSafetyMultiplier      *float64 `json:"margin_safety_multiplier"`
    WithdrawMinIntervalMs       *int     `json:"withdraw_min_interval_ms"`
    EntryMarginHeadroom         *float64 `json:"entry_margin_headroom"`
    RiskMonitorIntervalSec      *int     `json:"risk_monitor_interval_sec"`
    // ...
}
```

#### 3d. `handlePostConfig` apply:
Add after `MarginSafetyMultiplier` handling:
```go
if rk.WithdrawMinIntervalMs != nil && *rk.WithdrawMinIntervalMs >= 0 {
    s.cfg.WithdrawMinIntervalMs = *rk.WithdrawMinIntervalMs
}
```

#### 3e. Redis flat-map persistence (v4 addition)

At `handlers.go:~1518`, add `withdraw_min_interval_ms` to the flat risk map written to Redis (`arb:config`):

BEFORE:
```go
"margin_safety_multiplier":  strconv.FormatFloat(snapshot.Risk.MarginSafetyMultiplier, 'f', -1, 64),
"risk_monitor_interval_sec": strconv.Itoa(snapshot.Risk.RiskMonitorIntervalSec),
```

AFTER:
```go
"margin_safety_multiplier":  strconv.FormatFloat(snapshot.Risk.MarginSafetyMultiplier, 'f', -1, 64),
"withdraw_min_interval_ms":  strconv.Itoa(snapshot.Risk.WithdrawMinIntervalMs),
"risk_monitor_interval_sec": strconv.Itoa(snapshot.Risk.RiskMonitorIntervalSec),
```

### 4. Frontend — `web/src/pages/Config.tsx`

Add `NumberField` above `riskMonitorInterval`:

```tsx
<NumberField
    label={t('cfg.field.withdrawMinIntervalMs')}
    desc={t('cfg.desc.withdrawMinIntervalMs')}
    value={getByPath(config, ['risk', 'withdraw_min_interval_ms'])}
    unit="ms"
    onChange={(v) => handleChange(['risk', 'withdraw_min_interval_ms'], v)}
/>
```

### 5. i18n

#### `web/src/i18n/en.ts` — add:
```ts
'cfg.field.withdrawMinIntervalMs': 'Withdraw Min Interval (ms)',
'cfg.desc.withdrawMinIntervalMs': 'Minimum interval in milliseconds between batched rebalance withdraw requests from the same donor exchange. 0 = disabled.',
```

#### `web/src/i18n/zh-TW.ts` — add:
```ts
'cfg.field.withdrawMinIntervalMs': '提現最小間隔 (毫秒)',
'cfg.desc.withdrawMinIntervalMs': '同一來源交易所在批次再平衡提現請求之間的最小間隔（毫秒）。0 = 停用。',
```

### Note on zh-TW encoding
The zh-TW snippet is valid UTF-8 in the plan file. If Codex's PowerShell reader shows mojibake, that is a reader-side encoding issue (CP950 default on Windows) — the raw file bytes are correct and the implementer should copy them verbatim.

## Tests

Add regression coverage for the new config field in `internal/api/config_handlers_test.go`:
- GET /api/config returns `withdraw_min_interval_ms` in risk section with configured default (11000).
- POST /api/config with `{"risk":{"withdraw_min_interval_ms":5000}}` updates in-memory config.
- After POST, Redis flat-map contains `withdraw_min_interval_ms=5000`.
- After reload cycle (SaveJSON → load), value persists.

Extends existing risk-config test pattern at `internal/api/config_handlers_test.go:466-487` and `:575-624` — follow same structure as `margin_safety_multiplier` coverage.

## Risk

- 11s per same-donor withdraw adds ~10s delay when one donor funds multiple recipients (e.g. gateio→2 recipients = +11s wait, gateio→3 = +22s). Within 5-min `pollDeadline`.
- Applied globally as safety default — gateio confirmed 10s, bybit documented 10s-per-coin/chain. Binance/bitget/bingx/okx undocumented; 11s is a safe upper bound.
- No interaction with v0.32.24 Bugs A/B/C/D.

## Version
Bump to `v0.32.25`.

## Review History
- v1: Codex NEEDS-REVISION — anchor timing at dispatch not completion, full config wiring missing. Supplied exact AFTER code.
- v2: Codex NEEDS-REVISION — SaveJSON only placeholder comment; frontend/i18n deferred but repo requires full wiring.
- v3: Codex NEEDS-REVISION — flat Redis config persistence map at handlers.go:~1518 omitted.
- v4: Codex NEEDS-REVISION — zh-TW mojibake concern (file content is valid UTF-8; Codex's PowerShell reader issue); missing Tests section.
- v5: REVIEWING — added Tests section with concrete assertions extending existing config_handlers_test.go patterns. Clarified zh-TW encoding.
