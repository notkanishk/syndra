// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { copyLine, cooldownLine } from "@/components/results/copy";
import { TargetOverview } from "@/components/targets/TargetOverview";
import { ApiError } from "@/lib/api-client";
import { cooldownOf } from "@/lib/queries/fresh";
import { formatRelative } from "@/lib/format";

/** The wait is said once in its own status region too; these read the line on screen. */
const STATUS = '[role="status"]';

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
    useTargetInventoryAgain: () => {
      const cooldown = cooldownOf(state.again.error, state.again.errorAt);
      return { press: state.press, pending: false, error: state.again.error, errorAt: state.again.errorAt, cooldown };
    },
    useResolveLogFinding: inert,
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
    expect(screen.getByText(cooldownLine("The account list", 2), { ignore: STATUS })).toBeInTheDocument();
    expect(screen.getByText("ci-runner")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Read again" })).toHaveAttribute("aria-disabled", "true");
    act(() => void vi.advanceTimersByTime(2000));
    expect(screen.getByRole("button", { name: "Read again" })).not.toHaveAttribute("aria-disabled");
    expect(screen.queryByRole("alert")).toBeNull();
  });

  // A load never sends fresh, so a cooldown only ever comes from a press; with nothing to show, the press says it.
  it("with nothing to show, says so once, in the freshness line, never as a read that failed or a second countdown", () => {
    vi.useFakeTimers();
    state.inventory = {};
    state.again = { error: cooling(9) };
    renderTarget();
    expect(screen.getByText(cooldownLine("The account list", 9), { ignore: STATUS })).toBeInTheDocument();
    expect(screen.queryByText(/nothing was changed/)).toBeNull();
    expect(screen.queryByText(/You can try again in/)).toBeNull();
    expect(screen.queryByText("The account list could not be read")).toBeNull();
    expect(screen.queryByText("Nothing outside Syndra.")).toBeNull();
    expect(screen.getByRole("button", { name: "Read again" })).toHaveAttribute("aria-disabled", "true");
  });

  // Only a wait the reader pressed into is a warning, and on this card a press is the only way into one.
  it("draws the refusal of the reader's own press amber, with a copy on show or after a load that failed", () => {
    const dot = () => document.querySelector("[data-freshness-dot]") as HTMLElement;
    state.inventory = copyOf();
    state.again = { error: cooling(9) };
    const mine = renderTarget();
    expect(dot().className).toContain("bg-warn");
    mine.unmount();
    // The load failed (not a cooldown) and nothing is on show: the press's refusal is still said, and counts down.
    state.inventory = { error: new ApiError(503, { error: "TARGET_UNREACHABLE" }) };
    state.again = { error: cooling(9) };
    renderTarget();
    expect(dot().className).toContain("bg-warn");
    expect(screen.getByRole("button", { name: "Read again" })).toHaveAttribute("aria-disabled", "true");
  });
});

// A press answered 200 carries its cooldown in the data (read_stale), and a press is the only way it gets there.
describe("a cooldown carried in the account list's data", () => {
  const dot = () => document.querySelector("[data-freshness-dot]") as HTMLElement;
  const cooled = (since?: number) => ({ ...copyOf({ read_stale: { reason: "cooldown", retry_after: 30 } }), dataUpdatedAt: since });

  it("is the reader's own press, so it is amber", () => {
    state.inventory = cooled();
    renderTarget();
    expect(dot().className).toContain("bg-warn");
  });

  it("counts from when the copy arrived, so leaving and returning does not restart it", () => {
    vi.useFakeTimers();
    const arrived = Date.now();
    state.inventory = cooled(arrived);
    renderTarget().unmount();
    act(() => void vi.advanceTimersByTime(12_000));
    renderTarget();
    expect(screen.getByText(cooldownLine("The account list", 18), { ignore: STATUS })).toBeInTheDocument();
  });
});

// 7t-3 left the add-on's sentence on the inventory result; a stopped pass has an empty list it
// never measured, so the card says why instead of "Nothing outside Syndra."
describe("an account list whose pass stopped", () => {
  const stopped = (over: Record<string, unknown>) =>
    (state.inventory = { data: { target: "truenas", current: false, halted: true, ...over } });

  it("says the add-on's own sentence and next step as written, and never an empty list", () => {
    const detail = "Syndra's records of who has which account on TrueNAS can't be read. Check the add-on's storage, then try again.";
    stopped({ reason: "read_refused", detail });
    renderTarget();
    expect(screen.getByText(`The account list could not be read. ${detail}`)).toBeInTheDocument();
    expect(screen.queryByText("Nothing outside Syndra.")).toBeNull();
    expect(screen.queryByText("read_refused")).toBeNull();
  });

  it("words the reason and ends in a step when the add-on said nothing", () => {
    stopped({ reason: "target_unreachable" });
    renderTarget();
    // The system is the one that did not answer, not the list.
    expect(
      screen.getByText(/^The account list could not be read\. Truenas did not answer when Syndra last tried\. Read again once that is fixed\.$/i),
    ).toBeInTheDocument();
  });

  it("never shows a reason's code, and still says a next step", () => {
    stopped({ reason: "some_new_reason" });
    renderTarget();
    expect(
      screen.getByText(/^The account list could not be read\. Truenas could not be checked, and Syndra has no words for why\. Read again once that is fixed\.$/i),
    ).toBeInTheDocument();
    expect(screen.queryByText(/some_new_reason/)).toBeNull();
  });

  // Seen on staging with the add-on stopped: a 503 whose list was never read showed a count of 0 and
  // "This is the last state seen, 1 Jan 1". Nothing was read, so there is no count, no date, and the
  // card says it once.
  it("shows no count and no date, and says it was not read once", () => {
    stopped({ reason: "target_unreachable", read_at: "0001-01-01T00:00:00Z", queued: 0 });
    renderTarget();
    expect(screen.getByText("Not created by Syndra").parentElement!.querySelector("[data-testid=card-count]")).toBeNull();
    expect(screen.getAllByText(/could not be read|Could not reach/)).toHaveLength(1);
    expect(document.body.textContent).not.toMatch(/\b1 Jan\b|0001/);
  });

  it("says nothing of a stop for a pass that concluded", () => {
    state.inventory = copyOf({ halted: false, current: true, reason: undefined, read_stale: undefined });
    renderTarget();
    expect(screen.queryByText(/The account list could not be read/)).toBeNull();
  });
});
