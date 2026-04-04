package api

import (
	"testing"
	"time"

	"arb/internal/database"

	"github.com/alicebob/miniredis/v2"
)

func TestAuthStoreSessionSurvivesRestartViaRedis(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	db, err := database.New(mr.Addr(), "", 0)
	if err != nil {
		t.Fatalf("database.New: %v", err)
	}

	first := newAuthStore(db)
	token, err := first.createSession()
	if err != nil {
		t.Fatalf("createSession: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}

	second := newAuthStore(db)
	if !second.validate(token) {
		t.Fatal("expected redis-backed session to validate after restart")
	}
}

func TestAuthStoreSessionExpiresAfterTTL(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	db, err := database.New(mr.Addr(), "", 0)
	if err != nil {
		t.Fatalf("database.New: %v", err)
	}

	store := newAuthStore(db)
	token, err := store.createSession()
	if err != nil {
		t.Fatalf("createSession: %v", err)
	}

	mr.FastForward(sessionTTL + time.Minute)

	restarted := newAuthStore(db)
	if restarted.validate(token) {
		t.Fatal("expected expired redis-backed session to be rejected")
	}
}
