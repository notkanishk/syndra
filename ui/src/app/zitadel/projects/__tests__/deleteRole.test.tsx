// @vitest-environment jsdom
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import UpstreamProjectsPage from "@/app/zitadel/projects/page";
import { ApiError } from "@/lib/api-client";

const state = vi.hoisted(() => ({ remove: vi.fn() }));

vi.mock("@/lib/queries/useUpstream", () => ({
  useUpstreamProjects: () => ({
    data: { items: [{ id: "pLaser", name: "Laser Lab" }] },
    isLoading: false,
    error: null,
    refetch: () => {},
  }),
  useUpstreamProjectRoles: () => ({
    data: { items: [{ key: "trained", displayName: "Trained", group: "" }] },
    isLoading: false,
    error: null,
    refetch: () => {},
  }),
  useUpstreamCreateRole: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useUpstreamUpdateRole: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useUpstreamDeleteRole: () => ({ mutateAsync: state.remove, isPending: false }),
}));

const held = new ApiError(409, {
  error: "ROLE_HELD",
  message: "2 people hold this role in Zitadel and would lose it. Nothing was changed.",
  details: { count: "2", holders: "Ada Lovelace, Grace Hopper" },
});

beforeEach(() => {
  state.remove = vi.fn();
});

function pressDelete() {
  render(<UpstreamProjectsPage />);
  fireEvent.click(screen.getByRole("button", { name: "Delete Trained" }));
  fireEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Delete role in Zitadel" }));
}

// Deleting a role in Zitadel takes it from everybody who holds it. The count
// that confirms it comes from Syndra's refusal, never from this screen.
describe("deleting a role somebody holds", () => {
  it("names who loses it, then confirms with exactly that count", async () => {
    state.remove.mockRejectedValueOnce(held).mockResolvedValueOnce(undefined);
    pressDelete();

    const dialog = screen.getByRole("dialog");
    await waitFor(() => expect(dialog.textContent).toMatch(/Ada Lovelace, Grace Hopper/));
    const confirm = within(dialog).getByRole("button", { name: "Delete and revoke it for 2 people" });
    expect(confirm).toBeDisabled();

    fireEvent.click(within(dialog).getByRole("checkbox", { name: /revokes this role for 2 people/ }));
    fireEvent.click(within(dialog).getByRole("button", { name: "Delete and revoke it for 2 people" }));

    await waitFor(() =>
      expect(state.remove).toHaveBeenLastCalledWith({ projectId: "pLaser", key: "trained", confirmHolders: 2 }),
    );
    await waitFor(() => expect(dialog.textContent).toMatch(/trained deleted in Zitadel/));
  });

  it("deletes a role nobody holds without a count", async () => {
    state.remove.mockResolvedValue(undefined);
    pressDelete();
    await waitFor(() =>
      expect(state.remove).toHaveBeenCalledWith({ projectId: "pLaser", key: "trained", confirmHolders: undefined }),
    );
  });

  it("reports any other refusal in place", async () => {
    state.remove.mockRejectedValue(
      new ApiError(409, { error: "ROLE_IN_USE", message: "Syndra still gives this role (1 bundle)." }),
    );
    pressDelete();
    const report = await screen.findByRole("alert");
    expect(report.textContent).toContain("Syndra still gives this role");
  });
});
