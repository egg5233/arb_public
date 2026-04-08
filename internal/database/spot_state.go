package database

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"arb/internal/models"

	"github.com/redis/go-redis/v9"
)

// Redis key constants for spot-futures positions.
const (
	keySpotPositions       = "arb:spot_positions"
	keySpotPositionsActive = "arb:spot_positions:active"
	keySpotHistory         = "arb:spot_history"
	keySpotStats           = "arb:spot_stats"
	keySpotPersistPrefix   = "arb:spot_persistence:"
	keySpotMarketPrefix    = "arb:spot_market_exists:"

	spotHistoryMaxLen = 500
	spotPersistTTL    = 20 * time.Minute
	spotMarketTTL     = 24 * time.Hour
)

// ---------------------------------------------------------------------------
// Spot Position CRUD
// ---------------------------------------------------------------------------

// SaveSpotPosition serialises the position as JSON and stores it in the
// spot positions hash. Active/pending/exiting positions are tracked in the
// active set; closed positions are removed.
func (c *Client) SaveSpotPosition(pos *models.SpotFuturesPosition) error {
	ctx := context.Background()

	data, err := json.Marshal(pos)
	if err != nil {
		return fmt.Errorf("marshal spot position: %w", err)
	}

	pipe := c.rdb.Pipeline()
	pipe.HSet(ctx, keySpotPositions, pos.ID, data)

	switch pos.Status {
	case models.SpotStatusClosed:
		pipe.SRem(ctx, keySpotPositionsActive, pos.ID)
	default:
		pipe.SAdd(ctx, keySpotPositionsActive, pos.ID)
	}

	_, err = pipe.Exec(ctx)
	return err
}

// GetSpotPosition retrieves a single spot-futures position by ID.
func (c *Client) GetSpotPosition(id string) (*models.SpotFuturesPosition, error) {
	ctx := context.Background()

	data, err := c.rdb.HGet(ctx, keySpotPositions, id).Bytes()
	if err == redis.Nil {
		return nil, fmt.Errorf("spot position %s not found", id)
	}
	if err != nil {
		return nil, err
	}

	var pos models.SpotFuturesPosition
	if err := json.Unmarshal(data, &pos); err != nil {
		return nil, fmt.Errorf("unmarshal spot position: %w", err)
	}
	return &pos, nil
}

// GetActiveSpotPositions returns all spot-futures positions whose IDs are in
// the active set.
func (c *Client) GetActiveSpotPositions() ([]*models.SpotFuturesPosition, error) {
	ctx := context.Background()

	ids, err := c.rdb.SMembers(ctx, keySpotPositionsActive).Result()
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}

	vals, err := c.rdb.HMGet(ctx, keySpotPositions, ids...).Result()
	if err != nil {
		return nil, err
	}

	positions := make([]*models.SpotFuturesPosition, 0, len(vals))
	for _, v := range vals {
		if v == nil {
			continue
		}
		s, ok := v.(string)
		if !ok {
			continue
		}
		var pos models.SpotFuturesPosition
		if err := json.Unmarshal([]byte(s), &pos); err != nil {
			continue
		}
		positions = append(positions, &pos)
	}
	return positions, nil
}

// AddToSpotHistory pushes the position to the spot history list (most recent
// first) and trims to spotHistoryMaxLen entries.
func (c *Client) AddToSpotHistory(pos *models.SpotFuturesPosition) error {
	ctx := context.Background()

	data, err := json.Marshal(pos)
	if err != nil {
		return fmt.Errorf("marshal spot position: %w", err)
	}

	pipe := c.rdb.Pipeline()
	pipe.LPush(ctx, keySpotHistory, data)
	pipe.LTrim(ctx, keySpotHistory, 0, spotHistoryMaxLen-1)
	_, err = pipe.Exec(ctx)
	return err
}

// UpdateSpotStats atomically increments trade counters and accumulates PnL
// for spot-futures positions.
func (c *Client) UpdateSpotStats(pnl float64, won bool) error {
	ctx := context.Background()

	pipe := c.rdb.Pipeline()
	pipe.HIncrByFloat(ctx, keySpotStats, "total_pnl", pnl)
	pipe.HIncrBy(ctx, keySpotStats, "trade_count", 1)
	if won {
		pipe.HIncrBy(ctx, keySpotStats, "win_count", 1)
	} else {
		pipe.HIncrBy(ctx, keySpotStats, "loss_count", 1)
	}
	_, err := pipe.Exec(ctx)
	return err
}

// GetSpotHistory returns the most recent closed spot-futures positions.
func (c *Client) GetSpotHistory(limit int) ([]*models.SpotFuturesPosition, error) {
	ctx := context.Background()

	if limit <= 0 {
		limit = 50
	}

	vals, err := c.rdb.LRange(ctx, keySpotHistory, 0, int64(limit-1)).Result()
	if err != nil {
		return nil, err
	}

	positions := make([]*models.SpotFuturesPosition, 0, len(vals))
	for _, v := range vals {
		var pos models.SpotFuturesPosition
		if err := json.Unmarshal([]byte(v), &pos); err != nil {
			continue
		}
		positions = append(positions, &pos)
	}
	return positions, nil
}

// GetSpotStats returns the spot-futures stats hash as a map.
func (c *Client) GetSpotStats() (map[string]string, error) {
	ctx := context.Background()
	return c.rdb.HGetAll(ctx, keySpotStats).Result()
}

