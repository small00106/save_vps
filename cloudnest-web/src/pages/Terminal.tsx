import { useEffect, useRef } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { ArrowLeft } from "lucide-react";
import { Terminal as XTerminal } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import "@xterm/xterm/css/xterm.css";
import { useI18n } from "../i18n/useI18n";
import { PageHeader, SectionCard, StatusBadge } from "../components/ui";

interface TerminalControlMessage {
  type: "input" | "resize";
  data?: string;
  cols?: number;
  rows?: number;
}

function getTerminalTheme() {
  return {
    background: "#0b1220",
    foreground: "#e2e8f0",
    cursor: "#38bdf8",
    selectionBackground: "#0ea5e940",
    black: "#111827",
    red: "#f87171",
    green: "#4ade80",
    yellow: "#fbbf24",
    blue: "#60a5fa",
    magenta: "#c084fc",
    cyan: "#22d3ee",
    white: "#f8fafc",
  };
}

export default function Terminal() {
  const { uuid } = useParams<{ uuid: string }>();
  const navigate = useNavigate();
  const { tx } = useI18n();
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
      fontFamily: "'JetBrains Mono', monospace",
      theme: getTerminalTheme(),
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
        (nextSize.cols === sizeRef.current.cols && nextSize.rows === sizeRef.current.rows)
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

    const proto = location.protocol === "https:" ? "wss:" : "ws:";
    const ws = new WebSocket(`${proto}//${location.host}/api/ws/terminal/${uuid}?cols=${sizeRef.current.cols}&rows=${sizeRef.current.rows}`);
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

    term.onData((data) => {
      if (ws.readyState === WebSocket.OPEN) {
        const message: TerminalControlMessage = { type: "input", data };
        ws.send(JSON.stringify(message));
      }
    });

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
  }, [tx, uuid]);

  return (
    <div className="space-y-6">
      <PageHeader
        eyebrow={tx("远程操作", "Remote Operations")}
        title={tx("终端", "Terminal")}
        description={tx("终端容器在全站浅色主题中保留独立深色工作区，避免影响长时间命令阅读。", "The terminal keeps an isolated dark workspace within the lighter application shell for long-running command readability.")}
        actions={
          <>
            <StatusBadge tone="primary" label={uuid ? `${uuid.slice(0, 12)}...` : tx("未知节点", "Unknown node")} />
            <button type="button" onClick={() => navigate(-1)} className="inline-flex items-center gap-2 rounded-2xl border border-border bg-surface px-4 py-2.5 text-sm font-medium text-text-primary transition-colors hover:border-border-hover hover:bg-card">
              <ArrowLeft className="h-4 w-4" />
              {tx("返回", "Back")}
            </button>
          </>
        }
      />

      <SectionCard title={tx("终端会话", "Terminal Session")} description={tx("键盘输入将直接转发到节点 shell。", "Keyboard input is forwarded directly to the node shell.")}> 
        <div className="rounded-[28px] border border-slate-900 bg-[#0b1220] p-3 shadow-[inset_0_1px_0_rgba(255,255,255,0.04)]">
          <div ref={termRef} className="h-[calc(100vh-18rem)] min-h-[420px] rounded-[20px] bg-[#0b1220]" />
        </div>
      </SectionCard>
    </div>
  );
}
