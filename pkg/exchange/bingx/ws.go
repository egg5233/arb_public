package bingx

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	"arb/pkg/exchange"

	"github.com/gorilla/websocket"
)

const (
	publicWSURL      = "wss://open-api-swap.bingx.com/swap-market"
	wsPingPeriod     = 20 * time.Second
	wsReconnectDelay = 5 * time.Second
)

// PublicWS manages the public WebSocket connection for price streaming.
type PublicWS struct {
	conn            *websocket.Conn
	connMu          sync.Mutex
	priceStore      *sync.Map
	depthStore      *sync.Map
	symbols         []string
	depthSyms       map[string]bool
	done            chan struct{}
	metricsCallback exchange.WSMetricsCallback
}

// NewPublicWS creates a new public WebSocket handler.
func NewPublicWS(priceStore *sync.Map, depthStore *sync.Map) *PublicWS {
	return &PublicWS{
		priceStore: priceStore,
		depthStore: depthStore,
		depthSyms:  make(map[string]bool),
		done:       make(chan struct{}),
	}
}

func (ws *PublicWS) SetMetricsCallback(fn exchange.WSMetricsCallback) {
	ws.metricsCallback = fn
}

// Connect establishes the WebSocket connection and subscribes to symbols.
func (ws *PublicWS) Connect(symbols []string) {
	ws.symbols = symbols
	go ws.connectLoop()
}

// Subscribe adds a new symbol subscription to the public WebSocket.
func (ws *PublicWS) Subscribe(symbol string) bool {
	ws.connMu.Lock()
	conn := ws.conn
	ws.connMu.Unlock()

	if conn == nil {
		return false
	}

	ws.symbols = append(ws.symbols, symbol)

	msg := map[string]interface{}{
		"id":       generateUUID(),
		"reqType":  "sub",
		"dataType": symbol + "@bookTicker",
	}

	ws.connMu.Lock()
	defer ws.connMu.Unlock()
	if err := ws.conn.WriteJSON(msg); err != nil {
		log.Error("bingx public ws subscribe %s: %v", symbol, err)
		return false
	}
	return true
}

func (ws *PublicWS) connectLoop() {
	for {
		select {
		case <-ws.done:
			return
		default:
		}

		if err := ws.dial(); err != nil {
			log.Error("bingx public ws dial: %v", err)
			time.Sleep(wsReconnectDelay)
			continue
		}

		ws.subscribeAll()
		ws.readLoop()

		// readLoop exited, reconnect.
		log.Warn("bingx public ws disconnected, reconnecting in %v", wsReconnectDelay)
		time.Sleep(wsReconnectDelay)
	}
}

func (ws *PublicWS) dial() error {
	conn, _, err := websocket.DefaultDialer.Dial(publicWSURL, nil)
	if err != nil {
		return err
	}
	ws.connMu.Lock()
	if ws.conn != nil {
		ws.conn.Close()
	}
	ws.conn = conn
	ws.connMu.Unlock()
	if ws.metricsCallback != nil {
		ws.metricsCallback(exchange.WSEvent{Type: exchange.WSEventConnect, Timestamp: time.Now()})
	}
	log.Info("bingx public ws connected")
	return nil
}

func (ws *PublicWS) subscribeAll() {
	// Subscribe to BBO for each symbol
	for _, sym := range ws.symbols {
		msg := map[string]interface{}{
			"id":       generateUUID(),
			"reqType":  "sub",
			"dataType": sym + "@bookTicker",
		}
		ws.connMu.Lock()
		if err := ws.conn.WriteJSON(msg); err != nil {
			log.Error("bingx public ws subscribe BBO %s: %v", sym, err)
		}
		ws.connMu.Unlock()
	}

	// Re-subscribe depth channels
	ws.connMu.Lock()
	depthSymsCopy := make([]string, 0, len(ws.depthSyms))
	for sym := range ws.depthSyms {
		depthSymsCopy = append(depthSymsCopy, sym)
		// Delete stale depth using internal symbol format (BTCUSDT, not BTC-USDT)
		ws.depthStore.Delete(fromBingXSymbol(sym))
	}
	ws.connMu.Unlock()

	for _, sym := range depthSymsCopy {
		msg := map[string]interface{}{
			"id":       generateUUID(),
			"reqType":  "sub",
			"dataType": sym + "@depth20@500ms",
		}
		ws.connMu.Lock()
		if err := ws.conn.WriteJSON(msg); err != nil {
			log.Error("bingx public ws subscribe depth %s: %v", sym, err)
		}
		ws.connMu.Unlock()
	}
}

func (ws *PublicWS) readLoop() {
	for {
		_, message, err := ws.conn.ReadMessage()
		if err != nil {
			log.Error("bingx public ws read: %v", err)
			if ws.metricsCallback != nil {
				ws.metricsCallback(exchange.WSEvent{Type: exchange.WSEventDisconnect, Timestamp: time.Now()})
			}
			return
		}

		// BingX messages are gzip compressed
		decoded, err := gzipDecompress(message)
		if err != nil {
			// May be plain text
			decoded = message
		}

		// Handle Ping/Pong: BingX sends text "Ping", we reply "Pong"
		if string(decoded) == "Ping" {
			ws.connMu.Lock()
			err := ws.conn.WriteMessage(websocket.TextMessage, []byte("Pong"))
			ws.connMu.Unlock()
			if err != nil {
				log.Error("bingx public ws pong: %v", err)
				return
			}
			continue
		}

		if ws.metricsCallback != nil {
			ws.metricsCallback(exchange.WSEvent{Type: exchange.WSEventMessage, Timestamp: time.Now()})
		}
		ws.handleMessage(decoded)
	}
}

