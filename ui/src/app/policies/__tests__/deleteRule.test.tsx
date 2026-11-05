// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import AutomaticRulesPage from "@/app/policies/page";
import type { MappingRuleRow } from "@/lib/queries/useMappingRules";

const state = vi.hoisted(() => ({
  rules: [] as MappingRuleRow[],
  retired: [] as MappingRuleRow[],
  remove: vi.fn(),
}));

vi.mock("@/lib/queries/useMappingRules", () => ({
  useMappingRules: () => ({ data: state.rules, isLoading: false, error: null, refetch: () => {} }),
  useRetiredMappingRules: () => ({ data: state.retired }),
  useCreateMappingRule: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useUpdateMappingRule: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useDeleteMappingRule: () => ({ mutateAsync: state.remove, isPending: false }),
  useValidateMappingRule: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));

vi.mock("@/lib/queries/useProjects", () => ({
  useProjects: () => ({ data: [{ project: { id: "pLaser", name: "Laser Lab" } }] }),
}));

// Whether the role a rule gives waits is the only thing that holds its revokes.
const catalog = vi.hoisted(() => ({ waits: false }));
vi.mock("@/lib/queries/useRoles", () => ({
  useGlobalRoleCatalog: () => ({
    data: [
      {
        project_id: "pLaser",
        project_name: "Laser Lab",
        role_key: "trained",
        display_name: "Trained",
        waits_for_send: catalog.waits,
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

function openTheRule() {
  render(<AutomaticRulesPage />);
  fireEvent.click(screen.getByText("member"));
}

beforeEach(() => {
  catalog.waits = false;
  state.rules = [rule()];
  state.remove = vi.fn().mockResolvedValue({ message: "ok", cascade: { enqueued: 12, waiting: 12, wait_reason: "role_waits" } });
});

afterEach(cleanup);

describe("deleting an automatic rule", () => {
  // Two clicks and a stated consequence in between. The editor is where you already are when
  // you decide to retire a rule, so the confirmation takes over that same dialog.
  it("asks before it deletes, in the dialog already open", () => {
    openTheRule();
    fireEvent.click(screen.getByRole("button", { name: "Retire rule" }));

    expect(state.remove).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: "Retire rule and revoke access" })).toBeTruthy();
    // The form is gone — the question is not asked over the top of the thing it is about.
    expect(screen.queryByRole("button", { name: "Save rule" })).toBeNull();
  });

  // The holder count is the number that decides whether this is a cleanup or an outage.
  it("names how many people the rule acts on", () => {
    openTheRule();
    fireEvent.click(screen.getByRole("button", { name: "Retire rule" }));

    // Recorded, and labelled so: the rule applies to them in Syndra's records.
    expect(document.body.textContent).toMatch(/This rule gives .* to 12 people in Syndra.s records/);
    // And that holding it elsewhere keeps it — the backend works this out per person, so the
    // dialog must not promise a precise loss.
    expect(document.body.textContent).toMatch(/unless a bundle, direct access or another rule also gives it/);
  });

  // Retire keeps its history (owner, 19 Oct 2026): the dialog says so, and the list ends in a
  // fold for the rules already retired.
  it("says the rule can't be used again and stays in the history", () => {
    openTheRule();
    fireEvent.click(screen.getByRole("button", { name: "Retire rule" }));

    expect(document.body.textContent).toMatch(/can.t be used again\. It stays in the history/);
  });

  it("ends the list with a fold of retired rules, closed, once one is retired", () => {
    state.retired = [{ id: "old" } as MappingRuleRow];
    render(<AutomaticRulesPage />);
    expect(screen.getByRole("button", { name: /Retired rules/ })).toHaveAttribute("aria-expanded", "false");
    state.retired = [];
  });

  // A queued rule's revokes queue too. Saying "applied" here would have an operator believe a
  // door was locked while it was still open.
  it("says where the revokes go, per whether the role waits", () => {
    catalog.waits = true;
    const { unmount } = render(<AutomaticRulesPage />);
    fireEvent.click(screen.getByText("member"));
    fireEvent.click(screen.getByRole("button", { name: "Retire rule" }));
    expect(document.body.textContent).toMatch(/waits under Pending changes/);
    unmount();

    catalog.waits = false;
    render(<AutomaticRulesPage />);
    fireEvent.click(screen.getByText("member"));
    fireEvent.click(screen.getByRole("button", { name: "Retire rule" }));
    expect(document.body.textContent).toMatch(/ends as soon as the revocation reaches Zitadel/);
    expect(document.body.textContent).not.toMatch(/no review/);
  });

  it("says nothing is taken back when nobody holds the trigger role", () => {
    state.rules = [rule({ holder_count: 0 })];
    openTheRule();
    fireEvent.click(screen.getByRole("button", { name: "Retire rule" }));

    expect(document.body.textContent).toMatch(/retiring it revokes nothing/i);
    // Nobody to revoke from, so the act does not say it revokes.
    expect(screen.queryByRole("button", { name: "Retire rule and revoke access" })).toBeNull();
    expect(screen.getAllByRole("button", { name: "Retire rule" })).toHaveLength(1);
  });

  // The title never carries an id ("Retire this rule?"), so the chip above it is what
  // names which rule is about to be retired.
  it("names the rule by its project/role pair", () => {
    openTheRule();
    fireEvent.click(screen.getByRole("button", { name: "Retire rule" }));

    expect(document.body.textContent).toMatch(/pDoor\/member/);
    expect(document.body.textContent).toMatch(/pLaser\/trained/);
  });

  // Modal focuses the panel's first focusable element in DOM order on open.
  // The destructive button used to be that element — every rule-delete
  // dialog opened with the cursor already on "revoke this access".
  it("does not autofocus the destructive button", () => {
    openTheRule();
    fireEvent.click(screen.getByRole("button", { name: "Retire rule" }));

    expect(document.activeElement).toBe(screen.getByRole("button", { name: "Cancel" }));
    expect(document.activeElement).not.toBe(
      screen.getByRole("button", { name: "Retire rule and revoke access" }),
    );
  });

  it("backs out to the editor rather than closing everything", () => {
    openTheRule();
    fireEvent.click(screen.getByRole("button", { name: "Retire rule" }));
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));

    expect(screen.getByRole("button", { name: "Save rule" })).toBeTruthy();
    expect(state.remove).not.toHaveBeenCalled();
  });

  it("deletes on the confirming click", async () => {
    openTheRule();
    fireEvent.click(screen.getByRole("button", { name: "Retire rule" }));
    fireEvent.click(screen.getByRole("button", { name: "Retire rule and revoke access" }));

    await waitFor(() =>
      expect(state.remove).toHaveBeenCalledWith("aaaabbbb-1111-2222-3333-444455556666"),
    );
  });

  // Audit W3: who sends the revocations decides the sentence. A role that waits
  // is the operator's to send; a send that stopped is retried on its own; and
  // the confirmation-mode words ("without review") describe nothing now.
  it.each([
    ["role_waits", /12 revocations waiting for you to send.*wait under Pending changes until you send them/],
    ["halted:zitadel_offline", /12 revocations on their way.*Revocations send on their own\. Zitadel didn.t answer at .*, so they go when it does\./],
    [undefined, /12 revocations sent.*The access it gave was revoked\./],
  ])("says who sends the revocations, for %s", async (why, said) => {
    state.remove = vi.fn().mockResolvedValue({ message: "ok", cascade: { enqueued: 12, wait_reason: why } });
    openTheRule();
    fireEvent.click(screen.getByRole("button", { name: "Retire rule" }));
    fireEvent.click(screen.getByRole("button", { name: "Retire rule and revoke access" }));
    await waitFor(() => expect(document.body.textContent).toMatch(said));
    expect(document.body.textContent).not.toMatch(/without review/);
  });

  // A rule that does not exist yet cannot be retired.
  it("offers no delete on the create form", () => {
    render(<AutomaticRulesPage />);
    fireEvent.click(screen.getByRole("button", { name: "New rule" }));

    expect(screen.queryByRole("button", { name: "Retire rule" })).toBeNull();
  });
});

/**
 * What happens AFTER the delete lands.
 *
 * The rule-delete dialog took an `onDeleted` callback and never called it, so
 * deleting left the operator looking at an editor for a rule that no longer
 * exists. It cannot be called from the mutation either: `onDeleted` closes that
 * editor, and this dialog goes with it — taking the report nobody has read yet.
 * The sequence is the contract.
 */
describe("after a rule is deleted", () => {
  async function deleteTheRule() {
    state.remove = vi
      .fn()
      .mockResolvedValue({ cascade: { enqueued: 3 } });
    openTheRule();
    fireEvent.click(screen.getByRole("button", { name: "Retire rule" }));
    fireEvent.click(screen.getByRole("button", { name: "Retire rule and revoke access" }));
    await waitFor(() => expect(state.remove).toHaveBeenCalled());
  }

  it("keeps the report on screen and retires the destructive action", async () => {
    await deleteTheRule();

    await waitFor(() => expect(document.body.textContent).toMatch(/retired/i));
    expect(screen.queryByRole("button", { name: "Retire rule and revoke access" })).toBeNull();
    expect(screen.queryByRole("button", { name: "Cancel" })).toBeNull();
    expect(screen.getByRole("button", { name: "Close" })).toBeTruthy();
  });

  it("closes the editor on dismiss, rather than returning to it", async () => {
    await deleteTheRule();
    await waitFor(() => expect(screen.getByRole("button", { name: "Close" })).toBeTruthy());

    fireEvent.click(screen.getByRole("button", { name: "Close" }));

    // Not back to the form: there is no rule behind it any more. `Keep the
    // rule` would have landed here instead, which is the defect.
    await waitFor(() =>
      expect(screen.queryByRole("button", { name: "Save rule" })).toBeNull(),
    );
    expect(screen.queryByRole("button", { name: "Retire rule" })).toBeNull();
  });

  it("returns to the editor when the delete failed", async () => {
    state.remove = vi.fn().mockRejectedValue(new Error("Zitadel is unreachable"));
    openTheRule();
    fireEvent.click(screen.getByRole("button", { name: "Retire rule" }));
    fireEvent.click(screen.getByRole("button", { name: "Retire rule and revoke access" }));

    await waitFor(() => expect(document.body.textContent).toMatch(/unreachable/i));
    expect(screen.getByRole("button", { name: "Retire rule and revoke access" })).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Close" })).toBeNull();

    // The rule is still there, so backing out puts the operator back on it.
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    await waitFor(() => expect(screen.getByRole("button", { name: "Save rule" })).toBeTruthy());
  });
});
