// @vitest-environment jsdom
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { BundlePage } from "@/components/bundles/BundlePage";

/**
 * `RemovalImpact` opens inline under the role's row (its "Remove" verb) and has
 * no `open`/`onClose` of its own — it stays mounted until `onCancel` clears the
 * row being removed. Nothing
 * cleared it on success and the button was never wrapped, so "Drop it from
 * the working copy" stayed live after the drop had already landed, and the
 * next click dropped a role already gone from the draft.
 */

const state = vi.hoisted(() => ({
  bundles: [] as unknown[],
  roles: [] as unknown[],
  remove: vi.fn(),
}));

vi.mock("next/navigation", () => ({ useRouter: () => ({ push: vi.fn() }) }));

vi.mock("@/lib/queries/useBundles", () => ({
  useBundleRolesByBundle: () => ({ byId: {}, allLoaded: true }),
  useSetBundleInclude: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useRemoveBundleInclude: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useBundles: () => ({ data: state.bundles, isLoading: false, refetch: vi.fn() }),
  useBundleRoles: () => ({ data: state.roles, isLoading: false, refetch: vi.fn() }),
  useBundleImpact: () => ({ data: { users: [] }, isLoading: false }),
  useAddBundleRole: () => ({ mutate: vi.fn(), isPending: false }),
  useCreateBundle: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useDeleteBundle: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useRemoveBundleRole: () => ({ mutateAsync: state.remove, isPending: false }),
  useSetWelcomeBundle: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useUpdateBundle: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));

vi.mock("@/lib/queries/useBundleVersions", () => ({
  useVersionsByBundle: () => ({}),
  useBundleIncluders: () => ({ data: [] }),
  useBundleHolders: () => ({ data: [] }),
  useBundleDraft: () => ({ data: undefined }),
  useBundleVersions: () => ({ data: [], isLoading: false, refetch: vi.fn() }),
  useDiscardBundleDraft: () => ({ mutateAsync: vi.fn(), isPending: false }),
  draftChangeCount: () => 0,
}));

vi.mock("@/lib/queries/useMappingRules", () => ({ useMappingRules: () => ({ data: [] }) }));
vi.mock("@/lib/queries/useRoles", () => ({ useGlobalRoleCatalog: () => ({ data: [] }) }));
vi.mock("@/components/names", () => ({
  ProjectName: ({ id, name }: { id: string; name?: string }) => <span>{name || id}</span>,
  RoleRef: ({ roleKey }: { roleKey: string }) => <span>{roleKey}</span>,
  UserName: ({ id }: { id: string }) => <span>{id}</span>,
}));
vi.mock("@/components/bundles/BundleVersions", () => ({ BundleVersions: () => null, ThroughOtherBundles: () => null }));

beforeEach(() => {
  state.bundles = [{ id: "b1", name: "Lab Tech", holder_count: 2 }];
  state.roles = [{ zitadel_project_id: "p1", zitadel_role_key: "operator" }];
  state.remove = vi.fn().mockResolvedValue({});
});

describe("dropping a role from a bundle's working copy", () => {
  it("cannot be pressed a second time once it has landed", async () => {
    render(<BundlePage bundleId="b1" />);

    fireEvent.click(screen.getByRole("button", { name: "Remove" }));
    fireEvent.click(screen.getByRole("button", { name: "Drop it from the working copy" }));

    await waitFor(() => expect(screen.getByRole("button", { name: "Close" })).toBeInTheDocument());
    expect(screen.queryByRole("button", { name: "Drop it from the working copy" })).toBeNull();
    expect(state.remove).toHaveBeenCalledTimes(1);
  });

  it("keeps the button when the drop fails, so there is a retry", async () => {
    state.remove = vi.fn().mockRejectedValue(new Error("nope"));
    render(<BundlePage bundleId="b1" />);

    fireEvent.click(screen.getByRole("button", { name: "Remove" }));
    fireEvent.click(screen.getByRole("button", { name: "Drop it from the working copy" }));

    await waitFor(() => expect(screen.getByText(/Nothing was changed/i)).toBeInTheDocument());
    expect(screen.getByRole("button", { name: "Drop it from the working copy" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Close" })).toBeNull();
  });
});
