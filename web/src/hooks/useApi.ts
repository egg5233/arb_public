import { useState, useCallback } from 'react';
import type { Position, Opportunity, Stats, ExchangeInfo, TransferRecord, LogEntry, RejectedOpportunity } from '../types.ts';

const TOKEN_KEY = 'arb_token';

interface ApiResponse<T> {
  ok: boolean;
  data?: T;
  error?: string;
}

function getToken(): string | null {
  return localStorage.getItem(TOKEN_KEY);
}

function setToken(token: string) {
  localStorage.setItem(TOKEN_KEY, token);
}

function clearToken() {
  localStorage.removeItem(TOKEN_KEY);
}

// rawRequest returns the full JSON response without unwrapping.
async function rawRequest<T>(path: string, options?: RequestInit): Promise<T> {
  const token = getToken();
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...(options?.headers as Record<string, string> || {}),
  };
  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }
  const res = await fetch(path, { ...options, headers });
  if (res.status === 401) {
    clearToken();
    window.location.reload();
    throw new Error('Unauthorized');
  }
  if (!res.ok) {
    throw new Error(`HTTP ${res.status}`);
  }
  return res.json() as Promise<T>;
}

// request returns the unwrapped data field from the standard {ok, data, error} envelope.
async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const resp = await rawRequest<ApiResponse<T>>(path, options);
  if (!resp.ok) {
    throw new Error(resp.error || 'Request failed');
  }
  return resp.data as T;
}

export function useApi() {
  const [token, _setToken] = useState<string | null>(getToken());

  const login = useCallback(async (password: string) => {
    const data = await request<{ token: string }>('/api/login', {
      method: 'POST',
      body: JSON.stringify({ password }),
    });
    setToken(data.token);
    _setToken(data.token);
    return data;
  }, []);

  const getPositions = useCallback(() => {
    return request<Position[]>('/api/positions');
  }, []);

  const getHistory = useCallback((limit = 50) => {
    return request<Position[]>(`/api/history?limit=${limit}`);
  }, []);

  const getOpportunities = useCallback(() => {
    return request<Opportunity[]>('/api/opportunities');
  }, []);

  const getStats = useCallback(() => {
    return request<Stats>('/api/stats');
  }, []);

  const getConfig = useCallback(() => {
    return request<Record<string, unknown>>('/api/config');
  }, []);

  const updateConfig = useCallback((data: Record<string, unknown>) => {
    return request<Record<string, unknown>>('/api/config', {
      method: 'PUT',
      body: JSON.stringify(data),
    });
  }, []);

  const closePosition = useCallback(async (positionId: string) => {
    const token = getToken();
    const headers: Record<string, string> = { 'Content-Type': 'application/json' };
    if (token) headers['Authorization'] = `Bearer ${token}`;
    const res = await fetch('/api/positions/close', {
      method: 'POST',
      headers,
      body: JSON.stringify({ position_id: positionId }),
    });
    if (!res.ok) {
      const body = await res.json().catch(() => null) as ApiResponse<unknown> | null;
      throw new Error(body?.error || `Close failed (${res.status})`);
    }
  }, []);

  const openPosition = useCallback(async (symbol: string, longExchange: string, shortExchange: string, force = false) => {
    const token = getToken();
    const headers: Record<string, string> = { 'Content-Type': 'application/json' };
    if (token) headers['Authorization'] = `Bearer ${token}`;
    const res = await fetch('/api/positions/open', {
      method: 'POST',
      headers,
      body: JSON.stringify({ symbol, long_exchange: longExchange, short_exchange: shortExchange, force }),
    });
    if (res.status === 401) {
      clearToken();
      window.location.reload();
      throw new Error('Unauthorized');
    }
    if (!res.ok) {
      const body = await res.json().catch(() => null) as ApiResponse<unknown> | null;
      throw new Error(body?.error || `Open failed (${res.status})`);
    }
  }, []);

  const getExchanges = useCallback(() => {
    return request<ExchangeInfo[]>('/api/exchanges');
  }, []);

  const transfer = useCallback((data: { from: string; to: string; coin: string; chain: string; amount: string }) => {
    return rawRequest<ApiResponse<TransferRecord>>('/api/transfer', {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }, []);

  const getTransfers = useCallback((limit = 50) => {
    return rawRequest<ApiResponse<TransferRecord[]>>(`/api/transfers?limit=${limit}`);
  }, []);

  const getAddresses = useCallback(() => {
    return rawRequest<ApiResponse<Record<string, Record<string, string>>>>('/api/addresses');
  }, []);

  const updateAddresses = useCallback((data: Record<string, Record<string, string>>) => {
    return request<Record<string, Record<string, string>>>('/api/addresses', {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }, []);

  const getLogs = useCallback((limit = 200) => {
    return request<LogEntry[]>(`/api/logs?limit=${limit}`);
  }, []);

  const getRejections = useCallback(() => {
    return request<RejectedOpportunity[]>('/api/rejections');
  }, []);

  const diagnose = useCallback(() => {
    return request<{ analysis: string }>('/api/diagnose', { method: 'POST' });
  }, []);

  const checkUpdate = useCallback(() => {
    return request<{ currentVersion: string; latestVersion: string; hasUpdate: boolean; changelog: string }>('/api/check-update');
  }, []);

  const performUpdate = useCallback(() => {
    return request<{ output: string }>('/api/update', { method: 'POST' });
  }, []);

  const logout = useCallback(() => {
    clearToken();
    _setToken(null);
  }, []);

  return {
    token,
    login,
    logout,
    getPositions,
    getHistory,
    getOpportunities,
    getStats,
    getConfig,
    updateConfig,
    closePosition,
    openPosition,
    getExchanges,
    transfer,
    getTransfers,
    getAddresses,
    updateAddresses,
    getLogs,
    getRejections,
    diagnose,
    checkUpdate,
    performUpdate,
  };
}