// UpdateSpotPositionFields atomically re-reads a spot position from Redis and
// applies the given mutator function before saving. The mutator should return
// false if the update should be skipped.
func (c *Client) UpdateSpotPositionFields(id string, mutate func(pos *models.SpotFuturesPosition) bool) error {
	pos, err := c.GetSpotPosition(id)
	if err != nil {
		return err
	}

	if !mutate(pos) {
		return nil
	}

	pos.UpdatedAt = time.Now().UTC()
	return c.SaveSpotPosition(pos)
}

// SetSpotCooldown sets a per-symbol cooldown preventing re-entry for the given
// number of hours. The cooldown is stored as a Redis key with TTL.
func (c *Client) SetSpotCooldown(symbol string, hours int) error {
	if hours <= 0 {
		return nil
	}
	ctx := context.Background()
	key := "arb:spot_cooldown:" + symbol
	return c.rdb.Set(ctx, key, "1", time.Duration(hours)*time.Hour).Err()
}

// HasSpotCooldown checks whether a cooldown is active for the given symbol.
func (c *Client) HasSpotCooldown(symbol string) (bool, error) {
	ctx := context.Background()
	key := "arb:spot_cooldown:" + symbol
	n, err := c.rdb.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// ---------------------------------------------------------------------------
// Spot Persistence (consecutive-scan counters)
// ---------------------------------------------------------------------------

// IncrSpotPersistence atomically increments the consecutive-scan counter
// for a symbol and refreshes the TTL.
func (c *Client) IncrSpotPersistence(symbol string) (int64, error) {
	ctx := context.Background()
	key := keySpotPersistPrefix + symbol
	pipe := c.rdb.Pipeline()
	incr := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, spotPersistTTL)
	_, err := pipe.Exec(ctx)
	if err != nil {
		return 0, err
	}
	return incr.Val(), nil
}

// GetSpotPersistence returns the current consecutive-scan count.
func (c *Client) GetSpotPersistence(symbol string) (int, error) {
	ctx := context.Background()
	key := keySpotPersistPrefix + symbol
	val, err := c.rdb.Get(ctx, key).Int()
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return val, nil
}

// DeleteSpotPersistence removes the persistence counter (symbol disappeared).
func (c *Client) DeleteSpotPersistence(symbol string) error {
	ctx := context.Background()
	key := keySpotPersistPrefix + symbol
	return c.rdb.Del(ctx, key).Err()
}

// GetSpotMarketAvailability returns the cached spot-market existence flag for
// one exchange+symbol. The second return value reports whether a cache entry
// existed at all.
func (c *Client) GetSpotMarketAvailability(exchange, symbol string) (bool, bool, error) {
	ctx := context.Background()
	key := keySpotMarketPrefix + exchange + ":" + symbol
	val, err := c.rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return false, false, nil
	}
	if err != nil {
		return false, false, err
	}
	return val == "1", true, nil
}

// SetSpotMarketAvailability caches whether an exchange has a usable spot
// market for the given symbol. The key expires automatically so the cache can
// refresh over time while still surviving process restarts.
func (c *Client) SetSpotMarketAvailability(exchange, symbol string, exists bool) error {
	ctx := context.Background()
	key := keySpotMarketPrefix + exchange + ":" + symbol
	val := "0"
	if exists {
		val = "1"
	}
	return c.rdb.Set(ctx, key, val, spotMarketTTL).Err()
}

// ListSpotPersistenceSymbols returns all symbols that currently have a
// persistence counter in Redis. Used on startup to seed the lastSeen map
// so that counters for absent symbols are correctly cleaned up after restart.
func (c *Client) ListSpotPersistenceSymbols() ([]string, error) {
	ctx := context.Background()
	pattern := keySpotPersistPrefix + "*"
	prefixLen := len(keySpotPersistPrefix)
	var symbols []string
	var cursor uint64
	for {
		keys, next, err := c.rdb.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return nil, err
		}
		for _, k := range keys {
			if len(k) > prefixLen {
				symbols = append(symbols, k[prefixLen:])
			}
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}
	return symbols, nil
}

// ---------------------------------------------------------------------------
// Spot opportunity cache (for CA-04 dynamic shifting)
// ---------------------------------------------------------------------------

// spotOppsCacheTTL must match the TTL used by spotengine when writing "spot:opps:cache".
const spotOppsCacheTTL = 30 * time.Minute

// GetSpotEntryableOppsCount returns the number of entryable spot opportunities
// in the "spot:opps:cache" Redis key.
//
// Returns (count, nil) when the cache exists and is fresh — count may be 0
// (confirmed no opportunities) or > 0 (opportunities present).
// Returns (-1, err) when the cache is missing, expired, or stale — callers
// should treat this as "unknown" and keep conservative defaults.
//
// maxAge controls the staleness cutoff. If the key's age (30min − remaining TTL)
// exceeds maxAge, the cache is considered stale. Pass 0 to skip the age check.
func (c *Client) GetSpotEntryableOppsCount(maxAge time.Duration) (int, error) {
	ctx := context.Background()
	ttl, err := c.rdb.TTL(ctx, "spot:opps:cache").Result()
	if err != nil || ttl <= 0 {
		return -1, fmt.Errorf("spot cache missing or expired")
	}
	if maxAge > 0 {
		age := spotOppsCacheTTL - ttl
		if age > maxAge {
			return -1, fmt.Errorf("spot cache stale (age=%v > max=%v)", age, maxAge)
		}
	}
	raw, err := c.rdb.Get(ctx, "spot:opps:cache").Result()
	if err != nil {
		return -1, err
	}
	var opps []struct {
		FilterStatus string `json:"filter_status"`
	}
	if err := json.Unmarshal([]byte(raw), &opps); err != nil {
		return -1, err
	}
	count := 0
	for _, o := range opps {
		if o.FilterStatus == "" {
			count++
		}
	}
	return count, nil
}
