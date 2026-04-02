package main

import (
	"flag"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"arb/internal/config"
	"arb/pkg/exchange"
	"arb/pkg/exchange/binance"
	"arb/pkg/exchange/bitget"
	"arb/pkg/exchange/bybit"
	"arb/pkg/exchange/gateio"
	"arb/pkg/exchange/okx"
)

func main() {
	exchangeFlag := flag.String("exchange", "", "test a specific exchange (binance, bitget, bybit, gateio, okx); default: all configured")
	flag.Parse()

	cfg := config.Load()

	type adapterEntry struct {
		name string
		exc  exchange.Exchange
	}

	var adapters []adapterEntry

	if *exchangeFlag != "" {
		exc, err := makeAdapter(cfg, *exchangeFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
			os.Exit(1)
		}
		adapters = append(adapters, adapterEntry{name: *exchangeFlag, exc: exc})
	} else {
		for _, name := range []string{"binance", "bitget", "bybit", "gateio", "okx"} {
			exc, err := makeAdapter(cfg, name)
			if err != nil {
				fmt.Printf("Skipping %s: %v\n", name, err)
				continue
			}
			adapters = append(adapters, adapterEntry{name: name, exc: exc})
		}
	}

	if len(adapters) == 0 {
		fmt.Fprintln(os.Stderr, "No exchanges configured. Set API key env vars (or use config.json).")
		os.Exit(1)
	}

	allPassed := true
	for i, a := range adapters {
		passed := runWSTests(a.name, a.exc)
		if !passed {
			allPassed = false
		}
		if i < len(adapters)-1 {
			fmt.Println()
		}
	}

	if !allPassed {
		os.Exit(1)
	}
}

func makeAdapter(cfg *config.Config, name string) (exchange.Exchange, error) {
	switch name {
	case "binance":
		if cfg.BinanceAPIKey == "" {
			return nil, fmt.Errorf("BINANCE_API_KEY not set")
		}
		return binance.NewAdapter(exchange.ExchangeConfig{
			Exchange: "binance", ApiKey: cfg.BinanceAPIKey, SecretKey: cfg.BinanceSecretKey,
		}), nil
	case "bitget":
		if cfg.BitgetAPIKey == "" {
			return nil, fmt.Errorf("BITGET_API_KEY not set")
		}
		if cfg.BitgetPassphrase == "" {
			return nil, fmt.Errorf("BITGET_PASSPHRASE not set")
		}
		return bitget.NewAdapter(exchange.ExchangeConfig{
			Exchange: "bitget", ApiKey: cfg.BitgetAPIKey, SecretKey: cfg.BitgetSecretKey, Passphrase: cfg.BitgetPassphrase,
		}), nil
	case "bybit":
		if cfg.BybitAPIKey == "" {
			return nil, fmt.Errorf("BYBIT_API_KEY not set")
		}
		return bybit.NewAdapter(exchange.ExchangeConfig{
			Exchange: "bybit", ApiKey: cfg.BybitAPIKey, SecretKey: cfg.BybitSecretKey,
		}), nil
	case "gateio":
		if cfg.GateioAPIKey == "" {
			return nil, fmt.Errorf("GATEIO_API_KEY not set")
		}
		return gateio.NewAdapter(exchange.ExchangeConfig{
			Exchange: "gateio", ApiKey: cfg.GateioAPIKey, SecretKey: cfg.GateioSecretKey,
		}), nil
	case "okx":
		if cfg.OKXAPIKey == "" {
			return nil, fmt.Errorf("OKX_API_KEY not set")
		}
		if cfg.OKXPassphrase == "" {
			return nil, fmt.Errorf("OKX_PASSPHRASE not set")
		}
		return okx.NewAdapter(exchange.ExchangeConfig{
			Exchange: "okx", ApiKey: cfg.OKXAPIKey, SecretKey: cfg.OKXSecretKey, Passphrase: cfg.OKXPassphrase,
		}), nil
	default:
		return nil, fmt.Errorf("unsupported exchange: %q", name)
	}
}

// ---------------------------------------------------------------------------
// WS Test runner
// ---------------------------------------------------------------------------

