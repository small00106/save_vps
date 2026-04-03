import { useEffect, useState } from "react";
import {
  Loader2, Plus, X, Trash2, Bell, Send, Globe, Mail,
} from "lucide-react";
import {
  getAlertRules, createAlertRule, updateAlertRule, deleteAlertRule,
  getAlertChannels, createAlertChannel,
  type AlertRule, type AlertChannel,
} from "../api/client";

const CHANNEL_TYPE_STYLES: Record<string, { bg: string; text: string; icon: typeof Send }> = {
  telegram: { bg: "bg-blue-500/10", text: "text-[#3b82f6]", icon: Send },
  webhook: { bg: "bg-zinc-500/10", text: "text-[#a1a1aa]", icon: Globe },
  email: { bg: "bg-amber-500/10", text: "text-[#f59e0b]", icon: Mail },
  bark: { bg: "bg-green-500/10", text: "text-[#22c55e]", icon: Bell },
  serverchan: { bg: "bg-purple-500/10", text: "text-[#a855f7]", icon: Send },
};

const METRICS = ["cpu", "mem", "disk", "offline"];
const OPERATORS = ["gt", "lt"];

export default function Alerts() {
  const [rules, setRules] = useState<AlertRule[]>([]);
  const [channels, setChannels] = useState<AlertChannel[]>([]);
  const [loading, setLoading] = useState(true);
  const [showRuleForm, setShowRuleForm] = useState(false);
  const [showChannelForm, setShowChannelForm] = useState(false);

  // Rule form
  const [ruleName, setRuleName] = useState("");
  const [ruleNodeUuid, setRuleNodeUuid] = useState("");
  const [ruleMetric, setRuleMetric] = useState("cpu");
  const [ruleOperator, setRuleOperator] = useState("gt");
  const [ruleThreshold, setRuleThreshold] = useState(80);
  const [ruleDuration, setRuleDuration] = useState(60);
  const [ruleChannelId, setRuleChannelId] = useState(0);
  const [ruleSubmitting, setRuleSubmitting] = useState(false);

  // Channel form
  const [channelName, setChannelName] = useState("");
  const [channelType, setChannelType] = useState<"webhook" | "email" | "telegram" | "bark" | "serverchan">("webhook");
  const [channelConfigUrl, setChannelConfigUrl] = useState("");
  const [channelConfigSendKey, setChannelConfigSendKey] = useState("");
  const [channelConfigBotToken, setChannelConfigBotToken] = useState("");
  const [channelConfigChatId, setChannelConfigChatId] = useState("");
  const [channelConfigSmtpHost, setChannelConfigSmtpHost] = useState("");
  const [channelConfigSmtpPort, setChannelConfigSmtpPort] = useState("587");
  const [channelConfigSmtpUser, setChannelConfigSmtpUser] = useState("");
  const [channelConfigSmtpPass, setChannelConfigSmtpPass] = useState("");
  const [channelConfigFrom, setChannelConfigFrom] = useState("");
  const [channelConfigTo, setChannelConfigTo] = useState("");
  const [channelSubmitting, setChannelSubmitting] = useState(false);

  useEffect(() => {
    Promise.all([getAlertRules(), getAlertChannels()])
      .then(([r, c]) => {
        setRules(r);
        setChannels(c);
      })
      .catch(() => {})
      .finally(() => setLoading(false));
  }, []);

  const handleCreateRule = async () => {
    if (!ruleName) return;
    setRuleSubmitting(true);
    try {
      const rule = await createAlertRule({
        name: ruleName,
        node_uuid: ruleNodeUuid,
        metric: ruleMetric,
        operator: ruleOperator,
        threshold: ruleThreshold,
        duration: ruleDuration,
        channel_id: ruleChannelId,
        enabled: true,
      });
      setRules((prev) => [...prev, rule]);
      setShowRuleForm(false);
      setRuleName("");
      setRuleNodeUuid("");
    } catch {}
    setRuleSubmitting(false);
  };

  const handleToggleRule = async (rule: AlertRule) => {
    try {
      const updated = await updateAlertRule(rule.id, { enabled: !rule.enabled });
      setRules((prev) => prev.map((r) => (r.id === rule.id ? updated : r)));
    } catch {}
  };

  const handleDeleteRule = async (id: number) => {
    try {
      await deleteAlertRule(id);
      setRules((prev) => prev.filter((r) => r.id !== id));
    } catch {}
  };

  const handleCreateChannel = async () => {
    if (!channelName) return;
    setChannelSubmitting(true);

    let configObj: Record<string, string> = {};
    switch (channelType) {
      case "webhook":
      case "bark":
        configObj = { url: channelConfigUrl, server_url: channelConfigUrl };
        break;
      case "telegram":
        configObj = { bot_token: channelConfigBotToken, chat_id: channelConfigChatId };
        break;
      case "email":
        configObj = {
          smtp_host: channelConfigSmtpHost, smtp_port: channelConfigSmtpPort,
          username: channelConfigSmtpUser, password: channelConfigSmtpPass,
          from: channelConfigFrom, to: channelConfigTo,
        };
        break;
      case "serverchan":
        configObj = { send_key: channelConfigSendKey };
        break;
    }

    try {
      const ch = await createAlertChannel({
        name: channelName,
        type: channelType,
        config: JSON.stringify(configObj),
      });
      setChannels((prev) => [...prev, ch]);
      setShowChannelForm(false);
      setChannelName("");
      setChannelConfigUrl("");
      setChannelConfigBotToken("");
      setChannelConfigChatId("");
    } catch {}
    setChannelSubmitting(false);
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center h-[60vh]">
        <Loader2 className="w-6 h-6 text-[#3b82f6] animate-spin" />
      </div>
    );
  }

  const inputClass =
    "w-full h-9 px-3 rounded-lg bg-[#09090b] border border-[#27272a] text-white text-sm focus:outline-none focus:border-[#3b82f6] transition-colors";

  return (
    <div className="space-y-8 animate-[fadeIn_0.3s_ease-out]">
      {/* Alert Rules */}
      <section>
        <div className="flex items-center justify-between mb-4">
          <div className="flex items-center gap-2">
            <Bell className="w-5 h-5 text-[#3b82f6]" />
            <h2 className="text-lg font-bold text-[#fafafa]">Alert Rules</h2>
          </div>
          <button
            onClick={() => setShowRuleForm(!showRuleForm)}
            className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-[#3b82f6] hover:bg-blue-600 text-white text-sm font-medium transition-colors"
          >
            {showRuleForm ? <X className="w-4 h-4" /> : <Plus className="w-4 h-4" />}
            {showRuleForm ? "Cancel" : "New Rule"}
          </button>
        </div>

        {showRuleForm && (
          <div className="bg-[#18181b] border border-[#27272a] rounded-xl p-5 mb-4 space-y-4">
            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
              <div>
                <label className="block text-xs text-[#a1a1aa] mb-1">Name</label>
                <input value={ruleName} onChange={(e) => setRuleName(e.target.value)} className={inputClass} placeholder="High CPU Alert" />
              </div>
              <div>
                <label className="block text-xs text-[#a1a1aa] mb-1">Node UUID (optional)</label>
                <input value={ruleNodeUuid} onChange={(e) => setRuleNodeUuid(e.target.value)} className={inputClass} placeholder="All nodes if empty" />
              </div>
              <div>
                <label className="block text-xs text-[#a1a1aa] mb-1">Metric</label>
                <select value={ruleMetric} onChange={(e) => setRuleMetric(e.target.value)} className={inputClass}>
                  {METRICS.map((m) => (
                    <option key={m} value={m}>{m}</option>
                  ))}
                </select>
              </div>
              <div>
                <label className="block text-xs text-[#a1a1aa] mb-1">Operator</label>
                <select value={ruleOperator} onChange={(e) => setRuleOperator(e.target.value)} className={inputClass}>
                  {OPERATORS.map((o) => (
                    <option key={o} value={o}>{o}</option>
                  ))}
                </select>
              </div>
              <div>
                <label className="block text-xs text-[#a1a1aa] mb-1">Threshold</label>
                <input type="number" value={ruleThreshold} onChange={(e) => setRuleThreshold(Number(e.target.value))} className={inputClass} />
              </div>
              <div>
                <label className="block text-xs text-[#a1a1aa] mb-1">Duration (s)</label>
                <input type="number" value={ruleDuration} onChange={(e) => setRuleDuration(Number(e.target.value))} className={inputClass} />
              </div>
              <div>
                <label className="block text-xs text-[#a1a1aa] mb-1">Channel</label>
                <select value={ruleChannelId} onChange={(e) => setRuleChannelId(Number(e.target.value))} className={inputClass}>
                  <option value={0}>None</option>
                  {channels.map((c) => (
                    <option key={c.id} value={c.id}>{c.name}</option>
                  ))}
                </select>
              </div>
            </div>
            <button
              onClick={handleCreateRule}
              disabled={ruleSubmitting || !ruleName}
              className="flex items-center gap-2 px-4 py-2 rounded-lg bg-[#3b82f6] hover:bg-blue-600 text-white text-sm font-medium transition-colors disabled:opacity-50"
            >
              {ruleSubmitting && <Loader2 className="w-4 h-4 animate-spin" />}
              Create Rule
            </button>
          </div>
        )}

        {rules.length === 0 ? (
          <div className="bg-[#18181b] border border-[#27272a] rounded-xl p-8 text-center text-[#71717a] text-sm">
            No alert rules configured
          </div>
        ) : (
          <div className="bg-[#18181b] border border-[#27272a] rounded-xl overflow-hidden divide-y divide-[#27272a]">
            {rules.map((rule) => (
              <div
                key={rule.id}
                className="flex items-center gap-4 px-5 py-3 hover:bg-[#232329] transition-colors"
              >
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2 mb-0.5">
                    <span className="text-sm text-[#fafafa] font-medium">{rule.name}</span>
                    <span className="px-2 py-0.5 rounded text-[10px] font-medium bg-blue-500/10 text-[#3b82f6]">
                      {rule.metric}
                    </span>
                  </div>
                  <span className="text-xs text-[#71717a]">
                    {rule.operator} {rule.threshold} for {rule.duration}s
                  </span>
                </div>
                <button
                  onClick={() => handleToggleRule(rule)}
                  className={`relative w-9 h-5 rounded-full transition-colors ${
                    rule.enabled ? "bg-[#3b82f6]" : "bg-[#27272a]"
                  }`}
                >
                  <span
                    className={`absolute top-0.5 w-4 h-4 rounded-full bg-white transition-transform ${
                      rule.enabled ? "left-[18px]" : "left-0.5"
                    }`}
                  />
                </button>
                <button
                  onClick={() => handleDeleteRule(rule.id)}
                  className="text-[#71717a] hover:text-[#ef4444] transition-colors"
                >
                  <Trash2 className="w-4 h-4" />
                </button>
              </div>
            ))}
          </div>
        )}
      </section>

      {/* Notification Channels */}
      <section>
        <div className="flex items-center justify-between mb-4">
          <div className="flex items-center gap-2">
            <Send className="w-5 h-5 text-[#a855f7]" />
            <h2 className="text-lg font-bold text-[#fafafa]">Notification Channels</h2>
          </div>
          <button
            onClick={() => setShowChannelForm(!showChannelForm)}
            className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-[#3b82f6] hover:bg-blue-600 text-white text-sm font-medium transition-colors"
          >
            {showChannelForm ? <X className="w-4 h-4" /> : <Plus className="w-4 h-4" />}
            {showChannelForm ? "Cancel" : "New Channel"}
          </button>
        </div>

        {showChannelForm && (
          <div className="bg-[#18181b] border border-[#27272a] rounded-xl p-5 mb-4 space-y-4">
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
              <div>
                <label className="block text-xs text-[#a1a1aa] mb-1">Name</label>
                <input value={channelName} onChange={(e) => setChannelName(e.target.value)} className={inputClass} placeholder="Ops Team" />
              </div>
              <div>
                <label className="block text-xs text-[#a1a1aa] mb-1">Type</label>
                <select value={channelType} onChange={(e) => setChannelType(e.target.value as typeof channelType)} className={inputClass}>
                  <option value="webhook">Webhook</option>
                  <option value="telegram">Telegram</option>
                  <option value="email">Email</option>
                  <option value="bark">Bark</option>
                  <option value="serverchan">ServerChan</option>
                </select>
              </div>
            </div>

            {/* Webhook / Bark */}
            {(channelType === "webhook" || channelType === "bark") && (
              <div>
                <label className="block text-xs text-[#a1a1aa] mb-1">URL</label>
                <input value={channelConfigUrl} onChange={(e) => setChannelConfigUrl(e.target.value)} className={inputClass} placeholder="https://hooks.example.com/..." />
              </div>
            )}

            {/* Telegram */}
            {channelType === "telegram" && (
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                <div>
                  <label className="block text-xs text-[#a1a1aa] mb-1">Bot Token</label>
                  <input value={channelConfigBotToken} onChange={(e) => setChannelConfigBotToken(e.target.value)} className={inputClass} placeholder="123456:ABC-DEF..." />
                </div>
                <div>
                  <label className="block text-xs text-[#a1a1aa] mb-1">Chat ID</label>
                  <input value={channelConfigChatId} onChange={(e) => setChannelConfigChatId(e.target.value)} className={inputClass} placeholder="-1001234567890" />
                </div>
              </div>
            )}

            {/* Email */}
            {channelType === "email" && (
              <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
                <div>
                  <label className="block text-xs text-[#a1a1aa] mb-1">SMTP Host</label>
                  <input value={channelConfigSmtpHost} onChange={(e) => setChannelConfigSmtpHost(e.target.value)} className={inputClass} placeholder="smtp.gmail.com" />
                </div>
                <div>
                  <label className="block text-xs text-[#a1a1aa] mb-1">SMTP Port</label>
                  <input value={channelConfigSmtpPort} onChange={(e) => setChannelConfigSmtpPort(e.target.value)} className={inputClass} placeholder="587" />
                </div>
                <div>
                  <label className="block text-xs text-[#a1a1aa] mb-1">Username</label>
                  <input value={channelConfigSmtpUser} onChange={(e) => setChannelConfigSmtpUser(e.target.value)} className={inputClass} />
                </div>
                <div>
                  <label className="block text-xs text-[#a1a1aa] mb-1">Password</label>
                  <input type="password" value={channelConfigSmtpPass} onChange={(e) => setChannelConfigSmtpPass(e.target.value)} className={inputClass} />
                </div>
                <div>
                  <label className="block text-xs text-[#a1a1aa] mb-1">From</label>
                  <input value={channelConfigFrom} onChange={(e) => setChannelConfigFrom(e.target.value)} className={inputClass} placeholder="alerts@example.com" />
                </div>
                <div>
                  <label className="block text-xs text-[#a1a1aa] mb-1">To</label>
                  <input value={channelConfigTo} onChange={(e) => setChannelConfigTo(e.target.value)} className={inputClass} placeholder="admin@example.com" />
                </div>
              </div>
            )}

            {/* ServerChan */}
            {channelType === "serverchan" && (
              <div>
                <label className="block text-xs text-[#a1a1aa] mb-1">SendKey</label>
                <input value={channelConfigSendKey} onChange={(e) => setChannelConfigSendKey(e.target.value)} className={inputClass} placeholder="SCT1234567890" />
              </div>
            )}

            <button
              onClick={handleCreateChannel}
              disabled={channelSubmitting || !channelName}
              className="flex items-center gap-2 px-4 py-2 rounded-lg bg-[#3b82f6] hover:bg-blue-600 text-white text-sm font-medium transition-colors disabled:opacity-50"
            >
              {channelSubmitting && <Loader2 className="w-4 h-4 animate-spin" />}
              Create Channel
            </button>
          </div>
        )}

        {channels.length === 0 ? (
          <div className="bg-[#18181b] border border-[#27272a] rounded-xl p-8 text-center text-[#71717a] text-sm">
            No notification channels configured
          </div>
        ) : (
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            {channels.map((ch) => {
              const style = CHANNEL_TYPE_STYLES[ch.type] || CHANNEL_TYPE_STYLES.webhook;
              const Icon = style.icon;
              return (
                <div
                  key={ch.id}
                  className="bg-[#18181b] border border-[#27272a] rounded-xl p-4 flex items-center gap-3"
                >
                  <div className={`w-9 h-9 rounded-lg flex items-center justify-center ${style.bg}`}>
                    <Icon className={`w-4 h-4 ${style.text}`} />
                  </div>
                  <div className="min-w-0">
                    <p className="text-sm text-[#fafafa] font-medium">{ch.name}</p>
                    <p className={`text-xs ${style.text}`}>{ch.type}</p>
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </section>
    </div>
  );
}
