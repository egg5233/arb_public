# Testing Patterns

**Analysis Date:** 2026-04-01

## Test Framework

**Runner:**
- Go standard `testing` package (no third-party test framework)
- Config: none -- uses `go test` defaults

**Assertion Library:**
- No assertion library. Tests use direct `if` checks with `t.Errorf()`, `t.Fatalf()`, `t.Fatal()`
- Pattern: fail-fast with `t.Fatalf()` for setup errors, `t.Errorf()` for assertion failures

**Run Commands:**
```bash
go test ./...                          # Run all tests
go test ./internal/risk/...            # Run specific package tests
go test -v ./internal/spotengine/...   # Verbose output
go test -run TestPriceSpikeTriggers    # Run specific test
go test -count=1 ./...                 # No caching
```

**Live Exchange Tests (NOT go test):**
```bash
go run ./cmd/livetest/ --exchange binance          # Test single exchange
go run ./cmd/livetest/ --exchange bitget --skip-orders  # Skip order tests
go run ./cmd/livetest/ --test-margin               # Test spot margin (uses real funds)
```

## Test File Organization

**Location:** Co-located with source files (standard Go convention)

**Naming:** `{source_file}_test.go` in the same package

**Test Files (25 total):**

| Package | Test File | What It Tests |
|---------|-----------|---------------|
| `internal/config` | `config_test.go` | SaveJSON credential preservation, backup creation |
| `internal/engine` | `engine_test.go` | effectiveAdvanceMin scaling, rotation classification |
| `internal/discovery` | `ranker_test.go` | Interval propagation, cost ratio calculation, CoinGlass parsing, rate normalization |
| `internal/discovery` | `test_helpers_test.go` | Shared stub exchange and test logger (support file, not actual tests) |
| `internal/database` | `locks_test.go` | Owned lock release safety, lease loss detection |
| `internal/models` | `opportunity_test.go` | JSON serialization round-trip for IntervalHours |
| `internal/api` | `handlers_test.go` | Version comparison logic |
| `internal/api` | `config_handlers_test.go` | Config POST/GET round-trips for all config sections |
| `internal/api` | `spot_handlers_test.go` | Spot position health endpoint with timestamps |
| `internal/api` | `spot_stats_test.go` | Cold start, partial hash, full hash stats responses |
| `internal/risk` | `health_test.go` | Margin health level computation (L0-L5), hybrid fallback |
| `internal/risk` | `allocator_test.go` | Reserve/commit/release, strategy caps, TTL expiry, reconciliation, concurrent reservation |
| `internal/risk` | `liq_trend_test.go` | Trend tracking, linear regression, cold start, spike filtering |
| `internal/risk` | `spread_stability_test.go` | Spread stability gate, CV thresholds, sparse history rejection |
| `internal/risk` | `exchange_scorer_test.go` | Exchange health scoring: all-healthy, high latency, WS down scenarios |
| `internal/notify` | `telegram_test.go` | Nil safety, duration formatting, exit reason formatting |
| `internal/spotengine` | `discovery_test.go` | Discovery scan with non-positive funding retention |
| `internal/spotengine` | `execution_test.go` | Capital classification, PnL calculation, separate-account detection |
| `internal/spotengine` | `exit_triggers_test.go` | Price spike triggers (both directions), margin health, borrow cost grace timer, negative yield state machine |
| `internal/spotengine` | `risk_gate_test.go` | Risk gate ordering (dry_run vs capacity/duplicate/cooldown/persistence) |
| `internal/spotengine` | `rate_velocity_test.go` | Borrow rate spike detection with window/multiplier/absolute thresholds |
| `pkg/exchange/okx` | `adapter_test.go` | ctVal contract multiplier round-trip (position, order, WS) |
| `pkg/exchange/gateio` | `adapter_test.go` | quanto_multiplier round-trip, EnsureOneWayMode JSON body, IOC partial order status |
| `pkg/exchange/bybit` | `margin_test.go` | GetSpotMarginOrder fallback from realtime to history endpoint |
| `pkg/exchange/bitget` | `client_sign_test.go` | Chinese symbol query string signing consistency |

## Test Structure

**Suite Organization:**
- Table-driven tests are the dominant pattern
- `t.Run(tt.name, ...)` for subtests
- `t.Helper()` on all test helper/factory functions
- `t.Cleanup()` for teardown (Redis connections, miniredis servers)

**Standard Table-Driven Pattern:**
```go
func TestComputeLevel(t *testing.T) {
    cfg := &config.Config{
        MarginL3Threshold: 0.50,
        MarginL4Threshold: 0.80,
        MarginL5Threshold: 0.95,
    }
    h := &HealthMonitor{cfg: cfg}

    tests := []struct {
        name     string
        bal      *exchange.Balance
        pnl      float64
        posCount int
        want     HealthLevel
    }{
        {
            name:     "L0: no positions",
            bal:      &exchange.Balance{Total: 100, Available: 100, MarginRatio: 0},
            pnl:      0,
            posCount: 0,
            want:     L0None,
        },
        // ... more cases
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := h.computeLevel(h.normalizeMarginRatio(tt.bal), tt.pnl, tt.posCount)
            if got != tt.want {
                t.Errorf("computeLevel() = %s, want %s", got, tt.want)
            }
        })
    }
}
```

