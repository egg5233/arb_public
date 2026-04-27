package database

import (
	"testing"
	"time"

	"arb/internal/models"
	"arb/internal/strategy"

	"github.com/alicebob/miniredis/v2"
)

func TestDirBSLOFinalizesOnceOnActiveCompletion(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()
	db, err := New(mr.Addr(), "", 0)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	pending := strategy.PendingDirBConflict{
		DirBCandidateID:       "dirb-1",
		ConflictReservationID: "pp-1",
		Epoch:                 7,
		ConflictStrategy:      strategy.StrategyPP,
		ConflictCandidateID:   "pp-candidate",
		OverlappingKeys:       []strategy.LegKey{{Exchange: "binance", Market: "futures", Symbol: "BTCUSDT"}},
		DirBEVBpsH:            5,
		ConflictEVBpsH:        1,
		CreatedAt:             time.Now().UTC(),
		ExpiresAt:             time.Now().UTC().Add(15 * time.Minute),
	}
	if err := db.RecordDirBPendingConflict(pending); err != nil {
		t.Fatalf("RecordDirBPendingConflict: %v", err)
	}
	comp := strategy.ReservationCompletionRecord{
		ReservationID: "pp-1",
		Outcome:       strategy.ReservationOutcomeActive,
		Epoch:         7,
		Strategy:      strategy.StrategyPP,
		CandidateID:   "pp-candidate",
		Keys:          pending.OverlappingKeys,
		CompletedAt:   time.Now().UTC(),
	}
	if err := db.RecordReservationCompletion(comp); err != nil {
		t.Fatalf("RecordReservationCompletion: %v", err)
	}
	if err := db.FinalizeDirBConflicts(comp); err != nil {
		t.Fatalf("FinalizeDirBConflicts: %v", err)
	}
	if err := db.FinalizeDirBConflicts(comp); err != nil {
		t.Fatalf("FinalizeDirBConflicts duplicate: %v", err)
	}
	stats, err := db.GetDirBSLOStats()
	if err != nil {
		t.Fatalf("GetDirBSLOStats: %v", err)
	}
	if got := stats["dirb_slo_breach_count"].(int64); got != 1 {
		t.Fatalf("breach count = %d, want 1", got)
	}
}

func TestDirBSLOReplayFromActivePosition(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()
	db, err := New(mr.Addr(), "", 0)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	keys := []strategy.LegKey{{Exchange: "binance", Market: "futures", Symbol: "BTCUSDT"}}
	if err := db.RecordDirBPendingConflict(strategy.PendingDirBConflict{
		DirBCandidateID:       "dirb-1",
		ConflictReservationID: "pp-1",
		Epoch:                 3,
		ConflictStrategy:      strategy.StrategyPP,
		ConflictCandidateID:   "pp-candidate",
		OverlappingKeys:       keys,
		DirBEVBpsH:            5,
		ConflictEVBpsH:        1,
		CreatedAt:             time.Now().UTC(),
		ExpiresAt:             time.Now().UTC().Add(15 * time.Minute),
	}); err != nil {
		t.Fatalf("RecordDirBPendingConflict: %v", err)
	}
	if err := db.SavePosition(&models.ArbitragePosition{
		ID:                    "pos-1",
		Symbol:                "BTCUSDT",
		LongExchange:          "binance",
		ShortExchange:         "bybit",
		Status:                models.StatusActive,
		StrategyReservationID: "pp-1",
		StrategyCandidateID:   "pp-candidate",
		StrategyEpoch:         3,
		Strategy:              string(strategy.StrategyPP),
		StrategyLegKeys:       strategy.LegKeyStrings(keys),
	}); err != nil {
		t.Fatalf("SavePosition: %v", err)
	}
	if err := db.ReplayDirBSLOFromPositions(); err != nil {
		t.Fatalf("ReplayDirBSLOFromPositions: %v", err)
	}
	stats, err := db.GetDirBSLOStats()
	if err != nil {
		t.Fatalf("GetDirBSLOStats: %v", err)
	}
	if got := stats["dirb_slo_breach_count"].(int64); got != 1 {
		t.Fatalf("breach count = %d, want 1", got)
	}
}
