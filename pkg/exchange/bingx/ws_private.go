package bingx

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"arb/pkg/exchange"

	"github.com/gorilla/websocket"
)

const (
	privateWSURL       = "wss://open-api-swap.bingx.com/swap-market"
	listenKeyExtendInt = 30 * time.Minute
	privateWSPingInt   = 10 * time.Minute
)

// PrivateWS manages the private WebSocket connection for order updates.
type PrivateWS struct {
	client               *Client
	conn                 *websocket.Conn
	connMu               sync.Mutex
	orderStore           *sync.Map
	onFill               *func(exchange.OrderUpdate)
	normalize            func(symbol string, qty float64, price float64) (string, float64, float64)
	orderMetricsCallback exchange.OrderMetricsCallback
	listenKey            string
	done                 chan struct{}
}

// NewPrivateWS creates a new private WebSocket handler.
func NewPrivateWS(client *Client, orderStore *sync.Map, onFill *func(exchange.OrderUpdate), normalize ...func(symbol string, qty float64, price float64) (string, float64, float64)) *PrivateWS {
	ws := &PrivateWS{
		client:     client,
		orderStore: orderStore,
		onFill:     onFill,
		done:       make(chan struct{}),
	}
	if len(normalize) > 0 {
		ws.normalize = normalize[0]
	}
	return ws
}

func (ws *PrivateWS) SetOrderMetricsCallback(fn exchange.OrderMetricsCallback) {
	ws.orderMetricsCallback = fn
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

		// Create listen key via REST
		listenKey, err := ws.createListenKey()
		if err != nil {
			log.Error("bingx private ws create listen key: %v", err)
			time.Sleep(wsReconnectDelay)
			continue
		}
		ws.listenKey = listenKey

		if err := ws.dial(); err != nil {
			log.Error("bingx private ws dial: %v", err)
			time.Sleep(wsReconnectDelay)
			continue
		}

		// Start listen key keepalive and ping loop
		extendDone := make(chan struct{})
		go ws.extendListenKeyLoop(extendDone)
		go ws.pingLoop(extendDone)

		ws.readLoop()

		close(extendDone)
		ws.deleteListenKey()

		log.Warn("bingx private ws disconnected, reconnecting in %v", wsReconnectDelay)
		time.Sleep(wsReconnectDelay)
	}
}

