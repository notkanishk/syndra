// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import PendingChangesPage from "@/app/governance/pending/page";
import type { PendingRow } from "@/lib/queries/usePropagation";
import { setMediaQuery } from "@/test-utils/media";
import { motionInTests } from "@/test-utils/motion";

/** Motion v2 S13: the queue emptying, said once. */

const pending = vi.hoisted(() => ({ data: [] as PendingRow[] }));
const drain = vi.hoisted(() => ({ run: vi.fn() }));

vi.mock("@/lib/queries/usePropagation", () => ({
  usePendingPropagations: () => ({ ...pending, isLoading: false, error: null, refetch: () => {} }),
  useDrainPropagations: () => ({ mutateAsync: drain.run, isPending: false }),
}));
vi.mock("@/lib/queries/useBundles", () => ({ useBundles: () => ({ data: [], isLoading: false, error: null }) }));
vi.mock("@/lib/queries/useMappingRules", () => ({ useMappingRules: () => ({ data: [], isLoading: false, error: null }) }));
vi.mock("@/lib/queries/useGovernance", () => ({
  useGovernanceSummary: () => ({ data: { pending_propagation: { count: 0, zitadel_reachable: true } } }),
}));

const row = (id: string): PendingRow =>
  ({
    id,
    op_type: "add",
    user_id: "u1",
    project_id: "p1",
    role_keys: ["trained"],
    source: "direct",
    source_ref: "",
    cascade_id: id,
    status: "pending",
    attempts: 0,
    created_at: "2026-07-31T09:42:00Z",
  }) as PendingRow;

function tree() {
  return (
    <QueryClientProvider client={new QueryClient()}>
      <PendingChangesPage />
    </QueryClientProvider>
  );
}

const LINE = "Everything has reached Zitadel";
const sendButton = () => screen.queryByRole("button", { name: /^Send / });

beforeEach(() => {
  pending.data = [row("o1"), row("o2"), row("o3")];
  drain.run.mockReset();
  drain.run.mockResolvedValue({ applied: 3, failed: 0, requeued: 0, errored: 0 });
});

describe("Everything sent (S13), with the page still", () => {
  it("is not said for a queue that was empty when the page opened", () => {
    pending.data = [];
    render(tree());
    expect(screen.queryByText(LINE)).not.toBeInTheDocument();
    expect(screen.getByText("Nothing is waiting for you to send.")).toBeInTheDocument();
  });

  it("this operator's send says what went, when, and that nothing waits, with the dot blooming", async () => {
    vi.useFakeTimers({ toFake: ["Date"] });
    vi.setSystemTime(new Date("2026-10-19T11:20:00"));
    const view = render(tree());
    await act(async () => void fireEvent.click(sendButton()!));
    pending.data = [];
    view.rerender(tree());

    expect(screen.getByText(LINE)).toBeInTheDocument();
    expect(screen.getByText("3 changes sent at 11:20 · nothing is waiting for you")).toBeInTheDocument();
    expect(screen.getByText(LINE).previousElementSibling).toHaveClass("bloom");
    expect(screen.getByText(LINE).closest("[role=status]")).not.toBeNull();
    expect(sendButton()).toBeNull();
    vi.useRealTimers();
  });

  it("a queue someone else sent shows the line without the bloom, and claims no count or time", () => {
    const view = render(tree());
    pending.data = [];
    view.rerender(tree());
    expect(screen.getByText(LINE)).toBeInTheDocument();
    expect(screen.getByText(LINE).previousElementSibling).not.toHaveClass("bloom");
    expect(screen.getByText("Nothing is waiting for you to send")).toBeInTheDocument();
    expect(screen.queryByText(/sent at/)).not.toBeInTheDocument();
  });

  it("a send that failed is not a send: an emptied queue after it is still someone else's", async () => {
    drain.run.mockRejectedValue(new Error("no"));
    const view = render(tree());
    await act(async () => void fireEvent.click(sendButton()!));
    pending.data = [];
    view.rerender(tree());
    expect(screen.getByText(LINE).previousElementSibling).not.toHaveClass("bloom");
  });

  it("goes away when changes are waiting again", () => {
    const view = render(tree());
    pending.data = [];
    view.rerender(tree());
    pending.data = [row("o9")];
    view.rerender(tree());
    expect(screen.queryByText(LINE)).not.toBeInTheDocument();
    expect(sendButton()).toHaveTextContent("Send 1 change");
  });
});

describe("Everything sent (S13), in motion", () => {
  beforeEach(() => {
    motionInTests();
    // Without the Web Animations API a region opens and closes at once; this is
    // a browser's, and the fold's end is told by hand.
    Element.prototype.getAnimations = () => [];
  });
  afterEach(() => {
    delete (Element.prototype as { getAnimations?: unknown }).getAnimations;
  });

  it("the sent rows fade and the list folds first, then the line rises into its place", () => {
    const view = render(tree());
    pending.data = [];
    view.rerender(tree());

    // Still the rows, leaving: what was sent is what fades.
    expect(document.querySelector("[data-phase='closing']")).not.toBeNull();
    expect(screen.getAllByText("trained").length).toBeGreaterThan(0);
    expect(screen.queryByText(LINE)).not.toBeInTheDocument();
    // The button is on its way out, not clickable.
    expect(sendButton()?.closest(".tint-out")).not.toBeNull();

    // jsdom has no AnimationEvent, so React listens for the prefixed name here.
    const folding = document.querySelector("[data-phase='closing']")!;
    act(() => void fireEvent(folding, new Event("webkitAnimationEnd", { bubbles: true })));
    expect(screen.getByText(LINE)).toBeInTheDocument();
    expect(screen.getByText(LINE).closest("[role=status]")).toHaveClass("lift-in");
    expect(screen.queryByText("trained")).not.toBeInTheDocument();
    expect(sendButton()).toBeNull();
  });

  it("in reduced motion the line is simply there at once", () => {
    setMediaQuery("(prefers-reduced-motion: reduce)", true);
    // A browser's reduced-motion CSS ends the fold in 0.01ms; jsdom has no animations at all.
    delete (Element.prototype as { getAnimations?: unknown }).getAnimations;
    const view = render(tree());
    pending.data = [];
    view.rerender(tree());
    expect(screen.getByText(LINE)).toBeInTheDocument();
    expect(sendButton()).toBeNull();
  });
});

describe("A change that goes on a later read (S5)", () => {
  beforeEach(() => motionInTests());

  it("one cancelled elsewhere fades where it was and the group stops counting it", () => {
    pending.data = [
      { ...row("o1"), cascade_id: "same" },
      { ...row("o2"), cascade_id: "same", role_keys: ["door"] },
      { ...row("o3"), cascade_id: "same", role_keys: ["laser"] },
    ];
    const view = render(tree());
    expect(screen.getByText(/These 3 changes come from one edit/)).toBeInTheDocument();

    pending.data = [pending.data[0], pending.data[2]];
    view.rerender(tree());

    const door = screen.getByText("door").closest("[aria-hidden=true]");
    expect(door).not.toBeNull();
    expect(screen.getByText(/These 2 changes come from one edit/)).toBeInTheDocument();
    // Still the Send button's number: what is waiting is two.
    expect(sendButton()).toHaveTextContent("Send 2 changes");
  });
});
