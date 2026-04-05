import { useCallback } from "react";
import { usePreferences } from "../contexts/PreferencesContext";
import { getMessage, type MessageKey } from "./messages";

export function useI18n() {
  const { locale, localeMode, setLocaleMode } = usePreferences();

  const t = useCallback(
    (key: MessageKey) => getMessage(locale, key),
    [locale],
  );

  const tx = useCallback(
    (zh: string, en: string) => (locale === "zh" ? zh : en),
    [locale],
  );

  return {
    locale,
    localeMode,
    setLocaleMode,
    t,
    tx,
  };
}

