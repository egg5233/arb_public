package pricegaptrader

import (
	"fmt"
	"math"
	"strconv"
	"sync"
	"time"

	"arb/internal/models"
	"arb/pkg/exchange"
)

// fillResult — per-leg fill outcome captured concurrently by openPair.
type fillResult struct {
	orderID string
	filled  float64 // base-asset units actually filled
	price   float64 // best-effort fill price (mid at decision — adapter doesn't expose vwap)
	err     error
}

// openPair places simultaneous IOC market orders on both legs and returns the
// persisted PriceGapPosition, or an error if the pair could not be opened.
//
// Partial-fill reconciliation (D-10): if legs filled unevenly, unwind the
// over-filled leg via MARKET order (not IOC — positions must close fully per
// §Pitfall 4). If one leg filled zero, immediately market-close the other.
//
// Circuit breaker (D-10): 5 consecutive PlaceOrder failures sets t.paused=true.
//
// Adapter signature note: pkg/exchange.GetOrderFilledQty returns (qty, err) with
// no vwap. We stamp the per-leg price using det.MidLong / det.MidShort (mid at
// decision) as a best-effort fill price; realized slippage is computed at exit
// from actual PnL in Plan 06.
func (t *Tracker) openPair(
	cand models.PriceGapCandidate,
	sizeBase float64, // base-asset units, per-leg (both legs opened with same size)
	det DetectionResult,
) (*models.PriceGapPosition, error) {

	if t.isCircuitOpen() {
		return nil, ErrPriceGapCircuitBreaker
	}

	// Per-symbol lock via Redis so a second tick can't race us.
	lockRes := cand.Symbol + ":" + cand.LongExch + ":" + cand.ShortExch
	token, ok, err := t.db.AcquirePriceGapLock(lockRes, 30*time.Second)
	if err != nil || !ok {
		return nil, fmt.Errorf("openPair: lock busy: %v", err)
	}
	defer func() { _ = t.db.ReleasePriceGapLock(lockRes, token) }()

	longEx, ok := t.exchanges[cand.LongExch]
	if !ok {
		return nil, fmt.Errorf("openPair: long exchange %q not configured", cand.LongExch)
	}
	shortEx, ok := t.exchanges[cand.ShortExch]
	if !ok {
		return nil, fmt.Errorf("openPair: short exchange %q not configured", cand.ShortExch)
	}

	decL := t.sizeDecimals(cand.LongExch, cand.Symbol)
	decS := t.sizeDecimals(cand.ShortExch, cand.Symbol)

	longCh := make(chan fillResult, 1)
	shortCh := make(chan fillResult, 1)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		longCh <- t.placeLeg(longEx, cand.Symbol, exchange.SideBuy, sizeBase, decL, "ioc", det.MidLong)
	}()
	go func() {
		defer wg.Done()
		shortCh <- t.placeLeg(shortEx, cand.Symbol, exchange.SideSell, sizeBase, decS, "ioc", det.MidShort)
	}()
	wg.Wait()

	lr := <-longCh
	sr := <-shortCh

	// Circuit-breaker accounting — any failure bumps; a success on that leg resets.
	if lr.err != nil {
		t.bumpFailures()
	} else {
		t.resetFailures()
	}
	if sr.err != nil {
		t.bumpFailures()
	} else {
		t.resetFailures()
	}

	// Both errored → nothing to clean up.
	if lr.err != nil && sr.err != nil {
		return nil, fmt.Errorf("openPair: both legs failed (long=%v, short=%v)", lr.err, sr.err)
	}

	// One leg zero-fill → market-close the other (D-10, §Pitfall 4).
	if lr.filled == 0 && sr.filled > 0 {
		// Closing short = BUY back.
		t.closeLegMarket(shortEx, cand.Symbol, exchange.SideBuy, sr.filled, decS)
		return nil, fmt.Errorf("openPair: long zero-filled, short closed defensively")
	}
	if sr.filled == 0 && lr.filled > 0 {
		// Closing long = SELL.
		t.closeLegMarket(longEx, cand.Symbol, exchange.SideSell, lr.filled, decL)
		return nil, fmt.Errorf("openPair: short zero-filled, long closed defensively")
	}
	if lr.filled == 0 && sr.filled == 0 {
		return nil, fmt.Errorf("openPair: both legs zero-filled")
	}

	// Unwind to min(filled) — D-10 "unwind-to-match". Unwind uses MARKET
	// (not IOC) per §Pitfall 4 to guarantee completion.
	match := math.Min(lr.filled, sr.filled)
	if lr.filled > match {
		t.closeLegMarket(longEx, cand.Symbol, exchange.SideSell, lr.filled-match, decL)
	}
	if sr.filled > match {
		t.closeLegMarket(shortEx, cand.Symbol, exchange.SideBuy, sr.filled-match, decS)
	}

	// Persist the matched position.
	posID := fmt.Sprintf("pg_%s_%s_%s_%d", cand.Symbol, cand.LongExch, cand.ShortExch, time.Now().UnixNano())
	pos := &models.PriceGapPosition{
		ID:                 posID,
		Symbol:             cand.Symbol,
		LongExchange:       cand.LongExch,
		ShortExchange:      cand.ShortExch,
		Status:             models.PriceGapStatusOpen,
		EntrySpreadBps:     det.SpreadBps,
		ThresholdBps:       cand.ThresholdBps,
		NotionalUSDT:       match * ((lr.price + sr.price) / 2.0),
		LongFillPrice:      lr.price,
		ShortFillPrice:     sr.price,
		LongSize:           match,
		ShortSize:          match,
		LongMidAtDecision:  det.MidLong,
		ShortMidAtDecision: det.MidShort,
		ModeledSlipBps:     cand.ModeledSlippageBps,
		OpenedAt:           time.Now(),
	}
	if err := t.db.SavePriceGapPosition(pos); err != nil {
		t.log.Warn("pricegap: SavePriceGapPosition failed for %s: %v", posID, err)
	}
	return pos, nil
}

