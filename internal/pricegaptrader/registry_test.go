package pricegaptrader

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"arb/internal/config"
	"arb/internal/models"
	"arb/pkg/utils"
)

// fakeAuditWriter is an in-memory RegistryAuditWriter capturing every LPush
// and LTrim call so tests can assert audit semantics without a real Redis.
type fakeAuditWriter struct {
	mu        sync.Mutex
	pushes    []fakeAuditPush
	trims     []fakeAuditTrim
	pushErr   error
	trimErr   error
	failAfter int // if >0, return error after N pushes (-1 = never)
}

type fakeAuditPush struct {
	Key  string
	Vals []interface{}
}

type fakeAuditTrim struct {
	Key        string
	Start, End int64
}

func newFakeAuditWriter() *fakeAuditWriter {
	return &fakeAuditWriter{failAfter: -1}
}

func (f *fakeAuditWriter) LPush(ctx context.Context, key string, vals ...interface{}) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.pushErr != nil {
		return 0, f.pushErr
	}
	if f.failAfter == 0 {
		return 0, errors.New("fake: push failure")
	}
	if f.failAfter > 0 {
		f.failAfter--
	}
	f.pushes = append(f.pushes, fakeAuditPush{Key: key, Vals: vals})
	return int64(len(f.pushes)), nil
}

func (f *fakeAuditWriter) LTrim(ctx context.Context, key string, start, stop int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.trimErr != nil {
		return f.trimErr
	}
	f.trims = append(f.trims, fakeAuditTrim{Key: key, Start: start, End: stop})
	return nil
}

func (f *fakeAuditWriter) pushCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.pushes)
}

func (f *fakeAuditWriter) trimCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.trims)
}

// setupRegistryTest provides a fresh Config + Registry against a tempdir
// CONFIG_FILE. Returns the registry, the config path, and the audit writer.
func setupRegistryTest(t *testing.T, capCandidates int) (*Registry, string, *fakeAuditWriter) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	t.Setenv("CONFIG_FILE", path)
	// Seed an empty config.json so the load + bak-ring path has a file to read.
	seed := map[string]interface{}{
		"strategy":  map[string]interface{}{},
		"price_gap": map[string]interface{}{},
	}
	b, _ := json.MarshalIndent(seed, "", "  ")
	if err := os.WriteFile(path, b, 0644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	cfg := config.Load()
	cfg.PriceGapMaxCandidates = capCandidates
	cfg.PriceGapDiscoveryEnabled = false
	audit := newFakeAuditWriter()
	log := utils.NewLogger("registry-test")
	r := NewRegistry(cfg, audit, log)
	return r, path, audit
}

// candidate is a small builder to keep test rows compact.
func candidate(symbol, longExch, shortExch, direction string) models.PriceGapCandidate {
	return models.PriceGapCandidate{
		Symbol:    symbol,
		LongExch:  longExch,
		ShortExch: shortExch,
		Direction: direction,
	}
}

func TestRegistry_AddSuccess(t *testing.T) {
	r, path, _ := setupRegistryTest(t, 5)
	c := candidate("ALPHAUSDT", "binance", "bybit", models.PriceGapDirectionBidirectional)
	if err := r.Add(context.Background(), "test", c); err != nil {
		t.Fatalf("add: %v", err)
	}
	if got := len(r.cfg.PriceGapCandidates); got != 1 {
		t.Fatalf("in-memory len=%d want 1", got)
	}
	// On-disk should reflect the new candidate.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(data), "ALPHAUSDT") {
		t.Fatalf("on-disk config does not contain ALPHAUSDT: %s", string(data))
	}
}

func TestRegistry_AddDedupe(t *testing.T) {
	r, _, _ := setupRegistryTest(t, 5)
	c := candidate("ALPHAUSDT", "binance", "bybit", models.PriceGapDirectionBidirectional)
	if err := r.Add(context.Background(), "test", c); err != nil {
		t.Fatalf("first add: %v", err)
	}
	err := r.Add(context.Background(), "test", c)
	if !errors.Is(err, ErrDuplicateCandidate) {
		t.Fatalf("expected ErrDuplicateCandidate, got %v", err)
	}
	if got := len(r.cfg.PriceGapCandidates); got != 1 {
		t.Fatalf("len=%d after duplicate add (want 1)", got)
	}
}

