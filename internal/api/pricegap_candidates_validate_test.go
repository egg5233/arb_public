package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"arb/internal/models"
)

// postPriceGap is a helper that POSTs a JSON body to /api/config and returns
// the recorder + decoded envelope.
func postPriceGap(t *testing.T, s *Server, body any) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	buf, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/config", bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.handlePostConfig(rr, req)
	var env map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &env)
	return rr, env
}

// candidateMap returns a map[string]any of a single valid PriceGapCandidate
// suitable for JSON encoding. Callers override fields as needed for negative
// tests.
func candidateMap(overrides map[string]any) map[string]any {
	c := map[string]any{
		"symbol":               "BTCUSDT",
		"long_exch":            "binance",
		"short_exch":           "bybit",
		"threshold_bps":        200,
		"max_position_usdt":    5000,
		"modeled_slippage_bps": 5,
	}
	for k, v := range overrides {
		c[k] = v
	}
	return c
}

func TestPostConfigPriceGapCandidates(t *testing.T) {
	t.Run("Add", func(t *testing.T) {
		s, _, _, td := newPriceGapTestServer(t)
		defer td()
		// Start with an empty candidate list so the Add test can assert exactly one entry.
		s.cfg.PriceGapCandidates = nil

		body := map[string]any{
			"price_gap": map[string]any{
				"candidates": []any{candidateMap(nil)},
			},
		}
		rr, env := postPriceGap(t, s, body)
		if rr.Code != http.StatusOK {
			t.Fatalf("status: got %d, want 200 (body=%s)", rr.Code, rr.Body.String())
		}
		if ok, _ := env["ok"].(bool); !ok {
			t.Fatalf("ok=false, env=%v", env)
		}
		if len(s.cfg.PriceGapCandidates) != 1 {
			t.Fatalf("candidates len: got %d, want 1", len(s.cfg.PriceGapCandidates))
		}
		c := s.cfg.PriceGapCandidates[0]
		if c.Symbol != "BTCUSDT" || c.LongExch != "binance" || c.ShortExch != "bybit" {
			t.Fatalf("tuple: got %+v", c)
		}
		if c.ThresholdBps != 200 || c.MaxPositionUSDT != 5000 || c.ModeledSlippageBps != 5 {
			t.Fatalf("numerics: got %+v", c)
		}
	})

	t.Run("EmptyArrayDeleteAll", func(t *testing.T) {
		s, _, _, td := newPriceGapTestServer(t)
		defer td()
		// newPriceGapTestServer seeds 2 candidates by default.
		if len(s.cfg.PriceGapCandidates) != 2 {
			t.Fatalf("setup: expected 2 seeded candidates, got %d", len(s.cfg.PriceGapCandidates))
		}

		body := map[string]any{
			"price_gap": map[string]any{
				"candidates": []any{}, // intentional delete-all
			},
		}
		rr, _ := postPriceGap(t, s, body)
		if rr.Code != http.StatusOK {
			t.Fatalf("status: got %d, want 200 (body=%s)", rr.Code, rr.Body.String())
		}
		if len(s.cfg.PriceGapCandidates) != 0 {
			t.Fatalf("candidates len: got %d, want 0 (Pitfall 2 — empty array must clear)", len(s.cfg.PriceGapCandidates))
		}
	})

	t.Run("NilUntouched", func(t *testing.T) {
		s, _, _, td := newPriceGapTestServer(t)
		defer td()
		if len(s.cfg.PriceGapCandidates) != 2 {
			t.Fatalf("setup: expected 2 seeded candidates, got %d", len(s.cfg.PriceGapCandidates))
		}

		body := map[string]any{
			"price_gap": map[string]any{
				"enabled": true, // no candidates field → not provided
			},
		}
		rr, _ := postPriceGap(t, s, body)
		if rr.Code != http.StatusOK {
			t.Fatalf("status: got %d, want 200 (body=%s)", rr.Code, rr.Body.String())
		}
		if len(s.cfg.PriceGapCandidates) != 2 {
			t.Fatalf("candidates len: got %d, want 2 (nil pointer must leave slice untouched)", len(s.cfg.PriceGapCandidates))
		}
	})

	t.Run("Validation_SymbolFormat", func(t *testing.T) {
		s, _, _, td := newPriceGapTestServer(t)
		defer td()

		body := map[string]any{
			"price_gap": map[string]any{
				"candidates": []any{candidateMap(map[string]any{"symbol": "btc/usdt"})},
			},
		}
		rr, env := postPriceGap(t, s, body)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status: got %d, want 400 (body=%s)", rr.Code, rr.Body.String())
		}
		errStr, _ := env["error"].(string)
		if !strings.Contains(errStr, "symbol must match") {
			t.Fatalf("error: got %q, want contains 'symbol must match'", errStr)
		}
	})

	t.Run("Validation_ExchEqual", func(t *testing.T) {
		s, _, _, td := newPriceGapTestServer(t)
		defer td()

		body := map[string]any{
			"price_gap": map[string]any{
				"candidates": []any{candidateMap(map[string]any{
					"long_exch":  "binance",
					"short_exch": "binance",
				})},
			},
		}
		rr, env := postPriceGap(t, s, body)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status: got %d, want 400 (body=%s)", rr.Code, rr.Body.String())
		}
		errStr, _ := env["error"].(string)
		if !strings.Contains(errStr, "long_exch must differ from short_exch") {
			t.Fatalf("error: got %q, want contains 'long_exch must differ from short_exch'", errStr)
		}
	})

	t.Run("Validation_ExchEnum", func(t *testing.T) {
		s, _, _, td := newPriceGapTestServer(t)
		defer td()

		body := map[string]any{
			"price_gap": map[string]any{
				"candidates": []any{candidateMap(map[string]any{"long_exch": "kraken"})},
			},
		}
		rr, env := postPriceGap(t, s, body)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status: got %d, want 400 (body=%s)", rr.Code, rr.Body.String())
		}
		errStr, _ := env["error"].(string)
		if !strings.Contains(errStr, "long_exch invalid") {
			t.Fatalf("error: got %q, want contains 'long_exch invalid'", errStr)
		}
	})

	t.Run("Validation_ThresholdRange", func(t *testing.T) {
		s, _, _, td := newPriceGapTestServer(t)
		defer td()

		body := map[string]any{
			"price_gap": map[string]any{
				"candidates": []any{candidateMap(map[string]any{"threshold_bps": 0})},
			},
		}
		rr, env := postPriceGap(t, s, body)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status: got %d, want 400 (body=%s)", rr.Code, rr.Body.String())
		}
		errStr, _ := env["error"].(string)
		if !strings.Contains(errStr, "threshold_bps out of range") {
			t.Fatalf("error: got %q, want contains 'threshold_bps out of range'", errStr)
		}
	})

	t.Run("Validation_MaxPositionRange", func(t *testing.T) {
		s, _, _, td := newPriceGapTestServer(t)
		defer td()

		body := map[string]any{
			"price_gap": map[string]any{
				"candidates": []any{candidateMap(map[string]any{"max_position_usdt": 0})},
			},
		}
		rr, env := postPriceGap(t, s, body)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status: got %d, want 400 (body=%s)", rr.Code, rr.Body.String())
		}
		errStr, _ := env["error"].(string)
		if !strings.Contains(errStr, "max_position_usdt out of range") {
			t.Fatalf("error: got %q, want contains 'max_position_usdt out of range'", errStr)
		}
	})

	t.Run("Validation_SlippageRange", func(t *testing.T) {
		s, _, _, td := newPriceGapTestServer(t)
		defer td()

		body := map[string]any{
			"price_gap": map[string]any{
				"candidates": []any{candidateMap(map[string]any{"modeled_slippage_bps": 1001})},
			},
		}
		rr, env := postPriceGap(t, s, body)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status: got %d, want 400 (body=%s)", rr.Code, rr.Body.String())
		}
		errStr, _ := env["error"].(string)
		if !strings.Contains(errStr, "modeled_slippage_bps out of range") {
			t.Fatalf("error: got %q, want contains 'modeled_slippage_bps out of range'", errStr)
		}
	})

	t.Run("Validation_DuplicateTuple", func(t *testing.T) {
		s, _, _, td := newPriceGapTestServer(t)
		defer td()

		body := map[string]any{
			"price_gap": map[string]any{
				"candidates": []any{
					candidateMap(nil),
					candidateMap(nil), // identical tuple → duplicate
				},
			},
		}
		rr, env := postPriceGap(t, s, body)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status: got %d, want 400 (body=%s)", rr.Code, rr.Body.String())
		}
		errStr, _ := env["error"].(string)
		if !strings.Contains(errStr, "duplicate tuple") {
			t.Fatalf("error: got %q, want contains 'duplicate tuple'", errStr)
		}
	})

	t.Run("BlocksActiveDelete", func(t *testing.T) {
		s, _, _, td := newPriceGapTestServer(t)
		defer td()

		// Configure exactly one candidate (BTCUSDT, binance, bybit) so we can attempt to
		// delete it via empty array.
		s.cfg.PriceGapCandidates = []models.PriceGapCandidate{
			{Symbol: "BTCUSDT", LongExch: "binance", ShortExch: "bybit", ThresholdBps: 200, MaxPositionUSDT: 5000, ModeledSlippageBps: 5},
		}

		// Seed a matching active position via the real DB.
		pos := &models.PriceGapPosition{
			ID:            "BTCUSDT_binance_bybit",
			Symbol:        "BTCUSDT",
			LongExchange:  "binance",
			ShortExchange: "bybit",
			Status:        "open",
			Mode:          "paper",
		}
		if err := s.db.SavePriceGapPosition(pos); err != nil {
			t.Fatalf("seed SavePriceGapPosition: %v", err)
		}

		body := map[string]any{
			"price_gap": map[string]any{
				"candidates": []any{}, // attempt full delete while position is open
			},
		}
		rr, env := postPriceGap(t, s, body)
		if rr.Code != http.StatusConflict {
			t.Fatalf("status: got %d, want 409 (body=%s)", rr.Code, rr.Body.String())
		}
		errStr, _ := env["error"].(string)
		if !strings.Contains(errStr, "active position; close it first") {
			t.Fatalf("error: got %q, want contains 'active position; close it first'", errStr)
		}

		// Confirm candidate was NOT mutated.
		if len(s.cfg.PriceGapCandidates) != 1 {
			t.Fatalf("candidates len: got %d, want 1 (must NOT mutate on 409)", len(s.cfg.PriceGapCandidates))
		}
	})

	t.Run("TupleChangeOrphanGuard", func(t *testing.T) {
		s, _, _, td := newPriceGapTestServer(t)
		defer td()

		// Seed: one candidate at (BTCUSDT, binance, bybit) with active position on that exact tuple.
		s.cfg.PriceGapCandidates = []models.PriceGapCandidate{
			{Symbol: "BTCUSDT", LongExch: "binance", ShortExch: "bybit", ThresholdBps: 200, MaxPositionUSDT: 5000, ModeledSlippageBps: 5},
		}
		pos := &models.PriceGapPosition{
			ID:            "BTCUSDT_binance_bybit",
			Symbol:        "BTCUSDT",
			LongExchange:  "binance",
			ShortExchange: "bybit",
			Status:        "open",
			Mode:          "paper",
		}
		if err := s.db.SavePriceGapPosition(pos); err != nil {
			t.Fatalf("seed SavePriceGapPosition: %v", err)
		}

		// Replacement payload changes long_exch from binance → gateio (Pitfall 4).
		// Old tuple is removed implicitly; new tuple has no position. Should fail with 409.
		body := map[string]any{
			"price_gap": map[string]any{
				"candidates": []any{candidateMap(map[string]any{"long_exch": "gateio"})},
			},
		}
		rr, env := postPriceGap(t, s, body)
		if rr.Code != http.StatusConflict {
			t.Fatalf("status: got %d, want 409 (body=%s)", rr.Code, rr.Body.String())
		}
		errStr, _ := env["error"].(string)
		if !strings.Contains(errStr, "active position; close it first") {
			t.Fatalf("error: got %q, want contains 'active position; close it first'", errStr)
		}

		// Confirm tuple was NOT changed.
		if len(s.cfg.PriceGapCandidates) != 1 {
			t.Fatalf("candidates len: got %d, want 1 (must NOT mutate on 409)", len(s.cfg.PriceGapCandidates))
		}
		if got := s.cfg.PriceGapCandidates[0].LongExch; got != "binance" {
			t.Fatalf("long_exch: got %q, want 'binance' (must NOT mutate on 409)", got)
		}
	})
}
