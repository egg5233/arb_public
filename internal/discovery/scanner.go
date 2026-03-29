package discovery

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"arb/internal/config"
	"arb/internal/database"
	"arb/pkg/exchange"
	"arb/internal/models"
	"arb/pkg/utils"
)

// Compile-time check: *Scanner satisfies models.Discoverer.
var _ models.Discoverer = (*Scanner)(nil)

const (
	lorisURL = "https://api.loris.tools/funding"
)

// SupportedExchanges lists the five CEXes considered for arbitrage.
var SupportedExchanges = []string{"binance", "bybit", "gateio", "bitget", "okx", "bingx"}

// ScanType classifies the purpose of each scan cycle.
type ScanType string

const (
	NormalScan    ScanType = "normalScan"    // builds persistence history, feeds dashboard
	ExitScan      ScanType = "exitScan"      // triggers exit checks (:25 scan)
	RotateScan    ScanType = "rotateScan"    // triggers rotation checks (:45 scan)
	EntryScan     ScanType = "entryScan"     // triggers trade execution (:35 scan)
	RebalanceScan ScanType = "rebalanceScan" // triggers fund rebalancing (:20 scan)
)

// ScanResult carries opportunities from a scan cycle along with metadata
// about the scan timing, so the engine can decide whether to trigger execution.
type ScanResult struct {
	Opps []models.Opportunity
	Type ScanType
}

// scanRecord stores a single scan observation for persistence/stability tracking.
type scanRecord struct {
	Time   time.Time
	Spread float64
}

// Scanner polls the Loris API for funding rates, verifies them against
// exchange-native APIs, ranks arbitrage opportunities and broadcasts them.
type Scanner struct {
	exchanges map[string]exchange.Exchange
	contracts map[string]map[string]exchange.ContractInfo // exchange → symbol → info
	db        *database.Client
	cfg       *config.Config
	log       *utils.Logger
	client    *http.Client

	mu            sync.RWMutex
	opportunities []models.Opportunity

	scanHistory   map[string][]scanRecord // oppKey → scan records (time + spread)
	scanHistoryMu sync.Mutex

	lastLorisIntervals map[string]map[string]float64 // exchange → symbol → interval hours
	intervalsMu        sync.RWMutex

	// Backtest prefetch rate limiting.
	prefetchMu       sync.Mutex
	lorisBackoffMu   sync.RWMutex
	lorisBackoffUntil time.Time

	oppChan  chan ScanResult
	stopCh   chan struct{}
	wg       sync.WaitGroup
	rejStore *models.RejectionStore
}

// NewScanner creates a Scanner with the given dependencies.
func NewScanner(exchanges map[string]exchange.Exchange, db *database.Client, cfg *config.Config) *Scanner {
	return &Scanner{
		exchanges: exchanges,
		db:        db,
		cfg:       cfg,
		log:       utils.NewLogger("discovery"),
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
		scanHistory: make(map[string][]scanRecord),
		oppChan:     make(chan ScanResult, 1),
		stopCh:      make(chan struct{}),
	}
}

// SetRejectionStore sets the shared rejection store for recording filtered opportunities.
func (s *Scanner) SetRejectionStore(store *models.RejectionStore) {
	s.rejStore = store
}

// SetContracts stores loaded contract maps for symbol validation.
func (s *Scanner) SetContracts(contracts map[string]map[string]exchange.ContractInfo) {
	s.contracts = contracts
}

// activeExchanges returns the names of exchanges that have loaded adapters.
func (s *Scanner) activeExchanges() []string {
	names := make([]string, 0, len(s.exchanges))
	for name := range s.exchanges {
		names = append(names, name)
	}
	return names
}

// hasExchange checks if an exchange adapter is loaded.
func (s *Scanner) hasExchange(name string) bool {
	_, ok := s.exchanges[name]
	return ok
}

