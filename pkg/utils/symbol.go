package utils

import "regexp"

var validBaseSymbol = regexp.MustCompile(`^[A-Z0-9]+$`)

// IsValidBaseSymbol returns true if sym contains only A-Z and 0-9.
func IsValidBaseSymbol(sym string) bool {
	return validBaseSymbol.MatchString(sym)
}
