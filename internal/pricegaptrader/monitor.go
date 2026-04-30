package pricegaptrader

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"arb/internal/models"
	"arb/pkg/exchange"
)

// PAPER-MODE INVARIANT (Plan 09-03 Pitfall 2): monitor.go NEVER reads the
// global paper-mode config flag. Mode is stamped once at entry
// (execution.go openPair); this file reads pos.Mode exclusively so that
// flipping the global flag mid-life cannot flip an in-flight position
// from paper -> live close or vice versa.

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
	// pos.Mode drives the paper/live branch inside placeCloseLegIOC (Pitfall 2).
	var wg sync.WaitGroup
	var lr, sr fillResult
	wg.Add(2)
	go func() {
		defer wg.Done()
		lr = t.placeCloseLegIOC(longEx, pos, exchange.SideSell, pos.LongSize, decL, pos.LongMidAtDecision)
	}()
	go func() {
		defer wg.Done()
		sr = t.placeCloseLegIOC(shortEx, pos, exchange.SideBuy, pos.ShortSize, decS, pos.ShortMidAtDecision)
	}()
	wg.Wait()

	// Remainder via MARKET ReduceOnly (D-12 §Pitfall 4). In paper mode the IOC
	// leg synthesizes a full fill, so the remainder branches below are skipped;
	// the live path is unchanged.
	if lr.filled < pos.LongSize {
		rem := pos.LongSize - lr.filled
		t.closeLegMarketForPos(longEx, pos, exchange.SideSell, rem, decL, pos.LongMidAtDecision)
		lr.filled = pos.LongSize
	}
	if sr.filled < pos.ShortSize {
		rem := pos.ShortSize - sr.filled
		t.closeLegMarketForPos(shortEx, pos, exchange.SideBuy, rem, decS, pos.ShortMidAtDecision)
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

	// Realized slippage as round-trip bps (D-21 / D-23): entry + exit asymmetry
	// on both legs, stamped onto pos before AddPriceGapHistory so every history
	// row carries both modeled (set at entry in execution.go openPair) and
	// realized (set here) bps — Plan 05 reads realized-vs-modeled directly
	// from pg:history (D-24 simplification).
	//
	// Plan 03 paper-mode coupling: when the close path is the synthesized paper
	// close (fills at mid ± modeled/2), Plan 03 assigns
	// pos.RealizedSlipBps := pos.ModeledSlipBps at the synth point; the live
	// path below computes the measured value.
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

	// PG-VAL-03 (v0.34.10 closure): in paper mode the synth fills are exactly
	// `mid ± ModeledSlip/2` on both legs at both entry and exit. With no real
	// price drift between decision and close, entry slip (+ModeledSlip) and
	// exit slip (-ModeledSlip vs ENTRY mid, since the formula above uses
	// LongMidAtDecision/ShortMidAtDecision as the exit reference) cancel
	// algebraically and `pos.RealizedSlipBps` collapses to machine-zero —
	// hiding what the operator actually modeled. Override with pos.ModeledSlipBps
	// so the slippage column reflects intent. Live positions are unaffected;
	// the formula above is the source of truth for them.
	if pos.Mode == models.PriceGapModePaper {
		pos.RealizedSlipBps = pos.ModeledSlipBps
	}

	pos.ExitReason = reason
	pos.Status = models.PriceGapStatusClosed
	pos.ClosedAt = time.Now()

	// Phase 14 D-02: dated index for O(1) reconcile lookup. Best-effort
	// (matches existing pattern at lines below). ExchangeClosedAt is preferred
	// when populated; we fall back to ClosedAt (local clock) per D-10 — the
	// reconciler flags anomaly_missing_close_ts in the latter case.
	closeDate := pos.ExchangeClosedAt
	if closeDate.IsZero() {
		closeDate = pos.ClosedAt
	}
	if err := t.db.AddPriceGapClosedPositionForDate(pos.ID, closeDate); err != nil {
		t.log.Warn("pricegap: AddPriceGapClosedPositionForDate failed for %s: %v", pos.ID, err)
	}

	if err := t.db.SavePriceGapPosition(pos); err != nil {
		t.log.Warn("pricegap: SavePriceGapPosition(closed) failed for %s: %v", pos.ID, err)
	}
	_ = t.db.RemoveActivePriceGapPosition(pos.ID)
	_ = t.db.AddPriceGapHistory(pos)

	// Phase 9 Plan 06 exit hook — broadcast + Telegram AFTER history is LPUSHed
	// so consumers see the persisted final state. Both are nil-safe (Noops
	// installed at construction). Paper and live fire identically (D-22).
	duration := time.Since(pos.OpenedAt)
	t.broadcaster.BroadcastPriceGapEvent(PriceGapEvent{Type: "exit", Position: pos, Reason: reason})
	t.notifier.NotifyPriceGapExit(pos, reason, pos.RealizedPnL, duration)
	t.maybeBroadcastPositions()

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
//
// Paper-mode chokepoint (Plan 09-03 Pattern 2): when pos.Mode == "paper",
// the function synthesizes a close-leg fill at mid ± (pos.ModeledSlipBps / 2)
// and returns WITHOUT calling ex.PlaceOrder. The branch is gated on pos.Mode
// (not on the global paper-mode flag) so a mid-life flag flip cannot leak
// real orders into the wire for an in-flight paper position (Pitfall 2).
func (t *Tracker) placeCloseLegIOC(
	ex exchange.Exchange, pos *models.PriceGapPosition, side exchange.Side,
	size float64, decimals int, fallbackPrice float64,
) fillResult {
	if size <= 0 {
		return fillResult{}
	}
	symbol := pos.Symbol
	if pos.Mode == models.PriceGapModePaper {
		adverse := fallbackPrice * (pos.ModeledSlipBps / 2.0) / 10_000.0
		synth := fallbackPrice
		if side == exchange.SideBuy {
			synth = fallbackPrice + adverse
		}
		if side == exchange.SideSell {
			synth = fallbackPrice - adverse
		}
		return fillResult{
			orderID: fmt.Sprintf("paper_close_%s_%d", symbol, time.Now().UnixNano()),
			filled:  roundStep(size, decimals),
			price:   synth,
		}
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

// closeLegMarketForPos wraps closeLegMarket with paper-mode branching on pos.Mode.
// In paper mode it is a no-op (the IOC synth already filled the full size);
// live mode delegates to closeLegMarket exactly as before. Split out so the
// mode-read invariant lives in one place per package (Pitfall 2).
func (t *Tracker) closeLegMarketForPos(
	ex exchange.Exchange, pos *models.PriceGapPosition, side exchange.Side,
	size float64, decimals int, fallbackPrice float64,
) {
	if size <= 0 {
		return
	}
	if pos.Mode == models.PriceGapModePaper {
		// Paper IOC synth always returns a full fill; remainder branch is a no-op.
		// Keep symmetry with live path; no exchange call is made.
		_ = fallbackPrice
		return
	}
	t.closeLegMarket(ex, pos.Symbol, side, size, decimals)
}
