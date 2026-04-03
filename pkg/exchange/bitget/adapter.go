package bitget

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"arb/pkg/exchange"
)

// Adapter implements exchange.Exchange for Bitget.
type Adapter struct {
	client               *Client
	ws                   *WSClient
	wsPriv               *WSPrivateClient
	apiKey               string
	secretKey            string
	passphrase           string
	orderCallback        func(exchange.OrderUpdate)
	wsMetricsCallback    exchange.WSMetricsCallback
	orderMetricsCallback exchange.OrderMetricsCallback
}

// NewAdapter creates a Bitget adapter from the unified config.
// WebSocket clients are created lazily in their respective Start methods.
func NewAdapter(cfg exchange.ExchangeConfig) *Adapter {
	return &Adapter{
		client:     NewClient(cfg.ApiKey, cfg.SecretKey, cfg.Passphrase),
		apiKey:     cfg.ApiKey,
		secretKey:  cfg.SecretKey,
		passphrase: cfg.Passphrase,
	}
}

func (a *Adapter) Name() string { return "bitget" }

func (a *Adapter) SetOrderCallback(fn func(exchange.OrderUpdate)) {
	a.orderCallback = fn
}

func (a *Adapter) SetMetricsCallback(fn exchange.MetricsCallback) {
	if a.client != nil {
		a.client.SetMetricsCallback(fn)
	}
}

func (a *Adapter) SetWSMetricsCallback(fn exchange.WSMetricsCallback) {
	a.wsMetricsCallback = fn
	if a.ws != nil {
		a.ws.SetMetricsCallback(fn)
	}
}

func (a *Adapter) SetOrderMetricsCallback(fn exchange.OrderMetricsCallback) {
	a.orderMetricsCallback = fn
	if a.wsPriv != nil {
		a.wsPriv.SetOrderMetricsCallback(fn)
	}
}

func (a *Adapter) CheckPermissions() exchange.PermissionResult {
	r := exchange.PermissionResult{Method: "inferred"}
	// Bitget client returns raw JSON string without checking code.
	// Must parse response body for code "40009" (permission denied).
	checkBitget := func(resp string, err error) exchange.PermStatus {
		if err != nil {
			return exchange.PermUnknown
		}
		var body struct {
			Code string `json:"code"`
		}
		if json.Unmarshal([]byte(resp), &body) == nil && body.Code == "40009" {
			return exchange.PermDenied
		}
		return exchange.PermGranted
	}
	resp, err := a.client.Get("/api/v2/spot/account/assets", map[string]string{})
	r.Read = checkBitget(resp, err)
	resp, err = a.client.Get("/api/v2/mix/order/orders-pending", map[string]string{"productType": "USDT-FUTURES"})
	r.FuturesTrade = checkBitget(resp, err)
	resp, err = a.client.Post("/api/v2/spot/wallet/withdrawal", map[string]string{
		"coin": "USDT", "address": "test", "chain": "BSC", "size": "0", "transferType": "on_chain",
	})
	r.Withdraw = checkBitget(resp, err)
	resp, err = a.client.Post("/api/v2/spot/wallet/transfer", map[string]string{
		"fromType": "usdt_futures", "toType": "spot", "amount": "0", "coin": "USDT",
	})
	r.Transfer = checkBitget(resp, err)
	return r
}

// ==================== Orders ====================

func (a *Adapter) PlaceOrder(req exchange.PlaceOrderParams) (string, error) {
	params := map[string]string{
		"symbol":      req.Symbol,
		"productType": productTypeUSDTFutures,
		"marginCoin":  marginCoinUSDT,
		"marginMode":  "crossed",
		"size":        req.Size,
		"side":        string(req.Side),
		"orderType":   req.OrderType,
		"force":       req.Force,
	}
	if req.ClientOid != "" {
		params["clientOid"] = req.ClientOid
	} else {
		params["clientOid"] = fmt.Sprintf("arb-%d", time.Now().UnixNano())
	}
	if req.Price != "" {
		params["price"] = req.Price
	}
	if req.ReduceOnly {
		params["reduceOnly"] = "YES"
	}

	raw, err := a.client.Post("/api/v2/mix/order/place-order", params)
	if err != nil {
		return "", err
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
		return "", fmt.Errorf("unmarshal placeOrder: %w", err)
	}
	if resp.Code != "00000" {
		return "", fmt.Errorf("place-order failed: code=%s msg=%s", resp.Code, resp.Msg)
	}
	if a.orderMetricsCallback != nil {
		a.orderMetricsCallback(exchange.OrderMetricEvent{
			Type:      exchange.OrderMetricPlaced,
			OrderID:   resp.Data.OrderId,
			Timestamp: time.Now(),
		})
	}
	return resp.Data.OrderId, nil
}

func (a *Adapter) CancelOrder(symbol, orderID string) error {
	params := map[string]string{
		"symbol":      symbol,
		"productType": productTypeUSDTFutures,
		"orderId":     orderID,
	}

	raw, err := a.client.Post("/api/v2/mix/order/cancel-order", params)
	if err != nil {
		return err
	}
	var resp struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return fmt.Errorf("unmarshal cancel: %w", err)
	}
	// 00000 = success, 43011/43025 = order already completed/cancelled (idempotent)
	if resp.Code != "00000" && resp.Code != "43011" && resp.Code != "43025" {
		return fmt.Errorf("cancel failed: %s %s", resp.Code, resp.Msg)
	}
	return nil
}

