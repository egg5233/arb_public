package bitget

import (
	"encoding/json"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"arb/pkg/exchange"

	"github.com/gorilla/websocket"
)

const (
	wsURLPublic  = "wss://ws.bitget.com/v2/ws/public"
	wsURLPrivate = "wss://ws.bitget.com/v2/ws/private"
)

// bbo holds best bid and best ask (internal to WS store).
type bbo struct {
	Bid float64
	Ask float64
}

// dataStore is a thread-safe cache for price data.
type dataStore struct {
	prices sync.Map // map[symbol string]bbo
}

// WSClient manages the public WebSocket connection for price streams.
type WSClient struct {
	stop      chan struct{}
	mu        sync.Mutex
	conn      *websocket.Conn
	symbols   map[string]bool
	depthSyms map[string]bool
	store     *dataStore
	depthStore sync.Map // symbol -> *exchange.Orderbook
}

// NewWSClient creates a new public WebSocket client.
func NewWSClient() *WSClient {
	return &WSClient{
		stop:      make(chan struct{}),
		symbols:   make(map[string]bool),
		depthSyms: make(map[string]bool),
		store:     &dataStore{},
	}
}

// Start launches the public WebSocket stream (ticker channel) with auto-reconnect.
func (ws *WSClient) Start(symbols []string) {
	ws.mu.Lock()
	for _, s := range symbols {
		ws.symbols[s] = true
	}
	ws.mu.Unlock()

	go func() {
		for {
			select {
			case <-ws.stop:
				return
			default:
				if err := ws.connectPublic(); err != nil {
					log.Printf("[WS-Public] Error: %v. Reconnecting in 5s...", err)
				} else {
					log.Printf("[WS-Public] Closed. Reconnecting in 5s...")
				}
				time.Sleep(5 * time.Second)
			}
		}
	}()
}

// SubscribeSymbol dynamically subscribes to a new symbol on the live connection.
// Returns true if newly subscribed, false if already subscribed.
func (ws *WSClient) SubscribeSymbol(symbol string) bool {
	if !strings.Contains(symbol, "USDT") {
		symbol += "USDT"
	}

	ws.mu.Lock()
	if ws.symbols[symbol] {
		ws.mu.Unlock()
		return false
	}
	ws.symbols[symbol] = true
	conn := ws.conn
	ws.mu.Unlock()

	if conn != nil {
		subMsg := map[string]interface{}{
			"op": "subscribe",
			"args": []map[string]string{{
				"instType": "USDT-FUTURES",
				"channel":  "ticker",
				"instId":   symbol,
			}},
		}
		ws.mu.Lock()
		err := conn.WriteJSON(subMsg)
		ws.mu.Unlock()
		if err != nil {
			log.Printf("[WS-Public] Failed to subscribe %s: %v", symbol, err)
		} else {
			log.Printf("[WS-Public] Dynamically subscribed to %s", symbol)
		}
	}

	return true
}

// SubscribeDepth subscribes to top-5 orderbook depth for a symbol.
func (ws *WSClient) SubscribeDepth(symbol string) bool {
	if !strings.Contains(symbol, "USDT") {
		symbol += "USDT"
	}

	ws.mu.Lock()
	if ws.depthSyms[symbol] {
		ws.mu.Unlock()
		return false
	}
	ws.depthSyms[symbol] = true
	conn := ws.conn
	ws.mu.Unlock()

	if conn != nil {
		subMsg := map[string]interface{}{
			"op": "subscribe",
			"args": []map[string]string{{
				"instType": "USDT-FUTURES",
				"channel":  "books15",
				"instId":   symbol,
			}},
		}
		ws.mu.Lock()
		_ = conn.WriteJSON(subMsg)
		ws.mu.Unlock()
	}
	return true
}

// UnsubscribeDepth unsubscribes from top-5 orderbook depth for a symbol.
func (ws *WSClient) UnsubscribeDepth(symbol string) bool {
	if !strings.Contains(symbol, "USDT") {
		symbol += "USDT"
	}

	ws.mu.Lock()
	if !ws.depthSyms[symbol] {
		ws.mu.Unlock()
		return false
	}
	delete(ws.depthSyms, symbol)
	conn := ws.conn
	ws.mu.Unlock()

	ws.depthStore.Delete(symbol)

	if conn != nil {
		unsubMsg := map[string]interface{}{
			"op": "unsubscribe",
			"args": []map[string]string{{
				"instType": "USDT-FUTURES",
				"channel":  "books15",
				"instId":   symbol,
			}},
		}
		ws.mu.Lock()
		_ = conn.WriteJSON(unsubMsg)
		ws.mu.Unlock()
	}
	return true
}

