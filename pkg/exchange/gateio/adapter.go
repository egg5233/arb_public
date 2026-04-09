package gateio

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
)

var _ exchange.TradingFeeProvider = (*Adapter)(nil)

// Adapter implements the exchange.Exchange interface for Gate.io USDT-M Futures.
type Adapter struct {
	client    *Client
	apiKey    string
	secretKey string

	// Contract multipliers: internalSymbol -> quanto_multiplier (contracts → base asset)
	contractMult map[string]float64

	// Price stream
	priceStore sync.Map // symbol -> exchange.BBO
	priceMu    sync.Mutex
	priceSyms  map[string]bool
	priceWS    *PriceWS

	// Depth stream (top-5 orderbook via WS)
	depthStore sync.Map // Gate.io symbol -> *exchange.Orderbook

	// Private stream
	privateWS            *PrivateWS
	orderStore           sync.Map // orderID (string) -> exchange.OrderUpdate
	orderCallback        func(exchange.OrderUpdate)
	wsMetricsCallback    exchange.WSMetricsCallback
	orderMetricsCallback exchange.OrderMetricsCallback

	// Unified account detection
	isUnified bool
}

func (a *Adapter) SetMetricsCallback(fn exchange.MetricsCallback) {
	if a.client != nil {
		a.client.SetMetricsCallback(fn)
	}
}

