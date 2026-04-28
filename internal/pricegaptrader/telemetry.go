// Package pricegaptrader — telemetry.go: pg:scan:* Redis writers + WS
// broadcaster for the auto-discovery scanner (Plan 11-05 / PG-DISC-03).
//
// Plan 04 declared TelemetryWriter (4 methods) in scanner.go. This file
// implements that interface backed by Redis DB 2 + the dashboard WS hub.
//
// Redis schema (Plan 11-RESEARCH §"Redis Schema"):
//
//	pg:scan:cycles            LIST   RPush + LTrim 1000  — per-cycle envelope
//	pg:scan:scores:{symbol}   ZSET   7d EXPIRE           — accepted-record score history
//	pg:scan:rejections:{date} HASH   30d EXPIRE          — per-day reason counters
//	pg:scan:metrics           HASH                       — last cycle counters + cycle_failed
//	pg:scan:enabled           STRING                     — "0"/"1" master flag mirror
//
// WS broadcast policy (Plan 05 D-17):
//
//	pg_scan_cycle    — every WriteCycle (no throttle, low rate)
//	pg_scan_metrics  — 5s throttle from WriteCycle
//	pg_scan_score    — 1s debounce; coalesced to one event per (symbol)
//
// Redis errors are logged but non-fatal — telemetry is observability-only and
// must never block the scan loop. The cycle envelope is in-memory until the
// next successful WriteCycle replaces it; operators see staleness via
// last_run_at on the dashboard.
//
// Module boundary (CLAUDE.md): this file imports only internal/database,
// internal/config, internal/models (via scanner.go's existing surface), and
// pkg/utils. No imports of internal/engine or internal/spotengine.
package pricegaptrader

import (
	"context"
	"encoding/json"
	"strconv"
	"sync"
	"time"

	"arb/internal/config"
	"arb/internal/database"
	"arb/pkg/utils"

	"github.com/redis/go-redis/v9"
)

// DiscoveryBroadcaster is the narrow WS broadcast surface the Telemetry
// writer requires from the api Server. The api package's *Server satisfies
// it via BroadcastPriceGapDiscoveryEvent.
//
// Kept narrow (one method) to honor module boundaries — Telemetry must not
// reach into api package types directly.
type DiscoveryBroadcaster interface {
	BroadcastPriceGapDiscoveryEvent(eventType string, payload interface{})
}

// Redis key constants — exposed as `pg:scan:` prefix so an operator's
// `redis-cli KEYS pg:scan:*` lists every scanner-owned key.
const (
	keyCycles           = "pg:scan:cycles"
	keyMetrics          = "pg:scan:metrics"
	keyEnabled          = "pg:scan:enabled"
	keyScoresPrefix     = "pg:scan:scores:"
	keyRejectionsPrefix = "pg:scan:rejections:"
)

// TTLs (Plan 11-RESEARCH §"Redis Schema").
const (
	ttlScores     = 7 * 24 * time.Hour
	ttlRejections = 30 * 24 * time.Hour
)

// Default WS throttle / debounce windows. Tests override.
const (
	defaultMetricsThrottle = 5 * time.Second
	defaultScoreDebounce   = 1 * time.Second
)

// Telemetry implements TelemetryWriter against Redis DB 2 + the dashboard WS
// hub. Construction is via NewTelemetry; cmd/main.go wires exactly one
// instance per process and injects it into both the Scanner and the api
// Server (the Server uses GetState/GetScores for REST endpoints).
type Telemetry struct {
	db      *database.Client
	broker  DiscoveryBroadcaster
	cfg     *config.Config
	log     *utils.Logger
	nowFunc func() time.Time

	mu               sync.Mutex
	lastMetricsBroad time.Time
	pendingScores    map[string]CycleRecord // debounce buffer keyed by symbol
	debounceTimer    *time.Timer
	metricsThrottle  time.Duration
	debounceWindow   time.Duration
}

// NewTelemetry constructs a Telemetry writer. broker may be nil (NoopWriter
// equivalent — no events broadcast); cfg + db must be non-nil.
func NewTelemetry(db *database.Client, broker DiscoveryBroadcaster, cfg *config.Config, log *utils.Logger) *Telemetry {
	return &Telemetry{
		db:              db,
		broker:          broker,
		cfg:             cfg,
		log:             log,
		nowFunc:         time.Now,
		pendingScores:   map[string]CycleRecord{},
		metricsThrottle: defaultMetricsThrottle,
		debounceWindow:  defaultScoreDebounce,
	}
}

