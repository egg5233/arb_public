package pricegaptrader

import (
	"context"
	"math"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"arb/internal/models"
	"arb/pkg/exchange"
)

// monitorSeq — global atomic counter; every startMonitor invocation mints a
// unique token. The goroutine holds its own token; the cleanup defer compares
// against the currently-registered token to skip delete-on-exit when a newer
// startMonitor has already replaced it (rehydrate-twice idempotency path).
var monitorSeq int64

// monitorHandle — stored in Tracker.monitors. Combines the cancel func with
// the unique seq token so cleanup can detect replacement without relying on
// closure-pointer identity (which is unreliable for context.CancelFunc).
type monitorHandle struct {
	cancel context.CancelFunc
	seq    int64
}

// startMonitor spawns a per-position goroutine that watches for exit triggers.
// Called from openPair (on new position) and rehydrate (on startup).
//
// Idempotent: if a monitor already exists for the same posID it is cancelled
// and replaced. The replacement gets a new seq token; the old goroutine's
// cleanup defer compares seqs and will NOT delete the replacement's entry —
// prevents rehydrate-twice race (RESEARCH §Pitfall 3).
func (t *Tracker) startMonitor(pos *models.PriceGapPosition) {
	ctx, cancel := context.WithCancel(context.Background())
	mySeq := atomic.AddInt64(&monitorSeq, 1)

	t.monMu.Lock()
	if prev, exists := t.monitorHandles[pos.ID]; exists {
		prev.cancel()
	}
	t.monitorHandles[pos.ID] = monitorHandle{cancel: cancel, seq: mySeq}
	t.monitors[pos.ID] = cancel // keep legacy Stop()-path key in sync
	t.monMu.Unlock()

	t.exitWG.Add(1)
	go t.monitorPosition(ctx, pos, mySeq)
}

// monitorPosition polls at PriceGapPollIntervalSec cadence; fires exit on:
//
//	(a) |spread_bps| ≤ ThresholdBps * PriceGapExitReversionFactor   [reverted]
//	(b) time.Since(OpenedAt) ≥ PriceGapMaxHoldMin minutes            [max_hold]
//
// The per-tick work is factored into checkAndMaybeExit so tests can drive it
// synchronously without relying on ticker timing.
//
// mySeq is compared against the currently-registered handle seq in the cleanup
// defer: only the owning generation deletes the map entry, preventing a stale
// goroutine from clobbering a newer replacement (rehydrate-twice idempotency).
func (t *Tracker) monitorPosition(ctx context.Context, pos *models.PriceGapPosition, mySeq int64) {
	defer t.exitWG.Done()
	defer func() {
		t.monMu.Lock()
		if cur, ok := t.monitorHandles[pos.ID]; ok && cur.seq == mySeq {
			delete(t.monitorHandles, pos.ID)
			delete(t.monitors, pos.ID)
		}
		t.monMu.Unlock()
	}()

	interval := time.Duration(t.cfg.PriceGapPollIntervalSec) * time.Second
	if interval <= 0 {
		interval = 30 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.stopCh:
			return
		case now := <-ticker.C:
			if t.checkAndMaybeExit(pos, now) {
				return
			}
		}
	}
}

