// @vitest-environment jsdom
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { AddMappingDialog } from "@/components/targets/AddMappingDialog";
import type { MappingRehearsal } from "@/lib/queries/useMappings";

/**
 * Adding a mapping (designs M1, M5).
 *
 * The last change on this screen that did not rehearse, and the one most easily
 * mistaken for harmless: a new mapping looks like writing a row down, and it is
 * a grant. Entitlements are derived from mappings, so everybody holding the
 * role becomes entitled the moment it exists — and nothing else finds them,
 * because the periodic reconciler walks existing bindings and somebody never
 * bound to this target is in no list it reads.
 */

const state = {
  rehearsals: [] as Array<{ input: Record<string, unknown>; acknowledgeScope: boolean }>,
  created: [] as Array<Record<string, unknown>>,
  rehearsed: null as MappingRehearsal | null,
};

vi.mock("@/lib/queries/useMappings", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/queries/useMappings")>()),
  rehearseMappingCreate: vi.fn(async (input: Record<string, unknown>, acknowledgeScope: boolean) => {
    state.rehearsals.push({ input, acknowledgeScope });
    return state.rehearsed;
  }),
  useCreateMapping: () => ({
    mutateAsync: vi.fn(async (input: Record<string, unknown>) => {
      state.created.push(input);
      return { mapping: { id: "m1" }, queued_convergences: 2 };
    }),
    isPending: false,
  }),
}));

vi.mock("@/lib/queries/useProjects", () => ({
  useProjects: () => ({
    data: [
      { project: { id: "pLab", name: "Laser Lab" }, active_role_keys: ["trained", "lead"] },
      { project: { id: "pEmpty", name: "Studio" }, active_role_keys: [] },
    ],
    isLoading: false,
    error: null,
  }),
}));

vi.mock("@/components/names", () => ({
  UserName: ({ id }: { id: string }) => <span>{id}</span>,
}));

beforeEach(() => {
  state.rehearsals = [];
  state.created = [];
  state.rehearsed = {
    op: "create_mapping",
    plan_id: "plan_61a8",
    applied: false,
    outcomes: [
      { user_id: "u1", effect: "apply", detail: "trained starts conferring group = makers" },
      { user_id: "u2", effect: "apply", detail: "trained starts conferring group = makers" },
    ],
    summary: { total: 2, apply: 2, no_change: 0, blocked: 0, failed: 0, succeeded: 0, queued: 0 },
    value_checked: true,
  } as unknown as MappingRehearsal;
});

function compose() {
  render(<AddMappingDialog target="truenas" onClose={vi.fn()} />);
  fireEvent.change(screen.getByLabelText("Project"), { target: { value: "pLab" } });
  fireEvent.change(screen.getByLabelText("Role"), { target: { value: "trained" } });
  fireEvent.change(screen.getByLabelText("Value"), { target: { value: "makers" } });
}

describe("adding a mapping", () => {
  it("rehearses before it writes anything", async () => {
    compose();
    fireEvent.click(screen.getByRole("button", { name: /preview/i }));

    await waitFor(() => expect(state.rehearsals.length).toBe(1));
    expect(state.rehearsals[0].input).toMatchObject({
      target: "truenas",
      projectId: "pLab",
      roleKey: "trained",
      field: "group",
      value: "makers",
    });
    // The blast radius is a backend refusal, not a checkbox drawn upfront.
    expect(state.rehearsals[0].acknowledgeScope).toBe(false);
    expect(state.created).toEqual([]);
  });

  it("applies the plan it was shown, citing it", async () => {
    compose();
    fireEvent.click(screen.getByRole("button", { name: /preview/i }));
    fireEvent.click(await screen.findByRole("button", { name: /^Apply/ }));

    await waitFor(() => expect(state.created.length).toBe(1));
    expect(state.created[0]).toMatchObject({ planId: "plan_61a8", value: "makers" });
  });

  // A role belongs to a project, so a stale selection would offer a pair that
  // does not exist.
  //
  // Asserted by returning to the first project rather than by reading the
  // select after the switch: a `<select>` whose value has no matching option
  // reports "" on its own, so the obvious version of this test passes whether
  // or not the state was ever cleared. Coming back is what tells them apart —
  // an uncleared role reappears.
  it("clears the role when the project changes, and does not restore it", () => {
    compose();
    expect((screen.getByLabelText("Role") as HTMLSelectElement).value).toBe("trained");

    fireEvent.change(screen.getByLabelText("Project"), { target: { value: "pEmpty" } });
    fireEvent.change(screen.getByLabelText("Project"), { target: { value: "pLab" } });

    expect((screen.getByLabelText("Role") as HTMLSelectElement).value).toBe("");
    // And the form knows it is incomplete again.
    expect(screen.getByRole("button", { name: /preview/i }).hasAttribute("disabled")).toBe(true);
  });

  // Not an empty dropdown. A project with no roles cannot confer anything, and
  // saying so is the answer to why the list is empty.
  it("says why a project offers no roles", () => {
    render(<AddMappingDialog target="truenas" onClose={vi.fn()} />);
    fireEvent.change(screen.getByLabelText("Project"), { target: { value: "pEmpty" } });

    expect(screen.getByText(/This project has no roles/)).toBeTruthy();
  });

  it("will not rehearse until it has all four", () => {
    render(<AddMappingDialog target="truenas" onClose={vi.fn()} />);
    expect(screen.getByRole("button", { name: /preview/i }).hasAttribute("disabled")).toBe(true);
  });

  // The same honesty the edit path has: a check that could not run must not
  // read as one that passed.
  it("says when the value could not be checked", async () => {
    state.rehearsed = { ...(state.rehearsed as MappingRehearsal), value_checked: false };
    compose();
    fireEvent.click(screen.getByRole("button", { name: /preview/i }));

    await waitFor(() =>
      expect(document.body.textContent).toMatch(/could not be reached, so/),
    );
  });
});

