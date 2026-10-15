// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { RequestsScreen } from "@/components/requests/RequestsScreen";
import type { AccessRequest } from "@/lib/queries/useRequests";

// Owner, 15 Oct: an operator may approve their own request, and it is said,
// before and after, rather than reading like a second person's decision.

const rows: AccessRequest[] = [];

vi.mock("next/navigation", () => ({
  useSearchParams: () => new URLSearchParams(""),
  useRouter: () => ({ replace: () => {} }),
}));
vi.mock("@/lib/queries/useRequests", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/queries/useRequests")>()),
  useRequestsAdmin: () => ({ data: rows, isLoading: false, error: null, refetch: () => {} }),
  useDecideRequest: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useRehearseBulkDecision: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useApplyBulkDecision: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));
vi.mock("@/lib/queries/useProjects", () => ({ useProjects: () => ({ data: [], isLoading: false, error: null }) }));
vi.mock("@/lib/queries/useRoles", () => ({ useGlobalRoleCatalog: () => ({ data: [], isLoading: false, error: null }) }));
vi.mock("@/components/names", () => ({
  ProjectName: () => null,
  RoleRef: () => null,
  UserName: ({ id }: { id: string }) => <span>{id}</span>,
}));

const base: AccessRequest = {
  id: "r1",
  requester_id: "op1",
  project_id: "pLaser",
  role_key: "trained",
  justification: "Needs the laser.",
  status: "pending",
  created_at: "2026-10-15T06:00:00Z",
};

function renderQueue() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <RequestsScreen isOperator userId="op1" />
    </QueryClientProvider>,
  );
}

describe("an operator's own request", () => {
  it("says before approving that it is their own, and that it is recorded", () => {
    rows.splice(0, rows.length, base, { ...base, id: "r2", requester_id: "u9" });
    renderQueue();
    expect(screen.getAllByText(/Your own request\. Approving it is allowed/)).toHaveLength(1);
  });
});
