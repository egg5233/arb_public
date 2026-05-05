// Phase 14 Plan 14-05 — RampReconcileSection (PG-LIVE-01 / PG-LIVE-03).
//
// Dashboard operator surface for ramp state and daily reconcile.
//
// T-14-18 mitigation: live-capital badge reads `live_capital` from the
// /api/pg/ramp response, which is server-authoritative (see
// internal/api/pricegap_ramp_handlers.go — server reads cfg directly, NOT a
// client cache). Big color-coded badge (red ON / gray OFF) so the operator
// can never confuse paper-mode for live-mode at a glance.
import { useCallback, useEffect, useState, type FC } from 'react';
import { useLocale } from '../../i18n/index.ts';
import { BreakerSubsection } from './BreakerSubsection.tsx';
import { BreakerConfirmModal } from './BreakerConfirmModal.tsx';

interface RampState {
  current_stage: number;
  clean_day_counter: number;
  last_eval_ts: string;
  last_loss_day_ts: string;
  demote_count: number;
  live_capital: boolean;
  stage_1_size_usdt: number;
  stage_2_size_usdt: number;
  stage_3_size_usdt: number;
  hard_ceiling_usdt: number;
  clean_days_to_promote: number;
}

interface DailyTotals {
  realized_pnl_usdt: number;
  positions_closed: number;
  wins: number;
  losses: number;
  net_clean: boolean;
}

interface DailyAnomalies {
  high_slippage_count: number;
  missing_close_ts_count: number;
  flagged_ids: string[];
}

interface DailyRecord {
  schema_version: number;
  date: string;
  computed_at: string;
  totals: DailyTotals;
  anomalies: DailyAnomalies;
}

// Shared envelope for /api/pg/ramp + /api/pg/reconcile/{date}.
interface ApiEnvelope<T> {
  ok: boolean;
  data?: T;
  error?: string;
}

type RampAction = 'reset' | 'force-promote' | 'force-demote';

// Yesterday in UTC, formatted as YYYY-MM-DD. The reconciler runs at UTC 00:30
// for the previous UTC day, so "yesterday UTC" is the latest available
// reconcile when the dashboard mounts.
function yesterdayUTC(): string {
  const y = new Date();
  y.setUTCDate(y.getUTCDate() - 1);
  return y.toISOString().slice(0, 10);
}

// Render a Go time.Time stringification — empty / 0001-01-01 timestamps come
// from a never-set zero-value time.Time field; surface as em-dash so operators
// don't see "0001-01-01" in production.
function formatNullableDate(s: string): string {
  if (!s || s.startsWith('0001')) return '—';
  return s.slice(0, 10);
}

function authHeaders(): Record<string, string> {
  const token = localStorage.getItem('arb_token');
  return {
    'Content-Type': 'application/json',
    ...(token ? { Authorization: `Bearer ${token}` } : {}),
  };
}

