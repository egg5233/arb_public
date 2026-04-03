import { useState, useEffect, useCallback } from 'react';
import type { FC } from 'react';
import { useLocale } from '../i18n/index.ts';
import type { PnLSnapshot, StrategySummary, ExchangeMetric } from '../types.ts';
import TimeRangeSelector from '../components/TimeRangeSelector.tsx';
import PnLChart from '../components/PnLChart.tsx';
import StrategyComparison from '../components/StrategyComparison.tsx';
import ExchangeMetrics from '../components/ExchangeMetrics.tsx';

interface AnalyticsProps {
  getAnalyticsPnL: (from: number, to: number, strategy: string) => Promise<PnLSnapshot[]>;
  getAnalyticsSummary: (from: number, to: number) => Promise<{ strategies: StrategySummary[]; exchange_metrics: ExchangeMetric[] }>;
}

function rangeToSeconds(range: string): number {
  switch (range) {
    case '7d': return 7 * 24 * 3600;
    case '30d': return 30 * 24 * 3600;
    case '90d': return 90 * 24 * 3600;
    default: return 0; // 'all'
  }
}

const Analytics: FC<AnalyticsProps> = ({ getAnalyticsPnL, getAnalyticsSummary }) => {
  const { t } = useLocale();
  const [timeRange, setTimeRange] = useState('30d');
  const [pnlData, setPnlData] = useState<PnLSnapshot[]>([]);
  const [strategies, setStrategies] = useState<StrategySummary[]>([]);
  const [exchangeMetrics, setExchangeMetrics] = useState<ExchangeMetric[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchData = useCallback(async (range: string) => {
    setLoading(true);
    setError(null);
    try {
      const now = Math.floor(Date.now() / 1000);
      const seconds = rangeToSeconds(range);
      const from = seconds > 0 ? now - seconds : 0;

      const [pnl, summary] = await Promise.all([
        getAnalyticsPnL(from, now, 'all'),
        getAnalyticsSummary(from, now),
      ]);

      setPnlData(pnl);
      setStrategies(summary.strategies || []);
      setExchangeMetrics(summary.exchange_metrics || []);
    } catch {
      setError(t('analytics.loadError'));
    } finally {
      setLoading(false);
    }
  }, [getAnalyticsPnL, getAnalyticsSummary, t]);

  useEffect(() => {
    fetchData(timeRange);
  }, [timeRange, fetchData]);

  // Compute aggregate stats
  const totalPnl = strategies.reduce((sum, s) => sum + s.total_pnl, 0);
  const totalTrades = strategies.reduce((sum, s) => sum + s.trade_count, 0);
  const totalWins = strategies.reduce((sum, s) => sum + s.win_count, 0);
  const winRate = totalTrades > 0 ? totalWins / totalTrades : 0;
  const avgApr = strategies.length > 0 ? strategies.reduce((sum, s) => sum + s.apr, 0) / strategies.length : 0;
  const avgHold = strategies.length > 0 ? strategies.reduce((sum, s) => sum + s.avg_hold_hours, 0) / strategies.length : 0;

  function formatHold(hours: number): string {
    if (hours < 24) return `${hours.toFixed(1)}h`;
    const days = Math.floor(hours / 24);
    const rem = hours % 24;
    return `${days}d ${rem.toFixed(0)}h`;
  }

  if (loading) {
    return (
      <div className="space-y-8">
        <div className="flex items-center justify-between">
          <h2 className="text-xl font-bold text-gray-100">{t('analytics.title')}</h2>
          <TimeRangeSelector selected={timeRange} onChange={setTimeRange} />
        </div>
        <div className="bg-gray-800 animate-pulse rounded-lg" style={{ height: 400 }} />
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-8">
          <div className="bg-gray-800 animate-pulse rounded-lg" style={{ height: 300 }} />
          <div className="bg-gray-800 animate-pulse rounded-lg" style={{ height: 300 }} />
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="space-y-8">
        <div className="flex items-center justify-between">
          <h2 className="text-xl font-bold text-gray-100">{t('analytics.title')}</h2>
          <TimeRangeSelector selected={timeRange} onChange={setTimeRange} />
        </div>
        <div className="bg-gray-900 border border-gray-800 rounded-lg p-8 text-center">
          <p className="text-sm text-red-400">{error}</p>
        </div>
      </div>
    );
  }

  if (pnlData.length === 0 && strategies.length === 0) {
    return (
      <div className="space-y-8">
        <div className="flex items-center justify-between">
          <h2 className="text-xl font-bold text-gray-100">{t('analytics.title')}</h2>
          <TimeRangeSelector selected={timeRange} onChange={setTimeRange} />
        </div>
        <div className="bg-gray-900 border border-gray-800 rounded-lg p-12 text-center">
          <p className="text-lg text-gray-400 mb-2">{t('analytics.noData')}</p>
          <p className="text-sm text-gray-500">{t('analytics.noDataBody')}</p>
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-8">
      {/* Header */}
      <div className="flex items-center justify-between">
        <h2 className="text-xl font-bold text-gray-100">{t('analytics.title')}</h2>
        <TimeRangeSelector selected={timeRange} onChange={setTimeRange} />
      </div>

      {/* Cumulative PnL Chart */}
      <PnLChart data={pnlData} />

      {/* Strategy Comparison + Exchange Metrics */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-8">
        <StrategyComparison strategies={strategies} />
        <ExchangeMetrics metrics={exchangeMetrics} />
      </div>

      {/* Summary stat cards */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        <div className="bg-gray-900 border border-gray-800 rounded-lg p-4">
          <div className="text-xs text-gray-400 mb-1">{t('analytics.totalPnl')}</div>
          <div className={`text-lg font-bold ${totalPnl >= 0 ? 'text-green-400' : 'text-red-400'}`}>
            {totalPnl >= 0 ? '+' : ''}${totalPnl.toFixed(2)}
          </div>
        </div>
        <div className="bg-gray-900 border border-gray-800 rounded-lg p-4">
          <div className="text-xs text-gray-400 mb-1">{t('analytics.apr')}</div>
          <div className="text-lg font-bold text-gray-100">{avgApr.toFixed(1)}%</div>
        </div>
        <div className="bg-gray-900 border border-gray-800 rounded-lg p-4">
          <div className="text-xs text-gray-400 mb-1">{t('analytics.winRate')}</div>
          <div className="text-lg font-bold text-gray-100">{(winRate * 100).toFixed(1)}%</div>
        </div>
        <div className="bg-gray-900 border border-gray-800 rounded-lg p-4">
          <div className="text-xs text-gray-400 mb-1">{t('analytics.avgHoldTime')}</div>
          <div className="text-lg font-bold text-gray-100">{formatHold(avgHold)}</div>
        </div>
      </div>
    </div>
  );
};

export default Analytics;
