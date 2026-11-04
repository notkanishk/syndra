// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { MemberAccess } from "@/components/member/MemberAccess";

const access = vi.hoisted(() => ({ value: {} as Record<string, unknown> }));
const grants = vi.hoisted(() => ({ value: [] as unknown[] }));
const grantsRead = vi.hoisted(() => ({ failed: false }));
const changes = vi.hoisted(() => ({ value: [] as unknown[] }));
const mine = vi.hoisted(() => ({ requests: [] as unknown[], targets: [] as unknown[] }));

vi.mock("@/components/member/MemberCatalog", () => ({ MemberCatalog: () => null }));
vi.mock("@/components/names", () => ({
  ResolvedProjectName: ({ name }: { name: string }) => <span>{name}</span>,
  UserName: ({ id }: { id: string }) => <span>{id}</span>,
  BundleName: ({ id }: { id: string }) => <span>{id}</span>,
}));
vi.mock("@/lib/queries/useUsers", () => ({
  useUserAccess: () => access.value,
  useUserGrants: () => ({ data: grantsRead.failed ? undefined : grants.value, error: grantsRead.failed ? new Error("down") : null }),
}));
vi.mock("@/lib/queries/useMyStorage", () => ({ useMyStorage: () => ({ data: mine.targets }) }));
vi.mock("@/lib/queries/useRequests", () => ({ useRequestsMine: () => ({ data: mine.requests }) }));
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
  grantsRead.failed = false;
  changes.value = [];
  mine.requests = [];
  mine.targets = [];
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
    expect(screen.getByText("Zitadel hasn't answered yet, so your roles aren't shown.")).toBeTruthy();
  });

  // A-06 §2, N23: Zitadel never read. The card headers stay, with no count, and
  // there are no rows: a row would say what Syndra decided as though it were
  // what the person can use, and nothing has been observed to say so.
  it("shows card headers with no counts and no rows while Zitadel has never been read", () => {
    reset();
    access.value = view(undefined, undefined);
    render(<MemberAccess session={session} />);
    expect(screen.getByText("Fabrication")).toBeTruthy();
    expect(screen.queryByText("Laser cutter · Member")).toBeNull();
    expect(screen.queryByText("Given to you")).toBeNull();
    expect(screen.getByText("Zitadel hasn't answered yet, so your roles aren't shown.")).toBeTruthy();
    expect(screen.queryByText(/\b\d+ roles?\b/)).toBeNull();
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
    // Said nothing about who gave it: unknown is silent (G17).
    expect(screen.queryByText(/Nobody wrote down/)).toBeNull();
  });

  // Members never see the operator's step: they can't send.
  it("never tells a member to send", () => {
    reset();
    access.value = view(COMPLETE, [], {}, { queued: true });
    render(<MemberAccess session={session} />);
    expect(screen.queryByText(/Waiting for you to send/)).toBeNull();
    expect(screen.queryByText(/waiting to be sent/i)).toBeNull();
    expect(screen.getAllByText("On its way")).toHaveLength(2);
    expect(screen.getAllByText("It starts working once makerspace staff send it.")).toHaveLength(2);
  });
});

describe("MemberAccess when the grants were not read", () => {
  it("says the end date is unavailable, never 'Given to you', and names no end", () => {
    reset();
    grantsRead.failed = true;
    access.value = view(COMPLETE, ["laser", "mill"], { observed_role_count: 2 });
    render(<MemberAccess session={session} />);
    expect(screen.getAllByText("End date unavailable")).toHaveLength(2);
    expect(screen.queryByText("Given to you")).toBeNull();
    expect(screen.getByText("Nothing needs you right now.")).toBeTruthy();
  });
});

describe("MemberAccess expiry lives on its row", () => {
  it("tints the role that ends, says when, offers to keep it, and sorts it first", () => {
    reset();
    grants.value = [{ project_id: "p1", role_key: "mill", expires_at: inDays(10) }];
    access.value = view(COMPLETE, ["laser", "mill"], { observed_role_count: 2 });
    const { container } = render(<MemberAccess session={session} />);

    const rows = [...container.querySelectorAll(".row-divider")];
    expect(rows[0].textContent).toContain("Mill · Operator");
    expect(rows[0].getAttribute("data-ends")).toBe("true");
    expect(rows[0].textContent).toMatch(/Given to you until .+ · 10 days left/);
    const keep = screen.getByText("Ask to keep it").closest("a")!;
    expect(keep.textContent).not.toContain("→");
    expect(keep.getAttribute("href")).toBe("/requests?project=p1&role=mill&extend=1");
    expect(rows[1].getAttribute("data-ends")).toBeNull();
    // No separate warning box any more.
    expect(container.querySelector(".warn-note")).toBeNull();
  });
});

