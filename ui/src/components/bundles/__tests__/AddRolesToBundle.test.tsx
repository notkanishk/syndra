// @vitest-environment jsdom
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { AddRolesToBundle } from "@/components/bundles/AddRolesToBundle";
import type { BundleRow } from "@/lib/queries/useBundles";
import type { CatalogRole } from "@/lib/queries/useRoles";

type RoleRow = { zitadel_project_id: string; zitadel_role_key: string };

const state = vi.hoisted(() => ({
  catalog: [] as CatalogRole[],
  existing: [] as RoleRow[],
  addRole: vi.fn(),
  include: vi.fn(),
  bundles: [] as BundleRow[],
  bundleRoles: {} as Record<string, RoleRow[]>,
  includes: [] as Array<{ child_bundle_id: string }>,
}));

vi.mock("@/lib/queries/useRoles", () => ({
  useGlobalRoleCatalog: () => ({ data: state.catalog }),
}));

vi.mock("@/lib/queries/useBundles", () => ({
  useBundleRoles: () => ({ data: state.existing }),
  useAddBundleRole: () => ({ mutateAsync: state.addRole }),
  useSetBundleInclude: () => ({ mutateAsync: state.include }),
  useBundles: () => ({ data: state.bundles }),
  useBundleRolesByBundle: (ids: string[]) => ({
    byId: Object.fromEntries(ids.map((id) => [id, state.bundleRoles[id] ?? []])),
    allLoaded: true,
  }),
}));

vi.mock("@/lib/queries/useBundleVersions", () => ({
  useBundleDraft: () => ({ data: { includes: state.includes } }),
}));


function role(overrides: Partial<CatalogRole> = {}): CatalogRole {
  return {
    project_id: "p-print",
    project_name: "Printing Lab",
    role_key: "trained",
    display_name: "Trained operator",
    description: "",
    bundle_count: 0,
    rule_count: 0,
    assigned_user_count: 0,
    is_unused: false,
    source: "syndra",
    ...overrides,
  };
}

beforeEach(() => {
  state.catalog = [
    role(),
    role({ role_key: "maintainer", display_name: "Maintainer" }),
    role({
      project_id: "p-metal",
      project_name: "Metal Shop",
      role_key: "trained",
      display_name: "Trained operator",
    }),
  ];
  state.existing = [];
  state.addRole = vi.fn().mockResolvedValue({});
  state.include = vi.fn().mockResolvedValue({});
  state.bundles = [];
  state.bundleRoles = {};
  state.includes = [];
});

function open() {
  return render(
    <AddRolesToBundle bundleId="b-1" name="Lab Tech" holders={4} onClose={vi.fn()} />,
  );
}

// Each project is a fold, closed until searched in or something in it is chosen.
function openFolds() {
  for (const fold of screen.getAllByRole("button", { expanded: false })) fireEvent.click(fold);
}

describe("AddRolesToBundle", () => {
  it("groups the whole catalogue by project so one search spans all of them", () => {
    open();
    expect(screen.getByText("Printing Lab")).toBeInTheDocument();
    expect(screen.getByText("Metal Shop")).toBeInTheDocument();
  });

  it("keeps each project folded until the operator opens it", () => {
    open();
    const fold = screen.getByRole("button", { name: /Printing Lab/ });
    expect(fold).toHaveAttribute("aria-expanded", "false");
    expect(fold).toHaveTextContent("2 roles");

    fireEvent.click(fold);
    expect(fold).toHaveAttribute("aria-expanded", "true");
  });

  it("unticks a role from its chip", () => {
    open();
    fireEvent.click(screen.getByRole("button", { name: /Printing Lab/ }));
    fireEvent.click(screen.getByRole("checkbox", { name: /Maintainer/ }));

    expect(screen.getByRole("button", { name: "Add 1 role" })).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Remove Maintainer" }));

    expect(screen.queryByRole("button", { name: "Remove Maintainer" })).toBeNull();
    expect(screen.getByRole("checkbox", { name: /Maintainer/ })).not.toBeChecked();
    expect(screen.queryByRole("button", { name: /^Add \d+ roles?$/ })).toBeNull();
  });

  // S15: a chosen chip tints in and rises 4px, the same as the one-role picker's.
  it("a chosen role's chip arrives with the chip rise, not a bare tint", () => {
    open();
    fireEvent.click(screen.getByRole("button", { name: /Printing Lab/ }));
    fireEvent.click(screen.getByRole("checkbox", { name: /Maintainer/ }));
    const remove = screen.getByRole("button", { name: "Remove Maintainer" });
    expect(remove.closest(".chip-in")).not.toBeNull();
    expect(remove.closest(".tint-in")).toBeNull();
  });

  it("searches across projects, not within one", async () => {
    open();
    fireEvent.change(screen.getByLabelText("Search roles"), { target: { value: "metal" } });
    expect(screen.getByText("Metal Shop")).toBeInTheDocument();
    expect(screen.queryByText("Printing Lab")).not.toBeInTheDocument();
  });

  // Absent reads as "doesn't exist", which is how somebody ends up creating a
  // duplicate role for something the bundle already carries.
  it("shows roles already in the bundle as held rather than hiding them", () => {
    state.existing = [{ zitadel_project_id: "p-print", zitadel_role_key: "trained" }];
    open();
    const checkboxes = screen.getAllByRole("checkbox") as HTMLInputElement[];
    const held = checkboxes.filter((box) => box.disabled);
    expect(held).toHaveLength(1);
    expect(held[0].checked).toBe(true);
    expect(screen.getByText("already in Lab Tech")).toBeInTheDocument();
  });

  it("adds every picked role, one write at a time", async () => {
    open();
    openFolds();
    const boxes = screen.getAllByRole("checkbox");
    fireEvent.click(boxes[0]);
    fireEvent.click(boxes[1]);
    fireEvent.click(screen.getByRole("button", { name: "Add 2 roles" }));

    await waitFor(() => expect(state.addRole).toHaveBeenCalledTimes(2));
  });

  /**
   * The property the deleted picker had and the two-select row never did: the
   * API takes one role per call, so a mid-loop failure leaves a bundle
   * half-built. What already landed stays landed, what didn't stays ticked.
   */
  it("stops at the first failure and keeps the rest selected", async () => {
    state.addRole = vi
      .fn()
      .mockResolvedValueOnce({})
      .mockRejectedValueOnce(new Error("Zitadel refused that role."));

    open();
    openFolds();
    const boxes = screen.getAllByRole("checkbox");
    fireEvent.click(boxes[0]);
    fireEvent.click(boxes[1]);
    fireEvent.click(boxes[2]);
    fireEvent.click(screen.getByRole("button", { name: "Add 3 roles" }));

    // Third call never fires — the loop stops rather than pressing on.
    await waitFor(() => expect(state.addRole).toHaveBeenCalledTimes(2));
    expect(await screen.findByText(/Zitadel refused that role/)).toBeInTheDocument();
    // One succeeded and was cleared; two remain ticked and resumable.
    expect(screen.getByRole("button", { name: "Add 2 roles" })).toBeInTheDocument();
  });

  it("does not close the dialog on a failed apply", async () => {
    const onClose = vi.fn();
    state.addRole = vi.fn().mockRejectedValue(new Error("nope"));

    render(<AddRolesToBundle bundleId="b-1" name="Lab Tech" holders={4} onClose={onClose} />);
    openFolds();
    fireEvent.click(screen.getAllByRole("checkbox")[0]);
    fireEvent.click(screen.getByRole("button", { name: "Add 1 role" }));

    await waitFor(() => expect(state.addRole).toHaveBeenCalled());
    expect(onClose).not.toHaveBeenCalled();
  });
});

