import type { FC } from 'react';
import { useLocale, type Locale, type TranslationKey } from '../i18n/index.ts';
import { useTheme } from '../theme/index.ts';

interface SidebarProps {
  page: string;
  onNavigate: (page: string) => void;
  connected: boolean;
  onLogout: () => void;
  mobileOpen?: boolean;
  onMobileClose?: () => void;
}

const navItems: { id: string; labelKey: TranslationKey; icon: string }[] = [
  { id: 'overview', labelKey: 'nav.overview', icon: '\u25A0' },
  { id: 'opportunities', labelKey: 'nav.opportunities', icon: '\u25B2' },
  { id: 'positions', labelKey: 'nav.positions', icon: '\u25C6' },
  { id: 'spot-positions', labelKey: 'nav.spotPositions', icon: '\u25C8' },
  { id: 'price-gap', labelKey: 'nav.pricegap', icon: '\u25C7' },
  { id: 'history', labelKey: 'nav.history', icon: '\u25CB' },
  { id: 'analytics', labelKey: 'nav.analytics', icon: '\u2261' },
  { id: 'transfers', labelKey: 'nav.transfers', icon: '\u21C4' },
  { id: 'rejections', labelKey: 'nav.rejections', icon: '\u2717' },
  { id: 'logs', labelKey: 'nav.logs', icon: '\u2630' },
  { id: 'permissions', labelKey: 'nav.permissions', icon: '\u26BF' },
  { id: 'config', labelKey: 'nav.config', icon: '\u2699' },
];

export { navItems };

const LOCALES: { value: Locale; label: string }[] = [
  { value: 'zh-TW', label: '繁中' },
  { value: 'en', label: 'EN' },
];

