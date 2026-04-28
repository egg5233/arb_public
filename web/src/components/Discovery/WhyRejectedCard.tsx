// Phase 11 Plan 06 — WhyRejectedCard.
//
// Renders the reason → count map from the latest cycle as a sorted table.
// Reason labels resolve through pricegap.discovery.whyRejected.reason.{reason}
// i18n keys; unknown reasons fall back to the raw snake_case code so new
// rejection reasons emitted by future scanner versions still display
// (defensive UX per UI-SPEC §"Empty/error/destructive").
//
// Empty map renders the i18n empty key (no rejections in last cycle).
import type { FC } from 'react';
import { useLocale } from '../../i18n/index.ts';
import type { TranslationKey } from '../../i18n/en.ts';

export interface WhyRejectedCardProps {
  whyRejected: Record<string, number> | undefined;
}

// Allowlist of canonical reason codes shipped with this plan; UI-SPEC defines
// these and they must have i18n entries.  Unknown codes still render via
// fallback to the raw string.
const KNOWN_REASONS = new Set([
  'insufficient_persistence',
  'stale_bbo',
  'insufficient_depth',
  'denylist',
  'bybit_blackout',
  'symbol_not_listed_long',
  'symbol_not_listed_short',
  'sample_error',
]);

export const WhyRejectedCard: FC<WhyRejectedCardProps> = ({ whyRejected }) => {
  const { t } = useLocale();
  const panelCls = 'bg-gray-900 border border-gray-800 rounded-lg p-4';
  const entries = Object.entries(whyRejected ?? {})
    .filter(([, n]) => n > 0)
    .sort((a, b) => b[1] - a[1]);

  if (entries.length === 0) {
    return (
      <div className={panelCls} data-testid="why-rejected-card">
        <h3 className="text-base font-bold text-gray-100 mb-3">
          {t('pricegap.discovery.whyRejected.title')}
        </h3>
        <p className="text-xs text-gray-500" data-testid="why-rejected-empty">
          {t('pricegap.discovery.whyRejected.empty')}
        </p>
      </div>
    );
  }

  return (
    <div className={panelCls} data-testid="why-rejected-card">
      <h3 className="text-base font-bold text-gray-100 mb-3">
        {t('pricegap.discovery.whyRejected.title')}
      </h3>
      <table className="w-full text-sm">
        <thead>
          <tr className="text-xs text-gray-400 border-b border-gray-800">
            <th className="text-left py-2 pr-4 font-normal">
              {t('pricegap.discovery.whyRejected.colReason')}
            </th>
            <th className="text-right py-2 font-normal">
              {t('pricegap.discovery.whyRejected.colCount')}
            </th>
          </tr>
        </thead>
        <tbody>
          {entries.map(([reason, count]) => {
            const labelKey = `pricegap.discovery.whyRejected.reason.${reason}`;
            const label = KNOWN_REASONS.has(reason)
              ? t(labelKey as TranslationKey)
              : reason;
            return (
              <tr
                key={reason}
                className="border-b border-gray-800/50"
                data-testid={`why-rejected-row-${reason}`}
              >
                <td className="py-2 pr-4 text-gray-300">{label}</td>
                <td className="py-2 text-right text-gray-100 font-mono tabular-nums">
                  {count}
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
};

export default WhyRejectedCard;
