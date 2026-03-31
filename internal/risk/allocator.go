package risk

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"arb/internal/config"
	"arb/internal/database"
	"arb/internal/models"
	"arb/pkg/utils"

	"github.com/redis/go-redis/v9"
)

type Strategy string

const (
	StrategyPerpPerp    Strategy = "perp_perp"
	StrategySpotFutures Strategy = "spot_futures"

	allocatorVersionKey      = "risk:capital:version"
	allocatorReservationTTL  = 5 * time.Minute
	allocatorCommittedPrefix = "risk:capital:committed:"
	allocatorReservationPref = "risk:capital:reserved:"
	allocatorPositionPrefix  = "risk:capital:positions:"
)

type CapitalReservation struct {
	ID        string             `json:"id"`
	Strategy  Strategy           `json:"strategy"`
	Exposures map[string]float64 `json:"exposures"`
	CreatedAt time.Time          `json:"created_at"`
	ExpiresAt time.Time          `json:"expires_at"`
}

type allocatorPosition struct {
	Strategy    Strategy           `json:"strategy"`
	Exposures   map[string]float64 `json:"exposures"`
	CommittedAt time.Time          `json:"committed_at"`
}

type CapitalSummary struct {
	TotalExposure float64              `json:"total_exposure"`
	ByStrategy    map[Strategy]float64 `json:"by_strategy"`
	ByExchange    map[string]float64   `json:"by_exchange"`
	Reservations  int                  `json:"reservations"`
}

type allocatorState struct {
	total        float64
	byStrategy   map[Strategy]float64
	byExchange   map[string]float64
	reservations int
}

// CapitalAllocator enforces shared capital limits across perp-perp and
// spot-futures engines using Redis-backed reservations and committed totals.
type CapitalAllocator struct {
	db  *database.Client
	cfg *config.Config
	log *utils.Logger
}

func NewCapitalAllocator(db *database.Client, cfg *config.Config) *CapitalAllocator {
	return &CapitalAllocator{
		db:  db,
		cfg: cfg,
		log: utils.NewLogger("capital-allocator"),
	}
}

func (a *CapitalAllocator) Enabled() bool {
	return a != nil && a.cfg != nil && a.cfg.EnableCapitalAllocator
}

func (a *CapitalAllocator) Reserve(strategy Strategy, exposures map[string]float64) (*CapitalReservation, error) {
	if !a.Enabled() {
		return nil, nil
	}

	cleaned := cleanExposures(exposures)
	if len(cleaned) == 0 {
		return nil, nil
	}

	now := time.Now().UTC()
	ttl := a.reservationTTL()
	res := &CapitalReservation{
		ID:        utils.GenerateID("cap", now.UnixMilli()),
		Strategy:  strategy,
		Exposures: cleaned,
		CreatedAt: now,
		ExpiresAt: now.Add(ttl),
	}
	payload, err := json.Marshal(res)
	if err != nil {
		return nil, fmt.Errorf("marshal reservation: %w", err)
	}

	ctx := context.Background()
	if err := a.withVersionRetry(ctx, func(tx *redis.Tx) error {
		state, err := a.loadState(ctx, tx)
		if err != nil {
			return err
		}
		if err := a.checkCaps(state, strategy, cleaned); err != nil {
			return err
		}
		_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
			pipe.Set(ctx, a.reservationKey(res.ID), payload, ttl)
			pipe.Incr(ctx, allocatorVersionKey)
			return nil
		})
		return err
	}); err != nil {
		return nil, err
	}

	return res, nil
}

