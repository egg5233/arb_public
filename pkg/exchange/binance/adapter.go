package binance

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"arb/pkg/exchange"

	"github.com/gorilla/websocket"
)

// Compile-time check that *Adapter satisfies exchange.Exchange.
var _ exchange.Exchange = (*Adapter)(nil)
var _ exchange.TradingFeeProvider = (*Adapter)(nil)
var _ exchange.TradFiSigner = (*Adapter)(nil)

// Adapter implements the exchange.Exchange interface for Binance USDT-M Futures.
type Adapter struct {
	client    *Client
	apiKey    string
	secretKey string

	// Price stream
	priceStore sync.Map // symbol -> exchange.BBO
	priceConn  *websocket.Conn
	priceMu    sync.Mutex
	priceSyms  map[string]bool // tracked symbols

	// Depth stream (top-5 orderbook via WS)
	depthStore sync.Map        // symbol -> *exchange.Orderbook
	depthSyms  map[string]bool // symbols with active depth subscriptions

	// Private stream
	listenKey            string
	privConn             *websocket.Conn
	orderStore           sync.Map // orderID (string) -> exchange.OrderUpdate
	orderCallback        func(exchange.OrderUpdate)
	wsMetricsCallback    exchange.WSMetricsCallback
	orderMetricsCallback exchange.OrderMetricsCallback
	algoRemapCallback    exchange.AlgoRemapCallback
	algoRemapMu          sync.Mutex

	isUnified bool // true when Portfolio Margin is enabled
}

func (b *Adapter) SetMetricsCallback(fn exchange.MetricsCallback) {
	if b.client != nil {
		b.client.SetMetricsCallback(fn)
	}
}

func (b *Adapter) SetWSMetricsCallback(fn exchange.WSMetricsCallback) {
	b.wsMetricsCallback = fn
}

func (b *Adapter) SetOrderMetricsCallback(fn exchange.OrderMetricsCallback) {
	b.orderMetricsCallback = fn
}

func (b *Adapter) SetOrderCallback(fn func(exchange.OrderUpdate)) {
	b.orderCallback = fn
}

// SetAlgoRemapCallback registers a callback that fires when a Binance algo order
// (TAKE_PROFIT / STOP_MARKET conditional) is triggered and mapped to a matching-engine order ID.
func (b *Adapter) SetAlgoRemapCallback(fn exchange.AlgoRemapCallback) {
	b.algoRemapMu.Lock()
	b.algoRemapCallback = fn
	b.algoRemapMu.Unlock()
}

// getAlgoRemapCallback returns the current algoRemapCallback under the mutex.
func (b *Adapter) getAlgoRemapCallback() exchange.AlgoRemapCallback {
	b.algoRemapMu.Lock()
	defer b.algoRemapMu.Unlock()
	return b.algoRemapCallback
}

func (b *Adapter) SignTradFi() error {
	_, err := b.client.Post("/fapi/v1/stock/contract", map[string]string{})
	return err
}

func (b *Adapter) CheckPermissions() exchange.PermissionResult {
	// Must use api.binance.com (spot), not fapi.binance.com (futures).
	spotClient := b.client.WithBaseURL("https://api.binance.com")
	params := map[string]string{}
	data, err := spotClient.Get("/sapi/v1/account/apiRestrictions", params)
	if err != nil {
		return exchange.PermissionResult{Method: "direct", Error: err.Error(),
			Read: exchange.PermUnknown, FuturesTrade: exchange.PermUnknown,
			Withdraw: exchange.PermUnknown, Transfer: exchange.PermUnknown}
	}
	var resp struct {
		EnableReading            bool `json:"enableReading"`
		EnableFutures            bool `json:"enableFutures"`
		EnableWithdrawals        bool `json:"enableWithdrawals"`
		EnableInternalTransfer   bool `json:"enableInternalTransfer"`
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
		Withdraw: toBool(resp.EnableWithdrawals),
		Transfer: toBool(resp.EnableInternalTransfer || resp.PermitsUniversalTransfer),
		Method: "direct",
	}
}

// IsUnified returns true when the account has Portfolio Margin enabled.
func (b *Adapter) IsUnified() bool { return b.isUnified }

// DetectPortfolioMargin probes /sapi/v1/account/apiRestrictions and sets isUnified
// if enablePortfolioMarginTrading is true. Safe to call multiple times.
func (b *Adapter) DetectPortfolioMargin() {
	spotClient := b.client.WithBaseURL("https://api.binance.com")
	data, err := spotClient.Get("/sapi/v1/account/apiRestrictions", map[string]string{})
	if err != nil {
		return
	}
	var resp struct {
		EnablePortfolioMarginTrading bool `json:"enablePortfolioMarginTrading"`
	}
	if json.Unmarshal(data, &resp) == nil && resp.EnablePortfolioMarginTrading {
		b.isUnified = true
	}
}

// NewAdapter creates a Binance Adapter from ExchangeConfig.
func NewAdapter(cfg exchange.ExchangeConfig) *Adapter {
	return &Adapter{
		client:    NewClient(cfg.ApiKey, cfg.SecretKey),
		apiKey:    cfg.ApiKey,
		secretKey: cfg.SecretKey,
		priceSyms: make(map[string]bool),
		depthSyms: make(map[string]bool),
	}
}

func (b *Adapter) Name() string { return "binance" }

// ---------------------------------------------------------------------------
// Orders
// ---------------------------------------------------------------------------

