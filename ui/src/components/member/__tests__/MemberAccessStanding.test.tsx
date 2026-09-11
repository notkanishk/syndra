// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { MemberAccess } from "@/components/member/MemberAccess";

const upstream = vi.hoisted(() => ({ value: {} as Record<string, unknown> }));

vi.mock("@/components/member/MemberCatalog", () => ({ MemberCatalog: () => null }));
vi.mock("@/lib/queries/useUsers", () => ({
  useUserAccess: () => ({
    isLoading: false,
    error: null,
    data: {
      projects: [
        {
          project_id: "p1",
          project_name: "Fabrication",
          project_name_resolved: true,
          effective_role_keys: ["laser", "mill"],
          source_roles: [
            { role_key: "laser", reasons: [{ kind: "direct", queued: false }] },
            { role_key: "mill", reasons: [{ kind: "direct", queued: false }] },
          ],
          derived_roles: [],
        },
      ],
      bundles: [],
      allowances: [],
    },
  }),
  useUserGrants: () => ({ data: [] }),
}));
vi.mock("@/lib/queries/useUpstream", () => ({
  useUpstreamUserGrants: () => upstream.value,
}));

const session = { id: "u1", name: "Sam", email: "sam@x", role: "member" } as never;

/**
 * A member's row says whether Zitadel actually holds the role, from the same
 * observer pipe the operator page reads. Absence is only stated after a
 * complete read; a partial one says it could not check.
 */
describe("MemberAccess standing", () => {
  it("marks a held role ready and a missing one not there yet after a complete read", () => {
    upstream.value = {
      isLoading: false,
      error: null,
      data: { items: [{ id: "g", userId: "u1", projectId: "p1", roleKeys: ["laser"] }], complete: true },
    };
    render(<MemberAccess session={session} />);
    expect(screen.getByText("Ready to use")).toBeTruthy();
    expect(screen.getByText("Not there yet")).toBeTruthy();
  });

  it("never concludes an absence from a partial read", () => {
    upstream.value = {
      isLoading: false,
      error: null,
      data: { items: [{ id: "g", userId: "u1", projectId: "p1", roleKeys: ["laser"] }], complete: false },
    };
    render(<MemberAccess session={session} />);
    expect(screen.getByText("Ready to use")).toBeTruthy();
    expect(screen.queryByText("Not there yet")).toBeNull();
    expect(screen.getByText("Couldn’t check")).toBeTruthy();
  });
});
