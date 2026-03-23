package bybit

import (
	"encoding/json"
	"strings"
	"sync"
	"time"

	"arb/pkg/exchange"

	"github.com/gorilla/websocket"
)

const (
	publicWSURL      = "wss://stream.bybit.com/v5/public/linear"
	wsPingPeriod     = 20 * time.Second
	wsReconnectDelay = 5 * time.Second
)

// PublicWS manages the public WebSocket connection for price streaming.
type PublicWS struct {
	conn       *websocket.Conn
	connMu     sync.Mutex
	priceStore *sync.Map
	depthStore *sync.Map
	symbols    []string
	depthSyms  map[string]bool
	done       chan struct{}
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
		"op":   "subscribe",
		"args": []string{"tickers." + symbol},
	}

	ws.connMu.Lock()
	defer ws.connMu.Unlock()
	if err := ws.conn.WriteJSON(msg); err != nil {
		log.Error("bybit public ws subscribe %s: %v", symbol, err)
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
			log.Error("bybit public ws dial: %v", err)
			time.Sleep(wsReconnectDelay)
			continue
		}

		ws.subscribeAll()
		ws.readLoop()

		// readLoop exited, reconnect.
		log.Warn("bybit public ws disconnected, reconnecting in %v", wsReconnectDelay)
		time.Sleep(wsReconnectDelay)
	}
}

func (ws *PublicWS) dial() error {
	conn, _, err := websocket.DefaultDialer.Dial(publicWSURL, nil)
	if err != nil {
		return err
	}
	ws.connMu.Lock()
	ws.conn = conn
	ws.connMu.Unlock()
	log.Info("bybit public ws connected")
	return nil
}

func (ws *PublicWS) subscribeAll() {
	args := make([]string, 0, len(ws.symbols)+len(ws.depthSyms))
	for _, s := range ws.symbols {
		args = append(args, "tickers."+s)
	}

	// Re-subscribe depth channels and clear stale data
	ws.connMu.Lock()
	for sym := range ws.depthSyms {
		args = append(args, "orderbook.50."+sym)
		ws.depthStore.Delete(sym)
	}
	ws.connMu.Unlock()

	if len(args) == 0 {
		return
	}

	msg := map[string]interface{}{
		"op":   "subscribe",
		"args": args,
	}

	ws.connMu.Lock()
	defer ws.connMu.Unlock()
	if err := ws.conn.WriteJSON(msg); err != nil {
		log.Error("bybit public ws subscribe all: %v", err)
	}
}

func (ws *PublicWS) readLoop() {
	// Start ping goroutine.
	pingDone := make(chan struct{})
	go ws.pingLoop(pingDone)
	defer close(pingDone)

	for {
		_, message, err := ws.conn.ReadMessage()
		if err != nil {
			log.Error("bybit public ws read: %v", err)
			return
		}
		ws.handleMessage(message)
	}
}

func (ws *PublicWS) pingLoop(done chan struct{}) {
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
				log.Error("bybit public ws ping: %v", err)
				return
			}
		}
	}
}

func (ws *PublicWS) handleMessage(msg []byte) {
	var base struct {
		Topic string `json:"topic"`
	}
	if json.Unmarshal(msg, &base) != nil || base.Topic == "" {
		return
	}

	if strings.HasPrefix(base.Topic, "orderbook.50.") {
		ws.handleDepthMessage(msg, base.Topic)
		return
	}

	// Ticker update (BBO)
	var tickerMsg struct {
		Data struct {
			Symbol    string `json:"symbol"`
			Bid1Price string `json:"bid1Price"`
			Ask1Price string `json:"ask1Price"`
		} `json:"data"`
	}

	if json.Unmarshal(msg, &tickerMsg) != nil || tickerMsg.Data.Symbol == "" {
		return
	}

	bid := parseFloat(tickerMsg.Data.Bid1Price)
	ask := parseFloat(tickerMsg.Data.Ask1Price)

	if bid > 0 && ask > 0 {
		ws.priceStore.Store(tickerMsg.Data.Symbol, exchange.BBO{
			Bid: bid,
			Ask: ask,
		})
	}
}

