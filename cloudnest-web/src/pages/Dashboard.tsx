import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import {
  Server, HardDrive, Activity, ArrowDown, ArrowUp, Loader2, ServerOff,
  Wifi, WifiOff,
} from "lucide-react";
import { getNodes, type Node } from "../api/client";
import { useWebSocket } from "../hooks/useWebSocket";

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
    <div className="h-1.5 rounded-full bg-[#27272a] overflow-hidden">
      <div
        className="h-full rounded-full transition-all duration-500"
        style={{ width: `${Math.min(value, 100)}%`, backgroundColor: color }}
      />
    </div>
  );
}

export default function Dashboard() {
  const navigate = useNavigate();
  const [nodes, setNodes] = useState<Node[]>([]);
  const [loading, setLoading] = useState(true);
  const { nodeData, connected, statusVersion } = useWebSocket();

  useEffect(() => {
    getNodes()
      .then((data) => setNodes(Array.isArray(data) ? data : []))
      .catch(() => setNodes([]))
      .finally(() => setLoading(false));
  }, [statusVersion]);

  const onlineCount = nodes.filter((n) => n.status === "online").length;
  const totalStorage = nodes.reduce((s, n) => s + (n.disk_total || 0), 0);

  if (loading) {
    return (
      <div className="flex items-center justify-center h-[60vh]">
        <Loader2 className="w-6 h-6 text-[#3b82f6] animate-spin" />
      </div>
    );
  }

  if (nodes.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center h-[60vh] text-[#71717a]">
        <ServerOff className="w-12 h-12 mb-3" />
        <p className="text-lg font-medium">No nodes found</p>
        <p className="text-sm mt-1">Connect an agent to get started</p>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Stats bar */}
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
        {[
          { label: "Total Nodes", value: String(nodes.length), icon: Server, color: "#3b82f6" },
          { label: "Online", value: String(onlineCount), icon: Activity, color: "#22c55e" },
          { label: "Total Storage", value: formatBytes(totalStorage), icon: HardDrive, color: "#f59e0b" },
        ].map((s) => (
          <div key={s.label} className="bg-[#18181b] border border-[#27272a] rounded-xl p-4 flex items-center gap-4">
            <div
              className="w-10 h-10 rounded-lg flex items-center justify-center"
              style={{ backgroundColor: s.color + "15" }}
            >
              <s.icon className="w-5 h-5" style={{ color: s.color }} />
            </div>
            <div>
              <p className="text-[#71717a] text-xs">{s.label}</p>
              <p className="text-[#fafafa] text-xl font-semibold">{s.value}</p>
            </div>
          </div>
        ))}
      </div>

      {/* WebSocket indicator */}
      <div className="flex items-center gap-2 text-xs text-[#71717a]">
        {connected ? (
          <><Wifi className="w-3 h-3 text-[#22c55e]" /> Real-time connected</>
        ) : (
          <><WifiOff className="w-3 h-3 text-[#ef4444]" /> Real-time disconnected</>
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
              className="bg-[#18181b] border border-[#27272a] rounded-xl p-5 cursor-pointer hover:border-[#3b82f6]/40 transition-all duration-200 animate-[fadeIn_0.3s_ease-out_both]"
              style={{ animationDelay: `${i * 50}ms` }}
            >
              {/* Header */}
              <div className="flex items-start justify-between mb-3">
                <div className="min-w-0">
                  <h3 className="text-[#fafafa] font-semibold truncate">{node.hostname}</h3>
                  <div className="flex items-center gap-2 mt-0.5">
                    <span className="text-[#71717a] text-xs">{node.ip || node.region}</span>
                    <span className={`w-1.5 h-1.5 rounded-full ${isOnline ? "bg-[#22c55e]" : "bg-[#71717a]"}`} />
                    <span className="text-[#71717a] text-xs">{node.status}</span>
                  </div>
                </div>
                <span className="text-[10px] text-[#71717a] bg-[#27272a] rounded px-1.5 py-0.5 shrink-0">
                  {node.os}/{node.arch}
                </span>
              </div>

              {/* Tags */}
              {tags.length > 0 && (
                <div className="flex flex-wrap gap-1 mb-3">
                  {tags.map((tag) => (
                    <span key={tag} className="px-1.5 py-0.5 rounded text-[10px] bg-[#3b82f6]/10 text-[#3b82f6]">
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
                      <span className="text-[#71717a]">{m.label}</span>
                      <span className="text-[#a1a1aa]">{m.value.toFixed(1)}%</span>
                    </div>
                    <ProgressBar value={m.value} color={m.color} />
                  </div>
                ))}
              </div>

              {/* Footer */}
              <div className="flex items-center justify-between text-xs text-[#71717a] pt-2 border-t border-[#27272a]">
                <div className="flex items-center gap-3">
                  <span className="flex items-center gap-1">
                    <ArrowDown className="w-3 h-3" /> {formatBytes(netIn)}/s
                  </span>
                  <span className="flex items-center gap-1">
                    <ArrowUp className="w-3 h-3" /> {formatBytes(netOut)}/s
                  </span>
                </div>
                <span className="text-[10px]">up {formatUptime(uptime)}</span>
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}
