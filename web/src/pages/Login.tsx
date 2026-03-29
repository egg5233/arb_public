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
    <div className="min-h-screen bg-gray-950 flex items-center justify-center">
      <div className="bg-gray-900 border border-gray-800 rounded-lg p-8 w-full max-w-sm">
        <h1 className="text-xl font-bold text-gray-100 mb-6 text-center">{t('login.title')}</h1>
        <form onSubmit={handleSubmit}>
          <label className="block text-sm text-gray-400 mb-2">{t('login.password')}</label>
          <input
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            className="w-full bg-gray-800 border border-gray-700 rounded-md px-3 py-2 text-gray-100 text-sm focus:outline-none focus:border-blue-500 mb-4"
            placeholder={t('login.password')}
          />
          {error && <p className="text-red-400 text-sm mb-4">{t('login.error')}</p>}
          <button
            type="submit"
            disabled={loading}
            className="w-full bg-blue-600 hover:bg-blue-700 text-white rounded-md py-2 text-sm font-medium transition-colors disabled:opacity-50"
          >
            {loading ? t('login.loading') : t('login.submit')}
          </button>
        </form>
      </div>
    </div>
  );
};

export default Login;
