package spotengine

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"arb/internal/config"
	"arb/internal/models"
	"arb/internal/scraper"
	"arb/pkg/exchange"
)

func TestRunDiscoveryScan_KeepsActivePositionWithNonPositiveFunding(t *testing.T) {
	tests := []struct {
		name string
		apr  string
		want float64
	}{
		{name: "zero funding", apr: "0.00%", want: 0},
		{name: "negative funding", apr: "-5.00%", want: -0.05},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			engine, mr := newExecutionTestEngine(t)
			defer mr.Close()

			engine.cfg = &config.Config{}
			engine.spotMargin = map[string]exchange.SpotMarginExchange{
				"binance": &marginStubExchange{},
			}

			pos := &models.SpotFuturesPosition{
				ID:        "pos-1",
				Symbol:    "BTCUSDT",
				BaseCoin:  "BTC",
				Exchange:  "binance",
				Direction: "buy_spot_short",
				Status:    models.SpotStatusActive,
				CreatedAt: time.Now().UTC(),
				UpdatedAt: time.Now().UTC(),
			}
			if err := engine.db.SaveSpotPosition(pos); err != nil {
				t.Fatalf("SaveSpotPosition: %v", err)
			}

			payload := scraper.Payload{
				Timestamp: time.Now().UTC().Format(time.RFC3339),
				Data: []scraper.Opportunity{
					{
						Symbol:    "BTC",
						Portfolio: "Buy BTC",
						Exchange:  "Binance",
						APR:       tc.apr,
					},
				},
			}
			raw, err := json.Marshal(payload)
			if err != nil {
				t.Fatalf("json.Marshal: %v", err)
			}
			if err := engine.db.SetWithTTL("coinGlassSpotArb", string(raw), time.Minute); err != nil {
				t.Fatalf("SetWithTTL: %v", err)
			}

			opps := engine.runDiscoveryScan()
			if len(opps) != 1 {
				t.Fatalf("runDiscoveryScan returned %d opportunities, want 1", len(opps))
			}

			opp := opps[0]
			if opp.FundingAPR != tc.want {
				t.Fatalf("FundingAPR = %.4f, want %.4f", opp.FundingAPR, tc.want)
			}
			if !strings.Contains(opp.FilterStatus, "funding") {
				t.Fatalf("FilterStatus = %q, want funding filter", opp.FilterStatus)
			}

			engine.oppsMu.Lock()
			engine.latestOpps = opps
			engine.oppsMu.Unlock()
			if _, found := engine.lookupCurrentOpp("BTCUSDT", "binance", "buy_spot_short"); !found {
				t.Fatal("lookupCurrentOpp did not retain active position with non-positive funding")
			}
		})
	}
}

func TestCoinGlassDiscoveryCachesMissingSpotMarketAcrossRestart(t *testing.T) {
	engine, mr := newExecutionTestEngine(t)
	defer mr.Close()

	engine.cfg = &config.Config{SpotFuturesNativeScannerEnabled: false}
	stub := &nativeScannerStubExchange{
		bboErr: fmt.Errorf("GetSpotBBO: no OKX spot market for ONTUSDT"),
	}
	engine.spotMargin = map[string]exchange.SpotMarginExchange{
		"okx": stub,
	}

	payload := scraper.Payload{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Data: []scraper.Opportunity{
			{
				Symbol:    "ONT",
				Portfolio: "Buy ONT",
				Exchange:  "OKX",
				APR:       "12.50%",
			},
		},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if err := engine.db.SetWithTTL("coinGlassSpotArb", string(raw), time.Minute); err != nil {
		t.Fatalf("SetWithTTL: %v", err)
	}

	opps := engine.runDiscoveryScan()
	if len(opps) != 1 {
		t.Fatalf("expected 1 opportunity, got %d", len(opps))
	}
	if opps[0].FilterStatus != "spot market unavailable" {
		t.Fatalf("expected spot market unavailable, got %q", opps[0].FilterStatus)
	}
	if stub.bboCalls != 1 {
		t.Fatalf("expected one GetSpotBBO probe for CoinGlass scan, got %d", stub.bboCalls)
	}

	exists, found, err := engine.db.GetSpotMarketAvailability("okx", "ONTUSDT")
	if err != nil {
		t.Fatalf("GetSpotMarketAvailability: %v", err)
	}
	if !found {
		t.Fatal("expected cached spot market availability entry")
	}
	if exists {
		t.Fatal("expected cached spot market availability to be false")
	}

	engine2 := &SpotEngine{
		cfg:       &config.Config{SpotFuturesNativeScannerEnabled: false},
		db:        engine.db,
		log:       engine.log,
		stopCh:    make(chan struct{}),
		exitState: exitState{exiting: make(map[string]bool)},
		spotMargin: map[string]exchange.SpotMarginExchange{
			"okx": &nativeScannerStubExchange{},
		},
	}

	opps = engine2.runDiscoveryScan()
	if len(opps) != 1 {
		t.Fatalf("expected 1 opportunity after restart, got %d", len(opps))
	}
	if opps[0].FilterStatus != "spot market unavailable" {
		t.Fatalf("expected cached spot market unavailable after restart, got %q", opps[0].FilterStatus)
	}
	if secondStub := engine2.spotMargin["okx"].(*nativeScannerStubExchange); secondStub.bboCalls != 0 {
		t.Fatalf("expected no GetSpotBBO call after restart cache hit, got %d", secondStub.bboCalls)
	}
}
