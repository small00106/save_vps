import { useEffect, useMemo, useRef, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import {
  Activity,
  ArrowLeft,
  ChevronRight,
  Download,
  FileText,
  Folder,
  HardDrive,
  Loader2,
  Network,
  RefreshCw,
  ServerOff,
  Tag,
  Terminal,
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
  CartesianGrid,
  Legend,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import { useI18n } from "../i18n/useI18n";
import { getNodeDisplayName, parseNodeTags } from "../utils/nodeDisplayName";
import { Banner, EmptyState, MetricCard, PageHeader, SectionCard, StatusBadge, SurfaceBox } from "../components/ui";

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

function uploadFileToUrl(url: string, file: File, onProgress: (pct: number) => void): Promise<void> {
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
      .catch(() => setNode(null))
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
      .then((data) => setMetrics((Array.isArray(data) ? data : []) as MetricPoint[]))
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
      metrics.map((metric) => ({
        time: new Date(metric.timestamp || metric.bucket_time || "").toLocaleTimeString(),
        CPU: metric.cpu_percent,
        RAM: metric.mem_percent,
        Disk: metric.disk_percent,
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
      .map((tag) => tag.trim())
      .filter(Boolean);
    const raw = JSON.stringify(nextTags);
    try {
      await updateNodeTags(uuid, raw);
      setNode({ ...node, tags: raw });
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
          tx("当前目录已存在同名文件，是否覆盖？", "A file with the same name already exists in this directory. Overwrite it?"),
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
    } finally {
      setRunningCommand(false);
    }
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center py-24">
        <Loader2 className="h-8 w-8 animate-spin text-accent" />
      </div>
    );
  }

  if (!node) {
    return (
      <EmptyState
        icon={ServerOff}
        title={tx("节点不存在", "Node not found")}
        description={tx("无法加载该节点详情，请确认节点 UUID 是否正确。", "Unable to load node details. Verify that the node UUID is correct.")}
      />
    );
  }

  const isOnline = node.status === "online";
  const displayName = getNodeDisplayName(node.hostname, node.tags);
  const showHostname = displayName !== node.hostname;

  return (
    <div className="space-y-6">
      <PageHeader
        eyebrow={tx("节点详情", "Node Detail")}
        title={displayName}
        description={showHostname ? `${tx("主机名", "Hostname")}: ${node.hostname}` : `${node.ip || node.region || "-"}`}
        actions={
          <>
            <StatusBadge tone={isOnline ? "success" : "danger"} label={isOnline ? tx("在线", "Online") : tx("离线", "Offline")} />
            <button type="button" onClick={() => navigate(-1)} className="inline-flex items-center gap-2 rounded-2xl border border-border bg-surface px-4 py-2.5 text-sm font-medium text-text-primary transition-colors hover:border-border-hover hover:bg-card">
              <ArrowLeft className="h-4 w-4" />
              {tx("返回", "Back")}
            </button>
          </>
        }
      />

      <div className="grid grid-cols-1 gap-4 xl:grid-cols-3">
        <MetricCard label={tx("系统与硬件", "System and Hardware")} value={`${node.os}/${node.arch}`} meta={`${node.cpu_model} · ${node.cpu_cores} ${tx("核", "cores")}`} icon={HardDrive} tone="primary" />
        <MetricCard label={tx("网络位置", "Network Address")} value={`${node.ip || node.region || "-"}:${node.port}`} meta={node.region || tx("未配置地区", "Region not set")} icon={Network} tone="neutral" />
        <MetricCard label={tx("存储上限", "Disk Capacity")} value={formatBytes(node.disk_total)} meta={`${tx("内存", "Memory")} ${formatBytes(node.ram_total)}`} icon={Activity} tone="warning" />
      </div>

      <div className="grid grid-cols-1 gap-4 xl:grid-cols-[1.2fr_0.8fr]">
        <SectionCard title={tx("节点标签", "Node Tags")} description={tx("第一个标签会作为节点显示名。", "The first tag is used as the node display name.")}> 
          <div className="flex flex-col gap-4 md:flex-row md:items-end">
            <label className="min-w-0 flex-1 space-y-2">
              <span className="flex items-center gap-2 text-sm font-medium text-text-secondary"><Tag className="h-4 w-4" />{tx("标签", "Tags")}</span>
              <input value={tagsInput} onChange={(e) => setTagsInput(e.target.value)} className="w-full rounded-2xl border border-border bg-surface px-4 py-3 text-sm text-text-primary outline-none" placeholder="prod, beijing, gpu" />
            </label>
            <div className="flex gap-2">
              <button type="button" onClick={handleSaveTags} disabled={savingTags} className="gradient-button rounded-2xl px-4 py-3 text-sm font-medium text-white disabled:opacity-50">
                {savingTags ? tx("保存中...", "Saving...") : tx("保存标签", "Save Tags")}
              </button>
              <button type="button" onClick={() => navigate(`/terminal/${uuid}`)} className="rounded-2xl border border-border bg-surface px-4 py-3 text-sm font-medium text-text-primary transition-colors hover:border-border-hover hover:bg-card">
                <span className="inline-flex items-center gap-2"><Terminal className="h-4 w-4" />{tx("终端", "Terminal")}</span>
              </button>
            </div>
          </div>
        </SectionCard>

        <SectionCard title={tx("流量概览", "Traffic Summary")} description={tx("显示当前瞬时速率和累计流量。", "Show current rates and cumulative traffic.")}> 
          <div className="grid gap-3 sm:grid-cols-2">
            <SurfaceBox>
              <p className="text-xs font-medium uppercase tracking-[0.2em] text-text-muted">{tx("入站速率", "Inbound")}</p>
              <p className="mt-2 text-xl font-semibold text-text-primary">{formatBytes(traffic?.net_in_speed || 0)}/s</p>
              <p className="mt-1 text-xs text-text-muted">{tx("总量", "Total")} {formatBytes(traffic?.net_in_total || 0)}</p>
            </SurfaceBox>
            <SurfaceBox>
              <p className="text-xs font-medium uppercase tracking-[0.2em] text-text-muted">{tx("出站速率", "Outbound")}</p>
              <p className="mt-2 text-xl font-semibold text-text-primary">{formatBytes(traffic?.net_out_speed || 0)}/s</p>
              <p className="mt-1 text-xs text-text-muted">{tx("总量", "Total")} {formatBytes(traffic?.net_out_total || 0)}</p>
            </SurfaceBox>
          </div>
        </SectionCard>
      </div>

      <SectionCard title={tx("快速命令", "Quick Command")} description={tx("适合执行单条命令并查看输出。", "Useful for running a single command and reviewing the output.")}> 
        <div className="flex flex-col gap-3 md:flex-row">
          <input
            value={command}
            onChange={(e) => setCommand(e.target.value)}
            className="min-w-0 flex-1 rounded-2xl border border-border bg-surface px-4 py-3 font-mono text-sm text-text-primary outline-none"
            placeholder="uptime"
            onKeyDown={(e) => {
              if (e.key === "Enter") {
                void handleExecCommand();
              }
            }}
          />
          <button type="button" onClick={() => void handleExecCommand()} disabled={runningCommand || !command.trim()} className="gradient-button rounded-2xl px-4 py-3 text-sm font-medium text-white disabled:opacity-50">
            {runningCommand ? tx("执行中...", "Running...") : tx("执行", "Run")}
          </button>
        </div>

        {commandTask ? (
          <div className="mt-4 rounded-3xl border border-border bg-[#0b1220] p-4 text-slate-200">
            <div className="mb-3 flex items-center gap-3 text-xs">
              <StatusBadge
                tone={commandTask.status === "success" ? "success" : commandTask.status === "failed" ? "danger" : "warning"}
                label={commandTask.status}
              />
              <span className="font-mono text-slate-400">exit_code: {commandTask.exit_code}</span>
            </div>
            <pre className="max-h-56 overflow-y-auto whitespace-pre-wrap break-all font-mono text-xs leading-6 text-slate-200">
              {commandTask.output || tx("(无输出)", "(no output)")}
            </pre>
          </div>
        ) : null}
      </SectionCard>

      <div className="flex flex-wrap gap-2">
        {([
          ["metrics", tx("指标", "Metrics")],
          ["files", tx("文件", "Files")],
        ] as const).map(([value, label]) => (
          <button
            key={value}
            type="button"
            onClick={() => setTab(value)}
            className={[
              "rounded-full px-4 py-2 text-sm font-medium transition-colors",
              tab === value ? "bg-accent text-white" : "border border-border bg-surface text-text-secondary hover:border-border-hover hover:bg-card hover:text-text-primary",
            ].join(" ")}
          >
            {label}
          </button>
        ))}
      </div>

      {tab === "metrics" ? (
        <SectionCard
          title={tx("指标趋势", "Metric Trends")}
          description={tx("CPU、内存与磁盘占用趋势按时间范围切换。", "CPU, memory, and disk usage trends are switchable by time range.")}
          actions={
            <div className="flex flex-wrap gap-2">
              {RANGES.map((item) => (
                <button
                  key={item}
                  type="button"
                  onClick={() => setRange(item)}
                  className={[
                    "rounded-full px-3 py-1.5 text-xs font-medium transition-colors",
                    range === item ? "bg-accent text-white" : "border border-border bg-surface text-text-secondary hover:border-border-hover hover:bg-card",
                  ].join(" ")}
                >
                  {item}
                </button>
              ))}
            </div>
          }
        >
          {metricsLoading ? (
            <div className="flex items-center justify-center py-20"><Loader2 className="h-6 w-6 animate-spin text-accent" /></div>
          ) : chartData.length === 0 ? (
            <EmptyState icon={Activity} title={tx("暂无指标数据", "No metrics data available")} description={tx("当前时间范围没有可用的聚合数据。", "No aggregated metrics are available for the selected range.")} />
          ) : (
            <div className="h-[340px]">
              <ResponsiveContainer width="100%" height="100%">
                <LineChart data={chartData}>
                  <CartesianGrid strokeDasharray="3 3" stroke="rgba(148, 163, 184, 0.28)" />
                  <XAxis dataKey="time" stroke="rgba(100,116,139,0.9)" fontSize={11} tickLine={false} axisLine={{ stroke: "rgba(148,163,184,0.35)" }} />
                  <YAxis stroke="rgba(100,116,139,0.9)" fontSize={11} tickLine={false} domain={[0, 100]} unit="%" axisLine={{ stroke: "rgba(148,163,184,0.35)" }} />
                  <Tooltip
                    contentStyle={{
                      backgroundColor: "rgba(255,255,255,0.96)",
                      border: "1px solid rgba(148,163,184,0.35)",
                      borderRadius: 16,
                      color: "#0f172a",
                      fontSize: 12,
                    }}
                  />
                  <Legend wrapperStyle={{ paddingTop: 16 }} />
                  <Line type="monotone" dataKey="CPU" stroke="#0369a1" dot={false} strokeWidth={2.5} />
                  <Line type="monotone" dataKey="RAM" stroke="#0f766e" dot={false} strokeWidth={2.5} />
                  <Line type="monotone" dataKey="Disk" stroke="#b45309" dot={false} strokeWidth={2.5} />
                </LineChart>
              </ResponsiveContainer>
            </div>
          )}
        </SectionCard>
      ) : null}

      {tab === "files" ? (
        <div className="space-y-4">
          <SectionCard title={tx("上传到当前目录", "Upload to Current Directory")} description={tx("上传目标目录保持与当前浏览路径一致。", "Uploads target the same directory as the current browser path.")}> 
            <div className="grid gap-4 md:grid-cols-[1fr_auto] md:items-end">
              <div className="space-y-2">
                <label className="text-sm font-medium text-text-secondary">{tx("上传文件", "Upload File")}</label>
                <div className="flex items-center gap-3 rounded-2xl border border-dashed border-border bg-surface px-4 py-4">
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
                  <button type="button" onClick={() => fileInputRef.current?.click()} className="rounded-2xl border border-border bg-card px-4 py-2.5 text-sm font-medium text-text-primary transition-colors hover:border-border-hover hover:bg-surface-subtle">
                    <span className="inline-flex items-center gap-2"><Upload className="h-4 w-4" />{tx("选择文件", "Choose File")}</span>
                  </button>
                  <div className="min-w-0 flex-1">
                    <p className="truncate text-sm font-medium text-text-primary">{selectedFile ? selectedFile.name : tx("未选择文件", "No file selected")}</p>
                    {selectedFile ? <p className="text-xs text-text-muted">{formatBytes(selectedFile.size)}</p> : null}
                  </div>
                </div>
              </div>
              <button type="button" onClick={() => void handleUpload()} disabled={uploading || !selectedFile} className="gradient-button rounded-2xl px-4 py-3 text-sm font-medium text-white disabled:opacity-50">
                {uploading ? tx("上传中...", "Uploading...") : tx("上传到当前目录", "Upload Here")}
              </button>
            </div>

            {uploadError ? <Banner tone="danger" role="alert" className="mt-4">{uploadError}</Banner> : null}

            {uploading ? (
              <div className="mt-4 space-y-2">
                <div className="flex items-center justify-between text-xs text-text-secondary">
                  <span>{tx("上传进度", "Upload Progress")}</span>
                  <span className="font-mono text-text-primary">{uploadProgress}%</span>
                </div>
                <div className="h-2 overflow-hidden rounded-full bg-surface-subtle">
                  <div className="h-full rounded-full progress-gradient transition-all duration-200" style={{ width: `${uploadProgress}%` }} />
                </div>
              </div>
            ) : null}
          </SectionCard>

          <SectionCard
            title={tx("目录浏览", "Directory Browser")}
            description={tx("浏览 Agent 当前受管目录，并通过 Master 代理下载文件或文件夹。", "Browse the current managed directory on the agent and download files or folders through the master proxy.")}
            actions={
              <button type="button" onClick={handleRefreshFiles} className="inline-flex items-center gap-2 rounded-2xl border border-border bg-surface px-4 py-2.5 text-sm font-medium text-text-primary transition-colors hover:border-border-hover hover:bg-card">
                <RefreshCw className="h-4 w-4" />
                {tx("刷新", "Refresh")}
              </button>
            }
          >
            <div className="mb-4 flex flex-wrap items-center gap-2 text-sm">
              {breadcrumbs.map((crumb, index) => (
                <span key={crumb.path} className="inline-flex items-center gap-2">
                  {index > 0 ? <ChevronRight className="h-4 w-4 text-text-muted" /> : null}
                  <button type="button" onClick={() => setCurrentPath(crumb.path)} className={index === breadcrumbs.length - 1 ? "font-medium text-text-primary" : "text-text-secondary hover:text-text-primary"}>
                    {crumb.label}
                  </button>
                </span>
              ))}
            </div>

            {filesLoading ? (
              <div className="flex items-center justify-center py-20"><Loader2 className="h-6 w-6 animate-spin text-accent" /></div>
            ) : files.length === 0 ? (
              <EmptyState icon={Folder} title={tx("目录为空", "Empty directory")} description={tx("当前路径下暂时没有可展示的文件或文件夹。", "There are no visible files or folders in the current path yet.")} />
            ) : (
              <div className="overflow-hidden rounded-2xl border border-border">
                <div className="divide-y divide-border bg-card">
                  {currentPath !== "/" ? (
                    <button type="button" onClick={() => setCurrentPath(parentPath(currentPath))} className="flex w-full items-center gap-3 px-4 py-3 text-left text-sm text-text-secondary transition-colors hover:bg-surface">
                      <ArrowLeft className="h-4 w-4" />..
                    </button>
                  ) : null}
                  {[...files]
                    .sort((a, b) => (a.is_dir === b.is_dir ? a.name.localeCompare(b.name) : a.is_dir ? -1 : 1))
                    .map((entry) => (
                      <div key={entry.path} className="group flex items-center gap-3 px-4 py-3 transition-colors hover:bg-surface">
                        <button type="button" onClick={() => void handleFileClick(entry)} className="flex min-w-0 flex-1 items-center gap-3 text-left">
                          <span className="flex h-10 w-10 items-center justify-center rounded-2xl bg-surface-subtle text-text-secondary">
                            {entry.is_dir ? <Folder className="h-4 w-4" /> : <FileText className="h-4 w-4" />}
                          </span>
                          <div className="min-w-0 flex-1">
                            <p className="truncate text-sm font-medium text-text-primary">{entry.name}</p>
                            <p className="truncate text-xs text-text-muted">{entry.path}</p>
                          </div>
                          {!entry.is_dir ? <span className="shrink-0 text-xs text-text-muted">{formatBytes(entry.size)}</span> : null}
                        </button>
                        <button type="button" onClick={() => void handleDownload(entry.path)} className="rounded-2xl border border-transparent p-2 text-text-muted transition-colors hover:border-border hover:bg-card hover:text-accent" title={entry.is_dir ? tx("下载文件夹", "Download folder") : tx("下载文件", "Download file")}>
                          <Download className="h-4 w-4" />
                        </button>
                      </div>
                    ))}
                </div>
              </div>
            )}
          </SectionCard>
        </div>
      ) : null}
    </div>
  );
}
