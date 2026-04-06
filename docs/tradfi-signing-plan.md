# Plan: Binance TradFi-Perps Agreement Check & Signing

## Background
Binance commodity futures (NATGASUSDT, XAGUSDT, etc.) require signing a TradFi-Perps agreement via `POST /fapi/v1/stock/contract` (idempotent, weight 50, params: timestamp only). Without it, orders return error code `-4411`. Other 5 exchanges don't need this.

## Flow
```
Startup -> read config.TradFiSigned
  +- true  -> skip
  +- false -> check: is Binance in exchanges?
                +- no  -> skip (no banner)
                +- yes -> dashboard shows orange banner + [Sign] button
                            User clicks [Sign] -> POST /api/sign-tradfi
                              -> backend: type-assert binance to TradFiSigner
                              -> call Binance POST /fapi/v1/stock/contract
                              -> success -> Lock() -> TradFiSigned=true -> SaveJSON -> Unlock()
                              -> SaveJSON fail -> return 500 (signing succeeded but persist failed)
                              -> SaveJSON ok -> return { ok: true, data: { signed: true } }
                              -> frontend: banner disappears
```

## Changes (8 files)

### 1. `pkg/exchange/types.go`
Add optional interface (same pattern as `PermissionChecker`, `TradingFeeProvider`):
```go
// TradFiSigner is an optional interface for exchanges that require
// a TradFi-Perps agreement before commodity contract trading.
type TradFiSigner interface {
    SignTradFi() error
}
```

### 2. `pkg/exchange/binance/adapter.go`
Implement the interface:
```go
// Compile-time check
var _ exchange.TradFiSigner = (*Adapter)(nil)

func (b *Adapter) SignTradFi() error {
    _, err := b.client.Post("/fapi/v1/stock/contract", map[string]string{})
    return err
}
```
Note: `client.Post()` signature is `func (c *Client) Post(path string, params map[string]string) ([]byte, error)` (line 112) and auto-adds timestamp via `buildQuery`.

### 3. `internal/config/config.go`
Config struct -- add field alongside existing top-level bools:
```go
TradFiSigned bool // Binance TradFi-Perps agreement signed (persisted)
```

jsonConfig -- add JSON mapping (top-level, same level as `dry_run`):
```go
TradFiSigned *bool `json:"tradfi_signed"`
```

Load() -- read from JSON:
```go
if jc.TradFiSigned != nil {
    cfg.TradFiSigned = *jc.TradFiSigned
}
```
Default is Go zero-value `false` -- meaning "not yet signed". New installs without this field show the banner IF Binance is configured.

SaveJSON() -- write back:
```go
raw["tradfi_signed"] = c.TradFiSigned
```

handlePostConfig protection -- NOT needed. `handlePostConfig` (line 891) decodes into a `configUpdate` struct (line 713) with explicit pointer fields and applies field-by-field. `TradFiSigned` is not in `configUpdate`, so dashboard config saves cannot overwrite it.

Note: Config uses named field `mu sync.RWMutex` (line 27) with wrapper methods `Lock()`/`Unlock()`/`RLock()`/`RUnlock()` (lines 588-598), not embedded sync.RWMutex.

### 4. `internal/api/server.go`
Register routes (next to `/api/permissions`):
```go
mux.HandleFunc("/api/tradfi-status", s.cors(s.authMiddleware(s.handleTradFiStatus)))
mux.HandleFunc("/api/sign-tradfi", s.cors(s.authMiddleware(s.handleSignTradFi)))
```

### 5. `internal/api/handlers.go`
Two new handlers:

**handleTradFiStatus (GET)** -- runtime check with RLock (matches existing pattern at handlers.go lines 682, 1648, 1778):
```go
func (s *Server) handleTradFiStatus(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        return
    }
    // If Binance is not configured, no signing needed -- report as signed
    signed := true
    if _, hasBinance := s.exchanges["binance"]; hasBinance {
        s.cfg.RLock()
        signed = s.cfg.TradFiSigned
        s.cfg.RUnlock()
    }
    writeJSON(w, http.StatusOK, Response{OK: true, Data: map[string]bool{
        "signed": signed,
    }})
}
```

**handleSignTradFi (POST)** -- network I/O OUTSIDE config lock, SaveJSON failure returns 500:
```go
func (s *Server) handleSignTradFi(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        return
    }
    // Find binance exchange, type-assert to TradFiSigner
    exc, ok := s.exchanges["binance"]
    if !ok {
        writeJSON(w, http.StatusOK, Response{OK: true, Data: map[string]bool{"signed": true}})
        return
    }
    signer, ok := exc.(exchange.TradFiSigner)
    if !ok {
        writeJSON(w, http.StatusOK, Response{OK: true, Data: map[string]bool{"signed": true}})
        return
    }
    // Call Binance API (outside config lock)
    if err := signer.SignTradFi(); err != nil {
        writeJSON(w, http.StatusInternalServerError, Response{Error: fmt.Sprintf("Binance TradFi sign failed: %v", err)})
        return
    }
    // Success -- lock config briefly to persist
    s.cfg.Lock()
    s.cfg.TradFiSigned = true
    err := s.cfg.SaveJSON()
    s.cfg.Unlock()
    if err != nil {
        s.log.Error("save tradfi_signed to config.json: %v", err)
        writeJSON(w, http.StatusInternalServerError, Response{Error: "signed on Binance but failed to persist config"})
        return
    }
    writeJSON(w, http.StatusOK, Response{OK: true, Data: map[string]bool{"signed": true}})
}
```

