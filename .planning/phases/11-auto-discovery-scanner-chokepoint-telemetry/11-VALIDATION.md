---
phase: 11
slug: auto-discovery-scanner-chokepoint-telemetry
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-28
---

# Phase 11 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (backend) + vitest 1.x (frontend, web/) |
| **Config file** | none for Go (stdlib); `web/vitest.config.ts` for frontend |
| **Quick run command** | `go test ./internal/pricegaptrader/... -count=1 -timeout=30s` |
| **Full suite command** | `go test ./... -count=1 -timeout=120s && (cd web && npm run test -- --run)` |
| **Estimated runtime** | ~30s backend, ~15s frontend |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/pricegaptrader/... -count=1 -timeout=30s`
- **After every plan wave:** Run `go test ./internal/pricegaptrader/... ./internal/api/... ./internal/config/... -count=1 -timeout=60s`
- **Before `/gsd-verify-work`:** Full suite must be green (backend + frontend vitest + `make build` succeeds with frontend embed)
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

> Populated by planner. Every task in every PLAN.md MUST appear here with an automated command OR a Wave 0 dependency reference. The Nyquist auditor enforces no-three-consecutive-gaps.

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 11-XX-XX | XX | X | PG-DISC-XX | — | {expected secure behavior or "N/A"} | unit/integration | `go test ./...` | ✅ / ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/pricegaptrader/registry_test.go` — stubs for chokepoint serialisation, Add/Update/Delete/Replace, .bak.{ts} ring (PG-DISC-03)
- [ ] `internal/pricegaptrader/scanner_test.go` — stubs for universe bounding, persistence/freshness/depth gates, score determinism, default-OFF (PG-DISC-01)
- [ ] `internal/pricegaptrader/scanner_concurrency_test.go` — concurrent dashboard + pg-admin + scanner-attempt writes through chokepoint (zero lost mutations) (PG-DISC-03)
- [ ] `internal/api/handlers_pricegap_discovery_test.go` — REST contract for `/api/pg/discovery/state` and `/api/pg/discovery/scores/{symbol}` (PG-DISC-04)
- [ ] `web/src/components/pricegap/Discovery/__tests__/DiscoverySection.test.tsx` — vitest stubs for cycle stats card, score history chart, why-rejected breakdown (PG-DISC-04)
- [ ] `web/src/i18n/__tests__/discovery_keys.test.ts` — verify all `pricegap.discovery.*` keys exist in BOTH `en.ts` and `zh-TW.ts` (lockstep)

*Wave 0 stubs MUST be created before any implementation task in Wave 1+ runs. They define the contract the executor must satisfy.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Cross-exchange BBO join produces consistent canonical symbols on a known-good pair (e.g. BTCUSDT) under live data | PG-DISC-01 | Requires live exchange WS feeds (rate-limited; mock fixtures cover the join logic but not symbol-format drift across all 6 adapters) | (1) Set `PriceGapDiscoveryEnabled=true`, restart binary; (2) Wait one scan cycle; (3) Verify dashboard Discovery section lists BTCUSDT with all 6 exchange BBO points; (4) `redis-cli -n 2 ZRANGE pg:scan:scores:BTCUSDT 0 -1 WITHSCORES` shows score progression |
| Bybit blackout :04–:05:30 avoidance under wall-clock observation | PG-DISC-01 | Goroutine schedule is wall-clock-driven; unit test can't observe minute-boundary drift in CI | Tail logs across 2h and confirm no scanner activity inside :04–:05:30 windows; cycle stats `last_run_ts` never falls in that range |
| Frontend visual quality (chart legibility, table density, why-rejected breakdown UX) | PG-DISC-04 | Visual judgement; covered by `/gsd-ui-review` post-execution, not by unit tests | Captured via UI-SPEC.md design contract + `/gsd-ui-review` 6-pillar audit at end of phase |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
