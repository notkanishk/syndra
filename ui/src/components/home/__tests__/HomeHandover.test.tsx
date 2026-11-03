// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { Home } from "@/components/home/Home";
import { TINT_MS } from "@/lib/motion";
import type { SessionUser } from "@/lib/session";
import { setMediaQuery } from "@/test-utils/media";
import { motionInTests } from "@/test-utils/motion";

// A-12 §1: Home's calm line and its grid hand over by Unfold whatever caused the swap.
// The leaving one fades and folds at once, the arriving one opens one tint later, and
// what caused it decides only when it starts.

const state = vi.hoisted(() => ({
  advanced: true,
  governance: {} as Record<string, unknown>,
}));

const quiet = () => ({
  pending_requests: [],
  expiring_grants: [] as unknown[],
  cleanup_hints: [],
  pending_propagation: { count: 0, zitadel_reachable: true },
  drift: { count: 0, top: [] },
  unreconciled_targets: [] as unknown[],
  merge_findings: 0,
  unconfirmed_writes: { count: 0, top: [] },
});

vi.mock("@/lib/queries/useServiceAccounts", () => ({
  useServiceAccounts: () => ({ data: [], isLoading: false, error: null, refetch: vi.fn() }),
  serviceAccountHref: (id: string) => `/system/service-accounts/${id}`,
}));
vi.mock("@/lib/queries/useGovernance", () => ({
  useGovernanceSummary: () => ({ data: state.governance, isLoading: false, error: null, refetch: vi.fn() }),
}));
vi.mock("@/lib/queries/useRequests", () => ({
  useRequestsAdmin: () => ({ data: [], isLoading: false, error: null, refetch: vi.fn() }),
  useDecideRequest: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));
vi.mock("@/lib/queries/usePropagation", () => ({
  useDrainPropagations: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));
vi.mock("@/lib/queries/useUsers", () => ({
  useCreateGrant: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useUserAccess: () => ({ data: undefined }),
}));
vi.mock("@/lib/queries/useTargets", () => ({
  useTargets: () => ({ data: [{ target: "truenas", registered: true }], isLoading: false }),
}));
vi.mock("@/lib/queries/useOperations", () => ({
  useMissedOnboarding: () => ({
    data: { welcome_bundle_configured: true, missed: [] },
    isLoading: false,
    error: null,
    refetch: vi.fn(),
  }),
}));
vi.mock("@/lib/queries/useIndicators", () => ({
  useIndicatorsRead: () => ({ data: { unconfirmed_revocations: 0, holds_due: 0 }, isPlaceholderData: false }),
}));
vi.mock("@/lib/ui-view", () => ({ useIsAdvanced: () => state.advanced }));
vi.mock("@/components/home/Makerspace", () => ({ Makerspace: () => <div /> }));
vi.mock("@/components/requests/Holding", () => ({ HoldingNote: () => null, alreadyHeld: () => false }));

const session = { id: "op", name: "Kanishk", email: "k@example.org" } as unknown as SessionUser;
const client = () => new QueryClient({ defaultOptions: { queries: { retry: false } } });

function mount() {
  const queries = client();
  const home = () => (
    <QueryClientProvider client={queries}>
      <Home session={session} />
    </QueryClientProvider>
  );
  const view = render(home());
  return { reread: () => view.rerender(home()) };
}

/** The phase of the outermost Unfold around `element`: the calm line's, or the grid's. */
function phaseOf(element: Element | null): string | null {
  let region = element?.closest(".unfold") ?? null;
  for (let up = region?.parentElement?.closest(".unfold"); up; up = up.parentElement?.closest(".unfold")) region = up;
  return region?.getAttribute("data-phase") ?? null;
}
const calmPhase = () => phaseOf(screen.queryByText("Nothing here needs you."));
const gridPhase = () => phaseOf(screen.queryByRole("region", { name: "Time-sensitive" }));
const later = (ms: number) => act(() => vi.advanceTimersByTime(ms));

const drifting = () => ({ ...quiet(), drift: { count: 3, top: [] } });

beforeEach(() => {
  state.advanced = true;
  state.governance = drifting();
  Object.defineProperty(Element.prototype, "getAnimations", { value: () => [], configurable: true });
  vi.useFakeTimers();
  motionInTests();
});
afterEach(() => {
  vi.useRealTimers();
  delete (Element.prototype as { getAnimations?: unknown }).getAnimations;
});

describe("Home's calm line and grid hand over", () => {
  it("are simply there on the first read: nothing hands over as the page arrives", () => {
    mount();
    expect(gridPhase()).toBe("open");
    expect(calmPhase()).toBeNull();
  });

  // The 120ms overlap A-12 "For code" names: the calm line opened while the last block still showed.
  it("a read that empties Home folds the grid first and opens the calm line one tint later", () => {
    const home = mount();
    state.governance = quiet();
    home.reread();

    expect(gridPhase()).toBe("closing");
    expect(calmPhase()).toBeNull();
    later(TINT_MS - 1);
    expect(calmPhase()).toBeNull();
    later(1);
    expect(calmPhase()).toBe("opening");
  });

  it("the first item on a calm Home plays on the read, the calm line leaving first", () => {
    state.governance = quiet();
    const home = mount();
    expect(calmPhase()).toBe("open");

    state.governance = drifting();
    home.reread();
    expect(calmPhase()).toBe("closing");
    expect(gridPhase()).toBeNull();
    later(TINT_MS);
    expect(gridPhase()).toBe("opening");
  });

  // S5: someone else's change never moves what is under the pointer.
  it("waits while the pointer is inside the blocks, showing them as they were, then plays once it leaves", () => {
    const home = mount();
    const column = screen.getByRole("region", { name: "Time-sensitive" });
    fireEvent.pointerOver(column);

    state.governance = quiet();
    home.reread();
    later(TINT_MS * 4);
    expect(gridPhase()).toBe("open");
    expect(screen.getByRole("link", { name: "Review them" })).toBeTruthy();
    expect(calmPhase()).toBeNull();
    // The headline stays with what is on screen.
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent("One thing needs you.");

    fireEvent.pointerOut(column, { relatedTarget: document.body });
    expect(gridPhase()).toBe("closing");
    later(TINT_MS);
    expect(calmPhase()).toBe("opening");
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent("Nothing needs you right now.");
  });

  it("waits while focus is inside the blocks, too", () => {
    const home = mount();
    const link = screen.getByRole("region", { name: "Time-sensitive" }).querySelector("a")!;
    fireEvent.focus(link);

    state.governance = quiet();
    home.reread();
    expect(gridPhase()).toBe("open");

    fireEvent.blur(link, { relatedTarget: document.body });
    expect(gridPhase()).toBe("closing");
  });

  it("the view switch swaps at once, even with the pointer inside, because you asked", () => {
    const home = mount();
    fireEvent.pointerOver(screen.getByRole("region", { name: "Time-sensitive" }));

    state.advanced = false;
    home.reread();
    expect(gridPhase()).toBe("closing");
    later(TINT_MS);
    expect(calmPhase()).toBe("opening");
  });

  it("'Checked, nothing waiting:' goes with the grid, by the same Unfold", () => {
    const home = mount();
    const line = screen.getByText(/Checked, nothing waiting/);
    expect(line.closest(".unfold")).toHaveAttribute("data-phase", "open");

    state.governance = quiet();
    home.reread();
    expect(line.closest(".unfold")).toHaveAttribute("data-phase", "closing");
  });

  it("the headline's words crossfade in place (S4)", () => {
    const home = mount();
    state.governance = quiet();
    home.reread();
    const words = screen.getByText("Nothing needs you right now.");
    expect(words).toHaveClass("tint-in");
    expect(screen.getByText("One thing needs you.")).toHaveClass("tint-out");
  });

  it("under reduced motion both parts change at once", () => {
    setMediaQuery("(prefers-reduced-motion: reduce)", true);
    const home = mount();
    state.governance = quiet();
    home.reread();
    expect(calmPhase()).toBe("open");
    expect(gridPhase()).toBeNull();
  });
});
