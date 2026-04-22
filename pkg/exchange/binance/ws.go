package binance

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"arb/pkg/exchange"

	"github.com/gorilla/websocket"
)

// ---------------------------------------------------------------------------
// WebSocket: Price Stream (bookTicker)
// ---------------------------------------------------------------------------

func (b *Adapter) StartPriceStream(symbols []string) {
	b.priceMu.Lock()
	resolved := make([]string, 0, len(symbols))
	for _, s := range symbols {
		real, _, err := b.resolveSymbol(s)
		if err != nil {
			continue
		}
		b.priceSyms[real] = true
		resolved = append(resolved, real)
	}
	b.priceMu.Unlock()

	go b.runPriceStream(resolved)
}

func (b *Adapter) SubscribeSymbol(symbol string) bool {
	realSymbol, _, err := b.resolveSymbol(symbol)
	if err != nil {
		return false
	}
	b.priceMu.Lock()
	if b.priceSyms[realSymbol] {
		b.priceMu.Unlock()
		return false
	}
	b.priceSyms[realSymbol] = true
	b.priceMu.Unlock()

	// If the WS connection exists, send a subscribe message
	b.priceMu.Lock()
	conn := b.priceConn
	b.priceMu.Unlock()

	if conn != nil {
		stream := strings.ToLower(realSymbol) + "@bookTicker"
		msg := map[string]interface{}{
			"method": "SUBSCRIBE",
			"params": []string{stream},
			"id":     time.Now().UnixMilli(),
		}
		data, _ := json.Marshal(msg)
		_ = conn.WriteMessage(websocket.TextMessage, data)
	}
	return true
}

func (b *Adapter) GetBBO(symbol string) (exchange.BBO, bool) {
	realSymbol, mult, err := b.resolveSymbol(symbol)
	if err != nil {
		return exchange.BBO{}, false
	}
	val, ok := b.priceStore.Load(realSymbol)
	if !ok {
		return exchange.BBO{}, false
	}
	bbo := val.(exchange.BBO)
	return exchange.BBO{
		Bid: exchange.ScalePriceFromContracts(bbo.Bid, mult),
		Ask: exchange.ScalePriceFromContracts(bbo.Ask, mult),
	}, true
}

func (b *Adapter) GetPriceStore() *sync.Map {
	return &b.priceStore
}

// ---------------------------------------------------------------------------
// WebSocket: Depth Stream (top-5 orderbook)
// ---------------------------------------------------------------------------

func (b *Adapter) SubscribeDepth(symbol string) bool {
	realSymbol, _, err := b.resolveSymbol(symbol)
	if err != nil {
		return false
	}
	b.priceMu.Lock()
	if b.depthSyms[realSymbol] {
		b.priceMu.Unlock()
		return false
	}
	b.depthSyms[realSymbol] = true
	conn := b.priceConn
	b.priceMu.Unlock()

	if conn != nil {
		stream := strings.ToLower(realSymbol) + "@depth20@100ms"
		msg := map[string]interface{}{
			"method": "SUBSCRIBE",
			"params": []string{stream},
			"id":     time.Now().UnixMilli(),
		}
		data, _ := json.Marshal(msg)
		_ = conn.WriteMessage(websocket.TextMessage, data)
	}
	return true
}

func (b *Adapter) UnsubscribeDepth(symbol string) bool {
	realSymbol, _, err := b.resolveSymbol(symbol)
	if err != nil {
		return false
	}
	b.priceMu.Lock()
	if !b.depthSyms[realSymbol] {
		b.priceMu.Unlock()
		return false
	}
	delete(b.depthSyms, realSymbol)
	conn := b.priceConn
	b.priceMu.Unlock()

	b.depthStore.Delete(realSymbol)

	if conn != nil {
		stream := strings.ToLower(realSymbol) + "@depth20@100ms"
		msg := map[string]interface{}{
			"method": "UNSUBSCRIBE",
			"params": []string{stream},
			"id":     time.Now().UnixMilli(),
		}
		data, _ := json.Marshal(msg)
		_ = conn.WriteMessage(websocket.TextMessage, data)
	}
	return true
}

func (b *Adapter) GetDepth(symbol string) (*exchange.Orderbook, bool) {
	realSymbol, mult, err := b.resolveSymbol(symbol)
	if err != nil {
		return nil, false
	}
	val, ok := b.depthStore.Load(realSymbol)
	if !ok {
		return nil, false
	}
	ob := val.(*exchange.Orderbook)
	clone := &exchange.Orderbook{
		Symbol: symbol,
		Time:   ob.Time,
		Bids:   make([]exchange.PriceLevel, len(ob.Bids)),
		Asks:   make([]exchange.PriceLevel, len(ob.Asks)),
	}
	for i, level := range ob.Bids {
		clone.Bids[i] = exchange.PriceLevel{
			Price:    exchange.ScalePriceFromContracts(level.Price, mult),
			Quantity: exchange.ScaleSizeFromContracts(level.Quantity, mult),
		}
	}
	for i, level := range ob.Asks {
		clone.Asks[i] = exchange.PriceLevel{
			Price:    exchange.ScalePriceFromContracts(level.Price, mult),
			Quantity: exchange.ScaleSizeFromContracts(level.Quantity, mult),
		}
	}
	return clone, true
}

