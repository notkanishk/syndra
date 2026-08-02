// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { PersonActivity } from "@/components/people/PersonActivity";
import { PersonRequests } from "@/components/people/PersonRequests";
import type { AuditEntry } from "@/lib/queries/useAudit";
import type { AccessRequest } from "@/lib/queries/useRequests";

const audit = vi.hoisted(() => ({ data: [] as AuditEntry[], filter: null as unknown }));
const requests = vi.hoisted(() => ({ data: [] as AccessRequest[] }));

vi.mock("@/lib/queries/useAudit", () => ({
  useAuditEntries: (filter: unknown) => {
    audit.filter = filter;
    return { data: audit.data, isLoading: false, error: null, refetch: () => {} };
  },
}));

vi.mock("@/lib/queries/useRequests", () => ({
  useRequestsAdmin: () => ({ data: requests.data, isLoading: false, error: null, refetch: () => {} }),
  useDecideRequest: () => ({ isPending: false, mutateAsync: vi.fn() }),
}));

vi.mock("@/components/names", () => ({
  UserName: ({ id, fallback }: { id: string; fallback?: React.ReactNode }) => (
    <span>{id === "u1" ? "Ada Lovelace" : id === "op" ? "Sam Patel" : (fallback ?? id)}</span>
  ),
  ProjectName: ({ id }: { id: string }) => <span>{id === "pLaser" ? "Laser Lab" : id}</span>,
  // Mirrors the real component's shape: project name and role key are separate
  // elements, so a test can assert on either half of the pair.
  RoleRef: ({ projectId, roleKey }: { projectId: string; roleKey: string }) => (
    <span>
      <span>{projectId === "pLaser" ? "Laser Lab" : projectId}</span> / <span>{roleKey}</span>
    </span>
  ),
}));

function entry(overrides: Partial<AuditEntry> = {}): AuditEntry {
  return {
    id: "a1",
    actor_id: "op",
    target_id: "u1",
    action: "direct_grant.upserted",
    resource_id: "",
    created_at: "2026-07-30T10:15:00Z",
    ...overrides,
  };
}

function request(overrides: Partial<AccessRequest> = {}): AccessRequest {
  return {
    id: "r1",
    requester_id: "u1",
    project_id: "pLaser",
    role_key: "trained",
    justification: "Finishing my capstone",
    status: "pending",
    created_at: "2026-07-30T10:15:00Z",
    ...overrides,
  };
}

function renderIn(node: React.ReactElement) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(<QueryClientProvider client={client}>{node}</QueryClientProvider>);
}

beforeEach(() => {
  audit.data = [];
  audit.filter = null;
  requests.data = [];
});

describe("PersonActivity", () => {
  it("filters at the source rather than sifting the global tail", () => {
    renderIn(<PersonActivity userId="u1" name="Ada" />);
    // Client-filtering 200 global rows would render an empty feed for anyone
    // whose last action fell outside them — "nothing ever happened" is a very
    // different claim from "nothing in the last 200 events".
    expect(audit.filter).toMatchObject({ userId: "u1" });
  });

  it("says who acted when the person was acted upon", () => {
    audit.data = [entry({ actor_id: "op", target_id: "u1" })];
    renderIn(<PersonActivity userId="u1" name="Ada" />);
    expect(screen.getByText("Granted direct access")).toBeInTheDocument();
    // The person did not do this — the row must not imply they did.
    expect(screen.getByText(/by/)).toBeInTheDocument();
    expect(screen.getByText("Sam Patel")).toBeInTheDocument();
  });

  it("says who was affected when the person acted", () => {
    audit.data = [entry({ actor_id: "u1", target_id: "u2" })];
    renderIn(<PersonActivity userId="u1" name="Ada" />);
    expect(screen.getByText(/to/)).toBeInTheDocument();
  });

  it("marks a destructive verb and leaves the rest of the row uncoloured", () => {
    audit.data = [entry({ action: "direct_grant.revoked" })];
    renderIn(<PersonActivity userId="u1" name="Ada" />);
    expect(screen.getByText("Revoked direct access").className).toContain("text-danger-text");
  });

  it("admits when the feed is capped instead of implying it is complete", () => {
    audit.data = Array.from({ length: 200 }, (_, index) => entry({ id: `a${index}` }));
    renderIn(<PersonActivity userId="u1" name="Ada" />);
    expect(screen.getByText(/not their whole history/)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Load more" })).not.toBeInTheDocument();
  });

  it("names the person in its empty state", () => {
    renderIn(<PersonActivity userId="u1" name="Ada" />);
    expect(screen.getByText(/Nothing recorded for Ada yet/)).toBeInTheDocument();
  });
});

describe("PersonRequests", () => {
  it("shows only this person's requests", () => {
    requests.data = [request(), request({ id: "r2", requester_id: "u9", role_key: "operator" })];
    renderIn(<PersonRequests userId="u1" name="Ada" isOperator />);
    expect(screen.getByText("trained")).toBeInTheDocument();
    expect(screen.queryByText("operator")).not.toBeInTheDocument();
  });

  it("includes decided requests, which the shared queue does not carry", () => {
    // This is the whole reason the tab exists rather than linking to /requests:
    // the queue holds pending work, so it cannot answer "what did we decide?".
    requests.data = [request({ status: "rejected", review_note: "Not trained yet" })];
    renderIn(<PersonRequests userId="u1" name="Ada" isOperator />);
    expect(screen.getByText("Declined")).toBeInTheDocument();
    expect(screen.getByText("Not trained yet")).toBeInTheDocument();
  });

  it("lets an operator decide without leaving the person", () => {
    requests.data = [request()];
    renderIn(<PersonRequests userId="u1" name="Ada" isOperator />);
    expect(screen.getByRole("button", { name: "Approve" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Deny" })).toBeInTheDocument();
  });

  it("shows a member the outcome but never the decision buttons", () => {
    requests.data = [request()];
    renderIn(<PersonRequests userId="u1" name="Ada" isOperator={false} />);
    expect(screen.queryByRole("button", { name: "Approve" })).not.toBeInTheDocument();
    expect(screen.getByText("Waiting on a decision")).toBeInTheDocument();
  });

  it("names the person in its empty state", () => {
    renderIn(<PersonRequests userId="u1" name="Ada" isOperator />);
    expect(screen.getByText(/Ada hasn’t asked for anything/)).toBeInTheDocument();
  });
});
