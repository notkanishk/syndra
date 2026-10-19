// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { RequestsScreen } from "@/components/requests/RequestsScreen";
import type { AccessRequest } from "@/lib/queries/useRequests";

// Ask to keep it, from the other two seats (A-04 §8, A-06 §3): the operator
// who decides it, and the member (or operator) who asked.

const state = vi.hoisted(() => ({
  queue: [] as AccessRequest[],
  mine: [] as AccessRequest[],
  grants: [] as Array<{ id: string; project_id: string; role_key: string; expires_at?: string | null }>,
  decide: vi.fn(),
}));

vi.mock("next/navigation", () => ({
  useSearchParams: () => new URLSearchParams(""),
  useRouter: () => ({ replace: () => {} }),
}));
vi.mock("@/lib/queries/useRequests", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/queries/useRequests")>()),
  useRequestsAdmin: () => ({ data: state.queue, isLoading: false, error: null, refetch: () => {} }),
  useRequestsMine: () => ({ data: state.mine, isLoading: false, error: null, refetch: () => {} }),
  useDecideRequest: () => ({ mutateAsync: state.decide, isPending: false }),
  useWithdrawRequest: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useCreateRequest: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useRehearseBulkDecision: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useApplyBulkDecision: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));
vi.mock("@/lib/queries/useTerms", () => ({
  useTerms: () => ({
    data: [
      { name: "Monsoon", start: "2026-08-01", end: "2026-12-31" },
      { name: "Spring", start: "2027-01-01", end: "2027-05-31" },
    ],
  }),
}));
vi.mock("@/lib/queries/useUsers", () => ({
  useUserGrants: () => ({ data: state.grants }),
  useUserAccess: () => ({ data: { projects: [] } }),
}));
vi.mock("@/lib/queries/useMyChanges", () => ({ useMyChanges: () => ({ data: [] }) }));
vi.mock("@/lib/queries/useRoles", () => ({ useGlobalRoleCatalog: () => ({ data: [] }) }));
vi.mock("@/lib/queries/useCatalogUsers", () => ({
  useCatalogProjects: () => ({ data: [], isLoading: false, error: null, refetch: () => {} }),
}));
vi.mock("@/lib/queries/useNameResolver", () => ({
  useNameResolver: () => ({
    resolveUser: (id: string) => ({ value: { display_name: id === "u9" ? "Ada Lovelace" : "Kanishk Singh" } }),
    resolveRole: () => ({ value: { display_name: "Operator" } }),
  }),
}));
vi.mock("@/components/names", () => ({
  ProjectName: () => <span>Laser Lab</span>,
  RoleRef: () => <span>Laser Lab · Operator</span>,
  UserName: ({ id }: { id: string }) => <span>{id === "u9" ? "Ada Lovelace" : "Kanishk Singh"}</span>,
}));

const CURRENT = new Date(2026, 11, 18, 12).toISOString();
const NEW_MONTH = new Date(2027, 0, 17, 12).toISOString();
const NEW_TERM = new Date(2027, 4, 31, 12).toISOString();

function keep(overrides: Partial<AccessRequest> = {}): AccessRequest {
  return {
    id: "r1",
    requester_id: "u9",
    project_id: "pLaser",
    role_key: "operator",
    justification: "Capstone build.",
    duration_days: 30,
    status: "pending",
    created_at: "2026-10-16T06:00:00Z",
    kind: "extend",
    holding: { kind: "until", until: CURRENT },
    current_until: CURRENT,
    until: NEW_MONTH,
    ...overrides,
  };
}

function renderAs(isOperator: boolean) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <RequestsScreen isOperator={isOperator} userId={isOperator ? "op1" : "u9"} />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  state.queue = [];
  state.mine = [];
  state.grants = [];
  state.decide.mockReset();
});

