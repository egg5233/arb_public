package binance

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"arb/pkg/exchange"
)

// Compile-time check that *Adapter satisfies exchange.SpotMarginExchange.
var _ exchange.SpotMarginExchange = (*Adapter)(nil)

// ---------------------------------------------------------------------------
// Spot Margin: Borrow / Repay
// ---------------------------------------------------------------------------

// MarginBorrow borrows a coin on cross margin.
func (b *Adapter) MarginBorrow(params exchange.MarginBorrowParams) error {
	reqParams := map[string]string{
		"asset":      params.Coin,
		"amount":     params.Amount,
		"type":       "BORROW",
		"isIsolated": "FALSE",
	}
	_, err := b.client.SpotPost("/sapi/v1/margin/borrow-repay", reqParams)
	if err != nil {
		return fmt.Errorf("MarginBorrow: %w", err)
	}
	return nil
}

// MarginRepay repays a borrowed coin on cross margin.
func (b *Adapter) MarginRepay(params exchange.MarginRepayParams) error {
	reqParams := map[string]string{
		"asset":      params.Coin,
		"amount":     params.Amount,
		"type":       "REPAY",
		"isIsolated": "FALSE",
	}
	_, err := b.client.SpotPost("/sapi/v1/margin/borrow-repay", reqParams)
	if err != nil {
		return fmt.Errorf("MarginRepay: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Spot Margin: Order
// ---------------------------------------------------------------------------

// PlaceSpotMarginOrder places a buy or sell order on cross margin.
func (b *Adapter) PlaceSpotMarginOrder(params exchange.SpotMarginOrderParams) (string, error) {
	sideEffect := "NO_SIDE_EFFECT"
	if params.AutoBorrow {
		sideEffect = "AUTO_BORROW_REPAY"
	} else if params.AutoRepay {
		sideEffect = "AUTO_REPAY"
	}

	reqParams := map[string]string{
		"symbol":         params.Symbol,
		"side":           mapSide(params.Side),
		"type":           mapOrderType(params.OrderType),
		"quantity":       params.Size,
		"sideEffectType": sideEffect,
	}
	// Binance market BUY requires quoteOrderQty (USDT amount), not quantity (base coin).
	if strings.ToLower(params.OrderType) == "market" && params.Side == exchange.SideBuy && params.QuoteSize != "" {
		reqParams["quoteOrderQty"] = params.QuoteSize
		delete(reqParams, "quantity")
	}
	if strings.ToLower(params.OrderType) == "limit" {
		reqParams["price"] = params.Price
		reqParams["timeInForce"] = mapTimeInForce(params.Force)
	}
	if params.ClientOid != "" {
		reqParams["newClientOrderId"] = params.ClientOid
	}

	body, err := b.client.SpotPost("/sapi/v1/margin/order", reqParams)
	if err != nil {
		return "", fmt.Errorf("PlaceSpotMarginOrder: %w", err)
	}

	var resp struct {
		OrderID int64 `json:"orderId"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("PlaceSpotMarginOrder unmarshal: %w", err)
	}
	return strconv.FormatInt(resp.OrderID, 10), nil
}

// GetSpotMarginOrder returns the native margin order state for a spot margin order.
func (b *Adapter) GetSpotMarginOrder(orderID, symbol string) (*exchange.SpotMarginOrderStatus, error) {
	reqParams := map[string]string{
		"symbol":     symbol,
		"isIsolated": "FALSE",
		"orderId":    orderID,
	}
	body, err := b.client.SpotGet("/sapi/v1/margin/order", reqParams)
	if err != nil {
		return nil, fmt.Errorf("GetSpotMarginOrder: %w", err)
	}

	var resp struct {
		OrderID            int64  `json:"orderId"`
		Symbol             string `json:"symbol"`
		Status             string `json:"status"`
		ExecutedQty        string `json:"executedQty"`
		CumulativeQuoteQty string `json:"cummulativeQuoteQty"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("GetSpotMarginOrder unmarshal: %w", err)
	}

	qty, _ := strconv.ParseFloat(resp.ExecutedQty, 64)
	quoteQty, _ := strconv.ParseFloat(resp.CumulativeQuoteQty, 64)
	avgPrice := 0.0
	if qty > 0 && quoteQty > 0 {
		avgPrice = quoteQty / qty
	}

	status := strings.ToLower(resp.Status)
	if status == "canceled" {
		status = "cancelled"
	}

	return &exchange.SpotMarginOrderStatus{
		OrderID:   strconv.FormatInt(resp.OrderID, 10),
		Symbol:    resp.Symbol,
		Status:    status,
		FilledQty: qty,
		AvgPrice:  avgPrice,
	}, nil
}

// ---------------------------------------------------------------------------
// Spot Margin: Interest Rate
// ---------------------------------------------------------------------------

// GetMarginInterestRate returns the current borrow interest rate for a coin.
func (b *Adapter) GetMarginInterestRate(coin string) (*exchange.MarginInterestRate, error) {
	params := map[string]string{
		"assets":     coin,
		"isIsolated": "FALSE",
	}
	body, err := b.client.SpotGet("/sapi/v1/margin/next-hourly-interest-rate", params)
	if err != nil {
		return nil, fmt.Errorf("GetMarginInterestRate: %w", err)
	}

	var resp []struct {
		Asset                  string `json:"asset"`
		NextHourlyInterestRate string `json:"nextHourlyInterestRate"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("GetMarginInterestRate unmarshal: %w", err)
	}

	for _, r := range resp {
		if strings.EqualFold(r.Asset, coin) {
			hourly, _ := strconv.ParseFloat(r.NextHourlyInterestRate, 64)
			return &exchange.MarginInterestRate{
				Coin:       coin,
				HourlyRate: hourly,
				DailyRate:  hourly * 24,
			}, nil
		}
	}
	return nil, fmt.Errorf("GetMarginInterestRate: asset %s not found in response", coin)
}

// ---------------------------------------------------------------------------
// Spot Margin: Balance
// ---------------------------------------------------------------------------

// GetMarginBalance returns spot margin account info for a coin.
func (b *Adapter) GetMarginBalance(coin string) (*exchange.MarginBalance, error) {
	// Fetch max borrowable amount
	borrowParams := map[string]string{
		"asset": coin,
	}
	borrowBody, err := b.client.SpotGet("/sapi/v1/margin/maxBorrowable", borrowParams)
	if err != nil {
		return nil, fmt.Errorf("GetMarginBalance maxBorrowable: %w", err)
	}

	var borrowResp struct {
		Amount string `json:"amount"`
	}
	if err := json.Unmarshal(borrowBody, &borrowResp); err != nil {
		return nil, fmt.Errorf("GetMarginBalance maxBorrowable unmarshal: %w", err)
	}
	maxBorrowable, _ := strconv.ParseFloat(borrowResp.Amount, 64)

	// Fetch margin account to get asset details
	acctBody, err := b.client.SpotGet("/sapi/v1/margin/account", nil)
	if err != nil {
		return nil, fmt.Errorf("GetMarginBalance account: %w", err)
	}

	var acctResp struct {
		UserAssets []struct {
			Asset    string `json:"asset"`
			Free     string `json:"free"`
			Locked   string `json:"locked"`
			Borrowed string `json:"borrowed"`
			Interest string `json:"interest"`
			NetAsset string `json:"netAsset"`
		} `json:"userAssets"`
	}
	if err := json.Unmarshal(acctBody, &acctResp); err != nil {
		return nil, fmt.Errorf("GetMarginBalance account unmarshal: %w", err)
	}

	for _, a := range acctResp.UserAssets {
		if strings.EqualFold(a.Asset, coin) {
			free, _ := strconv.ParseFloat(a.Free, 64)
			locked, _ := strconv.ParseFloat(a.Locked, 64)
			borrowed, _ := strconv.ParseFloat(a.Borrowed, 64)
			interest, _ := strconv.ParseFloat(a.Interest, 64)
			netAsset, _ := strconv.ParseFloat(a.NetAsset, 64)
			return &exchange.MarginBalance{
				Coin:          coin,
				TotalBalance:  free + locked,
				Available:     free,
				Borrowed:      borrowed,
				Interest:      interest,
				NetBalance:    netAsset,
				MaxBorrowable: maxBorrowable,
			}, nil
		}
	}
	return nil, fmt.Errorf("GetMarginBalance: asset %s not found in margin account", coin)
}

// ---------------------------------------------------------------------------
// Spot Margin: Transfers
// ---------------------------------------------------------------------------

// TransferToMargin moves funds from the futures (USDT-M) account to the cross margin account.
func (b *Adapter) TransferToMargin(coin string, amount string) error {
	params := map[string]string{
		"asset":  coin,
		"amount": amount,
		"type":   "UMFUTURE_MARGIN",
	}
	_, err := b.client.SpotPost("/sapi/v1/asset/transfer", params)
	if err != nil {
		return fmt.Errorf("TransferToMargin: %w", err)
	}
	return nil
}

// TransferFromMargin moves funds from the cross margin account back to the futures (USDT-M) account.
func (b *Adapter) TransferFromMargin(coin string, amount string) error {
	params := map[string]string{
		"asset":  coin,
		"amount": amount,
		"type":   "MARGIN_UMFUTURE",
	}
	_, err := b.client.SpotPost("/sapi/v1/asset/transfer", params)
	if err != nil {
		return fmt.Errorf("TransferFromMargin: %w", err)
	}
	return nil
}
