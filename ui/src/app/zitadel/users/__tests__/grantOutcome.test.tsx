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

// A success used to close the dialog onto the row; the sheet now becomes its result (A-17 G4).
describe("replacing roles in Zitadel", () => {
  it("ends on its result in the sheet, with the act's word and one Close", async () => {
    state.update.mockResolvedValue(undefined);
    render(<UpstreamUsersPage />);
    submitEdit();

    const dialog = screen.getByRole("dialog");
    await waitFor(() => expect(within(dialog).getByText("Replaced")).toBeTruthy());
    expect(dialog.textContent).toMatch(/Roles replaced in Zitadel/);
    expect(within(dialog).queryByRole("button", { name: "Replace roles in Zitadel" })).toBeNull();
    expect(within(dialog).queryByRole("button", { name: "Cancel" })).toBeNull();
    fireEvent.click(within(dialog).getByRole("button", { name: "Close" }));
    expect(screen.queryByRole("dialog")).toBeNull();
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
