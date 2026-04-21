package database

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"arb/internal/models"

	"github.com/redis/go-redis/v9"
)

// Price-Gap Tracker key namespace — Phase 8, isolated from arb:* and arb:spot_*.
// Documented per CONTEXT §D-14; "pg" prefix chosen for brevity. If a future
// subsystem (e.g. "postgres", "payment_gateway") ever needs pg: too, rename
// THIS namespace, not the others.
const (
	keyPricegapPositions      = "pg:positions"           // HSET id -> JSON
	keyPricegapActive         = "pg:positions:active"    // SET of active ids
	keyPricegapHistory        = "pg:history"             // LIST, capped 500
	keyPricegapDisabledPrefix = "pg:candidate:disabled:" // plain key, val=reason
	keyPricegapSlippagePrefix = "pg:slippage:"           // LIST per candidate, capped 10

	priceGapHistoryCap  = 500
	priceGapSlippageCap = 10

	// priceGapLockPrefix sub-prefixes under arb:locks: (see lockKey in locks.go).
	// A caller asking for "SOON" yields the final key "arb:locks:pg:SOON", which
	// cannot collide with perp-perp's "arb:locks:<symbol>" namespace.
	priceGapLockPrefix = "arb:locks:pg:"
)

// Compile-time assertion that *Client satisfies the PriceGapStore interface.
var _ models.PriceGapStore = (*Client)(nil)

// ---------------------------------------------------------------------------
// PriceGap position CRUD (D-14)
// ---------------------------------------------------------------------------

// SavePriceGapPosition serialises the position as JSON and stores it in the
// pg:positions hash. Active/pending/exiting positions are tracked in the
// pg:positions:active set; closed positions are removed.
func (c *Client) SavePriceGapPosition(p *models.PriceGapPosition) error {
	if p == nil || p.ID == "" {
		return fmt.Errorf("pricegap: nil position or empty ID")
	}

	data, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("marshal pricegap position: %w", err)
	}

	ctx := context.Background()
	pipe := c.rdb.Pipeline()
	pipe.HSet(ctx, keyPricegapPositions, p.ID, data)
	switch p.Status {
	case models.PriceGapStatusClosed:
		pipe.SRem(ctx, keyPricegapActive, p.ID)
	default:
		pipe.SAdd(ctx, keyPricegapActive, p.ID)
	}
	_, err = pipe.Exec(ctx)
	return err
}

// GetPriceGapPosition retrieves a single price-gap position by ID.
func (c *Client) GetPriceGapPosition(id string) (*models.PriceGapPosition, error) {
	ctx := context.Background()
	data, err := c.rdb.HGet(ctx, keyPricegapPositions, id).Bytes()
	if err == redis.Nil {
		return nil, fmt.Errorf("pricegap position %s not found", id)
	}
	if err != nil {
		return nil, err
	}
	var p models.PriceGapPosition
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("unmarshal pricegap position: %w", err)
	}
	return &p, nil
}

// GetActivePriceGapPositions returns all price-gap positions whose IDs are in
// the active set. Orphan entries (SET member with no HASH value) are skipped.
func (c *Client) GetActivePriceGapPositions() ([]*models.PriceGapPosition, error) {
	ctx := context.Background()
	ids, err := c.rdb.SMembers(ctx, keyPricegapActive).Result()
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}

	vals, err := c.rdb.HMGet(ctx, keyPricegapPositions, ids...).Result()
	if err != nil {
		return nil, err
	}

	out := make([]*models.PriceGapPosition, 0, len(vals))
	for _, v := range vals {
		if v == nil {
			continue
		}
		s, ok := v.(string)
		if !ok {
			continue
		}
		var p models.PriceGapPosition
		if err := json.Unmarshal([]byte(s), &p); err != nil {
			continue
		}
		out = append(out, &p)
	}
	return out, nil
}

// RemoveActivePriceGapPosition removes the ID from pg:positions:active without
// touching the hash. Used by the orphan-recovery path.
func (c *Client) RemoveActivePriceGapPosition(id string) error {
	return c.rdb.SRem(context.Background(), keyPricegapActive, id).Err()
}

// AddPriceGapHistory pushes the position to the pg:history list (most recent
// first) and trims to priceGapHistoryCap entries.
func (c *Client) AddPriceGapHistory(p *models.PriceGapPosition) error {
	if p == nil {
		return fmt.Errorf("pricegap: nil position")
	}
	data, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("marshal pricegap position: %w", err)
	}

	ctx := context.Background()
	pipe := c.rdb.Pipeline()
	pipe.LPush(ctx, keyPricegapHistory, data)
	pipe.LTrim(ctx, keyPricegapHistory, 0, priceGapHistoryCap-1)
	_, err = pipe.Exec(ctx)
	return err
}