/**
 * B9 (3a): the picker on a bundle's page searches bundles too. Choosing one
 * includes it; one level deep, so some can't be chosen, and each says why.
 */
describe("AddRolesToBundle · bundles", () => {
  function bundle(overrides: Partial<BundleRow>): BundleRow {
    return { id: "x", name: "X", latest_version: 1, ...overrides };
  }

  beforeEach(() => {
    state.bundles = [
      bundle({ id: "b-1", name: "Lab Tech" }),
      bundle({
        id: "sb",
        name: "Safety basics",
        description: "Everyone who has done the safety induction",
        latest_version: 3,
        included_in: ["Staff"],
      }),
      bundle({ id: "st", name: "Staff", latest_version: 2, has_includes: true }),
    ];
    state.bundleRoles = { sb: [{ zitadel_project_id: "p-print", zitadel_role_key: "trained" }] };
  });

  it("lists the other bundles first, never the one being edited", () => {
    open();
    fireEvent.click(screen.getByRole("button", { name: /^Bundles/ }));
    expect(screen.getByRole("checkbox", { name: /^Safety basics· v3 · 1 role/ })).toBeInTheDocument();
    expect(screen.queryByRole("checkbox", { name: /^Lab Tech/ })).toBeNull();
  });

  it("says why a bundle that includes others can't go inside", () => {
    open();
    fireEvent.click(screen.getByRole("button", { name: /^Bundles/ }));
    const staff = screen.getByRole("checkbox", { name: /^Staff/ });
    expect(staff).toBeDisabled();
    expect(screen.getByText("Includes Safety basics, so it can't go inside another bundle")).toBeInTheDocument();
  });

  it("blocks every bundle when the one being edited is inside another", () => {
    state.bundles[0] = bundle({ id: "b-1", name: "Lab Tech", included_in: ["New members"] });
    open();
    fireEvent.click(screen.getByRole("button", { name: /^Bundles/ }));
    expect(screen.getAllByText("Lab Tech is inside New members, so it can't include other bundles")).toHaveLength(2);
    expect(screen.getByRole("checkbox", { name: /^Safety basics/ })).toBeDisabled();
  });

  it("ticks and locks a chosen bundle's roles, and includes it on Add", async () => {
    open();
    fireEvent.click(screen.getByRole("button", { name: /^Bundles/ }));
    fireEvent.click(screen.getByRole("checkbox", { name: /^Safety basics/ }));

    expect(screen.getByText("· bundle · 1 role")).toBeInTheDocument();
    // Its project opens on its own: something in it is now chosen.
    const trained = screen.getByRole("checkbox", { name: /^Trained operator.*In Safety basics/ });
    expect(trained).toBeDisabled();

    fireEvent.click(screen.getByRole("button", { name: "Add 1 bundle" }));
    await waitFor(() => expect(state.include).toHaveBeenCalledWith({ child: "sb" }));
    expect(state.addRole).not.toHaveBeenCalled();
  });

  it("shows a bundle already included as held", () => {
    state.includes = [{ child_bundle_id: "sb" }];
    open();
    const sb = screen.getByRole("checkbox", { name: /^Safety basics/ });
    expect(sb).toBeDisabled();
    expect(sb).toBeChecked();
  });
});
