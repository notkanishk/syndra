// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { RequestsScreen } from "@/components/requests/RequestsScreen";
import type { AccessRequest } from "@/lib/queries/useRequests";

const queue = vi.hoisted(() => ({ data: [] as AccessRequest[] }));

vi.mock("next/navigation", () => ({
  useSearchParams: () => new URLSearchParams(""),
  useRouter: () => ({ replace: () => {} }),
}));

vi.mock("@/lib/queries/useRequests", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/queries/useRequests")>()),
  useRequestsAdmin: () => ({ ...queue, isLoading: false, error: null, refetch: () => {} }),
  useDecideRequest: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useRehearseBulkDecision: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useApplyBulkDecision: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));

vi.mock("@/lib/queries/useProjects", () => ({
  useProjects: () => ({ data: [], isLoading: false, error: null }),
}));

vi.mock("@/lib/queries/useRoles", () => ({
  useGlobalRoleCatalog: () => ({ data: [], isLoading: false, error: null }),
}));

vi.mock("@/components/names", () => ({
  ProjectName: () => null,
  RoleRef: () => null,
  UserName: () => null,
}));

function request(at: number): AccessRequest {
  return {
    id: `r${at}`,
    requester_id: `u${at}`,
    project_id: "pLaser",
    role_key: "trained",
    justification: "Needs the laser for a build.",
    status: "pending",
    created_at: "2026-07-22T06:00:00Z",
  };
}

function renderQueue(count: number) {
  queue.data = Array.from({ length: count }, (_, at) => request(at));
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <RequestsScreen isOperator userId="op1" />
    </QueryClientProvider>,
  );
  fireEvent.click(screen.getByRole("button", { name: "Select" }));
}

beforeEach(() => {
  queue.data = [];
});

/**
 * `/requests/bulk-decision` refuses past 500. The bar has been able to say so
 * since the selection work and this queue was not telling it — so an operator
 * select-alled, tapped, and met the refusal afterwards, which is the one
 * sequence the bar exists to prevent.
 */
describe("the request queue states its ceiling before the tap", () => {
  it("names the limit over 500", () => {
    renderQueue(501);
    fireEvent.click(screen.getByRole("checkbox", { name: /Select these 501 requests/ }));

    expect(screen.getByText(/500 is the most that can run at once/)).toBeTruthy();
  });

  it("leaves a selection inside the ceiling alone", () => {
    renderQueue(3);
    fireEvent.click(screen.getByRole("checkbox", { name: /Select these 3 requests/ }));

    expect(screen.queryByText(/is the most that can run at once/)).toBeNull();
  });
});
