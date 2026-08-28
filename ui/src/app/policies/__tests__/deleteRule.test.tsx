// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import AutomaticRulesPage from "@/app/policies/page";
import type { MappingRuleRow } from "@/lib/queries/useMappingRules";

const state = vi.hoisted(() => ({
  rules: [] as MappingRuleRow[],
  remove: vi.fn(),
}));

vi.mock("@/lib/queries/useMappingRules", () => ({
  useMappingRules: () => ({ data: state.rules, isLoading: false, error: null, refetch: () => {} }),
  useCreateMappingRule: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useUpdateMappingRule: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useDeleteMappingRule: () => ({ mutateAsync: state.remove, isPending: false }),
  useSetRuleConfirmationMode: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useValidateMappingRule: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));

vi.mock("@/lib/queries/useConfirmationMode", () => ({
  useBulkSetConfirmationMode: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));

vi.mock("@/lib/queries/useProjects", () => ({
  useProjects: () => ({ data: [{ project: { id: "pLaser", name: "Laser Lab" } }] }),
}));

vi.mock("@/lib/queries/useRoles", () => ({
  useGlobalRoleCatalog: () => ({
    data: [
      {
        project_id: "pLaser",
        project_name: "Laser Lab",
        role_key: "trained",
        display_name: "Trained",
      },
    ],
  }),
}));

vi.mock("@/components/names", () => ({ ProjectName: () => null }));

function rule(overrides: Partial<MappingRuleRow> = {}): MappingRuleRow {
  return {
    id: "aaaabbbb-1111-2222-3333-444455556666",
    source_project: "pDoor",
    source_role: "member",
    target_project: "pLaser",
    target_role: "trained",
    confirmation_mode: "manual",
    holder_count: 12,
    created_at: "2026-01-01T00:00:00Z",
    ...overrides,
  };
}

function openTheRule() {
  render(<AutomaticRulesPage />);
  fireEvent.click(screen.getByText(/^R-/));
}

beforeEach(() => {
  state.rules = [rule()];
  state.remove = vi.fn().mockResolvedValue({ message: "ok", cascade: { enqueued: 12, mode: "manual" } });
});

afterEach(cleanup);

describe("deleting an automatic rule", () => {
  // Two clicks and a stated consequence in between. The editor is where you already are when
  // you decide to retire a rule, so the confirmation takes over that same dialog.
  it("asks before it deletes, in the dialog already open", () => {
    openTheRule();
    fireEvent.click(screen.getByRole("button", { name: "Delete rule" }));

    expect(state.remove).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: "Delete rule and revoke access" })).toBeTruthy();
    // The form is gone — the question is not asked over the top of the thing it is about.
    expect(screen.queryByRole("button", { name: "Save rule" })).toBeNull();
  });

  // The holder count is the number that decides whether this is a cleanup or an outage.
  it("names how many people the rule acts on", () => {
    openTheRule();
    fireEvent.click(screen.getByRole("button", { name: "Delete rule" }));

    expect(document.body.textContent).toMatch(/12 people have .* from this rule/);
    // And that holding it elsewhere keeps it — the backend works this out per person, so the
    // dialog must not promise a precise loss.
    expect(document.body.textContent).toMatch(/unless a bundle or a direct grant also gives it/);
  });

  // A queued rule's revokes queue too. Saying "applied" here would have an operator believe a
  // door was locked while it was still open.
  it("says where the revokes go, per the rule's own mode", () => {
    state.rules = [rule({ confirmation_mode: "manual" })];
    const { unmount } = render(<AutomaticRulesPage />);
    fireEvent.click(screen.getByText(/^R-/));
    fireEvent.click(screen.getByRole("button", { name: "Delete rule" }));
    expect(document.body.textContent).toMatch(/waits under Pending changes/);
    unmount();

    state.rules = [rule({ confirmation_mode: "auto" })];
    render(<AutomaticRulesPage />);
    fireEvent.click(screen.getByText(/^R-/));
    fireEvent.click(screen.getByRole("button", { name: "Delete rule" }));
    expect(document.body.textContent).toMatch(/ends at once, with no review/);
  });

  it("says nothing is taken back when nobody holds the trigger role", () => {
    state.rules = [rule({ holder_count: 0 })];
    openTheRule();
    fireEvent.click(screen.getByRole("button", { name: "Delete rule" }));

    expect(document.body.textContent).toMatch(/deleting it revokes nothing/i);
  });

  it("backs out to the editor rather than closing everything", () => {
    openTheRule();
    fireEvent.click(screen.getByRole("button", { name: "Delete rule" }));
    fireEvent.click(screen.getByRole("button", { name: "Keep the rule" }));

    expect(screen.getByRole("button", { name: "Save rule" })).toBeTruthy();
    expect(state.remove).not.toHaveBeenCalled();
  });

  it("deletes on the confirming click", async () => {
    openTheRule();
    fireEvent.click(screen.getByRole("button", { name: "Delete rule" }));
    fireEvent.click(screen.getByRole("button", { name: "Delete rule and revoke access" }));

    await waitFor(() =>
      expect(state.remove).toHaveBeenCalledWith("aaaabbbb-1111-2222-3333-444455556666"),
    );
  });

  // A rule that does not exist yet cannot be retired.
  it("offers no delete on the create form", () => {
    render(<AutomaticRulesPage />);
    fireEvent.click(screen.getByRole("button", { name: "New rule" }));

    expect(screen.queryByRole("button", { name: "Delete rule" })).toBeNull();
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
      .mockResolvedValue({ cascade: { enqueued: 3, mode: "auto" } });
    openTheRule();
    fireEvent.click(screen.getByRole("button", { name: "Delete rule" }));
    fireEvent.click(screen.getByRole("button", { name: "Delete rule and revoke access" }));
    await waitFor(() => expect(state.remove).toHaveBeenCalled());
  }

  it("keeps the report on screen and retires the destructive action", async () => {
    await deleteTheRule();

    await waitFor(() => expect(document.body.textContent).toMatch(/deleted/i));
    expect(screen.queryByRole("button", { name: "Delete rule and revoke access" })).toBeNull();
    expect(screen.queryByRole("button", { name: "Keep the rule" })).toBeNull();
    expect(screen.getByRole("button", { name: "Done" })).toBeTruthy();
  });

  it("closes the editor on dismiss, rather than returning to it", async () => {
    await deleteTheRule();
    await waitFor(() => expect(screen.getByRole("button", { name: "Done" })).toBeTruthy());

    fireEvent.click(screen.getByRole("button", { name: "Done" }));

    // Not back to the form: there is no rule behind it any more. `Keep the
    // rule` would have landed here instead, which is the defect.
    await waitFor(() =>
      expect(screen.queryByRole("button", { name: "Save rule" })).toBeNull(),
    );
    expect(screen.queryByRole("button", { name: "Delete rule" })).toBeNull();
  });

  it("returns to the editor when the delete failed", async () => {
    state.remove = vi.fn().mockRejectedValue(new Error("Zitadel is unreachable"));
    openTheRule();
    fireEvent.click(screen.getByRole("button", { name: "Delete rule" }));
    fireEvent.click(screen.getByRole("button", { name: "Delete rule and revoke access" }));

    await waitFor(() => expect(document.body.textContent).toMatch(/unreachable/i));
    expect(screen.getByRole("button", { name: "Delete rule and revoke access" })).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Done" })).toBeNull();

    // The rule is still there, so backing out puts the operator back on it.
    fireEvent.click(screen.getByRole("button", { name: "Keep the rule" }));
    await waitFor(() => expect(screen.getByRole("button", { name: "Save rule" })).toBeTruthy());
  });
});
