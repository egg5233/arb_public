// Phase 16 Plan 04 — Strategy 4 Configuration card (PG-OPS-09).
//
// Top-of-page collapsible card consolidating ALL Strategy 4 editable config
// in one place. Composes 4 subsections per UI-SPEC §"Composition shape":
//
//   1. Scanner       — read-only display of scanner config (config.json only;
//                      D-17 enumerates fields but Phase 16 backend exposure is
//                      out of scope for `files_modified`).
//   2. Breaker       — read-only display + ENABLE-BREAKER typed-phrase toggle
//                      (toggle attempts POST; full numeric editing deferred to
//                      pg-admin per scope alignment with Ramp section D-19).
//   3. Ramp          — read-only display banner per D-19 (edits via pg-admin).
//   4. Live-Capital  — ENABLE-LIVE-CAPITAL typed-phrase toggle + 4 migrated
//                      gate fields (price_gap_free_bps, max_price_gap_bps,
//                      enable_price_gap_gate, max_price_gap_pct). The gate
//                      fields ARE fully editable here since their server
//                      write paths already exist via strategy.discovery.* /
//                      spot_futures.* nested envelopes.
//
// Magic strings ENABLE-LIVE-CAPITAL and ENABLE-BREAKER are LITERAL — never
// translated, never lowercased, never trimmed. BreakerConfirmModal Phase 15
// contract enforces case-sensitive exact match.
//
// D-21 architecture note: ENABLE-LIVE-CAPITAL and ENABLE-BREAKER POST bodies
// do NOT include `operator_action: true`. The typed-phrase modal IS the
// safety mechanism for these toggles. The Plan 02 server guard
// (operator_action: true) is paper_mode-scoped only — coupling unrelated
// safety surfaces silently was rejected per checker review (warning #6).
//
// togglePaper (in PriceGap.tsx) DOES send operator_action: true to satisfy
// the PG-FIX-02 server guard.
import { useCallback, useEffect, useMemo, useState, type FC } from 'react';
import { useLocale } from '../../i18n/index.ts';
import type { TranslationKey } from '../../i18n/index.ts';
import { BreakerConfirmModal } from '../Ramp/BreakerConfirmModal.tsx';

// ─── Helpers ──────────────────────────────────────────────────────────────

function authHeaders(): Record<string, string> {
  const token = localStorage.getItem('arb_token') || '';
  return {
    'Content-Type': 'application/json',
    Authorization: `Bearer ${token}`,
  };
}

function replaceTokens(s: string, tokens: Record<string, string | number>): string {
  let out = s;
  for (const [k, v] of Object.entries(tokens)) {
    out = out.replaceAll(`{${k}}`, String(v));
  }
  return out;
}

const STORAGE_KEY = 'arb_pg_config_expanded';

// ─── Snapshot of read-only config values ─────────────────────────────────
// The dashboard consumes /api/config GET which returns the full Config
// snapshot. The shape we care about is a subset; we rely on duck typing.

interface ConfigSnapshot {
  // Top-level price_gap block (Phase 11/14/15 fields exposed in snapshot).
  price_gap?: {
    discovery_enabled?: boolean;
    discovery_interval_sec?: number;
    discovery_universe?: string[];
    auto_promote_score?: number;
    max_candidates?: number;
    breaker_enabled?: boolean;
    drawdown_limit_usdt?: number;
    breaker_interval_sec?: number;
    live_capital?: boolean;
    anomaly_slippage_bps?: number;
    ramp_stage_sizes_usdt?: number[];
    hard_ceiling_usdt?: number;
    clean_days_to_promote?: number;
    paper_mode?: boolean;
    enabled?: boolean;
  };
  strategy?: {
    discovery?: {
      price_gap_free_bps?: number;
      max_price_gap_bps?: number;
    };
  };
  spot_futures?: {
    enable_price_gap_gate?: boolean;
    max_price_gap_pct?: number;
  };
}

// ─── postConfig helper ────────────────────────────────────────────────────

