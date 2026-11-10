// @vitest-environment jsdom
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ApproveButton, HoldingNote, keepLength } from "@/components/requests/Holding";
import type { AccessRequest } from "@/lib/queries/useRequests";
import type { ActPlan } from "@/lib/usePlanConsent";
import { actPlan, cancelling, queuedChange, withPlanClient } from "@/test-utils/plan";

// What the planner answers (GET /users/{id}/plan?op=approve), and the reads it was asked.
const wire = vi.hoisted(() => ({
  plan: undefined as unknown as ActPlan,
  calls: [] as string[],
  fail: false,
  hang: false,
}));

vi.mock("@/lib/api-client", async (original) => ({
  ...(await original<typeof import("@/lib/api-client")>()),
  request: (path: string) => {
    if (!path.includes("/plan?")) return Promise.reject(new Error(`unmocked request ${path}`));
    wire.calls.push(path);
    if (wire.hang) return new Promise(() => {});
    return wire.fail ? Promise.reject(new Error("plan unreadable")) : Promise.resolve(wire.plan);
  },
}));
vi.mock("@/lib/queries/useNameResolver", () => ({
  useNameResolver: () => ({
    resolveUser: () => ({ value: undefined, resolved: true }),
    resolveRole: () => ({ value: undefined, resolved: true }),
    resolveProject: () => ({ value: undefined, resolved: true }),
  }),
}));

function ask(overrides: Partial<AccessRequest> = {}): AccessRequest {
  return {
    id: "r1",
    requester_id: "u1",
    project_id: "p1",
    role_key: "operator",
    justification: "",
    status: "pending",
    created_at: "2026-11-01T00:00:00Z",
    holding: { kind: "none" },
    ...overrides,
  } as AccessRequest;
}

beforeEach(() => {
  wire.plan = actPlan();
  wire.calls = [];
  wire.fail = false;
  wire.hang = false;
});

// Audit C2: approving gives the role, and the automatic rules follow it, so
// the row the operator approves from names what they also give: the planner's
// sentences, and the waiting change approving would cancel (A-21 §3.2).
describe("approving, with automatic rules", () => {
  it("names what the rules also give before the approve", async () => {
    wire.plan = actPlan({ rules: ["Also gives Bench in Workshop, by the automatic rule from Operator."] });
    render(withPlanClient(<HoldingNote request={ask()} />));
    expect(
      await screen.findByText("If you approve: Also gives Bench in Workshop, by the automatic rule from Operator."),
    ).toBeInTheDocument();
    expect(wire.calls).toEqual(["/users/u1/plan?op=approve&request_id=r1"]);
  });

  it("says the removal approving cancels, after the rules, in the same line", async () => {
    wire.plan = cancelling([queuedChange()], { rules: ["Also gives Bench in Workshop, by the automatic rule from Operator."] });
    render(withPlanClient(<HoldingNote request={ask()} />));
    expect(
      await screen.findByText(
        /^If you approve: Also gives Bench in Workshop, by the automatic rule from Operator\. Approving cancels removing .+ from .+, which was waiting for you to send\.$/,
      ),
    ).toBeInTheDocument();
  });

  it("says only the cancel when no rule follows", async () => {
    wire.plan = cancelling([queuedChange()]);
    render(withPlanClient(<HoldingNote request={ask()} />));
    expect(await screen.findByText(/^If you approve: Approving cancels removing .+ from .+, which was waiting for you to send\.$/)).toBeInTheDocument();
  });

  it("says nothing when no rule follows, and does not ask about a role already held", async () => {
    const { container } = render(withPlanClient(<HoldingNote request={ask()} />));
    await waitFor(() => expect(wire.calls).toHaveLength(1));
    expect(container.textContent).toBe("");
    render(withPlanClient(<HoldingNote request={ask({ holding: { kind: "for_good" } })} />));
    expect(wire.calls).toHaveLength(1);
  });

  it("reads no rule preview any more: one plan read serves the line and the button", async () => {
    render(
      withPlanClient(
        <>
          <HoldingNote request={ask()} />
          <ApproveButton request={ask()} onClick={vi.fn()} />
        </>,
      ),
    );
    await waitFor(() => expect(screen.getByRole("button", { name: "Approve" })).toBeEnabled());
    expect(wire.calls).toEqual(["/users/u1/plan?op=approve&request_id=r1"]);
  });
});