// checkAndMaybeExit performs one tick's exit evaluation. Returns true when the
// position was closed (caller exits monitor loop), false otherwise.
//
// Exit triggers (D-11):
//   - max-hold: now - OpenedAt >= PriceGapMaxHoldMin
//   - reversion: |spread_bps| <= ThresholdBps * PriceGapExitReversionFactor
//
// Extracted for test-injectability per Plan 06 task 4 note (b).
func (t *Tracker) checkAndMaybeExit(pos *models.PriceGapPosition, now time.Time) bool {
	maxHold := time.Duration(t.cfg.PriceGapMaxHoldMin) * time.Minute
	if maxHold > 0 && now.Sub(pos.OpenedAt) >= maxHold {
		t.closePair(pos, models.ExitReasonMaxHold)
		return true
	}

	reversionFactor := t.cfg.PriceGapExitReversionFactor
	if reversionFactor <= 0 {
		reversionFactor = 0.5
	}

	cand := models.PriceGapCandidate{
		Symbol:       pos.Symbol,
		LongExch:     pos.LongExchange,
		ShortExch:    pos.ShortExchange,
		ThresholdBps: pos.ThresholdBps,
	}
	midL, midS, _, err := sampleLegs(t.exchanges, cand)
	if err != nil {
		t.log.Warn("pricegap: monitor sampleLegs %s: %v", pos.ID, err)
		return false
	}
	spread := computeSpreadBps(midL, midS)
	if math.Abs(spread) <= pos.ThresholdBps*reversionFactor {
		t.closePair(pos, models.ExitReasonReverted)
		return true
	}
	return false
}

// closePair closes both legs (simultaneous IOC; remainder via MARKET per D-12),
// computes realized PnL + realized slippage, persists with ExitReason, moves to
// history, and follows up with the exec-quality rolling-window check.
//
// D-10 §Pitfall 4 rationale for MARKET (not IOC) on the remainder: the position
// must close fully — IOC on the retry can itself leave dust and strand delta.
func (t *Tracker) closePair(pos *models.PriceGapPosition, reason string) {
	pos.Status = models.PriceGapStatusExiting
	_ = t.db.SavePriceGapPosition(pos) // best-effort "exiting" state write

	longEx, ok := t.exchanges[pos.LongExchange]
	if !ok {
		t.log.Warn("pricegap: closePair %s: long exchange %q missing", pos.ID, pos.LongExchange)
		return
	}
	shortEx, ok := t.exchanges[pos.ShortExchange]
	if !ok {
		t.log.Warn("pricegap: closePair %s: short exchange %q missing", pos.ID, pos.ShortExchange)
		return
	}
	decL := t.sizeDecimals(pos.LongExchange, pos.Symbol)
	decS := t.sizeDecimals(pos.ShortExchange, pos.Symbol)

	// Simultaneous IOC ReduceOnly on both legs (D-12).
	var wg sync.WaitGroup
	var lr, sr fillResult
	wg.Add(2)
	go func() {
		defer wg.Done()
		lr = t.placeCloseLegIOC(longEx, pos.Symbol, exchange.SideSell, pos.LongSize, decL, pos.LongMidAtDecision)
	}()
	go func() {
		defer wg.Done()
		sr = t.placeCloseLegIOC(shortEx, pos.Symbol, exchange.SideBuy, pos.ShortSize, decS, pos.ShortMidAtDecision)
	}()
	wg.Wait()

	// Remainder via MARKET ReduceOnly (D-12 §Pitfall 4).
	if lr.filled < pos.LongSize {
		rem := pos.LongSize - lr.filled
		t.closeLegMarket(longEx, pos.Symbol, exchange.SideSell, rem, decL)
		lr.filled = pos.LongSize
	}
	if sr.filled < pos.ShortSize {
		rem := pos.ShortSize - sr.filled
		t.closeLegMarket(shortEx, pos.Symbol, exchange.SideBuy, rem, decS)
		sr.filled = pos.ShortSize
	}

	// Defensive: if any leg's fill price is unpopulated, fall back to mid-at-decision
	// so the downstream PnL / slippage math stays finite.
	longExitPrice := lr.price
	if longExitPrice == 0 {
		longExitPrice = pos.LongMidAtDecision
	}
	shortExitPrice := sr.price
	if shortExitPrice == 0 {
		shortExitPrice = pos.ShortMidAtDecision
	}

	// Realized PnL (delta-neutral, USDT-denominated):
	//   long leg:  (exit_long  - entry_long)  * size
	//   short leg: (entry_short - exit_short) * size
	pos.RealizedPnL =
		(longExitPrice-pos.LongFillPrice)*pos.LongSize +
			(pos.ShortFillPrice-shortExitPrice)*pos.ShortSize

	// Realized slippage as round-trip bps (D-21): entry + exit asymmetry on both legs.
	// Guarded: zero mids (bad data) produce zero contributions, not NaN.
	var entryLongBps, entryShortBps, exitLongBps, exitShortBps float64
	if pos.LongMidAtDecision > 0 {
		entryLongBps = (pos.LongFillPrice - pos.LongMidAtDecision) / pos.LongMidAtDecision * 10_000.0
		exitLongBps = (longExitPrice - pos.LongMidAtDecision) / pos.LongMidAtDecision * 10_000.0
	}
	if pos.ShortMidAtDecision > 0 {
		entryShortBps = (pos.ShortMidAtDecision - pos.ShortFillPrice) / pos.ShortMidAtDecision * 10_000.0
		exitShortBps = (pos.ShortMidAtDecision - shortExitPrice) / pos.ShortMidAtDecision * 10_000.0
	}
	pos.RealizedSlipBps = entryLongBps + entryShortBps + exitLongBps + exitShortBps

	pos.ExitReason = reason
	pos.Status = models.PriceGapStatusClosed
	pos.ClosedAt = time.Now()
	if err := t.db.SavePriceGapPosition(pos); err != nil {
		t.log.Warn("pricegap: SavePriceGapPosition(closed) failed for %s: %v", pos.ID, err)
	}
	_ = t.db.RemoveActivePriceGapPosition(pos.ID)
	_ = t.db.AddPriceGapHistory(pos)

	// Exec-quality rolling-window follow-up (PG-RISK-03 — see slippage.go).
	t.recordSlippageAndMaybeDisable(pos)

	t.log.Info("pricegap: closed %s reason=%s pnl=%.4f slipBps=%.1f",
		pos.ID, reason, pos.RealizedPnL, pos.RealizedSlipBps)
}

