// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { UiViewProvider } from "@/lib/ui-view";
import { act, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { TargetOverview } from "@/components/targets/TargetOverview";
import { FLASH_MS, FOLD_MS, TINT_MS } from "@/lib/motion";
import type { MergeFinding, TargetHealthWire, TargetInventory, TargetSummary } from "@/lib/queries/useTargets";
import { asRead, readNow } from "@/test-utils/wire";
import { gapSettled } from "@/test-utils/leaving";
import { motionInTests } from "@/test-utils/motion";

// D-1 motion on the TrueNAS page: M1 (the regions arrive one after another), M3 (the last item
// leaves in order, counted from what is drawn), M4 (Check now lands), M5 (the accounts table).

const state = {
  roster: [] as TargetSummary[],
  health: {} as TargetHealthWire,
  findings: [] as MergeFinding[],
  inventory: {} as Omit<TargetInventory, "read">,
  reconcile: undefined as undefined | Record<string, unknown>,
};

vi.mock("@/lib/queries/useNameResolver", () => ({
  useNameResolver: () => {
    const miss = { value: undefined, resolved: true } as const;
    return { resolveUser: () => miss, resolveProject: () => miss, resolveRole: () => miss, resolveBundle: () => miss };
  },
}));

vi.mock("@/lib/queries/useTargets", async () => {
  const actual = await vi.importActual<typeof import("@/lib/queries/useTargets")>("@/lib/queries/useTargets");
  return {
    ...actual,
    useTargets: () => ({ data: state.roster, isLoading: false, error: null }),
    useTargetHealth: () => ({ data: actual.healthReplyOf({ read: readNow(), ...state.health }), isLoading: false, error: null, refetch: vi.fn() }),
    useMergeFindings: () => ({ data: state.findings, isLoading: false, error: null, refetch: vi.fn() }),
    useResolveMergeFinding: () => ({
      mutate: (_input: unknown, opts?: { onSuccess?: () => void }) => opts?.onSuccess?.(),
      isPending: false,
      error: null,
    }),
    useUnconfirmedOperations: () => ({ data: [], isLoading: false, error: null }),
    useTargetInventory: () => ({ data: asRead(state.inventory), isLoading: false, error: null, refetch: vi.fn() }),
    useAdoptAccount: () => ({ mutate: vi.fn(), reset: vi.fn(), isPending: false, error: null }),
    useReleaseBinding: () => ({ mutate: vi.fn(), isPending: false, error: null }),
    useSetLifecycle: () => ({ mutate: vi.fn(), reset: vi.fn(), isPending: false }),
    useTargetSettings: () => ({ data: asRead({ settings: [] }), isLoading: false, error: null, refetch: vi.fn() }),
    useTargetHomes: () => ({ data: asRead({ accounts: [] }), isLoading: false, error: null }),
    useSetTargetSetting: () => ({ mutate: vi.fn(), isPending: false, error: null }),
    useSetTargetProject: () => ({ mutate: vi.fn(), isPending: false, error: null }),
    useReconcileTarget: () => ({ mutate: vi.fn(), isPending: false, data: state.reconcile, error: null }),
    useResolveLogFinding: () => ({ mutate: vi.fn(), reset: vi.fn(), isPending: false, error: null }),
  };
});

const roster = (): TargetSummary => ({
  target: "truenas",
  name: "TrueNAS",
  name_from: "product",
  registered: true,
  auth_mode: "derived",
  callable: true,
  operations: [],
  circuit_open: false,
  project: { id: "p1", name: "Network storage" },
});

const finding = (over: Partial<MergeFinding> = {}): MergeFinding => ({
  id: "f1",
  target: "truenas",
  subject_id: "sub-1",
  field: "enabled",
  outcome: "theirs_only",
  base: true,
  ours: true,
  theirs: false,
  detected_at: "2026-08-18T12:00:00Z",
  last_seen_at: "2026-08-19T12:00:00Z",
  ...over,
});

const anchor = {
  target: "truenas",
  head: "aaaaaaaaaaaaaaaa",
  records: 12,
  anchored_at: "2026-08-01T00:00:00Z",
  violation_reason: "records_decreased",
  violation_head: "bbbbbbbbbbbbbbbb",
  violation_records: 3,
  violation_at: "2026-08-09T00:00:00Z",
};

// A new element each time, so a rerender really draws again.
const tree = () => (
  <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
    <UiViewProvider isOperator initialView="advanced">
      <TargetOverview target="truenas" />
    </UiViewProvider>
  </QueryClientProvider>
);

const regionCount = () => document.getElementById("waiting-heading")!.parentElement!.textContent;
const proto = Element.prototype as { getAnimations?: () => unknown[] };

beforeEach(() => {
  state.roster = [roster()];
  state.health = { reachable: true, lifecycle: "active" };
  state.findings = [];
  state.inventory = { target: "truenas", bound: 0, unmanaged: [] };
  state.reconcile = undefined;
});
afterEach(() => {
  delete proto.getAnimations;
  motionInTests(false);
  vi.useRealTimers();
});

describe("the regions arrive one after another (M1)", () => {
  it("carries the stagger on each wrapper its regions sit in", () => {
    render(tree());
    const outer = document.getElementById("land")!.closest("[data-arrive-through]");
    const inner = document.getElementById("waiting")!.closest("[data-arrive-through]");
    expect(outer).not.toBeNull();
    expect(inner).not.toBeNull();
    // Two wrappers, not one: the waiting region's wrapper is the one inside the project's gate.
    expect(inner).not.toBe(outer);
    expect(outer!.contains(inner)).toBe(true);
  });
});

describe("the last item leaves in order (M3)", () => {
  beforeEach(() => motionInTests());

  it("counts the finding until its row has gone, then opens the all-clear, steady for a read", () => {
    state.findings = [finding()];
    const { rerender, container } = render(tree());
    expect(regionCount()).toBe("Waiting on you1");

    state.findings = [];
    rerender(tree());
    const row = container.querySelector("[data-leaving]")!;
    expect(row).not.toBeNull();
    // Read as empty, drawn as one: the region still counts the row on screen, and says nothing is
    // waiting only when it is not.
    expect(regionCount()).toBe("Waiting on you1");
    expect(screen.queryByText("Nothing is waiting on you.")).toBeNull();

    gapSettled(row);
    expect(regionCount()).toContain("0");
    const empty = screen.getByText("Nothing is waiting on you.").closest('[role="status"]')!;
    expect(empty.className).toContain("lift-in");
    // Opened with its card, not ahead of it: the card is inside the Unfold.
    expect(empty.closest(".card")!.closest(".unfold")).not.toBeNull();
    // Nobody decided anything: the dot is steady.
    expect(empty.querySelector(".ember-open")).toBeNull();
  });

  it("blooms when the reader's own decision emptied it", () => {
    state.findings = [finding()];
    const { rerender, container } = render(tree());
    fireEvent.click(screen.getByRole("button", { name: "Decide" }));
    fireEvent.change(screen.getByLabelText("Why"), { target: { value: "Checked it" } });
    fireEvent.click(screen.getByRole("button", { name: "Keep Syndra's value" }));
    state.findings = [];
    rerender(tree());
    gapSettled(container.querySelector("[data-leaving]")!);
    const empty = screen.getByText("Nothing is waiting on you.").closest('[role="status"]')!;
    expect(empty.className).toContain("lift-in");
    expect(empty.querySelector(".ember-open")).not.toBeNull();
  });

  it("lets the anchor's finding leave with its Notice, drawn while the count holds", () => {
    vi.useFakeTimers();
    proto.getAnimations = () => [];
    state.health = { reachable: true, lifecycle: "active", log_anchor: anchor };
    const { rerender } = render(tree());
    const finding = () => document.getElementById("waiting")!.querySelector("[data-phase]");
    expect(finding()).not.toBeNull();

    state.health = { reachable: true, lifecycle: "active", log_anchor: { ...anchor, violation_reason: undefined } };
    rerender(tree());
    // Closing, not gone: the row is still drawn while the count still says one.
    expect(finding()?.getAttribute("data-phase")).toBe("closing");
    expect(regionCount()).toBe("Waiting on you1");
  });

  it("holds the count for the Notice's fade and the all-clear for its fold", () => {
    vi.useFakeTimers();
    proto.getAnimations = () => [];
    state.health = { reachable: true, lifecycle: "active", log_anchor: anchor };
    const { rerender } = render(tree());
    expect(screen.getByText("One item under Waiting on you needs a decision.")).toBeTruthy();
    expect(regionCount()).toBe("Waiting on you1");

    state.health = { reachable: true, lifecycle: "active", log_anchor: { ...anchor, violation_reason: undefined } };
    rerender(tree());
    // The Notice is fading: still drawn, still readable, and the item still counted.
    expect(screen.getByText("One item under Waiting on you needs a decision.")).toBeTruthy();
    expect(regionCount()).toBe("Waiting on you1");

    act(() => void vi.advanceTimersByTime(TINT_MS));
    // Faded: now its space folds while the count crossfades to none, and the all-clear waits.
    expect(regionCount()).toContain("0");
    expect(screen.queryByText("Nothing is waiting on you.")).toBeNull();

    act(() => void vi.advanceTimersByTime(FOLD_MS));
    expect(screen.getByText("Nothing is waiting on you.")).toBeTruthy();
    act(() => void vi.advanceTimersByTime(1000));
    expect(screen.queryByText("One item under Waiting on you needs a decision.")).toBeNull();
  });
});

describe("Check now lands (M4)", () => {
  const read = (over: Record<string, unknown> = {}) => ({
    target: "truenas", bound: 4, queued: 1, read: readNow(), ...over,
  });
  const checkRow = () => screen.getByRole("button", { name: "Check now" }).closest("[data-card-row]")!;

  beforeEach(() => {
    motionInTests();
    vi.useFakeTimers();
  });

  it("crossfades the sentence on a new result and washes the row once, never for the same one", () => {
    state.reconcile = read();
    const { rerender } = render(tree());
    // The first sight of a result is an arrival, not news.
    expect(checkRow().className).not.toContain("flash");
    expect(screen.getByText(/4 people with a managed account/)).toBeTruthy();

    state.reconcile = read({ bound: 5, queued: 2 });
    rerender(tree());
    act(() => void vi.advanceTimersByTime(20));
    // The old sentence is held (hidden) for one tint while the new one fades in.
    expect(checkRow().querySelector(".tint-out")).not.toBeNull();
    expect(checkRow().className).toContain("flash");

    act(() => void vi.advanceTimersByTime(FLASH_MS + TINT_MS));
    expect(checkRow().className).not.toContain("flash");
    expect(checkRow().querySelector(".tint-out")).toBeNull();

    // The same result read again is not news.
    state.reconcile = read({ bound: 5, queued: 2 });
    rerender(tree());
    act(() => void vi.advanceTimersByTime(50));
    expect(checkRow().className).not.toContain("flash");
    expect(checkRow().querySelector(".tint-out")).toBeNull();
  });

  it("does not treat the gap while a check is out as a new result", () => {
    // Each press is a new mutation, whose data is undefined until it answers: read, none, read again.
    const stale = [{ subject_id: "s1", username: "becky", account_id: "3999" }];
    state.reconcile = read({ stale });
    const { rerender } = render(tree());
    const list = screen.getByText("becky").closest(".arrive-list");

    state.reconcile = undefined;
    rerender(tree());
    act(() => void vi.advanceTimersByTime(50));
    // The old words and rows stay until the new are read.
    expect(screen.getByText(/4 people with a managed account/)).toBeTruthy();
    expect(checkRow().className).not.toContain("flash");

    state.reconcile = read({ stale });
    rerender(tree());
    act(() => void vi.advanceTimersByTime(50));
    expect(checkRow().className).not.toContain("flash");
    expect(checkRow().querySelector(".tint-out")).toBeNull();
    // The list is the same element: it did not remount, so it did not replay its arrival.
    expect(screen.getByText("becky").closest(".arrive-list")).toBe(list);
  });

  it("lets the accounts a pass found arrive as a list", () => {
    state.reconcile = read({ stale: [{ subject_id: "s1", username: "becky", account_id: "3999" }] });
    render(tree());
    const list = screen.getByText("becky").closest(".arrive-list");
    expect(list).not.toBeNull();
  });
});

describe("accounts not created by Syndra leave by fading and folding (M5)", () => {
  beforeEach(() => motionInTests());

  it("marks an assigned account leaving, still drawn, before it goes", () => {
    state.inventory = {
      target: "truenas", bound: 0,
      unmanaged: [{ username: "iyo", account_id: "3000" }, { username: "kairi", account_id: "3001" }],
    };
    const { rerender, container } = render(tree());
    state.inventory = { ...state.inventory, unmanaged: [{ username: "kairi", account_id: "3001" }] };
    rerender(tree());

    const row = container.querySelector("[data-leaving]");
    expect(row).not.toBeNull();
    expect(row!.textContent).toContain("iyo");
    gapSettled(row!);
    expect(screen.queryByText("iyo")).toBeNull();
    expect(screen.getByText("kairi")).toBeTruthy();
  });
});
