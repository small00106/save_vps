import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import {
  Server, HardDrive, Activity, ArrowDown, ArrowUp, Loader2, ServerOff,
  Wifi, WifiOff,
} from "lucide-react";
import { getNodes, getSettings, type Node, type Settings } from "../api/client";
import { useWebSocket } from "../hooks/useWebSocket";
import { useI18n } from "../i18n/useI18n";
import { useCardGlow } from "../hooks/useMouseGlow";

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

function ProgressBar({ value, colorStart, colorEnd }: { value: number; colorStart: string; colorEnd: string }) {
  return (
    <div className="h-1.5 overflow-hidden rounded-full bg-border/50">
      <div
        className="h-full rounded-full transition-all duration-500"
        style={{ 
          width: `${Math.min(value, 100)}%`, 
          background: `linear-gradient(90deg, ${colorStart}, ${colorEnd})`,
          boxShadow: `0 0 8px ${colorStart}40`,
        }}
      />
    </div>
  );
}

function StatCard({ label, value, icon: Icon, gradient }: { 
  label: string; 
  value: string; 
  icon: typeof Server; 
  gradient: string;
}) {
  const { onMouseMove } = useCardGlow();

  return (
    <div 
      className="card-glow relative flex items-center gap-4 rounded-xl glass-card p-4 overflow-hidden"
      onMouseMove={onMouseMove}
    >
      <div
        className="flex h-12 w-12 items-center justify-center rounded-xl text-white"
        style={{ 
          background: gradient,
          boxShadow: `0 4px 15px ${gradient.includes("#7c3aed") ? "var(--ui-accent-glow)" : "rgba(0,0,0,0.1)"}`,
        }}
      >
        <Icon className="h-6 w-6" />
      </div>
      <div className="relative z-10">
        <p className="text-xs font-medium text-text-muted">{label}</p>
        <p className="text-2xl font-bold text-text-primary">{value}</p>
      </div>
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
  const { onMouseMove } = useCardGlow();

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
        <div className="flex flex-col items-center gap-3">
          <Loader2 className="h-8 w-8 animate-spin text-accent" />
          <span className="text-sm text-text-muted">{tx("加载中...", "Loading...")}</span>
        </div>
      </div>
    );
  }

  if (nodes.length === 0) {
    return (
      <div className="flex h-[60vh] flex-col items-center justify-center text-text-muted">
        <div 
          className="mb-4 flex h-20 w-20 items-center justify-center rounded-2xl"
          style={{
            background: "linear-gradient(135deg, var(--ui-accent-muted), var(--ui-accent-secondary))",
            opacity: 0.3,
          }}
        >
          <ServerOff className="w-10 h-10" />
        </div>
        <p className="text-lg font-semibold text-text-primary">{tx("暂无节点", "No nodes found")}</p>
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
        <StatCard 
          label={tx("节点总数", "Total Nodes")} 
          value={String(nodes.length)} 
          icon={Server} 
          gradient="linear-gradient(135deg, #7c3aed, #3b82f6)" 
        />
        <StatCard 
          label={tx("在线节点", "Online")} 
          value={String(onlineCount)} 
          icon={Activity} 
          gradient="linear-gradient(135deg, #10b981, #34d399)" 
        />
        <StatCard 
          label={tx("总存储", "Total Storage")} 
          value={formatBytes(totalStorage)} 
          icon={HardDrive} 
          gradient="linear-gradient(135deg, #f59e0b, #fbbf24)" 
        />
        <StatCard 
          label={tx("托管文件", "Managed Files")} 
          value={String(settings?.file_count || 0)} 
          icon={HardDrive} 
          gradient="linear-gradient(135deg, #ec4899, #f472b6)" 
        />
      </div>

      {/* WebSocket indicator */}
      <div className="flex items-center gap-2 text-xs text-text-muted">
        {connected ? (
          <>
            <span className="relative">
              <Wifi className="h-3.5 w-3.5 text-online" />
              <span className="absolute inset-0 animate-ping">
                <Wifi className="h-3.5 w-3.5 text-online opacity-50" />
              </span>
            </span>
            {tx("实时连接已建立", "Real-time connected")}
          </>
        ) : (
          <>
            <WifiOff className="h-3.5 w-3.5 text-offline" />
            {tx("实时连接已断开", "Real-time disconnected")}
          </>
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
              onMouseMove={onMouseMove}
              className="card-glow cursor-pointer rounded-xl glass-card p-5 transition-all duration-300 hover:scale-[1.02] hover:shadow-lg animate-slide-up"
              style={{ animationDelay: `${i * 60}ms` }}
            >
              {/* Header */}
              <div className="relative z-10 flex items-start justify-between mb-4">
                <div className="min-w-0">
                  <h3 className="truncate font-semibold text-text-primary text-lg">{node.hostname}</h3>
                  <div className="flex items-center gap-2 mt-1">
                    <span className="text-xs text-text-muted">{node.ip || node.region}</span>
                    <span className="flex items-center gap-1.5">
                      <span 
                        className={`h-2 w-2 rounded-full ${isOnline ? "bg-online" : "bg-text-muted"}`}
                        style={isOnline ? { boxShadow: "0 0 8px var(--ui-online)" } : {}}
                      />
                      <span className={`text-xs ${isOnline ? "text-online" : "text-text-muted"}`}>
                        {node.status}
                      </span>
                    </span>
                  </div>
                </div>
                <span className="shrink-0 rounded-lg bg-accent/10 px-2 py-1 text-[10px] font-medium text-accent">
                  {node.os}/{node.arch}
                </span>
              </div>

              {/* Tags */}
              {tags.length > 0 && (
                <div className="relative z-10 flex flex-wrap gap-1.5 mb-4">
                  {tags.map((tag) => (
                    <span 
                      key={tag} 
                      className="rounded-full bg-accent-secondary/10 px-2 py-0.5 text-[10px] font-medium text-accent-secondary"
                    >
                      {tag}
                    </span>
                  ))}
                </div>
              )}

              {/* Metrics bars */}
              <div className="relative z-10 space-y-3 mb-4">
                {[
                  { label: "CPU", value: cpu, colorStart: "#7c3aed", colorEnd: "#3b82f6" },
                  { label: "RAM", value: ram, colorStart: "#10b981", colorEnd: "#34d399" },
                  { label: "Disk", value: disk, colorStart: "#f59e0b", colorEnd: "#fbbf24" },
                ].map((m) => (
                  <div key={m.label}>
                    <div className="flex justify-between text-xs mb-1">
                      <span className="text-text-muted font-medium">{m.label}</span>
                      <span className="text-text-primary font-semibold">{m.value.toFixed(1)}%</span>
                    </div>
                    <ProgressBar value={m.value} colorStart={m.colorStart} colorEnd={m.colorEnd} />
                  </div>
                ))}
              </div>

              {/* Footer */}
              <div className="relative z-10 flex items-center justify-between border-t border-border/50 pt-3 text-xs text-text-muted">
                <div className="flex items-center gap-4">
                  <span className="flex items-center gap-1">
                    <ArrowDown className="w-3 h-3 text-accent-secondary" /> {formatBytes(netIn)}/s
                  </span>
                  <span className="flex items-center gap-1">
                    <ArrowUp className="w-3 h-3 text-accent-tertiary" /> {formatBytes(netOut)}/s
                  </span>
                </div>
                <span className="text-text-muted">
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
