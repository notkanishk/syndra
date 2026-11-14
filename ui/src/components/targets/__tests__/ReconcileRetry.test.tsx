// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { TargetOverview } from "@/components/targets/TargetOverview";

/**
 * "Check now" inside the add-on's cooldown (7l): the skipped pass carries
 * retry_after, and the press waits it out and runs once more by itself (RFC
 * 9110 §10.2.3), so one press ends on a live read. Once only, never past the
 * 60 s ceiling, and Stop waiting cancels it.
 */

vi.mock("@/lib/queries/useTargetSystemHealth", () => ({
  useTargetSystemHealth: () => ({
    data: undefined,
    isLoading: false,
    error: null,
  }),
}));

// Every helper lives INSIDE the factory: `vi.mock` is hoisted above every
// const in the file, so a top-level one is not yet initialised when it runs.
// The passes the backend answers, in order; the last repeats.
const passes = vi.hoisted(() => ({ answers: [] as object[], sent: 0 }));

vi.mock("@/lib/queries/useTargets", async () => {
  const React = await import("react");
  const idle = () => ({ data: undefined, isLoading: false, isError: false, error: null, refetch: vi.fn() });
  const inert = () => ({ mutate: vi.fn(), mutateAsync: vi.fn(), isPending: false, error: null, data: undefined, reset: vi.fn() });
  return {
    useTargets: () => ({ ...idle(), data: [] }),
    useUnconfirmedOperations: () => ({ data: [], isLoading: false, error: null }),
    useMergeFindings: () => ({ ...idle(), data: [] }),
    useTargetHealth: idle,
    useTargetInventory: idle,
    useTargetInventoryAgain: () => ({ press: async () => {}, pending: false, error: null }),
    useResolveLogFinding: inert,
    useResolveBindingConflict: inert,
    useSetLifecycle: inert,
    useReleaseBinding: inert,
    useAdoptAccount: inert,
    useReconcileTarget: () => {
      const [data, setData] = React.useState<object | undefined>(undefined);
      return {
        ...inert(),
        data,
        mutate: (_: unknown, opts?: { onSuccess?: (r: object) => void }) => {
          const answer = passes.answers[Math.min(passes.sent, passes.answers.length - 1)];
          passes.sent++;
          setData(answer);
          opts?.onSuccess?.(answer);
        },
      };
    },
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

const skipped = (wait: number) => ({ target: "truenas", bound: 0, queued: 0, current: false, halted: true, reason: "read_stale", retry_after: wait });
const ran = { target: "truenas", bound: 3, queued: 0, current: true };

beforeEach(() => {
  vi.useFakeTimers();
  passes.sent = 0;
});
afterEach(() => vi.useRealTimers());

describe("Check now inside the add-on's cooldown", () => {
  it("waits out retry_after and runs once more by itself", () => {
    passes.answers = [skipped(5), ran];
    renderTarget();
    fireEvent.click(screen.getByRole("button", { name: "Check now" }));
    expect(passes.sent).toBe(1);
    // Its start announced once, the line amber in the card's main line, and focus on Stop waiting.
    expect(screen.getByText(/was read moments ago; trying again by itself in 5 s$/, { selector: '[role="status"]' })).toBeInTheDocument();
    expect(screen.getByText(/was read moments ago; trying again by itself in 5 s$/, { selector: ".text-warn-text > span[aria-hidden]" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Stop waiting" })).toHaveFocus();
    expect(screen.queryByText(/stopped before it concluded anything/i)).toBeNull();
    // Off until the count ends, with the line as its reason, linked (§13).
    const check = screen.getByRole("button", { name: "Check now" });
    expect(check).toBeDisabled();
    expect(check).toHaveAccessibleDescription(/trying again by itself in 5 s$/);
    act(() => void vi.advanceTimersByTime(5000));
    expect(passes.sent).toBe(2);
    expect(screen.queryByText(/trying again by itself/)).toBeNull();
    expect(screen.queryByText(/stopped before it concluded anything/i)).toBeNull();
  });

  it("retries once only: a second skip says when to press again, never the pass's raw reason", () => {
    passes.answers = [skipped(5)];
    renderTarget();
    fireEvent.click(screen.getByRole("button", { name: "Check now" }));
    act(() => void vi.advanceTimersByTime(5000));
    expect(passes.sent).toBe(2);
    expect(screen.getByText(/was read moments ago; you can read again in 5 s\. Press Check now again when the count ends\.$/)).toBeInTheDocument();
    expect(screen.queryByText(/read_stale|stopped before it concluded|once it is reachable|Nothing was concluded/i)).toBeNull();
    // Off until its count ends, then on, and the step says so.
    expect(screen.getByRole("button", { name: "Check now" })).toBeDisabled();
    act(() => void vi.advanceTimersByTime(120_000));
    expect(passes.sent).toBe(2);
    expect(screen.getByText("Press Check now again.")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Check now" })).toBeEnabled();
  });

  it("leaving the page cancels the wait", () => {
    passes.answers = [skipped(5), ran];
    const page = renderTarget();
    fireEvent.click(screen.getByRole("button", { name: "Check now" }));
    page.unmount();
    act(() => void vi.advanceTimersByTime(120_000));
    expect(passes.sent).toBe(1);
  });

  it("Stop waiting cancels it, and the wait is never longer than 60 s", () => {
    passes.answers = [skipped(3600), skipped(3600), ran];
    renderTarget();
    fireEvent.click(screen.getByRole("button", { name: "Check now" }));
    expect(screen.getByText(/trying again by itself in 60 s$/, { selector: '[role="status"]' })).toBeInTheDocument();
    act(() => void vi.advanceTimersByTime(10_000));
    fireEvent.click(screen.getByRole("button", { name: "Stop waiting" }));
    // The skip it held is said, counting from when it arrived, with its step.
    expect(screen.getByText(/you can read again in 3590 s\. Press Check now again when the count ends\.$/)).toBeInTheDocument();
    act(() => void vi.advanceTimersByTime(120_000));
    expect(passes.sent).toBe(1);
    // The skip's own count still holds Check now off (§13), however long the add-on said.
    expect(screen.getByRole("button", { name: "Check now" })).toBeDisabled();
    act(() => void vi.advanceTimersByTime(3_600_000));

    // A new press is a new press: it may wait once again, the ceiling at most.
    fireEvent.click(screen.getByRole("button", { name: "Check now" }));
    act(() => void vi.advanceTimersByTime(59_000));
    expect(passes.sent).toBe(2);
    act(() => void vi.advanceTimersByTime(1000));
    expect(passes.sent).toBe(3);
  });
});