func (a *Adapter) GetPendingOrders(symbol string) ([]exchange.Order, error) {
	params := map[string]string{
		"symbol":      symbol,
		"productType": productTypeUSDTFutures,
	}

	raw, err := a.client.Get("/api/v2/mix/order/orders-pending", params)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			EntrustedList []struct {
				OrderId   string `json:"orderId"`
				ClientOid string `json:"clientOid"`
				Symbol    string `json:"symbol"`
				Side      string `json:"side"`
				OrderType string `json:"orderType"`
				Price     string `json:"price"`
				Size      string `json:"size"`
				Status    string `json:"status"`
			} `json:"entrustedList"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return nil, fmt.Errorf("unmarshal pending orders: %w", err)
	}
	if resp.Code != "00000" {
		return nil, fmt.Errorf("pending orders failed: %s %s", resp.Code, resp.Msg)
	}

	out := make([]exchange.Order, len(resp.Data.EntrustedList))
	for i, o := range resp.Data.EntrustedList {
		out[i] = exchange.Order{
			OrderID:   o.OrderId,
			ClientOid: o.ClientOid,
			Symbol:    o.Symbol,
			Side:      o.Side,
			OrderType: o.OrderType,
			Price:     o.Price,
			Size:      o.Size,
			Status:    o.Status,
		}
	}
	return out, nil
}

func (a *Adapter) GetOrderFilledQty(orderID, symbol string) (float64, error) {
	params := map[string]string{
		"symbol":      symbol,
		"orderId":     orderID,
		"productType": productTypeUSDTFutures,
	}

	raw, err := a.client.Get("/api/v2/mix/order/detail", params)
	if err != nil {
		return 0, err
	}

	var resp struct {
		Code string `json:"code"`
		Data struct {
			BaseVolume string `json:"baseVolume"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return 0, err
	}
	if resp.Data.BaseVolume == "" {
		return 0, nil
	}
	qty, err := strconv.ParseFloat(resp.Data.BaseVolume, 64)
	if err == nil && qty > 0 && a.orderMetricsCallback != nil {
		a.orderMetricsCallback(exchange.OrderMetricEvent{
			Type:      exchange.OrderMetricFilled,
			OrderID:   orderID,
			FilledQty: qty,
			Timestamp: time.Now(),
		})
	}
	return qty, err
}

// ==================== Positions ====================

func (a *Adapter) GetPosition(symbol string) ([]exchange.Position, error) {
	params := map[string]string{
		"symbol":      symbol,
		"marginCoin":  marginCoinUSDT,
		"productType": productTypeUSDTFutures,
	}

	raw, err := a.client.Get("/api/v2/mix/position/single-position", params)
	if err != nil {
		return nil, err
	}

	return parsePositions(raw)
}

func (a *Adapter) GetAllPositions() ([]exchange.Position, error) {
	params := map[string]string{
		"productType": productTypeUSDTFutures,
		"marginCoin":  marginCoinUSDT,
	}

	raw, err := a.client.Get("/api/v2/mix/position/all-position", params)
	if err != nil {
		return nil, err
	}

	return parsePositions(raw)
}

