// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import ExpiringAccessPage from "@/app/review/expiring-access/page";
import { ApiError } from "@/lib/api-client";
import type { ExpiringGrantRow } from "@/lib/queries/useExpiringAccess";
import type { ActPlan } from "@/lib/usePlanConsent";
import { actPlan, cancelling, queuedChange } from "@/test-utils/plan";

const state = vi.hoisted(() => ({
  rows: [] as ExpiringGrantRow[],
  acknowledge: vi.fn(),
  clear: vi.fn(),
  extend: vi.fn(),
  waits: false,
}));

vi.mock("@/lib/queries/useRoleWaits", () => ({
  useRoleWaits: () => () => state.waits,
}));

// A ceiling of four, so the unit logic can be exercised without rendering six
// hundred rows into jsdom — what is under test here is which number the page
// gates on and what its narrowing keeps, and the magnitude is irrelevant to
// both. The value itself is held against the backend constant by
// `lib/queries/__tests__/bulkCeiling.test.ts`, which was written because this
// comment used to claim that check existed before it did.
vi.mock("next/navigation", () => ({
  useSearchParams: () => new URLSearchParams(""),
  useRouter: () => ({ replace: vi.fn() }),
}));

vi.mock("@/lib/queries/useBulkGrants", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/queries/useBulkGrants")>()),
  BULK_MAX_USERS: 4,
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
  useCreateGrant: () => ({ mutateAsync: state.extend, isPending: false }),
}));

