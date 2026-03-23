package okx

import (
	"encoding/json"
	"log"
	"strconv"
	"sync"
	"time"

	"arb/pkg/exchange"

	"github.com/gorilla/websocket"
)

const (
	okxPublicWS  = "wss://ws.okx.com:8443/ws/v5/public"
	wsPingPeriod = 25 * time.Second
)

// ---------------------------------------------------------------------------
// WebSocket: Public Price Stream (tickers for BBO)
// ---------------------------------------------------------------------------

func (a *Adapter) StartPriceStream(symbols []string) {
	a.priceMu.Lock()
	for _, s := range symbols {
		a.priceSyms[s] = true
	}
	a.priceMu.Unlock()

	go a.runPriceStream(symbols)
}

func (a *Adapter) SubscribeSymbol(symbol string) bool {
	a.priceMu.Lock()
	if a.priceSyms[symbol] {
		a.priceMu.Unlock()
		return false
	}
	a.priceSyms[symbol] = true
	conn := a.priceConn
	a.priceMu.Unlock()

	if conn != nil {
		instID := toOKXInstID(symbol)
		msg := map[string]interface{}{
			"op": "subscribe",
			"args": []map[string]string{
				{"channel": "tickers", "instId": instID},
			},
		}
		data, _ := json.Marshal(msg)
		a.priceMu.Lock()
		_ = conn.WriteMessage(websocket.TextMessage, data)
		a.priceMu.Unlock()
	}
	return true
}

func (a *Adapter) GetBBO(symbol string) (exchange.BBO, bool) {
	val, ok := a.priceStore.Load(symbol)
	if !ok {
		return exchange.BBO{}, false
	}
	return val.(exchange.BBO), true
}

func (a *Adapter) GetPriceStore() *sync.Map {
	return &a.priceStore
}

// ---------------------------------------------------------------------------
// WebSocket: Depth Stream (top-5 orderbook)
// ---------------------------------------------------------------------------

func (a *Adapter) SubscribeDepth(symbol string) bool {
	a.priceMu.Lock()
	if a.depthSyms[symbol] {
		a.priceMu.Unlock()
		return false
	}
	a.depthSyms[symbol] = true
	conn := a.priceConn
	a.priceMu.Unlock()

	if conn != nil {
		instID := toOKXInstID(symbol)
		msg := map[string]interface{}{
			"op": "subscribe",
			"args": []map[string]string{
				{"channel": "books5", "instId": instID},
			},
		}
		data, _ := json.Marshal(msg)
		a.priceMu.Lock()
		_ = conn.WriteMessage(websocket.TextMessage, data)
		a.priceMu.Unlock()
	}
	return true
}

func (a *Adapter) UnsubscribeDepth(symbol string) bool {
	a.priceMu.Lock()
	if !a.depthSyms[symbol] {
		a.priceMu.Unlock()
		return false
	}
	delete(a.depthSyms, symbol)
	conn := a.priceConn
	a.priceMu.Unlock()

	a.depthStore.Delete(symbol)

	if conn != nil {
		instID := toOKXInstID(symbol)
		msg := map[string]interface{}{
			"op": "unsubscribe",
			"args": []map[string]string{
				{"channel": "books5", "instId": instID},
			},
		}
		data, _ := json.Marshal(msg)
		a.priceMu.Lock()
		_ = conn.WriteMessage(websocket.TextMessage, data)
		a.priceMu.Unlock()
	}
	return true
}

func (a *Adapter) GetDepth(symbol string) (*exchange.Orderbook, bool) {
	val, ok := a.depthStore.Load(symbol)
	if !ok {
		return nil, false
	}
	return val.(*exchange.Orderbook), true
}

func (a *Adapter) runPriceStream(symbols []string) {
	for {
		err := a.connectPriceWS(symbols)
		if err != nil {
			log.Printf("[okx] price stream error: %v, reconnecting in 5s", err)
		}
		time.Sleep(5 * time.Second)
	}
}

