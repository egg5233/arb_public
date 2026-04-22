// cmd/peertest/main.go
//
// Empirical non-ASCII symbol test for peer exchanges (Bybit, OKX, Gate.io, BingX, Binance).
//
// Purpose: verify whether each exchange handles symbols containing non-ASCII
// characters (e.g. Chinese token names) correctly on the same kind of signed
// GET endpoints where Bitget was found to silently fail.
//
// Read-only — NO order placement of any kind.
// Exit code 0 when the test run completes (failures are data, not fatal).
//
// Usage:
//
//	go run ./cmd/peertest/
//	go run ./cmd/peertest/ -exchange bybit
package main

import (
	"flag"
	"fmt"
	"os"
	"time"
	"unicode"

	"arb/internal/config"
	"arb/pkg/exchange"
	"arb/pkg/exchange/binance"
	"arb/pkg/exchange/bingx"
	"arb/pkg/exchange/bybit"
	"arb/pkg/exchange/gateio"
	"arb/pkg/exchange/okx"
)

func main() {
	exchangeFlag := flag.String("exchange", "", "test a specific exchange (binance, bybit, okx, gateio, bingx); default: all configured")
	flag.Parse()

	cfg := config.Load()

	type entry struct {
		name string
		exc  exchange.Exchange
	}

	var adapters []entry

	if *exchangeFlag != "" {
		exc, err := makeAdapter(cfg, *exchangeFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
			os.Exit(1)
		}
		adapters = append(adapters, entry{name: *exchangeFlag, exc: exc})
	} else {
		for _, name := range []string{"bybit", "okx", "gateio", "bingx", "binance"} {
			exc, err := makeAdapter(cfg, name)
			if err != nil {
				fmt.Printf("[%s] skipping: %v\n", name, err)
				continue
			}
			adapters = append(adapters, entry{name: name, exc: exc})
		}
	}

	if len(adapters) == 0 {
		fmt.Fprintln(os.Stderr, "No exchanges configured. Set API key env vars (or populate config.json).")
		fmt.Fprintln(os.Stderr, "Supported: BINANCE_API_KEY, BYBIT_API_KEY, OKX_API_KEY, GATEIO_API_KEY, BINGX_API_KEY")
		os.Exit(1)
	}

	for _, a := range adapters {
		runPeerTest(a.name, a.exc)
		fmt.Println()
	}

	// Always exit 0 — failures are reported data, not fatal.
}

// ---------------------------------------------------------------------------
// Adapter factory (read-only exchanges only — bitget intentionally omitted)
// ---------------------------------------------------------------------------

func makeAdapter(cfg *config.Config, name string) (exchange.Exchange, error) {
	switch name {
	case "binance":
		if cfg.BinanceAPIKey == "" {
			return nil, fmt.Errorf("BINANCE_API_KEY not configured")
		}
		return binance.NewAdapter(exchange.ExchangeConfig{
			Exchange:  "binance",
			ApiKey:    cfg.BinanceAPIKey,
			SecretKey: cfg.BinanceSecretKey,
		}), nil
	case "bybit":
		if cfg.BybitAPIKey == "" {
			return nil, fmt.Errorf("BYBIT_API_KEY not configured")
		}
		return bybit.NewAdapter(exchange.ExchangeConfig{
			Exchange:  "bybit",
			ApiKey:    cfg.BybitAPIKey,
			SecretKey: cfg.BybitSecretKey,
		}), nil
	case "okx":
		if cfg.OKXAPIKey == "" {
			return nil, fmt.Errorf("OKX_API_KEY not configured")
		}
		if cfg.OKXPassphrase == "" {
			return nil, fmt.Errorf("OKX_PASSPHRASE not configured (required for OKX signing)")
		}
		return okx.NewAdapter(exchange.ExchangeConfig{
			Exchange:   "okx",
			ApiKey:     cfg.OKXAPIKey,
			SecretKey:  cfg.OKXSecretKey,
			Passphrase: cfg.OKXPassphrase,
		}), nil
	case "gateio":
		ga := gateio.NewAdapter(exchange.ExchangeConfig{
			Exchange:  "gateio",
			ApiKey:    cfg.GateioAPIKey,
			SecretKey: cfg.GateioSecretKey,
		})
		if cfg.GateioAPIKey == "" {
			return nil, fmt.Errorf("GATEIO_API_KEY not configured")
		}
		ga.DetectUnifiedMode()
		return ga, nil
	case "bingx":
		if cfg.BingXAPIKey == "" {
			return nil, fmt.Errorf("BINGX_API_KEY not configured")
		}
		return bingx.NewAdapter(exchange.ExchangeConfig{
			Exchange:  "bingx",
			ApiKey:    cfg.BingXAPIKey,
			SecretKey: cfg.BingXSecretKey,
		}), nil
	default:
		return nil, fmt.Errorf("unsupported exchange %q (supported: binance, bybit, okx, gateio, bingx)", name)
	}
}