// SubScores is the per-record sub-score envelope persisted into the
// pg:scan:scores:{symbol} ZSET. Members are JSON-encoded ScorePoint.
type SubScores struct {
	SpreadBps       float64 `json:"spread_bps"`
	DepthScore      float64 `json:"depth_score"`
	FreshnessAgeS   int     `json:"freshness_age_s"`
	FundingBpsH     float64 `json:"funding_bps_h"`
	PersistenceBars int     `json:"persistence_bars"`
}

// ScorePoint is one accepted-record snapshot stored as a ZSET member with the
// integer score as the ZSET score field. Plan 06 UI plots ts → score line.
type ScorePoint struct {
	Ts        int64     `json:"ts"`
	Score     int       `json:"score"`
	SubScores SubScores `json:"sub_scores"`
}

// StateResponse is the GET /api/pg/discovery/state envelope (Plan 05 + 06
// UI-SPEC). Maps directly to the dashboard widget data contract.
type StateResponse struct {
	Enabled        bool            `json:"enabled"`
	LastRunAt      int64           `json:"last_run_at"`
	NextRunIn      int             `json:"next_run_in"`
	CandidatesSeen int             `json:"candidates_seen"`
	Accepted       int             `json:"accepted"`
	Rejected       int             `json:"rejected"`
	Errors         int             `json:"errors"`
	DurationMs     int64           `json:"duration_ms"`
	WhyRejected    map[string]int  `json:"why_rejected"`
	ScoreSnapshot  []SnapshotEntry `json:"score_snapshot"`
	CycleFailed    bool            `json:"cycle_failed"`
}

// SnapshotEntry is a single accepted-record row in the last-cycle score
// snapshot (per UI-SPEC §"Top score table" column ordering).
type SnapshotEntry struct {
	Symbol      string    `json:"symbol"`
	LongExch    string    `json:"long_exch"`
	ShortExch   string    `json:"short_exch"`
	Score       int       `json:"score"`
	SubScores   SubScores `json:"sub_scores"`
	GatesPassed []string  `json:"gates_passed"`
	GatesFailed []string  `json:"gates_failed"`
}

// ScoresResponse is the GET /api/pg/discovery/scores/{symbol} envelope.
type ScoresResponse struct {
	Symbol        string        `json:"symbol"`
	Points        []ScorePoint  `json:"points"`
	ThresholdBand ThresholdBand `json:"threshold_band"`
}

// ThresholdBand documents the score thresholds the UI overlays on the
// per-symbol score chart. AutoPromote is reserved for Plan 12; today the
// scanner is read-only.
type ThresholdBand struct {
	AutoPromote int `json:"auto_promote"`
}

// ---- TelemetryWriter implementation -----------------------------------------

// WriteCycle persists the cycle envelope + per-symbol scores + per-day
// rejection counters, then dispatches WS events with throttle/debounce.
//
// Redis errors are logged but non-fatal: telemetry is observability and must
// never block the scan loop. Each Redis call uses its own bounded context to
// avoid stalls in the worst case (DB unreachable).
func (t *Telemetry) WriteCycle(ctx context.Context, summary CycleSummary) error {
	if t.db == nil {
		return nil
	}
	rdb := t.db.Redis()

	// 1. Append cycle envelope to LIST + cap at 1000.
	if body, err := json.Marshal(summary); err == nil {
		c, cancel := boundCtx(ctx)
		if err := rdb.RPush(c, keyCycles, body).Err(); err != nil {
			t.warnf("pg:scan:cycles rpush: %v", err)
		}
		_ = rdb.LTrim(c, keyCycles, -1000, -1).Err()
		cancel()
	}

	// 2. Update pg:scan:metrics HASH.
	nextRunIn := t.cfg.PriceGapDiscoveryIntervalSec
	if nextRunIn <= 0 {
		nextRunIn = 300
	}
	metrics := map[string]interface{}{
		"last_run_at":     summary.CompletedAt,
		"candidates_seen": summary.CandidatesSeen,
		"accepted":        summary.Accepted,
		"rejected":        summary.Rejected,
		"errors":          summary.Errors,
		"duration_ms":     summary.DurationMs,
		"next_run_in":     nextRunIn,
		"cycle_failed":    "0",
	}
	c1, cancel1 := boundCtx(ctx)
	if err := rdb.HSet(c1, keyMetrics, metrics).Err(); err != nil {
		t.warnf("pg:scan:metrics hset: %v", err)
	}
	cancel1()

	// 3. Per-symbol score ZSET — only accepted records contribute.
	for _, rec := range summary.Records {
		if rec.WhyRejected != ReasonAccepted {
			continue
		}
		point := ScorePoint{
			Ts:    rec.Timestamp,
			Score: rec.Score,
			SubScores: SubScores{
				SpreadBps:       rec.SpreadBps,
				DepthScore:      rec.DepthScore,
				FreshnessAgeS:   rec.FreshnessAgeSec,
				FundingBpsH:     rec.FundingBpsPerHr,
				PersistenceBars: rec.PersistenceBars,
			},
		}
		body, err := json.Marshal(point)
		if err != nil {
			continue
		}
		key := keyScoresPrefix + rec.Symbol
		c2, cancel2 := boundCtx(ctx)
		_ = rdb.ZAdd(c2, key, redis.Z{Score: float64(rec.Score), Member: string(body)}).Err()
		_ = rdb.Expire(c2, key, ttlScores).Err()
		cancel2()
		t.queueScoreBroadcast(rec)
	}

	// 4. Per-day rejection HASH.
	if summary.CompletedAt > 0 && len(summary.WhyRejected) > 0 {
		date := time.Unix(summary.CompletedAt, 0).UTC().Format("2006-01-02")
		key := keyRejectionsPrefix + date
		c3, cancel3 := boundCtx(ctx)
		for reason, count := range summary.WhyRejected {
			if count <= 0 {
				continue
			}
			_ = rdb.HIncrBy(c3, key, string(reason), int64(count)).Err()
		}
		_ = rdb.Expire(c3, key, ttlRejections).Err()
		cancel3()
	}

	// 5. Broadcast cycle event (no throttle).
	t.broadcast("pg_scan_cycle", summary)

	// 6. Throttled metrics broadcast.
	t.maybeBroadcastMetrics(metrics)
	return nil
}

