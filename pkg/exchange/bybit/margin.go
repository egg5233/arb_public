package bybit

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"arb/pkg/exchange"
)

// Compile-time check that *Adapter satisfies exchange.SpotMarginExchange.
var _ exchange.SpotMarginExchange = (*Adapter)(nil)

// ---------------------------------------------------------------------------
// Spot Margin: Borrow / Repay
// ---------------------------------------------------------------------------

// MarginBorrow borrows a coin on spot margin via the unified account.
func (a *Adapter) MarginBorrow(params exchange.MarginBorrowParams) error {
	// Bybit requires borrow amounts as whole numbers for most coins.
	// Floor to integer to avoid "precision must be an integer multiple" error.
	amt, _ := strconv.ParseFloat(params.Amount, 64)
	if amt > 0 {
		amt = math.Floor(amt)
	}
	if amt <= 0 {
		return fmt.Errorf("bybit MarginBorrow: amount %s rounds to 0", params.Amount)
	}
	reqParams := map[string]string{
		"coin":   params.Coin,
		"amount": strconv.FormatFloat(amt, 'f', 0, 64),
	}
	_, err := a.client.Post("/v5/account/borrow", reqParams)
	if err != nil {
		return fmt.Errorf("bybit MarginBorrow: %w", err)
	}
	return nil
}

// MarginRepay repays a borrowed coin via no-convert-repay (safe, won't touch other assets).
// Omits the amount parameter — Bybit repays min(spot available, liability). When a specific
// amount exceeds the "lossLessRepaymentAmount" (spot available balance), Bybit rejects it
// with "remaining quota insufficient." Omitting amount lets Bybit repay what it can; the
// monitor retries until borrowed reaches 0.
// Bybit blocks repayment during minutes 04:00–05:30 of each hour UTC.
func (a *Adapter) MarginRepay(params exchange.MarginRepayParams) error {
	now := time.Now().UTC()
	min := now.Minute()
	sec := now.Second()
	totalSec := min*60 + sec
	// Blackout: 04:00 (240s) through 05:30 (330s) of each hour.
	if totalSec >= 4*60 && totalSec < 5*60+30 {
		retryAfter := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 5, 31, 0, time.UTC)
		return &exchange.ErrRepayBlackout{
			RetryAfter: retryAfter,
			Message:    fmt.Sprintf("bybit MarginRepay: repayment blackout (minute %02d:%02d UTC, retry after %02d:05:31)", min, sec, now.Hour()),
		}
	}

	// Omit amount — let Bybit repay whatever spot balance is available.
	// This avoids "remaining quota insufficient" when the requested amount
	// exceeds the lossLessRepaymentAmount (spot available after UTA settlement).
	reqParams := map[string]string{
		"coin": params.Coin,
	}
	result, err := a.client.Post("/v5/account/no-convert-repay", reqParams)
	if err != nil {
		return fmt.Errorf("bybit MarginRepay: %w", err)
	}

	// Bybit returns retCode=0 even when repay is still processing or failed.
	// Must check resultStatus to detect non-success outcomes.
	var repayResp struct {
		ResultStatus string `json:"resultStatus"`
	}
	if err := json.Unmarshal(result, &repayResp); err != nil {
		return fmt.Errorf("bybit MarginRepay: unmarshal result: %w", err)
	}
	switch repayResp.ResultStatus {
	case "SU":
		return nil // success
	case "FA":
		return fmt.Errorf("bybit MarginRepay: repay failed (resultStatus=FA)")
	case "P":
		return fmt.Errorf("bybit MarginRepay: repay still processing (resultStatus=P), will retry")
	default:
		return fmt.Errorf("bybit MarginRepay: unknown resultStatus=%q, treating as pending", repayResp.ResultStatus)
	}
}

// ---------------------------------------------------------------------------
// Spot Margin: Order
// ---------------------------------------------------------------------------

