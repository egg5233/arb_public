package bitget

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"arb/pkg/exchange"
)

// Compile-time interface check.
var _ exchange.SpotMarginExchange = (*Adapter)(nil)

// MarginBorrow borrows a coin on cross spot margin.
func (a *Adapter) MarginBorrow(params exchange.MarginBorrowParams) error {
	body := map[string]string{
		"coin":         params.Coin,
		"borrowAmount": params.Amount,
	}

	raw, err := a.client.Post("/api/v2/margin/crossed/account/borrow", body)
	if err != nil {
		return fmt.Errorf("MarginBorrow POST error: %w", err)
	}

	var resp struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return fmt.Errorf("MarginBorrow unmarshal: %w", err)
	}
	if resp.Code != "00000" {
		return fmt.Errorf("MarginBorrow failed: code=%s msg=%s", resp.Code, resp.Msg)
	}
	return nil
}

// MarginRepay repays a borrowed coin on cross spot margin.
func (a *Adapter) MarginRepay(params exchange.MarginRepayParams) error {
	body := map[string]string{
		"coin":        params.Coin,
		"repayAmount": params.Amount,
	}

	raw, err := a.client.Post("/api/v2/margin/crossed/account/repay", body)
	if err != nil {
		return fmt.Errorf("MarginRepay POST error: %w", err)
	}

	var resp struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return fmt.Errorf("MarginRepay unmarshal: %w", err)
	}
	if resp.Code != "00000" {
		return fmt.Errorf("MarginRepay failed: code=%s msg=%s", resp.Code, resp.Msg)
	}
	return nil
}

// PlaceSpotMarginOrder places a buy or sell order on cross spot margin.
func (a *Adapter) PlaceSpotMarginOrder(params exchange.SpotMarginOrderParams) (string, error) {
	// Determine loanType based on AutoBorrow / AutoRepay flags.
	loanType := "normal"
	if params.AutoBorrow && params.AutoRepay {
		loanType = "autoLoanAndRepay"
	} else if params.AutoBorrow {
		loanType = "autoLoan"
	} else if params.AutoRepay {
		loanType = "autoRepay"
	}

	body := map[string]string{
		"symbol":    params.Symbol,
		"side":      string(params.Side),
		"orderType": params.OrderType,
		"loanType":  loanType,
	}
	// Bitget margin: market BUY requires quoteSize (USDT amount),
	// limit + market SELL require baseSize (coin quantity).
	// force is invalid for market orders per Bitget docs.
	if params.OrderType == "market" && params.Side == exchange.SideBuy {
		body["quoteSize"] = params.QuoteSize
	} else {
		body["baseSize"] = params.Size
	}
	if params.OrderType != "market" && params.Force != "" {
		body["force"] = params.Force
	}
	if params.Price != "" {
		body["price"] = params.Price
	}
	if params.ClientOid != "" {
		body["clientOid"] = params.ClientOid
	}

	raw, err := a.client.Post("/api/v2/margin/crossed/place-order", body)
	if err != nil {
		return "", fmt.Errorf("PlaceSpotMarginOrder POST error: %w", err)
	}

	var resp struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			OrderId   string `json:"orderId"`
			ClientOid string `json:"clientOid"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return "", fmt.Errorf("PlaceSpotMarginOrder unmarshal: %w", err)
	}
	if resp.Code != "00000" {
		return "", fmt.Errorf("PlaceSpotMarginOrder failed: code=%s msg=%s", resp.Code, resp.Msg)
	}
	return resp.Data.OrderId, nil
}

// GetSpotMarginOrder returns the native cross-margin order state from Bitget.
func (a *Adapter) GetSpotMarginOrder(orderID, symbol string) (*exchange.SpotMarginOrderStatus, error) {
	startTime := strconv.FormatInt(time.Now().Add(-24*time.Hour).UnixMilli(), 10)
	order, err := a.getSpotMarginOrder("/api/v2/margin/crossed/open-orders", orderID, symbol, startTime)
	if err != nil {
		return nil, err
	}
	if order != nil {
		return order, nil
	}
	return a.getSpotMarginOrder("/api/v2/margin/crossed/history-orders", orderID, symbol, startTime)
}

func (a *Adapter) getSpotMarginOrder(path, orderID, symbol, startTime string) (*exchange.SpotMarginOrderStatus, error) {
	raw, err := a.client.Get(path, map[string]string{
		"symbol":    symbol,
		"orderId":   orderID,
		"startTime": startTime,
		"limit":     "100",
	})
	if err != nil {
		return nil, fmt.Errorf("GetSpotMarginOrder GET error: %w", err)
	}

	var resp struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			OrderList []struct {
				OrderID  string `json:"orderId"`
				Symbol   string `json:"symbol"`
				Status   string `json:"status"`
				Size     string `json:"size"`
				PriceAvg string `json:"priceAvg"`
			} `json:"orderList"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return nil, fmt.Errorf("GetSpotMarginOrder unmarshal: %w", err)
	}
	if resp.Code != "00000" {
		return nil, fmt.Errorf("GetSpotMarginOrder failed: code=%s msg=%s", resp.Code, resp.Msg)
	}
	if len(resp.Data.OrderList) == 0 {
		return nil, nil
	}

	qty, _ := strconv.ParseFloat(resp.Data.OrderList[0].Size, 64)
	avgPrice, _ := strconv.ParseFloat(resp.Data.OrderList[0].PriceAvg, 64)
	status := resp.Data.OrderList[0].Status
	switch status {
	case "partial_fill", "partially_fill":
		status = "partially_filled"
	case "reject":
		status = "rejected"
	}

	return &exchange.SpotMarginOrderStatus{
		OrderID:   resp.Data.OrderList[0].OrderID,
		Symbol:    resp.Data.OrderList[0].Symbol,
		Status:    status,
		FilledQty: qty,
		AvgPrice:  avgPrice,
	}, nil
}

