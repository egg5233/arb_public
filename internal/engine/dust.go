package engine

import (
	"math"
	"strconv"
)

// dustFloor is a hard minimum below which any remainder is treated as dust,
// even when contract metadata is missing. Funding-arb USDT-perps have step
// sizes >= 1e-4, so 1e-10 is many orders of magnitude below the smallest
// tradable unit and safely catches float64 epsilon residue from -= subtraction.
const dustFloor = 1e-10

// dustThreshold returns the minimum tradable size for a symbol on an exchange.
// Aligned with depth-exit loop done-criteria (exit.go:402-417, 515-523):
// the loop stops when remaining < step OR remaining < min, so the finalizer
// must accept the same threshold.
//
// Returns (threshold, true) when contract metadata is available with a
// positive threshold; (0, false) otherwise so the caller can fall back to
// formatSize-zero detection.
func (e *Engine) dustThreshold(exchName, symbol string) (float64, bool) {
	if e.contracts != nil {
		if exContracts, ok := e.contracts[exchName]; ok {
			if ci, ok := exContracts[symbol]; ok {
				t := math.Max(ci.StepSize, ci.MinSize)
				if t > 0 {
					return t, true
				}
			}
		}
	}
	return 0, false
}

// isDust reports whether a residual size is below the tradable threshold for
// (exchName, symbol). Two-tier check:
//  1. If contract metadata is available, remainder < max(step, min)
//  2. Else fall back to the existing repo idiom: formatSize rounds to 0
//  3. Hard floor: remainder < dustFloor (1e-10) catches float epsilon when
//     both contract metadata and formatSize fallback are unavailable.
//
// Negative or zero remainders are always dust.
func (e *Engine) isDust(exchName, symbol string, remainder float64) bool {
	if remainder <= 0 {
		return true
	}
	if remainder < dustFloor {
		return true
	}
	if t, ok := e.dustThreshold(exchName, symbol); ok {
		return remainder < t
	}
	formatted := e.formatSize(exchName, symbol, remainder)
	if v, _ := strconv.ParseFloat(formatted, 64); v <= 0 {
		return true
	}
	return false
}

