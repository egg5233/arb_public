package spotengine

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"arb/internal/api"
	"arb/internal/config"
	"arb/internal/database"
	"arb/internal/models"
	"arb/internal/risk"
	"arb/pkg/exchange"
	"arb/pkg/utils"

	"github.com/alicebob/miniredis/v2"
)

// buildFakeReservation constructs a risk.CapitalReservation for tests that
// feed OpenSelectedEntry a preheld reservation. The SpotEngine's allocator
// is nil in these tests so commit/release are no-ops; the structure just
// needs enough shape for OpenSelectedEntry's commit helper to find the
// exchange key.
func buildFakeReservation(exchangeName string, amount float64) *risk.CapitalReservation {
	return &risk.CapitalReservation{
		ID:        "test-reservation",
		Strategy:  risk.StrategySpotFutures,
		Exposures: map[string]float64{exchangeName: amount},
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(5 * time.Minute),
	}
}

// ---------------------------------------------------------------------------
// Selector test stubs
// ---------------------------------------------------------------------------
//
// These fake exchanges implement only what BuildEntryPlan / OpenSelectedEntry
// actually call. BuildEntryPlan needs GetOrderbook + LoadAllContracts and,
// for Dir A on unified accounts, GetMarginBalance. OpenSelectedEntry layers
// on order placement, transfer, and leverage calls.

type selectorFutExchange struct {
	name         string
	orderbook    *exchange.Orderbook
	contracts    map[string]exchange.ContractInfo
	placeCalls   int
	placeOrderID string
	orderUpdate  exchange.OrderUpdate
	transfers    []string
}

func (s *selectorFutExchange) Name() string                                { return s.name }
func (s *selectorFutExchange) SetMetricsCallback(exchange.MetricsCallback) {}
func (s *selectorFutExchange) PlaceOrder(exchange.PlaceOrderParams) (string, error) {
	s.placeCalls++
	id := s.placeOrderID
	if id == "" {
		id = "fut-sel-1"
	}
	return id, nil
}
func (s *selectorFutExchange) CancelOrder(string, string) error                  { return nil }
func (s *selectorFutExchange) CancelAllOrders(string) error                      { return nil }
func (s *selectorFutExchange) GetPendingOrders(string) ([]exchange.Order, error) { return nil, nil }
func (s *selectorFutExchange) GetOrderFilledQty(string, string) (float64, error) { return 0, nil }
func (s *selectorFutExchange) GetPosition(string) ([]exchange.Position, error)   { return nil, nil }
func (s *selectorFutExchange) GetAllPositions() ([]exchange.Position, error)     { return nil, nil }
func (s *selectorFutExchange) SetLeverage(string, string, string) error          { return nil }
func (s *selectorFutExchange) SetMarginMode(string, string) error                { return nil }
func (s *selectorFutExchange) LoadAllContracts() (map[string]exchange.ContractInfo, error) {
	return s.contracts, nil
}
func (s *selectorFutExchange) GetFundingRate(string) (*exchange.FundingRate, error) { return nil, nil }
func (s *selectorFutExchange) GetFundingInterval(string) (time.Duration, error)     { return 0, nil }
func (s *selectorFutExchange) GetFuturesBalance() (*exchange.Balance, error) {
	return &exchange.Balance{}, nil
}
func (s *selectorFutExchange) GetSpotBalance() (*exchange.Balance, error) { return nil, nil }
func (s *selectorFutExchange) Withdraw(exchange.WithdrawParams) (*exchange.WithdrawResult, error) {
	return nil, nil
}
func (s *selectorFutExchange) TransferToSpot(coin, amount string) error {
	s.transfers = append(s.transfers, "spot:"+coin+":"+amount)
	return nil
}
func (s *selectorFutExchange) TransferToFutures(string, string) error { return nil }
func (s *selectorFutExchange) GetOrderbook(string, int) (*exchange.Orderbook, error) {
	return s.orderbook, nil
}
func (s *selectorFutExchange) StartPriceStream([]string)                   {}
func (s *selectorFutExchange) SubscribeSymbol(string) bool                 { return false }
func (s *selectorFutExchange) GetBBO(string) (exchange.BBO, bool)          { return exchange.BBO{}, false }
func (s *selectorFutExchange) GetPriceStore() *sync.Map                    { return nil }
func (s *selectorFutExchange) SubscribeDepth(string) bool                  { return false }
func (s *selectorFutExchange) UnsubscribeDepth(string) bool                { return false }
func (s *selectorFutExchange) GetDepth(string) (*exchange.Orderbook, bool) { return nil, false }
func (s *selectorFutExchange) StartPrivateStream()                         {}
func (s *selectorFutExchange) GetOrderUpdate(orderID string) (exchange.OrderUpdate, bool) {
	if s.orderUpdate.OrderID == "" {
		return exchange.OrderUpdate{}, false
	}
	return s.orderUpdate, true
}
func (s *selectorFutExchange) SetOrderCallback(func(exchange.OrderUpdate)) {}
func (s *selectorFutExchange) PlaceStopLoss(exchange.StopLossParams) (string, error) {
	return "", nil
}
func (s *selectorFutExchange) CancelStopLoss(string, string) error { return nil }
func (s *selectorFutExchange) PlaceTakeProfit(exchange.TakeProfitParams) (string, error) {
	return "", nil
}
func (s *selectorFutExchange) CancelTakeProfit(string, string) error { return nil }
func (s *selectorFutExchange) GetUserTrades(string, time.Time, int) ([]exchange.Trade, error) {
	return nil, nil
}
func (s *selectorFutExchange) GetFundingFees(string, time.Time) ([]exchange.FundingPayment, error) {
	return nil, nil
}
func (s *selectorFutExchange) GetClosePnL(string, time.Time) ([]exchange.ClosePnL, error) {
	return nil, nil
}
func (s *selectorFutExchange) WithdrawFeeInclusive() bool { return false }
func (s *selectorFutExchange) GetWithdrawFee(string, string) (float64, float64, error) {
	return 0, 0, nil
}
func (s *selectorFutExchange) EnsureOneWayMode() error { return nil }
func (s *selectorFutExchange) Close()                  {}