// hasContract checks if a symbol exists on a given exchange.
func (s *Scanner) hasContract(exchName, symbol string) bool {
	if s.contracts == nil {
		return true // no contract data loaded, skip check
	}
	cm, ok := s.contracts[exchName]
	if !ok {
		return true
	}
	_, exists := cm[symbol]
	return exists
}

// nextScanTime returns the next scan time at or after t, using the configured schedule.
func nextScanTime(t time.Time, minutes []int) time.Time {
	hour := time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, t.Location())
	for _, m := range minutes {
		candidate := hour.Add(time.Duration(m) * time.Minute)
		if candidate.After(t) {
			return candidate
		}
	}
	// All this hour's scan times passed; first scan of next hour.
	return hour.Add(time.Hour).Add(time.Duration(minutes[0]) * time.Minute)
}

// Start launches a background goroutine that scans at the configured minutes each hour.
func (s *Scanner) Start() {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		// Run immediately on start.
		s.log.Info("Scanner started, running initial scan (schedule=%v rebalance=:%02d exit=:%02d entry=:%02d rotate=:%02d)",
			s.cfg.ScanMinutes, s.cfg.RebalanceScanMinute, s.cfg.ExitScanMinute, s.cfg.EntryScanMinute, s.cfg.RotateScanMinute)
		s.runCycleTagged(NormalScan)

		for {
			now := time.Now()
			next := nextScanTime(now, s.cfg.ScanMinutes)
			wait := time.Until(next)
			s.log.Info("next scan at %s (in %s)", next.Format("15:04:05"), wait.Round(time.Second))

			timer := time.NewTimer(wait)
			select {
			case <-timer.C:
				scanType := NormalScan
				if next.Minute() == s.cfg.RebalanceScanMinute {
					scanType = RebalanceScan
				} else if next.Minute() == s.cfg.EntryScanMinute {
					scanType = EntryScan
				} else if next.Minute() == s.cfg.ExitScanMinute {
					scanType = ExitScan
				} else if next.Minute() == s.cfg.RotateScanMinute {
					scanType = RotateScan
				}
				s.log.Info("scan firing at :%02d (type=%s)", next.Minute(), scanType)
				s.runCycleTagged(scanType)
			case <-s.stopCh:
				timer.Stop()
				s.log.Info("Scanner stopped")
				return
			}
		}
	}()
}

// Stop signals the background goroutine to exit and waits for it.
func (s *Scanner) Stop() {
	close(s.stopCh)
	s.wg.Wait()
}

// Poll fetches and parses the latest Loris funding rate data.
func (s *Scanner) Poll() (*models.LorisResponse, error) {
	req, err := http.NewRequest(http.MethodGet, lorisURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("loris request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("loris returned %d: %s", resp.StatusCode, string(body))
	}

	var loris models.LorisResponse
	if err := json.NewDecoder(resp.Body).Decode(&loris); err != nil {
		return nil, fmt.Errorf("decode loris response: %w", err)
	}
	return &loris, nil
}

// Scan runs a single poll-rank-verify cycle on demand and returns the
// resulting opportunities. This satisfies the Discoverer interface
// (Scan() []Opportunity) and is used by the engine at T-16min.
func (s *Scanner) Scan() []models.Opportunity {
	s.runCycle()
	return s.GetOpportunities()
}

// GetOpportunities returns the latest ranked opportunities (thread-safe).
func (s *Scanner) GetOpportunities() []models.Opportunity {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]models.Opportunity, len(s.opportunities))
	copy(out, s.opportunities)
	return out
}

// OpportunityChan returns a read-only channel that receives ScanResult
// each time a polling cycle completes successfully.
func (s *Scanner) OpportunityChan() <-chan ScanResult {
	return s.oppChan
}

// cgExchangeMap normalizes CoinGlass exchange names to internal IDs.
var cgExchangeMap = map[string]string{
	"Binance": "binance",
	"Bybit":   "bybit",
	"OKX":     "okx",
	"Bitget":  "bitget",
	"Gate":    "gateio",
}

