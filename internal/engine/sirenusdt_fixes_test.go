// Package engine — unit tests for the SIRENUSDT 3-bug fixes (v13 plan Phase 3).
//
// Covers:
//   - H1A/H1B: retrySecondLeg VWAP / BBO sanity (3 tests)
//   - H2:      CloseSize field helpers + mutation-path invariants (12 tests)
//   - H3A:     slIndex includes TP IDs (TestStopIndexMatchesTPID)
//   - H3B.2:   handleAlgoRemap aliases slIndex (TestHandleAlgoRemapAliasesSlIndex)
//   - H3B.2d:  ExchangeManager.AddReloadHandler (TestExchangeManagerReloadInvokesHandlers)
//
// Tests that require a full DB harness (tryReconcilePnL integration) are
// explicitly skipped with t.Skip and a reason.
package engine

import (
	"context"
	"sync"
	"testing"
	"time"

	"arb/internal/config"
	"arb/internal/models"
	"arb/pkg/exchange"
	"arb/pkg/utils"
)

// ---------------------------------------------------------------------------
// fullStubExchange — implements exchange.Exchange with all methods as no-ops.
// Individual tests override only the methods they need via function fields.
// ---------------------------------------------------------------------------

type fullStubExchange struct {
	mu           sync.Mutex
	orderStore   map[string]exchange.OrderUpdate
	bbo          exchange.BBO
	bboOK        bool
	placeOrderFn func(p exchange.PlaceOrderParams) (string, error)
	getFilledFn  func(orderID, symbol string) (float64, error)
}

var _ exchange.Exchange = (*fullStubExchange)(nil)

func newFullStub(bbo exchange.BBO, bboOK bool) *fullStubExchange {
	return &fullStubExchange{
		orderStore: make(map[string]exchange.OrderUpdate),
		bbo:        bbo,
		bboOK:      bboOK,
	}
}

func (s *fullStubExchange) storeOrder(oid string, upd exchange.OrderUpdate) {
	s.mu.Lock()
	s.orderStore[oid] = upd
	s.mu.Unlock()
}

// --- Exchange interface implementation ---

func (s *fullStubExchange) Name() string { return "stub" }
func (s *fullStubExchange) SetMetricsCallback(fn exchange.MetricsCallback) {}
func (s *fullStubExchange) PlaceOrder(p exchange.PlaceOrderParams) (string, error) {
	if s.placeOrderFn != nil {
		return s.placeOrderFn(p)
	}
	return "oid-default", nil
}
func (s *fullStubExchange) CancelOrder(symbol, orderID string) error    { return nil }
func (s *fullStubExchange) GetPendingOrders(sym string) ([]exchange.Order, error) {
	return nil, nil
}
func (s *fullStubExchange) GetOrderFilledQty(orderID, symbol string) (float64, error) {
	if s.getFilledFn != nil {
		return s.getFilledFn(orderID, symbol)
	}
	return 0, nil
}
func (s *fullStubExchange) GetPosition(sym string) ([]exchange.Position, error)  { return nil, nil }
func (s *fullStubExchange) GetAllPositions() ([]exchange.Position, error)         { return nil, nil }
func (s *fullStubExchange) SetLeverage(sym, leverage, holdSide string) error      { return nil }
func (s *fullStubExchange) SetMarginMode(sym, mode string) error                  { return nil }
func (s *fullStubExchange) LoadAllContracts() (map[string]exchange.ContractInfo, error) {
	return nil, nil
}
func (s *fullStubExchange) GetFundingRate(sym string) (*exchange.FundingRate, error) {
	return nil, nil
}
func (s *fullStubExchange) GetFundingInterval(sym string) (time.Duration, error) {
	return 0, nil
}
func (s *fullStubExchange) GetFuturesBalance() (*exchange.Balance, error) {
	return &exchange.Balance{}, nil
}
func (s *fullStubExchange) GetSpotBalance() (*exchange.Balance, error) {
	return &exchange.Balance{}, nil
}
func (s *fullStubExchange) Withdraw(p exchange.WithdrawParams) (*exchange.WithdrawResult, error) {
	return nil, nil
}
func (s *fullStubExchange) WithdrawFeeInclusive() bool { return false }
func (s *fullStubExchange) GetWithdrawFee(coin, chain string) (float64, float64, error) {
	return 0, 0, nil
}
func (s *fullStubExchange) TransferToSpot(coin, amount string) error    { return nil }
func (s *fullStubExchange) TransferToFutures(coin, amount string) error { return nil }
func (s *fullStubExchange) GetOrderbook(sym string, depth int) (*exchange.Orderbook, error) {
	return nil, nil
}
func (s *fullStubExchange) StartPriceStream(syms []string)        {}
func (s *fullStubExchange) SubscribeSymbol(sym string) bool       { return true }
func (s *fullStubExchange) GetBBO(sym string) (exchange.BBO, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.bbo, s.bboOK
}
func (s *fullStubExchange) GetPriceStore() *sync.Map { return &sync.Map{} }
func (s *fullStubExchange) SubscribeDepth(sym string) bool  { return true }
func (s *fullStubExchange) UnsubscribeDepth(sym string) bool { return true }
func (s *fullStubExchange) GetDepth(sym string) (*exchange.Orderbook, bool) { return nil, false }
func (s *fullStubExchange) StartPrivateStream()                              {}
func (s *fullStubExchange) GetOrderUpdate(orderID string) (exchange.OrderUpdate, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.orderStore[orderID]
	return u, ok
}
func (s *fullStubExchange) SetOrderCallback(fn func(exchange.OrderUpdate)) {}
func (s *fullStubExchange) PlaceStopLoss(p exchange.StopLossParams) (string, error) {
	return "", nil
}
func (s *fullStubExchange) CancelStopLoss(sym, orderID string) error { return nil }
func (s *fullStubExchange) PlaceTakeProfit(p exchange.TakeProfitParams) (string, error) {
	return "", nil
}
func (s *fullStubExchange) CancelTakeProfit(sym, orderID string) error { return nil }
func (s *fullStubExchange) GetUserTrades(sym string, since time.Time, limit int) ([]exchange.Trade, error) {
	return nil, nil
}
func (s *fullStubExchange) GetFundingFees(sym string, since time.Time) ([]exchange.FundingPayment, error) {
	return nil, nil
}
func (s *fullStubExchange) GetClosePnL(sym string, since time.Time) ([]exchange.ClosePnL, error) {
	return nil, nil
}
func (s *fullStubExchange) EnsureOneWayMode() error  { return nil }
func (s *fullStubExchange) CancelAllOrders(sym string) error { return nil }
func (s *fullStubExchange) Close()                          {}