func (a *CapitalAllocator) Commit(res *CapitalReservation, positionID string, exposures map[string]float64) error {
	if !a.Enabled() || res == nil || positionID == "" {
		return nil
	}

	committed := cleanExposures(exposures)
	if len(committed) == 0 {
		committed = cleanExposures(res.Exposures)
	}
	if len(committed) == 0 {
		return nil
	}

	position := allocatorPosition{
		Strategy:    res.Strategy,
		Exposures:   committed,
		CommittedAt: time.Now().UTC(),
	}
	positionPayload, err := json.Marshal(position)
	if err != nil {
		return fmt.Errorf("marshal committed position: %w", err)
	}

	ctx := context.Background()
	return a.withVersionRetry(ctx, func(tx *redis.Tx) error {
		for exchangeName, amount := range committed {
			current, err := a.readFloatKey(ctx, tx, a.committedKey(res.Strategy, exchangeName))
			if err != nil {
				return err
			}
			committed[exchangeName] = current + amount
		}

		_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
			for exchangeName, total := range committed {
				pipe.Set(ctx, a.committedKey(res.Strategy, exchangeName), strconv.FormatFloat(total, 'f', -1, 64), 0)
			}
			pipe.Del(ctx, a.reservationKey(res.ID))
			pipe.Set(ctx, a.positionKey(positionID), positionPayload, 0)
			pipe.Incr(ctx, allocatorVersionKey)
			return nil
		})
		return err
	})
}

func (a *CapitalAllocator) ReleaseReservation(reservationID string) error {
	if !a.Enabled() || reservationID == "" {
		return nil
	}

	ctx := context.Background()
	return a.withVersionRetry(ctx, func(tx *redis.Tx) error {
		_, err := tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
			pipe.Del(ctx, a.reservationKey(reservationID))
			pipe.Incr(ctx, allocatorVersionKey)
			return nil
		})
		return err
	})
}

func (a *CapitalAllocator) ReleasePosition(positionID string) error {
	if !a.Enabled() || positionID == "" {
		return nil
	}

	ctx := context.Background()
	return a.withVersionRetry(ctx, func(tx *redis.Tx) error {
		raw, err := tx.Get(ctx, a.positionKey(positionID)).Bytes()
		if err == redis.Nil {
			return nil
		}
		if err != nil {
			return err
		}

		var pos allocatorPosition
		if err := json.Unmarshal(raw, &pos); err != nil {
			return fmt.Errorf("unmarshal allocator position %s: %w", positionID, err)
		}

		nextTotals := make(map[string]float64, len(pos.Exposures))
		for exchangeName, amount := range cleanExposures(pos.Exposures) {
			current, err := a.readFloatKey(ctx, tx, a.committedKey(pos.Strategy, exchangeName))
			if err != nil {
				return err
			}
			next := current - amount
			if next < 0 {
				next = 0
			}
			nextTotals[exchangeName] = next
		}

		_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
			for exchangeName, next := range nextTotals {
				pipe.Set(ctx, a.committedKey(pos.Strategy, exchangeName), strconv.FormatFloat(next, 'f', -1, 64), 0)
			}
			pipe.Del(ctx, a.positionKey(positionID))
			pipe.Incr(ctx, allocatorVersionKey)
			return nil
		})
		return err
	})
}

func (a *CapitalAllocator) Summary() (*CapitalSummary, error) {
	if !a.Enabled() {
		return &CapitalSummary{
			ByStrategy: map[Strategy]float64{},
			ByExchange: map[string]float64{},
		}, nil
	}

	state, err := a.loadState(context.Background(), a.db.Redis())
	if err != nil {
		return nil, err
	}

	return &CapitalSummary{
		TotalExposure: state.total,
		ByStrategy:    state.byStrategy,
		ByExchange:    state.byExchange,
		Reservations:  state.reservations,
	}, nil
}

