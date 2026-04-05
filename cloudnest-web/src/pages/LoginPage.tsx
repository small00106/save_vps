import { useState } from "react";
import { useAuth } from "../hooks/useAuth";
import { Lock } from "lucide-react";
import { useI18n } from "../i18n/useI18n";

export default function LoginPage() {
  const { login } = useAuth();
  const { tx } = useI18n();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
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
    <div className="flex h-dvh items-center justify-center bg-bg p-4">
      <form
        onSubmit={handleSubmit}
        className="w-full max-w-sm animate-slide-up rounded-xl border border-border bg-card p-8"
      >
        <div className="mb-6 flex flex-col items-center gap-3">
          <div className="flex h-12 w-12 items-center justify-center rounded-xl bg-accent/15 text-accent">
            <Lock size={24} />
          </div>
          <h1 className="text-xl font-semibold text-text-primary">CloudNest</h1>
          <p className="text-sm text-text-muted">
            {tx("登录后进入控制台", "Sign in to your dashboard")}
          </p>
        </div>
        {error && (
          <div className="mb-4 rounded-lg bg-offline/10 px-3 py-2 text-sm text-offline">
            {error}
          </div>
        )}
        <label className="mb-4 block">
          <span className="mb-1 block text-xs text-text-muted">
            {tx("用户名", "Username")}
          </span>
          <input
            type="text"
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            required
            autoFocus
            className="w-full rounded-lg border border-border bg-bg px-3 py-2 text-sm text-text-primary outline-none transition focus:border-accent focus:ring-1 focus:ring-accent/30"
          />
        </label>
        <label className="mb-6 block">
          <span className="mb-1 block text-xs text-text-muted">
            {tx("密码", "Password")}
          </span>
          <input
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required
            className="w-full rounded-lg border border-border bg-bg px-3 py-2 text-sm text-text-primary outline-none transition focus:border-accent focus:ring-1 focus:ring-accent/30"
          />
        </label>
        <button
          type="submit"
          disabled={loading}
          className="w-full rounded-lg bg-accent py-2 text-sm font-medium text-white transition hover:bg-accent-hover disabled:opacity-50"
        >
          {loading
            ? tx("登录中...", "Signing in...")
            : tx("登录", "Sign in")}
        </button>
      </form>
    </div>
  );
}
