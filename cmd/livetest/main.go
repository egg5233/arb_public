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
	"arb/pkg/exchange/bingx"
	"arb/pkg/exchange/bitget"
	"arb/pkg/exchange/bybit"
	"arb/pkg/exchange/gateio"
	"arb/pkg/exchange/okx"
)

const totalTests = 28

func main() {
	exchangeFlag := flag.String("exchange", "", "test a specific exchange (binance, bitget, bybit, gateio, okx); default: all configured")
	skipOrders := flag.Bool("skip-orders", false, "skip order placement/cancel tests")
	testMargin := flag.Bool("test-margin", false, "run spot margin borrow/trade tests (requires margin account enabled, uses real funds)")
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
		for _, name := range []string{"binance", "bitget", "bybit", "gateio", "okx", "bingx"} {
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
		fmt.Fprintln(os.Stderr, "Supported: BINANCE_API_KEY, BITGET_API_KEY, BYBIT_API_KEY, GATEIO_API_KEY, OKX_API_KEY, BINGX_API_KEY")
		os.Exit(1)
	}

	allPassed := true
	for _, a := range adapters {
		passed := runTests(a.name, a.exc, *skipOrders, *testMargin)
		if !passed {
			allPassed = false
		}
		fmt.Println()
	}

	if !allPassed {
		os.Exit(1)
	}
}

func makeAdapter(cfg *config.Config, name string) (exchange.Exchange, error) {
	switch name {
	case "binance":
		if cfg.BinanceAPIKey == "" {
			return nil, fmt.Errorf("BINANCE_API_KEY not set — set env var or add to config.json")
		}
		return binance.NewAdapter(exchange.ExchangeConfig{
			Exchange:  "binance",
			ApiKey:    cfg.BinanceAPIKey,
			SecretKey: cfg.BinanceSecretKey,
		}), nil
	case "bitget":
		if cfg.BitgetAPIKey == "" {
			return nil, fmt.Errorf("BITGET_API_KEY not set — set env var or add to config.json")
		}
		if cfg.BitgetPassphrase == "" {
			return nil, fmt.Errorf("BITGET_PASSPHRASE not set — required for Bitget API signing")
		}
		return bitget.NewAdapter(exchange.ExchangeConfig{
			Exchange:   "bitget",
			ApiKey:     cfg.BitgetAPIKey,
			SecretKey:  cfg.BitgetSecretKey,
			Passphrase: cfg.BitgetPassphrase,
		}), nil
	case "bybit":
		if cfg.BybitAPIKey == "" {
			return nil, fmt.Errorf("BYBIT_API_KEY not set — set env var or add to config.json")
		}
		return bybit.NewAdapter(exchange.ExchangeConfig{
			Exchange:  "bybit",
			ApiKey:    cfg.BybitAPIKey,
			SecretKey: cfg.BybitSecretKey,
		}), nil
	case "gateio":
		if cfg.GateioAPIKey == "" {
			return nil, fmt.Errorf("GATEIO_API_KEY not set — set env var or add to config.json")
		}
		ga := gateio.NewAdapter(exchange.ExchangeConfig{
			Exchange:  "gateio",
			ApiKey:    cfg.GateioAPIKey,
			SecretKey: cfg.GateioSecretKey,
		})
		ga.DetectUnifiedMode()
		return ga, nil
	case "okx":
		if cfg.OKXAPIKey == "" {
			return nil, fmt.Errorf("OKX_API_KEY not set — set env var or add to config.json")
		}
		if cfg.OKXPassphrase == "" {
			return nil, fmt.Errorf("OKX_PASSPHRASE not set — required for OKX API signing")
		}
		return okx.NewAdapter(exchange.ExchangeConfig{
			Exchange:   "okx",
			ApiKey:     cfg.OKXAPIKey,
			SecretKey:  cfg.OKXSecretKey,
			Passphrase: cfg.OKXPassphrase,
		}), nil
	case "bingx":
		if cfg.BingXAPIKey == "" {
			return nil, fmt.Errorf("BINGX_API_KEY not set — set env var or add to config.json")
		}
		return bingx.NewAdapter(exchange.ExchangeConfig{
			Exchange:  "bingx",
			ApiKey:    cfg.BingXAPIKey,
			SecretKey: cfg.BingXSecretKey,
		}), nil
	default:
		return nil, fmt.Errorf("unsupported exchange: %q (supported: binance, bitget, bybit, gateio, okx, bingx)", name)
	}
}

// ---------------------------------------------------------------------------
// Test runner
// ---------------------------------------------------------------------------

type testResult struct {
	name    string
	pass    bool
	elapsed time.Duration
	detail  string
}

