import { useState, useEffect, useRef, useCallback } from 'react';
import type { Position, Opportunity, Stats, Alert, LogEntry, RejectedOpportunity, SpotPosition, SpotOpportunity } from '../types.ts';

interface WsMessage {
  type: string;
  data: unknown;
}

export function useWebSocket(enabled: boolean) {
  const [connected, setConnected] = useState(false);
  const [positions, setPositions] = useState<Position[]>([]);
  const [opportunities, setOpportunities] = useState<Opportunity[]>([]);
  const [alerts, setAlerts] = useState<Alert[]>([]);
  const [stats, setStats] = useState<Stats | null>(null);
  const [logs, setLogs] = useState<LogEntry[]>([]);
  const [rejections, setRejections] = useState<RejectedOpportunity[]>([]);
  const [spotPositions, setSpotPositions] = useState<SpotPosition[]>([]);
  const [spotOpportunities, setSpotOpportunities] = useState<SpotOpportunity[]>([]);
  const wsRef = useRef<WebSocket | null>(null);
  const reconnectTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  const connect = useCallback(() => {
    if (wsRef.current) {
      wsRef.current.close();
    }

    const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:';
    const token = localStorage.getItem('arb_token') || '';
    const ws = new WebSocket(`${protocol}//${location.host}/ws?token=${token}`);
    wsRef.current = ws;

    ws.onopen = () => {
      setConnected(true);
    };

    ws.onclose = () => {
      setConnected(false);
      wsRef.current = null;
      reconnectTimer.current = setTimeout(() => {
        if (enabled) connect();
      }, 3000);
    };

    ws.onerror = () => {
      ws.close();
    };

    ws.onmessage = (event) => {
      // The server may batch multiple JSON messages separated by newlines.
      const parts = event.data.split('\n');
      for (const part of parts) {
        const trimmed = part.trim();
        if (!trimmed) continue;
        try {
          const msg = JSON.parse(trimmed) as WsMessage;
          switch (msg.type) {
            case 'positions':
              setPositions((msg.data as Position[]) || []);
              break;
            case 'position_update':
              // Single position update — merge into the list.
              setPositions((prev) => {
                const updated = msg.data as Position;
                if (!updated || !updated.id) return prev;
                const idx = prev.findIndex((p) => p.id === updated.id);
                if (idx >= 0) {
                  const next = [...prev];
                  next[idx] = updated;
                  return next;
                }
                return [...prev, updated];
              });
              break;
            case 'opportunities':
              setOpportunities((msg.data as Opportunity[]) || []);
              break;
            case 'stats':
              setStats(msg.data as Stats);
              break;
            case 'alert':
              setAlerts((prev) => [msg.data as Alert, ...prev].slice(0, 50));
              break;
            case 'log':
              setLogs((prev) => [...prev, msg.data as LogEntry].slice(-500));
              break;
            case 'rejection':
              setRejections((prev) => [...prev, msg.data as RejectedOpportunity].slice(-500));
              break;
            case 'spot_positions':
              setSpotPositions((msg.data as SpotPosition[]) || []);
              break;
            case 'spot_position_update':
              setSpotPositions((prev) => {
                const updated = msg.data as SpotPosition;
                if (!updated || !updated.id) return prev;
                const idx = prev.findIndex((p) => p.id === updated.id);
                if (idx >= 0) {
                  const next = [...prev];
                  next[idx] = updated;
                  return next;
                }
                return [...prev, updated];
              });
              break;
            case 'spot_position_health':
              setSpotPositions((prev) => {
                const updated = msg.data as SpotPosition;
                if (!updated || !updated.id) return prev;
                const idx = prev.findIndex((p) => p.id === updated.id);
                if (idx >= 0) {
                  const next = [...prev];
                  next[idx] = { ...next[idx], ...updated };
                  return next;
                }
                return prev;
              });
              break;
            case 'spot_opportunities':
              setSpotOpportunities((msg.data as SpotOpportunity[]) || []);
              break;
          }
        } catch {
          // ignore parse errors
        }
      }
    };
  }, [enabled]);

  useEffect(() => {
    if (enabled) {
      connect();
    }
    return () => {
      if (reconnectTimer.current) {
        clearTimeout(reconnectTimer.current);
      }
      if (wsRef.current) {
        wsRef.current.close();
      }
    };
  }, [enabled, connect]);

  return { connected, positions, setPositions, opportunities, setOpportunities, alerts, stats, setStats, logs, setLogs, rejections, setRejections, spotPositions, setSpotPositions, spotOpportunities, setSpotOpportunities };
}