func parsePositions(raw string) ([]exchange.Position, error) {
	var resp struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
		Data []struct {
			Symbol           string `json:"symbol"`
			HoldSide         string `json:"holdSide"`
			Total            string `json:"total"`
			Available        string `json:"available"`
			AverageOpenPrice string `json:"openPriceAvg"`
			UnrealizedPL     string `json:"unrealizedPL"`
			Leverage         string `json:"leverage"`
			MarginMode       string `json:"marginMode"`
			LiquidationPrice string `json:"liquidationPrice"`
			MarkPrice        string `json:"markPrice"`
			TotalFee         string `json:"totalFee"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return nil, fmt.Errorf("unmarshal position: %w", err)
	}
	if resp.Code != "00000" {
		return nil, fmt.Errorf("position failed: code=%s msg=%s", resp.Code, resp.Msg)
	}

	out := make([]exchange.Position, 0, len(resp.Data))
	for _, p := range resp.Data {
		// Filter out zero-size positions
		total, _ := strconv.ParseFloat(p.Total, 64)
		if total == 0 {
			continue
		}
		out = append(out, exchange.Position{
			Symbol:           p.Symbol,
			HoldSide:         p.HoldSide,
			Total:            p.Total,
			Available:        p.Available,
			AverageOpenPrice: p.AverageOpenPrice,
			UnrealizedPL:     p.UnrealizedPL,
			Leverage:         p.Leverage,
			MarginMode:       p.MarginMode,
			LiquidationPrice: p.LiquidationPrice,
			MarkPrice:        p.MarkPrice,
			FundingFee:       p.TotalFee,
		})
	}
	return out, nil
}

// ==================== Account Config ====================

func (a *Adapter) SetLeverage(symbol string, leverage string, holdSide string) error {
	params := map[string]string{
		"symbol":      symbol,
		"productType": productTypeUSDTFutures,
		"marginCoin":  marginCoinUSDT,
		"leverage":    leverage,
		"holdSide":    holdSide,
	}

	raw, err := a.client.Post("/api/v2/mix/account/set-leverage", params)
	if err != nil {
		return fmt.Errorf("SetLeverage POST error: %w", err)
	}
	var resp struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return fmt.Errorf("SetLeverage unmarshal: %w", err)
	}
	if resp.Code != "00000" {
		return fmt.Errorf("SetLeverage failed: code=%s msg=%s", resp.Code, resp.Msg)
	}
	return nil
}

func (a *Adapter) SetMarginMode(symbol string, mode string) error {
	// Bitget API expects "crossed" not "cross" for cross margin mode.
	apiMode := mode
	if strings.ToLower(mode) == "cross" {
		apiMode = "crossed"
	}

	params := map[string]string{
		"symbol":      symbol,
		"productType": productTypeUSDTFutures,
		"marginCoin":  marginCoinUSDT,
		"marginMode":  apiMode, // "isolated" or "crossed"
	}

	raw, err := a.client.Post("/api/v2/mix/account/set-margin-mode", params)
	if err != nil {
		return fmt.Errorf("SetMarginMode POST error: %w", err)
	}
	var resp struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return fmt.Errorf("SetMarginMode unmarshal: %w", err)
	}
	// 00000 = success, 40872 = already in that mode (not an error)
	if resp.Code != "00000" && resp.Code != "40872" {
		return fmt.Errorf("SetMarginMode failed: code=%s msg=%s", resp.Code, resp.Msg)
	}
	return nil
}

// ==================== Contract Info ====================

func (a *Adapter) LoadAllContracts() (map[string]exchange.ContractInfo, error) {
	params := map[string]string{
		"productType": productTypeUSDTFutures,
	}

	raw, err := a.client.Get("/api/v2/mix/market/contracts", params)
	if err != nil {
		return nil, fmt.Errorf("GetContracts error: %w", err)
	}

	var resp struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
		Data []struct {
			Symbol         string `json:"symbol"`
			MinTradeNum    string `json:"minTradeNum"`
			VolumePlace    string `json:"volumePlace"`
			SizeMultiplier string `json:"sizeMultiplier"`
			MaxOrderQty    string `json:"maxOrderQty"`
			PricePlace     string `json:"pricePlace"`
			PriceEndStep   string `json:"priceEndStep"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return nil, fmt.Errorf("unmarshal contracts: %w", err)
	}
	if resp.Code != "00000" {
		return nil, fmt.Errorf("GetContracts failed code=%s msg=%s", resp.Code, resp.Msg)
	}

	result := make(map[string]exchange.ContractInfo, len(resp.Data))
	for _, c := range resp.Data {
		minSize, _ := strconv.ParseFloat(c.MinTradeNum, 64)
		stepSize, _ := strconv.ParseFloat(c.SizeMultiplier, 64)
		maxSize, _ := strconv.ParseFloat(c.MaxOrderQty, 64)
		volPlace, _ := strconv.Atoi(c.VolumePlace)

		priceEndStep, _ := strconv.ParseFloat(c.PriceEndStep, 64)
		priceDec, _ := strconv.Atoi(c.PricePlace)

		if stepSize <= 0 {
			if volPlace > 0 {
				stepSize = math.Pow10(-volPlace)
			} else {
				stepSize = 1e-4
			}
		}

		var priceStep float64
		if priceEndStep > 0 && priceDec >= 0 {
			priceStep = priceEndStep * math.Pow10(-priceDec)
		} else if priceDec > 0 {
			priceStep = math.Pow10(-priceDec)
		} else {
			priceStep = 1e-4
		}

		result[c.Symbol] = exchange.ContractInfo{
			Symbol:        c.Symbol,
			MinSize:       minSize,
			StepSize:      stepSize,
			MaxSize:       maxSize,
			SizeDecimals:  volPlace,
			PriceStep:     priceStep,
			PriceDecimals: priceDec,
		}
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("no contract settings loaded")
	}
	return result, nil
}

// ==================== Funding Rate ====================

