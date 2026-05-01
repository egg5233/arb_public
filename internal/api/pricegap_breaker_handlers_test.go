package api

import "testing"

// Wave 0 stubs for breaker REST handlers. Plan 15-04 implements.

// TestPgBreakerHandlers_TypedPhraseRequired — D-12: every mutation surface
// (recover, test-fire) rejects the request unless the body contains the
// exact typed phrase ("RECOVER" or "TEST-FIRE"). Magic strings, no i18n.
func TestPgBreakerHandlers_TypedPhraseRequired(t *testing.T) {
	t.Skip("Wave 0 stub — Plan 15-04 implements POST /api/pg/breaker/{recover,test-fire}")
}

// TestPgBreakerHandlers_RecoverEndpoint — happy path: typed phrase RECOVER
// + auth, controller clears sticky + pause flags, returns 200 with state.
func TestPgBreakerHandlers_RecoverEndpoint(t *testing.T) {
	t.Skip("Wave 0 stub — Plan 15-04 implements recovery endpoint")
}

// TestPgBreakerHandlers_TestFireEndpoint — D-14 default real trip; --dry-run
// supported via JSON body flag. Source recorded as test_fire / test_fire_dry_run.
func TestPgBreakerHandlers_TestFireEndpoint(t *testing.T) {
	t.Skip("Wave 0 stub — Plan 15-04 implements test-fire endpoint")
}

// TestPgBreakerHandlers_StateGetEndpoint — GET /api/pg/breaker/state returns
// snapshot of pg:breaker:state HASH + recent trips (LRANGE 0 9).
func TestPgBreakerHandlers_StateGetEndpoint(t *testing.T) {
	t.Skip("Wave 0 stub — Plan 15-04 implements state GET endpoint")
}

// TestPgBreakerHandlers_AuthRequired — bearer token required on every
// breaker handler (existing auth middleware reuse).
func TestPgBreakerHandlers_AuthRequired(t *testing.T) {
	t.Skip("Wave 0 stub — Plan 15-04 implements auth wiring")
}
