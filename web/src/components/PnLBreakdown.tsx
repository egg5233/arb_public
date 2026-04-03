import type { FC } from 'react';
import { useLocale } from '../i18n/index.ts';
import type { Position } from '../types.ts';

interface PnLBreakdownProps {
  position: Position;
}

function formatDollar(v: number | undefined | null, forceSign = false): string {
  if (v == null || v === 0) return '-';
  const sign = v > 0 && forceSign ? '+' : '';
  return `${sign}$${v.toFixed(4)}`;
}

function colorClass(v: number | undefined | null): string {
  if (v == null || v === 0) return 'text-gray-500';
  return v > 0 ? 'text-green-400' : 'text-red-400';
}

const PnLBreakdown: FC<PnLBreakdownProps> = ({ position }) => {
  const { t } = useLocale();

  // Detect if this is a pre-v0.27.0 position (no decomposition data)
  const hasDecomposition =
    (position.exit_fees != null && position.exit_fees !== 0) ||
    (position.basis_gain_loss != null && position.basis_gain_loss !== 0) ||
    (position.slippage != null && position.slippage !== 0);

  if (!hasDecomposition) {
    return (
      <div className="text-xs text-gray-500 py-2">
        {t('hist.dataUnavailable')}
      </div>
    );
  }

  // Detect spot-futures positions (they have a direction field on SpotPosition,
  // but in History these are perp-perp ArbitragePosition, so we check for borrow cost fields)
  // For perp-perp positions, borrow cost is not applicable
  const isSpotFutures = false; // perp-perp history positions don't have borrow cost

  // Compute APR client-side
  const holdMs = new Date(position.updated_at).getTime() - new Date(position.created_at).getTime();
  const holdHours = holdMs > 0 ? holdMs / 3600000 : 1;
  const notional = Math.max(
    position.long_entry * position.long_size,
    position.short_entry * position.short_size,
    1
  );
  const apr = notional > 0 && holdHours > 0
    ? (position.realized_pnl / notional) * (8760 / holdHours) * 100
    : 0;

  const cells = [
    { label: t('hist.entryFees'), value: position.entry_fees, negative: true },
    { label: t('hist.exitFees'), value: position.exit_fees, negative: true },
    { label: t('hist.funding'), value: position.funding_collected, forceSign: true },
    { label: t('hist.basisGainLoss'), value: position.basis_gain_loss, forceSign: true },
    ...(isSpotFutures ? [{ label: t('hist.borrowCost'), value: 0 as number | undefined, negative: true }] : []),
    { label: t('hist.slippage'), value: position.slippage, negative: true },
    { label: t('hist.netPnl'), value: position.realized_pnl, forceSign: true },
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
            <div className={`font-mono ${cell.negative ? 'text-red-400' : colorClass(cell.value)}`}>
              {cell.forceSign ? formatDollar(cell.value, true) : formatDollar(cell.value)}
            </div>
          )}
        </div>
      ))}
    </div>
  );
};

export default PnLBreakdown;