const cgStaleThreshold = 60 * time.Minute

// PollCoinGlass reads CoinGlass arbitrage data from Redis.
func (s *Scanner) PollCoinGlass() (*models.CoinGlassResponse, error) {
	raw, err := s.db.JSONGet("coinGlassArb")
	if err != nil {
		return nil, fmt.Errorf("read coinGlassArb: %w", err)
	}

	// JSON.GET with $ path returns a JSON array wrapper, e.g. [{...}]
	var wrapper []models.CoinGlassResponse
	if err := json.Unmarshal(raw, &wrapper); err != nil {
		return nil, fmt.Errorf("decode coinGlassArb: %w", err)
	}
	if len(wrapper) == 0 {
		return nil, fmt.Errorf("coinGlassArb: empty response")
	}
	return &wrapper[0], nil
}

// coinGlassToOpportunities converts CoinGlass arb data into scored Opportunity structs.
func (s *Scanner) coinGlassToOpportunities(cg *models.CoinGlassResponse) []models.Opportunity {
	now := time.Now().UTC()
	valueOfTimeHours := s.cfg.MinHoldTime.Hours()
	valueOfRatio := s.cfg.MaxCostRatio

	var opps []models.Opportunity
	for _, item := range cg.Data {
		longEx, lok := cgExchangeMap[item.LongEx]
		shortEx, sok := cgExchangeMap[item.ShortEx]
		if !lok || !sok || longEx == shortEx {
			continue
		}
		// Skip if either exchange is not active
		if !s.hasExchange(longEx) || !s.hasExchange(shortEx) {
			continue
		}

		symbol := strings.ToUpper(item.Pair) + "USDT"

		// Skip if symbol doesn't exist on both paired exchanges
		if !s.hasContract(longEx, symbol) || !s.hasContract(shortEx, symbol) {
			continue
		}

		// Use annualYield to derive bps/h (FundingRate interval varies and is unknown).
		// "432.74%" → 4.3274 decimal → 43274 bps/year → 43274/8760 bps/h
		annualDecimal := parseCGPercent(item.AnnualYield)
		if annualDecimal <= 0 {
			continue
		}
		spreadBpsH := annualDecimal * 10000 / 8760.0

		// Fee filter (same logic as ranker)
		feesA := exchangeFees[longEx]
		feesB := exchangeFees[shortEx]
		totalFeePct := feesA.Taker + feesB.Taker + feesA.Taker + feesB.Taker
		totalFeeBps := totalFeePct * 100

		// spread is bps/h, so total earned = spread * valueOfTimeHours
		var costRatio float64
		denom := spreadBpsH * valueOfTimeHours
		if denom > 0 {
			costRatio = totalFeeBps / denom
		} else {
			continue
		}
		if costRatio >= valueOfRatio {
			continue
		}

		// Parse OI for scoring
		oiLong := parseCGDollarAmount(item.OILong)
		oiShort := parseCGDollarAmount(item.OIShort)
		minOI := math.Min(oiLong, oiShort)

		// OI-based rank approximation: higher OI = lower rank number = better
		oiRank := 500
		if minOI >= 10_000_000 {
			oiRank = 10
		} else if minOI >= 1_000_000 {
			oiRank = 50
		} else if minOI >= 100_000 {
			oiRank = 150
		}

		score := spreadBpsH * (1.0 + 1.0/float64(oiRank))

		// Derive approximate per-leg rates from CoinGlass FundingRate (short side rate).
		// FundingRate is the rate on the short (collecting) exchange, e.g. "0.1976%".
		// Long side rate ≈ short rate - spread.
		shortRateBpsH := parseCGPercent(item.FundingRate) * 10000 / 8760.0
		longRateBpsH := shortRateBpsH - spreadBpsH

		opps = append(opps, models.Opportunity{
			Symbol:        symbol,
			LongExchange:  longEx,
			ShortExchange: shortEx,
			LongRate:      longRateBpsH,
			ShortRate:     shortRateBpsH,
			Spread:        spreadBpsH, // bps/h
			CostRatio:     costRatio,
			OIRank:        oiRank,
			Score:         score,
			IntervalHours: 0, // unknown from CoinGlass
			Source:        "coinglass",
			Timestamp:     now,
		})
	}
	return opps
}

