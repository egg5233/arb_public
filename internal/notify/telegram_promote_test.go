// Package notify — telegram_promote_test.go: Plan 12-02 Task 2 coverage
// for TelegramNotifier.NotifyPromoteEvent (D-13 cooldown key + D-14 message
// format).
//
// Cooldown contract (D-13):
//   key = "pg_promote:" + action + ":" + symbol + ":" + longExch + ":" +
//         shortExch + ":" + direction
//
// Two distinct candidates therefore have distinct keys → both fire even
// inside the existing 5-minute cooldown window. Same candidate flapping
// within 5min IS throttled (intentional — dashboard timeline carries flap
// detail).
package notify

import (
	"strings"
	"testing"
)

// ---- Test 1: format string carries every field exactly per D-14 ----

func TestNotifyPromoteEvent_FormatsMessageWithAllFields(t *testing.T) {
	tg, sink := newStubTelegram(t)
	tg.NotifyPromoteEvent("promote", "BTCUSDT", "binance", "bybit", "bidirectional", 82, 6)
	sink.waitForMessages(t, 1)

	want := "[PG promote] BTCUSDT binance\xe2\x86\x94bybit (bidirectional) score=82 streak=6"
	got := sink.last()
	if got != want {
		t.Errorf("message drift\n got=%q\nwant=%q", got, want)
	}
}

// ---- Test 2: action="demote" produces "[PG demote] ..." prefix ----

func TestNotifyPromoteEvent_DemoteFormat(t *testing.T) {
	tg, sink := newStubTelegram(t)
	tg.NotifyPromoteEvent("demote", "ETHUSDT", "okx", "gateio", "bidirectional", 40, 6)
	sink.waitForMessages(t, 1)

	got := sink.last()
	if !strings.HasPrefix(got, "[PG demote] ETHUSDT ") {
		t.Errorf("demote format wrong: %q", got)
	}
	if !strings.Contains(got, "score=40 streak=6") {
		t.Errorf("scoreline missing: %q", got)
	}
}

// ---- Test 3: distinct candidates within cooldown both fire ----

func TestNotifyPromoteEvent_UniqueKeyPerEvent(t *testing.T) {
	tg, sink := newStubTelegram(t)
	// Same action ("promote") and symbol but DIFFERENT exchange tuple.
	tg.NotifyPromoteEvent("promote", "BTCUSDT", "binance", "bybit", "bidirectional", 82, 6)
	tg.NotifyPromoteEvent("promote", "BTCUSDT", "okx", "gateio", "bidirectional", 78, 6)
	// Same symbol+tuple but DIFFERENT action → separate key.
	tg.NotifyPromoteEvent("demote", "BTCUSDT", "binance", "bybit", "bidirectional", 40, 6)
	sink.waitForMessages(t, 3)

	if got := sink.count(); got != 3 {
		t.Errorf("messages sent=%d want 3 (each distinct key fires once)", got)
	}
}

// ---- Test 4: same key twice within 5min cooldown → only one fires ----

func TestNotifyPromoteEvent_SameKeySuppressedWithinCooldown(t *testing.T) {
	tg, sink := newStubTelegram(t)
	tg.NotifyPromoteEvent("promote", "BTCUSDT", "binance", "bybit", "bidirectional", 82, 6)
	tg.NotifyPromoteEvent("promote", "BTCUSDT", "binance", "bybit", "bidirectional", 84, 7)
	sink.waitForMessages(t, 1)

	// Give the second goroutine time to attempt + be throttled (cooldown is
	// synchronous inside checkCooldown — second call returns false before
	// goroutine spawn). 100ms is more than enough.
	if got := sink.count(); got != 1 {
		t.Errorf("messages sent=%d want 1 (5min cooldown suppresses second)", got)
	}
}

// ---- Test 5: nil receiver is safe (matches project nil-safe convention) ----

func TestNotifyPromoteEvent_NilReceiverSafe(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("nil receiver panicked: %v", r)
		}
	}()
	var tg *TelegramNotifier
	tg.NotifyPromoteEvent("promote", "BTCUSDT", "binance", "bybit", "bidirectional", 82, 6)
}
