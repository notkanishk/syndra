// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import ExpiringAccessPage from "@/app/review/expiring-access/page";
import type { ExpiringGrantRow } from "@/lib/queries/useExpiringAccess";

const state = vi.hoisted(() => ({
  rows: [] as ExpiringGrantRow[],
  acknowledge: vi.fn(),
  clear: vi.fn(),
}));

vi.mock("@/lib/queries/useExpiringAccess", () => ({
  useExpiringGrants: () => ({
    data: state.rows,
    isLoading: false,
    error: null,
    refetch: vi.fn(),
  }),
  useAcknowledgeExpiry: () => ({ mutateAsync: state.acknowledge, isPending: false }),
  useClearExpiryAcknowledgement: () => ({ mutateAsync: state.clear, isPending: false }),
}));

vi.mock("@/lib/queries/useUsers", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/queries/useUsers")>()),
  useCreateGrant: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));

// Captures what the bulk dialog is handed, without rendering it.
const bulk = vi.hoisted(() => ({ props: null as Record<string, unknown> | null }));
vi.mock("@/components/people/BulkDialog", () => ({
  BulkDialog: (props: Record<string, unknown>) => {
    bulk.props = props;
    return null;
  },
}));


function grant(over: Partial<ExpiringGrantRow> = {}): ExpiringGrantRow {
  return {
    id: "g1",
    user_id: "u1",
    project_id: "pLaser",
    role_key: "trained",
    granted_by: "priya",
    reason: "",
    expires_at: "2026-09-01T12:00:00Z",
    created_at: "2026-06-01T12:00:00Z",
    ...over,
  };
}

function renderPage() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <ExpiringAccessPage />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  state.rows = [grant()];
  state.acknowledge = vi.fn().mockResolvedValue({ message: "ok" });
  state.clear = vi.fn().mockResolvedValue({ message: "ok" });
  bulk.props = null;
});

/**
 * Selection is a mode, announced by a named control. A test that wants a row checkbox has to ask
 * for one the way an operator does — nothing about a row at rest says it can be ticked.
 */
function enterSelect() {
  fireEvent.click(screen.getByRole("button", { name: "Select" }));
}

/**
 * The selection bar's verb, not a row's. The row's says "Extend" because it extends; the bar's
 * says what it actually does, which is open a plan.
 */
function clickBulkExtend() {
  const bar = screen.getByRole("region", { name: "Selection" });
  fireEvent.click(within(bar).getByRole("button", { name: "Rehearse an extension" }));
}

describe("Expiring access — bulk extend scope", () => {
  // The rows here ARE grants. Sending only the people would extend everything they hold that
  // expires, including grants beyond this screen's 30 days — access the operator was never shown.
  it("passes the ticked grant ids, not just the people they belong to", () => {
    state.rows = [
      grant({ id: "g1", user_id: "u1" }),
      grant({ id: "g2", user_id: "u1", project_id: "pWood", role_key: "member" }),
    ];
    renderPage();
    enterSelect();

    const checkboxes = screen.getAllByLabelText("Select this expiring grant");
    fireEvent.click(checkboxes[0]);
    clickBulkExtend();

    expect(bulk.props).not.toBeNull();
    expect(bulk.props?.grantIds).toEqual(["g1"]);
    // One person, but only one of their two expiring grants.
    expect(bulk.props?.userIds).toEqual(["u1"]);
  });

  it("carries every ticked row when more than one is selected", () => {
    state.rows = [grant({ id: "g1", user_id: "u1" }), grant({ id: "g2", user_id: "u2" })];
    renderPage();
    enterSelect();

    for (const box of screen.getAllByLabelText("Select this expiring grant")) {
      fireEvent.click(box);
    }
    clickBulkExtend();

    expect(bulk.props?.grantIds).toEqual(["g1", "g2"]);
    expect(bulk.props?.userIds).toEqual(["u1", "u2"]);
  });
});

