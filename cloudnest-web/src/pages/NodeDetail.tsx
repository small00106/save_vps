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

  return (
    <div className="space-y-6 animate-[fadeIn_0.3s_ease-out]">
      <div className="rounded-xl border border-border bg-card p-5">
        <div className="flex flex-wrap items-center gap-3 mb-2">
          <h1 className="text-xl font-bold text-text-primary">{node.hostname}</h1>
          <span
            className={`px-2 py-0.5 rounded-full text-xs font-medium ${
              isOnline ? "bg-online/10 text-online" : "bg-border text-text-muted"
            }`}
          >
            {node.status}
          </span>
        </div>
        <div className="flex flex-wrap gap-x-6 gap-y-1 text-sm text-text-secondary">
          <span>{node.ip}:{node.port}</span>
          <span>{node.os} / {node.arch}</span>
          <span>{node.cpu_model} ({node.cpu_cores} {tx("核", "cores")})</span>
          <span>RAM {formatBytes(node.ram_total)}</span>
          <span>{tx("磁盘", "Disk")} {formatBytes(node.disk_total)}</span>
        </div>
      </div>

      <div className="grid grid-cols-1 xl:grid-cols-3 gap-4">
        <div className="space-y-3 rounded-xl border border-border bg-card p-4">
          <p className="text-xs text-text-secondary">
            {tx("节点标签（逗号分隔）", "Node Tags (comma separated)")}
          </p>
          <input
            value={tagsInput}
            onChange={(e) => setTagsInput(e.target.value)}
            className="h-9 w-full rounded-lg border border-border bg-bg px-3 text-sm text-text-primary transition-colors focus:border-accent focus:outline-none"
            placeholder="prod,beijing,gpu"
          />
          <div className="flex items-center gap-2">
            <button
              onClick={handleSaveTags}
              disabled={savingTags}
              className="h-8 rounded-lg bg-accent px-3 text-xs font-medium text-white transition-colors hover:bg-accent-hover disabled:opacity-50"
            >
              {savingTags ? tx("保存中...", "Saving...") : tx("保存标签", "Save Tags")}
            </button>
            <button
              onClick={() => navigate(`/terminal/${uuid}`)}
              className="h-8 rounded-lg border border-border bg-card px-3 text-xs font-medium text-text-primary transition-colors hover:bg-border/50"
            >
              {tx("打开终端", "Open Terminal")}
            </button>
          </div>
        </div>

        <div className="rounded-xl border border-border bg-card p-4">
          <p className="mb-3 text-xs text-text-secondary">{tx("流量", "Traffic")}</p>
          <div className="space-y-2 text-sm">
            <div className="flex items-center justify-between">
              <span className="text-text-muted">{tx("入站速率", "Inbound Speed")}</span>
              <span className="text-text-primary">{formatBytes(traffic?.net_in_speed || 0)}/s</span>
            </div>
            <div className="flex items-center justify-between">
              <span className="text-text-muted">{tx("出站速率", "Outbound Speed")}</span>
              <span className="text-text-primary">{formatBytes(traffic?.net_out_speed || 0)}/s</span>
            </div>
            <div className="flex items-center justify-between">
              <span className="text-text-muted">{tx("入站总量", "Inbound Total")}</span>
              <span className="text-text-primary">{formatBytes(traffic?.net_in_total || 0)}</span>
            </div>
            <div className="flex items-center justify-between">
              <span className="text-text-muted">{tx("出站总量", "Outbound Total")}</span>
              <span className="text-text-primary">{formatBytes(traffic?.net_out_total || 0)}</span>
            </div>
          </div>
        </div>

        <div className="space-y-3 rounded-xl border border-border bg-card p-4">
          <p className="text-xs text-text-secondary">{tx("快速命令", "Quick Command")}</p>
          <input
            value={command}
            onChange={(e) => setCommand(e.target.value)}
            className="h-9 w-full rounded-lg border border-border bg-bg px-3 text-sm text-text-primary transition-colors focus:border-accent focus:outline-none"
            placeholder="uptime"
          />
          <button
            onClick={handleExecCommand}
            disabled={runningCommand || !command.trim()}
            className="h-8 rounded-lg bg-accent px-3 text-xs font-medium text-white transition-colors hover:bg-accent-hover disabled:opacity-50"
          >
            {runningCommand ? tx("执行中...", "Running...") : tx("执行", "Run")}
          </button>
          {commandTask && (
            <div className="rounded-lg border border-border bg-bg p-2">
              <p className="mb-1 text-[11px] text-text-secondary">
                {tx("状态", "Status")}: {commandTask.status}
              </p>
              <pre className="max-h-24 overflow-y-auto whitespace-pre-wrap break-all text-[11px] text-text-primary">
                {commandTask.output || tx("(无输出)", "(no output)")}
              </pre>
            </div>
          )}
        </div>
      </div>

      <div className="flex w-fit gap-1 rounded-lg border border-border bg-card p-1">
        {(["metrics", "files"] as const).map((value) => (
          <button
            key={value}
            onClick={() => setTab(value)}
            className={`px-4 py-1.5 rounded-md text-sm font-medium transition-colors ${
              tab === value ? "bg-border text-text-primary" : "text-text-muted hover:text-text-secondary"
            }`}
          >
            {value === "metrics" ? tx("指标", "Metrics") : tx("文件", "Files")}
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
                    ? "bg-accent text-white"
                    : "border border-border bg-card text-text-secondary hover:text-text-primary"
                }`}
              >
                {r}
              </button>
            ))}
          </div>

          <div className="rounded-xl border border-border bg-card p-5">
            {metricsLoading ? (
              <div className="flex items-center justify-center h-64">
                <Loader2 className="h-5 w-5 animate-spin text-accent" />
              </div>
            ) : chartData.length === 0 ? (
              <div className="flex h-64 items-center justify-center text-sm text-text-muted">
                {tx("暂无指标数据", "No metrics data available")}
              </div>
            ) : (
              <ResponsiveContainer width="100%" height={320}>
                <LineChart data={chartData}>
                  <CartesianGrid strokeDasharray="3 3" stroke="var(--color-border)" />
                  <XAxis dataKey="time" stroke="var(--color-text-muted)" fontSize={11} tickLine={false} />
                  <YAxis stroke="var(--color-text-muted)" fontSize={11} tickLine={false} domain={[0, 100]} unit="%" />
                  <Tooltip
                    contentStyle={{
                      backgroundColor: "var(--color-card)",
                      border: "1px solid var(--color-border)",
                      borderRadius: 8,
                      color: "var(--color-text-primary)",
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
        <div className="space-y-4">
          <div className="space-y-4 rounded-xl border border-border bg-card p-4">
            <div className="flex items-center justify-between gap-3 flex-wrap">
              <div>
                <p className="text-sm font-medium text-text-primary">{tx("当前目录", "Current Directory")}</p>
                <p className="break-all text-xs text-text-muted">{currentPath}</p>
              </div>
              <button
                onClick={handleRefreshFiles}
                className="flex h-8 items-center gap-2 rounded-lg border border-border bg-card px-3 text-xs font-medium text-text-primary transition-colors hover:bg-border/50"
              >
                <RefreshCw className="w-4 h-4" />
                {tx("刷新", "Refresh")}
              </button>
            </div>

            <div className="grid gap-3 sm:grid-cols-[1fr_auto] sm:items-end">
              <div className="space-y-2">
                <label className="block text-xs text-text-secondary">{tx("上传文件", "Upload File")}</label>
                <div className="flex items-center gap-3 rounded-lg border border-dashed border-border bg-bg px-3 py-2">
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
                    className="flex items-center gap-2 rounded-lg border border-border bg-card px-3 py-1.5 text-sm text-text-primary transition-colors hover:bg-border/50"
                  >
                    <Upload className="w-4 h-4" />
                    {tx("选择文件", "Choose File")}
                  </button>
                  <div className="min-w-0 flex-1">
                    <p className="truncate text-sm text-text-primary">
                      {selectedFile ? selectedFile.name : tx("未选择文件", "No file selected")}
                    </p>
                    {selectedFile && (
                      <p className="text-xs text-text-muted">{formatBytes(selectedFile.size)}</p>
                    )}
                  </div>
                </div>
              </div>

              <button
                onClick={handleUpload}
                disabled={uploading || !selectedFile}
                className="h-10 rounded-lg bg-accent px-4 text-sm font-medium text-white transition-colors hover:bg-accent-hover disabled:opacity-50"
              >
                {uploading ? tx("上传中...", "Uploading...") : tx("上传到当前目录", "Upload Here")}
              </button>
            </div>

            {uploadError && (
              <div className="rounded-lg border border-red-500/30 bg-red-500/10 px-3 py-2 text-sm text-red-300">
                {uploadError}
              </div>
            )}

            {uploading && (
              <div className="space-y-2">
                <div className="flex items-center justify-between text-xs text-text-secondary">
                  <span>{tx("上传进度", "Upload Progress")}</span>
                  <span>{uploadProgress}%</span>
                </div>
                <div className="h-2 overflow-hidden rounded-full bg-border">
                  <div
                    className="h-full rounded-full bg-accent transition-all duration-200"
                    style={{ width: `${uploadProgress}%` }}
                  />
                </div>
              </div>
            )}
          </div>

          <div className="overflow-hidden rounded-xl border border-border bg-card">
            <div className="flex items-center gap-1 overflow-x-auto border-b border-border px-4 py-3 text-sm">
              {breadcrumbs.map((crumb, index) => (
                <span key={crumb.path} className="flex items-center gap-1 shrink-0">
                  {index > 0 && <ChevronRight className="h-3 w-3 text-text-muted" />}
                  <button
                    onClick={() => setCurrentPath(crumb.path)}
                    className={`transition-colors hover:text-accent ${
                      index === breadcrumbs.length - 1 ? "text-text-primary" : "text-text-muted"
                    }`}
                  >
                    {crumb.label}
                  </button>
                </span>
              ))}
            </div>

            {filesLoading ? (
              <div className="flex items-center justify-center h-48">
                <Loader2 className="h-5 w-5 animate-spin text-accent" />
              </div>
            ) : files.length === 0 ? (
              <div className="flex h-48 items-center justify-center text-sm text-text-muted">
                {tx("目录为空", "Empty directory")}
              </div>
            ) : (
              <div className="divide-y divide-border">
                {currentPath !== "/" && (
                  <button
                    onClick={() => setCurrentPath(parentPath(currentPath))}
                    className="flex w-full items-center gap-3 px-4 py-2.5 text-left transition-colors hover:bg-border/50"
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
                      className="group flex w-full items-center gap-3 px-4 py-2.5 transition-colors hover:bg-border/50"
                    >
                      <button
                        onClick={() => handleFileClick(entry)}
                        className="flex items-center gap-3 flex-1 min-w-0 text-left"
                      >
                        {entry.is_dir ? (
                          <Folder className="h-4 w-4 shrink-0 text-accent" />
                        ) : (
                          <FileText className="h-4 w-4 shrink-0 text-text-muted" />
                        )}
                        <span className="truncate text-sm text-text-primary">{entry.name}</span>
                        {!entry.is_dir && (
                          <span className="ml-auto shrink-0 text-xs text-text-muted">
                            {formatBytes(entry.size)}
                          </span>
                        )}
                      </button>
                      <div className="flex items-center gap-1 shrink-0">
                        {entry.is_dir && (
                          <button
                            onClick={() => {
                              void handleDownload(entry.path);
                            }}
                            className="rounded p-1 text-text-muted opacity-0 transition-all hover:text-accent group-hover:opacity-100"
                            title={tx("下载文件夹", "Download folder")}
                          >
                            <Download className="w-3.5 h-3.5" />
                          </button>
                        )}
                        {!entry.is_dir && (
                          <button
                            onClick={() => {
                              void handleDownload(entry.path);
                            }}
                            className="rounded p-1 text-text-muted opacity-0 transition-all hover:text-accent group-hover:opacity-100"
                            title={tx("下载文件", "Download file")}
                          >
                            <Download className="w-3.5 h-3.5" />
                          </button>
                        )}
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
