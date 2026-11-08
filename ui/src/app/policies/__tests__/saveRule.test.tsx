// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import AutomaticRulesPage from "@/app/policies/page";
import type { MappingRuleRow } from "@/lib/queries/useMappingRules";

const state = vi.hoisted(() => ({
  rules: [] as MappingRuleRow[],
  update: vi.fn(),
  create: vi.fn(),
  targetWaits: false,
  validate: vi.fn(),
  whatIf: vi.fn(),
}));

// Who a rule reaches is the backend's answer (the what-if), down the chain.
vi.mock("@/lib/queries/useInsights", () => ({
  fetchWhatIf: (...args: unknown[]) => state.whatIf(...args),
}));

vi.mock("@/lib/queries/useMappingRules", () => ({
  useMappingRules: () => ({ data: state.rules, isLoading: false, error: null, refetch: () => {} }),
  useRetiredMappingRules: () => ({ data: [] }),
  useCreateMappingRule: () => ({ mutateAsync: (...a: unknown[]) => state.create(...a), isPending: false }),
  useUpdateMappingRule: () => ({ mutateAsync: state.update, isPending: false }),
  useDeleteMappingRule: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useValidateMappingRule: () => ({ mutateAsync: state.validate, isPending: false }),
}));

vi.mock("@/lib/queries/useProjects", () => ({
  useProjects: () => ({ data: [{ project: { id: "pLaser", name: "Laser Lab" } }] }),
}));

// Before the switch over: a rule's own mode still holds its changes.
vi.mock("@/lib/queries/useRolesThatWait", () => ({
  useRolesThatWait: () => ({ data: { switched_over: true, roles: [] } }),
}));
vi.mock("@/lib/queries/useRoles", () => ({
  useGlobalRoleCatalog: () => ({
    data: [
      {
        project_id: "pLaser",
        project_name: "Laser Lab",
        role_key: "trained",
        display_name: "Trained",
        waits_for_send: state.targetWaits,
        assigned_user_count: 3,
      },
    ],
  }),
}));

vi.mock("@/components/names", () => ({
  ProjectName: () => null,
  RoleRef: ({ projectId, roleKey }: { projectId: string; roleKey: string }) => (
    <>{`${projectId}/${roleKey}`}</>
  ),
}));

function rule(overrides: Partial<MappingRuleRow> = {}): MappingRuleRow {
  return {
    id: "aaaabbbb-1111-2222-3333-444455556666",
    source_project: "pDoor",
    source_role: "member",
    target_project: "pLaser",
    target_role: "trained",
    holder_count: 12,
    created_at: "2026-01-01T00:00:00Z",
    ...overrides,
  };
}

async function saveTheOpenRule() {
  render(<AutomaticRulesPage />);
  fireEvent.click(screen.getByText("member"));
  fireEvent.click(screen.getByRole("button", { name: "Check rule" }));
  await waitFor(() => expect(screen.getByRole("button", { name: "Check again" })).toBeTruthy());
  fireEvent.click(screen.getByRole("button", { name: "Save rule" }));
}

beforeEach(() => {
  state.whatIf = vi.fn().mockResolvedValue({ gains: [], losses: [], unchanged: 0 });
  state.rules = [rule()];
  state.update = vi.fn().mockResolvedValue({});
  state.create = vi.fn().mockResolvedValue({});
  state.targetWaits = false;
  state.validate = vi.fn().mockResolvedValue({});
});

afterEach(cleanup);

/**
 * Save used to call onClose() in the same tick as setOutcome(), which threw
 * away the outcome — including whether the rule's access waits under Pending
 * changes or applied at once — before the operator ever saw it, and the pill
 * said "Applied" regardless of mode even when it had painted.
 */
/** The editor is "Edit this rule", never an id; the chip above it names the project/role pair. */
describe("opening a rule to edit", () => {
  it("names the rule by its project/role pair", () => {
    render(<AutomaticRulesPage />);
    fireEvent.click(screen.getByText("member"));

    expect(document.body.textContent).toMatch(/pDoor\/member/);
    expect(document.body.textContent).toMatch(/pLaser\/trained/);
  });
});