func (a *Adapter) SetWSMetricsCallback(fn exchange.WSMetricsCallback) {
	a.wsMetricsCallback = fn
	if a.priceWS != nil {
		a.priceWS.SetMetricsCallback(fn)
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
	r := exchange.PermissionResult{
		Read: exchange.PermUnknown, FuturesTrade: exchange.PermUnknown,
		Withdraw: exchange.PermUnknown, Transfer: exchange.PermUnknown,
		Method: "unsupported",
	}
	// Basic auth check: if we can read the account, keys are valid.
	if _, err := a.client.Get("/futures/usdt/accounts", nil); err != nil {
		r.Error = "auth failed: " + err.Error()
	} else {
		r.Read = exchange.PermGranted
	}
	return r
}

// NewAdapter creates a Gate.io Adapter from ExchangeConfig.
func NewAdapter(cfg exchange.ExchangeConfig) *Adapter {
	return &Adapter{
		client:    NewClient(cfg.ApiKey, cfg.SecretKey),
		apiKey:    cfg.ApiKey,
		secretKey: cfg.SecretKey,
		priceSyms: make(map[string]bool),
	}
}

func (a *Adapter) Name() string { return "gateio" }

// IsUnified returns whether this account is in unified mode.
func (a *Adapter) IsUnified() bool { return a.isUnified }

// DetectUnifiedMode checks Gate.io account mode and sets isUnified flag.
// Falls back to classic mode if the endpoint fails (e.g. no unified permission).
func (a *Adapter) DetectUnifiedMode() {
	data, err := a.client.Get("/unified/unified_mode", nil)
	if err != nil {
		// Endpoint failed — likely no unified permission, use classic mode.
		return
	}
	var resp struct {
		Mode string `json:"mode"`
	}
	if json.Unmarshal(data, &resp) == nil && resp.Mode != "" && resp.Mode != "classic" {
		a.isUnified = true
	}
}

// ---------------------------------------------------------------------------
// Symbol mapping: internal "BTCUSDT" -> Gate.io "BTC_USDT"
// ---------------------------------------------------------------------------

// toGateSymbol converts internal symbol format "BTCUSDT" to Gate.io "BTC_USDT".
func toGateSymbol(symbol string) string {
	// If already contains underscore, return as-is
	if strings.Contains(symbol, "_") {
		return symbol
	}
	// Strip USDT suffix and insert underscore
	if strings.HasSuffix(symbol, "USDT") {
		base := symbol[:len(symbol)-4]
		return base + "_USDT"
	}
	return symbol
}

// fromGateSymbol converts Gate.io "BTC_USDT" back to internal "BTCUSDT".
func fromGateSymbol(symbol string) string {
	return strings.ReplaceAll(symbol, "_", "")
}

// ---------------------------------------------------------------------------
// Orders
// ---------------------------------------------------------------------------

func (a *Adapter) PlaceOrder(req exchange.PlaceOrderParams) (string, error) {
	log.Printf("[gateio] PlaceOrder: symbol=%s side=%s type=%s size=%s price=%s force=%s reduceOnly=%v",
		req.Symbol, req.Side, req.OrderType, req.Size, req.Price, req.Force, req.ReduceOnly)
	contract := toGateSymbol(req.Symbol)

	// Gate.io uses signed integer size in contracts.
	// The engine sizes in base asset units, so convert to contracts
	// by dividing by the quanto_multiplier.
	sizeF, err := strconv.ParseFloat(req.Size, 64)
	if err != nil {
		return "", fmt.Errorf("PlaceOrder: invalid size %q: %w", req.Size, err)
	}
	if a.contractMult != nil {
		if mult, ok := a.contractMult[req.Symbol]; ok && mult > 0 {
			sizeF = sizeF / mult
		}
	}
	size := int64(math.Round(sizeF))
	if size == 0 {
		size = 1
	}
	if req.Side == exchange.SideSell {
		size = -int64(math.Abs(float64(size)))
	} else {
		size = int64(math.Abs(float64(size)))
	}

	tif := "gtc"
	if req.Force != "" {
		tif = strings.ToLower(req.Force)
	}

	// GateIO market orders require price "0" with tif "ioc".
	price := req.Price
	if price == "" {
		price = "0"
	}

	orderReq := map[string]interface{}{
		"contract": contract,
		"size":     size,
		"price":    price,
		"tif":      tif,
	}
	if req.ReduceOnly {
		orderReq["reduce_only"] = true
	}
	if req.ClientOid != "" {
		orderReq["text"] = "t-" + req.ClientOid
	}

	bodyBytes, err := json.Marshal(orderReq)
	if err != nil {
		return "", fmt.Errorf("PlaceOrder: marshal: %w", err)
	}

	data, err := a.client.Post("/futures/usdt/orders", string(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("PlaceOrder: %w", err)
	}

	var resp struct {
		ID   int64  `json:"id"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", fmt.Errorf("PlaceOrder unmarshal: %w (body: %s)", err, string(data))
	}
	orderID := strconv.FormatInt(resp.ID, 10)
	if a.orderMetricsCallback != nil {
		a.orderMetricsCallback(exchange.OrderMetricEvent{
			Type:      exchange.OrderMetricPlaced,
			OrderID:   orderID,
			Timestamp: time.Now(),
		})
	}
	return orderID, nil
}

func (a *Adapter) CancelOrder(symbol, orderID string) error {
	path := "/futures/usdt/orders/" + orderID
	_, err := a.client.Delete(path, nil)
	if err != nil {
		// Ignore "ORDER_NOT_FOUND" -- already cancelled or filled
		if apiErr, ok := err.(*APIError); ok && apiErr.Label == "ORDER_NOT_FOUND" {
			return nil
		}
		return fmt.Errorf("CancelOrder: %w", err)
	}
	return nil
}

func (a *Adapter) GetPendingOrders(symbol string) ([]exchange.Order, error) {
	contract := toGateSymbol(symbol)
	params := map[string]string{
		"contract": contract,
		"status":   "open",
	}
	data, err := a.client.Get("/futures/usdt/orders", params)
	if err != nil {
		return nil, fmt.Errorf("GetPendingOrders: %w", err)
	}

	var raw []struct {
		ID       int64  `json:"id"`
		Text     string `json:"text"`
		Contract string `json:"contract"`
		Size     int64  `json:"size"`
		Price    string `json:"price"`
		Status   string `json:"status"`
		Tif      string `json:"tif"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("GetPendingOrders unmarshal: %w", err)
	}

	out := make([]exchange.Order, 0, len(raw))
	for _, o := range raw {
		side := "buy"
		size := o.Size
		if size < 0 {
			side = "sell"
			size = -size
		}
		clientOid := ""
		if strings.HasPrefix(o.Text, "t-") {
			clientOid = o.Text[2:]
		}
		out = append(out, exchange.Order{
			OrderID:   strconv.FormatInt(o.ID, 10),
			ClientOid: clientOid,
			Symbol:    fromGateSymbol(o.Contract),
			Side:      side,
			OrderType: "limit",
			Price:     o.Price,
			Size:      strconv.FormatInt(size, 10),
			Status:    o.Status,
		})
	}
	return out, nil
}

func (a *Adapter) GetOrderFilledQty(orderID, symbol string) (float64, error) {
	path := "/futures/usdt/orders/" + orderID
	data, err := a.client.Get(path, nil)
	if err != nil {
		return 0, fmt.Errorf("GetOrderFilledQty: %w", err)
	}

	var resp struct {
		Size      int64  `json:"size"`
		Left      int64  `json:"left"`
		FillPrice string `json:"fill_price"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return 0, fmt.Errorf("GetOrderFilledQty unmarshal: %w", err)
	}

	// Filled = total size - remaining (left). Both use absolute values.
	totalAbs := int64(math.Abs(float64(resp.Size)))
	leftAbs := int64(math.Abs(float64(resp.Left)))
	filled := float64(totalAbs - leftAbs)

	// Convert contract count to base asset units using quanto multiplier.
	if a.contractMult != nil {
		if mult, ok := a.contractMult[symbol]; ok && mult > 0 {
			filled *= mult
		}
	}
	if filled > 0 && a.orderMetricsCallback != nil {
		a.orderMetricsCallback(exchange.OrderMetricEvent{
			Type:      exchange.OrderMetricFilled,
			OrderID:   orderID,
			FilledQty: filled,
			Timestamp: time.Now(),
		})
	}
	return filled, nil
}

// ---------------------------------------------------------------------------
// Positions
// ---------------------------------------------------------------------------

func (a *Adapter) GetPosition(symbol string) ([]exchange.Position, error) {
	contract := toGateSymbol(symbol)
	path := "/futures/usdt/positions/" + contract
	data, err := a.client.Get(path, nil)
	if err != nil {
		return nil, fmt.Errorf("GetPosition: %w", err)
	}

	var raw struct {
		Contract           string      `json:"contract"`
		Size               int64       `json:"size"`
		EntryPrice         string      `json:"entry_price"`
		UnrealisedPnl      string      `json:"unrealised_pnl"`
		Leverage           string      `json:"leverage"`
		CrossLeverageLimit json.Number `json:"cross_leverage_limit"`
		Mode               string      `json:"mode"`
		LiqPrice           string      `json:"liq_price"`
		MarkPrice          string      `json:"mark_price"`
		PnlFund            string      `json:"pnl_fund"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("GetPosition unmarshal: %w", err)
	}

	if raw.Size == 0 {
		return []exchange.Position{}, nil
	}

	holdSide := "long"
	absSize := raw.Size
	if raw.Size < 0 {
		holdSide = "short"
		absSize = -raw.Size
	}

	// leverage="0" means cross mode; leverage > 0 means isolated mode
	lev, _ := strconv.ParseFloat(raw.Leverage, 64)
	marginMode := "crossed"
	if lev > 0 {
		marginMode = "isolated"
	}

	// Convert contract count to base asset units using quanto multiplier,
	// so position sizes are consistent with other exchanges and PlaceOrder.
	internalSym := fromGateSymbol(raw.Contract)
	sizeBase := float64(absSize)
	if a.contractMult != nil {
		if mult, ok := a.contractMult[internalSym]; ok && mult > 0 {
			sizeBase = float64(absSize) * mult
		}
	}
	sizeStr := strconv.FormatFloat(sizeBase, 'f', -1, 64)

	return []exchange.Position{{
		Symbol:           internalSym,
		HoldSide:         holdSide,
		Total:            sizeStr,
		Available:        sizeStr,
		AverageOpenPrice: raw.EntryPrice,
		UnrealizedPL:     raw.UnrealisedPnl,
		Leverage:         raw.Leverage,
		MarginMode:       marginMode,
		LiquidationPrice: raw.LiqPrice,
		MarkPrice:        raw.MarkPrice,
		FundingFee:       raw.PnlFund,
	}}, nil
}

func (a *Adapter) GetAllPositions() ([]exchange.Position, error) {
	data, err := a.client.Get("/futures/usdt/positions", nil)
	if err != nil {
		return nil, fmt.Errorf("GetAllPositions: %w", err)
	}

	var raw []struct {
		Contract           string      `json:"contract"`
		Size               int64       `json:"size"`
		EntryPrice         string      `json:"entry_price"`
		UnrealisedPnl      string      `json:"unrealised_pnl"`
		Leverage           string      `json:"leverage"`
		CrossLeverageLimit json.Number `json:"cross_leverage_limit"`
		Mode               string      `json:"mode"`
		LiqPrice           string      `json:"liq_price"`
		MarkPrice          string      `json:"mark_price"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("GetAllPositions unmarshal: %w", err)
	}

	out := make([]exchange.Position, 0)
	for _, p := range raw {
		if p.Size == 0 {
			continue
		}

		holdSide := "long"
		absSize := p.Size
		if p.Size < 0 {
			holdSide = "short"
			absSize = -p.Size
		}

		// leverage="0" means cross mode; leverage > 0 means isolated mode
		lev, _ := strconv.ParseFloat(p.Leverage, 64)
		marginMode := "crossed"
		if lev > 0 {
			marginMode = "isolated"
		}

		// Convert contract count to base asset units using quanto multiplier.
		internalSym := fromGateSymbol(p.Contract)
		sizeBase := float64(absSize)
		if a.contractMult != nil {
			if mult, ok := a.contractMult[internalSym]; ok && mult > 0 {
				sizeBase = float64(absSize) * mult
			}
		}
		sizeStr := strconv.FormatFloat(sizeBase, 'f', -1, 64)

		out = append(out, exchange.Position{
			Symbol:           internalSym,
			HoldSide:         holdSide,
			Total:            sizeStr,
			Available:        sizeStr,
			AverageOpenPrice: p.EntryPrice,
			UnrealizedPL:     p.UnrealisedPnl,
			Leverage:         p.Leverage,
			MarginMode:       marginMode,
			LiquidationPrice: p.LiqPrice,
			MarkPrice:        p.MarkPrice,
		})
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Account Config
// ---------------------------------------------------------------------------

func (a *Adapter) SetLeverage(symbol string, leverage string, holdSide string) error {
	contract := toGateSymbol(symbol)
	path := "/futures/usdt/positions/" + contract + "/leverage"
	// Gate.io cross margin: leverage=0 means "cross mode",
	// cross_leverage_limit sets the actual leverage for cross margin.
	// Setting leverage>0 switches to isolated mode — not what we want.
	params := map[string]string{
		"leverage":             "0",
		"cross_leverage_limit": leverage,
	}
	qs := buildQueryString(params)
	_, err := a.client.Post(path+"?"+qs, "")
	if err != nil {
		return fmt.Errorf("SetLeverage: %w", err)
	}
	return nil
}

func (a *Adapter) SetMarginMode(symbol string, mode string) error {
	contract := toGateSymbol(symbol)
	path := "/futures/usdt/positions/" + contract + "/leverage"

	if strings.ToLower(mode) == "isolated" {
		params := map[string]string{
			"leverage":             "10",
			"cross_leverage_limit": "10",
		}
		qs := buildQueryString(params)
		_, err := a.client.Post(path+"?"+qs, "")
		if err != nil {
			return fmt.Errorf("SetMarginMode: %w", err)
		}
		return nil
	}

	// Cross margin: leverage=0 means "cross mode". Don't override
	// cross_leverage_limit here — SetLeverage already sets it correctly.
	params := map[string]string{
		"leverage": "0",
	}
	qs := buildQueryString(params)
	_, err := a.client.Post(path+"?"+qs, "")
	if err != nil {
		return fmt.Errorf("SetMarginMode: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Contract Info
// ---------------------------------------------------------------------------

func (a *Adapter) LoadAllContracts() (map[string]exchange.ContractInfo, error) {
	data, err := a.client.Get("/futures/usdt/contracts", nil)
	if err != nil {
		return nil, fmt.Errorf("LoadAllContracts: %w", err)
	}

	var contracts []struct {
		Name             string      `json:"name"`
		QuantoMultiplier string      `json:"quanto_multiplier"`
		OrderSizeMin     json.Number `json:"order_size_min"`
		OrderSizeMax     json.Number `json:"order_size_max"`
		OrderPriceRound  string      `json:"order_price_round"`
		InTrade          bool        `json:"in_delisting"`
		MaintenanceRate  string      `json:"maintenance_rate"`
	}
	if err := json.Unmarshal(data, &contracts); err != nil {
		return nil, fmt.Errorf("LoadAllContracts unmarshal: %w", err)
	}

	result := make(map[string]exchange.ContractInfo, len(contracts))
	multipliers := make(map[string]float64, len(contracts))
	for _, c := range contracts {
		internalSymbol := fromGateSymbol(c.Name)
		quantoMult, _ := strconv.ParseFloat(c.QuantoMultiplier, 64)
		priceStep, _ := strconv.ParseFloat(c.OrderPriceRound, 64)

		if quantoMult > 0 {
			multipliers[internalSymbol] = quantoMult
		}

		// Step size for Gate.io is 1 contract (integer contracts)
		stepSize := 1.0
		if quantoMult > 0 {
			stepSize = quantoMult
		}

		// Convert min/max from contract counts to base asset units
		sizeMin, _ := c.OrderSizeMin.Int64()
		sizeMax, _ := c.OrderSizeMax.Int64()
		minBase := float64(sizeMin)
		maxBase := float64(sizeMax)
		if quantoMult > 0 {
			minBase *= quantoMult
			maxBase *= quantoMult
		}

		// Compute decimal places from quanto_multiplier so sizes in base-asset
		// units are formatted correctly (e.g., BTC quanto=0.0001 → 4 decimals).
		sizeDecimals := countDecimals(c.QuantoMultiplier)

		// Parse maintenance_rate (already decimal: "0.005" = 0.5%)
		mr, _ := strconv.ParseFloat(c.MaintenanceRate, 64)
		if mr <= 0 || mr >= 1.0 {
			mr = 0 // bounds check: treat invalid as unknown
		}

		ci := exchange.ContractInfo{
			Symbol:          internalSymbol,
			MinSize:         minBase,
			StepSize:        stepSize,
			MaxSize:         maxBase,
			SizeDecimals:    sizeDecimals,
			PriceStep:       priceStep,
			PriceDecimals:   countDecimals(c.OrderPriceRound),
			MaintenanceRate: mr,
		}
		result[internalSymbol] = ci
	}
	a.contractMult = multipliers

	if len(result) == 0 {
		return nil, fmt.Errorf("no contracts loaded from Gate.io")
	}
	return result, nil
}

// ---------------------------------------------------------------------------
// Maintenance Rate
// ---------------------------------------------------------------------------

// GetMaintenanceRate returns the maintenance margin rate for a symbol at a given
// notional size by querying the risk_limit_tiers endpoint. Falls back to the
// cached ContractInfo.MaintenanceRate if the tiered endpoint fails.
func (a *Adapter) GetMaintenanceRate(symbol string, notionalUSDT float64) (float64, error) {
	contract := toGateSymbol(symbol)
	path := "/futures/usdt/risk_limit_tiers?contract=" + contract
	data, err := a.client.Get(path, nil)
	if err != nil {
		return 0, fmt.Errorf("GetMaintenanceRate: %w", err)
	}

	var tiers []struct {
		RiskLimit       json.Number `json:"risk_limit"`
		MaintenanceRate string      `json:"maintenance_rate"`
	}
	if err := json.Unmarshal(data, &tiers); err != nil {
		return 0, fmt.Errorf("GetMaintenanceRate unmarshal: %w", err)
	}

	if len(tiers) == 0 {
		return 0, fmt.Errorf("GetMaintenanceRate: no tiers for %s", symbol)
	}

	// For notional=0, return the first (lowest) tier
	if notionalUSDT <= 0 {
		rate, _ := strconv.ParseFloat(tiers[0].MaintenanceRate, 64)
		if rate <= 0 || rate >= 1.0 {
			return 0, nil
		}
		return rate, nil
	}

	// Match tier where notionalUSDT <= risk_limit
	for _, tier := range tiers {
		riskLimit, err := tier.RiskLimit.Float64()
		if err != nil {
			continue
		}
		if notionalUSDT <= riskLimit {
			rate, _ := strconv.ParseFloat(tier.MaintenanceRate, 64)
			if rate <= 0 || rate >= 1.0 {
				return 0, nil
			}
			return rate, nil
		}
	}

	// If notional exceeds all tiers, return the last (highest) tier
	rate, _ := strconv.ParseFloat(tiers[len(tiers)-1].MaintenanceRate, 64)
	if rate <= 0 || rate >= 1.0 {
		return 0, nil
	}
	return rate, nil
}

// ---------------------------------------------------------------------------
// Funding Rate
// ---------------------------------------------------------------------------

func (a *Adapter) GetFundingRate(symbol string) (*exchange.FundingRate, error) {
	contract := toGateSymbol(symbol)
	path := "/futures/usdt/contracts/" + contract
	data, err := a.client.Get(path, nil)
	if err != nil {
		return nil, fmt.Errorf("GetFundingRate: %w", err)
	}

	var resp struct {
		Name             string  `json:"name"`
		FundingRate      string  `json:"funding_rate"`
		FundingNextApply float64 `json:"funding_next_apply"`
		FundingInterval  int     `json:"funding_interval"`
		FundingRateLimit string  `json:"funding_rate_limit"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("GetFundingRate unmarshal: %w", err)
	}

	rate, _ := strconv.ParseFloat(resp.FundingRate, 64)

	interval := 8 * time.Hour
	if resp.FundingInterval > 0 {
		interval = time.Duration(resp.FundingInterval) * time.Second
	}

	nextFunding := time.Unix(int64(resp.FundingNextApply), 0)

	fr := &exchange.FundingRate{
		Symbol:      fromGateSymbol(resp.Name),
		Rate:        rate,
		Interval:    interval,
		NextFunding: nextFunding,
	}

	if resp.FundingRateLimit != "" {
		if limit, err := strconv.ParseFloat(resp.FundingRateLimit, 64); err == nil {
			fr.MaxRate = &limit
			negLimit := -limit
			fr.MinRate = &negLimit
		}
	}

	return fr, nil
}

func (a *Adapter) GetFundingInterval(symbol string) (time.Duration, error) {
	contract := toGateSymbol(symbol)
	path := "/futures/usdt/contracts/" + contract
	data, err := a.client.Get(path, nil)
	if err != nil {
		return 0, fmt.Errorf("GetFundingInterval: %w", err)
	}

	var resp struct {
		FundingInterval int `json:"funding_interval"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return 0, fmt.Errorf("GetFundingInterval unmarshal: %w", err)
	}

	if resp.FundingInterval > 0 {
		return time.Duration(resp.FundingInterval) * time.Second, nil
	}
	// Default to 8 hours
	return 8 * time.Hour, nil
}

// ---------------------------------------------------------------------------
// Balance
// ---------------------------------------------------------------------------

func (a *Adapter) GetFuturesBalance() (*exchange.Balance, error) {
	if a.isUnified {
		bal, err := a.getUnifiedBalance()
		if err != nil {
			// Fallback to classic if unified endpoint fails (e.g. missing permission).
			return a.getClassicFuturesBalance()
		}
		return bal, nil
	}
	return a.getClassicFuturesBalance()
}

// getUnifiedBalance reads balance from /unified/accounts for unified account mode.
// Margin ratio calculation depends on account mode:
//   - single_currency: use per-currency (USDT) fields: mm / margin_balance
//   - multi_currency/portfolio: use top-level: total_maintenance_margin / unified_account_total_equity
func (a *Adapter) getUnifiedBalance() (*exchange.Balance, error) {
	data, err := a.client.Get("/unified/accounts", nil)
	if err != nil {
		return nil, fmt.Errorf("getUnifiedBalance: %w", err)
	}

	var resp struct {
		Mode                      string `json:"mode"`
		TotalAvailableMargin      string `json:"total_available_margin"`
		UnifiedAccountTotalEquity string `json:"unified_account_total_equity"`
		TotalMaintenanceMargin    string `json:"total_maintenance_margin"`
		Balances                  map[string]struct {
			Available       string `json:"available"`
			Equity          string `json:"equity"`
			Freeze          string `json:"freeze"`
			MM              string `json:"mm"`               // maintenance margin (per-currency)
			IM              string `json:"im"`               // initial margin (per-currency)
			MarginBalance   string `json:"margin_balance"`   // margin balance (per-currency)
			AvailableMargin string `json:"available_margin"` // available margin (per-currency)
		} `json:"balances"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("getUnifiedBalance unmarshal: %w", err)
	}

	equity, _ := strconv.ParseFloat(resp.UnifiedAccountTotalEquity, 64)
	availableMargin, _ := strconv.ParseFloat(resp.TotalAvailableMargin, 64)
	topLevelMM, _ := strconv.ParseFloat(resp.TotalMaintenanceMargin, 64)

	var marginRatio float64

	if resp.Mode == "single_currency" {
		// single_currency: top-level margin fields are 0, use per-currency USDT fields.
		if usdtBal, ok := resp.Balances["USDT"]; ok {
			if equity <= 0 {
				equity, _ = strconv.ParseFloat(usdtBal.Equity, 64)
			}
			if availableMargin <= 0 {
				availableMargin, _ = strconv.ParseFloat(usdtBal.AvailableMargin, 64)
				if availableMargin <= 0 {
					availableMargin, _ = strconv.ParseFloat(usdtBal.Available, 64)
				}
			}
			mm, _ := strconv.ParseFloat(usdtBal.MM, 64)
			mb, _ := strconv.ParseFloat(usdtBal.MarginBalance, 64)
			if mb > 0 && mm > 0 {
				marginRatio = mm / mb // maintenance margin / margin balance
			}
		}
	} else {
		// multi_currency / portfolio: use top-level fields.
		if usdtBal, ok := resp.Balances["USDT"]; ok {
			if equity <= 0 {
				equity, _ = strconv.ParseFloat(usdtBal.Equity, 64)
			}
			if availableMargin <= 0 {
				availableMargin, _ = strconv.ParseFloat(usdtBal.Available, 64)
			}
		}
		if equity > 0 && topLevelMM > 0 {
			marginRatio = topLevelMM / equity
		}
	}

	return &exchange.Balance{
		Total:       equity,
		Available:   availableMargin,
		Frozen:      equity - availableMargin,
		Currency:    "USDT",
		MarginRatio: marginRatio,
	}, nil
}

// getClassicFuturesBalance reads balance from /futures/usdt/accounts for classic mode.
func (a *Adapter) getClassicFuturesBalance() (*exchange.Balance, error) {
	data, err := a.client.Get("/futures/usdt/accounts", nil)
	if err != nil {
		return nil, fmt.Errorf("GetFuturesBalance: %w", err)
	}

	var resp struct {
		Total     string `json:"total"`
		Available string `json:"available"`
		Currency  string `json:"currency"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("GetFuturesBalance unmarshal: %w", err)
	}

	total, _ := strconv.ParseFloat(resp.Total, 64)
	available, _ := strconv.ParseFloat(resp.Available, 64)

	// Gate.io "total" includes unrealized PnL but can be near-zero (e.g. 4e-9)
	// when no positions exist. Use max(total, available) for a sane equity value.
	if total < available {
		total = available
	}
	// Do NOT fallback available to total — available=0 is a legitimate state
	// meaning all margin is frozen. Overwriting it would mask INSUFFICIENT errors.

	// Gate.io doesn't return margin ratio directly; estimate from available/total
	var marginRatio float64
	if total > 0 && available < total {
		marginRatio = 1.0 - (available / total)
	}

	return &exchange.Balance{
		Total:          total,
		Available:      available,
		Frozen:         total - available,
		Currency:       "USDT",
		MarginRatio:    marginRatio,
		MaxTransferOut: available, // Gate.io: available per docs (per-position available for withdrawal)
	}, nil
}

func (a *Adapter) GetSpotBalance() (*exchange.Balance, error) {
	// Unified account: spot and futures share the same margin pool.
	// GetFuturesBalance() already returns the full available_margin via /unified/accounts.
	// Returning /spot/accounts here would double-count (same money reported twice).
	if a.isUnified {
		return &exchange.Balance{Currency: "USDT"}, nil
	}
	data, err := a.client.Get("/spot/accounts", nil)
	if err != nil {
		return nil, fmt.Errorf("GetSpotBalance: %w", err)
	}

	var accounts []struct {
		Currency  string `json:"currency"`
		Available string `json:"available"`
		Locked    string `json:"locked"`
	}
	if err := json.Unmarshal(data, &accounts); err != nil {
		return nil, fmt.Errorf("GetSpotBalance unmarshal: %w", err)
	}

	for _, acc := range accounts {
		if strings.EqualFold(acc.Currency, "USDT") {
			avail, _ := strconv.ParseFloat(acc.Available, 64)
			locked, _ := strconv.ParseFloat(acc.Locked, 64)
			return &exchange.Balance{
				Total:     avail + locked,
				Available: avail,
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

func (a *Adapter) GetOrderbook(symbol string, depth int) (*exchange.Orderbook, error) {
	if depth <= 0 {
		depth = 20
	}
	contract := toGateSymbol(symbol)
	params := map[string]string{
		"contract": contract,
		"limit":    strconv.Itoa(depth),
	}
	data, err := a.client.Get("/futures/usdt/order_book", params)
	if err != nil {
		return nil, fmt.Errorf("GetOrderbook: %w", err)
	}

	var resp struct {
		Asks []struct {
			P string `json:"p"`
			S int64  `json:"s"`
		} `json:"asks"`
		Bids []struct {
			P string `json:"p"`
			S int64  `json:"s"`
		} `json:"bids"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("GetOrderbook unmarshal: %w", err)
	}

	// Gate.io orderbook sizes are in contracts. Convert to base asset units
	// using quanto_multiplier so slippage estimation uses correct units.
	mult := 1.0
	if a.contractMult != nil {
		if m, ok := a.contractMult[symbol]; ok && m > 0 {
			mult = m
		}
	}

	bids := make([]exchange.PriceLevel, 0, len(resp.Bids))
	for _, b := range resp.Bids {
		price, _ := strconv.ParseFloat(b.P, 64)
		bids = append(bids, exchange.PriceLevel{
			Price:    price,
			Quantity: math.Abs(float64(b.S)) * mult,
		})
	}

	asks := make([]exchange.PriceLevel, 0, len(resp.Asks))
	for _, ak := range resp.Asks {
		price, _ := strconv.ParseFloat(ak.P, 64)
		asks = append(asks, exchange.PriceLevel{
			Price:    price,
			Quantity: math.Abs(float64(ak.S)) * mult,
		})
	}

	return &exchange.Orderbook{
		Symbol: symbol,
		Bids:   bids,
		Asks:   asks,
		Time:   time.Now(),
	}, nil
}

// ---------------------------------------------------------------------------
// Internal Transfer / Withdraw
// ---------------------------------------------------------------------------

// TransferToSpot is a no-op for Gate.io — withdrawals come from spot by default.
func (a *Adapter) TransferToSpot(coin string, amount string) error { return nil }

// TransferToFutures moves funds from spot to futures account.
// No-op for unified accounts (all funds are shared).
func (a *Adapter) TransferToFutures(coin string, amount string) error {
	if a.isUnified {
		return nil
	}
	body := map[string]string{
		"currency": coin,
		"from":     "spot",
		"to":       "futures",
		"amount":   amount,
		"settle":   "usdt",
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("TransferToFutures marshal: %w", err)
	}
	_, err = a.client.Post("/wallet/transfers", string(bodyBytes))
	if err != nil {
		return fmt.Errorf("TransferToFutures: %w", err)
	}
	return nil
}

func (a *Adapter) Withdraw(params exchange.WithdrawParams) (*exchange.WithdrawResult, error) {
	chain := mapChainToGate(params.Chain)
	body := map[string]string{
		"currency": params.Coin,
		"address":  params.Address,
		"amount":   params.Amount,
		"chain":    chain,
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("Withdraw marshal: %w", err)
	}

	data, err := a.client.Post("/withdrawals", string(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("Withdraw: %w", err)
	}

	var resp struct {
		ID     string `json:"id"`
		TxID   string `json:"txid"`
		Amount string `json:"amount"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("Withdraw unmarshal: %w", err)
	}
	return &exchange.WithdrawResult{
		TxID:   resp.ID,
		Status: "submitted",
	}, nil
}

// WithdrawFeeInclusive returns true because Gate.io Withdraw amount includes fee (recipient gets amount - fee).
func (a *Adapter) WithdrawFeeInclusive() bool { return true }

// GetWithdrawFee queries the Gate.io API for the withdrawal fee of a coin on a given chain.
func (a *Adapter) GetWithdrawFee(coin, chain string) (fee float64, minWithdraw float64, err error) {
	network := mapChainToGateNetwork(chain)
	params := map[string]string{
		"currency": coin,
	}
	data, apiErr := a.client.Get("/wallet/withdraw_status", params)
	if apiErr != nil {
		return 0, 0, fmt.Errorf("gateio GetWithdrawFee: %w", apiErr)
	}

	var resp []struct {
		Currency            string            `json:"currency"`
		WithdrawFixOnChains map[string]string `json:"withdraw_fix_on_chains"`
		WithdrawPercent     string            `json:"withdraw_percent"`
		WithdrawAmountMini  string            `json:"withdraw_amount_mini"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return 0, 0, fmt.Errorf("gateio GetWithdrawFee unmarshal: %w", err)
	}

	if len(resp) == 0 {
		return 0, 0, fmt.Errorf("gateio GetWithdrawFee: coin %s not found", coin)
	}

	item := resp[0]

	// Check for percentage-based fee
	if item.WithdrawPercent != "" && item.WithdrawPercent != "0" {
		pct, err := strconv.ParseFloat(item.WithdrawPercent, 64)
		if err == nil && pct > 0 {
			return 0, 0, fmt.Errorf("gateio GetWithdrawFee: percentage-based fee not supported (coin=%s, pct=%s)", coin, item.WithdrawPercent)
		}
	}

	feeStr, ok := item.WithdrawFixOnChains[network]
	if !ok {
		return 0, 0, fmt.Errorf("gateio GetWithdrawFee: chain %s not found for %s", network, coin)
	}
	parsedFee, err := strconv.ParseFloat(feeStr, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("gateio GetWithdrawFee parse fee: %w", err)
	}

	var minWd float64
	if item.WithdrawAmountMini == "" {
		// withdraw_amount_mini may be absent from Gate.io API — return minWd=0 (no minimum)
	} else {
		minWd, _ = strconv.ParseFloat(item.WithdrawAmountMini, 64)
	}
	return parsedFee, minWd, nil
}

func mapChainToGateNetwork(chain string) string {
	switch chain {
	case "BEP20":
		return "BSC"
	case "APT":
		return "APT"
	default:
		return chain
	}
}

func mapChainToGate(chain string) string {
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
// WebSocket: Prices
// ---------------------------------------------------------------------------

func (a *Adapter) StartPriceStream(symbols []string) {
	a.priceMu.Lock()
	defer a.priceMu.Unlock()

	gateSymbols := make([]string, 0, len(symbols))
	for _, s := range symbols {
		gs := toGateSymbol(s)
		gateSymbols = append(gateSymbols, gs)
		a.priceSyms[s] = true
	}

	a.priceWS = NewPriceWS(&a.priceStore, &a.depthStore, a.getContractMult)
	a.priceWS.SetMetricsCallback(a.wsMetricsCallback)
	go a.priceWS.Connect(gateSymbols)
}

func (a *Adapter) SubscribeSymbol(symbol string) bool {
	a.priceMu.Lock()
	defer a.priceMu.Unlock()

	if a.priceSyms[symbol] {
		return false // already subscribed
	}
	a.priceSyms[symbol] = true

	if a.priceWS != nil {
		return a.priceWS.Subscribe(toGateSymbol(symbol))
	}
	return false
}

func (a *Adapter) GetBBO(symbol string) (exchange.BBO, bool) {
	val, ok := a.priceStore.Load(toGateSymbol(symbol))
	if !ok {
		return exchange.BBO{}, false
	}
	return val.(exchange.BBO), true
}

func (a *Adapter) GetPriceStore() *sync.Map {
	return &a.priceStore
}

// ---------------------------------------------------------------------------
// WebSocket: Depth (top-5 orderbook)
// ---------------------------------------------------------------------------

// getContractMult returns the quanto multiplier for a Gate.io format symbol.
func (a *Adapter) getContractMult(gateSymbol string) float64 {
	internalSym := fromGateSymbol(gateSymbol)
	if a.contractMult != nil {
		if m, ok := a.contractMult[internalSym]; ok {
			return m
		}
	}
	return 1.0
}

func (a *Adapter) SubscribeDepth(symbol string) bool {
	if a.priceWS == nil {
		return false
	}
	return a.priceWS.SubscribeDepth(toGateSymbol(symbol))
}

func (a *Adapter) UnsubscribeDepth(symbol string) bool {
	if a.priceWS == nil {
		return false
	}
	return a.priceWS.UnsubscribeDepth(toGateSymbol(symbol))
}

func (a *Adapter) GetDepth(symbol string) (*exchange.Orderbook, bool) {
	val, ok := a.depthStore.Load(toGateSymbol(symbol))
	if !ok {
		return nil, false
	}
	return val.(*exchange.Orderbook), true
}

// ---------------------------------------------------------------------------
// WebSocket: Private
// ---------------------------------------------------------------------------

func (a *Adapter) StartPrivateStream() {
	a.privateWS = NewPrivateWS(a.apiKey, a.secretKey, &a.orderStore, a.contractMult, &a.orderCallback)
	a.privateWS.SetOrderMetricsCallback(a.orderMetricsCallback)
	go a.privateWS.Connect()
}

func (a *Adapter) GetOrderUpdate(orderID string) (exchange.OrderUpdate, bool) {
	val, ok := a.orderStore.Load(orderID)
	if !ok {
		return exchange.OrderUpdate{}, false
	}
	return val.(exchange.OrderUpdate), true
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func countDecimals(s string) int {
	idx := strings.IndexByte(s, '.')
	if idx < 0 {
		return 0
	}
	d := strings.TrimRight(s[idx+1:], "0")
	return len(d)
}

// GetUserTrades returns filled trades for a symbol since startTime.
// Gate.io endpoint: GET /api/v4/futures/usdt/my_trades
func (a *Adapter) GetUserTrades(symbol string, startTime time.Time, limit int) ([]exchange.Trade, error) {
	if limit <= 0 || limit > 100 {
		limit = 100 // Gate.io max is 100
	}
	contract := toGateSymbol(symbol)
	params := map[string]string{
		"contract": contract,
		"limit":    strconv.Itoa(limit),
	}

	body, err := a.client.Get("/futures/usdt/my_trades", params)
	if err != nil {
		return nil, fmt.Errorf("GetUserTrades: %w", err)
	}

	var resp []struct {
		ID         int64   `json:"id"`
		OrderID    string  `json:"order_id"`
		Contract   string  `json:"contract"`
		Size       int64   `json:"size"` // positive=buy, negative=sell (in contracts)
		Price      string  `json:"price"`
		Fee        string  `json:"fee"`         // Gate.io returns fee as string
		CreateTime float64 `json:"create_time"` // unix seconds with decimals
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("GetUserTrades unmarshal: %w", err)
	}

	// Get quanto multiplier for contract→base conversion.
	multiplier := 1.0
	if v, ok := a.contractMult[symbol]; ok {
		multiplier = v
	}

	trades := make([]exchange.Trade, 0, len(resp))
	for _, t := range resp {
		tradeTime := time.Unix(int64(t.CreateTime), int64((t.CreateTime-float64(int64(t.CreateTime)))*1e9))
		if tradeTime.Before(startTime) {
			continue
		}
		price, _ := strconv.ParseFloat(t.Price, 64)
		qty := float64(t.Size)
		side := "buy"
		if qty < 0 {
			qty = -qty
			side = "sell"
		}
		qty *= multiplier // convert contracts to base asset

		fee, _ := strconv.ParseFloat(t.Fee, 64)
		if fee < 0 {
			fee = -fee
		}

		trades = append(trades, exchange.Trade{
			TradeID:  strconv.FormatInt(t.ID, 10),
			OrderID:  t.OrderID,
			Symbol:   fromGateSymbol(t.Contract),
			Side:     side,
			Price:    price,
			Quantity: qty,
			Fee:      fee,
			FeeCoin:  "USDT",
			Time:     tradeTime,
		})
	}
	return trades, nil
}

// GetFundingFees returns funding fee history for a symbol since the given time.
func (a *Adapter) GetFundingFees(symbol string, since time.Time) ([]exchange.FundingPayment, error) {
	contract := toGateSymbol(symbol)
	params := map[string]string{
		"contract": contract,
		"type":     "fund",
		"from":     strconv.FormatInt(since.Unix(), 10),
		"limit":    "100",
	}
	body, err := a.client.Get("/futures/usdt/account_book", params)
	if err != nil {
		return nil, fmt.Errorf("GetFundingFees: %w", err)
	}

	var resp []struct {
		Change string  `json:"change"`
		Time   float64 `json:"time"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("GetFundingFees unmarshal: %w", err)
	}

	out := make([]exchange.FundingPayment, 0, len(resp))
	for _, r := range resp {
		amt, _ := strconv.ParseFloat(r.Change, 64)
		sec := int64(r.Time)
		nsec := int64((r.Time - float64(sec)) * 1e9)
		out = append(out, exchange.FundingPayment{
			Amount: amt,
			Time:   time.Unix(sec, nsec),
		})
	}
	return out, nil
}

// GetClosePnL returns exchange-reported position-level PnL for recently closed positions.
func (a *Adapter) GetClosePnL(symbol string, since time.Time) ([]exchange.ClosePnL, error) {
	contract := toGateSymbol(symbol)
	params := map[string]string{
		"contract": contract,
		"from":     strconv.FormatInt(since.Unix(), 10),
		"limit":    "20",
	}
	body, err := a.client.Get("/futures/usdt/position_close", params)
	if err != nil {
		return nil, fmt.Errorf("GetClosePnL: %w", err)
	}

	var resp []struct {
		Pnl        string  `json:"pnl"`
		PnlPnl     string  `json:"pnl_pnl"`
		PnlFund    string  `json:"pnl_fund"`
		PnlFee     string  `json:"pnl_fee"`
		Side       string  `json:"side"`
		LongPrice  string  `json:"long_price"`
		ShortPrice string  `json:"short_price"`
		AccumSize  string  `json:"accum_size"`
		Time       float64 `json:"time"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("GetClosePnL unmarshal: %w", err)
	}

	out := make([]exchange.ClosePnL, 0, len(resp))
	for _, r := range resp {
		netPnL, _ := strconv.ParseFloat(r.Pnl, 64)
		pricePnL, _ := strconv.ParseFloat(r.PnlPnl, 64)
		funding, _ := strconv.ParseFloat(r.PnlFund, 64)
		fees, _ := strconv.ParseFloat(r.PnlFee, 64)
		closeSize, _ := strconv.ParseFloat(r.AccumSize, 64)

		var entryPrice, exitPrice float64
		if r.Side == "long" {
			entryPrice, _ = strconv.ParseFloat(r.LongPrice, 64)
			exitPrice, _ = strconv.ParseFloat(r.ShortPrice, 64)
		} else {
			entryPrice, _ = strconv.ParseFloat(r.ShortPrice, 64)
			exitPrice, _ = strconv.ParseFloat(r.LongPrice, 64)
		}

		sec := int64(r.Time)
		out = append(out, exchange.ClosePnL{
			PricePnL:   pricePnL,
			Fees:       fees,
			Funding:    funding,
			NetPnL:     netPnL,
			EntryPrice: entryPrice,
			ExitPrice:  exitPrice,
			CloseSize:  math.Abs(closeSize),
			Side:       r.Side, // already "long" or "short"
			CloseTime:  time.Unix(sec, 0),
		})
	}
	return out, nil
}

// PlaceStopLoss places a price-triggered conditional order on Gate.io futures.
func (a *Adapter) PlaceStopLoss(params exchange.StopLossParams) (string, error) {
	contract := toGateSymbol(params.Symbol)

	// Convert base-unit size to contracts.
	sizeF, err := strconv.ParseFloat(params.Size, 64)
	if err != nil {
		return "", fmt.Errorf("PlaceStopLoss: invalid size %q: %w", params.Size, err)
	}
	if a.contractMult != nil {
		if mult, ok := a.contractMult[params.Symbol]; ok && mult > 0 {
			sizeF = sizeF / mult
		}
	}
	size := int64(math.Round(sizeF))
	if size == 0 {
		size = 1
	}
	// Gate.io: sell orders use negative size.
	if params.Side == exchange.SideSell {
		size = -int64(math.Abs(float64(size)))
	} else {
		size = int64(math.Abs(float64(size)))
	}

	// trigger.rule: 1 = price >= trigger (short SL), 2 = price <= trigger (long SL)
	rule := 2
	if params.Side == exchange.SideBuy {
		rule = 1
	}

	orderReq := map[string]interface{}{
		"initial": map[string]interface{}{
			"contract":    contract,
			"size":        size,
			"price":       "0", // market price
			"tif":         "ioc",
			"reduce_only": true,
		},
		"trigger": map[string]interface{}{
			"strategy_type": 0,
			"price_type":    1, // mark price
			"price":         params.TriggerPrice,
			"rule":          rule,
		},
	}

	bodyBytes, err := json.Marshal(orderReq)
	if err != nil {
		return "", fmt.Errorf("PlaceStopLoss: marshal: %w", err)
	}

	data, err := a.client.Post("/futures/usdt/price_orders", string(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("PlaceStopLoss: %w", err)
	}

	var resp struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", fmt.Errorf("PlaceStopLoss unmarshal: %w (body: %s)", err, string(data))
	}
	return strconv.FormatInt(resp.ID, 10), nil
}

// PlaceTakeProfit places a take-profit conditional order on Gate.io futures.
func (a *Adapter) PlaceTakeProfit(params exchange.TakeProfitParams) (string, error) {
	contract := toGateSymbol(params.Symbol)

	// Convert base-unit size to contracts.
	sizeF, err := strconv.ParseFloat(params.Size, 64)
	if err != nil {
		return "", fmt.Errorf("PlaceTakeProfit: invalid size %q: %w", params.Size, err)
	}
	if a.contractMult != nil {
		if mult, ok := a.contractMult[params.Symbol]; ok && mult > 0 {
			sizeF = sizeF / mult
		}
	}
	size := int64(math.Round(sizeF))
	if size == 0 {
		size = 1
	}
	// Gate.io: sell orders use negative size.
	if params.Side == exchange.SideSell {
		size = -int64(math.Abs(float64(size)))
	} else {
		size = int64(math.Abs(float64(size)))
	}

	// trigger.rule for TP is opposite of SL:
	// 1 = price >= trigger (long TP — sell when price rises),
	// 2 = price <= trigger (short TP — buy when price drops).
	rule := 1
	if params.Side == exchange.SideBuy {
		rule = 2
	}

	orderReq := map[string]interface{}{
		"initial": map[string]interface{}{
			"contract":    contract,
			"size":        size,
			"price":       "0", // market price
			"tif":         "ioc",
			"reduce_only": true,
		},
		"trigger": map[string]interface{}{
			"strategy_type": 0,
			"price_type":    1, // mark price
			"price":         params.TriggerPrice,
			"rule":          rule,
		},
	}

	bodyBytes, err := json.Marshal(orderReq)
	if err != nil {
		return "", fmt.Errorf("PlaceTakeProfit: marshal: %w", err)
	}

	data, err := a.client.Post("/futures/usdt/price_orders", string(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("PlaceTakeProfit: %w", err)
	}

	var resp struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", fmt.Errorf("PlaceTakeProfit unmarshal: %w (body: %s)", err, string(data))
	}
	return strconv.FormatInt(resp.ID, 10), nil
}

// CancelTakeProfit cancels a price-triggered take-profit order on Gate.io futures.
func (a *Adapter) CancelTakeProfit(symbol, orderID string) error {
	return a.CancelStopLoss(symbol, orderID)
}

// CancelStopLoss cancels a price-triggered conditional order on Gate.io futures.
func (a *Adapter) CancelStopLoss(symbol, orderID string) error {
	path := "/futures/usdt/price_orders/" + orderID
	_, err := a.client.Delete(path, nil)
	if err != nil {
		if apiErr, ok := err.(*APIError); ok && apiErr.Label == "ORDER_NOT_FOUND" {
			return nil
		}
		return fmt.Errorf("CancelStopLoss: %w", err)
	}
	return nil
}

// CancelAllOrders cancels all open orders (regular + conditional/algo) for a symbol.
func (a *Adapter) CancelAllOrders(symbol string) error {
	gtSym := toGateSymbol(symbol)
	a.client.Delete("/futures/usdt/price_orders", map[string]string{"contract": gtSym})
	a.client.Delete("/futures/usdt/orders", map[string]string{"contract": gtSym})
	return nil
}

// EnsureOneWayMode ensures Gate.io is in single (non-dual) position mode.
// Close terminates all WebSocket connections for graceful shutdown.
func (a *Adapter) Close() {
	if a.priceWS != nil {
		a.priceWS.Close()
	}
	if a.privateWS != nil {
		a.privateWS.Close()
	}
}

func (a *Adapter) EnsureOneWayMode() error {
	// Gate.io annotations validate dual_mode as a JSON body, not a query parameter.
	_, err := a.client.Post("/futures/usdt/dual_mode", `{"dual_mode":false}`)
	if err != nil {
		errMsg := err.Error()
		// Already in single mode, has open positions, or no change needed
		if strings.Contains(errMsg, "INVALID_DUAL_MODE") || strings.Contains(errMsg, "not changed") ||
			strings.Contains(errMsg, "NO_CHANGE") || strings.Contains(errMsg, "POSITION_NOT_CLOSE") || strings.Contains(errMsg, "ORDER_NOT_CLOSE") {
			return nil
		}
		return fmt.Errorf("EnsureOneWayMode: %w", err)
	}
	return nil
}

// GetTradingFee returns the authenticated user's maker/taker fee rates for USDT-M futures.
func (a *Adapter) GetTradingFee() (*exchange.TradingFee, error) {
	data, err := a.client.Get("/wallet/fee", nil)
	if err != nil {
		return nil, fmt.Errorf("gateio GetTradingFee: %w", err)
	}

	var resp struct {
		FuturesTakerFee string `json:"futures_taker_fee"`
		FuturesMakerFee string `json:"futures_maker_fee"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("gateio GetTradingFee unmarshal: %w", err)
	}

	maker, err := strconv.ParseFloat(resp.FuturesMakerFee, 64)
	if err != nil {
		return nil, fmt.Errorf("gateio GetTradingFee parse maker: %w", err)
	}
	taker, err := strconv.ParseFloat(resp.FuturesTakerFee, 64)
	if err != nil {
		return nil, fmt.Errorf("gateio GetTradingFee parse taker: %w", err)
	}

	return &exchange.TradingFee{
		MakerRate: math.Abs(maker),
		TakerRate: taker,
	}, nil
}
