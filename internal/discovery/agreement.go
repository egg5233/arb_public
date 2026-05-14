package discovery

import "arb/internal/database"

// SetAgreementBlock persistently blocks a symbol on an exchange from discovery
// after receiving Bybit API error 110126 (must sign required agreement).
// The block has no TTL; it persists until cleared by the operator via the dashboard.
func (s *Scanner) SetAgreementBlock(exchange, symbol, reason string) {
	if err := s.db.SetAgreementBlock(exchange, symbol, reason); err != nil {
		s.log.Warn("agreement block: failed to persist block for %s/%s: %v", exchange, symbol, err)
	}
}

// IsAgreementBlocked reports whether the given exchange+symbol pair is
// blocked due to an unsigned Bybit contract agreement.
func (s *Scanner) IsAgreementBlocked(exchange, symbol string) bool {
	return s.db.IsAgreementBlocked(exchange, symbol)
}

// ListAgreementBlocks returns all currently blocked exchange+symbol pairs.
func (s *Scanner) ListAgreementBlocks() []database.AgreementBlock {
	return s.db.ListAgreementBlocks()
}

// ClearAgreementBlock removes the agreement block for an exchange+symbol pair,
// re-admitting it to discovery. Called from the dashboard "Clear" button.
func (s *Scanner) ClearAgreementBlock(exchange, symbol string) {
	if err := s.db.ClearAgreementBlock(exchange, symbol); err != nil {
		s.log.Warn("agreement block: failed to clear block for %s/%s: %v", exchange, symbol, err)
	}
}
