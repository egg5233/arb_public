// Phase 11 Plan 06 — DiscoverySection (container, PG-DISC-03).
//
// Owns layout for the read-only Discovery sub-section inside the Price-Gap
// dashboard tab.  Inserted into PriceGap.tsx between the Master-toggle row
// and the Candidates table (UI-SPEC §Placement).  Phase 16 (PG-OPS-09) will
// migrate this same component to a top-level tab without restructuring.
//
// Renders:
//   - Section header (title + status pill + last-run timestamp).
//   - Conditional <DiscoveryBanner> (disabled / errored variants).
//   - <CycleStatsCard> full-width.
//   - 2-col row (lg+) with <ScoreHistoryCard> (60%) + <WhyRejectedCard> (40%).
//   - <PromoteTimeline> full-width below (Phase 12 Plan 04 swap from placeholder).
//
// State machine (UI-SPEC §"Three scanner visual states"):
//   enabled=true,  errors==0, !cycle_failed → Scanner ON   (green pill)
//   enabled=false                            → Scanner OFF  (gray pill + banner)
//   enabled=true,  errors>0 || cycle_failed  → Scanner ERR  (red pill + banner)
import { useState, type FC } from 'react';
import { useLocale } from '../../i18n/index.ts';
import { usePgDiscovery } from '../../hooks/usePgDiscovery.ts';
import { DiscoveryBanner } from './DiscoveryBanner.tsx';
import { CycleStatsCard } from './CycleStatsCard.tsx';
import { ScoreHistoryCard } from './ScoreHistoryCard.tsx';
import { WhyRejectedCard } from './WhyRejectedCard.tsx';
import { PromoteTimeline } from './PromoteTimeline.tsx';

function formatLastRunPill(ts: number | null): string {
  if (ts == null || !Number.isFinite(ts) || ts <= 0) return '';
  const diff = Math.floor(Date.now() / 1000) - ts;
  if (diff < 0) return '';
  if (diff < 60) return `${diff}s ago`;
  if (diff < 3600) return `${Math.floor(diff / 60)}m ago`;
  if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`;
  return `${Math.floor(diff / 86400)}d ago`;
}

function authHeaders(): Record<string, string> {
  const token = localStorage.getItem('arb_token') || '';
  return {
    'Content-Type': 'application/json',
    Authorization: `Bearer ${token}`,
  };
}

export const DiscoverySection: FC = () => {
  const { t } = useLocale();
  const {
    state,
    scores,
    loadScoresFor,
    loadingSymbol,
    errored,
    enabled,
    lastRunAt,
    refresh,
  } = usePgDiscovery();
  const [enableBusy, setEnableBusy] = useState(false);
  const [enableError, setEnableError] = useState<string | null>(null);
  const [optimisticEnabled, setOptimisticEnabled] = useState(false);

  const scannerEnabled = enabled || optimisticEnabled;

  const enableDiscovery = async () => {
    setEnableBusy(true);
    setEnableError(null);
    try {
      const res = await fetch('/api/config', {
        method: 'POST',
        headers: authHeaders(),
        body: JSON.stringify({ price_gap: { discovery_enabled: true } }),
      });
      const body = (await res.json().catch(() => null)) as {
        ok?: boolean;
        error?: string;
      } | null;
      if (!res.ok || !body?.ok) {
        throw new Error(body?.error || `HTTP ${res.status}`);
      }
      setOptimisticEnabled(true);
      await refresh();
    } catch (err) {
      setEnableError(err instanceof Error ? err.message : String(err));
    } finally {
      setEnableBusy(false);
    }
  };

  // Variant resolution.
  let pillLabel = t('pricegap.discovery.scannerOff');
  let pillColor = 'bg-gray-700/50 text-gray-300 border-gray-600';
  let bannerVariant: 'disabled' | 'errored' | null = 'disabled';
  if (scannerEnabled && errored) {
    pillLabel = t('pricegap.discovery.scannerErrored');
    pillColor = 'bg-[#f6465d]/15 text-[#f6465d] border-[#f6465d]/40';
    bannerVariant = 'errored';
  } else if (scannerEnabled) {
    pillLabel = t('pricegap.discovery.scannerOn');
    pillColor = 'bg-[#0ecb81]/15 text-[#0ecb81] border-[#0ecb81]/40';
    bannerVariant = null;
  }

  const errorContext = {
    n: state?.errors ?? 0,
    // Plan 04/05 do not (yet) emit a per-exchange failure list — fall back to
    // the count + a generic placeholder.  When Plan 12 telemetry adds a list,
    // wire it here without changing the i18n contract.
    exchanges: ['exchanges'],
  };

  const lastRunPill = formatLastRunPill(lastRunAt);

  return (
    <section
      className="space-y-6"
      data-testid="discovery-section"
      aria-labelledby="discovery-section-title"
    >
      {/* Header */}
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h3
            id="discovery-section-title"
            className="text-sm font-bold text-gray-300 uppercase tracking-wide"
          >
            {t('pricegap.discovery.title')}
          </h3>
          <p className="text-xs text-gray-500 mt-1">
            {t('pricegap.discovery.subtitle')}
          </p>
        </div>
        <div className="flex items-center gap-3">
          {lastRunPill && (
            <span
              className="text-xs text-gray-500 font-mono tabular-nums"
              data-testid="discovery-last-run"
            >
              {t('pricegap.discovery.cycle.lastRun')}: {lastRunPill}
            </span>
          )}
          <span
            className={`text-xs font-bold border rounded-full px-2 py-0.5 ${pillColor}`}
            data-testid="discovery-status-pill"
          >
            {pillLabel}
          </span>
        </div>
      </div>

      {/* Conditional banner */}
      <DiscoveryBanner
        variant={bannerVariant}
        errorContext={errorContext}
        onEnable={enableDiscovery}
        enableBusy={enableBusy}
        enableError={enableError}
      />

      {/* Cycle stats — full width */}
      <CycleStatsCard state={state} />

      {/* 2-column row: score history (60%) + why rejected (40%) */}
      <div className="grid grid-cols-1 lg:grid-cols-5 gap-6">
        <div className="lg:col-span-3">
          <ScoreHistoryCard
            state={state}
            scores={scores}
            loadingSymbol={loadingSymbol}
            onSelectSymbol={loadScoresFor}
          />
        </div>
        <div className="lg:col-span-2">
          <WhyRejectedCard whyRejected={state?.why_rejected} />
        </div>
      </div>

      {/* Phase 12 Plan 04 — populated PromoteTimeline (replaces placeholder) */}
      <PromoteTimeline />
    </section>
  );
};

export default DiscoverySection;