// vwapReader is an optional interface an exchange adapter MAY implement to
// expose the volume-weighted fill price for a given orderID. The 6 production
// CEX adapters do NOT implement it today (GetOrderFilledQty returns only qty),
// so the default path falls back to the caller-supplied mid as the exit fill
// price. Tests implement this interface on stubExchange to drive exact exit
// prices for PnL math assertions.
type vwapReader interface {
	GetOrderVwap(orderID string) (float64, bool)
}

// placeCloseLegIOC submits a ReduceOnly IOC market order for the close direction.
// Fill qty is read back via GetOrderFilledQty. Exit price preference:
//  1. adapter implements vwapReader and returns a hit     → use that vwap
//  2. otherwise                                          → fall back to mid-at-decision
//
// Rationale: production adapters don't expose vwap today, so realized slippage
// at exit is approximated against the entry mid (D-21); this preserves the
// sign and magnitude of any adverse selection between entry and exit. Tests
// inject vwap via the interface to assert exact PnL math.
func (t *Tracker) placeCloseLegIOC(
	ex exchange.Exchange, symbol string, side exchange.Side,
	size float64, decimals int, fallbackPrice float64,
) fillResult {
	if size <= 0 {
		return fillResult{}
	}
	params := exchange.PlaceOrderParams{
		Symbol:     symbol,
		Side:       side,
		OrderType:  "market",
		Size:       strconv.FormatFloat(roundStep(size, decimals), 'f', decimals, 64),
		Force:      "ioc",
		ReduceOnly: true,
	}
	orderID, err := ex.PlaceOrder(params)
	if err != nil {
		t.bumpFailures()
		return fillResult{err: err}
	}
	qty, err := ex.GetOrderFilledQty(orderID, symbol)
	if err != nil {
		return fillResult{orderID: orderID, err: err}
	}
	price := fallbackPrice
	if vr, ok := ex.(vwapReader); ok {
		if v, hit := vr.GetOrderVwap(orderID); hit && v > 0 {
			price = v
		}
	}
	return fillResult{orderID: orderID, filled: qty, price: price}
}
