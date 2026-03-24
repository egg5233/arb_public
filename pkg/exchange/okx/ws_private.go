package okx

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"arb/pkg/exchange"
	"arb/pkg/utils"

	"github.com/gorilla/websocket"
)

var wsPrivLog = utils.NewLogger("okx-ws-priv")

const (
	okxPrivateWS = "wss://ws.okx.com:8443/ws/v5/private"
)

// ---------------------------------------------------------------------------
// WebSocket: Private Stream (order updates)
// ---------------------------------------------------------------------------

func (a *Adapter) StartPrivateStream() {
	go a.runPrivateStream()
}

func (a *Adapter) GetOrderUpdate(orderID string) (exchange.OrderUpdate, bool) {
	val, ok := a.orderStore.Load(orderID)
	if !ok {
		return exchange.OrderUpdate{}, false
	}
	return val.(exchange.OrderUpdate), true
}

func (a *Adapter) runPrivateStream() {
	for {
		err := a.connectPrivateWS()
		if err != nil {
			wsPrivLog.Error("private stream error: %v, reconnecting in 5s", err)
		}
		time.Sleep(5 * time.Second)
	}
}

func (a *Adapter) connectPrivateWS() error {
	conn, _, err := websocket.DefaultDialer.Dial(okxPrivateWS, nil)
	if err != nil {
		return fmt.Errorf("private ws dial: %w", err)
	}

	a.privMu.Lock()
	a.privConn = conn
	a.privMu.Unlock()
	wsPrivLog.Info("private ws connected")

	defer func() {
		conn.Close()
		a.privMu.Lock()
		a.privConn = nil
		a.privMu.Unlock()
	}()

	// Authenticate
	if err := a.wsLogin(conn); err != nil {
		return fmt.Errorf("private ws login: %w", err)
	}

	// Subscribe to orders channel
	subMsg := map[string]interface{}{
		"op": "subscribe",
		"args": []map[string]string{
			{"channel": "orders", "instType": "SWAP"},
		},
	}
	if err := wsWriteJSON(conn, &a.privMu, subMsg); err != nil {
		return fmt.Errorf("private ws subscribe: %w", err)
	}
	wsPrivLog.Info("subscribed to orders channel")

	// Start keepalive
	done := make(chan struct{})
	defer close(done)
	go a.wsPingLoop(conn, &a.privMu, done)

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("private ws read: %w", err)
		}

		if string(msg) == "pong" {
			continue
		}

		a.handleOrderMessage(msg)
	}
}

// wsLogin authenticates the WebSocket connection using HMAC-SHA256.
func (a *Adapter) wsLogin(conn *websocket.Conn) error {
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	sign := a.client.SignWS(timestamp)

	loginMsg := map[string]interface{}{
		"op": "login",
		"args": []map[string]string{
			{
				"apiKey":     a.apiKey,
				"passphrase": a.passphrase,
				"timestamp":  timestamp,
				"sign":       sign,
			},
		},
	}

	if err := wsWriteJSON(conn, &a.privMu, loginMsg); err != nil {
		return err
	}

	// Wait for login response
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	_, msg, err := conn.ReadMessage()
	conn.SetReadDeadline(time.Time{}) // clear deadline
	if err != nil {
		return fmt.Errorf("read login response: %w", err)
	}

	var resp struct {
		Event string `json:"event"`
		Code  string `json:"code"`
		Msg   string `json:"msg"`
	}
	if err := json.Unmarshal(msg, &resp); err != nil {
		return fmt.Errorf("unmarshal login response: %w (raw: %s)", err, string(msg))
	}

	if resp.Event != "login" || resp.Code != "0" {
		return fmt.Errorf("login failed: event=%s code=%s msg=%s", resp.Event, resp.Code, resp.Msg)
	}

	wsPrivLog.Info("private ws authenticated")
	return nil
}

func (a *Adapter) handleOrderMessage(msg []byte) {
	var envelope struct {
		Arg struct {
			Channel string `json:"channel"`
		} `json:"arg"`
		Data []struct {
			InstID    string `json:"instId"`
			OrdID     string `json:"ordId"`
			ClOrdID   string `json:"clOrdId"`
			State     string `json:"state"`
			FillSz    string `json:"fillSz"`
			AccFillSz string `json:"accFillSz"`
			AvgPx     string `json:"avgPx"`
		} `json:"data"`
	}

	if json.Unmarshal(msg, &envelope) != nil {
		return
	}
	if envelope.Arg.Channel != "orders" || len(envelope.Data) == 0 {
		return
	}

	for _, o := range envelope.Data {
		// OKX reports fill sizes in contracts; convert to base units via ctVal.
		filledVol, _ := strconv.ParseFloat(o.AccFillSz, 64)
		symbol := fromOKXInstID(o.InstID)
		ctVal := a.getCtVal(symbol)
		filledVol *= ctVal

		avgPrice, _ := strconv.ParseFloat(o.AvgPx, 64)

		update := exchange.OrderUpdate{
			OrderID:      o.OrdID,
			ClientOID:    o.ClOrdID,
			Status:       mapState(o.State),
			FilledVolume: filledVol,
			AvgPrice:     avgPrice,
		}

		wsPrivLog.Info("order update: %s state=%s filled=%.6f avg=%.8f", o.OrdID, o.State, filledVol, avgPrice)

		a.orderStore.Store(o.OrdID, update)
		if o.ClOrdID != "" {
			a.orderStore.Store(o.ClOrdID, update)
		}
		if update.Status == "filled" && update.FilledVolume > 0 && a.orderCallback != nil {
			a.orderCallback(update)
		}
	}
}
