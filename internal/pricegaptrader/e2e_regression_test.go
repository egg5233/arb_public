// Package pricegaptrader — E2E regression guardrails for Phase 9 (Plan 09-08 Task 1).
//
// These tests document the regression-detection contract: the existing
// perp-perp engine (internal/engine) and spot-futures engine (internal/spotengine)
// must remain byte-for-byte unchanged in behavior when Phase 9 is disabled
// (PriceGapEnabled=false) or live (PriceGapEnabled=true, PriceGapPaperMode=true|false).
//
// In the pricegaptrader package we cannot directly exercise internal/engine
// or internal/spotengine (D-02 module boundary — the tracker package MUST NOT
// import those packages). The real guardrails live in:
//
//   1. internal/notify/telegram_regression_test.go (Plan 09-04) — pins
//      byte-for-byte output for every pre-Phase-9 Notify* method. Any drift
//      in perp-perp or spot-futures Telegram formatting fails there.
//
//   2. `go test ./internal/engine/... -count=1 -race` staying green after
//      Phase 9 lands. The engine package's own tests are the authoritative
//      regression surface for funding-rate-arb behavior.
//
//   3. `go test ./internal/spotengine/... -count=1 -race` staying green.
//
//   4. The safety property test in this package
//      (TestPriceGapEnabled_DefaultOff_NoTrackerInstantiated in tracker_test.go)
//      asserts zero pg:* Redis writes on the PriceGapEnabled=false path, which
//      is what a pre-Phase-9-style deployment runs.
//
// The two tests below are deliberate t.Skip stubs. They document the
// regression contract in a discoverable location (next to the cutover tests)
// and will flag to future maintainers that they should NOT fabricate a
// cross-package regression harness here — that would either break the D-02
// boundary or produce a test that cannot actually assert what it claims.
package pricegaptrader

import "testing"

// TestE2ERegression_PerpPerpEngineUntouched — documents the guardrail; the
// actual assertion is delegated to `go test ./internal/engine/... -count=1 -race`
// remaining green, and to internal/notify/telegram_regression_test.go's
// byte-for-byte pinning of every pre-Phase-9 Notify* method.
//
// Why skip: building an in-process engine harness here would require importing
// internal/engine from pricegaptrader, which violates D-02 (pricegaptrader must
// depend on nothing above it in the dependency graph). A proper cross-package
// regression suite belongs under internal/engine/ itself.
func TestE2ERegression_PerpPerpEngineUntouched(t *testing.T) {
	t.Skip("guardrail: regression is enforced by (a) internal/engine/... tests staying green under -count=1 -race, and (b) internal/notify/telegram_regression_test.go pinning Notify* byte-for-byte. See plan 09-08 acceptance criteria.")
}

// TestE2ERegression_SpotFuturesEngineUntouched — documents the spot-futures
// guardrail; assertion delegated to `go test ./internal/spotengine/... -count=1 -race`
// remaining green after Phase 9 lands, plus the Telegram regression tests.
//
// Why skip: same D-02 reasoning as above. internal/spotengine is a sibling
// package of internal/pricegaptrader; importing it from here would create a
// disallowed dependency edge.
func TestE2ERegression_SpotFuturesEngineUntouched(t *testing.T) {
	t.Skip("guardrail: regression is enforced by (a) internal/spotengine/... tests staying green under -count=1 -race, and (b) internal/notify/telegram_regression_test.go pinning Notify* byte-for-byte. See plan 09-08 acceptance criteria.")
}
