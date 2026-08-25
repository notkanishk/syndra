// @vitest-environment jsdom
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { AddRolesToBundle } from "@/components/bundles/AddRolesToBundle";
import type { CatalogRole } from "@/lib/queries/useRoles";

const state = vi.hoisted(() => ({
  catalog: [] as CatalogRole[],
  existing: [] as Array<{ zitadel_project_id: string; zitadel_role_key: string }>,
  addRole: vi.fn(),
}));

vi.mock("@/lib/queries/useRoles", () => ({
  useGlobalRoleCatalog: () => ({ data: state.catalog }),
}));

vi.mock("@/lib/queries/useBundles", () => ({
  useBundleRoles: () => ({ data: state.existing }),
  useAddBundleRole: () => ({ mutateAsync: state.addRole }),
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
});

function open() {
  return render(
    <AddRolesToBundle bundleId="b-1" name="Lab Tech" holders={4} onClose={vi.fn()} />,
  );
}

describe("AddRolesToBundle", () => {
  it("groups the whole catalogue by project so one search spans all of them", () => {
    open();
    expect(screen.getByText("Printing Lab")).toBeInTheDocument();
    expect(screen.getByText("Metal Shop")).toBeInTheDocument();
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
    fireEvent.click(screen.getAllByRole("checkbox")[0]);
    fireEvent.click(screen.getByRole("button", { name: "Add 1 role" }));

    await waitFor(() => expect(state.addRole).toHaveBeenCalled());
    expect(onClose).not.toHaveBeenCalled();
  });
});
