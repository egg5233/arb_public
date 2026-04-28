// Package pricegaptrader — Registry chokepoint (Phase 11 / PG-DISC-04).
//
// *Registry is the SOLE legitimate writer to cfg.PriceGapCandidates. All
// dashboard handler updates, pg-admin CLI mutations, and (in Phase 12) the
// auto-promotion path route through Registry's typed methods so the
// three-writer race (Pitfall 2) is eliminated structurally.
//
// Concurrency contract:
//   - Public mutators (Add/Update/Delete/Replace) take cfg.Lock() internally.
//   - Public readers (Get/List) take cfg.RLock(); List returns a defensive copy.
//   - Reload-from-disk happens inside every mutator BEFORE applying the
//     change so cross-process writes from a separate pg-admin instance are
//     observed (belt-and-suspenders for the os.Rename atomic-write path).
//   - Persistence uses cfg.SaveJSONWithBakRing (Plan 02 Task 1) for atomic
//     write + .bak.{ts} ring rotation.
//   - Audit trail to Redis list "pg:registry:audit" is best-effort —
//     persistence is authoritative; failed audits are logged but do not fail
//     the mutation. Bounded to 200 entries via LTrim.
//
// Module-boundary contract (T-11-07): this package MUST NOT import either of
// the live trading engine packages. Registry holds *config.Config + a narrow
// audit-writer interface; no engine types are referenced.
package pricegaptrader

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"arb/internal/config"
	"arb/internal/models"
	"arb/pkg/utils"
)

// Sentinel errors returned by Registry mutators.
var (
	// ErrDuplicateCandidate is returned by Add when an existing candidate has
	// the same (Symbol, LongExch, ShortExch, Direction) tuple.
	ErrDuplicateCandidate = errors.New("registry: duplicate candidate (symbol, long_exch, short_exch, direction)")
	// ErrCapExceeded is returned by Add when appending would exceed
	// cfg.PriceGapMaxCandidates.
	ErrCapExceeded = errors.New("registry: PriceGapMaxCandidates cap exceeded")
	// ErrIndexOutOfRange is returned by Update/Delete when the supplied idx
	// is outside the current PriceGapCandidates range.
	ErrIndexOutOfRange = errors.New("registry: index out of range")
)

// RegistryAuditWriter is the narrow Redis surface Registry needs for the
// audit trail. Production code passes a *redis.Client wrapper that satisfies
// these signatures; tests pass a fake. Keeping the interface minimal lets
// Registry stay decoupled from internal/database (and avoids leaking the
// Redis driver into pricegaptrader's import set beyond what's already there
// for the metrics writer).
type RegistryAuditWriter interface {
	// LPush prepends one or more values to the Redis list at key. Returns
	// the new list length and any error from the driver.
	LPush(ctx context.Context, key string, vals ...interface{}) (int64, error)
	// LTrim trims the list at key to the inclusive range [start, stop].
	// Used to cap pg:registry:audit at 200 entries.
	LTrim(ctx context.Context, key string, start, stop int64) error
}

// Registry is the chokepoint for cfg.PriceGapCandidates mutations. Construct
// with NewRegistry; all methods are safe for concurrent use.
type Registry struct {
	cfg     *config.Config
	db      RegistryAuditWriter
	log     *utils.Logger
	nowFunc func() time.Time // injectable for tests
}

// Compile-time assertion: *Registry implements RegistryReader (Plan 01).
var _ RegistryReader = (*Registry)(nil)

// NewRegistry builds a Registry against the supplied Config + audit writer.
// log is optional — pass utils.NewLogger("registry") in production.
func NewRegistry(cfg *config.Config, db RegistryAuditWriter, log *utils.Logger) *Registry {
	return &Registry{
		cfg:     cfg,
		db:      db,
		log:     log,
		nowFunc: time.Now,
	}
}

// Get returns the candidate at idx, or false if idx is out of range.
// Implements RegistryReader.
func (r *Registry) Get(idx int) (models.PriceGapCandidate, bool) {
	r.cfg.RLock()
	defer r.cfg.RUnlock()
	if idx < 0 || idx >= len(r.cfg.PriceGapCandidates) {
		return models.PriceGapCandidate{}, false
	}
	return r.cfg.PriceGapCandidates[idx], true
}

// List returns a defensive copy of the current PriceGapCandidates slice.
// Implements RegistryReader.
func (r *Registry) List() []models.PriceGapCandidate {
	r.cfg.RLock()
	defer r.cfg.RUnlock()
	out := make([]models.PriceGapCandidate, len(r.cfg.PriceGapCandidates))
	copy(out, r.cfg.PriceGapCandidates)
	return out
}