// WriteEnabledFlag stamps the master switch state to a Redis STRING.
func (t *Telemetry) WriteEnabledFlag(ctx context.Context, enabled bool) error {
	if t.db == nil {
		return nil
	}
	flag := "0"
	if enabled {
		flag = "1"
	}
	c, cancel := boundCtx(ctx)
	defer cancel()
	if err := t.db.Redis().Set(c, keyEnabled, flag, 0).Err(); err != nil {
		t.warnf("pg:scan:enabled set: %v", err)
		return err
	}
	return nil
}

// WriteSymbolNoCrossPair increments the singleton-skip counter (D-08).
func (t *Telemetry) WriteSymbolNoCrossPair(ctx context.Context) error {
	if t.db == nil {
		return nil
	}
	c, cancel := boundCtx(ctx)
	defer cancel()
	return t.db.Redis().HIncrBy(c, keyMetrics, "symbol_no_cross_pair", 1).Err()
}

// WriteCycleFailed flags the most-recent cycle as aborted (consec-error
// budget tripped). The next successful WriteCycle clears it back to "0".
func (t *Telemetry) WriteCycleFailed(ctx context.Context) error {
	if t.db == nil {
		return nil
	}
	c, cancel := boundCtx(ctx)
	defer cancel()
	return t.db.Redis().HSet(c, keyMetrics, map[string]interface{}{"cycle_failed": "1"}).Err()
}

// ---- read helpers (Plan 05 REST handlers consume these) ---------------------

// GetState reads pg:scan:metrics + pg:scan:enabled + last cycle envelope and
// composes a StateResponse for the GET /api/pg/discovery/state endpoint.
func (t *Telemetry) GetState(ctx context.Context) (StateResponse, error) {
	out := StateResponse{
		WhyRejected:   map[string]int{},
		ScoreSnapshot: []SnapshotEntry{},
	}
	if t.db == nil {
		return out, nil
	}
	rdb := t.db.Redis()

	// enabled flag
	c1, cancel1 := boundCtx(ctx)
	if v, err := rdb.Get(c1, keyEnabled).Result(); err == nil {
		out.Enabled = v == "1"
	}
	cancel1()

	// metrics hash
	c2, cancel2 := boundCtx(ctx)
	if m, err := rdb.HGetAll(c2, keyMetrics).Result(); err == nil {
		out.LastRunAt = atoi64(m["last_run_at"])
		out.NextRunIn = atoi(m["next_run_in"])
		out.CandidatesSeen = atoi(m["candidates_seen"])
		out.Accepted = atoi(m["accepted"])
		out.Rejected = atoi(m["rejected"])
		out.Errors = atoi(m["errors"])
		out.DurationMs = atoi64(m["duration_ms"])
		out.CycleFailed = m["cycle_failed"] == "1"
	}
	cancel2()

	// last cycle for ScoreSnapshot + WhyRejected
	c3, cancel3 := boundCtx(ctx)
	if items, err := rdb.LRange(c3, keyCycles, -1, -1).Result(); err == nil && len(items) > 0 {
		var last CycleSummary
		if err := json.Unmarshal([]byte(items[0]), &last); err == nil {
			for r, c := range last.WhyRejected {
				out.WhyRejected[string(r)] = c
			}
			for _, rec := range last.Records {
				if rec.WhyRejected != ReasonAccepted {
					continue
				}
				out.ScoreSnapshot = append(out.ScoreSnapshot, SnapshotEntry{
					Symbol:    rec.Symbol,
					LongExch:  rec.LongExch,
					ShortExch: rec.ShortExch,
					Score:     rec.Score,
					SubScores: SubScores{
						SpreadBps:       rec.SpreadBps,
						DepthScore:      rec.DepthScore,
						FreshnessAgeS:   rec.FreshnessAgeSec,
						FundingBpsH:     rec.FundingBpsPerHr,
						PersistenceBars: rec.PersistenceBars,
					},
					GatesPassed: rec.GatesPassed,
					GatesFailed: rec.GatesFailed,
				})
			}
		}
	}
	cancel3()
	return out, nil
}

