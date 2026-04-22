package pricegaptrader

import (
	"math"
	"testing"
	"time"

	"arb/internal/models"
)

// fixedNow is the reference clock used across all metrics tests.
// Chosen to match CLAUDE.local.md "today's date" for reviewer sanity.
var fixedNow = time.Date(2026, 4, 22, 12, 0, 0, 0, time.UTC)

// mkPos builds a closed PriceGapPosition with the fields the aggregator needs.
// realizedBps * (notional/10_000) ⇒ realizedPnL, so tests can specify bps directly.
func mkPos(symbol, longEx, shortEx string, ageFromNow time.Duration, notional, realizedBps float64) *models.PriceGapPosition {
	pnl := realizedBps * notional / 10_000.0
	return &models.PriceGapPosition{
		ID:            symbol + "_" + longEx + "_" + shortEx + "_" + ageFromNow.String(),
		Symbol:        symbol,
		LongExchange:  longEx,
		ShortExchange: shortEx,
		Status:        models.PriceGapStatusClosed,
		Mode:          models.PriceGapModeLive,
		NotionalUSDT:  notional,
		RealizedPnL:   pnl,
		// Non-zero slippage fields to demonstrate pg:history carries them.
		ModeledSlipBps:  3,
		RealizedSlipBps: 4,
		ClosedAt:        fixedNow.Add(-ageFromNow),
	}
}

func floatsClose(a, b, tol float64) bool {
	return math.Abs(a-b) <= tol
}

func findRow(t *testing.T, rows []CandidateMetrics, key string) CandidateMetrics {
	t.Helper()
	for _, r := range rows {
		if r.Candidate == key {
			return r
		}
	}
	t.Fatalf("candidate %q not found in %+v", key, rows)
	return CandidateMetrics{}
}

// Empty input → empty slice, no panic.
func TestComputeCandidateMetrics_Empty(t *testing.T) {
	got := ComputeCandidateMetrics(nil, fixedNow)
	if got == nil {
		t.Fatalf("got nil, want non-nil empty slice")
	}
	if len(got) != 0 {
		t.Fatalf("got len=%d, want 0", len(got))
	}

	got2 := ComputeCandidateMetrics([]*models.PriceGapPosition{}, fixedNow)
	if len(got2) != 0 {
		t.Fatalf("got len=%d, want 0", len(got2))
	}
}

// 5 trades on BTCUSDT (Binance/Bybit), realized_bps=[10,20,-5,15,8] →
// WinPct=80, AvgRealizedBps=9.6. All within 24h so Bps24hPerDay = sum/1.
func TestComputeCandidateMetrics_SingleCandidate(t *testing.T) {
	positions := []*models.PriceGapPosition{
		mkPos("BTCUSDT", "binance", "bybit", 1*time.Hour, 1000, 10),
		mkPos("BTCUSDT", "binance", "bybit", 2*time.Hour, 1000, 20),
		mkPos("BTCUSDT", "binance", "bybit", 3*time.Hour, 1000, -5),
		mkPos("BTCUSDT", "binance", "bybit", 4*time.Hour, 1000, 15),
		mkPos("BTCUSDT", "binance", "bybit", 5*time.Hour, 1000, 8),
	}
	got := ComputeCandidateMetrics(positions, fixedNow)
	if len(got) != 1 {
		t.Fatalf("got %d rows, want 1", len(got))
	}
	row := got[0]
	if row.Symbol != "BTCUSDT" || row.LongExchange != "binance" || row.ShortExchange != "bybit" {
		t.Fatalf("identity wrong: %+v", row)
	}
	if row.Candidate != "BTCUSDT_binance_bybit" {
		t.Fatalf("candidate key = %q, want BTCUSDT_binance_bybit", row.Candidate)
	}
	if row.TradesWindow != 5 {
		t.Fatalf("TradesWindow=%d, want 5", row.TradesWindow)
	}
	if !floatsClose(row.WinPct, 80.0, 1e-9) {
		t.Fatalf("WinPct=%v, want 80", row.WinPct)
	}
	if !floatsClose(row.AvgRealizedBps, 9.6, 1e-9) {
		t.Fatalf("AvgRealizedBps=%v, want 9.6", row.AvgRealizedBps)
	}
}

