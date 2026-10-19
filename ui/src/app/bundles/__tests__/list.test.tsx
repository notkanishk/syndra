// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import BundlesPage from "@/app/bundles/page";
import type { BundleRow } from "@/lib/queries/useBundles";

const state = vi.hoisted(() => ({ bundles: [] as BundleRow[] }));

vi.mock("@/lib/queries/useBundles", () => ({
  useBundleRolesByBundle: () => ({ byId: {}, allLoaded: true }),
  useSetBundleInclude: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useRemoveBundleInclude: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useBundles: () => ({ data: state.bundles, isLoading: false, error: null, refetch: () => {} }),
  useCreateBundle: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));

function bundle(overrides: Partial<BundleRow> = {}): BundleRow {
  return { id: "b1", name: "Lab Tech", description: "Trained on the mill", holder_count: 11, roles: ["a", "b", "c"], ...overrides };
}

beforeEach(() => {
  state.bundles = [bundle()];
});

afterEach(cleanup);

// The list is a row per bundle: the name is the way in, what needs attention
// is written under it, and Roles and People are plain numbers.
describe("the bundles list", () => {
  it("links each name to that bundle's page", () => {
    state.bundles = [bundle(), bundle({ id: "b 2", name: "Mill Certified" })];
    render(<BundlesPage />);

    expect(screen.getByRole("link", { name: "Lab Tech" })).toHaveAttribute("href", "/bundles/b1");
    // The id is encoded, so an odd one cannot break the path.
    expect(screen.getByRole("link", { name: "Mill Certified" })).toHaveAttribute("href", "/bundles/b%202");
  });

  it("writes what needs attention under the name", () => {
    state.bundles = [bundle({ unpublished_changes: 2, stale_holders: 4, is_welcome: true })];
    render(<BundlesPage />);

    expect(screen.getByText("Unpublished changes")).toBeInTheDocument();
    expect(screen.getByText("4 people on an older version")).toBeInTheDocument();
    expect(screen.getByText("Given to everyone who joins")).toBeInTheDocument();
  });

  it("says 'person' for one stale holder, and shows no notes for a quiet bundle", () => {
    state.bundles = [bundle({ stale_holders: 1 })];
    const { rerender } = render(<BundlesPage />);
    expect(screen.getByText("1 person on an older version")).toBeInTheDocument();

    state.bundles = [bundle()];
    rerender(<BundlesPage />);
    expect(screen.queryByText(/older version|Unpublished|Given to everyone/)).toBeNull();
  });

  it("ends the list with a fold of retired bundles, closed", () => {
    render(<BundlesPage />);

    expect(screen.getByRole("button", { name: /Retired bundles/ })).toHaveAttribute("aria-expanded", "false");
  });

  it("shows the Roles and People counts", () => {
    render(<BundlesPage />);

    const row = screen.getByRole("link", { name: "Lab Tech" }).closest(".row-divider") as HTMLElement;
    expect(row).toHaveTextContent("3");
    expect(row).toHaveTextContent("11");
    expect(screen.getByText("Roles")).toBeInTheDocument();
    expect(screen.getByText("People")).toBeInTheDocument();
  });

  it("shows a dash for roles when the list does not carry them, and 0 people", () => {
    state.bundles = [bundle({ roles: undefined, holder_count: undefined })];
    render(<BundlesPage />);

    const row = screen.getByRole("link", { name: "Lab Tech" }).closest(".row-divider") as HTMLElement;
    expect(row).toHaveTextContent("—");
    expect(row).toHaveTextContent("0");
  });
});
