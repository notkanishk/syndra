// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { RequestsScreen } from "@/components/requests/RequestsScreen";
import type { AccessRequest } from "@/lib/queries/useRequests";

const decide = vi.fn().mockResolvedValue({ message: "Request resolved" });

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

  it("marks a role that waits in the Asked for cell", () => {
    renderQueue();
    expect(screen.getByRole("img", { name: "Waits for you" })).toBeTruthy();
  });

  it("approving with nothing waiting says they can use it now", async () => {
    renderQueue();
    fireEvent.click(screen.getByRole("button", { name: "Approve" }));
    await waitFor(() => expect(decide).toHaveBeenCalledWith({ id: "r1", status: "approved" }));
    expect(await screen.findByText(/Approved\. They can use it now\./)).toBeTruthy();
  });

  it("approving a role that always waits points at Pending changes", async () => {
    decide.mockResolvedValueOnce({ message: "Request resolved", wait_reason: "role_waits" });
    renderQueue();
    fireEvent.click(screen.getByRole("button", { name: "Approve" }));
    expect(
      await screen.findByText("Approved. This role always waits for you, so it's in Pending changes."),
    ).toBeTruthy();
    expect(screen.getByRole("link", { name: /Waiting for you to send/ })).toBeTruthy();
  });

  it("approving while Zitadel was offline says when, and that it goes when sent", async () => {
    decide.mockResolvedValueOnce({ message: "Request resolved", wait_reason: "halted:zitadel_offline" });
    renderQueue();
    fireEvent.click(screen.getByRole("button", { name: "Approve" }));
    expect(
      await screen.findByText(
        /^Approved\. Not in Zitadel yet: Zitadel didn't answer at \d\d:\d\d\. It goes when you send it\.$/,
      ),
    ).toBeTruthy();
  });

  it("approving after another stop uses that stop's words", async () => {
    decide.mockResolvedValueOnce({ message: "Request resolved", wait_reason: "halted:drain_in_progress" });
    renderQueue();
    fireEvent.click(screen.getByRole("button", { name: "Approve" }));
    expect(
      await screen.findByText(/Approved\. Not in Zitadel yet: Another send was running at \d\d:\d\d\./),
    ).toBeTruthy();
  });

  it("declining carries no Zitadel caveat — nothing was ever sent there", async () => {
    renderQueue();
    fireEvent.click(screen.getByRole("button", { name: "Decline" }));
    await waitFor(() => expect(decide).toHaveBeenCalledWith({ id: "r1", status: "rejected" }));
    expect(screen.getByText("Declined")).toBeTruthy();
    expect(screen.queryByText(/Zitadel/)).toBeNull();
  });
});
