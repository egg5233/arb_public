package pricegaptrader

import (
	"context"
	"sync"

	"arb/internal/models"
)

// fakeRegistry is the in-package RegistryWriter test double.
// Used by both promotion_test.go (this plan) and scanner_test.go (Plan 03's
// TestScanner_RunCycleCallsPromotionApply).
type fakeRegistry struct {
	mu     sync.Mutex
	cands  []models.PriceGapCandidate
	addErr map[string]error // keyed by Symbol
	delErr map[int]error    // keyed by idx
	addLog []fakeRegistryAddCall
	delLog []fakeRegistryDeleteCall
}

type fakeRegistryAddCall struct {
	Source    string
	Candidate models.PriceGapCandidate
}

type fakeRegistryDeleteCall struct {
	Source string
	Idx    int
}

func newFakeRegistry() *fakeRegistry {
	return &fakeRegistry{
		cands:  []models.PriceGapCandidate{},
		addErr: map[string]error{},
		delErr: map[int]error{},
	}
}

func (f *fakeRegistry) Add(ctx context.Context, source string, c models.PriceGapCandidate) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err, ok := f.addErr[c.Symbol]; ok {
		return err
	}
	f.cands = append(f.cands, c)
	f.addLog = append(f.addLog, fakeRegistryAddCall{Source: source, Candidate: c})
	return nil
}

func (f *fakeRegistry) Delete(ctx context.Context, source string, idx int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err, ok := f.delErr[idx]; ok {
		return err
	}
	if idx < 0 || idx >= len(f.cands) {
		return ErrIndexOutOfRange
	}
	f.cands = append(f.cands[:idx], f.cands[idx+1:]...)
	f.delLog = append(f.delLog, fakeRegistryDeleteCall{Source: source, Idx: idx})
	return nil
}

func (f *fakeRegistry) List() []models.PriceGapCandidate {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]models.PriceGapCandidate, len(f.cands))
	copy(out, f.cands)
	return out
}

// fakeSink — PromoteEventSink. Captures emitted events for assertion.
type fakeSink struct {
	mu     sync.Mutex
	events []PromoteEvent
	err    error
}

func (f *fakeSink) Emit(ctx context.Context, ev PromoteEvent) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = append(f.events, ev)
	return f.err
}

func (f *fakeSink) Events() []PromoteEvent {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]PromoteEvent, len(f.events))
	copy(out, f.events)
	return out
}

// fakeNotifier — PromoteNotifier. Captures NotifyPromoteEvent calls.
type fakeNotifier struct {
	mu    sync.Mutex
	calls []fakeNotifyCall
}

type fakeNotifyCall struct {
	Action, Symbol, LongExch, ShortExch, Direction string
	Score, Streak                                  int
}

func (f *fakeNotifier) NotifyPromoteEvent(action, symbol, longExch, shortExch, direction string, score, streak int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, fakeNotifyCall{
		Action: action, Symbol: symbol, LongExch: longExch, ShortExch: shortExch,
		Direction: direction, Score: score, Streak: streak,
	})
}

func (f *fakeNotifier) Calls() []fakeNotifyCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]fakeNotifyCall, len(f.calls))
	copy(out, f.calls)
	return out
}

// fakeGuard — ActivePositionChecker.
type fakeGuard struct {
	mu      sync.Mutex
	blocked map[string]bool // keyed by Symbol
	err     error
}

func (f *fakeGuard) IsActiveForCandidate(c models.PriceGapCandidate) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return true, f.err // D-05 fail-safe: error → blocked
	}
	return f.blocked[c.Symbol], nil
}

func (f *fakeGuard) setBlocked(symbol string, blocked bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.blocked == nil {
		f.blocked = map[string]bool{}
	}
	f.blocked[symbol] = blocked
}

// fakeTelemetrySink — TelemetrySink.
type fakeTelemetrySink struct {
	mu    sync.Mutex
	skips map[string]int // symbol → count
	err   error
}

func newFakeTelemetrySink() *fakeTelemetrySink {
	return &fakeTelemetrySink{skips: map[string]int{}}
}

func (f *fakeTelemetrySink) IncCapFullSkip(ctx context.Context, symbol string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.skips[symbol]++
	return nil
}

func (f *fakeTelemetrySink) Skips(symbol string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.skips[symbol]
}
