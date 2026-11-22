// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { UiViewProvider } from "@/lib/ui-view";
import { fireEvent, render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { TargetOverview } from "@/components/targets/TargetOverview";
import { expectFullStrength } from "@/test-utils/fullStrength";
import { makeProxyFetch } from "@/test-utils/proxyFetch";
import { OVERVIEW_NEXT, RETENTION_TITLE } from "@/components/usage/copy";
import { targetStats } from "@/components/usage/__tests__/fixtures";
import type { AccountHome, TargetHealthWire, TargetInventory, TargetSetting, TargetSummary } from "@/lib/queries/useTargets";
import type { WireRead } from "@/lib/queries/read";
import { asRead, NONE, readNow } from "@/test-utils/wire";
import { asHealth } from "@/test-utils/wireHealth";

const nameResolverMiss = { value: undefined, resolved: true } as const;

// Adoption asks an operator to type a raw Syndra ID with no picker — the one
// check available before an irreversible hand-off is showing whose name that
// ID actually resolves to, both while typing and in the result afterwards.
vi.mock("@/lib/queries/useNameResolver", () => ({
  useNameResolver: () => ({
    resolveUser: (id: string) =>
      id === "s-99" ? { value: { display_name: "Iyo Sky" }, resolved: true } : nameResolverMiss,
    resolveProject: () => nameResolverMiss,
    resolveRole: () => nameResolverMiss,
    resolveBundle: () => nameResolverMiss,
  }),
}));

// 9.2/9.20/9.21 — the operation set comes from the manifest, and the health
// states render as distinct things rather than as one "status".

const state = {
  roster: [] as TargetSummary[],
  health: {} as TargetHealthWire,
  healthLoading: false,
  healthError: null as unknown,
  findingsError: null as unknown,
  inventory: {} as Omit<TargetInventory, "read"> & { read?: WireRead },
  resolved: [] as Array<{ head: string; note: string }>,
  reconcile: undefined as undefined | Record<string, unknown>,
  released: [] as { subjectId: string; version?: string }[],
  releaseResult: { status: "released" } as Record<string, unknown>,
  adopted: [] as Array<{ username: string; subjectId: string }>,
  adoptResult: { status: "adopted", detail: "The account is now bound to that person." } as unknown,
  projectSet: [] as string[],
  settings: { settings: [] as TargetSetting[] } as { read?: WireRead; settings?: TargetSetting[] },
  settingsLoading: false,
  settingsError: null as unknown,
  settingsRefetches: 0,
  settingPut: [] as Array<{ name: string; value: string }>,
  homes: { accounts: [] as AccountHome[] } as { read?: WireRead; accounts: AccountHome[] },
  auto: true as boolean | null,
};

vi.mock("@/lib/queries/useIndicators", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/queries/useIndicators")>()),
  useAutoRetry: () => state.auto,
}));

vi.mock("@/lib/queries/useUsers", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/queries/useUsers")>()),
  useUsers: (q: string) => ({
    data: q
      ? [
          { user: { id: "s-99", name: "Iyo Sky", email: "iyo@example.edu", title: "Lab technician", team: "Design", status: "active", avatar: "" } },
          { user: { id: "s-12", name: "Kairi Sane", email: "kairi@example.edu", title: "Student", team: "", status: "active", avatar: "" } },
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
  const { withRead } = await import("@/lib/queries/read");
  const actual = await vi.importActual<typeof import("@/lib/queries/useTargets")>(
    "@/lib/queries/useTargets",
  );
  return {
    ...actual,
    useTargets: () => ({ data: state.roster, isLoading: false, error: null }),
    useTargetHealth: () => ({
      data: state.healthLoading ? undefined : asHealth(state.health),
      isLoading: state.healthLoading,
      error: state.healthError,
    }),
    useMergeFindings: () => ({ data: undefined, isLoading: false, error: state.findingsError }),
    useUnconfirmedOperations: () => ({ data: [], isLoading: false, error: null }),
    useTargetInventory: () => ({
      data: asRead(state.inventory),
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
      reset: vi.fn(),
      isPending: false,
      error: null,
    }),
    useReleaseBinding: () => ({
      mutate: (input: { subjectId: string; version?: string }, opts?: { onSuccess?: (r: unknown) => void }) => {
        state.released.push(input);
        opts?.onSuccess?.(state.releaseResult);
      },
      isPending: false,
      error: null,
    }),
    useSetLifecycle: () => ({ mutate: vi.fn(), reset: vi.fn(), isPending: false }),
    useTargetSettings: () => ({
      data: state.settingsLoading || !state.settings ? undefined : asRead(state.settings),
      isLoading: state.settingsLoading,
      error: state.settingsError,
      refetch: () => {
        state.settingsRefetches++;
      },
    }),
    useTargetHomes: () => ({ data: asRead(state.homes), isLoading: false, error: null }),
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
      data: state.reconcile && withRead(state.reconcile as never),
      error: null,
    }),
    useResolveLogFinding: () => ({
      mutate: (input: { head: string; note: string }, opts?: { onSuccess?: () => void }) => {
        state.resolved.push(input);
        opts?.onSuccess?.();
      },
      reset: vi.fn(),
      isPending: false,
      error: null,
    }),
  };
});

function summary(operations: TargetSummary["operations"]): TargetSummary {
  return {
    target: "truenas",
    name: "TrueNAS",
    name_from: "product",
    registered: true,
    auth_mode: "derived",
    callable: true,
    operations,
    circuit_open: false,
    project: { id: "p1", name: "Network storage" },
  };
}

