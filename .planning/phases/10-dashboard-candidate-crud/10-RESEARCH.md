# Phase 10: Dashboard Candidate CRUD - Research

**Researched:** 2026-04-25
**Domain:** Go HTTP handler extension + React modal CRUD over `/api/config`, hot-reload via shared `*Config` pointer
**Confidence:** HIGH (codebase entirely under inspection; no external library research needed)

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Modal Behavior & Layout**
- **D-01:** Single modal with `mode: "add" | "edit"` prop. Delete uses lightweight confirm dialog.
- **D-02:** Portal-style overlay; backdrop / ESC / Cancel all dismiss without saving.
- **D-03:** On-submit validation with inline field errors; Save enabled by default.
- **D-04:** "Add candidate" button above table, right-aligned, matching tab header pattern.

**Field Validation Rules**
- **D-05:** `symbol` — `^[A-Z0-9]+USDT$`, auto-uppercase, required.
- **D-06:** `long_exch` / `short_exch` — `<select>` of 6 values: `binance, bybit, gateio, bitget, okx, bingx`. Both required, must differ.
- **D-07:** `threshold_bps` — positive integer 50–1000, default 200.
- **D-08:** `max_position_usdt` — number 100–50000, default 5000, step 100.
- **D-09:** `modeled_slippage_bps` — number 0–100, default 5, step 1.
- **D-10:** Server-side re-validation in `internal/api/handlers.go` (defense in depth). `{ok: false, error}` envelope.

**Identity & Edit Semantics**
- **D-11:** Tuple `(symbol, long_exch, short_exch)` is natural key; collisions forbidden.
- **D-12:** Edit identifies row by existing tuple. Tuple-changing edits allowed but no collisions.
- **D-13:** Edit preserves Phase-9 `enabled` flag; modal does not expose it.

**Delete Safety**
- **D-14:** Block delete when active position open (tuple match against `pg:positions:active`). Returns `{ok: false, error: "candidate has active position; close it first"}`.
- **D-15:** Single-step confirm dialog showing full candidate detail. No "type symbol to confirm".

**Backend API Shape**
- **D-16:** Reuse existing `POST /api/config` with `pricegap.candidates` payload (full array replace, last-write-wins). No new route.
- **D-17:** Server applies in-memory + writes `config.json` via `SaveJSON` (`.bak` backup) — same path as `SpotFutures*` / `Risk*` fields.
- **D-18:** `GET /api/config` already returns `PriceGapCandidates`. Frontend seeds from this on tab mount.

**Tracker Hot-Reload**
- **D-19:** `pricegaptrader/tracker` reads `cfg.PriceGapCandidates` at start of each scan tick. Plan must verify no startup-cache.

**Real-Time Update Surface**
- **D-20:** No new WebSocket broadcast. POST response contains updated list; frontend re-renders from response.

**i18n Lockstep**
- **D-21:** Every key added to both `web/src/i18n/en.ts` and `web/src/i18n/zh-TW.ts` in same commit. Namespace `priceGap.candidates.*`.

### Claude's Discretion
- Tailwind class names for modal layout (match `Positions.tsx` aesthetic)
- Form field order within modal (suggest: symbol → exchanges → threshold → sizing)
- Optimistic-UI pattern (re-render from POST response is fine)
- Whether to show candidate id or just the tuple in table
- Storybook stories / component-level tests for modal

### Deferred Ideas (OUT OF SCOPE)
- pg-admin CLI parity for add/edit/delete (v2.2 if requested)
- WS broadcast on candidate change (v3.0)
- Bulk import / CSV upload
- Audit log of candidate edits (v3.0)
- Optimistic UI for candidate table
- Per-candidate "test fire" button
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| PG-OPS-07 | Price-Gap tab gains Add/Edit/Delete candidate modal. Fields: symbol, long_exch, short_exch, threshold_bps, max_position_usdt, modeled_slippage_bps. Posts to `/api/config`, persists via existing `SaveJSON`. EN+zh-TW locale keys lockstep. Existing Disable/Re-enable buttons unchanged. | All findings below; specifically §"Backend Wiring", §"Frontend Modal Pattern", §"i18n", and §"Tracker Hot-Reload" |
</phase_requirements>

## Project Constraints (from CLAUDE.local.md)

| Constraint | Implication for this phase |
|------------|---------------------------|
| **npm lockdown — no new packages, only `npm ci`** | Modal must be hand-rolled with React + Tailwind (already the established pattern in `PriceGap.tsx`) |
| **DO NOT modify `config.json` directly** | All writes go through `s.cfg.SaveJSON()` / `SaveJSONWithExchangeSecretOverrides()` invoked inside `handlePostConfig` |
| **Build order: `npm run build` (web/) THEN `go build`** | Frontend must build first (Vite emits to `web/dist/`, embedded by `web/embed.go` `//go:embed all:dist`) |
| **i18n: en.ts + zh-TW.ts in lockstep** | `TranslationKey` is `keyof typeof en` — missing zh-TW keys typecheck OK but render English at runtime. Manual / human-review parity gate |
| **Use LSP (gopls) over Grep for Go navigation** | Planner should hand off Go edits to teammates instructed to use LSP `goToDefinition` / `findReferences` |
| **Use graphify routers before raw grep** | `graphify-publish/AI_ROUTER.md` is the entry point for codebase questions |
| **Delegation mode**: 2+ files = delegate to teammates | Phase 10 touches 5+ files (handler + struct + tracker + 2 i18n + tsx) — must be delegated, not implemented inline |

