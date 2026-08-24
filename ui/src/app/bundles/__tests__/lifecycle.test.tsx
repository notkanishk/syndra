// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import BundlesPage from "@/app/bundles/page";
import type { BundleRow } from "@/lib/queries/useBundles";

const state = vi.hoisted(() => ({
  bundles: [] as BundleRow[],
  update: vi.fn(),
  remove: vi.fn(),
}));

vi.mock("@/lib/queries/useBundles", () => ({
  useBundles: () => ({ data: state.bundles, isLoading: false, error: null, refetch: () => {} }),
  useBundleRoles: () => ({ data: [], isLoading: false, error: null, refetch: () => {} }),
  useBundleImpact: () => ({ data: { role_count: 0, users: [] }, isLoading: false }),
  useCreateBundle: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useUpdateBundle: () => ({ mutateAsync: state.update, isPending: false }),
  useDeleteBundle: () => ({ mutateAsync: state.remove, isPending: false }),
  useRemoveBundleRole: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useSetWelcomeBundle: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));

vi.mock("@/lib/queries/useBundleVersions", () => ({
  useBundleDraft: () => ({ data: { latest_version: 2, next_version: 3, added: [], removed: [] } }),
  draftChangeCount: () => 0,
}));

vi.mock("@/lib/queries/useMappingRules", () => ({ useMappingRules: () => ({ data: [] }) }));

vi.mock("@/components/bundles/BundleVersions", () => ({ BundleVersions: () => null }));
vi.mock("@/components/names", () => ({
  RoleRef: () => null,
  UserName: () => null,
}));


function bundle(overrides: Partial<BundleRow> = {}): BundleRow {
  return {
    id: "b1",
    name: "Lab Tech",
    description: "Trained on the mill",
    holder_count: 11,
    latest_version: 2,
    ...overrides,
  };
}

beforeEach(() => {
  state.bundles = [bundle()];
  state.update = vi.fn().mockResolvedValue({ message: "ok" });
  state.remove = vi
    .fn()
    .mockResolvedValue({ message: "ok", was_welcome: false, cascade: { enqueued: 4, mode: "auto" } });
});

afterEach(cleanup);

describe("bundle deletion", () => {
  // Deleting a bundle strips access from everybody holding it. The number is the whole reason
  // to hesitate, so it has to be on screen before the click, not in a toast after it.
  it("names the holders before the confirming click", () => {
    render(<BundlesPage />);
    fireEvent.click(screen.getByRole("button", { name: "Delete bundle" }));

    expect(document.body.textContent).toMatch(/11 people hold it right now/);
    expect(state.remove).not.toHaveBeenCalled();
  });

  // Deleting the welcome bundle silently stops onboarding granting anything. That consequence
  // has no other place to be said — the flag lives on the row that is about to go.
  it("warns that onboarding stops when the welcome bundle is the one going", () => {
    state.bundles = [bundle({ is_welcome: true })];
    render(<BundlesPage />);
    fireEvent.click(screen.getByRole("button", { name: "Delete bundle" }));

    expect(document.body.textContent).toMatch(/default for new members/i);
    expect(document.body.textContent).toMatch(/stop handing anything out/i);
  });

  it("says nothing is taken away when nobody holds it", () => {
    state.bundles = [bundle({ holder_count: 0 })];
    render(<BundlesPage />);
    fireEvent.click(screen.getByRole("button", { name: "Delete bundle" }));

    expect(document.body.textContent).toMatch(/takes nothing away from anybody/i);
  });

  it("deletes only on the confirming button", async () => {
    render(<BundlesPage />);
    fireEvent.click(screen.getByRole("button", { name: "Delete bundle" }));
    fireEvent.click(screen.getByRole("button", { name: "Delete and revoke" }));

    await waitFor(() => expect(state.remove).toHaveBeenCalledWith("b1"));
  });
});

describe("bundle rename", () => {
  it("blocks a save that changes nothing", () => {
    render(<BundlesPage />);
    fireEvent.click(screen.getByRole("button", { name: "Rename" }));

    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    expect(state.update).not.toHaveBeenCalled();
    expect(document.body.textContent).toMatch(/Nothing changed yet/);
  });

  it("sends the trimmed name and the description together", async () => {
    render(<BundlesPage />);
    fireEvent.click(screen.getByRole("button", { name: "Rename" }));

    fireEvent.change(screen.getByLabelText("Name"), { target: { value: "  Lab Technician  " } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    await waitFor(() =>
      expect(state.update).toHaveBeenCalledWith({
        id: "b1",
        name: "Lab Technician",
        description: "Trained on the mill",
      }),
    );
  });
});

/**
 * What happens AFTER the delete lands.
 *
 * `DeleteBundleDialog` took an `onDeleted` callback and never called it, so the
 * page went on selecting a bundle that no longer existed. The fix could not
 * call it from the mutation either: clearing the selection unmounts this dialog
 * and would take the outcome the operator has not read yet with it. So the
 * sequence itself is the contract — the report stays, the destructive action
 * goes, and dismissing is what moves the page on.
 */
describe("after a bundle is deleted", () => {
  beforeEach(() => {
    state.bundles = [bundle(), bundle({ id: "b2", name: "Mill Certified", holder_count: 3 })];
  });

  async function deleteTheSecondBundle() {
    render(<BundlesPage />);
    fireEvent.click(screen.getByRole("button", { name: /Mill Certified/ }));
    fireEvent.click(screen.getByRole("button", { name: "Delete bundle" }));
    fireEvent.click(screen.getByRole("button", { name: "Delete and revoke" }));
    await waitFor(() => expect(state.remove).toHaveBeenCalledWith("b2"));
    // The refetch the mutation triggers, which the mocked list cannot do itself.
    state.bundles = [bundle()];
  }

  it("keeps the report on screen and retires the destructive action", async () => {
    await deleteTheSecondBundle();

    await waitFor(() =>
      expect(document.body.textContent).toMatch(/Mill Certified deleted/),
    );
    // Nothing left to destroy, so nothing offers to.
    expect(screen.queryByRole("button", { name: "Delete and revoke" })).toBeNull();
    // And the way out stops being a refusal, because there is nothing to refuse.
    expect(screen.queryByRole("button", { name: "Keep it" })).toBeNull();
    expect(screen.getByRole("button", { name: "Done" })).toBeTruthy();
  });

  it("clears the parent's selection only when the operator dismisses", async () => {
    await deleteTheSecondBundle();
    await waitFor(() => expect(screen.getByRole("button", { name: "Done" })).toBeTruthy());

    // Still pinned to the deleted bundle until this click: the report is being
    // read, and the page must not move under it.
    fireEvent.click(screen.getByRole("button", { name: "Done" }));

    // The selection is released, which re-renders the PAGE — and the page is
    // what holds the list. Closing the dialog alone re-renders the workspace
    // inside it and leaves the deleted bundle on screen, which is exactly what
    // the unwired callback did, so this assertion has to be about the page and
    // not about the dialog.
    await waitFor(() => expect(document.body.textContent).not.toMatch(/Mill Certified/));
    expect(document.body.textContent).toMatch(/Lab Tech/);
    expect(screen.getByRole("button", { name: "Delete bundle" })).toBeTruthy();
  });

  it("keeps the destructive action when the delete failed", async () => {
    state.remove = vi.fn().mockRejectedValue(new Error("the identity provider is unreachable"));
    render(<BundlesPage />);
    fireEvent.click(screen.getByRole("button", { name: "Delete bundle" }));
    fireEvent.click(screen.getByRole("button", { name: "Delete and revoke" }));

    await waitFor(() => expect(document.body.textContent).toMatch(/unreachable/i));
    // Nothing was deleted, so the offer stands and the way out is still a
    // refusal. Relabelling here would tell the operator it worked.
    expect(screen.getByRole("button", { name: "Delete and revoke" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "Keep it" })).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Done" })).toBeNull();
  });
});
