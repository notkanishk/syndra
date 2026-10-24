// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { RequestsScreen } from "@/components/requests/RequestsScreen";
import type { AccessRequest } from "@/lib/queries/useRequests";
import { gapSettled } from "@/test-utils/leaving";
import { motionInTests } from "@/test-utils/motion";

/**
 * Motion 5b + S5 on the operator queue: a decided row keeps its place with its
 * outcome in the Decision seat after the refetch drops it, for one beat; then
 * it fades and its gap closes. The mocked read below is the refetch: tests
 * reassign `data` and rerender.
 */

let data: AccessRequest[] = [];
const decide = vi.fn();
const applyBulk = vi.fn();

vi.mock("next/navigation", () => ({
  useSearchParams: () => new URLSearchParams(""),
  useRouter: () => ({ replace: () => {} }),
}));

vi.mock("@/lib/queries/useRequests", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/queries/useRequests")>()),
  useRequestsAdmin: () => ({ data, isLoading: false, error: null, refetch: () => {} }),
  useRequestsMine: () => ({ data: [], isLoading: false, error: null, refetch: () => {} }),
  useDecideRequest: () => ({ mutateAsync: decide, isPending: false }),
  useRehearseBulkDecision: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useApplyBulkDecision: () => ({ mutateAsync: applyBulk, isPending: false }),
}));

vi.mock("@/lib/queries/useRoles", () => ({ useGlobalRoleCatalog: () => ({ data: [] }) }));
vi.mock("@/lib/queries/useCatalogUsers", () => ({
  useCatalogProjects: () => ({ data: [], isLoading: false, error: null, refetch: () => {} }),
}));
vi.mock("@/components/names", () => ({
  ProjectName: () => null,
  RoleRef: () => null,
  UserName: ({ id }: { id: string }) => <span>{id}</span>,
}));

// The dialog's own flow is tested elsewhere: this one only applies and closes.
vi.mock("@/components/ui/RehearsalDialog", () => ({
  RehearsalDialog: ({
    onApply,
    onClose,
  }: {
    onApply: (planId: string) => Promise<unknown>;
    onClose: () => void;
  }) => (
    <div role="dialog">
      <button onClick={() => void onApply("plan")}>Apply</button>
      <button onClick={onClose}>Close</button>
    </div>
  ),
}));

const ask = (id: string, requester: string): AccessRequest => ({
  id,
  requester_id: requester,
  project_id: "pLaser",
  role_key: "trained",
  justification: `why ${id}`,
  status: "pending",
  created_at: "2026-07-22T06:00:00Z",
});

const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
function Screen() {
  return (
    <QueryClientProvider client={client}>
      <RequestsScreen isOperator userId="op1" />
    </QueryClientProvider>
  );
}

// jsdom has no AnimationEvent, so React listens for the prefixed name here.
const flush = () => act(async () => {});
const tick = (ms: number) => act(() => void vi.advanceTimersByTime(ms));

/** The row's wrapper: it carries the leaving state, the fade and the gap. */
const rowOf = (text: string) => screen.getByText(text).closest("[class*=row-divider]")!.parentElement!.parentElement!;
const fadeOf = rowOf;
const list = () => document.querySelector("[data-selection-scope]")!;

beforeEach(() => {
  vi.useFakeTimers();
  motionInTests();
  data = [ask("r1", "u1"), ask("r2", "u2")];
  decide.mockReset().mockResolvedValue({ message: "Request resolved" });
  applyBulk.mockReset();
});
afterEach(() => vi.useRealTimers());

