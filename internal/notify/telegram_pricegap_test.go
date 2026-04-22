package notify

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"arb/internal/models"
)

// newStubTelegram starts an httptest server that captures every Telegram
// sendMessage call and returns a notifier pointed at it. Call .Close() when
// done via t.Cleanup.
func newStubTelegram(t *testing.T) (*TelegramNotifier, *stubSink) {
	t.Helper()
	sink := &stubSink{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		sink.mu.Lock()
		sink.messages = append(sink.messages, r.FormValue("text"))
		sink.mu.Unlock()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(srv.Close)

	prev := telegramAPIBase
	telegramAPIBase = srv.URL
	t.Cleanup(func() { telegramAPIBase = prev })

	tg := NewTelegram("tok", "123")
	if tg == nil {
		t.Fatal("NewTelegram returned nil")
	}
	return tg, sink
}

type stubSink struct {
	mu       sync.Mutex
	messages []string
}

func (s *stubSink) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.messages)
}

func (s *stubSink) last() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.messages) == 0 {
		return ""
	}
	return s.messages[len(s.messages)-1]
}

func (s *stubSink) all() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, len(s.messages))
	copy(out, s.messages)
	return out
}

// waitForMessages waits up to 2s for the sink to reach at least n messages.
// Used because Notify* methods dispatch via `go t.send(...)`.
func (s *stubSink) waitForMessages(t *testing.T, n int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if s.count() >= n {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %d messages; got %d", n, s.count())
}

// ----------------------------------------------------------------------------
// NotifyPriceGapEntry
// ----------------------------------------------------------------------------

func TestNotifyPriceGapEntry_Live(t *testing.T) {
	tg, sink := newStubTelegram(t)
	pos := &models.PriceGapPosition{
		Symbol:         "SOONUSDT",
		LongExchange:   "binance",
		ShortExchange:  "bybit",
		LongSize:       1.2345,
		NotionalUSDT:   200.0,
		EntrySpreadBps: 12.5,
		ModeledSlipBps: 4.0,
		Mode:           models.PriceGapModeLive,
	}
	tg.NotifyPriceGapEntry(pos)
	sink.waitForMessages(t, 1)
	msg := sink.last()
	if !strings.HasPrefix(msg, "PRICE-GAP ENTRY [LIVE]") {
		t.Errorf("expected live prefix, got: %q", msg)
	}
	if strings.Contains(msg, "PAPER") {
		t.Errorf("live message should not contain PAPER, got: %q", msg)
	}
	for _, want := range []string{"SOONUSDT", "binance", "bybit", "12.5", "4.0"} {
		if !strings.Contains(msg, want) {
			t.Errorf("expected message to contain %q, got: %q", want, msg)
		}
	}
}

func TestNotifyPriceGapEntry_Paper(t *testing.T) {
	tg, sink := newStubTelegram(t)
	pos := &models.PriceGapPosition{
		Symbol:       "SOONUSDT",
		Mode:         models.PriceGapModePaper,
		NotionalUSDT: 200.0,
	}
	tg.NotifyPriceGapEntry(pos)
	sink.waitForMessages(t, 1)
	msg := sink.last()
	if !strings.HasPrefix(msg, "\xF0\x9F\x93\x9D PAPER PRICE-GAP ENTRY [PAPER]") {
		t.Errorf("expected paper prefix, got: %q", msg)
	}
}

func TestNotifyPriceGapEntry_Nil(t *testing.T) {
	var tg *TelegramNotifier
	// nil receiver
	tg.NotifyPriceGapEntry(&models.PriceGapPosition{Symbol: "X"})
	// nil pos (should also be safe on non-nil receiver)
	tg2, _ := newStubTelegram(t)
	tg2.NotifyPriceGapEntry(nil)
}

// ----------------------------------------------------------------------------
// NotifyPriceGapExit
// ----------------------------------------------------------------------------

func TestNotifyPriceGapExit_Fields(t *testing.T) {
	tg, sink := newStubTelegram(t)
	pos := &models.PriceGapPosition{
		Symbol:          "SOONUSDT",
		NotionalUSDT:    1000.0,
		RealizedSlipBps: 3.2,
		Mode:            models.PriceGapModeLive,
	}
	tg.NotifyPriceGapExit(pos, "reverted", 5.0, 90*time.Minute)
	sink.waitForMessages(t, 1)
	msg := sink.last()
	for _, want := range []string{"SOONUSDT", "reverted", "5.00", "50.0", "3.2", "1h30m"} {
		if !strings.Contains(msg, want) {
			t.Errorf("expected exit message to contain %q, got: %q", want, msg)
		}
	}
}

func TestNotifyPriceGapExit_PaperPrefix(t *testing.T) {
	tg, sink := newStubTelegram(t)
	pos := &models.PriceGapPosition{
		Symbol:       "X",
		NotionalUSDT: 100,
		Mode:         models.PriceGapModePaper,
	}
	tg.NotifyPriceGapExit(pos, "max_hold", 1.0, time.Hour)
	sink.waitForMessages(t, 1)
	if !strings.HasPrefix(sink.last(), "\xF0\x9F\x93\x9D PAPER ") {
		t.Errorf("expected paper prefix on exit, got: %q", sink.last())
	}
}

func TestNotifyPriceGapExit_ZeroNotionalSafe(t *testing.T) {
	tg, sink := newStubTelegram(t)
	pos := &models.PriceGapPosition{Symbol: "X", NotionalUSDT: 0, Mode: "live"}
	// must not divide by zero
	tg.NotifyPriceGapExit(pos, "reverted", 5.0, time.Minute)
	sink.waitForMessages(t, 1)
	if !strings.Contains(sink.last(), "0.0 bps") {
		t.Errorf("expected 0 bps when notional is 0, got: %q", sink.last())
	}
}

// ----------------------------------------------------------------------------
// NotifyPriceGapRiskBlock
// ----------------------------------------------------------------------------

func TestNotifyPriceGapRiskBlock_Cooldown(t *testing.T) {
	tg, sink := newStubTelegram(t)
	// First call: allowed.
	tg.NotifyPriceGapRiskBlock("SOONUSDT", "concentration", "per-symbol cap hit")
	sink.waitForMessages(t, 1)
	if sink.count() != 1 {
		t.Fatalf("expected 1 message, got %d", sink.count())
	}
	// Second call within cooldown: suppressed.
	tg.NotifyPriceGapRiskBlock("SOONUSDT", "concentration", "per-symbol cap hit")
	time.Sleep(50 * time.Millisecond)
	if sink.count() != 1 {
		t.Errorf("expected cooldown to suppress second call, got %d messages", sink.count())
	}
	// Simulate time passing by rewinding lastSent.
	tg.cooldownMu.Lock()
	for k := range tg.lastSent {
		tg.lastSent[k] = time.Now().Add(-10 * time.Minute)
	}
	tg.cooldownMu.Unlock()
	tg.NotifyPriceGapRiskBlock("SOONUSDT", "concentration", "per-symbol cap hit")
	sink.waitForMessages(t, 2)
	if sink.count() != 2 {
		t.Errorf("expected 2 messages after cooldown expiry, got %d", sink.count())
	}
}

func TestNotifyPriceGapRiskBlock_DifferentKeys(t *testing.T) {
	tg, sink := newStubTelegram(t)
	tg.NotifyPriceGapRiskBlock("SOONUSDT", "concentration", "x")
	tg.NotifyPriceGapRiskBlock("SOONUSDT", "max_concurrent", "x")
	tg.NotifyPriceGapRiskBlock("BTCUSDT", "concentration", "x")
	sink.waitForMessages(t, 3)
	if sink.count() != 3 {
		t.Errorf("expected 3 independent-key messages, got %d", sink.count())
	}
}

func TestNotifyPriceGapRiskBlock_UnknownGate(t *testing.T) {
	tg, sink := newStubTelegram(t)
	tg.NotifyPriceGapRiskBlock("SOONUSDT", "malicious", "bypass attempt")
	tg.NotifyPriceGapRiskBlock("SOONUSDT", "../evil", "crafted")
	tg.NotifyPriceGapRiskBlock("SOONUSDT", "", "empty gate")
	time.Sleep(100 * time.Millisecond)
	if sink.count() != 0 {
		t.Errorf("expected 0 messages for disallowed gates, got %d: %v", sink.count(), sink.all())
	}
}

func TestNotifyPriceGapRiskBlock_DetailSanitized(t *testing.T) {
	tg, sink := newStubTelegram(t)
	long := strings.Repeat("a", 500)
	detail := "line1\x00\x01\x07evil" + long
	tg.NotifyPriceGapRiskBlock("SOONUSDT", "budget", detail)
	sink.waitForMessages(t, 1)
	msg := sink.last()
	// control chars stripped
	for _, ch := range []string{"\x00", "\x01", "\x07"} {
		if strings.Contains(msg, ch) {
			t.Errorf("expected control char %q stripped, got: %q", ch, msg)
		}
	}
	// length cap enforced on the detail portion; full msg includes header so
	// assert the trailing "a" run got truncated.
	if strings.Count(msg, "a") >= 500 {
		t.Errorf("expected detail truncated to <=256 chars, got %d 'a's", strings.Count(msg, "a"))
	}
}

// ----------------------------------------------------------------------------
// Nil-safety guardrail
// ----------------------------------------------------------------------------

func TestNotifyPriceGap_AllNilSafe(t *testing.T) {
	var tg *TelegramNotifier
	tg.NotifyPriceGapEntry(&models.PriceGapPosition{Symbol: "X"})
	tg.NotifyPriceGapExit(&models.PriceGapPosition{Symbol: "X"}, "r", 0, 0)
	tg.NotifyPriceGapRiskBlock("X", "concentration", "d")
	// nil pos too
	tg.NotifyPriceGapEntry(nil)
	tg.NotifyPriceGapExit(nil, "r", 0, 0)
}

// ----------------------------------------------------------------------------
// sanitizeForTelegram unit test
// ----------------------------------------------------------------------------

func TestSanitizeForTelegram(t *testing.T) {
	cases := []struct {
		in   string
		max  int
		want string
	}{
		{"clean", 100, "clean"},
		{"a\x00b\x01c\x07d", 100, "abcd"},
		{"keep\ttab\nnewline", 100, "keep\ttab\nnewline"},
		{strings.Repeat("x", 300), 10, strings.Repeat("x", 10)},
	}
	for _, tc := range cases {
		got := sanitizeForTelegram(tc.in, tc.max)
		if got != tc.want {
			t.Errorf("sanitizeForTelegram(%q, %d) = %q, want %q", tc.in, tc.max, got, tc.want)
		}
	}
}