// ---------------------------------------------------------------------------
// Non-ASCII detection
// ---------------------------------------------------------------------------

// containsNonASCII returns true if s contains any rune outside the printable
// ASCII range (0x20–0x7E). This matches the condition that caused Bitget's
// HMAC signing to fail — percent-encoding non-ASCII bytes changes the query
// string length that the exchange validates against the signature.
func containsNonASCII(s string) bool {
	for _, r := range s {
		if r > unicode.MaxASCII || !unicode.IsPrint(r) {
			return true
		}
	}
	return false
}

// pickNonASCII scans contracts and returns the first symbol that contains a
// non-ASCII character. Returns "" if none found.
func pickNonASCII(contracts map[string]exchange.ContractInfo) string {
	for sym := range contracts {
		if containsNonASCII(sym) {
			return sym
		}
	}
	return ""
}

// ---------------------------------------------------------------------------
// Per-exchange test runner
// ---------------------------------------------------------------------------

func runPeerTest(name string, exc exchange.Exchange) {
	fmt.Printf("=== %s: loading contracts ===\n", name)

	contracts, err := exc.LoadAllContracts()
	if err != nil {
		fmt.Printf("[%s] LoadAllContracts FAIL: %v\n", name, err)
		return
	}
	fmt.Printf("[%s] LoadAllContracts OK — %d contracts\n", name, len(contracts))

	sym := pickNonASCII(contracts)
	if sym == "" {
		fmt.Printf("[%s] no non-ASCII symbol found — skipping endpoint tests (no test case available for this exchange)\n", name)
		return
	}

	fmt.Printf("=== %s: testing non-ASCII symbol %q ===\n", name, sym)

	runGetFundingRate(name, exc, sym)
	runGetUserTrades(name, exc, sym)
	runGetPosition(name, exc, sym)

	fmt.Printf("=== %s: done ===\n", name)
}

// ---------------------------------------------------------------------------
// Read-only endpoint probes
// ---------------------------------------------------------------------------

// runGetFundingRate calls GetFundingRate with the non-ASCII symbol.
// Any non-nil error is informative (HMAC issue would usually surface here).
func runGetFundingRate(name string, exc exchange.Exchange, sym string) {
	fmt.Printf("[%s] GetFundingRate(%q) ... ", name, sym)
	fr, err := exc.GetFundingRate(sym)
	if err != nil {
		fmt.Printf("FAIL — %v\n", err)
		return
	}
	if fr == nil {
		// Unexpected: (nil, nil) is a silent failure — no data and no error.
		fmt.Printf("WARN — (nil, nil): silent empty response; possible signing issue\n")
		return
	}
	fmt.Printf("OK — rate=%.6f interval=%v\n", fr.Rate, fr.Interval)
}

// runGetUserTrades calls GetUserTrades with since=24h ago and limit=1.
// Read-only; returns empty slice if no trades — that is fine.
func runGetUserTrades(name string, exc exchange.Exchange, sym string) {
	since := time.Now().Add(-24 * time.Hour)
	fmt.Printf("[%s] GetUserTrades(%q, since=24h ago, limit=1) ... ", name, sym)
	trades, err := exc.GetUserTrades(sym, since, 1)
	if err != nil {
		fmt.Printf("FAIL — %v\n", err)
		return
	}
	fmt.Printf("OK — %d trade(s) returned\n", len(trades))
}

// runGetPosition calls GetPosition with the non-ASCII symbol.
// Expects either a non-empty slice (live position) or an empty slice (no
// position). A (nil, nil) return is flagged as a potential silent failure.
func runGetPosition(name string, exc exchange.Exchange, sym string) {
	fmt.Printf("[%s] GetPosition(%q) ... ", name, sym)
	positions, err := exc.GetPosition(sym)
	if err != nil {
		fmt.Printf("FAIL — %v\n", err)
		return
	}
	if positions == nil {
		// (nil, nil) can indicate a signing error that the adapter swallowed.
		fmt.Printf("WARN — (nil, nil): silent empty response; possible signing issue\n")
		return
	}
	fmt.Printf("OK — %d position(s) returned (0 is normal if no open position)\n", len(positions))
}