describe("MemberAccess recent changes", () => {
  it("leads the page when there is news, and keeps its seat on a quiet month", () => {
    reset();
    access.value = view(COMPLETE, ["laser"]);
    const quiet = render(<MemberAccess session={session} />);
    expect(screen.getByText("Recent changes")).toBeTruthy();
    expect(screen.getByText("Nothing has changed in the last 30 days.")).toBeTruthy();
    quiet.unmount();

    changes.value = [
      { kind: "request_declined", at: inDays(-2), project_id: "p1", role_key: "mill", by: "op", note: "n", via: null },
    ];
    const { container } = render(<MemberAccess session={session} />);
    const heading = screen.getByText("Recent changes");
    const firstCardTitle = container.querySelector("#card-p1")!;
    expect(heading.compareDocumentPosition(firstCardTitle) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(screen.getByRole("link", { name: "All your requests" })).toBeTruthy();
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
    expect(screen.getAllByText("On its way").length).toBeGreaterThan(0);
    expect(
      screen.getByText("Given today. It starts working once makerspace staff send it."),
    ).toBeTruthy();
    expect(screen.queryByText(/Not there yet/)).toBeNull();
  });

  it("dates a role that was given on another day", () => {
    reset();
    changes.value = [
      { kind: "request_approved", at: "2026-01-15T09:00:00Z", project_id: "p1", role_key: "mill", by: "op", via: null, waiting: true, wait_reason: "role_waits" },
    ];
    access.value = view(COMPLETE, ["laser"], {}, { queued: true });
    render(<MemberAccess session={session} />);
    expect(
      screen.getByText(/^Given 15 Jan( 2026)?\. It starts working once makerspace staff send it\.$/),
    ).toBeTruthy();
  });

  it("dates it from when it was queued, the record's own time, over the feed's", () => {
    reset();
    changes.value = [
      { kind: "request_approved", at: "2026-02-01T09:00:00Z", project_id: "p1", role_key: "mill", by: "op", via: null, waiting: true, wait_reason: "role_waits" },
    ];
    access.value = view(COMPLETE, ["laser"], {}, { queued: true, queued_at: "2026-01-15T09:00:00Z" });
    render(<MemberAccess session={session} />);
    expect(
      screen.getAllByText(/^Given 15 Jan( 2026)?\. It starts working once makerspace staff send it\.$/).length,
    ).toBeGreaterThan(0);
  });

  it("says the same when the send halted: a member can't act on why", () => {
    reset();
    changes.value = [
      { kind: "request_approved", at: new Date().toISOString(), project_id: "p1", role_key: "mill", by: "op", via: null, waiting: true, wait_reason: "halted:zitadel_offline" },
    ];
    access.value = view(COMPLETE, ["laser"], {}, { queued: true });
    render(<MemberAccess session={session} />);
    expect(screen.getAllByText("On its way").length).toBeGreaterThan(0);
    expect(
      screen.getByText("Given today. It starts working once makerspace staff send it."),
    ).toBeTruthy();
  });
});

// S11: the member's home is a screen's first paint. The greeting tints in, the cards arrive after it.
describe("MemberAccess first paint", () => {
  it("wears the arrival on the greeting", () => {
    reset();
    access.value = view(COMPLETE, ["laser"]);
    render(<MemberAccess session={session} />);
    expect(screen.getByRole("heading", { level: 1 }).parentElement).toHaveClass("page-head");
  });

  // S7: a read that is slow shows its skeleton only after the delay; the greeting is there throughout.
  it("is greeted before the read lands, with no skeleton until it is slow", () => {
    reset();
    access.value = { isLoading: true, error: null, data: undefined };
    render(<MemberAccess session={session} />);
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent(/^(Morning|Afternoon|Evening), Sam\./);
    expect(screen.getByText("Checking what you can use.")).toBeTruthy();
    expect(screen.queryByLabelText("Loading your access")).toBeNull();
  });
});

describe("MemberAccess header line and cards", () => {
  it("says the calm line when nothing needs them", () => {
    reset();
    access.value = view(COMPLETE, ["laser", "mill"]);
    render(<MemberAccess session={session} />);
    expect(screen.getByText("Nothing needs you right now.")).toBeTruthy();
  });

  // Rung 1 names the role and offers the jump; the row keeps the date (G12).
  it("leads with an ending role and jumps to its card", () => {
    reset();
    grants.value = [{ project_id: "p1", role_key: "mill", expires_at: inDays(7) }];
    access.value = view(COMPLETE, ["laser", "mill"]);
    const { container } = render(<MemberAccess session={session} />);
    expect(screen.getByText("Mill · Operator in Fabrication ends in 7 days.")).toBeTruthy();
    const jump = screen.getByRole("button", { name: /Fabrication/ });
    expect(container.querySelector("#card-p1")).toBeTruthy();
    expect(jump.textContent).toContain("↓");
  });

  it("sends a storage step to Network storage", () => {
    reset();
    mine.targets = [{ target: "truenas", entitled: true, credential: { set: false } }];
    access.value = view(COMPLETE, ["laser", "mill"]);
    render(<MemberAccess session={session} />);
    expect(screen.getByText("Your TrueNAS account is ready. Set its password to connect.")).toBeTruthy();
    expect(screen.getByRole("link", { name: /Network storage/ })).toHaveAttribute("href", "/storage");
  });

  it("orders the project cards A to Z whatever order the API gave", () => {
    reset();
    const base = view(COMPLETE, ["laser"]) as { data: { projects: Array<Record<string, unknown>> } };
    const [fab] = base.data.projects;
    base.data.projects = [
      { ...fab, project_id: "p9", project_name: "Wood shop", observed_role_keys: [] },
      { ...fab, project_id: "p2", project_name: "3D printing", observed_role_keys: [] },
      { ...fab, project_id: "p1", project_name: "Laser cutting", observed_role_keys: [] },
    ];
    access.value = base;
    const { container } = render(<MemberAccess session={session} />);
    const titles = [...container.querySelectorAll("[id^='card-']")].map((el) => el.textContent);
    expect(titles).toEqual(["3D printing", "Laser cutting", "Wood shop"]);
  });
});