// GetScores reads pg:scan:scores:{symbol} and packages it for the
// /api/pg/discovery/scores/{symbol} endpoint. Empty results are valid (the
// UI shows "no history yet"); no symbol-existence check happens here.
func (t *Telemetry) GetScores(ctx context.Context, symbol string) (ScoresResponse, error) {
	out := ScoresResponse{
		Symbol: symbol,
		Points: []ScorePoint{},
		ThresholdBand: ThresholdBand{
			AutoPromote: t.cfg.PriceGapAutoPromoteScore,
		},
	}
	if t.db == nil {
		return out, nil
	}
	c, cancel := boundCtx(ctx)
	defer cancel()
	key := keyScoresPrefix + symbol
	members, err := t.db.Redis().ZRange(c, key, 0, -1).Result()
	if err != nil {
		return out, nil // empty is valid
	}
	for _, m := range members {
		var p ScorePoint
		if err := json.Unmarshal([]byte(m), &p); err == nil {
			out.Points = append(out.Points, p)
		}
	}
	// Sort ascending by ts so dashboards plot left-to-right.
	sortPointsByTs(out.Points)
	return out, nil
}

// ---- internal helpers -------------------------------------------------------

// boundCtx returns a 3-second-bounded ctx so a hung Redis can never freeze
// the scanLoop. Reuses the parent ctx's deadline if it's already shorter.
func boundCtx(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, 3*time.Second)
}

func (t *Telemetry) maybeBroadcastMetrics(metrics map[string]interface{}) {
	t.mu.Lock()
	now := t.nowFunc()
	if now.Sub(t.lastMetricsBroad) < t.metricsThrottle {
		t.mu.Unlock()
		return
	}
	t.lastMetricsBroad = now
	t.mu.Unlock()
	t.broadcast("pg_scan_metrics", metrics)
}

func (t *Telemetry) queueScoreBroadcast(rec CycleRecord) {
	t.mu.Lock()
	t.pendingScores[rec.Symbol] = rec
	if t.debounceTimer == nil {
		t.debounceTimer = time.AfterFunc(t.debounceWindow, t.flushPendingScores)
	}
	t.mu.Unlock()
}

func (t *Telemetry) flushPendingScores() {
	t.mu.Lock()
	pending := t.pendingScores
	t.pendingScores = map[string]CycleRecord{}
	t.debounceTimer = nil
	t.mu.Unlock()
	for sym, rec := range pending {
		t.broadcast("pg_scan_score", map[string]interface{}{
			"symbol": sym,
			"score":  rec.Score,
			"ts":     rec.Timestamp,
			"sub_scores": SubScores{
				SpreadBps:       rec.SpreadBps,
				DepthScore:      rec.DepthScore,
				FreshnessAgeS:   rec.FreshnessAgeSec,
				FundingBpsH:     rec.FundingBpsPerHr,
				PersistenceBars: rec.PersistenceBars,
			},
		})
	}
}

func (t *Telemetry) broadcast(eventType string, payload interface{}) {
	if t.broker == nil {
		return
	}
	t.broker.BroadcastPriceGapDiscoveryEvent(eventType, payload)
}

func (t *Telemetry) warnf(format string, args ...interface{}) {
	if t.log != nil {
		t.log.Warn(format, args...)
	}
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

func atoi64(s string) int64 {
	n, _ := strconv.ParseInt(s, 10, 64)
	return n
}

// sortPointsByTs orders ScorePoint slice ascending by Ts (insertion sort —
// expected count is bounded by ZSET size, typically <100 per symbol per day).
func sortPointsByTs(p []ScorePoint) {
	for i := 1; i < len(p); i++ {
		for j := i; j > 0 && p[j-1].Ts > p[j].Ts; j-- {
			p[j-1], p[j] = p[j], p[j-1]
		}
	}
}
