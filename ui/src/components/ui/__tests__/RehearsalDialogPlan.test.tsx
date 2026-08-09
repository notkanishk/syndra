// @vitest-environment jsdom
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { RehearsalDialog } from "@/components/ui/RehearsalDialog";
import { ApiError } from "@/lib/api-client";
import type { BulkPlan } from "@/lib/queries/useBulkGrants";

vi.mock("sonner", () => ({
  toast: Object.assign(vi.fn(), { success: vi.fn(), warning: vi.fn(), error: vi.fn() }),
}));

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
      { user_id: "u1", name: "Ada", email: "ada@x.edu", effect: "apply", detail: "Gains trained." },
    ],
    summary: { total: 1, apply: 1, no_change: 0, blocked: 0, failed: 0, succeeded: 0, queued: 0 },
    ...overrides,
  };
}

const applied = (): BulkPlan =>
  plan({
    plan_id: undefined,
    applied: true,
    outcomes: [{ user_id: "u1", name: "Ada", email: "ada@x.edu", effect: "applied", detail: "Done." }],
    summary: { total: 1, apply: 0, no_change: 0, blocked: 0, failed: 0, succeeded: 1, queued: 0 },
  });

let onRehearse: ReturnType<typeof vi.fn>;
let onApply: ReturnType<typeof vi.fn>;

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
    await waitFor(() => expect(onApply).toHaveBeenCalledWith("plan_1"));
  });

  it("cannot apply a rehearsal the backend did not record", async () => {
    onRehearse = vi.fn(async () => plan({ plan_id: undefined }));
    open();
    // Offering a button that can only fail is worse than not offering it.
    await waitFor(() =>
      expect(screen.getByRole("button", { name: "Apply to 1 person" })).toBeDisabled(),
    );
    expect(onApply).not.toHaveBeenCalled();
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
    await waitFor(() => expect(onRehearse).toHaveBeenCalledTimes(2));

    fireEvent.click(screen.getByRole("button", { name: "Apply to 1 person" }));
    await waitFor(() => expect(onApply).toHaveBeenCalledTimes(2));
    expect(onApply).toHaveBeenNthCalledWith(1, "plan_1");
    expect(onApply).toHaveBeenNthCalledWith(2, "plan_2");
  });
});

describe("a stale approval", () => {
  const staleCodes = [
    "PLAN_STALE",
    "PLAN_EXPIRED",
    "PLAN_NOT_FOUND",
    "PLAN_ALREADY_APPLIED",
    "PLAN_REQUEST_MISMATCH",
  ];

  it.each(staleCodes)("re-plans rather than failing, on %s", async (code) => {
    onApply = vi.fn().mockRejectedValue(new ApiError(409, { error: code, message: "no good" }));
    open();
    await screen.findByRole("button", { name: "Apply to 1 person" });

    fireEvent.click(screen.getByRole("button", { name: "Apply to 1 person" }));
    // Rehearsed once on open, and again to show current state.
    await waitFor(() => expect(onRehearse).toHaveBeenCalledTimes(2));
    // Still on review. The operator approved a diff, that diff is gone, and the
    // replacement is a new decision — applying it for them is the failure this
    // path exists to prevent.
    expect(screen.getByRole("button", { name: "Apply to 1 person" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Close" })).not.toBeInTheDocument();
    expect(onApply).toHaveBeenCalledTimes(1);
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

    const banner = await screen.findByRole("status");
    expect(banner).toHaveTextContent("u_moved");
    expect(banner).toHaveTextContent("u_also");
    expect(banner).toHaveTextContent("Nothing was applied");
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
    await screen.findByRole("status");

    fireEvent.click(screen.getByRole("button", { name: "Apply to 1 person" }));
    await screen.findByRole("button", { name: "Close" });
    expect(screen.queryByRole("status")).not.toBeInTheDocument();
  });

  it("leaves an ordinary failure alone rather than re-planning over it", async () => {
    onApply = vi
      .fn()
      .mockRejectedValue(new ApiError(500, { error: "DB_ERROR", message: "disk on fire" }));
    open();
    await screen.findByRole("button", { name: "Apply to 1 person" });

    fireEvent.click(screen.getByRole("button", { name: "Apply to 1 person" }));
    await waitFor(() => expect(onApply).toHaveBeenCalledTimes(1));
    // A backend fault is not a stale approval, and re-planning over it would
    // discard the approval the operator is still entitled to retry.
    expect(onRehearse).toHaveBeenCalledTimes(1);
    expect(screen.queryByRole("status")).not.toBeInTheDocument();
  });
});
