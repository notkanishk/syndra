// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { TargetOverview } from "@/components/targets/TargetOverview";
import type {
  AccountHome,
  TargetHealth,
  TargetInventory,
  TargetSetting,
  TargetSummary,
} from "@/lib/queries/useTargets";

const nameResolverMiss = { value: undefined, resolved: true } as const;

// Adoption asks an operator to type a raw Syndra ID with no picker — the one
// check available before an irreversible hand-off is showing whose name that
// ID actually resolves to, both while typing and in the result afterwards.
vi.mock("@/lib/queries/useNameResolver", () => ({
  useNameResolver: () => ({
    resolveUser: (id: string) =>
      id === "s-99" ? { value: { display_name: "Sai Khurana" }, resolved: true } : nameResolverMiss,
    resolveProject: () => nameResolverMiss,
    resolveRole: () => nameResolverMiss,
    resolveBundle: () => nameResolverMiss,
  }),
}));

// 9.2/9.20/9.21 — the operation set comes from the manifest, and the health
// states render as distinct things rather than as one "status".

const state = {
  roster: [] as TargetSummary[],
  health: {} as TargetHealth,
  inventory: {} as TargetInventory,
  resolved: [] as Array<{ head: string; note: string }>,
  reconcile: undefined as undefined | Record<string, unknown>,
  released: [] as string[],
  releaseResult: { status: "released" } as Record<string, unknown>,
  ownerDecided: [] as Array<{ id: string; owner: string; note: string }>,
  adopted: [] as Array<{ username: string; subjectId: string }>,
  adoptResult: { status: "adopted", detail: "The account is now bound to that person." } as unknown,
  projectSet: [] as string[],
  settings: { read: true, settings: [] as TargetSetting[] } as { read: boolean; settings?: TargetSetting[] },
  settingPut: [] as Array<{ name: string; value: string }>,
  homes: { read: true, usage_read: true, accounts: [] as AccountHome[] },
};

vi.mock("@/lib/queries/useUsers", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/queries/useUsers")>()),
  useUsers: (q: string) => ({
    data: q
      ? [
          { user: { id: "s-99", name: "Sai Khurana", email: "sai@example.edu", title: "Lab technician", team: "Design", status: "active", avatar: "" } },
          { user: { id: "s-12", name: "Kabir Rao", email: "kabir@example.edu", title: "Student", team: "", status: "active", avatar: "" } },
        ]
      : [],
    isLoading: false,
    error: null,
  }),
}));

vi.mock("@/lib/queries/useProjects", () => ({
  useProjects: () => ({
    data: [
      { project: { id: "p1", name: "Network storage", kind: "", description: "", roles: [{ key: "a", label: "A" }, { key: "b", label: "B" }] } },
      { project: { id: "p2", name: "Wood shop", kind: "", description: "", roles: [] } },
      { project: { id: "p3", name: "Laser Lab", kind: "", description: "", roles: [{ key: "op", label: "Operator" }] } },
    ],
    isLoading: false,
    error: null,
  }),
}));

vi.mock("@/lib/queries/useTargets", async () => {
  const actual = await vi.importActual<typeof import("@/lib/queries/useTargets")>(
    "@/lib/queries/useTargets",
  );
  return {
    ...actual,
    useTargets: () => ({ data: state.roster, isLoading: false, error: null }),
    useTargetHealth: () => ({ data: state.health, isLoading: false, error: null }),
    useTargetInventory: () => ({
      data: state.inventory,
      isLoading: false,
      error: null,
      refetch: vi.fn(),
    }),
    useAdoptAccount: () => ({
      mutate: (
        input: { username: string; subjectId: string },
        opts?: { onSuccess?: (r: unknown) => void },
      ) => {
        state.adopted.push(input);
        opts?.onSuccess?.(state.adoptResult);
      },
      isPending: false,
      error: null,
    }),
    useReleaseBinding: () => ({
      mutate: (subjectId: string, opts?: { onSuccess?: (r: unknown) => void }) => {
        state.released.push(subjectId);
        opts?.onSuccess?.(state.releaseResult);
      },
      isPending: false,
      error: null,
    }),
    useSetLifecycle: () => ({ mutate: vi.fn(), isPending: false }),
    useTargetSettings: () => ({ data: state.settings, isLoading: false, error: null, refetch: vi.fn() }),
    useTargetHomes: () => ({ data: state.homes, isLoading: false, error: null }),
    useSetTargetSetting: () => ({
      mutate: (input: { name: string; value: string }, opts?: { onSuccess?: () => void }) => {
        state.settingPut.push(input);
        opts?.onSuccess?.();
      },
      isPending: false,
      error: null,
    }),
    useSetTargetProject: () => ({
      mutate: (id: string, opts?: { onSuccess?: () => void }) => {
        state.projectSet.push(id);
        opts?.onSuccess?.();
      },
      isPending: false,
      error: null,
    }),
    useReconcileTarget: () => ({
      mutate: vi.fn(),
      isPending: false,
      data: state.reconcile,
      error: null,
    }),
    useResolveBindingConflict: () => ({
      mutate: (input: { id: string; owner: string; note: string }) => state.ownerDecided.push(input),
      isPending: false,
      error: null,
    }),
    useResolveLogFinding: () => ({
      mutate: (input: { head: string; note: string }) => state.resolved.push(input),
      isPending: false,
      error: null,
    }),
  };
});

