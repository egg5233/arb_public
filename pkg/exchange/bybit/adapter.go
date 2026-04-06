package bybit

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"arb/pkg/exchange"
	"arb/pkg/utils"
)

var log = utils.NewLogger("bybit")

// Adapter implements the exchange.Exchange interface for Bybit.
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
}

// NewAdapter creates a new Bybit exchange adapter.
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
	return "bybit"
}

// IsUnified returns true because all Bybit accounts use Unified Trading Account (UTA).
func (a *Adapter) IsUnified() bool { return true }

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

func (a *Adapter) CheckPermissions() exchange.PermissionResult {
	data, err := a.client.Get("/v5/user/query-api", nil)
	if err != nil {
		return exchange.PermissionResult{Method: "direct", Error: err.Error(),
			Read: exchange.PermUnknown, FuturesTrade: exchange.PermUnknown,
			Withdraw: exchange.PermUnknown, Transfer: exchange.PermUnknown}
	}
	var resp struct {
		ReadOnly    int                 `json:"readOnly"`
		Permissions map[string][]string `json:"permissions"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return exchange.PermissionResult{Method: "direct", Error: err.Error(),
			Read: exchange.PermUnknown, FuturesTrade: exchange.PermUnknown,
			Withdraw: exchange.PermUnknown, Transfer: exchange.PermUnknown}
	}
	read := exchange.PermGranted // if API responds, read is granted
	trade := exchange.PermDenied
	withdraw := exchange.PermDenied
	transfer := exchange.PermDenied
	// readOnly=1 means key can only read — no trade/wallet permissions regardless of what's listed
	if resp.ReadOnly == 1 {
		return exchange.PermissionResult{
			Read: read, FuturesTrade: exchange.PermDenied,
			Withdraw: exchange.PermDenied, Transfer: exchange.PermDenied,
			Method: "direct",
		}
	}
	if len(resp.Permissions["ContractTrade"]) > 0 {
		trade = exchange.PermGranted
	}
	for _, v := range resp.Permissions["Wallet"] {
		if v == "Withdraw" {
			withdraw = exchange.PermGranted
		}
		if v == "AccountTransfer" {
			transfer = exchange.PermGranted
		}
	}
	return exchange.PermissionResult{
		Read: read, FuturesTrade: trade, Withdraw: withdraw, Transfer: transfer,
		Method: "direct",
	}
}

// ---------- Side helpers ----------

func toBybitSide(side exchange.Side) string {
	switch side {
	case exchange.SideBuy:
		return "Buy"
	case exchange.SideSell:
		return "Sell"
	default:
		return string(side)
	}
}

func fromBybitSide(side string) string {
	switch side {
	case "Buy":
		return "buy"
	case "Sell":
		return "sell"
	default:
		return strings.ToLower(side)
	}
}

// ---------- Time-in-force mapping ----------

func toBybitTIF(force string) string {
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

func toBybitOrderType(orderType string) string {
	switch strings.ToLower(orderType) {
	case "limit":
		return "Limit"
	case "market":
		return "Market"
	default:
		return orderType
	}
}

// ---------- Orders ----------

// PlaceOrder places a new order on Bybit.
func (a *Adapter) PlaceOrder(req exchange.PlaceOrderParams) (string, error) {
	log.Info("PlaceOrder: symbol=%s side=%s type=%s size=%s price=%s force=%s reduceOnly=%v",
		req.Symbol, req.Side, req.OrderType, req.Size, req.Price, req.Force, req.ReduceOnly)
	params := map[string]string{
		"category":    "linear",
		"symbol":      req.Symbol,
		"side":        toBybitSide(req.Side),
		"orderType":   toBybitOrderType(req.OrderType),
		"qty":         req.Size,
		"timeInForce": toBybitTIF(req.Force),
	}
	if req.Price != "" && strings.ToLower(req.OrderType) == "limit" {
		params["price"] = req.Price
	}
	if req.ReduceOnly {
		params["reduceOnly"] = "true"
	}
	if req.ClientOid != "" {
		params["orderLinkId"] = req.ClientOid
	}

	result, err := a.client.Post("/v5/order/create", params)
	if err != nil {
		return "", fmt.Errorf("bybit PlaceOrder: %w", err)
	}

	var resp struct {
		OrderID     string `json:"orderId"`
		OrderLinkID string `json:"orderLinkId"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return "", fmt.Errorf("bybit PlaceOrder parse: %w", err)
	}
	if a.orderMetricsCallback != nil {
		a.orderMetricsCallback(exchange.OrderMetricEvent{
			Type:      exchange.OrderMetricPlaced,
			OrderID:   resp.OrderID,
			Timestamp: time.Now(),
		})
	}
	return resp.OrderID, nil
}

// CancelOrder cancels an order. Idempotent: returns nil if already cancelled/filled.
func (a *Adapter) CancelOrder(symbol, orderID string) error {
	params := map[string]string{
		"category": "linear",
		"symbol":   symbol,
		"orderId":  orderID,
	}
	_, err := a.client.Post("/v5/order/cancel", params)
	if err != nil {
		// Bybit returns 110001 for order not found (already cancelled/filled)
		if apiErr, ok := err.(*APIError); ok {
			if apiErr.Code == 110001 || apiErr.Code == 170213 {
				return nil
			}
		}
		return fmt.Errorf("bybit CancelOrder: %w", err)
	}
	return nil
}

