import { useState, useEffect, useCallback, type Dispatch, type FC, type SetStateAction } from 'react';
import type { Opportunity, SpotOpportunity, PriceGapResult } from '../types.ts';
import { useLocale } from '../i18n/index.ts';
import { ExchangeLink } from '../utils/tradingUrl.tsx';

type Tab = 'perp' | 'spot';
type TranslateFn = ReturnType<typeof useLocale>['t'];

interface SpotBacktestReport {
  sum_bps: number;
  projected_apr: number;
  settlement_count: number;
  coverage_pct: number;
  funding_bps?: number;
  borrow_bps?: number;
  days: { date: string; bps: number; funding_bps?: number; borrow_bps?: number }[];
}

interface BorrowResult {
  symbol: string;
  exchange: string;
  max_borrowable: number;
  error?: string;
}

interface OpportunitiesProps {
  opportunities: Opportunity[];
  spotOpportunities?: SpotOpportunity[];
  spotScannerMode?: string;
  onOpen?: (symbol: string, longExchange: string, shortExchange: string, force?: boolean) => Promise<void>;
  onSpotOpen?: (symbol: string, exchange: string, direction: string) => Promise<void>;
  onCheckPriceGap?: (symbol: string, exchange: string, direction: string) => Promise<PriceGapResult>;
  onBatchCheckGap?: (items: { symbol: string; exchange: string; direction: string }[]) => Promise<{ symbol: string; exchange: string; direction: string; gap_pct: number; error?: string }[]>;
  onBatchCheckBorrowable?: (items: { symbol: string; exchange: string }[]) => Promise<BorrowResult[]>;
  blacklist?: string[];
  onBlacklistToggle?: (symbol: string) => Promise<void>;
}

