import { AlertTriangle, Loader2 } from "lucide-react";
import { useI18n } from "../i18n/useI18n";
import { Banner } from "./ui";

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
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/35 p-4 backdrop-blur-sm">
      <div className="w-full max-w-md rounded-[28px] border border-border bg-card p-6 shadow-panel">
        <div className="mb-4 flex items-start gap-3">
          <div className="mt-0.5 flex h-10 w-10 shrink-0 items-center justify-center rounded-2xl bg-warning/10 text-warning">
            <AlertTriangle className="h-5 w-5" />
          </div>
          <div>
            <h2 className="text-lg font-semibold tracking-tight text-text-primary">
              {tx("请尽快修改默认密码", "Please change the default password soon")}
            </h2>
            <p className="mt-1 text-sm leading-6 text-text-secondary">
              {tx(
                "当前管理员账户仍在使用默认密码 admin。这个个人项目保留默认管理员作为初始入口，但建议你尽快前往设置页修改密码。",
                "The admin account is still using the default password admin. This personal project keeps the default admin as the initial entry point, but you should change it from Settings soon.",
              )}
            </p>
          </div>
        </div>

        {error ? <Banner tone="danger" role="alert" className="mb-4">{error}</Banner> : null}

        <div className="rounded-2xl border border-border bg-surface px-4 py-3 text-sm leading-6 text-text-secondary">
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
            className="gradient-button inline-flex items-center gap-2 rounded-2xl px-4 py-2.5 text-sm font-medium text-white disabled:opacity-50"
          >
            {acknowledging ? <Loader2 size={16} className="animate-spin" /> : null}
            {acknowledging ? tx("处理中...", "Working...") : tx("我知道了", "I Understand")}
          </button>
        </div>
      </div>
    </div>
  );
}