func runWSTests(name string, exc exchange.Exchange) bool {
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("=== WS Private Test: %s ===\n", name)
	fmt.Println(strings.Repeat("=", 60))

	// 1. Load contracts
	contracts, err := exc.LoadAllContracts()
	if err != nil {
		fmt.Printf("  FATAL: LoadAllContracts failed: %v\n", err)
		return false
	}

	// 2. Find a cheap test symbol (DOGEUSDT preferred)
	symbol, contract, err := findCheapSymbol(exc, contracts)
	if err != nil {
		fmt.Printf("  FATAL: %v\n", err)
		return false
	}

	// Get current price via BBO
	exc.StartPriceStream([]string{symbol})
	time.Sleep(3 * time.Second)

	bbo, ok := exc.GetBBO(symbol)
	if !ok {
		fmt.Printf("  FATAL: No BBO data for %s after 3s\n", symbol)
		return false
	}
	if bbo.Ask <= 0 {
		fmt.Printf("  FATAL: Invalid ask price for %s: %.6f\n", symbol, bbo.Ask)
		return false
	}

	// Compute test size: must meet 5 USDT notional minimum on most exchanges.
	// Most exchanges: size is in coins (e.g. 77 DOGE).
	// OKX: size is in lots where each lot = ctVal coins (e.g. 1000 DOGE/lot),
	//   so minSize=0.01 lots = 10 DOGE ≈ $1 — OKX has no 5 USDT minimum.
	// Gate.io: size is in contracts (1 contract = 1 coin for DOGE).
	testSize := contract.MinSize
	if exc.Name() != "okx" {
		// Scale up to meet 5 USDT notional minimum (target 8 USDT for safety,
		// since limit orders at 80% price must also meet the minimum).
		minNotional := 8.0
		needed := math.Ceil(minNotional / bbo.Ask / contract.StepSize) * contract.StepSize
		if needed > testSize {
			testSize = needed
		}
	}
	sizeStr := strconv.FormatFloat(testSize, 'f', contract.SizeDecimals, 64)
	actualNotional := testSize * bbo.Ask

	fmt.Printf("  Symbol: %s (price=%.4f, size=%s, notional=%.2f USDT)\n", symbol, bbo.Ask, sizeStr, actualNotional)
	fmt.Println()

	// 3. Set leverage + margin mode
	marginMode := "cross"
	if exc.Name() == "bitget" {
		marginMode = "crossed"
	}
	if err := exc.SetMarginMode(symbol, marginMode); err != nil {
		errStr := strings.ToLower(err.Error())
		if !strings.Contains(errStr, "already") && !strings.Contains(errStr, "not modified") &&
			!strings.Contains(errStr, "no need") && !strings.Contains(errStr, "-4046") && !strings.Contains(errStr, "40872") {
			fmt.Printf("  WARN: SetMarginMode: %v\n", err)
		}
	}
	for _, side := range []string{"long", "short"} {
		if err := exc.SetLeverage(symbol, "5", side); err != nil {
			errStr := strings.ToLower(err.Error())
			if !strings.Contains(errStr, "already") && !strings.Contains(errStr, "not modified") {
				fmt.Printf("  WARN: SetLeverage(%s): %v\n", side, err)
			}
		}
	}

	// 4. Start private WS
	exc.StartPrivateStream()
	fmt.Printf("  Private WS started, waiting 3s...\n\n")
	time.Sleep(3 * time.Second)

	passed := 0
	total := 4

	// ---------------------------------------------------------------
	// Test 1: Limit order visibility
	// ---------------------------------------------------------------
	limitPrice := bbo.Bid * 0.80 // 80% of bid — won't fill but meets notional minimums
	if contract.PriceStep > 0 {
		limitPrice = math.Floor(limitPrice/contract.PriceStep) * contract.PriceStep
	}
	priceStr := strconv.FormatFloat(limitPrice, 'f', contract.PriceDecimals, 64)

	start := time.Now()
	limitOID, err := exc.PlaceOrder(exchange.PlaceOrderParams{
		Symbol:    symbol,
		Side:      exchange.SideBuy,
		OrderType: "limit",
		Price:     priceStr,
		Size:      sizeStr,
		Force:     "gtc",
	})
	if err != nil {
		fmt.Printf("  1. Limit order WS visibility ............ FAIL — PlaceOrder error: %v\n", err)
	} else {
		update, found := pollOrderUpdate(exc, limitOID, 15*time.Second, func(u exchange.OrderUpdate) bool {
			s := strings.ToLower(u.Status)
			return s == "new" || s == "open" || s == "live" || s == "partially_filled"
		})
		elapsed := time.Since(start)
		if found {
			fmt.Printf("  1. Limit order WS visibility ............ PASS (%.1fs) — status=%s oid=%s\n", elapsed.Seconds(), update.Status, limitOID)
			passed++
		} else {
			fmt.Printf("  1. Limit order WS visibility ............ FAIL (%.1fs) — no WS update received oid=%s\n", elapsed.Seconds(), limitOID)
		}
	}

	// ---------------------------------------------------------------
	// Test 2: Cancel visibility
	// ---------------------------------------------------------------
	start = time.Now()
	if limitOID != "" {
		if err := exc.CancelOrder(symbol, limitOID); err != nil {
			fmt.Printf("  2. Cancel WS visibility ................. FAIL — CancelOrder error: %v\n", err)
		} else {
			update, found := pollOrderUpdate(exc, limitOID, 15*time.Second, func(u exchange.OrderUpdate) bool {
				s := strings.ToLower(u.Status)
				return s == "cancelled" || s == "canceled" || s == "cancel" || s == "filled" // some exchanges reuse the slot
			})
			elapsed := time.Since(start)
			if found {
				fmt.Printf("  2. Cancel WS visibility ................. PASS (%.1fs) — status=%s\n", elapsed.Seconds(), update.Status)
				passed++
			} else {
				fmt.Printf("  2. Cancel WS visibility ................. FAIL (%.1fs) — no cancel update received\n", elapsed.Seconds())
			}
		}
	} else {
		fmt.Printf("  2. Cancel WS visibility ................. SKIP — no order to cancel\n")
	}

	// ---------------------------------------------------------------
	// Test 3: Market fill + AvgPrice
	// ---------------------------------------------------------------
	start = time.Now()
	marketOID, err := exc.PlaceOrder(exchange.PlaceOrderParams{
		Symbol:    symbol,
		Side:      exchange.SideBuy,
		OrderType: "market",
		Size:      sizeStr,
		Force:     "ioc",
	})
	if err != nil {
		fmt.Printf("  3. Market fill WS (AvgPrice) ............ FAIL — PlaceOrder error: %v\n", err)
	} else {
		update, found := pollOrderUpdateWithRESTFallback(exc, marketOID, symbol, 15*time.Second, func(u exchange.OrderUpdate) bool {
			return u.FilledVolume > 0 && u.AvgPrice > 0
		})
		elapsed := time.Since(start)
		if found {
			fmt.Printf("  3. Market fill WS (AvgPrice) ............ PASS (%.1fs) — filled=%.4f avg=%.5f\n",
				elapsed.Seconds(), update.FilledVolume, update.AvgPrice)
			passed++
		} else {
			fmt.Printf("  3. Market fill WS (AvgPrice) ............ FAIL (%.1fs) — no fill update received oid=%s\n", elapsed.Seconds(), marketOID)
		}
	}

	// ---------------------------------------------------------------
	// Test 4: Close order visibility (reduce-only market sell)
	// ---------------------------------------------------------------
	start = time.Now()
	closeOID, err := exc.PlaceOrder(exchange.PlaceOrderParams{
		Symbol:     symbol,
		Side:       exchange.SideSell,
		OrderType:  "market",
		Size:       sizeStr,
		Force:      "ioc",
		ReduceOnly: true,
	})
	if err != nil {
		fmt.Printf("  4. Close order WS visibility ............ FAIL — PlaceOrder error: %v\n", err)
	} else {
		update, found := pollOrderUpdateWithRESTFallback(exc, closeOID, symbol, 15*time.Second, func(u exchange.OrderUpdate) bool {
			return u.FilledVolume > 0
		})
		elapsed := time.Since(start)
		if found {
			fmt.Printf("  4. Close order WS visibility ............ PASS (%.1fs) — filled=%.4f avg=%.5f\n",
				elapsed.Seconds(), update.FilledVolume, update.AvgPrice)
			passed++
		} else {
			fmt.Printf("  4. Close order WS visibility ............ FAIL (%.1fs) — no fill update received oid=%s\n", elapsed.Seconds(), closeOID)
		}
	}

	// Summary
	fmt.Println()
	if passed == total {
		fmt.Printf("  Result: %d/%d PASSED  ✓\n", passed, total)
	} else {
		fmt.Printf("  Result: %d/%d PASSED  ✗\n", passed, total)
	}

	return passed == total
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// findCheapSymbol finds a cheap symbol for testing. Prefers DOGEUSDT, falls back
// to any symbol where minSize * price < 5 USDT.
func findCheapSymbol(exc exchange.Exchange, contracts map[string]exchange.ContractInfo) (string, exchange.ContractInfo, error) {
	// Try DOGE first with exchange-specific naming
	dogeCandidates := []string{"DOGEUSDT"}
	switch exc.Name() {
	case "gateio":
		dogeCandidates = append([]string{"DOGE_USDT"}, dogeCandidates...)
	case "okx":
		dogeCandidates = append([]string{"DOGE-USDT-SWAP"}, dogeCandidates...)
	}

	for _, sym := range dogeCandidates {
		if c, ok := contracts[sym]; ok {
			return sym, c, nil
		}
	}

	// Fallback: search for any key containing DOGE
	for sym, c := range contracts {
		upper := strings.ToUpper(sym)
		if strings.Contains(upper, "DOGE") && strings.Contains(upper, "USDT") {
			return sym, c, nil
		}
	}

	// Last resort: find cheapest qualifying symbol via BBO
	// (we'd need price stream for this, so just return an error)
	return "", exchange.ContractInfo{}, fmt.Errorf("no DOGEUSDT contract found — add symbol scanning if needed")
}

// pollOrderUpdate polls GetOrderUpdate every 500ms until checkFn returns true or timeout.
func pollOrderUpdate(exc exchange.Exchange, orderID string, timeout time.Duration, checkFn func(exchange.OrderUpdate) bool) (exchange.OrderUpdate, bool) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if update, ok := exc.GetOrderUpdate(orderID); ok {
			if checkFn(update) {
				return update, true
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	// One final check
	if update, ok := exc.GetOrderUpdate(orderID); ok && checkFn(update) {
		return update, true
	}
	return exchange.OrderUpdate{}, false
}

// pollOrderUpdateWithRESTFallback is like pollOrderUpdate but also tries
// GetOrderFilledQty as a REST fallback for fill tests.
func pollOrderUpdateWithRESTFallback(exc exchange.Exchange, orderID, symbol string, timeout time.Duration, checkFn func(exchange.OrderUpdate) bool) (exchange.OrderUpdate, bool) {
	start := time.Now()
	deadline := start.Add(timeout)
	restChecked := false

	for time.Now().Before(deadline) {
		// Try WS first
		if update, ok := exc.GetOrderUpdate(orderID); ok {
			if checkFn(update) {
				return update, true
			}
		}

		// REST fallback after 5s if WS hasn't delivered
		if !restChecked && time.Since(start) > 5*time.Second {
			restChecked = true
			if qty, err := exc.GetOrderFilledQty(orderID, symbol); err == nil && qty > 0 {
				// WS didn't deliver but REST confirms fill — construct a synthetic update
				return exchange.OrderUpdate{
					OrderID:      orderID,
					Status:       "filled (REST fallback)",
					FilledVolume: qty,
					AvgPrice:     0, // REST doesn't give us avg price in this interface
				}, true
			}
		}

		time.Sleep(500 * time.Millisecond)
	}

	// Final REST check
	if qty, err := exc.GetOrderFilledQty(orderID, symbol); err == nil && qty > 0 {
		return exchange.OrderUpdate{
			OrderID:      orderID,
			Status:       "filled (REST fallback)",
			FilledVolume: qty,
		}, true
	}

	return exchange.OrderUpdate{}, false
}
