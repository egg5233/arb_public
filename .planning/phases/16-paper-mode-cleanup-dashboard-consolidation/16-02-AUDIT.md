# Phase 16 Plan 02 — PG-FIX-02 Audit (D-05)

## Reproduction status

- Bug observed: yes — operator UAT for v2.0 dashboard reported `paper_mode` flipping to `false` on Price-Gap tab page load (PG-FIX-02 ticket).
- Reproduces in current code: **NO** under static + grep audit (current build, post-Phase 15 v0.38.1, frontend tree as of 2026-05-02). Grep confirms only ONE `postConfig` call site touches `price_gap_paper_mode` — the operator-driven `togglePaper` callback at `web/src/pages/PriceGap.tsx:368`. No `useEffect` posts config from any tab. HAR evidence (Task 1.5 checkpoint, audit-doc-only fallback per CONTEXT D-09 if browser unavailable) finalises the runtime confirmation.

D-05 protocol: server-side guard (Task 2) ships regardless — establishing the negative is required.

## Frontend audit

### postConfig call sites (from grep)

Source: `uat-evidence/audit-grep/16-02-postconfig.txt`

```
web/src/pages/PriceGap.tsx:337:  const postConfig = useCallback(async (body: Record<string, unknown>): Promise<void> => {
web/src/pages/PriceGap.tsx:354:      await postConfig({ price_gap: { enabled: next } });
web/src/pages/PriceGap.tsx:361:  }, [enabled, postConfig, t]);
web/src/pages/PriceGap.tsx:368:      await postConfig({ price_gap_paper_mode: next });
web/src/pages/PriceGap.tsx:375:  }, [paperMode, postConfig, t]);
web/src/pages/PriceGap.tsx:386:      await postConfig({ price_gap: { debug_log: next } });
web/src/pages/PriceGap.tsx:393:  }, [debugLog, postConfig, t]);
web/src/pages/PriceGap.tsx:569:      await postConfig({ price_gap: { candidates: next } });
web/src/pages/PriceGap.tsx:578:  }, [candidates, editorOpen, formSymbol, formLongExch, formShortExch, formThresholdBps, formMaxPositionUSDT, formModeledSlippageBps, formDirection, postConfig, seed, validateLocalForm]);
web/src/pages/PriceGap.tsx:595:      await postConfig({ price_gap: { candidates: next } });
web/src/pages/PriceGap.tsx:609:  }, [candidates, deleteTarget, postConfig, seed, t]);
```

Single `postConfig` definition. Five operator-driven invocation sites — all gated by user clicks (toggleEnabled, togglePaper, toggleDebug, save-candidate, delete-candidate). Search hits in `PriceGap.candidates.test.ts` are static-test references, not runtime call sites.

### useEffect / useLayoutEffect that touch config (from grep)

Source: `uat-evidence/audit-grep/16-02-effects.txt` (full 69-line capture).

Filtered for any `useEffect`/`useLayoutEffect` that references `postConfig`, `price_gap_paper_mode`, or POSTs to `/api/config`:

- `web/src/pages/PriceGap.tsx`: 3 `useEffect` hooks. Bodies inspected at:
  - line ~165 — opens WebSocket (`ws.onopen`, etc.); GET-only.
  - line ~325 (approx) — `void seed();` calls GET-only `seed` callback.
  - line ~430+ — Esc-key handler (`if (e.key === 'Escape') setEditorOpen(null) ...`); state-only setters, no `postConfig`.
- `web/src/pages/Config.tsx`: 3 `useEffect` hooks — outside-click dropdown closer, focus-aware local-val mirror, `getConfig()` GET fetch on mount. None POST.
- All other matches are component-internal effects (Tooltips, dropdowns, click handlers) that do not call `postConfig` or `fetch /api/config POST`.

**Result: ZERO useEffect call sites POST to /api/config.**

### Verdict

- Legitimate write surfaces: `PriceGap.tsx:368` (`togglePaper` operator click) — the one and only paper_mode write site.
- Suspect write surfaces: NONE found in current code. Plan 04 will rewire `togglePaper` to send `operator_action: true`; until then this single site will be 409-rejected by the new server guard (CHANGELOG documents `pg-admin` workaround).

If the v2.0 UAT auto-POST regression existed, it has either been removed already (a prior plan touched the surface) or it lives in a path the static grep missed (e.g., a third-party library that auto-POSTs cached state). HAR evidence will close that uncertainty; D-05 protocol mandates the server guard ships either way.

## Backend audit

### handlers.go config POST handler chain

Source: `uat-evidence/audit-grep/16-02-paper-backend.txt`