## Mocking

**Framework:** No mocking framework. Hand-written stubs implementing the `exchange.Exchange` interface.

**Exchange Stub Pattern:**
- Each test package defines its own minimal stub with only the needed behavior
- All interface methods implemented as no-ops returning zero values
- Specific methods overridden for the test scenario

**Example stub from `internal/discovery/test_helpers_test.go`:**
```go
type stubExchange struct{ name string }

func (s stubExchange) Name() string                                         { return s.name }
func (s stubExchange) PlaceOrder(exchange.PlaceOrderParams) (string, error) { return "", nil }
func (s stubExchange) CancelOrder(string, string) error                     { return nil }
// ... all other interface methods return zero values

func makeNilExchangeMap(names ...string) map[string]exchange.Exchange {
    m := make(map[string]exchange.Exchange, len(names))
    for _, n := range names {
        m[n] = stubExchange{name: n}
    }
    return m
}
```

**Specialized Stubs:**
- `closeTestExchange` in `internal/spotengine/execution_test.go` -- tracks `placeCalls`, returns configurable `orderUpdates` and `orderbook`
- `priceStubExchange` in `internal/spotengine/exit_triggers_test.go` -- returns fixed price for orderbook queries
- `marginStubExchange` in `internal/spotengine/exit_triggers_test.go` -- implements `SpotMarginExchange` with configurable `available` balance

**Redis Mocking:**
- Uses `github.com/alicebob/miniredis/v2` for in-memory Redis in all tests that need state persistence
- Pattern:
```go
func newTestServer(t *testing.T) (*Server, *miniredis.Miniredis) {
    t.Helper()
    mr, err := miniredis.Run()
    if err != nil {
        t.Fatalf("miniredis: %v", err)
    }
    db, err := database.New(mr.Addr(), "", 0)
    if err != nil {
        mr.Close()
        t.Fatalf("database.New: %v", err)
    }
    // ... return server wired to miniredis
}
```
- `mr.FastForward(duration)` used to simulate TTL expiry (see `allocator_test.go`, `locks_test.go`)
- `mr.HSet()` used to seed Redis hashes directly (see `spot_stats_test.go`)
- `t.Cleanup(func() { _ = db.Close() })` for teardown

**HTTP Server Mocking:**
- `net/http/httptest` for exchange API tests
- Stub HTTP handlers return canned JSON responses
- Used in adapter tests: `pkg/exchange/okx/adapter_test.go`, `pkg/exchange/gateio/adapter_test.go`, `pkg/exchange/bybit/margin_test.go`

**Example httptest pattern:**
```go
srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    switch {
    case r.URL.Path == "/api/v5/public/instruments":
        respond([]map[string]interface{}{{
            "instId": instID,
            "ctVal":  "10",
            // ...
        }})
    case r.URL.Path == "/api/v5/trade/order" && r.Method == "POST":
        // capture request body for assertion
        json.NewDecoder(r.Body).Decode(&req)
        respond([]map[string]interface{}{{"ordId": "123456"}})
    }
}))
defer srv.Close()

adapter := &Adapter{client: NewClientWithBase(srv.URL)}
```

## Fixtures and Factories

**Test Data:**
- Inline struct literals in each test -- no shared fixtures directory
- Helper functions create pre-configured test objects:

```go
func newAllocatorTest(t *testing.T) (*CapitalAllocator, *database.Client, *miniredis.Miniredis) {
    t.Helper()
    mr := miniredis.RunT(t)
    db, err := database.New(mr.Addr(), "", 2)
    // ...
    cfg := &config.Config{
        EnableCapitalAllocator: true,
        MaxTotalExposureUSDT:   100,
        // ...
    }
    return NewCapitalAllocator(db, cfg), db, mr
}

func seedActivePosition(t *testing.T, db *database.Client, symbol, exchange string) {
    t.Helper()
    pos := &models.SpotFuturesPosition{
        ID:       "test-" + symbol + "-" + exchange,
        Symbol:   symbol,
        Exchange: exchange,
        Status:   models.SpotStatusActive,
    }
    if err := db.SaveSpotPosition(pos); err != nil {
        t.Fatalf("seed position: %v", err)
    }
}
```

**Test Logger:**
```go
func newTestLogger() *utils.Logger {
    return utils.NewLogger("test")
}
```

## Coverage

**Requirements:** No enforced coverage threshold

**View Coverage:**
```bash
go test -coverprofile=cover.out ./...
go tool cover -html=cover.out
```

## Test Types

**Unit Tests (Go standard `_test.go`):**
- Pure logic testing with table-driven cases
- No external dependencies (Redis mocked with miniredis, HTTP with httptest)
- Cover: risk calculations, config parsing, data model serialization, exit trigger logic, rate normalization, PnL math