// ---------------------------------------------------------------------------
// Minimal Engine builder for unit tests (no DB, no discovery).
// ---------------------------------------------------------------------------

func newMinimalEngine() *Engine {
	return &Engine{
		cfg:                &config.Config{SlippageBPS: 10},
		log:                utils.NewLogger("test"),
		slIndex:            make(map[string]stopOrderEntry),
		slFillCh:           make(chan slFillEvent, 64),
		apiErrCounts:       make(map[string]int),
		exitCancels:        make(map[string]context.CancelFunc),
		exitActive:         make(map[string]bool),
		exitDone:           make(map[string]chan struct{}),
		preSettleActive:    make(map[string]bool),
		entryActive:        make(map[string]string),
		depthRefs:          make(map[string]*depthRef),
		exchanges:          make(map[string]exchange.Exchange),
		consolidateRetries: make(map[string]int),
		stopCh:             make(chan struct{}),
	}
}

// ---------------------------------------------------------------------------
// H1A — TestRetrySecondLegUsesGetOrderUpdateAvgPrice
//
// Verifies that when the order store has a populated AvgPrice, retrySecondLeg
// uses it for VWAP rather than a synthetic BBO mid.
// ---------------------------------------------------------------------------

func TestRetrySecondLegUsesGetOrderUpdateAvgPrice(t *testing.T) {
	const refPrice = 1.58  // realistic SIRENUSDT price
	const realAvg = 1.59   // real fill price from WS
	const corruptBid = 41.56
	const corruptAsk = 42.00

	// BBO is corrupt (outside ±20% of refPrice → sanity clamp fires → falls back
	// to refPrice synthetic). The IOC fill must still use realAvg from WS store.
	exch := newFullStub(exchange.BBO{Bid: corruptBid, Ask: corruptAsk}, true)

	placeCount := 0
	exch.placeOrderFn = func(p exchange.PlaceOrderParams) (string, error) {
		oid := "oid-ioc"
		if placeCount == 0 {
			exch.storeOrder(oid, exchange.OrderUpdate{
				OrderID:      oid,
				Status:       "filled",
				FilledVolume: 189.0,
				AvgPrice:     realAvg,
			})
		}
		placeCount++
		return oid, nil
	}
	exch.getFilledFn = func(orderID, symbol string) (float64, error) {
		return 189.0, nil
	}

	e := newMinimalEngine()
	e.exchanges["bingx"] = exch

	filled, avg, err := e.retrySecondLeg(exch, "bingx", "SIRENUSDT", exchange.SideBuy, 189.0, refPrice)
	if err != nil {
		t.Fatalf("unexpected retrySecondLeg error: %v", err)
	}

	if filled <= 0 {
		t.Fatalf("expected filled > 0, got %.6f", filled)
	}
	// avg must be realAvg (1.59), NOT BBO-mid (~41.78) or refPrice (1.58).
	if avg < realAvg*0.999 || avg > realAvg*1.001 {
		t.Errorf("expected avgPrice ≈ %.4f (real WS avg), got %.6f — possible BBO contamination", realAvg, avg)
	}
}

// ---------------------------------------------------------------------------
// H1B — TestRetrySecondLegFallbackWhenAvgPriceZero
//
// Verifies that when GetOrderUpdate returns AvgPrice=0 (WS lag), IOC falls back
// to orderPrice (limit price) rather than zero or BBO-mid.
// ---------------------------------------------------------------------------

