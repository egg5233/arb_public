package strategy

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"arb/internal/config"
)

type Strategy string

const (
	StrategyPP   Strategy = "pp"
	StrategyDirA Strategy = "dir_a"
	StrategyDirB Strategy = "dir_b"
)

type StrategySnapshot struct {
	EnableStrategyPriority bool
	Mode                   config.StrategyPriority
	EffectiveMode          config.StrategyPriority
	ExpectedHoldHours      float64
	Epoch                  uint64
	CapitalAllocatorOn     bool
}

type ReservationState string

const (
	ReservationPending  ReservationState = "pending"
	ReservationInFlight ReservationState = "in_flight"
)

type ReservationOutcome string

const (
	ReservationOutcomeActive  ReservationOutcome = "active"
	ReservationOutcomeAborted ReservationOutcome = "aborted"
)

type LegKey struct {
	Exchange string
	Market   string
	Symbol   string
}

type Reservation struct {
	ID          string
	Keys        []LegKey
	Strategy    Strategy
	CandidateID string
	EVBpsH      float64
	Epoch       uint64
	State       ReservationState
	OrderIDs    map[LegKey]string
	ExpiresAt   time.Time

	priority int
	outcome  ReservationOutcome
}

type ReserveReason string

const (
	ReserveReasonGranted              ReserveReason = "granted"
	ReserveReasonDeniedMode           ReserveReason = "denied_mode"
	ReserveReasonManualOverrideNeeded ReserveReason = "override_required"
	ReserveReasonConflictPending      ReserveReason = "conflict_pending"
	ReserveReasonConflictInFlight     ReserveReason = "conflict_in_flight"
	ReserveReasonInvalidRequest       ReserveReason = "invalid_request"
)

type TryReserveResult struct {
	Granted       bool
	ReservationID string
	Reason        ReserveReason
	Conflicts     []Reservation
}

type PositionStrategyMeta struct {
	ReservationID string
	CandidateID   string
	StrategyEpoch uint64
	Strategy      Strategy
	Keys          []LegKey
}

type EVBreakdown struct {
	FundingBpsH  float64
	FeesBpsH     float64
	BorrowBpsH   float64
	RotationBpsH float64
	NetBpsH      float64
}

type ReservationCompletionRecord struct {
	ReservationID string
	Outcome       ReservationOutcome
	Epoch         uint64
	Strategy      Strategy
	CandidateID   string
	Keys          []LegKey
	CompletedAt   time.Time
}

type PendingDirBConflict struct {
	DirBCandidateID       string
	ConflictReservationID string
	Epoch                 uint64
	ConflictStrategy      Strategy
	ConflictCandidateID   string
	OverlappingKeys       []LegKey
	DirBEVBpsH            float64
	ConflictEVBpsH        float64
	CreatedAt             time.Time
	ExpiresAt             time.Time
}

type SLOStore interface {
	RecordReservationCompletion(ReservationCompletionRecord) error
	RecordDirBPendingConflict(PendingDirBConflict) error
	FinalizeDirBConflicts(ReservationCompletionRecord) error
}

type ReserveOptions struct {
	ManualOverride bool
	Source         string
	CandidateID    string
}

type Coordinator struct {
	mu           sync.RWMutex
	snapshot     StrategySnapshot
	reservations map[string]*Reservation
	byKey        map[LegKey]string
	nextID       uint64
	ttl          time.Duration
	store        SLOStore
}

func NewCoordinator(cfg *config.Config) *Coordinator {
	c := &Coordinator{
		reservations: make(map[string]*Reservation),
		byKey:        make(map[LegKey]string),
		ttl:          5 * time.Minute,
	}
	c.snapshot = c.snapshotFromConfig(cfg, 0)
	return c
}

func (c *Coordinator) Snapshot() StrategySnapshot {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.gcLocked(time.Now())
	return c.snapshot
}

