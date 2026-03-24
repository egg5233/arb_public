package discovery

import (
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const (
	binanceDelistURL    = "https://www.binance.com/bapi/composite/v1/public/cms/article/list/query?type=1&catalogId=161&pageNo=1&pageSize=20"
	delistPollInterval  = 6 * time.Hour
	delistRedisPrefix   = "arb:delist:"
	delistBufferDays    = 7 // keep blacklist for 7 days after delist date
)

// Regex patterns for parsing delist announcements.
var (
	// "Binance Will Delist A2Z, FORTH, HOOK, IDEX, LRC, NTRN, RDNT, SXP on 2026-04-01"
	reSpotDelist = regexp.MustCompile(`(?i)(?:Will Delist|Delists?)\s+(.+?)\s+on\s+(\d{4}-\d{2}-\d{2})`)
	// "APTUSD" or "OPUSD" in futures titles
	reFuturesCoin = regexp.MustCompile(`\b([A-Z]{2,10})USD[T]?\b`)
	// Date in parentheses: "(2026-03-25)"
	reParenDate = regexp.MustCompile(`\((\d{4}-\d{2}-\d{2})`)
	// Coins to ignore in regex matches
	ignoreCoins = map[string]bool{"COIN": true, "MULTIPLE": true, "AND": true, "WILL": true, "THE": true, "ON": true, "VIP": true}
)

type binanceArticle struct {
	Title       string `json:"title"`
	ReleaseDate int64  `json:"releaseDate"`
}

type binanceDelistResponse struct {
	Data struct {
		Catalogs []struct {
			Articles []binanceArticle `json:"articles"`
		} `json:"catalogs"`
	} `json:"data"`
}

// delistEntry represents a parsed delist announcement.
type delistEntry struct {
	Symbol    string // e.g. "NTRN"
	DelistDate time.Time
}

// StartDelistMonitor launches a background goroutine that polls Binance
// delist announcements and maintains a Redis blacklist.
func (s *Scanner) StartDelistMonitor() {
	go func() {
		// Immediate poll on startup.
		s.pollDelistAnnouncements()

		ticker := time.NewTicker(delistPollInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.pollDelistAnnouncements()
			case <-s.stopCh:
				s.log.Info("delist monitor stopped")
				return
			}
		}
	}()
}

// IsDelisted checks if a symbol is on the Binance delist blacklist.
func (s *Scanner) IsDelisted(symbol string) bool {
	key := delistRedisPrefix + symbol
	val, err := s.db.Get(key)
	return err == nil && val != ""
}

// pollDelistAnnouncements fetches Binance delist page and updates Redis blacklist.
func (s *Scanner) pollDelistAnnouncements() {
	req, err := http.NewRequest(http.MethodGet, binanceDelistURL, nil)
	if err != nil {
		s.log.Warn("delist monitor: failed to create request: %v", err)
		return
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Encoding", "identity") // avoid gzip issues

	resp, err := s.client.Do(req)
	if err != nil {
		s.log.Warn("delist monitor: API request failed: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		s.log.Warn("delist monitor: API returned %d: %s", resp.StatusCode, string(body))
		return
	}

	var delistResp binanceDelistResponse
	if err := json.NewDecoder(resp.Body).Decode(&delistResp); err != nil {
		s.log.Warn("delist monitor: failed to decode response: %v", err)
		return
	}

	var articles []binanceArticle
	for _, cat := range delistResp.Data.Catalogs {
		articles = append(articles, cat.Articles...)
	}

	entries := parseDelistArticles(articles)
	if len(entries) == 0 {
		s.log.Info("delist monitor: no delist coins found in %d articles", len(articles))
		return
	}

	// Write to Redis.
	now := time.Now().UTC()
	newCount := 0
	for _, entry := range entries {
		symbol := entry.Symbol + "USDT"
		key := delistRedisPrefix + symbol

		// Check if already blacklisted.
		if existing, err := s.db.Get(key); err == nil && existing != "" {
			continue
		}

		// TTL: until delist date + buffer, minimum 1 hour.
		ttl := time.Until(entry.DelistDate) + time.Duration(delistBufferDays)*24*time.Hour
		if ttl < time.Hour {
			ttl = time.Hour
		}

		dateStr := entry.DelistDate.Format("2006-01-02")
		if err := s.db.SetWithTTL(key, dateStr, ttl); err != nil {
			s.log.Error("delist monitor: failed to write Redis key %s: %v", key, err)
			continue
		}
		s.log.Warn("delist monitor: blacklisted %s (delist date: %s, TTL: %s)", symbol, dateStr, ttl.Round(time.Hour))
		newCount++
	}

	s.log.Info("delist monitor: processed %d articles, %d coins total, %d newly blacklisted (as of %s)",
		len(articles), len(entries), newCount, now.Format("2006-01-02 15:04"))
}

// parseDelistArticles extracts coin symbols and delist dates from article titles.
func parseDelistArticles(articles []binanceArticle) []delistEntry {
	seen := make(map[string]bool)
	var entries []delistEntry

	for _, art := range articles {
		title := art.Title

		// Pattern 1: Spot delist — "Binance Will Delist A2Z, FORTH, ... on 2026-04-01"
		if m := reSpotDelist.FindStringSubmatch(title); len(m) >= 3 {
			coinsStr := m[1]
			dateStr := m[2]
			delistDate, err := time.Parse("2006-01-02", dateStr)
			if err != nil {
				continue
			}

			coins := splitCoins(coinsStr)
			for _, coin := range coins {
				if !seen[coin] {
					seen[coin] = true
					entries = append(entries, delistEntry{Symbol: coin, DelistDate: delistDate})
				}
			}
		}

		// Pattern 2: Futures delist — "APTUSD and OPUSD Perpetual Contracts (2026-03-25)"
		if strings.Contains(title, "Perpetual") {
			matches := reFuturesCoin.FindAllStringSubmatch(title, -1)
			dateMatch := reParenDate.FindStringSubmatch(title)
			if len(matches) > 0 && len(dateMatch) >= 2 {
				delistDate, err := time.Parse("2006-01-02", dateMatch[1])
				if err != nil {
					continue
				}
				for _, m := range matches {
					base := m[1]
					if ignoreCoins[base] {
						continue
					}
					if !seen[base] {
						seen[base] = true
						entries = append(entries, delistEntry{Symbol: base, DelistDate: delistDate})
					}
				}
			}
		}
	}

	return entries
}

// splitCoins splits a comma/space-separated string of coin symbols.
func splitCoins(s string) []string {
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == ' '
	})
	var coins []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if len(p) >= 2 && len(p) <= 10 && isUpperAlphaNum(p) && !ignoreCoins[p] {
			coins = append(coins, p)
		}
	}
	return coins
}

func isUpperAlphaNum(s string) bool {
	for _, r := range s {
		if !((r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
			return false
		}
	}
	return true
}

// SetSymbolCooldown is a convenience alias used by the scanner for delist-related
// cooldowns. Re-exports the database method for consistency.
func (s *Scanner) SetDelistCooldown(symbol string, ttl time.Duration) {
	key := delistRedisPrefix + symbol
	s.db.SetWithTTL(key, "manual", ttl)
}

// GetDelistDate returns the delist date string for a symbol, or empty if not delisted.
func (s *Scanner) GetDelistDate(symbol string) string {
	key := delistRedisPrefix + symbol
	val, err := s.db.Get(key)
	if err != nil {
		return ""
	}
	return val
}
