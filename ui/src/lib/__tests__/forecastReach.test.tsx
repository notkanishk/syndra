// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { bulkForecast } from "@/components/ui/RehearsalDialog";
import { FORECAST, forecastFor } from "@/lib/outcome";
import { useRevokeDrift } from "@/lib/queries/useDrift";
import { useLiftHold } from "@/lib/queries/useHolds";
import { settleIndicators, type Indicators } from "@/lib/queries/useIndicators";
import { useDecideRequest } from "@/lib/queries/useRequests";
import type { BulkPlan } from "@/lib/queries/useBulkGrants";

/**
 * A-20 P8: the words of the forecast, and the rail's count moving at the answer
 * (never ahead of it, never on an error), through the real mutation hooks.
 */
const wire = vi.hoisted(() => ({
  pending: null as null | { resolve: (v: unknown) => void; reject: (e: unknown) => void },
}));
vi.mock("@/lib/api-client", async (original) => ({
  ...(await original<typeof import("@/lib/api-client")>()),
  request: () => new Promise((resolve, reject) => (wire.pending = { resolve, reject })),
}));

describe("forecastFor", () => {
  it("says the act in the future tense", () => {
    expect(forecastFor("approve")).toBe("Will be approved");
    expect(forecastFor("give")).toBe("Will be given");
    expect(forecastFor("remove", false)).toBe("Will be removed");
  });

  it("says 'Will wait for you to send' for a role that waits, for every act that reaches Zitadel", () => {
    for (const act of ["approve", "extend", "remove", "give"] as const) {
      expect(forecastFor(act, true)).toBe("Will wait for you to send");
    }
  });

  it("does not claim a wait for an act that never reaches Zitadel", () => {
    for (const act of ["decline", "keep", "lift", "cancel", "adopt", "mark", "replace"] as const) {
      expect(forecastFor(act, true)).toBe(FORECAST[act]);
    }
  });
});

function plan(op: BulkPlan["op"], effects: Array<"apply" | "queued" | "no_change">): BulkPlan {
  return {
    op,
    applied: false,
    outcomes: effects.map((effect, i) => ({ user_id: `u${i}`, effect, detail: "" })),
    summary: { total: effects.length, apply: 0, no_change: 0, blocked: 0, failed: 0, succeeded: 0, queued: 0 },
  } as BulkPlan;
}

describe("bulkForecast", () => {
  it("is the wait when every row that acts will wait, as the rehearsal marked them", () => {
    expect(bulkForecast(plan("assign_role", ["queued", "queued", "no_change"]))).toBe(FORECAST.queued);
  });

  it("is the act's own word when any row goes straight through", () => {
    expect(bulkForecast(plan("assign_role", ["apply", "queued"]))).toBe(FORECAST.give);
    expect(bulkForecast(plan("remove_bundle", ["apply"]))).toBe(FORECAST.remove);
  });

  it("uses the surface's word where the op has none, and says nothing where there is none", () => {
    expect(bulkForecast(plan("create_mapping", ["apply"]), FORECAST.adopt)).toBe(FORECAST.adopt);
    expect(bulkForecast(plan("create_mapping", ["apply"]))).toBeUndefined();
  });

  it("carries no number", () => {
    expect(bulkForecast(plan("assign_role", ["apply", "apply", "apply"]))).not.toMatch(/\d/);
  });
});

const rail: Indicators = {
  pending_requests: 5,
  expiring_grants: 1,
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

describe("the rail's count follows the answer", () => {
  it("never goes below zero, and writes nothing where it has read nothing", () => {
    settleIndicators(client, { drift: -99 });
    expect(read().drift).toBe(0);
    const empty = new QueryClient();
    settleIndicators(empty, { drift: -1 });
    expect(empty.getQueryData(["governance", "indicators"])).toBeUndefined();
  });

  it("deciding a request: unchanged while out, one fewer on success", async () => {
    const { result } = renderHook(() => useDecideRequest(), { wrapper });
    let done!: Promise<unknown>;
    act(() => {
      done = result.current.mutateAsync({ id: "r1", status: "approved" });
    });
    await waitFor(() => expect(wire.pending).not.toBeNull());
    expect(read().pending_requests).toBe(5);
    await act(async () => {
      wire.pending!.resolve({});
      await done;
    });
    expect(read().pending_requests).toBe(4);
    expect(read().pending_propagation).toBe(3);
  });

  it("an approval that waits also joins Pending changes", async () => {
    const { result } = renderHook(() => useDecideRequest(), { wrapper });
    let done!: Promise<unknown>;
    act(() => {
      done = result.current.mutateAsync({ id: "r1", status: "approved" });
    });
    await waitFor(() => expect(wire.pending).not.toBeNull());
    await act(async () => {
      wire.pending!.resolve({ wait_reason: "role_waits" });
      await done;
    });
    expect(read().pending_requests).toBe(4);
    expect(read().pending_propagation).toBe(4);
  });

  it("a failed decision leaves the rail exactly as it was", async () => {
    const { result } = renderHook(() => useDecideRequest(), { wrapper });
    let done!: Promise<unknown>;
    act(() => {
      done = result.current.mutateAsync({ id: "r1", status: "approved" }).catch(() => "failed");
    });
    await waitFor(() => expect(wire.pending).not.toBeNull());
    await act(async () => {
      wire.pending!.reject(new Error("boom"));
      await done;
    });
    expect(read()).toEqual(rail);
  });

  it("a Drift resolution: one fewer on drift, on success only", async () => {
    const { result } = renderHook(() => useRevokeDrift(), { wrapper });
    let done!: Promise<unknown>;
    act(() => {
      done = result.current.mutateAsync({ id: "d1" });
    });
    await waitFor(() => expect(wire.pending).not.toBeNull());
    expect(read().drift).toBe(7);
    await act(async () => {
      wire.pending!.resolve({});
      await done;
    });
    expect(read().drift).toBe(6);
  });

  it("lifting a hold: one fewer due", async () => {
    const { result } = renderHook(() => useLiftHold(), { wrapper });
    let done!: Promise<unknown>;
    act(() => {
      done = result.current.mutateAsync("h1");
    });
    await waitFor(() => expect(wire.pending).not.toBeNull());
    expect(read().holds_due).toBe(2);
    await act(async () => {
      wire.pending!.resolve({ status: "lifted" });
      await done;
    });
    expect(read().holds_due).toBe(1);
  });
});
