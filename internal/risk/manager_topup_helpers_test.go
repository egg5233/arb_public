package risk

import (
	"testing"

	"arb/internal/config"
	"arb/pkg/exchange"
)

// topupTestCfg returns a config tuned for top-up tests: small CapitalPerLeg so
// that a TransferablePerExchange of 200 is sufficient to cover the margin
// deficit when Available starts at 50.
//   needed = 100, bufferedNeed = 110, deficit ≈ 76 (covers both buffer and
//   post-trade L4 ratio).  200 > 76 → top-up succeeds.
func topupTestCfg() *config.Config {
	cfg := defaultCfg()
	cfg.CapitalPerLeg = 100
	return cfg
}

// newTestManagerWithBalances creates a Manager with the given per-exchange
// futures balances. It uses topupTestCfg (CapitalPerLeg=100) so that a
// TransferablePerExchange of 200 is sufficient to cover margin deficits
// when Available starts at 50.
func newTestManagerWithBalances(t *testing.T, balances map[string]*exchange.Balance) *Manager {
	t.Helper()
	exchanges := make(map[string]exchange.Exchange, len(balances))
	for name, bal := range balances {
		b := bal // capture
		exchanges[name] = &managerStubExchange{
			name:       name,
			futuresBal: b,
		}
	}
	m, cleanup := newTestManager(t, exchanges, topupTestCfg())
	t.Cleanup(cleanup)
	return m
}

// newTestOrderbook returns a symmetric orderbook centred at midPrice.
// ask = midPrice * 1.001, bid = midPrice * 0.999 with ample depth.
func newTestOrderbook(midPrice float64) *exchange.Orderbook {
	ask := midPrice * 1.001
	bid := midPrice * 0.999
	return &exchange.Orderbook{
		Asks: []exchange.PriceLevel{{Price: ask, Quantity: 1e9}},
		Bids: []exchange.PriceLevel{{Price: bid, Quantity: 1e9}},
	}
}
