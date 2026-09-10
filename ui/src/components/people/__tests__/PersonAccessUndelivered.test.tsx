// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { PersonAccess } from "@/components/people/PersonAccess";

/**
 * A role nothing has delivered must not read as held.
 *
 * The page builds access from the assignment tables, which say what somebody
 * has been GIVEN. In manual mode nothing reaches Zitadel until an operator
 * confirms Pending changes, and for that whole window a queued role rendered
 * with the same chip, the same section and the same "No expiry" as one that
 * landed a month ago. The operator who reported it took the page as
 * confirmation the work was done.
 */

const state = vi.hoisted(() => ({
  projects: [] as Array<Record<string, unknown>>,
  advanced: false,
  // What Zitadel reports. A role Syndra records and Zitadel does not have is
  // not "granted" — it is a discrepancy, and the page must say so.
  zitadel: [] as Array<{ id: string; projectId: string; roleKeys: string[] }>,
  unreachable: false,
}));

vi.mock("@/lib/ui-view", () => ({
  useIsAdvanced: () => state.advanced,
  useUiView: () => ({ revealInAdvanced: vi.fn() }),
}));

vi.mock("@/lib/queries/useUpstream", () => ({
  useUpstreamUserGrants: () => ({
    data: { items: state.zitadel, total: state.zitadel.length },
    isLoading: false,
    error: state.unreachable ? new Error("unreachable") : null,
  }),
}));

vi.mock("@/lib/queries/useUsers", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/queries/useUsers")>()),
  useUserAccess: () => ({
    data: {
      user: { id: "u1", name: "Shikha Yadav", email: "s@example.edu", status: "active", avatar: "SY" },
      bundles: [],
      projects: state.projects,
      allowances: [],
      cleanup_hints: [],
    },
    isLoading: false,
    error: null,
    refetch: () => {},
  }),
  useUserGrants: () => ({ data: [], isLoading: false, error: null }),
}));

vi.mock("@/lib/queries/useAudit", () => ({
  useAuditEntries: () => ({ data: [], isLoading: false, error: null, refetch: () => {} }),
}));
vi.mock("@/lib/queries/useRequests", () => ({
  useRequestsAdmin: () => ({ data: [], isLoading: false, error: null, refetch: () => {} }),
  useDecideRequest: () => ({ isPending: false, mutateAsync: vi.fn() }),
}));

function project(reasons: Array<Record<string, unknown>>) {
  return [
    {
      project_id: "p-audio",
      project_name: "Audio-Dash",
      source_roles: [
        { project_id: "p-audio", project_name: "Audio-Dash", role_key: "community", is_source: true, reasons },
      ],
      derived_roles: [],
      effective_role_keys: ["community"],
    },
  ];
}

function renderPerson() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <PersonAccess userId="u1" isOperator />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  state.zitadel = [];
  state.advanced = false;
  state.unreachable = false;
});

describe("a role whose grant has not been sent", () => {
  it("says so, in the slot that otherwise dates the access", () => {
    state.projects = project([
      { kind: "bundle", bundle_id: "b1", bundle_name: "Ops Admin", description: "", queued: true },
    ]);
    renderPerson();

    expect(screen.getByText("Waiting to be sent")).toBeInTheDocument();
    // An expiry on access that does not exist yet answers a question nobody
    // can ask, so the marker takes the slot rather than sitting beside it.
    expect(screen.queryByText("No expiry")).toBeNull();
  });

  it("says nothing once the grant has been delivered", () => {
    state.zitadel = [{ id: "zg1", projectId: "p-audio", roleKeys: ["community"] }];
    state.projects = project([
      { kind: "bundle", bundle_id: "b1", bundle_name: "Ops Admin", description: "" },
    ]);
    renderPerson();

    expect(screen.queryByText("Waiting to be sent")).toBeNull();
    expect(screen.getByText("No expiry")).toBeInTheDocument();
  });

  // The half that would be wrong in the other direction.
  it("does not mark a role that another source has already delivered", () => {
    state.zitadel = [{ id: "zg1", projectId: "p-audio", roleKeys: ["community"] }];
    state.projects = project([
      { kind: "direct", description: "" },
      { kind: "bundle", bundle_id: "b1", bundle_name: "Ops Admin", description: "", queued: true },
    ]);
    renderPerson();

    // They hold it. One of the two reasons is still owed, and saying the row
    // is waiting would deny access the person actually has.
    expect(screen.queryByText("Waiting to be sent")).toBeNull();
    expect(screen.getByText(/Held 2 ways/)).toBeInTheDocument();
  });
});

describe("the project header when Zitadel has no grant", () => {
  it("says it has not been sent, rather than sending the operator to Drift", () => {
    state.advanced = true;
    state.projects = project([
      { kind: "bundle", bundle_id: "b1", bundle_name: "Ops Admin", description: "", queued: true },
    ]);
    renderPerson();

    expect(screen.getByText(/have not been sent/i)).toBeInTheDocument();
    // Drift will not list it — a bundle's own grant is not drift — so the old
    // pointer ended in an empty screen and an operator concluding the drift
    // report was broken.
    expect(screen.queryByText(/see Drift/i)).toBeNull();
  });

  it("still points at Drift when the grant was sent and is missing anyway", () => {
    state.advanced = true;
    state.projects = project([
      { kind: "bundle", bundle_id: "b1", bundle_name: "Ops Admin", description: "" },
    ]);
    renderPerson();

    expect(screen.getByText(/see Drift/i)).toBeInTheDocument();
  });
});

describe("a role Syndra records that Zitadel does not have", () => {
  // The defect the whole page exists to make visible. Six outbox rows read
  // `applied` after a send that never happened, and every element on this page
  // said "Granted" — only a note at the top of the project said otherwise.
  it("says so on the row, not only at the top of the project", () => {
    state.zitadel = [];
    state.projects = project([
      { kind: "bundle", bundle_id: "b1", bundle_name: "Ops Admin", description: "" },
    ]);
    renderPerson();

    expect(screen.getByText("Not in Zitadel")).toBeInTheDocument();
    expect(screen.queryByText("No expiry")).toBeNull();
  });

  // An outage is not an absence. Reading it as one, on this screen, costs
  // somebody their afternoon.
  it("says it could not check when the read failed, never that it is missing", () => {
    state.unreachable = true;
    state.projects = project([
      { kind: "bundle", bundle_id: "b1", bundle_name: "Ops Admin", description: "" },
    ]);
    renderPerson();

    expect(screen.getByText(/Could not check Zitadel/i)).toBeInTheDocument();
    expect(screen.queryByText("Not in Zitadel")).toBeNull();
  });
});