## Summary

Phase 10 is a **pure additive extension** to two existing surfaces:
1. **Backend** (`internal/api/handlers.go`): the `priceGapUpdate` struct currently has only `{Enabled, PaperMode, DebugLog}` — add `Candidates *[]models.PriceGapCandidate`. Apply path is the existing `handlePostConfig` → `s.cfg.SaveJSONWithExchangeSecretOverrides()` → `s.configNotifier.Notify()`. Validation block sits between decode and apply.
2. **Frontend** (`web/src/pages/PriceGap.tsx`): Phase 9 already established the modal pattern (`fixed inset-0 bg-black/50 z-50`, ESC handler, `setModalBusy`/`setModalError` state). Add a third modal alongside the existing `disableTarget` and `reenableTarget` modals. Re-render the candidate table from the POST response (which returns full snapshot via `s.buildConfigResponse()`).

The tracker hot-reload story is solved by accident: tracker holds `*config.Config`, iterates `t.cfg.PriceGapCandidates` directly each tick (3 sites), and the POST handler mutates the same slice under `s.cfg.Lock()`. Zero new code needed for hot-reload — the planner must only **verify** no copy-cache exists in `NewTracker()`.

**Primary recommendation:** Treat this as a 5-task plan: (1) extend `priceGapUpdate` + apply block; (2) add server-side validation + tuple uniqueness + active-position safety check; (3) add the modal component + form state in `PriceGap.tsx`; (4) add `pricegap.candidates.*` i18n keys to both locales; (5) Vitest + Go test coverage for new validation paths and modal state. Delegate Go work and React work to separate teammates per `CLAUDE.local.md` delegation rules.

## Standard Stack

### Core (already in use, no installs)
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go stdlib `net/http` + `encoding/json` | go.mod 1.26+ | HTTP handler + JSON decode | Existing handler already uses this exact pattern |
| `github.com/redis/go-redis/v9` | per go.mod | Redis SET membership lookup for active-position safety check | `database.Client.GetActivePriceGapPositions()` already implemented |
| React | 19.2.0 (per `web/package.json`) | Frontend | Existing tab built on this |
| Tailwind 4.x via `@tailwindcss/vite` | 4.2.1 | Styling | Existing modal classes (`fixed inset-0 bg-black/50 z-50`) reuse same vocabulary |

### Supporting (already in use)
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| Vite 5.x | per package-lock | Bundler | `npm run build` (mandatory, see Build Order constraint) |
| Vitest + tsc -b | per package-lock | Frontend lint / typecheck | Component / form-state tests |

### Alternatives Considered
| Instead of | Could Use | Why we don't |
|------------|-----------|--------------|
| Reusing `POST /api/config` | A new `/api/pricegap/candidates` REST endpoint | D-16 explicitly locked the reuse path — matches `SpotFutures*` and `Risk*` field conventions |
| `<dialog>` HTML element | Existing fixed-overlay div | Phase 9 modals use the div pattern; introducing `<dialog>` would diverge for no benefit |
| React Hook Form / Formik | Hand-rolled `useState` per field | npm lockdown — no new deps. Existing form patterns in dashboard use plain `useState` |
| `react-portal` | Inline JSX | Not installed; Phase 9 modal mounts inline at the bottom of the page JSX and works fine |

**Installation:**
```bash
# NO new npm or Go packages. Phase 10 is pure code additions to existing modules.
# Verify build chain works:
cd web && npm ci   # ONLY if dependencies need refresh
cd web && npm run build
cd .. && go build
```

**Version verification:** Not applicable — no new packages.

## Architecture Patterns

### Recommended Project Structure (additions only)

```
internal/
├── api/
│   ├── handlers.go              # EDIT: extend priceGapUpdate{} + apply block + validate
│   └── pricegap_handlers.go     # OPTIONAL: shared validatePriceGapCandidate() helper here
├── config/
│   └── config.go                # NO CHANGE — Candidates already wired (line 1355-1357)
├── models/
│   └── pricegap_interfaces.go   # NO CHANGE — PriceGapCandidate struct already complete
└── pricegaptrader/
    └── tracker.go               # NO CHANGE if NewTracker() doesn't cache candidates;
                                 # otherwise add a re-read hook on configNotifier subscription

web/src/
├── pages/
│   └── PriceGap.tsx             # EDIT: add CandidateModal component + state + delete confirm
├── components/                  # OPTIONAL: extract CandidateModal into its own file
│   └── PriceGapCandidateModal.tsx
└── i18n/
    ├── en.ts                    # EDIT: add ~25 new pricegap.candidates.* keys
    └── zh-TW.ts                 # EDIT: same keys, translated
```

### Pattern 1: Pointer-Optional Config Update Struct

**What:** Every `/api/config` payload field is a pointer; nil means "untouched". Slice fields are the exception — empty slice means "no candidates", so use a pointer-to-slice (`*[]models.PriceGapCandidate`) to distinguish "not provided" from "provided empty".

**When to use:** Always for `/api/config` POST extensions.

