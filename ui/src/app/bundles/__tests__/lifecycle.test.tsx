// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { BundlePage } from "@/components/bundles/BundlePage";
import type { BundleRow } from "@/lib/queries/useBundles";

const state = vi.hoisted(() => ({
  bundles: [] as BundleRow[],
  update: vi.fn(),
  remove: vi.fn(),
  push: vi.fn(),
}));

vi.mock("next/navigation", () => ({ useRouter: () => ({ push: state.push }) }));

vi.mock("@/lib/queries/useBundles", () => ({
  useBundleRolesByBundle: () => ({ byId: {}, allLoaded: true }),
  useSetBundleInclude: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useRemoveBundleInclude: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useBundles: () => ({ data: state.bundles, isLoading: false, error: null, refetch: () => {} }),
  useBundleRoles: () => ({ data: [], isLoading: false, error: null, refetch: () => {} }),
  useBundleImpact: () => ({ data: { role_count: 0, users: [] }, isLoading: false }),
  useAddBundleRole: () => ({ mutate: vi.fn(), isPending: false }),
  useCreateBundle: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useUpdateBundle: () => ({ mutateAsync: state.update, isPending: false }),
  useDeleteBundle: () => ({ mutateAsync: state.remove, isPending: false }),
  useRemoveBundleRole: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useSetWelcomeBundle: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));

vi.mock("@/lib/queries/useBundleVersions", () => ({
  useVersionsByBundle: () => ({}),
  useBundleIncluders: () => ({ data: [] }),
  useBundleHolders: () => ({ data: [] }),
  useBundleDraft: () => ({ data: { latest_version: 2, next_version: 3, added: [], removed: [] } }),
  useBundleVersions: () => ({ data: [], isLoading: false, refetch: vi.fn() }),
  useDiscardBundleDraft: () => ({ mutateAsync: vi.fn(), isPending: false }),
  draftChangeCount: () => 0,
}));

vi.mock("@/lib/queries/useMappingRules", () => ({ useMappingRules: () => ({ data: [] }) }));

vi.mock("@/components/bundles/BundleVersions", () => ({ BundleVersions: () => null, ThroughOtherBundles: () => null }));
vi.mock("@/components/names", () => ({
  RoleRef: () => null,
  UserName: () => null,
}));
vi.mock("@/lib/queries/useRoles", () => ({ useGlobalRoleCatalog: () => ({ data: [] }) }));


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
  state.push = vi.fn();
  state.bundles = [bundle()];
  state.update = vi.fn().mockResolvedValue({ message: "ok" });
  state.remove = vi
    .fn()
    .mockResolvedValue({ message: "ok", was_welcome: false, cascade: { enqueued: 4, mode: "auto" } });
});

afterEach(cleanup);

describe("retiring a bundle", () => {
  // Retiring a bundle strips access from everybody holding it. The number is the whole reason
  // to hesitate, so it has to be on screen before the click, not in a toast after it.
  it("names the holders before the confirming click", () => {
    render(<BundlePage bundleId="b1" />);
    fireEvent.click(screen.getByRole("button", { name: "Retire bundle" }));

    expect(document.body.textContent).toMatch(/11 people hold it right now/);
    expect(state.remove).not.toHaveBeenCalled();
  });

  // Retiring the welcome bundle silently stops onboarding granting anything. That consequence
  // has no other place to be said — the flag lives on the row that is about to go.
  it("warns that onboarding stops when the welcome bundle is the one going", () => {
    state.bundles = [bundle({ is_welcome: true })];
    render(<BundlePage bundleId="b1" />);
    fireEvent.click(screen.getByRole("button", { name: "Retire bundle" }));

    expect(document.body.textContent).toMatch(/default for new members/i);
    expect(document.body.textContent).toMatch(/stop handing anything out/i);
  });

  it("says nothing is taken away when nobody holds it", () => {
    state.bundles = [bundle({ holder_count: 0 })];
    render(<BundlePage bundleId="b1" />);
    fireEvent.click(screen.getByRole("button", { name: "Retire bundle" }));

    expect(document.body.textContent).toMatch(/takes nothing away from anybody/i);
  });

  it("does not autofocus the destructive button on open", () => {
    render(<BundlePage bundleId="b1" />);
    fireEvent.click(screen.getByRole("button", { name: "Retire bundle" }));

    expect(document.activeElement).not.toBe(
      screen.getByRole("button", { name: "Retire and revoke" }),
    );
  });

  it("deletes only on the confirming button", async () => {
    render(<BundlePage bundleId="b1" />);
    fireEvent.click(screen.getByRole("button", { name: "Retire bundle" }));
    fireEvent.click(screen.getByRole("button", { name: "Retire and revoke" }));

    await waitFor(() => expect(state.remove).toHaveBeenCalledWith("b1"));
  });
});

