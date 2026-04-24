package engine

import (
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"arb/pkg/exchange"
	"arb/pkg/utils"
)

// delayedSpotStub is a minimal exchange stub whose GetSpotBalance controls
// the observed spot balance per-call. Only GetSpotBalance is implemented;
// other Exchange methods are NOT overridden and will panic if called —
// waitForSpotBalance MUST NOT exercise them.
type delayedSpotStub struct {
	exchange.Exchange // embed so unused methods are "valid" types; any actual call panics
	preSettle         float64
	target            float64
	delayPolls        int32 // atomic
	calls             int32 // atomic
	errOnPoll         int32 // atomic; if >0, this many polls return an error first
}

func (s *delayedSpotStub) GetSpotBalance() (*exchange.Balance, error) {
	n := atomic.AddInt32(&s.calls, 1)
	if atomic.LoadInt32(&s.errOnPoll) > 0 {
		atomic.AddInt32(&s.errOnPoll, -1)
		return nil, errors.New("transient network error")
	}
	if n <= atomic.LoadInt32(&s.delayPolls) {
		return &exchange.Balance{Available: s.preSettle, Total: s.preSettle}, nil
	}
	return &exchange.Balance{Available: s.target, Total: s.target}, nil
}

// singleReadStub returns an error on the first GetSpotBalance call,
// and a fixed value thereafter. Used to verify captureSpotBalanceForTransfer
// returns an error instead of using snapshotSpot or a later successful read.
type singleReadStub struct {
	exchange.Exchange
	value float64
	calls int32 // atomic
}

func (s *singleReadStub) GetSpotBalance() (*exchange.Balance, error) {
	n := atomic.AddInt32(&s.calls, 1)
	if n == 1 {
		return nil, errors.New("transient pre-read error")
	}
	return &exchange.Balance{Available: s.value, Total: s.value}, nil
}

// fixedReadStub returns a fixed spot balance on every call.
type fixedReadStub struct {
	exchange.Exchange
	value float64
	calls int32 // atomic
}

func (s *fixedReadStub) GetSpotBalance() (*exchange.Balance, error) {
	atomic.AddInt32(&s.calls, 1)
	return &exchange.Balance{Available: s.value, Total: s.value}, nil
}

// newWaitTestEngine returns the minimal *Engine fixture required by
// waitForSpotBalance + captureSpotBalanceForTransfer.
func newWaitTestEngine(exchanges map[string]exchange.Exchange) *Engine {
	return &Engine{
		exchanges: exchanges,
		log:       utils.NewLogger("test-wait-spot"),
		stopCh:    make(chan struct{}),
	}
}

// TestWaitForSpotBalance_EventualSuccess verifies the poll loop succeeds
// when the target balance appears after a few zero-reads.
func TestWaitForSpotBalance_EventualSuccess(t *testing.T) {
	stub := &delayedSpotStub{target: 120.75, delayPolls: 2}
	eng := newWaitTestEngine(map[string]exchange.Exchange{"binance": stub})

	start := time.Now()
	bal, err := eng.waitForSpotBalance("binance", 120.75, 10*time.Second)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("expected nil err after delayed balance appears, got: %v", err)
	}
	if bal == nil || bal.Available < 120.75 {
		t.Fatalf("expected returned bal.Available >= 120.75, got: %+v", bal)
	}
	if atomic.LoadInt32(&stub.calls) < 3 {
		t.Errorf("expected at least 3 GetSpotBalance calls (2 zero + 1 success), got %d", stub.calls)
	}
	if elapsed < 2*time.Second {
		t.Errorf("expected at least 2s elapsed (2 polls), got %s", elapsed)
	}
	if elapsed > 5*time.Second {
		t.Errorf("expected less than 5s elapsed, got %s (should succeed quickly)", elapsed)
	}
}

// TestWaitForSpotBalance_TransientError verifies the poll loop recovers
// after a transient GetSpotBalance error and still succeeds within timeout.
func TestWaitForSpotBalance_TransientError(t *testing.T) {
	stub := &delayedSpotStub{target: 50.0, delayPolls: 0, errOnPoll: 2}
	eng := newWaitTestEngine(map[string]exchange.Exchange{"binance": stub})

	start := time.Now()
	bal, err := eng.waitForSpotBalance("binance", 50.0, 10*time.Second)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("expected nil err after transient error recovers, got: %v", err)
	}
	if bal == nil || bal.Available < 50.0 {
		t.Fatalf("expected returned bal.Available >= 50, got: %+v", bal)
	}
	if atomic.LoadInt32(&stub.calls) < 3 {
		t.Errorf("expected at least 3 GetSpotBalance calls (2 err + 1 success), got %d", stub.calls)
	}
	if elapsed > 5*time.Second {
		t.Errorf("expected success within 5s, got %s", elapsed)
	}
}

// TestWaitForSpotBalance_PreExistingSpotBelowTarget verifies that when the
// donor has pre-existing spot=100 but target=200, the helper keeps polling
// until the spot actually reaches 200 (not just any non-zero value).
func TestWaitForSpotBalance_PreExistingSpotBelowTarget(t *testing.T) {
	stub := &delayedSpotStub{preSettle: 100.0, target: 200.0, delayPolls: 3}
	eng := newWaitTestEngine(map[string]exchange.Exchange{"binance": stub})

	bal, err := eng.waitForSpotBalance("binance", 200.0, 10*time.Second)
	if err != nil {
		t.Fatalf("expected eventual success, got err: %v", err)
	}
	if bal == nil || bal.Available < 200.0 {
		t.Fatalf("expected returned bal.Available >= 200, got: %+v", bal)
	}
	if atomic.LoadInt32(&stub.calls) < 4 {
		t.Errorf("expected at least 4 polls (3 pre-settle + 1 settled), got %d", stub.calls)
	}
}

