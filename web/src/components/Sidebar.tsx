import type { FC } from 'react';
import { useLocale, type Locale, type TranslationKey } from '../i18n/index.ts';

interface SidebarProps {
  page: string;
  onNavigate: (page: string) => void;
  connected: boolean;
  onLogout: () => void;
  mobileOpen?: boolean;
  onMobileClose?: () => void;
  version?: string;
}

const navItems: { id: string; labelKey: TranslationKey; icon: string }[] = [
  { id: 'overview', labelKey: 'nav.overview', icon: '\u25A0' },
  { id: 'opportunities', labelKey: 'nav.opportunities', icon: '\u25B2' },
  { id: 'positions', labelKey: 'nav.positions', icon: '\u25C6' },
  { id: 'history', labelKey: 'nav.history', icon: '\u25CB' },
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

const Sidebar: FC<SidebarProps> = ({ page, onNavigate, connected, onLogout, mobileOpen, onMobileClose, version }) => {
  const { locale, setLocale, t } = useLocale();

  const handleNavigate = (p: string) => {
    onNavigate(p);
    onMobileClose?.();
  };

  const sidebarContent = (
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
          {version && <span className="ml-auto text-xs text-gray-500">v{version}</span>}
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
            className="absolute inset-0 bg-black/60"
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
