// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { TargetOverview } from "@/components/targets/TargetOverview";
import { ApiError } from "@/lib/api-client";

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
  const { withRead } = await import("@/lib/queries/read");
  const idle = () => ({ data: undefined, isLoading: false, isError: false, error: null, refetch: vi.fn() });
  const inert = () => ({ mutate: vi.fn(), mutateAsync: vi.fn(), isPending: false, error: null, data: undefined, reset: vi.fn() });
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
    useReconcileTarget: () => {
      const [data, setData] = React.useState<object | undefined>(undefined);
      const [error, setError] = React.useState<unknown>(null);
      return {
        ...inert(),
        data,
        error,
        mutate: (_: unknown, opts?: { onSuccess?: (r: object) => void; onError?: (e: unknown) => void }) => {
          const answer = passes.answers[Math.min(passes.sent, passes.answers.length - 1)];
          passes.sent++;
          if (answer instanceof Error) {
            setData(undefined);
            setError(answer);
            opts?.onError?.(answer);
            return;
          }
          setError(null);
          const pass = withRead(answer as never);
          setData(pass);
          opts?.onSuccess?.(pass);
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

// A pass skipped inside the cooldown is answered from the add-on's kept copy, its wait in `read` (design §13.3).
const skipped = (wait: number) => ({
  target: "truenas",
  bound: 0,
  queued: 0,
  halted: true,
  reason: "read_stale",
  read: { state: "kept", taken_at: new Date().toISOString(), reason: "cooldown", retry_after: wait },
});
const ran = { target: "truenas", bound: 3, queued: 0, read: { state: "now", taken_at: new Date().toISOString() } };
// With nothing kept, the cooldown is a 503 TARGET_NOT_READ `cooldown`: it waits the same way.
const refused = (wait: number) =>
  new ApiError(503, { error: "TARGET_NOT_READ", message: `TrueNAS was read moments ago. Read it again in ${wait} s.`, details: { reason: "cooldown" }, retry_after: wait });

beforeEach(() => {
  vi.useFakeTimers();
  passes.sent = 0;
});
afterEach(() => vi.useRealTimers());

describe("Check now inside the add-on's cooldown", () => {
  it("waits out retry_after and runs once more by itself", () => {
    passes.answers = [skipped(5), ran];
    renderTarget();
    // The status region is there, empty, before any wait: it is filled, not born, when one starts.
    const pressed = screen.getByRole("button", { name: "Check now" });
    const row = within(pressed.closest(".card") as HTMLElement);
    const region = row.getByRole("status");
    expect(region).toHaveTextContent(/^$/);
    pressed.focus();
    fireEvent.click(pressed);
    expect(passes.sent).toBe(1);
    // Its start announced once in that same region, the line amber in the card's main line, and
    // focus on Stop waiting, which the line describes.
    expect(row.getByRole("status")).toBe(region);
    expect(region).toHaveTextContent(/was read moments ago; trying again by itself in 5 s$/);
    expect(screen.getByText(/was read moments ago; trying again by itself in 5 s$/, { selector: ".text-warn-text" })).toBeInTheDocument();
    const stop = screen.getByRole("button", { name: "Stop waiting" });
    expect(stop).toHaveFocus();
    // D-1 #11 and #2: Stop waiting and Check now each sit in their row's narrow seat.
    expect(stop.closest("[data-action-seat]")).toHaveAttribute("data-seat", "narrow");
    expect(pressed.closest("[data-action-seat]")).toHaveAttribute("data-seat", "narrow");
    expect(stop).toHaveAccessibleDescription(/trying again by itself in 5 s$/);
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

  it("waits out a cooldown with nothing kept (503) the same way, and never says it as a failure", () => {
    passes.answers = [refused(5), ran];
    renderTarget();
    fireEvent.click(screen.getByRole("button", { name: "Check now" }));
    expect(screen.getByText(/was read moments ago; trying again by itself in 5 s$/, { selector: ".text-warn-text" })).toBeInTheDocument();
    expect(screen.queryByText(/ask the person who runs the Syndra server/)).toBeNull();
    act(() => void vi.advanceTimersByTime(5000));
    expect(passes.sent).toBe(2);
    expect(screen.queryByText(/trying again by itself/)).toBeNull();
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
    const pressed = screen.getByRole("button", { name: "Check now" });
    pressed.focus();
    fireEvent.click(pressed);
    expect(within(pressed.closest(".card") as HTMLElement).getByRole("status")).toHaveTextContent(/trying again by itself in 60 s$/);
    act(() => void vi.advanceTimersByTime(10_000));
    fireEvent.click(screen.getByRole("button", { name: "Stop waiting" }));
    // The skip it held is said, counting from when it arrived, with its step.
    expect(screen.getByText(/you can read again in 3590 s\. Press Check now again when the count ends\.$/)).toBeInTheDocument();
    act(() => void vi.advanceTimersByTime(120_000));
    expect(passes.sent).toBe(1);
    // The skip's own count still holds Check now off (§13), however long the add-on said; focus
    // goes back to it the moment it can take it.
    expect(screen.getByRole("button", { name: "Check now" })).toBeDisabled();
    act(() => void vi.advanceTimersByTime(3_600_000));
    expect(screen.getByRole("button", { name: "Check now" })).toHaveFocus();

    // A new press is a new press: it may wait once again, the ceiling at most.
    fireEvent.click(screen.getByRole("button", { name: "Check now" }));
    act(() => void vi.advanceTimersByTime(59_000));
    expect(passes.sent).toBe(2);
    act(() => void vi.advanceTimersByTime(1000));
    expect(passes.sent).toBe(3);
  });
});
