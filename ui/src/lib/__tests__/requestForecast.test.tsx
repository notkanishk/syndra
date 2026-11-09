// @vitest-environment jsdom
import { QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { FORECAST } from "@/lib/outcome";
import { useRequestDecisions } from "@/lib/request-decisions";
import type { AccessRequest } from "@/lib/queries/useRequests";
import { actPlan, planClient } from "@/test-utils/plan";

/**
 * A-20 P8: the row seat's forecast for an approval is Syndra's own next step.
 * A role that waits forecasts the wait ("Will wait for you to send"), not
 * "Will be approved"; Decline is the same either way.
 */
const wire = vi.hoisted(() => ({
  waits: false,
  settle: null as null | { resolve: (v: unknown) => void; reject: (e: unknown) => void },
}));
// Approve reads the planner's plan before the press (A-21 §3).
vi.mock("@/lib/api-client", async (original) => ({
  ...(await original<typeof import("@/lib/api-client")>()),
  request: async (path: string) => {
    if (path.includes("/plan?")) return actPlan();
    throw new Error(`unmocked request ${path}`);
  },
}));
vi.mock("@/lib/queries/useRoleWaits", () => ({ useRoleWaits: () => () => wire.waits }));
vi.mock("@/lib/queries/useRequests", async (original) => ({
  ...(await original<typeof import("@/lib/queries/useRequests")>()),
  useDecideRequest: () => ({
    isPending: false,
    mutateAsync: () => new Promise((resolve, reject) => (wire.settle = { resolve, reject })),
  }),
}));
vi.mock("@/lib/queries/useNameResolver", () => ({
  useNameResolver: () => ({
    resolveUser: () => ({ value: { display_name: "Charlotte Flair" }, resolved: true }),
    resolveRole: () => ({ value: { display_name: "Trained" }, resolved: true }),
    resolveProject: () => ({ value: { name: "Laser" }, resolved: true }),
    resolveBundle: () => ({ value: undefined, resolved: true }),
  }),
}));

const entry = {
  id: "r1",
  requester_id: "u1",
  project_id: "pLaser",
  role_key: "trained",
  status: "pending",
  created_at: "2026-07-22T06:00:00Z",
} as AccessRequest;

let client = planClient();
const wrapper = ({ children }: { children: ReactNode }) => (
  <QueryClientProvider client={client}>{children}</QueryClientProvider>
);

beforeEach(() => {
  wire.waits = false;
  wire.settle = null;
  client = planClient();
});

async function press(next: "approved" | "rejected") {
  const { result } = renderHook(() => useRequestDecisions(), { wrapper });
  act(() => {
    void result.current.act(entry, next);
  });
  await waitFor(() => expect(wire.settle).not.toBeNull());
  return result;
}

describe("a request row's forecast", () => {
  it("says 'Will be approved' for an ordinary role", async () => {
    const result = await press("approved");
    expect(result.current.forecasts.r1).toBe(FORECAST.approve);
  });

  it("says 'Will wait for you to send' for a role that waits, and never 'Will be approved'", async () => {
    wire.waits = true;
    const result = await press("approved");
    expect(result.current.forecasts.r1).toBe("Will wait for you to send");
    expect(result.current.forecasts.r1).not.toBe(FORECAST.approve);
  });

  it("says 'Will be declined' either way: a decline reaches nothing", async () => {
    wire.waits = true;
    const result = await press("rejected");
    expect(result.current.forecasts.r1).toBe(FORECAST.decline);
  });

  it("drops the forecast when the answer arrives", async () => {
    const result = await press("approved");
    await act(async () => {
      wire.settle!.resolve({});
    });
    await waitFor(() => expect(result.current.forecasts.r1).toBeUndefined());
  });
});
