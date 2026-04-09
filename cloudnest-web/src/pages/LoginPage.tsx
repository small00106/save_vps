import { useState } from "react";
import { useAuth } from "../hooks/useAuth";
import { ArrowRight, FolderTree, Loader2, Lock, ShieldCheck, User, Waves } from "lucide-react";
import { useI18n } from "../i18n/useI18n";
import { Banner } from "../components/ui";
import { ApiError } from "../api/client";

export default function LoginPage() {
  const { login, notice, clearNotice } = useAuth();
  const { tx } = useI18n();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    clearNotice();
    setError("");
    setLoading(true);
    try {
      await login(username, password);
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        setError(tx("用户名或密码错误", "Invalid credentials"));
      } else {
        setError(
          tx(
            "无法连接到后端服务，请确认 Master 已启动。",
            "Unable to reach the backend service. Confirm the master is running.",
          ),
        );
      }
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="flex min-h-dvh items-center justify-center px-4 py-8">
      <div className="grid w-full max-w-6xl gap-6 lg:grid-cols-[1.1fr_0.9fr]">
        <section className="surface-card hidden rounded-[32px] px-8 py-10 lg:flex lg:flex-col lg:justify-between">
          <div className="space-y-8">
            <div className="space-y-4">
              <span className="inline-flex items-center rounded-full border border-accent/15 bg-accent-muted px-3 py-1 text-xs font-semibold uppercase tracking-[0.24em] text-accent">
                CloudNest
              </span>
              <div className="space-y-3">
                <h1 className="max-w-xl text-4xl font-semibold tracking-tight text-text-primary">
                  {tx("统一掌握节点、文件与告警状态。", "Operate nodes, files, and alerts from one console.")}
                </h1>
                <p className="max-w-xl text-base leading-7 text-text-secondary">
                  {tx(
                    "为个人运维场景整理成更克制、更稳定的控制台界面。登录后即可查看节点健康、文件代理与告警治理。",
                    "A restrained, stable console for personal operations. Sign in to review node health, storage proxying, and alert governance.",
                  )}
                </p>
              </div>
            </div>

            <div className="grid gap-4">
              {[
                {
                  icon: Waves,
                  title: tx("实时状态", "Real-time status"),
                  description: tx("连接情况、节点健康和流量信息集中呈现。", "Connectivity, health, and traffic are presented in one place."),
                },
                {
                  icon: FolderTree,
                  title: tx("文件代理", "File proxy"),
                  description: tx("统一浏览和下载托管文件，不暴露 Agent 数据面。", "Browse and download managed files without exposing the agent data plane."),
                },
                {
                  icon: ShieldCheck,
                  title: tx("治理闭环", "Governance loop"),
                  description: tx("规则、通知与审计日志维持同一套管理语境。", "Rules, notifications, and audit logs stay in the same management context."),
                },
              ].map(({ icon: Icon, title, description }) => (
                <div key={title} className="rounded-3xl border border-border bg-surface px-5 py-4">
                  <div className="flex items-start gap-4">
                    <span className="flex h-11 w-11 items-center justify-center rounded-2xl bg-accent-muted text-accent">
                      <Icon className="h-5 w-5" />
                    </span>
                    <div className="space-y-1">
                      <h2 className="text-base font-semibold text-text-primary">{title}</h2>
                      <p className="text-sm leading-6 text-text-secondary">{description}</p>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          </div>

          <div className="rounded-3xl border border-dashed border-border bg-surface px-5 py-4 text-sm leading-6 text-text-secondary">
            {tx(
              "默认管理员账号仍为 admin/admin。首次使用默认密码登录后，系统会保留一次性提醒，避免忽略安全收口。",
              "The default admin account remains admin/admin. A one-time reminder is kept after first use so the security cleanup is not missed.",
            )}
          </div>
        </section>

        <section className="surface-card rounded-[32px] px-6 py-8 md:px-8 md:py-10">
          <form onSubmit={handleSubmit} className="mx-auto w-full max-w-md">
            <div className="mb-8 space-y-4">
              <div className="flex h-14 w-14 items-center justify-center rounded-2xl bg-accent text-white shadow-[0_14px_26px_var(--ui-accent-glow)]">
                <Lock className="h-6 w-6" />
              </div>
              <div className="space-y-2">
                <h2 className="text-2xl font-semibold tracking-tight text-text-primary">{tx("登录控制台", "Sign in to CloudNest")}</h2>
                <p className="text-sm leading-6 text-text-secondary">{tx("输入管理员账户后进入控制台。", "Enter the admin account to access the console.")}</p>
              </div>
            </div>

            <div className="space-y-4">
              {notice ? <Banner tone="primary" role="status">{notice}</Banner> : null}
              {error ? <Banner tone="danger" role="alert">{error}</Banner> : null}

              <label className="block space-y-2">
                <span className="flex items-center gap-2 text-sm font-medium text-text-secondary">
                  <User className="h-4 w-4" />
                  {tx("用户名", "Username")}
                </span>
                <input
                  type="text"
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                  required
                  autoFocus
                  placeholder={tx("请输入用户名", "Enter your username")}
                  className="w-full rounded-2xl border border-border bg-surface px-4 py-3 text-sm text-text-primary placeholder:text-text-muted"
                />
              </label>

              <label className="block space-y-2">
                <span className="flex items-center gap-2 text-sm font-medium text-text-secondary">
                  <Lock className="h-4 w-4" />
                  {tx("密码", "Password")}
                </span>
                <input
                  type="password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  required
                  placeholder={tx("请输入密码", "Enter your password")}
                  className="w-full rounded-2xl border border-border bg-surface px-4 py-3 text-sm text-text-primary placeholder:text-text-muted"
                />
              </label>
            </div>

            <button
              type="submit"
              disabled={loading}
              className="gradient-button mt-8 flex w-full items-center justify-center gap-2 rounded-2xl py-3.5 text-sm font-semibold text-white disabled:cursor-not-allowed disabled:opacity-50"
            >
              {loading ? (
                <>
                  <Loader2 className="h-4 w-4 animate-spin" />
                  {tx("登录中...", "Signing in...")}
                </>
              ) : (
                <>
                  {tx("登录", "Sign in")}
                  <ArrowRight className="h-4 w-4" />
                </>
              )}
            </button>

            <p className="mt-4 rounded-2xl border border-border bg-surface px-4 py-3 text-xs leading-6 text-text-muted">
              {tx(
                "默认管理员账号为 admin/admin。个人项目会保留这个初始入口，并在首次使用默认密码登录后弹出一次修改提醒。",
                "The default admin account is admin/admin. This personal project keeps it as the initial entry point and shows a one-time password-change reminder after the first login with the default password.",
              )}
            </p>
          </form>
        </section>
      </div>
    </div>
  );
}
