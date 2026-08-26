// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { MappingManagement } from "@/components/targets/MappingManagement";
import type { MappingHistory, RoleMapping } from "@/lib/queries/useMappings";

/**
 * §24 — the highest-leverage object in the system, and the ceremony sized to it.
 *
 * The properties under test are the ones that make a mapping edit safe to do
 * routinely: it rehearses first, the blast radius arrives as a refusal rather
 * than a checkbox drawn upfront, a rollback names what it restores, and nothing
 * anywhere claims a change reached the target.
 */

const mapping: RoleMapping = {
  id: "m1",
  target: "truenas",
  project_id: "pLab",
  role_key: "maker",
  field: "group",
  value: "lab_makers",
  created_by: "op_1",
};

const state: {
  mappings: RoleMapping[];
  holders: string[];
  history: MappingHistory;
  rehearsals: Array<{ acknowledgeScope: boolean }>;
} = {
  mappings: [mapping],
  holders: [],
  history: { target: "truenas", current_version: 0, unpublished: false, versions: [] },
  rehearsals: [],
};

vi.mock("@/lib/queries/useMappings", async () => {
  const actual = await vi.importActual<typeof import("@/lib/queries/useMappings")>(
    "@/lib/queries/useMappings",
  );
  return {
    ...actual,
    useMappings: () => ({ data: state.mappings, isLoading: false, error: null, refetch: vi.fn() }),
    useMappingHolders: () => ({
      data: { mapping, holders: state.holders, count: state.holders.length },
      isLoading: false,
    }),
    useMappingHistory: () => ({
      data: state.history,
      isLoading: false,
      error: null,
      refetch: vi.fn(),
    }),
    usePublishMappingVersion: () => ({ mutate: vi.fn(), isPending: false }),
    useRollbackMappingVersion: () => ({ mutate: vi.fn(), isPending: false }),
    rehearseMappingEdit: vi.fn(async (_id: string, _value: string, acknowledgeScope: boolean) => {
      state.rehearsals.push({ acknowledgeScope });
      return {
        op: "edit_mapping",
        plan_id: "plan_1",
        applied: false,
        outcomes: [
          {
            user_id: "u1",
            name: "Ada",
            email: "ada@example.org",
            effect: "apply",
            detail: "maker confers group = lab_x on truenas instead of lab_makers.",
          },
        ],
        summary: { total: 1, apply: 1, no_change: 0, blocked: 0, failed: 0, succeeded: 0, queued: 0 },
      };
    }),
    rehearseMappingDelete: vi.fn(async () => ({
      op: "delete_mapping",
      plan_id: "plan_2",
      applied: false,
      outcomes: [],
      summary: { total: 0, apply: 0, no_change: 0, blocked: 0, failed: 0, succeeded: 0, queued: 0 },
    })),
    applyMappingEdit: vi.fn(async () => ({ status: "updated", queued_convergences: 1 })),
    applyMappingDelete: vi.fn(async () => ({ status: "deleted", queued_convergences: 0 })),
  };
});

function renderMappings() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <MappingManagement target="truenas" />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  state.mappings = [mapping];
  state.holders = [];
  state.history = { target: "truenas", current_version: 0, unpublished: false, versions: [] };
  state.rehearsals = [];
});

describe("what roles reach a target", () => {
  it("says how many people each mapping moves, beside the mapping", () => {
    state.holders = ["u1", "u2", "u3"];
    renderMappings();

    expect(screen.getByText("group = lab_makers")).toBeInTheDocument();
    expect(screen.getByText("3 people")).toBeInTheDocument();
  });

  // The cohort is the reason this row can offer a convergence at all: the
  // entitlement endpoints take an explicit list of subjects, and a control that
  // assembled its own would be assembling one nobody reviewed.
  it("offers no convergence for a role nobody holds, and says why", () => {
    state.holders = [];
    renderMappings();

    expect(screen.queryByRole("button", { name: /bring accounts in line/i })).toBeNull();
    expect(screen.getByText(/Nobody holds this role/i)).toBeInTheDocument();
  });

  // Rehearse-then-apply, through the shared dialog, so an operator who has read
  // a bulk-grant plan already knows where the button is.
  it("rehearses an edit before applying it, and asks unacknowledged first", async () => {
    state.holders = ["u1"];
    renderMappings();

    fireEvent.click(screen.getByRole("button", { name: "Change" }));
    fireEvent.change(screen.getByRole("textbox", { name: /new value/i }), {
      target: { value: "lab_x" },
    });
    fireEvent.click(screen.getByRole("button", { name: /rehearse/i }));

    await waitFor(() => expect(state.rehearsals.length).toBeGreaterThan(0));
    // The blast radius is a backend refusal, not a checkbox drawn upfront: the
    // first ask is always unacknowledged, and the ceremony appears only when
    // the backend says the change is bigger than the usual one.
    expect(state.rehearsals[0].acknowledgeScope).toBe(false);
  });
});

