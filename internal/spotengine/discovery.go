package spotengine

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"arb/internal/models"
	"arb/internal/scraper"
	"arb/pkg/exchange"
)

// SpotArbOpportunity represents a scored spot-futures arbitrage opportunity.
type SpotArbOpportunity struct {
	Symbol          string    `json:"symbol"`
	BaseCoin        string    `json:"base_coin"`
	Exchange        string    `json:"exchange"`
	Direction       string    `json:"direction"` // "borrow_sell_long" or "buy_spot_short"
	FundingAPR      float64   `json:"funding_apr"`
	BorrowAPR       float64   `json:"borrow_apr"` // 0 for Direction B
	FeePct          float64   `json:"fee_pct"`
	NetAPR          float64   `json:"net_apr"`
	MaintenanceRate float64   `json:"maintenance_rate"` // tier-1 rate from ContractInfo (display only, per D-15)
	Source          string    `json:"source"`
	Timestamp       time.Time `json:"timestamp"`
	FilterStatus    string    `json:"filter_status,omitempty"` // empty = passed all filters
}

// borrowRateEntry is a cached borrow rate with expiry.
type borrowRateEntry struct {
	rate      *exchange.MarginInterestRate
	fetchedAt time.Time
}

const borrowRateTTL = 5 * time.Minute

// borrowCache stores cached borrow rates keyed by "exchange:coin".
var borrowCache sync.Map

// spotExchangeMap normalizes CoinGlass spot arb exchange names to internal IDs.
var spotExchangeMap = map[string]string{
	"Binance": "binance",
	"Bybit":   "bybit",
	"OKX":     "okx",
	"Bitget":  "bitget",
	"Gate":    "gateio",
	"Gate.io": "gateio",
}

// spotFeePct estimates the round-trip fee cost as a flat percentage.
// 4 legs (spot entry + futures entry + spot exit + futures exit), all taker.
// One-time cost across 4 taker legs (spot entry + futures entry + spot exit + futures exit).
var spotFees = map[string]float64{
	"binance": 0.05 * 0.8 / 100, // 0.04% taker after rebate
	"bybit":   0.055 * 0.8 / 100,
	"okx":     0.05 * 0.8 / 100,
	"bitget":  0.06 * 0.8 / 100,
	"gateio":  0.05 * 0.8 / 100,
}

const lorisURL = "https://api.loris.tools/funding"

// runDiscoveryScan routes to the configured scanner mode: native, coinglass, or both.
func (e *SpotEngine) runDiscoveryScan() []SpotArbOpportunity {
	switch e.cfg.SpotFuturesScannerMode {
	case "both":
		return e.runBothScanners()
	case "coinglass":
		return e.runCoinGlassFallback()
	default: // "native"
		return e.runNativeDiscoveryScan()
	}
}

// runBothScanners runs native and CoinGlass scanners, returning all results
// from both sources without deduplication. Each opportunity retains its Source
// field ("native" or "coinglass_spot") so the dashboard can display them separately.
func (e *SpotEngine) runBothScanners() []SpotArbOpportunity {
	native := e.runNativeDiscoveryScan()
	cg := e.runCoinGlassFallback()
	all := make([]SpotArbOpportunity, 0, len(native)+len(cg))
	all = append(all, native...)
	all = append(all, cg...)
	return all
}

// pollLoris fetches and parses the latest Loris funding rate data.
func (e *SpotEngine) pollLoris() (*models.LorisResponse, error) {
	return e.pollLorisFromURL(lorisURL)
}

// pollLorisFromURL fetches Loris data from a specific URL (used for testing).
func (e *SpotEngine) pollLorisFromURL(url string) (*models.LorisResponse, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("loris request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("loris returned %d: %s", resp.StatusCode, string(body))
	}

	var loris models.LorisResponse
	if err := json.NewDecoder(resp.Body).Decode(&loris); err != nil {
		return nil, fmt.Errorf("decode loris response: %w", err)
	}
	return &loris, nil
}

