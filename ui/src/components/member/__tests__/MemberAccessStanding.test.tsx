// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { MemberAccess, summarySentence } from "@/components/member/MemberAccess";

const access = vi.hoisted(() => ({ value: {} as Record<string, unknown> }));
const grants = vi.hoisted(() => ({ value: [] as unknown[] }));
const changes = vi.hoisted(() => ({ value: [] as unknown[] }));

vi.mock("@/components/member/MemberCatalog", () => ({ MemberCatalog: () => null }));
vi.mock("@/components/names", () => ({
  ResolvedProjectName: ({ name }: { name: string }) => <span>{name}</span>,
  UserName: ({ id }: { id: string }) => <span>{id}</span>,
  BundleName: ({ id }: { id: string }) => <span>{id}</span>,
}));
vi.mock("@/lib/queries/useUsers", () => ({
  useUserAccess: () => access.value,
  useUserGrants: () => ({ data: grants.value }),
}));
vi.mock("@/lib/queries/useMyChanges", () => ({
  useMyChanges: () => ({ data: changes.value }),
}));
vi.mock("@/lib/queries/useCatalogUsers", () => ({
  useCatalogProjects: () => ({
    data: [
      {
        id: "p1",
        name: "Fabrication",
        roles: [
          { key: "laser", label: "Laser cutter · Member" },
          { key: "mill", label: "Mill · Operator" },
        ],
      },
    ],
  }),
}));

const session = { id: "u1", name: "Sam", email: "sam@x", role: "member" } as never;

