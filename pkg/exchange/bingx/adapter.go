package bingx

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"arb/pkg/exchange"
	"arb/pkg/utils"
)

var log = utils.NewLogger("bingx")

// Adapter implements the exchange.Exchange interface for BingX.
type Adapter struct {
	client               *Client
	cfg                  exchange.ExchangeConfig
	priceStore           sync.Map // symbol -> exchange.BBO
	depthStore           sync.Map // symbol -> *exchange.Orderbook
	orderStore           sync.Map // orderID -> exchange.OrderUpdate
	orderCallback        func(exchange.OrderUpdate)
	publicWS             *PublicWS
	privateWS            *PrivateWS
	wsMetricsCallback    exchange.WSMetricsCallback
	orderMetricsCallback exchange.OrderMetricsCallback

	// Funding rate batch cache
	fundingRateCache     map[string]*exchange.FundingRate
	fundingRateCacheMu   sync.Mutex
	fundingRateCacheTime time.Time

	// Funding fees batch cache — one API call returns all symbols
	fundingFeesCache      map[string][]exchange.FundingPayment // keyed by internal symbol
	fundingFeesCacheMu    sync.Mutex
	fundingFeesCacheTime  time.Time
	fundingFeesCacheSince time.Time // the 'since' param used for this cache

}

func (a *Adapter) SetMetricsCallback(fn exchange.MetricsCallback) {
	if a.client != nil {
		a.client.SetMetricsCallback(fn)
	}
}

func (a *Adapter) SetWSMetricsCallback(fn exchange.WSMetricsCallback) {
	a.wsMetricsCallback = fn
	if a.publicWS != nil {
		a.publicWS.SetMetricsCallback(fn)
	}
}

func (a *Adapter) SetOrderMetricsCallback(fn exchange.OrderMetricsCallback) {
	a.orderMetricsCallback = fn
	if a.privateWS != nil {
		a.privateWS.SetOrderMetricsCallback(fn)
	}
}

func (a *Adapter) SetOrderCallback(fn func(exchange.OrderUpdate)) {
	a.orderCallback = fn
}

func (a *Adapter) CheckPermissions() exchange.PermissionResult {
	// Use DoRequestRaw — this endpoint doesn't follow the standard {code,data} wrapper.
	data, err := a.client.DoRequestRaw("GET", "/openApi/v1/account/apiRestrictions", map[string]string{})
	if err != nil {
		return exchange.PermissionResult{Method: "direct", Error: err.Error(),
			Read: exchange.PermUnknown, FuturesTrade: exchange.PermUnknown,
			Withdraw: exchange.PermUnknown, Transfer: exchange.PermUnknown}
	}
	// Check for error envelope first ({"code":N,...})
	var errCheck struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if json.Unmarshal(data, &errCheck) == nil && errCheck.Code != 0 {
		return exchange.PermissionResult{Method: "direct", Error: errCheck.Msg,
			Read: exchange.PermUnknown, FuturesTrade: exchange.PermUnknown,
			Withdraw: exchange.PermUnknown, Transfer: exchange.PermUnknown}
	}
	var resp struct {
		EnableReading            bool `json:"enableReading"`
		EnableFutures            bool `json:"enableFutures"`
		PermitsUniversalTransfer bool `json:"permitsUniversalTransfer"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return exchange.PermissionResult{Method: "direct", Error: err.Error(),
			Read: exchange.PermUnknown, FuturesTrade: exchange.PermUnknown,
			Withdraw: exchange.PermUnknown, Transfer: exchange.PermUnknown}
	}
	toBool := func(v bool) exchange.PermStatus {
		if v {
			return exchange.PermGranted
		}
		return exchange.PermDenied
	}
	return exchange.PermissionResult{
		Read: toBool(resp.EnableReading), FuturesTrade: toBool(resp.EnableFutures),
		Withdraw: exchange.PermUnknown, Transfer: toBool(resp.PermitsUniversalTransfer),
		Method: "direct",
	}
}

// NewAdapter creates a new BingX exchange adapter.
func NewAdapter(cfg exchange.ExchangeConfig) *Adapter {
	return &Adapter{
		client: NewClient(cfg.ApiKey, cfg.SecretKey),
		cfg:    cfg,
	}
}

// Client returns the underlying REST client.
func (a *Adapter) Client() *Client { return a.client }

// Name returns the exchange name.
func (a *Adapter) Name() string {
	return "bingx"
}

// ---------- Symbol conversion ----------
// Internal: BTCUSDT -> BingX: BTC-USDT

func toBingXSymbol(symbol string) string {
	// BTCUSDT -> BTC-USDT
	if strings.HasSuffix(symbol, "USDT") && !strings.Contains(symbol, "-") {
		base := strings.TrimSuffix(symbol, "USDT")
		return base + "-USDT"
	}
	return symbol
}

func fromBingXSymbol(symbol string) string {
	// BTC-USDT -> BTCUSDT
	return strings.ReplaceAll(symbol, "-", "")
}

// ---------- Side helpers ----------

func toBingXSide(side exchange.Side) string {
	switch side {
	case exchange.SideBuy:
		return "BUY"
	case exchange.SideSell:
		return "SELL"
	default:
		return strings.ToUpper(string(side))
	}
}

func fromBingXSide(side string) string {
	return strings.ToLower(side)
}

// ---------- Time-in-force mapping ----------

func toBingXTIF(force string) string {
	switch strings.ToLower(force) {
	case "gtc":
		return "GTC"
	case "ioc":
		return "IOC"
	case "fok":
		return "FOK"
	case "post_only":
		return "PostOnly"
	default:
		return "GTC"
	}
}

// ---------- Order type mapping ----------

func toBingXOrderType(orderType string) string {
	switch strings.ToLower(orderType) {
	case "limit":
		return "LIMIT"
	case "market":
		return "MARKET"
	default:
		return strings.ToUpper(orderType)
	}
}

// ---------- Orders ----------

// PlaceOrder places a new order on BingX.
func (a *Adapter) PlaceOrder(req exchange.PlaceOrderParams) (string, error) {
	log.Info("PlaceOrder: symbol=%s side=%s type=%s size=%s price=%s force=%s reduceOnly=%v",
		req.Symbol, req.Side, req.OrderType, req.Size, req.Price, req.Force, req.ReduceOnly)
	params := map[string]string{
		"symbol":   toBingXSymbol(req.Symbol),
		"type":     toBingXOrderType(req.OrderType),
		"side":     toBingXSide(req.Side),
		"quantity": req.Size,
	}
	if req.Price != "" && strings.ToLower(req.OrderType) == "limit" {
		params["price"] = req.Price
		params["timeInForce"] = toBingXTIF(req.Force)
	}
	// BingX one-way mode: positionSide=BOTH, reduceOnly for close orders.
	params["positionSide"] = "BOTH"
	if req.ReduceOnly {
		params["reduceOnly"] = "true"
	}
	if req.ClientOid != "" {
		params["clientOrderId"] = req.ClientOid
	}

	result, err := a.client.Post("/openApi/swap/v2/trade/order", params)
	if err != nil {
		return "", fmt.Errorf("bingx PlaceOrder: %w", err)
	}

	var resp struct {
		Order struct {
			OrderID json.Number `json:"orderId"`
		} `json:"order"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return "", fmt.Errorf("bingx PlaceOrder parse: %w", err)
	}
	orderID := resp.Order.OrderID.String()
	if a.orderMetricsCallback != nil {
		a.orderMetricsCallback(exchange.OrderMetricEvent{
			Type:      exchange.OrderMetricPlaced,
			OrderID:   orderID,
			Timestamp: time.Now(),
		})
	}
	return orderID, nil
}

