package pricegaptrader

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestStaticCheck_NoDirectPaperModeRead — Phase 15 Pattern 4 regression guard.
//
// The chokepoint is Tracker.IsPaperModeActive(ctx). No production .go file in
// internal/pricegaptrader/ may read cfg.PriceGapPaperMode directly, EXCEPT
// tracker.go (where the helper lives) — it consults the cfg flag as the first
// check inside IsPaperModeActive itself.
//
// Mirrors scanner_static_test.go shape. Forbids future code paths from
// bypassing the breaker's sticky paper-mode guarantee.
func TestStaticCheck_NoDirectPaperModeRead(t *testing.T) {
	re := regexp.MustCompile(`cfg\.PriceGapPaperMode\b`)
	allowedFile := "tracker.go" // helper definition lives here

	err := filepath.WalkDir("./", func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		if strings.HasSuffix(path, "_test.go") {
			return nil // tests legitimately read/set the cfg flag
		}
		if filepath.Base(path) == allowedFile {
			return nil
		}
		b, _ := os.ReadFile(path)
		if re.Match(b) {
			t.Errorf("forbidden direct cfg.PriceGapPaperMode read in %s — must use Tracker.IsPaperModeActive(ctx) (Phase 15 D-07 chokepoint)", path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
}