export const RampReconcileSection: FC = () => {
  const { t } = useLocale();
  const [ramp, setRamp] = useState<RampState | null>(null);
  const [reconcile, setReconcile] = useState<DailyRecord | null>(null);
  const [rampError, setRampError] = useState<string | null>(null);
  const [reconcileMissing, setReconcileMissing] = useState(false);
  const [reconcileDate, setReconcileDate] = useState(yesterdayUTC);
  const [rampReason, setRampReason] = useState('');
  const [modalAction, setModalAction] = useState<RampAction | null>(null);
  const [actionBusy, setActionBusy] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);
  const [reconcileBusy, setReconcileBusy] = useState(false);
  const [reconcileError, setReconcileError] = useState<string | null>(null);

  const loadRamp = useCallback(async () => {
    fetch('/api/pg/ramp', { headers: authHeaders() })
      .then((r) => r.json() as Promise<ApiEnvelope<RampState>>)
      .then((j) => {
        if (j.ok && j.data) {
          setRamp(j.data);
          setRampError(null);
        } else {
          setRampError(j.error || 'unable to load ramp state');
        }
      })
      .catch((err) => {
        setRampError(String(err));
      });
  }, []);

  const loadReconcile = useCallback(async (dateStr: string) => {
    setReconcile(null);
    setReconcileMissing(false);
    fetch(`/api/pg/reconcile/${dateStr}`, { headers: authHeaders() })
      .then(async (r) => {
        if (r.status === 404) {
          setReconcileMissing(true);
          return null;
        }
        return r.json() as Promise<ApiEnvelope<DailyRecord>>;
      })
      .then((j) => {
        if (!j) return;
        if (j.ok && j.data) {
          setReconcile(j.data);
        }
      })
      .catch(() => {
        setReconcileMissing(true);
      });
  }, []);

  useEffect(() => {
    void loadRamp();
    void loadReconcile(reconcileDate);
  }, [loadRamp, loadReconcile, reconcileDate]);

  const runRampAction = useCallback(async (action: RampAction) => {
    setActionBusy(true);
    setActionError(null);
    const phrase =
      action === 'reset'
        ? 'RESET-RAMP'
        : action === 'force-promote'
          ? 'FORCE-PROMOTE'
          : 'FORCE-DEMOTE';
    try {
      const res = await fetch(`/api/pg/ramp/${action}`, {
        method: 'POST',
        headers: authHeaders(),
        body: JSON.stringify({
          confirmation_phrase: phrase,
          reason: rampReason,
        }),
      });
      const body = (await res.json().catch(() => null)) as ApiEnvelope<unknown> | null;
      if (!res.ok || !body?.ok) throw new Error(body?.error || `HTTP ${res.status}`);
      setModalAction(null);
      await loadRamp();
    } catch (err) {
      setActionError(err instanceof Error ? err.message : String(err));
    } finally {
      setActionBusy(false);
    }
  }, [loadRamp, rampReason]);

  const runReconcile = useCallback(async () => {
    setReconcileBusy(true);
    setReconcileError(null);
    try {
      const res = await fetch(`/api/pg/reconcile/${reconcileDate}/run`, {
        method: 'POST',
        headers: authHeaders(),
        body: JSON.stringify({}),
      });
      const body = (await res.json().catch(() => null)) as ApiEnvelope<unknown> | null;
      if (!res.ok || !body?.ok) throw new Error(body?.error || `HTTP ${res.status}`);
      await loadReconcile(reconcileDate);
      await loadRamp();
    } catch (err) {
      setReconcileError(err instanceof Error ? err.message : String(err));
    } finally {
      setReconcileBusy(false);
    }
  }, [loadRamp, loadReconcile, reconcileDate]);

  if (rampError && !ramp) {
    return (
      <section className="my-6 p-4 border border-red-300 rounded-lg bg-red-50" data-test="ramp-reconcile-section">
        <div className="text-red-700 text-sm">Ramp state unavailable: {rampError}</div>
      </section>
    );
  }

  if (!ramp) {
    return (
      <section className="my-6 p-4 border border-gray-300 rounded-lg" data-test="ramp-reconcile-section">
        <div className="text-gray-500 text-sm">Loading ramp state…</div>
      </section>
    );
  }

  const badgeClass = ramp.live_capital
    ? 'inline-block px-3 py-1 rounded font-bold bg-red-600 text-white'
    : 'inline-block px-3 py-1 rounded font-bold bg-gray-400 text-white';
  const badgeLabel = ramp.live_capital
    ? t('pricegap.ramp.liveCapitalOn')
    : t('pricegap.ramp.liveCapitalOff');

  // Per-leg size for the current stage. Falls back to 0 for any out-of-range
  // CurrentStage (e.g., bootstrap returned 0 — gate 6 fail-closed signal).
  const stageSizes = [ramp.stage_1_size_usdt, ramp.stage_2_size_usdt, ramp.stage_3_size_usdt];
  const stageSize = stageSizes[ramp.current_stage - 1] ?? 0;

  return (
    <section
      className="my-6 p-4 border border-gray-300 rounded-lg"
      data-test="ramp-reconcile-section"
    >
      <div className="flex items-center justify-between mb-4">
        <h2 className="text-lg font-bold">{t('pricegap.ramp.title')}</h2>
        <span className={badgeClass} data-test="live-capital-badge">
          {badgeLabel}
        </span>
      </div>

      <div className="grid grid-cols-2 md:grid-cols-3 gap-3 text-sm">
        <div>
          <span className="text-gray-500">{t('pricegap.ramp.stage')}:</span>{' '}
          <strong>
            {ramp.current_stage}/3
          </strong>{' '}
          (${stageSize.toFixed(0)}/leg)
        </div>
        <div>
          <span className="text-gray-500">{t('pricegap.ramp.cleanDayCounter')}:</span>{' '}
          <strong>
            {ramp.clean_day_counter}/{ramp.clean_days_to_promote}
          </strong>
        </div>
        <div>
          <span className="text-gray-500">{t('pricegap.ramp.demoteCount')}:</span>{' '}
          {ramp.demote_count}
        </div>
        <div>
          <span className="text-gray-500">{t('pricegap.ramp.lastLossDay')}:</span>{' '}
          {formatNullableDate(ramp.last_loss_day_ts)}
        </div>
        <div>
          <span className="text-gray-500">Hard Ceiling:</span>{' '}
          ${ramp.hard_ceiling_usdt.toFixed(0)}/leg
        </div>
      </div>

      <div className="mt-5 rounded border border-gray-700 bg-gray-900/40 p-3">
        <label className="block text-xs text-gray-400 mb-1">
          {t('pricegap.ramp.actionReason')}
        </label>
        <input
          type="text"
          value={rampReason}
          onChange={(e) => setRampReason(e.target.value)}
          className="w-full bg-gray-950 border border-gray-700 rounded px-2 py-1 text-sm text-white"
          placeholder={t('pricegap.ramp.actionReasonPlaceholder')}
          data-test="ramp-action-reason"
        />
        <div className="flex flex-wrap gap-2 mt-3">
          <button
            type="button"
            disabled={!rampReason.trim()}
            onClick={() => setModalAction('reset')}
            className="px-3 py-1.5 rounded bg-red-600 text-white text-sm disabled:opacity-50"
            data-test="ramp-reset-button"
          >
            {t('pricegap.ramp.resetButton')}
          </button>
          <button
            type="button"
            disabled={!rampReason.trim() || ramp.current_stage >= 3}
            onClick={() => setModalAction('force-promote')}
            className="px-3 py-1.5 rounded bg-blue-600 text-white text-sm disabled:opacity-50"
            data-test="ramp-promote-button"
          >
            {t('pricegap.ramp.forcePromoteButton')}
          </button>
          <button
            type="button"
            disabled={!rampReason.trim() || ramp.current_stage <= 1}
            onClick={() => setModalAction('force-demote')}
            className="px-3 py-1.5 rounded bg-orange-600 text-white text-sm disabled:opacity-50"
            data-test="ramp-demote-button"
          >
            {t('pricegap.ramp.forceDemoteButton')}
          </button>
        </div>
      </div>

      {reconcile ? (
        <div className="mt-6 pt-4 border-t border-gray-200" data-test="reconcile-summary">
          <h3 className="text-md font-semibold mb-2">
            {t('pricegap.reconcile.title')} ({reconcile.date})
          </h3>
          <div className="grid grid-cols-2 md:grid-cols-3 gap-3 text-sm">
            <div>
              <span className="text-gray-500">{t('pricegap.reconcile.totalPnl')}:</span>{' '}
              <strong className={reconcile.totals.realized_pnl_usdt >= 0 ? 'text-green-700' : 'text-red-700'}>
                ${reconcile.totals.realized_pnl_usdt.toFixed(2)}
              </strong>
            </div>
            <div>
              <span className="text-gray-500">{t('pricegap.reconcile.positionsClosed')}:</span>{' '}
              {reconcile.totals.positions_closed}
            </div>
            <div>
              <span className="text-gray-500">{t('pricegap.reconcile.winLossSplit')}:</span>{' '}
              {reconcile.totals.wins}W / {reconcile.totals.losses}L
            </div>
          </div>
          <div className="mt-3">
            <span className="text-gray-500 text-sm">{t('pricegap.reconcile.anomalies')}:</span>{' '}
            {reconcile.anomalies.flagged_ids.length === 0 ? (
              <span className="text-green-700 text-sm">{t('pricegap.reconcile.noAnomalies')}</span>
            ) : (
              <span className="text-orange-700 text-sm" data-test="anomaly-list">
                {reconcile.anomalies.high_slippage_count} high-slip + {reconcile.anomalies.missing_close_ts_count} missing-ts:{' '}
                {reconcile.anomalies.flagged_ids.join(', ')}
              </span>
            )}
          </div>
        </div>
      ) : (
        <div className="mt-4 text-gray-500 text-sm">
          {reconcileMissing ? 'No reconcile data for yesterday yet.' : 'Loading reconcile…'}
        </div>
      )}

      <div className="mt-4 rounded border border-gray-700 bg-gray-900/40 p-3">
        <label className="block text-xs text-gray-400 mb-1">
          {t('pricegap.reconcile.runDate')}
        </label>
        <div className="flex flex-wrap gap-2">
          <input
            type="date"
            value={reconcileDate}
            onChange={(e) => setReconcileDate(e.target.value)}
            className="bg-gray-950 border border-gray-700 rounded px-2 py-1 text-sm text-white"
            data-test="reconcile-date-input"
          />
          <button
            type="button"
            disabled={reconcileBusy || !reconcileDate}
            onClick={() => void runReconcile()}
            className="px-3 py-1.5 rounded bg-blue-600 text-white text-sm disabled:opacity-50"
            data-test="reconcile-run-button"
          >
            {reconcileBusy ? t('pricegap.reconcile.running') : t('pricegap.reconcile.runButton')}
          </button>
        </div>
        {reconcileError && (
          <div className="mt-2 text-xs text-red-400">{reconcileError}</div>
        )}
      </div>

      {/* Phase 15 Plan 15-05 — Drawdown Circuit Breaker subsection (PG-LIVE-02).
          Renders status badge + recover/test-fire controls. Phase 16
          (PG-OPS-09) will lift this widget into the new top-level Pricegap
          tab without restructuring the BreakerSubsection component. */}
      <BreakerSubsection />
      <BreakerConfirmModal
        open={modalAction === 'reset'}
        action="recover"
        magicPhrase="RESET-RAMP"
        promptKey="pricegap.ramp.confirmResetPrompt"
        titleKey="pricegap.ramp.confirmResetTitle"
        submitKey="pricegap.ramp.confirmSubmit"
        hideDryRun
        destructiveSubmit
        busy={actionBusy}
        errorMessage={actionError}
        onClose={() => {
          if (!actionBusy) setModalAction(null);
        }}
        onConfirm={() => void runRampAction('reset')}
      />
      <BreakerConfirmModal
        open={modalAction === 'force-promote'}
        action="recover"
        magicPhrase="FORCE-PROMOTE"
        promptKey="pricegap.ramp.confirmPromotePrompt"
        titleKey="pricegap.ramp.confirmPromoteTitle"
        submitKey="pricegap.ramp.confirmSubmit"
        hideDryRun
        busy={actionBusy}
        errorMessage={actionError}
        onClose={() => {
          if (!actionBusy) setModalAction(null);
        }}
        onConfirm={() => void runRampAction('force-promote')}
      />
      <BreakerConfirmModal
        open={modalAction === 'force-demote'}
        action="recover"
        magicPhrase="FORCE-DEMOTE"
        promptKey="pricegap.ramp.confirmDemotePrompt"
        titleKey="pricegap.ramp.confirmDemoteTitle"
        submitKey="pricegap.ramp.confirmSubmit"
        hideDryRun
        destructiveSubmit
        busy={actionBusy}
        errorMessage={actionError}
        onClose={() => {
          if (!actionBusy) setModalAction(null);
        }}
        onConfirm={() => void runRampAction('force-demote')}
      />
    </section>
  );
};

export default RampReconcileSection;
