// @vitest-environment jsdom
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { AddMappingDialog } from "@/components/targets/AddMappingDialog";
import { ApiError } from "@/lib/api-client";
import type { MappingRehearsal } from "@/lib/queries/useMappings";
import type { MappableValues, TargetReach } from "@/lib/queries/useTargets";

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
  rehearseError: null as unknown,
  values: null as MappableValues | null,
  valuesError: null as unknown,
  refetched: 0,
  reach: { read: true, current: true, read_at: "2026-10-14T09:00:00Z", groups: {} } as TargetReach,
};

vi.mock("@/lib/queries/useMappings", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/queries/useMappings")>()),
  rehearseMappingCreate: vi.fn(async (input: Record<string, unknown>, acknowledgeScope: boolean) => {
    state.rehearsals.push({ input, acknowledgeScope });
    if (state.rehearseError) throw state.rehearseError;
    return state.rehearsed;
  }),
  useCreateMapping: () => ({
    mutateAsync: vi.fn(async (input: Record<string, unknown>) => {
      state.created.push(input);
      return { mapping: { id: "m1" }, queued_convergences: 2 };
    }),
    isPending: false,
  }),
  useMappings: () => ({
    data: [{ id: "m0", target: "truenas", project_id: "pLab", role_key: "lead", field: "group", value: "leads" }],
    isLoading: false,
    error: null,
  }),
}));

const homeSettings = {
  data: { read: true, settings: [{ name: "home_root", label: "Where home folders go", value: "", values: [], values_read: true }] } as {
    read: boolean;
    settings: Array<{ name: string; label: string; value: string; values: string[]; labels?: Record<string, string>; values_read: boolean }>;
  },
};

vi.mock("@/lib/queries/useTargets", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/queries/useTargets")>()),
  useTargets: () => ({
    data: [
      {
        target: "truenas",
        project: { id: "pLab", name: "Laser Lab" },
        health: { status: "online", recent: [], started_at: "2026-10-14T09:38:00Z" },
      },
    ],
    isLoading: false,
    error: null,
  }),
  useMappableValues: () => ({
    data: state.values ?? undefined,
    error: state.valuesError,
    isLoading: false,
    refetch: () => {
      state.refetched++;
    },
  }),
  useTargetSettings: () => ({ data: homeSettings.data, isLoading: false, error: null, refetch: () => {} }),
  useTargetReach: () => ({ data: state.reach, isLoading: false, error: null }),
}));

vi.mock("@/lib/queries/useProjects", () => ({
  useProjects: () => ({
    data: [
      {
        project: {
          id: "pLab",
          name: "Laser Lab",
          roles: [
            { key: "trained", label: "Trained operator" },
            { key: "lead", label: "Lab lead" },
          ],
        },
      },
      { project: { id: "pOther", name: "Studio", roles: [{ key: "studio", label: "Studio user" }] } },
    ],
    isLoading: false,
    error: null,
  }),
}));

vi.mock("@/lib/queries/useNameResolver", () => ({
  useNameResolver: () => ({
    resolveRole: (_p: string, key: string) => ({ value: { display_name: key === "lead" ? "Lab lead" : key }, resolved: true }),
    resolveProject: () => ({ value: { name: "Laser Lab" }, resolved: true }),
    resolveUser: (id: string) => ({ value: { id, name: id, email: "" }, resolved: true }),
    resolveBundle: () => ({ value: undefined, resolved: true }),
  }),
}));

vi.mock("@/components/names", () => ({
  UserName: ({ id }: { id: string }) => <span>{id}</span>,
}));

function values(over: Partial<MappableValues> = {}): MappableValues {
  return {
    field: "group",
    label: "Group",
    single: false,
    values: ["makers", "leads"],
    enumerable: true,
    read_at: new Date().toISOString(),
    current: true,
    ...over,
  };
}

