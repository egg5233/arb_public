import type { FC } from 'react';
import { useLocale, type TranslationKey } from '../i18n/index.ts';

interface TimeRangeSelectorProps {
  selected: string;
  onChange: (range: string) => void;
}

const RANGES: { value: string; labelKey: TranslationKey }[] = [
  { value: '7d', labelKey: 'analytics.range7d' },
  { value: '30d', labelKey: 'analytics.range30d' },
  { value: '90d', labelKey: 'analytics.range90d' },
  { value: 'all', labelKey: 'analytics.rangeAll' },
];

const TimeRangeSelector: FC<TimeRangeSelectorProps> = ({ selected, onChange }) => {
  const { t } = useLocale();

  return (
    <div className="flex gap-2">
      {RANGES.map((r) => (
        <button
          key={r.value}
          type="button"
          aria-pressed={selected === r.value}
          onClick={() => onChange(r.value)}
          className={`px-3 py-2 text-sm rounded-md transition-colors ${
            selected === r.value
              ? 'bg-blue-500/20 text-blue-400'
              : 'bg-gray-800 text-gray-400 hover:text-gray-300 hover:bg-gray-700'
          }`}
        >
          {t(r.labelKey)}
        </button>
      ))}
    </div>
  );
};

export default TimeRangeSelector;
