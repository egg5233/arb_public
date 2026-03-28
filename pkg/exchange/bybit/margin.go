package bybit

import (
	"encoding/json"
	"fmt"
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
	reqParams := map[string]string{
		"coin":   params.Coin,
		"amount": params.Amount,
	}
	_, err := a.client.Post("/v5/account/borrow", reqParams)
	if err != nil {
		return fmt.Errorf("bybit MarginBorrow: %w", err)
	}
	return nil
}

// MarginRepay repays a borrowed coin without asset conversion.
// Bybit blocks repayment during minutes 04:00–05:30 of each hour UTC.
func (a *Adapter) MarginRepay(params exchange.MarginRepayParams) error {
	now := time.Now().UTC()
	min := now.Minute()
	sec := now.Second()
	totalSec := min*60 + sec
	// Blackout: 04:00 (240s) through 05:30 (330s) of each hour.
	if totalSec >= 4*60 && totalSec < 5*60+30 {
		return fmt.Errorf("bybit MarginRepay: repayment blackout (minute %02d:%02d UTC, retry after %02d:30)",
			min, sec, 5)
	}

	reqParams := map[string]string{
		"coin":   params.Coin,
		"amount": params.Amount,
	}
	_, err := a.client.Post("/v5/account/no-convert-repay", reqParams)
	if err != nil {
		return fmt.Errorf("bybit MarginRepay: %w", err)
	}
	return nil
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

// ---------------------------------------------------------------------------
// Spot Margin: Interest Rate
// ---------------------------------------------------------------------------

// GetMarginInterestRate returns the current borrow interest rate for a coin
// from the collateral-info endpoint.
func (a *Adapter) GetMarginInterestRate(coin string) (*exchange.MarginInterestRate, error) {
	params := map[string]string{
		"currency": coin,
	}
	result, err := a.client.Get("/v5/account/collateral-info", params)
	if err != nil {
		return nil, fmt.Errorf("bybit GetMarginInterestRate: %w", err)
	}

	var resp struct {
		List []struct {
			Currency         string `json:"currency"`
			HourlyBorrowRate string `json:"hourlyBorrowRate"`
		} `json:"list"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return nil, fmt.Errorf("bybit GetMarginInterestRate parse: %w", err)
	}

	for _, item := range resp.List {
		if strings.EqualFold(item.Currency, coin) {
			hourly, _ := strconv.ParseFloat(item.HourlyBorrowRate, 64)
			return &exchange.MarginInterestRate{
				Coin:       coin,
				HourlyRate: hourly,
				DailyRate:  hourly * 24,
			}, nil
		}
	}
	return nil, fmt.Errorf("bybit GetMarginInterestRate: currency %s not found in response", coin)
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
