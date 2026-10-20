// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { RequestsScreen } from "@/components/requests/RequestsScreen";
import type { AccessRequest } from "@/lib/queries/useRequests";

// route-requests: the queue's search seat and its Select (RULES 9b, N10). The
// search narrows what is drawn AND what can be selected, so a bulk decision can
// never reach a row the operator cannot see.

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
  ProjectName: () => <span>Laser Lab</span>,
  RoleRef: ({ roleKey }: { roleKey: string }) => <span>{ROLES[roleKey]}</span>,
  UserName: ({ id }: { id: string }) => <span>{NAMES[id]}</span>,
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

const box = () => screen.getByRole("textbox", { name: /Search requests/ }) as HTMLInputElement;
const type = (value: string) => fireEvent.change(box(), { target: { value } });

beforeEach(() => {
  queue.data = [];
});

describe("the queue's search", () => {
  const three = () => [ask("r1", "u1", "operator"), ask("r2", "u2", "trained"), ask("r3", "u3", "trained")];

  it("finds a request by who asked", () => {
    renderQueue(three());
    type("grace");
    expect(screen.getByText("Grace Hopper")).toBeInTheDocument();
    expect(screen.queryByText("Ada Lovelace")).not.toBeInTheDocument();
    expect(screen.queryByText("Alan Turing")).not.toBeInTheDocument();
  });

  it("finds a request by the role asked for", () => {
    renderQueue(three());
    type("bandsaw");
    expect(screen.getAllByRole("button", { name: /^Approve/ })).toHaveLength(2);
    expect(screen.queryByText("Ada Lovelace")).not.toBeInTheDocument();
  });

  it("says so when nothing matches, and clearing it brings the queue back", () => {
    renderQueue(three());
    type("zzz");
    expect(screen.getByText("No requests match that search.")).toBeInTheDocument();
    type("");
    expect(screen.getByText("Ada Lovelace")).toBeInTheDocument();
  });

  it("keeps a bulk decision to the rows the search shows", () => {
    renderQueue(three());
    type("bandsaw");
    fireEvent.click(screen.getByRole("button", { name: "Select" }));
    fireEvent.click(screen.getByRole("checkbox", { name: /Select these 2 requests/ }));
    expect(document.body.textContent).toMatch(/2 requests selected/);
  });
});

describe("Select", () => {
  it("enters and leaves selection mode, round seats only while it is on", () => {
    renderQueue([ask("r1", "u1", "operator")]);
    expect(screen.queryByRole("checkbox")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Select" }));
    expect(screen.getByRole("checkbox", { name: "Select this request" })).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Done selecting" }));
    expect(screen.queryByRole("checkbox")).not.toBeInTheDocument();
  });

  it("is absent when nothing is open to select, though the search stays", () => {
    renderQueue([]);
    expect(screen.queryByRole("button", { name: "Select" })).not.toBeInTheDocument();
    expect(box()).toBeInTheDocument();
  });
});