// runNativeDiscoveryScan uses Loris API + exchange borrow rates to produce
// ranked SpotArbOpportunity list. Falls back to CoinGlass on Loris failure.
func (e *SpotEngine) runNativeDiscoveryScan() []SpotArbOpportunity {
	loris, err := e.pollLoris()
	if err != nil {
		e.log.Warn("native discovery: Loris poll failed, falling back to CoinGlass: %v", err)
		return e.runCoinGlassFallback()
	}
	return e.runNativeDiscoveryScanFromLoris(loris)
}

// runNativeDiscoveryScanWithURL is a test helper that uses a custom Loris URL.
func (e *SpotEngine) runNativeDiscoveryScanWithURL(url string) []SpotArbOpportunity {
	loris, err := e.pollLorisFromURL(url)
	if err != nil {
		e.log.Warn("native discovery: Loris poll failed, falling back to CoinGlass: %v", err)
		return e.runCoinGlassFallback()
	}
	return e.runNativeDiscoveryScanFromLoris(loris)
}

// runNativeDiscoveryScanFromLoris processes a LorisResponse into ranked opportunities.
func (e *SpotEngine) runNativeDiscoveryScanFromLoris(loris *models.LorisResponse) []SpotArbOpportunity {

	// Build allowed exchanges set.
	allowedExchanges := make(map[string]bool)
	if len(e.cfg.SpotFuturesExchanges) > 0 {
		for _, ex := range e.cfg.SpotFuturesExchanges {
			allowedExchanges[strings.ToLower(ex)] = true
		}
	}

	// Build active-position keys so rate/yield filters don't drop symbols
	// that already have open positions.
	activeKeys := make(map[string]bool)
	if activePositions, err := e.db.GetActiveSpotPositions(); err == nil {
		for _, p := range activePositions {
			activeKeys[p.Symbol+":"+p.Exchange+":"+p.Direction] = true
		}
	} else {
		e.log.Warn("native discovery: GetActiveSpotPositions failed, active-position bypass disabled: %v", err)
	}

	now := time.Now().UTC()
	var opps []SpotArbOpportunity

	for _, baseSym := range loris.Symbols {
		if e.stopping() {
			e.log.Info("native discovery: shutdown requested, aborting scan early")
			return opps
		}
		symbol := strings.ToUpper(baseSym) + "USDT"

		for exchName, smExch := range e.spotMargin {
			if e.stopping() {
				e.log.Info("native discovery: shutdown requested, aborting scan early")
				return opps
			}
			// Check exchange is in allowed list (if configured).
			if len(allowedExchanges) > 0 && !allowedExchanges[exchName] {
				continue
			}

			// Look up funding rate for this exchange + symbol.
			exchRates, ok := loris.FundingRates[exchName]
			if !ok {
				continue
			}
			rawRate, ok := exchRates[baseSym]
			if !ok {
				continue
			}

			// Loris rate normalization (per project_loris_normalization.md):
			// Loris returns 8h-equivalent basis points.
			// Divide by 8 to get bps/hour.
			// Multiply by 8760 (hours/year) and divide by 10000 (bps to decimal) = APR.
			bpsPerHour := rawRate / 8.0
			fundingAPR := bpsPerHour * 8760.0 / 10000.0

			// Calculate one-time fee % (4 taker legs, no annualization).
			takerFee := spotFees[exchName]
			if takerFee == 0 {
				takerFee = 0.0005 // default 0.05%
			}
			totalRoundTripFee := takerFee * 4
			feePct := totalRoundTripFee

			// Dir A (long futures): pays funding when rate is positive, receives when negative.
			// Dir B (short futures): receives funding when rate is positive, pays when negative.
			dirAFundingAPR := -fundingAPR // long futures: opposite sign
			dirBFundingAPR := fundingAPR  // short futures: same sign
			spotAvailable, spotAvailErr := e.checkSpotMarketAvailable(exchName, symbol, smExch)
			spotFilterStatus := ""
			if spotAvailErr != nil {
				spotFilterStatus = "spot market check failed"
			} else if !spotAvailable {
				spotFilterStatus = "spot market unavailable"
			}

			// Per D-13: fetch maintenance_rate from ContractInfo for display.
			// Compute planned notional for tier matching (mirrors risk_gate.go pattern).
			maintRate := e.lookupMaintenanceRateForDisplay(symbol, exchName)

			// --- Dir A: borrow_sell_long ---
			{
				isActive := activeKeys[symbol+":"+exchName+":borrow_sell_long"]
				filterStatus := spotFilterStatus
				var borrowAPR float64

				if filterStatus == "" {
					rate, err := e.getCachedBorrowRate(exchName, strings.ToUpper(baseSym), smExch)
					if err != nil {
						filterStatus = "borrow rate unavailable"
					} else {
						borrowAPR = rate.HourlyRate * 24 * 365
						if e.cfg.SpotFuturesMaxBorrowAPR > 0 && borrowAPR > e.cfg.SpotFuturesMaxBorrowAPR && !isActive {
							filterStatus = fmt.Sprintf("borrow %.0f%% > max %.0f%%", borrowAPR*100, e.cfg.SpotFuturesMaxBorrowAPR*100)
						}
					}
				}

				netAPR := dirAFundingAPR - borrowAPR

				if filterStatus == "" && netAPR < e.cfg.SpotFuturesMinNetYieldAPR && !isActive {
					filterStatus = fmt.Sprintf("net %.1f%% < min %.1f%%", netAPR*100, e.cfg.SpotFuturesMinNetYieldAPR*100)
				}

				opps = append(opps, SpotArbOpportunity{
					Symbol:          symbol,
					BaseCoin:        strings.ToUpper(baseSym),
					Exchange:        exchName,
					Direction:       "borrow_sell_long",
					FundingAPR:      dirAFundingAPR,
					BorrowAPR:       borrowAPR,
					FeePct:          feePct,
					NetAPR:          netAPR,
					MaintenanceRate: maintRate,
					Source:          "native",
					Timestamp:       now,
					FilterStatus:    filterStatus,
				})
			}

			// --- Dir B: buy_spot_short ---
			{
				isActive := activeKeys[symbol+":"+exchName+":buy_spot_short"]
				filterStatus := spotFilterStatus
				borrowAPR := 0.0 // no borrow for Dir B
				netAPR := dirBFundingAPR - borrowAPR

				if filterStatus == "" && netAPR < e.cfg.SpotFuturesMinNetYieldAPR && !isActive {
					filterStatus = fmt.Sprintf("net %.1f%% < min %.1f%%", netAPR*100, e.cfg.SpotFuturesMinNetYieldAPR*100)
				}

				opps = append(opps, SpotArbOpportunity{
					Symbol:          symbol,
					BaseCoin:        strings.ToUpper(baseSym),
					Exchange:        exchName,
					Direction:       "buy_spot_short",
					FundingAPR:      dirBFundingAPR,
					BorrowAPR:       borrowAPR,
					FeePct:          feePct,
					NetAPR:          netAPR,
					MaintenanceRate: maintRate,
					Source:          "native",
					Timestamp:       now,
					FilterStatus:    filterStatus,
				})
			}
		}
	}

	// Sort by NetAPR descending (passed filters first, then filtered).
	sort.Slice(opps, func(i, j int) bool {
		pi, pj := opps[i].FilterStatus == "", opps[j].FilterStatus == ""
		if pi != pj {
			return pi
		}
		return opps[i].NetAPR > opps[j].NetAPR
	})

	e.log.Info("native discovery: %d opportunities from Loris (%d symbols)", len(opps), len(loris.Symbols))
	return opps
}

