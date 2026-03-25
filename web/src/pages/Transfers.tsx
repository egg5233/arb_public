import { useState, useEffect, type FC } from 'react';
import type { TransferRecord } from '../types.ts';
import { useLocale } from '../i18n/index.ts';

interface TransfersProps {
  transfer: (data: { from: string; to: string; coin: string; chain: string; amount: string }) => Promise<unknown>;
  getTransfers: (limit?: number) => Promise<unknown>;
  getAddresses: () => Promise<unknown>;
  updateAddresses: (data: Record<string, Record<string, string>>) => Promise<unknown>;
}

const EXCHANGES = ['binance', 'bitget', 'bybit', 'gateio', 'okx', 'bingx'];
const CHAINS = ['APT', 'BEP20'];

const Transfers: FC<TransfersProps> = ({ transfer, getTransfers, getAddresses, updateAddresses }) => {
  const { t } = useLocale();
  const [from, setFrom] = useState('');
  const [to, setTo] = useState('');
  const [chain, setChain] = useState('BEP20');
  const [amount, setAmount] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [success, setSuccess] = useState('');
  const [history, setHistory] = useState<TransferRecord[]>([]);
  const [addresses, setAddresses] = useState<Record<string, Record<string, string>>>({});
  const [editAddresses, setEditAddresses] = useState<Record<string, Record<string, string>>>({});
  const [addrSaving, setAddrSaving] = useState(false);
  const [addrMsg, setAddrMsg] = useState('');

  useEffect(() => {
    loadData();
  }, []);

  const loadData = async () => {
    try {
      const [transfersRes, addrsRes] = await Promise.all([
        getTransfers(50),
        getAddresses(),
      ]);
      const tData = transfersRes as { ok: boolean; data: TransferRecord[] };
      const aData = addrsRes as { ok: boolean; data: Record<string, Record<string, string>> };
      if (tData?.data) setHistory(tData.data);
      if (aData?.data) {
        setAddresses(aData.data);
        // Deep clone for editing
        const clone: Record<string, Record<string, string>> = {};
        for (const [exch, chains] of Object.entries(aData.data)) {
          clone[exch] = { ...chains };
        }
        // Ensure all exchanges have entries for all chains
        for (const exch of EXCHANGES) {
          if (!clone[exch]) clone[exch] = {};
          for (const c of CHAINS) {
            if (!clone[exch][c]) clone[exch][c] = '';
          }
        }
        setEditAddresses(clone);
      }
    } catch {
      // ignore
    }
  };

  // Filter available chains based on destination exchange's configured addresses
  const availableChains = CHAINS.filter((c) => {
    if (!to) return true;
    return addresses[to]?.[c];
  });

  // Auto-select first available chain when current selection is not available
  useEffect(() => {
    if (availableChains.length > 0 && !availableChains.includes(chain)) {
      setChain(availableChains[0]);
    }
  }, [to, availableChains.join(',')]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setSuccess('');

    if (!from || !to || !amount || !chain) {
      setError(t('xfer.allRequired'));
      return;
    }
    if (from === to) {
      setError(t('xfer.sameSrcDst'));
      return;
    }
    const amt = parseFloat(amount);
    if (isNaN(amt) || amt <= 0) {
      setError(t('xfer.positiveAmount'));
      return;
    }

    setLoading(true);
    try {
      const res = await transfer({ from, to, coin: 'USDT', chain, amount });
      const data = res as { ok: boolean; data: TransferRecord; error?: string };
      if (data?.ok) {
        setSuccess(`Transfer submitted: ${data.data.tx_id || data.data.id}`);
        setAmount('');
        loadData();
      } else {
        setError(data?.error || 'Transfer failed');
      }
    } catch (err) {
      setError(String(err));
    } finally {
      setLoading(false);
    }
  };

  return (
    <div>
      <h2 className="text-xl font-bold mb-6">{t('xfer.title')}</h2>

      {/* Transfer Form */}
      <div className="bg-gray-900 rounded-lg border border-gray-800 p-6 mb-6">
        <h3 className="text-sm font-semibold text-gray-400 uppercase mb-4">{t('xfer.newTransfer')}</h3>
        <form onSubmit={handleSubmit} className="grid grid-cols-1 md:grid-cols-5 gap-4 items-end">
          <div>
            <label className="block text-xs text-gray-400 mb-1">{t('xfer.from')}</label>
            <select
              value={from}
              onChange={(e) => setFrom(e.target.value)}
              className="w-full bg-gray-800 border border-gray-700 rounded px-3 py-2 text-sm text-gray-100"
            >
              <option value="">{t('xfer.select')}</option>
              {EXCHANGES.map((ex) => (
                <option key={ex} value={ex}>{ex}</option>
              ))}
            </select>
          </div>

          <div>
            <label className="block text-xs text-gray-400 mb-1">{t('xfer.to')}</label>
            <select
              value={to}
              onChange={(e) => setTo(e.target.value)}
              className="w-full bg-gray-800 border border-gray-700 rounded px-3 py-2 text-sm text-gray-100"
            >
              <option value="">{t('xfer.select')}</option>
              {EXCHANGES.filter((ex) => ex !== from).map((ex) => (
                <option key={ex} value={ex}>{ex}</option>
              ))}
            </select>
          </div>

          <div>
            <label className="block text-xs text-gray-400 mb-1">{t('xfer.chain')}</label>
            <select
              value={chain}
              onChange={(e) => setChain(e.target.value)}
              className="w-full bg-gray-800 border border-gray-700 rounded px-3 py-2 text-sm text-gray-100"
            >
              {availableChains.map((c) => (
                <option key={c} value={c}>{c}</option>
              ))}
            </select>
          </div>

          <div>
            <label className="block text-xs text-gray-400 mb-1">{t('xfer.amount')}</label>
            <input
              type="number"
              step="0.01"
              min="0"
              value={amount}
              onChange={(e) => setAmount(e.target.value)}
              className="w-full bg-gray-800 border border-gray-700 rounded px-3 py-2 text-sm text-gray-100"
            />
          </div>

          <button
            type="submit"
            disabled={loading}
            className="bg-blue-600 hover:bg-blue-700 disabled:bg-gray-700 text-white px-4 py-2 rounded text-sm font-medium transition-colors"
          >
            {loading ? t('xfer.sending') : t('xfer.submit')}
          </button>
        </form>

        {error && (
          <div className="mt-3 text-sm text-red-400 bg-red-900/20 border border-red-800 rounded px-3 py-2">
            {error}
          </div>
        )}
        {success && (
          <div className="mt-3 text-sm text-green-400 bg-green-900/20 border border-green-800 rounded px-3 py-2">
            {success}
          </div>
        )}
      </div>

      {/* Deposit Addresses */}
      <div className="bg-gray-900 rounded-lg border border-gray-800 p-6 mb-6">
        <h3 className="text-sm font-semibold text-gray-400 uppercase mb-4">{t('xfer.addresses')}</h3>
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-gray-800 text-gray-400 text-xs uppercase">
                <th className="text-left px-3 py-2">{t('xfer.exchange')}</th>
                {CHAINS.map((c) => (
                  <th key={c} className="text-left px-3 py-2">{c}</th>
                ))}
              </tr>
            </thead>
            <tbody>
              {EXCHANGES.map((exch) => (
                <tr key={exch} className="border-b border-gray-800/50">
                  <td className="px-3 py-2 text-gray-100 font-medium">{exch}</td>
                  {CHAINS.map((c) => (
                    <td key={c} className="px-3 py-1">
                      <input
                        type="text"
                        value={editAddresses[exch]?.[c] || ''}
                        onChange={(e) => {
                          setEditAddresses((prev) => ({
                            ...prev,
                            [exch]: { ...prev[exch], [c]: e.target.value },
                          }));
                        }}
                        placeholder={`${exch} ${c} address`}
                        className="w-full bg-gray-800 border border-gray-700 rounded px-2 py-1.5 text-xs text-gray-100 font-mono placeholder-gray-600"
                      />
                    </td>
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        <div className="mt-4 flex items-center gap-3">
          <button
            onClick={async () => {
              setAddrSaving(true);
              setAddrMsg('');
              try {
                await updateAddresses(editAddresses);
                setAddrMsg(t('xfer.addrSaved'));
                loadData();
              } catch (err) {
                setAddrMsg(String(err));
              } finally {
                setAddrSaving(false);
              }
            }}
            disabled={addrSaving}
            className="bg-blue-600 hover:bg-blue-700 disabled:bg-gray-700 text-white px-4 py-2 rounded text-sm font-medium transition-colors"
          >
            {addrSaving ? '...' : t('xfer.saveAddresses')}
          </button>
          {addrMsg && (
            <span className={`text-sm ${addrMsg.includes('Error') ? 'text-red-400' : 'text-green-400'}`}>
              {addrMsg}
            </span>
          )}
        </div>
      </div>

      {/* Transfer History */}
      <div className="bg-gray-900 rounded-lg border border-gray-800 overflow-hidden">
        <div className="px-4 py-3 border-b border-gray-800">
          <h3 className="text-sm font-semibold text-gray-400 uppercase">{t('xfer.history')}</h3>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-gray-800 text-gray-400 text-xs uppercase">
                <th className="text-left px-4 py-2">{t('xfer.date')}</th>
                <th className="text-left px-4 py-2">{t('xfer.route')}</th>
                <th className="text-left px-4 py-2">{t('xfer.chainCol')}</th>
                <th className="text-right px-4 py-2">{t('xfer.amountCol')}</th>
                <th className="text-right px-4 py-2">{t('xfer.fee')}</th>
                <th className="text-left px-4 py-2">{t('xfer.statusCol')}</th>
                <th className="text-left px-4 py-2">{t('xfer.txId')}</th>
              </tr>
            </thead>
            <tbody>
              {history.length === 0 ? (
                <tr>
                  <td colSpan={7} className="text-center text-gray-500 py-8">
                    {t('xfer.noTransfers')}
                  </td>
                </tr>
              ) : (
                history.map((tr) => (
                  <tr key={tr.id} className="border-b border-gray-800/50 hover:bg-gray-800/30">
                    <td className="px-4 py-2 text-gray-300">
                      {new Date(tr.created_at).toLocaleString()}
                    </td>
                    <td className="px-4 py-2">
                      <span className="text-gray-100">{tr.from}</span>
                      <span className="text-gray-500 mx-1">&rarr;</span>
                      <span className="text-gray-100">{tr.to}</span>
                    </td>
                    <td className="px-4 py-2 text-gray-300">{tr.chain}</td>
                    <td className="px-4 py-2 text-right text-gray-100">
                      {tr.amount} {tr.coin}
                    </td>
                    <td className="px-4 py-2 text-right text-gray-400">
                      {tr.fee || '-'}
                    </td>
                    <td className="px-4 py-2">
                      <span className={`text-xs px-2 py-0.5 rounded ${
                        tr.status === 'submitted' ? 'bg-yellow-900/30 text-yellow-400' :
                        tr.status === 'completed' ? 'bg-green-900/30 text-green-400' :
                        'bg-gray-800 text-gray-400'
                      }`}>
                        {tr.status}
                      </span>
                    </td>
                    <td className="px-4 py-2 text-gray-400 font-mono text-xs max-w-[120px] truncate">
                      {tr.tx_id || '-'}
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
};

export default Transfers;