**Example (mirror this exactly for `Candidates`):**
```go
// handlers.go ~line 783 — extend priceGapUpdate
type priceGapUpdate struct {
    Enabled    *bool                          `json:"enabled"`
    PaperMode  *bool                          `json:"paper_mode"`
    DebugLog   *bool                          `json:"debug_log"`
    Candidates *[]models.PriceGapCandidate    `json:"candidates"` // NEW
}

// handlers.go ~line 1589 — extend apply block
if pg := upd.PriceGap; pg != nil {
    if pg.Enabled != nil { /* … */ }
    if pg.PaperMode != nil { /* … */ }
    if pg.DebugLog != nil { /* … */ }
    if pg.Candidates != nil {                       // NEW
        if err := validateCandidates(*pg.Candidates); err != nil {
            s.cfg.Unlock()
            writeJSON(w, http.StatusBadRequest, Response{Error: err.Error()})
            return
        }
        if err := s.checkDeleteSafety(s.cfg.PriceGapCandidates, *pg.Candidates); err != nil {
            s.cfg.Unlock()
            writeJSON(w, http.StatusConflict, Response{Error: err.Error()})
            return
        }
        s.cfg.PriceGapCandidates = *pg.Candidates   // full replace per D-16
    }
}
```

### Pattern 2: Phase-9 Modal Pattern (mirror for Add/Edit)

**What:** A modal is a top-level `{condition && <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">...</div>}` block at the bottom of the JSX, with an inner card and an ESC handler in `useEffect`.

**Reference (verified at `PriceGap.tsx:982-1075`):**
```tsx
{/* Disable modal */}
{disableTarget && (
  <div
    className="fixed inset-0 bg-black/50 flex items-center justify-center z-50"
    onClick={() => {
      if (!modalBusy) { setDisableTarget(null); setModalError(null); }
    }}
  >
    <div className="bg-gray-800 ..." onClick={(e) => e.stopPropagation()}>
      ...
    </div>
  </div>
)}

// ESC handler at line 437-449:
useEffect(() => {
  if (!disableTarget && !reenableTarget) return;
  const h = (e: KeyboardEvent) => { if (e.key === 'Escape') ... };
  window.addEventListener('keydown', h);
  return () => window.removeEventListener('keydown', h);
}, [disableTarget, reenableTarget]);
```

### Pattern 3: postConfig helper (already implemented)

The `postConfig` callback at `PriceGap.tsx:315-325` is the right primitive for the modal save:
```tsx
const postConfig = useCallback(async (body: Record<string, unknown>): Promise<void> => {
  const res = await fetch('/api/config', { method: 'POST', headers: authHeaders(), body: JSON.stringify(body) });
  const payload = (await res.json().catch(() => null)) as { ok?: boolean; error?: string; data?: unknown } | null;
  if (!res.ok || !payload?.ok) throw new Error(payload?.error || `HTTP ${res.status}`);
}, []);

// Usage for save:
await postConfig({ price_gap: { candidates: nextCandidatesArray } });
```

The current helper discards `data`. Phase 10 should extend it (or add a sibling) to **return** the snapshot so the modal can re-render the table from the server's authoritative response (D-20).

### Anti-Patterns to Avoid

- **Don't auto-POST on mount.** PG-OPS-08 (deferred to Phase 13) tracks an existing 2026-04-25 incident where dashboard load auto-POSTed `/api/config`. Phase 10 modal must only POST in response to a user clicking Save / Confirm Delete. **Verification:** the planner should add an explicit task that uses DevTools Network tab to confirm zero POSTs on tab mount and modal open.
- **Don't decouple symbol from uppercase.** `PriceGapCandidate.ID()` is `Symbol + "_" + LongExch + "_" + ShortExch` — if the modal lets a user save lowercase symbol, the position ID drifts from the candidate ID and the tuple lookup fails. Auto-uppercase on input AND validate `^[A-Z0-9]+USDT$` server-side.
- **Don't trust array index.** D-12 says edit identifies by tuple, not index. Two reasons: (a) frontend may sort the table; (b) concurrent server-side mutations could shift indices. Always send the full replacement array; let server validate uniqueness.
- **Don't assume nested objects in i18n.** The codebase uses **flat dotted keys** (`'pricegap.candidates.modal.title'`), not nested objects. CONTEXT.md says "namespace `priceGap.candidates.*`" but this is conceptual — the actual prefix on disk is lowercase `pricegap.` (verified at `en.ts:766+`). Use `'pricegap.candidates.*'` exactly.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Config persistence | New file-write code | `s.cfg.SaveJSON()` (line 1489) | Already handles `.bak` backup, secret preservation, env var path override |
| Active position lookup | New Redis SCAN code | `s.db.GetActivePriceGapPositions()` | Already implemented; returns `[]*models.PriceGapPosition` with Symbol / LongExchange / ShortExchange fields |
| Candidate hot-reload | New refresh hook | Tracker reads `t.cfg.PriceGapCandidates` per tick | Pointer-shared `*Config`; mutation under `s.cfg.Lock()` is automatically visible (verify no copy in `NewTracker()`) |
| Modal portal | A custom Portal component | Inline `fixed inset-0 z-50` div | Established Phase-9 pattern at `PriceGap.tsx:982` |
| Form state | Install react-hook-form | Plain `useState` per field | npm lockdown; pattern works at the 6-field scale |
| Config-change notification | Custom subscription system | `s.configNotifier.Notify()` (line 1755) | Already invoked on every successful POST; engine + scanner subscribe |
| Response envelope | New error format | `writeJSON(w, status, Response{OK: true, Data: ...})` (line 730/734/1757) | Standard `{ok, data?, error?}` envelope |

