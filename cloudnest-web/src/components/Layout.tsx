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
    <div className="flex h-dvh overflow-hidden">
      {/* Mobile overlay */}
      {mobileOpen && (
        <div
          className="fixed inset-0 z-40 bg-black/60 backdrop-blur-sm md:hidden transition-opacity"
          onClick={() => setMobileOpen(false)}
        />
      )}

      {/* Sidebar */}
      <aside
        className={`
          fixed inset-y-0 left-0 z-50 flex w-[72px] flex-col items-center 
          glass-card border-r border-border/50 py-5
          transition-transform duration-300 ease-out md:static md:translate-x-0
          ${mobileOpen ? "translate-x-0" : "-translate-x-full"}
        `}
      >
        {/* Logo area */}
        <div 
          className="mb-8 flex h-10 w-10 items-center justify-center rounded-xl font-bold text-sm text-white"
          style={{
            background: "linear-gradient(135deg, var(--ui-accent), var(--ui-accent-secondary))",
            boxShadow: "0 4px 15px var(--ui-accent-glow)",
          }}
        >
          CN
        </div>

        {/* Nav items */}
        <nav className="flex flex-1 flex-col items-center gap-2">
          {navItems.map(({ to, icon: Icon, labelKey }) => (
            <NavLink
              key={to}
              to={to}
              end={to === "/"}
              onClick={() => setMobileOpen(false)}
              className={({ isActive }) =>
                `group relative flex h-11 w-11 items-center justify-center rounded-xl transition-all duration-200 ${
                  isActive
                    ? "bg-accent/15 text-accent shadow-sm"
                    : "text-text-muted hover:bg-accent/10 hover:text-accent"
                }`
              }
            >
              {({ isActive }) => (
                <>
                  <Icon size={20} />
                  {/* Active indicator glow */}
                  {isActive && (
                    <span 
                      className="absolute inset-0 rounded-xl opacity-40 -z-10"
                      style={{
                        boxShadow: "0 0 20px var(--ui-accent-glow)",
                      }}
                    />
                  )}
                  {/* Tooltip */}
                  <span className="pointer-events-none absolute left-full ml-3 hidden rounded-lg glass-card px-3 py-1.5 text-xs font-medium text-text-primary shadow-lg group-hover:block whitespace-nowrap">
                    {t(labelKey)}
                  </span>
                </>
              )}
            </NavLink>
          ))}
        </nav>

        {/* Logout */}
        <button
          onClick={logout}
          className="flex h-11 w-11 items-center justify-center rounded-xl text-text-muted transition-all duration-200 hover:bg-offline/10 hover:text-offline"
        >
          <LogOut size={20} />
        </button>
      </aside>

      {/* Main area */}
      <div className="flex flex-1 flex-col overflow-hidden">
        {/* Top bar */}
        <header className="relative flex h-14 shrink-0 items-center gap-3 border-b border-border/50 glass px-4">
          {/* Gradient bottom border */}
          <div 
            className="absolute bottom-0 left-0 right-0 h-[1px] opacity-50"
            style={{
              background: "linear-gradient(90deg, transparent, var(--ui-accent), var(--ui-accent-secondary), transparent)",
            }}
          />

          <button
            className="flex h-9 w-9 items-center justify-center rounded-lg text-text-muted hover:bg-accent/10 hover:text-accent md:hidden transition-colors"
            onClick={() => setMobileOpen(!mobileOpen)}
          >
            {mobileOpen ? <X size={20} /> : <Menu size={20} />}
          </button>

          <span className="text-sm font-semibold tracking-tight text-text-primary">
            {t("app.name")}
          </span>

          <div className="flex-1" />

          {/* Controls */}
          <div className="hidden items-center gap-3 lg:flex">
            <div className="flex items-center gap-2 text-xs">
              <label className="text-text-muted">{t("header.language")}</label>
              <select
                value={localeMode}
                onChange={handleLocaleChange}
                className="h-8 rounded-lg border border-border/50 bg-card/50 px-2 text-xs text-text-primary outline-none transition-all hover:border-border focus:border-accent"
              >
                <option value="system">{t("option.system")}</option>
                <option value="zh">{t("option.chinese")}</option>
                <option value="en">{t("option.english")}</option>
              </select>
            </div>

            <div className="flex items-center gap-2 text-xs">
              <label className="text-text-muted">{t("header.theme")}</label>
              <select
                value={themeMode}
                onChange={handleThemeChange}
                className="h-8 rounded-lg border border-border/50 bg-card/50 px-2 text-xs text-text-primary outline-none transition-all hover:border-border focus:border-accent"
              >
                <option value="system">{t("option.system")}</option>
                <option value="light">{t("option.light")}</option>
                <option value="dark">{t("option.dark")}</option>
              </select>
            </div>
          </div>

          {/* Connection status */}
          <div className="flex items-center gap-2 text-xs text-text-muted">
            <span
              className={`relative h-2 w-2 rounded-full ${
                connected ? "bg-online" : "bg-offline"
              }`}
            >
              {connected && (
                <span className="absolute inset-0 animate-ping rounded-full bg-online opacity-75" />
              )}
            </span>
            <span className="hidden sm:inline">
              {connected ? t("header.connected") : t("header.disconnected")}
            </span>
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
