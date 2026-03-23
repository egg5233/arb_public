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
	DryRun              bool          // if true, skip trade execution (log only)

	// Depth-driven entry execution
	EntryTimeoutSec int     // max seconds for depth fill loop (default 60)
	MinChunkUSDT    float64 // min notional per micro-order to avoid dust (default 5)

	// Margin health thresholds (marginRatio values, 0-1 scale where 1.0 = liquidation)
	MarginL3Threshold float64 // trigger fund transfer (default: 0.50)
	MarginL4Threshold float64 // trigger position reduction (default: 0.80)
	MarginL5Threshold float64 // trigger emergency close (default: 0.95)
	L4ReduceFraction  float64 // fraction to reduce at L4 (default: 0.50)

	// Exit strategy
	ExitDepthTimeoutSec  int     // depth-fill exit loop timeout before market fallback (default 45)

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
	SpreadReversalTolerance int // allow N reversals before triggering exit (default 0 = exit on first)

	// Anti-spike filters
	SpreadVolatilityMaxCV      float64 // max stddev/mean for spread across scans (default 0.5, 0=disabled)
	SpreadVolatilityMinSamples int     // min scan records to evaluate CV (default 3)
	FundingWindowMin           int     // max minutes before funding to allow entry (default 30)
	LossCooldownHours          float64 // hours to blacklist symbol after loss close (default 4.0)
	ReEnterCooldownHours       float64 // hours to block re-entry on same symbol after any close (default 0, disabled)

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
	BybitAPIKey      string
	BybitSecretKey   string
	GateioAPIKey     string
	GateioSecretKey  string
	BitgetAPIKey     string
	BitgetSecretKey  string
	BitgetPassphrase string
	OKXAPIKey        string
	OKXSecretKey     string
	OKXPassphrase    string
	BingXAPIKey      string
	BingXSecretKey   string

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
}

type jsonExchange struct {
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
}

type jsonExit struct {
	DepthTimeoutSec         *int     `json:"depth_timeout_sec"`
	SpreadReversalTolerance *int     `json:"spread_reversal_tolerance"`
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
	L4ReduceFraction       *float64 `json:"l4_reduce_fraction"`
	RiskMonitorIntervalSec *int     `json:"risk_monitor_interval_sec"`
}

// Load reads configuration from config.json (if present), with env var overrides.
// Priority: env var > config.json > default value.
func Load() *Config {
	c := &Config{
		MinHoldTime:             16 * time.Hour,
		MaxCostRatio:            0.50,
		MaxPositions:            1,
		Leverage:                3,
		SlippageBPS:             5,
		RebalanceScanMinute:     20,
		TopOpportunities:        5,
		PriceGapFreeBPS:         40,
		MaxPriceGapBPS:          250,
		MaxGapRecoveryIntervals: 1.0,
		EntryTimeoutSec:         60,
		MinChunkUSDT:            5,
		MarginL3Threshold:       0.50,
		MarginL4Threshold:       0.80,
		MarginL5Threshold:       0.95,
		L4ReduceFraction:        0.50,
		ExitDepthTimeoutSec:     45,
		RiskMonitorIntervalSec:  300,
		PersistLookback1h:       15 * time.Minute,
		PersistMinCount1h:       2,
		PersistLookback4h:       30 * time.Minute,
		PersistMinCount4h:       4,
		PersistLookback8h:       40 * time.Minute,
		PersistMinCount8h:       5,
		SpreadStabilityRatio1h:  0.5,
		SpreadStabilityOIRank1h: 100,
		SpreadVolatilityMaxCV:      0.5,
		SpreadVolatilityMinSamples: 3,
		FundingWindowMin:           30,
		LossCooldownHours:          4.0,
		ScanMinutes:             []int{5, 15, 25, 35, 45, 55},
		EntryScanMinute:         35,
		ExitScanMinute:          25,
		RotateScanMinute:        45,
		RotationThresholdBPS:    20,
		RotationCooldownMin:     30,
		RedisAddr:               "localhost:6379",
		RedisDB:                 2,
		DashboardAddr:           ":8080",
		AIModel:                 "gpt-5.4",
		AIMaxTokens:             4096,
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
		}

		// Exit
		if x := s.Exit; x != nil {
			if x.DepthTimeoutSec != nil {
				c.ExitDepthTimeoutSec = *x.DepthTimeoutSec
			}
			if x.SpreadReversalTolerance != nil {
				c.SpreadReversalTolerance = *x.SpreadReversalTolerance
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
		if rk.RiskMonitorIntervalSec != nil {
			c.RiskMonitorIntervalSec = *rk.RiskMonitorIntervalSec
		}
	}

	// Exchanges
	if ex, ok := jc.Exchanges["binance"]; ok {
		c.BinanceAPIKey = ex.APIKey
		c.BinanceSecretKey = ex.SecretKey
	}
	if ex, ok := jc.Exchanges["bybit"]; ok {
		c.BybitAPIKey = ex.APIKey
		c.BybitSecretKey = ex.SecretKey
	}
	if ex, ok := jc.Exchanges["gateio"]; ok {
		c.GateioAPIKey = ex.APIKey
		c.GateioSecretKey = ex.SecretKey
	}
	if ex, ok := jc.Exchanges["bitget"]; ok {
		c.BitgetAPIKey = ex.APIKey
		c.BitgetSecretKey = ex.SecretKey
		c.BitgetPassphrase = ex.Passphrase
	}
	if ex, ok := jc.Exchanges["okx"]; ok {
		c.OKXAPIKey = ex.APIKey
		c.OKXSecretKey = ex.SecretKey
		c.OKXPassphrase = ex.Passphrase
	}
	if ex, ok := jc.Exchanges["bingx"]; ok {
		c.BingXAPIKey = ex.APIKey
		c.BingXSecretKey = ex.SecretKey
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

	exit := getMap(strategy, "exit")
	exit["depth_timeout_sec"] = c.ExitDepthTimeoutSec
	exit["spread_reversal_tolerance"] = c.SpreadReversalTolerance

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
}

// EnabledExchanges returns a list of exchange names that have API keys configured.
func (c *Config) EnabledExchanges() []string {
	var enabled []string
	if c.BinanceAPIKey != "" {
		enabled = append(enabled, "binance")
	}
	if c.BybitAPIKey != "" {
		enabled = append(enabled, "bybit")
	}
	if c.GateioAPIKey != "" {
		enabled = append(enabled, "gateio")
	}
	if c.BitgetAPIKey != "" {
		enabled = append(enabled, "bitget")
	}
	if c.OKXAPIKey != "" {
		enabled = append(enabled, "okx")
	}
	if c.BingXAPIKey != "" {
		enabled = append(enabled, "bingx")
	}
	return enabled
}
