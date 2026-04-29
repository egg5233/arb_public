package engine

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	"arb/pkg/exchange"
)

type bingXPreflightStub struct {
	*fullStubExchange
	err   error
	calls []exchange.PlaceOrderParams
}

func (s *bingXPreflightStub) TestOrder(p exchange.PlaceOrderParams) error {
	s.calls = append(s.calls, p)
	return s.err
}

func TestPreflightBingXEntryOrderHardRejectsAPIOrdersDisabled(t *testing.T) {
	stub := &bingXPreflightStub{
		fullStubExchange: newFullStub(exchange.BBO{}, false),
		err: errors.New("bingx API error code=109400 msg=Reminder: Due to the large market fluctuations, " +
			"in order to reduce the risk of liquidation, API orders are temporarily disabled."),
	}

	var e Engine
	err := e.preflightBingXEntryOrder(stub, "bingx", "SPORTFUNUSDT", exchange.SideSell, 0.003733, "1000")
	if err == nil {
		t.Fatal("preflight returned nil, want hard reject")
	}
	if !strings.Contains(err.Error(), "bingx preflight hard reject for SPORTFUNUSDT") {
		t.Fatalf("error = %q, want hard reject context", err.Error())
	}
	if len(stub.calls) != 1 {
		t.Fatalf("TestOrder calls = %d, want 1", len(stub.calls))
	}

	call := stub.calls[0]
	if call.Symbol != "SPORTFUNUSDT" ||
		call.Side != exchange.SideSell ||
		call.OrderType != "limit" ||
		call.Price != "0.00742867" ||
		call.Size != "1000" ||
		call.Force != "ioc" {
		t.Fatalf("TestOrder params = %+v", call)
	}
	if call.ReduceOnly {
		t.Fatal("entry preflight TestOrder must not send reduceOnly")
	}
}

func TestPreflightBingXEntryOrderSkipsNonBingX(t *testing.T) {
	stub := &bingXPreflightStub{
		fullStubExchange: newFullStub(exchange.BBO{}, false),
		err:              errors.New("should not be called"),
	}

	var e Engine
	err := e.preflightBingXEntryOrder(stub, "bybit", "SPORTFUNUSDT", exchange.SideSell, 0.003733, "1000")
	if err != nil {
		t.Fatalf("preflight returned error for non-BingX exchange: %v", err)
	}
	if len(stub.calls) != 0 {
		t.Fatalf("TestOrder calls = %d, want 0", len(stub.calls))
	}
}

func TestPreflightBingXEntryOrderRaisesBuyProbeToBingXMinNotional(t *testing.T) {
	stub := &bingXPreflightStub{
		fullStubExchange: newFullStub(exchange.BBO{}, false),
	}

	var e Engine
	err := e.preflightBingXEntryOrder(stub, "bingx", "SKYAIUSDT", exchange.SideBuy, 0.23056, "352")
	if err != nil {
		t.Fatalf("preflight returned error: %v", err)
	}
	if len(stub.calls) != 1 {
		t.Fatalf("TestOrder calls = %d, want 1", len(stub.calls))
	}

	call := stub.calls[0]
	price, err := strconv.ParseFloat(call.Price, 64)
	if err != nil {
		t.Fatalf("probe price %q parse: %v", call.Price, err)
	}
	size, err := strconv.ParseFloat(call.Size, 64)
	if err != nil {
		t.Fatalf("probe size %q parse: %v", call.Size, err)
	}
	if price >= 0.23056 {
		t.Fatalf("probe price %.8f must stay non-marketable below ref", price)
	}
	if price <= 0.23056*0.20 {
		t.Fatalf("probe price %.8f must stay above BingX far-price floor", price)
	}
	if price*size < bingXMinProbeNotionalUSDT {
		t.Fatalf("probe notional %.8f < %.2f", price*size, bingXMinProbeNotionalUSDT)
	}
}

func TestPreflightBingXEntryOrderRejectsMinNotionalProbeThatWouldMarketBuy(t *testing.T) {
	stub := &bingXPreflightStub{
		fullStubExchange: newFullStub(exchange.BBO{}, false),
	}

	var e Engine
	err := e.preflightBingXEntryOrder(stub, "bingx", "TINYUSDT", exchange.SideBuy, 0.5, "1")
	if err == nil {
		t.Fatal("preflight returned nil, want min-notional probe error")
	}
	if !strings.Contains(err.Error(), "cannot stay non-marketable") {
		t.Fatalf("error = %q, want non-marketable min-notional context", err.Error())
	}
	if len(stub.calls) != 0 {
		t.Fatalf("TestOrder calls = %d, want 0", len(stub.calls))
	}
}
