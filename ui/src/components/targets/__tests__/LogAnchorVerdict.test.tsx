// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { TargetOverview } from "@/components/targets/TargetOverview";

/**
 * The log-tamper sentence has to follow the numbers, not the verdict.
 *
 * `ClassifyLogHead` returns `head_rewritten` for two different things
 * (`db/log_anchor.go:75-84`): the same number of records hashing to something
 * else, and records having GROWN while the head stayed where it was. The card
 * said "the number of entries is the same, but their contents changed" for
 * both — so the second case, entries appended that chain onto nothing Syndra
 * verified, was described as its own opposite on the one screen in the product
 * that exists to notice tampering.
 */

const anchor = (records: number, violationRecords: number) => ({
  target: "truenas",
  records,
  violation_records: violationRecords,
  violation_reason: "head_rewritten",
  head: "abcdef0123456789",
  anchored_at: "2026-08-01T10:00:00Z",
  violation_at: "2026-08-02T10:00:00Z",
});

let currentAnchor = anchor(10, 10);

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
    useTargetHealth: () => ({ ...idle(), data: { log_anchor: currentAnchor } }),
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

describe("what the log anchor says was done to the log", () => {
  it("says entries are gone when the target reports fewer than Syndra saw", () => {
    currentAnchor = anchor(10, 7);
    renderTarget();
    expect(screen.getByText(/Entries that existed are gone/)).toBeInTheDocument();
  });

  it("says the contents changed when the count is unchanged", () => {
    currentAnchor = anchor(10, 10);
    renderTarget();
    expect(screen.getByText(/number of entries is the same, but their contents changed/)).toBeInTheDocument();
  });

  // The branch that was told backwards.
  it("does not claim the count is unchanged when entries were added", () => {
    currentAnchor = anchor(10, 14);
    renderTarget();

    expect(screen.getByText(/Entries were added, but the log still ends where it did/)).toBeInTheDocument();
    expect(screen.queryByText(/number of entries is the same/)).toBeNull();
  });
});
