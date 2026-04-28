// scanner_static_test.go — Compile-time read-only invariant for the scanner.
//
// Phase 11 / D-13 / T-11-26: Plan 04 ships the scanner with RegistryReader
// (read-only). This file enforces that invariant by greppinng scanner.go's
// source and refusing references to the concrete *Registry type or any of
// its mutator method names. If a future change accidentally widens the
// dependency back to *Registry the test fails before review.
package pricegaptrader

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// TestScanner_NoRegistryMutators reads internal/pricegaptrader/scanner.go and
// asserts:
//   - The bare token `*Registry` (concrete type) does NOT appear — only the
//     interface `RegistryReader` is permitted.
//   - No call to registry.Add(, registry.Update(, registry.Delete(, or
//     registry.Replace(, or to s.registry.<mutator>( on the receiver.
//
// Comments are allowed to mention the words; the regex matches the typed
// token form `\*Registry\b` and the call-form `registry\.(Add|Update|Delete|Replace)\(`.
func TestScanner_NoRegistryMutators(t *testing.T) {
	src, err := os.ReadFile("scanner.go")
	if err != nil {
		t.Fatalf("read scanner.go: %v", err)
	}

	// Strip line comments before matching so legitimate prose mentioning
	// "*Registry" in comments doesn't trigger the regex. Block comments are
	// rare in this package; if used, the comment-trim helper below also
	// excises them.
	stripped := stripGoComments(string(src))

	if regexp.MustCompile(`\*Registry\b`).MatchString(stripped) {
		t.Errorf("scanner.go references concrete *Registry — must use RegistryReader interface (D-13 / T-11-26)")
	}

	mutatorRe := regexp.MustCompile(`registry\.(Add|Update|Delete|Replace)\(`)
	if mutatorRe.MatchString(stripped) {
		t.Errorf("scanner.go calls a Registry mutator — must be read-only (D-13 / T-11-26)")
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