function summary(operations: TargetSummary["operations"]): TargetSummary {
  return {
    target: "truenas",
    registered: true,
    auth_mode: "derived",
    callable: true,
    operations,
    circuit_open: false,
    project: { id: "p1", name: "Network storage" },
  };
}

function renderTarget() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <TargetOverview target="truenas" />
    </QueryClientProvider>,
  );
}

describe("one target's page", () => {
  it("never offers Syndra's own account for adoption, and says why", () => {
    // It is a real unmanaged account on the target, so it is listed. Adopting
    // it hands Syndra's own credential to a member; purging it deletes the
    // account Syndra reaches the target with. The add-on refuses both — this
    // says so before anybody meets the refusal.
    state.roster = [summary([])];
    state.health = { reachable: true, lifecycle: "active" };
    state.inventory = {
      target: "truenas",
      bound: 0,
      current: true,
      // A fresh read, because adoption is gated on one — without it the whole
      // list renders unadoptable and this test would pass for the wrong reason.
      read_at: new Date().toISOString(),
      unmanaged: [
        { username: "sai", uid: 3000 },
        { username: "syndra", uid: 3001, self: true },
      ],
    };
    renderTarget();

    expect(screen.getByText(/cannot be assigned to anyone/i)).toBeTruthy();
    // The ordinary account beside it is still adoptable, so the guard is not
    // just hiding the whole list.
    expect(screen.getAllByRole("button", { name: /assign to a person/i }).length).toBe(1);
  });

  it("warns when the key's expiry is unrecorded, and stays quiet when there is none", () => {
    // A key CAN expire without Syndra knowing, and the day it does the target
    // simply stops answering — which reads as an outage and sends an operator
    // to the NAS. An operator who states there is no expiry gets no warning.
    state.roster = [summary([])];
    state.inventory = { target: "truenas", bound: 0, unmanaged: [], current: true };

    state.health = { reachable: true, lifecycle: "active", key_expiry: "unrecorded" };
    const { unmount } = renderTarget();
    expect(screen.getByText(/expiry not recorded/i)).toBeTruthy();
    unmount();

    state.health = { reachable: true, lifecycle: "active", key_expiry: "none" };
    renderTarget();
    expect(screen.queryByText(/expiry not recorded/i)).toBeNull();
  });

  it("says SMB auditing is off before anyone runs an empty activity report", () => {
    state.roster = [summary([])];
    state.inventory = { target: "truenas", bound: 0, unmanaged: [], current: true };
    state.health = {
      reachable: true,
      lifecycle: "active",
      shares_readable: true,
      unaudited_shares: ["gitlab_data", "main"],
    };
    renderTarget();

    expect(screen.getByText(/auditing is off/i)).toBeTruthy();
    expect(screen.getByText("gitlab_data")).toBeTruthy();
  });

  it("does not claim auditing is fine when the share list could not be read", () => {
    state.roster = [summary([])];
    state.inventory = { target: "truenas", bound: 0, unmanaged: [], current: true };
    // shares_readable false: the difference between "nothing is unaudited" and
    // "could not look", which must not render identically.
    state.health = { reachable: true, lifecycle: "active", shares_readable: false };
    renderTarget();

    expect(screen.queryByText(/auditing is off/i)).toBeNull();
  });

  it("shows bindings that point at nothing, and says they were not converged", () => {
    // The sweep refuses to act on these: the plan for one says "create", so
    // acting would recreate an account somebody deleted.
    state.roster = [summary([])];
    state.inventory = { target: "truenas", bound: 0, unmanaged: [], current: true };
    state.health = { reachable: true, lifecycle: "active" };
    state.reconcile = {
      target: "truenas", bound: 4, queued: 1, current: true,
      stale: [{ subject_id: "s1", username: "alice", uid: 3999 }],
    };
    renderTarget();

    expect(screen.getByText(/recorded as owning an account that is no longer/i)).toBeTruthy();
    expect(screen.getByText("alice")).toBeTruthy();
    expect(screen.getByText(/yours to decide/i)).toBeTruthy();
  });

  // The surface named two resolutions — re-provision, or let it go — and could
  // reach neither. Letting go now exists; the first press must not do it, and
  // the sentence beside the second one has to say the account is not deleted,
  // because "forget" next to a row about a missing account reads as "remove".
  it("lets a stale binding go, and never on the first press", () => {
    state.released = [];
    state.releaseResult = { status: "released" };
    state.roster = [summary([])];
    state.inventory = { target: "truenas", bound: 0, unmanaged: [], current: true };
    state.health = { reachable: true, lifecycle: "active" };
    state.reconcile = {
      target: "truenas", bound: 4, queued: 1, current: true,
      stale: [{ subject_id: "s1", username: "alice", uid: 3999 }],
    };
    renderTarget();

    fireEvent.click(screen.getByText("Stop tracking this account"));
    expect(state.released).toEqual([]);
    expect(screen.getByText(/Nothing is deleted/i)).toBeTruthy();

    fireEvent.click(screen.getByText("Stop tracking alice"));
    expect(state.released).toEqual(["s1"]);
    // The row it acted on must stop saying it is unresolved. The reconcile
    // result is a mutation's answer, so nothing refetches it.
    expect(screen.getByText(/Stopped tracking it\. Nothing on TrueNAS was changed/i)).toBeTruthy();
    expect(screen.queryByText(/yours to decide/i)).toBeNull();
  });

  // 202 resolves like 200 — `request` rejects on non-2xx only — and the backend
  // answers 202 for two states that are not a release. Reading either as done
  // retires the only control that repairs it, and one of them is a binding the
  // add-on dropped while Syndra kept its own copy.
  it("does not call an unconfirmed release done, and keeps the control", () => {
    state.released = [];
    state.releaseResult = {
      status: "unconfirmed",
      detail: "The target did not confirm the release. Nothing was changed here.",
    };
    state.roster = [summary([])];
    state.inventory = { target: "truenas", bound: 0, unmanaged: [], current: true };
    state.health = { reachable: true, lifecycle: "active" };
    state.reconcile = {
      target: "truenas", bound: 4, queued: 1, current: true,
      stale: [{ subject_id: "s1", username: "alice", uid: 3999 }],
    };
    renderTarget();

    fireEvent.click(screen.getByText("Stop tracking this account"));
    fireEvent.click(screen.getByText("Stop tracking alice"));

    expect(screen.queryByText(/Stopped tracking it\. Nothing on TrueNAS was changed/i)).toBeNull();
    expect(screen.getByText(/did not confirm the release/i)).toBeTruthy();
    expect(screen.getByText("Stop tracking alice")).toBeTruthy();
  });

  // The split case. The add-on let go, Syndra's copy did not, and the answer
  // says pressing again repairs it — so the button that presses again has to
  // still be there, and the sentence has to be the backend's own.
  it("keeps a half-done release on screen with its repair", () => {
    state.released = [];
    state.releaseResult = {
      status: "released",
      warning:
        "The add-on released the binding and Syndra's own copy of it was not removed. Press release again to repair it.",
    };
    state.roster = [summary([])];
    state.inventory = { target: "truenas", bound: 0, unmanaged: [], current: true };
    state.health = { reachable: true, lifecycle: "active" };
    state.reconcile = {
      target: "truenas", bound: 4, queued: 1, current: true,
      stale: [{ subject_id: "s1", username: "alice", uid: 3999 }],
    };
    renderTarget();

    fireEvent.click(screen.getByText("Stop tracking this account"));
    fireEvent.click(screen.getByText("Stop tracking alice"));

    expect(screen.queryByText(/Stopped tracking it\. Nothing on TrueNAS was changed/i)).toBeNull();
    expect(screen.getByText(/Press release again to repair it/i)).toBeTruthy();
    expect(screen.getByText("Stop tracking alice")).toBeTruthy();
  });

  it("says a transport secret that stopped loading is a fault on THIS host", () => {
    // Above the reachability reading and worded away from the target on
    // purpose. An unreadable secret also makes the add-on look unreachable, and
    // an operator who reads "not answering" first drives to the NAS — the wrong
    // machine, and the one that takes longest to rule out.
    state.roster = [
      {
        ...summary([]),
        transport_status: "error",
        transport_error: "read ADDON_TRUENAS_SECRET_FILE (/run/secrets/addon/truenas.key): no such file or directory",
      },
    ];
    state.health = { reachable: false, lifecycle: "active" };
    state.inventory = { target: "truenas", bound: 0, unmanaged: [], current: true };
    renderTarget();

    expect(screen.getByText("Cannot connect.")).toBeTruthy();
    // A box means something is wrong (A-04 §7.1a): the Notice, and a blocking one.
    expect(screen.getByRole("alert")).toHaveTextContent("Cannot connect.");
    expect(screen.getByText(/a problem on/)).toBeTruthy();
    // The path, because "no secret configured" and "the mount is missing" are
    // the same symptom and different fixes.
    expect(screen.getByText(/run\/secrets\/addon\/truenas\.key/)).toBeTruthy();
  });

  it("does not claim a transport fault when the secret loads", () => {
    state.roster = [{ ...summary([]), transport_status: "ok" }];
    state.health = { reachable: true, lifecycle: "active" };
    state.inventory = { target: "truenas", bound: 0, unmanaged: [], current: true };
    renderTarget();

    expect(screen.queryByText(/Connection key missing/i)).toBeNull();
  });

  it("renders the operations the manifest offers, and nothing else", () => {
    state.roster = [
      summary([
        { id: "password.set", scope: "member", confirm: false, available: true },
        { id: "account.purge", scope: "admin", confirm: true, available: true },
      ]),
    ];
    state.health = { reachable: true, lifecycle: "active" };
    state.inventory = { target: "truenas", bound: 1, unmanaged: [], current: true };
    renderTarget();

    expect(screen.getByText("password.set")).toBeInTheDocument();
    expect(screen.getByText("account.purge")).toBeInTheDocument();
  });

  // These three shipped with no plain-language description, so the row read
  // as a bare operation id with nothing beside it — every other row had one.
  it("describes activity.get, password.rotate and storage.status in plain language", () => {
    state.roster = [
      summary([
        { id: "activity.get", scope: "member", confirm: false, available: true },
        { id: "password.rotate", scope: "member", confirm: true, available: true },
        { id: "storage.status", scope: "member", confirm: false, available: true },
      ]),
    ];
    renderTarget();

    expect(screen.getByText("read what happened on the system for one person")).toBeInTheDocument();
    expect(screen.getByText("replace a person's password on the system")).toBeInTheDocument();
    expect(screen.getByText("read space used and quota")).toBeInTheDocument();
  });

  // 9.2 — an operation removed from a manifest disappears without a frontend
  // change. Asserted by rendering a manifest without it: nothing in this
  // component names an operation, so there is no list to edit.
  it("drops an operation the manifest stopped declaring", () => {
    state.roster = [summary([{ id: "password.set", scope: "member", confirm: false, available: true }])];
    renderTarget();

    expect(screen.queryByText("account.purge")).toBeNull();
  });

  it("says an operation is unavailable rather than hiding it", () => {
    state.roster = [
      summary([
        {
          id: "account.purge",
          scope: "admin",
          confirm: true,
          available: false,
          unavailable_reason: "this target does not expose user.delete",
        },
      ]),
    ];
    renderTarget();

    // Shown disabled and explained. Omitted, an operator wonders whether the
    // feature exists at all.
    expect(screen.getByText("account.purge")).toBeInTheDocument();
    expect(screen.getByText(/does not expose user.delete/)).toBeInTheDocument();
  });

  it("offers nothing while the add-on has published no manifest", () => {
    state.roster = [{ ...summary([]), callable: false }];
    renderTarget();

    expect(screen.getByText(/has not answered yet/i)).toBeInTheDocument();
  });

  it("distinguishes a maintenance window from an outage", () => {
    state.roster = [summary([])];
    state.health = {
      reachable: true,
      lifecycle: "draining",
      lifecycle_note: "rotating the API key",
    };
    renderTarget();

    // A state somebody chose reads as a decision, not a fault: the reason they
    // gave is on screen, and nothing on the page calls it an outage.
    expect(screen.getByText(/set that on purpose: rotating the API key/)).toBeInTheDocument();
    expect(screen.queryByText(/did not answer/i)).toBeNull();
    expect(screen.queryByText(/Not answering/)).toBeNull();
  });

  // A-01: Syndra's breaker is "waiting to retry", and it sends the operator to
  // the Syndra server; an outage outranks it and sends them to TrueNAS.
  it("distinguishes Syndra waiting to retry from the target being down", () => {
    state.roster = [summary([])];
    state.health = { reachable: true, lifecycle: "active", status: "waiting_to_retry", health: { status: "online", recent: ["answered"] } };
    const { unmount } = renderTarget();
    expect(screen.getByText("Syndra is waiting before it calls TrueNAS again.")).toBeInTheDocument();
    expect(screen.getByText(/look at the Syndra server/)).toBeInTheDocument();
    unmount();

    state.health = {
      reachable: false,
      status: "offline",
      health: { status: "offline", recent: ["missed", "missed", "missed"], reachable_since: "2026-10-14T08:33:00Z" },
    };
    renderTarget();
    expect(screen.getByText("TrueNAS isn't answering.")).toBeInTheDocument();
    // A Notice, amber: Syndra can't reach it, and nothing is in doubt.
    expect(screen.getByText("TrueNAS isn't answering.").closest('[role="status"]')).toHaveClass("bg-warn-soft");
    expect(screen.getByText(/Look at TrueNAS\./)).toBeInTheDocument();
    expect(screen.getByRole("img", { name: "Last 3 checks: missed, missed, missed" })).toBeInTheDocument();
  });

  // One failed read is a missed check, not an outage: still Answering, said
  // faintly with the pips, and what is on screen is the last copy.
  it("keeps a missed check calm, and says the screen is the last copy", () => {
    state.roster = [summary([])];
    state.health = {
      reachable: false,
      status: "answering",
      health: { status: "online", recent: ["answered", "answered", "missed"], missed_since: "2026-10-14T09:40:00Z" },
    };
    renderTarget();
    expect(screen.getAllByText("Answering").length).toBeGreaterThan(0);
    expect(screen.getByText(/Missed its last check, at/)).toBeInTheDocument();
    expect(screen.getByText(/no earlier copy to show/)).toBeInTheDocument();
    expect(screen.queryByText(/isn't answering/)).toBeNull();
  });

  it("labels a stale inventory with its age and refuses adoption from it", () => {
    state.roster = [summary([])];
    state.health = { reachable: true, lifecycle: "active" };
    state.inventory = {
      target: "truenas",
      bound: 2,
      unmanaged: [{ username: "root", uid: 0 }],
      current: false,
      read_at: "2026-08-01T00:00:00Z",
    };
    renderTarget();

    // The age is always given — "stale" without a number is not something an
    // operator can act on — and the affordance is GONE rather than disabled
    // with a tooltip, with its reason as text beside the row.
    expect(screen.getByText(/last state seen/i)).toBeInTheDocument();
    expect(screen.getByText(/Refresh the list above first/i)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Assign to a person" })).toBeNull();
  });

  // 1.19 — the inventory is reported, never triaged. Nothing on this page calls
  // an unmanaged account drift, and nothing offers to revoke it.
  it("never presents an unmanaged account as drift", () => {
    state.roster = [summary([])];
    state.health = { reachable: true, lifecycle: "active" };
    state.inventory = {
      target: "truenas",
      bound: 2,
      unmanaged: [{ username: "root", uid: 0 }],
      current: true,
    };
    renderTarget();

    expect(screen.getByText("root")).toBeInTheDocument();
    expect(screen.getByText(/Syndra leaves them alone/i)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /revoke/i })).toBeNull();
  });
});

/**
 * The top band, restructured to match /zitadel: one status card and a
 * 3-tile row, replacing the "Can Syndra reach it" card and its label/value
 * grid.
 */
describe("the top band: one status card and three tiles", () => {
  it("gives one calm sentence when reachable and active, with no leftover grid", () => {
    state.roster = [summary([])];
    state.health = { reachable: true, lifecycle: "active" };
    state.inventory = { target: "truenas", bound: 0, unmanaged: [], current: true };
    renderTarget();

    expect(screen.getAllByText("Answering").length).toBeGreaterThan(0);
    expect(screen.getByText(/Accepting changes\. Nothing is in flight\./)).toBeInTheDocument();
    expect(screen.queryByText("Can Syndra reach it")).toBeNull();
    expect(screen.queryByText("Last answered")).toBeNull();
  });

  it("shows the accounts Syndra manages, read from the same inventory the census uses", () => {
    state.roster = [summary([])];
    state.health = { reachable: true, lifecycle: "active" };
    state.inventory = {
      target: "truenas",
      bound: 4,
      unmanaged: [],
      current: true,
      read_at: new Date().toISOString(),
    };
    renderTarget();

    const tile = screen.getByText("Accounts Syndra manages").closest(".card");
    expect(tile).not.toBeNull();
    expect(within(tile as HTMLElement).getByText("4")).toBeInTheDocument();
  });

  it("names the maintenance state on its own tile, matching the control below", () => {
    state.roster = [summary([])];
    state.health = { reachable: true, lifecycle: "draining" };
    state.inventory = { target: "truenas", bound: 0, unmanaged: [], current: true };
    renderTarget();

    // The tile's own label, not the "Maintenance" card title further down —
    // both are legitimately on screen at once.
    expect(screen.getAllByText("Maintenance").length).toBeGreaterThanOrEqual(2);
    expect(screen.getByText("Finishing up")).toBeInTheDocument();
  });

  it("does not double the product name beside its version", () => {
    state.roster = [summary([])];
    state.health = {
      reachable: true,
      lifecycle: "active",
      product: "TrueNAS",
      product_version: "TrueNAS-25.10.5",
    };
    state.inventory = { target: "truenas", bound: 0, unmanaged: [], current: true };
    renderTarget();

    expect(screen.queryByText(/TrueNAS TrueNAS-25\.10\.5/)).toBeNull();
    expect(screen.getByText("TrueNAS 25.10.5")).toBeInTheDocument();
  });
});

/**
 * Maintenance: one compact row, one helper line — not the reason repeated
 * under every button.
 */
describe("maintenance control is a single compact row", () => {
  it("shows the 'say why first' helper once, not once per button", () => {
    state.roster = [summary([])];
    state.health = { reachable: true, lifecycle: "active" };
    state.inventory = { target: "truenas", bound: 0, unmanaged: [], current: true };
    renderTarget();

    expect(screen.getAllByText(/Say why first/)).toHaveLength(1);
  });

  it("marks the current state and disables reselecting it", () => {
    state.roster = [summary([])];
    state.health = { reachable: true, lifecycle: "active" };
    state.inventory = { target: "truenas", bound: 0, unmanaged: [], current: true };
    renderTarget();

    expect(screen.getByRole("button", { name: /Already active/ })).toBeDisabled();
  });
});

/**
 * §23 — the finding said "this stays until somebody resolves it" beside no way
 * to resolve it, so a legitimate volume replacement pinned the target as
 * compromised forever.
 *
 * The ceremony is the fix, not the button: this is the only action in the
 * product that discards evidence, and the copy has to lead with what is lost.
 */
describe("resolving a log finding", () => {
  function renderWithFinding() {
    state.resolved = [];
    state.reconcile = undefined;
    state.roster = [summary([])];
    state.health = {
      target: "truenas",
      reachable: true,
      log_anchor: {
        target: "truenas",
        head: "aaaaaaaaaaaaaaaa",
        records: 12,
        anchored_at: "2026-08-01T00:00:00Z",
        violation_reason: "records_decreased",
        violation_head: "bbbbbbbbbbbbbbbb",
        violation_records: 3,
        violation_at: "2026-08-09T00:00:00Z",
      },
    } as TargetHealth;
    return renderTarget();
  }

  it("names what is given up rather than what the button does", () => {
    renderWithFinding();
    fireEvent.click(screen.getByRole("button", { name: /resolve this finding/i }));

    expect(screen.getByText(/stops being able to tell you they did/i)).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /accept this log and start over/i }),
    ).toBeDisabled();
  });

  it("takes rung 3 and an explanation, and cites the head that was read", () => {
    renderWithFinding();
    fireEvent.click(screen.getByRole("button", { name: /resolve this finding/i }));
    const confirm = screen.getByRole("button", { name: /accept this log and start over/i });

    fireEvent.change(screen.getByLabelText(/Why the log changed/), {
      target: { value: "we replaced the volume" },
    });
    expect(confirm).toBeDisabled();

    fireEvent.change(screen.getByRole("textbox", { name: /type the system name/i }), {
      target: { value: "truenas" },
    });
    // Re-queried: the button drops its "say why"/"type this" reason the
    // instant it becomes pressable, and Button.tsx renders that as a
    // different element (bare `<button>` instead of a `<span>` wrapping one),
    // which replaces the DOM node rather than just its attributes.
    const armed = screen.getByRole("button", { name: /accept this log and start over/i });
    expect(armed).toBeEnabled();

    fireEvent.click(armed);
    // The VIOLATING head, not the anchored one. Adopting "whatever is there
    // now" would swallow a second change made while the dialog was open, which
    // is the event the anchor exists to notice.
    expect(state.resolved).toEqual([
      { head: "bbbbbbbbbbbbbbbb", note: "we replaced the volume" },
    ]);
  });
});

