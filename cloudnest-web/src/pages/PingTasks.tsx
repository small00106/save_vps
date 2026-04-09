import { useEffect, useState } from "react";
import {
  CheckCircle2,
  ChevronDown,
  ChevronUp,
  Clock,
  Loader2,
  Plus,
  Target,
  Trash2,
  X,
  XCircle,
} from "lucide-react";
import {
  createPingTask,
  deletePingTask,
  getPingResults,
  getPingTasks,
  type PingResult,
  type PingTask,
} from "../api/client";
import { useI18n } from "../i18n/useI18n";
import { EmptyState, PageHeader, SectionCard, SelectField, StatusBadge } from "../components/ui";

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
      const data = await getPingResults(taskId);
      setResults(data);
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
    } catch {
      // ignore
    } finally {
      setSubmitting(false);
    }
  };

  const handleDelete = async (taskId: number) => {
    if (!confirm(tx("确认删除该 Ping 任务？", "Delete this ping task?"))) return;
    setDeletingId(taskId);
    try {
      await deletePingTask(taskId);
      setTasks((prev) => prev.filter((task) => task.id !== taskId));
      if (expandedId === taskId) {
        setExpandedId(null);
        setResults([]);
      }
    } finally {
      setDeletingId(null);
    }
  };

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
        eyebrow={tx("分布式探测", "Distributed Probing")}
        title={tx("Ping 任务", "Ping Tasks")}
        description={tx("为多节点探测任务提供统一的配置、展开查看与删除入口。", "Provide one place to configure, inspect, and delete distributed probe tasks.")}
        actions={
          <button
            type="button"
            onClick={() => setShowForm((value) => !value)}
            className="gradient-button inline-flex items-center gap-2 rounded-2xl px-4 py-2.5 text-sm font-medium text-white"
          >
            {showForm ? <X className="h-4 w-4" /> : <Plus className="h-4 w-4" />}
            {showForm ? tx("取消", "Cancel") : tx("新建任务", "New Task")}
          </button>
        }
      />

      {showForm ? (
        <SectionCard title={tx("新建任务", "Create Task")} description={tx("支持 ICMP / TCP / HTTP 三类探测。", "Support ICMP, TCP, and HTTP probes.")}>
          <div className="grid gap-4 md:grid-cols-2">
            <label className="space-y-2">
              <span className="text-sm font-medium text-text-secondary">{tx("名称", "Name")}</span>
              <input value={formName} onChange={(e) => setFormName(e.target.value)} className="w-full rounded-2xl border border-border bg-surface px-4 py-3 text-sm text-text-primary outline-none" placeholder={tx("例如：Ping Google DNS", "e.g. Ping Google DNS")} />
            </label>
            <label className="space-y-2">
              <span className="text-sm font-medium text-text-secondary">{tx("目标地址", "Target")}</span>
              <input value={formTarget} onChange={(e) => setFormTarget(e.target.value)} className="w-full rounded-2xl border border-border bg-surface px-4 py-3 text-sm text-text-primary outline-none" placeholder="8.8.8.8" />
            </label>
            <label className="space-y-2">
              <span className="text-sm font-medium text-text-secondary">{tx("类型", "Type")}</span>
              <SelectField value={formType} onChange={(e) => setFormType(e.target.value)}>
                <option value="icmp">ICMP</option>
                <option value="tcp">TCP</option>
                <option value="http">HTTP</option>
              </SelectField>
            </label>
            <label className="space-y-2">
              <span className="text-sm font-medium text-text-secondary">{tx("间隔（秒）", "Interval (seconds)")}</span>
              <input type="number" min={5} value={formInterval} onChange={(e) => setFormInterval(Number(e.target.value))} className="w-full rounded-2xl border border-border bg-surface px-4 py-3 text-sm text-text-primary outline-none" />
            </label>
          </div>

          <div className="mt-4 flex justify-end">
            <button
              type="button"
              onClick={handleCreate}
              disabled={submitting || !formName || !formTarget}
              className="gradient-button inline-flex items-center gap-2 rounded-2xl px-4 py-2.5 text-sm font-medium text-white disabled:opacity-50"
            >
              {submitting ? <Loader2 className="h-4 w-4 animate-spin" /> : null}
              {tx("创建任务", "Create Task")}
            </button>
          </div>
        </SectionCard>
      ) : null}

      {tasks.length === 0 ? (
        <EmptyState
          icon={Target}
          title={tx("暂无 Ping 任务", "No ping tasks yet")}
          description={tx("新建后即可让多个节点持续执行探测，并在展开项里查看最近结果。", "Create a task to let multiple nodes run probes and inspect recent results in the expanded panel.")}
        />
      ) : (
        <SectionCard title={tx("任务列表", "Task List")} description={tx("每个任务都可以展开查看最近 50 条探测结果。", "Each task can be expanded to inspect the latest 50 probe results.")}> 
          <div className="space-y-3">
            {tasks.map((task) => (
              <div key={task.id} className="overflow-hidden rounded-3xl border border-border bg-surface">
                <div className="flex items-center gap-3 px-4 py-4 md:px-5">
                  <button type="button" onClick={() => void toggleExpand(task.id)} className="flex min-w-0 flex-1 items-center gap-4 text-left">
                    <div className="min-w-0 flex-1 space-y-2">
                      <div className="flex flex-wrap items-center gap-3">
                        <span className="truncate text-sm font-semibold text-text-primary">{task.name}</span>
                        <StatusBadge tone={task.enabled ? "success" : "danger"} label={task.enabled ? tx("启用中", "Enabled") : tx("已禁用", "Disabled")} />
                        <span className="rounded-full border border-border bg-card px-3 py-1 text-xs text-text-secondary">{task.type.toUpperCase()}</span>
                      </div>
                      <div className="flex flex-wrap items-center gap-4 text-xs text-text-muted">
                        <span className="flex items-center gap-1"><Target className="h-3 w-3" />{task.target}</span>
                        <span className="flex items-center gap-1"><Clock className="h-3 w-3" />{task.interval}s</span>
                      </div>
                    </div>
                    {expandedId === task.id ? <ChevronUp className="h-4 w-4 text-text-muted" /> : <ChevronDown className="h-4 w-4 text-text-muted" />}
                  </button>
                  <button
                    type="button"
                    onClick={() => void handleDelete(task.id)}
                    disabled={deletingId === task.id}
                    className="rounded-2xl border border-border bg-card p-2 text-text-muted transition-colors hover:border-offline/20 hover:bg-offline/10 hover:text-offline disabled:opacity-50"
                    title={tx("删除任务", "Delete task")}
                  >
                    {deletingId === task.id ? <Loader2 className="h-4 w-4 animate-spin" /> : <Trash2 className="h-4 w-4" />}
                  </button>
                </div>

                {expandedId === task.id ? (
                  <div className="border-t border-border bg-card px-4 py-4 md:px-5">
                    {resultsLoading ? (
                      <div className="flex items-center justify-center py-8">
                        <Loader2 className="h-5 w-5 animate-spin text-accent" />
                      </div>
                    ) : results.length === 0 ? (
                      <p className="py-4 text-sm text-text-muted">{tx("暂无结果", "No results yet")}</p>
                    ) : (
                      <div className="space-y-2">
                        {results.slice(0, 50).map((result) => (
                          <div key={result.id} className="flex items-center gap-3 rounded-2xl border border-border bg-surface px-3 py-3 text-sm">
                            {result.success ? <CheckCircle2 className="h-4 w-4 text-online" /> : <XCircle className="h-4 w-4 text-offline" />}
                            <span className="font-mono text-xs text-text-muted">{new Date(result.timestamp).toLocaleTimeString()}</span>
                            <span className="text-sm text-text-primary">{result.latency.toFixed(1)} ms</span>
                          </div>
                        ))}
                      </div>
                    )}
                  </div>
                ) : null}
              </div>
            ))}
          </div>
        </SectionCard>
      )}
    </div>
  );
}
