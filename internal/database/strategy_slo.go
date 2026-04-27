package database

import (
	"context"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"

	"arb/internal/models"
	"arb/internal/strategy"

	"github.com/redis/go-redis/v9"
)

const (
	keyDirBSLOBreachCount        = "arb:dirb:slo_breach_count"
	keyDirBPendingConflictPrefix = "arb:dirb:pending_conflict:"
	keyDirBSLOFinalizedPrefix    = "arb:dirb:slo_finalized:"
	keyStrategyCompletePrefix    = "arb:strategy:reservation_complete:"
)

func (c *Client) RecordReservationCompletion(rec strategy.ReservationCompletionRecord) error {
	ctx := context.Background()
	key := keyStrategyCompletePrefix + rec.ReservationID
	fields := map[string]interface{}{
		"reservation_id": rec.ReservationID,
		"outcome":        string(rec.Outcome),
		"epoch":          strconv.FormatUint(rec.Epoch, 10),
		"strategy":       string(rec.Strategy),
		"candidate_id":   rec.CandidateID,
		"keys":           strings.Join(strategy.LegKeyStrings(rec.Keys), ","),
		"completed_at":   rec.CompletedAt.Format(time.RFC3339Nano),
	}
	pipe := c.rdb.Pipeline()
	pipe.HSet(ctx, key, fields)
	pipe.Expire(ctx, key, 25*time.Minute)
	_, err := pipe.Exec(ctx)
	return err
}

func (c *Client) RecordDirBPendingConflict(rec strategy.PendingDirBConflict) error {
	if rec.DirBCandidateID == "" || rec.ConflictReservationID == "" || len(rec.OverlappingKeys) == 0 {
		return nil
	}
	ctx := context.Background()
	key := dirBPendingConflictKey(rec.Epoch, rec.ConflictReservationID, rec.DirBCandidateID)
	ttl := time.Until(rec.ExpiresAt)
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	fields := map[string]interface{}{
		"dirb_candidate_id":       rec.DirBCandidateID,
		"conflict_reservation_id": rec.ConflictReservationID,
		"epoch":                   strconv.FormatUint(rec.Epoch, 10),
		"conflict_strategy":       string(rec.ConflictStrategy),
		"conflict_candidate_id":   rec.ConflictCandidateID,
		"overlapping_keys":        strings.Join(strategy.LegKeyStrings(rec.OverlappingKeys), ","),
		"dirb_ev_bps_h":           strconv.FormatFloat(rec.DirBEVBpsH, 'f', -1, 64),
		"conflict_ev_bps_h":       strconv.FormatFloat(rec.ConflictEVBpsH, 'f', -1, 64),
		"created_at":              rec.CreatedAt.Format(time.RFC3339Nano),
		"expires_at":              rec.ExpiresAt.Format(time.RFC3339Nano),
		"finalized":               "false",
	}
	pipe := c.rdb.Pipeline()
	pipe.HSet(ctx, key, fields)
	pipe.Expire(ctx, key, ttl)
	_, err := pipe.Exec(ctx)
	return err
}

func (c *Client) FinalizeDirBConflicts(rec strategy.ReservationCompletionRecord) error {
	if rec.ReservationID == "" || rec.Strategy != strategy.StrategyPP {
		return nil
	}
	ctx := context.Background()
	pattern := fmt.Sprintf("%s%d:%s:*", keyDirBPendingConflictPrefix, rec.Epoch, rec.ReservationID)
	return c.scanPendingConflicts(ctx, pattern, func(key string, fields map[string]string) error {
		return c.finalizePendingConflict(ctx, key, rec.Outcome == strategy.ReservationOutcomeActive)
	})
}

