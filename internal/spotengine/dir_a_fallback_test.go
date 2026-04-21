package spotengine

import (
	"context"
	"errors"
	"testing"
	"time"

	"arb/pkg/exchange"
)

// ---------------------------------------------------------------------------
// Mock SpotMarginExchange that controls GetMarginInterestRateHistory behavior.
// ---------------------------------------------------------------------------

type fallbackStubSpotMargin struct {
	historyErr error
	history    []exchange.MarginInterestRatePoint
}

func (m *fallbackStubSpotMargin) MarginBorrow(exchange.MarginBorrowParams) error { return nil }
func (m *fallbackStubSpotMargin) MarginRepay(exchange.MarginRepayParams) error   { return nil }
func (m *fallbackStubSpotMargin) PlaceSpotMarginOrder(exchange.SpotMarginOrderParams) (string, error) {
	return "", nil
}
func (m *fallbackStubSpotMargin) GetSpotBBO(string) (exchange.BBO, error) {
	return exchange.BBO{}, nil
}
func (m *fallbackStubSpotMargin) GetMarginInterestRate(string) (*exchange.MarginInterestRate, error) {
	return &exchange.MarginInterestRate{}, nil
}
func (m *fallbackStubSpotMargin) GetMarginBalance(string) (*exchange.MarginBalance, error) {
	return &exchange.MarginBalance{}, nil
}
func (m *fallbackStubSpotMargin) TransferToMargin(string, string) error   { return nil }
func (m *fallbackStubSpotMargin) TransferFromMargin(string, string) error { return nil }
func (m *fallbackStubSpotMargin) SpotOrderRules(string) (*exchange.SpotOrderRules, error) {
	return &exchange.SpotOrderRules{}, nil
}
func (m *fallbackStubSpotMargin) GetMarginInterestRateHistory(_ context.Context, _ string, _, _ time.Time) ([]exchange.MarginInterestRatePoint, error) {
	return m.history, m.historyErr
}

// ---------------------------------------------------------------------------
// canRunDirABacktest
// ---------------------------------------------------------------------------

func TestCanRunDirABacktest_NativeOnlyByDefault(t *testing.T) {
	e, mr := newSpotBacktestEngine(t)
	defer mr.Close()
	e.cfg.SpotFuturesBacktestCoinGlassFallback = false

	cases := map[string]bool{
		"binance": true,
		"bybit":   true,
		"gateio":  true,
		"okx":     false,
		"bitget":  false,
		"bingx":   false,
		"":        false,
	}
	for exch, want := range cases {
		if got := e.canRunDirABacktest(exch); got != want {
			t.Errorf("canRunDirABacktest(%q) flag=off = %v, want %v", exch, got, want)
		}
	}
}

func TestCanRunDirABacktest_WithCoinGlassFallback(t *testing.T) {
	e, mr := newSpotBacktestEngine(t)
	defer mr.Close()
	e.cfg.SpotFuturesBacktestCoinGlassFallback = true

	cases := map[string]bool{
		"binance": true,
		"bybit":   true,
		"gateio":  true,
		"okx":     true, // flipped on by fallback
		"bitget":  true, // flipped on by fallback
		"bingx":   false,
		"":        false,
	}
	for exch, want := range cases {
		if got := e.canRunDirABacktest(exch); got != want {
			t.Errorf("canRunDirABacktest(%q) flag=on = %v, want %v", exch, got, want)
		}
	}
}

// ---------------------------------------------------------------------------
// loadBorrowHistoryWithFallback
// ---------------------------------------------------------------------------

func TestLoadBorrowHistoryWithFallback_NativeSuccess(t *testing.T) {
	e, mr := newSpotBacktestEngine(t)
	defer mr.Close()

	pts := []exchange.MarginInterestRatePoint{
		{Timestamp: time.Unix(1000, 0).UTC(), HourlyRate: 1e-6},
	}
	sm := &fallbackStubSpotMargin{history: pts}

	got, err := e.loadBorrowHistoryWithFallback(context.Background(), sm, "binance", "BTC", time.Unix(0, 0), time.Unix(9999, 0))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].HourlyRate != 1e-6 {
		t.Fatalf("native path did not pass series through: %+v", got)
	}
}