interface SpotSectionProps {
  title?: string;
  sourceLabel?: string;
  opportunities: SpotOpportunity[];
  t: TranslateFn;
  onSpotOpen?: (symbol: string, exchange: string, direction: string) => Promise<void>;
  onOpenBacktest?: (opp: SpotOpportunity) => void;
  gapResults: Record<string, PriceGapResult>;
  borrowResults: Record<string, BorrowResult>;
  spotOpening: string | null;
  setSpotOpening: (value: string | null) => void;
  setSpotError: (value: string | null) => void;
  page: number;
  setPage: (p: number) => void;
  onBatchCheckGap?: (items: { symbol: string; exchange: string; direction: string }[]) => Promise<{ symbol: string; exchange: string; direction: string; gap_pct: number; error?: string }[]>;
  onBatchCheckBorrowable?: (items: { symbol: string; exchange: string }[]) => Promise<BorrowResult[]>;
  setGapResults: Dispatch<SetStateAction<Record<string, PriceGapResult>>>;
  setBorrowResults: Dispatch<SetStateAction<Record<string, BorrowResult>>>;
  gapLoading: boolean;
  setGapLoading: (v: boolean) => void;
  borrowLoading: boolean;
  setBorrowLoading: (v: boolean) => void;
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

function isCoinGlassSource(source?: string): boolean {
  return source === 'coinglass_spot' || source === 'coinglass';
}

function getSpotSourceLabel(t: TranslateFn, source?: string): string {
  if (source === 'native') return t('spot.sourceNative');
  if (isCoinGlassSource(source)) return t('spot.sourceCoinGlass');
  return t('spot.sourceFallback');
}

const PAGE_SIZE = 20;

const SPOT_DIR_A_BACKTEST_EXCHANGES = new Set(['binance', 'bybit', 'gateio']);

function canBacktest(opp: SpotOpportunity): { allowed: boolean; reason?: string } {
  if (opp.direction === 'buy_spot_short') return { allowed: true };
  if (opp.direction === 'borrow_sell_long' && SPOT_DIR_A_BACKTEST_EXCHANGES.has(opp.exchange)) {
    return { allowed: true };
  }
  return { allowed: false, reason: opp.exchange };
}

// ---------------------------------------------------------------------------
// SpotBacktestModal — on-demand backtest panel (Dir A + Dir B)
// ---------------------------------------------------------------------------
const SpotBacktestModal: FC<{ opp: SpotOpportunity; t: TranslateFn; onClose: () => void }> = ({ opp, t, onClose }) => {
  const [days, setDays] = useState(7);
  const [result, setResult] = useState<SpotBacktestReport | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const isA = opp.direction === 'borrow_sell_long';

  const handleRun = async () => {
    setLoading(true);
    setError(null);
    setResult(null);
    try {
      const token = localStorage.getItem('arb_token');
      const res = await fetch('/api/spot/backtest', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          ...(token ? { Authorization: `Bearer ${token}` } : {}),
        },
        body: JSON.stringify({ symbol: opp.symbol, exchange: opp.exchange, direction: opp.direction, days }),
      });
      if (!res.ok) {
        const body = await res.json().catch(() => null) as { error?: string } | null;
        throw new Error(body?.error || `HTTP ${res.status}`);
      }
      const data = await res.json() as { ok: boolean; data?: SpotBacktestReport; error?: string };
      if (!data.ok) throw new Error(data.error || 'Request failed');
      setResult(data.data ?? null);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed');
    } finally {
      setLoading(false);
    }
  };

  const maxBps = result && result.days.length > 0 ? Math.max(...result.days.map(d => Math.abs(d.bps)), 1) : 1;

  return (
    <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-50" onClick={onClose}>
      <div className="bg-gray-900 border border-gray-700 rounded-lg p-6 max-w-lg w-full mx-4 max-h-[90vh] overflow-y-auto" onClick={e => e.stopPropagation()}>
        <div className="flex items-center justify-between mb-4">
          <h3 className="text-lg font-bold text-gray-100">{t('spotBacktest.modal.title')}</h3>
          <button onClick={onClose} className="text-gray-500 hover:text-gray-200 text-xl leading-none">&times;</button>
        </div>

        <div className="flex items-center gap-3 mb-4 text-sm">
          <span className="font-mono font-bold text-gray-100">{opp.symbol}</span>
          <span className="text-gray-400">{opp.exchange}</span>
          <span className={`px-1.5 py-0.5 text-xs rounded ${isA ? 'bg-blue-500/20 text-blue-400' : 'bg-violet-500/20 text-violet-400'}`}>
            {isA ? 'Dir A' : 'Dir B'}
          </span>
        </div>

        <>
            <div className="flex items-center gap-3 mb-4">
              <label className="text-sm text-gray-400">Days:</label>
              <input
                type="number"
                min={1}
                max={30}
                value={days}
                onChange={(e) => setDays(Math.max(1, Math.min(30, parseInt(e.target.value, 10) || 7)))}
                className="w-20 bg-gray-800 border border-gray-700 rounded px-2 py-1 text-sm text-gray-100 focus:outline-none focus:border-blue-500"
              />
              <button
                disabled={loading}
                onClick={handleRun}
                className={`px-4 py-1.5 text-sm font-semibold rounded transition-colors ${loading ? 'bg-gray-700 text-gray-400 cursor-wait' : 'bg-violet-600/30 text-violet-300 hover:bg-violet-600/50'}`}
              >
                {loading ? t('spotBacktest.modal.loading') : t('spotBacktest.modal.run')}
              </button>
            </div>

            {error && (
              <div className="text-red-400 text-sm mb-3">{t('spotBacktest.modal.error')}: {error}</div>
            )}

            {result && (
              <div className="space-y-3">
                {opp.direction === 'borrow_sell_long' ? (
                  <>
                    <div className="grid grid-cols-3 gap-2">
                      <div className="bg-gray-800 rounded-lg p-2.5">
                        <div className="text-gray-500 text-[10px] uppercase tracking-wide mb-0.5">{t('spotBacktest.modal.fundingBps')}</div>
                        <div className={`font-mono text-sm font-bold ${(result.funding_bps ?? 0) >= 0 ? 'text-emerald-400' : 'text-red-400'}`}>{(result.funding_bps ?? 0).toFixed(1)}</div>
                      </div>
                      <div className="bg-gray-800 rounded-lg p-2.5">
                        <div className="text-gray-500 text-[10px] uppercase tracking-wide mb-0.5">{t('spotBacktest.modal.borrowBps')}</div>
                        <div className={`font-mono text-sm font-bold ${(result.borrow_bps ?? 0) <= 0 ? 'text-emerald-400' : 'text-amber-400'}`}>{(result.borrow_bps ?? 0).toFixed(1)}</div>
                      </div>
                      <div className="bg-gray-800 rounded-lg p-2.5">
                        <div className="text-gray-500 text-[10px] uppercase tracking-wide mb-0.5">{t('spotBacktest.modal.netBps')}</div>
                        <div className={`font-mono text-sm font-bold ${result.sum_bps >= 0 ? 'text-emerald-400' : 'text-red-400'}`}>{result.sum_bps.toFixed(1)}</div>
                      </div>
                    </div>
                    <div className="grid grid-cols-3 gap-2">
                      <div className="bg-gray-800 rounded-lg p-2.5">
                        <div className="text-gray-500 text-[10px] uppercase tracking-wide mb-0.5">{t('spotBacktest.modal.projectedApr')}</div>
                        <div className={`font-mono text-sm font-bold ${result.projected_apr >= 0 ? 'text-emerald-400' : 'text-red-400'}`}>{(result.projected_apr * 100).toFixed(1)}%</div>
                      </div>
                      <div className="bg-gray-800 rounded-lg p-2.5">
                        <div className="text-gray-500 text-[10px] uppercase tracking-wide mb-0.5">{t('spotBacktest.modal.settlements')}</div>
                        <div className="font-mono text-sm font-bold text-gray-100">{result.settlement_count}</div>
                      </div>
                      <div className="bg-gray-800 rounded-lg p-2.5">
                        <div className="text-gray-500 text-[10px] uppercase tracking-wide mb-0.5">{t('spotBacktest.modal.coverage')}</div>
                        <div className={`font-mono text-sm font-bold ${result.coverage_pct >= 80 ? 'text-emerald-400' : result.coverage_pct >= 50 ? 'text-amber-400' : 'text-red-400'}`}>{result.coverage_pct.toFixed(0)}%</div>
                      </div>
                    </div>
                  </>
                ) : (
                  <div className="grid grid-cols-2 sm:grid-cols-4 gap-2">
                    <div className="bg-gray-800 rounded-lg p-2.5">
                      <div className="text-gray-500 text-[10px] uppercase tracking-wide mb-0.5">{t('spotBacktest.modal.sumBps')}</div>
                      <div className={`font-mono text-sm font-bold ${result.sum_bps >= 0 ? 'text-emerald-400' : 'text-red-400'}`}>{result.sum_bps.toFixed(1)}</div>
                    </div>
                    <div className="bg-gray-800 rounded-lg p-2.5">
                      <div className="text-gray-500 text-[10px] uppercase tracking-wide mb-0.5">{t('spotBacktest.modal.projectedApr')}</div>
                      <div className={`font-mono text-sm font-bold ${result.projected_apr >= 0 ? 'text-emerald-400' : 'text-red-400'}`}>{(result.projected_apr * 100).toFixed(1)}%</div>
                    </div>
                    <div className="bg-gray-800 rounded-lg p-2.5">
                      <div className="text-gray-500 text-[10px] uppercase tracking-wide mb-0.5">{t('spotBacktest.modal.settlements')}</div>
                      <div className="font-mono text-sm font-bold text-gray-100">{result.settlement_count}</div>
                    </div>
                    <div className="bg-gray-800 rounded-lg p-2.5">
                      <div className="text-gray-500 text-[10px] uppercase tracking-wide mb-0.5">{t('spotBacktest.modal.coverage')}</div>
                      <div className={`font-mono text-sm font-bold ${result.coverage_pct >= 80 ? 'text-emerald-400' : result.coverage_pct >= 50 ? 'text-amber-400' : 'text-red-400'}`}>{result.coverage_pct.toFixed(0)}%</div>
                    </div>
                  </div>
                )}

                {result.days.length > 0 && (
                  <div className="bg-gray-800 rounded-lg p-3">
                    <div className="text-gray-500 text-[10px] uppercase tracking-wide mb-2">Daily Funding (bps)</div>
                    <div className="space-y-1">
                      {result.days.map((d, i) => (
                        <div key={i} className="flex items-center gap-2 text-[10px]"
                          title={opp.direction === 'borrow_sell_long'
                            ? `Funding: ${(d.funding_bps ?? 0).toFixed(1)} bps | Borrow: ${(d.borrow_bps ?? 0).toFixed(1)} bps | Net: ${d.bps.toFixed(1)} bps`
                            : undefined}>
                          <span className="text-gray-600 w-16 shrink-0 font-mono">{d.date}</span>
                          <div className="flex-1 bg-gray-700 rounded-full h-2">
                            <div
                              className={`h-2 rounded-full ${d.bps >= 0 ? 'bg-emerald-500' : 'bg-red-500'}`}
                              style={{ width: `${Math.max(2, (Math.abs(d.bps) / maxBps) * 100)}%` }}
                            />
                          </div>
                          <span className={`w-12 text-right font-mono shrink-0 ${d.bps >= 0 ? 'text-emerald-400' : 'text-red-400'}`}>{d.bps.toFixed(1)}</span>
                        </div>
                      ))}
                    </div>
                  </div>
                )}
              </div>
            )}
          </>
      </div>
    </div>
  );
};

// ---------------------------------------------------------------------------
// Spot opportunity row (shared between full and compact modes)
// ---------------------------------------------------------------------------
interface SpotRowProps {
  opp: SpotOpportunity;
  index: number;
  compact?: boolean;
  t: TranslateFn;
  onSpotOpen?: (symbol: string, exchange: string, direction: string) => Promise<void>;
  onOpenBacktest?: (opp: SpotOpportunity) => void;
  gapResult?: PriceGapResult;
  borrowResult?: BorrowResult;
  spotOpening: string | null;
  setSpotOpening: (v: string | null) => void;
  setSpotError: (v: string | null) => void;
}

