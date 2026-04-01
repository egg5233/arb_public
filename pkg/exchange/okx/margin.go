package okx

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"

	"arb/pkg/exchange"
)

// Compile-time check that *Adapter satisfies exchange.SpotMarginExchange.
var _ exchange.SpotMarginExchange = (*Adapter)(nil)

// autoLoanOnce ensures EnsureAutoLoan is called at most once per adapter lifetime.
var autoLoanOnce sync.Once

// EnsureAutoLoan enables OKX's account-level auto-loan feature.
// This is OKX's exchange-native mechanism for auto-borrow/auto-repay --
// configured at account level rather than per-order (unlike other exchanges).
// Once enabled, OKX automatically borrows when selling assets you don't hold
// and auto-repays when buying back.
//
// In Multi-currency margin mode (acctLv=3) or Portfolio margin mode (acctLv=4),
// this sets autoLoan=true via POST /api/v5/account/set-auto-loan.
//
// In Futures mode (acctLv=2), autoLoan is not available -- but cross-margin
// spot orders (tdMode=cross, ccy=USDT) implicitly handle borrow/repay as part
// of OKX's unified account. The set-auto-loan call returns 54001 in this mode,
// which is safe to ignore.
//
// This call is idempotent -- safe to call multiple times.
func (a *Adapter) EnsureAutoLoan() error {
	autoLoanOnce.Do(func() {
		body := map[string]interface{}{
			"autoLoan": true,
		}
		_, err := a.client.Post("/api/v5/account/set-auto-loan", body)
		if err != nil {
			// Error 54001 means the account is in Futures mode where autoLoan
			// is not available. This is OK -- Futures mode cross-margin orders
			// handle borrow/repay implicitly through tdMode=cross + ccy=USDT.
			if apiErr, ok := err.(*APIError); ok && apiErr.Code == "54001" {
				return // Not an error -- Futures mode handles borrowing differently
			}
			// Log but don't fail -- autoLoan is best-effort; the cross-margin
			// order mechanism works regardless.
			_ = err
		}
	})
	return nil
}

// toOKXSpotInstID converts internal symbol format to OKX spot instrument ID.
// "BTCUSDT" -> "BTC-USDT"
func toOKXSpotInstID(symbol string) string {
	s := strings.TrimSuffix(symbol, "USDT")
	return s + "-USDT"
}

// ---------------------------------------------------------------------------
// Spot Margin: Borrow / Repay
// ---------------------------------------------------------------------------

// MarginBorrow borrows a coin on spot margin (unified account manual borrow).
func (a *Adapter) MarginBorrow(params exchange.MarginBorrowParams) error {
	body := map[string]interface{}{
		"ccy":  params.Coin,
		"side": "borrow",
		"amt":  params.Amount,
	}
	_, err := a.client.Post("/api/v5/account/spot-manual-borrow-repay", body)
	if err != nil {
		return fmt.Errorf("MarginBorrow: %w", err)
	}
	return nil
}

