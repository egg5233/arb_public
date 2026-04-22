# Deferred Items — Phase 09

Pre-existing issues discovered during Phase 09 execution but out of scope
for the current plan's changes. Tracked here for future cleanup.

## 09-01

- **cmd/gatecheck/main.go:48** — `fmt.Println("...\n")` vet warning
  (redundant newline). Pre-existing. `go vet ./cmd/gatecheck/...` alone
  passes, but `go test ./... -vet=all` (Go's default in test mode) flags it.
  Trivial fix (`Println("...")` + separate `Println()`, or `Printf`), but
  unrelated to Plan 09-01's file set. Scope boundary: defer.
