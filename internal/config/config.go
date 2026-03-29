package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// normalizeChain maps chain name variants to a canonical form.
func normalizeChain(chain string) string {
	switch strings.ToLower(chain) {
	case "bsc", "bep20":
		return "BEP20"
	case "apt", "aptos":
		return "APT"
	default:
		return strings.ToUpper(chain)
	}
}

// Config holds all application configuration.
type Config struct {
	// Strategy parameters
	MinHoldTime         time.Duration // timed exit hold duration (default 16h)
	MaxCostRatio        float64       // max fees/revenue ratio for profitability filter (default 0.50)
	MaxPositions        int           // max concurrent arb positions (default 1)
	Leverage            int           // default leverage (default 3)
	SlippageBPS         float64       // max acceptable orderbook slippage in bps (default 5)
	RebalanceScanMinute int           // minute mark that triggers rebalance (default 20)
	TopOpportunities    int           // max opportunities to return from scanner (default 5)
	CapitalPerLeg       float64       // fixed USDT per leg; 0 = auto from balance (default 0)
	PriceGapFreeBPS     float64       // below this gap, no check (default 40)
	MaxPriceGapBPS      float64       // hard reject above this gap (default 250)
	MaxGapRecoveryIntervals float64   // max funding intervals to recover gap (default 1.0)
	MaxIntervalHours    float64       // max funding interval hours to accept (0=disabled, e.g. 1=only 1h)
	AllowMixedIntervals bool          // allow cross-interval pairs in ranker (default false)
	DryRun              bool          // if true, skip trade execution (log only)

	// Depth-driven entry execution
	EntryTimeoutSec int     // max seconds for depth fill loop (default 60)
	MinChunkUSDT    float64 // min notional per micro-order to avoid dust (default 5)

	// Margin health thresholds (marginRatio values, 0-1 scale where 1.0 = liquidation)
	MarginL3Threshold float64 // trigger fund transfer (default: 0.50)
	MarginL4Threshold float64 // trigger position reduction (default: 0.80)
	MarginL5Threshold float64 // trigger emergency close (default: 0.95)
	L4ReduceFraction      float64 // fraction to reduce at L4 (default: 0.50)
	MarginSafetyMultiplier float64 // margin buffer multiplier for entry check (default: 2.0)

	// Exit strategy
	ExitDepthTimeoutSec  int     // depth-fill exit loop timeout before market fallback (default 300)
	ExitMaxGapBPS        float64 // max cross-exchange gap at exit; ramps to 3x over timeout (default 10)

	// Risk monitor (log-only)
	RiskMonitorIntervalSec int // interval between risk monitor checks in seconds (default: 300)

	// Persistence filter: require opportunities to appear across multiple scans
	PersistLookback1h time.Duration // lookback window for 1h-interval pairs (default 15m)
	PersistMinCount1h int           // min appearances in lookback for 1h pairs (default 2)
	PersistLookback4h time.Duration // lookback window for 4h-interval pairs (default 30m)
	PersistMinCount4h int           // min appearances in lookback for 4h pairs (default 4)
	PersistLookback8h time.Duration // lookback window for 8h-interval pairs (default 40m)
	PersistMinCount8h int           // min appearances in lookback for 8h pairs (default 5)

	// Spread stability: reject opportunities with volatile spreads across scans
	SpreadStabilityRatio1h  float64 // min/max spread ratio required for 1h pairs (default 0.5, 0=disabled)
	SpreadStabilityOIRank1h int     // only check stability when OIRank >= this (default 100)
	SpreadStabilityRatio4h  float64 // min/max spread ratio for 4h pairs (default 0, disabled)
	SpreadStabilityOIRank4h int     // OI rank threshold for 4h (default 0, disabled)
	SpreadStabilityRatio8h  float64 // min/max spread ratio for 8h pairs (default 0, disabled)
	SpreadStabilityOIRank8h int     // OI rank threshold for 8h (default 0, disabled)

	// Exit tuning
	EnableSpreadReversal    bool // enable spread reversal exit check (default true)
	SpreadReversalTolerance int  // allow N reversals before triggering exit (default 0 = exit on first)
	ZeroSpreadTolerance     int // exit after N consecutive checks where both legs have equal funding rate (0=disabled)

	// Anti-spike filters
	SpreadVolatilityMaxCV      float64 // max stddev/mean for spread across scans (default 0.5, 0=disabled)
	SpreadVolatilityMinSamples int     // min scan records to evaluate CV (default 3)
	FundingWindowMin           int     // max minutes before funding to allow entry (default 30)
	LossCooldownHours          float64 // hours to blacklist symbol after loss close (default 4.0)
	ReEnterCooldownHours       float64 // hours to block re-entry on same symbol after any close (default 0, disabled)
	BacktestDays               int     // days of historical funding to check (default 3, 0 = disabled)
	BacktestMinProfit          float64 // minimum net profit to pass backtest filter (default 0)
	DelistFilterEnabled        bool    // enable Binance delist monitoring & filtering (default true)

	// Scan schedule
	ScanMinutes      []int // minutes within each hour when scans fire (default [5,15,25,35,45,55])
	EntryScanMinute    int // minute mark that triggers trade execution (default 35)
	ExitScanMinute     int // minute mark that triggers exit checks (default 25)
	RotateScanMinute   int // minute mark that triggers rotation checks (default 45)

	// Leg rotation parameters
	RotationThresholdBPS float64 // min spread improvement to trigger rotation (default: 20)
	RotationCooldownMin  int     // cooldown after rotation before next allowed (default: 30)

	// Exchange API keys
	BinanceAPIKey    string
	BinanceSecretKey string
	BinanceEnabled   *bool // nil = auto (enabled if apiKey set)
	BybitAPIKey      string
	BybitSecretKey   string
	BybitEnabled     *bool
	GateioAPIKey     string
	GateioSecretKey  string
	GateioEnabled    *bool
	BitgetAPIKey     string
	BitgetSecretKey  string
	BitgetPassphrase string
	BitgetEnabled    *bool
	OKXAPIKey        string
	OKXSecretKey     string
	OKXPassphrase    string
	OKXEnabled       *bool
	BingXAPIKey      string
	BingXSecretKey   string
	BingXEnabled     *bool

	// Redis
	RedisAddr string
	RedisPass string
	RedisDB   int

	// Dashboard
	DashboardAddr     string
	DashboardPassword string

	// Exchange deposit addresses: exchange -> chain -> address
	ExchangeAddresses map[string]map[string]string

	// AI Diagnose
	AIEndpoint  string
	AIAPIKey    string
	AIModel     string
	AIMaxTokens int

	// Spot-futures arbitrage scraper
	SpotArbEnabled    bool
	SpotArbSchedule   string // comma-separated minutes, e.g. "15,35"
	SpotArbChromePath string

	// Spot-futures arbitrage engine
	SpotFuturesEnabled           bool
	SpotFuturesMaxPositions      int
	SpotFuturesCapitalPerPosition float64
	SpotFuturesLeverage          int
	SpotFuturesMonitorIntervalSec int
	SpotFuturesMinNetYieldAPR    float64  // minimum net APR after costs (decimal, e.g. 0.10 = 10%)
	SpotFuturesMaxBorrowAPR      float64  // maximum borrow APR for Direction A (decimal, e.g. 0.50 = 50%)
	SpotFuturesExchanges         []string // exchanges to consider (empty = all SpotMargin-capable)
	SpotFuturesScanIntervalMin   int      // discovery scan interval in minutes
}

