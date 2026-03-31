package gateio

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"strconv"
	"sync"
	"time"

	"arb/pkg/exchange"

	"github.com/gorilla/websocket"
)

const (
	publicWSURL      = "wss://fx-ws.gateio.ws/v4/ws/usdt"
	wsPingPeriod     = 15 * time.Second
	wsReconnectDelay = 5 * time.Second
)

// PriceWS manages the public WebSocket connection for BBO (best bid/offer) data.
type PriceWS struct {
	store           *sync.Map // symbol (Gate.io format) -> exchange.BBO
	depthStore      *sync.Map // Gate.io symbol -> *exchange.Orderbook
	conn            *websocket.Conn
	mu              sync.Mutex
	done            chan struct{}
	subs            []string             // Gate.io symbol list (BBO)
	depthSubs       map[string]bool      // Gate.io symbols with depth subscriptions
	multFunc        func(string) float64 // contractMult lookup
	metricsCallback exchange.WSMetricsCallback
}

// NewPriceWS creates a new public WebSocket manager.
func NewPriceWS(store *sync.Map, depthStore *sync.Map, multFunc func(string) float64) *PriceWS {
	return &PriceWS{
		store:      store,
		depthStore: depthStore,
		depthSubs:  make(map[string]bool),
		multFunc:   multFunc,
		done:       make(chan struct{}),
	}
}

func (ws *PriceWS) SetMetricsCallback(fn exchange.WSMetricsCallback) {
	ws.metricsCallback = fn
}

// Connect establishes the WebSocket connection and subscribes to book ticker updates.
// This method blocks and auto-reconnects on disconnect.
func (ws *PriceWS) Connect(symbols []string) {
	ws.mu.Lock()
	ws.subs = symbols
	ws.mu.Unlock()

	for {
		select {
		case <-ws.done:
			return
		default:
		}

		if err := ws.connectAndListen(); err != nil {
			log.Printf("[gateio] price ws error: %v, reconnecting in %v", err, wsReconnectDelay)
		}
		time.Sleep(wsReconnectDelay)
	}
}

func (ws *PriceWS) connectAndListen() error {
	ws.mu.Lock()
	conn, _, err := websocket.DefaultDialer.Dial(publicWSURL, nil)
	if err != nil {
		ws.mu.Unlock()
		return fmt.Errorf("dial: %w", err)
	}
	ws.conn = conn
	syms := make([]string, len(ws.subs))
	copy(syms, ws.subs)
	ws.mu.Unlock()
	if ws.metricsCallback != nil {
		ws.metricsCallback(exchange.WSEvent{Type: exchange.WSEventConnect, Timestamp: time.Now()})
	}

	defer func() {
		ws.mu.Lock()
		ws.conn = nil
		conn.Close()
		ws.mu.Unlock()
		if ws.metricsCallback != nil {
			ws.metricsCallback(exchange.WSEvent{Type: exchange.WSEventDisconnect, Timestamp: time.Now()})
		}
	}()

	// Subscribe to book ticker for all symbols
	if len(syms) > 0 {
		if err := ws.sendSubscribe(conn, syms); err != nil {
			return fmt.Errorf("subscribe: %w", err)
		}
	}

	// Re-subscribe depth channels and clear stale data
	ws.mu.Lock()
	depthSyms := make([]string, 0, len(ws.depthSubs))
	for sym := range ws.depthSubs {
		depthSyms = append(depthSyms, sym)
		ws.depthStore.Delete(sym)
	}
	ws.mu.Unlock()
	for _, sym := range depthSyms {
		ws.sendDepthSubscribe(conn, sym)
	}

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
		if ws.metricsCallback != nil {
			ws.metricsCallback(exchange.WSEvent{Type: exchange.WSEventMessage, Timestamp: time.Now()})
		}
		ws.handleMessage(message)
	}
}

func (ws *PriceWS) sendSubscribe(conn *websocket.Conn, symbols []string) error {
	msg := map[string]interface{}{
		"time":    time.Now().Unix(),
		"channel": "futures.book_ticker",
		"event":   "subscribe",
		"payload": symbols,
	}
	return conn.WriteJSON(msg)
}

