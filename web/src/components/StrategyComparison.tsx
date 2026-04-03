import type { FC } from 'react';
import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  Tooltip,
  ResponsiveContainer,
  CartesianGrid,
  Legend,
} from 'recharts';
import { useLocale } from '../i18n/index.ts';
import type { StrategySummary } from '../types.ts';

interface StrategyComparisonProps {
  strategies: StrategySummary[];
}

function formatHoldTime(hours: number): string {
  if (hours < 24) return `${hours.toFixed(1)}h`;
  const days = Math.floor(hours / 24);
  const rem = hours % 24;
  return `${days}d ${rem.toFixed(0)}h`;
}

const CustomTooltip: FC<{ active?: boolean; payload?: Array<{ name: string; value: number; color: string }>; label?: string }> = ({
  active,
  payload,
  label,
}) => {
  if (!active || !payload || !payload.length) return null;
  return (
    <div
      style={{
        backgroundColor: '#1F2937',
        border: '1px solid #374151',
        borderRadius: '6px',
        padding: '8px 12px',
      }}
    >
      <p className="text-xs text-gray-400 mb-1">{label}</p>
      {payload.map((entry, i) => (
        <p key={i} className="text-sm font-mono" style={{ color: entry.color }}>
          {entry.name}: ${entry.value.toFixed(2)}
        </p>
      ))}
    </div>
  );
};

const StrategyComparison: FC<StrategyComparisonProps> = ({ strategies }) => {
  const { t } = useLocale();

  const perp = strategies.find((s) => s.strategy === 'perp');
  const spot = strategies.find((s) => s.strategy === 'spot');

  // Build bar chart data — single grouped bar comparing the two strategies
  const chartData = [
    {
      name: t('analytics.totalPnl'),
      perp: perp?.total_pnl ?? 0,
      spot: spot?.total_pnl ?? 0,
    },
    {
      name: t('analytics.profit'),
      perp: perp?.funding_total ?? 0,
      spot: spot?.funding_total ?? 0,
    },
  ];

  return (
    <div className="bg-gray-900 border border-gray-800 rounded-lg p-4">
      <h3 className="text-base font-bold text-gray-100 mb-4">{t('analytics.strategyComparison')}</h3>
      <ResponsiveContainer width="100%" height={300}>
        <BarChart data={chartData}>
          <CartesianGrid strokeDasharray="3 3" stroke="#374151" />
          <XAxis dataKey="name" stroke="#9CA3AF" tick={{ fontSize: 12 }} />
          <YAxis tickFormatter={(v: number) => `$${v}`} stroke="#9CA3AF" tick={{ fontSize: 12 }} />
          <Tooltip content={<CustomTooltip />} />
          <Legend
            wrapperStyle={{ fontSize: '12px', color: '#9CA3AF' }}
            formatter={(value: string) => <span className="text-xs text-gray-400">{value}</span>}
          />
          <Bar
            dataKey="perp"
            name={t('analytics.perpPerp')}
            fill="#60A5FA"
            radius={[4, 4, 0, 0]}
          />
          <Bar
            dataKey="spot"
            name={t('analytics.spotFutures')}
            fill="#C084FC"
            radius={[4, 4, 0, 0]}
          />
        </BarChart>
      </ResponsiveContainer>

      {/* Summary stats below chart */}
      <div className="grid grid-cols-2 gap-4 mt-4 text-sm">
        {[perp, spot].map((s) => {
          if (!s) return null;
          const label = s.strategy === 'perp' ? t('analytics.perpPerp') : t('analytics.spotFutures');
          const color = s.strategy === 'perp' ? 'text-blue-400' : 'text-purple-400';
          return (
            <div key={s.strategy} className="space-y-1">
              <div className={`font-bold ${color}`}>{label}</div>
              <div className="text-gray-400 text-xs">
                {t('analytics.totalPnl')}: <span className={s.total_pnl >= 0 ? 'text-green-400' : 'text-red-400'}>${s.total_pnl.toFixed(2)}</span>
              </div>
              <div className="text-gray-400 text-xs">
                {t('analytics.winRate')}: <span className="text-gray-100">{(s.win_rate * 100).toFixed(1)}%</span>
              </div>
              <div className="text-gray-400 text-xs">
                {t('analytics.apr')}: <span className="text-gray-100">{(s.apr * 100).toFixed(1)}%</span>
              </div>
              <div className="text-gray-400 text-xs">
                {t('analytics.avgHoldTime')}: <span className="text-gray-100">{formatHoldTime(s.avg_hold_hours)}</span>
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
};

export default StrategyComparison;
