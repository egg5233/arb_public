package binance

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

// SpotOrderRules returns lot/notional rules for the symbol from Binance's
// spot exchangeInfo endpoint: GET /api/v3/exchangeInfo?symbol={pair}
// Parses LOT_SIZE (minQty, stepSize) and MIN_NOTIONAL/NOTIONAL filters.
// Results are cached per symbol with a 5-minute TTL.
func (b *Adapter) SpotOrderRules(symbol string) (*exchange.SpotOrderRules, error) {
	spotRulesCacheMu.Lock()
	if entry, ok := spotRulesCache[symbol]; ok && time.Now().Before(entry.expiresAt) {
		spotRulesCacheMu.Unlock()
		return entry.rules, nil
	}
	spotRulesCacheMu.Unlock()

	data, err := b.client.SpotPublicGet("/api/v3/exchangeInfo", map[string]string{"symbol": symbol})
	if err != nil {
		return nil, fmt.Errorf("SpotOrderRules binance %s: %w", symbol, err)
	}

	var resp struct {
		Symbols []struct {
			Symbol  string `json:"symbol"`
			Filters []struct {
				FilterType  string `json:"filterType"`
				MinQty      string `json:"minQty"`
				StepSize    string `json:"stepSize"`
				MinNotional string `json:"minNotional"` // used by MIN_NOTIONAL and NOTIONAL filters
			} `json:"filters"`
		} `json:"symbols"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("SpotOrderRules binance unmarshal %s: %w", symbol, err)
	}
	if len(resp.Symbols) == 0 {
		return nil, fmt.Errorf("SpotOrderRules binance: no symbol info for %s", symbol)
	}

	var minBase, qtyStep, minNotional float64
	qtyStep = 1.0
	for _, f := range resp.Symbols[0].Filters {
		switch f.FilterType {
		case "LOT_SIZE":
			minBase, _ = strconv.ParseFloat(f.MinQty, 64)
			if s, err := strconv.ParseFloat(f.StepSize, 64); err == nil && s > 0 {
				qtyStep = s
			}
		case "MIN_NOTIONAL", "NOTIONAL":
			if minNotional == 0 {
				minNotional, _ = strconv.ParseFloat(f.MinNotional, 64)
			}
		}
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
