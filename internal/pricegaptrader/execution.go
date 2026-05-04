package pricegaptrader

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"arb/internal/models"
	"arb/pkg/exchange"
	"arb/pkg/utils"
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

	// Per-symbol lock via Redis so a second tick can't race us. Lock key uses
	// the CONFIGURED tuple (not wire-side) so a single bidirectional candidate
	// cannot fire forward and inverse concurrently — preserves T-08-08
	// single-fire-per-candidate invariant. PG-DIR-01.
	lockRes := cand.Symbol + ":" + cand.LongExch + ":" + cand.ShortExch
	token, ok, err := t.db.AcquirePriceGapLock(lockRes, 30*time.Second)
	if err != nil || !ok {
		return nil, fmt.Errorf("openPair: lock busy: %v", err)
	}
	defer func() { _ = t.db.ReleasePriceGapLock(lockRes, token) }()

	paperMode, paperModeErr := t.IsPaperModeActive(context.TODO())
	if paperModeErr != nil {
		t.log.Warn("execution: IsPaperModeActive site=entry-open returned err, fail-safe paper=true: %v", paperModeErr)
	}

	// PG-DIR-01: resolve wire-side leg roles. For bidirectional candidates an
	// inverse-sign fire (det.SpreadBps < 0 means configured short_exch is
	// cheaper than configured long_exch) flips the wire-side roles: BUY on
	// configured short_exch, SELL on configured long_exch. Pinned candidates
	// always trade configured-side (detector enforces positive-sign-only via
	// allExceedDirected).
	//
	// The lock key (lockRes above) and CandidateLongExch/CandidateShortExch on
	// the persisted position preserve the CONFIGURED tuple — these are what
	// Phase 10's active-position guard matches against (Pitfall 5).
	firedDirection := models.PriceGapFiredForward
	wireLongExch := cand.LongExch
	wireShortExch := cand.ShortExch
	wireMidLong := det.MidLong
	wireMidShort := det.MidShort
	if cand.Direction == models.PriceGapDirectionBidirectional && det.SpreadBps < 0 {
		firedDirection = models.PriceGapFiredInverse
		wireLongExch, wireShortExch = cand.ShortExch, cand.LongExch
		wireMidLong, wireMidShort = det.MidShort, det.MidLong
	}

	longEx, ok := t.exchanges[wireLongExch]
	if !ok {
		return nil, fmt.Errorf("openPair: long exchange %q not configured", wireLongExch)
	}
	shortEx, ok := t.exchanges[wireShortExch]
	if !ok {
		return nil, fmt.Errorf("openPair: short exchange %q not configured", wireShortExch)
	}

	decL := t.sizeDecimals(wireLongExch, cand.Symbol)
	decS := t.sizeDecimals(wireShortExch, cand.Symbol)

	if err := t.preflightBingXPriceGapEntry(longEx, wireLongExch, cand.Symbol, exchange.SideBuy, sizeBase, decL, wireMidLong, paperMode); err != nil {
		t.bumpFailures()
		return nil, err
	}
	if err := t.preflightBingXPriceGapEntry(shortEx, wireShortExch, cand.Symbol, exchange.SideSell, sizeBase, decS, wireMidShort, paperMode); err != nil {
		t.bumpFailures()
		return nil, err
	}

	longCh := make(chan fillResult, 1)
	shortCh := make(chan fillResult, 1)

	// Paper-mode synthesis uses candidate's ModeledSlippageBps — derived from
	// the Phase 8 detector and already stamped on pos.ModeledSlipBps below.
	modeledEdgeBps := cand.ModeledSlippageBps

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		longCh <- t.placeLeg(longEx, cand.Symbol, exchange.SideBuy, sizeBase, decL, "ioc", wireMidLong, modeledEdgeBps, paperMode)
	}()
	go func() {
		defer wg.Done()
		shortCh <- t.placeLeg(shortEx, cand.Symbol, exchange.SideSell, sizeBase, decS, "ioc", wireMidShort, modeledEdgeBps, paperMode)
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
		t.closeLegMarketWithMode(shortEx, cand.Symbol, exchange.SideBuy, sr.filled, decS, paperMode)
		return nil, fmt.Errorf("openPair: long zero-filled, short closed defensively")
	}
	if sr.filled == 0 && lr.filled > 0 {
		// Closing long = SELL.
		t.closeLegMarketWithMode(longEx, cand.Symbol, exchange.SideSell, lr.filled, decL, paperMode)
		return nil, fmt.Errorf("openPair: short zero-filled, long closed defensively")
	}
	if lr.filled == 0 && sr.filled == 0 {
		return nil, fmt.Errorf("openPair: both legs zero-filled")
	}

	// Unwind to min(filled) — D-10 "unwind-to-match". Unwind uses MARKET
	// (not IOC) per §Pitfall 4 to guarantee completion.
	match := math.Min(lr.filled, sr.filled)
	if lr.filled > match {
		t.closeLegMarketWithMode(longEx, cand.Symbol, exchange.SideSell, lr.filled-match, decL, paperMode)
	}
	if sr.filled > match {
		t.closeLegMarketWithMode(shortEx, cand.Symbol, exchange.SideBuy, sr.filled-match, decS, paperMode)
	}

	// Persist the matched position.
	//
	// Mode is stamped ONCE at entry (Pitfall 2 — immutability through lifecycle).
	// All downstream monitors / close paths read pos.Mode, NEVER the global flag
	// mid-life. Phase 15 D-07: paper-mode is read once through the chokepoint
	// helper so the breaker's sticky flag participates without leg divergence.
	mode := models.PriceGapModeLive
	if paperMode {
		mode = models.PriceGapModePaper
	}
	// posID uses CONFIGURED tuple (cand.LongExch/cand.ShortExch) — preserves
	// Phase 8 D-15 ID format and Phase 10 D-11 tuple identity (Pitfall 5).
	posID := fmt.Sprintf("pg_%s_%s_%s_%d", cand.Symbol, cand.LongExch, cand.ShortExch, time.Now().UnixNano())
	pos := &models.PriceGapPosition{
		ID:                 posID,
		Symbol:             cand.Symbol,
		LongExchange:       wireLongExch,   // wire-side (may differ from configured for inverse fire)
		ShortExchange:      wireShortExch,  // wire-side (may differ from configured for inverse fire)
		CandidateLongExch:  cand.LongExch,  // configured tuple — for Phase 10 D-11 guard
		CandidateShortExch: cand.ShortExch, // configured tuple — for Phase 10 D-11 guard
		FiredDirection:     firedDirection, // PG-DIR-01: "forward" | "inverse"
		Status:             models.PriceGapStatusOpen,
		Mode:               mode,
		EntrySpreadBps:     det.SpreadBps, // KEEP signed value — analytics needs the sign
		ThresholdBps:       cand.ThresholdBps,
		NotionalUSDT:       match * ((lr.price + sr.price) / 2.0),
		LongFillPrice:      lr.price, // wire-side fill
		ShortFillPrice:     sr.price, // wire-side fill
		LongSize:           match,
		ShortSize:          match,
		LongMidAtDecision:  wireMidLong,  // wire-side mid (post-swap for inverse)
		ShortMidAtDecision: wireMidShort, // wire-side mid (post-swap for inverse)
		ModeledSlipBps:     cand.ModeledSlippageBps,
		OpenedAt:           time.Now(),
	}
	if err := t.db.SavePriceGapPosition(pos); err != nil {
		t.log.Warn("pricegap: SavePriceGapPosition failed for %s: %v", posID, err)
	}

	// Phase 9 Plan 06 entry hook — broadcast + Telegram AFTER persistence so
	// consumers see the stored state. Both sides are nil-safe (NoopBroadcaster
	// / NoopNotifier defaults; TelegramNotifier nil-receiver safe per Plan 04).
	// Paper-mode fires identically (D-22 — paper parity).
	t.broadcaster.BroadcastPriceGapEvent(PriceGapEvent{Type: "entry", Position: pos})
	t.notifier.NotifyPriceGapEntry(pos)
	t.maybeBroadcastPositions()
	return pos, nil
}

