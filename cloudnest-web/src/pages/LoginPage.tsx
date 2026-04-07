import { useState } from "react";
import { useAuth } from "../hooks/useAuth";
import { Lock, User, ArrowRight, Loader2 } from "lucide-react";
import { useI18n } from "../i18n/useI18n";

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
    } catch {
      setError(tx("用户名或密码错误", "Invalid credentials"));
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="relative flex min-h-dvh items-center justify-center p-4 overflow-hidden">
      {/* Animated gradient background blobs */}
      <div className="absolute inset-0 -z-10 overflow-hidden">
        <div 
          className="absolute -top-1/4 -left-1/4 h-[600px] w-[600px] rounded-full opacity-30"
          style={{
            background: "radial-gradient(circle, var(--ui-accent) 0%, transparent 70%)",
            filter: "blur(80px)",
            animation: "float 8s ease-in-out infinite",
          }}
        />
        <div 
          className="absolute -bottom-1/4 -right-1/4 h-[500px] w-[500px] rounded-full opacity-25"
          style={{
            background: "radial-gradient(circle, var(--ui-accent-secondary) 0%, transparent 70%)",
            filter: "blur(80px)",
            animation: "float 10s ease-in-out infinite reverse",
          }}
        />
        <div 
          className="absolute top-1/3 right-1/4 h-[400px] w-[400px] rounded-full opacity-20"
          style={{
            background: "radial-gradient(circle, var(--ui-accent-tertiary) 0%, transparent 70%)",
            filter: "blur(60px)",
            animation: "float 12s ease-in-out infinite",
          }}
        />
      </div>

      <form
        onSubmit={handleSubmit}
        className="relative w-full max-w-md animate-slide-up glass-card rounded-2xl p-8 md:p-10"
      >
        {/* Decorative gradient border effect */}
        <div 
          className="absolute -inset-[1px] rounded-2xl opacity-50 -z-10"
          style={{
            background: "linear-gradient(135deg, var(--ui-accent), var(--ui-accent-secondary), var(--ui-accent-tertiary))",
          }}
        />

        {/* Header */}
        <div className="mb-8 flex flex-col items-center gap-4">
          <div 
            className="flex h-16 w-16 items-center justify-center rounded-2xl"
            style={{
              background: "linear-gradient(135deg, var(--ui-accent), var(--ui-accent-secondary))",
              boxShadow: "0 8px 32px var(--ui-accent-glow)",
            }}
          >
            <Lock className="h-8 w-8 text-white" />
          </div>
          <div className="text-center">
            <h1 className="text-2xl font-bold text-text-primary">
              {tx("欢迎回来", "Welcome back")}
            </h1>
            <p className="mt-1 text-sm text-text-muted">
              {tx("登录后进入控制台", "Sign in to your dashboard")}
            </p>
          </div>
        </div>

        {/* Error message */}
        {notice && (
          <div className="mb-6 rounded-xl border border-accent/20 bg-accent/10 px-4 py-3 text-sm text-text-primary animate-slide-up">
            {notice}
          </div>
        )}
        {error && (
          <div className="mb-6 rounded-xl bg-offline/10 border border-offline/20 px-4 py-3 text-sm text-offline animate-slide-up">
            {error}
          </div>
        )}

        {/* Form fields */}
        <div className="space-y-5">
          <label className="block">
            <span className="mb-2 flex items-center gap-2 text-sm font-medium text-text-secondary">
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
              className="w-full rounded-xl border border-border bg-bg/50 px-4 py-3 text-sm text-text-primary placeholder-text-muted outline-none transition-all duration-200 focus:border-accent focus:bg-bg/80"
            />
          </label>

          <label className="block">
            <span className="mb-2 flex items-center gap-2 text-sm font-medium text-text-secondary">
              <Lock className="h-4 w-4" />
              {tx("密码", "Password")}
            </span>
            <input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
              placeholder={tx("请输入密码", "Enter your password")}
              className="w-full rounded-xl border border-border bg-bg/50 px-4 py-3 text-sm text-text-primary placeholder-text-muted outline-none transition-all duration-200 focus:border-accent focus:bg-bg/80"
            />
          </label>
        </div>

        {/* Submit button */}
        <button
          type="submit"
          disabled={loading}
          className="gradient-button mt-8 flex w-full items-center justify-center gap-2 rounded-xl py-3.5 text-sm font-semibold text-white disabled:opacity-50 disabled:cursor-not-allowed"
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

        <p className="mt-4 text-center text-xs text-text-muted">
          {tx(
            "默认管理员账号为 admin/admin。个人项目会保留这个初始入口，并在首次使用默认密码登录后弹出一次修改提醒。",
            "The default admin account is `admin/admin`. This personal project keeps it as the initial entry point and shows a one-time password-change reminder after the first login with the default password.",
          )}
        </p>
      </form>
    </div>
  );
}