// runCoinGlassFallback reads CoinGlass spot arb data from Redis, scores all
// opportunities by net yield. Every opportunity is returned (for dashboard
// display); those that fail entry filters have FilterStatus set to a reason
// string so the UI can distinguish actionable from informational rows.
func (e *SpotEngine) runCoinGlassFallback() []SpotArbOpportunity {
	// 1. Read CoinGlass spot arb data from Redis.
	raw, err := e.db.Get("coinGlassSpotArb")
	if err != nil {
		e.log.Error("spot discovery: failed to read coinGlassSpotArb: %v", err)
		return nil
	}

	var payload scraper.Payload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		e.log.Error("spot discovery: failed to parse coinGlassSpotArb: %v", err)
		return nil
	}

	// Staleness check: skip if data is older than 60 minutes.
	ts, err := time.Parse(time.RFC3339, payload.Timestamp)
	if err != nil {
		e.log.Warn("spot discovery: failed to parse timestamp %q, skipping", payload.Timestamp)
		return nil
	}
	if time.Since(ts) > 60*time.Minute {
		e.log.Warn("spot discovery: data is %.0f min old, skipping", time.Since(ts).Minutes())
		return nil
	}

	e.log.Info("spot discovery: read %d items from coinGlassSpotArb (age: %s)",
		len(payload.Data), time.Since(ts).Round(time.Second))

	// 2. Build allowed exchanges set.
	allowedExchanges := make(map[string]bool)
	if len(e.cfg.SpotFuturesExchanges) > 0 {
		for _, ex := range e.cfg.SpotFuturesExchanges {
			allowedExchanges[strings.ToLower(ex)] = true
		}
	}

	// Build active-position keys so rate/yield filters don't drop symbols
	// that already have open positions (monitor/exit still needs fresh data).
	activeKeys := make(map[string]bool)
	if activePositions, err := e.db.GetActiveSpotPositions(); err == nil {
		for _, p := range activePositions {
			activeKeys[p.Symbol+":"+p.Exchange+":"+p.Direction] = true
		}
	} else {
		e.log.Warn("spot discovery: GetActiveSpotPositions failed, active-position bypass disabled: %v", err)
	}

	now := time.Now().UTC()
	var opps []SpotArbOpportunity

	for _, item := range payload.Data {
		if e.stopping() {
			e.log.Info("spot discovery: shutdown requested, aborting fallback scan early")
			return opps
		}
		// Normalize exchange name.
		exchName, ok := spotExchangeMap[item.Exchange]
		if !ok {
			continue
		}

		// Check exchange is in allowed list (if configured).
		if len(allowedExchanges) > 0 && !allowedExchanges[exchName] {
			continue
		}

		// Check exchange implements SpotMarginExchange.
		_, ok = e.spotMargin[exchName]
		if !ok {
			continue
		}

		// Determine direction from Portfolio field.
		direction, baseCoin := parsePortfolio(item.Symbol, item.Portfolio)
		if direction == "" || baseCoin == "" {
			continue
		}

		spotSymbol := normalizeSymbol(item.Symbol)
		isActive := activeKeys[spotSymbol+":"+exchName+":"+direction]

		// Parse funding APR from the APR field (e.g. "43.21%"). Keep zero/negative
		// funding rows in the cache as filtered so active positions still expose
		// live economics to monitor/exit paths.
		fundingAPR := parsePercent(item.APR)

		// Calculate one-time fee % (4 taker legs, no annualization).
		takerFee := spotFees[exchName]
		if takerFee == 0 {
			takerFee = 0.0005 // default 0.05%
		}
		totalRoundTripFee := takerFee * 4
		feePct := totalRoundTripFee

		// --- Entry filters: mark failures in FilterStatus instead of skipping ---
		var filterStatus string
		var borrowAPR float64
		if fundingAPR <= 0 {
			filterStatus = fmt.Sprintf("funding %.1f%% <= 0", fundingAPR*100)
		}

		// Check spot margin availability for all directions — ensures the
		// symbol actually exists on the exchange's cross-margin market.
		// For borrow_sell_long, also check borrowability and borrow rate.
		smExch := e.spotMargin[exchName]
		spotAvailable, spotAvailErr := e.checkSpotMarketAvailable(exchName, spotSymbol, smExch)
		if spotAvailErr != nil {
			filterStatus = "spot market check failed"
		} else if !spotAvailable {
			filterStatus = "spot market unavailable"
		}
		if filterStatus == "" {
			spotBal, err := smExch.GetMarginBalance(baseCoin)
			if err != nil {
				filterStatus = "margin unavailable"
			} else if direction == "borrow_sell_long" && spotBal.MaxBorrowable <= 0 {
				filterStatus = "not borrowable"
			}
		}

		if direction == "borrow_sell_long" {
			// Fetch borrow rate.
			if filterStatus == "" {
				rate, err := e.getCachedBorrowRate(exchName, baseCoin, smExch)
				if err != nil {
					filterStatus = "borrow rate unavailable"
				} else {
					borrowAPR = rate.HourlyRate * 24 * 365
					if e.cfg.SpotFuturesMaxBorrowAPR > 0 && borrowAPR > e.cfg.SpotFuturesMaxBorrowAPR && !isActive {
						filterStatus = fmt.Sprintf("borrow %.0f%% > max %.0f%%", borrowAPR*100, e.cfg.SpotFuturesMaxBorrowAPR*100)
					}
				}
			}
		}

		netAPR := fundingAPR - borrowAPR

		// Check net yield threshold.
		if filterStatus == "" && netAPR < e.cfg.SpotFuturesMinNetYieldAPR && !isActive {
			filterStatus = fmt.Sprintf("net %.1f%% < min %.1f%%", netAPR*100, e.cfg.SpotFuturesMinNetYieldAPR*100)
		}

		if filterStatus != "" && !isActive {
		}

		// Per D-13: fetch maintenance_rate from ContractInfo for display.
		maintRate := e.lookupMaintenanceRateForDisplay(spotSymbol, exchName)

		opps = append(opps, SpotArbOpportunity{
			Symbol:          spotSymbol,
			BaseCoin:        baseCoin,
			Exchange:        exchName,
			Direction:       direction,
			FundingAPR:      fundingAPR,
			BorrowAPR:       borrowAPR,
			FeePct:          feePct,
			NetAPR:          netAPR,
			MaintenanceRate: maintRate,
			Source:          "coinglass_spot",
			Timestamp:       now,
			FilterStatus:    filterStatus,
		})
	}

	// Rank by netAPR descending (passed filters first, then filtered).
	sort.Slice(opps, func(i, j int) bool {
		// Passed filters sort before filtered.
		pi, pj := opps[i].FilterStatus == "", opps[j].FilterStatus == ""
		if pi != pj {
			return pi
		}
		return opps[i].NetAPR > opps[j].NetAPR
	})

	return opps
}

