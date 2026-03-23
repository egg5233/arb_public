package models

import (
	"sync"
	"time"
)

// RejectedOpportunity records why an opportunity was filtered out.
type RejectedOpportunity struct {
	Symbol        string    `json:"symbol"`
	LongExchange  string    `json:"long_exchange"`
	ShortExchange string    `json:"short_exchange"`
	Spread        float64   `json:"spread"`
	Stage         string    `json:"stage"`  // "scanner", "verifier", "risk", "engine"
	Reason        string    `json:"reason"`
	Timestamp     time.Time `json:"timestamp"`
}

// RejectionStore is a thread-safe ring buffer for rejected opportunities.
type RejectionStore struct {
	mu      sync.RWMutex
	entries []RejectedOpportunity
	max     int
	onAdd   func(RejectedOpportunity)
}

// NewRejectionStore creates a store with the given capacity.
// onAdd is called (outside the lock) for each new rejection — use for WS broadcast.
func NewRejectionStore(max int, onAdd func(RejectedOpportunity)) *RejectionStore {
	return &RejectionStore{
		entries: make([]RejectedOpportunity, 0, max),
		max:     max,
		onAdd:   onAdd,
	}
}

// Add records a rejected opportunity.
func (s *RejectionStore) Add(symbol, longExch, shortExch string, spread float64, stage, reason string) {
	r := RejectedOpportunity{
		Symbol:        symbol,
		LongExchange:  longExch,
		ShortExchange: shortExch,
		Spread:        spread,
		Stage:         stage,
		Reason:        reason,
		Timestamp:     time.Now().UTC(),
	}

	s.mu.Lock()
	s.entries = append(s.entries, r)
	if len(s.entries) > s.max {
		s.entries = s.entries[len(s.entries)-s.max:]
	}
	s.mu.Unlock()

	if s.onAdd != nil {
		s.onAdd(r)
	}
}

// AddOpp is a convenience wrapper that extracts fields from an Opportunity.
func (s *RejectionStore) AddOpp(opp Opportunity, stage, reason string) {
	s.Add(opp.Symbol, opp.LongExchange, opp.ShortExchange, opp.Spread, stage, reason)
}

// GetAll returns a copy of all stored rejections (newest last).
func (s *RejectionStore) GetAll() []RejectedOpportunity {
	s.mu.RLock()
	out := make([]RejectedOpportunity, len(s.entries))
	copy(out, s.entries)
	s.mu.RUnlock()
	return out
}
