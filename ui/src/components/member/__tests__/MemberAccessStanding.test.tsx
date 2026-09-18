// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { MemberAccess } from "@/components/member/MemberAccess";

const access = vi.hoisted(() => ({ value: {} as Record<string, unknown> }));

vi.mock("@/components/member/MemberCatalog", () => ({ MemberCatalog: () => null }));
vi.mock("@/lib/queries/useUsers", () => ({
  useUserAccess: () => access.value,
  useUserGrants: () => ({ data: [] }),
}));

const session = { id: "u1", name: "Sam", email: "sam@x", role: "member" } as never;

function view(
  observation: Record<string, unknown> | undefined,
  observedRoleKeys: string[] | undefined,
  extra: Record<string, unknown> = {},
) {
  return {
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
          observed_role_keys: observedRoleKeys,
        },
      ],
      bundles: [],
      allowances: [],
      observation,
      ...extra,
    },
  };
}

const COMPLETE = { read_at: "2026-09-18T10:00:00Z", current: true, truncated: false };

/**
 * A member's row says whether Zitadel actually holds the role — from the SAME
 * response that carries the records explaining it, not a second call whose
 * answer could disagree with the headline above it. Absence is only stated
 * after a complete read; a partial one says it could not check.
 */
describe("MemberAccess standing", () => {
  it("marks a held role ready and a missing one not there yet after a complete read", () => {
    access.value = view(COMPLETE, ["laser"]);
    render(<MemberAccess session={session} />);
    expect(screen.getByText("Ready to use")).toBeTruthy();
    expect(screen.getByText("Not there yet")).toBeTruthy();
  });

  it("never concludes an absence from a partial read", () => {
    access.value = view({ ...COMPLETE, truncated: true }, ["laser"]);
    render(<MemberAccess session={session} />);
    expect(screen.getByText("Ready to use")).toBeTruthy();
    expect(screen.queryByText("Not there yet")).toBeNull();
    expect(screen.getByText("Couldn’t check")).toBeTruthy();
  });

  it("says checking, not missing, when nothing has been observed", () => {
    access.value = view({ current: false, truncated: false }, undefined);
    render(<MemberAccess session={session} />);
    expect(screen.queryByText("Not there yet")).toBeNull();
    expect(screen.queryByText("Ready to use")).toBeNull();
  });

  // The headline used to sum effective_role_keys — Syndra's records — above
  // rows marked from Zitadel. "Two permissions" sat over two rows that both
  // said the role had not arrived yet.
  it("counts what Zitadel holds in the headline, not what was decided", () => {
    access.value = view(COMPLETE, ["laser"], { observed_role_count: 1 });
    render(<MemberAccess session={session} />);
    expect(screen.getByText("One permission.")).toBeTruthy();
  });

  it("does not name a number of permissions before anything has been observed", () => {
    access.value = view({ current: false, truncated: false }, undefined);
    render(<MemberAccess session={session} />);
    expect(screen.getByText("Checking what you can use.")).toBeTruthy();
  });

  // Access the person holds that Syndra has no reason for. The operator's
  // People page calls it "unexplained"; the holder used to see nothing at all.
  it("shows a role Zitadel holds that no record explains", () => {
    access.value = view(COMPLETE, ["laser", "kiln"], { observed_role_count: 2 });
    render(<MemberAccess session={session} />);
    expect(screen.getByText("Kiln")).toBeTruthy();
    expect(screen.getByText("You can use this. Nobody wrote down who gave it to you.")).toBeTruthy();
  });
});
