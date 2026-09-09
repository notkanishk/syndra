// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import BundlesPage from "@/app/bundles/page";
import type { BundleRow } from "@/lib/queries/useBundles";

const state = vi.hoisted(() => ({
  bundles: [] as BundleRow[],
}));

vi.mock("@/lib/queries/useBundles", () => ({
  useBundles: () => ({ data: state.bundles, isLoading: false, error: null, refetch: () => {} }),
  useBundleRoles: () => ({ data: [], isLoading: false, error: null, refetch: () => {} }),
  useBundleImpact: () => ({ data: { role_count: 0, users: [] }, isLoading: false }),
  useCreateBundle: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useUpdateBundle: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useDeleteBundle: () => ({ mutateAsync: vi.fn(), isPending: false }),
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
});

afterEach(cleanup);

// Button.tsx's own rule: a disabled control states its reason in visible
// copy. The welcome toggle rendered "On" and disabled with nothing saying
// why — this is the reason showing up as required.
describe("default-for-new-members toggle", () => {
  it("says why the toggle is disabled once this bundle already is the default", () => {
    state.bundles = [bundle({ is_welcome: true })];
    render(<BundlesPage />);

    const toggle = screen.getByRole("button", { name: "On" });
    expect(toggle).toBeDisabled();
    expect(screen.getByText(/already is — there's nothing to set/i)).toBeInTheDocument();
  });

  it("stays an enabled, reason-free action when this bundle is not the default yet", () => {
    state.bundles = [bundle({ is_welcome: false })];
    render(<BundlesPage />);

    const toggle = screen.getByRole("button", { name: "Set to Lab Tech" });
    expect(toggle).not.toBeDisabled();
    expect(screen.queryByText(/already is — there's nothing to set/i)).not.toBeInTheDocument();
  });
});