| File | Line | Reference |
|------|------|-----------|
| `internal/api/handlers.go` | 783 | `PriceGapPaperMode *bool \`json:"price_gap_paper_mode"\`` — flat shortcut field on `configUpdate` |
| `internal/api/handlers.go` | 795 | `PaperMode *bool \`json:"paper_mode"\`` — nested field on `priceGapUpdate` |
| `internal/api/handlers.go` | 1602–1603 | Apply-comment block for "nested or flat shortcut" |
| `internal/api/handlers.go` | 1617–1619 | `if pg.PaperMode != nil { s.cfg.PriceGapPaperMode = *pg.PaperMode }` — nested write path |
| `internal/api/handlers.go` | 1666–1668 | `if upd.PriceGapPaperMode != nil && (upd.PriceGap == nil || upd.PriceGap.PaperMode == nil) { s.cfg.PriceGapPaperMode = *upd.PriceGapPaperMode }` — flat write path |

**Lock pattern:** `handlePostConfig` at line 995 takes `s.cfg.Lock()` at line 1003 (manual, no `defer`). Error returns call `s.cfg.Unlock()` then `writeJSON` then `return` (mirrors the pattern at lines 1637, 1652, 1656). The new `paperModeRequested` guard MUST follow this same manual-unlock-and-return pattern.

**Reachable-block invariant:** Both write sites (1617–1619 nested + 1666–1668 flat) are inside the same locked region. The guard will be inserted BEFORE both, immediately after `upd.DryRun` apply (line 1004) but ideally just above the price-gap block (line 1601) so the rejection comment co-locates with the apply block — `grep -A 5 'paperModeRequested' | grep http.StatusConflict` will pass on a single contiguous block.

### IsPaperModeActive read chokepoint (Phase 15 D-07) — preservation check

Source: `uat-evidence/audit-grep/16-02-ispaperactive.txt` (39 matches across `internal/`).

Engine-side reads ALL go through `Tracker.IsPaperModeActive(ctx)`:

- `internal/pricegaptrader/execution.go` — 4 call sites (entry-order-placement, entry-synth-fill, entry-bingx-guard, close-leg-paper). All invoke `t.IsPaperModeActive(context.TODO())`.
- `internal/pricegaptrader/breaker_controller.go` — 2 references (chokepoint comment + chokepoint definition).
- `internal/pricegaptrader/tracker.go` — definition at line 261, signature at line 283.
- `internal/pricegaptrader/breaker_static_test.go` — static guard at line 43: forbids any direct `cfg.PriceGapPaperMode` read in production code; "must use Tracker.IsPaperModeActive(ctx) (Phase 15 D-07 chokepoint)".
- Test files: 7 helper tests in `breaker_controller_test.go` exercise sticky/cfg/expired/error paths.

**Engine-side direct reads of `cfg.PriceGapPaperMode`:** ZERO in production code outside the chokepoint definition. The static test in `breaker_static_test.go` enforces the rule (forbidden-token regex). Phase 15 D-07 chokepoint preserved by Plan 16-02 — no engine code path is changed.

The TWO API-layer reads (`internal/api/pricegap_handlers.go:177` and `internal/api/handlers.go` config GET via `buildConfigResponse`) are **dashboard hydration reads**, not engine reads. They remain — the dashboard needs to display the current state. The Pitfall-1 regression (GET that mutates) is closed by Test 5 (`TestConfigGet_DoesNotMutate_PaperMode`).

## HAR evidence

- File: `.planning/phases/16-paper-mode-cleanup-dashboard-consolidation/uat-evidence/page-load.har`
- Captured: pending — operator captures via DevTools (Task 1.5 checkpoint). Audit-doc-only fallback per CONTEXT D-09 acceptable if browser unavailable.
- Findings: `<filled in by operator after HAR capture>`

## Conclusion

- Reproduce: NO under current code (static grep + handler-chain audit). Pending HAR confirmation at Task 1.5 checkpoint.
- Root cause (if reproduced): To be filled by HAR analysis. Most likely candidates if HAR shows a stray POST: (a) a third-party library auto-replaying cached config; (b) a regressed hydration path that grep didn't catch (unlikely given the single write site found).
- Defense: server-side guard at `internal/api/handlers.go` (Task 2) is the chokepoint. The guard sits in the same locked block as both nested + flat write paths and rejects with HTTP 409 unless `operator_action: true` is on the request body. Plan 04 (Wave 2, depends_on: [16-02]) ships the `togglePaper` wire-up. Both ship even if reproduction failed (D-05: "establishing the negative is required").
- Phase 9 `pos.Mode` per-position immutability: untouched (engine path unchanged).
- Phase 15 `IsPaperModeActive` sticky-flag chokepoint: untouched (engine reads still go through Tracker chokepoint; static test in `breaker_static_test.go` enforces).
