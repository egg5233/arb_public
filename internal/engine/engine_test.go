package engine

import (
	"fmt"
	"testing"

	"arb/internal/config"
	"arb/internal/models"
)

func TestEffectiveAdvanceMin(t *testing.T) {
	tests := []struct {
		name          string
		cfgAdvance    int
		intervalHours float64
		want          int
	}{
		{"standard 8h uses config", 16, 8.0, 16},
		{"zero interval uses config", 16, 0, 16},
		{"negative interval uses config", 16, -1, 16},
		{"4h interval scales to 48min", 16, 4.0, 16},        // 4*60*0.20=48, capped at cfgAdvance=16
		{"1h interval scales to 12min", 16, 1.0, 12},        // 1*60*0.20=12
		{"2h interval scales to 24min capped", 16, 2.0, 16}, // 2*60*0.20=24, capped at 16
		{"0.25h interval floors to 3min", 16, 0.25, 3},      // 0.25*60*0.20=3
		{"very short interval floors to 3min", 16, 0.1, 3},  // 0.1*60*0.20=1.2 → 1, floored to 3
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := effectiveAdvanceMin(tt.cfgAdvance, tt.intervalHours)
			if got != tt.want {
				t.Errorf("effectiveAdvanceMin(%d, %v) = %d, want %d",
					tt.cfgAdvance, tt.intervalHours, got, tt.want)
			}
		})
	}
}

func TestClassifyRotation(t *testing.T) {
	tests := []struct {
		name       string
		pos        *models.ArbitragePosition
		opp        *models.Opportunity
		wantShared string
		wantOld    string
		wantNew    string
		wantSide   string
	}{
		{
			name:       "same long, different short → rotate short",
			pos:        &models.ArbitragePosition{LongExchange: "bitget", ShortExchange: "binance"},
			opp:        &models.Opportunity{LongExchange: "bitget", ShortExchange: "gateio"},
			wantShared: "long", wantOld: "binance", wantNew: "gateio", wantSide: "short",
		},
		{
			name:       "different long, same short → rotate long",
			pos:        &models.ArbitragePosition{LongExchange: "binance", ShortExchange: "gateio"},
			opp:        &models.Opportunity{LongExchange: "bitget", ShortExchange: "gateio"},
			wantShared: "short", wantOld: "binance", wantNew: "bitget", wantSide: "long",
		},
		{
			name:       "both same → no rotation",
			pos:        &models.ArbitragePosition{LongExchange: "bitget", ShortExchange: "gateio"},
			opp:        &models.Opportunity{LongExchange: "bitget", ShortExchange: "gateio"},
			wantShared: "", wantOld: "", wantNew: "", wantSide: "",
		},
		{
			name:       "both different → no rotation",
			pos:        &models.ArbitragePosition{LongExchange: "binance", ShortExchange: "bybit"},
			opp:        &models.Opportunity{LongExchange: "bitget", ShortExchange: "gateio"},
			wantShared: "", wantOld: "", wantNew: "", wantSide: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shared, old, new_, side := classifyRotation(tt.pos, tt.opp)
			if shared != tt.wantShared || old != tt.wantOld || new_ != tt.wantNew || side != tt.wantSide {
				t.Errorf("classifyRotation() = (%q,%q,%q,%q), want (%q,%q,%q,%q)",
					shared, old, new_, side, tt.wantShared, tt.wantOld, tt.wantNew, tt.wantSide)
			}
		})
	}
}

func TestAPIErrorCounter(t *testing.T) {
	e := &Engine{
		apiErrCounts: make(map[string]int),
		cfg:          &config.Config{EnablePerpTelegram: true},
	}
	// Record 2 errors -- should not trigger (threshold is 3).
	e.recordAPIError("binance", fmt.Errorf("timeout"))
	e.recordAPIError("binance", fmt.Errorf("timeout"))
	if e.apiErrCounts["binance"] != 2 {
		t.Errorf("expected 2 errors, got %d", e.apiErrCounts["binance"])
	}
	// Third error hits threshold (notification fires but TelegramNotifier is nil -- nil-safe).
	e.recordAPIError("binance", fmt.Errorf("timeout"))
	if e.apiErrCounts["binance"] != 3 {
		t.Errorf("expected 3 errors, got %d", e.apiErrCounts["binance"])
	}
	// Fourth error should still increment but not re-trigger (only at exactly 3).
	e.recordAPIError("binance", fmt.Errorf("timeout"))
	if e.apiErrCounts["binance"] != 4 {
		t.Errorf("expected 4 errors, got %d", e.apiErrCounts["binance"])
	}
}

func TestAPIErrorCounterReset(t *testing.T) {
	e := &Engine{
		apiErrCounts: make(map[string]int),
		cfg:          &config.Config{EnablePerpTelegram: true},
	}
	e.recordAPIError("binance", fmt.Errorf("timeout"))
	e.recordAPIError("binance", fmt.Errorf("timeout"))
	e.recordAPISuccess("binance")
	if e.apiErrCounts["binance"] != 0 {
		t.Errorf("expected 0 after reset, got %d", e.apiErrCounts["binance"])
	}
}

func TestAPIErrorCounterDisabled(t *testing.T) {
	e := &Engine{
		apiErrCounts: make(map[string]int),
		cfg:          &config.Config{EnablePerpTelegram: false},
	}
	// When EnablePerpTelegram is false, errors should not be tracked.
	e.recordAPIError("binance", fmt.Errorf("timeout"))
	if e.apiErrCounts["binance"] != 0 {
		t.Errorf("expected 0 when disabled, got %d", e.apiErrCounts["binance"])
	}
}

