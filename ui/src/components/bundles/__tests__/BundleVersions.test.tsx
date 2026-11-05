// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { BundleVersions, ThroughOtherBundles } from "@/components/bundles/BundleVersions";

vi.mock("@/lib/queries/useBundles", () => ({
  useSetBundleInclude: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));

vi.mock("@/lib/queries/useBundleVersions", () => ({
  useBundleVersions: () => ({
    data: [
      { id: "v2", version: 2, holder_count: 0, published_at: "2026-09-01T00:00:00Z", published_by: "u0", note: "", roles: [] },
      { id: "v1", version: 1, holder_count: 3, published_at: "2026-08-01T00:00:00Z", published_by: "u0", note: "", roles: [] },
    ],
    isLoading: false,
    error: null,
    refetch: vi.fn(),
    dataUpdatedAt: Date.now(),
    isError: false,
  }),
  useBundleHolders: () => ({
    data: [
      { bundle_id: "b1", user_id: "u1", version_id: "v1", version: 1 },
      { bundle_id: "b1", user_id: "u2", version_id: "v1", version: 1 },
      { bundle_id: "b1", user_id: "u3", version_id: "v1", version: 1 },
    ],
  }),
  useRehearseMoveHolders: () => ({}),
  useApplyMoveHolders: () => ({}),
}));

vi.mock("@/components/names", () => ({
  RoleRef: ({ roleKey }: { roleKey: string }) => <span>{roleKey}</span>,
  UserAvatar: () => null,
  UserName: ({ id }: { id: string }) => <a href={`/users/${id}`}>{id === "u0" ? "Kanishk Singh" : id}</a>,
}));

afterEach(cleanup);

describe("a bundle's versions", () => {
  it("says who is behind once in the note and once as the button, not on every older version", () => {
    render(<BundleVersions bundleId="b1" name="Lab Tech" />);

    expect(screen.getByText("3 people are on an older version")).toBeInTheDocument();
    expect(screen.getAllByRole("button", { name: /^Move/ })).toHaveLength(1);
    expect(screen.getByRole("button", { name: "Move everyone to v2" })).toBeInTheDocument();
    expect(screen.queryByText(/still on this version/)).toBeNull();
  });

  it("links who published a version to their page, and carries a read line", () => {
    render(<BundleVersions bundleId="b1" name="Lab Tech" />);

    expect(screen.getAllByRole("link", { name: "Kanishk Singh" })[0]).toHaveAttribute("href", "/users/u0");
    expect(screen.getByText(/Lab Tech's versions/)).toBeInTheDocument();
  });
});

describe("through other bundles", () => {
  it("lists the bundles that include it A to Z, with no dashed edges", () => {
    const includer = (bundle_id: string, name: string) => ({
      bundle_id,
      name,
      included_version: 2,
      draft_version: 2,
      people: 5,
    });
    const { container } = render(
      <ThroughOtherBundles
        name="Safety basics"
        bundleId="sb"
        latest={2}
        includers={[includer("n", "New members"), includer("a", "Arrivals")]}
        bundles={[]}
      />,
    );

    const names = screen.getAllByRole("link").map((l) => l.textContent);
    expect(names).toEqual(["Arrivals", "New members"]);
    expect(container.innerHTML).not.toMatch(/dashed/);
  });
});
