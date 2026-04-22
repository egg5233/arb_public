// Package pricegap holds black-box integration tests for the price-gap
// tracker's cross-cutting invariants. Kept as its own package so the tests
// exercise only exported behaviour of internal/pricegaptrader + internal/models
// + internal/database. See D-23 / D-24 in 09-CONTEXT.md for rationale.
package pricegap

import (
	"encoding/json"
	"reflect"
	"testing"

	"arb/internal/models"
)

// TestSlippageWritePath_FieldsExist — PriceGapPosition must carry both
// ModeledSlipBps (float64) and RealizedSlipBps (float64) with persisted
// JSON tags so every pg:history row captures the per-trade measurements
// Plan 05 metrics depend on (D-23 / D-24).
func TestSlippageWritePath_FieldsExist(t *testing.T) {
	typ := reflect.TypeOf(models.PriceGapPosition{})

	mf, ok := typ.FieldByName("ModeledSlipBps")
	if !ok {
		t.Fatal("ModeledSlipBps field missing on PriceGapPosition")
	}
	if mf.Type.Kind() != reflect.Float64 {
		t.Fatalf("ModeledSlipBps kind: got %v, want float64", mf.Type.Kind())
	}
	if tag := mf.Tag.Get("json"); tag == "" {
		t.Fatal("ModeledSlipBps missing json tag")
	}

	rf, ok := typ.FieldByName("RealizedSlipBps")
	if !ok {
		t.Fatal("RealizedSlipBps field missing on PriceGapPosition")
	}
	if rf.Type.Kind() != reflect.Float64 {
		t.Fatalf("RealizedSlipBps kind: got %v, want float64", rf.Type.Kind())
	}
	if tag := rf.Tag.Get("json"); tag == "" {
		t.Fatal("RealizedSlipBps missing json tag")
	}
}

// TestSlippageWritePath_JSONPersistsFields — both fields must survive a full
// marshal/unmarshal cycle so Plan 05 metrics can read them from pg:history.
func TestSlippageWritePath_JSONPersistsFields(t *testing.T) {
	want := &models.PriceGapPosition{
		ID:              "pg_WP_binance_bybit_1",
		Symbol:          "SOONUSDT",
		Status:          models.PriceGapStatusClosed,
		ModeledSlipBps:  8.0,
		RealizedSlipBps: 12.5,
	}
	data, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got models.PriceGapPosition
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.ModeledSlipBps != want.ModeledSlipBps {
		t.Fatalf("ModeledSlipBps round-trip: got %v, want %v", got.ModeledSlipBps, want.ModeledSlipBps)
	}
	if got.RealizedSlipBps != want.RealizedSlipBps {
		t.Fatalf("RealizedSlipBps round-trip: got %v, want %v", got.RealizedSlipBps, want.RealizedSlipBps)
	}
}
