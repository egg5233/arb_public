package main

import (
	"flag"
	"fmt"
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

const totalTests = 12

func main() {
	exchangeFlag := flag.String("exchange", "", "test a specific exchange (binance, bitget, bybit, gateio, okx, bingx)")
	verbose := flag.Bool("verbose", false, "show detailed struct fields in output")
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
		os.Exit(1)
	}

	type exchangeResult struct {
		name   string
		passed int
		total  int
	}

	var summaries []exchangeResult
	anyFail := false

	for _, a := range adapters {
		passed := runValidation(a.name, a.exc, *verbose)
		summaries = append(summaries, exchangeResult{name: a.name, passed: passed, total: totalTests})
		if passed < totalTests {
			anyFail = true
		}
		fmt.Println()
	}

	// Final summary
	fmt.Println("=== SUMMARY ===")
	for _, s := range summaries {
		status := "PASS"
		if s.passed < s.total {
			status = "FAIL"
		}
		fmt.Printf("  %-10s %d/%d %s\n", s.name+":", s.passed, s.total, status)
	}

	if anyFail {
		os.Exit(1)
	}
}

// ---------------------------------------------------------------------------
// Adapter factory (reused from livetest)
// ---------------------------------------------------------------------------

func makeAdapter(cfg *config.Config, name string) (exchange.Exchange, error) {
	switch name {
	case "binance":
		if cfg.BinanceAPIKey == "" {
			return nil, fmt.Errorf("BINANCE_API_KEY not set")
		}
		return binance.NewAdapter(exchange.ExchangeConfig{
			Exchange:  "binance",
			ApiKey:    cfg.BinanceAPIKey,
			SecretKey: cfg.BinanceSecretKey,
		}), nil
	case "bitget":
		if cfg.BitgetAPIKey == "" {
			return nil, fmt.Errorf("BITGET_API_KEY not set")
		}
		if cfg.BitgetPassphrase == "" {
			return nil, fmt.Errorf("BITGET_PASSPHRASE not set")
		}
		return bitget.NewAdapter(exchange.ExchangeConfig{
			Exchange:   "bitget",
			ApiKey:     cfg.BitgetAPIKey,
			SecretKey:  cfg.BitgetSecretKey,
			Passphrase: cfg.BitgetPassphrase,
		}), nil
	case "bybit":
		if cfg.BybitAPIKey == "" {
			return nil, fmt.Errorf("BYBIT_API_KEY not set")
		}
		return bybit.NewAdapter(exchange.ExchangeConfig{
			Exchange:  "bybit",
			ApiKey:    cfg.BybitAPIKey,
			SecretKey: cfg.BybitSecretKey,
		}), nil
	case "gateio":
		if cfg.GateioAPIKey == "" {
			return nil, fmt.Errorf("GATEIO_API_KEY not set")
		}
		return gateio.NewAdapter(exchange.ExchangeConfig{
			Exchange:  "gateio",
			ApiKey:    cfg.GateioAPIKey,
			SecretKey: cfg.GateioSecretKey,
		}), nil
	case "okx":
		if cfg.OKXAPIKey == "" {
			return nil, fmt.Errorf("OKX_API_KEY not set")
		}
		if cfg.OKXPassphrase == "" {
			return nil, fmt.Errorf("OKX_PASSPHRASE not set")
		}
		return okx.NewAdapter(exchange.ExchangeConfig{
			Exchange:   "okx",
			ApiKey:     cfg.OKXAPIKey,
			SecretKey:  cfg.OKXSecretKey,
			Passphrase: cfg.OKXPassphrase,
		}), nil
	case "bingx":
		if cfg.BingXAPIKey == "" {
			return nil, fmt.Errorf("BINGX_API_KEY not set")
		}
		return bingx.NewAdapter(exchange.ExchangeConfig{
			Exchange:  "bingx",
			ApiKey:    cfg.BingXAPIKey,
			SecretKey: cfg.BingXSecretKey,
		}), nil
	default:
		return nil, fmt.Errorf("unsupported exchange: %q", name)
	}
}

