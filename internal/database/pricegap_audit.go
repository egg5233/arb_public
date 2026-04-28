// Package database — pricegap_audit.go: thin RegistryAuditWriter adapter.
//
// Plan 11-03 / PG-DISC-04 wires *pricegaptrader.Registry to a narrow
// RegistryAuditWriter interface (just LPush + LTrim). The Registry must not
// import internal/database (interface-driven DI), so we expose a small adapter
// here that wraps *database.Client's go-redis client and satisfies the
// pricegaptrader.RegistryAuditWriter signatures verbatim.
//
// The adapter is intentionally thin — every method delegates one-to-one to
// the underlying *redis.Client so the existing connection, dial timeouts, and
// retry policies apply. No in-process buffering, no Redis-side validation
// beyond what the driver does, no logging (Registry handles best-effort
// logging upstream).
package database

import "context"

// PriceGapAuditWriter is a *database.Client adapter satisfying the
// pricegaptrader.RegistryAuditWriter interface. Construct via
// (*Client).PriceGapAudit().
type PriceGapAuditWriter struct {
	c *Client
}

// PriceGapAudit returns a RegistryAuditWriter-compatible adapter that writes
// to the same go-redis client this *database.Client owns. Safe to call
// concurrently.
func (c *Client) PriceGapAudit() *PriceGapAuditWriter {
	return &PriceGapAuditWriter{c: c}
}

// LPush prepends one or more values to the Redis list at key. Returns the new
// list length and any error from the driver.
func (w *PriceGapAuditWriter) LPush(ctx context.Context, key string, vals ...interface{}) (int64, error) {
	return w.c.rdb.LPush(ctx, key, vals...).Result()
}

// LTrim trims the list at key to the inclusive range [start, stop].
func (w *PriceGapAuditWriter) LTrim(ctx context.Context, key string, start, stop int64) error {
	return w.c.rdb.LTrim(ctx, key, start, stop).Err()
}