describe("bundle rename", () => {
  it("blocks a save that changes nothing", () => {
    render(<BundlePage bundleId="b1" />);
    fireEvent.click(screen.getByRole("button", { name: "Rename" }));

    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    expect(state.update).not.toHaveBeenCalled();
    expect(document.body.textContent).toMatch(/Nothing changed yet/);
  });

  it("sends the trimmed name and the description together", async () => {
    render(<BundlePage bundleId="b1" />);
    fireEvent.click(screen.getByRole("button", { name: "Rename" }));

    fireEvent.change(screen.getByLabelText("Name"), { target: { value: "  Lab Technician  " } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    await waitFor(() =>
      expect(state.update).toHaveBeenCalledWith({
        id: "b1",
        name: "Lab Technician",
        description: "Trained on the mill",
      }),
    );
  });

  /**
   * `setOutcome` used to fire alongside `onClose`, which unmounts the dialog
   * before "Now called Lab Technician" is ever painted. It now stays open on
   * success until Close is pressed, same fix as the create dialog.
   */
  it("shows the rename landed and stays open until Close is pressed", async () => {
    render(<BundlePage bundleId="b1" />);
    fireEvent.click(screen.getByRole("button", { name: "Rename" }));

    fireEvent.change(screen.getByLabelText("Name"), { target: { value: "Lab Technician" } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    await waitFor(() =>
      expect(document.body.textContent).toMatch(/Now called Lab Technician/),
    );
    expect(screen.queryByRole("button", { name: "Save" })).toBeNull();

    fireEvent.click(screen.getByRole("button", { name: "Close" }));
    expect(document.body.textContent).not.toMatch(/Now called Lab Technician/);
  });
});

/**
 * What happens AFTER the delete lands.
 *
 * The page remembers the row it last saw, so the list refetch dropping it does
 * not unmount the dialog that is still reporting. The sequence is the
 * contract: the report stays, the destructive action goes, and dismissing is
 * what moves the page on (to the list).
 */
describe("after a bundle is retired", () => {
  beforeEach(() => {
    state.bundles = [bundle(), bundle({ id: "b2", name: "Mill Certified", holder_count: 3 })];
  });

  async function retireTheSecondBundle() {
    render(<BundlePage bundleId="b2" />);
    fireEvent.click(screen.getByRole("button", { name: "Retire bundle" }));
    fireEvent.click(screen.getByRole("button", { name: "Retire and revoke" }));
    await waitFor(() => expect(state.remove).toHaveBeenCalledWith("b2"));
    // The refetch the mutation triggers, which the mocked list cannot do itself.
    state.bundles = [bundle()];
  }

  it("keeps the report on screen and retires the destructive action", async () => {
    await retireTheSecondBundle();

    await waitFor(() =>
      expect(document.body.textContent).toMatch(/Mill Certified retired/),
    );
    // Nothing left to destroy, so nothing offers to.
    expect(screen.queryByRole("button", { name: "Retire and revoke" })).toBeNull();
    // And the way out stops being a refusal, because there is nothing to refuse.
    expect(screen.queryByRole("button", { name: "Keep it" })).toBeNull();
    expect(screen.getByRole("button", { name: "Close" })).toBeTruthy();
  });

  it("goes back to the list only when the operator dismisses", async () => {
    await retireTheSecondBundle();
    await waitFor(() => expect(screen.getByRole("button", { name: "Close" })).toBeTruthy());

    // Still on the retired bundle until this click: the report is being read.
    expect(state.push).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "Close" }));

    expect(state.push).toHaveBeenCalledWith("/bundles");
  });

  it("keeps the destructive action when the delete failed", async () => {
    state.remove = vi.fn().mockRejectedValue(new Error("the identity provider is unreachable"));
    render(<BundlePage bundleId="b1" />);
    fireEvent.click(screen.getByRole("button", { name: "Retire bundle" }));
    fireEvent.click(screen.getByRole("button", { name: "Retire and revoke" }));

    await waitFor(() => expect(document.body.textContent).toMatch(/unreachable/i));
    // Nothing was deleted, so the offer stands and the way out is still a
    // refusal. Relabelling here would tell the operator it worked.
    expect(screen.getByRole("button", { name: "Retire and revoke" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "Keep it" })).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Close" })).toBeNull();
    expect(state.push).not.toHaveBeenCalled();
  });
});