// GetPendingOrders returns open orders for a symbol.
func (a *Adapter) GetPendingOrders(symbol string) ([]exchange.Order, error) {
	params := map[string]string{
		"category": "linear",
		"symbol":   symbol,
		"openOnly": "0",
	}
	result, err := a.client.Get("/v5/order/realtime", params)
	if err != nil {
		return nil, fmt.Errorf("bybit GetPendingOrders: %w", err)
	}

	var resp struct {
		List []struct {
			OrderID     string `json:"orderId"`
			OrderLinkID string `json:"orderLinkId"`
			Symbol      string `json:"symbol"`
			Side        string `json:"side"`
			OrderType   string `json:"orderType"`
			Price       string `json:"price"`
			Qty         string `json:"qty"`
			OrderStatus string `json:"orderStatus"`
		} `json:"list"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return nil, fmt.Errorf("bybit GetPendingOrders parse: %w", err)
	}

	orders := make([]exchange.Order, 0, len(resp.List))
	for _, o := range resp.List {
		orders = append(orders, exchange.Order{
			OrderID:   o.OrderID,
			ClientOid: o.OrderLinkID,
			Symbol:    o.Symbol,
			Side:      fromBybitSide(o.Side),
			OrderType: strings.ToLower(o.OrderType),
			Price:     o.Price,
			Size:      o.Qty,
			Status:    strings.ToLower(o.OrderStatus),
		})
	}
	return orders, nil
}

// GetOrderFilledQty returns the cumulative filled quantity for an order.
func (a *Adapter) GetOrderFilledQty(orderID, symbol string) (float64, error) {
	params := map[string]string{
		"category": "linear",
		"symbol":   symbol,
		"orderId":  orderID,
	}
	result, err := a.client.Get("/v5/order/realtime", params)
	if err != nil {
		return 0, fmt.Errorf("bybit GetOrderFilledQty: %w", err)
	}

	var resp struct {
		List []struct {
			CumExecQty string `json:"cumExecQty"`
		} `json:"list"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return 0, fmt.Errorf("bybit GetOrderFilledQty parse: %w", err)
	}
	if len(resp.List) == 0 {
		return 0, fmt.Errorf("bybit GetOrderFilledQty: order %s not found", orderID)
	}

	qty, err := strconv.ParseFloat(resp.List[0].CumExecQty, 64)
	if err != nil {
		return 0, fmt.Errorf("bybit GetOrderFilledQty parse qty: %w", err)
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
		"category": "linear",
		"symbol":   symbol,
	}
	result, err := a.client.Get("/v5/position/list", params)
	if err != nil {
		return nil, fmt.Errorf("bybit GetPosition: %w", err)
	}
	return a.parsePositions(result)
}

// GetAllPositions returns all open positions.
func (a *Adapter) GetAllPositions() ([]exchange.Position, error) {
	params := map[string]string{
		"category":   "linear",
		"settleCoin": "USDT",
	}
	result, err := a.client.Get("/v5/position/list", params)
	if err != nil {
		return nil, fmt.Errorf("bybit GetAllPositions: %w", err)
	}
	return a.parsePositions(result)
}

func (a *Adapter) parsePositions(data json.RawMessage) ([]exchange.Position, error) {
	var resp struct {
		List []struct {
			Symbol        string `json:"symbol"`
			Side          string `json:"side"`
			Size          string `json:"size"`
			AvgPrice      string `json:"avgPrice"`
			UnrealisedPnl string `json:"unrealisedPnl"`
			Leverage      string `json:"leverage"`
			TradeMode     int    `json:"tradeMode"`
			PositionValue string `json:"positionValue"`
			LiqPrice      string `json:"liqPrice"`
			MarkPrice     string `json:"markPrice"`
		} `json:"list"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("bybit parsePositions: %w", err)
	}

	positions := make([]exchange.Position, 0, len(resp.List))
	for _, p := range resp.List {
		// Skip empty positions.
		size, _ := strconv.ParseFloat(p.Size, 64)
		if size == 0 {
			continue
		}

		holdSide := "long"
		if p.Side == "Sell" {
			holdSide = "short"
		}

		marginMode := "cross"
		if p.TradeMode == 1 {
			marginMode = "isolated"
		}

		positions = append(positions, exchange.Position{
			Symbol:           p.Symbol,
			HoldSide:         holdSide,
			Total:            p.Size,
			Available:        p.Size,
			AverageOpenPrice: p.AvgPrice,
			UnrealizedPL:     p.UnrealisedPnl,
			Leverage:         p.Leverage,
			MarginMode:       marginMode,
			LiquidationPrice: p.LiqPrice,
			MarkPrice:        p.MarkPrice,
		})
	}
	return positions, nil
}

// ---------- Account Config ----------

// SetLeverage sets the leverage for a symbol.
func (a *Adapter) SetLeverage(symbol string, leverage string, holdSide string) error {
	params := map[string]string{
		"category":     "linear",
		"symbol":       symbol,
		"buyLeverage":  leverage,
		"sellLeverage": leverage,
	}
	_, err := a.client.Post("/v5/position/set-leverage", params)
	if err != nil {
		// Bybit returns 110043 if leverage is already set to the same value.
		if apiErr, ok := err.(*APIError); ok && apiErr.Code == 110043 {
			return nil
		}
		return fmt.Errorf("bybit SetLeverage: %w", err)
	}
	return nil
}

// SetMarginMode sets the margin mode for a symbol.
// mode: "cross" or "isolated"
func (a *Adapter) SetMarginMode(symbol string, mode string) error {
	tradeMode := "0" // cross
	if strings.ToLower(mode) == "isolated" {
		tradeMode = "1"
	}
	params := map[string]string{
		"category":  "linear",
		"symbol":    symbol,
		"tradeMode": tradeMode,
		// Bybit requires leverage values when switching margin mode.
		"buyLeverage":  "10",
		"sellLeverage": "10",
	}
	_, err := a.client.Post("/v5/position/switch-isolated", params)
	if err != nil {
		if apiErr, ok := err.(*APIError); ok {
			// 110026 = already in the requested mode
			// 100028 = unified trading account (margin mode managed at account level)
			if apiErr.Code == 110026 || apiErr.Code == 100028 {
				return nil
			}
		}
		return fmt.Errorf("bybit SetMarginMode: %w", err)
	}
	return nil
}

// ---------- Contract Info ----------

// LoadAllContracts loads all linear USDT perpetual contract specifications.
func (a *Adapter) LoadAllContracts() (map[string]exchange.ContractInfo, error) {
	params := map[string]string{
		"category": "linear",
	}
	result, err := a.client.Get("/v5/market/instruments-info", params)
	if err != nil {
		return nil, fmt.Errorf("bybit LoadAllContracts: %w", err)
	}

	var resp struct {
		List []struct {
			Symbol        string `json:"symbol"`
			Status        string `json:"status"`
			LotSizeFilter struct {
				MinOrderQty string `json:"minOrderQty"`
				MaxOrderQty string `json:"maxOrderQty"`
				QtyStep     string `json:"qtyStep"`
			} `json:"lotSizeFilter"`
			PriceFilter struct {
				TickSize string `json:"tickSize"`
			} `json:"priceFilter"`
			FundingInterval json.Number `json:"fundingInterval"`
		} `json:"list"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return nil, fmt.Errorf("bybit LoadAllContracts parse: %w", err)
	}

	contracts := make(map[string]exchange.ContractInfo, len(resp.List))
	for _, inst := range resp.List {
		if inst.Status != "Trading" {
			continue
		}

		minSize, _ := strconv.ParseFloat(inst.LotSizeFilter.MinOrderQty, 64)
		maxSize, _ := strconv.ParseFloat(inst.LotSizeFilter.MaxOrderQty, 64)
		stepSize, _ := strconv.ParseFloat(inst.LotSizeFilter.QtyStep, 64)
		priceStep, _ := strconv.ParseFloat(inst.PriceFilter.TickSize, 64)

		contracts[inst.Symbol] = exchange.ContractInfo{
			Symbol:        inst.Symbol,
			MinSize:       minSize,
			StepSize:      stepSize,
			MaxSize:       maxSize,
			SizeDecimals:  countDecimals(stepSize),
			PriceStep:     priceStep,
			PriceDecimals: countDecimals(priceStep),
		}
	}

	// Load tier-1 maintenance rates from risk-limit endpoint
	a.loadMaintenanceRates(contracts)

	return contracts, nil
}

