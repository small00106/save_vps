import { useEffect, useMemo, useRef, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import {
  ArrowLeft,
  ChevronRight,
  Download,
  FileText,
  Folder,
  Loader2,
  RefreshCw,
  Upload,
  Terminal,
  Activity,
  HardDrive,
  Network,
  Tag,
} from "lucide-react";
import {
  ApiError,
  execCommand,
  getCommandTask,
  getNode,
  getNodeDownloadURL,
  getNodeFiles,
  getNodeMetrics,
  getNodeTraffic,
  initUpload,
  type CommandTask,
  type DownloadResponse,
  type FileEntry,
  type Node,
  type NodeUploadResponse,
  updateNodeTags,
} from "../api/client";
import { triggerDownload } from "../utils/download";
import {
  ResponsiveContainer,
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  Legend,
} from "recharts";
import { useI18n } from "../i18n/useI18n";
import { useCardGlow } from "../hooks/useMouseGlow";
import { getNodeDisplayName, parseNodeTags } from "../utils/nodeDisplayName";

function formatBytes(bytes: number, decimals = 1): string {
  if (!bytes || bytes === 0) return "0 B";
  const k = 1024;
  const sizes = ["B", "KB", "MB", "GB", "TB", "PB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(decimals)) + " " + sizes[i];
}

function parseTagsToInput(tags: string): string {
  return parseNodeTags(tags).join(", ");
}

function formatPath(path: string): string {
  return path === "/" ? "/" : path.replace(/\/+$/, "");
}

function parentPath(path: string): string {
  const normalized = formatPath(path);
  if (normalized === "/") return "/";
  const idx = normalized.lastIndexOf("/");
  return idx <= 0 ? "/" : normalized.slice(0, idx);
}

function extractDownloadFilename(res: DownloadResponse): string {
  return res.filename?.trim() ? res.filename : "download";
}

function extractUploadURL(res: NodeUploadResponse): string | null {
  if (res.url?.trim()) return res.url;
  if (res.target?.url?.trim()) return res.target.url;
  if (res.targets?.[0]?.url?.trim()) return res.targets[0].url;
  return null;
}

function uploadFileToUrl(
  url: string,
  file: File,
  onProgress: (pct: number) => void,
): Promise<void> {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    xhr.open("PUT", url, true);
    xhr.upload.onprogress = (e) => {
      if (e.lengthComputable) {
        onProgress(Math.round((e.loaded / e.total) * 100));
      }
    };
    xhr.onload = () => (xhr.status >= 200 && xhr.status < 300 ? resolve() : reject(new Error(`HTTP ${xhr.status}`)));
    xhr.onerror = () => reject(new Error("Network error"));
    xhr.send(file);
  });
}

