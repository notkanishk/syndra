// @vitest-environment jsdom
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { RehearsalDialog, queuedNote } from "@/components/ui/RehearsalDialog";
import { ApiError } from "@/lib/api-client";
import type { BulkPlan } from "@/lib/queries/useBulkGrants";

/**
 * 3.8/3.9 — the dialog holds the approval and recovers from a stale one.
 *
 * Tested here rather than per surface on purpose: the plan id and the
 * stale-plan path live in this component precisely so three surfaces cannot
 * each grow their own version, and the version that would go wrong is the one
 * that silently re-plans and applies the new diff as though it had been read.
 */

function plan(overrides: Partial<BulkPlan> = {}): BulkPlan {
  return {
    op: "assign_role",
    plan_id: "plan_1",
    applied: false,
    outcomes: [
      { user_id: "u1", name: "Ada", email: "ada@example.edu", effect: "apply", detail: "Gains trained." },
    ],
    summary: { total: 1, apply: 1, no_change: 0, blocked: 0, failed: 0, succeeded: 0, queued: 0 },
    ...overrides,
  };
}

const applied = (): BulkPlan =>
  plan({
    plan_id: undefined,
    applied: true,
    outcomes: [{ user_id: "u1", name: "Ada", email: "ada@example.edu", effect: "applied", detail: "Done." }],
    summary: { total: 1, apply: 0, no_change: 0, blocked: 0, failed: 0, succeeded: 1, queued: 0 },
  });

let onRehearse: (acknowledgeScope: boolean) => Promise<BulkPlan>;
let onApply: (planId: string) => Promise<BulkPlan>;

function open() {
  return render(
    <RehearsalDialog
      title="Grant a role"
      lede="1 person selected."
      noun={["person", "people"]}
      onRehearse={onRehearse}
      onApply={onApply}
      onClose={() => {}}
    />,
  );
}

beforeEach(() => {
  onRehearse = vi.fn(async () => plan());
  onApply = vi.fn(async () => applied());
});

describe("the approval the dialog holds", () => {
  it("applies the id the rehearsal returned, not one the caller composed", async () => {
    open();
    await screen.findByRole("button", { name: "Apply to 1 person" });

    fireEvent.click(screen.getByRole("button", { name: "Apply to 1 person" }));
    await waitFor(() => expect(vi.mocked(onApply)).toHaveBeenCalledWith("plan_1"));
  });

  it("cannot apply a rehearsal the backend did not record", async () => {
    onRehearse = vi.fn(async () => plan({ plan_id: undefined }));
    open();
    // Offering a button that can only fail is worse than not offering it.
    await waitFor(() =>
      expect(screen.getByRole("button", { name: "Apply to 1 person" })).toBeDisabled(),
    );
    expect(vi.mocked(onApply)).not.toHaveBeenCalled();
  });

  it("cites the id from the CURRENT plan after a re-plan, never the spent one", async () => {
    onApply = vi
      .fn()
      .mockRejectedValueOnce(
        new ApiError(409, { error: "PLAN_STALE", message: "moved", details: { u1: "moved" } }),
      )
      .mockResolvedValueOnce(applied());
    onRehearse = vi.fn().mockResolvedValueOnce(plan()).mockResolvedValueOnce(plan({ plan_id: "plan_2" }));

    open();
    await screen.findByRole("button", { name: "Apply to 1 person" });
    fireEvent.click(screen.getByRole("button", { name: "Apply to 1 person" }));
    await waitFor(() => expect(vi.mocked(onRehearse)).toHaveBeenCalledTimes(2));

    fireEvent.click(screen.getByRole("button", { name: "Apply to 1 person" }));
    await waitFor(() => expect(vi.mocked(onApply)).toHaveBeenCalledTimes(2));
    expect(vi.mocked(onApply)).toHaveBeenNthCalledWith(1, "plan_1");
    expect(vi.mocked(onApply)).toHaveBeenNthCalledWith(2, "plan_2");
  });
});

