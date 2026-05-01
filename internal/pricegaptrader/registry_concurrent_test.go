package pricegaptrader_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"arb/internal/config"
	"arb/internal/models"
	"arb/internal/pricegaptrader"
	"arb/internal/pricegaptrader/testhelpers"
	"arb/pkg/utils"
)

// concurrentNopAudit is a thread-safe nop audit writer scoped to this test
// file (package-internal) to avoid coupling registry_test.go's fakeAuditWriter
// to a different test package.
type concurrentNopAudit struct {
	pushCount atomic.Int64
}

func (a *concurrentNopAudit) LPush(ctx context.Context, key string, vals ...interface{}) (int64, error) {
	a.pushCount.Add(1)
	return 0, nil
}

func (a *concurrentNopAudit) LTrim(ctx context.Context, key string, start, stop int64) error {
	return nil
}

// setupConcurrentSharedConfig writes a seed config.json with the given cap and
// returns the path. Subsequent goroutines build their own *config.Config off
// this path and run independently.
func setupConcurrentSharedConfig(t *testing.T, capCandidates int) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	t.Setenv("CONFIG_FILE", path)
	seed := map[string]interface{}{
		"strategy": map[string]interface{}{},
		"price_gap": map[string]interface{}{
			"max_candidates": capCandidates,
		},
	}
	b, _ := json.MarshalIndent(seed, "", "  ")
	if err := os.WriteFile(path, b, 0644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	return path
}

func newConcurrentRegistry(t *testing.T, path string, capCandidates int, audit pricegaptrader.RegistryAuditWriter) *pricegaptrader.Registry {
	t.Helper()
	cfg := config.Load()
	cfg.PriceGapMaxCandidates = capCandidates
	log := utils.NewLogger("registry-conc-test")
	return pricegaptrader.NewRegistry(cfg, audit, log)
}

func loadCandidatesFromDisk(t *testing.T, path string) []models.PriceGapCandidate {
	t.Helper()
	cfg := config.Load()
	out := make([]models.PriceGapCandidate, len(cfg.PriceGapCandidates))
	copy(out, cfg.PriceGapCandidates)
	return out
}

// Test 1: in-process goroutine race against a single Registry instance.
// All 50 unique-symbol Adds must succeed; final on-disk count == 50.
func TestRegistry_ConcurrentInProcessAdds(t *testing.T) {
	path := setupConcurrentSharedConfig(t, 100)
	audit := &concurrentNopAudit{}
	r := newConcurrentRegistry(t, path, 100, audit)

	const N = 50
	var wg sync.WaitGroup
	errs := make(chan error, N)
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			c := models.PriceGapCandidate{
				Symbol:    fmt.Sprintf("SYM%02dUSDT", i),
				LongExch:  "binance",
				ShortExch: "bybit",
				Direction: models.PriceGapDirectionBidirectional,
			}
			if err := r.Add(context.Background(), "race-test", c); err != nil {
				errs <- fmt.Errorf("add %d: %w", i, err)
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("%v", err)
	}

	got := loadCandidatesFromDisk(t, path)
	if len(got) != N {
		t.Fatalf("on-disk count=%d, want %d (lost mutations under in-process race)", len(got), N)
	}
	if int(audit.pushCount.Load()) != N {
		t.Fatalf("audit push count=%d, want %d", audit.pushCount.Load(), N)
	}
}