const Sidebar: FC<SidebarProps> = ({ page, onNavigate, connected, onLogout, mobileOpen, onMobileClose }) => {
  const { locale, setLocale, t } = useLocale();
  const { theme } = useTheme();

  const handleNavigate = (p: string) => {
    onNavigate(p);
    onMobileClose?.();
  };

  // ── Classic sidebar content ───────────────────────────────────────────────
  const sidebarContentClassic = (
    <div className="w-56 bg-gray-900 border-r border-gray-800 flex flex-col h-screen">
      <div className="p-4 border-b border-gray-800">
        <h1 className="text-lg font-bold text-gray-100">{t('sidebar.title')}</h1>
        <div className="flex items-center gap-2 mt-2 text-sm text-gray-400">
          <span
            className={`inline-block w-2 h-2 rounded-full ${
              connected ? 'bg-green-500' : 'bg-red-500'
            }`}
          />
          {connected ? t('sidebar.connected') : t('sidebar.disconnected')}
          <span className="ml-auto text-xs text-gray-500">v{__APP_VERSION__}</span>
        </div>
      </div>
      <nav className="flex-1 p-2">
        {navItems.map((item) => (
          <button
            key={item.id}
            onClick={() => handleNavigate(item.id)}
            className={`w-full text-left px-3 py-2 rounded-md mb-1 flex items-center gap-2 text-sm transition-colors ${
              page === item.id
                ? 'bg-blue-500/20 text-blue-400'
                : 'text-gray-400 hover:text-gray-100 hover:bg-gray-800'
            }`}
          >
            <span className="text-xs">{item.icon}</span>
            {t(item.labelKey)}
          </button>
        ))}
      </nav>
      <div className="p-4 border-t border-gray-800 space-y-2">
        <div className="flex gap-1">
          {LOCALES.map((l) => (
            <button
              key={l.value}
              onClick={() => setLocale(l.value)}
              className={`flex-1 text-xs px-2 py-1 rounded transition-colors ${
                locale === l.value
                  ? 'bg-blue-500/20 text-blue-400'
                  : 'text-gray-500 hover:text-gray-300 hover:bg-gray-800'
              }`}
            >
              {l.label}
            </button>
          ))}
        </div>
        <button
          onClick={onLogout}
          className="w-full text-left px-3 py-2 text-sm text-gray-400 hover:text-gray-100 hover:bg-gray-800 rounded-md transition-colors"
        >
          {t('sidebar.logout')}
        </button>
      </div>
    </div>
  );

  // ── New (Binance) sidebar content ─────────────────────────────────────────
  const sidebarContentNew = (
    <aside className="w-60 bg-[#0b0e11] border-r border-[#1e2026] flex flex-col h-screen">
      {/* Brand mark — Binance-yellow diamond + wordmark */}
      <div className="px-5 pt-5 pb-4 border-b border-[#1e2026]">
        <div className="flex items-center gap-2.5">
          <span
            aria-hidden
            className="inline-block w-6 h-6 rotate-45 bg-[#f0b90b] rounded-[2px]"
          />
          <span className="text-[15px] font-bold tracking-tight text-gray-100 uppercase">
            {t('sidebar.title')}
          </span>
        </div>
        <div className="flex items-center gap-2 mt-3 text-[11px] font-medium uppercase tracking-wider">
          <span
            className={`inline-block w-1.5 h-1.5 rounded-full ${
              connected ? 'bg-[#0ecb81]' : 'bg-[#f6465d]'
            }`}
          />
          <span className={connected ? 'text-[#0ecb81]' : 'text-[#f6465d]'}>
            {connected ? t('sidebar.connected') : t('sidebar.disconnected')}
          </span>
          <span className="ml-auto text-[10px] text-gray-500 normal-case tracking-normal">v{__APP_VERSION__}</span>
        </div>
      </div>

      {/* Navigation */}
      <nav className="flex-1 px-3 py-4 overflow-y-auto">
        {navItems.map((item) => {
          const active = page === item.id;
          return (
            <button
              key={item.id}
              onClick={() => handleNavigate(item.id)}
              className={`w-full text-left px-3 py-2 rounded-md mb-0.5 flex items-center gap-3 text-[13px] font-medium transition-colors relative ${
                active
                  ? 'bg-[#1e2026] text-[#f0b90b]'
                  : 'text-gray-400 hover:text-gray-100 hover:bg-[#17181b]'
              }`}
            >
              {active && (
                <span
                  aria-hidden
                  className="absolute left-0 top-1.5 bottom-1.5 w-[3px] rounded-r bg-[#f0b90b]"
                />
              )}
              <span className={`text-[11px] ${active ? 'text-[#f0b90b]' : 'text-gray-500'}`}>
                {item.icon}
              </span>
              {t(item.labelKey)}
            </button>
          );
        })}
      </nav>

      {/* Footer — locale pills + logout */}
      <div className="px-4 py-4 border-t border-[#1e2026] space-y-3">
        <div className="flex gap-1 p-0.5 bg-[#17181b] rounded-full">
          {LOCALES.map((l) => (
            <button
              key={l.value}
              onClick={() => setLocale(l.value)}
              className={`flex-1 text-[11px] font-semibold px-2 py-1 rounded-full transition-colors ${
                locale === l.value
                  ? 'bg-[#f0b90b] text-[#0b0e11]'
                  : 'text-gray-400 hover:text-gray-100'
              }`}
            >
              {l.label}
            </button>
          ))}
        </div>
        <button
          onClick={onLogout}
          className="w-full px-3 py-2 text-[12px] font-semibold uppercase tracking-wider text-gray-400 hover:text-[#f0b90b] border border-[#2b2f36] hover:border-[#f0b90b] rounded-full transition-colors"
        >
          {t('sidebar.logout')}
        </button>
      </div>
    </aside>
  );

  const sidebarContent = theme === 'classic' ? sidebarContentClassic : sidebarContentNew;

  return (
    <>
      {/* Desktop sidebar — always visible */}
      <div className="hidden md:block sticky top-0 h-screen">
        {sidebarContent}
      </div>

      {/* Mobile overlay drawer */}
      {mobileOpen && (
        <div className="fixed inset-0 z-50 md:hidden">
          {/* Backdrop */}
          <div
            className={`absolute inset-0 ${theme === 'classic' ? 'bg-black/60' : 'bg-black/70'}`}
            onClick={onMobileClose}
          />
          {/* Drawer */}
          <div className="absolute inset-y-0 left-0 animate-slide-in-left">
            {sidebarContent}
          </div>
        </div>
      )}
    </>
  );
};

export default Sidebar;
