package analytics

import (
	"math"
	"sort"
	"time"

	"arb/internal/models"
	"arb/pkg/utils"
)

// SnapshotWriter receives position-close events on a buffered channel
// and writes PnL snapshots to the SQLite store in a background goroutine.
type SnapshotWriter struct {
	store  *Store
	ch     chan snapshotEvent
	log    *utils.Logger
	stopCh chan struct{}
}

type snapshotEvent struct {
	Strategy     string
	Exchange     string
	PnL          float64
	IsWin        bool
	FundingTotal float64
	FeesTotal    float64
}

// NewSnapshotWriter creates a new SnapshotWriter backed by the given store.
func NewSnapshotWriter(store *Store) *SnapshotWriter {
	return &SnapshotWriter{
		store:  store,
		ch:     make(chan snapshotEvent, 100),
		log:    utils.NewLogger("analytics"),
		stopCh: make(chan struct{}),
	}
}

// Start begins the background goroutine that reads from the channel
// and writes snapshots to the SQLite store.
func (sw *SnapshotWriter) Start() {
	go sw.run()
}

// Stop signals the background goroutine to stop.
func (sw *SnapshotWriter) Stop() {
	close(sw.stopCh)
}

func (sw *SnapshotWriter) run() {
	for {
		select {
		case <-sw.stopCh:
			return
		case ev := <-sw.ch:
			winCount := 0
			lossCount := 0
			if ev.IsWin {
				winCount = 1
			} else {
				lossCount = 1
			}

			snap := PnLSnapshot{
				Timestamp:     time.Now().UTC().Unix(),
				Strategy:      ev.Strategy,
				Exchange:      ev.Exchange,
				CumulativePnL: ev.PnL,
				PositionCount: 1,
				WinCount:      winCount,
				LossCount:     lossCount,
				FundingTotal:  ev.FundingTotal,
				FeesTotal:     ev.FeesTotal,
			}
			if err := sw.store.WritePnLSnapshot(snap); err != nil {
				sw.log.Error("snapshot write: %v", err)
			}
		}
	}
}

// RecordPerpClose sends a perp-perp position close event to the snapshot channel.
// Non-blocking: if the channel is full, the event is dropped with a warning log.
func (sw *SnapshotWriter) RecordPerpClose(pos *models.ArbitragePosition) {
	var perpFees float64
	if pos.HasReconciled {
		perpFees = math.Abs(pos.ExitFees) // reconciled: use actual fees (may be 0)
	} else {
		perpFees = pos.EntryFees // pre-reconciliation estimate
	}
	ev := snapshotEvent{
		Strategy:     "perp",
		Exchange:     "", // perp spans two exchanges — aggregate
		PnL:          pos.RealizedPnL,
		IsWin:        pos.RealizedPnL > 0,
		FundingTotal: pos.FundingCollected,
		FeesTotal:    perpFees,
	}
	select {
	case sw.ch <- ev:
	default:
		sw.log.Warn("snapshot channel full, dropping perp close event for %s", pos.ID)
	}
}

// RecordSpotClose sends a spot-futures position close event to the snapshot channel.
// Non-blocking: if the channel is full, the event is dropped with a warning log.
func (sw *SnapshotWriter) RecordSpotClose(pos *models.SpotFuturesPosition) {
	ev := snapshotEvent{
		Strategy:     "spot",
		Exchange:     pos.Exchange,
		PnL:          pos.RealizedPnL,
		IsWin:        pos.RealizedPnL > 0,
		FundingTotal: pos.FundingCollected,
		FeesTotal:    pos.EntryFees + pos.ExitFees,
	}
	select {
	case sw.ch <- ev:
	default:
		sw.log.Warn("snapshot channel full, dropping spot close event for %s", pos.ID)
	}
}

// BackfillFromHistory populates SQLite from Redis history on first startup.
// Only runs if store has no existing data (GetLatestTimestamp returns 0).
// Creates daily aggregate snapshots from position close timestamps.
func BackfillFromHistory(store *Store, perps []*models.ArbitragePosition, spots []*models.SpotFuturesPosition, log *utils.Logger) {
	latest, err := store.GetLatestTimestamp()
	if err != nil {
		log.Error("backfill: check latest timestamp: %v", err)
		return
	}
	if latest > 0 {
		log.Info("backfill: skipping — store already has data (latest=%d)", latest)
		return
	}

	if len(perps) == 0 && len(spots) == 0 {
		log.Info("backfill: no history to backfill")
		return
	}

	// Collect all close events with timestamps.
	type closeEvent struct {
		ts       time.Time
		strategy string
		exchange string
		pnl      float64
		isWin    bool
		funding  float64
		fees     float64
	}

	var events []closeEvent
	for _, p := range perps {
		events = append(events, closeEvent{
			ts:       p.UpdatedAt,
			strategy: "perp",
			exchange: "",
			pnl:      p.RealizedPnL,
			isWin:    p.RealizedPnL > 0,
			funding:  p.FundingCollected,
			fees: func() float64 {
				if p.HasReconciled { return math.Abs(p.ExitFees) }
				return p.EntryFees
			}(),
		})
	}
	for _, s := range spots {
		events = append(events, closeEvent{
			ts:       s.UpdatedAt,
			strategy: "spot",
			exchange: s.Exchange,
			pnl:      s.RealizedPnL,
			isWin:    s.RealizedPnL > 0,
			funding:  s.FundingCollected,
			fees:     s.EntryFees + s.ExitFees,
		})
	}

	// Sort by timestamp ascending.
	sort.Slice(events, func(i, j int) bool {
		return events[i].ts.Before(events[j].ts)
	})

	// Group by day (truncate to midnight UTC) and create running snapshots.
	type dayKey struct {
		day      int64 // midnight unix
		strategy string
	}

	daySnaps := make(map[dayKey]*PnLSnapshot)
	for _, ev := range events {
		midnight := time.Date(ev.ts.Year(), ev.ts.Month(), ev.ts.Day(), 0, 0, 0, 0, time.UTC)
		key := dayKey{day: midnight.Unix(), strategy: ev.strategy}
		snap, ok := daySnaps[key]
		if !ok {
			snap = &PnLSnapshot{
				Timestamp: midnight.Unix(),
				Strategy:  ev.strategy,
				Exchange:  ev.exchange,
			}
			daySnaps[key] = snap
		}
		snap.CumulativePnL += ev.pnl
		snap.PositionCount++
		if ev.isWin {
			snap.WinCount++
		} else {
			snap.LossCount++
		}
		snap.FundingTotal += ev.funding
		snap.FeesTotal += ev.fees
	}

	// Convert to slice and sort by timestamp.
	snaps := make([]PnLSnapshot, 0, len(daySnaps))
	for _, s := range daySnaps {
		snaps = append(snaps, *s)
	}
	sort.Slice(snaps, func(i, j int) bool {
		return snaps[i].Timestamp < snaps[j].Timestamp
	})

	if err := store.WritePnLSnapshots(snaps); err != nil {
		log.Error("backfill: write failed: %v", err)
		return
	}
	log.Info("backfill: wrote %d daily snapshots from %d perp + %d spot positions",
		len(snaps), len(perps), len(spots))
}
