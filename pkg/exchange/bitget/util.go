package bitget

// containsNonASCII reports whether s contains any non-ASCII runes.
// Kept in the bitget package (per PLAN-bitget-error-handling.md) rather than
// pkg/utils so the helper stays co-located with the non-ASCII fallback paths
// that depend on it.
func containsNonASCII(s string) bool {
	for _, r := range s {
		if r > 127 {
			return true
		}
	}
	return false
}