const SpotRow: FC<SpotRowProps> = ({ opp, index, compact, t, onSpotOpen, onOpenBacktest, gapResult, borrowResult, spotOpening, setSpotOpening, setSpotError }) => {
  const filtered = !!opp.filter_status;
  const isA = opp.direction === 'borrow_sell_long';
  const oppKey = `${opp.symbol}-${opp.exchange}-${opp.direction}`;
  const dirLabel = isA ? t('spot.dirA') : t('spot.dirB');
  const dirDesc = isA ? t('spot.dirADesc') : t('spot.dirBDesc');

  const handleOpen = async () => {
    if (!onSpotOpen) return;
    setSpotOpening(oppKey);
    setSpotError(null);
    try {
      await onSpotOpen(opp.symbol, opp.exchange, opp.direction);
    } catch (err: unknown) {
      setSpotError(err instanceof Error ? err.message : 'Open failed');
    } finally {
      setSpotOpening(null);
    }
  };

  // Borrow warning: max_borrowable too low (< 5 USDT worth)
  const borrowWarn = isA && borrowResult && !borrowResult.error && borrowResult.max_borrowable <= 0;

  if (compact) {
    const detailTooltip = `Funding: ${(opp.funding_apr * 100).toFixed(1)}% | Borrow: ${opp.borrow_apr > 0 ? (opp.borrow_apr * 100).toFixed(1) + '%' : '-'} | Fees: ${(opp.fee_pct * 100).toFixed(2)}%${opp.filter_status ? ' | ' + opp.filter_status : ''}`;
    return (
      <tr className={`${filtered ? 'text-gray-600' : borrowWarn ? 'text-gray-100 bg-red-500/5' : 'text-gray-100 hover:bg-gray-800/40'}`} title={detailTooltip}>
        <td className="py-1.5 pr-2 font-mono text-gray-500 text-xs">{index + 1}</td>
        <td className="py-1.5 pr-2 font-mono text-xs">{opp.symbol}</td>
        <td className="py-1.5 pr-2 capitalize text-xs">
          {filtered ? opp.exchange : <ExchangeLink exchange={opp.exchange} symbol={opp.symbol} className="text-gray-100" />}
        </td>
        <td className={`py-1.5 pr-2 font-mono text-xs ${filtered ? '' : isA ? 'text-blue-400' : 'text-purple-400'}`}>{dirLabel}</td>
        <td className={`py-1.5 pr-2 text-right font-mono text-xs font-semibold ${filtered ? '' : opp.net_apr >= 0 ? 'text-emerald-400' : 'text-red-400'}`}>{(opp.net_apr * 100).toFixed(1)}%</td>
        <td className="py-1.5 pr-2 text-right font-mono text-[10px]">
          {opp.maintenance_rate > 0 ? (
            <span className={opp.maintenance_rate >= 0.10 ? 'text-red-400' : opp.maintenance_rate >= 0.05 ? 'text-amber-400' : 'text-gray-400'}>
              {(opp.maintenance_rate * 100).toFixed(1)}%
            </span>
          ) : <span className="text-gray-600">-</span>}
        </td>
        <td className="py-1.5 pr-2 text-right font-mono text-[10px]">
          {gapResult && !filtered && (
            gapResult.gap_pct <= -999 ? <span className="text-gray-600">N/A</span>
            : <span className={gapResult.gap_pct <= 0 ? 'text-emerald-400' : gapResult.gap_pct < 0.5 ? 'text-amber-400' : 'text-red-400'}>{gapResult.gap_pct.toFixed(3)}%</span>
          )}
        </td>
        <td className="py-1.5 pr-2 text-right font-mono text-[10px]">
          {!isA && !filtered && <span className="text-gray-600">—</span>}
          {isA && borrowResult && !filtered && (
            borrowResult.error ? <span className="text-gray-600" title={borrowResult.error}>err</span>
            : borrowResult.max_borrowable <= 0 ? <span className="text-red-400">0</span>
            : <span className="text-emerald-400">{borrowResult.max_borrowable.toFixed(2)}</span>
          )}
        </td>
        <td className="py-1.5 text-right whitespace-nowrap">
          <div className="flex items-center justify-end gap-1">
            {onSpotOpen && !filtered && !borrowWarn && (
              <button disabled={spotOpening === oppKey} onClick={handleOpen}
                className={`px-1.5 py-0.5 text-[10px] rounded transition-colors ${spotOpening === oppKey ? 'bg-gray-700 text-gray-400 cursor-wait' : 'bg-emerald-600/20 text-emerald-400 hover:bg-emerald-600/40'}`}>
                {spotOpening === oppKey ? '..' : 'Open'}
              </button>
            )}
            {onOpenBacktest && (() => {
              const bt = canBacktest(opp);
              return bt.allowed ? (
                <button onClick={() => onOpenBacktest(opp)}
                  className="px-1.5 py-0.5 text-[10px] rounded transition-colors bg-violet-600/20 text-violet-400 hover:bg-violet-600/40">
                  BT
                </button>
              ) : (
                <span title={opp.exchange === 'bingx' ? t('spotBacktest.modal.notSupportedDirA') : t('spotBacktest.modal.notSupportedExchange').replace('{exchange}', bt.reason ?? opp.exchange)}
                  className="px-1.5 py-0.5 text-[10px] rounded bg-gray-700/40 text-gray-600 cursor-not-allowed select-none">
                  BT
                </span>
              );
            })()}
            {filtered && !onSpotOpen && !onOpenBacktest && <span className="w-1.5 h-1.5 rounded-full bg-gray-600 inline-block" />}
          </div>
        </td>
      </tr>
    );
  }

  // Full row for single-source view
  return (
    <tr key={oppKey} className={`${filtered ? 'text-gray-600' : borrowWarn ? 'text-gray-100 bg-red-500/5' : 'text-gray-100'}`}>
      <td className="py-2 font-mono text-gray-500">{index + 1}</td>
      <td className="py-2 font-mono">{opp.symbol}</td>
      <td className="py-2 capitalize">
        {filtered ? opp.exchange : <ExchangeLink exchange={opp.exchange} symbol={opp.symbol} className="text-gray-100" />}
      </td>
      <td className={`py-2 font-mono ${filtered ? '' : isA ? 'text-blue-400' : 'text-purple-400'}`} title={dirDesc}>
        {dirLabel} <span className="text-xs text-gray-500 font-normal hidden sm:inline">({dirDesc})</span>
      </td>
      <td className={`py-2 text-right font-mono ${filtered ? '' : 'text-emerald-400'}`}>{(opp.funding_apr * 100).toFixed(1)}%</td>
      <td className={`py-2 text-right font-mono ${filtered ? '' : 'text-amber-400'}`}>{opp.borrow_apr > 0 ? `${(opp.borrow_apr * 100).toFixed(1)}%` : '-'}</td>
      <td className={`py-2 text-right font-mono ${filtered ? '' : 'text-gray-400'}`}>{(opp.fee_pct * 100).toFixed(2)}%</td>
      <td className={`py-2 text-right font-mono font-semibold ${filtered ? '' : opp.net_apr >= 0 ? 'text-emerald-400' : 'text-red-400'}`}>{(opp.net_apr * 100).toFixed(1)}%</td>
      <td className="py-2 text-right font-mono text-xs">
        {opp.maintenance_rate > 0 ? (
          <span className={opp.maintenance_rate >= 0.10 ? 'text-red-400' : opp.maintenance_rate >= 0.05 ? 'text-amber-400' : 'text-gray-400'}>
            {(opp.maintenance_rate * 100).toFixed(2)}%
          </span>
        ) : (
          <span className="text-gray-600">-</span>
        )}
      </td>
      <td className="py-2 text-right font-mono text-xs">
        {gapResult && !filtered && (
          gapResult.gap_pct < 0 ? <span className="text-gray-600">N/A</span>
          : <span className={gapResult.gap_pct < 0.3 ? 'text-emerald-400' : 'text-amber-400'}>{gapResult.gap_pct.toFixed(3)}%</span>
        )}
      </td>
      <td className="py-2 text-right font-mono text-xs">
        {!isA && !filtered && <span className="text-gray-600">—</span>}
        {isA && borrowResult && !filtered && (
          borrowResult.error ? <span className="text-gray-600" title={borrowResult.error}>err</span>
          : borrowResult.max_borrowable <= 0 ? <span className="text-red-400">0</span>
          : <span className="text-emerald-400">{borrowResult.max_borrowable.toFixed(2)}</span>
        )}
      </td>
      <td className="py-2">
        {filtered
          ? <span className="inline-flex items-center gap-1 text-xs text-gray-600"><span className="w-1.5 h-1.5 rounded-full bg-gray-600" />{t('spot.filtered')}</span>
          : <span className="inline-flex items-center gap-1 text-xs text-emerald-400"><span className="w-1.5 h-1.5 rounded-full bg-emerald-400" />{t('spot.ready')}</span>}
      </td>
      <td className="px-2 py-1">
        <div className="flex items-center gap-1.5">
          {onSpotOpen && !filtered && !borrowWarn && (
            <button disabled={spotOpening === oppKey} onClick={handleOpen}
              className={`px-2 py-0.5 text-xs rounded transition-colors ${spotOpening === oppKey ? 'bg-gray-700 text-gray-400 cursor-wait' : 'bg-emerald-600/20 text-emerald-400 hover:bg-emerald-600/40'}`}>
              {spotOpening === oppKey ? 'Opening...' : 'Open'}
            </button>
          )}
          {onOpenBacktest && (() => {
            const bt = canBacktest(opp);
            return bt.allowed ? (
              <button onClick={() => onOpenBacktest(opp)}
                className="px-2 py-0.5 text-xs rounded transition-colors bg-violet-600/20 text-violet-400 hover:bg-violet-600/40">
                Backtest
              </button>
            ) : (
              <span title={opp.exchange === 'bingx' ? t('spotBacktest.modal.notSupportedDirA') : t('spotBacktest.modal.notSupportedExchange').replace('{exchange}', bt.reason ?? opp.exchange)}
                className="px-2 py-0.5 text-xs rounded bg-gray-700/40 text-gray-600 cursor-not-allowed select-none">
                Backtest
              </span>
            );
          })()}
        </div>
      </td>
    </tr>
  );
};