// getFreshBorrowRate always fetches a live rate from the exchange and updates the cache.
// Used by the monitor path where per-tick freshness matters for safety triggers.
func (e *SpotEngine) getFreshBorrowRate(exchName, coin string, smExch exchange.SpotMarginExchange) (*exchange.MarginInterestRate, error) {
	rate, err := smExch.GetMarginInterestRate(coin)
	if err != nil {
		return nil, fmt.Errorf("GetMarginInterestRate(%s): %w", coin, err)
	}

	cacheKey := exchName + ":" + coin
	borrowCache.Store(cacheKey, &borrowRateEntry{
		rate:      rate,
		fetchedAt: time.Now(),
	})

	return rate, nil
}

// getCachedBorrowRate returns a cached borrow rate or fetches a fresh one.
func (e *SpotEngine) getCachedBorrowRate(exchName, coin string, smExch exchange.SpotMarginExchange) (*exchange.MarginInterestRate, error) {
	cacheKey := exchName + ":" + coin

	if cached, ok := borrowCache.Load(cacheKey); ok {
		entry := cached.(*borrowRateEntry)
		if time.Since(entry.fetchedAt) < borrowRateTTL {
			return entry.rate, nil
		}
	}

	rate, err := smExch.GetMarginInterestRate(coin)
	if err != nil {
		return nil, fmt.Errorf("GetMarginInterestRate(%s): %w", coin, err)
	}

	borrowCache.Store(cacheKey, &borrowRateEntry{
		rate:      rate,
		fetchedAt: time.Now(),
	})

	return rate, nil
}

