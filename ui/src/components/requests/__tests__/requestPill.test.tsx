// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { RequestsScreen } from "@/components/requests/RequestsScreen";
import type { AccessRequest } from "@/lib/queries/useRequests";

// A-20 P19: the status pill is remembered across visits, so anything but Open is also said in words.

const queue = vi.hoisted(() => ({ data: [] as AccessRequest[] }));
const NAMES = vi.hoisted(() => ({ u1: "Ada Lovelace", u2: "Grace Hopper", u3: "Alan Turing" }) as Record<string, string>);
const ROLES = vi.hoisted(() => ({ operator: "Laser operator", trained: "Bandsaw trained" }) as Record<string, string>);

vi.mock("next/navigation", () => ({
  useSearchParams: () => new URLSearchParams(""),
  useRouter: () => ({ replace: () => {} }),
}));
vi.mock("@/lib/queries/useRequests", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/queries/useRequests")>()),
  useRequestsAdmin: () => ({ ...queue, isLoading: false, error: null, refetch: () => {} }),
  useRequestsMine: () => ({ data: [], isLoading: false, error: null, refetch: () => {} }),
  useDecideRequest: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useRehearseBulkDecision: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useApplyBulkDecision: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));
vi.mock("@/lib/queries/useCatalogUsers", () => ({
  useCatalogProjects: () => ({ data: [], isLoading: false, error: null, refetch: () => {} }),
}));
// One object for every render, as the real provider's memoised value is.
const resolver = vi.hoisted(() => ({
  resolveUser: (id: string) => ({ value: { display_name: NAMES[id], email: `${id}@example.edu` } }),
  resolveProject: () => ({ value: { name: "Laser Lab" } }),
  resolveRole: (_p: string, key: string) => ({ value: { display_name: ROLES[key] } }),
}));
vi.mock("@/lib/queries/useNameResolver", () => ({ useNameResolver: () => resolver }));
vi.mock("@/components/names", () => ({
  UserAvatar: () => null,
  ProjectName: () => <span>Laser Lab</span>,
  RoleRef: ({ roleKey }: { roleKey: string }) => <span>{ROLES[roleKey]}</span>,
  UserName: ({ id }: { id: string }) => <a href={`/users/${id}`}>{NAMES[id]}</a>,
}));

function ask(id: string, who: string, role: string, status = "pending"): AccessRequest {
  return {
    id,
    requester_id: who,
    project_id: "pLaser",
    role_key: role,
    justification: "A build.",
    status,
    created_at: "2026-10-16T06:00:00Z",
  };
}

function renderQueue(rows: AccessRequest[]) {
  queue.data = rows;
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <RequestsScreen isOperator userId="op1" />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  window.localStorage.clear();
});

describe("a remembered status is never silent", () => {
  it("says nothing extra on Open, the default", () => {
    renderQueue([ask("r1", "u1", "operator")]);
    expect(screen.queryByRole("button", { name: "Show open requests again" })).toBeNull();
  });

  it("says Declined in words when the pill was left there, and the chip puts it back to Open", () => {
    window.localStorage.setItem("syndra:view:requests", JSON.stringify({ pill: "rejected" }));
    renderQueue([ask("r1", "u1", "operator", "rejected")]);
    expect(screen.getByText("Showing declined requests")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Show open requests again" }));
    expect(screen.getByRole("radio", { name: "Open" })).toBeChecked();
  });
});
