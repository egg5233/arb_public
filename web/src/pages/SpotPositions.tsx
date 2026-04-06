import { useState, useEffect, Fragment, type FC } from 'react';
import type { SpotPosition, SpotStats } from '../types.ts';
import { useLocale } from '../i18n/index.ts';
import { tradingUrl } from '../utils/tradingUrl.tsx';

interface SpotPositionsProps {
  positions: SpotPosition[];
  onClose?: (positionId: string) => Promise<void>;
  getStats?: () => Promise<SpotStats>;
  getHistory?: (limit?: number) => Promise<SpotPosition[]>;
}

function formatAge(created: string): string {
  const diff = Date.now() - new Date(created).getTime();
  const hours = Math.floor(diff / 3600000);
  if (hours < 24) return `${hours}h`;
  const days = Math.floor(hours / 24);
  return `${days}d ${hours % 24}h`;
}

function formatPrice(price: unknown): string {
  const v = n(price);
  if (v >= 100) return v.toFixed(2);
  if (v >= 1) return v.toFixed(4);
  return v.toFixed(6);
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

function statusBadge(status: string): string {
  switch (status) {
    case 'active': return 'bg-green-500/20 text-green-400';
    case 'exiting': return 'bg-amber-500/20 text-amber-400';
    case 'pending_recovery': return 'bg-red-500/20 text-red-400';
    case 'closed': return 'bg-gray-500/20 text-gray-400';
    default: return 'bg-gray-500/20 text-gray-400';
  }
}

function isDirA(direction: string): boolean {
  return direction === 'borrow_sell_long';
}

// Safe number accessor — coerces strings, returns 0 for undefined/null/NaN
function n(v: unknown): number {
  if (v == null) return 0;
  const num = typeof v === 'string' ? parseFloat(v) : Number(v);
  return isNaN(num) ? 0 : num;
}

const SpotPositions: FC<SpotPositionsProps> = ({ positions, onClose, getStats, getHistory }) => {
  const { t } = useLocale();
  const [tab, setTab] = useState<'active' | 'history'>('active');
  const [historyData, setHistoryData] = useState<SpotPosition[]>([]);
  const [stats, setStats] = useState<SpotStats | null>(null);
  const [statsError, setStatsError] = useState<string | null>(null);
  const [closingId, setClosingId] = useState<string | null>(null);
  const [closing, setClosing] = useState(false);
  const [closeError, setCloseError] = useState<string | null>(null);
  const [expandedId, setExpandedId] = useState<string | null>(null);

  // Fetch stats on mount and tab change
  useEffect(() => {
    if (!getStats) return;
    setStatsError(null);
    getStats().then(setStats).catch((err) => setStatsError(err instanceof Error ? err.message : 'Failed'));
  }, [getStats, tab]);

  // Fetch history when switching to history tab
  useEffect(() => {
    if (tab !== 'history' || !getHistory) return;
    getHistory(100).then(setHistoryData).catch(() => setHistoryData([]));
  }, [tab, getHistory]);

  const displayData = tab === 'active' ? positions : historyData;

  // Collapse if expanded position disappears
  useEffect(() => {
    if (expandedId && !displayData.find(p => p.id === expandedId)) {
      setExpandedId(null);
    }
  }, [displayData, expandedId]);

  const toggleExpand = (id: string) => {
    setExpandedId(expandedId === id ? null : id);
  };

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

  const totalPnl = stats ? n(stats.total_pnl) : 0;
  const totalNotional = displayData.reduce((sum, p) => sum + n(p.notional_usdt), 0);

  return (
    <div className="space-y-6">
      <h2 className="text-xl font-bold text-gray-100">{t('spotPos.title')}</h2>

      {/* Stats summary bar */}
      <div className="flex gap-3 flex-wrap">
        <div className="bg-gray-900 border border-gray-800 rounded-lg p-3 flex-1 min-w-[140px]">
          <div className="text-gray-400 text-xs">{t('spotPos.active')}</div>
          <div className="text-gray-100 font-mono text-lg">{positions.length}</div>
        </div>
        <div className="bg-gray-900 border border-gray-800 rounded-lg p-3 flex-1 min-w-[140px]">
          <div className="text-gray-400 text-xs">{t('overview.totalPnl')}</div>
          <div className={`font-mono text-lg ${pnlColor(totalPnl)}`}>
            ${totalPnl.toFixed(2)}
          </div>
        </div>
        <div className="bg-gray-900 border border-gray-800 rounded-lg p-3 flex-1 min-w-[140px]">
          <div className="text-gray-400 text-xs">{t('spotPos.winLoss')}</div>
          <div className="text-gray-100 font-mono text-lg">
            {stats ? `${stats.win_count}/${stats.loss_count}` : '-'}
          </div>
        </div>
        <div className="bg-gray-900 border border-gray-800 rounded-lg p-3 flex-1 min-w-[140px]">
          <div className="text-gray-400 text-xs">{t('spotPos.totalNotional')}</div>
          <div className="text-gray-100 font-mono text-lg">${totalNotional.toFixed(2)}</div>
        </div>
      </div>
      {statsError && <div className="text-red-400 text-xs">{t('spotPos.statsError')}: {statsError}</div>}

      {/* Active/History tabs */}
      <div className="flex gap-2">
        <button
          onClick={() => setTab('active')}
          className={`px-4 py-1.5 text-sm rounded ${tab === 'active' ? 'bg-gray-700 text-white' : 'text-gray-400 hover:text-gray-200'}`}
        >
          {t('spotPos.active')}
        </button>
        <button
          onClick={() => setTab('history')}
          className={`px-4 py-1.5 text-sm rounded ${tab === 'history' ? 'bg-gray-700 text-white' : 'text-gray-400 hover:text-gray-200'}`}
        >
          {t('spotPos.history')}
        </button>
      </div>

      {/* Main table */}
      <div className="bg-gray-900 border border-gray-800 rounded-lg p-4 overflow-x-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="text-gray-400 text-left border-b border-gray-800">
              <th className="pb-2">{t('pos.symbol')}</th>
              <th className="pb-2">{t('spotPos.exchange')}</th>
              <th className="pb-2">{t('spotPos.dir')}</th>
              <th className="pb-2">{t('spotPos.status')}</th>
              <th className="pb-2 text-right">{t('spotPos.notional')}</th>
              <th className="pb-2 text-right">{t('spotPos.netYield')}</th>
              <th className="pb-2 text-right">{t('spotPos.borrowCost')}</th>
              <th className="pb-2 text-right">{t('spotPos.entryFees')}</th>
              <th className="pb-2 text-right">{t('spotPos.margin')}</th>
              <th className="pb-2 text-right">{t('spotPos.age')}</th>
              <th className="pb-2"></th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-800">
            {displayData.map((p) => (
              <Fragment key={p.id}>
              <tr className={`text-gray-100 cursor-pointer hover:bg-gray-800/40 ${expandedId === p.id ? 'bg-gray-800/30' : ''}`} onClick={() => toggleExpand(p.id)}>
                <td className="py-2 font-mono">
                  <span className="mr-1 text-gray-500 text-xs">{expandedId === p.id ? '▼' : '▶'}</span>
                  {p.symbol}
                </td>
                <td className="py-2 text-sm">
                  <a href={tradingUrl(p.exchange, p.symbol)} target="_blank" rel="noopener noreferrer"
                    className="text-green-400 hover:underline cursor-pointer" onClick={e => e.stopPropagation()}>{p.exchange}</a>
                </td>
                <td className="py-2">
                  {isDirA(p.direction) ? (
                    <span className="px-1.5 py-0.5 text-xs rounded bg-teal-500/20 text-teal-400">{t('spotPos.dirA')}</span>
                  ) : (
                    <span className="px-1.5 py-0.5 text-xs rounded bg-violet-500/20 text-violet-400">{t('spotPos.dirB')}</span>
                  )}
                </td>
                <td className="py-2">
                  <span className={`px-1.5 py-0.5 text-xs rounded ${statusBadge(p.status)}`}>{p.status}</span>
                </td>
                <td className="py-2 text-right font-mono">${n(p.notional_usdt).toFixed(2)}</td>
                <td className={`py-2 text-right font-mono ${pnlColor(n(p.current_net_yield_apr))}`}>
                  {(n(p.current_net_yield_apr) * 100).toFixed(1)}%
                  {p.yield_data_source === 'fallback' && <span className="text-gray-500 ml-0.5">*</span>}
                </td>
                <td className="py-2 text-right font-mono">
                  {isDirA(p.direction) ? (
                    <span className="text-red-400">-${n(p.borrow_cost_accrued).toFixed(2)}</span>
                  ) : (
                    <span className="text-gray-500">-</span>
                  )}
                </td>
                <td className="py-2 text-right font-mono text-red-400">
                  {n(p.entry_fees) > 0 ? `-$${n(p.entry_fees).toFixed(2)}` : '-'}
                </td>
                <td className={`py-2 text-right font-mono ${n(p.margin_utilization_pct) > 80 ? 'text-red-400' : n(p.margin_utilization_pct) > 50 ? 'text-amber-400' : 'text-gray-400'}`}>
                  {n(p.margin_utilization_pct).toFixed(0)}%
                </td>
                <td className="py-2 text-right font-mono text-gray-400">{formatAge(p.created_at)}</td>
                <td className="px-2 py-1 whitespace-nowrap">
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

              {/* Expanded row */}
              {expandedId === p.id && (
                <tr className="bg-gray-800/50">
                  <td colSpan={11} className="px-4 py-3">
                    <div className="space-y-4 text-sm">
                      {/* Section 1: Position Details */}
                      <div>
                        <div className="text-gray-400 text-xs mb-1">
                          Open: <span className="text-gray-200">{formatDateTime(p.created_at)}</span>
                        </div>
                        <div className="text-gray-400 text-xs mb-1">
                          {t('spotPos.dir')}: <span className="text-gray-200">
                            {isDirA(p.direction) ? t('spotPos.dirADesc') : t('spotPos.dirBDesc')}
                          </span>
                        </div>
                        <div className="text-gray-400 text-xs">
                          {t('spotPos.spotLeg')}:{' '}
                          <span className="font-mono text-gray-200">{n(p.spot_size).toFixed(6)} @ {formatPrice(n(p.spot_entry_price))}</span>
                        </div>
                        <div className="text-gray-400 text-xs">
                          {t('spotPos.futuresLeg')}:{' '}
                          <span className="font-mono text-gray-200">
                            {n(p.futures_size).toFixed(6)} @ {formatPrice(n(p.futures_entry))} ({p.futures_side || '-'})
                          </span>
                        </div>
                      </div>

                      {/* Section 2: Yield Breakdown */}
                      <div>
                        <div className="text-gray-300 text-xs font-semibold mb-1">{t('spotPos.yieldBreakdown')}</div>
                        <div className="text-gray-400 text-xs space-y-0.5">
                          <div>{t('spotPos.fundingApr')}: <span className={`font-mono ${pnlColor(n(p.current_funding_apr))}`}>{(n(p.current_funding_apr) * 100).toFixed(1)}%</span> (entry: {(n(p.funding_apr) * 100).toFixed(1)}%)</div>
                          {isDirA(p.direction) && (
                            <div>{t('spotPos.borrowApr')}: <span className="font-mono text-red-400">-{(n(p.current_borrow_apr) * 100).toFixed(1)}%</span></div>
                          )}
                          <div>{t('spotPos.feeImpact')}: <span className="font-mono text-red-400">-{(n(p.current_fee_pct) * 100).toFixed(2)}%</span></div>
                          <div>{t('spotPos.netYield')}: <span className={`font-mono ${pnlColor(n(p.current_net_yield_apr))}`}>{(n(p.current_net_yield_apr) * 100).toFixed(1)}%</span></div>
                          <div>{t('spotPos.dataSource')}: <span className="font-mono text-gray-200">{p.yield_data_source}</span></div>
                        </div>
                      </div>

                      {/* Section 3: Risk Metrics */}
                      <div>
                        <div className="text-gray-300 text-xs font-semibold mb-1">{t('spotPos.riskMetrics')}</div>
                        <div className="text-gray-400 text-xs space-y-0.5">
                          <div>
                            {t('spotPos.peakMove')}:{' '}
                            <span className={`font-mono ${n(p.peak_price_move_pct) > 10 ? 'text-red-400' : n(p.peak_price_move_pct) > 5 ? 'text-amber-400' : 'text-gray-200'}`}>
                              {n(p.peak_price_move_pct).toFixed(1)}%
                            </span>
                          </div>
                          <div>{t('spotPos.margin')}: <span className="font-mono text-gray-200">{n(p.margin_utilization_pct).toFixed(1)}%</span></div>
                          {isDirA(p.direction) && (
                            <div>{t('spotPos.borrowAmount')}: <span className="font-mono text-gray-200">{n(p.borrow_amount).toFixed(1)}</span></div>
                          )}
                        </div>
                      </div>

                      {/* Section 4: Exit Info (only for closed/exiting) */}
                      {(p.status === 'closed' || p.status === 'exiting') && (
                        <div>
                          <div className="text-gray-300 text-xs font-semibold mb-1">{t('spotPos.exitInfo')}</div>
                          <div className="text-gray-400 text-xs space-y-0.5">
                            {p.exit_reason && (
                              <div>{t('spotPos.exitReason')}: <span className="font-mono text-gray-200">{p.exit_reason}</span></div>
                            )}
                            <div>{t('spotPos.exitFees')}: <span className="font-mono text-red-400">-${n(p.exit_fees).toFixed(2)}</span></div>
                            <div>{t('spotPos.realizedPnl')}: <span className={`font-mono ${pnlColor(n(p.realized_pnl))}`}>${n(p.realized_pnl).toFixed(2)}</span></div>
                          </div>
                        </div>
                      )}
                    </div>
                  </td>
                </tr>
              )}
              </Fragment>
            ))}
            {displayData.length === 0 && (
              <tr>
                <td colSpan={11} className="py-4 text-center text-gray-500">{t('spotPos.noPositions')}</td>
              </tr>
            )}
          </tbody>
        </table>
      </div>

      {/* Close confirmation dialog */}
      {closingId && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="bg-gray-900 border border-gray-700 rounded-lg p-6 max-w-sm">
            <h3 className="text-lg font-semibold text-gray-100 mb-2">{t('spotPos.closeConfirmTitle')}</h3>
            <p className="text-gray-300 text-sm mb-4">
              {t('spotPos.closeConfirmMsg')}: <span className="font-mono font-bold">{displayData.find(p => p.id === closingId)?.symbol}</span>
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

export default SpotPositions;
