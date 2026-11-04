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

describe("EmptyState's bloom", () => {
  it("draws the bloom and the lift only when asked, and the plain arrival otherwise", () => {
    const own = render(<EmptyState resolved lift bloom title="Done." />);
    expect(own.container.querySelector(".bloom")).toBeTruthy();
    expect(own.container.querySelector(".lift-in")).toBeTruthy();
    own.unmount();
    const plain = render(<EmptyState resolved title="Done." />);
    expect(plain.container.querySelector(".bloom")).toBeNull();
    expect(plain.container.querySelector(".lift-in")).toBeNull();
  });
});
