---
phase: 10
slug: dashboard-candidate-crud
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-25
---

# Phase 10 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Detail derived from `.planning/phases/10-dashboard-candidate-crud/10-RESEARCH.md` §"Validation Architecture".

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework (Go)** | `go test` (stdlib) |
| **Framework (TS)** | `vitest` (already in `web/package.json`) |
| **Config file** | `web/vitest.config.ts` (existing); `go.mod` |
| **Quick run command** | `cd /var/solana/data/arb && go test ./internal/api/ ./internal/config/ -run "PriceGap" -count=1` |
| **Full suite command** | `cd /var/solana/data/arb && go test ./... -count=1 && cd web && npm run test -- --run` |
| **Estimated runtime** | ~30s Go, ~10s vitest |

---

## Sampling Rate

- **After every task commit:** Quick run command (Go subset + vitest --run)
- **After every plan wave:** Full suite command
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 60 seconds

---

## Per-Task Verification Map

To be filled by planner from PLAN.md tasks. Each task must map to:
- A test file (Go `*_test.go` or TS `*.test.tsx`)
- An automated command (grep / test invocation / build check)
- The requirement ID (PG-OPS-07)
- A threat-model reference if applicable (e.g., T-10-01 = backend validation, T-10-02 = active-position guard)

---

## Wave 0 Requirements

- [ ] `internal/api/config_handlers_pricegap_test.go` — new test file for the candidates apply path (no existing infrastructure for this code)
- [ ] `internal/pricegaptrader/tracker_test.go` — verify per-tick re-read confirmation (assumption A1 in RESEARCH.md)
- [ ] `web/src/pages/PriceGap.candidates.test.tsx` — vitest for modal validation logic (RTL already configured in repo)

*Existing `vitest` and `go test` infrastructure covers framework needs — no installs required.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Modal does not auto-POST on mount | PG-OPS-07 (Phase 9 PG-OPS-08 carryover) | Browser-only behavior; need DevTools Network tab | Open Price-Gap tab, open Add modal, observe Network panel — confirm zero `/api/config` POSTs without explicit Save click |
| EN/zh-TW lockstep visual check | PG-OPS-07 | i18n key parity is type-checked but visual layout (Chinese line-wrap) is not | Switch locale to zh-TW, open Add/Edit modal, confirm no overflow / truncation in field labels and error text |
| `.bak` backup created | PG-OPS-07 | File-system effect outside test harness | After first POST: `ls -la config.json config.json.bak` confirms both exist with backup mtime older than current |

---

## Validation Sign-Off

- [ ] All tasks have automated verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags (vitest must run with `--run`)
- [ ] Feedback latency < 60s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
