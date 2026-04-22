// Package notify — pricegap_assert.go asserts at compile time that
// *TelegramNotifier satisfies pricegaptrader.PriceGapNotifier (Phase 9 Plan
// 04 method surface, wired in Plan 06 cmd/main.go).
//
// This lives in a standalone file in internal/notify (not in the pricegaptrader
// package, where the interface is declared) so the dependency arrow points
// correctly: internal/notify → internal/pricegaptrader is fine, but a mirror
// import in pricegaptrader would create a cycle (pricegaptrader must NOT
// import notify per D-02 module boundary).
package notify

import "arb/internal/pricegaptrader"

var _ pricegaptrader.PriceGapNotifier = (*TelegramNotifier)(nil)
