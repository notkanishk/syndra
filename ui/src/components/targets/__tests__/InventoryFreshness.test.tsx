// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { copyLine, cooldownLine } from "@/components/results/copy";
import { TargetOverview } from "@/components/targets/TargetOverview";
import { ApiError } from "@/lib/api-client";
import { formatRelative } from "@/lib/format";

/**
 * The account list on a target's page, in each freshness the add-on can answer:
 * its copy (read again is the next step), a Read again refused inside the
 * cooldown (a 429, with or without a copy to keep showing).
 */

const READ = new Date(Date.now() - 3 * 60_000).toISOString();
const cooling = (wait: number) => new ApiError(429, { error: "FRESH_READ_COOLDOWN", retry_after: wait });

const state: { inventory: Record<string, unknown>; again: { error: unknown; errorAt?: number }; press: ReturnType<typeof vi.fn> } = {
  inventory: {},
  again: { error: null },
  press: vi.fn(),
};

vi.mock("@/lib/queries/useTargetSystemHealth", () => ({
  useTargetSystemHealth: () => ({ data: undefined, isLoading: false, error: null }),
}));
vi.mock("@/lib/queries/useTargets", () => {
  const idle = () => ({ data: undefined, isLoading: false, isError: false, error: null, refetch: vi.fn() });
  const inert = () => ({ mutate: vi.fn(), mutateAsync: vi.fn(), isPending: false, error: null, data: undefined, reset: vi.fn() });
  return {
    useTargets: () => ({ ...idle(), data: [] }),
    useUnconfirmedOperations: () => ({ data: [], isLoading: false, error: null }),
    useMergeFindings: () => ({ ...idle(), data: [] }),
    useTargetHealth: () => ({ ...idle(), data: { reachable: true, lifecycle: "active" } }),
    useTargetInventory: () => ({ ...idle(), refetch: vi.fn(), ...state.inventory }),
    useTargetInventoryAgain: () => ({ press: state.press, pending: false, error: state.again.error, errorAt: state.again.errorAt }),
    useResolveLogFinding: inert,
    useResolveBindingConflict: inert,
    useReconcileTarget: inert,
    useSetLifecycle: inert,
    useReleaseBinding: inert,
    useAdoptAccount: inert,
  };
});
vi.mock("@/components/targets/MappingManagement", () => ({ MappingManagement: () => null }));
vi.mock("@/components/targets/PeopleOnTarget", () => ({ PeopleOnTarget: () => null }));
vi.mock("@/components/targets/DormantAccounts", () => ({ DormantAccounts: () => null }));
vi.mock("@/components/targets/MergeFindings", () => ({ MergeFindings: () => null }));

const copyOf = (over: Record<string, unknown> = {}) => ({
  data: {
    target: "truenas",
    bound: 1,
    unmanaged: [{ username: "ci-runner", self: false }],
    read_at: READ,
    current: false,
    reason: "read_stale",
    read_stale: { reason: "cached" },
    ...over,
  },
});

function renderTarget() {
  return render(
    <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
      <TargetOverview target="truenas" />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  state.inventory = {};
  state.again = { error: null };
  state.press = vi.fn(async () => {});
});
afterEach(() => vi.useRealTimers());

describe("the account list served from the add-on's copy", () => {
  it("is dated, offers Read again, presses it once, and keeps Assign off until it is current", () => {
    state.inventory = copyOf();
    renderTarget();
    expect(screen.getByText(copyLine("The account list", formatRelative(READ)))).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Assign to a person" })).toBeDisabled();
    fireEvent.click(screen.getByRole("button", { name: "Read again" }));
    expect(state.press).toHaveBeenCalledTimes(1);
  });
});

describe("a Read again the add-on refused inside its cooldown", () => {
  it("keeps the copy on show, counts down, and gives the button back at 0", () => {
    vi.useFakeTimers();
    state.inventory = copyOf();
    state.again = { error: cooling(2) };
    renderTarget();
    expect(screen.getByText(cooldownLine("The account list", 2))).toBeInTheDocument();
    expect(screen.getByText("ci-runner")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Read again" })).toBeDisabled();
    act(() => void vi.advanceTimersByTime(2000));
    expect(screen.getByRole("button", { name: "Read again" })).toBeEnabled();
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("with nothing to show, says so once, in the freshness line, never as a read that failed or a second countdown", () => {
    vi.useFakeTimers();
    state.inventory = { error: cooling(9) };
    renderTarget();
    expect(screen.getByText(cooldownLine("The account list", 9))).toBeInTheDocument();
    expect(screen.queryByText(/nothing was changed/)).toBeNull();
    expect(screen.queryByText(/You can try again in/)).toBeNull();
    expect(screen.queryByText("The account list could not be read")).toBeNull();
    expect(screen.queryByText("Nothing outside Syndra.")).toBeNull();
    expect(screen.getByRole("button", { name: "Read again" })).toBeDisabled();
  });
});
