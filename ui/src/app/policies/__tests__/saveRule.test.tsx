// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import AutomaticRulesPage from "@/app/policies/page";
import type { MappingRuleRow } from "@/lib/queries/useMappingRules";

const state = vi.hoisted(() => ({
  rules: [] as MappingRuleRow[],
  update: vi.fn(),
  targetWaits: false,
  validate: vi.fn(),
}));

vi.mock("@/lib/queries/useMappingRules", () => ({
  useMappingRules: () => ({ data: state.rules, isLoading: false, error: null, refetch: () => {} }),
  useRetiredMappingRules: () => ({ data: [] }),
  useCreateMappingRule: () => ({ mutateAsync: vi.fn(), isPending: false }),
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
  state.rules = [rule()];
  state.update = vi.fn().mockResolvedValue({});
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
    // The second picker is still empty: Save stays blocked until both are chosen.
    expect(screen.getByRole("button", { name: "Create rule" })).toBeDisabled();
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

  it("says things to know in an amber Notice", async () => {
    state.rules = [rule({ source_project: "pLaser", source_role: "trained" })];
    await check({});
    const notice = screen.getByRole("status");
    expect(notice).toHaveTextContent("Checked: 1 thing to know");
    expect(notice).toHaveTextContent("3 people already hold the first role");
    expect(notice).toHaveClass("bg-warn-soft");
  });

  it("says an all-clear as a caption, with no box", async () => {
    await check({});
    expect(screen.queryByRole("alert")).toBeNull();
    expect(screen.queryByRole("status")).toBeNull();
    expect(screen.getByText("Checked.")).toBeInTheDocument();
  });
});
