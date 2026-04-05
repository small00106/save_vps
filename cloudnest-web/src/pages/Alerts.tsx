import { useEffect, useState } from "react";
import {
  Loader2, Plus, X, Trash2, Bell, Send, Globe, Mail, Pencil,
} from "lucide-react";
import {
  getAlertRules, createAlertRule, updateAlertRule, deleteAlertRule,
  getAlertChannels, createAlertChannel, updateAlertChannel,
  type AlertRule, type AlertChannel,
} from "../api/client";
import { useI18n } from "../i18n/useI18n";

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
  const { tx } = useI18n();
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
  const [editingChannelId, setEditingChannelId] = useState<number | null>(null);
  const [editChannelName, setEditChannelName] = useState("");
  const [editChannelConfig, setEditChannelConfig] = useState("");
  const [channelUpdating, setChannelUpdating] = useState(false);

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
    } catch (err) {
      void err;
    }
    setRuleSubmitting(false);
  };

  const handleToggleRule = async (rule: AlertRule) => {
    try {
      const updated = await updateAlertRule(rule.id, { enabled: !rule.enabled });
      setRules((prev) => prev.map((r) => (r.id === rule.id ? updated : r)));
    } catch (err) {
      void err;
    }
  };

  const handleDeleteRule = async (id: number) => {
    try {
      await deleteAlertRule(id);
      setRules((prev) => prev.filter((r) => r.id !== id));
    } catch (err) {
      void err;
    }
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
    } catch (err) {
      void err;
    }
    setChannelSubmitting(false);
  };

  const openEditChannel = (channel: AlertChannel) => {
    setEditingChannelId(channel.id);
    setEditChannelName(channel.name);
    try {
      setEditChannelConfig(JSON.stringify(JSON.parse(channel.config), null, 2));
    } catch {
      setEditChannelConfig(channel.config || "{}");
    }
  };

  const handleUpdateChannel = async () => {
    if (!editingChannelId || !editChannelName.trim()) return;
    try {
      JSON.parse(editChannelConfig);
    } catch {
      alert(tx("配置必须是合法 JSON", "Config must be valid JSON"));
      return;
    }
    setChannelUpdating(true);
    try {
      const updated = await updateAlertChannel(editingChannelId, {
        name: editChannelName.trim(),
        config: editChannelConfig,
      });
      setChannels((prev) => prev.map((c) => (c.id === updated.id ? updated : c)));
      setEditingChannelId(null);
      setEditChannelName("");
      setEditChannelConfig("");
    } catch {
      // ignore
    } finally {
      setChannelUpdating(false);
    }
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center h-[60vh]">
        <Loader2 className="h-6 w-6 animate-spin text-accent" />
      </div>
    );
  }

  const inputClass =
    "h-9 w-full rounded-lg border border-border bg-bg px-3 text-sm text-text-primary transition-colors focus:border-accent focus:outline-none";

  return (
    <div className="space-y-8 animate-[fadeIn_0.3s_ease-out]">
      {/* Alert Rules */}
      <section>
        <div className="flex items-center justify-between mb-4">
          <div className="flex items-center gap-2">
            <Bell className="h-5 w-5 text-accent" />
            <h2 className="text-lg font-bold text-text-primary">{tx("告警规则", "Alert Rules")}</h2>
          </div>
          <button
            onClick={() => setShowRuleForm(!showRuleForm)}
            className="flex items-center gap-1.5 rounded-lg bg-accent px-3 py-1.5 text-sm font-medium text-white transition-colors hover:bg-accent-hover"
          >
            {showRuleForm ? <X className="w-4 h-4" /> : <Plus className="w-4 h-4" />}
            {showRuleForm ? tx("取消", "Cancel") : tx("新建规则", "New Rule")}
          </button>
        </div>

        {showRuleForm && (
          <div className="mb-4 space-y-4 rounded-xl border border-border bg-card p-5">
            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
              <div>
                <label className="mb-1 block text-xs text-text-secondary">{tx("名称", "Name")}</label>
                <input value={ruleName} onChange={(e) => setRuleName(e.target.value)} className={inputClass} placeholder={tx("高 CPU 告警", "High CPU Alert")} />
              </div>
              <div>
                <label className="mb-1 block text-xs text-text-secondary">{tx("节点 UUID（可选）", "Node UUID (optional)")}</label>
                <input value={ruleNodeUuid} onChange={(e) => setRuleNodeUuid(e.target.value)} className={inputClass} placeholder={tx("留空表示全部节点", "All nodes if empty")} />
              </div>
              <div>
                <label className="mb-1 block text-xs text-text-secondary">{tx("指标", "Metric")}</label>
                <select value={ruleMetric} onChange={(e) => setRuleMetric(e.target.value)} className={inputClass}>
                  {METRICS.map((m) => (
                    <option key={m} value={m}>{m}</option>
                  ))}
                </select>
              </div>
              <div>
                <label className="mb-1 block text-xs text-text-secondary">{tx("运算符", "Operator")}</label>
                <select value={ruleOperator} onChange={(e) => setRuleOperator(e.target.value)} className={inputClass}>
                  {OPERATORS.map((o) => (
                    <option key={o} value={o}>{o}</option>
                  ))}
                </select>
              </div>
              <div>
                <label className="mb-1 block text-xs text-text-secondary">{tx("阈值", "Threshold")}</label>
                <input type="number" value={ruleThreshold} onChange={(e) => setRuleThreshold(Number(e.target.value))} className={inputClass} />
              </div>
              <div>
                <label className="mb-1 block text-xs text-text-secondary">{tx("持续时间（秒）", "Duration (s)")}</label>
                <input type="number" value={ruleDuration} onChange={(e) => setRuleDuration(Number(e.target.value))} className={inputClass} />
              </div>
              <div>
                <label className="mb-1 block text-xs text-text-secondary">{tx("通知渠道", "Channel")}</label>
                <select value={ruleChannelId} onChange={(e) => setRuleChannelId(Number(e.target.value))} className={inputClass}>
                  <option value={0}>{tx("无", "None")}</option>
                  {channels.map((c) => (
                    <option key={c.id} value={c.id}>{c.name}</option>
                  ))}
                </select>
              </div>
            </div>
            <button
              onClick={handleCreateRule}
              disabled={ruleSubmitting || !ruleName}
              className="flex items-center gap-2 rounded-lg bg-accent px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-accent-hover disabled:opacity-50"
            >
              {ruleSubmitting && <Loader2 className="w-4 h-4 animate-spin" />}
              {tx("创建规则", "Create Rule")}
            </button>
          </div>
        )}

        {rules.length === 0 ? (
          <div className="rounded-xl border border-border bg-card p-8 text-center text-sm text-text-muted">
            {tx("尚未配置告警规则", "No alert rules configured")}
          </div>
        ) : (
          <div className="overflow-hidden rounded-xl border border-border bg-card divide-y divide-border">
            {rules.map((rule) => (
              <div
                key={rule.id}
                className="flex items-center gap-4 px-5 py-3 transition-colors hover:bg-border/50"
              >
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2 mb-0.5">
                    <span className="text-sm font-medium text-text-primary">{rule.name}</span>
                    <span className="rounded bg-accent/10 px-2 py-0.5 text-[10px] font-medium text-accent">
                      {rule.metric}
                    </span>
                  </div>
                  <span className="text-xs text-text-muted">
                    {rule.operator} {rule.threshold} {tx("持续", "for")} {rule.duration}s
                  </span>
                </div>
                <button
                  onClick={() => handleToggleRule(rule)}
                  className={`relative w-9 h-5 rounded-full transition-colors ${
                    rule.enabled ? "bg-accent" : "bg-border"
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
                  className="text-text-muted transition-colors hover:text-offline"
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
            <Send className="h-5 w-5 text-[#a855f7]" />
            <h2 className="text-lg font-bold text-text-primary">
              {tx("通知渠道", "Notification Channels")}
            </h2>
          </div>
          <button
            onClick={() => setShowChannelForm(!showChannelForm)}
            className="flex items-center gap-1.5 rounded-lg bg-accent px-3 py-1.5 text-sm font-medium text-white transition-colors hover:bg-accent-hover"
          >
            {showChannelForm ? <X className="w-4 h-4" /> : <Plus className="w-4 h-4" />}
            {showChannelForm ? tx("取消", "Cancel") : tx("新建渠道", "New Channel")}
          </button>
        </div>

        {showChannelForm && (
          <div className="mb-4 space-y-4 rounded-xl border border-border bg-card p-5">
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
              <div>
                <label className="mb-1 block text-xs text-text-secondary">{tx("名称", "Name")}</label>
                <input value={channelName} onChange={(e) => setChannelName(e.target.value)} className={inputClass} placeholder={tx("运维团队", "Ops Team")} />
              </div>
              <div>
                <label className="mb-1 block text-xs text-text-secondary">{tx("类型", "Type")}</label>
                <select value={channelType} onChange={(e) => setChannelType(e.target.value as typeof channelType)} className={inputClass}>
                  <option value="webhook">{tx("Webhook", "Webhook")}</option>
                  <option value="telegram">{tx("Telegram", "Telegram")}</option>
                  <option value="email">{tx("邮件", "Email")}</option>
                  <option value="bark">{tx("Bark", "Bark")}</option>
                  <option value="serverchan">{tx("ServerChan", "ServerChan")}</option>
                </select>
              </div>
            </div>

            {/* Webhook / Bark */}
            {(channelType === "webhook" || channelType === "bark") && (
              <div>
                <label className="mb-1 block text-xs text-text-secondary">
                  {tx("地址 URL", "URL")}
                </label>
                <input value={channelConfigUrl} onChange={(e) => setChannelConfigUrl(e.target.value)} className={inputClass} placeholder="https://hooks.example.com/..." />
              </div>
            )}

            {/* Telegram */}
            {channelType === "telegram" && (
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                <div>
                  <label className="mb-1 block text-xs text-text-secondary">
                    {tx("Bot Token", "Bot Token")}
                  </label>
                  <input value={channelConfigBotToken} onChange={(e) => setChannelConfigBotToken(e.target.value)} className={inputClass} placeholder="123456:ABC-DEF..." />
                </div>
                <div>
                  <label className="mb-1 block text-xs text-text-secondary">
                    {tx("Chat ID", "Chat ID")}
                  </label>
                  <input value={channelConfigChatId} onChange={(e) => setChannelConfigChatId(e.target.value)} className={inputClass} placeholder="-1001234567890" />
                </div>
              </div>
            )}

            {/* Email */}
            {channelType === "email" && (
              <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
                <div>
                  <label className="mb-1 block text-xs text-text-secondary">
                    {tx("SMTP 主机", "SMTP Host")}
                  </label>
                  <input value={channelConfigSmtpHost} onChange={(e) => setChannelConfigSmtpHost(e.target.value)} className={inputClass} placeholder="smtp.gmail.com" />
                </div>
                <div>
                  <label className="mb-1 block text-xs text-text-secondary">
                    {tx("SMTP 端口", "SMTP Port")}
                  </label>
                  <input value={channelConfigSmtpPort} onChange={(e) => setChannelConfigSmtpPort(e.target.value)} className={inputClass} placeholder="587" />
                </div>
                <div>
                  <label className="mb-1 block text-xs text-text-secondary">{tx("用户名", "Username")}</label>
                  <input value={channelConfigSmtpUser} onChange={(e) => setChannelConfigSmtpUser(e.target.value)} className={inputClass} />
                </div>
                <div>
                  <label className="mb-1 block text-xs text-text-secondary">{tx("密码", "Password")}</label>
                  <input type="password" value={channelConfigSmtpPass} onChange={(e) => setChannelConfigSmtpPass(e.target.value)} className={inputClass} />
                </div>
                <div>
                  <label className="mb-1 block text-xs text-text-secondary">{tx("发件人", "From")}</label>
                  <input value={channelConfigFrom} onChange={(e) => setChannelConfigFrom(e.target.value)} className={inputClass} placeholder="alerts@example.com" />
                </div>
                <div>
                  <label className="mb-1 block text-xs text-text-secondary">{tx("收件人", "To")}</label>
                  <input value={channelConfigTo} onChange={(e) => setChannelConfigTo(e.target.value)} className={inputClass} placeholder="admin@example.com" />
                </div>
              </div>
            )}

            {/* ServerChan */}
            {channelType === "serverchan" && (
              <div>
                <label className="mb-1 block text-xs text-text-secondary">
                  {tx("SendKey", "SendKey")}
                </label>
                <input value={channelConfigSendKey} onChange={(e) => setChannelConfigSendKey(e.target.value)} className={inputClass} placeholder="SCT1234567890" />
              </div>
            )}

            <button
              onClick={handleCreateChannel}
              disabled={channelSubmitting || !channelName}
              className="flex items-center gap-2 rounded-lg bg-accent px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-accent-hover disabled:opacity-50"
            >
              {channelSubmitting && <Loader2 className="w-4 h-4 animate-spin" />}
              {tx("创建渠道", "Create Channel")}
            </button>
          </div>
        )}

        {editingChannelId && (
          <div className="mb-4 space-y-4 rounded-xl border border-border bg-card p-5">
            <div className="flex items-center justify-between">
              <h3 className="text-sm font-semibold text-text-primary">{tx("编辑渠道", "Edit Channel")}</h3>
              <button
                onClick={() => setEditingChannelId(null)}
                className="rounded p-1 text-text-muted transition-colors hover:text-text-primary"
              >
                <X className="w-4 h-4" />
              </button>
            </div>
            <div>
              <label className="mb-1 block text-xs text-text-secondary">{tx("名称", "Name")}</label>
              <input
                value={editChannelName}
                onChange={(e) => setEditChannelName(e.target.value)}
                className={inputClass}
              />
            </div>
            <div>
              <label className="mb-1 block text-xs text-text-secondary">
                {tx("配置（JSON）", "Config (JSON)")}
              </label>
              <textarea
                value={editChannelConfig}
                onChange={(e) => setEditChannelConfig(e.target.value)}
                className="min-h-32 w-full rounded-lg border border-border bg-bg px-3 py-2 font-mono text-xs text-text-primary transition-colors focus:border-accent focus:outline-none"
              />
            </div>
            <button
              onClick={handleUpdateChannel}
              disabled={channelUpdating || !editChannelName.trim()}
              className="flex items-center gap-2 rounded-lg bg-accent px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-accent-hover disabled:opacity-50"
            >
              {channelUpdating && <Loader2 className="w-4 h-4 animate-spin" />}
              {tx("保存渠道", "Save Channel")}
            </button>
          </div>
        )}

        {channels.length === 0 ? (
          <div className="rounded-xl border border-border bg-card p-8 text-center text-sm text-text-muted">
            {tx("尚未配置通知渠道", "No notification channels configured")}
          </div>
        ) : (
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            {channels.map((ch) => {
              const style = CHANNEL_TYPE_STYLES[ch.type] || CHANNEL_TYPE_STYLES.webhook;
              const Icon = style.icon;
              return (
                <div
                  key={ch.id}
                  className="flex items-center gap-3 rounded-xl border border-border bg-card p-4"
                >
                  <div className={`w-9 h-9 rounded-lg flex items-center justify-center ${style.bg}`}>
                    <Icon className={`w-4 h-4 ${style.text}`} />
                  </div>
                  <div className="min-w-0">
                    <p className="text-sm font-medium text-text-primary">{ch.name}</p>
                    <p className={`text-xs ${style.text}`}>{ch.type}</p>
                  </div>
                  <button
                    onClick={() => openEditChannel(ch)}
                    className="ml-auto rounded-md p-1.5 text-text-muted transition-colors hover:bg-accent/10 hover:text-accent"
                    title={tx("编辑渠道", "Edit channel")}
                  >
                    <Pencil className="w-4 h-4" />
                  </button>
                </div>
              );
            })}
          </div>
        )}
      </section>
    </div>
  );
}