// CancelOrder cancels an order. Idempotent: returns nil if already cancelled/filled.
func (a *Adapter) CancelOrder(symbol, orderID string) error {
	params := map[string]string{
		"symbol":  toBingXSymbol(symbol),
		"orderId": orderID,
	}
	_, err := a.client.Delete("/openApi/swap/v2/trade/order", params)
	if err != nil {
		// 80018 = already filled, 80016 = order not exist
		if apiErr, ok := err.(*APIError); ok {
			if apiErr.Code == 80018 || apiErr.Code == 80016 {
				return nil
			}
		}
		return fmt.Errorf("bingx CancelOrder: %w", err)
	}
	return nil
}

// GetPendingOrders returns open orders for a symbol.
func (a *Adapter) GetPendingOrders(symbol string) ([]exchange.Order, error) {
	params := map[string]string{
		"symbol": toBingXSymbol(symbol),
	}
	result, err := a.client.Get("/openApi/swap/v2/trade/openOrders", params)
	if err != nil {
		return nil, fmt.Errorf("bingx GetPendingOrders: %w", err)
	}

	var resp struct {
		Orders []struct {
			OrderID       json.Number `json:"orderId"`
			ClientOrderID string      `json:"clientOrderId"`
			Symbol        string      `json:"symbol"`
			Side          string      `json:"side"`
			Type          string      `json:"type"`
			Price         string      `json:"price"`
			Quantity      string      `json:"origQty"`
			Status        string      `json:"status"`
		} `json:"orders"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return nil, fmt.Errorf("bingx GetPendingOrders parse: %w", err)
	}

	orders := make([]exchange.Order, 0, len(resp.Orders))
	for _, o := range resp.Orders {
		orders = append(orders, exchange.Order{
			OrderID:   o.OrderID.String(),
			ClientOid: o.ClientOrderID,
			Symbol:    fromBingXSymbol(o.Symbol),
			Side:      fromBingXSide(o.Side),
			OrderType: strings.ToLower(o.Type),
			Price:     o.Price,
			Size:      o.Quantity,
			Status:    normalizeBingXOrderStatus(o.Status),
		})
	}
	return orders, nil
}

// GetOrderFilledQty returns the cumulative filled quantity for an order.
// It also stores the avg price in the order store so confirmFill can pick it up.
func (a *Adapter) GetOrderFilledQty(orderID, symbol string) (float64, error) {
	params := map[string]string{
		"symbol":  toBingXSymbol(symbol),
		"orderId": orderID,
	}
	result, err := a.client.Get("/openApi/swap/v2/trade/order", params)
	if err != nil {
		return 0, fmt.Errorf("bingx GetOrderFilledQty: %w", err)
	}

	var resp struct {
		Order struct {
			ExecutedQty string `json:"executedQty"`
			AvgPrice    string `json:"avgPrice"`
			Status      string `json:"status"`
		} `json:"order"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return 0, fmt.Errorf("bingx GetOrderFilledQty parse: %w", err)
	}

	qty, err := strconv.ParseFloat(resp.Order.ExecutedQty, 64)
	if err != nil {
		return 0, fmt.Errorf("bingx GetOrderFilledQty parse qty: %w", err)
	}

	// Store avg price in order store so confirmFill can retrieve it.
	avgPrice, _ := strconv.ParseFloat(resp.Order.AvgPrice, 64)
	if qty > 0 && avgPrice > 0 {
		a.orderStore.Store(orderID, exchange.OrderUpdate{
			OrderID:      orderID,
			Status:       normalizeBingXOrderStatus(resp.Order.Status),
			FilledVolume: qty,
			AvgPrice:     avgPrice,
		})
	}
	if qty > 0 && a.orderMetricsCallback != nil {
		a.orderMetricsCallback(exchange.OrderMetricEvent{
			Type:      exchange.OrderMetricFilled,
			OrderID:   orderID,
			FilledQty: qty,
			Timestamp: time.Now(),
		})
	}

	return qty, nil
}