func TestLoadBorrowHistoryWithFallback_FallbackDisabledReturnsSentinel(t *testing.T) {
	e, mr := newSpotBacktestEngine(t)
	defer mr.Close()
	e.cfg.SpotFuturesBacktestCoinGlassFallback = false

	sm := &fallbackStubSpotMargin{historyErr: exchange.ErrHistoricalBorrowNotSupported}

	_, err := e.loadBorrowHistoryWithFallback(context.Background(), sm, "okx", "BTC", time.Unix(0, 0), time.Unix(9999, 0))
	if !errors.Is(err, exchange.ErrHistoricalBorrowNotSupported) {
		t.Fatalf("flag=off should preserve sentinel, got %v", err)
	}
}

func TestLoadBorrowHistoryWithFallback_FallbackOnEmptyRedisReturnsSentinel(t *testing.T) {
	e, mr := newSpotBacktestEngine(t)
	defer mr.Close()
	e.cfg.SpotFuturesBacktestCoinGlassFallback = true

	sm := &fallbackStubSpotMargin{historyErr: exchange.ErrHistoricalBorrowNotSupported}

	_, err := e.loadBorrowHistoryWithFallback(context.Background(), sm, "okx", "BTC", time.Unix(0, 0), time.Unix(9999, 0))
	if !errors.Is(err, exchange.ErrHistoricalBorrowNotSupported) {
		t.Fatalf("flag=on + empty redis should still return sentinel, got %v", err)
	}
}

func TestLoadBorrowHistoryWithFallback_FallbackOnReadsCoinGlass(t *testing.T) {
	e, mr := newSpotBacktestEngine(t)
	defer mr.Close()
	e.cfg.SpotFuturesBacktestCoinGlassFallback = true

	// Seed miniredis with 3 entries that the CoinGlass scraper would have written.
	// Scraper LPUSHes newest-first, so reverse-chronological order here.
	now := time.Now().UTC().Truncate(time.Hour)
	entries := []string{
		`{"t":` + iFormat(now.Unix()) + `,"rate":2.5e-6,"limit":"100"}`,
		`{"t":` + iFormat(now.Add(-time.Hour).Unix()) + `,"rate":2.6e-6,"limit":"100"}`,
		`{"t":` + iFormat(now.Add(-2*time.Hour).Unix()) + `,"rate":2.7e-6,"limit":"100"}`,
	}
	for _, e := range entries {
		mr.Lpush("coinGlassMarginFee:hist:okx:BTC", e)
	}

	sm := &fallbackStubSpotMargin{historyErr: exchange.ErrHistoricalBorrowNotSupported}
	start := now.Add(-24 * time.Hour)
	end := now.Add(time.Minute)

	got, err := e.loadBorrowHistoryWithFallback(context.Background(), sm, "okx", "BTC", start, end)
	if err != nil {
		t.Fatalf("expected fallback success, got error: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 points from CoinGlass fallback, got %d", len(got))
	}
	// Verify at least one point has the expected rate.
	found := false
	for _, p := range got {
		if p.HourlyRate == 2.5e-6 || p.HourlyRate == 2.6e-6 || p.HourlyRate == 2.7e-6 {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected one of the seeded rates (2.5e-6, 2.6e-6, 2.7e-6), got %+v", got)
	}
}

func TestLoadBorrowHistoryWithFallback_FallbackFiltersOutOfRange(t *testing.T) {
	e, mr := newSpotBacktestEngine(t)
	defer mr.Close()
	e.cfg.SpotFuturesBacktestCoinGlassFallback = true

	// Seed one in-range point and one that's 7 days old.
	now := time.Now().UTC().Truncate(time.Hour)
	mr.Lpush("coinGlassMarginFee:hist:bitget:ETH",
		`{"t":`+iFormat(now.Unix())+`,"rate":3.3e-6,"limit":"50"}`)
	mr.Lpush("coinGlassMarginFee:hist:bitget:ETH",
		`{"t":`+iFormat(now.Add(-7*24*time.Hour).Unix())+`,"rate":9.9e-6,"limit":"50"}`)

	sm := &fallbackStubSpotMargin{historyErr: exchange.ErrHistoricalBorrowNotSupported}
	start := now.Add(-24 * time.Hour)
	end := now.Add(time.Minute)

	got, err := e.loadBorrowHistoryWithFallback(context.Background(), sm, "bitget", "ETH", start, end)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 in-range point, got %d: %+v", len(got), got)
	}
	if got[0].HourlyRate != 3.3e-6 {
		t.Fatalf("expected only the in-range point (3.3e-6), got %v", got[0].HourlyRate)
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func iFormat(n int64) string {
	// Simple int64 → string without importing strconv in hot path; used by test literals.
	return formatInt64(n)
}

func formatInt64(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
