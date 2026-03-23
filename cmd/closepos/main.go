// closepos closes a specific position by placing reduce-only market IOC orders
// on both exchanges.
//
// Usage: go run ./cmd/closepos/ --symbol PIPPINUSDT --long gateio --short okx --size 230
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"arb/internal/config"
	"arb/pkg/exchange"
	"arb/pkg/exchange/binance"
	"arb/pkg/exchange/bitget"
	"arb/pkg/exchange/bybit"
	"arb/pkg/exchange/gateio"
	"arb/pkg/exchange/okx"
	"arb/pkg/utils"
)

func main() {
	symbol := flag.String("symbol", "", "symbol to close (e.g. PIPPINUSDT)")
	longExch := flag.String("long", "", "long exchange")
	shortExch := flag.String("short", "", "short exchange")
	size := flag.String("size", "", "size in base units")
	flag.Parse()

	if *symbol == "" || *longExch == "" || *shortExch == "" || *size == "" {
		fmt.Fprintln(os.Stderr, "Usage: go run ./cmd/closepos/ --symbol PIPPINUSDT --long gateio --short okx --size 230")
		os.Exit(1)
	}

	log := utils.NewLogger("closepos")
	cfg := config.Load()

	exchanges := map[string]exchange.Exchange{}
	for _, name := range []string{*longExch, *shortExch} {
		if _, exists := exchanges[name]; exists {
			continue
		}
		exc, err := makeAdapter(cfg, name)
		if err != nil {
			log.Error("%s: %v", name, err)
			os.Exit(1)
		}
		exc.LoadAllContracts()
		exc.StartPrivateStream()
		exchanges[name] = exc
	}
	time.Sleep(2 * time.Second)

	// Close long (sell)
	log.Info("closing long %s on %s size=%s", *symbol, *longExch, *size)
	oid, err := exchanges[*longExch].PlaceOrder(exchange.PlaceOrderParams{
		Symbol: *symbol, Side: exchange.SideSell, OrderType: "market",
		Size: *size, Force: "ioc", ReduceOnly: true,
	})
	if err != nil {
		log.Error("long close failed: %v", err)
	} else {
		log.Info("long close order: %s", oid)
	}

	// Close short (buy)
	log.Info("closing short %s on %s size=%s", *symbol, *shortExch, *size)
	oid, err = exchanges[*shortExch].PlaceOrder(exchange.PlaceOrderParams{
		Symbol: *symbol, Side: exchange.SideBuy, OrderType: "market",
		Size: *size, Force: "ioc", ReduceOnly: true,
	})
	if err != nil {
		log.Error("short close failed: %v", err)
	} else {
		log.Info("short close order: %s", oid)
	}

	time.Sleep(3 * time.Second)

	// Verify
	for _, name := range []string{*longExch, *shortExch} {
		pos, err := exchanges[name].GetPosition(*symbol)
		if err != nil {
			log.Error("%s GetPosition: %v", name, err)
			continue
		}
		if len(pos) == 0 {
			log.Info("%s: position closed (0 positions)", name)
		} else {
			for _, p := range pos {
				log.Warn("%s: remaining %s %s size=%s", name, p.Symbol, p.HoldSide, p.Total)
			}
		}
	}
}

func makeAdapter(cfg *config.Config, name string) (exchange.Exchange, error) {
	switch name {
	case "binance":
		return binance.NewAdapter(exchange.ExchangeConfig{
			Exchange: "binance", ApiKey: cfg.BinanceAPIKey, SecretKey: cfg.BinanceSecretKey,
		}), nil
	case "bitget":
		return bitget.NewAdapter(exchange.ExchangeConfig{
			Exchange: "bitget", ApiKey: cfg.BitgetAPIKey, SecretKey: cfg.BitgetSecretKey, Passphrase: cfg.BitgetPassphrase,
		}), nil
	case "bybit":
		return bybit.NewAdapter(exchange.ExchangeConfig{
			Exchange: "bybit", ApiKey: cfg.BybitAPIKey, SecretKey: cfg.BybitSecretKey,
		}), nil
	case "gateio":
		return gateio.NewAdapter(exchange.ExchangeConfig{
			Exchange: "gateio", ApiKey: cfg.GateioAPIKey, SecretKey: cfg.GateioSecretKey,
		}), nil
	case "okx":
		return okx.NewAdapter(exchange.ExchangeConfig{
			Exchange: "okx", ApiKey: cfg.OKXAPIKey, SecretKey: cfg.OKXSecretKey, Passphrase: cfg.OKXPassphrase,
		}), nil
	}
	return nil, fmt.Errorf("unknown exchange: %s", name)
}
