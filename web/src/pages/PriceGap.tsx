import { useState, useEffect, useMemo, useCallback, type FC } from 'react';
import { useLocale } from '../i18n/index.ts';

// ─── Data shapes ──────────────────────────────────────────────────────────
// Mirror the Go response shapes from internal/api/pricegap_handlers.go
// (priceGapStateResponse + priceGapCandidateView) and WS payloads from
// internal/pricegaptrader/notify.go (PriceGapEvent, PriceGapCandidateUpdate).

interface PriceGapCandidate {
  symbol: string;
  long_exch: string;
  short_exch: string;
  threshold_bps: number;
  max_position_usdt: number;
  modeled_slippage_bps?: number;
  disabled: boolean;
  reason?: string;
  disabled_at?: number;
}

interface PriceGapPosition {
  id: string;
  symbol: string;
  long_exchange: string;
  short_exchange: string;
  status: string;
  mode: 'paper' | 'live';
  entry_spread_bps: number;
  threshold_bps: number;
  notional_usdt: number;
  long_size: number;
  short_size: number;
  long_fill_price: number;
  short_fill_price: number;
  realized_pnl: number;
  modeled_slippage_bps: number;
  realized_slippage_bps: number;
  exit_reason?: string;
  opened_at: string;
  closed_at?: string;
  current_spread_bps?: number;
  current_pnl?: number;
}

interface CandidateMetrics {
  candidate: string;
  symbol: string;
  long_exchange: string;
  short_exchange: string;
  trades_window: number;
  win_pct: number;
  avg_realized_bps: number;
  bps_24h_per_day: number;
  bps_7d_per_day: number;
  bps_30d_per_day: number;
}

interface PriceGapState {
  enabled: boolean;
  paper_mode: boolean;
  debug_log: boolean; // Phase 9 gap-closure Gap #1 (Plan 09-09): rate-limited non-fire logger gate
  budget: number;
  candidates: PriceGapCandidate[];
  active_positions: PriceGapPosition[] | null;
  recent_closed: PriceGapPosition[] | null;
  metrics: CandidateMetrics[] | null;
}

interface PriceGapEvent {
  type: 'entry' | 'exit' | 'auto_disable';
  position?: PriceGapPosition;
  symbol?: string;
  reason?: string;
}

interface PriceGapCandidateUpdate {
  symbol: string;
  disabled: boolean;
  reason?: string;
  disabled_at?: number;
}

// ─── Helpers (inherited from Positions.tsx pattern) ───────────────────────

function formatAge(ts: number | string | undefined): string {
  if (ts == null || ts === '') return '-';
  const when = typeof ts === 'number' ? ts * 1000 : new Date(ts).getTime();
  if (!Number.isFinite(when) || when === 0) return '-';
  const diff = Date.now() - when;
  if (diff < 0) return '-';
  const mins = Math.floor(diff / 60000);
  if (mins < 60) return `${mins}m`;
  const hours = Math.floor(mins / 60);
  if (hours < 24) return `${hours}h`;
  const days = Math.floor(hours / 24);
  return `${days}d ${hours % 24}h`;
}

function formatDateTime(ts: string | undefined): string {
  if (!ts) return '-';
  const d = new Date(ts);
  if (isNaN(d.getTime())) return '-';
  const utc = d.toLocaleString('sv-SE', { timeZone: 'UTC' }).replace('T', ' ');
  const tw8 = d.toLocaleString('sv-SE', { timeZone: 'Asia/Taipei' }).replace('T', ' ');
  return `${utc} UTC / ${tw8} +8`;
}

function pnlColor(v: number): string {
  if (v > 0) return 'text-green-400';
  if (v < 0) return 'text-red-400';
  return 'text-gray-400';
}

function holdDuration(openedAt: string, closedAt?: string): string {
  const start = new Date(openedAt).getTime();
  const end = closedAt ? new Date(closedAt).getTime() : Date.now();
  if (!Number.isFinite(start) || !Number.isFinite(end) || end < start) return '-';
  const diff = end - start;
  const mins = Math.floor(diff / 60000);
  if (mins < 60) return `${mins}m`;
  const hours = Math.floor(mins / 60);
  if (hours < 24) return `${hours}h ${mins % 60}m`;
  const days = Math.floor(hours / 24);
  return `${days}d ${hours % 24}h`;
}

function replaceTokens(s: string, tokens: Record<string, string | number>): string {
  let out = s;
  for (const [k, v] of Object.entries(tokens)) {
    out = out.replace(new RegExp(`\\{${k}\\}`, 'g'), String(v));
  }
  return out;
}

// ─── WS subscription hook (page-local, mirrors useWebSocket pattern) ──────

function usePriceGapWebSocket(
  onPositions: (positions: PriceGapPosition[]) => void,
  onEvent: (evt: PriceGapEvent) => void,
  onCandidateUpdate: (upd: PriceGapCandidateUpdate) => void,
  enabled: boolean,
) {
  const [wsConnected, setWsConnected] = useState(true);

  useEffect(() => {
    if (!enabled) return;
    const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:';
    const token = localStorage.getItem('arb_token') || '';
    const ws = new WebSocket(`${protocol}//${location.host}/ws?token=${token}`);
    let cancelled = false;

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
            case 'pg_positions':
              onPositions((msg.data as PriceGapPosition[]) || []);
              break;
            case 'pg_event':
              onEvent(msg.data as PriceGapEvent);
              break;
            case 'pg_candidate_update':
              onCandidateUpdate(msg.data as PriceGapCandidateUpdate);
              break;
          }
        } catch {
          // ignore parse errors
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
  }, [enabled, onPositions, onEvent, onCandidateUpdate]);

  return wsConnected;
}

// ─── Auth helper ──────────────────────────────────────────────────────────

function authHeaders(): Record<string, string> {
  const token = localStorage.getItem('arb_token') || '';
  return {
    'Content-Type': 'application/json',
    Authorization: `Bearer ${token}`,
  };
}

// ─── Page component ───────────────────────────────────────────────────────

