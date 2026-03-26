package okx

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"arb/pkg/exchange"

	"github.com/gorilla/websocket"
)

// getCtVal returns the contract value (ctVal) for a given internal symbol.
// OKX sizes are in contracts; ctVal converts: base_units = contracts * ctVal.
func (a *Adapter) getCtVal(symbol string) float64 {
	instID := toOKXInstID(symbol)
	if v, ok := a.ctValCache.Load(instID); ok {
		return v.(float64)
	}
	return 1 // default: 1 contract = 1 base unit (safe for most USDT pairs)
}

// Adapter implements the exchange.Exchange interface for OKX USDT-margined perpetual swaps.
type Adapter struct {
	client     *Client
	apiKey     string
	secretKey  string
	passphrase string

	// Contract value cache: instId -> ctVal (contract value per contract)
	ctValCache sync.Map // string -> float64

	// Price stream
	priceStore sync.Map // internal symbol -> exchange.BBO
	priceConn  *websocket.Conn
	priceMu    sync.Mutex
	priceSyms  map[string]bool

	// Depth stream (top-5 orderbook via WS)
	depthStore sync.Map        // internal symbol -> *exchange.Orderbook
	depthSyms  map[string]bool // internal symbols with active depth subscriptions

	// Private stream
	privConn   *websocket.Conn
	privMu     sync.Mutex
	orderStore    sync.Map // orderID -> exchange.OrderUpdate
	orderCallback func(exchange.OrderUpdate)
}

func (a *Adapter) SetOrderCallback(fn func(exchange.OrderUpdate)) {
	a.orderCallback = fn
}

func (a *Adapter) CheckPermissions() exchange.PermissionResult {
	r := exchange.PermissionResult{Method: "inferred"}
	checkOKX := func(data []byte, err error) exchange.PermStatus {
		if err != nil {
			s := err.Error()
			if strings.Contains(s, "50111") { return exchange.PermDenied }
			if strings.Contains(s, "403") { return exchange.PermUnknown }
			return exchange.PermUnknown
		}
		return exchange.PermGranted
	}
	// Read
	d, err := a.client.Get("/api/v5/account/balance", nil)
	r.Read = checkOKX(d, err)
	if r.Read == exchange.PermUnknown && err != nil && strings.Contains(err.Error(), "403") {
		r.Error = "IP restricted or access denied"
		r.FuturesTrade = exchange.PermUnknown
		r.Withdraw = exchange.PermUnknown
		r.Transfer = exchange.PermUnknown
		return r
	}
	// Futures Trade
	d, err = a.client.Get("/api/v5/trade/orders-pending", map[string]string{"instType": "SWAP"})
	r.FuturesTrade = checkOKX(d, err)
	// Withdraw
	_, err = a.client.Post("/api/v5/asset/withdrawal", map[string]string{
		"ccy": "USDT", "amt": "0", "dest": "4", "toAddr": "test", "fee": "0", "chain": "USDT-TRC20",
	})
	r.Withdraw = checkOKX(nil, err)
	if err == nil { r.Withdraw = exchange.PermGranted }
	// Transfer
	_, err = a.client.Post("/api/v5/asset/transfer", map[string]string{
		"ccy": "USDT", "amt": "0", "from": "6", "to": "18", "type": "0",
	})
	r.Transfer = checkOKX(nil, err)
	if err == nil { r.Transfer = exchange.PermGranted }
	return r
}

// NewAdapter creates an OKX Adapter from ExchangeConfig.
func NewAdapter(cfg exchange.ExchangeConfig) *Adapter {
	return &Adapter{
		client:     NewClient(cfg.ApiKey, cfg.SecretKey, cfg.Passphrase),
		apiKey:     cfg.ApiKey,
		secretKey:  cfg.SecretKey,
		passphrase: cfg.Passphrase,
		priceSyms:  make(map[string]bool),
		depthSyms:  make(map[string]bool),
	}
}

func (a *Adapter) Name() string { return "okx" }

// ---------------------------------------------------------------------------
// Symbol mapping
// Internal: "BTCUSDT" -> OKX: "BTC-USDT-SWAP"
// ---------------------------------------------------------------------------

// toOKXInstID converts internal symbol format to OKX instrument ID.
// "BTCUSDT" -> "BTC-USDT-SWAP"
func toOKXInstID(symbol string) string {
	s := strings.TrimSuffix(symbol, "USDT")
	return s + "-USDT-SWAP"
}

// fromOKXInstID converts OKX instrument ID to internal symbol format.
// "BTC-USDT-SWAP" -> "BTCUSDT"
func fromOKXInstID(instID string) string {
	// Remove -SWAP suffix, then replace remaining dashes
	s := strings.TrimSuffix(instID, "-SWAP")
	s = strings.Replace(s, "-", "", -1)
	return s
}

// ---------------------------------------------------------------------------
// Orders
// ---------------------------------------------------------------------------

