package notify

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"arb/internal/models"
	"arb/pkg/utils"
)

// TelegramNotifier sends trade alerts via Telegram Bot API.
// Used by both spot-futures and perp-perp engines.
type TelegramNotifier struct {
	botToken string
	chatID   string
	client   *http.Client
	log      *utils.Logger

	// Per-event-type cooldown to prevent notification spam.
	cooldownMu sync.Mutex
	lastSent   map[string]time.Time // event type key -> last sent time
}

// NewTelegram creates a notifier. Returns nil if botToken or chatID is empty.
func NewTelegram(botToken, chatID string) *TelegramNotifier {
	if botToken == "" || chatID == "" {
		return nil
	}
	return &TelegramNotifier{
		botToken: botToken,
		chatID:   chatID,
		client:   &http.Client{Timeout: 10 * time.Second},
		log:      utils.NewLogger("telegram"),
		lastSent: make(map[string]time.Time),
	}
}

// send delivers a message via Telegram sendMessage API. Best-effort: logs
// errors but never blocks the caller.
func (t *TelegramNotifier) send(text string) {
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", t.botToken)
	form := url.Values{
		"chat_id":    {t.chatID},
		"text":       {text},
		"parse_mode": {"Markdown"},
	}
	resp, err := t.client.PostForm(apiURL, form)
	if err != nil {
		t.log.Warn("sendMessage failed: %v", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.log.Warn("sendMessage HTTP %d: %s", resp.StatusCode, string(body))
	}
}

// NotifyAutoEntry sends an alert when a new spot-futures position is opened automatically.
func (t *TelegramNotifier) NotifyAutoEntry(pos *models.SpotFuturesPosition, expectedYieldAPR float64) {
	if t == nil {
		return
	}
	dir := "Dir A (borrow+sell+long)"
	if pos.Direction == "buy_spot_short" {
		dir = "Dir B (buy+short)"
	}
	text := fmt.Sprintf(
		"*Auto-Entry*\n"+
			"Symbol: `%s`\n"+
			"Direction: %s\n"+
			"Exchange: %s\n"+
			"Size: %.2f USDT\n"+
			"Expected Yield: %.1f%% APR",
		pos.Symbol, dir, pos.Exchange,
		pos.NotionalUSDT, expectedYieldAPR*100,
	)
	go t.send(text)
}

// NotifyAutoExit sends an alert when a position is closed by an automated exit trigger.
func (t *TelegramNotifier) NotifyAutoExit(pos *models.SpotFuturesPosition, reason string, pnl float64, duration time.Duration) {
	if t == nil {
		return
	}
	pnlSign := "+"
	if pnl < 0 {
		pnlSign = ""
	}
	reasonLabel := formatExitReason(reason)
	text := fmt.Sprintf(
		"*Auto-Exit*\n"+
			"Symbol: `%s`\n"+
			"Reason: %s\n"+
			"PnL: %s%.4f USDT\n"+
			"Duration: %s\n"+
			"Exchange: %s",
		pos.Symbol, reasonLabel,
		pnlSign, pnl,
		formatDuration(duration),
		pos.Exchange,
	)
	go t.send(text)
}

// NotifyEmergencyClose sends a high-priority alert for emergency position closure.
func (t *TelegramNotifier) NotifyEmergencyClose(pos *models.SpotFuturesPosition, trigger string, pnl float64) {
	if t == nil {
		return
	}
	pnlSign := "+"
	if pnl < 0 {
		pnlSign = ""
	}
	triggerLabel := formatExitReason(trigger)
	text := fmt.Sprintf(
		"\xE2\x9A\xA0 *EMERGENCY CLOSE*\n"+
			"Symbol: `%s`\n"+
			"Trigger: %s\n"+
			"PnL: %s%.4f USDT\n"+
			"Exchange: %s",
		pos.Symbol, triggerLabel,
		pnlSign, pnl,
		pos.Exchange,
	)
	go t.send(text)
}

// ---------------------------------------------------------------------------
// Cooldown: per-event-type dedup (5 minutes)
// ---------------------------------------------------------------------------

// checkCooldown returns true if enough time has passed since last notification
// of this type. Thread-safe. Uses wall-clock time.
func (t *TelegramNotifier) checkCooldown(eventKey string) bool {
	return t.checkCooldownAt(eventKey, time.Now())
}

// checkCooldownAt is the testable core of checkCooldown with an explicit timestamp.
func (t *TelegramNotifier) checkCooldownAt(eventKey string, now time.Time) bool {
	t.cooldownMu.Lock()
	defer t.cooldownMu.Unlock()
	if last, ok := t.lastSent[eventKey]; ok && now.Sub(last) < 5*time.Minute {
		return false
	}
	t.lastSent[eventKey] = now
	return true
}

// ---------------------------------------------------------------------------
// Perp-perp notification methods
// ---------------------------------------------------------------------------

// NotifySLTriggered sends an alert when a stop-loss fill is detected on a
// perp-perp position leg. Nil-receiver safe.
func (t *TelegramNotifier) NotifySLTriggered(pos *models.ArbitragePosition, leg, exchName string) {
	if t == nil {
		return
	}
	if !t.checkCooldown("sl_triggered") {
		return
	}
	sym := ""
	if pos != nil {
		sym = pos.Symbol
	}
	text := fmt.Sprintf(
		"\xE2\x9A\xA0 *SL Triggered*\n"+
			"Symbol: `%s`\n"+
			"Exchange: %s\n"+
			"Leg: %s\n"+
			"Emergency close initiated",
		sym, exchName, leg,
	)
	go t.send(text)
}

// NotifyEmergencyClosePerp sends an alert for L4/L5 margin health emergency
// actions on perp-perp positions. Nil-receiver safe.
func (t *TelegramNotifier) NotifyEmergencyClosePerp(exchName string, level string, posCount int) {
	if t == nil {
		return
	}
	if !t.checkCooldown("emergency_close_perp:" + exchName) {
		return
	}
	text := fmt.Sprintf(
		"\xE2\x9A\xA0 *%s Emergency Action*\n"+
			"Exchange: %s\n"+
			"Positions affected: %d\n"+
			"Margin health critical — reducing exposure",
		level, exchName, posCount,
	)
	go t.send(text)
}

// NotifyConsecutiveAPIErrors sends an alert when an exchange accumulates 3+
// consecutive API errors. Nil-receiver safe.
func (t *TelegramNotifier) NotifyConsecutiveAPIErrors(exchName string, errCount int, lastErr error) {
	if t == nil {
		return
	}
	if !t.checkCooldown("api_errors:" + exchName) {
		return
	}
	errMsg := "<nil>"
	if lastErr != nil {
		errMsg = lastErr.Error()
	}
	text := fmt.Sprintf(
		"\xE2\x9A\xA0 *API Error Alert*\n"+
			"Exchange: %s\n"+
			"Consecutive failures: %d\n"+
			"Last error: %s",
		exchName, errCount, errMsg,
	)
	go t.send(text)
}

// NotifyLossLimitBreached sends an alert when daily or weekly loss limits are
// breached. New entries halted but existing positions continue. Nil-receiver safe.
func (t *TelegramNotifier) NotifyLossLimitBreached(breachType string, dailyLoss, dailyLimit, weeklyLoss, weeklyLimit float64) {
	if t == nil {
		return
	}
	if !t.checkCooldown("loss_limit_breached") {
		return
	}
	text := fmt.Sprintf(
		"\xE2\x9A\xA0 *Loss Limit Breached (%s)*\n"+
			"Daily: %.2f / %.2f USDT\n"+
			"Weekly: %.2f / %.2f USDT\n"+
			"New entries halted. Existing positions continue.",
		breachType, dailyLoss, dailyLimit, weeklyLoss, weeklyLimit,
	)
	go t.send(text)
}

func formatExitReason(reason string) string {
	switch reason {
	case "borrow_cost_exceeded":
		return "Borrow cost exceeded"
	case "borrow_rate_spike":
		return "Borrow rate spike"
	case "yield_below_minimum":
		return "Yield below minimum"
	case "price_spike_exit":
		return "Price spike"
	case "emergency_price_spike":
		return "Emergency price spike"
	case "margin_health_exit":
		return "Margin health"
	case "manual_close":
		return "Manual close"
	default:
		return strings.ReplaceAll(reason, "_", " ")
	}
}

func formatDuration(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh%dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}
