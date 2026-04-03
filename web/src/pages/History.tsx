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

  return (
    <div className="space-y-6">
      <h2 className="text-xl font-bold text-gray-100">{t('hist.title')}</h2>
      <div className="bg-gray-900 border border-gray-800 rounded-lg p-4 overflow-x-auto">
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
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-800">
            {trades.map((tr) => (
              <Fragment key={tr.id}>
              <tr
                className="text-gray-100 cursor-pointer hover:bg-gray-800/30"
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
            {trades.length === 0 && (
              <tr>
                <td colSpan={15} className="py-4 text-center text-gray-500">{t('hist.noHistory')}</td>
              </tr>
            )}
          </tbody>
        </table>
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
    </div>
  );
};

export default History;
