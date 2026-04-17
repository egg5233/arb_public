import { useState, useEffect, useCallback, Fragment } from 'react';
import type { FC } from 'react';
import type { Position } from '../types.ts';
import { useLocale } from '../i18n/index.ts';
import { ExchangeLink } from '../utils/tradingUrl.tsx';
import PnLBreakdown from '../components/PnLBreakdown.tsx';

interface HistoryProps {
  getHistory: (limit: number) => Promise<Position[]>;
}

function formatHoldDuration(created: string, updated: string): string {
  const start = new Date(created).getTime();
  const end = updated ? new Date(updated).getTime() : Date.now();
  if (isNaN(start) || isNaN(end)) return '-';
  const diff = end - start;
  if (diff < 0) return '-';
  const hours = Math.floor(diff / 3600000);
  if (hours < 24) return `${hours}h`;
  const days = Math.floor(hours / 24);
  return `${days}d ${hours % 24}h`;
}

const History: FC<HistoryProps> = ({ getHistory }) => {
  const { t } = useLocale();
  const [trades, setTrades] = useState<Position[]>([]);
  const [limit, setLimit] = useState(50);
  const [loading, setLoading] = useState(false);
  const [expandedRow, setExpandedRow] = useState<string | null>(null);
  const [statusFilter, setStatusFilter] = useState<'all' | 'success' | 'failed'>('all');

  const load = useCallback(async (n: number) => {
    setLoading(true);
    try {
      const data = await getHistory(n);
      setTrades(data);
    } catch {
      // ignore
    } finally {
      setLoading(false);
    }
  }, [getHistory]);

  useEffect(() => {
    load(limit);
  }, [load, limit]);

  const loadMore = () => {
    setLimit((prev) => prev + 50);
  };

  const filteredTrades = trades.filter((tr) => {
    if (statusFilter === 'success') return !tr.failure_reason;
    if (statusFilter === 'failed') return !!tr.failure_reason;
    return true;
  });

  return (
    <div className="space-y-6">
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3">
        <h2 className="text-xl font-bold text-gray-100">{t('hist.title')}</h2>
        <div className="flex gap-1 bg-gray-800 rounded-lg p-1 self-start">
          {(['all', 'success', 'failed'] as const).map((f) => (
            <button
              key={f}
              onClick={() => setStatusFilter(f)}
              className={`px-3 py-1.5 text-sm rounded-md transition-colors ${
                statusFilter === f
                  ? f === 'failed' ? 'bg-red-600 text-white' : f === 'success' ? 'bg-green-600 text-white' : 'bg-gray-600 text-white'
                  : 'text-gray-400 hover:text-gray-200'
              }`}
            >
              {t(f === 'all' ? 'hist.filterAll' : f === 'success' ? 'hist.filterSuccess' : 'hist.filterFailed')}
            </button>
          ))}
        </div>
      </div>

      {/* Desktop (≥ md) — full table */}
      <div className="hidden md:block bg-gray-900 border border-gray-800 rounded-lg p-4 overflow-x-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="text-gray-400 text-left border-b border-gray-800">
              <th className="pb-2">{t('hist.dateOpened')}</th>
              <th className="pb-2">{t('hist.dateClosed')}</th>
              <th className="pb-2">{t('hist.symbol')}</th>
              <th className="pb-2">{t('hist.long')}</th>
              <th className="pb-2">{t('hist.short')}</th>
              <th className="pb-2 text-right">{t('hist.longPrice')}</th>
              <th className="pb-2 text-right">{t('hist.shortPrice')}</th>
              <th className="pb-2 text-right">{t('hist.spread')}</th>
              <th className="pb-2 text-right">{t('hist.exitSpread')}</th>
              <th className="pb-2 text-right">{t('hist.fundingCollected')}</th>
              <th className="pb-2 text-right">{t('hist.rotPnl')}</th>
              <th className="pb-2 text-right">{t('hist.pnl')}</th>
              <th className="pb-2 text-right">{t('hist.duration')}</th>
              <th className="pb-2 text-right">{t('hist.rotations')}</th>
              <th className="pb-2">{t('hist.exitReason')}</th>
              <th className="pb-2 whitespace-nowrap">{t('hist.status')}</th>
              <th className="pb-2 whitespace-nowrap">{t('hist.failureReason')}</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-800">
            {(typeof filteredTrades !== 'undefined' ? filteredTrades : trades).map((tr) => (
              <Fragment key={tr.id}>
              <tr
                className={`text-gray-100 cursor-pointer hover:bg-gray-800/30 ${tr.failure_reason ? 'border-l-2 border-l-red-500 bg-red-950/20' : ''}`}
                aria-expanded={expandedRow === tr.id}
                onClick={() => setExpandedRow(expandedRow === tr.id ? null : tr.id)}
              >
                <td className="py-2 text-gray-400 text-xs">{new Date(tr.created_at).toLocaleString()}</td>
                <td className="py-2 text-gray-400 text-xs">{new Date(tr.updated_at).toLocaleString()}</td>
                <td className="py-2 font-mono">{tr.symbol}</td>
                <td className="py-2 text-green-400 text-xs"><ExchangeLink exchange={tr.long_exchange} symbol={tr.symbol} /></td>
                <td className="py-2 text-red-400 text-xs"><ExchangeLink exchange={tr.short_exchange} symbol={tr.symbol} /></td>
                <td className="py-2 text-right font-mono text-xs text-gray-300">
                  {tr.long_entry > 0 ? tr.long_entry.toPrecision(6) : '-'}
                  {tr.long_exit > 0 ? ` → ${tr.long_exit.toPrecision(6)}` : ''}
                </td>
                <td className="py-2 text-right font-mono text-xs text-gray-300">
                  {tr.short_entry > 0 ? tr.short_entry.toPrecision(6) : '-'}
                  {tr.short_exit > 0 ? ` → ${tr.short_exit.toPrecision(6)}` : ''}
                </td>
                <td className="py-2 text-right font-mono">{tr.entry_spread.toFixed(1)} bps/h</td>
                <td className={`py-2 text-right font-mono ${(tr.current_spread ?? 0) < 0 ? 'text-red-400' : 'text-gray-400'}`}>
                  {tr.current_spread != null ? `${tr.current_spread.toFixed(1)} bps/h` : '-'}
                </td>
                <td className={`py-2 text-right font-mono ${tr.funding_collected > 0 ? 'text-green-400' : tr.funding_collected < 0 ? 'text-red-400' : 'text-gray-400'}`}>${tr.funding_collected.toFixed(2)}</td>
                <td className={`py-2 text-right font-mono ${(tr.rotation_pnl ?? 0) >= 0 ? 'text-gray-400' : 'text-red-400'}`}>
                  {(tr.rotation_pnl ?? 0) !== 0 ? `$${(tr.rotation_pnl ?? 0).toFixed(2)}` : '-'}
                </td>
                <td className={`py-2 text-right font-mono ${tr.realized_pnl >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                  ${tr.realized_pnl.toFixed(2)}
                </td>
                <td className="py-2 text-right font-mono text-gray-400">{formatHoldDuration(tr.created_at, tr.updated_at)}</td>
                <td className="py-2 text-right font-mono text-gray-500">
                  {(tr.rotation_count ?? 0) > 0 ? tr.rotation_count : '-'}
                </td>
                <td className="py-2 text-gray-400 text-xs max-w-[200px]">
                  {expandedRow === tr.id ? (
                    <span className="whitespace-normal break-words">{tr.exit_reason || '-'}</span>
                  ) : (
                    <span className="block truncate">{tr.exit_reason || '-'}</span>
                  )}
                </td>
                <td className="py-2 whitespace-nowrap">
                  {tr.failure_reason ? (
                    <span className="px-2 py-0.5 rounded text-xs font-medium bg-red-500/20 text-red-400">{t('hist.statusFailed')}</span>
                  ) : (
                    <span className="px-2 py-0.5 rounded text-xs font-medium bg-green-500/20 text-green-400">{t('hist.statusSuccess')}</span>
                  )}
                </td>
                <td className="py-2 text-red-400 text-xs max-w-[200px]">
                  <span className="block truncate">{tr.failure_reason || ''}</span>
                </td>
              </tr>
              {expandedRow === tr.id && (
                <tr className="bg-gray-800/50">
                  <td colSpan={15} className="px-4 py-3">
                    <PnLBreakdown position={tr} />
                  </td>
                </tr>
              )}
              </Fragment>
            ))}
            {filteredTrades.length === 0 && (
              <tr>
                <td colSpan={17} className="py-4 text-center text-gray-500">{t('hist.noHistory')}</td>
              </tr>
            )}
          </tbody>
        </table>
      </div>

      {/* Mobile (< md) — card list */}
      <div className="md:hidden space-y-3">
        {filteredTrades.length === 0 && (
          <div className="bg-gray-900 border border-gray-800 rounded-lg p-6 text-center text-gray-500 text-sm">
            {t('hist.noHistory')}
          </div>
        )}
        {filteredTrades.map((tr) => {
          const expanded = expandedRow === tr.id;
          const isFailed = !!tr.failure_reason;
          const pnl = tr.realized_pnl;
          const pnlClass = pnl > 0 ? 'text-green-400' : pnl < 0 ? 'text-red-400' : 'text-gray-400';
          return (
            <div
              key={tr.id}
              className={`bg-gray-900 border rounded-lg overflow-hidden ${
                isFailed ? 'border-red-500/40 bg-red-950/20' : 'border-gray-800'
              }`}
            >
              <button
                type="button"
                onClick={() => setExpandedRow(expanded ? null : tr.id)}
                className="w-full text-left px-4 py-3 hover:bg-gray-800/40 active:bg-gray-800/60 transition-colors"
                aria-expanded={expanded}
              >
                {/* Header row */}
                <div className="flex items-center justify-between gap-2 mb-2">
                  <div className="flex items-center gap-2 min-w-0">
                    <span className="text-gray-500 text-xs">{expanded ? '▼' : '▶'}</span>
                    <span className="font-mono font-semibold text-gray-100 truncate">{tr.symbol}</span>
                    {(tr.rotation_count ?? 0) > 0 && (
                      <span className="shrink-0 px-1.5 py-0.5 text-[10px] font-bold rounded bg-yellow-500/15 text-yellow-400">
                        R{tr.rotation_count}
                      </span>
                    )}
                    {isFailed ? (
                      <span className="shrink-0 px-1.5 py-0.5 text-[10px] font-semibold rounded bg-red-500/20 text-red-400">
                        {t('hist.statusFailed')}
                      </span>
                    ) : (
                      <span className="shrink-0 px-1.5 py-0.5 text-[10px] font-semibold rounded bg-green-500/20 text-green-400">
                        {t('hist.statusSuccess')}
                      </span>
                    )}
                  </div>
                  <span className={`font-mono font-semibold tabular-nums shrink-0 ${pnlClass}`}>
                    ${pnl.toFixed(2)}
                  </span>
                </div>

                {/* Long / Short row */}
                <div className="flex items-center gap-2 text-xs mb-1.5">
                  <span className="text-green-400"><ExchangeLink exchange={tr.long_exchange} symbol={tr.symbol} /></span>
                  <span className="text-gray-600">→</span>
                  <span className="text-red-400"><ExchangeLink exchange={tr.short_exchange} symbol={tr.symbol} /></span>
                  <span className="text-gray-500 ml-auto font-mono">{formatHoldDuration(tr.created_at, tr.updated_at)}</span>
                </div>

                {/* Spread + funding + rot */}
                <div className="grid grid-cols-3 gap-2 text-[11px] border-t border-gray-800/50 pt-2">
                  <div>
                    <div className="text-gray-500 uppercase tracking-wide text-[9px]">{t('hist.spread')}</div>
                    <div className="font-mono tabular-nums text-gray-300">
                      {tr.entry_spread.toFixed(1)}
                      {tr.current_spread != null && (
                        <span className={(tr.current_spread ?? 0) < 0 ? ' text-red-400' : ' text-gray-500'}>
                          {' → '}{tr.current_spread.toFixed(1)}
                        </span>
                      )}
                    </div>
                  </div>
                  <div>
                    <div className="text-gray-500 uppercase tracking-wide text-[9px]">{t('hist.fundingCollected')}</div>
                    <div className={`font-mono tabular-nums ${tr.funding_collected > 0 ? 'text-green-400' : tr.funding_collected < 0 ? 'text-red-400' : 'text-gray-400'}`}>
                      ${tr.funding_collected.toFixed(2)}
                    </div>
                  </div>
                  <div>
                    <div className="text-gray-500 uppercase tracking-wide text-[9px]">{t('hist.rotPnl')}</div>
                    <div className={`font-mono tabular-nums ${(tr.rotation_pnl ?? 0) >= 0 ? 'text-gray-400' : 'text-red-400'}`}>
                      {(tr.rotation_pnl ?? 0) !== 0 ? `$${(tr.rotation_pnl ?? 0).toFixed(2)}` : '-'}
                    </div>
                  </div>
                </div>

                {/* Failure reason on failed trades */}
                {isFailed && tr.failure_reason && !expanded && (
                  <div className="mt-2 text-[11px] text-red-400/80 truncate">{tr.failure_reason}</div>
                )}
              </button>

              {/* Expanded — dates, prices, exit reason, PnL breakdown */}
              {expanded && (
                <div className="px-4 pb-4 border-t border-gray-800 pt-3 space-y-3">
                  <div className="grid grid-cols-1 gap-1.5 text-[11px] text-gray-400">
                    <div>
                      <span className="text-gray-500 uppercase tracking-wide text-[9px]">{t('hist.dateOpened')}</span>
                      <div className="font-mono text-gray-300">{new Date(tr.created_at).toLocaleString()}</div>
                    </div>
                    <div>
                      <span className="text-gray-500 uppercase tracking-wide text-[9px]">{t('hist.dateClosed')}</span>
                      <div className="font-mono text-gray-300">{new Date(tr.updated_at).toLocaleString()}</div>
                    </div>
                  </div>
                  <div className="grid grid-cols-2 gap-2 text-[11px] border-t border-gray-800/50 pt-2">
                    <div>
                      <div className="text-gray-500 uppercase tracking-wide text-[9px]">{t('hist.longPrice')}</div>
                      <div className="font-mono tabular-nums text-gray-300">
                        {tr.long_entry > 0 ? tr.long_entry.toPrecision(6) : '-'}
                        {tr.long_exit > 0 ? ` → ${tr.long_exit.toPrecision(6)}` : ''}
                      </div>
                    </div>
                    <div>
                      <div className="text-gray-500 uppercase tracking-wide text-[9px]">{t('hist.shortPrice')}</div>
                      <div className="font-mono tabular-nums text-gray-300">
                        {tr.short_entry > 0 ? tr.short_entry.toPrecision(6) : '-'}
                        {tr.short_exit > 0 ? ` → ${tr.short_exit.toPrecision(6)}` : ''}
                      </div>
                    </div>
                  </div>
                  {tr.exit_reason && (
                    <div>
                      <div className="text-gray-500 uppercase tracking-wide text-[9px]">{t('hist.exitReason')}</div>
                      <div className="text-gray-300 text-xs">{tr.exit_reason}</div>
                    </div>
                  )}
                  {isFailed && tr.failure_reason && (
                    <div>
                      <div className="text-red-500 uppercase tracking-wide text-[9px]">{t('hist.failureReason')}</div>
                      <div className="text-red-400 text-xs">{tr.failure_reason}</div>
                    </div>
                  )}
                  <div className="border-t border-gray-800/50 pt-3">
                    <PnLBreakdown position={tr} />
                  </div>
                </div>
              )}
            </div>
          );
        })}
      </div>

      {trades.length >= limit && (
        <div className="mt-4 text-center">
          <button
            onClick={loadMore}
            disabled={loading}
            className="px-4 py-2 bg-gray-800 hover:bg-gray-700 text-gray-300 rounded-md text-sm transition-colors disabled:opacity-50"
          >
            {loading ? t('hist.loading') : t('hist.loadMore')}
          </button>
        </div>
      )}
    </div>
  );
};

export default History;