**Key insight:** This phase is almost entirely "find the existing pattern and follow it". The risky moves are (a) the new validation logic, (b) the active-position safety check (which crosses the api↔database↔models boundary), and (c) i18n lockstep discipline.

## Runtime State Inventory

> Phase 10 is greenfield-on-existing-config — adds new editor UI to existing `PriceGapCandidates` slice that already round-trips to `config.json`. No data migration required. Inventory check below for completeness.

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | `pg:positions:active` Redis SET (members are position IDs of form `symbol_longExch_shortExch`); `config.json` already contains `price_gap.candidates: []` | Read-only check during delete — no migration |
| Live service config | `config.json` `price_gap.candidates` block (already present per Phase 8) | No structural change; just add CRUD UX over the existing slice |
| OS-registered state | None — no systemd / launchd registrations involve candidate identities | None |
| Secrets/env vars | None — candidate fields are not secrets | None |
| Build artifacts | `web/dist/` is rebuilt by `npm run build`; `arb` binary embeds it via `//go:embed all:dist` | Standard build order: frontend before Go binary |

**Nothing found in category:** OS-registered state, secrets, build-artifact-needs-regen — verified by grep across `internal/` and `web/`.

## Common Pitfalls

### Pitfall 1: Tracker startup-cache silently breaks hot-reload
**What goes wrong:** If `NewTracker(cfg)` ever copies `cfg.PriceGapCandidates` into a local field (`t.candidates = cfg.PriceGapCandidates`), the tick loop will iterate the stale snapshot, and Add/Edit/Delete will require a process restart to take effect.
**Why it happens:** Defensive coding; "don't share mutable state" instinct.
**How to avoid:** Verify with `grep -n 't.cfg.PriceGapCandidates\|t.candidates' internal/pricegaptrader/tracker.go` — must show only `t.cfg.PriceGapCandidates`. Currently CLEAN as of 2026-04-25 (3 read sites, all directly `t.cfg.PriceGapCandidates`).
**Warning signs:** Any field on the tracker struct that's a `[]models.PriceGapCandidate` or `[]string` of symbols.

### Pitfall 2: Empty array = delete all (vs not provided)
**What goes wrong:** If the modal sends `{ price_gap: { candidates: [] } }` to delete the last entry, and the field is `[]models.PriceGapCandidate` (not pointer-to-slice), Go's `len(slice) > 0` check at config.go line 1355 silently drops it — the candidates list never empties.
**Why it happens:** Existing config.go pattern: `if len(pg.Candidates) > 0 { c.PriceGapCandidates = pg.Candidates }` — this is "load from disk" semantics, deliberate. The handler path needs different semantics.
**How to avoid:** In `handlers.go priceGapUpdate`, declare `Candidates *[]models.PriceGapCandidate` (pointer-to-slice) and check `if pg.Candidates != nil` (not `len > 0`) — empty array is a valid intentional state.
**Warning signs:** `len(pg.Candidates) > 0` guard on the handler-side apply path.

### Pitfall 3: Symbol whitespace / case drift
**What goes wrong:** User types ` btcusdt ` (lowercase + spaces). Server stores it as-is. Tracker's `t.cfg.PriceGapCandidates[i].Symbol` doesn't match the uppercase symbol that the discovery path emits → no fires.
**How to avoid:** Sanitize on **both** sides — frontend auto-uppercase `.toUpperCase().trim()`, server validates `^[A-Z0-9]+USDT$` after `strings.TrimSpace + strings.ToUpper`. Server fix prevents API clients from polluting state.
**Warning signs:** Test with leading/trailing spaces and lowercase to confirm.

### Pitfall 4: Tuple uniqueness vs replacement edits
**What goes wrong:** Edit changes the tuple from `(BTCUSDT, binance, bybit)` to `(BTCUSDT, gateio, bybit)`. The full-replacement payload contains the new tuple. The validator says "no duplicates", which it isn't (the original is gone in the replacement). But the `pg:positions:active` set still has a position keyed `BTCUSDT_binance_bybit` — and the tracker now considers it orphaned.
**How to avoid:** Treat tuple-changing edit as logically a delete-of-old + add-of-new. Apply the active-position safety check **against the OLD tuple's removal**, not just naive symbol matching. Practical implementation: compare old vs new arrays, find tuples present in old but not new ("removals"), reject if any removal has an active position.
**Warning signs:** Tuple edit feature accepts changes while leaving orphaned positions in `pg:positions:active`.

### Pitfall 5: i18n typecheck-passes-but-shows-English
**What goes wrong:** Adding `'pricegap.candidates.modal.title': 'Add Candidate'` to en.ts only — `TranslationKey` is `keyof typeof en`, so calls to `t('pricegap.candidates.modal.title')` typecheck. zh-TW renders English at runtime.
**How to avoid:** Add an explicit task: `diff <(grep -oP "'[a-z][a-zA-Z0-9_.]+'" en.ts | sort -u) <(grep -oP "'[a-z][a-zA-Z0-9_.]+'" zh-TW.ts | sort -u)` — must be empty. Currently EN has 203 pricegap keys, zh-TW has 67 — drift exists already; do not let Phase 10 widen it.
**Warning signs:** zh-TW key count significantly less than en.ts.