// ---------- Nested JSON config structs ----------

type jsonConfig struct {
	DryRun    *bool                   `json:"dry_run"`
	Exchanges map[string]jsonExchange `json:"exchanges"`
	Redis     *jsonRedis              `json:"redis"`
	Dashboard *jsonDashboard          `json:"dashboard"`
	Strategy  *jsonStrategy           `json:"strategy"`
	Fund      *jsonFund               `json:"fund"`
	Risk      *jsonRisk               `json:"risk"`
	AI        *jsonAI                 `json:"ai"`
	SpotArb     *jsonSpotArb            `json:"spot_arb"`
	SpotFutures *jsonSpotFutures        `json:"spot_futures"`
}

type jsonSpotArb struct {
	Enabled    *bool  `json:"enabled"`
	Schedule   string `json:"schedule"`
	ChromePath string `json:"chrome_path"`
}

type jsonSpotFutures struct {
	Enabled            *bool    `json:"enabled"`
	MaxPositions       *int     `json:"max_positions"`
	CapitalPerPosition *float64 `json:"capital_per_position"`
	Leverage           *int     `json:"leverage"`
	MonitorIntervalSec *int     `json:"monitor_interval_sec"`
	MinNetYieldAPR     *float64 `json:"min_net_yield_apr"`
	MaxBorrowAPR       *float64 `json:"max_borrow_apr"`
	Exchanges          []string `json:"exchanges"`
	ScanIntervalMin    *int     `json:"scan_interval_min"`
}

type jsonExchange struct {
	Enabled    *bool             `json:"enabled,omitempty"`
	APIKey     string            `json:"api_key"`
	SecretKey  string            `json:"secret_key"`
	Passphrase string            `json:"passphrase,omitempty"`
	Address    map[string]string `json:"address,omitempty"`
}

type jsonRedis struct {
	Addr     string `json:"addr"`
	Password string `json:"password"`
	DB       *int   `json:"db"`
}

type jsonDashboard struct {
	Addr     string `json:"addr"`
	Password string `json:"password"`
}

type jsonAI struct {
	Endpoint  string `json:"endpoint"`
	APIKey    string `json:"api_key"`
	Model     string `json:"model"`
	MaxTokens *int   `json:"max_tokens"`
}

type jsonStrategy struct {
	TopOpportunities *int           `json:"top_opportunities"`
	ScanMinutes      []int          `json:"scan_minutes"`
	EntryScanMinute     *int           `json:"entry_scan_minute"`
	ExitScanMinute      *int           `json:"exit_scan_minute"`
	RotateScanMinute    *int           `json:"rotate_scan_minute"`
	RebalanceScanMinute *int           `json:"rebalance_scan_minute"`
	Discovery        *jsonDiscovery `json:"discovery"`
	Entry            *jsonEntry     `json:"entry"`
	Exit             *jsonExit      `json:"exit"`
	Rotation         *jsonRotation  `json:"rotation"`
}

type jsonDiscovery struct {
	MinHoldTimeHours    *int             `json:"min_hold_time_hours"`
	MaxCostRatio        *float64         `json:"max_cost_ratio"`
	MaxPriceGapBPS      *float64         `json:"max_price_gap_bps"`
	PriceGapFreeBPS     *float64         `json:"price_gap_free_bps"`
	MaxGapRecoveryIntervals *float64     `json:"max_gap_recovery_intervals"`
	MaxIntervalHours    *float64         `json:"max_interval_hours"`
	AllowMixedIntervals *bool            `json:"allow_mixed_intervals"`
	DelistFilter        *bool            `json:"delist_filter"`
	Persistence         *jsonPersistence `json:"persistence"`
}