// placeLeg submits a single leg and returns a fillResult populated from
// GetOrderFilledQty. The fill price is stamped as the caller-supplied mid
// (det.MidLong / det.MidShort) because the adapter interface returns only qty.
//
// Paper-mode chokepoint (Plan 09-03 Pattern 2; Phase 15 D-07 migration):
// when paper-mode is active (cfg flag OR breaker's sticky flag), the function
// DOES NOT call ex.PlaceOrder. It synthesizes a fill at mid ± (modeledEdgeBps / 2)
// so realized slippage is non-zero and the Phase 8 slippage pipeline is fully
// exercised (Pitfall 7). This is the SINGLE divergence point between paper and
// live on the entry path; openPair's lock acquisition, circuit-breaker
// accounting, and persistence all run identically in paper mode.
func (t *Tracker) placeLeg(
	ex exchange.Exchange, symbol string, side exchange.Side,
	sizeBase float64, decimals int, force string, fillPrice float64, modeledEdgeBps float64, paperMode bool,
) fillResult {
	if paperMode {
		adverse := fillPrice * (modeledEdgeBps / 2.0) / 10_000.0
		synth := fillPrice
		if side == exchange.SideBuy {
			synth = fillPrice + adverse // buy adverse = higher than mid
		}
		if side == exchange.SideSell {
			synth = fillPrice - adverse // sell adverse = lower than mid
		}
		return fillResult{
			orderID: fmt.Sprintf("paper_%s_%d", symbol, time.Now().UnixNano()),
			filled:  roundStep(sizeBase, decimals),
			price:   synth,
		}
	}
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

func (t *Tracker) preflightBingXPriceGapEntry(
	ex exchange.Exchange,
	exchName, symbol string,
	side exchange.Side,
	sizeBase float64,
	decimals int,
	refPrice float64,
	paperMode bool,
) error {
	// Phase 15 D-07: paper-mode skips the BingX preflight (no live order to test).
	if paperMode || !strings.EqualFold(exchName, "bingx") {
		return nil
	}
	preflight, ok := ex.(exchange.OrderPreflight)
	if !ok {
		return nil
	}
	priceStr, err := priceGapBingXProbePrice(ex, symbol, side, refPrice)
	if err != nil {
		return err
	}
	sizeStr := strconv.FormatFloat(roundStep(sizeBase, decimals), 'f', decimals, 64)
	err = preflight.TestOrder(exchange.PlaceOrderParams{
		Symbol:    symbol,
		Side:      side,
		OrderType: "limit",
		Price:     priceStr,
		Size:      sizeStr,
		Force:     "ioc",
	})
	if err == nil {
		return nil
	}
	return fmt.Errorf("pricegap: bingx preflight blocked %s %s: %w", symbol, side, err)
}

func priceGapBingXProbePrice(ex exchange.Exchange, symbol string, side exchange.Side, refPrice float64) (string, error) {
	if refPrice <= 0 {
		return "", fmt.Errorf("pricegap: bingx preflight invalid reference price for %s: %.8f", symbol, refPrice)
	}
	if side != exchange.SideBuy && side != exchange.SideSell {
		return "", fmt.Errorf("pricegap: bingx preflight invalid side for %s: %s", symbol, side)
	}

	priceStep, priceDecimals := priceGapBingXPriceFormat(ex, symbol)
	probePrice := refPrice * 0.01
	if side == exchange.SideSell {
		probePrice = refPrice * 1.99
	}
	if priceStep > 0 {
		if side == exchange.SideBuy {
			probePrice = utils.RoundToStep(probePrice, priceStep)
			if probePrice <= 0 {
				probePrice = priceStep
			}
		} else {
			probePrice = utils.RoundUpToStep(probePrice, priceStep)
			if probePrice <= refPrice {
				probePrice = utils.RoundUpToStep(refPrice+priceStep, priceStep)
			}
		}
	}

	priceStr := utils.FormatPrice(probePrice, priceDecimals)
	parsed, _ := strconv.ParseFloat(priceStr, 64)
	if parsed <= 0 {
		return "", fmt.Errorf("pricegap: bingx preflight invalid probe price for %s: ref=%.8f probe=%s", symbol, refPrice, priceStr)
	}
	if side == exchange.SideBuy && parsed >= refPrice {
		return "", fmt.Errorf("pricegap: bingx preflight cannot build non-marketable buy probe for %s: ref=%.8f probe=%s", symbol, refPrice, priceStr)
	}
	if side == exchange.SideSell && parsed <= refPrice {
		return "", fmt.Errorf("pricegap: bingx preflight cannot build non-marketable sell probe for %s: ref=%.8f probe=%s", symbol, refPrice, priceStr)
	}
	return priceStr, nil
}

func priceGapBingXPriceFormat(ex exchange.Exchange, symbol string) (float64, int) {
	contracts, err := ex.LoadAllContracts()
	if err == nil {
		if ci, ok := contracts[symbol]; ok && ci.PriceStep > 0 {
			return ci.PriceStep, ci.PriceDecimals
		}
	}
	return 0, 8
}

// closeLegMarket submits a MARKET order (no IOC) with ReduceOnly for exit/unwind.
// Returns best-effort; errors are logged and bump the failure counter.
//
// Paper-mode chokepoint (Plan 09-03 Pattern 2 / D-12; Phase 15 D-07 migration):
// when paper-mode is active (cfg flag OR breaker sticky flag) the live
// ex.PlaceOrder call is skipped. placeLeg synthesizes fills without touching
// the exchange, so there is no real position to reduce; hitting the wire here
// would always be wrong (and was observed to fire reduceOnly market orders
// against non-existent bingx positions in the 2026-04-24 UAT attempt). Every
// closeLegMarket is the legacy global-paper wrapper; openPair snapshots mode
// and calls closeLegMarketWithMode directly to avoid mixed live/paper legs.
func (t *Tracker) closeLegMarket(ex exchange.Exchange, symbol string, side exchange.Side, size float64, decimals int) {
	if size <= 0 {
		return
	}
	paperMode, paperModeErr := t.IsPaperModeActive(context.TODO())
	if paperModeErr != nil {
		t.log.Warn("execution: IsPaperModeActive site=close-leg-paper returned err, fail-safe paper=true: %v", paperModeErr)
	}
	t.closeLegMarketWithMode(ex, symbol, side, size, decimals, paperMode)
}

func (t *Tracker) closeLegMarketWithMode(ex exchange.Exchange, symbol string, side exchange.Side, size float64, decimals int, paperMode bool) {
	if size <= 0 {
		return
	}
	if paperMode {
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
