import type { FC } from 'react';
import { useLocale } from '../i18n/index.ts';
import type { Position } from '../types.ts';

interface PnLBreakdownProps {
  position: Position;
}

function fmt(v: number | undefined | null, forceSign = false): string {
  if (v == null || v === 0) return '-';
  const sign = v > 0 && forceSign ? '+' : '';
  return `${sign}$${v.toFixed(4)}`;
}

function cc(v: number | undefined | null): string {
  if (v == null || v === 0) return 'text-gray-500';
  return v > 0 ? 'text-green-400' : 'text-red-400';
}

const PnLBreakdown: FC<PnLBreakdownProps> = ({ position: p }) => {
  const { t } = useLocale();

  const hasPerLeg =
    (p.long_total_fees != null && p.long_total_fees !== 0) ||
    (p.short_total_fees != null && p.short_total_fees !== 0) ||
    (p.long_funding != null && p.long_funding !== 0) ||
    (p.short_funding != null && p.short_funding !== 0) ||
    (p.long_close_pnl != null && p.long_close_pnl !== 0) ||
    (p.short_close_pnl != null && p.short_close_pnl !== 0);

  // Detect pre-v0.27.0 positions (no decomposition data at all)
  const hasDecomposition =
    hasPerLeg ||
    (p.exit_fees != null && p.exit_fees !== 0) ||
    (p.basis_gain_loss != null && p.basis_gain_loss !== 0) ||
    (p.slippage != null && p.slippage !== 0) ||
    (p.rotation_pnl != null && p.rotation_pnl !== 0);

  if (!hasDecomposition) {
    return (
      <div className="text-xs text-gray-500 py-2">
        {t('hist.dataUnavailable')}
      </div>
    );
  }

  // APR calculation
  const holdMs = new Date(p.updated_at).getTime() - new Date(p.created_at).getTime();
  const holdHours = holdMs > 0 ? holdMs / 3600000 : 1;
  const notional = p.entry_notional && p.entry_notional > 0
    ? p.entry_notional
    : Math.max(p.long_entry * p.long_size, p.short_entry * p.short_size, 1);
  const apr = notional > 0 && holdHours > 0
    ? (p.realized_pnl / notional) * (8760 / holdHours) * 100
    : 0;

  // Per-leg two-column layout
  if (hasPerLeg) {
    const lFees = -Math.abs(p.long_total_fees ?? 0);
    const sFees = -Math.abs(p.short_total_fees ?? 0);
    const lFund = p.long_funding ?? 0;
    const sFund = p.short_funding ?? 0;
    const lClose = p.long_close_pnl ?? 0;
    const sClose = p.short_close_pnl ?? 0;
    const lSub = lFees + lFund + lClose;
    const sSub = sFees + sFund + sClose;
    const hasRotation = (p.rotation_pnl ?? 0) !== 0;

    const rows: { label: string; long: number; short: number; bold?: boolean }[] = [
      { label: t('hist.tradingFee'), long: lFees, short: sFees },
      { label: t('hist.funding'), long: lFund, short: sFund },
      { label: t('hist.closePnl'), long: lClose, short: sClose },
      { label: t('hist.subtotal'), long: lSub, short: sSub, bold: true },
    ];

    return (
      <div className="text-sm space-y-1">
        <table className="w-full text-right">
          <thead>
            <tr className="text-gray-500 text-xs">
              <th className="text-left w-1/3"></th>
              <th className="w-1/3">{t('hist.longSide')}</th>
              <th className="w-1/3">{t('hist.shortSide')}</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((r) => (
              <tr key={r.label} className={r.bold ? 'border-t border-gray-700' : ''}>
                <td className="text-left text-gray-500 text-xs">{r.label}</td>
                <td className={`font-mono ${r.bold ? 'font-bold' : ''} ${cc(r.long)}`}>{fmt(r.long, true)}</td>
                <td className={`font-mono ${r.bold ? 'font-bold' : ''} ${cc(r.short)}`}>{fmt(r.short, true)}</td>
              </tr>
            ))}
            {hasRotation && (
              <tr>
                <td className="text-left text-gray-500 text-xs">{t('hist.rotationPnl')}</td>
                <td colSpan={2} className={`font-mono text-center ${cc(p.rotation_pnl)}`}>{fmt(p.rotation_pnl, true)}</td>
              </tr>
            )}
            <tr className="border-t border-gray-600">
              <td className="text-left text-gray-500 text-xs font-bold">{t('hist.totalPnl')}</td>
              <td colSpan={2} className={`font-mono text-center font-bold ${cc(p.realized_pnl)}`}>{fmt(p.realized_pnl, true)}</td>
            </tr>
            <tr>
              <td className="text-left text-gray-500 text-xs">{t('hist.positionApr')}</td>
              <td colSpan={2} className={`font-mono text-center ${apr >= 0 ? 'text-green-400' : 'text-red-400'}`}>{apr.toFixed(1)}%</td>
            </tr>
          </tbody>
        </table>
      </div>
    );
  }

  // Fallback: old single-column layout for pre-per-leg positions
  const cells = [
    { label: t('hist.entryFees'), value: p.entry_fees, negative: true },
    { label: t('hist.exitFees'), value: -Math.abs(p.exit_fees ?? 0), negative: true },
    { label: t('hist.funding'), value: p.funding_collected, forceSign: true },
    { label: t('hist.basisGainLoss'), value: p.basis_gain_loss, forceSign: true },
    { label: t('hist.slippage'), value: p.slippage, negative: true },
    { label: t('hist.netPnl'), value: p.realized_pnl, forceSign: true },
    { label: t('hist.positionApr'), value: undefined as number | undefined, aprValue: apr },
  ];

  return (
    <div className="grid grid-cols-2 md:grid-cols-4 gap-4 text-sm">
      {cells.map((cell) => (
        <div key={cell.label}>
          <div className="text-gray-500 text-xs">{cell.label}</div>
          {cell.aprValue !== undefined ? (
            <div className={`font-mono ${cell.aprValue >= 0 ? 'text-green-400' : 'text-red-400'}`}>
              {cell.aprValue.toFixed(1)}%
            </div>
          ) : (
            <div className={`font-mono ${cell.negative ? 'text-red-400' : cc(cell.value)}`}>
              {cell.forceSign ? fmt(cell.value, true) : fmt(cell.value)}
            </div>
          )}
        </div>
      ))}
    </div>
  );
};

export default PnLBreakdown;