async function postConfig(body: Record<string, unknown>): Promise<void> {
  const res = await fetch('/api/config', {
    method: 'POST',
    headers: authHeaders(),
    body: JSON.stringify(body),
  });
  const payload = (await res.json().catch(() => null)) as {
    ok?: boolean;
    error?: string;
  } | null;
  if (!res.ok || !payload?.ok) {
    const err = new Error(payload?.error || `HTTP ${res.status}`);
    // Attach status for the caller to discriminate 409 vs 422 vs 500.
    (err as Error & { status?: number }).status = res.status;
    throw err;
  }
}

// ─── Sub-component: Read-only field row ──────────────────────────────────

interface ReadOnlyRowProps {
  labelKey: TranslationKey;
  helpKey: TranslationKey;
  value: string;
}

const ReadOnlyRow: FC<ReadOnlyRowProps> = ({ labelKey, helpKey, value }) => {
  const { t } = useLocale();
  return (
    <div>
      <label className="block text-sm font-normal text-gray-200 mb-1">
        {t(labelKey)}
      </label>
      <div className="bg-gray-900 border border-gray-700 rounded px-2 py-1 text-sm text-gray-100 tabular-nums">
        {value}
      </div>
      <p className="text-xs text-gray-500 mt-1">{t(helpKey)}</p>
    </div>
  );
};

// ─── Sub-component: Editable number field ────────────────────────────────

interface NumberRowProps {
  labelKey: TranslationKey;
  helpKey: TranslationKey;
  value: number;
  min?: number;
  max?: number;
  step?: number;
  onChange: (v: number) => void;
  disabled?: boolean;
}

const NumberRow: FC<NumberRowProps> = ({
  labelKey,
  helpKey,
  value,
  min,
  max,
  step,
  onChange,
  disabled,
}) => {
  const { t } = useLocale();
  return (
    <div>
      <label className="block text-sm font-normal text-gray-200 mb-1">
        {t(labelKey)}
      </label>
      <input
        type="number"
        value={Number.isFinite(value) ? value : ''}
        onChange={(e) => onChange(Number(e.target.value))}
        min={min}
        max={max}
        step={step}
        disabled={disabled}
        className="w-full bg-gray-900 border border-gray-700 rounded px-2 py-1 text-sm text-white tabular-nums focus:outline-none focus:border-yellow-400"
      />
      <p className="text-xs text-gray-500 mt-1">{t(helpKey)}</p>
    </div>
  );
};

// ─── Sub-component: Toggle row ───────────────────────────────────────────

interface ToggleRowProps {
  labelKey: TranslationKey;
  helpKey: TranslationKey;
  on: boolean;
  onChange: () => void;
  disabled?: boolean;
}

const ToggleRow: FC<ToggleRowProps> = ({ labelKey, helpKey, on, onChange, disabled }) => {
  const { t } = useLocale();
  return (
    <div>
      <label className="block text-sm font-normal text-gray-200 mb-1">
        {t(labelKey)}
      </label>
      <div className="flex items-center gap-3">
        <input
          type="checkbox"
          checked={on}
          onChange={() => onChange()}
          disabled={disabled}
          className="h-4 w-4"
        />
        <span
          className={`text-sm font-semibold ${on ? 'text-green-400' : 'text-red-400'}`}
        >
          {on ? 'ON' : 'OFF'}
        </span>
      </div>
      <p className="text-xs text-gray-500 mt-1">{t(helpKey)}</p>
    </div>
  );
};

// ─── ConfigCard ───────────────────────────────────────────────────────────

