// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import AutomaticRulesPage from "@/app/policies/page";
import type { MappingRuleRow } from "@/lib/queries/useMappingRules";

const state = vi.hoisted(() => ({
  rules: [] as MappingRuleRow[],
  update: vi.fn(),
}));

vi.mock("@/lib/queries/useMappingRules", () => ({
  useMappingRules: () => ({ data: state.rules, isLoading: false, error: null, refetch: () => {} }),
  useCreateMappingRule: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useUpdateMappingRule: () => ({ mutateAsync: state.update, isPending: false }),
  useDeleteMappingRule: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useSetRuleConfirmationMode: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useValidateMappingRule: () => ({ mutateAsync: vi.fn().mockResolvedValue({}), isPending: false }),
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
    confirmation_mode: "manual",
    holder_count: 12,
    created_at: "2026-01-01T00:00:00Z",
    ...overrides,
  };
}

async function saveTheOpenRule() {
  render(<AutomaticRulesPage />);
  fireEvent.click(screen.getByText(/^R-/));
  fireEvent.click(screen.getByRole("button", { name: "Check rule" }));
  await waitFor(() => expect(screen.getByRole("button", { name: "Check again" })).toBeTruthy());
  fireEvent.click(screen.getByRole("button", { name: "Save rule" }));
}

beforeEach(() => {
  state.rules = [rule()];
  state.update = vi.fn().mockResolvedValue({});
});

afterEach(cleanup);

/**
 * Save used to call onClose() in the same tick as setOutcome(), which threw
 * away the outcome — including whether the rule's access waits under Pending
 * changes or applied at once — before the operator ever saw it, and the pill
 * said "Applied" regardless of mode even when it had painted.
 */
/**
 * The editor's title is only ever the rule's short id ("R-aaaa") — meaningless
 * to the person deciding what they're changing. The chip above it names the
 * actual project/role pair so the id is never the only label in the dialog.
 */
describe("opening a rule to edit", () => {
  it("names the rule by its project/role pair, not only its id", () => {
    render(<AutomaticRulesPage />);
    fireEvent.click(screen.getByText(/^R-/));

    expect(document.body.textContent).toMatch(/pDoor\/member/);
    expect(document.body.textContent).toMatch(/pLaser\/trained/);
  });
});

describe("after saving a rule", () => {
  it("keeps the dialog open and reports where the access goes, for a manual rule", async () => {
    await saveTheOpenRule();

    await waitFor(() => expect(document.body.textContent).toMatch(/Rule updated/));
    // Not "Applied" — a manual rule's access still waits under Pending changes,
    // so the pill and the detail sentence must agree.
    expect(document.body.textContent).toMatch(/Waiting to be sent/);
    expect(document.body.textContent).toMatch(/waits under Pending changes/);
    expect(screen.queryByRole("button", { name: "Save rule" })).toBeNull();
    expect(screen.getByRole("button", { name: "Done" })).toBeTruthy();
  });

  it("reports Applied for an auto rule, and closes on Done", async () => {
    state.rules = [rule({ confirmation_mode: "auto" })];
    await saveTheOpenRule();

    await waitFor(() => expect(document.body.textContent).toMatch(/Applied/));
    expect(document.body.textContent).toMatch(/takes effect the moment the rule applies|get the second at once/);

    fireEvent.click(screen.getByRole("button", { name: "Done" }));
    await waitFor(() => expect(screen.queryByRole("button", { name: "Done" })).toBeNull());
  });
});
