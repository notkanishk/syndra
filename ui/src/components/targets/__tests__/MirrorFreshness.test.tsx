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

vi.mock("@/lib/queries/useTargetSystemHealth", () => ({
  useTargetSystemHealth: () => ({ data: undefined, isLoading: false, error: null }),
}));

vi.mock("@/lib/queries/useTargets", () => {
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
    useMergeFindings: () => ({ ...idle(), data: [] }),
    useTargetHealth: () => ({ ...idle(), data: currentHealth }),
    useTargetInventory: idle,
    useResolveLogFinding: inert,
    useResolveBindingConflict: inert,
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
      reachable: true,
      product: "truenas_scale",
      product_version: "TrueNAS-25.10.5",
      lifecycle: "active",
      version_tested: true,
      last_read_at: new Date().toISOString(),
      snapshot_taken_at: FOUR_HOURS_AGO,
    };
    renderTarget();

    // The card's own answer stands unqualified.
    expect(screen.getByText(/talking to TrueNAS normally/)).toBeInTheDocument();
    expect(screen.queryByText(/too old to act on/)).toBeNull();
    // And above all: no instruction that cannot change what it points at.
    expect(screen.queryByText(/reload the page to read it again/)).toBeNull();
  });

  it("says what is on screen is a copy, and how old, once the target goes quiet", () => {
    currentHealth = {
      reachable: false,
      detail: "the target could not be read",
      snapshot_taken_at: FOUR_HOURS_AGO,
    };
    renderTarget();

    expect(screen.getByText(/this is the last state seen/)).toBeInTheDocument();
    // `provisional`, never `stale`: a copy is a copy at any age, and the
    // ten-minute threshold belongs to the adoption gate.
    expect(screen.queryByText(/too old to act on/)).toBeNull();
  });

  it("does not imply an earlier copy exists when none does", () => {
    currentHealth = { reachable: false, detail: "the target could not be read" };
    renderTarget();

    expect(screen.getByText(/no earlier copy to show/)).toBeInTheDocument();
  });
});
