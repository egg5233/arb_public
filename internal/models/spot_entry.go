package models

import "time"

// SpotEntryCandidate is the per-symbol input the unified cross-strategy entry
// selector consumes for the spot-futures strategy. It is a projection of a
// freshly-filtered spot arbitrage opportunity: enough information for the
// selector to size the position and score its expected value over the
// configured hold horizon, without exposing the raw opportunity cache.
//
// All APR and FeePct fields are expressed as decimal fractions (e.g. 0.12 for
// 12% APR), NOT percentages. Sign conventions for FundingAPR follow the
// direction semantics documented on Direction.
type SpotEntryCandidate struct {
	// Symbol is the internal symbol identifier (e.g. "BTCUSDT"), matching the
	// format used elsewhere in the engine.
	Symbol string `json:"symbol"`

	// BaseCoin is the base asset (e.g. "BTC") — used for spot margin borrow
	// lookup on the executing exchange.
	BaseCoin string `json:"base_coin"`

	// Exchange is the venue the spot-futures position is opened on. The spot
	// and futures legs always share an exchange in this strategy.
	Exchange string `json:"exchange"`

	// Direction encodes which leg of the delta-neutral pair is which.
	//   "borrow_sell_long" — Dir A: long futures + short spot (borrow spot, sell)
	//   "buy_spot_short"   — Dir B: short futures + long spot  (buy spot)
	Direction string `json:"direction"`

	// FundingAPR is the annualized funding rate (decimal fraction) on the leg
	// that receives funding for this direction.
	FundingAPR float64 `json:"funding_apr"`

	// BorrowAPR is the annualized spot-margin borrow cost (decimal fraction).
	// Zero for Direction B (no borrow required).
	BorrowAPR float64 `json:"borrow_apr"`

	// FeePct is the one-time round-trip fee cost (decimal fraction) estimated
	// for this candidate.
	FeePct float64 `json:"fee_pct"`

	// Timestamp records when the source opportunity was captured; the selector
	// uses it together with the discovery freshness window to drop stale
	// candidates.
	Timestamp time.Time `json:"timestamp"`
}

// SpotEntryPlan is the sized, feasibility-checked plan the unified selector
// produces from a SpotEntryCandidate. It carries the exact USDT amount that
// will be reserved (PlannedNotionalUSDT) so the batch reservation and
// scoring stages see the same number that execution will actually commit.
//
// PlannedBaseSize is post-cap and post-futures-step-rounding. PlannedNotionalUSDT
// equals PlannedBaseSize * MidPrice — always derived from the rounded size so
// no silent drift enters the reservation path.
type SpotEntryPlan struct {
	// Candidate is the source candidate this plan was built from.
	Candidate SpotEntryCandidate `json:"candidate"`

	// CapitalBudgetUSDT is the raw per-exchange capital budget returned by
	// SpotEngine.capitalForExchange(exchange), BEFORE any per-strategy or
	// cross-strategy cap enforcement.
	CapitalBudgetUSDT float64 `json:"capital_budget_usdt"`

	// MidPrice is the planning-time mid price snapshot from the futures
	// orderbook BBO (mirroring live ManualOpen) so planning and execution use
	// the same price source.
	MidPrice float64 `json:"mid_price"`

	// PlannedBaseSize is the final base-asset size AFTER Direction A
	// MaxBorrowable capping (when applicable) AND futures step rounding.
	PlannedBaseSize float64 `json:"planned_base_size"`

	// PlannedNotionalUSDT is PlannedBaseSize * MidPrice — the exact USDT value
	// the selector reserves and that scoring is computed against. This is the
	// authoritative exposure number for the batch reservation.
	PlannedNotionalUSDT float64 `json:"planned_notional_usdt"`

	// FuturesMarginUSDT is the margin required for the futures leg —
	// PlannedNotionalUSDT / cfg.SpotFuturesLeverage.
	FuturesMarginUSDT float64 `json:"futures_margin_usdt"`

	// MaxBorrowableBase is the borrow ceiling (in base units) applied for
	// Direction A on unified accounts. Zero when not applicable (Dir B, or
	// Dir A on separate accounts where the authoritative poll happens
	// post-transfer at execution time).
	MaxBorrowableBase float64 `json:"max_borrowable_base"`

	// RequiresInternalTransfer is true only for separate-account venues that
	// need a USDT transfer between futures and margin wallets at execution
	// time (Binance / Bitget in current live code). False for unified
	// accounts (Bybit UTA / OKX / Gate.io) where a single pool covers both
	// legs.
	RequiresInternalTransfer bool `json:"requires_internal_transfer"`

	// TransferTarget names the destination wallet for the internal transfer
	// on separate-account venues:
	//   "margin" for Direction A on separate (borrow on margin, sell spot)
	//   "spot"   for Direction B on separate (buy spot with USDT)
	//   ""       for unified accounts or when no transfer is required
	TransferTarget string `json:"transfer_target"`
}