// What the planner answers at the press (GET /users/{id}/plan?op=extend), and what it was asked.
const plan = vi.hoisted(() => ({ next: undefined as unknown as ActPlan, asked: [] as unknown[] }));
vi.mock("@/lib/usePlanConsent", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/usePlanConsent")>()),
  fetchPlan: (_client: unknown, action: unknown) => {
    plan.asked.push(action);
    return Promise.resolve(plan.next);
  },
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
  state.waits = false;
  state.acknowledge = vi.fn().mockResolvedValue({ message: "ok" });
  state.clear = vi.fn().mockResolvedValue({ message: "ok" });
  // An extension is a ledger edit: no outbox row, done when the write answers.
  state.extend = vi.fn().mockResolvedValue({ outbox_id: "", status: "applied" });
  plan.next = actPlan({ delivery: "none", detail: "Keeps Trained in Laser Lab until 30 Nov 2026." });
  plan.asked = [];
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
  fireEvent.click(within(bar).getByRole("button", { name: "Preview an extension" }));
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

    const checkboxes = screen.getAllByLabelText(/^Select .+, ending /);
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

    for (const box of screen.getAllByLabelText(/^Select .+, ending /)) {
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
    expect(document.body.textContent).toMatch(/as it would have/i);
    expect(document.body.textContent).toMatch(/records that you looked/i);
  });

  // The reopen rule is the reason this shape was chosen over a timer or a permanent dismissal, so
  // it is stated where the decision is made rather than only in a design doc.
  it("states the reopen rule in the dialog", () => {
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: "Let it lapse" }));
    expect(document.body.textContent).toMatch(/asks again if somebody changes the date/i);
    expect(document.body.textContent).toMatch(/the row asks again/i);
  });

  // The date travels. Without it the backend cannot enforce the reopen rule, and an
  // acknowledgement would be permanent — which is the rule we deliberately did not pick.
  it("sends the expiry the operator was shown", async () => {
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: "Let it lapse" }));
    fireEvent.change(screen.getByLabelText("Why (optional)"), {
      target: { value: "  Cohort ends  " },
    });
    fireEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Let it lapse" }));

    await waitFor(() =>
      expect(state.acknowledge).toHaveBeenCalledWith({
        grantId: "g1",
        expiresAt: "2026-09-01T12:00:00Z",
        note: "Cohort ends",
      }),
    );
  });

  // A role that waits is not ended by the sweep alone: every place that says
  // "it ends on its date" says where it goes instead, and what to do then.
  it("a role that waits: its removal goes to Pending changes on the date, for you to send", async () => {
    state.waits = true;
    state.acknowledge.mockResolvedValue({});
    renderPage();
    expect(document.body.textContent).toMatch(/A role that waits for you goes to Pending changes on its date instead, for you to send\./);
    fireEvent.click(screen.getByRole("button", { name: "Let it lapse" }));
    expect(within(screen.getByRole("dialog")).getByText(/its removal goes to Pending changes for you to send/)).toBeTruthy();
    fireEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Let it lapse" }));
    expect(await screen.findByText("Its removal goes to Pending changes on its date, for you to send.")).toBeTruthy();
    expect(screen.getByText(/press Send on Pending changes\./)).toBeTruthy();
    expect(screen.queryByText("Nothing for you to do.")).toBeNull();
    state.waits = false;
  });

  it("a failed acknowledgement says Not recorded and the dialog keeps its button", async () => {
    state.acknowledge.mockRejectedValue(new ApiError(500, { error: "BOOM", message: "boom" }));
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: "Let it lapse" }));
    fireEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Let it lapse" }));
    expect(await screen.findByText("Not recorded")).toBeTruthy();
    expect(screen.queryByText("Recorded")).toBeNull();
    expect(within(screen.getByRole("dialog")).getByRole("button", { name: "Let it lapse" })).toBeTruthy();
  });

  it("does not require a note", async () => {
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: "Let it lapse" }));
    fireEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Let it lapse" }));
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
    expect(document.body.textContent).toMatch(/let it lapse on/i);
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
    expect(screen.getByRole("button", { name: "Undecided" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "Let lapse" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "Undo letting it lapse" })).toBeTruthy();
  });

  it("takes an acknowledgement back", async () => {
    state.rows = [grant({ acknowledged: { by: "priya", at: "2026-08-04T09:00:00Z" } })];
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: "Undo letting it lapse" }));
    await waitFor(() => expect(state.clear).toHaveBeenCalledWith("g1"));
  });

  // Changing your mind toward KEEPING access must never be harder than letting it go.
  it("keeps Extend available on an acknowledged row", () => {
    state.rows = [grant({ acknowledged: { by: "priya", at: "2026-08-04T09:00:00Z" } })];
    renderPage();
    expect(screen.getByRole("button", { name: "Extend 90 days" })).toBeTruthy();
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
    expect(screen.getAllByLabelText(/^Select .+, ending /)).toHaveLength(1);
  });

  // A stale page is the reopen rule arriving early. The server's own message says to reload, and
  // the dialog must stay open — a closed dialog would read as a saved decision.
  it("keeps the dialog open when the expiry moved underneath it", async () => {
    state.acknowledge = vi
      .fn()
      .mockRejectedValue(new Error("This grant's expiry changed since you loaded the page."));
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: "Let it lapse" }));
    fireEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Let it lapse" }));

    await waitFor(() => expect(state.acknowledge).toHaveBeenCalled());
    expect(within(screen.getByRole("dialog")).getByRole("button", { name: "Let it lapse" })).toBeTruthy();
  });

  // Nothing called `onClose()` on success, so the dialog stayed open with the
  // same "Let it lapse" button live — a second click recorded the same
  // acknowledgment again, against a grant that had not moved.
  // A-17 G4: Cancel is the ghost half of the pair beside the act (placed by ModalFooter); after
  // the result there is one outline Close and nothing else.
  it("pairs a ghost Cancel with the act, and closes with one outline Close", async () => {
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: "Let it lapse" }));
    const dialog = screen.getByRole("dialog");
    const cancel = within(dialog).getByRole("button", { name: "Cancel" });
    const record = within(dialog).getByRole("button", { name: "Let it lapse" });
    expect(cancel).not.toHaveClass("border-line-strong");

    fireEvent.click(record);
    const close = await within(dialog).findByRole("button", { name: "Close" });
    expect(close).toHaveClass("border-line-strong");
    expect(within(dialog).queryByRole("button", { name: "Cancel" })).toBeNull();
  });

  it("cannot record the same decision twice", async () => {
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: "Let it lapse" }));
    fireEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Let it lapse" }));

    await waitFor(() => expect(screen.getByRole("button", { name: "Close" })).toBeInTheDocument());
    expect(within(screen.getByRole("dialog")).queryByRole("button", { name: "Let it lapse" })).toBeNull();
    expect(state.acknowledge).toHaveBeenCalledTimes(1);
  });
});

/**
 * This screen's rows are grants and its only bulk verb extends by PERSON —
 * `services.BulkMaxUsers` caps `user_ids`, not grant ids. It was the last
 * select-all surface with a capped endpoint and no ceiling, and the one where
 * copying the prop from People would have been wrong: the number the bar
 * counts is not the number the server refuses on.
 */
