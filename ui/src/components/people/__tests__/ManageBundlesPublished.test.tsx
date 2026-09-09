// @vitest-environment jsdom
import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ManageBundles } from "@/components/people/ManageBundles";

/**
 * What the assign panel promises has to be what the assign gives.
 *
 * An assignment pins the latest PUBLISHED version of a bundle and nothing else.
 * This panel asked for the working copy, so it listed every unpublished edit as
 * a role the person was about to receive — the operator saw the roles they had
 * just added promised here and called "unpublished" on the bundle screen, and
 * the apply then granted neither of the sets they had been shown.
 */

const state = vi.hoisted(() => ({ roleQueries: vi.fn() }));

vi.mock("@/lib/queries/useBundles", () => ({
  useBundles: () => ({
    data: [
      {
        id: "b1",
        name: "Lab Tech",
        latest_version: 2,
        unpublished_changes: 2,
        holder_count: 0,
      },
    ],
  }),
  useBundleRolesByBundle: (...args: unknown[]) => state.roleQueries(...args),
  useRemoveBundle: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));

vi.mock("@/lib/queries/useMappingRules", () => ({ useMappingRules: () => ({ data: [] }) }));

vi.mock("@/lib/queries/useUsers", () => ({
  useAssignBundle: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useUserAccess: () => ({ data: { projects: [] } }),
}));

vi.mock("@/components/names", () => ({
  RoleRef: ({ roleKey }: { roleKey: string }) => <span>{roleKey}</span>,
}));

beforeEach(() => {
  state.roleQueries = vi.fn().mockReturnValue({
    byId: {
      b1: [{ bundle_id: "b1", zitadel_project_id: "p1", zitadel_role_key: "published-role" }],
    },
    allLoaded: true,
  });
});

function open() {
  return render(
    <ManageBundles userId="u1" userName="Ada" assigned={[]} open onClose={vi.fn()} />,
  );
}

describe("ManageBundles", () => {
  it("asks for the published version, not the working copy", () => {
    open();

    expect(state.roleQueries).toHaveBeenCalledWith(["b1"], { published: true });
  });

  it("previews the roles the assignment will actually grant", () => {
    open();
    // Tick the bundle to select it for preview.
    fireEvent.click(screen.getByRole("checkbox", { name: /Lab Tech/ }));

    expect(screen.getByText("published-role")).toBeInTheDocument();
  });

  // The panel is right and the bundle screen shows a different set, so an
  // operator who has just added those roles would read their absence here as
  // this panel being stale rather than as the truth about what a pin means.
  it("says the unpublished edits are not part of the assignment", () => {
    open();
    fireEvent.click(screen.getByRole("checkbox", { name: /Lab Tech/ }));

    expect(screen.getByText(/2 unpublished changes/)).toBeInTheDocument();
    expect(screen.getByText(/an assignment gives v2/)).toBeInTheDocument();
  });
});