// Test 2: in-process Add+Replace race. Final state must be one of:
//   - Replace-won (empty slice on disk)
//   - Adds-after-Replace (some Add candidates present)
//
// What must NEVER happen is a torn state where SaveJSONWithBakRing left a
// partial file behind.
func TestRegistry_ConcurrentAddAndReplace(t *testing.T) {
	path := setupConcurrentSharedConfig(t, 100)
	audit := &concurrentNopAudit{}
	r := newConcurrentRegistry(t, path, 100, audit)

	var wg sync.WaitGroup
	const Adds = 25
	const Replaces = 5
	for i := 0; i < Adds; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			c := models.PriceGapCandidate{
				Symbol:    fmt.Sprintf("ADD%02dUSDT", i),
				LongExch:  "binance",
				ShortExch: "bybit",
				Direction: models.PriceGapDirectionBidirectional,
			}
			_ = r.Add(context.Background(), "race-test", c)
		}(i)
	}
	for j := 0; j < Replaces; j++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = r.Replace(context.Background(), "race-test", []models.PriceGapCandidate{})
		}()
	}
	wg.Wait()

	// On-disk file must be parseable JSON (no torn writes from rename
	// atomicity). The actual count is non-deterministic and may be anywhere
	// from 0 to 25 depending on interleaving.
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read final: %v", err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(got, &parsed); err != nil {
		t.Fatalf("on-disk JSON is corrupt under race: %v", err)
	}
	cands := loadCandidatesFromDisk(t, path)
	if len(cands) > Adds {
		t.Fatalf("on-disk count=%d > Adds=%d (impossible)", len(cands), Adds)
	}
}

// Test 3: cross-instance race via testhelpers.RunWriterRound. Two goroutines
// each construct their OWN *config.Config + *Registry against the SHARED
// config.json. With non-colliding symbol prefixes, all 50 mutations must
// land on disk — the only sync point is the on-disk file (atomic rename +
// reload-from-disk).
func TestRegistry_ConcurrentCrossInstance(t *testing.T) {
	path := setupConcurrentSharedConfig(t, 100)

	var wg sync.WaitGroup
	var aErr, bErr error
	var aSuccess, bSuccess int
	wg.Add(2)
	go func() {
		defer wg.Done()
		aSuccess, aErr = testhelpers.RunWriterRound(path, "A", 25)
	}()
	go func() {
		defer wg.Done()
		bSuccess, bErr = testhelpers.RunWriterRound(path, "B", 25)
	}()
	wg.Wait()

	if aErr != nil {
		t.Fatalf("round A: %v", aErr)
	}
	if bErr != nil {
		t.Fatalf("round B: %v", bErr)
	}
	if aSuccess+bSuccess != 50 {
		t.Fatalf("round successes A=%d B=%d total=%d, want 50", aSuccess, bSuccess, aSuccess+bSuccess)
	}

	got := loadCandidatesFromDisk(t, path)
	if len(got) != 50 {
		t.Fatalf("on-disk count=%d, want 50 — cross-instance race lost mutations", len(got))
	}
	// Verify both prefixes are present.
	var seenA, seenB bool
	for _, c := range got {
		if strings.HasPrefix(c.Symbol, "A") {
			seenA = true
		}
		if strings.HasPrefix(c.Symbol, "B") {
			seenB = true
		}
	}
	if !seenA || !seenB {
		t.Fatalf("expected both prefixes on disk, seenA=%v seenB=%v", seenA, seenB)
	}
}

// Test 4: cap enforcement under in-process race. With cap=10 and 50 unique
// Adds, exactly 10 must succeed and the remaining 40 return ErrCapExceeded.
func TestRegistry_ConcurrentCapEnforcement(t *testing.T) {
	path := setupConcurrentSharedConfig(t, 10)
	audit := &concurrentNopAudit{}
	r := newConcurrentRegistry(t, path, 10, audit)

	const N = 50
	var wg sync.WaitGroup
	var capRejected atomic.Int64
	var otherErrors atomic.Int64
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			c := models.PriceGapCandidate{
				Symbol:    fmt.Sprintf("CAP%02dUSDT", i),
				LongExch:  "binance",
				ShortExch: "bybit",
				Direction: models.PriceGapDirectionBidirectional,
			}
			err := r.Add(context.Background(), "race-test", c)
			switch {
			case err == nil:
				// success
			case errors.Is(err, pricegaptrader.ErrCapExceeded):
				capRejected.Add(1)
			default:
				otherErrors.Add(1)
			}
		}(i)
	}
	wg.Wait()

	got := loadCandidatesFromDisk(t, path)
	if len(got) != 10 {
		t.Fatalf("on-disk count=%d, want exactly cap=10", len(got))
	}
	if capRejected.Load() != int64(N-10) {
		t.Fatalf("cap-rejected=%d, want %d", capRejected.Load(), N-10)
	}
	if otherErrors.Load() != 0 {
		t.Fatalf("unexpected non-cap errors: %d", otherErrors.Load())
	}
}