/**
 * §29's surface — two of Syndra's own records disagreeing about who owns an
 * account.
 *
 * It used to land among ordinary drain failures, where "the target refused this
 * call" and this read the same to anybody scanning. Those want different places
 * and different actions: one is retried after fixing the target, and this one
 * is never retried at all.
 */
describe("a disputed account", () => {
  function renderWithConflict() {
    state.ownerDecided = [];
    state.roster = [summary([])];
    state.health = {
      target: "truenas",
      reachable: true,
      binding_conflicts: [
        {
          id: "c1",
          target: "truenas",
          username: "ada",
          account_uid: 3001,
          converged_subject_id: "u-converged",
          bound_subject_id: "u-bound",
          outbox_id: "o1",
          detected_at: "2026-08-10T00:00:00Z",
        },
      ],
    } as TargetHealth;
    return renderTarget();
  }

  it("names both claimants and calls neither correct", () => {
    renderWithConflict();
    expect(screen.getByText(/Two records disagree about who owns/)).toBeInTheDocument();
    // The change LANDED. Every other terminal failure on that path means it did
    // not, and an operator reading this as "it did not go through" retries it.
    expect(screen.getByText(/The change reached TrueNAS/)).toBeInTheDocument();
  });

  it("offers the two claimants and no third, and takes rung 3", () => {
    renderWithConflict();
    fireEvent.click(screen.getByRole("button", { name: /decide who owns it/i }));

    const record = screen.getByRole("button", { name: /record the owner/i });
    expect(record).toBeDisabled();
    // Radios, not a free field: assigning it to somebody the finding does not
    // name is a different decision, and the backend refuses it.
    expect(screen.getAllByRole("radio")).toHaveLength(2);

    fireEvent.click(screen.getAllByRole("radio")[0]);
    fireEvent.change(screen.getByLabelText(/How you know/), {
      target: { value: "checked the home directory with her" },
    });
    expect(record).toBeDisabled(); // the name is still untyped

    fireEvent.change(screen.getByRole("textbox", { name: /type the account/i }), {
      target: { value: "ada" },
    });
    // Re-queried for the same reason as the log-finding dialog above: losing
    // its reason swaps the button's DOM node, not just its `disabled` attribute.
    const armed = screen.getByRole("button", { name: /record the owner/i });
    expect(armed).toBeEnabled();

    fireEvent.click(armed);
    expect(state.ownerDecided).toEqual([
      { id: "c1", owner: "u-bound", note: "checked the home directory with her" },
    ]);
  });

  it("says the other person loses it without being told", () => {
    renderWithConflict();
    fireEvent.click(screen.getByRole("button", { name: /decide who owns it/i }));
    expect(screen.getByText(/without\s+being told/)).toBeInTheDocument();
    // And that the account is not right yet. The change that caused the
    // conflict overwrote one person's entitlements with the other's, so a
    // convergence is queued for both — and until it drains the account still
    // holds what that change wrote.
    expect(screen.getByText(/Fixes for both people wait in Pending changes/)).toBeInTheDocument();
  });
});

