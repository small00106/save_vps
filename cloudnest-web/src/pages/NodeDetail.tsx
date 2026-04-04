import { useEffect, useMemo, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import {
  Loader2, Folder, FileText, ChevronRight, Download, ArrowLeft,
} from "lucide-react";
import {
  getNode,
  getNodeMetrics,
  getNodeFiles,
  getNodeDownloadURL,
  getNodeTraffic,
  updateNodeTags,
  execCommand,
  getCommandTask,
  type CommandTask,
  type FileEntry,
  type Node,
} from "../api/client";
import {
  ResponsiveContainer, LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, Legend,
} from "recharts";

function formatBytes(bytes: number, decimals = 1): string {
  if (!bytes || bytes === 0) return "0 B";
  const k = 1024;
  const sizes = ["B", "KB", "MB", "GB", "TB", "PB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(decimals)) + " " + sizes[i];
}

function parseTagsToInput(tags: string): string {
  try {
    const arr = JSON.parse(tags);
    if (Array.isArray(arr)) {
      return arr.map((x) => String(x)).join(", ");
    }
  } catch {
    // ignore
  }
  return "";
}

type MetricPoint = {
  timestamp?: string;
  bucket_time?: string;
  cpu_percent: number;
  mem_percent: number;
  disk_percent: number;
};

const RANGES = ["1h", "4h", "24h", "7d"] as const;

export default function NodeDetail() {
  const { uuid } = useParams<{ uuid: string }>();
  const navigate = useNavigate();
  const [node, setNode] = useState<Node | null>(null);
  const [metrics, setMetrics] = useState<MetricPoint[]>([]);
  const [range, setRange] = useState<string>("1h");
  const [tab, setTab] = useState<"metrics" | "files">("metrics");
  const [loading, setLoading] = useState(true);
  const [metricsLoading, setMetricsLoading] = useState(false);

  const [currentPath, setCurrentPath] = useState("/");
  const [files, setFiles] = useState<FileEntry[]>([]);
  const [filesLoading, setFilesLoading] = useState(false);

  const [tagsInput, setTagsInput] = useState("");
  const [savingTags, setSavingTags] = useState(false);
  const [traffic, setTraffic] = useState<{
    net_in_total: number;
    net_out_total: number;
    net_in_speed: number;
    net_out_speed: number;
  } | null>(null);
  const [command, setCommand] = useState("");
  const [runningCommand, setRunningCommand] = useState(false);
  const [commandTask, setCommandTask] = useState<CommandTask | null>(null);

  useEffect(() => {
    if (!uuid) return;
    setLoading(true);
    getNode(uuid)
      .then((res) => {
        setNode(res.node);
        setTagsInput(parseTagsToInput(res.node.tags));
      })
      .catch(() => {})
      .finally(() => setLoading(false));
  }, [uuid]);

  useEffect(() => {
    if (!uuid || tab !== "metrics") return;
    setMetricsLoading(true);
    getNodeMetrics(uuid, range)
      .then((data) => {
        const next = (Array.isArray(data) ? data : []) as MetricPoint[];
        setMetrics(next);
      })
      .catch(() => setMetrics([]))
      .finally(() => setMetricsLoading(false));
  }, [uuid, range, tab]);

  useEffect(() => {
    if (!uuid || tab !== "files") return;
    setFilesLoading(true);
    getNodeFiles(uuid, currentPath)
      .then((data) => setFiles(Array.isArray(data) ? data : []))
      .catch(() => setFiles([]))
      .finally(() => setFilesLoading(false));
  }, [uuid, currentPath, tab]);

  useEffect(() => {
    if (!uuid) return;
    getNodeTraffic(uuid)
      .then((data) => setTraffic(data))
      .catch(() => setTraffic(null));
  }, [uuid]);

  const chartData = useMemo(
    () =>
      metrics.map((m) => ({
        time: new Date(m.timestamp || m.bucket_time || "").toLocaleTimeString(),
        CPU: m.cpu_percent,
        RAM: m.mem_percent,
        Disk: m.disk_percent,
      })),
    [metrics],
  );

  const breadcrumbs = useMemo(() => {
    const parts = currentPath.split("/").filter(Boolean);
    const crumbs = [{ label: "Root", path: "/" }];
    let acc = "";
    for (const p of parts) {
      acc += "/" + p;
      crumbs.push({ label: p, path: acc });
    }
    return crumbs;
  }, [currentPath]);

  const handleFileClick = async (entry: FileEntry) => {
    if (entry.is_dir) {
      setCurrentPath(entry.path);
    } else if (uuid) {
      try {
        const res = await getNodeDownloadURL(uuid, entry.path);
        window.open(res.url, "_blank");
      } catch {
        // ignore
      }
    }
  };

  const handleSaveTags = async () => {
    if (!uuid || !node) return;
    setSavingTags(true);
    const nextTags = tagsInput
      .split(",")
      .map((t) => t.trim())
      .filter(Boolean);
    const raw = JSON.stringify(nextTags);
    try {
      await updateNodeTags(uuid, raw);
      setNode({ ...node, tags: raw });
    } catch {
      // ignore
    } finally {
      setSavingTags(false);
    }
  };

  const handleExecCommand = async () => {
    if (!uuid || !command.trim()) return;
    setRunningCommand(true);
    setCommandTask(null);
    try {
      const res = await execCommand(uuid, command.trim());
      for (let i = 0; i < 60; i += 1) {
        await new Promise((resolve) => setTimeout(resolve, 1000));
        const task = await getCommandTask(res.task_id);
        setCommandTask(task);
        if (task.status !== "running" && task.status !== "pending") {
          break;
        }
      }
    } catch {
      // ignore
    } finally {
      setRunningCommand(false);
    }
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center h-[60vh]">
        <Loader2 className="w-6 h-6 text-[#3b82f6] animate-spin" />
      </div>
    );
  }

  if (!node) {
    return (
      <div className="flex items-center justify-center h-[60vh] text-[#71717a]">
        Node not found
      </div>
    );
  }

  const isOnline = node.status === "online";

  return (
    <div className="space-y-6 animate-[fadeIn_0.3s_ease-out]">
      <div className="bg-[#18181b] border border-[#27272a] rounded-xl p-5">
        <div className="flex flex-wrap items-center gap-3 mb-2">
          <h1 className="text-xl font-bold text-[#fafafa]">{node.hostname}</h1>
          <span
            className={`px-2 py-0.5 rounded-full text-xs font-medium ${
              isOnline ? "bg-green-500/10 text-[#22c55e]" : "bg-zinc-500/10 text-[#71717a]"
            }`}
          >
            {node.status}
          </span>
        </div>
        <div className="flex flex-wrap gap-x-6 gap-y-1 text-sm text-[#a1a1aa]">
          <span>{node.ip}:{node.port}</span>
          <span>{node.os} / {node.arch}</span>
          <span>{node.cpu_model} ({node.cpu_cores} cores)</span>
          <span>RAM {formatBytes(node.ram_total)}</span>
          <span>Disk {formatBytes(node.disk_total)}</span>
        </div>
      </div>

      <div className="grid grid-cols-1 xl:grid-cols-3 gap-4">
        <div className="bg-[#18181b] border border-[#27272a] rounded-xl p-4 space-y-3">
          <p className="text-xs text-[#a1a1aa]">Node Tags (comma separated)</p>
          <input
            value={tagsInput}
            onChange={(e) => setTagsInput(e.target.value)}
            className="w-full h-9 px-3 rounded-lg bg-[#09090b] border border-[#27272a] text-white text-sm focus:outline-none focus:border-[#3b82f6] transition-colors"
            placeholder="prod,beijing,gpu"
          />
          <div className="flex items-center gap-2">
            <button
              onClick={handleSaveTags}
              disabled={savingTags}
              className="h-8 px-3 rounded-lg bg-[#3b82f6] hover:bg-blue-600 text-white text-xs font-medium transition-colors disabled:opacity-50"
            >
              {savingTags ? "Saving..." : "Save Tags"}
            </button>
            <button
              onClick={() => navigate(`/terminal/${uuid}`)}
              className="h-8 px-3 rounded-lg bg-[#18181b] border border-[#27272a] hover:bg-[#232329] text-[#fafafa] text-xs font-medium transition-colors"
            >
              Open Terminal
            </button>
          </div>
        </div>

        <div className="bg-[#18181b] border border-[#27272a] rounded-xl p-4">
          <p className="text-xs text-[#a1a1aa] mb-3">Traffic</p>
          <div className="space-y-2 text-sm">
            <div className="flex items-center justify-between">
              <span className="text-[#71717a]">Inbound Speed</span>
              <span className="text-[#fafafa]">{formatBytes(traffic?.net_in_speed || 0)}/s</span>
            </div>
            <div className="flex items-center justify-between">
              <span className="text-[#71717a]">Outbound Speed</span>
              <span className="text-[#fafafa]">{formatBytes(traffic?.net_out_speed || 0)}/s</span>
            </div>
            <div className="flex items-center justify-between">
              <span className="text-[#71717a]">Inbound Total</span>
              <span className="text-[#fafafa]">{formatBytes(traffic?.net_in_total || 0)}</span>
            </div>
            <div className="flex items-center justify-between">
              <span className="text-[#71717a]">Outbound Total</span>
              <span className="text-[#fafafa]">{formatBytes(traffic?.net_out_total || 0)}</span>
            </div>
          </div>
        </div>

        <div className="bg-[#18181b] border border-[#27272a] rounded-xl p-4 space-y-3">
          <p className="text-xs text-[#a1a1aa]">Quick Command</p>
          <input
            value={command}
            onChange={(e) => setCommand(e.target.value)}
            className="w-full h-9 px-3 rounded-lg bg-[#09090b] border border-[#27272a] text-white text-sm focus:outline-none focus:border-[#3b82f6] transition-colors"
            placeholder="uptime"
          />
          <button
            onClick={handleExecCommand}
            disabled={runningCommand || !command.trim()}
            className="h-8 px-3 rounded-lg bg-[#3b82f6] hover:bg-blue-600 text-white text-xs font-medium transition-colors disabled:opacity-50"
          >
            {runningCommand ? "Running..." : "Run"}
          </button>
          {commandTask && (
            <div className="rounded-lg border border-[#27272a] bg-[#09090b] p-2">
              <p className="text-[11px] text-[#a1a1aa] mb-1">Status: {commandTask.status}</p>
              <pre className="text-[11px] text-[#fafafa] whitespace-pre-wrap break-all max-h-24 overflow-y-auto">
                {commandTask.output || "(no output)"}
              </pre>
            </div>
          )}
        </div>
      </div>

      <div className="flex gap-1 bg-[#18181b] border border-[#27272a] rounded-lg p-1 w-fit">
        {(["metrics", "files"] as const).map((t) => (
          <button
            key={t}
            onClick={() => setTab(t)}
            className={`px-4 py-1.5 rounded-md text-sm font-medium transition-colors ${
              tab === t ? "bg-[#27272a] text-[#fafafa]" : "text-[#71717a] hover:text-[#a1a1aa]"
            }`}
          >
            {t === "metrics" ? "Metrics" : "Files"}
          </button>
        ))}
      </div>

      {tab === "metrics" && (
        <div className="space-y-4">
          <div className="flex gap-2">
            {RANGES.map((r) => (
              <button
                key={r}
                onClick={() => setRange(r)}
                className={`px-3 py-1 rounded-md text-sm transition-colors ${
                  range === r
                    ? "bg-[#3b82f6] text-white"
                    : "bg-[#18181b] border border-[#27272a] text-[#a1a1aa] hover:text-[#fafafa]"
                }`}
              >
                {r}
              </button>
            ))}
          </div>

          <div className="bg-[#18181b] border border-[#27272a] rounded-xl p-5">
            {metricsLoading ? (
              <div className="flex items-center justify-center h-64">
                <Loader2 className="w-5 h-5 text-[#3b82f6] animate-spin" />
              </div>
            ) : chartData.length === 0 ? (
              <div className="flex items-center justify-center h-64 text-[#71717a] text-sm">
                No metrics data available
              </div>
            ) : (
              <ResponsiveContainer width="100%" height={320}>
                <LineChart data={chartData}>
                  <CartesianGrid strokeDasharray="3 3" stroke="#27272a" />
                  <XAxis dataKey="time" stroke="#71717a" fontSize={11} tickLine={false} />
                  <YAxis stroke="#71717a" fontSize={11} tickLine={false} domain={[0, 100]} unit="%" />
                  <Tooltip
                    contentStyle={{
                      backgroundColor: "#18181b",
                      border: "1px solid #27272a",
                      borderRadius: 8,
                      color: "#fafafa",
                      fontSize: 12,
                    }}
                  />
                  <Legend />
                  <Line type="monotone" dataKey="CPU" stroke="#3b82f6" dot={false} strokeWidth={2} />
                  <Line type="monotone" dataKey="RAM" stroke="#22c55e" dot={false} strokeWidth={2} />
                  <Line type="monotone" dataKey="Disk" stroke="#f59e0b" dot={false} strokeWidth={2} />
                </LineChart>
              </ResponsiveContainer>
            )}
          </div>
        </div>
      )}

      {tab === "files" && (
        <div className="bg-[#18181b] border border-[#27272a] rounded-xl overflow-hidden">
          <div className="flex items-center gap-1 px-4 py-3 border-b border-[#27272a] text-sm overflow-x-auto">
            {breadcrumbs.map((crumb, i) => (
              <span key={crumb.path} className="flex items-center gap-1 shrink-0">
                {i > 0 && <ChevronRight className="w-3 h-3 text-[#71717a]" />}
                <button
                  onClick={() => setCurrentPath(crumb.path)}
                  className={`hover:text-[#3b82f6] transition-colors ${
                    i === breadcrumbs.length - 1 ? "text-[#fafafa]" : "text-[#71717a]"
                  }`}
                >
                  {crumb.label}
                </button>
              </span>
            ))}
          </div>

          {filesLoading ? (
            <div className="flex items-center justify-center h-48">
              <Loader2 className="w-5 h-5 text-[#3b82f6] animate-spin" />
            </div>
          ) : files.length === 0 ? (
            <div className="flex items-center justify-center h-48 text-[#71717a] text-sm">
              Empty directory
            </div>
          ) : (
            <div className="divide-y divide-[#27272a]">
              {currentPath !== "/" && (
                <button
                  onClick={() => {
                    const parent = currentPath.split("/").slice(0, -1).join("/") || "/";
                    setCurrentPath(parent);
                  }}
                  className="flex items-center gap-3 w-full px-4 py-2.5 hover:bg-[#232329] transition-colors text-left"
                >
                  <ArrowLeft className="w-4 h-4 text-[#71717a]" />
                  <span className="text-sm text-[#a1a1aa]">..</span>
                </button>
              )}
              {[...files]
                .sort((a, b) => (a.is_dir === b.is_dir ? a.name.localeCompare(b.name) : a.is_dir ? -1 : 1))
                .map((entry) => (
                  <button
                    key={entry.path}
                    onClick={() => handleFileClick(entry)}
                    className="flex items-center gap-3 w-full px-4 py-2.5 hover:bg-[#232329] transition-colors text-left group"
                  >
                    {entry.is_dir ? (
                      <Folder className="w-4 h-4 text-[#3b82f6] shrink-0" />
                    ) : (
                      <FileText className="w-4 h-4 text-[#71717a] shrink-0" />
                    )}
                    <span className="text-sm text-[#fafafa] truncate flex-1">{entry.name}</span>
                    {!entry.is_dir && (
                      <span className="text-xs text-[#71717a] shrink-0">{formatBytes(entry.size)}</span>
                    )}
                    {!entry.is_dir && (
                      <Download className="w-3 h-3 text-[#71717a] opacity-0 group-hover:opacity-100 transition-opacity shrink-0" />
                    )}
                  </button>
                ))}
            </div>
          )}
        </div>
      )}
    </div>
  );
}
