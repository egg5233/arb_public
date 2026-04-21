package database

import (
	"context"
	"testing"
	"time"

	"arb/internal/models"

	"github.com/alicebob/miniredis/v2"
)

// newPriceGapTestClient spins up miniredis + a *Client pointing at it.
// Mirrors the pattern used by locks_test.go.
func newPriceGapTestClient(t *testing.T) (*Client, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	db, err := New(mr.Addr(), "", 0)
	if err != nil {
		t.Fatalf("database.New: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db, mr
}

// fixturePosition produces a fully-populated PriceGapPosition so the
// round-trip test exercises every field.
func fixturePosition(id string, status string) *models.PriceGapPosition {
	opened := time.Date(2026, 4, 21, 10, 0, 0, 0, time.UTC)
	closed := opened.Add(2 * time.Hour)
	return &models.PriceGapPosition{
		ID:                 id,
		Symbol:             "SOONUSDT",
		LongExchange:       "binance",
		ShortExchange:      "bybit",
		Status:             status,
		EntrySpreadBps:     35.5,
		ThresholdBps:       25.0,
		NotionalUSDT:       500.0,
		LongFillPrice:      1.2345,
		ShortFillPrice:     1.2389,
		LongSize:           405.0,
		ShortSize:          405.0,
		LongMidAtDecision:  1.2340,
		ShortMidAtDecision: 1.2395,
		ModeledSlipBps:     8.0,
		RealizedSlipBps:    12.5,
		RealizedPnL:        2.17,
		ExitReason:         models.ExitReasonReverted,
		OpenedAt:           opened,
		ClosedAt:           closed,
	}
}

func TestPriceGapState_PositionRoundTrip(t *testing.T) {
	db, _ := newPriceGapTestClient(t)

	want := fixturePosition("SOONUSDT_binance_bybit_1", models.PriceGapStatusOpen)
	if err := db.SavePriceGapPosition(want); err != nil {
		t.Fatalf("SavePriceGapPosition: %v", err)
	}

	got, err := db.GetPriceGapPosition(want.ID)
	if err != nil {
		t.Fatalf("GetPriceGapPosition: %v", err)
	}

	// Field-by-field comparison. time.Time via .Equal.
	if got.ID != want.ID ||
		got.Symbol != want.Symbol ||
		got.LongExchange != want.LongExchange ||
		got.ShortExchange != want.ShortExchange ||
		got.Status != want.Status ||
		got.EntrySpreadBps != want.EntrySpreadBps ||
		got.ThresholdBps != want.ThresholdBps ||
		got.NotionalUSDT != want.NotionalUSDT ||
		got.LongFillPrice != want.LongFillPrice ||
		got.ShortFillPrice != want.ShortFillPrice ||
		got.LongSize != want.LongSize ||
		got.ShortSize != want.ShortSize ||
		got.LongMidAtDecision != want.LongMidAtDecision ||
		got.ShortMidAtDecision != want.ShortMidAtDecision ||
		got.ModeledSlipBps != want.ModeledSlipBps ||
		got.RealizedSlipBps != want.RealizedSlipBps ||
		got.RealizedPnL != want.RealizedPnL ||
		got.ExitReason != want.ExitReason {
		t.Fatalf("round-trip mismatch:\n got=%+v\nwant=%+v", got, want)
	}
	if !got.OpenedAt.Equal(want.OpenedAt) {
		t.Fatalf("OpenedAt mismatch: got=%v want=%v", got.OpenedAt, want.OpenedAt)
	}
	if !got.ClosedAt.Equal(want.ClosedAt) {
		t.Fatalf("ClosedAt mismatch: got=%v want=%v", got.ClosedAt, want.ClosedAt)
	}
}

func TestPriceGapState_ActiveSetTransitions(t *testing.T) {
	db, _ := newPriceGapTestClient(t)
	ctx := context.Background()

	open := fixturePosition("id-open", models.PriceGapStatusOpen)
	if err := db.SavePriceGapPosition(open); err != nil {
		t.Fatalf("save open: %v", err)
	}

	members, err := db.rdb.SMembers(ctx, keyPricegapActive).Result()
	if err != nil {
		t.Fatalf("SMembers: %v", err)
	}
	if len(members) != 1 || members[0] != "id-open" {
		t.Fatalf("active set after open: got %v, want [id-open]", members)
	}

	// Now close the same ID.
	open.Status = models.PriceGapStatusClosed
	if err := db.SavePriceGapPosition(open); err != nil {
		t.Fatalf("save closed: %v", err)
	}

	members, err = db.rdb.SMembers(ctx, keyPricegapActive).Result()
	if err != nil {
		t.Fatalf("SMembers after close: %v", err)
	}
	if len(members) != 0 {
		t.Fatalf("active set after close: got %v, want empty", members)
	}

	// GetActive must also be empty.
	active, err := db.GetActivePriceGapPositions()
	if err != nil {
		t.Fatalf("GetActivePriceGapPositions: %v", err)
	}
	if len(active) != 0 {
		t.Fatalf("GetActivePriceGapPositions: got %d, want 0", len(active))
	}

	// Exiting status must also track in active set.
	exiting := fixturePosition("id-exiting", models.PriceGapStatusExiting)
	if err := db.SavePriceGapPosition(exiting); err != nil {
		t.Fatalf("save exiting: %v", err)
	}
	members, _ = db.rdb.SMembers(ctx, keyPricegapActive).Result()
	if len(members) != 1 || members[0] != "id-exiting" {
		t.Fatalf("active set after exiting: got %v, want [id-exiting]", members)
	}

	// RemoveActivePriceGapPosition drops it.
	if err := db.RemoveActivePriceGapPosition("id-exiting"); err != nil {
		t.Fatalf("RemoveActivePriceGapPosition: %v", err)
	}
	members, _ = db.rdb.SMembers(ctx, keyPricegapActive).Result()
	if len(members) != 0 {
		t.Fatalf("active set after remove: got %v, want empty", members)
	}
}

func TestPriceGapState_HistoryCap(t *testing.T) {
	db, _ := newPriceGapTestClient(t)
	ctx := context.Background()

	// Push 600 entries to force LTrim to kick in.
	for i := 0; i < 600; i++ {
		p := fixturePosition("hist-"+time.Now().Format("150405.000000")+string(rune(i)), models.PriceGapStatusClosed)
		if err := db.AddPriceGapHistory(p); err != nil {
			t.Fatalf("AddPriceGapHistory[%d]: %v", i, err)
		}
	}

	llen, err := db.rdb.LLen(ctx, keyPricegapHistory).Result()
	if err != nil {
		t.Fatalf("LLen: %v", err)
	}
	if llen != priceGapHistoryCap {
		t.Fatalf("history length: got %d, want %d", llen, priceGapHistoryCap)
	}
}

func TestPriceGapState_DisableFlag(t *testing.T) {
	db, _ := newPriceGapTestClient(t)

	// Initially missing.
	disabled, reason, err := db.IsCandidateDisabled("SOONUSDT")
	if err != nil {
		t.Fatalf("IsCandidateDisabled initial: %v", err)
	}
	if disabled || reason != "" {
		t.Fatalf("initial disable: got (%v,%q), want (false,\"\")", disabled, reason)
	}

	// Set → Is returns (true, reason).
	if err := db.SetCandidateDisabled("SOONUSDT", "exec_quality_breach"); err != nil {
		t.Fatalf("SetCandidateDisabled: %v", err)
	}
	disabled, reason, err = db.IsCandidateDisabled("SOONUSDT")
	if err != nil {
		t.Fatalf("IsCandidateDisabled after set: %v", err)
	}
	if !disabled || reason != "exec_quality_breach" {
		t.Fatalf("after set: got (%v,%q), want (true,\"exec_quality_breach\")", disabled, reason)
	}

	// Clear → back to false.
	if err := db.ClearCandidateDisabled("SOONUSDT"); err != nil {
		t.Fatalf("ClearCandidateDisabled: %v", err)
	}
	disabled, reason, err = db.IsCandidateDisabled("SOONUSDT")
	if err != nil {
		t.Fatalf("IsCandidateDisabled after clear: %v", err)
	}
	if disabled || reason != "" {
		t.Fatalf("after clear: got (%v,%q), want (false,\"\")", disabled, reason)
	}
}

func TestPriceGapState_SlippageWindow(t *testing.T) {
	db, _ := newPriceGapTestClient(t)
	candidateID := "SOONUSDT_binance_bybit"

	// Append 15 samples with timestamps 1..15.
	base := time.Date(2026, 4, 21, 10, 0, 0, 0, time.UTC)
	for i := 1; i <= 15; i++ {
		s := models.SlippageSample{
			PositionID: "pos-" + time.Duration(i).String(),
			Realized:   float64(i),
			Modeled:    float64(i) - 0.5,
			Timestamp:  base.Add(time.Duration(i) * time.Second),
		}
		if err := db.AppendSlippageSample(candidateID, s); err != nil {
			t.Fatalf("AppendSlippageSample[%d]: %v", i, err)
		}
	}

	window, err := db.GetSlippageWindow(candidateID, 10)
	if err != nil {
		t.Fatalf("GetSlippageWindow: %v", err)
	}
	if len(window) != 10 {
		t.Fatalf("window size: got %d, want 10", len(window))
	}

	// Newest-first: the first entry must be ts=15, the last must be ts=6.
	if want := float64(15); window[0].Realized != want {
		t.Fatalf("window[0].Realized: got %v, want %v (newest)", window[0].Realized, want)
	}
	if want := float64(6); window[9].Realized != want {
		t.Fatalf("window[9].Realized: got %v, want %v (oldest in cap)", window[9].Realized, want)
	}

	// n<=0 and n>cap should both return full cap.
	wFull, _ := db.GetSlippageWindow(candidateID, 0)
	if len(wFull) != priceGapSlippageCap {
		t.Fatalf("n=0: got %d, want %d", len(wFull), priceGapSlippageCap)
	}
	wOver, _ := db.GetSlippageWindow(candidateID, 999)
	if len(wOver) != priceGapSlippageCap {
		t.Fatalf("n=999: got %d, want %d", len(wOver), priceGapSlippageCap)
	}
}

func TestPriceGapState_LockPrefix(t *testing.T) {
	db, mr := newPriceGapTestClient(t)
	ctx := context.Background()

	token1, ok, err := db.AcquirePriceGapLock("SOONUSDT", 5*time.Second)
	if err != nil {
		t.Fatalf("first Acquire: %v", err)
	}
	if !ok || token1 == "" {
		t.Fatalf("first Acquire: ok=%v token=%q, want ok=true token!=\"\"", ok, token1)
	}

	// Verify the final key uses the pg: sub-prefix under arb:locks:.
	if !mr.Exists("arb:locks:pg:SOONUSDT") {
		keys := mr.Keys()
		t.Fatalf("expected key arb:locks:pg:SOONUSDT in miniredis; actual keys: %v", keys)
	}

	// Second acquire must fail.
	token2, ok, err := db.AcquirePriceGapLock("SOONUSDT", 5*time.Second)
	if err != nil {
		t.Fatalf("second Acquire: %v", err)
	}
	if ok || token2 != "" {
		t.Fatalf("second Acquire: ok=%v token=%q, want ok=false token=\"\"", ok, token2)
	}

	// Release with the wrong token must NOT delete the lock (compare-and-delete).
	if err := db.ReleasePriceGapLock("SOONUSDT", "not-the-real-token"); err != nil {
		t.Fatalf("Release wrong token: %v", err)
	}
	if !mr.Exists("arb:locks:pg:SOONUSDT") {
		t.Fatalf("lock key was deleted by wrong-token release")
	}

	// Release with the correct token succeeds.
	if err := db.ReleasePriceGapLock("SOONUSDT", token1); err != nil {
		t.Fatalf("Release correct token: %v", err)
	}
	if mr.Exists("arb:locks:pg:SOONUSDT") {
		t.Fatalf("lock key still present after correct-token release")
	}

	// Third acquire must now succeed.
	token3, ok, err := db.AcquirePriceGapLock("SOONUSDT", 5*time.Second)
	if err != nil {
		t.Fatalf("third Acquire: %v", err)
	}
	if !ok || token3 == "" {
		t.Fatalf("third Acquire: ok=%v token=%q, want ok=true", ok, token3)
	}
	// Clean up.
	_ = db.ReleasePriceGapLock("SOONUSDT", token3)

	// Sanity: GetActivePriceGapPositions on empty state is err-free.
	if _, err := db.GetActivePriceGapPositions(); err != nil {
		t.Fatalf("GetActivePriceGapPositions empty: %v", err)
	}
	// Silence unused ctx warning if refactor removes the earlier use.
	_ = ctx
}

func TestPriceGapState_InterfaceAssertion(t *testing.T) {
	db, _ := newPriceGapTestClient(t)
	// Compile-time interface check — the assignment itself is the assertion.
	var store models.PriceGapStore = db
	if store == nil {
		t.Fatal("store nil after assertion")
	}
}
