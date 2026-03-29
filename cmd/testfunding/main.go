package main

import (
	"fmt"
	"time"

	"arb/internal/config"
	"arb/pkg/exchange"
	"arb/pkg/exchange/binance"
	"arb/pkg/exchange/bingx"
)

func main() {
	cfg := config.Load()

	bn := binance.NewAdapter(exchange.ExchangeConfig{
		Exchange: "binance", ApiKey: cfg.BinanceAPIKey, SecretKey: cfg.BinanceSecretKey,
	})
	bx := bingx.NewAdapter(exchange.ExchangeConfig{
		Exchange: "bingx", ApiKey: cfg.BingXAPIKey, SecretKey: cfg.BingXSecretKey,
	})

	since, _ := time.Parse(time.RFC3339, "2026-03-26T11:40:05Z")

	fmt.Println("=== Binance GetClosePnL (KATUSDT) ===")
	pnls, err := bn.GetClosePnL("KATUSDT", since)
	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
	} else {
		for _, p := range pnls {
			fmt.Printf("  Side=%s Entry=%.8f Exit=%.8f PricePnL=%.4f Fees=%.4f Funding=%.4f Net=%.4f Size=%.2f Time=%s\n",
				p.Side, p.EntryPrice, p.ExitPrice, p.PricePnL, p.Fees, p.Funding, p.NetPnL, p.CloseSize, p.CloseTime.Format(time.RFC3339))
		}
	}

	fmt.Println("\n=== BingX GetClosePnL (KATUSDT) ===")
	pnls, err = bx.GetClosePnL("KATUSDT", since)
	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
	} else {
		for _, p := range pnls {
			fmt.Printf("  Side=%s Entry=%.8f Exit=%.8f PricePnL=%.4f Fees=%.4f Funding=%.4f Net=%.4f Size=%.2f Time=%s\n",
				p.Side, p.EntryPrice, p.ExitPrice, p.PricePnL, p.Fees, p.Funding, p.NetPnL, p.CloseSize, p.CloseTime.Format(time.RFC3339))
		}
	}
}
