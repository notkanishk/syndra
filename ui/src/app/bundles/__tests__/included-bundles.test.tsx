// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { BundlePage } from "@/components/bundles/BundlePage";
import { ApiError } from "@/lib/api-client";

/**
 * Included bundles (B8, route-bundles-v2 2e): the parent's page offers the
 * child's newer version and changes only its working copy; the child's page
 * counts people directly and through bundles; retiring an included bundle is
 * refused, and says by whom.
 */
const state = vi.hoisted(() => ({
  bundles: [] as unknown[],
  draft: undefined as unknown,
  versions: [] as unknown[],
  childVersions: {} as Record<string, unknown[]>,
  includers: [] as unknown[],
  holders: [] as unknown[],
  setInclude: vi.fn(),
  removeInclude: vi.fn(),
  del: vi.fn(),
}));

vi.mock("next/navigation", () => ({ useRouter: () => ({ push: vi.fn() }) }));

vi.mock("@/lib/queries/useBundles", () => ({
  useBundles: () => ({ data: state.bundles, isLoading: false, error: null, refetch: () => {} }),
  useBundleRoles: () => ({ data: [], isLoading: false, error: null, refetch: () => {} }),
  useBundleRolesByBundle: () => ({ byId: {}, allLoaded: true }),
  useBundleImpact: () => ({ data: null, isLoading: false }),
  useAddBundleRole: () => ({ mutate: vi.fn(), isPending: false }),
  useUpdateBundle: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useDeleteBundle: () => ({ mutateAsync: state.del, isPending: false }),
  useRemoveBundleRole: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useSetWelcomeBundle: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useSetBundleInclude: (parent: string) => ({
    mutateAsync: (input: unknown) => state.setInclude(parent, input),
    isPending: false,
  }),
  useRemoveBundleInclude: (parent: string) => ({
    mutateAsync: (child: string) => state.removeInclude(parent, child),
    isPending: false,
  }),
}));

vi.mock("@/lib/queries/useBundleVersions", () => ({
  useBundleDraft: () => ({ data: state.draft }),
  useBundleVersions: () => ({ data: state.versions, isLoading: false, error: null, refetch: () => {} }),
  useVersionsByBundle: (ids: string[]) => Object.fromEntries(ids.map((id) => [id, state.childVersions[id]])),
  useBundleIncluders: () => ({ data: state.includers }),
  useBundleHolders: () => ({ data: state.holders }),
  useDiscardBundleDraft: () => ({ mutateAsync: vi.fn(), isPending: false }),
  draftChangeCount: (d: { added: unknown[]; removed: unknown[]; include_changes: unknown[] } | undefined) =>
    (d?.added?.length ?? 0) + (d?.removed?.length ?? 0) + (d?.include_changes?.length ?? 0),
}));

vi.mock("@/lib/queries/useMappingRules", () => ({ useMappingRules: () => ({ data: [] }) }));
vi.mock("@/lib/queries/useRoles", () => ({
  useGlobalRoleCatalog: () => ({
    data: [
      { project_id: "main", project_name: "Main lab", role_key: "door", display_name: "Front door", description: "Badge entry" },
      { project_id: "main", project_name: "Main lab", role_key: "lockers", display_name: "Lockers", description: "" },
      { project_id: "nas", project_name: "Network storage", role_key: "member", display_name: "Member", description: "" },
    ],
  }),
}));
vi.mock("@/components/bundles/BundleVersions", async (original) => ({
  ...(await original<typeof import("@/components/bundles/BundleVersions")>()),
  BundlePeople: () => <div>who has it</div>,
  BundleVersions: () => null,
}));
vi.mock("@/components/names", () => ({
  RoleRef: ({ roleKey }: { roleKey: string }) => <span>{roleKey}</span>,
  UserName: ({ id }: { id: string }) => <span>{id}</span>,
}));

const role = (project: string, key: string) => ({ bundle_id: "sb", zitadel_project_id: project, zitadel_role_key: key });
const version = (n: number, roles: unknown[] = []) => ({
  id: `v${n}`,
  bundle_id: "x",
  version: n,
  note: "",
  published_by: "u1",
  published_at: "2026-10-09T00:00:00Z",
  holder_count: 0,
  roles,
});
const emptyDraft = { latest_version: 1, next_version: 2, added: [], removed: [], holder_count: 0, includes: [], include_changes: [] };

