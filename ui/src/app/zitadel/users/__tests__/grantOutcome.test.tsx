// @vitest-environment jsdom
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import UpstreamUsersPage from "@/app/zitadel/users/page";

const state = vi.hoisted(() => ({ update: vi.fn() }));

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
  useUpstreamUpdateGrant: () => ({ mutateAsync: state.update, isPending: false }),
  useUpstreamRemoveGrant: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));

vi.mock("@/components/names", () => ({ ProjectName: () => null }));

beforeEach(() => {
  state.update = vi.fn();
});

function submitEdit() {
  fireEvent.click(screen.getByRole("button", { name: "Change roles" }));
  const dialog = screen.getByRole("dialog");
  fireEvent.click(within(dialog).getByRole("checkbox"));
  fireEvent.click(within(dialog).getByRole("button", { name: "Replace roles in Zitadel" }));
}

// A success closed the dialog and took its outcome with it: the operator saw
// nothing say the change had happened.
describe("replacing roles in Zitadel", () => {
  it("keeps its outcome visible on the page after the dialog closes", async () => {
    state.update.mockResolvedValue(undefined);
    render(<UpstreamUsersPage />);
    submitEdit();

    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(document.body.textContent).toMatch(/Roles replaced in Zitadel/);
  });

  it("keeps a failure in the dialog, beside the retry", async () => {
    state.update.mockRejectedValue(new Error("nope"));
    render(<UpstreamUsersPage />);
    submitEdit();

    await waitFor(() => expect(state.update).toHaveBeenCalled());
    const dialog = screen.getByRole("dialog");
    await waitFor(() => expect(within(dialog).getByRole("alert")).toBeTruthy());
    expect(document.body.textContent).not.toMatch(/Roles replaced in Zitadel/);
  });
});
