package gateio

import (
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"arb/pkg/exchange"
	"arb/pkg/utils"

	"github.com/gorilla/websocket"
)

var wsPrivLog = utils.NewLogger("gateio-ws-priv")

// PrivateWS manages the private WebSocket connection for order updates.
type PrivateWS struct {
	apiKey               string
	secretKey            string
	store                *sync.Map // orderID -> exchange.OrderUpdate
	onFill               *func(exchange.OrderUpdate)
	orderMetricsCallback exchange.OrderMetricsCallback
	contractMult         map[string]float64 // internalSymbol -> quanto multiplier
	conn                 *websocket.Conn
	mu                   sync.Mutex
	done                 chan struct{}
}

func (ws *PrivateWS) SetOrderMetricsCallback(fn exchange.OrderMetricsCallback) {
	ws.orderMetricsCallback = fn
}

// NewPrivateWS creates a new private WebSocket manager.
func NewPrivateWS(apiKey, secretKey string, store *sync.Map, contractMult map[string]float64, onFill *func(exchange.OrderUpdate)) *PrivateWS {
	return &PrivateWS{
		apiKey:       apiKey,
		secretKey:    secretKey,
		store:        store,
		onFill:       onFill,
		contractMult: contractMult,
		done:         make(chan struct{}),
	}
}

// wsSign generates the HMAC-SHA512 signature for WebSocket authentication.
func (ws *PrivateWS) wsSign(channel, event string, ts int64) string {
	message := fmt.Sprintf("channel=%s&event=%s&time=%d", channel, event, ts)
	mac := hmac.New(sha512.New, []byte(ws.secretKey))
	mac.Write([]byte(message))
	return hex.EncodeToString(mac.Sum(nil))
}

// Connect establishes the private WebSocket connection and subscribes to order updates.
// This method blocks and auto-reconnects on disconnect.
func (ws *PrivateWS) Connect() {
	for {
		select {
		case <-ws.done:
			return
		default:
		}

		if err := ws.connectAndListen(); err != nil {
			wsPrivLog.Error("private ws error: %v, reconnecting in %v", err, wsReconnectDelay)
		}
		time.Sleep(wsReconnectDelay)
	}
}

func (ws *PrivateWS) connectAndListen() error {
	ws.mu.Lock()
	conn, _, err := websocket.DefaultDialer.Dial(publicWSURL, nil)
	if err != nil {
		ws.mu.Unlock()
		return fmt.Errorf("dial: %w", err)
	}
	ws.conn = conn
	ws.mu.Unlock()
	wsPrivLog.Info("private ws connected to %s", publicWSURL)

	defer func() {
		ws.mu.Lock()
		ws.conn = nil
		conn.Close()
		ws.mu.Unlock()
	}()

	// Subscribe to futures.orders with authentication
	if err := ws.subscribeOrders(conn); err != nil {
		return fmt.Errorf("subscribe orders: %w", err)
	}
	wsPrivLog.Info("subscribed to futures.orders")

	// Start ping goroutine
	pingDone := make(chan struct{})
	go ws.pingLoop(conn, pingDone)
	defer close(pingDone)

	// Read messages
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("read: %w", err)
		}
		ws.handleMessage(message)
	}
}

func (ws *PrivateWS) subscribeOrders(conn *websocket.Conn) error {
	ts := time.Now().Unix()
	channel := "futures.orders"
	event := "subscribe"
	sign := ws.wsSign(channel, event, ts)

	msg := map[string]interface{}{
		"time":    ts,
		"channel": channel,
		"event":   event,
		"payload": []string{"!all"},
		"auth": map[string]interface{}{
			"method": "api_key",
			"KEY":    ws.apiKey,
			"SIGN":   sign,
		},
	}
	return conn.WriteJSON(msg)
}

func (ws *PrivateWS) pingLoop(conn *websocket.Conn, done <-chan struct{}) {
	ticker := time.NewTicker(wsPingPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			msg := map[string]interface{}{
				"time":    time.Now().Unix(),
				"channel": "futures.ping",
			}
			ws.mu.Lock()
			err := conn.WriteJSON(msg)
			ws.mu.Unlock()
			if err != nil {
				return
			}
		}
	}
}