type selectorSpotMargin struct {
	marginBal         *exchange.MarginBalance
	marginBalErr      error
	placeOrderID      string
	placeCalls        int
	queryStates       []*exchange.SpotMarginOrderStatus
	transfersToMargin []string
	transferToMargin  func(coin, amount string) error
}

func (s *selectorSpotMargin) MarginBorrow(exchange.MarginBorrowParams) error { return nil }
func (s *selectorSpotMargin) MarginRepay(exchange.MarginRepayParams) error   { return nil }
func (s *selectorSpotMargin) PlaceSpotMarginOrder(exchange.SpotMarginOrderParams) (string, error) {
	s.placeCalls++
	id := s.placeOrderID
	if id == "" {
		id = "spot-sel-1"
	}
	return id, nil
}
func (s *selectorSpotMargin) GetMarginInterestRate(string) (*exchange.MarginInterestRate, error) {
	return nil, nil
}
func (s *selectorSpotMargin) GetMarginBalance(string) (*exchange.MarginBalance, error) {
	if s.marginBalErr != nil {
		return nil, s.marginBalErr
	}
	if s.marginBal != nil {
		return s.marginBal, nil
	}
	return &exchange.MarginBalance{}, nil
}
func (s *selectorSpotMargin) GetSpotBBO(string) (exchange.BBO, error) {
	return exchange.BBO{Bid: 100, Ask: 100.1}, nil
}
func (s *selectorSpotMargin) TransferToMargin(coin, amount string) error {
	s.transfersToMargin = append(s.transfersToMargin, coin+":"+amount)
	if s.transferToMargin != nil {
		return s.transferToMargin(coin, amount)
	}
	return nil
}
func (s *selectorSpotMargin) TransferFromMargin(string, string) error { return nil }
func (s *selectorSpotMargin) GetSpotMarginOrder(string, string) (*exchange.SpotMarginOrderStatus, error) {
	if len(s.queryStates) == 0 {
		return nil, nil
	}
	st := s.queryStates[0]
	s.queryStates = s.queryStates[1:]
	return st, nil
}

