package api

import "testing"

func TestVersionNewer(t *testing.T) {
	tests := []struct {
		remote string
		local  string
		want   bool
	}{
		// remote newer
		{"0.22.31", "0.22.30", true},
		{"0.23.0", "0.22.30", true},
		{"1.0.0", "0.22.30", true},
		// same version
		{"0.22.30", "0.22.30", false},
		// local newer (should NOT report update)
		{"0.22.25", "0.22.30", false},
		{"0.21.0", "0.22.30", false},
		// edge: unparseable falls back to inequality
		{"unknown", "0.22.30", true},
		{"0.22.30", "unknown", true},
		{"unknown", "unknown", false},
	}
	for _, tt := range tests {
		got := versionNewer(tt.remote, tt.local)
		if got != tt.want {
			t.Errorf("versionNewer(%q, %q) = %v, want %v", tt.remote, tt.local, got, tt.want)
		}
	}
}
