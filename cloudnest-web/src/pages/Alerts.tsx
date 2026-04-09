import { useEffect, useState } from "react";
import {
  Bell,
  Globe,
  Loader2,
  Mail,
  Pencil,
  Plus,
  Send,
  Trash2,
  X,
} from "lucide-react";
import {
  createAlertChannel,
  createAlertRule,
  deleteAlertRule,
  getAlertChannels,
  getAlertRules,
  updateAlertChannel,
  updateAlertRule,
  type AlertChannel,
  type AlertRule,
} from "../api/client";
import { useI18n } from "../i18n/useI18n";
import { EmptyState, PageHeader, SectionCard, SelectField, StatusBadge } from "../components/ui";

const CHANNEL_TYPE_STYLES: Record<string, { tone: string; icon: typeof Send }> = {
  telegram: { tone: "text-sky-600 dark:text-sky-300", icon: Send },
  webhook: { tone: "text-slate-600 dark:text-slate-300", icon: Globe },
  email: { tone: "text-amber-600 dark:text-amber-300", icon: Mail },
  bark: { tone: "text-emerald-600 dark:text-emerald-300", icon: Bell },
  serverchan: { tone: "text-violet-600 dark:text-violet-300", icon: Send },
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

  const [ruleName, setRuleName] = useState("");
  const [ruleNodeUuid, setRuleNodeUuid] = useState("");
  const [ruleMetric, setRuleMetric] = useState("cpu");
  const [ruleOperator, setRuleOperator] = useState("gt");
  const [ruleThreshold, setRuleThreshold] = useState(80);
  const [ruleDuration, setRuleDuration] = useState(60);
  const [ruleChannelId, setRuleChannelId] = useState(0);
  const [ruleSubmitting, setRuleSubmitting] = useState(false);

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
      .then(([ruleData, channelData]) => {
        setRules(ruleData);
        setChannels(channelData);
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
    } finally {
      setRuleSubmitting(false);
    }
  };

  const handleToggleRule = async (rule: AlertRule) => {
    try {
      const updated = await updateAlertRule(rule.id, { enabled: !rule.enabled });
      setRules((prev) => prev.map((item) => (item.id === rule.id ? updated : item)));
    } catch {
      // ignore
    }
  };

  const handleDeleteRule = async (id: number) => {
    try {
      await deleteAlertRule(id);
      setRules((prev) => prev.filter((rule) => rule.id !== id));
    } catch {
      // ignore
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
          smtp_host: channelConfigSmtpHost,
          smtp_port: channelConfigSmtpPort,
          username: channelConfigSmtpUser,
          password: channelConfigSmtpPass,
          from: channelConfigFrom,
          to: channelConfigTo,
        };
        break;
      case "serverchan":
        configObj = { send_key: channelConfigSendKey };
        break;
    }

    try {
      const channel = await createAlertChannel({
        name: channelName,
        type: channelType,
        config: JSON.stringify(configObj),
      });
      setChannels((prev) => [...prev, channel]);
      setShowChannelForm(false);
      setChannelName("");
      setChannelConfigUrl("");
      setChannelConfigBotToken("");
      setChannelConfigChatId("");
    } finally {
      setChannelSubmitting(false);
    }
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
      setChannels((prev) => prev.map((channel) => (channel.id === updated.id ? updated : channel)));
      setEditingChannelId(null);
      setEditChannelName("");
      setEditChannelConfig("");
    } finally {
      setChannelUpdating(false);
    }
  };

  const inputClass = "w-full rounded-2xl border border-border bg-surface px-4 py-3 text-sm text-text-primary outline-none";

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
        eyebrow={tx("告警治理", "Alert Governance")}
        title={tx("规则与通知渠道", "Rules and Notification Channels")}
        description={tx("把阈值、持续时间和通知通道放在同一工作区，减少重复配置和漏配。", "Keep thresholds, durations, and channels in one workspace to reduce repeated setup and omissions.")}
      />

      <SectionCard
        title={tx("告警规则", "Alert Rules")}
        description={tx("规则按指标、条件、持续时间和通知渠道组合定义。", "Rules are defined by metric, condition, duration, and notification channel.")}
        actions={
          <button type="button" onClick={() => setShowRuleForm((value) => !value)} className="gradient-button inline-flex items-center gap-2 rounded-2xl px-4 py-2.5 text-sm font-medium text-white">
            {showRuleForm ? <X className="h-4 w-4" /> : <Plus className="h-4 w-4" />}
            {showRuleForm ? tx("取消", "Cancel") : tx("新建规则", "New Rule")}
          </button>
        }
      >
        {showRuleForm ? (
          <div className="mb-4 grid gap-4 md:grid-cols-2 xl:grid-cols-3">
            <label className="space-y-2">
              <span className="text-sm font-medium text-text-secondary">{tx("名称", "Name")}</span>
              <input value={ruleName} onChange={(e) => setRuleName(e.target.value)} className={inputClass} placeholder={tx("高 CPU 告警", "High CPU Alert")} />
            </label>
            <label className="space-y-2">
              <span className="text-sm font-medium text-text-secondary">{tx("节点 UUID（可选）", "Node UUID (optional)")}</span>
              <input value={ruleNodeUuid} onChange={(e) => setRuleNodeUuid(e.target.value)} className={inputClass} placeholder="node-uuid" />
            </label>
            <label className="space-y-2">
              <span className="text-sm font-medium text-text-secondary">{tx("指标", "Metric")}</span>
              <SelectField value={ruleMetric} onChange={(e) => setRuleMetric(e.target.value)} className={inputClass}>
                {METRICS.map((metric) => <option key={metric} value={metric}>{metric}</option>)}
              </SelectField>
            </label>
            <label className="space-y-2">
              <span className="text-sm font-medium text-text-secondary">{tx("条件", "Operator")}</span>
              <SelectField value={ruleOperator} onChange={(e) => setRuleOperator(e.target.value)} className={inputClass}>
                {OPERATORS.map((operator) => <option key={operator} value={operator}>{operator}</option>)}
              </SelectField>
            </label>
            <label className="space-y-2">
              <span className="text-sm font-medium text-text-secondary">{tx("阈值", "Threshold")}</span>
              <input type="number" value={ruleThreshold} onChange={(e) => setRuleThreshold(Number(e.target.value))} className={inputClass} />
            </label>
            <label className="space-y-2">
              <span className="text-sm font-medium text-text-secondary">{tx("持续秒数", "Duration (seconds)")}</span>
              <input type="number" value={ruleDuration} onChange={(e) => setRuleDuration(Number(e.target.value))} className={inputClass} />
            </label>
            <label className="space-y-2 md:col-span-2 xl:col-span-1">
              <span className="text-sm font-medium text-text-secondary">{tx("通知渠道", "Notification Channel")}</span>
              <SelectField value={ruleChannelId} onChange={(e) => setRuleChannelId(Number(e.target.value))} className={inputClass}>
                <option value={0}>{tx("未选择", "Not selected")}</option>
                {channels.map((channel) => <option key={channel.id} value={channel.id}>{channel.name}</option>)}
              </SelectField>
            </label>
            <div className="flex items-end justify-end md:col-span-2 xl:col-span-3">
              <button type="button" onClick={handleCreateRule} disabled={ruleSubmitting || !ruleName} className="gradient-button inline-flex items-center gap-2 rounded-2xl px-4 py-2.5 text-sm font-medium text-white disabled:opacity-50">
                {ruleSubmitting ? <Loader2 className="h-4 w-4 animate-spin" /> : null}
                {tx("创建规则", "Create Rule")}
              </button>
            </div>
          </div>
        ) : null}

        {rules.length === 0 ? (
          <EmptyState icon={Bell} title={tx("尚未配置告警规则", "No alert rules configured")} description={tx("规则创建后会按持续时间窗口评估，并通过已绑定的渠道发送通知。", "Once created, rules are evaluated by duration window and notify through the selected channel.")} />
        ) : (
          <div className="space-y-3">
            {rules.map((rule) => (
              <div key={rule.id} className="flex flex-col gap-4 rounded-3xl border border-border bg-surface px-4 py-4 md:flex-row md:items-center md:justify-between md:px-5">
                <div className="min-w-0 flex-1 space-y-2">
                  <div className="flex flex-wrap items-center gap-3">
                    <span className="truncate text-sm font-semibold text-text-primary">{rule.name}</span>
                    <StatusBadge tone={rule.enabled ? "success" : "danger"} label={rule.enabled ? tx("启用中", "Enabled") : tx("已禁用", "Disabled")} />
                    <span className="rounded-full border border-border bg-card px-3 py-1 text-xs text-text-secondary">{rule.metric}</span>
                  </div>
                  <p className="text-xs text-text-muted">
                    {rule.operator} {rule.threshold} · {tx("持续", "for")} {rule.duration}s · channel #{rule.channel_id || 0}
                  </p>
                </div>
                <div className="flex items-center gap-2">
                  <button type="button" onClick={() => void handleToggleRule(rule)} className={`rounded-full px-3 py-2 text-xs font-medium ${rule.enabled ? "bg-accent-muted text-accent" : "bg-card text-text-secondary"}`}>
                    {rule.enabled ? tx("停用", "Disable") : tx("启用", "Enable")}
                  </button>
                  <button type="button" onClick={() => void handleDeleteRule(rule.id)} className="rounded-2xl border border-border bg-card p-2 text-text-muted transition-colors hover:border-offline/20 hover:bg-offline/10 hover:text-offline">
                    <Trash2 className="h-4 w-4" />
                  </button>
                </div>
              </div>
            ))}
          </div>
        )}
      </SectionCard>

      <SectionCard
        title={tx("通知渠道", "Notification Channels")}
        description={tx("支持 Webhook、Telegram、Email、Bark 与 ServerChan。", "Support Webhook, Telegram, Email, Bark, and ServerChan.")}
        actions={
          <button type="button" onClick={() => setShowChannelForm((value) => !value)} className="gradient-button inline-flex items-center gap-2 rounded-2xl px-4 py-2.5 text-sm font-medium text-white">
            {showChannelForm ? <X className="h-4 w-4" /> : <Plus className="h-4 w-4" />}
            {showChannelForm ? tx("取消", "Cancel") : tx("新建渠道", "New Channel")}
          </button>
        }
      >
        {showChannelForm ? (
          <div className="mb-4 space-y-4 rounded-3xl border border-border bg-surface px-4 py-4 md:px-5">
            <div className="grid gap-4 md:grid-cols-2">
              <label className="space-y-2">
                <span className="text-sm font-medium text-text-secondary">{tx("名称", "Name")}</span>
                <input value={channelName} onChange={(e) => setChannelName(e.target.value)} className={inputClass} placeholder={tx("运维团队", "Ops Team")} />
              </label>
              <label className="space-y-2">
                <span className="text-sm font-medium text-text-secondary">{tx("类型", "Type")}</span>
                <SelectField value={channelType} onChange={(e) => setChannelType(e.target.value as typeof channelType)} className={inputClass}>
                  <option value="webhook">Webhook</option>
                  <option value="telegram">Telegram</option>
                  <option value="email">{tx("邮件", "Email")}</option>
                  <option value="bark">Bark</option>
                  <option value="serverchan">ServerChan</option>
                </SelectField>
              </label>
            </div>

            {(channelType === "webhook" || channelType === "bark") ? (
              <label className="space-y-2">
                <span className="text-sm font-medium text-text-secondary">URL</span>
                <input value={channelConfigUrl} onChange={(e) => setChannelConfigUrl(e.target.value)} className={inputClass} placeholder="https://hooks.example.com/..." />
              </label>
            ) : null}

            {channelType === "telegram" ? (
              <div className="grid gap-4 md:grid-cols-2">
                <label className="space-y-2">
                  <span className="text-sm font-medium text-text-secondary">Bot Token</span>
                  <input value={channelConfigBotToken} onChange={(e) => setChannelConfigBotToken(e.target.value)} className={inputClass} placeholder="123456:ABC-DEF..." />
                </label>
                <label className="space-y-2">
                  <span className="text-sm font-medium text-text-secondary">Chat ID</span>
                  <input value={channelConfigChatId} onChange={(e) => setChannelConfigChatId(e.target.value)} className={inputClass} placeholder="-1001234567890" />
                </label>
              </div>
            ) : null}

            {channelType === "email" ? (
              <div className="grid gap-4 md:grid-cols-3">
                <label className="space-y-2"><span className="text-sm font-medium text-text-secondary">SMTP Host</span><input value={channelConfigSmtpHost} onChange={(e) => setChannelConfigSmtpHost(e.target.value)} className={inputClass} placeholder="smtp.gmail.com" /></label>
                <label className="space-y-2"><span className="text-sm font-medium text-text-secondary">SMTP Port</span><input value={channelConfigSmtpPort} onChange={(e) => setChannelConfigSmtpPort(e.target.value)} className={inputClass} placeholder="587" /></label>
                <label className="space-y-2"><span className="text-sm font-medium text-text-secondary">{tx("用户名", "Username")}</span><input value={channelConfigSmtpUser} onChange={(e) => setChannelConfigSmtpUser(e.target.value)} className={inputClass} /></label>
                <label className="space-y-2"><span className="text-sm font-medium text-text-secondary">{tx("密码", "Password")}</span><input type="password" value={channelConfigSmtpPass} onChange={(e) => setChannelConfigSmtpPass(e.target.value)} className={inputClass} /></label>
                <label className="space-y-2"><span className="text-sm font-medium text-text-secondary">{tx("发件人", "From")}</span><input value={channelConfigFrom} onChange={(e) => setChannelConfigFrom(e.target.value)} className={inputClass} placeholder="alerts@example.com" /></label>
                <label className="space-y-2"><span className="text-sm font-medium text-text-secondary">{tx("收件人", "To")}</span><input value={channelConfigTo} onChange={(e) => setChannelConfigTo(e.target.value)} className={inputClass} placeholder="admin@example.com" /></label>
              </div>
            ) : null}

            {channelType === "serverchan" ? (
              <label className="space-y-2">
                <span className="text-sm font-medium text-text-secondary">SendKey</span>
                <input value={channelConfigSendKey} onChange={(e) => setChannelConfigSendKey(e.target.value)} className={inputClass} placeholder="SCT1234567890" />
              </label>
            ) : null}

            <div className="flex justify-end">
              <button type="button" onClick={handleCreateChannel} disabled={channelSubmitting || !channelName} className="gradient-button inline-flex items-center gap-2 rounded-2xl px-4 py-2.5 text-sm font-medium text-white disabled:opacity-50">
                {channelSubmitting ? <Loader2 className="h-4 w-4 animate-spin" /> : null}
                {tx("创建渠道", "Create Channel")}
              </button>
            </div>
          </div>
        ) : null}

        {editingChannelId ? (
          <div className="mb-4 space-y-4 rounded-3xl border border-border bg-surface px-4 py-4 md:px-5">
            <div className="flex items-center justify-between">
              <h3 className="text-sm font-semibold text-text-primary">{tx("编辑渠道", "Edit Channel")}</h3>
              <button type="button" onClick={() => setEditingChannelId(null)} className="rounded-2xl border border-border bg-card p-2 text-text-muted transition-colors hover:text-text-primary">
                <X className="h-4 w-4" />
              </button>
            </div>
            <label className="space-y-2">
              <span className="text-sm font-medium text-text-secondary">{tx("名称", "Name")}</span>
              <input value={editChannelName} onChange={(e) => setEditChannelName(e.target.value)} className={inputClass} />
            </label>
            <label className="space-y-2">
              <span className="text-sm font-medium text-text-secondary">{tx("配置（JSON）", "Config (JSON)")}</span>
              <textarea value={editChannelConfig} onChange={(e) => setEditChannelConfig(e.target.value)} className="min-h-36 w-full rounded-2xl border border-border bg-card px-4 py-3 font-mono text-xs text-text-primary outline-none" />
            </label>
            <div className="flex justify-end">
              <button type="button" onClick={handleUpdateChannel} disabled={channelUpdating || !editChannelName.trim()} className="gradient-button inline-flex items-center gap-2 rounded-2xl px-4 py-2.5 text-sm font-medium text-white disabled:opacity-50">
                {channelUpdating ? <Loader2 className="h-4 w-4 animate-spin" /> : null}
                {tx("保存渠道", "Save Channel")}
              </button>
            </div>
          </div>
        ) : null}

        {channels.length === 0 ? (
          <EmptyState icon={Send} title={tx("尚未配置通知渠道", "No notification channels configured")} description={tx("先创建渠道，再把规则绑定到具体的通知出口。", "Create a channel first, then bind rules to a concrete notification destination.")} />
        ) : (
          <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
            {channels.map((channel) => {
              const style = CHANNEL_TYPE_STYLES[channel.type] || CHANNEL_TYPE_STYLES.webhook;
              const Icon = style.icon;
              return (
                <div key={channel.id} className="flex items-center gap-3 rounded-3xl border border-border bg-surface px-4 py-4">
                  <span className={`flex h-11 w-11 items-center justify-center rounded-2xl bg-card ${style.tone}`}>
                    <Icon className="h-4 w-4" />
                  </span>
                  <div className="min-w-0 flex-1">
                    <p className="truncate text-sm font-semibold text-text-primary">{channel.name}</p>
                    <p className="text-xs text-text-muted">{channel.type}</p>
                  </div>
                  <button type="button" onClick={() => openEditChannel(channel)} className="rounded-2xl border border-border bg-card p-2 text-text-muted transition-colors hover:border-accent/20 hover:bg-accent-muted hover:text-accent">
                    <Pencil className="h-4 w-4" />
                  </button>
                </div>
              );
            })}
          </div>
        )}
      </SectionCard>
    </div>
  );
}
