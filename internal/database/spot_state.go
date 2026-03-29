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

	spotHistoryMaxLen = 500
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
