package risk

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"arb/pkg/utils"

	"github.com/redis/go-redis/v9"
)

// BatchReservationItem is a single candidate reservation within a batch. Units
// match ReserveWithCap:
//   - Key is a caller-supplied unique identifier used for later Commit /
//     Release and for addressing items inside the returned BatchReservation.
//   - Exposures map keys are exchange names (case-insensitive) and values are
//     USDT exposure amounts (per-leg margin for perp, planned notional USDT
//     for spot — same units ReserveWithCap takes today).
//   - CapOverride mirrors ReserveWithCap's capOverride parameter; 0 means "use
//     static / effective strategy percentage".
type BatchReservationItem struct {
	Key         string
	Strategy    Strategy
	Exposures   map[string]float64
	CapOverride float64
}

// BatchReservation is the atomic all-or-nothing result of ReserveBatch. Items
// is keyed by the caller's BatchReservationItem.Key so downstream dispatch can
// commit winners individually and release the remainder via ReleaseBatch.
// Each individual *CapitalReservation inside Items is a fully persisted Redis
// reservation identical to the ones ReserveWithCap produces.
type BatchReservation struct {
	ID        string
	Items     map[string]*CapitalReservation
	CreatedAt time.Time
	ExpiresAt time.Time
}

// ReserveBatch atomically reserves capital for every BatchReservationItem.
// Semantics:
//   - Cumulative cap validation runs ONCE against the live allocator state
//     plus the accumulated exposures of prior items in the same batch, so a
//     batch cannot silently exceed total / per-strategy / per-exchange caps
//     even when no single item would individually breach them.
//   - On any validation failure the function returns an error and NO
//     reservations persist — the Redis write happens inside a single
//     optimistic transaction guarded by the allocator version key, so either
//     all reservation keys land together or none of them do.
//   - An empty or fully empty-exposure input returns (nil, nil) to mirror
//     ReserveWithCap's no-op behavior.
//
// The allocator must be enabled; a disabled allocator returns (nil, nil) to
// match the existing single-reservation APIs and let callers no-op gracefully
// when allocator is off.
func (a *CapitalAllocator) ReserveBatch(items []BatchReservationItem) (*BatchReservation, error) {
	if !a.Enabled() {
		return nil, nil
	}
	if len(items) == 0 {
		return nil, nil
	}

	// Pre-clean inputs. Drop items whose exposures reduce to empty; these are
	// no-ops in the single-reservation path and we preserve that here.
	type prepared struct {
		item     BatchReservationItem
		cleaned  map[string]float64
		resID    string
		resKey   string
		payload  []byte
		createAt time.Time
		expireAt time.Time
	}
	ttl := a.reservationTTL()
	now := time.Now().UTC()
	prepList := make([]prepared, 0, len(items))
	for idx, it := range items {
		cleaned := cleanExposures(it.Exposures)
		if len(cleaned) == 0 {
			continue
		}
		id := utils.GenerateID("capbatch", now.UnixMilli()+int64(idx))
		res := &CapitalReservation{
			ID:        id,
			Strategy:  it.Strategy,
			Exposures: cleaned,
			CreatedAt: now,
			ExpiresAt: now.Add(ttl),
		}
		payload, err := json.Marshal(res)
		if err != nil {
			return nil, fmt.Errorf("marshal batch reservation %s: %w", it.Key, err)
		}
		prepList = append(prepList, prepared{
			item:     it,
			cleaned:  cleaned,
			resID:    id,
			resKey:   a.reservationKey(id),
			payload:  payload,
			createAt: now,
			expireAt: now.Add(ttl),
		})
	}
	if len(prepList) == 0 {
		return nil, nil
	}

	batchID := utils.GenerateID("capbatchid", now.UnixMilli())
	batch := &BatchReservation{
		ID:        batchID,
		Items:     make(map[string]*CapitalReservation, len(prepList)),
		CreatedAt: now,
		ExpiresAt: now.Add(ttl),
	}

	ctx := context.Background()
	if err := a.withVersionRetry(ctx, func(tx *redis.Tx) error {
		state, err := a.loadState(ctx, tx)
		if err != nil {
			return err
		}

		// Cumulative cap validation: mutate a local copy of the state as if
		// each prior item in the batch has already been accepted, so the
		// batch is rejected atomically when the combined exposures breach
		// any cap even if no single item alone would.
		cumByStrategy := make(map[Strategy]float64, len(state.byStrategy))
		for k, v := range state.byStrategy {
			cumByStrategy[k] = v
		}
		cumByExchange := make(map[string]float64, len(state.byExchange))
		for k, v := range state.byExchange {
			cumByExchange[k] = v
		}
		cumTotal := state.total

		for _, p := range prepList {
			simulated := allocatorState{
				total:        cumTotal,
				byStrategy:   cumByStrategy,
				byExchange:   cumByExchange,
				reservations: state.reservations,
			}
			if err := a.checkCapsWithOverride(simulated, p.item.Strategy, p.cleaned, p.item.CapOverride); err != nil {
				return fmt.Errorf("batch item %q: %w", p.item.Key, err)
			}
			// Accept this item into the cumulative totals for the next
			// item's check.
			for exchangeName, amount := range p.cleaned {
				cumByExchange[exchangeName] += amount
				cumByStrategy[p.item.Strategy] += amount
				cumTotal += amount
			}
		}

		// All items pass — persist every reservation key in one transaction.
		// Using TxPipelined inside the watched tx gives all-or-nothing
		// persistence: either every SET lands together or the tx is retried
		// by withVersionRetry when the version key has changed.
		_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
			for _, p := range prepList {
				pipe.Set(ctx, p.resKey, p.payload, ttl)
			}
			pipe.Incr(ctx, allocatorVersionKey)
			return nil
		})
		return err
	}); err != nil {
		return nil, err
	}

	for _, p := range prepList {
		res := &CapitalReservation{
			ID:        p.resID,
			Strategy:  p.item.Strategy,
			Exposures: p.cleaned,
			CreatedAt: p.createAt,
			ExpiresAt: p.expireAt,
		}
		batch.Items[p.item.Key] = res
	}
	return batch, nil
}