// ---------------------------------------------------------------------------
// SpotCard — mobile (< md) card rendering of a spot opportunity
// ---------------------------------------------------------------------------
const SpotCard: FC<SpotRowProps> = ({ opp, index, t, onSpotOpen, onOpenBacktest, gapResult, borrowResult, spotOpening, setSpotOpening, setSpotError }) => {
  const filtered = !!opp.filter_status;
  const isA = opp.direction === 'borrow_sell_long';
  const oppKey = `${opp.symbol}-${opp.exchange}-${opp.direction}`;
  const dirLabel = isA ? t('spot.dirA') : t('spot.dirB');
  const borrowWarn = isA && borrowResult && !borrowResult.error && borrowResult.max_borrowable <= 0;

  const handleOpen = async () => {
    if (!onSpotOpen) return;
    setSpotOpening(oppKey);
    setSpotError(null);
    try {
      await onSpotOpen(opp.symbol, opp.exchange, opp.direction);
    } catch (err: unknown) {
      setSpotError(err instanceof Error ? err.message : 'Open failed');
    } finally {
      setSpotOpening(null);
    }
  };

  return (
    <div
      className={`rounded-lg border px-3 py-2.5 ${
        filtered
          ? 'border-gray-800/50 bg-gray-900/30 opacity-60'
          : borrowWarn
          ? 'border-red-500/30 bg-red-500/5'
          : 'border-gray-800 bg-gray-900'
      }`}
    >
      <div className="flex items-center justify-between gap-2 mb-1.5">
        <div className="flex items-center gap-1.5 min-w-0">
          <span className="font-mono text-[10px] text-gray-500 shrink-0">{index + 1}</span>
          <span className="font-mono font-semibold text-gray-100 truncate">{opp.symbol}</span>
          <span className={`shrink-0 px-1.5 py-0.5 text-[10px] font-bold rounded ${
            filtered ? 'bg-gray-700/40 text-gray-500' : isA ? 'bg-blue-500/15 text-blue-400' : 'bg-purple-500/15 text-purple-400'
          }`}>{dirLabel}</span>
        </div>
        <span className={`font-mono font-bold tabular-nums shrink-0 ${
          filtered ? 'text-gray-500' : opp.net_apr >= 0 ? 'text-emerald-400' : 'text-red-400'
        }`}>{(opp.net_apr * 100).toFixed(1)}%</span>
      </div>

      <div className="flex items-center gap-2 text-xs mb-2">
        <span className="text-gray-500 uppercase tracking-wider text-[10px]">{t('spot.exchange')}:</span>
        {filtered ? (
          <span className="text-gray-500 capitalize">{opp.exchange}</span>
        ) : (
          <ExchangeLink exchange={opp.exchange} symbol={opp.symbol} className="text-gray-200" />
        )}
      </div>

      <div className="grid grid-cols-4 gap-2 text-[11px]">
        <div>
          <div className="text-gray-500 text-[9px] uppercase tracking-wide">{t('spot.funding')}</div>
          <div className={`font-mono tabular-nums ${filtered ? 'text-gray-500' : 'text-emerald-400'}`}>{(opp.funding_apr * 100).toFixed(1)}%</div>
        </div>
        <div>
          <div className="text-gray-500 text-[9px] uppercase tracking-wide">{t('spot.borrow')}</div>
          <div className={`font-mono tabular-nums ${filtered ? 'text-gray-500' : 'text-amber-400'}`}>{opp.borrow_apr > 0 ? `${(opp.borrow_apr * 100).toFixed(1)}%` : '-'}</div>
        </div>
        <div>
          <div className="text-gray-500 text-[9px] uppercase tracking-wide">{t('spot.fees')}</div>
          <div className="font-mono tabular-nums text-gray-400">{(opp.fee_pct * 100).toFixed(2)}%</div>
        </div>
        <div>
          <div className="text-gray-500 text-[9px] uppercase tracking-wide" title={t('spot.maintenanceRateTooltip')}>MR</div>
          <div className="font-mono tabular-nums">
            {opp.maintenance_rate > 0 ? (
              <span className={opp.maintenance_rate >= 0.10 ? 'text-red-400' : opp.maintenance_rate >= 0.05 ? 'text-amber-400' : 'text-gray-400'}>
                {(opp.maintenance_rate * 100).toFixed(1)}%
              </span>
            ) : <span className="text-gray-600">-</span>}
          </div>
        </div>
      </div>

      <div className="flex items-center justify-between gap-2 mt-2 pt-2 border-t border-gray-800/50">
        <div className="flex items-center gap-3 text-[11px]">
          {!filtered && (
            <>
              <span>
                <span className="text-gray-500 text-[9px] uppercase tracking-wide mr-1">Gap</span>
                {gapResult ? (
                  gapResult.gap_pct <= -999 ? <span className="text-gray-600">N/A</span>
                  : <span className={`font-mono tabular-nums ${gapResult.gap_pct <= 0 ? 'text-emerald-400' : gapResult.gap_pct < 0.5 ? 'text-amber-400' : 'text-red-400'}`}>{gapResult.gap_pct.toFixed(3)}%</span>
                ) : <span className="text-gray-600">-</span>}
              </span>
              {isA && (
                <span>
                  <span className="text-gray-500 text-[9px] uppercase tracking-wide mr-1">Bor</span>
                  {borrowResult ? (
                    borrowResult.error ? <span className="text-gray-600" title={borrowResult.error}>err</span>
                    : borrowResult.max_borrowable <= 0 ? <span className="text-red-400 font-mono">0</span>
                    : <span className="text-emerald-400 font-mono tabular-nums">{borrowResult.max_borrowable.toFixed(2)}</span>
                  ) : <span className="text-gray-600">-</span>}
                </span>
              )}
            </>
          )}
          {filtered && <span className="text-gray-500 text-[10px]">{opp.filter_status}</span>}
        </div>
        <div className="flex items-center gap-1.5">
          {onSpotOpen && !filtered && !borrowWarn && (
            <button
              disabled={spotOpening === oppKey}
              onClick={handleOpen}
              className={`px-3 py-1 text-xs font-semibold rounded transition-colors ${
                spotOpening === oppKey ? 'bg-gray-700 text-gray-400 cursor-wait' : 'bg-emerald-600/20 text-emerald-400 hover:bg-emerald-600/40'
              }`}
            >
              {spotOpening === oppKey ? '...' : 'Open'}
            </button>
          )}
          {onOpenBacktest && (() => {
            const bt = canBacktest(opp);
            return bt.allowed ? (
              <button onClick={() => onOpenBacktest(opp)}
                className="px-3 py-1 text-xs font-semibold rounded transition-colors bg-violet-600/20 text-violet-400 hover:bg-violet-600/40">
                BT
              </button>
            ) : (
              <span title={opp.exchange === 'bingx' ? t('spotBacktest.modal.notSupportedDirA') : t('spotBacktest.modal.notSupportedExchange').replace('{exchange}', bt.reason ?? opp.exchange)}
                className="px-3 py-1 text-xs font-semibold rounded bg-gray-700/40 text-gray-600 cursor-not-allowed select-none">
                BT
              </span>
            );
          })()}
        </div>
      </div>
    </div>
  );
};

