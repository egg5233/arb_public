// Phase 12 Plan 04 — PromoteTimeline (PG-DISC-02).
//
// Replaces Phase 11 PromoteTimelinePlaceholder with a populated card.
// Subscribes to pg_promote_event WS and seeds from
// /api/pg/discovery/promote-events. Renders newest-first; cap 1000 in memory,
// 50 visible default with "Show all (N)".
//
// Design tokens inherited verbatim from Phase 11 sibling cards
// (CycleStatsCard / WhyRejectedCard / DiscoveryBanner) per 12-UI-SPEC.
import { useState, type FC } from 'react';
import { useLocale } from '../../i18n/index.ts';
import { usePgDiscovery, type PromoteEvent } from '../../hooks/usePgDiscovery.ts';

const MAX_VISIBLE_DEFAULT = 50;
const HARD_CAP = 1000;

// Asia/Taipei (UTC+8) per CLAUDE.local.md project memory + CycleStatsCard
// precedent. Render as MM-DD HH:mm:ss for compact tabular display.
function formatTimestamp(tsMs: number): string {
  const d = new Date(tsMs);
  const tz = new Intl.DateTimeFormat('en-CA', {
    timeZone: 'Asia/Taipei',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false,
  });
  const parts = tz.formatToParts(d).reduce<Record<string, string>>((acc, p) => {
    acc[p.type] = p.value;
    return acc;
  }, {});
  return `${parts.month}-${parts.day} ${parts.hour}:${parts.minute}:${parts.second}`;
}

interface RowProps {
  ev: PromoteEvent;
  t: (key: import('../../i18n/index.ts').TranslationKey) => string;
}

const TimelineRow: FC<RowProps> = ({ ev, t }) => {
  const isPromote = ev.action === 'promote';
  // UI-SPEC §Color: accent reserved for chip + glyph + 2px left rule.
  const accentColor = isPromote ? '#0ecb81' : '#f6465d';
  const glyph = isPromote ? '▲' : '▼';
  const chipText = isPromote
    ? t('pricegap.discovery.timeline.actionPromote')
    : t('pricegap.discovery.timeline.actionDemote');
  const srText = isPromote
    ? t('pricegap.discovery.timeline.sr.promote')
    : t('pricegap.discovery.timeline.sr.demote');
  const showDirection = ev.direction !== 'bidirectional';

  return (
    <div
      role="listitem"
      className="flex items-center gap-2 py-2 pr-2 pl-3 border-b border-gray-800/50 hover:bg-gray-800/50"
      style={{ borderLeft: `2px solid ${accentColor}99` }}
    >
      {/* Color-not-only signal (a11y mandatory): sr-only announce with action verb. */}
      <span className="sr-only">
        {srText} {ev.symbol} {ev.long_exch} {ev.short_exch}{' '}
        {t('pricegap.discovery.timeline.scoreLabel')} {ev.score}
      </span>
      <span className="text-xs" style={{ color: accentColor }} aria-hidden="true">
        {glyph}
      </span>
      <span
        className="text-xs font-semibold uppercase tracking-wide rounded-full px-2 py-0.5 border"
        style={{
          backgroundColor: `${accentColor}26`, // 15% opacity
          color: accentColor,
          borderColor: `${accentColor}66`, // 40% opacity
        }}
      >
        {chipText}
      </span>
      <span className="text-sm font-semibold text-gray-100">{ev.symbol}</span>
      <span className="text-sm text-gray-300">
        {ev.long_exch}↔{ev.short_exch}
        {showDirection && (
          <span className="text-xs text-gray-500"> ({ev.direction})</span>
        )}
      </span>
      <span className="text-xs font-mono tabular-nums text-gray-300 ml-auto">
        {t('pricegap.discovery.timeline.scoreLabel')} {ev.score}
      </span>
      <span className="text-xs font-mono tabular-nums text-gray-500">
        {t('pricegap.discovery.timeline.streakLabel')} {ev.streak_cycles}
      </span>
      <span className="text-xs text-gray-500 font-mono tabular-nums">
        {formatTimestamp(ev.ts)}
      </span>
    </div>
  );
};

