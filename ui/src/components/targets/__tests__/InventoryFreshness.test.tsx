// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { copyLine, cooldownLine } from "@/components/results/copy";
import { TargetOverview } from "@/components/targets/TargetOverview";
import { formatRelative } from "@/lib/format";
import type { Answered } from "@/lib/queries/read";
import type { TargetInventory } from "@/lib/queries/useTargets";
import { asRead } from "@/test-utils/wire";

/** The wait is said once in its own status region too; these read the line on screen. */
const STATUS = '[role="status"]';

/**
 * The account list on a target's page, in each state its read can be in
 * (design §13.6): the add-on's kept copy (Read again is the next step), a copy
 * kept inside the cooldown, and nothing to give at all (`none`), which has no
 * list, no count and no date.
 */

const READ = new Date(Date.now() - 3 * 60_000).toISOString();

const state: { read: Answered<TargetInventory> | undefined; press: ReturnType<typeof vi.fn> } = {
  read: undefined,
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
    useTargetHealthAgain: () => ({ press: async () => {} }),
    useTargetHealth: idle,
    useTargetInventory: () => ({ ...idle(), data: state.read }),
    useTargetInventoryAgain: () => ({ press: state.press, pending: false, error: null }),
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

const list = { target: "truenas", bound: 1, unmanaged: [{ username: "ci-runner", self: false }] };
const kept = (why: Record<string, unknown> = { reason: "cached" }, since?: number) => {
  const read = asRead({ ...list, read: { state: "kept" as const, taken_at: READ, ...why } });
  return read.state === "kept" && since ? { ...read, why: { ...read.why, since } } : read;
};
const none = (why: Record<string, unknown>, since?: number): Answered<TargetInventory> => ({
  state: "none",
  why: { reason: "addon_unreachable", ...why, ...(since ? { since } : {}) } as never,
});

function renderTarget() {
  return render(
    <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
      <TargetOverview target="truenas" />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  state.read = undefined;
  state.press = vi.fn(async () => {});
});
afterEach(() => vi.useRealTimers());

describe("the account list served from the add-on's copy", () => {
  it("is dated, offers Read again, presses it once, and keeps Assign off until it is current", () => {
    state.read = kept();
    renderTarget();
    expect(screen.getByText(copyLine("The account list", formatRelative(READ)))).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Assign to a person" })).toBeDisabled();
    fireEvent.click(screen.getByRole("button", { name: "Read again" }));
    expect(state.press).toHaveBeenCalledTimes(1);
  });

  it("allows Assign off a read taken now", () => {
    state.read = asRead({ ...list, read: { state: "now", taken_at: new Date().toISOString() } });
    renderTarget();
    expect(screen.getByRole("button", { name: "Assign to a person" })).toBeEnabled();
  });
});

// A press inside the cooldown with a copy is answered 200 from the copy (design §13.3), the wait in it.
describe("a copy kept inside the cooldown", () => {
  const dot = () => document.querySelector("[data-freshness-dot]") as HTMLElement;

  it("keeps the copy on show, counts down, and gives the button back at 0", () => {
    vi.useFakeTimers();
    state.read = kept({ reason: "cooldown", retry_after: 2 }, Date.now());
    renderTarget();
    expect(screen.getByText(cooldownLine("The account list", 2, formatRelative(READ)), { ignore: STATUS })).toBeInTheDocument();
    expect(screen.getByText("ci-runner")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Read again" })).toHaveAttribute("aria-disabled", "true");
    act(() => void vi.advanceTimersByTime(2000));
    expect(screen.getByRole("button", { name: "Read again" })).not.toHaveAttribute("aria-disabled");
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("is the reader's own press, so it is amber", () => {
    state.read = kept({ reason: "cooldown", retry_after: 30 }, Date.now());
    renderTarget();
    expect(dot().className).toContain("bg-warn");
  });

  it("counts from when the copy arrived, so leaving and returning does not restart it", () => {
    vi.useFakeTimers();
    state.read = kept({ reason: "cooldown", retry_after: 30 }, Date.now());
    renderTarget().unmount();
    act(() => void vi.advanceTimersByTime(12_000));
    renderTarget();
    expect(screen.getByText(cooldownLine("The account list", 18, formatRelative(READ)), { ignore: STATUS })).toBeInTheDocument();
  });
});

// Nothing to give (503 TARGET_NOT_READ): no list, no count, no date, and the why said once, above the cards.
describe("an account list with nothing to give", () => {
  it("inside the cooldown says the one wait wording, once, and the button waits", () => {
    vi.useFakeTimers();
    state.read = none({ reason: "cooldown", retryAfter: 9 }, Date.now());
    renderTarget();
    expect(screen.getByText(/^Truenas was read moments ago; you can read again in 9 s$/i, { ignore: STATUS })).toBeInTheDocument();
    expect(screen.queryByText(/could not be read/)).toBeNull();
    expect(screen.queryByText("Nothing outside Syndra.")).toBeNull();
    expect(screen.getByRole("button", { name: "Read again" })).toHaveAttribute("aria-disabled", "true");
  });

  it("says the add-on's own sentence and next step as written, and never an empty list", () => {
    const detail = "Syndra's records of who has which account on TrueNAS can't be read. Check the add-on's storage, then try again.";
    state.read = none({ reason: "read_refused", detail });
    renderTarget();
    expect(screen.getByText(`The account list could not be read. ${detail}`)).toBeInTheDocument();
    expect(screen.queryByText("Nothing outside Syndra.")).toBeNull();
    expect(screen.queryByText("read_refused")).toBeNull();
  });

  it("says which side is down, and what to check, in §13.1's words", () => {
    state.read = none({ cause: "wrong_host" });
    renderTarget();
    expect(screen.getByText(/^The account list could not be read\. Something answered at Truenas's add-on address/i)).toBeInTheDocument();
  });

  it("never shows a reason's code, and still says a next step", () => {
    state.read = none({ reason: "some_new_reason" });
    renderTarget();
    expect(screen.getByText(/^The account list could not be read\. Truenas's add-on didn't take this\. Read the add-on's log\.$/i)).toBeInTheDocument();
    expect(screen.queryByText(/some_new_reason/)).toBeNull();
  });

  // Seen on staging with the add-on stopped: a list never read showed a count of 0 and "This is the
  // last state seen, 1 Jan 1". Nothing was read, so there is no count, no date, and it is said once.
  it("shows no count and no date, and says it was not read once", () => {
    state.read = none({});
    renderTarget();
    expect(screen.getByText("Not created by Syndra").parentElement!.querySelector("[data-testid=card-count]")).toBeNull();
    expect(screen.getAllByText(/could not be read/)).toHaveLength(1);
    expect(document.body.textContent).not.toMatch(/\b1 Jan\b|0001/);
  });

  it("says nothing of a stop for a read taken now", () => {
    state.read = asRead({ ...list, read: { state: "now", taken_at: new Date().toISOString() } });
    renderTarget();
    expect(screen.queryByText(/The account list could not be read/)).toBeNull();
  });
});
