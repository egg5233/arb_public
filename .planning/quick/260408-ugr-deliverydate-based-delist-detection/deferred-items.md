# Deferred items — quick task 260408-ugr

Items discovered during execution that are out of scope for this plan.

## Pre-existing test build failure: `internal/discovery/test_helpers_test.go:75`

**Discovered during:** Task 2 verification (`go test ./internal/discovery/...`)

**Symptom:**
```
internal/discovery/test_helpers_test.go:75:10: cannot use stubExchange{…} (value of struct type stubExchange) as exchange.Exchange value in assignment: stubExchange does not implement exchange.Exchange (missing method CancelAllOrders)
```

**Root cause:** The `stubExchange` test fixture in `test_helpers_test.go` was not updated when the `CancelAllOrders` method was added to the `exchange.Exchange` interface in v0.29.2 (`feat: CancelAllOrders interface — new Exchange method cancels all open orders` — see CHANGELOG.md). The test package fails to build, blocking ALL discovery package tests from running.

**Verification it pre-dates this PR:** Stashing my Task 1+2 changes (`git stash --include-untracked`) reproduces the same failure on HEAD `6298fdc`, confirming it's pre-existing.

**Why not fixed here:** Per CLAUDE.md scope boundary rule, only auto-fix issues directly caused by the current task's changes. This is unrelated to the deliveryDate work — the `stubExchange` test fixture has nothing to do with `LoadAllContracts` or contract refresh. Touching it would expand the diff and risk breaking unrelated tests.

**Suggested follow-up:** Single-line addition of `CancelAllOrders(symbol string) error { return nil }` to `stubExchange` in `test_helpers_test.go` (or wherever the stub is defined). Should be a separate quick task.

**Impact on this PR:** None for production code. The new code in `contract_refresh.go` and the modifications to `scanner.go`, `config.go`, `cmd/main.go`, `engine.go`, `database/state.go`, and `spotengine/{risk_gate,monitor}.go` all build cleanly with `go build ./...`. Unit tests in `pkg/exchange/binance/...` and `pkg/exchange/bybit/...` (the relevant adapter tests for this PR) all pass.
