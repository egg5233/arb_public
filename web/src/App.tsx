import { useState, useEffect, useCallback } from 'react';
import { useApi } from './hooks/useApi.ts';
import { useWebSocket } from './hooks/useWebSocket.ts';
import Sidebar from './components/Sidebar.tsx';
import Login from './pages/Login.tsx';
import Overview from './pages/Overview.tsx';
import Opportunities from './pages/Opportunities.tsx';
import Positions from './pages/Positions.tsx';
import History from './pages/History.tsx';
import Config from './pages/Config.tsx';
import Transfers from './pages/Transfers.tsx';
import Logs from './pages/Logs.tsx';
import Rejections from './pages/Rejections.tsx';
import { LocaleContext, getStoredLocale, storeLocale, t as translate, type Locale } from './i18n/index.ts';
import type { ExchangeInfo } from './types.ts';

type Page = 'overview' | 'opportunities' | 'positions' | 'history' | 'config' | 'transfers' | 'logs' | 'rejections';

const UPDATE_DISMISS_KEY = 'arb_update_dismissed';

function App() {
  const api = useApi();
  const [page, setPage] = useState<Page>('overview');
  const [exchanges, setExchanges] = useState<ExchangeInfo[]>([]);
  const [locale, setLocaleState] = useState<Locale>(getStoredLocale);
  const ws = useWebSocket(!!api.token);

  // Update state
  const [updateInfo, setUpdateInfo] = useState<{ latestVersion: string; changelog: string } | null>(null);
  const [showUpdateModal, setShowUpdateModal] = useState(false);
  const [updateOutput, setUpdateOutput] = useState('');
  const [updating, setUpdating] = useState(false);

  const setLocale = useCallback((l: Locale) => {
    setLocaleState(l);
    storeLocale(l);
  }, []);

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

  // Seed WS state from REST on initial load, and refresh exchanges periodically.
  useEffect(() => {
    if (!api.token) return;
    api.getPositions().then(ws.setPositions).catch(() => {});
    api.getOpportunities().then(ws.setOpportunities).catch(() => {});
    api.getStats().then(ws.setStats).catch(() => {});
    const loadExchanges = () => {
      api.getExchanges().then(setExchanges).catch(() => {});
    };
    loadExchanges();
    const interval = setInterval(loadExchanges, 60000);

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
      <LocaleContext.Provider value={{ locale, setLocale, t }}>
        <Login onLogin={handleLogin} />
      </LocaleContext.Provider>
    );
  }

  const renderPage = () => {
    switch (page) {
      case 'overview':
        return (
          <Overview
            positions={ws.positions}
            stats={ws.stats}
            exchanges={exchanges}
            onDiagnose={api.diagnose}
          />
        );
      case 'opportunities':
        return (
          <Opportunities
            opportunities={ws.opportunities}
            onOpen={api.openPosition}
          />
        );
      case 'positions':
        return (
          <Positions
            positions={ws.positions}
            onClose={api.closePosition}
          />
        );
      case 'history':
        return <History getHistory={api.getHistory} />;
      case 'config':
        return <Config getConfig={api.getConfig} updateConfig={api.updateConfig} />;
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

  return (
    <LocaleContext.Provider value={{ locale, setLocale, t }}>
      <div className="flex min-h-screen bg-gray-950 text-gray-100">
        <Sidebar
          page={page}
          onNavigate={(p) => setPage(p as Page)}
          connected={ws.connected}
          onLogout={handleLogout}
        />
        <div className="flex-1 flex flex-col overflow-auto">
          {/* Update banner */}
          {updateInfo && (
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
          )}
          <main className="flex-1 p-6">
            {renderPage()}
          </main>
        </div>

        {/* Update modal */}
        {showUpdateModal && updateInfo && (
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
        )}
      </div>
    </LocaleContext.Provider>
  );
}

export default App;
