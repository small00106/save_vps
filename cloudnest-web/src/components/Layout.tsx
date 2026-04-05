import { NavLink, Outlet, useLocation } from "react-router-dom";
import {
  LayoutDashboard,
  FolderOpen,
  Activity,
  Bell,
  ScrollText,
  Settings,
  LogOut,
  Menu,
  X,
} from "lucide-react";
import { useState } from "react";
import { useAuth } from "../hooks/useAuth";
import { useWebSocket } from "../hooks/useWebSocket";
import { usePreferences, type LocaleMode, type ThemeMode } from "../contexts/PreferencesContext";
import { useI18n } from "../i18n/useI18n";

const navItems = [
  { to: "/", icon: LayoutDashboard, labelKey: "nav.dashboard" },
  { to: "/files", icon: FolderOpen, labelKey: "nav.files" },
  { to: "/ping", icon: Activity, labelKey: "nav.ping" },
  { to: "/alerts", icon: Bell, labelKey: "nav.alerts" },
  { to: "/audit", icon: ScrollText, labelKey: "nav.audit" },
  { to: "/settings", icon: Settings, labelKey: "nav.settings" },
] as const;

export default function Layout() {
  const { logout } = useAuth();
  const { connected } = useWebSocket();
  const { themeMode, setThemeMode } = usePreferences();
  const { t, localeMode, setLocaleMode } = useI18n();
  const [mobileOpen, setMobileOpen] = useState(false);
  const location = useLocation();

  const handleLocaleChange = (event: React.ChangeEvent<HTMLSelectElement>) => {
    setLocaleMode(event.target.value as LocaleMode);
  };

  const handleThemeChange = (event: React.ChangeEvent<HTMLSelectElement>) => {
    setThemeMode(event.target.value as ThemeMode);
  };

  return (
    <div className="flex h-dvh overflow-hidden bg-bg">
      {/* Mobile overlay */}
      {mobileOpen && (
        <div
          className="fixed inset-0 z-40 bg-black/50 md:hidden"
          onClick={() => setMobileOpen(false)}
        />
      )}

      {/* Sidebar */}
      <aside
        className={`
          fixed inset-y-0 left-0 z-50 flex w-16 flex-col items-center border-r border-border bg-card py-4
          transition-transform duration-200 md:static md:translate-x-0
          ${mobileOpen ? "translate-x-0" : "-translate-x-full"}
        `}
      >
        {/* Brand */}
        <div className="mb-6 flex h-8 w-8 items-center justify-center rounded-lg bg-accent font-bold text-sm text-white">
          CN
        </div>

        {/* Nav items */}
        <nav className="flex flex-1 flex-col items-center gap-1">
          {navItems.map(({ to, icon: Icon, labelKey }) => (
            <NavLink
              key={to}
              to={to}
              end={to === "/"}
              onClick={() => setMobileOpen(false)}
              className={({ isActive }) =>
                `group relative flex h-10 w-10 items-center justify-center rounded-lg transition-colors duration-150 ${
                  isActive
                    ? "bg-accent/15 text-accent"
                    : "text-text-muted hover:bg-border/50 hover:text-text-secondary"
                }`
              }
            >
              <Icon size={20} />
              <span className="pointer-events-none absolute left-14 hidden rounded-md bg-card px-2 py-1 text-xs text-text-primary shadow-lg border border-border group-hover:block">
                {t(labelKey)}
              </span>
            </NavLink>
          ))}
        </nav>

        {/* Logout */}
        <button
          onClick={logout}
          className="flex h-10 w-10 items-center justify-center rounded-lg text-text-muted transition-colors hover:bg-border/50 hover:text-offline"
        >
          <LogOut size={20} />
        </button>
      </aside>

      {/* Main area */}
      <div className="flex flex-1 flex-col overflow-hidden">
        {/* Top bar */}
        <header className="flex h-12 shrink-0 items-center gap-3 border-b border-border bg-card/50 px-4">
          <button
            className="flex h-8 w-8 items-center justify-center rounded-lg text-text-muted hover:bg-border/50 md:hidden"
            onClick={() => setMobileOpen(!mobileOpen)}
          >
            {mobileOpen ? <X size={18} /> : <Menu size={18} />}
          </button>
          <span className="text-sm font-semibold tracking-tight text-text-primary">
            {t("app.name")}
          </span>
          <div className="flex-1" />
          <div className="hidden items-center gap-2 text-xs text-text-muted lg:flex">
            <label className="text-text-muted">{t("header.language")}</label>
            <select
              value={localeMode}
              onChange={handleLocaleChange}
              className="h-8 rounded-lg border border-border bg-card px-2 text-xs text-text-primary outline-none transition focus:border-accent"
            >
              <option value="system">{t("option.system")}</option>
              <option value="zh">{t("option.chinese")}</option>
              <option value="en">{t("option.english")}</option>
            </select>
          </div>
          <div className="hidden items-center gap-2 text-xs text-text-muted lg:flex">
            <label className="text-text-muted">{t("header.theme")}</label>
            <select
              value={themeMode}
              onChange={handleThemeChange}
              className="h-8 rounded-lg border border-border bg-card px-2 text-xs text-text-primary outline-none transition focus:border-accent"
            >
              <option value="system">{t("option.system")}</option>
              <option value="light">{t("option.light")}</option>
              <option value="dark">{t("option.dark")}</option>
            </select>
          </div>
          <div className="flex items-center gap-2 text-xs text-text-muted">
            <span
              className={`h-2 w-2 rounded-full ${
                connected ? "bg-online animate-pulse-slow" : "bg-offline"
              }`}
            />
            {connected ? t("header.connected") : t("header.disconnected")}
          </div>
        </header>

        {/* Page content */}
        <main className="flex-1 overflow-y-auto p-4 md:p-6">
          <div className="animate-fade-in" key={location.pathname}>
            <Outlet />
          </div>
        </main>
      </div>
    </div>
  );
}
