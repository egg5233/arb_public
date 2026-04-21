import type { FC } from 'react';
import { useTheme } from '../theme/index.ts';

interface StatusBadgeProps {
  status: string;
}

// Binance semantic colors (new design)
const colorMapNew: Record<string, string> = {
  pending: 'bg-[#f0b90b]/15 text-[#f0b90b] border border-[#f0b90b]/30',
  partial: 'bg-[#f0b90b]/15 text-[#f0b90b] border border-[#f0b90b]/30',
  opening: 'bg-[#0ecb81]/15 text-[#0ecb81] border border-[#0ecb81]/30',
  active: 'bg-[#0ecb81]/15 text-[#0ecb81] border border-[#0ecb81]/30',
  exiting: 'bg-[#f6465d]/15 text-[#f6465d] border border-[#f6465d]/30',
  closing: 'bg-[#f6465d]/15 text-[#f6465d] border border-[#f6465d]/30',
  closed: 'bg-[#2b2f36] text-[#848e9c] border border-[#3a3e45]',
  error: 'bg-[#f6465d]/20 text-[#f6465d] border border-[#f6465d]/40',
  liquidated: 'bg-[#f6465d]/20 text-[#f6465d] border border-[#f6465d]/40',
};

// Classic Tailwind semantic colors
const colorMapClassic: Record<string, string> = {
  pending: 'bg-purple-500/20 text-purple-400',
  partial: 'bg-blue-500/20 text-blue-400',
  opening: 'bg-blue-500/20 text-blue-400',
  active: 'bg-green-500/20 text-green-400',
  closing: 'bg-yellow-500/20 text-yellow-400',
  exiting: 'bg-yellow-500/20 text-yellow-400',
  closed: 'bg-gray-500/20 text-gray-400',
  error: 'bg-red-500/20 text-red-400',
  liquidated: 'bg-red-500/20 text-red-400',
};

const StatusBadge: FC<StatusBadgeProps> = ({ status }) => {
  const { theme } = useTheme();

  if (theme === 'classic') {
    const colors = colorMapClassic[status.toLowerCase()] || 'bg-gray-500/20 text-gray-400';
    return (
      <span className={`inline-block px-2 py-0.5 rounded text-xs font-medium ${colors}`}>
        {status}
      </span>
    );
  }

  // new design
  const colors =
    colorMapNew[status.toLowerCase()] ||
    'bg-[#2b2f36] text-[#848e9c] border border-[#3a3e45]';
  return (
    <span
      className={`inline-flex items-center px-2 py-[2px] rounded-full text-[10px] font-semibold uppercase tracking-wider ${colors}`}
    >
      {status}
    </span>
  );
};

export default StatusBadge;
