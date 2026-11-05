// @vitest-environment jsdom
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import UpstreamUsersPage from "@/app/zitadel/users/page";

const state = vi.hoisted(() => ({
  remove: vi.fn(),
  roleKeys: ["trained"] as string[],
  // The server's own answer to "what would this revoke" (re-audit R18):
  // undefined data is a preview that couldn't be read.
  preview: { revoked_roles: ["pLaser/trained"], retained_roles: [] as string[] } as
    | { revoked_roles: string[]; retained_roles: string[] }
    | undefined,
  waits: [] as Array<{ project_id: string; role_key: string }>,
}));

vi.mock("@/lib/queries/useUpstream", () => ({
  useUpstreamUsers: () => ({
    data: { items: [{ id: "u1", displayName: "Priya", userName: "priya", email: "priya@example.org", state: "USER_STATE_ACTIVE" }] },
    isLoading: false,
    error: null,
    refetch: () => {},
  }),
  useUpstreamUserGrants: () => ({
    data: { items: [{ id: "g1", projectId: "pLaser", roleKeys: state.roleKeys }] },
    isLoading: false,
    error: null,
    refetch: () => {},
  }),
  useUpstreamProjects: () => ({ data: { items: [] } }),
  useUpstreamProjectRoles: () => ({ data: { items: [] } }),
  useUpstreamAssignGrant: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useUpstreamUpdateGrant: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useUpstreamRemoveGrant: () => ({ mutateAsync: state.remove, isPending: false }),
  useGrantRemovalPreview: () => ({ isLoading: false, data: state.preview }),
}));

vi.mock("@/lib/queries/useRolesThatWait", () => ({
  useRolesThatWait: () => ({ data: { roles: state.waits } }),
}));

vi.mock("@/components/names", () => ({ ProjectName: () => null }));

beforeEach(() => {
  state.remove = vi.fn().mockResolvedValue(undefined);
  state.roleKeys = ["trained"];
  state.preview = { revoked_roles: ["pLaser/trained"], retained_roles: [] };
  state.waits = [];
});

function openRevoke() {
  render(<UpstreamUsersPage />);
  fireEvent.click(screen.getByRole("button", { name: "Revoke roles" }));
}

