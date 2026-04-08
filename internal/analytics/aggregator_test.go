package analytics

import (
	"math"
	"testing"
	"time"

	"arb/internal/models"
)

func TestAPRCalculation(t *testing.T) {
	// (10/1000) * (8760/24) * 100 = 365.0
	apr := CalculateAPR(10, 1000, 24*time.Hour)
	if math.Abs(apr-365.0) > 0.01 {
		t.Errorf("expected 365.0, got %f", apr)
	}

	// Zero PnL returns 0.
	apr = CalculateAPR(0, 1000, 24*time.Hour)
	if apr != 0.0 {
		t.Errorf("expected 0, got %f", apr)
	}

	// Zero notional returns 0.
	apr = CalculateAPR(10, 0, 24*time.Hour)
	if apr != 0.0 {
		t.Errorf("expected 0, got %f", apr)
	}

	// holdDuration < 1h clamps to 1h: (10/1000) * (8760/1) * 100 = 87600.0
	apr = CalculateAPR(10, 1000, 30*time.Minute)
	expected := (10.0 / 1000.0) * (8760.0 / 1.0) * 100.0 // 87600.0
	if math.Abs(apr-expected) > 0.01 {
		t.Errorf("expected %f (clamped to 1h), got %f", expected, apr)
	}
}

func TestComputeExchangeMetrics(t *testing.T) {
	now := time.Now()
	// 5 closed perp positions: 3 Binance-related, 2 Bybit-related.
	// Position 1: Long=Binance, Short=Bybit, PnL=10, Slippage=0.01
	// Position 2: Long=Binance, Short=Binance, PnL=5, Slippage=0.02
	// Position 3: Long=Bybit, Short=Binance, PnL=5, Slippage=0.03
	// Position 4: Long=Binance, Short=Bybit, PnL=-5, Slippage=0.02
	// Position 5: Long=Bybit, Short=Bybit, PnL=10, Slippage=0.01
	perps := []*models.ArbitragePosition{
		{
			ID: "p1", LongExchange: "Binance", ShortExchange: "Bybit",
			RealizedPnL: 10, Slippage: 0.01, Status: "closed",
			LongEntry: 100, LongSize: 1, ShortEntry: 100, ShortSize: 1,
			CreatedAt: now.Add(-24 * time.Hour), UpdatedAt: now,
		},
		{
			ID: "p2", LongExchange: "Binance", ShortExchange: "Binance",
			RealizedPnL: 5, Slippage: 0.02, Status: "closed",
			LongEntry: 100, LongSize: 1, ShortEntry: 100, ShortSize: 1,
			CreatedAt: now.Add(-24 * time.Hour), UpdatedAt: now,
		},
		{
			ID: "p3", LongExchange: "Bybit", ShortExchange: "Binance",
			RealizedPnL: 5, Slippage: 0.03, Status: "closed",
			LongEntry: 100, LongSize: 1, ShortEntry: 100, ShortSize: 1,
			CreatedAt: now.Add(-24 * time.Hour), UpdatedAt: now,
		},
		{
			ID: "p4", LongExchange: "Binance", ShortExchange: "Bybit",
			RealizedPnL: -5, Slippage: 0.02, Status: "closed",
			LongEntry: 100, LongSize: 1, ShortEntry: 100, ShortSize: 1,
			CreatedAt: now.Add(-24 * time.Hour), UpdatedAt: now,
		},
		{
			ID: "p5", LongExchange: "Bybit", ShortExchange: "Bybit",
			RealizedPnL: 10, Slippage: 0.01, Status: "closed",
			LongEntry: 100, LongSize: 1, ShortEntry: 100, ShortSize: 1,
			CreatedAt: now.Add(-24 * time.Hour), UpdatedAt: now,
		},
	}

	metrics := ComputeExchangeMetrics(perps, nil)
	metricMap := make(map[string]ExchangeMetric)
	for _, m := range metrics {
		metricMap[m.Exchange] = m
	}

	// Binance: p1(long half=5), p2(both halves=5), p3(short half=2.5), p4(long half=-2.5) = 10
	// Trade count: p1, p2(x2), p3, p4 = 5 leg-appearances
	binance := metricMap["Binance"]
	expectedBinanceProfit := 5.0 + 5.0 + 2.5 - 2.5 // = 10.0
	if math.Abs(binance.Profit-expectedBinanceProfit) > 0.01 {
		t.Errorf("Binance profit: expected %f, got %f", expectedBinanceProfit, binance.Profit)
	}

	// Bybit: p1(short half=5), p3(long half=2.5), p4(short half=-2.5), p5(both halves=10) = 15.0
	bybit := metricMap["Bybit"]
	expectedBybitProfit := 5.0 + 2.5 - 2.5 + 10.0 // = 15.0
	if math.Abs(bybit.Profit-expectedBybitProfit) > 0.01 {
		t.Errorf("Bybit profit: expected %f, got %f", expectedBybitProfit, bybit.Profit)
	}
}

