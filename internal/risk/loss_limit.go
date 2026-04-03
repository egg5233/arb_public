package risk

import (
	"time"

	"arb/internal/config"
	"arb/internal/database"
	"arb/internal/notify"
	"arb/pkg/utils"
)

// LossLimitStatus represents the current state of rolling loss limits.
type LossLimitStatus struct {
	Enabled     bool    `json:"enabled"`
	DailyLoss   float64 `json:"daily_loss"`
	DailyLimit  float64 `json:"daily_limit"`
	WeeklyLoss  float64 `json:"weekly_loss"`
	WeeklyLimit float64 `json:"weekly_limit"`
	Breached    bool    `json:"breached"`
	BreachType  string  `json:"breach_type"` // "daily", "weekly", or ""
}

// LossLimitChecker tracks realized PnL and enforces rolling window loss limits.
// Independent from L0-L5 risk tiers (per D-08).
type LossLimitChecker struct {
	db       *database.Client
	cfg      *config.Config // pointer to shared config for hot-reload (per pitfall 5)
	log      *utils.Logger
	telegram *notify.TelegramNotifier
}

// NewLossLimitChecker creates a new loss limit checker.
func NewLossLimitChecker(db *database.Client, cfg *config.Config, log *utils.Logger, telegram *notify.TelegramNotifier) *LossLimitChecker {
	return &LossLimitChecker{
		db:       db,
		cfg:      cfg,
		log:      log,
		telegram: telegram,
	}
}

// RecordClosedPnL stores a closed position's PnL in the rolling window.
// Records ALL trades (wins and losses) for accurate net PnL calculation.
func (l *LossLimitChecker) RecordClosedPnL(posID string, pnl float64, symbol string, closedAt time.Time) {
	if l == nil {
		return
	}
	if err := l.db.RecordLossEvent(posID, pnl, symbol, closedAt); err != nil {
		if l.log != nil {
			l.log.Error("loss_limit: failed to record PnL event: %v", err)
		}
	}
}

// CheckLimits queries rolling 24h and 7d windows and returns whether entries should be blocked.
// Reads thresholds from cfg at call time (not construction time) for hot-reload support.
func (l *LossLimitChecker) CheckLimits() (bool, LossLimitStatus) {
	if l == nil {
		return false, LossLimitStatus{}
	}

	status := LossLimitStatus{
		Enabled:     l.cfg.EnableLossLimits,
		DailyLimit:  l.cfg.DailyLossLimitUSDT,
		WeeklyLimit: l.cfg.WeeklyLossLimitUSDT,
	}

	if !l.cfg.EnableLossLimits {
		return false, status
	}

	now := time.Now().UTC()

	// Query 24h rolling window
	dailyPnls, err := l.db.GetLossEventsInWindow(now.Add(-24 * time.Hour))
	if err != nil {
		if l.log != nil {
			l.log.Error("loss_limit: failed to query 24h window: %v", err)
		}
		return false, status // fail-open: don't block on query errors
	}
	for _, pnl := range dailyPnls {
		status.DailyLoss += pnl
	}

	// Query 7d rolling window
	weeklyPnls, err := l.db.GetLossEventsInWindow(now.Add(-7 * 24 * time.Hour))
	if err != nil {
		if l.log != nil {
			l.log.Error("loss_limit: failed to query 7d window: %v", err)
		}
		return false, status
	}
	for _, pnl := range weeklyPnls {
		status.WeeklyLoss += pnl
	}

	// Check thresholds (loss values are negative, thresholds are positive)
	// Breach when net loss exceeds threshold: e.g., dailyLoss=-120, limit=100 -> -120 < -100 -> breached
	if status.DailyLimit > 0 && status.DailyLoss < -status.DailyLimit {
		status.Breached = true
		status.BreachType = "daily"
		return true, status
	}
	if status.WeeklyLimit > 0 && status.WeeklyLoss < -status.WeeklyLimit {
		status.Breached = true
		status.BreachType = "weekly"
		return true, status
	}

	return false, status
}