// Revoking in Zitadel used to be one click with no sentence beside it — the
// only direct-to-Zitadel change with no consequence and no tick.
describe("revoking a person's roles in Zitadel", () => {
  it("asks first, names the consequence, and unlocks on the tick", async () => {
    openRevoke();

    expect(state.remove).not.toHaveBeenCalled();
    expect(document.body.textContent).toMatch(/Priya loses it once the change reaches Zitadel/);
    expect(document.body.textContent).toMatch(/It is sent to Zitadel straight away\./);
    expect(screen.getByRole("button", { name: "Revoke this role in Zitadel" })).toBeDisabled();

    fireEvent.click(screen.getByRole("checkbox", { name: /I understand this revokes 1 role/ }));
    // Re-query rather than reuse the disabled-state node: a disabled control
    // now renders its reason as a sibling (Button.tsx wraps in a <span> only
    // when `reason` is set), so acknowledging swaps the tree and the old
    // reference goes stale.
    fireEvent.click(screen.getByRole("button", { name: "Revoke this role in Zitadel" }));

    await waitFor(() => expect(state.remove).toHaveBeenCalledWith({ userId: "u1", grantId: "g1" }));
    // The result is in the sheet, in the act's word, and the sheet stays until Close.
    await waitFor(() => expect(screen.getByText("Revoked")).toBeTruthy());
    expect(document.body.textContent).toMatch(/Roles revoked in Zitadel/);
    expect(screen.getByRole("dialog")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Close" }));
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("keeps the roles when backed out of", () => {
    openRevoke();
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    expect(state.remove).not.toHaveBeenCalled();
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  // C4: the lede says untraced roles go too, and the count is only what goes.
  it("counts only the roles that will be removed, and names the ones that stay", () => {
    state.roleKeys = ["trained", "lead", "stray"];
    // The server's answer, not a walk here: it is what the revoke then does.
    state.preview = { revoked_roles: ["pLaser/trained", "pLaser/stray"], retained_roles: ["pLaser/lead"] };
    openRevoke();

    expect(document.body.textContent).toMatch(/Every other role on this grant is revoked, including any Syndra has no record of/);
    expect(document.body.textContent).toMatch(/Stays, because a bundle or a rule still gives it: lead\./);
    expect(screen.getByRole("checkbox", { name: /I understand this revokes 2 roles/ })).toBeTruthy();
    expect(screen.getByRole("button", { name: "Revoke these 2 roles in Zitadel" })).toBeTruthy();
  });

  // Re-audit R18: no consent on a count the server didn't give.
  it("won't ask for a count it couldn't read", () => {
    state.preview = undefined;
    openRevoke();
    expect(document.body.textContent).toMatch(/couldn.t read what this would revoke/);
    expect(screen.queryByRole("checkbox")).toBeNull();
    expect(within(screen.getByRole("dialog")).getByRole("button", { name: /in Zitadel$/ })).toBeDisabled();
  });

  it("keeps a role a rule gives from a role that stays", () => {
    state.roleKeys = ["trained", "lead"];
    state.preview = { revoked_roles: [], retained_roles: ["pLaser/lead", "pLaser/trained"] };
    openRevoke();
    expect(document.body.textContent).toMatch(/Nothing on this grant would be revoked/);
    expect(screen.queryByRole("checkbox")).toBeNull();
    expect(within(screen.getByRole("dialog")).getByRole("button", { name: /in Zitadel$/ })).toBeDisabled();
  });

  it("names a role that waits in the consequence", () => {
    state.waits = [{ project_id: "pLaser", role_key: "trained" }];
    openRevoke();
    expect(document.body.textContent).toMatch(/except trained: that role always waits for you/);
  });

  // The backend keeps a role a bundle or a rule still gives (W1a); the result
  // says which, in its words rather than this screen's guess.
  it("says which roles it kept because something else still gives them", async () => {
    state.remove = vi.fn().mockResolvedValue({
      revoked_roles: [],
      retained_roles: ["pLaser/trained"],
      status: "unchanged",
      roles: [{ role: "pLaser/trained", change: "kept", status: "kept" }],
    });
    openRevoke();
    fireEvent.click(screen.getByRole("checkbox", { name: /I understand this revokes 1 role/ }));
    fireEvent.click(screen.getByRole("button", { name: "Revoke this role in Zitadel" }));

    await waitFor(() => expect(screen.getByText("Nothing revoked")).toBeTruthy());
    expect(document.body.textContent).toMatch(/Kept, because a bundle or a rule still gives it: trained\./);
    expect(document.body.textContent).not.toMatch(/Roles revoked in Zitadel/);
  });

  // W8: each role's real status, never "done" for one that is not in Zitadel yet.
  it("shows each role's real status", async () => {
    state.roleKeys = ["trained", "stray"];
    state.preview = { revoked_roles: ["pLaser/trained", "pLaser/stray"], retained_roles: [] };
    state.remove = vi.fn().mockResolvedValue({
      status: "pending",
      roles: [
        { role: "pLaser/trained", change: "revoked", status: "applied" },
        { role: "pLaser/stray", change: "revoked", status: "pending" },
      ],
    });
    openRevoke();
    fireEvent.click(screen.getByRole("checkbox", { name: /I understand this revokes 2 roles/ }));
    fireEvent.click(screen.getByRole("button", { name: "Revoke these 2 roles in Zitadel" }));

    await waitFor(() => expect(document.body.textContent).toMatch(/Revoked in Zitadel: trained\./));
    expect(document.body.textContent).toMatch(/Not revoked in Zitadel yet, because Zitadel didn't take it\. Revocations send on their own: stray\./);
    expect(screen.getByText("On its way")).toBeTruthy();
    expect(document.body.textContent).not.toMatch(/Roles revoked in Zitadel/);
  });
});