describe("a stale approval", () => {
  const staleCodes = [
    "PLAN_STALE",
    "PLAN_EXPIRED",
    "PLAN_NOT_FOUND",
    "PLAN_ALREADY_APPLIED",
    "PLAN_REQUEST_MISMATCH",
    // The sixth. Missing from the set, it fell through to a bare toast — the
    // one recovery path the rehearsal exists to close.
    "PLAN_NOT_CITABLE_HERE",
    // And the seventh: a re-plan is issued to the current operator, so it
    // resolves this refusal rather than only reporting it.
    "PLAN_NOT_YOURS",
  ];

  it.each(staleCodes)("re-plans rather than failing, on %s", async (code) => {
    onApply = vi.fn().mockRejectedValue(new ApiError(409, { error: code, message: "no good" }));
    open();
    await screen.findByRole("button", { name: "Apply to 1 person" });

    fireEvent.click(screen.getByRole("button", { name: "Apply to 1 person" }));
    // Rehearsed once on open, and again to show current state.
    await waitFor(() => expect(vi.mocked(onRehearse)).toHaveBeenCalledTimes(2));
    // Still on review. The operator approved a diff, that diff is gone, and the
    // replacement is a new decision — applying it for them is the failure this
    // path exists to prevent.
    expect(screen.getByRole("button", { name: "Apply to 1 person" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Close" })).not.toBeInTheDocument();
    expect(vi.mocked(onApply)).toHaveBeenCalledTimes(1);
  });

  it("names the rows that moved rather than reporting a generic failure", async () => {
    onApply = vi.fn().mockRejectedValue(
      new ApiError(409, {
        error: "PLAN_STALE",
        message: "changed",
        details: { u_moved: "moved", u_also: "moved" },
      }),
    );
    open();
    await screen.findByRole("button", { name: "Apply to 1 person" });
    fireEvent.click(screen.getByRole("button", { name: "Apply to 1 person" }));

    const banner = await screen.findByRole("alert");
    expect(banner).toHaveTextContent("u_moved");
    expect(banner).toHaveTextContent("u_also");
    expect(banner).toHaveTextContent("Nothing was applied");
  });

  // Five of the six codes carry no details, so keying the banner on the moved
  // list alone swapped the approved plan for a fresh one with nothing on screen
  // but nothing on screen saying so. An operator could then press Apply believing they
  // had already read this diff.
  it.each(staleCodes)("says the plan is new even when %s names no rows", async (code) => {
    onApply = vi.fn().mockRejectedValue(new ApiError(409, { error: code, message: "no good" }));
    open();
    await screen.findByRole("button", { name: "Apply to 1 person" });
    fireEvent.click(screen.getByRole("button", { name: "Apply to 1 person" }));

    const banner = await screen.findByRole("alert");
    expect(banner).toHaveTextContent("Nothing was applied");
    expect(banner).toHaveTextContent("review it again before applying");
  });

  // The one code where "nothing was applied" is false: the earlier apply
  // landed, and only the second attempt did nothing. The bold line is read
  // first, so it must not contradict the sentence under it.
  it("does not claim nothing happened when the plan had already been applied", async () => {
    onApply = vi
      .fn()
      .mockRejectedValue(new ApiError(409, { error: "PLAN_ALREADY_APPLIED", message: "spent" }));
    open();
    await screen.findByRole("button", { name: "Apply to 1 person" });
    fireEvent.click(screen.getByRole("button", { name: "Apply to 1 person" }));

    const banner = await screen.findByRole("alert");
    expect(banner).toHaveTextContent("Nothing was applied twice");
    expect(banner).toHaveTextContent("already applied once");
  });

  it("clears the notice once a fresh approval is applied", async () => {
    onApply = vi
      .fn()
      .mockRejectedValueOnce(
        new ApiError(409, { error: "PLAN_STALE", message: "moved", details: { u1: "moved" } }),
      )
      .mockResolvedValueOnce(applied());
    open();
    await screen.findByRole("button", { name: "Apply to 1 person" });

    fireEvent.click(screen.getByRole("button", { name: "Apply to 1 person" }));
    await screen.findByRole("alert");

    fireEvent.click(screen.getByRole("button", { name: "Apply to 1 person" }));
    await screen.findByRole("button", { name: "Close" });
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("leaves an ordinary failure alone rather than re-planning over it", async () => {
    onApply = vi
      .fn()
      .mockRejectedValue(new ApiError(500, { error: "DB_ERROR", message: "disk on fire" }));
    open();
    await screen.findByRole("button", { name: "Apply to 1 person" });

    fireEvent.click(screen.getByRole("button", { name: "Apply to 1 person" }));
    await waitFor(() => expect(vi.mocked(onApply)).toHaveBeenCalledTimes(1));
    // A backend fault is not a stale approval, and re-planning over it would
    // discard the approval the operator is still entitled to retry.
    expect(vi.mocked(onRehearse)).toHaveBeenCalledTimes(1);
    expect(screen.queryByRole("status")).not.toBeInTheDocument();
  });
});

describe("a change bigger than the usual one", () => {
  const refusal = () =>
    new ApiError(422, {
      error: "COHORT_ACKNOWLEDGEMENT_REQUIRED",
      message: "this affects 63 subjects",
      details: { affected: "63", limit: "25" },
    });

  it("stops on its own step with the count, rather than closing on a toast", async () => {
    onRehearse = vi.fn().mockRejectedValue(refusal());
    open();

    const notice = await screen.findByRole("status");
    // The number it computed. "Too large" leaves an operator guessing at what
    // they are being warned about.
    expect(notice).toHaveTextContent("63");
    expect(notice).toHaveTextContent("25");
    expect(screen.getByRole("button", { name: /Plan for 63 people/ })).toBeInTheDocument();
    // Nothing has been computed, so there is nothing to apply.
    expect(screen.queryByRole("button", { name: /^Apply/ })).not.toBeInTheDocument();
  });

  // Rung 2 is the tick, not the label. A button carrying the count is something
  // a hand reaches past; the ceremony has to be an act, and it has to be an act
  // about the number.
  it("holds the plan behind an acknowledgement carrying the count", async () => {
    onRehearse = vi.fn().mockRejectedValue(refusal());
    open();
    await screen.findByRole("status");

    const tick = screen.getByRole("checkbox");
    expect(tick).not.toBeChecked();
    expect(screen.getByText(/I understand this moves/)).toHaveTextContent("63 people");
    expect(screen.getByRole("button", { name: /Plan for 63 people/ })).toBeDisabled();

    fireEvent.click(tick);
    expect(screen.getByRole("button", { name: /Plan for 63 people/ })).toBeEnabled();
  });

  // It computes a plan and writes nothing, so it is not the solid red fill that
  // is reserved for the button which performs a destruction.
  it("does not dress computing a plan as a destructive confirm", async () => {
    onRehearse = vi.fn().mockRejectedValue(refusal());
    open();
    await screen.findByRole("status");

    expect(screen.getByRole("button", { name: /Plan for 63 people/ }).className).not.toMatch(
      /bg-danger\b/,
    );
  });

  it("re-rehearses with the acknowledgement, and only then", async () => {
    onRehearse = vi.fn().mockRejectedValueOnce(refusal()).mockResolvedValueOnce(plan());
    open();
    await screen.findByRole("status");

    expect(vi.mocked(onRehearse)).toHaveBeenNthCalledWith(1, false);
    fireEvent.click(screen.getByRole("checkbox"));
    fireEvent.click(screen.getByRole("button", { name: /Plan for 63 people/ }));
    await screen.findByRole("button", { name: "Apply to 1 person" });
    expect(vi.mocked(onRehearse)).toHaveBeenNthCalledWith(2, true);
  });

  it("does not make the operator confirm the size again to see what moved", async () => {
    onApply = vi
      .fn()
      .mockRejectedValue(new ApiError(409, { error: "PLAN_STALE", message: "moved", details: {} }));
    onRehearse = vi.fn(async () => plan());
    open();
    await screen.findByRole("button", { name: "Apply to 1 person" });

    fireEvent.click(screen.getByRole("button", { name: "Apply to 1 person" }));
    await waitFor(() => expect(vi.mocked(onRehearse)).toHaveBeenCalledTimes(2));
    // The cohort has not grown. Re-asking buries the thing they need to read.
    expect(vi.mocked(onRehearse)).toHaveBeenNthCalledWith(2, true);
  });
});

// §7: an operator who has just approved something must never have to know which
// drain rule applied. Two rules drain this queue and they are not symmetric —
// withdrawals leave on a background runner, everything conferring access waits
// for a human — so the copy says what will happen rather than naming the rule.
describe("queued rows say what happens next", () => {
  it("tells a withdrawal it sends itself", () => {
    const note = queuedNote({
      op: "remove_role",
      applied: true,
      outcomes: [],
      summary: { total: 1, apply: 1, no_change: 0, blocked: 0, failed: 0, succeeded: 0, queued: 1 },
    });
    expect(note).toMatch(/send themselves|leaves within/i);
    expect(note).toMatch(/access holds until it does/i);
  });

  it("tells a grant it is waiting for someone", () => {
    const note = queuedNote({
      op: "assign_role",
      applied: true,
      outcomes: [],
      summary: { total: 1, apply: 1, no_change: 0, blocked: 0, failed: 0, succeeded: 0, queued: 1 },
    });
    expect(note).toMatch(/resume the queue/i);
  });

  it("says nothing when nothing is queued", () => {
    const note = queuedNote({
      op: "assign_role",
      applied: true,
      outcomes: [],
      summary: { total: 1, apply: 1, no_change: 0, blocked: 0, failed: 0, succeeded: 1, queued: 0 },
    });
    expect(note).toBeUndefined();
  });
});

/**
 * A plan that arrives without its rows.
 *
 * The list is read off a payload like every other list in the product, and every
 * other one is read with `?? []`. This one was not, so a short payload threw
 * inside render and the error boundary blanked the screen — on the surface an
 * operator is standing on halfway through approving a change that moves
 * somebody's access.
 *
 * The right failure is a plan with no rows beside its summary. The backend
 * being wrong is not a reason to lose the page.
 */
describe("a plan whose rows did not arrive", () => {
  it("renders without them rather than taking the screen down", async () => {
    onRehearse = vi
      .fn()
      .mockResolvedValue({ ...plan(), outcomes: undefined } as unknown as BulkPlan);
    open();

    expect(await screen.findByText(/came back without its rows/)).toBeInTheDocument();
    // And the approval is still reachable: the summary is what it reported.
    expect(screen.getByRole("button", { name: /^Apply/ })).toBeInTheDocument();
  });

  // The other half of the same payload problem, and the worse one: it does not
  // fail loudly, it puts a non-number in the label of the button that performs
  // the change.
  it("does not put a count it does not have on the apply button", async () => {
    onRehearse = vi.fn().mockResolvedValue({
      ...plan(),
      summary: { ...plan().summary, apply: undefined },
    } as unknown as BulkPlan);
    open();

    const apply = await screen.findByRole("button", { name: /^Apply/ });
    expect(apply).toHaveTextContent("Apply this plan");
    expect(apply.textContent).not.toMatch(/undefined|NaN/);
  });
});