// countDecimals returns the number of decimal places in a float.
func countDecimals(v float64) int {
	if v == 0 {
		return 0
	}
	s := strconv.FormatFloat(v, 'f', -1, 64)
	idx := strings.IndexByte(s, '.')
	if idx < 0 {
		return 0
	}
	return len(s) - idx - 1
}

// ---------- Maintenance Rate ----------

// loadMaintenanceRates fetches risk-limit data for all symbols and populates
// the tier-1 (lowest risk) maintenance rate in each ContractInfo.
// CRITICAL: Bybit maintenanceMargin is a PERCENTAGE string ("0.5" = 0.5%).
// Must divide by 100 to get decimal.
func (a *Adapter) loadMaintenanceRates(contracts map[string]exchange.ContractInfo) {
	params := map[string]string{
		"category": "linear",
	}
	cursor := ""
	for {
		if cursor != "" {
			params["cursor"] = cursor
		}
		result, err := a.client.Get("/v5/market/risk-limit", params)
		if err != nil {
			log.Warn("loadMaintenanceRates: %v", err)
			return
		}

		var resp struct {
			List []struct {
				Symbol            string `json:"symbol"`
				MaintenanceMargin string `json:"maintenanceMargin"`
				IsLowestRisk      int    `json:"isLowestRisk"`
			} `json:"list"`
			NextPageCursor string `json:"nextPageCursor"`
		}
		if err := json.Unmarshal(result, &resp); err != nil {
			log.Warn("loadMaintenanceRates unmarshal: %v", err)
			return
		}

		for _, item := range resp.List {
			if item.IsLowestRisk != 1 {
				continue
			}
			mm, _ := strconv.ParseFloat(item.MaintenanceMargin, 64)
			// Bybit returns percentage: "0.5" means 0.5%. Divide by 100.
			rate := mm / 100.0
			if rate <= 0 || rate >= 1.0 {
				continue
			}
			if ci, ok := contracts[item.Symbol]; ok {
				ci.MaintenanceRate = rate
				contracts[item.Symbol] = ci
			}
		}

		if resp.NextPageCursor == "" || len(resp.List) == 0 {
			break
		}
		cursor = resp.NextPageCursor
	}
}

