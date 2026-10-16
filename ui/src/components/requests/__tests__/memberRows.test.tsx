// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { RequestsScreen } from "@/components/requests/RequestsScreen";
import type { AccessRequest } from "@/lib/queries/useRequests";

// E8 (route-requests, "Your requests"): one status seat per row — dot, word,
// note — never a pill plus "Applied"; Withdraw is outline and last; the row
// names the role, not only the place.

const state = vi.hoisted(() => ({
  data: [] as AccessRequest[],
  withdraw: vi.fn(),
}));

vi.mock("next/navigation", () => ({
  useSearchParams: () => new URLSearchParams(""),
  useRouter: () => ({ replace: () => {} }),
}));
vi.mock("@/lib/queries/useRequests", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/queries/useRequests")>()),
  useRequestsMine: () => ({ data: state.data, isLoading: false, error: null, refetch: () => {} }),
  useWithdrawRequest: () => ({ mutateAsync: state.withdraw, isPending: false }),
  useCreateRequest: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));
vi.mock("@/lib/queries/useCatalogUsers", () => ({
  useCatalogProjects: () => ({ data: [], isLoading: false, error: null, refetch: () => {} }),
}));
vi.mock("@/components/names", () => ({
  ProjectName: ({ id }: { id: string }) => <span>{id === "pLaser" ? "Laser Lab" : id}</span>,
  RoleRef: () => null,
  UserName: ({ id }: { id: string }) => <span>{id}</span>,
}));

function request(overrides: Partial<AccessRequest> = {}): AccessRequest {
  return {
    id: "r1",
    requester_id: "u1",
    project_id: "pLaser",
    role_key: "operator",
    justification: "Capstone build.",
    duration_days: 30,
    status: "pending",
    created_at: "2026-10-16T06:00:00Z",
    ...overrides,
  };
}

function renderMine(rows: AccessRequest[]) {
  state.data = rows;
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <RequestsScreen isOperator={false} userId="u1" />
    </QueryClientProvider>,
  );
}

describe("a member's request rows", () => {
  it("names the role, not only the place", () => {
    renderMine([request()]);
    expect(screen.getByText(/Laser Lab/).closest("div")!.textContent).toBe("Laser Lab · Operator");
  });

  it("says each status in the member's own word, with no pill", () => {
    renderMine([
      request(),
      request({ id: "r2", status: "rejected" }),
      request({ id: "r3", status: "approved" }),
    ]);
    expect(screen.getByText("Waiting")).toBeTruthy();
    expect(screen.getByText("Not approved")).toBeTruthy();
    expect(screen.getByText("Approved")).toBeTruthy();
    expect(screen.queryByText("Declined")).toBeNull();
  });

  // A-04 §13.1: a request closed because it was given another way needs the
  // note, or it looks like a lost one.
  it("says why a request closed because staff gave it to you directly", () => {
    renderMine([request({ status: "fulfilled", resolved_at: "2026-01-15T09:00:00Z" })]);
    expect(screen.getByText("You have this")).toBeTruthy();
    expect(
      screen.getByText(
        /^Makerspace staff gave it to you directly on 15 Jan( 2026)?, so there was nothing left to decide\.$/,
      ),
    ).toBeTruthy();
  });

  it("leaves the date out when the request carries none", () => {
    renderMine([request({ status: "fulfilled" })]);
    expect(
      screen.getByText("Makerspace staff gave it to you directly, so there was nothing left to decide."),
    ).toBeTruthy();
  });

  it("gives a waiting row the length only, from when it's approved", () => {
    renderMine([request()]);
    expect(screen.getByText("For a month, from when it's approved.")).toBeTruthy();
    expect(screen.queryByText(/until/)).toBeNull();
  });

  it("puts Withdraw last in the row, outline, only on a waiting request", () => {
    renderMine([request(), request({ id: "r2", status: "approved" })]);
    const withdraw = screen.getAllByRole("button", { name: "Withdraw" });
    expect(withdraw).toHaveLength(1);
    const row = screen.getByText("Waiting").closest(".row-divider")!;
    expect(row.lastElementChild!.contains(withdraw[0])).toBe(true);
  });

  // #51: a withdrawal is never Declined, and never "Applied".
  it("reports a withdrawal in its seat: You withdrew this, and who won't be asked", async () => {
    state.withdraw.mockResolvedValueOnce(undefined);
    renderMine([request()]);
    fireEvent.click(screen.getByRole("button", { name: "Withdraw" }));

    const seat = await screen.findByRole("status");
    expect(seat.textContent).toBe("You withdrew thisNobody will be asked to decide it");
    expect(screen.queryByText("Applied")).toBeNull();
    expect(screen.queryByRole("button", { name: "Withdraw" })).toBeNull();
  });
});