### Pitfall 6: Phase-13 auto-POST regression
**What goes wrong:** Modal mount, focus, or visibility-change handlers accidentally trigger a POST. Already happened once (incident logged 2026-04-25 07:47, deferred to PG-OPS-08 / Phase 13).
**How to avoid:** Modal must POST **only** from explicit Save / Confirm Delete onClick handlers. Add a verification task to capture DevTools Network during: tab mount → Add modal open → Add modal cancel → Edit modal open → Edit modal cancel → Delete confirm dismiss. Expected POST count: 0 across all of these.
**Warning signs:** `useEffect` that calls postConfig without an explicit user-action gate.

### Pitfall 7: Concurrent edits — last-write-wins
**What goes wrong:** Two browser tabs open the modal, both edit candidate `BTCUSDT`, both save. Last save wins, first save's changes silently lost. CONTEXT D-20 acknowledges this is acceptable for v2.1 (single operator).
**How to avoid:** Document the limitation in user-facing copy ("changes from other tabs may be overwritten"). No code mitigation in Phase 10 — explicit v3.0 territory.
**Warning signs:** Any UAT scenario involving multiple browsers / tabs.

## Code Examples

Verified patterns from the existing codebase:

### Backend — extending `priceGapUpdate` and apply block
```go
// File: internal/api/handlers.go ~line 783 (extend struct)
type priceGapUpdate struct {
    Enabled    *bool                       `json:"enabled"`
    PaperMode  *bool                       `json:"paper_mode"`
    DebugLog   *bool                       `json:"debug_log"`
    Candidates *[]models.PriceGapCandidate `json:"candidates"` // Phase 10
}

// ~line 1589 (extend apply block — sketch)
if pg := upd.PriceGap; pg != nil {
    // ... existing Enabled/PaperMode/DebugLog branches ...
    if pg.Candidates != nil {
        next := *pg.Candidates
        if errs := validatePriceGapCandidates(next); len(errs) > 0 {
            s.cfg.Unlock()
            writeJSON(w, http.StatusBadRequest, Response{Error: strings.Join(errs, "; ")})
            return
        }
        if blocker, err := s.guardActivePositionRemoval(s.cfg.PriceGapCandidates, next); err != nil {
            s.cfg.Unlock()
            writeJSON(w, http.StatusInternalServerError, Response{Error: "active-position check: " + err.Error()})
            return
        } else if blocker != "" {
            s.cfg.Unlock()
            writeJSON(w, http.StatusConflict, Response{Error: blocker})
            return
        }
        s.cfg.PriceGapCandidates = next
    }
}
```

### Backend — validation helper (new function in pricegap_handlers.go)
```go
// validatePriceGapCandidates enforces D-05..D-10 invariants.
// Returns one error string per failed candidate, indexed for clarity.
var symbolRE = regexp.MustCompile(`^[A-Z0-9]+USDT$`)
var allowedExch = map[string]struct{}{
    "binance": {}, "bybit": {}, "gateio": {}, "bitget": {}, "okx": {}, "bingx": {},
}

func validatePriceGapCandidates(cs []models.PriceGapCandidate) []string {
    var errs []string
    seen := make(map[string]struct{}, len(cs))
    for i, c := range cs {
        prefix := fmt.Sprintf("candidate[%d] %s/%s/%s", i, c.Symbol, c.LongExch, c.ShortExch)
        sym := strings.TrimSpace(strings.ToUpper(c.Symbol))
        if !symbolRE.MatchString(sym) {
            errs = append(errs, prefix+": symbol must match ^[A-Z0-9]+USDT$")
        }
        if _, ok := allowedExch[c.LongExch]; !ok {
            errs = append(errs, prefix+": long_exch invalid")
        }
        if _, ok := allowedExch[c.ShortExch]; !ok {
            errs = append(errs, prefix+": short_exch invalid")
        }
        if c.LongExch == c.ShortExch {
            errs = append(errs, prefix+": long_exch must differ from short_exch")
        }
        if c.ThresholdBps < 50 || c.ThresholdBps > 1000 {
            errs = append(errs, prefix+": threshold_bps out of range [50,1000]")
        }
        if c.MaxPositionUSDT < 100 || c.MaxPositionUSDT > 50000 {
            errs = append(errs, prefix+": max_position_usdt out of range [100,50000]")
        }
        if c.ModeledSlippageBps < 0 || c.ModeledSlippageBps > 100 {
            errs = append(errs, prefix+": modeled_slippage_bps out of range [0,100]")
        }
        key := sym + "|" + c.LongExch + "|" + c.ShortExch
        if _, dup := seen[key]; dup {
            errs = append(errs, prefix+": duplicate tuple")
        }
        seen[key] = struct{}{}
    }
    return errs
}
```

### Backend — active-position safety check (active set membership lookup)
```go
// guardActivePositionRemoval blocks removal of any candidate whose tuple has
// an open position. Returns "" when safe.
func (s *Server) guardActivePositionRemoval(prev, next []models.PriceGapCandidate) (string, error) {
    nextSet := make(map[string]struct{}, len(next))
    for _, c := range next {
        nextSet[c.Symbol+"|"+c.LongExch+"|"+c.ShortExch] = struct{}{}
    }

    // Find removed tuples (present in prev but not next).
    var removed []models.PriceGapCandidate
    for _, c := range prev {
        if _, kept := nextSet[c.Symbol+"|"+c.LongExch+"|"+c.ShortExch]; !kept {
            removed = append(removed, c)
        }
    }
    if len(removed) == 0 {
        return "", nil
    }

    if s.db == nil {
        return "", nil // tests without a DB — skip
    }
    active, err := s.db.GetActivePriceGapPositions()
    if err != nil {
        return "", err
    }
    for _, p := range active {
        for _, rm := range removed {
            if p.Symbol == rm.Symbol && p.LongExchange == rm.LongExch && p.ShortExchange == rm.ShortExch {
                return fmt.Sprintf("candidate %s/%s/%s has active position %s; close it first",
                    rm.Symbol, rm.LongExch, rm.ShortExch, p.ID), nil
            }
        }
    }
    return "", nil
}
```