describe("version history", () => {
  it("names who published each version and why", () => {
    state.history = {
      target: "truenas",
      current_version: 2,
      unpublished: false,
      versions: [
        {
          version: 2,
          note: "after the laser lab split",
          published_by: "op_7",
          published_at: "2026-08-01T00:00:00Z",
          entries: [{ project_id: "pLab", role_key: "maker", field: "group", value: "lab_makers" }],
        },
      ],
    };
    renderMappings();

    // A rollback target with no reason attached is a guess.
    expect(screen.getByText(/after the laser lab split/)).toBeInTheDocument();
    expect(screen.getByText("Version 2")).toBeInTheDocument();
    expect(screen.getByText("current")).toBeInTheDocument();
  });

  // "Current version 2" is true and misleading when what is live is version 2
  // plus three edits: an operator rolling back from there undoes work that is
  // listed nowhere.
  it("says when the working copy has drifted from the newest version", () => {
    state.history = {
      target: "truenas",
      current_version: 2,
      unpublished: true,
      versions: [
        { version: 2, note: "n", published_by: "op_7", published_at: "2026-08-01T00:00:00Z", entries: [] },
      ],
    };
    renderMappings();

    expect(screen.getAllByText(/unpublished changes/i).length).toBeGreaterThan(0);
  });

  it("acknowledges what a rollback restores before it will run", () => {
    state.history = {
      target: "truenas",
      current_version: 3,
      unpublished: false,
      versions: [
        { version: 3, note: "now", published_by: "op_7", published_at: "2026-08-01T00:00:00Z", entries: [] },
        {
          version: 1,
          note: "before the split",
          published_by: "op_2",
          published_at: "2026-07-01T00:00:00Z",
          entries: [
            { project_id: "pLab", role_key: "maker", field: "group", value: "old_group" },
            { project_id: "pLab", role_key: "lead", field: "group", value: "leads" },
          ],
        },
      ],
    };
    renderMappings();

    fireEvent.click(screen.getAllByRole("button", { name: /roll back to this/i })[0]);

    // Rung 2: the number sits inside the sentence being ticked, and it is the
    // binding count rather than a head count — how many people it moves depends
    // on who holds those roles when the drain runs, and claiming a person count
    // would be claiming a rehearsal this endpoint does not do.
    // In the sentence being ticked, not merely somewhere on the row.
    expect(
      screen.getByText(/I understand this restores/i).textContent,
    ).toMatch(/2 bindings/);
    const confirm = screen.getByRole("button", { name: /roll back to version 1/i });
    expect(confirm).toBeDisabled();

    fireEvent.click(screen.getByRole("checkbox"));
    expect(confirm).toBeEnabled();
  });
});

/**
 * The consequence the plan's own numbers cannot state (design M3).
 *
 * A mapping edit moves a group. It does not move what the old group owns, and
 * Syndra has no way to: the files stay owned by the group that owned them, and
 * everybody who moved loses access to them until somebody on the target re-owns
 * them.
 *
 * No count implies that, so it is stated beside the plan rather than left to be
 * discovered afterwards by the thirty-four people it happens to.
 */
describe("changing what a role reaches · the consequence no count implies", () => {
  it("says the old group keeps its files, and names the target that must re-own them", async () => {
    renderMappings();

    fireEvent.click(screen.getByRole("button", { name: "Change" }));
    fireEvent.change(screen.getByRole("textbox", { name: /new value/i }), {
      target: { value: "lab_x" },
    });
    fireEvent.click(screen.getByRole("button", { name: /rehearse/i }));

    // On the REVIEW step, beside the plan — it is part of what is being
    // approved, not a caveat about the form that composed it.
    await waitFor(() => expect(screen.getByText(/stay owned by it/)).toBeTruthy());
    expect(screen.getByText(/re-owns them on TrueNAS/)).toBeTruthy();
    // The group it is leaving, named — "the old group" is not something an
    // operator can check against the NAS.
    expect(screen.getByText("lab_makers")).toBeTruthy();
  });
});
