package utils

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// RoundToStep rounds a value to the nearest step size (floored to avoid overshoot).
func RoundToStep(value, step float64) float64 {
	if step <= 0 {
		return value
	}
	return math.Floor(value/step) * step
}

// FormatSize formats a size value to the given number of decimal places.
func FormatSize(size float64, decimals int) string {
	return strconv.FormatFloat(size, 'f', decimals, 64)
}

// FormatPrice formats a price value to the given number of decimal places.
func FormatPrice(price float64, decimals int) string {
	return strconv.FormatFloat(price, 'f', decimals, 64)
}

// CountDecimals returns the number of meaningful decimal places in a string number.
func CountDecimals(s string) int {
	idx := strings.IndexByte(s, '.')
	if idx < 0 {
		return 0
	}
	d := strings.TrimRight(s[idx+1:], "0")
	return len(d)
}

// ParseFloat is a convenience wrapper around strconv.ParseFloat.
func ParseFloat(s string) (float64, error) {
	return strconv.ParseFloat(s, 64)
}

// BpsToPercent converts basis points to percentage.
func BpsToPercent(bps float64) float64 {
	return bps / 100.0
}

// RateToBpsPerHour converts a funding rate (already in bps) to bps-per-hour.
// intervalHours is the funding interval in hours (e.g., 1, 4, 8).
func RateToBpsPerHour(rate float64, intervalHours float64) float64 {
	if intervalHours <= 0 {
		intervalHours = 8
	}
	return rate / intervalHours
}

// NormalizeRate8h is deprecated — use RateToBpsPerHour instead.
func NormalizeRate8h(rate float64, intervalHours float64) float64 {
	if intervalHours <= 0 {
		return rate
	}
	return rate * (8.0 / intervalHours)
}

// EstimateSlippage estimates the slippage cost in bps for a given order size
// against the order book levels.
func EstimateSlippage(levels []struct{ Price, Qty float64 }, size float64, midPrice float64) float64 {
	if len(levels) == 0 || midPrice <= 0 {
		return math.MaxFloat64
	}
	remaining := size
	weightedPrice := 0.0
	for _, l := range levels {
		fill := math.Min(remaining, l.Qty)
		weightedPrice += fill * l.Price
		remaining -= fill
		if remaining <= 0 {
			break
		}
	}
	if remaining > 0 {
		return math.MaxFloat64 // not enough liquidity
	}
	avgPrice := weightedPrice / size
	slippagePct := math.Abs(avgPrice-midPrice) / midPrice
	return slippagePct * 10000 // convert to bps
}

// GenerateID creates a simple position ID from symbol and timestamp.
func GenerateID(symbol string, ts int64) string {
	return fmt.Sprintf("%s-%d", strings.ToLower(symbol), ts)
}
