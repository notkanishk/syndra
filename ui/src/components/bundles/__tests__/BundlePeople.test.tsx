// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { BundlePeople } from "@/components/bundles/BundleVersions";

vi.mock("@/lib/queries/useBundleVersions", () => ({
  useBundleVersions: () => ({
    data: [
      { id: "v3", version: 3, holder_count: 2, published_at: "2026-10-01T00:00:00Z", published_by: "u0", note: "", bundle_id: "b" },
      { id: "v2", version: 2, holder_count: 1, published_at: "2026-09-01T00:00:00Z", published_by: "u0", note: "", bundle_id: "b" },
    ],
    isLoading: false,
    error: null,
  }),
  useBundleHolders: () => ({
    data: [
      { bundle_id: "b", user_id: "u1", version_id: "v3", version: 3, assigned_at: "2026-09-28T00:00:00Z", source: "direct", assigned_by: "u0" },
      { bundle_id: "b", user_id: "u2", version_id: "v3", version: 3, assigned_at: "2026-09-28T00:00:00Z", source: "moved", moved_from_version: 2 },
      { bundle_id: "b", user_id: "u3", version_id: "v2", version: 2, assigned_at: "2026-09-03T00:00:00Z", source: "welcome" },
    ],
    isLoading: false,
    error: null,
    isError: false,
    dataUpdatedAt: Date.now(),
    refetch: () => {},
  }),
  useRehearseMoveHolders: () => ({}),
  useApplyMoveHolders: () => ({}),
}));
vi.mock("@/components/names", () => ({
  RoleRef: () => null,
  UserAvatar: () => null,
  UserName: ({ id }: { id: string }) => <>{id === "u0" ? "Kanishk Singh" : id}</>,
}));

// Bundles 6 + B7: grouped by version, and each person says how they got it.
describe("who has a bundle", () => {
  it("groups by version and offers the move to anyone behind", () => {
    render(<BundlePeople bundleId="b" name="Lab Tech" />);
    expect(screen.getByRole("heading", { name: "On v3" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "On v2" })).toBeInTheDocument();
    expect(screen.getByText(/1 person · v3 is the latest/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Move them to v3" })).toBeInTheDocument();
  });

  // The board's People tab opens on a read line (route-bundles-v2, bundle-people).
  it("says when who has the bundle was read", () => {
    render(<BundlePeople bundleId="b" name="Lab Tech" />);
    expect(screen.getByText("Who has Lab Tech was read just now")).toBeInTheDocument();
  });

  it("says how each person got it", () => {
    render(<BundlePeople bundleId="b" name="Lab Tech" />);
    expect(screen.getByText(/Given by/)).toHaveTextContent("Given by Kanishk Singh");
    expect(screen.getByText("Moved from v2")).toBeInTheDocument();
    expect(screen.getByText("Given when they joined")).toBeInTheDocument();
  });
});
