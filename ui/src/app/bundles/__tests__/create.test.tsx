// @vitest-environment jsdom
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import BundlesPage from "@/app/bundles/page";

/**
 * A bundle is created as the thing it was described as.
 *
 * Creating one with a name alone published an empty v1: assigning it granted
 * nothing, and the roles the operator then added came back to them as "2
 * unpublished changes" against a version that had never described anything.
 * There was no point in that sequence where the screen told the truth about a
 * bundle the operator considered finished.
 */

const state = vi.hoisted(() => ({ create: vi.fn() }));

vi.mock("@/lib/queries/useBundles", () => ({
  useBundles: () => ({ data: [], isLoading: false, refetch: vi.fn() }),
  useBundleRoles: () => ({ data: [], isLoading: false, refetch: vi.fn() }),
  useBundleImpact: () => ({ data: null, isLoading: false }),
  useCreateBundle: () => ({ mutateAsync: state.create, isPending: false }),
  useDeleteBundle: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useRemoveBundleRole: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useSetWelcomeBundle: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useUpdateBundle: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));

vi.mock("@/lib/queries/useBundleVersions", () => ({
  useBundleDraft: () => ({ data: undefined }),
  draftChangeCount: () => 0,
}));

vi.mock("@/lib/queries/useMappingRules", () => ({ useMappingRules: () => ({ data: [] }) }));

vi.mock("@/lib/queries/useRoles", () => ({
  useGlobalRoleCatalog: () => ({
    data: [
      {
        project_id: "p1",
        project_name: "Fabrication",
        role_key: "laser_trained",
        display_name: "Laser trained",
      },
      {
        project_id: "p1",
        project_name: "Fabrication",
        role_key: "cnc_trained",
        display_name: "CNC trained",
      },
    ],
  }),
}));

vi.mock("@/components/names", () => ({
  RoleRef: ({ roleKey }: { roleKey: string }) => <span>{roleKey}</span>,
  UserName: ({ id }: { id: string }) => <span>{id}</span>,
}));

vi.mock("@/components/bundles/BundleVersions", () => ({ BundleVersions: () => null }));

beforeEach(() => {
  state.create = vi.fn().mockResolvedValue({ id: "b1" });
});

function openDialog() {
  render(<BundlesPage />);
  fireEvent.click(screen.getByRole("button", { name: "New bundle" }));
}

describe("creating a bundle", () => {
  it("will not create one with no roles, and says why", () => {
    openDialog();
    fireEvent.change(screen.getByLabelText("Name"), { target: { value: "Lab Tech" } });

    expect(screen.getByRole("button", { name: "Create bundle" })).toBeDisabled();
    expect(screen.getByText(/Pick at least one role/)).toBeInTheDocument();
    expect(state.create).not.toHaveBeenCalled();
  });

  it("will not create one with no name, and says why", () => {
    openDialog();
    fireEvent.click(screen.getByRole("checkbox", { name: /Laser trained/ }));

    expect(screen.getByRole("button", { name: /^Create with 1 role$/ })).toBeDisabled();
    expect(screen.getByText("A bundle needs a name.")).toBeInTheDocument();
  });

  it("sends the ticked roles, so v1 is already what was described", async () => {
    openDialog();
    fireEvent.change(screen.getByLabelText("Name"), { target: { value: "  Lab Tech  " } });
    fireEvent.click(screen.getByRole("checkbox", { name: /Laser trained/ }));
    fireEvent.click(screen.getByRole("checkbox", { name: /CNC trained/ }));

    const create = screen.getByRole("button", { name: /^Create with 2 roles$/ });
    expect(create).toBeEnabled();
    fireEvent.click(create);

    await waitFor(() => expect(state.create).toHaveBeenCalled());
    expect(state.create.mock.calls[0][0]).toMatchObject({
      name: "Lab Tech",
      roles: [
        { project_id: "p1", role_key: "laser_trained" },
        { project_id: "p1", role_key: "cnc_trained" },
      ],
    });
  });
});