// ---------------------------------------------------------------------------
// SpotSection — renders a panel of spot opportunities (full or compact)
// ---------------------------------------------------------------------------
// ---------------------------------------------------------------------------
// Pagination controls
// ---------------------------------------------------------------------------
const Pagination: FC<{ page: number; total: number; setPage: (p: number) => void }> = ({ page, total, setPage }) => {
  const totalPages = Math.ceil(total / PAGE_SIZE);
  if (totalPages <= 1) return null;
  return (
    <div className="flex items-center justify-between pt-2 border-t border-gray-800/50">
      <span className="text-[10px] text-gray-500">{page * PAGE_SIZE + 1}–{Math.min((page + 1) * PAGE_SIZE, total)} of {total}</span>
      <div className="flex gap-1">
        <button onClick={() => setPage(page - 1)} disabled={page === 0}
          className="px-2 py-0.5 text-[10px] rounded bg-gray-800 text-gray-400 hover:text-gray-200 disabled:opacity-30 disabled:cursor-not-allowed">&lt;</button>
        <button onClick={() => setPage(page + 1)} disabled={page >= totalPages - 1}
          className="px-2 py-0.5 text-[10px] rounded bg-gray-800 text-gray-400 hover:text-gray-200 disabled:opacity-30 disabled:cursor-not-allowed">&gt;</button>
      </div>
    </div>
  );
};

