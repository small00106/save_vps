import type { Locale } from "../contexts/PreferencesContext";

export const messages = {
  en: {
    "app.name": "CloudNest",
    "nav.dashboard": "Dashboard",
    "nav.files": "Files",
    "nav.ping": "Ping",
    "nav.alerts": "Alerts",
    "nav.audit": "Audit",
    "nav.settings": "Settings",
    "header.connected": "Connected",
    "header.disconnected": "Disconnected",
    "header.language": "Language",
    "header.theme": "Theme",
    "option.system": "System",
    "option.chinese": "中文",
    "option.english": "English",
    "option.light": "Light",
    "option.dark": "Dark",
  },
  zh: {
    "app.name": "CloudNest",
    "nav.dashboard": "总览",
    "nav.files": "文件",
    "nav.ping": "Ping 探测",
    "nav.alerts": "告警",
    "nav.audit": "审计日志",
    "nav.settings": "设置",
    "header.connected": "已连接",
    "header.disconnected": "未连接",
    "header.language": "语言",
    "header.theme": "主题",
    "option.system": "跟随系统",
    "option.chinese": "中文",
    "option.english": "English",
    "option.light": "浅色",
    "option.dark": "深色",
  },
} as const;

export type MessageKey = keyof typeof messages.en;

export function getMessage(locale: Locale, key: MessageKey): string {
  return messages[locale][key] ?? messages.en[key] ?? key;
}

