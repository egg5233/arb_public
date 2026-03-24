package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"arb/internal/api"
	"arb/internal/config"
	"arb/internal/database"
	"arb/internal/discovery"
	"arb/internal/engine"
	"arb/pkg/exchange"
	"arb/pkg/exchange/binance"
	"arb/pkg/exchange/bitget"
	"arb/pkg/exchange/bybit"
	"arb/pkg/exchange/gateio"
	"arb/pkg/exchange/bingx"
	"arb/pkg/exchange/okx"
	"arb/internal/models"
	"arb/internal/risk"
	"arb/pkg/utils"
)

func main() {
	log := utils.NewLogger("main")
	log.Info("Starting funding rate arbitrage bot...")

	// Load configuration
	cfg := config.Load()
	log.Info("Configuration loaded")
	if cfg.DryRun {
		log.Info("*** DRY RUN MODE ENABLED — no trades will be placed ***")
	}

	// Connect to Redis
	db, err := database.New(cfg.RedisAddr, cfg.RedisPass, cfg.RedisDB)
	if err != nil {
		log.Error("Failed to connect to Redis: %v", err)
		os.Exit(1)
	}
	log.Info("Connected to Redis (db %d)", cfg.RedisDB)

	// Initialize enabled exchanges
	exchanges := make(map[string]exchange.Exchange)
	for _, name := range cfg.EnabledExchanges() {
		excCfg := exchangeConfig(cfg, name)
		exc, err := newExchange(name, excCfg)
		if err != nil {
			log.Error("Failed to init exchange %s: %v", name, err)
			continue
		}
		exchanges[name] = exc
		log.Info("Initialized exchange: %s", name)
	}

	if len(exchanges) < 2 {
		log.Error("Need at least 2 exchanges configured, got %d", len(exchanges))
		os.Exit(1)
	}
	log.Info("Active exchanges: %d", len(exchanges))

	// Load contract info for all exchanges
	allContracts := make(map[string]map[string]exchange.ContractInfo)
	for name, exc := range exchanges {
		contracts, err := exc.LoadAllContracts()
		if err != nil {
			log.Warn("Failed to load contracts for %s: %v", name, err)
		} else {
			allContracts[name] = contracts
			log.Info("Loaded %d contracts for %s", len(contracts), name)
		}
	}

	// Start WebSocket streams
	for name, exc := range exchanges {
		exc.StartPriceStream(nil) // will subscribe dynamically
		// Note: SL callbacks are registered later by engine.Start() via SetOrderCallback.
		// The callback field is set before any fills can arrive because StartPrivateStream
		// takes time to connect + authenticate. Engine.Start() runs before first fills.
		exc.StartPrivateStream()
		log.Info("Started WebSocket streams for %s", name)
	}

	// Event channels (section 3.11 of plan.md).
	// These are currently consumed by the engine internally but are
	// exposed here so that additional subscribers (e.g. logging, metrics)
	// can be wired in later.
	opportunityCh := make(chan []models.Opportunity, 4)
	positionUpdateCh := make(chan models.ArbitragePosition, 16)
	riskAlertCh := make(chan models.RiskAlert, 32)

	// Initialize components
	scanner := discovery.NewScanner(exchanges, db, cfg)
	scanner.SetContracts(allContracts)
	riskMgr := risk.NewManager(exchanges, db, cfg)
	riskMon := risk.NewMonitor(exchanges, db, cfg)
	healthMon := risk.NewHealthMonitor(exchanges, db, cfg)
	apiSrv := api.NewServer(db, cfg, exchanges)
	eng := engine.NewEngine(exchanges, scanner, riskMgr, riskMon, healthMon, db, apiSrv, cfg)
	eng.SetContracts(allContracts)

	// Ensure all exchanges are in cross-margin one-way mode.
	for name, exch := range exchanges {
		if err := exch.EnsureOneWayMode(); err != nil {
			log.Error("EnsureOneWayMode on %s: %v", name, err)
		} else {
			log.Info("%s: one-way mode confirmed", name)
		}
	}

	// Create shared rejection store and wire to all components.
	rejStore := models.NewRejectionStore(500, func(r models.RejectedOpportunity) {
		apiSrv.BroadcastRejection(r)
	})
	scanner.SetRejectionStore(rejStore)
	eng.SetRejectionStore(rejStore)
	apiSrv.SetRejectionStore(rejStore)

	// Register manual close/open handlers with the API server.
	apiSrv.SetCloseHandler(eng.ManualClose)
	apiSrv.SetOpenHandler(eng.ManualOpen)

	// Wire event channels — forward to dashboard API.
	go func() {
		for opps := range opportunityCh {
			apiSrv.SetOpportunities(opps)
			apiSrv.BroadcastOpportunities(opps)
		}
	}()
	go func() {
		for pos := range positionUpdateCh {
			p := pos // capture loop variable
			apiSrv.BroadcastPositionUpdate(&p)
		}
	}()
	go func() {
		for alert := range riskAlertCh {
			apiSrv.BroadcastAlert(alert)
		}
	}()

	// Keep channels referenced to avoid unused-variable errors. The engine
	// currently drives these flows itself; when it is refactored to accept
	// channels, these can be passed directly.
	_ = opportunityCh
	_ = positionUpdateCh
	_ = riskAlertCh

	// Periodically refresh exchange balances into Redis for the dashboard.
	go func() {
		refreshBalances := func() {
			for name, exc := range exchanges {
				bal, err := exc.GetFuturesBalance()
				if err != nil {
					log.Warn("balance refresh %s: %v", name, err)
					continue
				}
					// Save Total (equity) so dashboard shows full account value
				// including margin locked in positions, not just free margin.
				amount := bal.Total
				spotBal, err := exc.GetSpotBalance()
				if err == nil {
					db.SaveSpotBalance(name, spotBal.Available)
					// Gate.io unified: spot balance = true account equity,
					// futures balance is just the futures-allocated portion.
					if name == "gateio" {
						amount = spotBal.Available
					}
				}
				if err := db.SaveBalance(name, amount); err != nil {
					log.Warn("save balance %s: %v", name, err)
				}
			}
		}
		refreshBalances() // initial fetch
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			refreshBalances()
		}
	}()
	log.Info("Balance refresh started (60s interval)")

	// Start all components in order
	scanner.Start()
	scanner.StartDelistMonitor()
	log.Info("Discovery scanner started")

	riskMon.Start()
	log.Info("Risk monitor started")

	healthMon.Start()
	log.Info("Health monitor started")

	go apiSrv.Start()
	log.Info("Dashboard API server started")

	eng.MergeExistingDuplicates()
	eng.Start()
	log.Info("Engine started")

	log.Info("Bot fully initialized. Waiting for shutdown signal...")

	// Graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	fmt.Println()
	log.Info("Received signal %v, shutting down...", sig)

	// Stop in reverse order
	eng.Stop()
	log.Info("Engine stopped")

	apiSrv.Stop()
	log.Info("Dashboard stopped")

	healthMon.Stop()
	log.Info("Health monitor stopped")

	riskMon.Stop()
	log.Info("Risk monitor stopped")

	scanner.Stop()
	log.Info("Discovery scanner stopped")

	_ = db.Close()
	log.Info("Shutdown complete")
}

