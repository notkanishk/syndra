// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { UiViewProvider } from "@/lib/ui-view";
import { act, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { MergeFindings } from "@/components/targets/MergeFindings";
import { FOLD_MS, TINT_MS } from "@/lib/motion";
import type { MergeFinding } from "@/lib/queries/useTargets";
import { gapSettled } from "@/test-utils/leaving";
import { motionInTests } from "@/test-utils/motion";

// D-1 M3 and M6 on the findings list: the last finding leaves in order (its row fades and folds, the
// count follows, then the all-clear opens), and a finding's Decide form is the row's own panel.

const state = { findings: [] as MergeFinding[] };

vi.mock("@/lib/queries/useTargets", async () => {
  const actual = await vi.importActual<typeof import("@/lib/queries/useTargets")>("@/lib/queries/useTargets");
  return {
    ...actual,
    useMergeFindings: () => ({ data: state.findings, isLoading: false, error: null, refetch: vi.fn() }),
    useResolveMergeFinding: () => ({
      // A saved decision, answered at once.
      mutate: (_input: unknown, opts?: { onSuccess?: () => void }) => opts?.onSuccess?.(),
      isPending: false,
      error: null,
    }),
  };
});

vi.mock("@/lib/queries/useFieldSchema", async () => (await import("@/lib/__fixtures__/field-schema-mock")).fieldSchemaMock);

vi.mock("@/components/names", () => ({
  UserName: ({ id }: { id: string }) => <span>{id}</span>,
}));

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

const tree = () => (
  <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
    <UiViewProvider isOperator initialView="basic">
      <MergeFindings target="truenas" />
    </UiViewProvider>
  </QueryClientProvider>
);

const header = () => screen.getByText("Waiting on a decision").closest(".card") as HTMLElement;
const proto = Element.prototype as { getAnimations?: () => unknown[] };

beforeEach(() => {
  state.findings = [finding()];
  motionInTests();
});
afterEach(() => {
  delete proto.getAnimations;
  motionInTests(false);
  vi.useRealTimers();
});

describe("the last finding leaves in order (M3)", () => {
  it("keeps the count and the row until its fade and fold have played, then opens the all-clear", () => {
    const { rerender, container } = render(tree());
    expect(header().querySelector("[data-card-row]")).not.toBeNull();

    state.findings = [];
    rerender(tree());
    const row = container.querySelector("[data-leaving]")!;
    // Not gone, and not yet counted gone: still one in the header, no all-clear.
    expect(row).not.toBeNull();
    expect(screen.getByTestId("card-count").textContent).toBe("1");
    expect(screen.queryByText("Nothing disputed.")).toBeNull();

    gapSettled(row);
    expect(container.querySelector("[data-leaving]")).toBeNull();
    expect(screen.getByTestId("card-count").textContent).toContain("0");
    expect(screen.getByText("Nothing disputed.")).toBeTruthy();
  });

  it("says it is resolved and lifts, and blooms only when the reader's own decision emptied it", () => {
    const { rerender, container } = render(tree());
    state.findings = [];
    rerender(tree());
    gapSettled(container.querySelector("[data-leaving]")!);
    const empty = screen.getByText("Nothing disputed.").closest('[role="status"]')!;
    expect(empty.className).toContain("lift-in");
    // The healthy dot is the "resolved" one; nobody decided anything, so it is steady.
    expect(empty.querySelector(".bloom")).toBeNull();
  });

  it("blooms when the reader's own decision was just saved", () => {
    const { rerender, container } = render(tree());
    fireEvent.click(screen.getByRole("button", { name: "Decide" }));
    fireEvent.change(screen.getByLabelText("Why"), { target: { value: "Checked it" } });
    fireEvent.click(screen.getByRole("button", { name: "Keep Syndra's value" }));
    state.findings = [];
    rerender(tree());
    gapSettled(container.querySelector("[data-leaving]")!);
    const empty = screen.getByText("Nothing disputed.").closest('[role="status"]')!;
    expect(empty.className).toContain("lift-in");
    expect(empty.querySelector(".bloom")).not.toBeNull();
  });

  it("does not lift or bloom a list that was empty when it was first read", () => {
    state.findings = [];
    render(tree());
    const empty = screen.getByText("Nothing disputed.").closest('[role="status"]')!;
    expect(empty.querySelector(".bloom")).toBeNull();
    // It does not rise on a first paint either, nor open by Unfold: it is simply there.
    expect(empty.className).not.toContain("lift-in");
    // Not opened by Unfold either: it is the page's first paint, simply there.
    expect(empty.closest(".unfold")).toBeNull();
  });

  it("reports what is drawn to the region that counts it, after the row has gone", () => {
    const seen: Array<[number, boolean]> = [];
    const client = new QueryClient();
    const probe = () => (
      <QueryClientProvider client={client}>
        <UiViewProvider isOperator initialView="basic">
          <MergeFindings target="truenas" onDrawn={(n, by) => void seen.push([n, by])} />
        </UiViewProvider>
      </QueryClientProvider>
    );
    const { rerender, container } = render(probe());
    expect(seen.at(-1)).toEqual([1, false]);
    state.findings = [];
    rerender(probe());
    // The read says none; the row is still drawn, so the region still counts it.
    expect(seen.at(-1)?.[0]).toBe(1);
    gapSettled(container.querySelector("[data-leaving]")!);
    expect(seen.at(-1)?.[0]).toBe(0);
  });
});

describe("a finding's Decide form is the row's own panel (M6)", () => {
  it("is not shown once somebody else has decided the finding", () => {
    const { rerender } = render(tree());
    fireEvent.click(screen.getByRole("button", { name: "Decide" }));
    expect(screen.getByLabelText("Why")).toBeTruthy();
    state.findings = [finding({ decision: "keep_ours", decided_by: "op-2" })];
    rerender(tree());
    expect(screen.queryByLabelText("Why")).toBeNull();
  });

  it("opens under the row without changing the row, and closes by fading before it is gone", () => {
    vi.useFakeTimers();
    proto.getAnimations = () => [];
    render(tree());
    const row = screen.getByText("sub-1").closest("[data-card-row]");

    fireEvent.click(screen.getByRole("button", { name: "Decide" }));
    // The same row element: nothing around it was swapped, so nothing remounted.
    expect(screen.getByText("sub-1").closest("[data-card-row]")).toBe(row);
    const form = screen.getByLabelText("Why");
    expect(form.closest(".unfold")).not.toBeNull();
    // The seat's word crossfades: the old Decide is held, hidden, for one tint.
    const seat = row!.querySelector("[data-action-seat]")!;
    expect(seat.querySelector(".tint-out button")).not.toBeNull();
    act(() => void vi.advanceTimersByTime(TINT_MS));
    expect(seat.querySelector("button")).toBeNull();

    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    // Closing: still there, still readable, while it fades.
    expect(screen.getByLabelText("Why").closest(".unfold")?.getAttribute("data-phase")).toBe("closing");
    act(() => void vi.advanceTimersByTime(TINT_MS + FOLD_MS + 200));
    expect(screen.queryByLabelText("Why")).toBeNull();
    // And the control comes back in its seat.
    expect(screen.getByRole("button", { name: "Decide" })).toBeTruthy();
  });
});
