import { useState } from "react";
import { ApiError, changePassword } from "../api/client";
import { useAuth } from "../hooks/useAuth";
import { KeyRound, Loader2, ShieldCheck } from "lucide-react";
import { useI18n } from "../i18n/useI18n";
import { usePreferences, type LocaleMode, type ThemeMode } from "../contexts/PreferencesContext";
import { Banner, PageHeader, SectionCard, SelectField } from "../components/ui";

function getErrorMessage(error: unknown, fallback: string): string {
  if (!(error instanceof ApiError)) {
    return fallback;
  }

  try {
    const parsed = JSON.parse(error.message) as { error?: string };
    if (parsed.error) {
      return parsed.error;
    }
  } catch {
    // ignore invalid JSON error payloads
  }

  return error.message || fallback;
}

export default function SettingsPage() {
  const { user } = useAuth();
  const { tx } = useI18n();
  const { localeMode, setLocaleMode, themeMode, setThemeMode } = usePreferences();
  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState("");
  const [success, setSuccess] = useState("");

  const handleSubmit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setError("");
    setSuccess("");

    if (newPassword !== confirmPassword) {
      setError(tx("新密码与确认密码不一致", "New password and confirmation do not match"));
      return;
    }

    setSubmitting(true);
    try {
      await changePassword(currentPassword, newPassword);
      setSuccess(tx("密码更新成功", "Password updated successfully"));
      setCurrentPassword("");
      setNewPassword("");
      setConfirmPassword("");
    } catch (submitError) {
      setError(getErrorMessage(submitError, tx("密码更新失败", "Failed to update password")));
    } finally {
      setSubmitting(false);
    }
  };

  const inputClass =
    "w-full rounded-2xl border border-border bg-surface px-4 py-3 text-sm text-text-primary outline-none";

  return (
    <div className="space-y-6">
      <PageHeader
        eyebrow={tx("账户与偏好", "Account and Preferences")}
        title={tx("设置", "Settings")}
        description={tx(
          `当前登录用户：${user?.username ?? tx("未知", "unknown")}。在这里调整界面偏好，并维护管理员账户安全。`,
          `Signed in as ${user?.username ?? tx("unknown", "unknown")}. Adjust interface preferences and maintain admin account security here.`,
        )}
      />

      <SectionCard
        title={tx("界面偏好", "Interface Preferences")}
        description={tx("语言与主题设置会立即生效，并持久化到本地浏览器。", "Language and theme settings apply immediately and are persisted locally.")}
      >
        <div className="grid gap-4 sm:grid-cols-2">
          <label className="block space-y-2">
            <span className="text-sm font-medium text-text-secondary">{tx("语言", "Language")}</span>
            <SelectField
              value={localeMode}
              onChange={(event) => setLocaleMode(event.target.value as LocaleMode)}
              className={inputClass}
            >
              <option value="system">{tx("跟随系统", "System")}</option>
              <option value="zh">{tx("中文", "Chinese")}</option>
              <option value="en">{tx("英文", "English")}</option>
            </SelectField>
          </label>
          <label className="block space-y-2">
            <span className="text-sm font-medium text-text-secondary">{tx("主题", "Theme")}</span>
            <SelectField
              value={themeMode}
              onChange={(event) => setThemeMode(event.target.value as ThemeMode)}
              className={inputClass}
            >
              <option value="system">{tx("跟随系统", "System")}</option>
              <option value="light">{tx("浅色", "Light")}</option>
              <option value="dark">{tx("深色", "Dark")}</option>
            </SelectField>
          </label>
        </div>
      </SectionCard>

      <SectionCard
        title={tx("修改密码", "Change Password")}
        description={tx("密码修改会立刻影响当前管理员账户。", "Changing the password takes effect immediately for the current admin account.")}
        actions={
          <span className="inline-flex items-center gap-2 rounded-full border border-accent/15 bg-accent-muted px-3 py-1.5 text-xs font-medium text-accent">
            <ShieldCheck size={14} />
            {tx("安全设置", "Security")}
          </span>
        }
      >
        <form onSubmit={handleSubmit} className="space-y-4">
          <Banner tone="warning">
            {user?.default_password_notice_required
              ? tx(
                  "当前仍在使用默认密码 admin。建议尽快在此页面完成修改。",
                  "The account is still using the default password admin. Change it from this page soon.",
                )
              : tx(
                  "个人项目保留默认管理员作为初始入口；如果你还没改过默认密码，请尽快在这里更新。",
                  "This personal project keeps the default admin as the initial entry point. If you have not changed it yet, update it here soon.",
                )}
          </Banner>

          {error ? <Banner tone="danger" role="alert">{error}</Banner> : null}
          {success ? <Banner tone="success" role="status">{success}</Banner> : null}

          <div className="grid gap-4 md:grid-cols-3">
            <label className="block space-y-2">
              <span className="text-sm font-medium text-text-secondary">{tx("当前密码", "Current password")}</span>
              <input
                type="password"
                value={currentPassword}
                onChange={(event) => setCurrentPassword(event.target.value)}
                required
                className={inputClass}
              />
            </label>

            <label className="block space-y-2">
              <span className="text-sm font-medium text-text-secondary">{tx("新密码", "New password")}</span>
              <input
                type="password"
                value={newPassword}
                onChange={(event) => setNewPassword(event.target.value)}
                required
                className={inputClass}
              />
            </label>

            <label className="block space-y-2">
              <span className="text-sm font-medium text-text-secondary">{tx("确认新密码", "Confirm new password")}</span>
              <input
                type="password"
                value={confirmPassword}
                onChange={(event) => setConfirmPassword(event.target.value)}
                required
                className={inputClass}
              />
            </label>
          </div>

          <div className="flex items-center justify-end">
            <button
              type="submit"
              disabled={submitting || !currentPassword || !newPassword || !confirmPassword}
              className="gradient-button inline-flex items-center gap-2 rounded-2xl px-4 py-2.5 text-sm font-medium text-white disabled:opacity-50"
            >
              <KeyRound size={16} />
              {submitting ? <Loader2 size={16} className="animate-spin" /> : null}
              {submitting ? tx("更新中...", "Updating...") : tx("更新密码", "Update Password")}
            </button>
          </div>
        </form>
      </SectionCard>
    </div>
  );
}