func (b *Adapter) PlaceOrder(req exchange.PlaceOrderParams) (string, error) {
	log.Printf("[binance] PlaceOrder: symbol=%s side=%s type=%s size=%s price=%s force=%s reduceOnly=%v",
		req.Symbol, req.Side, req.OrderType, req.Size, req.Price, req.Force, req.ReduceOnly)
	params := map[string]string{
		"symbol":   req.Symbol,
		"side":     mapSide(req.Side),
		"type":     mapOrderType(req.OrderType),
		"quantity": req.Size,
	}
	if req.OrderType == "limit" {
		params["price"] = req.Price
		params["timeInForce"] = mapTimeInForce(req.Force)
	}
	if req.ReduceOnly {
		params["reduceOnly"] = "true"
	}
	if req.ClientOid != "" {
		params["newClientOrderId"] = req.ClientOid
	}

	body, err := b.client.Post("/fapi/v1/order", params)
	if err != nil {
		return "", fmt.Errorf("PlaceOrder: %w", err)
	}

	var resp struct {
		OrderID       int64  `json:"orderId"`
		ClientOrderID string `json:"clientOrderId"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("PlaceOrder unmarshal: %w", err)
	}
	orderID := strconv.FormatInt(resp.OrderID, 10)
	if b.orderMetricsCallback != nil {
		b.orderMetricsCallback(exchange.OrderMetricEvent{
			Type:      exchange.OrderMetricPlaced,
			OrderID:   orderID,
			Timestamp: time.Now(),
		})
	}
	return orderID, nil
}

func (b *Adapter) CancelOrder(symbol, orderID string) error {
	params := map[string]string{
		"symbol":  symbol,
		"orderId": orderID,
	}
	_, err := b.client.Delete("/fapi/v1/order", params)
	if err != nil {
		// Ignore "Unknown order" -- already cancelled or filled
		if isAPIError(err, -2011) {
			return nil
		}
		return fmt.Errorf("CancelOrder: %w", err)
	}
	return nil
}

func (b *Adapter) GetPendingOrders(symbol string) ([]exchange.Order, error) {
	params := map[string]string{"symbol": symbol}
	body, err := b.client.Get("/fapi/v1/openOrders", params)
	if err != nil {
		return nil, fmt.Errorf("GetPendingOrders: %w", err)
	}

	var raw []struct {
		OrderID       int64  `json:"orderId"`
		ClientOrderID string `json:"clientOrderId"`
		Symbol        string `json:"symbol"`
		Side          string `json:"side"`
		Type          string `json:"type"`
		Price         string `json:"price"`
		OrigQty       string `json:"origQty"`
		Status        string `json:"status"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("GetPendingOrders unmarshal: %w", err)
	}

	out := make([]exchange.Order, 0, len(raw))
	for _, o := range raw {
		// Skip stop orders
		if o.Type == "STOP_MARKET" || o.Type == "TAKE_PROFIT_MARKET" {
			continue
		}
		out = append(out, exchange.Order{
			OrderID:   strconv.FormatInt(o.OrderID, 10),
			ClientOid: o.ClientOrderID,
			Symbol:    o.Symbol,
			Side:      strings.ToLower(o.Side),
			OrderType: strings.ToLower(o.Type),
			Price:     o.Price,
			Size:      o.OrigQty,
			Status:    o.Status,
		})
	}
	return out, nil
}

