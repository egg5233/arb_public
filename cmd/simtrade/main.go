// simtrade replays the full executeArbitrage flow using live exchange
// connections. It discovers opportunities, runs risk approval, subscribes
// to depth, and enters the depth fill loop — exactly like the real engine.
//
// Usage:
//
//	go run ./cmd/simtrade/                          # discover + show depth analysis (dry run)
//	go run ./cmd/simtrade/ --live                   # actually place orders
//	go run ./cmd/simtrade/ --symbol LYNUSDT         # force a specific symbol
//	go run ./cmd/simtrade/ --symbol LYNUSDT --long binance --short bybit
//	go run ./cmd/simtrade/ --timeout 30             # custom fill loop timeout (seconds)
//	go run ./cmd/simtrade/ --skip-risk              # skip risk approval (use fixed size)
//	go run ./cmd/simtrade/ --size 100               # fixed notional USDT per leg
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"arb/internal/api"
	"arb/internal/config"
	"arb/internal/database"
	"arb/internal/discovery"
	"arb/internal/engine"
	"arb/internal/models"
	"arb/internal/risk"
	"arb/pkg/exchange"
	"arb/pkg/exchange/binance"
	"arb/pkg/exchange/bitget"
	"arb/pkg/exchange/bybit"
	"arb/pkg/exchange/gateio"
	"arb/pkg/exchange/okx"
	"arb/pkg/utils"
)

