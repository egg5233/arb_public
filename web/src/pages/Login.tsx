import { useState } from 'react';
import type { FC, FormEvent } from 'react';
import { useLocale } from '../i18n/index.ts';

interface LoginProps {
  onLogin: (password: string) => Promise<void>;
}

const Login: FC<LoginProps> = ({ onLogin }) => {
  const { t } = useLocale();
  const [password, setPassword] = useState('');
  const [error, setError] = useState(false);
  const [loading, setLoading] = useState(false);

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setError(false);
    setLoading(true);
    try {
      await onLogin(password);
    } catch {
      setError(true);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen bg-[#0b0e11] flex items-center justify-center px-4 relative overflow-hidden">
      {/* Ambient gold glow in corner — a subtle Binance flourish */}
      <div
        aria-hidden
        className="pointer-events-none absolute -top-40 -right-40 w-[480px] h-[480px] rounded-full opacity-[0.08]"
        style={{
          background:
            'radial-gradient(circle, #f0b90b 0%, transparent 70%)',
        }}
      />
      <div
        aria-hidden
        className="pointer-events-none absolute -bottom-40 -left-40 w-[480px] h-[480px] rounded-full opacity-[0.05]"
        style={{
          background:
            'radial-gradient(circle, #f0b90b 0%, transparent 70%)',
        }}
      />

      <div className="relative w-full max-w-sm">
        {/* Brand mark */}
        <div className="flex items-center gap-3 justify-center mb-8">
          <span
            aria-hidden
            className="inline-block w-8 h-8 rotate-45 bg-[#f0b90b] rounded-[3px]"
          />
          <span className="text-xl font-bold tracking-tight text-gray-100 uppercase">
            {t('sidebar.title')}
          </span>
        </div>

        <div className="bg-[#1e2026] border border-[#2b2f36] rounded-2xl p-8 shadow-[0_8px_40px_rgba(0,0,0,0.4)]">
          <h1 className="text-2xl font-bold text-gray-100 mb-1">{t('login.title')}</h1>
          <p className="text-sm text-gray-400 mb-6">
            {t('login.password')}
          </p>
          <form onSubmit={handleSubmit}>
            <input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              className="w-full bg-[#0b0e11] border border-[#2b2f36] rounded-lg px-4 py-3 text-gray-100 text-sm placeholder:text-gray-500 focus:outline-none focus:border-[#f0b90b] transition-colors mb-4"
              placeholder={t('login.password')}
              autoFocus
            />
            {error && (
              <p className="text-[#f6465d] text-sm mb-4 flex items-center gap-2">
                <span aria-hidden>✕</span>
                {t('login.error')}
              </p>
            )}
            <button
              type="submit"
              disabled={loading}
              className="btn-primary w-full py-3"
            >
              {loading ? t('login.loading') : t('login.submit')}
            </button>
          </form>
        </div>

        <p className="text-center text-[11px] text-gray-600 mt-6 tracking-wider uppercase">
          Funding Rate Arbitrage Terminal
        </p>
      </div>
    </div>
  );
};

export default Login;
