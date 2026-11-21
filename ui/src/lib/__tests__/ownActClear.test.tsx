// @vitest-environment jsdom
import { QueryClientProvider } from "@tanstack/react-query";
import { act, render, renderHook } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { EmptyState } from "@/components/states";
import { makeQueryClient } from "@/lib/query-client";
import { useQueueCleared } from "@/lib/useQueueCleared";

beforeEach(() => vi.useFakeTimers());
afterEach(() => vi.useRealTimers());

const A = { id: "a" };

describe("an all-clear blooms only after the operator's own act", () => {
  it("claims a queue that empties just after their own write", async () => {
    const qc = makeQueryClient();
    const wrapper = ({ children }: { children: ReactNode }) =>
      createElement(QueryClientProvider, { client: qc }, children);
    const { result, rerender } = renderHook(({ rows }) => useQueueCleared(rows, true, true), {
      initialProps: { rows: [A] },
      wrapper,
    });
    await act(async () => {
      await qc.getMutationCache().build(qc, { mutationFn: async () => "ok" }).execute(undefined);
    });
    rerender({ rows: [] });
    expect(result.current.cleared?.mine).not.toBeNull();
    expect(result.current.glow).toBe(true);
  });

  // The real path: an all-clear that mounts when the page says so (a leaving row can hold it for seconds),
  // an error in its place while a read fails, and the all-clear again when the retry lands.
  type Props = { rows: typeof A[]; ready: boolean; show: boolean };
  const Page = ({ rows, ready, show }: Props) => {
    const { glow, spend } = useQueueCleared(rows, ready, true);
    if (!ready) return createElement("p", null, "The read failed");
    if (rows.length > 0) return createElement("p", null, "The list");
    if (!show) return createElement("p", null, "Still leaving");
    return createElement(EmptyState, { resolved: true, lift: glow, glow, onOpened: spend, title: "Done." });
  };
  const own = async () => {
    const qc = makeQueryClient();
    const wrapper = ({ children }: { children: ReactNode }) => createElement(QueryClientProvider, { client: qc }, children);
    const view = render(createElement(Page, { rows: [A], ready: true, show: true }), { wrapper });
    await act(async () => {
      await qc.getMutationCache().build(qc, { mutationFn: async () => "ok" }).execute(undefined);
    });
    const again = (props: Props) => view.rerender(createElement(Page, props));
    return { view, again };
  };
  const dot = (c: HTMLElement) => c.querySelector("[data-dot]");

  it("(a) a slow leave: the all-clear that mounts seconds after the clearing still lifts and opens, once", async () => {
    const { view, again } = await own();
    again({ rows: [], ready: true, show: false });
    act(() => void vi.advanceTimersByTime(5_000));
    again({ rows: [], ready: true, show: true });
    expect(dot(view.container)).toHaveClass("ember-open");
    expect(view.container.querySelector("[role=status]")).toHaveClass("lift-in");
  });

  it("(b) clear, then a read that fails, then one that lands: the same all-clear comes back, with no second opening", async () => {
    const { view, again } = await own();
    again({ rows: [], ready: true, show: true });
    expect(dot(view.container)).toHaveClass("ember-open");
    again({ rows: [], ready: false, show: true });
    expect(dot(view.container)).toBeNull();
    again({ rows: [], ready: true, show: true });
    expect(dot(view.container)).toHaveClass("ember");
    expect(dot(view.container)).not.toHaveClass("ember-open");
    expect(view.container.querySelector("[role=status]")).not.toHaveClass("lift-in");
  });

  it("(b') a read that fails before the all-clear ever mounted spends the opening too", async () => {
    const { view, again } = await own();
    again({ rows: [], ready: true, show: false });
    again({ rows: [], ready: false, show: false });
    again({ rows: [], ready: true, show: true });
    expect(dot(view.container)).not.toHaveClass("ember-open");
    expect(view.container.querySelector("[role=status]")).not.toHaveClass("lift-in");
  });

  it("(c) a re-render or a remount after the opening does not replay it, and the opening is not cut short", async () => {
    const { view, again } = await own();
    again({ rows: [], ready: true, show: true });
    const first = dot(view.container);
    again({ rows: [], ready: true, show: true });
    again({ rows: [], ready: true, show: true });
    expect(dot(view.container)).toBe(first);
    expect(first).toHaveClass("ember-open");
    again({ rows: [], ready: true, show: false });
    again({ rows: [], ready: true, show: true });
    expect(dot(view.container)).not.toHaveClass("ember-open");
  });


  it("does not claim a queue someone else emptied after it refilled, on the strength of a write from before the refill", async () => {
    const qc = makeQueryClient();
    const wrapper = ({ children }: { children: ReactNode }) =>
      createElement(QueryClientProvider, { client: qc }, children);
    const { result, rerender } = renderHook(({ rows }) => useQueueCleared(rows, true, true), {
      initialProps: { rows: [A] },
      wrapper,
    });
    act(() => void vi.advanceTimersByTime(1));
    await act(async () => {
      await qc.getMutationCache().build(qc, { mutationFn: async () => "ok" }).execute(undefined);
    });
    rerender({ rows: [] });
    expect(result.current.cleared?.mine).not.toBeNull();
    // It refills, and within the 30s someone else clears it: their write was before the refill.
    act(() => void vi.advanceTimersByTime(1_000));
    rerender({ rows: [{ id: "b" }] });
    expect(result.current.cleared).toBeNull();
    act(() => void vi.advanceTimersByTime(1_000));
    rerender({ rows: [] });
    expect(result.current.cleared?.mine).toBeNull();
    expect(result.current.glow).toBe(false);
  });

  it("does not claim one that empties on a read nobody caused, or that arrived empty", () => {
    vi.setSystemTime(new Date(2030, 0, 1)); // later than any write another test made
    const { result, rerender } = renderHook(({ rows }) => useQueueCleared(rows, true, true), {
      initialProps: { rows: [A] },
    });
    rerender({ rows: [] });
    expect(result.current.cleared?.mine).toBeNull();

    const arrived = renderHook(() => useQueueCleared([] as typeof A[], true, true));
    expect(arrived.result.current.cleared).toBeNull();
  });
});

describe("EmptyState's glow", () => {
  it("draws the ember's opening and the lift only when asked, and the plain arrival otherwise", () => {
    const own = render(<EmptyState resolved lift glow title="Done." />);
    expect(own.container.querySelector(".ember-open")).toBeTruthy();
    expect(own.container.querySelector(".lift-in")).toBeTruthy();
    own.unmount();
    const plain = render(<EmptyState resolved title="Done." />);
    expect(plain.container.querySelector(".ember-open")).toBeNull();
    expect(plain.container.querySelector(".lift-in")).toBeNull();
  });
});
