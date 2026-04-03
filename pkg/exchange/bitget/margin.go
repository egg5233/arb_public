package bitget

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
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

// FlashRepay uses Bitget's flash-repay to convert USDT collateral and repay
// borrow in one step. No buy order needed — exchange handles the conversion.
func (a *Adapter) FlashRepay(coin string) (string, error) {
	body := map[string]string{
		"coin": coin,
	}
	raw, err := a.client.Post("/api/v2/margin/crossed/account/flash-repay", body)
	if err != nil {
		return "", fmt.Errorf("FlashRepay POST error: %w", err)
	}

	var resp struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			RepayID string `json:"repayId"`
			Coin    string `json:"coin"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return "", fmt.Errorf("FlashRepay unmarshal: %w", err)
	}
	if resp.Code != "00000" {
		return "", fmt.Errorf("FlashRepay failed: code=%s msg=%s", resp.Code, resp.Msg)
	}
	return resp.Data.RepayID, nil
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

// PlaceSpotMarginOrder places a buy or sell order on cross margin or regular spot.
// When AutoBorrow or AutoRepay is set, uses the margin endpoint.
// Otherwise, uses the regular spot endpoint so assets stay in the spot wallet —
// required for Dir B (buy-spot-short) on Bitget's separate accounts.
func (a *Adapter) PlaceSpotMarginOrder(params exchange.SpotMarginOrderParams) (string, error) {
	useMarginEndpoint := params.AutoBorrow || params.AutoRepay

	body := map[string]string{
		"symbol":    params.Symbol,
		"side":      string(params.Side),
		"orderType": params.OrderType,
	}

	if useMarginEndpoint {
		loanType := "normal"
		if params.AutoBorrow && params.AutoRepay {
			loanType = "autoLoanAndRepay"
		} else if params.AutoBorrow {
			loanType = "autoLoan"
		} else if params.AutoRepay {
			loanType = "autoRepay"
		}
		body["loanType"] = loanType
	}

	// Bitget: market BUY only accepts quoteSize (USDT), giving approximate fill.
	// To get exact base qty on BUY, convert to limit IOC with the current ask
	// price + 1% buffer. Querying the live ticker avoids stale-price issues that
	// can exceed Bitget's ~2% price ceiling on limit orders.
	if params.OrderType == "market" && params.Side == exchange.SideBuy && params.Size != "" {
		limitPrice := 0.0
		// Query live spot ticker for current price.
		if raw, err := a.client.Get("/api/v2/spot/market/tickers", map[string]string{"symbol": params.Symbol}); err == nil {
			var tickerResp struct {
				Code string `json:"code"`
				Data []struct {
					AskPr string `json:"askPr"`
				} `json:"data"`
			}
			if json.Unmarshal([]byte(raw), &tickerResp) == nil && tickerResp.Code == "00000" && len(tickerResp.Data) > 0 {
				askPrice, _ := strconv.ParseFloat(tickerResp.Data[0].AskPr, 64)
				if askPrice > 0 {
					limitPrice = askPrice * 1.01 // 1% above current ask, well under 2% ceiling
				}
			}
		}
		// Fallback: derive from quoteSize/size if ticker failed.
		if limitPrice <= 0 {
			sz, _ := strconv.ParseFloat(params.Size, 64)
			qs, _ := strconv.ParseFloat(params.QuoteSize, 64)
			if sz > 0 && qs > 0 {
				limitPrice = qs / sz
			}
		}
		if limitPrice > 0 {
			body["orderType"] = "limit"
			body["force"] = "ioc"
			body["price"] = strconv.FormatFloat(limitPrice, 'f', 2, 64)
			if useMarginEndpoint {
				body["baseSize"] = params.Size
			} else {
				body["size"] = params.Size
			}
		} else if useMarginEndpoint {
			body["quoteSize"] = params.QuoteSize
		} else {
			body["size"] = params.QuoteSize
		}
	} else if params.OrderType == "market" && params.Side == exchange.SideBuy {
		if useMarginEndpoint {
			body["quoteSize"] = params.QuoteSize
		} else {
			body["size"] = params.QuoteSize
		}
	} else {
		if useMarginEndpoint {
			body["baseSize"] = params.Size
		} else {
			body["size"] = params.Size
		}
	}
	if body["orderType"] != "limit" && params.OrderType != "market" && params.Force != "" {
		body["force"] = params.Force
	}
	if params.Price != "" {
		body["price"] = params.Price
	}
	if params.ClientOid != "" {
		body["clientOid"] = params.ClientOid
	}

	var endpoint string
	if useMarginEndpoint {
		endpoint = "/api/v2/margin/crossed/place-order"
	} else {
		body["force"] = "gtc"
		endpoint = "/api/v2/spot/trade/place-order"
	}

	raw, err := a.client.Post(endpoint, body)
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

// GetSpotMarginOrder returns the native order state from Bitget.
// Tries regular spot endpoints first (Dir B orders), falls back to margin endpoints (Dir A orders).
// Also queries trade fills to populate FeeDeducted for BUY orders.
func (a *Adapter) GetSpotMarginOrder(orderID, symbol string) (*exchange.SpotMarginOrderStatus, error) {
	startTime := strconv.FormatInt(time.Now().Add(-24*time.Hour).UnixMilli(), 10)

	// Try regular spot endpoints first (Dir B orders placed on spot).
	// Spot endpoints have different response format: data[] with baseVolume/priceAvg.
	order := a.getSpotTradeOrder("/api/v2/spot/trade/unfilled-orders", orderID, symbol, startTime)
	if order == nil {
		order = a.getSpotTradeOrder("/api/v2/spot/trade/history-orders", orderID, symbol, startTime)
	}
	if order != nil {
		a.populateBitgetFeeDeducted(order, orderID, symbol)
		return order, nil
	}

	// Fall back to margin endpoints (Dir A orders placed on margin).
	order, err := a.getSpotMarginOrder("/api/v2/margin/crossed/open-orders", orderID, symbol, startTime)
	if err != nil {
		return nil, err
	}
	if order != nil {
		a.populateBitgetFeeDeducted(order, orderID, symbol)
		return order, nil
	}
	order, err = a.getSpotMarginOrder("/api/v2/margin/crossed/history-orders", orderID, symbol, startTime)
	if order != nil {
		a.populateBitgetFeeDeducted(order, orderID, symbol)
	}
	return order, err
}

// GetSpotBBO returns the current best bid/offer for the Bitget spot market.
func (a *Adapter) GetSpotBBO(symbol string) (exchange.BBO, error) {
	raw, err := a.client.Get("/api/v2/spot/market/tickers", map[string]string{"symbol": symbol})
	if err != nil {
		return exchange.BBO{}, fmt.Errorf("GetSpotBBO: %w", err)
	}

	var resp struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
		Data []struct {
			BidPr string `json:"bidPr"`
			AskPr string `json:"askPr"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return exchange.BBO{}, fmt.Errorf("GetSpotBBO unmarshal: %w", err)
	}
	if resp.Code != "00000" {
		return exchange.BBO{}, fmt.Errorf("GetSpotBBO failed: code=%s msg=%s", resp.Code, resp.Msg)
	}
	if len(resp.Data) == 0 {
		return exchange.BBO{}, fmt.Errorf("GetSpotBBO: no data for %s", symbol)
	}

	bid, _ := strconv.ParseFloat(resp.Data[0].BidPr, 64)
	ask, _ := strconv.ParseFloat(resp.Data[0].AskPr, 64)
	if bid <= 0 || ask <= 0 {
		return exchange.BBO{}, fmt.Errorf("GetSpotBBO: invalid bid/ask for %s", symbol)
	}
	return exchange.BBO{Bid: bid, Ask: ask}, nil
}