func TestComputeExchangeMetrics_IncludesSpotPositions(t *testing.T) {
	now := time.Now()
	perps := []*models.ArbitragePosition{
		{
			ID: "p1", LongExchange: "Binance", ShortExchange: "Bybit",
			RealizedPnL: 10, Status: "closed",
			LongEntry: 100, LongSize: 1, ShortEntry: 100, ShortSize: 1,
			CreatedAt: now.Add(-24 * time.Hour), UpdatedAt: now,
		},
	}

	spots := []*models.SpotFuturesPosition{
		{
			ID: "s1", Exchange: "Binance", RealizedPnL: 20, Status: "closed",
			NotionalUSDT: 1000,
			CreatedAt: now.Add(-48 * time.Hour), UpdatedAt: now,
		},
		{
			ID: "s2", Exchange: "Gate.io", RealizedPnL: -5, Status: "closed",
			NotionalUSDT: 500,
			CreatedAt: now.Add(-24 * time.Hour), UpdatedAt: now,
		},
	}

	metrics := ComputeExchangeMetrics(perps, spots)
	metricMap := make(map[string]ExchangeMetric)
	for _, m := range metrics {
		metricMap[m.Exchange] = m
	}

	// Binance: perp half (5) + spot (20) = 25
	binance := metricMap["Binance"]
	if math.Abs(binance.Profit-25.0) > 0.01 {
		t.Errorf("Binance profit: expected 25.0, got %f", binance.Profit)
	}

	// Gate.io: spot only (-5)
	gateio := metricMap["Gate.io"]
	if math.Abs(gateio.Profit-(-5.0)) > 0.01 {
		t.Errorf("Gate.io profit: expected -5.0, got %f", gateio.Profit)
	}
	if gateio.TradeCount != 1 {
		t.Errorf("Gate.io trade count: expected 1, got %d", gateio.TradeCount)
	}
}