func TestRegistry_AddCapEnforcement(t *testing.T) {
	r, _, _ := setupRegistryTest(t, 2)
	if err := r.Add(context.Background(), "test", candidate("ALPHAUSDT", "binance", "bybit", models.PriceGapDirectionBidirectional)); err != nil {
		t.Fatalf("first: %v", err)
	}
	if err := r.Add(context.Background(), "test", candidate("BETAUSDT", "binance", "bybit", models.PriceGapDirectionBidirectional)); err != nil {
		t.Fatalf("second: %v", err)
	}
	err := r.Add(context.Background(), "test", candidate("GAMMAUSDT", "binance", "bybit", models.PriceGapDirectionBidirectional))
	if !errors.Is(err, ErrCapExceeded) {
		t.Fatalf("expected ErrCapExceeded at cap, got %v", err)
	}
	if got := len(r.cfg.PriceGapCandidates); got != 2 {
		t.Fatalf("len=%d after cap rejection (want 2)", got)
	}
}

func TestRegistry_Update(t *testing.T) {
	r, _, _ := setupRegistryTest(t, 5)
	if err := r.Add(context.Background(), "test", candidate("ALPHAUSDT", "binance", "bybit", models.PriceGapDirectionBidirectional)); err != nil {
		t.Fatalf("seed: %v", err)
	}
	updated := candidate("ALPHAUSDT", "binance", "okx", models.PriceGapDirectionBidirectional)
	if err := r.Update(context.Background(), "test", 0, updated); err != nil {
		t.Fatalf("update: %v", err)
	}
	if got := r.cfg.PriceGapCandidates[0].ShortExch; got != "okx" {
		t.Fatalf("shortExch=%q after update (want okx)", got)
	}
	err := r.Update(context.Background(), "test", 99, updated)
	if !errors.Is(err, ErrIndexOutOfRange) {
		t.Fatalf("expected ErrIndexOutOfRange for idx 99, got %v", err)
	}
}

func TestRegistry_Delete(t *testing.T) {
	r, _, _ := setupRegistryTest(t, 5)
	if err := r.Add(context.Background(), "test", candidate("ALPHAUSDT", "binance", "bybit", models.PriceGapDirectionBidirectional)); err != nil {
		t.Fatalf("seed1: %v", err)
	}
	if err := r.Add(context.Background(), "test", candidate("BETAUSDT", "binance", "bybit", models.PriceGapDirectionBidirectional)); err != nil {
		t.Fatalf("seed2: %v", err)
	}
	if err := r.Delete(context.Background(), "test", 0); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if got := len(r.cfg.PriceGapCandidates); got != 1 {
		t.Fatalf("len=%d after delete (want 1)", got)
	}
	if got := r.cfg.PriceGapCandidates[0].Symbol; got != "BETAUSDT" {
		t.Fatalf("after delete idx 0, symbol=%q want BETAUSDT", got)
	}
	err := r.Delete(context.Background(), "test", 99)
	if !errors.Is(err, ErrIndexOutOfRange) {
		t.Fatalf("expected ErrIndexOutOfRange for idx 99, got %v", err)
	}
}

func TestRegistry_Replace(t *testing.T) {
	r, path, _ := setupRegistryTest(t, 5)
	if err := r.Add(context.Background(), "test", candidate("ALPHAUSDT", "binance", "bybit", models.PriceGapDirectionBidirectional)); err != nil {
		t.Fatalf("seed: %v", err)
	}
	next := []models.PriceGapCandidate{
		candidate("BETAUSDT", "binance", "bybit", models.PriceGapDirectionBidirectional),
		candidate("GAMMAUSDT", "binance", "okx", models.PriceGapDirectionBidirectional),
	}
	if err := r.Replace(context.Background(), "test", next); err != nil {
		t.Fatalf("replace: %v", err)
	}
	if got := len(r.cfg.PriceGapCandidates); got != 2 {
		t.Fatalf("len=%d after replace (want 2)", got)
	}
	if r.cfg.PriceGapCandidates[0].Symbol != "BETAUSDT" || r.cfg.PriceGapCandidates[1].Symbol != "GAMMAUSDT" {
		t.Fatalf("replace did not swap correctly: %+v", r.cfg.PriceGapCandidates)
	}
	// On-disk should not contain ALPHAUSDT.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if strings.Contains(string(data), "ALPHAUSDT") {
		t.Fatalf("on-disk config still contains ALPHAUSDT after replace")
	}
}

