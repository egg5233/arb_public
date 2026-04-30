package pricegaptrader

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"arb/internal/config"
	"arb/internal/models"
)

// recAccepted builds an accepted CycleRecord. NOTE: no Direction field is set
// because CycleRecord (scanner.go:89) has no Direction field — D-18.
func recAccepted(symbol, longExch, shortExch string, score int) CycleRecord {
	return CycleRecord{
		Symbol:      symbol,
		LongExch:    longExch,
		ShortExch:   shortExch,
		Score:       score,
		WhyRejected: ReasonAccepted,
	}
}

// newTestController wires fakes for testing.
func newTestController(t *testing.T, threshold, maxCands int) (
	*PromotionController, *fakeRegistry, *fakeSink, *fakeNotifier, *fakeGuard, *fakeTelemetrySink,
) {
	t.Helper()
	cfg := &config.Config{
		PriceGapAutoPromoteScore: threshold,
		PriceGapMaxCandidates:    maxCands,
	}
	reg := newFakeRegistry()
	sink := &fakeSink{}
	notif := &fakeNotifier{}
	guard := &fakeGuard{}
	telem := newFakeTelemetrySink()

	c, err := NewPromotionController(cfg, reg, guard, sink, notif, telem, nil)
	if err != nil {
		t.Fatalf("NewPromotionController: %v", err)
	}
	// Deterministic timestamp for event assertions.
	c.SetNowFunc(func() int64 { return 1700000000000 })
	return c, reg, sink, notif, guard, telem
}

// summaryWith builds a CycleSummary using ONLY the real CycleSummary fields
// from scanner.go (StartedAt, CompletedAt, DurationMs, Records).
func summaryWith(records ...CycleRecord) CycleSummary {
	return CycleSummary{
		StartedAt:   1,
		CompletedAt: 2,
		DurationMs:  1,
		Records:     records,
	}
}

// TestPromotionStreak — D-01: 5 accepted cycles → streak counter 5; no promote
// fires (threshold is 6).
func TestPromotionStreak(t *testing.T) {
	c, reg, sink, _, _, _ := newTestController(t, 70, 12)
	ctx := context.Background()
	rec := recAccepted("BTCUSDT", "binance", "bybit", 80)
	for i := 0; i < 5; i++ {
		if err := c.Apply(ctx, summaryWith(rec)); err != nil {
			t.Fatalf("Apply cycle %d: %v", i, err)
		}
	}
	if len(reg.addLog) != 0 {
		t.Errorf("expected 0 promotes after 5 cycles, got %d", len(reg.addLog))
	}
	if len(sink.events) != 0 {
		t.Errorf("expected 0 events after 5 cycles, got %d", len(sink.events))
	}
	key := candidateKey("BTCUSDT", "binance", "bybit", "bidirectional")
	if c.promoteStreaks[key] != 5 {
		t.Errorf("promote streak: want 5, got %d", c.promoteStreaks[key])
	}
}

// TestPromotionStreakReset — D-02: any miss resets the streak to 0.
func TestPromotionStreakReset(t *testing.T) {
	c, _, _, _, _, _ := newTestController(t, 70, 12)
	ctx := context.Background()
	rec := recAccepted("BTCUSDT", "binance", "bybit", 80)
	// 3 accepted cycles
	for i := 0; i < 3; i++ {
		_ = c.Apply(ctx, summaryWith(rec))
	}
	// 1 missed cycle (empty Records)
	_ = c.Apply(ctx, summaryWith())
	key := candidateKey("BTCUSDT", "binance", "bybit", "bidirectional")
	if got, ok := c.promoteStreaks[key]; ok && got != 0 {
		t.Errorf("after miss expected streak=0 (or absent), got %d", got)
	}
	// 1 accepted again
	_ = c.Apply(ctx, summaryWith(rec))
	if c.promoteStreaks[key] != 1 {
		t.Errorf("after resume expected streak=1, got %d", c.promoteStreaks[key])
	}
}