// parseCGPercent parses "432.74%" or "0.1976%" into a float (4.3274 or 0.001976).
func parseCGPercent(s string) float64 {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, "%")
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return f / 100 // percent to decimal
}

// parseCGDollarAmount parses "$1.64M", "$294.20K", "$500" into a float.
func parseCGDollarAmount(s string) float64 {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "$")
	s = strings.ReplaceAll(s, ",", "")

	multiplier := 1.0
	if strings.HasSuffix(s, "B") {
		multiplier = 1_000_000_000
		s = strings.TrimSuffix(s, "B")
	} else if strings.HasSuffix(s, "M") {
		multiplier = 1_000_000
		s = strings.TrimSuffix(s, "M")
	} else if strings.HasSuffix(s, "K") {
		multiplier = 1_000
		s = strings.TrimSuffix(s, "K")
	}

	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return f * multiplier
}

// mergeOpportunities deduplicates by (symbol, longExchange, shortExchange), keeping higher score.
func mergeOpportunities(a, b []models.Opportunity) []models.Opportunity {
	type key struct{ sym, long, short string }
	best := make(map[key]models.Opportunity)

	for _, opp := range a {
		k := key{opp.Symbol, opp.LongExchange, opp.ShortExchange}
		if existing, ok := best[k]; !ok || opp.Score > existing.Score {
			best[k] = opp
		}
	}
	for _, opp := range b {
		k := key{opp.Symbol, opp.LongExchange, opp.ShortExchange}
		if existing, ok := best[k]; !ok || opp.Score > existing.Score {
			best[k] = opp
		}
	}

	merged := make([]models.Opportunity, 0, len(best))
	for _, opp := range best {
		merged = append(merged, opp)
	}
	return merged
}

// oppKey returns the unique identity for an opportunity (direction-sensitive).
func oppKey(opp models.Opportunity) string {
	return opp.Symbol + "|" + opp.LongExchange + "|" + opp.ShortExchange
}

// recordScanHistory records that each opportunity was seen in this scan cycle.
func (s *Scanner) recordScanHistory(opps []models.Opportunity) {
	s.scanHistoryMu.Lock()
	defer s.scanHistoryMu.Unlock()

	now := time.Now()
	for _, opp := range opps {
		key := oppKey(opp)
		s.scanHistory[key] = append(s.scanHistory[key], scanRecord{Time: now, Spread: opp.Spread})
	}

	// Prune entries older than max lookback (8h tier)
	maxLookback := s.cfg.PersistLookback8h
	cutoff := now.Add(-maxLookback - time.Minute)
	for key, records := range s.scanHistory {
		i := 0
		for i < len(records) && records[i].Time.Before(cutoff) {
			i++
		}
		if i == len(records) {
			delete(s.scanHistory, key)
		} else if i > 0 {
			s.scanHistory[key] = records[i:]
		}
	}
}

