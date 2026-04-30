// Package pricegaptrader — Phase 12 Plan 02 RedisWSPromoteSink (D-10, D-11).
//
// Concrete implementation of PromoteEventSink (declared in promotion.go).
// Emits each PromoteEvent to two surfaces:
//
//  1. Redis LIST pg:promote:events (RPush + LTrim 1000) — REST seed source
//     consumed by GET /api/pg/discovery/promote-events (Plan 12-02 Task 2).
//  2. WS hub Broadcast("pg_promote_event", payload) — live dashboard push,
//     consumed by Plan 12-04 frontend timeline component.
//
// Mirrors the pg:scan:cycles pattern from telemetry.go (RPush + LTrim 1000).
// Redis errors propagate up to the controller, which logs best-effort and
// continues the cycle (Pitfall 6 — never abort the scanner cycle on a
// telemetry/sink hiccup). The WS broadcast is best-effort fire-and-forget.
//
// Module boundary (D-15): this file imports only context, encoding/json,
// arb/internal/database. No internal/api import — instead we declare the
// narrow WSBroadcaster interface here and let the api Server's *Hub satisfy
// it via duck-typing at the wiring site (Plan 12-03 cmd/main.go).
package pricegaptrader

import (
	"context"
	"encoding/json"

	"arb/internal/database"
)

// WSBroadcaster is the narrow surface RedisWSPromoteSink requires from the
// dashboard WS hub. Satisfied by *api.Hub via its Broadcast(eventType,
// payload) method. Defined here so this package does not import internal/api
// (D-15 module boundary).
type WSBroadcaster interface {
	Broadcast(eventType string, payload interface{})
}

// Redis key + WS event constants — exposed at package scope so:
//   - operators can `redis-cli LRANGE pg:promote:events 0 -1` without
//     guessing the key name,
//   - the REST handler in internal/api can re-use keyPromoteEvents via
//     copy-by-string-literal (the api package cannot import this constant
//     without violating its own dependency direction; the literal is asserted
//     identical by the handler test).
const (
	keyPromoteEvents    = "pg:promote:events"
	promoteEventsMaxLen = 1000
	wsEventPromoteEvent = "pg_promote_event"
)

// RedisWSPromoteSink fans an emitted PromoteEvent out to Redis (durable
// LIST) and the dashboard WS hub (live push). hub may be nil — Emit then
// only writes Redis (used by tests + by deployments without a UI).
type RedisWSPromoteSink struct {
	db  *database.Client
	hub WSBroadcaster
}

// NewRedisWSPromoteSink constructs the sink. db must be non-nil; hub may be
// nil (some unit tests + Redis-only deployments).
func NewRedisWSPromoteSink(db *database.Client, hub WSBroadcaster) *RedisWSPromoteSink {
	return &RedisWSPromoteSink{db: db, hub: hub}
}

// Emit RPushes a JSON-serialized PromoteEvent to pg:promote:events, trims
// the LIST to its last 1000 entries (D-10 bound), and broadcasts the same
// event over the WS hub.
//
// On a Redis error during RPush, returns the error so the controller can
// log it. LTrim and Broadcast errors are swallowed (best-effort) — the
// event is already durable in Redis at that point and a dropped WS frame
// is recoverable from the REST seed at the next dashboard reload.
func (s *RedisWSPromoteSink) Emit(ctx context.Context, ev PromoteEvent) error {
	raw, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	rdb := s.db.Redis()
	if err := rdb.RPush(ctx, keyPromoteEvents, raw).Err(); err != nil {
		return err
	}
	// LTrim bound to last 1000 (D-10). Best-effort.
	_ = rdb.LTrim(ctx, keyPromoteEvents, -promoteEventsMaxLen, -1).Err()
	if s.hub != nil {
		s.hub.Broadcast(wsEventPromoteEvent, ev)
	}
	return nil
}
