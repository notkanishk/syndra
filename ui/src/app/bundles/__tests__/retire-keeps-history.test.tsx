// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { BundlePage } from "@/components/bundles/BundlePage";

/**
 * Retire keeps its history (owner, 19 Oct 2026): the dialog says what Retire does now, and a
 * bundle that is no longer on the list opens as a retired bundle's page instead of "not found".
 */

const state = vi.hoisted(() => ({ bundles: [] as unknown[] }));

vi.mock("next/navigation", () => ({ useRouter: () => ({ push: vi.fn() }) }));

vi.mock("@/components/bundles/RetiredBundle", () => ({
  RetiredBundlePage: ({ bundleId }: { bundleId: string }) => <div>retired page for {bundleId}</div>,
}));

vi.mock("@/lib/queries/useBundles", () => ({
  useBundleRolesByBundle: () => ({ byId: {}, allLoaded: true }),
  useSetBundleInclude: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useRemoveBundleInclude: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useBundles: () => ({ data: state.bundles, isLoading: false, error: null, refetch: vi.fn() }),
  useBundleRoles: () => ({ data: [], isLoading: false, refetch: vi.fn() }),
  useBundleImpact: () => ({ data: null, isLoading: false }),
  useAddBundleRole: () => ({ mutate: vi.fn(), isPending: false }),
  useDeleteBundle: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useRemoveBundleRole: () => ({ mutateAsync: vi.fn(), isPending: false }),
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
  RoleRef: ({ roleKey }: { roleKey: string }) => <span>{roleKey}</span>,
  UserName: ({ id }: { id: string }) => <span>{id}</span>,
}));
vi.mock("@/components/bundles/BundleVersions", () => ({ BundleVersions: () => null, ThroughOtherBundles: () => null }));

beforeEach(() => {
  state.bundles = [{ id: "b1", name: "Lab Tech", holder_count: 2 }];
});

afterEach(cleanup);

describe("retiring a bundle", () => {
  it("says what Retire does: people lose what it gave, it can't be given again, its history stays", () => {
    render(<BundlePage bundleId="b1" />);
    fireEvent.click(screen.getByRole("button", { name: "Retire bundle" }));

    expect(screen.getByText(/Everyone holding it loses/)).toBeInTheDocument();
    expect(screen.getByText(/can.t be given again\. Its history stays readable\./)).toBeInTheDocument();
    // The old promise was that the history went with it; it no longer does.
    expect(screen.queryByText(/history goes with it/)).toBeNull();
    expect(screen.queryByText(/cannot be brought back/)).toBeNull();
  });
});

describe("a bundle that is not on the list", () => {
  it("opens as a retired bundle's page rather than as not found", () => {
    state.bundles = [{ id: "other", name: "Other" }];
    render(<BundlePage bundleId="b1" />);

    expect(screen.getByText("retired page for b1")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Retire bundle" })).toBeNull();
  });
});
