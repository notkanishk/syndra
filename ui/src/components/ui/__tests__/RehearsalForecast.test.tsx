// @vitest-environment jsdom
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { RehearsalDialog } from "@/components/ui/RehearsalDialog";
import { FORECAST } from "@/lib/outcome";
import type { BulkPlan } from "@/lib/queries/useBulkGrants";

/**
 * A-20 P8: a bulk apply forecasts while it is out. The rehearsal has already
 * marked which rows will wait, so that is the forecast's source: all rows
 * waiting says the wait, otherwise the act's own word. Words, never a count.
 */
const wire = vi.hoisted(() => ({ settle: null as null | { resolve: () => void; reject: (e: unknown) => void } }));

function plan(effects: Array<"apply" | "queued">, op: BulkPlan["op"] = "assign_role"): BulkPlan {
  return {
    op,
    plan_id: "plan_1",
    applied: false,
    outcomes: effects.map((effect, i) => ({ user_id: `u${i}`, name: `Person ${i}`, email: "", effect, detail: "" })),
    summary: { total: effects.length, apply: effects.length, no_change: 0, blocked: 0, failed: 0, succeeded: 0, queued: 0 },
  } as BulkPlan;
}

let rehearsal: BulkPlan;
const applied = (p: BulkPlan): BulkPlan => ({
  ...p,
  plan_id: undefined,
  applied: true,
  outcomes: p.outcomes.map((o) => ({ ...o, effect: "applied" as const })),
  summary: { ...p.summary, apply: 0, succeeded: p.outcomes.length },
});

function open(extra: { forecast?: string } = {}) {
  render(
    <RehearsalDialog
      title="Give a role"
      lede=""
      noun={["person", "people"]}
      onRehearse={async () => rehearsal}
      onApply={() => new Promise<BulkPlan>((resolve, reject) => (wire.settle = { resolve: () => resolve(applied(rehearsal)), reject }))}
      onClose={() => {}}
      {...extra}
    />,
  );
}

async function apply() {
  fireEvent.click(await screen.findByRole("button", { name: /^Apply to \d+ (person|people)$/ }));
}

beforeEach(() => {
  wire.settle = null;
});

describe("a bulk apply: the first 100ms", () => {
  it("says 'Will be given' while it is out, then the true word, and no number", async () => {
    rehearsal = plan(["apply", "apply"]);
    open();
    await apply();
    const word = await screen.findByText(FORECAST.give);
    expect(word.textContent).not.toMatch(/\d/);
    wire.settle!.resolve();
    await screen.findByRole("button", { name: "Close" });
    expect(screen.queryByText(FORECAST.give)).toBeNull();
  });

  it("says 'Will wait for you to send' when the plan marked every acting row as waiting", async () => {
    rehearsal = plan(["queued", "queued"]);
    open();
    await apply();
    // The plan's own rows wear the same words, so look at the forecast seat (a status).
    await waitFor(() =>
      expect(screen.getAllByRole("status").some((el) => el.textContent === FORECAST.queued)).toBe(true),
    );
    expect(screen.queryByText(FORECAST.give)).toBeNull();
    wire.settle!.resolve();
  });

  it("uses the surface's own word where the op has none (Drift's adopt, a request's decline)", async () => {
    rehearsal = plan(["apply"], "create_mapping");
    open({ forecast: FORECAST.adopt });
    await apply();
    expect(await screen.findByText(FORECAST.adopt)).toBeInTheDocument();
    wire.settle!.resolve();
  });

  it("shows the honest error and no forecast when the apply fails", async () => {
    rehearsal = plan(["apply"]);
    open();
    await apply();
    await screen.findByText(FORECAST.give);
    wire.settle!.reject(new Error("boom"));
    await waitFor(() => expect(screen.queryByText(FORECAST.give)).toBeNull());
    expect(screen.getByText(/Nothing was (changed|applied)/i)).toBeInTheDocument();
  });
});
