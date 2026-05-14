package engine

import (
	"errors"
	"testing"
)

func TestIsBybitAgreementError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil error", nil, false},
		{"110126 code", errors.New("ret_code=110126: You must sign the required agreement"), true},
		{"110126 lowercase", errors.New("ret_code=110126"), true},
		{"sign phrase", errors.New("You must sign the required agreement before trading"), true},
		{"sign phrase lowercase", errors.New("you must sign the required agreement before trading this contract"), true},
		{"generic bybit error", errors.New("ret_code=10001: Params Error"), false},
		{"rate limit error", errors.New("too many requests"), false},
		{"empty string", errors.New(""), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := isBybitAgreementError(tc.err)
			if got != tc.want {
				t.Errorf("isBybitAgreementError(%q) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}