describe("Expiring access — acknowledgement (C4)", () => {
  // The single most dangerous misreading: that recording a decision keeps the access alive. The
  // dialog has to deny it in words, on the screen where the button is.
  it("says acknowledging does not keep the access", () => {
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: "Let it lapse" }));
    expect(document.body.textContent).toMatch(/still will/i);
    expect(document.body.textContent).toMatch(/records that you looked/i);
  });

  // The reopen rule is the reason this shape was chosen over a timer or a permanent dismissal, so
  // it is stated where the decision is made rather than only in a design doc.
  it("states the reopen rule in the dialog", () => {
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: "Let it lapse" }));
    expect(document.body.textContent).toMatch(/extends or re-grants/i);
    expect(document.body.textContent).toMatch(/comes back/i);
  });

  // The date travels. Without it the backend cannot enforce the reopen rule, and an
  // acknowledgement would be permanent — which is the rule we deliberately did not pick.
  it("sends the expiry the operator was shown", async () => {
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: "Let it lapse" }));
    fireEvent.change(screen.getByLabelText("Why (optional)"), {
      target: { value: "  Cohort ends  " },
    });
    fireEvent.click(screen.getByRole("button", { name: "Record it" }));

    await waitFor(() =>
      expect(state.acknowledge).toHaveBeenCalledWith({
        grantId: "g1",
        expiresAt: "2026-09-01T12:00:00Z",
        note: "Cohort ends",
      }),
    );
  });

  it("does not require a note", async () => {
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: "Let it lapse" }));
    fireEvent.click(screen.getByRole("button", { name: "Record it" }));
    await waitFor(() => expect(state.acknowledge).toHaveBeenCalled());
    expect(state.acknowledge.mock.calls[0][0].note).toBe("");
  });

  it("names who decided, and why, on the row itself", () => {
    state.rows = [
      grant({
        acknowledged: { by: "priya", at: "2026-08-04T09:00:00Z", note: "Cohort ends" },
      }),
    ];
    renderPage();
    expect(document.body.textContent).toMatch(/let this lapse on/i);
    expect(document.body.textContent).toMatch(/Cohort ends/);
  });

  // An acknowledged row moves under a heading rather than disappearing. Hiding it would be the
  // client-side dismissal the design brief forbids, and it would also hide the decision from the
  // person who made it.
  it("groups acknowledged rows without hiding them", () => {
    state.rows = [
      grant({ id: "g1" }),
      grant({
        id: "g2",
        user_id: "u2",
        acknowledged: { by: "priya", at: "2026-08-04T09:00:00Z" },
      }),
    ];
    renderPage();
    expect(document.body.textContent).toMatch(/Acknowledged · 1 grant that will lapse/);
    expect(screen.getByRole("button", { name: "Undo" })).toBeTruthy();
  });

  it("takes an acknowledgement back", async () => {
    state.rows = [grant({ acknowledged: { by: "priya", at: "2026-08-04T09:00:00Z" } })];
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: "Undo" }));
    await waitFor(() => expect(state.clear).toHaveBeenCalledWith("g1"));
  });

  // Changing your mind toward KEEPING access must never be harder than letting it go.
  it("keeps Extend available on an acknowledged row", () => {
    state.rows = [grant({ acknowledged: { by: "priya", at: "2026-08-04T09:00:00Z" } })];
    renderPage();
    expect(screen.getByRole("button", { name: "Extend" })).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Let it lapse" })).toBeNull();
  });

  // A queue whose only rows are acknowledged ones looks identical to one nobody has touched.
  it("says outright when nothing is waiting on a decision", () => {
    state.rows = [grant({ acknowledged: { by: "priya", at: "2026-08-04T09:00:00Z" } })];
    renderPage();
    expect(document.body.textContent).toMatch(/Nothing here is waiting on a decision/i);
  });

  // Acknowledging is per-row on purpose: the record's whole value is that somebody read the row,
  // and a checkbox on a decided row would also let a bulk extend quietly undo the decision.
  it("offers no checkbox on an acknowledged row", () => {
    state.rows = [
      grant({ id: "g1" }),
      grant({
        id: "g2",
        user_id: "u2",
        acknowledged: { by: "priya", at: "2026-08-04T09:00:00Z" },
      }),
    ];
    renderPage();
    enterSelect();
    expect(screen.getAllByLabelText("Select this expiring grant")).toHaveLength(1);
  });

  // A stale page is the reopen rule arriving early. The server's own message says to reload, and
  // the dialog must stay open — a closed dialog would read as a saved decision.
  it("keeps the dialog open when the expiry moved underneath it", async () => {
    state.acknowledge = vi
      .fn()
      .mockRejectedValue(new Error("This grant's expiry changed since you loaded the page."));
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: "Let it lapse" }));
    fireEvent.click(screen.getByRole("button", { name: "Record it" }));

    await waitFor(() => expect(state.acknowledge).toHaveBeenCalled());
    expect(screen.getByRole("button", { name: "Record it" })).toBeTruthy();
  });
});