func (a *Adapter) PlaceOrder(req exchange.PlaceOrderParams) (string, error) {
	instID := toOKXInstID(req.Symbol)

	// OKX sz is in contracts. Engine sends base units, so divide by ctVal.
	sz := req.Size
	ctVal := a.getCtVal(req.Symbol)
	if ctVal != 1 {
		sizeF, err := strconv.ParseFloat(req.Size, 64)
		if err == nil {
			contracts := math.Round(sizeF / ctVal)
			sz = strconv.FormatFloat(contracts, 'f', 0, 64)
		}
	}

	body := map[string]interface{}{
		"instId":  instID,
		"tdMode":  "cross",
		"side":    string(req.Side),
		"ordType": mapOKXOrdType(req.OrderType, req.Force),
		"sz":      sz,
	}

	if req.OrderType == "limit" {
		body["px"] = req.Price
	}
	if req.ReduceOnly {
		body["reduceOnly"] = true
	}
	if req.ClientOid != "" {
		body["clOrdId"] = req.ClientOid
	}

	data, err := a.client.Post("/api/v5/trade/order", body)
	if err != nil {
		return "", fmt.Errorf("PlaceOrder: %w", err)
	}

	var resp []struct {
		OrdID   string `json:"ordId"`
		ClOrdID string `json:"clOrdId"`
		SCode   string `json:"sCode"`
		SMsg    string `json:"sMsg"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", fmt.Errorf("PlaceOrder unmarshal: %w", err)
	}
	if len(resp) == 0 {
		return "", fmt.Errorf("PlaceOrder: empty response")
	}
	if resp[0].SCode != "0" {
		return "", fmt.Errorf("PlaceOrder: code=%s msg=%s", resp[0].SCode, resp[0].SMsg)
	}
	return resp[0].OrdID, nil
}

func (a *Adapter) CancelOrder(symbol, orderID string) error {
	instID := toOKXInstID(symbol)
	body := map[string]interface{}{
		"instId": instID,
		"ordId":  orderID,
	}

	_, err := a.client.Post("/api/v5/trade/cancel-order", body)
	if err != nil {
		// Ignore "order does not exist" or already cancelled
		if apiErr, ok := err.(*APIError); ok {
			// 51400: cancellation failed (already filled/cancelled)
			if apiErr.Code == "51400" || apiErr.Code == "51401" {
				return nil
			}
		}
		return fmt.Errorf("CancelOrder: %w", err)
	}
	return nil
}

func (a *Adapter) GetPendingOrders(symbol string) ([]exchange.Order, error) {
	params := map[string]string{
		"instType": "SWAP",
	}
	if symbol != "" {
		params["instId"] = toOKXInstID(symbol)
	}

	data, err := a.client.Get("/api/v5/trade/orders-pending", params)
	if err != nil {
		return nil, fmt.Errorf("GetPendingOrders: %w", err)
	}

	var raw []struct {
		OrdID   string `json:"ordId"`
		ClOrdID string `json:"clOrdId"`
		InstID  string `json:"instId"`
		Side    string `json:"side"`
		OrdType string `json:"ordType"`
		Px      string `json:"px"`
		Sz      string `json:"sz"`
		State   string `json:"state"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("GetPendingOrders unmarshal: %w", err)
	}

	out := make([]exchange.Order, 0, len(raw))
	for _, o := range raw {
		// OKX sz is in contracts; convert to base units using ctVal.
		sym := fromOKXInstID(o.InstID)
		szF, _ := strconv.ParseFloat(o.Sz, 64)
		szF *= a.getCtVal(sym)
		sz := strconv.FormatFloat(szF, 'f', -1, 64)
		out = append(out, exchange.Order{
			OrderID:   o.OrdID,
			ClientOid: o.ClOrdID,
			Symbol:    sym,
			Side:      o.Side,
			OrderType: o.OrdType,
			Price:     o.Px,
			Size:      sz,
			Status:    mapState(o.State),
		})
	}
	return out, nil
}

