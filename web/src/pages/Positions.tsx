import { useState, type FC } from 'react';
import type { Position } from '../types.ts';
import { useLocale } from '../i18n/index.ts';
import { tradingUrl } from '../utils/tradingUrl.tsx';


interface PositionsProps {
  positions: Position[];
  onClose?: (positionId: string) => Promise<void>;
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

const Positions: FC<PositionsProps> = ({ positions, onClose }) => {
  const { t } = useLocale();
  const [closingId, setClosingId] = useState<string | null>(null);
  const [closing, setClosing] = useState(false);
  const [closeError, setCloseError] = useState<string | null>(null);

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
              <th className="pb-2 text-right">{t('pos.rotPnl')}</th>
              <th className="pb-2 text-right">{t('pos.nextFund')}</th>
              <th className="pb-2 text-right">{t('pos.age')}</th>
              <th className="pb-2 text-center">{t('pos.sl')}</th>
              <th className="pb-2"></th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-800">
            {positions.map((p) => (
              <tr key={p.id} className="text-gray-100">
                <td className="py-2 font-mono">
                  {p.symbol}
                  {(p.rotation_count ?? 0) > 0 && (
                    <span className="ml-1.5 text-xs text-yellow-500" title={`Rotated ${p.rotation_count}x, last from ${p.last_rotated_from}${p.all_exchanges ? '. All: ' + p.all_exchanges.join(', ') : ''}`}>
                      R{p.rotation_count}
                    </span>
                  )}
                </td>
                <td className="py-2 text-sm">
                  <a href={tradingUrl(p.long_exchange, p.symbol)} target="_blank" rel="noopener noreferrer"
                    className="text-green-400 hover:underline cursor-pointer">{p.long_exchange}</a>{' '}
                  <span className="font-mono text-xs">{p.long_size.toFixed(4)}@{formatPrice(p.long_entry)}</span>
                </td>
                <td className="py-2 text-sm">
                  <a href={tradingUrl(p.short_exchange, p.symbol)} target="_blank" rel="noopener noreferrer"
                    className="text-red-400 hover:underline cursor-pointer">{p.short_exchange}</a>{' '}
                  <span className="font-mono text-xs">{p.short_size.toFixed(4)}@{formatPrice(p.short_entry)}</span>
                </td>
                <td className="py-2 text-right font-mono">{p.entry_spread.toFixed(1)} bps/h</td>
                <td className={`py-2 text-right font-mono ${(p.current_spread ?? 0) > 0 ? 'text-green-400' : (p.current_spread ?? 0) < 0 ? 'text-red-400' : 'text-gray-400'}`}>
                  {p.current_spread != null ? `${p.current_spread.toFixed(1)} bps/h` : '-'}
                </td>
                <td className={`py-2 text-right font-mono ${p.funding_collected > 0 ? 'text-green-400' : p.funding_collected < 0 ? 'text-red-400' : 'text-gray-400'}`}>${p.funding_collected.toFixed(2)}</td>
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
                      onClick={() => setClosingId(p.id)}
                      disabled={closing}
                      className="px-2 py-0.5 text-xs bg-red-600/20 text-red-400 rounded hover:bg-red-600/40 disabled:opacity-50"
                    >
                      {t('pos.close')}
                    </button>
                  )}
                </td>
              </tr>
            ))}
            {positions.length === 0 && (
              <tr>
                <td colSpan={11} className="py-4 text-center text-gray-500">{t('pos.noPositions')}</td>
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