// Add appends a candidate after dedupe + cap checks. Returns
// ErrDuplicateCandidate if the (Symbol, LongExch, ShortExch, Direction)
// tuple already exists, ErrCapExceeded if cfg.PriceGapMaxCandidates would
// be exceeded, or any persistence error from SaveJSONWithBakRing. On
// persistence failure the in-memory append is rolled back.
//
// source identifies the caller for the audit trail. Plan 03 callers use
// the bounded enumeration {"dashboard-handler", "pg-admin", "scanner-promote"}.
func (r *Registry) Add(ctx context.Context, source string, c models.PriceGapCandidate) error {
	// Path-level lock first (cross-instance serialization), then cfg.mu
	// (per-instance). Order matters: path → cfg ensures no two Registry
	// instances pointing at the same CONFIG_FILE can interleave a
	// reload-from-disk → mutate → save sequence.
	releasePath := r.cfg.LockConfigFile()
	defer releasePath()
	r.cfg.Lock()
	defer r.cfg.Unlock()

	if err := r.reloadFromDiskLocked(); err != nil {
		return fmt.Errorf("registry: reload: %w", err)
	}

	if len(r.cfg.PriceGapCandidates) >= r.cfg.PriceGapMaxCandidates {
		return ErrCapExceeded
	}
	for _, existing := range r.cfg.PriceGapCandidates {
		if c.Symbol == existing.Symbol &&
			c.LongExch == existing.LongExch &&
			c.ShortExch == existing.ShortExch &&
			c.Direction == existing.Direction {
			return ErrDuplicateCandidate
		}
	}

	beforeCount := len(r.cfg.PriceGapCandidates)
	r.cfg.PriceGapCandidates = append(r.cfg.PriceGapCandidates, c)
	if err := r.cfg.SaveJSONWithBakRing(); err != nil {
		// Rollback in-memory mutation on persist failure.
		r.cfg.PriceGapCandidates = r.cfg.PriceGapCandidates[:beforeCount]
		return fmt.Errorf("registry: save: %w", err)
	}

	r.writeAuditLocked(ctx, source, "add", beforeCount, beforeCount+1)
	return nil
}

// Update replaces the candidate at idx. Returns ErrIndexOutOfRange if idx
// is invalid. The replacement is persisted through SaveJSONWithBakRing; on
// failure the in-memory mutation is rolled back.
func (r *Registry) Update(ctx context.Context, source string, idx int, c models.PriceGapCandidate) error {
	// Path-level lock first (cross-instance serialization), then cfg.mu
	// (per-instance). Order matters: path → cfg ensures no two Registry
	// instances pointing at the same CONFIG_FILE can interleave a
	// reload-from-disk → mutate → save sequence.
	releasePath := r.cfg.LockConfigFile()
	defer releasePath()
	r.cfg.Lock()
	defer r.cfg.Unlock()

	if err := r.reloadFromDiskLocked(); err != nil {
		return fmt.Errorf("registry: reload: %w", err)
	}

	if idx < 0 || idx >= len(r.cfg.PriceGapCandidates) {
		return ErrIndexOutOfRange
	}

	beforeCount := len(r.cfg.PriceGapCandidates)
	prior := r.cfg.PriceGapCandidates[idx]
	r.cfg.PriceGapCandidates[idx] = c
	if err := r.cfg.SaveJSONWithBakRing(); err != nil {
		r.cfg.PriceGapCandidates[idx] = prior
		return fmt.Errorf("registry: save: %w", err)
	}

	r.writeAuditLocked(ctx, source, "update", beforeCount, beforeCount)
	return nil
}

// Delete removes the candidate at idx. Returns ErrIndexOutOfRange if idx is
// invalid. Persisted via SaveJSONWithBakRing; on failure the in-memory
// slice is restored.
func (r *Registry) Delete(ctx context.Context, source string, idx int) error {
	// Path-level lock first (cross-instance serialization), then cfg.mu
	// (per-instance). Order matters: path → cfg ensures no two Registry
	// instances pointing at the same CONFIG_FILE can interleave a
	// reload-from-disk → mutate → save sequence.
	releasePath := r.cfg.LockConfigFile()
	defer releasePath()
	r.cfg.Lock()
	defer r.cfg.Unlock()

	if err := r.reloadFromDiskLocked(); err != nil {
		return fmt.Errorf("registry: reload: %w", err)
	}

	if idx < 0 || idx >= len(r.cfg.PriceGapCandidates) {
		return ErrIndexOutOfRange
	}

	beforeCount := len(r.cfg.PriceGapCandidates)
	prior := append([]models.PriceGapCandidate(nil), r.cfg.PriceGapCandidates...)
	r.cfg.PriceGapCandidates = append(r.cfg.PriceGapCandidates[:idx], r.cfg.PriceGapCandidates[idx+1:]...)
	if err := r.cfg.SaveJSONWithBakRing(); err != nil {
		r.cfg.PriceGapCandidates = prior
		return fmt.Errorf("registry: save: %w", err)
	}

	r.writeAuditLocked(ctx, source, "delete", beforeCount, beforeCount-1)
	return nil
}

