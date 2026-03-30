package spotengine

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"arb/internal/scraper"
	"arb/pkg/exchange"
)

// SpotArbOpportunity represents a scored spot-futures arbitrage opportunity.
type SpotArbOpportunity struct {
	Symbol     string    `json:"symbol"`
	BaseCoin   string    `json:"base_coin"`
	Exchange   string    `json:"exchange"`
	Direction  string    `json:"direction"` // "borrow_sell_long" or "buy_spot_short"
	FundingAPR float64   `json:"funding_apr"`
	BorrowAPR  float64   `json:"borrow_apr"` // 0 for Direction B
	FeeAPR     float64   `json:"fee_apr"`
	NetAPR     float64   `json:"net_apr"`
	Source     string    `json:"source"`
	Timestamp  time.Time `json:"timestamp"`
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

// spotFeeAPR estimates the round-trip fee cost as an annualized rate.
// 4 legs (spot entry + futures entry + spot exit + futures exit), all taker.
// Assumes ~30 day hold for annualization: feeAPR = totalFeePct * (365/30).
var spotFees = map[string]float64{
	"binance": 0.05 * 0.8 / 100, // 0.04% taker after rebate
	"bybit":   0.055 * 0.8 / 100,
	"okx":     0.05 * 0.8 / 100,
	"bitget":  0.06 * 0.8 / 100,
	"gateio":  0.05 * 0.8 / 100,
}

const assumedHoldDays = 30.0

// runDiscoveryScan reads CoinGlass spot arb data from Redis, scores
// opportunities by net yield, and returns the top ranked results.
func (e *SpotEngine) runDiscoveryScan() []SpotArbOpportunity {
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

	now := time.Now().UTC()
	var opps []SpotArbOpportunity

	for _, item := range payload.Data {
		// Normalize exchange name.
		exchName, ok := spotExchangeMap[item.Exchange]
		if !ok {
			continue
		}

		// Check exchange is in allowed list (if configured).
		if len(allowedExchanges) > 0 && !allowedExchanges[exchName] {
			e.log.Info("spot discovery: FILTERED %s on %s — exchange not in allowed list", item.Symbol, exchName)
			continue
		}

		// Check exchange implements SpotMarginExchange.
		smExch, ok := e.spotMargin[exchName]
		if !ok {
			e.log.Info("spot discovery: FILTERED %s on %s — no SpotMarginExchange support", item.Symbol, exchName)
			continue
		}

		// Determine direction from Portfolio field.
		direction, baseCoin := parsePortfolio(item.Symbol, item.Portfolio)
		if direction == "" || baseCoin == "" {
			e.log.Info("spot discovery: FILTERED %s on %s — cannot parse portfolio: %q", item.Symbol, exchName, item.Portfolio)
			continue
		}

		spotSymbol := normalizeSymbol(item.Symbol)

		// For Direction A (borrow-sell), verify spot margin supports this coin.
		// For Direction B (buy spot), we don't use spot margin — skip margin check.
		var spotBal *exchange.MarginBalance
		if direction == "borrow_sell_long" {
			var err error
			spotBal, err = smExch.GetMarginBalance(baseCoin)
			if err != nil {
				e.log.Info("spot discovery: FILTERED %s on %s — spot margin not available: %v", spotSymbol, exchName, err)
				continue
			}
		}

		// Parse funding APR from the APR field (e.g. "43.21%").
		fundingAPR := parsePercent(item.APR)
		if fundingAPR <= 0 {
			e.log.Info("spot discovery: FILTERED %s on %s — invalid funding APR: %q", spotSymbol, exchName, item.APR)
			continue
		}

		// Calculate borrow APR (Direction A only).
		var borrowAPR float64
		if direction == "borrow_sell_long" {
			rate, err := e.getCachedBorrowRate(exchName, baseCoin, smExch)
			if err != nil {
				e.log.Info("spot discovery: FILTERED %s on %s — borrow unavailable: %v", spotSymbol, exchName, err)
				continue
			}
			borrowAPR = rate.HourlyRate * 24 * 365

			// Filter: borrow too expensive.
			if borrowAPR > e.cfg.SpotFuturesMaxBorrowAPR {
				e.log.Info("spot discovery: FILTERED %s on %s — interest APR %.1f%% > max %.1f%%",
					spotSymbol, exchName, borrowAPR*100, e.cfg.SpotFuturesMaxBorrowAPR*100)
				continue
			}

			// Verify coin is actually borrowable (interest rate API may return rates
			// for coins that cannot be borrowed).
			if spotBal.MaxBorrowable <= 0 {
				e.log.Info("spot discovery: FILTERED %s on %s — not borrowable (maxBorrowable=0)", spotSymbol, exchName)
				continue
			}
		}

		// Calculate fee APR (4 taker legs, annualized over assumed hold).
		takerFee := spotFees[exchName]
		if takerFee == 0 {
			takerFee = 0.0005 // default 0.05%
		}
		totalRoundTripFee := takerFee * 4 // spot in + futures in + spot out + futures out
		feeAPR := totalRoundTripFee * (365.0 / assumedHoldDays)

		// Net yield.
		netAPR := fundingAPR - borrowAPR - feeAPR

		// Filter: net yield too low.
		if netAPR < e.cfg.SpotFuturesMinNetYieldAPR {
			e.log.Info("spot discovery: FILTERED %s on %s — net APR %.1f%% < min %.1f%% (funding=%.1f%% borrow=%.1f%% fees=%.1f%%)",
				item.Symbol, exchName, netAPR*100, e.cfg.SpotFuturesMinNetYieldAPR*100,
				fundingAPR*100, borrowAPR*100, feeAPR*100)
			continue
		}

		opps = append(opps, SpotArbOpportunity{
			Symbol:     normalizeSymbol(item.Symbol),
			BaseCoin:   baseCoin,
			Exchange:   exchName,
			Direction:  direction,
			FundingAPR: fundingAPR,
			BorrowAPR:  borrowAPR,
			FeeAPR:     feeAPR,
			NetAPR:     netAPR,
			Source:     "coinglass_spot",
			Timestamp:  now,
		})
	}

	// Rank by netAPR descending.
	sort.Slice(opps, func(i, j int) bool {
		return opps[i].NetAPR > opps[j].NetAPR
	})

	// Limit to top N.
	topN := e.cfg.SpotFuturesMaxPositions * 3 // show 3x max positions
	if topN < 5 {
		topN = 5
	}
	if len(opps) > topN {
		opps = opps[:topN]
	}

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

// logDiscoveryResults logs the top opportunities in a table format.
func (e *SpotEngine) logDiscoveryResults(opps []SpotArbOpportunity) {
	if len(opps) == 0 {
		e.log.Info("spot discovery: no opportunities passed filters")
		return
	}

	e.log.Info("spot discovery: %d opportunities found", len(opps))
	for i, opp := range opps {
		borrowStr := "n/a"
		if opp.Direction == "borrow_sell_long" {
			borrowStr = fmt.Sprintf("%.1f%%", opp.BorrowAPR*100)
		}
		e.log.Info("  [%d] %s on %s (%s) | Funding: %.1f%% | Interest: %s | Fees: %.1f%% | Net: %.1f%% APR",
			i+1, opp.Symbol, opp.Exchange, opp.Direction,
			opp.FundingAPR*100, borrowStr, opp.FeeAPR*100, opp.NetAPR*100)
	}
}