beforeEach(() => {
  state.rehearsals = [];
  state.created = [];
  state.rehearseError = null;
  state.values = values();
  state.valuesError = null;
  state.refetched = 0;
  state.reach = { read: true, current: true, read_at: "2026-10-14T09:00:00Z", groups: {} };
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
  fireEvent.change(screen.getByLabelText("Role in Laser Lab"), { target: { value: "trained" } });
  fireEvent.change(screen.getByLabelText("Group on TrueNAS"), { target: { value: "makers" } });
}

const preview = () => screen.getByRole("button", { name: /preview/i });

describe("adding a mapping", () => {
  it("rehearses before it writes anything", async () => {
    compose();
    fireEvent.click(preview());

    await waitFor(() => expect(state.rehearsals.length).toBe(1));
    expect(state.rehearsals[0].input).toMatchObject({
      target: "truenas",
      projectId: "pLab",
      roleKey: "trained",
      field: "group",
      value: "makers",
    });
    expect(state.rehearsals[0].acknowledgeScope).toBe(false);
    expect(state.created).toEqual([]);
  });

  it("applies the plan it was shown, citing it", async () => {
    compose();
    fireEvent.click(preview());
    fireEvent.click(await screen.findByRole("button", { name: /^Apply/ }));

    await waitFor(() => expect(state.created.length).toBe(1));
    expect(state.created[0]).toMatchObject({ planId: "plan_61a8", value: "makers", projectId: "pLab" });
  });

  // The system has one project; its roles are the only ones offered, less
  // those already mapped.
  it("offers only the system's project's roles that aren't mapped yet", () => {
    render(<AddMappingDialog target="truenas" onClose={vi.fn()} />);
    const roles = Array.from((screen.getByLabelText("Role in Laser Lab") as HTMLSelectElement).options).map((o) => o.textContent);
    expect(roles).toEqual(["Choose a role", "Trained operator"]);
    expect(screen.getByText("Roles already mapped are not offered.")).toBeTruthy();
  });

  it("will not rehearse until it has a role and a group", () => {
    render(<AddMappingDialog target="truenas" onClose={vi.fn()} />);
    expect(preview().hasAttribute("disabled")).toBe(true);
    fireEvent.change(screen.getByLabelText("Role in Laser Lab"), { target: { value: "trained" } });
    expect(preview().hasAttribute("disabled")).toBe(true);
  });

  it("says what the picked group opens before the preview", () => {
    state.reach = {
      read: true,
      current: true,
      groups: {
        makers: [
          { share: "main", folder: "waddle_share", path: "/mnt/p/main/waddle_share", access: "change" },
          { share: "main", folder: "quack_share", path: "/mnt/p/main/quack_share", access: "read" },
        ],
        leads: [],
      },
    };
    compose();
    expect(document.body.textContent).toMatch(/Opens main › waddle_share, can change files, and main › quack_share, read only\./);
    fireEvent.change(screen.getByLabelText("Group on TrueNAS"), { target: { value: "leads" } });
    expect(screen.getByText("Opens nothing on TrueNAS yet.")).toBeTruthy();
  });

  it("says when the value could not be checked", async () => {
    state.rehearsed = { ...(state.rehearsed as MappingRehearsal), value_checked: false };
    compose();
    fireEvent.click(preview());
    await waitFor(() => expect(document.body.textContent).toMatch(/could not be reached, so/));
  });
});

describe("the group list", () => {
  // A-01 §3: a NAS with only built-in groups is a fresh NAS, not a fault.
  it("explains a NAS with no groups of its own, and reads again", () => {
    state.values = values({ values: [], builtin_count: 34, builtin_examples: ["builtin_users", "builtin_administrators"] });
    render(<AddMappingDialog target="truenas" onClose={vi.fn()} />);
    expect((screen.getByLabelText("Group on TrueNAS") as HTMLSelectElement).disabled).toBe(true);
    expect(screen.getByText("TrueNAS has no groups of its own yet.")).toBeTruthy();
    expect(screen.getByText(/It lists 34 groups, all of them built in, like builtin_users and builtin_administrators/)).toBeTruthy();
    expect(screen.getByText("Nothing to preview until TrueNAS has a group of its own.")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Read again" }));
    expect(state.refetched).toBe(1);
  });

  it("says when the list was never read, with no count", () => {
    state.values = null;
    state.valuesError = new ApiError(409, { error: "VALUES_NEVER_READ", message: "never" });
    render(<AddMappingDialog target="truenas" onClose={vi.fn()} />);
    expect(screen.getByText("TrueNAS hasn't listed its groups yet.")).toBeTruthy();
    expect(screen.getByText(/hasn't answered since Syndra restarted at \d\d:\d\d/)).toBeTruthy();
    expect(screen.queryByText(/It lists/)).toBeNull();
    expect(preview().hasAttribute("disabled")).toBe(true);
  });

  it("labels a last-known list with its time, and lets it through", () => {
    state.values = values({ current: false, read_at: "2026-10-14T09:02:00Z" });
    compose();
    expect(screen.getByText(/^The list TrueNAS gave at \d\d:\d\d\.$/)).toBeTruthy();
    expect(screen.getByText(/^This list is from/)).toBeTruthy();
    expect(preview().hasAttribute("disabled")).toBe(false);
  });

  // The one refusal a picker leaves: the group went away after the list was
  // read. The list is read again and the choice cleared.
  it("reads the list again when the group went away", async () => {
    state.rehearseError = new ApiError(400, { error: "VALIDATION_FAILED", message: "gone", details: { value: "unresolvable" } });
    compose();
    fireEvent.click(preview());
    await waitFor(() => expect(document.body.textContent).toMatch(/makers is no longer on TrueNAS/));
    expect(state.refetched).toBe(1);
    expect((screen.getByLabelText("Group on TrueNAS") as HTMLSelectElement).value).toBe("");
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

// A-03 §2: the dialog is the one write place for what a role gives, its home
// folder and limit included.
describe("a role's home folder", () => {
  const withRoot = (value: string) => {
    homeSettings.data = {
      read: true,
      settings: [
        {
          name: "home_root",
          label: "Where home folders go",
          value,
          values: ["/mnt/p/main/home_dir"],
          labels: { "/mnt/p/main/home_dir": "main › home_dir" },
          values_read: true,
        },
      ],
    };
  };

  it("says where to choose the location first, and works without it", () => {
    withRoot("");
    compose();
    expect(screen.queryByRole("radio", { name: "Gives a home folder" })).toBeNull();
    expect(screen.getByRole("link", { name: "Choose where home folders go" }).getAttribute("href")).toBe("/system/targets/truenas");
    expect(preview().hasAttribute("disabled")).toBe(false);
  });

  it("rehearses the group, the home and its limit as one plan, and counts the homes it makes", async () => {
    withRoot("/mnt/p/main/home_dir");
    compose();
    expect(screen.getByText(/Home folders go in main › home_dir, set on TrueNAS's page for every role/)).toBeTruthy();
    fireEvent.click(screen.getByRole("radio", { name: "Gives a home folder" }));
    // A size is the first choice, and nothing previews until it has one.
    expect(preview().hasAttribute("disabled")).toBe(true);
    fireEvent.change(screen.getByLabelText("Limit in GB"), { target: { value: "20" } });
    fireEvent.click(preview());

    await waitFor(() => expect(state.rehearsals.length).toBe(3));
    expect(state.rehearsals.map((r) => [r.input.field, r.input.value])).toEqual([
      ["group", "makers"],
      ["home", "true"],
      ["home_quota", "21474836480"],
    ]);
    // Two people in the plan, both made a home: the home row's own rehearsal.
    await waitFor(() => expect(document.body.textContent).toMatch(/Makes 2 home folders in main › home_dir\./));
    fireEvent.click(await screen.findByRole("button", { name: /^Apply/ }));
    await waitFor(() => expect(state.created.length).toBe(3));
  });

  it("gives no limit as an explicit choice", async () => {
    withRoot("/mnt/p/main/home_dir");
    compose();
    fireEvent.click(screen.getByRole("radio", { name: "Gives a home folder" }));
    fireEvent.click(screen.getByRole("radio", { name: "No limit" }));
    fireEvent.click(preview());
    await waitFor(() => expect(state.rehearsals.length).toBe(3));
    expect(state.rehearsals[2].input).toMatchObject({ field: "home_quota", value: "0" });
    withRoot("");
  });
});
