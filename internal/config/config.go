package config

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
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
	mu sync.RWMutex // protects concurrent read/write of config fields

	// Strategy parameters
	MinHoldTime             time.Duration // timed exit hold duration (default 16h)
	MaxCostRatio            float64       // max fees/revenue ratio for profitability filter (default 0.50)
	MaxPositions            int           // max concurrent arb positions (default 1)
	Leverage                int           // default leverage (default 3)
	SlippageBPS             float64       // max acceptable orderbook slippage in bps (default 5)
	RebalanceScanMinute     int           // minute mark that triggers rebalance (default 10)
	TopOpportunities        int           // max opportunities to return from scanner (default 5)
	CapitalPerLeg           float64       // fixed USDT per leg; 0 = auto from balance (default 0)
	PriceGapFreeBPS         float64       // below this gap, no check (default 40)
	MaxPriceGapBPS          float64       // hard reject above this gap (default 250)
	MaxGapRecoveryIntervals float64       // max funding intervals to recover gap (default 1.0)
	MaxIntervalHours        float64       // max funding interval hours to accept (0=disabled, e.g. 1=only 1h)
	AllowMixedIntervals     bool          // allow cross-interval pairs in ranker (default false)
	DryRun                  bool          // if true, skip trade execution (log only)
	TradFiSigned            bool          // true after Binance TradFi-Perps agreement is signed

	// Depth-driven entry execution
	EntryTimeoutSec int     // max seconds for depth fill loop (default 60)
	MinChunkUSDT    float64 // min notional per micro-order to avoid dust (default 5)

	// Margin health thresholds (marginRatio values, 0-1 scale where 1.0 = liquidation)
	MarginL3Threshold      float64 // trigger fund transfer (default: 0.50)
	MarginL4Threshold      float64 // trigger position reduction (default: 0.80)
	MarginL5Threshold      float64 // trigger emergency close (default: 0.95)
	L4ReduceFraction       float64 // fraction to reduce at L4 (default: 0.50)
	MarginSafetyMultiplier float64 // margin buffer multiplier for entry check (default: 2.0)
	EntryMarginHeadroom    float64 // fraction of L3 threshold used as entry limit (default: 0.80, i.e. L3×0.80)

	// Exit strategy
	ExitDepthTimeoutSec int     // depth-fill exit loop timeout before market fallback (default 300)
	ExitMaxGapBPS       float64 // max cross-exchange gap at exit; ramps to 3x over timeout (default 10)

	// Risk monitor (log-only)
	RiskMonitorIntervalSec int // interval between risk monitor checks in seconds (default: 300)
	EnableLiqTrendTracking bool
	LiqProjectionMinutes   int
	LiqWarningSlopeThresh  float64
	LiqCriticalSlopeThresh float64
	LiqMinSamples          int

	// Cross-strategy capital allocator (default disabled for production safety)
	EnableCapitalAllocator      bool
	MaxTotalExposureUSDT        float64
	MaxPerpPerpPct              float64
	MaxSpotFuturesPct           float64
	MaxPerExchangePct           float64
	ReservationTTLSec           int
	EnableExchangeHealthScoring bool
	ExchHealthLatencyMs         int
	ExchHealthMinUptime         float64
	ExchHealthMinFillRate       float64
	ExchHealthMinScore          float64
	ExchHealthWindowMin         int

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
	ZeroSpreadTolerance     int  // exit after N consecutive checks where both legs have equal funding rate (0=disabled)

	// Anti-spike filters
	EnableSpreadStabilityGate       bool    // enable spread stability pre-trade gate (default false)
	SpreadVolatilityMaxCV           float64 // max stddev/mean for spread across scans (default 0.5, 0=disabled)
	SpreadVolatilityMinSamples      int     // min scan records to evaluate CV (default 3)
	SpreadStabilityStricterForAuto  bool    // apply tighter spread CV threshold for automated entries (default true)
	SpreadStabilityAutoCVMultiplier float64 // multiplier applied to CV threshold for automated entries (default 0.7)
	FundingWindowMin                int     // max minutes before funding to allow entry (default 30)
	LossCooldownHours               float64 // hours to blacklist symbol after loss close (default 4.0)
	ReEnterCooldownHours            float64 // hours to block re-entry on same symbol after any close (default 0, disabled)
	BacktestDays                    int     // days of historical funding to check (default 3, 0 = disabled)
	BacktestMinProfit               float64 // minimum net profit to pass backtest filter (default 0)
	DelistFilterEnabled             bool    // enable Binance delist monitoring & filtering (default true)

	// Scan schedule
	ScanMinutes      []int // minutes within each hour when scans fire (default [10,20,30,35,40,45,50])
	EntryScanMinute  int   // minute mark that triggers trade execution (default 40)
	ExitScanMinute   int   // minute mark that triggers exit checks (default 30)
	RotateScanMinute int   // minute mark that triggers rotation checks (default 35)

	// Rebalance scheduling
	EnablePoolAllocator    bool
	TopPairsPerSymbol      int
	AllocatorTimeoutMs int

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
	SpotFuturesEnabled            bool
	SpotFuturesMaxPositions       int
	SpotFuturesLeverage           int
	SpotFuturesMonitorIntervalSec int
	SpotFuturesMinNetYieldAPR     float64  // minimum net APR after costs (decimal, e.g. 0.10 = 10%)
	SpotFuturesMaxBorrowAPR       float64  // maximum borrow APR for Direction A (decimal, e.g. 0.50 = 50%)
	EnableBorrowSpikeDetection    bool     // if true, auto-exit on rapid borrow APR spikes (default: false)
	BorrowSpikeWindowMin          int      // trailing window size for borrow spike detection (default: 60)
	BorrowSpikeMultiplier         float64  // multiplier threshold for borrow spike detection (default: 2.0)
	BorrowSpikeMinAbsolute        float64  // minimum absolute APR increase required to count as a spike (default: 0.10)
	SpotFuturesExchanges          []string // exchanges to consider (empty = all SpotMargin-capable)
	SpotFuturesScanIntervalMin    int      // discovery scan interval in minutes
	SpotFuturesBorrowGraceMin     int      // minutes of negative yield before exit (default: 30)
	SpotFuturesPriceExitPct       float64  // price move % to trigger normal exit (default: 20.0)
	SpotFuturesPriceEmergencyPct  float64  // price move % to trigger emergency exit (default: 30.0)
	SpotFuturesMarginExitPct      float64  // margin utilization % for normal exit (default: 85.0)
	SpotFuturesMarginEmergencyPct float64  // margin utilization % for emergency exit (default: 95.0)
	SpotFuturesLossCooldownHours  int      // hours to cool down after a losing trade (default: 4)
	SpotFuturesAutoEnabled        bool     // enable automated spot-futures entry from discovery loop (default: false)
	SpotFuturesDryRun             bool     // if true, log auto-entry decisions but skip execution (default: true)
	SpotFuturesPersistenceScans   int      // consecutive scans a symbol must appear before auto-entry (default: 2)

	// Separate-account exchange support (Binance/Bitget)
	SpotFuturesProfitTransferEnabled bool    // enable auto profit-to-margin transfers after exit (default: false)
	SpotFuturesCapitalSeparate       float64 // capital per position for separate-account exchanges (default: 200)
	SpotFuturesCapitalUnified        float64 // capital per position for unified-account exchanges (default: 500)

	// Phase 2: native scanner + exit/entry guards
	SpotFuturesScannerMode           string  // scanner data source: "native", "coinglass", or "both" (default: "native")
	SpotFuturesEnableMinHold         bool    // enable min-hold gate before yield-based exit (default: false)
	SpotFuturesMinHoldHours          int     // hours before yield-based exit allowed (default: 8)
	SpotFuturesEnableSettlementGuard bool    // skip exit eval during settlement window (default: false)
	SpotFuturesSettlementWindowMin   int     // minutes around settlement to guard (default: 10)
	SpotFuturesEnablePriceGapGate    bool    // reject entry when spot-vs-futures price gap is too wide (default: false)
	SpotFuturesMaxPriceGapPct        float64 // max spot-vs-futures price gap % for entry (default: 0.5)
	SpotFuturesEnableExitSpreadGate  bool    // gate exits by unwind slippage (default: false)
	SpotFuturesExitSpreadPct         float64 // max slippage % to allow exit (default: 0.3)

	// Phase 6: maintenance rate risk hardening
	SpotFuturesEnableMaintenanceGate bool    // enable maintenance_rate pre-entry and runtime liq distance checks (default: false)
	SpotFuturesMaintenanceDefault    float64 // conservative default when rate=0/unknown (default: 0.05 = 5%)
	SpotFuturesMaintenanceCacheTTL   int     // cache TTL in minutes for tiered rate data (default: 60)

	// Telegram notifications
	TelegramBotToken string
	TelegramChatID   string

	// ---------------------------------------------------------------------------
	// Unified capital allocation (Phase 5)
	// ---------------------------------------------------------------------------
	EnableUnifiedCapital   bool    // Master on/off for unified capital (default OFF per D-13)
	TotalCapitalUSDT       float64 // Total USDT pool; 0 = disabled, use CapitalPerLeg (per D-09)
	RiskProfile            string  // "conservative", "balanced", "aggressive", "custom" (per D-03)
	AllocationLookbackDays int     // Performance lookback window in days (default 7, per D-07)
	AllocationFloorPct     float64 // Min allocation per strategy (default 0.20, per D-06)
	AllocationCeilingPct   float64 // Max allocation per strategy (default 0.80, per D-06)
	SizeMultiplier         float64 // Profile-driven position size multiplier (default 1.0)

	// ---------------------------------------------------------------------------
	// Safety: perp-perp Telegram notifications (struct field only; remaining
	// touch points -- JSON, default, apply, toJSON, fromEnv -- added in Plan 03)
	// ---------------------------------------------------------------------------
	EnablePerpTelegram  bool    // On/off for perp-perp Telegram alerts (default OFF per D-10)
	EnableLossLimits    bool    // Master on/off for rolling loss limits (default OFF per D-10)
	DailyLossLimitUSDT  float64 // 24-hour rolling net loss threshold in USDT
	WeeklyLossLimitUSDT float64 // 7-day rolling net loss threshold in USDT
	TelegramCooldownSec int     // Per-event-type cooldown in seconds (default 300 = 5 min per D-03)

	// ---------------------------------------------------------------------------
	// Analytics: performance analytics with SQLite store and API endpoints
	// ---------------------------------------------------------------------------
	EnableAnalytics bool   // Master on/off for analytics (SQLite store, API endpoints). Default OFF.
	AnalyticsDBPath string // Path to SQLite analytics database file. Default "data/analytics.db".
}