describe("after saving a rule", () => {
  it("keeps the dialog open and reports where the access goes, when the role waits", async () => {
    state.targetWaits = true;
    await saveTheOpenRule();

    await waitFor(() => expect(document.body.textContent).toMatch(/Rule updated/));
    // Not "Applied" — a manual rule's access still waits under Pending changes,
    // so the link to send it and the detail sentence must agree.
    expect(screen.getByRole("link", { name: /Waiting for you to send/ }).getAttribute("href")).toBe(
      "/governance/pending",
    );
    expect(document.body.textContent).toMatch(/waits under Pending changes/);
    expect(screen.queryByRole("button", { name: "Save rule" })).toBeNull();
    expect(screen.getByRole("button", { name: "Close" })).toBeTruthy();
  });

  it("reports Saved when the role does not wait, and closes on Close", async () => {
    await saveTheOpenRule();

    await waitFor(() => expect(document.body.textContent).toMatch(/Saved/));
    expect(document.body.textContent).toMatch(/get the second at once/);
    // Audit W6: a rule follows roles given through Syndra; one added by hand in
    // Zitadel is drift, and the rule follows once it is adopted.
    expect(document.body.textContent).toMatch(/anyone given the first role through Syndra/);
    expect(document.body.textContent).toMatch(/added directly in Zitadel shows in Drift, and the rule follows once you adopt it/);
    expect(document.body.textContent).not.toMatch(/given the first role later/);

    fireEvent.click(screen.getByRole("button", { name: "Close" }));
    await waitFor(() => expect(screen.queryByRole("button", { name: "Close" })).toBeNull());
  });
});

/** Roles are chosen in the picker, never a native select; the choice shows as a chip. */
describe("choosing the two roles", () => {
  it("offers no native select, and keeps the two pickers' choices apart", () => {
    render(<AutomaticRulesPage />);
    fireEvent.click(screen.getByRole("button", { name: "New rule" }));
    expect(document.querySelector("select")).toBeNull();

    const source = within(screen.getByRole("radiogroup", { name: "If someone holds" }));
    fireEvent.click(source.getByRole("button", { name: /Laser Lab/ }));
    fireEvent.click(source.getByRole("radio", { name: /Trained/ }));

    expect(screen.getAllByRole("button", { name: "Remove Trained" })).toHaveLength(1);
    // The second picker is still empty: the check stays blocked until both are chosen.
    expect(screen.getByRole("button", { name: "Check rule" })).toBeDisabled();
    // The chosen list folded to its chip, so the second list starts in view (A-22 G30).
    expect(screen.queryByRole("radiogroup", { name: "If someone holds" })).toBeNull();
    expect(screen.getByRole("radiogroup", { name: "then also give them" })).toBeTruthy();
  });

  // Browser pass, 8 Nov: the second list starts below the fold, and its matches
  // were out of view until the dialog was scrolled.
  it("brings the second list into view when its search changes, and not the first", () => {
    const scroll = vi.fn();
    const was = Element.prototype.scrollIntoView;
    Element.prototype.scrollIntoView = scroll;
    try {
      render(<AutomaticRulesPage />);
      fireEvent.click(screen.getByRole("button", { name: "New rule" }));
      fireEvent.input(screen.getByRole("searchbox", { name: "Search the role they hold" }), { target: { value: "tra" } });
      expect(scroll).not.toHaveBeenCalled();
      fireEvent.input(screen.getByRole("searchbox", { name: "Search the role to give" }), { target: { value: "tra" } });
      expect(scroll).toHaveBeenCalledWith({ block: "start" });
      expect(scroll.mock.contexts[0]).toContainElement(screen.getByText("then also give them"));
    } finally {
      Element.prototype.scrollIntoView = was;
    }
  });

  it("names no rule by an id anywhere in the list", () => {
    render(<AutomaticRulesPage />);
    expect(document.body.textContent).not.toMatch(/R-[0-9a-f]{4}/);
  });
});

describe("the before-line", () => {
  function pickBothRoles() {
    render(<AutomaticRulesPage />);
    fireEvent.click(screen.getByRole("button", { name: "New rule" }));
    for (const picker of ["If someone holds", "then also give them"]) {
      const group = within(screen.getByRole("radiogroup", { name: picker }));
      fireEvent.click(group.getByRole("button", { name: /Laser Lab/ }));
      fireEvent.click(group.getByRole("radio", { name: /Trained/ }));
    }
  }

  it("is neutral when the role it gives does not wait", () => {
    pickBothRoles();
    expect(document.body.textContent).toContain("What this rule gives goes straight to Zitadel.");
    expect(screen.queryByRole("img", { name: "Waits for you" })).toBeNull();
  });

  it("warns, and marks the role, when the role it gives waits", () => {
    state.targetWaits = true;
    pickBothRoles();
    expect(document.body.textContent).toContain(
      "Trained always waits for you, so what this rule gives will wait for you to send.",
    );
    expect(screen.getAllByRole("img", { name: "Waits for you" }).length).toBeGreaterThan(0);
  });
});

