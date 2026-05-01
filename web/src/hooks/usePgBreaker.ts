// Phase 15 Plan 15-05 — usePgBreaker hook (PG-LIVE-02 frontend).
//
// Owns the data layer for the Drawdown Circuit Breaker subsection:
//
//   1. REST seed on mount via GET /api/pg/breaker/state.
//   2. WebSocket subscription mirroring usePgDiscovery's pattern — opens its
//      own /ws connection, switches on msg.type:
//        - pg.breaker.trip    → setState(tripped=true), setLastTrip(payload)
//        - pg.breaker.recover → setState(tripped=false, sticky=0), keep lastTrip
//      Event names match Hub.BroadcastPriceGapBreakerEvent calls in
//      internal/pricegaptrader/breaker_recovery.go + breaker_test_fire.go.
//   3. Mutators recover() + testFire(dryRun) call the typed-phrase REST
//      endpoints. The hook embeds the magic strings literally — they are
//      NOT i18n keys (CONTEXT D-12: magic strings are case-sensitive
//      regardless of locale).
//
// Module boundary: hook is unaware of the Phase 14 RampReconcileSection — it
// owns its own /ws connection so a future move of BreakerSubsection out of
// the Phase 14 widget (Phase 16 PG-OPS-09) is a pure relocation.
import { useCallback, useEffect, useRef, useState } from 'react';

// ─── Response types — mirror Go shapes in
//     internal/api/pricegap_breaker_handlers.go (handlePgBreakerStateGet
//     response Data map) + internal/models/pricegap_breaker.go.

export interface BreakerState {
  enabled: boolean;
  pending_strike: number;
  strike1_ts_ms: number;
  sticky_until_ms: number;
  last_eval_pnl_usdt: number;
  last_eval_ts_ms: number;
  threshold_usdt: number;
  armed: boolean;
  tripped: boolean;
  last_trip: BreakerTripRecord | null;
}

export interface BreakerTripRecord {
  trip_ts_ms: number;
  trip_pnl_usdt: number;
  threshold_usdt: number;
  ramp_stage: number;
  paused_candidate_count: number;
  recovery_ts_ms?: number | null;
  recovery_operator?: string | null;
  source: string;
}

interface ApiEnvelope<T> {
  ok: boolean;
  data?: T;
  error?: string;
}

interface MutatorResult {
  ok: boolean;
  error?: string;
  data?: Record<string, unknown>;
}

interface UsePgBreakerResult {
  state: BreakerState | null;
  recover: () => Promise<MutatorResult>;
  testFire: (dryRun: boolean) => Promise<MutatorResult>;
  refresh: () => Promise<void>;
  wsConnected: boolean;
  seedError: string | null;
  busy: boolean;
}

function authHeaders(): Record<string, string> {
  const token =
    (typeof localStorage !== 'undefined' ? localStorage.getItem('arb_token') : null) || '';
  return {
    'Content-Type': 'application/json',
    Authorization: `Bearer ${token}`,
  };
}

