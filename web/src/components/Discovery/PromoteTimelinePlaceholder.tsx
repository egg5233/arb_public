// Phase 11 Plan 06 — PromoteTimelinePlaceholder.
//
// Empty card with a dashed border signalling "Phase 12 will fill this in"
// (auto-promotion).  Renders title + low-emphasis placeholder copy.
import type { FC } from 'react';
import { useLocale } from '../../i18n/index.ts';

export const PromoteTimelinePlaceholder: FC = () => {
  const { t } = useLocale();
  return (
    <div
      className="bg-gray-900 border border-dashed border-gray-700 rounded-lg p-8 text-center"
      data-testid="promote-timeline-placeholder"
    >
      <h4 className="text-base font-bold text-gray-300 mb-2">
        {t('pricegap.discovery.timeline.title')}
      </h4>
      <p className="text-sm text-gray-500">
        {t('pricegap.discovery.timeline.placeholder')}
      </p>
    </div>
  );
};

export default PromoteTimelinePlaceholder;
