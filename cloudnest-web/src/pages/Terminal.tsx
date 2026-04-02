import { useEffect, useRef } from "react";
import { useParams, useNavigate } from "react-router-dom";
import { ArrowLeft } from "lucide-react";
import { Terminal as XTerminal } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import "@xterm/xterm/css/xterm.css";

export default function Terminal() {
  const { uuid } = useParams<{ uuid: string }>();
  const navigate = useNavigate();
  const termRef = useRef<HTMLDivElement>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const xtermRef = useRef<XTerminal | null>(null);

  useEffect(() => {
    if (!termRef.current || !uuid) return;

    const term = new XTerminal({
      cursorBlink: true,
      fontSize: 14,
      fontFamily: "'JetBrains Mono', 'Fira Code', monospace",
      theme: {
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
      },
    });
    const fitAddon = new FitAddon();
    term.loadAddon(fitAddon);
    term.open(termRef.current);
    fitAddon.fit();
    xtermRef.current = term;

    term.writeln("\x1b[36mConnecting to agent...\x1b[0m");

    // Connect WebSocket
    const proto = location.protocol === "https:" ? "wss:" : "ws:";
    const ws = new WebSocket(`${proto}//${location.host}/api/ws/terminal/${uuid}`);
    wsRef.current = ws;

    ws.onopen = () => {
      term.writeln("\x1b[32mConnected.\x1b[0m\r\n");
    };

    ws.onmessage = (event) => {
      term.write(event.data);
    };

    ws.onclose = () => {
      term.writeln("\r\n\x1b[31mDisconnected.\x1b[0m");
    };

    ws.onerror = () => {
      term.writeln("\r\n\x1b[31mConnection error.\x1b[0m");
    };

    // Send terminal input to WebSocket
    term.onData((data) => {
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(data);
      }
    });

    // Handle resize
    const handleResize = () => fitAddon.fit();
    window.addEventListener("resize", handleResize);

    return () => {
      window.removeEventListener("resize", handleResize);
      ws.close();
      term.dispose();
    };
  }, [uuid]);

  return (
    <div className="space-y-4 animate-[fadeIn_0.3s_ease-out]">
      <div className="flex items-center gap-3">
        <button
          onClick={() => navigate(-1)}
          className="p-1.5 rounded-lg hover:bg-[#27272a] transition-colors"
        >
          <ArrowLeft className="w-4 h-4 text-[#a1a1aa]" />
        </button>
        <h1 className="text-xl font-bold text-[#fafafa]">Terminal</h1>
        <span className="text-xs text-[#71717a] font-mono">{uuid?.slice(0, 12)}...</span>
      </div>
      <div
        ref={termRef}
        className="bg-[#09090b] border border-[#27272a] rounded-xl p-2 h-[calc(100vh-12rem)]"
      />
    </div>
  );
}