type jsonPersistence struct {
	LookbackMin1h *int `json:"lookback_min_1h"`
	MinCount1h    *int `json:"min_count_1h"`
	LookbackMin4h *int `json:"lookback_min_4h"`
	MinCount4h    *int `json:"min_count_4h"`
	LookbackMin8h *int `json:"lookback_min_8h"`
	MinCount8h    *int `json:"min_count_8h"`

	SpreadStabilityRatio1h  *float64 `json:"spread_stability_ratio_1h"`
	SpreadStabilityOIRank1h *int     `json:"spread_stability_oi_rank_1h"`
	SpreadStabilityRatio4h  *float64 `json:"spread_stability_ratio_4h"`
	SpreadStabilityOIRank4h *int     `json:"spread_stability_oi_rank_4h"`
	SpreadStabilityRatio8h  *float64 `json:"spread_stability_ratio_8h"`
	SpreadStabilityOIRank8h *int     `json:"spread_stability_oi_rank_8h"`

	SpreadVolatilityMaxCV      *float64 `json:"spread_volatility_max_cv"`
	SpreadVolatilityMinSamples *int     `json:"spread_volatility_min_samples"`
	FundingWindowMin           *int     `json:"funding_window_min"`
}

type jsonEntry struct {
	SlippageLimitBPS     *float64 `json:"slippage_limit_bps"`
	MinChunkUSDT         *float64 `json:"min_chunk_usdt"`
	EntryTimeoutSec      *int     `json:"entry_timeout_sec"`
	LossCooldownHours    *float64 `json:"loss_cooldown_hours"`
	ReEnterCooldownHours *float64 `json:"re_enter_cooldown_hours"`
	BacktestDays         *int     `json:"backtest_days"`
	BacktestMinProfit    *float64 `json:"backtest_min_profit"`
}

type jsonExit struct {
	DepthTimeoutSec         *int      `json:"depth_timeout_sec"`
	EnableSpreadReversal    *bool     `json:"enable_spread_reversal"`
	SpreadReversalTolerance *int      `json:"spread_reversal_tolerance"`
	ZeroSpreadTolerance     *int     `json:"zero_spread_tolerance"`
	MaxGapBPS               *float64  `json:"max_gap_bps"`
}

type jsonRotation struct {
	ThresholdBPS *float64 `json:"threshold_bps"`
	CooldownMin  *int     `json:"cooldown_min"`
}

type jsonFund struct {
	MaxPositions        *int     `json:"max_positions"`
	Leverage            *int     `json:"leverage"`
	CapitalPerLeg       *float64 `json:"capital_per_leg"`
	RebalanceScanMinute *int     `json:"rebalance_scan_minute"`  // legacy: also accepted here
	RebalanceAdvanceMin *int     `json:"rebalance_advance_min"`  // deprecated alias
}

type jsonRisk struct {
	MarginL3Threshold      *float64 `json:"margin_l3_threshold"`
	MarginL4Threshold      *float64 `json:"margin_l4_threshold"`
	MarginL5Threshold      *float64 `json:"margin_l5_threshold"`
	L4ReduceFraction        *float64 `json:"l4_reduce_fraction"`
	MarginSafetyMultiplier  *float64 `json:"margin_safety_multiplier"`
	RiskMonitorIntervalSec  *int     `json:"risk_monitor_interval_sec"`
}

