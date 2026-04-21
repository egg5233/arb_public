package database

import (
	"strconv"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
)

func newDBForMarginFeeTest(t *testing.T) (*Client, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	db, err := New(mr.Addr(), "", 0)
	if err != nil {
		t.Fatalf("database.New: %v", err)
	}
	return db, mr
}

func TestGetCoinGlassMarginFeeHistory_EmptyKey(t *testing.T) {
	db, mr := newDBForMarginFeeTest(t)
	defer mr.Close()

	got, err := db.GetCoinGlassMarginFeeHistory("okx", "BTC", time.Unix(0, 0), time.Now())
	if err != nil {
		t.Fatalf("unexpected error on empty key: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty result, got %d", len(got))
	}
}

func TestGetCoinGlassMarginFeeHistory_RoundtripAndFilter(t *testing.T) {
	db, mr := newDBForMarginFeeTest(t)
	defer mr.Close()

	// Simulate 4 hourly samples that the CoinGlass scraper would have LPUSHed.
	now := time.Now().UTC().Truncate(time.Hour)
	points := []struct {
		t    int64
		rate float64
	}{
		{now.Unix(), 1.1e-6},
		{now.Add(-time.Hour).Unix(), 1.2e-6},
		{now.Add(-5 * time.Hour).Unix(), 1.5e-6},
		{now.Add(-48 * time.Hour).Unix(), 9.9e-6}, // out of range
	}
	for _, p := range points {
		entry := `{"t":` + strconv.FormatInt(p.t, 10) + `,"rate":` + strconv.FormatFloat(p.rate, 'g', -1, 64) + `,"limit":"100"}`
		mr.Lpush("coinGlassMarginFee:hist:okx:BTC", entry)
	}

	// Query the last 24h.
	start := now.Add(-24 * time.Hour)
	end := now.Add(time.Minute)
	got, err := db.GetCoinGlassMarginFeeHistory("okx", "BTC", start, end)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 in-range points (48h one filtered out), got %d: %+v", len(got), got)
	}
	// Rates should round-trip.
	ratesSeen := map[float64]bool{}
	for _, p := range got {
		ratesSeen[p.HourlyRate] = true
	}
	for _, want := range []float64{1.1e-6, 1.2e-6, 1.5e-6} {
		if !ratesSeen[want] {
			t.Errorf("expected rate %v in results, got %+v", want, got)
		}
	}
	if ratesSeen[9.9e-6] {
		t.Errorf("rate 9.9e-6 was 48h old and should have been filtered out")
	}
}

// TestGetCoinGlassMarginFeeHistory_PartialEntryDoesNotInheritPriorValues
// locks in the fix for the JSON-reuse bug flagged by codex review e466a1de:
// a valid entry followed by `{}` would previously cause the empty object to
// inherit the prior sample's t/rate via the reused `entry` struct and a
// permissive Unmarshal, silently poisoning borrow-rate history.
func TestGetCoinGlassMarginFeeHistory_PartialEntryDoesNotInheritPriorValues(t *testing.T) {
	db, mr := newDBForMarginFeeTest(t)
	defer mr.Close()

	now := time.Now().UTC().Truncate(time.Hour)
	// LPUSH order: head is most recent. Put the "{}" (missing t/rate) AFTER
	// a valid entry, so LRange returns them in the order that triggers the bug.
	mr.Lpush("coinGlassMarginFee:hist:okx:BTC", `{}`)
	mr.Lpush("coinGlassMarginFee:hist:okx:BTC", `{"t":`+strconv.FormatInt(now.Unix(), 10)+`,"rate":5e-6}`)

	got, err := db.GetCoinGlassMarginFeeHistory("okx", "BTC", now.Add(-time.Hour), now.Add(time.Minute))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 point (the `{}` entry must be dropped, not inherit prior values), got %d: %+v", len(got), got)
	}
	if got[0].HourlyRate != 5e-6 {
		t.Fatalf("expected rate 5e-6 from the valid entry, got %v", got[0].HourlyRate)
	}
}

func TestGetCoinGlassMarginFeeHistory_MalformedEntriesSkipped(t *testing.T) {
	db, mr := newDBForMarginFeeTest(t)
	defer mr.Close()

	now := time.Now().UTC().Truncate(time.Hour)
	mr.Lpush("coinGlassMarginFee:hist:bitget:ETH", "not-json")
	mr.Lpush("coinGlassMarginFee:hist:bitget:ETH", `{"t":`+strconv.FormatInt(now.Unix(), 10)+`,"rate":5e-6}`)
	mr.Lpush("coinGlassMarginFee:hist:bitget:ETH", "{}")

	got, err := db.GetCoinGlassMarginFeeHistory("bitget", "ETH", now.Add(-time.Hour), now.Add(time.Minute))
	if err != nil {
		t.Fatalf("malformed entries should not error, got: %v", err)
	}
	// One valid entry in range. The "{}" entry has t=0 which is epoch — far out of range.
	if len(got) != 1 {
		t.Fatalf("expected 1 valid in-range entry, got %d", len(got))
	}
	if got[0].HourlyRate != 5e-6 {
		t.Fatalf("expected rate 5e-6, got %v", got[0].HourlyRate)
	}
}
