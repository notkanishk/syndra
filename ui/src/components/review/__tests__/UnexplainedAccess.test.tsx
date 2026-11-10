// @vitest-environment jsdom
import { QueryClient, QueryClientProvider, useMutation } from "@tanstack/react-query";
import { seedAutoRetry } from "@/test-utils/drain";
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { UnexplainedAccess } from "@/components/review/UnexplainedAccess";
import { ApiError } from "@/lib/api-client";
import type { DriftTriageItem } from "@/lib/queries/useDrift";

const drift = vi.hoisted(() => ({ data: [] as DriftTriageItem[] }));

// Who Syndra can name. An actor it cannot name is dropped from the sentence, never shown as an id.
const NAMES = vi.hoisted(() => ({ current: {} as Record<string, string> }));
vi.mock("@/lib/queries/useNameResolver", () => ({
  useNameResolver: () => ({
    resolveUser: (id: string) => ({
      value: NAMES.current[id] ? { display_name: NAMES.current[id] } : undefined,
      resolved: true,
    }),
    resolveProject: () => ({ value: undefined, resolved: true }),
  }),
}));

// The per-row origin read, asked only when somebody clicks.
const origin = vi.hoisted(() => ({
  data: undefined as Record<string, unknown> | undefined,
  isLoading: false,
  isError: false,
  asks: 0,
}));

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

// Zitadel not answering, as the shell's one Notice decides it.
const zitadel = vi.hoisted(() => ({ down: false }));
vi.mock("@/components/states/DegradedBanner", () => ({ useDegraded: () => zitadel.down }));

vi.mock("next/navigation", () => ({
  useSearchParams: () => new URLSearchParams(""),
  useRouter: () => ({ replace: () => {} }),
}));

// Controls what the single-item Revoke confirm does, for the "cannot be
// pressed twice" tests: how many times it actually ran, and whether it fails.
const revokeCtl = vi.hoisted(() => ({
  calls: 0,
  fails: false,
  // A promise the revoke waits on, so a test can look at the first 100ms.
  hold: null as null | Promise<void>,
  waits: false,
  // What the server answered: a wait_reason when the revoke was held.
  answer: {} as Record<string, unknown>,
  // What the last revoke was sent: the plan it was shown (task 1.9).
  sent: undefined as unknown,
}));
// Which roles are Roles that wait; the shared setup answers "none".
vi.mock("@/lib/queries/useRoleWaits", () => ({ useRoleWaits: () => () => revokeCtl.waits }));

// What adopting does, from the planner (GET /users/{id}/plan?op=adopt): the
// rules' sentences, a plan that couldn't be read, what it was asked; and what
// the adoption's write was sent and answered.
const ADOPT_SAID = "Records a direct grant, so the access stops being unexplained and starts being owned. Nothing is sent to Zitadel for it.";
const adoptCtl = vi.hoisted(() => ({
  rules: [] as string[],
  answer: {} as Record<string, unknown>,
  error: false,
  refetch: vi.fn(),
  asked: [] as unknown[],
  writes: [] as unknown[],
  reject: null as unknown,
}));
vi.mock("@/lib/usePlanConsent", async (importOriginal) => {
  const original = await importOriginal<typeof import("@/lib/usePlanConsent")>();
  const { actPlan } = await import("@/test-utils/plan");
  return {
    ...original,
    usePlanConsent: (action: { op: string } | null, doing: string) => {
      if (action) adoptCtl.asked.push(action);
      const failed = Boolean(action) && adoptCtl.error;
      const plan =
        action && !failed
          ? actPlan({ detail: "Adopted into Syndra (operator).", said: [ADOPT_SAID, ...adoptCtl.rules], rules: adoptCtl.rules })
          : undefined;
      const words = original.planWords(doing);
      return {
        plan,
        rules: plan?.rules ?? [],
        cancels: plan?.cancels ?? [],
        seen: original.seenOf(plan),
        hold: failed ? words.held : undefined,
        checking: false,
        failed,
        unread: words.unread,
        retry: adoptCtl.refetch,
      };
    },
  };
});