func (c *Client) ReplayDirBSLOFromPositions() error {
	ctx := context.Background()
	active, err := c.GetActivePositions()
	if err != nil {
		return err
	}
	byReservation := make(map[string]*models.ArbitragePosition, len(active))
	for _, pos := range active {
		if pos != nil && pos.Strategy == string(strategy.StrategyPP) && pos.StrategyReservationID != "" {
			byReservation[pos.StrategyReservationID] = pos
		}
	}
	return c.scanPendingConflicts(ctx, keyDirBPendingConflictPrefix+"*", func(key string, fields map[string]string) error {
		conflictID := fields["conflict_reservation_id"]
		epoch, _ := strconv.ParseUint(fields["epoch"], 10, 64)
		if comp, err := c.rdb.HGetAll(ctx, keyStrategyCompletePrefix+conflictID).Result(); err == nil && len(comp) > 0 {
			outcome := comp["outcome"] == string(strategy.ReservationOutcomeActive)
			return c.finalizePendingConflict(ctx, key, outcome)
		}
		pos := byReservation[conflictID]
		if pos == nil || pos.StrategyEpoch != epoch {
			return nil
		}
		if fields["conflict_candidate_id"] != "" && fields["conflict_candidate_id"] != pos.StrategyCandidateID {
			return nil
		}
		if !keysOverlapCSV(fields["overlapping_keys"], pos.StrategyLegKeys) {
			return nil
		}
		return c.finalizePendingConflict(ctx, key, true)
	})
}

func (c *Client) GetDirBSLOStats() (map[string]interface{}, error) {
	ctx := context.Background()
	count, err := c.rdb.Get(ctx, keyDirBSLOBreachCount).Int64()
	if err == redis.Nil {
		count = 0
		err = nil
	}
	if err != nil {
		return nil, err
	}
	var pending int64
	if err := c.scanPendingConflicts(ctx, keyDirBPendingConflictPrefix+"*", func(_ string, _ map[string]string) error {
		pending++
		return nil
	}); err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"dirb_slo_breach_count": count,
		"pending_conflicts":     pending,
	}, nil
}

func (c *Client) scanPendingConflicts(ctx context.Context, pattern string, fn func(key string, fields map[string]string) error) error {
	var cursor uint64
	for {
		keys, next, err := c.rdb.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return err
		}
		for _, key := range keys {
			fields, err := c.rdb.HGetAll(ctx, key).Result()
			if err != nil {
				return err
			}
			if len(fields) == 0 {
				continue
			}
			if err := fn(key, fields); err != nil {
				return err
			}
		}
		cursor = next
		if cursor == 0 {
			return nil
		}
	}
}

func (c *Client) finalizePendingConflict(ctx context.Context, pendingKey string, increment bool) error {
	script := redis.NewScript(`
local guard = KEYS[1]
local pending = KEYS[2]
local counter = KEYS[3]
local ttl = tonumber(ARGV[1])
local increment = ARGV[2]
if redis.call("EXISTS", guard) == 1 then
  return 0
end
if redis.call("EXISTS", pending) == 0 then
  return 0
end
redis.call("SET", guard, "1", "EX", ttl)
if increment == "1" then
  redis.call("INCR", counter)
end
redis.call("DEL", pending)
return 1
`)
	guard := keyDirBSLOFinalizedPrefix + encodeKey(pendingKey)
	inc := "0"
	if increment {
		inc = "1"
	}
	return script.Run(ctx, c.rdb, []string{guard, pendingKey, keyDirBSLOBreachCount}, int((25*time.Minute)/time.Second), inc).Err()
}

func dirBPendingConflictKey(epoch uint64, reservationID, candidateID string) string {
	return fmt.Sprintf("%s%d:%s:%s", keyDirBPendingConflictPrefix, epoch, reservationID, encodeKey(candidateID))
}

func encodeKey(s string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(s))
}

func keysOverlapCSV(csv string, keys []string) bool {
	if csv == "" || len(keys) == 0 {
		return false
	}
	seen := make(map[string]bool, len(keys))
	for _, key := range keys {
		seen[key] = true
	}
	for _, key := range strings.Split(csv, ",") {
		if seen[key] {
			return true
		}
	}
	return false
}
