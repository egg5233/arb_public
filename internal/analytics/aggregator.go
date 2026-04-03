package analytics

import (
	"math"
	"time"

	"arb/internal/models"
)

// hoursPerYear is the standard annualization constant.
const hoursPerYear = 8760.0

// CalculateAPR computes annualized percentage return.
// Formula: (pnl / notional) * (8760 / holdHours) * 100
// Clamps holdHours to minimum 1.0 to avoid division issues.
// Returns 0 if notional <= 0 or holdDuration <= 0.
func CalculateAPR(pnl, notional float64, holdDuration time.Duration) float64 {
	if notional <= 0 || holdDuration <= 0 {
		return 0
	}
	hours := holdDuration.Hours()
	if hours < 1.0 {
		hours = 1.0
	}
	return (pnl / notional) * (hoursPerYear / hours) * 100.0
}

// ExchangeMetric holds aggregated metrics for one exchange.
type ExchangeMetric struct {
	Exchange    string  `json:"exchange"`
	Profit      float64 `json:"profit"`
	TradeCount  int     `json:"trade_count"`
	WinCount    int     `json:"win_count"`
	LossCount   int     `json:"loss_count"`
	WinRate     float64 `json:"win_rate"`      // percentage 0-100
	AvgSlippage float64 `json:"avg_slippage"`  // average absolute slippage
	APR         float64 `json:"apr"`           // annualized return
}

// exchangeAccum is an internal accumulator for building ExchangeMetric.
type exchangeAccum struct {
	profit       float64
	tradeCount   int
	winCount     int
	lossCount    int
	slippageSum  float64
	slippageCnt  int
	notionalSum  float64
	durationSum  time.Duration
}

// ComputeExchangeMetrics aggregates metrics per exchange from both perp-perp and spot-futures history.
// For perp-perp: uses LongExchange and ShortExchange (each position spans 2 exchanges -- split PnL 50/50).
// For spot-futures: uses Exchange field (single exchange).
func ComputeExchangeMetrics(perps []*models.ArbitragePosition, spots []*models.SpotFuturesPosition) []ExchangeMetric {
	accum := make(map[string]*exchangeAccum)

	getOrCreate := func(name string) *exchangeAccum {
		if a, ok := accum[name]; ok {
			return a
		}
		a := &exchangeAccum{}
		accum[name] = a
		return a
	}

	for _, p := range perps {
		halfPnL := p.RealizedPnL / 2.0
		notional := math.Max(p.LongEntry*p.LongSize, p.ShortEntry*p.ShortSize)
		halfNotional := notional / 2.0
		duration := p.UpdatedAt.Sub(p.CreatedAt)
		isWin := p.RealizedPnL > 0

		// Attribute to long exchange.
		longAcc := getOrCreate(p.LongExchange)
		longAcc.profit += halfPnL
		longAcc.tradeCount++
		if isWin {
			longAcc.winCount++
		} else {
			longAcc.lossCount++
		}
		if p.Slippage != 0 {
			longAcc.slippageSum += math.Abs(p.Slippage)
			longAcc.slippageCnt++
		}
		longAcc.notionalSum += halfNotional
		longAcc.durationSum += duration

		// Attribute to short exchange.
		shortAcc := getOrCreate(p.ShortExchange)
		shortAcc.profit += halfPnL
		shortAcc.tradeCount++
		if isWin {
			shortAcc.winCount++
		} else {
			shortAcc.lossCount++
		}
		if p.Slippage != 0 {
			shortAcc.slippageSum += math.Abs(p.Slippage)
			shortAcc.slippageCnt++
		}
		shortAcc.notionalSum += halfNotional
		shortAcc.durationSum += duration
	}

	for _, s := range spots {
		acc := getOrCreate(s.Exchange)
		acc.profit += s.RealizedPnL
		acc.tradeCount++
		if s.RealizedPnL > 0 {
			acc.winCount++
		} else {
			acc.lossCount++
		}
		acc.notionalSum += s.NotionalUSDT
		acc.durationSum += s.UpdatedAt.Sub(s.CreatedAt)
	}

	result := make([]ExchangeMetric, 0, len(accum))
	for name, a := range accum {
		m := ExchangeMetric{
			Exchange:   name,
			Profit:     a.profit,
			TradeCount: a.tradeCount,
			WinCount:   a.winCount,
			LossCount:  a.lossCount,
			WinRate:    ComputeWinRate(a.winCount, a.tradeCount),
		}
		if a.slippageCnt > 0 {
			m.AvgSlippage = a.slippageSum / float64(a.slippageCnt)
		}
		if a.notionalSum > 0 && a.durationSum > 0 {
			avgDuration := a.durationSum / time.Duration(a.tradeCount)
			m.APR = CalculateAPR(a.profit, a.notionalSum, avgDuration)
		}
		result = append(result, m)
	}
	return result
}