func TestRegistry_Get(t *testing.T) {
	r, _, _ := setupRegistryTest(t, 5)
	c := candidate("ALPHAUSDT", "binance", "bybit", models.PriceGapDirectionBidirectional)
	if err := r.Add(context.Background(), "test", c); err != nil {
		t.Fatalf("seed: %v", err)
	}
	got, ok := r.Get(0)
	if !ok {
		t.Fatalf("Get(0) returned ok=false")
	}
	if got.Symbol != "ALPHAUSDT" {
		t.Fatalf("Get(0).Symbol=%q want ALPHAUSDT", got.Symbol)
	}
	if _, ok := r.Get(99); ok {
		t.Fatalf("Get(99) returned ok=true for out-of-range")
	}
	if _, ok := r.Get(-1); ok {
		t.Fatalf("Get(-1) returned ok=true for out-of-range")
	}
}

func TestRegistry_ListDefensiveCopy(t *testing.T) {
	r, _, _ := setupRegistryTest(t, 5)
	if err := r.Add(context.Background(), "test", candidate("ALPHAUSDT", "binance", "bybit", models.PriceGapDirectionBidirectional)); err != nil {
		t.Fatalf("seed: %v", err)
	}
	first := r.List()
	if len(first) != 1 {
		t.Fatalf("first list len=%d", len(first))
	}
	first[0].Symbol = "MUTATED"
	second := r.List()
	if second[0].Symbol == "MUTATED" {
		t.Fatalf("List() did not return a defensive copy — mutation leaked back")
	}
}

func TestRegistry_RegistryReaderInterfaceSatisfied(t *testing.T) {
	// Compile-time assertion is in registry.go (`var _ RegistryReader = (*Registry)(nil)`),
	// but runtime check guards against accidental signature drift.
	var rr RegistryReader = (*Registry)(nil)
	_ = rr
}

func TestRegistry_AuditTrail(t *testing.T) {
	r, _, audit := setupRegistryTest(t, 5)
	if err := r.Add(context.Background(), "dashboard-handler", candidate("ALPHAUSDT", "binance", "bybit", models.PriceGapDirectionBidirectional)); err != nil {
		t.Fatalf("add: %v", err)
	}
	if got := audit.pushCount(); got != 1 {
		t.Fatalf("audit pushes=%d want 1", got)
	}
	if got := audit.trimCount(); got != 1 {
		t.Fatalf("audit trims=%d want 1", got)
	}
	push := audit.pushes[0]
	if push.Key != "pg:registry:audit" {
		t.Fatalf("audit key=%q want pg:registry:audit", push.Key)
	}
	if len(push.Vals) != 1 {
		t.Fatalf("expected single value in LPush, got %d", len(push.Vals))
	}
	payload, ok := push.Vals[0].(string)
	if !ok {
		t.Fatalf("audit payload not string: %T", push.Vals[0])
	}
	var rec map[string]interface{}
	if err := json.Unmarshal([]byte(payload), &rec); err != nil {
		t.Fatalf("payload not JSON: %v (%s)", err, payload)
	}
	for _, k := range []string{"ts", "source", "op", "before_count", "after_count"} {
		if _, ok := rec[k]; !ok {
			t.Fatalf("audit payload missing key %q: %+v", k, rec)
		}
	}
	if rec["op"] != "add" {
		t.Fatalf("op=%v want add", rec["op"])
	}
	if rec["source"] != "dashboard-handler" {
		t.Fatalf("source=%v want dashboard-handler", rec["source"])
	}
	if rec["before_count"].(float64) != 0 || rec["after_count"].(float64) != 1 {
		t.Fatalf("counts wrong: %+v", rec)
	}
}

func TestRegistry_AuditOnFailureNotWritten(t *testing.T) {
	r, _, audit := setupRegistryTest(t, 1)
	if err := r.Add(context.Background(), "test", candidate("ALPHAUSDT", "binance", "bybit", models.PriceGapDirectionBidirectional)); err != nil {
		t.Fatalf("seed: %v", err)
	}
	beforePushes := audit.pushCount()
	err := r.Add(context.Background(), "test", candidate("BETAUSDT", "binance", "bybit", models.PriceGapDirectionBidirectional))
	if !errors.Is(err, ErrCapExceeded) {
		t.Fatalf("expected ErrCapExceeded, got %v", err)
	}
	if got := audit.pushCount(); got != beforePushes {
		t.Fatalf("audit pushes increased on failed add: %d → %d", beforePushes, got)
	}
}

