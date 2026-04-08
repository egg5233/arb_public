package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"arb/internal/analytics"
	"arb/internal/api"
	"arb/internal/config"
	"arb/internal/database"
	"arb/internal/discovery"
	"arb/internal/engine"
	"arb/internal/models"
	"arb/internal/notify"
	"arb/internal/risk"
	"arb/internal/scraper"
	"arb/internal/spotengine"
	"arb/pkg/exchange"
	"arb/pkg/exchange/binance"
	"arb/pkg/exchange/bingx"
	"arb/pkg/exchange/bitget"
	"arb/pkg/exchange/bybit"
	"arb/pkg/exchange/gateio"
	"arb/pkg/exchange/okx"
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

	// Config persistence architecture:
	// - config.json is the PRIMARY persistence layer. The dashboard POST handler
	//   (handlers.go) writes ALL 75+ config fields to config.json on every save.
	// - config.Load() already reads config.json at startup (before this point),
	//   so all dashboard-changed settings survive restarts automatically.
	// - Redis hash arb:config is a SECONDARY backup written alongside config.json.
	//   It is not loaded on startup because config.json is authoritative and complete.
	if n, err := db.GetAllConfig(); err != nil {
		log.Warn("Redis config hash unreadable (config.json is authoritative): %v", err)
	} else {
		log.Info("Redis config hash has %d fields (config.json is authoritative)", len(n))
	}

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

	// Detect unified/portfolio margin account modes.
	if ga, ok := exchanges["gateio"].(*gateio.Adapter); ok {
		ga.DetectUnifiedMode()
		if ga.IsUnified() {
			log.Info("Gate.io account mode: unified")
		} else {
			log.Info("Gate.io account mode: classic (fallback)")
		}
	}
	if oa, ok := exchanges["okx"].(*okx.Adapter); ok {
		oa.DetectUnifiedMode()
		if oa.IsUnified() {
			log.Info("OKX account mode: unified (level 3/4)")
		} else {
			log.Info("OKX account mode: standard (level 1/2)")
		}
	}
	if ba, ok := exchanges["binance"].(*binance.Adapter); ok {
		ba.DetectPortfolioMargin()
		if ba.IsUnified() {
			log.Info("Binance account mode: Portfolio Margin (unified)")
		} else {
			log.Info("Binance account mode: classic")
		}
	}

	// Check API key permissions
	permResults := make(map[string]exchange.PermissionResult)
	for name, exc := range exchanges {
		if checker, ok := exc.(exchange.PermissionChecker); ok {
			perm := checker.CheckPermissions()
			permResults[name] = perm
			log.Info("Permissions %s [%s]: read=%s trade=%s withdraw=%s transfer=%s",
				name, perm.Method, perm.Read, perm.FuturesTrade, perm.Withdraw, perm.Transfer)
			if perm.Error != "" {
				log.Warn("Permissions %s: %s", name, perm.Error)
			}
			if perm.Read == exchange.PermDenied || perm.FuturesTrade == exchange.PermDenied {
				log.Error("CRITICAL: %s missing required permission (read=%s trade=%s) — bot cannot trade on this exchange",
					name, perm.Read, perm.FuturesTrade)
			}
			if perm.Withdraw == exchange.PermDenied {
				log.Warn("%s: no withdraw permission — cross-exchange transfers disabled", name)
			}
			if perm.Transfer == exchange.PermDenied {
				log.Warn("%s: no transfer permission — spot↔futures rebalancing disabled", name)
			}
		}
	}

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
	scanner.InitTradingFees(exchanges)
	allocator := risk.NewCapitalAllocator(db, cfg)
	if err := allocator.Reconcile(); err != nil {
		log.Warn("capital allocator reconcile failed: %v", err)
	}
	if allocator.Enabled() {
		go func() {
			ticker := time.NewTicker(5 * time.Minute)
			defer ticker.Stop()
			for range ticker.C {
				if err := allocator.Reconcile(); err != nil {
					log.Warn("capital allocator periodic reconcile failed: %v", err)
				}
			}
		}()
	}
	scorer := risk.NewExchangeScorer(cfg)
	for name, exch := range exchanges {
		exchangeName := name
		exch.SetMetricsCallback(func(endpoint string, latency time.Duration, err error) {
			scorer.RecordLatency(exchangeName, endpoint, latency, err)
		})
		if setter, ok := exch.(exchange.WSMetricsCallbackSetter); ok {
			setter.SetWSMetricsCallback(func(event exchange.WSEvent) {
				scorer.RecordWSEvent(exchangeName, event)
			})
		}
		if setter, ok := exch.(exchange.OrderMetricsCallbackSetter); ok {
			setter.SetOrderMetricsCallback(func(event exchange.OrderMetricEvent) {
				scorer.RecordOrderEvent(exchangeName, event)
			})
		}
	}
	riskMgr := risk.NewManager(exchanges, db, cfg, allocator)
	riskMgr.SetSpreadHistoryProvider(scanner.GetSpreadHistorySnapshot)
	riskMgr.SetExchangeScorer(scorer)
	riskMon := risk.NewMonitor(exchanges, db, cfg)
	healthMon := risk.NewHealthMonitor(exchanges, db, cfg)
	apiSrv := api.NewServer(db, cfg, exchanges)
	apiSrv.SetPermissions(permResults)
	apiSrv.SetExchangeScorer(scorer)
	apiSrv.SetCapitalAllocator(allocator)
	eng := engine.NewEngine(exchanges, scanner, riskMgr, riskMon, healthMon, db, apiSrv, cfg, allocator)
	eng.SetContracts(allContracts)

	// Create shared Telegram notifier for both engines.
	tg := notify.NewTelegram(cfg.TelegramBotToken, cfg.TelegramChatID)
	eng.SetTelegram(tg)

	// Create and inject loss limit checker for pre-entry gating.
	lossLimiter := risk.NewLossLimitChecker(db, cfg, log, tg)
	eng.SetLossLimiter(lossLimiter)

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

	// ---------------------------------------------------------------------------
	// Analytics: SQLite store, snapshot writer, backfill (guarded by config)
	// ---------------------------------------------------------------------------
	var snapWriter *analytics.SnapshotWriter
	if cfg.EnableAnalytics {
		os.MkdirAll("data", 0755)

		analyticsStore, err := analytics.NewStore(cfg.AnalyticsDBPath)
		if err != nil {
			log.Error("analytics store init failed: %v (analytics disabled)", err)
		} else {
			defer analyticsStore.Close()

			// Inject into API server for /api/analytics/* endpoints.
			apiSrv.SetAnalyticsStore(analyticsStore)

			// Create and start background snapshot writer.
			snapWriter = analytics.NewSnapshotWriter(analyticsStore)
			snapWriter.Start()
			defer snapWriter.Stop()

			// Wire snapshot writer to perp engine.
			eng.SetSnapshotWriter(snapWriter)

			// Run backfill in background (non-blocking startup).
			go func() {
				perps, _ := db.GetHistory(500)
				spots, _ := db.GetSpotHistory(500)
				analytics.BackfillFromHistory(analyticsStore, perps, spots, log)
			}()

			log.Info("analytics enabled (db=%s)", cfg.AnalyticsDBPath)
		}
	} else {
		log.Info("analytics disabled (enable_analytics=false)")
	}

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
				// Gate.io unified: /spot/accounts returns the SAME pool as /unified/accounts.
				// Fetching spot separately would double-count. Skip for unified Gate.io.
				if !isSpotSameAsFutures(name, exc) {
					spotBal, err := exc.GetSpotBalance()
					if err == nil {
						db.SaveSpotBalance(name, spotBal.Available)
					}
				} else {
					db.SaveSpotBalance(name, 0)
				}
				// Fetch cross-margin USDT balance for exchanges with separate margin accounts.
				// Unified exchanges (Bybit UTA, OKX, Gate.io unified) already include
				// margin in the futures balance — fetching separately would double-count.
				if hasSeparateMargin(name, exc) {
					if smExch, ok := exc.(exchange.SpotMarginExchange); ok {
						if mb, err := smExch.GetMarginBalance("USDT"); err == nil {
							db.SaveMarginBalance(name, mb.NetBalance)
						}
					} else {
						db.SaveMarginBalance(name, 0)
					}
				} else if isSpotSameAsFutures(name, exc) {
					// Gate.io unified: also check isolated margin accounts for residual USDT.
					// These are separate from the unified pool and not included in GetFuturesBalance.
					if ga, ok := exc.(*gateio.Adapter); ok {
						if isoMargin, err := ga.GetIsolatedMarginUSDT(); err == nil {
							db.SaveMarginBalance(name, isoMargin)
						} else {
							log.Warn("balance refresh %s isolated margin: %v", name, err)
							// Don't overwrite — keep previous cached value on transient errors
						}
					} else {
						db.SaveMarginBalance(name, 0)
					}
				} else {
					db.SaveMarginBalance(name, 0)
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

	// Wire config hot-reload notifications
	notifier := apiSrv.ConfigNotifier()
	scanner.SetConfigNotify(notifier.Subscribe())
	riskMon.SetConfigNotify(notifier.Subscribe())
	healthMon.SetConfigNotify(notifier.Subscribe())

	// Exchange hot-reload: rebuild adapters when API keys or enabled state change.
	exchMgr := engine.NewExchangeManager(cfg)
	exchMgr.SetExchanges(exchanges)
	go func() {
		ch := notifier.Subscribe()
		for range ch {
			exchMgr.Reload()
		}
	}()

	// Start all components in order
	scanner.Start()
	if cfg.DelistFilterEnabled {
		scanner.StartDelistMonitor()
		log.Info("Binance delist monitor enabled")
		// New deliveryDate-based contract refresh poller. Reuses
		// DelistFilterEnabled — same blacklist key (arb:delist:{SYMBOL})
		// and same buffer days as the article scraper. Acts as the
		// primary delist signal; the article scraper at delist.go is
		// retained as belt-and-suspenders.
		if cfg.ContractRefreshInterval > 0 {
			scanner.StartContractRefresh()
			log.Info("Contract refresh poller started (interval: %s)", cfg.ContractRefreshInterval)
		} else {
			log.Info("Contract refresh poller disabled (contract_refresh_min=0)")
		}
	} else {
		log.Info("Binance delist monitor disabled by config")
	}
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

	// Start spot-futures arbitrage scraper if enabled.
	if cfg.SpotArbEnabled {
		scraper.StartSpotArbScraper(scraper.SpotArbConfig{
			Schedule:   cfg.SpotArbSchedule,
			ChromePath: cfg.SpotArbChromePath,
		}, db, log)
		log.Info("Spot-futures arbitrage scraper started")
	}

	// Start spot-futures arbitrage engine if enabled.
	var spotEng *spotengine.SpotEngine
	if cfg.SpotFuturesEnabled {
		spotEng = spotengine.NewSpotEngine(exchanges, db, apiSrv, cfg, allocator, tg)
		spotEng.SetConfigNotify(notifier.Subscribe(), notifier.Subscribe())
		if snapWriter != nil {
			spotEng.SetSnapshotWriter(snapWriter)
		}
		apiSrv.SetSpotOpenHandler(spotEng.ManualOpen)
		apiSrv.SetSpotCloseHandler(spotEng.ManualClose)
		apiSrv.SetSpotTestInjectHandler(spotEng.InjectTestOpportunity)
		apiSrv.SetSpotMaintenanceWarning(spotEng.MaintenanceWarning)
		spotEng.Start()
		eng.SetSpotCloseCallback(spotEng.ClosePosition)
		log.Info("Spot-futures engine started")
	}

	log.Info("Bot fully initialized. Waiting for shutdown signal...")

	// Graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	fmt.Println()
	log.Info("Received signal %v, shutting down...", sig)

	// Stop in reverse order
	if spotEng != nil {
		spotEng.Stop()
		log.Info("Spot-futures engine stopped")
	}

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

	// Close all exchange WebSocket connections.
	for name, exch := range exchanges {
		exch.Close()
		log.Info("Exchange %s WebSocket closed", name)
	}

	_ = db.Close()
	log.Info("Shutdown complete")

	// If the drift monitor triggered this shutdown, exit non-zero so
	// systemd Restart=on-failure restarts onto the new binary (ARB-87).
	if api.DriftRestartRequested() {
		log.Info("Drift restart requested — exiting with code 1 for supervisor restart")
		os.Exit(1)
	}
}

// isSpotSameAsFutures returns true when the spot wallet and futures wallet read
// from the same pool, so fetching both would double-count.
// Gate.io unified: /spot/accounts and /unified/accounts are the same USDT pool.
// All other exchanges have truly separate spot wallets (or spot=0 for Bybit FUND).
func isSpotSameAsFutures(name string, exc exchange.Exchange) bool {
	if name == "gateio" {
		type unifiedChecker interface{ IsUnified() bool }
		if uc, ok := exc.(unifiedChecker); ok {
			return uc.IsUnified()
		}
	}
	return false
}

// hasSeparateMargin returns true when an exchange has a distinct cross-margin
// account that is NOT already included in GetFuturesBalance/GetSpotBalance.
// For unified exchanges the margin collateral sits inside the futures (trading)
// balance, so fetching it separately would double-count.
// Gate.io is runtime-detected: classic mode has separate margin; unified does not.
func hasSeparateMargin(name string, _ exchange.Exchange) bool {
	switch name {
	case "binance", "bitget":
		return true
	default:
		// Gate.io: margin handled via GetIsolatedMarginUSDT (isSpotSameAsFutures path)
		// bybit, okx, bingx — all unified or no separate margin account
		return false
	}
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