func main() {
	symbol := flag.String("symbol", "", "force a specific symbol (e.g. LYNUSDT)")
	longExchFlag := flag.String("long", "", "force long exchange (e.g. binance)")
	shortExchFlag := flag.String("short", "", "force short exchange (e.g. gateio)")
	live := flag.Bool("live", false, "actually place orders (default: dry run)")
	timeout := flag.Int("timeout", 0, "fill loop timeout in seconds (0 = use config)")
	skipRisk := flag.Bool("skip-risk", false, "skip risk approval, use fixed size")
	sizeUSDT := flag.Float64("size", 0, "fixed notional USDT per leg (0 = use config)")
	flag.Parse()

	log := utils.NewLogger("simtrade")

	cfg := config.Load()
	if !*live {
		cfg.DryRun = true
		log.Info("DRY RUN mode — no orders will be placed")
	} else {
		log.Info("LIVE mode — orders WILL be placed")
	}
	if *timeout > 0 {
		cfg.EntryTimeoutSec = *timeout
	}
	if *sizeUSDT > 0 {
		cfg.CapitalPerLeg = *sizeUSDT
	}

	// Connect to Redis
	db, err := database.New(cfg.RedisAddr, cfg.RedisPass, cfg.RedisDB)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Redis connection failed: %v\n", err)
		os.Exit(1)
	}
	log.Info("connected to Redis")

	// Initialize exchanges
	exchanges := map[string]exchange.Exchange{}
	for _, name := range []string{"binance", "bitget", "bybit", "gateio", "okx"} {
		exc, err := makeAdapter(cfg, name)
		if err != nil {
			log.Warn("skipping %s: %v", name, err)
			continue
		}
		exchanges[name] = exc
	}
	if len(exchanges) == 0 {
		fmt.Fprintln(os.Stderr, "No exchanges configured")
		os.Exit(1)
	}
	log.Info("initialized %d exchanges", len(exchanges))

	// Load contracts
	allContracts := map[string]map[string]exchange.ContractInfo{}
	for name, exc := range exchanges {
		contracts, err := exc.LoadAllContracts()
		if err != nil {
			log.Warn("failed to load contracts for %s: %v", name, err)
		} else {
			allContracts[name] = contracts
			log.Info("loaded %d contracts for %s", len(contracts), name)
		}
	}

	// Start WebSocket streams (needed for depth + order confirmations)
	for name, exc := range exchanges {
		exc.StartPriceStream(nil)
		exc.StartPrivateStream()
		log.Info("started WS streams for %s", name)
	}
	log.Info("waiting 3s for WS connections...")
	time.Sleep(3 * time.Second)

	// Discover or build opportunity
	var opp models.Opportunity

	if *symbol != "" && *longExchFlag != "" && *shortExchFlag != "" {
		opp = models.Opportunity{
			Symbol:        *symbol,
			LongExchange:  *longExchFlag,
			ShortExchange: *shortExchFlag,
			Spread:        100,
		}
		log.Info("manual opportunity: %s long=%s short=%s", opp.Symbol, opp.LongExchange, opp.ShortExchange)
	} else {
		scanner := discovery.NewScanner(exchanges, db, cfg)
		scanner.SetContracts(allContracts)
		opps := scanner.GetOpportunities()
		if len(opps) == 0 {
			log.Error("no opportunities found")
			os.Exit(1)
		}

		if *symbol != "" {
			found := false
			for _, o := range opps {
				if o.Symbol == *symbol {
					opp = o
					found = true
					break
				}
			}
			if !found {
				log.Error("symbol %s not found in %d opportunities:", *symbol, len(opps))
				for i, o := range opps {
					log.Info("  [%d] %s long=%s short=%s spread=%.1f bps/h",
						i+1, o.Symbol, o.LongExchange, o.ShortExchange, o.Spread)
				}
				os.Exit(1)
			}
		} else {
			opp = opps[0]
		}

		log.Info("discovered %d opportunities:", len(opps))
		for i, o := range opps {
			marker := "  "
			if o.Symbol == opp.Symbol {
				marker = "→ "
			}
			log.Info("%s[%d] %s long=%s short=%s spread=%.1f bps/h score=%.1f",
				marker, i+1, o.Symbol, o.LongExchange, o.ShortExchange, o.Spread, o.Score)
		}
	}

	log.Info("========================================")
	log.Info("SELECTED: %s long=%s short=%s spread=%.1f bps/h",
		opp.Symbol, opp.LongExchange, opp.ShortExchange, opp.Spread)
	log.Info("========================================")

	// Check balances
	for _, leg := range []struct {
		name string
		side string
	}{
		{opp.LongExchange, "long"},
		{opp.ShortExchange, "short"},
	} {
		exc := exchanges[leg.name]
		futBal, err := exc.GetFuturesBalance()
		if err != nil {
			log.Error("%s futures balance: %v", leg.name, err)
		} else {
			log.Info("%s (%s) futures: total=%.2f available=%.2f marginRatio=%.4f",
				leg.name, leg.side, futBal.Total, futBal.Available, futBal.MarginRatio)
		}
		spotBal, err := exc.GetSpotBalance()
		if err != nil {
			log.Error("%s spot balance: %v", leg.name, err)
		} else {
			log.Info("%s (%s) spot: available=%.2f", leg.name, leg.side, spotBal.Available)
		}
	}

	// Risk approval
	var tradeSize, tradePrice float64
	allocator := risk.NewCapitalAllocator(db, cfg)
	riskMgr := risk.NewManager(exchanges, db, cfg, allocator)

	if *skipRisk {
		price := getPrice(exchanges, opp)
		if price <= 0 {
			log.Error("cannot determine price for %s", opp.Symbol)
			os.Exit(1)
		}
		notional := cfg.CapitalPerLeg
		if notional <= 0 {
			notional = 50
		}
		tradeSize = notional * float64(cfg.Leverage) / price
		tradePrice = price
		log.Info("skip-risk: size=%.6f price=%.6f notional=%.2f", tradeSize, tradePrice, notional)
	} else {
		approval, err := riskMgr.Approve(opp)
		if err != nil {
			log.Error("risk approval error: %v", err)
			os.Exit(1)
		}
		if !approval.Approved {
			log.Error("risk rejected: %s", approval.Reason)
			os.Exit(1)
		}
		tradeSize = approval.Size
		tradePrice = approval.Price
		log.Info("risk approved: size=%.6f price=%.6f", tradeSize, tradePrice)
	}

	// Depth analysis
	log.Info("subscribing to depth...")
	longExc := exchanges[opp.LongExchange]
	shortExc := exchanges[opp.ShortExchange]
	longExc.SubscribeDepth(opp.Symbol)
	shortExc.SubscribeDepth(opp.Symbol)
	defer longExc.UnsubscribeDepth(opp.Symbol)
	defer shortExc.UnsubscribeDepth(opp.Symbol)

	depthReady := false
	for i := 0; i < 40; i++ {
		_, lok := longExc.GetDepth(opp.Symbol)
		_, sok := shortExc.GetDepth(opp.Symbol)
		if lok && sok {
			depthReady = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !depthReady {
		log.Error("depth not available after 4s")
		os.Exit(1)
	}

	// Print depth
	printDepth(log, longExc, opp.LongExchange, opp.Symbol, "long/asks")
	printDepth(log, shortExc, opp.ShortExchange, opp.Symbol, "short/bids")

	// Cross-exchange spread
	longDepth, _ := longExc.GetDepth(opp.Symbol)
	shortDepth, _ := shortExc.GetDepth(opp.Symbol)
	if len(longDepth.Asks) > 0 && len(shortDepth.Bids) > 0 {
		askPrice := longDepth.Asks[0].Price
		bidPrice := shortDepth.Bids[0].Price
		spreadBPS := (askPrice/bidPrice - 1) * 10000
		log.Info("cross-exchange spread: %.1f bps (long ask=%.6f short bid=%.6f)",
			spreadBPS, askPrice, bidPrice)
	}

	// BBO
	for _, leg := range []struct {
		name string
		exc  exchange.Exchange
	}{
		{opp.LongExchange, longExc},
		{opp.ShortExchange, shortExc},
	} {
		if bbo, ok := leg.exc.GetBBO(opp.Symbol); ok {
			log.Info("BBO %s: bid=%.6f ask=%.6f mid=%.6f spread=%.1fbps",
				leg.name, bbo.Bid, bbo.Ask, (bbo.Bid+bbo.Ask)/2, (bbo.Ask/bbo.Bid-1)*10000)
		} else {
			log.Warn("BBO %s: not available", leg.name)
		}
	}

	// Contract info
	for _, leg := range []struct{ name, side string }{
		{opp.LongExchange, "long"},
		{opp.ShortExchange, "short"},
	} {
		if ec, ok := allContracts[leg.name]; ok {
			if ci, ok := ec[opp.Symbol]; ok {
				log.Info("contract %s/%s: stepSize=%.6f minSize=%.6f priceStep=%.8f",
					leg.name, opp.Symbol, ci.StepSize, ci.MinSize, ci.PriceStep)
			}
		}
	}

	log.Info("========================================")
	log.Info("TRADE: size=%.6f price=%.6f timeout=%ds dry_run=%v",
		tradeSize, tradePrice, cfg.EntryTimeoutSec, cfg.DryRun)
	log.Info("========================================")

	if cfg.DryRun {
		log.Info("DRY RUN — analysis complete. Use --live to execute.")
		return
	}

	// Build a real engine and execute
	apiSrv := api.NewServer(db, cfg, exchanges)
	riskMon := risk.NewMonitor(exchanges, db, cfg)
	healthMon := risk.NewHealthMonitor(exchanges, db, cfg)
	eng := engine.NewEngine(exchanges, nil, riskMgr, riskMon, healthMon, db, apiSrv, cfg, allocator)
	eng.SetContracts(allContracts)

	err = eng.SimExecuteTradeV2(opp, tradeSize, tradePrice, 0)
	if err != nil {
		log.Error("trade execution failed: %v", err)
		os.Exit(1)
	}
	log.Info("trade execution complete")
}

func getPrice(exchanges map[string]exchange.Exchange, opp models.Opportunity) float64 {
	if exc, ok := exchanges[opp.LongExchange]; ok {
		if bbo, ok := exc.GetBBO(opp.Symbol); ok && bbo.Ask > 0 {
			return bbo.Ask
		}
		if ob, err := exc.GetOrderbook(opp.Symbol, 5); err == nil && len(ob.Asks) > 0 {
			return ob.Asks[0].Price
		}
	}
	return 0
}

func printDepth(log *utils.Logger, exc exchange.Exchange, name, symbol, label string) {
	depth, ok := exc.GetDepth(symbol)
	if !ok {
		log.Warn("no depth for %s on %s", symbol, name)
		return
	}
	log.Info("depth %s (%s) age=%s:", name, label, time.Since(depth.Time).Round(time.Millisecond))
	if label == "long/asks" {
		for i, lvl := range depth.Asks {
			if i >= 5 {
				break
			}
			log.Info("  ask[%d] price=%.6f qty=%.4f (%.2f USDT)", i, lvl.Price, lvl.Quantity, lvl.Price*lvl.Quantity)
		}
	} else {
		for i, lvl := range depth.Bids {
			if i >= 5 {
				break
			}
			log.Info("  bid[%d] price=%.6f qty=%.4f (%.2f USDT)", i, lvl.Price, lvl.Quantity, lvl.Price*lvl.Quantity)
		}
	}
}

func makeAdapter(cfg *config.Config, name string) (exchange.Exchange, error) {
	switch name {
	case "binance":
		if cfg.BinanceAPIKey == "" {
			return nil, fmt.Errorf("no API key")
		}
		return binance.NewAdapter(exchange.ExchangeConfig{
			Exchange: "binance", ApiKey: cfg.BinanceAPIKey, SecretKey: cfg.BinanceSecretKey,
		}), nil
	case "bitget":
		if cfg.BitgetAPIKey == "" {
			return nil, fmt.Errorf("no API key")
		}
		return bitget.NewAdapter(exchange.ExchangeConfig{
			Exchange: "bitget", ApiKey: cfg.BitgetAPIKey, SecretKey: cfg.BitgetSecretKey, Passphrase: cfg.BitgetPassphrase,
		}), nil
	case "bybit":
		if cfg.BybitAPIKey == "" {
			return nil, fmt.Errorf("no API key")
		}
		return bybit.NewAdapter(exchange.ExchangeConfig{
			Exchange: "bybit", ApiKey: cfg.BybitAPIKey, SecretKey: cfg.BybitSecretKey,
		}), nil
	case "gateio":
		if cfg.GateioAPIKey == "" {
			return nil, fmt.Errorf("no API key")
		}
		return gateio.NewAdapter(exchange.ExchangeConfig{
			Exchange: "gateio", ApiKey: cfg.GateioAPIKey, SecretKey: cfg.GateioSecretKey,
		}), nil
	case "okx":
		if cfg.OKXAPIKey == "" {
			return nil, fmt.Errorf("no API key")
		}
		return okx.NewAdapter(exchange.ExchangeConfig{
			Exchange: "okx", ApiKey: cfg.OKXAPIKey, SecretKey: cfg.OKXSecretKey, Passphrase: cfg.OKXPassphrase,
		}), nil
	}
	return nil, fmt.Errorf("unknown exchange: %s", name)
}
