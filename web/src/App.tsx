import { useState, useEffect, useCallback } from 'react';
import { useApi } from './hooks/useApi.ts';
import { useWebSocket } from './hooks/useWebSocket.ts';
import Sidebar, { navItems } from './components/Sidebar.tsx';
import Login from './pages/Login.tsx';
import Overview from './pages/Overview.tsx';
import Opportunities from './pages/Opportunities.tsx';
import Positions from './pages/Positions.tsx';
import History from './pages/History.tsx';
import Config from './pages/Config.tsx';
import Transfers from './pages/Transfers.tsx';
import Logs from './pages/Logs.tsx';
import Rejections from './pages/Rejections.tsx';
import Permissions from './pages/Permissions.tsx';
import Analytics from './pages/Analytics.tsx';
import SpotPositions from './pages/SpotPositions.tsx';
import PriceGap from './pages/PriceGap.tsx';
import { LocaleContext, getStoredLocale, storeLocale, t as translate, type Locale } from './i18n/index.ts';
import { ThemeContext, getStoredTheme, storeTheme, type Theme } from './theme/index.ts';
import type { ExchangeInfo } from './types.ts';

type Page = 'overview' | 'opportunities' | 'positions' | 'spot-positions' | 'price-gap' | 'history' | 'analytics' | 'config' | 'transfers' | 'logs' | 'rejections' | 'permissions';

const UPDATE_DISMISS_KEY = 'arb_update_dismissed';

function readSpotScannerMode(config: Record<string, unknown>): string | undefined {
  const spotFutures = config.spot_futures;
  if (!spotFutures || typeof spotFutures !== 'object') return undefined;
  const scannerMode = (spotFutures as Record<string, unknown>).scanner_mode;
  return typeof scannerMode === 'string' ? scannerMode : undefined;
}

