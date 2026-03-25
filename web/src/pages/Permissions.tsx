import { useState, useEffect, type FC } from 'react';
import { useLocale } from '../i18n/index.ts';

type PermResult = {
  read: string;
  futures_trade: string;
  withdraw: string;
  transfer: string;
  method: string;
  error?: string;
};

interface PermissionsProps {
  getPermissions: () => Promise<Record<string, PermResult>>;
}

function permIcon(s: string) {
  if (s === 'granted') return '✅';
  if (s === 'denied') return '❌';
  return '❓';
}

function permLabel(s: string) {
  if (s === 'granted') return 'Granted';
  if (s === 'denied') return 'Denied';
  return 'Unknown';
}

const Permissions: FC<PermissionsProps> = ({ getPermissions }) => {
  const { t } = useLocale();
  const [permissions, setPermissions] = useState<Record<string, PermResult>>({});
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    getPermissions()
      .then(setPermissions)
      .catch(() => {})
      .finally(() => setLoading(false));
  }, [getPermissions]);

  return (
    <div className="space-y-6">
      <h2 className="text-xl font-bold text-gray-100">{t('perm.title')}</h2>

      <div className="bg-gray-900 border border-gray-800 rounded-lg p-4">
        <p className="text-sm text-gray-400 mb-4">{t('perm.description')}</p>

        {loading ? (
          <p className="text-gray-500 text-sm">{t('perm.loading')}</p>
        ) : Object.keys(permissions).length === 0 ? (
          <p className="text-gray-500 text-sm">{t('perm.noData')}</p>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="text-gray-400 text-left border-b border-gray-800 text-xs uppercase">
                  <th className="pb-2 px-3">{t('perm.exchange')}</th>
                  <th className="pb-2 px-3 text-center">{t('perm.read')}</th>
                  <th className="pb-2 px-3 text-center">{t('perm.trade')}</th>
                  <th className="pb-2 px-3 text-center">{t('perm.withdraw')}</th>
                  <th className="pb-2 px-3 text-center">{t('perm.transfer')}</th>
                  <th className="pb-2 px-3 text-center">{t('perm.method')}</th>
                  <th className="pb-2 px-3">{t('perm.note')}</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-800">
                {Object.entries(permissions).map(([name, p]) => (
                  <tr key={name} className="text-gray-100">
                    <td className="py-2.5 px-3 capitalize font-medium">{name}</td>
                    <td className="py-2.5 px-3 text-center" title={permLabel(p.read)}>{permIcon(p.read)}</td>
                    <td className="py-2.5 px-3 text-center" title={permLabel(p.futures_trade)}>{permIcon(p.futures_trade)}</td>
                    <td className="py-2.5 px-3 text-center" title={permLabel(p.withdraw)}>{permIcon(p.withdraw)}</td>
                    <td className="py-2.5 px-3 text-center" title={permLabel(p.transfer)}>{permIcon(p.transfer)}</td>
                    <td className="py-2.5 px-3 text-center">
                      <span className={`text-xs px-2 py-0.5 rounded ${
                        p.method === 'direct' ? 'bg-green-900/30 text-green-400' :
                        p.method === 'inferred' ? 'bg-yellow-900/30 text-yellow-400' :
                        'bg-gray-800 text-gray-500'
                      }`}>
                        {p.method}
                      </span>
                    </td>
                    <td className="py-2.5 px-3 text-xs text-gray-500">{p.error || '-'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}

        <div className="mt-4 text-xs text-gray-500 space-y-1">
          <p>✅ {t('perm.legendGranted')}</p>
          <p>❌ {t('perm.legendDenied')}</p>
          <p>❓ {t('perm.legendUnknown')}</p>
        </div>

        <div className="mt-4 p-3 bg-yellow-900/20 border border-yellow-800/50 rounded text-xs text-yellow-300 space-y-1">
          <p className="font-semibold">{t('perm.tipTitle')}</p>
          <p>{t('perm.tipGateio')}</p>
        </div>
      </div>
    </div>
  );
};

export default Permissions;