// isPersistent checks if an opportunity has appeared enough times in the lookback window
// and, for low-OI pairs, whether the spread has been stable across scans.
// Returns empty string if persistent, or a rejection reason.
func (s *Scanner) isPersistent(opp models.Opportunity) string {
	s.scanHistoryMu.Lock()
	defer s.scanHistoryMu.Unlock()

	key := oppKey(opp)
	records := s.scanHistory[key]

	var lookback time.Duration
	var minCount int
	var stabilityRatio float64
	var oiRankThreshold int
	switch {
	case opp.IntervalHours <= 1:
		lookback = s.cfg.PersistLookback1h
		minCount = s.cfg.PersistMinCount1h
		stabilityRatio = s.cfg.SpreadStabilityRatio1h
		oiRankThreshold = s.cfg.SpreadStabilityOIRank1h
	case opp.IntervalHours <= 4:
		lookback = s.cfg.PersistLookback4h
		minCount = s.cfg.PersistMinCount4h
		stabilityRatio = s.cfg.SpreadStabilityRatio4h
		oiRankThreshold = s.cfg.SpreadStabilityOIRank4h
	default:
		lookback = s.cfg.PersistLookback8h
		minCount = s.cfg.PersistMinCount8h
		stabilityRatio = s.cfg.SpreadStabilityRatio8h
		oiRankThreshold = s.cfg.SpreadStabilityOIRank8h
	}

	cutoff := time.Now().Add(-lookback)
	count := 0
	var minSpread, maxSpread float64
	first := true
	for _, r := range records {
		if !r.Time.Before(cutoff) {
			count++
			if first || r.Spread < minSpread {
				minSpread = r.Spread
			}
			if first || r.Spread > maxSpread {
				maxSpread = r.Spread
			}
			first = false
		}
	}
	if count < minCount {
		return fmt.Sprintf("seen %d/%d times in %v lookback", count, minCount, lookback)
	}

	// Spread stability check: only for low-OI (high rank number) pairs
	if stabilityRatio > 0 && oiRankThreshold > 0 && opp.OIRank >= oiRankThreshold && maxSpread > 0 {
		ratio := minSpread / maxSpread
		if ratio < stabilityRatio {
			return fmt.Sprintf("spread unstable (min/max=%.2f/%.2f=%.2f < %.2f, OIRank=%d)", minSpread, maxSpread, ratio, stabilityRatio, opp.OIRank)
		}
	}

	return ""
}

// SetSymbolCooldown sets a cooldown on a symbol after a loss close (persisted to Redis with TTL).
func (s *Scanner) SetSymbolCooldown(symbol string, d time.Duration) {
	if err := s.db.SetLossCooldown(symbol, d); err != nil {
		s.log.Error("failed to set loss cooldown for %s: %v", symbol, err)
		return
	}
	expiry := time.Now().Add(d)
	s.log.Info("symbol cooldown set: %s until %s (%.1fh)", symbol, expiry.Format("15:04 UTC"), d.Hours())
}

// isSymbolCoolingDown returns a rejection reason if the symbol is in any cooldown (loss or re-entry), or "".
func (s *Scanner) isSymbolCoolingDown(symbol string) string {
	if remaining := s.db.GetLossCooldown(symbol); remaining > 0 {
		expiry := time.Now().Add(remaining)
		return fmt.Sprintf("loss cooldown until %s (%.0fmin remaining)", expiry.Format("15:04 UTC"), remaining.Minutes())
	}
	if remaining := s.db.GetReEnterCooldown(symbol); remaining > 0 {
		expiry := time.Now().Add(remaining)
		return fmt.Sprintf("re-enter cooldown until %s (%.0fmin remaining)", expiry.Format("15:04 UTC"), remaining.Minutes())
	}
	return ""
}

// SetReEnterCooldown sets a re-entry cooldown on a symbol after any close (persisted to Redis with TTL).
func (s *Scanner) SetReEnterCooldown(symbol string, d time.Duration) {
	if err := s.db.SetReEnterCooldown(symbol, d); err != nil {
		s.log.Error("failed to set re-enter cooldown for %s: %v", symbol, err)
		return
	}
	expiry := time.Now().Add(d)
	s.log.Info("re-enter cooldown set: %s until %s (%.1fh)", symbol, expiry.Format("15:04 UTC"), d.Hours())
}

