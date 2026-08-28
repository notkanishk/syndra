// @vitest-environment jsdom
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import UpstreamUsersPage from "@/app/zitadel/users/page";

const state = vi.hoisted(() => ({ remove: vi.fn() }));

vi.mock("@/lib/queries/useUpstream", () => ({
  useUpstreamUsers: () => ({
    data: { items: [{ id: "u1", displayName: "Priya", userName: "priya", email: "p@x", state: "USER_STATE_ACTIVE" }] },
    isLoading: false,
    error: null,
    refetch: () => {},
  }),
  useUpstreamUserGrants: () => ({
    data: { items: [{ id: "g1", projectId: "pLaser", roleKeys: ["trained"] }] },
    isLoading: false,
    error: null,
    refetch: () => {},
  }),
  useUpstreamProjects: () => ({ data: { items: [] } }),
  useUpstreamProjectRoles: () => ({ data: { items: [] } }),
  useUpstreamAssignGrant: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useUpstreamUpdateGrant: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useUpstreamRemoveGrant: () => ({ mutateAsync: state.remove, isPending: false }),
}));

vi.mock("@/components/names", () => ({ ProjectName: () => null }));

beforeEach(() => {
  state.remove = vi.fn().mockResolvedValue(undefined);
});

// Revoking in Zitadel used to be one click with no sentence beside it — the
// only direct-to-Zitadel change with no consequence and no tick.
describe("revoking a person's roles in Zitadel", () => {
  it("asks first, names the consequence, and unlocks on the tick", async () => {
    render(<UpstreamUsersPage />);
    fireEvent.click(screen.getByRole("button", { name: "Revoke roles" }));

    expect(state.remove).not.toHaveBeenCalled();
    expect(document.body.textContent).toMatch(/Priya loses these roles in Zitadel at once/);
    const confirm = screen.getByRole("button", { name: "Revoke this role in Zitadel" });
    expect(confirm).toBeDisabled();

    fireEvent.click(screen.getByRole("checkbox", { name: /I understand this revokes 1 role/ }));
    fireEvent.click(confirm);

    await waitFor(() => expect(state.remove).toHaveBeenCalledWith({ userId: "u1", grantId: "g1" }));
    await waitFor(() => expect(document.body.textContent).toMatch(/Roles revoked in Zitadel/));
  });

  it("keeps the roles when backed out of", () => {
    render(<UpstreamUsersPage />);
    fireEvent.click(screen.getByRole("button", { name: "Revoke roles" }));
    fireEvent.click(screen.getByRole("button", { name: "Keep the roles" }));
    expect(state.remove).not.toHaveBeenCalled();
    expect(screen.queryByRole("dialog")).toBeNull();
  });
});
