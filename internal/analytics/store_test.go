package analytics

import (
	"path/filepath"
	"testing"
)

func TestNewStore(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	// Verify the DB is accessible.
	if err := store.db.Ping(); err != nil {
		t.Fatalf("Ping failed: %v", err)
	}
}

func TestTimeSeriesStore(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	// Write 3 snapshots at different timestamps and strategies.
	snaps := []PnLSnapshot{
		{Timestamp: 1000, Strategy: "perp", CumulativePnL: 10.0, PositionCount: 1, WinCount: 1},
		{Timestamp: 2000, Strategy: "spot", CumulativePnL: 20.0, PositionCount: 2, WinCount: 1, LossCount: 1},
		{Timestamp: 3000, Strategy: "perp", CumulativePnL: 30.0, PositionCount: 3, WinCount: 2, LossCount: 1},
	}
	for _, snap := range snaps {
		if err := store.WritePnLSnapshot(snap); err != nil {
			t.Fatalf("WritePnLSnapshot: %v", err)
		}
	}

	// Query full range, all strategies.
	all, err := store.GetPnLHistory(0, 5000, "all")
	if err != nil {
		t.Fatalf("GetPnLHistory(all): %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3, got %d", len(all))
	}
	// Verify ascending order.
	if all[0].Timestamp != 1000 || all[1].Timestamp != 2000 || all[2].Timestamp != 3000 {
		t.Errorf("wrong order: %v, %v, %v", all[0].Timestamp, all[1].Timestamp, all[2].Timestamp)
	}

	// Query sub-range [1500, 2500].
	sub, err := store.GetPnLHistory(1500, 2500, "all")
	if err != nil {
		t.Fatalf("GetPnLHistory(sub-range): %v", err)
	}
	if len(sub) != 1 {
		t.Fatalf("sub-range expected 1, got %d", len(sub))
	}
	if sub[0].Timestamp != 2000 {
		t.Errorf("expected ts=2000, got %d", sub[0].Timestamp)
	}

	// Query by strategy filter: "perp".
	perps, err := store.GetPnLHistory(0, 5000, "perp")
	if err != nil {
		t.Fatalf("GetPnLHistory(perp): %v", err)
	}
	if len(perps) != 2 {
		t.Fatalf("perp filter expected 2, got %d", len(perps))
	}
	for _, p := range perps {
		if p.Strategy != "perp" {
			t.Errorf("expected strategy=perp, got %q", p.Strategy)
		}
	}

	// Query by strategy filter: "spot".
	spots, err := store.GetPnLHistory(0, 5000, "spot")
	if err != nil {
		t.Fatalf("GetPnLHistory(spot): %v", err)
	}
	if len(spots) != 1 {
		t.Fatalf("spot filter expected 1, got %d", len(spots))
	}
	if spots[0].CumulativePnL != 20.0 {
		t.Errorf("expected pnl=20, got %f", spots[0].CumulativePnL)
	}
}

func TestWritePnLSnapshots_Batch(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	// Create 10 snapshots for batch insert.
	snaps := make([]PnLSnapshot, 10)
	for i := range snaps {
		snaps[i] = PnLSnapshot{
			Timestamp:     int64(1000 + i*100),
			Strategy:      "perp",
			CumulativePnL: float64(i + 1) * 5.0,
			PositionCount: i + 1,
			WinCount:      i,
		}
	}

	if err := store.WritePnLSnapshots(snaps); err != nil {
		t.Fatalf("WritePnLSnapshots: %v", err)
	}

	// Verify all 10 are queryable.
	all, err := store.GetPnLHistory(0, 10000, "all")
	if err != nil {
		t.Fatalf("GetPnLHistory: %v", err)
	}
	if len(all) != 10 {
		t.Fatalf("expected 10, got %d", len(all))
	}
}

func TestGetLatestTimestamp(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	// Write snapshots with known timestamps.
	snaps := []PnLSnapshot{
		{Timestamp: 1000, Strategy: "perp", CumulativePnL: 10},
		{Timestamp: 5000, Strategy: "spot", CumulativePnL: 50},
		{Timestamp: 3000, Strategy: "perp", CumulativePnL: 30},
	}
	if err := store.WritePnLSnapshots(snaps); err != nil {
		t.Fatalf("WritePnLSnapshots: %v", err)
	}

	latest, err := store.GetLatestTimestamp()
	if err != nil {
		t.Fatalf("GetLatestTimestamp: %v", err)
	}
	if latest != 5000 {
		t.Errorf("expected 5000, got %d", latest)
	}
}

func TestGetLatestTimestamp_Empty(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	latest, err := store.GetLatestTimestamp()
	if err != nil {
		t.Fatalf("GetLatestTimestamp: %v", err)
	}
	if latest != 0 {
		t.Errorf("expected 0 for empty DB, got %d", latest)
	}
}
