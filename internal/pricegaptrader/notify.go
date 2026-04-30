// Package pricegaptrader — notify.go defines the Broadcaster + PriceGapNotifier
// DI seams for Phase 9 dashboard WebSocket push and Telegram alerts. The
// interfaces are owned by this package to preserve the D-02 module boundary
// (this package depends on nothing above it in the dependency graph).
// *api.Server implements Broadcaster (Plan 02); *notify.TelegramNotifier
// implements PriceGapNotifier (Plan 04). Plan 06 wires both into tracker
// at cmd/main.go startup.
package pricegaptrader

import (
	"time"

	"arb/internal/models"
)

// PriceGapEvent is the envelope for lifecycle events pushed to the dashboard.
// Type is one of: "entry" | "exit" | "auto_disable".
type PriceGapEvent struct {
	Type     string                   `json:"type"`
	Position *models.PriceGapPosition `json:"position,omitempty"`
	Symbol   string                   `json:"symbol,omitempty"`
	Reason   string                   `json:"reason,omitempty"`
}

// PriceGapCandidateUpdate signals a change in candidate disable state
// (PG-RISK-03 exec-quality auto-disable, operator reversal via pg-admin).
type PriceGapCandidateUpdate struct {
	Symbol     string `json:"symbol"`
	Disabled   bool   `json:"disabled"`
	Reason     string `json:"reason,omitempty"`
	DisabledAt int64  `json:"disabled_at,omitempty"` // unix seconds; 0 when disabled=false
}

// Broadcaster is the outbound-events contract satisfied by the dashboard
// WS hub (see Plan 06). Tests and CLI paths receive NoopBroadcaster{} to
// avoid nil derefs.
type Broadcaster interface {
	BroadcastPriceGapPositions(positions []*models.PriceGapPosition)
	BroadcastPriceGapEvent(evt PriceGapEvent)
	BroadcastPriceGapCandidateUpdate(update PriceGapCandidateUpdate)
}

// NoopBroadcaster is the zero-behavior default used when no WS hub is wired.
// Safe to call on unit-test Trackers and in pg-admin CLI paths.
type NoopBroadcaster struct{}

// BroadcastPriceGapPositions is a no-op.
func (NoopBroadcaster) BroadcastPriceGapPositions([]*models.PriceGapPosition) {}

// BroadcastPriceGapEvent is a no-op.
func (NoopBroadcaster) BroadcastPriceGapEvent(PriceGapEvent) {}

// BroadcastPriceGapCandidateUpdate is a no-op.
func (NoopBroadcaster) BroadcastPriceGapCandidateUpdate(PriceGapCandidateUpdate) {}

// PriceGapNotifier is the outbound-alerts contract satisfied by the Telegram
// notifier (*notify.TelegramNotifier — see internal/notify/telegram.go for
// the implementation shipped in Plan 04). Tracker holds an instance of this
// interface so the tracker package does NOT import internal/notify (avoids
// circular imports and preserves the D-02 module boundary).
//
// All methods MUST be nil-receiver-safe on the implementation side; Plan 04
// guarantees this for *notify.TelegramNotifier (nil-receiver returns early,
// nil-pos guarded on Entry/Exit).
//
// Phase 14 (Plan 14-02 + Plan 14-04) extends the interface with two reconcile
// methods. NotifyPriceGapDailyDigest is the routine UTC-00:30 summary;
// NotifyPriceGapReconcileFailure is the critical-path Telegram dispatch
// triggered by the 3-retry loop in Reconciler.RunForDate. Plan 14-04 ships the
// real Telegram impl; until then NoopNotifier provides safe nil-receiver
// behavior.
type PriceGapNotifier interface {
	NotifyPriceGapEntry(pos *models.PriceGapPosition)
	NotifyPriceGapExit(pos *models.PriceGapPosition, reason string, pnl float64, duration time.Duration)
	NotifyPriceGapRiskBlock(symbol, gate, detail string)
	NotifyPriceGapDailyDigest(date string, record DailyReconcileRecord, ramp models.RampState)
	NotifyPriceGapReconcileFailure(date string, err error)
	// Phase 14 Plan 14-03 ramp surfaces — emitted by RampController.Eval (loss-day demote)
	// and pg-admin force-promote/force-demote/reset.
	NotifyPriceGapRampDemote(prior, next int, reason string)
	NotifyPriceGapRampForceOp(action string, prior, next int, operator, reason string)
}

// NoopNotifier is the zero-behavior default used when no Telegram notifier is
// wired (unit tests, CLI tools, development environments without Telegram
// configured). Safe in all contexts; prevents nil-deref on t.notifier.* calls.
type NoopNotifier struct{}

// NotifyPriceGapEntry is a no-op.
func (NoopNotifier) NotifyPriceGapEntry(*models.PriceGapPosition) {}

// NotifyPriceGapExit is a no-op.
func (NoopNotifier) NotifyPriceGapExit(*models.PriceGapPosition, string, float64, time.Duration) {}

// NotifyPriceGapRiskBlock is a no-op.
func (NoopNotifier) NotifyPriceGapRiskBlock(string, string, string) {}

// NotifyPriceGapDailyDigest is a no-op (Phase 14 Plan 14-04 ships Telegram impl).
func (NoopNotifier) NotifyPriceGapDailyDigest(string, DailyReconcileRecord, models.RampState) {}

// NotifyPriceGapReconcileFailure is a no-op (Phase 14 Plan 14-04 ships Telegram impl).
func (NoopNotifier) NotifyPriceGapReconcileFailure(string, error) {}

// NotifyPriceGapRampDemote is a no-op (Phase 14 Plan 14-04 ships Telegram impl).
func (NoopNotifier) NotifyPriceGapRampDemote(int, int, string) {}

// NotifyPriceGapRampForceOp is a no-op (Phase 14 Plan 14-04 ships Telegram impl).
func (NoopNotifier) NotifyPriceGapRampForceOp(string, int, int, string, string) {}
