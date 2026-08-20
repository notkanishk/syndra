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