// ---------------------------------------------------------------------------
// SpotSection — renders a panel of spot opportunities with pagination + batch actions
// ---------------------------------------------------------------------------
const SpotSection: FC<SpotSectionProps & { compact?: boolean; accent?: 'emerald' | 'amber' }> = ({
  title,
  sourceLabel,
  opportunities,
  t,
  compact,
  accent = 'emerald',
  page,
  setPage,
  gapResults,
  borrowResults,
  onBatchCheckGap,
  onBatchCheckBorrowable,
  setGapResults,
  setBorrowResults,
  gapLoading,
  setGapLoading,
  borrowLoading,
  setBorrowLoading,
  ...rowProps
}) => {
  const actionableCount = opportunities.filter((opp) => !opp.filter_status).length;
  const bestApr = opportunities.reduce((best, o) => !o.filter_status && o.net_apr > best ? o.net_apr : best, -Infinity);
  const accentBorder = accent === 'amber' ? 'border-l-amber-500/60' : 'border-l-emerald-500/60';
  const accentBg = accent === 'amber' ? 'bg-amber-500/5' : 'bg-emerald-500/5';
  const accentText = accent === 'amber' ? 'text-amber-400' : 'text-emerald-400';
  const accentBadge = accent === 'amber' ? 'bg-amber-500/20 text-amber-300' : 'bg-emerald-500/20 text-emerald-300';

  const pageOpps = opportunities.slice(page * PAGE_SIZE, (page + 1) * PAGE_SIZE);

  const handleBatchGap = async () => {
    if (!onBatchCheckGap || gapLoading) return;
    setGapLoading(true);
    try {
      const items = pageOpps.filter(o => !o.filter_status).map(o => ({ symbol: o.symbol, exchange: o.exchange, direction: o.direction }));
      if (items.length === 0) return;
      const results = await onBatchCheckGap(items);
      setGapResults(prev => {
        const next = { ...prev };
        for (const r of results) {
          const key = `${r.symbol}-${r.exchange}-${r.direction}`;
          if (r.error) {
            next[key] = { gap_pct: -999, spot_bid: 0, spot_ask: 0, futures_bid: 0, futures_ask: 0, direction: r.direction };
          } else {
            next[key] = { gap_pct: r.gap_pct, spot_bid: 0, spot_ask: 0, futures_bid: 0, futures_ask: 0, direction: r.direction };
          }
        }
        return next;
      });
    } catch { /* ignore */ } finally { setGapLoading(false); }
  };

  const handleBatchBorrow = async () => {
    if (!onBatchCheckBorrowable || borrowLoading) return;
    setBorrowLoading(true);
    try {
      const dirAOpps = pageOpps.filter(o => !o.filter_status && o.direction === 'borrow_sell_long');
      const items = dirAOpps.map(o => ({ symbol: o.symbol, exchange: o.exchange }));
      if (items.length === 0) return;
      const results = await onBatchCheckBorrowable(items);
      setBorrowResults(prev => {
        const next = { ...prev };
        for (const r of results) next[`${r.symbol}-${r.exchange}`] = r;
        return next;
      });
    } catch { /* ignore */ } finally { setBorrowLoading(false); }
  };

  const actionBar = (
    <div className="flex items-center gap-2">
      {onBatchCheckGap && (
        <button onClick={handleBatchGap} disabled={gapLoading}
          className={`px-2.5 py-1 text-[10px] font-semibold rounded transition-colors ${gapLoading ? 'bg-gray-700 text-gray-400 cursor-wait' : 'bg-sky-600/20 text-sky-300 hover:bg-sky-600/40'}`}>
          {gapLoading ? t('spot.batchChecking') : t('spot.batchCheckGap')}
        </button>
      )}
      {onBatchCheckBorrowable && (
        <button onClick={handleBatchBorrow} disabled={borrowLoading}
          className={`px-2.5 py-1 text-[10px] font-semibold rounded transition-colors ${borrowLoading ? 'bg-gray-700 text-gray-400 cursor-wait' : 'bg-violet-600/20 text-violet-300 hover:bg-violet-600/40'}`}>
          {borrowLoading ? t('spot.batchChecking') : t('spot.batchCheckBorrow')}
        </button>
      )}
    </div>
  );

  if (compact) {
    return (
      <div className={`bg-gray-900 border border-gray-800 border-l-2 ${accentBorder} rounded-lg overflow-hidden`}>
        <div className={`flex items-center justify-between px-4 py-2.5 ${accentBg} border-b border-gray-800`}>
          <div className="flex items-center gap-2">
            <h3 className={`text-xs font-bold uppercase tracking-wider ${accentText}`}>{title}</h3>
            <span className={`px-1.5 py-0.5 text-[10px] font-bold rounded-full ${accentBadge}`}>{opportunities.length}</span>
          </div>
          <div className="flex items-center gap-3">
            {actionBar}
            <div className="text-[10px] text-gray-500">
              <span className="inline-block w-1.5 h-1.5 rounded-full bg-emerald-400 mr-1" />{actionableCount} ready
              {bestApr > -Infinity && <span className="ml-2 font-mono text-emerald-400">Best: {(bestApr * 100).toFixed(1)}%</span>}
            </div>
          </div>
        </div>
        <div className="px-3 py-2">
          {/* Desktop table */}
          <div className="hidden md:block overflow-x-auto">
            <table className="w-full text-xs">
              <thead>
                <tr className="text-gray-500 text-left border-b border-gray-800/50">
                  <th className="pb-1.5 pr-2">#</th>
                  <th className="pb-1.5 pr-2">{t('spot.symbol')}</th>
                  <th className="pb-1.5 pr-2">{t('spot.exchange')}</th>
                  <th className="pb-1.5 pr-2">Dir</th>
                  <th className="pb-1.5 pr-2 text-right">{t('spot.netApr')}</th>
                  <th className="pb-1.5 pr-2 text-right" title={t('spot.maintenanceRateTooltip')}>MR</th>
                  <th className="pb-1.5 pr-2 text-right">Gap</th>
                  <th className="pb-1.5 pr-2 text-right">Borrow</th>
                  <th className="pb-1.5 text-right"></th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-800/30">
                {pageOpps.map((opp, i) => {
                  const oppKey = `${opp.symbol}-${opp.exchange}-${opp.direction}`;
                  const borrowKey = `${opp.symbol}-${opp.exchange}`;
                  return <SpotRow key={oppKey} opp={opp} index={page * PAGE_SIZE + i} compact t={t}
                    gapResult={gapResults[oppKey]} borrowResult={borrowResults[borrowKey]} {...rowProps} />;
                })}
                {pageOpps.length === 0 && (
                  <tr><td colSpan={9} className="py-6 text-center text-gray-600 text-xs">{t('spot.noOpportunities')}</td></tr>
                )}
              </tbody>
            </table>
          </div>
          {/* Mobile card list */}
          <div className="md:hidden space-y-2">
            {pageOpps.length === 0 && (
              <div className="py-6 text-center text-gray-600 text-xs">{t('spot.noOpportunities')}</div>
            )}
            {pageOpps.map((opp, i) => {
              const oppKey = `${opp.symbol}-${opp.exchange}-${opp.direction}`;
              const borrowKey = `${opp.symbol}-${opp.exchange}`;
              return <SpotCard key={oppKey} opp={opp} index={page * PAGE_SIZE + i} t={t}
                gapResult={gapResults[oppKey]} borrowResult={borrowResults[borrowKey]} {...rowProps} />;
            })}
          </div>
          <Pagination page={page} total={opportunities.length} setPage={setPage} />
        </div>
      </div>
    );
  }

  // Full-width single-source view
  return (
    <div className="bg-gray-900 border border-gray-800 rounded-lg p-4">
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-2 mb-3">
        <div className="flex items-center gap-4 text-xs text-gray-500 flex-wrap">
          {title && <h3 className="text-sm font-semibold text-gray-100 uppercase tracking-wide">{title}</h3>}
          <span><span className="inline-block w-2 h-2 rounded-full bg-emerald-400 mr-1.5" />{actionableCount} {t('spot.actionable')}</span>
          <span><span className="inline-block w-2 h-2 rounded-full bg-gray-600 mr-1.5" />{opportunities.length - actionableCount} {t('spot.filtered')}</span>
          {sourceLabel && <span>{t('spot.source')}: {sourceLabel}</span>}
        </div>
        {actionBar}
      </div>
      {/* Desktop table */}
      <div className="hidden md:block overflow-x-auto">
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
              <th className="pb-2 text-right" title={t('spot.maintenanceRateTooltip')}>{t('spot.maintenanceRate')}</th>
              <th className="pb-2 text-right">Gap</th>
              <th className="pb-2 text-right">Borrow</th>
              <th className="pb-2">{t('spot.status')}</th>
              <th className="pb-2"></th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-800/50">
            {pageOpps.map((opp, i) => {
              const oppKey = `${opp.symbol}-${opp.exchange}-${opp.direction}`;
              const borrowKey = `${opp.symbol}-${opp.exchange}`;
              return <SpotRow key={oppKey} opp={opp} index={page * PAGE_SIZE + i} t={t}
                gapResult={gapResults[oppKey]} borrowResult={borrowResults[borrowKey]} {...rowProps} />;
            })}
            {pageOpps.length === 0 && (
              <tr><td colSpan={13} className="py-8 text-center text-gray-500">{t('spot.noOpportunities')}</td></tr>
            )}
          </tbody>
        </table>
      </div>
      {/* Mobile card list */}
      <div className="md:hidden space-y-2">
        {pageOpps.length === 0 && (
          <div className="py-8 text-center text-gray-500">{t('spot.noOpportunities')}</div>
        )}
        {pageOpps.map((opp, i) => {
          const oppKey = `${opp.symbol}-${opp.exchange}-${opp.direction}`;
          const borrowKey = `${opp.symbol}-${opp.exchange}`;
          return <SpotCard key={oppKey} opp={opp} index={page * PAGE_SIZE + i} t={t}
            gapResult={gapResults[oppKey]} borrowResult={borrowResults[borrowKey]} {...rowProps} />;
        })}
      </div>
      <Pagination page={page} total={opportunities.length} setPage={setPage} />
    </div>
  );
};