/** A box means something is wrong (A-04 §7.1a): the check's findings are a Notice, its all-clear is not. */
describe("the rule check's result", () => {
  async function check(result: unknown) {
    state.validate = vi.fn().mockResolvedValue(result);
    render(<AutomaticRulesPage />);
    fireEvent.click(screen.getAllByText(/^(member|trained)$/)[0]);
    fireEvent.click(screen.getByRole("button", { name: "Check rule" }));
    await waitFor(() => expect(screen.getByRole("button", { name: "Check again" })).toBeTruthy());
  }

  it("says a rule that can't be saved in a red Notice that blocks", async () => {
    await check({ would_cycle: true, reason: "It would give a role to itself." });
    const notice = screen.getByRole("alert");
    expect(notice).toHaveTextContent("This rule can't be saved");
    expect(notice).toHaveClass("bg-danger-soft");
    expect(notice).toHaveTextContent("It would give a role to itself.");
  });

  it("says things to know as the What-happens list, never a Notice", async () => {
    state.rules = [rule({ source_project: "pLaser", source_role: "trained" })];
    state.whatIf = vi.fn().mockResolvedValue({
      gains: ["u1", "u2", "u3"].map((u) => ({ user_id: u, name: u, roles: [{ project_id: "pLaser", role_key: "trained" }] })),
      losses: [],
      unchanged: 0,
    });
    await check({});
    const list = screen.getByRole("group", { name: "What happens" });
    expect(list).toHaveTextContent("Saving would give 3 people Laser Lab / trained.");
    expect(list).toHaveClass("bg-warn-soft");
    expect(screen.queryByRole("status")).toBeNull();
    expect(document.body.textContent).not.toMatch(/Checked: 1 thing/);
    // An edit asks about replacing this rule, not adding a second one.
    expect(state.whatIf).toHaveBeenCalledWith(expect.objectContaining({ kind: "rule_added", rule_id: "aaaabbbb-1111-2222-3333-444455556666" }));
  });

  it("names every role the chain gives, from the backend", async () => {
    state.rules = [rule()];
    state.whatIf = vi.fn().mockResolvedValue({
      gains: [
        {
          user_id: "u1",
          name: "u1",
          roles: [
            { project_id: "pLaser", role_key: "trained" },
            { project_id: "pLaser", role_key: "senior" },
            { project_id: "pLaser", role_key: "keys" },
          ],
        },
      ],
      losses: [],
      unchanged: 0,
    });
    await check({});
    const notice = screen.getByRole("group", { name: "What happens" });
    expect(notice).toHaveTextContent("Saving would give 1 person Laser Lab / trained.");
    expect(notice).toHaveTextContent("Also sets off other rules, which would then give them Laser Lab / keys and Laser Lab / senior.");
  });

  // Audit W12, rules-on-the-pipeline 6.5d: the chain is the backend's closure
  // over the unsaved rule, never walked over the browser's rule list. A chain
  // in the list that the what-if does not report is not named.
  it("never walks the rule list in the browser", async () => {
    state.rules = [
      rule(),
      rule({ id: "r2", source_project: "pLaser", source_role: "trained", target_project: "pShop", target_role: "bench" }),
      rule({ id: "r3", source_project: "pShop", source_role: "bench", target_project: "pShop", target_role: "key" }),
    ];
    await check({});
    expect(document.body.textContent).not.toMatch(/Also sets off/);
    expect(state.whatIf).toHaveBeenCalledWith(
      expect.objectContaining({ kind: "rule_added", target_project_id: "pLaser", target_role_key: "trained" }),
    );
  });

  it("says 'another rule' for one chained role", async () => {
    state.whatIf = vi.fn().mockResolvedValue({
      gains: [{ user_id: "u1", name: "u1", roles: [{ project_id: "pLaser", role_key: "trained" }, { project_id: "pLaser", role_key: "keys" }] }],
      losses: [],
      unchanged: 0,
    });
    await check({});
    expect(screen.getByRole("group", { name: "What happens" })).toHaveTextContent("Also sets off another rule, which would then give them Laser Lab / keys.");
  });

  it("says an all-clear as a caption, with no box", async () => {
    state.rules = [rule()];
    await check({});
    expect(screen.queryByRole("alert")).toBeNull();
    expect(screen.queryByRole("status")).toBeNull();
    expect(screen.getByText("Checked.")).toBeInTheDocument();
  });
});

