package binance

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"arb/pkg/exchange"
	"arb/pkg/utils"

	"github.com/gorilla/websocket"
)

var wsPrivLog = utils.NewLogger("binance-ws-priv")

// ---------------------------------------------------------------------------
// WebSocket: Private Stream (listenKey-based)
// ---------------------------------------------------------------------------

func (b *Adapter) StartPrivateStream() {
	go b.runPrivateStream()
}

func (b *Adapter) GetOrderUpdate(orderID string) (exchange.OrderUpdate, bool) {
	val, ok := b.orderStore.Load(orderID)
	if !ok {
		return exchange.OrderUpdate{}, false
	}
	return val.(exchange.OrderUpdate), true
}

func (b *Adapter) runPrivateStream() {
	for {
		err := b.connectPrivateWS()
		if err != nil {
			wsPrivLog.Error("private stream error: %v, reconnecting in 5s", err)
		}
		time.Sleep(5 * time.Second)
	}
}

func (b *Adapter) connectPrivateWS() error {
	// Create or refresh listenKey
	listenKey, err := b.createListenKey()
	if err != nil {
		return fmt.Errorf("create listenKey: %w", err)
	}
	b.listenKey = listenKey

	wsURL := "wss://fstream.binance.com/ws/" + listenKey
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return fmt.Errorf("private ws dial: %w", err)
	}
	b.privConn = conn
	wsPrivLog.Info("private ws connected (listenKey=%s...)", listenKey[:8])
	defer func() {
		conn.Close()
		b.privConn = nil
	}()

	// Start keepalive goroutine (PUT every 30 minutes)
	stopKeepalive := make(chan struct{})
	go func() {
		ticker := time.NewTicker(30 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := b.keepaliveListenKey(); err != nil {
					wsPrivLog.Error("listenKey keepalive failed: %v", err)
				}
			case <-stopKeepalive:
				return
			}
		}
	}()
	defer close(stopKeepalive)

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("private ws read: %w", err)
		}
		b.handlePrivateMessage(msg)
	}
}

func (b *Adapter) handlePrivateMessage(msg []byte) {
	var base struct {
		EventType string `json:"e"`
		EventTime int64  `json:"E"` // must declare to prevent case-insensitive collision
	}
	if err := json.Unmarshal(msg, &base); err != nil {
		wsPrivLog.Error("unmarshal base: %v", err)
		return
	}

	if base.EventType == "ORDER_TRADE_UPDATE" {
		var evt struct {
			Order struct {
				Symbol        string `json:"s"`
				ClientOrderID string `json:"c"`
				Side          string `json:"S"`
				OrderType     string `json:"o"`
				OrderStatus   string `json:"X"`
				OrderID       int64  `json:"i"`
				AvgPrice      string `json:"ap"`
				FilledQty     string `json:"z"`
				ReduceOnly    bool   `json:"R"`
				OrigQty       string `json:"q"`
				ClosePosition bool   `json:"cp"`
				OrigOrderType string `json:"ot"`
			} `json:"o"`
		}
		if json.Unmarshal(msg, &evt) != nil {
			return
		}

		o := evt.Order
		oid := strconv.FormatInt(o.OrderID, 10)
		filledVol, _ := strconv.ParseFloat(o.FilledQty, 64)
		avgPrice, _ := strconv.ParseFloat(o.AvgPrice, 64)

		// ReduceOnly is true when the exchange explicitly marks the fill as reducing position,
		// OR when it results from a TRIGGERED conditional order with closePosition=true.
		// Do NOT broadly classify by orderType alone (STOP_MARKET without cp=true could be opening).
		isCloseFill := o.ReduceOnly ||
			(o.ClosePosition && (o.OrigOrderType == "STOP_MARKET" || o.OrigOrderType == "TAKE_PROFIT_MARKET"))

		wsPrivLog.Info("order update: %s %s %s status=%s filled=%.6f avg=%.8f R=%v cp=%v ot=%s isClose=%v",
			o.Symbol, o.Side, oid, o.OrderStatus, filledVol, avgPrice, o.ReduceOnly, o.ClosePosition, o.OrigOrderType, isCloseFill)

		upd := exchange.OrderUpdate{
			OrderID:      oid,
			ClientOID:    o.ClientOrderID,
			Status:       strings.ToLower(o.OrderStatus),
			FilledVolume: filledVol,
			AvgPrice:     avgPrice,
			Symbol:       o.Symbol,
			ReduceOnly:   isCloseFill,
		}
		b.orderStore.Store(oid, upd)
		if upd.Status == "filled" && upd.FilledVolume > 0 && b.orderMetricsCallback != nil {
			b.orderMetricsCallback(exchange.OrderMetricEvent{
				Type:      exchange.OrderMetricFilled,
				OrderID:   oid,
				FilledQty: upd.FilledVolume,
				Timestamp: time.Now(),
			})
		}
		if upd.Status == "filled" && upd.FilledVolume > 0 && b.orderCallback != nil {
			b.orderCallback(upd)
		}
		return
	}

	if base.EventType == "ALGO_UPDATE" {
		var algo struct {
			Order struct {
				AlgoID     int64  `json:"aid"`
				RealID     string `json:"ai"`
				Symbol     string `json:"s"`
				AlgoStatus string `json:"X"`
			} `json:"o"`
		}
		if json.Unmarshal(msg, &algo) != nil {
			return
		}
		if algo.Order.AlgoStatus == "TRIGGERED" && algo.Order.RealID != "" {
			remap := exchange.AlgoRemap{
				AlgoID: strconv.FormatInt(algo.Order.AlgoID, 10),
				RealID: algo.Order.RealID,
				Symbol: algo.Order.Symbol,
			}
			wsPrivLog.Info("algo remap: %s algoID=%s → realID=%s", algo.Order.Symbol, remap.AlgoID, remap.RealID)
			if cb := b.getAlgoRemapCallback(); cb != nil {
				cb(remap)
			}
		}
	}
}

func (b *Adapter) createListenKey() (string, error) {
	body, err := b.client.Post("/fapi/v1/listenKey", nil)
	if err != nil {
		return "", err
	}
	var resp struct {
		ListenKey string `json:"listenKey"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", err
	}
	return resp.ListenKey, nil
}

func (b *Adapter) keepaliveListenKey() error {
	_, err := b.client.Put("/fapi/v1/listenKey", nil)
	return err
}
