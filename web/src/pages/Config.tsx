import { useState, useEffect, useRef } from 'react';
import type { FC, FormEvent } from 'react';
import { useLocale, type TranslationKey } from '../i18n/index.ts';

interface ConfigProps {
  getConfig: () => Promise<Record<string, unknown>>;
  updateConfig: (data: Record<string, unknown>) => Promise<Record<string, unknown>>;
}

interface FieldDef {
  path: string[];  // nested path e.g. ["strategy", "entry", "slippage_limit_bps"]
  labelKey: TranslationKey;
  descKey?: TranslationKey;
  unit?: string;
}

interface SectionDef {
  titleKey: TranslationKey;
  descKey: TranslationKey;
  fields: FieldDef[];
}

const SECTIONS: SectionDef[] = [
  {
    titleKey: 'cfg.fundMgmt',
    descKey: 'cfg.fundMgmtDesc',
    fields: [
      { path: ['fund', 'max_positions'], labelKey: 'cfg.field.maxPositions', descKey: 'cfg.desc.maxPositions' },
      { path: ['fund', 'leverage'], labelKey: 'cfg.field.leverage', descKey: 'cfg.desc.leverage', unit: 'x' },
      { path: ['fund', 'capital_per_leg'], labelKey: 'cfg.field.capitalPerLeg', descKey: 'cfg.desc.capitalPerLeg', unit: 'USDT' },
    ],
  },
  {
    titleKey: 'cfg.schedule',
    descKey: 'cfg.scheduleDesc',
    fields: [
      { path: ['strategy', 'rebalance_scan_minute'], labelKey: 'cfg.field.rebalanceScanMinute', descKey: 'cfg.desc.rebalanceScanMinute' },
      { path: ['strategy', 'exit_scan_minute'], labelKey: 'cfg.field.exitScanMinute', descKey: 'cfg.desc.exitScanMinute' },
      { path: ['strategy', 'entry_scan_minute'], labelKey: 'cfg.field.entryScanMinute', descKey: 'cfg.desc.entryScanMinute' },
      { path: ['strategy', 'rotate_scan_minute'], labelKey: 'cfg.field.rotateScanMinute', descKey: 'cfg.desc.rotateScanMinute' },
    ],
  },
  {
    titleKey: 'cfg.discovery',
    descKey: 'cfg.discoveryDesc',
    fields: [
      { path: ['strategy', 'top_opportunities'], labelKey: 'cfg.field.topOpportunities', descKey: 'cfg.desc.topOpportunities' },
      { path: ['strategy', 'discovery', 'min_hold_time_hours'], labelKey: 'cfg.field.minHoldTime', descKey: 'cfg.desc.minHoldTime', unit: 'hours' },
      { path: ['strategy', 'discovery', 'max_cost_ratio'], labelKey: 'cfg.field.maxCostRatio', descKey: 'cfg.desc.maxCostRatio' },
      { path: ['strategy', 'discovery', 'price_gap_free_bps'], labelKey: 'cfg.field.freeGap', descKey: 'cfg.desc.freeGap', unit: 'bps' },
      { path: ['strategy', 'discovery', 'max_price_gap_bps'], labelKey: 'cfg.field.maxGap', descKey: 'cfg.desc.maxGap', unit: 'bps' },
      { path: ['strategy', 'discovery', 'max_gap_recovery_intervals'], labelKey: 'cfg.field.recoveryIntervals', descKey: 'cfg.desc.recoveryIntervals' },
      { path: ['strategy', 'discovery', 'persistence', 'funding_window_min'], labelKey: 'cfg.field.fundingWindowMin', descKey: 'cfg.desc.fundingWindowMin', unit: 'min' },
      { path: ['strategy', 'discovery', 'persistence', 'lookback_min_1h'], labelKey: 'cfg.field.lookbackMin1h', descKey: 'cfg.desc.lookbackMin1h', unit: 'min' },
      { path: ['strategy', 'discovery', 'persistence', 'lookback_min_4h'], labelKey: 'cfg.field.lookbackMin4h', descKey: 'cfg.desc.lookbackMin4h', unit: 'min' },
      { path: ['strategy', 'discovery', 'persistence', 'lookback_min_8h'], labelKey: 'cfg.field.lookbackMin8h', descKey: 'cfg.desc.lookbackMin8h', unit: 'min' },
      { path: ['strategy', 'discovery', 'persistence', 'min_count_1h'], labelKey: 'cfg.field.minCount1h', descKey: 'cfg.desc.minCount1h' },
      { path: ['strategy', 'discovery', 'persistence', 'min_count_4h'], labelKey: 'cfg.field.minCount4h', descKey: 'cfg.desc.minCount4h' },
      { path: ['strategy', 'discovery', 'persistence', 'min_count_8h'], labelKey: 'cfg.field.minCount8h', descKey: 'cfg.desc.minCount8h' },
      { path: ['strategy', 'discovery', 'persistence', 'spread_stability_ratio_1h'], labelKey: 'cfg.field.stabilityRatio1h', descKey: 'cfg.desc.stabilityRatio1h' },
      { path: ['strategy', 'discovery', 'persistence', 'spread_stability_ratio_4h'], labelKey: 'cfg.field.stabilityRatio4h', descKey: 'cfg.desc.stabilityRatio4h' },
      { path: ['strategy', 'discovery', 'persistence', 'spread_stability_ratio_8h'], labelKey: 'cfg.field.stabilityRatio8h', descKey: 'cfg.desc.stabilityRatio8h' },
      { path: ['strategy', 'discovery', 'persistence', 'spread_stability_oi_rank_1h'], labelKey: 'cfg.field.stabilityOIRank1h', descKey: 'cfg.desc.stabilityOIRank1h' },
      { path: ['strategy', 'discovery', 'persistence', 'spread_stability_oi_rank_4h'], labelKey: 'cfg.field.stabilityOIRank4h', descKey: 'cfg.desc.stabilityOIRank4h' },
      { path: ['strategy', 'discovery', 'persistence', 'spread_stability_oi_rank_8h'], labelKey: 'cfg.field.stabilityOIRank8h', descKey: 'cfg.desc.stabilityOIRank8h' },
      { path: ['strategy', 'discovery', 'persistence', 'spread_volatility_max_cv'], labelKey: 'cfg.field.spreadVolatilityMaxCV', descKey: 'cfg.desc.spreadVolatilityMaxCV' },
      { path: ['strategy', 'discovery', 'persistence', 'spread_volatility_min_samples'], labelKey: 'cfg.field.spreadVolatilityMinSamples', descKey: 'cfg.desc.spreadVolatilityMinSamples' },
    ],
  },
  {
    titleKey: 'cfg.entryExec',
    descKey: 'cfg.entryExecDesc',
    fields: [
      { path: ['strategy', 'entry', 'entry_timeout_sec'], labelKey: 'cfg.field.entryTimeout', descKey: 'cfg.desc.entryTimeout', unit: 'sec' },
      { path: ['strategy', 'entry', 'min_chunk_usdt'], labelKey: 'cfg.field.minChunkSize', descKey: 'cfg.desc.minChunkSize', unit: 'USDT' },
      { path: ['strategy', 'entry', 'slippage_limit_bps'], labelKey: 'cfg.field.slippageLimit', descKey: 'cfg.desc.slippageLimit', unit: 'bps' },
      { path: ['strategy', 'entry', 'order_advance_min'], labelKey: 'cfg.field.orderAdvance', unit: 'min' },
      { path: ['strategy', 'entry', 'loss_cooldown_hours'], labelKey: 'cfg.field.lossCooldownHours', descKey: 'cfg.desc.lossCooldownHours', unit: 'h' },
      { path: ['strategy', 'entry', 're_enter_cooldown_hours'], labelKey: 'cfg.field.reEnterCooldownHours', descKey: 'cfg.desc.reEnterCooldownHours', unit: 'h' },
      { path: ['strategy', 'entry', 'backtest_days'], labelKey: 'cfg.field.backtestDays', descKey: 'cfg.desc.backtestDays', unit: 'd' },
      { path: ['strategy', 'entry', 'backtest_min_profit'], labelKey: 'cfg.field.backtestMinProfit', descKey: 'cfg.desc.backtestMinProfit' },
    ],
  },
  {
    titleKey: 'cfg.exit',
    descKey: 'cfg.exitDesc',
    fields: [
      { path: ['strategy', 'exit', 'exit_mode'], labelKey: 'cfg.field.exitMode' },
      { path: ['strategy', 'exit', 'depth_timeout_sec'], labelKey: 'cfg.field.depthExitTimeout', descKey: 'cfg.desc.depthExitTimeout', unit: 'sec' },
      { path: ['strategy', 'exit', 'spread_reversal_tolerance'], labelKey: 'cfg.field.spreadReversalTolerance', descKey: 'cfg.desc.spreadReversalTolerance' },
    ],
  },
  {
    titleKey: 'cfg.rotation',
    descKey: 'cfg.rotationDesc',
    fields: [
      { path: ['strategy', 'rotation', 'threshold_bps'], labelKey: 'cfg.field.threshold', descKey: 'cfg.desc.rotationThreshold', unit: 'bps' },
      { path: ['strategy', 'rotation', 'cooldown_min'], labelKey: 'cfg.field.cooldown', descKey: 'cfg.desc.rotationCooldown', unit: 'min' },
    ],
  },
  {
    titleKey: 'cfg.marginHealth',
    descKey: 'cfg.marginHealthDesc',
    fields: [
      { path: ['risk', 'margin_l3_threshold'], labelKey: 'cfg.field.l3TransferTrigger', descKey: 'cfg.desc.l3TransferTrigger' },
      { path: ['risk', 'margin_l4_threshold'], labelKey: 'cfg.field.l4ReduceTrigger', descKey: 'cfg.desc.l4ReduceTrigger' },
      { path: ['risk', 'margin_l5_threshold'], labelKey: 'cfg.field.l5EmergencyClose', descKey: 'cfg.desc.l5EmergencyClose' },
      { path: ['risk', 'l4_reduce_fraction'], labelKey: 'cfg.field.l4ReduceFraction', descKey: 'cfg.desc.l4ReduceFraction' },
    ],
  },
  {
    titleKey: 'cfg.system',
    descKey: 'cfg.systemDesc',
    fields: [
      { path: ['dry_run'], labelKey: 'cfg.field.dryRun', descKey: 'cfg.desc.dryRun' },
    ],
  },
];