func (a *Adapter) GetFundingRate(symbol string) (*exchange.FundingRate, error) {
	params := map[string]string{
		"symbol":      symbol,
		"productType": productTypeUSDTFutures,
	}

	raw, err := a.client.Get("/api/v2/mix/market/current-fund-rate", params)
	if err != nil {
		return nil, fmt.Errorf("GetFundingRate error: %w", err)
	}

	var resp struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
		Data []struct {
			Symbol              string `json:"symbol"`
			FundingRate         string `json:"fundingRate"`
			NextFundRate        string `json:"nextFundRate"`
			FundingTime         string `json:"fundingTime"`         // ms timestamp (legacy)
			NextUpdate          string `json:"nextUpdate"`          // ms timestamp
			FundingRateInterval string `json:"fundingRateInterval"` // hours, e.g. "4"
			MaxFundingRate      string `json:"maxFundingRate"`
			MinFundingRate      string `json:"minFundingRate"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return nil, fmt.Errorf("unmarshal funding rate: %w", err)
	}
	if resp.Code != "00000" {
		return nil, fmt.Errorf("GetFundingRate failed: code=%s msg=%s", resp.Code, resp.Msg)
	}
	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("GetFundingRate: no data for %s", symbol)
	}

	d := resp.Data[0]
	rate, _ := strconv.ParseFloat(d.FundingRate, 64)
	nextRate, _ := strconv.ParseFloat(d.NextFundRate, 64)

	var nextFunding time.Time
	// Prefer nextUpdate over legacy fundingTime.
	if ts, err := strconv.ParseInt(d.NextUpdate, 10, 64); err == nil && ts > 0 {
		nextFunding = time.UnixMilli(ts)
	} else if ts, err := strconv.ParseInt(d.FundingTime, 10, 64); err == nil && ts > 0 {
		nextFunding = time.UnixMilli(ts)
	}

	// Use interval from the funding rate response itself (most reliable).
	// Falls back to contracts API, then 8h default.
	var interval time.Duration
	if h, err := strconv.Atoi(d.FundingRateInterval); err == nil && h > 0 {
		interval = time.Duration(h) * time.Hour
	} else {
		interval, err = a.GetFundingInterval(symbol)
		if err != nil || interval <= 0 {
			interval = 8 * time.Hour
		}
	}

	fr := &exchange.FundingRate{
		Symbol:      d.Symbol,
		Rate:        rate,
		NextRate:    nextRate,
		Interval:    interval,
		NextFunding: nextFunding,
	}
	if v, err := strconv.ParseFloat(d.MaxFundingRate, 64); err == nil {
		fr.MaxRate = &v
	}
	if v, err := strconv.ParseFloat(d.MinFundingRate, 64); err == nil {
		fr.MinRate = &v
	}
	return fr, nil
}

func (a *Adapter) GetFundingInterval(symbol string) (time.Duration, error) {
	params := map[string]string{
		"symbol":      symbol,
		"productType": productTypeUSDTFutures,
	}

	raw, err := a.client.Get("/api/v2/mix/market/contracts", params)
	if err != nil {
		return 8 * time.Hour, nil // default on error
	}

	var resp struct {
		Code string `json:"code"`
		Data []struct {
			Symbol          string `json:"symbol"`
			FundingInterval string `json:"fundInterval"` // e.g. "8" (hours)
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return 8 * time.Hour, nil
	}
	if resp.Code != "00000" || len(resp.Data) == 0 {
		return 8 * time.Hour, nil
	}

	hours, err := strconv.Atoi(resp.Data[0].FundingInterval)
	if err != nil || hours <= 0 {
		return 8 * time.Hour, nil
	}
	return time.Duration(hours) * time.Hour, nil
}

// ==================== Balance ====================

func (a *Adapter) GetFuturesBalance() (*exchange.Balance, error) {
	params := map[string]string{
		"symbol":      "BTCUSDT", // required param, any symbol works
		"productType": productTypeUSDTFutures,
		"marginCoin":  marginCoinUSDT,
	}

	raw, err := a.client.Get("/api/v2/mix/account/account", params)
	if err != nil {
		return nil, fmt.Errorf("GetFuturesBalance: %w", err)
	}

	var resp struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			AccountEquity   string `json:"accountEquity"`
			Available       string `json:"available"`
			Frozen          string `json:"locked"`
			CrossedRiskRate string `json:"crossedRiskRate"`
			MaxTransferOut  string `json:"maxTransferOut"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return nil, fmt.Errorf("GetFuturesBalance unmarshal: %w", err)
	}
	if resp.Code != "00000" {
		return nil, fmt.Errorf("GetFuturesBalance failed: code=%s msg=%s", resp.Code, resp.Msg)
	}

	total, _ := strconv.ParseFloat(resp.Data.AccountEquity, 64)
	avail, _ := strconv.ParseFloat(resp.Data.Available, 64)
	frozen, _ := strconv.ParseFloat(resp.Data.Frozen, 64)
	marginRatio, _ := strconv.ParseFloat(resp.Data.CrossedRiskRate, 64)

	// Defensive: if available is 0 but equity exists, fall back to equity minus frozen.
	if avail <= 0 && total > 0 {
		avail = total - frozen
	}

	maxTransferOut, _ := strconv.ParseFloat(resp.Data.MaxTransferOut, 64)

	return &exchange.Balance{
		Total:          total,
		Available:      avail,
		Frozen:         frozen,
		Currency:       marginCoinUSDT,
		MarginRatio:    marginRatio,
		MaxTransferOut: maxTransferOut,
	}, nil
}