// runTests returns true if all tests passed.
func runTests(name string, exc exchange.Exchange, skipOrders bool, testMargin bool) bool {
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("=== Testing %s ===\n", name)
	fmt.Println(strings.Repeat("=", 60))

	var results []testResult

	// We need the contract map and a test symbol for subsequent tests.
	var contracts map[string]exchange.ContractInfo
	var btcSymbol string
	var btcContract exchange.ContractInfo

	// ---------------------------------------------------------------
	// 1. LoadAllContracts
	// ---------------------------------------------------------------
	res := runTest("1. LoadAllContracts", func() (string, bool) {
		var err error
		contracts, err = exc.LoadAllContracts()
		if err != nil {
			return fmt.Sprintf("ERROR: %v", err), false
		}
		// Find BTC contract — try common keys
		for _, candidate := range []string{"BTCUSDT", "SBTCUSDT_UMCBL", "SBTCUSDT"} {
			if c, ok := contracts[candidate]; ok {
				btcSymbol = candidate
				btcContract = c
				break
			}
		}
		// Fallback: search for any key containing BTC
		if btcSymbol == "" {
			for sym, c := range contracts {
				if strings.Contains(strings.ToUpper(sym), "BTC") && strings.Contains(strings.ToUpper(sym), "USDT") {
					btcSymbol = sym
					btcContract = c
					break
				}
			}
		}

		detail := fmt.Sprintf("Contracts loaded: %d", len(contracts))
		if btcSymbol != "" {
			detail += fmt.Sprintf("\n   BTC symbol:   %s", btcSymbol)
			detail += fmt.Sprintf("\n   Min size:     %v  Step: %v  Max: %v", btcContract.MinSize, btcContract.StepSize, btcContract.MaxSize)
			detail += fmt.Sprintf("\n   Price decimals: %d  Price step: %v", btcContract.PriceDecimals, btcContract.PriceStep)
		} else {
			detail += "\n   WARNING: no BTC contract found"
		}
		return detail, len(contracts) > 0
	})
	results = append(results, res)

	if btcSymbol == "" {
		fmt.Println("Cannot continue without a BTC symbol. Aborting remaining tests.")
		printSummary(name, results, totalTests)
		return false
	}

	// ---------------------------------------------------------------
	// 2. GetFuturesBalance
	// ---------------------------------------------------------------
	res = runTest("2. GetFuturesBalance", func() (string, bool) {
		bal, err := exc.GetFuturesBalance()
		if err != nil {
			return fmt.Sprintf("ERROR: %v", err), false
		}
		return fmt.Sprintf("Available USDT: %.4f  (Total: %.4f  Frozen: %.4f)", bal.Available, bal.Total, bal.Frozen), true
	})
	results = append(results, res)

	// ---------------------------------------------------------------
	// 3. GetSpotBalance
	// ---------------------------------------------------------------
	res = runTest("3. GetSpotBalance", func() (string, bool) {
		bal, err := exc.GetSpotBalance()
		if err != nil {
			return fmt.Sprintf("ERROR: %v", err), false
		}
		return fmt.Sprintf("Spot Available USDT: %.4f  (Total: %.4f  Frozen: %.4f)", bal.Available, bal.Total, bal.Frozen), true
	})
	results = append(results, res)

	// ---------------------------------------------------------------
	// 4. GetFundingRate
	// ---------------------------------------------------------------
	res = runTest("4. GetFundingRate("+btcSymbol+")", func() (string, bool) {
		fr, err := exc.GetFundingRate(btcSymbol)
		if err != nil {
			return fmt.Sprintf("ERROR: %v", err), false
		}
		detail := fmt.Sprintf("Current rate: %.6f%%", fr.Rate*100)
		detail += fmt.Sprintf("  Next rate: %.6f%%", fr.NextRate*100)
		detail += fmt.Sprintf("  Interval: %s", fr.Interval)
		detail += fmt.Sprintf("  Next funding: %s", fr.NextFunding.Format(time.RFC3339))
		return detail, true
	})
	results = append(results, res)

	// ---------------------------------------------------------------
	// 5. GetFundingInterval
	// ---------------------------------------------------------------
	res = runTest("5. GetFundingInterval("+btcSymbol+")", func() (string, bool) {
		interval, err := exc.GetFundingInterval(btcSymbol)
		if err != nil {
			return fmt.Sprintf("ERROR: %v", err), false
		}
		// Validate interval is a reasonable value (1h to 24h)
		if interval < time.Hour || interval > 24*time.Hour {
			return fmt.Sprintf("Interval %s looks unusual (expected 1h-24h)", interval), false
		}
		return fmt.Sprintf("Interval: %s", interval), true
	})
	results = append(results, res)

	// ---------------------------------------------------------------
	// 6. GetOrderbook
	// ---------------------------------------------------------------
	var bestAsk float64
	res = runTest("6. GetOrderbook("+btcSymbol+", 5)", func() (string, bool) {
		ob, err := exc.GetOrderbook(btcSymbol, 5)
		if err != nil {
			return fmt.Sprintf("ERROR: %v", err), false
		}
		if len(ob.Bids) == 0 || len(ob.Asks) == 0 {
			return "Empty orderbook — no bids or asks returned", false
		}
		bestBid := ob.Bids[0].Price
		bestAsk = ob.Asks[0].Price
		spread := bestAsk - bestBid
		spreadBps := (spread / bestBid) * 10000
		detail := fmt.Sprintf("Best bid: %.2f  Best ask: %.2f  Spread: %.2f (%.2f bps)", bestBid, bestAsk, spread, spreadBps)
		detail += fmt.Sprintf("\n   Bid levels: %d  Ask levels: %d", len(ob.Bids), len(ob.Asks))
		return detail, true
	})
	results = append(results, res)

	// ---------------------------------------------------------------
	// 7. StartPriceStream + GetBBO
	// ---------------------------------------------------------------
	res = runTest("7. StartPriceStream + GetBBO", func() (string, bool) {
		exc.StartPriceStream([]string{btcSymbol})
		fmt.Printf("   Waiting 5s for WebSocket data...\n")
		time.Sleep(5 * time.Second)

		bbo, ok := exc.GetBBO(btcSymbol)
		if !ok {
			return "No BBO data received from WebSocket — check WS connection and symbol format", false
		}
		return fmt.Sprintf("WS Bid: %.2f  WS Ask: %.2f", bbo.Bid, bbo.Ask), bbo.Bid > 0 && bbo.Ask > 0
	})
	results = append(results, res)

	// ---------------------------------------------------------------
	// 8. SubscribeSymbol (dynamic subscribe on existing WS)
	// ---------------------------------------------------------------
	res = runTest("8. SubscribeSymbol (ETH)", func() (string, bool) {
		// Find an ETH symbol to subscribe dynamically — prefer exact ETHUSDT
		var ethSymbol string
		for _, candidate := range []string{"ETHUSDT", "ETH_USDT", "ETH-USDT-SWAP"} {
			if _, ok := contracts[candidate]; ok {
				ethSymbol = candidate
				break
			}
		}
		if ethSymbol == "" {
			// Fallback: search for any key starting with ETH and containing USDT
			for sym := range contracts {
				upper := strings.ToUpper(sym)
				if strings.HasPrefix(upper, "ETH") && strings.Contains(upper, "USDT") && !strings.Contains(upper, "ETHFI") && !strings.Contains(upper, "ETHW") {
					ethSymbol = sym
					break
				}
			}
		}
		if ethSymbol == "" {
			return "No ETH contract found to test dynamic subscribe", false
		}

		ok := exc.SubscribeSymbol(ethSymbol)
		if !ok {
			return fmt.Sprintf("SubscribeSymbol(%s) returned false — WS may not be connected", ethSymbol), false
		}
		fmt.Printf("   Subscribed to %s, waiting 3s for data...\n", ethSymbol)
		time.Sleep(3 * time.Second)

		bbo, got := exc.GetBBO(ethSymbol)
		if !got {
			return fmt.Sprintf("Subscribed to %s but no BBO data arrived after 3s", ethSymbol), false
		}
		return fmt.Sprintf("Dynamic subscribe OK: %s Bid=%.2f Ask=%.2f", ethSymbol, bbo.Bid, bbo.Ask), bbo.Bid > 0 && bbo.Ask > 0
	})
	results = append(results, res)

	// ---------------------------------------------------------------
	// 9. SetMarginMode
	// ---------------------------------------------------------------
	res = runTest("9. SetMarginMode("+btcSymbol+", cross)", func() (string, bool) {
		marginMode := "cross"
		// Bitget uses "crossed"
		if exc.Name() == "bitget" {
			marginMode = "crossed"
		}
		err := exc.SetMarginMode(btcSymbol, marginMode)
		if err != nil {
			errStr := err.Error()
			// Treat "already set" responses as success
			if strings.Contains(strings.ToLower(errStr), "already") ||
				strings.Contains(errStr, "-4046") ||
				strings.Contains(errStr, "40872") ||
				strings.Contains(errStr, "margin mode is not modified") ||
				strings.Contains(strings.ToLower(errStr), "no need to change") {
				return "Already set to cross (OK)", true
			}
			return fmt.Sprintf("ERROR: %v", err), false
		}
		return "Margin mode set to cross", true
	})
	results = append(results, res)

	// ---------------------------------------------------------------
	// 10. SetLeverage
	// ---------------------------------------------------------------
	res = runTest("10. SetLeverage("+btcSymbol+", 3)", func() (string, bool) {
		err := exc.SetLeverage(btcSymbol, "3", "long")
		if err != nil {
			errStr := err.Error()
			if !strings.Contains(strings.ToLower(errStr), "already") &&
				!strings.Contains(strings.ToLower(errStr), "not modified") {
				return fmt.Sprintf("ERROR (long): %v", err), false
			}
		}
		err = exc.SetLeverage(btcSymbol, "3", "short")
		if err != nil {
			errStr := err.Error()
			if !strings.Contains(strings.ToLower(errStr), "already") &&
				!strings.Contains(strings.ToLower(errStr), "not modified") {
				return fmt.Sprintf("ERROR (short): %v", err), false
			}
		}
		return "Leverage set to 3x for both sides", true
	})
	results = append(results, res)

	// ---------------------------------------------------------------
	// 11-13: Order tests (skip if --skip-orders)
	// ---------------------------------------------------------------
	var testOrderID string

	if skipOrders {
		for _, n := range []string{
			"11. PlaceOrder (limit buy far below market)",
			"12. GetPendingOrders + GetOrderFilledQty",
			"13. CancelOrder",
		} {
			r := testResult{name: n, pass: false, detail: "SKIPPED (--skip-orders)"}
			fmt.Printf("\n--- %s ---\n", n)
			fmt.Printf("   SKIPPED (--skip-orders)\n")
			results = append(results, r)
		}
	} else {
		// 11. PlaceOrder
		res = runTest("11. PlaceOrder (limit buy far below market)", func() (string, bool) {
			if bestAsk == 0 {
				return "No ask price available from orderbook — run orderbook test first", false
			}

			// Price 5% below current ask — should never fill
			safePrice := bestAsk * 0.95
			// Round to exchange price step
			if btcContract.PriceStep > 0 {
				safePrice = math.Floor(safePrice/btcContract.PriceStep) * btcContract.PriceStep
			}
			priceStr := strconv.FormatFloat(safePrice, 'f', btcContract.PriceDecimals, 64)

			// Ensure order notional meets exchange minimum (Binance requires >= 100 USDT).
			minOrderSize := btcContract.MinSize
			if safePrice > 0 {
				minNotional := 110.0 // 110 USDT to have margin above 100 minimum
				minSizeForNotional := minNotional / safePrice
				if btcContract.StepSize > 0 {
					minSizeForNotional = math.Ceil(minSizeForNotional/btcContract.StepSize) * btcContract.StepSize
				}
				if minSizeForNotional > minOrderSize {
					minOrderSize = minSizeForNotional
				}
			}
			sizeStr := strconv.FormatFloat(minOrderSize, 'f', btcContract.SizeDecimals, 64)

			fmt.Printf("   Placing limit buy: price=%s size=%s\n", priceStr, sizeStr)

			oid, err := exc.PlaceOrder(exchange.PlaceOrderParams{
				Symbol:    btcSymbol,
				Side:      exchange.SideBuy,
				OrderType: "limit",
				Price:     priceStr,
				Size:      sizeStr,
				Force:     "gtc",
			})
			if err != nil {
				return fmt.Sprintf("ERROR: %v", err), false
			}
			testOrderID = oid
			return fmt.Sprintf("Order ID: %s", oid), oid != ""
		})
		results = append(results, res)

		// 12. GetPendingOrders + GetOrderFilledQty
		res = runTest("12. GetPendingOrders + GetOrderFilledQty("+btcSymbol+")", func() (string, bool) {
			orders, err := exc.GetPendingOrders(btcSymbol)
			if err != nil {
				return fmt.Sprintf("ERROR (GetPendingOrders): %v", err), false
			}
			found := false
			for _, o := range orders {
				if o.OrderID == testOrderID {
					found = true
					break
				}
			}
			detail := fmt.Sprintf("Pending orders: %d", len(orders))
			if testOrderID != "" {
				if found {
					detail += fmt.Sprintf("  (our order %s FOUND)", testOrderID)
				} else {
					detail += fmt.Sprintf("  (our order %s NOT found — may have been rejected)", testOrderID)
				}
			}

			// Also test GetOrderFilledQty
			if testOrderID != "" {
				filledQty, err := exc.GetOrderFilledQty(testOrderID, btcSymbol)
				if err != nil {
					detail += fmt.Sprintf("\n   GetOrderFilledQty ERROR: %v", err)
				} else {
					detail += fmt.Sprintf("\n   GetOrderFilledQty: %.6f (expected 0 for unfilled order)", filledQty)
				}
			}

			return detail, found || testOrderID == ""
		})
		results = append(results, res)

		// 13. CancelOrder
		res = runTest("13. CancelOrder", func() (string, bool) {
			if testOrderID == "" {
				return "No order to cancel (PlaceOrder failed or was skipped)", false
			}
			err := exc.CancelOrder(btcSymbol, testOrderID)
			if err != nil {
				return fmt.Sprintf("ERROR: %v", err), false
			}
			return fmt.Sprintf("Cancelled order %s", testOrderID), true
		})
		results = append(results, res)
	}

	// ---------------------------------------------------------------
	// 14. GetPosition (single symbol)
	// ---------------------------------------------------------------
	res = runTest("14. GetPosition("+btcSymbol+")", func() (string, bool) {
		positions, err := exc.GetPosition(btcSymbol)
		if err != nil {
			return fmt.Sprintf("ERROR: %v", err), false
		}
		if len(positions) == 0 {
			return "No open positions (OK)", true
		}
		var parts []string
		for _, p := range positions {
			parts = append(parts, fmt.Sprintf("%s %s size=%s leverage=%s pnl=%s",
				p.Symbol, p.HoldSide, p.Total, p.Leverage, p.UnrealizedPL))
		}
		return strings.Join(parts, "\n   "), true
	})
	results = append(results, res)

	// ---------------------------------------------------------------
	// 15. GetAllPositions
	// ---------------------------------------------------------------
	res = runTest("15. GetAllPositions", func() (string, bool) {
		positions, err := exc.GetAllPositions()
		if err != nil {
			return fmt.Sprintf("ERROR: %v", err), false
		}
		if len(positions) == 0 {
			return "No open positions across any symbol (OK)", true
		}
		detail := fmt.Sprintf("Open positions: %d", len(positions))
		for _, p := range positions {
			detail += fmt.Sprintf("\n   %s %s size=%s leverage=%s",
				p.Symbol, p.HoldSide, p.Total, p.Leverage)
		}
		return detail, true
	})
	results = append(results, res)

	// ---------------------------------------------------------------
	// 16. StartPrivateStream + GetOrderUpdate
	// ---------------------------------------------------------------
	res = runTest("16. StartPrivateStream + GetOrderUpdate", func() (string, bool) {
		exc.StartPrivateStream()
		fmt.Printf("   Private WS connected\n")
		fmt.Printf("   Waiting 3s for connection to stabilize...\n")
		time.Sleep(3 * time.Second)

		// Try to read an order update for a fake order ID — should return false, not panic
		_, found := exc.GetOrderUpdate("nonexistent-order-id-12345")
		if found {
			return "GetOrderUpdate returned true for nonexistent order — unexpected", false
		}
		return "Private WS started, GetOrderUpdate correctly returns false for unknown orders", true
	})
	results = append(results, res)

	// ---------------------------------------------------------------
	// 17. GetPriceStore (verify sync.Map accessible)
	// ---------------------------------------------------------------
	res = runTest("17. GetPriceStore", func() (string, bool) {
		store := exc.GetPriceStore()
		if store == nil {
			return "GetPriceStore returned nil — price data will not be accessible", false
		}
		// Count entries in the price store
		count := 0
		store.Range(func(key, value interface{}) bool {
			count++
			return true
		})
		return fmt.Sprintf("PriceStore accessible, %d entries", count), true
	})
	results = append(results, res)

	// ---------------------------------------------------------------
	// 18. SubscribeDepth + GetDepth (top-5 orderbook via WS)
	// ---------------------------------------------------------------
	res = runTest("18. SubscribeDepth + GetDepth("+btcSymbol+")", func() (string, bool) {
		ok := exc.SubscribeDepth(btcSymbol)
		if !ok {
			return "SubscribeDepth returned false — WS may not be connected or already subscribed", false
		}
		fmt.Printf("   Subscribed to depth, waiting 3s for data...\n")
		time.Sleep(3 * time.Second)

		ob, got := exc.GetDepth(btcSymbol)
		if !got || ob == nil {
			return "No depth data received after 3s", false
		}
		if len(ob.Bids) == 0 || len(ob.Asks) == 0 {
			return fmt.Sprintf("Depth received but empty: bids=%d asks=%d", len(ob.Bids), len(ob.Asks)), false
		}

		detail := fmt.Sprintf("Depth OK: %d bids, %d asks", len(ob.Bids), len(ob.Asks))
		detail += fmt.Sprintf("\n   Best bid: %.2f (%.4f)  Best ask: %.2f (%.4f)",
			ob.Bids[0].Price, ob.Bids[0].Quantity, ob.Asks[0].Price, ob.Asks[0].Quantity)
		if len(ob.Bids) >= 5 && len(ob.Asks) >= 5 {
			detail += "\n   Full 5 levels on both sides"
		}

		// Test unsubscribe
		exc.UnsubscribeDepth(btcSymbol)
		_, stillGot := exc.GetDepth(btcSymbol)
		if stillGot {
			detail += "\n   WARNING: GetDepth still returns data after UnsubscribeDepth"
		} else {
			detail += "\n   UnsubscribeDepth cleared store correctly"
		}

		return detail, true
	})
	results = append(results, res)

	// ---------------------------------------------------------------
	// 19-22: Stop-Loss tests (open position → place SL → cancel SL → close)
	// ---------------------------------------------------------------
	if skipOrders {
		for _, n := range []string{
			"19. Open min-size long (for SL test)",
			"20. PlaceStopLoss",
			"21. CancelStopLoss",
			"22. Close position (cleanup)",
		} {
			r := testResult{name: n, pass: false, detail: "SKIPPED (--skip-orders)"}
			fmt.Printf("\n--- %s ---\n", n)
			fmt.Printf("   SKIPPED (--skip-orders)\n")
			results = append(results, r)
		}
	} else {
		var slPositionOpen bool
		var slOrderID string

		// Find a suitable symbol for SL test: need min notional < $20 USDT.
		// BTC might be too expensive (min 0.001 * ~70k = $70, Binance min notional $100).
		// Prefer a cheaper coin like DOGE, SOL, or just bump BTC size.
		slSymbol := btcSymbol
		slContract := btcContract
		slAsk := bestAsk

		// Try to find a cheaper symbol where minSize * price < $20.
		for _, candidate := range []string{"DOGEUSDT", "DOGE_USDT", "1000PEPEUSDT", "XRPUSDT"} {
			if c, ok := contracts[candidate]; ok {
				ob, err := exc.GetOrderbook(candidate, 5)
				if err == nil && len(ob.Asks) > 0 {
					minNotional := ob.Asks[0].Price * c.MinSize
					if minNotional > 0 && minNotional < 20 {
						slSymbol = candidate
						slContract = c
						slAsk = ob.Asks[0].Price
						break
					}
				}
			}
		}

		// If BTC, compute size to meet $110 notional.
		slSize := slContract.MinSize
		if slAsk*slSize < 100 && slAsk > 0 {
			// Need at least $110 notional.
			needed := math.Ceil(110/(slAsk*slContract.StepSize)) * slContract.StepSize
			if needed > slSize {
				slSize = needed
			}
		}

		// 19. Open a min-size long position via market order.
		res = runTest("19. Open min-size long (for SL test)", func() (string, bool) {
			if slAsk == 0 {
				return "No ask price — orderbook test must pass first", false
			}
			sizeStr := strconv.FormatFloat(slSize, 'f', slContract.SizeDecimals, 64)
			notional := slAsk * slSize

			fmt.Printf("   Opening long: market buy %s %s (~$%.2f)\n", sizeStr, slSymbol, notional)
			oid, err := exc.PlaceOrder(exchange.PlaceOrderParams{
				Symbol:    slSymbol,
				Side:      exchange.SideBuy,
				OrderType: "market",
				Size:      sizeStr,
				Force:     "ioc",
			})
			if err != nil {
				return fmt.Sprintf("ERROR: %v", err), false
			}
			// Wait briefly for fill.
			time.Sleep(2 * time.Second)
			filled, _ := exc.GetOrderFilledQty(oid, slSymbol)
			if filled <= 0 {
				return fmt.Sprintf("Market buy placed (oid=%s) but fill not confirmed", oid), false
			}
			slPositionOpen = true
			return fmt.Sprintf("Long opened on %s: oid=%s filled=%.6f (~$%.2f)", slSymbol, oid, filled, filled*slAsk), true
		})
		results = append(results, res)

		// 20. PlaceStopLoss — trigger well below market so it won't fire.
		res = runTest("20. PlaceStopLoss", func() (string, bool) {
			if !slPositionOpen {
				return "No open position — skipping", false
			}
			// SL at 30% below current ask — should never trigger.
			triggerPrice := slAsk * 0.70
			if slContract.PriceStep > 0 {
				triggerPrice = math.Floor(triggerPrice/slContract.PriceStep) * slContract.PriceStep
			}
			tp := strconv.FormatFloat(triggerPrice, 'f', slContract.PriceDecimals, 64)
			sizeStr := strconv.FormatFloat(slSize, 'f', slContract.SizeDecimals, 64)

			fmt.Printf("   Placing SL: sell %s trigger=%s size=%s\n", slSymbol, tp, sizeStr)
			oid, err := exc.PlaceStopLoss(exchange.StopLossParams{
				Symbol:       slSymbol,
				Side:         exchange.SideSell,
				Size:         sizeStr,
				TriggerPrice: tp,
			})
			if err != nil {
				return fmt.Sprintf("ERROR: %v", err), false
			}
			slOrderID = oid
			return fmt.Sprintf("SL placed: orderID=%s trigger=%s", oid, tp), oid != ""
		})
		results = append(results, res)

		// 21. CancelStopLoss.
		res = runTest("21. CancelStopLoss", func() (string, bool) {
			if slOrderID == "" {
				return "No SL order to cancel — skipping", false
			}
			err := exc.CancelStopLoss(slSymbol, slOrderID)
			if err != nil {
				return fmt.Sprintf("ERROR: %v", err), false
			}
			return fmt.Sprintf("SL cancelled: %s", slOrderID), true
		})
		results = append(results, res)

		// 22. Close position (cleanup).
		res = runTest("22. Close position (cleanup)", func() (string, bool) {
			if !slPositionOpen {
				return "No position to close — skipping", true
			}
			sizeStr := strconv.FormatFloat(slSize, 'f', slContract.SizeDecimals, 64)
			fmt.Printf("   Closing long: market sell %s %s\n", sizeStr, slSymbol)
			oid, err := exc.PlaceOrder(exchange.PlaceOrderParams{
				Symbol:     slSymbol,
				Side:       exchange.SideSell,
				OrderType:  "market",
				Size:       sizeStr,
				Force:      "ioc",
				ReduceOnly: true,
			})
			if err != nil {
				return fmt.Sprintf("ERROR: %v", err), false
			}
			time.Sleep(2 * time.Second)
			return fmt.Sprintf("Position closed: oid=%s", oid), true
		})
		results = append(results, res)
	}

	// ---------------------------------------------------------------
	// 23-26: Spot Margin tests
	// ---------------------------------------------------------------

	// Check if this exchange supports SpotMarginExchange
	marginExc, hasMargin := exc.(exchange.SpotMarginExchange)

	if !hasMargin {
		// Exchange doesn't support spot margin (e.g. BingX)
		for _, n := range []string{
			"23. GetMarginInterestRate (BTC)",
			"24. GetMarginBalance (USDT)",
			"25. MarginBorrow + MarginRepay (min amount)",
			"26. TransferToMargin + TransferFromMargin",
		} {
			r := testResult{name: n, pass: false, detail: "SKIPPED (exchange does not support spot margin)"}
			fmt.Printf("\n--- %s ---\n", n)
			fmt.Printf("   SKIPPED (exchange does not support spot margin)\n")
			results = append(results, r)
		}
	} else {
		// 23. GetMarginInterestRate (read-only, always run)
		res = runTest("23. GetMarginInterestRate (BTC)", func() (string, bool) {
			rate, err := marginExc.GetMarginInterestRate("BTC")
			if err != nil {
				return fmt.Sprintf("ERROR: %v", err), false
			}
			annualPct := rate.HourlyRate * 24 * 365 * 100
			return fmt.Sprintf("BTC borrow rate: hourly=%.10f daily=%.10f (~%.2f%% APR)",
				rate.HourlyRate, rate.DailyRate, annualPct), true
		})
		results = append(results, res)

		// 24. GetMarginBalance (read-only, always run)
		res = runTest("24. GetMarginBalance (USDT)", func() (string, bool) {
			bal, err := marginExc.GetMarginBalance("USDT")
			if err != nil {
				return fmt.Sprintf("ERROR: %v", err), false
			}
			return fmt.Sprintf("Total: %.4f  Available: %.4f  Borrowed: %.4f  Interest: %.4f  MaxBorrowable: %.4f",
				bal.TotalBalance, bal.Available, bal.Borrowed, bal.Interest, bal.MaxBorrowable), true
		})
		results = append(results, res)

		// 25-26: Borrow/transfer tests (only with --test-margin)
		if !testMargin {
			for _, n := range []string{
				"25. MarginBorrow + MarginRepay (min amount)",
				"26. TransferToMargin + TransferFromMargin",
			} {
				r := testResult{name: n, pass: false, detail: "SKIPPED (use --test-margin to run)"}
				fmt.Printf("\n--- %s ---\n", n)
				fmt.Printf("   SKIPPED (use --test-margin to run)\n")
				results = append(results, r)
			}
		} else {
			// For exchanges with separate margin accounts (binance, bitget),
			// transfer USDT to margin so there's collateral for borrow/order tests.
			// This is a prerequisite — without collateral, borrow will fail with
			// "maximum borrow amount exceeded" (0 collateral = 0 borrowable).
			separateMarginExchanges := map[string]bool{"binance": true, "bitget": true}
			var marginTestTransferred bool
			if separateMarginExchanges[name] {
				fmt.Printf("\n   [Pre-margin] Transferring 200 USDT to margin for %s (separate account)...\n", name)
				if tErr := marginExc.TransferToMargin("USDT", "200"); tErr != nil {
					fmt.Printf("   [Pre-margin] TransferToMargin WARNING: %v — margin tests may fail\n", tErr)
				} else {
					marginTestTransferred = true
					fmt.Printf("   [Pre-margin] 200 USDT transferred to margin\n")
					time.Sleep(1 * time.Second)
				}
			}

			// 25. MarginBorrow + MarginRepay with minimum USDT
			res = runTest("25. MarginBorrow + MarginRepay (min amount)", func() (string, bool) {
				// Borrow a small amount of USDT (100 to meet exchange min borrow thresholds)
				borrowAmt := "100"
				err := marginExc.MarginBorrow(exchange.MarginBorrowParams{
					Coin:   "USDT",
					Amount: borrowAmt,
				})
				if err != nil {
					return fmt.Sprintf("Borrow ERROR: %v", err), false
				}

				detail := fmt.Sprintf("Borrowed %s USDT", borrowAmt)

				// Small delay for the borrow to settle
				time.Sleep(1 * time.Second)

				// Repay immediately
				err = marginExc.MarginRepay(exchange.MarginRepayParams{
					Coin:   "USDT",
					Amount: borrowAmt,
				})
				if err != nil {
					return fmt.Sprintf("Borrow OK but Repay ERROR: %v (MANUAL REPAY NEEDED!)", err), false
				}
				detail += fmt.Sprintf(" → Repaid %s USDT", borrowAmt)
				return detail, true
			})
			results = append(results, res)

			// 26. TransferToMargin + TransferFromMargin
			res = runTest("26. TransferToMargin + TransferFromMargin", func() (string, bool) {
				transferAmt := "1"
				err := marginExc.TransferToMargin("USDT", transferAmt)
				if err != nil {
					// No-op exchanges return nil; real transfer errors are caught here
					errStr := err.Error()
					if strings.Contains(strings.ToLower(errStr), "no need") ||
						strings.Contains(strings.ToLower(errStr), "same account") {
						return "Transfer not needed (unified account)", true
					}
					return fmt.Sprintf("TransferToMargin ERROR: %v", err), false
				}

				detail := fmt.Sprintf("Transferred %s USDT to margin", transferAmt)

				time.Sleep(1 * time.Second)

				err = marginExc.TransferFromMargin("USDT", transferAmt)
				if err != nil {
					return fmt.Sprintf("TransferToMargin OK but TransferFromMargin ERROR: %v", err), false
				}
				detail += fmt.Sprintf(" → Transferred %s USDT back", transferAmt)
				return detail, true
			})
			results = append(results, res)

			// Variables shared between test 27 and test 28
			var marginTestOrderID string
			var marginTestSymbol string

			// ---------------------------------------------------------------
			// 27. PlaceSpotMarginOrder (Dir A sell with auto-borrow + Dir B buy)
			// ---------------------------------------------------------------
			res = runTest("27. PlaceSpotMarginOrder (Dir A auto-borrow sell + Dir B QuoteSize buy)", func() (string, bool) {
				// Pick a cheap, liquid symbol for testing
				testSymbol := ""
				var testPrice float64
				for _, candidate := range []string{"SEIUSDT", "DOGEUSDT", "XRPUSDT"} {
					if _, ok := contracts[candidate]; ok {
						ob, err := exc.GetOrderbook(candidate, 5)
						if err == nil && len(ob.Asks) > 0 {
							testSymbol = candidate
							testPrice = ob.Asks[0].Price
							break
						}
					}
				}
				if testSymbol == "" {
					return "No suitable cheap symbol found (tried SEI, DOGE, XRP)", false
				}
				marginTestSymbol = testSymbol

				baseCoin := strings.TrimSuffix(testSymbol, "USDT")
				fmt.Printf("   Test symbol: %s  Price: %.4f  Base: %s\n", testSymbol, testPrice, baseCoin)

				// --- Dir A: auto-borrow sell, then auto-repay buyback ---
				fmt.Printf("   [Dir A] Auto-borrow SELL...\n")

				// Calculate sell quantity: ~$12 worth to meet exchange minimum notional
				sellQty := 12.0 / testPrice
				// Round up to a reasonable size (at least 1 for coins like SEI/DOGE)
				if sellQty < 1 {
					sellQty = 1
				}
				sellQty = math.Floor(sellQty)
				if sellQty <= 0 {
					sellQty = 1
				}
				sellQtyStr := strconv.FormatFloat(sellQty, 'f', 0, 64)

				// Place sell with auto-borrow
				sellOID, err := marginExc.PlaceSpotMarginOrder(exchange.SpotMarginOrderParams{
					Symbol:     testSymbol,
					Side:       exchange.SideSell,
					OrderType:  "market",
					Size:       sellQtyStr,
					AutoBorrow: true,
				})
				if err != nil {
					return fmt.Sprintf("[Dir A] PlaceSpotMarginOrder SELL ERROR: %v", err), false
				}
				marginTestOrderID = sellOID
				fmt.Printf("   [Dir A] Sell order ID: %s  qty: %s\n", sellOID, sellQtyStr)

				time.Sleep(2 * time.Second)

				// Check margin balance to confirm auto-borrow worked
				bal, err := marginExc.GetMarginBalance(baseCoin)
				if err != nil {
					fmt.Printf("   [Dir A] GetMarginBalance(%s) warning: %v\n", baseCoin, err)
				} else {
					fmt.Printf("   [Dir A] %s margin: Borrowed=%.4f  Available=%.4f  Total=%.4f\n",
						baseCoin, bal.Borrowed, bal.Available, bal.TotalBalance)
					if bal.Borrowed <= 0 {
						fmt.Printf("   [Dir A] WARNING: Borrowed is 0 — auto-borrow may not have worked\n")
					}
				}

				// Buyback with auto-repay: use QuoteSize with 5% slippage buffer
				buybackQuote := sellQty * testPrice * 1.05
				buybackQuoteStr := strconv.FormatFloat(buybackQuote, 'f', 2, 64)
				fmt.Printf("   [Dir A] Auto-repay BUY back (QuoteSize=%s USDT)...\n", buybackQuoteStr)

				buyOID, err := marginExc.PlaceSpotMarginOrder(exchange.SpotMarginOrderParams{
					Symbol:    testSymbol,
					Side:      exchange.SideBuy,
					OrderType: "market",
					QuoteSize: buybackQuoteStr,
					AutoRepay: true,
				})
				if err != nil {
					fmt.Printf("   [Dir A] BUY BACK ERROR: %v — CLEANUP NEEDED\n", err)
					// Attempt cleanup: try with larger quote size
					buybackQuote2 := sellQty * testPrice * 1.10
					buybackQuoteStr2 := strconv.FormatFloat(buybackQuote2, 'f', 2, 64)
					buyOID2, err2 := marginExc.PlaceSpotMarginOrder(exchange.SpotMarginOrderParams{
						Symbol:    testSymbol,
						Side:      exchange.SideBuy,
						OrderType: "market",
						QuoteSize: buybackQuoteStr2,
						AutoRepay: true,
					})
					if err2 != nil {
						return fmt.Sprintf("[Dir A] Sell OK (oid=%s) but buyback FAILED: %v; RETRY FAILED: %v — MANUAL CLEANUP NEEDED", sellOID, err, err2), false
					}
					fmt.Printf("   [Dir A] Retry buyback OK: oid=%s\n", buyOID2)
				} else {
					fmt.Printf("   [Dir A] Buyback order ID: %s\n", buyOID)
				}

				time.Sleep(2 * time.Second)

				// Verify borrow is cleared after auto-repay
				balAfter, err := marginExc.GetMarginBalance(baseCoin)
				if err != nil {
					fmt.Printf("   [Dir A] Post-repay GetMarginBalance(%s) warning: %v\n", baseCoin, err)
				} else {
					fmt.Printf("   [Dir A] Post-repay %s margin: Borrowed=%.4f\n", baseCoin, balAfter.Borrowed)
					if balAfter.Borrowed > 0.01 {
						// Some exchanges (e.g., Bybit UTA) don't auto-repay via isLeverage=1 alone.
						// Explicitly repay any residual borrow — this matches production behavior in
						// closeDirectionA() which calls MarginRepay for residual debt.
						repayAmt := strconv.FormatFloat(math.Ceil(balAfter.Borrowed), 'f', 0, 64)
						fmt.Printf("   [Dir A] Residual borrow %.4f — explicit MarginRepay(%s %s)...\n", balAfter.Borrowed, repayAmt, baseCoin)
						repayErr := marginExc.MarginRepay(exchange.MarginRepayParams{
							Coin:   baseCoin,
							Amount: repayAmt,
						})
						if repayErr != nil {
							fmt.Printf("   [Dir A] MarginRepay WARNING: %v\n", repayErr)
						} else {
							fmt.Printf("   [Dir A] Explicit repay OK\n")
							time.Sleep(1 * time.Second)
							balFinal, _ := marginExc.GetMarginBalance(baseCoin)
							if balFinal != nil {
								fmt.Printf("   [Dir A] Final %s margin: Borrowed=%.4f\n", baseCoin, balFinal.Borrowed)
							}
						}
					}
				}

				// --- Dir B: buy with QuoteSize, then sell back ---
				fmt.Printf("   [Dir B] QuoteSize BUY ($12 USDT)...\n")

				dirBBuyOID, err := marginExc.PlaceSpotMarginOrder(exchange.SpotMarginOrderParams{
					Symbol:    testSymbol,
					Side:      exchange.SideBuy,
					OrderType: "market",
					QuoteSize: "12",
				})
				if err != nil {
					return fmt.Sprintf("[Dir A] OK, [Dir B] PlaceSpotMarginOrder BUY ERROR: %v", err), false
				}
				fmt.Printf("   [Dir B] Buy order ID: %s\n", dirBBuyOID)

				time.Sleep(2 * time.Second)

				// Check what we bought — get order status to find filled qty
				var dirBFilledQty float64
				if querier, ok := exc.(exchange.SpotMarginOrderQuerier); ok {
					status, err := querier.GetSpotMarginOrder(dirBBuyOID, testSymbol)
					if err == nil && status != nil {
						dirBFilledQty = status.FilledQty
						fmt.Printf("   [Dir B] Filled: %.4f %s @ %.4f\n", status.FilledQty, baseCoin, status.AvgPrice)
					} else if err != nil {
						fmt.Printf("   [Dir B] GetSpotMarginOrder warning: %v\n", err)
					}
				}

				// Sell back what we bought
				if dirBFilledQty <= 0 {
					// Estimate from $12 / price
					dirBFilledQty = math.Floor(12.0 / testPrice)
					if dirBFilledQty <= 0 {
						dirBFilledQty = 1
					}
					fmt.Printf("   [Dir B] Estimated filled qty: %.0f (order query unavailable)\n", dirBFilledQty)
				}
				sellBackQtyStr := strconv.FormatFloat(math.Floor(dirBFilledQty), 'f', 0, 64)
				fmt.Printf("   [Dir B] Selling back %s %s...\n", sellBackQtyStr, baseCoin)

				dirBSellOID, err := marginExc.PlaceSpotMarginOrder(exchange.SpotMarginOrderParams{
					Symbol:    testSymbol,
					Side:      exchange.SideSell,
					OrderType: "market",
					Size:      sellBackQtyStr,
				})
				if err != nil {
					return fmt.Sprintf("[Dir A] OK, [Dir B] Buy OK (oid=%s) but sell-back FAILED: %v — MANUAL CLEANUP NEEDED", dirBBuyOID, err), false
				}
				fmt.Printf("   [Dir B] Sell-back order ID: %s\n", dirBSellOID)

				return fmt.Sprintf("[Dir A] Sell oid=%s → buyback OK | [Dir B] Buy oid=%s → sell-back oid=%s OK", sellOID, dirBBuyOID, dirBSellOID), true
			})
			results = append(results, res)

			// ---------------------------------------------------------------
			// 28. GetSpotMarginOrder (fill reconciliation)
			// ---------------------------------------------------------------
			res = runTest("28. GetSpotMarginOrder (fill reconciliation)", func() (string, bool) {
				querier, ok := exc.(exchange.SpotMarginOrderQuerier)
				if !ok {
					return "SKIPPED (exchange does not implement SpotMarginOrderQuerier)", true
				}
				if marginTestOrderID == "" || marginTestSymbol == "" {
					return "SKIPPED (Test 27 did not produce an order ID)", true
				}

				status, err := querier.GetSpotMarginOrder(marginTestOrderID, marginTestSymbol)
				if err != nil {
					return fmt.Sprintf("ERROR: %v", err), false
				}
				if status == nil {
					return fmt.Sprintf("Order %s not found (nil response)", marginTestOrderID), false
				}

				detail := fmt.Sprintf("OrderID: %s  Symbol: %s  Status: %s  FilledQty: %.6f  AvgPrice: %.4f",
					status.OrderID, status.Symbol, status.Status, status.FilledQty, status.AvgPrice)

				// Verify fill fields
				if status.FilledQty <= 0 {
					detail += "\n   WARNING: FilledQty is 0 — order may not have filled"
				}
				if status.AvgPrice <= 0 {
					detail += "\n   WARNING: AvgPrice is 0 — price not reported"
				}
				passed := status.FilledQty > 0 && status.AvgPrice > 0
				return detail, passed
			})
			results = append(results, res)

			// Cleanup: transfer USDT back from margin to futures if we moved it earlier.
			if marginTestTransferred {
				fmt.Printf("\n   [Post-margin] Transferring USDT back from margin to futures...\n")
				// Query remaining margin balance and transfer it all back.
				if mb, mbErr := marginExc.GetMarginBalance("USDT"); mbErr == nil && mb.Available > 0.01 {
					transferBack := strconv.FormatFloat(math.Floor(mb.Available*100)/100, 'f', 2, 64)
					if tfErr := marginExc.TransferFromMargin("USDT", transferBack); tfErr != nil {
						fmt.Printf("   [Post-margin] TransferFromMargin WARNING: %v\n", tfErr)
					} else {
						fmt.Printf("   [Post-margin] Transferred %s USDT back to futures\n", transferBack)
					}
				} else if mbErr != nil {
					fmt.Printf("   [Post-margin] GetMarginBalance WARNING: %v — manual cleanup may be needed\n", mbErr)
				}
			}
		}
	}

	return printSummary(name, results, totalTests)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func runTest(name string, fn func() (string, bool)) testResult {
	fmt.Printf("\n--- %s ---\n", name)
	start := time.Now()
	detail, pass := fn()
	elapsed := time.Since(start)

	status := "PASS"
	if !pass {
		status = "FAIL"
	}
	fmt.Printf("   %s\n", detail)
	fmt.Printf("   [%s] (%s)\n", status, elapsed.Round(time.Millisecond))

	return testResult{
		name:    name,
		pass:    pass,
		elapsed: elapsed,
		detail:  detail,
	}
}

// printSummary prints the test summary and returns true if all tests passed.
func printSummary(name string, results []testResult, total int) bool {
	fmt.Println()
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("=== Summary for %s ===\n", name)
	fmt.Println(strings.Repeat("=", 60))

	passed := 0
	skipped := 0
	for _, r := range results {
		if r.pass {
			passed++
		}
		if strings.Contains(r.detail, "SKIPPED") {
			skipped++
		}
		status := "PASS"
		if !r.pass {
			if strings.Contains(r.detail, "SKIPPED") {
				status = "SKIP"
			} else {
				status = "FAIL"
			}
		}
		fmt.Printf("  [%s] %s  (%s)\n", status, r.name, r.elapsed.Round(time.Millisecond))
	}
	fmt.Printf("\nResult: %d/%d tests passed", passed, total)
	if skipped > 0 {
		fmt.Printf(" (%d skipped)", skipped)
	}
	fmt.Printf(" for %s\n", name)

	return passed+skipped >= total
}
