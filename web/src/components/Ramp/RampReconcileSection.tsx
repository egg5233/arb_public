// Phase 14 Plan 14-05 — RampReconcileSection (PG-LIVE-01 / PG-LIVE-03).
//
// READ-ONLY widget per D-14. Force-promote / force-demote / reset live in
// pg-admin CLI only this phase. Phase 16 (PG-OPS-09) will absorb this widget
// into the new top-level Pricegap tab and MAY introduce mutators behind a
// separate auth layer.
//
// DO NOT add buttons that POST/PUT/PATCH/DELETE anywhere — that's Phase 16
// territory. T-14-17 mitigation: this comment + the absence of any mutation
// fetch is the read-only invariant. A reviewer must reject any PR that
// introduces a mutating fetch in this file.
//
// T-14-18 mitigation: live-capital badge reads `live_capital` from the
// /api/pg/ramp response, which is server-authoritative (see
// internal/api/pricegap_ramp_handlers.go — server reads cfg directly, NOT a
// client cache). Big color-coded badge (red ON / gray OFF) so the operator
// can never confuse paper-mode for live-mode at a glance.
import { useEffect, useState, type FC } from 'react';
import { useLocale } from '../../i18n/index.ts';
import { BreakerSubsection } from './BreakerSubsection.tsx';

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

export const RampReconcileSection: FC = () => {
  const { t } = useLocale();
  const [ramp, setRamp] = useState<RampState | null>(null);
  const [reconcile, setReconcile] = useState<DailyRecord | null>(null);
  const [rampError, setRampError] = useState<string | null>(null);
  const [reconcileMissing, setReconcileMissing] = useState(false);

  useEffect(() => {
    const token = localStorage.getItem('arb_token');
    const headers: Record<string, string> = token ? { Authorization: `Bearer ${token}` } : {};

    let cancelled = false;

    fetch('/api/pg/ramp', { headers })
      .then((r) => r.json() as Promise<ApiEnvelope<RampState>>)
      .then((j) => {
        if (cancelled) return;
        if (j.ok && j.data) {
          setRamp(j.data);
          setRampError(null);
        } else {
          setRampError(j.error || 'unable to load ramp state');
        }
      })
      .catch((err) => {
        if (!cancelled) setRampError(String(err));
      });

    const dateStr = yesterdayUTC();
    fetch(`/api/pg/reconcile/${dateStr}`, { headers })
      .then(async (r) => {
        if (r.status === 404) {
          if (!cancelled) setReconcileMissing(true);
          return null;
        }
        return r.json() as Promise<ApiEnvelope<DailyRecord>>;
      })
      .then((j) => {
        if (cancelled || !j) return;
        if (j.ok && j.data) {
          setReconcile(j.data);
        }
      })
      .catch(() => {
        // Silent: reconcile may simply not exist yet — UI shows the "no reconcile"
        // empty state. Real transport errors are surfaced through ramp's loading
        // path so the operator sees that the API is unhealthy.
      });

    return () => {
      cancelled = true;
    };
  }, []);

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

      {/* Phase 15 Plan 15-05 — Drawdown Circuit Breaker subsection (PG-LIVE-02).
          Renders status badge + recover/test-fire controls. Phase 16
          (PG-OPS-09) will lift this widget into the new top-level Pricegap
          tab without restructuring the BreakerSubsection component. */}
      <BreakerSubsection />
    </section>
  );
};

export default RampReconcileSection;