// ---------------------------------------------------------------------------
// Harness
// ---------------------------------------------------------------------------

func newSelectorTestEngine(t *testing.T, cfg *config.Config) (*SpotEngine, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(func() { mr.Close() })
	db, err := database.New(mr.Addr(), "", 0)
	if err != nil {
		t.Fatalf("database.New: %v", err)
	}
	eng := &SpotEngine{
		cfg:        cfg,
		db:         db,
		log:        utils.NewLogger("sel-test"),
		exchanges:  map[string]exchange.Exchange{},
		spotMargin: map[string]exchange.SpotMarginExchange{},
		lastSeen:   make(map[string]bool),
		stopCh:     make(chan struct{}),
		exitState:  exitState{exiting: make(map[string]bool)},
	}
	eng.api = api.NewServer(db, cfg, nil)
	return eng, mr
}

func defaultTestCfg() *config.Config {
	return &config.Config{
		SpotFuturesLeverage:        3,
		SpotFuturesMaxPositions:    5,
		MaxPositions:               10,
		SpotFuturesCapitalUnified:  500,
		SpotFuturesCapitalSeparate: 200,
	}
}

func orderbookAt(price float64) *exchange.Orderbook {
	// BBO centered on `price` yields midPrice == price for deterministic math.
	return &exchange.Orderbook{
		Bids: []exchange.PriceLevel{{Price: price, Quantity: 10}},
		Asks: []exchange.PriceLevel{{Price: price, Quantity: 10}},
	}
}

func contractWith(symbol string, step, min float64, decimals int) map[string]exchange.ContractInfo {
	return map[string]exchange.ContractInfo{
		symbol: {
			Symbol:       symbol,
			StepSize:     step,
			MinSize:      min,
			SizeDecimals: decimals,
		},
	}
}

// ---------------------------------------------------------------------------
// ListEntryCandidates
// ---------------------------------------------------------------------------

func TestListEntryCandidates_FiltersFilterStatusAndStale(t *testing.T) {
	cfg := defaultTestCfg()
	eng, _ := newSelectorTestEngine(t, cfg)

	now := time.Now()
	eng.latestOpps = []SpotArbOpportunity{
		{ // fresh + passed
			Symbol:     "BTCUSDT",
			BaseCoin:   "BTC",
			Exchange:   "bybit",
			Direction:  "borrow_sell_long",
			FundingAPR: 0.1,
			BorrowAPR:  0.02,
			FeePct:     0.002,
			Timestamp:  now,
		},
		{ // filtered
			Symbol:       "ETHUSDT",
			BaseCoin:     "ETH",
			Exchange:     "bybit",
			Direction:    "buy_spot_short",
			Timestamp:    now,
			FilterStatus: "borrow_unavailable",
		},
		{ // stale
			Symbol:    "SOLUSDT",
			BaseCoin:  "SOL",
			Exchange:  "okx",
			Direction: "borrow_sell_long",
			Timestamp: now.Add(-2 * time.Hour),
		},
	}

	got := eng.ListEntryCandidates(10 * time.Minute)
	if len(got) != 1 {
		t.Fatalf("ListEntryCandidates = %d candidates, want 1 (fresh + passed only); got=%+v", len(got), got)
	}
	if got[0].Symbol != "BTCUSDT" {
		t.Fatalf("expected BTCUSDT survivor, got %s", got[0].Symbol)
	}
	if got[0].FundingAPR != 0.1 || got[0].BorrowAPR != 0.02 || got[0].FeePct != 0.002 {
		t.Fatalf("candidate projection lost APR/fee fields: %+v", got[0])
	}
}