**Integration Tests (API handler tests):**
- `internal/api/*_test.go` -- test full HTTP handler chains
- Wire up `Server` with real `database.Client` (backed by miniredis)
- Test config POST/GET round-trips including Redis persistence and file persistence
- Use `httptest.NewRequest` + `httptest.NewRecorder`

**Exchange Adapter Tests:**
- `pkg/exchange/{name}/*_test.go` -- test adapter logic with canned HTTP responses
- Focus on unit conversions (ctVal, quanto_multiplier), signing consistency, endpoint fallbacks
- Use `httptest.NewServer` to mock exchange REST APIs

**Live Exchange Tests (cmd/livetest):**
- Located in `cmd/livetest/main.go`
- Tests real exchange connectivity with actual API keys
- 26 test cases per exchange covering: contracts, balance, orders, positions, funding, WS
- Flags: `--exchange`, `--skip-orders`, `--test-margin`
- NOT run via `go test` -- invoked manually: `go run ./cmd/livetest/ --exchange binance`
- Requires real API keys and network access

## Common Patterns

**Async/Concurrency Testing:**
```go
func TestCapitalAllocatorConcurrentReservePreventsOvercommit(t *testing.T) {
    allocator, _, _ := newAllocatorTest(t)

    var wg sync.WaitGroup
    var mu sync.Mutex
    successes := 0

    for range 8 {
        wg.Add(1)
        go func() {
            defer wg.Done()
            res, err := allocator.Reserve(StrategyPerpPerp, map[string]float64{"binance": 35})
            if err != nil || res == nil {
                return
            }
            mu.Lock()
            successes++
            mu.Unlock()
        }()
    }
    wg.Wait()

    if successes != 1 {
        t.Fatalf("expected exactly 1 successful reservation, got %d", successes)
    }
}
```

**Time-Based Testing (TTL Expiry):**
```go
func TestCapitalAllocatorReservationTTLExpiry(t *testing.T) {
    allocator, _, mr := newAllocatorTest(t)
    if _, err := allocator.Reserve(...); err != nil {
        t.Fatalf("Reserve: %v", err)
    }
    mr.FastForward(3 * time.Second)  // simulate TTL expiry
    summary, _ := allocator.Summary()
    if summary.TotalExposure != 0 {
        t.Fatalf("expected reservation to expire")
    }
}
```

**Environment Variable Injection:**
```go
func TestSaveJSON_PreservesExistingCredentials(t *testing.T) {
    configPath := filepath.Join(t.TempDir(), "config.json")
    // ... write initial config
    t.Setenv("CONFIG_FILE", configPath)
    // ... test SaveJSON behavior
}
```

**Regression Test Labeling:**
- Tests reference ticket IDs in comments: `// Regression test for ARB-62`, `// Regression test for ARB-74`
- Helps trace test purpose back to the original bug report

## What Is Tested vs What Is Not

**Well-Tested Areas:**
- Risk module: health levels, capital allocation, spread stability, exchange scoring, liquidation trend tracking
- Spot-futures engine: exit triggers (price spike, margin health, borrow cost), risk gate ordering, PnL calculation, discovery scan logic, rate velocity detection
- Config: save/load round-trips, credential preservation, zero-value guards
- API handlers: config endpoints, spot stats, spot position health
- Exchange adapters: contract multiplier conversions (OKX, Gate.io), signing (Bitget), endpoint fallbacks (Bybit)
- Models: JSON serialization
- Database: distributed lock safety

**Not Tested / Gaps:**
- **Engine core flow**: `internal/engine/engine.go` -- the main orchestration loop (discovery -> risk -> execute -> monitor -> exit) has only 2 unit tests for helper functions. The depth-fill execution, consolidation, and exit flow are untested.
- **Discovery scanner network logic**: `internal/discovery/scanner.go` Loris API polling, verification against exchange-native APIs -- no tests for the HTTP interaction or scan scheduling.
- **WebSocket client code**: `pkg/exchange/{name}/ws.go` and `ws_private.go` -- no tests for WS connection, reconnection, or message parsing (except OKX order update parsing in adapter_test).
- **Frontend**: Zero test files. No unit tests, no component tests, no E2E tests for the React dashboard.
- **Telegram notifications**: Only nil-safety and formatting tests. No tests for actual HTTP calls to Telegram API.
- **Database state operations**: `internal/database/state.go`, `spot_state.go` -- CRUD operations for positions/history/funding not tested directly (tested indirectly through API handler tests).
- **Scraper**: `internal/scraper/spotarb.go` -- CoinGlass scraping via Chromedp has no tests.
- **Transfer/balance operations**: Fund transfers between exchanges (`cmd/transfer/`, `cmd/balance/`) have no unit tests.

**Frontend Testing:**
- Framework: None configured
- No test files exist in `web/src/`
- No test runner configured in `web/package.json` (Vite only)
- E2E framework: Not used

---

*Testing analysis: 2026-04-01*