### Frontend — modal scaffold (extend `PriceGap.tsx`)
```tsx
// State (add near line 232):
type ModalMode = 'add' | 'edit';
const [editorOpen, setEditorOpen] = useState<{ mode: ModalMode; target?: PriceGapCandidate } | null>(null);
const [deleteTarget, setDeleteTarget] = useState<PriceGapCandidate | null>(null);
const [formSymbol, setFormSymbol] = useState('');
const [formLongExch, setFormLongExch] = useState('');
const [formShortExch, setFormShortExch] = useState('');
const [formThresholdBps, setFormThresholdBps] = useState(200);
const [formMaxPositionUSDT, setFormMaxPositionUSDT] = useState(5000);
const [formModeledSlippageBps, setFormModeledSlippageBps] = useState(5);
const [formErrors, setFormErrors] = useState<Record<string, string>>({});

// Save handler:
const handleSave = useCallback(async () => {
  setModalBusy(true);
  setModalError(null);
  setFormErrors({});
  // Local validation mirrors server (D-03: errors after first submit).
  const errs = validateLocal({ symbol: formSymbol.trim().toUpperCase(), /* ... */ });
  if (Object.keys(errs).length > 0) {
    setFormErrors(errs);
    setModalBusy(false);
    return;
  }
  // Build full replacement candidate array.
  const next: PriceGapCandidate[] = (() => {
    const draft: PriceGapCandidate = {
      symbol: formSymbol.trim().toUpperCase(),
      long_exch: formLongExch,
      short_exch: formShortExch,
      threshold_bps: formThresholdBps,
      max_position_usdt: formMaxPositionUSDT,
      modeled_slippage_bps: formModeledSlippageBps,
    };
    if (editorOpen?.mode === 'add') return [...candidates, draft];
    // Edit: replace by old tuple identity.
    return candidates.map((c) =>
      c.symbol === editorOpen!.target!.symbol &&
      c.long_exch === editorOpen!.target!.long_exch &&
      c.short_exch === editorOpen!.target!.short_exch
        ? draft
        : c
    );
  })();
  try {
    await postConfig({ price_gap: { candidates: next } });
    // Re-seed from /api/pricegap/state to pick up server-canonical view (incl. enabled flags)
    await seed();
    setEditorOpen(null);
  } catch (err) {
    setModalError(err instanceof Error ? err.message : String(err));
  } finally {
    setModalBusy(false);
  }
}, [/* deps */]);
```

### i18n — flat-key additions
```ts
// web/src/i18n/en.ts (insert with other pricegap.* keys)
'pricegap.candidates.add.button': 'Add candidate',
'pricegap.candidates.modal.add.title': 'Add candidate',
'pricegap.candidates.modal.edit.title': 'Edit {symbol} ({long}/{short})',
'pricegap.candidates.modal.symbol.label': 'Symbol',
'pricegap.candidates.modal.symbol.placeholder': 'BTCUSDT',
'pricegap.candidates.modal.longExch.label': 'Long exchange',
'pricegap.candidates.modal.shortExch.label': 'Short exchange',
'pricegap.candidates.modal.thresholdBps.label': 'Threshold (bps)',
'pricegap.candidates.modal.maxPositionUSDT.label': 'Max position (USDT)',
'pricegap.candidates.modal.modeledSlippageBps.label': 'Modeled slippage (bps)',
'pricegap.candidates.modal.save': 'Save',
'pricegap.candidates.modal.cancel': 'Cancel',
'pricegap.candidates.errors.symbolFormat': 'Symbol must be uppercase, end with USDT',
'pricegap.candidates.errors.exchangesEqual': 'Long and short exchanges must differ',
'pricegap.candidates.errors.thresholdRange': 'Threshold must be 50–1000 bps',
'pricegap.candidates.errors.maxPositionRange': 'Max position must be 100–50000 USDT',
'pricegap.candidates.errors.slippageRange': 'Modeled slippage must be 0–100 bps',
'pricegap.candidates.errors.tupleCollision': 'Candidate with this symbol+exchanges already exists',
'pricegap.candidates.errors.activePositionBlocksDelete': 'Cannot delete: candidate has an active position. Close it first.',
'pricegap.candidates.confirmDelete.title': 'Delete candidate?',
'pricegap.candidates.confirmDelete.body': '{symbol}: {long} ↔ {short}, threshold {bps} bps. This cannot be undone.',
'pricegap.candidates.confirmDelete.confirm': 'Delete',
'pricegap.candidates.confirmDelete.cancel': 'Cancel',
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Hand-edit `config.json`, restart binary | Dashboard CRUD via `POST /api/config` + hot-reload via `s.configNotifier.Notify()` | Phase 8 (config persist) → Phase 9 (paper-mode toggle, Disable/Re-enable buttons) → Phase 10 (this) | No restart needed |
| pg-admin CLI for any candidate change | pg-admin remains read-only on candidate **content**; dashboard handles add/edit/delete | Phase 9 (status/enable/disable in CLI) → Phase 10 (CRUD in dashboard) | Operators have one place to add/remove pairs |
| Redis HSET for live config persistence | `config.json` is single source of truth; Redis HSET removed for config | 2026-04-05 | Persistence flow simplified; CLAUDE.local.md `## CRITICAL: Do not modify config.json` is the binding constraint |

