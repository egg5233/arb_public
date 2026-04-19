package spotengine

import (
	"context"
	"fmt"
	"testing"
	"time"

	"arb/internal/config"
	"arb/internal/database"
	"arb/pkg/exchange"
	"arb/pkg/utils"

	"github.com/alicebob/miniredis/v2"
)

// priceGapStubExchange returns fixed futures and spot BBOs for price-gap tests.
type priceGapStubExchange struct {
	priceStubExchange
	futBid  float64
	futAsk  float64
	spotBid float64
	spotAsk float64
	futErr  error
	spotErr error
}

func (s priceGapStubExchange) GetOrderbook(string, int) (*exchange.Orderbook, error) {
	if s.futErr != nil {
		return nil, s.futErr
	}
	return &exchange.Orderbook{
		Bids: []exchange.PriceLevel{{Price: s.futBid, Quantity: 1}},
		Asks: []exchange.PriceLevel{{Price: s.futAsk, Quantity: 1}},
	}, nil
}

func (s priceGapStubExchange) GetSpotBBO(string) (exchange.BBO, error) {
	if s.spotErr != nil {
		return exchange.BBO{}, s.spotErr
	}
	return exchange.BBO{Bid: s.spotBid, Ask: s.spotAsk}, nil
}

func (s priceGapStubExchange) MarginBorrow(exchange.MarginBorrowParams) error { return nil }
func (s priceGapStubExchange) MarginRepay(exchange.MarginRepayParams) error   { return nil }
func (s priceGapStubExchange) PlaceSpotMarginOrder(exchange.SpotMarginOrderParams) (string, error) {
	return "", nil
}
func (s priceGapStubExchange) GetMarginInterestRate(string) (*exchange.MarginInterestRate, error) {
	return nil, nil
}
func (s priceGapStubExchange) GetMarginBalance(string) (*exchange.MarginBalance, error) {
	return &exchange.MarginBalance{}, nil
}
func (s priceGapStubExchange) TransferToMargin(string, string) error   { return nil }
func (s priceGapStubExchange) TransferFromMargin(string, string) error { return nil }
func (s priceGapStubExchange) GetMarginInterestRateHistory(_ context.Context, _ string, _, _ time.Time) ([]exchange.MarginInterestRatePoint, error) {
	return nil, exchange.ErrHistoricalBorrowNotSupported
}
func (s priceGapStubExchange) CancelAllOrders(string) error { return nil }
func (s priceGapStubExchange) SpotOrderRules(string) (*exchange.SpotOrderRules, error) {
	return nil, nil
}

func newPriceGapGateEngine(t *testing.T, cfg *config.Config, stub priceGapStubExchange) (*SpotEngine, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	db, err := database.New(mr.Addr(), "", 0)
	if err != nil {
		mr.Close()
		t.Fatalf("database.New: %v", err)
	}
	exchanges := map[string]exchange.Exchange{"testexch": stub}
	spotMargin := map[string]exchange.SpotMarginExchange{"testexch": stub}
	return &SpotEngine{
		cfg:        cfg,
		db:         db,
		exchanges:  exchanges,
		spotMargin: spotMargin,
		log:        utils.NewLogger("test"),
	}, mr
}

func TestPriceGapGateEntry(t *testing.T) {
	tests := []struct {
		name       string
		enabled    bool
		direction  string
		futBid     float64
		futAsk     float64
		spotBid    float64
		spotAsk    float64
		maxGap     float64
		futErr     error
		spotErr    error
		wantReason string
	}{
		{
			name:       "dir A gap exceeds threshold",
			enabled:    true,
			direction:  "borrow_sell_long",
			futBid:     100.8,
			futAsk:     101.0,
			spotBid:    100.0,
			spotAsk:    100.1,
			maxGap:     0.5,
			wantReason: "price_gap_1.00%>0.50%",
		},
		{
			name:       "dir B gap exceeds threshold",
			enabled:    true,
			direction:  "buy_spot_short",
			futBid:     100.0,
			futAsk:     100.1,
			spotBid:    100.8,
			spotAsk:    101.0,
			maxGap:     0.5,
			wantReason: "price_gap_1.00%>0.50%",
		},
		{
			name:       "gap at threshold allowed",
			enabled:    true,
			direction:  "borrow_sell_long",
			futBid:     100.4,
			futAsk:     100.5,
			spotBid:    100.0,
			spotAsk:    100.1,
			maxGap:     0.5,
			wantReason: "",
		},
		{
			name:       "gate disabled",
			enabled:    false,
			direction:  "borrow_sell_long",
			futBid:     100.8,
			futAsk:     101.0,
			spotBid:    100.0,
			spotAsk:    100.1,
			maxGap:     0.5,
			wantReason: "",
		},
		{
			name:       "futures error fails closed",
			enabled:    true,
			direction:  "borrow_sell_long",
			futBid:     100.0,
			futAsk:     100.1,
			spotBid:    100.0,
			spotAsk:    100.1,
			maxGap:     0.5,
			futErr:     fmt.Errorf("connection timeout"),
			wantReason: "price_gap_check_error",
		},
		{
			name:       "spot error fails closed",
			enabled:    true,
			direction:  "buy_spot_short",
			futBid:     100.0,
			futAsk:     100.1,
			spotBid:    100.0,
			spotAsk:    100.1,
			maxGap:     0.5,
			spotErr:    fmt.Errorf("spot timeout"),
			wantReason: "price_gap_check_error",
		},
		{
			name:       "default threshold when config is zero",
			enabled:    true,
			direction:  "borrow_sell_long",
			futBid:     100.8,
			futAsk:     101.0,
			spotBid:    100.0,
			spotAsk:    100.1,
			maxGap:     0,
			wantReason: "price_gap_1.00%>0.50%",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.Config{
				SpotFuturesEnablePriceGapGate: tc.enabled,
				SpotFuturesMaxPriceGapPct:     tc.maxGap,
				SpotFuturesMaxPositions:       5,
				SpotFuturesPersistenceScans:   0,
			}
			eng, mr := newPriceGapGateEngine(t, cfg, priceGapStubExchange{
				futBid:  tc.futBid,
				futAsk:  tc.futAsk,
				spotBid: tc.spotBid,
				spotAsk: tc.spotAsk,
				futErr:  tc.futErr,
				spotErr: tc.spotErr,
			})
			defer mr.Close()

			result := eng.checkRiskGate(SpotArbOpportunity{
				Symbol:    "BTCUSDT",
				Exchange:  "testexch",
				Direction: tc.direction,
				NetAPR:    0.5,
			})

			if tc.wantReason == "" {
				if !result.Allowed {
					t.Fatalf("expected allowed, got blocked: %q", result.Reason)
				}
				return
			}
			if result.Allowed {
				t.Fatal("expected blocked, got allowed")
			}
			if result.Reason != tc.wantReason {
				t.Fatalf("reason = %q, want %q", result.Reason, tc.wantReason)
			}
		})
	}
}

