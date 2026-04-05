/* eslint-disable react-refresh/only-export-components */
import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";

export type Locale = "zh" | "en";
export type LocaleMode = Locale | "system";

export type Theme = "light" | "dark";
export type ThemeMode = Theme | "system";

interface PreferencesContextValue {
  localeMode: LocaleMode;
  locale: Locale;
  setLocaleMode: (mode: LocaleMode) => void;
  themeMode: ThemeMode;
  theme: Theme;
  setThemeMode: (mode: ThemeMode) => void;
}

const LOCALE_STORAGE_KEY = "cloudnest.localeMode";
const THEME_STORAGE_KEY = "cloudnest.themeMode";

function isLocaleMode(value: string): value is LocaleMode {
  return value === "system" || value === "zh" || value === "en";
}

function isThemeMode(value: string): value is ThemeMode {
  return value === "system" || value === "light" || value === "dark";
}

function detectSystemLocale(): Locale {
  const language = navigator.language.toLowerCase();
  return language.startsWith("zh") ? "zh" : "en";
}

function detectSystemTheme(): Theme {
  return window.matchMedia("(prefers-color-scheme: dark)").matches
    ? "dark"
    : "light";
}

function readLocaleMode(): LocaleMode {
  const raw = localStorage.getItem(LOCALE_STORAGE_KEY);
  if (raw && isLocaleMode(raw)) return raw;
  return "system";
}

function readThemeMode(): ThemeMode {
  const raw = localStorage.getItem(THEME_STORAGE_KEY);
  if (raw && isThemeMode(raw)) return raw;
  return "system";
}

const PreferencesContext = createContext<PreferencesContextValue | null>(null);

export function PreferencesProvider({ children }: { children: ReactNode }) {
  const [localeMode, setLocaleModeState] = useState<LocaleMode>(readLocaleMode);
  const [themeMode, setThemeModeState] = useState<ThemeMode>(readThemeMode);
  const [systemLocale, setSystemLocale] = useState<Locale>(detectSystemLocale);
  const [systemTheme, setSystemTheme] = useState<Theme>(detectSystemTheme);

  useEffect(() => {
    const media = window.matchMedia("(prefers-color-scheme: dark)");
    const handleChange = (event: MediaQueryListEvent) => {
      setSystemTheme(event.matches ? "dark" : "light");
    };

    media.addEventListener("change", handleChange);
    return () => media.removeEventListener("change", handleChange);
  }, []);

  useEffect(() => {
    const handleLanguageChange = () => {
      setSystemLocale(detectSystemLocale());
    };

    window.addEventListener("languagechange", handleLanguageChange);
    return () => window.removeEventListener("languagechange", handleLanguageChange);
  }, []);

  useEffect(() => {
    localStorage.setItem(LOCALE_STORAGE_KEY, localeMode);
  }, [localeMode]);

  useEffect(() => {
    localStorage.setItem(THEME_STORAGE_KEY, themeMode);
  }, [themeMode]);

  const locale = localeMode === "system" ? systemLocale : localeMode;
  const theme = themeMode === "system" ? systemTheme : themeMode;

  useEffect(() => {
    document.documentElement.dataset.theme = theme;
    document.documentElement.style.colorScheme = theme;
  }, [theme]);

  const setLocaleMode = useCallback((mode: LocaleMode) => {
    setLocaleModeState(mode);
  }, []);

  const setThemeMode = useCallback((mode: ThemeMode) => {
    setThemeModeState(mode);
  }, []);

  const value = useMemo<PreferencesContextValue>(
    () => ({
      localeMode,
      locale,
      setLocaleMode,
      themeMode,
      theme,
      setThemeMode,
    }),
    [locale, localeMode, setLocaleMode, setThemeMode, theme, themeMode],
  );

  return (
    <PreferencesContext.Provider value={value}>
      {children}
    </PreferencesContext.Provider>
  );
}

export function usePreferences() {
  const ctx = useContext(PreferencesContext);
  if (!ctx) {
    throw new Error("usePreferences must be used within PreferencesProvider");
  }
  return ctx;
}

