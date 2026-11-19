// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { Home } from "@/components/home/Home";
import { TINT_MS } from "@/lib/motion";
import { noteSignIn, resetDoorForTests } from "@/lib/motion/door";
import { inZone } from "@/test-utils/zone";
import type { SessionUser } from "@/lib/session";
import { setMediaQuery } from "@/test-utils/media";
import { motionInTests } from "@/test-utils/motion";

// A-12 §1: Home's calm line and its grid hand over by Unfold whatever caused the swap.
// The leaving one fades and folds at once, the arriving one opens one tint later, and
// what caused it decides only when it starts.

const state = vi.hoisted(() => ({
  advanced: true,
  // Whether a write of the reader's own landed in the last 30s (lib/query-client).
  own: false,
  terms: [] as Array<{ name: string; start: string; end: string }> | undefined,
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
vi.mock("@/lib/ui-view", () => ({
  useIsAdvanced: () => state.advanced,
  useUiView: () => ({ view: "basic", setView: () => {} }),
}));
vi.mock("@/lib/queries/useTerms", () => ({ useTerms: () => ({ data: state.terms }) }));
vi.mock("@/lib/query-client", async (original) => ({
  ...(await original<typeof import("@/lib/query-client")>()),
  ownActWithin: () => state.own,
}));
vi.mock("@/components/home/Makerspace", () => ({ Makerspace: () => <div /> }));
vi.mock("@/components/requests/Holding", () => ({ HoldingNote: () => null, alreadyHeld: () => false, ApproveButton: ({ onClick }: { onClick: () => void }) => <button type="button" onClick={onClick}>Approve</button> }));

const session = { id: "op", name: "Kanishk", email: "k@example.org" } as unknown as SessionUser;
const client = () => new QueryClient({ defaultOptions: { queries: { retry: false } } });

function mount(who: SessionUser = session) {
  const queries = client();
  const home = () => (
    <QueryClientProvider client={queries}>
      <Home session={who} />
    </QueryClientProvider>
  );
  const view = render(home());
  return { reread: () => view.rerender(home()), unmount: view.unmount };
}

/** The phase of the outermost Unfold around `element`: the calm line's, or the grid's. */
function phaseOf(element: Element | null): string | null {
  let region = element?.closest(".unfold") ?? null;
  for (let up = region?.parentElement?.closest(".unfold"); up; up = up.parentElement?.closest(".unfold")) region = up;
  return region?.getAttribute("data-phase") ?? null;
}
const calmPhase = () => phaseOf(screen.queryByText("Nothing here needs you."));
const gridPhase = () => phaseOf(screen.queryByRole("region", { name: "Time-sensitive", hidden: true })); // closing is inert and hidden
const later = (ms: number) => act(() => vi.advanceTimersByTime(ms));

const drifting = () => ({ ...quiet(), drift: { count: 3, top: [] } });

beforeEach(() => {
  state.advanced = true;
  state.own = false;
  state.terms = [];
  // The day's close is kept on this device: every test starts with none.
  window.localStorage.clear();
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
    expect(screen.getByRole("link", { name: "Review on Drift" }).closest("[data-action-seat]")).toHaveAttribute("data-seat", "narrow");
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

// A-24 §4.1: the ending of Home's queue. It lifts when Home empties in front of the reader,
// blooms when their own act emptied it, and closes (the vivid ending) once a day at most.
describe("Home's all-clear, as an ending", () => {
  const line = () => screen.getByText("Nothing here needs you.").closest('[role="status"]')!;
  const empty = () => {
    const home = mount();
    state.governance = quiet();
    home.reread();
    later(TINT_MS);
    return home;
  };

  it("is simply there on arrival: no lift, no bloom, no close", () => {
    state.governance = quiet();
    mount();
    expect(line()).toHaveClass("tint-in");
    expect(line().querySelector(".bloom")).toBeNull();
    expect(line().querySelector("[data-close-line]")).toBeNull();
  });

  it("emptied by a read, not by the reader: it lifts, and neither blooms nor closes", () => {
    empty();
    expect(line()).toHaveClass("lift-in");
    expect(line().querySelector(".bloom")).toBeNull();
    expect(line().querySelector("[data-close-line]")).toBeNull();
  });

  it("a switch to a view with less in it is not an emptying: no lift, no bloom, no close", () => {
    state.own = true;
    const home = mount(); // Advanced, with drift: Basic has no drift block
    state.advanced = false;
    home.reread();
    later(TINT_MS);
    expect(line()).not.toHaveClass("lift-in");
    expect(line().querySelector(".bloom")).toBeNull();
    expect(line().querySelector("[data-close-line]")).toBeNull();
  });

  it("emptied by the reader's own act: it blooms, the list closes, and only the first time today", () => {
    state.own = true;
    const home = empty();
    expect(line().querySelector(".bloom")).not.toBeNull();
    expect(line().querySelector("[data-close-line]")).toHaveClass("close-line");
    expect(screen.getByText("Nothing here needs you.")).toHaveClass("type-section-title");

    // Work comes in, and the reader clears it again the same day: it blooms, and does not close twice.
    state.governance = drifting();
    home.reread();
    later(TINT_MS);
    state.governance = quiet();
    home.reread();
    later(TINT_MS);
    expect(line().querySelector(".bloom")).not.toBeNull();
    expect(line().querySelector("[data-close-line]")).toBeNull();
  });

  it("closes again the next day", () => {
    state.own = true;
    const home = empty();
    expect(line().querySelector("[data-close-line]")).not.toBeNull();
    state.governance = drifting();
    home.reread();
    later(TINT_MS);

    vi.setSystemTime(Date.now() + 24 * 60 * 60 * 1000);
    state.governance = quiet();
    home.reread();
    later(TINT_MS);
    expect(line().querySelector("[data-close-line]")).not.toBeNull();
  });

  it("draws the closing title at its size from the first frame: no class changes after it is in", () => {
    // Under reduced motion the calm line is in the same frame Home empties, with no tint to hide a resize.
    setMediaQuery("(prefers-reduced-motion: reduce)", true);
    state.own = true;
    const changed: string[] = [];
    const watch = new MutationObserver((records) => {
      for (const record of records) if (record.target.textContent === "Nothing here needs you.") changed.push(String(record.attributeName));
    });
    watch.observe(document.body, { subtree: true, attributes: true, attributeFilter: ["class"] });
    empty();
    watch.takeRecords().forEach((record) => {
      if (record.target.textContent === "Nothing here needs you.") changed.push(String(record.attributeName));
    });
    watch.disconnect();
    expect(screen.getByText("Nothing here needs you.")).toHaveClass("type-section-title");
    expect(changed).toEqual([]);
  });
});

// A-24 §3.1, Q2: the hour's light, behind Home's head and nowhere else, on the makerspace's clock.
describe("the room knows the hour", () => {
  // The setup holds the zone from TZ for the whole file: each test that picks its own puts it back.
  afterEach(() => inZone(process.env.TZ));

  it("lights the head for the part of the day the greeting names, hidden from assistive technology", () => {
    inZone("UTC");
    vi.setSystemTime(new Date("2026-11-18T08:10:00Z"));
    mount();
    const light = document.querySelector(".hour-light")!;
    expect(light).toHaveAttribute("data-hour", "morning");
    expect(light).toHaveAttribute("aria-hidden");
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent(/^Morning/);
    // Behind the head: inside it, and the only light on the page.
    expect(light.parentElement).toHaveClass("page-head", "isolate");
    expect(document.querySelectorAll(".hour-light")).toHaveLength(1);
  });

  it("is a lamp in the quiet hours, with the greeting and the calm line's sentence from the same part of the day", () => {
    inZone("UTC");
    vi.setSystemTime(new Date("2026-11-18T23:40:00Z"));
    state.governance = quiet();
    mount();
    expect(document.querySelector(".hour-light")).toHaveAttribute("data-hour", "quiet");
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent(/^Evening/);
    expect(screen.getByText(/It will keep until morning\./)).toBeInTheDocument();
  });

  it("turns the greeting and the light together on a Home left open across a boundary", () => {
    inZone("UTC");
    vi.setSystemTime(new Date("2026-11-18T11:59:30Z"));
    mount();
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent(/^Morning/);
    later(60_000);
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent(/^Afternoon/);
    expect(document.querySelector(".hour-light")).toHaveAttribute("data-hour", "afternoon");
  });

  it("follows the makerspace's zone, not the browser's: 20:00Z is quiet hours in Kolkata", () => {
    inZone("Asia/Kolkata");
    vi.setSystemTime(new Date("2026-11-18T20:00:00Z"));
    mount();
    expect(document.querySelector(".hour-light")).toHaveAttribute("data-hour", "quiet");
  });
});

// A-24 §3.2, Q4: the door's light travels the greeting once, straight after signing in.
describe("the first arrival of a sign-in", () => {
  const signedIn = { ...session, signedInAt: "2026-11-19T08:00:00.000Z" } as SessionUser;
  const arch = () => document.querySelector("[data-door-light]");
  const pool = () => document.querySelector(".hour-light");
  beforeEach(() => {
    resetDoorForTests();
    window.history.replaceState({}, "", "/");
  });
  afterEach(() => vi.restoreAllMocks());

  it("travels the greeting behind its words, then settles into the pool, with nothing waiting for it", () => {
    mount(signedIn);
    const greeting = screen.getByRole("heading", { level: 1 });
    // Inside the greeting's own box, so it travels the words' length; hidden from assistive technology.
    expect(arch()).toHaveAttribute("aria-hidden");
    expect(arch()).toHaveClass("greeting-light");
    expect(arch()!.parentElement).toBe(greeting);
    expect(greeting).toHaveClass("relative", "w-fit");
    // Not a word of the greeting: the heading reads as before.
    expect(greeting).toHaveTextContent(/^\w+, Kanishk\./);
    // The pool comes up as it settles; the words are there in the same paint.
    expect(pool()).toHaveAttribute("data-door");
    expect(document.querySelectorAll(".hour-light")).toHaveLength(1);
  });

  it("is never a full-window layer", () => {
    mount(signedIn);
    expect(document.querySelector(".door-through, [data-door-through]")).toBeNull();
    expect(arch()!.closest(".page-head")).not.toBeNull();
  });

  it("is once: not when Home is shown again, not on a reload, not for the same sign-in", () => {
    const first = mount(signedIn);
    expect(arch()).not.toBeNull();
    first.unmount();
    mount(signedIn); // back to Home in the same visit
    expect(arch()).toBeNull();
    expect(pool()).not.toHaveAttribute("data-door");
    resetDoorForTests(); // a reload: this device remembers the sign-in it opened for
    mount(signedIn);
    expect(document.querySelectorAll("[data-door-light]")).toHaveLength(0);
  });

  it("is spent by whichever page the sign-in lands on, so Home later is quiet", () => {
    window.history.replaceState({}, "", "/users/u7");
    noteSignIn(signedIn.signedInAt);
    window.history.replaceState({}, "", "/");
    mount(signedIn);
    expect(arch()).toBeNull();
  });

  it("opens again for the next sign-in", () => {
    mount(signedIn).unmount();
    resetDoorForTests();
    mount({ ...signedIn, signedInAt: "2026-11-20T08:00:00.000Z" } as SessionUser);
    expect(arch()).not.toBeNull();
  });

  it("shows nothing extra when this device can't remember, rather than opening every time", () => {
    // A private window or blocked site data: the store itself can't be reached.
    vi.spyOn(window, "localStorage", "get").mockImplementation(() => {
      throw new Error("blocked");
    });
    mount(signedIn);
    expect(arch()).toBeNull();
    expect(pool()).not.toHaveAttribute("data-door");
  });

  it("has no door without a sign-in instant to keep it to", () => {
    mount(session);
    expect(arch()).toBeNull();
  });
});

// A-24 §3.1, Q3: the term line, from the terms setting and Ending soon's own read.
describe("the term line on Home", () => {
  const term = () => screen.queryByText(/^Monsoon term, day 110 of 153$/)?.closest("[data-term-line]");

  it("is there from the first paint when its reads are in, with Ending soon's days on its track", () => {
    vi.setSystemTime(new Date("2026-11-18T08:10:00Z"));
    state.terms = [{ name: "Monsoon", start: "2026-08-01", end: "2026-12-31" }];
    state.governance = { ...drifting(), expiring_grants: [{ id: "g1", user_id: "u1", project_id: "p", role_key: "r", expires_at: "2026-11-24T10:00:00Z" }] };
    mount();
    expect(screen.getByRole("img", { name: "Access ends for one person on 24 Nov." })).toBeInTheDocument();
    // Not opened into: nothing below it is pushed down on arrival.
    expect(term()?.closest(".unfold")).toHaveAttribute("data-phase", "open");
  });

  it("unfolds when the terms land late, rather than popping in, and is gone between terms", () => {
    vi.setSystemTime(new Date("2026-11-18T08:10:00Z"));
    state.terms = undefined;
    const home = mount();
    expect(term()).toBeFalsy();
    state.terms = [{ name: "Monsoon", start: "2026-08-01", end: "2026-12-31" }];
    home.reread();
    expect(term()?.closest(".unfold")).toHaveAttribute("data-phase", "opening");
    state.terms = [{ name: "Spring", start: "2027-01-10", end: "2027-05-20" }];
    home.reread();
    later(TINT_MS * 4);
    expect(screen.queryByText(/term, day/)).toBeNull();
  });
});
