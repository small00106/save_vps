import { AlertTriangle, Loader2 } from "lucide-react";
import { useI18n } from "../i18n/useI18n";

export default function DefaultPasswordNoticeModal({
  acknowledging,
  error,
  onAcknowledge,
}: {
  acknowledging: boolean;
  error: string;
  onAcknowledge: () => Promise<void>;
}) {
  const { tx } = useI18n();

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
      <div className="w-full max-w-md rounded-2xl border border-amber-500/20 bg-card p-6 shadow-lg">
        <div className="mb-4 flex items-start gap-3">
          <div className="mt-0.5 flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-amber-500/10 text-amber-500">
            <AlertTriangle className="h-5 w-5" />
          </div>
          <div>
            <h2 className="text-lg font-semibold text-text-primary">
              {tx("请尽快修改默认密码", "Please change the default password soon")}
            </h2>
            <p className="mt-1 text-sm text-text-secondary">
              {tx(
                "当前管理员账户仍在使用默认密码 admin。这个个人项目保留默认管理员作为初始入口，但建议你尽快前往设置页修改密码。",
                "The admin account is still using the default password `admin`. This personal project keeps the default admin as the initial entry point, but you should change it from Settings soon.",
              )}
            </p>
          </div>
        </div>

        {error && (
          <div className="mb-4 rounded-lg bg-offline/10 px-3 py-2 text-sm text-offline">
            {error}
          </div>
        )}

        <div className="rounded-xl bg-bg px-4 py-3 text-sm text-text-muted">
          {tx(
            "该提醒在整个系统中只会出现一次；关闭后不会再次弹出。",
            "This reminder is shown only once for the whole system and will not appear again after dismissal.",
          )}
        </div>

        <div className="mt-5 flex justify-end">
          <button
            type="button"
            onClick={() => void onAcknowledge()}
            disabled={acknowledging}
            className="inline-flex items-center gap-2 rounded-lg bg-accent px-4 py-2 text-sm font-medium text-white transition hover:bg-accent-hover disabled:opacity-50"
          >
            {acknowledging && <Loader2 size={16} className="animate-spin" />}
            {acknowledging
              ? tx("处理中...", "Working...")
              : tx("我知道了", "I Understand")}
          </button>
        </div>
      </div>
    </div>
  );
}
