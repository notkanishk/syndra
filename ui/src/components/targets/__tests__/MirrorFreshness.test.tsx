// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { TargetOverview } from "@/components/targets/TargetOverview";

/**
 * The reachability card must not date the mirror while the target answers.
 *
 * `snapshot_taken_at` is the age of the copy in the ADD-ON's store, rewritten
 * only by a subjects read. Rendered unconditionally it sat two lines under
 * "answering · last answered just now" and said "was read 4 hours ago — too old
 * to act on — reload the page to read it again": two true sentences reading as
 * a contradiction, under a threshold meant for the adoption gate, with a way
 * out that cannot move the timestamp it points at.
 */

const FOUR_HOURS_AGO = new Date(Date.now() - 4 * 60 * 60_000).toISOString();

let currentHealth: Record<string, unknown> = {};
const NOW = () => ({ state: "now", taken_at: new Date().toISOString() });

vi.mock("@/lib/queries/useTargetSystemHealth", () => ({
  useTargetSystemHealth: () => ({ data: undefined, isLoading: false, error: null }),
}));

vi.mock("@/lib/queries/useTargets", async () => {
  const { healthReplyOf } = await vi.importActual<typeof import("@/lib/queries/useTargets")>("@/lib/queries/useTargets");
  const idle = () => ({
    data: undefined,
    isLoading: false,
    isError: false,
    error: null,
    refetch: vi.fn(),
  });
  const inert = () => ({
    mutate: vi.fn(),
    mutateAsync: vi.fn(),
    isPending: false,
    error: null,
    data: undefined,
    reset: vi.fn(),
  });
  return {
    useTargets: () => ({ ...idle(), data: [] }),
    useUnconfirmedOperations: () => ({ data: [], isLoading: false, error: null }),
    useMergeFindings: () => ({ ...idle(), data: [] }),
    useTargetHealthAgain: () => ({ press: async () => {} }),
    useTargetHealth: () => ({ ...idle(), data: healthReplyOf(currentHealth) }),
    useTargetInventory: idle,
    useTargetInventoryAgain: () => ({ press: async () => {}, pending: false, error: null }),
    useResolveLogFinding: inert,
    useReconcileTarget: inert,
    useSetLifecycle: inert,
    useReleaseBinding: inert,
    useAdoptAccount: inert,
  };
});

vi.mock("@/components/targets/MappingManagement", () => ({ MappingManagement: () => null }));
vi.mock("@/components/targets/PeopleOnTarget", () => ({ PeopleOnTarget: () => null }));
vi.mock("@/components/targets/DormantAccounts", () => ({ DormantAccounts: () => null }));
vi.mock("@/components/targets/MergeFindings", () => ({ MergeFindings: () => null }));

function renderTarget() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <TargetOverview target="truenas" />
    </QueryClientProvider>,
  );
}

describe("how old the add-on's mirror is", () => {
  it("is not raised at all while the target is answering", () => {
    currentHealth = {
      read: NOW(),
      reachable: true,
      product: "truenas_scale",
      product_version: "TrueNAS-25.10.5",
      lifecycle: "active",
      version_check: { state: "unchanged" },
      last_read_at: new Date().toISOString(),
      snapshot_taken_at: FOUR_HOURS_AGO,
    };
    renderTarget();

    // The card's own answer stands unqualified.
    expect(screen.getByText(/Accepting changes\. Nothing is in flight\./)).toBeInTheDocument();
    expect(screen.queryByText(/too old to act on/)).toBeNull();
    // And above all: no instruction that cannot change what it points at.
    expect(screen.queryByText(/reload the page to read it again/)).toBeNull();
  });

  // Design §13.3: with the add-on down there is no copy, so nothing on the page is dated by one.
  it("dates nothing once the add-on goes quiet, and says why", () => {
    currentHealth = {
      read: { state: "none", reason: "addon_unreachable", cause: "no_answer" },
      status: "addon_unreachable",
      health: { state: "addon_unreachable", cause: "no_answer", recent: ["missed", "missed", "missed"], since: FOUR_HOURS_AGO },
    };
    renderTarget();

    expect(screen.queryByText(/last state seen|no earlier copy/)).toBeNull();
    expect(screen.queryByText(/was read/)).toBeNull();
    expect(screen.getAllByText(/Syndra can't reach Truenas's add-on/i).length).toBeGreaterThan(0);
  });
});

// Slice 9c: a write on an untested version is held, not refused (TARGET_VERSION_UNSUPPORTED is
// 503, the row waits unspent), and the notice ends with the one step that releases it (copy guide §19).
describe("the untested-version notice", () => {
  it("says changes are held, and to update the add-on", () => {
    currentHealth = {
      read: NOW(),
      reachable: true,
      product: "truenas_scale",
      product_version: "TrueNAS-26.04.0",
      lifecycle: "active",
      version_check: { state: "changed" },
      last_read_at: new Date().toISOString(),
    };
    renderTarget();

    expect(
      screen.getByText(
        /This version of Truenas has not been tested with Syndra\. Reads keep working; changes are held\. Update the add-on\.$/,
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText(/refused/)).toBeNull();
  });
});