func (a *Adapter) connectPriceWS(symbols []string) error {
	conn, _, err := websocket.DefaultDialer.Dial(okxPublicWS, nil)
	if err != nil {
		return err
	}

	a.priceMu.Lock()
	a.priceConn = conn
	a.priceMu.Unlock()

	defer func() {
		conn.Close()
		a.priceMu.Lock()
		a.priceConn = nil
		a.priceMu.Unlock()
	}()

	// Subscribe to tickers for all symbols + depth for active depth symbols
	args := make([]map[string]string, 0, len(symbols)+len(a.depthSyms))
	for _, s := range symbols {
		args = append(args, map[string]string{
			"channel": "tickers",
			"instId":  toOKXInstID(s),
		})
	}
	// Re-subscribe depth channels and clear stale data
	a.priceMu.Lock()
	for sym := range a.depthSyms {
		args = append(args, map[string]string{
			"channel": "books5",
			"instId":  toOKXInstID(sym),
		})
		a.depthStore.Delete(sym)
	}
	a.priceMu.Unlock()

	subMsg := map[string]interface{}{
		"op":   "subscribe",
		"args": args,
	}
	data, _ := json.Marshal(subMsg)

	a.priceMu.Lock()
	err = conn.WriteMessage(websocket.TextMessage, data)
	a.priceMu.Unlock()
	if err != nil {
		return err
	}

	// Start keepalive goroutine
	done := make(chan struct{})
	defer close(done)
	go a.wsPingLoop(conn, &a.priceMu, done)

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return err
		}

		// OKX sends "pong" as a text response to "ping"
		if string(msg) == "pong" {
			continue
		}

		a.handleTickerMessage(msg)
	}
}

func (a *Adapter) handleTickerMessage(msg []byte) {
	var envelope struct {
		Arg struct {
			Channel string `json:"channel"`
			InstID  string `json:"instId"`
		} `json:"arg"`
	}
	if json.Unmarshal(msg, &envelope) != nil {
		return
	}

	switch envelope.Arg.Channel {
	case "tickers":
		a.handleBBO(msg)
	case "books5":
		a.handleBooks5(msg)
	}
}

func (a *Adapter) handleBBO(msg []byte) {
	var envelope struct {
		Data []struct {
			InstID string `json:"instId"`
			BidPx  string `json:"bidPx"`
			AskPx  string `json:"askPx"`
		} `json:"data"`
	}
	if json.Unmarshal(msg, &envelope) != nil || len(envelope.Data) == 0 {
		return
	}

	for _, t := range envelope.Data {
		bid, _ := strconv.ParseFloat(t.BidPx, 64)
		ask, _ := strconv.ParseFloat(t.AskPx, 64)
		if bid > 0 && ask > 0 {
			symbol := fromOKXInstID(t.InstID)
			a.priceStore.Store(symbol, exchange.BBO{Bid: bid, Ask: ask})
		}
	}
}

func (a *Adapter) handleBooks5(msg []byte) {
	var envelope struct {
		Arg struct {
			InstID string `json:"instId"`
		} `json:"arg"`
		Data []struct {
			Asks [][]string `json:"asks"` // [["price","size","0","orders"],...]
			Bids [][]string `json:"bids"`
			Ts   string     `json:"ts"`
		} `json:"data"`
	}
	if json.Unmarshal(msg, &envelope) != nil || len(envelope.Data) == 0 {
		return
	}

	d := envelope.Data[0]
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

	symbol := fromOKXInstID(envelope.Arg.InstID)
	tsMs, _ := strconv.ParseInt(d.Ts, 10, 64)
	ob := &exchange.Orderbook{
		Symbol: symbol,
		Bids:   parseLevels(d.Bids),
		Asks:   parseLevels(d.Asks),
		Time:   time.UnixMilli(tsMs),
	}
	a.depthStore.Store(symbol, ob)
}

// wsPingLoop sends "ping" every wsPingPeriod to keep the connection alive.
func (a *Adapter) wsPingLoop(conn *websocket.Conn, mu *sync.Mutex, done <-chan struct{}) {
	ticker := time.NewTicker(wsPingPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			mu.Lock()
			err := conn.WriteMessage(websocket.TextMessage, []byte("ping"))
			mu.Unlock()
			if err != nil {
				return
			}
		}
	}
}

// wsWriteJSON marshals v to JSON and writes it to the connection under the given mutex.
func wsWriteJSON(conn *websocket.Conn, mu *sync.Mutex, v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	mu.Lock()
	defer mu.Unlock()
	return conn.WriteMessage(websocket.TextMessage, data)
}