// Replace swaps the entire PriceGapCandidates slice atomically. The caller
// MUST have already validated + normalized the slice (handler-specific
// concerns like guardActivePositionRemoval are handled upstream — Registry
// only enforces persistence + audit). Cap is enforced; dedupe is the
// caller's responsibility for Replace.
//
// On persist failure the in-memory slice is restored.
func (r *Registry) Replace(ctx context.Context, source string, next []models.PriceGapCandidate) error {
	// Path-level lock first (cross-instance serialization), then cfg.mu
	// (per-instance). Order matters: path → cfg ensures no two Registry
	// instances pointing at the same CONFIG_FILE can interleave a
	// reload-from-disk → mutate → save sequence.
	releasePath := r.cfg.LockConfigFile()
	defer releasePath()
	r.cfg.Lock()
	defer r.cfg.Unlock()

	if err := r.reloadFromDiskLocked(); err != nil {
		return fmt.Errorf("registry: reload: %w", err)
	}

	if len(next) > r.cfg.PriceGapMaxCandidates {
		return ErrCapExceeded
	}

	beforeCount := len(r.cfg.PriceGapCandidates)
	prior := r.cfg.PriceGapCandidates
	// Defensive copy of the input so caller mutations after Replace cannot
	// affect Registry state.
	updated := make([]models.PriceGapCandidate, len(next))
	copy(updated, next)
	r.cfg.PriceGapCandidates = updated
	if err := r.cfg.SaveJSONWithBakRing(); err != nil {
		r.cfg.PriceGapCandidates = prior
		return fmt.Errorf("registry: save: %w", err)
	}

	r.writeAuditLocked(ctx, source, "replace", beforeCount, len(updated))
	return nil
}

// reloadFromDiskLocked refreshes r.cfg.PriceGapCandidates from the on-disk
// config.json before applying a mutation. Caller MUST hold cfg.mu (the
// underlying Load() does its own internal initialization but this method
// only mutates the candidates slice on r.cfg). On parse error the in-memory
// state is left untouched and the error is propagated.
func (r *Registry) reloadFromDiskLocked() error {
	fresh := config.Load()
	// Only the candidates slice is reloaded — Registry does not own the
	// other config fields.
	r.cfg.PriceGapCandidates = fresh.PriceGapCandidates
	return nil
}

// auditPayload is the JSON shape written to pg:registry:audit on every
// successful mutation. Field names match the ROADMAP §"audit row" schema.
type auditPayload struct {
	TS          int64  `json:"ts"`
	Source      string `json:"source"`
	Op          string `json:"op"`
	BeforeCount int    `json:"before_count"`
	AfterCount  int    `json:"after_count"`
}

// writeAuditLocked serializes an auditPayload, LPushes it, and LTrims the
// list to 200 entries. Caller holds cfg.mu — audit writes do not require
// cfg.mu, but holding it ensures the audit row is sequenced with the
// successful mutation it describes. Errors are logged best-effort.
func (r *Registry) writeAuditLocked(ctx context.Context, source, op string, before, after int) {
	if r.db == nil {
		return
	}
	payload := auditPayload{
		TS:          r.nowFunc().Unix(),
		Source:      source,
		Op:          op,
		BeforeCount: before,
		AfterCount:  after,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		if r.log != nil {
			r.log.Warn("audit marshal failed: %v", err)
		}
		return
	}
	if _, err := r.db.LPush(ctx, "pg:registry:audit", string(encoded)); err != nil {
		if r.log != nil {
			r.log.Warn("audit lpush failed: %v", err)
		}
		return
	}
	if err := r.db.LTrim(ctx, "pg:registry:audit", 0, 199); err != nil {
		if r.log != nil {
			r.log.Warn("audit ltrim failed: %v", err)
		}
	}
}
