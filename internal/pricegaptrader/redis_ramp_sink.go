// Package pricegaptrader — redis_ramp_sink.go is Phase 14 Plan 14-03's
// concrete RampEvent persistence + WS broadcast sink. Mirrors the Phase 12
// RedisWSPromoteSink shape (D-15 boundary preserved).
//
// Emit fans every RampEvent out to two surfaces:
//
//  1. Redis LIST pg:ramp:events (RPUSH + LTRIM 500) via the underlying
//     RampStateStore.AppendPriceGapRampEvent — the trim is enforced on the
//     storage side (database.AppendPriceGapRampEvent applies LTRIM).
//  2. WS hub Broadcast("pg_ramp_event", payload) — live dashboard push,
//     consumed by future dashboard timeline component (Plan 14-04 wires;
//     Plan 14-05 surfaces).
//
// Module boundary (D-15): this file imports only encoding/json,
// arb/internal/models, arb/pkg/utils. No internal/api — instead we re-use
// the WSBroadcaster narrow interface from promote_event_sink.go (declared
// at package scope) and let *api.Hub satisfy it via duck typing at the
// wiring site (Plan 14-04 cmd/main.go).
package pricegaptrader

import (
	"encoding/json"

	"arb/internal/models"
	"arb/pkg/utils"
)

// wsEventRampEvent is the WS event-type tag broadcast for every RampEvent.
const wsEventRampEvent = "pg_ramp_event"

// RedisWSRampSink emits RampEvents to Redis (durable LIST via RampStateStore)
// and the dashboard WS hub (live push). hub may be nil — Emit then only
// writes Redis (used by tests + by deployments without a UI).
type RedisWSRampSink struct {
	store     RampStateStore
	broadcast WSBroadcaster
	log       *utils.Logger
}

// NewRedisWSRampSink constructs the sink. store must be non-nil; broadcast
// may be nil (Redis-only deployments + unit tests).
func NewRedisWSRampSink(store RampStateStore, broadcast WSBroadcaster, log *utils.Logger) *RedisWSRampSink {
	if log == nil {
		log = utils.NewLogger("pg-ramp-sink")
	}
	return &RedisWSRampSink{store: store, broadcast: broadcast, log: log}
}

// Emit appends the event to the Redis list (best-effort) and broadcasts it
// over the WS hub. Errors are LOGGED, never returned — the controller
// emits ramp events as a side-channel; an unreliable Redis or unreliable
// hub must NEVER block ramp-state changes.
func (s *RedisWSRampSink) Emit(ev models.RampEvent) {
	if s.store != nil {
		if err := s.store.AppendPriceGapRampEvent(ev); err != nil {
			s.log.Warn("pg-ramp-sink: append failed: %v", err)
		}
	}
	if s.broadcast == nil {
		return
	}
	// Marshal once for both sinks (LIST persistence is handled by the store
	// side; broadcast payload is the raw JSON for symmetry with
	// RedisWSPromoteSink).
	payload, err := json.Marshal(ev)
	if err != nil {
		s.log.Warn("pg-ramp-sink: marshal failed: %v", err)
		return
	}
	s.broadcast.Broadcast(wsEventRampEvent, payload)
}
