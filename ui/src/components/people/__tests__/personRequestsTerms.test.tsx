// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { PersonRequests } from "@/components/people/PersonRequests";
import type { AccessRequest } from "@/lib/queries/useRequests";

// A person's Requests tab reads a request to keep a role the way the queue
// does: "To the end of next term" needs the term setting to know the end is one.

const CURRENT = new Date(2026, 11, 18, 12).toISOString();
const request: AccessRequest = {
  id: "r1",
  requester_id: "u9",
  project_id: "pLaser",
  role_key: "operator",
  justification: "Capstone build.",
  duration_days: 164,
  status: "pending",
  created_at: "2026-10-16T06:00:00Z",
  kind: "extend",
  holding: { kind: "until", until: CURRENT },
  current_until: CURRENT,
  until: new Date(2027, 4, 31, 12).toISOString(),
};

vi.mock("@/lib/queries/useRequests", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/queries/useRequests")>()),
  useRequestsAdmin: () => ({ data: [request], isLoading: false, error: null, refetch: () => {} }),
  useDecideRequest: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));
vi.mock("@/lib/queries/useTerms", () => ({
  useTerms: () => ({ data: [{ name: "Spring", start: "2027-01-01", end: "2027-05-31" }] }),
}));
vi.mock("@/components/names", () => ({ RoleRef: () => <span>Laser Lab · Operator</span> }));

describe("PersonRequests · a request to keep a role", () => {
  it("says To the end of next term when the new end is a term's last day", () => {
    render(
      <QueryClientProvider client={new QueryClient()}>
        <PersonRequests userId="u9" name="Ada" isOperator />
      </QueryClientProvider>,
    );
    expect(screen.getByText(/^To the end of next term · to 31 May 2027 instead of 18 Dec$/)).toBeInTheDocument();
  });
});
