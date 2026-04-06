package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"arb/internal/models"
	"arb/pkg/exchange"
)

type fundingStubExchange struct {
	name string
	fees []exchange.FundingPayment
}

func (s fundingStubExchange) Name() string { return s.name }
func (s fundingStubExchange) SetMetricsCallback(exchange.MetricsCallback) {}
func (s fundingStubExchange) PlaceOrder(exchange.PlaceOrderParams) (string, error) { return "", nil }
func (s fundingStubExchange) CancelOrder(string, string) error { return nil }
func (s fundingStubExchange) GetPendingOrders(string) ([]exchange.Order, error) { return nil, nil }
func (s fundingStubExchange) GetOrderFilledQty(string, string) (float64, error) { return 0, nil }
func (s fundingStubExchange) GetPosition(string) ([]exchange.Position, error) { return nil, nil }
func (s fundingStubExchange) GetAllPositions() ([]exchange.Position, error) { return nil, nil }
func (s fundingStubExchange) SetLeverage(string, string, string) error { return nil }
func (s fundingStubExchange) SetMarginMode(string, string) error { return nil }
func (s fundingStubExchange) LoadAllContracts() (map[string]exchange.ContractInfo, error) { return nil, nil }
func (s fundingStubExchange) GetFundingRate(string) (*exchange.FundingRate, error) { return nil, nil }
func (s fundingStubExchange) GetFundingInterval(string) (time.Duration, error) { return 0, nil }
func (s fundingStubExchange) GetFuturesBalance() (*exchange.Balance, error) { return nil, nil }
func (s fundingStubExchange) GetSpotBalance() (*exchange.Balance, error) { return nil, nil }
func (s fundingStubExchange) Withdraw(exchange.WithdrawParams) (*exchange.WithdrawResult, error) { return nil, nil }
func (s fundingStubExchange) WithdrawFeeInclusive() bool { return false }
func (s fundingStubExchange) GetWithdrawFee(string, string) (float64, error) { return 0, nil }
func (s fundingStubExchange) TransferToSpot(string, string) error { return nil }
func (s fundingStubExchange) TransferToFutures(string, string) error { return nil }
func (s fundingStubExchange) GetOrderbook(string, int) (*exchange.Orderbook, error) { return nil, nil }
func (s fundingStubExchange) StartPriceStream([]string) {}
func (s fundingStubExchange) SubscribeSymbol(string) bool { return false }
func (s fundingStubExchange) GetBBO(string) (exchange.BBO, bool) { return exchange.BBO{}, false }
func (s fundingStubExchange) GetPriceStore() *sync.Map { return &sync.Map{} }
func (s fundingStubExchange) SubscribeDepth(string) bool { return false }
func (s fundingStubExchange) UnsubscribeDepth(string) bool { return false }
func (s fundingStubExchange) GetDepth(string) (*exchange.Orderbook, bool) { return nil, false }
func (s fundingStubExchange) StartPrivateStream() {}
func (s fundingStubExchange) GetOrderUpdate(string) (exchange.OrderUpdate, bool) { return exchange.OrderUpdate{}, false }
func (s fundingStubExchange) SetOrderCallback(func(exchange.OrderUpdate)) {}
func (s fundingStubExchange) PlaceStopLoss(exchange.StopLossParams) (string, error) { return "", nil }
func (s fundingStubExchange) CancelStopLoss(string, string) error                  { return nil }
func (s fundingStubExchange) PlaceTakeProfit(exchange.TakeProfitParams) (string, error) {
	return "", nil
}
func (s fundingStubExchange) CancelTakeProfit(string, string) error { return nil }
func (s fundingStubExchange) GetUserTrades(string, time.Time, int) ([]exchange.Trade, error) { return nil, nil }
func (s fundingStubExchange) GetFundingFees(string, time.Time) ([]exchange.FundingPayment, error) { return s.fees, nil }
func (s fundingStubExchange) GetClosePnL(string, time.Time) ([]exchange.ClosePnL, error) { return nil, nil }
func (s fundingStubExchange) EnsureOneWayMode() error { return nil }
func (s fundingStubExchange) Close() {}

func TestHandleGetPositionFundingIncludesRotatedAwayBybitWindow(t *testing.T) {
	s, mr := newTestServer(t)
	defer mr.Close()

	createdAt := time.Date(2026, 4, 4, 0, 0, 0, 0, time.UTC)
	rotationAt := time.Date(2026, 4, 4, 6, 0, 0, 0, time.UTC)

	pos := &models.ArbitragePosition{
		ID:            "pos-1",
		Symbol:        "BTCUSDT",
		LongExchange:  "binance",
		ShortExchange: "okx",
		Status:        models.StatusActive,
		CreatedAt:     createdAt,
		RotationHistory: []models.RotationRecord{
			{
				From:      "bybit",
				To:        "binance",
				LegSide:   "long",
				Timestamp: rotationAt,
			},
		},
	}
	if err := s.db.SavePosition(pos); err != nil {
		t.Fatalf("SavePosition: %v", err)
	}

	s.exchanges = map[string]exchange.Exchange{
		"bybit": fundingStubExchange{
			name: "bybit",
			fees: []exchange.FundingPayment{
				{Amount: 1.25, Time: time.Date(2026, 4, 4, 5, 0, 0, 0, time.UTC)},
				{Amount: 1.50, Time: time.Date(2026, 4, 4, 6, 0, 0, 0, time.UTC)},
				{Amount: 1.75, Time: time.Date(2026, 4, 4, 7, 0, 0, 0, time.UTC)},
			},
		},
		"binance": fundingStubExchange{name: "binance"},
		"okx":     fundingStubExchange{name: "okx"},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/positions/pos-1/funding", nil)
	req.SetPathValue("id", "pos-1")
	w := httptest.NewRecorder()
	s.handleGetPositionFunding(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp struct {
		OK   bool `json:"ok"`
		Data []struct {
			Exchange string    `json:"exchange"`
			Side     string    `json:"side"`
			Amount   float64   `json:"amount"`
			Time     time.Time `json:"time"`
		} `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.OK {
		t.Fatal("expected ok=true")
	}

	if len(resp.Data) != 2 {
		t.Fatalf("expected 2 bybit events through rotation boundary, got %d", len(resp.Data))
	}
	if resp.Data[0].Exchange != "bybit" || resp.Data[0].Amount != 1.25 {
		t.Fatalf("first event = %+v, want bybit 1.25", resp.Data[0])
	}
	if !resp.Data[1].Time.Equal(rotationAt) || resp.Data[1].Amount != 1.50 {
		t.Fatalf("second event = %+v, want bybit settlement at rotation time", resp.Data[1])
	}
}