// isSpreadVolatile computes the coefficient of variation (stddev/mean) for
// spread values across recent scans. Returns rejection reason if CV exceeds threshold.
func (s *Scanner) isSpreadVolatile(opp models.Opportunity) string {
	maxCV := s.cfg.SpreadVolatilityMaxCV
	if maxCV <= 0 {
		return "" // disabled
	}
	minSamples := s.cfg.SpreadVolatilityMinSamples
	if minSamples < 2 {
		minSamples = 2
	}

	s.scanHistoryMu.Lock()
	defer s.scanHistoryMu.Unlock()

	key := oppKey(opp)
	records := s.scanHistory[key]

	// Determine tier-appropriate lookback
	var lookback time.Duration
	switch {
	case opp.IntervalHours <= 1:
		lookback = s.cfg.PersistLookback1h
	case opp.IntervalHours <= 4:
		lookback = s.cfg.PersistLookback4h
	default:
		lookback = s.cfg.PersistLookback8h
	}

	cutoff := time.Now().Add(-lookback)
	var spreads []float64
	for _, r := range records {
		if !r.Time.Before(cutoff) {
			spreads = append(spreads, r.Spread)
		}
	}

	if len(spreads) < minSamples {
		return "" // not enough data to evaluate
	}

	// Compute mean
	var sum float64
	for _, v := range spreads {
		sum += v
	}
	mean := sum / float64(len(spreads))
	if mean <= 0 {
		return "" // can't compute CV with non-positive mean
	}

	// Compute population stddev
	var sqDiffSum float64
	for _, v := range spreads {
		d := v - mean
		sqDiffSum += d * d
	}
	stddev := math.Sqrt(sqDiffSum / float64(len(spreads)))
	cv := stddev / mean

	if cv > maxCV {
		return fmt.Sprintf("spread volatile (CV=%.2f > %.2f, n=%d)", cv, maxCV, len(spreads))
	}
	return ""
}

// runCycleTagged performs a scan cycle and broadcasts with the isLastScan flag.
func (s *Scanner) runCycleTagged(scanType ScanType) {
	s.runCycleInternal(scanType)
}

// runCycle performs a single poll-rank-verify-broadcast cycle (untagged, for Scan()).
func (s *Scanner) runCycle() {
	s.runCycleInternal(NormalScan)
}

