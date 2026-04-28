// Package main — main_registry_test.go: Plan 11-03 Task 2 tests for the new
// `pg-admin candidates {list|add|delete|replace}` subcommands. The
// subcommands route through *pricegaptrader.Registry's typed mutators with
// source="pg-admin" — drift defense for T-11-19 (handlers + pg-admin share
// validation) and T-11-20 (pg-admin × dashboard concurrent writes).
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"arb/internal/models"
	"arb/internal/pricegaptrader"
)

// fakeRegistry is a CandidateRegistry double for pg-admin tests. Captures
// every mutator call so tests can assert source string + payload.
type fakeRegistry struct {
	mu        sync.Mutex
	addCalls  []addCall
	delCalls  []delCall
	repCalls  []repCall
	updCalls  []updCall
	addErr    error
	delErr    error
	repErr    error
	listValue []models.PriceGapCandidate
}

type addCall struct {
	Source string
	C      models.PriceGapCandidate
}
type delCall struct {
	Source string
	Idx    int
}
type repCall struct {
	Source string
	Next   []models.PriceGapCandidate
}
type updCall struct {
	Source string
	Idx    int
	C      models.PriceGapCandidate
}

func (f *fakeRegistry) Add(ctx context.Context, source string, c models.PriceGapCandidate) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.addCalls = append(f.addCalls, addCall{Source: source, C: c})
	return f.addErr
}
func (f *fakeRegistry) Update(ctx context.Context, source string, idx int, c models.PriceGapCandidate) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.updCalls = append(f.updCalls, updCall{Source: source, Idx: idx, C: c})
	return nil
}
func (f *fakeRegistry) Delete(ctx context.Context, source string, idx int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.delCalls = append(f.delCalls, delCall{Source: source, Idx: idx})
	return f.delErr
}
func (f *fakeRegistry) Replace(ctx context.Context, source string, next []models.PriceGapCandidate) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := make([]models.PriceGapCandidate, len(next))
	copy(cp, next)
	f.repCalls = append(f.repCalls, repCall{Source: source, Next: cp})
	return f.repErr
}
func (f *fakeRegistry) Get(idx int) (models.PriceGapCandidate, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if idx < 0 || idx >= len(f.listValue) {
		return models.PriceGapCandidate{}, false
	}
	return f.listValue[idx], true
}
func (f *fakeRegistry) List() []models.PriceGapCandidate {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]models.PriceGapCandidate, len(f.listValue))
	copy(out, f.listValue)
	return out
}

// runWithFake executes the pg-admin Run entrypoint with a fake registry +
// captured stdout/stderr. Returns exit code, stdout, stderr.
func runWithFake(t *testing.T, args []string, fr *fakeRegistry) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	deps := Dependencies{
		Registry: fr,
		Stdout:   &stdout,
		Stderr:   &stderr,
	}
	code := Run(args, deps)
	return code, stdout.String(), stderr.String()
}

// TestPgAdmin_CandidatesAdd_Success verifies the happy path: subcommand parses
// flags, validates the candidate, and routes Registry.Add with source="pg-admin".
func TestPgAdmin_CandidatesAdd_Success(t *testing.T) {
	fr := &fakeRegistry{}
	args := []string{
		"candidates", "add",
		"--symbol", "BTCUSDT",
		"--long", "binance",
		"--short", "bybit",
		"--threshold-bps", "200",
		"--max-position-usdt", "5000",
	}
	code, _, _ := runWithFake(t, args, fr)
	if code != 0 {
		t.Fatalf("exit code: got %d, want 0", code)
	}
	if len(fr.addCalls) != 1 {
		t.Fatalf("Add call count: got %d, want 1", len(fr.addCalls))
	}
	got := fr.addCalls[0]
	if got.Source != "pg-admin" {
		t.Errorf("source: got %q, want %q", got.Source, "pg-admin")
	}
	if got.C.Symbol != "BTCUSDT" || got.C.LongExch != "binance" || got.C.ShortExch != "bybit" {
		t.Errorf("candidate: got %+v", got.C)
	}
}

