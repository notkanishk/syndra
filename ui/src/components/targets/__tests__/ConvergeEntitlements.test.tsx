// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ConvergeEntitlements } from "@/components/targets/ConvergeEntitlements";
import type { EntitlementApplyResult, EntitlementPlan } from "@/lib/queries/useEntitlements";

/**
 * §23 — plan, then apply, on the one endpoint whose plan can be computed
 * against state nobody could read.
 *
 * The two properties no other planned surface has: a provisional plan stays
 * applicable and must be labelled WITH THE AGE of what it was computed against,
 * and the apply is a queue receipt rather than a report of work done.
 */

const state: {
  plan: EntitlementPlan;
  applied: EntitlementApplyResult;
  applyCalls: Array<{ planId: string; subjectIds: string[] }>;
} = {
  plan: {
    op: "converge_entitlements",
    plan_id: "plan_1",
    applied: false,
    outcomes: [
      { user_id: "u1", name: "Ada", effect: "apply", detail: "Creates ada." },
      { user_id: "u2", name: "Leo", effect: "no_change", detail: "Already correct." },
    ],
    summary: { total: 2, apply: 1, no_change: 1, blocked: 0, failed: 0, succeeded: 0, queued: 0 },
    provisional: false,
    state_read_at: new Date(Date.now() - 2 * 60_000).toISOString(),
  },
  applied: {
    plan_id: "plan_1",
    target: "truenas",
    provisional: false,
    queued: [{ subject_id: "u1", outbox_id: "o1" }],
    summary: { total: 2, queued: 1, no_change: 1, blocked: 0, succeeded: 0 },
  },
  applyCalls: [],
};

vi.mock("@/lib/queries/useEntitlements", async () => {
  const actual = await vi.importActual<typeof import("@/lib/queries/useEntitlements")>(
    "@/lib/queries/useEntitlements",
  );
  return {
    ...actual,
    useRehearseEntitlements: () => ({ mutateAsync: async () => state.plan }),
    useApplyEntitlements: () => ({
      mutateAsync: async (input: { planId: string; subjectIds: string[] }) => {
        state.applyCalls.push(input);
        return state.applied;
      },
    }),
  };
});

function renderConverge() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <ConvergeEntitlements
        target="truenas"
        subjectIds={["u1", "u2"]}
        label="everybody holding maker"
        onClose={() => {}}
      />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  state.applyCalls = [];
  state.plan = { ...state.plan, provisional: false, truncated: false };
});

describe("converging a cohort", () => {
  it("counts the rows that need nothing rather than hiding them", async () => {
    renderConverge();
    // "This changes less than you think" is the most useful thing a plan can
    // say, and a screen that filtered the unchanged rows would leave an
    // operator wondering where the rest of their cohort went.
    await waitFor(() => expect(screen.getByText(/Already correct/)).toBeInTheDocument());
    expect(screen.getByText("Ada")).toBeInTheDocument();
    expect(screen.getByText("Leo")).toBeInTheDocument();
  });

  it("applies the plan id it was given, never the original submission", async () => {
    renderConverge();
    await waitFor(() => expect(screen.getByText(/Already correct/)).toBeInTheDocument());

    fireEvent.click(screen.getByRole("button", { name: /queue|apply/i }));
    await waitFor(() => expect(state.applyCalls.length).toBe(1));
    expect(state.applyCalls[0].planId).toBe("plan_1");
  });

  // The endpoint's `succeeded` is always zero, present precisely so a client
  // cannot default it. Nothing here has reached the target.
  it("never reports a convergence as done", async () => {
    renderConverge();
    await waitFor(() => expect(screen.getByText(/Already correct/)).toBeInTheDocument());

    fireEvent.click(screen.getByRole("button", { name: /queue|apply/i }));
    await waitFor(() => expect(state.applyCalls.length).toBe(1));

    const body = document.body.textContent ?? "";
    expect(body).not.toMatch(/\bdone\b/i);
    expect(body).toMatch(/queued/i);
  });

  // Labelled with the AGE, not merely with the word: "computed against
  // last-known state" with no number is a label nobody can act on.
  it("labels a provisional plan with how old the state it saw is", async () => {
    state.plan = {
      ...state.plan,
      provisional: true,
      state_read_at: new Date(Date.now() - 14 * 60_000).toISOString(),
    };
    renderConverge();

    await waitFor(() => expect(screen.getByText(/last state seen/i)).toBeInTheDocument());
    expect(screen.getByText(/14m ago/)).toBeInTheDocument();
  });

  // Unlike adoption, a provisional plan is still applicable: applying joins a
  // queue an operator can inspect, while adopting binds an identity. §31 A is
  // explicit that these two must not be unified.
  it("still lets a provisional plan be applied", async () => {
    state.plan = { ...state.plan, provisional: true };
    renderConverge();
    await waitFor(() => expect(screen.getByText(/Already correct/)).toBeInTheDocument());

    expect(screen.getByRole("button", { name: /queue|apply/i })).toBeEnabled();
  });

  it("says a truncated read is not the whole list", async () => {
    state.plan = { ...state.plan, truncated: true };
    renderConverge();

    await waitFor(() => expect(screen.getByText(/not the whole list/i)).toBeInTheDocument());
  });
});
