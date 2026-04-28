package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// seedConfigFileForBakRing writes a minimal valid config.json into the given
// path so that SaveJSONWithBakRing finds an originalData payload to back up.
// Returns the path that was written.
func seedConfigFileForBakRing(t *testing.T, path string) {
	t.Helper()
	seed := map[string]interface{}{
		"strategy": map[string]interface{}{},
		"price_gap": map[string]interface{}{
			"enabled":    false,
			"paper_mode": true,
		},
	}
	b, err := json.MarshalIndent(seed, "", "  ")
	if err != nil {
		t.Fatalf("marshal seed: %v", err)
	}
	if err := os.WriteFile(path, b, 0644); err != nil {
		t.Fatalf("write seed: %v", err)
	}
}

// loadConfigForBakRingTest returns a *Config with CONFIG_FILE set to the seed
// path. Tests can hold cfg.Lock() and call SaveJSONWithBakRing directly.
func loadConfigForBakRingTest(t *testing.T) (*Config, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	t.Setenv("CONFIG_FILE", path)
	seedConfigFileForBakRing(t, path)
	cfg := Load()
	return cfg, path
}

// listBakFiles returns the basenames of `<path>.bak.<ts>` files (NOT the
// legacy plain `<path>.bak` from SaveJSON), sorted by ts ascending. Skips the
// plain `.bak` entry — only the timestamped ring is asserted on.
func listBakFiles(t *testing.T, path string) []string {
	t.Helper()
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	prefix := base + ".bak."
	var found []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, prefix) {
			found = append(found, name)
		}
	}
	sort.Slice(found, func(i, j int) bool {
		ti, _ := strconv.ParseInt(strings.TrimPrefix(found[i], prefix), 10, 64)
		tj, _ := strconv.ParseInt(strings.TrimPrefix(found[j], prefix), 10, 64)
		return ti < tj
	})
	return found
}

func TestSaveJSONWithBakRing_Prunes(t *testing.T) {
	cfg, path := loadConfigForBakRingTest(t)

	// Inject a deterministic nowFunc — emit unix_ts at one-second intervals so
	// every save produces a unique .bak.{ts} filename.
	cfg.Lock()
	start := time.Now()
	count := 0
	cfg.nowFunc = func() time.Time {
		count++
		return start.Add(time.Duration(count) * time.Second)
	}
	cfg.Unlock()

	for i := 0; i < 7; i++ {
		cfg.Lock()
		if err := cfg.SaveJSONWithBakRing(); err != nil {
			cfg.Unlock()
			t.Fatalf("save iter %d: %v", i, err)
		}
		cfg.Unlock()
	}

	got := listBakFiles(t, path)
	if len(got) != 5 {
		t.Fatalf("expected 5 .bak.{ts} files after 7 saves, got %d (%v)", len(got), got)
	}
}

func TestSaveJSONWithBakRing_TmpRemoved(t *testing.T) {
	cfg, path := loadConfigForBakRingTest(t)

	cfg.Lock()
	if err := cfg.SaveJSONWithBakRing(); err != nil {
		cfg.Unlock()
		t.Fatalf("save: %v", err)
	}
	cfg.Unlock()

	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("expected %s.tmp to NOT exist after successful save, stat err=%v", path, err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config.json: %v", err)
	}
	if len(got) == 0 {
		t.Fatalf("config.json is empty after save")
	}
}

func TestSaveJSONWithBakRing_AtomicityOnRenameFailure(t *testing.T) {
	cfg, path := loadConfigForBakRingTest(t)

	// Capture the prior content (the seed payload that's currently on disk).
	priorContent, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read prior: %v", err)
	}

	// Inject a renameFunc that ALWAYS fails.
	cfg.Lock()
	cfg.renameFunc = func(oldpath, newpath string) error {
		return errors.New("injected rename failure")
	}
	// Mutate something so the would-be save differs from prior.
	cfg.PriceGapDiscoveryEnabled = true
	cfg.Unlock()

	cfg.Lock()
	err = cfg.SaveJSONWithBakRing()
	cfg.Unlock()
	if err == nil {
		t.Fatalf("expected save to fail when rename injected error, got nil")
	}

	// On-disk config.json should be unchanged (atomic — failed rename leaves
	// prior file intact).
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read post-fail: %v", err)
	}
	if string(got) != string(priorContent) {
		t.Fatalf("on-disk config changed despite rename failure\nprior: %q\nnow:   %q", priorContent, got)
	}

	// The .tmp leftover should be cleaned up after the rename failure.
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("expected %s.tmp to be cleaned up on rename failure, stat err=%v", path, err)
	}
}

func TestSaveJSONWithBakRing_ConcurrentSerial(t *testing.T) {
	cfg, path := loadConfigForBakRingTest(t)

	// nowFunc emits monotonically increasing ts so each save gets a distinct
	// .bak.{ts}.
	cfg.Lock()
	start := time.Now()
	tick := int64(0)
	cfg.nowFunc = func() time.Time {
		tick++
		return start.Add(time.Duration(tick) * time.Second)
	}
	cfg.Unlock()

	// Serialize 10 saves through cfg.Lock() (caller-holds-lock contract).
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			cfg.Lock()
			defer cfg.Unlock()
			if err := cfg.SaveJSONWithBakRing(); err != nil {
				t.Errorf("save %d: %v", i, err)
			}
		}(i)
	}
	wg.Wait()

	got := listBakFiles(t, path)
	if len(got) != 5 {
		t.Fatalf("expected ring pruned to 5 after 10 concurrent saves, got %d (%v)", len(got), got)
	}
}

