// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { trueSeat } from "@/test-utils/seat";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { RequestsScreen } from "@/components/requests/RequestsScreen";
import { ApiError } from "@/lib/api-client";
import { actPlan, cancelling, queuedChange } from "@/test-utils/plan";
import type { ActPlan } from "@/lib/usePlanConsent";
import { motionInTests } from "@/test-utils/motion";
import type { AccessRequest } from "@/lib/queries/useRequests";

const decide = vi.fn().mockResolvedValue({ message: "Request resolved" });

// What the planner answers about approving (A-21 §3); by default nothing follows, so Approve is not held.
const wire = vi.hoisted(() => ({ plan: undefined as unknown as ActPlan, calls: [] as string[] }));
vi.mock("@/lib/api-client", async (original) => ({
  ...(await original<typeof import("@/lib/api-client")>()),
  request: async (path: string) => {
    if (path.includes("/plan?")) {
      wire.calls.push(path);
      return wire.plan;
    }
    throw new Error(`unmocked request ${path}`);
  },
}));
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
  UserAvatar: () => null,
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

beforeEach(() => {
  wire.plan = actPlan();
  wire.calls = [];
});

// Approve is held until the plan is read; then the press goes.
async function approve() {
  await waitFor(() => expect(screen.getByRole("button", { name: "Approve" })).toBeEnabled());
  fireEvent.click(screen.getByRole("button", { name: "Approve" }));
}

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
  return await trueSeat();
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
    await approve();
    await waitFor(() => expect(decide).toHaveBeenCalledWith({ id: "r1", status: "approved", fingerprint: "fp-1", cancel_ids: [] }));
    const seat = await decisionSeat();
    expect(seat.textContent).toBe("ApprovedSent · not yet seen in ZitadelNothing for you to do.");
    expect(screen.queryByRole("button", { name: "Approve" })).toBeNull();
    expect(screen.queryByText("Applied")).toBeNull();
  });

  // A-19 P2: the seat forecasts in the plan's tense until the server answers.
  it("forecasts Will be approved in the seat while the write is in flight, then says the true word", async () => {
    let answer!: (v: unknown) => void;
    decide.mockReturnValueOnce(new Promise((resolve) => (answer = resolve)));
    renderQueue();
    await approve();

    const seat = await screen.findByRole("status");
    expect(seat.textContent).toBe("Will be approved");
    expect(seat.querySelector(".breathe")).not.toBeNull();
    expect(screen.queryByRole("button", { name: "Approve" })).toBeNull();

    answer({ message: "Request resolved" });
    expect((await trueSeat()).textContent).toBe("ApprovedSent · not yet seen in ZitadelNothing for you to do.");
  });

  it("a refused decision returns the buttons beside the honest error, with no forecast left", async () => {
    decide.mockRejectedValueOnce(new Error("The role no longer exists."));
    renderQueue();
    await approve();

    expect((await screen.findByRole("alert")).textContent).toContain("The role no longer exists.");
    expect(screen.queryByText("Will be approved")).toBeNull();
    expect(screen.getByRole("button", { name: "Approve" })).toBeTruthy();
  });

  // Motion v2 S4: the buttons and the decision share one seat; they crossfade, and
  // the dot blooms because the decision went well, because of you, on screen.
  it("approving fades the decision in, in the seat the buttons held, and blooms once", async () => {
    motionInTests();
    renderQueue();
    await approve();
    const seat = await decisionSeat();
    expect(seat.closest(".tint-in")).not.toBeNull();
    expect(seat.querySelector(".bloom")).not.toBeNull();
    motionInTests(false);
  });

  it("approving a role that always waits says why and points at Pending changes", async () => {
    decide.mockResolvedValueOnce({ message: "Request resolved", wait_reason: "role_waits" });
    renderQueue();
    await approve();
    const seat = await decisionSeat();
    expect(seat.textContent).toContain("Approved");
    expect(seat.textContent).toContain("Trained always waits for you");
    expect(screen.getByRole("link", { name: /Waiting for you to send/ })).toBeTruthy();
  });

  it("approving while Zitadel was offline says so, in the board's words", async () => {
    decide.mockResolvedValueOnce({ message: "Request resolved", wait_reason: "halted:zitadel_offline" });
    renderQueue();
    await approve();
    const seat = await decisionSeat();
    expect(seat.textContent).toContain("Zitadel didn't answer when it was approved");
    expect(screen.getByRole("link", { name: /Waiting for you to send/ })).toBeTruthy();
  });

  it("approving after another stop uses that stop's words", async () => {
    decide.mockResolvedValueOnce({ message: "Request resolved", wait_reason: "halted:drain_in_progress" });
    renderQueue();
    await approve();
    const seat = await decisionSeat();
    expect(seat.textContent).toMatch(/Another send was running at \d\d:\d\d/);
  });

  it("declining says Nothing was sent — it never reached Zitadel", async () => {
    renderQueue();
    fireEvent.click(screen.getByRole("button", { name: "Decline" }));
    await waitFor(() => expect(decide).toHaveBeenCalledWith({ id: "r1", status: "rejected" }));
    const seat = await decisionSeat();
    expect(seat.textContent).toBe("DeclinedNothing was sentNothing for you to do.");
    expect(seat.textContent).not.toMatch(/Zitadel/);
  });

  it("a failed decision puts the buttons back and says why under them", async () => {
    decide.mockRejectedValueOnce(new Error("boom"));
    renderQueue();
    await approve();
    expect(await screen.findByRole("alert")).toBeTruthy();
    expect(screen.getByRole("button", { name: "Approve" })).toBeTruthy();
  });
});