// GetMaintenanceRate returns the maintenance margin rate for a symbol at a given
// notional size by querying the per-symbol risk-limit endpoint.
// CRITICAL: Bybit maintenanceMargin is a PERCENTAGE. Divide by 100.
func (a *Adapter) GetMaintenanceRate(symbol string, notionalUSDT float64) (float64, error) {
	params := map[string]string{
		"category": "linear",
		"symbol":   symbol,
	}
	result, err := a.client.Get("/v5/market/risk-limit", params)
	if err != nil {
		return 0, fmt.Errorf("GetMaintenanceRate: %w", err)
	}

	var resp struct {
		List []struct {
			RiskLimitValue    string `json:"riskLimitValue"`
			MaintenanceMargin string `json:"maintenanceMargin"`
			IsLowestRisk      int    `json:"isLowestRisk"`
		} `json:"list"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return 0, fmt.Errorf("GetMaintenanceRate unmarshal: %w", err)
	}

	if len(resp.List) == 0 {
		return 0, fmt.Errorf("GetMaintenanceRate: no tiers for %s", symbol)
	}

	// For notional=0, return the lowest-risk tier
	if notionalUSDT <= 0 {
		for _, tier := range resp.List {
			if tier.IsLowestRisk == 1 {
				mm, _ := strconv.ParseFloat(tier.MaintenanceMargin, 64)
				rate := mm / 100.0
				if rate <= 0 || rate >= 1.0 {
					return 0, nil
				}
				return rate, nil
			}
		}
		// Fallback to first tier
		mm, _ := strconv.ParseFloat(resp.List[0].MaintenanceMargin, 64)
		rate := mm / 100.0
		if rate <= 0 || rate >= 1.0 {
			return 0, nil
		}
		return rate, nil
	}

	// Sort tiers by riskLimitValue and find matching tier
	// Tiers are ordered by riskLimitValue ascending
	for _, tier := range resp.List {
		limit, _ := strconv.ParseFloat(tier.RiskLimitValue, 64)
		if notionalUSDT <= limit {
			mm, _ := strconv.ParseFloat(tier.MaintenanceMargin, 64)
			rate := mm / 100.0
			if rate <= 0 || rate >= 1.0 {
				return 0, nil
			}
			return rate, nil
		}
	}

	// Exceeds all tiers: return last tier's rate
	last := resp.List[len(resp.List)-1]
	mm, _ := strconv.ParseFloat(last.MaintenanceMargin, 64)
	rate := mm / 100.0
	if rate <= 0 || rate >= 1.0 {
		return 0, nil
	}
	return rate, nil
}

// ---------- Funding Rate ----------

// GetFundingRate returns the current funding rate for a symbol.
func (a *Adapter) GetFundingRate(symbol string) (*exchange.FundingRate, error) {
	params := map[string]string{
		"category": "linear",
		"symbol":   symbol,
	}
	result, err := a.client.Get("/v5/market/tickers", params)
	if err != nil {
		return nil, fmt.Errorf("bybit GetFundingRate: %w", err)
	}

	var resp struct {
		List []struct {
			Symbol          string `json:"symbol"`
			FundingRate     string `json:"fundingRate"`
			NextFundingTime string `json:"nextFundingTime"`
		} `json:"list"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return nil, fmt.Errorf("bybit GetFundingRate parse: %w", err)
	}
	if len(resp.List) == 0 {
		return nil, fmt.Errorf("bybit GetFundingRate: no ticker for %s", symbol)
	}

	t := resp.List[0]
	rate, _ := strconv.ParseFloat(t.FundingRate, 64)
	nextMS, _ := strconv.ParseInt(t.NextFundingTime, 10, 64)
	nextTime := time.UnixMilli(nextMS)

	// Fetch per-symbol funding interval + rate caps from instruments-info
	interval := 8 * time.Hour // default
	var maxRate, minRate *float64
	instParams := map[string]string{
		"category": "linear",
		"symbol":   symbol,
	}
	if instData, instErr := a.client.Get("/v5/market/instruments-info", instParams); instErr == nil {
		var instResp struct {
			List []struct {
				FundingInterval  json.Number `json:"fundingInterval"`
				UpperFundingRate string      `json:"upperFundingRate"`
				LowerFundingRate string      `json:"lowerFundingRate"`
			} `json:"list"`
		}
		if json.Unmarshal(instData, &instResp) == nil && len(instResp.List) > 0 {
			inst := instResp.List[0]
			if mins, e := inst.FundingInterval.Float64(); e == nil {
				interval = time.Duration(mins) * time.Minute
			}
			if v, e := strconv.ParseFloat(inst.UpperFundingRate, 64); e == nil {
				maxRate = &v
			}
			if v, e := strconv.ParseFloat(inst.LowerFundingRate, 64); e == nil {
				minRate = &v
			}
		}
	}

	return &exchange.FundingRate{
		Symbol:      t.Symbol,
		Rate:        rate,
		Interval:    interval,
		NextFunding: nextTime,
		MaxRate:     maxRate,
		MinRate:     minRate,
	}, nil
}

// GetFundingInterval returns the funding interval for a symbol from instruments-info.
func (a *Adapter) GetFundingInterval(symbol string) (time.Duration, error) {
	params := map[string]string{
		"category": "linear",
		"symbol":   symbol,
	}
	result, err := a.client.Get("/v5/market/instruments-info", params)
	if err != nil {
		return 0, fmt.Errorf("bybit GetFundingInterval: %w", err)
	}

	var resp struct {
		List []struct {
			FundingInterval json.Number `json:"fundingInterval"`
		} `json:"list"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return 0, fmt.Errorf("bybit GetFundingInterval parse: %w", err)
	}
	if len(resp.List) == 0 {
		return 0, fmt.Errorf("bybit GetFundingInterval: no instrument for %s", symbol)
	}

	// fundingInterval is in minutes (may come as string or number).
	minutesF, err := resp.List[0].FundingInterval.Float64()
	if err != nil {
		return 0, fmt.Errorf("bybit GetFundingInterval parse interval: %w", err)
	}
	return time.Duration(minutesF) * time.Minute, nil
}

// ---------- Account ----------

// GetFuturesBalance returns the unified (trading) account balance.
func (a *Adapter) GetFuturesBalance() (*exchange.Balance, error) {
	params := map[string]string{
		"accountType": "UNIFIED",
		"coin":        "USDT",
	}
	result, err := a.client.Get("/v5/account/wallet-balance", params)
	if err != nil {
		return nil, fmt.Errorf("bybit GetFuturesBalance: %w", err)
	}

	var resp struct {
		List []struct {
			AccountMMRate string `json:"accountMMRate"`
			Coin          []struct {
				Coin                string `json:"coin"`
				Equity              string `json:"equity"`
				AvailableToWithdraw string `json:"availableToWithdraw"`
				Locked              string `json:"locked"`
			} `json:"coin"`
		} `json:"list"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return nil, fmt.Errorf("bybit GetFuturesBalance parse: %w", err)
	}

	if len(resp.List) == 0 || len(resp.List[0].Coin) == 0 {
		return &exchange.Balance{Currency: "USDT"}, nil
	}

	marginRatio, _ := strconv.ParseFloat(resp.List[0].AccountMMRate, 64)

	c := resp.List[0].Coin[0]
	total, _ := strconv.ParseFloat(c.Equity, 64)
	available, _ := strconv.ParseFloat(c.AvailableToWithdraw, 64)
	locked, _ := strconv.ParseFloat(c.Locked, 64)

	// Bybit unified account may report availableToWithdraw=0 even when funds
	// are fully available for trading. Fall back to equity minus locked.
	if available <= 0 && total > 0 {
		available = total - locked
	}

	// Query precise transferable amount via dedicated endpoint (availableToWithdraw deprecated for UNIFIED since 2025-01-09)
	var maxTransferOut float64
	if wdResult, wdErr := a.client.Get("/v5/account/withdrawal", map[string]string{"coinName": "USDT"}); wdErr == nil {
		var wdResp struct {
			AvailableWithdrawal string `json:"availableWithdrawal"`
		}
		if json.Unmarshal(wdResult, &wdResp) == nil && wdResp.AvailableWithdrawal != "" {
			maxTransferOut, _ = strconv.ParseFloat(wdResp.AvailableWithdrawal, 64)
		}
	}
	// If dedicated endpoint failed, leave MaxTransferOut=0 so engine uses L4 formula fallback.
	// Do NOT fallback to 'available' — it may be from deprecated availableToWithdraw (always 0 for UNIFIED)
	// or from equity-locked which overstates transferable amount.

	return &exchange.Balance{
		Total:          total,
		Available:      available,
		Frozen:         locked,
		Currency:       "USDT",
		MarginRatio:    marginRatio,
		MaxTransferOut: maxTransferOut,
	}, nil
}

