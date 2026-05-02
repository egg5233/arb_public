// Package main — bingxprobe: ticker-only REST probe against the BingX swap
// market via the production pkg/exchange/bingx adapter. Restored in Phase 16
// Plan 03 (DEV-01) per CONTEXT D-11. The original 21cb60b^ utility was a
// public WS bookTicker subscriber for a 2026-04-24 BBO-decode bug (fixed
// v0.34.6) and has been fully replaced.
//
// Probe scope (ticker-only, downscoped from D-11 per Task 1 TestOrder safety
// verdict — see .planning/phases/16-.../16-03-RESTORATION-NOTES.md):
//   - Construct the BingX adapter from BINGX_API_KEY / BINGX_SECRET_KEY env vars
//   - REST GetOrderbook(depth=5) for one or more symbols
//   - Print best bid / best ask / mid / spread / timestamp + per-symbol verdict
//   - Exit 0 on success, non-zero on failure
//
// The plan's <interfaces> contract called for OrderPreflight.TestOrder + ticker.
// BingX's adapter.TestOrder does NOT hit a dry-run endpoint — it places a real
// non-marketable IOC limit order and cancels it immediately. For an operator-run
// debug utility we deliberately omit TestOrder so the probe never risks placing
// real orders, even briefly. Production code at internal/pricegaptrader/
// execution.go:296 continues to exercise the live preflight on every Strategy 4
// entry, so this downscoping does NOT reduce production validation coverage.
//
// Usage:
//
//	BINGX_API_KEY=... BINGX_SECRET_KEY=... make probe-bingx
//	BINGX_API_KEY=... BINGX_SECRET_KEY=... go run ./cmd/bingxprobe -symbol SOON-USDT
//
// Multiple symbols can be passed via -symbols comma-separated.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"arb/pkg/exchange"
	"arb/pkg/exchange/bingx"
)

const (
	defaultSymbols = "SOON-USDT"
	depth          = 5
	envAPIKey      = "BINGX_API_KEY"
	envSecretKey   = "BINGX_SECRET_KEY"
)

func main() {
	var symbolsFlag string
	flag.StringVar(&symbolsFlag, "symbols", defaultSymbols, "comma-separated BingX symbols (e.g. SOON-USDT,HOLO-USDT)")
	flag.Parse()

	// Credentials live in env (matches adapter expectations + repo CLAUDE.local.md
	// — no hard-coded keys, never reads config.json).
	apiKey := os.Getenv(envAPIKey)
	secretKey := os.Getenv(envSecretKey)
	if apiKey == "" || secretKey == "" {
		fmt.Fprintf(os.Stderr, "bingxprobe: %s and %s must be set\n", envAPIKey, envSecretKey)
		os.Exit(2)
	}

	adapter := bingx.NewAdapter(exchange.ExchangeConfig{
		Exchange:  "bingx",
		ApiKey:    apiKey,
		SecretKey: secretKey,
	})

	symbols := splitSymbols(symbolsFlag)
	if len(symbols) == 0 {
		fmt.Fprintln(os.Stderr, "bingxprobe: -symbols produced no usable entries")
		os.Exit(2)
	}

	fmt.Printf("bingxprobe: ticker-only probe via pkg/exchange/bingx (%d symbol(s))\n", len(symbols))
	fmt.Printf("bingxprobe: started at %s\n", time.Now().UTC().Format(time.RFC3339))

	failures := 0
	for _, sym := range symbols {
		if err := probeSymbol(adapter, sym); err != nil {
			failures++
			fmt.Fprintf(os.Stderr, "bingxprobe: %s FAIL: %v\n", sym, err)
		}
	}

	if failures > 0 {
		fmt.Fprintf(os.Stderr, "bingxprobe: %d/%d symbol(s) failed\n", failures, len(symbols))
		os.Exit(1)
	}
	fmt.Printf("bingxprobe: %d/%d symbol(s) OK — exit 0\n", len(symbols), len(symbols))
}

// probeSymbol fetches the top-of-book orderbook (depth=5) for sym via the
// production BingX REST client and prints best bid/ask/mid/spread.
func probeSymbol(adapter *bingx.Adapter, sym string) error {
	ob, err := adapter.GetOrderbook(sym, depth)
	if err != nil {
		return fmt.Errorf("GetOrderbook: %w", err)
	}
	if ob == nil {
		return fmt.Errorf("GetOrderbook returned nil orderbook")
	}
	if len(ob.Bids) == 0 || len(ob.Asks) == 0 {
		return fmt.Errorf("empty book (bids=%d asks=%d)", len(ob.Bids), len(ob.Asks))
	}
	bid := ob.Bids[0].Price
	ask := ob.Asks[0].Price
	if bid <= 0 || ask <= 0 {
		return fmt.Errorf("non-positive top-of-book (bid=%.8f ask=%.8f)", bid, ask)
	}
	if ask <= bid {
		return fmt.Errorf("crossed/locked book (bid=%.8f ask=%.8f)", bid, ask)
	}
	mid := (bid + ask) / 2
	spreadBps := (ask - bid) / mid * 10000
	fmt.Printf(
		"bingxprobe: %s OK bid=%.8f ask=%.8f mid=%.8f spread=%.2fbps t=%s\n",
		sym, bid, ask, mid, spreadBps, ob.Time.UTC().Format(time.RFC3339),
	)
	return nil
}

// splitSymbols splits the -symbols flag value, trims whitespace, and drops
// empty fragments.
func splitSymbols(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		s := strings.TrimSpace(p)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}