// ---------- Nested JSON config structs ----------

type jsonConfig struct {
	DryRun      *bool                   `json:"dry_run"`
	TradFiSigned *bool                  `json:"tradfi_signed"`
	Exchanges   map[string]jsonExchange `json:"exchanges"`
	Redis       *jsonRedis              `json:"redis"`
	Dashboard   *jsonDashboard          `json:"dashboard"`
	Strategy    *jsonStrategy           `json:"strategy"`
	Fund        *jsonFund               `json:"fund"`
	Risk        *jsonRisk               `json:"risk"`
	AI          *jsonAI                 `json:"ai"`
	SpotArb     *jsonSpotArb            `json:"spot_arb"`
	SpotFutures *jsonSpotFutures        `json:"spot_futures"`
	Telegram    *jsonTelegram           `json:"telegram"`
	Safety      *jsonSafety             `json:"safety"`
	Analytics   *jsonAnalytics          `json:"analytics"`
	Allocation  *jsonAllocation        `json:"allocation"`
}

type jsonAnalytics struct {
	EnableAnalytics *bool   `json:"enable_analytics"`
	AnalyticsDBPath *string `json:"analytics_db_path,omitempty"`
}

type jsonAllocation struct {
	EnableUnifiedCapital   *bool    `json:"enable_unified_capital"`
	TotalCapitalUSDT       *float64 `json:"total_capital_usdt"`
	RiskProfile            *string  `json:"risk_profile"`
	AllocationLookbackDays *int     `json:"allocation_lookback_days"`
	AllocationFloorPct     *float64 `json:"allocation_floor_pct"`
	AllocationCeilingPct   *float64 `json:"allocation_ceiling_pct"`
	SizeMultiplier         *float64 `json:"size_multiplier"`
}

type jsonSafety struct {
	EnableLossLimits    *bool    `json:"enable_loss_limits"`
	DailyLossLimitUSDT  *float64 `json:"daily_loss_limit_usdt"`
	WeeklyLossLimitUSDT *float64 `json:"weekly_loss_limit_usdt"`
	EnablePerpTelegram  *bool    `json:"enable_perp_telegram"`
	TelegramCooldownSec *int     `json:"telegram_cooldown_sec"`
}

type jsonTelegram struct {
	BotToken string `json:"bot_token"`
	ChatID   string `json:"chat_id"`
}

type jsonSpotArb struct {
	Enabled    *bool  `json:"enabled"`
	Schedule   string `json:"schedule"`
	ChromePath string `json:"chrome_path"`
}

type jsonSpotFutures struct {
	Enabled                    *bool    `json:"enabled"`
	MaxPositions               *int     `json:"max_positions"`
	Leverage                   *int     `json:"leverage"`
	MonitorIntervalSec         *int     `json:"monitor_interval_sec"`
	MinNetYieldAPR             *float64 `json:"min_net_yield_apr"`
	MaxBorrowAPR               *float64 `json:"max_borrow_apr"`
	EnableBorrowSpikeDetection *bool    `json:"enable_borrow_spike_detection"`
	BorrowSpikeWindowMin       *int     `json:"borrow_spike_window_min"`
	BorrowSpikeMultiplier      *float64 `json:"borrow_spike_multiplier"`
	BorrowSpikeMinAbsolute     *float64 `json:"borrow_spike_min_absolute"`
	Exchanges                  []string `json:"exchanges"`
	ScanIntervalMin            *int     `json:"scan_interval_min"`
	BorrowGraceMin             *int     `json:"borrow_grace_min"`
	PriceExitPct               *float64 `json:"price_exit_pct"`
	PriceEmergencyPct          *float64 `json:"price_emergency_pct"`
	MarginExitPct              *float64 `json:"margin_exit_pct"`
	MarginEmergencyPct         *float64 `json:"margin_emergency_pct"`
	LossCooldownHours          *int     `json:"loss_cooldown_hours"`
	AutoEnabled                *bool    `json:"auto_enabled"`
	AutoDryRun                 *bool    `json:"auto_dry_run"`
	PersistenceScans           *int     `json:"persistence_scans"`

	ProfitTransferEnabled *bool    `json:"profit_transfer_enabled"`
	CapitalSeparateUSDT   *float64 `json:"capital_separate_usdt"`
	CapitalUnifiedUSDT    *float64 `json:"capital_unified_usdt"`

	// Phase 2: native scanner + exit/entry guards
	NativeScannerEnabled  *bool    `json:"native_scanner_enabled"`  // deprecated: backward compat
	ScannerMode           *string  `json:"scanner_mode"`
	EnableMinHold         *bool    `json:"enable_min_hold"`
	MinHoldHours          *int     `json:"min_hold_hours"`
	EnableSettlementGuard *bool    `json:"enable_settlement_guard"`
	SettlementWindowMin   *int     `json:"settlement_window_min"`
	EnablePriceGapGate    *bool    `json:"enable_price_gap_gate"`
	MaxPriceGapPct        *float64 `json:"max_price_gap_pct"`
	EnableExitSpreadGate  *bool    `json:"enable_exit_spread_gate"`
	ExitSpreadPct         *float64 `json:"exit_spread_pct"`

	// Phase 6: maintenance rate risk hardening
	EnableMaintenanceGate *bool    `json:"enable_maintenance_gate"`
	MaintenanceDefault    *float64 `json:"maintenance_default"`
	MaintenanceCacheTTL   *int     `json:"maintenance_cache_ttl"`
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
	TopOpportunities       *int           `json:"top_opportunities"`
	ScanMinutes            []int          `json:"scan_minutes"`
	EntryScanMinute        *int           `json:"entry_scan_minute"`
	ExitScanMinute         *int           `json:"exit_scan_minute"`
	RotateScanMinute       *int           `json:"rotate_scan_minute"`
	RebalanceScanMinute    *int           `json:"rebalance_scan_minute"`
	EnablePoolAllocator    *bool          `json:"enable_pool_allocator"`
	TopPairsPerSymbol      *int           `json:"top_pairs_per_symbol"`
	AllocatorTimeoutMs *int           `json:"allocator_timeout_ms"`
	Discovery          *jsonDiscovery `json:"discovery"`
	Entry                  *jsonEntry     `json:"entry"`
	Exit                   *jsonExit      `json:"exit"`
	Rotation               *jsonRotation  `json:"rotation"`
}

