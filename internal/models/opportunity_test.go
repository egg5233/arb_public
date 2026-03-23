package models

import (
	"encoding/json"
	"testing"
	"time"
)

func TestOpportunityIntervalHoursJSON(t *testing.T) {
	opp := Opportunity{
		Symbol:        "BTCUSDT",
		LongExchange:  "binance",
		ShortExchange: "bybit",
		LongRate:      1.5,
		ShortRate:     3.0,
		Spread:        1.5,
		CostRatio:     0.3,
		OIRank:        10,
		Score:         1.65,
		IntervalHours: 4.0,
		Source:        "loris",
		Timestamp:     time.Date(2026, 3, 14, 0, 0, 0, 0, time.UTC),
	}

	data, err := json.Marshal(opp)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded Opportunity
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.IntervalHours != 4.0 {
		t.Errorf("IntervalHours = %v, want 4.0", decoded.IntervalHours)
	}
	if decoded.Source != "loris" {
		t.Errorf("Source = %q, want %q", decoded.Source, "loris")
	}

	// Verify JSON key name
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal raw: %v", err)
	}
	if _, ok := raw["interval_hours"]; !ok {
		t.Error("JSON output missing 'interval_hours' key")
	}
}

func TestOpportunityIntervalHoursValues(t *testing.T) {
	tests := []struct {
		name     string
		interval float64
	}{
		{"1h interval", 1.0},
		{"4h interval", 4.0},
		{"8h interval", 8.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opp := Opportunity{IntervalHours: tt.interval}
			data, err := json.Marshal(opp)
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}
			var decoded Opportunity
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}
			if decoded.IntervalHours != tt.interval {
				t.Errorf("IntervalHours = %v, want %v", decoded.IntervalHours, tt.interval)
			}
		})
	}
}