func TestRegistry_AuditListTrim(t *testing.T) {
	r, _, audit := setupRegistryTest(t, 250)
	for i := 0; i < 201; i++ {
		c := models.PriceGapCandidate{
			Symbol:    "S" + paddedIndex(i) + "USDT",
			LongExch:  "binance",
			ShortExch: "bybit",
			Direction: models.PriceGapDirectionBidirectional,
		}
		if err := r.Add(context.Background(), "test", c); err != nil {
			t.Fatalf("add %d: %v", i, err)
		}
	}
	// Each successful Add must call LTrim with bounds (0, 199).
	for _, tr := range audit.trims {
		if tr.Key != "pg:registry:audit" {
			t.Fatalf("trim key=%q want pg:registry:audit", tr.Key)
		}
		if tr.Start != 0 || tr.End != 199 {
			t.Fatalf("trim bounds=(%d,%d) want (0,199)", tr.Start, tr.End)
		}
	}
	if got := audit.trimCount(); got != 201 {
		t.Fatalf("expected 201 trim calls, got %d", got)
	}
}

func paddedIndex(i int) string {
	if i < 10 {
		return "00" + string(rune('0'+i))
	}
	if i < 100 {
		return string(rune('0'+i/10)) + string(rune('0'+i%10))
	}
	a := i / 100
	b := (i / 10) % 10
	c := i % 10
	return string(rune('0'+a)) + string(rune('0'+b)) + string(rune('0'+c))
}

func TestRegistry_ReloadFromDisk(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	t.Setenv("CONFIG_FILE", path)

	// Seed a config.json that contains one PriceGap candidate already.
	seed := map[string]interface{}{
		"strategy": map[string]interface{}{},
		"price_gap": map[string]interface{}{
			"candidates": []map[string]interface{}{
				{"symbol": "EXTERNALUSDT", "long_exch": "binance", "short_exch": "okx", "direction": "bidirectional"},
			},
		},
	}
	b, _ := json.MarshalIndent(seed, "", "  ")
	if err := os.WriteFile(path, b, 0644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// Construct Registry with a fresh Load() — its in-memory cfg has the
	// EXTERNALUSDT candidate from disk after Load().
	cfg := config.Load()
	cfg.PriceGapMaxCandidates = 5
	audit := newFakeAuditWriter()
	log := utils.NewLogger("registry-test")
	r := NewRegistry(cfg, audit, log)

	// Simulate an external writer (pg-admin) appending a candidate to the
	// disk file while the registry's in-memory cfg is unaware.
	current := []map[string]interface{}{
		{"symbol": "EXTERNALUSDT", "long_exch": "binance", "short_exch": "okx", "direction": "bidirectional"},
		{"symbol": "EXTERNAL2USDT", "long_exch": "binance", "short_exch": "bybit", "direction": "bidirectional"},
	}
	seed["price_gap"].(map[string]interface{})["candidates"] = current
	b2, _ := json.MarshalIndent(seed, "", "  ")
	if err := os.WriteFile(path, b2, 0644); err != nil {
		t.Fatalf("rewrite: %v", err)
	}

	// Now Add via the registry — it must reload from disk first and pick up
	// the externally-added candidate before applying its own.
	if err := r.Add(context.Background(), "test", candidate("OWNUSDT", "binance", "bybit", models.PriceGapDirectionBidirectional)); err != nil {
		t.Fatalf("add: %v", err)
	}

	// Final on-disk state should contain all three: the seed, the external
	// addition, and the registry's own.
	final, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read final: %v", err)
	}
	for _, sym := range []string{"EXTERNALUSDT", "EXTERNAL2USDT", "OWNUSDT"} {
		if !strings.Contains(string(final), sym) {
			t.Fatalf("final on-disk config missing %s: %s", sym, string(final))
		}
	}
	if got := len(r.cfg.PriceGapCandidates); got != 3 {
		t.Fatalf("in-memory len=%d after add (want 3 — reload-from-disk picked up external)", got)
	}
}
