// Phase 11 Plan 06 — ScoreHistoryCard.
//
// Symbol-selector dropdown + Recharts <LineChart> with <ReferenceArea>
// threshold-band overlay reading PriceGapAutoPromoteScore.  Reuses
// Recharts subcomponents already locked via PnLChart.tsx import — no
// new npm dependency.
//
// Empty states (UI-SPEC §"Empty states"):
//   - score_snapshot empty       → render noUniverse copy
//   - symbol selected, no points → render "No score history yet"
//   - loading symbol             → small inline spinner copy
import { useEffect, useState } from 'react';
import type { FC } from 'react';
import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  Tooltip,
  ResponsiveContainer,
  CartesianGrid,
  ReferenceArea,
} from 'recharts';
import { useLocale } from '../../i18n/index.ts';
import type {
  DiscoveryState,
  ScoresResponse,
} from '../../hooks/usePgDiscovery.ts';

export interface ScoreHistoryCardProps {
  state: DiscoveryState | null;
  scores: Record<string, ScoresResponse>;
  loadingSymbol: string | null;
  onSelectSymbol: (symbol: string) => void;
}

function formatTime(ts: number): string {
  if (!Number.isFinite(ts) || ts <= 0) return '';
  const d = new Date(ts * 1000);
  if (Number.isNaN(d.getTime())) return '';
  const mm = String(d.getMonth() + 1).padStart(2, '0');
  const dd = String(d.getDate()).padStart(2, '0');
  const hh = String(d.getHours()).padStart(2, '0');
  return `${mm}/${dd} ${hh}h`;
}

function uniqueSymbols(state: DiscoveryState | null): string[] {
  if (!state || !Array.isArray(state.score_snapshot)) return [];
  const seen = new Set<string>();
  for (const row of state.score_snapshot) {
    if (row && typeof row.symbol === 'string') seen.add(row.symbol);
  }
  return Array.from(seen).sort();
}

export const ScoreHistoryCard: FC<ScoreHistoryCardProps> = ({
  state,
  scores,
  loadingSymbol,
  onSelectSymbol,
}) => {
  const { t } = useLocale();
  const [selected, setSelected] = useState<string>('');

  const symbols = uniqueSymbols(state);

  // Auto-pick first symbol once the universe arrives so the chart isn't blank
  // when the operator opens the page mid-cycle.
  useEffect(() => {
    if (selected) return;
    if (symbols.length === 0) return;
    const first = symbols[0]!;
    setSelected(first);
    onSelectSymbol(first);
  }, [symbols, selected, onSelectSymbol]);

  const panelCls = 'bg-gray-900 border border-gray-800 rounded-lg p-4';
  const sel = selected && scores[selected] ? scores[selected]! : null;
  const isLoading = loadingSymbol != null && loadingSymbol === selected;

  // Empty universe — operator hasn't configured PriceGapDiscoveryUniverse.
  if (symbols.length === 0) {
    return (
      <div className={panelCls} data-testid="score-history-card">
        <h3 className="text-base font-bold text-gray-100 mb-3">
          {t('pricegap.discovery.scoreHistory.title')}
        </h3>
        <p
          className="text-xs text-gray-500"
          data-testid="score-history-no-universe"
        >
          {t('pricegap.discovery.scoreHistory.noUniverse')}
        </p>
      </div>
    );
  }

  const handleChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
    const value = e.target.value;
    setSelected(value);
    if (value) onSelectSymbol(value);
  };

  return (
    <div className={panelCls} data-testid="score-history-card">
      <div className="flex items-center justify-between mb-3 gap-3">
        <h3 className="text-base font-bold text-gray-100">
          {t('pricegap.discovery.scoreHistory.title')}
        </h3>
        <label className="flex items-center gap-2 text-xs text-gray-300">
          <span className="text-gray-400">
            {t('pricegap.discovery.scoreHistory.symbolSelector')}
          </span>
          <select
            value={selected}
            onChange={handleChange}
            className="bg-gray-800 border border-gray-700 rounded px-2 py-1 text-xs text-gray-100"
            data-testid="score-history-selector"
          >
            <option value="">
              {t('pricegap.discovery.scoreHistory.selectPrompt')}
            </option>
            {symbols.map((s) => (
              <option key={s} value={s}>
                {s}
              </option>
            ))}
          </select>
        </label>
      </div>

      {isLoading && (
        <div
          className="text-xs text-gray-500"
          data-testid="score-history-loading"
        >
          {t('pricegap.discovery.scoreHistory.loading')}
        </div>
      )}

      {!isLoading && selected && (!sel || sel.points.length === 0) && (
        <div
          className="text-xs text-gray-500"
          data-testid="score-history-empty"
        >
          {t('pricegap.discovery.scoreHistory.empty')}
        </div>
      )}

      {!isLoading && sel && sel.points.length > 0 && (
        <div data-testid="score-history-chart">
          <ResponsiveContainer width="100%" height={300}>
            <LineChart data={sel.points}>
              <CartesianGrid strokeDasharray="3 3" stroke="#374151" />
              <XAxis
                dataKey="ts"
                tickFormatter={formatTime}
                stroke="#9CA3AF"
                tick={{ fontSize: 12 }}
              />
              <YAxis
                domain={[0, 100]}
                stroke="#9CA3AF"
                tick={{ fontSize: 12 }}
              />
              <Tooltip
                contentStyle={{
                  backgroundColor: '#1F2937',
                  border: '1px solid #374151',
                  borderRadius: '6px',
                  fontSize: '12px',
                }}
                labelFormatter={(label: unknown) =>
                  typeof label === 'number' ? formatTime(label) : String(label ?? '')
                }
              />
              {sel.threshold_band && sel.threshold_band.auto_promote > 0 && (
                <ReferenceArea
                  y1={sel.threshold_band.auto_promote}
                  y2={100}
                  fill="#f0b90b"
                  fillOpacity={0.1}
                  stroke="#f0b90b"
                  strokeOpacity={0.4}
                  label={{
                    value: t('pricegap.discovery.scoreHistory.thresholdBand'),
                    position: 'insideTopRight',
                    fill: '#f0b90b',
                    fontSize: 10,
                  }}
                />
              )}
              <Line
                type="monotone"
                dataKey="score"
                stroke="#0ecb81"
                dot={false}
                strokeWidth={2}
              />
            </LineChart>
          </ResponsiveContainer>
        </div>
      )}
    </div>
  );
};

export default ScoreHistoryCard;