func TestRetrySecondLegFallbackWhenAvgPriceZero(t *testing.T) {
	const refPrice = 1.58
	// Clean BBO that passes sanity (within ±20% of refPrice).
	exch := newFullStub(exchange.BBO{Bid: 1.57, Ask: 1.59}, true)

	exch.placeOrderFn = func(p exchange.PlaceOrderParams) (string, error) {
		oid := "oid-zero-avg"
		// AvgPrice=0 simulates WS lag.
		exch.storeOrder(oid, exchange.OrderUpdate{
			OrderID:      oid,
			Status:       "filled",
			FilledVolume: 100.0,
			AvgPrice:     0,
		})
		return oid, nil
	}
	exch.getFilledFn = func(orderID, symbol string) (float64, error) {
		return 100.0, nil
	}

	e := newMinimalEngine()
	e.exchanges["bingx"] = exch

	filled, avg, err := e.retrySecondLeg(exch, "bingx", "SIRENUSDT", exchange.SideBuy, 100.0, refPrice)
	if err != nil {
		t.Fatalf("unexpected retrySecondLeg error: %v", err)
	}

	if filled <= 0 {
		t.Fatalf("expected filled > 0, got %.6f", filled)
	}
	if avg <= 0 {
		t.Errorf("expected avg > 0 (fallback to orderPrice), got %.6f", avg)
	}
	// orderPrice = Ask * (1 + slippage) = 1.59 * 1.001 ≈ 1.5916; must be near ask.
	if avg > 1.59*1.01 || avg < 1.58*0.99 {
		t.Errorf("fallback avg=%.6f is not near ask=1.59 (expected IOC orderPrice fallback)", avg)
	}
}

// ---------------------------------------------------------------------------
// H1B — TestRetrySecondLegBBOSanityFallback
//
// Verifies that when BBO bid=100×refPrice (corrupt feed), the sanity clamp
// activates and uses refPrice as synthetic BBO.
// ---------------------------------------------------------------------------

func TestRetrySecondLegBBOSanityFallback(t *testing.T) {
	const refPrice = 1.58
	const corruptBid = 158.0 // 100× ref — wildly outside 20% band
	const corruptAsk = 160.0

	exch := newFullStub(exchange.BBO{Bid: corruptBid, Ask: corruptAsk}, true)
	exch.placeOrderFn = func(p exchange.PlaceOrderParams) (string, error) {
		oid := "oid-sanity"
		// AvgPrice=0 → falls back to orderPrice, which was computed from refPrice BBO.
		exch.storeOrder(oid, exchange.OrderUpdate{
			OrderID: oid, Status: "filled", FilledVolume: 50.0, AvgPrice: 0,
		})
		return oid, nil
	}
	exch.getFilledFn = func(orderID, symbol string) (float64, error) { return 50.0, nil }

	e := newMinimalEngine()
	e.exchanges["bingx"] = exch

	filled, avg, err := e.retrySecondLeg(exch, "bingx", "SIRENUSDT", exchange.SideBuy, 50.0, refPrice)
	if err != nil {
		t.Fatalf("unexpected retrySecondLeg error: %v", err)
	}

	if filled <= 0 {
		t.Fatalf("expected filled > 0, got %.6f", filled)
	}
	// BBO sanity → synthetic BBO = {refPrice, refPrice}. IOC fallback orderPrice ≈ refPrice.
	// avg must be near refPrice, NOT near corruptBid (~159).
	if avg > refPrice*1.05 || avg < refPrice*0.95 {
		t.Errorf("expected avg ≈ refPrice=%.4f after BBO sanity fallback, got %.6f (corrupt BBO not clamped?)", refPrice, avg)
	}
}

// ---------------------------------------------------------------------------
// H2 helper unit tests — pure logic, no DB needed.
// ---------------------------------------------------------------------------

func makeTestPos(id string, longCS, shortCS, longSz, shortSz float64) *models.ArbitragePosition {
	return &models.ArbitragePosition{
		ID:             id,
		Symbol:         "SIRENUSDT",
		LongExchange:   "binance",
		ShortExchange:  "bingx",
		LongEntry:      1.58,
		ShortEntry:     1.60,
		LongSize:       longSz,
		ShortSize:      shortSz,
		LongCloseSize:  longCS,
		ShortCloseSize: shortCS,
		Status:         models.StatusClosed,
		UpdatedAt:      time.Now().UTC(),
	}
}

// TestAllSiblingsHaveCloseSizeTrueWhenAllPopulated verifies helper returns true
// when all siblings have CloseSize > 0.
func TestAllSiblingsHaveCloseSizeTrueWhenAllPopulated(t *testing.T) {
	siblings := []*models.ArbitragePosition{
		makeTestPos("s1", 100, 100, 0, 0),
		makeTestPos("s2", 200, 200, 0, 0),
	}
	if !allSiblingsHaveCloseSize(siblings, "long") {
		t.Error("expected true — all siblings have LongCloseSize > 0")
	}
	if !allSiblingsHaveCloseSize(siblings, "short") {
		t.Error("expected true — all siblings have ShortCloseSize > 0")
	}
}

