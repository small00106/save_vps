import {
  createContext,
  useContext,
  useState,
  useEffect,
  useCallback,
  type ReactNode,
} from "react";
import { useNavigate, useLocation } from "react-router-dom";
import {
  getMe,
  login as apiLogin,
  logout as apiLogout,
  type User,
  ApiError,
} from "../api/client";
import { createElement } from "react";

interface AuthState {
  user: User | null;
  loading: boolean;
  login: (username: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
}

const AuthContext = createContext<AuthState | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);
  const navigate = useNavigate();
  const location = useLocation();

  useEffect(() => {
    getMe()
      .then(setUser)
      .catch(() => {
        setUser(null);
        if (location.pathname !== "/login") {
          navigate("/login", { replace: true });
        }
      })
      .finally(() => setLoading(false));
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const login = useCallback(
    async (username: string, password: string) => {
      await apiLogin(username, password);
      const me = await getMe();
      setUser(me);
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
    setUser(null);
    navigate("/login", { replace: true });
  }, [navigate]);

  return createElement(
    AuthContext.Provider,
    { value: { user, loading, login, logout } },
    children,
  );
}

export function useAuth(): AuthState {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth must be used within AuthProvider");
  return ctx;
}