// ---------------------------------------------------------------------------
// Candidate disable flag (D-19, D-20)
// ---------------------------------------------------------------------------

// IsCandidateDisabled returns (true, reason, nil) when the disable flag exists
// for the symbol; (false, "", nil) when the key is missing.
func (c *Client) IsCandidateDisabled(symbol string) (bool, string, error) {
	reason, err := c.rdb.Get(context.Background(), keyPricegapDisabledPrefix+symbol).Result()
	if err == redis.Nil {
		return false, "", nil
	}
	if err != nil {
		return false, "", err
	}
	return true, reason, nil
}

// SetCandidateDisabled writes the disable flag with the given human-readable
// reason. No TTL — exec-quality disable is a sticky flag cleared via /api.
func (c *Client) SetCandidateDisabled(symbol, reason string) error {
	return c.rdb.Set(context.Background(), keyPricegapDisabledPrefix+symbol, reason, 0).Err()
}

// ClearCandidateDisabled removes the disable flag for the symbol.
func (c *Client) ClearCandidateDisabled(symbol string) error {
	return c.rdb.Del(context.Background(), keyPricegapDisabledPrefix+symbol).Err()
}

// ---------------------------------------------------------------------------
// Slippage rolling window (D-19, D-21; N=10 per PG-RISK-03)
// ---------------------------------------------------------------------------

// AppendSlippageSample pushes the sample JSON to the per-candidate list and
// trims to priceGapSlippageCap entries (10).
func (c *Client) AppendSlippageSample(candidateID string, sample models.SlippageSample) error {
	data, err := json.Marshal(sample)
	if err != nil {
		return fmt.Errorf("marshal slippage sample: %w", err)
	}

	key := keyPricegapSlippagePrefix + candidateID
	ctx := context.Background()
	pipe := c.rdb.Pipeline()
	pipe.LPush(ctx, key, data)
	pipe.LTrim(ctx, key, 0, priceGapSlippageCap-1)
	_, err = pipe.Exec(ctx)
	return err
}

// GetSlippageWindow returns up to N newest-first samples for the candidate.
// If n <= 0 or n > cap, the full cap is used.
func (c *Client) GetSlippageWindow(candidateID string, n int) ([]models.SlippageSample, error) {
	if n <= 0 || n > priceGapSlippageCap {
		n = priceGapSlippageCap
	}
	raws, err := c.rdb.LRange(context.Background(), keyPricegapSlippagePrefix+candidateID, 0, int64(n-1)).Result()
	if err != nil {
		return nil, err
	}
	out := make([]models.SlippageSample, 0, len(raws))
	for _, r := range raws {
		var s models.SlippageSample
		if err := json.Unmarshal([]byte(r), &s); err != nil {
			continue
		}
		out = append(out, s)
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Distributed locks — pg: sub-prefix under arb:locks:
// ---------------------------------------------------------------------------

// AcquirePriceGapLock acquires a SET-NX distributed lock with a per-caller
// token. The final Redis key is "arb:locks:pg:<resource>" so it cannot collide
// with the perp-perp "arb:locks:<symbol>" namespace (T-08-08).
//
// Returns (token, true, nil) on success. Returns ("", false, nil) when the
// lock is already held. Release must present the same token.
func (c *Client) AcquirePriceGapLock(resource string, ttl time.Duration) (string, bool, error) {
	token, err := newLockToken()
	if err != nil {
		return "", false, err
	}

	ctx := context.Background()
	ok, err := c.rdb.SetNX(ctx, priceGapLockPrefix+resource, token, ttl).Result()
	if err != nil {
		return "", false, fmt.Errorf("acquire pricegap lock %s: %w", resource, err)
	}
	if !ok {
		return "", false, nil
	}
	return token, true, nil
}

// ReleasePriceGapLock performs compare-and-delete: only removes the key if
// the stored token matches. Reuses the existing releaseLockScript from locks.go.
func (c *Client) ReleasePriceGapLock(resource, token string) error {
	ctx := context.Background()
	_, err := releaseLockScript.Run(ctx, c.rdb, []string{priceGapLockPrefix + resource}, token).Result()
	if err != nil && err != redis.Nil {
		return fmt.Errorf("release pricegap lock %s: %w", resource, err)
	}
	return nil
}
