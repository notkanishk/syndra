// @vitest-environment jsdom
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { MappingManagement } from "@/components/targets/MappingManagement";
import type { BulkPlan } from "@/lib/queries/useBulkGrants";
import type { MappingHistory, RoleMapping } from "@/lib/queries/useMappings";

/**
 * A rollback, all the way through the dialog (design M7).
 *
 * The rehearsal used to hand back a plan with no `plan_id`. The shared dialog
 * disables Apply without one — correctly, since an apply cites the approval it
 * was shown — so every rollback that reached anybody was a dead end on screen,
 * while the endpoint itself would still change the mapping set for anyone
 * calling it directly. A ceremony only the UI performs is a suggestion; one the
 * UI cannot complete is worse than having none.
 */

const state = {
  rehearsed: null as BulkPlan | null,
  applied: [] as Array<{ version: number; planId: string }>,
};

const mapping: RoleMapping = {
  id: "m1",
  target: "truenas",
  project_id: "pLab",
  role_key: "maker",
  field: "group",
  value: "lab_makers",
  created_by: "op_1",
};

const history: MappingHistory = {
  target: "truenas",
  current_version: 3,
  unpublished: false,
  unpublished_changes: [],
  versions: [
    { version: 3, note: "now", published_by: "op_7", published_at: "2026-08-01T00:00:00Z", entries: [] },
    { version: 1, note: "before the split", published_by: "op_2", published_at: "2026-07-01T00:00:00Z", entries: [] },
  ],
};

vi.mock("@/lib/queries/useMappings", async () => {
  const actual = await vi.importActual<typeof import("@/lib/queries/useMappings")>(
    "@/lib/queries/useMappings",
  );
  return {
    ...actual,
    useMappings: () => ({ data: [mapping], isLoading: false, error: null, refetch: vi.fn() }),
    useMappingHistory: () => ({ data: history, isLoading: false, error: null, refetch: vi.fn() }),
    useMappingHolders: () => ({ data: [], isLoading: false }),
    usePublishMappingVersion: () => ({ mutate: vi.fn(), isPending: false, error: null }),
    useRollbackMappingVersion: () => ({
      mutate: (
        input: { version: number; planId: string },
        opts?: { onSuccess?: (r: unknown) => void },
      ) => {
        state.applied.push(input);
        opts?.onSuccess?.({ status: "rolled_back", version: input.version, queued_convergences: 2 });
      },
      isPending: false,
    }),
    rehearseMappingRollback: vi.fn(async () => state.rehearsed as BulkPlan),
  };
});

vi.mock("@/components/names", () => ({
  UserName: ({ id }: { id: string }) => <span>{id}</span>,
  RoleRef: ({ roleKey }: { roleKey: string }) => <span>{roleKey}</span>,
}));

beforeEach(() => {
  state.applied = [];
  state.rehearsed = {
    op: "rollback_mappings",
    plan_id: "plan_61a8",
    applied: false,
    outcomes: [
      { user_id: "u1", effect: "apply", detail: "resolved again from version 1" },
      { user_id: "u2", effect: "apply", detail: "resolved again from version 1" },
    ],
    summary: { total: 2, apply: 2, no_change: 0, blocked: 0, failed: 0, succeeded: 0, queued: 0 },
  } as unknown as BulkPlan;
});

describe("rolling back, through the dialog", () => {
  it("can be applied, and cites the approval it was shown", async () => {
    render(<MappingManagement target="truenas" />);

    fireEvent.click(screen.getAllByRole("button", { name: /roll back to this/i })[0]);
    const apply = await screen.findByRole("button", { name: /^Apply/ });
    expect(apply.hasAttribute("disabled")).toBe(false);

    fireEvent.click(apply);
    await waitFor(() => expect(state.applied.length).toBe(1));
    expect(state.applied[0]).toEqual({ version: 1, planId: "plan_61a8" });
  });

  // The dead end this test exists to prevent from returning.
  it("cannot be applied when the rehearsal hands back no approval", async () => {
    state.rehearsed = { ...(state.rehearsed as BulkPlan), plan_id: undefined };
    render(<MappingManagement target="truenas" />);

    fireEvent.click(screen.getAllByRole("button", { name: /roll back to this/i })[0]);
    const apply = await screen.findByRole("button", { name: /^Apply/ });

    expect(apply.hasAttribute("disabled")).toBe(true);
    expect(state.applied).toEqual([]);
  });
});
