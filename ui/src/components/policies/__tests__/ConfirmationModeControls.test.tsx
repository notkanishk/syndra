// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { makeProxyFetch } from "@/test-utils/proxyFetch";

import { ConfirmationModeControls } from "../ConfirmationModeControls";

vi.mock("@/lib/toast", () => ({
  toastSuccess: vi.fn(),
  toastError: vi.fn(),
  toastInfo: vi.fn(),
}));

let proxy: ReturnType<typeof makeProxyFetch>;

beforeEach(() => {
  proxy = makeProxyFetch();
  global.fetch = proxy.fetchImpl;
  proxy.register("POST", /\/api\/proxy\/policies\/confirmation-mode(\?|$)/, () => ({
    updated: 2,
    kind: "rule",
    mode: "manual",
  }));
});

afterEach(() => {
  vi.restoreAllMocks();
});

function renderControls(selectedIds: Set<string>, onDone = vi.fn()) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, staleTime: 0, gcTime: 0 } },
  });
  return {
    onDone,
    ...render(
      <QueryClientProvider client={client}>
        <ConfirmationModeControls kind="rule" selectedIds={selectedIds} onDone={onDone} />
      </QueryClientProvider>,
    ),
  };
}

describe("ConfirmationModeControls", () => {
  it("shows the current selection count", () => {
    renderControls(new Set(["r1", "r2"]));
    expect(screen.getByText("2 selected")).toBeInTheDocument();
  });

  it("disables Apply when nothing is selected", () => {
    renderControls(new Set());
    expect(screen.getByRole("button", { name: /Apply to 0 selected/i })).toBeDisabled();
  });

  it("Apply POSTs the chosen mode and selected ids, then calls onDone", async () => {
    const { onDone } = renderControls(new Set(["r1", "r2"]));

    fireEvent.change(screen.getByLabelText("Confirmation mode"), { target: { value: "manual" } });
    fireEvent.click(screen.getByRole("button", { name: /Apply to 2 selected/i }));

    await waitFor(() => {
      const call = proxy.calls.find(
        (c) => c.method === "POST" && c.url.includes("/policies/confirmation-mode"),
      );
      expect(call).toBeDefined();
      expect(call?.body).toMatchObject({ kind: "rule", ids: ["r1", "r2"], mode: "manual" });
    });
    await waitFor(() => {
      expect(onDone).toHaveBeenCalled();
    });
  });
});
