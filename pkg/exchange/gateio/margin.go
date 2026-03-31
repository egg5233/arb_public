package gateio

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"arb/pkg/exchange"
)

// Compile-time interface check.
var _ exchange.SpotMarginExchange = (*Adapter)(nil)

// ---------------------------------------------------------------------------
// Spot Margin: Borrow / Repay (Unified Account — /unified/*)
// ---------------------------------------------------------------------------

// MarginBorrow borrows a coin on Gate.io unified account (POST /unified/loans).
// No currency_pair needed — unified account uses cross margin.
func (a *Adapter) MarginBorrow(params exchange.MarginBorrowParams) error {
	body := map[string]string{
		"currency": params.Coin,
		"type":     "borrow",
		"amount":   params.Amount,
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("MarginBorrow marshal: %w", err)
	}
	_, err = a.client.Post("/unified/loans", string(bodyBytes))
	if err != nil {
		return fmt.Errorf("MarginBorrow: %w", err)
	}
	return nil
}

// MarginRepay repays a borrowed coin on Gate.io unified account (POST /unified/loans with type=repay).
func (a *Adapter) MarginRepay(params exchange.MarginRepayParams) error {
	body := map[string]interface{}{
		"currency": params.Coin,
		"type":     "repay",
		"amount":   params.Amount,
	}
	// If amount is "0" or empty, use repaid_all to repay everything.
	if params.Amount == "" || params.Amount == "0" {
		body["repaid_all"] = true
		body["amount"] = "0"
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("MarginRepay marshal: %w", err)
	}
	_, err = a.client.Post("/unified/loans", string(bodyBytes))
	if err != nil {
		return fmt.Errorf("MarginRepay: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Spot Margin: Place Order
// ---------------------------------------------------------------------------

// PlaceSpotMarginOrder places a buy or sell order on spot with unified margin (POST /spot/orders).
// Uses account="unified" for unified cross-margin orders.
func (a *Adapter) PlaceSpotMarginOrder(params exchange.SpotMarginOrderParams) (string, error) {
	pair := toGateSymbol(params.Symbol)

	tif := "gtc"
	if params.Force != "" {
		tif = strings.ToLower(params.Force)
	}

	orderReq := map[string]interface{}{
		"currency_pair": pair,
		"side":          string(params.Side),
		"type":          params.OrderType,
		"account":       "unified",
		"time_in_force": tif,
	}

	if params.OrderType == "limit" {
		orderReq["amount"] = params.Size
		orderReq["price"] = params.Price
	} else {
		orderReq["amount"] = params.Size
	}

	if params.AutoBorrow {
		orderReq["auto_borrow"] = true
	}
	if params.AutoRepay {
		orderReq["auto_repay"] = true
	}
	if params.ClientOid != "" {
		orderReq["text"] = "t-" + params.ClientOid
	}

	bodyBytes, err := json.Marshal(orderReq)
	if err != nil {
		return "", fmt.Errorf("PlaceSpotMarginOrder marshal: %w", err)
	}

	data, err := a.client.Post("/spot/orders", string(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("PlaceSpotMarginOrder: %w", err)
	}

	var resp struct {
		ID   string `json:"id"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", fmt.Errorf("PlaceSpotMarginOrder unmarshal: %w (body: %s)", err, string(data))
	}
	return resp.ID, nil
}

// GetSpotMarginOrder returns the native unified spot order state from Gate.io.
func (a *Adapter) GetSpotMarginOrder(orderID, symbol string) (*exchange.SpotMarginOrderStatus, error) {
	pair := toGateSymbol(symbol)
	data, err := a.client.Get("/spot/orders/"+orderID, map[string]string{
		"currency_pair": pair,
		"account":       "unified",
	})
	if err != nil {
		return nil, fmt.Errorf("GetSpotMarginOrder: %w", err)
	}

	var resp struct {
		ID           string `json:"id"`
		Status       string `json:"status"`
		FinishAs     string `json:"finish_as"`
		CurrencyPair string `json:"currency_pair"`
		FilledAmount string `json:"filled_amount"`
		AvgDealPrice string `json:"avg_deal_price"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("GetSpotMarginOrder unmarshal: %w", err)
	}

	qty, _ := strconv.ParseFloat(resp.FilledAmount, 64)
	avgPrice, _ := strconv.ParseFloat(resp.AvgDealPrice, 64)
	status := strings.ToLower(resp.Status)
	switch status {
	case "open":
		status = "live"
	case "closed":
		if strings.ToLower(resp.FinishAs) == "filled" {
			status = "filled"
		} else {
			status = "cancelled"
		}
	}

	return &exchange.SpotMarginOrderStatus{
		OrderID:   resp.ID,
		Symbol:    resp.CurrencyPair,
		Status:    status,
		FilledQty: qty,
		AvgPrice:  avgPrice,
	}, nil
}

// ---------------------------------------------------------------------------
// Spot Margin: Interest Rate
// ---------------------------------------------------------------------------

// GetMarginInterestRate returns the current borrow interest rate for a coin
// via GET /unified/estimate_rate.
func (a *Adapter) GetMarginInterestRate(coin string) (*exchange.MarginInterestRate, error) {
	params := map[string]string{
		"currencies": coin,
	}
	data, err := a.client.Get("/unified/estimate_rate", params)
	if err != nil {
		return nil, fmt.Errorf("GetMarginInterestRate: %w", err)
	}

	// Response is a key-value object: {"BTC": "0.000002", "GT": "0.000001"}
	var rateMap map[string]string
	if err := json.Unmarshal(data, &rateMap); err != nil {
		return nil, fmt.Errorf("GetMarginInterestRate unmarshal: %w", err)
	}

	rateStr, ok := rateMap[strings.ToUpper(coin)]
	if !ok || rateStr == "" {
		return nil, fmt.Errorf("GetMarginInterestRate: no rate returned for %s", coin)
	}
	rate, _ := strconv.ParseFloat(rateStr, 64)
	// Gate.io estimate_rate returns hourly rate.
	return &exchange.MarginInterestRate{
		Coin:       coin,
		HourlyRate: rate,
		DailyRate:  rate * 24,
	}, nil
}

// ---------------------------------------------------------------------------
// Spot Margin: Balance
// ---------------------------------------------------------------------------

// GetMarginBalance returns unified account margin info for a coin.
// Uses GET /unified/accounts for balances and GET /unified/borrowable for max borrowable.
func (a *Adapter) GetMarginBalance(coin string) (*exchange.MarginBalance, error) {
	// 1. Get unified account balances.
	acctParams := map[string]string{
		"currency": coin,
	}
	acctData, err := a.client.Get("/unified/accounts", acctParams)
	if err != nil {
		return nil, fmt.Errorf("GetMarginBalance accounts: %w", err)
	}

	var acctResp struct {
		Balances map[string]struct {
			Available    string `json:"available"`
			Freeze       string `json:"freeze"`
			Borrowed     string `json:"borrowed"`
			TotalLiab    string `json:"total_liab"`
			Equity       string `json:"equity"`
			NegativeLiab string `json:"negative_liab"`
		} `json:"balances"`
	}
	if err := json.Unmarshal(acctData, &acctResp); err != nil {
		return nil, fmt.Errorf("GetMarginBalance accounts unmarshal: %w", err)
	}

	var totalBalance, available, borrowed, interest float64
	upperCoin := strings.ToUpper(coin)
	if bal, ok := acctResp.Balances[upperCoin]; ok {
		available, _ = strconv.ParseFloat(bal.Available, 64)
		frozen, _ := strconv.ParseFloat(bal.Freeze, 64)
		totalBalance = available + frozen
		borrowed, _ = strconv.ParseFloat(bal.Borrowed, 64)
		// total_liab includes borrowed + interest; interest = total_liab - borrowed
		totalLiab, _ := strconv.ParseFloat(bal.TotalLiab, 64)
		interest = totalLiab - borrowed
		if interest < 0 {
			interest = 0
		}
	}

	// 2. Get max borrowable from GET /unified/borrowable.
	var maxBorrowable float64
	borrowParams := map[string]string{
		"currency": coin,
	}
	borrowData, err := a.client.Get("/unified/borrowable", borrowParams)
	if err == nil {
		var borrowResp struct {
			Currency string `json:"currency"`
			Amount   string `json:"amount"`
		}
		if json.Unmarshal(borrowData, &borrowResp) == nil {
			maxBorrowable, _ = strconv.ParseFloat(borrowResp.Amount, 64)
		}
	}

	return &exchange.MarginBalance{
		Coin:          coin,
		TotalBalance:  totalBalance,
		Available:     available,
		Borrowed:      borrowed,
		Interest:      interest,
		NetBalance:    totalBalance - borrowed - interest,
		MaxBorrowable: maxBorrowable,
	}, nil
}

// ---------------------------------------------------------------------------
// Spot Margin: Transfers (no-op for Unified Account)
// ---------------------------------------------------------------------------

// TransferToMargin is a no-op for Gate.io unified account.
// In unified mode, all assets are in a single account — no transfer needed.
func (a *Adapter) TransferToMargin(_ string, _ string) error {
	return nil
}

// TransferFromMargin is a no-op for Gate.io unified account.
func (a *Adapter) TransferFromMargin(_ string, _ string) error {
	return nil
}