describe("a decided request keeps its seat for a beat", () => {
  it("shows its outcome after the refetch drops it, then fades, then closes its gap", async () => {
    const { rerender } = render(<Screen />);
    fireEvent.click(screen.getAllByRole("button", { name: "Approve" })[0]);
    await flush();

    // The refetch: r1 is no longer pending.
    data = [ask("r2", "u2")];
    rerender(<Screen />);

    expect(screen.getByText(/why r1/)).toBeInTheDocument();
    expect(within(rowOf(/why r1/)).getByRole("status").textContent).toBe("ApprovedSent · not yet seen in Zitadel");
    expect(fadeOf(/why r1/)).not.toHaveAttribute("data-leaving");

    tick(899);
    expect(fadeOf(/why r1/)).not.toHaveAttribute("data-leaving");
    tick(1);
    expect(fadeOf(/why r1/)).toHaveAttribute("data-leaving");
    expect(rowOf(/why r1/)).toHaveAttribute("data-closing");
    expect(rowOf(/why r1/)).toHaveAttribute("aria-hidden", "true");

    gapSettled(rowOf(/why r1/));
    expect(screen.queryByText(/why r1/)).not.toBeInTheDocument();
    expect(screen.getByText(/why r2/)).toBeInTheDocument();
  });

  it("says Declined the same way", async () => {
    const { rerender } = render(<Screen />);
    fireEvent.click(screen.getAllByRole("button", { name: "Decline" })[0]);
    await flush();
    data = [ask("r2", "u2")];
    rerender(<Screen />);
    expect(within(rowOf(/why r1/)).getByRole("status").textContent).toBe("DeclinedNothing was sent");
  });

  it("leaves the other rows' buttons working during the beat", async () => {
    const { rerender } = render(<Screen />);
    fireEvent.click(screen.getAllByRole("button", { name: "Approve" })[0]);
    await flush();
    data = [ask("r2", "u2")];
    rerender(<Screen />);

    tick(300);
    fireEvent.click(screen.getByRole("button", { name: "Approve" }));
    await flush();
    expect(decide).toHaveBeenLastCalledWith({ id: "r2", status: "approved" });
    // r2's own beat starts when it is decided, not with r1's.
    tick(600);
    expect(fadeOf(/why r1/)).toHaveAttribute("data-leaving");
    expect(fadeOf(/why r2/)).not.toHaveAttribute("data-leaving");
  });

  it("keeps the gap open while the pointer is in the list, and closes it when it leaves", async () => {
    const { rerender } = render(<Screen />);
    fireEvent.pointerEnter(list());
    fireEvent.click(screen.getAllByRole("button", { name: "Approve" })[0]);
    await flush();
    data = [ask("r2", "u2")];
    rerender(<Screen />);
    tick(900);

    expect(fadeOf(/why r1/)).toHaveAttribute("data-leaving");
    expect(rowOf(/why r1/)).not.toHaveAttribute("data-closing");
    fireEvent.pointerLeave(list());
    expect(rowOf(/why r1/)).toHaveAttribute("data-closing");
  });

  it("shows the empty state only after the last row has left", async () => {
    data = [ask("r1", "u1")];
    const { rerender } = render(<Screen />);
    fireEvent.click(screen.getByRole("button", { name: "Approve" }));
    await flush();
    data = [];
    rerender(<Screen />);

    expect(screen.queryByText("No open requests.")).not.toBeInTheDocument();
    expect(screen.getByRole("status").textContent).toContain("Approved");
    tick(900);
    expect(screen.queryByText("No open requests.")).not.toBeInTheDocument();
    gapSettled(rowOf(/why r1/));
    expect(screen.getByText("No open requests.")).toBeInTheDocument();
  });

  it("a failed decision puts the buttons back, whatever the read does", async () => {
    decide.mockRejectedValueOnce(new Error("boom"));
    const { rerender } = render(<Screen />);
    fireEvent.click(screen.getAllByRole("button", { name: "Approve" })[0]);
    await flush();
    rerender(<Screen />);

    expect(screen.getByRole("alert")).toBeInTheDocument();
    expect(screen.getAllByRole("button", { name: "Approve" })).toHaveLength(2);
    tick(2000);
    expect(screen.getByText(/why r1/)).toBeInTheDocument();
    expect(fadeOf(/why r1/)).not.toHaveAttribute("data-leaving");
  });

  it("changing the filter is a different list: nothing fades out of it", async () => {
    const { rerender } = render(<Screen />);
    data = [{ ...ask("r9", "u9"), status: "approved" }];
    fireEvent.click(screen.getByRole("radio", { name: "Approved" }));
    rerender(<Screen />);
    expect(screen.queryByText(/why r1/)).not.toBeInTheDocument();
    expect(screen.getByText(/why r9/)).toBeInTheDocument();
  });
});

describe("under reduced motion", () => {
  it("shows the outcome in place for the same beat, then the row is simply gone", async () => {
    motionInTests(false);
    const { rerender } = render(<Screen />);
    fireEvent.click(screen.getAllByRole("button", { name: "Approve" })[0]);
    await flush();
    data = [ask("r2", "u2")];
    rerender(<Screen />);

    expect(within(rowOf(/why r1/)).getByRole("status").textContent).toContain("Approved");
    expect(rowOf(/why r1/)).not.toHaveAttribute("data-closing");
    tick(900);
    expect(screen.queryByText(/why r1/)).not.toBeInTheDocument();
  });
});

describe("a bulk decision from Select mode", () => {
  async function approveBoth() {
    applyBulk.mockResolvedValue({
      op: "request-decision",
      applied: true,
      summary: {},
      outcomes: [
        { user_id: "u1", name: "", email: "", effect: "applied", detail: "" },
        { user_id: "u2", name: "", email: "", effect: "queued", detail: "This role always waits for you." },
      ],
    });
    const view = render(<Screen />);
    fireEvent.click(screen.getByRole("button", { name: "Select" }));
    for (const box of screen.getAllByRole("checkbox", { name: "Select this request" })) fireEvent.click(box);
    fireEvent.click(screen.getByRole("button", { name: "Review approving these" }));
    fireEvent.click(screen.getByRole("button", { name: "Apply" }));
    await flush();
    data = [];
    view.rerender(<Screen />);
    return view;
  }

  it("each decided row keeps its seat with its own outcome, and the beat starts when the dialog closes", async () => {
    await approveBoth();

    const first = within(rowOf(/why r1/)).getByRole("status");
    const second = within(rowOf(/why r2/)).getByRole("status");
    expect(first.textContent).toBe("ApprovedSent · not yet seen in Zitadel");
    expect(second.textContent).toContain("Approved");
    expect(second.textContent).toContain("This role always waits for you.");

    // The operator is reading the dialog's result: nothing leaves behind it.
    tick(5000);
    expect(fadeOf(/why r1/)).not.toHaveAttribute("data-leaving");

    fireEvent.click(screen.getByRole("button", { name: "Close" }));
    tick(899);
    expect(fadeOf(/why r1/)).not.toHaveAttribute("data-leaving");
    tick(1);
    expect(fadeOf(/why r1/)).toHaveAttribute("data-leaving");
    expect(fadeOf(/why r2/)).toHaveAttribute("data-leaving");
    expect(screen.queryByText("No open requests.")).not.toBeInTheDocument();

    gapSettled(rowOf(/why r1/));
    gapSettled(rowOf(/why r2/));
    expect(screen.getByText("No open requests.")).toBeInTheDocument();
  });
});