describe("the expiry queue gates on people, not on grants", () => {
  function selectAll(rows: ExpiringGrantRow[]) {
    state.rows = rows;
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: "Select" }));
    fireEvent.click(
      screen.getByRole("checkbox", { name: new RegExp(`Select these ${rows.length} role`) }),
    );
  }

  /** `count` grants spread two-per-person, so the two units diverge. */
  function pairs(count: number): ExpiringGrantRow[] {
    return Array.from({ length: count }, (_, at) =>
      grant({ id: `g${at}`, user_id: `u${Math.floor(at / 2)}` }),
    );
  }

  it("lets six grants through when they belong to three people", () => {
    selectAll(pairs(6));

    expect(screen.queryByText(/you can change at most/)).toBeNull();
    expect(screen.getByRole("button", { name: "Preview an extension" })).toBeTruthy();
  });

  it("refuses five grants held by five people, and says which number it refuses on", () => {
    selectAll(Array.from({ length: 5 }, (_, at) => grant({ id: `g${at}`, user_id: `u${at}` })));

    expect(
      screen.getByText(/they cover 5 people, and you can change at most 4 people at once/),
    ).toBeTruthy();
    // The grant count stays on screen: the operator ticked grants and has to
    // recognise what they ticked.
    expect(screen.getByText(/5 roles selected/)).toBeTruthy();
  });

  // Narrowing takes whole people. Dropping a person's later grants would
  // extend part of their access and leave the rest to lapse, which is worse
  // than refusing.
  it("narrows to whole people, keeping every grant the cohort holds", () => {
    selectAll(pairs(12));

    fireEvent.click(
      screen.getByRole("button", { name: "Select the first 4 people in the order shown" }),
    );

    // Four people at two grants each — eight, not four.
    expect(screen.getByText(/8 roles selected/)).toBeTruthy();
    expect(screen.queryByText(/you can change at most/)).toBeNull();
  });
});

describe("extending a single row", () => {
  // Extending is planned (task 1.2a) and is a ledger edit: Zitadel has no end
  // dates, so nothing is sent and nothing waits for Send, whatever the role.
  it("reads the plan, writes with its fingerprint, and says it is done with the plan's words", async () => {
    state.extend = vi.fn().mockResolvedValue({ outbox_id: "", status: "applied" });
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: "Extend 90 days" }));

    await waitFor(() => expect(document.body.textContent).toMatch(/Extended by 90 days/));
    expect(document.body.textContent).toMatch(/Keeps Trained in Laser Lab until 30 Nov 2026\. It leaves this list on the next read\./);
    expect(document.body.textContent).toMatch(/Nothing for you to do\./);
    expect(screen.queryByRole("link", { name: /Waiting for you to send/ })).toBeNull();
    expect(plan.asked).toEqual([{ op: "extend", userId: "u1", projectId: "pLaser", roleKey: "trained", days: 90 }]);
    expect(state.extend).toHaveBeenCalledWith(
      expect.objectContaining({ project_id: "pLaser", role_key: "trained", duration_days: 90, extend: true, fingerprint: "fp-1", cancel_ids: [] }),
    );
  });

  it("a plan that refuses writes nothing, says why, and ends on its step", async () => {
    plan.next = actPlan({ effect: "no_change", refusal: "runs_past", detail: "Already ends later than this would make it." });
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: "Extend 90 days" }));

    await waitFor(() => expect(document.body.textContent).toMatch(/Already ends later than this would make it\. Nothing was changed\./));
    expect(document.body.textContent).toMatch(/Nothing for you to do\./);
    expect(state.extend).not.toHaveBeenCalled();
  });

  it("their access moved before the write: nothing changed, and Extend is still there to press again", async () => {
    state.extend = vi.fn().mockRejectedValue(
      new ApiError(409, { error: "PLAN_STALE", message: "Nothing was changed. Their access moved while this was open, so read it again before you go ahead." }),
    );
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: "Extend 90 days" }));

    await waitFor(() => expect(document.body.textContent).toMatch(/Nothing was changed\. Their access moved while you looked\./));
    expect(document.body.textContent).toMatch(/Press Extend again to read it afresh\./);
    expect(screen.getByRole("button", { name: "Extend 90 days" })).toBeEnabled();
  });

  it("one that would cancel a waiting removal pushes the step at once, and the write names it", async () => {
    const waiting = queuedChange({ project_id: "pLaser", role_keys: ["trained"] });
    plan.next = cancelling([waiting]);
    state.extend = vi.fn().mockResolvedValue({ outbox_id: "", status: "applied" });
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: "Extend 90 days" }));

    fireEvent.click(await screen.findByRole("button", { name: "Cancel removing it and extend" }));
    await waitFor(() => expect(state.extend).toHaveBeenCalledWith(expect.objectContaining({ cancel_ids: [waiting.id] })));
  });
});

// UserName's own "cannot resolve" fallback always appends the id as a muted
// mono trailer — this row used to hand it the id AGAIN as the fallback text,
// so an acknowledger nobody could resolve rendered as the same raw id twice
// side by side, with no readable label anywhere in it.
it("names who acknowledged with a readable label, not the raw id twice over", () => {
  state.rows = [
    grant({ acknowledged: { by: "op_2f81a9", at: "2026-08-04T09:00:00Z" } }),
  ];
  renderPage();
  // Three, not one: the row's "Given by" (the laptop line and the desktop column, one shown by CSS)
  // already use this same fallback phrase for an unresolved id, and the acknowledger does too. Before the
  // fix, the acknowledger fell back to its own raw id instead of this phrase,
  // so only one match existed here.
  expect(screen.getAllByText(/somebody no longer listed/i)).toHaveLength(3);
});