// TestWaitForSpotBalance_Timeout verifies timeout returns an error.
func TestWaitForSpotBalance_Timeout(t *testing.T) {
	stub := &delayedSpotStub{target: 120.75, delayPolls: 9999} // never satisfies
	eng := newWaitTestEngine(map[string]exchange.Exchange{"binance": stub})

	start := time.Now()
	bal, err := eng.waitForSpotBalance("binance", 120.75, 3*time.Second)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if bal != nil {
		t.Errorf("expected nil balance on timeout, got: %+v", bal)
	}
	if !strings.Contains(err.Error(), "timeout") {
		t.Errorf("expected error message to mention timeout, got: %v", err)
	}
	if elapsed < 3*time.Second {
		t.Errorf("expected at least 3s elapsed (timeout), got %s", elapsed)
	}
	if elapsed > 5*time.Second {
		t.Errorf("expected less than 5s elapsed (timeout should fire promptly), got %s", elapsed)
	}
}

// TestWaitForSpotBalance_UnknownDonor verifies the helper fails fast when
// the donor key is not in e.exchanges (bug guard).
func TestWaitForSpotBalance_UnknownDonor(t *testing.T) {
	eng := newWaitTestEngine(map[string]exchange.Exchange{})
	_, err := eng.waitForSpotBalance("nonexistent", 100, 1*time.Second)
	if err == nil {
		t.Fatal("expected error for unknown donor, got nil")
	}
	if !strings.Contains(err.Error(), "nonexistent") {
		t.Errorf("expected error to contain donor name, got: %v", err)
	}
}

// TestWaitForSpotBalance_StopAware verifies the poll loop returns promptly
// when e.stopCh is closed (engine shutdown).
func TestWaitForSpotBalance_StopAware(t *testing.T) {
	stub := &delayedSpotStub{preSettle: 0, target: 100, delayPolls: 9999} // never satisfies
	eng := newWaitTestEngine(map[string]exchange.Exchange{"binance": stub})

	go func() {
		time.Sleep(1500 * time.Millisecond)
		close(eng.stopCh)
	}()

	start := time.Now()
	bal, err := eng.waitForSpotBalanceWithInterval("binance", 100, 5*time.Second, 500*time.Millisecond)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected stop-aware error, got nil")
	}
	if bal != nil {
		t.Errorf("expected nil balance on stop, got: %+v", bal)
	}
	if !strings.Contains(err.Error(), "stopped") {
		t.Errorf("expected error to mention stop, got: %v", err)
	}
	if elapsed >= 4*time.Second {
		t.Errorf("expected stop-aware abort before 4s (timeout=5s), got %s", elapsed)
	}
}

// TestCaptureSpotBalanceForTransfer_Success verifies that when the
// GetSpotBalance read succeeds, the helper returns the live Available
// value (not the snapshot).
func TestCaptureSpotBalanceForTransfer_Success(t *testing.T) {
	stub := &fixedReadStub{value: 150.0}
	eng := newWaitTestEngine(map[string]exchange.Exchange{"binance": stub})

	pre, err := eng.captureSpotBalanceForTransfer("binance", 100.0) // snapshot=100, live=150
	if err != nil {
		t.Fatalf("expected nil err, got: %v", err)
	}
	if pre != 150.0 {
		t.Errorf("expected pre=150 (live read preferred over snapshot), got %f", pre)
	}
}

// TestCaptureSpotBalanceForTransfer_ReadErrorSkips verifies that when
// GetSpotBalance returns an error, the helper returns an error rather
// than silently falling back to snapshotSpot. Caller must skip donor.
func TestCaptureSpotBalanceForTransfer_ReadErrorSkips(t *testing.T) {
	stub := &singleReadStub{value: 999.0}
	eng := newWaitTestEngine(map[string]exchange.Exchange{"binance": stub})

	_, err := eng.captureSpotBalanceForTransfer("binance", 100.0)
	if err == nil {
		t.Fatal("expected error when GetSpotBalance fails, got nil (snapshot fallback was removed)")
	}
	if !strings.Contains(err.Error(), "GetSpotBalance failed") {
		t.Errorf("expected error to mention GetSpotBalance failure, got: %v", err)
	}
	if atomic.LoadInt32(&stub.calls) != 1 {
		t.Errorf("expected exactly 1 GetSpotBalance call, got %d", stub.calls)
	}
}

// TestCaptureSpotBalanceForTransfer_UnknownDonor verifies the helper
// fails fast when donor key is not registered.
func TestCaptureSpotBalanceForTransfer_UnknownDonor(t *testing.T) {
	eng := newWaitTestEngine(map[string]exchange.Exchange{})
	_, err := eng.captureSpotBalanceForTransfer("nonexistent", 100.0)
	if err == nil {
		t.Fatal("expected error for unknown donor, got nil")
	}
	if !strings.Contains(err.Error(), "nonexistent") {
		t.Errorf("expected error to contain donor name, got: %v", err)
	}
}