### 6. `web/src/hooks/useApi.ts`
Add two methods:
```typescript
const getTradFiStatus = useCallback(() => {
    return request<{ signed: boolean }>('/api/tradfi-status');
}, []);

const signTradFi = useCallback(() => {
    return request<{ signed: boolean }>('/api/sign-tradfi', { method: 'POST' });
}, []);
```
Add to return object.

### 7. `web/src/App.tsx`
New state:
```typescript
const [tradfiUnsigned, setTradfiUnsigned] = useState(false);
```

In login useEffect (alongside `silentCheckUpdate`):
```typescript
api.getTradFiStatus().then(d => {
    if (!d.signed) {
        const dismissed = sessionStorage.getItem('arb_tradfi_dismissed');
        if (!dismissed) setTradfiUnsigned(true);
    }
}).catch(() => {});
```

Banner (below update banner, orange instead of blue):
```tsx
{tradfiUnsigned && (
    <div className="bg-orange-600 text-white px-4 py-2 flex items-center justify-between text-sm">
        <span>{t('tradfi.banner')}</span>
        <div className="flex items-center gap-2">
            <button onClick={handleSignTradFi} className="bg-white/20 hover:bg-white/30 px-3 py-1 rounded text-xs font-medium">
                {t('tradfi.sign')}
            </button>
            <button onClick={() => { sessionStorage.setItem('arb_tradfi_dismissed', '1'); setTradfiUnsigned(false); }}
                className="text-white/70 hover:text-white">x</button>
        </div>
    </div>
)}
```

Sign handler:
```typescript
const handleSignTradFi = useCallback(async () => {
    try {
        await api.signTradFi();
        setTradfiUnsigned(false);
    } catch { /* show error? */ }
}, [api]);
```

Dismiss: uses **sessionStorage** (not localStorage) -- closing browser tab resets it. This is intentional: unsigned is a functional blocker, should reappear on new session until signed.

Frontend only checks TradFi status on login/page load. If Binance is added mid-session (hot-reload), the banner won't appear until next page load. This is acceptable since adding an exchange requires a restart anyway.

### 8. `web/src/i18n/en.ts` + `zh-TW.ts`
```typescript
// en.ts
'tradfi.banner': 'Binance TradFi-Perps agreement not signed -- commodity contracts cannot trade',
'tradfi.sign': 'Sign Now',
'tradfi.signed': 'Agreement signed successfully',

// zh-TW.ts
'tradfi.banner': 'Binance TradFi-Perps 協議未簽署，商品合約無法交易',
'tradfi.sign': '立即簽署',
'tradfi.signed': '協議簽署成功',
```

## Design Decisions

1. **TradFiSigner interface** -- follows existing optional-interface pattern (PermissionChecker, TradingFeeProvider). API layer never imports `pkg/exchange/binance` directly.
2. **Network I/O outside config lock** -- call Binance first, only Lock() briefly for in-memory update + SaveJSON persist.
3. **SaveJSON failure = HTTP 500** -- signing succeeded on Binance (idempotent, can retry) but config not persisted. User sees error, can retry. Next restart would still show banner since config wasn't saved.
4. **Session dismiss (sessionStorage)** -- not 24h localStorage. Unsigned = functional blocker, should reappear on new browser session until signed.
5. **Runtime Binance check** -- `handleTradFiStatus` checks `s.exchanges["binance"]` at runtime. No Binance = `signed: true` (no banner). Binance added later = `TradFiSigned` is Go zero-value `false` = banner appears on next page load. Signed once = persisted in config = never again.
6. **Config protection** -- `TradFiSigned` is NOT in `configUpdate` struct, so `handlePostConfig` field-by-field apply cannot overwrite it. No additional protection needed.
7. **Config mutex** -- uses named field `mu sync.RWMutex` (line 27) with wrapper methods `Lock()`/`Unlock()`/`RLock()`/`RUnlock()` (lines 588-598), not embedded.

## Edge Cases

| Scenario | Behavior |
|----------|----------|
| Fresh install, no Binance | No banner (Binance not in exchanges) |
| Fresh install, with Binance | Banner shows (TradFiSigned defaults false) |
| User adds Binance later | Next page load: banner shows (TradFiSigned still false) |
| User signs via dashboard | Config persisted, banner gone permanently |
| User removes Binance after signing | No banner (Binance not in exchanges) |
| POST /fapi/v1/stock/contract already signed | Idempotent, returns SUCCESS again |
| SaveJSON fails after successful sign | HTTP 500, user can retry (Binance side already signed) |
| Dashboard config save (/api/config POST) | TradFiSigned preserved (not in configUpdate struct) |

## NOT doing
- Not changing PermissionResult / Permissions page
- Not changing depth fill logic or adding -4411 special error handling
- Not auto-signing on startup (user must click)
