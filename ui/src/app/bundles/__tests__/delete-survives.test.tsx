// @vitest-environment jsdom
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { BundlePage } from "@/components/bundles/BundlePage";

/**
 * A dialog must outlive the mutation it is reporting on.
 *
 * `DeleteBundleDialog` deliberately stays open after the delete lands, to say
 * how many revocations were queued and whether they reached Zitadel.
 *
 * The list refetch used to destroy that report: `useDeleteBundle` invalidates
 * the bundle list, so the row vanished and took the dialog with it. BundlePage
 * now remembers the row it last saw, so the page (and the dialog it owns)
 * outlives the refetch, and leaves for /bundles only when Close is pressed.
 */

const state = vi.hoisted(() => ({
  bundles: [] as unknown[],
  remove: vi.fn(),
  push: vi.fn(),
}));

vi.mock("next/navigation", () => ({ useRouter: () => ({ push: state.push }) }));

vi.mock("@/lib/queries/useBundles", () => ({
  useBundles: () => ({ data: state.bundles, isLoading: false, refetch: vi.fn() }),
  useBundleRoles: () => ({ data: [], isLoading: false, refetch: vi.fn() }),
  useBundleImpact: () => ({ data: null, isLoading: false }),
  useAddBundleRole: () => ({ mutate: vi.fn(), isPending: false }),
  useCreateBundle: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useDeleteBundle: () => ({ mutateAsync: state.remove, isPending: false }),
  useRemoveBundleRole: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useSetWelcomeBundle: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useUpdateBundle: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));

vi.mock("@/lib/queries/useBundleVersions", () => ({
  useBundleDraft: () => ({ data: undefined }),
  useBundleVersions: () => ({ data: [], isLoading: false, refetch: vi.fn() }),
  useDiscardBundleDraft: () => ({ mutateAsync: vi.fn(), isPending: false }),
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
  state.push = vi.fn();
  state.bundles = [{ id: "b1", name: "Lab Tech", holder_count: 2 }];
  state.remove = vi.fn().mockResolvedValue({
    message: "Bundle deleted",
    was_welcome: false,
    cascade: { enqueued: 4, mode: "manual" },
  });
});

describe("deleting a bundle", () => {
  it("keeps the report on screen after the list stops listing the bundle", async () => {
    const { rerender } = render(<BundlePage bundleId="b1" />);

    fireEvent.click(screen.getByRole("button", { name: "Retire bundle" }));
    fireEvent.click(screen.getByRole("button", { name: "Delete and revoke" }));

    await waitFor(() => expect(state.remove).toHaveBeenCalled());

    // What the invalidation does: the bundle is gone from the list.
    state.bundles = [];
    rerender(<BundlePage bundleId="b1" />);

    // The report survives, and names the queued revocations.
    expect(screen.getByText(/4 changes waiting/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Close" })).toBeInTheDocument();
  });

  it("leaves for the list only when the operator presses Close", async () => {
    const { rerender } = render(<BundlePage bundleId="b1" />);

    fireEvent.click(screen.getByRole("button", { name: "Retire bundle" }));
    fireEvent.click(screen.getByRole("button", { name: "Delete and revoke" }));
    await waitFor(() => expect(state.remove).toHaveBeenCalled());

    state.bundles = [];
    rerender(<BundlePage bundleId="b1" />);

    // Not before: the report is being read and the page must not move under it.
    expect(state.push).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "Close" }));

    expect(state.push).toHaveBeenCalledWith("/bundles");
  });
});
