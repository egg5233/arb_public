package gateio

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

// SpotOrderRules returns lot/notional rules for the symbol from Gate.io's
// spot currency-pair endpoint: GET /spot/currency_pairs/{pair}
// Results are cached per symbol with a 5-minute TTL.
func (a *Adapter) SpotOrderRules(symbol string) (*exchange.SpotOrderRules, error) {
	spotRulesCacheMu.Lock()
	if entry, ok := spotRulesCache[symbol]; ok && time.Now().Before(entry.expiresAt) {
		spotRulesCacheMu.Unlock()
		return entry.rules, nil
	}
	spotRulesCacheMu.Unlock()

	pair := toGateSymbol(symbol) // BTCUSDT -> BTC_USDT
	data, err := a.client.Get("/spot/currency_pairs/"+pair, nil)
	if err != nil {
		return nil, fmt.Errorf("SpotOrderRules gateio %s: %w", symbol, err)
	}

	var resp struct {
		MinBaseAmount   string `json:"min_base_amount"`
		AmountPrecision int    `json:"amount_precision"` // decimal places for amount
		MinQuoteAmount  string `json:"min_quote_amount"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("SpotOrderRules gateio unmarshal %s: %w", symbol, err)
	}

	minBase, _ := strconv.ParseFloat(resp.MinBaseAmount, 64)
	minNotional, _ := strconv.ParseFloat(resp.MinQuoteAmount, 64)

	// QtyStep derived from amount_precision (e.g. precision=0 → step=1, precision=2 → step=0.01)
	qtyStep := 1.0
	if resp.AmountPrecision > 0 {
		s := 1.0
		for i := 0; i < resp.AmountPrecision; i++ {
			s /= 10
		}
		qtyStep = s
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
