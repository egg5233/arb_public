package database

import (
	"context"
	"encoding/json"
	"math"
	"testing"

	"arb/internal/models"
)

// TestPriceGapState_SaveLoadBreakerState — round-trip the 5-field HASH at
// pg:breaker:state. Includes PaperModeStickyUntil=math.MaxInt64 to verify the
// int64 sentinel survives the decimal-string roundtrip without truncation.
func TestPriceGapState_SaveLoadBreakerState(t *testing.T) {
	db, _ := newPriceGapTestClient(t)

	want := models.BreakerState{
		PendingStrike:        1,
		Strike1Ts:            1714521600000, // unix-ms
		LastEvalTs:           1714521900000,
		LastEvalPnLUSDT:      -42.5,
		PaperModeStickyUntil: math.MaxInt64,
	}
	if err := db.SaveBreakerState(want); err != nil {
		t.Fatalf("SaveBreakerState: %v", err)
	}

	got, exists, err := db.LoadBreakerState()
	if err != nil {
		t.Fatalf("LoadBreakerState: %v", err)
	}
	if !exists {
		t.Fatal("LoadBreakerState: exists=false after save, want true")
	}
	if got != want {
		t.Fatalf("roundtrip mismatch:\n got %+v\nwant %+v", got, want)
	}
}

// TestPriceGapState_LoadBreakerStateMissing — fresh DB, no HASH yet. Boot
// guard relies on (zero, exists=false, nil) signal to fresh-init the state.
func TestPriceGapState_LoadBreakerStateMissing(t *testing.T) {
	db, _ := newPriceGapTestClient(t)

	got, exists, err := db.LoadBreakerState()
	if err != nil {
		t.Fatalf("LoadBreakerState: unexpected err on missing key: %v", err)
	}
	if exists {
		t.Fatal("LoadBreakerState: exists=true on missing key, want false")
	}
	if got != (models.BreakerState{}) {
		t.Fatalf("LoadBreakerState: expected zero-value state, got %+v", got)
	}
}

// TestPriceGapState_BreakerTripsCap — the LIST is capped at 500 entries via
// LTRIM (D-18). Insert 502 entries and verify exactly 500 remain, oldest 2
// evicted, newest at the head (LPUSH inserts at index 0).
func TestPriceGapState_BreakerTripsCap(t *testing.T) {
	db, _ := newPriceGapTestClient(t)

	for i := 0; i < 502; i++ {
		rec := models.BreakerTripRecord{
			TripTs:      int64(i),
			TripPnLUSDT: float64(-i),
			Threshold:   -100,
			RampStage:   1,
			Source:      "live",
		}
		if err := db.AppendBreakerTrip(rec); err != nil {
			t.Fatalf("AppendBreakerTrip i=%d: %v", i, err)
		}
	}

	ctx := context.Background()
	length, err := db.rdb.LLen(ctx, keyPriceGapBreakerTrips).Result()
	if err != nil {
		t.Fatalf("LLen: %v", err)
	}
	if length != 500 {
		t.Fatalf("expected list length 500, got %d", length)
	}

	// Newest at index 0 (LPUSH semantics).
	head, err := db.rdb.LIndex(ctx, keyPriceGapBreakerTrips, 0).Result()
	if err != nil {
		t.Fatalf("LIndex 0: %v", err)
	}
	var headRec models.BreakerTripRecord
	if err := json.Unmarshal([]byte(head), &headRec); err != nil {
		t.Fatalf("unmarshal head: %v", err)
	}
	if headRec.TripTs != 501 {
		t.Fatalf("expected newest TripTs=501 at index 0, got %d", headRec.TripTs)
	}

	// Oldest survivor at index 499 must be TripTs=2 (entries 0 and 1 evicted).
	tail, err := db.rdb.LIndex(ctx, keyPriceGapBreakerTrips, 499).Result()
	if err != nil {
		t.Fatalf("LIndex 499: %v", err)
	}
	var tailRec models.BreakerTripRecord
	if err := json.Unmarshal([]byte(tail), &tailRec); err != nil {
		t.Fatalf("unmarshal tail: %v", err)
	}
	if tailRec.TripTs != 2 {
		t.Fatalf("expected oldest survivor TripTs=2 at index 499, got %d", tailRec.TripTs)
	}
}

// TestPriceGapState_UpdateBreakerTripRecovery — LPush a trip with no recovery,
// then UpdateBreakerTripRecovery rewrites entry via LSET with backfilled
// RecoveryTs + RecoveryOperator.
func TestPriceGapState_UpdateBreakerTripRecovery(t *testing.T) {
	db, _ := newPriceGapTestClient(t)

	rec := models.BreakerTripRecord{
		TripTs:      1714521600000,
		TripPnLUSDT: -42.5,
		Threshold:   -50,
		RampStage:   2,
		Source:      "live",
	}
	if err := db.AppendBreakerTrip(rec); err != nil {
		t.Fatalf("AppendBreakerTrip: %v", err)
	}

	const recoveryTs = int64(1714521900000)
	const operator = "alice"
	if err := db.UpdateBreakerTripRecovery(0, recoveryTs, operator); err != nil {
		t.Fatalf("UpdateBreakerTripRecovery: %v", err)
	}

	ctx := context.Background()
	got, err := db.rdb.LIndex(ctx, keyPriceGapBreakerTrips, 0).Result()
	if err != nil {
		t.Fatalf("LIndex 0: %v", err)
	}
	var updated models.BreakerTripRecord
	if err := json.Unmarshal([]byte(got), &updated); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if updated.RecoveryTs == nil || *updated.RecoveryTs != recoveryTs {
		t.Fatalf("RecoveryTs not backfilled: %+v", updated.RecoveryTs)
	}
	if updated.RecoveryOperator == nil || *updated.RecoveryOperator != operator {
		t.Fatalf("RecoveryOperator not backfilled: %+v", updated.RecoveryOperator)
	}
	// Pre-existing fields preserved.
	if updated.TripTs != rec.TripTs || updated.TripPnLUSDT != rec.TripPnLUSDT {
		t.Fatalf("LSET corrupted existing fields: got %+v want %+v", updated, rec)
	}
}

// TestPriceGapState_StickyMaxInt64Roundtrip — D-07 sentinel encoding. Stored
// as decimal string of int64; must NOT lose precision through float64
// conversion or any other intermediate.
func TestPriceGapState_StickyMaxInt64Roundtrip(t *testing.T) {
	db, _ := newPriceGapTestClient(t)

	state := models.BreakerState{
		PaperModeStickyUntil: math.MaxInt64,
	}
	if err := db.SaveBreakerState(state); err != nil {
		t.Fatalf("SaveBreakerState: %v", err)
	}
	got, exists, err := db.LoadBreakerState()
	if err != nil || !exists {
		t.Fatalf("LoadBreakerState: exists=%v err=%v", exists, err)
	}
	if got.PaperModeStickyUntil != math.MaxInt64 {
		t.Fatalf("PaperModeStickyUntil sentinel lost: got %d want %d",
			got.PaperModeStickyUntil, int64(math.MaxInt64))
	}
}