type jsonDiscovery struct {
	MinHoldTimeHours        *int             `json:"min_hold_time_hours"`
	MaxCostRatio            *float64         `json:"max_cost_ratio"`
	MaxPriceGapBPS          *float64         `json:"max_price_gap_bps"`
	PriceGapFreeBPS         *float64         `json:"price_gap_free_bps"`
	MaxGapRecoveryIntervals *float64         `json:"max_gap_recovery_intervals"`
	MaxIntervalHours        *float64         `json:"max_interval_hours"`
	AllowMixedIntervals     *bool            `json:"allow_mixed_intervals"`
	DelistFilter            *bool            `json:"delist_filter"`
	Persistence             *jsonPersistence `json:"persistence"`
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

	EnableSpreadStabilityGate       *bool    `json:"enable_spread_stability_gate"`
	SpreadVolatilityMaxCV           *float64 `json:"spread_volatility_max_cv"`
	SpreadVolatilityMinSamples      *int     `json:"spread_volatility_min_samples"`
	SpreadStabilityStricterForAuto  *bool    `json:"spread_stability_stricter_for_auto"`
	SpreadStabilityAutoCVMultiplier *float64 `json:"spread_stability_auto_cv_multiplier"`
	FundingWindowMin                *int     `json:"funding_window_min"`
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
	DepthTimeoutSec         *int     `json:"depth_timeout_sec"`
	EnableSpreadReversal    *bool    `json:"enable_spread_reversal"`
	SpreadReversalTolerance *int     `json:"spread_reversal_tolerance"`
	ZeroSpreadTolerance     *int     `json:"zero_spread_tolerance"`
	MaxGapBPS               *float64 `json:"max_gap_bps"`
}

type jsonRotation struct {
	ThresholdBPS *float64 `json:"threshold_bps"`
	CooldownMin  *int     `json:"cooldown_min"`
}

type jsonFund struct {
	MaxPositions        *int     `json:"max_positions"`
	Leverage            *int     `json:"leverage"`
	CapitalPerLeg       *float64 `json:"capital_per_leg"`
	RebalanceScanMinute *int     `json:"rebalance_scan_minute"` // legacy: also accepted here
}

type jsonRisk struct {
	MarginL3Threshold           *float64 `json:"margin_l3_threshold"`
	MarginL4Threshold           *float64 `json:"margin_l4_threshold"`
	MarginL5Threshold           *float64 `json:"margin_l5_threshold"`
	L4ReduceFraction            *float64 `json:"l4_reduce_fraction"`
	MarginSafetyMultiplier      *float64 `json:"margin_safety_multiplier"`
	EntryMarginHeadroom         *float64 `json:"entry_margin_headroom"`
	RiskMonitorIntervalSec      *int     `json:"risk_monitor_interval_sec"`
	EnableLiqTrendTracking      *bool    `json:"enable_liq_trend_tracking"`
	LiqProjectionMinutes        *int     `json:"liq_projection_minutes"`
	LiqWarningSlopeThresh       *float64 `json:"liq_warning_slope_thresh"`
	LiqCriticalSlopeThresh      *float64 `json:"liq_critical_slope_thresh"`
	LiqMinSamples               *int     `json:"liq_min_samples"`
	EnableCapitalAllocator      *bool    `json:"enable_capital_allocator"`
	MaxTotalExposureUSDT        *float64 `json:"max_total_exposure_usdt"`
	MaxPerpPerpPct              *float64 `json:"max_perp_perp_pct"`
	MaxSpotFuturesPct           *float64 `json:"max_spot_futures_pct"`
	MaxPerExchangePct           *float64 `json:"max_per_exchange_pct"`
	ReservationTTLSec           *int     `json:"reservation_ttl_sec"`
	EnableExchangeHealthScoring *bool    `json:"enable_exchange_health_scoring"`
	ExchHealthLatencyMs         *int     `json:"exch_health_latency_ms"`
	ExchHealthMinUptime         *float64 `json:"exch_health_min_uptime"`
	ExchHealthMinFillRate       *float64 `json:"exch_health_min_fill_rate"`
	ExchHealthMinScore          *float64 `json:"exch_health_min_score"`
	ExchHealthWindowMin         *int     `json:"exch_health_window_min"`
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
		MinHoldTime:                      16 * time.Hour,
		MaxCostRatio:                     0.50,
		MaxPositions:                     1,
		Leverage:                         3,
		SlippageBPS:                      50,
		RebalanceScanMinute:              10,
		EnablePoolAllocator:              false,
		TopPairsPerSymbol:                1,
		AllocatorTimeoutMs:               5,
		TopOpportunities:                 25,
		PriceGapFreeBPS:                  40,
		MaxPriceGapBPS:                   200,
		MaxGapRecoveryIntervals:          1.0,
		EntryTimeoutSec:                  300,
		MinChunkUSDT:                     10,
		MarginL3Threshold:                0.50,
		MarginL4Threshold:                0.80,
		MarginL5Threshold:                0.95,
		L4ReduceFraction:                 0.30,
		MarginSafetyMultiplier:           2.0,
		EntryMarginHeadroom:              0.80,
		ExitDepthTimeoutSec:              300,
		ExitMaxGapBPS:                    10.0,
		RiskMonitorIntervalSec:           300,
		EnableLiqTrendTracking:           false,
		LiqProjectionMinutes:             15,
		LiqWarningSlopeThresh:            0.002,
		LiqCriticalSlopeThresh:           0.004,
		LiqMinSamples:                    5,
		MaxPerpPerpPct:                   0.60,
		MaxSpotFuturesPct:                0.60,
		MaxPerExchangePct:                0.60,
		ReservationTTLSec:                300,
		EnableExchangeHealthScoring:      false,
		ExchHealthLatencyMs:              2000,
		ExchHealthMinUptime:              0.95,
		ExchHealthMinFillRate:            0.80,
		ExchHealthMinScore:               0.50,
		ExchHealthWindowMin:              60,
		PersistLookback1h:                90 * time.Minute,
		PersistMinCount1h:                1,
		PersistLookback4h:                180 * time.Minute,
		PersistMinCount4h:                1,
		PersistLookback8h:                360 * time.Minute,
		PersistMinCount8h:                1,
		SpreadStabilityRatio1h:           0.5,
		SpreadStabilityOIRank1h:          0,
		EnableSpreadStabilityGate:        false,
		SpreadVolatilityMaxCV:            0,
		SpreadVolatilityMinSamples:       10,
		SpreadStabilityStricterForAuto:   true,
		SpreadStabilityAutoCVMultiplier:  0.7,
		FundingWindowMin:                 30,
		LossCooldownHours:                4.0,
		BacktestDays:                     3,
		DelistFilterEnabled:              true,
		ScanMinutes:                      []int{10, 20, 30, 35, 40, 45, 50},
		EntryScanMinute:                  40,
		ExitScanMinute:                   30,
		RotateScanMinute:                 35,
		RotationThresholdBPS:             100,
		RotationCooldownMin:              180,
		EnableSpreadReversal:             true,
		SpreadReversalTolerance:          1,
		ZeroSpreadTolerance:              2,
		ReEnterCooldownHours:             1.0,
		RedisAddr:                        "localhost:6379",
		RedisDB:                          2,
		DashboardAddr:                    ":8080",
		AIModel:                          "gpt-5.4",
		AIMaxTokens:                      4096,
		SpotArbSchedule:                  "15,35",
		SpotArbChromePath:                detectChromePath(),
		SpotFuturesMaxPositions:          1,
		SpotFuturesLeverage:              3,
		SpotFuturesMonitorIntervalSec:    60,
		SpotFuturesMinNetYieldAPR:        0.10, // 10%
		SpotFuturesMaxBorrowAPR:          0.50, // 50%
		EnableBorrowSpikeDetection:       false,
		BorrowSpikeWindowMin:             60,
		BorrowSpikeMultiplier:            2.0,
		BorrowSpikeMinAbsolute:           0.10,
		SpotFuturesScanIntervalMin:       10,
		SpotFuturesBorrowGraceMin:        30,
		SpotFuturesPriceExitPct:          20.0,
		SpotFuturesPriceEmergencyPct:     30.0,
		SpotFuturesMarginExitPct:         85.0,
		SpotFuturesMarginEmergencyPct:    95.0,
		SpotFuturesLossCooldownHours:     4,
		SpotFuturesDryRun:                true,
		SpotFuturesPersistenceScans:      2,
		SpotFuturesCapitalSeparate:       200,
		SpotFuturesCapitalUnified:        500,
		SpotFuturesScannerMode:           "native",
		SpotFuturesEnableMinHold:         false,
		SpotFuturesMinHoldHours:          8,
		SpotFuturesEnableSettlementGuard: false,
		SpotFuturesSettlementWindowMin:   10,
		SpotFuturesEnablePriceGapGate:    false,
		SpotFuturesMaxPriceGapPct:        0.5,
		SpotFuturesEnableExitSpreadGate:  false,
		SpotFuturesExitSpreadPct:         0.3,

		// Phase 6: maintenance rate risk hardening defaults
		SpotFuturesEnableMaintenanceGate: false,
		SpotFuturesMaintenanceDefault:    0.05,
		SpotFuturesMaintenanceCacheTTL:   60,

		// Unified capital allocation defaults (Phase 5)
		EnableUnifiedCapital:   false,
		TotalCapitalUSDT:       0,
		RiskProfile:            "balanced",
		AllocationLookbackDays: 7,
		AllocationFloorPct:     0.20,
		AllocationCeilingPct:   0.80,
		SizeMultiplier:         1.0,

		// Safety defaults (all off by default per D-10)
		EnablePerpTelegram:  false,
		EnableLossLimits:    false,
		DailyLossLimitUSDT:  100.0,
		WeeklyLossLimitUSDT: 300.0,
		TelegramCooldownSec: 300,

		// Analytics defaults (off by default)
		EnableAnalytics: false,
		AnalyticsDBPath: "data/analytics.db",
	}

	// Load from JSON file
	c.loadJSON()

	// Environment variables override JSON values
	c.loadEnvOverrides()

	// Ensure special scan minutes are present in the schedule.
	c.EnsureScanMinutes()

	return c
}