**Deprecated/outdated:**
- Treating `t.cfg.PriceGapCandidates` as immutable after `NewTracker(cfg)` — was never true; tracker reads per tick.
- WS broadcast on candidate change — explicitly out of scope per D-20; do not add for v2.1.

## Assumptions Log

> Claims tagged `[ASSUMED]` need user confirmation before execution.

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Tracker `NewTracker()` does not copy `cfg.PriceGapCandidates` into a private cache | Pattern 1, Pitfall 1 | Hot-reload silently broken; needs `Notify()`-driven refresh hook (verified read-only via 3-site grep, but constructor not exhaustively read) |
| A2 | `postConfig` helper's response `data` payload contains the full `priceGapStateResponse`-shaped snapshot | Pattern 3, frontend save flow | If `handlePostConfig` returns only `configResponse` (likely — see line 1757 `s.buildConfigResponse()`), the modal must still call `seed()` after save instead of using the response directly. Code example above defensively does this |
| A3 | `pricegap.candidates.*` flat-key namespace is acceptable to the project (CONTEXT D-21 says "namespace `priceGap.candidates.*`" but actual i18n uses lowercase `pricegap.`) | Pattern 4, i18n | Cosmetic — easy to flip to either, but planner should pick one and stick to it |
| A4 | Server-side validation of `symbol == nil after trim` is OK to express via the regex check (empty string fails the regex) | Code example | Negligible |
| A5 | Active-position lookup uses `GetActivePriceGapPositions()` reading both Redis and the position struct's `LongExchange`/`ShortExchange` fields; field names are `LongExchange` (full word) per `pricegap_position.go:35-36` | Code example, active-position guard | If field names are different, the guard fails compile-time — caught in build, not runtime |

## Open Questions

1. **Should the planner add a tracker `Notify()` subscription as a defensive measure?**
   - What we know: Tracker reads `t.cfg.PriceGapCandidates` directly per tick. `s.configNotifier.Notify()` already fires on every successful POST. No code currently subscribes to it from the tracker.
   - What's unclear: Whether subscribing buys anything beyond what per-tick reads already provide. Probably no — per-tick is finer-grained than Notify() events.
   - Recommendation: **Don't add** unless investigation finds a copy-cache. Document the per-tick read invariant in a code comment so future refactors don't break it.

2. **Should the modal allow editing the tuple, or only the four numeric/text fields?**
   - What we know: D-12 says tuple-changing edits are allowed, with collision check.
   - What's unclear: UX subtlety — re-rendering an "edit modal" with editable symbol/exchanges feels close to "add modal" with pre-fill. Operationally, editing a tuple is a destructive op (existing position binding becomes orphaned per Pitfall 4).
   - Recommendation: Allow it (D-12 locked) but disable the symbol/long/short fields when there's an active position for the original tuple, and surface a tooltip explaining why. Otherwise treat as full edit.

3. **Should there be a per-modal "test" button that validates server-side without committing?**
   - What we know: D-03 says on-submit validation. CONTEXT explicitly defers "Per-candidate test fire button".
   - Recommendation: Skip — D-03 + inline error messages are sufficient.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain (1.26+) | Backend build | ✓ (per go.mod) | 1.26+ | None — install.sh provisions 1.22 (drift), see CLAUDE.local.md `### Known drift` |
| Node 22.13.0 (nvm) | `npm run build` | ✓ (per memory) | v22.13.0 | None — required for Vite 5 |
| Redis (DB 2) | Active-position lookup | ✓ (live system) | per CLAUDE.local.md | None — required for delete safety |
| `npm ci` (locked dependencies only) | Frontend dependency tree | ✓ | per package-lock.json | None — npm lockdown forbids `npm install` |

**Missing dependencies with no fallback:** None for development. Production: Go 1.22 on fresh installs (CLAUDE.local.md known drift).

**Missing dependencies with fallback:** None.

## Validation Architecture

> Phase 10 touches: backend handler validation, frontend modal state, integration round-trip from modal → /api/config → config.json → tracker hot-reload.