func (b *Adapter) GetOrderFilledQty(orderID, symbol string) (float64, error) {
	params := map[string]string{
		"symbol":  symbol,
		"orderId": orderID,
	}
	body, err := b.client.Get("/fapi/v1/order", params)
	if err != nil {
		return 0, fmt.Errorf("GetOrderFilledQty: %w", err)
	}

	var resp struct {
		ExecutedQty string `json:"executedQty"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return 0, fmt.Errorf("GetOrderFilledQty unmarshal: %w", err)
	}
	qty, _ := strconv.ParseFloat(resp.ExecutedQty, 64)
	if qty > 0 && b.orderMetricsCallback != nil {
		b.orderMetricsCallback(exchange.OrderMetricEvent{
			Type:      exchange.OrderMetricFilled,
			OrderID:   orderID,
			FilledQty: qty,
			Timestamp: time.Now(),
		})
	}
	return qty, nil
}

// ---------------------------------------------------------------------------
// Positions
// ---------------------------------------------------------------------------

func (b *Adapter) GetPosition(symbol string) ([]exchange.Position, error) {
	params := map[string]string{"symbol": symbol}
	return b.fetchPositions(params)
}

func (b *Adapter) GetAllPositions() ([]exchange.Position, error) {
	positions, err := b.fetchPositions(nil)
	if err != nil {
		return nil, err
	}
	// Filter to only non-zero positions
	out := make([]exchange.Position, 0)
	for _, p := range positions {
		amt, _ := strconv.ParseFloat(p.Total, 64)
		if amt != 0 {
			out = append(out, p)
		}
	}
	return out, nil
}

func (b *Adapter) fetchPositions(params map[string]string) ([]exchange.Position, error) {
	body, err := b.client.Get("/fapi/v2/positionRisk", params)
	if err != nil {
		return nil, fmt.Errorf("GetPosition: %w", err)
	}

	var raw []struct {
		Symbol           string `json:"symbol"`
		PositionAmt      string `json:"positionAmt"`
		EntryPrice       string `json:"entryPrice"`
		UnRealizedProfit string `json:"unRealizedProfit"`
		Leverage         string `json:"leverage"`
		MarginType       string `json:"marginType"`
		LiquidationPrice string `json:"liquidationPrice"`
		MarkPrice        string `json:"markPrice"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("GetPosition unmarshal: %w", err)
	}

	out := make([]exchange.Position, 0, len(raw))
	for _, p := range raw {
		amt, _ := strconv.ParseFloat(p.PositionAmt, 64)
		holdSide := "long"
		if amt < 0 {
			holdSide = "short"
		}
		absAmt := strconv.FormatFloat(math.Abs(amt), 'f', -1, 64)

		marginMode := "crossed"
		if strings.ToLower(p.MarginType) == "isolated" {
			marginMode = "isolated"
		}

		out = append(out, exchange.Position{
			Symbol:           p.Symbol,
			HoldSide:         holdSide,
			Total:            absAmt,
			Available:        absAmt,
			AverageOpenPrice: p.EntryPrice,
			UnrealizedPL:     p.UnRealizedProfit,
			Leverage:         p.Leverage,
			MarginMode:       marginMode,
			LiquidationPrice: p.LiquidationPrice,
			MarkPrice:        p.MarkPrice,
		})
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Account Config
// ---------------------------------------------------------------------------

func (b *Adapter) SetLeverage(symbol string, leverage string, holdSide string) error {
	params := map[string]string{
		"symbol":   symbol,
		"leverage": leverage,
	}
	_, err := b.client.Post("/fapi/v1/leverage", params)
	if err != nil {
		return fmt.Errorf("SetLeverage: %w", err)
	}
	return nil
}

func (b *Adapter) SetMarginMode(symbol string, mode string) error {
	binanceMode := "CROSSED"
	if strings.ToLower(mode) == "isolated" {
		binanceMode = "ISOLATED"
	}

	params := map[string]string{
		"symbol":     symbol,
		"marginType": binanceMode,
	}
	_, err := b.client.Post("/fapi/v1/marginType", params)
	if err != nil {
		// -4046: "No need to change margin type" -- already set
		if isAPIError(err, -4046) {
			return nil
		}
		if isAPIError(err, -4067) {
			// Open orders blocking margin type change — cancel and retry
			b.CancelAllOrders(symbol)
			_, err = b.client.Post("/fapi/v1/marginType", params)
			if err != nil && isAPIError(err, -4046) {
				return nil
			}
			return err
		}
		return fmt.Errorf("SetMarginMode: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Contract Info
// ---------------------------------------------------------------------------

func (b *Adapter) LoadAllContracts() (map[string]exchange.ContractInfo, error) {
	body, err := b.client.Get("/fapi/v1/exchangeInfo", nil)
	if err != nil {
		return nil, fmt.Errorf("LoadAllContracts: %w", err)
	}

	var resp struct {
		Symbols []struct {
			Symbol       string `json:"symbol"`
			Status       string `json:"status"`
			ContractType string `json:"contractType"`
			DeliveryDate int64  `json:"deliveryDate"`
			Filters      []struct {
				FilterType string `json:"filterType"`
				MinQty     string `json:"minQty"`
				MaxQty     string `json:"maxQty"`
				StepSize   string `json:"stepSize"`
				TickSize   string `json:"tickSize"`
			} `json:"filters"`
		} `json:"symbols"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("LoadAllContracts unmarshal: %w", err)
	}

	// Year-2099 cutoff (ms since epoch): Binance uses 4133404800000
	// (2100-01-01) as the "no scheduled delivery" sentinel for live perpetuals.
	// Anything below this cutoff is treated as a real scheduled delist.
	const deliveryDateSentinelCutoffMs int64 = 4102444800000 // 2099-12-31 UTC

	result := make(map[string]exchange.ContractInfo, len(resp.Symbols))
	for _, sym := range resp.Symbols {
		if sym.Status != "TRADING" {
			continue
		}
		ci := exchange.ContractInfo{Symbol: sym.Symbol}
		// Flag scheduled delist via deliveryDate ONLY for true perpetuals.
		// Dated quarterlies have contractType like "CURRENT_QUARTER" — skip
		// them so this field means "perpetual with scheduled delist".
		if sym.ContractType == "PERPETUAL" &&
			sym.DeliveryDate > 0 &&
			sym.DeliveryDate < deliveryDateSentinelCutoffMs {
			ci.DeliveryDate = time.UnixMilli(sym.DeliveryDate).UTC()
		}
		for _, f := range sym.Filters {
			switch f.FilterType {
			case "LOT_SIZE":
				ci.MinSize, _ = strconv.ParseFloat(f.MinQty, 64)
				ci.MaxSize, _ = strconv.ParseFloat(f.MaxQty, 64)
				ci.StepSize, _ = strconv.ParseFloat(f.StepSize, 64)
				ci.SizeDecimals = countDecimals(f.StepSize)
			case "PRICE_FILTER":
				ci.PriceStep, _ = strconv.ParseFloat(f.TickSize, 64)
				ci.PriceDecimals = countDecimals(f.TickSize)
			}
		}
		result[sym.Symbol] = ci
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("no contracts loaded from Binance exchangeInfo")
	}

	// Load tier-1 maintenance rates from leverageBracket (authenticated)
	b.loadMaintenanceRates(result)

	return result, nil
}

// ---------------------------------------------------------------------------
// Maintenance Rate
// ---------------------------------------------------------------------------

// loadMaintenanceRates fetches leverageBracket data for all symbols and populates
// the first bracket's maintMarginRatio in each ContractInfo.
// NOTE: leverageBracket is a USER_DATA endpoint requiring authentication.
func (b *Adapter) loadMaintenanceRates(contracts map[string]exchange.ContractInfo) {
	body, err := b.client.Get("/fapi/v1/leverageBracket", map[string]string{})
	if err != nil {
		log.Printf("[binance] loadMaintenanceRates: %v", err)
		return
	}

	var brackets []struct {
		Symbol   string `json:"symbol"`
		Brackets []struct {
			Bracket          int     `json:"bracket"`
			NotionalCap      float64 `json:"notionalCap"`
			NotionalFloor    float64 `json:"notionalFloor"`
			MaintMarginRatio float64 `json:"maintMarginRatio"`
			Cum              float64 `json:"cum"`
		} `json:"brackets"`
	}
	if err := json.Unmarshal(body, &brackets); err != nil {
		log.Printf("[binance] loadMaintenanceRates unmarshal: %v", err)
		return
	}

	for _, item := range brackets {
		ci, ok := contracts[item.Symbol]
		if !ok || len(item.Brackets) == 0 {
			continue
		}
		// First bracket (lowest notional) maintenance rate
		rate := item.Brackets[0].MaintMarginRatio
		if rate > 0 && rate < 1.0 {
			ci.MaintenanceRate = rate
			contracts[item.Symbol] = ci
		}
	}
}

// GetMaintenanceRate returns the maintenance margin rate for a symbol at a given
// notional size by querying the authenticated leverageBracket endpoint.
// Binance maintMarginRatio is already decimal (0.0065 = 0.65%).
func (b *Adapter) GetMaintenanceRate(symbol string, notionalUSDT float64) (float64, error) {
	params := map[string]string{
		"symbol": symbol,
	}
	body, err := b.client.Get("/fapi/v1/leverageBracket", params)
	if err != nil {
		return 0, fmt.Errorf("GetMaintenanceRate: %w", err)
	}

	var brackets []struct {
		Symbol   string `json:"symbol"`
		Brackets []struct {
			Bracket          int     `json:"bracket"`
			NotionalCap      float64 `json:"notionalCap"`
			NotionalFloor    float64 `json:"notionalFloor"`
			MaintMarginRatio float64 `json:"maintMarginRatio"`
			Cum              float64 `json:"cum"`
		} `json:"brackets"`
	}
	if err := json.Unmarshal(body, &brackets); err != nil {
		return 0, fmt.Errorf("GetMaintenanceRate unmarshal: %w", err)
	}

	if len(brackets) == 0 || len(brackets[0].Brackets) == 0 {
		return 0, fmt.Errorf("GetMaintenanceRate: no brackets for %s", symbol)
	}

	bkts := brackets[0].Brackets

	// For notional=0, return the first (lowest) bracket
	if notionalUSDT <= 0 {
		rate := bkts[0].MaintMarginRatio
		if rate <= 0 || rate >= 1.0 {
			return 0, nil
		}
		return rate, nil
	}

	// Find bracket where notionalFloor <= notionalUSDT < notionalCap
	for _, bkt := range bkts {
		if notionalUSDT >= bkt.NotionalFloor && notionalUSDT < bkt.NotionalCap {
			rate := bkt.MaintMarginRatio
			if rate <= 0 || rate >= 1.0 {
				return 0, nil
			}
			return rate, nil
		}
	}

	// Exceeds all brackets: return last bracket's rate
	rate := bkts[len(bkts)-1].MaintMarginRatio
	if rate <= 0 || rate >= 1.0 {
		return 0, nil
	}
	return rate, nil
}

// ---------------------------------------------------------------------------
// Funding Rate
// ---------------------------------------------------------------------------

func (b *Adapter) GetFundingRate(symbol string) (*exchange.FundingRate, error) {
	params := map[string]string{"symbol": symbol}
	body, err := b.client.Get("/fapi/v1/premiumIndex", params)
	if err != nil {
		return nil, fmt.Errorf("GetFundingRate: %w", err)
	}

	var resp struct {
		Symbol               string `json:"symbol"`
		LastFundingRate      string `json:"lastFundingRate"`
		NextFundingTime      int64  `json:"nextFundingTime"`
		InterestRate         string `json:"interestRate"`
		EstimatedSettlePrice string `json:"estimatedSettlePrice"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("GetFundingRate unmarshal: %w", err)
	}

	rate, _ := strconv.ParseFloat(resp.LastFundingRate, 64)
	nextFunding := time.UnixMilli(resp.NextFundingTime)

	// Get funding interval and rate caps from fundingInfo
	fi, err := b.getFundingInfo(symbol)
	if err != nil {
		// Default to 8 hours if we can't determine the interval
		fi = &fundingInfo{Interval: 8 * time.Hour}
	}

	return &exchange.FundingRate{
		Symbol:      resp.Symbol,
		Rate:        rate,
		Interval:    fi.Interval,
		NextFunding: nextFunding,
		MaxRate:     fi.MaxRate,
		MinRate:     fi.MinRate,
	}, nil
}

// fundingInfo holds parsed data from /fapi/v1/fundingInfo for a single symbol.
type fundingInfo struct {
	Interval time.Duration
	MaxRate  *float64 // adjustedFundingRateCap (per-period decimal), nil if absent
	MinRate  *float64 // adjustedFundingRateFloor (per-period decimal), nil if absent
}

// getFundingInfo fetches interval and rate caps from /fapi/v1/fundingInfo.
func (b *Adapter) getFundingInfo(symbol string) (*fundingInfo, error) {
	body, err := b.client.Get("/fapi/v1/fundingInfo", nil)
	if err != nil {
		return nil, fmt.Errorf("getFundingInfo: %w", err)
	}

	var infos []struct {
		Symbol                   string `json:"symbol"`
		FundingIntervalHours     int    `json:"fundingIntervalHours"`
		AdjustedFundingRateCap   string `json:"adjustedFundingRateCap"`
		AdjustedFundingRateFloor string `json:"adjustedFundingRateFloor"`
	}
	if err := json.Unmarshal(body, &infos); err != nil {
		return nil, fmt.Errorf("getFundingInfo unmarshal: %w", err)
	}

	for _, info := range infos {
		if info.Symbol == symbol {
			fi := &fundingInfo{Interval: 8 * time.Hour}
			if info.FundingIntervalHours > 0 {
				fi.Interval = time.Duration(info.FundingIntervalHours) * time.Hour
			}
			if info.AdjustedFundingRateCap != "" {
				v, err := strconv.ParseFloat(info.AdjustedFundingRateCap, 64)
				if err == nil {
					fi.MaxRate = &v
				}
			}
			if info.AdjustedFundingRateFloor != "" {
				v, err := strconv.ParseFloat(info.AdjustedFundingRateFloor, 64)
				if err == nil {
					fi.MinRate = &v
				}
			}
			return fi, nil
		}
	}

	// Symbol not found — return defaults
	return &fundingInfo{Interval: 8 * time.Hour}, nil
}

func (b *Adapter) GetFundingInterval(symbol string) (time.Duration, error) {
	fi, err := b.getFundingInfo(symbol)
	if err != nil {
		return 0, err
	}
	return fi.Interval, nil
}

// ---------------------------------------------------------------------------
// Balance
// ---------------------------------------------------------------------------

func (b *Adapter) GetFuturesBalance() (*exchange.Balance, error) {
	body, err := b.client.Get("/fapi/v2/account", nil)
	if err != nil {
		return nil, fmt.Errorf("GetFuturesBalance: %w", err)
	}

	var resp struct {
		TotalMarginBalance string `json:"totalMarginBalance"`
		TotalMaintMargin   string `json:"totalMaintMargin"`
		AvailableBalance   string `json:"availableBalance"`
		MaxWithdrawAmount string `json:"maxWithdrawAmount"`
		Assets            []struct {
			Asset              string `json:"asset"`
			WalletBalance      string `json:"walletBalance"`
			MarginBalance      string `json:"marginBalance"`
			AvailableBalance   string `json:"availableBalance"`
			MaxWithdrawAmount  string `json:"maxWithdrawAmount"`
		} `json:"assets"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("GetFuturesBalance unmarshal: %w", err)
	}

	// Compute margin ratio from account-level fields
	var marginRatio float64
	marginBal, _ := strconv.ParseFloat(resp.TotalMarginBalance, 64)
	maintMargin, _ := strconv.ParseFloat(resp.TotalMaintMargin, 64)
	if marginBal > 0 {
		marginRatio = maintMargin / marginBal
	}

	for _, asset := range resp.Assets {
		if asset.Asset == "USDT" {
			// Use marginBalance (= walletBalance + unrealizedPnL) for Total
			// so the dashboard reflects true equity including open position PnL.
			total, _ := strconv.ParseFloat(asset.MarginBalance, 64)
			available, _ := strconv.ParseFloat(asset.AvailableBalance, 64)

			// Defensive: if availableBalance is 0 but wallet has funds, fall back.
			if available <= 0 && total > 0 {
				available = total
			}

			maxTransferOut, _ := strconv.ParseFloat(asset.MaxWithdrawAmount, 64)

			return &exchange.Balance{
				Total:                       total,
				Available:                   available,
				Frozen:                      total - available,
				Currency:                    "USDT",
				MarginRatio:                 marginRatio,
				MaxTransferOut:              maxTransferOut,
				MaxTransferOutAuthoritative: true,
			}, nil
		}
	}
	return &exchange.Balance{Currency: "USDT", MarginRatio: marginRatio}, nil
}

func (b *Adapter) GetSpotBalance() (*exchange.Balance, error) {
	body, err := b.client.SpotGet("/sapi/v1/capital/config/getall", nil)
	if err != nil {
		return nil, fmt.Errorf("GetSpotBalance: %w", err)
	}
	var coins []struct {
		Coin   string `json:"coin"`
		Free   string `json:"free"`
		Locked string `json:"locked"`
	}
	if err := json.Unmarshal(body, &coins); err != nil {
		return nil, fmt.Errorf("GetSpotBalance unmarshal: %w", err)
	}
	for _, c := range coins {
		if strings.EqualFold(c.Coin, "USDT") {
			free, _ := strconv.ParseFloat(c.Free, 64)
			locked, _ := strconv.ParseFloat(c.Locked, 64)
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

// ---------------------------------------------------------------------------
// Orderbook
// ---------------------------------------------------------------------------

func (b *Adapter) GetOrderbook(symbol string, depth int) (*exchange.Orderbook, error) {
	if depth <= 0 {
		depth = 20
	}
	params := map[string]string{
		"symbol": symbol,
		"limit":  strconv.Itoa(depth),
	}
	body, err := b.client.Get("/fapi/v1/depth", params)
	if err != nil {
		return nil, fmt.Errorf("GetOrderbook: %w", err)
	}

	var resp struct {
		Bids [][]json.RawMessage `json:"bids"` // [[price, qty], ...]
		Asks [][]json.RawMessage `json:"asks"`
		T    int64               `json:"T"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("GetOrderbook unmarshal: %w", err)
	}

	parseLevels := func(raw [][]json.RawMessage) []exchange.PriceLevel {
		levels := make([]exchange.PriceLevel, 0, len(raw))
		for _, entry := range raw {
			if len(entry) < 2 {
				continue
			}
			var priceStr, qtyStr string
			if json.Unmarshal(entry[0], &priceStr) != nil {
				continue
			}
			if json.Unmarshal(entry[1], &qtyStr) != nil {
				continue
			}
			price, _ := strconv.ParseFloat(priceStr, 64)
			qty, _ := strconv.ParseFloat(qtyStr, 64)
			levels = append(levels, exchange.PriceLevel{Price: price, Quantity: qty})
		}
		return levels
	}

	ob := &exchange.Orderbook{
		Symbol: symbol,
		Bids:   parseLevels(resp.Bids),
		Asks:   parseLevels(resp.Asks),
		Time:   time.UnixMilli(resp.T),
	}
	return ob, nil
}

// ---------------------------------------------------------------------------
// Internal Transfer / Withdraw
// ---------------------------------------------------------------------------

// TransferToSpot moves funds from the futures (USDT-M) account to the spot account.
func (b *Adapter) TransferToSpot(coin string, amount string) error {
	params := map[string]string{
		"asset":  coin,
		"amount": amount,
		"type":   "UMFUTURE_MAIN",
	}
	_, err := b.client.SpotPost("/sapi/v1/asset/transfer", params)
	if err != nil {
		return fmt.Errorf("TransferToSpot: %w", err)
	}
	return nil
}

// TransferToFutures moves funds from spot to futures account.
func (b *Adapter) TransferToFutures(coin string, amount string) error {
	params := map[string]string{
		"asset":  coin,
		"amount": amount,
		"type":   "1", // 1 = spot -> futures
	}
	_, err := b.client.SpotPost("/sapi/v1/futures/transfer", params)
	if err != nil {
		return fmt.Errorf("TransferToFutures: %w", err)
	}
	return nil
}

func (b *Adapter) Withdraw(params exchange.WithdrawParams) (*exchange.WithdrawResult, error) {
	// Check spot balance first; only transfer from futures if spot is insufficient
	amt, _ := strconv.ParseFloat(params.Amount, 64)
	spotBal := b.getSpotAvailable(params.Coin)
	if spotBal < amt {
		need := fmt.Sprintf("%.8f", amt-spotBal)
		transferParams := map[string]string{
			"asset":  params.Coin,
			"amount": need,
			"type":   "2", // 2 = futures -> spot
		}
		_, err := b.client.SpotPost("/sapi/v1/futures/transfer", transferParams)
		if err != nil {
			return nil, fmt.Errorf("Withdraw (futures->spot transfer): %w", err)
		}
	}

	network := mapChainToNetwork(params.Chain)
	reqParams := map[string]string{
		"coin":    params.Coin,
		"network": network,
		"address": params.Address,
		"amount":  params.Amount,
	}

	body, err := b.client.SpotPost("/sapi/v1/capital/withdraw/apply", reqParams)
	if err != nil {
		return nil, fmt.Errorf("Withdraw: %w", err)
	}

	var resp struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("Withdraw unmarshal: %w", err)
	}
	return &exchange.WithdrawResult{
		TxID:   resp.ID,
		Status: "submitted",
	}, nil
}

// getSpotBalance queries the spot account balance for a specific asset.
func (b *Adapter) getSpotAvailable(asset string) float64 {
	bal, err := b.GetSpotBalance()
	if err != nil {
		return 0
	}
	return bal.Available
}

// WithdrawFeeInclusive returns true because Binance Withdraw amount includes fee (recipient gets amount - fee).
func (b *Adapter) WithdrawFeeInclusive() bool { return true }

// GetWithdrawFee queries the Binance API for the withdrawal fee of a coin on a given chain.
func (b *Adapter) GetWithdrawFee(coin, chain string) (fee float64, minWithdraw float64, err error) {
	spotClient := b.client.WithBaseURL("https://api.binance.com")
	data, apiErr := spotClient.Get("/sapi/v1/capital/config/getall", nil)
	if apiErr != nil {
		return 0, 0, fmt.Errorf("GetWithdrawFee: %w", apiErr)
	}

	network := mapChainToBinanceNetwork(chain)

	var coins []struct {
		Coin        string `json:"coin"`
		NetworkList []struct {
			Network     string `json:"network"`
			WithdrawFee string `json:"withdrawFee"`
			WithdrawMin string `json:"withdrawMin"`
		} `json:"networkList"`
	}
	if err := json.Unmarshal(data, &coins); err != nil {
		return 0, 0, fmt.Errorf("GetWithdrawFee unmarshal: %w", err)
	}

	for _, c := range coins {
		if !strings.EqualFold(c.Coin, coin) {
			continue
		}
		for _, n := range c.NetworkList {
			if strings.EqualFold(n.Network, network) {
				parsedFee, err := strconv.ParseFloat(n.WithdrawFee, 64)
				if err != nil {
					return 0, 0, fmt.Errorf("GetWithdrawFee parse fee: %w", err)
				}
				minWd, _ := strconv.ParseFloat(n.WithdrawMin, 64)
				return parsedFee, minWd, nil
			}
		}
		return 0, 0, fmt.Errorf("GetWithdrawFee: network %s not found for %s", network, coin)
	}
	return 0, 0, fmt.Errorf("GetWithdrawFee: coin %s not found", coin)
}

func mapChainToBinanceNetwork(chain string) string {
	switch chain {
	case "BEP20":
		return "BSC"
	case "APT":
		return "APT"
	default:
		return chain
	}
}

func mapChainToNetwork(chain string) string {
	switch chain {
	case "BEP20":
		return "BSC"
	case "APT":
		return "APT"
	default:
		return chain
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func mapSide(s exchange.Side) string {
	switch s {
	case exchange.SideBuy:
		return "BUY"
	case exchange.SideSell:
		return "SELL"
	default:
		return strings.ToUpper(string(s))
	}
}

func mapOrderType(t string) string {
	switch strings.ToLower(t) {
	case "limit":
		return "LIMIT"
	case "market":
		return "MARKET"
	default:
		return strings.ToUpper(t)
	}
}

func mapTimeInForce(f string) string {
	switch strings.ToLower(f) {
	case "gtc", "":
		return "GTC"
	case "ioc":
		return "IOC"
	case "fok":
		return "FOK"
	case "post_only", "gtx":
		return "GTX"
	default:
		return strings.ToUpper(f)
	}
}

func countDecimals(s string) int {
	idx := strings.IndexByte(s, '.')
	if idx < 0 {
		return 0
	}
	d := strings.TrimRight(s[idx+1:], "0")
	return len(d)
}

func isAPIError(err error, code int) bool {
	if apiErr, ok := err.(*APIError); ok {
		return apiErr.Code == code
	}
	return false
}

// GetUserTrades returns filled trades for a symbol since startTime.
// Binance endpoint: GET /fapi/v1/userTrades
func (b *Adapter) GetUserTrades(symbol string, startTime time.Time, limit int) ([]exchange.Trade, error) {
	if limit <= 0 {
		limit = 100
	}
	params := map[string]string{
		"symbol":    symbol,
		"startTime": strconv.FormatInt(startTime.UnixMilli(), 10),
		"limit":     strconv.Itoa(limit),
	}
	body, err := b.client.Get("/fapi/v1/userTrades", params)
	if err != nil {
		return nil, fmt.Errorf("GetUserTrades: %w", err)
	}

	var resp []struct {
		ID              int64  `json:"id"`
		OrderID         int64  `json:"orderId"`
		Symbol          string `json:"symbol"`
		Side            string `json:"side"` // BUY or SELL
		Price           string `json:"price"`
		Qty             string `json:"qty"`
		Commission      string `json:"commission"`
		CommissionAsset string `json:"commissionAsset"`
		Time            int64  `json:"time"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("GetUserTrades unmarshal: %w", err)
	}

	trades := make([]exchange.Trade, 0, len(resp))
	for _, t := range resp {
		price, _ := strconv.ParseFloat(t.Price, 64)
		qty, _ := strconv.ParseFloat(t.Qty, 64)
		fee, _ := strconv.ParseFloat(t.Commission, 64)
		if fee < 0 {
			fee = -fee
		}
		trades = append(trades, exchange.Trade{
			TradeID:  strconv.FormatInt(t.ID, 10),
			OrderID:  strconv.FormatInt(t.OrderID, 10),
			Symbol:   t.Symbol,
			Side:     strings.ToLower(t.Side),
			Price:    price,
			Quantity: qty,
			Fee:      fee,
			FeeCoin:  t.CommissionAsset,
			Time:     time.UnixMilli(t.Time),
		})
	}
	return trades, nil
}

// GetFundingFees returns funding fee history for a symbol since the given time.
func (b *Adapter) GetFundingFees(symbol string, since time.Time) ([]exchange.FundingPayment, error) {
	params := map[string]string{
		"symbol":     symbol,
		"incomeType": "FUNDING_FEE",
		"startTime":  strconv.FormatInt(since.UnixMilli(), 10),
		"limit":      "1000",
	}
	body, err := b.client.Get("/fapi/v1/income", params)
	if err != nil {
		return nil, fmt.Errorf("GetFundingFees: %w", err)
	}

	var resp []struct {
		Income string `json:"income"`
		Time   int64  `json:"time"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("GetFundingFees unmarshal: %w", err)
	}

	out := make([]exchange.FundingPayment, 0, len(resp))
	for _, r := range resp {
		amt, _ := strconv.ParseFloat(r.Income, 64)
		out = append(out, exchange.FundingPayment{
			Amount: amt,
			Time:   time.UnixMilli(r.Time),
		})
	}
	return out, nil
}

// GetClosePnL returns position-level PnL by aggregating income records.
// Binance has no single position-close endpoint, so we sum REALIZED_PNL,
// COMMISSION, and FUNDING_FEE income records for the symbol.
func (b *Adapter) GetClosePnL(symbol string, since time.Time) ([]exchange.ClosePnL, error) {
	var pricePnL, fees, funding float64

	// Query each income type separately for reliability.
	for _, incomeType := range []string{"REALIZED_PNL", "COMMISSION", "FUNDING_FEE"} {
		params := map[string]string{
			"symbol":     symbol,
			"incomeType": incomeType,
			"startTime":  strconv.FormatInt(since.UnixMilli(), 10),
			"limit":      "1000",
		}
		body, err := b.client.Get("/fapi/v1/income", params)
		if err != nil {
			return nil, fmt.Errorf("GetClosePnL income(%s): %w", incomeType, err)
		}

		var records []struct {
			Income string `json:"income"`
		}
		if err := json.Unmarshal(body, &records); err != nil {
			return nil, fmt.Errorf("GetClosePnL unmarshal(%s): %w", incomeType, err)
		}

		for _, r := range records {
			amt, _ := strconv.ParseFloat(r.Income, 64)
			switch incomeType {
			case "REALIZED_PNL":
				pricePnL += amt
			case "COMMISSION":
				fees += amt
			case "FUNDING_FEE":
				funding += amt
			}
		}
	}

	// Return a single aggregated record (no side info available from income API).
	return []exchange.ClosePnL{{
		PricePnL:  pricePnL,
		Fees:      fees,
		Funding:   funding,
		NetPnL:    pricePnL + fees + funding,
		Side:      "", // Binance income API doesn't provide position side
		CloseTime: time.Now().UTC(),
	}}, nil
}

// PlaceStopLoss places a STOP_MARKET algo order on Binance futures.
// Since 2025-12-09, conditional orders must use POST /fapi/v1/algoOrder.
func (b *Adapter) PlaceStopLoss(params exchange.StopLossParams) (string, error) {
	p := map[string]string{
		"algoType":      "CONDITIONAL",
		"symbol":        params.Symbol,
		"side":          mapSide(params.Side),
		"type":          "STOP_MARKET",
		"triggerPrice":  params.TriggerPrice,
		"closePosition": "true",
	}

	body, err := b.client.Post("/fapi/v1/algoOrder", p)
	if err != nil {
		return "", fmt.Errorf("PlaceStopLoss: %w", err)
	}

	var resp struct {
		AlgoID int64 `json:"algoId"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("PlaceStopLoss unmarshal: %w", err)
	}
	return strconv.FormatInt(resp.AlgoID, 10), nil
}

// PlaceTakeProfit places a take-profit market order on Binance futures using the algo order API.
func (b *Adapter) PlaceTakeProfit(params exchange.TakeProfitParams) (string, error) {
	p := map[string]string{
		"algoType":      "CONDITIONAL",
		"symbol":        params.Symbol,
		"side":          mapSide(params.Side),
		"type":          "TAKE_PROFIT_MARKET",
		"triggerPrice":  params.TriggerPrice,
		"quantity":      params.Size,
		"closePosition": "false",
	}

	body, err := b.client.Post("/fapi/v1/algoOrder", p)
	if err != nil {
		return "", fmt.Errorf("PlaceTakeProfit: %w", err)
	}

	var resp struct {
		AlgoID int64 `json:"algoId"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("PlaceTakeProfit unmarshal: %w", err)
	}
	return strconv.FormatInt(resp.AlgoID, 10), nil
}

// CancelTakeProfit cancels an algo take-profit order on Binance futures.
func (b *Adapter) CancelTakeProfit(symbol, orderID string) error {
	return b.CancelStopLoss(symbol, orderID)
}

// CancelStopLoss cancels an algo stop-loss order on Binance futures.
func (b *Adapter) CancelStopLoss(symbol, orderID string) error {
	params := map[string]string{
		"algoId": orderID,
	}
	_, err := b.client.Delete("/fapi/v1/algoOrder", params)
	if err != nil {
		if isAPIError(err, -2011) {
			return nil
		}
		return fmt.Errorf("CancelStopLoss: %w", err)
	}
	return nil
}

// EnsureOneWayMode sets the account to one-way position mode (not hedge).
func (b *Adapter) EnsureOneWayMode() error {
	params := map[string]string{
		"dualSidePosition": "false",
	}
	_, err := b.client.Post("/fapi/v1/positionSide/dual", params)
	if err != nil {
		// "No need to change position side" = already one-way
		errMsg := err.Error()
		if strings.Contains(errMsg, "No need") || strings.Contains(errMsg, "-4059") || strings.Contains(errMsg, "-4067") {
			return nil
		}
		return fmt.Errorf("EnsureOneWayMode: %w", err)
	}
	return nil
}

// CancelAllOrders cancels all open orders (regular + conditional/algo) for a symbol.
func (b *Adapter) CancelAllOrders(symbol string) error {
	b.client.Delete("/fapi/v1/allOpenOrders", map[string]string{"symbol": symbol})
	b.client.Delete("/fapi/v1/algoOpenOrders", map[string]string{"symbol": symbol})
	return nil
}

// Close terminates all WebSocket connections for graceful shutdown.
func (b *Adapter) Close() {
	b.priceMu.Lock()
	if b.priceConn != nil {
		b.priceConn.Close()
		b.priceConn = nil
	}
	if b.privConn != nil {
		b.privConn.Close()
		b.privConn = nil
	}
	b.priceMu.Unlock()
}

// GetTradingFee returns the authenticated user's maker/taker fee rates for USDT-M futures.
func (b *Adapter) GetTradingFee() (*exchange.TradingFee, error) {
	params := map[string]string{"symbol": "BTCUSDT"}
	body, err := b.client.Get("/fapi/v1/commissionRate", params)
	if err != nil {
		return nil, fmt.Errorf("binance GetTradingFee: %w", err)
	}

	var resp struct {
		MakerCommissionRate string `json:"makerCommissionRate"`
		TakerCommissionRate string `json:"takerCommissionRate"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("binance GetTradingFee unmarshal: %w", err)
	}

	maker, err := strconv.ParseFloat(resp.MakerCommissionRate, 64)
	if err != nil {
		return nil, fmt.Errorf("binance GetTradingFee parse maker: %w", err)
	}
	taker, err := strconv.ParseFloat(resp.TakerCommissionRate, 64)
	if err != nil {
		return nil, fmt.Errorf("binance GetTradingFee parse taker: %w", err)
	}

	return &exchange.TradingFee{
		MakerRate: maker,
		TakerRate: taker,
	}, nil
}