const Opportunities: FC<OpportunitiesProps> = ({
  opportunities,
  spotOpportunities = [],
  spotScannerMode,
  onOpen,
  onSpotOpen,
  onCheckPriceGap,
  onBatchCheckGap,
  onBatchCheckBorrowable,
  blacklist = [],
  onBlacklistToggle,
}) => {
  const { t } = useLocale();
  const [tab, setTab] = useState<Tab>('perp');
  const [openingOpp, setOpeningOpp] = useState<Opportunity | null>(null);
  const [opening, setOpening] = useState(false);
  const [openError, setOpenError] = useState<string | null>(null);
  const [spotOpening, setSpotOpening] = useState<string | null>(null);
  const [gapResults, setGapResults] = useState<Record<string, PriceGapResult>>({});
  const [borrowResults, setBorrowResults] = useState<Record<string, BorrowResult>>({});
  const [spotError, setSpotError] = useState<string | null>(null);
  const [nativePage, setNativePage] = useState(0);
  const [cgPage, setCgPage] = useState(0);
  const [singlePage, setSinglePage] = useState(0);
  const [gapLoading, setGapLoading] = useState(false);
  const [borrowLoading, setBorrowLoading] = useState(false);
  const [backtestOpp, setBacktestOpp] = useState<SpotOpportunity | null>(null);

  const dismissModal = useCallback(() => { if (!opening) setOpeningOpp(null); }, [opening]);
  useEffect(() => {
    const handler = (e: KeyboardEvent) => { if (e.key === 'Escape') dismissModal(); };
    document.addEventListener('keydown', handler);
    return () => document.removeEventListener('keydown', handler);
  }, [dismissModal]);

  const sorted = [...opportunities].sort((a, b) => b.score - a.score);
  const spotPassed = spotOpportunities.filter((o) => !o.filter_status);
  const nativeSpotOpportunities = spotOpportunities.filter((o) => o.source === 'native');
  const coinGlassSpotOpportunities = spotOpportunities.filter((o) => isCoinGlassSource(o.source));
  const fallbackSpotOpportunities = spotOpportunities.filter((o) => o.source !== 'native' && !isCoinGlassSource(o.source));
  const showSplitSpotView = spotScannerMode === 'both';

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
      <div className="flex items-center justify-between">
        <h2 className="text-xl font-bold text-gray-100">{t('opp.title')}</h2>

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

      {tab === 'perp' && (
        <>
          {/* Desktop table */}
          <div className="hidden md:block bg-gray-900 border border-gray-800 rounded-lg p-4 overflow-x-auto">
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
                        <button
                          onClick={() => { setOpeningOpp(opp); setOpenError(null); }}
                          disabled={opening}
                          className="px-2 py-0.5 text-xs bg-green-600/20 text-green-400 rounded hover:bg-green-600/40 disabled:opacity-50"
                        >
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
          {/* Mobile card list */}
          <div className="md:hidden space-y-2">
            {sorted.length === 0 && (
              <div className="bg-gray-900 border border-gray-800 rounded-lg p-6 text-center text-gray-500 text-sm">
                {t('opp.noOpportunities')}
              </div>
            )}
            {sorted.map((opp, i) => {
              const banned = blacklist.includes(opp.symbol);
              return (
                <div
                  key={`${opp.symbol}-${opp.long_exchange}-${opp.short_exchange}`}
                  className={`rounded-lg border px-3 py-2.5 ${scoreColor(opp.score)} ${banned ? 'border-yellow-500/30' : 'border-gray-800'} bg-gray-900`}
                >
                  <div className="flex items-center justify-between gap-2 mb-2">
                    <div className="flex items-center gap-1.5 min-w-0">
                      <span className="font-mono text-[10px] text-gray-500 shrink-0">{i + 1}</span>
                      <span className="font-mono font-semibold text-gray-100 truncate">{opp.symbol}</span>
                      {banned && (
                        <span className="shrink-0 px-1.5 py-0.5 text-[10px] font-bold rounded bg-yellow-500/15 text-yellow-400">BAN</span>
                      )}
                    </div>
                    <span className="font-mono font-bold text-gray-100 tabular-nums shrink-0">{opp.spread.toFixed(1)} bps/h</span>
                  </div>

                  <div className="space-y-1 text-xs mb-2">
                    <div className="flex items-center justify-between gap-2">
                      <div className="flex items-center gap-2">
                        <span className="text-[10px] font-bold text-green-500 uppercase tracking-wider w-10 shrink-0">Long</span>
                        <span className="text-green-400"><ExchangeLink exchange={opp.long_exchange} symbol={opp.symbol} /></span>
                      </div>
                      <span className="font-mono tabular-nums text-gray-400">{opp.long_rate.toFixed(2)} bps/h</span>
                    </div>
                    <div className="flex items-center justify-between gap-2">
                      <div className="flex items-center gap-2">
                        <span className="text-[10px] font-bold text-red-500 uppercase tracking-wider w-10 shrink-0">Short</span>
                        <span className="text-red-400"><ExchangeLink exchange={opp.short_exchange} symbol={opp.symbol} /></span>
                      </div>
                      <span className="font-mono tabular-nums text-gray-400">{opp.short_rate.toFixed(2)} bps/h</span>
                    </div>
                  </div>

                  <div className="flex items-center justify-between gap-2 text-[11px] text-gray-500 border-t border-gray-800/50 pt-1.5">
                    <span>{t('opp.interval')}: <span className="font-mono text-gray-400">{formatInterval(opp.interval_hours)}</span></span>
                    <span>{t('opp.nextFund')}: <span className="font-mono text-gray-400">{formatFundingCountdown(opp.next_funding)}</span></span>
                    <span>OI: <span className="font-mono text-gray-400">{opp.oi_rank}</span></span>
                  </div>

                  {(onBlacklistToggle || onOpen) && (
                    <div className="flex gap-2 mt-2">
                      {onBlacklistToggle && (
                        <button
                          onClick={() => onBlacklistToggle(opp.symbol)}
                          className={`flex-1 px-3 py-1.5 text-xs font-medium rounded transition-colors ${
                            banned
                              ? 'bg-yellow-600/20 text-yellow-400 hover:bg-yellow-600/40'
                              : 'bg-gray-700/40 text-gray-300 hover:bg-gray-700/60'
                          }`}
                        >
                          {banned ? t('opp.unblock') : t('opp.block')}
                        </button>
                      )}
                      {onOpen && (
                        <button
                          onClick={() => { setOpeningOpp(opp); setOpenError(null); }}
                          disabled={opening}
                          className="flex-1 px-3 py-1.5 text-xs font-semibold bg-green-600/20 text-green-400 rounded hover:bg-green-600/40 disabled:opacity-50 transition-colors"
                        >
                          {t('opp.open')}
                        </button>
                      )}
                    </div>
                  )}
                </div>
              );
            })}
          </div>
        </>
      )}

      {tab === 'spot' && (
        <div className="space-y-4">
          {/* Info legend */}
          <div className="bg-gray-900/50 border border-gray-800 rounded-lg px-4 py-2.5 space-y-1.5">
            <div className="flex items-center gap-4 flex-wrap">
              <span className="text-blue-400 text-xs font-semibold">Dir A</span>
              <span className="text-xs text-gray-500">{t('spot.dirLegend.dirA')}</span>
              <span className="text-gray-700">|</span>
              <span className="text-purple-400 text-xs font-semibold">Dir B</span>
              <span className="text-xs text-gray-500">{t('spot.dirLegend.dirB')}</span>
            </div>
            <div className="flex items-center gap-3 flex-wrap text-[10px] border-t border-gray-800/50 pt-1.5">
              <span className="text-gray-500 font-semibold">{t('spot.gapLegend')}</span>
              <span><span className="text-emerald-400 font-mono">-0.5%</span> <span className="text-gray-600">{t('spot.gapGood')}</span></span>
              <span><span className="text-amber-400 font-mono">+0.3%</span> <span className="text-gray-600">{t('spot.gapWarn')}</span></span>
              <span className="text-gray-700">|</span>
              <span className="text-gray-500 font-semibold">{t('spot.borrowLegend')}</span>
              <span><span className="text-emerald-400 font-mono">12.50</span> <span className="text-gray-600">{t('spot.borrowGood')}</span></span>
              <span><span className="text-red-400 font-mono">0</span> <span className="text-gray-600">{t('spot.borrowBad')}</span></span>
            </div>
          </div>
          {spotError && (
            <div className="flex items-center justify-between bg-red-500/10 border border-red-500/30 rounded-lg px-3 py-2 text-sm text-red-400">
              <span>{spotError}</span>
              <button onClick={() => setSpotError(null)} className="ml-2 text-red-500 hover:text-red-300">&times;</button>
            </div>
          )}

          {showSplitSpotView ? (
            <div className="space-y-3">
              {/* Comparison summary bar */}
              <div className="flex items-center gap-2 bg-gray-900/50 border border-gray-800 rounded-lg px-4 py-2.5">
                <div className="flex items-center gap-2 flex-1">
                  <span className="w-2 h-2 rounded-full bg-emerald-400" />
                  <span className="text-xs font-semibold text-emerald-400 uppercase tracking-wide">{t('spot.sourceNative')}</span>
                  <span className="text-xs text-gray-500">{nativeSpotOpportunities.filter(o => !o.filter_status).length}/{nativeSpotOpportunities.length}</span>
                </div>
                <div className="text-[10px] text-gray-600 font-mono px-3">vs</div>
                <div className="flex items-center gap-2 flex-1 justify-end">
                  <span className="text-xs text-gray-500">{coinGlassSpotOpportunities.filter(o => !o.filter_status).length}/{coinGlassSpotOpportunities.length}</span>
                  <span className="text-xs font-semibold text-amber-400 uppercase tracking-wide">{t('spot.sourceCoinGlass')}</span>
                  <span className="w-2 h-2 rounded-full bg-amber-400" />
                </div>
              </div>
              {/* Side-by-side panels */}
              <div className="grid gap-3 xl:grid-cols-2">
                <SpotSection
                  title={t('spot.sourceNative')}
                  sourceLabel={t('spot.sourceNative')}
                  opportunities={nativeSpotOpportunities}
                  t={t}
                  compact accent="emerald"
                  page={nativePage} setPage={setNativePage}
                  gapResults={gapResults} setGapResults={setGapResults}
                  borrowResults={borrowResults} setBorrowResults={setBorrowResults}
                  gapLoading={gapLoading} setGapLoading={setGapLoading}
                  borrowLoading={borrowLoading} setBorrowLoading={setBorrowLoading}
                  onBatchCheckGap={onBatchCheckGap} onBatchCheckBorrowable={onBatchCheckBorrowable}
                  onSpotOpen={onSpotOpen} spotOpening={spotOpening}
                  setSpotOpening={setSpotOpening} setSpotError={setSpotError}
                  onOpenBacktest={setBacktestOpp}
                />
                <SpotSection
                  title={t('spot.sourceCoinGlass')}
                  sourceLabel={t('spot.sourceCoinGlass')}
                  opportunities={coinGlassSpotOpportunities}
                  t={t}
                  compact accent="amber"
                  page={cgPage} setPage={setCgPage}
                  gapResults={gapResults} setGapResults={setGapResults}
                  borrowResults={borrowResults} setBorrowResults={setBorrowResults}
                  gapLoading={gapLoading} setGapLoading={setGapLoading}
                  borrowLoading={borrowLoading} setBorrowLoading={setBorrowLoading}
                  onBatchCheckGap={onBatchCheckGap} onBatchCheckBorrowable={onBatchCheckBorrowable}
                  onSpotOpen={onSpotOpen} spotOpening={spotOpening}
                  setSpotOpening={setSpotOpening} setSpotError={setSpotError}
                  onOpenBacktest={setBacktestOpp}
                />
              </div>
              {fallbackSpotOpportunities.length > 0 && (
                <SpotSection
                  title={t('spot.sourceFallback')}
                  sourceLabel={t('spot.sourceFallback')}
                  opportunities={fallbackSpotOpportunities}
                  t={t}
                  compact accent="emerald"
                  page={0} setPage={() => {}}
                  gapResults={gapResults} setGapResults={setGapResults}
                  borrowResults={borrowResults} setBorrowResults={setBorrowResults}
                  gapLoading={gapLoading} setGapLoading={setGapLoading}
                  borrowLoading={borrowLoading} setBorrowLoading={setBorrowLoading}
                  onBatchCheckGap={onBatchCheckGap} onBatchCheckBorrowable={onBatchCheckBorrowable}
                  onSpotOpen={onSpotOpen} spotOpening={spotOpening}
                  setSpotOpening={setSpotOpening} setSpotError={setSpotError}
                  onOpenBacktest={setBacktestOpp}
                />
              )}
            </div>
          ) : (
            <SpotSection
              opportunities={spotOpportunities}
              sourceLabel={spotOpportunities.length > 0 ? getSpotSourceLabel(t, spotOpportunities[0].source) : undefined}
              t={t}
              page={singlePage} setPage={setSinglePage}
              gapResults={gapResults} setGapResults={setGapResults}
              borrowResults={borrowResults} setBorrowResults={setBorrowResults}
              gapLoading={gapLoading} setGapLoading={setGapLoading}
              borrowLoading={borrowLoading} setBorrowLoading={setBorrowLoading}
              onBatchCheckGap={onBatchCheckGap} onBatchCheckBorrowable={onBatchCheckBorrowable}
              onSpotOpen={onSpotOpen} spotOpening={spotOpening}
              setSpotOpening={setSpotOpening} setSpotError={setSpotError}
              onOpenBacktest={setBacktestOpp}
            />
          )}
        </div>
      )}

      {backtestOpp && (
        <SpotBacktestModal opp={backtestOpp} t={t} onClose={() => setBacktestOpp(null)} />
      )}

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
                  <button
                    onClick={() => handleConfirmOpen(true)}
                    disabled={opening}
                    className="mt-2 px-3 py-1.5 text-sm bg-yellow-600/20 text-yellow-400 rounded hover:bg-yellow-600/40 disabled:opacity-50"
                  >
                    {opening ? '...' : t('opp.forceOpen')}
                  </button>
                )}
              </div>
            )}
            <div className="flex gap-3 justify-end">
              <button
                onClick={() => setOpeningOpp(null)}
                disabled={opening}
                className="px-4 py-2 text-sm text-gray-400 hover:text-gray-200 disabled:opacity-50"
              >
                {t('opp.openCancel')}
              </button>
              <button
                onClick={() => handleConfirmOpen(false)}
                disabled={opening}
                className="px-4 py-2 text-sm bg-green-600 text-white rounded hover:bg-green-700 disabled:opacity-50"
              >
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
