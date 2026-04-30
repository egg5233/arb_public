// scanner_static_test.go — Compile-time chokepoint invariant for the scanner.
//
// Phase 11 / D-13 / T-11-26 (original) shipped this test forbidding `*Registry`
// in scanner.go to enforce that Plan 04 only consumed the read-only
// RegistryReader. Phase 12 D-17 widens scanner.go to hold *Registry so the
// PromotionController (now reachable via s.promotion.Apply) can call Add and
// Delete on the chokepoint. The chokepoint discipline is preserved by a
// different invariant:
//
//   - PromotionController has NO *config.Config-mutation surface (it only
//     READS PriceGapAutoPromoteScore + PriceGapMaxCandidates under cfg.RLock,
//     never assigns cfg.PriceGapCandidates), so the only way scanner.go could
//     bypass the chokepoint is via a raw `cfg.PriceGapCandidates =` or
//     `s.cfg.PriceGapCandidates =` assignment.
//
// This file enforces the new (relaxed) invariant by greppinng scanner.go for
// the raw-mutation pattern and rejecting it. The original `*Registry` and
// `registry.(Add|Update|Delete|Replace)(` checks are intentionally REMOVED;
// those calls are now legitimate via the PromotionController path.
//
// What is still forbidden in scanner.go:
//   - Raw assignment to cfg.PriceGapCandidates (any receiver — `cfg.`, `s.cfg.`).
//
// What is now permitted:
//   - Holding `*Registry` as a field (s.registry is *Registry per D-17).
//   - Calling s.promotion.Apply(ctx, summary), which transitively calls
//     s.registry.Add / s.registry.Delete — the chokepoint.
package pricegaptrader

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// TestScanner_NoRawCfgCandidatesAssignment asserts that scanner.go never
// directly writes cfg.PriceGapCandidates. The only legitimate writers in
// Phase 12 are:
//   - dashboard / pg-admin (via internal/api → pgRegistry chokepoint)
//   - PromotionController (via pgRegistry.Add/Delete chokepoint)
//
// Both paths route through *Registry, which under cfg.Lock applies the diff.
// The scanner package has no other path to mutate cfg.PriceGapCandidates;
// this regex enforces that contract at the source-text level.
func TestScanner_NoRawCfgCandidatesAssignment(t *testing.T) {
	src, err := os.ReadFile("scanner.go")
	if err != nil {
		t.Fatalf("read scanner.go: %v", err)
	}

	// Strip line + block comments before matching so legitimate prose mentions
	// of forbidden patterns inside comments (this very file's doc block,
	// scanner.go's package comment) do not trigger the regex.
	stripped := stripGoComments(string(src))

	// Forbid `cfg.PriceGapCandidates = ...` and `s.cfg.PriceGapCandidates = ...`
	// or any other receiver-prefixed form. The `\s*=` (without `==`) excludes
	// equality comparisons.
	rawAssignRe := regexp.MustCompile(`PriceGapCandidates\s*=[^=]`)
	if rawAssignRe.MatchString(stripped) {
		t.Errorf("scanner.go contains raw `cfg.PriceGapCandidates =` assignment — must use *Registry chokepoint via PromotionController (Phase 12 D-17)")
	}
}

// stripGoComments removes // line comments and /* */ block comments from src.
// It is deliberately simple — the goal is to keep prose mentions of forbidden
// tokens out of the regex. String literals are NOT preserved; the package
// avoids embedding regex-trip-words inside string literals so a naive strip
// is sufficient for this static test.
func stripGoComments(src string) string {
	var b strings.Builder
	i := 0
	n := len(src)
	for i < n {
		// Block comment.
		if i+1 < n && src[i] == '/' && src[i+1] == '*' {
			j := strings.Index(src[i+2:], "*/")
			if j < 0 {
				break
			}
			i = i + 2 + j + 2
			continue
		}
		// Line comment.
		if i+1 < n && src[i] == '/' && src[i+1] == '/' {
			j := strings.Index(src[i:], "\n")
			if j < 0 {
				break
			}
			i += j + 1
			continue
		}
		b.WriteByte(src[i])
		i++
	}
	return b.String()
}