describe("The queue · a request to keep a role", () => {
  it("reads Keep {role}, then how much longer and where it lands, and what Approve does", () => {
    state.queue = [keep()];
    renderAs(true);
    const row = screen.getByText(/^Keep/).closest(".row-divider") as HTMLElement;
    expect(within(row).getByText(/^Keep/)).toHaveTextContent("Keep Laser Lab · Operator");
    expect(within(row).getByText(/^Another month · to 17 Jan( 2027)? instead of 18 Dec$/)).toBeInTheDocument();
    expect(within(row).getByText(/^“Capstone build\.”$/)).toBeInTheDocument();
    expect(
      within(row).getByText(/^Approving moves Ada[’']s end date to 17 Jan( 2027)?\. Nothing else changes\.$/),
    ).toBeInTheDocument();
    expect(within(row).queryByText(/Asks to keep it longer/)).not.toBeInTheDocument();
  });

  it("names a term when the new end is the end of one, and counts days otherwise", () => {
    state.queue = [keep({ until: NEW_TERM, duration_days: 164 }), keep({ id: "r2", until: new Date(2027, 0, 1, 12).toISOString(), duration_days: 14 })];
    renderAs(true);
    expect(screen.getByText(/^To the end of next term · to 31 May( 2027)? instead of 18 Dec$/)).toBeInTheDocument();
    expect(screen.getByText(/^14 days · to 1 Jan( 2027)? instead of 18 Dec$/)).toBeInTheDocument();
  });

  it("says only the length when the dates aren't known", () => {
    state.queue = [keep({ until: undefined, current_until: undefined, holding: { kind: "until", until: CURRENT } })];
    renderAs(true);
    expect(screen.getByText("Another month")).toBeInTheDocument();
    expect(screen.queryByText(/Approving moves/)).not.toBeInTheDocument();
  });

  it("approving reports Extended, and what it now ends instead of", async () => {
    state.queue = [keep()];
    state.decide.mockResolvedValue({ effect: "extended", until: NEW_MONTH });
    renderAs(true);
    fireEvent.click(screen.getByRole("button", { name: /^Approve/ }));
    const seat = await screen.findByRole("status");
    expect(seat).toHaveTextContent(/^ExtendedNow ends 17 Jan( 2027)?, not 18 Dec\.$/);
  });

  it("declining reports Declined, and that it still ends", async () => {
    state.queue = [keep()];
    state.decide.mockResolvedValue({});
    renderAs(true);
    fireEvent.click(screen.getByRole("button", { name: "Decline" }));
    const seat = await screen.findByRole("status");
    expect(seat).toHaveTextContent("DeclinedNothing was sent · it still ends 18 Dec");
  });
});

describe("A member's request to keep a role", () => {
  it("is Waiting, and says it still ends until it's decided", () => {
    state.mine = [keep({ requester_id: "u9" })];
    renderAs(false);
    expect(screen.getByText(/^Keep/)).toHaveTextContent("Keep Laser Lab · Operator");
    expect(screen.getByText("Waiting")).toBeInTheDocument();
    expect(screen.getByText(/^Until it[’']s decided, it still ends 18 Dec\.$/)).toBeInTheDocument();
  });

  it("is Kept once approved, with what it now ends", () => {
    state.mine = [keep({ status: "approved", resolved_at: "2026-10-17T09:00:00Z", holding: undefined, current_until: undefined, until: undefined })];
    state.grants = [{ id: "g1", project_id: "pLaser", role_key: "operator", expires_at: NEW_MONTH }];
    renderAs(false);
    expect(screen.getByText("Kept")).toBeInTheDocument();
    expect(screen.getByText(/^It now ends 17 Jan( 2027)?\.$/)).toBeInTheDocument();
    expect(screen.queryByText("Approved")).not.toBeInTheDocument();
  });

  it("is Not approved when declined, and still ends when it did", () => {
    state.mine = [keep({ status: "rejected", holding: undefined, current_until: undefined, until: undefined })];
    state.grants = [{ id: "g1", project_id: "pLaser", role_key: "operator", expires_at: CURRENT }];
    renderAs(false);
    expect(screen.getByText("Not approved")).toBeInTheDocument();
    expect(screen.getByText("It still ends 18 Dec.")).toBeInTheDocument();
  });

  it("says only what is known when it can't read the end", () => {
    state.mine = [keep({ status: "approved", holding: undefined, current_until: undefined, until: undefined })];
    renderAs(false);
    expect(screen.getByText("Kept")).toBeInTheDocument();
    expect(screen.queryByText(/It now ends/)).not.toBeInTheDocument();
  });
});

describe("Your requests, for an operator", () => {
  const own = keep({ requester_id: "op1", status: "pending" });

  it("sits under the queue once they have asked for anything, in the member's own rows", () => {
    state.queue = [];
    state.mine = [own];
    renderAs(true);
    const card = screen.getByText("Your requests").closest("div")!.parentElement as HTMLElement;
    expect(within(card).getByText("Waiting")).toBeInTheDocument();
    expect(within(card).getByRole("button", { name: "Withdraw" })).toBeInTheDocument();
    // After the queue.
    const heading = screen.getByRole("heading", { name: "Requests" });
    expect(heading.compareDocumentPosition(screen.getByText("Your requests")) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  });

  it("is absent until they have asked", () => {
    renderAs(true);
    expect(screen.queryByText("Your requests")).not.toBeInTheDocument();
  });
});
