package discovery

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var lorisHistoricalURL = "https://loris.tools/api/funding/historical"

// ErrLorisRateLimited is returned by FetchLorisHistoricalSeries when the API returns 429.
var ErrLorisRateLimited = errors.New("Loris API rate limited (429)")

// LorisHistoricalResponse is the parsed response from the Loris historical funding API.
type LorisHistoricalResponse struct {
	Symbol  string                      `json:"symbol"`
	Series  map[string][]LorisDataPoint `json:"series"`
	Notices []string                    `json:"notices"`
}

// LorisDataPoint is a single funding settlement from the Loris historical API.
type LorisDataPoint struct {
	T string  `json:"t"` // ISO 8601 timestamp
	Y float64 `json:"y"` // settlement rate in basis points
}

// FetchLorisHistoricalSeries fetches historical funding data for a base coin (e.g. "BTC")
// from the given exchanges over [start, end]. Returns ErrLorisRateLimited on 429.
func FetchLorisHistoricalSeries(client *http.Client, base string, exchanges []string, start, end time.Time) (*LorisHistoricalResponse, error) {
	params := url.Values{}
	params.Set("symbol", base)
	params.Set("start", start.Format(time.RFC3339))
	params.Set("end", end.Format(time.RFC3339))
	params.Set("exchanges", strings.Join(exchanges, ","))

	reqURL := lorisHistoricalURL + "?" + params.Encode()
	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "arb-bot/1.0")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 429 {
		return nil, ErrLorisRateLimited
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned %d: %s", resp.StatusCode, string(body))
	}

	var historical LorisHistoricalResponse
	if err := json.NewDecoder(resp.Body).Decode(&historical); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &historical, nil
}

// FilterValidSettlements filters data points to valid settlement times and
// deduplicates by rounded hour.
func FilterValidSettlements(series []LorisDataPoint, intervalHours float64) []LorisDataPoint {
	seen := make(map[string]bool)
	var out []LorisDataPoint

	for _, p := range series {
		t, err := time.Parse(time.RFC3339, p.T)
		if err != nil {
			continue
		}
		rounded := RoundToNearestHour(t)
		if !IsValidSettlementHour(rounded, intervalHours) {
			continue
		}
		key := rounded.Format(time.RFC3339)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, p)
	}
	return out
}

// RoundToNearestHour rounds a timestamp to the nearest hour.
// e.g. 07:59 → 08:00, 08:01 → 08:00, 08:31 → 09:00
func RoundToNearestHour(t time.Time) time.Time {
	if t.Minute() >= 30 {
		return t.Truncate(time.Hour).Add(time.Hour)
	}
	return t.Truncate(time.Hour)
}

// IsValidSettlementHour checks if a rounded-to-hour timestamp is a valid
// funding settlement time for the given interval.
func IsValidSettlementHour(t time.Time, intervalHours float64) bool {
	h := t.Hour()
	switch {
	case intervalHours <= 1:
		return true // every hour
	case intervalHours <= 4:
		return h%4 == 0 // 0,4,8,12,16,20
	case intervalHours <= 8:
		return h%8 == 0 // 0,8,16
	default:
		return true // unknown interval, accept on-the-hour
	}
}