// Reconcile rebuilds committed allocator state from the currently active
// perp-perp and spot-futures positions stored in Redis.
func (a *CapitalAllocator) Reconcile() error {
	if !a.Enabled() {
		return nil
	}

	perpPositions, err := a.db.GetActivePositions()
	if err != nil {
		return fmt.Errorf("load active perp positions: %w", err)
	}
	spotPositions, err := a.db.GetActiveSpotPositions()
	if err != nil {
		return fmt.Errorf("load active spot positions: %w", err)
	}

	committed := make(map[string]float64)
	positionPayloads := make(map[string][]byte)
	leverage := float64(a.cfg.Leverage)
	if leverage <= 0 {
		leverage = 1
	}

	for _, pos := range perpPositions {
		if pos == nil || pos.Status == models.StatusClosed {
			continue
		}
		exposures := cleanExposures(map[string]float64{
			pos.LongExchange:  (pos.LongSize * pos.LongEntry) / leverage,
			pos.ShortExchange: (pos.ShortSize * pos.ShortEntry) / leverage,
		})
		if len(exposures) == 0 {
			continue
		}
		for exchangeName, amount := range exposures {
			committed[a.committedKey(StrategyPerpPerp, exchangeName)] += amount
		}
		payload, err := json.Marshal(allocatorPosition{
			Strategy:    StrategyPerpPerp,
			Exposures:   exposures,
			CommittedAt: time.Now().UTC(),
		})
		if err != nil {
			return fmt.Errorf("marshal perp allocator position %s: %w", pos.ID, err)
		}
		positionPayloads[a.positionKey(pos.ID)] = payload
	}

	for _, pos := range spotPositions {
		if pos == nil || pos.Status == models.SpotStatusClosed {
			continue
		}
		exposures := cleanExposures(map[string]float64{
			pos.Exchange: pos.NotionalUSDT,
		})
		if len(exposures) == 0 {
			continue
		}
		for exchangeName, amount := range exposures {
			committed[a.committedKey(StrategySpotFutures, exchangeName)] += amount
		}
		payload, err := json.Marshal(allocatorPosition{
			Strategy:    StrategySpotFutures,
			Exposures:   exposures,
			CommittedAt: time.Now().UTC(),
		})
		if err != nil {
			return fmt.Errorf("marshal spot allocator position %s: %w", pos.ID, err)
		}
		positionPayloads[a.positionKey(pos.ID)] = payload
	}

	ctx := context.Background()
	return a.withVersionRetry(ctx, func(tx *redis.Tx) error {
		committedKeys, err := tx.Keys(ctx, allocatorCommittedPrefix+"*").Result()
		if err != nil && err != redis.Nil {
			return err
		}
		positionKeys, err := tx.Keys(ctx, allocatorPositionPrefix+"*").Result()
		if err != nil && err != redis.Nil {
			return err
		}

		_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
			if len(committedKeys) > 0 {
				pipe.Del(ctx, committedKeys...)
			}
			if len(positionKeys) > 0 {
				pipe.Del(ctx, positionKeys...)
			}
			for key, amount := range committed {
				pipe.Set(ctx, key, strconv.FormatFloat(amount, 'f', -1, 64), 0)
			}
			for key, payload := range positionPayloads {
				pipe.Set(ctx, key, payload, 0)
			}
			pipe.Incr(ctx, allocatorVersionKey)
			return nil
		})
		return err
	})
}

func (a *CapitalAllocator) withVersionRetry(ctx context.Context, fn func(tx *redis.Tx) error) error {
	if !a.Enabled() {
		return nil
	}

	var lastErr error
	for range 8 {
		err := a.db.Redis().Watch(ctx, fn, allocatorVersionKey)
		if err == nil {
			return nil
		}
		if err == redis.TxFailedErr {
			lastErr = err
			continue
		}
		return err
	}
	if lastErr != nil {
		return fmt.Errorf("allocator transaction retry exhausted: %w", lastErr)
	}
	return fmt.Errorf("allocator transaction retry exhausted")
}