/** A-22 §3.7: two steps in one sheet, and the sheet becomes its result. */
describe("the rule editor's two steps", () => {
  function chooseBoth() {
    for (const picker of ["If someone holds", "then also give them"]) {
      const group = within(screen.getByRole("radiogroup", { name: picker }));
      fireEvent.click(group.getByRole("button", { name: /Laser Lab/ }));
      fireEvent.click(group.getByRole("radio", { name: /Trained/ }));
    }
  }
  const gains = {
    gains: ["u1", "u2"].map((u) => ({ user_id: u, name: u, roles: [{ project_id: "pLaser", role_key: "trained" }] })),
    losses: [],
    unchanged: 0,
  };

  it("chooses, checks, reads the findings, saves, and ends on the result", async () => {
    const create = vi.fn().mockResolvedValue({});
    state.create = create;
    state.whatIf = vi.fn().mockResolvedValue(gains);
    render(<AutomaticRulesPage />);
    fireEvent.click(screen.getByRole("button", { name: "New rule" }));
    const dialog = within(screen.getByRole("dialog"));

    // Before the check: Cancel and Check rule, no Save.
    expect(dialog.getByRole("button", { name: "Cancel" })).toBeTruthy();
    expect(dialog.queryByRole("button", { name: "Create rule" })).toBeNull();
    chooseBoth();
    // Both lists are folded to chips.
    expect(screen.queryByRole("radiogroup")).toBeNull();
    fireEvent.click(dialog.getByRole("button", { name: "Check rule" }));

    // After the check: findings, Check again beside them, Cancel and Create rule in the pair.
    const list = await dialog.findByRole("group", { name: "What happens" });
    expect(list).toHaveTextContent("Saving would give 2 people Laser Lab / trained.");
    expect(dialog.getByRole("button", { name: "Check again" })).toBeTruthy();
    expect(dialog.queryByRole("button", { name: "Check rule" })).toBeNull();

    fireEvent.click(dialog.getByRole("button", { name: "Create rule" }));
    await waitFor(() => expect(create).toHaveBeenCalled());
    // The sheet is its result: Created, the form, findings and lede are gone, one Close.
    await dialog.findByText("Created");
    expect(screen.queryByRole("group", { name: "What happens" })).toBeNull();
    expect(screen.queryByText("then also give them")).toBeNull();
    expect(document.body.textContent).not.toMatch(/Anybody who holds the first role/);
    expect(dialog.queryByRole("button", { name: "Cancel" })).toBeNull();
    expect(dialog.getByRole("button", { name: "Close" })).toBeTruthy();
  });

  it("says Not created when the write fails, and the form stays to press again", async () => {
    state.create = vi.fn().mockRejectedValue(new Error("Zitadel is unreachable"));
    render(<AutomaticRulesPage />);
    fireEvent.click(screen.getByRole("button", { name: "New rule" }));
    chooseBoth();
    fireEvent.click(screen.getByRole("button", { name: "Check rule" }));
    fireEvent.click(await screen.findByRole("button", { name: "Create rule" }));

    await waitFor(() => expect(document.body.textContent).toMatch(/Not created/));
    expect(document.body.textContent).not.toMatch(/Didn.t go through|Couldn.t/);
    expect(screen.getByRole("button", { name: "Create rule" })).toBeTruthy();
  });

  it("clears the old result before a press, so one slot holds one result", async () => {
    state.update = vi.fn().mockRejectedValueOnce(new Error("first trouble")).mockRejectedValueOnce(new Error("second trouble"));
    render(<AutomaticRulesPage />);
    fireEvent.click(screen.getByText("member"));
    fireEvent.click(screen.getByRole("button", { name: "Check rule" }));
    fireEvent.click(await screen.findByRole("button", { name: "Save rule" }));
    await waitFor(() => expect(document.body.textContent).toMatch(/first trouble/));
    fireEvent.click(screen.getByRole("button", { name: "Save rule" }));
    await waitFor(() => expect(document.body.textContent).toMatch(/second trouble/));
    expect(document.body.textContent).not.toMatch(/first trouble/);
  });

  it("removing a chip reopens that list", () => {
    render(<AutomaticRulesPage />);
    fireEvent.click(screen.getByRole("button", { name: "New rule" }));
    const group = within(screen.getByRole("radiogroup", { name: "If someone holds" }));
    fireEvent.click(group.getByRole("button", { name: /Laser Lab/ }));
    fireEvent.click(group.getByRole("radio", { name: /Trained/ }));
    expect(screen.queryByRole("radiogroup", { name: "If someone holds" })).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "Remove Trained" }));
    expect(screen.getByRole("radiogroup", { name: "If someone holds" })).toBeTruthy();
  });
});
