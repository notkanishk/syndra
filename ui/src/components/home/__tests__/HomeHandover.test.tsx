// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { Home } from "@/components/home/Home";
import { TINT_MS } from "@/lib/motion";
import { DOOR_BACKSTOP_MS, noteSignIn, resetDoorForTests } from "@/lib/motion/door";
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
  // Whether the queue's first read is still out.
  loading: false,
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
  useGovernanceSummary: () => ({ data: state.loading ? undefined : state.governance, isLoading: state.loading, error: null, refetch: vi.fn() }),
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
  state.loading = false;
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
  // The door is the head itself, staged (board part 1, frames 5 to 7): no extra layer, no band.
  const arch = () => document.querySelector('.page-head[data-door] > .hour-light[data-door]');
  const pool = () => document.querySelector(".hour-light");
  beforeEach(() => {
    resetDoorForTests();
    window.history.replaceState({}, "", "/");
  });
  afterEach(() => vi.restoreAllMocks());

  it("stages the head: the light first, then the greeting, the date line and the lines, each in its own beat", () => {
    mount(signedIn);
    const head = screen.getByRole("heading", { level: 1 }).parentElement!;
    expect(head).toHaveClass("page-head");
    expect(head).toHaveAttribute("data-door");
    expect(arch()).toHaveAttribute("aria-hidden");
    // The beats are plain boxes, the head's own children, in reading order.
    const steps = Array.from(head.children).map((child) => child.getAttribute("data-door-step") ?? child.tagName.toLowerCase());
    expect(steps.filter((step) => step === "h1" || step === "date" || step === "lines")).toEqual(["h1", "date", "lines"]);
    // Nothing waits on it: every word is in the page from the first paint, the greeting reading as ever.
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent(/^\w+, Kanishk\./);
    expect(screen.getByText(/^Claims/, { exact: false })).toBeInTheDocument();
  });

  it("is never a layer of its own: no band, no full-window door, one light", () => {
    mount(signedIn);
    expect(document.querySelector(".door-through, [data-door-through], [data-door-light], .greeting-light")).toBeNull();
    expect(document.querySelectorAll(".hour-light")).toHaveLength(1);
  });

  it("ends when its last block has arrived, and not on any other arrival, whatever the head's clock says", () => {
    mount(signedIn);
    const head = screen.getByRole("heading", { level: 1 }).parentElement!;
    const page = head.parentElement!;
    // jsdom has no AnimationEvent, so React listens for the prefixed name; the name is the native event's.
    const ended = (el: Element, name: string) =>
      act(() => {
        const event = new Event("webkitAnimationEnd", { bubbles: true });
        Object.defineProperty(event, "animationName", { value: name });
        el.dispatchEvent(event);
      });
    // The head settling on its own timer does not end the door: the blocks wait for the light, not the clock.
    later(TINT_MS + 1000);
    expect(head).toHaveAttribute("data-door");
    // The last box that paints: through a list's `display: contents` wrapper, its own last child.
    const wrapper = page.lastElementChild!;
    // Home's blocks come in a list whose wrapper is `display: contents`: the case the door must see through.
    expect(wrapper).toHaveClass("contents");
    const last = wrapper.classList.contains("contents") ? wrapper.lastElementChild! : wrapper;
    ended(head.querySelector("h1")!, "fade");
    ended(last, "fade");
    expect(head).toHaveAttribute("data-door");
    if (last !== wrapper) {
      ended(wrapper, "arrive");
      expect(head).toHaveAttribute("data-door");
    }
    ended(last, "arrive");
    expect(head).not.toHaveAttribute("data-door");
  });

  it("is taken away by its backstop if its last block never arrives", () => {
    mount(signedIn);
    const head = screen.getByRole("heading", { level: 1 }).parentElement!;
    later(DOOR_BACKSTOP_MS - 1);
    expect(head).toHaveAttribute("data-door");
    later(1);
    expect(head).not.toHaveAttribute("data-door");
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
    expect(document.querySelectorAll(".page-head[data-door]")).toHaveLength(0);
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

  it("holds its seat while the terms are out, then fills it, and folds only when the read says there is no term", () => {
    vi.setSystemTime(new Date("2026-11-18T08:10:00Z"));
    state.terms = undefined;
    const home = mount();
    // The head is its full height from the first frame: the line's seat is held, open, before the read lands.
    const seat = document.querySelector('[data-line-seat="term"]');
    expect(seat).not.toBeNull();
    expect(seat!.closest(".unfold")).toHaveAttribute("data-phase", "open");
    // The term lands in the seat that was already there: nothing opens, nothing moves.
    state.terms = [{ name: "Monsoon", start: "2026-08-01", end: "2026-12-31" }];
    home.reread();
    expect(term()?.closest(".unfold")).toHaveAttribute("data-phase", "open");
    expect(document.querySelector('[data-line-seat="term"]')).toBeNull();
    // Between terms the read says there is none: the line folds, smoothly, as any line does.
    state.terms = [{ name: "Spring", start: "2027-01-10", end: "2027-05-20" }];
    home.reread();
    later(TINT_MS * 4);
    expect(screen.queryByText(/term, day/)).toBeNull();
  });

  it("folds its held seat when the read lands with no term at all", () => {
    vi.setSystemTime(new Date("2026-11-18T08:10:00Z"));
    state.terms = undefined;
    const home = mount();
    const unfold = document.querySelector('[data-line-seat="term"]')!.closest(".unfold")!;
    state.terms = [];
    home.reread();
    expect(unfold.getAttribute("data-phase")).not.toBe("open");
  });

  it("holds the Checked line's seat while the queue is read, so the head never grows when it lands", () => {
    vi.setSystemTime(new Date("2026-11-18T08:10:00Z"));
    state.loading = true;
    const home = mount();
    const seat = document.querySelector('[data-line-seat="checked"]');
    expect(seat).not.toBeNull();
    expect(seat).toHaveAttribute("aria-hidden");
    expect(seat!.closest(".unfold")).toHaveAttribute("data-phase", "open");
    // The read lands with a queue that has clear blocks: the real line takes the seat.
    state.loading = false;
    home.reread();
    expect(document.querySelector('[data-line-seat="checked"]')).toBeNull();
  });
});