/**
 * A mapping on a role nobody holds — the ordinary definition-before-assignment
 * case, and the one the dialog could not express.
 *
 * The shared dialog assumed every plan moves at least one person. That is right
 * for a bulk grant, where a plan reaching nobody means nobody was selected, and
 * wrong here: a mapping on an unheld role changes what the role WILL confer,
 * the backend writes it and issues no approval because there is nothing to
 * review, and Apply was then disabled on both counts — no `plan_id`, and a
 * cohort of zero.
 *
 * So an operator could rehearse a definition and never save it.
 */
describe("a mapping on a role nobody holds", () => {
  beforeEach(() => {
    state.rehearsed = {
      op: "create_mapping",
      applied: false,
      // No approval: the backend issues none when there is nothing to review.
      outcomes: [],
      summary: { total: 0, apply: 0, no_change: 0, blocked: 0, failed: 0, succeeded: 0, queued: 0 },
      value_checked: true,
    } as unknown as MappingRehearsal;
  });

  it("can be saved, without an approval it was never given", async () => {
    compose();
    fireEvent.click(screen.getByRole("button", { name: /preview/i }));

    const save = await screen.findByRole("button", { name: "Save mapping" });
    expect(save.hasAttribute("disabled")).toBe(false);

    fireEvent.click(save);
    await waitFor(() => expect(state.created.length).toBe(1));
    // No citation, because none exists — and the backend requires one only
    // once the role has holders.
    expect(state.created[0].planId).toBe("");
  });

  // "Nothing to apply" is what the shared label says at zero, and it is exactly
  // the wrong sentence: there IS something to do, and it is the thing the
  // operator came here for.
  it("does not tell the operator there is nothing to apply", async () => {
    compose();
    fireEvent.click(screen.getByRole("button", { name: /preview/i }));
    await screen.findByRole("button", { name: "Save mapping" });

    expect(screen.queryByRole("button", { name: /Nothing to apply/ })).toBeNull();
  });

  // An absent outcomes array is a payload that arrived short. An empty one is a
  // change that reaches nobody. Telling an operator their good plan is broken
  // is the failure this distinction exists to prevent.
  it("says it reaches nobody, not that its rows failed to arrive", async () => {
    compose();
    fireEvent.click(screen.getByRole("button", { name: /preview/i }));
    await screen.findByRole("button", { name: "Save mapping" });

    expect(document.body.textContent).toMatch(/Nobody holds this role yet/);
    expect(document.body.textContent).not.toMatch(/came back without its rows/);
  });

  // The relaxation is exact: the moment a plan reaches somebody, the approval
  // is required again whatever the surface was told.
  it("still requires the approval once the role has holders", async () => {
    state.rehearsed = {
      ...(state.rehearsed as MappingRehearsal),
      plan_id: undefined,
      outcomes: [{ user_id: "u1", effect: "apply", detail: "x" }],
      summary: { total: 1, apply: 1, no_change: 0, blocked: 0, failed: 0, succeeded: 0, queued: 0 },
    } as unknown as MappingRehearsal;
    compose();
    fireEvent.click(screen.getByRole("button", { name: /preview/i }));

    const apply = await screen.findByRole("button", { name: /^Apply/ });
    expect(apply.hasAttribute("disabled")).toBe(true);
    expect(state.created).toEqual([]);
  });
});
