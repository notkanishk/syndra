// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { TargetOverview } from "@/components/targets/TargetOverview";
import { ApiError } from "@/lib/api-client";
import { LONG_WAIT_MS } from "@/lib/useLongWait";

/**
 * A pass that concluded nothing must not report three zeroes.
 *
 * `halt()` in the add-on reconciler returns the result struct untouched —
 * `bound`, `queued` and `stale` are still at their zero values because nothing
 * was ever measured. The card rendered them anyway: "0 accounts managed · 0
 * fixes waiting for you to send · 0 accounts missing", which is what a healthy,
 * fully-converged system also looks like. The two states that most need
 * telling apart were displayed identically, and the flag that distinguishes
 * them was missing from the UI's own type.
 */

vi.mock("@/lib/queries/useTargetSystemHealth", () => ({
  useTargetSystemHealth: () => ({
    data: undefined,
    isLoading: false,
    error: null,
  }),
}));

// The pass the card shows; a test may swap it before rendering.
const pass = vi.hoisted(() => ({
  pending: false,
  data: {
    target: "truenas",
    queued: 0,
    read: { state: "now", taken_at: new Date().toISOString() },
    halted: true,
    reason: "a_reason_nobody_worded",
  } as Record<string, unknown> | undefined,
  error: null as unknown,
}));

// Every helper lives INSIDE the factory: `vi.mock` is hoisted above every
// const in the file, so a top-level one is not yet initialised when it runs.
vi.mock("@/lib/queries/useTargets", async () => {
  const { withRead } = await import("@/lib/queries/read");
  const idle = () => ({
    data: undefined,
    isLoading: false,
    isError: false,
    error: null,
    refetch: vi.fn(),
  });
  const inert = () => ({
    mutate: vi.fn(),
    mutateAsync: vi.fn(),
    isPending: false,
    error: null,
    data: undefined,
    reset: vi.fn(),
  });
  return {
    useTargets: () => ({ ...idle(), data: [] }),
    useUnconfirmedOperations: () => ({ data: [], isLoading: false, error: null }),
    useMergeFindings: () => ({ ...idle(), data: [] }),
    useTargetHealthAgain: () => ({ press: async () => {} }),
    useTargetHealth: idle,
    useTargetInventory: idle,
    useTargetInventoryAgain: () => ({ press: async () => {}, pending: false, error: null }),
    useResolveLogFinding: inert,
    useSetLifecycle: inert,
    useReleaseBinding: inert,
    useAdoptAccount: inert,
    useReconcileTarget: () => ({ ...inert(), data: pass.data && withRead(pass.data as never), error: pass.error, isPending: pass.pending }),
  };
});

vi.mock("@/components/targets/MappingManagement", () => ({
  MappingManagement: () => null,
}));
vi.mock("@/components/targets/PeopleOnTarget", () => ({
  PeopleOnTarget: () => null,
}));
vi.mock("@/components/targets/DormantAccounts", () => ({
  DormantAccounts: () => null,
}));
vi.mock("@/components/targets/MergeFindings", () => ({
  MergeFindings: () => null,
}));

function renderTarget() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={client}>
      <TargetOverview target="truenas" />
    </QueryClientProvider>,
  );
}

describe("a reconcile pass that halted", () => {
  it("says nothing was concluded, and does not report its unmeasured zeroes", () => {
    renderTarget();

    expect(
      screen.getByText(/stopped before it concluded anything/i),
    ).toBeInTheDocument();
    expect(screen.getAllByText(/Syndra has no words for why/i).length).toBeGreaterThan(
      0,
    );

    // The exact sentence a healthy run produces. A halted pass must never be
    // able to render it.
    expect(screen.queryByText(/0 people with a managed account/)).toBeNull();
    expect(screen.queryByText(/fixes waiting for you to send/)).toBeNull();
  });

  // Nothing to read (design §13.3): a 503 TARGET_NOT_READ, said in the backend's §13.1 sentence,
  // which names the side that is down and its step, never as zeroes or as Syndra's own failure.
  const notRead = (reason: string, message: string) =>
    new ApiError(503, { error: "TARGET_NOT_READ", message, details: { reason } });

  it("says the add-on's own words when it refused the read", () => {
    const detail =
      "Syndra's records of who has which account on TrueNAS can't be read. Check the add-on's storage, then try again.";
    pass.data = undefined;
    pass.error = notRead("read_refused", detail);
    renderTarget();

    expect(screen.getByText(`The check stopped before it concluded anything. ${detail}`)).toBeInTheDocument();
    expect(screen.queryByText(/read_refused/)).toBeNull();
    expect(screen.queryByText(/ask the person who runs the Syndra server/)).toBeNull();
    expect(screen.queryByText(/fixes waiting for you to send/)).toBeNull();
  });

  it("says the add-on is outdated and to update it", () => {
    pass.data = undefined;
    pass.error = notRead("addon_outdated", "TrueNAS's add-on doesn't offer this. Update the add-on.");
    renderTarget();

    expect(screen.getByText("The check stopped before it concluded anything. TrueNAS's add-on doesn't offer this. Update the add-on.")).toBeInTheDocument();
    expect(screen.queryByText(/addon_outdated/)).toBeNull();
  });
});

// A-24 §6: past ten seconds a check says so, once. Never that its result waits on return:
// it is a mutation's answer, gone if the page is left (A-24 §13).
describe("a check that takes long", () => {
  afterEach(() => {
    pass.pending = false;
    vi.useRealTimers();
  });

  it("says it is still reading, only after ten seconds, and never that you can leave", () => {
    vi.useFakeTimers();
    pass.pending = true;
    renderTarget();
    act(() => void vi.advanceTimersByTime(LONG_WAIT_MS - 1));
    expect(screen.queryByText(/^Still reading /)).toBeNull();
    act(() => void vi.advanceTimersByTime(1));
    expect(screen.getByText(/^Still reading .+\.$/)).toBeInTheDocument();
    expect(screen.queryByText(/You can leave this page/)).toBeNull();
  });
});