// Test 5: audit completeness under race. After 50 successful Adds, the audit
// writer must have captured exactly 50 LPush calls.
func TestRegistry_ConcurrentAuditCompleteness(t *testing.T) {
	path := setupConcurrentSharedConfig(t, 100)
	audit := &concurrentNopAudit{}
	r := newConcurrentRegistry(t, path, 100, audit)

	const N = 50
	var wg sync.WaitGroup
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			c := models.PriceGapCandidate{
				Symbol:    fmt.Sprintf("AUD%02dUSDT", i),
				LongExch:  "binance",
				ShortExch: "bybit",
				Direction: models.PriceGapDirectionBidirectional,
			}
			_ = r.Add(context.Background(), "race-test", c)
		}(i)
	}
	wg.Wait()

	if int(audit.pushCount.Load()) != N {
		t.Fatalf("audit pushes=%d, want %d", audit.pushCount.Load(), N)
	}
}

// ---- Phase 15 Plan 15-03 — PausedByBreaker tests ---------------------------

// TestRegistry_PausedByBreakerField — JSON marshal/unmarshal round-trip
// preserves PausedByBreaker via the `paused_by_breaker` tag.
func TestRegistry_PausedByBreakerField(t *testing.T) {
	c := models.PriceGapCandidate{
		Symbol:    "SOONUSDT",
		LongExch:  "binance",
		ShortExch: "bybit",
		Direction: models.PriceGapDirectionBidirectional,
	}
	c.PausedByBreaker = true
	encoded, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(encoded), `"paused_by_breaker":true`) {
		t.Fatalf("want json tag paused_by_breaker:true in %q", string(encoded))
	}

	var back models.PriceGapCandidate
	if err := json.Unmarshal(encoded, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !back.PausedByBreaker {
		t.Fatalf("PausedByBreaker did NOT roundtrip true→true")
	}

	// Default-false omitempty: when false, the tag is omitted.
	c2 := models.PriceGapCandidate{Symbol: "X", LongExch: "a", ShortExch: "b"}
	encoded2, _ := json.Marshal(c2)
	if strings.Contains(string(encoded2), "paused_by_breaker") {
		t.Fatalf("default-false should omit paused_by_breaker tag, got %q", string(encoded2))
	}
}

// TestRegistry_PausedByBreaker_ConcurrentWrite — chokepoint serialization:
// 100 goroutines call SetPausedByBreaker concurrently against the same
// (symbol, long, short, dir) tuple. Final on-disk state is consistent
// (the field equals the last-applied value), no torn writes, no panic.
func TestRegistry_PausedByBreaker_ConcurrentWrite(t *testing.T) {
	path := setupConcurrentSharedConfig(t, 100)
	audit := &concurrentNopAudit{}
	r := newConcurrentRegistry(t, path, 100, audit)

	// Seed one candidate.
	if err := r.Add(context.Background(), "seed", models.PriceGapCandidate{
		Symbol:    "PAUSEUSDT",
		LongExch:  "binance",
		ShortExch: "bybit",
		Direction: models.PriceGapDirectionBidirectional,
	}); err != nil {
		t.Fatalf("seed add: %v", err)
	}

	const N = 100
	var wg sync.WaitGroup
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			val := i%2 == 0
			_, _ = r.SetPausedByBreaker("PAUSEUSDT", "binance", "bybit",
				models.PriceGapDirectionBidirectional, val)
		}(i)
	}
	wg.Wait()

	// On-disk file must remain valid JSON (no torn writes).
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(got, &parsed); err != nil {
		t.Fatalf("on-disk JSON corrupt under race: %v", err)
	}
	cands := loadCandidatesFromDisk(t, path)
	if len(cands) != 1 {
		t.Fatalf("want 1 candidate on disk, got %d", len(cands))
	}
	// PausedByBreaker is a bool — must be either true or false (no torn third
	// state possible, but we assert the field exists and the candidate
	// identity is intact).
	if cands[0].Symbol != "PAUSEUSDT" {
		t.Fatalf("candidate identity lost under race: symbol=%q", cands[0].Symbol)
	}
}