// GetSpotBalance returns the funding (withdrawable) account balance.
func (a *Adapter) GetSpotBalance() (*exchange.Balance, error) {
	result, err := a.client.Get("/v5/asset/transfer/query-account-coins-balance", map[string]string{
		"accountType": "FUND",
		"coin":        "USDT",
	})
	if err != nil {
		return nil, fmt.Errorf("bybit GetSpotBalance: %w", err)
	}

	var resp struct {
		Balance []struct {
			Coin            string `json:"coin"`
			TransferBalance string `json:"transferBalance"`
			WalletBalance   string `json:"walletBalance"`
		} `json:"balance"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return nil, fmt.Errorf("bybit GetSpotBalance parse: %w", err)
	}

	for _, b := range resp.Balance {
		if b.Coin == "USDT" {
			total, _ := strconv.ParseFloat(b.WalletBalance, 64)
			available, _ := strconv.ParseFloat(b.TransferBalance, 64)
			return &exchange.Balance{
				Total:     total,
				Available: available,
				Frozen:    total - available,
				Currency:  "USDT",
			}, nil
		}
	}
	return &exchange.Balance{Currency: "USDT"}, nil
}

// ---------- Withdraw ----------

// TransferToFutures moves funds from funding account to unified trading account.
// Deposits and cross-exchange transfers land in FUND, not UNIFIED.
func (a *Adapter) TransferToFutures(coin string, amount string) error {
	reqParams := map[string]string{
		"transferId":      generateUUID(),
		"coin":            coin,
		"amount":          amount,
		"fromAccountType": "FUND",
		"toAccountType":   "UNIFIED",
	}
	_, err := a.client.Post("/v5/asset/transfer/inter-transfer", reqParams)
	if err != nil {
		return fmt.Errorf("TransferToFutures: %w", err)
	}
	return nil
}

// TransferToSpot moves funds from unified trading account to funding account.
func (a *Adapter) TransferToSpot(coin string, amount string) error {
	reqParams := map[string]string{
		"transferId":      generateUUID(),
		"coin":            coin,
		"amount":          amount,
		"fromAccountType": "UNIFIED",
		"toAccountType":   "FUND",
	}

	_, err := a.client.Post("/v5/asset/transfer/inter-transfer", reqParams)
	if err != nil {
		return fmt.Errorf("TransferToSpot: %w", err)
	}
	return nil
}

func (a *Adapter) Withdraw(params exchange.WithdrawParams) (*exchange.WithdrawResult, error) {
	chain := mapChainToBybit(params.Chain)
	reqParams := map[string]string{
		"coin":      params.Coin,
		"chain":     chain,
		"address":   params.Address,
		"amount":    params.Amount,
		"timestamp": fmt.Sprintf("%d", time.Now().UnixMilli()),
	}

	result, err := a.client.Post("/v5/asset/withdraw/create", reqParams)
	if err != nil {
		return nil, fmt.Errorf("bybit Withdraw: %w", err)
	}

	var resp struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return nil, fmt.Errorf("bybit Withdraw parse: %w", err)
	}
	return &exchange.WithdrawResult{
		TxID:   resp.ID,
		Status: "submitted",
	}, nil
}

// WithdrawFeeInclusive returns false because Bybit Withdraw amount is net (recipient gets full amount, fee deducted separately).
func (a *Adapter) WithdrawFeeInclusive() bool { return false }

// GetWithdrawFee queries the Bybit API for the withdrawal fee of a coin on a given chain.
func (a *Adapter) GetWithdrawFee(coin, chain string) (float64, error) {
	if a.client == nil || a.cfg.ApiKey == "" {
		return 0, fmt.Errorf("bybit GetWithdrawFee: API key not configured")
	}

	network := mapChainToBybitNetwork(chain)
	params := map[string]string{
		"coin": coin,
	}
	data, err := a.client.Get("/v5/asset/coin/query-info", params)
	if err != nil {
		return 0, fmt.Errorf("bybit GetWithdrawFee: %w", err)
	}

	var resp struct {
		Rows []struct {
			Coin   string `json:"coin"`
			Chains []struct {
				Chain                string `json:"chain"`
				WithdrawFee          string `json:"withdrawFee"`
				WithdrawPercentageFee string `json:"withdrawPercentageFee"`
			} `json:"chains"`
		} `json:"rows"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return 0, fmt.Errorf("bybit GetWithdrawFee unmarshal: %w", err)
	}

	for _, row := range resp.Rows {
		if !strings.EqualFold(row.Coin, coin) {
			continue
		}
		for _, ch := range row.Chains {
			if strings.EqualFold(ch.Chain, network) {
				if ch.WithdrawPercentageFee != "" {
					if pct, err := strconv.ParseFloat(ch.WithdrawPercentageFee, 64); err == nil && pct > 0 {
						return 0, fmt.Errorf("bybit GetWithdrawFee: percentage-based fee not supported (chain=%s, pct=%s)", network, ch.WithdrawPercentageFee)
					}
				}
				fee, err := strconv.ParseFloat(ch.WithdrawFee, 64)
				if err != nil {
					return 0, fmt.Errorf("bybit GetWithdrawFee parse fee: %w", err)
				}
				return fee, nil
			}
		}
		return 0, fmt.Errorf("bybit GetWithdrawFee: chain %s not found for %s", network, coin)
	}
	return 0, fmt.Errorf("bybit GetWithdrawFee: coin %s not found", coin)
}

