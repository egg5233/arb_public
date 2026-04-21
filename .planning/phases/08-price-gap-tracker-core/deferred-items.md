# Deferred Items — Phase 08 Price-Gap Tracker Core

Out-of-scope issues discovered during execution. Tracked for later cleanup.

## Pre-existing vet warnings

### cmd/gatecheck/main.go:48
- **Issue:** `fmt.Println arg list ends with redundant newline`
- **Found during:** Plan 07 Task 3 wiring (`go vet ./...` and `go test ./cmd/...`)
- **Status:** Pre-existing. Last touched in commit cba254f, unrelated to Phase 08 work.
- **Impact:** `go test ./cmd/gatecheck` fails at the implicit vet step; does not affect runtime.
- **Action:** Defer to a separate hygiene PR — fixing in Plan 07 would scope-creep beyond the price-gap tracker.
