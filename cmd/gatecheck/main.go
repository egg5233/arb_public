package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"arb/pkg/exchange"
	"arb/pkg/exchange/gateio"
)

func main() {
	raw, _ := os.ReadFile("config.json")
	var jc struct {
		Exchanges struct {
			Gateio struct {
				APIKey string `json:"api_key"`
				Secret string `json:"secret_key"`
			} `json:"gateio"`
		} `json:"exchanges"`
	}
	json.Unmarshal(raw, &jc)

	adapter := gateio.NewAdapter(exchange.ExchangeConfig{
		Exchange:  "gateio",
		ApiKey:    jc.Exchanges.Gateio.APIKey,
		SecretKey: jc.Exchanges.Gateio.Secret,
	})

	coin := "BTC"
	if len(os.Args) > 1 {
		coin = os.Args[1]
	}

	fmt.Printf("=== Gate.io FlashRepay Test (%s) ===\n\n", coin)

	// Step 1: Check current balance before borrow
	fmt.Printf("--- Step 1: Check %s balance before borrow ---\n", coin)
	mb, err := adapter.GetMarginBalance(coin)
	if err != nil {
		fmt.Printf("ERROR GetMarginBalance: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("  Borrowed: %.8f  Interest: %.8f  Available: %.8f\n\n", mb.Borrowed, mb.Interest, mb.Available)

	if mb.Borrowed > 0 {
		fmt.Println("*** Already have outstanding borrow — skipping borrow, going straight to FlashRepay ***\n")
	} else {
		// Step 2: Borrow a tiny amount
		borrowAmt := "0.00005"
		fmt.Printf("--- Step 2: Borrow %s %s ---\n", borrowAmt, coin)
		err = adapter.MarginBorrow(exchange.MarginBorrowParams{
			Coin:   coin,
			Amount: borrowAmt,
		})
		if err != nil {
			fmt.Printf("ERROR MarginBorrow: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("  Borrow OK")
		time.Sleep(2 * time.Second)

		// Verify borrow exists
		mb, err = adapter.GetMarginBalance(coin)
		if err != nil {
			fmt.Printf("ERROR GetMarginBalance after borrow: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("  Borrowed: %.8f  Interest: %.8f  Available: %.8f\n\n", mb.Borrowed, mb.Interest, mb.Available)
		if mb.Borrowed <= 0 {
			fmt.Println("ERROR: borrow appears to be 0 after borrowing — aborting")
			os.Exit(1)
		}
	}

	// Step 3: FlashRepay
	fmt.Printf("--- Step 3: FlashRepay %s ---\n", coin)
	repayID, err := adapter.FlashRepay(coin)
	if err != nil {
		fmt.Printf("ERROR FlashRepay: %v\n", err)
		fmt.Println("\nChecking residual balance...")
		mb, _ = adapter.GetMarginBalance(coin)
		if mb != nil {
			fmt.Printf("  Borrowed: %.8f  Interest: %.8f  Available: %.8f\n", mb.Borrowed, mb.Interest, mb.Available)
		}
		os.Exit(1)
	}
	fmt.Printf("  RepayID: %s\n", repayID)
	time.Sleep(2 * time.Second)

	// Step 4: Verify debt is cleared
	fmt.Printf("\n--- Step 4: Verify %s debt cleared ---\n", coin)
	mb, err = adapter.GetMarginBalance(coin)
	if err != nil {
		fmt.Printf("ERROR GetMarginBalance after repay: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("  Borrowed: %.8f  Interest: %.8f  Available: %.8f\n", mb.Borrowed, mb.Interest, mb.Available)

	if mb.Borrowed+mb.Interest <= 0 {
		fmt.Println("\n✓ FlashRepay SUCCESS — debt fully cleared")
	} else {
		fmt.Printf("\n✗ FlashRepay INCOMPLETE — %.8f %s debt remains\n", mb.Borrowed+mb.Interest, coin)
		os.Exit(1)
	}
}
