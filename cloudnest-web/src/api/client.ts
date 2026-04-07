const BASE = "/api";
const AUTH_EXPIRED_EVENT = "cloudnest:auth-expired";

type RequestOptions = {
  handleUnauthorized?: boolean;
};

let authExpiredDispatched = false;

function dispatchAuthExpired() {
  if (authExpiredDispatched || typeof window === "undefined") return;
  authExpiredDispatched = true;
  window.dispatchEvent(new Event(AUTH_EXPIRED_EVENT));
}

export function onAuthExpired(handler: () => void) {
  if (typeof window === "undefined") return () => {};

  const listener = () => handler();
  window.addEventListener(AUTH_EXPIRED_EVENT, listener);
  return () => window.removeEventListener(AUTH_EXPIRED_EVENT, listener);
}

export function resetAuthExpiredState() {
  authExpiredDispatched = false;
}

async function request<T>(
  method: string,
  path: string,
  body?: unknown,
  options?: RequestOptions,
): Promise<T> {
  const opts: RequestInit = {
    method,
    credentials: "include",
    headers: { "Content-Type": "application/json" },
  };
  if (body !== undefined) {
    opts.body = JSON.stringify(body);
  }
  const res = await fetch(`${BASE}${path}`, opts);
  if (!res.ok) {
    const text = await res.text().catch(() => "");
    if (res.status === 401 && options?.handleUnauthorized !== false) {
      dispatchAuthExpired();
    }
    throw new ApiError(res.status, text || res.statusText);
  }
  if (res.status === 204) return undefined as T;
  return res.json() as Promise<T>;
}

export class ApiError extends Error {
  status: number;
  constructor(status: number, message: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
  }
}

export const api = {
  get: <T>(path: string, options?: RequestOptions) => request<T>("GET", path, undefined, options),
  post: <T>(path: string, body?: unknown, options?: RequestOptions) => request<T>("POST", path, body, options),
  put: <T>(path: string, body?: unknown, options?: RequestOptions) => request<T>("PUT", path, body, options),
  delete: <T>(path: string, options?: RequestOptions) => request<T>("DELETE", path, undefined, options),
};

// ========================
// Types — match Go backend JSON tags exactly
// ========================

export interface User {
  username: string;
  default_password_notice_required: boolean;
}

export interface Node {
  uuid: string;
  hostname: string;
  ip: string;
  port: number;
  region: string;
  tags: string; // JSON array string e.g. '["日本","备份"]'
  os: string;
  arch: string;
  cpu_model: string;
  cpu_cores: number;
  disk_total: number;
  disk_used: number;
  ram_total: number;
  status: string; // "online" | "offline" | "draining"
  version: string;
  rate_limit: number;
  last_seen: string;
  created_at: string;
  updated_at: string;
  latest_metric?: NodeMetric;
}

export interface NodeMetric {
  id: number;
  node_uuid: string;
  cpu_percent: number;
  mem_percent: number;
  swap_used: number;
  swap_total: number;
  disk_percent: number;
  load1: number;
  load5: number;
  load15: number;
  net_in_speed: number;
  net_out_speed: number;
  net_in_total: number;
  net_out_total: number;
  tcp_conns: number;
  udp_conns: number;
  process_count: number;
  uptime: number;
  timestamp: string;
}

export interface NodeMetricCompact {
  id: number;
  node_uuid: string;
  cpu_percent: number;
  mem_percent: number;
  disk_percent: number;
  net_in_speed: number;
  net_out_speed: number;
  bucket_time: string;
}

export interface FileEntry {
  path: string;
  name: string;
  size: number;
  is_dir: boolean;
  mod_time: string;
}

export interface StoredFile {
  file_id: string;
  name: string;
  path: string;
  size: number;
  mime_type: string;
  checksum: string;
  is_dir: boolean;
  status: string;
  created_at: string;
}

export interface DownloadResponse {
  url: string;
  filename: string;
  size?: number;
}

export interface NodeUploadTarget {
  node_uuid: string;
  url: string;
}

export interface NodeUploadResponse {
  file_id: string;
  url?: string;
  target?: NodeUploadTarget;
  targets?: NodeUploadTarget[];
}

export interface NodeUploadRequest {
  name: string;
  size: number;
  path: string;
  node_uuid: string;
  overwrite?: boolean;
}

export interface CommandTask {
  id: number;
  node_uuid: string;
  command: string;
  output: string;
  exit_code: number;
  status: string;
  created_at: string;
}

export interface PingTask {
  id: number;
  name: string;
  type: string; // "icmp" | "tcp" | "http"
  target: string;
  interval: number;
  enabled: boolean;
}

export interface PingResult {
  id: number;
  task_id: number;
  node_uuid: string;
  latency: number; // ms
  success: boolean;
  timestamp: string;
}

export interface AlertRule {
  id: number;
  name: string;
  node_uuid: string;
  metric: string; // "cpu" | "mem" | "disk" | "offline"
  operator: string; // "gt" | "lt"
  threshold: number;
  duration: number;
  channel_id: number;
  enabled: boolean;
  last_fired_at: string;
  created_at: string;
}