func TestPriceGapGateBeforeDryRun(t *testing.T) {
	cfg := &config.Config{
		SpotFuturesEnablePriceGapGate: true,
		SpotFuturesMaxPriceGapPct:     0.5,
		SpotFuturesDryRun:             true,
		SpotFuturesMaxPositions:       5,
		SpotFuturesPersistenceScans:   0,
	}
	eng, mr := newPriceGapGateEngine(t, cfg, priceGapStubExchange{
		futBid:  100.8,
		futAsk:  101.0,
		spotBid: 100.0,
		spotAsk: 100.1,
	})
	defer mr.Close()

	result := eng.checkRiskGate(SpotArbOpportunity{
		Symbol:    "BTCUSDT",
		Exchange:  "testexch",
		Direction: "borrow_sell_long",
		NetAPR:    0.5,
	})
	if result.Allowed {
		t.Fatal("expected blocked, got allowed")
	}
	if result.Reason != "price_gap_1.00%>0.50%" {
		t.Fatalf("expected price-gap rejection before dry_run, got %q", result.Reason)
	}
}

func TestPriceGapGateDryRunPassthrough(t *testing.T) {
	cfg := &config.Config{
		SpotFuturesEnablePriceGapGate: true,
		SpotFuturesMaxPriceGapPct:     0.5,
		SpotFuturesDryRun:             true,
		SpotFuturesMaxPositions:       5,
		SpotFuturesPersistenceScans:   0,
	}
	eng, mr := newPriceGapGateEngine(t, cfg, priceGapStubExchange{
		futBid:  100.0,
		futAsk:  100.2,
		spotBid: 100.0,
		spotAsk: 100.1,
	})
	defer mr.Close()

	result := eng.checkRiskGate(SpotArbOpportunity{
		Symbol:    "BTCUSDT",
		Exchange:  "testexch",
		Direction: "borrow_sell_long",
		NetAPR:    0.5,
	})
	if result.Allowed {
		t.Fatal("expected dry_run block, got allowed")
	}
	if result.Reason != "dry_run" {
		t.Fatalf("expected dry_run reason, got %q", result.Reason)
	}
}

func TestCalculateEntryPriceGap(t *testing.T) {
	eng := &SpotEngine{
		exchanges: map[string]exchange.Exchange{
			"testexch": priceGapStubExchange{futBid: 100.0, futAsk: 101.0, spotBid: 99.0, spotAsk: 100.5},
		},
		spotMargin: map[string]exchange.SpotMarginExchange{
			"testexch": priceGapStubExchange{futBid: 100.0, futAsk: 101.0, spotBid: 99.0, spotAsk: 100.5},
		},
		log: utils.NewLogger("test"),
	}

	t.Run("dir A", func(t *testing.T) {
		got, err := eng.calculateEntryPriceGap("BTCUSDT", "testexch", "borrow_sell_long")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := (101.0 - 99.0) / 99.0 * 100
		if diff := got - want; diff > 0.01 || diff < -0.01 {
			t.Fatalf("gap = %.4f%%, want %.4f%%", got, want)
		}
	})

	t.Run("dir B", func(t *testing.T) {
		got, err := eng.calculateEntryPriceGap("BTCUSDT", "testexch", "buy_spot_short")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := (100.5 - 100.0) / 100.0 * 100
		if diff := got - want; diff > 0.01 || diff < -0.01 {
			t.Fatalf("gap = %.4f%%, want %.4f%%", got, want)
		}
	})
}

func TestCalculateEntryPriceGap_ExchangeNotFound(t *testing.T) {
	eng := &SpotEngine{
		exchanges:  map[string]exchange.Exchange{},
		spotMargin: map[string]exchange.SpotMarginExchange{},
		log:        utils.NewLogger("test"),
	}

	if _, err := eng.calculateEntryPriceGap("BTCUSDT", "missing", "borrow_sell_long"); err == nil {
		t.Fatal("expected error for missing exchange")
	}
}
