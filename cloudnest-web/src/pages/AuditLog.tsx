import { useCallback, useEffect, useState } from "react";
import { Loader2, RefreshCw, ScrollText } from "lucide-react";
import { getAuditLogs, type AuditLog as AuditLogEntry } from "../api/client";
import { useI18n } from "../i18n/useI18n";

function formatDate(s: string): string {
  if (!s) return "-";
  return new Date(s).toLocaleString();
}

const ACTION_STYLES: Record<string, string> = {
  login_success: "bg-blue-500/10 text-[#3b82f6]",
  login_failed: "bg-red-500/10 text-[#ef4444]",
  logout: "bg-zinc-500/10 text-[#a1a1aa]",
  password_changed: "bg-amber-500/10 text-[#f59e0b]",
  password_change_failed: "bg-red-500/10 text-[#ef4444]",
  command_exec_requested: "bg-cyan-500/10 text-[#06b6d4]",
  command_exec_rejected: "bg-red-500/10 text-[#ef4444]",
  command_exec_completed: "bg-emerald-500/10 text-[#22c55e]",
  settings_updated: "bg-amber-500/10 text-[#f59e0b]",
  alert_fired: "bg-orange-500/10 text-[#f97316]",
};

const STATUS_STYLES: Record<string, string> = {
  success: "bg-emerald-500/10 text-[#22c55e]",
  failed: "bg-red-500/10 text-[#ef4444]",
  info: "bg-zinc-500/10 text-[#a1a1aa]",
};

const ACTION_OPTIONS = [
  "all",
  "login_success",
  "login_failed",
  "logout",
  "password_changed",
  "password_change_failed",
  "command_exec_requested",
  "command_exec_rejected",
  "command_exec_completed",
  "settings_updated",
  "alert_fired",
] as const;

const STATUS_OPTIONS = ["all", "success", "failed", "info"] as const;

function getActionStyle(action: string): string {
  return ACTION_STYLES[action] || "bg-border text-text-secondary";
}

function getStatusStyle(status: string): string {
  return STATUS_STYLES[status] || "bg-border text-text-secondary";
}

function getActionLabel(action: string, tx: (zh: string, en: string) => string): string {
  switch (action) {
    case "login_success":
      return tx("登录成功", "Login Success");
    case "login_failed":
      return tx("登录失败", "Login Failed");
    case "logout":
      return tx("退出登录", "Logout");
    case "password_changed":
      return tx("修改密码", "Password Changed");
    case "password_change_failed":
      return tx("修改密码失败", "Password Change Failed");
    case "command_exec_requested":
      return tx("发起远程命令", "Command Requested");
    case "command_exec_rejected":
      return tx("远程命令被拒绝", "Command Rejected");
    case "command_exec_completed":
      return tx("远程命令完成", "Command Completed");
    case "settings_updated":
      return tx("更新设置", "Settings Updated");
    case "alert_fired":
      return tx("告警触发", "Alert Fired");
    default:
      return action || "-";
  }
}

function getStatusLabel(status: string, tx: (zh: string, en: string) => string): string {
  switch (status) {
    case "success":
      return tx("成功", "Success");
    case "failed":
      return tx("失败", "Failed");
    case "info":
      return tx("信息", "Info");
    default:
      return status || "-";
  }
}

function formatTarget(log: AuditLogEntry): string {
  const parts = [];
  if (log.target_type) {
    parts.push(log.target_id ? `${log.target_type}:${log.target_id}` : log.target_type);
  } else if (log.target_id) {
    parts.push(log.target_id);
  }
  if (log.node_uuid) {
    parts.push(`node:${log.node_uuid}`);
  }
  return parts.length > 0 ? parts.join(" · ") : "-";
}

