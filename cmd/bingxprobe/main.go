// Package main — bingxprobe: minimal CLI that connects to BingX swap-market
// public WebSocket, subscribes to a few @bookTicker channels, and prints raw
// gzip-decoded messages for ~10 seconds. Used to confirm what BingX is
// actually emitting on the wire when adapter behavior looks wrong.
//
// Origin: written 2026-04-24 to diagnose nonsense BingX BBO values
// (SOON@bingx bid=24565 ask=28345 vs real ~$0.18). Live probe confirmed
// BingX itself emits correct prices, isolating the bug to Go's
// case-insensitive JSON decode in pkg/exchange/bingx/ws.go (fixed v0.34.6).
//
// Usage: go run ./cmd/bingxprobe
package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gorilla/websocket"
)

func main() {
	c, _, err := websocket.DefaultDialer.Dial("wss://open-api-swap.bingx.com/swap-market", nil)
	if err != nil { panic(err) }
	defer c.Close()

	for _, sym := range []string{"SOON-USDT", "HOLO-USDT", "SIGN-USDT", "RAVE-USDT"} {
		msg := map[string]interface{}{"id": sym, "reqType": "sub", "dataType": sym + "@bookTicker"}
		b, _ := json.Marshal(msg)
		c.WriteMessage(websocket.TextMessage, b)
	}

	done := time.After(10 * time.Second)
	for {
		select {
		case <-done: return
		default:
		}
		c.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, raw, err := c.ReadMessage()
		if err != nil { fmt.Println("read err:", err); continue }

		// BingX gzips messages
		gz, err := gzip.NewReader(bytes.NewReader(raw))
		if err != nil { fmt.Println("gzip err:", err, "raw:", string(raw)); continue }
		out := new(bytes.Buffer)
		out.ReadFrom(gz)
		gz.Close()
		fmt.Println(out.String())
	}
}