vi.mock("@/lib/queries/useDrift", () => ({
  useDriftItems: () => ({ ...drift, isLoading: false, error: null, refetch: () => {} }),
  useAttributeDrift: () => ({
    mutateAsync: async (args: unknown) => {
      adoptCtl.writes.push(args);
      if (adoptCtl.reject) throw adoptCtl.reject;
      return adoptCtl.answer;
    },
    isPending: false,
  }),
  useRevokeDrift: () =>
    useMutation({
      mutationFn: async (args: unknown) => {
        revokeCtl.calls += 1;
        revokeCtl.sent = args;
        if (revokeCtl.hold) await revokeCtl.hold;
        if (revokeCtl.fails) throw new Error("revoke failed");
        return revokeCtl.answer;
      },
    }),
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
  useDriftOrigin: (_id: string, enabled: boolean) => {
    if (enabled) origin.asks += 1;
    return enabled
      ? { data: origin.data, isLoading: origin.isLoading, isError: origin.isError }
      : { data: undefined, isLoading: false, isError: false };
  },
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

/**
 * Selection is a mode now, announced by a named control, so a test that wants
 * a checkbox has to ask for one the way an operator does. That is the point of
 * the change: nothing about a row at rest suggests it is selectable.
 */
function enterSelect() {
  fireEvent.click(screen.getByRole("button", { name: "Select" }));
}

function renderTriage() {
  const client = seedAutoRetry(new QueryClient({ defaultOptions: { queries: { retry: false } } }), true);
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
  revokeCtl.calls = 0;
  revokeCtl.fails = false;
  revokeCtl.hold = null;
  revokeCtl.waits = false;
  revokeCtl.answer = {};
  adoptCtl.rules = [];
  adoptCtl.answer = { status: "attributed", rule_outbox_ids: [], rule_waiting: 0 };
  adoptCtl.error = false;
  adoptCtl.refetch = vi.fn();
  adoptCtl.asked = [];
  adoptCtl.writes = [];
  adoptCtl.reject = null;
  NAMES.current = {};
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

  // The board retires the red box under the list: it reports no failure, and
  // the bar says it where somebody selecting rows would look for Revoke.
  it("states that bulk revoke is deliberately absent, in the bar, rather than leaving a hole", () => {
    renderTriage();
    expect(screen.queryByText(/You can adopt several rows at once/)).not.toBeInTheDocument();
    enterSelect();
    fireEvent.click(screen.getAllByRole("checkbox")[1]);
    expect(
      within(screen.getByRole("region", { name: "Selection" })).getByText(
        "Revoking is one row at a time.",
      ),
    ).toBeInTheDocument();
  });

  it("offers bulk adopt and bulk mark-external once rows are selected — and never bulk revoke", () => {
    renderTriage();
    enterSelect();
    fireEvent.click(screen.getAllByRole("checkbox")[1]);

    // Both verbs name the next step — a plan — rather than the write at the end
    // of it, because tapping either one resolves nothing on its own.
    const bar = screen.getByRole("region", { name: "Selection" });
    expect(within(bar).getByRole("button", { name: "Preview adopting these" })).toBeInTheDocument();
    expect(
      within(bar).getByRole("button", { name: "Preview marking these owned elsewhere" }),
    ).toBeInTheDocument();
    expect(within(bar).queryByRole("button", { name: /Revoke/ })).not.toBeInTheDocument();
  });

  it("says nothing about selecting until the named control is tapped", () => {
    renderTriage();
    expect(screen.queryAllByRole("checkbox")).toHaveLength(0);
    enterSelect();
    expect(screen.getAllByRole("checkbox").length).toBeGreaterThan(0);
  });

  it("has a select-all that covers the whole queue, not the rendered page", () => {
    drift.data = Array.from({ length: 20 }, (_, index) =>
      item({ id: `d${index}`, user_id: `u${index}` }),
    );
    renderTriage();
    enterSelect();

    // The queue pages at 12, but the queue is what you are triaging.
    fireEvent.click(screen.getByRole("checkbox", { name: /Select these 20 items/ }));
    expect(screen.getByText(/20 items selected/)).toBeInTheDocument();
  });

  it("extends a range on shift-click instead of making you tick each row", () => {
    drift.data = Array.from({ length: 5 }, (_, index) =>
      item({ id: `d${index}`, user_id: `u${index}` }),
    );
    renderTriage();
    enterSelect();

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
    enterSelect();
    fireEvent.click(screen.getByRole("checkbox", { name: /Select these/ }));

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
    // amount of shift-clicking finds the cluster as reliably as asking for it. The ask is in the
    // selection bar, from the row that is ticked, not a text button on every row.
    expect(screen.queryByRole("button", { name: "Select similar rows" })).toBeNull();
    enterSelect();
    fireEvent.click(screen.getAllByRole("checkbox", { name: /Select the operator row/ })[0]);
    fireEvent.click(
      within(screen.getByRole("region", { name: "Selection" })).getByRole("button", { name: "Select similar rows" }),
    );
    expect(screen.getByText(/3 items selected/)).toBeInTheDocument();
  });

  it("names the upstream actor and date when the detector knew them", () => {
    NAMES.current = { "svc-badge-sync": "Badge sync" };
    drift.data = [
      item({ upstream_actor: "svc-badge-sync", upstream_created_at: "2026-07-21T14:12:04Z" }),
    ];
    renderTriage();
    expect(screen.getByText(/Created in Zitadel on 21 \w+ by Badge sync\./)).toBeInTheDocument();
  });

  // A raw id is never a name (A-17 G10): an actor Syndra cannot name is dropped with its "by".
  it("drops the 'by' for an actor it cannot name, and never shows the id", () => {
    drift.data = [
      item({ upstream_actor: "365914614526903302", upstream_created_at: "2026-07-21T14:12:04Z" }),
    ];
    renderTriage();
    expect(screen.getByText(/^Created in Zitadel on 21 \w+\.$/)).toBeInTheDocument();
    expect(screen.queryByText(/365914614526903302/)).toBeNull();
  });

  // What Syndra gives nothing for is said once, as the section's note, not on every row.
  it("says what nothing in Syndra gives once, in the section's note", () => {
    drift.data = [item({ id: "a" }), item({ id: "b", user_id: "u2" })];
    renderTriage();
    expect(screen.getAllByText(/no direct access, no bundle, no automatic rule/)).toHaveLength(1);
  });

  it("admits it does not know the actor rather than naming a plausible one", () => {
    renderTriage();
    expect(
      screen.getByText(/compares lists and cannot see who made the change/),
    ).toBeInTheDocument();
  });

  it("neutralises Adopt for a service account instead of hiding it", () => {
    drift.data = [item({ user_is_service_account: true })];
    renderTriage();
    expect(screen.getByRole("button", { name: "Adopt" })).toBeDisabled();
    // Still the accent act, with its reason beside it, never a ghost that looks like nothing.
    expect(screen.getByRole("button", { name: "Adopt" })).toHaveClass("bg-accent-dense/25");
    expect(screen.getByText(/re-creates its own access on every deploy/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Owned elsewhere" })).toBeEnabled();
  });

  // A-07 §29: Drift keeps service accounts, and the name opens the account's own page.
  it("links a service account's name to its page and a person's to theirs", () => {
    drift.data = [item({ id: "a", user_id: "svc", user_is_service_account: true }), item({ id: "b", user_id: "u2" })];
    renderTriage();
    const hrefs = screen.getAllByRole("link").map((a) => a.getAttribute("href"));
    expect(hrefs).toContain("/system/service-accounts/svc");
    expect(hrefs).toContain("/users/u2");
    expect(hrefs).not.toContain("/users/svc");
  });

  it("says a role is missing from the catalogue, in words as well as colour", () => {
    drift.data = [item({ role_in_catalogue: false })];
    renderTriage();
    expect(screen.getByText("Role unknown to Syndra")).toBeInTheDocument();
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
    // The row toggle, by its own hook. Matching on "a collapsed disclosure
    // that does not describe itself" was a heuristic about every OTHER
    // disclosure on the page, so it broke once when the lede gained a <Term>
    // and again when the filters became a panel.
    fireEvent.click(screen.getAllByTestId("drift-row-toggle")[0]);
    expect(screen.getByText("zg_4f19c8")).toBeInTheDocument();
  });

  // "Made by" used to render `item.upstream_actor` — Zitadel's raw editor id —
  // as the entire value, with no name resolution at all. It is a Zitadel user
  // id like any other, so it goes through the same UserName component the
  // rest of the row uses, and its own "cannot resolve" fallback labels the id
  // instead of leaving it to stand alone.
  it("does not show the evidence panel's 'Made by' as a bare id", () => {
    drift.data = [item({ zitadel_grant_id: "zg_1", upstream_actor: "228719872436817921" })];
    renderTriage();

    fireEvent.click(screen.getAllByTestId("drift-row-toggle")[0]);

    // Scoped to the evidence row itself — the row's holder (also unresolved
    // in this test environment) would otherwise give a second "Unknown
    // account" and make a plain getByText ambiguous.
    const madeByLabel = screen.getByText("Made by");
    expect(madeByLabel.nextElementSibling?.textContent).toMatch(/Unknown account/);
  });
});

describe("Unexplained access — bulk resolution is rehearsed", () => {
  function selectTwo() {
    drift.data = [item({ id: "d1" }), item({ id: "d2", user_id: "u2" })];
    renderTriage();
    enterSelect();
    fireEvent.click(screen.getAllByRole("checkbox")[1]);
    fireEvent.click(screen.getAllByRole("checkbox")[2]);
  }

  it("shows what would happen before anything is written", async () => {
    bulk.plan = {
      op: "adopt",
      applied: false,
      outcomes: [
        { user_id: "d1", name: "Charlotte Flair", email: "u1", effect: "apply", detail: "Adopted into Syndra (trained)." },
        { user_id: "d2", name: "Rey Mysterio", email: "u2", effect: "no_change", detail: "Already resolved as adopted." },
      ],
      plan_id: "plan_1",
      summary: { total: 2, apply: 1, no_change: 1, blocked: 0, failed: 0, succeeded: 0, queued: 0 },
    };
    selectTwo();
    fireEvent.click(screen.getByRole("button", { name: "Preview adopting these" }));

    // Rows are named, and the confirm button counts only what will change —
    // offering "Apply to 2" when one is already resolved would be a lie.
    expect(await screen.findByText("Charlotte Flair")).toBeInTheDocument();
    expect(screen.getByText(/Already resolved as adopted/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Apply to 1 item" })).toBeInTheDocument();
    expect(bulk.applies).toBe(0);
  });

  it("does not write until the plan is confirmed", async () => {
    bulk.plan = {
      op: "adopt",
      applied: false,
      outcomes: [{ user_id: "d1", name: "Charlotte", email: "u1", effect: "apply", detail: "Adopted." }],
      plan_id: "plan_1",
      summary: { total: 1, apply: 1, no_change: 0, blocked: 0, failed: 0, succeeded: 0, queued: 0 },
    };
    selectTwo();
    fireEvent.click(screen.getByRole("button", { name: "Preview adopting these" }));

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
        { user_id: "d1", name: "Charlotte", email: "u1", effect: "no_change", detail: "Already resolved." },
      ],
      plan_id: "plan_1",
      summary: { total: 1, apply: 0, no_change: 1, blocked: 0, failed: 0, succeeded: 0, queued: 0 },
    };
    selectTwo();
    fireEvent.click(screen.getByRole("button", { name: "Preview adopting these" }));

    // The label stays an ACTION at every count — a button named after its own
    // emptiness reads as broken rather than as waiting. What it would do to
    // nobody is said in the reason beside it, not in the label.
    expect(await screen.findByRole("button", { name: "Apply" })).toBeDisabled();
    expect(screen.getByText(/nothing to apply/i)).toBeInTheDocument();
  });

  it("reports what actually happened, per row, after applying", async () => {
    bulk.plan = {
      op: "adopt",
      applied: false,
      outcomes: [
        { user_id: "d1", name: "Charlotte", email: "u1", effect: "apply", detail: "Adopted." },
        { user_id: "d2", name: "Rey", email: "u2", effect: "apply", detail: "Adopted." },
      ],
      plan_id: "plan_1",
      summary: { total: 2, apply: 2, no_change: 0, blocked: 0, failed: 0, succeeded: 0, queued: 0 },
    };
    selectTwo();
    fireEvent.click(screen.getByRole("button", { name: "Preview adopting these" }));
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
  expect(screen.queryByText("Role unknown to Syndra")).not.toBeInTheDocument();

});

// A grant somebody removed by hand is the SAME entitlement Syndra applied,
// and the row has to say so. Told as "a queued write that never landed" — the
// sentence every syndra_only row used to get — it reads as a stranger, and
// the operator's next move is wrong.
it("tells a removal as the history of the grant Syndra applied", () => {
  NAMES.current = { "op-molly": "Molly Holly", "op-lacey": "Lacey Evans" };
  drift.data = [
    item({
      drift_type: "syndra_only",
      upstream_actor: "op-lacey",
      provenance: {
        granted_by: "op-molly",
        granted_at: "2026-08-03T09:00:00Z",
        reason: "inducted on the laser",
        last_observed_at: "2026-08-19T03:00:00Z",
      },
    }),
  ];
  renderTriage();

  expect(screen.getByText(/Given by Molly Holly/i)).toBeTruthy();
  expect(screen.getByText(/inducted on the laser/i)).toBeTruthy();
  expect(screen.getByText(/does not now, so somebody removed it there/i)).toBeTruthy();
  expect(screen.getByText(/Removed by Lacey Evans/i)).toBeTruthy();
});

// And one nobody ever saw the target holding keeps the old reading, which is
// the honest one for it: a write that never landed.
it("still calls an unobserved grant a write that never landed", () => {
  drift.data = [
    item({
      drift_type: "syndra_only",
      provenance: { granted_by: "op-molly", granted_at: "2026-08-03T09:00:00Z" },
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
        granted_by: "op-molly",
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

/**
 * `boundBulkIDs` refuses past 500 on both drift bulk routes. The bar has said
 * so since the selection work, but only the People page was passing it a
 * ceiling — so on this queue an operator select-alled, tapped, and met the
 * refusal afterwards. Which is the thing the bar exists to prevent.
 */
describe("the queue states its ceiling before the tap", () => {
  it("names the limit and offers the narrowing move over 500", () => {
    drift.data = Array.from({ length: 501 }, (_, at) =>
      item({ id: `d${at}`, user_id: `u${at}` }),
    );
    renderTriage();

    fireEvent.click(screen.getByRole("button", { name: "Select" }));
    fireEvent.click(screen.getByRole("checkbox", { name: /Select these 501 items/ }));

    expect(screen.getByText(/you can change at most 500 items at once/)).toBeTruthy();
  });

  it("leaves a selection inside the ceiling alone", () => {
    drift.data = Array.from({ length: 3 }, (_, at) => item({ id: `d${at}`, user_id: `u${at}` }));
    renderTriage();

    fireEvent.click(screen.getByRole("button", { name: "Select" }));
    fireEvent.click(screen.getByRole("checkbox", { name: /Select these 3 items/ }));

    expect(screen.queryByText(/you can change at most/)).toBeNull();
  });
});

/**
 * "Who made it" — the question the sweep cannot answer for itself.
 *
 * A sweep row compares grant SETS, and a set difference has no author. Zitadel
 * is event-sourced, so the answer is recorded rather than inferred — which is
 * why this side reads the log instead of keeping the merge base the add-on
 * targets need. These assert the three answers stay distinct, and that the
 * queue does not ask until somebody does.
 */
describe("who made it", () => {
  beforeEach(() => {
    origin.data = undefined;
    origin.isLoading = false;
    origin.isError = false;
    origin.asks = 0;
    drift.data = [item({ id: "d1", zitadel_grant_id: "g-1", upstream_actor: undefined })];
  });

  it("does not ask the identity provider until somebody asks", () => {
    renderTriage();
    expect(screen.getByRole("button", { name: "Who made it?" })).toBeTruthy();
    // Dozens of rows on a real queue; one call each on load would be a burst
    // to answer a question nobody put.
    expect(origin.asks).toBe(0);
  });

  it("names the actor once asked", async () => {
    origin.data = {
      id: "d1", readable: true, recorded: true, attributed: true,
      actor_name: "Bianca Belair", service: "Management-API", at: "2026-08-03T09:25:46Z",
    };
    renderTriage();
    fireEvent.click(screen.getByRole("button", { name: "Who made it?" }));

    await waitFor(() => expect(screen.getByText(/Bianca Belair/)).toBeInTheDocument());
    expect(screen.getByText(/Management-API/)).toBeInTheDocument();
  });

  it("does not turn a failed lookup into 'nobody made it'", async () => {
    origin.data = { id: "d1", readable: false, detail: "zitadel api 503" };
    renderTriage();
    fireEvent.click(screen.getByRole("button", { name: "Who made it?" }));

    await waitFor(() =>
      expect(screen.getByText(/not known who/i)).toBeInTheDocument(),
    );
  });

  it("separates a short log from an anonymous event", async () => {
    origin.data = { id: "d1", readable: true, recorded: false };
    const { unmount } = renderTriage();
    fireEvent.click(screen.getByRole("button", { name: "Who made it?" }));
    await waitFor(() => expect(screen.getByText(/does not go back/i)).toBeInTheDocument());
    unmount();

    origin.data = { id: "d1", readable: true, recorded: true, attributed: false };
    renderTriage();
    fireEvent.click(screen.getByRole("button", { name: "Who made it?" }));
    await waitFor(() =>
      expect(screen.getByText(/recorded the change and not who made it/i)).toBeInTheDocument(),
    );
  });

  it("offers nothing where there is nothing to read", () => {
    // Already attributed by the webhook, and a row naming no grant.
    drift.data = [
      item({ id: "d1", zitadel_grant_id: "g-1", upstream_actor: "someone@example.org" }),
      item({ id: "d2", zitadel_grant_id: undefined }),
    ];
    renderTriage();
    expect(screen.queryByRole("button", { name: "Who made it?" })).toBeNull();
  });
});

describe("the revoke confirm, once it has run", () => {
  // Task 1.9: a drift revoke is a planned act. The sheet reads its plan
  // (op=revoke on the drift row) and the write carries the fingerprint and
  // cancel ids it was shown, so the server writes only that Change.
  it("reads the revoke's plan and sends the plan it was shown", async () => {
    drift.data = [item({ id: "d1" })];
    renderTriage();
    fireEvent.click(screen.getByRole("button", { name: "Revoke" }));
    expect(adoptCtl.asked).toContainEqual({ op: "revoke", userId: "u1", driftId: "d1" });

    const dialog = screen.getByRole("dialog");
    fireEvent.click(within(dialog).getByRole("button", { name: "Revoke this role" }));
    await waitFor(() => expect(revokeCtl.calls).toBe(1));
    expect(revokeCtl.sent).toEqual({ id: "d1", body: { fingerprint: "fp-1", cancel_ids: [] } });
  });

  it("cannot be pressed a second time on the highest-stakes screen", async () => {
    drift.data = [item({ id: "d1" })];
    renderTriage();
    fireEvent.click(screen.getByRole("button", { name: "Revoke" }));

    const dialog = screen.getByRole("dialog");
    fireEvent.click(within(dialog).getByRole("button", { name: "Revoke this role" }));

    await waitFor(() =>
      expect(within(dialog).getByRole("button", { name: "Close" })).toBeInTheDocument(),
    );
    // Gone, not just relabelled — a second click must have nothing left to hit.
    expect(within(dialog).queryByRole("button", { name: "Revoke this role" })).toBeNull();
    expect(revokeCtl.calls).toBe(1);
  });

  // A-19 P14. Code decides (the revocation drain claims op_type "revoke" with no
  // operator in the loop), so the result says that one thing: no amber dot, no
  // "Waiting for you to send" link, no second line saying it has not arrived.
  it("says one thing: a queued revoke is on its way and sends on its own", async () => {
    drift.data = [item({ id: "d1" })];
    renderTriage();
    fireEvent.click(screen.getByRole("button", { name: "Revoke" }));

    const dialog = screen.getByRole("dialog");
    fireEvent.click(within(dialog).getByRole("button", { name: "Revoke this role" }));

    const result = await within(dialog).findByRole("status");
    expect(result.textContent).toContain("On its way");
    expect(result.textContent).toContain("Revocations send on their own");
    expect(result.textContent).not.toMatch(/waiting for you/i);
    expect(result.textContent).not.toMatch(/has not reached/i);
    expect(within(dialog).queryByRole("link", { name: /Waiting for you to send/ })).toBeNull();
    // A neutral dot, never the amber of a step left.
    expect(result.querySelector(".bg-warn")).toBeNull();
    expect(result.querySelector(".bg-faint")).not.toBeNull();
  });

  // Audit C5: a role that waits holds the revoke, as the forecast said; the
  // result says it waits for the operator, never "On its way".
  it("a revoke on a role that waits is not sent, and says it waits for you", async () => {
    revokeCtl.waits = true;
    revokeCtl.answer = { status: "revoked", outbox_id: "o1", wait_reason: "role_waits" };
    drift.data = [item({ id: "d1" })];
    renderTriage();
    fireEvent.click(screen.getByRole("button", { name: "Revoke" }));

    const dialog = screen.getByRole("dialog");
    expect(within(dialog).getByText(/once you send it, because operator always waits for you/)).toBeInTheDocument();
    fireEvent.click(within(dialog).getByRole("button", { name: "Revoke this role" }));

    const result = await within(dialog).findByRole("status");
    expect(result.textContent).toContain("Waiting for you to send");
    expect(result.textContent).toContain("operator always waits for you.");
    expect(result.textContent).not.toContain("On its way");
  });

  // A-20 P8: the first 100ms of Drift's highest-stakes act, said in Syndra's own tense.
  describe("forecast", () => {
    async function pressRevoke() {
      let release!: () => void;
      revokeCtl.hold = new Promise<void>((resolve) => (release = resolve));
      drift.data = [item({ id: "d1" })];
      renderTriage();
      fireEvent.click(screen.getByRole("button", { name: "Revoke" }));
      const dialog = screen.getByRole("dialog");
      fireEvent.click(within(dialog).getByRole("button", { name: "Revoke this role" }));
      return { dialog, release };
    }

    it("says 'Will be revoked' while it is out, then crosses to the true word and drops the forecast", async () => {
      const { dialog, release } = await pressRevoke();
      expect(await within(dialog).findByText("Will be revoked")).toBeInTheDocument();
      // No outcome has been claimed ahead of the server.
      expect(within(dialog).queryByText("On its way")).toBeNull();
      release();
      await within(dialog).findByText("On its way");
      expect(within(dialog).queryByText("Will be revoked")).toBeNull();
    });

    it("says 'Will wait for you to send' for a role that waits, never 'Will be revoked'", async () => {
      revokeCtl.waits = true;
      const { dialog, release } = await pressRevoke();
      expect(await within(dialog).findByText("Will wait for you to send")).toBeInTheDocument();
      expect(within(dialog).queryByText("Will be revoked")).toBeNull();
      release();
    });

    it("shows the honest error, with no forecast left behind, when it fails", async () => {
      revokeCtl.fails = true;
      const { dialog, release } = await pressRevoke();
      expect(await within(dialog).findByText("Will be revoked")).toBeInTheDocument();
      release();
      await waitFor(() => expect(within(dialog).getByText(/Nothing was changed/i)).toBeInTheDocument());
      expect(within(dialog).queryByText("Will be revoked")).toBeNull();
    });
  });

  // A-22 §3.6: the sheet is its result once it has run.
  it("titles the act, then unmounts lede and What-happens list with the result", async () => {
    drift.data = [item({ id: "d1" })];
    renderTriage();
    fireEvent.click(screen.getByRole("button", { name: "Revoke" }));
    const dialog = screen.getByRole("dialog");
    expect(within(dialog).getByRole("heading", { name: "Revoke this role?" })).toBeInTheDocument();
    expect(within(dialog).getByRole("button", { name: "Keep it" })).toBeInTheDocument();
    expect(dialog.querySelector(".bg-tint-1")).toBeNull();
    fireEvent.click(within(dialog).getByRole("button", { name: "Revoke this role" }));
    await within(dialog).findByRole("status");
    expect(within(dialog).queryByText(/They will lose this role/)).toBeNull();
    expect(within(dialog).queryByText(/records your decision in Syndra/)).toBeNull();
    expect(within(dialog).queryByRole("button", { name: "Keep it" })).toBeNull();
    expect(within(dialog).getByRole("heading", { name: "Revoke this role?" })).toBeInTheDocument();
  });

  it("a failed revoke says Not revoked and keeps the form", async () => {
    revokeCtl.fails = true;
    drift.data = [item({ id: "d1" })];
    renderTriage();
    fireEvent.click(screen.getByRole("button", { name: "Revoke" }));
    const dialog = screen.getByRole("dialog");
    fireEvent.click(within(dialog).getByRole("button", { name: "Revoke this role" }));
    await waitFor(() => expect(within(dialog).getByText("Not revoked")).toBeInTheDocument());
    expect(within(dialog).getByText(/They will lose this role/)).toBeInTheDocument();
  });

  it("keeps the confirm button when the revoke fails, so there is a retry", async () => {
    revokeCtl.fails = true;
    drift.data = [item({ id: "d1" })];
    renderTriage();
    fireEvent.click(screen.getByRole("button", { name: "Revoke" }));

    const dialog = screen.getByRole("dialog");
    fireEvent.click(within(dialog).getByRole("button", { name: "Revoke this role" }));

    await waitFor(() => expect(within(dialog).getByText(/Nothing was changed/i)).toBeInTheDocument());
    expect(within(dialog).getByRole("button", { name: "Revoke this role" })).toBeInTheDocument();
    expect(within(dialog).queryByRole("button", { name: "Close" })).toBeNull();
  });

  it("opens with Keep it focused, not the destructive Revoke this role button", () => {
    drift.data = [item({ id: "d1" })];
    renderTriage();
    fireEvent.click(screen.getByRole("button", { name: "Revoke" }));

    const dialog = screen.getByRole("dialog");
    expect(within(dialog).getByRole("button", { name: "Keep it" })).toHaveFocus();
  });

  it("doesn't claim the grant leaves Zitadel on confirm when the result says it only queues", () => {
    // The pre-confirm copy used to say "removed from Zitadel as you confirm" —
    // a promise the outcome directly underneath it (queued, sends on its own)
    // contradicts. Both halves of the dialog must agree on the same tense.
    drift.data = [item({ id: "d1" })];
    renderTriage();
    fireEvent.click(screen.getByRole("button", { name: "Revoke" }));

    const dialog = screen.getByRole("dialog");
    expect(within(dialog).queryByText(/removed from Zitadel as you confirm/i)).toBeNull();
    expect(within(dialog).queryByText(/send on their own/i)).toBeNull();
    // The removal is said once, in the lede, and the result says the rest.
    expect(within(dialog).getAllByText(/removes the access in Zitadel/i)).toHaveLength(1);
  });
});

// Audit C1: adopting is when a rule starting from the role fires, so the
// dialog names what the rules give before the operator confirms, and its copy
// never says the access stays exactly as it is when rules add roles.
describe("the adopt confirm, with automatic rules", () => {
  function openAdopt() {
    drift.data = [item({ id: "d1" })];
    renderTriage();
    fireEvent.click(screen.getByRole("button", { name: "Adopt" }));
    return screen.getByRole("dialog");
  }

  // Task 1.3: one source for what rules add, the planner's; the write carries the plan.
  it("names what adopting does and what the rules give, in the planner's words, and writes the plan back", async () => {
    adoptCtl.rules = ["Also gives Bench in Workshop, by the automatic rule from Operator."];
    adoptCtl.answer = { status: "attributed", rule_outbox_ids: ["o2"], rule_waiting: 0 };
    const dialog = openAdopt();
    expect(adoptCtl.asked).toContainEqual({ op: "adopt", userId: "u1", driftId: "d1" });
    expect(within(dialog).getByText(ADOPT_SAID)).toBeInTheDocument();
    expect(within(dialog).getByText("Also gives Bench in Workshop, by the automatic rule from Operator.")).toBeInTheDocument();
    expect(dialog.textContent).toMatch(/stays exactly as it is/);
    expect(dialog.textContent).not.toMatch(/Pending changes, for you to send/);

    fireEvent.click(within(dialog).getByRole("button", { name: "Adopt in Syndra" }));
    const result = await within(dialog).findByRole("status");
    expect(adoptCtl.writes).toEqual([{ id: "d1", body: { source: "external_backfill", fingerprint: "fp-1", cancel_ids: [] } }]);
    // What the rules gave was sent straight away: nothing waits, so nothing is said to.
    expect(result.textContent).not.toMatch(/wait/);
    expect(result.textContent).toContain("Nothing for you to do.");
  });

  it("says what the rules left waiting only when some of it waits for you", async () => {
    adoptCtl.rules = ["Also gives Bench in Workshop, by the automatic rule from Operator."];
    adoptCtl.answer = { status: "attributed", rule_outbox_ids: ["o2"], rule_waiting: 1, wait_reason: "role_waits" };
    const dialog = openAdopt();
    fireEvent.click(within(dialog).getByRole("button", { name: "Adopt in Syndra" }));
    const result = await within(dialog).findByRole("status");
    expect(result.textContent).toContain("1 more change from automatic rules waits in Pending changes for you to send.");
  });

  it("holds the adopt when the plan can't be read, and Try again reads it again", () => {
    adoptCtl.error = true;
    const dialog = openAdopt();
    expect(within(dialog).getByRole("button", { name: "Adopt in Syndra" })).toBeDisabled();
    expect(within(dialog).getByText(/Syndra couldn.t read what adopting it does/)).toBeInTheDocument();
    fireEvent.click(within(dialog).getByRole("button", { name: "Try again" }));
    expect(adoptCtl.refetch).toHaveBeenCalledTimes(1);
  });

  it("their access moved before the write: nothing changed, it says so and reads the plan again", async () => {
    adoptCtl.reject = new ApiError(409, { error: "PLAN_STALE", message: "Nothing was changed." });
    const dialog = openAdopt();
    fireEvent.click(within(dialog).getByRole("button", { name: "Adopt in Syndra" }));
    expect(await within(dialog).findByText(/access moved while this was open\./)).toBeInTheDocument();
    expect(within(dialog).getByText(/press Adopt in Syndra again/)).toBeInTheDocument();
    expect(adoptCtl.refetch).toHaveBeenCalled();
    expect(within(dialog).getByRole("button", { name: "Adopt in Syndra" })).toBeEnabled();
  });

  it("a failed adopt says Not adopted and the form keeps its button", async () => {
    adoptCtl.reject = new ApiError(500, { error: "BOOM", message: "boom" });
    const dialog = openAdopt();
    fireEvent.click(within(dialog).getByRole("button", { name: "Adopt in Syndra" }));
    expect(await within(dialog).findByText("Not adopted")).toBeInTheDocument();
    expect(within(dialog).queryByText("Adopted")).toBeNull();
    expect(within(dialog).getByRole("button", { name: "Adopt in Syndra" })).toBeEnabled();
  });

  it("says the access stays exactly as it is when no rule follows", async () => {
    const dialog = openAdopt();
    expect(dialog.textContent).toMatch(/stays exactly as it is/);
    expect(within(dialog).queryByText(/Also gives/)).toBeNull();
    fireEvent.click(within(dialog).getByRole("button", { name: "Adopt in Syndra" }));
    const result = await within(dialog).findByRole("status");
    expect(result.textContent).not.toMatch(/automatic rules/);
  });
});

// The board's degraded frame: last known rows stay readable, every answer waits.
// UI only: the buttons are disabled, no request is changed.
describe("Drift while Zitadel is not answering", () => {
  beforeEach(() => {
    drift.data = [item()];
  });

  it("disables the decisions and says they wait, in words", () => {
    zitadel.down = true;
    render(
      <QueryClientProvider client={seedAutoRetry(new QueryClient(), true)}>
        <UnexplainedAccess />
      </QueryClientProvider>,
    );
    for (const name of ["Adopt", "Revoke", "Owned elsewhere", "Check Zitadel again"]) {
      expect(screen.getByRole("button", { name })).toBeDisabled();
    }
    expect(screen.getByText(/wait until Zitadel answers/)).toBeInTheDocument();
    // The reasons stay readable.
    expect(screen.getByRole("button", { name: "Show evidence" })).not.toBeDisabled();
    zitadel.down = false;
  });

  it("leaves them live when Zitadel answers", () => {
    zitadel.down = false;
    render(
      <QueryClientProvider client={seedAutoRetry(new QueryClient(), true)}>
        <UnexplainedAccess />
      </QueryClientProvider>,
    );
    for (const name of ["Adopt", "Revoke", "Owned elsewhere"]) {
      expect(screen.getByRole("button", { name })).not.toBeDisabled();
    }
  });
});
