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

function open(extra: { definitionLabel?: string } = {}) {
  return render(
    <RehearsalDialog
      title="Grant a role"
      lede="1 person selected."
      noun={["person", "people"]}
      onRehearse={onRehearse}
      onApply={onApply}
      onClose={() => {}}
      {...extra}
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
    expect(banner).toHaveTextContent("This is a fresh preview");
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
    expect(banner).toHaveTextContent("Nothing was applied a second time");
    expect(banner).toHaveTextContent("fresh preview");
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
    expect(screen.getByRole("button", { name: /Preview the change for 63 people/ })).toBeInTheDocument();
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
    expect(screen.getByText(/I understand this changes access for/)).toHaveTextContent("63 people");
    expect(screen.getByRole("button", { name: /Preview the change for 63 people/ })).toBeDisabled();

    fireEvent.click(tick);
    expect(screen.getByRole("button", { name: /Preview the change for 63 people/ })).toBeEnabled();
  });

  // It computes a plan and writes nothing, so it is not the solid red fill that
  // is reserved for the button which performs a destruction.
  it("does not dress computing a plan as a destructive confirm", async () => {
    onRehearse = vi.fn().mockRejectedValue(refusal());
    open();
    await screen.findByRole("status");

    expect(screen.getByRole("button", { name: /Preview the change for 63 people/ }).className).not.toMatch(
      /bg-danger\b/,
    );
  });

  it("re-rehearses with the acknowledgement, and only then", async () => {
    onRehearse = vi.fn().mockRejectedValueOnce(refusal()).mockResolvedValueOnce(plan());
    open();
    await screen.findByRole("status");

    expect(vi.mocked(onRehearse)).toHaveBeenNthCalledWith(1, false);
    fireEvent.click(screen.getByRole("checkbox"));
    fireEvent.click(screen.getByRole("button", { name: /Preview the change for 63 people/ }));
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
    expect(note).toMatch(/sends it on its own/i);
    expect(note).toMatch(/the person still has the access/i);
  });

  it("tells a grant it is waiting for someone", () => {
    const note = queuedNote({
      op: "assign_role",
      applied: true,
      outcomes: [],
      summary: { total: 1, apply: 1, no_change: 0, blocked: 0, failed: 0, succeeded: 0, queued: 1 },
    });
    expect(note).toMatch(/Pending changes/);
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

    expect(await screen.findByText(/The list of people did not load/)).toBeInTheDocument();
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
    expect(apply).toHaveTextContent("Apply the change");
    expect(apply.textContent).not.toMatch(/undefined|NaN/);
  });
});

/**
 * A change that reaches nobody, on a surface where that is a real act.
 *
 * The dialog assumed every plan moves at least one person. That is right for a
 * bulk grant — a plan for nobody means nobody was selected — and wrong for the
 * mapping surfaces, where a change to a role nobody holds alters what the role
 * WILL confer. The backend writes those and issues no approval, because there
 * is nothing to review; the dialog then disabled Apply on both counts.
 *
 * Defining before assigning is the ordinary order, and it was the one order
 * this dialog could not express — for creates, edits, deletes and rollbacks
 * alike, since all four share it.
 */
describe("applying a change that reaches nobody", () => {
  const empty = (): BulkPlan =>
    ({
      ...plan(),
      plan_id: undefined,
      outcomes: [],
      summary: { total: 0, apply: 0, no_change: 0, blocked: 0, failed: 0, succeeded: 0, queued: 0 },
    }) as unknown as BulkPlan;

  it("is refused by default, because for most surfaces it is a no-op", async () => {
    onRehearse = vi.fn().mockResolvedValue(empty());
    open();

    const apply = await screen.findByRole("button", { name: /Nothing to apply/ });
    expect(apply).toBeDisabled();
  });

  it("is offered where the surface says a definition is an act", async () => {
    onRehearse = vi.fn().mockResolvedValue(empty());
    open({ definitionLabel: "Save mapping" });

    const save = await screen.findByRole("button", { name: "Save mapping" });
    expect(save).toBeEnabled();

    fireEvent.click(save);
    // Applied without an approval, because none was issued.
    await waitFor(() => expect(vi.mocked(onApply)).toHaveBeenCalledWith(""));
  });

  /**
   * "Reaches nobody" is three conditions, and `apply === 0` is only one of
   * them.
   *
   * A plan can count forty people and change nothing for any of them: forty
   * rows, every one `no_change`, an apply count of zero. That is not a
   * definition — it reaches forty people — and reading it as one would put the
   * definition label on it, submit it with no citation, and meet a backend
   * refusal the label had just promised would not happen.
   *
   * The backend is not fooled either way; it rechecks holders and refuses a
   * missing citation. What is at stake is the UI telling an operator the wrong
   * thing about what they are doing, and then being contradicted.
   */
  describe("what counts as reaching nobody", () => {
    const shaped = (over: Record<string, unknown>): BulkPlan =>
      ({ ...empty(), ...over }) as unknown as BulkPlan;

    it("does not read forty unchanged people as a definition", async () => {
      onRehearse = vi.fn().mockResolvedValue(
        shaped({
          outcomes: Array.from({ length: 40 }, (_, i) => ({
            user_id: `u${i}`,
            effect: "no_change",
            detail: "already has it",
          })),
          summary: { total: 40, apply: 0, no_change: 40, blocked: 0, failed: 0, succeeded: 0, queued: 0 },
        }),
      );
      open({ definitionLabel: "Save mapping" });

      expect(await screen.findByRole("button", { name: /Nothing to apply/ })).toBeDisabled();
      expect(screen.queryByRole("button", { name: "Save mapping" })).toBeNull();
    });

    // An empty list with a non-zero total is a plan this dialog does not
    // understand. The safe reading of one of those is the ordinary path.
    it("does not read an empty list with a count as a definition", async () => {
      onRehearse = vi.fn().mockResolvedValue(
        shaped({
          outcomes: [],
          summary: { total: 40, apply: 0, no_change: 40, blocked: 0, failed: 0, succeeded: 0, queued: 0 },
        }),
      );
      open({ definitionLabel: "Save mapping" });

      expect(await screen.findByRole("button", { name: /Nothing to apply/ })).toBeDisabled();
    });

    // An absent array is a payload that arrived short, not an empty cohort.
    it("does not read a payload that arrived short as a definition", async () => {
      onRehearse = vi.fn().mockResolvedValue(shaped({ outcomes: undefined }));
      open({ definitionLabel: "Save mapping" });

      expect(await screen.findByRole("button", { name: /Nothing to apply/ })).toBeDisabled();
      expect(screen.getByText(/The list of people did not load/)).toBeInTheDocument();
    });
  });

  // The relaxation is exact. A surface that can define is still a surface that
  // must cite an approval the moment its change reaches somebody.
  it("does not relax the citation once the change reaches somebody", async () => {
    onRehearse = vi
      .fn()
      .mockResolvedValue({ ...plan(), plan_id: undefined } as unknown as BulkPlan);
    open({ definitionLabel: "Save mapping" });

    const apply = await screen.findByRole("button", { name: /^Apply/ });
    expect(apply).toBeDisabled();
    expect(vi.mocked(onApply)).not.toHaveBeenCalled();
  });
});