// Get a nested value by path
function getByPath(obj: Record<string, unknown>, path: string[]): unknown {
  let cur: unknown = obj;
  for (const key of path) {
    if (cur == null || typeof cur !== 'object') return undefined;
    cur = (cur as Record<string, unknown>)[key];
  }
  return cur;
}

// Set a nested value by path (immutable)
function setByPath(obj: Record<string, unknown>, path: string[], value: unknown): Record<string, unknown> {
  if (path.length === 0) return obj;
  const [head, ...rest] = path;
  const child = (obj[head] ?? {}) as Record<string, unknown>;
  return {
    ...obj,
    [head]: rest.length === 0 ? value : setByPath(child, rest, value),
  };
}

// Tooltip component with styled popover
const Tooltip: FC<{ text: string }> = ({ text }) => {
  const [show, setShow] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!show) return;
    const handleClick = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setShow(false);
    };
    document.addEventListener('mousedown', handleClick);
    return () => document.removeEventListener('mousedown', handleClick);
  }, [show]);

  return (
    <div ref={ref} className="relative inline-flex ml-1">
      <button
        type="button"
        onClick={() => setShow(!show)}
        onMouseEnter={() => setShow(true)}
        onMouseLeave={() => setShow(false)}
        className="w-4 h-4 rounded-full bg-gray-700 hover:bg-gray-600 text-gray-400 hover:text-gray-200 text-[10px] leading-4 text-center transition-colors cursor-help"
      >
        ?
      </button>
      {show && (
        <div className="absolute z-50 bottom-full left-1/2 -translate-x-1/2 mb-2 w-64 px-3 py-2 text-xs text-gray-200 bg-gray-800 border border-gray-600 rounded-lg shadow-lg">
          {text}
          <div className="absolute top-full left-1/2 -translate-x-1/2 -mt-px">
            <div className="w-2 h-2 bg-gray-800 border-r border-b border-gray-600 rotate-45 -translate-y-1" />
          </div>
        </div>
      )}
    </div>
  );
};