// 24h boundary: trades at -1h, -12h, -25h → only first two in 24h window.
func TestComputeCandidateMetrics_WindowBoundary_24h(t *testing.T) {
	positions := []*models.PriceGapPosition{
		mkPos("BTCUSDT", "binance", "bybit", 1*time.Hour, 1000, 10),
		mkPos("BTCUSDT", "binance", "bybit", 12*time.Hour, 1000, 20),
		mkPos("BTCUSDT", "binance", "bybit", 25*time.Hour, 1000, 40),
	}
	got := ComputeCandidateMetrics(positions, fixedNow)
	row := got[0]
	// 24h window contains 10+20=30 bps → Bps24hPerDay = 30.
	if !floatsClose(row.Bps24hPerDay, 30.0, 1e-9) {
		t.Fatalf("Bps24hPerDay=%v, want 30", row.Bps24hPerDay)
	}
	// All three in 7d window: sum=70 → 70/7=10.
	if !floatsClose(row.Bps7dPerDay, 10.0, 1e-9) {
		t.Fatalf("Bps7dPerDay=%v, want 10", row.Bps7dPerDay)
	}
}

// 7d boundary: -6d23h counts, -7d1h does not.
func TestComputeCandidateMetrics_WindowBoundary_7d(t *testing.T) {
	positions := []*models.PriceGapPosition{
		mkPos("BTCUSDT", "binance", "bybit", 6*24*time.Hour+23*time.Hour, 1000, 70),
		mkPos("BTCUSDT", "binance", "bybit", 7*24*time.Hour+1*time.Hour, 1000, 140),
	}
	got := ComputeCandidateMetrics(positions, fixedNow)
	row := got[0]
	// 7d window: only first trade → 70/7=10.
	if !floatsClose(row.Bps7dPerDay, 10.0, 1e-9) {
		t.Fatalf("Bps7dPerDay=%v, want 10 (only -6d23h counts)", row.Bps7dPerDay)
	}
	// 30d window: both → (70+140)/30=7.
	if !floatsClose(row.Bps30dPerDay, 7.0, 1e-9) {
		t.Fatalf("Bps30dPerDay=%v, want 7", row.Bps30dPerDay)
	}
}

// 30d boundary: -29d counts, -31d does not.
func TestComputeCandidateMetrics_WindowBoundary_30d(t *testing.T) {
	positions := []*models.PriceGapPosition{
		mkPos("BTCUSDT", "binance", "bybit", 29*24*time.Hour, 1000, 60),
		mkPos("BTCUSDT", "binance", "bybit", 31*24*time.Hour, 1000, 90),
	}
	got := ComputeCandidateMetrics(positions, fixedNow)
	row := got[0]
	// 30d: only -29d → 60/30=2.
	if !floatsClose(row.Bps30dPerDay, 2.0, 1e-9) {
		t.Fatalf("Bps30dPerDay=%v, want 2 (only -29d counts)", row.Bps30dPerDay)
	}
	// TradesWindow counts trades in widest window (30d) — only one.
	if row.TradesWindow != 1 {
		t.Fatalf("TradesWindow=%d, want 1 (only -29d within 30d)", row.TradesWindow)
	}
}

// bps/day formula: 140 bps in 7d → Bps7dPerDay=20.
func TestComputeCandidateMetrics_BpsPerDayFormula(t *testing.T) {
	// Two trades inside 7d, each 70 bps → sum=140.
	positions := []*models.PriceGapPosition{
		mkPos("BTCUSDT", "binance", "bybit", 3*24*time.Hour, 1000, 70),
		mkPos("BTCUSDT", "binance", "bybit", 6*24*time.Hour, 1000, 70),
	}
	got := ComputeCandidateMetrics(positions, fixedNow)
	row := got[0]
	if !floatsClose(row.Bps7dPerDay, 20.0, 1e-9) {
		t.Fatalf("Bps7dPerDay=%v, want 20 (140/7)", row.Bps7dPerDay)
	}
}

// 3 symbols → 3 rows, sorted by Bps30dPerDay desc.
func TestComputeCandidateMetrics_MultipleCandidates(t *testing.T) {
	positions := []*models.PriceGapPosition{
		mkPos("BTCUSDT", "binance", "bybit", 1*time.Hour, 1000, 30),    // 30d=1
		mkPos("ETHUSDT", "binance", "bybit", 1*time.Hour, 1000, 90),    // 30d=3
		mkPos("SOLUSDT", "binance", "bybit", 1*time.Hour, 1000, 60),    // 30d=2
	}
	got := ComputeCandidateMetrics(positions, fixedNow)
	if len(got) != 3 {
		t.Fatalf("got %d rows, want 3", len(got))
	}
	if got[0].Symbol != "ETHUSDT" || got[1].Symbol != "SOLUSDT" || got[2].Symbol != "BTCUSDT" {
		t.Fatalf("sort order wrong: %+v", []string{got[0].Symbol, got[1].Symbol, got[2].Symbol})
	}
}

