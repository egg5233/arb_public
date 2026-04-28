---
phase: 11
slug: auto-discovery-scanner-chokepoint-telemetry
status: ready
nyquist_compliant: true
wave_0_complete: true
created: 2026-04-28
updated: 2026-04-28
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

> One row per task across all 6 plans. Every task has either an automated verify command OR a checkpoint variant. Wave 0 stubs are produced by the plan that owns the file.

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | Wave 0 Source | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|---------------|--------|
| 11-01-01 | 01 | 1 | PG-DISC-01, PG-DISC-04 | T-11-01..07 | Config schema validation rejects malformed universe/denylist; RegistryReader interface declared | unit | `go test ./internal/config/... -run TestPriceGapDiscovery_ -count=1` | self-authors registry_reader.go | ⬜ pending |
| 11-01-02 | 01 | 1 | PG-DISC-01 | T-11-02 | Universe/denylist regex + ≤20 cap enforced at config load; tests cover empty/malformed/oversized | unit | `go test ./internal/config/... -run TestPriceGapDiscoveryUniverse -count=1` | self | ⬜ pending |
| 11-02-01 | 02 | 2 | PG-DISC-04 | T-11-16 | Atomic write tmp→fsync→rename; .bak ring of 5; renameFunc/nowFunc injectable | unit | `go test ./internal/config/... -run 'TestBakRing|TestSaveJSONWithBakRing' -count=1 -timeout=30s` | self-authors config_bakring_test.go | ⬜ pending |
| 11-02-02 | 02 | 2 | PG-DISC-04 | T-11-08, T-11-10..15 | Registry mutators take cfg.mu; reload-from-disk; in-memory rollback on persist failure; audit-trail to pg:registry:audit | unit | `go test ./internal/pricegaptrader/ -run 'TestRegistry_(Add|Update|Delete|Replace|Get|List|Audit|Reload)' -count=1 -timeout=30s` | self-authors registry_test.go | ⬜ pending |
| 11-02-03 | 02 | 2 | PG-DISC-04 | T-11-09, T-11-46 | Cross-instance goroutine race via internal/pricegaptrader/testhelpers (no os.Exec) proves disk-as-sync-point; cmd/ tree unchanged | integration | `go test ./internal/pricegaptrader/ -run TestRegistry_Concurrent -count=1 -timeout=60s` | self-authors registry_concurrent_test.go + testhelpers/concurrent_writer.go | ⬜ pending |
| 11-03-01 | 03 | 3 | PG-DISC-04 | T-11-18..22 | Dashboard PUT/POST handlers route through Registry.Replace; legacy direct-mutation site at handlers.go:1700 deleted | unit | `go test ./internal/api/ -run 'TestPostConfigCandidates_|TestPriceGapHandler' -count=1` | uses 11-02 registry_test.go fixtures | ⬜ pending |
| 11-03-02 | 03 | 3 | PG-DISC-04 | T-11-23..25 | pg-admin CLI hard-cut onto Registry; ValidateCandidate helper extracted; legacy direct-mutation removed | unit | `go test ./cmd/pg-admin/... -count=1 -timeout=30s` | uses 11-02 registry_test.go fixtures | ⬜ pending |
| 11-03-03 | 03 | 3 | PG-DISC-04 | T-11-26 | Static check: handlers.go + pg-admin/main.go contain NO direct cfg.PriceGapCandidates = ... assignments | static (regex) | `go test ./internal/api/... ./cmd/pg-admin/... -run TestNoLegacyCandidatesMutation -count=1` | self | ⬜ pending |
| 11-04-01 | 04 | 4 | PG-DISC-01 | T-11-26..30 | Scanner core: gate-then-magnitude scoring; 0-100; sub-scores; reason-coded rejection; default-OFF; compile-time read-only | unit | `go test ./internal/pricegaptrader/ -run 'TestScanner_' -count=1 -timeout=30s` | self-authors scanner_test.go + scanner_static_test.go | ⬜ pending |
| 11-05-01 | 05 | 5 | PG-DISC-03, PG-DISC-01 | T-11-31..36 | Telemetry: pg:scan:* schema + WS broadcasts + GetState/GetScores helpers; Tracker scanLoop + subscribeUniverse | unit + integration | `go test ./internal/pricegaptrader/ -run 'TestTelemetry_|TestTracker_(ScanLoop|SubscribeUniverse)' -count=1 -timeout=30s` | self-authors telemetry_test.go | ⬜ pending |
| 11-05-02 | 05 | 5 | PG-DISC-03 | T-11-33 | REST handlers GET /api/pg/discovery/state and /scores/{symbol}; Bearer auth; ValidateSymbol regex on path-param | unit | `go test ./internal/api/ ./internal/pricegaptrader/ -run 'TestPgDiscovery|TestValidateSymbol' -count=1 -timeout=30s` | self-authors handlers_pricegap_discovery_test.go | ⬜ pending |
| 11-05-03 | 05 | 5 | PG-DISC-01, PG-DISC-03 | T-11-44 | cmd/arb/main.go bootstrap: Telemetry unconditional, Scanner conditional on cfg.PriceGapDiscoveryEnabled; injected via Tracker options BEFORE apiServer.Start | static + build | `go build ./cmd/arb && go vet ./cmd/arb/...` | static grep + build | ⬜ pending |
| 11-06-01 | 06 | 6 | PG-DISC-03 | T-11-38 | i18n keys ≥30 in en.ts + zh-TW.ts (lockstep); TranslationKey derivation enforces compile-time check | unit + tsc | `cd web && npx vitest run --testNamePattern discovery_keys && npx tsc --noEmit` | self-authors discovery_keys.test.ts | ⬜ pending |
| 11-06-02 | 06 | 6 | PG-DISC-03 | T-11-37, T-11-39, T-11-42, T-11-43 | 6 React components + usePgDiscovery hook + DiscoverySection insertion; zero new npm deps; Recharts subcomponents only | unit + build | `cd web && npx vitest run --testNamePattern 'DiscoverySection|usePgDiscovery|CycleStatsCard|ScoreHistoryCard|WhyRejectedCard' && npx tsc --noEmit && npm run build && git diff --exit-code web/package.json web/package-lock.json` | self-authors DiscoverySection.test.tsx | ⬜ pending |
| 11-06-03 | 06 | 6 | PG-DISC-03 | T-11-40, T-11-41 | Visual checkpoint: three scanner states (OFF/ON/ERRORED) + locale switching render correctly | checkpoint:human-verify | manual — operator validates per how-to-verify | n/a (UI runtime) | ⬜ pending |
| 11-06-04 | 06 | 6 | PG-DISC-01 (Success #5) | T-11-45 | Live BBO join checkpoint: BTCUSDT cross-pair across all 6 exchanges under real feeds; pg:scan:scores:BTCUSDT populated | checkpoint:human-verify | manual — operator runs verification per how-to-verify | n/a (live runtime) | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [x] `internal/pricegaptrader/registry_test.go` — stubs for chokepoint serialisation, Add/Update/Delete/Replace, .bak.{ts} ring (PG-DISC-04) — produced by Plan 02 Task 2
- [x] `internal/pricegaptrader/registry_concurrent_test.go` — concurrent dashboard + pg-admin (via testhelpers) writes through chokepoint (zero lost mutations) (PG-DISC-04) — produced by Plan 02 Task 3
- [x] `internal/pricegaptrader/testhelpers/concurrent_writer.go` — library helper for cross-instance race tests (test-only) — produced by Plan 02 Task 3
- [x] `internal/pricegaptrader/scanner_test.go` — stubs for universe bounding, persistence/freshness/depth gates, score determinism, default-OFF (PG-DISC-01) — produced by Plan 04 Task 1
- [x] `internal/pricegaptrader/scanner_static_test.go` — static read-only invariant test (no `*Registry` or registry.Add/Update/Delete/Replace tokens) — produced by Plan 04 Task 1
- [x] `internal/pricegaptrader/telemetry_test.go` — pg:scan:* Redis schema + WS throttle/debounce + GetState/GetScores (PG-DISC-03) — produced by Plan 05 Task 1
- [x] `internal/api/pricegap_discovery_handlers_test.go` — REST contract for `/api/pg/discovery/state` and `/api/pg/discovery/scores/{symbol}` (PG-DISC-03) — produced by Plan 05 Task 2
- [x] `web/src/components/Discovery/__tests__/DiscoverySection.test.tsx` — vitest stubs for cycle stats card, score history chart, why-rejected breakdown (PG-DISC-03) — produced by Plan 06 Task 2
- [x] `web/src/i18n/__tests__/discovery_keys.test.ts` — verify all `pricegap.discovery.*` keys exist in BOTH `en.ts` and `zh-TW.ts` (lockstep) — produced by Plan 06 Task 1

*Wave 0 stubs are produced inline by the plan that owns the file (TDD discipline within each task). Plan 04 declares the TelemetryWriter interface that Plan 05 implements, ensuring Wave 4 → Wave 5 contract handoff is type-checked.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Plan-Task | Test Instructions |
|----------|-------------|------------|-----------|-------------------|
| Cross-exchange BBO join produces consistent canonical symbols on a known-good pair (e.g. BTCUSDT) under live data | PG-DISC-01 (Success #5) | Requires live exchange WS feeds (rate-limited; mock fixtures in Plan 04 Test 12 cover the join logic but not symbol-format drift across all 6 adapters) | 11-06 Task 4 | (1) Set `PriceGapDiscoveryEnabled=true`, restart binary; (2) Wait one scan cycle; (3) Verify dashboard Discovery section lists BTCUSDT with all 6 exchange BBO points; (4) `redis-cli -n 2 LRANGE pg:scan:cycles -1 -1 \| jq '.records[]'` shows records for all 30 ordered cross-pairs of {binance, bybit, gate, bitget, okx, bingx} |
| Bybit blackout :04–:05:30 avoidance under wall-clock observation | PG-DISC-01 | Goroutine schedule is wall-clock-driven; unit test can't observe minute-boundary drift in CI | 11-06 Task 4 (subsumed) | Tail logs across 2h and confirm no scanner activity inside :04–:05:30 windows; cycle stats `last_run_ts` never falls in that range |
| Frontend visual quality (chart legibility, table density, why-rejected breakdown UX) | PG-DISC-03 | Visual judgement; covered by `/gsd-ui-review` post-execution, not by unit tests | 11-06 Task 3 | Captured via UI-SPEC.md design contract + `/gsd-ui-review` 6-pillar audit at end of phase |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies (matrix above is fully populated)
- [x] Sampling continuity: no 3 consecutive tasks without automated verify (the only checkpoints are 11-06-03 and 11-06-04, separated by an automated task earlier in Wave 6)
- [x] Wave 0 covers all MISSING references (every Wave 0 stub maps to its producing plan-task)
- [x] No watch-mode flags
- [x] Feedback latency < 30s
- [x] `nyquist_compliant: true` set in frontmatter
- [x] `wave_0_complete: true` set in frontmatter (Wave 0 stubs are inlined into the producing tasks per project TDD discipline)

**Approval:** ready for execution
</content>
</invoke>