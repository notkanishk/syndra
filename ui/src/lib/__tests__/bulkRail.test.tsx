// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { BulkPlan } from "@/lib/queries/useBulkGrants";
import { useApplyBulk } from "@/lib/queries/useBulkGrants";
import type { Indicators } from "@/lib/queries/useIndicators";
import { useApplyBulkDecision, useRehearseBulkDecision } from "@/lib/queries/useRequests";

/**
 * A-20 P8, the bulk half: the rail's count moves at the answer of a bulk
 * decision and a bulk extend, only on the apply, never on an error.
 */
const wire = vi.hoisted(() => ({
  pending: null as null | { resolve: (v: unknown) => void; reject: (e: unknown) => void },
}));
vi.mock("@/lib/api-client", async (original) => ({
  ...(await original<typeof import("@/lib/api-client")>()),
  request: () => new Promise((resolve, reject) => (wire.pending = { resolve, reject })),
}));

const rail: Indicators = {
  pending_requests: 9,
  expiring_grants: 6,
  pending_propagation: 3,
  drift: 7,
  unconfirmed_revocations: 0,
  holds_due: 2,
  revocations_escalated: false,
  zitadel_reachable: true,
};

let client: QueryClient;
const read = () => client.getQueryData<Indicators>(["governance", "indicators"])!;
const wrapper = ({ children }: { children: ReactNode }) => (
  <QueryClientProvider client={client}>{children}</QueryClientProvider>
);

beforeEach(() => {
  wire.pending = null;
  client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  client.setQueryData(["governance", "indicators"], rail);
});

type Row = { effect: string; wait_reason?: string; grant_ids?: string[] };
const result = (op: string, rows: Row[], summary: Partial<BulkPlan["summary"]> = {}) =>
  ({
    op,
    applied: true,
    outcomes: rows.map((row, i) => ({ user_id: `u${i}`, name: "", email: "", detail: "", ...row })),
    summary: { total: rows.length, apply: 0, no_change: 0, blocked: 0, failed: 0, succeeded: 0, queued: 0, ...summary },
  }) as unknown as BulkPlan;

async function send(run: () => Promise<unknown>, answer: unknown) {
  let done!: Promise<unknown>;
  act(() => {
    done = run().catch(() => "failed");
  });
  await waitFor(() => expect(wire.pending).not.toBeNull());
  expect(read()).toEqual(rail); // nothing moves while it is out
  await act(async () => {
    if (answer instanceof Error) wire.pending!.reject(answer);
    else wire.pending!.resolve(answer);
    await done;
  });
}

describe("bulk decision", () => {
  const input = { ids: ["a", "b", "c", "d"], status: "approved" as const };

  it("takes every decided request off the queue; a waiting approval joins Pending changes", async () => {
    const { result: hook } = renderHook(() => useApplyBulkDecision(), { wrapper });
    await send(
      () => hook.current.mutateAsync(input),
      result(
        "decide",
        [{ effect: "applied" }, { effect: "applied" }, { effect: "queued", wait_reason: "role_waits" }, { effect: "blocked" }],
        { succeeded: 2, queued: 1, blocked: 1 },
      ),
    );
    expect(read().pending_requests).toBe(6);
    expect(read().pending_propagation).toBe(4);
    expect(read().expiring_grants).toBe(6);
  });

  it("a rehearsal moves nothing", async () => {
    const { result: hook } = renderHook(() => useRehearseBulkDecision(), { wrapper });
    await send(() => hook.current.mutateAsync(input), result("decide", [{ effect: "apply" }], { apply: 1 }));
    expect(read()).toEqual(rail);
  });

  it("a failed bulk decision leaves the rail as it was", async () => {
    const { result: hook } = renderHook(() => useApplyBulkDecision(), { wrapper });
    await send(() => hook.current.mutateAsync(input), new Error("boom"));
    expect(read()).toEqual(rail);
  });
});

describe("bulk extend", () => {
  const input = { op: "extend" as const, user_ids: ["a", "b"], reason: "term", duration_days: 30 };

  it("moves the extended grants out of Expiring access, and nothing else", async () => {
    const { result: hook } = renderHook(() => useApplyBulk(), { wrapper });
    await send(
      () => hook.current.mutateAsync(input),
      result("extend", [{ effect: "applied", grant_ids: ["g1", "g2"] }, { effect: "applied", grant_ids: ["g3"] }, { effect: "blocked" }]),
    );
    expect(read().expiring_grants).toBe(3);
    expect(read().pending_propagation).toBe(3);
    expect(read().pending_requests).toBe(9);
  });

  it("a failed extend leaves the rail as it was", async () => {
    const { result: hook } = renderHook(() => useApplyBulk(), { wrapper });
    await send(() => hook.current.mutateAsync(input), new Error("boom"));
    expect(read()).toEqual(rail);
  });

  it("another bulk op does not touch Expiring access", async () => {
    const { result: hook } = renderHook(() => useApplyBulk(), { wrapper });
    await send(
      () => hook.current.mutateAsync({ ...input, op: "assign_role" }),
      result("assign_role", [{ effect: "applied", grant_ids: ["g1"] }]),
    );
    expect(read().expiring_grants).toBe(6);
  });
});
