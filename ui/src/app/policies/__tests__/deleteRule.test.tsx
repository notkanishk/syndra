// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import AutomaticRulesPage from "@/app/policies/page";
import { WouldCancelQueued } from "@/lib/api-client";
import type { QueuedChange } from "@/lib/cancel-confirm";
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

// Retire rule lives on the rule's row (A-22 §3.7); the dialog's own act is named in the dialog.
function openTheRule() {
  render(<AutomaticRulesPage />);
}

beforeEach(() => {
  catalog.waits = false;
  state.rules = [rule()];
  state.remove = vi.fn().mockResolvedValue({ message: "ok", cascade: { enqueued: 12, waiting: 12, wait_reason: "role_waits" } });
});

afterEach(cleanup);

describe("deleting an automatic rule", () => {
  // Two clicks and a stated consequence in between, from the rule's own row.
  it("asks before it deletes, from the row", () => {
    openTheRule();
    fireEvent.click(screen.getByRole("button", { name: "Retire rule" }));

    expect(state.remove).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: "Retire rule and revoke access" })).toBeTruthy();
    // The editor is not involved: Retire rule is not one of its buttons.
    expect(screen.queryByRole("button", { name: "Save rule" })).toBeNull();
    expect(screen.queryByRole("button", { name: "Check rule" })).toBeNull();
    expect(screen.getByRole("button", { name: "Keep it" })).toBeTruthy();
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
    fireEvent.click(screen.getByRole("button", { name: "Retire rule" }));
    expect(document.body.textContent).toMatch(/waits under Pending changes/);
    unmount();

    catalog.waits = false;
    render(<AutomaticRulesPage />);
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
    // The row's button and the dialog's act.
    expect(screen.getAllByRole("button", { name: "Retire rule" })).toHaveLength(2);
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

    expect(document.activeElement).toBe(screen.getByRole("button", { name: "Keep it" }));
    expect(document.activeElement).not.toBe(
      screen.getByRole("button", { name: "Retire rule and revoke access" }),
    );
  });

  // A-22 Q1: retiring takes something away, so the dismiss says what it keeps, never Cancel.
  it("keeps the rule on Keep it, and never offers Cancel", () => {
    openTheRule();
    fireEvent.click(screen.getByRole("button", { name: "Retire rule" }));
    expect(screen.queryByRole("button", { name: "Cancel" })).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "Keep it" }));

    expect(screen.queryByRole("button", { name: "Keep it" })).toBeNull();
    expect(screen.getByRole("button", { name: "Retire rule" })).toBeTruthy();
    expect(state.remove).not.toHaveBeenCalled();
  });

  it("deletes on the confirming click", async () => {
    openTheRule();
    fireEvent.click(screen.getByRole("button", { name: "Retire rule" }));
    fireEvent.click(screen.getByRole("button", { name: "Retire rule and revoke access" }));

    await waitFor(() =>
      expect(state.remove).toHaveBeenCalledWith({ id: "aaaabbbb-1111-2222-3333-444455556666" }),
    );
  });

  // Not on the planner: the first retire names nothing; refused with the
  // removal it would cancel, the step shows it, and going ahead retires again
  // naming exactly that id (`?cancel_ids=`, no header).
  it("retires again naming the waiting change it showed, after the operator goes ahead", async () => {
    const waiting: QueuedChange = {
      id: "00000000-0000-0000-0000-0000000000a1",
      op_type: "add",
      user_id: "u1",
      project_id: "pLaser",
      role_keys: ["trained"],
      source: "rule",
      initiated_by: "op-2",
      initiated_by_name: "Sam Lee",
      queued_at: new Date(Date.now() - 3_600_000).toISOString(),
    };
    state.remove = vi
      .fn()
      .mockRejectedValueOnce(new WouldCancelQueued([waiting], "DELETE", "/rules/mapping/aaaabbbb-1111-2222-3333-444455556666"))
      .mockResolvedValue({ message: "ok", cascade: { enqueued: 0 } });
    openTheRule();
    fireEvent.click(screen.getByRole("button", { name: "Retire rule" }));
    fireEvent.click(screen.getByRole("button", { name: "Retire rule and revoke access" }));
    fireEvent.click(await screen.findByRole("button", { name: /^Cancel giving it and / }));

    await waitFor(() => expect(state.remove).toHaveBeenCalledTimes(2));
    expect(state.remove.mock.calls.map((c) => c[0])).toEqual([
      { id: "aaaabbbb-1111-2222-3333-444455556666", cancelIds: undefined },
      { id: "aaaabbbb-1111-2222-3333-444455556666", cancelIds: [waiting.id] },
    ]);
  });

  // Audit W3, one-path 1.8e: who sends the revocations decides the sentence,
  // in the words a retired bundle uses (lib/retry.ts retireOutcome). A role
  // that waits is the operator's to send; a send that stopped, or that Zitadel
  // didn't take, is retried on its own with its next try; and the
  // confirmation-mode words ("without review") describe nothing now.
  it.each([
    ["role_waits", undefined, /12 changes wait for you to send.*wait under Pending changes until you send them/],
    ["halted:zitadel_offline", undefined, /Not in Zitadel yet\. Zitadel didn.t answer, so Syndra sends them again on its own\..*nothing waits for you/],
    ["halted:retrying", { state: "retrying", try: 2, of: 6, next_try: "2099-11-05T08:35:00Z" }, /Syndra sends them again at .* \(try 2 of 6\)/],
    [undefined, undefined, /12 changes went to Zitadel\..*The access it gave was revoked\./],
  ])("says who sends the revocations, for %s", async (why, retry, said) => {
    state.remove = vi.fn().mockResolvedValue({ message: "ok", cascade: { enqueued: 12, wait_reason: why, drain: { retry } } });
    openTheRule();
    fireEvent.click(screen.getByRole("button", { name: "Retire rule" }));
    fireEvent.click(screen.getByRole("button", { name: "Retire rule and revoke access" }));
    await waitFor(() => expect(document.body.textContent).toMatch(said));
    expect(document.body.textContent).not.toMatch(/without review/);
  });

  // A rule that does not exist yet cannot be retired, and the editor never carries Retire rule.
  it("offers no delete in the editor, new or existing", () => {
    render(<AutomaticRulesPage />);
    fireEvent.click(screen.getByRole("button", { name: "New rule" }));
    const dialog = () => within(screen.getByRole("dialog"));
    expect(dialog().queryByRole("button", { name: "Retire rule" })).toBeNull();
    fireEvent.click(dialog().getByRole("button", { name: "Cancel" }));

    fireEvent.click(screen.getByText("member"));
    expect(dialog().getByRole("button", { name: "Check rule" })).toBeTruthy();
    expect(dialog().queryByRole("button", { name: "Retire rule" })).toBeNull();
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
    expect(screen.queryByRole("button", { name: "Keep it" })).toBeNull();
    expect(screen.getByRole("button", { name: "Close" })).toBeTruthy();
  });

  // A-22 Q3: after the result the sheet is its result; the before-moment unmounts with the body.
  it("unmounts the lede, the What-happens list and the before-line after the result", async () => {
    openTheRule();
    fireEvent.click(screen.getByRole("button", { name: "Retire rule" }));
    expect(document.body.textContent).toMatch(/What happens/);
    expect(document.body.textContent).toMatch(/only because of this rule/);
    state.remove = vi.fn().mockResolvedValue({ cascade: { enqueued: 3 } });
    fireEvent.click(screen.getByRole("button", { name: "Retire rule and revoke access" }));

    await waitFor(() => expect(screen.getByRole("button", { name: "Close" })).toBeTruthy());
    expect(document.body.textContent).not.toMatch(/What happens/);
    expect(document.body.textContent).not.toMatch(/only because of this rule/);
    expect(document.body.textContent).not.toMatch(/edit it instead of retiring it/);
    expect(document.body.textContent).toMatch(/Retired/);
  });

  it("closes the dialog on Close, leaving the list", async () => {
    await deleteTheRule();
    await waitFor(() => expect(screen.getByRole("button", { name: "Close" })).toBeTruthy());

    fireEvent.click(screen.getByRole("button", { name: "Close" }));

    await waitFor(() => expect(screen.queryByRole("button", { name: "Close" })).toBeNull());
    expect(screen.queryByRole("button", { name: "Retire rule and revoke access" })).toBeNull();
  });

  it("stays pressable, as Not retired, when the delete failed", async () => {
    state.remove = vi.fn().mockRejectedValue(new Error("Zitadel is unreachable"));
    openTheRule();
    fireEvent.click(screen.getByRole("button", { name: "Retire rule" }));
    fireEvent.click(screen.getByRole("button", { name: "Retire rule and revoke access" }));

    await waitFor(() => expect(document.body.textContent).toMatch(/unreachable/i));
    expect(screen.getByRole("button", { name: "Retire rule and revoke access" })).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Close" })).toBeNull();
    expect(document.body.textContent).toMatch(/Not retired/);

    // The rule is still there: Keep it closes the dialog and the row remains.
    fireEvent.click(screen.getByRole("button", { name: "Keep it" }));
    await waitFor(() => expect(screen.queryByRole("button", { name: "Keep it" })).toBeNull());
    expect(screen.getByRole("button", { name: "Retire rule" })).toBeTruthy();
  });

  it("holds one result slot: a second failure replaces the first", async () => {
    state.remove = vi.fn().mockRejectedValueOnce(new Error("first trouble")).mockRejectedValueOnce(new Error("second trouble"));
    openTheRule();
    fireEvent.click(screen.getByRole("button", { name: "Retire rule" }));
    fireEvent.click(screen.getByRole("button", { name: "Retire rule and revoke access" }));
    await waitFor(() => expect(document.body.textContent).toMatch(/first trouble/));
    fireEvent.click(screen.getByRole("button", { name: "Retire rule and revoke access" }));
    await waitFor(() => expect(document.body.textContent).toMatch(/second trouble/));
    expect(document.body.textContent).not.toMatch(/first trouble/);
  });
});
