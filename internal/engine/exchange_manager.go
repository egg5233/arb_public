package engine

import (
	"fmt"
	"sync"

	"arb/internal/config"
	"arb/pkg/exchange"
	"arb/pkg/exchange/binance"
	"arb/pkg/exchange/bingx"
	"arb/pkg/exchange/bitget"
	"arb/pkg/exchange/bybit"
	"arb/pkg/exchange/gateio"
	"arb/pkg/exchange/okx"
	"arb/pkg/utils"
)

// adapterSnapshot stores the config used to create an adapter so we can
// detect changes on reload.
type adapterSnapshot struct {
	apiKey, secretKey, passphrase string
	enabled                       bool
}

// ExchangeManager manages exchange adapters and supports hot-reloading
// when config changes (API keys, enabled/disabled state).
type ExchangeManager struct {
	mu        sync.RWMutex
	exchanges map[string]exchange.Exchange
	snapshots map[string]adapterSnapshot
	cfg       *config.Config
	log       *utils.Logger
}

// NewExchangeManager creates an ExchangeManager. Call SetExchanges to provide
// the initial adapter map built at startup.
func NewExchangeManager(cfg *config.Config) *ExchangeManager {
	return &ExchangeManager{
		cfg:       cfg,
		log:       utils.NewLogger("exch-mgr"),
		snapshots: make(map[string]adapterSnapshot),
	}
}

// SetExchanges stores the initial exchange map and records a config
// snapshot for each adapter so that Reload can detect changes later.
func (m *ExchangeManager) SetExchanges(exchanges map[string]exchange.Exchange) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.exchanges = exchanges
	// Record the initial config snapshot for each exchange.
	m.cfg.RLock()
	defer m.cfg.RUnlock()
	for name := range exchanges {
		m.snapshots[name] = m.currentSnapshot(name)
	}
}

// Get returns a shallow copy of the current exchange map so callers always
// see a consistent set even if a reload is in progress.
func (m *ExchangeManager) Get() map[string]exchange.Exchange {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cp := make(map[string]exchange.Exchange, len(m.exchanges))
	for k, v := range m.exchanges {
		cp[k] = v
	}
	return cp
}

// GetExchange returns a single exchange adapter by name.
func (m *ExchangeManager) GetExchange(name string) (exchange.Exchange, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	exc, ok := m.exchanges[name]
	return exc, ok
}

// allExchangeNames is the canonical order of exchanges.
var allExchangeNames = []string{"binance", "bybit", "gateio", "bitget", "okx", "bingx"}

// Reload compares the current config against the stored snapshots and
// rebuilds adapters whose credentials changed, adds newly-enabled
// exchanges, and removes disabled ones.
func (m *ExchangeManager) Reload() {
	m.cfg.RLock()
	enabledSet := make(map[string]bool, len(allExchangeNames))
	for _, name := range allExchangeNames {
		enabledSet[name] = m.cfg.IsExchangeEnabled(name)
	}
	// Take a snapshot of all exchange configs while holding the read lock.
	newSnaps := make(map[string]adapterSnapshot, len(allExchangeNames))
	for _, name := range allExchangeNames {
		newSnaps[name] = m.currentSnapshot(name)
	}
	m.cfg.RUnlock()

	m.mu.Lock()
	defer m.mu.Unlock()

	// 1. Remove disabled exchanges.
	for name, exc := range m.exchanges {
		if !enabledSet[name] {
			m.log.Info("Exchange %s disabled — closing adapter", name)
			exc.Close()
			delete(m.exchanges, name)
			delete(m.snapshots, name)
		}
	}

	// 2. For each enabled exchange, check if credentials changed.
	for _, name := range allExchangeNames {
		if !enabledSet[name] {
			continue
		}
		newSnap := newSnaps[name]
		oldSnap, existed := m.snapshots[name]

		needRebuild := false
		if !existed {
			// Newly enabled exchange.
			needRebuild = true
			m.log.Info("Exchange %s newly enabled — creating adapter", name)
		} else if newSnap != oldSnap {
			needRebuild = true
			m.log.Info("Exchange %s credentials changed — rebuilding adapter", name)
		}

		if !needRebuild {
			continue
		}

		// Close old adapter if it exists.
		if old, ok := m.exchanges[name]; ok {
			old.Close()
		}

		excCfg := buildExchangeConfig(name, newSnap)
		adapter, err := buildAdapter(name, excCfg)
		if err != nil {
			m.log.Error("Failed to rebuild %s: %v", name, err)
			delete(m.exchanges, name)
			delete(m.snapshots, name)
			continue
		}

		// Start websocket streams on the new adapter.
		adapter.StartPriceStream(nil)
		adapter.StartPrivateStream()

		m.exchanges[name] = adapter
		m.snapshots[name] = newSnap
		m.log.Info("Exchange %s adapter reloaded", name)
	}
}

// currentSnapshot reads the current config fields for a given exchange.
// MUST be called while cfg.RLock is held.
func (m *ExchangeManager) currentSnapshot(name string) adapterSnapshot {
	switch name {
	case "binance":
		return adapterSnapshot{
			apiKey: m.cfg.BinanceAPIKey, secretKey: m.cfg.BinanceSecretKey,
			enabled: m.cfg.IsExchangeEnabled("binance"),
		}
	case "bybit":
		return adapterSnapshot{
			apiKey: m.cfg.BybitAPIKey, secretKey: m.cfg.BybitSecretKey,
			enabled: m.cfg.IsExchangeEnabled("bybit"),
		}
	case "gateio":
		return adapterSnapshot{
			apiKey: m.cfg.GateioAPIKey, secretKey: m.cfg.GateioSecretKey,
			enabled: m.cfg.IsExchangeEnabled("gateio"),
		}
	case "bitget":
		return adapterSnapshot{
			apiKey: m.cfg.BitgetAPIKey, secretKey: m.cfg.BitgetSecretKey,
			passphrase: m.cfg.BitgetPassphrase,
			enabled:    m.cfg.IsExchangeEnabled("bitget"),
		}
	case "okx":
		return adapterSnapshot{
			apiKey: m.cfg.OKXAPIKey, secretKey: m.cfg.OKXSecretKey,
			passphrase: m.cfg.OKXPassphrase,
			enabled:    m.cfg.IsExchangeEnabled("okx"),
		}
	case "bingx":
		return adapterSnapshot{
			apiKey: m.cfg.BingXAPIKey, secretKey: m.cfg.BingXSecretKey,
			enabled: m.cfg.IsExchangeEnabled("bingx"),
		}
	}
	return adapterSnapshot{}
}

// buildExchangeConfig creates an ExchangeConfig from a snapshot.
func buildExchangeConfig(name string, snap adapterSnapshot) exchange.ExchangeConfig {
	return exchange.ExchangeConfig{
		Exchange:   name,
		ApiKey:     snap.apiKey,
		SecretKey:  snap.secretKey,
		Passphrase: snap.passphrase,
	}
}

// buildAdapter creates an exchange.Exchange from a name and config.
func buildAdapter(name string, cfg exchange.ExchangeConfig) (exchange.Exchange, error) {
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