// populateBitgetFeeDeducted queries spot or margin fills for an order and sets FeeDeducted
// when the fee is paid in the base coin (i.e., deducted from the received coin on BUY orders).
func (a *Adapter) populateBitgetFeeDeducted(order *exchange.SpotMarginOrderStatus, orderID, symbol string) {
	if order == nil || order.FilledQty <= 0 {
		return
	}
	baseCoin := strings.TrimSuffix(symbol, "USDT")

	type fillResp struct {
		Code string `json:"code"`
		Data []struct {
			FeeDetail struct {
				FeeCoin  string `json:"feeCoin"`
				TotalFee string `json:"totalFee"`
			} `json:"feeDetail"`
		} `json:"data"`
	}

	fillParams := map[string]string{
		"symbol":  symbol,
		"orderId": orderID,
	}

	var totalFee float64
	var found bool

	// Try spot fills first.
	if raw, err := a.client.Get("/api/v2/spot/trade/fills", fillParams); err == nil {
		var resp fillResp
		if json.Unmarshal([]byte(raw), &resp) == nil && resp.Code == "00000" {
			for _, f := range resp.Data {
				if strings.EqualFold(f.FeeDetail.FeeCoin, baseCoin) {
					fee, _ := strconv.ParseFloat(f.FeeDetail.TotalFee, 64)
					totalFee += fee
					found = true
				}
			}
		}
	}

	// Try margin fills if spot fills returned nothing.
	if !found {
		if raw, err := a.client.Get("/api/v2/margin/crossed/fills", fillParams); err == nil {
			var resp fillResp
			if json.Unmarshal([]byte(raw), &resp) == nil && resp.Code == "00000" {
				for _, f := range resp.Data {
					if strings.EqualFold(f.FeeDetail.FeeCoin, baseCoin) {
						fee, _ := strconv.ParseFloat(f.FeeDetail.TotalFee, 64)
						totalFee += fee
					}
				}
			}
		}
	}

	if totalFee != 0 {
		order.FeeDeducted = math.Abs(totalFee)
	}
}

