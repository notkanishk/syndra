// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { TargetOverview } from "@/components/targets/TargetOverview";
import type { TargetHealth, TargetInventory, TargetSummary } from "@/lib/queries/useTargets";

// 9.2/9.20/9.21 — the operation set comes from the manifest, and the health
// states render as distinct things rather than as one "status".

const state = {
  roster: [] as TargetSummary[],
  health: {} as TargetHealth,
  inventory: {} as TargetInventory,
  resolved: [] as Array<{ head: string; note: string }>,
  ownerDecided: [] as Array<{ id: string; owner: string; note: string }>,
};

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
    useAdoptAccount: () => ({ mutate: vi.fn(), isPending: false }),
    useSetLifecycle: () => ({ mutate: vi.fn(), isPending: false }),
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

    expect(screen.getByText(/Transport secret unreadable/i)).toBeTruthy();
    // The path, because "no secret configured" and "the mount is missing" are
    // the same symptom and different fixes.
    expect(screen.getByText(/run\/secrets\/addon\/truenas\.key/)).toBeTruthy();
  });

  it("does not claim a transport fault when the secret loads", () => {
    state.roster = [{ ...summary([]), transport_status: "ok" }];
    state.health = { reachable: true, lifecycle: "active" };
    state.inventory = { target: "truenas", bound: 0, unmanaged: [], current: true };
    renderTarget();

    expect(screen.queryByText(/Transport secret unreadable/i)).toBeNull();
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

    expect(screen.getByText(/has not published a capability manifest/i)).toBeInTheDocument();
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
    expect(screen.getByText(/Set deliberately: rotating the API key/)).toBeInTheDocument();
    expect(screen.queryByText(/did not answer/i)).toBeNull();
    expect(screen.queryByText(/Not answering/)).toBeNull();
  });

  it("distinguishes Syndra backing off from the target being down", () => {
    state.roster = [summary([])];
    state.health = { reachable: true, lifecycle: "active", circuit_open: true };
    renderTarget();

    expect(screen.getByText(/refusing its own calls/i)).toBeInTheDocument();
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
    expect(screen.getByText(/Adoption needs a current read/i)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Adopt" })).toBeNull();
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
    expect(screen.getByText(/These are not drift/i)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /revoke/i })).toBeNull();
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
      screen.getByRole("button", { name: /adopt this log as the baseline/i }),
    ).toBeDisabled();
  });

  it("takes rung 3 and an explanation, and cites the head that was read", () => {
    renderWithFinding();
    fireEvent.click(screen.getByRole("button", { name: /resolve this finding/i }));
    const confirm = screen.getByRole("button", { name: /adopt this log as the baseline/i });

    fireEvent.change(screen.getByLabelText(/Why the log changed/), {
      target: { value: "we replaced the volume" },
    });
    expect(confirm).toBeDisabled();

    fireEvent.change(screen.getByRole("textbox", { name: /type the target/i }), {
      target: { value: "truenas" },
    });
    expect(confirm).toBeEnabled();

    fireEvent.click(confirm);
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
    expect(screen.getByText(/The change landed on the target/)).toBeInTheDocument();
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
    expect(record).toBeEnabled();

    fireEvent.click(record);
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
    expect(screen.getByText(/queued for both people/)).toBeInTheDocument();
  });
});