// TestPgAdmin_CandidatesAdd_Duplicate verifies the duplicate-error sentinel
// returns exit code 3.
func TestPgAdmin_CandidatesAdd_Duplicate(t *testing.T) {
	fr := &fakeRegistry{addErr: pricegaptrader.ErrDuplicateCandidate}
	args := []string{
		"candidates", "add",
		"--symbol", "BTCUSDT",
		"--long", "binance",
		"--short", "bybit",
		"--threshold-bps", "200",
		"--max-position-usdt", "5000",
	}
	code, _, errOut := runWithFake(t, args, fr)
	if code != 3 {
		t.Fatalf("exit code: got %d, want 3 (duplicate)", code)
	}
	if !strings.Contains(strings.ToLower(errOut), "duplicate") {
		t.Errorf("stderr should mention duplicate: %q", errOut)
	}
}

// TestPgAdmin_CandidatesAdd_CapExceeded verifies cap-exceeded sentinel maps
// to exit 4.
func TestPgAdmin_CandidatesAdd_CapExceeded(t *testing.T) {
	fr := &fakeRegistry{addErr: pricegaptrader.ErrCapExceeded}
	args := []string{
		"candidates", "add",
		"--symbol", "BTCUSDT",
		"--long", "binance",
		"--short", "bybit",
		"--threshold-bps", "200",
		"--max-position-usdt", "5000",
	}
	code, _, _ := runWithFake(t, args, fr)
	if code != 4 {
		t.Fatalf("exit code: got %d, want 4 (cap)", code)
	}
}

// TestPgAdmin_CandidatesAdd_ValidationFails verifies an invalid candidate
// fails validation BEFORE Registry is called.
func TestPgAdmin_CandidatesAdd_ValidationFails(t *testing.T) {
	fr := &fakeRegistry{}
	args := []string{
		"candidates", "add",
		"--symbol", "btc/usdt", // bad — must be uppercase canonical
		"--long", "binance",
		"--short", "bybit",
		"--threshold-bps", "200",
		"--max-position-usdt", "5000",
	}
	code, _, _ := runWithFake(t, args, fr)
	if code != 2 {
		t.Fatalf("exit code: got %d, want 2 (validation)", code)
	}
	if len(fr.addCalls) != 0 {
		t.Fatalf("Registry.Add must not be called on validation failure; got %d", len(fr.addCalls))
	}
}

// TestPgAdmin_CandidatesAdd_DefaultsLowercaseExch verifies --long BINANCE is
// normalised to lowercase before validation/Registry call (handler parity).
func TestPgAdmin_CandidatesAdd_DefaultsLowercaseExch(t *testing.T) {
	fr := &fakeRegistry{}
	args := []string{
		"candidates", "add",
		"--symbol", "btcusdt", // lowercase — normaliser will uppercase
		"--long", "BINANCE",
		"--short", "BYBIT",
		"--threshold-bps", "200",
		"--max-position-usdt", "5000",
	}
	code, _, _ := runWithFake(t, args, fr)
	if code != 0 {
		t.Fatalf("exit code: got %d, want 0 (normalised input should pass)", code)
	}
	if len(fr.addCalls) != 1 {
		t.Fatalf("Add call count: got %d, want 1", len(fr.addCalls))
	}
	got := fr.addCalls[0].C
	if got.Symbol != "BTCUSDT" || got.LongExch != "binance" || got.ShortExch != "bybit" {
		t.Errorf("normalisation drift: got %+v", got)
	}
}

// TestPgAdmin_CandidatesDelete_Success verifies happy path.
func TestPgAdmin_CandidatesDelete_Success(t *testing.T) {
	fr := &fakeRegistry{}
	args := []string{"candidates", "delete", "--idx", "0"}
	code, _, _ := runWithFake(t, args, fr)
	if code != 0 {
		t.Fatalf("exit: got %d, want 0", code)
	}
	if len(fr.delCalls) != 1 {
		t.Fatalf("Delete call count: got %d, want 1", len(fr.delCalls))
	}
	if fr.delCalls[0].Source != "pg-admin" || fr.delCalls[0].Idx != 0 {
		t.Errorf("delete call: got %+v", fr.delCalls[0])
	}
}

// TestPgAdmin_CandidatesDelete_OutOfRange verifies sentinel→exit 5.
func TestPgAdmin_CandidatesDelete_OutOfRange(t *testing.T) {
	fr := &fakeRegistry{delErr: pricegaptrader.ErrIndexOutOfRange}
	args := []string{"candidates", "delete", "--idx", "99"}
	code, _, _ := runWithFake(t, args, fr)
	if code != 5 {
		t.Fatalf("exit: got %d, want 5 (out of range)", code)
	}
}