func (a *Adapter) GetSpotBalance() (*exchange.Balance, error) {
	raw, err := a.client.Get("/api/v2/spot/account/assets", map[string]string{"coin": "USDT"})
	if err != nil {
		return nil, fmt.Errorf("GetSpotBalance: %w", err)
	}

	var resp struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
		Data []struct {
			Coin      string `json:"coin"`
			Available string `json:"available"`
			Frozen    string `json:"locked"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return nil, fmt.Errorf("GetSpotBalance unmarshal: %w", err)
	}
	if resp.Code != "00000" {
		return nil, fmt.Errorf("GetSpotBalance failed: code=%s msg=%s", resp.Code, resp.Msg)
	}

	for _, d := range resp.Data {
		if strings.EqualFold(d.Coin, "USDT") {
			avail, _ := strconv.ParseFloat(d.Available, 64)
			frozen, _ := strconv.ParseFloat(d.Frozen, 64)
			return &exchange.Balance{
				Total:     avail + frozen,
				Available: avail,
				Frozen:    frozen,
				Currency:  "USDT",
			}, nil
		}
	}
	return &exchange.Balance{Currency: "USDT"}, nil
}

// ==================== Orderbook ====================

func (a *Adapter) GetOrderbook(symbol string, depth int) (*exchange.Orderbook, error) {
	if depth <= 0 {
		depth = 20
	}

	params := map[string]string{
		"symbol":      symbol,
		"productType": productTypeUSDTFutures,
		"limit":       strconv.Itoa(depth),
	}

	raw, err := a.client.Get("/api/v2/mix/market/merge-depth", params)
	if err != nil {
		return nil, fmt.Errorf("GetOrderbook error: %w", err)
	}

	var resp struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			Asks [][]json.RawMessage `json:"asks"` // [price, size] — may be string or number
			Bids [][]json.RawMessage `json:"bids"`
			Ts   string              `json:"ts"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return nil, fmt.Errorf("unmarshal orderbook: %w", err)
	}
	if resp.Code != "00000" {
		return nil, fmt.Errorf("GetOrderbook failed: code=%s msg=%s", resp.Code, resp.Msg)
	}

	ob := &exchange.Orderbook{
		Symbol: symbol,
	}

	ts, _ := strconv.ParseInt(resp.Data.Ts, 10, 64)
	if ts > 0 {
		ob.Time = time.UnixMilli(ts)
	} else {
		ob.Time = time.Now()
	}

	ob.Asks = make([]exchange.PriceLevel, 0, len(resp.Data.Asks))
	for _, ask := range resp.Data.Asks {
		if len(ask) < 2 {
			continue
		}
		p := parseRawFloat(ask[0])
		q := parseRawFloat(ask[1])
		ob.Asks = append(ob.Asks, exchange.PriceLevel{Price: p, Quantity: q})
	}

	ob.Bids = make([]exchange.PriceLevel, 0, len(resp.Data.Bids))
	for _, bid := range resp.Data.Bids {
		if len(bid) < 2 {
			continue
		}
		p := parseRawFloat(bid[0])
		q := parseRawFloat(bid[1])
		ob.Bids = append(ob.Bids, exchange.PriceLevel{Price: p, Quantity: q})
	}

	return ob, nil
}

// ==================== Internal Transfer ====================

// TransferToSpot moves funds from futures (usdt-futures) to spot account.
func (a *Adapter) TransferToSpot(coin string, amount string) error {
	reqParams := map[string]string{
		"fromType": "usdt_futures",
		"toType":   "spot",
		"coin":     coin,
		"amount":   amount,
	}

	raw, err := a.client.Post("/api/v2/spot/wallet/transfer", reqParams)
	if err != nil {
		return fmt.Errorf("TransferToSpot: %w", err)
	}

	var resp struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return fmt.Errorf("TransferToSpot unmarshal: %w", err)
	}
	if resp.Code != "00000" {
		return fmt.Errorf("TransferToSpot failed: code=%s msg=%s", resp.Code, resp.Msg)
	}
	return nil
}

// TransferToFutures moves funds from spot to futures (usdt-futures) account.
func (a *Adapter) TransferToFutures(coin string, amount string) error {
	reqParams := map[string]string{
		"fromType": "spot",
		"toType":   "usdt_futures",
		"coin":     coin,
		"amount":   amount,
	}

	raw, err := a.client.Post("/api/v2/spot/wallet/transfer", reqParams)
	if err != nil {
		return fmt.Errorf("TransferToFutures: %w", err)
	}

	var resp struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return fmt.Errorf("TransferToFutures unmarshal: %w", err)
	}
	if resp.Code != "00000" {
		return fmt.Errorf("TransferToFutures failed: code=%s msg=%s", resp.Code, resp.Msg)
	}
	return nil
}

// ==================== Withdraw ====================

func (a *Adapter) Withdraw(params exchange.WithdrawParams) (*exchange.WithdrawResult, error) {
	chain := mapChainToBitget(params.Chain)
	reqParams := map[string]string{
		"coin":         params.Coin,
		"transferType": "on_chain",
		"chain":        chain,
		"address":      params.Address,
		"size":         params.Amount,
	}

	fmt.Printf("[bitget] Withdraw request: coin=%s chain=%s address=%s size=%s\n",
		params.Coin, chain, params.Address, params.Amount)

	raw, err := a.client.Post("/api/v2/spot/wallet/withdrawal", reqParams)
	if err != nil {
		return nil, fmt.Errorf("Withdraw: %w", err)
	}

	var resp struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			OrderId string `json:"orderId"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return nil, fmt.Errorf("Withdraw unmarshal: %w", err)
	}
	if resp.Code != "00000" {
		return nil, fmt.Errorf("Withdraw failed: code=%s msg=%s", resp.Code, resp.Msg)
	}
	return &exchange.WithdrawResult{
		TxID:   resp.Data.OrderId,
		Status: "submitted",
	}, nil
}

// WithdrawFeeInclusive returns false because Bitget Withdraw amount is net (recipient gets full amount, fee deducted separately).
func (a *Adapter) WithdrawFeeInclusive() bool { return false }

// GetWithdrawFee queries the Bitget API for the withdrawal fee of a coin on a given chain.
func (a *Adapter) GetWithdrawFee(coin, chain string) (float64, error) {
	network := mapChainToBitgetNetwork(chain)
	params := map[string]string{
		"coin": coin,
	}
	raw, err := a.client.Get("/api/v2/spot/public/coins", params)
	if err != nil {
		return 0, fmt.Errorf("bitget GetWithdrawFee: %w", err)
	}

	var resp struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
		Data []struct {
			Coin   string `json:"coin"`
			Chains []struct {
				Chain            string `json:"chain"`
				WithdrawFee      string `json:"withdrawFee"`
				ExtraWithdrawFee string `json:"extraWithdrawFee"`
			} `json:"chains"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return 0, fmt.Errorf("bitget GetWithdrawFee unmarshal: %w", err)
	}
	if resp.Code != "00000" {
		return 0, fmt.Errorf("bitget GetWithdrawFee API error: code=%s msg=%s", resp.Code, resp.Msg)
	}

	for _, c := range resp.Data {
		if !strings.EqualFold(c.Coin, coin) {
			continue
		}
		for _, ch := range c.Chains {
			if strings.EqualFold(ch.Chain, network) {
				fee, err := strconv.ParseFloat(ch.WithdrawFee, 64)
				if err != nil {
					return 0, fmt.Errorf("bitget GetWithdrawFee parse fee: %w", err)
				}
				extra, _ := strconv.ParseFloat(ch.ExtraWithdrawFee, 64)
				return fee + extra, nil
			}
		}
		return 0, fmt.Errorf("bitget GetWithdrawFee: chain %s not found for %s", network, coin)
	}
	return 0, fmt.Errorf("bitget GetWithdrawFee: coin %s not found", coin)
}

