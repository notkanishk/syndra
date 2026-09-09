// @vitest-environment jsdom
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import BundlesPage from "@/app/bundles/page";

/**
 * A dialog must outlive the mutation it is reporting on.
 *
 * `DeleteBundleDialog` deliberately stays open after the delete lands, to say
 * how many revocations were queued and whether they reached Zitadel — and the
 * page's own comment says clearing the selection early throws away an outcome
 * the operator has not read yet.
 *
 * The list refetch was doing exactly that. `useDeleteBundle` invalidates the
 * bundle list, so the deleted row vanished from `rows` and the inline
 * `rows.find(...)` lookup either returned undefined (unmounting the workspace
 * that owns the dialog) or fell through to `rows[0]`, remounting the workspace
 * on a different bundle. The report was destroyed before it could be read and
 * the dialog's own Done button was unreachable.
 */

const state = vi.hoisted(() => ({
  bundles: [] as unknown[],
  remove: vi.fn(),
}));

vi.mock("@/lib/queries/useBundles", () => ({
  useBundles: () => ({ data: state.bundles, isLoading: false, refetch: vi.fn() }),
  useBundleRoles: () => ({ data: [], isLoading: false, refetch: vi.fn() }),
  useBundleImpact: () => ({ data: null, isLoading: false }),
  useCreateBundle: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useDeleteBundle: () => ({ mutateAsync: state.remove, isPending: false }),
  useRemoveBundleRole: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useSetWelcomeBundle: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useUpdateBundle: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));

vi.mock("@/lib/queries/useBundleVersions", () => ({
  useBundleDraft: () => ({ data: undefined }),
  draftChangeCount: () => 0,
}));

vi.mock("@/lib/queries/useMappingRules", () => ({ useMappingRules: () => ({ data: [] }) }));
vi.mock("@/lib/queries/useRoles", () => ({ useGlobalRoleCatalog: () => ({ data: [] }) }));
vi.mock("@/components/names", () => ({
  RoleRef: ({ roleKey }: { roleKey: string }) => <span>{roleKey}</span>,
  UserName: ({ id }: { id: string }) => <span>{id}</span>,
}));
vi.mock("@/components/bundles/BundleVersions", () => ({ BundleVersions: () => null }));

beforeEach(() => {
  state.bundles = [{ id: "b1", name: "Lab Tech", holder_count: 2 }];
  state.remove = vi.fn().mockResolvedValue({
    message: "Bundle deleted",
    was_welcome: false,
    cascade: { enqueued: 4, mode: "manual" },
  });
});

describe("deleting a bundle", () => {
  it("keeps the report on screen after the list stops listing the bundle", async () => {
    const { rerender } = render(<BundlesPage />);

    fireEvent.click(screen.getByRole("button", { name: "Delete bundle" }));
    fireEvent.click(screen.getByRole("button", { name: "Delete and revoke" }));

    await waitFor(() => expect(state.remove).toHaveBeenCalled());

    // What the invalidation does: the bundle is gone from the list.
    state.bundles = [];
    rerender(<BundlesPage />);

    // The report survives, and names the queued revocations.
    expect(screen.getByText(/4 changes waiting/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Done" })).toBeInTheDocument();
  });

  it("lets go of the bundle only when the operator presses Done", async () => {
    const { rerender } = render(<BundlesPage />);

    fireEvent.click(screen.getByRole("button", { name: "Delete bundle" }));
    fireEvent.click(screen.getByRole("button", { name: "Delete and revoke" }));
    await waitFor(() => expect(state.remove).toHaveBeenCalled());

    state.bundles = [];
    rerender(<BundlesPage />);

    fireEvent.click(screen.getByRole("button", { name: "Done" }));

    expect(screen.queryByRole("button", { name: "Done" })).toBeNull();
    expect(screen.getByText("No bundles yet.")).toBeInTheDocument();
  });
});