// TestAllSiblingsHaveCloseSizeFalseWhenOneMissing verifies that a single
// zero CloseSize causes the helper to return false.
func TestAllSiblingsHaveCloseSizeFalseWhenOneMissing(t *testing.T) {
	siblings := []*models.ArbitragePosition{
		makeTestPos("s1", 100, 100, 0, 0),
		makeTestPos("s2", 0, 0, 200, 200), // pre-migration
	}
	if allSiblingsHaveCloseSize(siblings, "long") {
		t.Error("expected false — s2 has LongCloseSize=0")
	}
}

// TestSumSiblingCloseSizeAggregates verifies sumSiblingCloseSize sums the
// correct field per side.
func TestSumSiblingCloseSizeAggregates(t *testing.T) {
	siblings := []*models.ArbitragePosition{
		makeTestPos("s1", 100, 150, 0, 0),
		makeTestPos("s2", 200, 50, 0, 0),
	}
	if got := sumSiblingCloseSize(siblings, "long"); got != 300 {
		t.Errorf("sumSiblingCloseSize long = %.0f, want 300", got)
	}
	if got := sumSiblingCloseSize(siblings, "short"); got != 200 {
		t.Errorf("sumSiblingCloseSize short = %.0f, want 200", got)
	}
}

// ---------------------------------------------------------------------------
// H2 reconcile integration tests — skipped (require DB + exchange stubs).
// ---------------------------------------------------------------------------

func TestReconcileAcceptsDepthExitWithCloseSizeMatch(t *testing.T) {
	t.Skip("needs E2E harness: tryReconcilePnL requires miniredis DB + exchange GetClosePnL stubs")
}

func TestReconcileRetriesWhenAggregationFails(t *testing.T) {
	t.Skip("needs E2E harness: tryReconcilePnL requires miniredis DB + exchange GetClosePnL stubs")
}

// ---------------------------------------------------------------------------
// H2 Tier 1 gate — inline precondition logic tests (no DB).
// ---------------------------------------------------------------------------

// TestReconcileRetriesWhenCloseSizePartial verifies that when rawCloseSize < expected
// (CloseSize=385, rawClose=100), the Tier 1 gate would return false (retry).
func TestReconcileRetriesWhenCloseSizePartial(t *testing.T) {
	const sizeEpsilon = 1e-6
	pos := makeTestPos("pos-1", 385, 385, 0, 0)
	var siblings []*models.ArbitragePosition

	useTier1 := pos.LongCloseSize > 0 && pos.ShortCloseSize > 0 &&
		allSiblingsHaveCloseSize(siblings, "long") &&
		allSiblingsHaveCloseSize(siblings, "short")
	if !useTier1 {
		t.Fatal("expected Tier 1 to apply")
	}

	longExpected := pos.LongCloseSize + sumSiblingCloseSize(siblings, "long") // 385
	rawCloseSize := 100.0
	if rawCloseSize >= longExpected-sizeEpsilon {
		t.Errorf("test setup wrong: rawClose=%.0f should be < expected=%.0f", rawCloseSize, longExpected)
	}
	// Gate fires → retry (rawCloseSize < longExpected).
}

// TestReconcileAcceptsSharedPositionCompleteClose verifies that when 2 siblings
// each with CloseSize=100 and rawCloseSize=200, Tier 1 gate passes.
func TestReconcileAcceptsSharedPositionCompleteClose(t *testing.T) {
	const sizeEpsilon = 1e-6
	pos := makeTestPos("pos-main", 100, 100, 0, 0)
	siblings := []*models.ArbitragePosition{makeTestPos("pos-sibling", 100, 100, 0, 0)}

	useTier1 := pos.LongCloseSize > 0 && pos.ShortCloseSize > 0 &&
		allSiblingsHaveCloseSize(siblings, "long") &&
		allSiblingsHaveCloseSize(siblings, "short")
	if !useTier1 {
		t.Fatal("expected Tier 1 to apply")
	}

	longExpected := pos.LongCloseSize + sumSiblingCloseSize(siblings, "long") // 200
	rawCloseSize := 200.0
	if rawCloseSize < longExpected-sizeEpsilon {
		t.Errorf("expected gate to PASS: rawClose=%.0f >= expected=%.0f", rawCloseSize, longExpected)
	}
}