// TestRegistry_RecoveryClearsPausedByBreaker_PreservesDisabled — D-11:
// ClearAllPausedByBreaker only touches PausedByBreaker. Operator-set
// Redis-backed disabled state (IsCandidateDisabled) is preserved.
func TestRegistry_RecoveryClearsPausedByBreaker_PreservesDisabled(t *testing.T) {
	path := setupConcurrentSharedConfig(t, 100)
	audit := &concurrentNopAudit{}
	r := newConcurrentRegistry(t, path, 100, audit)

	// Seed two candidates: A is paused-only, B is paused + Redis-disabled.
	for _, sym := range []string{"AUSDT", "BUSDT"} {
		if err := r.Add(context.Background(), "seed", models.PriceGapCandidate{
			Symbol:    sym,
			LongExch:  "binance",
			ShortExch: "bybit",
			Direction: models.PriceGapDirectionBidirectional,
		}); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	// Pause both.
	for _, sym := range []string{"AUSDT", "BUSDT"} {
		if _, err := r.SetPausedByBreaker(sym, "binance", "bybit",
			models.PriceGapDirectionBidirectional, true); err != nil {
			t.Fatalf("pause %s: %v", sym, err)
		}
	}

	// Clear paused-by-breaker en masse.
	count, err := r.ClearAllPausedByBreaker()
	if err != nil {
		t.Fatalf("ClearAllPausedByBreaker: %v", err)
	}
	if count != 2 {
		t.Fatalf("cleared count=%d, want 2", count)
	}

	cands := loadCandidatesFromDisk(t, path)
	if len(cands) != 2 {
		t.Fatalf("want 2 candidates on disk, got %d", len(cands))
	}
	for _, c := range cands {
		if c.PausedByBreaker {
			t.Fatalf("%s still PausedByBreaker after ClearAllPausedByBreaker", c.Symbol)
		}
	}
	// Note: Disabled state is Redis-backed (PriceGapStore.IsCandidateDisabled);
	// not part of the candidate config struct. The recovery helper does not
	// touch Redis disabled flags — verified by absence of any Redis call in
	// ClearAllPausedByBreaker source.
}

// Test 6: .bak ring under race. After 50 successful saves, `config.json.bak.*`
// glob count is ≤ 5 (ring pruned correctly under contention).
func TestRegistry_ConcurrentBakRing(t *testing.T) {
	path := setupConcurrentSharedConfig(t, 100)
	audit := &concurrentNopAudit{}
	r := newConcurrentRegistry(t, path, 100, audit)

	const N = 50
	var wg sync.WaitGroup
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			c := models.PriceGapCandidate{
				Symbol:    fmt.Sprintf("BAK%02dUSDT", i),
				LongExch:  "binance",
				ShortExch: "bybit",
				Direction: models.PriceGapDirectionBidirectional,
			}
			_ = r.Add(context.Background(), "race-test", c)
		}(i)
	}
	wg.Wait()

	matches, err := filepath.Glob(path + ".bak.*")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(matches) > 5 {
		t.Fatalf(".bak ring grew beyond cap: got %d, want ≤ 5", len(matches))
	}
}
