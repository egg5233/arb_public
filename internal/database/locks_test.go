package database

import (
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
)

func TestOwnedLockReleaseDoesNotDeleteReacquiredLease(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	db, err := New(mr.Addr(), "", 0)
	if err != nil {
		t.Fatalf("database.New: %v", err)
	}

	first, ok, err := db.AcquireOwnedLock("spot_entry", 5*time.Second)
	if err != nil {
		t.Fatalf("AcquireOwnedLock first: %v", err)
	}
	if !ok {
		t.Fatal("first lock was not acquired")
	}

	mr.FastForward(6 * time.Second)

	second, ok, err := db.AcquireOwnedLock("spot_entry", 5*time.Second)
	if err != nil {
		t.Fatalf("AcquireOwnedLock second: %v", err)
	}
	if !ok {
		t.Fatal("second lock was not acquired after lease expiry")
	}

	if err := first.Release(); err != nil {
		t.Fatalf("first Release: %v", err)
	}

	third, ok, err := db.AcquireOwnedLock("spot_entry", 5*time.Second)
	if err != nil {
		t.Fatalf("AcquireOwnedLock third: %v", err)
	}
	if ok {
		_ = third.Release()
		t.Fatal("stale release deleted the reacquired lease")
	}

	if err := second.Release(); err != nil {
		t.Fatalf("second Release: %v", err)
	}
}

func TestOwnedLockCheckDetectsLostLease(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	db, err := New(mr.Addr(), "", 0)
	if err != nil {
		t.Fatalf("database.New: %v", err)
	}

	first, ok, err := db.AcquireOwnedLock("spot_entry", 5*time.Second)
	if err != nil {
		t.Fatalf("AcquireOwnedLock first: %v", err)
	}
	if !ok {
		t.Fatal("first lock was not acquired")
	}

	mr.FastForward(6 * time.Second)

	second, ok, err := db.AcquireOwnedLock("spot_entry", 5*time.Second)
	if err != nil {
		t.Fatalf("AcquireOwnedLock second: %v", err)
	}
	if !ok {
		t.Fatal("second lock was not acquired after lease expiry")
	}
	defer func() {
		if err := second.Release(); err != nil {
			t.Fatalf("second Release: %v", err)
		}
	}()

	err = first.Check()
	if err == nil {
		t.Fatal("first Check should fail after the lease was lost")
	}
	if got, want := err.Error(), "no longer owned"; !strings.Contains(got, want) {
		t.Fatalf("first Check error = %q, want substring %q", got, want)
	}

	if err := first.Check(); err == nil {
		t.Fatal("first Check should stay failed after the first lease-loss signal")
	}
}
