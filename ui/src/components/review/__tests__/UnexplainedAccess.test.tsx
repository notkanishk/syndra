// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { UnexplainedAccess } from "@/components/review/UnexplainedAccess";
import type { DriftTriageItem } from "@/lib/queries/useDrift";

const drift = vi.hoisted(() => ({ data: [] as DriftTriageItem[] }));

// The plan the bulk endpoints return — the same shape every bulk surface in the
// product returns, so the triage queue and the People page share one renderer.
const bulk = vi.hoisted(() => ({
  plan: {
    op: "adopt",
    applied: false,
    outcomes: [] as Array<Record<string, unknown>>,
    plan_id: "plan_1",
    summary: { total: 0, apply: 0, no_change: 0, blocked: 0, failed: 0, succeeded: 0, queued: 0 },
  },
  rehearsals: 0,
  applies: 0,
}));

vi.mock("next/navigation", () => ({
  useSearchParams: () => new URLSearchParams(""),
  useRouter: () => ({ replace: () => {} }),
}));

vi.mock("@/lib/queries/useDrift", () => ({
  useDriftItems: () => ({ ...drift, isLoading: false, error: null, refetch: () => {} }),
  useAttributeDrift: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useRevokeDrift: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useMarkExternalDrift: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useRehearseAdoptDrift: () => ({
    mutateAsync: async () => {
      bulk.rehearsals += 1;
      return bulk.plan;
    },
    isPending: false,
  }),
  useRehearseMarkExternalDrift: () => ({
    mutateAsync: async () => {
      bulk.rehearsals += 1;
      return bulk.plan;
    },
    isPending: false,
  }),
  useBulkAttributeDrift: () => ({
    mutateAsync: async () => {
      bulk.applies += 1;
      return { ...bulk.plan, applied: true };
    },
    isPending: false,
  }),
  useBulkMarkExternalDrift: () => ({
    mutateAsync: async () => {
      bulk.applies += 1;
      return { ...bulk.plan, applied: true };
    },
    isPending: false,
  }),
  useReconcileNow: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));

vi.mock("@/lib/queries/useGrants", () => ({
  useReconciliationDiff: () => ({ data: undefined, isLoading: false, error: null, refetch: () => {} }),
}));

function item(overrides: Partial<DriftTriageItem> = {}): DriftTriageItem {
  return {
    id: "d1",
    user_id: "u1",
    project_id: "p1",
    role_keys: ["operator"],
    drift_type: "target_only",
    detection_source: "reconciliation_sweep",
    detected_at: "2026-07-22T06:00:00Z",
    target: "zitadel",
    role_in_catalogue: true,
    role_catalogue_applies: true,
    user_is_service_account: false,
    other_items_for_user: 0,
    ...overrides,
  } as DriftTriageItem;
}

function renderTriage() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <UnexplainedAccess />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  drift.data = [item()];
  bulk.plan = {
    op: "adopt",
    applied: false,
    outcomes: [],
    plan_id: "plan_1",
    summary: { total: 0, apply: 0, no_change: 0, blocked: 0, failed: 0, succeeded: 0, queued: 0 },
  };
  bulk.rehearsals = 0;
  bulk.applies = 0;
});