// parsePortfolio extracts direction and base coin from the CoinGlass Portfolio field.
// Examples: "Sell BTC" → ("borrow_sell_long", "BTC"), "Buy ETH" → ("buy_spot_short", "ETH")
func parsePortfolio(symbol, portfolio string) (direction, baseCoin string) {
	portfolio = strings.TrimSpace(portfolio)

	if strings.HasPrefix(portfolio, "Sell") {
		direction = "borrow_sell_long"
	} else if strings.HasPrefix(portfolio, "Buy") {
		direction = "buy_spot_short"
	} else {
		return "", ""
	}

	// Extract base coin: try from portfolio text first, fall back to symbol.
	parts := strings.Fields(portfolio)
	if len(parts) >= 2 {
		// "Sell BTC" or "Buy ETH/Sell ETHUSDT" — take the first coin token.
		coin := parts[1]
		coin = strings.Split(coin, "/")[0] // handle "BTC/Sell" format
		baseCoin = strings.ToUpper(coin)
	}

	if baseCoin == "" {
		// Fallback: derive from symbol (e.g. "BTCUSDT" → "BTC").
		baseCoin = strings.TrimSuffix(strings.ToUpper(symbol), "USDT")
	}

	return direction, baseCoin
}

// normalizeSymbol ensures the symbol ends with USDT and is uppercase.
func normalizeSymbol(s string) string {
	s = strings.ToUpper(strings.TrimSpace(s))
	if !strings.HasSuffix(s, "USDT") {
		s += "USDT"
	}
	return s
}

