import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import {
  Server, HardDrive, Activity, ArrowDown, ArrowUp, Loader2, ServerOff,
  Wifi, WifiOff,
} from "lucide-react";
import { getNodes, getSettings, type Node, type Settings } from "../api/client";
import { useWebSocket } from "../hooks/useWebSocket";
import { useI18n } from "../i18n/useI18n";

function formatBytes(bytes: number, decimals = 1): string {
  if (!bytes || bytes === 0) return "0 B";
  const k = 1024;
  const sizes = ["B", "KB", "MB", "GB", "TB", "PB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(decimals)) + " " + sizes[i];
}

function formatUptime(seconds: number): string {
  if (!seconds) return "-";
  const d = Math.floor(seconds / 86400);
  const h = Math.floor((seconds % 86400) / 3600);
  if (d > 0) return `${d}d ${h}h`;
  const m = Math.floor((seconds % 3600) / 60);
  if (h > 0) return `${h}h ${m}m`;
  return `${m}m`;
}

function parseTags(tags: string): string[] {
  try {
    const arr = JSON.parse(tags);
    return Array.isArray(arr) ? arr : [];
  } catch {
    return [];
  }
}

function ProgressBar({ value, color }: { value: number; color: string }) {
  return (
    <div className="h-1.5 overflow-hidden rounded-full bg-border">
      <div
        className="h-full rounded-full transition-all duration-500"
        style={{ width: `${Math.min(value, 100)}%`, backgroundColor: color }}
      />
    </div>
  );
}

export default function Dashboard() {
  const navigate = useNavigate();
  const { tx } = useI18n();
  const [nodes, setNodes] = useState<Node[]>([]);
  const [settings, setSettings] = useState<Settings | null>(null);
  const [loading, setLoading] = useState(true);
  const { nodeData, connected, statusVersion } = useWebSocket();

  useEffect(() => {
    Promise.all([getNodes(), getSettings()])
      .then(([nodeDataRes, settingsRes]) => {
        setNodes(Array.isArray(nodeDataRes) ? nodeDataRes : []);
        setSettings(settingsRes || null);
      })
      .catch(() => {
        setNodes([]);
        setSettings(null);
      })
      .finally(() => setLoading(false));
  }, [statusVersion]);

  const onlineCount = nodes.filter((n) => n.status === "online").length;
  const totalStorage = nodes.reduce((s, n) => s + (n.disk_total || 0), 0);

  if (loading) {
    return (
      <div className="flex items-center justify-center h-[60vh]">
        <Loader2 className="h-6 w-6 animate-spin text-accent" />
      </div>
    );
  }

  if (nodes.length === 0) {
    return (
      <div className="flex h-[60vh] flex-col items-center justify-center text-text-muted">
        <ServerOff className="w-12 h-12 mb-3" />
        <p className="text-lg font-medium">{tx("暂无节点", "No nodes found")}</p>
        <p className="mt-1 text-sm">
          {tx("请先连接 Agent 节点", "Connect an agent to get started")}
        </p>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Stats bar */}
      <div className="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-4 gap-4">
        {[
          { label: tx("节点总数", "Total Nodes"), value: String(nodes.length), icon: Server, color: "#3b82f6" },
          { label: tx("在线节点", "Online"), value: String(onlineCount), icon: Activity, color: "#22c55e" },
          { label: tx("总存储", "Total Storage"), value: formatBytes(totalStorage), icon: HardDrive, color: "#f59e0b" },
          { label: tx("托管文件", "Managed Files"), value: String(settings?.file_count || 0), icon: HardDrive, color: "#14b8a6" },
        ].map((s) => (
          <div key={s.label} className="flex items-center gap-4 rounded-xl border border-border bg-card p-4">
            <div
              className="flex h-10 w-10 items-center justify-center rounded-lg"
              style={{ backgroundColor: s.color + "15" }}
            >
              <s.icon className="h-5 w-5" style={{ color: s.color }} />
            </div>
            <div>
              <p className="text-xs text-text-muted">{s.label}</p>
              <p className="text-xl font-semibold text-text-primary">{s.value}</p>
            </div>
          </div>
        ))}
      </div>

      {/* WebSocket indicator */}
      <div className="flex items-center gap-2 text-xs text-text-muted">
        {connected ? (
          <><Wifi className="h-3 w-3 text-online" /> {tx("实时连接已建立", "Real-time connected")}</>
        ) : (
          <><WifiOff className="h-3 w-3 text-offline" /> {tx("实时连接已断开", "Real-time disconnected")}</>
        )}
      </div>

      {/* Node cards grid */}
      <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
        {nodes.map((node, i) => {
          const live = nodeData.get(node.uuid);
          const metric = node.latest_metric;
          const cpu = live?.metrics.cpu_percent ?? metric?.cpu_percent ?? 0;
          const ram = live?.metrics.mem_percent ?? metric?.mem_percent ?? 0;
          const disk = live?.metrics.disk_percent ?? metric?.disk_percent ?? 0;
          const netIn = live?.metrics.net_in_speed ?? metric?.net_in_speed ?? 0;
          const netOut = live?.metrics.net_out_speed ?? metric?.net_out_speed ?? 0;
          const uptime = live?.metrics.uptime ?? metric?.uptime ?? 0;
          const isOnline = node.status === "online";
          const tags = parseTags(node.tags);

          return (
            <div
              key={node.uuid}
              onClick={() => navigate(`/nodes/${node.uuid}`)}
              className="cursor-pointer rounded-xl border border-border bg-card p-5 transition-all duration-200 hover:border-accent/40 animate-[fadeIn_0.3s_ease-out_both]"
              style={{ animationDelay: `${i * 50}ms` }}
            >
              {/* Header */}
              <div className="flex items-start justify-between mb-3">
                <div className="min-w-0">
                  <h3 className="truncate font-semibold text-text-primary">{node.hostname}</h3>
                  <div className="flex items-center gap-2 mt-0.5">
                    <span className="text-xs text-text-muted">{node.ip || node.region}</span>
                    <span className={`h-1.5 w-1.5 rounded-full ${isOnline ? "bg-online" : "bg-text-muted"}`} />
                    <span className="text-xs text-text-muted">{node.status}</span>
                  </div>
                </div>
                <span className="shrink-0 rounded bg-border px-1.5 py-0.5 text-[10px] text-text-muted">
                  {node.os}/{node.arch}
                </span>
              </div>

              {/* Tags */}
              {tags.length > 0 && (
                <div className="flex flex-wrap gap-1 mb-3">
                  {tags.map((tag) => (
                    <span key={tag} className="rounded bg-accent/10 px-1.5 py-0.5 text-[10px] text-accent">
                      {tag}
                    </span>
                  ))}
                </div>
              )}

              {/* Metrics bars */}
              <div className="space-y-2.5 mb-3">
                {[
                  { label: "CPU", value: cpu, color: "#3b82f6" },
                  { label: "RAM", value: ram, color: "#22c55e" },
                  { label: "Disk", value: disk, color: "#f59e0b" },
                ].map((m) => (
                  <div key={m.label}>
                    <div className="flex justify-between text-xs mb-1">
                      <span className="text-text-muted">{m.label}</span>
                      <span className="text-text-secondary">{m.value.toFixed(1)}%</span>
                    </div>
                    <ProgressBar value={m.value} color={m.color} />
                  </div>
                ))}
              </div>

              {/* Footer */}
              <div className="flex items-center justify-between border-t border-border pt-2 text-xs text-text-muted">
                <div className="flex items-center gap-3">
                  <span className="flex items-center gap-1">
                    <ArrowDown className="w-3 h-3" /> {formatBytes(netIn)}/s
                  </span>
                  <span className="flex items-center gap-1">
                    <ArrowUp className="w-3 h-3" /> {formatBytes(netOut)}/s
                  </span>
                </div>
                <span className="text-[10px]">
                  {tx("运行", "up")} {formatUptime(uptime)}
                </span>
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}