/**
 * Board §21, "Capabilities · rendered from the manifest".
 *
 * The manifest says which operations stop and ask before they run, and this
 * list is the only place an operator can learn that before pressing one. It was
 * being dropped — `confirm` arrived on every operation and reached no pixel.
 *
 * The section's own argument for showing an unavailable operation rather than
 * omitting it applies here unchanged: what is absent from the list reads as not
 * existing, so an operator who has not seen "confirmation required" concludes
 * that nothing here will ask.
 */
describe("what it can do · the confirmation the manifest declares", () => {
  it("says which operations stop and ask, and does not say it of the rest", () => {
    state.roster = [
      summary([
        { id: "account.provision", scope: "member", confirm: false, available: true },
        { id: "account.purge", scope: "member", confirm: true, available: true },
      ]),
    ];
    state.health = { reachable: true, lifecycle: "active" };
    state.inventory = { target: "truenas", bound: 0, unmanaged: [], current: true };
    renderTarget();

    const purge = screen.getByText("account.purge").parentElement;
    expect(purge?.textContent).toContain("confirmation required");

    const provision = screen.getByText("account.provision").parentElement;
    expect(provision?.textContent).not.toContain("confirmation required");
  });
});

/**
 * The census line, and what left with it (design M6).
 *
 * Two panels — the mapping rows and their published versions — moved to their
 * own screen, because editing one mapping moves access for everybody holding
 * that role and a fact with that reach has no room to breathe in a table row.
 *
 * What has to stay true afterwards: the target page still answers "why is she
 * bound here", and there is a way through to the screen that can change it.
 * A page that dropped the panels and offered no route would have lost a
 * capability rather than moved one.
 */