func (b *Adapter) runPriceStream(symbols []string) {
	for {
		err := b.connectPriceWS(symbols)
		if err != nil {
			log.Printf("[binance] price stream error: %v, reconnecting in 5s", err)
		}
		time.Sleep(5 * time.Second)
	}
}

func (b *Adapter) connectPriceWS(symbols []string) error {
	streams := make([]string, 0, len(symbols)*2)
	for _, s := range symbols {
		streams = append(streams, strings.ToLower(s)+"@bookTicker")
	}

	// Include depth streams for any currently subscribed depth symbols
	b.priceMu.Lock()
	for sym := range b.depthSyms {
		streams = append(streams, strings.ToLower(sym)+"@depth20@100ms")
	}
	// Clear stale depth data before reconnect
	for sym := range b.depthSyms {
		b.depthStore.Delete(sym)
	}
	b.priceMu.Unlock()

	wsURL := "wss://fstream.binance.com/stream?streams=" + strings.Join(streams, "/")

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return fmt.Errorf("price ws dial: %w", err)
	}
	b.priceMu.Lock()
	b.priceConn = conn
	b.priceMu.Unlock()
	if b.wsMetricsCallback != nil {
		b.wsMetricsCallback(exchange.WSEvent{Type: exchange.WSEventConnect, Timestamp: time.Now()})
	}

	defer func() {
		conn.Close()
		b.priceMu.Lock()
		b.priceConn = nil
		b.priceMu.Unlock()
		if b.wsMetricsCallback != nil {
			b.wsMetricsCallback(exchange.WSEvent{Type: exchange.WSEventDisconnect, Timestamp: time.Now()})
		}
	}()

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("price ws read: %w", err)
		}
		if b.wsMetricsCallback != nil {
			b.wsMetricsCallback(exchange.WSEvent{Type: exchange.WSEventMessage, Timestamp: time.Now()})
		}

		// Combined stream format: {"stream":"btcusdt@bookTicker","data":{...}}
		var envelope struct {
			Stream string          `json:"stream"`
			Data   json.RawMessage `json:"data"`
		}
		if json.Unmarshal(msg, &envelope) != nil || envelope.Data == nil {
			continue
		}

		if strings.HasSuffix(envelope.Stream, "@bookTicker") {
			b.handleBBOMessage(envelope.Data)
		} else if strings.Contains(envelope.Stream, "@depth") {
			// Stream format: "btcusdt@depth20@100ms" — extract symbol
			sym := strings.ToUpper(strings.Split(envelope.Stream, "@")[0])
			b.parseDepthForSymbol(sym, envelope.Data)
		}
	}
}

func (b *Adapter) handleBBOMessage(data json.RawMessage) {
	var ticker struct {
		Symbol string `json:"s"`
		Bid    string `json:"b"`
		BidQty string `json:"B"`
		Ask    string `json:"a"`
		AskQty string `json:"A"`
	}
	if json.Unmarshal(data, &ticker) != nil {
		return
	}

	bid, _ := strconv.ParseFloat(ticker.Bid, 64)
	ask, _ := strconv.ParseFloat(ticker.Ask, 64)
	if bid > 0 && ask > 0 {
		b.priceStore.Store(ticker.Symbol, exchange.BBO{Bid: bid, Ask: ask})
	}
}

// parseDepthForSymbol parses a depth5 snapshot and stores it for the given symbol.
func (b *Adapter) parseDepthForSymbol(symbol string, data json.RawMessage) {
	// Binance futures depth5 uses "bids"/"asks", but diff depth uses "b"/"a".
	// Try both formats.
	var depth struct {
		Bids [][]string `json:"bids"`
		Asks [][]string `json:"asks"`
		B    [][]string `json:"b"`
		A    [][]string `json:"a"`
		T    int64      `json:"T"`
	}
	if json.Unmarshal(data, &depth) != nil {
		return
	}
	// Prefer "bids"/"asks", fall back to "b"/"a"
	bids := depth.Bids
	asks := depth.Asks
	if len(bids) == 0 {
		bids = depth.B
	}
	if len(asks) == 0 {
		asks = depth.A
	}

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

	ob := &exchange.Orderbook{
		Symbol: symbol,
		Bids:   parseLevels(bids),
		Asks:   parseLevels(asks),
		Time:   time.UnixMilli(depth.T),
	}
	b.depthStore.Store(symbol, ob)
}
