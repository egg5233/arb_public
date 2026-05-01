package database

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
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

	// Phase 14 (PG-LIVE-01, PG-LIVE-03) — daily reconcile + ramp persistence.
	// All keys live under the existing pg:* namespace to keep operator mental
	// model consistent. Per-day SET enables O(1) reconcile lookup (D-02);
	// reconcile daily aggregate is byte-identical on repeat (D-04 idempotency).
	keyPricegapClosedPrefix       = "pg:positions:closed:" // SET per-day, key = prefix + YYYY-MM-DD (D-02)
	keyPricegapReconcileDailyPref = "pg:reconcile:daily:"  // STRING per-day (D-04 byte-identical)
	keyPricegapRampState          = "pg:ramp:state"        // HASH 5 fields (PG-LIVE-01 hard contract)
	keyPricegapRampEvents         = "pg:ramp:events"       // LIST capped 500

	priceGapRampEventsCap = 500

	// Phase 15 (PG-LIVE-02) — Drawdown circuit breaker persistence. HASH for
	// the 5-field BreakerState (D-05); capped LIST for the trip log (D-18).
	keyPriceGapBreakerState = "pg:breaker:state"
	keyPriceGapBreakerTrips = "pg:breaker:trips"

	priceGapBreakerTripsCap = 500
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