func TestSpotOpenSelectedEntry_CrossStrategySymbolExclusion(t *testing.T) {
	cfg := defaultTestCfg()
	cfg.MaxPositions = 5
	eng, _ := newSelectorTestEngine(t, cfg)
	eng.exchanges["bybit"] = &selectorFutExchange{name: "bybit", orderbook: orderbookAt(100), contracts: contractWith("BTCUSDT", 0.001, 0.001, 3)}
	eng.spotMargin["bybit"] = &selectorSpotMargin{}

	if err := eng.db.SavePosition(&models.ArbitragePosition{
		ID:            "perp-1",
		Symbol:        "BTCUSDT",
		LongExchange:  "binance",
		ShortExchange: "okx",
		Status:        models.StatusActive,
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}); err != nil {
		t.Fatalf("SavePosition: %v", err)
	}

	plan := &models.SpotEntryPlan{
		Candidate: models.SpotEntryCandidate{
			Symbol:     "BTCUSDT",
			BaseCoin:   "BTC",
			Exchange:   "bybit",
			Direction:  "buy_spot_short",
			FundingAPR: 0.10,
		},
		MidPrice:            100,
		PlannedBaseSize:     1,
		PlannedNotionalUSDT: 100,
		CapitalBudgetUSDT:   100,
	}

	err := eng.OpenSelectedEntry(plan, 0, buildFakeReservation("bybit", 100))
	if err == nil || !strings.Contains(err.Error(), "already open") {
		t.Fatalf("OpenSelectedEntry error = %v, want cross-strategy duplicate rejection", err)
	}
}

// ---------------------------------------------------------------------------
// BuildEntryPlan — Dir A unified (Bybit / OKX / Gate.io)
// ---------------------------------------------------------------------------

func TestBuildEntryPlan_DirA_CapsToMaxBorrowableAndRoundsToFuturesStep(t *testing.T) {
	cfg := defaultTestCfg()
	cfg.SpotFuturesCapitalUnified = 500
	eng, _ := newSelectorTestEngine(t, cfg)

	fut := &selectorFutExchange{
		name:      "bybit",
		orderbook: orderbookAt(100),
		contracts: contractWith("BTCUSDT", 0.01, 0.001, 3),
	}
	sm := &selectorSpotMargin{
		marginBal: &exchange.MarginBalance{MaxBorrowable: 3.0}, // cap below budget/price (=5)
	}
	eng.exchanges["bybit"] = fut
	eng.spotMargin["bybit"] = sm

	c := models.SpotEntryCandidate{
		Symbol:    "BTCUSDT",
		BaseCoin:  "BTC",
		Exchange:  "bybit",
		Direction: "borrow_sell_long",
		Timestamp: time.Now(),
	}
	plan, err := eng.BuildEntryPlan(c)
	if err != nil {
		t.Fatalf("BuildEntryPlan: %v", err)
	}
	// rawSize = 500/100 = 5; capped at 3.0; step=0.01 → 3.0.
	if plan.PlannedBaseSize != 3.0 {
		t.Fatalf("PlannedBaseSize = %.6f, want 3.0 (cap=3.0, step=0.01)", plan.PlannedBaseSize)
	}
	// PlannedNotional = 3 * 100 = 300.
	if plan.PlannedNotionalUSDT != 300 {
		t.Fatalf("PlannedNotionalUSDT = %.2f, want 300", plan.PlannedNotionalUSDT)
	}
	if plan.MaxBorrowableBase != 3.0 {
		t.Fatalf("MaxBorrowableBase = %.6f, want 3.0 (unified Dir A must carry the cap)", plan.MaxBorrowableBase)
	}
	if plan.RequiresInternalTransfer {
		t.Fatalf("RequiresInternalTransfer = true, want false on unified bybit")
	}
	if plan.TransferTarget != "" {
		t.Fatalf("TransferTarget = %q, want empty on unified", plan.TransferTarget)
	}
	if plan.CapitalBudgetUSDT != 500 {
		t.Fatalf("CapitalBudgetUSDT = %.2f, want 500", plan.CapitalBudgetUSDT)
	}
	if plan.FuturesMarginUSDT != 100 { // 300 / leverage 3
		t.Fatalf("FuturesMarginUSDT = %.2f, want 100", plan.FuturesMarginUSDT)
	}
	if plan.MidPrice != 100 {
		t.Fatalf("MidPrice = %.2f, want 100", plan.MidPrice)
	}
}

