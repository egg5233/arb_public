package bitget

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
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
	aliases              exchange.SymbolAliasCache
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

// IsUnified returns false — Bitget has no unified account mode.
func (a *Adapter) IsUnified() bool { return false }

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
	// After #1 (doRequest strict errors), bitget client returns *APIError on
	// non-2xx HTTP or logical failure instead of raw body. Branch on error class:
	//   - 40009                       -> PermDenied
	//   - retryableCodes (transient)  -> PermUnknown
	//   - 5xx HTTP status             -> PermUnknown
	//   - other *APIError             -> PermGranted (endpoint reached, rejected for
	//                                    another reason — matches legacy semantic)
	//   - other non-API error         -> PermUnknown (network/transport)
	//   - nil error                   -> PermGranted
	checkBitget := func(_ string, err error) exchange.PermStatus {
		var apiErr *APIError
		if errors.As(err, &apiErr) {
			if apiErr.Code == "40009" {
				return exchange.PermDenied
			}
			if retryableCodes[apiErr.Code] {
				return exchange.PermUnknown
			}
			if n, convErr := strconv.Atoi(apiErr.Code); convErr == nil && n >= 500 && n <= 599 {
				return exchange.PermUnknown
			}
			return exchange.PermGranted
		}
		if err != nil {
			return exchange.PermUnknown
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

func (a *Adapter) resolveSymbol(symbol string) (string, float64, error) {
	if real, mult, hit := a.aliases.ResolveCached(symbol); hit {
		return real, mult, nil
	}
	if err := a.aliases.Ensure(func() error {
		_, err := a.LoadAllContracts()
		return err
	}); err != nil {
		return "", 0, fmt.Errorf("resolveSymbol %s: %w", symbol, err)
	}
	real, mult, _ := a.aliases.ResolveCached(symbol)
	return real, mult, nil
}

func (a *Adapter) canonicalSymbol(symbol string) (string, float64, error) {
	if bare, mult, hit := a.aliases.CanonicalCached(symbol); hit {
		return bare, mult, nil
	}
	if err := a.aliases.Ensure(func() error {
		_, err := a.LoadAllContracts()
		return err
	}); err != nil {
		return "", 0, fmt.Errorf("canonicalSymbol %s: %w", symbol, err)
	}
	bare, mult, _ := a.aliases.CanonicalCached(symbol)
	return bare, mult, nil
}

func (a *Adapter) contractInfo(symbol string) (exchange.ContractInfo, error) {
	contracts, err := a.LoadAllContracts()
	if err != nil {
		return exchange.ContractInfo{}, err
	}
	info, ok := contracts[symbol]
	if !ok {
		return exchange.ContractInfo{}, fmt.Errorf("contract info not found for %s", symbol)
	}
	return info, nil
}

func (a *Adapter) nativeOrderSize(symbol string, sizeBase string, mult float64) (string, error) {
	size, err := strconv.ParseFloat(sizeBase, 64)
	if err != nil {
		return "", err
	}
	info, err := a.contractInfo(symbol)
	if err != nil {
		return "", err
	}
	step := exchange.NativeContractStep(info)
	minSize := exchange.NativeContractMin(info)
	contracts := exchange.ScaleSizeToContracts(size, mult)
	if step > 0 {
		contracts = math.Floor(contracts/step) * step
	}
	if contracts <= 0 || (minSize > 0 && contracts < minSize) {
		return "", exchange.ErrBelowMinSize
	}
	return exchange.FormatFloat(contracts), nil
}

func (a *Adapter) nativeOrderPrice(priceBase string, mult float64) (string, error) {
	if priceBase == "" {
		return "", nil
	}
	price, err := strconv.ParseFloat(priceBase, 64)
	if err != nil {
		return "", err
	}
	return exchange.FormatFloat(exchange.ScalePriceToContracts(price, mult)), nil
}

// ==================== Orders ====================

func (a *Adapter) PlaceOrder(req exchange.PlaceOrderParams) (string, error) {
	log.Printf("[bitget] PlaceOrder: symbol=%s side=%s type=%s size=%s price=%s force=%s reduceOnly=%v",
		req.Symbol, req.Side, req.OrderType, req.Size, req.Price, req.Force, req.ReduceOnly)
	realSymbol, mult, err := a.resolveSymbol(req.Symbol)
	if err != nil {
		return "", fmt.Errorf("PlaceOrder resolve: %w", err)
	}
	size, err := a.nativeOrderSize(req.Symbol, req.Size, mult)
	if err != nil {
		return "", fmt.Errorf("PlaceOrder size: %w", err)
	}
	params := map[string]string{
		"symbol":      realSymbol,
		"productType": productTypeUSDTFutures,
		"marginCoin":  marginCoinUSDT,
		"marginMode":  "crossed",
		"size":        size,
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
		price, err := a.nativeOrderPrice(req.Price, mult)
		if err != nil {
			return "", fmt.Errorf("PlaceOrder price: %w", err)
		}
		params["price"] = price
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
	realSymbol, _, err := a.resolveSymbol(symbol)
	if err != nil {
		return fmt.Errorf("cancel resolve: %w", err)
	}
	params := map[string]string{
		"symbol":      realSymbol,
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
	realSymbol, _, err := a.resolveSymbol(symbol)
	if err != nil {
		return nil, fmt.Errorf("GetPendingOrders resolve: %w", err)
	}
	params := map[string]string{
		"symbol":      realSymbol,
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
		bare, mult, err := a.canonicalSymbol(o.Symbol)
		if err != nil {
			return nil, fmt.Errorf("GetPendingOrders canonical: %w", err)
		}
		price, _ := strconv.ParseFloat(o.Price, 64)
		size, _ := strconv.ParseFloat(o.Size, 64)
		out[i] = exchange.Order{
			OrderID:   o.OrderId,
			ClientOid: o.ClientOid,
			Symbol:    bare,
			Side:      o.Side,
			OrderType: o.OrderType,
			Price:     exchange.FormatFloat(exchange.ScalePriceFromContracts(price, mult)),
			Size:      exchange.FormatFloat(exchange.ScaleSizeFromContracts(size, mult)),
			Status:    o.Status,
		}
	}
	return out, nil
}

func (a *Adapter) GetOrderFilledQty(orderID, symbol string) (float64, error) {
	// /api/v2/mix/order/detail requires symbol in the signed query, which fails
	// bitget HMAC validation for non-ASCII symbols (e.g. 龙虾USDT). Fall back to
	// /api/v2/mix/order/fills with orderId filter (no symbol required).
	if containsNonASCII(symbol) {
		return a.getOrderFilledQtyViaFills(orderID)
	}
	realSymbol, mult, err := a.resolveSymbol(symbol)
	if err != nil {
		return 0, fmt.Errorf("GetOrderFilledQty resolve: %w", err)
	}
	params := map[string]string{
		"symbol":      realSymbol,
		"orderId":     orderID,
		"productType": productTypeUSDTFutures,
	}

	raw, err := a.client.Get("/api/v2/mix/order/detail", params)
	if err != nil {
		return 0, err
	}

	var resp struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			BaseVolume string `json:"baseVolume"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return 0, err
	}
	if resp.Code != "" && resp.Code != "00000" {
		return 0, &APIError{Code: resp.Code, Msg: resp.Msg}
	}
	if resp.Data.BaseVolume == "" {
		return 0, nil
	}
	qty, err := strconv.ParseFloat(resp.Data.BaseVolume, 64)
	if err == nil && qty > 0 && a.orderMetricsCallback != nil {
		a.orderMetricsCallback(exchange.OrderMetricEvent{
			Type:      exchange.OrderMetricFilled,
			OrderID:   orderID,
			FilledQty: exchange.ScaleSizeFromContracts(qty, mult),
			Timestamp: time.Now(),
		})
	}
	return exchange.ScaleSizeFromContracts(qty, mult), err
}

// getOrderFilledQtyViaFills is the non-ASCII fallback for GetOrderFilledQty.
// /api/v2/mix/order/fills accepts orderId without symbol, so the signed query
// stays ASCII-clean even for symbols like 龙虾USDT. Sums baseVolume across all
// fills matching orderID.
func (a *Adapter) getOrderFilledQtyViaFills(orderID string) (float64, error) {
	params := map[string]string{
		"productType": productTypeUSDTFutures,
		"orderId":     orderID,
	}
	raw, err := a.client.Get("/api/v2/mix/order/fills", params)
	if err != nil {
		return 0, fmt.Errorf("GetOrderFilledQty (fills): %w", err)
	}
	var resp struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			FillList []struct {
				OrderID    string `json:"orderId"`
				BaseVolume string `json:"baseVolume"`
			} `json:"fillList"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return 0, err
	}
	if resp.Code != "" && resp.Code != "00000" {
		return 0, &APIError{Code: resp.Code, Msg: resp.Msg}
	}
	var total float64
	for _, f := range resp.Data.FillList {
		if f.OrderID != orderID {
			continue
		}
		v, _ := strconv.ParseFloat(f.BaseVolume, 64)
		total += v
	}
	if total > 0 && a.orderMetricsCallback != nil {
		a.orderMetricsCallback(exchange.OrderMetricEvent{
			Type:      exchange.OrderMetricFilled,
			OrderID:   orderID,
			FilledQty: total,
			Timestamp: time.Now(),
		})
	}
	return total, nil
}

// ==================== Positions ====================

func (a *Adapter) GetPosition(symbol string) ([]exchange.Position, error) {
	// /api/v2/mix/position/single-position requires symbol in the signed query,
	// which fails bitget HMAC validation for non-ASCII symbols. Fall back to
	// GetAllPositions (symbol-less endpoint) and filter locally.
	if containsNonASCII(symbol) {
		all, err := a.GetAllPositions()
		if err != nil {
			return nil, err
		}
		filtered := make([]exchange.Position, 0, len(all))
		for _, p := range all {
			if strings.EqualFold(p.Symbol, symbol) {
				filtered = append(filtered, p)
			}
		}
		return filtered, nil
	}
	realSymbol, _, err := a.resolveSymbol(symbol)
	if err != nil {
		return nil, fmt.Errorf("GetPosition resolve: %w", err)
	}
	params := map[string]string{
		"symbol":      realSymbol,
		"marginCoin":  marginCoinUSDT,
		"productType": productTypeUSDTFutures,
	}

	raw, err := a.client.Get("/api/v2/mix/position/single-position", params)
	if err != nil {
		return nil, err
	}

	return a.parsePositions(raw)
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

	return a.parsePositions(raw)
}

func (a *Adapter) parsePositions(raw string) ([]exchange.Position, error) {
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
		bare, mult, err := a.canonicalSymbol(p.Symbol)
		if err != nil {
			return nil, fmt.Errorf("parsePositions canonical: %w", err)
		}
		available, _ := strconv.ParseFloat(p.Available, 64)
		openPrice, _ := strconv.ParseFloat(p.AverageOpenPrice, 64)
		liqPrice, _ := strconv.ParseFloat(p.LiquidationPrice, 64)
		markPrice, _ := strconv.ParseFloat(p.MarkPrice, 64)
		out = append(out, exchange.Position{
			Symbol:           bare,
			HoldSide:         p.HoldSide,
			Total:            exchange.FormatFloat(exchange.ScaleSizeFromContracts(total, mult)),
			Available:        exchange.FormatFloat(exchange.ScaleSizeFromContracts(available, mult)),
			AverageOpenPrice: exchange.FormatFloat(exchange.ScalePriceFromContracts(openPrice, mult)),
			UnrealizedPL:     p.UnrealizedPL,
			Leverage:         p.Leverage,
			MarginMode:       p.MarginMode,
			LiquidationPrice: exchange.FormatFloat(exchange.ScalePriceFromContracts(liqPrice, mult)),
			MarkPrice:        exchange.FormatFloat(exchange.ScalePriceFromContracts(markPrice, mult)),
			FundingFee:       p.TotalFee,
		})
	}
	return out, nil
}

// ==================== Account Config ====================

func (a *Adapter) SetLeverage(symbol string, leverage string, holdSide string) error {
	realSymbol, _, err := a.resolveSymbol(symbol)
	if err != nil {
		return fmt.Errorf("SetLeverage resolve: %w", err)
	}
	params := map[string]string{
		"symbol":      realSymbol,
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
	realSymbol, _, err := a.resolveSymbol(symbol)
	if err != nil {
		return fmt.Errorf("SetMarginMode resolve: %w", err)
	}
	// Bitget API expects "crossed" not "cross" for cross margin mode.
	apiMode := mode
	if strings.ToLower(mode) == "cross" {
		apiMode = "crossed"
	}

	params := map[string]string{
		"symbol":      realSymbol,
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
			BaseCoin       string `json:"baseCoin"`
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
	aliasMap := make(map[string]string, len(resp.Data))
	reverseMap := make(map[string]string, len(resp.Data))
	multiplierMap := make(map[string]float64, len(resp.Data))
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

		bareBase, mult := exchange.DetectPrefixMultiplier(c.BaseCoin)
		if bareBase == "" {
			bareBase = strings.TrimSuffix(c.Symbol, "USDT")
		}
		bareSymbol := bareBase + "USDT"
		info := exchange.ContractInfo{
			Symbol:        bareSymbol,
			MinSize:       exchange.ScaleSizeFromContracts(minSize, mult),
			StepSize:      exchange.ScaleSizeFromContracts(stepSize, mult),
			MaxSize:       exchange.ScaleSizeFromContracts(maxSize, mult),
			SizeDecimals:  countDecimalsFloat(exchange.ScaleSizeFromContracts(stepSize, mult)),
			PriceStep:     exchange.ScalePriceFromContracts(priceStep, mult),
			PriceDecimals: countDecimalsFloat(exchange.ScalePriceFromContracts(priceStep, mult)),
			Multiplier:    exchange.NormalizeMultiplier(mult),
		}
		result[bareSymbol] = info
		multiplierMap[bareSymbol] = exchange.NormalizeMultiplier(mult)
		if mult > 1 {
			aliasMap[bareSymbol] = c.Symbol
			reverseMap[c.Symbol] = bareSymbol
		}
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("no contract settings loaded")
	}
	a.aliases.Replace(aliasMap, reverseMap, multiplierMap)
	return result, nil
}

// ==================== Maintenance Rate ====================

// GetMaintenanceRate returns the maintenance margin rate for a symbol at a given
// notional size by querying the query-position-lever endpoint.
// Bitget keepMarginRate is already decimal string ("0.004" = 0.4%). Parse directly.
func (a *Adapter) GetMaintenanceRate(symbol string, notionalUSDT float64) (float64, error) {
	params := map[string]string{
		"symbol":      symbol,
		"productType": productTypeUSDTFutures,
	}

	raw, err := a.client.Get("/api/v2/mix/market/query-position-lever", params)
	if err != nil {
		return 0, fmt.Errorf("GetMaintenanceRate: %w", err)
	}

	var resp struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
		Data []struct {
			Level          string `json:"level"`
			StartUnit      string `json:"startUnit"`
			EndUnit        string `json:"endUnit"`
			Leverage       string `json:"leverage"`
			KeepMarginRate string `json:"keepMarginRate"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return 0, fmt.Errorf("GetMaintenanceRate unmarshal: %w", err)
	}
	if resp.Code != "00000" {
		return 0, fmt.Errorf("GetMaintenanceRate: code=%s msg=%s", resp.Code, resp.Msg)
	}

	if len(resp.Data) == 0 {
		return 0, fmt.Errorf("GetMaintenanceRate: no tiers for %s", symbol)
	}

	// For notional=0, return the first (lowest) tier
	if notionalUSDT <= 0 {
		rate, _ := strconv.ParseFloat(resp.Data[0].KeepMarginRate, 64)
		if rate <= 0 || rate >= 1.0 {
			return 0, nil
		}
		return rate, nil
	}

	// Match tier where startUnit <= notionalUSDT <= endUnit
	for _, tier := range resp.Data {
		start, _ := strconv.ParseFloat(tier.StartUnit, 64)
		end, _ := strconv.ParseFloat(tier.EndUnit, 64)
		if notionalUSDT >= start && notionalUSDT <= end {
			rate, _ := strconv.ParseFloat(tier.KeepMarginRate, 64)
			if rate <= 0 || rate >= 1.0 {
				return 0, nil
			}
			return rate, nil
		}
	}

	// Exceeds all tiers: return last tier's rate
	last := resp.Data[len(resp.Data)-1]
	rate, _ := strconv.ParseFloat(last.KeepMarginRate, 64)
	if rate <= 0 || rate >= 1.0 {
		return 0, nil
	}
	return rate, nil
}

// ==================== Funding Rate ====================

func (a *Adapter) GetFundingRate(symbol string) (*exchange.FundingRate, error) {
	noFilter := containsNonASCII(symbol)
	params := map[string]string{
		"productType": productTypeUSDTFutures,
	}
	if !noFilter {
		realSymbol, _, err := a.resolveSymbol(symbol)
		if err != nil {
			return nil, fmt.Errorf("GetFundingRate resolve: %w", err)
		}
		params["symbol"] = realSymbol
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
	if resp.Code != "" && resp.Code != "00000" {
		return nil, &APIError{Code: resp.Code, Msg: resp.Msg}
	}
	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("GetFundingRate: no data for %s", symbol)
	}

	idx := 0
	if noFilter {
		idx = -1
		for i := range resp.Data {
			if strings.EqualFold(resp.Data[i].Symbol, symbol) {
				idx = i
				break
			}
		}
		if idx < 0 {
			return nil, fmt.Errorf("GetFundingRate: symbol %s not found in no-filter response", symbol)
		}
	}

	d := resp.Data[idx]
	rate, err := strconv.ParseFloat(d.FundingRate, 64)
	if err != nil {
		return nil, fmt.Errorf("GetFundingRate parse %q: %w", d.FundingRate, err)
	}
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
	} else if !noFilter {
		interval, err = a.GetFundingInterval(symbol)
		if err != nil || interval <= 0 {
			interval = 8 * time.Hour
		}
	} else {
		// Skip GetFundingInterval for non-ASCII symbols; that endpoint likely
		// has the same HMAC issue. Use default 8h interval instead.
		interval = 8 * time.Hour
	}

	fr := &exchange.FundingRate{
		Symbol:      symbol,
		Rate:        rate,
		NextRate:    nextRate,
		Interval:    interval,
		NextFunding: nextFunding,
	}
	if fr.Symbol == "" {
		fr.Symbol = symbol
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
	realSymbol, _, err := a.resolveSymbol(symbol)
	if err != nil {
		return 8 * time.Hour, nil
	}
	params := map[string]string{
		"symbol":      realSymbol,
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
			CrossedMaxAvail string `json:"crossedMaxAvailable"`
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
	crossedMaxAvail, _ := strconv.ParseFloat(resp.Data.CrossedMaxAvail, 64)
	frozen, _ := strconv.ParseFloat(resp.Data.Frozen, 64)
	marginRatio, _ := strconv.ParseFloat(resp.Data.CrossedRiskRate, 64)

	// Use crossedMaxAvailable (actual openable amount in cross margin mode)
	// instead of generic available, which may not account for margin tiers.
	// Distinguish "field present with value 0" (real: no margin left) from
	// "field absent/empty" (API didn't return it, fall back to available).
	if resp.Data.CrossedMaxAvail != "" {
		avail = crossedMaxAvail // trust the value even if 0
	} else if avail <= 0 && total > 0 {
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
	realSymbol, mult, err := a.resolveSymbol(symbol)
	if err != nil {
		return nil, fmt.Errorf("GetOrderbook resolve: %w", err)
	}

	params := map[string]string{
		"symbol":      realSymbol,
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
		ob.Asks = append(ob.Asks, exchange.PriceLevel{
			Price:    exchange.ScalePriceFromContracts(p, mult),
			Quantity: exchange.ScaleSizeFromContracts(q, mult),
		})
	}

	ob.Bids = make([]exchange.PriceLevel, 0, len(resp.Data.Bids))
	for _, bid := range resp.Data.Bids {
		if len(bid) < 2 {
			continue
		}
		p := parseRawFloat(bid[0])
		q := parseRawFloat(bid[1])
		ob.Bids = append(ob.Bids, exchange.PriceLevel{
			Price:    exchange.ScalePriceFromContracts(p, mult),
			Quantity: exchange.ScaleSizeFromContracts(q, mult),
		})
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
func (a *Adapter) GetWithdrawFee(coin, chain string) (fee float64, minWithdraw float64, err error) {
	network := mapChainToBitgetNetwork(chain)
	params := map[string]string{
		"coin": coin,
	}
	raw, apiErr := a.client.Get("/api/v2/spot/public/coins", params)
	if apiErr != nil {
		return 0, 0, fmt.Errorf("bitget GetWithdrawFee: %w", apiErr)
	}

	var resp struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
		Data []struct {
			Coin   string `json:"coin"`
			Chains []struct {
				Chain             string `json:"chain"`
				WithdrawFee       string `json:"withdrawFee"`
				ExtraWithdrawFee  string `json:"extraWithdrawFee"`
				MinWithdrawAmount string `json:"minWithdrawAmount"`
			} `json:"chains"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return 0, 0, fmt.Errorf("bitget GetWithdrawFee unmarshal: %w", err)
	}
	if resp.Code != "00000" {
		return 0, 0, fmt.Errorf("bitget GetWithdrawFee API error: code=%s msg=%s", resp.Code, resp.Msg)
	}

	for _, c := range resp.Data {
		if !strings.EqualFold(c.Coin, coin) {
			continue
		}
		for _, ch := range c.Chains {
			if strings.EqualFold(ch.Chain, network) {
				parsedFee, err := strconv.ParseFloat(ch.WithdrawFee, 64)
				if err != nil {
					return 0, 0, fmt.Errorf("bitget GetWithdrawFee parse fee: %w", err)
				}
				extra, _ := strconv.ParseFloat(ch.ExtraWithdrawFee, 64)
				minWd, _ := strconv.ParseFloat(ch.MinWithdrawAmount, 64)
				return parsedFee + extra, minWd, nil
			}
		}
		return 0, 0, fmt.Errorf("bitget GetWithdrawFee: chain %s not found for %s", network, coin)
	}
	return 0, 0, fmt.Errorf("bitget GetWithdrawFee: coin %s not found", coin)
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
	resolved := make([]string, 0, len(symbols))
	for _, symbol := range symbols {
		real, _, err := a.resolveSymbol(symbol)
		if err != nil {
			continue
		}
		resolved = append(resolved, real)
	}
	a.ws.Start(resolved)
}

func (a *Adapter) SubscribeSymbol(symbol string) bool {
	if a.ws == nil {
		return false
	}
	real, _, err := a.resolveSymbol(symbol)
	if err != nil {
		return false
	}
	return a.ws.SubscribeSymbol(real)
}

func (a *Adapter) GetBBO(symbol string) (exchange.BBO, bool) {
	if a.ws == nil {
		return exchange.BBO{}, false
	}
	real, mult, err := a.resolveSymbol(symbol)
	if err != nil {
		return exchange.BBO{}, false
	}
	val, ok := a.ws.store.prices.Load(real)
	if !ok {
		return exchange.BBO{}, false
	}
	bbo := val.(bbo)
	return exchange.BBO{
		Bid: exchange.ScalePriceFromContracts(bbo.Bid, mult),
		Ask: exchange.ScalePriceFromContracts(bbo.Ask, mult),
	}, true
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
	real, _, err := a.resolveSymbol(symbol)
	if err != nil {
		return false
	}
	return a.ws.SubscribeDepth(real)
}

func (a *Adapter) UnsubscribeDepth(symbol string) bool {
	if a.ws == nil {
		return false
	}
	real, _, err := a.resolveSymbol(symbol)
	if err != nil {
		return false
	}
	return a.ws.UnsubscribeDepth(real)
}

func (a *Adapter) GetDepth(symbol string) (*exchange.Orderbook, bool) {
	if a.ws == nil {
		return nil, false
	}
	real, mult, err := a.resolveSymbol(symbol)
	if err != nil {
		return nil, false
	}
	val, ok := a.ws.depthStore.Load(real)
	if !ok {
		return nil, false
	}
	ob := val.(*exchange.Orderbook)
	clone := &exchange.Orderbook{
		Symbol: symbol,
		Time:   ob.Time,
		Bids:   make([]exchange.PriceLevel, len(ob.Bids)),
		Asks:   make([]exchange.PriceLevel, len(ob.Asks)),
	}
	for i, level := range ob.Bids {
		clone.Bids[i] = exchange.PriceLevel{
			Price:    exchange.ScalePriceFromContracts(level.Price, mult),
			Quantity: exchange.ScaleSizeFromContracts(level.Quantity, mult),
		}
	}
	for i, level := range ob.Asks {
		clone.Asks[i] = exchange.PriceLevel{
			Price:    exchange.ScalePriceFromContracts(level.Price, mult),
			Quantity: exchange.ScaleSizeFromContracts(level.Quantity, mult),
		}
	}
	return clone, true
}

// ==================== WebSocket: Private ====================

func (a *Adapter) StartPrivateStream() {
	a.wsPriv = NewWSPrivateClient(a.apiKey, a.secretKey, a.passphrase, &a.orderCallback, func(symbol string, qty float64, price float64) (string, float64, float64) {
		bare, mult, err := a.canonicalSymbol(symbol)
		if err != nil {
			return symbol, qty, price
		}
		return bare, exchange.ScaleSizeFromContracts(qty, mult), exchange.ScalePriceFromContracts(price, mult)
	})
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
		Symbol:       info.Symbol,
	}, true
}

// parseRawFloat parses a JSON value that may be a string or number into float64.
func parseRawFloat(raw json.RawMessage) float64 {
	s := strings.Trim(string(raw), "\"")
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

func countDecimalsFloat(v float64) int {
	return countDecimals(exchange.FormatFloat(v))
}

func countDecimals(s string) int {
	idx := strings.IndexByte(s, '.')
	if idx < 0 {
		return 0
	}
	d := strings.TrimRight(s[idx+1:], "0")
	return len(d)
}

// Compile-time interface check.
var _ exchange.Exchange = (*Adapter)(nil)
var _ exchange.TradingFeeProvider = (*Adapter)(nil)

// GetUserTrades returns filled trades for a symbol since startTime.
// Bitget endpoint: GET /api/v2/mix/order/fills (symbol optional per docs).
// For non-ASCII symbols the signed query omits `symbol` and filters locally
// after response.
func (b *Adapter) GetUserTrades(symbol string, startTime time.Time, limit int) ([]exchange.Trade, error) {
	if limit <= 0 || limit > 100 {
		limit = 100 // Bitget max is 100
	}
	nonASCII := containsNonASCII(symbol)
	params := map[string]string{
		"productType": "USDT-FUTURES",
		"startTime":   strconv.FormatInt(startTime.UnixMilli(), 10),
		"endTime":     strconv.FormatInt(time.Now().UnixMilli(), 10),
		"limit":       strconv.Itoa(limit),
	}
	if !nonASCII {
		realSymbol, _, err := b.resolveSymbol(symbol)
		if err != nil {
			return nil, fmt.Errorf("GetUserTrades resolve: %w", err)
		}
		params["symbol"] = realSymbol
	}
	body, err := b.client.Get("/api/v2/mix/order/fills", params)
	if err != nil {
		return nil, fmt.Errorf("GetUserTrades: %w", err)
	}

	var resp struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
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
	if resp.Code != "" && resp.Code != "00000" {
		return nil, &APIError{Code: resp.Code, Msg: resp.Msg}
	}

	trades := make([]exchange.Trade, 0, len(resp.Data.FillList))
	for _, t := range resp.Data.FillList {
		if nonASCII && !strings.EqualFold(t.Symbol, symbol) {
			continue
		}
		price, _ := strconv.ParseFloat(t.Price, 64)
		qty, _ := strconv.ParseFloat(t.BaseVolume, 64)
		bare, mult, err := b.canonicalSymbol(t.Symbol)
		if err != nil {
			return nil, fmt.Errorf("GetUserTrades canonical: %w", err)
		}
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
			Symbol:   bare,
			Side:     strings.ToLower(t.Side),
			Price:    exchange.ScalePriceFromContracts(price, mult),
			Quantity: exchange.ScaleSizeFromContracts(qty, mult),
			Fee:      fee,
			FeeCoin:  feeCoin,
			Time:     time.UnixMilli(ms),
		})
	}
	return trades, nil
}

// GetFundingFees returns funding fee history for a symbol since the given time.
// For non-ASCII symbols, falls back to the no-filter /account/bill endpoint and
// paginates via the endId cursor (symbol in signed query would fail bitget HMAC).
func (a *Adapter) GetFundingFees(symbol string, since time.Time) ([]exchange.FundingPayment, error) {
	if containsNonASCII(symbol) {
		return a.getFundingFeesViaNoFilter(symbol, since)
	}
	realSymbol, _, err := a.resolveSymbol(symbol)
	if err != nil {
		return nil, fmt.Errorf("GetFundingFees resolve: %w", err)
	}
	params := map[string]string{
		"symbol":       realSymbol,
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
		Msg  string `json:"msg"`
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
	if resp.Code != "" && resp.Code != "00000" {
		return nil, &APIError{Code: resp.Code, Msg: resp.Msg}
	}

	out := make([]exchange.FundingPayment, 0, len(resp.Data.Bills))
	for _, b := range resp.Data.Bills {
		// Filter: only include bills matching the requested symbol.
		if b.Symbol != "" && b.Symbol != realSymbol {
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

// getFundingFeesViaNoFilter is the non-ASCII fallback for GetFundingFees.
// Pages through /api/v2/mix/account/bill with the endId cursor because the
// 100-row page limit would otherwise lose data on busy accounts.
func (a *Adapter) getFundingFeesViaNoFilter(symbol string, since time.Time) ([]exchange.FundingPayment, error) {
	out := make([]exchange.FundingPayment, 0)
	var idLessThan string
	endTime := time.Now().UnixMilli()
	startTime := since.UnixMilli()
	for {
		params := map[string]string{
			"productType":  "USDT-FUTURES",
			"businessType": "contract_settle_fee",
			"startTime":    strconv.FormatInt(startTime, 10),
			"endTime":      strconv.FormatInt(endTime, 10),
			"limit":        "100",
		}
		if idLessThan != "" {
			params["idLessThan"] = idLessThan
		}
		body, err := a.client.Get("/api/v2/mix/account/bill", params)
		if err != nil {
			return nil, fmt.Errorf("GetFundingFees (no-filter page): %w", err)
		}
		var resp struct {
			Code string `json:"code"`
			Msg  string `json:"msg"`
			Data struct {
				EndID string `json:"endId"`
				Bills []struct {
					BillID string `json:"billId"`
					Amount string `json:"amount"`
					CTime  string `json:"cTime"`
					Symbol string `json:"symbol"`
				} `json:"bills"`
			} `json:"data"`
		}
		if err := json.Unmarshal([]byte(body), &resp); err != nil {
			return nil, fmt.Errorf("GetFundingFees (no-filter) unmarshal: %w", err)
		}
		if resp.Code != "" && resp.Code != "00000" {
			return nil, &APIError{Code: resp.Code, Msg: resp.Msg}
		}
		if len(resp.Data.Bills) == 0 {
			break
		}
		var oldestCTime int64
		for _, b := range resp.Data.Bills {
			if !strings.EqualFold(b.Symbol, symbol) {
				if ct, _ := strconv.ParseInt(b.CTime, 10, 64); ct < oldestCTime || oldestCTime == 0 {
					oldestCTime = ct
				}
				continue
			}
			amt, _ := strconv.ParseFloat(b.Amount, 64)
			ms, _ := strconv.ParseInt(b.CTime, 10, 64)
			out = append(out, exchange.FundingPayment{Amount: amt, Time: time.UnixMilli(ms)})
			if ms < oldestCTime || oldestCTime == 0 {
				oldestCTime = ms
			}
		}
		// Stop paginating once oldest returned bill is older than our since window.
		if oldestCTime != 0 && oldestCTime < startTime {
			break
		}
		if len(resp.Data.Bills) < 100 {
			break
		}
		// Per bitget docs, use data.endId as next-page cursor. Fall back to last
		// row's billId if endId is empty. Detect cursor deadlock.
		nextCursor := strings.TrimSpace(resp.Data.EndID)
		if nextCursor == "" {
			nextCursor = strings.TrimSpace(resp.Data.Bills[len(resp.Data.Bills)-1].BillID)
		}
		if nextCursor == "" || nextCursor == idLessThan {
			return nil, fmt.Errorf("GetFundingFees no-filter pagination stalled: cursor=%q rows=%d", idLessThan, len(resp.Data.Bills))
		}
		idLessThan = nextCursor
	}
	return out, nil
}

// GetClosePnL returns exchange-reported position-level PnL for recently closed positions.
// Bitget /api/v2/mix/position/history-position accepts symbol optionally; for
// non-ASCII symbols the signed query omits `symbol` and filters locally.
func (a *Adapter) GetClosePnL(symbol string, since time.Time) ([]exchange.ClosePnL, error) {
	nonASCII := containsNonASCII(symbol)
	mult := 1.0
	params := map[string]string{
		"productType": "USDT-FUTURES",
		"startTime":   strconv.FormatInt(since.UnixMilli(), 10),
		"limit":       "20",
	}
	if !nonASCII {
		realSymbol, resolvedMult, err := a.resolveSymbol(symbol)
		if err != nil {
			return nil, fmt.Errorf("GetClosePnL resolve: %w", err)
		}
		params["symbol"] = realSymbol
		mult = resolvedMult
	}
	body, err := a.client.Get("/api/v2/mix/position/history-position", params)
	if err != nil {
		return nil, fmt.Errorf("GetClosePnL: %w", err)
	}

	var resp struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			List []struct {
				Symbol        string `json:"symbol"`
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
	if resp.Code != "" && resp.Code != "00000" {
		return nil, &APIError{Code: resp.Code, Msg: resp.Msg}
	}

	out := make([]exchange.ClosePnL, 0, len(resp.Data.List))
	for _, r := range resp.Data.List {
		if nonASCII && !strings.EqualFold(r.Symbol, symbol) {
			continue
		}
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
			EntryPrice: exchange.ScalePriceFromContracts(entryPrice, mult),
			ExitPrice:  exchange.ScalePriceFromContracts(exitPrice, mult),
			CloseSize:  exchange.ScaleSizeFromContracts(closeSize, mult),
			Side:       r.HoldSide, // already "long" or "short"
			CloseTime:  time.UnixMilli(ms),
		})
	}
	return out, nil
}

// PlaceStopLoss places a plan (conditional) order on Bitget.
func (a *Adapter) PlaceStopLoss(params exchange.StopLossParams) (string, error) {
	realSymbol, mult, err := a.resolveSymbol(params.Symbol)
	if err != nil {
		return "", fmt.Errorf("PlaceStopLoss resolve: %w", err)
	}
	size, err := a.nativeOrderSize(params.Symbol, params.Size, mult)
	if err != nil {
		return "", fmt.Errorf("PlaceStopLoss size: %w", err)
	}
	triggerPrice, err := a.nativeOrderPrice(params.TriggerPrice, mult)
	if err != nil {
		return "", fmt.Errorf("PlaceStopLoss trigger: %w", err)
	}
	p := map[string]string{
		"symbol":       realSymbol,
		"productType":  productTypeUSDTFutures,
		"marginCoin":   marginCoinUSDT,
		"marginMode":   "crossed",
		"planType":     "normal_plan",
		"orderType":    "market",
		"triggerPrice": triggerPrice,
		"triggerType":  "mark_price",
		"size":         size,
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

// PlaceTakeProfit places a take-profit plan order on Bitget futures.
func (a *Adapter) PlaceTakeProfit(params exchange.TakeProfitParams) (string, error) {
	realSymbol, mult, err := a.resolveSymbol(params.Symbol)
	if err != nil {
		return "", fmt.Errorf("PlaceTakeProfit resolve: %w", err)
	}
	size, err := a.nativeOrderSize(params.Symbol, params.Size, mult)
	if err != nil {
		return "", fmt.Errorf("PlaceTakeProfit size: %w", err)
	}
	triggerPrice, err := a.nativeOrderPrice(params.TriggerPrice, mult)
	if err != nil {
		return "", fmt.Errorf("PlaceTakeProfit trigger: %w", err)
	}
	p := map[string]string{
		"symbol":       realSymbol,
		"productType":  productTypeUSDTFutures,
		"marginCoin":   marginCoinUSDT,
		"marginMode":   "crossed",
		"planType":     "normal_plan",
		"orderType":    "market",
		"triggerPrice": triggerPrice,
		"triggerType":  "mark_price",
		"size":         size,
		"side":         string(params.Side),
		"reduceOnly":   "YES",
	}

	raw, err := a.client.Post("/api/v2/mix/order/place-plan-order", p)
	if err != nil {
		return "", fmt.Errorf("PlaceTakeProfit: %w", err)
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
		return "", fmt.Errorf("PlaceTakeProfit unmarshal: %w", err)
	}
	if resp.Code != "00000" {
		return "", fmt.Errorf("PlaceTakeProfit failed: code=%s msg=%s", resp.Code, resp.Msg)
	}
	return resp.Data.OrderId, nil
}

// CancelTakeProfit cancels a plan (take-profit) order on Bitget.
func (a *Adapter) CancelTakeProfit(symbol, orderID string) error {
	return a.CancelStopLoss(symbol, orderID)
}

// CancelStopLoss cancels a plan (conditional) order on Bitget.
func (a *Adapter) CancelStopLoss(symbol, orderID string) error {
	realSymbol, _, err := a.resolveSymbol(symbol)
	if err != nil {
		return fmt.Errorf("CancelStopLoss resolve: %w", err)
	}
	p := map[string]string{
		"symbol":      realSymbol,
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

// CancelAllOrders cancels all open orders (regular + conditional/algo) for a symbol.
// Surfaces both cancel errors joined via errors.Join so callers can log which
// subset failed; previous HEAD silently discarded both errors.
func (a *Adapter) CancelAllOrders(symbol string) error {
	realSymbol, _, err := a.resolveSymbol(symbol)
	if err != nil {
		return fmt.Errorf("CancelAllOrders resolve: %w", err)
	}
	var errs []error
	if _, err := a.client.Post("/api/v2/mix/order/cancel-plan-order", map[string]string{
		"symbol": realSymbol, "productType": "USDT-FUTURES",
	}); err != nil {
		errs = append(errs, fmt.Errorf("cancel plan orders: %w", err))
	}
	if _, err := a.client.Post("/api/v2/mix/order/batch-cancel-orders", map[string]string{
		"symbol": realSymbol, "productType": "USDT-FUTURES",
	}); err != nil {
		errs = append(errs, fmt.Errorf("batch cancel orders: %w", err))
	}
	return errors.Join(errs...)
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