// TestPgAdmin_CandidatesReplace_FromFile verifies the JSON file replace path
// calls Registry.Replace once with the validated slice.
func TestPgAdmin_CandidatesReplace_FromFile(t *testing.T) {
	fr := &fakeRegistry{}
	tmp := t.TempDir()
	path := filepath.Join(tmp, "candidates.json")
	cands := []models.PriceGapCandidate{
		{Symbol: "BTCUSDT", LongExch: "binance", ShortExch: "bybit", ThresholdBps: 100, MaxPositionUSDT: 1000},
		{Symbol: "ETHUSDT", LongExch: "okx", ShortExch: "gateio", ThresholdBps: 150, MaxPositionUSDT: 2000},
	}
	data, _ := json.Marshal(cands)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	args := []string{"candidates", "replace", "--file", path}
	code, _, _ := runWithFake(t, args, fr)
	if code != 0 {
		t.Fatalf("exit: got %d, want 0", code)
	}
	if len(fr.repCalls) != 1 {
		t.Fatalf("Replace call count: got %d, want 1", len(fr.repCalls))
	}
	got := fr.repCalls[0]
	if got.Source != "pg-admin" {
		t.Errorf("source: got %q", got.Source)
	}
	if len(got.Next) != 2 {
		t.Errorf("next len: got %d, want 2", len(got.Next))
	}
}

// TestPgAdmin_CandidatesReplace_ValidationFails verifies the file path runs
// shared validation before Registry.Replace.
func TestPgAdmin_CandidatesReplace_ValidationFails(t *testing.T) {
	fr := &fakeRegistry{}
	tmp := t.TempDir()
	path := filepath.Join(tmp, "bad.json")
	bad := []models.PriceGapCandidate{
		{Symbol: "btc/usdt", LongExch: "binance", ShortExch: "bybit", ThresholdBps: 100, MaxPositionUSDT: 1000},
	}
	data, _ := json.Marshal(bad)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	args := []string{"candidates", "replace", "--file", path}
	code, _, _ := runWithFake(t, args, fr)
	if code != 2 {
		t.Fatalf("exit: got %d, want 2 (validation)", code)
	}
	if len(fr.repCalls) != 0 {
		t.Fatalf("Replace must not be called on validation failure; got %d", len(fr.repCalls))
	}
}

// TestPgAdmin_CandidatesList verifies stdout contains JSON of current
// candidates from Registry.List.
func TestPgAdmin_CandidatesList(t *testing.T) {
	fr := &fakeRegistry{
		listValue: []models.PriceGapCandidate{
			{Symbol: "BTCUSDT", LongExch: "binance", ShortExch: "bybit", ThresholdBps: 100, MaxPositionUSDT: 1000},
			{Symbol: "ETHUSDT", LongExch: "okx", ShortExch: "gateio", ThresholdBps: 150, MaxPositionUSDT: 2000},
		},
	}
	args := []string{"candidates", "list"}
	code, out, _ := runWithFake(t, args, fr)
	if code != 0 {
		t.Fatalf("exit: got %d, want 0", code)
	}
	var got []models.PriceGapCandidate
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("decode stdout JSON: %v (out=%q)", err, out)
	}
	if len(got) != 2 || got[0].Symbol != "BTCUSDT" || got[1].Symbol != "ETHUSDT" {
		t.Errorf("list output: got %+v", got)
	}
}

// TestPgAdmin_CandidatesUnknownSubcommand verifies bad sub-subcommand returns
// exit 2 (usage).
func TestPgAdmin_CandidatesUnknownSubcommand(t *testing.T) {
	fr := &fakeRegistry{}
	args := []string{"candidates", "frobnicate"}
	code, _, errOut := runWithFake(t, args, fr)
	if code != 2 {
		t.Fatalf("exit: got %d, want 2 (usage)", code)
	}
	if !strings.Contains(strings.ToLower(errOut), "candidates") {
		t.Errorf("stderr missing usage hint: %q", errOut)
	}
}

// guarantee the pricegaptrader sentinels are reachable to lint.
var _ = errors.Is
