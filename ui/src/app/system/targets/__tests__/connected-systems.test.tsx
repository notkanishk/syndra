// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import ConnectedSystemsPage from "@/app/system/targets/page";
import type { TargetSummary } from "@/lib/queries/useTargets";

/**
 * The index exists for the deployment that has registered NOTHING. That case is
 * the one worth testing hardest: before this page it produced no row, no page
 * and no sentence anywhere in the product, and an operator reading that silence
 * concluded the add-on platform had not shipped.
 */

const state = {
  targets: [] as TargetSummary[],
  isLoading: false,
  error: null as unknown,
};

vi.mock("@/lib/queries/useTargets", async () => {
  const actual = await vi.importActual<typeof import("@/lib/queries/useTargets")>(
    "@/lib/queries/useTargets",
  );
  return {
    ...actual,
    useTargets: () => ({
      data: state.targets,
      isLoading: state.isLoading,
      error: state.error,
      refetch: vi.fn(),
    }),
  };
});

function target(over: Partial<TargetSummary> = {}): TargetSummary {
  return {
    target: "truenas",
    registered: true,
    auth_mode: "derived",
    callable: true,
    operations: [
      { id: "account.provision", scope: "member", confirm: false, available: true },
      { id: "account.release", scope: "member", confirm: true, available: true },
    ],
    circuit_open: false,
    ...over,
  };
}

beforeEach(() => {
  state.targets = [];
  state.isLoading = false;
  state.error = null;
});

describe("connected systems", () => {
  it("says so, in words, when the deployment registered nothing", () => {
    render(<ConnectedSystemsPage />);

    expect(screen.getByText("No system is connected.")).toBeTruthy();
    // And says what would connect one, because the operator's next question is
    // not "is it empty" but "what do I do about it".
    expect(screen.getByText(/ADDON_TARGETS/)).toBeTruthy();
  });

  it("does not claim a count it does not have", () => {
    render(<ConnectedSystemsPage />);
    expect(screen.queryByText(/registered$/)).toBeNull();
  });

  it("lists a registered target and what it can do", () => {
    state.targets = [target()];
    render(<ConnectedSystemsPage />);

    expect(screen.getByText("TrueNAS")).toBeTruthy();
    expect(screen.getByText("1 add-on registered")).toBeTruthy();
    expect(screen.getByText("answering")).toBeTruthy();
    expect(screen.getByText("2")).toBeTruthy();
  });

  // Registration is a deployment fact and a manifest is a runtime one. Showing
  // `0` for a target that has never answered would be a claim about the TARGET
  // — that it can do nothing — rather than about Syndra not having asked.
  it("shows no operation count for a target that has never answered", () => {
    state.targets = [target({ callable: false, operations: [] })];
    render(<ConnectedSystemsPage />);

    expect(screen.getByText("no manifest yet")).toBeTruthy();
    expect(screen.queryByText("0")).toBeNull();
    expect(screen.getByText("—")).toBeTruthy();
  });

  // A secret that will not load is a fault on THIS side. Reporting it as the
  // target being unreachable sends an operator to the wrong machine.
  it("separates a transport fault from a target that is merely quiet", () => {
    state.targets = [target({ transport_status: "error", transport_error: "no such file" })];
    render(<ConnectedSystemsPage />);

    expect(screen.getByText("transport failed")).toBeTruthy();
    expect(screen.queryByText("answering")).toBeNull();
  });

  it("reports a suspended breaker as its own state, not as health", () => {
    state.targets = [target({ circuit_open: true })];
    render(<ConnectedSystemsPage />);

    expect(screen.getByText("calls suspended")).toBeTruthy();
    expect(screen.queryByText("answering")).toBeNull();
  });
});