func (c *Coordinator) UpdatePriority(cfg *config.Config) {
	c.mu.Lock()
	defer c.mu.Unlock()

	next := c.snapshotFromConfig(cfg, c.snapshot.Epoch)
	if snapshotFieldsEqual(c.snapshot, next) {
		return
	}
	next.Epoch = c.snapshot.Epoch + 1
	c.snapshot = next
}

func (c *Coordinator) SetSLOStore(store SLOStore) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.store = store
}

func DirBEV(futuresShortFundingBpsH, roundTripFeeBps, expectedHoldHours float64) EVBreakdown {
	fees := amortizedFees(roundTripFeeBps, expectedHoldHours)
	return EVBreakdown{
		FundingBpsH: futuresShortFundingBpsH,
		FeesBpsH:    fees,
		NetBpsH:     futuresShortFundingBpsH - fees,
	}
}

func DirAEV(publishedFundingBpsH, roundTripFeeBps, borrowAPR, expectedHoldHours float64) EVBreakdown {
	fees := amortizedFees(roundTripFeeBps, expectedHoldHours)
	borrow := borrowAPR * 10000 / (365 * 24)
	funding := -publishedFundingBpsH
	return EVBreakdown{
		FundingBpsH: funding,
		FeesBpsH:    fees,
		BorrowBpsH:  borrow,
		NetBpsH:     funding - fees - borrow,
	}
}

func PPEV(shortFundingBpsH, longFundingBpsH, roundTripFeeBps, rotation7dTotalBps, expectedHoldHours float64) EVBreakdown {
	fees := amortizedFees(roundTripFeeBps, expectedHoldHours)
	rotation := rotation7dTotalBps / (7 * 24)
	funding := shortFundingBpsH - longFundingBpsH
	return EVBreakdown{
		FundingBpsH:  funding,
		FeesBpsH:     fees,
		RotationBpsH: rotation,
		NetBpsH:      funding - fees - rotation,
	}
}

func amortizedFees(roundTripFeeBps, expectedHoldHours float64) float64 {
	if expectedHoldHours <= 0 {
		expectedHoldHours = 24
	}
	return roundTripFeeBps / expectedHoldHours
}