// detectChromePath returns the first Chrome binary found in common locations.
func detectChromePath() string {
	candidates := []string{
		"/home/solana/.cache/puppeteer/chrome/linux-146.0.7680.153/chrome-linux64/chrome",
		"/usr/bin/google-chrome",
		"/usr/bin/chromium-browser",
		"/usr/bin/chromium",
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// Load reads configuration from config.json (if present), with env var overrides.
// Priority: env var > config.json > default value.
func Load() *Config {
	c := &Config{
		MinHoldTime:             16 * time.Hour,
		MaxCostRatio:            0.50,
		MaxPositions:            1,
		Leverage:                3,
		SlippageBPS:             50,
		RebalanceScanMinute:     10,
		TopOpportunities:        25,
		PriceGapFreeBPS:         40,
		MaxPriceGapBPS:          200,
		MaxGapRecoveryIntervals: 1.0,
		EntryTimeoutSec:         300,
		MinChunkUSDT:            10,
		MarginL3Threshold:       0.50,
		MarginL4Threshold:       0.80,
		MarginL5Threshold:       0.95,
		L4ReduceFraction:           0.30,
		MarginSafetyMultiplier:     2.0,
		ExitDepthTimeoutSec:     300,
		ExitMaxGapBPS:           10.0,
		RiskMonitorIntervalSec:  300,
		PersistLookback1h:       90 * time.Minute,
		PersistMinCount1h:       1,
		PersistLookback4h:       180 * time.Minute,
		PersistMinCount4h:       1,
		PersistLookback8h:       360 * time.Minute,
		PersistMinCount8h:       1,
		SpreadStabilityRatio1h:  0.5,
		SpreadStabilityOIRank1h: 0,
		SpreadVolatilityMaxCV:      0,
		SpreadVolatilityMinSamples: 10,
		FundingWindowMin:           30,
		LossCooldownHours:          4.0,
		BacktestDays:               3,
		DelistFilterEnabled:        true,
		ScanMinutes:             []int{10, 20, 30, 35, 40, 45, 50},
		EntryScanMinute:         40,
		ExitScanMinute:          30,
		RotateScanMinute:        35,
		RotationThresholdBPS:    100,
		RotationCooldownMin:        180,
		EnableSpreadReversal:       true,
		SpreadReversalTolerance:    1,
		ZeroSpreadTolerance:        2,
		ReEnterCooldownHours:       1.0,
		RedisAddr:               "localhost:6379",
		RedisDB:                 2,
		DashboardAddr:           ":8080",
		AIModel:                 "gpt-5.4",
		AIMaxTokens:             4096,
		SpotArbSchedule:         "15,35",
		SpotArbChromePath:       detectChromePath(),
		SpotFuturesMaxPositions:       1,
		SpotFuturesCapitalPerPosition: 200,
		SpotFuturesLeverage:           3,
		SpotFuturesMonitorIntervalSec: 300,
		SpotFuturesMinNetYieldAPR:     0.10, // 10%
		SpotFuturesMaxBorrowAPR:       0.50, // 50%
		SpotFuturesScanIntervalMin:    10,
	}

	// Load from JSON file
	c.loadJSON()

	// Environment variables override JSON values
	c.loadEnvOverrides()

	// Ensure special scan minutes are present in the schedule.
	c.EnsureScanMinutes()

	return c
}

// EnsureScanMinutes adds rebalance/exit/entry/rotate minutes to ScanMinutes
// if they are not already present, then sorts the slice.
func (c *Config) EnsureScanMinutes() {
	required := []int{c.RebalanceScanMinute, c.ExitScanMinute, c.EntryScanMinute, c.RotateScanMinute}
	have := make(map[int]bool, len(c.ScanMinutes))
	for _, m := range c.ScanMinutes {
		have[m] = true
	}
	for _, m := range required {
		if !have[m] {
			c.ScanMinutes = append(c.ScanMinutes, m)
			have[m] = true
			fmt.Printf("[config] auto-added :%02d to scan_minutes\n", m)
		}
	}
	// Sort ascending
	for i := 0; i < len(c.ScanMinutes); i++ {
		for j := i + 1; j < len(c.ScanMinutes); j++ {
			if c.ScanMinutes[j] < c.ScanMinutes[i] {
				c.ScanMinutes[i], c.ScanMinutes[j] = c.ScanMinutes[j], c.ScanMinutes[i]
			}
		}
	}
}

func (c *Config) loadJSON() {
	// Try config.json in current directory, then /var/solana/data/arb/config.json
	paths := []string{"config.json", "/var/solana/data/arb/config.json"}
	if p := os.Getenv("CONFIG_FILE"); p != "" {
		paths = []string{p}
	}

	var data []byte
	var err error
	for _, path := range paths {
		data, err = os.ReadFile(path)
		if err == nil {
			fmt.Printf("[config] Loaded from %s\n", path)
			break
		}
	}
	if data == nil {
		return // no config file found, use defaults + env
	}

	var jc jsonConfig
	if err := json.Unmarshal(data, &jc); err != nil {
		fmt.Printf("[config] WARNING: failed to parse config.json: %v\n", err)
		return
	}

	c.applyJSON(&jc)
}

func (c *Config) applyJSON(jc *jsonConfig) {
	if jc.DryRun != nil {
		c.DryRun = *jc.DryRun
	}

	// Strategy
	if s := jc.Strategy; s != nil {
		if s.TopOpportunities != nil {
			c.TopOpportunities = *s.TopOpportunities
		}
		if len(s.ScanMinutes) > 0 {
			c.ScanMinutes = s.ScanMinutes
		}
		if s.EntryScanMinute != nil {
			c.EntryScanMinute = *s.EntryScanMinute
		}
		if s.ExitScanMinute != nil {
			c.ExitScanMinute = *s.ExitScanMinute
		}
		if s.RotateScanMinute != nil {
			c.RotateScanMinute = *s.RotateScanMinute
		}
		if s.RebalanceScanMinute != nil {
			c.RebalanceScanMinute = *s.RebalanceScanMinute
		}

		// Discovery
		if d := s.Discovery; d != nil {
			if d.MinHoldTimeHours != nil {
				c.MinHoldTime = time.Duration(*d.MinHoldTimeHours) * time.Hour
			}
			if d.MaxCostRatio != nil {
				c.MaxCostRatio = *d.MaxCostRatio
			}
			if d.MaxPriceGapBPS != nil {
				c.MaxPriceGapBPS = *d.MaxPriceGapBPS
			}
			if d.PriceGapFreeBPS != nil {
				c.PriceGapFreeBPS = *d.PriceGapFreeBPS
			}
			if d.MaxGapRecoveryIntervals != nil {
				c.MaxGapRecoveryIntervals = *d.MaxGapRecoveryIntervals
			}
			if d.MaxIntervalHours != nil {
				c.MaxIntervalHours = *d.MaxIntervalHours
			}
			if d.AllowMixedIntervals != nil {
				c.AllowMixedIntervals = *d.AllowMixedIntervals
			}
			if d.DelistFilter != nil {
				c.DelistFilterEnabled = *d.DelistFilter
			}
			if p := d.Persistence; p != nil {
				if p.LookbackMin1h != nil {
					c.PersistLookback1h = time.Duration(*p.LookbackMin1h) * time.Minute
				}
				if p.MinCount1h != nil {
					c.PersistMinCount1h = *p.MinCount1h
				}
				if p.LookbackMin4h != nil {
					c.PersistLookback4h = time.Duration(*p.LookbackMin4h) * time.Minute
				}
				if p.MinCount4h != nil {
					c.PersistMinCount4h = *p.MinCount4h
				}
				if p.LookbackMin8h != nil {
					c.PersistLookback8h = time.Duration(*p.LookbackMin8h) * time.Minute
				}
				if p.MinCount8h != nil {
					c.PersistMinCount8h = *p.MinCount8h
				}
				if p.SpreadStabilityRatio1h != nil {
					c.SpreadStabilityRatio1h = *p.SpreadStabilityRatio1h
				}
				if p.SpreadStabilityOIRank1h != nil {
					c.SpreadStabilityOIRank1h = *p.SpreadStabilityOIRank1h
				}
				if p.SpreadStabilityRatio4h != nil {
					c.SpreadStabilityRatio4h = *p.SpreadStabilityRatio4h
				}
				if p.SpreadStabilityOIRank4h != nil {
					c.SpreadStabilityOIRank4h = *p.SpreadStabilityOIRank4h
				}
				if p.SpreadStabilityRatio8h != nil {
					c.SpreadStabilityRatio8h = *p.SpreadStabilityRatio8h
				}
				if p.SpreadStabilityOIRank8h != nil {
					c.SpreadStabilityOIRank8h = *p.SpreadStabilityOIRank8h
				}
				if p.SpreadVolatilityMaxCV != nil {
					c.SpreadVolatilityMaxCV = *p.SpreadVolatilityMaxCV
				}
				if p.SpreadVolatilityMinSamples != nil {
					c.SpreadVolatilityMinSamples = *p.SpreadVolatilityMinSamples
				}
				if p.FundingWindowMin != nil {
					c.FundingWindowMin = *p.FundingWindowMin
				}
			}
		}

		// Entry
		if e := s.Entry; e != nil {
			if e.SlippageLimitBPS != nil {
				c.SlippageBPS = *e.SlippageLimitBPS
			}
			if e.MinChunkUSDT != nil {
				c.MinChunkUSDT = *e.MinChunkUSDT
			}
			if e.EntryTimeoutSec != nil {
				c.EntryTimeoutSec = *e.EntryTimeoutSec
			}
			if e.LossCooldownHours != nil {
				c.LossCooldownHours = *e.LossCooldownHours
			}
			if e.ReEnterCooldownHours != nil {
				c.ReEnterCooldownHours = *e.ReEnterCooldownHours
			}
			if e.BacktestDays != nil {
				c.BacktestDays = *e.BacktestDays
			}
			if e.BacktestMinProfit != nil {
				c.BacktestMinProfit = *e.BacktestMinProfit
			}
		}

		// Exit
		if x := s.Exit; x != nil {
			if x.DepthTimeoutSec != nil {
				c.ExitDepthTimeoutSec = *x.DepthTimeoutSec
			}
			if x.EnableSpreadReversal != nil {
				c.EnableSpreadReversal = *x.EnableSpreadReversal
			}
			if x.SpreadReversalTolerance != nil {
				c.SpreadReversalTolerance = *x.SpreadReversalTolerance
			}
			if x.ZeroSpreadTolerance != nil {
				c.ZeroSpreadTolerance = *x.ZeroSpreadTolerance
			}
			if x.MaxGapBPS != nil {
				c.ExitMaxGapBPS = *x.MaxGapBPS
			}
		}

		// Rotation
		if r := s.Rotation; r != nil {
			if r.ThresholdBPS != nil {
				c.RotationThresholdBPS = *r.ThresholdBPS
			}
			if r.CooldownMin != nil {
				c.RotationCooldownMin = *r.CooldownMin
			}
		}
	}

	// Fund
	if f := jc.Fund; f != nil {
		if f.MaxPositions != nil {
			c.MaxPositions = *f.MaxPositions
		}
		if f.Leverage != nil {
			c.Leverage = *f.Leverage
		}
		if f.CapitalPerLeg != nil {
			c.CapitalPerLeg = *f.CapitalPerLeg
		}
		if f.RebalanceScanMinute != nil {
			c.RebalanceScanMinute = *f.RebalanceScanMinute
		} else if f.RebalanceAdvanceMin != nil {
			// Deprecated: accept old field name as fallback.
			c.RebalanceScanMinute = *f.RebalanceAdvanceMin
		}
	}

	// Risk
	if rk := jc.Risk; rk != nil {
		if rk.MarginL3Threshold != nil {
			c.MarginL3Threshold = *rk.MarginL3Threshold
		}
		if rk.MarginL4Threshold != nil {
			c.MarginL4Threshold = *rk.MarginL4Threshold
		}
		if rk.MarginL5Threshold != nil {
			c.MarginL5Threshold = *rk.MarginL5Threshold
		}
		if rk.L4ReduceFraction != nil {
			c.L4ReduceFraction = *rk.L4ReduceFraction
		}
		if rk.MarginSafetyMultiplier != nil {
			c.MarginSafetyMultiplier = *rk.MarginSafetyMultiplier
		}
		if rk.RiskMonitorIntervalSec != nil {
			c.RiskMonitorIntervalSec = *rk.RiskMonitorIntervalSec
		}
	}

	// Exchanges
	if ex, ok := jc.Exchanges["binance"]; ok {
		c.BinanceAPIKey = ex.APIKey
		c.BinanceSecretKey = ex.SecretKey
		c.BinanceEnabled = ex.Enabled
	}
	if ex, ok := jc.Exchanges["bybit"]; ok {
		c.BybitAPIKey = ex.APIKey
		c.BybitSecretKey = ex.SecretKey
		c.BybitEnabled = ex.Enabled
	}
	if ex, ok := jc.Exchanges["gateio"]; ok {
		c.GateioAPIKey = ex.APIKey
		c.GateioSecretKey = ex.SecretKey
		c.GateioEnabled = ex.Enabled
	}
	if ex, ok := jc.Exchanges["bitget"]; ok {
		c.BitgetAPIKey = ex.APIKey
		c.BitgetSecretKey = ex.SecretKey
		c.BitgetPassphrase = ex.Passphrase
		c.BitgetEnabled = ex.Enabled
	}
	if ex, ok := jc.Exchanges["okx"]; ok {
		c.OKXAPIKey = ex.APIKey
		c.OKXSecretKey = ex.SecretKey
		c.OKXPassphrase = ex.Passphrase
		c.OKXEnabled = ex.Enabled
	}
	if ex, ok := jc.Exchanges["bingx"]; ok {
		c.BingXAPIKey = ex.APIKey
		c.BingXSecretKey = ex.SecretKey
		c.BingXEnabled = ex.Enabled
	}

	// Parse deposit addresses
	c.ExchangeAddresses = make(map[string]map[string]string)
	for name, ex := range jc.Exchanges {
		if len(ex.Address) == 0 {
			continue
		}
		addrs := make(map[string]string)
		for chain, addr := range ex.Address {
			normalized := normalizeChain(chain)
			addrs[normalized] = addr
		}
		c.ExchangeAddresses[name] = addrs
	}

	// Redis
	if r := jc.Redis; r != nil {
		if r.Addr != "" {
			c.RedisAddr = r.Addr
		}
		if r.Password != "" {
			c.RedisPass = r.Password
		}
		if r.DB != nil {
			c.RedisDB = *r.DB
		}
	}

	// Dashboard
	if d := jc.Dashboard; d != nil {
		if d.Addr != "" {
			c.DashboardAddr = d.Addr
		}
		if d.Password != "" {
			c.DashboardPassword = d.Password
		}
	}

	// AI Diagnose
	if a := jc.AI; a != nil {
		if a.Endpoint != "" {
			c.AIEndpoint = a.Endpoint
		}
		if a.APIKey != "" {
			c.AIAPIKey = a.APIKey
		}
		if a.Model != "" {
			c.AIModel = a.Model
		}
		if a.MaxTokens != nil {
			c.AIMaxTokens = *a.MaxTokens
		}
	}

	// Spot-futures arbitrage scraper
	if sa := jc.SpotArb; sa != nil {
		if sa.Enabled != nil {
			c.SpotArbEnabled = *sa.Enabled
		}
		if sa.Schedule != "" {
			c.SpotArbSchedule = sa.Schedule
		}
		if sa.ChromePath != "" {
			c.SpotArbChromePath = sa.ChromePath
		}
	}

	// Spot-futures arbitrage engine
	if sf := jc.SpotFutures; sf != nil {
		if sf.Enabled != nil {
			c.SpotFuturesEnabled = *sf.Enabled
		}
		if sf.MaxPositions != nil {
			c.SpotFuturesMaxPositions = *sf.MaxPositions
		}
		if sf.CapitalPerPosition != nil {
			c.SpotFuturesCapitalPerPosition = *sf.CapitalPerPosition
		}
		if sf.Leverage != nil {
			c.SpotFuturesLeverage = *sf.Leverage
		}
		if sf.MonitorIntervalSec != nil {
			c.SpotFuturesMonitorIntervalSec = *sf.MonitorIntervalSec
		}
		if sf.MinNetYieldAPR != nil {
			c.SpotFuturesMinNetYieldAPR = *sf.MinNetYieldAPR
		}
		if sf.MaxBorrowAPR != nil {
			c.SpotFuturesMaxBorrowAPR = *sf.MaxBorrowAPR
		}
		if len(sf.Exchanges) > 0 {
			c.SpotFuturesExchanges = sf.Exchanges
		}
		if sf.ScanIntervalMin != nil {
			c.SpotFuturesScanIntervalMin = *sf.ScanIntervalMin
		}
	}
}

// SaveJSON writes the current runtime config back to config.json,
// preserving credentials and structure from the original file.
func (c *Config) SaveJSON() error {
	// Find the config file path
	paths := []string{"config.json", "/var/solana/data/arb/config.json"}
	if p := os.Getenv("CONFIG_FILE"); p != "" {
		paths = []string{p}
	}
	var filePath string
	var raw map[string]interface{}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err == nil {
			filePath = path
			json.Unmarshal(data, &raw)
			break
		}
	}
	if filePath == "" {
		return fmt.Errorf("config.json not found")
	}
	if raw == nil {
		raw = make(map[string]interface{})
	}

	// Helper to get or create nested maps
	getMap := func(parent map[string]interface{}, key string) map[string]interface{} {
		if v, ok := parent[key]; ok {
			if m, ok := v.(map[string]interface{}); ok {
				return m
			}
		}
		m := make(map[string]interface{})
		parent[key] = m
		return m
	}

	raw["dry_run"] = c.DryRun

	strategy := getMap(raw, "strategy")
	strategy["top_opportunities"] = c.TopOpportunities
	strategy["scan_minutes"] = c.ScanMinutes
	strategy["entry_scan_minute"] = c.EntryScanMinute
	strategy["exit_scan_minute"] = c.ExitScanMinute
	strategy["rotate_scan_minute"] = c.RotateScanMinute
	strategy["rebalance_scan_minute"] = c.RebalanceScanMinute

	disc := getMap(strategy, "discovery")
	disc["min_hold_time_hours"] = int(c.MinHoldTime.Hours())
	disc["max_cost_ratio"] = c.MaxCostRatio
	disc["max_price_gap_bps"] = c.MaxPriceGapBPS
	disc["price_gap_free_bps"] = c.PriceGapFreeBPS
	disc["max_gap_recovery_intervals"] = c.MaxGapRecoveryIntervals
	if c.MaxIntervalHours > 0 {
		disc["max_interval_hours"] = c.MaxIntervalHours
	}
	disc["allow_mixed_intervals"] = c.AllowMixedIntervals
	disc["delist_filter"] = c.DelistFilterEnabled

	persist := getMap(disc, "persistence")
	persist["lookback_min_1h"] = int(c.PersistLookback1h.Minutes())
	persist["min_count_1h"] = c.PersistMinCount1h
	persist["lookback_min_4h"] = int(c.PersistLookback4h.Minutes())
	persist["min_count_4h"] = c.PersistMinCount4h
	persist["lookback_min_8h"] = int(c.PersistLookback8h.Minutes())
	persist["min_count_8h"] = c.PersistMinCount8h
	persist["spread_stability_ratio_1h"] = c.SpreadStabilityRatio1h
	persist["spread_stability_oi_rank_1h"] = c.SpreadStabilityOIRank1h
	persist["spread_stability_ratio_4h"] = c.SpreadStabilityRatio4h
	persist["spread_stability_oi_rank_4h"] = c.SpreadStabilityOIRank4h
	persist["spread_stability_ratio_8h"] = c.SpreadStabilityRatio8h
	persist["spread_stability_oi_rank_8h"] = c.SpreadStabilityOIRank8h
	persist["spread_volatility_max_cv"] = c.SpreadVolatilityMaxCV
	persist["spread_volatility_min_samples"] = c.SpreadVolatilityMinSamples
	persist["funding_window_min"] = c.FundingWindowMin

	entry := getMap(strategy, "entry")
	entry["slippage_limit_bps"] = c.SlippageBPS
	entry["min_chunk_usdt"] = c.MinChunkUSDT
	entry["entry_timeout_sec"] = c.EntryTimeoutSec
	entry["loss_cooldown_hours"] = c.LossCooldownHours
	entry["re_enter_cooldown_hours"] = c.ReEnterCooldownHours
	entry["backtest_days"] = c.BacktestDays
	entry["backtest_min_profit"] = c.BacktestMinProfit

	exit := getMap(strategy, "exit")
	exit["depth_timeout_sec"] = c.ExitDepthTimeoutSec
	exit["enable_spread_reversal"] = c.EnableSpreadReversal
	exit["spread_reversal_tolerance"] = c.SpreadReversalTolerance
	exit["zero_spread_tolerance"] = c.ZeroSpreadTolerance
	exit["max_gap_bps"] = c.ExitMaxGapBPS

	rot := getMap(strategy, "rotation")
	rot["threshold_bps"] = c.RotationThresholdBPS
	rot["cooldown_min"] = c.RotationCooldownMin

	fund := getMap(raw, "fund")
	fund["max_positions"] = c.MaxPositions
	fund["leverage"] = c.Leverage
	fund["capital_per_leg"] = c.CapitalPerLeg

	risk := getMap(raw, "risk")
	risk["margin_l3_threshold"] = c.MarginL3Threshold
	risk["margin_l4_threshold"] = c.MarginL4Threshold
	risk["margin_l5_threshold"] = c.MarginL5Threshold
	risk["l4_reduce_fraction"] = c.L4ReduceFraction
	risk["margin_safety_multiplier"] = c.MarginSafetyMultiplier
	risk["risk_monitor_interval_sec"] = c.RiskMonitorIntervalSec

	ai := getMap(raw, "ai")
	if c.AIEndpoint != "" {
		ai["endpoint"] = c.AIEndpoint
	}
	if c.AIAPIKey != "" {
		ai["api_key"] = c.AIAPIKey
	}
	if c.AIModel != "" {
		ai["model"] = c.AIModel
	}
	ai["max_tokens"] = c.AIMaxTokens

	// Exchanges — build the full exchanges map from current runtime config
	exchanges := getMap(raw, "exchanges")
	type exchDef struct {
		name       string
		apiKey     string
		secretKey  string
		passphrase string
		hasPass    bool
	}
	type exchDef2 struct {
		exchDef
		enabled *bool
	}
	exchDefs := []exchDef2{
		{exchDef{"binance", c.BinanceAPIKey, c.BinanceSecretKey, "", false}, c.BinanceEnabled},
		{exchDef{"bybit", c.BybitAPIKey, c.BybitSecretKey, "", false}, c.BybitEnabled},
		{exchDef{"gateio", c.GateioAPIKey, c.GateioSecretKey, "", false}, c.GateioEnabled},
		{exchDef{"bitget", c.BitgetAPIKey, c.BitgetSecretKey, c.BitgetPassphrase, true}, c.BitgetEnabled},
		{exchDef{"okx", c.OKXAPIKey, c.OKXSecretKey, c.OKXPassphrase, true}, c.OKXEnabled},
		{exchDef{"bingx", c.BingXAPIKey, c.BingXSecretKey, "", false}, c.BingXEnabled},
	}
	for _, ed := range exchDefs {
		exMap := getMap(exchanges, ed.name)
		exMap["api_key"] = ed.apiKey
		exMap["secret_key"] = ed.secretKey
		if ed.hasPass {
			exMap["passphrase"] = ed.passphrase
		}
		if ed.enabled != nil {
			exMap["enabled"] = *ed.enabled
		}
		if addrs, ok := c.ExchangeAddresses[ed.name]; ok && len(addrs) > 0 {
			exMap["address"] = addrs
		}
	}

	out, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	out = append(out, '\n')
	return os.WriteFile(filePath, out, 0644)
}

func (c *Config) loadEnvOverrides() {
	if v := os.Getenv("VALUE_OF_TIME_HOURS"); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			c.MinHoldTime = time.Duration(i) * time.Hour
		}
	}
	if v := os.Getenv("VALUE_OF_RATIO"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			c.MaxCostRatio = f
		}
	}
	if v := os.Getenv("MAX_POSITIONS"); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			c.MaxPositions = i
		}
	}
	if v := os.Getenv("LEVERAGE"); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			c.Leverage = i
		}
	}
	if v := os.Getenv("SLIPPAGE_LIMIT_BPS"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			c.SlippageBPS = f
		}
	}
	if v := os.Getenv("REBALANCE_SCAN_MINUTE"); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			c.RebalanceScanMinute = i
		}
	} else if v := os.Getenv("REBALANCE_ADVANCE_MIN"); v != "" {
		// Deprecated: accept old env var as fallback.
		if i, err := strconv.Atoi(v); err == nil {
			c.RebalanceScanMinute = i
		}
	}
	if v := os.Getenv("TOP_OPPORTUNITIES"); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			c.TopOpportunities = i
		}
	}
	if v := os.Getenv("CAPITAL_PER_LEG"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			c.CapitalPerLeg = f
		}
	}
	if v := os.Getenv("PRICE_GAP_FREE_BPS"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			c.PriceGapFreeBPS = f
		}
	}
	if v := os.Getenv("MAX_PRICE_GAP_BPS"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			c.MaxPriceGapBPS = f
		}
	}
	if v := os.Getenv("MAX_GAP_RECOVERY_INTERVALS"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			c.MaxGapRecoveryIntervals = f
		}
	}
	if v := os.Getenv("DRY_RUN"); v != "" {
		c.DryRun = v == "1" || v == "true" || v == "yes"
	}
	if v := os.Getenv("ENTRY_TIMEOUT_SEC"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.EntryTimeoutSec = n
		}
	}
	if v := os.Getenv("MIN_CHUNK_USDT"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			c.MinChunkUSDT = f
		}
	}

	// Env vars override JSON for API keys
	if v := os.Getenv("BINANCE_API_KEY"); v != "" {
		c.BinanceAPIKey = v
	}
	if v := os.Getenv("BINANCE_SECRET_KEY"); v != "" {
		c.BinanceSecretKey = v
	}
	if v := os.Getenv("BYBIT_API_KEY"); v != "" {
		c.BybitAPIKey = v
	}
	if v := os.Getenv("BYBIT_SECRET_KEY"); v != "" {
		c.BybitSecretKey = v
	}
	if v := os.Getenv("GATEIO_API_KEY"); v != "" {
		c.GateioAPIKey = v
	}
	if v := os.Getenv("GATEIO_SECRET_KEY"); v != "" {
		c.GateioSecretKey = v
	}
	if v := os.Getenv("BITGET_API_KEY"); v != "" {
		c.BitgetAPIKey = v
	}
	if v := os.Getenv("BITGET_SECRET_KEY"); v != "" {
		c.BitgetSecretKey = v
	}
	if v := os.Getenv("BITGET_PASSPHRASE"); v != "" {
		c.BitgetPassphrase = v
	}
	if v := os.Getenv("OKX_API_KEY"); v != "" {
		c.OKXAPIKey = v
	}
	if v := os.Getenv("OKX_SECRET_KEY"); v != "" {
		c.OKXSecretKey = v
	}
	if v := os.Getenv("OKX_PASSPHRASE"); v != "" {
		c.OKXPassphrase = v
	}
	if v := os.Getenv("BINGX_API_KEY"); v != "" {
		c.BingXAPIKey = v
	}
	if v := os.Getenv("BINGX_SECRET_KEY"); v != "" {
		c.BingXSecretKey = v
	}
	if v := os.Getenv("REDIS_ADDR"); v != "" {
		c.RedisAddr = v
	}
	if v := os.Getenv("REDIS_PASS"); v != "" {
		c.RedisPass = v
	}
	if v := os.Getenv("REDIS_DB"); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			c.RedisDB = i
		}
	}
	if v := os.Getenv("DASHBOARD_ADDR"); v != "" {
		c.DashboardAddr = v
	}
	if v := os.Getenv("DASHBOARD_PASSWORD"); v != "" {
		c.DashboardPassword = v
	}

	// Spot-futures arbitrage scraper
	if v := os.Getenv("SPOT_ARB_ENABLED"); v != "" {
		c.SpotArbEnabled = v == "1" || v == "true" || v == "yes"
	}
	if v := os.Getenv("SPOT_ARB_SCHEDULE"); v != "" {
		c.SpotArbSchedule = v
	}
	if v := os.Getenv("SPOT_ARB_CHROME_PATH"); v != "" {
		c.SpotArbChromePath = v
	}

	// Spot-futures arbitrage engine
	if v := os.Getenv("SPOT_FUTURES_ENABLED"); v != "" {
		c.SpotFuturesEnabled = v == "1" || v == "true" || v == "yes"
	}
	if v := os.Getenv("SPOT_FUTURES_MAX_POSITIONS"); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			c.SpotFuturesMaxPositions = i
		}
	}
	if v := os.Getenv("SPOT_FUTURES_CAPITAL_PER_POSITION"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			c.SpotFuturesCapitalPerPosition = f
		}
	}
	if v := os.Getenv("SPOT_FUTURES_LEVERAGE"); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			c.SpotFuturesLeverage = i
		}
	}
	if v := os.Getenv("SPOT_FUTURES_MONITOR_INTERVAL"); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			c.SpotFuturesMonitorIntervalSec = i
		}
	}
}