beforeEach(() => {
  state.setInclude = vi.fn().mockResolvedValue({});
  state.removeInclude = vi.fn().mockResolvedValue({});
  state.del = vi.fn();
  state.includers = [];
  state.holders = [];
});

afterEach(cleanup);

describe("a bundle that includes another", () => {
  beforeEach(() => {
    state.bundles = [{ id: "nm", name: "New members", holder_count: 40, has_includes: true }];
    state.versions = [version(1)];
    state.draft = {
      ...emptyDraft,
      bundle_id: "nm",
      includes: [
        { child_bundle_id: "sb", child_name: "Safety basics", child_version_id: "v3", child_version: 3, child_latest_version: 4 },
      ],
    };
    state.childVersions = {
      sb: [
        version(4, [role("main", "door"), role("main", "lockers"), role("nas", "member")]),
        version(3, [role("main", "door"), role("main", "lockers")]),
      ],
    };
  });

  it("draws it as an Included bundle card, each role saying where it comes from", () => {
    render(<BundlePage bundleId="nm" />);
    expect(screen.getByText("Included bundle")).toBeInTheDocument();
    expect(screen.getByText("v3 · 2 roles")).toBeInTheDocument();
    expect(screen.getAllByText("Comes with Safety basics")).toHaveLength(2);
    expect(screen.getByRole("link", { name: /Change what.s in Safety basics/ })).toHaveAttribute("href", "/bundles/sb");
  });

  it("offers the newer version, and adding it writes only the working copy", async () => {
    render(<BundlePage bundleId="nm" />);
    expect(screen.getByText("Safety basics v4 is out.")).toBeInTheDocument();
    expect(screen.getByText(/It adds Network storage \/ Member\. Nothing changes for New members until you add it and publish\./)).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Add to next version" }));
    await waitFor(() => expect(state.setInclude).toHaveBeenCalledWith("nm", { child: "sb", version: 4 }));
  });

  it("offers nothing when it already includes the latest", () => {
    (state.draft as { includes: Array<{ child_latest_version: number }> }).includes[0].child_latest_version = 3;
    render(<BundlePage bundleId="nm" />);
    expect(screen.queryByText(/is out\./)).toBeNull();
    expect(screen.queryByRole("button", { name: "Add to next version" })).toBeNull();
  });

  it("removes it from the working copy", async () => {
    render(<BundlePage bundleId="nm" />);
    fireEvent.click(screen.getByRole("button", { name: "Remove from New members" }));
    await waitFor(() => expect(state.removeInclude).toHaveBeenCalledWith("nm", "sb"));
  });

  it("lists include changes in the unpublished panel", () => {
    state.draft = {
      ...(state.draft as object),
      include_changes: [
        { child_bundle_id: "sb", child_name: "Safety basics", from_version: 3, to_version: 4 },
        { child_bundle_id: "ws", child_name: "Wood shop", from_version: 0, to_version: 2 },
        { child_bundle_id: "st", child_name: "Studio", from_version: 1, to_version: 0 },
      ],
    };
    render(<BundlePage bundleId="nm" />);
    expect(screen.getByRole("heading", { name: "3 changes, not published yet" })).toBeInTheDocument();
    expect(screen.getByText("Safety basics v3 → v4")).toBeInTheDocument();
    expect(screen.getByText("Wood shop v2").previousSibling).toHaveTextContent("+");
    expect(screen.getByText("Studio").previousSibling).toHaveAttribute("aria-label", "Drops");
  });
});

