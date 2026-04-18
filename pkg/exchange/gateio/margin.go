package gateio

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"arb/pkg/exchange"
)

// Compile-time interface checks.
var _ exchange.SpotMarginExchange = (*Adapter)(nil)
var _ exchange.FlashRepayer = (*Adapter)(nil)

// ---------------------------------------------------------------------------
// Flash Repay (Flash Swap USDT→coin + MarginRepay)
// ---------------------------------------------------------------------------

// FlashRepay converts USDT collateral to the borrowed coin via Gate.io's flash
// swap API, then repays the full outstanding borrow. This avoids the spot
// buyback path entirely — no partial fills, no dust, no minimum order issues.
//
// Flow: GetMarginBalance → flash swap preview → flash swap execute → MarginRepay.
func (a *Adapter) FlashRepay(coin string) (string, error) {
	// 1. Get outstanding borrow amount.
	mb, err := a.GetMarginBalance(coin)
	if err != nil {
		return "", fmt.Errorf("FlashRepay: get borrow balance: %w", err)
	}
	debt := mb.Borrowed + mb.Interest
	if debt <= 0 {
		return "no_debt", nil
	}

	// Buy slightly more than the debt to cover any rounding/interest accrual
	// during the swap settlement window.
	buyAmount := strconv.FormatFloat(debt*1.002, 'f', 8, 64)

	// 2. Preview flash swap: sell USDT, buy {coin}.
	previewReq := map[string]string{
		"sell_currency": "USDT",
		"buy_currency":  strings.ToUpper(coin),
		"buy_amount":    buyAmount,
	}
	previewBody, _ := json.Marshal(previewReq)
	previewData, err := a.client.Post("/flash_swap/orders/preview", string(previewBody))
	if err != nil {
		return "", fmt.Errorf("FlashRepay: flash swap preview: %w", err)
	}

	var preview struct {
		PreviewID  string `json:"preview_id"`
		SellAmount string `json:"sell_amount"`
		BuyAmount  string `json:"buy_amount"`
	}
	if err := json.Unmarshal(previewData, &preview); err != nil {
		return "", fmt.Errorf("FlashRepay: preview unmarshal: %w (body: %s)", err, string(previewData))
	}
	if preview.PreviewID == "" {
		return "", fmt.Errorf("FlashRepay: preview returned empty ID (body: %s)", string(previewData))
	}

	// 3. Execute flash swap with preview ID.
	swapReq := map[string]string{
		"preview_id":    preview.PreviewID,
		"sell_currency": "USDT",
		"sell_amount":   preview.SellAmount,
		"buy_currency":  strings.ToUpper(coin),
		"buy_amount":    preview.BuyAmount,
	}
	swapBody, _ := json.Marshal(swapReq)
	swapData, err := a.client.Post("/flash_swap/orders", string(swapBody))
	if err != nil {
		return "", fmt.Errorf("FlashRepay: flash swap execute: %w", err)
	}

	var swapResp struct {
		ID     int64  `json:"id"`
		Status int    `json:"status"`
		ErrMsg string `json:"message"`
	}
	if err := json.Unmarshal(swapData, &swapResp); err != nil {
		return "", fmt.Errorf("FlashRepay: swap unmarshal: %w (body: %s)", err, string(swapData))
	}
	if swapResp.Status == 2 {
		return "", fmt.Errorf("FlashRepay: swap failed: %s", swapResp.ErrMsg)
	}

	// 4. Repay the full borrow (repaid_all=true handles any dust).
	if err := a.MarginRepay(exchange.MarginRepayParams{
		Coin:   coin,
		Amount: "0", // triggers repaid_all=true
	}); err != nil {
		// Swap succeeded but repay failed — coin is in account, will be
		// picked up by the step 3b dust sweep in the engine.
		return fmt.Sprintf("swap-%d", swapResp.ID), fmt.Errorf("FlashRepay: swap OK but repay failed: %w", err)
	}

	return fmt.Sprintf("swap-%d", swapResp.ID), nil
}

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
	// Gate.io does not support gtc for market orders — must use ioc.
	if strings.ToLower(params.OrderType) == "market" && tif == "gtc" {
		tif = "ioc"
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
	} else if params.Side == exchange.SideBuy && params.QuoteSize != "" {
		// Gate.io market BUY: amount is in quote currency (USDT).
		orderReq["amount"] = params.QuoteSize
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
// Also queries trade fills to populate FeeDeducted for BUY orders.
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

	result := &exchange.SpotMarginOrderStatus{
		OrderID:   resp.ID,
		Symbol:    resp.CurrencyPair,
		Status:    status,
		FilledQty: qty,
		AvgPrice:  avgPrice,
	}

	// Query fills to get fee deducted from received coin on BUY orders.
	if qty > 0 {
		baseCoin := strings.TrimSuffix(symbol, "USDT")
		fillData, fillErr := a.client.Get("/spot/my_trades", map[string]string{
			"currency_pair": pair,
			"order_id":      orderID,
		})
		if fillErr == nil {
			var fills []struct {
				Fee         string `json:"fee"`
				FeeCurrency string `json:"fee_currency"`
			}
			if json.Unmarshal(fillData, &fills) == nil {
				var totalFee float64
				for _, f := range fills {
					if strings.EqualFold(f.FeeCurrency, baseCoin) {
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

// GetSpotBBO returns the current best bid/offer for the Gate.io spot market.
func (a *Adapter) GetSpotBBO(symbol string) (exchange.BBO, error) {
	data, err := a.client.Get("/spot/tickers", map[string]string{"currency_pair": toGateSymbol(symbol)})
	if err != nil {
		return exchange.BBO{}, fmt.Errorf("GetSpotBBO: %w", err)
	}

	var resp []struct {
		HighestBid string `json:"highest_bid"`
		LowestAsk  string `json:"lowest_ask"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return exchange.BBO{}, fmt.Errorf("GetSpotBBO unmarshal: %w", err)
	}
	if len(resp) == 0 {
		return exchange.BBO{}, fmt.Errorf("GetSpotBBO: no data for %s", symbol)
	}

	bid, _ := strconv.ParseFloat(resp[0].HighestBid, 64)
	ask, _ := strconv.ParseFloat(resp[0].LowestAsk, 64)
	if bid <= 0 || ask <= 0 {
		return exchange.BBO{}, fmt.Errorf("GetSpotBBO: invalid bid/ask for %s", symbol)
	}
	return exchange.BBO{Bid: bid, Ask: ask}, nil
}

// ---------------------------------------------------------------------------
// Spot Margin: Interest Rate
// ---------------------------------------------------------------------------

// GetMarginInterestRateHistory returns historical hourly borrow rates for a coin
// over [start, end]. Gate.io's endpoint has no date-range params so results are
// paginated and filtered client-side. Sorted newest-first; stops when oldest
// record in a page predates start.
func (a *Adapter) GetMarginInterestRateHistory(ctx context.Context, coin string, start, end time.Time) ([]exchange.MarginInterestRatePoint, error) {
	const maxPages = 200
	var all []exchange.MarginInterestRatePoint

	for page := 1; page <= maxPages; page++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		params := map[string]string{
			"currency": strings.ToUpper(coin),
			"tier":     "0",
			"limit":    "100",
			"page":     strconv.Itoa(page),
		}
		data, err := a.client.Get("/unified/history_loan_rate", params)
		if err != nil {
			return nil, fmt.Errorf("gateio GetMarginInterestRateHistory: %w", err)
		}
		var resp struct {
			Currency string `json:"currency"`
			Rates    []struct {
				Time int64  `json:"time"`
				Rate string `json:"rate"`
			} `json:"rates"`
		}
		if err := json.Unmarshal(data, &resp); err != nil {
			return nil, fmt.Errorf("gateio GetMarginInterestRateHistory unmarshal: %w", err)
		}
		if len(resp.Rates) == 0 {
			break
		}
		reachedStart := false
		for _, r := range resp.Rates {
			ts := time.UnixMilli(r.Time)
			if ts.Before(start) {
				reachedStart = true
				continue
			}
			if ts.After(end) {
				continue
			}
			rate, _ := strconv.ParseFloat(r.Rate, 64)
			all = append(all, exchange.MarginInterestRatePoint{
				Timestamp:  ts,
				HourlyRate: rate,
			})
		}
		// Oldest record in this page predates start — no need to go further back.
		oldest := time.UnixMilli(resp.Rates[len(resp.Rates)-1].Time)
		if oldest.Before(start) || reachedStart {
			break
		}
		if len(resp.Rates) < 100 {
			break // last page
		}
	}
	return all, nil
}

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

// GetIsolatedMarginUSDT returns the total USDT balance across all isolated margin
// accounts. In unified mode these are residual balances from positions that were
// opened in isolated margin before the account was upgraded, or from the spot-futures
// engine which uses isolated margin on certain pairs.
func (a *Adapter) GetIsolatedMarginUSDT() (float64, error) {
	data, err := a.client.Get("/margin/accounts", nil)
	if err != nil {
		return 0, fmt.Errorf("GetIsolatedMarginUSDT: %w", err)
	}
	var accounts []struct {
		Quote struct {
			Currency  string `json:"currency"`
			Available string `json:"available"`
			Locked    string `json:"locked"`
		} `json:"quote"`
	}
	if err := json.Unmarshal(data, &accounts); err != nil {
		return 0, fmt.Errorf("GetIsolatedMarginUSDT unmarshal: %w", err)
	}
	var total float64
	for _, acc := range accounts {
		if strings.EqualFold(acc.Quote.Currency, "USDT") {
			avail, _ := strconv.ParseFloat(acc.Quote.Available, 64)
			locked, _ := strconv.ParseFloat(acc.Quote.Locked, 64)
			total += avail + locked
		}
	}
	return total, nil
}

// SweepIsolatedMarginUSDT transfers all idle USDT from isolated margin accounts
// back to the spot wallet. Only sweeps accounts with no borrows (fully idle).
// Uses GET /margin/accounts to find balances, GET /margin/transferable to check
// max transferable, and POST /wallet/transfers to move funds.
// Returns total USDT swept.
func (a *Adapter) SweepIsolatedMarginUSDT() (float64, error) {
	data, err := a.client.Get("/margin/accounts", nil)
	if err != nil {
		return 0, fmt.Errorf("SweepIsolatedMarginUSDT list: %w", err)
	}
	var accounts []struct {
		CurrencyPair string `json:"currency_pair"`
		Quote        struct {
			Currency  string `json:"currency"`
			Available string `json:"available"`
			Borrowed  string `json:"borrowed"`
			Interest  string `json:"interest"`
		} `json:"quote"`
	}
	if err := json.Unmarshal(data, &accounts); err != nil {
		return 0, fmt.Errorf("SweepIsolatedMarginUSDT unmarshal: %w", err)
	}

	var totalSwept float64
	var firstErr error
	for _, acc := range accounts {
		if !strings.EqualFold(acc.Quote.Currency, "USDT") {
			continue
		}
		avail, _ := strconv.ParseFloat(acc.Quote.Available, 64)
		borrowed, _ := strconv.ParseFloat(acc.Quote.Borrowed, 64)
		interest, _ := strconv.ParseFloat(acc.Quote.Interest, 64)
		if avail < 1.0 || borrowed > 0 || interest > 0 {
			continue // skip dust or accounts with active borrows/interest
		}

		// Check max transferable
		tData, err := a.client.Get("/margin/transferable", map[string]string{
			"currency":      "USDT",
			"currency_pair": acc.CurrencyPair,
		})
		if err != nil {
			firstErr = fmt.Errorf("transferable %s: %w", acc.CurrencyPair, err)
			continue
		}
		var tResp struct {
			Amount string `json:"amount"`
		}
		if json.Unmarshal(tData, &tResp) != nil {
			continue
		}
		transferable, _ := strconv.ParseFloat(tResp.Amount, 64)
		if transferable < 1.0 {
			continue
		}
		// Round down to 2 decimals to avoid precision issues
		amt := math.Floor(transferable*100) / 100
		amtStr := strconv.FormatFloat(amt, 'f', 2, 64)

		body := map[string]string{
			"currency":      "USDT",
			"from":          "margin",
			"to":            "spot",
			"amount":        amtStr,
			"currency_pair": acc.CurrencyPair,
		}
		bodyBytes, _ := json.Marshal(body)
		if _, err := a.client.Post("/wallet/transfers", string(bodyBytes)); err != nil {
			firstErr = fmt.Errorf("transfer %s: %w", acc.CurrencyPair, err)
			continue // try remaining pairs
		}
		totalSwept += amt
	}
	return totalSwept, firstErr
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
