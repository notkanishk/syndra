// @vitest-environment jsdom
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import AutomaticRulesPage from "@/app/policies/page";
import { ApiError } from "@/lib/api-client";
import type { MappingRuleRow } from "@/lib/queries/useMappingRules";

const state = vi.hoisted(() => ({
  bulk: vi.fn(),
}));

vi.mock("@/lib/queries/useMappingRules", () => ({
  useMappingRules: () => ({
    data: [
      {
        id: "aaaabbbb-1111-2222-3333-444455556666",
        source_project: "pDoor",
        source_role: "member",
        target_project: "pLaser",
        target_role: "trained",
        confirmation_mode: "manual",
        holder_count: 12,
        created_at: "2026-01-01T00:00:00Z",
      } satisfies MappingRuleRow,
    ],
    isLoading: false,
    error: null,
    refetch: () => {},
  }),
  useCreateMappingRule: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useUpdateMappingRule: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useDeleteMappingRule: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useSetRuleConfirmationMode: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useValidateMappingRule: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));

vi.mock("@/lib/queries/useConfirmationMode", () => ({
  useBulkSetConfirmationMode: () => ({ mutateAsync: state.bulk, isPending: false }),
}));

vi.mock("@/lib/queries/useProjects", () => ({
  useProjects: () => ({ data: [{ project: { id: "pLaser", name: "Laser Lab" } }] }),
}));

vi.mock("@/lib/queries/useRoles", () => ({
  useGlobalRoleCatalog: () => ({ data: [] }),
}));

vi.mock("@/components/names", () => ({ ProjectName: () => null }));

function selectTheRule() {
  render(<AutomaticRulesPage />);
  fireEvent.click(screen.getByRole("button", { name: "Select" }));
  fireEvent.click(screen.getByRole("checkbox", { name: /Select these 1 rule/ }));
}

beforeEach(() => {
  state.bulk.mockReset();
});

/**
 * These two verbs apply on tap: there is no plan to read first, so the report
 * afterwards is the only account of what happened. The page held one in state
 * and rendered nothing, which meant a refused bulk change looked exactly like
 * an applied one.
 */
describe("the bulk confirmation-mode verbs report their result", () => {
  it("reports a refusal in place, and says nothing changed", async () => {
    state.bulk.mockRejectedValue(new ApiError(403, { error: "FORBIDDEN", message: "Not allowed" }));
    selectTheRule();

    fireEvent.click(screen.getByRole("button", { name: /fire immediately/ }));

    const report = await screen.findByRole("alert");
    expect(report.textContent).toContain("Refused");
    expect(report.textContent).toMatch(/Nothing/i);
  });

  it("reports what it applied, and to how many", async () => {
    state.bulk.mockResolvedValue(undefined);
    selectTheRule();

    fireEvent.click(screen.getByRole("button", { name: /queue for confirmation/ }));

    await waitFor(() =>
      expect(screen.getByRole("status").textContent).toContain(
        "1 rule now queue for confirmation",
      ),
    );
    expect(screen.getByRole("status").textContent).toContain("Applied");
  });
});