const PriceGap: FC = () => {
  const { t } = useLocale();

  const [loading, setLoading] = useState(true);
  const [seedError, setSeedError] = useState<string | null>(null);
  const [enabled, setEnabled] = useState(false);
  const [paperMode, setPaperMode] = useState(false);
  const [debugLog, setDebugLog] = useState(false); // Phase 9 gap-closure Gap #1 (Plan 09-09)
  const [budget, setBudget] = useState(0);
  const [candidates, setCandidates] = useState<PriceGapCandidate[]>([]);
  const [activePositions, setActivePositions] = useState<PriceGapPosition[]>([]);
  const [closedLog, setClosedLog] = useState<PriceGapPosition[]>([]);
  const [metrics, setMetrics] = useState<CandidateMetrics[]>([]);
  const [closedPage, setClosedPage] = useState(1); // 100 rows per page, cap 5
  const [closedLoading, setClosedLoading] = useState(false);
  const [metricsSortKey, setMetricsSortKey] = useState<keyof CandidateMetrics>('bps_30d_per_day');
  const [metricsSortDesc, setMetricsSortDesc] = useState(true);

  // Toggle in-flight error
  const [toggleError, setToggleError] = useState<string | null>(null);

  // Modal state
  const [disableTarget, setDisableTarget] = useState<PriceGapCandidate | null>(null);
  const [disableReason, setDisableReason] = useState('');
  const [reenableTarget, setReenableTarget] = useState<PriceGapCandidate | null>(null);
  const [modalBusy, setModalBusy] = useState(false);
  const [modalError, setModalError] = useState<string | null>(null);

  // Phase 10: Add/Edit modal + Delete confirm dialog state
  // (Phase-9 disable/reenable state above is UNCHANGED — D-13 invariant)
  type CandidateModalMode = 'add' | 'edit';
  const [editorOpen, setEditorOpen] = useState<{ mode: CandidateModalMode; target?: PriceGapCandidate } | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<PriceGapCandidate | null>(null);
  const [formSymbol, setFormSymbol] = useState('');
  const [formLongExch, setFormLongExch] = useState('binance');
  const [formShortExch, setFormShortExch] = useState('bybit');
  const [formThresholdBps, setFormThresholdBps] = useState(200);
  const [formMaxPositionUSDT, setFormMaxPositionUSDT] = useState(5000);
  const [formModeledSlippageBps, setFormModeledSlippageBps] = useState(5);
  const [formErrors, setFormErrors] = useState<Record<string, string>>({});

  // Flash highlight IDs (newly-entered rows)
  const [flashIds, setFlashIds] = useState<Set<string>>(new Set());

  const seed = useCallback(async () => {
    setSeedError(null);
    try {
      const res = await fetch('/api/pricegap/state', { headers: authHeaders() });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const body = (await res.json()) as { ok: boolean; data?: PriceGapState; error?: string };
      if (!body.ok || !body.data) throw new Error(body.error || 'seed failed');
      const d = body.data;
      setEnabled(d.enabled);
      setPaperMode(d.paper_mode);
      setDebugLog(d.debug_log || false);
      setBudget(d.budget || 0);
      setCandidates(d.candidates || []);
      setActivePositions(d.active_positions || []);
      setClosedLog(d.recent_closed || []);
      setMetrics(d.metrics || []);
    } catch (err) {
      setSeedError(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void seed();
  }, [seed]);

  // WS handlers
  const onPositions = useCallback((positions: PriceGapPosition[]) => {
    setActivePositions(positions);
  }, []);

  const onEvent = useCallback((evt: PriceGapEvent) => {
    if (evt.type === 'exit' && evt.position) {
      setClosedLog((prev) => {
        // Prepend (newest first) and cap at 500 absolute
        const next = [evt.position!, ...prev];
        return next.slice(0, 500);
      });
    } else if (evt.type === 'entry' && evt.position) {
      const id = evt.position.id;
      setFlashIds((prev) => new Set(prev).add(id));
      setTimeout(() => {
        setFlashIds((prev) => {
          const n = new Set(prev);
          n.delete(id);
          return n;
        });
      }, 800);
    } else if (evt.type === 'auto_disable' && evt.symbol) {
      setCandidates((prev) =>
        prev.map((c) =>
          c.symbol === evt.symbol
            ? { ...c, disabled: true, reason: evt.reason, disabled_at: Math.floor(Date.now() / 1000) }
            : c,
        ),
      );
    }
  }, []);

  const onCandidateUpdate = useCallback((upd: PriceGapCandidateUpdate) => {
    setCandidates((prev) =>
      prev.map((c) =>
        c.symbol === upd.symbol
          ? { ...c, disabled: upd.disabled, reason: upd.reason, disabled_at: upd.disabled_at }
          : c,
      ),
    );
  }, []);

  const wsConnected = usePriceGapWebSocket(onPositions, onEvent, onCandidateUpdate, !loading);

  // ── Toggle handlers ─────────────────────────────────────────────────────

  const postConfig = useCallback(async (body: Record<string, unknown>): Promise<void> => {
    const res = await fetch('/api/config', {
      method: 'POST',
      headers: authHeaders(),
      body: JSON.stringify(body),
    });
    const payload = (await res.json().catch(() => null)) as { ok?: boolean; error?: string } | null;
    if (!res.ok || !payload?.ok) {
      throw new Error(payload?.error || `HTTP ${res.status}`);
    }
  }, []);

  const toggleMaster = useCallback(async () => {
    const next = !enabled;
    setEnabled(next);
    setToggleError(null);
    try {
      await postConfig({ price_gap: { enabled: next } });
    } catch (err) {
      setEnabled(!next);
      setToggleError(replaceTokens(t('pricegap.err.toggleFailed'), {
        error: err instanceof Error ? err.message : String(err),
      }));
    }
  }, [enabled, postConfig, t]);

  const togglePaper = useCallback(async () => {
    const next = !paperMode;
    setPaperMode(next);
    setToggleError(null);
    try {
      await postConfig({ price_gap_paper_mode: next });
    } catch (err) {
      setPaperMode(!next);
      setToggleError(replaceTokens(t('pricegap.err.toggleFailed'), {
        error: err instanceof Error ? err.message : String(err),
      }));
    }
  }, [paperMode, postConfig, t]);

  // Phase 9 gap-closure Gap #1 (Plan 09-09): debug-log toggle.
  // POSTs the nested price_gap.debug_log form — the flat shortcut used for
  // paper_mode is intentionally not introduced here; debug_log is the only
  // price-gap flag added post-Phase-9 that the handler accepts nested only.
  const toggleDebugLog = useCallback(async () => {
    const next = !debugLog;
    setDebugLog(next);
    setToggleError(null);
    try {
      await postConfig({ price_gap: { debug_log: next } });
    } catch (err) {
      setDebugLog(!next);
      setToggleError(replaceTokens(t('pricegap.err.toggleFailed'), {
        error: err instanceof Error ? err.message : String(err),
      }));
    }
  }, [debugLog, postConfig, t]);

  // ── Candidate disable/enable ────────────────────────────────────────────

  const confirmDisable = useCallback(async () => {
    if (!disableTarget) return;
    setModalBusy(true);
    setModalError(null);
    try {
      const res = await fetch(
        `/api/pricegap/candidate/${encodeURIComponent(disableTarget.symbol)}/disable`,
        {
          method: 'POST',
          headers: authHeaders(),
          body: JSON.stringify({ reason: disableReason.trim() || 'manual' }),
        },
      );
      const body = (await res.json().catch(() => null)) as { ok?: boolean; error?: string } | null;
      if (!res.ok || !body?.ok) throw new Error(body?.error || `HTTP ${res.status}`);
      // Optimistic local update (WS will confirm)
      setCandidates((prev) =>
        prev.map((c) =>
          c.symbol === disableTarget.symbol
            ? {
                ...c,
                disabled: true,
                reason: disableReason.trim() || 'manual',
                disabled_at: Math.floor(Date.now() / 1000),
              }
            : c,
        ),
      );
      setDisableTarget(null);
      setDisableReason('');
    } catch (err) {
      setModalError(err instanceof Error ? err.message : String(err));
    } finally {
      setModalBusy(false);
    }
  }, [disableTarget, disableReason]);

  const confirmReenable = useCallback(async () => {
    if (!reenableTarget) return;
    setModalBusy(true);
    setModalError(null);
    try {
      const res = await fetch(
        `/api/pricegap/candidate/${encodeURIComponent(reenableTarget.symbol)}/enable`,
        { method: 'POST', headers: authHeaders() },
      );
      const body = (await res.json().catch(() => null)) as { ok?: boolean; error?: string } | null;
      if (!res.ok || !body?.ok) throw new Error(body?.error || `HTTP ${res.status}`);
      setCandidates((prev) =>
        prev.map((c) =>
          c.symbol === reenableTarget.symbol ? { ...c, disabled: false, reason: undefined, disabled_at: undefined } : c,
        ),
      );
      setReenableTarget(null);
    } catch (err) {
      setModalError(err instanceof Error ? err.message : String(err));
    } finally {
      setModalBusy(false);
    }
  }, [reenableTarget]);

  // ── Phase 10: Add/Edit/Delete handlers ─────────────────────────────────

  const openEditor = useCallback((mode: CandidateModalMode, target?: PriceGapCandidate) => {
    setFormErrors({});
    setModalError(null);
    if (mode === 'edit' && target) {
      setFormSymbol(target.symbol);
      setFormLongExch(target.long_exch);
      setFormShortExch(target.short_exch);
      setFormThresholdBps(target.threshold_bps);
      setFormMaxPositionUSDT(target.max_position_usdt);
      setFormModeledSlippageBps(target.modeled_slippage_bps ?? 5);
    } else {
      setFormSymbol('');
      setFormLongExch('binance');
      setFormShortExch('bybit');
      setFormThresholdBps(200);
      setFormMaxPositionUSDT(5000);
      setFormModeledSlippageBps(5);
    }
    setEditorOpen({ mode, target });
  }, []);

  // Local on-submit validation (D-03 + D-10 defense in depth — backend re-validates).
  const validateLocalForm = useCallback((): Record<string, string> => {
    const errs: Record<string, string> = {};
    const sym = formSymbol.trim().toUpperCase();
    if (!/^[A-Z0-9]+USDT$/.test(sym)) errs.symbol = t('pricegap.candidates.errors.symbolFormat');
    if (formLongExch === formShortExch) errs.exchanges = t('pricegap.candidates.errors.exchangesEqual');
    if (formThresholdBps < 50 || formThresholdBps > 1000) errs.threshold = t('pricegap.candidates.errors.thresholdRange');
    if (formMaxPositionUSDT < 100 || formMaxPositionUSDT > 50000) errs.maxPosition = t('pricegap.candidates.errors.maxPositionRange');
    if (formModeledSlippageBps < 0 || formModeledSlippageBps > 100) errs.slippage = t('pricegap.candidates.errors.slippageRange');
    // Tuple collision (D-11): Add → reject any existing tuple match; Edit → reject if changed tuple matches a *different* row.
    const tuple = `${sym}|${formLongExch}|${formShortExch}`;
    const collides = candidates.some((c) => {
      const k = `${c.symbol}|${c.long_exch}|${c.short_exch}`;
      if (editorOpen?.mode === 'edit' && editorOpen.target) {
        const oldKey = `${editorOpen.target.symbol}|${editorOpen.target.long_exch}|${editorOpen.target.short_exch}`;
        return k === tuple && k !== oldKey;
      }
      return k === tuple;
    });
    if (collides) errs.tuple = t('pricegap.candidates.errors.tupleCollision');
    return errs;
  }, [formSymbol, formLongExch, formShortExch, formThresholdBps, formMaxPositionUSDT, formModeledSlippageBps, candidates, editorOpen, t]);

  // Close modal on Esc — Phase 10 extends Phase-9 handler to cover editorOpen + deleteTarget
  useEffect(() => {
    if (!disableTarget && !reenableTarget && !editorOpen && !deleteTarget) return;
    const h = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        setDisableTarget(null);
        setReenableTarget(null);
        setEditorOpen(null);
        setDeleteTarget(null);
        setModalError(null);
        setFormErrors({});
      }
    };
    window.addEventListener('keydown', h);
    return () => window.removeEventListener('keydown', h);
  }, [disableTarget, reenableTarget, editorOpen, deleteTarget]);

  // Save handler for Add/Edit modal — POSTs full replacement candidates array
  // (D-16 last-write-wins). On 400/409, surfaces server error; on 409 active-position,
  // maps to friendly i18n string per D-14.
  const handleEditorSave = useCallback(async () => {
    setModalBusy(true);
    setModalError(null);
    const errs = validateLocalForm();
    setFormErrors(errs);
    if (Object.keys(errs).length > 0) {
      setModalBusy(false);
      return;
    }
    const draft: PriceGapCandidate = {
      symbol: formSymbol.trim().toUpperCase(),
      long_exch: formLongExch,
      short_exch: formShortExch,
      threshold_bps: formThresholdBps,
      max_position_usdt: formMaxPositionUSDT,
      modeled_slippage_bps: formModeledSlippageBps,
      // Phase-9 disable state lives in Redis keyed by symbol — preserve in-memory
      // mirror so the table re-render shows correct status until WS confirms.
      disabled: editorOpen?.target?.disabled ?? false,
      reason: editorOpen?.target?.reason,
      disabled_at: editorOpen?.target?.disabled_at,
    };
    const next: PriceGapCandidate[] = editorOpen?.mode === 'add'
      ? [...candidates, draft]
      : candidates.map((c) =>
          editorOpen?.target &&
          c.symbol === editorOpen.target.symbol &&
          c.long_exch === editorOpen.target.long_exch &&
          c.short_exch === editorOpen.target.short_exch
            ? draft
            : c,
        );
    try {
      await postConfig({ price_gap: { candidates: next } });
      await seed(); // re-fetch /api/pricegap/state for server-canonical view
      setEditorOpen(null);
      setFormErrors({});
    } catch (err) {
      setModalError(err instanceof Error ? err.message : String(err));
    } finally {
      setModalBusy(false);
    }
  }, [candidates, editorOpen, formSymbol, formLongExch, formShortExch, formThresholdBps, formMaxPositionUSDT, formModeledSlippageBps, postConfig, seed, validateLocalForm]);

  // Delete handler — POSTs candidates filtered to exclude deleteTarget tuple.
  // On 409 with "active position" wording, surface friendly D-14 copy.
  const handleConfirmDelete = useCallback(async () => {
    if (!deleteTarget) return;
    setModalBusy(true);
    setModalError(null);
    const next = candidates.filter(
      (c) =>
        !(
          c.symbol === deleteTarget.symbol &&
          c.long_exch === deleteTarget.long_exch &&
          c.short_exch === deleteTarget.short_exch
        ),
    );
    try {
      await postConfig({ price_gap: { candidates: next } });
      await seed();
      setDeleteTarget(null);
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err);
      // D-14: backend returns 409 with "...has active position; close it first..." — map to friendly i18n.
      if (msg.toLowerCase().includes('active position')) {
        setModalError(t('pricegap.candidates.errors.activePositionBlocksDelete'));
      } else {
        setModalError(msg);
      }
    } finally {
      setModalBusy(false);
    }
  }, [candidates, deleteTarget, postConfig, seed, t]);

  // ── Closed-log pagination ───────────────────────────────────────────────

  const loadMoreClosed = useCallback(async () => {
    if (closedLoading) return;
    setClosedLoading(true);
    try {
      const offset = closedPage * 100;
      if (offset >= 500) return; // cap 500 rows
      const res = await fetch(`/api/pricegap/closed?offset=${offset}&limit=100`, {
        headers: authHeaders(),
      });
      const body = (await res.json().catch(() => null)) as
        | { ok?: boolean; data?: PriceGapPosition[]; error?: string }
        | null;
      if (res.ok && body?.ok && Array.isArray(body.data)) {
        setClosedLog((prev) => {
          const seen = new Set(prev.map((p) => p.id));
          const next = [...prev];
          for (const row of body.data!) {
            if (!seen.has(row.id)) next.push(row);
          }
          return next.slice(0, 500);
        });
        setClosedPage((p) => p + 1);
      }
    } finally {
      setClosedLoading(false);
    }
  }, [closedLoading, closedPage]);

  // ── Sorted metrics ──────────────────────────────────────────────────────

  const sortedMetrics = useMemo(() => {
    const arr = [...metrics];
    arr.sort((a, b) => {
      const av = a[metricsSortKey];
      const bv = b[metricsSortKey];
      if (typeof av === 'number' && typeof bv === 'number') {
        return metricsSortDesc ? bv - av : av - bv;
      }
      return metricsSortDesc
        ? String(bv).localeCompare(String(av))
        : String(av).localeCompare(String(bv));
    });
    return arr;
  }, [metrics, metricsSortKey, metricsSortDesc]);

  const toggleMetricsSort = (key: keyof CandidateMetrics) => {
    if (key === metricsSortKey) setMetricsSortDesc((d) => !d);
    else {
      setMetricsSortKey(key);
      setMetricsSortDesc(true);
    }
  };

  // ── Rendering ───────────────────────────────────────────────────────────

  const panelCls = 'bg-gray-900 border border-gray-800 rounded-lg p-4 overflow-x-auto';
  const sectionTitle = 'text-sm font-bold text-gray-300 uppercase tracking-wide mb-3';
  const skeletonRows = (cols: number) => (
    <>
      {[0, 1, 2].map((i) => (
        <tr key={i}>
          {Array.from({ length: cols }).map((_, j) => (
            <td key={j} className="py-2 pr-4">
              <div className="h-4 bg-gray-800 animate-pulse rounded" />
            </td>
          ))}
        </tr>
      ))}
    </>
  );

  // Header stat strip
  const openCount = activePositions.length;
  const budgetUsed = activePositions.reduce((s, p) => s + (p.notional_usdt || 0), 0);

  return (
    <div className="space-y-8">
      {/* WS disconnect banner */}
      {!wsConnected && !loading && (
        <div className="bg-yellow-500/10 text-yellow-400 text-xs px-4 py-2 rounded">
          {t('pricegap.warn.wsDisconnected')}
        </div>
      )}

      {/* Section 1: Header / Status bar */}
      <div className={panelCls}>
        <div className="flex flex-wrap items-center justify-between gap-4">
          <h2 className="text-xl font-bold text-gray-100">{t('pricegap.pageTitle')}</h2>
          <div className="flex items-center gap-6">
            <div className="text-xs text-gray-400 font-mono tabular-nums">
              {replaceTokens(t('pricegap.openCount'), { n: openCount })}
              <span className="mx-2 text-gray-600">·</span>
              {replaceTokens(t('pricegap.budgetStat'), {
                used: budgetUsed.toFixed(0),
                total: budget.toFixed(0),
              })}
            </div>
            <label
              className="flex items-center gap-2 text-xs text-gray-300 cursor-pointer"
              title={t('pricegap.masterEnableTooltip')}
            >
              <input
                type="checkbox"
                checked={enabled}
                onChange={toggleMaster}
                aria-label={t('pricegap.masterEnable')}
                className="h-3 w-3"
              />
              <span>{t('pricegap.masterEnable')}</span>
            </label>
            <label
              className={`flex items-center gap-2 text-xs cursor-pointer px-2 py-1 rounded ${
                paperMode ? 'bg-violet-500/15 text-violet-400' : 'text-gray-300'
              }`}
              title={t('pricegap.paperModeTooltip')}
            >
              <input
                type="checkbox"
                checked={paperMode}
                onChange={togglePaper}
                aria-label={t('pricegap.paperMode')}
                className="h-3 w-3"
              />
              <span className="font-bold">{t('pricegap.paperMode')}</span>
              <span
                className={`text-[10px] font-bold ${
                  paperMode ? 'text-violet-400' : 'text-green-400'
                }`}
              >
                {paperMode ? t('pricegap.paperOn') : t('pricegap.liveOn')}
              </span>
            </label>
            {/* Phase 9 gap-closure Gap #1 (Plan 09-09) — debug-log toggle.
                Amber tint when ON signals "diagnostic logging active". */}
            <label
              className={`flex items-center gap-2 text-xs cursor-pointer px-2 py-1 rounded ${
                debugLog ? 'bg-amber-500/15 text-amber-400' : 'text-gray-300'
              }`}
              title={t('pricegap.debugLogTooltip')}
            >
              <input
                type="checkbox"
                checked={debugLog}
                onChange={toggleDebugLog}
                aria-label={t('pricegap.debugLog')}
                className="h-3 w-3"
              />
              <span className="font-bold">{t('pricegap.debugLog')}</span>
            </label>
          </div>
        </div>
        {toggleError && (
          <div className="mt-2 text-red-400 text-xs">{toggleError}</div>
        )}
      </div>

      {/* Section 2: Candidates */}
      <div className={panelCls}>
        <div className="flex items-center justify-between mb-2">
          <h3 className={sectionTitle}>{t('pricegap.section.candidates')}</h3>
          {/* Phase 10 D-04: Add candidate button, right-aligned above table */}
          <button
            type="button"
            className="px-3 py-1.5 bg-blue-600 hover:bg-blue-500 text-white rounded text-sm"
            onClick={() => openEditor('add')}
          >
            {t('pricegap.candidates.add.button')}
          </button>
        </div>
        {seedError ? (
          <div className="text-red-400 text-xs p-4">
            {replaceTokens(t('pricegap.err.seedFailed'), {
              section: t('pricegap.section.candidates'),
            })}{' '}
            <button
              onClick={() => void seed()}
              className="underline hover:text-red-300"
            >
              {t('pricegap.action.retry')}
            </button>
          </div>
        ) : (
          <table className="w-full text-sm">
            <thead>
              <tr className="text-gray-400 text-left border-b border-gray-800">
                <th className="pb-2 pr-4">{t('pricegap.col.symbol')}</th>
                <th className="pb-2 pr-4">{t('pricegap.col.longExch')}</th>
                <th className="pb-2 pr-4">{t('pricegap.col.shortExch')}</th>
                <th className="pb-2 pr-4 text-right">{t('pricegap.col.thresholdBps')}</th>
                <th className="pb-2 pr-4 text-right">{t('pricegap.col.maxNotional')}</th>
                <th className="pb-2 pr-4 text-center">{t('pricegap.col.status')}</th>
                <th className="pb-2 pr-4">{t('pricegap.col.disabledInfo')}</th>
                <th className="pb-2 pr-4 text-right"></th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-800">
              {loading && skeletonRows(8)}
              {!loading &&
                candidates
                  .slice()
                  .sort((a, b) => a.symbol.localeCompare(b.symbol))
                  .map((c) => (
                    <tr
                      key={c.symbol}
                      className={
                        c.disabled ? 'bg-yellow-500/5 opacity-60' : 'text-gray-100'
                      }
                    >
                      <td className="py-2 pr-4 font-mono">{c.symbol}</td>
                      <td className="py-2 pr-4 text-green-400">{c.long_exch}</td>
                      <td className="py-2 pr-4 text-red-400">{c.short_exch}</td>
                      <td className="py-2 pr-4 text-right font-mono tabular-nums">
                        {c.threshold_bps.toFixed(1)}
                      </td>
                      <td className="py-2 pr-4 text-right font-mono tabular-nums">
                        ${c.max_position_usdt.toFixed(0)}
                      </td>
                      <td className="py-2 pr-4 text-center">
                        {c.disabled ? (
                          <span className="text-yellow-400">{t('pricegap.status.disabled')}</span>
                        ) : (
                          <span className="text-green-400">{t('pricegap.status.active')}</span>
                        )}
                      </td>
                      <td className="py-2 pr-4 text-xs text-gray-400">
                        {c.disabled && c.reason
                          ? `${c.reason} · ${formatAge(c.disabled_at)} ago`
                          : '-'}
                      </td>
                      <td className="py-2 pr-4 text-right">
                        <div className="inline-flex gap-1">
                          {c.disabled ? (
                            <button
                              onClick={() => setReenableTarget(c)}
                              className="bg-yellow-600/20 text-yellow-400 hover:bg-yellow-600/40 px-2 py-1 text-xs rounded"
                            >
                              {t('pricegap.action.reenable')}
                            </button>
                          ) : (
                            <button
                              onClick={() => {
                                setDisableReason('');
                                setDisableTarget(c);
                              }}
                              className="bg-gray-700/40 text-gray-300 hover:bg-gray-700/60 text-xs rounded px-2 py-1"
                            >
                              {t('pricegap.action.disable')}
                            </button>
                          )}
                          {/* Phase 10: Edit/Delete buttons (Phase-9 Disable/Re-enable above unchanged) */}
                          <button
                            type="button"
                            onClick={() => openEditor('edit', c)}
                            className="bg-blue-600/20 text-blue-300 hover:bg-blue-600/40 px-2 py-1 text-xs rounded"
                          >
                            {t('pricegap.candidates.row.edit')}
                          </button>
                          <button
                            type="button"
                            onClick={() => setDeleteTarget(c)}
                            className="bg-red-600/20 text-red-300 hover:bg-red-600/40 px-2 py-1 text-xs rounded"
                          >
                            {t('pricegap.candidates.row.delete')}
                          </button>
                        </div>
                      </td>
                    </tr>
                  ))}
              {!loading && candidates.length === 0 && (
                <tr>
                  <td colSpan={8} className="py-4 text-center text-gray-500 text-xs">
                    {t('pricegap.empty.candidates')}
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        )}
      </div>

      {/* Section 3: Live Positions */}
      <div className={panelCls}>
        <h3 className={sectionTitle}>{t('pricegap.section.livePositions')}</h3>
        <table className="w-full text-sm">
          <thead>
            <tr className="text-gray-400 text-left border-b border-gray-800">
              <th className="pb-2 pr-4">{t('pricegap.col.symbol')}</th>
              <th className="pb-2 pr-4">{t('pricegap.col.longExch')}</th>
              <th className="pb-2 pr-4">{t('pricegap.col.shortExch')}</th>
              <th className="pb-2 pr-4 text-right">{t('pricegap.col.size')}</th>
              <th className="pb-2 pr-4 text-right">{t('pricegap.col.entryBps')}</th>
              <th className="pb-2 pr-4 text-right">{t('pricegap.col.currentBps')}</th>
              <th className="pb-2 pr-4 text-right">{t('pricegap.col.hold')}</th>
              <th className="pb-2 pr-4 text-right">{t('pricegap.col.pnl')}</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-800">
            {loading && skeletonRows(8)}
            {!loading &&
              activePositions
                .slice()
                .sort((a, b) => new Date(b.opened_at).getTime() - new Date(a.opened_at).getTime())
                .map((p) => {
                  const isPaper = p.mode === 'paper';
                  const flashing = flashIds.has(p.id);
                  const rowBase = isPaper ? 'bg-violet-500/5' : '';
                  const rowCls = flashing ? 'bg-green-500/10' : rowBase;
                  const pnlVal = p.current_pnl ?? 0;
                  const pnlBps =
                    p.notional_usdt > 0 ? (pnlVal / p.notional_usdt) * 10000 : 0;
                  return (
                    <tr key={p.id} className={`text-gray-100 ${rowCls}`}>
                      <td className="py-2 pr-4 font-mono">
                        {p.symbol}{' '}
                        <span
                          className={`ml-1 text-[10px] font-bold ${
                            isPaper ? 'text-violet-400' : 'text-green-400'
                          }`}
                        >
                          {isPaper ? t('pricegap.paperOn') : t('pricegap.liveOn')}
                        </span>
                      </td>
                      <td className="py-2 pr-4 text-green-400">{p.long_exchange}</td>
                      <td className="py-2 pr-4 text-red-400">{p.short_exchange}</td>
                      <td className="py-2 pr-4 text-right font-mono tabular-nums">
                        ${p.notional_usdt.toFixed(0)}
                      </td>
                      <td className="py-2 pr-4 text-right font-mono tabular-nums">
                        {p.entry_spread_bps.toFixed(1)}
                      </td>
                      <td
                        className={`py-2 pr-4 text-right font-mono tabular-nums ${
                          p.current_spread_bps == null
                            ? 'text-gray-400'
                            : p.current_spread_bps > p.entry_spread_bps
                              ? 'text-red-400'
                              : 'text-green-400'
                        }`}
                      >
                        {p.current_spread_bps != null ? p.current_spread_bps.toFixed(1) : '-'}
                      </td>
                      <td className="py-2 pr-4 text-right font-mono text-gray-400 tabular-nums">
                        {formatAge(p.opened_at)}
                      </td>
                      <td className={`py-2 pr-4 text-right font-mono tabular-nums ${pnlColor(pnlVal)}`}>
                        ${pnlVal.toFixed(2)}
                        <div className="text-[10px] text-gray-500">
                          {pnlBps >= 0 ? '+' : ''}
                          {pnlBps.toFixed(1)} bps
                        </div>
                      </td>
                    </tr>
                  );
                })}
            {!loading && activePositions.length === 0 && (
              <tr>
                <td colSpan={8} className="py-4 text-center text-gray-500 text-xs">
                  {t('pricegap.empty.livePositions')}
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>

      {/* Section 4: Closed Log */}
      <div className={panelCls}>
        <h3 className={sectionTitle}>{t('pricegap.section.closedLog')}</h3>
        <table className="w-full text-sm">
          <thead>
            <tr className="text-gray-400 text-left border-b border-gray-800">
              <th className="pb-2 pr-4">{t('pricegap.col.closedAt')}</th>
              <th className="pb-2 pr-4">{t('pricegap.col.symbol')}</th>
              <th className="pb-2 pr-4">{t('pricegap.col.legs')}</th>
              <th className="pb-2 pr-4 text-right">{t('pricegap.col.size')}</th>
              <th className="pb-2 pr-4 text-right">{t('pricegap.col.entryBps')}</th>
              <th className="pb-2 pr-4 text-right">{t('pricegap.col.exitBps')}</th>
              <th className="pb-2 pr-4 text-right">{t('pricegap.col.holdDur')}</th>
              <th className="pb-2 pr-4">{t('pricegap.col.reason')}</th>
              <th className="pb-2 pr-4 text-right">{t('pricegap.col.realizedBps')}</th>
              <th className="pb-2 pr-4 text-right">{t('pricegap.col.modeledBps')}</th>
              <th className="pb-2 pr-4 text-right">{t('pricegap.col.deltaBps')}</th>
              <th className="pb-2 pr-4 text-right">{t('pricegap.col.pnlUsdt')}</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-800">
            {loading && skeletonRows(12)}
            {!loading &&
              closedLog
                .slice()
                .sort((a, b) => {
                  // Pinned close-time desc
                  const at = a.closed_at ? new Date(a.closed_at).getTime() : 0;
                  const bt = b.closed_at ? new Date(b.closed_at).getTime() : 0;
                  return bt - at;
                })
                .slice(0, closedPage * 100)
                .map((p) => {
                  const isPaper = p.mode === 'paper';
                  const realized = p.realized_slippage_bps;
                  const modeled = p.modeled_slippage_bps;
                  const delta = realized - modeled;
                  const realizedCls = realized > modeled ? 'text-red-400' : 'text-green-400';
                  return (
                    <tr
                      key={p.id}
                      className={`text-gray-100 ${isPaper ? 'bg-violet-500/5' : ''}`}
                    >
                      <td className="py-2 pr-4 text-xs text-gray-400 font-mono">
                        {formatDateTime(p.closed_at)}
                      </td>
                      <td className="py-2 pr-4 font-mono">
                        {p.symbol}{' '}
                        <span
                          className={`ml-1 text-[10px] font-bold ${
                            isPaper ? 'text-violet-400' : 'text-green-400'
                          }`}
                        >
                          {isPaper ? t('pricegap.paperOn') : t('pricegap.liveOn')}
                        </span>
                      </td>
                      <td className="py-2 pr-4 text-xs">
                        <span className="text-green-400">{p.long_exchange}</span>
                        <span className="text-gray-500"> / </span>
                        <span className="text-red-400">{p.short_exchange}</span>
                      </td>
                      <td className="py-2 pr-4 text-right font-mono tabular-nums">
                        ${p.notional_usdt.toFixed(0)}
                      </td>
                      <td className="py-2 pr-4 text-right font-mono tabular-nums">
                        {p.entry_spread_bps.toFixed(1)}
                      </td>
                      <td className="py-2 pr-4 text-right font-mono tabular-nums">
                        {p.current_spread_bps != null ? p.current_spread_bps.toFixed(1) : '-'}
                      </td>
                      <td className="py-2 pr-4 text-right font-mono text-gray-400 tabular-nums">
                        {holdDuration(p.opened_at, p.closed_at)}
                      </td>
                      <td className="py-2 pr-4 text-xs text-gray-400">{p.exit_reason || '-'}</td>
                      <td className={`py-2 pr-4 text-right font-mono tabular-nums ${realizedCls}`}>
                        {realized.toFixed(1)}
                      </td>
                      <td className="py-2 pr-4 text-right font-mono tabular-nums text-gray-400">
                        {modeled.toFixed(1)}
                      </td>
                      <td
                        className={`py-2 pr-4 text-right font-mono tabular-nums ${pnlColor(-delta)}`}
                      >
                        {delta >= 0 ? '+' : ''}
                        {delta.toFixed(1)}
                      </td>
                      <td
                        className={`py-2 pr-4 text-right font-mono tabular-nums ${pnlColor(p.realized_pnl)}`}
                      >
                        ${p.realized_pnl.toFixed(2)}
                      </td>
                    </tr>
                  );
                })}
            {!loading && closedLog.length === 0 && (
              <tr>
                <td colSpan={12} className="py-4 text-center text-gray-500 text-xs">
                  {t('pricegap.empty.closedLog')}
                </td>
              </tr>
            )}
          </tbody>
        </table>
        {!loading && closedLog.length >= closedPage * 100 && closedPage * 100 < 500 && (
          <div className="mt-3 text-center">
            <button
              onClick={() => void loadMoreClosed()}
              disabled={closedLoading}
              className="text-xs bg-gray-700/40 text-gray-300 hover:bg-gray-700/60 rounded px-3 py-1 disabled:opacity-50"
            >
              {t('pricegap.action.loadMore')}
            </button>
          </div>
        )}
      </div>

      {/* Section 5: Rolling Metrics */}
      <div className={panelCls}>
        <h3 className={sectionTitle}>{t('pricegap.section.metrics')}</h3>
        <table className="w-full text-sm">
          <thead>
            <tr className="text-gray-400 text-left border-b border-gray-800">
              {([
                ['candidate', t('pricegap.col.candidate'), 'left'],
                ['trades_window', t('pricegap.col.trades'), 'right'],
                ['win_pct', t('pricegap.col.winPct'), 'right'],
                ['avg_realized_bps', t('pricegap.col.avgRealized'), 'right'],
                ['bps_24h_per_day', t('pricegap.col.bps24h'), 'right'],
                ['bps_7d_per_day', t('pricegap.col.bps7d'), 'right'],
                ['bps_30d_per_day', t('pricegap.col.bps30d'), 'right'],
              ] as Array<[keyof CandidateMetrics, string, 'left' | 'right']>).map(
                ([key, label, align]) => (
                  <th
                    key={String(key)}
                    className={`pb-2 pr-4 cursor-pointer hover:text-gray-200 ${
                      align === 'right' ? 'text-right' : ''
                    }`}
                    onClick={() => toggleMetricsSort(key)}
                  >
                    {label}
                    {metricsSortKey === key && (
                      <span className="ml-1 text-[10px]">
                        {metricsSortDesc ? '▼' : '▲'}
                      </span>
                    )}
                  </th>
                ),
              )}
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-800">
            {loading && skeletonRows(7)}
            {!loading &&
              sortedMetrics.map((m) => (
                <tr key={m.candidate} className="text-gray-100">
                  <td className="py-2 pr-4 font-mono text-xs">
                    {m.symbol}{' '}
                    <span className="text-green-400">{m.long_exchange}</span>
                    <span className="text-gray-500"> / </span>
                    <span className="text-red-400">{m.short_exchange}</span>
                  </td>
                  <td className="py-2 pr-4 text-right font-mono tabular-nums">
                    {m.trades_window || '-'}
                  </td>
                  <td className="py-2 pr-4 text-right font-mono tabular-nums">
                    {m.trades_window ? `${m.win_pct.toFixed(1)}%` : '-'}
                  </td>
                  <td className="py-2 pr-4 text-right font-mono tabular-nums">
                    {m.trades_window ? m.avg_realized_bps.toFixed(1) : '-'}
                  </td>
                  <td className="py-2 pr-4 text-right font-mono tabular-nums">
                    {m.trades_window ? m.bps_24h_per_day.toFixed(1) : '-'}
                  </td>
                  <td className="py-2 pr-4 text-right font-mono tabular-nums">
                    {m.trades_window ? m.bps_7d_per_day.toFixed(1) : '-'}
                  </td>
                  <td className="py-2 pr-4 text-right font-mono tabular-nums">
                    {m.trades_window ? m.bps_30d_per_day.toFixed(1) : '-'}
                  </td>
                </tr>
              ))}
            {!loading && sortedMetrics.length === 0 && (
              <tr>
                <td colSpan={7} className="py-4 text-center text-gray-500 text-xs">
                  {t('pricegap.empty.metrics')}
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>

      {/* Disable modal */}
      {disableTarget && (
        <div
          className="fixed inset-0 bg-black/50 flex items-center justify-center z-50"
          onClick={() => {
            if (!modalBusy) {
              setDisableTarget(null);
              setModalError(null);
            }
          }}
        >
          <div
            role="dialog"
            aria-labelledby="pg-disable-title"
            aria-describedby="pg-disable-body"
            className="bg-gray-900 border border-gray-700 rounded-lg p-6 max-w-sm"
            onClick={(e) => e.stopPropagation()}
          >
            <h3 id="pg-disable-title" className="text-lg font-bold text-gray-100 mb-2">
              {replaceTokens(t('pricegap.modal.disableTitle'), { symbol: disableTarget.symbol })}
            </h3>
            <p id="pg-disable-body" className="text-gray-300 text-sm mb-4">
              {t('pricegap.modal.disableBody')}
            </p>
            <label className="block mb-3">
              <span className="block text-xs text-gray-400 mb-1">
                {t('pricegap.modal.disableReasonLabel')}
              </span>
              <input
                type="text"
                value={disableReason}
                onChange={(e) => setDisableReason(e.target.value)}
                placeholder={t('pricegap.modal.disableReasonPlaceholder')}
                maxLength={256}
                className="w-full bg-gray-800 border border-gray-700 rounded px-2 py-1 text-sm text-gray-100"
              />
            </label>
            {modalError && <p className="text-red-400 text-xs mb-3">{modalError}</p>}
            <div className="flex gap-3">
              <button
                onClick={() => void confirmDisable()}
                disabled={modalBusy}
                className="px-4 py-2 text-sm bg-red-600 text-white rounded hover:bg-red-700 disabled:opacity-50"
              >
                {t('pricegap.modal.disable.confirm')}
              </button>
              <button
                onClick={() => {
                  setDisableTarget(null);
                  setModalError(null);
                }}
                disabled={modalBusy}
                className="px-4 py-2 text-sm bg-gray-700 text-gray-300 rounded hover:bg-gray-600 disabled:opacity-50"
              >
                {t('pricegap.modal.disable.keepActive')}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Re-enable modal */}
      {reenableTarget && (
        <div
          className="fixed inset-0 bg-black/50 flex items-center justify-center z-50"
          onClick={() => {
            if (!modalBusy) {
              setReenableTarget(null);
              setModalError(null);
            }
          }}
        >
          <div
            role="dialog"
            aria-labelledby="pg-reenable-title"
            aria-describedby="pg-reenable-body"
            className="bg-gray-900 border border-gray-700 rounded-lg p-6 max-w-sm"
            onClick={(e) => e.stopPropagation()}
          >
            <h3 id="pg-reenable-title" className="text-lg font-bold text-gray-100 mb-2">
              {replaceTokens(t('pricegap.modal.reenableTitle'), {
                symbol: reenableTarget.symbol,
              })}
            </h3>
            <p id="pg-reenable-body" className="text-gray-300 text-sm mb-4">
              {replaceTokens(t('pricegap.modal.reenableBody'), {
                age: formatAge(reenableTarget.disabled_at),
                reason: reenableTarget.reason || 'manual',
              })}
              {reenableTarget.reason && reenableTarget.reason.startsWith('exec_quality') && (
                <span className="block mt-2 text-xs text-gray-400">
                  {t('pricegap.modal.reenableExecQuality')}
                </span>
              )}
            </p>
            {modalError && <p className="text-red-400 text-xs mb-3">{modalError}</p>}
            <div className="flex gap-3">
              <button
                onClick={() => void confirmReenable()}
                disabled={modalBusy}
                className="px-4 py-2 text-sm bg-red-600 text-white rounded hover:bg-red-700 disabled:opacity-50"
              >
                {t('pricegap.modal.reEnable.confirm')}
              </button>
              <button
                onClick={() => {
                  setReenableTarget(null);
                  setModalError(null);
                }}
                disabled={modalBusy}
                className="px-4 py-2 text-sm bg-gray-700 text-gray-300 rounded hover:bg-gray-600 disabled:opacity-50"
              >
                {t('pricegap.modal.reEnable.keepDisabled')}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Phase 10: Add/Edit modal (mirrors Phase-9 overlay pattern) */}
      {editorOpen && (
        <div
          className="fixed inset-0 bg-black/50 flex items-center justify-center z-50"
          onClick={() => {
            if (!modalBusy) {
              setEditorOpen(null);
              setModalError(null);
              setFormErrors({});
            }
          }}
        >
          <div
            role="dialog"
            aria-labelledby="pg-editor-title"
            className="bg-gray-800 border border-gray-700 rounded-lg shadow-xl w-full max-w-md p-5"
            onClick={(e) => e.stopPropagation()}
          >
            <h3 id="pg-editor-title" className="text-lg font-semibold text-white mb-3">
              {editorOpen.mode === 'add'
                ? t('pricegap.candidates.modal.add.title')
                : replaceTokens(t('pricegap.candidates.modal.edit.title'), {
                    symbol: editorOpen.target?.symbol ?? '',
                    long: editorOpen.target?.long_exch ?? '',
                    short: editorOpen.target?.short_exch ?? '',
                  })}
            </h3>

            {/* Symbol */}
            <label className="block text-sm text-gray-300 mb-1">
              {t('pricegap.candidates.modal.symbol.label')}
            </label>
            <input
              type="text"
              className="w-full bg-gray-900 border border-gray-600 rounded px-2 py-1 mb-1 text-white font-mono"
              value={formSymbol}
              placeholder={t('pricegap.candidates.modal.symbol.placeholder')}
              onChange={(e) => setFormSymbol(e.target.value.toUpperCase())}
            />
            {formErrors.symbol && (
              <p className="text-red-400 text-xs mb-2">{formErrors.symbol}</p>
            )}

            {/* Long / Short exchange selects */}
            <div className="grid grid-cols-2 gap-2 mb-1 mt-2">
              <div>
                <label className="block text-sm text-gray-300 mb-1">
                  {t('pricegap.candidates.modal.longExch.label')}
                </label>
                <select
                  className="w-full bg-gray-900 border border-gray-600 rounded px-2 py-1 text-white"
                  value={formLongExch}
                  onChange={(e) => setFormLongExch(e.target.value)}
                >
                  {['binance', 'bybit', 'gateio', 'bitget', 'okx', 'bingx'].map((x) => (
                    <option key={x} value={x}>
                      {x}
                    </option>
                  ))}
                </select>
              </div>
              <div>
                <label className="block text-sm text-gray-300 mb-1">
                  {t('pricegap.candidates.modal.shortExch.label')}
                </label>
                <select
                  className="w-full bg-gray-900 border border-gray-600 rounded px-2 py-1 text-white"
                  value={formShortExch}
                  onChange={(e) => setFormShortExch(e.target.value)}
                >
                  {['binance', 'bybit', 'gateio', 'bitget', 'okx', 'bingx'].map((x) => (
                    <option key={x} value={x}>
                      {x}
                    </option>
                  ))}
                </select>
              </div>
            </div>
            {formErrors.exchanges && (
              <p className="text-red-400 text-xs mb-2">{formErrors.exchanges}</p>
            )}

            {/* Numeric: threshold_bps */}
            <label className="block text-sm text-gray-300 mb-1 mt-2">
              {t('pricegap.candidates.modal.thresholdBps.label')}
            </label>
            <input
              type="number"
              min={50}
              max={1000}
              step={1}
              className="w-full bg-gray-900 border border-gray-600 rounded px-2 py-1 mb-1 text-white tabular-nums"
              value={formThresholdBps}
              onChange={(e) => setFormThresholdBps(Number(e.target.value))}
            />
            {formErrors.threshold && (
              <p className="text-red-400 text-xs mb-2">{formErrors.threshold}</p>
            )}

            {/* Numeric: max_position_usdt */}
            <label className="block text-sm text-gray-300 mb-1 mt-2">
              {t('pricegap.candidates.modal.maxPositionUSDT.label')}
            </label>
            <input
              type="number"
              min={100}
              max={50000}
              step={100}
              className="w-full bg-gray-900 border border-gray-600 rounded px-2 py-1 mb-1 text-white tabular-nums"
              value={formMaxPositionUSDT}
              onChange={(e) => setFormMaxPositionUSDT(Number(e.target.value))}
            />
            {formErrors.maxPosition && (
              <p className="text-red-400 text-xs mb-2">{formErrors.maxPosition}</p>
            )}

            {/* Numeric: modeled_slippage_bps */}
            <label className="block text-sm text-gray-300 mb-1 mt-2">
              {t('pricegap.candidates.modal.modeledSlippageBps.label')}
            </label>
            <input
              type="number"
              min={0}
              max={100}
              step={1}
              className="w-full bg-gray-900 border border-gray-600 rounded px-2 py-1 mb-1 text-white tabular-nums"
              value={formModeledSlippageBps}
              onChange={(e) => setFormModeledSlippageBps(Number(e.target.value))}
            />
            {formErrors.slippage && (
              <p className="text-red-400 text-xs mb-2">{formErrors.slippage}</p>
            )}

            {formErrors.tuple && (
              <p className="text-red-400 text-xs mb-2">{formErrors.tuple}</p>
            )}
            {modalError && <p className="text-red-400 text-sm mb-2">{modalError}</p>}

            <div className="flex justify-end gap-2 mt-4">
              <button
                type="button"
                disabled={modalBusy}
                className="px-3 py-1.5 bg-gray-700 hover:bg-gray-600 text-white rounded text-sm disabled:opacity-50"
                onClick={() => {
                  if (!modalBusy) {
                    setEditorOpen(null);
                    setModalError(null);
                    setFormErrors({});
                  }
                }}
              >
                {t('pricegap.candidates.modal.cancel')}
              </button>
              <button
                type="button"
                disabled={modalBusy}
                className="px-3 py-1.5 bg-blue-600 hover:bg-blue-500 text-white rounded text-sm disabled:opacity-50"
                onClick={() => void handleEditorSave()}
              >
                {t('pricegap.candidates.modal.save')}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Phase 10: Delete confirm dialog (D-15 single-step, full-detail) */}
      {deleteTarget && (
        <div
          className="fixed inset-0 bg-black/50 flex items-center justify-center z-50"
          onClick={() => {
            if (!modalBusy) {
              setDeleteTarget(null);
              setModalError(null);
            }
          }}
        >
          <div
            role="dialog"
            aria-labelledby="pg-delete-title"
            className="bg-gray-800 border border-gray-700 rounded-lg shadow-xl w-full max-w-sm p-5"
            onClick={(e) => e.stopPropagation()}
          >
            <h3 id="pg-delete-title" className="text-lg font-semibold text-white mb-3">
              {t('pricegap.candidates.confirmDelete.title')}
            </h3>
            <p className="text-sm text-gray-300 mb-3">
              {replaceTokens(t('pricegap.candidates.confirmDelete.body'), {
                symbol: deleteTarget.symbol,
                long: deleteTarget.long_exch,
                short: deleteTarget.short_exch,
                bps: deleteTarget.threshold_bps.toFixed(0),
              })}
            </p>
            {modalError && <p className="text-red-400 text-sm mb-2">{modalError}</p>}
            <div className="flex justify-end gap-2">
              <button
                type="button"
                disabled={modalBusy}
                className="px-3 py-1.5 bg-gray-700 hover:bg-gray-600 text-white rounded text-sm disabled:opacity-50"
                onClick={() => {
                  if (!modalBusy) {
                    setDeleteTarget(null);
                    setModalError(null);
                  }
                }}
              >
                {t('pricegap.candidates.confirmDelete.cancel')}
              </button>
              <button
                type="button"
                disabled={modalBusy}
                className="px-3 py-1.5 bg-red-600 hover:bg-red-500 text-white rounded text-sm disabled:opacity-50"
                onClick={() => void handleConfirmDelete()}
              >
                {t('pricegap.candidates.confirmDelete.confirm')}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};

export default PriceGap;
