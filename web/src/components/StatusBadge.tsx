import type { FC } from 'react';

interface StatusBadgeProps {
  status: string;
}

// Binance semantic colors — Crypto Green for live, Crypto Red for critical,
// Binance Yellow for attention, Slate for dormant. Rendered as a thin pill
// with uppercase micro-type so it reads like a trading-terminal tag.
const colorMap: Record<string, string> = {
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

const StatusBadge: FC<StatusBadgeProps> = ({ status }) => {
  const colors =
    colorMap[status.toLowerCase()] ||
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
