import { useState, useEffect, useRef } from 'react';
import type { FC, FormEvent } from 'react';
import { useLocale, type TranslationKey } from '../i18n/index.ts';

interface ConfigProps {
  getConfig: () => Promise<Record<string, unknown>>;
  updateConfig: (data: Record<string, unknown>) => Promise<Record<string, unknown>>;
}

// ---------------------------------------------------------------------------
// Tab definitions
// ---------------------------------------------------------------------------
type Strategy = 'exchanges' | 'perp' | 'spot';
type PerpTabId = 'fund' | 'schedule' | 'discovery' | 'persist' | 'entry' | 'exit' | 'risk';
type SpotTabId = 'sf-general' | 'sf-sizing' | 'sf-discovery' | 'sf-exit';
type TabId = PerpTabId | SpotTabId;

const PERP_TABS: { id: PerpTabId; labelKey: TranslationKey }[] = [
  { id: 'fund', labelKey: 'cfg.tab.fund' },
  { id: 'schedule', labelKey: 'cfg.tab.schedule' },
  { id: 'discovery', labelKey: 'cfg.tab.discovery' },
  { id: 'persist', labelKey: 'cfg.tab.persistence' },
  { id: 'entry', labelKey: 'cfg.tab.entry' },
  { id: 'exit', labelKey: 'cfg.tab.exitRotation' },
  { id: 'risk', labelKey: 'cfg.tab.risk' },
];

const SPOT_TABS: { id: SpotTabId; labelKey: TranslationKey }[] = [
  { id: 'sf-general', labelKey: 'cfg.sf.tabGeneral' },
  { id: 'sf-sizing', labelKey: 'cfg.sf.tabSizing' },
  { id: 'sf-discovery', labelKey: 'cfg.sf.tabDiscovery' },
  { id: 'sf-exit', labelKey: 'cfg.sf.tabExitRisk' },
];

// Exchange metadata
const EXCHANGE_LIST = [
  { id: 'binance', name: 'Binance', hasPassphrase: false },
  { id: 'bybit', name: 'Bybit', hasPassphrase: false },
  { id: 'gateio', name: 'Gate.io', hasPassphrase: false },
  { id: 'bitget', name: 'Bitget', hasPassphrase: true },
  { id: 'okx', name: 'OKX', hasPassphrase: true },
  { id: 'bingx', name: 'BingX', hasPassphrase: false },
];

const LEVERAGE_OPTIONS = [1, 2, 3, 5, 10, 20];
const SF_LEVERAGE_OPTIONS = [1, 2, 3, 5];

// ---------------------------------------------------------------------------
// Utility: nested path get/set
// ---------------------------------------------------------------------------
function getByPath(obj: Record<string, unknown>, path: string[]): unknown {
  let cur: unknown = obj;
  for (const key of path) {
    if (cur == null || typeof cur !== 'object') return undefined;
    cur = (cur as Record<string, unknown>)[key];
  }
  return cur;
}

function setByPath(obj: Record<string, unknown>, path: string[], value: unknown): Record<string, unknown> {
  if (path.length === 0) return obj;
  const [head, ...rest] = path;
  const child = (obj[head] ?? {}) as Record<string, unknown>;
  return {
    ...obj,
    [head]: rest.length === 0 ? value : setByPath(child, rest, value),
  };
}

// ---------------------------------------------------------------------------
// Tooltip component
// ---------------------------------------------------------------------------
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

// ---------------------------------------------------------------------------
// Toggle Switch component
// ---------------------------------------------------------------------------
const ToggleSwitch: FC<{ on: boolean; onChange: (v: boolean) => void; disabled?: boolean }> = ({ on, onChange, disabled }) => (
  <button
    type="button"
    disabled={disabled}
    onClick={() => onChange(!on)}
    className={`relative inline-flex h-6 w-11 shrink-0 rounded-full transition-colors duration-200 focus:outline-none ${on ? 'bg-blue-600' : 'bg-gray-600'} ${disabled ? 'opacity-50 cursor-not-allowed' : 'cursor-pointer'}`}
  >
    <span className={`pointer-events-none inline-block h-5 w-5 rounded-full bg-white shadow transform transition-transform duration-200 translate-y-0.5 ${on ? 'translate-x-[22px]' : 'translate-x-0.5'}`} />
  </button>
);

// ---------------------------------------------------------------------------
// Password Input with eye toggle
// ---------------------------------------------------------------------------
const PasswordInput: FC<{
  value: string;
  preview?: string;
  hasValue?: boolean;
  onChange: (v: string) => void;
  placeholder?: string;
}> = ({ value, preview, hasValue, onChange, placeholder }) => {
  const [visible, setVisible] = useState(false);
  const displayPlaceholder = hasValue ? (preview ? `${preview}` : 'Set') : (placeholder || '');

  return (
    <div className="relative">
      <input
        type={visible ? 'text' : 'password'}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={displayPlaceholder}
        className="w-full bg-gray-800 border border-gray-700 rounded-lg py-1.5 px-3 pr-9 text-sm font-mono text-gray-100 focus:outline-none focus:border-blue-500"
      />
      <button
        type="button"
        onClick={() => setVisible(!visible)}
        className="absolute right-2 top-1/2 -translate-y-1/2 text-gray-500 hover:text-gray-300"
      >
        {visible ? (
          <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13.875 18.825A10.05 10.05 0 0112 19c-4.478 0-8.268-2.943-9.543-7a9.97 9.97 0 011.563-3.029m5.858.908a3 3 0 114.243 4.243M9.878 9.878l4.242 4.242M9.878 9.878L3 3m6.878 6.878L21 21" />
          </svg>
        ) : (
          <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z" />
          </svg>
        )}
      </button>
    </div>
  );
};

// ---------------------------------------------------------------------------
// Range Slider component
// ---------------------------------------------------------------------------
const RangeSlider: FC<{
  value: number;
  min: number;
  max: number;
  step?: number;
  onChange: (v: number) => void;
  colorClass?: string;
  unit?: string;
  displayMultiplier?: number;
}> = ({ value, min, max, step = 0.01, onChange, colorClass = 'text-blue-400', unit = '%', displayMultiplier = 100 }) => (
  <div>
    <input
      type="range"
      min={min}
      max={max}
      step={step}
      value={value}
      onChange={(e) => onChange(parseFloat(e.target.value))}
      className="w-full accent-blue-500"
      style={{ WebkitAppearance: 'none', appearance: 'none', background: 'transparent' }}
    />
    <div className={`text-right text-sm font-mono mt-1 ${colorClass}`}>
      {(value * displayMultiplier).toFixed(0)}{unit}
    </div>
  </div>
);

// ---------------------------------------------------------------------------
// Accordion component
// ---------------------------------------------------------------------------
const Accordion: FC<{ title: string; defaultOpen?: boolean; children: React.ReactNode }> = ({ title, defaultOpen = false, children }) => {
  const [open, setOpen] = useState(defaultOpen);
  return (
    <div className="bg-gray-900 rounded-xl border border-gray-800 overflow-hidden">
      <button
        type="button"
        className="w-full flex items-center justify-between px-4 py-3 hover:bg-gray-800/50 transition"
        onClick={() => setOpen(!open)}
      >
        <span className="text-sm font-semibold">{title}</span>
        <svg className={`w-4 h-4 text-gray-500 transition-transform duration-200 ${open ? 'rotate-180' : ''}`} fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
        </svg>
      </button>
      <div className={`overflow-hidden transition-all duration-300 ${open ? 'max-h-[600px] pb-4 px-4' : 'max-h-0'}`}>
        {children}
      </div>
    </div>
  );
};

