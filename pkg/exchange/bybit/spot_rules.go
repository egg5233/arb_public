package bybit

import (
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	"arb/pkg/exchange"
)

type spotRulesCacheEntry struct {
	rules     *exchange.SpotOrderRules
	expiresAt time.Time
}

var (
	spotRulesCacheMu sync.Mutex
	spotRulesCache   = make(map[string]spotRulesCacheEntry)
)

const spotRulesTTL = 5 * time.Minute

// SpotOrderRules returns lot/notional rules for the symbol from Bybit's
// spot instruments endpoint: GET /v5/market/instruments-info?category=spot&symbol={pair}
// Results are cached per symbol with a 5-minute TTL.
func (a *Adapter) SpotOrderRules(symbol string) (*exchange.SpotOrderRules, error) {
	spotRulesCacheMu.Lock()
	if entry, ok := spotRulesCache[symbol]; ok && time.Now().Before(entry.expiresAt) {
		spotRulesCacheMu.Unlock()
		return entry.rules, nil
	}
	spotRulesCacheMu.Unlock()

	raw, err := a.client.Get("/v5/market/instruments-info", map[string]string{
		"category": "spot",
		"symbol":   symbol,
	})
	if err != nil {
		return nil, fmt.Errorf("SpotOrderRules bybit %s: %w", symbol, err)
	}

	// Bybit client already unwraps the envelope and returns Result directly.
	var result struct {
		List []struct {
			LotSizeFilter struct {
				MinOrderQty   string `json:"minOrderQty"`
				BasePrecision string `json:"basePrecision"`
			} `json:"lotSizeFilter"`
			QuoteFilter struct {
				MinOrderAmt string `json:"minOrderAmt"`
			} `json:"quoteFilter"`
		} `json:"list"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("SpotOrderRules bybit unmarshal %s: %w", symbol, err)
	}
	if len(result.List) == 0 {
		return nil, fmt.Errorf("SpotOrderRules bybit: no instrument info for %s", symbol)
	}

	item := result.List[0]
	minBase, _ := strconv.ParseFloat(item.LotSizeFilter.MinOrderQty, 64)
	minNotional, _ := strconv.ParseFloat(item.QuoteFilter.MinOrderAmt, 64)

	// basePrecision is the step size (e.g. "0.000001" means 6dp)
	qtyStep := 1.0
	if s, err := strconv.ParseFloat(item.LotSizeFilter.BasePrecision, 64); err == nil && s > 0 {
		qtyStep = s
	}
	// Fallback: if step > minBase, use minBase as step
	if qtyStep > minBase && minBase > 0 {
		qtyStep = minBase
	}

	rules := &exchange.SpotOrderRules{
		MinBaseQty:  minBase,
		QtyStep:     qtyStep,
		MinNotional: minNotional,
	}

	spotRulesCacheMu.Lock()
	spotRulesCache[symbol] = spotRulesCacheEntry{rules: rules, expiresAt: time.Now().Add(spotRulesTTL)}
	spotRulesCacheMu.Unlock()

	return rules, nil
}