describe("what roles reach here · the census and its handoff", () => {
  it("no longer holds the mappings table or the version history", () => {
    state.roster = [summary([])];
    state.health = { reachable: true, lifecycle: "active" };
    state.inventory = { target: "truenas", bound: 0, unmanaged: [], current: true };
    renderTarget();

    expect(screen.queryByText("What roles reach here")).toBeNull();
    expect(screen.queryByText("Published versions")).toBeNull();
  });

  it("offers the route that received them", () => {
    state.roster = [summary([])];
    state.health = { reachable: true, lifecycle: "active" };
    state.inventory = { target: "truenas", bound: 0, unmanaged: [], current: true };
    renderTarget();

    const through = screen.getByRole("link", { name: /mapping/i });
    expect(through.getAttribute("href")).toBe("/system/targets/truenas/mappings");
  });

  // The sentence is the reason somebody clicks, and it is repeated verbatim at
  // the top of the screen it leads to.
  it("keeps the sentence that explains why the mappings left", () => {
    state.roster = [summary([])];
    state.health = { reachable: true, lifecycle: "active" };
    state.inventory = { target: "truenas", bound: 0, unmanaged: [], current: true };
    renderTarget();

    expect(
      screen.getByText(/Editing one changes access for everybody holding that role/),
    ).toBeTruthy();
  });
});