export interface AlertChannel {
  id: number;
  name: string;
  type: string; // "telegram" | "webhook" | "email" | "bark" | "serverchan"
  config: string; // JSON string
}

export interface AuditLog {
  id: number;
  action: string;
  detail: string;
  ip: string;
  created_at: string;
}

export interface Settings {
  node_count: number;
  online_count: number;
  file_count: number;
}

// ========================
// Auth
// ========================

export function login(username: string, password: string) {
  return api.post<{ token: string; username: string; default_password_notice_required: boolean }>(
    "/auth/login",
    { username, password },
    { handleUnauthorized: false },
  );
}

export function logout() {
  return api.post<void>("/auth/logout", undefined, { handleUnauthorized: false });
}

export function getMe() {
  return api.get<User>("/auth/me", { handleUnauthorized: false });
}

export function changePassword(currentPassword: string, newPassword: string) {
  return api.post<{ message: string }>("/auth/change-password", {
    current_password: currentPassword,
    new_password: newPassword,
  });
}

export function acknowledgeDefaultPasswordNotice() {
  return api.post<{ message: string }>("/auth/default-password-notice/ack");
}

// ========================
// Nodes
// ========================

export function getNodes() {
  return api.get<Node[]>("/nodes");
}

export function getNode(uuid: string) {
  return api.get<{ node: Node; latest_metric: NodeMetric | null }>(`/nodes/${uuid}`);
}

export function getNodeMetrics(uuid: string, timeRange = "1h") {
  return api.get<(NodeMetric | NodeMetricCompact)[]>(`/nodes/${uuid}/metrics?range=${timeRange}`);
}

export function getNodeTraffic(uuid: string) {
  return api.get<{ net_in_total: number; net_out_total: number; net_in_speed: number; net_out_speed: number }>(`/nodes/${uuid}/traffic`);
}

export function updateNodeTags(uuid: string, tags: string) {
  return api.put<{ message: string }>(`/nodes/${uuid}/tags`, { tags });
}

// ========================
// Node Files (browsing agent's real directory)
// ========================

export function getNodeFiles(uuid: string, path = "/") {
  return api.get<FileEntry[]>(`/nodes/${uuid}/files?path=${encodeURIComponent(path)}`);
}

export function getNodeDownloadURL(uuid: string, path: string) {
  return api.get<DownloadResponse>(`/nodes/${uuid}/download?path=${encodeURIComponent(path)}`);
}

// ========================
// Files (virtual managed storage)
// ========================

export function initUpload(data: NodeUploadRequest) {
  return api.post<NodeUploadResponse>("/files/upload", data);
}

export function getDownloadURL(id: string) {
  return api.get<DownloadResponse>(`/files/download/${id}`);
}

export function listFiles(path = "/") {
  return api.get<StoredFile[]>(`/files?path=${encodeURIComponent(path)}`);
}

export function searchFiles(q: string) {
  return api.get<StoredFile[]>(`/files/search?q=${encodeURIComponent(q)}`);
}

export function createDir(path: string, name: string) {
  return api.post<StoredFile>("/files/mkdir", { path, name });
}

export function deleteFile(id: string) {
  return api.delete<{ message: string }>(`/files/${id}`);
}

export function moveFile(
  id: string,
  data: { new_path?: string; new_name?: string },
) {
  return api.put<{ message: string }>(`/files/${id}/move`, data);
}

// ========================
// Remote Operations
// ========================

export function execCommand(uuid: string, command: string) {
  return api.post<{ task_id: number; status: string }>(`/nodes/${uuid}/exec`, { command });
}

export function getCommandTask(id: number) {
  return api.get<CommandTask>(`/commands/${id}`);
}

// ========================
// Ping
// ========================

export function getPingTasks() {
  return api.get<PingTask[]>("/ping/tasks");
}

export function createPingTask(data: Partial<PingTask>) {
  return api.post<PingTask>("/ping/tasks", data);
}

export function getPingResults(taskId: number) {
  return api.get<PingResult[]>(`/ping/tasks/${taskId}/results`);
}

export function deletePingTask(taskId: number) {
  return api.delete<{ message: string }>(`/ping/tasks/${taskId}`);
}

// ========================
// Alerts
// ========================

export function getAlertRules() {
  return api.get<AlertRule[]>("/alerts/rules");
}

export function createAlertRule(data: Partial<AlertRule>) {
  return api.post<AlertRule>("/alerts/rules", data);
}

export function updateAlertRule(id: number, data: Partial<AlertRule>) {
  return api.put<AlertRule>(`/alerts/rules/${id}`, data);
}

export function deleteAlertRule(id: number) {
  return api.delete<void>(`/alerts/rules/${id}`);
}

export function getAlertChannels() {
  return api.get<AlertChannel[]>("/alerts/channels");
}

export function createAlertChannel(data: Partial<AlertChannel>) {
  return api.post<AlertChannel>("/alerts/channels", data);
}

export function updateAlertChannel(id: number, data: Partial<AlertChannel>) {
  return api.put<AlertChannel>(`/alerts/channels/${id}`, data);
}

// ========================
// Admin
// ========================

export function getAuditLogs() {
  return api.get<AuditLog[]>("/admin/audit");
}

export function getSettings() {
  return api.get<Settings>("/admin/settings");
}