func (ws *PrivateWS) createListenKey() (string, error) {
	// Listen key endpoint returns {"listenKey":"..."} at top level,
	// not wrapped in the standard {"code":0,"data":...} format.
	// Use DoRequestRaw to get the full body.
	body, err := ws.client.DoRequestRaw("POST", "/openApi/user/auth/userDataStream", map[string]string{})
	if err != nil {
		return "", err
	}

	var resp struct {
		ListenKey string `json:"listenKey"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", err
	}
	if resp.ListenKey == "" {
		return "", fmt.Errorf("empty listen key in response: %s", string(body))
	}
	return resp.ListenKey, nil
}

func (ws *PrivateWS) extendListenKey() error {
	params := map[string]string{
		"listenKey": ws.listenKey,
	}
	_, err := ws.client.DoRequestRaw("PUT", "/openApi/user/auth/userDataStream", params)
	return err
}

func (ws *PrivateWS) deleteListenKey() {
	if ws.listenKey == "" {
		return
	}
	params := map[string]string{
		"listenKey": ws.listenKey,
	}
	ws.client.DoRequestRaw("DELETE", "/openApi/user/auth/userDataStream", params)
}

func (ws *PrivateWS) extendListenKeyLoop(done chan struct{}) {
	ticker := time.NewTicker(listenKeyExtendInt)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case <-ws.done:
			return
		case <-ticker.C:
			if err := ws.extendListenKey(); err != nil {
				log.Error("bingx private ws extend listen key: %v", err)
			}
		}
	}
}

func (ws *PrivateWS) pingLoop(done chan struct{}) {
	ticker := time.NewTicker(privateWSPingInt)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case <-ws.done:
			return
		case <-ticker.C:
			ws.connMu.Lock()
			if ws.conn != nil {
				err := ws.conn.WriteMessage(websocket.TextMessage, []byte("Pong"))
				if err != nil {
					log.Error("bingx private ws ping: %v", err)
				}
			}
			ws.connMu.Unlock()
		}
	}
}

func (ws *PrivateWS) dial() error {
	wsURL := privateWSURL + "?listenKey=" + ws.listenKey
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return err
	}
	ws.connMu.Lock()
	if ws.conn != nil {
		ws.conn.Close()
	}
	ws.conn = conn
	ws.connMu.Unlock()
	log.Info("bingx private ws connected")
	return nil
}

func (ws *PrivateWS) readLoop() {
	for {
		_, message, err := ws.conn.ReadMessage()
		if err != nil {
			log.Error("bingx private ws read: %v", err)
			return
		}

		// BingX messages are gzip compressed
		decoded, err := gzipDecompress(message)
		if err != nil {
			decoded = message
		}

		// Handle Ping/Pong
		if string(decoded) == "Ping" {
			ws.connMu.Lock()
			err := ws.conn.WriteMessage(websocket.TextMessage, []byte("Pong"))
			ws.connMu.Unlock()
			if err != nil {
				log.Error("bingx private ws pong: %v", err)
				return
			}
			continue
		}

		ws.handleMessage(decoded)
	}
}

func (ws *PrivateWS) handleMessage(msg []byte) {
	var base struct {
		Event string `json:"e"`
	}
	if json.Unmarshal(msg, &base) != nil {
		return
	}

	if base.Event != "ORDER_TRADE_UPDATE" {
		return
	}

	var orderMsg struct {
		Order struct {
			Symbol        string `json:"s"`  // BTC-USDT
			Side          string `json:"S"`  // BUY or SELL
			Status        string `json:"X"`  // FILLED, PARTIALLY_FILLED, etc.
			OrderID       string `json:"i"`  // order ID
			ClientOrderID string `json:"c"`  // client order ID
			Quantity      string `json:"q"`  // original qty
			AvgPrice      string `json:"ap"` // average fill price
			FilledQty     string `json:"z"`  // cumulative filled qty
			ReduceOnly    bool   `json:"ro"` // reduce-only flag
			OrderType     string `json:"o"`  // order type (e.g. STOP_MARKET)
		} `json:"o"`
	}
	if json.Unmarshal(msg, &orderMsg) != nil {
		return
	}

	o := orderMsg.Order
	filledQty, _ := strconv.ParseFloat(o.FilledQty, 64)
	avgPrice, _ := strconv.ParseFloat(o.AvgPrice, 64)

	// Convert BingX symbol format "BTC-USDT" → "BTCUSDT"
	symbol := strings.ReplaceAll(o.Symbol, "-", "")
	if ws.normalize != nil {
		symbol, filledQty, avgPrice = ws.normalize(symbol, filledQty, avgPrice)
	}

	// reduceOnly: explicit flag or inferred from STOP_MARKET / TAKE_PROFIT_MARKET order type
	reduceOnly := o.ReduceOnly || o.OrderType == "STOP_MARKET" || o.OrderType == "TAKE_PROFIT_MARKET"

	update := exchange.OrderUpdate{
		OrderID:      o.OrderID,
		ClientOID:    o.ClientOrderID,
		Status:       normalizeBingXWSStatus(o.Status),
		FilledVolume: filledQty,
		AvgPrice:     avgPrice,
		Symbol:       symbol,
		ReduceOnly:   reduceOnly,
	}

	ws.orderStore.Store(o.OrderID, update)
	log.Info("order update: %s %s status=%s filled=%.6f avg=%.8f reduceOnly=%v symbol=%s",
		o.Symbol, o.OrderID, update.Status, filledQty, avgPrice, reduceOnly, symbol)
	if update.Status == "filled" && update.FilledVolume > 0 && ws.orderMetricsCallback != nil {
		ws.orderMetricsCallback(exchange.OrderMetricEvent{
			Type:      exchange.OrderMetricFilled,
			OrderID:   o.OrderID,
			FilledQty: update.FilledVolume,
			Timestamp: time.Now(),
		})
	}
	if update.Status == "filled" && update.FilledVolume > 0 && ws.onFill != nil && *ws.onFill != nil {
		(*ws.onFill)(update)
	}
}

// normalizeBingXWSStatus converts BingX WS order status to lowercase standard.
func normalizeBingXWSStatus(status string) string {
	switch status {
	case "NEW", "Pending":
		return "new"
	case "PARTIALLY_FILLED", "PartiallyFilled":
		return "partially_filled"
	case "FILLED", "Filled":
		return "filled"
	case "CANCELED", "Cancelled":
		return "cancelled"
	case "FAILED", "Failed":
		return "failed"
	default:
		return strings.ToLower(status)
	}
}

// Close closes the private WebSocket connection.
func (ws *PrivateWS) Close() {
	close(ws.done)
	ws.deleteListenKey()
	ws.connMu.Lock()
	defer ws.connMu.Unlock()
	if ws.conn != nil {
		ws.conn.Close()
	}
}