### Test Framework
| Property | Value |
|----------|-------|
| Backend framework | Go stdlib `testing` (per go.mod, existing tests at `config_handlers_test.go`, `pricegap_handlers_test.go`) |
| Backend config file | none (stdlib `testing`) |
| Backend quick run | `go test ./internal/api/... -run TestPostConfigPriceGapCandidates -v` |
| Backend full suite | `go test ./...` |
| Frontend framework | Vitest (devDependency in package.json — verify) + tsc -b |
| Frontend config file | `web/vite.config.ts` (typically extends test block) |
| Frontend quick run | `cd web && npx vitest run --reporter=verbose src/pages/PriceGap.test.tsx` (file to be created in Wave 0) |
| Frontend full suite | `cd web && npm run build && npx vitest run` |
| Manual / E2E | Playwright via `.claude/skills/playwright-skill` (per skill inventory) — only for the final round-trip |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| PG-OPS-07 | POST /api/config with `pricegap.candidates` updates `s.cfg.PriceGapCandidates` and persists via SaveJSON | unit | `go test ./internal/api/ -run TestPostConfigPriceGapCandidates_Add -v` | ❌ Wave 0 |
| PG-OPS-07 | POST validation rejects bad symbol / out-of-range / equal exchanges / duplicate tuple | unit | `go test ./internal/api/ -run TestPostConfigPriceGapCandidates_Validation -v` | ❌ Wave 0 |
| PG-OPS-07 | DELETE attempt blocked when active position exists | integration | `go test ./internal/api/ -run TestPostConfigPriceGapCandidates_BlocksActiveDelete -v` | ❌ Wave 0 |
| PG-OPS-07 | Tracker re-reads `t.cfg.PriceGapCandidates` per tick (no startup cache) | unit | `go test ./internal/pricegaptrader/ -run TestTracker_HotReloadCandidates -v` | ❌ Wave 0 |
| PG-OPS-07 | Frontend modal opens, validates locally, posts on Save, closes on Cancel | unit (Vitest + RTL) | `cd web && npx vitest run src/pages/PriceGapCandidateModal.test.tsx` | ❌ Wave 0 |
| PG-OPS-07 | Modal does NOT auto-POST on mount (PG-OPS-08 incident regression) | integration | Manual DevTools Network capture per Pitfall 6, plus a Vitest spy on `fetch` | ❌ Wave 0 |
| PG-OPS-07 | EN/zh-TW key parity for `pricegap.candidates.*` namespace | lint | `node web/scripts/check-i18n-parity.cjs` (script to be created) or hand-script in test | ❌ Wave 0 |
| PG-OPS-07 | Full round-trip: open modal → save → page reload → candidate persists | E2E (manual or Playwright) | Optional Playwright suite at `e2e/pricegap-crud.spec.ts` | ❌ Wave 0; manual acceptable |

### Sampling Rate
- **Per task commit:** `go test ./internal/api/... -run TestPostConfigPriceGapCandidates -v && cd web && npx vitest run src/pages/PriceGap.test.tsx`
- **Per wave merge:** `go test ./... && cd web && npm run build`
- **Phase gate:** Full suite green + manual E2E (Add → Edit → Delete + active-position-blocks-delete) before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] `internal/api/pricegap_candidates_handler_test.go` — covers all PG-OPS-07 backend behaviors above
- [ ] `internal/pricegaptrader/tracker_hotreload_test.go` — pinning the per-tick read invariant against future refactors
- [ ] `web/src/pages/PriceGapCandidateModal.test.tsx` — Vitest + React Testing Library
- [ ] `web/src/pages/PriceGap.candidates.test.tsx` — integration: modal open + save + table re-render
- [ ] `web/scripts/check-i18n-parity.cjs` — diff key sets between en.ts and zh-TW.ts (or include in Vitest)
- [ ] Optional Playwright spec in `.claude/skills/playwright-skill` workspace if planner judges manual UAT insufficient

## Sources

### Primary (HIGH confidence)
- `internal/api/handlers.go:730-1758` — `handleGetConfig`, `handlePostConfig`, `priceGapUpdate` struct (verified directly)
- `internal/api/server.go:84,214` — `/api/config` mux registration + dispatch
- `internal/api/pricegap_handlers.go:85-126` — `handlePriceGapState` precedent for shared state response
- `internal/config/config.go:282,321,1355,1487-1495` — `PriceGapCandidates` slice + JSON tag + load + SaveJSON
- `internal/models/pricegap_interfaces.go:5-20` — `PriceGapCandidate` struct + `ID()` method
- `internal/database/pricegap_state.go:18-117` — `pg:positions:active` SET + `GetActivePriceGapPositions()`
- `internal/pricegaptrader/tracker.go:226,293,413` — three direct `t.cfg.PriceGapCandidates` read sites
- `web/src/pages/PriceGap.tsx:220-449,982-1075` — modal state pattern + `postConfig` helper + Phase 9 modals
- `web/src/i18n/en.ts:766+, 841` — pricegap key namespace + `TranslationKey` type derivation
- `web/embed.go:6` — `//go:embed all:dist`
- `.planning/REQUIREMENTS.md:13-15` — PG-OPS-07 acceptance criteria (verbatim)
- `.planning/phases/10-dashboard-candidate-crud/10-CONTEXT.md` — 21 locked decisions

### Secondary (MEDIUM confidence)
- `web/package.json` — React 19.2.0 + Tailwind 4.x + Vite 5.x (no Vitest visible in dependencies snippet — confirm during planning)

### Tertiary (LOW confidence)
- None — phase is entirely codebase-internal; no external library research needed

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all components are pre-existing in repo
- Architecture (apply-block extension, modal scaffold, validation, active-position guard): HIGH — patterns directly verified in existing handlers / Phase 9 modals
- Pitfalls: HIGH — each pitfall has an explicit codepath citation
- Tracker hot-reload: HIGH (no startup cache observed via grep) but with one assumption (A1) about `NewTracker()` constructor body which the planner should verify by reading the constructor

**Research date:** 2026-04-25
**Valid until:** ~2026-05-25 (codebase moves fast; re-verify after any /api/config or pricegap-handler changes land)