func TestSaveJSONWithBakRing_KeepNonZeroTripwirePreserved(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	t.Setenv("CONFIG_FILE", path)

	// Seed config with a non-zero spot_futures.max_positions to exercise the
	// keepNonZero tripwire.
	seed := map[string]interface{}{
		"strategy": map[string]interface{}{},
		"spot_futures": map[string]interface{}{
			"max_positions":         5,
			"leverage":              3,
			"capital_unified_usdt":  100.0,
			"capital_separate_usdt": 50.0,
		},
		"price_gap": map[string]interface{}{},
	}
	b, _ := json.MarshalIndent(seed, "", "  ")
	if err := os.WriteFile(path, b, 0644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	cfg := Load()
	// In-memory copy has zero (the load path preserves the JSON value, but for
	// the purposes of this test we explicitly clobber the in-memory value to
	// simulate a buggy update path that tries to zero a critical field).
	cfg.Lock()
	cfg.SpotFuturesMaxPositions = 0
	cfg.Unlock()

	cfg.Lock()
	if err := cfg.SaveJSONWithBakRing(); err != nil {
		cfg.Unlock()
		t.Fatalf("save: %v", err)
	}
	cfg.Unlock()

	// On-disk file should still carry the non-zero value (tripwire preserved).
	on, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(on, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	sf, ok := raw["spot_futures"].(map[string]interface{})
	if !ok {
		t.Fatalf("spot_futures missing")
	}
	mp, _ := sf["max_positions"].(float64)
	if mp != 5 {
		t.Fatalf("expected keepNonZero to preserve max_positions=5, got %v", sf["max_positions"])
	}
}

func TestSaveJSONWithBakRing_FilenamesSortableByUnixTs(t *testing.T) {
	cfg, path := loadConfigForBakRingTest(t)

	// Distinct unix_ts via injected nowFunc.
	cfg.Lock()
	tick := int64(1700000000)
	cfg.nowFunc = func() time.Time {
		tick++
		return time.Unix(tick, 0)
	}
	cfg.Unlock()

	for i := 0; i < 3; i++ {
		cfg.Lock()
		if err := cfg.SaveJSONWithBakRing(); err != nil {
			cfg.Unlock()
			t.Fatalf("save %d: %v", i, err)
		}
		cfg.Unlock()
	}

	got := listBakFiles(t, path)
	if len(got) != 3 {
		t.Fatalf("expected 3 .bak.{ts} files, got %d (%v)", len(got), got)
	}

	prefix := filepath.Base(path) + ".bak."
	var nums []int64
	for _, name := range got {
		n, err := strconv.ParseInt(strings.TrimPrefix(name, prefix), 10, 64)
		if err != nil {
			t.Fatalf("filename %q does not have a parseable unix_ts suffix: %v", name, err)
		}
		nums = append(nums, n)
	}
	for i := 1; i < len(nums); i++ {
		if nums[i] <= nums[i-1] {
			t.Fatalf("expected ascending unix_ts in sorted list, got %v", nums)
		}
	}
	// Sanity: format pattern is exactly `<base>.bak.<digits>`.
	last := got[len(got)-1]
	if !strings.HasPrefix(last, prefix) {
		t.Fatalf("last filename missing prefix: %q", last)
	}
	suffix := strings.TrimPrefix(last, prefix)
	if _, err := strconv.ParseInt(suffix, 10, 64); err != nil {
		t.Fatalf("suffix %q is not a unix_ts integer: %v", suffix, err)
	}
}

// TestBakRing_PruneNewestFiveRetained verifies that prune deletes the oldest
// entries beyond index 4, keeping the newest 5.
func TestBakRing_PruneNewestFiveRetained(t *testing.T) {
	cfg, path := loadConfigForBakRingTest(t)
	dir := filepath.Dir(path)

	// Pre-create 7 timestamped bak files spanning a wide time range.
	for i := 1; i <= 7; i++ {
		ts := int64(1700000000 + i)
		name := fmt.Sprintf("%s.bak.%d", filepath.Base(path), ts)
		if err := os.WriteFile(filepath.Join(dir, name), []byte("dummy"), 0644); err != nil {
			t.Fatalf("seed bak %d: %v", i, err)
		}
	}

	// Now trigger a prune by calling the helper directly via a save (which
	// will add an 8th entry then prune to 5).
	cfg.Lock()
	cfg.nowFunc = func() time.Time { return time.Unix(1700000099, 0) }
	if err := cfg.SaveJSONWithBakRing(); err != nil {
		cfg.Unlock()
		t.Fatalf("save: %v", err)
	}
	cfg.Unlock()

	got := listBakFiles(t, path)
	if len(got) != 5 {
		t.Fatalf("expected 5 retained, got %d (%v)", len(got), got)
	}

	// Newest 5 timestamps should be 1700000004..1700000007 + 1700000099.
	prefix := filepath.Base(path) + ".bak."
	var nums []int64
	for _, name := range got {
		n, _ := strconv.ParseInt(strings.TrimPrefix(name, prefix), 10, 64)
		nums = append(nums, n)
	}
	want := []int64{1700000004, 1700000005, 1700000006, 1700000007, 1700000099}
	if len(nums) != len(want) {
		t.Fatalf("nums len mismatch: got %v want %v", nums, want)
	}
	for i, n := range nums {
		if n != want[i] {
			t.Fatalf("nums[%d]=%d want %d (full=%v)", i, n, want[i], nums)
		}
	}
}
