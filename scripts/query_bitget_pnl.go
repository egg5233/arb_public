package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	bitgetpkg "arb/pkg/exchange/bitget"
)

func main() {
	cfgData, _ := os.ReadFile("/var/solana/data/arb/config.json")
	var cfg struct {
		Exchanges struct {
			Bitget struct {
				APIKey     string `json:"api_key"`
				SecretKey  string `json:"secret_key"`
				Passphrase string `json:"passphrase"`
			} `json:"bitget"`
		} `json:"exchanges"`
	}
	json.Unmarshal(cfgData, &cfg)

	client := bitgetpkg.NewClient(cfg.Exchanges.Bitget.APIKey, cfg.Exchanges.Bitget.SecretKey, cfg.Exchanges.Bitget.Passphrase)

	since := time.Date(2026, 3, 26, 19, 0, 0, 0, time.UTC)
	params := map[string]string{
		"symbol":      "BARDUSDT",
		"productType": "USDT-FUTURES",
		"startTime":   fmt.Sprintf("%d", since.UnixMilli()),
		"limit":       "20",
	}

	data, err := client.Get("/api/v2/mix/position/history-position", params)
	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
		return
	}

	fmt.Printf("Raw: %s\n", string(data)[:min(len(data), 1500)])
}

func min(a, b int) int {
	if a < b { return a }
	return b
}
