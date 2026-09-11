// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { RequestsScreen } from "@/components/requests/RequestsScreen";
import type { AccessRequest } from "@/lib/queries/useRequests";

const decide = vi.fn().mockResolvedValue(undefined);

vi.mock("next/navigation", () => ({
  useSearchParams: () => new URLSearchParams(""),
  useRouter: () => ({ replace: () => {} }),
}));

vi.mock("@/lib/queries/useRequests", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/queries/useRequests")>()),
  useRequestsAdmin: () => ({
    data: [request()],
    isLoading: false,
    error: null,
    refetch: () => {},
  }),
  useDecideRequest: () => ({ mutateAsync: decide, isPending: false }),
  useRehearseBulkDecision: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useApplyBulkDecision: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));

vi.mock("@/lib/queries/useProjects", () => ({
  useProjects: () => ({ data: [], isLoading: false, error: null }),
}));

vi.mock("@/lib/queries/useRoles", () => ({
  useGlobalRoleCatalog: () => ({ data: [], isLoading: false, error: null }),
}));

// The real UserName resolves an id to a name via useNameResolver; stubbing it
// here to a fixed name is enough to prove the row renders the name rather
// than the raw id it's given.
vi.mock("@/components/names", () => ({
  ProjectName: () => null,
  RoleRef: () => null,
  UserName: ({ id }: { id: string }) => <span>{id === "u1" ? "Ada Lovelace" : id}</span>,
}));

function request(): AccessRequest {
  return {
    id: "r1",
    requester_id: "u1",
    project_id: "pLaser",
    role_key: "trained",
    justification: "Needs the laser for a build.",
    status: "pending",
    created_at: "2026-07-22T06:00:00Z",
  };
}

function renderQueue() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <RequestsScreen isOperator userId="op1" />
    </QueryClientProvider>,
  );
}

describe("a decided request names its person and its next step", () => {
  it("renders the requester's name, not their id", () => {
    renderQueue();
    expect(screen.getByText("Ada Lovelace")).toBeTruthy();
    expect(screen.queryByText("u1")).toBeNull();
  });

  it("approving says Zitadel hasn't confirmed it yet — the response never says so", async () => {
    renderQueue();
    fireEvent.click(screen.getByRole("button", { name: "Approve" }));
    await waitFor(() => expect(decide).toHaveBeenCalledWith({ id: "r1", status: "approved" }));
    expect(await screen.findByText(/Approved\. Sent; Zitadel has not been read back yet\./)).toBeTruthy();
  });

  it("declining carries no Zitadel caveat — nothing was ever sent there", async () => {
    renderQueue();
    fireEvent.click(screen.getByRole("button", { name: "Decline" }));
    await waitFor(() => expect(decide).toHaveBeenCalledWith({ id: "r1", status: "rejected" }));
    expect(screen.getByText("Declined")).toBeTruthy();
    expect(screen.queryByText(/Zitadel/)).toBeNull();
  });
});