func (a *Adapter) GetOrderFilledQty(orderID, symbol string) (float64, error) {
	instID := toOKXInstID(symbol)
	params := map[string]string{
		"instId": instID,
		"ordId":  orderID,
	}

	data, err := a.client.Get("/api/v5/trade/order", params)
	if err != nil {
		return 0, fmt.Errorf("GetOrderFilledQty: %w", err)
	}

	var raw []struct {
		FillSz    string `json:"fillSz"`
		AccFillSz string `json:"accFillSz"`
		AvgPx     string `json:"avgPx"`
		State     string `json:"state"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return 0, fmt.Errorf("GetOrderFilledQty unmarshal: %w", err)
	}
	if len(raw) == 0 {
		return 0, nil
	}

	// OKX returns fill size in contracts; convert to base units.
	qty, _ := strconv.ParseFloat(raw[0].AccFillSz, 64)
	qty *= a.getCtVal(symbol)

	// Store REST result in orderStore so confirmFill can read avgPrice.
	avgPx, _ := strconv.ParseFloat(raw[0].AvgPx, 64)
	if qty > 0 && avgPx > 0 {
		status := raw[0].State
		if status == "filled" || status == "canceled" || status == "cancelled" {
			// normalize to engine convention
			if status == "canceled" {
				status = "cancelled"
			}
		} else {
			status = "partially_filled"
		}
		a.orderStore.Store(orderID, exchange.OrderUpdate{
			OrderID:      orderID,
			Status:       status,
			FilledVolume: qty,
			AvgPrice:     avgPx,
		})
	}

	return qty, nil
}

// ---------------------------------------------------------------------------
// Positions
// ---------------------------------------------------------------------------

func (a *Adapter) GetPosition(symbol string) ([]exchange.Position, error) {
	params := map[string]string{
		"instType": "SWAP",
		"instId":   toOKXInstID(symbol),
	}
	return a.fetchPositions(params)
}

func (a *Adapter) GetAllPositions() ([]exchange.Position, error) {
	params := map[string]string{
		"instType": "SWAP",
	}
	positions, err := a.fetchPositions(params)
	if err != nil {
		return nil, err
	}
	// Filter to non-zero positions
	out := make([]exchange.Position, 0)
	for _, p := range positions {
		amt, _ := strconv.ParseFloat(p.Total, 64)
		if amt != 0 {
			out = append(out, p)
		}
	}
	return out, nil
}

func (a *Adapter) fetchPositions(params map[string]string) ([]exchange.Position, error) {
	data, err := a.client.Get("/api/v5/account/positions", params)
	if err != nil {
		return nil, fmt.Errorf("GetPosition: %w", err)
	}

	var raw []struct {
		InstID   string `json:"instId"`
		PosSide  string `json:"posSide"`
		Pos      string `json:"pos"`
		AvailPos string `json:"availPos"`
		AvgPx    string `json:"avgPx"`
		Upl      string `json:"upl"`
		Lever    string `json:"lever"`
		MgnMode    string `json:"mgnMode"`
		LiqPx      string `json:"liqPx"`
		MarkPx     string `json:"markPx"`
		FundingFee string `json:"fundingFee"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("GetPosition unmarshal: %w", err)
	}

	out := make([]exchange.Position, 0, len(raw))
	for _, p := range raw {
		holdSide := p.PosSide
		if holdSide == "net" {
			amt, _ := strconv.ParseFloat(p.Pos, 64)
			if amt >= 0 {
				holdSide = "long"
			} else {
				holdSide = "short"
			}
		}

		symbol := fromOKXInstID(p.InstID)
		ctVal := a.getCtVal(symbol)

		// OKX pos is in contracts; convert to base units.
		pos, _ := strconv.ParseFloat(p.Pos, 64)
		absBase := math.Abs(pos) * ctVal
		absPos := strconv.FormatFloat(absBase, 'f', -1, 64)

		availF, _ := strconv.ParseFloat(p.AvailPos, 64)
		availBase := math.Abs(availF) * ctVal
		availPos := strconv.FormatFloat(availBase, 'f', -1, 64)
		if p.AvailPos == "" {
			availPos = absPos
		}

		marginMode := "crossed"
		if p.MgnMode == "isolated" {
			marginMode = "isolated"
		}

		out = append(out, exchange.Position{
			Symbol:           symbol,
			HoldSide:         holdSide,
			Total:            absPos,
			Available:        availPos,
			AverageOpenPrice: p.AvgPx,
			UnrealizedPL:     p.Upl,
			Leverage:         p.Lever,
			MarginMode:       marginMode,
			LiquidationPrice: p.LiqPx,
			MarkPrice:        p.MarkPx,
			FundingFee:       p.FundingFee,
		})
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Account Config
// ---------------------------------------------------------------------------

func (a *Adapter) SetLeverage(symbol string, leverage string, holdSide string) error {
	instID := toOKXInstID(symbol)

	body := map[string]interface{}{
		"instId":  instID,
		"lever":   leverage,
		"mgnMode": "cross",
	}

	_, err := a.client.Post("/api/v5/account/set-leverage", body)
	if err != nil {
		return fmt.Errorf("SetLeverage: %w", err)
	}
	return nil
}

func (a *Adapter) SetMarginMode(symbol string, mode string) error {
	// OKX margin mode (cross/isolated) is set per-instrument via set-leverage.
	// This is a no-op here because SetLeverage already passes mgnMode=cross.
	// Position mode (net_mode) is set once in ensureNetMode, called from LoadAllContracts.
	return nil
}

// ensureNetMode sets the account to net_mode (one-way) for consistency with
// all other exchange adapters. Called once during LoadAllContracts.
func (a *Adapter) ensureNetMode() {
	body := map[string]interface{}{
		"posMode": "net_mode",
	}
	_, err := a.client.Post("/api/v5/account/set-position-mode", body)
	if err != nil {
		if apiErr, ok := err.(*APIError); ok && apiErr.Code == "59000" {
			return // already in net_mode
		}
		// Log but don't fail — position mode may already be correct.
		fmt.Printf("[okx] warning: set-position-mode: %v\n", err)
	}
}

// ---------------------------------------------------------------------------
// Contract Info
// ---------------------------------------------------------------------------

func (a *Adapter) LoadAllContracts() (map[string]exchange.ContractInfo, error) {
	// Ensure one-way (net) position mode on first load.
	a.ensureNetMode()

	params := map[string]string{
		"instType": "SWAP",
	}
	data, err := a.client.Get("/api/v5/public/instruments", params)
	if err != nil {
		return nil, fmt.Errorf("LoadAllContracts: %w", err)
	}

	var raw []struct {
		InstID    string `json:"instId"`
		TickSz    string `json:"tickSz"`
		LotSz     string `json:"lotSz"`
		MinSz     string `json:"minSz"`
		CtVal     string `json:"ctVal"`
		State     string `json:"state"`
		SettleCcy string `json:"settleCcy"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("LoadAllContracts unmarshal: %w", err)
	}

	result := make(map[string]exchange.ContractInfo, len(raw))
	for _, inst := range raw {
		if inst.State != "live" {
			continue
		}
		// Only include USDT-settled contracts
		if inst.SettleCcy != "USDT" {
			continue
		}

		symbol := fromOKXInstID(inst.InstID)
		lotSz, _ := strconv.ParseFloat(inst.MinSz, 64)
		stepLot, _ := strconv.ParseFloat(inst.LotSz, 64)
		priceStep, _ := strconv.ParseFloat(inst.TickSz, 64)
		ctVal, _ := strconv.ParseFloat(inst.CtVal, 64)

		// Convert contract units to base asset units for engine compatibility.
		minSize := lotSz * ctVal
		stepSize := stepLot * ctVal

		// Cache contract value for sizing calculations
		a.ctValCache.Store(inst.InstID, ctVal)

		result[symbol] = exchange.ContractInfo{
			Symbol:        symbol,
			MinSize:       minSize,
			StepSize:      stepSize,
			MaxSize:       0, // OKX does not expose maxSz in instruments
			SizeDecimals:  countDecimalsFloat(stepSize),
			PriceStep:     priceStep,
			PriceDecimals: countDecimals(inst.TickSz),
		}
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("no contracts loaded from OKX instruments")
	}
	return result, nil
}

// ---------------------------------------------------------------------------
// Funding Rate
// ---------------------------------------------------------------------------

func (a *Adapter) GetFundingRate(symbol string) (*exchange.FundingRate, error) {
	instID := toOKXInstID(symbol)
	params := map[string]string{
		"instId": instID,
	}

	data, err := a.client.Get("/api/v5/public/funding-rate", params)
	if err != nil {
		return nil, fmt.Errorf("GetFundingRate: %w", err)
	}

	var raw []struct {
		InstID          string `json:"instId"`
		FundingRate     string `json:"fundingRate"`
		NextFundingRate string `json:"nextFundingRate"`
		FundingTime     string `json:"fundingTime"`
		MaxFundingRate  string `json:"maxFundingRate"`
		MinFundingRate  string `json:"minFundingRate"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("GetFundingRate unmarshal: %w", err)
	}
	if len(raw) == 0 {
		return nil, fmt.Errorf("GetFundingRate: no data for %s", symbol)
	}

	rate, _ := strconv.ParseFloat(raw[0].FundingRate, 64)
	nextRate, _ := strconv.ParseFloat(raw[0].NextFundingRate, 64)
	fundingTimeMs, _ := strconv.ParseInt(raw[0].FundingTime, 10, 64)
	nextFunding := time.UnixMilli(fundingTimeMs)

	interval, err := a.GetFundingInterval(symbol)
	if err != nil {
		interval = 8 * time.Hour
	}

	fr := &exchange.FundingRate{
		Symbol:      symbol,
		Rate:        rate,
		NextRate:    nextRate,
		Interval:    interval,
		NextFunding: nextFunding,
	}

	if raw[0].MaxFundingRate != "" {
		if v, err := strconv.ParseFloat(raw[0].MaxFundingRate, 64); err == nil {
			fr.MaxRate = &v
		}
	}
	if raw[0].MinFundingRate != "" {
		if v, err := strconv.ParseFloat(raw[0].MinFundingRate, 64); err == nil {
			fr.MinRate = &v
		}
	}

	return fr, nil
}

func (a *Adapter) GetFundingInterval(symbol string) (time.Duration, error) {
	// OKX does not directly expose the funding interval in the funding-rate
	// endpoint.  We infer it from two consecutive historical funding timestamps.
	instID := toOKXInstID(symbol)
	params := map[string]string{
		"instId": instID,
		"limit":  "2",
	}

	data, err := a.client.Get("/api/v5/public/funding-rate-history", params)
	if err != nil {
		return 8 * time.Hour, nil
	}

	var raw []struct {
		FundingTime string `json:"fundingTime"`
	}
	if err := json.Unmarshal(data, &raw); err != nil || len(raw) < 2 {
		return 8 * time.Hour, nil
	}

	t1, _ := strconv.ParseInt(raw[0].FundingTime, 10, 64)
	t2, _ := strconv.ParseInt(raw[1].FundingTime, 10, 64)
	if t1 > 0 && t2 > 0 {
		diff := t1 - t2
		if diff < 0 {
			diff = -diff
		}
		interval := time.Duration(diff) * time.Millisecond
		// Sanity check: should be 1h, 4h, or 8h
		if interval >= 1*time.Hour && interval <= 24*time.Hour {
			return interval, nil
		}
	}

	return 8 * time.Hour, nil
}

// ---------------------------------------------------------------------------
// Balance
// ---------------------------------------------------------------------------

func (a *Adapter) GetFuturesBalance() (*exchange.Balance, error) {
	data, err := a.client.Get("/api/v5/account/balance", map[string]string{"ccy": "USDT"})
	if err != nil {
		return nil, fmt.Errorf("GetFuturesBalance: %w", err)
	}

	var raw []struct {
		MgnRatio string `json:"mgnRatio"`
		Details  []struct {
			Ccy       string `json:"ccy"`
			Eq        string `json:"eq"`
			AvailEq   string `json:"availEq"`
			FrozenBal string `json:"frozenBal"`
		} `json:"details"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("GetFuturesBalance unmarshal: %w", err)
	}

	if len(raw) > 0 {
		// OKX mgnRatio is margin ratio at account level
		// OKX returns mgnRatio as a multiplier (e.g. "100" means safe).
		// We need maintenanceMargin/equity. mgnRatio = equity/maintMargin,
		// so our ratio = 1/mgnRatio. If mgnRatio is very large, ratio is ~0 (safe).
		var marginRatio float64
		mgnRatio, _ := strconv.ParseFloat(raw[0].MgnRatio, 64)
		if mgnRatio > 0 {
			marginRatio = 1.0 / mgnRatio
		}

		for _, d := range raw[0].Details {
			if d.Ccy == "USDT" {
				total, _ := strconv.ParseFloat(d.Eq, 64)
				available, _ := strconv.ParseFloat(d.AvailEq, 64)
				frozen, _ := strconv.ParseFloat(d.FrozenBal, 64)

				// Defensive: if availEq is 0 but equity exists, fall back.
				if available <= 0 && total > 0 {
					available = total - frozen
				}

				return &exchange.Balance{
					Total:       total,
					Available:   available,
					Frozen:      frozen,
					Currency:    "USDT",
					MarginRatio: marginRatio,
				}, nil
			}
		}
		return &exchange.Balance{Currency: "USDT", MarginRatio: marginRatio}, nil
	}
	return &exchange.Balance{Currency: "USDT"}, nil
}

func (a *Adapter) GetSpotBalance() (*exchange.Balance, error) {
	data, err := a.client.Get("/api/v5/asset/balances", map[string]string{"ccy": "USDT"})
	if err != nil {
		return nil, fmt.Errorf("GetSpotBalance: %w", err)
	}

	var raw []struct {
		Ccy       string `json:"ccy"`
		Bal       string `json:"bal"`
		AvailBal  string `json:"availBal"`
		FrozenBal string `json:"frozenBal"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("GetSpotBalance unmarshal: %w", err)
	}

	for _, d := range raw {
		if d.Ccy == "USDT" {
			total, _ := strconv.ParseFloat(d.Bal, 64)
			available, _ := strconv.ParseFloat(d.AvailBal, 64)
			frozen, _ := strconv.ParseFloat(d.FrozenBal, 64)
			return &exchange.Balance{
				Total:     total,
				Available: available,
				Frozen:    frozen,
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
	instID := toOKXInstID(symbol)
	params := map[string]string{
		"instId": instID,
		"sz":     strconv.Itoa(depth),
	}

	data, err := a.client.Get("/api/v5/market/books", params)
	if err != nil {
		return nil, fmt.Errorf("GetOrderbook: %w", err)
	}

	var raw []struct {
		Bids [][]string `json:"bids"` // [[price, qty, _, numOrders], ...]
		Asks [][]string `json:"asks"`
		Ts   string     `json:"ts"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("GetOrderbook unmarshal: %w", err)
	}
	if len(raw) == 0 {
		return &exchange.Orderbook{Symbol: symbol, Time: time.Now()}, nil
	}

	// OKX orderbook quantities are in contracts — convert to base units via ctVal.
	ctVal := a.getCtVal(symbol)
	parseLevels := func(levels [][]string) []exchange.PriceLevel {
		out := make([]exchange.PriceLevel, 0, len(levels))
		for _, l := range levels {
			if len(l) < 2 {
				continue
			}
			price, _ := strconv.ParseFloat(l[0], 64)
			qty, _ := strconv.ParseFloat(l[1], 64)
			qty *= ctVal // contracts → base units
			out = append(out, exchange.PriceLevel{Price: price, Quantity: qty})
		}
		return out
	}

	tsMs, _ := strconv.ParseInt(raw[0].Ts, 10, 64)

	return &exchange.Orderbook{
		Symbol: symbol,
		Bids:   parseLevels(raw[0].Bids),
		Asks:   parseLevels(raw[0].Asks),
		Time:   time.UnixMilli(tsMs),
	}, nil
}

// ---------------------------------------------------------------------------
// Withdraw
// ---------------------------------------------------------------------------

// TransferToFutures moves funds from funding account (type 6) to trading account (type 18).
// OKX "unified" mode still separates funding vs trading below the 10,000 USDT threshold,
// so an explicit transfer is required before the bot can open positions.
func (a *Adapter) TransferToFutures(coin string, amount string) error {
	body := map[string]interface{}{
		"ccy":  coin,
		"amt":  amount,
		"from": "6",  // funding account
		"to":   "18", // trading account
	}

	_, err := a.client.Post("/api/v5/asset/transfer", body)
	if err != nil {
		return fmt.Errorf("TransferToFutures: %w", err)
	}
	return nil
}

// TransferToSpot moves funds from trading account to funding account.
func (a *Adapter) TransferToSpot(coin string, amount string) error {
	body := map[string]interface{}{
		"ccy":  coin,
		"amt":  amount,
		"from": "18", // trading account
		"to":   "6",  // funding account
	}

	_, err := a.client.Post("/api/v5/asset/transfer", body)
	if err != nil {
		return fmt.Errorf("TransferToSpot: %w", err)
	}
	return nil
}

func (a *Adapter) Withdraw(params exchange.WithdrawParams) (*exchange.WithdrawResult, error) {
	chain := mapChainToOKX(params.Coin, params.Chain)
	body := map[string]interface{}{
		"ccy":    params.Coin,
		"amt":    params.Amount,
		"dest":   "4", // on-chain withdrawal
		"toAddr": params.Address,
		"chain":  chain,
	}

	data, err := a.client.Post("/api/v5/asset/withdrawal", body)
	if err != nil {
		return nil, fmt.Errorf("Withdraw: %w", err)
	}

	var resp []struct {
		WdID string `json:"wdId"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("Withdraw unmarshal: %w", err)
	}
	if len(resp) == 0 {
		return nil, fmt.Errorf("Withdraw: empty response")
	}
	return &exchange.WithdrawResult{
		TxID:   resp[0].WdID,
		Status: "submitted",
	}, nil
}

func mapChainToOKX(coin, chain string) string {
	switch chain {
	case "BEP20":
		return coin + "-BSC"
	case "APT":
		return coin + "-Aptos"
	default:
		return coin + "-" + chain
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func mapOrderType(t string) string {
	switch strings.ToLower(t) {
	case "limit":
		return "limit"
	case "market":
		return "market"
	default:
		return strings.ToLower(t)
	}
}

// mapOKXOrdType maps order type + force (TIF) to OKX's ordType field.
// OKX encodes IOC/FOK/post_only directly in ordType rather than a separate TIF field.
// Market orders always stay "market" regardless of force — IOC/FOK only apply to limit orders.
func mapOKXOrdType(orderType, force string) string {
	if strings.ToLower(orderType) == "market" {
		return "market"
	}
	force = strings.ToLower(force)
	if force == "ioc" || force == "fok" || force == "post_only" {
		return force
	}
	return mapOrderType(orderType)
}

// mapState converts OKX order state to a normalised status string.
func mapState(state string) string {
	switch state {
	case "live":
		return "open"
	case "partially_filled":
		return "partially_filled"
	case "filled":
		return "filled"
	case "canceled":
		return "cancelled"
	default:
		return state
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

func countDecimalsFloat(v float64) int {
	s := strconv.FormatFloat(v, 'f', -1, 64)
	return countDecimals(s)
}

// GetUserTrades returns filled trades for a symbol since startTime.
// OKX endpoint: GET /api/v5/trade/fills-history
func (a *Adapter) GetUserTrades(symbol string, startTime time.Time, limit int) ([]exchange.Trade, error) {
	if limit <= 0 || limit > 100 {
		limit = 100 // OKX fills-history max is 100
	}
	instID := toOKXInstID(symbol)
	params := map[string]string{
		"instType": "SWAP",
		"instId":   instID,
		"begin":    strconv.FormatInt(startTime.UnixMilli(), 10),
		"limit":    strconv.Itoa(limit),
	}
	body, err := a.client.Get("/api/v5/trade/fills-history", params)
	if err != nil {
		return nil, fmt.Errorf("GetUserTrades: %w", err)
	}

	var fills []struct {
		TradeID string `json:"tradeId"`
		OrderID string `json:"ordId"`
		InstID  string `json:"instId"`
		Side    string `json:"side"` // buy or sell
		FillPx  string `json:"fillPx"`
		FillSz  string `json:"fillSz"`
		Fee     string `json:"fee"` // negative value = cost
		FeeCcy  string `json:"feeCcy"`
		TS      string `json:"ts"` // ms timestamp
	}
	if err := json.Unmarshal(body, &fills); err != nil {
		return nil, fmt.Errorf("GetUserTrades unmarshal: %w", err)
	}

	// OKX fillSz is in contracts; convert to base units using ctVal.
	ctVal := a.getCtVal(symbol)

	trades := make([]exchange.Trade, 0, len(fills))
	for _, t := range fills {
		price, _ := strconv.ParseFloat(t.FillPx, 64)
		qty, _ := strconv.ParseFloat(t.FillSz, 64)
		qty *= ctVal // contracts → base units
		fee, _ := strconv.ParseFloat(t.Fee, 64)
		if fee < 0 {
			fee = -fee // OKX returns negative fee
		}
		ms, _ := strconv.ParseInt(t.TS, 10, 64)
		trades = append(trades, exchange.Trade{
			TradeID:  t.TradeID,
			OrderID:  t.OrderID,
			Symbol:   fromOKXInstID(t.InstID),
			Side:     strings.ToLower(t.Side),
			Price:    price,
			Quantity: qty,
			Fee:      fee,
			FeeCoin:  t.FeeCcy,
			Time:     time.UnixMilli(ms),
		})
	}
	return trades, nil
}

// GetFundingFees returns funding fee history for a symbol since the given time.
func (a *Adapter) GetFundingFees(symbol string, since time.Time) ([]exchange.FundingPayment, error) {
	instID := toOKXInstID(symbol)
	params := map[string]string{
		"instType": "SWAP",
		"instId":   instID,
		"type":     "8",
		"begin":    strconv.FormatInt(since.UnixMilli(), 10),
		"limit":    "100",
	}
	body, err := a.client.Get("/api/v5/account/bills", params)
	if err != nil {
		return nil, fmt.Errorf("GetFundingFees: %w", err)
	}

	// OKX client already unwraps the { data: [...] } envelope,
	// so body is the raw data array.
	var records []struct {
		BalChg string `json:"balChg"`
		TS     string `json:"ts"`
	}
	if err := json.Unmarshal(body, &records); err != nil {
		return nil, fmt.Errorf("GetFundingFees unmarshal: %w", err)
	}

	out := make([]exchange.FundingPayment, 0, len(records))
	for _, r := range records {
		amt, _ := strconv.ParseFloat(r.BalChg, 64)
		ms, _ := strconv.ParseInt(r.TS, 10, 64)
		out = append(out, exchange.FundingPayment{
			Amount: amt,
			Time:   time.UnixMilli(ms),
		})
	}
	return out, nil
}

// GetClosePnL returns exchange-reported position-level PnL for recently closed positions.
func (a *Adapter) GetClosePnL(symbol string, since time.Time) ([]exchange.ClosePnL, error) {
	instID := toOKXInstID(symbol)
	params := map[string]string{
		"instType": "SWAP",
		"instId":   instID,
		"limit":    "20",
	}
	body, err := a.client.Get("/api/v5/account/positions-history", params)
	if err != nil {
		return nil, fmt.Errorf("GetClosePnL: %w", err)
	}

	// OKX client already unwraps the { data: [...] } envelope,
	// so body is the raw data array.
	var records []struct {
		Pnl           string `json:"pnl"`         // PnL excluding fees
		Fee           string `json:"fee"`          // trading fee (negative = charged)
		FundingFee    string `json:"fundingFee"`   // funding fee
		RealizedPnl   string `json:"realizedPnl"`  // = pnl + fee + fundingFee + liqPenalty
		OpenAvgPx     string `json:"openAvgPx"`
		CloseAvgPx    string `json:"closeAvgPx"`
		CloseTotalPos string `json:"closeTotalPos"`
		PosSide       string `json:"posSide"` // "long", "short", or "net" (one-way mode)
		Direction     string `json:"direction"` // "long" or "short" (available directly)
		OpenMaxPos    string `json:"openMaxPos"`
		UTime         string `json:"uTime"`
	}
	if err := json.Unmarshal(body, &records); err != nil {
		return nil, fmt.Errorf("GetClosePnL unmarshal: %w", err)
	}

	out := make([]exchange.ClosePnL, 0, len(records))
	for _, r := range records {
		ms, _ := strconv.ParseInt(r.UTime, 10, 64)

		// OKX positions-history has no server-side time filter —
		// filter client-side using the since parameter.
		if time.UnixMilli(ms).Before(since) {
			continue
		}

		pricePnL, _ := strconv.ParseFloat(r.Pnl, 64)
		fee, _ := strconv.ParseFloat(r.Fee, 64)
		fundingFee, _ := strconv.ParseFloat(r.FundingFee, 64)
		realizedPnL, _ := strconv.ParseFloat(r.RealizedPnl, 64)
		entryPrice, _ := strconv.ParseFloat(r.OpenAvgPx, 64)
		exitPrice, _ := strconv.ParseFloat(r.CloseAvgPx, 64)
		closeSize, _ := strconv.ParseFloat(r.CloseTotalPos, 64)
		closeSize *= a.getCtVal(symbol) // contracts → base units

		// Use realizedPnl if available (= pnl + fee + fundingFee + liqPenalty),
		// otherwise fall back to pricePnL.
		netPnL := realizedPnL
		if netPnL == 0 && pricePnL != 0 {
			netPnL = pricePnL + fee + fundingFee
		}

		// Determine side: prefer direction field, fall back to posSide/inference.
		side := r.Direction
		if side == "" {
			side = r.PosSide
		}
		if side == "net" || side == "" {
			maxPos, _ := strconv.ParseFloat(r.OpenMaxPos, 64)
			if maxPos > 0 {
				side = "long"
			} else if maxPos < 0 {
				side = "short"
			} else if entryPrice > 0 && exitPrice > 0 {
				if exitPrice > entryPrice {
					side = "long"
				} else {
					side = "short"
				}
			}
		}

		out = append(out, exchange.ClosePnL{
			PricePnL:   pricePnL,
			Fees:       fee,
			Funding:    fundingFee,
			NetPnL:     netPnL,
			EntryPrice: entryPrice,
			ExitPrice:  exitPrice,
			CloseSize:  closeSize,
			Side:       side,
			CloseTime:  time.UnixMilli(ms),
		})
	}
	return out, nil
}

// PlaceStopLoss places an algo order (stop-loss) on OKX.
func (a *Adapter) PlaceStopLoss(params exchange.StopLossParams) (string, error) {
	instID := toOKXInstID(params.Symbol)

	// Convert base units to contracts.
	sz := params.Size
	ctVal := a.getCtVal(params.Symbol)
	if ctVal != 1 {
		sizeF, err := strconv.ParseFloat(params.Size, 64)
		if err == nil {
			contracts := math.Round(sizeF / ctVal)
			sz = strconv.FormatFloat(contracts, 'f', 0, 64)
		}
	}

	body := map[string]interface{}{
		"instId":      instID,
		"tdMode":      "cross",
		"side":        string(params.Side),
		"ordType":     "conditional",
		"sz":          sz,
		"slTriggerPx": params.TriggerPrice,
		"slOrdPx":     "-1", // market price
		"reduceOnly":  true,
	}

	data, err := a.client.Post("/api/v5/trade/order-algo", body)
	if err != nil {
		return "", fmt.Errorf("PlaceStopLoss: %w", err)
	}

	var resp []struct {
		AlgoID string `json:"algoId"`
		SCode  string `json:"sCode"`
		SMsg   string `json:"sMsg"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", fmt.Errorf("PlaceStopLoss unmarshal: %w", err)
	}
	if len(resp) == 0 {
		return "", fmt.Errorf("PlaceStopLoss: empty response")
	}
	if resp[0].SCode != "0" {
		return "", fmt.Errorf("PlaceStopLoss: code=%s msg=%s", resp[0].SCode, resp[0].SMsg)
	}
	return resp[0].AlgoID, nil
}

// CancelStopLoss cancels an algo order (stop-loss) on OKX.
func (a *Adapter) CancelStopLoss(symbol, orderID string) error {
	instID := toOKXInstID(symbol)
	body := []map[string]interface{}{
		{
			"instId": instID,
			"algoId": orderID,
		},
	}

	_, err := a.client.Post("/api/v5/trade/cancel-algos", body)
	if err != nil {
		if apiErr, ok := err.(*APIError); ok {
			// 51400: algo order does not exist / already triggered
			if apiErr.Code == "51400" || apiErr.Code == "51401" {
				return nil
			}
		}
		return fmt.Errorf("CancelStopLoss: %w", err)
	}
	return nil
}

// EnsureOneWayMode sets the account to net (one-way) position mode.
func (a *Adapter) EnsureOneWayMode() error {
	body := map[string]string{
		"posMode": "net_mode",
	}
	_, err := a.client.Post("/api/v5/account/set-position-mode", body)
	if err != nil {
		errMsg := err.Error()
		// Already in net mode or no change needed.
		if strings.Contains(errMsg, "already") || strings.Contains(errMsg, "no need") {
			return nil
		}
		// 59000: can't change mode with open positions/orders.
		// Verify current mode — if already net_mode, suppress the error.
		if strings.Contains(errMsg, "59000") {
			if mode := a.getPositionMode(); mode == "net_mode" {
				return nil
			}
		}
		return fmt.Errorf("EnsureOneWayMode: %w", err)
	}
	return nil
}

// Close terminates all WebSocket connections for graceful shutdown.
func (a *Adapter) Close() {
	a.priceMu.Lock()
	if a.priceConn != nil {
		a.priceConn.Close()
		a.priceConn = nil
	}
	a.priceMu.Unlock()
	a.privMu.Lock()
	if a.privConn != nil {
		a.privConn.Close()
		a.privConn = nil
	}
	a.privMu.Unlock()
}

// getPositionMode queries the current position mode from OKX.
func (a *Adapter) getPositionMode() string {
	data, err := a.client.Get("/api/v5/account/config", nil)
	if err != nil {
		return ""
	}
	var resp []struct {
		PosMode string `json:"posMode"`
	}
	if json.Unmarshal(data, &resp) == nil && len(resp) > 0 {
		return resp[0].PosMode
	}
	return ""
}