// TestPromotionFires — D-01 + D-10 + D-13/14 + D-18: 6 consecutive accepted
// cycles fire Registry.Add + sink.Emit + Telegram with the locked field
// values.
func TestPromotionFires(t *testing.T) {
	c, reg, sink, notif, _, _ := newTestController(t, 70, 12)
	ctx := context.Background()
	rec := recAccepted("BTCUSDT", "binance", "bybit", 82)
	for i := 0; i < 6; i++ {
		_ = c.Apply(ctx, summaryWith(rec))
	}
	if len(reg.addLog) != 1 {
		t.Fatalf("expected 1 Registry.Add, got %d", len(reg.addLog))
	}
	if reg.addLog[0].Source != "scanner-promote" {
		t.Errorf("Add source: want scanner-promote, got %q", reg.addLog[0].Source)
	}
	if reg.addLog[0].Candidate.Direction != "bidirectional" {
		t.Errorf("Direction: want bidirectional, got %q", reg.addLog[0].Candidate.Direction)
	}
	if len(sink.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(sink.events))
	}
	ev := sink.events[0]
	if ev.Action != "promote" || ev.StreakCycles != 6 || ev.Reason != "score_threshold_met" || ev.Direction != "bidirectional" {
		t.Errorf("unexpected event: %+v", ev)
	}
	if len(notif.calls) != 1 {
		t.Fatalf("expected 1 Telegram call, got %d", len(notif.calls))
	}
	if notif.calls[0].Direction != "bidirectional" || notif.calls[0].Action != "promote" {
		t.Errorf("unexpected notifier call: %+v", notif.calls[0])
	}
}

// TestPromotionCapFull — D-08: cap-full → silent skip + IncCapFullSkip + HOLD
// streak at 6.
func TestPromotionCapFull(t *testing.T) {
	c, reg, sink, notif, _, telem := newTestController(t, 70, 2)
	ctx := context.Background()
	// Pre-seed cap with PINNED candidates (Direction != "bidirectional") so
	// they occupy the cap-counting len(currentList) universe but do NOT
	// enter the controller's bidirectional streak universe — pinned tuples
	// are not the controller's responsibility (D-09 + D-18 compatibility).
	_ = reg.Add(ctx, "test-seed", models.PriceGapCandidate{Symbol: "ETHUSDT", LongExch: "binance", ShortExch: "bybit", Direction: "pinned"})
	_ = reg.Add(ctx, "test-seed", models.PriceGapCandidate{Symbol: "SOLUSDT", LongExch: "binance", ShortExch: "bybit", Direction: "pinned"})
	// Capture pre-test add count so we ignore the seed adds.
	preAdds := len(reg.addLog)
	rec := recAccepted("BTCUSDT", "binance", "bybit", 90)
	for i := 0; i < 6; i++ {
		_ = c.Apply(ctx, summaryWith(rec))
	}
	if telem.Skips("BTCUSDT") < 1 {
		t.Errorf("IncCapFullSkip(BTCUSDT) not called; got count %d", telem.Skips("BTCUSDT"))
	}
	if got := len(reg.addLog) - preAdds; got != 0 {
		t.Errorf("Add should not have fired during cap-full; got %d new adds", got)
	}
	if len(sink.events) != 0 || len(notif.calls) != 0 {
		t.Errorf("no events/notifications on cap-full; got events=%d, calls=%d", len(sink.events), len(notif.calls))
	}
	key := candidateKey("BTCUSDT", "binance", "bybit", "bidirectional")
	if c.promoteStreaks[key] != 6 {
		t.Errorf("promote streak should HOLD at 6; got %d", c.promoteStreaks[key])
	}
}

// TestPromotionDedupe — already-promoted candidate: controller short-circuits
// and emits no event.
func TestPromotionDedupe(t *testing.T) {
	c, reg, sink, notif, _, _ := newTestController(t, 70, 12)
	ctx := context.Background()
	// Pre-seed the candidate the controller would otherwise promote.
	_ = reg.Add(ctx, "test-seed", models.PriceGapCandidate{Symbol: "BTCUSDT", LongExch: "binance", ShortExch: "bybit", Direction: "bidirectional"})
	addLogBefore := len(reg.addLog)
	rec := recAccepted("BTCUSDT", "binance", "bybit", 80)
	for i := 0; i < 6; i++ {
		_ = c.Apply(ctx, summaryWith(rec))
	}
	if len(reg.addLog)-addLogBefore != 0 {
		t.Errorf("controller should NOT re-Add already-promoted candidate; got %d new adds", len(reg.addLog)-addLogBefore)
	}
	if len(sink.events) != 0 || len(notif.calls) != 0 {
		t.Errorf("no events for already-promoted; got events=%d, calls=%d", len(sink.events), len(notif.calls))
	}
}

