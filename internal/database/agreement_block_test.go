package database

import (
	"testing"

	"github.com/alicebob/miniredis/v2"
)

func TestAgreementBlock_RoundTrip(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	db, err := New(mr.Addr(), "", 0)
	if err != nil {
		t.Fatalf("database.New: %v", err)
	}

	// Initially not blocked.
	if db.IsAgreementBlocked("bybit", "HOODUSDT") {
		t.Error("expected not blocked before set")
	}

	// Set a block.
	if err := db.SetAgreementBlock("bybit", "HOODUSDT", "110126: must sign agreement"); err != nil {
		t.Fatalf("SetAgreementBlock: %v", err)
	}

	// Should be blocked now.
	if !db.IsAgreementBlocked("bybit", "HOODUSDT") {
		t.Error("expected IsAgreementBlocked=true after set")
	}

	// List should return the block.
	blocks := db.ListAgreementBlocks()
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	b := blocks[0]
	if b.Exchange != "bybit" {
		t.Errorf("exchange: got %q, want %q", b.Exchange, "bybit")
	}
	if b.Symbol != "HOODUSDT" {
		t.Errorf("symbol: got %q, want %q", b.Symbol, "HOODUSDT")
	}
	if b.Reason != "110126: must sign agreement" {
		t.Errorf("reason: got %q, want %q", b.Reason, "110126: must sign agreement")
	}
	if b.BlockedAt.IsZero() {
		t.Error("expected BlockedAt to be set")
	}

	// Clear the block.
	if err := db.ClearAgreementBlock("bybit", "HOODUSDT"); err != nil {
		t.Fatalf("ClearAgreementBlock: %v", err)
	}

	// Should be gone now.
	if db.IsAgreementBlocked("bybit", "HOODUSDT") {
		t.Error("expected not blocked after clear")
	}
	if len(db.ListAgreementBlocks()) != 0 {
		t.Error("expected empty list after clear")
	}
}

func TestAgreementBlock_MultipleSymbols(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	db, err := New(mr.Addr(), "", 0)
	if err != nil {
		t.Fatalf("database.New: %v", err)
	}

	symbols := []string{"HOODUSDT", "SOONUSDT", "MEWUSDT"}
	for _, sym := range symbols {
		if err := db.SetAgreementBlock("bybit", sym, "test"); err != nil {
			t.Fatalf("SetAgreementBlock %s: %v", sym, err)
		}
	}

	blocks := db.ListAgreementBlocks()
	if len(blocks) != 3 {
		t.Fatalf("expected 3 blocks, got %d", len(blocks))
	}
}

func TestAgreementBlock_NoTTL(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	db, err := New(mr.Addr(), "", 0)
	if err != nil {
		t.Fatalf("database.New: %v", err)
	}

	if err := db.SetAgreementBlock("bybit", "TESTUSDT", "no ttl test"); err != nil {
		t.Fatalf("SetAgreementBlock: %v", err)
	}

	// Check that no TTL is set (miniredis returns -1 for no TTL).
	key := keyAgreementBlockPrefix + "bybit:TESTUSDT"
	ttl := mr.TTL(key)
	if ttl != 0 { // miniredis returns 0 for no expiry
		t.Errorf("expected no TTL (0), got %v", ttl)
	}
}

func TestAgreementBlock_DifferentExchanges(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	db, err := New(mr.Addr(), "", 0)
	if err != nil {
		t.Fatalf("database.New: %v", err)
	}

	// Block on bybit only — binance same symbol should not be blocked.
	if err := db.SetAgreementBlock("bybit", "HOODUSDT", "bybit-only"); err != nil {
		t.Fatalf("SetAgreementBlock: %v", err)
	}

	if !db.IsAgreementBlocked("bybit", "HOODUSDT") {
		t.Error("expected bybit HOODUSDT to be blocked")
	}
	if db.IsAgreementBlocked("binance", "HOODUSDT") {
		t.Error("expected binance HOODUSDT to NOT be blocked")
	}
}
