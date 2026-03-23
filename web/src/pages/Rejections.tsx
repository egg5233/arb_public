import { useState, useEffect, type FC } from 'react';
import type { RejectedOpportunity } from '../types.ts';
import { useLocale } from '../i18n/index.ts';

interface RejectionsProps {
  rejections: RejectedOpportunity[];
  getRejections: () => Promise<RejectedOpportunity[]>;
  setRejections: (r: RejectedOpportunity[]) => void;
}

function stageBadge(stage: string): { bg: string; text: string } {
  switch (stage) {
    case 'scanner': return { bg: 'bg-blue-500/20', text: 'text-blue-400' };
    case 'verifier': return { bg: 'bg-purple-500/20', text: 'text-purple-400' };
    case 'risk': return { bg: 'bg-yellow-500/20', text: 'text-yellow-400' };
    case 'engine': return { bg: 'bg-red-500/20', text: 'text-red-400' };
    default: return { bg: 'bg-gray-500/20', text: 'text-gray-400' };
  }
}

function formatTime(ts: string): string {
  const d = new Date(ts);
  if (isNaN(d.getTime())) return '-';
  return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });
}

const Rejections: FC<RejectionsProps> = ({ rejections, getRejections, setRejections }) => {
  const { t } = useLocale();
  const [stageFilter, setStageFilter] = useState<string>('all');

  useEffect(() => {
    getRejections().then(setRejections).catch(() => {});
  }, [getRejections, setRejections]);

  const filtered = stageFilter === 'all'
    ? rejections
    : rejections.filter((r) => r.stage === stageFilter);

  // Show newest first
  const sorted = [...filtered].reverse();

  const stages = ['all', 'scanner', 'verifier', 'risk', 'engine'] as const;

  return (
    <div className="space-y-6">
      <h2 className="text-xl font-bold text-gray-100">
        {t('rej.title')}
        <span className="ml-2 text-sm font-normal text-gray-500">{rejections.length} {t('rej.total')}</span>
      </h2>

      <div className="flex gap-2">
        {stages.map((s) => (
          <button key={s} onClick={() => setStageFilter(s)}
            className={`px-3 py-1 text-xs rounded transition-colors ${
              stageFilter === s
                ? 'bg-blue-500/20 text-blue-400'
                : 'text-gray-500 hover:text-gray-300 hover:bg-gray-800'
            }`}
          >
            {t(`rej.${s}` as any)}
          </button>
        ))}
      </div>

      <div className="bg-gray-900 border border-gray-800 rounded-lg p-4 overflow-x-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="text-gray-400 text-left border-b border-gray-800">
              <th className="pb-2">{t('rej.time')}</th>
              <th className="pb-2">{t('rej.symbol')}</th>
              <th className="pb-2">{t('rej.long')}</th>
              <th className="pb-2">{t('rej.short')}</th>
              <th className="pb-2 text-right">{t('rej.spread')}</th>
              <th className="pb-2">{t('rej.stage')}</th>
              <th className="pb-2">{t('rej.reason')}</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-800">
            {sorted.map((r, i) => {
              const badge = stageBadge(r.stage);
              return (
                <tr key={i} className="text-gray-100">
                  <td className="py-2 font-mono text-gray-500 text-xs">{formatTime(r.timestamp)}</td>
                  <td className="py-2 font-mono">{r.symbol}</td>
                  <td className="py-2 text-green-400 text-xs">{r.long_exchange}</td>
                  <td className="py-2 text-red-400 text-xs">{r.short_exchange}</td>
                  <td className="py-2 text-right font-mono">{r.spread.toFixed(1)}</td>
                  <td className="py-2">
                    <span className={`px-2 py-0.5 rounded text-xs ${badge.bg} ${badge.text}`}>{r.stage}</span>
                  </td>
                  <td className="py-2 text-gray-400 text-xs max-w-xs truncate">{r.reason}</td>
                </tr>
              );
            })}
            {sorted.length === 0 && (
              <tr>
                <td colSpan={7} className="py-4 text-center text-gray-500">{t('rej.none')}</td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
};

export default Rejections;