// runCycleInternal performs a single poll-rank-verify-broadcast cycle.
func (s *Scanner) runCycleInternal(scanType ScanType) {
	// 1. Poll Loris
	loris, err := s.Poll()
	if err != nil {
		s.log.Error("Loris poll failed: %v", err)
	}

	var lorisOpps []models.Opportunity
	if loris != nil {
		s.log.Info("Polled Loris: %d symbols", len(loris.Symbols))
		lorisOpps = s.RankOpportunities(loris)

		// Cache funding intervals for backtest settlement validation.
		if loris.FundingIntervals != nil {
			s.intervalsMu.Lock()
			s.lastLorisIntervals = loris.FundingIntervals
			s.intervalsMu.Unlock()
		}
	}

	// 2. Poll CoinGlass from Redis
	var cgOpps []models.Opportunity
	cg, err := s.PollCoinGlass()
	if err != nil {
		s.log.Warn("CoinGlass poll failed: %v", err)
	} else {
		// Staleness check: skip if older than threshold
		ts, parseErr := time.Parse(time.RFC3339Nano, cg.Timestamp)
		if parseErr != nil {
			s.log.Warn("CoinGlass: invalid timestamp %q, skipping", cg.Timestamp)
		} else if time.Since(ts) > cgStaleThreshold {
			s.log.Warn("CoinGlass: data is %.0f min old, skipping (threshold %v)", time.Since(ts).Minutes(), cgStaleThreshold)
		} else {
			cgOpps = s.coinGlassToOpportunities(cg)
			s.log.Info("Polled CoinGlass: %d opportunities", len(cgOpps))
		}
	}

	// 3. Merge and deduplicate
	merged := mergeOpportunities(lorisOpps, cgOpps)
	if len(merged) == 0 {
		s.log.Info("No profitable opportunities found")
		return
	}

	// 4. Sort by score descending, take top N
	topN := s.cfg.TopOpportunities
	if topN <= 0 {
		topN = 5
	}
	// Sort
	for i := 0; i < len(merged); i++ {
		for j := i + 1; j < len(merged); j++ {
			if merged[j].Score > merged[i].Score {
				merged[i], merged[j] = merged[j], merged[i]
			}
		}
	}
	if len(merged) > topN {
		merged = merged[:topN]
	}

	s.log.Info("Merged: %d total opportunities (loris=%d, coinglass=%d), top %d selected",
		len(lorisOpps)+len(cgOpps), len(lorisOpps), len(cgOpps), len(merged))

	// 5. Verify rates via exchange APIs (also populates NextFunding)
	verified := s.VerifyRates(merged)
	s.log.Info("Verified %d/%d opportunities", len(verified), len(merged))

	// 5b. Record all verified opportunities in scan history (every scan, not just :35)
	s.recordScanHistory(verified)

	// 5c. On last scan, apply persistence filter before funding window check
	if scanType == EntryScan {
		var persistent []models.Opportunity
		for _, opp := range verified {
			if reason := s.isPersistent(opp); reason == "" {
				persistent = append(persistent, opp)
			} else {
				s.log.Info("filtering %s (%s/%s): %s (interval=%.0fh)",
					opp.Symbol, opp.LongExchange, opp.ShortExchange, reason, opp.IntervalHours)
				if s.rejStore != nil {
					s.rejStore.AddOpp(opp, "scanner", reason)
				}
			}
		}
		s.log.Info("Persistence filter: %d/%d passed", len(persistent), len(verified))
		verified = persistent
	}

	// 5d. On entry scan, apply spread volatility filter
	if scanType == EntryScan {
		var stable []models.Opportunity
		for _, opp := range verified {
			if reason := s.isSpreadVolatile(opp); reason == "" {
				stable = append(stable, opp)
			} else {
				s.log.Info("filtering %s (%s/%s): %s",
					opp.Symbol, opp.LongExchange, opp.ShortExchange, reason)
				if s.rejStore != nil {
					s.rejStore.AddOpp(opp, "scanner", reason)
				}
			}
		}
		s.log.Info("Volatility filter: %d/%d passed", len(stable), len(verified))
		verified = stable
	}

	// 5e. On entry scan, apply symbol cooldown filter
	if scanType == EntryScan {
		var notCooling []models.Opportunity
		for _, opp := range verified {
			if reason := s.isSymbolCoolingDown(opp.Symbol); reason == "" {
				notCooling = append(notCooling, opp)
			} else {
				s.log.Info("filtering %s (%s/%s): %s",
					opp.Symbol, opp.LongExchange, opp.ShortExchange, reason)
				if s.rejStore != nil {
					s.rejStore.AddOpp(opp, "scanner", reason)
				}
			}
		}
		s.log.Info("Symbol cooldown filter: %d/%d passed", len(notCooling), len(verified))
		verified = notCooling
	}

	// 5f. On entry scan, filter by max funding interval hours.
	if scanType == EntryScan && s.cfg.MaxIntervalHours > 0 {
		var intervalOK []models.Opportunity
		for _, opp := range verified {
			if opp.IntervalHours > 0 && opp.IntervalHours > s.cfg.MaxIntervalHours {
				reason := fmt.Sprintf("interval %.0fh exceeds max %.0fh", opp.IntervalHours, s.cfg.MaxIntervalHours)
				s.log.Info("filtering %s (%s/%s): %s", opp.Symbol, opp.LongExchange, opp.ShortExchange, reason)
				if s.rejStore != nil {
					s.rejStore.AddOpp(opp, "scanner", reason)
				}
				continue
			}
			intervalOK = append(intervalOK, opp)
		}
		s.log.Info("Interval filter: %d/%d passed (max %.0fh)", len(intervalOK), len(verified), s.cfg.MaxIntervalHours)
		verified = intervalOK
	}

	// 6. On entry scan, filter to only opportunities with imminent funding.
	if scanType == EntryScan {
		maxFundingWindow := time.Duration(s.cfg.FundingWindowMin) * time.Minute
		var imminent []models.Opportunity
		for _, opp := range verified {
			if opp.NextFunding.IsZero() || time.Until(opp.NextFunding) <= maxFundingWindow {
				imminent = append(imminent, opp)
			} else {
				reason := fmt.Sprintf("next funding %s is %.0fmin away (>%v)",
					opp.NextFunding.Format("15:04 UTC"),
					time.Until(opp.NextFunding).Minutes(), maxFundingWindow)
				s.log.Info("filtering %s: %s", opp.Symbol, reason)
				if s.rejStore != nil {
					s.rejStore.AddOpp(opp, "scanner", reason)
				}
			}
		}
		s.log.Info("Funding filter: %d/%d have imminent funding", len(imminent), len(verified))
		verified = imminent
	}

	// 7. On entry scan, apply historical backtest filter.
	if scanType == EntryScan && s.cfg.BacktestDays > 0 {
		var backtested []models.Opportunity
		for _, opp := range verified {
			if pass, reason := s.backtestFundingHistory(opp); pass {
				backtested = append(backtested, opp)
			} else {
				s.log.Info("filtering %s (%s/%s): %s",
					opp.Symbol, opp.LongExchange, opp.ShortExchange, reason)
				if s.rejStore != nil {
					s.rejStore.AddOpp(opp, "scanner", reason)
				}
			}
		}
		s.log.Info("Backtest filter: %d/%d passed", len(backtested), len(verified))
		verified = backtested
	}

	// 8. Filter delisted coins (all scan types — block from appearing anywhere).
	if s.cfg.DelistFilterEnabled {
		var notDelisted []models.Opportunity
		for _, opp := range verified {
			if s.IsDelisted(opp.Symbol) {
				s.log.Warn("filtering %s (%s/%s): coin delisting on Binance (date: %s)",
					opp.Symbol, opp.LongExchange, opp.ShortExchange, s.GetDelistDate(opp.Symbol))
				if s.rejStore != nil {
					s.rejStore.AddOpp(opp, "scanner", "coin delisting on Binance")
				}
				continue
			}
			notDelisted = append(notDelisted, opp)
		}
		if len(notDelisted) < len(verified) {
			s.log.Info("Delist filter: %d/%d passed", len(notDelisted), len(verified))
		}
		verified = notDelisted
	}

	for i, opp := range verified {
		s.log.Info("  [%d] %s (%s) | Long: %s (%.4f bps/h) | Short: %s (%.4f bps/h) | Spread: %.4f bps/h | Interval: %.0fh | CostRatio: %.4f | OIRank: %d | Score: %.4f | NextFunding: %s",
			i+1, opp.Symbol, opp.Source, opp.LongExchange, opp.LongRate, opp.ShortExchange, opp.ShortRate,
			opp.Spread, opp.IntervalHours, opp.CostRatio, opp.OIRank, opp.Score, opp.NextFunding.Format("15:04 UTC"))
	}

	s.mu.Lock()
	s.opportunities = verified
	s.mu.Unlock()

	// Pre-fetch backtest data for non-entry scans to warm cache.
	if scanType != EntryScan && s.cfg.BacktestDays > 0 && len(verified) > 0 {
		go s.prefetchBacktestData(verified)
	}

	// Non-blocking send to channel so the engine can consume at its pace.
	select {
	case s.oppChan <- ScanResult{Opps: verified, Type: scanType}:
	default:
	}
}
