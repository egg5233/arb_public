package database

import (
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