// Final verification N5: what approving does is part of the consent, so
// Approve waits for the plan and is held, with why, when it can't be read.
describe("approving, while the plan is not read", () => {
  it("holds Approve while the plan loads", () => {
    wire.hang = true;
    render(withPlanClient(<ApproveButton request={ask()} onClick={vi.fn()} />));
    expect(screen.getByRole("button", { name: "Approve" })).toBeDisabled();
    expect(screen.getByText("Checking what approving does.")).toBeInTheDocument();
  });

  it("holds Approve when the plan fails, and says why, beside the row's own line", async () => {
    wire.fail = true;
    const click = vi.fn();
    render(
      withPlanClient(
        <>
          <HoldingNote request={ask()} />
          <ApproveButton request={ask()} onClick={click} />
        </>,
      ),
    );
    expect(await screen.findByText("Held until Syndra can read what approving does.")).toBeInTheDocument();
    const button = screen.getByRole("button", { name: "Approve" });
    expect(button).toBeDisabled();
    button.click();
    expect(click).not.toHaveBeenCalled();
    // The row says why and offers Try again; the button says it is held.
    expect(
      screen.getByText(
        "Syndra couldn't read what approving does, so it can't show what else this would give or cancel. Try again to read it.",
      ),
    ).toBeInTheDocument();
    wire.fail = false;
    fireEvent.click(screen.getByRole("button", { name: "Try again" }));
    await waitFor(() => expect(screen.getByRole("button", { name: "Approve" })).toBeEnabled());
  });

  it("is checking, not failed, while Try again reads the plan", async () => {
    wire.fail = true;
    render(
      withPlanClient(
        <>
          <HoldingNote request={ask()} />
          <ApproveButton request={ask()} onClick={vi.fn()} />
        </>,
      ),
    );
    fireEvent.click(await screen.findByRole("button", { name: "Try again" }));
    wire.hang = true;
    fireEvent.click(await screen.findByRole("button", { name: "Try again" }));
    await waitFor(() => expect(screen.queryByRole("button", { name: "Try again" })).toBeNull());
    expect(screen.getByRole("button", { name: "Approve" })).toBeDisabled();
    expect(screen.getByText("Checking what approving does.")).toBeInTheDocument();
  });

  it("lets Approve go once the plan is read", async () => {
    const click = vi.fn();
    render(withPlanClient(<ApproveButton request={ask()} onClick={click} />));
    await waitFor(() => expect(screen.getByRole("button", { name: "Approve" })).toBeEnabled());
    screen.getByRole("button", { name: "Approve" }).click();
    expect(click).toHaveBeenCalledTimes(1);
  });

  it("holds it, with the planner's reason, when the planner refuses the approval", async () => {
    wire.plan = actPlan({ effect: "blocked", detail: "Charlotte has left.", consequence: "Decline the request to close it." });
    render(withPlanClient(<ApproveButton request={ask()} onClick={vi.fn()} />));
    expect(await screen.findByText("Charlotte has left. Decline the request to close it.")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Approve" })).toBeDisabled();
  });

  it("still holds a role they already have, without asking the planner", () => {
    wire.fail = true;
    render(withPlanClient(<ApproveButton request={ask({ holding: { kind: "for_good" } })} onClick={vi.fn()} />));
    expect(screen.getByText("They already have this.")).toBeInTheDocument();
    expect(wire.calls).toEqual([]);
  });
});

// Audit S6: a term's last day is matched in the makerspace's day, not the
// browser's: an end at 00:00 IST on 27 Nov is 26 Nov in UTC.
describe("keepLength", () => {
  it("matches the term's last day in the makerspace's zone", () => {
    const zone = process.env.TZ;
    process.env.TZ = "UTC";
    try {
      const terms = [{ name: "Monsoon", start: "2026-08-03", end: "2026-11-27" }];
      expect(keepLength(ask({ kind: "extend", duration_days: 40, until: "2026-11-26T18:30:00Z" }), terms)).toBe(
        "To the end of next term",
      );
    } finally {
      process.env.TZ = zone;
    }
  });
});
