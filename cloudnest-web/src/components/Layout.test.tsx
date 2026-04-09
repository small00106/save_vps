import { render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { describe, expect, it, vi } from "vitest";
import Layout from "./Layout";

vi.mock("../hooks/useAuth", () => ({
  useAuth: () => ({
    logout: vi.fn(),
  }),
}));

vi.mock("../hooks/useWebSocket", () => ({
  useWebSocket: () => ({
    connected: true,
  }),
}));

vi.mock("../contexts/PreferencesContext", () => ({
  usePreferences: () => ({
    themeMode: "system",
    setThemeMode: vi.fn(),
  }),
}));

vi.mock("../i18n/useI18n", () => ({
  useI18n: () => ({
    localeMode: "zh",
    setLocaleMode: vi.fn(),
    tx: (zh: string) => zh,
    t: (key: string) =>
      (
        {
          "app.name": "CloudNest",
          "header.language": "语言",
          "header.theme": "主题",
          "header.connected": "已连接",
          "header.disconnected": "已断开",
          "option.system": "跟随系统",
          "option.chinese": "中文",
          "option.english": "英文",
          "option.light": "浅色",
          "option.dark": "深色",
          "nav.dashboard": "仪表盘",
          "nav.files": "文件",
          "nav.ping": "Ping 任务",
          "nav.alerts": "告警",
          "nav.audit": "审计",
          "nav.settings": "设置",
        } as Record<string, string>
      )[key] ?? key,
  }),
}));

describe("Layout", () => {
  it("按新信息架构显示分组导航", () => {
    render(
      <MemoryRouter initialEntries={["/"]}>
        <Routes>
          <Route element={<Layout />}>
            <Route index element={<div>dashboard body</div>} />
          </Route>
        </Routes>
      </MemoryRouter>,
    );

    expect(screen.getByText("总览")).toBeInTheDocument();
    expect(screen.getByText("治理")).toBeInTheDocument();
    expect(screen.getByText("已连接")).toBeInTheDocument();
    expect(screen.getByLabelText("语言")).toHaveClass("appearance-none");
  });
});