func mapChainToBybitNetwork(chain string) string {
	switch chain {
	case "BEP20":
		return "BSC"
	case "APT":
		return "APTOS"
	default:
		return chain
	}
}

func mapChainToBybit(chain string) string {
	switch chain {
	case "BEP20":
		return "BSC"
	case "APT":
		return "APTOS"
	default:
		return chain
	}
}

// ---------- Orderbook ----------

// GetOrderbook returns the order book for a symbol.
func (a *Adapter) GetOrderbook(symbol string, depth int) (*exchange.Orderbook, error) {
	params := map[string]string{
		"category": "linear",
		"symbol":   symbol,
		"limit":    strconv.Itoa(depth),
	}
	result, err := a.client.Get("/v5/market/orderbook", params)
	if err != nil {
		return nil, fmt.Errorf("bybit GetOrderbook: %w", err)
	}

	var resp struct {
		Symbol string     `json:"s"`
		Bids   [][]string `json:"b"`
		Asks   [][]string `json:"a"`
		Ts     int64      `json:"ts"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return nil, fmt.Errorf("bybit GetOrderbook parse: %w", err)
	}

	ob := &exchange.Orderbook{
		Symbol: symbol,
		Bids:   make([]exchange.PriceLevel, 0, len(resp.Bids)),
		Asks:   make([]exchange.PriceLevel, 0, len(resp.Asks)),
		Time:   time.UnixMilli(resp.Ts),
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
	a.publicWS = NewPublicWS(&a.priceStore, &a.depthStore)
	a.publicWS.SetMetricsCallback(a.wsMetricsCallback)
	a.publicWS.Connect(symbols)
}

// SubscribeSymbol subscribes to a new symbol on the public WebSocket.
func (a *Adapter) SubscribeSymbol(symbol string) bool {
	if a.publicWS == nil {
		return false
	}
	return a.publicWS.Subscribe(symbol)
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

// SubscribeDepth subscribes to top-5 orderbook depth via the public WebSocket.
func (a *Adapter) SubscribeDepth(symbol string) bool {
	if a.publicWS == nil {
		return false
	}
	return a.publicWS.SubscribeDepth(symbol)
}

// UnsubscribeDepth unsubscribes from top-5 orderbook depth.
func (a *Adapter) UnsubscribeDepth(symbol string) bool {
	if a.publicWS == nil {
		return false
	}
	return a.publicWS.UnsubscribeDepth(symbol)
}

// GetDepth returns the latest top-5 orderbook depth snapshot.
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
	a.privateWS = NewPrivateWS(a.cfg.ApiKey, a.cfg.SecretKey, &a.orderStore, &a.orderCallback)
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

// GetUserTrades returns filled trades for a symbol since startTime.
// Bybit endpoint: GET /v5/execution/list
func (b *Adapter) GetUserTrades(symbol string, startTime time.Time, limit int) ([]exchange.Trade, error) {
	if limit <= 0 || limit > 100 {
		limit = 100 // Bybit execution list max is 100
	}
	params := map[string]string{
		"category":  "linear",
		"symbol":    symbol,
		"startTime": strconv.FormatInt(startTime.UnixMilli(), 10),
		"limit":     strconv.Itoa(limit),
	}
	raw, err := b.client.Get("/v5/execution/list", params)
	if err != nil {
		return nil, fmt.Errorf("GetUserTrades: %w", err)
	}

	var resp struct {
		List []struct {
			ExecID      string `json:"execId"`
			OrderID     string `json:"orderId"`
			Symbol      string `json:"symbol"`
			Side        string `json:"side"` // Buy or Sell
			ExecPrice   string `json:"execPrice"`
			ExecQty     string `json:"execQty"`
			ExecFee     string `json:"execFee"`
			FeeCurrency string `json:"feeCurrency"`
			ExecTime    string `json:"execTime"` // ms timestamp string
		} `json:"list"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("GetUserTrades unmarshal: %w", err)
	}

	trades := make([]exchange.Trade, 0, len(resp.List))
	for _, t := range resp.List {
		price, _ := strconv.ParseFloat(t.ExecPrice, 64)
		qty, _ := strconv.ParseFloat(t.ExecQty, 64)
		fee, _ := strconv.ParseFloat(t.ExecFee, 64)
		if fee < 0 {
			fee = -fee
		}
		ms, _ := strconv.ParseInt(t.ExecTime, 10, 64)
		trades = append(trades, exchange.Trade{
			TradeID:  t.ExecID,
			OrderID:  t.OrderID,
			Symbol:   t.Symbol,
			Side:     strings.ToLower(t.Side),
			Price:    price,
			Quantity: qty,
			Fee:      fee,
			FeeCoin:  t.FeeCurrency,
			Time:     time.UnixMilli(ms),
		})
	}
	return trades, nil
}

