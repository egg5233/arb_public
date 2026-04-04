package models

import "time"

// AlternativePair represents another viable long/short exchange pair for the
// same symbol. The main Opportunity fields remain the primary selected pair.
type AlternativePair struct {
	LongExchange  string  `json:"long_exchange"`
	ShortExchange string  `json:"short_exchange"`
	LongRate      float64 `json:"long_rate"`
	ShortRate     float64 `json:"short_rate"`
	Spread        float64 `json:"spread"`
	CostRatio     float64 `json:"cost_ratio"`
	Score         float64 `json:"score"`
	IntervalHours float64 `json:"interval_hours"`
}

// Opportunity represents a funding rate arbitrage opportunity between two exchanges.
type Opportunity struct {
	Symbol        string            `json:"symbol"`
	LongExchange  string            `json:"long_exchange"`  // go long here (lower rate, receive funding)
	ShortExchange string            `json:"short_exchange"` // go short here (higher rate)
	LongRate      float64           `json:"long_rate"`      // bps per hour
	ShortRate     float64           `json:"short_rate"`     // bps per hour
	Spread        float64           `json:"spread"`         // ShortRate - LongRate in bps/h (positive = profitable)
	CostRatio     float64           `json:"cost_ratio"`     // total_fees / (spread_bps_h * hold_hours)
	OIRank        int               `json:"oi_rank"`
	Score         float64           `json:"score"`
	IntervalHours float64           `json:"interval_hours"` // funding interval in hours (e.g. 1, 4, 8)
	NextFunding   time.Time         `json:"next_funding"`   // earliest next funding snapshot across both legs
	Source        string            `json:"source"`         // "loris" or "coinglass"
	Timestamp     time.Time         `json:"timestamp"`
	Alternatives  []AlternativePair `json:"alternatives,omitempty"`
}