function renderTarget(view: "basic" | "advanced" = "advanced") {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <UiViewProvider isOperator initialView={view}>
        <TargetOverview target="truenas" />
      </UiViewProvider>
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  state.auto = true;
  state.healthLoading = false;
  state.healthError = null;
  state.findingsError = null;
  state.settingsLoading = false;
  state.settingsError = null;
  state.settingsRefetches = 0;
  state.settings = { settings: [] };
});

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
      // A fresh read, because adoption is gated on one — without it the whole
      // list renders unadoptable and this test would pass for the wrong reason.
      read: readNow(new Date().toISOString()),
      unmanaged: [
        { username: "iyo", account_id: "3000" },
        { username: "syndra", account_id: "3001", self: true },
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
    state.inventory = { target: "truenas", bound: 0, unmanaged: [] };

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
    state.inventory = { target: "truenas", bound: 0, unmanaged: [] };
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
    state.inventory = { target: "truenas", bound: 0, unmanaged: [] };
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
    state.inventory = { target: "truenas", bound: 0, unmanaged: [] };
    state.health = { reachable: true, lifecycle: "active" };
    state.reconcile = {
      target: "truenas", bound: 4, queued: 1, read: readNow(),
      stale: [{ subject_id: "s1", username: "becky", account_id: "3999" }],
    };
    renderTarget();

    expect(screen.getByText(/recorded as owning an account that is no longer/i)).toBeTruthy();
    expect(screen.getByText("becky")).toBeTruthy();
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
    state.inventory = { target: "truenas", bound: 0, unmanaged: [] };
    state.health = { reachable: true, lifecycle: "active" };
    state.reconcile = {
      target: "truenas", bound: 4, queued: 1, read: readNow(),
      stale: [{ subject_id: "s1", username: "becky", account_id: "3999", version: "v-read" }],
    };
    renderTarget();

    fireEvent.click(screen.getByText("Stop tracking this account"));
    expect(state.released).toEqual([]);
    expect(screen.getByText(/Nothing is deleted/i)).toBeTruthy();

    fireEvent.click(screen.getByText("Stop tracking becky"));
    // The version the page read goes with the press: a stale page is refused by the backend.
    expect(state.released).toEqual([{ subjectId: "s1", version: "v-read" }]);
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
      operation: "op-1",
      status: "unconfirmed",
      detail: "This may have been applied. Syndra is checking; don't send it again.",
    };
    state.roster = [summary([])];
    state.inventory = { target: "truenas", bound: 0, unmanaged: [] };
    state.health = { reachable: true, lifecycle: "active" };
    state.reconcile = {
      target: "truenas", bound: 4, queued: 1, read: readNow(),
      stale: [{ subject_id: "s1", username: "becky", account_id: "3999" }],
    };
    renderTarget();

    fireEvent.click(screen.getByText("Stop tracking this account"));
    fireEvent.click(screen.getByText("Stop tracking becky"));

    expect(screen.queryByText(/Stopped tracking it\. Nothing on TrueNAS was changed/i)).toBeNull();
    // The one unconfirmed outcome every write says (design §13.4).
    expect(screen.getByText(/This may have been applied\. Syndra is checking; don't send it again\./)).toBeTruthy();
    expect(screen.getByText("Stop tracking becky")).toBeTruthy();
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
    state.inventory = { target: "truenas", bound: 0, unmanaged: [] };
    state.health = { reachable: true, lifecycle: "active" };
    state.reconcile = {
      target: "truenas", bound: 4, queued: 1, read: readNow(),
      stale: [{ subject_id: "s1", username: "becky", account_id: "3999" }],
    };
    renderTarget();

    fireEvent.click(screen.getByText("Stop tracking this account"));
    fireEvent.click(screen.getByText("Stop tracking becky"));

    expect(screen.queryByText(/Stopped tracking it\. Nothing on TrueNAS was changed/i)).toBeNull();
    expect(screen.getByText(/Press release again to repair it/i)).toBeTruthy();
    expect(screen.getByText("Stop tracking becky")).toBeTruthy();
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
    state.inventory = { target: "truenas", bound: 0, unmanaged: [] };
    renderTarget();

    expect(screen.getByText("Cannot connect.")).toBeTruthy();
    // A box means something is wrong (A-04 §7.1a): the Notice, and a blocking one.
    expect(screen.getByRole("alert")).toHaveTextContent("Cannot connect.");
    expect(screen.getByText(/a problem on/)).toBeTruthy();
    // The path, because "no secret configured" and "the mount is missing" are
    // the same symptom and different fixes.
    expect(screen.getByText(/run\/secrets\/addon\/truenas\.key/)).toBeTruthy();
  });

  // 8g fix round 3: busy past half an hour stops the send, and the page says
  // since when, even while the system reads as not reachable.
  it("says a system busy past the limit, and since when", () => {
    state.roster = [{ ...summary([]), busy_since: "2026-11-22T10:00:00Z" }];
    state.health = { reachable: false, lifecycle: "active" };
    state.inventory = { target: "truenas", bound: 0, unmanaged: [] };
    renderTarget();

    expect(screen.getByText(/TrueNAS has been busy since /)).toBeTruthy();
    expect(
      screen.getByText("No change was sent. Once TrueNAS answers, revocations go on their own; send the rest again."),
    ).toBeTruthy();
  });

  // PR 261 review: with the background drain off, nothing goes on its own,
  // so the page never promises it does.
  it("says busy changes wait for Send when nothing sends them on its own", () => {
    state.auto = false;
    state.roster = [{ ...summary([]), busy_since: "2026-11-22T10:00:00Z" }];
    state.health = { reachable: true, lifecycle: "active" };
    state.inventory = { target: "truenas", bound: 0, unmanaged: [] };
    renderTarget();

    expect(screen.getByText("No change was sent. Send it again from Pending changes once TrueNAS answers.")).toBeTruthy();
    expect(screen.queryByText(/revocations go on their own|keeps trying/)).toBeNull();
  });

  it("does not claim a transport fault when the secret loads", () => {
    state.roster = [{ ...summary([]), transport_status: "ok" }];
    state.health = { reachable: true, lifecycle: "active" };
    state.inventory = { target: "truenas", bound: 0, unmanaged: [] };
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
    state.inventory = { target: "truenas", bound: 1, unmanaged: [] };
    renderTarget();

    // Words outside; the ids wait under Technical detail until it is opened (A-20 P11).
    expect(screen.getByText("Deletes an account")).toBeInTheDocument();
    expect(screen.queryByText("account.purge")).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: /Technical detail/ }));
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

    expect(screen.getByText("Reads a share's activity")).toBeInTheDocument();
    expect(screen.getByText("Replaces a person's password, never logged")).toBeInTheDocument();
    expect(screen.getByText("Reads space used and quota")).toBeInTheDocument();
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
    expect(screen.getByText("Deletes an account")).toBeInTheDocument();
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
    // Said in the reach card and again in Maintenance, from the one sentence (lifecycleWhy).
    expect(screen.getAllByText(/set that on purpose: “rotating the API key”/)).toHaveLength(2);
    expect(screen.queryByText(/did not answer/i)).toBeNull();
    expect(screen.queryByText(/Not answering/)).toBeNull();
  });

  // The add-on's own reason codes are never shown as codes, in the reach card or in Maintenance.
  it("says an add-on's own lifecycle reason as a sentence, never the code", () => {
    state.roster = [summary([])];
    state.health = { reachable: true, lifecycle: "read_only", lifecycle_note: "startup" };
    renderTarget();
    expect(screen.getAllByText(/It was started this way by its configuration\./)).toHaveLength(2);
    expect(screen.queryByText(/startup/)).toBeNull();
    expect(screen.queryByText(/Somebody set that on purpose: startup/)).toBeNull();
  });

  // A paused add-on refuses every change, and Maintenance is the way back. Basic does not show it
  // (structure never moves with data), so the reach card is the next step: it switches to Advanced.
  it("gives a paused add-on a next step in both views", () => {
    state.roster = [summary([])];
    state.health = { reachable: true, lifecycle: "read_only", lifecycle_note: "startup" };
    const basic = renderTarget("basic");
    expect(screen.queryByRole("heading", { name: "Maintenance" })).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "Open Maintenance in Advanced" }));
    expect(screen.getByText("Maintenance")).toBeInTheDocument();
    expect(document.getElementById("maintenance")).not.toBeNull();
    expect(screen.getByRole("button", { name: "Open Maintenance" })).toBeInTheDocument();
    basic.unmount();

    renderTarget("advanced");
    expect(document.getElementById("maintenance")).not.toBeNull();
    expect(screen.getByRole("button", { name: "Open Maintenance" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /in Advanced/ })).toBeNull();
  });

  // R4: Basic hides the machine's words; the sections around them keep their order and their anchors.
  it("hides the technical parts in Basic without moving or removing any section", () => {
    state.roster = [{ ...summary([{ id: "account.purge", scope: "admin", confirm: true, available: true }]), also_says: ["x-vendor: 1"] }];
    state.health = { reachable: true, lifecycle: "active" };
    const sections = () => Array.from(document.querySelectorAll("section[id]")).map((x) => x.id);
    const advanced = renderTarget("advanced");
    expect(screen.getByText("Maintenance")).toBeInTheDocument();
    expect(screen.getByText("What the add-on also says")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Technical detail/ })).toBeInTheDocument();
    const inAdvanced = sections();
    advanced.unmount();

    renderTarget("basic");
    expect(screen.queryByText("Maintenance")).toBeNull();
    expect(screen.queryByText("What the add-on also says")).toBeNull();
    expect(screen.queryByRole("button", { name: /Technical detail/ })).toBeNull();
    // What stays: the operations in words, and every region, in the same order, jumpable by id.
    expect(screen.getByText("Deletes an account")).toBeInTheDocument();
    expect(sections()).toEqual(inAdvanced);
    expect(screen.getAllByRole("link", { name: /What runs here/ }).length).toBeGreaterThan(0);
  });

  it("lays the accounts it did not create out in columns, whatever the username's length", () => {
    state.roster = [summary([])];
    state.health = { reachable: true, lifecycle: "active" };
    state.inventory = {
      target: "truenas",
      bound: 0,
      read: readNow(new Date().toISOString()),
      unmanaged: [
        { username: "a", account_id: "1", standing: { usable: false, waiting_on: "none", note: "Switched off on TrueNAS." } },
        { username: "a-much-longer-service-account", account_id: "2" },
      ],
    };
    renderTarget();
    // As on "Accounts Syndra manages": the account flexes and the status has a fixed seat before the
    // action seat, so it starts at one x whatever the username's length.
    const names = ["a", "a-much-longer-service-account"].map((n) => screen.getByText(n));
    for (const n of names) expect(n).toHaveClass("flex-1");
    const seats = document.querySelectorAll("[data-standing-seat]");
    expect(seats).toHaveLength(2);
    for (const s of Array.from(seats)) expect(s).toHaveClass("w-(--seat-status)", "flex-none");
    // The status column holds the state and its note; the next step is the row's detail line (D-1 §5 #39).
    const row = names[0].closest("[data-card-row]") as HTMLElement;
    const step = row.querySelector("[data-row-detail] [data-standing-step]") as HTMLElement;
    expect(step.textContent).toMatch(/\S/);
    expect(row.querySelector("[data-standing-seat]")).not.toHaveTextContent(step.textContent!);
  });

  // D-1 Q5: a healthy row is silent here too; a usable account still waiting on someone is not.
  it("leaves a healthy account it did not create without a step, and writes one where there is one", () => {
    state.roster = [summary([])];
    state.health = { reachable: true, lifecycle: "active" };
    state.inventory = {
      target: "truenas",
      bound: 0,
      read: readNow(new Date().toISOString()),
      unmanaged: [
        { username: "healthy", account_id: "1", standing: { usable: true, waiting_on: "none" } },
        { username: "waits", account_id: "2", standing: { usable: true, waiting_on: "member" } },
      ],
    };
    renderTarget();
    const row = (name: string) => screen.getByText(name).closest("[data-card-row]") as HTMLElement;
    expect(row("healthy")).toHaveTextContent("Ready to use");
    expect(row("healthy").querySelector("[data-row-detail]")).toBeNull();
    expect(row("healthy")).not.toHaveTextContent("Nothing for you to do.");
    expect(row("waits").querySelector("[data-row-detail] [data-standing-step]")).toHaveTextContent("Nothing for you to do.");
  });

  // Design §13.1: two outages, never one word. The add-on down sends the operator to its container
  // (and an open breaker says when Syndra tries again); the app down sends them to the system.
  it("tells the add-on being down from the system behind it being down", () => {
    state.roster = [summary([])];
    state.health = {
      read: NONE,
      status: "addon_unreachable",
      health: { state: "addon_unreachable", cause: "no_answer", retry_after: 30, recent: ["missed", "missed", "missed"], since: "2026-10-14T08:33:00Z" },
    };
    const { unmount } = renderTarget();
    expect(screen.getByText("TrueNAS's add-on isn't answering.")).toBeInTheDocument();
    expect(screen.getByText("TrueNAS's add-on isn't answering.").closest('[role="status"]')).toHaveClass("bg-warn-soft");
    expect(screen.getAllByText(/Check that its container is running\. Syndra keeps trying\. Syndra tries again in 30 s\./).length).toBeGreaterThan(0);
    expect(screen.getByRole("img", { name: "Last 3 checks: missed, missed, missed" })).toBeInTheDocument();
    unmount();

    state.health = {
      read: { state: "none", reason: "app_unreachable" },
      status: "app_unreachable",
      health: { state: "app_unreachable", recent: ["answered", "answered", "answered"] },
    };
    renderTarget();
    expect(screen.getByText("TrueNAS isn't answering.")).toBeInTheDocument();
    expect(screen.getAllByText(/TrueNAS's add-on is running but can't reach TrueNAS\. Check that TrueNAS is on\./).length).toBeGreaterThan(0);
    expect(screen.queryByText(/container/)).toBeNull();
  });

  // One failed live read is a missed check, not an outage: still Answering, said faintly with the
  // pips, and the add-on's part that was not read says why, never "accepting changes".
  it("keeps a missed check calm, and says why the add-on's part was not read", () => {
    state.roster = [summary([])];
    state.health = {
      read: NONE,
      status: "answering",
      health: { state: "answering", recent: ["answered", "answered", "missed"], missed_since: "2026-10-14T09:40:00Z" },
    };
    renderTarget();
    expect(screen.getAllByText("Answering").length).toBeGreaterThan(0);
    expect(screen.getByText(/Missed its last check, at/)).toBeInTheDocument();
    expect(screen.getByText(/Syndra can't reach TrueNAS's add-on/)).toBeInTheDocument();
    expect(screen.queryByText(/Accepting changes/)).toBeNull();
    expect(screen.queryByText(/isn't answering/)).toBeNull();
  });

  it("labels a stale inventory with its age and refuses adoption from it", () => {
    state.roster = [summary([])];
    state.health = { reachable: true, lifecycle: "active" };
    state.inventory = {
      target: "truenas",
      bound: 2,
      unmanaged: [{ username: "root", account_id: "0" }],
      read: { state: "kept", taken_at: "2026-08-01T00:00:00Z", reason: "cached" },
    };
    renderTarget();

    // The age is always given — "stale" without a number is not something an
    // operator can act on — and the control is off with its reason said once
    // for the card, as text, never a tooltip and never under every row (A-13).
    expect(screen.getByText(/The account list was read/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Assign to a person" })).toBeDisabled();
    expect(screen.getAllByText(/Assigning waits for a current read of TrueNAS/)).toHaveLength(1);
    // A mirror read: the card recedes, its words at full strength.
    const card = screen.getByText("Not created by Syndra").closest(".card") as HTMLElement;
    expect(card).toHaveAttribute("data-stale");
  });

  // At phone width the status wraps under the name rather than being squeezed, as in the managed rows.
  it("gives an unmanaged row's status a width of its own", () => {
    state.roster = [summary([])];
    state.health = { reachable: true, lifecycle: "active" };
    state.inventory = { target: "truenas", bound: 0, unmanaged: [{ username: "root", account_id: "0" }] };
    renderTarget();
    expect(document.querySelector("[data-standing-seat]")).toHaveClass("w-(--seat-status)");
  });

  // 1.19 — the inventory is reported, never triaged. Nothing on this page calls
  // an unmanaged account drift, and nothing offers to revoke it.
  it("never presents an unmanaged account as drift", () => {
    state.roster = [summary([])];
    state.health = { reachable: true, lifecycle: "active" };
    state.inventory = {
      target: "truenas",
      bound: 2,
      unmanaged: [{ username: "root", account_id: "0" }],
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
describe("the top band: Syndra's reach, what the system reports, and Maintenance", () => {
  it("gives one calm sentence when reachable and active, with no leftover grid", () => {
    state.roster = [summary([])];
    state.health = { reachable: true, lifecycle: "active" };
    state.inventory = { target: "truenas", bound: 0, unmanaged: [] };
    renderTarget();

    expect(screen.getAllByText("Answering").length).toBeGreaterThan(0);
    expect(screen.getByText(/Accepting changes\. Nothing is in flight\./)).toBeInTheDocument();
    expect(screen.queryByText("Can Syndra reach it")).toBeNull();
    expect(screen.queryByText("Last answered")).toBeNull();
  });

  it("does not say it is accepting changes when the live read failed", () => {
    state.roster = [summary([])];
    state.health = { read: NONE, status: "answering", health: { state: "answering", recent: ["answered"] } };
    state.inventory = { target: "truenas", bound: 0, unmanaged: [] };
    renderTarget();

    expect(screen.queryByText(/Accepting changes/)).toBeNull();
    expect(screen.getByText(/Syndra can't reach TrueNAS's add-on/)).toBeInTheDocument();
  });

  it("shows the accounts Syndra manages, read from the same inventory the census uses", () => {
    state.roster = [summary([])];
    state.health = { reachable: true, lifecycle: "active" };
    state.inventory = {
      target: "truenas",
      bound: 4,
      unmanaged: [],
      read: readNow(new Date().toISOString()),
    };
    renderTarget();

    const card = screen.getByText("Accounts Syndra manages").closest(".card");
    expect(card).not.toBeNull();
    expect(within(card as HTMLElement).getByText("4")).toBeInTheDocument();
  });

  it("does not call a target Active, or offer to drain it, before health has answered", () => {
    state.roster = [summary([])];
    state.health = {};
    state.healthLoading = true;
    state.inventory = { target: "truenas", bound: 0, unmanaged: [] };
    renderTarget();

    expect(screen.getByText("Maintenance")).toBeInTheDocument();
    expect(screen.queryByText("Active")).toBeNull();
    expect(screen.queryByRole("button", { name: "Start draining" })).toBeNull();
    expect(screen.queryByRole("button", { name: "Set read-only" })).toBeNull();
  });

  it("says the maintenance state was not read when health answered without one", () => {
    state.roster = [summary([])];
    state.health = { read: NONE };
    state.inventory = { target: "truenas", bound: 0, unmanaged: [] };
    renderTarget();

    expect(screen.getByText(/Not read just now\./)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Start draining" })).toBeNull();
  });

  it("does not count Waiting on you as zero when the findings read failed", () => {
    state.roster = [summary([])];
    state.health = { reachable: true, lifecycle: "active" };
    state.inventory = { target: "truenas", bound: 0, unmanaged: [] };
    state.findingsError = new Error("boom");
    renderTarget();

    expect(screen.getAllByLabelText("count could not be read").length).toBeGreaterThan(0);
  });

  // A-20 P5: the region keeps one card, and the card says so when it is true.
  it("says Nothing is waiting on you, in its card, when every read answered and nothing is waiting", () => {
    state.roster = [summary([])];
    state.health = { reachable: true, lifecycle: "active" };
    state.inventory = { target: "truenas", bound: 0, unmanaged: [] };
    renderTarget();

    const region = document.getElementById("waiting")!;
    expect(within(region).getByText("Nothing is waiting on you.")).toBeInTheDocument();
    expect(within(region).getByText("Nothing is waiting on you.").closest(".card")).not.toBeNull();
  });

  it("does not say nothing is waiting when a read failed: an unread region is not an empty one", () => {
    state.roster = [summary([])];
    state.health = { reachable: true, lifecycle: "active" };
    state.inventory = { target: "truenas", bound: 0, unmanaged: [] };
    state.findingsError = new Error("boom");
    renderTarget();

    expect(screen.queryByText("Nothing is waiting on you.")).toBeNull();
  });

  it("names the maintenance state in the Maintenance card, matching its action rows", () => {
    state.roster = [summary([])];
    state.health = { reachable: true, lifecycle: "draining" };
    state.inventory = { target: "truenas", bound: 0, unmanaged: [] };
    renderTarget();

    expect(screen.getByText("Maintenance")).toBeInTheDocument();
    expect(screen.getByText("Finishing up")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Make active" })).toBeInTheDocument();
  });

  it("does not double the product name beside its version", () => {
    state.roster = [summary([])];
    state.health = {
      reachable: true,
      lifecycle: "active",
      product: "TrueNAS",
      product_version: "TrueNAS-25.10.5",
    };
    state.inventory = { target: "truenas", bound: 0, unmanaged: [] };
    renderTarget();

    expect(screen.queryByText(/TrueNAS TrueNAS-25\.10\.5/)).toBeNull();
    expect(screen.getByText("TrueNAS 25.10.5")).toBeInTheDocument();
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
    };
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
    // The sheet is its result: Resolved and one Close, the form and its act are gone.
    expect(screen.getByText("Resolved")).toBeInTheDocument();
    expect(screen.queryByRole("textbox", { name: /type the system name/i })).toBeNull();
    expect(screen.queryByRole("button", { name: /accept this log and start over/i })).toBeNull();
    expect(screen.getByRole("button", { name: "Close" })).toBeInTheDocument();
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
    state.inventory = { target: "truenas", bound: 0, unmanaged: [] };
    renderTarget();

    // The board's words: "Stops and asks" / "Runs by itself" (route-system-targets-id).
    const purge = screen.getByText("Deletes an account").closest(".flex-wrap");
    expect(purge?.textContent).toContain("Stops and asks");

    const provision = screen.getByText("Creates an account").closest(".flex-wrap");
    expect(provision?.textContent).not.toContain("Stops and asks");
    expect(provision?.textContent).toContain("Runs by itself");
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
    state.inventory = { target: "truenas", bound: 0, unmanaged: [] };
    renderTarget();

    expect(screen.queryByText("What roles reach here")).toBeNull();
    expect(screen.queryByText("Published versions")).toBeNull();
  });

  it("offers the route that received them", () => {
    state.roster = [summary([])];
    state.health = { reachable: true, lifecycle: "active" };
    state.inventory = { target: "truenas", bound: 0, unmanaged: [] };
    renderTarget();

    const through = screen.getByRole("link", { name: /mapping/i });
    expect(through.getAttribute("href")).toBe("/system/targets/truenas/mappings");
  });

  // The setup strip is the one place that says it; the Accounts region repeats nothing.
  it("does not repeat the setup strip in a census card", () => {
    state.roster = [summary([])];
    state.health = { reachable: true, lifecycle: "active" };
    state.inventory = { target: "truenas", bound: 0, unmanaged: [] };
    renderTarget();

    expect(screen.queryByText(/Editing one changes access for everybody holding that role/)).toBeNull();
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
      { target: "truenas", subject_id: "s-12", username: "alexa.r", bound_by: "op", bound_at: "", last_seen_at: "" },
    ],
    unmanaged: [
      { username: "iyo", account_id: "3000" },
      { username: "alexa", account_id: "3002" },
    ],
    // Adoption is blocked off anything but a live or ageing read (§31 A), so
    // the fixture has to carry one for the form to be reachable at all.
    read: readNow(new Date().toISOString()),
  };

  function openAdoptForSai() {
    state.adopted = [];
    state.roster = [summary([])];
    state.health = { reachable: true, lifecycle: "active" };
    state.inventory = twoAccounts;
    renderTarget();
    // A to Z: alexa, then iyo.
    fireEvent.click(screen.getAllByRole("button", { name: "Assign to a person" })[1]);
  }

  function chooseSai() {
    fireEvent.change(screen.getByLabelText("Search people by name or email"), { target: { value: "sa" } });
    fireEvent.click(screen.getByRole("radio", { name: /Iyo Sky/ }));
  }

  // #57: a dialog about the account it was opened from.
  it("opens a dialog about the account it was opened from", () => {
    openAdoptForSai();
    const dialog = screen.getByRole("dialog");
    expect(within(dialog).getByText("Assign iyo to a person")).toBeInTheDocument();
    expect(dialog.textContent).not.toContain("alexa ");
  });

  // Red is this product's word for a click that takes access away. This one
  // gives somebody an account.
  it("does not dress a grant as a destruction", () => {
    openAdoptForSai();
    const submit = screen.getByRole("button", { name: /^Assign iyo/ });
    expect(submit.className, "dangerConfirm is reserved for removing access").not.toContain("bg-danger");
    expect(screen.getByText(/There is no undo/)).toBeInTheDocument();
    expect(screen.getByText(/Type the account name/)).toBeInTheDocument();
  });

  // Found by name or email, never by a pasted ID; namesakes told apart by
  // title, team and email; someone with an account here can't be chosen.
  it("finds the person by name, and won't give a second account", () => {
    openAdoptForSai();
    fireEvent.change(screen.getByLabelText("Search people by name or email"), { target: { value: "a" } });

    const iyo = screen.getByRole("radio", { name: /Iyo Sky/ }).closest("label")!;
    expect(iyo.textContent).toContain("Lab technician · Design");
    expect(iyo.textContent).toContain("iyo@example.edu");
    const kairiRadio = screen.getByRole("radio", { name: /Kairi Sane/ });
    const alexa = kairiRadio.closest("label")!;
    expect(kairiRadio).toBeDisabled();
    expect(alexa.textContent).toContain("Already has alexa.r on TrueNAS");
    expect(alexa.textContent).not.toMatch(/—/);
  });

  it("says what changes and what stays, once someone is chosen", () => {
    openAdoptForSai();
    expect(screen.queryByText("Changes")).toBeNull();
    chooseSai();
    expect(screen.getByRole("radio", { name: /Iyo Sky/ })).toBeChecked();
    expect(screen.getByText("Changes")).toBeInTheDocument();
    expect(screen.getByText(/belongs to/).textContent).toContain("Iyo Sky");
    expect(screen.getByText(/Its home folder, its shares and its group memberships/)).toBeInTheDocument();
  });

  it("assigns only once the name is typed, and reports it under the account", () => {
    openAdoptForSai();
    chooseSai();
    const submit = screen.getByRole("button", { name: "Assign iyo to Iyo Sky" });
    expect(submit.hasAttribute("disabled")).toBe(true);

    fireEvent.change(screen.getByLabelText(/Type the account name/), { target: { value: "iyo" } });
    fireEvent.click(screen.getByRole("button", { name: "Assign iyo to Iyo Sky" }));

    // With the account the row showed: a name handed to another account since is refused, never adopted.
    expect(state.adopted).toEqual([{ username: "iyo", subjectId: "s-99", accountId: "3000" }]);
    // The sheet is its result (Assigned, Close); no Dismiss button anywhere.
    expect(screen.getByRole("dialog")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /dismiss this result/i })).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "Close" }));
    expect(screen.queryByRole("dialog")).toBeNull();
    const outcome = screen.getByText(/bound to that person/).closest("[role=status]")!;
    expect(outcome.closest(".unfold")!.parentElement!.textContent).toContain("iyo");
    expect(screen.getByText("Assigned")).toBeInTheDocument();
    expect(outcome.textContent).toContain("iyo is now Iyo Sky");
  });

  // D-1 M5: the account just assigned leaves the unmanaged list on the next read, and its row is
  // what carries the outcome. The row stays until that outcome has been read.
  it("keeps the assigned account's row, and the outcome under it, when the next read drops it", () => {
    state.adopted = [];
    state.roster = [summary([])];
    state.health = { reachable: true, lifecycle: "active" };
    state.inventory = twoAccounts;
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const tree = () => (
      <QueryClientProvider client={client}>
        <UiViewProvider isOperator initialView="advanced">
          <TargetOverview target="truenas" />
        </UiViewProvider>
      </QueryClientProvider>
    );
    const { rerender } = render(tree());
    fireEvent.click(screen.getAllByRole("button", { name: "Assign to a person" })[1]);
    chooseSai();
    fireEvent.change(screen.getByLabelText(/Type the account name/), { target: { value: "iyo" } });
    fireEvent.click(screen.getByRole("button", { name: "Assign iyo to Iyo Sky" }));
    fireEvent.click(screen.getByRole("button", { name: "Close" }));

    state.inventory = { ...twoAccounts, unmanaged: twoAccounts.unmanaged.filter((a) => a.username !== "iyo") };
    rerender(tree());
    expect(screen.getByText(/iyo is now Iyo Sky/)).toBeTruthy();
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
    state.health = { reachable: true, lifecycle: "active", status: "answering", health: { state: "answering", recent: ["answered"] } };
    state.inventory = { target: "truenas", bound: 0, unmanaged: [] };
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
    expect(screen.getByText("Whether changes can land").closest("[aria-disabled]")).toBeNull();

    fireEvent.change(select, { target: { value: "p1" } });
    fireEvent.click(screen.getByRole("button", { name: "Use this project" }));
    expect(state.projectSet).toEqual(["p1"]);
    expect(screen.getByText("TrueNAS uses Network storage.")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Map its roles to TrueNAS groups" })).toBeInTheDocument();
    expect(fence()).toBeNull();
  });

  // D-1 Q4 (owner, 18 Nov 2026): the name is the settings card's first row, in every state, and the Name
  // region is gone, with nothing left pointing at it.
  it("opens its settings card with the name, before and after the project, and has no Name region", () => {
    for (const project of [null, { id: "p1", name: "Network storage" }]) {
      state.roster = [{ ...summary([]), project }];
      state.health = { reachable: true, lifecycle: "active" };
      state.inventory = { target: "truenas", bound: 0, unmanaged: [] };
      const { unmount } = renderTarget();
      const change = screen.getByRole("button", { name: "Change name" });
      const card = change.closest("[data-card-row]")!.parentElement as HTMLElement;
      expect(card.querySelector("[data-card-row]")).toBe(change.closest("[data-card-row]"));
      expect(within(card).queryByText("Zitadel project") === null).toBe(project === null);
      expect(screen.queryByText("What members see it called")).toBeNull();
      expect(document.getElementById("name")).toBeNull();
      expect(document.querySelector('a[href$="#name"]')).toBeNull();
      unmount();
    }
  });

  it("is a thin strip once set, and the only link to the mappings", () => {
    state.roster = [summary([])];
    state.health = { reachable: true, lifecycle: "active" };
    state.inventory = { target: "truenas", bound: 0, unmanaged: [] };
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
    state.inventory = { target: "truenas", bound: 0, unmanaged: [] };
    state.settings = { settings: [setting(value, values)] };
    state.settingPut = [];
    state.homes = {
      accounts: [
        { subject_id: "a", username: "charlotte", gives_home: true, home: "/mnt/p/main/home_dir/charlotte" },
        { subject_id: "b", username: "booker", gives_home: true, home: "/mnt/p/main/home_dir/booker" },
        { subject_id: "c", username: "cora", gives_home: false },
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
    // Nothing is chosen until the person picks: a placeholder, no default, and the primary is off
    // with its reason beside it.
    const select = within(dialog).getByLabelText(/Folder in main/) as HTMLSelectElement;
    expect(select.value).toBe("");
    expect(within(dialog).getByRole("option", { name: "Choose a folder…" })).toBeDisabled();
    const use = within(dialog).getByRole("button", { name: "Use this folder" });
    expect(use).toBeDisabled();
    expect(within(dialog).getByText("Choose a folder first.")).toBeInTheDocument();
    expect(state.settingPut).toEqual([]);
    fireEvent.change(select, { target: { value: "/mnt/p/main/home_dir" } });
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

  // The row used to be absent until the settings were read, so everything under it jumped when they
  // landed. It holds its place, with its button's seat filled, before they resolve.
  function openSettings(kind: string | undefined, settings: "loading" | "failed" | "unread" | "not offered") {
    state.roster = [{ ...summary([]), kind }];
    state.health = { reachable: true, lifecycle: "active" };
    state.inventory = { target: "truenas", bound: 0, unmanaged: [] };
    state.settingsLoading = settings === "loading";
    state.settingsError = settings === "failed" ? new Error("down") : null;
    state.settings =
      settings === "unread" ? { read: NONE } : settings === "failed" ? (undefined as never) : { settings: [] };
    return renderTarget();
  }
  const homeRow = () => (screen.queryByText("Home folders")?.closest("[data-card-row]") ?? null) as HTMLElement | null;

  it("holds its row, two lines tall as Not chosen yet is, while a storage system's settings are read", () => {
    openSettings("storage", "loading");
    const row = homeRow()!;
    const value = row.querySelector("[aria-busy]") as HTMLElement;
    expect(value.children).toHaveLength(2);
    expect(value.lastElementChild!.querySelector(".skeleton-bar")).not.toBeNull();
    expect(row.querySelector("[data-action-seat] .skeleton-bar")).not.toBeNull();
    expect(within(row).queryByRole("button")).toBeNull();
  });

  // A read that failed keeps the row, says so, and offers the step.
  it.each(["failed"] as const)("keeps the row when the settings are %s, with Try again", (how) => {
    openSettings("storage", how);
    const row = homeRow()!;
    expect(row).toHaveTextContent("Could not be read");
    expect(row).toHaveTextContent("Try again in a minute.");
    const value = within(row).getByText("Could not be read").parentElement as HTMLElement;
    expect(value.children).toHaveLength(2);
    fireEvent.click(within(row).getByRole("button", { name: "Try again" }));
    expect(state.settingsRefetches).toBe(1);
  });

  // A read with nothing to give (design §13.6) keeps the row and says why, in §13.1's words, never "not chosen".
  it("keeps the row when the settings are unread, and says why", () => {
    openSettings("storage", "unread");
    const row = homeRow()!;
    expect(row).toHaveTextContent("Syndra can't reach TrueNAS's add-on.");
    expect(row).not.toHaveTextContent("Not chosen yet");
  });

  // The same reads on a storage system draw the row (above), so each absence here is the kind's doing.
  it.each(["loading", "failed", "unread", "not offered"] as const)(
    "draws no row for a system that does not keep home folders (%s)",
    (how) => {
      const other = openSettings(undefined, how);
      expect(homeRow()).toBeNull();
      other.unmount();
      openSettings("storage", how);
      expect(homeRow() === null).toBe(how === "not offered");
    },
  );

  it("says when main has no folder to make homes in", () => {
    open("", []);
    fireEvent.click(screen.getByRole("button", { name: "Choose folder" }));
    const dialog = screen.getByRole("dialog");
    expect(within(dialog).getByText("No folder to offer")).toBeInTheDocument();
    expect(within(dialog).getByRole("button", { name: "Read again" })).toBeInTheDocument();
    expect(within(dialog).getByText(/Nothing to choose until/)).toBeInTheDocument();
  });
});

// A-13 (d): while TrueNAS isn't answering (its add-on answers from the copy it kept, design §13.3),
// its own cards are that copy.
// They recede to the page and keep every word at full strength; the controls
// that need it live are off with one reason each; Syndra's own cards stay
// raised. The 0.62 dim shipped twice before this rule replaced it.
describe("while TrueNAS isn't answering", () => {
  function openStale() {
    state.roster = [summary([])];
    state.health = { read: { state: "none", reason: "app_unreachable" }, status: "app_unreachable", health: { state: "app_unreachable", recent: [] } };
    state.inventory = {
      target: "truenas",
      bound: 1,
      accounts: [{ target: "truenas", subject_id: "s-99", username: "iyo", bound_by: "op", bound_at: "2026-10-01T00:00:00Z", last_seen_at: "2026-10-01T00:00:00Z" }],
      unmanaged: [{ username: "kiosk", account_id: "1001" }, { username: "lab", account_id: "1002" }],
      read: { state: "kept", taken_at: new Date().toISOString(), reason: "app_unreachable" },
    };
    state.settings = {
      settings: [
        {
          name: "home_root",
          label: "Where home folders go",
          value: "/mnt/p/main/homes",
          values: ["/mnt/p/main/homes"],
          labels: { "/mnt/p/main/homes": "main › homes" },
          values_read: true,
        },
      ],
    };
    renderTarget();
  }
  const card = (title: string) => screen.getByText(title).closest(".card") as HTMLElement;

  it("recedes its account cards and dims none of them", () => {
    openStale();
    for (const title of ["Accounts Syndra manages", "Not created by Syndra", "No longer needed"]) {
      expect(card(title), title).toHaveAttribute("data-stale");
      expectFullStrength(card(title));
    }
    // Syndra's own knowledge is current, so it stays raised.
    expect(card("Check for differences")).not.toHaveAttribute("data-stale");
  });

  it("turns off what needs it live, with one reason per card, and leaves what only queues", () => {
    openStale();
    const assign = screen.getAllByRole("button", { name: "Assign to a person" });
    expect(assign).toHaveLength(2);
    for (const b of assign) expect(b).toBeDisabled();
    expect(screen.getAllByText(/Assigning waits for a current read of TrueNAS/)).toHaveLength(1);

    expect(screen.getByRole("button", { name: "Check now" })).toBeDisabled();
    expect(screen.getByText("Check now waits too: it reads TrueNAS first.")).toBeInTheDocument();

    expect(screen.getByRole("button", { name: "Change folder" })).toBeDisabled();
    expect(screen.getByText("Change folder waits too: it lists the folders on TrueNAS.")).toBeInTheDocument();
  });

  it("is back to normal the moment it answers", () => {
    state.roster = [summary([])];
    state.health = { reachable: true, lifecycle: "active" };
    state.inventory = {
      target: "truenas",
      bound: 0,
      unmanaged: [{ username: "kiosk", account_id: "1001" }],
      read: readNow(new Date().toISOString()),
    };
    renderTarget();
    expect(card("Not created by Syndra")).not.toHaveAttribute("data-stale");
    expect(screen.getByRole("button", { name: "Assign to a person" })).toBeEnabled();
    expect(screen.getByRole("button", { name: "Check now" })).toBeEnabled();
    expect(screen.queryByText(/waits too/)).toBeNull();
  });
});

describe("what has been used here (A-23 R2)", () => {
  it("keeps the retention control in Advanced only, as the region's last card", async () => {
    const stub = makeProxyFetch();
    stub.register("GET", "/api/proxy/targets/truenas/stats", () => ({ ...targetStats(), target: "truenas" }));
    vi.stubGlobal("fetch", stub.fetchImpl);
    state.roster = [summary([])];
    state.health = { reachable: true, lifecycle: "active" };
    const { unmount } = renderTarget("advanced");
    const region = document.getElementById("stats") as HTMLElement;
    expect(await within(region).findByText(RETENTION_TITLE)).toBeInTheDocument();
    const cards = region.querySelectorAll(".card");
    expect(cards[cards.length - 1]).toHaveTextContent(RETENTION_TITLE);
    unmount();
    renderTarget("basic");
    const basic = document.getElementById("stats") as HTMLElement;
    expect(await within(basic).findByText(OVERVIEW_NEXT)).toBeInTheDocument();
    expect(within(basic).queryByText(RETENTION_TITLE)).toBeNull();
    vi.unstubAllGlobals();
  });
});

/**
 * D-1 pass 2: every action sits in its row's seat, the seat is the table's width, and a sentence
 * about a row is its detail line.
 */
describe("every action in its seat (D-1)", () => {
  const seatOf = (el: HTMLElement) => el.closest("[data-action-seat]") as HTMLElement | null;
  const cardOf = (title: string) => screen.getByText(title).closest(".card") as HTMLElement;
  const seatsIn = (card: HTMLElement) => Array.from(card.querySelectorAll("[data-card-row] > [data-action-seat]"));

  it("Syndra's reach: Open Maintenance is in the narrow seat, the sentence is the detail under the dot", () => {
    state.roster = [summary([])];
    state.health = { reachable: true, lifecycle: "read_only", lifecycle_note: "startup" };
    renderTarget("basic");
    const card = cardOf("Syndra's reach");
    const open = within(card).getByRole("button", { name: "Open Maintenance in Advanced" });
    expect(seatOf(open)).toHaveAttribute("data-seat", "narrow");
    const detail = card.querySelector("[data-row-detail]") as HTMLElement;
    expect(detail).toHaveTextContent(/Syndra can reach TrueNAS, but it is/);
    expect(detail.firstElementChild).toHaveClass("pl-(--dot-indent)");
    expect(open.closest("p")).toBeNull();
  });

  it("Syndra's reach: an active system keeps the empty narrow seat", () => {
    state.roster = [summary([])];
    state.health = { reachable: true, lifecycle: "active" };
    renderTarget();
    const seats = seatsIn(cardOf("Syndra's reach"));
    expect(seats).toHaveLength(1);
    expect(seats[0]).toHaveAttribute("data-seat", "narrow");
    expect(seats[0]).toBeEmptyDOMElement();
  });

  it("Check for differences: Check now is in the narrow seat of a top-aligned row", () => {
    state.roster = [summary([])];
    state.health = { reachable: true, lifecycle: "active" };
    renderTarget();
    const check = within(cardOf("Check for differences")).getByRole("button", { name: "Check now" });
    expect(seatOf(check)).toHaveAttribute("data-seat", "narrow");
    expect(check.closest("[data-card-row]")).toHaveClass("row-top");
  });

  it("accounts no longer there: the default seat holds the buttons, the sentence is the detail", () => {
    state.released = [];
    state.releaseResult = { status: "released" };
    state.roster = [summary([])];
    state.health = { reachable: true, lifecycle: "active" };
    state.inventory = { target: "truenas", bound: 0, unmanaged: [] };
    state.reconcile = {
      target: "truenas", bound: 4, queued: 1, read: readNow(),
      stale: [
        { subject_id: "s1", username: "becky", account_id: "3999", version: "v1" },
        { subject_id: "s2", username: "dana", account_id: "3998", version: "v2" },
      ],
    };
    renderTarget();
    const rowOf = (n: string) => screen.getByText(n).closest("[data-card-row]") as HTMLElement;
    // Asked: the sentence says what happens, under the first line, and both buttons are in the seat.
    expect(within(rowOf("becky")).getByText(/yours to decide/)).toHaveAttribute("data-row-detail");
    fireEvent.click(within(rowOf("becky")).getByRole("button", { name: "Stop tracking this account" }));
    const confirm = screen.getByRole("button", { name: "Stop tracking becky" });
    expect(seatOf(confirm)).not.toHaveAttribute("data-seat");
    expect(seatOf(screen.getByRole("button", { name: "Keep tracking it" }))).toBe(seatOf(confirm));
    expect(screen.getByText(/Nothing is deleted/).closest("[data-row-detail]")).not.toBeNull();
    // Done: the seat stays, empty, so the other row's button does not move.
    fireEvent.click(confirm);
    const done = rowOf("becky").querySelector("[data-action-seat]") as HTMLElement;
    expect(done).toBeEmptyDOMElement();
    expect(screen.getByText(/Stopped tracking it/)).toHaveAttribute("data-row-detail");
    const other = seatOf(within(rowOf("dana")).getByRole("button", { name: "Stop tracking this account" }))!;
    expect(other.dataset.seat).toBe(done.dataset.seat);
  });

  it("Not created by Syndra: Assign to a person and Syndra's own account share one seat", () => {
    state.roster = [summary([])];
    state.health = { reachable: true, lifecycle: "active" };
    state.inventory = {
      target: "truenas",
      bound: 0,
      read: readNow(new Date().toISOString()),
      unmanaged: [
        { username: "iyo", account_id: "3000" },
        { username: "syndra", account_id: "3001", self: true },
      ],
    };
    renderTarget();
    const assign = screen.getByRole("button", { name: "Assign to a person" });
    expect(seatOf(assign)).not.toHaveAttribute("data-seat");
    // Syndra's own row keeps its seat empty and its sentence on the detail line, where the other rows say
    // their next step, so the two read aligned.
    const own = screen.getByText(/cannot be assigned to anyone/);
    expect(own.closest("[data-row-detail]")).not.toBeNull();
    const ownSeat = own.closest("[data-card-row]")!.querySelector("[data-action-seat]");
    expect(ownSeat).toBeEmptyDOMElement();
    expect(ownSeat).not.toHaveAttribute("data-seat");
  });

  it("Maintenance: the sentence first, the verb in one narrow seat on every row", () => {
    state.roster = [summary([])];
    state.health = { reachable: true, lifecycle: "active" };
    renderTarget();
    const card = cardOf("Maintenance");
    const seats = seatsIn(card);
    // The state row, then Start draining and Set read-only.
    expect(seats).toHaveLength(3);
    for (const seat of seats) expect(seat).toHaveAttribute("data-seat", "narrow");
    const drain = within(card).getByRole("button", { name: "Start draining" });
    expect(seatOf(drain)).toBe(seats[1]);
    const row = drain.closest("[data-card-row]")!;
    expect(row.firstElementChild).toHaveTextContent(/Lets calls already sent finish/);
    // Asking why: the reason is a form, whose one button is the verb.
    fireEvent.click(drain);
    expect(seatOf(within(card).getByRole("button", { name: "Cancel" }))).toBe(seats[1]);
    const confirm = within(card).getByRole("button", { name: "Start draining" });
    expect(confirm.closest("form")).not.toBeNull();
    expect(seatOf(confirm)).toBeNull();
  });

  it("the long explanations are captions, not notes", () => {
    state.roster = [summary([])];
    state.health = { reachable: true, lifecycle: "active" };
    state.inventory = { target: "truenas", bound: 0, unmanaged: [] };
    renderTarget();
    for (const title of ["Check for differences", "Not created by Syndra"]) {
      const card = cardOf(title);
      expect(card.querySelector("[data-card-note]")).toBeNull();
      expect(card.querySelector("p[data-prose]")).toHaveTextContent(/Syndra/);
    }
  });
});

vi.mock("@/lib/queries/useFieldSchema", async () => (await import("@/lib/__fixtures__/field-schema-mock")).fieldSchemaMock);

// Syndra's server failed the page's read: Try again asks the add-on live
// (`?fresh=true`), never the copy that just failed (8b fix round 2).
describe("health read failed", () => {
  it("Try again reads the health live", async () => {
    state.healthError = new Error("502");
    state.healthLoading = true; // no data: the read failed before any came
    const fetch = vi.spyOn(globalThis, "fetch").mockResolvedValue(new Response(JSON.stringify({ reachable: true }), { status: 200 }));
    renderTarget();
    fireEvent.click(screen.getByRole("button", { name: "Try again" }));
    const health = () => fetch.mock.calls.map((c) => String(c[0])).filter((url) => url.includes("/targets/truenas/health"));
    await vi.waitFor(() => expect(health()).toEqual([expect.stringMatching(/\/targets\/truenas\/health\?fresh=true$/)]));
    fetch.mockRestore();
  });
});