// StrategySummary holds aggregated metrics for one strategy.
type StrategySummary struct {
	Strategy     string  `json:"strategy"`       // "perp" or "spot"
	TotalPnL     float64 `json:"total_pnl"`
	TradeCount   int     `json:"trade_count"`
	WinCount     int     `json:"win_count"`
	LossCount    int     `json:"loss_count"`
	WinRate      float64 `json:"win_rate"`       // percentage 0-100
	APR          float64 `json:"apr"`            // weighted average APR
	AvgHoldHours float64 `json:"avg_hold_hours"`
	FundingTotal float64 `json:"funding_total"`
	FeesTotal    float64 `json:"fees_total"`
}

// ComputeStrategySummary computes per-strategy aggregates.
func ComputeStrategySummary(perps []*models.ArbitragePosition, spots []*models.SpotFuturesPosition) []StrategySummary {
	var summaries []StrategySummary

	// Perp strategy.
	if len(perps) > 0 {
		var s StrategySummary
		s.Strategy = "perp"
		var totalDuration time.Duration
		var totalNotional float64

		for _, p := range perps {
			s.TotalPnL += p.RealizedPnL
			s.TradeCount++
			if p.RealizedPnL > 0 {
				s.WinCount++
			} else {
				s.LossCount++
			}
			s.FundingTotal += p.FundingCollected
			s.FeesTotal += p.EntryFees + p.ExitFees

			dur := p.UpdatedAt.Sub(p.CreatedAt)
			totalDuration += dur
			notional := math.Max(p.LongEntry*p.LongSize, p.ShortEntry*p.ShortSize)
			totalNotional += notional
		}

		s.WinRate = ComputeWinRate(s.WinCount, s.TradeCount)
		if s.TradeCount > 0 {
			avgDur := totalDuration / time.Duration(s.TradeCount)
			s.AvgHoldHours = avgDur.Hours()
		}
		if totalNotional > 0 && totalDuration > 0 {
			avgDur := totalDuration / time.Duration(s.TradeCount)
			s.APR = CalculateAPR(s.TotalPnL, totalNotional, avgDur)
		}
		summaries = append(summaries, s)
	}

	// Spot strategy.
	if len(spots) > 0 {
		var s StrategySummary
		s.Strategy = "spot"
		var totalDuration time.Duration
		var totalNotional float64

		for _, sp := range spots {
			s.TotalPnL += sp.RealizedPnL
			s.TradeCount++
			if sp.RealizedPnL > 0 {
				s.WinCount++
			} else {
				s.LossCount++
			}
			s.FundingTotal += sp.FundingCollected
			s.FeesTotal += sp.EntryFees + sp.ExitFees

			dur := sp.UpdatedAt.Sub(sp.CreatedAt)
			totalDuration += dur
			totalNotional += sp.NotionalUSDT
		}

		s.WinRate = ComputeWinRate(s.WinCount, s.TradeCount)
		if s.TradeCount > 0 {
			avgDur := totalDuration / time.Duration(s.TradeCount)
			s.AvgHoldHours = avgDur.Hours()
		}
		if totalNotional > 0 && totalDuration > 0 {
			avgDur := totalDuration / time.Duration(s.TradeCount)
			s.APR = CalculateAPR(s.TotalPnL, totalNotional, avgDur)
		}
		summaries = append(summaries, s)
	}

	return summaries
}

// ComputeWinRate computes win rate as a percentage (0-100).
// Returns 0 if total == 0.
func ComputeWinRate(wins, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(wins) / float64(total) * 100.0
}