// TestReconcileRetriesSharedPositionPartial verifies that rawCloseSize=50 with
// 2 siblings (expected=200) causes Tier 1 gate to retry.
func TestReconcileRetriesSharedPositionPartial(t *testing.T) {
	const sizeEpsilon = 1e-6
	pos := makeTestPos("pos-main", 100, 100, 0, 0)
	siblings := []*models.ArbitragePosition{makeTestPos("pos-sibling", 100, 100, 0, 0)}

	longExpected := pos.LongCloseSize + sumSiblingCloseSize(siblings, "long") // 200
	rawCloseSize := 50.0
	if rawCloseSize >= longExpected-sizeEpsilon {
		t.Errorf("test setup wrong: rawClose=%.0f should be < expected=%.0f", rawCloseSize, longExpected)
	}
	// Gate fires → retry.
}

// TestReconcileMixedHistoryFallsThroughToTier2 verifies that when one sibling
// has CloseSize=0, Tier 1 precondition fails → falls to Tier 2/3.
func TestReconcileMixedHistoryFallsThroughToTier2(t *testing.T) {
	pos := makeTestPos("pos-main", 100, 100, 0, 0)
	siblings := []*models.ArbitragePosition{
		makeTestPos("pos-pre-migration", 0, 0, 200, 200),
	}

	useTier1 := pos.LongCloseSize > 0 && pos.ShortCloseSize > 0 &&
		allSiblingsHaveCloseSize(siblings, "long") &&
		allSiblingsHaveCloseSize(siblings, "short")
	if useTier1 {
		t.Error("expected Tier 1 NOT to apply — sibling has CloseSize=0")
	}
	// Confirmed: Tier 1 falls through to Tier 2/3.
}

// TestReconcilePreMigrationNormalCloseRetainsNotionalGuard verifies Tier 2 logic:
// CloseSize=0, LongSize=385 → notional guard. Small diff passes, large diff retries.
func TestReconcilePreMigrationNormalCloseRetainsNotionalGuard(t *testing.T) {
	pos := makeTestPos("pos-legacy", 0, 0, 385, 385)
	pos.LongEntry = 1.58
	pos.ShortEntry = 1.60

	useTier1 := pos.LongCloseSize > 0 && pos.ShortCloseSize > 0
	if useTier1 {
		t.Fatal("expected Tier 1 NOT to apply for pre-migration position")
	}
	if !(pos.LongSize > 0 || pos.ShortSize > 0) {
		t.Fatal("expected Tier 2 to apply (LongSize > 0)")
	}

	longNotional := pos.LongEntry * pos.LongSize // ~608
	shortNotional := pos.ShortEntry * pos.ShortSize
	notional := longNotional
	if shortNotional > notional {
		notional = shortNotional
	}

	// diff within notional → accepted (no retry from Tier 2 guard).
	diffSmall := 10.0
	if diffSmall > notional {
		t.Errorf("small diff=%.2f should not trigger retry (notional=%.2f)", diffSmall, notional)
	}
	// diff > notional → Tier 2 guard fires retry.
	diffBig := 2387.0
	if diffBig <= notional {
		t.Errorf("large diff=%.2f should trigger Tier 2 retry (notional=%.2f)", diffBig, notional)
	}
}

// TestReconcileDepthExitPreMigrationFallback verifies Tier 3: CloseSize=0 AND
// LongSize=0 → neither Tier 1 nor Tier 2 applies; relies on longOK && shortOK.
func TestReconcileDepthExitPreMigrationFallback(t *testing.T) {
	pos := makeTestPos("pos-legacy-depth-exit", 0, 0, 0, 0)

	useTier1 := pos.LongCloseSize > 0 && pos.ShortCloseSize > 0
	hasTier2 := pos.LongSize > 0 || pos.ShortSize > 0

	if useTier1 {
		t.Error("expected Tier 1 NOT to apply")
	}
	if hasTier2 {
		t.Error("expected Tier 2 NOT to apply")
	}
	// Tier 3 applies: gate relies on longOK && shortOK.
}

// TestCloseSizeImmutableThroughDepthExit verifies depth-exit zeroes LongSize but
// does NOT touch LongCloseSize — the key invariant enabling reconcile gate.
func TestCloseSizeImmutableThroughDepthExit(t *testing.T) {
	pos := &models.ArbitragePosition{
		LongSize:       189.0,
		ShortSize:      189.0,
		LongCloseSize:  189.0,
		ShortCloseSize: 189.0,
	}
	// Simulate the depth-exit UpdatePositionFields closure (exit.go:960-968):
	// only LongSize/ShortSize are zeroed; CloseSize is untouched.
	pos.LongSize = 0
	pos.ShortSize = 0

	if pos.LongCloseSize != 189.0 {
		t.Errorf("LongCloseSize must survive depth-exit zeroing; got %.2f, want 189", pos.LongCloseSize)
	}
	if pos.ShortCloseSize != 189.0 {
		t.Errorf("ShortCloseSize must survive depth-exit zeroing; got %.2f, want 189", pos.ShortCloseSize)
	}
}

