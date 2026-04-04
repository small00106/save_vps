import { useEffect, useState, useCallback } from "react";

export interface HeartbeatData {
  cpu_percent: number;
  mem_percent: number;
  disk_percent: number;
  net_in_speed: number;
  net_out_speed: number;
  uptime: number;
}

export interface NodeRealtimeData {
  uuid: string;
  status: string;
  metrics: HeartbeatData;
}

// Singleton state shared across all hook consumers
let globalData = new Map<string, NodeRealtimeData>();
let globalConnected = false;
let globalWs: WebSocket | null = null;
let globalRetry = 0;
let globalTimer: ReturnType<typeof setTimeout> | undefined;
const globalListeners = new Set<() => void>();
let globalDestroyed = false;
let globalStatusVersion = 0; // bumped on status changes so consumers can re-fetch

function notifyAll() {
  globalListeners.forEach((fn) => fn());
}

function connectGlobal() {
  if (globalDestroyed) return;
  if (globalWs?.readyState === WebSocket.OPEN) return;

  const proto = location.protocol === "https:" ? "wss:" : "ws:";
  const ws = new WebSocket(`${proto}//${location.host}/api/ws/dashboard`);
  globalWs = ws;

  ws.onopen = () => {
    globalConnected = true;
    globalRetry = 0;
    notifyAll();
  };

  ws.onmessage = (event) => {
    try {
      const msg = JSON.parse(event.data);
      if (msg.type === "heartbeat" && msg.node && msg.data) {
        const next = new Map(globalData);
        next.set(msg.node, {
          uuid: msg.node,
          status: "online",
          metrics: {
            cpu_percent: msg.data.cpu_percent ?? 0,
            mem_percent: msg.data.mem_percent ?? 0,
            disk_percent: msg.data.disk_percent ?? 0,
            net_in_speed: msg.data.net_in_speed ?? 0,
            net_out_speed: msg.data.net_out_speed ?? 0,
            uptime: msg.data.uptime ?? 0,
          },
        });
        globalData = next;
        notifyAll();
      } else if (msg.type === "status" && msg.node && msg.status) {
        const next = new Map(globalData);
        if (msg.status === "offline") {
          next.delete(msg.node);
        } else {
          const existing = next.get(msg.node);
          if (existing) {
            next.set(msg.node, { ...existing, status: msg.status });
          }
        }
        globalData = next;
        globalStatusVersion++;
        notifyAll();
      }
    } catch {
      // ignore
    }
  };

  ws.onclose = () => {
    globalConnected = false;
    globalWs = null;
    notifyAll();
    if (!globalDestroyed && globalListeners.size > 0) {
      const delay = Math.min(1000 * 2 ** globalRetry, 30000);
      globalRetry += 1;
      globalTimer = setTimeout(connectGlobal, delay);
    }
  };

  ws.onerror = () => ws.close();
}

/**
 * Singleton WebSocket hook. Multiple components can call this;
 * only one connection is ever created.
 */
export function useWebSocket() {
  const [, forceUpdate] = useState(0);

  const listener = useCallback(() => {
    forceUpdate((n) => n + 1);
  }, []);

  useEffect(() => {
    globalDestroyed = false;
    globalListeners.add(listener);

    // Start connection if this is the first consumer
    if (globalListeners.size === 1 && !globalWs) {
      connectGlobal();
    }

    return () => {
      globalListeners.delete(listener);
      // If no more consumers, tear down
      if (globalListeners.size === 0) {
        globalDestroyed = true;
        clearTimeout(globalTimer);
        globalWs?.close();
        globalWs = null;
      }
    };
  }, [listener]);

  return {
    nodeData: globalData,
    connected: globalConnected,
    statusVersion: globalStatusVersion,
  };
}
