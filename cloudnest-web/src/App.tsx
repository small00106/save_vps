import { Routes, Route, Navigate } from "react-router-dom";
import { AuthProvider, useAuth } from "./hooks/useAuth";
import { PreferencesProvider } from "./contexts/PreferencesContext";
import Layout from "./components/Layout";
import { lazy, Suspense, type ReactNode } from "react";

const LoginPage = lazy(() => import("./pages/LoginPage"));
const Dashboard = lazy(() => import("./pages/Dashboard"));
const NodeDetail = lazy(() => import("./pages/NodeDetail"));
const FileBrowser = lazy(() => import("./pages/FileBrowser"));
const Terminal = lazy(() => import("./pages/Terminal"));
const PingTasks = lazy(() => import("./pages/PingTasks"));
const Alerts = lazy(() => import("./pages/Alerts"));
const AuditLog = lazy(() => import("./pages/AuditLog"));
const SettingsPage = lazy(() => import("./pages/SettingsPage"));

function Loading() {
  return (
    <div className="flex h-dvh items-center justify-center bg-bg">
      <div className="h-6 w-6 animate-spin rounded-full border-2 border-accent border-t-transparent" />
    </div>
  );
}

function RequireAuth({ children }: { children: ReactNode }) {
  const { user, loading } = useAuth();
  if (loading) return <Loading />;
  if (!user) return <Navigate to="/login" replace />;
  return <>{children}</>;
}

function AppRoutes() {
  return (
    <Suspense fallback={<Loading />}>
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route
          element={
            <RequireAuth>
              <Layout />
            </RequireAuth>
          }
        >
          <Route index element={<Dashboard />} />
          <Route path="nodes/:uuid" element={<NodeDetail />} />
          <Route path="files" element={<FileBrowser />} />
          <Route path="terminal/:uuid" element={<Terminal />} />
          <Route path="ping" element={<PingTasks />} />
          <Route path="alerts" element={<Alerts />} />
          <Route path="audit" element={<AuditLog />} />
          <Route path="settings" element={<SettingsPage />} />
        </Route>
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </Suspense>
  );
}

export default function App() {
  return (
    <PreferencesProvider>
      <AuthProvider>
        <AppRoutes />
      </AuthProvider>
    </PreferencesProvider>
  );
}
