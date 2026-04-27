package engine

import (
	"errors"
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
