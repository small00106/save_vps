import { Link, Outlet, useLocation } from "react-router-dom";
import { Activity, Bell, ChevronRight, FolderOpen, LayoutDashboard, LogOut, Menu, ScrollText, Settings, X } from "lucide-react";
import { useMemo, useState } from "react";
import { useAuth } from "../hooks/useAuth";
import { useWebSocket } from "../hooks/useWebSocket";
import { usePreferences, type ThemeMode } from "../contexts/PreferencesContext";
import { useI18n } from "../i18n/useI18n";
import { SelectField, StatusBadge } from "./ui";

const navGroups = [
  {
    id: "overview",
    title: { zh: "总览", en: "Overview" },
    items: [
      {
        to: "/",
        icon: LayoutDashboard,
        labelKey: "nav.dashboard",
        match: (path: string) => path === "/" || path.startsWith("/nodes/") || path.startsWith("/terminal/"),
      },
    ],
  },
  {
    id: "storage",
    title: { zh: "存储", en: "Storage" },
    items: [{ to: "/files", icon: FolderOpen, labelKey: "nav.files", match: (path: string) => path.startsWith("/files") }],
  },
  {
    id: "operations",
    title: { zh: "运维", en: "Operations" },
    items: [{ to: "/ping", icon: Activity, labelKey: "nav.ping", match: (path: string) => path.startsWith("/ping") }],
  },
  {
    id: "governance",
    title: { zh: "治理", en: "Governance" },
    items: [
      { to: "/alerts", icon: Bell, labelKey: "nav.alerts", match: (path: string) => path.startsWith("/alerts") },
      { to: "/audit", icon: ScrollText, labelKey: "nav.audit", match: (path: string) => path.startsWith("/audit") },
    ],
  },
  {
    id: "settings",
    title: { zh: "设置", en: "Settings" },
    items: [{ to: "/settings", icon: Settings, labelKey: "nav.settings", match: (path: string) => path.startsWith("/settings") }],
  },
] as const;