// LoadPriceGapPosition is the (pos, exists, error) variant required by
// pricegaptrader.ReconcileStore (Phase 14 Plan 14-02). It distinguishes
// missing-id (exists=false, err=nil) from real Redis errors so the daemon's
// per-position error path can count skipped positions without confusing
// "not found" with transport failures (T-14-11).
func (c *Client) LoadPriceGapPosition(id string) (*models.PriceGapPosition, bool, error) {
	ctx := context.Background()
	data, err := c.rdb.HGet(ctx, keyPricegapPositions, id).Bytes()
	if err == redis.Nil {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	var p models.PriceGapPosition
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, false, fmt.Errorf("unmarshal pricegap position: %w", err)
	}
	return &p, true, nil
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

// GetPriceGapHistory returns up to `limit` closed positions from the pg:history
// LIST, newest first, skipping the first `offset` entries. Returns (nil, nil)
// for limit <= 0. Records missing a Mode field (pre-Phase-9) are normalized to
// "live" via models.NormalizeMode on decode (Phase 9 D-12).
func (c *Client) GetPriceGapHistory(offset, limit int) ([]*models.PriceGapPosition, error) {
	if limit <= 0 {
		return nil, nil
	}
	if offset < 0 {
		offset = 0
	}
	ctx := context.Background()
	raws, err := c.rdb.LRange(ctx, keyPricegapHistory, int64(offset), int64(offset+limit-1)).Result()
	if err != nil {
		return nil, err
	}
	out := make([]*models.PriceGapPosition, 0, len(raws))
	for _, r := range raws {
		var p models.PriceGapPosition
		if err := json.Unmarshal([]byte(r), &p); err != nil {
			continue
		}
		models.NormalizeMode(&p)
		out = append(out, &p)
	}
	return out, nil
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

// disabledPayload is the JSON shape written by SetCandidateDisabled since
// Phase 9 (Pitfall 6). Legacy pre-Phase-9 values were plain strings — see
// the json.Unmarshal fallback in IsCandidateDisabled for backward compat.
type disabledPayload struct {
	Reason     string `json:"reason"`
	DisabledAt int64  `json:"disabled_at"`
}

// IsCandidateDisabled returns (true, reason, disabledAt, nil) when the disable
// flag exists for the symbol; (false, "", 0, nil) when the key is missing.
//
// The on-disk value is expected to be a JSON blob {reason, disabled_at} since
// Phase 9. To preserve compatibility with legacy plain-string values written by
// pre-Phase-9 pg-admin, a failed json.Unmarshal falls back to treating the raw
// string as reason with disabledAt=0.
func (c *Client) IsCandidateDisabled(symbol string) (bool, string, int64, error) {
	raw, err := c.rdb.Get(context.Background(), keyPricegapDisabledPrefix+symbol).Result()
	if err == redis.Nil {
		return false, "", 0, nil
	}
	if err != nil {
		return false, "", 0, err
	}
	var p disabledPayload
	if jerr := json.Unmarshal([]byte(raw), &p); jerr == nil && (p.Reason != "" || p.DisabledAt != 0) {
		return true, p.Reason, p.DisabledAt, nil
	}
	// Legacy plain-string value — treat as reason with no timestamp.
	return true, raw, 0, nil
}

// SetCandidateDisabled writes the disable flag as a JSON blob
// {reason, disabled_at: now-unix-seconds}. Single-writer invariant:
// both dashboard /api/pricegap/candidate/.../disable and pg-admin CLI
// call this method (Pitfall 6). No TTL — exec-quality disable is sticky
// and cleared only via /api/pricegap/candidate/.../enable or pg-admin.
func (c *Client) SetCandidateDisabled(symbol, reason string) error {
	payload := disabledPayload{
		Reason:     reason,
		DisabledAt: time.Now().Unix(),
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal disabled payload: %w", err)
	}
	return c.rdb.Set(context.Background(), keyPricegapDisabledPrefix+symbol, data, 0).Err()
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

// ---------------------------------------------------------------------------
// Phase 14 — daily reconcile + ramp persistence (PG-LIVE-01, PG-LIVE-03)
// ---------------------------------------------------------------------------

// pricegapDateString formats date in UTC YYYY-MM-DD for use in key suffixes.
// Centralized so every reconcile path uses the same format (D-04 keying).
func pricegapDateString(d time.Time) string {
	return d.UTC().Format("2006-01-02")
}

// AddPriceGapClosedPositionForDate SADDs the position id into a per-day SET
// keyed by the UTC date of `date` (D-02). No TTL — operator backfill must
// work for any past date. Caller is the close-path hook in monitor.closePair;
// the SADD is best-effort (matches the existing precedent at lines 248–249
// of monitor.go where SavePriceGapPosition / RemoveActivePriceGapPosition
// failures are logged but not raised — T-14-09).
func (c *Client) AddPriceGapClosedPositionForDate(posID string, date time.Time) error {
	if posID == "" {
		return fmt.Errorf("pricegap: AddPriceGapClosedPositionForDate empty posID")
	}
	key := keyPricegapClosedPrefix + pricegapDateString(date)
	return c.rdb.SAdd(context.Background(), key, posID).Err()
}

// GetPriceGapClosedPositionsForDate returns the position IDs SADDed for the
// given UTC date string (YYYY-MM-DD). Returns nil slice on empty SET; non-nil
// only when at least one ID exists.
func (c *Client) GetPriceGapClosedPositionsForDate(date string) ([]string, error) {
	key := keyPricegapClosedPrefix + date
	ids, err := c.rdb.SMembers(context.Background(), key).Result()
	if err != nil {
		return nil, fmt.Errorf("smembers pg:positions:closed:%s: %w", date, err)
	}
	if len(ids) == 0 {
		return nil, nil
	}
	return ids, nil
}

// SavePriceGapReconcileDaily SETs the byte-identical aggregate JSON for the
// given UTC date. Plain SET (overwrite) — D-04 byte-identical re-runs are
// the reconciler's responsibility, not Redis's.
func (c *Client) SavePriceGapReconcileDaily(date string, payload []byte) error {
	if date == "" {
		return fmt.Errorf("pricegap: SavePriceGapReconcileDaily empty date")
	}
	key := keyPricegapReconcileDailyPref + date
	return c.rdb.Set(context.Background(), key, payload, 0).Err()
}

// LoadPriceGapReconcileDaily returns (payload, exists, err). exists=false on
// redis.Nil (no value yet for this date). Callers use exists=false to decide
// whether to run the reconcile aggregation for the first time.
func (c *Client) LoadPriceGapReconcileDaily(date string) ([]byte, bool, error) {
	key := keyPricegapReconcileDailyPref + date
	data, err := c.rdb.Get(context.Background(), key).Bytes()
	if err == redis.Nil {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("get pg:reconcile:daily:%s: %w", date, err)
	}
	return data, true, nil
}

// SavePriceGapRampState writes the 5-field RampState atomically to a HASH at
// keyPricegapRampState. Numeric fields are stored as decimal strings;
// timestamps as RFC3339Nano so reconciliation is sub-second precise. The
// HSet pipeline guarantees all-or-nothing semantics (consumer reading mid-
// write sees either the prior state or the new state).
func (c *Client) SavePriceGapRampState(s models.RampState) error {
	ctx := context.Background()
	pipe := c.rdb.Pipeline()
	pipe.HSet(ctx, keyPricegapRampState, map[string]any{
		"current_stage":     s.CurrentStage,
		"clean_day_counter": s.CleanDayCounter,
		"last_eval_ts":      s.LastEvalTs.UTC().Format(time.RFC3339Nano),
		"last_loss_day_ts":  s.LastLossDayTs.UTC().Format(time.RFC3339Nano),
		"demote_count":      s.DemoteCount,
	})
	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("hset pg:ramp:state: %w", err)
	}
	return nil
}

// LoadPriceGapRampState returns (state, exists, err). On a fresh database
// (HASH absent), exists=false and state is the zero value — Phase 14 ramp
// controller treats the zero value as "stage 1, no clean days". On a populated
// HASH, all 5 fields are decoded; an unparseable field returns an error so
// the controller can fail-closed (Plan 14-03 Gate 6).
func (c *Client) LoadPriceGapRampState() (models.RampState, bool, error) {
	ctx := context.Background()
	m, err := c.rdb.HGetAll(ctx, keyPricegapRampState).Result()
	if err != nil {
		return models.RampState{}, false, fmt.Errorf("hgetall pg:ramp:state: %w", err)
	}
	if len(m) == 0 {
		return models.RampState{}, false, nil
	}
	var s models.RampState
	// CurrentStage
	if v, ok := m["current_stage"]; ok && v != "" {
		stage, perr := strconv.Atoi(v)
		if perr != nil {
			return models.RampState{}, false, fmt.Errorf("parse current_stage=%q: %w", v, perr)
		}
		s.CurrentStage = stage
	}
	// CleanDayCounter
	if v, ok := m["clean_day_counter"]; ok && v != "" {
		ctr, perr := strconv.Atoi(v)
		if perr != nil {
			return models.RampState{}, false, fmt.Errorf("parse clean_day_counter=%q: %w", v, perr)
		}
		s.CleanDayCounter = ctr
	}
	// LastEvalTs
	if v, ok := m["last_eval_ts"]; ok && v != "" {
		ts, perr := time.Parse(time.RFC3339Nano, v)
		if perr != nil {
			return models.RampState{}, false, fmt.Errorf("parse last_eval_ts=%q: %w", v, perr)
		}
		s.LastEvalTs = ts
	}
	// LastLossDayTs
	if v, ok := m["last_loss_day_ts"]; ok && v != "" {
		ts, perr := time.Parse(time.RFC3339Nano, v)
		if perr != nil {
			return models.RampState{}, false, fmt.Errorf("parse last_loss_day_ts=%q: %w", v, perr)
		}
		s.LastLossDayTs = ts
	}
	// DemoteCount
	if v, ok := m["demote_count"]; ok && v != "" {
		dc, perr := strconv.Atoi(v)
		if perr != nil {
			return models.RampState{}, false, fmt.Errorf("parse demote_count=%q: %w", v, perr)
		}
		s.DemoteCount = dc
	}
	return s, true, nil
}

// AppendPriceGapRampEvent JSON-marshals ev, RPUSHes onto the pg:ramp:events
// LIST, and trims to priceGapRampEventsCap (500). Best-effort — cap-overflow
// drops the oldest entries; callers expect a bounded operator log, not a
// durable audit trail.
func (c *Client) AppendPriceGapRampEvent(ev models.RampEvent) error {
	data, err := json.Marshal(ev)
	if err != nil {
		return fmt.Errorf("marshal ramp event: %w", err)
	}
	ctx := context.Background()
	pipe := c.rdb.Pipeline()
	pipe.RPush(ctx, keyPricegapRampEvents, data)
	pipe.LTrim(ctx, keyPricegapRampEvents, -priceGapRampEventsCap, -1)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("rpush+ltrim pg:ramp:events: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Phase 15 (PG-LIVE-02) — Drawdown circuit breaker persistence
// ---------------------------------------------------------------------------

// SaveBreakerState writes the 5-field BreakerState atomically to the HASH at
// keyPriceGapBreakerState (D-05). All numeric fields are stored as decimal
// strings so the int64 sentinel PaperModeStickyUntil=math.MaxInt64 (D-07)
// roundtrips without float truncation. The HSet pipeline guarantees
// all-or-nothing semantics — concurrent readers see either the prior or the
// new state, never a partial mix.
func (c *Client) SaveBreakerState(s models.BreakerState) error {
	ctx := context.Background()
	pipe := c.rdb.Pipeline()
	pipe.HSet(ctx, keyPriceGapBreakerState, map[string]any{
		"pending_strike":          strconv.Itoa(s.PendingStrike),
		"strike1_ts":              strconv.FormatInt(s.Strike1Ts, 10),
		"last_eval_ts":            strconv.FormatInt(s.LastEvalTs, 10),
		"last_eval_pnl_usdt":      strconv.FormatFloat(s.LastEvalPnLUSDT, 'f', -1, 64),
		"paper_mode_sticky_until": strconv.FormatInt(s.PaperModeStickyUntil, 10),
	})
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("hset pg:breaker:state: %w", err)
	}
	return nil
}

// LoadBreakerState returns (state, exists, err). On a fresh database (HASH
// absent), exists=false and state is the zero value — boot-time fresh-init
// signal per Phase 15 RESEARCH §"Boot guard". Decode failures return an error
// so the BreakerController can fail-safe-to-paper (Pitfall 8).
func (c *Client) LoadBreakerState() (models.BreakerState, bool, error) {
	ctx := context.Background()
	m, err := c.rdb.HGetAll(ctx, keyPriceGapBreakerState).Result()
	if err != nil {
		return models.BreakerState{}, false, fmt.Errorf("hgetall pg:breaker:state: %w", err)
	}
	if len(m) == 0 {
		return models.BreakerState{}, false, nil
	}
	var s models.BreakerState
	if v, ok := m["pending_strike"]; ok && v != "" {
		n, perr := strconv.Atoi(v)
		if perr != nil {
			return models.BreakerState{}, false, fmt.Errorf("parse pending_strike=%q: %w", v, perr)
		}
		s.PendingStrike = n
	}
	if v, ok := m["strike1_ts"]; ok && v != "" {
		n, perr := strconv.ParseInt(v, 10, 64)
		if perr != nil {
			return models.BreakerState{}, false, fmt.Errorf("parse strike1_ts=%q: %w", v, perr)
		}
		s.Strike1Ts = n
	}
	if v, ok := m["last_eval_ts"]; ok && v != "" {
		n, perr := strconv.ParseInt(v, 10, 64)
		if perr != nil {
			return models.BreakerState{}, false, fmt.Errorf("parse last_eval_ts=%q: %w", v, perr)
		}
		s.LastEvalTs = n
	}
	if v, ok := m["last_eval_pnl_usdt"]; ok && v != "" {
		f, perr := strconv.ParseFloat(v, 64)
		if perr != nil {
			return models.BreakerState{}, false, fmt.Errorf("parse last_eval_pnl_usdt=%q: %w", v, perr)
		}
		s.LastEvalPnLUSDT = f
	}
	if v, ok := m["paper_mode_sticky_until"]; ok && v != "" {
		n, perr := strconv.ParseInt(v, 10, 64)
		if perr != nil {
			return models.BreakerState{}, false, fmt.Errorf("parse paper_mode_sticky_until=%q: %w", v, perr)
		}
		s.PaperModeStickyUntil = n
	}
	return s, true, nil
}

// AppendBreakerTrip JSON-marshals the record, LPUSHes it onto the trip LIST,
// and LTRIMs to priceGapBreakerTripsCap (500) — D-18. Newest-first ordering
// (LPUSH inserts at index 0) lets recovery's LSET target index 0 by default.
func (c *Client) AppendBreakerTrip(record models.BreakerTripRecord) error {
	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal breaker trip: %w", err)
	}
	ctx := context.Background()
	pipe := c.rdb.Pipeline()
	pipe.LPush(ctx, keyPriceGapBreakerTrips, data)
	// LTrim 0 499 caps at 500 entries (priceGapBreakerTripsCap = 500).
	pipe.LTrim(ctx, keyPriceGapBreakerTrips, 0, 499)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("lpush+ltrim pg:breaker:trips: %w", err)
	}
	return nil
}

// UpdateBreakerTripRecovery loads the trip record at the given index, backfills
// RecoveryTs + RecoveryOperator, and LSETs it back. Used by the recovery path
// (D-09 + D-15) to record operator action without disturbing the original trip
// fields. Returns an error if the index is out of range.
func (c *Client) UpdateBreakerTripRecovery(index int64, recoveryTs int64, operator string) error {
	ctx := context.Background()
	raw, err := c.rdb.LIndex(ctx, keyPriceGapBreakerTrips, index).Result()
	if err == redis.Nil {
		return fmt.Errorf("trip log index %d: not found", index)
	}
	if err != nil {
		return fmt.Errorf("lindex pg:breaker:trips %d: %w", index, err)
	}
	var rec models.BreakerTripRecord
	if err := json.Unmarshal([]byte(raw), &rec); err != nil {
		return fmt.Errorf("unmarshal trip @%d: %w", index, err)
	}
	rec.RecoveryTs = &recoveryTs
	rec.RecoveryOperator = &operator
	updated, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("marshal updated trip: %w", err)
	}
	if err := c.rdb.LSet(ctx, keyPriceGapBreakerTrips, index, updated).Err(); err != nil {
		return fmt.Errorf("lset pg:breaker:trips %d: %w", index, err)
	}
	return nil
}
