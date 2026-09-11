// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import RoleMembersPage from "@/app/projects/[id]/roles/[key]/page";
import type { RoleMembersView } from "@/lib/queries/useRoleMembers";

/**
 * The page reads its route params through React's `use()` on a Promise,
 * which suspends on first render even for an already-resolved plain
 * Promise — `use()` only reads synchronously off a thenable already tagged
 * "fulfilled", the shape React gives its own cached route-param promises.
 * Handing it one directly is what every real render of this page gets from
 * Next.js, and it's what keeps this test synchronous.
 */
function fulfilled<T>(value: T): Promise<T> {
  return { status: "fulfilled", value, then() {} } as unknown as Promise<T>;
}

const state = vi.hoisted(() => ({ data: undefined as RoleMembersView | undefined }));

vi.mock("@/lib/queries/useRoleMembers", () => ({
  useRoleMembers: () => ({ data: state.data, isLoading: false, error: null, refetch: () => {} }),
  useRemoveDirectGrant: () => ({ mutate: vi.fn(), isPending: false }),
}));

function view(overrides: Partial<RoleMembersView> = {}): RoleMembersView {
  return {
    project_id: "pLaser",
    project_name: "Laser Lab",
    role_key: "trained",
    members: [],
    withheld_count: 0,
    direct_count: 0,
    bundle_count: 0,
    automatic_count: 0,
    observed_only: null,
    ...overrides,
  };
}

function renderPage() {
  return render(<RoleMembersPage params={fulfilled({ id: "pLaser", key: "trained" })} />);
}

describe("role members — holding is observed", () => {
  it("counts people Zitadel shows holding it even with no Syndra record", () => {
    state.data = view({
      members: [
        {
          user: { id: "u1", name: "Ada Lovelace", email: "ada@example.edu", title: "", team: "" },
          reasons: [{ kind: "direct" }],
        },
      ],
      observed_only: [
        { id: "u2", name: "Nina Roy", email: "nina@example.edu", title: "", team: "", status: "active", avatar: "" },
      ],
    });
    renderPage();

    expect(screen.getByText("2 people hold this role")).toBeInTheDocument();
    expect(screen.getByText("In Zitadel without Syndra giving it")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Nina Roy/ })).toHaveAttribute("href", "/users/u2");
    expect(screen.getByRole("link", { name: /Review in drift/ })).toHaveAttribute(
      "href",
      "/governance/drift?project=pLaser",
    );
  });

  it("says nothing extra when nothing was observed beyond Syndra's record", () => {
    state.data = view({
      members: [
        {
          user: { id: "u1", name: "Ada Lovelace", email: "ada@example.edu", title: "", team: "" },
          reasons: [{ kind: "direct" }],
        },
      ],
      observed_only: null,
    });
    renderPage();

    expect(screen.getByText("1 person holds this role")).toBeInTheDocument();
    expect(screen.queryByText("In Zitadel without Syndra giving it")).not.toBeInTheDocument();
  });

  it("flags a member Syndra granted it to but Zitadel doesn't show holding it", () => {
    state.data = view({
      members: [
        {
          user: { id: "u1", name: "Ada Lovelace", email: "ada@example.edu", title: "", team: "" },
          reasons: [{ kind: "direct" }],
          in_zitadel: false,
        },
      ],
    });
    renderPage();

    expect(screen.getByText("Not in Zitadel yet")).toBeInTheDocument();
  });

  it("shows the email when there is no title or team to show instead", () => {
    state.data = view({
      members: [
        {
          user: { id: "u1", name: "Ada Lovelace", email: "ada@example.edu", title: "", team: "" },
          reasons: [{ kind: "direct" }],
        },
      ],
    });
    renderPage();

    expect(screen.getByText("ada@example.edu")).toBeInTheDocument();
  });
});
