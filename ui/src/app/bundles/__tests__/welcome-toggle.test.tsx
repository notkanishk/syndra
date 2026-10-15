// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { BundlePage } from "@/components/bundles/BundlePage";
import type { BundleRow } from "@/lib/queries/useBundles";

const state = vi.hoisted(() => ({
  bundles: [] as BundleRow[],
  setWelcome: vi.fn(),
}));

vi.mock("next/navigation", () => ({ useRouter: () => ({ push: vi.fn() }) }));

vi.mock("@/lib/queries/useBundles", () => ({
  useBundles: () => ({ data: state.bundles, isLoading: false, error: null, refetch: () => {} }),
  useBundleRoles: () => ({ data: [], isLoading: false, error: null, refetch: () => {} }),
  useBundleImpact: () => ({ data: { role_count: 0, users: [] }, isLoading: false }),
  useAddBundleRole: () => ({ mutate: vi.fn(), isPending: false }),
  useUpdateBundle: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useDeleteBundle: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useRemoveBundleRole: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useSetWelcomeBundle: () => ({ mutateAsync: state.setWelcome, isPending: false }),
}));

vi.mock("@/lib/queries/useBundleVersions", () => ({
  useBundleDraft: () => ({ data: undefined }),
  useBundleVersions: () => ({ data: [], isLoading: false, refetch: () => {} }),
  useDiscardBundleDraft: () => ({ mutateAsync: vi.fn(), isPending: false }),
  draftChangeCount: () => 0,
}));

vi.mock("@/lib/queries/useMappingRules", () => ({ useMappingRules: () => ({ data: [] }) }));
vi.mock("@/lib/queries/useRoles", () => ({ useGlobalRoleCatalog: () => ({ data: [] }) }));
vi.mock("@/components/bundles/BundleVersions", () => ({ BundleVersions: () => null }));
vi.mock("@/components/names", () => ({ RoleRef: () => null, UserName: () => null }));

function bundle(overrides: Partial<BundleRow> = {}): BundleRow {
  return { id: "b1", name: "Lab Tech", holder_count: 11, latest_version: 2, ...overrides };
}

beforeEach(() => {
  state.bundles = [bundle()];
  state.setWelcome = vi.fn().mockResolvedValue({});
});

afterEach(cleanup);

// "Given to everyone who joins" is one bundle at a time. When this bundle
// already is the one there is nothing to press, and the card says so in words
// rather than rendering a disabled button with a reason.
describe("given to everyone who joins", () => {
  it("offers no button when this bundle already is the default, and says so", () => {
    state.bundles = [bundle({ is_welcome: true })];
    render(<BundlePage bundleId="b1" />);

    expect(screen.getByText("Lab Tech is given to new members now. One bundle at a time.")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /^Give Lab Tech/ })).toBeNull();
  });

  it("offers 'Give X instead' when another is the default", async () => {
    state.bundles = [bundle(), bundle({ id: "b2", name: "Mill Certified", is_welcome: true })];
    render(<BundlePage bundleId="b1" />);

    expect(screen.getByText("Mill Certified is given to new members now. One bundle at a time.")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Give Lab Tech instead" }));

    await waitFor(() => expect(state.setWelcome).toHaveBeenCalledWith("b1"));
    expect(await screen.findByText(/^Lab Tech is now given to everyone who joins/)).toBeInTheDocument();
  });

  it("offers 'Give X to everyone who joins' when no bundle is the default", () => {
    render(<BundlePage bundleId="b1" />);

    expect(screen.getByText(/No bundle is given to new members now/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Give Lab Tech to everyone who joins" })).toBeEnabled();
  });
});
