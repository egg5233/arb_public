package models

import "time"

// RampState — exactly 5 fields per PG-LIVE-01 hard contract.
//
// CurrentStage: 1, 2, or 3 — drives the per-leg sizing cap (100/500/1000 USDT
// at v2.2 defaults). Persisted as decimal string in Redis HASH `pg:ramp:state`.
//
// CleanDayCounter: number of consecutive UTC clean days observed by the
// reconciler. A "clean day" is one with no anomaly flags + no realized loss.
// Promotion to next stage requires CleanDayCounter >= cfg.PriceGapCleanDaysToPromote.
//
// LastEvalTs: timestamp of the most recent ramp evaluation (daemon tick or
// pg-admin force action). Used by the daemon to detect "no activity" days
// where the counter is held (not incremented, not reset).
//
// LastLossDayTs: timestamp of the most recent loss-day. Used to compute the
// asymmetric ratchet: a loss day immediately demotes one stage and resets
// the counter; a clean day only increments.
//
// DemoteCount: cumulative auto-demote count. Operator visibility — surfaces
// instability in the candidate universe even when stage rebounds.
type RampState struct {
	CurrentStage    int       `json:"current_stage"`
	CleanDayCounter int       `json:"clean_day_counter"`
	LastEvalTs      time.Time `json:"last_eval_ts"`
	LastLossDayTs   time.Time `json:"last_loss_day_ts"`
	DemoteCount     int       `json:"demote_count"`
}

// RampEvent — emitted on every state change (daemon eval, force-promote,
// force-demote, reset). Persisted as JSON-encoded entries in Redis LIST
// `pg:ramp:events`, capped at 500 entries (RPUSH + LTRIM).
//
// Action ∈ {"evaluate", "force_promote", "force_demote", "reset"}.
// Operator ∈ {"daemon", "pg-admin", "dashboard"} — identifies the trigger.
type RampEvent struct {
	Action     string    `json:"action"`
	PriorStage int       `json:"prior_stage"`
	NewStage   int       `json:"new_stage"`
	Operator   string    `json:"operator"`
	Timestamp  time.Time `json:"timestamp"`
	Reason     string    `json:"reason"`
}
