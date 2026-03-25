import { useState, type FC } from 'react';
import type { Position, Stats, ExchangeInfo } from '../types.ts';
import StatusBadge from '../components/StatusBadge.tsx';
import { useLocale } from '../i18n/index.ts';

interface OverviewProps {
  positions: Position[];
  stats: Stats | null;
  exchanges: ExchangeInfo[];
  onDiagnose?: () => Promise<{ analysis: string }>;
}

function formatFundingCountdown(next: string | undefined): string {
  if (!next) return '-';
  const d = new Date(next);
  if (isNaN(d.getTime()) || d.getTime() === 0) return '-';
  const diff = d.getTime() - Date.now();
  if (diff <= 0) return 'passed';
  const mins = Math.floor(diff / 60000);
  if (mins < 60) return `${mins}m`;
  const hours = Math.floor(mins / 60);
  return `${hours}h ${mins % 60}m`;
}

const Overview: FC<OverviewProps> = ({ positions, stats, exchanges, onDiagnose }) => {
  const { t } = useLocale();
  const [diagnosing, setDiagnosing] = useState(false);
  const [diagnosis, setDiagnosis] = useState<string | null>(null);
  const [diagError, setDiagError] = useState<string | null>(null);

  const handleDiagnose = async () => {
    if (!onDiagnose) return;
    setDiagnosing(true);
    setDiagError(null);
    setDiagnosis(null);
    try {
      const result = await onDiagnose();
      setDiagnosis(result.analysis);
    } catch (err) {
      setDiagError(err instanceof Error ? err.message : t('ai.error'));
    } finally {
      setDiagnosing(false);
    }
  };

  const totalPnl = stats ? parseFloat(stats.total_pnl) : 0;
  const wins = stats ? parseInt(stats.win_count) : 0;
  const losses = stats ? parseInt(stats.loss_count) : 0;
  const trades = stats ? parseInt(stats.trade_count) : 0;
  const winRate = trades > 0 ? ((wins / trades) * 100).toFixed(1) : '0.0';

  const totalFunding = positions.reduce((sum, p) => sum + p.funding_collected, 0);
  const totalBalance = exchanges.reduce((sum, e) => {
    if (e.account_type === 'unified') return sum + e.balance; // unified: don't double-count spot
    return sum + e.balance + (e.spot_balance || 0);
  }, 0);

  const statCards = [
    { label: t('overview.totalPnl'), value: `$${totalPnl.toFixed(2)}`, color: totalPnl >= 0 ? 'text-green-400' : 'text-red-400' },
    { label: t('overview.winRate'), value: `${winRate}%`, sub: `${wins}W / ${losses}L of ${trades}`, color: 'text-blue-400' },
    { label: t('overview.activePositions'), value: String(positions.length), sub: `$${totalFunding.toFixed(2)} ${t('overview.funding')}`, color: 'text-yellow-400' },
    { label: t('overview.totalBalance'), value: `$${totalBalance.toFixed(2)}`, sub: `${exchanges.length} ${t('overview.exchanges')}`, color: 'text-gray-100' },
  ];

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h2 className="text-xl font-bold text-gray-100">{t('overview.title')}</h2>
        {onDiagnose && (
          <button
            onClick={handleDiagnose}
            disabled={diagnosing}
            className="px-3 py-1.5 text-sm bg-purple-600/20 text-purple-400 rounded hover:bg-purple-600/40 disabled:opacity-50"
          >
            {diagnosing ? t('ai.running') : t('ai.diagnose')}
          </button>
        )}
      </div>

      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        {statCards.map((card) => (
          <div key={card.label} className="bg-gray-900 border border-gray-800 rounded-lg p-4">
            <p className="text-sm text-gray-400">{card.label}</p>
            <p className={`text-2xl font-mono font-bold mt-1 ${card.color}`}>{card.value}</p>
            {card.sub && <p className="text-xs text-gray-500 mt-0.5">{card.sub}</p>}
          </div>
        ))}
      </div>

      {/* Exchange Balances */}
      {exchanges.length > 0 && (
        <div className="bg-gray-900 border border-gray-800 rounded-lg p-4">
          <h3 className="text-sm font-semibold text-gray-400 mb-3">{t('overview.exchangeBalances')}</h3>
          <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-5 gap-3">
            {exchanges.map((ex) => (
              <div key={ex.name} className="bg-gray-800/50 rounded-md px-3 py-2">
                <p className="text-xs text-gray-400 capitalize">{ex.name}</p>
                {ex.account_type === 'unified' ? (
                  <p className="font-mono text-sm text-gray-100">
                    <span className="text-gray-500 text-xs">{t('overview.unified')}: </span>${ex.balance.toFixed(2)}
                  </p>
                ) : (
                  <>
                    <p className="font-mono text-sm text-gray-100">
                      <span className="text-gray-500 text-xs">{t('overview.futures')}: </span>${ex.balance.toFixed(2)}
                    </p>
                    <p className="font-mono text-xs text-gray-400">
                      <span className="text-gray-500">{t('overview.spot')}: </span>${(ex.spot_balance || 0).toFixed(2)}
                    </p>
                  </>
                )}
              </div>
            ))}
          </div>
        </div>
      )}

      <div className="bg-gray-900 border border-gray-800 rounded-lg p-4 overflow-x-auto">
        <h3 className="text-sm font-semibold text-gray-400 mb-3">{t('overview.activePositionsSection')}</h3>
        {positions.length === 0 ? (
          <p className="text-gray-500 text-sm">{t('overview.noPositions')}</p>
        ) : (
          <table className="w-full text-sm">
            <thead>
              <tr className="text-gray-400 text-left border-b border-gray-800">
                <th className="pb-2">{t('overview.symbol')}</th>
                <th className="pb-2">{t('overview.long')}</th>
                <th className="pb-2">{t('overview.short')}</th>
                <th className="pb-2">{t('overview.status')}</th>
                <th className="pb-2 text-right">{t('overview.spread')}</th>
                <th className="pb-2 text-right">{t('overview.fundingCol')}</th>
                <th className="pb-2 text-right">{t('overview.nextFund')}</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-800">
              {positions.map((p) => (
                <tr key={p.id} className="text-gray-100">
                  <td className="py-2 font-mono">
                    {p.symbol}
                    {(p.rotation_count ?? 0) > 0 && (
                      <span className="ml-1 text-xs text-yellow-500">R{p.rotation_count}</span>
                    )}
                  </td>
                  <td className="py-2 text-green-400">{p.long_exchange}</td>
                  <td className="py-2 text-red-400">{p.short_exchange}</td>
                  <td className="py-2"><StatusBadge status={p.status} /></td>
                  <td className="py-2 text-right font-mono">{p.entry_spread.toFixed(1)} bps/h</td>
                  <td className="py-2 text-right font-mono">${p.funding_collected.toFixed(2)}</td>
                  <td className="py-2 text-right font-mono text-gray-400 text-xs">
                    {formatFundingCountdown(p.next_funding)}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      {/* AI Diagnosis Modal */}
      {(diagnosis || diagError) && (
        <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-50 p-4">
          <div className="bg-gray-900 border border-gray-700 rounded-lg p-6 max-w-3xl w-full max-h-[80vh] flex flex-col">
            <div className="flex items-center justify-between mb-4">
              <h3 className="text-lg font-semibold text-gray-100">{t('ai.diagnose')}</h3>
              <button
                onClick={() => { setDiagnosis(null); setDiagError(null); }}
                className="px-3 py-1 text-sm bg-gray-700 text-gray-300 rounded hover:bg-gray-600"
              >
                {t('ai.close')}
              </button>
            </div>
            {diagError && <p className="text-red-400 text-sm">{diagError}</p>}
            {diagnosis && (
              <pre className="text-sm text-gray-200 whitespace-pre-wrap overflow-y-auto flex-1 bg-gray-800/50 rounded p-4">
                {diagnosis}
              </pre>
            )}
          </div>
        </div>
      )}

      <p className="text-xs text-gray-600 text-center mt-2">
        Funding rate data provided by <a href="https://loris.tools" target="_blank" rel="noopener noreferrer" className="text-gray-500 hover:text-gray-400 underline">Loris Tools</a>
      </p>
    </div>
  );
};

export default Overview;
