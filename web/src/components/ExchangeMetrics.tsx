import type { FC } from 'react';
import { useLocale } from '../i18n/index.ts';
import type { ExchangeMetric } from '../types.ts';

interface ExchangeMetricsProps {
  metrics: ExchangeMetric[];
}

const ExchangeMetrics: FC<ExchangeMetricsProps> = ({ metrics }) => {
  const { t } = useLocale();

  // Sort by profit descending
  const sorted = [...metrics].sort((a, b) => b.profit - a.profit);
  const maxProfit = sorted.length > 0 ? Math.max(...sorted.map((m) => Math.abs(m.profit)), 1) : 1;

  return (
    <div className="bg-gray-900 border border-gray-800 rounded-lg p-4">
      <h3 className="text-base font-bold text-gray-100 mb-4">{t('analytics.exchangePerformance')}</h3>
      <div className="overflow-x-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="text-gray-400 text-xs text-left border-b border-gray-800">
              <th className="pb-2">{t('analytics.exchange')}</th>
              <th className="pb-2 text-right">{t('analytics.profit')}</th>
              <th className="pb-2 text-right">{t('analytics.apr')}</th>
              <th className="pb-2 text-right">{t('analytics.winRate')}</th>
              <th className="pb-2 text-right">{t('analytics.trades')}</th>
              <th className="pb-2 text-right">{t('analytics.avgSlippage')}</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-800">
            {sorted.map((m) => {
              const barWidth = Math.min((Math.abs(m.profit) / maxProfit) * 100, 100);
              const barColor = m.profit >= 0 ? 'bg-green-500/60' : 'bg-red-500/60';
              const profitColor = m.profit >= 0 ? 'text-green-400' : 'text-red-400';

              return (
                <tr key={m.exchange} className="text-gray-100 hover:bg-gray-800/50 transition-colors">
                  <td className="py-2 capitalize">{m.exchange}</td>
                  <td className="py-2 text-right">
                    <div className="flex items-center justify-end gap-2">
                      <span className={`font-mono ${profitColor}`}>
                        {m.profit >= 0 ? '+' : ''}${m.profit.toFixed(2)}
                      </span>
                      <div className="w-[120px] bg-gray-700 rounded-full h-2">
                        <div
                          className={`h-2 rounded-full ${barColor}`}
                          style={{ width: `${barWidth}%` }}
                        />
                      </div>
                    </div>
                  </td>
                  <td className="py-2 text-right font-mono text-gray-300">{m.apr.toFixed(1)}%</td>
                  <td className="py-2 text-right font-mono text-gray-300">{m.win_rate.toFixed(1)}%</td>
                  <td className="py-2 text-right font-mono text-gray-400">{m.trade_count}</td>
                  <td className="py-2 text-right font-mono text-gray-400">{m.avg_slippage.toFixed(2)} bps</td>
                </tr>
              );
            })}
            {sorted.length === 0 && (
              <tr>
                <td colSpan={6} className="py-4 text-center text-gray-500">{t('analytics.noData')}</td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
};

export default ExchangeMetrics;
