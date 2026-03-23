import { useState, useEffect, useRef, type FC } from 'react';
import type { LogEntry } from '../types.ts';
import { useLocale } from '../i18n/index.ts';

interface LogsProps {
  logs: LogEntry[];
  connected: boolean;
  getLogs: (limit?: number) => Promise<LogEntry[]>;
  setLogs: React.Dispatch<React.SetStateAction<LogEntry[]>>;
}

type LevelFilter = 'ALL' | 'INFO' | 'WARN' | 'ERROR' | 'DEBUG';

const levelColors: Record<string, string> = {
  INFO: 'bg-blue-500/20 text-blue-400',
  WARN: 'bg-yellow-500/20 text-yellow-400',
  ERROR: 'bg-red-500/20 text-red-400',
  DEBUG: 'bg-gray-500/20 text-gray-400',
  RAW: 'bg-gray-500/10 text-gray-500',
};

const Logs: FC<LogsProps> = ({ logs, connected, getLogs, setLogs }) => {
  const { t } = useLocale();
  const [levelFilter, setLevelFilter] = useState<LevelFilter>('ALL');
  const [moduleFilter, setModuleFilter] = useState('');
  const [autoScroll, setAutoScroll] = useState(true);
  const scrollRef = useRef<HTMLDivElement>(null);
  const prevConnected = useRef(connected);
  const seeded = useRef(false);

  // Seed historical logs on mount
  useEffect(() => {
    if (seeded.current) return;
    seeded.current = true;
    getLogs(500).then((entries) => {
      setLogs(entries);
    }).catch(() => {});
  }, [getLogs, setLogs]);

  // Reconnect backfill: when connected transitions false→true, re-fetch
  useEffect(() => {
    if (!prevConnected.current && connected) {
      getLogs(500).then((fresh) => {
        setLogs((prev) => {
          // Dedup by timestamp+message, keep fresh + any WS-only entries
          const seen = new Set(fresh.map((e) => `${e.timestamp}|${e.message}`));
          const extra = prev.filter((e) => !seen.has(`${e.timestamp}|${e.message}`));
          const merged = [...fresh, ...extra].slice(-500);
          return merged;
        });
      }).catch(() => {});
    }
    prevConnected.current = connected;
  }, [connected, getLogs, setLogs]);

  // Auto-scroll to bottom when new logs arrive
  useEffect(() => {
    if (autoScroll && scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [logs, autoScroll]);

  const filtered = logs.filter((entry) => {
    if (levelFilter !== 'ALL' && entry.level !== levelFilter) return false;
    if (moduleFilter && !entry.module.toLowerCase().includes(moduleFilter.toLowerCase())) return false;
    return true;
  });

  const levels: LevelFilter[] = ['ALL', 'INFO', 'WARN', 'ERROR', 'DEBUG'];

  return (
    <div className="space-y-4">
      <h2 className="text-xl font-bold text-gray-100">{t('logs.title')}</h2>

      {/* Filters */}
      <div className="flex flex-wrap items-center gap-3">
        <div className="flex gap-1">
          {levels.map((level) => (
            <button
              key={level}
              onClick={() => setLevelFilter(level)}
              className={`px-3 py-1 text-xs rounded transition-colors ${
                levelFilter === level
                  ? 'bg-blue-500/20 text-blue-400'
                  : 'bg-gray-800 text-gray-400 hover:text-gray-200'
              }`}
            >
              {t(`logs.${level.toLowerCase()}` as 'logs.all')}
            </button>
          ))}
        </div>
        <input
          type="text"
          value={moduleFilter}
          onChange={(e) => setModuleFilter(e.target.value)}
          placeholder={t('logs.filter')}
          className="px-3 py-1 text-sm bg-gray-800 border border-gray-700 rounded text-gray-100 placeholder-gray-500 w-40"
        />
        <label className="flex items-center gap-1.5 text-sm text-gray-400 cursor-pointer ml-auto">
          <input
            type="checkbox"
            checked={autoScroll}
            onChange={(e) => setAutoScroll(e.target.checked)}
            className="rounded"
          />
          {t('logs.autoScroll')}
        </label>
        <button
          onClick={() => setLogs([])}
          className="px-3 py-1 text-xs bg-gray-800 text-gray-400 hover:text-gray-200 rounded transition-colors"
        >
          {t('logs.clear')}
        </button>
      </div>

      {/* Log entries */}
      <div
        ref={scrollRef}
        className="bg-gray-900 border border-gray-800 rounded-lg p-4 h-[calc(100vh-200px)] overflow-y-auto font-mono text-xs leading-5"
      >
        {filtered.length === 0 ? (
          <p className="text-gray-500 text-sm">{t('logs.noLogs')}</p>
        ) : (
          filtered.map((entry, i) => (
            <div key={i} className="flex gap-2 hover:bg-gray-800/50 px-1 rounded">
              <span className="text-gray-500 whitespace-nowrap shrink-0">
                {entry.timestamp || '--'}
              </span>
              <span
                className={`px-1.5 rounded text-center w-12 shrink-0 ${
                  levelColors[entry.level] || levelColors.RAW
                }`}
              >
                {entry.level}
              </span>
              {entry.module && (
                <span className="text-purple-400 shrink-0">[{entry.module}]</span>
              )}
              <span className="text-gray-200 break-all">{entry.message}</span>
            </div>
          ))
        )}
      </div>
    </div>
  );
};

export default Logs;