export function usePgBreaker(): UsePgBreakerResult {
  const [state, setState] = useState<BreakerState | null>(null);
  const [seedError, setSeedError] = useState<string | null>(null);
  const [wsConnected, setWsConnected] = useState(true);
  const [busy, setBusy] = useState(false);

  const seed = useCallback(async () => {
    setSeedError(null);
    try {
      const res = await fetch('/api/pg/breaker/state', { headers: authHeaders() });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const body = (await res.json()) as ApiEnvelope<BreakerState>;
      if (!body.ok || !body.data) throw new Error(body.error || 'breaker seed failed');
      setState(body.data);
    } catch (err) {
      setSeedError(err instanceof Error ? err.message : String(err));
    }
  }, []);

  useEffect(() => {
    void seed();
  }, [seed]);

  // Re-seed on WS reconnect to recover any state drift during disconnect.
  const prevWsConnected = useRef<boolean>(wsConnected);
  useEffect(() => {
    if (!prevWsConnected.current && wsConnected) {
      void seed();
    }
    prevWsConnected.current = wsConnected;
  }, [wsConnected, seed]);

  // ── WebSocket subscription ──────────────────────────────────────────────
  // Mirrors usePgDiscovery WS pattern. Listens for the two event types
  // emitted by the BreakerController (Plan 15-03 trip + Plan 15-04 recover).
  useEffect(() => {
    if (typeof WebSocket === 'undefined' || typeof location === 'undefined') return;
    const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:';
    const token =
      typeof localStorage !== 'undefined' ? localStorage.getItem('arb_token') || '' : '';
    let cancelled = false;
    let ws: WebSocket;
    try {
      ws = new WebSocket(`${protocol}//${location.host}/ws?token=${token}`);
    } catch {
      setWsConnected(false);
      return;
    }

    ws.onopen = () => {
      if (!cancelled) setWsConnected(true);
    };
    ws.onclose = () => {
      if (!cancelled) setWsConnected(false);
    };
    ws.onerror = () => {
      if (!cancelled) setWsConnected(false);
    };
    ws.onmessage = (event) => {
      const parts = String(event.data).split('\n');
      for (const part of parts) {
        const trimmed = part.trim();
        if (!trimmed) continue;
        try {
          const msg = JSON.parse(trimmed) as { type: string; data: unknown };
          if (msg.type === 'pg.breaker.trip') {
            const rec = msg.data as BreakerTripRecord;
            if (!rec || typeof rec !== 'object') continue;
            setState((prev) => {
              if (!prev) return prev;
              return {
                ...prev,
                tripped: true,
                armed: false,
                sticky_until_ms: rec.trip_ts_ms,
                last_trip: rec,
              };
            });
          } else if (msg.type === 'pg.breaker.recover') {
            // Recovery payload may include the trip record with recovery
            // backfill fields populated; mark tripped=false either way.
            const rec = msg.data as BreakerTripRecord | null;
            setState((prev) => {
              if (!prev) return prev;
              return {
                ...prev,
                tripped: false,
                armed: prev.enabled,
                sticky_until_ms: 0,
                pending_strike: 0,
                strike1_ts_ms: 0,
                last_trip: rec ?? prev.last_trip,
              };
            });
          }
        } catch {
          // ignore malformed frames
        }
      }
    };

    return () => {
      cancelled = true;
      try {
        ws.close();
      } catch {
        /* ignore */
      }
    };
  }, []);

  // ── Mutators ────────────────────────────────────────────────────────────
  // Magic strings are LITERAL per CONTEXT D-12 — never i18n'd, never
  // lowercased, never trimmed.

  const recover = useCallback(async (): Promise<MutatorResult> => {
    setBusy(true);
    try {
      const res = await fetch('/api/pg/breaker/recover', {
        method: 'POST',
        headers: authHeaders(),
        body: JSON.stringify({ confirmation_phrase: 'RECOVER' }),
      });
      const body = (await res.json().catch(() => null)) as MutatorResult | null;
      if (!res.ok || !body?.ok) {
        return { ok: false, error: body?.error || `HTTP ${res.status}` };
      }
      // Server returns 200; WS pg.breaker.recover will push the state
      // transition. As a belt+suspenders, refresh once.
      void seed();
      return { ok: true, data: body.data };
    } catch (err) {
      return { ok: false, error: err instanceof Error ? err.message : String(err) };
    } finally {
      setBusy(false);
    }
  }, [seed]);

  const testFire = useCallback(
    async (dryRun: boolean): Promise<MutatorResult> => {
      setBusy(true);
      try {
        const res = await fetch('/api/pg/breaker/test-fire', {
          method: 'POST',
          headers: authHeaders(),
          body: JSON.stringify({
            confirmation_phrase: 'TEST-FIRE',
            dry_run: dryRun,
          }),
        });
        const body = (await res.json().catch(() => null)) as MutatorResult | null;
        if (!res.ok || !body?.ok) {
          return { ok: false, error: body?.error || `HTTP ${res.status}` };
        }
        // For real trip the WS pg.breaker.trip arrives within 2s; for dry-run
        // there is no state transition. Refresh either way.
        void seed();
        return { ok: true, data: body.data };
      } catch (err) {
        return { ok: false, error: err instanceof Error ? err.message : String(err) };
      } finally {
        setBusy(false);
      }
    },
    [seed],
  );

  return {
    state,
    recover,
    testFire,
    refresh: seed,
    wsConnected,
    seedError,
    busy,
  };
}