func (ws *PrivateWS) handleMessage(data []byte) {
	var msg struct {
		Channel string `json:"channel"`
		Event   string `json:"event"`
		Error   *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(data, &msg); err != nil {
		return
	}

	// Log subscription responses and errors.
	if msg.Event == "subscribe" {
		if msg.Error != nil {
			wsPrivLog.Error("subscribe %s failed: code=%d msg=%s", msg.Channel, msg.Error.Code, msg.Error.Message)
		} else {
			wsPrivLog.Info("subscribe %s confirmed", msg.Channel)
		}
		return
	}

	if msg.Channel != "futures.orders" || msg.Event != "update" {
		return
	}

	// Gate.io sends order updates as an array
	var orders []struct {
		ID           int64       `json:"id"`
		Contract     string      `json:"contract"` // e.g. "BTC_USDT"
		Text         string      `json:"text"`
		Status       string      `json:"status"` // "open", "finished"
		Size         int64       `json:"size"`
		Left         int64       `json:"left"`
		FillPrice    json.Number `json:"fill_price"`     // Gate.io sends as number or string
		FinishAs     string      `json:"finish_as"`      // "filled", "cancelled", "ioc", etc.
		IsReduceOnly bool        `json:"is_reduce_only"` // true if reduce-only order
		IsClose      bool        `json:"is_close"`       // true if close-position order
	}
	if err := json.Unmarshal(msg.Result, &orders); err != nil {
		wsPrivLog.Error("unmarshal order update: %v", err)
		return
	}

	for _, o := range orders {
		orderID := strconv.FormatInt(o.ID, 10)

		// Compute filled volume: abs(size) - abs(left) in contracts,
		// then multiply by quanto_multiplier to get base asset units.
		totalAbs := int64(math.Abs(float64(o.Size)))
		leftAbs := int64(math.Abs(float64(o.Left)))
		filledVol := float64(totalAbs - leftAbs)

		// Apply quanto multiplier: convert contracts → base units
		internalSymbol := fromGateSymbol(o.Contract)
		if ws.contractMult != nil {
			if mult, ok := ws.contractMult[internalSymbol]; ok && mult > 0 {
				filledVol *= mult
			}
		}

		avgPrice, _ := o.FillPrice.Float64()

		// Map Gate.io status to unified status
		status := o.Status
		if o.FinishAs == "filled" || (o.FinishAs == "ioc" && filledVol > 0) {
			status = "filled"
		} else if o.FinishAs == "cancelled" {
			status = "cancelled"
		}

		clientOID := ""
		if strings.HasPrefix(o.Text, "t-") {
			clientOID = o.Text[2:]
		}

		reduceOnly := o.IsReduceOnly || o.IsClose

		wsPrivLog.Info("order update: %s sym=%s status=%s filled=%.6f avg=%.8f finishAs=%s reduceOnly=%v",
			orderID, internalSymbol, status, filledVol, avgPrice, o.FinishAs, reduceOnly)

		upd := exchange.OrderUpdate{
			OrderID:      orderID,
			ClientOID:    clientOID,
			Status:       status,
			FilledVolume: filledVol,
			AvgPrice:     avgPrice,
			Symbol:       internalSymbol,
			ReduceOnly:   reduceOnly,
		}
		ws.store.Store(orderID, upd)
		if upd.Status == "filled" && upd.FilledVolume > 0 && ws.orderMetricsCallback != nil {
			ws.orderMetricsCallback(exchange.OrderMetricEvent{
				Type:      exchange.OrderMetricFilled,
				OrderID:   orderID,
				FilledQty: upd.FilledVolume,
				Timestamp: time.Now(),
			})
		}
		if upd.Status == "filled" && upd.FilledVolume > 0 && ws.onFill != nil && *ws.onFill != nil {
			(*ws.onFill)(upd)
		}
	}
}

// Close shuts down the private WebSocket connection.
func (ws *PrivateWS) Close() {
	close(ws.done)
	ws.mu.Lock()
	if ws.conn != nil {
		ws.conn.Close()
	}
	ws.mu.Unlock()
}