function getErrorMessage(error: unknown): string {
  if (error instanceof Error) return error.message;
  return "Operation failed";
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
  const { tx } = useI18n();
  const [node, setNode] = useState<Node | null>(null);
  const [metrics, setMetrics] = useState<MetricPoint[]>([]);
  const [range, setRange] = useState<string>("1h");
  const [tab, setTab] = useState<"metrics" | "files">("metrics");
  const [loading, setLoading] = useState(true);
  const [metricsLoading, setMetricsLoading] = useState(false);

  const [currentPath, setCurrentPath] = useState("/");
  const [files, setFiles] = useState<FileEntry[]>([]);
  const [filesLoading, setFilesLoading] = useState(false);
  const [filesRefreshTick, setFilesRefreshTick] = useState(0);
  const [selectedFile, setSelectedFile] = useState<File | null>(null);
  const [uploading, setUploading] = useState(false);
  const [uploadProgress, setUploadProgress] = useState(0);
  const [uploadError, setUploadError] = useState("");
  const fileInputRef = useRef<HTMLInputElement>(null);

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
  
  const cardGlow = useCardGlow();

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
    setCurrentPath("/");
    setFiles([]);
    setFilesRefreshTick(0);
    setSelectedFile(null);
    setUploadProgress(0);
    setUploadError("");
    if (fileInputRef.current) {
      fileInputRef.current.value = "";
    }
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
  }, [uuid, currentPath, tab, filesRefreshTick]);

  useEffect(() => {
    if (!uuid || tab !== "files") return;
    const timer = window.setInterval(() => {
      setFilesRefreshTick((value) => value + 1);
    }, 10000);
    return () => window.clearInterval(timer);
  }, [uuid, tab]);

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
    const crumbs = [{ label: tx("根目录", "Root"), path: "/" }];
    let acc = "";
    for (const part of parts) {
      acc += "/" + part;
      crumbs.push({ label: part, path: acc });
    }
    return crumbs;
  }, [currentPath, tx]);

  const handleDownload = async (path: string) => {
    if (!uuid) return;
    const res = await getNodeDownloadURL(uuid, path);
    triggerDownload(res.url, extractDownloadFilename(res));
  };

  const handleFileClick = async (entry: FileEntry) => {
    if (entry.is_dir) {
      setCurrentPath(entry.path);
      return;
    }
    try {
      await handleDownload(entry.path);
    } catch {
      // ignore
    }
  };

  const handleRefreshFiles = () => {
    setFilesRefreshTick((value) => value + 1);
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

  const uploadToCurrentDirectory = async (overwrite: boolean) => {
    if (!uuid || !selectedFile) return;
    const res = await initUpload({
      name: selectedFile.name,
      size: selectedFile.size,
      path: currentPath,
      node_uuid: uuid,
      overwrite,
    });
    const uploadURL = extractUploadURL(res);
    if (!uploadURL) {
      throw new Error(tx("缺少上传地址", "Upload URL missing"));
    }
    await uploadFileToUrl(uploadURL, selectedFile, setUploadProgress);
  };

  const handleUpload = async () => {
    if (!selectedFile || !uuid) return;
    setUploading(true);
    setUploadError("");
    setUploadProgress(0);

    try {
      await uploadToCurrentDirectory(false);
      setSelectedFile(null);
      if (fileInputRef.current) {
        fileInputRef.current.value = "";
      }
      setFilesRefreshTick((value) => value + 1);
    } catch (error) {
      if (error instanceof ApiError && error.status === 409) {
        const confirmed = window.confirm(
          tx(
            "当前目录已存在同名文件，是否覆盖？",
            "A file with the same name already exists in this directory. Overwrite it?",
          ),
        );
        if (confirmed) {
          try {
            setUploadProgress(0);
            await uploadToCurrentDirectory(true);
            setSelectedFile(null);
            if (fileInputRef.current) {
              fileInputRef.current.value = "";
            }
            setFilesRefreshTick((value) => value + 1);
            return;
          } catch (retryError) {
            setUploadError(getErrorMessage(retryError));
            return;
          }
        }
        return;
      }
      setUploadError(getErrorMessage(error));
    } finally {
      setUploading(false);
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
        <Loader2 className="h-6 w-6 animate-spin text-accent" />
      </div>
    );
  }

  if (!node) {
    return (
      <div className="flex h-[60vh] items-center justify-center text-text-muted">
        {tx("节点不存在", "Node not found")}
      </div>
    );
  }

  const isOnline = node.status === "online";
  const displayName = getNodeDisplayName(node.hostname, node.tags);
  const showHostname = displayName !== node.hostname;

  return (
    <div className="space-y-6 animate-[fadeIn_0.3s_ease-out]">
      {/* Node Header Card */}
      <div 
        className="glass-card rounded-2xl p-6 card-glow relative overflow-hidden"
        onMouseMove={cardGlow.onMouseMove}

      >
        {/* Gradient accent line */}
        <div className="absolute top-0 left-0 right-0 h-1 bg-gradient-to-r from-purple-500 via-blue-500 to-pink-500" />
        
        <div className="flex flex-wrap items-center gap-4 mb-3">
          <h1 className="text-2xl font-bold bg-gradient-to-r from-purple-400 via-blue-400 to-pink-400 bg-clip-text text-transparent">
            {displayName}
          </h1>
          <span
            className={`px-3 py-1 rounded-full text-xs font-semibold flex items-center gap-1.5 ${
              isOnline 
                ? "bg-gradient-to-r from-emerald-500/20 to-green-500/20 text-emerald-400 border border-emerald-500/30" 
                : "bg-gray-500/20 text-gray-400 border border-gray-500/30"
            }`}
          >
            <span className={`w-2 h-2 rounded-full ${isOnline ? "bg-emerald-400 animate-pulse" : "bg-gray-500"}`} />
            {node.status}
          </span>
        </div>
        {showHostname && (
          <div className="mb-3 text-sm text-text-secondary">
            {tx("主机名", "Hostname")}: {node.hostname}
          </div>
        )}
        
        <div className="flex flex-wrap gap-x-6 gap-y-2 text-sm">
          <span className="flex items-center gap-2 text-text-secondary">
            <Network className="w-4 h-4 text-purple-400" />
            {node.ip}:{node.port}
          </span>
          <span className="flex items-center gap-2 text-text-secondary">
            <HardDrive className="w-4 h-4 text-blue-400" />
            {node.os} / {node.arch}
          </span>
          <span className="text-text-secondary">{node.cpu_model} ({node.cpu_cores} {tx("核", "cores")})</span>
          <span className="text-text-secondary">RAM {formatBytes(node.ram_total)}</span>
          <span className="text-text-secondary">{tx("磁盘", "Disk")} {formatBytes(node.disk_total)}</span>
        </div>
      </div>

      {/* Info Cards Grid */}
      <div className="grid grid-cols-1 xl:grid-cols-3 gap-4">
        {/* Tags Card */}
        <div 
          className="glass-card rounded-2xl p-5 card-glow relative overflow-hidden group"
          onMouseMove={cardGlow.onMouseMove}
  
        >
          <div className="flex items-center gap-2 mb-4">
            <div className="p-2 rounded-lg bg-gradient-to-br from-purple-500/20 to-pink-500/20">
              <Tag className="w-4 h-4 text-purple-400" />
            </div>
            <p className="text-sm font-medium text-text-primary">
              {tx("节点标签", "Node Tags")}
            </p>
          </div>
          <input
            value={tagsInput}
            onChange={(e) => setTagsInput(e.target.value)}
            className="h-10 w-full rounded-xl border border-white/10 bg-white/5 px-4 text-sm text-text-primary transition-all focus:border-purple-500/50 focus:bg-white/10 focus:outline-none focus:ring-2 focus:ring-purple-500/20"
            placeholder="prod, beijing, gpu"
          />
          <p className="mt-2 text-xs text-text-muted">
            {tx("第一个标签会作为节点显示名", "The first tag is used as the node display name")}
          </p>
          <div className="flex items-center gap-2 mt-4">
            <button
              onClick={handleSaveTags}
              disabled={savingTags}
              className="gradient-button h-9 rounded-xl px-4 text-sm font-medium text-white disabled:opacity-50"
            >
              {savingTags ? tx("保存中...", "Saving...") : tx("保存标签", "Save Tags")}
            </button>
            <button
              onClick={() => navigate(`/terminal/${uuid}`)}
              className="h-9 rounded-xl border border-white/10 bg-white/5 px-4 text-sm font-medium text-text-primary transition-all hover:bg-white/10 hover:border-purple-500/30 flex items-center gap-2"
            >
              <Terminal className="w-4 h-4" />
              {tx("终端", "Terminal")}
            </button>
          </div>
        </div>

        {/* Traffic Card */}
        <div 
          className="glass-card rounded-2xl p-5 card-glow relative overflow-hidden"
          onMouseMove={cardGlow.onMouseMove}
  
        >
          <div className="flex items-center gap-2 mb-4">
            <div className="p-2 rounded-lg bg-gradient-to-br from-blue-500/20 to-cyan-500/20">
              <Activity className="w-4 h-4 text-blue-400" />
            </div>
            <p className="text-sm font-medium text-text-primary">{tx("流量", "Traffic")}</p>
          </div>
          <div className="space-y-3 text-sm">
            <div className="flex items-center justify-between p-2 rounded-lg bg-white/5">
              <span className="text-text-muted">{tx("入站速率", "Inbound")}</span>
              <span className="text-emerald-400 font-mono">{formatBytes(traffic?.net_in_speed || 0)}/s</span>
            </div>
            <div className="flex items-center justify-between p-2 rounded-lg bg-white/5">
              <span className="text-text-muted">{tx("出站速率", "Outbound")}</span>
              <span className="text-blue-400 font-mono">{formatBytes(traffic?.net_out_speed || 0)}/s</span>
            </div>
            <div className="flex items-center justify-between">
              <span className="text-text-muted text-xs">{tx("入站总量", "In Total")}</span>
              <span className="text-text-secondary text-xs font-mono">{formatBytes(traffic?.net_in_total || 0)}</span>
            </div>
            <div className="flex items-center justify-between">
              <span className="text-text-muted text-xs">{tx("出站总量", "Out Total")}</span>
              <span className="text-text-secondary text-xs font-mono">{formatBytes(traffic?.net_out_total || 0)}</span>
            </div>
          </div>
        </div>

        {/* Quick Command Card */}
        <div 
          className="glass-card rounded-2xl p-5 card-glow relative overflow-hidden"
          onMouseMove={cardGlow.onMouseMove}
  
        >
          <div className="flex items-center gap-2 mb-4">
            <div className="p-2 rounded-lg bg-gradient-to-br from-emerald-500/20 to-green-500/20">
              <Terminal className="w-4 h-4 text-emerald-400" />
            </div>
            <p className="text-sm font-medium text-text-primary">{tx("快速命令", "Quick Command")}</p>
          </div>
          <div className="flex gap-2">
            <input
              value={command}
              onChange={(e) => setCommand(e.target.value)}
              className="h-10 flex-1 rounded-xl border border-white/10 bg-white/5 px-4 font-mono text-sm text-text-primary transition-all focus:border-emerald-500/50 focus:bg-white/10 focus:outline-none focus:ring-2 focus:ring-emerald-500/20"
              placeholder="uptime"
              onKeyDown={(e) => e.key === "Enter" && handleExecCommand()}
            />
            <button
              onClick={handleExecCommand}
              disabled={runningCommand || !command.trim()}
              className="h-10 rounded-xl bg-gradient-to-r from-emerald-500 to-green-500 px-4 text-sm font-medium text-white transition-all hover:opacity-90 disabled:opacity-50"
            >
              {runningCommand ? <Loader2 className="w-4 h-4 animate-spin" /> : tx("执行", "Run")}
            </button>
          </div>
          {commandTask && (
            <div className="mt-4 rounded-xl border border-white/10 bg-black/30 p-3 font-mono">
              <p className="mb-2 text-xs text-text-muted flex items-center gap-2">
                <span className={`w-2 h-2 rounded-full ${
                  commandTask.status === "success" ? "bg-emerald-400" : 
                  commandTask.status === "failed" ? "bg-red-400" : "bg-yellow-400 animate-pulse"
                }`} />
                {commandTask.status}
              </p>
              <pre className="max-h-32 overflow-y-auto whitespace-pre-wrap break-all text-xs text-emerald-300/80 scrollbar-thin">
                {commandTask.output || tx("(无输出)", "(no output)")}
              </pre>
            </div>
          )}
        </div>
      </div>

      {/* Tab Switcher with gradient indicator */}
      <div className="flex w-fit gap-1 rounded-2xl glass-card p-1.5">
        {(["metrics", "files"] as const).map((value) => (
          <button
            key={value}
            onClick={() => setTab(value)}
            className={`relative px-6 py-2 rounded-xl text-sm font-medium transition-all duration-300 ${
              tab === value 
                ? "text-white" 
                : "text-text-muted hover:text-text-secondary"
            }`}
          >
            {tab === value && (
              <span className="absolute inset-0 rounded-xl bg-gradient-to-r from-purple-500 via-blue-500 to-pink-500 -z-10" />
            )}
            {value === "metrics" ? tx("指标", "Metrics") : tx("文件", "Files")}
          </button>
        ))}
      </div>

      {tab === "metrics" && (
        <div className="space-y-4">
          {/* Range selector */}
          <div className="flex gap-2">
            {RANGES.map((r) => (
              <button
                key={r}
                onClick={() => setRange(r)}
                className={`px-4 py-2 rounded-xl text-sm font-medium transition-all ${
                  range === r
                    ? "bg-gradient-to-r from-purple-500 to-blue-500 text-white shadow-lg shadow-purple-500/20"
                    : "glass-card text-text-secondary hover:text-text-primary hover:border-purple-500/30"
                }`}
              >
                {r}
              </button>
            ))}
          </div>

          {/* Chart Card */}
          <div 
            className="glass-card rounded-2xl p-6 card-glow relative overflow-hidden"
            onMouseMove={cardGlow.onMouseMove}
    
          >
            {metricsLoading ? (
              <div className="flex items-center justify-center h-72">
                <div className="flex flex-col items-center gap-3">
                  <Loader2 className="h-8 w-8 animate-spin text-purple-400" />
                  <span className="text-sm text-text-muted">{tx("加载中...", "Loading...")}</span>
                </div>
              </div>
            ) : chartData.length === 0 ? (
              <div className="flex h-72 items-center justify-center">
                <div className="text-center">
                  <Activity className="w-12 h-12 mx-auto mb-3 text-text-muted/50" />
                  <p className="text-sm text-text-muted">{tx("暂无指标数据", "No metrics data available")}</p>
                </div>
              </div>
            ) : (
              <ResponsiveContainer width="100%" height={320}>
                <LineChart data={chartData}>
                  <defs>
                    <linearGradient id="cpuGradient" x1="0" y1="0" x2="1" y2="0">
                      <stop offset="0%" stopColor="#8b5cf6" />
                      <stop offset="100%" stopColor="#a78bfa" />
                    </linearGradient>
                    <linearGradient id="ramGradient" x1="0" y1="0" x2="1" y2="0">
                      <stop offset="0%" stopColor="#3b82f6" />
                      <stop offset="100%" stopColor="#60a5fa" />
                    </linearGradient>
                    <linearGradient id="diskGradient" x1="0" y1="0" x2="1" y2="0">
                      <stop offset="0%" stopColor="#ec4899" />
                      <stop offset="100%" stopColor="#f472b6" />
                    </linearGradient>
                  </defs>
                  <CartesianGrid strokeDasharray="3 3" stroke="rgba(255,255,255,0.06)" />
                  <XAxis 
                    dataKey="time" 
                    stroke="rgba(255,255,255,0.4)" 
                    fontSize={11} 
                    tickLine={false}
                    axisLine={{ stroke: 'rgba(255,255,255,0.1)' }}
                  />
                  <YAxis 
                    stroke="rgba(255,255,255,0.4)" 
                    fontSize={11} 
                    tickLine={false} 
                    domain={[0, 100]} 
                    unit="%"
                    axisLine={{ stroke: 'rgba(255,255,255,0.1)' }}
                  />
                  <Tooltip
                    contentStyle={{
                      backgroundColor: "rgba(15, 10, 30, 0.95)",
                      border: "1px solid rgba(255,255,255,0.1)",
                      borderRadius: 12,
                      color: "#fff",
                      fontSize: 12,
                      boxShadow: "0 8px 32px rgba(0,0,0,0.4)",
                      backdropFilter: "blur(8px)",
                    }}
                    cursor={{ stroke: 'rgba(139, 92, 246, 0.3)' }}
                  />
                  <Legend 
                    wrapperStyle={{ paddingTop: 16 }}
                    formatter={(value) => <span className="text-text-secondary text-sm">{value}</span>}
                  />
                  <Line 
                    type="monotone" 
                    dataKey="CPU" 
                    stroke="url(#cpuGradient)" 
                    dot={false} 
                    strokeWidth={2.5}
                    activeDot={{ r: 6, fill: '#8b5cf6', stroke: '#fff', strokeWidth: 2 }}
                  />
                  <Line 
                    type="monotone" 
                    dataKey="RAM" 
                    stroke="url(#ramGradient)" 
                    dot={false} 
                    strokeWidth={2.5}
                    activeDot={{ r: 6, fill: '#3b82f6', stroke: '#fff', strokeWidth: 2 }}
                  />
                  <Line 
                    type="monotone" 
                    dataKey="Disk" 
                    stroke="url(#diskGradient)" 
                    dot={false} 
                    strokeWidth={2.5}
                    activeDot={{ r: 6, fill: '#ec4899', stroke: '#fff', strokeWidth: 2 }}
                  />
                </LineChart>
              </ResponsiveContainer>
            )}
          </div>
        </div>
      )}

      {tab === "files" && (
        <div className="space-y-4">
          {/* Upload Section Card */}
          <div 
            className="glass-card rounded-2xl p-5 card-glow relative overflow-hidden"
            onMouseMove={cardGlow.onMouseMove}
    
          >
            <div className="flex items-center justify-between gap-4 flex-wrap mb-4">
              <div>
                <p className="text-sm font-medium text-text-primary flex items-center gap-2">
                  <Folder className="w-4 h-4 text-purple-400" />
                  {tx("当前目录", "Current Directory")}
                </p>
                <p className="break-all text-xs text-text-muted mt-1 font-mono">{currentPath}</p>
              </div>
              <button
                onClick={handleRefreshFiles}
                className="flex h-9 items-center gap-2 rounded-xl glass-card px-4 text-sm font-medium text-text-primary transition-all hover:border-purple-500/30"
              >
                <RefreshCw className="w-4 h-4" />
                {tx("刷新", "Refresh")}
              </button>
            </div>

            <div className="grid gap-4 sm:grid-cols-[1fr_auto] sm:items-end">
              <div className="space-y-2">
                <label className="block text-xs text-text-secondary">{tx("上传文件", "Upload File")}</label>
                <div className="flex items-center gap-3 rounded-xl border-2 border-dashed border-white/10 bg-white/5 px-4 py-3 transition-all hover:border-purple-500/30 hover:bg-white/10">
                  <input
                    ref={fileInputRef}
                    type="file"
                    className="hidden"
                    onChange={(e) => {
                      const file = e.target.files?.[0] || null;
                      setSelectedFile(file);
                      setUploadError("");
                    }}
                  />
                  <button
                    onClick={() => fileInputRef.current?.click()}
                    className="flex items-center gap-2 rounded-xl bg-gradient-to-r from-purple-500/20 to-blue-500/20 border border-purple-500/30 px-4 py-2 text-sm text-text-primary transition-all hover:from-purple-500/30 hover:to-blue-500/30"
                  >
                    <Upload className="w-4 h-4 text-purple-400" />
                    {tx("选择文件", "Choose File")}
                  </button>
                  <div className="min-w-0 flex-1">
                    <p className="truncate text-sm text-text-primary">
                      {selectedFile ? selectedFile.name : tx("未选择文件", "No file selected")}
                    </p>
                    {selectedFile && (
                      <p className="text-xs text-text-muted font-mono">{formatBytes(selectedFile.size)}</p>
                    )}
                  </div>
                </div>
              </div>

              <button
                onClick={handleUpload}
                disabled={uploading || !selectedFile}
                className="h-11 rounded-xl gradient-button px-6 text-sm font-medium text-white disabled:opacity-50"
              >
                {uploading ? tx("上传中...", "Uploading...") : tx("上传到当前目录", "Upload Here")}
              </button>
            </div>

            {uploadError && (
              <div className="mt-4 rounded-xl border border-red-500/30 bg-red-500/10 px-4 py-3 text-sm text-red-300 flex items-center gap-2">
                <span className="w-2 h-2 rounded-full bg-red-400" />
                {uploadError}
              </div>
            )}

            {uploading && (
              <div className="mt-4 space-y-2">
                <div className="flex items-center justify-between text-xs">
                  <span className="text-text-secondary">{tx("上传进度", "Upload Progress")}</span>
                  <span className="text-purple-400 font-mono">{uploadProgress}%</span>
                </div>
                <div className="h-2 overflow-hidden rounded-full bg-white/10">
                  <div
                    className="h-full rounded-full bg-gradient-to-r from-purple-500 via-blue-500 to-pink-500 transition-all duration-200 relative"
                    style={{ width: `${uploadProgress}%` }}
                  >
                    <div className="absolute inset-0 bg-gradient-to-r from-transparent via-white/30 to-transparent animate-shimmer" />
                  </div>
                </div>
              </div>
            )}
          </div>

          {/* File Browser Card */}
          <div className="overflow-hidden rounded-2xl glass-card">
            {/* Breadcrumbs */}
            <div className="flex items-center gap-1 overflow-x-auto border-b border-white/10 px-5 py-3 text-sm bg-white/5">
              {breadcrumbs.map((crumb, index) => (
                <span key={crumb.path} className="flex items-center gap-1 shrink-0">
                  {index > 0 && <ChevronRight className="h-3 w-3 text-text-muted" />}
                  <button
                    onClick={() => setCurrentPath(crumb.path)}
                    className={`transition-colors hover:text-purple-400 ${
                      index === breadcrumbs.length - 1 ? "text-purple-400 font-medium" : "text-text-muted"
                    }`}
                  >
                    {crumb.label}
                  </button>
                </span>
              ))}
            </div>

            {filesLoading ? (
              <div className="flex items-center justify-center h-48">
                <Loader2 className="h-6 w-6 animate-spin text-purple-400" />
              </div>
            ) : files.length === 0 ? (
              <div className="flex h-48 items-center justify-center">
                <div className="text-center">
                  <Folder className="w-12 h-12 mx-auto mb-3 text-text-muted/30" />
                  <p className="text-sm text-text-muted">{tx("目录为空", "Empty directory")}</p>
                </div>
              </div>
            ) : (
              <div className="divide-y divide-white/5">
                {currentPath !== "/" && (
                  <button
                    onClick={() => setCurrentPath(parentPath(currentPath))}
                    className="flex w-full items-center gap-3 px-5 py-3 text-left transition-all hover:bg-white/5"
                  >
                    <ArrowLeft className="h-4 w-4 text-text-muted" />
                    <span className="text-sm text-text-secondary">..</span>
                  </button>
                )}
                {[...files]
                  .sort((a, b) => (a.is_dir === b.is_dir ? a.name.localeCompare(b.name) : a.is_dir ? -1 : 1))
                  .map((entry) => (
                    <div
                      key={entry.path}
                      className="group flex w-full items-center gap-3 px-5 py-3 transition-all hover:bg-gradient-to-r hover:from-purple-500/5 hover:to-blue-500/5"
                    >
                      <button
                        onClick={() => handleFileClick(entry)}
                        className="flex items-center gap-3 flex-1 min-w-0 text-left"
                      >
                        {entry.is_dir ? (
                          <div className="p-1.5 rounded-lg bg-purple-500/20">
                            <Folder className="h-4 w-4 shrink-0 text-purple-400" />
                          </div>
                        ) : (
                          <div className="p-1.5 rounded-lg bg-white/10">
                            <FileText className="h-4 w-4 shrink-0 text-text-muted" />
                          </div>
                        )}
                        <span className="truncate text-sm text-text-primary group-hover:text-purple-300 transition-colors">
                          {entry.name}
                        </span>
                        {!entry.is_dir && (
                          <span className="ml-auto shrink-0 text-xs text-text-muted font-mono">
                            {formatBytes(entry.size)}
                          </span>
                        )}
                      </button>
                      <div className="flex items-center gap-1 shrink-0">
                        <button
                          onClick={() => {
                            void handleDownload(entry.path);
                          }}
                          className="rounded-lg p-2 text-text-muted opacity-0 transition-all hover:text-purple-400 hover:bg-purple-500/10 group-hover:opacity-100"
                          title={entry.is_dir ? tx("下载文件夹", "Download folder") : tx("下载文件", "Download file")}
                        >
                          <Download className="w-4 h-4" />
                        </button>
                      </div>
                    </div>
                  ))}
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );
}

