// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import RoleMembersPage from "@/app/projects/[id]/roles/[key]/page";
import type { RoleMember, RoleMembersView } from "@/lib/queries/useRoleMembers";

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

const state = vi.hoisted(() => ({ data: undefined as RoleMembersView | undefined, advanced: false }));
vi.mock("@/lib/ui-view", () => ({ useIsAdvanced: () => state.advanced, useUiView: () => ({ audience: "basic" }) }));

vi.mock("@/lib/queries/useNameResolver", () => ({
  useNameResolver: () => ({
    resolveProject: () => ({ value: undefined, resolved: true }),
  }),
}));
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
    observation: { read_at: "2026-11-10T08:00:00Z", current: true, truncated: false },
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
          user: { id: "u1", name: "Charlotte Flair", email: "charlotte@example.edu", title: "", team: "" },
          reasons: [{ kind: "direct" }],
          standing: "held",
        },
      ],
      observed_only: [
        { id: "u2", name: "Alba Fyre", email: "alba@example.edu", title: "", team: "", status: "active", avatar: "" },
      ],
    });
    renderPage();

    expect(screen.getByText("2 people hold this role")).toBeInTheDocument();
    expect(screen.getByText("In Zitadel only")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Alba Fyre/ })).toHaveAttribute("href", "/users/u2");
    expect(screen.getByRole("link", { name: /Review in drift/ })).toHaveAttribute(
      "href",
      "/governance/drift?project=pLaser&role=trained",
    );
  });

  it("says nothing extra when nothing was observed beyond Syndra's record", () => {
    state.data = view({
      members: [
        {
          user: { id: "u1", name: "Charlotte Flair", email: "charlotte@example.edu", title: "", team: "" },
          reasons: [{ kind: "direct" }],
          standing: "held",
        },
      ],
      observed_only: null,
    });
    renderPage();

    expect(screen.getByText("1 person holds this role")).toBeInTheDocument();
    expect(screen.queryByText("In Zitadel only")).not.toBeInTheDocument();
  });

  it("flags a member Syndra granted it to but Zitadel doesn't show holding it", () => {
    state.data = view({
      members: [
        {
          user: { id: "u1", name: "Charlotte Flair", email: "charlotte@example.edu", title: "", team: "" },
          reasons: [{ kind: "direct" }],
          standing: "not_held",
        },
      ],
    });
    renderPage();

    expect(screen.getByText("Not in Zitadel")).toBeInTheDocument();
    // A record explains; it does not hold. The sentence counts what Zitadel shows.
    expect(screen.getByText("0 people hold this role")).toBeInTheDocument();
  });

  it("counts held and being removed as holding, and sent, waiting and not held as not", () => {
    const member = (id: string, standing: RoleMember["standing"]) => ({
      user: { id, name: `Person ${id}`, email: `${id}@example.edu`, title: "", team: "" },
      reasons: [{ kind: "direct" }],
      standing,
    });
    state.data = view({
      members: [
        member("a", "held"),
        member("b", "removing"),
        member("c", "not_held"),
        member("d", "sent"),
        member("e", "waiting"),
      ],
    });
    renderPage();

    expect(screen.getByText("2 people hold this role")).toBeInTheDocument();
    expect(screen.getByText("Being removed")).toBeInTheDocument();
    expect(screen.getByText("Not in Zitadel")).toBeInTheDocument();
    expect(screen.getByText("Sent, not seen back yet")).toBeInTheDocument();
    expect(screen.getByText("Not sent to Zitadel yet")).toBeInTheDocument();
  });

  it("says a member the backend can't place couldn't be checked, and counts them as not held", () => {
    state.data = view({
      members: [
        {
          user: { id: "u1", name: "Charlotte Flair", email: "charlotte@example.edu", title: "", team: "" },
          reasons: [{ kind: "direct" }],
          standing: null,
        },
      ],
    });
    renderPage();

    expect(screen.getByText("Couldn\u2019t check Zitadel")).toBeInTheDocument();
    expect(screen.getByText("0 people hold this role")).toBeInTheDocument();
  });

  it("does not count when Zitadel was never read", () => {
    state.data = view({
      members: [
        {
          user: { id: "u1", name: "Charlotte Flair", email: "charlotte@example.edu", title: "", team: "" },
          reasons: [{ kind: "direct" }],
          standing: "waiting",
        },
      ],
      observation: { read_at: null, current: false, truncated: false },
    });
    renderPage();

    expect(screen.getByText("Not checked yet")).toBeInTheDocument();
    expect(screen.queryByText(/^\d+ (people hold|person holds) this role$/)).not.toBeInTheDocument();
  });

  it("shows the email when there is no title or team to show instead", () => {
    state.data = view({
      members: [
        {
          user: { id: "u1", name: "Charlotte Flair", email: "charlotte@example.edu", title: "", team: "" },
          reasons: [{ kind: "direct" }],
          standing: "held",
        },
      ],
    });
    renderPage();

    expect(screen.getByText("charlotte@example.edu")).toBeInTheDocument();
  });

  it("prints the raw role key in Advanced and not in Basic (A-20 P17)", () => {
    state.data = view();
    state.advanced = false;
    const { unmount } = renderPage();
    expect(screen.queryByText("trained")).not.toBeInTheDocument();
    unmount();
    state.advanced = true;
    renderPage();
    expect(screen.getByText("trained")).toBeInTheDocument();
    state.advanced = false;
  });
});