func mapChainToBitgetNetwork(chain string) string {
	switch chain {
	case "BEP20":
		return "bep20"
	case "APT":
		return "Aptos"
	default:
		return strings.ToLower(chain)
	}
}

func mapChainToBitget(chain string) string {
	switch chain {
	case "BEP20":
		return "bep20"
	case "APT":
		return "aptos"
	default:
		return strings.ToLower(chain)
	}
}

// ==================== WebSocket: Prices ====================

func (a *Adapter) StartPriceStream(symbols []string) {
	a.ws = NewWSClient()
	a.ws.SetMetricsCallback(a.wsMetricsCallback)
	a.ws.Start(symbols)
}

func (a *Adapter) SubscribeSymbol(symbol string) bool {
	if a.ws == nil {
		return false
	}
	return a.ws.SubscribeSymbol(symbol)
}

func (a *Adapter) GetBBO(symbol string) (exchange.BBO, bool) {
	if a.ws == nil {
		return exchange.BBO{}, false
	}
	if !strings.Contains(symbol, "USDT") {
		symbol += "USDT"
	}
	val, ok := a.ws.store.prices.Load(symbol)
	if !ok {
		return exchange.BBO{}, false
	}
	bbo := val.(bbo)
	return exchange.BBO{Bid: bbo.Bid, Ask: bbo.Ask}, true
}

func (a *Adapter) GetPriceStore() *sync.Map {
	if a.ws == nil {
		return &sync.Map{}
	}
	return &a.ws.store.prices
}

// ==================== WebSocket: Depth ====================

func (a *Adapter) SubscribeDepth(symbol string) bool {
	if a.ws == nil {
		return false
	}
	return a.ws.SubscribeDepth(symbol)
}

func (a *Adapter) UnsubscribeDepth(symbol string) bool {
	if a.ws == nil {
		return false
	}
	return a.ws.UnsubscribeDepth(symbol)
}

func (a *Adapter) GetDepth(symbol string) (*exchange.Orderbook, bool) {
	if a.ws == nil {
		return nil, false
	}
	if !strings.Contains(symbol, "USDT") {
		symbol += "USDT"
	}
	val, ok := a.ws.depthStore.Load(symbol)
	if !ok {
		return nil, false
	}
	return val.(*exchange.Orderbook), true
}

// ==================== WebSocket: Private ====================

func (a *Adapter) StartPrivateStream() {
	a.wsPriv = NewWSPrivateClient(a.apiKey, a.secretKey, a.passphrase, &a.orderCallback)
	a.wsPriv.SetOrderMetricsCallback(a.orderMetricsCallback)
	a.wsPriv.Start()
}

func (a *Adapter) GetOrderUpdate(orderID string) (exchange.OrderUpdate, bool) {
	if a.wsPriv == nil {
		return exchange.OrderUpdate{}, false
	}
	info, ok := a.wsPriv.store.GetOrder(orderID)
	if !ok {
		return exchange.OrderUpdate{}, false
	}
	return exchange.OrderUpdate{
		OrderID:      info.OrderID,
		ClientOID:    info.ClientOID,
		Status:       info.Status,
		FilledVolume: info.FilledVolume,
		AvgPrice:     info.AvgPrice,
	}, true
}

