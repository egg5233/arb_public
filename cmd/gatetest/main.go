package main

import (
	"encoding/json"
	"fmt"
	"os"

	"arb/internal/config"
	"arb/pkg/exchange"
	"arb/pkg/exchange/gateio"
)

func main() {
	cfg := config.Load()

	if cfg.GateioAPIKey == "" {
		fmt.Println("ERROR: Gate.io API key not configured")
		os.Exit(1)
	}

	adapter := gateio.NewAdapter(exchange.ExchangeConfig{
		Exchange:  "gateio",
		ApiKey:    cfg.GateioAPIKey,
		SecretKey: cfg.GateioSecretKey,
	})
	client := gateio.NewClient(cfg.GateioAPIKey, cfg.GateioSecretKey)

	fmt.Println("=== Gate.io API Endpoint Tests ===")
	fmt.Println()

	// Test 1: GET /unified/unified_mode
	fmt.Println("--- GET /unified/unified_mode ---")
	data, err := client.Get("/unified/unified_mode", nil)
	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
	} else {
		var pretty json.RawMessage = data
		out, _ := json.MarshalIndent(pretty, "", "  ")
		fmt.Println(string(out))

		var mode struct {
			Mode     string `json:"mode"`
			Settings struct {
				UsdtFutures bool `json:"usdt_futures"`
				SpotHedge   bool `json:"spot_hedge"`
			} `json:"settings"`
		}
		if json.Unmarshal(data, &mode) == nil {
			fmt.Printf("\nParsed mode: %q\n", mode.Mode)
			fmt.Printf("Is unified: %v\n", mode.Mode != "" && mode.Mode != "classic")
		}
	}
	fmt.Println()

	// Test 2: GET /unified/accounts
	fmt.Println("--- GET /unified/accounts ---")
	data, err = client.Get("/unified/accounts", nil)
	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
	} else {
		var resp struct {
			TotalAvailableMargin      string `json:"total_available_margin"`
			UnifiedAccountTotalEquity string `json:"unified_account_total_equity"`
			TotalMaintenanceMargin    string `json:"total_maintenance_margin"`
			Balances                  map[string]struct {
				Available string `json:"available"`
				Equity    string `json:"equity"`
				Freeze    string `json:"freeze"`
			} `json:"balances"`
		}
		if json.Unmarshal(data, &resp) == nil {
			fmt.Printf("Total equity:     %s\n", resp.UnifiedAccountTotalEquity)
			fmt.Printf("Available margin: %s\n", resp.TotalAvailableMargin)
			fmt.Printf("Maintenance:      %s\n", resp.TotalMaintenanceMargin)
			if usdt, ok := resp.Balances["USDT"]; ok {
				fmt.Printf("USDT available:   %s\n", usdt.Available)
				fmt.Printf("USDT equity:      %s\n", usdt.Equity)
				fmt.Printf("USDT freeze:      %s\n", usdt.Freeze)
			}
		} else {
			out, _ := json.MarshalIndent(json.RawMessage(data), "", "  ")
			fmt.Println(string(out))
		}
	}
	fmt.Println()

	// Test 3: GET /futures/usdt/accounts (current method)
	fmt.Println("--- GET /futures/usdt/accounts (current) ---")
	data, err = client.Get("/futures/usdt/accounts", nil)
	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
	} else {
		var resp struct {
			Total     string `json:"total"`
			Available string `json:"available"`
			Currency  string `json:"currency"`
		}
		if json.Unmarshal(data, &resp) == nil {
			fmt.Printf("Total:     %s\n", resp.Total)
			fmt.Printf("Available: %s\n", resp.Available)
			fmt.Printf("Currency:  %s\n", resp.Currency)
		}
	}
	fmt.Println()

	// Test 4: GET /spot/accounts (spot balance)
	fmt.Println("--- GET /spot/accounts (spot USDT) ---")
	data, err = client.Get("/spot/accounts", nil)
	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
	} else {
		var accounts []struct {
			Currency  string `json:"currency"`
			Available string `json:"available"`
			Locked    string `json:"locked"`
		}
		if json.Unmarshal(data, &accounts) == nil {
			for _, a := range accounts {
				if a.Currency == "USDT" {
					fmt.Printf("USDT available: %s\n", a.Available)
					fmt.Printf("USDT locked:    %s\n", a.Locked)
				}
			}
		}
	}
	fmt.Println()

	// Test 5: DetectUnifiedMode via adapter
	fmt.Println("--- Adapter.DetectUnifiedMode() ---")
	adapter.DetectUnifiedMode()
	fmt.Printf("IsUnified: %v\n", adapter.IsUnified())
	fmt.Println()

	// Test 6: GetFuturesBalance via adapter
	fmt.Println("--- Adapter.GetFuturesBalance() ---")
	bal, err := adapter.GetFuturesBalance()
	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
	} else {
		fmt.Printf("Total:       %.4f\n", bal.Total)
		fmt.Printf("Available:   %.4f\n", bal.Available)
		fmt.Printf("Frozen:      %.4f\n", bal.Frozen)
		fmt.Printf("MarginRatio: %.6f\n", bal.MarginRatio)
	}
}