export const ConfigCard: FC = () => {
  const { t } = useLocale();

  // Persisted expanded state.
  const [expanded, setExpanded] = useState<boolean>(() => {
    return localStorage.getItem(STORAGE_KEY) === '1';
  });
  useEffect(() => {
    localStorage.setItem(STORAGE_KEY, expanded ? '1' : '0');
  }, [expanded]);

  const [config, setConfig] = useState<ConfigSnapshot | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);

  // Fetch config snapshot (best-effort; failures show loading state).
  useEffect(() => {
    let cancelled = false;
    fetch('/api/config', { headers: authHeaders() })
      .then((r) => r.json())
      .then((body: { ok?: boolean; data?: ConfigSnapshot; error?: string } | null) => {
        if (cancelled) return;
        if (body?.ok && body.data) {
          setConfig(body.data);
          setLoadError(null);
        } else {
          setLoadError(body?.error || 'load failed');
        }
      })
      .catch((e: unknown) => {
        if (!cancelled) {
          setLoadError(e instanceof Error ? e.message : String(e));
        }
      });
    return () => {
      cancelled = true;
    };
  }, []);

  // ── Modal state for typed-phrase toggles ────────────────────────────────

  type ModalState = {
    kind: 'enable-live-capital' | 'disable-live-capital' | 'enable-breaker' | 'disable-breaker';
  } | null;
  const [modal, setModal] = useState<ModalState>(null);
  const [modalBusy, setModalBusy] = useState(false);
  const [modalError, setModalError] = useState<string | null>(null);

  const closeModal = useCallback(() => {
    if (!modalBusy) {
      setModal(null);
      setModalError(null);
    }
  }, [modalBusy]);

  const submitLiveCapital = useCallback(
    async (next: boolean) => {
      setModalBusy(true);
      setModalError(null);
      try {
        // D-21 (CRITICAL): live-capital POST does NOT include operator_action.
        // Typed-phrase modal IS the safety mechanism. The Plan 02 server
        // guard is paper_mode-scoped only.
        await postConfig({ price_gap_live_capital: next });
        setConfig((prev) => prev ? { ...prev, price_gap: { ...prev.price_gap, live_capital: next } } : prev);
        setModal(null);
      } catch (e: unknown) {
        const err = e as Error & { status?: number };
        if (err.status === 409) {
          setModalError(t('pricegap.config.error.operatorActionRequired'));
        } else if (err.status === 422) {
          setModalError(replaceTokens(t('pricegap.config.error.serverValidation'), {
            reason: err.message,
          }));
        } else {
          setModalError(t('pricegap.config.error.saveNetwork'));
        }
      } finally {
        setModalBusy(false);
      }
    },
    [t],
  );

  const submitBreaker = useCallback(
    async (next: boolean) => {
      setModalBusy(true);
      setModalError(null);
      try {
        // D-21: no operator_action; typed-phrase modal is the safety surface.
        await postConfig({ price_gap_breaker_enabled: next });
        setConfig((prev) => prev ? { ...prev, price_gap: { ...prev.price_gap, breaker_enabled: next } } : prev);
        setModal(null);
      } catch (e: unknown) {
        const err = e as Error & { status?: number };
        if (err.status === 422) {
          setModalError(replaceTokens(t('pricegap.config.error.serverValidation'), {
            reason: err.message,
          }));
        } else {
          setModalError(t('pricegap.config.error.saveNetwork'));
        }
      } finally {
        setModalBusy(false);
      }
    },
    [t],
  );

  // ── Migrated gate field state (editable; existing nested write paths) ──

  // Local-edit mirror so unsaved edits don't fight the server snapshot.
  const [gateDraft, setGateDraft] = useState<{
    free_bps: number;
    max_bps: number;
    sf_enable: boolean;
    sf_max_pct: number;
  } | null>(null);
  const [gateSaving, setGateSaving] = useState(false);
  const [gateError, setGateError] = useState<string | null>(null);

  // Initialize draft when config first loads.
  useEffect(() => {
    if (!config || gateDraft) return;
    setGateDraft({
      free_bps: config.strategy?.discovery?.price_gap_free_bps ?? 0,
      max_bps: config.strategy?.discovery?.max_price_gap_bps ?? 0,
      sf_enable: config.spot_futures?.enable_price_gap_gate ?? false,
      sf_max_pct: config.spot_futures?.max_price_gap_pct ?? 0,
    });
  }, [config, gateDraft]);

  const gateDirty = useMemo(() => {
    if (!gateDraft || !config) return false;
    const cur = {
      free_bps: config.strategy?.discovery?.price_gap_free_bps ?? 0,
      max_bps: config.strategy?.discovery?.max_price_gap_bps ?? 0,
      sf_enable: config.spot_futures?.enable_price_gap_gate ?? false,
      sf_max_pct: config.spot_futures?.max_price_gap_pct ?? 0,
    };
    return (
      cur.free_bps !== gateDraft.free_bps ||
      cur.max_bps !== gateDraft.max_bps ||
      cur.sf_enable !== gateDraft.sf_enable ||
      cur.sf_max_pct !== gateDraft.sf_max_pct
    );
  }, [gateDraft, config]);

  const saveGate = useCallback(async () => {
    if (!gateDraft) return;
    setGateSaving(true);
    setGateError(null);
    try {
      // Migrated gate fields — no operator_action (no paper_mode coupling).
      await postConfig({
        strategy: {
          discovery: {
            price_gap_free_bps: gateDraft.free_bps,
            max_price_gap_bps: gateDraft.max_bps,
          },
        },
        spot_futures: {
          enable_price_gap_gate: gateDraft.sf_enable,
          max_price_gap_pct: gateDraft.sf_max_pct,
        },
      });
      // Sync mirror.
      setConfig((prev) =>
        prev
          ? {
              ...prev,
              strategy: {
                ...prev.strategy,
                discovery: {
                  ...prev.strategy?.discovery,
                  price_gap_free_bps: gateDraft.free_bps,
                  max_price_gap_bps: gateDraft.max_bps,
                },
              },
              spot_futures: {
                ...prev.spot_futures,
                enable_price_gap_gate: gateDraft.sf_enable,
                max_price_gap_pct: gateDraft.sf_max_pct,
              },
            }
          : prev,
      );
    } catch (e: unknown) {
      const err = e as Error & { status?: number };
      if (err.status === 422) {
        setGateError(replaceTokens(t('pricegap.config.error.serverValidation'), {
          reason: err.message,
        }));
      } else {
        setGateError(t('pricegap.config.error.saveNetwork'));
      }
    } finally {
      setGateSaving(false);
    }
  }, [gateDraft, t]);

  // ── Render ──────────────────────────────────────────────────────────────

  const fmtNum = (v: number | undefined): string =>
    v == null || !Number.isFinite(v) ? '—' : String(v);
  const fmtBool = (v: boolean | undefined): string => (v ? 'ON' : 'OFF');
  const fmtList = (v: number[] | undefined): string =>
    !v || v.length === 0 ? '—' : v.join(', ');

  // Live-capital toggle click handler.
  const onLiveCapitalToggle = useCallback(() => {
    const cur = config?.price_gap?.live_capital === true;
    setModal({ kind: cur ? 'disable-live-capital' : 'enable-live-capital' });
    setModalError(null);
  }, [config]);

  // Breaker toggle click handler.
  const onBreakerToggle = useCallback(() => {
    const cur = config?.price_gap?.breaker_enabled === true;
    setModal({ kind: cur ? 'disable-breaker' : 'enable-breaker' });
    setModalError(null);
  }, [config]);

  return (
    <div className="card-surface p-4 mt-4 mb-6" data-test="pg-config-card">
      {/* Card heading — full row is click target */}
      <button
        type="button"
        className="w-full flex items-center justify-between text-left"
        onClick={() => setExpanded((v) => !v)}
        aria-expanded={expanded}
        aria-controls="pg-config-card-body"
        data-test="pg-config-card-header"
      >
        <div>
          <h2 className="text-lg font-semibold text-gray-100">
            {t('pricegap.config.cardTitle')}
          </h2>
          <p className="text-xs text-gray-400 mt-0.5">
            {expanded ? t('pricegap.config.cardSubtitle') : t('pricegap.config.collapsed')}
          </p>
        </div>
        <span className="text-gray-400 text-xl" aria-hidden="true">
          {expanded ? '▾' : '▸'}
        </span>
      </button>

      {expanded && (
        <div id="pg-config-card-body" className="mt-4">
          {!config && !loadError && (
            <p className="text-sm text-gray-400">{t('pricegap.config.loading')}</p>
          )}
          {loadError && (
            <p className="text-sm text-red-400">{loadError}</p>
          )}

          {config && (
            <>
              {/* ── Subsection 1: Scanner ──────────────────────────────── */}
              <section data-test="pg-config-section-scanner">
                <h3 className="text-base font-semibold text-gray-100">
                  {t('pricegap.config.section.scanner')}
                </h3>
                <p className="text-xs text-gray-500 mb-3">
                  {t('pricegap.config.ramp.readOnlyNote')}
                </p>
                <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                  <ReadOnlyRow
                    labelKey="pricegap.config.scanner.discovery.label"
                    helpKey="pricegap.config.scanner.discovery.help"
                    value={fmtBool(config.price_gap?.discovery_enabled)}
                  />
                  <ReadOnlyRow
                    labelKey="pricegap.config.scanner.interval.label"
                    helpKey="pricegap.config.scanner.interval.help"
                    value={fmtNum(config.price_gap?.discovery_interval_sec)}
                  />
                  <ReadOnlyRow
                    labelKey="pricegap.config.scanner.maxUniverse.label"
                    helpKey="pricegap.config.scanner.maxUniverse.help"
                    value={String(config.price_gap?.discovery_universe?.length ?? 0)}
                  />
                  <ReadOnlyRow
                    labelKey="pricegap.config.scanner.promoteScore.label"
                    helpKey="pricegap.config.scanner.promoteScore.help"
                    value={fmtNum(config.price_gap?.auto_promote_score)}
                  />
                  <ReadOnlyRow
                    labelKey="pricegap.config.scanner.maxCandidates.label"
                    helpKey="pricegap.config.scanner.maxCandidates.help"
                    value={fmtNum(config.price_gap?.max_candidates)}
                  />
                </div>
              </section>

              {/* ── Subsection 2: Drawdown Circuit Breaker ─────────────── */}
              <section
                className="mt-6 pt-4 border-t border-gray-700"
                data-test="pg-config-section-breaker"
              >
                <h3 className="text-base font-semibold text-gray-100">
                  {t('pricegap.config.section.breaker')}
                </h3>
                <div className="grid grid-cols-1 md:grid-cols-2 gap-3 mt-3">
                  {/* Breaker enable toggle — typed-phrase gated */}
                  <div>
                    <label className="block text-sm font-normal text-gray-200 mb-1">
                      {t('pricegap.config.breaker.enabled.label')}
                    </label>
                    <button
                      type="button"
                      onClick={onBreakerToggle}
                      className="flex items-center gap-3"
                      data-test="pg-config-breaker-toggle"
                    >
                      <input
                        type="checkbox"
                        checked={config.price_gap?.breaker_enabled === true}
                        readOnly
                        className="h-4 w-4 pointer-events-none"
                      />
                      <span
                        className={`text-sm font-semibold ${
                          config.price_gap?.breaker_enabled
                            ? 'text-green-400'
                            : 'text-red-400'
                        }`}
                      >
                        {fmtBool(config.price_gap?.breaker_enabled)}
                      </span>
                    </button>
                    <p className="text-xs text-gray-500 mt-1">
                      {t('pricegap.config.breaker.enabled.help')}
                    </p>
                  </div>
                  <ReadOnlyRow
                    labelKey="pricegap.config.breaker.limit.label"
                    helpKey="pricegap.config.breaker.limit.help"
                    value={fmtNum(config.price_gap?.drawdown_limit_usdt)}
                  />
                  <ReadOnlyRow
                    labelKey="pricegap.config.breaker.interval.label"
                    helpKey="pricegap.config.breaker.interval.help"
                    value={fmtNum(config.price_gap?.breaker_interval_sec)}
                  />
                </div>
              </section>

              {/* ── Subsection 3: Live-Capital Ramp (read-only D-19) ───── */}
              <section
                className="mt-6 pt-4 border-t border-gray-700"
                data-test="pg-config-section-ramp"
              >
                <h3 className="text-base font-semibold text-gray-100">
                  {t('pricegap.config.section.ramp')}
                </h3>
                <div className="mt-2 mb-3 p-3 rounded border border-yellow-700 bg-yellow-900/20 text-yellow-300 text-xs">
                  {t('pricegap.config.ramp.readOnlyNote')}
                </div>
                <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                  <div>
                    <label className="block text-sm font-normal text-gray-200 mb-1">
                      Stage sizes (USDT)
                    </label>
                    <div className="bg-gray-900 border border-gray-700 rounded px-2 py-1 text-sm text-gray-100 tabular-nums">
                      {fmtList(config.price_gap?.ramp_stage_sizes_usdt)}
                    </div>
                  </div>
                  <div>
                    <label className="block text-sm font-normal text-gray-200 mb-1">
                      Hard ceiling (USDT/leg)
                    </label>
                    <div className="bg-gray-900 border border-gray-700 rounded px-2 py-1 text-sm text-gray-100 tabular-nums">
                      {fmtNum(config.price_gap?.hard_ceiling_usdt)}
                    </div>
                  </div>
                  <div>
                    <label className="block text-sm font-normal text-gray-200 mb-1">
                      Clean days to promote
                    </label>
                    <div className="bg-gray-900 border border-gray-700 rounded px-2 py-1 text-sm text-gray-100 tabular-nums">
                      {fmtNum(config.price_gap?.clean_days_to_promote)}
                    </div>
                  </div>
                </div>
              </section>

              {/* ── Subsection 4: Live-Capital + Risk ─────────────────── */}
              <section
                className="mt-6 pt-4 border-t border-gray-700"
                data-test="pg-config-section-live-capital"
              >
                <h3 className="text-base font-semibold text-gray-100">
                  {t('pricegap.config.section.liveCapital')}
                </h3>
                <div className="grid grid-cols-1 md:grid-cols-2 gap-3 mt-3">
                  {/* Live capital toggle — typed-phrase gated */}
                  <div>
                    <label className="block text-sm font-normal text-gray-200 mb-1">
                      {t('pricegap.config.liveCapital.master.label')}
                    </label>
                    <button
                      type="button"
                      onClick={onLiveCapitalToggle}
                      className="flex items-center gap-3"
                      data-test="pg-config-live-capital-toggle"
                    >
                      <input
                        type="checkbox"
                        checked={config.price_gap?.live_capital === true}
                        readOnly
                        className="h-4 w-4 pointer-events-none"
                      />
                      <span
                        className={`text-sm font-semibold ${
                          config.price_gap?.live_capital
                            ? 'text-green-400'
                            : 'text-red-400'
                        }`}
                      >
                        {fmtBool(config.price_gap?.live_capital)}
                      </span>
                    </button>
                    <p className="text-xs text-gray-500 mt-1">
                      {t('pricegap.config.liveCapital.master.help')}
                    </p>
                  </div>
                  <ReadOnlyRow
                    labelKey="pricegap.config.liveCapital.anomalyBps.label"
                    helpKey="pricegap.config.liveCapital.anomalyBps.help"
                    value={fmtNum(config.price_gap?.anomaly_slippage_bps)}
                  />

                  {/* Migrated gate fields — fully editable */}
                  {gateDraft && (
                    <>
                      <NumberRow
                        labelKey="pricegap.config.gate.freeBps.label"
                        helpKey="pricegap.config.gate.freeBps.help"
                        value={gateDraft.free_bps}
                        step={1}
                        onChange={(v) =>
                          setGateDraft((d) => (d ? { ...d, free_bps: v } : d))
                        }
                        disabled={gateSaving}
                      />
                      <NumberRow
                        labelKey="pricegap.config.gate.maxBps.label"
                        helpKey="pricegap.config.gate.maxBps.help"
                        value={gateDraft.max_bps}
                        step={1}
                        onChange={(v) =>
                          setGateDraft((d) => (d ? { ...d, max_bps: v } : d))
                        }
                        disabled={gateSaving}
                      />
                      <ToggleRow
                        labelKey="pricegap.config.gate.spotFuturesEnabled.label"
                        helpKey="pricegap.config.gate.spotFuturesEnabled.help"
                        on={gateDraft.sf_enable}
                        onChange={() =>
                          setGateDraft((d) =>
                            d ? { ...d, sf_enable: !d.sf_enable } : d,
                          )
                        }
                        disabled={gateSaving}
                      />
                      <NumberRow
                        labelKey="pricegap.config.gate.maxPct.label"
                        helpKey="pricegap.config.gate.maxPct.help"
                        value={gateDraft.sf_max_pct}
                        step={0.01}
                        onChange={(v) =>
                          setGateDraft((d) => (d ? { ...d, sf_max_pct: v } : d))
                        }
                        disabled={gateSaving}
                      />
                    </>
                  )}
                </div>

                {gateError && (
                  <p className="text-red-400 text-xs mt-2">{gateError}</p>
                )}

                <div className="mt-4 flex justify-end">
                  <button
                    type="button"
                    disabled={!gateDirty || gateSaving}
                    onClick={saveGate}
                    className="btn-primary px-3 py-1.5 text-sm rounded disabled:opacity-50"
                    data-test="pg-config-save-gate"
                  >
                    {gateSaving
                      ? t('pricegap.config.saving')
                      : replaceTokens(t('pricegap.config.saveButton'), {
                          section: t('pricegap.config.section.liveCapital'),
                        })}
                  </button>
                </div>
              </section>
            </>
          )}
        </div>
      )}

      {/* ── Modals ──────────────────────────────────────────────────────── */}
      <BreakerConfirmModal
        open={modal?.kind === 'enable-live-capital'}
        action="recover"
        magicPhrase="ENABLE-LIVE-CAPITAL"
        promptKey="pricegap.config.confirmPrompt.enableLiveCapital"
        titleKey="pricegap.config.confirmTitle.enableLiveCapital"
        submitKey="pricegap.config.confirmSubmit.enableLiveCapital"
        forceWarning
        hideDryRun
        destructiveSubmit
        busy={modalBusy}
        errorMessage={modalError}
        onClose={closeModal}
        onConfirm={() => submitLiveCapital(true)}
      />
      <BreakerConfirmModal
        open={modal?.kind === 'disable-live-capital'}
        action="recover"
        magicPhrase={'' as never}
        promptKey="pricegap.config.confirmPrompt.disableLiveCapital"
        titleKey="pricegap.config.confirmTitle.disableLiveCapital"
        submitKey="pricegap.config.confirmSubmit.disableLiveCapital"
        hideDryRun
        busy={modalBusy}
        errorMessage={modalError}
        onClose={closeModal}
        onConfirm={() => submitLiveCapital(false)}
      />
      <BreakerConfirmModal
        open={modal?.kind === 'enable-breaker'}
        action="recover"
        magicPhrase="ENABLE-BREAKER"
        promptKey="pricegap.config.confirmPrompt.enableBreaker"
        titleKey="pricegap.config.confirmTitle.enableBreaker"
        submitKey="pricegap.config.confirmSubmit.enableBreaker"
        hideDryRun
        busy={modalBusy}
        errorMessage={modalError}
        onClose={closeModal}
        onConfirm={() => submitBreaker(true)}
      />
      <BreakerConfirmModal
        open={modal?.kind === 'disable-breaker'}
        action="recover"
        magicPhrase={'' as never}
        promptKey="pricegap.config.confirmPrompt.disableBreaker"
        titleKey="pricegap.config.confirmTitle.disableBreaker"
        submitKey="pricegap.config.confirmSubmit.disableBreaker"
        hideDryRun
        destructiveSubmit
        busy={modalBusy}
        errorMessage={modalError}
        onClose={closeModal}
        onConfirm={() => submitBreaker(false)}
      />
    </div>
  );
};

export default ConfigCard;
