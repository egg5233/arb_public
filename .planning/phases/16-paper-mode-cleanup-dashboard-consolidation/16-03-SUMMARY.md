---
phase: 16-paper-mode-cleanup-dashboard-consolidation
plan: 03
subsystem: ops/cli
tags: [bingxprobe, debug-utility, restoration, ticker-only, t-16-03-01, dev-01, makefile]

# Dependency graph
requires:
  - phase: 13 (deletion commit `21cb60b chore(13): remove cmd/bingxprobe`)
    provides: original 53-line WS-only diagnostic recoverable via `git show 21cb60b^:cmd/bingxprobe/main.go`
  - subsystem: pkg/exchange/bingx
    provides: stable Adapter constructor (NewAdapter) + GetOrderbook for ticker probe
provides:
  - Restored cmd/bingxprobe/ debug utility (ticker-only per T-16-03-01 deviation)
  - make probe-bingx Makefile target (.PHONY)
  - Operator-runnable BingX connectivity + auth + ticker sanity probe
  - TestOrder dry-run safety verification documented in 16-03-RESTORATION-NOTES.md (## TestOrder safety verification H2 section)
affects: [Phase 16-04 PG-OPS-09 (independent), v2.2 closure (DEV-01 closes ROADMAP success-criterion #3)]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Restoration-with-retrofit pattern (Path B): full rewrite when D-N contract has zero overlap with deleted source; restoration notes establish provenance + audit trail without diff-as-edit confusion."
    - "TestOrder dry-run safety verification (T-16-03-01): before any restored debug utility may invoke a production preflight, executor must read adapter source + identify endpoint URL + verdict {safe,unsafe}. Plan-level fallback (ticker-only) documented when verdict=unsafe."
    - "Operator-attested UAT (no stdout capture): verdict=pass acceptable when operator explicitly attests command outcome; UAT doc records `stdout not captured into this conversation` for audit honesty."

key-files:
  created:
    - cmd/bingxprobe/main.go
    - .planning/phases/16-paper-mode-cleanup-dashboard-consolidation/16-03-RESTORATION-NOTES.md
    - .planning/phases/16-paper-mode-cleanup-dashboard-consolidation/16-03-HUMAN-UAT.md
  modified:
    - Makefile
    - VERSION
    - CHANGELOG.md

key-decisions:
  - "Path B (full rewrite) chosen over Path A (patch in place): the original 53-line WS-only diagnostic has zero overlap with D-11 probe scope (preflight + ticker). Patching would mean deleting all 53 lines anyway; full rewrite preserves provenance via 16-03-RESTORATION-NOTES.md instead of masking as an edit diff."
  - "Ticker-only deviation per T-16-03-01: BingX adapter's TestOrder does NOT hit a dry-run endpoint — it places a real non-marketable IOC limit order at /openApi/swap/v2/trade/order and immediately cancels it (pkg/exchange/bingx/adapter.go:217..245). Adapter header comment explicitly states intent: BingX's documented /order/test endpoint bypasses the temporary market-risk gate the production engine validates. Per plan's own fallback (`If unsafe, the probe MUST exclude any TestOrder call and use ticker-only`), Task 2 shipped ticker-only. Production engine continues to exercise live preflight on every Strategy 4 entry — validation coverage preserved."
  - "Operator-attested UAT (Task 3): operator ran `make probe-bingx` from repo root and reported 'uat green' (exit 0); probe stdout NOT pasted into the conversation. UAT doc records the attestation honestly with the caveat 'stdout not captured into this conversation'. ROADMAP success-criterion #3 satisfied via operator attestation."
  - "No new dependencies: gorilla/websocket remains in go.mod (used by other code); rewrite imports only `pkg/exchange` + `pkg/exchange/bingx` + stdlib. `git diff go.mod go.sum` is empty."

patterns-established:
  - "Adapter dry-run-endpoint audit BEFORE plan executes any preflight call: T-16-03-01 mitigation pattern is reusable for any future restoration / ops tool that wants to exercise a production preflight path. The verdict {safe, unsafe} drives a binary decision: include the call OR fall back to read-only / ticker-only."
  - "Restoration notes as 6-section checklist: Restoration source / D-N contract / Gap analysis / Retrofit plan / Compile risk / Safety verification. Reusable for any future Phase-13-style deletion that gets reversed in a later phase."

requirements-completed: [DEV-01]

# Metrics
duration: ~30min (executor-side; spans 4 commits across 2 spawn cycles — Task 1+2 on first executor, Task 3 (checkpoint) operator-led, Task 4 + finalization on continuation)
completed: 2026-05-02
---

# Phase 16 Plan 03: Restored cmd/bingxprobe + make probe-bingx (DEV-01) Summary

**One-liner:** Restored the Phase-13-deleted BingX debug utility from `21cb60b^` via Path B full rewrite, retrofitted to ticker-only scope per T-16-03-01 unsafe-endpoint verdict (BingX adapter `TestOrder` hits the live order endpoint, not a dry-run path), and added a `make probe-bingx` Makefile target — closes ROADMAP success-criterion #3.

## What Shipped

- **`cmd/bingxprobe/main.go`** — REST-based BingX ticker probe. Reads `BINGX_API_KEY` + `BINGX_SECRET_KEY` from env, constructs `bingx.NewAdapter`, calls `Adapter.GetOrderbook` for top-of-book bid/ask/mid, prints to stdout, exits 0 on success.
- **`make probe-bingx`** — `.PHONY` target appended to existing Makefile (5 targets → 6); body runs `go run ./cmd/bingxprobe/`.
- **`16-03-RESTORATION-NOTES.md`** — 6 H2 sections (Restoration source / D-11 contract / Gap analysis / Retrofit plan / Compile risk / TestOrder safety verification) documenting the full restoration audit trail + T-16-03-01 mitigation evidence.
- **`16-03-HUMAN-UAT.md`** — operator-attested UAT pass (verdict: pass; stdout not captured into conversation).
- **VERSION 0.38.3 + CHANGELOG** — entry references original deletion commit `21cb60b`, restoration source `21cb60b^`, ticker-only deviation rationale, and operator UAT path.

## Tasks Executed

| Task | Name | Commit |
|------|------|--------|
| 1 | Inspect 21cb60b^ source + assess D-11 gap + verify TestOrder dry-run safety | 74194a7 |
| 2 | Restore cmd/bingxprobe/main.go (Path B, ticker-only) + Makefile target | cd75411 |
| 3 | Operator runs `make probe-bingx` (checkpoint:human-verify) | 4d1a54f (UAT doc) |
| 4 | VERSION + CHANGELOG bump for DEV-01 | 6697901 |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 — Bug-class adjustment] Adapter signature drift between plan `<interfaces>` block and live code**

- **Found during:** Task 1
- **Issue:** Plan stated `TestOrder(ctx, PlaceOrderRequest)` based on `pkg/exchange/types.go:79-83` interface; actual BingX adapter implements `TestOrder(req exchange.PlaceOrderParams) error` — no context, different param struct name.
- **Fix:** Documented in 16-03-RESTORATION-NOTES.md `## D-11 contract` section. Plan's stated *intent* (mirror execution.go:296) honoured over literal interface text. Did not affect execution because Task 2 ultimately did not call TestOrder (see deviation 2).
- **Files modified:** `.planning/phases/16-paper-mode-cleanup-dashboard-consolidation/16-03-RESTORATION-NOTES.md`
- **Commit:** 74194a7

**2. [Rule 2 — Critical safety, T-16-03-01 mitigation] BingX adapter `TestOrder` is unsafe per plan's narrow definition**

- **Found during:** Task 1 Step 4 (TestOrder dry-run safety verification, per checker info #8)
- **Issue:** Adapter's `TestOrder` (pkg/exchange/bingx/adapter.go:220) places a real non-marketable IOC limit order at `/openApi/swap/v2/trade/order` and cancels it — does NOT hit `/openApi/swap/v2/trade/order/test`. Adapter header comment is explicit that this is intentional (BingX's `/order/test` bypasses the temporary market-risk gate the engine validates). Per the plan's narrow definition (`safe = path string contains /test`), verdict is **unsafe**.
- **Fix:** Per plan's own fallback (`If unsafe, the probe MUST exclude any TestOrder call and use ticker-only`), Task 2 shipped ticker-only (REST `Adapter.GetOrderbook` for top-of-book bid/ask/mid). No `TestOrder` call. CHANGELOG documents the deviation.
- **Files modified:** `cmd/bingxprobe/main.go` (ticker-only impl), `.planning/phases/16-paper-mode-cleanup-dashboard-consolidation/16-03-RESTORATION-NOTES.md` (verdict + action)
- **Commit:** cd75411

## Authentication Gates

None (BingX API keys read from existing env vars; matches adapter expectations; no new credential surface).

## Self-Check

**Files exist:**
- FOUND: cmd/bingxprobe/main.go
- FOUND: .planning/phases/16-paper-mode-cleanup-dashboard-consolidation/16-03-RESTORATION-NOTES.md
- FOUND: .planning/phases/16-paper-mode-cleanup-dashboard-consolidation/16-03-HUMAN-UAT.md

**Commits exist:**
- FOUND: 74194a7 (Task 1)
- FOUND: cd75411 (Task 2)
- FOUND: 4d1a54f (Task 3 UAT doc)
- FOUND: 6697901 (Task 4 VERSION/CHANGELOG)

**Build verification:**
- `go build ./...` → exit 0
- `go vet ./...` → exit 0
- `git diff config.json` → empty
- `git diff go.mod go.sum` → empty

## Self-Check: PASSED
