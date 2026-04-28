// Package testhelpers contains test-only utilities for the pricegaptrader
// package. Lives under internal/ to keep the production cmd/ tree clean —
// this is a library, NOT a binary. Tests that need cross-instance race
// semantics import it directly (no os.Exec required).
//
// SCOPE BOUNDARY (Plan 02 D-09 / T-11-46): This package must NOT be imported
// by production code. Only `_test.go` files in internal/pricegaptrader and
// any future Plan 03+ test packages should reach in. Audit:
//
//	grep -r "pricegaptrader/testhelpers" --include='*.go' \
//	  --exclude='*_test.go' internal/ cmd/ pkg/ # must return empty
package testhelpers

import (
	"context"
	"fmt"

	"arb/internal/config"
	"arb/internal/models"
	"arb/internal/pricegaptrader"
	"arb/pkg/utils"
)

// NopAuditWriter satisfies pricegaptrader.RegistryAuditWriter without
// recording anything. Used in cross-instance race tests where audit-capture
// is not the property under test (each *Registry instance has its own
// in-memory audit writer; the test cares only about disk-state convergence).
type NopAuditWriter struct{}

// LPush is a no-op satisfying pricegaptrader.RegistryAuditWriter.
func (NopAuditWriter) LPush(ctx context.Context, key string, vals ...interface{}) (int64, error) {
	return 0, nil
}

// LTrim is a no-op satisfying pricegaptrader.RegistryAuditWriter.
func (NopAuditWriter) LTrim(ctx context.Context, key string, start, stop int64) error {
	return nil
}

// RunWriterRound constructs a fresh *config.Config + *pricegaptrader.Registry
// against the given CONFIG_FILE path and calls Add `count` times with the
// given symbol prefix. Each invocation is INDEPENDENT — separate in-memory
// cfg, separate registry, shared disk file. Models the pg-admin × dashboard
// race surface in a single process so tests can fan out via goroutines
// without spawning OS processes.
//
// Returns the count of successful Adds (equal to count if no contention
// collisions or cap rejections occurred).
//
// SCOPE: cfgPath must be a sandbox path (CONFIG_FILE env was set in the
// test). Symbols use the pattern "<symbolPrefix><NN>USDT".
func RunWriterRound(cfgPath, symbolPrefix string, count int) (int, error) {
	cfg := config.Load()
	// Allow at least `count` candidates so cap doesn't reject the round.
	// (If two RunWriterRound calls race against the same cfgPath with
	// non-colliding prefixes, the on-disk file's max_candidates governs the
	// shared cap — caller is responsible for sizing it.)
	if cfg.PriceGapMaxCandidates == 0 {
		cfg.PriceGapMaxCandidates = count * 4
	}
	log := utils.NewLogger("testhelpers-writer")
	r := pricegaptrader.NewRegistry(cfg, NopAuditWriter{}, log)

	successes := 0
	for i := 0; i < count; i++ {
		c := models.PriceGapCandidate{
			Symbol:    fmt.Sprintf("%s%02dUSDT", symbolPrefix, i),
			LongExch:  "binance",
			ShortExch: "bybit",
			Direction: models.PriceGapDirectionBidirectional,
		}
		if err := r.Add(context.Background(), "testhelpers", c); err != nil {
			continue
		}
		successes++
	}
	return successes, nil
}