// IsExchangeEnabled returns true if the exchange has an API key and is not
// explicitly disabled. A nil Enabled pointer means "auto" (enabled if key set).
func (c *Config) IsExchangeEnabled(name string) bool {
	var key string
	var flag *bool
	switch name {
	case "binance":
		key, flag = c.BinanceAPIKey, c.BinanceEnabled
	case "bybit":
		key, flag = c.BybitAPIKey, c.BybitEnabled
	case "gateio":
		key, flag = c.GateioAPIKey, c.GateioEnabled
	case "bitget":
		key, flag = c.BitgetAPIKey, c.BitgetEnabled
	case "okx":
		key, flag = c.OKXAPIKey, c.OKXEnabled
	case "bingx":
		key, flag = c.BingXAPIKey, c.BingXEnabled
	default:
		return false
	}
	if key == "" {
		return false // no API key — can't enable
	}
	if flag != nil {
		return *flag
	}
	return true // auto: enabled when key exists
}

// EnabledExchanges returns a list of exchange names that are enabled.
func (c *Config) EnabledExchanges() []string {
	var enabled []string
	for _, name := range []string{"binance", "bybit", "gateio", "bitget", "okx", "bingx"} {
		if c.IsExchangeEnabled(name) {
			enabled = append(enabled, name)
		}
	}
	return enabled
}
