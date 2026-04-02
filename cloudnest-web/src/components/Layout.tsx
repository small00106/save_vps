import { NavLink, Outlet, useLocation } from "react-router-dom";
import {
  LayoutDashboard,
  FolderOpen,
  Activity,
  Bell,
  ScrollText,
  LogOut,
  Menu,
  X,
} from "lucide-react";
import { useState } from "react";
import { useAuth } from "../hooks/useAuth";
import { useWebSocket } from "../hooks/useWebSocket";

const navItems = [
  { to: "/", icon: LayoutDashboard, label: "Dashboard" },
  { to: "/files", icon: FolderOpen, label: "Files" },
  { to: "/ping", icon: Activity, label: "Ping" },
  { to: "/alerts", icon: Bell, label: "Alerts" },
  { to: "/audit", icon: ScrollText, label: "Audit" },
] as const;

export default function Layout() {
  const { logout } = useAuth();
  const { connected } = useWebSocket();
  const [mobileOpen, setMobileOpen] = useState(false);
  const location = useLocation();

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
          {navItems.map(({ to, icon: Icon, label }) => (
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
                {label}
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
            CloudNest
          </span>
          <div className="flex-1" />
          <div className="flex items-center gap-2 text-xs text-text-muted">
            <span
              className={`h-2 w-2 rounded-full ${
                connected ? "bg-online animate-pulse-slow" : "bg-offline"
              }`}
            />
            {connected ? "Connected" : "Disconnected"}
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
