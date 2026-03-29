package scraper

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"arb/internal/database"
	"arb/pkg/utils"

	"github.com/chromedp/chromedp"
)

const (
	pageURL  = "https://www.coinglass.com/ArbitrageList"
	redisKey = "coinGlassSpotArb"
)

// Opportunity represents a single spot-futures arbitrage entry from CoinGlass.
type Opportunity struct {
	Symbol      string `json:"symbol"`
	Portfolio   string `json:"portfolio"`
	Exchange    string `json:"exchange"`
	FundingRate string `json:"fundingRate"`
	PnL         string `json:"pnl"`
	CumRate3D   string `json:"cumRate3d"`
	Revenue3D   string `json:"revenue3d"`
	APR         string `json:"apr"`
	AnnualRev   string `json:"annualRev"`
}

// Payload is the JSON structure written to Redis.
type Payload struct {
	Timestamp    string        `json:"timestamp"`
	TotalScraped int           `json:"totalScraped"`
	Data         []Opportunity `json:"data"`
}

// SpotArbConfig holds configuration for the spot-futures arbitrage scraper.
type SpotArbConfig struct {
	Enabled    bool
	Schedule   string // comma-separated minutes, e.g. "15,35"
	ChromePath string
}

// StartSpotArbScraper launches the CoinGlass spot-futures arbitrage scraper
// as a background goroutine on the configured schedule.
func StartSpotArbScraper(cfg SpotArbConfig, db *database.Client, log *utils.Logger) {
	minutes := parseSchedule(cfg.Schedule)
	if len(minutes) == 0 {
		log.Warn("spotarb: no valid schedule minutes, defaulting to 15,35")
		minutes = []int{15, 35}
	}
	log.Info("spotarb: scraper started (schedule: minute %s every hour, chrome: %s)",
		cfg.Schedule, cfg.ChromePath)

	go func() {
		// Run once immediately at startup.
		runScrape(cfg.ChromePath, db, log)

		for {
			now := time.Now()
			next := nextCronTime(now, minutes)
			log.Info("spotarb: next run at %s (in %s)",
				next.Format("15:04:05"), time.Until(next).Round(time.Second))
			time.Sleep(time.Until(next))
			runScrape(cfg.ChromePath, db, log)
		}
	}()
}

// RunOnce performs a single scrape and writes to Redis. Used by the standalone CLI.
func RunOnce(chromePath string, db *database.Client, log *utils.Logger) (Payload, error) {
	opps, err := Scrape(chromePath)
	if err != nil {
		return Payload{}, err
	}

	payload := Payload{
		Timestamp:    time.Now().UTC().Format(time.RFC3339),
		TotalScraped: len(opps),
		Data:         opps,
	}

	if db != nil {
		if err := writeRedis(db, payload); err != nil {
			return payload, fmt.Errorf("redis write: %w", err)
		}
	}
	return payload, nil
}

func runScrape(chromePath string, db *database.Client, log *utils.Logger) {
	log.Info("spotarb: fetching spot-futures arbitrage data from CoinGlass...")

	opps, err := Scrape(chromePath)
	if err != nil {
		log.Error("spotarb: scrape failed: %v", err)
		return
	}

	payload := Payload{
		Timestamp:    time.Now().UTC().Format(time.RFC3339),
		TotalScraped: len(opps),
		Data:         opps,
	}

	if err := writeRedis(db, payload); err != nil {
		log.Error("spotarb: redis write failed: %v", err)
	} else {
		log.Info("spotarb: written %d opportunities to Redis key %s", len(opps), redisKey)
	}
}

// Scrape fetches spot-futures arbitrage data from CoinGlass using headless Chrome.
func Scrape(chromePath string) ([]Opportunity, error) {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(chromePath),
		chromedp.Flag("headless", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-setuid-sandbox", true),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.UserAgent("Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
		chromedp.WindowSize(1920, 1080),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 90*time.Second)
	defer cancel()

	var rawRows [][]string

	err := chromedp.Run(ctx,
		chromedp.Navigate(pageURL),
		chromedp.Sleep(15*time.Second),
		chromedp.Evaluate(`(() => {
			const result = [];
			document.querySelectorAll('table tbody tr').forEach(row => {
				const cells = [];
				row.querySelectorAll('td').forEach(td => cells.push(td.innerText.trim()));
				if (cells.some(c => c.length > 0)) result.push(cells);
			});
			return result;
		})()`, &rawRows),
	)
	if err != nil {
		return nil, fmt.Errorf("chromedp: %w", err)
	}

	var opps []Opportunity
	for _, cells := range rawRows {
		if len(cells) < 9 || cells[0] == "" {
			continue
		}
		opps = append(opps, Opportunity{
			Symbol:      cells[0],
			Portfolio:   cells[1],
			Exchange:    cells[8],
			FundingRate: cells[2],
			PnL:         cells[3],
			CumRate3D:   cells[4],
			Revenue3D:   cells[5],
			APR:         cells[6],
			AnnualRev:   cells[7],
		})
	}
	return opps, nil
}

func writeRedis(db *database.Client, payload Payload) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return db.SetWithTTL(redisKey, string(data), 0)
}

func nextCronTime(now time.Time, minutes []int) time.Time {
	for _, m := range minutes {
		t := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), m, 0, 0, now.Location())
		if t.After(now) {
			return t
		}
	}
	return time.Date(now.Year(), now.Month(), now.Day(), now.Hour()+1, minutes[0], 0, 0, now.Location())
}

func parseSchedule(schedule string) []int {
	var minutes []int
	for _, s := range strings.Split(schedule, ",") {
		s = strings.TrimSpace(s)
		if m, err := strconv.Atoi(s); err == nil && m >= 0 && m <= 59 {
			minutes = append(minutes, m)
		}
	}
	return minutes
}
