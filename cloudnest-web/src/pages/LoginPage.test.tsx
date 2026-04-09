import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import LoginPage from "./LoginPage";
import { ApiError } from "../api/client";

const loginMock = vi.fn();
const clearNoticeMock = vi.fn();

vi.mock("../hooks/useAuth", () => ({
  useAuth: () => ({
    login: loginMock,
    notice: "",
    clearNotice: clearNoticeMock,
  }),
}));

vi.mock("../i18n/useI18n", () => ({
  useI18n: () => ({
    tx: (zh: string) => zh,
  }),
}));

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe("LoginPage", () => {
  it("登录失败时通过 alert 区域播报错误", async () => {
    loginMock.mockRejectedValueOnce(new ApiError(401, "unauthorized"));
    const user = userEvent.setup();

    render(<LoginPage />);

    await user.type(screen.getByLabelText("用户名"), "admin");
    await user.type(screen.getByLabelText("密码"), "wrong");
    await user.click(screen.getByRole("button", { name: "登录" }));

    expect(await screen.findByRole("alert")).toHaveTextContent("用户名或密码错误");
  });

  it("后端不可用时提示服务连接失败", async () => {
    loginMock.mockRejectedValueOnce(new TypeError("Failed to fetch"));
    const user = userEvent.setup();

    render(<LoginPage />);

    await user.type(screen.getByLabelText("用户名"), "admin");
    await user.type(screen.getByLabelText("密码"), "admin");
    await user.click(screen.getByRole("button", { name: "登录" }));

    expect(await screen.findByRole("alert")).toHaveTextContent("无法连接到后端服务");
  });
});