// ---------------------------------------------------------------------------
// Validation runner
// ---------------------------------------------------------------------------

type testResult struct {
	num    int
	name   string
	pass   bool
	detail string
}

func runValidation(name string, exc exchange.Exchange, verbose bool) int {
	fmt.Printf("=== %s ===\n", name)

	var results []testResult
	testSymbol := "BTCUSDT" // all adapters accept BTCUSDT and convert internally

	// 1. LoadAllContracts
	var contracts map[string]exchange.ContractInfo
	results = append(results, runCheck(1, "LoadAllContracts", func() (string, bool) {
		var err error
		contracts, err = exc.LoadAllContracts()
		if err != nil {
			return fmt.Sprintf("error: %v", err), false
		}
		if len(contracts) == 0 {
			return "0 contracts returned", false
		}

		// Spot-check BTCUSDT (find it under any key variation)
		var btcInfo exchange.ContractInfo
		var btcKey string
		for _, candidate := range []string{"BTCUSDT", "BTC_USDT", "BTC-USDT-SWAP"} {
			if c, ok := contracts[candidate]; ok {
				btcInfo = c
				btcKey = candidate
				break
			}
		}
		if btcKey == "" {
			for sym, c := range contracts {
				if strings.Contains(strings.ToUpper(sym), "BTC") && strings.Contains(strings.ToUpper(sym), "USDT") {
					btcInfo = c
					btcKey = sym
					break
				}
			}
		}

		detail := fmt.Sprintf("%d contracts", len(contracts))
		if btcKey != "" {
			detail += fmt.Sprintf(", %s stepSize=%v", btcKey, btcInfo.StepSize)
		} else {
			detail += ", WARNING: no BTC contract found"
		}
		if verbose {
			detail += fmt.Sprintf("\n           BTC contract: %+v", btcInfo)
		}

		pass := len(contracts) > 100
		if !pass {
			detail += fmt.Sprintf(" (expected >100, got %d)", len(contracts))
		}
		return detail, pass
	}))

	// 2. GetFundingRate
	results = append(results, runCheck(2, "GetFundingRate", func() (string, bool) {
		fr, err := exc.GetFundingRate(testSymbol)
		if err != nil {
			return fmt.Sprintf("error: %v", err), false
		}
		if fr == nil {
			return "returned nil", false
		}

		detail := fmt.Sprintf("rate=%.6f, interval=%s", fr.Rate, fr.Interval)
		if fr.MaxRate != nil {
			detail += fmt.Sprintf(", maxRate=%.4f", *fr.MaxRate)
		} else {
			detail += ", maxRate=nil"
		}
		if fr.MinRate != nil {
			detail += fmt.Sprintf(", minRate=%.4f", *fr.MinRate)
		} else {
			detail += ", minRate=nil"
		}
		if verbose {
			detail += fmt.Sprintf("\n           full: %+v", *fr)
		}

		pass := true
		var issues []string
		if fr.Interval <= 0 {
			issues = append(issues, "Interval<=0")
			pass = false
		}
		if fr.MaxRate == nil {
			issues = append(issues, "MaxRate=nil (cap parsing missing)")
			pass = false
		}
		if fr.MinRate == nil {
			issues = append(issues, "MinRate=nil (cap parsing missing)")
			pass = false
		}
		if len(issues) > 0 {
			detail += " [ISSUES: " + strings.Join(issues, ", ") + "]"
		}
		return detail, pass
	}))

	// 3. GetFundingInterval
	results = append(results, runCheck(3, "GetFundingInterval", func() (string, bool) {
		interval, err := exc.GetFundingInterval(testSymbol)
		if err != nil {
			return fmt.Sprintf("error: %v", err), false
		}
		pass := interval > 0
		detail := fmt.Sprintf("%s", interval)
		if !pass {
			detail += " (expected > 0)"
		}
		return detail, pass
	}))

	// 4. GetFuturesBalance
	results = append(results, runCheck(4, "GetFuturesBalance", func() (string, bool) {
		bal, err := exc.GetFuturesBalance()
		if err != nil {
			return fmt.Sprintf("error: %v", err), false
		}
		if bal == nil {
			return "returned nil", false
		}

		detail := fmt.Sprintf("total=%.2f, available=%.2f", bal.Total, bal.Available)
		if verbose {
			detail += fmt.Sprintf("\n           full: %+v", *bal)
		}

		pass := true
		var issues []string
		if bal.Total < 0 {
			issues = append(issues, "Total<0")
			pass = false
		}
		if bal.Available < 0 {
			issues = append(issues, "Available<0")
			pass = false
		}
		if len(issues) > 0 {
			detail += " [ISSUES: " + strings.Join(issues, ", ") + "]"
		}
		return detail, pass
	}))

	// 5. GetSpotBalance
	results = append(results, runCheck(5, "GetSpotBalance", func() (string, bool) {
		bal, err := exc.GetSpotBalance()
		if err != nil {
			return fmt.Sprintf("error: %v", err), false
		}
		if bal == nil {
			return "returned nil", false
		}
		detail := fmt.Sprintf("total=%.2f", bal.Total)
		if verbose {
			detail += fmt.Sprintf("\n           full: %+v", *bal)
		}
		return detail, true
	}))

	// 6. GetPosition
	results = append(results, runCheck(6, "GetPosition", func() (string, bool) {
		positions, err := exc.GetPosition(testSymbol)
		if err != nil {
			return fmt.Sprintf("error: %v", err), false
		}
		detail := fmt.Sprintf("%d positions", len(positions))
		if verbose && len(positions) > 0 {
			for _, p := range positions {
				detail += fmt.Sprintf("\n           %+v", p)
			}
		}
		return detail, true
	}))

	// 7. GetAllPositions
	results = append(results, runCheck(7, "GetAllPositions", func() (string, bool) {
		positions, err := exc.GetAllPositions()
		if err != nil {
			return fmt.Sprintf("error: %v", err), false
		}

		detail := fmt.Sprintf("%d positions", len(positions))
		pass := true
		var issues []string

		for _, p := range positions {
			size, _ := strconv.ParseFloat(p.Total, 64)
			entry, _ := strconv.ParseFloat(p.AverageOpenPrice, 64)
			if size > 0 && entry <= 0 {
				issues = append(issues, fmt.Sprintf("%s %s: size=%s but EntryPrice=%s (missing!)", p.Symbol, p.HoldSide, p.Total, p.AverageOpenPrice))
				pass = false
			}
		}

		if len(positions) > 0 {
			first := positions[0]
			detail += fmt.Sprintf(" (%s entry=%s)", first.Symbol, first.AverageOpenPrice)
		}

		if len(issues) > 0 {
			detail += " [ISSUES: " + strings.Join(issues, "; ") + "]"
		}
		if verbose && len(positions) > 0 {
			for _, p := range positions {
				detail += fmt.Sprintf("\n           %+v", p)
			}
		}
		return detail, pass
	}))

	// 8. GetOrderbook
	results = append(results, runCheck(8, "GetOrderbook", func() (string, bool) {
		ob, err := exc.GetOrderbook(testSymbol, 5)
		if err != nil {
			return fmt.Sprintf("error: %v", err), false
		}
		if ob == nil {
			return "returned nil", false
		}

		pass := len(ob.Bids) > 0 && len(ob.Asks) > 0
		detail := fmt.Sprintf("%d bids, %d asks", len(ob.Bids), len(ob.Asks))
		if !pass {
			detail += " (expected non-empty)"
		}
		if verbose && len(ob.Bids) > 0 && len(ob.Asks) > 0 {
			detail += fmt.Sprintf("\n           bestBid=%.2f bestAsk=%.2f", ob.Bids[0].Price, ob.Asks[0].Price)
		}
		return detail, pass
	}))

	// 9. GetPendingOrders
	results = append(results, runCheck(9, "GetPendingOrders", func() (string, bool) {
		orders, err := exc.GetPendingOrders(testSymbol)
		if err != nil {
			return fmt.Sprintf("error: %v", err), false
		}
		detail := fmt.Sprintf("%d orders", len(orders))
		if verbose && len(orders) > 0 {
			for _, o := range orders {
				detail += fmt.Sprintf("\n           %+v", o)
			}
		}
		return detail, true
	}))

	// 10. GetUserTrades
	results = append(results, runCheck(10, "GetUserTrades", func() (string, bool) {
		trades, err := exc.GetUserTrades(testSymbol, time.Now().Add(-24*time.Hour), 10)
		if err != nil {
			return fmt.Sprintf("error: %v", err), false
		}

		detail := fmt.Sprintf("%d trades", len(trades))
		pass := true

		if len(trades) > 0 {
			hasFee := false
			for _, t := range trades {
				if t.Fee > 0 {
					hasFee = true
					break
				}
			}
			if !hasFee {
				detail += " [ISSUE: no trade has Fee>0 (empty-fee bug?)]"
				pass = false
			} else {
				detail += ", fee populated=true"
			}
		}

		if verbose && len(trades) > 0 {
			for i, t := range trades {
				if i >= 3 {
					detail += fmt.Sprintf("\n           ... and %d more", len(trades)-3)
					break
				}
				detail += fmt.Sprintf("\n           %+v", t)
			}
		}
		return detail, pass
	}))

	// 11. GetFundingFees
	results = append(results, runCheck(11, "GetFundingFees", func() (string, bool) {
		payments, err := exc.GetFundingFees(testSymbol, time.Now().Add(-7*24*time.Hour))
		if err != nil {
			return fmt.Sprintf("error: %v", err), false
		}
		detail := fmt.Sprintf("%d payments", len(payments))
		if verbose && len(payments) > 0 {
			for i, p := range payments {
				if i >= 3 {
					detail += fmt.Sprintf("\n           ... and %d more", len(payments)-3)
					break
				}
				detail += fmt.Sprintf("\n           %+v", p)
			}
		}
		return detail, true
	}))

	// 12. GetClosePnL
	results = append(results, runCheck(12, "GetClosePnL", func() (string, bool) {
		records, err := exc.GetClosePnL(testSymbol, time.Now().Add(-7*24*time.Hour))
		if err != nil {
			return fmt.Sprintf("error: %v", err), false
		}

		detail := fmt.Sprintf("%d records", len(records))
		pass := true

		if len(records) > 0 {
			hasFees := false
			for _, r := range records {
				if r.Fees != 0 {
					hasFees = true
					break
				}
			}
			detail += fmt.Sprintf(", fees populated=%v", hasFees)
			if !hasFees {
				detail += " [ISSUE: no record has Fees!=0 (zero-fees bug?)]"
				pass = false
			}
		}

		if verbose && len(records) > 0 {
			for i, r := range records {
				if i >= 3 {
					detail += fmt.Sprintf("\n           ... and %d more", len(records)-3)
					break
				}
				detail += fmt.Sprintf("\n           %+v", r)
			}
		}
		return detail, pass
	}))

	// Print results
	passed := 0
	for _, r := range results {
		status := "PASS"
		if !r.pass {
			status = "FAIL"
		}
		fmt.Printf("  [%s] %2d. %-24s %s\n", status, r.num, r.name, r.detail)
	}

	for _, r := range results {
		if r.pass {
			passed++
		}
	}
	fmt.Printf("  Result: %d/%d passed\n", passed, totalTests)
	return passed
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func runCheck(num int, name string, fn func() (string, bool)) testResult {
	detail, pass := fn()
	return testResult{
		num:    num,
		name:   name,
		pass:   pass,
		detail: detail,
	}
}