func newExchange(name string, cfg exchange.ExchangeConfig) (exchange.Exchange, error) {
	switch name {
	case "binance":
		return binance.NewAdapter(cfg), nil
	case "bitget":
		return bitget.NewAdapter(cfg), nil
	case "bybit":
		return bybit.NewAdapter(cfg), nil
	case "gateio":
		return gateio.NewAdapter(cfg), nil
	case "okx":
		return okx.NewAdapter(cfg), nil
	case "bingx":
		return bingx.NewAdapter(cfg), nil
	default:
		return nil, fmt.Errorf("unsupported exchange: %s", name)
	}
}

func exchangeConfig(cfg *config.Config, name string) exchange.ExchangeConfig {
	switch name {
	case "binance":
		return exchange.ExchangeConfig{
			Exchange:  "binance",
			ApiKey:    cfg.BinanceAPIKey,
			SecretKey: cfg.BinanceSecretKey,
		}
	case "bybit":
		return exchange.ExchangeConfig{
			Exchange:  "bybit",
			ApiKey:    cfg.BybitAPIKey,
			SecretKey: cfg.BybitSecretKey,
		}
	case "gateio":
		return exchange.ExchangeConfig{
			Exchange:  "gateio",
			ApiKey:    cfg.GateioAPIKey,
			SecretKey: cfg.GateioSecretKey,
		}
	case "bitget":
		return exchange.ExchangeConfig{
			Exchange:   "bitget",
			ApiKey:     cfg.BitgetAPIKey,
			SecretKey:  cfg.BitgetSecretKey,
			Passphrase: cfg.BitgetPassphrase,
		}
	case "okx":
		return exchange.ExchangeConfig{
			Exchange:   "okx",
			ApiKey:     cfg.OKXAPIKey,
			SecretKey:  cfg.OKXSecretKey,
			Passphrase: cfg.OKXPassphrase,
		}
	case "bingx":
		return exchange.ExchangeConfig{
			Exchange:  "bingx",
			ApiKey:    cfg.BingXAPIKey,
			SecretKey: cfg.BingXSecretKey,
		}
	default:
		return exchange.ExchangeConfig{Exchange: name}
	}
}