describe("a bundle that others include", () => {
  beforeEach(() => {
    state.bundles = [
      { id: "sb", name: "Safety basics", holder_count: 8, included_in: ["New members", "Staff"] },
      { id: "nm", name: "New members", description: "Everyone in their first term" },
      { id: "st", name: "Staff", description: "Makerspace staff" },
    ];
    state.versions = [version(4), version(3)];
    state.draft = { ...emptyDraft, bundle_id: "sb", latest_version: 4, next_version: 5 };
    state.childVersions = {};
    state.includers = [
      { bundle_id: "nm", name: "New members", included_version: 3, people: 40 },
      { bundle_id: "st", name: "Staff", included_version: 4, people: 5 },
    ];
    state.holders = [
      ...Array.from({ length: 5 }, (_, i) => ({ user_id: `a${i}`, version: 4 })),
      ...Array.from({ length: 3 }, (_, i) => ({ user_id: `b${i}`, version: 3 })),
    ];
  });

  it("counts people directly, through bundles, and behind the latest, each linking to its group", () => {
    render(<BundlePage bundleId="sb" />);
    const direct = screen.getByRole("link", { name: "Directly" });
    expect(direct).toHaveAttribute("href", "#on-v4");
    expect(direct).toHaveTextContent("8");
    expect(direct).toHaveTextContent("5 on v4 · 3 on v3");

    const through = screen.getByRole("link", { name: "Through bundles" });
    expect(through).toHaveAttribute("href", "#through-bundles");
    expect(through).toHaveTextContent("45");
    expect(through).toHaveTextContent("In New members and Staff");

    const behind = screen.getByRole("link", { name: "Behind v4" });
    expect(behind).toHaveAttribute("href", "#on-v3");
    expect(behind).toHaveTextContent("43");
    expect(behind).toHaveTextContent("3 directly · 40 through New members");
  });

  it("opens the People tab and glides to the group when a count is pressed", async () => {
    const scroll = vi.fn();
    Element.prototype.scrollIntoView = scroll; // jsdom has none
    render(<BundlePage bundleId="sb" />);
    fireEvent.click(screen.getByRole("link", { name: "Through bundles" }));
    expect(screen.getByText("who has it")).toBeInTheDocument();
    await waitFor(() => expect(scroll).toHaveBeenCalled());
  });

  it("shows the bundles that include it on the People tab, and updates one's working copy", async () => {
    render(<BundlePage bundleId="sb" />);
    fireEvent.click(screen.getByRole("button", { name: /^People/ }));

    const card = screen.getByRole("heading", { name: "Through other bundles" }).closest("section") as HTMLElement;
    expect(within(card).getByText("45 people in 2 bundles")).toBeInTheDocument();
    expect(within(card).getByText("Includes v3, one behind")).toBeInTheDocument();
    expect(within(card).getByText("Includes v4")).toBeInTheDocument();
    expect(within(card).getByText("Bundle · Everyone in their first term")).toBeInTheDocument();
    expect(within(card).getAllByRole("button", { name: /Update to/ })).toHaveLength(1);

    fireEvent.click(within(card).getByRole("button", { name: "Update to v4" }));
    await waitFor(() => expect(state.setInclude).toHaveBeenCalledWith("nm", { child: "sb", version: 4 }));
  });

  it("says when the update already waits in that bundle's next version", () => {
    state.includers = [{ bundle_id: "nm", name: "New members", included_version: 3, draft_version: 4, people: 40 }];
    render(<BundlePage bundleId="sb" />);
    fireEvent.click(screen.getByRole("button", { name: /^People/ }));
    expect(screen.getByText("v4 in its next version")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Update to v4" })).toBeNull();
  });

  it("refuses to retire while others include it, and says which", () => {
    render(<BundlePage bundleId="sb" />);
    expect(screen.getByText("Refused while New members and Staff include it.")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Retire bundle" })).toBeDisabled();
  });
});

describe("retiring, refused by the server", () => {
  it("shows the refusal in the retire dialog", async () => {
    state.bundles = [{ id: "sb", name: "Safety basics", holder_count: 0 }];
    state.versions = [version(1)];
    state.draft = { ...emptyDraft, bundle_id: "sb" };
    state.del = vi.fn().mockRejectedValue(
      new ApiError(409, {
        error: "BUNDLE_INCLUDED",
        message: "Refused while New members and Staff include it. Nothing was changed.",
      }),
    );
    render(<BundlePage bundleId="sb" />);
    fireEvent.click(screen.getByRole("button", { name: "Retire bundle" }));
    fireEvent.click(screen.getByRole("button", { name: "Retire and revoke" }));

    expect(await screen.findByText(/Refused while New members and Staff include it\. Nothing was changed\./)).toBeInTheDocument();
  });
});