export default function AuditLog() {
  const { tx } = useI18n();
  const [logs, setLogs] = useState<AuditLogEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [selectedAction, setSelectedAction] = useState<(typeof ACTION_OPTIONS)[number]>("all");
  const [selectedStatus, setSelectedStatus] = useState<(typeof STATUS_OPTIONS)[number]>("all");

  const loadLogs = useCallback(async (action = selectedAction, status = selectedStatus) => {
    setLoading(true);
    try {
      const data = await getAuditLogs({
        action: action === "all" ? undefined : action,
        status: status === "all" ? undefined : status,
        limit: 200,
      });
      setLogs(Array.isArray(data) ? data : []);
    } catch {
      setLogs([]);
    } finally {
      setLoading(false);
    }
  }, [selectedAction, selectedStatus]);

  useEffect(() => {
    void loadLogs();
  }, [loadLogs]);

  const selectClass =
    "h-9 rounded-lg border border-border bg-bg px-3 text-sm text-text-primary outline-none transition-colors focus:border-accent";

  return (
    <div className="space-y-4 animate-[fadeIn_0.3s_ease-out]">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <h1 className="text-xl font-bold text-text-primary">{tx("审计日志", "Audit Log")}</h1>
        <div className="flex flex-col gap-2 sm:flex-row">
          <select
            value={selectedAction}
            onChange={(event) => setSelectedAction(event.target.value as (typeof ACTION_OPTIONS)[number])}
            className={selectClass}
          >
            {ACTION_OPTIONS.map((action) => (
              <option key={action} value={action}>
                {action === "all" ? tx("全部动作", "All Actions") : getActionLabel(action, tx)}
              </option>
            ))}
          </select>
          <select
            value={selectedStatus}
            onChange={(event) => setSelectedStatus(event.target.value as (typeof STATUS_OPTIONS)[number])}
            className={selectClass}
          >
            {STATUS_OPTIONS.map((status) => (
              <option key={status} value={status}>
                {status === "all" ? tx("全部状态", "All Statuses") : getStatusLabel(status, tx)}
              </option>
            ))}
          </select>
          <button
            type="button"
            onClick={() => void loadLogs()}
            className="inline-flex h-9 items-center justify-center gap-2 rounded-lg border border-border bg-card px-3 text-sm text-text-primary transition-colors hover:bg-border/60"
          >
            <RefreshCw className={`h-4 w-4 ${loading ? "animate-spin" : ""}`} />
            {tx("刷新", "Refresh")}
          </button>
        </div>
      </div>

      {loading ? (
        <div className="flex items-center justify-center h-[60vh]">
          <Loader2 className="h-6 w-6 animate-spin text-accent" />
        </div>
      ) : logs.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-16 text-text-muted">
          <ScrollText className="w-10 h-10 mb-3" />
          <p className="text-sm">{tx("当前还没有审计事件", "No audit events yet")}</p>
        </div>
      ) : (
        <div className="overflow-hidden rounded-xl border border-border bg-card">
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-border">
                  <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-text-muted">
                    {tx("时间", "Time")}
                  </th>
                  <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-text-muted">
                    {tx("动作", "Action")}
                  </th>
                  <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-text-muted">
                    {tx("操作者", "Actor")}
                  </th>
                  <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-text-muted">
                    {tx("状态", "Status")}
                  </th>
                  <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-text-muted">
                    {tx("目标", "Target")}
                  </th>
                  <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-text-muted">
                    {tx("详情", "Detail")}
                  </th>
                  <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-text-muted">
                    IP
                  </th>
                </tr>
              </thead>
              <tbody className="divide-y divide-border">
                {logs.map((log) => (
                  <tr key={log.id} className="transition-colors hover:bg-border/50">
                    <td className="whitespace-nowrap px-4 py-3 font-mono text-xs text-text-secondary">
                      {formatDate(log.created_at)}
                    </td>
                    <td className="px-4 py-3">
                      <span
                        className={`inline-block rounded px-2 py-0.5 text-xs font-medium ${getActionStyle(log.action)}`}
                      >
                        {getActionLabel(log.action, tx)}
                      </span>
                    </td>
                    <td className="px-4 py-3 text-xs text-text-primary">{log.actor || "-"}</td>
                    <td className="px-4 py-3">
                      <span
                        className={`inline-block rounded px-2 py-0.5 text-xs font-medium ${getStatusStyle(log.status)}`}
                      >
                        {getStatusLabel(log.status, tx)}
                      </span>
                    </td>
                    <td className="whitespace-nowrap px-4 py-3 font-mono text-xs text-text-muted">
                      {formatTarget(log)}
                    </td>
                    <td className="max-w-xl px-4 py-3 text-xs text-text-primary">{log.detail || "-"}</td>
                    <td className="px-4 py-3 font-mono text-xs text-text-muted">{log.ip || "-"}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  );
}
