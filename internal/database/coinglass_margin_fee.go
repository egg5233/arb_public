package database

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// Redis key format for CoinGlass margin fee history, matching what
// /var/solana/data/coinGlass/fetch_margin_fee.js writes hourly:
//
//	coinGlassMarginFee:hist:{exchange}:{coin}
//
// Each list entry is JSON: {"t": <unix_seconds>, "rate": <hourly_rate_decimal>, "limit": <string>}.
// LPUSH + LTRIM 0 719 keeps the most recent 720 hourly samples (~30 days).
const coinGlassMarginFeeHistPrefix = "coinGlassMarginFee:hist:"

// CoinGlassMarginFeePoint is one hourly sample from the CoinGlass /MarginFee scraper.
type CoinGlassMarginFeePoint struct {
	Timestamp  time.Time
	HourlyRate float64 // decimal, e.g. 0.000342% → 3.42e-6
}

// GetCoinGlassMarginFeeHistory reads the rolling CoinGlass borrow-rate history for
// (exchange, coin) and returns the points whose timestamp falls in [start, end].
// Used by the spot-futures Dir A backtest as a fallback for exchanges that don't
// expose a public historical borrow-rate API (OKX, Bitget).
//
// `exchange` is lower-case (e.g. "okx"). `coin` is upper-case base asset (e.g. "BTC").
// Returns an empty slice (nil error) when the key is missing or empty — the caller
// decides whether to treat that as unsupported or fail-open.
func (c *Client) GetCoinGlassMarginFeeHistory(exchange, coin string, start, end time.Time) ([]CoinGlassMarginFeePoint, error) {
	ctx := context.Background()
	key := coinGlassMarginFeeHistPrefix + exchange + ":" + coin

	raw, err := c.rdb.LRange(ctx, key, 0, -1).Result()
	if err != nil {
		return nil, fmt.Errorf("lrange %s: %w", key, err)
	}
	if len(raw) == 0 {
		return nil, nil
	}

	var entry struct {
		T    int64   `json:"t"`
		Rate float64 `json:"rate"`
	}
	out := make([]CoinGlassMarginFeePoint, 0, len(raw))
	for _, s := range raw {
		if err := json.Unmarshal([]byte(s), &entry); err != nil {
			// Skip malformed entries — scraper drift shouldn't kill a backtest.
			continue
		}
		t := time.Unix(entry.T, 0).UTC()
		if t.Before(start) || t.After(end) {
			continue
		}
		out = append(out, CoinGlassMarginFeePoint{
			Timestamp:  t,
			HourlyRate: entry.Rate,
		})
	}
	return out, nil
}