// getSpotTradeOrder queries Bitget's regular spot trade endpoints which return
// data[] with baseVolume/priceAvg (different from margin's data.orderList[]).
func (a *Adapter) getSpotTradeOrder(path, orderID, symbol, startTime string) *exchange.SpotMarginOrderStatus {
	raw, err := a.client.Get(path, map[string]string{
		"symbol":    symbol,
		"orderId":   orderID,
		"startTime": startTime,
		"limit":     "100",
	})
	if err != nil {
		return nil
	}

	var resp struct {
		Code string `json:"code"`
		Data []struct {
			OrderID    string `json:"orderId"`
			Symbol     string `json:"symbol"`
			Status     string `json:"status"`
			BaseVolume string `json:"baseVolume"`
			PriceAvg   string `json:"priceAvg"`
		} `json:"data"`
	}
	if json.Unmarshal([]byte(raw), &resp) != nil || resp.Code != "00000" || len(resp.Data) == 0 {
		return nil
	}

	// Find the matching order.
	for _, o := range resp.Data {
		if o.OrderID == orderID {
			qty, _ := strconv.ParseFloat(o.BaseVolume, 64)
			avgPrice, _ := strconv.ParseFloat(o.PriceAvg, 64)
			status := o.Status
			switch status {
			case "partial_fill", "partially_fill":
				status = "partially_filled"
			case "reject":
				status = "rejected"
			}
			return &exchange.SpotMarginOrderStatus{
				OrderID:   o.OrderID,
				Symbol:    o.Symbol,
				Status:    status,
				FilledQty: qty,
				AvgPrice:  avgPrice,
			}
		}
	}
	return nil
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
		// Coin not in margin account — return zero balance. This is normal for
		// exchanges with separate margin accounts when no funds have been
		// transferred or borrowed yet.
		maxBorrowable, _ := a.fetchMaxBorrowable(coin)
		return &exchange.MarginBalance{
			Coin:          coin,
			MaxBorrowable: maxBorrowable,
		}, nil
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
