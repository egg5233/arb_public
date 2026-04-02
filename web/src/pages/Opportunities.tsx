import { useState, useEffect, useCallback, type FC } from 'react';
import type { Opportunity, SpotOpportunity } from '../types.ts';
import { useLocale } from '../i18n/index.ts';
import { ExchangeLink } from '../utils/tradingUrl.tsx';

type Tab = 'perp' | 'spot';

interface OpportunitiesProps {
  opportunities: Opportunity[];
  spotOpportunities?: SpotOpportunity[];
  onOpen?: (symbol: string, longExchange: string, shortExchange: string, force?: boolean) => Promise<void>;
  onSpotOpen?: (symbol: string, exchange: string, direction: string) => Promise<void>;
  blacklist?: string[];
  onBlacklistToggle?: (symbol: string) => Promise<void>;
}

function scoreColor(score: number): string {
  if (score >= 70) return 'bg-emerald-500/5';
  if (score >= 40) return 'bg-amber-500/5';
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

const Opportunities: FC<OpportunitiesProps> = ({ opportunities, spotOpportunities = [], onOpen, onSpotOpen, blacklist = [], onBlacklistToggle }) => {
  const { t } = useLocale();
  const [tab, setTab] = useState<Tab>('perp');
  const [openingOpp, setOpeningOpp] = useState<Opportunity | null>(null);
  const [opening, setOpening] = useState(false);
  const [openError, setOpenError] = useState<string | null>(null);
  const [spotOpening, setSpotOpening] = useState<string | null>(null);
  const [spotError, setSpotError] = useState<string | null>(null);

  const dismissModal = useCallback(() => { if (!opening) setOpeningOpp(null); }, [opening]);
  useEffect(() => {
    const handler = (e: KeyboardEvent) => { if (e.key === 'Escape') dismissModal(); };
    document.addEventListener('keydown', handler);
    return () => document.removeEventListener('keydown', handler);
  }, [dismissModal]);

  const sorted = [...opportunities].sort((a, b) => b.score - a.score);
  const spotPassed = spotOpportunities.filter((o) => !o.filter_status);

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
    <div className="space-y-4">
      {/* Header with segmented toggle */}
      <div className="flex items-center justify-between">
        <h2 className="text-xl font-bold text-gray-100">{t('opp.title')}</h2>

        {/* Strategy toggle */}
        <div className="flex bg-gray-900 border border-gray-700 rounded-lg p-0.5 gap-0.5">
          <button
            onClick={() => setTab('perp')}
            className={`px-4 py-1.5 text-xs font-semibold rounded-md transition-all duration-150 ${
              tab === 'perp'
                ? 'bg-gray-800 text-gray-100 shadow-sm shadow-black/30'
                : 'text-gray-500 hover:text-gray-300'
            }`}
          >
            {t('spot.tabPerp')}
            <span className={`ml-1.5 font-mono ${tab === 'perp' ? 'text-emerald-400' : 'text-gray-600'}`}>
              {sorted.length}
            </span>
          </button>
          <button
            onClick={() => setTab('spot')}
            className={`px-4 py-1.5 text-xs font-semibold rounded-md transition-all duration-150 ${
              tab === 'spot'
                ? 'bg-gray-800 text-gray-100 shadow-sm shadow-black/30'
                : 'text-gray-500 hover:text-gray-300'
            }`}
          >
            {t('spot.tabSpot')}
            <span className={`ml-1.5 font-mono ${tab === 'spot' ? 'text-emerald-400' : 'text-gray-600'}`}>
              {spotPassed.length}/{spotOpportunities.length}
            </span>
          </button>
        </div>
      </div>

      {/* Perp-Perp Tab */}
      {tab === 'perp' && (
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
                  <td className="py-2 font-mono">
                    {opp.symbol}
                    {blacklist.includes(opp.symbol) && (
                      <span className="ml-1.5 text-xs text-yellow-500" title={t('opp.blocked')}>BAN</span>
                    )}
                  </td>
                  <td className="py-2 text-green-400"><ExchangeLink exchange={opp.long_exchange} symbol={opp.symbol} /></td>
                  <td className="py-2 text-red-400"><ExchangeLink exchange={opp.short_exchange} symbol={opp.symbol} /></td>
                  <td className="py-2 text-right font-mono text-gray-400">{opp.long_rate.toFixed(2)}</td>
                  <td className="py-2 text-right font-mono text-gray-400">{opp.short_rate.toFixed(2)}</td>
                  <td className="py-2 text-right font-mono font-semibold">{opp.spread.toFixed(1)} bps/h</td>
                  <td className="py-2 text-right font-mono text-gray-400">{formatInterval(opp.interval_hours)}</td>
                  <td className="py-2 text-right font-mono text-gray-400 text-xs">{formatFundingCountdown(opp.next_funding)}</td>
                  <td className="py-2 text-right font-mono">{opp.oi_rank}</td>
                  <td className="py-2 text-right font-mono text-gray-500 text-xs">{formatTimestamp(opp.timestamp)}</td>
                  <td className="px-2 py-1 whitespace-nowrap">
                    {onBlacklistToggle && (
                      <button
                        onClick={() => onBlacklistToggle(opp.symbol)}
                        className={`px-2 py-0.5 text-xs rounded mr-1 ${
                          blacklist.includes(opp.symbol)
                            ? 'bg-yellow-600/20 text-yellow-400 hover:bg-yellow-600/40'
                            : 'bg-gray-600/20 text-gray-400 hover:bg-gray-600/40'
                        }`}
                        title={blacklist.includes(opp.symbol) ? t('opp.unblock') : t('opp.block')}
                      >
                        {blacklist.includes(opp.symbol) ? t('opp.unblock') : t('opp.block')}
                      </button>
                    )}
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
                  <td colSpan={12} className="py-8 text-center text-gray-500">{t('opp.noOpportunities')}</td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      )}

      {/* Spot-Futures Tab */}
      {tab === 'spot' && (
        <div className="bg-gray-900 border border-gray-800 rounded-lg p-4 overflow-x-auto">
          {/* Summary bar */}
          <div className="flex items-center gap-4 mb-3 text-xs text-gray-500">
            <span>
              <span className="inline-block w-2 h-2 rounded-full bg-emerald-400 mr-1.5" />
              {spotPassed.length} {t('spot.actionable')}
            </span>
            <span>
              <span className="inline-block w-2 h-2 rounded-full bg-gray-600 mr-1.5" />
              {spotOpportunities.length - spotPassed.length} {t('spot.filtered')}
            </span>
          </div>

          {spotError && (
            <div className="flex items-center justify-between bg-red-500/10 border border-red-500/30 rounded-lg px-3 py-2 text-sm text-red-400">
              <span>{spotError}</span>
              <button onClick={() => setSpotError(null)} className="ml-2 text-red-500 hover:text-red-300">&times;</button>
            </div>
          )}

          <table className="w-full text-sm">
            <thead>
              <tr className="text-gray-400 text-left border-b border-gray-800">
                <th className="pb-2">#</th>
                <th className="pb-2">{t('spot.symbol')}</th>
                <th className="pb-2">{t('spot.exchange')}</th>
                <th className="pb-2">{t('spot.direction')}</th>
                <th className="pb-2 text-right">{t('spot.funding')}</th>
                <th className="pb-2 text-right">{t('spot.borrow')}</th>
                <th className="pb-2 text-right">{t('spot.fees')}</th>
                <th className="pb-2 text-right">{t('spot.netApr')}</th>
                <th className="pb-2">{t('spot.status')}</th>
                <th className="pb-2">{t('spot.reason')}</th>
                <th className="pb-2"></th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-800/50">
              {spotOpportunities.map((opp, i) => {
                const filtered = !!opp.filter_status;
                const isA = opp.direction === 'borrow_sell_long';
                const oppKey = `${opp.symbol}-${opp.exchange}-${opp.direction}`;
                const dirLabel = isA ? t('spot.dirA') : t('spot.dirB');
                const dirDesc = isA ? t('spot.dirADesc') : t('spot.dirBDesc');
                return (
                  <tr
                    key={`${opp.symbol}-${opp.exchange}-${opp.direction}`}
                    className={filtered ? 'text-gray-600' : 'text-gray-100'}
                  >
                    <td className="py-2 font-mono text-gray-500">{i + 1}</td>
                    <td className="py-2 font-mono">{opp.symbol}</td>
                    <td className="py-2 capitalize">
                      {filtered
                        ? opp.exchange
                        : <ExchangeLink exchange={opp.exchange} symbol={opp.symbol} className="text-gray-100" />
                      }
                    </td>
                    <td className={`py-2 font-mono ${
                      filtered ? '' : isA ? 'text-blue-400' : 'text-purple-400'
                    }`} title={dirDesc}>
                      {dirLabel} <span className="text-xs text-gray-500 font-normal hidden sm:inline">({dirDesc})</span>
                    </td>
                    <td className={`py-2 text-right font-mono ${filtered ? '' : 'text-emerald-400'}`}>
                      {(opp.funding_apr * 100).toFixed(1)}%
                    </td>
                    <td className={`py-2 text-right font-mono ${filtered ? '' : 'text-amber-400'}`}>
                      {opp.borrow_apr > 0 ? `${(opp.borrow_apr * 100).toFixed(1)}%` : '-'}
                    </td>
                    <td className={`py-2 text-right font-mono ${filtered ? '' : 'text-gray-400'}`}>
                      {(opp.fee_apr * 100).toFixed(1)}%
                    </td>
                    <td className={`py-2 text-right font-mono font-semibold ${
                      filtered ? '' : opp.net_apr >= 0 ? 'text-emerald-400' : 'text-red-400'
                    }`}>
                      {(opp.net_apr * 100).toFixed(1)}%
                    </td>
                    <td className="py-2">
                      {filtered ? (
                        <span className="inline-flex items-center gap-1 text-xs text-gray-600">
                          <span className="w-1.5 h-1.5 rounded-full bg-gray-600" />
                          {t('spot.filtered')}
                        </span>
                      ) : (
                        <span className="inline-flex items-center gap-1 text-xs text-emerald-400">
                          <span className="w-1.5 h-1.5 rounded-full bg-emerald-400" />
                          {t('spot.ready')}
                        </span>
                      )}
                    </td>
                    <td className="py-2">
                      {filtered && (
                        <span className="text-xs text-gray-600 truncate max-w-[200px] inline-block" title={opp.filter_status}>
                          {opp.filter_status}
                        </span>
                      )}
                    </td>
                    <td className="px-2 py-1">
                      {onSpotOpen && !filtered && (
                        <button
                          disabled={spotOpening === oppKey}
                          onClick={async () => {
                            setSpotOpening(oppKey);
                            setSpotError(null);
                            try {
                              await onSpotOpen(opp.symbol, opp.exchange, opp.direction);
                              setSpotOpening(null);
                            } catch (err: unknown) {
                              setSpotError(err instanceof Error ? err.message : 'Open failed');
                              setSpotOpening(null);
                            }
                          }}
                          className={`px-2 py-0.5 text-xs rounded transition-colors ${
                            spotOpening === oppKey
                              ? 'bg-gray-700 text-gray-400 cursor-wait'
                              : 'bg-emerald-600/20 text-emerald-400 hover:bg-emerald-600/40'
                          }`}
                        >
                          {spotOpening === oppKey ? 'Opening...' : 'Open'}
                        </button>
                      )}
                    </td>
                  </tr>
                );
              })}
              {spotOpportunities.length === 0 && (
                <tr>
                  <td colSpan={11} className="py-8 text-center text-gray-500">
                    {t('spot.noOpportunities')}
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      )}

      {/* Open confirmation dialog (perp-perp only) */}
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
