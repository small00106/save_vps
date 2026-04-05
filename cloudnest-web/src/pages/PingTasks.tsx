import { useEffect, useState } from "react";
import {
  Loader2, Plus, X, ChevronDown, ChevronUp, Clock, Target, CheckCircle2, XCircle, Trash2,
} from "lucide-react";
import {
  getPingTasks, createPingTask, getPingResults, deletePingTask,
  type PingTask, type PingResult,
} from "../api/client";
import { useI18n } from "../i18n/useI18n";

export default function PingTasks() {
  const { tx } = useI18n();
  const [tasks, setTasks] = useState<PingTask[]>([]);
  const [loading, setLoading] = useState(true);
  const [showForm, setShowForm] = useState(false);
  const [expandedId, setExpandedId] = useState<number | null>(null);
  const [results, setResults] = useState<PingResult[]>([]);
  const [resultsLoading, setResultsLoading] = useState(false);

  const [formName, setFormName] = useState("");
  const [formTarget, setFormTarget] = useState("");
  const [formType, setFormType] = useState("icmp");
  const [formInterval, setFormInterval] = useState(60);
  const [submitting, setSubmitting] = useState(false);
  const [deletingId, setDeletingId] = useState<number | null>(null);

  useEffect(() => {
    getPingTasks()
      .then(setTasks)
      .catch(() => {})
      .finally(() => setLoading(false));
  }, []);

  const toggleExpand = async (taskId: number) => {
    if (expandedId === taskId) {
      setExpandedId(null);
      return;
    }
    setExpandedId(taskId);
    setResultsLoading(true);
    try {
      const r = await getPingResults(taskId);
      setResults(r);
    } catch {
      setResults([]);
    } finally {
      setResultsLoading(false);
    }
  };

  const handleCreate = async () => {
    if (!formName || !formTarget) return;
    setSubmitting(true);
    try {
      const task = await createPingTask({
        name: formName,
        type: formType,
        target: formTarget,
        interval: formInterval,
        enabled: true,
      });
      setTasks((prev) => [...prev, task]);
      setShowForm(false);
      setFormName("");
      setFormTarget("");
      setFormType("icmp");
      setFormInterval(60);
    } catch (err) {
      void err;
    }
    setSubmitting(false);
  };

  const handleDelete = async (taskId: number) => {
    if (!confirm(tx("确认删除该 Ping 任务？", "Delete this ping task?"))) return;
    setDeletingId(taskId);
    try {
      await deletePingTask(taskId);
      setTasks((prev) => prev.filter((t) => t.id !== taskId));
      if (expandedId === taskId) {
        setExpandedId(null);
        setResults([]);
      }
    } catch {
      // ignore
    } finally {
      setDeletingId(null);
    }
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center h-[60vh]">
        <Loader2 className="h-6 w-6 animate-spin text-accent" />
      </div>
    );
  }

  return (
    <div className="space-y-6 animate-[fadeIn_0.3s_ease-out]">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-bold text-text-primary">{tx("Ping 任务", "Ping Tasks")}</h1>
        <button
          onClick={() => setShowForm(!showForm)}
          className="flex items-center gap-2 rounded-lg bg-accent px-3 py-1.5 text-sm font-medium text-white transition-colors hover:bg-accent-hover"
        >
          {showForm ? <X className="w-4 h-4" /> : <Plus className="w-4 h-4" />}
          {showForm ? tx("取消", "Cancel") : tx("新建任务", "New Task")}
        </button>
      </div>

      {showForm && (
        <div className="space-y-4 rounded-xl border border-border bg-card p-5">
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <div>
              <label className="mb-1 block text-xs text-text-secondary">{tx("名称", "Name")}</label>
              <input
                value={formName}
                onChange={(e) => setFormName(e.target.value)}
                className="h-9 w-full rounded-lg border border-border bg-bg px-3 text-sm text-text-primary transition-colors focus:border-accent focus:outline-none"
                placeholder={tx("例如：Ping Google DNS", "e.g. Ping Google DNS")}
              />
            </div>
            <div>
              <label className="mb-1 block text-xs text-text-secondary">{tx("目标地址", "Target")}</label>
              <input
                value={formTarget}
                onChange={(e) => setFormTarget(e.target.value)}
                className="h-9 w-full rounded-lg border border-border bg-bg px-3 text-sm text-text-primary transition-colors focus:border-accent focus:outline-none"
                placeholder="8.8.8.8"
              />
            </div>
            <div>
              <label className="mb-1 block text-xs text-text-secondary">{tx("类型", "Type")}</label>
              <select
                value={formType}
                onChange={(e) => setFormType(e.target.value)}
                className="h-9 w-full rounded-lg border border-border bg-bg px-3 text-sm text-text-primary transition-colors focus:border-accent focus:outline-none"
              >
                <option value="icmp">ICMP</option>
                <option value="tcp">TCP</option>
                <option value="http">HTTP</option>
              </select>
            </div>
            <div>
              <label className="mb-1 block text-xs text-text-secondary">
                {tx("间隔（秒）", "Interval (seconds)")}
              </label>
              <input
                type="number"
                value={formInterval}
                onChange={(e) => setFormInterval(Number(e.target.value))}
                min={5}
                className="h-9 w-full rounded-lg border border-border bg-bg px-3 text-sm text-text-primary transition-colors focus:border-accent focus:outline-none"
              />
            </div>
          </div>
          <button
            onClick={handleCreate}
            disabled={submitting || !formName || !formTarget}
            className="flex items-center gap-2 rounded-lg bg-accent px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-accent-hover disabled:opacity-50"
          >
            {submitting && <Loader2 className="w-4 h-4 animate-spin" />}
            {tx("创建任务", "Create Task")}
          </button>
        </div>
      )}

      {tasks.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-16 text-text-muted">
          <Target className="w-10 h-10 mb-3" />
          <p className="text-sm">{tx("暂无 Ping 任务", "No ping tasks yet")}</p>
        </div>
      ) : (
        <div className="space-y-3">
          {tasks.map((task) => (
            <div
              key={task.id}
              className="overflow-hidden rounded-xl border border-border bg-card"
            >
              <div className="flex items-center gap-3 px-5 py-4 transition-colors hover:bg-border/50">
                <button
                  onClick={() => toggleExpand(task.id)}
                  className="flex items-center gap-4 flex-1 min-w-0 text-left"
                >
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2 mb-1">
                      <span className="text-sm font-medium text-text-primary">{task.name}</span>
                      {!task.enabled && (
                        <span className="rounded bg-border px-2 py-0.5 text-[10px] text-text-muted">
                          {tx("已禁用", "Disabled")}
                        </span>
                      )}
                    </div>
                    <div className="flex items-center gap-4 text-xs text-text-muted">
                      <span className="flex items-center gap-1">
                        <Target className="w-3 h-3" />
                        {task.target}
                      </span>
                      <span className="flex items-center gap-1">
                        <Clock className="w-3 h-3" />
                        {task.interval}s
                      </span>
                    </div>
                  </div>
                  {expandedId === task.id ? (
                    <ChevronUp className="h-4 w-4 shrink-0 text-text-muted" />
                  ) : (
                    <ChevronDown className="h-4 w-4 shrink-0 text-text-muted" />
                  )}
                </button>
                <button
                  onClick={() => handleDelete(task.id)}
                  disabled={deletingId === task.id}
                  className="rounded-md p-1.5 text-text-muted transition-colors hover:bg-offline/10 hover:text-offline disabled:opacity-50"
                  title={tx("删除任务", "Delete task")}
                >
                  {deletingId === task.id ? (
                    <Loader2 className="w-4 h-4 animate-spin" />
                  ) : (
                    <Trash2 className="w-4 h-4" />
                  )}
                </button>
              </div>

              {expandedId === task.id && (
                <div className="border-t border-border px-5 py-3">
                  {resultsLoading ? (
                    <div className="flex items-center justify-center py-6">
                      <Loader2 className="h-4 w-4 animate-spin text-accent" />
                    </div>
                  ) : results.length === 0 ? (
                    <p className="py-3 text-center text-sm text-text-muted">
                      {tx("暂无结果", "No results yet")}
                    </p>
                  ) : (
                    <div className="max-h-64 overflow-y-auto space-y-1">
                      {results.slice(0, 50).map((r) => (
                        <div key={r.id} className="flex items-center gap-3 py-1.5 text-sm">
                          {r.success ? (
                            <CheckCircle2 className="h-3.5 w-3.5 shrink-0 text-online" />
                          ) : (
                            <XCircle className="h-3.5 w-3.5 shrink-0 text-offline" />
                          )}
                          <span className="font-mono text-xs text-text-secondary">
                            {new Date(r.timestamp).toLocaleTimeString()}
                          </span>
                          <span className="text-xs text-text-primary">
                            {r.latency.toFixed(1)} ms
                          </span>
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
