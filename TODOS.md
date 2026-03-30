# TODOS

## Gate.io

### P0: Fix TestQuantoMultiplierRoundTrip (pre-existing)
**Priority:** P0
**File:** `pkg/exchange/gateio/adapter_test.go:142`
**Error:** `PlaceOrder sent 0 contracts, expected 88 (base units 880 / mult 10)`
**Noticed on:** `dev` branch (pre-existing — fails on `main` too)
**Branch noticed:** dev
**Description:** The quanto multiplier round-trip test fails because `PlaceOrder` sends 0 contracts instead of the expected 88. Not caused by any change on the current branch — the branch's only gateio change is in `EnsureOneWayMode`. This needs root cause investigation in the PlaceOrder / contract sizing logic.

## Completed