// ReleaseBatch releases every reservation in batch that has NOT already been
// committed. Committed reservations (their reservation keys already deleted by
// Commit) are silently skipped so ReleaseBatch is safe to call once dispatch
// completes — winners that were committed stay committed, losers and failures
// are released back to the pool.
//
// A disabled allocator or a nil batch is a no-op, mirroring ReleaseReservation.
func (a *CapitalAllocator) ReleaseBatch(batch *BatchReservation) error {
	if !a.Enabled() || batch == nil {
		return nil
	}
	if len(batch.Items) == 0 {
		return nil
	}

	ctx := context.Background()
	return a.withVersionRetry(ctx, func(tx *redis.Tx) error {
		// Inspect each reservation key under the watched tx. When the key is
		// missing the reservation was either committed (reservation key
		// deleted by Commit) or expired — either way there is nothing to
		// release and we skip it.
		toDelete := make([]string, 0, len(batch.Items))
		for _, res := range batch.Items {
			if res == nil || res.ID == "" {
				continue
			}
			key := a.reservationKey(res.ID)
			exists, err := tx.Exists(ctx, key).Result()
			if err != nil {
				return fmt.Errorf("check batch reservation %s: %w", res.ID, err)
			}
			if exists == 0 {
				continue
			}
			toDelete = append(toDelete, key)
		}
		if len(toDelete) == 0 {
			// Still bump the version key so concurrent readers observe a
			// consistent snapshot even on a fully-committed batch release.
			_, err := tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
				pipe.Incr(ctx, allocatorVersionKey)
				return nil
			})
			return err
		}

		_, err := tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
			pipe.Del(ctx, toDelete...)
			pipe.Incr(ctx, allocatorVersionKey)
			return nil
		})
		return err
	})
}