func TestComputeStrategySummary(t *testing.T) {
	now := time.Now()

	perps := []*models.ArbitragePosition{
		{
			// ExitFees is negative (cost) and represents total fees (open+close) after reconciliation.
			// EntryFees=1 is the pre-reconciliation entry fee; ExitFees=-2 overrides it after close.
			ID: "p1", LongExchange: "Binance", ShortExchange: "Bybit",
			RealizedPnL: 10, FundingCollected: 8, EntryFees: 1, ExitFees: -2, HasReconciled: true, Status: "closed",
			LongEntry: 50000, LongSize: 0.01, ShortEntry: 50000, ShortSize: 0.01,
			CreatedAt: now.Add(-48 * time.Hour), UpdatedAt: now,
		},
		{
			// ExitFees=-1 is negative (cost) total fees after reconciliation.
			ID: "p2", LongExchange: "Binance", ShortExchange: "Bybit",
			RealizedPnL: -3, FundingCollected: 2, EntryFees: 0.5, ExitFees: -1, HasReconciled: true, Status: "closed",
			LongEntry: 50000, LongSize: 0.01, ShortEntry: 50000, ShortSize: 0.01,
			CreatedAt: now.Add(-24 * time.Hour), UpdatedAt: now,
		},
	}

	spots := []*models.SpotFuturesPosition{
		{
			ID: "s1", Exchange: "Binance", RealizedPnL: 15, FundingCollected: 12,
			EntryFees: 2, ExitFees: 2, NotionalUSDT: 2000, Status: "closed",
			CreatedAt: now.Add(-72 * time.Hour), UpdatedAt: now,
		},
	}

	summaries := ComputeStrategySummary(perps, spots)
	summaryMap := make(map[string]StrategySummary)
	for _, s := range summaries {
		summaryMap[s.Strategy] = s
	}

	perp := summaryMap["perp"]
	if perp.TradeCount != 2 {
		t.Errorf("perp trade count: expected 2, got %d", perp.TradeCount)
	}
	if math.Abs(perp.TotalPnL-7.0) > 0.01 {
		t.Errorf("perp total PnL: expected 7.0, got %f", perp.TotalPnL)
	}
	if perp.WinCount != 1 {
		t.Errorf("perp win count: expected 1, got %d", perp.WinCount)
	}
	if perp.LossCount != 1 {
		t.Errorf("perp loss count: expected 1, got %d", perp.LossCount)
	}
	if math.Abs(perp.FundingTotal-10.0) > 0.01 {
		t.Errorf("perp funding total: expected 10.0, got %f", perp.FundingTotal)
	}
	if math.Abs(perp.FeesTotal-3.0) > 0.01 {
		t.Errorf("perp fees total: expected 3.0, got %f", perp.FeesTotal)
	}

	// HasReconciled=false → falls back to EntryFees as fee estimate.
	t.Run("fallback_to_entry_fees_when_not_reconciled", func(t *testing.T) {
		unreconciled := []*models.ArbitragePosition{
			{
				ID: "u1", LongExchange: "Binance", ShortExchange: "Bybit",
				RealizedPnL: 5, FundingCollected: 3, EntryFees: 2.0, ExitFees: -4, HasReconciled: false, Status: "closed",
				LongEntry: 50000, LongSize: 0.01, ShortEntry: 50000, ShortSize: 0.01,
				CreatedAt: now.Add(-24 * time.Hour), UpdatedAt: now,
			},
		}
		sums := ComputeStrategySummary(unreconciled, nil)
		var found StrategySummary
		for _, s := range sums {
			if s.Strategy == "perp" {
				found = s
			}
		}
		// HasReconciled=false: fees come from EntryFees (2.0), not abs(ExitFees)=4.
		if math.Abs(found.FeesTotal-2.0) > 0.01 {
			t.Errorf("expected FeesTotal=2.0 (EntryFees fallback), got %f", found.FeesTotal)
		}
	})

	// HasReconciled=true, ExitFees=0 → zero-fee VIP; FeesTotal must be 0, not EntryFees.
	t.Run("zero_fees_vip_when_reconciled_and_exit_fees_zero", func(t *testing.T) {
		vip := []*models.ArbitragePosition{
			{
				ID: "v1", LongExchange: "Binance", ShortExchange: "Bybit",
				RealizedPnL: 8, FundingCollected: 5, EntryFees: 1.5, ExitFees: 0, HasReconciled: true, Status: "closed",
				LongEntry: 50000, LongSize: 0.01, ShortEntry: 50000, ShortSize: 0.01,
				CreatedAt: now.Add(-24 * time.Hour), UpdatedAt: now,
			},
		}
		sums := ComputeStrategySummary(vip, nil)
		var found StrategySummary
		for _, s := range sums {
			if s.Strategy == "perp" {
				found = s
			}
		}
		// HasReconciled=true, ExitFees=0: reconciled zero-fee VIP — FeesTotal must be 0.
		if math.Abs(found.FeesTotal-0.0) > 0.001 {
			t.Errorf("expected FeesTotal=0.0 (VIP zero-fee), got %f", found.FeesTotal)
		}
	})

	spot := summaryMap["spot"]
	if spot.TradeCount != 1 {
		t.Errorf("spot trade count: expected 1, got %d", spot.TradeCount)
	}
	if math.Abs(spot.TotalPnL-15.0) > 0.01 {
		t.Errorf("spot total PnL: expected 15.0, got %f", spot.TotalPnL)
	}
}

func TestComputeWinRate(t *testing.T) {
	rate := ComputeWinRate(7, 10)
	if math.Abs(rate-70.0) > 0.01 {
		t.Errorf("expected 70.0, got %f", rate)
	}

	rate = ComputeWinRate(0, 0)
	if rate != 0 {
		t.Errorf("expected 0 for zero total, got %f", rate)
	}
}