func (a *CapitalAllocator) loadState(ctx context.Context, reader redis.Cmdable) (allocatorState, error) {
	state := allocatorState{
		byStrategy: make(map[Strategy]float64),
		byExchange: make(map[string]float64),
	}

	committedKeys, err := reader.Keys(ctx, allocatorCommittedPrefix+"*").Result()
	if err != nil && err != redis.Nil {
		return state, err
	}
	for _, key := range committedKeys {
		amount, err := a.readFloatKey(ctx, reader, key)
		if err != nil {
			return state, err
		}
		if amount <= 0 {
			continue
		}
		strategy, exchangeName, ok := parseCommittedKey(key)
		if !ok {
			continue
		}
		state.total += amount
		state.byStrategy[strategy] += amount
		state.byExchange[exchangeName] += amount
	}

	reservationKeys, err := reader.Keys(ctx, allocatorReservationPref+"*").Result()
	if err != nil && err != redis.Nil {
		return state, err
	}
	now := time.Now().UTC()
	for _, key := range reservationKeys {
		raw, err := reader.Get(ctx, key).Bytes()
		if err == redis.Nil {
			continue
		}
		if err != nil {
			return state, err
		}
		var res CapitalReservation
		if err := json.Unmarshal(raw, &res); err != nil {
			return state, fmt.Errorf("unmarshal reservation %s: %w", key, err)
		}
		if !res.ExpiresAt.IsZero() && now.After(res.ExpiresAt) {
			continue
		}
		state.reservations++
		for exchangeName, amount := range cleanExposures(res.Exposures) {
			state.total += amount
			state.byStrategy[res.Strategy] += amount
			state.byExchange[exchangeName] += amount
		}
	}

	return state, nil
}

func (a *CapitalAllocator) checkCaps(state allocatorState, strategy Strategy, exposures map[string]float64) error {
	totalCap := a.cfg.MaxTotalExposureUSDT
	if totalCap <= 0 {
		return nil
	}

	requestTotal := 0.0
	for _, amount := range exposures {
		requestTotal += amount
	}
	if requestTotal <= 0 {
		return nil
	}

	if state.total+requestTotal > totalCap {
		return fmt.Errorf("total capital cap exceeded: need %.2f, available %.2f", requestTotal, totalCap-state.total)
	}

	strategyCap := totalCap * a.strategyPct(strategy)
	if strategyCap > 0 && state.byStrategy[strategy]+requestTotal > strategyCap {
		return fmt.Errorf("%s cap exceeded: requested %.2f would reach %.2f / %.2f", strategy, requestTotal, state.byStrategy[strategy]+requestTotal, strategyCap)
	}

	exchangeCap := totalCap * a.cfg.MaxPerExchangePct
	if exchangeCap > 0 {
		for exchangeName, amount := range exposures {
			if state.byExchange[exchangeName]+amount > exchangeCap {
				return fmt.Errorf("exchange cap exceeded on %s: requested %.2f would reach %.2f / %.2f", exchangeName, amount, state.byExchange[exchangeName]+amount, exchangeCap)
			}
		}
	}

	return nil
}

func (a *CapitalAllocator) strategyPct(strategy Strategy) float64 {
	switch strategy {
	case StrategySpotFutures:
		return a.cfg.MaxSpotFuturesPct
	default:
		return a.cfg.MaxPerpPerpPct
	}
}

func (a *CapitalAllocator) reservationTTL() time.Duration {
	if a.cfg == nil || a.cfg.ReservationTTLSec <= 0 {
		return allocatorReservationTTL
	}
	return time.Duration(a.cfg.ReservationTTLSec) * time.Second
}

func (a *CapitalAllocator) committedKey(strategy Strategy, exchangeName string) string {
	return allocatorCommittedPrefix + string(strategy) + ":" + strings.ToLower(exchangeName)
}

func (a *CapitalAllocator) reservationKey(id string) string {
	return allocatorReservationPref + id
}

func (a *CapitalAllocator) positionKey(positionID string) string {
	return allocatorPositionPrefix + positionID
}

func (a *CapitalAllocator) readFloatKey(ctx context.Context, reader redis.Cmdable, key string) (float64, error) {
	val, err := reader.Get(ctx, key).Result()
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	parsed, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return 0, fmt.Errorf("parse float %s=%q: %w", key, val, err)
	}
	return parsed, nil
}

func cleanExposures(exposures map[string]float64) map[string]float64 {
	out := make(map[string]float64)
	for exchangeName, amount := range exposures {
		if amount <= 0 {
			continue
		}
		out[strings.ToLower(strings.TrimSpace(exchangeName))] += amount
	}
	return out
}

func parseCommittedKey(key string) (Strategy, string, bool) {
	trimmed := strings.TrimPrefix(key, allocatorCommittedPrefix)
	parts := strings.SplitN(trimmed, ":", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	return Strategy(parts[0]), parts[1], true
}
