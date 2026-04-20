package okx

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

// SpotOrderRules returns lot/notional rules for the symbol from OKX's
// spot instruments endpoint: GET /api/v5/public/instruments?instType=SPOT&instId={pair}
// Results are cached per symbol with a 5-minute TTL.
func (a *Adapter) SpotOrderRules(symbol string) (*exchange.SpotOrderRules, error) {
	spotRulesCacheMu.Lock()
	if entry, ok := spotRulesCache[symbol]; ok && time.Now().Before(entry.expiresAt) {
		spotRulesCacheMu.Unlock()
		return entry.rules, nil
	}
	spotRulesCacheMu.Unlock()

	instID := toOKXSpotInstID(symbol) // BTCUSDT -> BTC-USDT
	data, err := a.client.Get("/api/v5/public/instruments", map[string]string{
		"instType": "SPOT",
		"instId":   instID,
	})
	if err != nil {
		return nil, fmt.Errorf("SpotOrderRules okx %s: %w", symbol, err)
	}

	// OKX client already unwraps the envelope and returns Data directly.
	var instruments []struct {
		MinSz       string `json:"minSz"`       // minimum order size (base)
		LotSz       string `json:"lotSz"`       // lot size / step
		MinNotional string `json:"minNotional"` // minimum notional (may be absent)
	}
	if err := json.Unmarshal(data, &instruments); err != nil {
		return nil, fmt.Errorf("SpotOrderRules okx unmarshal %s: %w", symbol, err)
	}
	if len(instruments) == 0 {
		return nil, fmt.Errorf("SpotOrderRules okx: no instrument info for %s", symbol)
	}

	item := instruments[0]
	minBase, _ := strconv.ParseFloat(item.MinSz, 64)
	qtyStep := 1.0
	if s, err := strconv.ParseFloat(item.LotSz, 64); err == nil && s > 0 {
		qtyStep = s
	}
	minNotional, _ := strconv.ParseFloat(item.MinNotional, 64)

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
