---
status: partial
phase: 06-spot-futures-risk-hardening
source: [06-VERIFICATION.md]
started: 2026-04-05T22:30:00.000Z
updated: 2026-04-05T22:30:00.000Z
---

## Current Test

[awaiting human testing]

## Tests

### 1. Pre-entry gate live rejection
expected: Auto-entry rejects high-maintenance-rate contracts (e.g. GUAUSDT at 30% maint, 3x leverage)
result: [pending]

### 2. Liquidation distance trigger activation
expected: Exit trigger fires at correct distance thresholds (warn 50%, exit 20%, emergency 10%)
result: [pending]

### 3. Dashboard MR column visual
expected: Maintenance rate column shows in spot opportunities with red/amber/gray color coding
result: [pending]

## Summary

total: 3
passed: 0
issues: 0
pending: 3
skipped: 0
blocked: 0

## Gaps