// GetFundingFees returns funding fee history for a symbol since the given time.
func (a *Adapter) GetFundingFees(symbol string, since time.Time) ([]exchange.FundingPayment, error) {
	var out []exchange.FundingPayment
	start := since.UTC()
	now := time.Now().UTC()
	if !start.Before(now) {
		return out, nil
	}

	for {
		end := start.Add(24 * time.Hour)
		if end.After(now) {
			end = now
		}

		params := map[string]string{
			"category":  "linear",
			"symbol":    symbol,
			"type":      "SETTLEMENT",
			"startTime": strconv.FormatInt(start.UnixMilli(), 10),
			"endTime":   strconv.FormatInt(end.UnixMilli(), 10),
			"limit":     "50",
		}
		cursor := ""
		for {
			if cursor != "" {
				params["cursor"] = cursor
			} else {
				delete(params, "cursor")
			}

			raw, err := a.client.Get("/v5/account/transaction-log", params)
			if err != nil {
				return nil, fmt.Errorf("GetFundingFees: %w", err)
			}

			var resp struct {
				List []struct {
					Funding         string `json:"funding"`
					TransactionTime string `json:"transactionTime"`
				} `json:"list"`
				NextPageCursor string `json:"nextPageCursor"`
			}
			if err := json.Unmarshal(raw, &resp); err != nil {
				return nil, fmt.Errorf("GetFundingFees unmarshal: %w", err)
			}

			for _, r := range resp.List {
				amt, _ := strconv.ParseFloat(r.Funding, 64)
				ms, _ := strconv.ParseInt(r.TransactionTime, 10, 64)
				out = append(out, exchange.FundingPayment{
					Amount: amt,
					Time:   time.UnixMilli(ms),
				})
			}

			if resp.NextPageCursor == "" || len(resp.List) == 0 || resp.NextPageCursor == cursor {
				break
			}
			cursor = resp.NextPageCursor
		}

		if !end.Before(now) {
			break
		}
		start = end.Add(time.Millisecond)
	}

	return out, nil
}

// GetClosePnL returns exchange-reported position-level PnL for recently closed positions.
// Bybit's closedPnl already includes funding fees.
func (a *Adapter) GetClosePnL(symbol string, since time.Time) ([]exchange.ClosePnL, error) {
	params := map[string]string{
		"category":  "linear",
		"symbol":    symbol,
		"startTime": strconv.FormatInt(since.UnixMilli(), 10),
		"limit":     "50",
	}
	raw, err := a.client.Get("/v5/position/closed-pnl", params)
	if err != nil {
		return nil, fmt.Errorf("GetClosePnL: %w", err)
	}

	var resp struct {
		List []struct {
			ClosedPnl     string `json:"closedPnl"`
			CumEntryValue string `json:"cumEntryValue"`
			CumExitValue  string `json:"cumExitValue"`
			AvgEntryPrice string `json:"avgEntryPrice"`
			AvgExitPrice  string `json:"avgExitPrice"`
			OpenFee       string `json:"openFee"`
			CloseFee      string `json:"closeFee"`
			ClosedSize    string `json:"closedSize"`
			Side          string `json:"side"`
			UpdatedTime   string `json:"updatedTime"`
		} `json:"list"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("GetClosePnL unmarshal: %w", err)
	}

	// Query funding fees separately — closedPnl already includes funding,
	// but we still need the Funding field for FundingCollected reconciliation.
	var totalFunding float64
	fundingFees, fErr := a.GetFundingFees(symbol, since)
	if fErr != nil {
		return nil, fmt.Errorf("GetClosePnL funding: %w", fErr)
	}
	for _, f := range fundingFees {
		totalFunding += f.Amount
	}

	out := make([]exchange.ClosePnL, 0, len(resp.List))
	for _, r := range resp.List {
		closedPnl, _ := strconv.ParseFloat(r.ClosedPnl, 64)
		cumEntry, _ := strconv.ParseFloat(r.CumEntryValue, 64)
		cumExit, _ := strconv.ParseFloat(r.CumExitValue, 64)
		openFee, _ := strconv.ParseFloat(r.OpenFee, 64)
		closeFee, _ := strconv.ParseFloat(r.CloseFee, 64)
		entryPrice, _ := strconv.ParseFloat(r.AvgEntryPrice, 64)
		exitPrice, _ := strconv.ParseFloat(r.AvgExitPrice, 64)
		closeSize, _ := strconv.ParseFloat(r.ClosedSize, 64)
		ms, _ := strconv.ParseInt(r.UpdatedTime, 10, 64)

		// Normalize side: Bybit uses "Buy"/"Sell" for the close order side.
		// "Buy" close = was short, "Sell" close = was long.
		side := "long"
		if r.Side == "Buy" {
			side = "short"
		}

		// closedPnl = (cumExitValue - cumEntryValue) - openFee - closeFee (net of fees)
		// pricePnL = cumExitValue - cumEntryValue (raw price movement)
		pricePnL := cumExit - cumEntry

		// closedPnl already includes funding, so NetPnL = closedPnl as-is.
		// Funding is queried separately for FundingCollected reconciliation.
		out = append(out, exchange.ClosePnL{
			PricePnL:   pricePnL,
			Fees:       openFee + closeFee,
			Funding:    totalFunding,
			NetPnL:     closedPnl,
			EntryPrice: entryPrice,
			ExitPrice:  exitPrice,
			CloseSize:  closeSize,
			Side:       side,
			CloseTime:  time.UnixMilli(ms),
		})
	}
	return out, nil
}

// PlaceStopLoss places a conditional stop order on Bybit V5.
func (a *Adapter) PlaceStopLoss(params exchange.StopLossParams) (string, error) {
	// triggerDirection: 2 = triggered when price falls below (long SL),
	// 1 = triggered when price rises above (short SL).
	triggerDir := "2"
	if params.Side == exchange.SideBuy {
		triggerDir = "1"
	}

	p := map[string]string{
		"category":         "linear",
		"symbol":           params.Symbol,
		"side":             toBybitSide(params.Side),
		"orderType":        "Market",
		"qty":              params.Size,
		"triggerPrice":     params.TriggerPrice,
		"triggerDirection": triggerDir,
		"triggerBy":        "MarkPrice",
		"orderFilter":      "StopOrder",
		"timeInForce":      "GTC",
		"reduceOnly":       "true",
	}

	result, err := a.client.Post("/v5/order/create", p)
	if err != nil {
		return "", fmt.Errorf("bybit PlaceStopLoss: %w", err)
	}

	var resp struct {
		OrderID string `json:"orderId"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return "", fmt.Errorf("bybit PlaceStopLoss parse: %w", err)
	}
	return resp.OrderID, nil
}