// Stop shuts down the public WebSocket client.
func (ws *WSClient) Stop() {
	close(ws.stop)
}

func (ws *WSClient) connectPublic() error {
	log.Println("[WS-Public] Connecting...")
	c, _, err := websocket.DefaultDialer.Dial(wsURLPublic, nil)
	if err != nil {
		return err
	}
	defer func() {
		ws.mu.Lock()
		ws.conn = nil
		ws.mu.Unlock()
		c.Close()
	}()

	ws.mu.Lock()
	ws.conn = c
	symbols := make([]string, 0, len(ws.symbols))
	for s := range ws.symbols {
		symbols = append(symbols, s)
	}
	ws.mu.Unlock()

	// Ping loop
	go keepAlive(c, "Public", ws.stop)

	// Subscribe to ticker for all symbols
	args := make([]map[string]string, 0, len(symbols)+len(ws.depthSyms))
	for _, s := range symbols {
		args = append(args, map[string]string{
			"instType": "USDT-FUTURES",
			"channel":  "ticker",
			"instId":   s,
		})
	}
	// Re-subscribe depth channels and clear stale data
	ws.mu.Lock()
	for sym := range ws.depthSyms {
		args = append(args, map[string]string{
			"instType": "USDT-FUTURES",
			"channel":  "books15",
			"instId":   sym,
		})
		ws.depthStore.Delete(sym)
	}
	ws.mu.Unlock()

	subMsg := map[string]interface{}{"op": "subscribe", "args": args}
	ws.mu.Lock()
	err = c.WriteJSON(subMsg)
	ws.mu.Unlock()
	if err != nil {
		return err
	}
	log.Printf("[WS-Public] Subscribed to %d channels.", len(args))

	// Read loop
	for {
		_, message, err := c.ReadMessage()
		if err != nil {
			return err
		}
		if string(message) == "pong" {
			continue
		}

		// Determine channel from the arg field
		var argPeek struct {
			Arg struct {
				Channel string `json:"channel"`
				InstId  string `json:"instId"`
			} `json:"arg"`
			Action string `json:"action"`
		}
		if json.Unmarshal(message, &argPeek) != nil {
			continue
		}

		if argPeek.Arg.Channel == "books15" && (argPeek.Action == "snapshot" || argPeek.Action == "update") {
			ws.handleBooks5Message(message)
			continue
		}

		var event struct {
			Action string `json:"action"`
			Data   []struct {
				InstId  string `json:"instId"`
				BestBid string `json:"bidPr"`
				BestAsk string `json:"askPr"`
			} `json:"data"`
		}

		if err := json.Unmarshal(message, &event); err != nil {
			continue
		}

		if event.Action == "snapshot" || event.Action == "update" {
			for _, d := range event.Data {
				bid, _ := strconv.ParseFloat(d.BestBid, 64)
				ask, _ := strconv.ParseFloat(d.BestAsk, 64)
				if bid > 0 && ask > 0 {
					ws.store.prices.Store(d.InstId, bbo{Bid: bid, Ask: ask})
				}
			}
		}
	}
}

func (ws *WSClient) handleBooks5Message(message []byte) {
	var msg struct {
		Arg struct {
			InstId string `json:"instId"`
		} `json:"arg"`
		Data []struct {
			Asks [][]string `json:"asks"` // [["price","size"],...]
			Bids [][]string `json:"bids"`
			Ts   string     `json:"ts"`
		} `json:"data"`
	}
	if json.Unmarshal(message, &msg) != nil || len(msg.Data) == 0 {
		return
	}

	d := msg.Data[0]
	parseLevels := func(raw [][]string) []exchange.PriceLevel {
		levels := make([]exchange.PriceLevel, 0, len(raw))
		for _, entry := range raw {
			if len(entry) < 2 {
				continue
			}
			price, _ := strconv.ParseFloat(entry[0], 64)
			qty, _ := strconv.ParseFloat(entry[1], 64)
			levels = append(levels, exchange.PriceLevel{Price: price, Quantity: qty})
		}
		return levels
	}

	ts, _ := strconv.ParseInt(d.Ts, 10, 64)
	ob := &exchange.Orderbook{
		Symbol: msg.Arg.InstId,
		Bids:   parseLevels(d.Bids),
		Asks:   parseLevels(d.Asks),
		Time:   time.UnixMilli(ts),
	}
	ws.depthStore.Store(msg.Arg.InstId, ob)
}

// keepAlive sends periodic "ping" messages to keep the WebSocket alive.
func keepAlive(c *websocket.Conn, tag string, stop chan struct{}) {
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			if err := c.WriteMessage(websocket.TextMessage, []byte("ping")); err != nil {
				log.Printf("[WS-%s] Ping Failed: %v", tag, err)
				return
			}
		}
	}
}
