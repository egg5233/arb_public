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

function App() {
  const api = useApi();
  const [page, setPage] = useState<Page>('overview');
  const [exchanges, setExchanges] = useState<ExchangeInfo[]>([]);
  const [locale, setLocaleState] = useState<Locale>(getStoredLocale);
  const ws = useWebSocket(!!api.token);

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
    return () => clearInterval(interval);
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
        <main className="flex-1 p-6 overflow-auto">
          {renderPage()}
        </main>
      </div>
    </LocaleContext.Provider>
  );
}

export default App;
