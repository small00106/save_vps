import { useEffect, useState } from "react";
import {
  Loader2, Plus, X, ChevronDown, ChevronUp, Clock, Target, CheckCircle2, XCircle,
} from "lucide-react";
import {
  getPingTasks, createPingTask, getPingResults,
  type PingTask, type PingResult,
} from "../api/client";

export default function PingTasks() {
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
    } catch {}
    setSubmitting(false);
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center h-[60vh]">
        <Loader2 className="w-6 h-6 text-[#3b82f6] animate-spin" />
      </div>
    );
  }

  return (
    <div className="space-y-6 animate-[fadeIn_0.3s_ease-out]">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-bold text-[#fafafa]">Ping Tasks</h1>
        <button
          onClick={() => setShowForm(!showForm)}
          className="flex items-center gap-2 px-3 py-1.5 rounded-lg bg-[#3b82f6] hover:bg-blue-600 text-white text-sm font-medium transition-colors"
        >
          {showForm ? <X className="w-4 h-4" /> : <Plus className="w-4 h-4" />}
          {showForm ? "Cancel" : "New Task"}
        </button>
      </div>

      {showForm && (
        <div className="bg-[#18181b] border border-[#27272a] rounded-xl p-5 space-y-4">
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <div>
              <label className="block text-xs text-[#a1a1aa] mb-1">Name</label>
              <input
                value={formName}
                onChange={(e) => setFormName(e.target.value)}
                className="w-full h-9 px-3 rounded-lg bg-[#09090b] border border-[#27272a] text-white text-sm focus:outline-none focus:border-[#3b82f6] transition-colors"
                placeholder="Ping Google DNS"
              />
            </div>
            <div>
              <label className="block text-xs text-[#a1a1aa] mb-1">Target</label>
              <input
                value={formTarget}
                onChange={(e) => setFormTarget(e.target.value)}
                className="w-full h-9 px-3 rounded-lg bg-[#09090b] border border-[#27272a] text-white text-sm focus:outline-none focus:border-[#3b82f6] transition-colors"
                placeholder="8.8.8.8"
              />
            </div>
            <div>
              <label className="block text-xs text-[#a1a1aa] mb-1">Type</label>
              <select
                value={formType}
                onChange={(e) => setFormType(e.target.value)}
                className="w-full h-9 px-3 rounded-lg bg-[#09090b] border border-[#27272a] text-white text-sm focus:outline-none focus:border-[#3b82f6] transition-colors"
              >
                <option value="icmp">ICMP</option>
                <option value="tcp">TCP</option>
                <option value="http">HTTP</option>
              </select>
            </div>
            <div>
              <label className="block text-xs text-[#a1a1aa] mb-1">Interval (seconds)</label>
              <input
                type="number"
                value={formInterval}
                onChange={(e) => setFormInterval(Number(e.target.value))}
                min={5}
                className="w-full h-9 px-3 rounded-lg bg-[#09090b] border border-[#27272a] text-white text-sm focus:outline-none focus:border-[#3b82f6] transition-colors"
              />
            </div>
          </div>
          <button
            onClick={handleCreate}
            disabled={submitting || !formName || !formTarget}
            className="flex items-center gap-2 px-4 py-2 rounded-lg bg-[#3b82f6] hover:bg-blue-600 text-white text-sm font-medium transition-colors disabled:opacity-50"
          >
            {submitting && <Loader2 className="w-4 h-4 animate-spin" />}
            Create Task
          </button>
        </div>
      )}

      {tasks.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-16 text-[#71717a]">
          <Target className="w-10 h-10 mb-3" />
          <p className="text-sm">No ping tasks yet</p>
        </div>
      ) : (
        <div className="space-y-3">
          {tasks.map((task) => (
            <div
              key={task.id}
              className="bg-[#18181b] border border-[#27272a] rounded-xl overflow-hidden"
            >
              <button
                onClick={() => toggleExpand(task.id)}
                className="flex items-center gap-4 w-full px-5 py-4 hover:bg-[#232329] transition-colors text-left"
              >
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2 mb-1">
                    <span className="text-[#fafafa] font-medium text-sm">{task.name}</span>
                    {!task.enabled && (
                      <span className="px-2 py-0.5 rounded text-[10px] bg-[#27272a] text-[#71717a]">
                        Disabled
                      </span>
                    )}
                  </div>
                  <div className="flex items-center gap-4 text-xs text-[#71717a]">
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
                  <ChevronUp className="w-4 h-4 text-[#71717a] shrink-0" />
                ) : (
                  <ChevronDown className="w-4 h-4 text-[#71717a] shrink-0" />
                )}
              </button>

              {expandedId === task.id && (
                <div className="border-t border-[#27272a] px-5 py-3">
                  {resultsLoading ? (
                    <div className="flex items-center justify-center py-6">
                      <Loader2 className="w-4 h-4 text-[#3b82f6] animate-spin" />
                    </div>
                  ) : results.length === 0 ? (
                    <p className="text-sm text-[#71717a] py-3 text-center">No results yet</p>
                  ) : (
                    <div className="max-h-64 overflow-y-auto space-y-1">
                      {results.slice(0, 50).map((r) => (
                        <div key={r.id} className="flex items-center gap-3 py-1.5 text-sm">
                          {r.success ? (
                            <CheckCircle2 className="w-3.5 h-3.5 text-[#22c55e] shrink-0" />
                          ) : (
                            <XCircle className="w-3.5 h-3.5 text-[#ef4444] shrink-0" />
                          )}
                          <span className="text-[#a1a1aa] font-mono text-xs">
                            {new Date(r.timestamp).toLocaleTimeString()}
                          </span>
                          <span className="text-[#fafafa] text-xs">
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