// PlaceTakeProfit places a take-profit conditional order on Bybit V5.
func (a *Adapter) PlaceTakeProfit(params exchange.TakeProfitParams) (string, error) {
	// triggerDirection for TP is opposite of SL:
	// 1 = triggered when price rises above (long TP — sell when price goes up),
	// 2 = triggered when price falls below (short TP — buy when price goes down).
	triggerDir := "1"
	if params.Side == exchange.SideBuy {
		triggerDir = "2"
	}

	p := map[string]string{
		"category":         "linear",
		"symbol":           params.Symbol,
		"side":             toBybitSide(params.Side),
		"orderType":        "Market",
		"qty":              params.Size,
		"triggerPrice":     params.TriggerPrice,
		"triggerDirection": triggerDir,
		"triggerBy":        "MarkPrice",
		"orderFilter":      "StopOrder",
		"timeInForce":      "GTC",
		"reduceOnly":       "true",
	}

	result, err := a.client.Post("/v5/order/create", p)
	if err != nil {
		return "", fmt.Errorf("bybit PlaceTakeProfit: %w", err)
	}

	var resp struct {
		OrderID string `json:"orderId"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return "", fmt.Errorf("bybit PlaceTakeProfit parse: %w", err)
	}
	return resp.OrderID, nil
}

// CancelTakeProfit cancels a conditional take-profit order on Bybit V5.
func (a *Adapter) CancelTakeProfit(symbol, orderID string) error {
	return a.CancelStopLoss(symbol, orderID)
}

// CancelStopLoss cancels a conditional stop order on Bybit V5.
func (a *Adapter) CancelStopLoss(symbol, orderID string) error {
	params := map[string]string{
		"category":    "linear",
		"symbol":      symbol,
		"orderId":     orderID,
		"orderFilter": "StopOrder",
	}
	_, err := a.client.Post("/v5/order/cancel", params)
	if err != nil {
		if apiErr, ok := err.(*APIError); ok {
			if apiErr.Code == 110001 || apiErr.Code == 170213 {
				return nil
			}
		}
		return fmt.Errorf("bybit CancelStopLoss: %w", err)
	}
	return nil
}

// CancelAllOrders cancels all open orders (regular + conditional/algo) for a symbol.
func (a *Adapter) CancelAllOrders(symbol string) error {
	a.client.Post("/v5/order/cancel-all", map[string]string{
		"category": "linear", "symbol": symbol,
	})
	return nil
}

// EnsureOneWayMode sets the account to one-way (MergedSingle) position mode.
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
	params := map[string]string{
		"category": "linear",
		"mode":     "0",
		"coin":     "USDT",
	}
	_, err := a.client.Post("/v5/position/switch-mode", params)
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "not modified") || strings.Contains(errMsg, "110025") {
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

// isOneWayMode checks if Bybit is currently in MergedSingle (one-way) mode.
func (a *Adapter) isOneWayMode() bool {
	data, err := a.client.Get("/v5/position/list", map[string]string{
		"category": "linear",
		"symbol":   "BTCUSDT",
		"limit":    "1",
	})
	if err != nil {
		return false
	}
	// If positions use "Both" positionIdx=0, it's one-way mode.
	// In hedge mode, positions have positionIdx 1 (Buy) or 2 (Sell).
	var resp struct {
		List []struct {
			PositionIdx int `json:"positionIdx"`
		} `json:"list"`
	}
	if json.Unmarshal(data, &resp) == nil {
		// No positions or positionIdx=0 means one-way mode.
		if len(resp.List) == 0 || resp.List[0].PositionIdx == 0 {
			return true
		}
	}
	return false
}

// GetTradingFee returns the authenticated user's maker/taker fee rates for linear perpetuals.
func (a *Adapter) GetTradingFee() (*exchange.TradingFee, error) {
	params := map[string]string{
		"category": "linear",
		"symbol":   "BTCUSDT",
	}
	result, err := a.client.Get("/v5/account/fee-rate", params)
	if err != nil {
		return nil, fmt.Errorf("bybit GetTradingFee: %w", err)
	}

	var resp struct {
		List []struct {
			MakerFeeRate string `json:"makerFeeRate"`
			TakerFeeRate string `json:"takerFeeRate"`
		} `json:"list"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return nil, fmt.Errorf("bybit GetTradingFee unmarshal: %w", err)
	}
	if len(resp.List) == 0 {
		return nil, fmt.Errorf("bybit GetTradingFee: empty fee rate list")
	}

	maker, err := strconv.ParseFloat(resp.List[0].MakerFeeRate, 64)
	if err != nil {
		return nil, fmt.Errorf("bybit GetTradingFee parse maker: %w", err)
	}
	taker, err := strconv.ParseFloat(resp.List[0].TakerFeeRate, 64)
	if err != nil {
		return nil, fmt.Errorf("bybit GetTradingFee parse taker: %w", err)
	}

	return &exchange.TradingFee{
		MakerRate: maker,
		TakerRate: taker,
	}, nil
}
