import type { FC } from 'react';

interface StatusBadgeProps {
  status: string;
}

const colorMap: Record<string, string> = {
  pending: 'bg-purple-500/20 text-purple-400',
  partial: 'bg-blue-500/20 text-blue-400',
  opening: 'bg-blue-500/20 text-blue-400',
  active: 'bg-green-500/20 text-green-400',
  closing: 'bg-yellow-500/20 text-yellow-400',
  closed: 'bg-gray-500/20 text-gray-400',
  error: 'bg-red-500/20 text-red-400',
  liquidated: 'bg-red-500/20 text-red-400',
};

const StatusBadge: FC<StatusBadgeProps> = ({ status }) => {
  const colors = colorMap[status.toLowerCase()] || 'bg-gray-500/20 text-gray-400';
  return (
    <span className={`inline-block px-2 py-0.5 rounded text-xs font-medium ${colors}`}>
      {status}
    </span>
  );
};

export default StatusBadge;