// Same symbol, different exchange pairs → distinct rows keyed by <sym>_<long>_<short>.
func TestComputeCandidateMetrics_CandidateKey(t *testing.T) {
	positions := []*models.PriceGapPosition{
		mkPos("BTCUSDT", "binance", "bybit", 1*time.Hour, 1000, 10),
		mkPos("BTCUSDT", "okx", "gate", 1*time.Hour, 1000, 20),
	}
	got := ComputeCandidateMetrics(positions, fixedNow)
	if len(got) != 2 {
		t.Fatalf("got %d rows, want 2 distinct keys", len(got))
	}
	// Must find both keys.
	_ = findRow(t, got, "BTCUSDT_binance_bybit")
	_ = findRow(t, got, "BTCUSDT_okx_gate")
}

// NotionalUSDT=0 → skip, no divide-by-zero (T-09-24 mitigation).
func TestComputeCandidateMetrics_NoNotionalNoPanic(t *testing.T) {
	positions := []*models.PriceGapPosition{
		mkPos("BTCUSDT", "binance", "bybit", 1*time.Hour, 0, 0), // skipped
		mkPos("BTCUSDT", "binance", "bybit", 2*time.Hour, 1000, 20),
	}
	got := ComputeCandidateMetrics(positions, fixedNow)
	if len(got) != 1 {
		t.Fatalf("got %d rows, want 1", len(got))
	}
	if got[0].TradesWindow != 1 {
		t.Fatalf("TradesWindow=%d, want 1 (zero-notional skipped)", got[0].TradesWindow)
	}
	if !floatsClose(got[0].AvgRealizedBps, 20.0, 1e-9) {
		t.Fatalf("AvgRealizedBps=%v, want 20", got[0].AvgRealizedBps)
	}
}

// Pure function emits NO row for candidates with zero closed trades.
// Padding to the config list is the HANDLER's job (Task 2), not this function's.
func TestComputeCandidateMetrics_ZeroTradesRow(t *testing.T) {
	// Only BTCUSDT has a trade — ETHUSDT not present in input must not appear.
	positions := []*models.PriceGapPosition{
		mkPos("BTCUSDT", "binance", "bybit", 1*time.Hour, 1000, 10),
	}
	got := ComputeCandidateMetrics(positions, fixedNow)
	if len(got) != 1 {
		t.Fatalf("got %d rows, want 1 (pure fn does NOT pad)", len(got))
	}
	if got[0].Symbol != "BTCUSDT" {
		t.Fatalf("got symbol %q, want BTCUSDT", got[0].Symbol)
	}
}

// D-24 simplification: pg:history fields alone are sufficient for non-zero metrics —
// no pg:slippage:* read necessary.
func TestComputeCandidateMetrics_HistoryOnlyDataSource(t *testing.T) {
	positions := []*models.PriceGapPosition{
		mkPos("BTCUSDT", "binance", "bybit", 1*24*time.Hour, 1000, 15),
		mkPos("BTCUSDT", "binance", "bybit", 2*24*time.Hour, 1000, 10),
		mkPos("BTCUSDT", "binance", "bybit", 3*24*time.Hour, 1000, 20),
		mkPos("BTCUSDT", "binance", "bybit", 4*24*time.Hour, 1000, -5),
		mkPos("BTCUSDT", "binance", "bybit", 5*24*time.Hour, 1000, 25),
	}
	// All five have non-zero ModeledSlipBps + RealizedSlipBps + RealizedPnL +
	// NotionalUSDT + ClosedAt (see mkPos). Proves pg:history alone suffices.
	got := ComputeCandidateMetrics(positions, fixedNow)
	if len(got) != 1 {
		t.Fatalf("got %d rows, want 1", len(got))
	}
	row := got[0]
	if row.Bps7dPerDay == 0 {
		t.Fatalf("Bps7dPerDay=0, want non-zero (D-24: pg:history alone must suffice)")
	}
	// Sanity check on value: sum=65 bps, 7d → 65/7 ≈ 9.2857.
	if !floatsClose(row.Bps7dPerDay, 65.0/7.0, 1e-9) {
		t.Fatalf("Bps7dPerDay=%v, want %v", row.Bps7dPerDay, 65.0/7.0)
	}
}
