package spotengine

import (
	"sync"
	"time"
)

// maintenanceRateProvider is an optional interface for exchange adapters that
// can query tiered maintenance margin rates for a symbol at a given notional.
// BingX does not implement this interface. Use type assertion to check.
type maintenanceRateProvider interface {
	GetMaintenanceRate(symbol string, notionalUSDT float64) (float64, error)
}

// ---------------------------------------------------------------------------
// maintenanceRateCache holds cached per-symbol maintenance rates with TTL.
// ---------------------------------------------------------------------------

type maintenanceRateCache struct {
	mu      sync.RWMutex
	entries map[string]maintenanceCacheEntry
	ttl     time.Duration
}

type maintenanceCacheEntry struct {
	rate      float64
	fetchedAt time.Time
}

func newMaintenanceRateCache(ttl time.Duration) *maintenanceRateCache {
	return &maintenanceRateCache{
		entries: make(map[string]maintenanceCacheEntry),
		ttl:     ttl,
	}
}

// get returns the cached rate and true if present and not expired.
func (c *maintenanceRateCache) get(key string) (float64, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.entries[key]
	if !ok {
		return 0, false
	}
	if time.Since(entry.fetchedAt) > c.ttl {
		return 0, false
	}
	return entry.rate, true
}

// set stores a rate in the cache.
func (c *maintenanceRateCache) set(key string, rate float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = maintenanceCacheEntry{
		rate:      rate,
		fetchedAt: time.Now(),
	}
}

// ---------------------------------------------------------------------------
// SpotEngine.getMaintenanceRate — cached, adapter-aware, with safe fallback.
// ---------------------------------------------------------------------------

// getMaintenanceRate returns the maintenance margin rate for a symbol on a
// given exchange at a given notional size. It uses an in-memory cache to avoid
// redundant API calls.
//
// Lookup order:
//  1. Cache hit (key = exchName:symbol:notional_bucket)
//  2. Adapter implements maintenanceRateProvider -> call GetMaintenanceRate
//  3. Fall back to ContractInfo.MaintenanceRate from LoadAllContracts
//  4. If result is 0 (unknown), return cfg.SpotFuturesMaintenanceDefault
//
// Per threat model T-06-03: maintenanceRate=0 is never silently passed.
func (e *SpotEngine) getMaintenanceRate(symbol, exchName string, notionalUSDT float64) float64 {
	// Initialize cache on first use (lazy init, safe for single-goroutine access
	// from discoveryLoop/monitorLoop; cache itself is mutex-protected).
	if e.maintCache == nil {
		ttlMin := e.cfg.SpotFuturesMaintenanceCacheTTL
		if ttlMin <= 0 {
			ttlMin = 60
		}
		e.maintCache = newMaintenanceRateCache(time.Duration(ttlMin) * time.Minute)
	}

	// Round notional to nearest 1000 for cache efficiency (rates don't change
	// for small notional differences within the same tier).
	bucket := int64(notionalUSDT/1000) * 1000
	cacheKey := exchName + ":" + symbol + ":" + itoa(bucket)

	// 1. Cache hit
	if rate, ok := e.maintCache.get(cacheKey); ok {
		return e.applyMaintenanceDefault(rate, symbol, exchName)
	}

	// 2. Try adapter-specific tiered lookup
	exch, ok := e.exchanges[exchName]
	if ok {
		if provider, ok := exch.(maintenanceRateProvider); ok {
			rate, err := provider.GetMaintenanceRate(symbol, notionalUSDT)
			if err != nil {
				e.log.Warn("maintenance: %s %s GetMaintenanceRate: %v", exchName, symbol, err)
			} else if rate > 0 && rate < 1.0 {
				e.maintCache.set(cacheKey, rate)
				return rate
			}
		}
	}

	// 3. Fall back to ContractInfo.MaintenanceRate
	if ok {
		contracts, err := exch.LoadAllContracts()
		if err == nil {
			if ci, ok := contracts[symbol]; ok && ci.MaintenanceRate > 0 && ci.MaintenanceRate < 1.0 {
				e.maintCache.set(cacheKey, ci.MaintenanceRate)
				return ci.MaintenanceRate
			}
		}
	}

	// 4. Cache miss + no adapter data -> return default
	return e.applyMaintenanceDefault(0, symbol, exchName)
}

// applyMaintenanceDefault returns the rate if valid, otherwise the conservative
// default from config. Logs a warning when falling back.
func (e *SpotEngine) applyMaintenanceDefault(rate float64, symbol, exchName string) float64 {
	if rate > 0 && rate < 1.0 {
		return rate
	}
	def := e.cfg.SpotFuturesMaintenanceDefault
	if def <= 0 || def >= 1.0 {
		def = 0.05 // hard fallback
	}
	e.log.Warn("maintenance: using default %.1f%% for %s on %s (rate=0/unavailable)", def*100, symbol, exchName)
	return def
}

// itoa converts int64 to string without importing strconv (avoids import for
// a single use in cache key building).
func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	pos := len(buf)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}
