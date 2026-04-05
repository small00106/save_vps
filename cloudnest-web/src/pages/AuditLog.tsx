import { useEffect, useState } from "react";
import { Loader2, ScrollText } from "lucide-react";
import { getAuditLogs, type AuditLog as AuditLogEntry } from "../api/client";
import { useI18n } from "../i18n/useI18n";

function formatDate(s: string): string {
  if (!s) return "-";
  return new Date(s).toLocaleString();
}

const ACTION_STYLES: Record<string, string> = {
  login: "bg-blue-500/10 text-[#3b82f6]",
  logout: "bg-zinc-500/10 text-[#a1a1aa]",
  create: "bg-green-500/10 text-[#22c55e]",
  update: "bg-amber-500/10 text-[#f59e0b]",
  delete: "bg-red-500/10 text-[#ef4444]",
  alert: "bg-orange-500/10 text-[#f97316]",
};

function getActionStyle(action: string): string {
  const key = Object.keys(ACTION_STYLES).find((k) => action.toLowerCase().includes(k));
  return key ? ACTION_STYLES[key] : "bg-border text-text-secondary";
}

export default function AuditLog() {
  const { tx } = useI18n();
  const [logs, setLogs] = useState<AuditLogEntry[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    getAuditLogs()
      .then((data) => setLogs(Array.isArray(data) ? data : []))
      .catch(() => setLogs([]))
      .finally(() => setLoading(false));
  }, []);

  if (loading) {
    return (
      <div className="flex items-center justify-center h-[60vh]">
        <Loader2 className="h-6 w-6 animate-spin text-accent" />
      </div>
    );
  }

  return (
    <div className="space-y-4 animate-[fadeIn_0.3s_ease-out]">
      <h1 className="text-xl font-bold text-text-primary">{tx("审计日志", "Audit Log")}</h1>

      {logs.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-16 text-text-muted">
          <ScrollText className="w-10 h-10 mb-3" />
          <p className="text-sm">{tx("暂无审计日志", "No audit logs yet")}</p>
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
                        className={`inline-block px-2 py-0.5 rounded text-xs font-medium ${getActionStyle(log.action)}`}
                      >
                        {log.action}
                      </span>
                    </td>
                    <td className="max-w-md truncate px-4 py-3 text-xs text-text-primary">
                      {log.detail || "-"}
                    </td>
                    <td className="px-4 py-3 font-mono text-xs text-text-muted">
                      {log.ip || "-"}
                    </td>
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
