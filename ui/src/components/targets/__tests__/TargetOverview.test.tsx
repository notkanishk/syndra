// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { TargetOverview } from "@/components/targets/TargetOverview";
import type { TargetHealth, TargetInventory, TargetSummary } from "@/lib/queries/useTargets";

// 9.2/9.20/9.21 — the operation set comes from the manifest, and the health
// states render as distinct things rather than as one "status".

const state = {
  roster: [] as TargetSummary[],
  health: {} as TargetHealth,
  inventory: {} as TargetInventory,
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
  };
});

function summary(operations: TargetSummary["operations"]): TargetSummary {
  return {
    target: "truenas",
    registered: true,
    auth_mode: "mtls",
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
