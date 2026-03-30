package notify

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"arb/internal/models"
	"arb/pkg/utils"
)

// TelegramNotifier sends spot-futures trade alerts via Telegram Bot API.
type TelegramNotifier struct {
	botToken string
	chatID   string
	client   *http.Client
	log      *utils.Logger
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

func formatExitReason(reason string) string {
	switch reason {
	case "borrow_cost_exceeded":
		return "Borrow cost exceeded"
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