// ---------------------------------------------------------------------------
// ToggleField - a card with label, tooltip, and on/off toggle
// ---------------------------------------------------------------------------
const ToggleField: FC<{
  label: string;
  desc?: string;
  value: unknown;
  onChange: (v: boolean) => void;
}> = ({ label, desc, value, onChange }) => {
  const checked = Boolean(value);
  return (
    <div className="bg-gray-900 rounded-xl p-4 border border-gray-800">
      <div className="flex items-center justify-between">
        <div>
          <div className="text-sm font-medium text-gray-200">{label}</div>
          {desc && <div className="text-xs text-gray-500 mt-1">{desc}</div>}
        </div>
        <button
          type="button"
          onClick={() => onChange(!checked)}
          className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors ${checked ? 'bg-blue-600' : 'bg-gray-700'}`}
        >
          <span className={`inline-block h-4 w-4 transform rounded-full bg-white transition-transform ${checked ? 'translate-x-6' : 'translate-x-1'}`} />
        </button>
      </div>
    </div>
  );
};

// NumberField - a card with label, tooltip, and number input
// ---------------------------------------------------------------------------
const NumberField: FC<{
  label: string;
  desc?: string;
  value: unknown;
  unit?: string;
  onChange: (v: string) => void;
}> = ({ label, desc, value, unit, onChange }) => {
  const [localVal, setLocalVal] = useState(String(value ?? ''));
  const [focused, setFocused] = useState(false);

  useEffect(() => {
    if (!focused) setLocalVal(String(value ?? ''));
  }, [value, focused]);

  return (
    <div className="bg-gray-900 rounded-xl p-4 border border-gray-800">
      <div className="flex items-center gap-2 mb-2">
        <label className="text-sm font-medium">{label}</label>
        {unit && <span className="text-xs text-gray-500">({unit})</span>}
        {desc && <Tooltip text={desc} />}
      </div>
      <input
        type="text"
        value={focused ? localVal : String(value ?? '')}
        onFocus={() => { setFocused(true); setLocalVal(String(value ?? '')); }}
        onChange={(e) => { setLocalVal(e.target.value); }}
        onBlur={() => { setFocused(false); onChange(localVal); }}
        className="w-full bg-gray-800 border border-gray-700 rounded-lg py-1.5 px-3 text-sm font-mono text-gray-100 focus:outline-none focus:border-blue-500"
      />
    </div>
  );
}

// ---------------------------------------------------------------------------
// Main Config Component
// ---------------------------------------------------------------------------
const Config: FC<ConfigProps> = ({ getConfig, updateConfig }) => {
  const { t } = useLocale();
  const [config, setConfig] = useState<Record<string, unknown>>({});
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState('');
  const [loading, setLoading] = useState(true);
  const [dirty, setDirty] = useState(false);
  const [strategy, setStrategy] = useState<Strategy>('exchanges');
  const [activeTab, setActiveTab] = useState<TabId>('fund');
  const tabBarRef = useRef<HTMLDivElement>(null);

  // Exchange overrides: only fields the user actually typed
  const [exchangeOverrides, setExchangeOverrides] = useState<Record<string, Record<string, string>>>({});

  // Track which config paths were changed (dirty fields only)
  const [dirtyPaths, setDirtyPaths] = useState<Set<string>>(new Set());

  useEffect(() => {
    getConfig()
      .then((data) => {
        setConfig(data);
        setLoading(false);
      })
      .catch(() => setLoading(false));
  }, [getConfig]);

  // Generic change handler for config paths
  const handleChange = (path: string[], value: string) => {
    setDirty(true);
    setDirtyPaths((prev) => new Set(prev).add(path[0]));
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

  const handleBoolChange = (path: string[], value: boolean) => {
    setDirty(true);
    setDirtyPaths((prev) => new Set(prev).add(path[0]));
    setConfig((prev) => setByPath(prev, path, value));
  };

  const handleNumberChange = (path: string[], value: number) => {
    setDirty(true);
    setDirtyPaths((prev) => new Set(prev).add(path[0]));
    setConfig((prev) => setByPath(prev, path, value));
  };

  // Exchange field change - track overrides
  const handleExchangeField = (exId: string, field: string, value: string) => {
    setDirty(true);
    setExchangeOverrides((prev) => ({
      ...prev,
      [exId]: { ...(prev[exId] || {}), [field]: value },
    }));
  };

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setSaving(true);
    setMessage('');
    try {
      // Only send dirty sections — backend supports partial updates
      const submitData: Record<string, unknown> = {};
      for (const key of dirtyPaths) {
        if (key !== 'exchanges') {
          submitData[key] = config[key];
        }
      }

      // Exchange overrides: only send fields the user actually typed
      if (Object.keys(exchangeOverrides).length > 0) {
        submitData.exchanges = exchangeOverrides;
      }

      const updated = await updateConfig(submitData);
      setConfig(updated);
      setMessage(t('cfg.saved'));
      setDirty(false);
      setDirtyPaths(new Set());
      setExchangeOverrides({});
    } catch (err) {
      setMessage(err instanceof Error ? err.message : t('cfg.failed'));
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

  // Helper to get exchange info from config
  const getExchangeInfo = (exId: string) => {
    const exchanges = (config.exchanges || {}) as Record<string, Record<string, unknown>>;
    return exchanges[exId] || {};
  };

  // =========================================================================
  // Tab: Exchanges
  // =========================================================================
  const renderExchangesTab = () => (
    <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
      {EXCHANGE_LIST.map((ex) => {
        const info = getExchangeInfo(ex.id);
        const enabled = Boolean(info.enabled);
        const hasApiKey = Boolean(info.has_api_key);
        const hasSecretKey = Boolean(info.has_secret_key);
        const hasPassphrase = info.has_passphrase === true;
        const apiKeyPreview = (info.api_key_preview as string) || '';
        const address = (info.address as Record<string, string>) || {};
        const overrides = exchangeOverrides[ex.id] || {};

        return (
          <div
            key={ex.id}
            className={`bg-gray-900 rounded-xl p-4 border border-gray-800 transition ${!enabled ? 'opacity-45' : ''}`}
          >
            {/* Header with toggle */}
            <div className="flex items-center justify-between mb-3">
              <span className="font-semibold text-sm">{ex.name}</span>
              <ToggleSwitch
                on={enabled}
                onChange={(v) => {
                  setDirty(true);
                  const exchanges = ((config.exchanges || {}) as Record<string, unknown>);
                  const exData = ((exchanges[ex.id] as Record<string, unknown>) || {});
                  const updated = { ...exchanges, [ex.id]: { ...exData, enabled: v } };
                  setConfig((prev) => ({ ...prev, exchanges: updated }));
                }}
              />
            </div>

            {/* Body */}
            <div className={!enabled ? 'pointer-events-none' : ''}>
              {/* API Key */}
              <div className="mb-3">
                <label className="text-xs text-gray-400 block mb-1">
                  API Key
                  {hasApiKey && <span className="text-green-400 text-xs ml-2">&#x2713;</span>}
                </label>
                <PasswordInput
                  value={overrides.api_key || ''}
                  preview={apiKeyPreview}
                  hasValue={hasApiKey}
                  onChange={(v) => handleExchangeField(ex.id, 'api_key', v)}
                  placeholder={t('cfg.exchange.notConfigured')}
                />
              </div>

              {/* Secret Key */}
              <div className="mb-3">
                <label className="text-xs text-gray-400 block mb-1">
                  Secret Key
                  {hasSecretKey && <span className="text-green-400 text-xs ml-2">&#x2713;</span>}
                </label>
                <PasswordInput
                  value={overrides.secret_key || ''}
                  hasValue={hasSecretKey}
                  onChange={(v) => handleExchangeField(ex.id, 'secret_key', v)}
                  placeholder={t('cfg.exchange.notConfigured')}
                />
              </div>

              {/* Passphrase (only bitget, okx) */}
              {ex.hasPassphrase && (
                <div className="mb-3">
                  <label className="text-xs text-gray-400 block mb-1">
                    Passphrase
                    {hasPassphrase && <span className="text-green-400 text-xs ml-2">&#x2713;</span>}
                  </label>
                  <PasswordInput
                    value={overrides.passphrase || ''}
                    hasValue={hasPassphrase}
                    onChange={(v) => handleExchangeField(ex.id, 'passphrase', v)}
                    placeholder={t('cfg.exchange.notConfigured')}
                  />
                </div>
              )}

            </div>
          </div>
        );
      })}
    </div>
  );

  // =========================================================================
  // Tab: Fund Management
  // =========================================================================
  const renderFundTab = () => {
    const dryRun = getByPath(config, ['dry_run']) as boolean | undefined;

    return (
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
        <NumberField
          label={t('cfg.field.maxPositions')}
          desc={t('cfg.desc.maxPositions')}
          value={getByPath(config, ['fund', 'max_positions'])}
          onChange={(v) => handleChange(['fund', 'max_positions'], v)}
        />
        {/* Leverage dropdown */}
        <div className="bg-gray-900 rounded-xl p-4 border border-gray-800">
          <div className="flex items-center gap-2 mb-2">
            <label className="text-sm font-medium">{t('cfg.field.leverage')}</label>
            <Tooltip text={t('cfg.desc.leverage')} />
          </div>
          <select
            value={String(getByPath(config, ['fund', 'leverage']) ?? 3)}
            onChange={(e) => handleChange(['fund', 'leverage'], e.target.value)}
            className="w-full bg-gray-800 border border-gray-700 rounded-lg py-2 px-3 text-sm text-gray-100 focus:outline-none focus:border-blue-500"
          >
            {LEVERAGE_OPTIONS.map((lev) => (
              <option key={lev} value={lev}>{lev}x</option>
            ))}
          </select>
        </div>

        <NumberField
          label={t('cfg.field.capitalPerLeg')}
          desc={t('cfg.desc.capitalPerLeg')}
          value={getByPath(config, ['fund', 'capital_per_leg'])}
          unit="USDT"
          onChange={(v) => handleChange(['fund', 'capital_per_leg'], v)}
        />

        {/* Dry Run toggle */}
        <div className="bg-gray-900 rounded-xl p-4 border border-gray-800">
          <div className="flex items-center gap-2 mb-2">
            <label className="text-sm font-medium">{t('cfg.field.dryRun')}</label>
            <Tooltip text={t('cfg.desc.dryRun')} />
          </div>
          <div className="flex items-center gap-3">
            <ToggleSwitch
              on={dryRun === true}
              onChange={(v) => handleBoolChange(['dry_run'], v)}
            />
            <span className={`text-sm font-semibold ${dryRun ? 'text-green-400' : 'text-red-400'}`}>
              {dryRun ? t('cfg.field.dryRunOn') : t('cfg.field.dryRunOff')}
            </span>
          </div>
        </div>
      </div>
    );
  };

  // =========================================================================
  // Tab: Schedule
  // =========================================================================
  const renderScheduleTab = () => (
    <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
      <ToggleField
        label={t('cfg.field.rebalanceAfterExit')}
        desc={t('cfg.desc.rebalanceAfterExit')}
        value={getByPath(config, ['strategy', 'rebalance_after_exit'])}
        onChange={(v) => handleChange(['strategy', 'rebalance_after_exit'], String(v))}
      />
      <NumberField
        label={t('cfg.field.rebalanceScanMinute')}
        desc={t('cfg.desc.rebalanceScanMinute')}
        value={getByPath(config, ['strategy', 'rebalance_scan_minute'])}
        onChange={(v) => handleChange(['strategy', 'rebalance_scan_minute'], v)}
      />
      <NumberField
        label={t('cfg.field.exitScanMinute')}
        desc={t('cfg.desc.exitScanMinute')}
        value={getByPath(config, ['strategy', 'exit_scan_minute'])}
        onChange={(v) => handleChange(['strategy', 'exit_scan_minute'], v)}
      />
      <NumberField
        label={t('cfg.field.entryScanMinute')}
        desc={t('cfg.desc.entryScanMinute')}
        value={getByPath(config, ['strategy', 'entry_scan_minute'])}
        onChange={(v) => handleChange(['strategy', 'entry_scan_minute'], v)}
      />
      <NumberField
        label={t('cfg.field.rotateScanMinute')}
        desc={t('cfg.desc.rotateScanMinute')}
        value={getByPath(config, ['strategy', 'rotate_scan_minute'])}
        onChange={(v) => handleChange(['strategy', 'rotate_scan_minute'], v)}
      />
    </div>
  );

  // =========================================================================
  // Tab: Discovery (without persistence fields)
  // =========================================================================
  const renderDiscoveryTab = () => (
    <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
      <NumberField
        label={t('cfg.field.topOpportunities')}
        desc={t('cfg.desc.topOpportunities')}
        value={getByPath(config, ['strategy', 'top_opportunities'])}
        onChange={(v) => handleChange(['strategy', 'top_opportunities'], v)}
      />
      <NumberField
        label={t('cfg.field.minHoldTime')}
        desc={t('cfg.desc.minHoldTime')}
        value={getByPath(config, ['strategy', 'discovery', 'min_hold_time_hours'])}
        unit="hours"
        onChange={(v) => handleChange(['strategy', 'discovery', 'min_hold_time_hours'], v)}
      />
      <NumberField
        label={t('cfg.field.maxCostRatio')}
        desc={t('cfg.desc.maxCostRatio')}
        value={getByPath(config, ['strategy', 'discovery', 'max_cost_ratio'])}
        onChange={(v) => handleChange(['strategy', 'discovery', 'max_cost_ratio'], v)}
      />
      <NumberField
        label={t('cfg.field.freeGap')}
        desc={t('cfg.desc.freeGap')}
        value={getByPath(config, ['strategy', 'discovery', 'price_gap_free_bps'])}
        unit="bps"
        onChange={(v) => handleChange(['strategy', 'discovery', 'price_gap_free_bps'], v)}
      />
      <NumberField
        label={t('cfg.field.maxGap')}
        desc={t('cfg.desc.maxGap')}
        value={getByPath(config, ['strategy', 'discovery', 'max_price_gap_bps'])}
        unit="bps"
        onChange={(v) => handleChange(['strategy', 'discovery', 'max_price_gap_bps'], v)}
      />
      <NumberField
        label={t('cfg.field.recoveryIntervals')}
        desc={t('cfg.desc.recoveryIntervals')}
        value={getByPath(config, ['strategy', 'discovery', 'max_gap_recovery_intervals'])}
        onChange={(v) => handleChange(['strategy', 'discovery', 'max_gap_recovery_intervals'], v)}
      />
      <NumberField
        label={t('cfg.field.maxIntervalHours')}
        desc={t('cfg.desc.maxIntervalHours')}
        value={getByPath(config, ['strategy', 'discovery', 'max_interval_hours'])}
        unit="h"
        onChange={(v) => handleChange(['strategy', 'discovery', 'max_interval_hours'], v)}
      />
      <NumberField
        label={t('cfg.field.fundingWindowMin')}
        desc={t('cfg.desc.fundingWindowMin')}
        value={getByPath(config, ['strategy', 'discovery', 'persistence', 'funding_window_min'])}
        unit="min"
        onChange={(v) => handleChange(['strategy', 'discovery', 'persistence', 'funding_window_min'], v)}
      />
      <div>
        <div className="flex items-center gap-2 mb-2">
          <label className="text-sm font-medium">{t('cfg.field.delistFilter')}</label>
          <Tooltip text={t('cfg.desc.delistFilter')} />
        </div>
        <div className="flex items-center gap-3">
          <ToggleSwitch
            on={getByPath(config, ['strategy', 'discovery', 'delist_filter']) === true}
            onChange={(v) => handleBoolChange(['strategy', 'discovery', 'delist_filter'], v)}
          />
          <span className={`text-sm font-semibold ${getByPath(config, ['strategy', 'discovery', 'delist_filter']) ? 'text-green-400' : 'text-red-400'}`}>
            {getByPath(config, ['strategy', 'discovery', 'delist_filter']) ? 'ON' : 'OFF'}
          </span>
        </div>
      </div>
    </div>
  );

  // =========================================================================
  // Tab: Persistence (accordion)
  // =========================================================================
  const renderPersistenceField = (labelKey: TranslationKey, descKey: TranslationKey | undefined, path: string[], unit?: string) => {
    const val = getByPath(config, path);
    if (val === undefined) return null;
    return (
      <div className="flex items-center justify-between">
        <span className="text-xs text-gray-400 flex items-center">
          {t(labelKey)}
          {descKey && <Tooltip text={t(descKey)} />}
        </span>
        <div className="flex items-center gap-1">
          <input
            type="text"
            value={String(val ?? '')}
            onChange={(e) => handleChange(path, e.target.value)}
            className="w-16 text-center bg-gray-800 border border-gray-700 rounded py-1 text-xs font-mono text-gray-100 focus:outline-none focus:border-blue-500"
          />
          {unit && <span className="text-xs text-gray-500">{unit}</span>}
        </div>
      </div>
    );
  };

  const renderPersistTab = () => (
    <div className="space-y-3">
      {/* 1h */}
      <Accordion title={t('cfg.persist.1h')} defaultOpen={true}>
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
          {renderPersistenceField('cfg.field.lookbackMin1h', 'cfg.desc.lookbackMin1h', ['strategy', 'discovery', 'persistence', 'lookback_min_1h'], 'min')}
          {renderPersistenceField('cfg.field.minCount1h', 'cfg.desc.minCount1h', ['strategy', 'discovery', 'persistence', 'min_count_1h'])}
          {renderPersistenceField('cfg.field.stabilityRatio1h', 'cfg.desc.stabilityRatio1h', ['strategy', 'discovery', 'persistence', 'spread_stability_ratio_1h'])}
          {renderPersistenceField('cfg.field.stabilityOIRank1h', 'cfg.desc.stabilityOIRank1h', ['strategy', 'discovery', 'persistence', 'spread_stability_oi_rank_1h'])}
        </div>
      </Accordion>

      {/* 4h */}
      <Accordion title={t('cfg.persist.4h')}>
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
          {renderPersistenceField('cfg.field.lookbackMin4h', 'cfg.desc.lookbackMin4h', ['strategy', 'discovery', 'persistence', 'lookback_min_4h'], 'min')}
          {renderPersistenceField('cfg.field.minCount4h', 'cfg.desc.minCount4h', ['strategy', 'discovery', 'persistence', 'min_count_4h'])}
          {renderPersistenceField('cfg.field.stabilityRatio4h', 'cfg.desc.stabilityRatio4h', ['strategy', 'discovery', 'persistence', 'spread_stability_ratio_4h'])}
          {renderPersistenceField('cfg.field.stabilityOIRank4h', 'cfg.desc.stabilityOIRank4h', ['strategy', 'discovery', 'persistence', 'spread_stability_oi_rank_4h'])}
        </div>
      </Accordion>

      {/* 8h */}
      <Accordion title={t('cfg.persist.8h')}>
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
          {renderPersistenceField('cfg.field.lookbackMin8h', 'cfg.desc.lookbackMin8h', ['strategy', 'discovery', 'persistence', 'lookback_min_8h'], 'min')}
          {renderPersistenceField('cfg.field.minCount8h', 'cfg.desc.minCount8h', ['strategy', 'discovery', 'persistence', 'min_count_8h'])}
          {renderPersistenceField('cfg.field.stabilityRatio8h', 'cfg.desc.stabilityRatio8h', ['strategy', 'discovery', 'persistence', 'spread_stability_ratio_8h'])}
          {renderPersistenceField('cfg.field.stabilityOIRank8h', 'cfg.desc.stabilityOIRank8h', ['strategy', 'discovery', 'persistence', 'spread_stability_oi_rank_8h'])}
        </div>
      </Accordion>

      {/* Volatility */}
      <Accordion title={t('cfg.persist.volatility')}>
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
          <div className="bg-gray-950 rounded-lg border border-gray-800 px-3 py-2">
            <div className="flex items-center gap-2 mb-2">
              <span className="text-xs text-gray-400">{t('cfg.field.enableSpreadStabilityGate')}</span>
              <Tooltip text={t('cfg.desc.enableSpreadStabilityGate')} />
            </div>
            <div className="flex items-center gap-3">
              <ToggleSwitch
                on={getByPath(config, ['strategy', 'discovery', 'persistence', 'enable_spread_stability_gate']) === true}
                onChange={(v) => handleBoolChange(['strategy', 'discovery', 'persistence', 'enable_spread_stability_gate'], v)}
              />
              <span className={`text-sm font-semibold ${getByPath(config, ['strategy', 'discovery', 'persistence', 'enable_spread_stability_gate']) ? 'text-green-400' : 'text-red-400'}`}>
                {getByPath(config, ['strategy', 'discovery', 'persistence', 'enable_spread_stability_gate']) ? 'ON' : 'OFF'}
              </span>
            </div>
          </div>
          {renderPersistenceField('cfg.field.spreadVolatilityMaxCV', 'cfg.desc.spreadVolatilityMaxCV', ['strategy', 'discovery', 'persistence', 'spread_volatility_max_cv'])}
          {renderPersistenceField('cfg.field.spreadVolatilityMinSamples', 'cfg.desc.spreadVolatilityMinSamples', ['strategy', 'discovery', 'persistence', 'spread_volatility_min_samples'])}
          {renderPersistenceField('cfg.field.spreadStabilityAutoCVMultiplier', 'cfg.desc.spreadStabilityAutoCVMultiplier', ['strategy', 'discovery', 'persistence', 'spread_stability_auto_cv_multiplier'])}
          <div className="bg-gray-950 rounded-lg border border-gray-800 px-3 py-2">
            <div className="flex items-center gap-2 mb-2">
              <span className="text-xs text-gray-400">{t('cfg.field.spreadStabilityStricterForAuto')}</span>
              <Tooltip text={t('cfg.desc.spreadStabilityStricterForAuto')} />
            </div>
            <div className="flex items-center gap-3">
              <ToggleSwitch
                on={getByPath(config, ['strategy', 'discovery', 'persistence', 'spread_stability_stricter_for_auto']) === true}
                onChange={(v) => handleBoolChange(['strategy', 'discovery', 'persistence', 'spread_stability_stricter_for_auto'], v)}
              />
              <span className={`text-sm font-semibold ${getByPath(config, ['strategy', 'discovery', 'persistence', 'spread_stability_stricter_for_auto']) ? 'text-green-400' : 'text-red-400'}`}>
                {getByPath(config, ['strategy', 'discovery', 'persistence', 'spread_stability_stricter_for_auto']) ? 'ON' : 'OFF'}
              </span>
            </div>
          </div>
        </div>
      </Accordion>
    </div>
  );

  // =========================================================================
  // Tab: Entry (without order_advance_min)
  // =========================================================================
  const renderEntryTab = () => (
    <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
      <NumberField
        label={t('cfg.field.entryTimeout')}
        desc={t('cfg.desc.entryTimeout')}
        value={getByPath(config, ['strategy', 'entry', 'entry_timeout_sec'])}
        unit="sec"
        onChange={(v) => handleChange(['strategy', 'entry', 'entry_timeout_sec'], v)}
      />
      <NumberField
        label={t('cfg.field.minChunkSize')}
        desc={t('cfg.desc.minChunkSize')}
        value={getByPath(config, ['strategy', 'entry', 'min_chunk_usdt'])}
        unit="USDT"
        onChange={(v) => handleChange(['strategy', 'entry', 'min_chunk_usdt'], v)}
      />
      <NumberField
        label={t('cfg.field.slippageLimit')}
        desc={t('cfg.desc.slippageLimit')}
        value={getByPath(config, ['strategy', 'entry', 'slippage_limit_bps'])}
        unit="bps"
        onChange={(v) => handleChange(['strategy', 'entry', 'slippage_limit_bps'], v)}
      />
      <NumberField
        label={t('cfg.field.lossCooldownHours')}
        desc={t('cfg.desc.lossCooldownHours')}
        value={getByPath(config, ['strategy', 'entry', 'loss_cooldown_hours'])}
        unit="h"
        onChange={(v) => handleChange(['strategy', 'entry', 'loss_cooldown_hours'], v)}
      />
      <NumberField
        label={t('cfg.field.reEnterCooldownHours')}
        desc={t('cfg.desc.reEnterCooldownHours')}
        value={getByPath(config, ['strategy', 'entry', 're_enter_cooldown_hours'])}
        unit="h"
        onChange={(v) => handleChange(['strategy', 'entry', 're_enter_cooldown_hours'], v)}
      />
      <NumberField
        label={t('cfg.field.backtestDays')}
        desc={t('cfg.desc.backtestDays')}
        value={getByPath(config, ['strategy', 'entry', 'backtest_days'])}
        unit="d"
        onChange={(v) => handleChange(['strategy', 'entry', 'backtest_days'], v)}
      />
      <NumberField
        label={t('cfg.field.backtestMinProfit')}
        desc={t('cfg.desc.backtestMinProfit')}
        value={getByPath(config, ['strategy', 'entry', 'backtest_min_profit'])}
        onChange={(v) => handleChange(['strategy', 'entry', 'backtest_min_profit'], v)}
      />
    </div>
  );

  // =========================================================================
  // Tab: Exit & Rotation (without exit_mode)
  // =========================================================================
  const renderExitTab = () => (
    <div className="space-y-4">
      <h3 className="text-sm font-semibold text-gray-400 uppercase tracking-wider">{t('cfg.exit')}</h3>
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
        <NumberField
          label={t('cfg.field.depthExitTimeout')}
          desc={t('cfg.desc.depthExitTimeout')}
          value={getByPath(config, ['strategy', 'exit', 'depth_timeout_sec'])}
          unit="sec"
          onChange={(v) => handleChange(['strategy', 'exit', 'depth_timeout_sec'], v)}
        />
        <div className="bg-gray-900 rounded-xl p-4 border border-gray-800">
          <div className="flex items-center gap-2 mb-2">
            <label className="text-sm font-medium">{t('cfg.field.enableSpreadReversal')}</label>
            <Tooltip text={t('cfg.desc.enableSpreadReversal')} />
          </div>
          <div className="flex items-center gap-3">
            <ToggleSwitch
              on={getByPath(config, ['strategy', 'exit', 'enable_spread_reversal']) === true}
              onChange={(v) => handleBoolChange(['strategy', 'exit', 'enable_spread_reversal'], v)}
            />
            <span className={`text-sm font-semibold ${getByPath(config, ['strategy', 'exit', 'enable_spread_reversal']) ? 'text-green-400' : 'text-red-400'}`}>
              {getByPath(config, ['strategy', 'exit', 'enable_spread_reversal']) ? 'ON' : 'OFF'}
            </span>
          </div>
        </div>
        <NumberField
          label={t('cfg.field.spreadReversalTolerance')}
          desc={t('cfg.desc.spreadReversalTolerance')}
          value={getByPath(config, ['strategy', 'exit', 'spread_reversal_tolerance'])}
          onChange={(v) => handleChange(['strategy', 'exit', 'spread_reversal_tolerance'], v)}
        />
        <div className="bg-gray-900 rounded-xl p-4 border border-gray-800">
          <div className="flex items-center gap-2 mb-2">
            <label className="text-sm font-medium">{t('cfg.field.reversalResetOnRecover')}</label>
            <Tooltip text={t('cfg.desc.reversalResetOnRecover')} />
          </div>
          <div className="flex items-center gap-3">
            <ToggleSwitch
              on={getByPath(config, ['strategy', 'exit', 'reversal_reset_on_recover']) === true}
              onChange={(v) => handleBoolChange(['strategy', 'exit', 'reversal_reset_on_recover'], v)}
            />
            <span className={`text-sm font-semibold ${getByPath(config, ['strategy', 'exit', 'reversal_reset_on_recover']) ? 'text-green-400' : 'text-red-400'}`}>
              {getByPath(config, ['strategy', 'exit', 'reversal_reset_on_recover']) ? 'ON' : 'OFF'}
            </span>
          </div>
        </div>
        <NumberField
          label={t('cfg.field.zeroSpreadTolerance')}
          desc={t('cfg.desc.zeroSpreadTolerance')}
          value={getByPath(config, ['strategy', 'exit', 'zero_spread_tolerance'])}
          onChange={(v) => handleChange(['strategy', 'exit', 'zero_spread_tolerance'], v)}
        />
      </div>

      <h3 className="text-sm font-semibold text-gray-400 uppercase tracking-wider pt-2">{t('cfg.rotation')}</h3>
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
        <NumberField
          label={t('cfg.field.threshold')}
          desc={t('cfg.desc.rotationThreshold')}
          value={getByPath(config, ['strategy', 'rotation', 'threshold_bps'])}
          unit="bps"
          onChange={(v) => handleChange(['strategy', 'rotation', 'threshold_bps'], v)}
        />
        <NumberField
          label={t('cfg.field.cooldown')}
          desc={t('cfg.desc.rotationCooldown')}
          value={getByPath(config, ['strategy', 'rotation', 'cooldown_min'])}
          unit="min"
          onChange={(v) => handleChange(['strategy', 'rotation', 'cooldown_min'], v)}
        />
      </div>
    </div>
  );

  // =========================================================================
  // Tab: Risk
  // =========================================================================
  const renderRiskTab = () => {
    const l3 = (getByPath(config, ['risk', 'margin_l3_threshold']) as number) ?? 0;
    const l4 = (getByPath(config, ['risk', 'margin_l4_threshold']) as number) ?? 0;
    const l5 = (getByPath(config, ['risk', 'margin_l5_threshold']) as number) ?? 0;
    const l4r = (getByPath(config, ['risk', 'l4_reduce_fraction']) as number) ?? 0;
    const liqTrendEnabled = getByPath(config, ['risk', 'enable_liq_trend_tracking']) === true;
    const allocatorEnabled = getByPath(config, ['risk', 'enable_capital_allocator']) === true;
    const exchangeHealthEnabled = getByPath(config, ['risk', 'enable_exchange_health_scoring']) === true;

    return (
      <div className="space-y-4">
        {/* Visual margin bar */}
        <div className="bg-gray-900 rounded-xl p-4 border border-gray-800">
          <h3 className="text-sm font-semibold mb-4">{t('cfg.risk.overview')}</h3>
          <div className="relative h-10 rounded-full overflow-hidden mb-2" style={{ background: 'linear-gradient(to right, #22c55e 0%, #eab308 50%, #ef4444 100%)' }}>
            {/* L3 marker */}
            <div className="absolute top-0 h-full flex flex-col items-center" style={{ left: `${l3 * 100}%` }}>
              <div className="w-0.5 h-full bg-white" />
              <span className="absolute -top-5 text-xs font-bold text-yellow-400 whitespace-nowrap">L3</span>
            </div>
            {/* L4 marker */}
            <div className="absolute top-0 h-full flex flex-col items-center" style={{ left: `${l4 * 100}%` }}>
              <div className="w-0.5 h-full bg-white" />
              <span className="absolute -top-5 text-xs font-bold text-orange-400 whitespace-nowrap">L4</span>
            </div>
            {/* L5 marker */}
            <div className="absolute top-0 h-full flex flex-col items-center" style={{ left: `${l5 * 100}%` }}>
              <div className="w-0.5 h-full bg-white" />
              <span className="absolute -top-5 text-xs font-bold text-red-400 whitespace-nowrap">L5</span>
            </div>
          </div>
          <div className="flex justify-between text-xs text-gray-500 px-1">
            <span>0%</span><span>50%</span><span>100%</span>
          </div>
        </div>

        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
          {/* L3 */}
          <div className="bg-gray-900 rounded-xl p-4 border border-gray-800">
            <div className="flex items-center gap-2 mb-2">
              <label className="text-sm font-medium">{t('cfg.field.l3TransferTrigger')}</label>
              <Tooltip text={t('cfg.desc.l3TransferTrigger')} />
            </div>
            <RangeSlider
              value={l3}
              min={0}
              max={1}
              step={0.01}
              onChange={(v) => handleNumberChange(['risk', 'margin_l3_threshold'], v)}
              colorClass="text-yellow-400"
            />
          </div>

          {/* L4 */}
          <div className="bg-gray-900 rounded-xl p-4 border border-gray-800">
            <div className="flex items-center gap-2 mb-2">
              <label className="text-sm font-medium">{t('cfg.field.l4ReduceTrigger')}</label>
              <Tooltip text={t('cfg.desc.l4ReduceTrigger')} />
            </div>
            <RangeSlider
              value={l4}
              min={0}
              max={1}
              step={0.01}
              onChange={(v) => handleNumberChange(['risk', 'margin_l4_threshold'], v)}
              colorClass="text-orange-400"
            />
          </div>

          {/* L5 */}
          <div className="bg-gray-900 rounded-xl p-4 border border-gray-800">
            <div className="flex items-center gap-2 mb-2">
              <label className="text-sm font-medium">{t('cfg.field.l5EmergencyClose')}</label>
              <Tooltip text={t('cfg.desc.l5EmergencyClose')} />
            </div>
            <RangeSlider
              value={l5}
              min={0}
              max={1}
              step={0.01}
              onChange={(v) => handleNumberChange(['risk', 'margin_l5_threshold'], v)}
              colorClass="text-red-400"
            />
          </div>

          {/* L4 reduce fraction */}
          <div className="bg-gray-900 rounded-xl p-4 border border-gray-800">
            <div className="flex items-center gap-2 mb-2">
              <label className="text-sm font-medium">{t('cfg.field.l4ReduceFraction')}</label>
              <Tooltip text={t('cfg.desc.l4ReduceFraction')} />
            </div>
            <RangeSlider
              value={l4r}
              min={0}
              max={1}
              step={0.01}
              onChange={(v) => handleNumberChange(['risk', 'l4_reduce_fraction'], v)}
              colorClass="text-blue-400"
            />
          </div>

          {/* Margin safety multiplier */}
          <NumberField
            label={t('cfg.field.marginSafetyMultiplier')}
            desc={t('cfg.desc.marginSafetyMultiplier')}
            value={getByPath(config, ['risk', 'margin_safety_multiplier'])}
            unit="×"
            onChange={(v) => handleChange(['risk', 'margin_safety_multiplier'], v)}
          />

          {/* Risk monitor interval */}
          <NumberField
            label={t('cfg.field.riskMonitorInterval')}
            desc={t('cfg.desc.riskMonitorInterval')}
            value={getByPath(config, ['risk', 'risk_monitor_interval_sec'])}
            unit="sec"
            onChange={(v) => handleChange(['risk', 'risk_monitor_interval_sec'], v)}
          />

          <div className="bg-gray-900 rounded-xl p-4 border border-gray-800">
            <div className="flex items-center gap-2 mb-2">
              <label className="text-sm font-medium">{t('cfg.field.enableLiqTrendTracking')}</label>
              <Tooltip text={t('cfg.desc.enableLiqTrendTracking')} />
            </div>
            <div className="flex items-center gap-3">
              <ToggleSwitch
                on={liqTrendEnabled}
                onChange={(v) => handleBoolChange(['risk', 'enable_liq_trend_tracking'], v)}
              />
              <span className={`text-sm font-semibold ${liqTrendEnabled ? 'text-green-400' : 'text-red-400'}`}>
                {liqTrendEnabled ? 'ON' : 'OFF'}
              </span>
            </div>
          </div>

          <NumberField
            label={t('cfg.field.liqProjectionMinutes')}
            desc={t('cfg.desc.liqProjectionMinutes')}
            value={getByPath(config, ['risk', 'liq_projection_minutes'])}
            unit="min"
            onChange={(v) => handleChange(['risk', 'liq_projection_minutes'], v)}
          />

          <NumberField
            label={t('cfg.field.liqWarningSlopeThresh')}
            desc={t('cfg.desc.liqWarningSlopeThresh')}
            value={getByPath(config, ['risk', 'liq_warning_slope_thresh'])}
            unit="/min"
            onChange={(v) => handleChange(['risk', 'liq_warning_slope_thresh'], v)}
          />

          <NumberField
            label={t('cfg.field.liqCriticalSlopeThresh')}
            desc={t('cfg.desc.liqCriticalSlopeThresh')}
            value={getByPath(config, ['risk', 'liq_critical_slope_thresh'])}
            unit="/min"
            onChange={(v) => handleChange(['risk', 'liq_critical_slope_thresh'], v)}
          />

          <NumberField
            label={t('cfg.field.liqMinSamples')}
            desc={t('cfg.desc.liqMinSamples')}
            value={getByPath(config, ['risk', 'liq_min_samples'])}
            unit="samples"
            onChange={(v) => handleChange(['risk', 'liq_min_samples'], v)}
          />

          <div className="bg-gray-900 rounded-xl p-4 border border-gray-800">
            <div className="flex items-center gap-2 mb-2">
              <label className="text-sm font-medium">{t('cfg.field.enableCapitalAllocator')}</label>
              <Tooltip text={t('cfg.desc.enableCapitalAllocator')} />
            </div>
            <div className="flex items-center gap-3">
              <ToggleSwitch
                on={allocatorEnabled}
                onChange={(v) => handleBoolChange(['risk', 'enable_capital_allocator'], v)}
              />
              <span className={`text-sm font-semibold ${allocatorEnabled ? 'text-green-400' : 'text-red-400'}`}>
                {allocatorEnabled ? 'ON' : 'OFF'}
              </span>
            </div>
          </div>

          <div className="bg-gray-900 rounded-xl p-4 border border-gray-800">
            <div className="flex items-center gap-2 mb-2">
              <label className="text-sm font-medium">{t('cfg.field.enableExchangeHealthScoring')}</label>
              <Tooltip text={t('cfg.desc.enableExchangeHealthScoring')} />
            </div>
            <div className="flex items-center gap-3">
              <ToggleSwitch
                on={exchangeHealthEnabled}
                onChange={(v) => handleBoolChange(['risk', 'enable_exchange_health_scoring'], v)}
              />
              <span className={`text-sm font-semibold ${exchangeHealthEnabled ? 'text-green-400' : 'text-red-400'}`}>
                {exchangeHealthEnabled ? 'ON' : 'OFF'}
              </span>
            </div>
          </div>

          <NumberField
            label={t('cfg.field.maxTotalExposureUSDT')}
            desc={t('cfg.desc.maxTotalExposureUSDT')}
            value={getByPath(config, ['risk', 'max_total_exposure_usdt'])}
            unit="USDT"
            onChange={(v) => handleChange(['risk', 'max_total_exposure_usdt'], v)}
          />

          <NumberField
            label={t('cfg.field.maxPerpPerpPct')}
            desc={t('cfg.desc.maxPerpPerpPct')}
            value={getByPath(config, ['risk', 'max_perp_perp_pct'])}
            onChange={(v) => handleChange(['risk', 'max_perp_perp_pct'], v)}
          />

          <NumberField
            label={t('cfg.field.maxSpotFuturesPct')}
            desc={t('cfg.desc.maxSpotFuturesPct')}
            value={getByPath(config, ['risk', 'max_spot_futures_pct'])}
            onChange={(v) => handleChange(['risk', 'max_spot_futures_pct'], v)}
          />

          <NumberField
            label={t('cfg.field.maxPerExchangePct')}
            desc={t('cfg.desc.maxPerExchangePct')}
            value={getByPath(config, ['risk', 'max_per_exchange_pct'])}
            onChange={(v) => handleChange(['risk', 'max_per_exchange_pct'], v)}
          />

          <NumberField
            label={t('cfg.field.reservationTTLSec')}
            desc={t('cfg.desc.reservationTTLSec')}
            value={getByPath(config, ['risk', 'reservation_ttl_sec'])}
            unit="sec"
            onChange={(v) => handleChange(['risk', 'reservation_ttl_sec'], v)}
          />
        </div>
      </div>
    );
  };

  // =========================================================================
  // Tab: Spot-Futures General
  // =========================================================================
  const renderSfGeneralTab = () => {
    const sfEnabled = getByPath(config, ['spot_futures', 'enabled']) === true;
    const sfAutoEnabled = getByPath(config, ['spot_futures', 'auto_enabled']) === true;
    const sfDryRun = getByPath(config, ['spot_futures', 'auto_dry_run']) === true;
    const sfExchanges = (getByPath(config, ['spot_futures', 'exchanges']) as string[] | undefined) || [];

    return (
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
        {/* Engine enabled toggle */}
        <div className="bg-gray-900 rounded-xl p-4 border border-gray-800">
          <div className="flex items-center gap-2 mb-2">
            <label className="text-sm font-medium">{t('cfg.sf.enabled')}</label>
            <Tooltip text={t('cfg.sf.enabledDesc')} />
          </div>
          <div className="flex items-center gap-3">
            <ToggleSwitch
              on={sfEnabled}
              onChange={(v) => handleBoolChange(['spot_futures', 'enabled'], v)}
            />
            <span className={`text-sm font-semibold ${sfEnabled ? 'text-green-400' : 'text-red-400'}`}>
              {sfEnabled ? 'ON' : 'OFF'}
            </span>
          </div>
        </div>

        {/* Auto entry toggle */}
        <div className="bg-gray-900 rounded-xl p-4 border border-gray-800">
          <div className="flex items-center gap-2 mb-2">
            <label className="text-sm font-medium">{t('cfg.sf.autoEnabled')}</label>
            <Tooltip text={t('cfg.sf.autoEnabledDesc')} />
          </div>
          <div className="flex items-center gap-3">
            <ToggleSwitch
              on={sfAutoEnabled}
              onChange={(v) => handleBoolChange(['spot_futures', 'auto_enabled'], v)}
            />
            <span className={`text-sm font-semibold ${sfAutoEnabled ? 'text-green-400' : 'text-red-400'}`}>
              {sfAutoEnabled ? 'ON' : 'OFF'}
            </span>
          </div>
        </div>

        {/* Dry run toggle */}
        <div className="bg-gray-900 rounded-xl p-4 border border-gray-800">
          <div className="flex items-center gap-2 mb-2">
            <label className="text-sm font-medium">{t('cfg.sf.autoDryRun')}</label>
            <Tooltip text={t('cfg.sf.autoDryRunDesc')} />
          </div>
          <div className="flex items-center gap-3">
            <ToggleSwitch
              on={sfDryRun}
              onChange={(v) => handleBoolChange(['spot_futures', 'auto_dry_run'], v)}
            />
            <span className={`text-sm font-semibold ${sfDryRun ? 'text-green-400' : 'text-red-400'}`}>
              {sfDryRun ? 'ON' : 'OFF'}
            </span>
          </div>
        </div>

        {/* Leverage dropdown */}
        <div className="bg-gray-900 rounded-xl p-4 border border-gray-800">
          <div className="flex items-center gap-2 mb-2">
            <label className="text-sm font-medium">{t('cfg.sf.leverage')}</label>
            <Tooltip text={t('cfg.sf.leverageDesc')} />
          </div>
          <select
            value={String(getByPath(config, ['spot_futures', 'leverage']) ?? 3)}
            onChange={(e) => handleChange(['spot_futures', 'leverage'], e.target.value)}
            className="w-full bg-gray-800 border border-gray-700 rounded-lg py-2 px-3 text-sm text-gray-100 focus:outline-none focus:border-blue-500"
          >
            {SF_LEVERAGE_OPTIONS.map((lev) => (
              <option key={lev} value={lev}>{lev}x</option>
            ))}
          </select>
        </div>

        {/* Exchange allowlist */}
        <div className="bg-gray-900 rounded-xl p-4 border border-gray-800 sm:col-span-2">
          <div className="flex items-center gap-2 mb-2">
            <label className="text-sm font-medium">{t('cfg.sf.exchanges')}</label>
            <Tooltip text={t('cfg.sf.exchangesDesc')} />
          </div>
          <input
            type="text"
            value={sfExchanges.join(', ')}
            onChange={(e) => {
              const arr = e.target.value.split(',').map((s) => s.trim()).filter(Boolean);
              setDirty(true);
              setDirtyPaths((prev) => new Set(prev).add('spot_futures'));
              setConfig((prev) => setByPath(prev, ['spot_futures', 'exchanges'], arr));
            }}
            placeholder="binance, bybit, gateio, bitget, okx"
            className="w-full bg-gray-800 border border-gray-700 rounded-lg py-1.5 px-3 text-sm font-mono text-gray-100 focus:outline-none focus:border-blue-500"
          />
        </div>
      </div>
    );
  };

  // =========================================================================
  // Tab: Spot-Futures Sizing
  // =========================================================================
  const renderSfSizingTab = () => (
    <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
      <NumberField
        label={t('cfg.sf.maxPositions')}
        desc={t('cfg.sf.maxPositionsDesc')}
        value={getByPath(config, ['spot_futures', 'max_positions'])}
        onChange={(v) => handleChange(['spot_futures', 'max_positions'], v)}
      />
      <NumberField
        label={t('cfg.sf.capitalPerPosition')}
        desc={t('cfg.sf.capitalPerPositionDesc')}
        value={getByPath(config, ['spot_futures', 'capital_per_position'])}
        unit="USDT"
        onChange={(v) => handleChange(['spot_futures', 'capital_per_position'], v)}
      />
      <NumberField
        label={t('cfg.sf.separateAcct')}
        desc={t('cfg.sf.separateAcctDesc')}
        value={getByPath(config, ['spot_futures', 'separate_acct_max_usdt'])}
        unit="USDT"
        onChange={(v) => handleChange(['spot_futures', 'separate_acct_max_usdt'], v)}
      />
      <NumberField
        label={t('cfg.sf.unifiedAcct')}
        desc={t('cfg.sf.unifiedAcctDesc')}
        value={getByPath(config, ['spot_futures', 'unified_acct_max_usdt'])}
        unit="USDT"
        onChange={(v) => handleChange(['spot_futures', 'unified_acct_max_usdt'], v)}
      />
    </div>
  );

  // =========================================================================
  // Tab: Spot-Futures Discovery
  // =========================================================================
  const renderSfDiscoveryTab = () => (
    <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
      <NumberField
        label={t('cfg.sf.scanInterval')}
        desc={t('cfg.sf.scanIntervalDesc')}
        value={getByPath(config, ['spot_futures', 'scan_interval_min'])}
        unit="min"
        onChange={(v) => handleChange(['spot_futures', 'scan_interval_min'], v)}
      />
      <NumberField
        label={t('cfg.sf.persistenceScans')}
        desc={t('cfg.sf.persistenceScansDesc')}
        value={getByPath(config, ['spot_futures', 'persistence_scans'])}
        onChange={(v) => handleChange(['spot_futures', 'persistence_scans'], v)}
      />
      <NumberField
        label={t('cfg.sf.minNetYield')}
        desc={t('cfg.sf.minNetYieldDesc')}
        value={getByPath(config, ['spot_futures', 'min_net_yield_apr'])}
        onChange={(v) => handleChange(['spot_futures', 'min_net_yield_apr'], v)}
      />
      <NumberField
        label={t('cfg.sf.maxBorrowApr')}
        desc={t('cfg.sf.maxBorrowAprDesc')}
        value={getByPath(config, ['spot_futures', 'max_borrow_apr'])}
        onChange={(v) => handleChange(['spot_futures', 'max_borrow_apr'], v)}
      />
    </div>
  );

  // =========================================================================
  // Tab: Spot-Futures Exit & Risk
  // =========================================================================
  const renderSfExitTab = () => {
    const sfProfitTransfer = getByPath(config, ['spot_futures', 'profit_transfer_enabled']) === true;
    const sfBorrowSpikeEnabled = getByPath(config, ['spot_futures', 'enable_borrow_spike_detection']) === true;

    return (
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
        <NumberField
          label={t('cfg.sf.monitorInterval')}
          desc={t('cfg.sf.monitorIntervalDesc')}
          value={getByPath(config, ['spot_futures', 'monitor_interval_sec'])}
          unit="sec"
          onChange={(v) => handleChange(['spot_futures', 'monitor_interval_sec'], v)}
        />
        <NumberField
          label={t('cfg.sf.borrowGrace')}
          desc={t('cfg.sf.borrowGraceDesc')}
          value={getByPath(config, ['spot_futures', 'borrow_grace_min'])}
          unit="min"
          onChange={(v) => handleChange(['spot_futures', 'borrow_grace_min'], v)}
        />
        <div className="bg-gray-900 rounded-xl p-4 border border-gray-800">
          <div className="flex items-center gap-2 mb-2">
            <label className="text-sm font-medium">{t('cfg.sf.borrowSpikeEnabled')}</label>
            <Tooltip text={t('cfg.sf.borrowSpikeEnabledDesc')} />
          </div>
          <div className="flex items-center gap-3">
            <ToggleSwitch
              on={sfBorrowSpikeEnabled}
              onChange={(v) => handleBoolChange(['spot_futures', 'enable_borrow_spike_detection'], v)}
            />
            <span className={`text-sm font-semibold ${sfBorrowSpikeEnabled ? 'text-green-400' : 'text-red-400'}`}>
              {sfBorrowSpikeEnabled ? 'ON' : 'OFF'}
            </span>
          </div>
        </div>
        <NumberField
          label={t('cfg.sf.borrowSpikeWindow')}
          desc={t('cfg.sf.borrowSpikeWindowDesc')}
          value={getByPath(config, ['spot_futures', 'borrow_spike_window_min'])}
          unit="min"
          onChange={(v) => handleChange(['spot_futures', 'borrow_spike_window_min'], v)}
        />
        <NumberField
          label={t('cfg.sf.borrowSpikeMultiplier')}
          desc={t('cfg.sf.borrowSpikeMultiplierDesc')}
          value={getByPath(config, ['spot_futures', 'borrow_spike_multiplier'])}
          onChange={(v) => handleChange(['spot_futures', 'borrow_spike_multiplier'], v)}
        />
        <NumberField
          label={t('cfg.sf.borrowSpikeMinAbsolute')}
          desc={t('cfg.sf.borrowSpikeMinAbsoluteDesc')}
          value={getByPath(config, ['spot_futures', 'borrow_spike_min_absolute'])}
          onChange={(v) => handleChange(['spot_futures', 'borrow_spike_min_absolute'], v)}
        />
        <NumberField
          label={t('cfg.sf.priceExit')}
          desc={t('cfg.sf.priceExitDesc')}
          value={getByPath(config, ['spot_futures', 'price_exit_pct'])}
          unit="%"
          onChange={(v) => handleChange(['spot_futures', 'price_exit_pct'], v)}
        />
        <NumberField
          label={t('cfg.sf.priceEmergency')}
          desc={t('cfg.sf.priceEmergencyDesc')}
          value={getByPath(config, ['spot_futures', 'price_emergency_pct'])}
          unit="%"
          onChange={(v) => handleChange(['spot_futures', 'price_emergency_pct'], v)}
        />
        <NumberField
          label={t('cfg.sf.marginExit')}
          desc={t('cfg.sf.marginExitDesc')}
          value={getByPath(config, ['spot_futures', 'margin_exit_pct'])}
          unit="%"
          onChange={(v) => handleChange(['spot_futures', 'margin_exit_pct'], v)}
        />
        <NumberField
          label={t('cfg.sf.marginEmergency')}
          desc={t('cfg.sf.marginEmergencyDesc')}
          value={getByPath(config, ['spot_futures', 'margin_emergency_pct'])}
          unit="%"
          onChange={(v) => handleChange(['spot_futures', 'margin_emergency_pct'], v)}
        />
        <NumberField
          label={t('cfg.sf.lossCooldown')}
          desc={t('cfg.sf.lossCooldownDesc')}
          value={getByPath(config, ['spot_futures', 'loss_cooldown_hours'])}
          unit="h"
          onChange={(v) => handleChange(['spot_futures', 'loss_cooldown_hours'], v)}
        />

        {/* Profit transfer toggle */}
        <div className="bg-gray-900 rounded-xl p-4 border border-gray-800">
          <div className="flex items-center gap-2 mb-2">
            <label className="text-sm font-medium">{t('cfg.sf.profitTransfer')}</label>
            <Tooltip text={t('cfg.sf.profitTransferDesc')} />
          </div>
          <div className="flex items-center gap-3">
            <ToggleSwitch
              on={sfProfitTransfer}
              onChange={(v) => handleBoolChange(['spot_futures', 'profit_transfer_enabled'], v)}
            />
            <span className={`text-sm font-semibold ${sfProfitTransfer ? 'text-green-400' : 'text-red-400'}`}>
              {sfProfitTransfer ? 'ON' : 'OFF'}
            </span>
          </div>
        </div>
      </div>
    );
  };

  // =========================================================================
  // Render active tab content
  // =========================================================================
  const renderTabContent = () => {
    if (strategy === 'exchanges') return renderExchangesTab();
    switch (activeTab) {
      case 'fund': return renderFundTab();
      case 'schedule': return renderScheduleTab();
      case 'discovery': return renderDiscoveryTab();
      case 'persist': return renderPersistTab();
      case 'entry': return renderEntryTab();
      case 'exit': return renderExitTab();
      case 'risk': return renderRiskTab();
      case 'sf-general': return renderSfGeneralTab();
      case 'sf-sizing': return renderSfSizingTab();
      case 'sf-discovery': return renderSfDiscoveryTab();
      case 'sf-exit': return renderSfExitTab();
      default: return null;
    }
  };

  return (
    <div className="pb-20">
      {/* Title */}
      <h2 className="text-xl font-bold text-gray-100 mb-4">{t('cfg.title')}</h2>

      {/* Strategy toggle */}
      <div className="flex bg-gray-900 border border-gray-700 rounded-lg p-0.5 gap-0.5 mb-4 w-fit">
        <button
          type="button"
          onClick={() => setStrategy('exchanges')}
          className={`px-4 py-1.5 text-xs font-semibold rounded-md transition-all duration-150 ${
            strategy === 'exchanges'
              ? 'bg-gray-700 text-gray-100 shadow-sm'
              : 'text-gray-500 hover:text-gray-300'
          }`}
        >
          {t('cfg.tab.exchanges')}
        </button>
        <button
          type="button"
          onClick={() => { setStrategy('perp'); setActiveTab('fund'); }}
          className={`px-4 py-1.5 text-xs font-semibold rounded-md transition-all duration-150 ${
            strategy === 'perp'
              ? 'bg-gray-700 text-gray-100 shadow-sm'
              : 'text-gray-500 hover:text-gray-300'
          }`}
        >
          {t('cfg.strategyPerp')}
        </button>
        <button
          type="button"
          onClick={() => { setStrategy('spot'); setActiveTab('sf-general'); }}
          className={`px-4 py-1.5 text-xs font-semibold rounded-md transition-all duration-150 ${
            strategy === 'spot'
              ? 'bg-gray-700 text-gray-100 shadow-sm'
              : 'text-gray-500 hover:text-gray-300'
          }`}
        >
          {t('cfg.strategySpot')}
        </button>
      </div>

      {/* Tab bar (hidden for exchanges — no sub-tabs) */}
      {strategy !== 'exchanges' && <div
        ref={tabBarRef}
        className="flex gap-1 overflow-x-auto pb-3 mb-4 scrollbar-none"
        style={{ scrollbarWidth: 'none', WebkitOverflowScrolling: 'touch' }}
      >
        {(strategy === 'perp' ? PERP_TABS : SPOT_TABS).map((tab) => (
          <button
            key={tab.id}
            type="button"
            onClick={() => setActiveTab(tab.id)}
            className={`whitespace-nowrap px-3 py-1.5 rounded-full text-sm font-medium transition shrink-0 ${
              activeTab === tab.id
                ? 'bg-blue-600 text-white'
                : 'text-gray-400 hover:text-gray-200 hover:bg-gray-800'
            }`}
          >
            {t(tab.labelKey)}
          </button>
        ))}
      </div>}

      {/* Tab content */}
      <form onSubmit={handleSubmit}>
        {renderTabContent()}
      </form>

      {/* Sticky bottom save bar */}
      <div className="fixed bottom-0 left-0 right-0 bg-gray-900/95 backdrop-blur border-t border-gray-800 z-50">
        <div className="max-w-5xl mx-auto px-4 py-3 flex items-center justify-between">
          <span className="text-xs text-gray-500">
            {message ? (
              <span className={message === t('cfg.saved') ? 'text-green-400' : 'text-red-400'}>{message}</span>
            ) : dirty ? (
              <span className="text-yellow-400">{t('cfg.unsavedChanges')}</span>
            ) : null}
          </span>
          <button
            type="button"
            onClick={handleSubmit as unknown as () => void}
            disabled={saving}
            className="px-6 py-2.5 bg-blue-600 hover:bg-blue-500 active:bg-blue-700 text-white font-semibold rounded-xl text-sm transition disabled:opacity-50"
          >
            {saving ? t('cfg.saving') : t('cfg.save')}
          </button>
        </div>
      </div>
    </div>
  );
};

export default Config;