func (ws *PriceWS) pingLoop(conn *websocket.Conn, done <-chan struct{}) {
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

func (ws *PriceWS) handleMessage(data []byte) {
	var msg struct {
		Channel string          `json:"channel"`
		Event   string          `json:"event"`
		Result  json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(data, &msg); err != nil {
		return
	}

	if msg.Channel == "futures.order_book" && (msg.Event == "all" || msg.Event == "update") {
		ws.handleDepthMessage(msg.Result)
		return
	}

	if msg.Channel != "futures.book_ticker" || msg.Event != "update" {
		return
	}

	// Gate.io book_ticker has both lowercase "b"/"a" (price strings) and
	// uppercase "B"/"A" (volume ints). Go's json is case-insensitive, so
	// struct tags collide. Use a raw map to extract the correct fields.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(msg.Result, &raw); err != nil {
		return
	}

	contract := ""
	if v, ok := raw["s"]; ok {
		json.Unmarshal(v, &contract)
	}
	var bid, ask float64
	if v, ok := raw["b"]; ok {
		var s string
		if json.Unmarshal(v, &s) == nil {
			fmt.Sscanf(s, "%f", &bid)
		}
	}
	if v, ok := raw["a"]; ok {
		var s string
		if json.Unmarshal(v, &s) == nil {
			fmt.Sscanf(s, "%f", &ask)
		}
	}

	if bid > 0 && ask > 0 {
		ws.store.Store(contract, exchange.BBO{
			Bid: bid,
			Ask: ask,
		})
	}
}

func (ws *PriceWS) handleDepthMessage(result json.RawMessage) {
	var depth struct {
		Contract string `json:"contract"`
		Asks     []struct {
			P string `json:"p"`
			S int64  `json:"s"`
		} `json:"asks"`
		Bids []struct {
			P string `json:"p"`
			S int64  `json:"s"`
		} `json:"bids"`
	}
	if json.Unmarshal(result, &depth) != nil || depth.Contract == "" {
		return
	}

	mult := 1.0
	if ws.multFunc != nil {
		if m := ws.multFunc(depth.Contract); m > 0 {
			mult = m
		}
	}

	parseLevels := func(raw []struct {
		P string `json:"p"`
		S int64  `json:"s"`
	}) []exchange.PriceLevel {
		levels := make([]exchange.PriceLevel, 0, len(raw))
		for _, entry := range raw {
			price, _ := strconv.ParseFloat(entry.P, 64)
			qty := math.Abs(float64(entry.S)) * mult
			levels = append(levels, exchange.PriceLevel{Price: price, Quantity: qty})
		}
		return levels
	}

	bids := parseLevels(depth.Bids)
	asks := parseLevels(depth.Asks)

	ob := &exchange.Orderbook{
		Symbol: depth.Contract,
		Bids:   bids,
		Asks:   asks,
		Time:   time.Now(),
	}
	ws.depthStore.Store(depth.Contract, ob)
}

// Subscribe adds a new symbol to the subscription list.
// Must be called after Connect has been started.
func (ws *PriceWS) Subscribe(symbol string) bool {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	// Track the symbol
	ws.subs = append(ws.subs, symbol)

	// If connected, send subscribe message
	if ws.conn != nil {
		msg := map[string]interface{}{
			"time":    time.Now().Unix(),
			"channel": "futures.book_ticker",
			"event":   "subscribe",
			"payload": []string{symbol},
		}
		if err := ws.conn.WriteJSON(msg); err != nil {
			log.Printf("[gateio] subscribe %s error: %v", symbol, err)
			return false
		}
	}
	return true
}

func (ws *PriceWS) sendDepthSubscribe(conn *websocket.Conn, symbol string) {
	msg := map[string]interface{}{
		"time":     time.Now().Unix(),
		"channel":  "futures.order_book",
		"event":    "subscribe",
		"payload":  []string{symbol},
		"accuracy": "0",
		"limit":    5,
	}
	if err := conn.WriteJSON(msg); err != nil {
		log.Printf("[gateio] depth subscribe %s error: %v", symbol, err)
	}
}

// SubscribeDepth subscribes to top-5 orderbook depth for a symbol (Gate.io format).
func (ws *PriceWS) SubscribeDepth(symbol string) bool {
	ws.mu.Lock()
	if ws.depthSubs[symbol] {
		ws.mu.Unlock()
		return false
	}
	ws.depthSubs[symbol] = true
	conn := ws.conn
	ws.mu.Unlock()

	if conn != nil {
		ws.sendDepthSubscribe(conn, symbol)
	}
	return true
}

// UnsubscribeDepth unsubscribes from top-5 orderbook depth.
func (ws *PriceWS) UnsubscribeDepth(symbol string) bool {
	ws.mu.Lock()
	if !ws.depthSubs[symbol] {
		ws.mu.Unlock()
		return false
	}
	delete(ws.depthSubs, symbol)
	conn := ws.conn
	ws.mu.Unlock()

	ws.depthStore.Delete(symbol)

	if conn != nil {
		msg := map[string]interface{}{
			"time":    time.Now().Unix(),
			"channel": "futures.order_book",
			"event":   "unsubscribe",
			"payload": []string{symbol},
		}
		ws.mu.Lock()
		_ = conn.WriteJSON(msg)
		ws.mu.Unlock()
	}
	return true
}

// Close shuts down the WebSocket connection.
func (ws *PriceWS) Close() {
	close(ws.done)
	ws.mu.Lock()
	if ws.conn != nil {
		ws.conn.Close()
	}
	ws.mu.Unlock()
}
