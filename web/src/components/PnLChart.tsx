import type { FC } from 'react';
import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  Tooltip,
  Brush,
  ResponsiveContainer,
  CartesianGrid,
} from 'recharts';
import { useLocale } from '../i18n/index.ts';
import type { PnLSnapshot } from '../types.ts';

interface PnLChartProps {
  data: PnLSnapshot[];
}

function formatDate(ts: number): string {
  const d = new Date(ts * 1000);
  const mm = String(d.getMonth() + 1).padStart(2, '0');
  const dd = String(d.getDate()).padStart(2, '0');
  return `${mm}/${dd}`;
}

function formatDateFull(ts: number): string {
  const d = new Date(ts * 1000);
  const yyyy = d.getFullYear();
  const mm = String(d.getMonth() + 1).padStart(2, '0');
  const dd = String(d.getDate()).padStart(2, '0');
  const hh = String(d.getHours()).padStart(2, '0');
  const min = String(d.getMinutes()).padStart(2, '0');
  return `${yyyy}-${mm}-${dd} ${hh}:${min}`;
}

function formatDollar(v: number): string {
  return `$${v.toFixed(2)}`;
}

const CustomTooltip: FC<{ active?: boolean; payload?: Array<{ value: number }>; label?: number }> = ({
  active,
  payload,
  label,
}) => {
  if (!active || !payload || !payload.length || label == null) return null;
  return (
    <div
      style={{
        backgroundColor: '#1F2937',
        border: '1px solid #374151',
        borderRadius: '6px',
        padding: '8px 12px',
      }}
    >
      <p className="text-xs text-gray-400 mb-1">{formatDateFull(label)}</p>
      <p className="text-sm text-green-400 font-mono">{formatDollar(payload[0].value)}</p>
    </div>
  );
};

const PnLChart: FC<PnLChartProps> = ({ data }) => {
  const { t } = useLocale();

  if (data.length === 0) {
    return (
      <div className="bg-gray-900 border border-gray-800 rounded-lg p-4 flex items-center justify-center" style={{ height: 400 }}>
        <p className="text-sm text-gray-500">{t('analytics.noData')}</p>
      </div>
    );
  }

  // Compute running cumulative PnL from per-snapshot values
  const sorted = [...data].sort((a, b) => a.timestamp - b.timestamp);
  let runningTotal = 0;
  const cumulativeData = sorted.map((snap) => {
    runningTotal += snap.cumulative_pnl;
    return { ...snap, cumulative_pnl: runningTotal };
  });

  return (
    <div className="bg-gray-900 border border-gray-800 rounded-lg p-4">
      <h3 className="text-base font-bold text-gray-100 mb-4">{t('analytics.cumulativePnl')}</h3>
      <ResponsiveContainer width="100%" height={400}>
        <LineChart data={cumulativeData}>
          <CartesianGrid strokeDasharray="3 3" stroke="#374151" />
          <XAxis
            dataKey="timestamp"
            tickFormatter={formatDate}
            stroke="#9CA3AF"
            tick={{ fontSize: 12 }}
          />
          <YAxis
            tickFormatter={(v: number) => `$${v}`}
            stroke="#9CA3AF"
            tick={{ fontSize: 12 }}
          />
          <Tooltip content={<CustomTooltip />} />
          <Brush
            dataKey="timestamp"
            height={30}
            stroke="#4B5563"
            tickFormatter={formatDate}
          />
          <Line
            type="monotone"
            dataKey="cumulative_pnl"
            stroke="#34D399"
            dot={false}
            strokeWidth={2}
          />
        </LineChart>
      </ResponsiveContainer>
    </div>
  );
};

export default PnLChart;