// PlaceSpotMarginOrder places a buy or sell order on spot with optional margin leverage.
func (a *Adapter) PlaceSpotMarginOrder(params exchange.SpotMarginOrderParams) (string, error) {
	reqParams := map[string]string{
		"category":    "spot",
		"symbol":      params.Symbol,
		"side":        toBybitSide(params.Side),
		"orderType":   toBybitOrderType(params.OrderType),
		"qty":         params.Size,
		"timeInForce": toBybitTIF(params.Force),
	}
	// Bybit spot market orders: timeInForce is invalid; market BUY defaults
	// qty to quote currency, so specify marketUnit or swap to QuoteSize.
	if strings.ToLower(params.OrderType) == "market" {
		delete(reqParams, "timeInForce") // invalid for market orders on Bybit
		if params.Side == exchange.SideBuy && params.QuoteSize != "" {
			reqParams["qty"] = params.QuoteSize
		} else if params.Side == exchange.SideBuy {
			reqParams["marketUnit"] = "baseCoin"
		}
	}
	if strings.ToLower(params.OrderType) == "limit" && params.Price != "" {
		reqParams["price"] = params.Price
	}
	if params.AutoBorrow || params.AutoRepay {
		reqParams["isLeverage"] = "1"
	}
	if params.ClientOid != "" {
		reqParams["orderLinkId"] = params.ClientOid
	}

	result, err := a.client.Post("/v5/order/create", reqParams)
	if err != nil {
		return "", fmt.Errorf("bybit PlaceSpotMarginOrder: %w", err)
	}

	var resp struct {
		OrderID     string `json:"orderId"`
		OrderLinkID string `json:"orderLinkId"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return "", fmt.Errorf("bybit PlaceSpotMarginOrder parse: %w", err)
	}
	return resp.OrderID, nil
}

// GetSpotMarginOrder returns the native spot order state from Bybit UTA.
func (a *Adapter) GetSpotMarginOrder(orderID, symbol string) (*exchange.SpotMarginOrderStatus, error) {
	params := map[string]string{
		"category": "spot",
		"symbol":   symbol,
		"orderId":  orderID,
	}
	status, err := a.getSpotMarginOrderFromPath("/v5/order/realtime", params)
	if err != nil {
		return nil, fmt.Errorf("bybit GetSpotMarginOrder: %w", err)
	}
	if status != nil {
		return status, nil
	}

	// Filled/cancelled unified-account orders may disappear from realtime after
	// Bybit service restarts; fall back to order history for reconciliation.
	status, err = a.getSpotMarginOrderFromPath("/v5/order/history", params)
	if err != nil {
		return nil, fmt.Errorf("bybit GetSpotMarginOrder history: %w", err)
	}
	return status, nil
}

func (a *Adapter) getSpotMarginOrderFromPath(path string, params map[string]string) (*exchange.SpotMarginOrderStatus, error) {
	result, err := a.client.Get(path, params)
	if err != nil {
		return nil, err
	}
	var resp struct {
		List []struct {
			OrderID     string `json:"orderId"`
			Symbol      string `json:"symbol"`
			OrderStatus string `json:"orderStatus"`
			CumExecQty  string `json:"cumExecQty"`
			AvgPrice    string `json:"avgPrice"`
		} `json:"list"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if len(resp.List) == 0 {
		return nil, nil
	}

	qty, _ := strconv.ParseFloat(resp.List[0].CumExecQty, 64)
	avgPrice, _ := strconv.ParseFloat(resp.List[0].AvgPrice, 64)
	status := strings.ToLower(resp.List[0].OrderStatus)
	switch status {
	case "new":
		status = "live"
	case "partiallyfilled":
		status = "partially_filled"
	case "partiallyfilledcanceled":
		status = "cancelled"
	}

	return &exchange.SpotMarginOrderStatus{
		OrderID:   resp.List[0].OrderID,
		Symbol:    resp.List[0].Symbol,
		Status:    status,
		FilledQty: qty,
		AvgPrice:  avgPrice,
	}, nil
}

// ---------------------------------------------------------------------------
// Spot Margin: Interest Rate
// ---------------------------------------------------------------------------

// GetMarginInterestRate returns the current borrow interest rate for a coin
// via the crypto-loan loanable-data endpoint (public, no auth needed).
// Returns the actual flexible annualized rate, not the free-quota-adjusted rate.
func (a *Adapter) GetMarginInterestRate(coin string) (*exchange.MarginInterestRate, error) {
	params := map[string]string{
		"currency": coin,
	}
	result, err := a.client.Get("/v5/crypto-loan-common/loanable-data", params)
	if err != nil {
		return nil, fmt.Errorf("bybit GetMarginInterestRate: %w", err)
	}

	var resp struct {
		List []struct {
			Currency                       string `json:"currency"`
			FlexibleAnnualizedInterestRate string `json:"flexibleAnnualizedInterestRate"`
			FlexibleBorrowable             bool   `json:"flexibleBorrowable"`
		} `json:"list"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return nil, fmt.Errorf("bybit GetMarginInterestRate parse: %w", err)
	}

	for _, item := range resp.List {
		if strings.EqualFold(item.Currency, coin) {
			if !item.FlexibleBorrowable {
				return nil, fmt.Errorf("bybit GetMarginInterestRate: %s does not support flexible borrowing", coin)
			}
			annualRate, _ := strconv.ParseFloat(item.FlexibleAnnualizedInterestRate, 64)
			hourly := annualRate / 8760 // annualized → hourly
			return &exchange.MarginInterestRate{
				Coin:       coin,
				HourlyRate: hourly,
				DailyRate:  annualRate / 365,
			}, nil
		}
	}
	return nil, fmt.Errorf("bybit GetMarginInterestRate: currency %s not found", coin)
}

// ---------------------------------------------------------------------------
// Spot Margin: Balance
// ---------------------------------------------------------------------------

// GetMarginBalance returns spot margin account info for a coin.
// In Bybit UTA the unified wallet holds both spot and derivatives collateral.
func (a *Adapter) GetMarginBalance(coin string) (*exchange.MarginBalance, error) {
	// 1. Wallet balance for the coin.
	walletParams := map[string]string{
		"accountType": "UNIFIED",
		"coin":        coin,
	}
	walletResult, err := a.client.Get("/v5/account/wallet-balance", walletParams)
	if err != nil {
		return nil, fmt.Errorf("bybit GetMarginBalance wallet: %w", err)
	}

	var walletResp struct {
		List []struct {
			Coin []struct {
				Coin            string `json:"coin"`
				WalletBalance   string `json:"walletBalance"`
				Equity          string `json:"equity"`
				Locked          string `json:"locked"`
				BorrowAmount    string `json:"borrowAmount"`
				AccruedInterest string `json:"accruedInterest"`
			} `json:"coin"`
		} `json:"list"`
	}
	if err := json.Unmarshal(walletResult, &walletResp); err != nil {
		return nil, fmt.Errorf("bybit GetMarginBalance wallet parse: %w", err)
	}

	if len(walletResp.List) == 0 {
		return nil, fmt.Errorf("bybit GetMarginBalance: no account data returned")
	}

	// Find the coin entry.
	var walletBalance, available, borrowed, interest float64
	found := false
	for _, c := range walletResp.List[0].Coin {
		if strings.EqualFold(c.Coin, coin) {
			walletBalance, _ = strconv.ParseFloat(c.WalletBalance, 64)
			equity, _ := strconv.ParseFloat(c.Equity, 64)
			locked, _ := strconv.ParseFloat(c.Locked, 64)
			available = equity - locked
			borrowed, _ = strconv.ParseFloat(c.BorrowAmount, 64)
			interest, _ = strconv.ParseFloat(c.AccruedInterest, 64)
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("bybit GetMarginBalance: coin %s not found in wallet", coin)
	}

	// 2. Max borrowable amount.
	borrowParams := map[string]string{
		"currency": coin,
	}
	var maxBorrowable float64
	borrowResult, borrowErr := a.client.Get("/v5/spot-margin-trade/max-borrowable", borrowParams)
	if borrowErr == nil {
		var borrowResp struct {
			MaxLoan  string `json:"maxLoan"`
			Currency string `json:"currency"`
		}
		if json.Unmarshal(borrowResult, &borrowResp) == nil {
			maxBorrowable, _ = strconv.ParseFloat(borrowResp.MaxLoan, 64)
		}
	}

	return &exchange.MarginBalance{
		Coin:          coin,
		TotalBalance:  walletBalance,
		Available:     available,
		Borrowed:      borrowed,
		Interest:      interest,
		NetBalance:    walletBalance - borrowed - interest,
		MaxBorrowable: maxBorrowable,
	}, nil
}

// ---------------------------------------------------------------------------
// Spot Margin: Transfers (no-ops for Bybit UTA)
// ---------------------------------------------------------------------------

// TransferToMargin is a no-op for Bybit UTA — the unified account IS the margin account.
func (a *Adapter) TransferToMargin(coin string, amount string) error { return nil }

// TransferFromMargin is a no-op for Bybit UTA — the unified account IS the margin account.
func (a *Adapter) TransferFromMargin(coin string, amount string) error { return nil }