// Lock acquires the config write lock.
func (c *Config) Lock() { c.mu.Lock() }

// Unlock releases the config write lock.
func (c *Config) Unlock() { c.mu.Unlock() }

// RLock acquires the config read lock.
func (c *Config) RLock() { c.mu.RLock() }

// RUnlock releases the config read lock.
func (c *Config) RUnlock() { c.mu.RUnlock() }

// isValidScannerMode returns true if mode is one of the accepted scanner modes.
func isValidScannerMode(mode string) bool {
	return mode == "native" || mode == "coinglass" || mode == "both"
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
	validMinute := func(v *int) bool {
		return v != nil && *v > 0 && *v <= 59
	}

	if jc.DryRun != nil {
		c.DryRun = *jc.DryRun
	}
	if jc.TradFiSigned != nil {
		c.TradFiSigned = *jc.TradFiSigned
	}

	// Strategy
	if s := jc.Strategy; s != nil {
		if s.TopOpportunities != nil && *s.TopOpportunities > 0 {
			c.TopOpportunities = *s.TopOpportunities
		}
		if len(s.ScanMinutes) > 0 {
			c.ScanMinutes = s.ScanMinutes
		}
		if validMinute(s.EntryScanMinute) {
			c.EntryScanMinute = *s.EntryScanMinute
		}
		if validMinute(s.ExitScanMinute) {
			c.ExitScanMinute = *s.ExitScanMinute
		}
		if validMinute(s.RotateScanMinute) {
			c.RotateScanMinute = *s.RotateScanMinute
		}
		if validMinute(s.RebalanceScanMinute) {
			c.RebalanceScanMinute = *s.RebalanceScanMinute
		}
		if s.EnablePoolAllocator != nil {
			c.EnablePoolAllocator = *s.EnablePoolAllocator
		}
		if s.TopPairsPerSymbol != nil && *s.TopPairsPerSymbol > 0 {
			c.TopPairsPerSymbol = *s.TopPairsPerSymbol
		}
		if s.AllocatorTimeoutMs != nil && *s.AllocatorTimeoutMs > 0 {
			c.AllocatorTimeoutMs = *s.AllocatorTimeoutMs
		}

		// Discovery
		if d := s.Discovery; d != nil {
			if d.MinHoldTimeHours != nil && *d.MinHoldTimeHours > 0 {
				c.MinHoldTime = time.Duration(*d.MinHoldTimeHours) * time.Hour
			}
			if d.MaxCostRatio != nil && *d.MaxCostRatio > 0 {
				c.MaxCostRatio = *d.MaxCostRatio
			}
			if d.MaxPriceGapBPS != nil && *d.MaxPriceGapBPS > 0 {
				c.MaxPriceGapBPS = *d.MaxPriceGapBPS
			}
			if d.PriceGapFreeBPS != nil && *d.PriceGapFreeBPS > 0 {
				c.PriceGapFreeBPS = *d.PriceGapFreeBPS
			}
			if d.MaxGapRecoveryIntervals != nil && *d.MaxGapRecoveryIntervals > 0 {
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
				if p.LookbackMin1h != nil && *p.LookbackMin1h > 0 {
					c.PersistLookback1h = time.Duration(*p.LookbackMin1h) * time.Minute
				}
				if p.MinCount1h != nil && *p.MinCount1h > 0 {
					c.PersistMinCount1h = *p.MinCount1h
				}
				if p.LookbackMin4h != nil && *p.LookbackMin4h > 0 {
					c.PersistLookback4h = time.Duration(*p.LookbackMin4h) * time.Minute
				}
				if p.MinCount4h != nil && *p.MinCount4h > 0 {
					c.PersistMinCount4h = *p.MinCount4h
				}
				if p.LookbackMin8h != nil && *p.LookbackMin8h > 0 {
					c.PersistLookback8h = time.Duration(*p.LookbackMin8h) * time.Minute
				}
				if p.MinCount8h != nil && *p.MinCount8h > 0 {
					c.PersistMinCount8h = *p.MinCount8h
				}
				if p.SpreadStabilityRatio1h != nil && *p.SpreadStabilityRatio1h > 0 {
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
				if p.EnableSpreadStabilityGate != nil {
					c.EnableSpreadStabilityGate = *p.EnableSpreadStabilityGate
				}
				if p.SpreadVolatilityMaxCV != nil {
					c.SpreadVolatilityMaxCV = *p.SpreadVolatilityMaxCV
				}
				if p.SpreadVolatilityMinSamples != nil && *p.SpreadVolatilityMinSamples > 0 {
					c.SpreadVolatilityMinSamples = *p.SpreadVolatilityMinSamples
				}
				if p.SpreadStabilityStricterForAuto != nil {
					c.SpreadStabilityStricterForAuto = *p.SpreadStabilityStricterForAuto
				}
				if p.SpreadStabilityAutoCVMultiplier != nil && *p.SpreadStabilityAutoCVMultiplier > 0 {
					c.SpreadStabilityAutoCVMultiplier = *p.SpreadStabilityAutoCVMultiplier
				}
				if p.FundingWindowMin != nil && *p.FundingWindowMin > 0 {
					c.FundingWindowMin = *p.FundingWindowMin
				}
			}
		}

		// Entry
		if e := s.Entry; e != nil {
			if e.SlippageLimitBPS != nil && *e.SlippageLimitBPS > 0 {
				c.SlippageBPS = *e.SlippageLimitBPS
			}
			if e.MinChunkUSDT != nil && *e.MinChunkUSDT > 0 {
				c.MinChunkUSDT = *e.MinChunkUSDT
			}
			if e.EntryTimeoutSec != nil && *e.EntryTimeoutSec > 0 {
				c.EntryTimeoutSec = *e.EntryTimeoutSec
			}
			if e.LossCooldownHours != nil {
				c.LossCooldownHours = *e.LossCooldownHours
			}
			if e.ReEnterCooldownHours != nil {
				c.ReEnterCooldownHours = *e.ReEnterCooldownHours
			}
			if e.BacktestDays != nil && *e.BacktestDays > 0 {
				c.BacktestDays = *e.BacktestDays
			}
			if e.BacktestMinProfit != nil {
				c.BacktestMinProfit = *e.BacktestMinProfit
			}
		}

		// Exit
		if x := s.Exit; x != nil {
			if x.DepthTimeoutSec != nil && *x.DepthTimeoutSec > 0 {
				c.ExitDepthTimeoutSec = *x.DepthTimeoutSec
			}
			if x.EnableSpreadReversal != nil {
				c.EnableSpreadReversal = *x.EnableSpreadReversal
			}
			if x.SpreadReversalTolerance != nil && *x.SpreadReversalTolerance > 0 {
				c.SpreadReversalTolerance = *x.SpreadReversalTolerance
			}
			if x.ZeroSpreadTolerance != nil && *x.ZeroSpreadTolerance > 0 {
				c.ZeroSpreadTolerance = *x.ZeroSpreadTolerance
			}
			if x.MaxGapBPS != nil && *x.MaxGapBPS > 0 {
				c.ExitMaxGapBPS = *x.MaxGapBPS
			}
		}

		// Rotation
		if r := s.Rotation; r != nil {
			if r.ThresholdBPS != nil && *r.ThresholdBPS > 0 {
				c.RotationThresholdBPS = *r.ThresholdBPS
			}
			if r.CooldownMin != nil && *r.CooldownMin > 0 {
				c.RotationCooldownMin = *r.CooldownMin
			}
		}
	}

	// Fund
	if f := jc.Fund; f != nil {
		if f.MaxPositions != nil && *f.MaxPositions > 0 {
			c.MaxPositions = *f.MaxPositions
		}
		if f.Leverage != nil && *f.Leverage > 0 {
			c.Leverage = *f.Leverage
		}
		if f.CapitalPerLeg != nil {
			c.CapitalPerLeg = *f.CapitalPerLeg
		}
		if validMinute(f.RebalanceScanMinute) && (jc.Strategy == nil || jc.Strategy.RebalanceScanMinute == nil) {
			c.RebalanceScanMinute = *f.RebalanceScanMinute
		}
	}

	// Risk
	if rk := jc.Risk; rk != nil {
		if rk.MarginL3Threshold != nil && *rk.MarginL3Threshold > 0 {
			c.MarginL3Threshold = *rk.MarginL3Threshold
		}
		if rk.MarginL4Threshold != nil && *rk.MarginL4Threshold > 0 {
			c.MarginL4Threshold = *rk.MarginL4Threshold
		}
		if rk.MarginL5Threshold != nil && *rk.MarginL5Threshold > 0 {
			c.MarginL5Threshold = *rk.MarginL5Threshold
		}
		if rk.L4ReduceFraction != nil && *rk.L4ReduceFraction > 0 {
			c.L4ReduceFraction = *rk.L4ReduceFraction
		}
		if rk.MarginSafetyMultiplier != nil && *rk.MarginSafetyMultiplier > 0 {
			c.MarginSafetyMultiplier = *rk.MarginSafetyMultiplier
		}
		if rk.EntryMarginHeadroom != nil && *rk.EntryMarginHeadroom > 0 {
			c.EntryMarginHeadroom = *rk.EntryMarginHeadroom
		}
		if rk.RiskMonitorIntervalSec != nil && *rk.RiskMonitorIntervalSec > 0 {
			c.RiskMonitorIntervalSec = *rk.RiskMonitorIntervalSec
		}
		if rk.EnableLiqTrendTracking != nil {
			c.EnableLiqTrendTracking = *rk.EnableLiqTrendTracking
		}
		if rk.LiqProjectionMinutes != nil && *rk.LiqProjectionMinutes > 0 {
			c.LiqProjectionMinutes = *rk.LiqProjectionMinutes
		}
		if rk.LiqWarningSlopeThresh != nil && *rk.LiqWarningSlopeThresh > 0 {
			c.LiqWarningSlopeThresh = *rk.LiqWarningSlopeThresh
		}
		if rk.LiqCriticalSlopeThresh != nil && *rk.LiqCriticalSlopeThresh > 0 {
			c.LiqCriticalSlopeThresh = *rk.LiqCriticalSlopeThresh
		}
		if rk.LiqMinSamples != nil && *rk.LiqMinSamples > 0 {
			c.LiqMinSamples = *rk.LiqMinSamples
		}
		if rk.EnableCapitalAllocator != nil {
			c.EnableCapitalAllocator = *rk.EnableCapitalAllocator
		}
		if rk.MaxTotalExposureUSDT != nil {
			c.MaxTotalExposureUSDT = *rk.MaxTotalExposureUSDT
		}
		if rk.MaxPerpPerpPct != nil && *rk.MaxPerpPerpPct > 0 {
			c.MaxPerpPerpPct = *rk.MaxPerpPerpPct
		}
		if rk.MaxSpotFuturesPct != nil && *rk.MaxSpotFuturesPct > 0 {
			c.MaxSpotFuturesPct = *rk.MaxSpotFuturesPct
		}
		if rk.MaxPerExchangePct != nil && *rk.MaxPerExchangePct > 0 {
			c.MaxPerExchangePct = *rk.MaxPerExchangePct
		}
		if rk.ReservationTTLSec != nil && *rk.ReservationTTLSec > 0 {
			c.ReservationTTLSec = *rk.ReservationTTLSec
		}
		if rk.EnableExchangeHealthScoring != nil {
			c.EnableExchangeHealthScoring = *rk.EnableExchangeHealthScoring
		}
		if rk.ExchHealthLatencyMs != nil && *rk.ExchHealthLatencyMs > 0 {
			c.ExchHealthLatencyMs = *rk.ExchHealthLatencyMs
		}
		if rk.ExchHealthMinUptime != nil && *rk.ExchHealthMinUptime > 0 {
			c.ExchHealthMinUptime = *rk.ExchHealthMinUptime
		}
		if rk.ExchHealthMinFillRate != nil && *rk.ExchHealthMinFillRate > 0 {
			c.ExchHealthMinFillRate = *rk.ExchHealthMinFillRate
		}
		if rk.ExchHealthMinScore != nil && *rk.ExchHealthMinScore > 0 {
			c.ExchHealthMinScore = *rk.ExchHealthMinScore
		}
		if rk.ExchHealthWindowMin != nil && *rk.ExchHealthWindowMin > 0 {
			c.ExchHealthWindowMin = *rk.ExchHealthWindowMin
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
		if a.MaxTokens != nil && *a.MaxTokens > 0 {
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
		if sf.MaxPositions != nil && *sf.MaxPositions > 0 {
			c.SpotFuturesMaxPositions = *sf.MaxPositions
		}
		if sf.Leverage != nil && *sf.Leverage > 0 {
			c.SpotFuturesLeverage = *sf.Leverage
		}
		if sf.MonitorIntervalSec != nil && *sf.MonitorIntervalSec > 0 {
			c.SpotFuturesMonitorIntervalSec = *sf.MonitorIntervalSec
		}
		if sf.MinNetYieldAPR != nil {
			c.SpotFuturesMinNetYieldAPR = *sf.MinNetYieldAPR
		}
		if sf.MaxBorrowAPR != nil {
			c.SpotFuturesMaxBorrowAPR = *sf.MaxBorrowAPR
		}
		if sf.EnableBorrowSpikeDetection != nil {
			c.EnableBorrowSpikeDetection = *sf.EnableBorrowSpikeDetection
		}
		if sf.BorrowSpikeWindowMin != nil && *sf.BorrowSpikeWindowMin > 0 {
			c.BorrowSpikeWindowMin = *sf.BorrowSpikeWindowMin
		}
		if sf.BorrowSpikeMultiplier != nil && *sf.BorrowSpikeMultiplier > 0 {
			c.BorrowSpikeMultiplier = *sf.BorrowSpikeMultiplier
		}
		if sf.BorrowSpikeMinAbsolute != nil && *sf.BorrowSpikeMinAbsolute > 0 {
			c.BorrowSpikeMinAbsolute = *sf.BorrowSpikeMinAbsolute
		}
		if len(sf.Exchanges) > 0 {
			c.SpotFuturesExchanges = sf.Exchanges
		}
		if sf.ScanIntervalMin != nil && *sf.ScanIntervalMin > 0 {
			c.SpotFuturesScanIntervalMin = *sf.ScanIntervalMin
		}
		if sf.BorrowGraceMin != nil {
			c.SpotFuturesBorrowGraceMin = *sf.BorrowGraceMin
		}
		if sf.PriceExitPct != nil && *sf.PriceExitPct > 0 {
			c.SpotFuturesPriceExitPct = *sf.PriceExitPct
		}
		if sf.PriceEmergencyPct != nil && *sf.PriceEmergencyPct > 0 {
			c.SpotFuturesPriceEmergencyPct = *sf.PriceEmergencyPct
		}
		if sf.MarginExitPct != nil && *sf.MarginExitPct > 0 {
			c.SpotFuturesMarginExitPct = *sf.MarginExitPct
		}
		if sf.MarginEmergencyPct != nil && *sf.MarginEmergencyPct > 0 {
			c.SpotFuturesMarginEmergencyPct = *sf.MarginEmergencyPct
		}
		if sf.LossCooldownHours != nil {
			c.SpotFuturesLossCooldownHours = *sf.LossCooldownHours
		}
		if sf.AutoEnabled != nil {
			c.SpotFuturesAutoEnabled = *sf.AutoEnabled
		}
		if sf.AutoDryRun != nil {
			c.SpotFuturesDryRun = *sf.AutoDryRun
		}
		if sf.PersistenceScans != nil && *sf.PersistenceScans > 0 {
			c.SpotFuturesPersistenceScans = *sf.PersistenceScans
		}
		if sf.ProfitTransferEnabled != nil {
			c.SpotFuturesProfitTransferEnabled = *sf.ProfitTransferEnabled
		}
		if sf.CapitalSeparateUSDT != nil && *sf.CapitalSeparateUSDT > 0 {
			c.SpotFuturesCapitalSeparate = *sf.CapitalSeparateUSDT
		}
		if sf.CapitalUnifiedUSDT != nil && *sf.CapitalUnifiedUSDT > 0 {
			c.SpotFuturesCapitalUnified = *sf.CapitalUnifiedUSDT
		}
		if sf.ScannerMode != nil && isValidScannerMode(*sf.ScannerMode) {
			c.SpotFuturesScannerMode = *sf.ScannerMode
		} else if sf.NativeScannerEnabled != nil {
			// Backward compat: map old bool to new mode.
			if *sf.NativeScannerEnabled {
				c.SpotFuturesScannerMode = "native"
			} else {
				c.SpotFuturesScannerMode = "coinglass"
			}
		}
		if sf.EnableMinHold != nil {
			c.SpotFuturesEnableMinHold = *sf.EnableMinHold
		}
		if sf.MinHoldHours != nil && *sf.MinHoldHours > 0 {
			c.SpotFuturesMinHoldHours = *sf.MinHoldHours
		}
		if sf.EnableSettlementGuard != nil {
			c.SpotFuturesEnableSettlementGuard = *sf.EnableSettlementGuard
		}
		if sf.SettlementWindowMin != nil && *sf.SettlementWindowMin > 0 {
			c.SpotFuturesSettlementWindowMin = *sf.SettlementWindowMin
		}
		if sf.EnablePriceGapGate != nil {
			c.SpotFuturesEnablePriceGapGate = *sf.EnablePriceGapGate
		}
		if sf.MaxPriceGapPct != nil && *sf.MaxPriceGapPct > 0 {
			c.SpotFuturesMaxPriceGapPct = *sf.MaxPriceGapPct
		}
		if sf.EnableExitSpreadGate != nil {
			c.SpotFuturesEnableExitSpreadGate = *sf.EnableExitSpreadGate
		}
		if sf.ExitSpreadPct != nil && *sf.ExitSpreadPct > 0 {
			c.SpotFuturesExitSpreadPct = *sf.ExitSpreadPct
		}
		// Phase 6: maintenance rate risk hardening
		if sf.EnableMaintenanceGate != nil {
			c.SpotFuturesEnableMaintenanceGate = *sf.EnableMaintenanceGate
		}
		if sf.MaintenanceDefault != nil && *sf.MaintenanceDefault > 0 && *sf.MaintenanceDefault < 1.0 {
			c.SpotFuturesMaintenanceDefault = *sf.MaintenanceDefault
		}
		if sf.MaintenanceCacheTTL != nil && *sf.MaintenanceCacheTTL >= 1 {
			c.SpotFuturesMaintenanceCacheTTL = *sf.MaintenanceCacheTTL
		}
	}

	// Telegram
	if tg := jc.Telegram; tg != nil {
		if tg.BotToken != "" {
			c.TelegramBotToken = tg.BotToken
		}
		if tg.ChatID != "" {
			c.TelegramChatID = tg.ChatID
		}
	}

	// Allocation (Phase 5)
	if alloc := jc.Allocation; alloc != nil {
		if alloc.EnableUnifiedCapital != nil {
			c.EnableUnifiedCapital = *alloc.EnableUnifiedCapital
		}
		if alloc.TotalCapitalUSDT != nil && *alloc.TotalCapitalUSDT >= 0 {
			c.TotalCapitalUSDT = *alloc.TotalCapitalUSDT
		}
		if alloc.RiskProfile != nil {
			c.RiskProfile = *alloc.RiskProfile
		}
		if alloc.AllocationLookbackDays != nil && *alloc.AllocationLookbackDays > 0 {
			c.AllocationLookbackDays = *alloc.AllocationLookbackDays
		}
		if alloc.AllocationFloorPct != nil && *alloc.AllocationFloorPct >= 0 {
			c.AllocationFloorPct = *alloc.AllocationFloorPct
		}
		if alloc.AllocationCeilingPct != nil && *alloc.AllocationCeilingPct > 0 {
			c.AllocationCeilingPct = *alloc.AllocationCeilingPct
		}
		if alloc.SizeMultiplier != nil && *alloc.SizeMultiplier > 0 {
			c.SizeMultiplier = *alloc.SizeMultiplier
		}
	}

	// Safety
	if sa := jc.Safety; sa != nil {
		if sa.EnableLossLimits != nil {
			c.EnableLossLimits = *sa.EnableLossLimits
		}
		if sa.DailyLossLimitUSDT != nil && *sa.DailyLossLimitUSDT > 0 {
			c.DailyLossLimitUSDT = *sa.DailyLossLimitUSDT
		}
		if sa.WeeklyLossLimitUSDT != nil && *sa.WeeklyLossLimitUSDT > 0 {
			c.WeeklyLossLimitUSDT = *sa.WeeklyLossLimitUSDT
		}
		if sa.EnablePerpTelegram != nil {
			c.EnablePerpTelegram = *sa.EnablePerpTelegram
		}
		if sa.TelegramCooldownSec != nil && *sa.TelegramCooldownSec >= 0 {
			c.TelegramCooldownSec = *sa.TelegramCooldownSec
		}
	}

	// Analytics
	if an := jc.Analytics; an != nil {
		if an.EnableAnalytics != nil {
			c.EnableAnalytics = *an.EnableAnalytics
		}
		if an.AnalyticsDBPath != nil && *an.AnalyticsDBPath != "" {
			c.AnalyticsDBPath = *an.AnalyticsDBPath
		}
	}
}

// ExchangeSecretOverride contains explicit secret replacements to persist.
// Fields left empty preserve the current on-disk values.
type ExchangeSecretOverride struct {
	APIKey     string
	SecretKey  string
	Passphrase string
}

// SaveJSON writes the current runtime config back to config.json while
// preserving exchange secrets from the original file.
func (c *Config) SaveJSON() error {
	return c.SaveJSONWithExchangeSecretOverrides(nil)
}

// SaveJSONWithExchangeSecretOverrides writes the current runtime config back to
// config.json and applies only the explicitly supplied exchange secret updates.
func (c *Config) SaveJSONWithExchangeSecretOverrides(overrides map[string]ExchangeSecretOverride) error {
	// Find the config file path
	paths := []string{"config.json", "/var/solana/data/arb/config.json"}
	if p := os.Getenv("CONFIG_FILE"); p != "" {
		paths = []string{p}
	}
	var filePath string
	var originalData []byte
	var raw map[string]interface{}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err == nil {
			filePath = path
			originalData = data
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
	raw["tradfi_signed"] = c.TradFiSigned

	strategy := getMap(raw, "strategy")
	strategy["top_opportunities"] = c.TopOpportunities
	strategy["scan_minutes"] = c.ScanMinutes
	// Only persist scan minutes when explicitly set (non-zero).
	// Writing 0 would overwrite the user's tuned values on next load.
	if c.EntryScanMinute > 0 {
		strategy["entry_scan_minute"] = c.EntryScanMinute
	}
	if c.ExitScanMinute > 0 {
		strategy["exit_scan_minute"] = c.ExitScanMinute
	}
	if c.RotateScanMinute > 0 {
		strategy["rotate_scan_minute"] = c.RotateScanMinute
	}
	if c.RebalanceScanMinute > 0 {
		strategy["rebalance_scan_minute"] = c.RebalanceScanMinute
	}
	strategy["enable_pool_allocator"] = c.EnablePoolAllocator
	strategy["top_pairs_per_symbol"] = c.TopPairsPerSymbol
	strategy["allocator_timeout_ms"] = c.AllocatorTimeoutMs

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
	persist["enable_spread_stability_gate"] = c.EnableSpreadStabilityGate
	persist["spread_volatility_max_cv"] = c.SpreadVolatilityMaxCV
	persist["spread_volatility_min_samples"] = c.SpreadVolatilityMinSamples
	persist["spread_stability_stricter_for_auto"] = c.SpreadStabilityStricterForAuto
	persist["spread_stability_auto_cv_multiplier"] = c.SpreadStabilityAutoCVMultiplier
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
	risk["entry_margin_headroom"] = c.EntryMarginHeadroom
	risk["risk_monitor_interval_sec"] = c.RiskMonitorIntervalSec
	risk["enable_liq_trend_tracking"] = c.EnableLiqTrendTracking
	risk["liq_projection_minutes"] = c.LiqProjectionMinutes
	risk["liq_warning_slope_thresh"] = c.LiqWarningSlopeThresh
	risk["liq_critical_slope_thresh"] = c.LiqCriticalSlopeThresh
	risk["liq_min_samples"] = c.LiqMinSamples
	risk["enable_capital_allocator"] = c.EnableCapitalAllocator
	risk["max_total_exposure_usdt"] = c.MaxTotalExposureUSDT
	risk["max_perp_perp_pct"] = c.MaxPerpPerpPct
	risk["max_spot_futures_pct"] = c.MaxSpotFuturesPct
	risk["max_per_exchange_pct"] = c.MaxPerExchangePct
	risk["reservation_ttl_sec"] = c.ReservationTTLSec
	risk["enable_exchange_health_scoring"] = c.EnableExchangeHealthScoring
	risk["exch_health_latency_ms"] = c.ExchHealthLatencyMs
	risk["exch_health_min_uptime"] = c.ExchHealthMinUptime
	risk["exch_health_min_fill_rate"] = c.ExchHealthMinFillRate
	risk["exch_health_min_score"] = c.ExchHealthMinScore
	risk["exch_health_window_min"] = c.ExchHealthWindowMin

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
		if override, ok := overrides[ed.name]; ok {
			if override.APIKey != "" {
				exMap["api_key"] = override.APIKey
			}
			if override.SecretKey != "" {
				exMap["secret_key"] = override.SecretKey
			}
			if ed.hasPass && override.Passphrase != "" {
				exMap["passphrase"] = override.Passphrase
			}
		}
		if ed.enabled != nil {
			exMap["enabled"] = *ed.enabled
		}
		if addrs, ok := c.ExchangeAddresses[ed.name]; ok && len(addrs) > 0 {
			exMap["address"] = addrs
		}
	}

	sf := getMap(raw, "spot_futures")
	sf["enabled"] = c.SpotFuturesEnabled
	sf["max_positions"] = c.SpotFuturesMaxPositions
	sf["leverage"] = c.SpotFuturesLeverage
	sf["monitor_interval_sec"] = c.SpotFuturesMonitorIntervalSec
	sf["min_net_yield_apr"] = c.SpotFuturesMinNetYieldAPR
	sf["max_borrow_apr"] = c.SpotFuturesMaxBorrowAPR
	sf["enable_borrow_spike_detection"] = c.EnableBorrowSpikeDetection
	sf["borrow_spike_window_min"] = c.BorrowSpikeWindowMin
	sf["borrow_spike_multiplier"] = c.BorrowSpikeMultiplier
	sf["borrow_spike_min_absolute"] = c.BorrowSpikeMinAbsolute
	sf["exchanges"] = c.SpotFuturesExchanges
	sf["scan_interval_min"] = c.SpotFuturesScanIntervalMin
	sf["borrow_grace_min"] = c.SpotFuturesBorrowGraceMin
	sf["price_exit_pct"] = c.SpotFuturesPriceExitPct
	sf["price_emergency_pct"] = c.SpotFuturesPriceEmergencyPct
	sf["margin_exit_pct"] = c.SpotFuturesMarginExitPct
	sf["margin_emergency_pct"] = c.SpotFuturesMarginEmergencyPct
	sf["loss_cooldown_hours"] = c.SpotFuturesLossCooldownHours
	sf["auto_enabled"] = c.SpotFuturesAutoEnabled
	sf["auto_dry_run"] = c.SpotFuturesDryRun
	sf["persistence_scans"] = c.SpotFuturesPersistenceScans
	sf["profit_transfer_enabled"] = c.SpotFuturesProfitTransferEnabled
	sf["capital_separate_usdt"] = c.SpotFuturesCapitalSeparate
	sf["capital_unified_usdt"] = c.SpotFuturesCapitalUnified
	sf["scanner_mode"] = c.SpotFuturesScannerMode
	delete(sf, "native_scanner_enabled") // clean old key
	sf["enable_min_hold"] = c.SpotFuturesEnableMinHold
	sf["min_hold_hours"] = c.SpotFuturesMinHoldHours
	sf["enable_settlement_guard"] = c.SpotFuturesEnableSettlementGuard
	sf["settlement_window_min"] = c.SpotFuturesSettlementWindowMin
	sf["enable_price_gap_gate"] = c.SpotFuturesEnablePriceGapGate
	sf["max_price_gap_pct"] = c.SpotFuturesMaxPriceGapPct
	sf["enable_exit_spread_gate"] = c.SpotFuturesEnableExitSpreadGate
	sf["exit_spread_pct"] = c.SpotFuturesExitSpreadPct
	sf["enable_maintenance_gate"] = c.SpotFuturesEnableMaintenanceGate
	sf["maintenance_default"] = c.SpotFuturesMaintenanceDefault
	sf["maintenance_cache_ttl"] = c.SpotFuturesMaintenanceCacheTTL
	delete(sf, "capital_per_position") // removed field

	// Allocation (Phase 5)
	alloc := getMap(raw, "allocation")
	alloc["enable_unified_capital"] = c.EnableUnifiedCapital
	alloc["total_capital_usdt"] = c.TotalCapitalUSDT
	alloc["risk_profile"] = c.RiskProfile
	alloc["allocation_lookback_days"] = c.AllocationLookbackDays
	alloc["allocation_floor_pct"] = c.AllocationFloorPct
	alloc["allocation_ceiling_pct"] = c.AllocationCeilingPct
	alloc["size_multiplier"] = c.SizeMultiplier

	// Safety
	safety := getMap(raw, "safety")
	safety["enable_loss_limits"] = c.EnableLossLimits
	safety["daily_loss_limit_usdt"] = c.DailyLossLimitUSDT
	safety["weekly_loss_limit_usdt"] = c.WeeklyLossLimitUSDT
	safety["enable_perp_telegram"] = c.EnablePerpTelegram
	safety["telegram_cooldown_sec"] = c.TelegramCooldownSec

	// Analytics
	analyticsMap := getMap(raw, "analytics")
	analyticsMap["enable_analytics"] = c.EnableAnalytics
	analyticsMap["analytics_db_path"] = c.AnalyticsDBPath

	out, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	out = append(out, '\n')

	if len(originalData) > 0 {
		if err := os.WriteFile(filePath+".bak", originalData, 0644); err != nil {
			return fmt.Errorf("write backup config: %w", err)
		}
	}

	// Log caller so config.json writes are traceable.
	_, caller, line, _ := runtime.Caller(1)
	fmt.Fprintf(os.Stderr, "[config] SaveJSON: writing %s (caller: %s:%d)\n", filePath, caller, line)

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
	if v := os.Getenv("SPOT_FUTURES_ENABLE_BORROW_SPIKE_DETECTION"); v != "" {
		c.EnableBorrowSpikeDetection = v == "1" || v == "true" || v == "yes"
	}
	if v := os.Getenv("SPOT_FUTURES_BORROW_SPIKE_WINDOW_MIN"); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			c.BorrowSpikeWindowMin = i
		}
	}
	if v := os.Getenv("SPOT_FUTURES_BORROW_SPIKE_MULTIPLIER"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			c.BorrowSpikeMultiplier = f
		}
	}
	if v := os.Getenv("SPOT_FUTURES_BORROW_SPIKE_MIN_ABSOLUTE"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			c.BorrowSpikeMinAbsolute = f
		}
	}
	if v := os.Getenv("SPOT_FUTURES_SCANNER_MODE"); v != "" {
		if isValidScannerMode(v) {
			c.SpotFuturesScannerMode = v
		}
	} else if v := os.Getenv("SPOT_FUTURES_NATIVE_SCANNER_ENABLED"); v != "" {
		// Backward compat: map old env var to new mode.
		if v == "1" || v == "true" || v == "yes" {
			c.SpotFuturesScannerMode = "native"
		} else {
			c.SpotFuturesScannerMode = "coinglass"
		}
	}
	if v := os.Getenv("SPOT_FUTURES_ENABLE_MIN_HOLD"); v != "" {
		c.SpotFuturesEnableMinHold = v == "1" || v == "true" || v == "yes"
	}
	if v := os.Getenv("SPOT_FUTURES_MIN_HOLD_HOURS"); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			c.SpotFuturesMinHoldHours = i
		}
	}
	if v := os.Getenv("SPOT_FUTURES_ENABLE_SETTLEMENT_GUARD"); v != "" {
		c.SpotFuturesEnableSettlementGuard = v == "1" || v == "true" || v == "yes"
	}
	if v := os.Getenv("SPOT_FUTURES_SETTLEMENT_WINDOW_MIN"); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			c.SpotFuturesSettlementWindowMin = i
		}
	}
	if v := os.Getenv("SPOT_FUTURES_ENABLE_PRICE_GAP_GATE"); v != "" {
		c.SpotFuturesEnablePriceGapGate = v == "1" || v == "true" || v == "yes"
	} else if v := os.Getenv("SPOT_FUTURES_ENABLE_BASIS_GATE"); v != "" {
		c.SpotFuturesEnablePriceGapGate = v == "1" || v == "true" || v == "yes"
	}
	if v := os.Getenv("SPOT_FUTURES_MAX_PRICE_GAP_PCT"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			c.SpotFuturesMaxPriceGapPct = f
		}
	} else if v := os.Getenv("SPOT_FUTURES_MAX_BASIS_PCT"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			c.SpotFuturesMaxPriceGapPct = f
		}
	}
	if v := os.Getenv("SPOT_FUTURES_ENABLE_EXIT_SPREAD_GATE"); v != "" {
		c.SpotFuturesEnableExitSpreadGate = v == "1" || v == "true" || v == "yes"
	}
	if v := os.Getenv("SPOT_FUTURES_EXIT_SPREAD_PCT"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			c.SpotFuturesExitSpreadPct = f
		}
	}
	// Phase 6: maintenance rate risk hardening env vars
	if v := os.Getenv("SPOT_FUTURES_ENABLE_MAINTENANCE_GATE"); v != "" {
		c.SpotFuturesEnableMaintenanceGate = v == "1" || v == "true" || v == "yes"
	}
	if v := os.Getenv("SPOT_FUTURES_MAINTENANCE_DEFAULT"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 && f < 1.0 {
			c.SpotFuturesMaintenanceDefault = f
		}
	}
	if v := os.Getenv("SPOT_FUTURES_MAINTENANCE_CACHE_TTL"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 1 {
			c.SpotFuturesMaintenanceCacheTTL = n
		}
	}

	// Telegram
	if v := os.Getenv("TELEGRAM_BOT_TOKEN"); v != "" {
		c.TelegramBotToken = v
	}
	if v := os.Getenv("TELEGRAM_CHAT_ID"); v != "" {
		c.TelegramChatID = v
	}

	// Allocation (Phase 5)
	if v := os.Getenv("ENABLE_UNIFIED_CAPITAL"); v == "true" || v == "1" {
		c.EnableUnifiedCapital = true
	}
	if v := os.Getenv("TOTAL_CAPITAL_USDT"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			c.TotalCapitalUSDT = f
		}
	}
	if v := os.Getenv("RISK_PROFILE"); v != "" {
		c.RiskProfile = v
	}
	if v := os.Getenv("ALLOCATION_LOOKBACK_DAYS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			c.AllocationLookbackDays = n
		}
	}

	// Safety
	if v := os.Getenv("ENABLE_LOSS_LIMITS"); v != "" {
		c.EnableLossLimits = v == "1" || v == "true"
	}
	if v := os.Getenv("DAILY_LOSS_LIMIT_USDT"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 {
			c.DailyLossLimitUSDT = f
		}
	}
	if v := os.Getenv("WEEKLY_LOSS_LIMIT_USDT"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 {
			c.WeeklyLossLimitUSDT = f
		}
	}
	if v := os.Getenv("ENABLE_PERP_TELEGRAM"); v != "" {
		c.EnablePerpTelegram = v == "1" || v == "true"
	}
	if v := os.Getenv("TELEGRAM_COOLDOWN_SEC"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			c.TelegramCooldownSec = n
		}
	}

	// Analytics
	if v := os.Getenv("ENABLE_ANALYTICS"); v != "" {
		c.EnableAnalytics = v == "1" || v == "true"
	}
	if v := os.Getenv("ANALYTICS_DB_PATH"); v != "" {
		c.AnalyticsDBPath = v
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