func (ws *PublicWS) handleDepthMessage(msg []byte, topic string) {
	// topic: "orderbook.50.BTCUSDT"
	symbol := strings.TrimPrefix(topic, "orderbook.50.")

	var depthMsg struct {
		Type string `json:"type"` // "snapshot" or "delta"
		Ts   int64  `json:"ts"`   // outer timestamp (milliseconds)
		Data struct {
			B  [][]string `json:"b"` // [["price","size"],...]
			A  [][]string `json:"a"`
			S  string     `json:"s"`
			U  int64      `json:"u"`  // update ID
			Ts int64      `json:"ts"` // inner timestamp (some versions)
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

	// Use outer ts; fall back to inner data.ts; fall back to now
	ts := depthMsg.Ts
	if ts == 0 {
		ts = depthMsg.Data.Ts
	}
	depthTime := time.Now().UTC()
	if ts > 0 {
		depthTime = time.UnixMilli(ts)
	}

	if depthMsg.Type == "snapshot" {
		ob := &exchange.Orderbook{
			Symbol: symbol,
			Bids:   parseLevels(depthMsg.Data.B),
			Asks:   parseLevels(depthMsg.Data.A),
			Time:   depthTime,
		}
		ws.depthStore.Store(symbol, ob)
		return
	}

	// Delta: apply changes to existing orderbook
	val, ok := ws.depthStore.Load(symbol)
	if !ok {
		return // no snapshot yet
	}
	existing := val.(*exchange.Orderbook)

	applyDelta := func(existing []exchange.PriceLevel, updates [][]string) []exchange.PriceLevel {
		// Build map of existing levels
		m := make(map[float64]float64, len(existing))
		for _, l := range existing {
			m[l.Price] = l.Quantity
		}
		// Apply updates: size=0 means remove
		for _, entry := range updates {
			if len(entry) < 2 {
				continue
			}
			price := parseFloat(entry[0])
			qty := parseFloat(entry[1])
			if qty == 0 {
				delete(m, price)
			} else {
				m[price] = qty
			}
		}
		// Rebuild sorted slice
		levels := make([]exchange.PriceLevel, 0, len(m))
		for p, q := range m {
			levels = append(levels, exchange.PriceLevel{Price: p, Quantity: q})
		}
		return levels
	}

	sortBidsDesc := func(levels []exchange.PriceLevel) {
		for i := 1; i < len(levels); i++ {
			for j := i; j > 0 && levels[j].Price > levels[j-1].Price; j-- {
				levels[j], levels[j-1] = levels[j-1], levels[j]
			}
		}
	}
	sortAsksAsc := func(levels []exchange.PriceLevel) {
		for i := 1; i < len(levels); i++ {
			for j := i; j > 0 && levels[j].Price < levels[j-1].Price; j-- {
				levels[j], levels[j-1] = levels[j-1], levels[j]
			}
		}
	}

	bids := applyDelta(existing.Bids, depthMsg.Data.B)
	sortBidsDesc(bids)
	asks := applyDelta(existing.Asks, depthMsg.Data.A)
	sortAsksAsc(asks)

	ob := &exchange.Orderbook{
		Symbol: symbol,
		Bids:   bids,
		Asks:   asks,
		Time:   depthTime,
	}
	ws.depthStore.Store(symbol, ob)
}

// SubscribeDepth subscribes to top-5 orderbook depth for a symbol.
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
			"op":   "subscribe",
			"args": []string{"orderbook.50." + symbol},
		}
		ws.connMu.Lock()
		if err := ws.conn.WriteJSON(msg); err != nil {
			log.Error("bybit public ws subscribe depth %s: %v", symbol, err)
		}
		ws.connMu.Unlock()
	}
	return true
}

// UnsubscribeDepth unsubscribes from top-5 orderbook depth for a symbol.
func (ws *PublicWS) UnsubscribeDepth(symbol string) bool {
	ws.connMu.Lock()
	if !ws.depthSyms[symbol] {
		ws.connMu.Unlock()
		return false
	}
	delete(ws.depthSyms, symbol)
	conn := ws.conn
	ws.connMu.Unlock()

	ws.depthStore.Delete(symbol)

	if conn != nil {
		msg := map[string]interface{}{
			"op":   "unsubscribe",
			"args": []string{"orderbook.50." + symbol},
		}
		ws.connMu.Lock()
		if err := ws.conn.WriteJSON(msg); err != nil {
			log.Error("bybit public ws unsubscribe depth %s: %v", symbol, err)
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