// TestDemoteStreak — D-04: 6 cycles where promoted candidate is absent fires
// Registry.Delete with source=scanner-demote.
func TestDemoteStreak(t *testing.T) {
	c, reg, sink, notif, guard, _ := newTestController(t, 70, 12)
	ctx := context.Background()
	_ = reg.Add(ctx, "test-seed", models.PriceGapCandidate{Symbol: "BTCUSDT", LongExch: "binance", ShortExch: "bybit", Direction: "bidirectional"})
	guard.setBlocked("BTCUSDT", false)
	// 6 empty cycles (candidate absent).
	for i := 0; i < 6; i++ {
		_ = c.Apply(ctx, summaryWith())
	}
	if len(reg.delLog) != 1 {
		t.Fatalf("expected 1 Delete, got %d", len(reg.delLog))
	}
	if reg.delLog[0].Source != "scanner-demote" {
		t.Errorf("Delete source: want scanner-demote, got %q", reg.delLog[0].Source)
	}
	if len(sink.events) != 1 || sink.events[0].Action != "demote" || sink.events[0].Reason != "score_below_threshold" {
		t.Errorf("expected 1 demote event with reason=score_below_threshold; got %+v", sink.events)
	}
	if len(notif.calls) != 1 || notif.calls[0].Action != "demote" {
		t.Errorf("expected 1 demote notifier call; got %+v", notif.calls)
	}
}

// TestDemoteBlockedByGuard — D-05: active-position guard blocks demote;
// streak HELD; once unblocked the very next cycle fires.
func TestDemoteBlockedByGuard(t *testing.T) {
	c, reg, sink, notif, guard, _ := newTestController(t, 70, 12)
	ctx := context.Background()
	_ = reg.Add(ctx, "test-seed", models.PriceGapCandidate{Symbol: "BTCUSDT", LongExch: "binance", ShortExch: "bybit", Direction: "bidirectional"})
	guard.setBlocked("BTCUSDT", true)
	for i := 0; i < 6; i++ {
		_ = c.Apply(ctx, summaryWith())
	}
	if len(reg.delLog) != 0 || len(sink.events) != 0 || len(notif.calls) != 0 {
		t.Errorf("guard-blocked: no delete/event/notify expected; got del=%d events=%d calls=%d",
			len(reg.delLog), len(sink.events), len(notif.calls))
	}
	key := candidateKey("BTCUSDT", "binance", "bybit", "bidirectional")
	if c.demoteStreaks[key] != 6 {
		t.Errorf("demote streak HELD at 6; got %d", c.demoteStreaks[key])
	}
	// 7th blocked cycle still HELD.
	_ = c.Apply(ctx, summaryWith())
	if c.demoteStreaks[key] != 6 {
		t.Errorf("demote streak still HELD at 6 across blocked cycles; got %d", c.demoteStreaks[key])
	}
	// Unblock and run again.
	guard.setBlocked("BTCUSDT", false)
	_ = c.Apply(ctx, summaryWith())
	if len(reg.delLog) != 1 {
		t.Errorf("after guard unblocks demote should fire on next cycle; got delLog=%d", len(reg.delLog))
	}
}

// TestDemoteGuardError — D-05: guard read error → fail-safe to "blocked";
// no delete fires.
func TestDemoteGuardError(t *testing.T) {
	c, reg, _, _, guard, _ := newTestController(t, 70, 12)
	ctx := context.Background()
	_ = reg.Add(ctx, "test-seed", models.PriceGapCandidate{Symbol: "BTCUSDT", LongExch: "binance", ShortExch: "bybit", Direction: "bidirectional"})
	guard.err = errors.New("redis down")
	for i := 0; i < 6; i++ {
		_ = c.Apply(ctx, summaryWith())
	}
	if len(reg.delLog) != 0 {
		t.Errorf("guard error should fail-safe to blocked; got %d deletes", len(reg.delLog))
	}
}

// TestPromoteEventLog — D-10 + D-18: PromoteEvent JSON shape matches the
// locked snake_case keys; Direction is always "bidirectional".
func TestPromoteEventLog(t *testing.T) {
	c, reg, sink, _, _, _ := newTestController(t, 70, 12)
	ctx := context.Background()
	rec := recAccepted("BTCUSDT", "binance", "bybit", 82)
	for i := 0; i < 6; i++ {
		_ = c.Apply(ctx, summaryWith(rec))
	}
	if len(sink.events) != 1 {
		t.Fatalf("expected 1 event after 6-cycle promote; got %d", len(sink.events))
	}
	raw, err := json.Marshal(sink.events[0])
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}
	s := string(raw)
	for _, want := range []string{
		`"ts":`,
		`"action":"promote"`,
		`"symbol":"BTCUSDT"`,
		`"long_exch":"binance"`,
		`"short_exch":"bybit"`,
		`"direction":"bidirectional"`,
		`"score":82`,
		`"streak_cycles":6`,
		`"reason":"score_threshold_met"`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("event JSON missing %q\nfull: %s", want, s)
		}
	}

	// Now demote. fakeGuard zero-value is unblocked.
	for i := 0; i < 6; i++ {
		_ = c.Apply(ctx, summaryWith())
	}
	_ = reg // keep referenced
	if len(sink.events) != 2 {
		t.Fatalf("expected 2 events (promote+demote); got %d", len(sink.events))
	}
	if sink.events[1].Action != "demote" || sink.events[1].Direction != "bidirectional" {
		t.Errorf("demote event shape: %+v", sink.events[1])
	}
}

