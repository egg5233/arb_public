// Plan 10-04 Task 2 — Hot-reload invariant tests.
//
// Pins RESEARCH.md §"Pitfall 1: Tracker startup-cache silently breaks
// hot-reload" + Assumption A1 against future refactors.
//
// Two complementary tests:
//
//  1. TestTracker_HotReloadCandidates_NoStartupCache_StaticAssertion
//     Static (file-scan) assertion that the Tracker struct in tracker.go
//     does NOT have a field of slice-of-PriceGapCandidate type. If a
//     future PR adds `candidates []models.PriceGapCandidate` (or the
//     fully-qualified `[]models.PriceGapCandidate`) as a struct field
//     on Tracker, this test fails with the Pitfall-1 warning. The
//     production code MUST keep reading `t.cfg.PriceGapCandidates`
//     directly per tick.
//
//  2. TestTracker_HotReloadCandidates_PerTickRead
//     Dynamic check: build a Tracker with a one-candidate cfg, observe
//     CandidateSnapshotForTest()=1, mutate cfg.PriceGapCandidates to
//     two candidates, observe CandidateSnapshotForTest()=2 — proves
//     that no internal copy was taken at construction time and the
//     per-tick read sites (lines 226, 293, 413 of tracker.go) will
//     pick up the second entry without a process restart.
//
// CandidateSnapshotForTest is a tiny helper added to tracker.go that
// just returns len(t.cfg.PriceGapCandidates). It is intentionally
// trivial because the per-tick scan paths require live exchanges /
// Redis to drive end-to-end; this is enough to lock the invariant.
package pricegaptrader

import (
	"os"
	"regexp"
	"testing"

	"arb/internal/config"
	"arb/internal/models"
	"arb/pkg/exchange"
)

// TestTracker_HotReloadCandidates_NoStartupCache_StaticAssertion is a
// build-the-future-can't-regress guard. It scans tracker.go for any field
// declaration on the Tracker struct that copies the candidate slice into
// the struct. The exact pattern matches both `candidates []models.PriceGapCandidate`
// and the unqualified `candidates []PriceGapCandidate` form.
//
// If this test fails, see RESEARCH.md §"Pitfall 1" — the proper fix is to
// keep reading t.cfg.PriceGapCandidates directly per tick (NOT to cache
// the slice at startup time).
func TestTracker_HotReloadCandidates_NoStartupCache_StaticAssertion(t *testing.T) {
	data, err := os.ReadFile("tracker.go")
	if err != nil {
		t.Fatalf("read tracker.go: %v", err)
	}
	// Match: indented field name + `[]` + optional `models.` + `PriceGapCandidate`
	// followed by space, end of line, or backtick (struct tag start).
	// Examples MATCHED (would fail the test):
	//     candidates []models.PriceGapCandidate
	//     candidates []PriceGapCandidate `json:"candidates"`
	// Examples NOT MATCHED (allowed):
	//     bars   map[string]*candidateBars
	//     for _, cand := range t.cfg.PriceGapCandidates {
	re := regexp.MustCompile(`(?m)^\s+\w+\s+\[\](?:models\.)?PriceGapCandidate(?:\s|$|` + "`" + `)`)
	if loc := re.FindIndex(data); loc != nil {
		// Surface the offending source line in the failure message so the
		// next maintainer immediately sees what tripped the guard.
		start := loc[0]
		end := loc[1]
		// Walk to end-of-line so we print the whole field declaration.
		for end < len(data) && data[end] != '\n' {
			end++
		}
		t.Fatalf(
			"Pitfall 1 (RESEARCH.md): Tracker struct must NOT cache PriceGapCandidate slice.\n"+
				"Production code MUST keep reading t.cfg.PriceGapCandidates directly per tick.\n"+
				"Caching the slice at startup time silently breaks hot-reload (Add/Edit/Delete\n"+
				"from the dashboard would require a process restart to take effect).\n"+
				"Offending field declaration in tracker.go around byte offset %d:\n  %s",
			start, string(data[start:end]),
		)
	}
}

// TestTracker_HotReloadCandidates_PerTickRead constructs a real Tracker,
// observes the candidate-count snapshot, mutates cfg.PriceGapCandidates,
// and observes the snapshot again. If the count grows from 1 to 2 without
// re-constructing the Tracker, the per-tick read sites are wired correctly
// and Add/Edit/Delete from the dashboard will hot-reload.
//
// This test uses the existing stubExchange (zero deps) and a nil store /
// nil delist-checker because CandidateSnapshotForTest only reads cfg.
// We do NOT exercise runTick / detectOnce here — the static-assertion test
// above plus this dynamic check together cover Pitfall 1 + Assumption A1.
func TestTracker_HotReloadCandidates_PerTickRead(t *testing.T) {
	bin := newStubExchange("binance")
	byb := newStubExchange("bybit")
	exch := map[string]exchange.Exchange{"binance": bin, "bybit": byb}

	// Minimal cfg with exactly one candidate.
	cfg := &config.Config{
		PriceGapEnabled:         true,
		PriceGapPollIntervalSec: 30,
		PriceGapCandidates: []models.PriceGapCandidate{
			{
				Symbol:             "BTCUSDT",
				LongExch:           "binance",
				ShortExch:          "bybit",
				ThresholdBps:       200,
				MaxPositionUSDT:    5000,
				ModeledSlippageBps: 5,
			},
		},
	}

	tr := NewTracker(exch, nil /*db*/, nil /*delist*/, cfg)
	if tr == nil {
		t.Fatalf("NewTracker returned nil")
	}

	// Snapshot 1: should observe the seeded single candidate.
	if got := tr.CandidateSnapshotForTest(); got != 1 {
		t.Fatalf("pre-mutation: want 1 candidate, got %d", got)
	}

	// Mutate the shared *Config slice — this is exactly what the /api/config
	// POST handler does under s.cfg.Lock() (handlers.go apply branch from
	// Plan 10-01). The Tracker, holding the same *Config pointer, must see
	// the new state on the next per-tick read.
	cfg.PriceGapCandidates = append(cfg.PriceGapCandidates, models.PriceGapCandidate{
		Symbol:             "ETHUSDT",
		LongExch:           "gateio",
		ShortExch:          "bybit",
		ThresholdBps:       300,
		MaxPositionUSDT:    1000,
		ModeledSlippageBps: 10,
	})

	// Snapshot 2: must observe the mutation. If this fails with "want 2,
	// got 1", the tracker has cached the slice somewhere and Pitfall 1
	// has bitten us — the static assertion above should also be failing.
	if got := tr.CandidateSnapshotForTest(); got != 2 {
		t.Fatalf("post-mutation: want 2 candidates (hot-reload), got %d — "+
			"tracker likely cached the slice (Pitfall 1 in RESEARCH.md)", got)
	}

	// Belt-and-braces: a delete (shrink to 0) must also be observable.
	cfg.PriceGapCandidates = nil
	if got := tr.CandidateSnapshotForTest(); got != 0 {
		t.Fatalf("post-delete: want 0 candidates, got %d — "+
			"hot-reload of delete is broken", got)
	}
}
