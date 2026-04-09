import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import {
  Activity,
  HardDrive,
  Loader2,
  Server,
  ServerOff,
  Waves,
} from "lucide-react";
import { getNodes, getSettings, type Node, type Settings } from "../api/client";
import { useWebSocket } from "../hooks/useWebSocket";
import { useI18n } from "../i18n/useI18n";
import { getNodeDisplayName, parseNodeTags } from "../utils/nodeDisplayName";
import { EmptyState, MetricCard, PageHeader, SectionCard, StatusBadge } from "../components/ui";

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

function ProgressBar({ value }: { value: number }) {
  return (
    <div className="h-2 overflow-hidden rounded-full bg-surface-subtle">
      <div className="h-full rounded-full progress-gradient transition-all duration-300" style={{ width: `${Math.min(value, 100)}%` }} />
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
  const totalStorage = nodes.reduce((sum, node) => sum + (node.disk_total || 0), 0);

  if (loading) {
    return (
      <div className="flex items-center justify-center py-24">
        <Loader2 className="h-8 w-8 animate-spin text-accent" />
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <PageHeader
        eyebrow={tx("CloudNest 控制台", "CloudNest Console")}
        title={tx("节点总览", "Node Overview")}
        description={tx("统一查看节点健康、实时连接和托管文件规模。", "Monitor node health, live connectivity, and managed storage volume in one place.")}
        actions={
          <StatusBadge
            tone={connected ? "success" : "danger"}
            label={connected ? tx("实时连接正常", "Realtime connected") : tx("实时连接中断", "Realtime disconnected")}
          />
        }
      />

      <div className="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-4">
        <MetricCard label={tx("节点总数", "Total nodes")} value={String(nodes.length)} meta={tx("已注册到 Master 的节点数", "Nodes registered to the master")} icon={Server} tone="primary" />
        <MetricCard label={tx("在线节点", "Online nodes")} value={String(onlineCount)} meta={tx("最近心跳仍然活跃", "Nodes with recent heartbeat")} icon={Activity} tone="success" />
        <MetricCard label={tx("总存储", "Total storage")} value={formatBytes(totalStorage)} meta={tx("节点上报的磁盘总量", "Disk total reported by nodes")} icon={HardDrive} tone="warning" />
        <MetricCard label={tx("托管文件", "Managed files")} value={String(settings?.file_count || 0)} meta={tx("已接入统一检索的文件", "Files included in unified search")} icon={Waves} tone="neutral" />
      </div>

      <SectionCard
        title={tx("节点健康", "Node health")}
        description={tx("桌面端优先展示密度更高的节点概览；移动端保持浏览和进入详情可用。", "Prioritize dense overviews on desktop while keeping mobile browsing usable.")}
      >
        {nodes.length === 0 ? (
          <EmptyState
            icon={ServerOff}
            title={tx("暂无节点", "No nodes found")}
            description={tx("请先连接 Agent 节点", "Connect an agent to get started")}
          />
        ) : (
          <div className="space-y-3">
            {nodes.map((node) => {
              const live = nodeData.get(node.uuid);
              const metric = node.latest_metric;
              const cpu = live?.metrics.cpu_percent ?? metric?.cpu_percent ?? 0;
              const ram = live?.metrics.mem_percent ?? metric?.mem_percent ?? 0;
              const disk = live?.metrics.disk_percent ?? metric?.disk_percent ?? 0;
              const netIn = live?.metrics.net_in_speed ?? metric?.net_in_speed ?? 0;
              const netOut = live?.metrics.net_out_speed ?? metric?.net_out_speed ?? 0;
              const uptime = live?.metrics.uptime ?? metric?.uptime ?? 0;
              const isOnline = node.status === "online";
              const tags = parseNodeTags(node.tags);
              const displayName = getNodeDisplayName(node.hostname, node.tags);
              const showHostname = displayName !== node.hostname;

              return (
                <button
                  key={node.uuid}
                  type="button"
                  onClick={() => navigate(`/nodes/${node.uuid}`)}
                  className="w-full rounded-3xl border border-border bg-surface px-4 py-4 text-left transition-colors hover:border-border-hover hover:bg-card md:px-5"
                >
                  <div className="flex flex-col gap-4 xl:flex-row xl:items-center xl:justify-between">
                    <div className="min-w-0 flex-1 space-y-3">
                      <div className="flex flex-wrap items-center gap-3">
                        <h3 className="truncate text-lg font-semibold text-text-primary">{displayName}</h3>
                        <StatusBadge tone={isOnline ? "success" : "danger"} label={isOnline ? tx("在线", "Online") : tx("离线", "Offline")} />
                        <span className="rounded-full border border-border bg-card px-3 py-1 text-xs font-medium text-text-secondary">
                          {node.os}/{node.arch}
                        </span>
                      </div>

                      {showHostname ? (
                        <div className="truncate text-xs text-text-muted">
                          {tx("主机名", "Hostname")}: {node.hostname}
                        </div>
                      ) : null}

                      <div className="flex flex-wrap items-center gap-4 text-xs text-text-muted">
                        <span>{node.ip || node.region || "-"}</span>
                        <span>{tx("运行", "Up")} {formatUptime(uptime)}</span>
                        <span>{tx("入站", "In")} {formatBytes(netIn)}/s</span>
                        <span>{tx("出站", "Out")} {formatBytes(netOut)}/s</span>
                      </div>

                      {tags.length > 0 ? (
                        <div className="flex flex-wrap gap-2">
                          {tags.map((tag) => (
                            <span key={tag} className="rounded-full border border-border bg-card px-2.5 py-1 text-[11px] font-medium text-text-secondary">
                              {tag}
                            </span>
                          ))}
                        </div>
                      ) : null}
                    </div>

                    <div className="grid min-w-0 gap-3 md:grid-cols-3 xl:w-[420px]">
                      {[
                        { label: "CPU", value: cpu },
                        { label: "RAM", value: ram },
                        { label: "Disk", value: disk },
                      ].map((item) => (
                        <div key={item.label} className="space-y-2 rounded-2xl border border-border bg-card px-3 py-3">
                          <div className="flex items-center justify-between text-xs">
                            <span className="font-medium text-text-secondary">{item.label}</span>
                            <span className="font-semibold text-text-primary">{item.value.toFixed(1)}%</span>
                          </div>
                          <ProgressBar value={item.value} />
                        </div>
                      ))}
                    </div>
                  </div>
                </button>
              );
            })}
          </div>
        )}
      </SectionCard>
    </div>
  );
}

