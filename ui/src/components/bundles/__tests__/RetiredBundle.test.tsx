// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { RetiredBundlePage, RetiredBundlesFold } from "@/components/bundles/RetiredBundle";

/**
 * Retire keeps its history (owner, 19 Oct 2026). A retired bundle's page still opens, says it was
 * retired and by whom, offers nothing to press, and reads what it gave and how it changed.
 */

const state = vi.hoisted(() => ({
  one: { data: undefined, isLoading: false, error: null } as {
    data?: unknown;
    isLoading: boolean;
    error: unknown;
  },
  retired: [] as unknown[],
}));

vi.mock("@/lib/queries/useBundles", () => ({
  useBundle: () => ({ ...state.one, refetch: vi.fn() }),
  useBundleRoles: () => ({
    data: [{ bundle_id: "b1", zitadel_project_id: "p1", zitadel_role_key: "door" }],
    isLoading: false,
    error: null,
    refetch: vi.fn(),
  }),
  useRetiredBundles: () => ({ data: state.retired, isLoading: false, error: null, refetch: vi.fn() }),
}));

vi.mock("@/lib/queries/useBundleVersions", () => ({
  useBundleVersions: () => ({
    data: [
      { id: "v2", version: 2, holder_count: 0, published_at: "2026-09-01T00:00:00Z", published_by: "u0", note: "Added the door", roles: [] },
      // Somebody still "on" an older version would be offered a move on a live bundle.
      { id: "v1", version: 1, holder_count: 3, published_at: "2026-08-01T00:00:00Z", published_by: "system", note: "", roles: [] },
    ],
    isLoading: false,
    error: null,
    refetch: vi.fn(),
  }),
  useBundleHolders: () => ({
    data: [{ bundle_id: "b1", user_id: "u1", version_id: "v1", version: 1 }],
    isLoading: false,
    error: null,
    refetch: vi.fn(),
  }),
  useRehearseMoveHolders: () => ({}),
  useApplyMoveHolders: () => ({}),
}));

vi.mock("@/components/names", () => ({
  RoleRef: ({ roleKey }: { roleKey: string }) => <span>{roleKey}</span>,
  UserAvatar: () => null,
  UserName: ({ id }: { id: string }) => <>{id === "u0" ? "Kanishk Singh" : id}</>,
}));

beforeEach(() => {
  state.one = {
    data: { id: "b1", name: "Old Door Access", retired_at: "2020-03-04T09:00:00Z", retired_by: "u0" },
    isLoading: false,
    error: null,
  };
  state.retired = [];
});

afterEach(cleanup);

describe("a retired bundle's page", () => {
  it("says it was retired, when and by whom, with nothing to press", () => {
    render(<RetiredBundlePage bundleId="b1" />);

    expect(screen.getByRole("heading", { name: "Old Door Access" })).toBeInTheDocument();
    expect(screen.getByText(/^Retired 4 Mar 2020/)).toBeInTheDocument();
    expect(screen.getByText("by Kanishk Singh")).toBeInTheDocument();
    for (const action of ["Rename", "Add people", "Retire bundle", "Publish", "Add roles"]) {
      expect(screen.queryByRole("button", { name: new RegExp(action) })).toBeNull();
    }
    expect(screen.queryByRole("link", { name: "Add people" })).toBeNull();
  });

  it("opens on its history, read-only: no move, and nobody is on an older version", () => {
    render(<RetiredBundlePage bundleId="b1" />);

    expect(screen.getByText("v2")).toBeInTheDocument();
    expect(screen.getByText("Added the door")).toBeInTheDocument();
    expect(screen.getByText("Last")).toBeInTheDocument();
    expect(screen.queryByText("Current")).toBeNull();
    expect(screen.queryByRole("button", { name: /Move/ })).toBeNull();
    expect(screen.queryByText(/still on this version/)).toBeNull();
    expect(screen.getByText(/lost what it gave when it was retired/)).toBeInTheDocument();
  });

  it("reads what it gave in its last version, and says nobody has it", () => {
    render(<RetiredBundlePage bundleId="b1" />);

    fireEvent.click(screen.getByRole("button", { name: /What it gave/ }));
    expect(screen.getByText("door")).toBeInTheDocument();
    expect(screen.getByText("In v2, its last version")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /People/ }));
    expect(screen.getByText("Nobody has it.")).toBeInTheDocument();
  });

  it("is the same 'no bundle here' as ever for an id that never existed", () => {
    state.one = { data: undefined, isLoading: false, error: Object.assign(new Error("nope"), { status: 404 }) };
    render(<RetiredBundlePage bundleId="nope" />);

    expect(screen.getByText("There's no bundle here.")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "All bundles" })).toHaveAttribute("href", "/bundles");
  });

  it("does not claim a live bundle is retired", () => {
    state.one = { data: { id: "b1", name: "Lab Tech" }, isLoading: false, error: null };
    render(<RetiredBundlePage bundleId="b1" />);

    expect(screen.getByText("There's no bundle here.")).toBeInTheDocument();
    expect(screen.queryByText(/^Retired/)).toBeNull();
  });
});

describe("the retired bundles fold", () => {
  it("is closed until asked, then lists them as links with when", () => {
    state.retired = [
      { id: "b 9", name: "Old Door Access", retired_at: "2020-03-04T09:00:00Z" },
      { id: "b8", name: "Spring Intake", retired_at: "2020-01-02T09:00:00Z" },
    ];
    render(<RetiredBundlesFold />);
    expect(screen.queryByText("Old Door Access")).toBeNull();

    fireEvent.click(screen.getByRole("button", { name: /Retired bundles/ }));
    expect(screen.getByRole("link", { name: /Old Door Access/ })).toHaveAttribute("href", "/bundles/b%209");
    expect(screen.getByText("Retired 4 Mar 2020")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Spring Intake/ })).toBeInTheDocument();
  });

  it("is not there at all when nothing has been retired", () => {
    state.retired = [];
    const { container } = render(<RetiredBundlesFold />);
    expect(container).toBeEmptyDOMElement();
  });
});