// placeLeg submits a single leg and returns a fillResult populated from
// GetOrderFilledQty. The fill price is stamped as the caller-supplied mid
// (det.MidLong / det.MidShort) because the adapter interface returns only qty.
func (t *Tracker) placeLeg(
	ex exchange.Exchange, symbol string, side exchange.Side,
	sizeBase float64, decimals int, force string, fillPrice float64,
) fillResult {
	params := exchange.PlaceOrderParams{
		Symbol:    symbol,
		Side:      side,
		OrderType: "market",
		Size:      strconv.FormatFloat(roundStep(sizeBase, decimals), 'f', decimals, 64),
		Force:     force,
	}
	orderID, err := ex.PlaceOrder(params)
	if err != nil {
		return fillResult{err: err}
	}
	// Adapter returns (qty, err) — no vwap. Use caller-supplied mid as fill price.
	qty, err := ex.GetOrderFilledQty(orderID, symbol)
	if err != nil {
		return fillResult{orderID: orderID, err: fmt.Errorf("fill fetch: %w", err)}
	}
	return fillResult{orderID: orderID, filled: qty, price: fillPrice}
}

// closeLegMarket submits a MARKET order (no IOC) with ReduceOnly for exit/unwind.
// Returns best-effort; errors are logged and bump the failure counter.
func (t *Tracker) closeLegMarket(ex exchange.Exchange, symbol string, side exchange.Side, size float64, decimals int) {
	if size <= 0 {
		return
	}
	_, err := ex.PlaceOrder(exchange.PlaceOrderParams{
		Symbol:     symbol,
		Side:       side,
		OrderType:  "market",
		Size:       strconv.FormatFloat(roundStep(size, decimals), 'f', decimals, 64),
		ReduceOnly: true,
	})
	if err != nil {
		t.log.Warn("pricegap: closeLegMarket failed on %s %s: %v", symbol, side, err)
		t.bumpFailures()
	}
}

// Circuit breaker: 5 consecutive failures → paused=true until operator reset (Phase 9).
func (t *Tracker) bumpFailures() {
	t.failMu.Lock()
	defer t.failMu.Unlock()
	t.failures++
	if t.failures >= 5 && !t.paused {
		t.paused = true
		t.log.Warn("pricegap: CIRCUIT BREAKER OPEN after %d failures — manual reset required", t.failures)
	}
}

func (t *Tracker) resetFailures() {
	t.failMu.Lock()
	defer t.failMu.Unlock()
	t.failures = 0
}

func (t *Tracker) isCircuitOpen() bool {
	t.failMu.Lock()
	defer t.failMu.Unlock()
	return t.paused
}

// sizeDecimals — look up ContractInfo.SizeDecimals; fall back to 6 on miss.
func (t *Tracker) sizeDecimals(exName, symbol string) int {
	ex, ok := t.exchanges[exName]
	if !ok {
		return 6
	}
	contracts, err := ex.LoadAllContracts()
	if err != nil || contracts == nil {
		return 6
	}
	if ci, ok := contracts[symbol]; ok {
		return ci.SizeDecimals
	}
	return 6
}

// roundStep — floor to the given decimal places.
func roundStep(v float64, decimals int) float64 {
	f := math.Pow(10, float64(decimals))
	return math.Floor(v*f) / f
}