func (ws *PublicWS) handleMessage(msg []byte) {
	var base struct {
		DataType string `json:"dataType"`
	}
	if json.Unmarshal(msg, &base) != nil || base.DataType == "" {
		return
	}

	if strings.Contains(base.DataType, "@depth") {
		ws.handleDepthMessage(msg, base.DataType)
		return
	}

	if strings.Contains(base.DataType, "@bookTicker") {
		ws.handleBookTickerMessage(msg, base.DataType)
		return
	}
}

func (ws *PublicWS) handleBookTickerMessage(msg []byte, dataType string) {
	// dataType: "BTC-USDT@bookTicker"
	var tickerMsg struct {
		Data struct {
			Symbol   string `json:"s"`
			BidPrice string `json:"b"`
			AskPrice string `json:"a"`
		} `json:"data"`
	}
	if json.Unmarshal(msg, &tickerMsg) != nil {
		return
	}

	bid := parseFloat(tickerMsg.Data.BidPrice)
	ask := parseFloat(tickerMsg.Data.AskPrice)

	if bid > 0 && ask > 0 {
		// Store with internal symbol format (BTCUSDT)
		internalSymbol := fromBingXSymbol(tickerMsg.Data.Symbol)
		ws.priceStore.Store(internalSymbol, exchange.BBO{
			Bid: bid,
			Ask: ask,
		})
	}
}

func (ws *PublicWS) handleDepthMessage(msg []byte, dataType string) {
	// dataType: "BTC-USDT@depth20@500ms"
	parts := strings.Split(dataType, "@")
	if len(parts) < 2 {
		return
	}
	bingxSymbol := parts[0]
	internalSymbol := fromBingXSymbol(bingxSymbol)

	var depthMsg struct {
		Data struct {
			Bids [][]string `json:"bids"`
			Asks [][]string `json:"asks"`
		} `json:"data"`
	}
	if json.Unmarshal(msg, &depthMsg) != nil {
		return
	}

	parseLevels := func(raw [][]string) []exchange.PriceLevel {
		levels := make([]exchange.PriceLevel, 0, len(raw))
		for _, entry := range raw {
			if len(entry) < 2 {
				continue
			}
			price := parseFloat(entry[0])
			qty := parseFloat(entry[1])
			levels = append(levels, exchange.PriceLevel{Price: price, Quantity: qty})
		}
		return levels
	}

	ob := &exchange.Orderbook{
		Symbol: internalSymbol,
		Bids:   parseLevels(depthMsg.Data.Bids),
		Asks:   parseLevels(depthMsg.Data.Asks),
		Time:   time.Now().UTC(),
	}
	ws.depthStore.Store(internalSymbol, ob)
}

// SubscribeDepth subscribes to orderbook depth for a symbol.
func (ws *PublicWS) SubscribeDepth(symbol string) bool {
	ws.connMu.Lock()
	if ws.depthSyms[symbol] {
		ws.connMu.Unlock()
		return false
	}
	ws.depthSyms[symbol] = true
	conn := ws.conn
	ws.connMu.Unlock()

	if conn != nil {
		msg := map[string]interface{}{
			"id":       generateUUID(),
			"reqType":  "sub",
			"dataType": symbol + "@depth20@500ms",
		}
		ws.connMu.Lock()
		if err := ws.conn.WriteJSON(msg); err != nil {
			log.Error("bingx public ws subscribe depth %s: %v", symbol, err)
		}
		ws.connMu.Unlock()
	}
	return true
}

// UnsubscribeDepth unsubscribes from orderbook depth for a symbol.
func (ws *PublicWS) UnsubscribeDepth(symbol string) bool {
	ws.connMu.Lock()
	if !ws.depthSyms[symbol] {
		ws.connMu.Unlock()
		return false
	}
	delete(ws.depthSyms, symbol)
	conn := ws.conn
	ws.connMu.Unlock()

	internalSymbol := fromBingXSymbol(symbol)
	ws.depthStore.Delete(internalSymbol)

	if conn != nil {
		msg := map[string]interface{}{
			"id":       generateUUID(),
			"reqType":  "unsub",
			"dataType": symbol + "@depth20@500ms",
		}
		ws.connMu.Lock()
		if err := ws.conn.WriteJSON(msg); err != nil {
			log.Error("bingx public ws unsubscribe depth %s: %v", symbol, err)
		}
		ws.connMu.Unlock()
	}
	return true
}

// Close closes the WebSocket connection.
func (ws *PublicWS) Close() {
	close(ws.done)
	ws.connMu.Lock()
	defer ws.connMu.Unlock()
	if ws.conn != nil {
		ws.conn.Close()
	}
}

// ---------- Helpers ----------

// gzipDecompress decompresses gzip-encoded data.
func gzipDecompress(data []byte) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(reader)
}

// parseFloat is a helper to parse float strings.
func parseFloat(s string) float64 {
	v, _ := strconv.ParseFloat(s, 64)
	return v
}
