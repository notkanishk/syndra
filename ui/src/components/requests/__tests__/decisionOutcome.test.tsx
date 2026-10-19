// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { RequestsScreen } from "@/components/requests/RequestsScreen";
import { WouldCancelQueued } from "@/lib/api-client";
import type { QueuedChange } from "@/lib/cancel-confirm";
import type { AccessRequest } from "@/lib/queries/useRequests";

const decide = vi.fn().mockResolvedValue({ message: "Request resolved" });

vi.mock("next/navigation", () => ({
  useSearchParams: () => new URLSearchParams(""),
  useRouter: () => ({ replace: () => {} }),
}));

vi.mock("@/lib/queries/useRequests", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/queries/useRequests")>()),
  useRequestsAdmin: () => ({
    data: queue,
    isLoading: false,
    error: null,
    refetch: () => {},
  }),
  useDecideRequest: () => ({ mutateAsync: decide, isPending: false }),
  useRehearseBulkDecision: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useApplyBulkDecision: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));

vi.mock("@/lib/queries/useRoles", () => ({
  useGlobalRoleCatalog: () => ({
    data: [
      {
        project_id: "pLaser",
        role_key: "trained",
        project_name: "Laser",
        display_name: "Trained",
        assigned_user_count: 2,
        waits_for_send: true,
      },
    ],
  }),
}));

vi.mock("@/lib/queries/useCatalogUsers", () => ({
  useCatalogProjects: () => ({ data: [], isLoading: false, error: null, refetch: () => {} }),
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

// One read, the same array every render, as the real query gives: the queue's
// leaving-rows pass treats a new array as a new read.
const queue = [request()];

function renderQueue() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <RequestsScreen isOperator userId="op1" />
    </QueryClientProvider>,
  );
}

/** The Decision seat, once a decision is in it: the one `role="status"` in the row. */
async function decisionSeat() {
  return await screen.findByRole("status");
}

describe("a decided request names its person and its next step", () => {
  it("renders the requester's name, not their id", () => {
    renderQueue();
    expect(screen.getByText("Ada Lovelace")).toBeTruthy();
    expect(screen.queryByText("u1")).toBeNull();
  });

  it("marks a role that waits in the Asked for cell", () => {
    renderQueue();
    expect(screen.getByRole("img", { name: "Waits for you" })).toBeTruthy();
  });

  // #96: the row reports in its own Decision seat — dot, word, what happened —
  // in place of the buttons, never a pill reading "Applied".
  it("approving with nothing waiting says Approved, sent and not yet seen", async () => {
    renderQueue();
    fireEvent.click(screen.getByRole("button", { name: "Approve" }));
    await waitFor(() => expect(decide).toHaveBeenCalledWith({ id: "r1", status: "approved" }));
    const seat = await decisionSeat();
    expect(seat.textContent).toBe("ApprovedSent · not yet seen in Zitadel");
    expect(screen.queryByRole("button", { name: "Approve" })).toBeNull();
    expect(screen.queryByText("Applied")).toBeNull();
  });

  it("approving a role that always waits says why and points at Pending changes", async () => {
    decide.mockResolvedValueOnce({ message: "Request resolved", wait_reason: "role_waits" });
    renderQueue();
    fireEvent.click(screen.getByRole("button", { name: "Approve" }));
    const seat = await decisionSeat();
    expect(seat.textContent).toContain("Approved");
    expect(seat.textContent).toContain("Trained always waits for you");
    expect(screen.getByRole("link", { name: /Waiting for you to send/ })).toBeTruthy();
  });

  it("approving while Zitadel was offline says so, in the board's words", async () => {
    decide.mockResolvedValueOnce({ message: "Request resolved", wait_reason: "halted:zitadel_offline" });
    renderQueue();
    fireEvent.click(screen.getByRole("button", { name: "Approve" }));
    const seat = await decisionSeat();
    expect(seat.textContent).toContain("Zitadel didn't answer when it was approved");
    expect(screen.getByRole("link", { name: /Waiting for you to send/ })).toBeTruthy();
  });

  it("approving after another stop uses that stop's words", async () => {
    decide.mockResolvedValueOnce({ message: "Request resolved", wait_reason: "halted:drain_in_progress" });
    renderQueue();
    fireEvent.click(screen.getByRole("button", { name: "Approve" }));
    const seat = await decisionSeat();
    expect(seat.textContent).toMatch(/Another send was running at \d\d:\d\d/);
  });

  it("declining says Nothing was sent — it never reached Zitadel", async () => {
    renderQueue();
    fireEvent.click(screen.getByRole("button", { name: "Decline" }));
    await waitFor(() => expect(decide).toHaveBeenCalledWith({ id: "r1", status: "rejected" }));
    const seat = await decisionSeat();
    expect(seat.textContent).toBe("DeclinedNothing was sent");
    expect(seat.textContent).not.toMatch(/Zitadel/);
  });

  it("a failed decision puts the buttons back and says why under them", async () => {
    decide.mockRejectedValueOnce(new Error("boom"));
    renderQueue();
    fireEvent.click(screen.getByRole("button", { name: "Approve" }));
    expect(await screen.findByRole("alert")).toBeTruthy();
    expect(screen.getByRole("button", { name: "Approve" })).toBeTruthy();
  });
});

describe("approving a request that would cancel a change still waiting", () => {
  const waiting: QueuedChange = {
    id: "00000000-0000-0000-0000-000000000042",
    op_type: "revoke",
    user_id: "u1",
    project_id: "pLaser",
    role_keys: ["trained"],
    source: "direct",
    initiated_by: "op-2",
    initiated_by_name: "Sam Iyer",
    reason: "left the course",
    queued_at: new Date(Date.now() - 2 * 3_600_000).toISOString(),
  };
  const refusal = () => new WouldCancelQueued([waiting], "POST", "/requests/r1/decision");

  it("opens a small sheet holding only the step; Keep it leaves the request waiting in its seat", async () => {
    decide.mockClear();
    decide.mockRejectedValueOnce(refusal());
    renderQueue();
    fireEvent.click(screen.getByRole("button", { name: "Approve" }));

    const dialog = await screen.findByRole("dialog");
    expect(dialog.textContent).toContain("This cancels a removal that's waiting");
    expect(screen.getByRole("button", { name: "Cancel it and approve" })).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Keep it" }));

    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(decide).toHaveBeenCalledTimes(1);
    expect(screen.getByRole("button", { name: "Approve" })).toBeTruthy();
    expect(screen.queryByRole("status")).toBeNull();
  });

  it("going ahead approves, and the seat says Approved", async () => {
    decide.mockClear();
    decide.mockRejectedValueOnce(refusal()).mockResolvedValueOnce({ message: "Request resolved" });
    renderQueue();
    fireEvent.click(screen.getByRole("button", { name: "Approve" }));
    fireEvent.click(await screen.findByRole("button", { name: "Cancel it and approve" }));

    const seat = await decisionSeat();
    expect(seat.textContent).toContain("Approved");
    expect(decide).toHaveBeenCalledTimes(2);
  });
});
