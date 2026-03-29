package bybit

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	"arb/pkg/exchange"

	"github.com/gorilla/websocket"
)

const (
	privateWSURL = "wss://stream.bybit.com/v5/private"
)

// PrivateWS manages the private WebSocket connection for order updates.
type PrivateWS struct {
	apiKey     string
	secretKey  string
	conn       *websocket.Conn
	connMu     sync.Mutex
	orderStore *sync.Map
	onFill     *func(exchange.OrderUpdate)
	done       chan struct{}
}

// NewPrivateWS creates a new private WebSocket handler.
func NewPrivateWS(apiKey, secretKey string, orderStore *sync.Map, onFill *func(exchange.OrderUpdate)) *PrivateWS {
	return &PrivateWS{
		apiKey:     apiKey,
		secretKey:  secretKey,
		orderStore: orderStore,
		onFill:     onFill,
		done:       make(chan struct{}),
	}
}

// Connect establishes the private WebSocket connection.
func (ws *PrivateWS) Connect() {
	go ws.connectLoop()
}

func (ws *PrivateWS) connectLoop() {
	for {
		select {
		case <-ws.done:
			return
		default:
		}

		if err := ws.dial(); err != nil {
			log.Error("bybit private ws dial: %v", err)
			time.Sleep(wsReconnectDelay)
			continue
		}

		if err := ws.authenticate(); err != nil {
			log.Error("bybit private ws auth: %v", err)
			ws.connMu.Lock()
			ws.conn.Close()
			ws.connMu.Unlock()
			time.Sleep(wsReconnectDelay)
			continue
		}

		ws.subscribeOrders()
		ws.readLoop()

		log.Warn("bybit private ws disconnected, reconnecting in %v", wsReconnectDelay)
		time.Sleep(wsReconnectDelay)
	}
}

func (ws *PrivateWS) dial() error {
	conn, _, err := websocket.DefaultDialer.Dial(privateWSURL, nil)
	if err != nil {
		return err
	}
	ws.connMu.Lock()
	ws.conn = conn
	ws.connMu.Unlock()
	log.Info("bybit private ws connected")
	return nil
}

func (ws *PrivateWS) authenticate() error {
	// Expires is timestamp in milliseconds, 10 seconds from now.
	expires := time.Now().UnixMilli() + 10000
	expiresStr := fmt.Sprintf("%d", expires)

	// Signature: HMAC-SHA256("GET/realtime" + expires, secretKey)
	message := "GET/realtime" + expiresStr
	mac := hmac.New(sha256.New, []byte(ws.secretKey))
	mac.Write([]byte(message))
	signature := hex.EncodeToString(mac.Sum(nil))

	authMsg := map[string]interface{}{
		"op":   "auth",
		"args": []interface{}{ws.apiKey, expires, signature},
	}

	ws.connMu.Lock()
	err := ws.conn.WriteJSON(authMsg)
	ws.connMu.Unlock()
	if err != nil {
		return fmt.Errorf("write auth: %w", err)
	}

	// Read auth response.
	_, msg, err := ws.conn.ReadMessage()
	if err != nil {
		return fmt.Errorf("read auth response: %w", err)
	}

	var resp struct {
		Op      string `json:"op"`
		Success bool   `json:"success"`
		RetMsg  string `json:"ret_msg"`
	}
	if err := json.Unmarshal(msg, &resp); err != nil {
		return fmt.Errorf("parse auth response: %w", err)
	}
	if !resp.Success {
		return fmt.Errorf("auth failed: %s", resp.RetMsg)
	}

	log.Info("bybit private ws authenticated")
	return nil
}

func (ws *PrivateWS) subscribeOrders() {
	msg := map[string]interface{}{
		"op":   "subscribe",
		"args": []string{"order"},
	}

	ws.connMu.Lock()
	defer ws.connMu.Unlock()
	if err := ws.conn.WriteJSON(msg); err != nil {
		log.Error("bybit private ws subscribe orders: %v", err)
	}
}

func (ws *PrivateWS) readLoop() {
	// Start ping goroutine.
	pingDone := make(chan struct{})
	go ws.pingLoop(pingDone)
	defer close(pingDone)

	for {
		_, message, err := ws.conn.ReadMessage()
		if err != nil {
			log.Error("bybit private ws read: %v", err)
			return
		}
		ws.handleMessage(message)
	}
}

func (ws *PrivateWS) pingLoop(done chan struct{}) {
	ticker := time.NewTicker(wsPingPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			ws.connMu.Lock()
			err := ws.conn.WriteJSON(map[string]string{"op": "ping"})
			ws.connMu.Unlock()
			if err != nil {
				log.Error("bybit private ws ping: %v", err)
				return
			}
		}
	}
}

func (ws *PrivateWS) handleMessage(msg []byte) {
	var orderMsg struct {
		Topic string `json:"topic"`
		Data  []struct {
			Symbol      string `json:"symbol"`
			OrderID     string `json:"orderId"`
			OrderLinkID string `json:"orderLinkId"`
			OrderStatus string `json:"orderStatus"`
			CumExecQty  string `json:"cumExecQty"`
			AvgPrice    string `json:"avgPrice"`
			ReduceOnly  bool   `json:"reduceOnly"`
		} `json:"data"`
	}

	if err := json.Unmarshal(msg, &orderMsg); err != nil {
		return
	}

	if orderMsg.Topic != "order" {
		return
	}

	for _, o := range orderMsg.Data {
		filledQty, _ := strconv.ParseFloat(o.CumExecQty, 64)
		avgPrice, _ := strconv.ParseFloat(o.AvgPrice, 64)

		update := exchange.OrderUpdate{
			OrderID:      o.OrderID,
			ClientOID:    o.OrderLinkID,
			Status:       normalizeOrderStatus(o.OrderStatus),
			FilledVolume: filledQty,
			AvgPrice:     avgPrice,
			Symbol:       o.Symbol,
			ReduceOnly:   o.ReduceOnly,
		}

		ws.orderStore.Store(o.OrderID, update)
		log.Info("order update: %s %s status=%s filled=%.6f avg=%.8f reduceOnly=%v",
			o.Symbol, o.OrderID, update.Status, filledQty, avgPrice, o.ReduceOnly)
		if update.Status == "filled" && update.FilledVolume > 0 && ws.onFill != nil && *ws.onFill != nil {
			(*ws.onFill)(update)
		}
	}
}

// normalizeOrderStatus converts Bybit order status to lowercase.
func normalizeOrderStatus(status string) string {
	switch status {
	case "New":
		return "new"
	case "PartiallyFilled":
		return "partially_filled"
	case "Filled":
		return "filled"
	case "Cancelled":
		return "cancelled"
	case "Rejected":
		return "rejected"
	case "Deactivated":
		return "deactivated"
	default:
		return status
	}
}

// Close closes the private WebSocket connection.
func (ws *PrivateWS) Close() {
	close(ws.done)
	ws.connMu.Lock()
	defer ws.connMu.Unlock()
	if ws.conn != nil {
		ws.conn.Close()
	}
}

// parseFloat is a helper to parse float strings.
func parseFloat(s string) float64 {
	v, _ := strconv.ParseFloat(s, 64)
	return v
}