function App() {
  const api = useApi();
  const [page, setPage] = useState<Page>('overview');
  const [exchanges, setExchanges] = useState<ExchangeInfo[]>([]);
  const [blacklist, setBlacklist] = useState<string[]>([]);
  const [spotScannerMode, setSpotScannerMode] = useState('native');
  const [locale, setLocaleState] = useState<Locale>(getStoredLocale);
  const [theme, setThemeState] = useState<Theme>(getStoredTheme);
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const ws = useWebSocket(!!api.token);

  // TradFi signing state
  const [tradfiUnsigned, setTradfiUnsigned] = useState(false);

  // Update state
  const [updateInfo, setUpdateInfo] = useState<{ latestVersion: string; changelog: string } | null>(null);
  const [showUpdateModal, setShowUpdateModal] = useState(false);
  const [updateOutput, setUpdateOutput] = useState('');
  const [updating, setUpdating] = useState(false);

  const setLocale = useCallback((l: Locale) => {
    setLocaleState(l);
    storeLocale(l);
  }, []);

  const setTheme = useCallback((t: Theme) => {
    setThemeState(t);
    storeTheme(t);
  }, []);

  // Sync data-theme attribute on <html> whenever theme changes
  useEffect(() => {
    document.documentElement.dataset.theme = theme;
  }, [theme]);

  const t = useCallback((key: Parameters<typeof translate>[0]) => translate(key, locale), [locale]);

  const handleLogin = useCallback(async (password: string) => {
    await api.login(password);
  }, [api]);

  const handleLogout = useCallback(() => {
    api.logout();
  }, [api]);

  // Silent update check — on login and every 30 minutes.
  const silentCheckUpdate = useCallback(async () => {
    try {
      const data = await api.checkUpdate();
      if (data.hasUpdate) {
        // Check if user dismissed this version in the last 24h.
        const dismissed = localStorage.getItem(UPDATE_DISMISS_KEY);
        if (dismissed) {
          const { version, time } = JSON.parse(dismissed);
          if (version === data.latestVersion && Date.now() - time < 24 * 60 * 60 * 1000) {
            return;
          }
        }
        setUpdateInfo({ latestVersion: data.latestVersion, changelog: data.changelog });
      } else {
        setUpdateInfo(null);
      }
    } catch { /* ignore */ }
  }, [api]);

  const dismissUpdate = useCallback(() => {
    if (updateInfo) {
      localStorage.setItem(UPDATE_DISMISS_KEY, JSON.stringify({ version: updateInfo.latestVersion, time: Date.now() }));
    }
    setUpdateInfo(null);
  }, [updateInfo]);

  const handleUpdate = useCallback(async () => {
    setUpdating(true);
    setUpdateOutput('');
    try {
      const data = await api.performUpdate();
      setUpdateOutput(data.output);
      // Server will restart — wait and reload.
      setTimeout(() => window.location.reload(), 5000);
    } catch (err) {
      setUpdateOutput(t('update.failed') + ': ' + (err instanceof Error ? err.message : String(err)));
    } finally {
      setUpdating(false);
    }
  }, [api, t]);

  const handleSignTradFi = useCallback(async () => {
    try {
      await api.signTradFi();
      setTradfiUnsigned(false);
    } catch (err) {
      alert(t('tradfi.error') + ': ' + (err instanceof Error ? err.message : String(err)));
    }
  }, [api]);

  const handleBlacklistToggle = useCallback(async (symbol: string) => {
    try {
      if (blacklist.includes(symbol)) {
        await api.removeFromBlacklist(symbol);
        setBlacklist(prev => prev.filter(s => s !== symbol));
      } else {
        await api.addToBlacklist(symbol);
        setBlacklist(prev => [...prev, symbol]);
      }
    } catch { /* ignore */ }
  }, [api, blacklist]);

  const handleUpdateConfig = useCallback(async (data: Record<string, unknown>) => {
    const updated = await api.updateConfig(data);
    setSpotScannerMode(readSpotScannerMode(updated) ?? readSpotScannerMode(data) ?? 'native');
    return updated;
  }, [api]);

  // Seed WS state from REST on initial load, and refresh exchanges periodically.
  useEffect(() => {
    if (!api.token) return;
    api.getPositions().then(ws.setPositions).catch(() => {});
    api.getOpportunities().then(ws.setOpportunities).catch(() => {});
    api.getStats().then(ws.setStats).catch(() => {});
    api.getSpotPositions().then(ws.setSpotPositions).catch(() => {});
    api.getSpotOpportunities().then(ws.setSpotOpportunities).catch(() => {});
    api.getConfig().then((config) => {
      setSpotScannerMode(readSpotScannerMode(config) ?? 'native');
    }).catch(() => {});
    api.getBlacklist().then(setBlacklist).catch(() => {});
    const loadExchanges = () => {
      api.getExchanges().then(setExchanges).catch(() => {});
    };
    loadExchanges();
    const interval = setInterval(loadExchanges, 60000);

    // Check TradFi signing status.
    api.getTradFiStatus().then(d => {
      if (!d.signed) {
        const dismissed = sessionStorage.getItem('arb_tradfi_dismissed');
        if (!dismissed) setTradfiUnsigned(true);
      }
    }).catch(() => {});

    // Check for updates on login and every 30 minutes.
    silentCheckUpdate();
    const updateInterval = setInterval(silentCheckUpdate, 30 * 60 * 1000);

    return () => {
      clearInterval(interval);
      clearInterval(updateInterval);
    };
  }, [api.token]); // eslint-disable-line react-hooks/exhaustive-deps

  if (!api.token) {
    return (
      <ThemeContext.Provider value={{ theme, setTheme }}>
        <LocaleContext.Provider value={{ locale, setLocale, t }}>
          <Login onLogin={handleLogin} />
        </LocaleContext.Provider>
      </ThemeContext.Provider>
    );
  }

  // Get current page label for mobile header
  const currentNavItem = navItems.find((item) => item.id === page);
  const pageTitle = currentNavItem ? t(currentNavItem.labelKey) : '';

  const renderPage = () => {
    switch (page) {
      case 'overview':
        return (
          <Overview
            positions={ws.positions}
            stats={ws.stats}
            exchanges={exchanges}
            onDiagnose={api.diagnose}
            onResolveSpotPosition={api.spotManualClose}
            spotPositions={ws.spotPositions}
            lossLimits={ws.lossLimits}
          />
        );
      case 'opportunities':
        return (
          <Opportunities
            opportunities={ws.opportunities}
            spotOpportunities={ws.spotOpportunities}
            spotScannerMode={spotScannerMode}
            onOpen={api.openPosition}
            onSpotOpen={api.spotManualOpen}
            onCheckPriceGap={api.checkSpotPriceGap}
            onBatchCheckGap={api.batchCheckGap}
            onBatchCheckBorrowable={api.batchCheckBorrowable}
            blacklist={blacklist}
            onBlacklistToggle={handleBlacklistToggle}
          />
        );
      case 'positions':
        return (
          <Positions
            positions={ws.positions}
            onClose={api.closePosition}
            onFetchFunding={api.getPositionFunding}
            blacklist={blacklist}
            onBlacklistToggle={handleBlacklistToggle}
          />
        );
      case 'spot-positions':
        return (
          <SpotPositions
            positions={ws.spotPositions}
            onClose={api.spotManualClose}
            getStats={api.getSpotStats}
            getHistory={api.getSpotHistory}
          />
        );
      case 'price-gap':
        return <PriceGap />;
      case 'history':
        return <History getHistory={api.getHistory} />;
      case 'analytics':
        return <Analytics getAnalyticsPnL={api.getAnalyticsPnL} getAnalyticsSummary={api.getAnalyticsSummary} />;
      case 'config':
        return <Config getConfig={api.getConfig} updateConfig={handleUpdateConfig} blacklist={blacklist} onBlacklistRemove={async (s) => { await api.removeFromBlacklist(s); setBlacklist(prev => prev.filter(x => x !== s)); }} />;
      case 'transfers':
        return (
          <Transfers
            transfer={api.transfer}
            getTransfers={api.getTransfers}
            getAddresses={api.getAddresses}
            updateAddresses={api.updateAddresses}
          />
        );
      case 'logs':
        return (
          <Logs
            logs={ws.logs}
            connected={ws.connected}
            getLogs={api.getLogs}
            setLogs={ws.setLogs}
          />
        );
      case 'permissions':
        return <Permissions getPermissions={api.getPermissions} />;
      case 'rejections':
        return (
          <Rejections
            rejections={ws.rejections}
            getRejections={api.getRejections}
            setRejections={ws.setRejections}
          />
        );
    }
  };

  // ── New design shell ──────────────────────────────────────────────────────
  const renderMobileHeader = () => {
    if (theme === 'classic') {
      return (
        <div className="flex md:hidden items-center gap-3 px-4 py-3 bg-gray-900 border-b border-gray-800 sticky top-0 z-40">
          <button
            onClick={() => setSidebarOpen(true)}
            className="text-gray-300 hover:text-gray-100 p-1"
            aria-label="Open menu"
          >
            <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 6h16M4 12h16M4 18h16" />
            </svg>
          </button>
          <h1 className="text-base font-semibold text-gray-100">{pageTitle}</h1>
        </div>
      );
    }
    return (
      <div className="flex md:hidden items-center gap-3 px-4 py-3 bg-[#1e2026] border-b border-[#2b2f36] sticky top-0 z-40">
        <button
          onClick={() => setSidebarOpen(true)}
          className="text-gray-300 hover:text-[#f0b90b] p-1 transition-colors"
          aria-label="Open menu"
        >
          <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 6h16M4 12h16M4 18h16" />
          </svg>
        </button>
        <span aria-hidden className="inline-block w-5 h-5 rotate-45 bg-[#f0b90b] rounded-[2px]" />
        <h1 className="text-sm font-bold uppercase tracking-wide text-gray-100">{pageTitle}</h1>
      </div>
    );
  };

  const renderUpdateBanner = () => {
    if (!updateInfo) return null;
    if (theme === 'classic') {
      return (
        <div className="bg-blue-600 text-white px-4 py-2 flex items-center justify-between text-sm">
          <button
            onClick={() => setShowUpdateModal(true)}
            className="hover:underline"
          >
            {t('update.available').replace('{version}', updateInfo.latestVersion)}
          </button>
          <button
            onClick={dismissUpdate}
            className="ml-4 text-white/70 hover:text-white"
          >
            ✕
          </button>
        </div>
      );
    }
    return (
      <div className="bg-[#f0b90b] text-[#0b0e11] px-4 py-2 flex items-center justify-between text-sm font-semibold">
        <button
          onClick={() => setShowUpdateModal(true)}
          className="hover:underline decoration-2 underline-offset-2 text-left"
        >
          {t('update.available').replace('{version}', updateInfo.latestVersion)}
        </button>
        <button
          onClick={dismissUpdate}
          className="ml-4 w-6 h-6 flex items-center justify-center rounded-full hover:bg-[#0b0e11]/10"
          aria-label="Dismiss"
        >
          ✕
        </button>
      </div>
    );
  };

  const renderTradFiBanner = () => {
    if (!tradfiUnsigned) return null;
    if (theme === 'classic') {
      return (
        <div className="bg-orange-600 text-white px-4 py-2 flex items-center justify-between text-sm">
          <span>{t('tradfi.banner')}</span>
          <div className="flex items-center gap-2">
            <button onClick={handleSignTradFi} className="bg-white/20 hover:bg-white/30 px-3 py-1 rounded text-xs font-medium">
              {t('tradfi.sign')}
            </button>
            <button onClick={() => { sessionStorage.setItem('arb_tradfi_dismissed', '1'); setTradfiUnsigned(false); }}
              className="text-white/70 hover:text-white">✕</button>
          </div>
        </div>
      );
    }
    return (
      <div className="bg-[#2a1f02] border-b border-[#f0b90b]/40 text-[#f0b90b] px-4 py-2 flex items-center justify-between text-sm">
        <span className="font-medium">{t('tradfi.banner')}</span>
        <div className="flex items-center gap-2">
          <button
            onClick={handleSignTradFi}
            className="bg-[#f0b90b] text-[#0b0e11] hover:bg-[#fcd535] px-3 py-1 rounded-full text-xs font-semibold transition-colors"
          >
            {t('tradfi.sign')}
          </button>
          <button
            onClick={() => { sessionStorage.setItem('arb_tradfi_dismissed', '1'); setTradfiUnsigned(false); }}
            className="w-6 h-6 flex items-center justify-center rounded-full text-[#f0b90b]/70 hover:text-[#f0b90b] hover:bg-[#f0b90b]/10"
            aria-label="Dismiss"
          >
            ✕
          </button>
        </div>
      </div>
    );
  };

  const renderUpdateModal = () => {
    if (!showUpdateModal || !updateInfo) return null;
    if (theme === 'classic') {
      return (
        <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-50">
          <div className="bg-gray-900 rounded-lg shadow-xl w-full max-w-lg mx-4 max-h-[80vh] flex flex-col">
            <div className="p-4 border-b border-gray-700 flex justify-between items-center">
              <h2 className="text-lg font-semibold">{t('update.title')} — v{updateInfo.latestVersion}</h2>
              <button onClick={() => setShowUpdateModal(false)} className="text-gray-400 hover:text-white">✕</button>
            </div>
            {updateInfo.changelog && (
              <div className="p-4 border-b border-gray-700 overflow-auto flex-1">
                <h3 className="text-sm font-medium text-gray-400 mb-2">{t('update.changelog')}</h3>
                <pre className="text-xs text-gray-300 whitespace-pre-wrap">{updateInfo.changelog}</pre>
              </div>
            )}
            {updateOutput && (
              <div className="p-4 border-b border-gray-700">
                <pre className="text-xs text-green-400 bg-gray-950 p-3 rounded whitespace-pre-wrap max-h-40 overflow-auto">{updateOutput}</pre>
              </div>
            )}
            <div className="p-4 flex justify-end gap-3">
              <button
                onClick={() => setShowUpdateModal(false)}
                className="px-4 py-2 text-sm bg-gray-700 hover:bg-gray-600 rounded"
                disabled={updating}
              >
                {t('update.cancel')}
              </button>
              <button
                onClick={handleUpdate}
                className="px-4 py-2 text-sm bg-blue-600 hover:bg-blue-500 rounded disabled:opacity-50"
                disabled={updating}
              >
                {updating ? t('update.updating') : t('update.confirm')}
              </button>
            </div>
          </div>
        </div>
      );
    }
    return (
      <div className="fixed inset-0 bg-black/70 backdrop-blur-sm flex items-center justify-center z-50 p-4">
        <div className="bg-[#1e2026] border border-[#2b2f36] rounded-2xl shadow-[0_20px_50px_rgba(0,0,0,0.5)] w-full max-w-lg max-h-[80vh] flex flex-col overflow-hidden">
          <div className="px-6 py-4 border-b border-[#2b2f36] flex justify-between items-center">
            <h2 className="text-lg font-bold text-gray-100">
              {t('update.title')} <span className="text-[#f0b90b]">v{updateInfo.latestVersion}</span>
            </h2>
            <button
              onClick={() => setShowUpdateModal(false)}
              className="w-8 h-8 flex items-center justify-center rounded-full text-gray-400 hover:text-gray-100 hover:bg-[#2b2f36] transition-colors"
              aria-label="Close"
            >
              ✕
            </button>
          </div>
          {updateInfo.changelog && (
            <div className="px-6 py-4 border-b border-[#2b2f36] overflow-auto flex-1">
              <h3 className="text-xs font-semibold uppercase tracking-wider text-gray-400 mb-2">
                {t('update.changelog')}
              </h3>
              <pre className="text-xs text-gray-300 font-mono whitespace-pre-wrap leading-relaxed">
                {updateInfo.changelog}
              </pre>
            </div>
          )}
          {updateOutput && (
            <div className="px-6 py-4 border-b border-[#2b2f36]">
              <pre className="text-xs text-[#0ecb81] bg-[#0b0e11] border border-[#2b2f36] p-3 rounded-lg whitespace-pre-wrap max-h-40 overflow-auto font-mono">
                {updateOutput}
              </pre>
            </div>
          )}
          <div className="px-6 py-4 flex justify-end gap-3 bg-[#17181b]">
            <button
              onClick={() => setShowUpdateModal(false)}
              className="btn-secondary"
              disabled={updating}
            >
              {t('update.cancel')}
            </button>
            <button
              onClick={handleUpdate}
              className="btn-primary"
              disabled={updating}
            >
              {updating ? t('update.updating') : t('update.confirm')}
            </button>
          </div>
        </div>
      </div>
    );
  };

  return (
    <ThemeContext.Provider value={{ theme, setTheme }}>
      <LocaleContext.Provider value={{ locale, setLocale, t }}>
        <div className={`flex flex-col md:flex-row min-h-screen ${theme === 'classic' ? 'bg-gray-950' : 'bg-[#0b0e11]'} text-gray-100`}>
          <Sidebar
            page={page}
            onNavigate={(p) => setPage(p as Page)}
            connected={ws.connected}
            onLogout={handleLogout}
            mobileOpen={sidebarOpen}
            onMobileClose={() => setSidebarOpen(false)}
          />

          <div className="flex-1 flex flex-col overflow-auto">
            {renderMobileHeader()}
            {renderUpdateBanner()}
            {renderTradFiBanner()}
            <main className={`flex-1 ${theme === 'classic' ? 'p-3 md:p-6' : 'p-4 md:p-8'}`}>
              {renderPage()}
            </main>
          </div>

          {renderUpdateModal()}
        </div>
      </LocaleContext.Provider>
    </ThemeContext.Provider>
  );
}

export default App;