export const PromoteTimeline: FC = () => {
  const { t } = useLocale();
  const {
    promoteEvents,
    promoteEventsLoading,
    promoteEventsError,
    wsConnected,
  } = usePgDiscovery();
  const [showAll, setShowAll] = useState(false);

  // Inherited card chrome — exact match with Phase 11 sibling cards.
  const cardCls = 'bg-gray-900 border border-gray-800 rounded-lg p-4';

  // Loading state — initial REST seed in flight.
  if (promoteEvents == null && promoteEventsLoading) {
    return (
      <div className={cardCls} data-testid="promote-timeline">
        <h4 className="text-base font-bold text-gray-300 mb-3">
          {t('pricegap.discovery.timeline.title')}
        </h4>
        <p className="text-xs text-gray-500">
          {t('pricegap.discovery.timeline.loading')}
        </p>
      </div>
    );
  }

  // Seed-error state — REST failed; live updates will resume on next WS event.
  if (promoteEventsError) {
    return (
      <div className={cardCls} data-testid="promote-timeline">
        <h4 className="text-base font-bold text-gray-300 mb-3">
          {t('pricegap.discovery.timeline.title')}
        </h4>
        <p role="status" className="text-xs" style={{ color: '#f6465d' }}>
          {t('pricegap.discovery.timeline.seedError')}
        </p>
      </div>
    );
  }

  const events = promoteEvents ?? [];

  // Empty state — seed succeeded, no events yet.
  if (events.length === 0) {
    return (
      <div className={cardCls} data-testid="promote-timeline">
        <div className="flex items-center justify-between mb-3">
          <h4 className="text-base font-bold text-gray-300">
            {t('pricegap.discovery.timeline.title')}
          </h4>
          {!wsConnected && (
            <span className="text-xs text-gray-500">
              {t('pricegap.discovery.timeline.wsDisconnected')}
            </span>
          )}
        </div>
        <p className="text-xs text-gray-500">
          {t('pricegap.discovery.timeline.empty')}
        </p>
      </div>
    );
  }

  // Populated state.
  const visible = showAll
    ? events.slice(0, HARD_CAP)
    : events.slice(0, MAX_VISIBLE_DEFAULT);
  const hasMore = !showAll && events.length > MAX_VISIBLE_DEFAULT;
  const loadMoreLabel = t('pricegap.discovery.timeline.loadMore').replace(
    '{n}',
    String(events.length),
  );

  return (
    <div className={cardCls} data-testid="promote-timeline">
      <div className="flex items-center justify-between mb-3">
        <h4 className="text-base font-bold text-gray-300">
          {t('pricegap.discovery.timeline.title')}
        </h4>
        {!wsConnected && (
          <span className="text-xs text-gray-500">
            {t('pricegap.discovery.timeline.wsDisconnected')}
          </span>
        )}
      </div>
      <div
        role="list"
        tabIndex={0}
        aria-live="polite"
        aria-atomic={false}
        className="max-h-[480px] overflow-y-auto focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-gray-700"
      >
        {visible.map((ev, i) => (
          <TimelineRow
            key={`${ev.ts}-${ev.action}-${ev.symbol}-${ev.long_exch}-${ev.short_exch}-${ev.direction}-${i}`}
            ev={ev}
            t={t}
          />
        ))}
      </div>
      {hasMore && (
        <div className="text-xs text-center pt-2">
          <button
            type="button"
            className="text-gray-300 hover:text-gray-100 underline"
            onClick={() => setShowAll(true)}
          >
            {loadMoreLabel}
          </button>
        </div>
      )}
    </div>
  );
};

export default PromoteTimeline;