// parseRawFloat parses a JSON value that may be a string or number into float64.
func parseRawFloat(raw json.RawMessage) float64 {
	s := strings.Trim(string(raw), "\"")
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

// Compile-time interface check.
var _ exchange.Exchange = (*Adapter)(nil)
var _ exchange.TradingFeeProvider = (*Adapter)(nil)

// GetUserTrades returns filled trades for a symbol since startTime.
// Bitget endpoint: GET /api/v2/mix/order/fill-history
func (b *Adapter) GetUserTrades(symbol string, startTime time.Time, limit int) ([]exchange.Trade, error) {
	if limit <= 0 || limit > 100 {
		limit = 100 // Bitget max is 100
	}
	params := map[string]string{
		"symbol":      symbol,
		"productType": "USDT-FUTURES",
		"startTime":   strconv.FormatInt(startTime.UnixMilli(), 10),
		"limit":       strconv.Itoa(limit),
	}
	body, err := b.client.Get("/api/v2/mix/order/fills", params)
	if err != nil {
		return nil, fmt.Errorf("GetUserTrades: %w", err)
	}

	var resp struct {
		Code string `json:"code"`
		Data struct {
			FillList []struct {
				TradeID    string `json:"tradeId"`
				OrderID    string `json:"orderId"`
				Symbol     string `json:"symbol"`
				Side       string `json:"side"` // buy or sell
				Price      string `json:"price"`
				BaseVolume string `json:"baseVolume"`
				FeeDetail  []struct {
					FeeCoin  string `json:"feeCoin"`
					TotalFee string `json:"totalFee"`
				} `json:"feeDetail"`
				CTime string `json:"cTime"` // ms timestamp
			} `json:"fillList"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil, fmt.Errorf("GetUserTrades unmarshal: %w", err)
	}

	trades := make([]exchange.Trade, 0, len(resp.Data.FillList))
	for _, t := range resp.Data.FillList {
		price, _ := strconv.ParseFloat(t.Price, 64)
		qty, _ := strconv.ParseFloat(t.BaseVolume, 64)
		var fee float64
		feeCoin := "USDT"
		if len(t.FeeDetail) > 0 {
			fee, _ = strconv.ParseFloat(t.FeeDetail[0].TotalFee, 64)
			if t.FeeDetail[0].FeeCoin != "" {
				feeCoin = t.FeeDetail[0].FeeCoin
			}
		}
		if fee < 0 {
			fee = -fee
		}
		ms, _ := strconv.ParseInt(t.CTime, 10, 64)
		trades = append(trades, exchange.Trade{
			TradeID:  t.TradeID,
			OrderID:  t.OrderID,
			Symbol:   t.Symbol,
			Side:     strings.ToLower(t.Side),
			Price:    price,
			Quantity: qty,
			Fee:      fee,
			FeeCoin:  feeCoin,
			Time:     time.UnixMilli(ms),
		})
	}
	return trades, nil
}

// GetFundingFees returns funding fee history for a symbol since the given time.
func (a *Adapter) GetFundingFees(symbol string, since time.Time) ([]exchange.FundingPayment, error) {
	params := map[string]string{
		"symbol":       symbol,
		"productType":  "USDT-FUTURES",
		"businessType": "contract_settle_fee",
		"startTime":    strconv.FormatInt(since.UnixMilli(), 10),
		"limit":        "100",
	}
	body, err := a.client.Get("/api/v2/mix/account/bill", params)
	if err != nil {
		return nil, fmt.Errorf("GetFundingFees: %w", err)
	}

	var resp struct {
		Code string `json:"code"`
		Data struct {
			Bills []struct {
				Amount string `json:"amount"`
				CTime  string `json:"cTime"`
				Symbol string `json:"symbol"`
			} `json:"bills"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil, fmt.Errorf("GetFundingFees unmarshal: %w", err)
	}

	out := make([]exchange.FundingPayment, 0, len(resp.Data.Bills))
	for _, b := range resp.Data.Bills {
		// Filter: only include bills matching the requested symbol.
		if b.Symbol != "" && b.Symbol != symbol {
			continue
		}
		amt, _ := strconv.ParseFloat(b.Amount, 64)
		ms, _ := strconv.ParseInt(b.CTime, 10, 64)
		out = append(out, exchange.FundingPayment{
			Amount: amt,
			Time:   time.UnixMilli(ms),
		})
	}
	return out, nil
}

// GetClosePnL returns exchange-reported position-level PnL for recently closed positions.
func (a *Adapter) GetClosePnL(symbol string, since time.Time) ([]exchange.ClosePnL, error) {
	params := map[string]string{
		"symbol":      symbol,
		"productType": "USDT-FUTURES",
		"startTime":   strconv.FormatInt(since.UnixMilli(), 10),
		"limit":       "20",
	}
	body, err := a.client.Get("/api/v2/mix/position/history-position", params)
	if err != nil {
		return nil, fmt.Errorf("GetClosePnL: %w", err)
	}

	var resp struct {
		Code string `json:"code"`
		Data struct {
			List []struct {
				NetProfit     string `json:"netProfit"`
				Pnl           string `json:"pnl"`
				TotalFunding  string `json:"totalFunding"`
				OpenFee       string `json:"openFee"`
				CloseFee      string `json:"closeFee"`
				OpenAvgPrice  string `json:"openAvgPrice"`
				CloseAvgPrice string `json:"closeAvgPrice"`
				CloseTotalPos string `json:"closeTotalPos"`
				HoldSide      string `json:"holdSide"`
				UTime         string `json:"utime"`
			} `json:"list"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil, fmt.Errorf("GetClosePnL unmarshal: %w", err)
	}

	out := make([]exchange.ClosePnL, 0, len(resp.Data.List))
	for _, r := range resp.Data.List {
		pricePnL, _ := strconv.ParseFloat(r.Pnl, 64)
		openFee, _ := strconv.ParseFloat(r.OpenFee, 64)
		closeFee, _ := strconv.ParseFloat(r.CloseFee, 64)
		funding, _ := strconv.ParseFloat(r.TotalFunding, 64)
		netProfit, _ := strconv.ParseFloat(r.NetProfit, 64)
		entryPrice, _ := strconv.ParseFloat(r.OpenAvgPrice, 64)
		exitPrice, _ := strconv.ParseFloat(r.CloseAvgPrice, 64)
		closeSize, _ := strconv.ParseFloat(r.CloseTotalPos, 64)
		ms, _ := strconv.ParseInt(r.UTime, 10, 64)

		out = append(out, exchange.ClosePnL{
			PricePnL:   pricePnL,
			Fees:       openFee + closeFee,
			Funding:    funding,
			NetPnL:     netProfit,
			EntryPrice: entryPrice,
			ExitPrice:  exitPrice,
			CloseSize:  closeSize,
			Side:       r.HoldSide, // already "long" or "short"
			CloseTime:  time.UnixMilli(ms),
		})
	}
	return out, nil
}

// PlaceStopLoss places a plan (conditional) order on Bitget.
func (a *Adapter) PlaceStopLoss(params exchange.StopLossParams) (string, error) {
	p := map[string]string{
		"symbol":       params.Symbol,
		"productType":  productTypeUSDTFutures,
		"marginCoin":   marginCoinUSDT,
		"marginMode":   "crossed",
		"planType":     "normal_plan",
		"orderType":    "market",
		"triggerPrice": params.TriggerPrice,
		"triggerType":  "mark_price",
		"size":         params.Size,
		"side":         string(params.Side),
		"reduceOnly":   "YES",
	}

	raw, err := a.client.Post("/api/v2/mix/order/place-plan-order", p)
	if err != nil {
		return "", fmt.Errorf("PlaceStopLoss: %w", err)
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
		return "", fmt.Errorf("PlaceStopLoss unmarshal: %w", err)
	}
	if resp.Code != "00000" {
		return "", fmt.Errorf("PlaceStopLoss failed: code=%s msg=%s", resp.Code, resp.Msg)
	}
	return resp.Data.OrderId, nil
}

// CancelStopLoss cancels a plan (conditional) order on Bitget.
func (a *Adapter) CancelStopLoss(symbol, orderID string) error {
	p := map[string]string{
		"symbol":      symbol,
		"productType": productTypeUSDTFutures,
		"orderId":     orderID,
	}

	raw, err := a.client.Post("/api/v2/mix/order/cancel-plan-order", p)
	if err != nil {
		return fmt.Errorf("CancelStopLoss: %w", err)
	}
	var resp struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return fmt.Errorf("CancelStopLoss unmarshal: %w", err)
	}
	if resp.Code != "00000" && resp.Code != "43011" && resp.Code != "43025" {
		return fmt.Errorf("CancelStopLoss failed: %s %s", resp.Code, resp.Msg)
	}
	return nil
}

// EnsureOneWayMode sets the account to one-way position mode.
// Close terminates all WebSocket connections for graceful shutdown.
func (a *Adapter) Close() {
	if a.ws != nil {
		a.ws.Stop()
	}
	if a.wsPriv != nil {
		a.wsPriv.Stop()
	}
}

func (a *Adapter) EnsureOneWayMode() error {
	params := map[string]string{
		"productType": "USDT-FUTURES",
		"posMode":     "one_way_mode",
	}
	_, err := a.client.Post("/api/v2/mix/account/set-position-mode", params)
	if err != nil {
		errMsg := strings.ToLower(err.Error())
		// Already in one-way mode
		if strings.Contains(errMsg, "already") || strings.Contains(errMsg, "not need") || strings.Contains(errMsg, "40774") {
			return nil
		}
		// Can't change with open positions — verify current mode is already one-way.
		if a.isOneWayMode() {
			return nil
		}
		return fmt.Errorf("EnsureOneWayMode: %w", err)
	}
	return nil
}

// isOneWayMode queries Bitget account config to check if position mode is one-way.
func (a *Adapter) isOneWayMode() bool {
	raw, err := a.client.Get("/api/v2/mix/account/account", map[string]string{
		"productType": "USDT-FUTURES",
		"symbol":      "BTCUSDT",
		"marginCoin":  "USDT",
	})
	if err != nil {
		return false
	}
	var resp struct {
		Data struct {
			PosMode string `json:"posMode"`
		} `json:"data"`
	}
	if json.Unmarshal([]byte(raw), &resp) == nil {
		return resp.Data.PosMode == "one_way_mode"
	}
	return false
}

// GetTradingFee returns maker/taker fee rates from the public contracts endpoint.
func (a *Adapter) GetTradingFee() (*exchange.TradingFee, error) {
	params := map[string]string{
		"symbol":      "BTCUSDT",
		"productType": productTypeUSDTFutures,
	}
	raw, err := a.client.Get("/api/v2/mix/market/contracts", params)
	if err != nil {
		return nil, fmt.Errorf("bitget GetTradingFee: %w", err)
	}

	var resp struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
		Data []struct {
			MakerRate string `json:"makerFeeRate"`
			TakerRate string `json:"takerFeeRate"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return nil, fmt.Errorf("bitget GetTradingFee unmarshal: %w", err)
	}
	if resp.Code != "00000" {
		return nil, fmt.Errorf("bitget GetTradingFee failed: code=%s msg=%s", resp.Code, resp.Msg)
	}
	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("bitget GetTradingFee: empty data")
	}

	maker, err := strconv.ParseFloat(resp.Data[0].MakerRate, 64)
	if err != nil {
		return nil, fmt.Errorf("bitget GetTradingFee parse maker: %w", err)
	}
	taker, err := strconv.ParseFloat(resp.Data[0].TakerRate, 64)
	if err != nil {
		return nil, fmt.Errorf("bitget GetTradingFee parse taker: %w", err)
	}

	return &exchange.TradingFee{
		MakerRate: maker,
		TakerRate: taker,
	}, nil
}