// GetMarginInterestRate returns the current borrow interest rate for a coin.
func (a *Adapter) GetMarginInterestRate(coin string) (*exchange.MarginInterestRate, error) {
	raw, err := a.client.Get("/api/v2/margin/crossed/interest-rate-and-limit", map[string]string{
		"coin": coin,
	})
	if err != nil {
		return nil, fmt.Errorf("GetMarginInterestRate GET error: %w", err)
	}

	var resp struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
		Data []struct {
			Coin              string `json:"coin"`
			DailyInterestRate string `json:"dailyInterestRate"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return nil, fmt.Errorf("GetMarginInterestRate unmarshal: %w", err)
	}
	if resp.Code != "00000" {
		return nil, fmt.Errorf("GetMarginInterestRate failed: code=%s msg=%s", resp.Code, resp.Msg)
	}
	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("GetMarginInterestRate: no data returned for coin %s", coin)
	}

	// dailyInterestRate is a decimal fraction (e.g. "0.0005" = 0.05% daily).
	dailyRate, err := strconv.ParseFloat(resp.Data[0].DailyInterestRate, 64)
	if err != nil {
		return nil, fmt.Errorf("GetMarginInterestRate parse dailyInterestRate: %w", err)
	}
	hourlyRate := dailyRate / 24.0

	return &exchange.MarginInterestRate{
		Coin:       coin,
		HourlyRate: hourlyRate,
		DailyRate:  dailyRate,
	}, nil
}

// GetMarginBalance returns cross margin account info for a coin.
func (a *Adapter) GetMarginBalance(coin string) (*exchange.MarginBalance, error) {
	raw, err := a.client.Get("/api/v2/margin/crossed/account/assets", map[string]string{
		"coin": coin,
	})
	if err != nil {
		return nil, fmt.Errorf("GetMarginBalance GET error: %w", err)
	}

	var resp struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
		Data []struct {
			Coin        string `json:"coin"`
			TotalAmount string `json:"totalAmount"`
			Available   string `json:"available"`
			Borrow      string `json:"borrow"`
			Interest    string `json:"interest"`
			Net         string `json:"net"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return nil, fmt.Errorf("GetMarginBalance unmarshal: %w", err)
	}
	if resp.Code != "00000" {
		return nil, fmt.Errorf("GetMarginBalance failed: code=%s msg=%s", resp.Code, resp.Msg)
	}

	// Find the matching coin entry.
	var found bool
	var entry struct {
		TotalAmount string
		Available   string
		Borrow      string
		Interest    string
		Net         string
	}
	for _, d := range resp.Data {
		if d.Coin == coin {
			entry.TotalAmount = d.TotalAmount
			entry.Available = d.Available
			entry.Borrow = d.Borrow
			entry.Interest = d.Interest
			entry.Net = d.Net
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("GetMarginBalance: no data for coin %s", coin)
	}

	totalBalance, _ := strconv.ParseFloat(entry.TotalAmount, 64)
	available, _ := strconv.ParseFloat(entry.Available, 64)
	borrowed, _ := strconv.ParseFloat(entry.Borrow, 64)
	interest, _ := strconv.ParseFloat(entry.Interest, 64)
	netBalance, _ := strconv.ParseFloat(entry.Net, 64)

	// Fetch max borrowable amount.
	maxBorrowable, err := a.fetchMaxBorrowable(coin)
	if err != nil {
		return nil, err
	}

	return &exchange.MarginBalance{
		Coin:          coin,
		TotalBalance:  totalBalance,
		Available:     available,
		Borrowed:      borrowed,
		Interest:      interest,
		NetBalance:    netBalance,
		MaxBorrowable: maxBorrowable,
	}, nil
}

// fetchMaxBorrowable retrieves the maximum additional borrowable amount for a coin.
func (a *Adapter) fetchMaxBorrowable(coin string) (float64, error) {
	raw, err := a.client.Get("/api/v2/margin/crossed/account/max-borrowable-amount", map[string]string{
		"coin": coin,
	})
	if err != nil {
		return 0, fmt.Errorf("fetchMaxBorrowable GET error: %w", err)
	}

	var resp struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			MaxBorrowableAmount string `json:"maxBorrowableAmount"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return 0, fmt.Errorf("fetchMaxBorrowable unmarshal: %w", err)
	}
	if resp.Code != "00000" {
		return 0, fmt.Errorf("fetchMaxBorrowable failed: code=%s msg=%s", resp.Code, resp.Msg)
	}

	maxBorrowable, err := strconv.ParseFloat(resp.Data.MaxBorrowableAmount, 64)
	if err != nil {
		return 0, fmt.Errorf("fetchMaxBorrowable parse amount: %w", err)
	}
	return maxBorrowable, nil
}

// TransferToMargin moves funds from the USDT futures account to the cross margin account.
func (a *Adapter) TransferToMargin(coin string, amount string) error {
	body := map[string]string{
		"fromType": "usdt_futures",
		"toType":   "crossed_margin",
		"coin":     coin,
		"amount":   amount,
	}

	raw, err := a.client.Post("/api/v2/spot/wallet/transfer", body)
	if err != nil {
		return fmt.Errorf("TransferToMargin POST error: %w", err)
	}

	var resp struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return fmt.Errorf("TransferToMargin unmarshal: %w", err)
	}
	if resp.Code != "00000" {
		return fmt.Errorf("TransferToMargin failed: code=%s msg=%s", resp.Code, resp.Msg)
	}
	return nil
}

// TransferFromMargin moves funds from the cross margin account back to USDT futures.
func (a *Adapter) TransferFromMargin(coin string, amount string) error {
	body := map[string]string{
		"fromType": "crossed_margin",
		"toType":   "usdt_futures",
		"coin":     coin,
		"amount":   amount,
	}

	raw, err := a.client.Post("/api/v2/spot/wallet/transfer", body)
	if err != nil {
		return fmt.Errorf("TransferFromMargin POST error: %w", err)
	}

	var resp struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return fmt.Errorf("TransferFromMargin unmarshal: %w", err)
	}
	if resp.Code != "00000" {
		return fmt.Errorf("TransferFromMargin failed: code=%s msg=%s", resp.Code, resp.Msg)
	}
	return nil
}