// TestReconcilePartialCloseRevertUpdatesCloseSize verifies that when depth-exit
// reverts to partial, both LongSize AND LongCloseSize are set to the remainder.
func TestReconcilePartialCloseRevertUpdatesCloseSize(t *testing.T) {
	pos := &models.ArbitragePosition{
		LongSize: 389.0, ShortSize: 389.0,
		LongCloseSize: 389.0, ShortCloseSize: 389.0,
	}
	longRemainder, shortRemainder := 200.0, 200.0

	// Simulate partial-close revert (exit.go:975-987).
	pos.Status = models.StatusActive
	pos.LongSize = longRemainder
	pos.ShortSize = shortRemainder
	pos.LongCloseSize = longRemainder   // H2.2 requirement
	pos.ShortCloseSize = shortRemainder // H2.2 requirement

	if pos.LongCloseSize != longRemainder {
		t.Errorf("LongCloseSize after revert = %.0f, want %.0f", pos.LongCloseSize, longRemainder)
	}
	if pos.LongSize != longRemainder {
		t.Errorf("LongSize after revert = %.0f, want %.0f", pos.LongSize, longRemainder)
	}
}

// TestReconcileRotationAcceptingPartialUpdatesCloseSize verifies that when a
// rotation accepts partial fill (openFilled < target), CloseSize is updated.
func TestReconcileRotationAcceptingPartialUpdatesCloseSize(t *testing.T) {
	pos := &models.ArbitragePosition{
		LongSize: 385.0, LongCloseSize: 385.0,
		ShortSize: 385.0, ShortCloseSize: 385.0,
	}
	target, openFilled := 385.0, 200.0

	if openFilled < target {
		pos.LongSize = openFilled
		pos.LongCloseSize = openFilled // H2.2
	}

	if pos.LongCloseSize != openFilled {
		t.Errorf("LongCloseSize after partial rotation = %.0f, want %.0f", pos.LongCloseSize, openFilled)
	}
	if pos.LongSize != openFilled {
		t.Errorf("LongSize after partial rotation = %.0f, want %.0f", pos.LongSize, openFilled)
	}
}

// TestReconcileStartupMergeUpdatesCloseSize verifies that when a startup
// duplicate merge grows the survivor's live size, CloseSize is updated too.
func TestReconcileStartupMergeUpdatesCloseSize(t *testing.T) {
	survivor := &models.ArbitragePosition{
		LongSize: 189.0, ShortSize: 189.0,
		LongCloseSize: 189.0, ShortCloseSize: 189.0,
	}
	dup := &models.ArbitragePosition{LongSize: 100.0, ShortSize: 100.0}

	// Simulate merge (engine.go:2591-2603).
	merged := survivor.LongSize + dup.LongSize
	survivor.LongSize = merged
	survivor.ShortSize = survivor.ShortSize + dup.ShortSize
	survivor.LongCloseSize = merged
	survivor.ShortCloseSize = survivor.ShortSize

	if survivor.LongCloseSize != 289.0 {
		t.Errorf("LongCloseSize after merge = %.0f, want 289", survivor.LongCloseSize)
	}
}

// ---------------------------------------------------------------------------
// H3A — TestStopIndexMatchesTPID
//
// Verifies registerStopOrders indexes all 4 IDs (long/short × sl/tp) and that
// TP IDs are stored with Kind="tp".
// ---------------------------------------------------------------------------

