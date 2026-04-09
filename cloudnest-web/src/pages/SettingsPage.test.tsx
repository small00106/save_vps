import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import SettingsPage from "./SettingsPage";

vi.mock("../api/client", async () => {
  const actual = await vi.importActual("../api/client");
  return {
    ...actual,
    changePassword: vi.fn(),
  };
});

vi.mock("../hooks/useAuth", () => ({
  useAuth: () => ({
    user: {
      username: "admin",
      default_password_notice_required: true,
    },
  }),
}));

vi.mock("../i18n/useI18n", () => ({
  useI18n: () => ({
    tx: (zh: string) => zh,
  }),
}));

vi.mock("../contexts/PreferencesContext", () => ({
  usePreferences: () => ({
    localeMode: "zh",
    setLocaleMode: vi.fn(),
    themeMode: "light",
    setThemeMode: vi.fn(),
  }),
}));

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe("SettingsPage", () => {
  it("界面偏好的下拉框使用统一适配样式", () => {
    render(<SettingsPage />);

    expect(screen.getByLabelText("语言")).toHaveClass("appearance-none");
    expect(screen.getByLabelText("主题")).toHaveClass("appearance-none");
  });

  it("密码校验失败时通过 alert 区域提示错误", async () => {
    const user = userEvent.setup();
    render(<SettingsPage />);

    const passwordFields = screen.getAllByLabelText(/密码/);
    await user.type(passwordFields[0], "old-password");
    await user.type(passwordFields[1], "new-password");
    await user.type(screen.getByLabelText("确认新密码"), "different-password");
    await user.click(screen.getByRole("button", { name: "更新密码" }));

    expect(await screen.findByRole("alert")).toHaveTextContent("新密码与确认密码不一致");
  });
});
