import { useState } from "react";
import { ApiError, changePassword } from "../api/client";
import { useAuth } from "../hooks/useAuth";
import { KeyRound, Loader2, ShieldCheck } from "lucide-react";
import { useI18n } from "../i18n/useI18n";
import { usePreferences, type LocaleMode, type ThemeMode } from "../contexts/PreferencesContext";

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
    "w-full rounded-lg border border-border bg-bg px-3 py-2 text-sm text-text-primary outline-none transition focus:border-accent focus:ring-1 focus:ring-accent/30";

  return (
    <div className="space-y-6 animate-[fadeIn_0.3s_ease-out]">
      <div className="rounded-xl border border-border bg-card p-5">
        <div className="flex items-center gap-3">
          <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-accent/15 text-accent">
            <ShieldCheck size={20} />
          </div>
          <div>
            <h1 className="text-xl font-semibold text-text-primary">{tx("设置", "Settings")}</h1>
            <p className="text-sm text-text-muted">
              {tx("当前登录用户：", "Signed in as ")}
              {user?.username ?? tx("未知", "unknown")}
            </p>
          </div>
        </div>
      </div>

      <div className="max-w-2xl rounded-xl border border-border bg-card p-5">
        <h2 className="mb-4 text-base font-semibold text-text-primary">
          {tx("界面偏好", "Interface Preferences")}
        </h2>
        <div className="grid gap-4 sm:grid-cols-2">
          <label className="block">
            <span className="mb-1 block text-xs text-text-muted">{tx("语言", "Language")}</span>
            <select
              value={localeMode}
              onChange={(event) => setLocaleMode(event.target.value as LocaleMode)}
              className={inputClass}
            >
              <option value="system">{tx("跟随系统", "System")}</option>
              <option value="zh">{tx("中文", "Chinese")}</option>
              <option value="en">{tx("英文", "English")}</option>
            </select>
          </label>
          <label className="block">
            <span className="mb-1 block text-xs text-text-muted">{tx("主题", "Theme")}</span>
            <select
              value={themeMode}
              onChange={(event) => setThemeMode(event.target.value as ThemeMode)}
              className={inputClass}
            >
              <option value="system">{tx("跟随系统", "System")}</option>
              <option value="light">{tx("浅色", "Light")}</option>
              <option value="dark">{tx("深色", "Dark")}</option>
            </select>
          </label>
        </div>
      </div>

      <form
        onSubmit={handleSubmit}
        className="max-w-2xl rounded-xl border border-border bg-card p-5"
      >
        <div className="mb-5 flex items-center gap-3">
          <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-accent/10 text-accent">
            <KeyRound size={18} />
          </div>
          <div>
            <h2 className="text-base font-semibold text-text-primary">
              {tx("修改密码", "Change Password")}
            </h2>
            <p className="text-sm text-text-muted">
              {tx(
                "在此页面更新当前账户密码。",
                "Update your current account password from this page.",
              )}
            </p>
          </div>
        </div>

        {error && (
          <div className="mb-4 rounded-lg bg-offline/10 px-3 py-2 text-sm text-offline">
            {error}
          </div>
        )}
        {success && (
          <div className="mb-4 rounded-lg bg-online/10 px-3 py-2 text-sm text-online">
            {success}
          </div>
        )}

        <div className="space-y-4">
          <label className="block">
            <span className="mb-1 block text-xs text-text-muted">
              {tx("当前密码", "Current password")}
            </span>
            <input
              type="password"
              value={currentPassword}
              onChange={(event) => setCurrentPassword(event.target.value)}
              required
              className={inputClass}
            />
          </label>

          <label className="block">
            <span className="mb-1 block text-xs text-text-muted">
              {tx("新密码", "New password")}
            </span>
            <input
              type="password"
              value={newPassword}
              onChange={(event) => setNewPassword(event.target.value)}
              required
              className={inputClass}
            />
          </label>

          <label className="block">
            <span className="mb-1 block text-xs text-text-muted">
              {tx("确认新密码", "Confirm new password")}
            </span>
            <input
              type="password"
              value={confirmPassword}
              onChange={(event) => setConfirmPassword(event.target.value)}
              required
              className={inputClass}
            />
          </label>
        </div>

        <div className="mt-5 flex items-center justify-end">
          <button
            type="submit"
            disabled={
              submitting ||
              !currentPassword ||
              !newPassword ||
              !confirmPassword
            }
            className="inline-flex items-center gap-2 rounded-lg bg-accent px-4 py-2 text-sm font-medium text-white transition hover:bg-accent-hover disabled:opacity-50"
          >
            {submitting && <Loader2 size={16} className="animate-spin" />}
            {submitting
              ? tx("更新中...", "Updating...")
              : tx("更新密码", "Update Password")}
          </button>
        </div>
      </form>
    </div>
  );
}