// ---------- Positions ----------

// GetPosition returns positions for a specific symbol.
func (a *Adapter) GetPosition(symbol string) ([]exchange.Position, error) {
	params := map[string]string{
		"symbol": toBingXSymbol(symbol),
	}
	result, err := a.client.Get("/openApi/swap/v2/user/positions", params)
	if err != nil {
		return nil, fmt.Errorf("bingx GetPosition: %w", err)
	}
	return a.parsePositions(result)
}

// GetAllPositions returns all open positions.
func (a *Adapter) GetAllPositions() ([]exchange.Position, error) {
	result, err := a.client.Get("/openApi/swap/v2/user/positions", map[string]string{})
	if err != nil {
		return nil, fmt.Errorf("bingx GetAllPositions: %w", err)
	}
	return a.parsePositions(result)
}

func (a *Adapter) parsePositions(data json.RawMessage) ([]exchange.Position, error) {
	var positions []struct {
		Symbol           string      `json:"symbol"`
		PositionSide     string      `json:"positionSide"` // LONG or SHORT
		PositionAmt      string      `json:"positionAmt"`
		AvailableAmt     string      `json:"availableAmt"`
		AvgPrice         string      `json:"avgPrice"`
		UnrealizedProfit string      `json:"unrealizedProfit"`
		Leverage         json.Number `json:"leverage"`
		Isolated         bool        `json:"isolated"`
		LiquidationPrice json.Number `json:"liquidationPrice"`
		MarkPrice        json.Number `json:"markPrice"`
	}
	if err := json.Unmarshal(data, &positions); err != nil {
		return nil, fmt.Errorf("bingx parsePositions: %w", err)
	}

	result := make([]exchange.Position, 0, len(positions))
	for _, p := range positions {
		size, _ := strconv.ParseFloat(p.PositionAmt, 64)
		if size == 0 {
			continue
		}

		holdSide := "long"
		if size < 0 || strings.ToUpper(p.PositionSide) == "SHORT" {
			holdSide = "short"
		}

		// BingX one-way mode: size can be negative for shorts
		absSize := fmt.Sprintf("%g", abs(size))

		marginMode := "cross"
		if p.Isolated {
			marginMode = "isolated"
		}

		avail := p.AvailableAmt
		if avail == "" {
			avail = absSize
		}

		result = append(result, exchange.Position{
			Symbol:           fromBingXSymbol(p.Symbol),
			HoldSide:         holdSide,
			Total:            absSize,
			Available:        avail,
			AverageOpenPrice: p.AvgPrice,
			UnrealizedPL:     p.UnrealizedProfit,
			Leverage:         p.Leverage.String(),
			MarginMode:       marginMode,
			LiquidationPrice: p.LiquidationPrice.String(),
			MarkPrice:        p.MarkPrice.String(),
		})
	}
	return result, nil
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// ---------- Account Config ----------

// SetLeverage sets the leverage for a symbol.
func (a *Adapter) SetLeverage(symbol string, leverage string, holdSide string) error {
	// BingX one-way mode: set leverage with side=BOTH
	sides := []string{"BOTH"}
	for _, side := range sides {
		params := map[string]string{
			"symbol":   toBingXSymbol(symbol),
			"side":     side,
			"leverage": leverage,
		}
		_, err := a.client.Post("/openApi/swap/v2/trade/leverage", params)
		if err != nil {
			// Idempotent: "already set" or "no need to change" errors are ok
			if apiErr, ok := err.(*APIError); ok {
				errMsg := strings.ToLower(apiErr.Msg)
				if strings.Contains(errMsg, "no need") || strings.Contains(errMsg, "already") ||
					strings.Contains(errMsg, "not modified") {
					continue
				}
			}
			return fmt.Errorf("bingx SetLeverage (%s): %w", side, err)
		}
	}
	return nil
}

// SetMarginMode sets the margin mode for a symbol.
// mode: "cross" or "isolated"
func (a *Adapter) SetMarginMode(symbol string, mode string) error {
	marginType := "CROSSED"
	if strings.ToLower(mode) == "isolated" {
		marginType = "ISOLATED"
	}
	params := map[string]string{
		"symbol":     toBingXSymbol(symbol),
		"marginType": marginType,
	}
	_, err := a.client.Post("/openApi/swap/v2/trade/marginType", params)
	if err != nil {
		// Idempotent: already in requested mode
		if apiErr, ok := err.(*APIError); ok {
			errMsg := strings.ToLower(apiErr.Msg)
			if strings.Contains(errMsg, "no need to change") || strings.Contains(errMsg, "already") {
				return nil
			}
		}
		return fmt.Errorf("bingx SetMarginMode: %w", err)
	}
	return nil
}

// ---------- Contract Info ----------

// LoadAllContracts loads all USDT perpetual contract specifications.
func (a *Adapter) LoadAllContracts() (map[string]exchange.ContractInfo, error) {
	result, err := a.client.Get("/openApi/swap/v2/quote/contracts", map[string]string{})
	if err != nil {
		return nil, fmt.Errorf("bingx LoadAllContracts: %w", err)
	}

	var contracts []struct {
		Symbol            string      `json:"symbol"`
		Size              json.Number `json:"size"`             // stepSize
		TradeMinQuantity  json.Number `json:"tradeMinQuantity"` // minSize
		PricePrecision    int         `json:"pricePrecision"`
		QuantityPrecision int         `json:"quantityPrecision"`
		Status            int         `json:"status"` // 1 = trading
	}
	if err := json.Unmarshal(result, &contracts); err != nil {
		return nil, fmt.Errorf("bingx LoadAllContracts parse: %w", err)
	}

	out := make(map[string]exchange.ContractInfo, len(contracts))
	for _, c := range contracts {
		if c.Status != 1 {
			continue
		}

		// Only include USDT pairs
		internalSymbol := fromBingXSymbol(c.Symbol)
		if !strings.HasSuffix(internalSymbol, "USDT") {
			continue
		}

		minSize, _ := c.TradeMinQuantity.Float64()
		stepSize, _ := c.Size.Float64()

		// Compute price step from precision
		priceStep := 1.0
		for i := 0; i < c.PricePrecision; i++ {
			priceStep /= 10
		}

		out[internalSymbol] = exchange.ContractInfo{
			Symbol:        internalSymbol,
			MinSize:       minSize,
			StepSize:      stepSize,
			SizeDecimals:  c.QuantityPrecision,
			PriceStep:     priceStep,
			PriceDecimals: c.PricePrecision,
		}
	}
	return out, nil
}

// ---------- Funding Rate ----------

// fetchAllFundingRates queries all funding rates in a single API call (no symbol param).
func (a *Adapter) fetchAllFundingRates() (map[string]*exchange.FundingRate, error) {
	result, err := a.client.Get("/openApi/swap/v2/quote/premiumIndex", map[string]string{})
	if err != nil {
		return nil, fmt.Errorf("bingx fetchAllFundingRates: %w", err)
	}

	var items []struct {
		Symbol               string `json:"symbol"`
		LastFundingRate      string `json:"lastFundingRate"`
		NextFundingTime      int64  `json:"nextFundingTime"`
		MarkPrice            string `json:"markPrice"`
		IndexPrice           string `json:"indexPrice"`
		FundingIntervalHours int    `json:"fundingIntervalHours"`
		MinFundingRate       string `json:"minFundingRate"`
		MaxFundingRate       string `json:"maxFundingRate"`
	}
	if err := json.Unmarshal(result, &items); err != nil {
		return nil, fmt.Errorf("bingx fetchAllFundingRates parse: %w", err)
	}

	cache := make(map[string]*exchange.FundingRate, len(items))
	for _, item := range items {
		rate, _ := strconv.ParseFloat(item.LastFundingRate, 64)
		nextTime := time.UnixMilli(item.NextFundingTime)

		interval := 8 * time.Hour
		if item.FundingIntervalHours > 0 {
			interval = time.Duration(item.FundingIntervalHours) * time.Hour
		}

		fr := &exchange.FundingRate{
			Symbol:      fromBingXSymbol(item.Symbol),
			Rate:        rate,
			Interval:    interval,
			NextFunding: nextTime,
		}

		if item.MaxFundingRate != "" {
			if v, err := strconv.ParseFloat(item.MaxFundingRate, 64); err == nil {
				fr.MaxRate = &v
			}
		}
		if item.MinFundingRate != "" {
			if v, err := strconv.ParseFloat(item.MinFundingRate, 64); err == nil {
				fr.MinRate = &v
			}
		}

		cache[fromBingXSymbol(item.Symbol)] = fr
	}

	return cache, nil
}

// GetFundingRate returns the current funding rate for a symbol.
// Uses a batch cache (30s TTL) to avoid per-symbol API calls.
func (a *Adapter) GetFundingRate(symbol string) (*exchange.FundingRate, error) {
	a.fundingRateCacheMu.Lock()
	defer a.fundingRateCacheMu.Unlock()

	// Return from cache if fresh enough
	if a.fundingRateCache != nil && time.Since(a.fundingRateCacheTime) < 30*time.Second {
		if fr, ok := a.fundingRateCache[symbol]; ok {
			return fr, nil
		}
	}

	// Refresh cache with batch query
	cache, err := a.fetchAllFundingRates()
	if err != nil {
		return nil, err
	}
	a.fundingRateCache = cache
	a.fundingRateCacheTime = time.Now()

	if fr, ok := cache[symbol]; ok {
		return fr, nil
	}
	return nil, fmt.Errorf("bingx GetFundingRate: symbol %s not found", symbol)
}

// GetFundingInterval returns the funding interval for a symbol.
// BingX default is 8h for most pairs.
func (a *Adapter) GetFundingInterval(symbol string) (time.Duration, error) {
	// BingX doesn't have a dedicated funding interval field in premiumIndex.
	// Default to 8 hours; LoadAllContracts can be checked if needed.
	return 8 * time.Hour, nil
}

// ---------- Account ----------

// GetFuturesBalance returns the futures account balance.
func (a *Adapter) GetFuturesBalance() (*exchange.Balance, error) {
	result, err := a.client.Get("/openApi/swap/v3/user/balance", map[string]string{})
	if err != nil {
		return nil, fmt.Errorf("bingx GetFuturesBalance: %w", err)
	}

	var balances []struct {
		Asset           string `json:"asset"`
		Balance         string `json:"balance"`
		Equity          string `json:"equity"`
		AvailableMargin string `json:"availableMargin"`
		UsedMargin      string `json:"usedMargin"`
		FreezedMargin   string `json:"freezedMargin"`
	}
	if err := json.Unmarshal(result, &balances); err != nil {
		return nil, fmt.Errorf("bingx GetFuturesBalance parse: %w", err)
	}

	for _, b := range balances {
		if b.Asset == "USDT" {
			total, _ := strconv.ParseFloat(b.Equity, 64)
			available, _ := strconv.ParseFloat(b.AvailableMargin, 64)
			used, _ := strconv.ParseFloat(b.UsedMargin, 64)
			var marginRatio float64
			if total > 0 && available >= 0 {
				marginRatio = 1 - available/total // consistent with health monitor fallback
			}
			// BingX has no dedicated max transferable API.
			// Calculate from: balance (wallet, no unrealized PnL) - usedMargin - freezedMargin
			walletBal, _ := strconv.ParseFloat(b.Balance, 64)
			freezed, _ := strconv.ParseFloat(b.FreezedMargin, 64)
			maxTransfer := walletBal - used - freezed
			if maxTransfer < 0 {
				maxTransfer = 0
			}

			return &exchange.Balance{
				Total:          total,
				Available:      available,
				Frozen:         used,
				Currency:       "USDT",
				// BingX v3 balance does not expose a native maintenance-style risk
				// ratio. The derived available/equity heuristic is useful for local
				// display, but it is not comparable to Binance/Bybit/OKX health
				// thresholds, so global health logic must treat it as unavailable.
				MarginRatio:            marginRatio,
				MarginRatioUnavailable: true,
				MaxTransferOut: maxTransfer,
			}, nil
		}
	}
	return &exchange.Balance{Currency: "USDT"}, nil
}

// GetSpotBalance returns the fund account balance (where deposits land).
// BingX split into 4 accounts (Fund/Spot/Standard Futures/Perpetual Futures)
// in May 2025; deposits always go to the Fund account.
func (a *Adapter) GetSpotBalance() (*exchange.Balance, error) {
	result, err := a.client.Get("/openApi/fund/v1/account/balance", map[string]string{})
	if err != nil {
		return nil, fmt.Errorf("bingx GetSpotBalance: %w", err)
	}

	var resp struct {
		Assets []struct {
			Asset  string      `json:"asset"`
			Free   json.Number `json:"free"`
			Locked json.Number `json:"locked"`
		} `json:"assets"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return nil, fmt.Errorf("bingx GetSpotBalance parse: %w", err)
	}

	for _, b := range resp.Assets {
		if b.Asset == "USDT" {
			free, _ := b.Free.Float64()
			locked, _ := b.Locked.Float64()
			return &exchange.Balance{
				Total:     free + locked,
				Available: free,
				Frozen:    locked,
				Currency:  "USDT",
			}, nil
		}
	}
	return &exchange.Balance{Currency: "USDT"}, nil
}

// ---------- Withdraw & Transfer ----------

// TransferToSpot moves funds from perpetual futures to fund account.
// Uses the asset transfer API (innerTransfer is for inter-user transfers).
func (a *Adapter) TransferToSpot(coin string, amount string) error {
	params := map[string]string{
		"type":   "PFUTURES_FUND",
		"asset":  coin,
		"amount": amount,
	}
	_, err := a.client.Post("/openApi/api/v3/post/asset/transfer", params)
	if err != nil {
		return fmt.Errorf("bingx TransferToSpot: %w", err)
	}
	return nil
}

// TransferToFutures moves funds from fund account to perpetual futures.
// Uses the asset transfer API (innerTransfer is for inter-user transfers).
func (a *Adapter) TransferToFutures(coin string, amount string) error {
	params := map[string]string{
		"type":   "FUND_PFUTURES",
		"asset":  coin,
		"amount": amount,
	}
	_, err := a.client.Post("/openApi/api/v3/post/asset/transfer", params)
	if err != nil {
		return fmt.Errorf("bingx TransferToFutures: %w", err)
	}
	return nil
}

// Withdraw initiates a withdrawal from BingX.
func (a *Adapter) Withdraw(params exchange.WithdrawParams) (*exchange.WithdrawResult, error) {
	reqParams := map[string]string{
		"coin":    params.Coin,
		"network": mapChainToBingXNetwork(params.Chain),
		"address": params.Address,
		"amount":  params.Amount,
	}
	result, err := a.client.Post("/openApi/wallets/v1/capital/withdraw/apply", reqParams)
	if err != nil {
		return nil, fmt.Errorf("bingx Withdraw: %w", err)
	}

	var resp struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return nil, fmt.Errorf("bingx Withdraw parse: %w", err)
	}
	return &exchange.WithdrawResult{
		TxID:   resp.ID,
		Status: "submitted",
	}, nil
}

// WithdrawFeeInclusive returns true because BingX Withdraw amount includes fee (recipient gets amount - fee).
func (a *Adapter) WithdrawFeeInclusive() bool { return true }

// GetWithdrawFee queries the BingX API for the withdrawal fee of a coin on a given chain.
func (a *Adapter) GetWithdrawFee(coin, chain string) (float64, error) {
	network := mapChainToBingXNetwork(chain)
	params := map[string]string{
		"coin": coin,
	}
	data, err := a.client.Get("/openApi/wallets/v1/capital/config/getall", params)
	if err != nil {
		return 0, fmt.Errorf("bingx GetWithdrawFee: %w", err)
	}

	var coins []struct {
		Coin        string `json:"coin"`
		NetworkList []struct {
			Network     string `json:"network"`
			WithdrawFee string `json:"withdrawFee"`
		} `json:"networkList"`
	}
	if err := json.Unmarshal(data, &coins); err != nil {
		return 0, fmt.Errorf("bingx GetWithdrawFee unmarshal: %w", err)
	}

	for _, c := range coins {
		if !strings.EqualFold(c.Coin, coin) {
			continue
		}
		for _, n := range c.NetworkList {
			if strings.EqualFold(n.Network, network) {
				fee, err := strconv.ParseFloat(n.WithdrawFee, 64)
				if err != nil {
					return 0, fmt.Errorf("bingx GetWithdrawFee parse fee: %w", err)
				}
				return fee, nil
			}
		}
		return 0, fmt.Errorf("bingx GetWithdrawFee: network %s not found for %s", network, coin)
	}
	return 0, fmt.Errorf("bingx GetWithdrawFee: coin %s not found", coin)
}

func mapChainToBingXNetwork(chain string) string {
	switch chain {
	case "APT":
		return "APT"
	case "BEP20":
		return "BEP20"
	default:
		return chain
	}
}

// ---------- Orderbook ----------

// GetOrderbook returns the order book for a symbol.
func (a *Adapter) GetOrderbook(symbol string, depth int) (*exchange.Orderbook, error) {
	params := map[string]string{
		"symbol": toBingXSymbol(symbol),
		"limit":  strconv.Itoa(depth),
	}
	result, err := a.client.Get("/openApi/swap/v2/quote/depth", params)
	if err != nil {
		return nil, fmt.Errorf("bingx GetOrderbook: %w", err)
	}

	var resp struct {
		Bids [][]string `json:"bids"` // [["price","qty"],...]
		Asks [][]string `json:"asks"`
		T    int64      `json:"T"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return nil, fmt.Errorf("bingx GetOrderbook parse: %w", err)
	}

	ob := &exchange.Orderbook{
		Symbol: symbol,
		Bids:   make([]exchange.PriceLevel, 0, len(resp.Bids)),
		Asks:   make([]exchange.PriceLevel, 0, len(resp.Asks)),
		Time:   time.UnixMilli(resp.T),
	}

	for _, level := range resp.Bids {
		if len(level) < 2 {
			continue
		}
		price, _ := strconv.ParseFloat(level[0], 64)
		qty, _ := strconv.ParseFloat(level[1], 64)
		ob.Bids = append(ob.Bids, exchange.PriceLevel{Price: price, Quantity: qty})
	}
	for _, level := range resp.Asks {
		if len(level) < 2 {
			continue
		}
		price, _ := strconv.ParseFloat(level[0], 64)
		qty, _ := strconv.ParseFloat(level[1], 64)
		ob.Asks = append(ob.Asks, exchange.PriceLevel{Price: price, Quantity: qty})
	}

	return ob, nil
}

// ---------- WebSocket: Prices ----------

// StartPriceStream starts the public WebSocket for price streaming.
func (a *Adapter) StartPriceStream(symbols []string) {
	// Convert symbols to BingX format
	bxSymbols := make([]string, len(symbols))
	for i, s := range symbols {
		bxSymbols[i] = toBingXSymbol(s)
	}
	a.publicWS = NewPublicWS(&a.priceStore, &a.depthStore)
	a.publicWS.SetMetricsCallback(a.wsMetricsCallback)
	a.publicWS.Connect(bxSymbols)
}

// SubscribeSymbol subscribes to a new symbol on the public WebSocket.
func (a *Adapter) SubscribeSymbol(symbol string) bool {
	if a.publicWS == nil {
		return false
	}
	return a.publicWS.Subscribe(toBingXSymbol(symbol))
}

// GetBBO returns the best bid/offer for a symbol.
func (a *Adapter) GetBBO(symbol string) (exchange.BBO, bool) {
	val, ok := a.priceStore.Load(symbol)
	if !ok {
		return exchange.BBO{}, false
	}
	bbo, ok := val.(exchange.BBO)
	return bbo, ok
}

// GetPriceStore returns the underlying sync.Map for BBO data.
func (a *Adapter) GetPriceStore() *sync.Map {
	return &a.priceStore
}

// ---------- WebSocket: Depth ----------

// SubscribeDepth subscribes to orderbook depth via the public WebSocket.
func (a *Adapter) SubscribeDepth(symbol string) bool {
	if a.publicWS == nil {
		return false
	}
	return a.publicWS.SubscribeDepth(toBingXSymbol(symbol))
}

// UnsubscribeDepth unsubscribes from orderbook depth.
func (a *Adapter) UnsubscribeDepth(symbol string) bool {
	if a.publicWS == nil {
		return false
	}
	return a.publicWS.UnsubscribeDepth(toBingXSymbol(symbol))
}

// GetDepth returns the latest orderbook depth snapshot.
func (a *Adapter) GetDepth(symbol string) (*exchange.Orderbook, bool) {
	val, ok := a.depthStore.Load(symbol)
	if !ok {
		return nil, false
	}
	return val.(*exchange.Orderbook), true
}

// ---------- WebSocket: Private ----------

// StartPrivateStream starts the private WebSocket for order updates.
func (a *Adapter) StartPrivateStream() {
	a.privateWS = NewPrivateWS(a.client, &a.orderStore, &a.orderCallback)
	a.privateWS.SetOrderMetricsCallback(a.orderMetricsCallback)
	a.privateWS.Connect()
}

// GetOrderUpdate returns the latest order update for an order ID.
func (a *Adapter) GetOrderUpdate(orderID string) (exchange.OrderUpdate, bool) {
	val, ok := a.orderStore.Load(orderID)
	if !ok {
		return exchange.OrderUpdate{}, false
	}
	upd, ok := val.(exchange.OrderUpdate)
	return upd, ok
}

// ---------- Stop-Loss ----------

// PlaceStopLoss places a stop-market order on BingX.
func (a *Adapter) PlaceStopLoss(params exchange.StopLossParams) (string, error) {
	// BingX one-way mode: positionSide=BOTH, reduceOnly for SL.
	p := map[string]string{
		"symbol":       toBingXSymbol(params.Symbol),
		"type":         "STOP_MARKET",
		"side":         toBingXSide(params.Side),
		"positionSide": "BOTH",
		"quantity":     params.Size,
		"stopPrice":    params.TriggerPrice,
		"reduceOnly":   "true",
	}

	result, err := a.client.Post("/openApi/swap/v2/trade/order", p)
	if err != nil {
		return "", fmt.Errorf("bingx PlaceStopLoss: %w", err)
	}

	var resp struct {
		Order struct {
			OrderID json.Number `json:"orderId"`
		} `json:"order"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return "", fmt.Errorf("bingx PlaceStopLoss parse: %w", err)
	}
	return resp.Order.OrderID.String(), nil
}

// CancelStopLoss cancels a stop-loss order (same as CancelOrder on BingX).
func (a *Adapter) CancelStopLoss(symbol, orderID string) error {
	return a.CancelOrder(symbol, orderID)
}

// ---------- Trade History ----------

// GetUserTrades returns filled trades for a symbol since startTime.
// Uses /openApi/swap/v2/trade/fillHistory (newer endpoint with correct params).
func (a *Adapter) GetUserTrades(symbol string, startTime time.Time, limit int) ([]exchange.Trade, error) {
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	params := map[string]string{
		"symbol":  toBingXSymbol(symbol),
		"startTs": strconv.FormatInt(startTime.UnixMilli(), 10),
		"endTs":   strconv.FormatInt(time.Now().UnixMilli(), 10),
	}
	if limit > 0 {
		params["pageSize"] = strconv.Itoa(limit)
	}
	result, err := a.client.Get("/openApi/swap/v2/trade/fillHistory", params)
	if err != nil {
		return nil, fmt.Errorf("bingx GetUserTrades: %w", err)
	}

	var resp struct {
		FillOrders []struct {
			FilledTime      string `json:"filledTime"` // datetime string e.g. "2026-03-26T16:53:55.000+08:00"
			OrderID         string `json:"orderId"`
			Symbol          string `json:"symbol"`
			Side            string `json:"side"`
			Price           string `json:"price"`
			Qty             string `json:"qty"`
			Commission      string `json:"commission"`
			CommissionAsset string `json:"commissionAsset"`
			TradeID         string `json:"tradeId"`
		} `json:"fill_history_orders"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return nil, fmt.Errorf("bingx GetUserTrades parse: %w", err)
	}

	trades := make([]exchange.Trade, 0, len(resp.FillOrders))
	for _, t := range resp.FillOrders {
		price, _ := strconv.ParseFloat(t.Price, 64)
		qty, _ := strconv.ParseFloat(t.Qty, 64)
		fee, _ := strconv.ParseFloat(t.Commission, 64)
		if fee < 0 {
			fee = -fee
		}
		fillTime, _ := time.Parse("2006-01-02T15:04:05.000-07:00", t.FilledTime)
		trades = append(trades, exchange.Trade{
			TradeID:  t.TradeID,
			OrderID:  t.OrderID,
			Symbol:   fromBingXSymbol(t.Symbol),
			Side:     fromBingXSide(t.Side),
			Price:    price,
			Quantity: qty,
			Fee:      fee,
			FeeCoin:  t.CommissionAsset,
			Time:     fillTime,
		})
	}
	return trades, nil
}

// ---------- Funding Fee History ----------

// fetchAllFundingFees calls the income endpoint WITHOUT symbol to get all symbols' funding fees in one request.
func (a *Adapter) fetchAllFundingFees(since time.Time) (map[string][]exchange.FundingPayment, error) {
	params := map[string]string{
		"incomeType": "FUNDING_FEE",
		"startTime":  strconv.FormatInt(since.UnixMilli(), 10),
		"limit":      "1000",
	}
	result, err := a.client.Get("/openApi/swap/v2/user/income", params)
	if err != nil {
		return nil, fmt.Errorf("bingx fetchAllFundingFees: %w", err)
	}

	var records []struct {
		Symbol string `json:"symbol"`
		Income string `json:"income"`
		Time   int64  `json:"time"`
	}
	if err := json.Unmarshal(result, &records); err != nil {
		return nil, fmt.Errorf("bingx fetchAllFundingFees parse: %w", err)
	}

	out := make(map[string][]exchange.FundingPayment, len(records))
	for _, r := range records {
		sym := fromBingXSymbol(r.Symbol)
		amt, _ := strconv.ParseFloat(r.Income, 64)
		out[sym] = append(out[sym], exchange.FundingPayment{
			Amount: amt,
			Time:   time.UnixMilli(r.Time),
		})
	}
	return out, nil
}

// GetFundingFees returns funding fee history for a symbol since the given time.
// Uses a batch cache (30s TTL) so multiple per-symbol calls share one API request.
func (a *Adapter) GetFundingFees(symbol string, since time.Time) ([]exchange.FundingPayment, error) {
	a.fundingFeesCacheMu.Lock()
	defer a.fundingFeesCacheMu.Unlock()

	now := time.Now()
	// Cache is valid if fresh AND covers the requested time range (since >= cached since).
	cacheValid := a.fundingFeesCache != nil &&
		now.Sub(a.fundingFeesCacheTime) < 30*time.Second &&
		!since.Before(a.fundingFeesCacheSince)

	if !cacheValid {
		fees, err := a.fetchAllFundingFees(since)
		if err != nil {
			return nil, err
		}
		a.fundingFeesCache = fees
		a.fundingFeesCacheTime = now
		a.fundingFeesCacheSince = since
	}

	// Filter cached results to only return payments after caller's since.
	allPayments, ok := a.fundingFeesCache[symbol]
	if !ok {
		return nil, nil
	}
	var filtered []exchange.FundingPayment
	for _, p := range allPayments {
		if !p.Time.Before(since) {
			filtered = append(filtered, p)
		}
	}
	return filtered, nil
}

// ---------- Close PnL ----------

// GetClosePnL returns exchange-reported position-level PnL for recently closed positions.
func (a *Adapter) GetClosePnL(symbol string, since time.Time) ([]exchange.ClosePnL, error) {
	params := map[string]string{
		"symbol":  toBingXSymbol(symbol),
		"startTs": strconv.FormatInt(since.UnixMilli(), 10),
		"endTs":   strconv.FormatInt(time.Now().UnixMilli(), 10),
	}
	result, err := a.client.Get("/openApi/swap/v1/trade/positionHistory", params)
	if err != nil {
		return nil, fmt.Errorf("bingx GetClosePnL: %w", err)
	}

	var resp struct {
		PositionHistory []struct {
			NetProfit          string `json:"netProfit"`
			RealisedProfit     string `json:"realisedProfit"`
			PositionCommission string `json:"positionCommission"`
			TotalFunding       string `json:"totalFunding"`
			AvgPrice           string `json:"avgPrice"`
			AvgClosePrice      string `json:"avgClosePrice"`
			PositionAmt        string `json:"positionAmt"`
			PositionSide       string `json:"positionSide"`
			UpdateTime         int64  `json:"updateTime"`
		} `json:"positionHistory"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return nil, fmt.Errorf("bingx GetClosePnL parse: %w", err)
	}

	out := make([]exchange.ClosePnL, 0, len(resp.PositionHistory))
	for _, r := range resp.PositionHistory {
		netPnL, _ := strconv.ParseFloat(r.NetProfit, 64)
		pricePnL, _ := strconv.ParseFloat(r.RealisedProfit, 64)
		fees, _ := strconv.ParseFloat(r.PositionCommission, 64)
		funding, _ := strconv.ParseFloat(r.TotalFunding, 64)
		entryPrice, _ := strconv.ParseFloat(r.AvgPrice, 64)
		exitPrice, _ := strconv.ParseFloat(r.AvgClosePrice, 64)
		closeSize, _ := strconv.ParseFloat(r.PositionAmt, 64)

		// Normalize side: BingX uses "LONG"/"SHORT"
		side := "long"
		if r.PositionSide == "SHORT" {
			side = "short"
		}

		// BingX netProfit already includes realisedProfit + commission + totalFunding.
		out = append(out, exchange.ClosePnL{
			PricePnL:   pricePnL,
			Fees:       fees,
			Funding:    funding,
			NetPnL:     netPnL,
			EntryPrice: entryPrice,
			ExitPrice:  exitPrice,
			CloseSize:  math.Abs(closeSize),
			Side:       side,
			CloseTime:  time.UnixMilli(r.UpdateTime),
		})
	}
	return out, nil
}

// ---------- Helpers ----------

// normalizeBingXOrderStatus converts BingX order status to lowercase standard format.
func normalizeBingXOrderStatus(status string) string {
	switch status {
	case "Pending", "NEW":
		return "new"
	case "PartiallyFilled", "PARTIALLY_FILLED":
		return "partially_filled"
	case "Filled", "FILLED":
		return "filled"
	case "Cancelled", "CANCELED":
		return "cancelled"
	case "Failed", "EXPIRED":
		return "failed"
	default:
		return strings.ToLower(status)
	}
}

// generateUUID creates a random UUID v4 string.
func generateUUID() string {
	var b [16]byte
	rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// Ensure Adapter implements exchange.Exchange at compile time.
var _ exchange.Exchange = (*Adapter)(nil)
var _ exchange.TradingFeeProvider = (*Adapter)(nil)

// EnsureOneWayMode is a no-op for BingX — position mode is set via UI only.
// The adapter already uses positionSide=BOTH (one-way mode).
// Close terminates all WebSocket connections for graceful shutdown.
func (a *Adapter) Close() {
	if a.publicWS != nil {
		a.publicWS.Close()
	}
	if a.privateWS != nil {
		a.privateWS.Close()
	}
}

func (a *Adapter) EnsureOneWayMode() error {
	return nil
}

// GetTradingFee returns the authenticated user's maker/taker fee rates.
func (a *Adapter) GetTradingFee() (*exchange.TradingFee, error) {
	result, err := a.client.Get("/openApi/swap/v2/user/commissionRate", map[string]string{})
	if err != nil {
		return nil, fmt.Errorf("bingx GetTradingFee: %w", err)
	}

	var resp struct {
		Commission struct {
			TakerCommissionRate float64 `json:"takerCommissionRate"`
			MakerCommissionRate float64 `json:"makerCommissionRate"`
		} `json:"commission"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return nil, fmt.Errorf("bingx GetTradingFee unmarshal: %w", err)
	}

	return &exchange.TradingFee{
		MakerRate: resp.Commission.MakerCommissionRate,
		TakerRate: resp.Commission.TakerCommissionRate,
	}, nil
}
