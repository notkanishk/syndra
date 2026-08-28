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
    expect(screen.queryByText(/connected$/)).toBeNull();
  });

  it("lists a registered target and what it can do", () => {
    state.targets = [target()];
    render(<ConnectedSystemsPage />);

    expect(screen.getByText("TrueNAS")).toBeTruthy();
    expect(screen.getByText("1 system connected")).toBeTruthy();
    expect(screen.getByText("Answering")).toBeTruthy();
    expect(screen.getByText("2")).toBeTruthy();
  });

  // Registration is a deployment fact and a manifest is a runtime one. Showing
  // `0` for a target that has never answered would be a claim about the TARGET
  // — that it can do nothing — rather than about Syndra not having asked.
  it("shows no operation count for a target that has never answered", () => {
    state.targets = [target({ callable: false, operations: [] })];
    render(<ConnectedSystemsPage />);

    expect(screen.getByText("Not answered yet")).toBeTruthy();
    expect(screen.queryByText("0")).toBeNull();
    expect(screen.getByText("—")).toBeTruthy();
  });

  // A secret that will not load is a fault on THIS side. Reporting it as the
  // target being unreachable sends an operator to the wrong machine.
  it("separates a transport fault from a target that is merely quiet", () => {
    state.targets = [target({ transport_status: "error", transport_error: "no such file" })];
    render(<ConnectedSystemsPage />);

    expect(screen.getByText("Cannot connect")).toBeTruthy();
    expect(screen.queryByText("Answering")).toBeNull();
  });

  it("reports a suspended breaker as its own state, not as health", () => {
    state.targets = [target({ circuit_open: true })];
    render(<ConnectedSystemsPage />);

    expect(screen.getByText("Paused after failures")).toBeTruthy();
    expect(screen.queryByText("Answering")).toBeNull();
  });
});

/**
 * The tone on "registered, no manifest read yet".
 *
 * It shipped amber, and amber in this system is a deadline or a broken
 * assumption. This is neither: it is where every add-on starts, and it resolves
 * on its own within a refresh interval. Spending amber on the ordinary first
 * minute of something's life is how amber stops meaning anything on the screens
 * where it does — the unrecorded key expiry two cards up, for one.
 */
describe("the tone a reading wears", () => {
  const toneOf = (label: string) =>
    screen.getByText(label).className.match(/text-(muted|warn-text|danger-text|healthy)/)?.[1];

  it("does not spend amber on a target that has simply not answered yet", () => {
    state.targets = [target({ callable: false, operations: [] })];
    render(<ConnectedSystemsPage />);

    expect(toneOf("Not answered yet")).toBe("muted");
  });

  it("keeps amber for the reading that is a real fault on this host", () => {
    state.targets = [target({ transport_status: "error", transport_error: "no such file" })];
    render(<ConnectedSystemsPage />);

    expect(toneOf("Cannot connect")).toBe("danger-text");
  });

  it("keeps amber for a breaker somebody has to act on", () => {
    state.targets = [target({ circuit_open: true })];
    render(<ConnectedSystemsPage />);

    expect(toneOf("Paused after failures")).toBe("warn-text");
  });
});