/**
 * Adopting one account, and the four things that were wrong about asking.
 *
 * Adoption is the one irreversible act on this page, so the form that performs
 * it has to be readable under pressure: attached to the account it names, in
 * the product's own motion, in the product's own colour for a grant, and in
 * sentences that do not appear to contradict each other.
 */
describe("adopting an account", () => {
  const twoAccounts = {
    target: "truenas",
    bound: 1,
    accounts: [
      { target: "truenas", subject_id: "s-12", username: "kabir.r", bound_by: "op", bound_at: "", last_seen_at: "" },
    ],
    unmanaged: [
      { username: "sai", uid: 3000 },
      { username: "kabir", uid: 3002 },
    ],
    current: true,
    // Adoption is blocked off anything but a live or ageing read (§31 A), so
    // the fixture has to carry one for the form to be reachable at all.
    read_at: new Date().toISOString(),
  };

  function openAdoptForSai() {
    state.adopted = [];
    state.roster = [summary([])];
    state.health = { reachable: true, lifecycle: "active" };
    state.inventory = twoAccounts;
    renderTarget();
    fireEvent.click(screen.getAllByRole("button", { name: "Assign to a person" })[0]);
  }

  function chooseSai() {
    fireEvent.change(screen.getByLabelText("Search people by name or email"), { target: { value: "sa" } });
    fireEvent.click(screen.getByRole("radio", { name: /Sai Khurana/ }));
  }

  // #57: a dialog about the account it was opened from.
  it("opens a dialog about the account it was opened from", () => {
    openAdoptForSai();
    const dialog = screen.getByRole("dialog");
    expect(within(dialog).getByText("Assign sai to a person")).toBeInTheDocument();
    expect(dialog.textContent).not.toContain("kabir ");
  });

  // Red is this product's word for a click that takes access away. This one
  // gives somebody an account.
  it("does not dress a grant as a destruction", () => {
    openAdoptForSai();
    const submit = screen.getByRole("button", { name: /^Assign sai/ });
    expect(submit.className, "dangerConfirm is reserved for removing access").not.toContain("bg-danger");
    expect(screen.getByText(/There is no undo/)).toBeInTheDocument();
    expect(screen.getByText(/Type the account name/)).toBeInTheDocument();
  });

  // Found by name or email, never by a pasted ID; namesakes told apart by
  // title, team and email; someone with an account here can't be chosen.
  it("finds the person by name, and won't give a second account", () => {
    openAdoptForSai();
    fireEvent.change(screen.getByLabelText("Search people by name or email"), { target: { value: "a" } });

    const sai = screen.getByRole("radio", { name: /Sai Khurana/ });
    expect(sai.textContent).toContain("Lab technician · Design");
    expect(sai.textContent).toContain("sai@example.edu");
    const kabir = screen.getByRole("radio", { name: /Kabir Rao/ });
    expect(kabir.hasAttribute("disabled")).toBe(true);
    expect(kabir.textContent).toContain("Already has kabir.r on TrueNAS");
    // No team: the line is the title alone, never a dangling "·" or a "—".
    expect(kabir.textContent).toContain("Studentkabir@example.edu");
    expect(kabir.textContent).not.toMatch(/Student ·|—/);
  });

  it("says what changes and what stays, once someone is chosen", () => {
    openAdoptForSai();
    expect(screen.queryByText("Changes")).toBeNull();
    chooseSai();
    expect(screen.getByRole("radio", { name: /Sai Khurana/ }).getAttribute("aria-checked")).toBe("true");
    expect(screen.getByText("Changes")).toBeInTheDocument();
    expect(screen.getByText(/belongs to/).textContent).toContain("Sai Khurana");
    expect(screen.getByText(/Its home folder, its shares and its group memberships/)).toBeInTheDocument();
  });

  it("assigns only once the name is typed, and reports it under the account", () => {
    openAdoptForSai();
    chooseSai();
    const submit = screen.getByRole("button", { name: "Assign sai to Sai Khurana" });
    expect(submit.hasAttribute("disabled")).toBe(true);

    fireEvent.change(screen.getByLabelText(/Type the account name/), { target: { value: "sai" } });
    fireEvent.click(screen.getByRole("button", { name: "Assign sai to Sai Khurana" }));

    expect(state.adopted).toEqual([{ username: "sai", subjectId: "s-99" }]);
    expect(screen.queryByRole("dialog")).toBeNull();
    const outcome = screen.getByText(/bound to that person/).closest("[role=status]")!;
    expect(outcome.parentElement!.parentElement!.textContent).toContain("sai");
    expect(screen.getByText(/Assigned to/)).toBeInTheDocument();
  });
});


