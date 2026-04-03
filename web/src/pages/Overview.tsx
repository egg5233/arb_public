import { useState, type FC } from 'react';
import type { Position, Stats, ExchangeInfo, SpotPosition, LossLimitStatus } from '../types.ts';
import StatusBadge from '../components/StatusBadge.tsx';
import { useLocale } from '../i18n/index.ts';

interface OverviewProps {
  positions: Position[];
  stats: Stats | null;
  exchanges: ExchangeInfo[];
  onDiagnose?: () => Promise<{ analysis: string }>;
  onResolveSpotPosition?: (positionId: string) => Promise<void>;
  spotPositions?: SpotPosition[];
  lossLimits?: LossLimitStatus | null;
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

const Overview: FC<OverviewProps> = ({
  positions,
  stats,
  exchanges,
  onDiagnose,
  onResolveSpotPosition,
  spotPositions = [],
  lossLimits,
}) => {
  const { t } = useLocale();
  const [diagnosing, setDiagnosing] = useState(false);
  const [diagnosis, setDiagnosis] = useState<string | null>(null);
  const [diagError, setDiagError] = useState<string | null>(null);
  const [resolvingSpotId, setResolvingSpotId] = useState<string | null>(null);
  const [spotResolveErrorId, setSpotResolveErrorId] = useState<string | null>(null);
  const [spotResolveError, setSpotResolveError] = useState<string | null>(null);

  const activeSpotPositions = spotPositions.filter((p) =>
    p.status === 'active' ||
    p.status === 'exiting' ||
    (p.status === 'pending' && p.exit_reason === 'manual_intervention_required')
  );

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

  const handleResolveSpotPosition = async (positionId: string) => {
    if (!onResolveSpotPosition) return;
    setResolvingSpotId(positionId);
    setSpotResolveErrorId(null);
    setSpotResolveError(null);
    try {
      await onResolveSpotPosition(positionId);
    } catch (err) {
      setSpotResolveErrorId(positionId);
      setSpotResolveError(err instanceof Error ? err.message : 'Resolve failed');
    } finally {
      setResolvingSpotId(null);
    }
  };

  const totalPnl = stats ? parseFloat(stats.total_pnl || '0') : 0;
  const wins = stats ? parseInt(stats.win_count || '0') : 0;
  const losses = stats ? parseInt(stats.loss_count || '0') : 0;
  const trades = stats ? parseInt(stats.trade_count || '0') : 0;
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

      {lossLimits && lossLimits.enabled && (
        <div className={`rounded-lg p-3 border ${
          lossLimits.breached
            ? 'bg-red-900/30 border-red-700 text-red-300'
            : (lossLimits.daily_limit > 0 && Math.abs(lossLimits.daily_loss) / lossLimits.daily_limit > 0.8) ||
              (lossLimits.weekly_limit > 0 && Math.abs(lossLimits.weekly_loss) / lossLimits.weekly_limit > 0.8)
              ? 'bg-yellow-900/30 border-yellow-700 text-yellow-300'
              : 'bg-gray-900 border-gray-800 text-gray-300'
        }`}>
          <div className="flex items-center justify-between text-sm">
            <span className="font-medium">{t('overview.lossLimits')}</span>
            <div className="flex gap-4 font-mono text-xs">
              <span>{t('overview.daily')}: ${Math.abs(lossLimits.daily_loss).toFixed(2)} / ${lossLimits.daily_limit.toFixed(2)}</span>
              <span>{t('overview.weekly')}: ${Math.abs(lossLimits.weekly_loss).toFixed(2)} / ${lossLimits.weekly_limit.toFixed(2)}</span>
            </div>
          </div>
          {lossLimits.breached && (
            <div className="text-xs mt-1 font-semibold">{t('overview.lossLimitBreached')}</div>
          )}
        </div>
      )}

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

      {/* Spot-Futures Active Positions */}
      {activeSpotPositions.length > 0 && (
        <div className="bg-gray-900 border border-gray-800 rounded-lg p-4">
          <h3 className="text-sm font-semibold text-gray-400 mb-3">Spot-Futures Positions</h3>
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
            {activeSpotPositions.map((pos) => {
              const netYield = pos.yield_data_source && pos.yield_data_source !== 'entry_fallback'
                ? pos.current_net_yield_apr
                : pos.funding_apr - pos.current_borrow_apr;
              const isFallback = pos.yield_data_source === 'entry_fallback';
              const duration = Math.floor((Date.now() - new Date(pos.created_at).getTime()) / 3600000);
              const dir = pos.direction === 'borrow_sell_long' ? 'A' : 'B';
              const isManualRecovery = pos.status === 'pending' && pos.exit_reason === 'manual_intervention_required';
              return (
                <div key={pos.id} className="bg-gray-800/50 rounded-md px-3 py-2 space-y-1">
                  <div className="flex items-center justify-between">
                    <span className="font-mono text-sm text-gray-100">{pos.symbol}</span>
                    <span className={`text-xs px-1.5 py-0.5 rounded ${
                      pos.status === 'exiting'
                        ? 'bg-red-900/50 text-red-400'
                        : isManualRecovery
                          ? 'bg-yellow-900/50 text-yellow-300'
                          : 'bg-green-900/50 text-green-400'
                    }`}>
                      {isManualRecovery ? `pending recovery (Dir ${dir})` : `${pos.status} (Dir ${dir})`}
                    </span>
                  </div>
                  <div className="text-xs text-gray-400 capitalize">{pos.exchange} &middot; {duration}h held</div>
                  {isManualRecovery && (
                    <div className="rounded border border-yellow-800/60 bg-yellow-950/40 px-2 py-1 text-xs text-yellow-200">
                      Manual intervention required. After the exchange is flat, clear this record to release allocator exposure.
                    </div>
                  )}
                  <div className="grid grid-cols-2 gap-x-3 text-xs">
                    <div>
                      <span className="text-gray-500">Borrow APR</span>
                      <span className={`ml-1 font-mono ${pos.current_borrow_apr > 0.3 ? 'text-red-400' : 'text-gray-300'}`}>
                        {(pos.current_borrow_apr * 100).toFixed(1)}%
                      </span>
                    </div>
                    <div>
                      <span className="text-gray-500">Net Yield{isFallback ? ' *' : ''}</span>
                      <span className={`ml-1 font-mono ${netYield < 0 ? 'text-red-400' : 'text-green-400'}`} title={isFallback ? 'Entry-time estimate (live scan unavailable)' : 'Live scan'}>
                        {(netYield * 100).toFixed(1)}%
                      </span>
                    </div>
                    <div>
                      <span className="text-gray-500">Borrow Cost</span>
                      <span className="ml-1 font-mono text-gray-300">${pos.borrow_cost_accrued.toFixed(2)}</span>
                    </div>
                    <div>
                      <span className="text-gray-500">Notional</span>
                      <span className="ml-1 font-mono text-gray-300">${pos.notional_usdt.toFixed(0)}</span>
                    </div>
                    {pos.margin_utilization_pct > 0 && (
                      <div>
                        <span className="text-gray-500">Margin</span>
                        <span className={`ml-1 font-mono ${pos.margin_utilization_pct > 85 ? 'text-red-400' : pos.margin_utilization_pct > 70 ? 'text-yellow-400' : 'text-gray-300'}`}>
                          {pos.margin_utilization_pct.toFixed(0)}%
                        </span>
                      </div>
                    )}
                    {pos.peak_price_move_pct > 5 && (
                      <div>
                        <span className="text-gray-500">Peak Move</span>
                        <span className={`ml-1 font-mono ${pos.peak_price_move_pct > 20 ? 'text-red-400' : 'text-yellow-400'}`}>
                          {pos.peak_price_move_pct.toFixed(1)}%
                        </span>
                      </div>
                    )}
                  </div>
                  {isManualRecovery && onResolveSpotPosition && (
                    <div className="pt-1">
                      <button
                        onClick={() => void handleResolveSpotPosition(pos.id)}
                        disabled={resolvingSpotId === pos.id}
                        className="rounded bg-yellow-900/60 px-2 py-1 text-xs text-yellow-100 hover:bg-yellow-800/70 disabled:cursor-not-allowed disabled:opacity-60"
                      >
                        {resolvingSpotId === pos.id ? 'Clearing...' : 'Clear After Flatten'}
                      </button>
                      {spotResolveError && spotResolveErrorId === pos.id && (
                        <p className="mt-1 text-xs text-red-400">{spotResolveError}</p>
                      )}
                    </div>
                  )}
                </div>
              );
            })}
          </div>
        </div>
      )}

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
