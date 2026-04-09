import { useCallback, useEffect, useState } from "react";
import { Loader2, RefreshCw, ScrollText } from "lucide-react";
import { getAuditLogs, type AuditLog as AuditLogEntry } from "../api/client";
import { useI18n } from "../i18n/useI18n";
import { EmptyState, PageHeader, SectionCard, SelectField } from "../components/ui";

function formatDate(s: string): string {
  if (!s) return "-";
  return new Date(s).toLocaleString();
}

const ACTION_STYLES: Record<string, string> = {
  login_success: "border-sky-200 bg-sky-50 text-sky-700 dark:border-sky-500/20 dark:bg-sky-500/10 dark:text-sky-300",
  login_failed: "border-red-200 bg-red-50 text-red-700 dark:border-red-500/20 dark:bg-red-500/10 dark:text-red-300",
  logout: "border-slate-200 bg-slate-50 text-slate-700 dark:border-slate-500/20 dark:bg-slate-500/10 dark:text-slate-300",
  password_changed: "border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-500/20 dark:bg-amber-500/10 dark:text-amber-300",
  password_change_failed: "border-red-200 bg-red-50 text-red-700 dark:border-red-500/20 dark:bg-red-500/10 dark:text-red-300",
  command_exec_requested: "border-cyan-200 bg-cyan-50 text-cyan-700 dark:border-cyan-500/20 dark:bg-cyan-500/10 dark:text-cyan-300",
  command_exec_rejected: "border-red-200 bg-red-50 text-red-700 dark:border-red-500/20 dark:bg-red-500/10 dark:text-red-300",
  command_exec_completed: "border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-500/20 dark:bg-emerald-500/10 dark:text-emerald-300",
  settings_updated: "border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-500/20 dark:bg-amber-500/10 dark:text-amber-300",
  alert_fired: "border-orange-200 bg-orange-50 text-orange-700 dark:border-orange-500/20 dark:bg-orange-500/10 dark:text-orange-300",
};

const STATUS_STYLES: Record<string, string> = {
  success: "border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-500/20 dark:bg-emerald-500/10 dark:text-emerald-300",
  failed: "border-red-200 bg-red-50 text-red-700 dark:border-red-500/20 dark:bg-red-500/10 dark:text-red-300",
  info: "border-slate-200 bg-slate-50 text-slate-700 dark:border-slate-500/20 dark:bg-slate-500/10 dark:text-slate-300",
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
  return ACTION_STYLES[action] || "border-border bg-surface text-text-secondary";
}

function getStatusStyle(status: string): string {
  return STATUS_STYLES[status] || "border-border bg-surface text-text-secondary";
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

  const selectClass = "rounded-2xl border border-border bg-surface px-4 py-3 text-sm text-text-primary outline-none";

  return (
    <div className="space-y-6">
      <PageHeader
        eyebrow={tx("治理记录", "Governance Record")}
        title={tx("审计日志", "Audit Log")}
        description={tx("按动作与状态过滤关键事件，快速定位登录、设置、命令与告警相关记录。", "Filter critical events by action and status to locate login, settings, commands, and alerts quickly.")}
        actions={
          <button type="button" onClick={() => void loadLogs()} className="inline-flex items-center gap-2 rounded-2xl border border-border bg-surface px-4 py-2.5 text-sm font-medium text-text-primary transition-colors hover:border-border-hover hover:bg-card">
            <RefreshCw className={`h-4 w-4 ${loading ? "animate-spin" : ""}`} />
            {tx("刷新", "Refresh")}
          </button>
        }
      />

      <SectionCard title={tx("筛选条件", "Filters")} description={tx("默认拉取最近 200 条记录。", "Fetch the latest 200 records by default.")}> 
        <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-[260px_260px_auto]">
          <label className="space-y-2">
            <span className="text-sm font-medium text-text-secondary">{tx("动作", "Action")}</span>
            <SelectField value={selectedAction} onChange={(event) => setSelectedAction(event.target.value as (typeof ACTION_OPTIONS)[number])} className={selectClass}>
              {ACTION_OPTIONS.map((action) => (
                <option key={action} value={action}>
                  {action === "all" ? tx("全部动作", "All Actions") : getActionLabel(action, tx)}
                </option>
              ))}
            </SelectField>
          </label>
          <label className="space-y-2">
            <span className="text-sm font-medium text-text-secondary">{tx("状态", "Status")}</span>
            <SelectField value={selectedStatus} onChange={(event) => setSelectedStatus(event.target.value as (typeof STATUS_OPTIONS)[number])} className={selectClass}>
              {STATUS_OPTIONS.map((status) => (
                <option key={status} value={status}>
                  {status === "all" ? tx("全部状态", "All Statuses") : getStatusLabel(status, tx)}
                </option>
              ))}
            </SelectField>
          </label>
        </div>
      </SectionCard>

      {loading ? (
        <div className="flex items-center justify-center py-24">
          <Loader2 className="h-8 w-8 animate-spin text-accent" />
        </div>
      ) : logs.length === 0 ? (
        <EmptyState icon={ScrollText} title={tx("当前还没有审计事件", "No audit events yet")} description={tx("当系统发生登录、命令、设置和告警相关动作时，这里会记录详细上下文。", "This page will record detailed context for login, command, settings, and alert actions.")} />
      ) : (
        <SectionCard title={tx("事件列表", "Events")} description={tx("表格保留较高信息密度，优先照顾桌面端排查效率。", "The table keeps a dense layout for efficient investigation on desktop.")}> 
          <div className="overflow-hidden rounded-2xl border border-border">
            <div className="overflow-x-auto">
              <table className="min-w-full text-sm">
                <thead className="bg-surface-subtle text-xs uppercase tracking-[0.18em] text-text-muted">
                  <tr>
                    <th className="px-4 py-3 text-left">{tx("时间", "Time")}</th>
                    <th className="px-4 py-3 text-left">{tx("动作", "Action")}</th>
                    <th className="px-4 py-3 text-left">{tx("操作者", "Actor")}</th>
                    <th className="px-4 py-3 text-left">{tx("状态", "Status")}</th>
                    <th className="px-4 py-3 text-left">{tx("目标", "Target")}</th>
                    <th className="px-4 py-3 text-left">{tx("详情", "Detail")}</th>
                    <th className="px-4 py-3 text-left">IP</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-border bg-card">
                  {logs.map((log) => (
                    <tr key={log.id} className="align-top transition-colors hover:bg-surface">
                      <td className="whitespace-nowrap px-4 py-3 font-mono text-xs text-text-secondary">{formatDate(log.created_at)}</td>
                      <td className="px-4 py-3">
                        <span className={`inline-flex rounded-full border px-2.5 py-1 text-[11px] font-medium ${getActionStyle(log.action)}`}>
                          {getActionLabel(log.action, tx)}
                        </span>
                      </td>
                      <td className="px-4 py-3 text-xs text-text-primary">{log.actor || "-"}</td>
                      <td className="px-4 py-3">
                        <span className={`inline-flex rounded-full border px-2.5 py-1 text-[11px] font-medium ${getStatusStyle(log.status)}`}>
                          {getStatusLabel(log.status, tx)}
                        </span>
                      </td>
                      <td className="whitespace-nowrap px-4 py-3 font-mono text-xs text-text-muted">{formatTarget(log)}</td>
                      <td className="max-w-xl px-4 py-3 text-xs leading-6 text-text-primary">{log.detail || "-"}</td>
                      <td className="px-4 py-3 font-mono text-xs text-text-muted">{log.ip || "-"}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        </SectionCard>
      )}
    </div>
  );
}