describe("Unexplained access — triage", () => {
  it("offers the three resolutions in one fixed order on every row", () => {
    drift.data = [item({ id: "a" }), item({ id: "b", user_id: "u2" })];
    renderTriage();

    const rows = screen.getAllByRole("button", { name: "Adopt" });
    expect(rows).toHaveLength(2);

    // A scanning eye should learn one sequence: Adopt · Revoke · Owned elsewhere.
    const labels = screen
      .getAllByRole("button")
      .map((button) => button.textContent?.trim())
      .filter((label) => label === "Adopt" || label === "Revoke" || label === "Owned elsewhere");
    expect(labels.slice(0, 3)).toEqual(["Adopt", "Revoke", "Owned elsewhere"]);
  });

  it("states that bulk revoke is deliberately absent rather than leaving a hole", () => {
    renderTriage();
    expect(
      screen.getByText(/Bulk adopt and bulk mark-as-external exist; bulk revoke does not\./),
    ).toBeInTheDocument();
  });

  it("offers bulk adopt and bulk mark-external once rows are selected — and never bulk revoke", () => {
    renderTriage();
    fireEvent.click(screen.getAllByRole("checkbox")[1]);

    const bar = screen.getByRole("region", { name: "Selection" });
    expect(within(bar).getByRole("button", { name: "Adopt in Syndra" })).toBeInTheDocument();
    expect(within(bar).getByRole("button", { name: "Mark as owned elsewhere" })).toBeInTheDocument();
    expect(within(bar).queryByRole("button", { name: /Revoke/ })).not.toBeInTheDocument();
  });

  it("has a select-all that covers the whole queue, not the rendered page", () => {
    drift.data = Array.from({ length: 20 }, (_, index) =>
      item({ id: `d${index}`, user_id: `u${index}` }),
    );
    renderTriage();

    // The queue pages at 12, but the queue is what you are triaging.
    fireEvent.click(screen.getByRole("checkbox", { name: /Select all 20 unexplained items/ }));
    expect(screen.getByText(/20 items selected/)).toBeInTheDocument();
  });

  it("extends a range on shift-click instead of making you tick each row", () => {
    drift.data = Array.from({ length: 5 }, (_, index) =>
      item({ id: `d${index}`, user_id: `u${index}` }),
    );
    renderTriage();

    const boxes = screen.getAllByRole("checkbox").slice(1);
    fireEvent.click(boxes[0]);
    fireEvent.click(boxes[3], { shiftKey: true });
    expect(screen.getByText(/4 items selected/)).toBeInTheDocument();
  });

  it("says what the selection is made of, not just how much of it there is", () => {
    drift.data = [
      item({ id: "d1", user_id: "u1", role_group: "Safety-gated" }),
      item({ id: "d2", user_id: "u1", role_group: "Open bench" }),
    ];
    renderTriage();
    fireEvent.click(screen.getByRole("checkbox", { name: /Select all/ }));

    // Safety-gated is what the queue's own ordering keys on, so it is what an
    // operator needs before resolving several rows at once.
    expect(screen.getByText(/1 safety-gated/)).toBeInTheDocument();
    expect(screen.getByText(/1 person/)).toBeInTheDocument();
  });

  it("selects the whole cluster a drift row belongs to", () => {
    drift.data = [
      item({ id: "d1", user_id: "u1", project_id: "p1" }),
      item({ id: "d2", user_id: "u1", project_id: "p9" }),
      item({ id: "d3", user_id: "u7", project_id: "p1" }),
      item({ id: "d4", user_id: "u7", project_id: "p9" }),
    ];
    renderTriage();

    // Drift arrives in clusters — one rule, one person, one project — and no
    // amount of shift-clicking finds the cluster as reliably as asking for it.
    fireEvent.click(screen.getAllByRole("button", { name: "Select similar" })[0]);
    expect(screen.getByText(/3 items selected/)).toBeInTheDocument();
  });

  it("names the upstream actor and date when the detector knew them", () => {
    drift.data = [
      item({ upstream_actor: "svc-badge-sync", upstream_created_at: "2026-07-21T14:12:04Z" }),
    ];
    renderTriage();
    expect(screen.getByText(/by svc-badge-sync/)).toBeInTheDocument();
  });

  it("admits it does not know the actor rather than naming a plausible one", () => {
    renderTriage();
    expect(
      screen.getByText(/compares grant lists and can't see who made the change/),
    ).toBeInTheDocument();
  });

  it("neutralises Adopt for a service account instead of hiding it", () => {
    drift.data = [item({ user_is_service_account: true })];
    renderTriage();
    expect(screen.getByRole("button", { name: "Adopt" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Owned elsewhere" })).toBeEnabled();
  });

  it("says a role is missing from the catalogue, in words as well as colour", () => {
    drift.data = [item({ role_in_catalogue: false })];
    renderTriage();
    expect(screen.getByText("Role not in catalogue")).toBeInTheDocument();
  });

  it("puts the revoke consequence and the person's other items inside the dialog", () => {
    drift.data = [item({ other_items_for_user: 2 })];
    renderTriage();
    fireEvent.click(screen.getByRole("button", { name: "Revoke" }));

    const dialog = screen.getByRole("dialog");
    expect(within(dialog).getByText(/They will lose this role\./)).toBeInTheDocument();
    expect(within(dialog).getByText(/2 more/)).toBeInTheDocument();
  });

  it("shows the evidence grid only when a row is expanded", () => {
    drift.data = [item({ zitadel_grant_id: "zg_4f19c8" })];
    renderTriage();

    expect(screen.queryByText("zg_4f19c8")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { expanded: false }));
    expect(screen.getByText("zg_4f19c8")).toBeInTheDocument();
  });
});

describe("Unexplained access — bulk resolution is rehearsed", () => {
  function selectTwo() {
    drift.data = [item({ id: "d1" }), item({ id: "d2", user_id: "u2" })];
    renderTriage();
    fireEvent.click(screen.getAllByRole("checkbox")[1]);
    fireEvent.click(screen.getAllByRole("checkbox")[2]);
  }

  it("shows what would happen before anything is written", async () => {
    bulk.plan = {
      op: "adopt",
      applied: false,
      outcomes: [
        { user_id: "d1", name: "Ada Lovelace", email: "u1", effect: "apply", detail: "Adopted into Syndra (trained)." },
        { user_id: "d2", name: "Sam Patel", email: "u2", effect: "no_change", detail: "Already resolved as adopted." },
      ],
      plan_id: "plan_1",
      summary: { total: 2, apply: 1, no_change: 1, blocked: 0, failed: 0, succeeded: 0, queued: 0 },
    };
    selectTwo();
    fireEvent.click(screen.getByRole("button", { name: "Adopt in Syndra" }));

    // Rows are named, and the confirm button counts only what will change —
    // offering "Apply to 2" when one is already resolved would be a lie.
    expect(await screen.findByText("Ada Lovelace")).toBeInTheDocument();
    expect(screen.getByText(/Already resolved as adopted/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Apply to 1 item" })).toBeInTheDocument();
    expect(bulk.applies).toBe(0);
  });

  it("does not write until the plan is confirmed", async () => {
    bulk.plan = {
      op: "adopt",
      applied: false,
      outcomes: [{ user_id: "d1", name: "Ada", email: "u1", effect: "apply", detail: "Adopted." }],
      plan_id: "plan_1",
      summary: { total: 1, apply: 1, no_change: 0, blocked: 0, failed: 0, succeeded: 0, queued: 0 },
    };
    selectTwo();
    fireEvent.click(screen.getByRole("button", { name: "Adopt in Syndra" }));

    await waitFor(() => expect(bulk.rehearsals).toBeGreaterThan(0));
    expect(bulk.applies).toBe(0);

    fireEvent.click(await screen.findByRole("button", { name: "Apply to 1 item" }));
    await waitFor(() => expect(bulk.applies).toBe(1));
  });

  it("refuses to apply a plan that would change nothing", async () => {
    bulk.plan = {
      op: "adopt",
      applied: false,
      outcomes: [
        { user_id: "d1", name: "Ada", email: "u1", effect: "no_change", detail: "Already resolved." },
      ],
      plan_id: "plan_1",
      summary: { total: 1, apply: 0, no_change: 1, blocked: 0, failed: 0, succeeded: 0, queued: 0 },
    };
    selectTwo();
    fireEvent.click(screen.getByRole("button", { name: "Adopt in Syndra" }));

    expect(await screen.findByRole("button", { name: "Nothing to apply" })).toBeDisabled();
  });

  it("reports what actually happened, per row, after applying", async () => {
    bulk.plan = {
      op: "adopt",
      applied: false,
      outcomes: [
        { user_id: "d1", name: "Ada", email: "u1", effect: "apply", detail: "Adopted." },
        { user_id: "d2", name: "Sam", email: "u2", effect: "apply", detail: "Adopted." },
      ],
      plan_id: "plan_1",
      summary: { total: 2, apply: 2, no_change: 0, blocked: 0, failed: 0, succeeded: 0, queued: 0 },
    };
    selectTwo();
    fireEvent.click(screen.getByRole("button", { name: "Adopt in Syndra" }));
    fireEvent.click(await screen.findByRole("button", { name: "Apply to 2 items" }));

    // The result is a diff against the plan that was approved, not a fresh
    // document with no relationship to it.
    await waitFor(() => expect(bulk.applies).toBe(1));
    expect(await screen.findByRole("button", { name: "Close" })).toBeInTheDocument();
  });
});

// A finding on a target Syndra holds no role catalogue for is not a retired
// role — nothing was retired, there was never a catalogue to retire it from.
// The pill said otherwise for every add-on row, which is the loudest badge in
// the queue applied to the one thing it cannot mean.
it("does not call an add-on role retired", () => {
  drift.data = [
    item({
      target: "truenas",
      role_catalogue_applies: false,
      role_in_catalogue: false,
      role_keys: ["tank/projects:rw"],
    }),
  ];
  renderTriage();
  expect(screen.queryByText("Role not in catalogue")).not.toBeInTheDocument();

});

// A grant somebody removed by hand is the SAME entitlement Syndra applied,
// and the row has to say so. Told as "a queued write that never landed" — the
// sentence every syndra_only row used to get — it reads as a stranger, and
// the operator's next move is wrong.
it("tells a removal as the history of the grant Syndra applied", () => {
  drift.data = [
    item({
      drift_type: "syndra_only",
      upstream_actor: "op-marta",
      provenance: {
        granted_by: "op-ada",
        granted_at: "2026-08-03T09:00:00Z",
        reason: "inducted on the laser",
        last_observed_at: "2026-08-19T03:00:00Z",
      },
    }),
  ];
  renderTriage();

  expect(screen.getByText(/Granted by op-ada/i)).toBeTruthy();
  expect(screen.getByText(/inducted on the laser/i)).toBeTruthy();
  expect(screen.getByText(/does not now, so somebody removed it there/i)).toBeTruthy();
  expect(screen.getByText(/Removed by op-marta/i)).toBeTruthy();
});

// And one nobody ever saw the target holding keeps the old reading, which is
// the honest one for it: a write that never landed.
it("still calls an unobserved grant a write that never landed", () => {
  drift.data = [
    item({
      drift_type: "syndra_only",
      provenance: { granted_by: "op-ada", granted_at: "2026-08-03T09:00:00Z" },
    }),
  ];
  renderTriage();

  expect(screen.getByText(/never been seen holding it/i)).toBeTruthy();
});

// The strongest thing Syndra can say, and the only thing it can say about a
// grant applied and removed between two sweeps: the target ACCEPTED this write,
// at a known time. No read ever saw that one.
it("tells a removal by when the write landed, even with nothing ever observed", () => {
  drift.data = [
    item({
      drift_type: "syndra_only",
      provenance: {
        granted_by: "op-ada",
        reason: "inducted on the laser",
        applied_at: "2026-08-19T12:04:00Z",
      },
    }),
  ];
  renderTriage();

  expect(screen.getByText(/Syndra applied it on/i)).toBeTruthy();
  expect(screen.getByText(/accepted it/i)).toBeTruthy();
  expect(screen.getByText(/somebody removed it/i)).toBeTruthy();
});