// 3a: the project comes before anything that gives access. Health and the
// pause stay live; what waits is drawn whole and inert under one line.
describe("the system's Zitadel project", () => {
  it("asks for it first, and holds back what waits on it", () => {
    state.roster = [
      { ...summary([]), project: null },
      { ...summary([]), target: "door", project: { id: "p3", name: "Laser Lab" } },
    ];
    state.health = { reachable: true, lifecycle: "active", status: "answering", health: { status: "online", recent: ["answered"] } };
    state.inventory = { target: "truenas", bound: 0, unmanaged: [], current: true };
    state.projectSet = [];
    renderTarget();

    expect(screen.getByText("Choose TrueNAS's Zitadel project")).toBeInTheDocument();
    expect(screen.getByText("Step 1 of 2")).toBeInTheDocument();
    const select = screen.getByLabelText("Zitadel project") as HTMLSelectElement;
    const options = Array.from(select.options).map((o) => [o.textContent, o.disabled]);
    expect(options).toContainEqual(["Network storage · 2 roles", false]);
    expect(options).toContainEqual(["Wood shop · no roles yet", false]);
    expect(options).toContainEqual(["Laser Lab · used by Door", true]);

    expect(screen.getByText("Waits for TrueNAS's Zitadel project")).toBeInTheDocument();
    const fence = () => document.getElementById("waiting")?.closest("[aria-disabled]");
    expect(fence()?.hasAttribute("inert")).toBe(true);
    // What reports on the system stays live.
    expect(screen.getByText("Is it answering").closest("[aria-disabled]")).toBeNull();

    fireEvent.change(select, { target: { value: "p1" } });
    fireEvent.click(screen.getByRole("button", { name: "Use this project" }));
    expect(state.projectSet).toEqual(["p1"]);
    expect(screen.getByText("TrueNAS uses Network storage.")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Map its roles to TrueNAS groups" })).toBeInTheDocument();
    expect(fence()).toBeNull();
  });

  it("is a thin strip once set, and the only link to the mappings", () => {
    state.roster = [summary([])];
    state.health = { reachable: true, lifecycle: "active" };
    state.inventory = { target: "truenas", bound: 0, unmanaged: [], current: true };
    renderTarget();

    expect(screen.getByRole("link", { name: "Network storage" }).getAttribute("href")).toBe("/projects/p1");
    expect(screen.getAllByRole("link", { name: /mapping/i })).toHaveLength(1);
    expect(screen.queryByText("Choose TrueNAS's Zitadel project")).toBeNull();
  });
});

// A-03 §1: where home folders go is chosen once, on the system page, in the
// settings card; changing it moves nothing, and says how many stay.
describe("where home folders go", () => {
  const setting = (value: string, values = ["/mnt/p/main/home_dir", "/mnt/p/main/homes"]): TargetSetting => ({
    name: "home_root",
    label: "Where home folders go",
    value,
    values,
    labels: { "/mnt/p/main/home_dir": "main › home_dir", "/mnt/p/main/homes": "main › homes" },
    values_read: true,
  });
  function open(value: string, values?: string[]) {
    state.roster = [summary([])];
    state.health = { reachable: true, lifecycle: "active" };
    state.inventory = { target: "truenas", bound: 0, unmanaged: [], current: true };
    state.settings = { read: true, settings: [setting(value, values)] };
    state.settingPut = [];
    state.homes = {
      read: true,
      usage_read: true,
      accounts: [
        { subject_id: "a", username: "ada", gives_home: true, home: "/mnt/p/main/home_dir/ada" },
        { subject_id: "b", username: "bo", gives_home: true, home: "/mnt/p/main/home_dir/bo" },
        { subject_id: "c", username: "cy", gives_home: false },
      ],
    };
    renderTarget();
  }

  it("is chosen once, and says mappings work without it", () => {
    open("");
    expect(screen.getByText("Not chosen yet")).toBeInTheDocument();
    expect(screen.getByText(/none can give a home folder until it is chosen/)).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Choose folder" }));
    const dialog = screen.getByRole("dialog");
    expect(within(dialog).getByText("Choose where home folders go")).toBeInTheDocument();
    expect(within(dialog).getByText(/Nothing is made until a mapping gives home folders/)).toBeInTheDocument();
    fireEvent.click(within(dialog).getByRole("button", { name: "Use main › home_dir" }));
    expect(state.settingPut).toEqual([{ name: "home_root", value: "/mnt/p/main/home_dir" }]);
  });

  it("changes without moving anything, and says how many homes stay", () => {
    open("/mnt/p/main/home_dir");
    expect(screen.getByText("main › home_dir")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Change folder" }));
    const dialog = screen.getByRole("dialog");
    expect(within(dialog).queryByText("Changes")).toBeNull();
    fireEvent.change(within(dialog).getByLabelText("Folder in main"), { target: { value: "/mnt/p/main/homes" } });
    expect(within(dialog).getByText(/The 2 home folders already in/)).toBeInTheDocument();
    expect(within(dialog).getByText(/Nothing is moved, and nobody's access changes/)).toBeInTheDocument();
    fireEvent.click(within(dialog).getByRole("button", { name: "Make new homes in main › homes" }));
    expect(state.settingPut).toEqual([{ name: "home_root", value: "/mnt/p/main/homes" }]);
  });

  it("says when main has no folder to make homes in", () => {
    open("", []);
    fireEvent.click(screen.getByRole("button", { name: "Choose folder" }));
    const dialog = screen.getByRole("dialog");
    expect(within(dialog).getByText("No folder to offer")).toBeInTheDocument();
    expect(within(dialog).getByRole("button", { name: "Read again" })).toBeInTheDocument();
    expect(within(dialog).getByText(/Nothing to choose until/)).toBeInTheDocument();
  });
});