const Config: FC<ConfigProps> = ({ getConfig, updateConfig }) => {
  const { t } = useLocale();
  const [config, setConfig] = useState<Record<string, unknown>>({});
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState('');
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    getConfig()
      .then((data) => {
        setConfig(data);
        setLoading(false);
      })
      .catch(() => setLoading(false));
  }, [getConfig]);

  const handleChange = (path: string[], value: string) => {
    setConfig((prev) => {
      const original = getByPath(prev, path);
      let parsed: unknown = value;
      if (typeof original === 'number') {
        const n = parseFloat(value);
        if (!isNaN(n)) parsed = n;
      } else if (typeof original === 'boolean') {
        parsed = value === 'true';
      }
      return setByPath(prev, path, parsed);
    });
  };

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setSaving(true);
    setMessage('');
    try {
      await updateConfig(config);
      setMessage(t('cfg.saved'));
    } catch {
      setMessage(t('cfg.failed'));
    } finally {
      setSaving(false);
      setTimeout(() => setMessage(''), 3000);
    }
  };

  if (loading) {
    return (
      <div className="space-y-6">
        <h2 className="text-xl font-bold text-gray-100">{t('cfg.title')}</h2>
        <p className="text-gray-500 text-sm">{t('cfg.loading')}</p>
      </div>
    );
  }

  const renderField = (field: FieldDef) => {
    const val = getByPath(config, field.path);
    if (val === undefined) return null;
    const isBoolean = typeof val === 'boolean';
    const isExitMode = field.path[field.path.length - 1] === 'exit_mode';

    return (
      <div key={field.path.join('.')} className="flex flex-col sm:flex-row sm:items-center gap-1.5 sm:gap-3">
        <label className="text-sm text-gray-400 sm:w-48 shrink-0 flex items-center">
          {t(field.labelKey)}
          {field.unit && <span className="text-gray-600 ml-1">({field.unit})</span>}
          {field.descKey && <Tooltip text={t(field.descKey)} />}
        </label>
        {isBoolean ? (
          <select
            value={String(val)}
            onChange={(e) => handleChange(field.path, e.target.value)}
            className="w-full sm:w-48 bg-gray-800 border border-gray-700 rounded-md px-3 py-1.5 text-gray-100 text-sm focus:outline-none focus:border-blue-500"
          >
            <option value="true">true</option>
            <option value="false">false</option>
          </select>
        ) : isExitMode ? (
          <select
            value={String(val)}
            onChange={(e) => handleChange(field.path, e.target.value)}
            className="w-full sm:w-48 bg-gray-800 border border-gray-700 rounded-md px-3 py-1.5 text-gray-100 text-sm focus:outline-none focus:border-blue-500"
          >
            <option value="wait">wait</option>
            <option value="spread_reversal">spread_reversal</option>
          </select>
        ) : (
          <input
            type="text"
            value={String(val ?? '')}
            onChange={(e) => handleChange(field.path, e.target.value)}
            className="w-full sm:w-48 bg-gray-800 border border-gray-700 rounded-md px-3 py-1.5 text-gray-100 text-sm font-mono focus:outline-none focus:border-blue-500"
          />
        )}
      </div>
    );
  };

  return (
    <div className="space-y-6">
      <h2 className="text-xl font-bold text-gray-100">{t('cfg.title')}</h2>
      <form onSubmit={handleSubmit} className="space-y-6 max-w-2xl">
        {SECTIONS.map((section) => {
          const hasFields = section.fields.some((f) => getByPath(config, f.path) !== undefined);
          if (!hasFields) return null;
          return (
            <div key={section.titleKey} className="bg-gray-900 border border-gray-800 rounded-lg p-4">
              <h3 className="text-sm font-semibold text-gray-200 mb-0.5">{t(section.titleKey)}</h3>
              <p className="text-xs text-gray-500 mb-3">{t(section.descKey)}</p>
              <div className="space-y-2.5">
                {section.fields.map(renderField)}
              </div>
            </div>
          );
        })}
        <div className="flex items-center gap-4">
          <button
            type="submit"
            disabled={saving}
            className="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-md text-sm font-medium transition-colors disabled:opacity-50"
          >
            {saving ? t('cfg.saving') : t('cfg.save')}
          </button>
          {message && (
            <span className={`text-sm ${message === t('cfg.saved') ? 'text-green-400' : 'text-red-400'}`}>
              {message}
            </span>
          )}
        </div>
      </form>
    </div>
  );
};

export default Config;
