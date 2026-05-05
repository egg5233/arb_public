// Phase 11 Plan 06 — DiscoveryBanner.
//
// Conditional banner above the Discovery cards.  Three variants:
//   - 'disabled' : gray banner, role="status", explains config flag.
//   - 'errored'  : red banner, role="alert", with {n} + {exchanges} tokens.
//   - null       : returns null (healthy state has no banner per UI-SPEC).
//
// Reuses replaceTokens-style {n}/{exchanges} interpolation matching the
// existing helper in PriceGap.tsx; kept inline to avoid a circular import.
import type { FC } from 'react';
import { useLocale } from '../../i18n/index.ts';

export interface DiscoveryBannerProps {
  variant: 'disabled' | 'errored' | null;
  errorContext?: { n: number; exchanges: string[] };
  onEnable?: () => void;
  enableBusy?: boolean;
  enableError?: string | null;
}

function replaceTokens(
  s: string,
  tokens: Record<string, string | number>,
): string {
  let out = s;
  for (const [k, v] of Object.entries(tokens)) {
    out = out.replace(new RegExp(`\\{${k}\\}`, 'g'), String(v));
  }
  return out;
}

export const DiscoveryBanner: FC<DiscoveryBannerProps> = ({
  variant,
  errorContext,
  onEnable,
  enableBusy,
  enableError,
}) => {
  const { t } = useLocale();

  if (variant === null) return null;

  if (variant === 'disabled') {
    return (
      <div
        role="status"
        data-testid="discovery-banner-disabled"
        className="bg-gray-800 text-gray-400 border border-gray-700 rounded-lg p-4"
      >
        <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <div className="text-sm font-bold mb-1">
              {t('pricegap.discovery.banner.disabled.heading')}
            </div>
            <div className="text-xs">
              {t('pricegap.discovery.banner.disabled.body')}
            </div>
            {enableError && (
              <div className="text-xs text-[#f6465d] mt-2">{enableError}</div>
            )}
          </div>
          {onEnable && (
            <button
              type="button"
              className="btn-primary px-3 py-1.5 rounded text-sm whitespace-nowrap disabled:opacity-50"
              onClick={onEnable}
              disabled={enableBusy}
              data-testid="discovery-enable-button"
            >
              {enableBusy
                ? t('pricegap.discovery.banner.disabled.enabling')
                : t('pricegap.discovery.banner.disabled.enableButton')}
            </button>
          )}
        </div>
      </div>
    );
  }

  // variant === 'errored'
  const ctx = errorContext ?? { n: 0, exchanges: [] };
  const exchangesLabel = ctx.exchanges.length > 0 ? ctx.exchanges.join(', ') : '-';
  const body = replaceTokens(t('pricegap.discovery.banner.error.body'), {
    n: ctx.n,
    exchanges: exchangesLabel,
  });
  return (
    <div
      role="alert"
      data-testid="discovery-banner-errored"
      className="bg-[#f6465d]/15 text-[#f6465d] border border-[#f6465d]/40 rounded-lg p-4"
    >
      <div className="text-sm font-bold mb-1">
        {t('pricegap.discovery.banner.error.heading')}
      </div>
      <div className="text-xs">{body}</div>
    </div>
  );
};

export default DiscoveryBanner;
