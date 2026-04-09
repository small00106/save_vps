import { render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { describe, expect, it, vi } from "vitest";
import Dashboard from "./Dashboard";

vi.mock("../api/client", () => ({
  getNodes: vi.fn().mockResolvedValue([]),
  getSettings: vi.fn().mockResolvedValue({ node_count: 0, online_count: 0, file_count: 0 }),
}));

vi.mock("../hooks/useWebSocket", () => ({
  useWebSocket: () => ({
    nodeData: new Map(),
    connected: false,
    statusVersion: 1,
  }),
}));

vi.mock("../i18n/useI18n", () => ({
  useI18n: () => ({
    tx: (zh: string) => zh,
  }),
}));

vi.mock("../hooks/useMouseGlow", () => ({
  useCardGlow: () => ({
    onMouseMove: vi.fn(),
  }),
}));

describe("Dashboard", () => {
  it("即使在空状态也保留页面标题和主状态标题", async () => {
    render(
      <MemoryRouter>
        <Dashboard />
      </MemoryRouter>,
    );

    await waitFor(() => {
      expect(screen.getByText("节点总览")).toBeInTheDocument();
    });
    expect(screen.getByText("暂无节点")).toBeInTheDocument();
  });
});
