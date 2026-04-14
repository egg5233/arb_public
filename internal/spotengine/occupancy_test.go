package spotengine

import (
	"testing"
	"time"

	"arb/internal/models"
)

func saveAdmissionPerp(t *testing.T, e *SpotEngine, id, symbol, status string) {
	t.Helper()
	pos := &models.ArbitragePosition{
		ID:            id,
		Symbol:        symbol,
		LongExchange:  "binance",
		ShortExchange: "bybit",
		Status:        status,
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}
	if err := e.db.SavePosition(pos); err != nil {
		t.Fatalf("SavePosition: %v", err)
	}
}

func saveAdmissionSpot(t *testing.T, e *SpotEngine, id, symbol, status string) {
	t.Helper()
	pos := &models.SpotFuturesPosition{
		ID:        id,
		Symbol:    symbol,
		Exchange:  "binance",
		Status:    status,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := e.db.SaveSpotPosition(pos); err != nil {
		t.Fatalf("SaveSpotPosition: %v", err)
	}
}

func TestLoadUnifiedAdmission_PerpOnly(t *testing.T) {
	e, mr := newExecutionTestEngine(t)
	defer mr.Close()

	saveAdmissionPerp(t, e, "p1", "BTCUSDT", models.StatusActive)
	saveAdmissionPerp(t, e, "p2", "ETHUSDT", models.StatusClosed)

	activePerp, activeSpot, occupied, err := e.loadUnifiedAdmission()
	if err != nil {
		t.Fatalf("loadUnifiedAdmission: %v", err)
	}
	if activePerp != 1 || activeSpot != 0 {
		t.Fatalf("counts = (%d,%d), want (1,0)", activePerp, activeSpot)
	}
	if _, ok := occupied["BTCUSDT"]; !ok {
		t.Fatal("BTCUSDT missing from occupied set")
	}
	if _, ok := occupied["ETHUSDT"]; ok {
		t.Fatal("closed perp symbol should be excluded")
	}
}

func TestLoadUnifiedAdmission_SpotOnly(t *testing.T) {
	e, mr := newExecutionTestEngine(t)
	defer mr.Close()

	saveAdmissionSpot(t, e, "s1", "SOLUSDT", models.SpotStatusPending)
	saveAdmissionSpot(t, e, "s2", "XRPUSDT", models.SpotStatusClosed)

	activePerp, activeSpot, occupied, err := e.loadUnifiedAdmission()
	if err != nil {
		t.Fatalf("loadUnifiedAdmission: %v", err)
	}
	if activePerp != 0 || activeSpot != 1 {
		t.Fatalf("counts = (%d,%d), want (0,1)", activePerp, activeSpot)
	}
	if _, ok := occupied["SOLUSDT"]; !ok {
		t.Fatal("SOLUSDT missing from occupied set")
	}
	if _, ok := occupied["XRPUSDT"]; ok {
		t.Fatal("closed spot symbol should be excluded")
	}
}

func TestLoadUnifiedAdmission_CrossStrategyUnion(t *testing.T) {
	e, mr := newExecutionTestEngine(t)
	defer mr.Close()

	saveAdmissionPerp(t, e, "p1", "ADAUSDT", models.StatusPending)
	saveAdmissionSpot(t, e, "s1", "DOGEUSDT", models.SpotStatusActive)

	activePerp, activeSpot, occupied, err := e.loadUnifiedAdmission()
	if err != nil {
		t.Fatalf("loadUnifiedAdmission: %v", err)
	}
	if activePerp != 1 || activeSpot != 1 {
		t.Fatalf("counts = (%d,%d), want (1,1)", activePerp, activeSpot)
	}
	if len(occupied) != 2 {
		t.Fatalf("occupied size = %d, want 2", len(occupied))
	}
}
