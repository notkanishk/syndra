// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { BundlePage } from "@/components/bundles/BundlePage";

const state = vi.hoisted(() => ({
  draft: undefined as unknown,
  versions: [] as unknown[],
  discard: vi.fn(),
}));

vi.mock("next/navigation", () => ({ useRouter: () => ({ push: vi.fn() }) }));

vi.mock("@/lib/queries/useBundles", () => ({
  useBundleRolesByBundle: () => ({ byId: {}, allLoaded: true }),
  useSetBundleInclude: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useRemoveBundleInclude: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useBundles: () => ({
    data: [{ id: "b1", name: "Lab Tech", holder_count: 3 }],
    isLoading: false,
    error: null,
    refetch: () => {},
  }),
  useBundleRoles: () => ({ data: [], isLoading: false, error: null, refetch: () => {} }),
  useBundleImpact: () => ({ data: null, isLoading: false }),
  useAddBundleRole: () => ({ mutate: vi.fn(), isPending: false }),
  useUpdateBundle: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useDeleteBundle: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useRemoveBundleRole: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useSetWelcomeBundle: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));

vi.mock("@/lib/queries/useBundleVersions", () => ({
  useVersionsByBundle: () => ({}),
  useBundleIncluders: () => ({ data: [] }),
  useBundleHolders: () => ({ data: [] }),
  useBundleDraft: () => ({ data: state.draft }),
  useBundleVersions: () => ({ data: state.versions, isLoading: false, error: null, refetch: () => {} }),
  useDiscardBundleDraft: () => ({ mutateAsync: state.discard, isPending: false }),
  draftChangeCount: (d: { added: unknown[]; removed: unknown[] } | undefined) =>
    (d?.added?.length ?? 0) + (d?.removed?.length ?? 0),
}));

vi.mock("@/lib/queries/useMappingRules", () => ({ useMappingRules: () => ({ data: [] }) }));
vi.mock("@/lib/queries/useRoles", () => ({ useGlobalRoleCatalog: () => ({ data: [] }) }));
vi.mock("@/components/bundles/BundleVersions", () => ({
  BundlePeople: () => <div>who has it</div>,
  BundleVersions: () => <div>every version</div>,
}));
vi.mock("@/components/names", () => ({
  RoleRef: ({ roleKey }: { roleKey: string }) => <span>{roleKey}</span>,
  UserName: ({ id }: { id: string }) => <span>{id}</span>,
}));

const role = (key: string) => ({ bundle_id: "b1", zitadel_project_id: "p1", zitadel_role_key: key });

beforeEach(() => {
  state.discard = vi.fn().mockResolvedValue({});
  state.draft = {
    bundle_id: "b1",
    latest_version: 2,
    next_version: 3,
    added: [role("laser"), role("cnc")],
    removed: [role("mill")],
    holder_count: 3,
  };
  state.versions = [
    { id: "v1", bundle_id: "b1", version: 1, note: "", published_by: "", published_at: "2026-01-02T00:00:00Z", holder_count: 0 },
    { id: "v2", bundle_id: "b1", version: 2, note: "Added laser", published_by: "u1", published_at: "2026-02-03T00:00:00Z", holder_count: 3 },
  ];
});

afterEach(cleanup);

describe("a bundle's page", () => {
  it("titles the unpublished panel with the change count and names the next version", () => {
    render(<BundlePage bundleId="b1" />);

    expect(screen.getByRole("heading", { name: "3 changes, not published yet" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Publish v3" })).toBeInTheDocument();
  });

  it("says 'change' for one", () => {
    state.draft = { ...(state.draft as object), added: [role("laser")], removed: [] };
    render(<BundlePage bundleId="b1" />);

    expect(screen.getByRole("heading", { name: "1 change, not published yet" })).toBeInTheDocument();
  });

  it("shows no unpublished panel when the working copy matches the latest version", () => {
    state.draft = { ...(state.draft as object), added: [], removed: [] };
    render(<BundlePage bundleId="b1" />);

    expect(screen.queryByRole("button", { name: "Discard changes" })).toBeNull();
    expect(screen.queryByText(/not published yet/)).toBeNull();
  });

  it("discards the working copy from the panel", async () => {
    render(<BundlePage bundleId="b1" />);
    fireEvent.click(screen.getByRole("button", { name: "Discard changes" }));

    await waitFor(() => expect(state.discard).toHaveBeenCalledTimes(1));
  });

  it("says so when discarding fails, and keeps the button", async () => {
    state.discard = vi.fn().mockRejectedValue(new Error("the server is unreachable"));
    render(<BundlePage bundleId="b1" />);
    fireEvent.click(screen.getByRole("button", { name: "Discard changes" }));

    await waitFor(() => expect(document.body.textContent).toMatch(/unreachable/i));
    expect(screen.getByRole("button", { name: "Discard changes" })).toBeInTheDocument();
  });

  it("switches to History and shows every version", () => {
    render(<BundlePage bundleId="b1" />);
    fireEvent.click(screen.getByRole("button", { name: /History/ }));
    expect(screen.getByText("every version")).toBeInTheDocument();
    expect(screen.queryByText("who has it")).not.toBeInTheDocument();
  });

  it("switches to People and shows who has it", () => {
    render(<BundlePage bundleId="b1" />);
    fireEvent.click(screen.getByRole("button", { name: /People/ }));

    expect(screen.getByText("who has it")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /People/ })).toHaveAttribute("aria-current", "page");
  });
});
