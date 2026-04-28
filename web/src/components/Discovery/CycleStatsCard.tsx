// Phase 11 Plan 06 — CycleStatsCard.
//
// Renders a 7-tile responsive stat grid showing the most recent scanner
// cycle metrics (last_run, next_run_in, candidates_seen, accepted, rejected,
// errors, duration).  All numeric values use `font-mono tabular-nums` so
// columns align across cards (UI-SPEC §Typography).
//
// Empty / disabled state: when state is null OR last_run_at is 0, numeric
// tiles render "—" and the Last-run tile renders the i18n "Never" key.
import type { FC } from 'react';
import { useLocale } from '../../i18n/index.ts';
import type { DiscoveryState } from '../../hooks/usePgDiscovery.ts';

export interface CycleStatsCardProps {
  state: DiscoveryState | null;
}

function formatLastRun(ts: number): string {
  if (!Number.isFinite(ts) || ts <= 0) return '';
  // Asia/Taipei (UTC+8) per CLAUDE.local.md memory; use sv-SE for ISO-like format.
  const d = new Date(ts * 1000);
  if (Number.isNaN(d.getTime())) return '';
  const utc = d
    .toLocaleString('sv-SE', { timeZone: 'UTC' })
    .replace('T', ' ');
  return `${utc} UTC`;
}

function formatNextRunIn(secs: number | undefined): string {
  if (!Number.isFinite(secs) || secs == null || secs <= 0) return '—';
  const s = Math.floor(secs);
  if (s < 60) return `${s}s`;
  const m = Math.floor(s / 60);
  if (m < 60) return `${m}m ${s % 60}s`;
  const h = Math.floor(m / 60);
  return `${h}h ${m % 60}m`;
}

function formatDurationMs(ms: number | undefined): string {
  if (!Number.isFinite(ms) || ms == null || ms < 0) return '—';
  if (ms < 1000) return `${Math.round(ms)}ms`;
  const s = Math.round(ms / 100) / 10; // one decimal
  if (s < 60) return `${s.toFixed(1)}s`;
  const mins = Math.floor(s / 60);
  return `${mins}m ${Math.round(s % 60)}s`;
}

const tileNumeric = 'text-lg font-bold font-mono tabular-nums text-gray-100';
const tileLabel = 'text-xs text-gray-400';

const Tile: FC<{ label: string; value: string; testId: string }> = ({
  label,
  value,
  testId,
}) => (
  <div className="p-3" data-testid={testId}>
    <div className={tileLabel}>{label}</div>
    <div className={tileNumeric}>{value}</div>
  </div>
);

export const CycleStatsCard: FC<CycleStatsCardProps> = ({ state }) => {
  const { t } = useLocale();
  const panelCls = 'bg-gray-900 border border-gray-800 rounded-lg p-4';

  const last = state?.last_run_at && state.last_run_at > 0
    ? formatLastRun(state.last_run_at)
    : t('pricegap.discovery.cycle.never');
  const numeric = (v: number | undefined): string =>
    state == null || v == null ? '—' : String(v);

  return (
    <div className={panelCls} data-testid="cycle-stats-card">
      <h3 className="text-base font-bold text-gray-100 mb-3">
        {t('pricegap.discovery.cycle.title')}
      </h3>
      <div className="grid grid-cols-2 sm:grid-cols-4 lg:grid-cols-7 gap-4">
        <Tile
          label={t('pricegap.discovery.cycle.lastRun')}
          value={last}
          testId="tile-last-run"
        />
        <Tile
          label={t('pricegap.discovery.cycle.nextRunIn')}
          value={formatNextRunIn(state?.next_run_in)}
          testId="tile-next-run-in"
        />
        <Tile
          label={t('pricegap.discovery.cycle.candidatesSeen')}
          value={numeric(state?.candidates_seen)}
          testId="tile-candidates-seen"
        />
        <Tile
          label={t('pricegap.discovery.cycle.accepted')}
          value={numeric(state?.accepted)}
          testId="tile-accepted"
        />
        <Tile
          label={t('pricegap.discovery.cycle.rejected')}
          value={numeric(state?.rejected)}
          testId="tile-rejected"
        />
        <Tile
          label={t('pricegap.discovery.cycle.errors')}
          value={numeric(state?.errors)}
          testId="tile-errors"
        />
        <Tile
          label={t('pricegap.discovery.cycle.duration')}
          value={formatDurationMs(state?.duration_ms)}
          testId="tile-duration"
        />
      </div>
    </div>
  );
};

export default CycleStatsCard;
