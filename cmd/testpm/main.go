package main

import (
	"fmt"
	"time"

	"arb/internal/config"
	"arb/pkg/exchange"
	"arb/pkg/exchange/binance"
)

func main() {
	cfg := config.Load()
	bn := binance.NewAdapter(exchange.ExchangeConfig{
		Exchange: "binance", ApiKey: cfg.BinanceAPIKey, SecretKey: cfg.BinanceSecretKey,
	})

	fmt.Println("=== 1. Balance ===")
	bal, err := bn.GetFuturesBalance()
	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
		return
	}
	fmt.Printf("Total=%.4f Available=%.4f Frozen=%.4f MarginRatio=%.6f\n",
		bal.Total, bal.Available, bal.Frozen, bal.MarginRatio)

	fmt.Println("\n=== 2. Place limit buy BTCUSDT at 50000 (won't fill, ~$150 notional) ===")
	oid, err := bn.PlaceOrder(exchange.PlaceOrderParams{
		Symbol:    "BTCUSDT",
		Side:      exchange.SideBuy,
		OrderType: "limit",
		Price:     "50000",
		Size:      "0.003",
		Force:     "gtc",
	})
	if err != nil {
		fmt.Printf("PlaceOrder ERROR: %v\n", err)
		return
	}
	fmt.Printf("Order placed: %s\n", oid)

	fmt.Println("\n=== 3. Check pending orders ===")
	orders, err := bn.GetPendingOrders("BTCUSDT")
	if err != nil {
		fmt.Printf("GetPendingOrders ERROR: %v\n", err)
	} else {
		fmt.Printf("Pending: %d\n", len(orders))
		for _, o := range orders {
			fmt.Printf("  %s %s price=%s size=%s\n", o.OrderID, o.Side, o.Price, o.Size)
		}
	}

	fmt.Println("\n=== 4. Cancel order ===")
	err = bn.CancelOrder("BTCUSDT", oid)
	if err != nil {
		fmt.Printf("CancelOrder ERROR: %v\n", err)
	} else {
		fmt.Printf("Cancelled: %s\n", oid)
	}

	fmt.Println("\n=== 5. Market buy 0.002 BTC (open long, ~$166 notional) ===")
	oid2, err := bn.PlaceOrder(exchange.PlaceOrderParams{
		Symbol:    "BTCUSDT",
		Side:      exchange.SideBuy,
		OrderType: "market",
		Size:      "0.002",
		Force:     "gtc",
	})
	if err != nil {
		fmt.Printf("Market buy ERROR: %v\n", err)
		return
	}
	fmt.Printf("Market buy order: %s\n", oid2)
	time.Sleep(2 * time.Second)

	filled, err := bn.GetOrderFilledQty(oid2, "BTCUSDT")
	if err != nil {
		fmt.Printf("GetOrderFilledQty ERROR: %v\n", err)
	} else {
		fmt.Printf("Filled: %.6f\n", filled)
	}

	fmt.Println("\n=== 6. Check position ===")
	positions, err := bn.GetPosition("BTCUSDT")
	if err != nil {
		fmt.Printf("GetPosition ERROR: %v\n", err)
	} else if len(positions) == 0 {
		fmt.Println("No positions found")
	} else {
		for _, p := range positions {
			fmt.Printf("  %s %s size=%s entry=%s upl=%s\n", p.Symbol, p.HoldSide, p.Total, p.AverageOpenPrice, p.UnrealizedPL)
		}
	}

	fmt.Println("\n=== 7. Market sell 0.002 BTC (close long) ===")
	oid3, err := bn.PlaceOrder(exchange.PlaceOrderParams{
		Symbol:     "BTCUSDT",
		Side:       exchange.SideSell,
		OrderType:  "market",
		Size:       "0.002",
		Force:      "gtc",
		ReduceOnly: true,
	})
	if err != nil {
		fmt.Printf("Market sell ERROR: %v\n", err)
		return
	}
	fmt.Printf("Market sell order: %s\n", oid3)
	time.Sleep(2 * time.Second)

	fmt.Println("\n=== 8. Verify position closed ===")
	positions, err = bn.GetPosition("BTCUSDT")
	if err != nil {
		fmt.Printf("GetPosition ERROR: %v\n", err)
	} else if len(positions) == 0 {
		fmt.Println("Position closed (no positions)")
	} else {
		for _, p := range positions {
			fmt.Printf("  %s %s size=%s\n", p.Symbol, p.HoldSide, p.Total)
		}
	}

	fmt.Println("\n=== Done ===")
}