export default function Layout() {
  const { logout } = useAuth();
  const { connected } = useWebSocket();
  const { themeMode, setThemeMode } = usePreferences();
  const { t, tx, localeMode, setLocaleMode } = useI18n();
  const [mobileOpen, setMobileOpen] = useState(false);
  const location = useLocation();

  const pageMeta = useMemo(() => {
    const pathname = location.pathname;

    if (pathname.startsWith("/nodes/")) {
      return {
        title: tx("节点详情", "Node Detail"),
      };
    }
    if (pathname.startsWith("/files")) {
      return {
        title: tx("文件检索", "File Search"),
      };
    }
    if (pathname.startsWith("/ping")) {
      return {
        title: tx("Ping 任务", "Ping Tasks"),
      };
    }
    if (pathname.startsWith("/alerts")) {
      return {
        title: tx("告警治理", "Alert Governance"),
      };
    }
    if (pathname.startsWith("/audit")) {
      return {
        title: tx("审计日志", "Audit Log"),
      };
    }
    if (pathname.startsWith("/settings")) {
      return {
        title: tx("设置", "Settings"),
      };
    }
    return {
      title: tx("节点总览", "Node Overview"),
    };
  }, [location.pathname, tx]);

  const handleThemeChange = (event: React.ChangeEvent<HTMLSelectElement>) => {
    setThemeMode(event.target.value as ThemeMode);
  };

  return (
    <div className="relative flex min-h-dvh bg-transparent text-text-primary md:gap-5">
      {mobileOpen ? (
        <div className="fixed inset-0 z-40 bg-slate-950/30 backdrop-blur-sm md:hidden" onClick={() => setMobileOpen(false)} />
      ) : null}

      <aside
        className={[
          "surface-card fixed inset-y-4 left-4 z-50 flex w-[248px] flex-col rounded-[28px] px-3 py-4 transition-transform md:sticky md:top-4 md:h-[calc(100dvh-2rem)] md:translate-x-0",
          mobileOpen ? "translate-x-0" : "-translate-x-[120%]",
        ].join(" ")}
      >
        <div className="flex justify-end px-1 md:hidden">
          <button
            className="rounded-xl p-2 text-text-muted hover:bg-surface-subtle hover:text-text-primary md:hidden"
            onClick={() => setMobileOpen(false)}
            aria-label={tx("关闭导航", "Close navigation")}
          >
            <X size={18} />
          </button>
        </div>

        <nav className="mt-4 flex flex-1 flex-col overflow-y-auto pr-1" aria-label="Primary">
          <div className="space-y-5">
            {navGroups.map((group, index) => (
              <div
                key={group.id}
                className={[
                  "space-y-2",
                  index === 1 ? "pt-4" : "",
                ].join(" ").trim()}
              >
                <p className="px-2 text-xs font-semibold uppercase tracking-[0.22em] text-text-muted">
                  {tx(group.title.zh, group.title.en)}
                </p>
                <div className="space-y-1">
                  {group.items.map(({ to, icon: Icon, labelKey, match }) => {
                    const isActive = match(location.pathname);
                    return (
                      <Link
                        key={to}
                        to={to}
                        aria-current={isActive ? "page" : undefined}
                        onClick={() => setMobileOpen(false)}
                        className={[
                          "group flex items-center justify-between gap-3 rounded-2xl px-3 py-3 text-sm font-medium transition-all duration-200",
                          isActive
                            ? "bg-accent-muted text-accent"
                            : "text-text-secondary hover:bg-surface-subtle hover:text-text-primary",
                        ].join(" ")}
                      >
                        <span className="flex items-center gap-3">
                          <span className={isActive ? "text-accent" : "text-text-muted"}>
                            <Icon size={18} />
                          </span>
                          <span>{t(labelKey)}</span>
                        </span>
                        <ChevronRight size={16} className={isActive ? "text-accent" : "text-text-muted"} />
                      </Link>
                    );
                  })}
                </div>
              </div>
            ))}
          </div>
        </nav>

        <div className="mt-4 border-t border-border px-2 pt-4">
          <button
            onClick={() => void logout()}
            className="flex w-full items-center justify-center gap-2 rounded-2xl border border-border bg-surface px-4 py-3 text-sm font-medium text-text-secondary transition-colors hover:border-offline/30 hover:bg-offline/10 hover:text-offline"
          >
            <LogOut size={16} />
            {tx("退出登录", "Sign out")}
          </button>
        </div>
      </aside>

      <div className="flex min-w-0 flex-1 flex-col md:pr-4">
        <header className="sticky top-0 z-30 px-4 pt-4 md:px-0">
          <div className="surface-card flex min-h-[88px] items-center rounded-[28px] px-4 py-4 md:px-6">
            <div className="flex min-w-0 flex-1 items-start gap-3">
              <button
                className="rounded-xl p-2 text-text-muted hover:bg-surface-subtle hover:text-text-primary md:hidden"
                onClick={() => setMobileOpen(true)}
                aria-label={tx("打开导航", "Open navigation")}
              >
                <Menu size={18} />
              </button>
              <div className="min-w-0">
                <h1 className="truncate text-xl font-semibold tracking-tight text-text-primary">{pageMeta.title}</h1>
              </div>
            </div>

            <div className="hidden items-center gap-3 xl:flex">
              <label className="flex items-center gap-2 text-xs text-text-muted">
                <span>{t("header.language")}</span>
                <SelectField
                  value={localeMode}
                  onChange={(event) => setLocaleMode(event.target.value as typeof localeMode)}
                  className="!w-auto min-w-[104px] !rounded-xl !px-3 !py-2 !pr-10"
                >
                  <option value="system">{t("option.system")}</option>
                  <option value="zh">{t("option.chinese")}</option>
                  <option value="en">{t("option.english")}</option>
                </SelectField>
              </label>
              <label className="flex items-center gap-2 text-xs text-text-muted">
                <span>{t("header.theme")}</span>
                <SelectField
                  value={themeMode}
                  onChange={handleThemeChange}
                  className="!w-auto min-w-[104px] !rounded-xl !px-3 !py-2 !pr-10"
                >
                  <option value="system">{t("option.system")}</option>
                  <option value="light">{t("option.light")}</option>
                  <option value="dark">{t("option.dark")}</option>
                </SelectField>
              </label>
            </div>

            <div className="ml-3">
              <StatusBadge tone={connected ? "success" : "danger"} label={connected ? t("header.connected") : t("header.disconnected")} />
            </div>
          </div>
        </header>

        <main className="flex-1 px-4 pb-6 pt-6 md:px-0">
          <div className="mx-auto max-w-[1680px] animate-fade-in">
            <Outlet />
          </div>
        </main>
      </div>
    </div>
  );
}