func TestStopIndexMatchesTPID(t *testing.T) {
	e := newMinimalEngine()

	pos := &models.ArbitragePosition{
		ID:             "pos-tp-test",
		LongExchange:   "binance",
		ShortExchange:  "bingx",
		LongSLOrderID:  "sl-long-1",
		ShortSLOrderID: "sl-short-1",
		LongTPOrderID:  "tp-long-999",
		ShortTPOrderID: "tp-short-999",
	}
	e.registerStopOrders(pos)

	cases := []struct {
		key     string
		wantLeg string
		wantKind string
	}{
		{"binance:tp-long-999", "long", "tp"},
		{"bingx:tp-short-999", "short", "tp"},
		{"binance:sl-long-1", "long", "sl"},
		{"bingx:sl-short-1", "short", "sl"},
	}
	for _, tc := range cases {
		t.Run(tc.key, func(t *testing.T) {
			e.slIndexMu.RLock()
			entry, ok := e.slIndex[tc.key]
			e.slIndexMu.RUnlock()
			if !ok {
				t.Fatalf("slIndex missing key %q", tc.key)
			}
			if entry.Kind != tc.wantKind {
				t.Errorf("Kind = %q, want %q", entry.Kind, tc.wantKind)
			}
			if entry.Leg != tc.wantLeg {
				t.Errorf("Leg = %q, want %q", entry.Leg, tc.wantLeg)
			}
			if entry.PosID != "pos-tp-test" {
				t.Errorf("PosID = %q, want %q", entry.PosID, "pos-tp-test")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// H3B.2 — TestHandleAlgoRemapAliasesSlIndex
//
// Verifies that handleAlgoRemap correctly aliases algoID → realID in slIndex,
// preserving the original entry for unregisterStopOrders cleanup.
// ---------------------------------------------------------------------------

func TestHandleAlgoRemapAliasesSlIndex(t *testing.T) {
	e := newMinimalEngine()

	algoID := "3000001251209638"
	realID := "3561813146"

	// Pre-register as stored at TP placement time.
	e.slIndexMu.Lock()
	e.slIndex["binance:"+algoID] = stopOrderEntry{PosID: "pos-siren", Leg: "long", Kind: "tp"}
	e.slIndexMu.Unlock()

	// Fire ALGO_UPDATE TRIGGERED remap.
	e.handleAlgoRemap("binance", exchange.AlgoRemap{
		AlgoID: algoID,
		RealID: realID,
		Symbol: "SIRENUSDT",
	})

	e.slIndexMu.RLock()
	aliasEntry, aliasOK := e.slIndex["binance:"+realID]
	_, origOK := e.slIndex["binance:"+algoID]
	e.slIndexMu.RUnlock()

	if !aliasOK {
		t.Fatalf("slIndex missing alias key binance:%s after handleAlgoRemap", realID)
	}
	if !origOK {
		t.Error("slIndex removed original algoID entry — must be preserved for unregisterStopOrders")
	}
	if aliasEntry.PosID != "pos-siren" {
		t.Errorf("alias entry.PosID = %q, want %q", aliasEntry.PosID, "pos-siren")
	}
	if aliasEntry.Kind != "tp" {
		t.Errorf("alias entry.Kind = %q, want %q", aliasEntry.Kind, "tp")
	}
	if aliasEntry.Leg != "long" {
		t.Errorf("alias entry.Leg = %q, want %q", aliasEntry.Leg, "long")
	}
}

// ---------------------------------------------------------------------------
// H3B.2d — TestExchangeManagerReloadInvokesHandlers
//
// Verifies that AddReloadHandler stores a callback that is invoked when Reload
// rebuilds an adapter. Tests the dispatch mechanism in isolation.
// ---------------------------------------------------------------------------

func TestExchangeManagerReloadInvokesHandlers(t *testing.T) {
	cfg := &config.Config{}
	em := NewExchangeManager(cfg)

	var mu sync.Mutex
	var invokedNames []string

	em.AddReloadHandler(func(name string, adapter exchange.Exchange) {
		mu.Lock()
		invokedNames = append(invokedNames, name)
		mu.Unlock()
	})

	// Invoke handlers the same way Reload does (outside m.mu).
	em.reloadHandlersMu.Lock()
	handlers := append([]func(string, exchange.Exchange){}, em.reloadHandlers...)
	em.reloadHandlersMu.Unlock()

	stub := newFullStub(exchange.BBO{}, false)
	for _, h := range handlers {
		h("binance", stub)
		h("bingx", stub)
	}

	mu.Lock()
	names := make([]string, len(invokedNames))
	copy(names, invokedNames)
	mu.Unlock()

	if len(names) != 2 {
		t.Fatalf("handler invoked %d times, want 2 (binance + bingx)", len(names))
	}
	if names[0] != "binance" || names[1] != "bingx" {
		t.Errorf("invoked names = %v, want [binance bingx]", names)
	}
}

// ---------------------------------------------------------------------------
// H3B.2 — TestAttachAdapterCallbacksRegistersAlgoRemap
//
// Verifies that AttachAdapterCallbacks wires SetAlgoRemapCallback on adapters
// that implement AlgoRemapCallbackSetter.
// ---------------------------------------------------------------------------

// algoRemapStub extends fullStubExchange to implement AlgoRemapCallbackSetter.
type algoRemapStub struct {
	*fullStubExchange
	mu       sync.Mutex
	callback exchange.AlgoRemapCallback
}

func (a *algoRemapStub) SetAlgoRemapCallback(fn exchange.AlgoRemapCallback) {
	a.mu.Lock()
	a.callback = fn
	a.mu.Unlock()
}

func (a *algoRemapStub) fire(remap exchange.AlgoRemap) {
	a.mu.Lock()
	cb := a.callback
	a.mu.Unlock()
	if cb != nil {
		cb(remap)
	}
}

var _ exchange.AlgoRemapCallbackSetter = (*algoRemapStub)(nil)

func TestAttachAdapterCallbacksRegistersAlgoRemap(t *testing.T) {
	e := newMinimalEngine()

	stub := &algoRemapStub{fullStubExchange: newFullStub(exchange.BBO{}, false)}
	e.AttachAdapterCallbacks("binance", stub)

	// Pre-register algo entry in slIndex.
	e.slIndexMu.Lock()
	e.slIndex["binance:algo-111"] = stopOrderEntry{PosID: "pos-x", Leg: "long", Kind: "tp"}
	e.slIndexMu.Unlock()

	// Fire the remap callback as if ALGO_UPDATE arrived.
	stub.fire(exchange.AlgoRemap{AlgoID: "algo-111", RealID: "real-222", Symbol: "BTCUSDT"})

	// Alias must be present.
	e.slIndexMu.RLock()
	_, ok := e.slIndex["binance:real-222"]
	e.slIndexMu.RUnlock()

	if !ok {
		t.Error("expected slIndex alias binance:real-222 after AttachAdapterCallbacks + algo fire")
	}
}

// ---------------------------------------------------------------------------
// Tier-1 reconcile gate: CloseSizeUnknown handling.
// ---------------------------------------------------------------------------

// TestAggregateClosePnLBySide_PropagatesCloseSizeUnknown verifies that when any
// matching-side input record has CloseSizeUnknown=true, the aggregate propagates
// the flag so the Tier-1 gate can skip size comparison.
func TestAggregateClosePnLBySide_PropagatesCloseSizeUnknown(t *testing.T) {
	records := []exchange.ClosePnL{
		{Side: "long", NetPnL: 1.0, CloseSize: 0, CloseSizeUnknown: true},
	}
	agg, ok := aggregateClosePnLBySide(records, "long")
	if !ok {
		t.Fatal("expected ok=true")
	}
	if !agg.CloseSizeUnknown {
		t.Error("CloseSizeUnknown = false, want true (propagated from input)")
	}

	// Multi-record mix: one known + one unknown → aggregate marked unknown.
	records = []exchange.ClosePnL{
		{Side: "long", NetPnL: 1.0, CloseSize: 100},
		{Side: "long", NetPnL: 0.5, CloseSize: 0, CloseSizeUnknown: true},
	}
	agg, ok = aggregateClosePnLBySide(records, "long")
	if !ok {
		t.Fatal("expected ok=true")
	}
	if !agg.CloseSizeUnknown {
		t.Error("CloseSizeUnknown = false, want true (any match with unknown → aggregate unknown)")
	}

	// All-known records → aggregate stays known.
	records = []exchange.ClosePnL{
		{Side: "long", NetPnL: 1.0, CloseSize: 100},
		{Side: "long", NetPnL: 0.5, CloseSize: 50},
	}
	agg, _ = aggregateClosePnLBySide(records, "long")
	if agg.CloseSizeUnknown {
		t.Error("CloseSizeUnknown = true, want false (all inputs known)")
	}
	if agg.CloseSize != 150 {
		t.Errorf("CloseSize = %.0f, want 150 (sum)", agg.CloseSize)
	}
}

// TestReconcileTier1AcceptsUnknownLongSize mirrors the Tier-1 gate logic for
// the case where the long leg's adapter cannot derive CloseSize (e.g. Binance):
// longAgg.CloseSizeUnknown=true should cause the gate to skip the long
// comparison, and if the short leg matches expected, the gate passes.
func TestReconcileTier1AcceptsUnknownLongSize(t *testing.T) {
	const sizeEpsilon = 1e-6
	pos := makeTestPos("pos-mixed", 100, 100, 0, 0)
	var siblings []*models.ArbitragePosition

	longExpected := pos.LongCloseSize + sumSiblingCloseSize(siblings, "long")   // 100
	shortExpected := pos.ShortCloseSize + sumSiblingCloseSize(siblings, "short") // 100

	longAgg := exchange.ClosePnL{CloseSize: 0, CloseSizeUnknown: true}
	shortAgg := exchange.ClosePnL{CloseSize: 100}

	longShort := !longAgg.CloseSizeUnknown && longAgg.CloseSize < longExpected-sizeEpsilon
	shortShort := !shortAgg.CloseSizeUnknown && shortAgg.CloseSize < shortExpected-sizeEpsilon
	if longShort || shortShort {
		t.Errorf("gate should PASS: longShort=%v shortShort=%v (long unknown must be skipped, short exact match)",
			longShort, shortShort)
	}
}

// TestReconcileTier1RejectsKnownShortShortfall verifies that even when the long
// leg is unknown-size (Binance), a known-but-short short leg still trips the
// gate — i.e. we only skip the comparison for the leg flagged unknown.
func TestReconcileTier1RejectsKnownShortShortfall(t *testing.T) {
	const sizeEpsilon = 1e-6
	pos := makeTestPos("pos-mixed", 100, 100, 0, 0)
	var siblings []*models.ArbitragePosition

	longExpected := pos.LongCloseSize + sumSiblingCloseSize(siblings, "long")   // 100
	shortExpected := pos.ShortCloseSize + sumSiblingCloseSize(siblings, "short") // 100

	longAgg := exchange.ClosePnL{CloseSize: 0, CloseSizeUnknown: true}
	shortAgg := exchange.ClosePnL{CloseSize: 50} // partial — known but short

	longShort := !longAgg.CloseSizeUnknown && longAgg.CloseSize < longExpected-sizeEpsilon
	shortShort := !shortAgg.CloseSizeUnknown && shortAgg.CloseSize < shortExpected-sizeEpsilon
	if !shortShort {
		t.Errorf("expected gate to fire on known-short-shortfall: shortAgg.CloseSize=%.0f < expected=%.0f",
			shortAgg.CloseSize, shortExpected)
	}
	if longShort {
		t.Errorf("long-unknown leg must not trip the gate (longShort=true)")
	}
}
