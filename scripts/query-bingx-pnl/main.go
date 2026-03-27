package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	bingxpkg "arb/pkg/exchange/bingx"
)

func main() {
	cfgData, _ := os.ReadFile("/var/solana/data/arb/config.json")
	var cfg struct {
		Exchanges struct {
			BingX struct {
				APIKey    string `json:"api_key"`
				SecretKey string `json:"secret_key"`
			} `json:"bingx"`
		} `json:"exchanges"`
	}
	json.Unmarshal(cfgData, &cfg)

	client := bingxpkg.NewClient(cfg.Exchanges.BingX.APIKey, cfg.Exchanges.BingX.SecretKey)

	// Query position history for BARD since March 26
	since := time.Date(2026, 3, 26, 0, 0, 0, 0, time.UTC)
	params := map[string]string{
		"symbol":  "BARD-USDT",
		"startTs": fmt.Sprintf("%d", since.UnixMilli()),
		"endTs":   fmt.Sprintf("%d", time.Now().UnixMilli()),
	}

	data, err := client.Get("/openApi/swap/v1/trade/positionHistory", params)
	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
		return
	}

	var resp struct {
		PositionHistory []json.RawMessage `json:"positionHistory"`
	}
	json.Unmarshal(data, &resp)
	fmt.Printf("Total records: %d\n\n", len(resp.PositionHistory))

	for i, r := range resp.PositionHistory {
		var pretty map[string]interface{}
		json.Unmarshal(r, &pretty)
		b, _ := json.MarshalIndent(pretty, "", "  ")
		fmt.Printf("=== Record %d ===\n%s\n\n", i+1, string(b))
	}
}
