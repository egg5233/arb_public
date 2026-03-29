// cmd/spotarb/main.go — Standalone CoinGlass spot-futures arbitrage scraper.
// Uses the shared internal/scraper package. Can run as one-shot or cron daemon.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"arb/internal/database"
	"arb/internal/scraper"
)

func main() {
	cronMode := flag.Bool("cron", false, "run as cron daemon (every 15,35 minutes)")
	noRedis := flag.Bool("no-redis", false, "skip writing to Redis")
	jsonOutput := flag.Bool("json", false, "output as JSON")
	chromePath := flag.String("chrome", "", "path to Chrome binary (auto-detect if empty)")
	flag.Parse()

	chrome := *chromePath
	if chrome == "" {
		chrome = "/home/solana/.cache/puppeteer/chrome/linux-146.0.7680.153/chrome-linux64/chrome"
	}

	if *cronMode {
		log.Printf("Cron mode started. Schedule: minute 15,35 every hour")
		run(chrome, *noRedis, *jsonOutput)
		for {
			now := time.Now()
			next := nextCronTime(now)
			log.Printf("Next run at %s (in %s)", next.Format("15:04:05"), time.Until(next).Round(time.Second))
			time.Sleep(time.Until(next))
			run(chrome, *noRedis, *jsonOutput)
		}
	} else {
		run(chrome, *noRedis, *jsonOutput)
	}
}

func nextCronTime(now time.Time) time.Time {
	minutes := []int{15, 35}
	for _, m := range minutes {
		t := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), m, 0, 0, now.Location())
		if t.After(now) {
			return t
		}
	}
	return time.Date(now.Year(), now.Month(), now.Day(), now.Hour()+1, minutes[0], 0, 0, now.Location())
}

func run(chromePath string, noRedis, jsonOutput bool) {
	log.Printf("Fetching spot-futures arbitrage data from CoinGlass...")

	var db *database.Client
	if !noRedis {
		var err error
		db, err = database.New("localhost:6379", readRedisPassword(), 2)
		if err != nil {
			log.Printf("Redis connect failed: %v", err)
			return
		}
		defer db.Close()
	}

	payload, err := scraper.RunOnce(chromePath, db, nil)
	if err != nil {
		log.Printf("Scrape failed: %v", err)
		return
	}

	if !noRedis {
		log.Printf("Redis: written %d opportunities to key coinGlassSpotArb", payload.TotalScraped)
	}

	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(payload)
	} else {
		log.Printf("Total: %d opportunities", payload.TotalScraped)
		if len(payload.Data) > 0 {
			fmt.Printf("  %-8s %-10s %-12s %-10s %-12s %-14s\n",
				"Symbol", "Exchange", "FundingRate", "PnL", "APR", "3D CumRate")
			for _, o := range payload.Data {
				fmt.Printf("  %-8s %-10s %-12s %-10s %-12s %-14s\n",
					o.Symbol, o.Exchange, o.FundingRate, o.PnL, o.APR, o.CumRate3D)
			}
		}
	}

	log.Printf("Done.")
}

func readRedisPassword() string {
	data, err := os.ReadFile("config.json")
	if err != nil {
		return ""
	}
	var cfg struct {
		Redis struct {
			Password string `json:"password"`
		} `json:"redis"`
	}
	if json.Unmarshal(data, &cfg) == nil {
		return cfg.Redis.Password
	}
	return ""
}
