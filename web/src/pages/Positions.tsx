import { useState, useEffect, useRef, Fragment, type FC } from 'react';
import type { Position, FundingEvent } from '../types.ts';
import { useLocale } from '../i18n/index.ts';
import { tradingUrl } from '../utils/tradingUrl.tsx';


interface PositionsProps {
  positions: Position[];
  onClose?: (positionId: string) => Promise<void>;
  onFetchFunding?: (positionId: string) => Promise<FundingEvent[]>;
}

function formatAge(created: string): string {
  const diff = Date.now() - new Date(created).getTime();
  const hours = Math.floor(diff / 3600000);
  if (hours < 24) return `${hours}h`;
  const days = Math.floor(hours / 24);
  return `${days}d ${hours % 24}h`;
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

function formatPrice(price: number): string {
  if (price >= 100) return price.toFixed(2);
  if (price >= 1) return price.toFixed(4);
  return price.toFixed(6);
}

function formatDateTime(ts: string): string {
  const d = new Date(ts);
  if (isNaN(d.getTime())) return '-';
  const utc = d.toLocaleString('sv-SE', { timeZone: 'UTC' }).replace('T', ' ');
  const tw8 = d.toLocaleString('sv-SE', { timeZone: 'Asia/Taipei' }).replace('T', ' ');
  return `${utc} UTC / ${tw8} +8`;
}

function pnlColor(v: number): string {
  if (v > 0) return 'text-green-400';
  if (v < 0) return 'text-red-400';
  return 'text-gray-400';
}

const Positions: FC<PositionsProps> = ({ positions, onClose, onFetchFunding }) => {
  const { t } = useLocale();
  const [closingId, setClosingId] = useState<string | null>(null);
  const [closing, setClosing] = useState(false);
  const [closeError, setCloseError] = useState<string | null>(null);
  const [expandedId, setExpandedId] = useState<string | null>(null);
  const [fundingHistory, setFundingHistory] = useState<FundingEvent[]>([]);
  const [fundingLoading, setFundingLoading] = useState(false);
  const fetchIdRef = useRef<string | null>(null);

  const toggleExpand = async (id: string) => {
    if (expandedId === id) {
      setExpandedId(null);
      setFundingHistory([]);
      fetchIdRef.current = null;
      return;
    }
    setExpandedId(id);
    setFundingHistory([]);
    fetchIdRef.current = id;
    if (onFetchFunding) {
      setFundingLoading(true);
      try {
        const events = await onFetchFunding(id);
        // Only apply if this is still the expanded position (prevents race).
        if (fetchIdRef.current === id) {
          setFundingHistory(events);
        }
      } catch {
        if (fetchIdRef.current === id) {
          setFundingHistory([]);
        }
      } finally {
        if (fetchIdRef.current === id) {
          setFundingLoading(false);
        }
      }
    }
  };

  // Collapse if the expanded position disappears
  useEffect(() => {
    if (expandedId && !positions.find(p => p.id === expandedId)) {
      setExpandedId(null);
      setFundingHistory([]);
    }
  }, [positions, expandedId]);

  const handleClose = async () => {
    if (!closingId || !onClose) return;
    setClosing(true);
    setCloseError(null);
    try {
      await onClose(closingId);
      setClosingId(null);
    } catch (err) {
      setCloseError(err instanceof Error ? err.message : 'Close failed');
    } finally {
      setClosing(false);
    }
  };

  const dismissDialog = () => {
    setClosingId(null);
    setCloseError(null);
  };

  return (
    <div className="space-y-6">
      <h2 className="text-xl font-bold text-gray-100">{t('pos.title')}</h2>
      <div className="bg-gray-900 border border-gray-800 rounded-lg p-4 overflow-x-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="text-gray-400 text-left border-b border-gray-800">
              <th className="pb-2">{t('pos.symbol')}</th>
              <th className="pb-2">{t('pos.long')}</th>
              <th className="pb-2">{t('pos.short')}</th>
              <th className="pb-2 text-right">{t('pos.entry')}</th>
              <th className="pb-2 text-right">{t('pos.current')}</th>
              <th className="pb-2 text-right">{t('pos.fundingCollected')}</th>
              <th className="pb-2 text-right">{t('pos.entryFees')}</th>
              <th className="pb-2 text-right">{t('pos.rotPnl')}</th>
              <th className="pb-2 text-right">{t('pos.nextFund')}</th>
              <th className="pb-2 text-right">{t('pos.age')}</th>
              <th className="pb-2 text-center">{t('pos.sl')}</th>
              <th className="pb-2"></th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-800">
            {positions.map((p) => (
              <Fragment key={p.id}>
              <tr className={`text-gray-100 cursor-pointer hover:bg-gray-800/40 ${expandedId === p.id ? 'bg-gray-800/30' : ''}`} onClick={() => toggleExpand(p.id)}>
                <td className="py-2 font-mono">
                  <span className="mr-1 text-gray-500 text-xs">{expandedId === p.id ? '▼' : '▶'}</span>
                  {p.symbol}
                  {(p.rotation_count ?? 0) > 0 && (
                    <span className="ml-1.5 text-xs text-yellow-500" title={`Rotated ${p.rotation_count}x, last from ${p.last_rotated_from}${p.all_exchanges ? '. All: ' + p.all_exchanges.join(', ') : ''}`}>
                      R{p.rotation_count}
                    </span>
                  )}
                </td>
                <td className="py-2 text-sm">
                  <a href={tradingUrl(p.long_exchange, p.symbol)} target="_blank" rel="noopener noreferrer"
                    className="text-green-400 hover:underline cursor-pointer" onClick={e => e.stopPropagation()}>{p.long_exchange}</a>{' '}
                  <span className="font-mono text-xs">{p.long_size.toFixed(4)}@{formatPrice(p.long_entry)}</span>
                </td>
                <td className="py-2 text-sm">
                  <a href={tradingUrl(p.short_exchange, p.symbol)} target="_blank" rel="noopener noreferrer"
                    className="text-red-400 hover:underline cursor-pointer" onClick={e => e.stopPropagation()}>{p.short_exchange}</a>{' '}
                  <span className="font-mono text-xs">{p.short_size.toFixed(4)}@{formatPrice(p.short_entry)}</span>
                </td>
                <td className="py-2 text-right font-mono">{p.entry_spread.toFixed(1)} bps/h</td>
                <td className={`py-2 text-right font-mono ${(p.current_spread ?? 0) > 0 ? 'text-green-400' : (p.current_spread ?? 0) < 0 ? 'text-red-400' : 'text-gray-400'}`}>
                  {p.current_spread != null ? `${p.current_spread.toFixed(1)} bps/h` : '-'}
                </td>
                <td className={`py-2 text-right font-mono ${p.funding_collected > 0 ? 'text-green-400' : p.funding_collected < 0 ? 'text-red-400' : 'text-gray-400'}`}>${p.funding_collected.toFixed(2)}</td>
                <td className="py-2 text-right font-mono text-red-400">
                  {(p.entry_fees ?? 0) > 0 ? `-$${(p.entry_fees ?? 0).toFixed(2)}` : '-'}
                </td>
                <td className={`py-2 text-right font-mono ${(p.rotation_pnl ?? 0) >= 0 ? 'text-gray-400' : 'text-red-400'}`}>
                  {(p.rotation_pnl ?? 0) !== 0 ? `$${(p.rotation_pnl ?? 0).toFixed(2)}` : '-'}
                </td>
                <td className="py-2 text-right font-mono text-gray-400 text-xs">
                  {formatFundingCountdown(p.next_funding)}
                </td>
                <td className="py-2 text-right font-mono text-gray-400">{formatAge(p.created_at)}</td>
                <td className="py-2 text-center">
                  {(p.long_sl_order_id || p.short_sl_order_id) ? (
                    <span className="text-xs text-green-500" title={`L:${p.long_sl_order_id || 'none'} S:${p.short_sl_order_id || 'none'}`}>ON</span>
                  ) : (
                    <span className="text-xs text-gray-600">-</span>
                  )}
                </td>
                <td className="px-2 py-1">
                  {p.status === 'active' && onClose && (
                    <button
                      onClick={(e) => { e.stopPropagation(); setClosingId(p.id); }}
                      disabled={closing}
                      className="px-2 py-0.5 text-xs bg-red-600/20 text-red-400 rounded hover:bg-red-600/40 disabled:opacity-50"
                    >
                      {t('pos.close')}
                    </button>
                  )}
                </td>
              </tr>
              {expandedId === p.id && (
                <tr className="bg-gray-800/50">
                  <td colSpan={12} className="px-4 py-3">
                    <div className="space-y-4 text-sm">
                      {/* Section 1: Position Info */}
                      <div>
                        <div className="text-gray-400 text-xs mb-1">{t('pos.openTime')}: <span className="text-gray-200">{formatDateTime(p.created_at)}</span></div>
                        <div className="text-gray-400 text-xs">
                          {t('pos.unrealizedPnl')}:{' '}
                          <span className={pnlColor(p.long_unrealized_pnl ?? 0)}>
                            {t('pos.long')}: ${(p.long_unrealized_pnl ?? 0).toFixed(2)} ({p.long_exchange})
                          </span>
                          <span className="mx-2 text-gray-600">|</span>
                          <span className={pnlColor(p.short_unrealized_pnl ?? 0)}>
                            {t('pos.short')}: ${(p.short_unrealized_pnl ?? 0).toFixed(2)} ({p.short_exchange})
                          </span>
                        </div>
                      </div>

                      {/* Section 2: Funding History */}
                      <div>
                        <div className="flex items-center justify-between mb-2">
                          <span className="text-gray-300 text-xs font-semibold tracking-wide uppercase">{t('pos.fundingHistory')}</span>
                          <span className={`text-xs font-mono ${pnlColor(p.funding_collected)}`}>
                            {t('pos.fundingCollected')}: ${p.funding_collected.toFixed(4)}
                          </span>
                        </div>
                        {fundingLoading ? (
                          <div className="text-gray-500 text-xs py-3 text-center">{t('pos.loading')}</div>
                        ) : fundingHistory.length === 0 ? (
                          <div className="text-gray-500 text-xs">-</div>
                        ) : (() => {
                          // Group by date
                          const groups: Record<string, FundingEvent[]> = {};
                          for (const f of fundingHistory) {
                            const d = new Date(f.time);
                            const key = isNaN(d.getTime()) ? 'Unknown' : d.toLocaleDateString('sv-SE', { timeZone: 'Asia/Taipei' });
                            (groups[key] ??= []).push(f);
                          }
                          const sortedDates = Object.keys(groups).sort((a, b) => a.localeCompare(b));

                          return (
                            <div className="space-y-1 max-h-64 overflow-y-auto pr-1">
                              {sortedDates.map((date) => {
                                const items = groups[date];
                                const dayTotal = items.reduce((s, f) => s + f.amount, 0);

                                return (
                                  <details key={date} open={date === sortedDates[sortedDates.length - 1]} className="group">
                                    <summary className="flex items-center justify-between cursor-pointer select-none py-1.5 px-2 rounded bg-gray-700/30 hover:bg-gray-700/50 transition-colors">
                                      <div className="flex items-center gap-2">
                                        <span className="text-[10px] text-gray-500 group-open:rotate-90 transition-transform">▶</span>
                                        <span className="text-xs font-mono text-gray-300">{date}</span>
                                        <span className="text-[10px] text-gray-500">{items.length} payments</span>
                                      </div>
                                      <span className={`text-xs font-mono ${pnlColor(dayTotal)}`}>
                                        {dayTotal >= 0 ? '+' : ''}{dayTotal.toFixed(4)}
                                      </span>
                                    </summary>
                                    <div className="mt-0.5 ml-4 border-l border-gray-700/50 pl-2">
                                      {items.map((f, i) => {
                                        const time = new Date(f.time);
                                        const utcStr = isNaN(time.getTime()) ? '-' : time.toLocaleTimeString('en-GB', { hour: '2-digit', minute: '2-digit', timeZone: 'UTC' });
                                        const tw8Str = isNaN(time.getTime()) ? '' : time.toLocaleTimeString('en-GB', { hour: '2-digit', minute: '2-digit', timeZone: 'Asia/Taipei' });
                                        const timeStr = isNaN(time.getTime()) ? '-' : `${utcStr} / ${tw8Str} +8`;
                                        return (
                                          <div key={i} className="flex items-center justify-between py-0.5 text-xs hover:bg-gray-700/20 rounded px-1">
                                            <div className="flex items-center gap-2">
                                              <span className="font-mono text-gray-500 w-28">{timeStr}</span>
                                              <span className={`px-1.5 py-0 rounded text-[10px] font-medium ${
                                                f.side === 'long' ? 'bg-green-500/15 text-green-400' : 'bg-red-500/15 text-red-400'
                                              }`}>{f.side}</span>
                                              <span className="text-gray-400">{f.exchange}</span>
                                            </div>
                                            <span className={`font-mono ${pnlColor(f.amount)}`}>
                                              {f.amount >= 0 ? '+' : ''}{f.amount.toFixed(4)}
                                            </span>
                                          </div>
                                        );
                                      })}
                                    </div>
                                  </details>
                                );
                              })}
                            </div>
                          );
                        })()}
                      </div>

                      {/* Section 3: Rotation History */}
                      {p.rotation_history && p.rotation_history.length > 0 && (
                        <div>
                          <div className="text-gray-300 text-xs font-semibold mb-1">{t('pos.rotationHistory')}</div>
                          <table className="w-full text-xs">
                            <thead>
                              <tr className="text-gray-500 text-left">
                                <th className="pr-4 pb-1">Time</th>
                                <th className="pr-4 pb-1">{t('pos.from')} → {t('pos.to')}</th>
                                <th className="pr-4 pb-1">Leg</th>
                                <th className="pr-4 pb-1 text-right">PnL</th>
                              </tr>
                            </thead>
                            <tbody>
                              {p.rotation_history.map((r, i) => (
                                <tr key={i} className="text-gray-300">
                                  <td className="pr-4 py-0.5 font-mono">{formatDateTime(r.timestamp)}</td>
                                  <td className="pr-4 py-0.5">{r.from} → {r.to}</td>
                                  <td className="pr-4 py-0.5">{r.leg_side}</td>
                                  <td className={`pr-4 py-0.5 text-right font-mono ${pnlColor(r.pnl ?? 0)}`}>
                                    {r.pnl != null ? `$${r.pnl.toFixed(2)}` : '-'}
                                  </td>
                                </tr>
                              ))}
                            </tbody>
                          </table>
                        </div>
                      )}
                    </div>
                  </td>
                </tr>
              )}
              </Fragment>
            ))}
            {positions.length === 0 && (
              <tr>
                <td colSpan={12} className="py-4 text-center text-gray-500">{t('pos.noPositions')}</td>
              </tr>
            )}
          </tbody>
        </table>
      </div>

      {closingId && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="bg-gray-900 border border-gray-700 rounded-lg p-6 max-w-sm">
            <h3 className="text-lg font-semibold text-gray-100 mb-2">{t('pos.closeConfirmTitle')}</h3>
            <p className="text-gray-300 text-sm mb-4">
              {t('pos.closeConfirmMsg')}: <span className="font-mono font-bold">{positions.find(p => p.id === closingId)?.symbol}</span>
            </p>
            {closeError && (
              <p className="text-red-400 text-sm mb-3">{closeError}</p>
            )}
            <div className="flex gap-3">
              <button
                onClick={handleClose}
                disabled={closing}
                className="px-4 py-2 text-sm bg-red-600 text-white rounded hover:bg-red-700 disabled:opacity-50"
              >
                {closing ? '...' : t('pos.closeConfirm')}
              </button>
              <button
                onClick={dismissDialog}
                disabled={closing}
                className="px-4 py-2 text-sm bg-gray-700 text-gray-300 rounded hover:bg-gray-600 disabled:opacity-50"
              >
                {t('pos.closeCancel')}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};

export default Positions;