// parsePercent parses "43.21%" into a decimal (0.4321).
func parsePercent(s string) float64 {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, "%")
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return f / 100
}

func (e *SpotEngine) checkSpotMarketAvailable(exchName, symbol string, smExch exchange.SpotMarginExchange) (bool, error) {
	if cached, found, err := e.db.GetSpotMarketAvailability(exchName, symbol); err == nil && found {
		return cached, nil
	} else if err != nil {
		e.log.Warn("spot discovery: spot-market cache read failed for %s on %s: %v", symbol, exchName, err)
	}

	if _, err := smExch.GetSpotBBO(symbol); err != nil {
		if isMissingSpotMarketError(err) {
			if cacheErr := e.db.SetSpotMarketAvailability(exchName, symbol, false); cacheErr != nil {
				e.log.Warn("spot discovery: failed to cache missing spot market for %s on %s: %v", symbol, exchName, cacheErr)
			}
			return false, nil
		}
		return false, err
	}

	if err := e.db.SetSpotMarketAvailability(exchName, symbol, true); err != nil {
		e.log.Warn("spot discovery: failed to cache spot market for %s on %s: %v", symbol, exchName, err)
	}
	return true, nil
}

func isMissingSpotMarketError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "no okx spot market") ||
		strings.Contains(msg, "doesn't exist") ||
		strings.Contains(msg, "does not exist") ||
		strings.Contains(msg, "invalid symbol") ||
		strings.Contains(msg, "no data for")
}

// lookupMaintenanceRateForDisplay returns the maintenance margin rate for a
// symbol on an exchange, suitable for display in the opportunities table.
// Uses planned notional from config for tier matching (mirrors risk_gate.go).
// Returns 0 if unavailable (UI shows dash). Per D-15: display only, no scoring.
func (e *SpotEngine) lookupMaintenanceRateForDisplay(symbol, exchName string) float64 {
	capitalPerLeg := e.cfg.SpotFuturesCapitalSeparate
	if !isSeparateAccount(exchName) {
		capitalPerLeg = e.cfg.SpotFuturesCapitalUnified
	}
	leverage := float64(e.cfg.SpotFuturesLeverage)
	if leverage <= 0 {
		leverage = 3.0
	}
	plannedNotional := capitalPerLeg * leverage
	return e.getMaintenanceRate(symbol, exchName, plannedNotional)
}

// logDiscoveryResults logs the top opportunities in a table format.
func (e *SpotEngine) logDiscoveryResults(opps []SpotArbOpportunity) {
	// if len(opps) == 0 {
	// 	e.log.Info("spot discovery: no opportunities passed filters")
	// 	return
	// }

	// e.log.Info("spot discovery: %d opportunities found", len(opps))
	// for i, opp := range opps {
	// 	borrowStr := "n/a"
	// 	if opp.Direction == "borrow_sell_long" {
	// 		borrowStr = fmt.Sprintf("%.1f%%", opp.BorrowAPR*100)
	// 	}
	// 	e.log.Info("  [%d] %s on %s (%s) | Funding: %.1f%% | Interest: %s | Fees: %.1f%% | Net: %.1f%% APR",
	// 		i+1, opp.Symbol, opp.Exchange, opp.Direction,
	// 		opp.FundingAPR*100, borrowStr, opp.FeePct*100, opp.NetAPR*100)
	// }
}
