package bitget

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

// SpotOrderRules returns lot/notional rules for the symbol from Bitget's
// spot symbols endpoint: GET /api/v2/spot/public/symbols?symbol={pair}
// Results are cached per symbol with a 5-minute TTL.
func (a *Adapter) SpotOrderRules(symbol string) (*exchange.SpotOrderRules, error) {
	spotRulesCacheMu.Lock()
	if entry, ok := spotRulesCache[symbol]; ok && time.Now().Before(entry.expiresAt) {
		spotRulesCacheMu.Unlock()
		return entry.rules, nil
	}
	spotRulesCacheMu.Unlock()

	raw, err := a.client.Get("/api/v2/spot/public/symbols", map[string]string{"symbol": symbol})
	if err != nil {
		return nil, fmt.Errorf("SpotOrderRules bitget %s: %w", symbol, err)
	}

	var resp struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
		Data []struct {
			MinTradeAmount    string `json:"minTradeAmount"`    // min base quantity (often "0" — treat as no base floor)
			QuantityPrecision string `json:"quantityPrecision"` // decimal places for quantity — current field
			QuantityScale     string `json:"quantityScale"`     // legacy v1 name, kept as fallback
			MinTradeUSDT      string `json:"minTradeUSDT"`      // min notional in USDT
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return nil, fmt.Errorf("SpotOrderRules bitget unmarshal %s: %w", symbol, err)
	}
	if resp.Code != "00000" {
		return nil, fmt.Errorf("SpotOrderRules bitget %s: code=%s msg=%s", symbol, resp.Code, resp.Msg)
	}
	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("SpotOrderRules bitget: no symbol info for %s", symbol)
	}

	item := resp.Data[0]
	minBase, _ := strconv.ParseFloat(item.MinTradeAmount, 64)
	minNotional, _ := strconv.ParseFloat(item.MinTradeUSDT, 64)

	// Prefer quantityPrecision (Bitget v2 current); fall back to legacy quantityScale
	// if an older response shape ever appears. Both carry the same "decimals" semantics
	// (e.g. "2" → step=0.01). Missing/zero → step=1 (integer-only).
	precisionStr := item.QuantityPrecision
	if precisionStr == "" {
		precisionStr = item.QuantityScale
	}
	qtyStep := 1.0
	if scale, err := strconv.Atoi(precisionStr); err == nil && scale > 0 {
		s := 1.0
		for i := 0; i < scale; i++ {
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
