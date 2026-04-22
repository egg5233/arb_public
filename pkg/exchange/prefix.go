package exchange

import (
	"math"
	"strconv"
	"strings"
	"sync"
)

// DetectPrefixMultiplier parses a well-known power-of-10 prefix from an
// exchange base-asset name.
func DetectPrefixMultiplier(baseAsset string) (bare string, multiplier float64) {
	prefixes := []struct {
		s string
		m float64
	}{
		{"10000000", 10_000_000},
		{"1000000", 1_000_000},
		{"100000", 100_000},
		{"10000", 10_000},
		{"1000", 1_000},
	}
	for _, p := range prefixes {
		if strings.HasPrefix(baseAsset, p.s) && len(baseAsset) > len(p.s) {
			rest := baseAsset[len(p.s):]
			if rest[0] >= 'A' && rest[0] <= 'Z' {
				return rest, p.m
			}
		}
	}
	return baseAsset, 1
}

func NormalizeMultiplier(mult float64) float64 {
	if mult <= 0 {
		return 1
	}
	return mult
}

func ScaleSizeFromContracts(size, mult float64) float64 {
	return size * NormalizeMultiplier(mult)
}

func ScaleSizeToContracts(size, mult float64) float64 {
	return size / NormalizeMultiplier(mult)
}

func ScalePriceFromContracts(price, mult float64) float64 {
	return price / NormalizeMultiplier(mult)
}

func ScalePriceToContracts(price, mult float64) float64 {
	return price * NormalizeMultiplier(mult)
}

func NativeContractStep(info ContractInfo) float64 {
	mult := NormalizeMultiplier(info.Multiplier)
	if info.StepSize <= 0 {
		return 0
	}
	return info.StepSize / mult
}

func NativeContractMin(info ContractInfo) float64 {
	mult := NormalizeMultiplier(info.Multiplier)
	if info.MinSize <= 0 {
		return 0
	}
	return info.MinSize / mult
}

func FormatFloat(v float64) string {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return "0"
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// SymbolAliasCache stores bare/native symbol aliases plus multipliers for one adapter.
type SymbolAliasCache struct {
	mu            sync.RWMutex
	loadMu        sync.Mutex
	aliasMap      map[string]string
	reverseMap    map[string]string
	multiplierMap map[string]float64
	built         bool
}

func (c *SymbolAliasCache) Replace(aliasMap, reverseMap map[string]string, multiplierMap map[string]float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.aliasMap = aliasMap
	c.reverseMap = reverseMap
	c.multiplierMap = multiplierMap
	c.built = true
}

func (c *SymbolAliasCache) ResolveCached(bare string) (real string, mult float64, hit bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if !c.built {
		return bare, 1, false
	}
	if real, ok := c.aliasMap[bare]; ok {
		return real, NormalizeMultiplier(c.multiplierMap[bare]), true
	}
	return bare, 1, true
}

func (c *SymbolAliasCache) CanonicalCached(real string) (bare string, mult float64, hit bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if !c.built {
		return real, 1, false
	}
	if bare, ok := c.reverseMap[real]; ok {
		return bare, NormalizeMultiplier(c.multiplierMap[bare]), true
	}
	return real, 1, true
}

func (c *SymbolAliasCache) Ensure(load func() error) error {
	c.mu.RLock()
	built := c.built
	c.mu.RUnlock()
	if built {
		return nil
	}

	c.loadMu.Lock()
	defer c.loadMu.Unlock()

	c.mu.RLock()
	built = c.built
	c.mu.RUnlock()
	if built {
		return nil
	}
	return load()
}
