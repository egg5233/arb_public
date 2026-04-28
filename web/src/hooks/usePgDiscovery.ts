// Phase 11 Plan 06 — usePgDiscovery hook (PG-DISC-03 frontend).
//
// Owns the data layer for the Discovery sub-section in the Price-Gap tab:
//
//   1. REST seed on mount via GET /api/pg/discovery/state.
//   2. Per-symbol score history via GET /api/pg/discovery/scores/{symbol},
//      lazily fetched when the operator picks a symbol in <ScoreHistoryCard>.
//   3. WebSocket subscription mirroring the existing usePriceGapWebSocket
//      pattern in PriceGap.tsx — newline-separated JSON, switch on msg.type:
//        - pg_scan_cycle    → replace summary + why_rejected + score_snapshot
//        - pg_scan_metrics  → merge metrics fields only (5s throttled)
//        - pg_scan_score    → append point to scores[symbol] (debounced)
//
// The hook returns derived booleans (enabled/errored) and the per-symbol
// scores map so the container component can render the three scanner
// states (ON / OFF / ERRORED) per UI-SPEC.
import { useCallback, useEffect, useRef, useState } from 'react';

// ─── Response types — mirror Go telemetry types in
//     internal/pricegaptrader/telemetry.go (StateResponse, ScoresResponse). ──

export interface SubScores {
  spread_bps: number;
  depth_score: number;
  freshness_age_s: number;
  funding_bps_h: number;
  persistence_bars: number;
}

export interface SnapshotEntry {
  symbol: string;
  long_exch: string;
  short_exch: string;
  score: number;
  sub_scores: SubScores;
  gates_passed: string[];
  gates_failed: string[];
}

export interface DiscoveryState {
  enabled: boolean;
  last_run_at: number;
  next_run_in: number;
  candidates_seen: number;
  accepted: number;
  rejected: number;
  errors: number;
  duration_ms: number;
  why_rejected: Record<string, number>;
  score_snapshot: SnapshotEntry[];
  cycle_failed: boolean;
}

export interface ScorePoint {
  ts: number;
  score: number;
  sub_scores: SubScores;
}

export interface ThresholdBand {
  auto_promote: number;
}

export interface ScoresResponse {
  symbol: string;
  points: ScorePoint[];
  threshold_band: ThresholdBand;
}

interface ApiEnvelope<T> {
  ok: boolean;
  data?: T;
  error?: string;
}

interface UsePgDiscoveryResult {
  state: DiscoveryState | null;
  scores: Record<string, ScoresResponse>;
  loadScoresFor: (symbol: string) => void;
  loadingSymbol: string | null;
  errored: boolean;
  enabled: boolean;
  lastRunAt: number | null;
  wsConnected: boolean;
  seedError: string | null;
}

function authHeaders(): Record<string, string> {
  const token = (typeof localStorage !== 'undefined'
    ? localStorage.getItem('arb_token')
    : null) || '';
  return {
    'Content-Type': 'application/json',
    Authorization: `Bearer ${token}`,
  };
}

export function usePgDiscovery(): UsePgDiscoveryResult {
  const [state, setState] = useState<DiscoveryState | null>(null);
  const [scores, setScores] = useState<Record<string, ScoresResponse>>({});
  const [loadingSymbol, setLoadingSymbol] = useState<string | null>(null);
  const [seedError, setSeedError] = useState<string | null>(null);
  const [wsConnected, setWsConnected] = useState(true);

  // Track which symbol is currently displayed; used to filter pg_scan_score
  // events so we only mutate state for the active subscription.
  const subscribedSymbol = useRef<string | null>(null);

  // ── REST seed on mount ───────────────────────────────────────────────────
  const seed = useCallback(async () => {
    setSeedError(null);
    try {
      const res = await fetch('/api/pg/discovery/state', { headers: authHeaders() });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const body = (await res.json()) as ApiEnvelope<DiscoveryState>;
      if (!body.ok || !body.data) throw new Error(body.error || 'discovery seed failed');
      setState(body.data);
    } catch (err) {
      setSeedError(err instanceof Error ? err.message : String(err));
    }
  }, []);

  useEffect(() => {
    void seed();
  }, [seed]);

  // ── Lazy per-symbol score history ────────────────────────────────────────
  const loadScoresFor = useCallback((symbol: string) => {
    if (!symbol) return;
    subscribedSymbol.current = symbol;
    setLoadingSymbol(symbol);
    void (async () => {
      try {
        const res = await fetch(`/api/pg/discovery/scores/${encodeURIComponent(symbol)}`, {
          headers: authHeaders(),
        });
        if (!res.ok) throw new Error(`HTTP ${res.status}`);
        const body = (await res.json()) as ApiEnvelope<ScoresResponse>;
        if (!body.ok || !body.data) throw new Error(body.error || 'scores fetch failed');
        setScores((prev) => ({ ...prev, [symbol]: body.data! }));
      } catch {
        // Non-fatal: empty state copy renders if scores[symbol] is absent.
      } finally {
        setLoadingSymbol((curr) => (curr === symbol ? null : curr));
      }
    })();
  }, []);

  // ── WebSocket subscription ──────────────────────────────────────────────
  // Mirrors usePriceGapWebSocket (PriceGap.tsx:141-199): newline-separated
  // JSON, switch on msg.type. We open a separate connection so unmounting
  // the Discovery section cleans up just its subscriptions.
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
          switch (msg.type) {
            case 'pg_scan_cycle': {
              const next = msg.data as DiscoveryState;
              if (next && typeof next === 'object') {
                setState((prev) => ({ ...(prev ?? next), ...next }));
              }
              break;
            }
            case 'pg_scan_metrics': {
              const m = msg.data as Partial<DiscoveryState>;
              if (m && typeof m === 'object') {
                setState((prev) => (prev ? { ...prev, ...m } : null));
              }
              break;
            }
            case 'pg_scan_score': {
              const evt = msg.data as { symbol: string; point: ScorePoint };
              if (!evt || typeof evt.symbol !== 'string' || !evt.point) break;
              setScores((prev) => {
                const existing = prev[evt.symbol];
                if (!existing) return prev; // ignore symbols user hasn't loaded
                const points = [...existing.points, evt.point];
                // Trim to a sane upper bound (7d ZSET has ~2000 max @ 5min).
                const trimmed = points.length > 5000 ? points.slice(-5000) : points;
                return {
                  ...prev,
                  [evt.symbol]: { ...existing, points: trimmed },
                };
              });
              break;
            }
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

  const enabled = state?.enabled === true;
  const errored =
    state != null && (state.cycle_failed === true || (state.errors ?? 0) > 0);
  const lastRunAt = state?.last_run_at && state.last_run_at > 0 ? state.last_run_at : null;

  return {
    state,
    scores,
    loadScoresFor,
    loadingSymbol,
    errored,
    enabled,
    lastRunAt,
    wsConnected,
    seedError,
  };
}
