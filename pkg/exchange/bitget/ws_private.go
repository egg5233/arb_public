package bitget

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"arb/pkg/exchange"
	"sync"
	"time"

	"arb/pkg/utils"

	"github.com/gorilla/websocket"
)

var wsPrivLog = utils.NewLogger("bitget-ws-priv")

// orderInfo holds real-time order status from private WebSocket.
type orderInfo struct {
	OrderID      string
	ClientOID    string
	Status       string  // NEW, PARTIALLY_FILLED, FILLED, CANCELED
	FilledVolume float64 // accBaseVolume
	AvgPrice     float64
}

// orderStore is a thread-safe store for order updates.
type orderStore struct {
	orders sync.Map // map[orderId string]orderInfo
}

func (s *orderStore) UpdateOrder(info orderInfo) {
	s.orders.Store(info.OrderID, info)
}

func (s *orderStore) GetOrder(orderID string) (orderInfo, bool) {
	val, ok := s.orders.Load(orderID)
	if !ok {
		return orderInfo{}, false
	}
	return val.(orderInfo), true
}

// WSPrivateClient manages the private WebSocket connection for order/position updates.
type WSPrivateClient struct {
	apiKey     string
	secretKey  string
	passphrase string
	stop       chan struct{}
	store      *orderStore
	onFill     *func(exchange.OrderUpdate)
}

// NewWSPrivateClient creates a new private WebSocket client.
func NewWSPrivateClient(apiKey, secretKey, passphrase string, onFill *func(exchange.OrderUpdate)) *WSPrivateClient {
	return &WSPrivateClient{
		apiKey:     apiKey,
		secretKey:  secretKey,
		passphrase: passphrase,
		stop:       make(chan struct{}),
		store:      &orderStore{},
		onFill:     onFill,
	}
}

// Start launches the private WebSocket stream with auto-reconnect.
func (ws *WSPrivateClient) Start() {
	go func() {
		for {
			select {
			case <-ws.stop:
				return
			default:
				if err := ws.connectPrivate(); err != nil {
					wsPrivLog.Error("private stream error: %v, reconnecting in 5s", err)
				} else {
					wsPrivLog.Warn("private stream closed, reconnecting in 5s")
				}
				time.Sleep(5 * time.Second)
			}
		}
	}()
}

// Stop shuts down the private WebSocket client.
func (ws *WSPrivateClient) Stop() {
	close(ws.stop)
}

func (ws *WSPrivateClient) connectPrivate() error {
	wsPrivLog.Info("connecting...")
	c, _, err := websocket.DefaultDialer.Dial(wsURLPrivate, nil)
	if err != nil {
		return err
	}
	defer c.Close()

	// 1. Login with HMAC-SHA256 signature
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	sign := ws.generateSign(timestamp, "GET", "/user/verify")
	loginMsg := map[string]interface{}{
		"op": "login",
		"args": []map[string]string{
			{
				"apiKey":     ws.apiKey,
				"passphrase": ws.passphrase,
				"timestamp":  timestamp,
				"sign":       sign,
			},
		},
	}
	if err := c.WriteJSON(loginMsg); err != nil {
		return fmt.Errorf("login write failed: %w", err)
	}

	// Read login response
	_, loginResp, err := c.ReadMessage()
	if err != nil {
		return fmt.Errorf("login read failed: %w", err)
	}
	if strings.Contains(string(loginResp), `"code":0`) || strings.Contains(string(loginResp), `"code":"0"`) {
		wsPrivLog.Info("login success")
	} else {
		return fmt.Errorf("login failed: %s", string(loginResp))
	}

	// 2. Ping loop
	go keepAlive(c, "Private", ws.stop)

	// 3. Subscribe to orders channel
	subMsg := map[string]interface{}{
		"op": "subscribe",
		"args": []map[string]string{
			{
				"instType": "USDT-FUTURES",
				"channel":  "orders",
				"instId":   "default", // all symbols
			},
		},
	}
	if err := c.WriteJSON(subMsg); err != nil {
		return err
	}
	wsPrivLog.Info("subscribed to orders channel")

	// 4. Read loop
	for {
		_, message, err := c.ReadMessage()
		if err != nil {
			return err
		}
		if string(message) == "pong" {
			continue
		}
		ws.handlePrivateMessage(message)
	}
}

func (ws *WSPrivateClient) handlePrivateMessage(msg []byte) {
	var event struct {
		Action string            `json:"action"`
		Arg    map[string]string `json:"arg"`
		Data   []json.RawMessage `json:"data"`
	}

	if err := json.Unmarshal(msg, &event); err != nil {
		return
	}

	if event.Arg["channel"] != "orders" || len(event.Data) == 0 {
		return
	}

	for _, rawData := range event.Data {
		var dataMap map[string]interface{}
		if err := json.Unmarshal(rawData, &dataMap); err != nil {
			continue
		}

		orderID, _ := dataMap["orderId"].(string)
		clientOid, _ := dataMap["clientOid"].(string)
		status, _ := dataMap["status"].(string)
		instId, _ := dataMap["instId"].(string)
		reduceOnly, _ := dataMap["reduceOnly"].(string) // "YES" or "NO"

		accFillStr, _ := dataMap["accBaseVolume"].(string)
		filled, _ := strconv.ParseFloat(accFillStr, 64)

		avgPriceStr, _ := dataMap["priceAvg"].(string)
		avgPrice, _ := strconv.ParseFloat(avgPriceStr, 64)

		info := orderInfo{
			OrderID:      orderID,
			ClientOID:    clientOid,
			Status:       strings.ToLower(status),
			FilledVolume: filled,
			AvgPrice:     avgPrice,
		}
		ws.store.UpdateOrder(info)

		// In one-way mode, reduceOnly="YES" means this is a close fill
		// (SL trigger, TP trigger, or liquidation).
		isClose := strings.EqualFold(reduceOnly, "YES")

		// Normalize instId to internal symbol format (e.g. "4USDT").
		symbol := strings.TrimSuffix(instId, "_UMCBL")

		if filled > 0 {
			wsPrivLog.Info("order update: %s status=%s filled=%.6f avg=%.8f reduceOnly=%s symbol=%s",
				orderID, status, filled, avgPrice, reduceOnly, symbol)
		}
		if info.Status == "filled" && info.FilledVolume > 0 && ws.onFill != nil && *ws.onFill != nil {
			(*ws.onFill)(exchange.OrderUpdate{
				OrderID:      info.OrderID,
				ClientOID:    info.ClientOID,
				Status:       info.Status,
				FilledVolume: info.FilledVolume,
				AvgPrice:     info.AvgPrice,
				Symbol:       symbol,
				ReduceOnly:   isClose,
			})
		}
	}
}

func (ws *WSPrivateClient) generateSign(timestamp, method, requestPath string) string {
	message := timestamp + method + requestPath
	mac := hmac.New(sha256.New, []byte(ws.secretKey))
	mac.Write([]byte(message))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}