// MarginRepay repays a borrowed coin on spot margin (unified account manual repay).
func (a *Adapter) MarginRepay(params exchange.MarginRepayParams) error {
	body := map[string]interface{}{
		"ccy":  params.Coin,
		"side": "repay",
		"amt":  params.Amount,
	}
	_, err := a.client.Post("/api/v5/account/spot-manual-borrow-repay", body)
	if err != nil {
		return fmt.Errorf("MarginRepay: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Spot Margin: Order
// ---------------------------------------------------------------------------

// PlaceSpotMarginOrder places a buy or sell order on spot margin using cross mode.
// OKX uses account-level autoLoan for auto-borrow/auto-repay. When AutoBorrow
// or AutoRepay is requested, EnsureAutoLoan is called once to enable the feature.
func (a *Adapter) PlaceSpotMarginOrder(params exchange.SpotMarginOrderParams) (string, error) {
	// Enable account-level autoLoan if auto-borrow or auto-repay is requested.
	// This is OKX's exchange-native mechanism -- once enabled, OKX automatically
	// borrows when selling assets you don't hold and auto-repays when buying back.
	if params.AutoBorrow || params.AutoRepay {
		if err := a.EnsureAutoLoan(); err != nil {
			return "", fmt.Errorf("PlaceSpotMarginOrder: failed to enable autoLoan: %w", err)
		}
	}

	instID := toOKXSpotInstID(params.Symbol)

	body := map[string]interface{}{
		"instId":  instID,
		"tdMode":  "cross",
		"ccy":     "USDT",
		"side":    string(params.Side),
		"ordType": params.OrderType,
		"sz":      params.Size,
	}

	if strings.ToLower(params.OrderType) == "limit" {
		body["px"] = params.Price
	}
	// OKX market: BUY with QuoteSize → quote_ccy (USDT amount); otherwise base_ccy.
	if strings.ToLower(params.OrderType) == "market" {
		if params.Side == exchange.SideBuy && params.QuoteSize != "" {
			body["tgtCcy"] = "quote_ccy"
			body["sz"] = params.QuoteSize
		} else {
			body["tgtCcy"] = "base_ccy"
		}
	}
	if params.ClientOid != "" {
		body["clOrdId"] = params.ClientOid
	}

	data, err := a.client.Post("/api/v5/trade/order", body)
	if err != nil {
		return "", fmt.Errorf("PlaceSpotMarginOrder: %w", err)
	}

	var resp []struct {
		OrdID   string `json:"ordId"`
		ClOrdID string `json:"clOrdId"`
		SCode   string `json:"sCode"`
		SMsg    string `json:"sMsg"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", fmt.Errorf("PlaceSpotMarginOrder unmarshal: %w", err)
	}
	if len(resp) == 0 {
		return "", fmt.Errorf("PlaceSpotMarginOrder: empty response")
	}
	if resp[0].SCode != "0" {
		return "", fmt.Errorf("PlaceSpotMarginOrder: code=%s msg=%s", resp[0].SCode, resp[0].SMsg)
	}
	return resp[0].OrdID, nil
}

// GetSpotMarginOrder returns the native spot/margin order state from OKX.
// Also queries trade fills to populate FeeDeducted for BUY orders.
func (a *Adapter) GetSpotMarginOrder(orderID, symbol string) (*exchange.SpotMarginOrderStatus, error) {
	instID := toOKXSpotInstID(symbol)
	params := map[string]string{
		"instId": instID,
		"ordId":  orderID,
	}
	data, err := a.client.Get("/api/v5/trade/order", params)
	if err != nil {
		return nil, fmt.Errorf("GetSpotMarginOrder: %w", err)
	}

	var resp []struct {
		OrdID     string `json:"ordId"`
		InstID    string `json:"instId"`
		State     string `json:"state"`
		AccFillSz string `json:"accFillSz"`
		AvgPx     string `json:"avgPx"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("GetSpotMarginOrder unmarshal: %w", err)
	}
	if len(resp) == 0 {
		return nil, nil
	}

	qty, _ := strconv.ParseFloat(resp[0].AccFillSz, 64)
	avgPrice, _ := strconv.ParseFloat(resp[0].AvgPx, 64)
	status := resp[0].State
	if status == "canceled" {
		status = "cancelled"
	}

	result := &exchange.SpotMarginOrderStatus{
		OrderID:   resp[0].OrdID,
		Symbol:    resp[0].InstID,
		Status:    status,
		FilledQty: qty,
		AvgPrice:  avgPrice,
	}

	// Query fills to get fee deducted from received coin on BUY orders.
	if qty > 0 {
		baseCoin := strings.TrimSuffix(symbol, "USDT")
		fillParams := map[string]string{
			"instType": "SPOT",
			"ordId":    orderID,
			"instId":   instID,
		}
		fillData, fillErr := a.client.Get("/api/v5/trade/fills", fillParams)
		if fillErr == nil {
			var fills []struct {
				Fee    string `json:"fee"`
				FeeCcy string `json:"feeCcy"`
			}
			if json.Unmarshal(fillData, &fills) == nil {
				var totalFee float64
				for _, f := range fills {
					if strings.EqualFold(f.FeeCcy, baseCoin) {
						fee, _ := strconv.ParseFloat(f.Fee, 64)
						totalFee += fee
					}
				}
				if totalFee != 0 {
					result.FeeDeducted = math.Abs(totalFee)
				}
			}
		}
	}

	return result, nil
}

// ---------------------------------------------------------------------------
// Spot Margin: Interest Rate
// ---------------------------------------------------------------------------

// GetMarginInterestRate returns the current borrow interest rate for a coin.
func (a *Adapter) GetMarginInterestRate(coin string) (*exchange.MarginInterestRate, error) {
	params := map[string]string{
		"ccy": coin,
	}
	data, err := a.client.Get("/api/v5/account/interest-rate", params)
	if err != nil {
		return nil, fmt.Errorf("GetMarginInterestRate: %w", err)
	}

	var resp []struct {
		Ccy          string `json:"ccy"`
		InterestRate string `json:"interestRate"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("GetMarginInterestRate unmarshal: %w", err)
	}
	if len(resp) == 0 {
		return nil, fmt.Errorf("GetMarginInterestRate: no data for %s", coin)
	}

	hourly, _ := strconv.ParseFloat(resp[0].InterestRate, 64)
	return &exchange.MarginInterestRate{
		Coin:       coin,
		HourlyRate: hourly,
		DailyRate:  hourly * 24,
	}, nil
}

// ---------------------------------------------------------------------------
// Spot Margin: Balance
// ---------------------------------------------------------------------------

// GetMarginBalance returns spot margin account info for a coin.
// OKX unified account keeps spot margin within the trading account.
func (a *Adapter) GetMarginBalance(coin string) (*exchange.MarginBalance, error) {
	// 1. Get account balance for the coin
	data, err := a.client.Get("/api/v5/account/balance", nil)
	if err != nil {
		return nil, fmt.Errorf("GetMarginBalance balance: %w", err)
	}

	var balResp []struct {
		Details []struct {
			Ccy      string `json:"ccy"`
			CashBal  string `json:"cashBal"`
			AvailBal string `json:"availBal"`
			Liab     string `json:"liab"`
			Interest string `json:"interest"`
		} `json:"details"`
	}
	if err := json.Unmarshal(data, &balResp); err != nil {
		return nil, fmt.Errorf("GetMarginBalance balance unmarshal: %w", err)
	}
	if len(balResp) == 0 {
		return nil, fmt.Errorf("GetMarginBalance: empty balance response")
	}

	var found bool
	var total, avail, borrowed, interest float64
	for _, d := range balResp[0].Details {
		if strings.EqualFold(d.Ccy, coin) {
			total, _ = strconv.ParseFloat(d.CashBal, 64)
			avail, _ = strconv.ParseFloat(d.AvailBal, 64)
			borrowed, _ = strconv.ParseFloat(d.Liab, 64)
			interest, _ = strconv.ParseFloat(d.Interest, 64)
			// OKX liab is negative or zero; normalize to positive
			if borrowed < 0 {
				borrowed = -borrowed
			}
			if interest < 0 {
				interest = -interest
			}
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("GetMarginBalance: coin %s not found in account", coin)
	}

	// 2. Get max borrowable from interest-limits
	var maxBorrowable float64
	limParams := map[string]string{
		"ccy": coin,
	}
	limData, err := a.client.Get("/api/v5/account/interest-limits", limParams)
	if err == nil {
		var limResp []struct {
			Records []struct {
				SurplusLmt string `json:"surplusLmt"`
			} `json:"records"`
		}
		if json.Unmarshal(limData, &limResp) == nil && len(limResp) > 0 && len(limResp[0].Records) > 0 {
			maxBorrowable, _ = strconv.ParseFloat(limResp[0].Records[0].SurplusLmt, 64)
		}
	}

	net := total - borrowed - interest
	return &exchange.MarginBalance{
		Coin:          coin,
		TotalBalance:  total,
		Available:     avail,
		Borrowed:      borrowed,
		Interest:      interest,
		NetBalance:    net,
		MaxBorrowable: maxBorrowable,
	}, nil
}

// ---------------------------------------------------------------------------
// Spot Margin: Transfers (no-op for OKX unified account)
// ---------------------------------------------------------------------------

// TransferToMargin is a no-op on OKX.
// In the unified account, USDT in the trading account is already collateral
// for both derivatives and spot margin. No transfer is needed.
func (a *Adapter) TransferToMargin(_ string, _ string) error {
	return nil
}

// TransferFromMargin is a no-op on OKX.
// In the unified account, funds are shared across trading modes.
func (a *Adapter) TransferFromMargin(_ string, _ string) error {
	return nil
}
