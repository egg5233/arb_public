package models

import "time"

// FundingRate represents a single exchange's funding rate for a symbol.
type FundingRate struct {
	Exchange    string        `json:"exchange"`
	Symbol      string        `json:"symbol"`
	Rate        float64       `json:"rate"`         // raw rate in bps per interval
	RateBpsH    float64       `json:"rate_bps_h"`   // bps per hour
	Interval    time.Duration `json:"interval"`     // funding interval for this symbol on this exchange
	NextFunding time.Time     `json:"next_funding"` // next funding snapshot time
	UpdatedAt   time.Time     `json:"updated_at"`
}

// FundingSnapshot holds all exchange rates for a symbol at a point in time.
type FundingSnapshot struct {
	Symbol    string        `json:"symbol"`
	Rates     []FundingRate `json:"rates"`
	Timestamp time.Time     `json:"timestamp"`
}

// LorisResponse represents the response from the Loris API.
type LorisResponse struct {
	Symbols          []string                      `json:"symbols"`
	Exchanges        LorisExchanges                `json:"exchanges"`
	FundingRates     map[string]map[string]float64 `json:"funding_rates"` // exchange -> symbol -> rate
	OIRankings       map[string]interface{}        `json:"oi_rankings"`
	DefaultOIRank    string                        `json:"default_oi_rank"`
	Timestamp        string                        `json:"timestamp"`
	FundingIntervals map[string]map[string]float64 `json:"funding_intervals"` // exchange -> symbol -> hours
}

// LorisExchangeName represents one exchange entry from Loris.
type LorisExchangeName struct {
	Name     string  `json:"name"`
	Display  string  `json:"display"`
	Interval float64 `json:"interval"`
}

// LorisExchanges holds exchange metadata from Loris.
type LorisExchanges struct {
	ExchangeNames []LorisExchangeName `json:"exchange_names"`
	Exchanges     []string            `json:"exchanges"`
}

// CoinGlassResponse represents the CoinGlass arbitrage data stored in Redis.
type CoinGlassResponse struct {
	Timestamp    string         `json:"timestamp"`
	TotalScraped int            `json:"totalScraped"`
	Filtered     int            `json:"filtered"`
	Exchanges    []string       `json:"exchanges"`
	Data         []CoinGlassArb `json:"data"`
}

// CoinGlassArb represents a single arbitrage pair from CoinGlass.
type CoinGlassArb struct {
	Rank        string `json:"rank"`
	Pair        string `json:"pair"`     // "JCT" (base symbol)
	LongPair    string `json:"longPair"` // "JCT/USDT"
	LongEx      string `json:"longEx"`   // "Bitget"
	ShortPair   string `json:"shortPair"`
	ShortEx     string `json:"shortEx"`     // "Gate"
	AnnualYield string `json:"annualYield"` // "432.74%"
	FundingRate string `json:"fundingRate"` // "0.1976%"
	PriceChange string `json:"priceChange"`
	OILong      string `json:"oiLong"` // "$1.64M"
	OIShort     string `json:"oiShort"`
	Countdown   string `json:"countdown"` // "02:14:50"
}