func (c *Coordinator) TryReserveMany(snap StrategySnapshot, keys []LegKey, strategy Strategy, evBpsH float64, opts ReserveOptions) TryReserveResult {
	if !snap.EnableStrategyPriority {
		return TryReserveResult{Granted: true, Reason: ReserveReasonGranted}
	}

	keys = CanonicalLegKeys(keys)
	if len(keys) == 0 || !validStrategy(strategy) {
		result := TryReserveResult{Reason: ReserveReasonInvalidRequest}
		c.logReserveResult(strategy, keys, evBpsH, result)
		return result
	}
	if strings.Contains(opts.Source, "auto") && strings.TrimSpace(opts.CandidateID) == "" {
		result := TryReserveResult{Reason: ReserveReasonInvalidRequest}
		c.logReserveResult(strategy, keys, evBpsH, result)
		return result
	}

	priority, allowed := modePriority(snap.EffectiveMode, strategy)
	if !allowed {
		if !opts.ManualOverride {
			result := TryReserveResult{Reason: ReserveReasonManualOverrideNeeded}
			c.logReserveResult(strategy, keys, evBpsH, result)
			return result
		}
		priority = 0
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.gcLocked(time.Now())

	var conflicts []Reservation
	bumpIDs := make(map[string]bool)
	for _, key := range keys {
		existingID, ok := c.byKey[key]
		if !ok {
			continue
		}
		existing, ok := c.reservations[existingID]
		if !ok {
			delete(c.byKey, key)
			continue
		}
		conflicts = append(conflicts, cloneReservation(existing))
		if existing.State == ReservationInFlight {
			result := TryReserveResult{Reason: ReserveReasonConflictInFlight, Conflicts: conflicts}
			c.recordDirBConflictIfNeeded(snap, keys, strategy, evBpsH, opts, existing)
			c.logReserveResult(strategy, keys, evBpsH, result)
			return result
		}
		if !incomingBeatsExisting(priority, evBpsH, existing.priority, existing.EVBpsH) {
			result := TryReserveResult{Reason: ReserveReasonConflictPending, Conflicts: conflicts}
			c.logReserveResult(strategy, keys, evBpsH, result)
			return result
		}
		bumpIDs[existing.ID] = true
	}

	for id := range bumpIDs {
		c.releaseLocked(id)
	}

	c.nextID++
	id := fmt.Sprintf("strat-%d-%d", time.Now().UnixNano(), c.nextID)
	res := &Reservation{
		ID:          id,
		Keys:        keys,
		Strategy:    strategy,
		CandidateID: strings.TrimSpace(opts.CandidateID),
		EVBpsH:      evBpsH,
		Epoch:       snap.Epoch,
		State:       ReservationPending,
		OrderIDs:    make(map[LegKey]string),
		ExpiresAt:   time.Now().Add(c.ttl),
		priority:    priority,
	}
	c.reservations[id] = res
	for _, key := range keys {
		c.byKey[key] = id
	}

	result := TryReserveResult{Granted: true, ReservationID: id, Reason: ReserveReasonGranted}
	c.logReserveResult(strategy, keys, evBpsH, result)
	return result
}

func (c *Coordinator) MarkInFlight(reservationID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.gcLocked(time.Now())

	res, ok := c.reservations[reservationID]
	if !ok {
		return fmt.Errorf("reservation %s not found", reservationID)
	}
	for _, key := range res.Keys {
		if c.byKey[key] != reservationID {
			return fmt.Errorf("reservation %s no longer owns %s", reservationID, LegKeyString(key))
		}
	}
	res.State = ReservationInFlight
	return nil
}

func (c *Coordinator) BindOrder(reservationID string, key LegKey, exchangeOrderID string) error {
	key = NormalizeLegKey(key)
	c.mu.Lock()
	defer c.mu.Unlock()

	res, ok := c.reservations[reservationID]
	if !ok {
		return fmt.Errorf("reservation %s not found", reservationID)
	}
	if !ownsKey(res, key) {
		return fmt.Errorf("reservation %s does not own %s", reservationID, LegKeyString(key))
	}
	if existing := res.OrderIDs[key]; existing != "" && existing != exchangeOrderID {
		return fmt.Errorf("reservation %s key %s already bound to order %s", reservationID, LegKeyString(key), existing)
	}
	res.OrderIDs[key] = exchangeOrderID
	return nil
}

func (c *Coordinator) CompleteReservation(reservationID string, outcome ReservationOutcome) {
	if reservationID == "" {
		return
	}
	c.mu.Lock()

	res, ok := c.reservations[reservationID]
	if !ok {
		c.mu.Unlock()
		return
	}
	if res.outcome != "" && res.outcome != outcome {
		c.mu.Unlock()
		return
	}
	res.outcome = outcome
	record := ReservationCompletionRecord{
		ReservationID: res.ID,
		Outcome:       outcome,
		Epoch:         res.Epoch,
		Strategy:      res.Strategy,
		CandidateID:   res.CandidateID,
		Keys:          append([]LegKey(nil), res.Keys...),
		CompletedAt:   time.Now().UTC(),
	}
	store := c.store
	c.mu.Unlock()

	if store != nil {
		if err := store.RecordReservationCompletion(record); err != nil {
			log.Printf("[coordinator] record completion %s: %v", reservationID, err)
		}
		if err := store.FinalizeDirBConflicts(record); err != nil {
			log.Printf("[coordinator] finalize dirb conflicts %s: %v", reservationID, err)
		}
	}
}

func (c *Coordinator) Release(reservationID string) {
	if reservationID == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.releaseLocked(reservationID)
}

func (c *Coordinator) ReservationMeta(reservationID string) (PositionStrategyMeta, bool) {
	if reservationID == "" {
		return PositionStrategyMeta{}, false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	res, ok := c.reservations[reservationID]
	if !ok {
		return PositionStrategyMeta{}, false
	}
	return PositionStrategyMeta{
		ReservationID: res.ID,
		CandidateID:   res.CandidateID,
		StrategyEpoch: res.Epoch,
		Strategy:      res.Strategy,
		Keys:          append([]LegKey(nil), res.Keys...),
	}, true
}

func CanonicalLegKeys(keys []LegKey) []LegKey {
	seen := make(map[LegKey]bool, len(keys))
	out := make([]LegKey, 0, len(keys))
	for _, key := range keys {
		key = NormalizeLegKey(key)
		if key.Exchange == "" || key.Market == "" || key.Symbol == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, key)
	}
	sort.Slice(out, func(i, j int) bool {
		return LegKeyString(out[i]) < LegKeyString(out[j])
	})
	return out
}

func NormalizeLegKey(key LegKey) LegKey {
	return LegKey{
		Exchange: strings.ToLower(strings.TrimSpace(key.Exchange)),
		Market:   strings.ToLower(strings.TrimSpace(key.Market)),
		Symbol:   strings.ToUpper(strings.TrimSpace(key.Symbol)),
	}
}

func LegKeyString(key LegKey) string {
	key = NormalizeLegKey(key)
	return key.Exchange + ":" + key.Market + ":" + key.Symbol
}

func LegKeyStrings(keys []LegKey) []string {
	keys = CanonicalLegKeys(keys)
	out := make([]string, 0, len(keys))
	for _, key := range keys {
		out = append(out, LegKeyString(key))
	}
	return out
}

func (c *Coordinator) snapshotFromConfig(cfg *config.Config, epoch uint64) StrategySnapshot {
	s := StrategySnapshot{
		Mode:              config.StrategyPriorityPerpPerpFirst,
		EffectiveMode:     config.StrategyPriorityPerpPerpFirst,
		ExpectedHoldHours: 24,
		Epoch:             epoch,
	}
	if cfg == nil {
		return s
	}

	cfg.RLock()
	defer cfg.RUnlock()

	mode, ok := config.NormalizeStrategyPriority(string(cfg.StrategyPriority))
	if !ok {
		mode = config.StrategyPriorityPerpPerpFirst
	}
	expectedHoldHours := cfg.ExpectedHoldHours
	if !config.ValidateExpectedHoldHours(expectedHoldHours) {
		expectedHoldHours = 24
	}

	s.EnableStrategyPriority = cfg.EnableStrategyPriority
	s.Mode = mode
	s.EffectiveMode = config.StrategyPriorityPerpPerpFirst
	if cfg.EnableStrategyPriority {
		s.EffectiveMode = mode
	}
	s.ExpectedHoldHours = expectedHoldHours
	s.CapitalAllocatorOn = cfg.EnableCapitalAllocator
	return s
}

func snapshotFieldsEqual(a, b StrategySnapshot) bool {
	return a.EnableStrategyPriority == b.EnableStrategyPriority &&
		a.Mode == b.Mode &&
		a.EffectiveMode == b.EffectiveMode &&
		a.ExpectedHoldHours == b.ExpectedHoldHours &&
		a.CapitalAllocatorOn == b.CapitalAllocatorOn
}

func (c *Coordinator) recordDirBConflictIfNeeded(snap StrategySnapshot, incomingKeys []LegKey, incoming Strategy, incomingEV float64, opts ReserveOptions, existing *Reservation) {
	if incoming != StrategyDirB || existing == nil || existing.Strategy != StrategyPP {
		return
	}
	if existing.Epoch != snap.Epoch || incomingEV <= existing.EVBpsH+0.01 {
		return
	}
	store := c.store
	if store == nil {
		return
	}
	overlap := overlappingKeys(incomingKeys, existing.Keys)
	if len(overlap) == 0 {
		return
	}
	now := time.Now().UTC()
	rec := PendingDirBConflict{
		DirBCandidateID:       strings.TrimSpace(opts.CandidateID),
		ConflictReservationID: existing.ID,
		Epoch:                 snap.Epoch,
		ConflictStrategy:      existing.Strategy,
		ConflictCandidateID:   existing.CandidateID,
		OverlappingKeys:       overlap,
		DirBEVBpsH:            incomingEV,
		ConflictEVBpsH:        existing.EVBpsH,
		CreatedAt:             now,
		ExpiresAt:             now.Add(maxDuration(c.ttl+5*time.Minute, 15*time.Minute)),
	}
	if err := store.RecordDirBPendingConflict(rec); err != nil {
		log.Printf("[coordinator] record dirb pending conflict: %v", err)
	}
}

func (c *Coordinator) logReserveResult(strategy Strategy, keys []LegKey, evBpsH float64, result TryReserveResult) {
	log.Printf("[coordinator] reserve result strategy=%s keys=%s ev_bps_h=%.4f granted=%v reason=%s conflicts=%d",
		strategy, strings.Join(LegKeyStrings(keys), ","), evBpsH, result.Granted, result.Reason, len(result.Conflicts))
}

func overlappingKeys(a, b []LegKey) []LegKey {
	seen := make(map[LegKey]bool, len(a))
	for _, key := range CanonicalLegKeys(a) {
		seen[key] = true
	}
	var out []LegKey
	for _, key := range CanonicalLegKeys(b) {
		if seen[key] {
			out = append(out, key)
		}
	}
	return out
}

func maxDuration(a, b time.Duration) time.Duration {
	if a > b {
		return a
	}
	return b
}

func validStrategy(strategy Strategy) bool {
	return strategy == StrategyPP || strategy == StrategyDirA || strategy == StrategyDirB
}

func modePriority(mode config.StrategyPriority, strategy Strategy) (int, bool) {
	switch mode {
	case config.StrategyPriorityPerpPerpFirst:
		if strategy == StrategyPP {
			return 2, true
		}
		return 1, true
	case config.StrategyPriorityDirBFirst:
		if strategy == StrategyDirB {
			return 2, true
		}
		return 1, true
	case config.StrategyPriorityDirBOnly:
		if strategy == StrategyDirB {
			return 2, true
		}
		return 0, false
	case config.StrategyPriorityPerpPerpOnly:
		if strategy == StrategyPP {
			return 2, true
		}
		return 0, false
	default:
		return modePriority(config.StrategyPriorityPerpPerpFirst, strategy)
	}
}

func incomingBeatsExisting(inPriority int, inEV float64, existingPriority int, existingEV float64) bool {
	if inPriority > existingPriority {
		return true
	}
	if inPriority < existingPriority {
		return false
	}
	return inEV > existingEV+0.01
}

func (c *Coordinator) gcLocked(now time.Time) {
	for id, res := range c.reservations {
		if now.After(res.ExpiresAt) && res.State == ReservationPending {
			c.releaseLocked(id)
		}
	}
}

func (c *Coordinator) releaseLocked(id string) {
	res, ok := c.reservations[id]
	if !ok {
		return
	}
	for _, key := range res.Keys {
		if c.byKey[key] == id {
			delete(c.byKey, key)
		}
	}
	delete(c.reservations, id)
}

func cloneReservation(res *Reservation) Reservation {
	out := *res
	out.Keys = append([]LegKey(nil), res.Keys...)
	out.OrderIDs = make(map[LegKey]string, len(res.OrderIDs))
	for key, orderID := range res.OrderIDs {
		out.OrderIDs[key] = orderID
	}
	return out
}

func ownsKey(res *Reservation, key LegKey) bool {
	for _, owned := range res.Keys {
		if owned == key {
			return true
		}
	}
	return false
}
