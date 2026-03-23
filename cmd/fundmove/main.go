package main

import (
	"fmt"
	"os"

	"arb/internal/config"
	"arb/pkg/exchange"
	"arb/pkg/exchange/bitget"
)

func main() {
	cfg := config.Load()

	bg := bitget.NewAdapter(exchange.ExchangeConfig{
		Exchange:   "bitget",
		ApiKey:     cfg.BitgetAPIKey,
		SecretKey:  cfg.BitgetSecretKey,
		Passphrase: cfg.BitgetPassphrase,
	})

	// Step 1: Transfer 160 USDT from futures to spot
	fmt.Println("=== Bitget: Futures → Spot (160 USDT) ===")
	if err := bg.TransferToSpot("USDT", "160"); err != nil {
		fmt.Printf("ERROR: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Done.")

	// Step 2: Withdraw 40 USDT to each exchange
	targets := []string{"binance", "bybit", "gateio", "okx"}
	for _, dest := range targets {
		addrs := cfg.ExchangeAddresses[dest]
		// Pick APT first, then BEP20
		chain, addr := "", ""
		for _, c := range []string{"APT", "BEP20"} {
			if a, ok := addrs[c]; ok && a != "" {
				chain, addr = c, a
				break
			}
		}
		if chain == "" {
			fmt.Printf("%-10s SKIP: no APT/BEP20 address\n", dest)
			continue
		}

		fmt.Printf("\n=== Bitget → %s (40 USDT via %s) ===\n", dest, chain)
		result, err := bg.Withdraw(exchange.WithdrawParams{
			Coin: "USDT", Chain: chain, Address: addr, Amount: "40",
		})
		if err != nil {
			fmt.Printf("%-10s ERROR: %v\n", dest, err)
		} else {
			fmt.Printf("%-10s OK TxID=%s\n", dest, result.TxID)
		}
	}
}
