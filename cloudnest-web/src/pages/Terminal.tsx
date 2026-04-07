import { useEffect, useRef } from "react";
import { useParams, useNavigate } from "react-router-dom";
import { ArrowLeft } from "lucide-react";
import { Terminal as XTerminal } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import "@xterm/xterm/css/xterm.css";
import { useI18n } from "../i18n/useI18n";
import { usePreferences, type Theme } from "../contexts/PreferencesContext";

interface TerminalControlMessage {
  type: "input" | "resize";
  data?: string;
  cols?: number;
  rows?: number;
}

function getTerminalTheme(theme: Theme) {
  if (theme === "light") {
    return {
      background: "#f8fafc",
      foreground: "#0f172a",
      cursor: "#2563eb",
      selectionBackground: "#bfdbfe",
      black: "#334155",
      red: "#dc2626",
      green: "#16a34a",
      yellow: "#d97706",
      blue: "#2563eb",
      magenta: "#9333ea",
      cyan: "#0891b2",
      white: "#0f172a",
    };
  }

  return {
    background: "#09090b",
    foreground: "#fafafa",
    cursor: "#3b82f6",
    selectionBackground: "#3b82f640",
    black: "#18181b",
    red: "#ef4444",
    green: "#22c55e",
    yellow: "#f59e0b",
    blue: "#3b82f6",
    magenta: "#a855f7",
    cyan: "#06b6d4",
    white: "#fafafa",
  };
}

export default function Terminal() {
  const { uuid } = useParams<{ uuid: string }>();
  const navigate = useNavigate();
  const { tx } = useI18n();
  const { theme } = usePreferences();
  const termRef = useRef<HTMLDivElement>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const xtermRef = useRef<XTerminal | null>(null);
  const resizeTimerRef = useRef<number | null>(null);
  const sizeRef = useRef({ cols: 0, rows: 0 });

  useEffect(() => {
    if (!termRef.current || !uuid) return;

    const term = new XTerminal({
      cursorBlink: true,
      fontSize: 14,
      fontFamily: "'JetBrains Mono', 'Fira Code', monospace",
      theme: getTerminalTheme(theme),
    });
    const fitAddon = new FitAddon();
    term.loadAddon(fitAddon);
    term.open(termRef.current);
    xtermRef.current = term;

    const syncSize = (ws?: WebSocket) => {
      fitAddon.fit();
      const nextSize = { cols: term.cols, rows: term.rows };
      if (
        nextSize.cols <= 0 ||
        nextSize.rows <= 0 ||
        (
          nextSize.cols === sizeRef.current.cols &&
          nextSize.rows === sizeRef.current.rows
        )
      ) {
        return;
      }
      sizeRef.current = nextSize;
      if (ws && ws.readyState === WebSocket.OPEN) {
        const message: TerminalControlMessage = {
          type: "resize",
          cols: nextSize.cols,
          rows: nextSize.rows,
        };
        ws.send(JSON.stringify(message));
      }
    };

    syncSize();

    term.writeln(`\x1b[36m${tx("正在连接节点...", "Connecting to agent...")}\x1b[0m`);

    // Connect WebSocket
    const proto = location.protocol === "https:" ? "wss:" : "ws:";
    const ws = new WebSocket(
      `${proto}//${location.host}/api/ws/terminal/${uuid}?cols=${sizeRef.current.cols}&rows=${sizeRef.current.rows}`,
    );
    wsRef.current = ws;

    ws.onopen = () => {
      term.writeln(`\x1b[32m${tx("已连接。", "Connected.")}\x1b[0m\r\n`);
      syncSize(ws);
    };

    ws.onmessage = (event) => {
      term.write(event.data);
    };

    ws.onclose = () => {
      term.writeln(`\r\n\x1b[31m${tx("连接已断开。", "Disconnected.")}\x1b[0m`);
    };

    ws.onerror = () => {
      term.writeln(`\r\n\x1b[31m${tx("连接错误。", "Connection error.")}\x1b[0m`);
    };

    // Send terminal input to WebSocket
    term.onData((data) => {
      if (ws.readyState === WebSocket.OPEN) {
        const message: TerminalControlMessage = { type: "input", data };
        ws.send(JSON.stringify(message));
      }
    });

    // Handle resize
    const scheduleResize = () => {
      if (resizeTimerRef.current !== null) {
        window.clearTimeout(resizeTimerRef.current);
      }
      resizeTimerRef.current = window.setTimeout(() => {
        resizeTimerRef.current = null;
        syncSize(ws);
      }, 50);
    };
    const resizeObserver = new ResizeObserver(() => scheduleResize());
    resizeObserver.observe(termRef.current);
    const handleResize = () => scheduleResize();
    window.addEventListener("resize", handleResize);

    return () => {
      if (resizeTimerRef.current !== null) {
        window.clearTimeout(resizeTimerRef.current);
        resizeTimerRef.current = null;
      }
      resizeObserver.disconnect();
      window.removeEventListener("resize", handleResize);
      ws.close();
      term.dispose();
    };
  }, [tx, uuid]); // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    if (!xtermRef.current) return;
    xtermRef.current.options.theme = getTerminalTheme(theme);
  }, [theme]);

  return (
    <div className="space-y-4 animate-[fadeIn_0.3s_ease-out]">
      <div className="flex items-center gap-3">
        <button
          onClick={() => navigate(-1)}
          className="rounded-lg p-1.5 transition-colors hover:bg-border/60"
        >
          <ArrowLeft className="h-4 w-4 text-text-secondary" />
        </button>
        <h1 className="text-xl font-bold text-text-primary">{tx("终端", "Terminal")}</h1>
        <span className="font-mono text-xs text-text-muted">{uuid?.slice(0, 12)}...</span>
      </div>
      <div
        ref={termRef}
        className="h-[calc(100vh-12rem)] rounded-xl border border-border bg-card p-2"
      />
    </div>
  );
}
