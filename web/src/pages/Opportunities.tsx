import { useState, useEffect, useCallback, type FC } from 'react';
import type { Opportunity } from '../types.ts';
import { useLocale } from '../i18n/index.ts';
import { ExchangeLink } from '../utils/tradingUrl.tsx';

interface OpportunitiesProps {
  opportunities: Opportunity[];
  onOpen?: (symbol: string, longExchange: string, shortExchange: string, force?: boolean) => Promise<void>;
}

function scoreColor(score: number): string {
  if (score >= 70) return 'bg-green-500/5';
  if (score >= 40) return 'bg-yellow-500/5';
  return 'bg-red-500/5';
}

function formatInterval(hours?: number): string {
  if (!hours) return '-';
  return `${hours}h`;
}

function formatFundingCountdown(next?: string): string {
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

function formatTimestamp(ts?: string): string {
  if (!ts) return '-';
  const d = new Date(ts);
  if (isNaN(d.getTime())) return '-';
  return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });
}

const Opportunities: FC<OpportunitiesProps> = ({ opportunities, onOpen }) => {
  const { t } = useLocale();
  const [openingOpp, setOpeningOpp] = useState<Opportunity | null>(null);
  const [opening, setOpening] = useState(false);
  const [openError, setOpenError] = useState<string | null>(null);

  // Dismiss modal on Escape key
  const dismissModal = useCallback(() => { if (!opening) setOpeningOpp(null); }, [opening]);
  useEffect(() => {
    const handler = (e: KeyboardEvent) => { if (e.key === 'Escape') dismissModal(); };
    document.addEventListener('keydown', handler);
    return () => document.removeEventListener('keydown', handler);
  }, [dismissModal]);

  const sorted = [...opportunities].sort((a, b) => b.score - a.score);

  const handleConfirmOpen = async (force = false) => {
    if (!openingOpp || !onOpen) return;
    setOpening(true);
    setOpenError(null);
    try {
      await onOpen(openingOpp.symbol, openingOpp.long_exchange, openingOpp.short_exchange, force);
      setOpeningOpp(null);
    } catch (err: unknown) {
      setOpenError(err instanceof Error ? err.message : 'Open failed');
    } finally {
      setOpening(false);
    }
  };

  return (
    <div className="space-y-6">
      <h2 className="text-xl font-bold text-gray-100">
        {t('opp.title')}
        <span className="ml-2 text-sm font-normal text-gray-500">{sorted.length} {t('opp.found')}</span>
      </h2>
      <div className="bg-gray-900 border border-gray-800 rounded-lg p-4 overflow-x-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="text-gray-400 text-left border-b border-gray-800">
              <th className="pb-2">#</th>
              <th className="pb-2">{t('opp.symbol')}</th>
              <th className="pb-2">{t('opp.long')}</th>
              <th className="pb-2">{t('opp.short')}</th>
              <th className="pb-2 text-right">{t('opp.longRate')}</th>
              <th className="pb-2 text-right">{t('opp.shortRate')}</th>
              <th className="pb-2 text-right">{t('opp.spread')}</th>
              <th className="pb-2 text-right">{t('opp.interval')}</th>
              <th className="pb-2 text-right">{t('opp.nextFund')}</th>
              <th className="pb-2 text-right">{t('opp.oi')}</th>
              <th className="pb-2 text-right">{t('opp.updated')}</th>
              <th className="pb-2"></th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-800">
            {sorted.map((opp, i) => (
              <tr key={`${opp.symbol}-${opp.long_exchange}-${opp.short_exchange}`} className={`text-gray-100 ${scoreColor(opp.score)}`}>
                <td className="py-2 font-mono text-gray-500">{i + 1}</td>
                <td className="py-2 font-mono">{opp.symbol}</td>
                <td className="py-2 text-green-400"><ExchangeLink exchange={opp.long_exchange} symbol={opp.symbol} /></td>
                <td className="py-2 text-red-400"><ExchangeLink exchange={opp.short_exchange} symbol={opp.symbol} /></td>
                <td className="py-2 text-right font-mono text-gray-400">{opp.long_rate.toFixed(2)}</td>
                <td className="py-2 text-right font-mono text-gray-400">{opp.short_rate.toFixed(2)}</td>
                <td className="py-2 text-right font-mono font-semibold">{opp.spread.toFixed(1)} bps/h</td>
                <td className="py-2 text-right font-mono text-gray-400">{formatInterval(opp.interval_hours)}</td>
                <td className="py-2 text-right font-mono text-gray-400 text-xs">{formatFundingCountdown(opp.next_funding)}</td>
                <td className="py-2 text-right font-mono">{opp.oi_rank}</td>
                <td className="py-2 text-right font-mono text-gray-500 text-xs">{formatTimestamp(opp.timestamp)}</td>
                <td className="px-2 py-1">
                  {onOpen && (
                    <button onClick={() => { setOpeningOpp(opp); setOpenError(null); }} disabled={opening}
                      className="px-2 py-0.5 text-xs bg-green-600/20 text-green-400 rounded hover:bg-green-600/40 disabled:opacity-50">
                      {t('opp.open')}
                    </button>
                  )}
                </td>
              </tr>
            ))}
            {sorted.length === 0 && (
              <tr>
                <td colSpan={12} className="py-4 text-center text-gray-500">{t('opp.noOpportunities')}</td>
              </tr>
            )}
          </tbody>
        </table>
      </div>

      {/* Open confirmation dialog */}
      {openingOpp && (
        <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-50">
          <div className="bg-gray-900 border border-gray-700 rounded-lg p-6 max-w-md w-full mx-4">
            <h3 className="text-lg font-bold text-gray-100 mb-4">{t('opp.openConfirmTitle')}</h3>
            <p className="text-gray-300 mb-2">
              {t('opp.openConfirmMsg')} <span className="font-mono font-bold text-white">{openingOpp.symbol}</span>
            </p>
            <p className="text-sm text-gray-400 mb-1">
              <span className="text-green-400">{openingOpp.long_exchange}</span>
              {' → '}
              <span className="text-red-400">{openingOpp.short_exchange}</span>
            </p>
            <p className="text-sm text-gray-400 mb-4">
              {t('opp.spread')}: <span className="font-mono">{openingOpp.spread.toFixed(1)} bps/h</span>
            </p>
            {openError && (
              <div className="mb-4">
                <p className="text-red-400 text-sm">{openError}</p>
                {openError.includes('risk rejected') && (
                  <button onClick={() => handleConfirmOpen(true)} disabled={opening}
                    className="mt-2 px-3 py-1.5 text-sm bg-yellow-600/20 text-yellow-400 rounded hover:bg-yellow-600/40 disabled:opacity-50">
                    {opening ? '...' : t('opp.forceOpen')}
                  </button>
                )}
              </div>
            )}
            <div className="flex gap-3 justify-end">
              <button onClick={() => setOpeningOpp(null)} disabled={opening}
                className="px-4 py-2 text-sm text-gray-400 hover:text-gray-200 disabled:opacity-50">
                {t('opp.openCancel')}
              </button>
              <button onClick={() => handleConfirmOpen(false)} disabled={opening}
                className="px-4 py-2 text-sm bg-green-600 text-white rounded hover:bg-green-700 disabled:opacity-50">
                {opening ? '...' : t('opp.openConfirm')}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};

export default Opportunities;
