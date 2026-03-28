package main

import (
	"fmt"
	"time"

	"arb/internal/config"
	"arb/pkg/exchange"
	"arb/pkg/exchange/bitget"
)

func main() {
	cfg := config.Load()

	bg := bitget.NewAdapter(exchange.ExchangeConfig{
		Exchange: "bitget", ApiKey: cfg.BitgetAPIKey, SecretKey: cfg.BitgetSecretKey, Passphrase: cfg.BitgetPassphrase,
	})

	since, _ := time.Parse(time.RFC3339, "2026-03-26T11:40:05Z")

	fees, err := bg.GetFundingFees("4USDT", since)
	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
		return
	}
	fmt.Printf("Total payments (after filter): %d\n\n", len(fees))

	var sum float64
	for _, f := range fees {
		sum += f.Amount
		fmt.Printf("  %s  %+.8f\n", f.Time.UTC().Format("2006-01-02 15:04"), f.Amount)
	}
	fmt.Printf("\nSum: %.8f\n", sum)
	fmt.Printf("Position funding_collected: 0.81865975732\n")
	fmt.Printf("Difference: %.8f\n", sum-0.81865975732)
}
