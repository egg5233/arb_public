package main

import (
	"fmt"

	"arb/internal/config"
	"arb/pkg/exchange"
	"arb/pkg/exchange/binance"
	"arb/pkg/exchange/bingx"
	"arb/pkg/exchange/bitget"
	"arb/pkg/exchange/bybit"
	"arb/pkg/exchange/gateio"
	"arb/pkg/exchange/okx"
)

func main() {
	cfg := config.Load()

	fmt.Printf("%-10s  %-10s  %10s  %10s  %10s\n", "Exchange", "Account", "Available", "Total", "Frozen")
	fmt.Println("--------------------------------------------------------------")

	for _, name := range cfg.EnabledExchanges() {
		exc := makeAdapter(cfg, name)
		if exc == nil {
			continue
		}

		futBal, futErr := exc.GetFuturesBalance()
		spotBal, spotErr := exc.GetSpotBalance()

		if futErr != nil {
			fmt.Printf("%-10s  %-10s  ERROR: %v\n", name, "futures", futErr)
		} else {
			fmt.Printf("%-10s  %-10s  %10.4f  %10.4f  %10.4f\n", name, "futures", futBal.Available, futBal.Total, futBal.Frozen)
		}

		if spotErr != nil {
			fmt.Printf("%-10s  %-10s  ERROR: %v\n", name, "spot", spotErr)
		} else {
			fmt.Printf("%-10s  %-10s  %10.4f  %10.4f  %10.4f\n", name, "spot", spotBal.Available, spotBal.Total, spotBal.Frozen)
		}
	}
}

func makeAdapter(cfg *config.Config, name string) exchange.Exchange {
	switch name {
	case "binance":
		return binance.NewAdapter(exchange.ExchangeConfig{Exchange: "binance", ApiKey: cfg.BinanceAPIKey, SecretKey: cfg.BinanceSecretKey})
	case "bitget":
		return bitget.NewAdapter(exchange.ExchangeConfig{Exchange: "bitget", ApiKey: cfg.BitgetAPIKey, SecretKey: cfg.BitgetSecretKey, Passphrase: cfg.BitgetPassphrase})
	case "bybit":
		return bybit.NewAdapter(exchange.ExchangeConfig{Exchange: "bybit", ApiKey: cfg.BybitAPIKey, SecretKey: cfg.BybitSecretKey})
	case "gateio":
		return gateio.NewAdapter(exchange.ExchangeConfig{Exchange: "gateio", ApiKey: cfg.GateioAPIKey, SecretKey: cfg.GateioSecretKey})
	case "okx":
		return okx.NewAdapter(exchange.ExchangeConfig{Exchange: "okx", ApiKey: cfg.OKXAPIKey, SecretKey: cfg.OKXSecretKey, Passphrase: cfg.OKXPassphrase})
	case "bingx":
		return bingx.NewAdapter(exchange.ExchangeConfig{Exchange: "bingx", ApiKey: cfg.BingXAPIKey, SecretKey: cfg.BingXSecretKey})
	default:
		return nil
	}
}
