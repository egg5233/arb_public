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

// Compile-time check: *Client satisfies models.StateStore.
var _ models.StateStore = (*Client)(nil)

// Redis key constants.
const (
	keyConfig           = "arb:config"
	keyPositions        = "arb:positions"
	keyPositionsActive  = "arb:positions:active"
	keyHistory          = "arb:history"
	keyFundingLatest    = "arb:funding:latest"
	keyStats            = "arb:stats"
	keyTransfers        = "arb:transfers"
	keyTransfersHistory = "arb:transfers:history"
	keyLossCooldownPrefix    = "arb:lossCooldown:"
	keyReEnterCooldownPrefix = "arb:reEnterCooldown:"

	historyMaxLen         = 1000
	fundingSnapshotMaxLen = 100
	transfersMaxLen       = 200
)

// TransferRecord represents a cross-exchange fund transfer.
type TransferRecord struct {
	ID        string `json:"id"`
	From      string `json:"from"`
	To        string `json:"to"`
	Coin      string `json:"coin"`
	Chain     string `json:"chain"`
	Amount    string `json:"amount"`
	Fee       string `json:"fee"`
	TxID      string `json:"tx_id"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}

func fundingSnapshotsKey(symbol string) string {
	return fmt.Sprintf("arb:funding:snapshots:%s", symbol)
}

func balanceKey(exchange string) string {
	return fmt.Sprintf("arb:exchange:%s:balance", exchange)
}

func spotBalanceKey(exchange string) string {
	return fmt.Sprintf("arb:exchange:%s:spotBalance", exchange)
}

// ---------------------------------------------------------------------------
// Config (arb:config HASH)
// ---------------------------------------------------------------------------

// SetConfigField sets a single field in the arb:config hash.
func (c *Client) SetConfigField(field, value string) error {
	ctx := context.Background()
	return c.rdb.HSet(ctx, keyConfig, field, value).Err()
}

// SetConfigFields sets multiple fields in the arb:config hash at once.
func (c *Client) SetConfigFields(fields map[string]interface{}) error {
	ctx := context.Background()
	return c.rdb.HSet(ctx, keyConfig, fields).Err()
}

// GetConfigField retrieves a single field from the arb:config hash.
func (c *Client) GetConfigField(field string) (string, error) {
	ctx := context.Background()
	val, err := c.rdb.HGet(ctx, keyConfig, field).Result()
	if err == redis.Nil {
		return "", nil
	}
	return val, err
}

// GetAllConfig returns all fields from the arb:config hash.
func (c *Client) GetAllConfig() (map[string]string, error) {
	ctx := context.Background()
	return c.rdb.HGetAll(ctx, keyConfig).Result()
}

// ---------------------------------------------------------------------------
// Position CRUD
// ---------------------------------------------------------------------------

// SavePosition serialises the position as JSON and stores it in the positions
// hash. If the position status is active, pending, partial, or closing it is
// also added to the active set.
func (c *Client) SavePosition(pos *models.ArbitragePosition) error {
	ctx := context.Background()

	data, err := json.Marshal(pos)
	if err != nil {
		return fmt.Errorf("marshal position: %w", err)
	}

	pipe := c.rdb.Pipeline()
	pipe.HSet(ctx, keyPositions, pos.ID, data)

	switch pos.Status {
	case models.StatusClosed:
		pipe.SRem(ctx, keyPositionsActive, pos.ID)
	default:
		pipe.SAdd(ctx, keyPositionsActive, pos.ID)
	}

	_, err = pipe.Exec(ctx)
	return err
}

// GetPosition retrieves a single position by ID.
func (c *Client) GetPosition(id string) (*models.ArbitragePosition, error) {
	ctx := context.Background()

	data, err := c.rdb.HGet(ctx, keyPositions, id).Bytes()
	if err == redis.Nil {
		return nil, fmt.Errorf("position %s not found", id)
	}
	if err != nil {
		return nil, err
	}

	var pos models.ArbitragePosition
	if err := json.Unmarshal(data, &pos); err != nil {
		return nil, fmt.Errorf("unmarshal position: %w", err)
	}
	return &pos, nil
}

// GetActivePositions returns all positions whose IDs are in the active set.
func (c *Client) GetActivePositions() ([]*models.ArbitragePosition, error) {
	ctx := context.Background()

	ids, err := c.rdb.SMembers(ctx, keyPositionsActive).Result()
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}

	vals, err := c.rdb.HMGet(ctx, keyPositions, ids...).Result()
	if err != nil {
		return nil, err
	}

	positions := make([]*models.ArbitragePosition, 0, len(vals))
	for _, v := range vals {
		if v == nil {
			continue
		}
		s, ok := v.(string)
		if !ok {
			continue
		}
		var pos models.ArbitragePosition
		if err := json.Unmarshal([]byte(s), &pos); err != nil {
			continue
		}
		positions = append(positions, &pos)
	}
	return positions, nil
}

// UpdatePositionStatus changes a position's status field and persists it.
func (c *Client) UpdatePositionStatus(id, status string) error {
	pos, err := c.GetPosition(id)
	if err != nil {
		return err
	}

	pos.Status = status
	pos.UpdatedAt = time.Now().UTC()
	return c.SavePosition(pos)
}

// UpdatePositionFields atomically re-reads a position from Redis and applies
// the given mutator function before saving. This prevents concurrent goroutines
// from overwriting each other's changes (e.g. status set by closePosition being
// reverted by updateFundingCollected saving a stale copy).
// The mutator should return false if the update should be skipped (e.g. if the
// position status has changed since the caller last read it).
func (c *Client) UpdatePositionFields(id string, mutate func(pos *models.ArbitragePosition) bool) error {
	pos, err := c.GetPosition(id)
	if err != nil {
		return err
	}

	if !mutate(pos) {
		return nil // caller decided to skip
	}

	pos.UpdatedAt = time.Now().UTC()
	return c.SavePosition(pos)
}

// ---------------------------------------------------------------------------
// History
// ---------------------------------------------------------------------------

// AddToHistory pushes the position to the history list (most recent first)
// and trims the list to historyMaxLen entries.
func (c *Client) AddToHistory(pos *models.ArbitragePosition) error {
	ctx := context.Background()

	data, err := json.Marshal(pos)
	if err != nil {
		return fmt.Errorf("marshal position: %w", err)
	}

	pipe := c.rdb.Pipeline()
	pipe.LPush(ctx, keyHistory, data)
	pipe.LTrim(ctx, keyHistory, 0, historyMaxLen-1)
	_, err = pipe.Exec(ctx)
	return err
}

// UpdateHistoryEntry finds a position in the history list by ID and replaces
// it with the updated data. Used by reconciliation to correct PnL after close.
func (c *Client) UpdateHistoryEntry(pos *models.ArbitragePosition) error {
	ctx := context.Background()

	vals, err := c.rdb.LRange(ctx, keyHistory, 0, historyMaxLen-1).Result()
	if err != nil {
		return fmt.Errorf("read history: %w", err)
	}

	for i, v := range vals {
		var entry models.ArbitragePosition
		if err := json.Unmarshal([]byte(v), &entry); err != nil {
			continue
		}
		if entry.ID == pos.ID {
			data, err := json.Marshal(pos)
			if err != nil {
				return fmt.Errorf("marshal position: %w", err)
			}
			if err := c.rdb.LSet(ctx, keyHistory, int64(i), data).Err(); err != nil {
				return fmt.Errorf("update history[%d]: %w", i, err)
			}
			return nil
		}
	}
	return nil // not found — may have been trimmed
}

// GetHistory returns the most recent `limit` entries from the history list.
func (c *Client) GetHistory(limit int) ([]*models.ArbitragePosition, error) {
	ctx := context.Background()

	if limit <= 0 {
		limit = 50
	}

	vals, err := c.rdb.LRange(ctx, keyHistory, 0, int64(limit-1)).Result()
	if err != nil {
		return nil, err
	}

	positions := make([]*models.ArbitragePosition, 0, len(vals))
	for _, v := range vals {
		var pos models.ArbitragePosition
		if err := json.Unmarshal([]byte(v), &pos); err != nil {
			continue
		}
		positions = append(positions, &pos)
	}
	return positions, nil
}

// ---------------------------------------------------------------------------
// Funding
// ---------------------------------------------------------------------------

// SaveFundingSnapshot stores the snapshot in the latest hash (keyed by symbol)
// and appends it to the per-symbol historical list (capped at 100).
func (c *Client) SaveFundingSnapshot(snap *models.FundingSnapshot) error {
	ctx := context.Background()

	data, err := json.Marshal(snap)
	if err != nil {
		return fmt.Errorf("marshal funding snapshot: %w", err)
	}

	histKey := fundingSnapshotsKey(snap.Symbol)

	pipe := c.rdb.Pipeline()
	pipe.HSet(ctx, keyFundingLatest, snap.Symbol, data)
	pipe.LPush(ctx, histKey, data)
	pipe.LTrim(ctx, histKey, 0, fundingSnapshotMaxLen-1)
	_, err = pipe.Exec(ctx)
	return err
}

// GetLatestFunding returns the most recent funding snapshot for a symbol.
func (c *Client) GetLatestFunding(symbol string) (*models.FundingSnapshot, error) {
	ctx := context.Background()

	data, err := c.rdb.HGet(ctx, keyFundingLatest, symbol).Bytes()
	if err == redis.Nil {
		return nil, fmt.Errorf("no funding data for %s", symbol)
	}
	if err != nil {
		return nil, err
	}

	var snap models.FundingSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, fmt.Errorf("unmarshal funding snapshot: %w", err)
	}
	return &snap, nil
}

// ---------------------------------------------------------------------------
// Stats
// ---------------------------------------------------------------------------

// UpdateStats atomically increments trade counters and accumulates PnL.
func (c *Client) UpdateStats(pnl float64, won bool) error {
	ctx := context.Background()

	pipe := c.rdb.Pipeline()
	pipe.HIncrByFloat(ctx, keyStats, "total_pnl", pnl)
	pipe.HIncrBy(ctx, keyStats, "trade_count", 1)
	if won {
		pipe.HIncrBy(ctx, keyStats, "win_count", 1)
	} else {
		pipe.HIncrBy(ctx, keyStats, "loss_count", 1)
	}
	_, err := pipe.Exec(ctx)
	return err
}

// GetStats returns all fields from the stats hash.
func (c *Client) GetStats() (map[string]string, error) {
	ctx := context.Background()
	return c.rdb.HGetAll(ctx, keyStats).Result()
}

// ---------------------------------------------------------------------------
// Balance
// ---------------------------------------------------------------------------

// SaveBalance caches the latest balance for an exchange.
func (c *Client) SaveBalance(exchange string, balance float64) error {
	ctx := context.Background()
	return c.rdb.Set(ctx, balanceKey(exchange), strconv.FormatFloat(balance, 'f', -1, 64), 0).Err()
}

// SaveSpotBalance caches the latest spot balance for an exchange.
func (c *Client) SaveSpotBalance(exchange string, balance float64) error {
	ctx := context.Background()
	return c.rdb.Set(ctx, spotBalanceKey(exchange), strconv.FormatFloat(balance, 'f', -1, 64), 0).Err()
}

// GetSpotBalance retrieves the cached spot balance for an exchange.
func (c *Client) GetSpotBalance(exchange string) (float64, error) {
	ctx := context.Background()
	val, err := c.rdb.Get(ctx, spotBalanceKey(exchange)).Result()
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return strconv.ParseFloat(val, 64)
}

// GetBalance retrieves the cached balance for an exchange.
func (c *Client) GetBalance(exchange string) (float64, error) {
	ctx := context.Background()

	val, err := c.rdb.Get(ctx, balanceKey(exchange)).Result()
	if err == redis.Nil {
		return 0, fmt.Errorf("no balance for exchange %s", exchange)
	}
	if err != nil {
		return 0, err
	}

	return strconv.ParseFloat(val, 64)
}

// ---------------------------------------------------------------------------
// Transfers
// ---------------------------------------------------------------------------

// SaveTransfer stores a transfer record and appends its ID to the history list.
func (c *Client) SaveTransfer(t *TransferRecord) error {
	ctx := context.Background()

	data, err := json.Marshal(t)
	if err != nil {
		return fmt.Errorf("marshal transfer: %w", err)
	}

	pipe := c.rdb.Pipeline()
	pipe.HSet(ctx, keyTransfers, t.ID, data)
	pipe.LPush(ctx, keyTransfersHistory, t.ID)
	pipe.LTrim(ctx, keyTransfersHistory, 0, transfersMaxLen-1)
	_, err = pipe.Exec(ctx)
	return err
}

// GetTransfers returns the most recent transfer records.
func (c *Client) GetTransfers(limit int) ([]*TransferRecord, error) {
	ctx := context.Background()

	if limit <= 0 {
		limit = 50
	}

	ids, err := c.rdb.LRange(ctx, keyTransfersHistory, 0, int64(limit-1)).Result()
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}

	vals, err := c.rdb.HMGet(ctx, keyTransfers, ids...).Result()
	if err != nil {
		return nil, err
	}

	transfers := make([]*TransferRecord, 0, len(vals))
	for _, v := range vals {
		if v == nil {
			continue
		}
		s, ok := v.(string)
		if !ok {
			continue
		}
		var t TransferRecord
		if err := json.Unmarshal([]byte(s), &t); err != nil {
			continue
		}
		transfers = append(transfers, &t)
	}
	return transfers, nil
}

// UpdateTransferStatus updates the status of a transfer record.
func (c *Client) UpdateTransferStatus(id, status string) error {
	ctx := context.Background()

	data, err := c.rdb.HGet(ctx, keyTransfers, id).Bytes()
	if err == redis.Nil {
		return fmt.Errorf("transfer %s not found", id)
	}
	if err != nil {
		return err
	}

	var t TransferRecord
	if err := json.Unmarshal(data, &t); err != nil {
		return fmt.Errorf("unmarshal transfer: %w", err)
	}

	t.Status = status
	updated, err := json.Marshal(t)
	if err != nil {
		return fmt.Errorf("marshal transfer: %w", err)
	}
	return c.rdb.HSet(ctx, keyTransfers, id, updated).Err()
}

// SetLossCooldown sets a loss cooldown on a symbol with automatic TTL expiry.
func (c *Client) SetLossCooldown(symbol string, d time.Duration) error {
	ctx := context.Background()
	key := keyLossCooldownPrefix + symbol
	return c.rdb.Set(ctx, key, "1", d).Err()
}

// GetLossCooldown returns the remaining cooldown duration for a symbol.
// Returns 0 if no cooldown is active.
func (c *Client) GetLossCooldown(symbol string) time.Duration {
	ctx := context.Background()
	key := keyLossCooldownPrefix + symbol
	ttl, err := c.rdb.TTL(ctx, key).Result()
	if err != nil || ttl <= 0 {
		return 0
	}
	return ttl
}

// SetReEnterCooldown sets a re-entry cooldown on a symbol with automatic TTL expiry.
func (c *Client) SetReEnterCooldown(symbol string, d time.Duration) error {
	ctx := context.Background()
	key := keyReEnterCooldownPrefix + symbol
	return c.rdb.Set(ctx, key, "1", d).Err()
}

// GetReEnterCooldown returns the remaining re-entry cooldown duration for a symbol.
// Returns 0 if no cooldown is active.
func (c *Client) GetReEnterCooldown(symbol string) time.Duration {
	ctx := context.Background()
	key := keyReEnterCooldownPrefix + symbol
	ttl, err := c.rdb.TTL(ctx, key).Result()
	if err != nil || ttl <= 0 {
		return 0
	}
	return ttl
}
