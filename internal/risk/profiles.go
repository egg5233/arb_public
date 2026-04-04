package risk

import (
	"fmt"

	"arb/internal/config"
)

// ProfilePreset defines the bundled configuration values for a named risk profile.
type ProfilePreset struct {
	MaxPositions      int
	Leverage          int
	MaxCostRatio      float64 // perp-perp entry threshold
	MinNetYieldAPR    float64 // spot-futures entry threshold
	PerpPerpPct       float64 // base allocation to perp-perp
	SpotFuturesPct    float64 // base allocation to spot-futures
	SizeMultiplier    float64 // multiplier on derived CapitalPerLeg
}

// Profiles maps named risk profiles to their preset parameter bundles.
var Profiles = map[string]ProfilePreset{
	"conservative": {
		MaxPositions:   1,
		Leverage:       2,
		MaxCostRatio:   0.30,
		MinNetYieldAPR: 0.15,
		PerpPerpPct:    0.60,
		SpotFuturesPct: 0.40,
		SizeMultiplier: 0.7,
	},
	"balanced": {
		MaxPositions:   3,
		Leverage:       3,
		MaxCostRatio:   0.50,
		MinNetYieldAPR: 0.10,
		PerpPerpPct:    0.50,
		SpotFuturesPct: 0.50,
		SizeMultiplier: 1.0,
	},
	"aggressive": {
		MaxPositions:   5,
		Leverage:       5,
		MaxCostRatio:   0.70,
		MinNetYieldAPR: 0.05,
		PerpPerpPct:    0.40,
		SpotFuturesPct: 0.60,
		SizeMultiplier: 1.3,
	},
}

// ApplyProfile looks up the named profile and overwrites all bundled config
// fields with the preset values. Returns an error for unknown profile names
// without modifying the config.
func ApplyProfile(cfg *config.Config, profileName string) error {
	preset, ok := Profiles[profileName]
	if !ok {
		return fmt.Errorf("unknown risk profile: %q", profileName)
	}

	cfg.MaxPositions = preset.MaxPositions
	cfg.Leverage = preset.Leverage
	cfg.MaxCostRatio = preset.MaxCostRatio
	cfg.SpotFuturesMinNetYieldAPR = preset.MinNetYieldAPR
	cfg.MaxPerpPerpPct = preset.PerpPerpPct
	cfg.MaxSpotFuturesPct = preset.SpotFuturesPct
	cfg.SizeMultiplier = preset.SizeMultiplier
	cfg.RiskProfile = profileName
	return nil
}

// ProfileBundledFields returns the JSON config paths that are controlled by
// risk profiles. Used by handlePostConfig to detect manual override and
// automatically set RiskProfile to "custom".
func ProfileBundledFields() []string {
	return []string{
		"fund.max_positions",
		"fund.leverage",
		"strategy.discovery.max_cost_ratio",
		"spot_futures.min_net_yield_apr",
		"risk.max_perp_perp_pct",
		"risk.max_spot_futures_pct",
		"allocation.size_multiplier",
	}
}