describe("approving a request that would cancel a change still waiting", () => {
  const waiting = queuedChange({
    id: "00000000-0000-0000-0000-000000000042",
    user_id: "u1",
    project_id: "pLaser",
    role_keys: ["trained"],
    initiated_by_name: "Sam Iyer",
  });

  it("says before the press what approving cancels, in the row's If you approve line", async () => {
    decide.mockClear();
    wire.plan = cancelling([waiting]);
    renderQueue();
    expect(await screen.findByText("If you approve: Approving cancels the removal Sam Iyer queued 2h ago.")).toBeTruthy();
    expect(decide).not.toHaveBeenCalled();
  });

  it("the press opens a small sheet holding only the step at once, with no write; Keep it leaves the request waiting in its seat", async () => {
    decide.mockClear();
    wire.plan = cancelling([waiting]);
    renderQueue();
    await approve();

    const dialog = await screen.findByRole("dialog");
    expect(dialog.textContent).toContain("This cancels a removal that's waiting");
    expect(screen.getByRole("button", { name: "Cancel it and approve" })).toBeTruthy();
    // The step came from the plan: nothing was written to learn it.
    expect(decide).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "Keep it" }));

    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(decide).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: "Approve" })).toBeTruthy();
    expect(screen.queryByRole("status")).toBeNull();
  });

  it("going ahead approves once, carrying the fingerprint and the cancel ids, and the seat says Approved", async () => {
    decide.mockClear();
    decide.mockResolvedValueOnce({ message: "Request resolved" });
    wire.plan = cancelling([waiting], { fingerprint: "fp-42" });
    renderQueue();
    await approve();
    fireEvent.click(await screen.findByRole("button", { name: "Cancel it and approve" }));

    const seat = await decisionSeat();
    expect(seat.textContent).toContain("Approved");
    expect(decide).toHaveBeenCalledTimes(1);
    expect(decide).toHaveBeenCalledWith({ id: "r1", status: "approved", fingerprint: "fp-42", cancel_ids: [waiting.id] });
  });

  it("a plan that cancels nothing pushes no step", async () => {
    decide.mockClear();
    renderQueue();
    await approve();
    await decisionSeat();
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(decide).toHaveBeenCalledTimes(1);
  });

  it("declining reads and sends no plan, and pushes no step", async () => {
    decide.mockClear();
    wire.plan = cancelling([waiting]);
    renderQueue();
    fireEvent.click(screen.getByRole("button", { name: "Decline" }));
    await decisionSeat();
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(decide).toHaveBeenCalledWith({ id: "r1", status: "rejected" });
  });

  it("when their access moved: the seat says nothing was changed and what to do, and the plan is read again", async () => {
    decide.mockClear();
    decide.mockRejectedValueOnce(new ApiError(409, { error: "PLAN_STALE", message: "Moved." }));
    renderQueue();
    await approve();

    expect(await screen.findByText(/^Nothing was changed\. .* access moved while you looked\.$/)).toBeTruthy();
    expect(screen.getByText("Read the line above, then approve again.")).toBeTruthy();
    // Nothing was decided; the plan is invalidated, so it is read again.
    await waitFor(() => expect(wire.calls.length).toBeGreaterThan(1));
    expect(decide).toHaveBeenCalledTimes(1);
  });
});
