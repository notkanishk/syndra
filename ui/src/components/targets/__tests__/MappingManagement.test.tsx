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
  /** Whatever the rehearsal should carry beyond the plan — `value_checked`. */
  rehearsalExtras: Record<string, unknown>;
} = {
  mappings: [mapping],
  holders: [],
  history: { target: "truenas", current_version: 0, unpublished: false, versions: [] },
  rehearsals: [],
  rehearsalExtras: {},
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
        ...state.rehearsalExtras,
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
  state.rehearsalExtras = {};
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

    // It rehearses now, like every other change on this screen. It used to
    // acknowledge a BINDING count instead — an honest number at the time,
    // because no endpoint could tell it how many people the set moved. One can,
    // so the ceremony is the plan and the count is people.
    expect(screen.getByRole("dialog", { name: /Roll back to version 1/i })).toBeTruthy();
    expect(screen.getByText(/it does not merge one/)).toBeTruthy();
    // And the thing an operator would otherwise not think to check: a rollback
    // reaches the people it takes a mapping AWAY from, not only those it gives
    // one to.
    expect(screen.getByText(/not only those who gain one/)).toBeTruthy();
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

/**
 * Design M4's pair, and the half that would otherwise read as a bug.
 *
 * The value check fails open on everything except a definite no: an unreadable
 * target, a field the add-on cannot enumerate, and an unregistered add-on all
 * pass, because refusing an edit while a NAS reboots would make an outage look
 * like the operator's mistake.
 *
 * That is right, and it was invisible. "Checked and fine" and "nobody could be
 * asked" both arrived as a plain success, so the screen could not tell an
 * operator which of the two it was showing them.
 */
describe("a value the target could not be asked about", () => {
  it("says the check did not run, and why the edit was allowed through anyway", async () => {
    state.rehearsalExtras = { value_checked: false };
    renderMappings();

    fireEvent.click(screen.getByRole("button", { name: "Change" }));
    fireEvent.change(screen.getByRole("textbox", { name: /new value/i }), {
      target: { value: "archive-write" },
    });
    fireEvent.click(screen.getByRole("button", { name: /rehearse/i }));

    await waitFor(() => expect(screen.getByText(/could not be asked whether/)).toBeTruthy());
    expect(screen.getByText(/make an outage look like your mistake/)).toBeTruthy();
    // And it says where the consequence lands instead of here.
    expect(screen.getByText(/queued change that will not settle/)).toBeTruthy();
  });

  it("says nothing of the sort when the target answered", async () => {
    state.rehearsalExtras = { value_checked: true };
    renderMappings();

    fireEvent.click(screen.getByRole("button", { name: "Change" }));
    fireEvent.change(screen.getByRole("textbox", { name: /new value/i }), {
      target: { value: "archive-write" },
    });
    fireEvent.click(screen.getByRole("button", { name: /rehearse/i }));

    await waitFor(() => expect(screen.getByText(/stay owned by it/)).toBeTruthy());
    expect(screen.queryByText(/could not be asked whether/)).toBeNull();
  });
});

/**
 * One publish control, and it enforces what it says.
 *
 * The band and the history panel both owned a note field and a Publish button.
 * Two of each on one screen is two things that can disagree, and they did: the
 * panel refused a blank note and the band published with one. A reader working
 * out which is authoritative has already lost.
 *
 * The note is the only record of why a set was the right one, and its whole
 * reader is somebody months later deciding whether to roll back to it. A blank
 * one makes the version a date with no argument.
 */
describe("publishing a version", () => {
  it("is offered in one place, not two", () => {
    state.history = {
      target: "truenas",
      current_version: 0,
      unpublished: true,
      versions: [],
    };
    renderMappings();

    expect(screen.queryAllByRole("button", { name: /^Publish/ })).toHaveLength(0);
    expect(screen.queryAllByPlaceholderText(/Why this set is the one to keep/)).toHaveLength(0);
  });
});