function view(
  observation: Record<string, unknown> | undefined,
  observedRoleKeys: string[] | undefined,
  extra: Record<string, unknown> = {},
  reasons: Record<string, unknown> = {},
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
            { role_key: "laser", reasons: [{ kind: "direct", queued: false, ...reasons }] },
            { role_key: "mill", reasons: [{ kind: "direct", queued: false, ...reasons }] },
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
const inDays = (n: number) => new Date(Date.now() + n * 864e5).toISOString();

function reset() {
  grants.value = [];
  changes.value = [];
}

/**
 * A member's row says whether Zitadel actually holds the role — from the SAME
 * response that carries the records explaining it. Healthy is silence: only
 * "Not there yet" (after a complete read) and "Couldn't check" speak.
 */
describe("MemberAccess standing", () => {
  it("says nothing for a held role and 'Not there yet' for a missing one", () => {
    reset();
    access.value = view(COMPLETE, ["laser"]);
    render(<MemberAccess session={session} />);
    expect(screen.queryByText("Ready to use")).toBeNull();
    expect(screen.getAllByText("Not there yet")).toHaveLength(1);
    expect(screen.getAllByText("Given to you")).toHaveLength(2);
  });

  it("never concludes an absence from a partial read", () => {
    reset();
    access.value = view({ ...COMPLETE, truncated: true }, ["laser"]);
    render(<MemberAccess session={session} />);
    expect(screen.queryByText("Not there yet")).toBeNull();
    expect(screen.getByText("Couldn’t check")).toBeTruthy();
  });

  it("says neither while nothing has been observed", () => {
    reset();
    access.value = view({ current: false, truncated: false }, undefined);
    render(<MemberAccess session={session} />);
    expect(screen.queryByText("Not there yet")).toBeNull();
    expect(screen.queryByText("Couldn’t check")).toBeNull();
    expect(screen.getByText("Checking what you can use.")).toBeTruthy();
  });

  it("names roles by their catalogue label", () => {
    reset();
    access.value = view(COMPLETE, ["laser", "mill"]);
    render(<MemberAccess session={session} />);
    expect(screen.getByText("Laser cutter · Member")).toBeTruthy();
  });

  it("shows a role Zitadel holds that no record explains", () => {
    reset();
    access.value = view(COMPLETE, ["laser", "kiln"], { observed_role_count: 2 });
    render(<MemberAccess session={session} />);
    expect(screen.getByText("Kiln")).toBeTruthy();
    expect(screen.getByText("You can use this. Nobody wrote down who gave it to you.")).toBeTruthy();
  });

  // Members never see the operator's step: they can't send.
  it("never tells a member to send", () => {
    reset();
    access.value = view(COMPLETE, [], {}, { queued: true });
    render(<MemberAccess session={session} />);
    expect(screen.queryByText(/Waiting for you to send/)).toBeNull();
    expect(screen.getAllByText("Not there yet")).toHaveLength(2);
  });
});

describe("MemberAccess expiry lives on its row", () => {
  it("tints the role that runs out, says when, offers to keep it, and sorts it first", () => {
    reset();
    grants.value = [{ project_id: "p1", role_key: "mill", expires_at: inDays(10) }];
    access.value = view(COMPLETE, ["laser", "mill"], { observed_role_count: 2 });
    const { container } = render(<MemberAccess session={session} />);

    const rows = [...container.querySelectorAll(".row-divider")];
    expect(rows[0].textContent).toContain("Mill · Operator");
    expect(rows[0].getAttribute("data-ends")).toBe("true");
    expect(rows[0].className).toContain("bg-warn-soft");
    expect(rows[0].textContent).toMatch(/Given to you until .+ · 10 days left/);
    const keep = screen.getByText("Ask to keep it longer").closest("a")!;
    expect(keep.getAttribute("href")).toBe("/requests?project=p1&role=mill&extend=1");
    expect(rows[1].getAttribute("data-ends")).toBeNull();
    // No separate warning box any more.
    expect(container.querySelector(".warn-note")).toBeNull();
  });

  it("puts it in the summary", () => {
    expect(summarySentence(4, 2, ["2026-10-20T10:00:00Z"])).toBe(
      "Four roles across two rooms. One runs out on 20 Oct.",
    );
    expect(summarySentence(1, 1, [])).toBe("One role across one room.");
    expect(summarySentence(undefined, 0, [])).toBe("Checking what you can use.");
  });
});

describe("MemberAccess recent changes", () => {
  it("leads the page when there is news, and is not drawn on a quiet month", () => {
    reset();
    access.value = view(COMPLETE, ["laser"]);
    const quiet = render(<MemberAccess session={session} />);
    expect(screen.queryByText("Recent changes")).toBeNull();
    quiet.unmount();

    changes.value = [
      { kind: "request_declined", at: inDays(-2), project_id: "p1", role_key: "mill", by: "op", note: "n", via: null },
    ];
    const { container } = render(<MemberAccess session={session} />);
    const heading = screen.getByText("Recent changes");
    const firstCardTitle = screen.getByText("Fabrication");
    expect(heading.compareDocumentPosition(firstCardTitle) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(container.textContent).toContain("All your requests →");
  });

  it("shows in the empty state too: a member with nothing still sees a declined request", () => {
    reset();
    changes.value = [
      { kind: "request_declined", at: inDays(-2), project_id: "p1", role_key: "laser", by: "op", via: null },
    ];
    access.value = { isLoading: false, error: null, data: { projects: [], bundles: [], allowances: [], observation: COMPLETE, observed_role_count: 0 } };
    render(<MemberAccess session={session} />);
    expect(screen.getByText("Recent changes")).toBeTruthy();
    expect(screen.getByText("You don't have access to anything yet.")).toBeTruthy();
  });

  // N19: the approval's role sits in its room, not there yet, and why.
  it("says why an approved role isn't on the account yet, in its row", () => {
    reset();
    changes.value = [
      { kind: "request_approved", at: new Date().toISOString(), project_id: "p1", role_key: "mill", by: "op", via: null, waiting: true, wait_reason: "role_waits" },
    ];
    access.value = view(COMPLETE, ["laser"], {}, { queued: true });
    render(<MemberAccess session={session} />);
    expect(
      screen.getByText("Not there yet · approved today, and makerspace staff check this role before it reaches you"),
    ).toBeTruthy();
  });

  it("says Syndra couldn't reach the sign-in system when the send halted", () => {
    reset();
    changes.value = [
      { kind: "request_approved", at: new Date().toISOString(), project_id: "p1", role_key: "mill", by: "op", via: null, waiting: true, wait_reason: "halted:zitadel_offline" },
    ];
    access.value = view(COMPLETE, ["laser"], {}, { queued: true });
    render(<MemberAccess session={session} />);
    expect(
      screen.getByText("Not there yet · approved today, and Syndra couldn't reach the sign-in system; staff will send it"),
    ).toBeTruthy();
  });
});