func TestBuildEntryPlan_DirA_RejectsWhenRoundedSizeBelowMin(t *testing.T) {
	cfg := defaultTestCfg()
	cfg.SpotFuturesCapitalUnified = 500
	eng, _ := newSelectorTestEngine(t, cfg)

	fut := &selectorFutExchange{
		name:      "bybit",
		orderbook: orderbookAt(100),
		// futMin 10, step 0.01: rawSize=5 < futMin=10 → rejection
		contracts: contractWith("BTCUSDT", 0.01, 10, 3),
	}
	sm := &selectorSpotMargin{
		marginBal: &exchange.MarginBalance{MaxBorrowable: 100},
	}
	eng.exchanges["bybit"] = fut
	eng.spotMargin["bybit"] = sm

	c := models.SpotEntryCandidate{
		Symbol: "BTCUSDT", BaseCoin: "BTC", Exchange: "bybit",
		Direction: "borrow_sell_long", Timestamp: time.Now(),
	}
	_, err := eng.BuildEntryPlan(c)
	if err == nil {
		t.Fatal("expected rejection when rounded size below futures min")
	}
	if !strings.Contains(err.Error(), "below futures min") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildEntryPlan_DirA_RejectsWhenNotionalBelowBudgetFloor(t *testing.T) {
	cfg := defaultTestCfg()
	cfg.SpotFuturesCapitalUnified = 500
	eng, _ := newSelectorTestEngine(t, cfg)

	// rawSize = 500/100 = 5; MaxBorrowable caps to 0.1 so plannedBase=0.1.
	// notional = 0.1*100 = 10 < floor = max(500*0.10, 5) = 50 → reject.
	fut := &selectorFutExchange{
		name:      "bybit",
		orderbook: orderbookAt(100),
		contracts: contractWith("BTCUSDT", 0.01, 0.01, 3),
	}
	sm := &selectorSpotMargin{
		marginBal: &exchange.MarginBalance{MaxBorrowable: 0.1},
	}
	eng.exchanges["bybit"] = fut
	eng.spotMargin["bybit"] = sm

	c := models.SpotEntryCandidate{
		Symbol: "BTCUSDT", BaseCoin: "BTC", Exchange: "bybit",
		Direction: "borrow_sell_long", Timestamp: time.Now(),
	}
	_, err := eng.BuildEntryPlan(c)
	if err == nil {
		t.Fatal("expected rejection when notional below budget floor")
	}
	if !strings.Contains(err.Error(), "below floor") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// BuildEntryPlan — Dir B (all venues)
// ---------------------------------------------------------------------------

func TestBuildEntryPlan_DirB_NoBorrowNeeded(t *testing.T) {
	cfg := defaultTestCfg()
	eng, _ := newSelectorTestEngine(t, cfg)

	// Dir B must never consult GetMarginBalance — marginBalErr would explode.
	fut := &selectorFutExchange{
		name:      "okx",
		orderbook: orderbookAt(100),
		contracts: contractWith("BTCUSDT", 0.01, 0.001, 3),
	}
	sm := &selectorSpotMargin{
		marginBalErr: fmt.Errorf("GetMarginBalance must not be called for Dir B"),
	}
	eng.exchanges["okx"] = fut
	eng.spotMargin["okx"] = sm

	c := models.SpotEntryCandidate{
		Symbol: "BTCUSDT", BaseCoin: "BTC", Exchange: "okx",
		Direction: "buy_spot_short", Timestamp: time.Now(),
	}
	plan, err := eng.BuildEntryPlan(c)
	if err != nil {
		t.Fatalf("BuildEntryPlan: %v", err)
	}
	// rawSize = 500/100 = 5; step=0.01 → 5.0.
	if plan.PlannedBaseSize != 5.0 {
		t.Fatalf("PlannedBaseSize = %.6f, want 5.0", plan.PlannedBaseSize)
	}
	if plan.MaxBorrowableBase != 0 {
		t.Fatalf("MaxBorrowableBase = %.6f, want 0 for Dir B", plan.MaxBorrowableBase)
	}
	if plan.RequiresInternalTransfer {
		t.Fatalf("Dir B on okx unified must not require transfer")
	}
}

// ---------------------------------------------------------------------------
// BuildEntryPlan — transfer flag per venue
// ---------------------------------------------------------------------------

func TestBuildEntryPlan_BinanceBitget_SetInternalTransferFlag(t *testing.T) {
	for _, name := range []string{"binance", "bitget"} {
		t.Run(name, func(t *testing.T) {
			cfg := defaultTestCfg()
			eng, _ := newSelectorTestEngine(t, cfg)

			fut := &selectorFutExchange{
				name:      name,
				orderbook: orderbookAt(100),
				contracts: contractWith("BTCUSDT", 0.01, 0.001, 3),
			}
			// On separate accounts, selector MUST NOT call GetMarginBalance
			// (that poll happens post-transfer at execution time). Use an
			// error-throwing stub to enforce the contract.
			sm := &selectorSpotMargin{
				marginBalErr: fmt.Errorf("GetMarginBalance must not be called on separate account during planning"),
			}
			eng.exchanges[name] = fut
			eng.spotMargin[name] = sm

			dirACase := models.SpotEntryCandidate{
				Symbol: "BTCUSDT", BaseCoin: "BTC", Exchange: name,
				Direction: "borrow_sell_long", Timestamp: time.Now(),
			}
			planA, err := eng.BuildEntryPlan(dirACase)
			if err != nil {
				t.Fatalf("Dir A BuildEntryPlan: %v", err)
			}
			if !planA.RequiresInternalTransfer {
				t.Fatalf("RequiresInternalTransfer=false on %s Dir A, want true", name)
			}
			if planA.TransferTarget != "margin" {
				t.Fatalf("TransferTarget=%q on %s Dir A, want margin", planA.TransferTarget, name)
			}
			if planA.MaxBorrowableBase != 0 {
				t.Fatalf("MaxBorrowableBase=%.6f on %s Dir A separate — must be 0 pre-transfer",
					planA.MaxBorrowableBase, name)
			}

			dirBCase := models.SpotEntryCandidate{
				Symbol: "BTCUSDT", BaseCoin: "BTC", Exchange: name,
				Direction: "buy_spot_short", Timestamp: time.Now(),
			}
			planB, err := eng.BuildEntryPlan(dirBCase)
			if err != nil {
				t.Fatalf("Dir B BuildEntryPlan: %v", err)
			}
			if !planB.RequiresInternalTransfer {
				t.Fatalf("RequiresInternalTransfer=false on %s Dir B, want true", name)
			}
			if planB.TransferTarget != "spot" {
				t.Fatalf("TransferTarget=%q on %s Dir B, want spot", planB.TransferTarget, name)
			}
		})
	}
}

func TestBuildEntryPlan_GateioUnified_NoInternalTransfer(t *testing.T) {
	assertUnifiedNoTransfer(t, "gateio")
}

func TestBuildEntryPlan_BybitUTA_NoInternalTransfer(t *testing.T) {
	assertUnifiedNoTransfer(t, "bybit")
}

func TestBuildEntryPlan_OKXUnified_NoInternalTransfer(t *testing.T) {
	assertUnifiedNoTransfer(t, "okx")
}

func assertUnifiedNoTransfer(t *testing.T, name string) {
	t.Helper()
	cfg := defaultTestCfg()
	eng, _ := newSelectorTestEngine(t, cfg)

	fut := &selectorFutExchange{
		name:      name,
		orderbook: orderbookAt(100),
		contracts: contractWith("BTCUSDT", 0.01, 0.001, 3),
	}
	sm := &selectorSpotMargin{
		marginBal: &exchange.MarginBalance{MaxBorrowable: 100},
	}
	eng.exchanges[name] = fut
	eng.spotMargin[name] = sm

	for _, dir := range []string{"borrow_sell_long", "buy_spot_short"} {
		c := models.SpotEntryCandidate{
			Symbol: "BTCUSDT", BaseCoin: "BTC", Exchange: name,
			Direction: dir, Timestamp: time.Now(),
		}
		plan, err := eng.BuildEntryPlan(c)
		if err != nil {
			t.Fatalf("BuildEntryPlan(%s, %s): %v", name, dir, err)
		}
		if plan.RequiresInternalTransfer {
			t.Fatalf("%s %s: RequiresInternalTransfer=true, want false on unified", name, dir)
		}
		if plan.TransferTarget != "" {
			t.Fatalf("%s %s: TransferTarget=%q, want empty on unified", name, dir, plan.TransferTarget)
		}
	}
}

func TestBuildEntryPlan_BingXFilteredOut(t *testing.T) {
	cfg := defaultTestCfg()
	eng, _ := newSelectorTestEngine(t, cfg)

	// Install BingX as a futures adapter but do NOT register it as
	// SpotMarginExchange — that matches live setup (BingX has no margin).
	fut := &selectorFutExchange{
		name:      "bingx",
		orderbook: orderbookAt(100),
		contracts: contractWith("BTCUSDT", 0.01, 0.001, 3),
	}
	eng.exchanges["bingx"] = fut
	// NOTE: no spotMargin["bingx"] registration — BingX is filtered.

	c := models.SpotEntryCandidate{
		Symbol: "BTCUSDT", BaseCoin: "BTC", Exchange: "bingx",
		Direction: "buy_spot_short", Timestamp: time.Now(),
	}
	_, err := eng.BuildEntryPlan(c)
	if err == nil {
		t.Fatal("expected rejection for BingX (no spot margin)")
	}
	if !strings.Contains(err.Error(), "spot margin") {
		t.Fatalf("BingX rejection error = %v, want reference to spot margin", err)
	}
}

// ---------------------------------------------------------------------------
// Plan invariants
// ---------------------------------------------------------------------------

func TestBuildEntryPlan_UsesPlannedNotionalForScoreAndReserve(t *testing.T) {
	cfg := defaultTestCfg()
	cfg.SpotFuturesCapitalUnified = 1000
	eng, _ := newSelectorTestEngine(t, cfg)

	// Budget/price = 1000/200 = 5. Step=0.01 → plannedBase=5.0.
	// PlannedNotionalUSDT MUST equal plannedBase * midPrice = 5 * 200 = 1000
	// (identical to the reserve/score input).
	fut := &selectorFutExchange{
		name:      "bybit",
		orderbook: orderbookAt(200),
		contracts: contractWith("BTCUSDT", 0.01, 0.001, 3),
	}
	sm := &selectorSpotMargin{
		marginBal: &exchange.MarginBalance{MaxBorrowable: 100},
	}
	eng.exchanges["bybit"] = fut
	eng.spotMargin["bybit"] = sm

	c := models.SpotEntryCandidate{
		Symbol: "BTCUSDT", BaseCoin: "BTC", Exchange: "bybit",
		Direction: "borrow_sell_long", Timestamp: time.Now(),
	}
	plan, err := eng.BuildEntryPlan(c)
	if err != nil {
		t.Fatalf("BuildEntryPlan: %v", err)
	}
	want := plan.PlannedBaseSize * plan.MidPrice
	if plan.PlannedNotionalUSDT != want {
		t.Fatalf("PlannedNotionalUSDT = %.4f, want plannedBase*midPrice = %.4f (drift makes reserve/score inconsistent)",
			plan.PlannedNotionalUSDT, want)
	}
	if plan.PlannedNotionalUSDT > plan.CapitalBudgetUSDT+1e-6 {
		t.Fatalf("PlannedNotionalUSDT %.2f > CapitalBudgetUSDT %.2f — reservation cannot exceed budget",
			plan.PlannedNotionalUSDT, plan.CapitalBudgetUSDT)
	}
}

// ---------------------------------------------------------------------------
// OpenSelectedEntry
// ---------------------------------------------------------------------------

// TestOpenSelectedEntry_NoLatestOppsLookup verifies OpenSelectedEntry honors
// the supplied plan even when the opportunity cache is empty — the entire
// point of the contract is that the dispatcher feeds off the plan, not the
// cache. This is a non-execution path test: we just assert that the initial
// plan validation and lookup stages don't bail on "opportunity not found".
func TestOpenSelectedEntry_NoLatestOppsLookup(t *testing.T) {
	cfg := defaultTestCfg()
	cfg.SpotFuturesDryRun = true // stop at the dry-run gate before trading
	eng, _ := newSelectorTestEngine(t, cfg)

	// Empty latestOpps — the legacy ManualOpen would fail "opportunity not
	// found in latest scan". OpenSelectedEntry must NOT consult this cache.
	eng.latestOpps = nil

	fut := &selectorFutExchange{
		name:      "bybit",
		orderbook: orderbookAt(100),
		contracts: contractWith("BTCUSDT", 0.01, 0.001, 3),
	}
	sm := &selectorSpotMargin{
		marginBal: &exchange.MarginBalance{MaxBorrowable: 100},
	}
	eng.exchanges["bybit"] = fut
	eng.spotMargin["bybit"] = sm

	plan := &models.SpotEntryPlan{
		Candidate: models.SpotEntryCandidate{
			Symbol: "BTCUSDT", BaseCoin: "BTC", Exchange: "bybit",
			Direction: "borrow_sell_long", FundingAPR: 0.1, BorrowAPR: 0.02,
			FeePct: 0.002, Timestamp: time.Now(),
		},
		CapitalBudgetUSDT:   500,
		MidPrice:            100,
		PlannedBaseSize:     4.5,
		PlannedNotionalUSDT: 450,
		FuturesMarginUSDT:   150,
	}

	err := eng.OpenSelectedEntry(plan, 0, nil)
	if err == nil {
		t.Fatal("expected dry-run block — plan should reach the dry-run gate")
	}
	if !strings.Contains(err.Error(), "dry run") {
		t.Fatalf("OpenSelectedEntry error = %v; want dry-run block (proving plan was accepted without cache lookup)", err)
	}
}

// TestOpenSelectedEntry_UsesPreheldReservationNoReserveCall verifies the
// preheld path skips reserveSpotCapital entirely. We install a disabled
// allocator so Reserve is a no-op; the test asserts OpenSelectedEntry reaches
// the dry-run gate without erroring on capital reservation (and without
// attempting to call the nil allocator's Reserve).
func TestOpenSelectedEntry_UsesPreheldReservationNoReserveCall(t *testing.T) {
	cfg := defaultTestCfg()
	cfg.SpotFuturesDryRun = true
	eng, _ := newSelectorTestEngine(t, cfg)

	fut := &selectorFutExchange{
		name:      "bybit",
		orderbook: orderbookAt(100),
		contracts: contractWith("BTCUSDT", 0.01, 0.001, 3),
	}
	sm := &selectorSpotMargin{
		marginBal: &exchange.MarginBalance{MaxBorrowable: 100},
	}
	eng.exchanges["bybit"] = fut
	eng.spotMargin["bybit"] = sm

	plan := &models.SpotEntryPlan{
		Candidate: models.SpotEntryCandidate{
			Symbol: "BTCUSDT", BaseCoin: "BTC", Exchange: "bybit",
			Direction: "buy_spot_short", FundingAPR: 0.1, BorrowAPR: 0,
			FeePct: 0.002, Timestamp: time.Now(),
		},
		CapitalBudgetUSDT:   500,
		MidPrice:            100,
		PlannedBaseSize:     4.5,
		PlannedNotionalUSDT: 450,
		FuturesMarginUSDT:   150,
	}

	// Supply a non-nil preheld reservation — allocator is nil so Commit is a
	// no-op, but OpenSelectedEntry must route through the preheld branch
	// rather than calling reserveSpotCapital. Reaching the dry-run gate
	// without erroring on capital reservation demonstrates the preheld path
	// executed successfully.
	res := buildFakeReservation("bybit", 450)

	err := eng.OpenSelectedEntry(plan, 0, res)
	if err == nil {
		t.Fatal("expected dry-run block (indicates preheld path reached dry-run gate cleanly)")
	}
	if !strings.Contains(err.Error(), "dry run") {
		t.Fatalf("OpenSelectedEntry error = %v; want dry-run block", err)
	}
}
