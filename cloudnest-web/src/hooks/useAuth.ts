import {
  createContext,
  createElement,
  useContext,
  useState,
  useEffect,
  useCallback,
  type ReactNode,
} from "react";
import { useNavigate, useLocation } from "react-router-dom";
import {
  acknowledgeDefaultPasswordNotice as apiAcknowledgeDefaultPasswordNotice,
  getMe,
  login as apiLogin,
  logout as apiLogout,
  onAuthExpired,
  resetAuthExpiredState,
  type User,
  ApiError,
} from "../api/client";
import DefaultPasswordNoticeModal from "../components/DefaultPasswordNoticeModal";
import { useI18n } from "../i18n/useI18n";

interface AuthState {
  user: User | null;
  loading: boolean;
  notice: string;
  clearNotice: () => void;
  login: (username: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
}

const AuthContext = createContext<AuthState | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);
  const [notice, setNotice] = useState("");
  const [acknowledgingDefaultPasswordNotice, setAcknowledgingDefaultPasswordNotice] = useState(false);
  const [defaultPasswordNoticeError, setDefaultPasswordNoticeError] = useState("");
  const navigate = useNavigate();
  const location = useLocation();
  const { tx } = useI18n();

  const clearNotice = useCallback(() => {
    setNotice("");
  }, []);

  const clearLocalSession = useCallback(() => {
    setUser(null);
    setDefaultPasswordNoticeError("");
  }, []);

  useEffect(() => {
    getMe()
      .then((me) => {
        setUser(me);
        resetAuthExpiredState();
        setNotice("");
        if (!me.default_password_notice_required) {
          setDefaultPasswordNoticeError("");
        }
      })
      .catch(() => {
        clearLocalSession();
        if (location.pathname !== "/login") {
          navigate("/login", { replace: true });
        }
      })
      .finally(() => setLoading(false));
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    return onAuthExpired(() => {
      clearLocalSession();
      setNotice(
        tx(
          "登录已过期，请重新登录。",
          "Your session has expired. Please sign in again.",
        ),
      );
      if (location.pathname !== "/login") {
        navigate("/login", { replace: true });
      }
    });
  }, [clearLocalSession, location.pathname, navigate, tx]);

  const login = useCallback(
    async (username: string, password: string) => {
      setNotice("");
      await apiLogin(username, password);
      const me = await getMe();
      setUser(me);
      resetAuthExpiredState();
      if (!me.default_password_notice_required) {
        setDefaultPasswordNoticeError("");
      }
      navigate("/", { replace: true });
    },
    [navigate],
  );

  const logout = useCallback(async () => {
    try {
      await apiLogout();
    } catch (e) {
      if (!(e instanceof ApiError && e.status === 401)) throw e;
    }
    clearLocalSession();
    resetAuthExpiredState();
    setNotice("");
    navigate("/login", { replace: true });
  }, [clearLocalSession, navigate]);

  const acknowledgeDefaultPasswordNotice = useCallback(async () => {
    if (!user?.default_password_notice_required) return;

    setAcknowledgingDefaultPasswordNotice(true);
    setDefaultPasswordNoticeError("");
    try {
      await apiAcknowledgeDefaultPasswordNotice();
      setUser((currentUser) =>
        currentUser
          ? { ...currentUser, default_password_notice_required: false }
          : currentUser,
      );
    } catch {
      setDefaultPasswordNoticeError(
        tx(
          "关闭默认密码提醒失败，请稍后重试。",
          "Failed to dismiss the default-password reminder. Please try again later.",
        ),
      );
    } finally {
      setAcknowledgingDefaultPasswordNotice(false);
    }
  }, [tx, user]);

  return createElement(
    AuthContext.Provider,
    { value: { user, loading, notice, clearNotice, login, logout } },
    children,
    user?.default_password_notice_required
      ? createElement(DefaultPasswordNoticeModal, {
          acknowledging: acknowledgingDefaultPasswordNotice,
          error: defaultPasswordNoticeError,
          onAcknowledge: acknowledgeDefaultPasswordNotice,
        })
      : null,
  );
}

export function useAuth(): AuthState {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth must be used within AuthProvider");
  return ctx;
}
