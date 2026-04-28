package pricegaptrader

import (
	"os"
	"reflect"
	"regexp"
	"testing"

	"arb/internal/models"
)

// stubReader implements only the read-only RegistryReader contract so the
// compile-time assertion in TestRegistryReader_StubImplementsInterface holds.
type stubReader struct {
	candidates []models.PriceGapCandidate
}

func (s *stubReader) Get(idx int) (models.PriceGapCandidate, bool) {
	if idx < 0 || idx >= len(s.candidates) {
		return models.PriceGapCandidate{}, false
	}
	return s.candidates[idx], true
}

func (s *stubReader) List() []models.PriceGapCandidate {
	out := make([]models.PriceGapCandidate, len(s.candidates))
	copy(out, s.candidates)
	return out
}

// Compile-time assertion: stubReader must satisfy RegistryReader. If a future
// commit adds a mutator method (Add/Update/Delete/Replace) to the interface,
// this line stops compiling — surfacing the breach to CI immediately.
var _ RegistryReader = (*stubReader)(nil)

func TestRegistryReader_StubImplementsInterface(t *testing.T) {
	// The compile-time assertion above already proves the contract; this test
	// exists as an explicit regression hook so a future drift produces a clear
	// failure name in CI logs.
	var r RegistryReader = &stubReader{}
	if r == nil {
		t.Fatal("stubReader must satisfy RegistryReader at runtime")
	}
}

func TestRegistryReader_NoMutatorMethods(t *testing.T) {
	// Defense against accidental drift: even if someone adds a mutator method
	// to the interface in registry_reader.go, this grep harness fails CI.
	body, err := os.ReadFile("registry_reader.go")
	if err != nil {
		t.Fatalf("read registry_reader.go: %v", err)
	}

	// Scope to the interface declaration block. A non-greedy match captures
	// from `type RegistryReader interface {` through the matching `}`.
	scope := regexp.MustCompile(`(?s)type RegistryReader interface \{[^}]+\}`)
	loc := scope.Find(body)
	if loc == nil {
		t.Fatal("could not locate `type RegistryReader interface {...}` block")
	}

	mutator := regexp.MustCompile(`\b(Add|Update|Delete|Replace)\(`)
	if matches := mutator.FindAll(loc, -1); len(matches) > 0 {
		t.Fatalf("RegistryReader interface contains mutator method(s): %v — D-13 violation", matches)
	}
}

func TestRegistryReader_ListReturnTypeIsCandidateSlice(t *testing.T) {
	stub := &stubReader{candidates: []models.PriceGapCandidate{{Symbol: "BTCUSDT"}}}
	got := stub.List()

	wantType := reflect.TypeOf([]models.PriceGapCandidate{})
	gotType := reflect.TypeOf(got)
	if gotType != wantType {
		t.Fatalf("List() return type mismatch: got %v, want %v", gotType, wantType)
	}
	if len(got) != 1 || got[0].Symbol != "BTCUSDT" {
		t.Fatalf("List() returned unexpected payload: %+v", got)
	}
}

func TestRegistryReader_GetOutOfRangeReturnsFalse(t *testing.T) {
	stub := &stubReader{}
	if _, ok := stub.Get(99); ok {
		t.Fatal("Get(99) on empty stub must return ok=false")
	}
	if _, ok := stub.Get(-1); ok {
		t.Fatal("Get(-1) on empty stub must return ok=false")
	}

	stub2 := &stubReader{candidates: []models.PriceGapCandidate{{Symbol: "ETHUSDT"}}}
	got, ok := stub2.Get(0)
	if !ok {
		t.Fatal("Get(0) on populated stub must return ok=true")
	}
	if got.Symbol != "ETHUSDT" {
		t.Fatalf("Get(0).Symbol mismatch: got %q want ETHUSDT", got.Symbol)
	}
}

func TestRegistryReader_ModuleBoundary(t *testing.T) {
	// Module-boundary check (T-11-07): registry_reader.go must NOT import
	// internal/engine or internal/spotengine.
	body, err := os.ReadFile("registry_reader.go")
	if err != nil {
		t.Fatalf("read registry_reader.go: %v", err)
	}
	bad := regexp.MustCompile(`internal/(engine|spotengine)`)
	if bad.Match(body) {
		t.Fatal("registry_reader.go must not import internal/engine or internal/spotengine")
	}
}