// TestPromoteTelegramKey — D-13: notifier receives all 7 fields so the impl
// can build cooldown key "pg_promote:promote:BTCUSDT:binance:bybit:bidirectional".
func TestPromoteTelegramKey(t *testing.T) {
	c, _, _, notif, _, _ := newTestController(t, 70, 12)
	ctx := context.Background()
	rec := recAccepted("BTCUSDT", "binance", "bybit", 82)
	for i := 0; i < 6; i++ {
		_ = c.Apply(ctx, summaryWith(rec))
	}
	if len(notif.calls) != 1 {
		t.Fatalf("expected 1 notifier call; got %d", len(notif.calls))
	}
	call := notif.calls[0]
	if call.Action != "promote" || call.Symbol != "BTCUSDT" || call.LongExch != "binance" ||
		call.ShortExch != "bybit" || call.Direction != "bidirectional" || call.Score != 82 || call.Streak != 6 {
		t.Errorf("notifier call missing/wrong fields: %+v", call)
	}
}

// ---- Audit guards: protect against the contract drifts that triggered the
// Wave 2/3 audit replan. ----

// TestPromotion_NoDirectionFromCycleRecord — D-18 audit: source MUST NOT
// contain any read of CycleRecord.Direction (the field doesn't exist).
func TestPromotion_NoDirectionFromCycleRecord(t *testing.T) {
	src, err := os.ReadFile("promotion.go")
	if err != nil {
		t.Fatalf("read promotion.go: %v", err)
	}
	for _, banned := range []string{"rec.Direction", "record.Direction", "Records[0].Direction", "summary.Records[i].Direction"} {
		// Only trip on substrings outside of comment lines: walk lines and
		// strip leading whitespace + // before checking.
		for _, line := range strings.Split(string(src), "\n") {
			trimmed := strings.TrimLeft(line, " \t")
			if strings.HasPrefix(trimmed, "//") {
				continue
			}
			if strings.Contains(trimmed, banned) {
				t.Errorf("audit: promotion.go contains code reference %q — CycleRecord has no Direction field (D-18)", banned)
			}
		}
	}
}

// TestPromotion_CycleSummaryFieldNames — audit: source must reference the real
// CycleSummary field "Records" and must NOT reference the invented "EndedAt".
func TestPromotion_CycleSummaryFieldNames(t *testing.T) {
	src, err := os.ReadFile("promotion.go")
	if err != nil {
		t.Fatalf("read promotion.go: %v", err)
	}
	if !strings.Contains(string(src), "summary.Records") {
		t.Errorf("audit: expected summary.Records reference (real CycleSummary field)")
	}
	for _, banned := range []string{"summary.EndedAt", "summary.endedAt"} {
		for _, line := range strings.Split(string(src), "\n") {
			trimmed := strings.TrimLeft(line, " \t")
			if strings.HasPrefix(trimmed, "//") {
				continue
			}
			if strings.Contains(trimmed, banned) {
				t.Errorf("audit: promotion.go contains %q — CycleSummary uses CompletedAt, not EndedAt", banned)
			}
		}
	}
}

// TestPromotion_BidirectionalLiteralUsed — D-18 audit: literal "bidirectional"
// must appear at minimum 4 times (const + Add candidate + List filter +
// PromoteEvent payload — counted across both promote and demote arms).
func TestPromotion_BidirectionalLiteralUsed(t *testing.T) {
	src, err := os.ReadFile("promotion.go")
	if err != nil {
		t.Fatalf("read promotion.go: %v", err)
	}
	count := strings.Count(string(src), "bidirectional")
	if count < 4 {
		t.Errorf("audit: expected 'bidirectional' literal >=4x (const + Add + List filter + PromoteEvent default), got %d", count)
	}
}
