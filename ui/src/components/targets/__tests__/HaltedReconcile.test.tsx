// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { TargetOverview } from "@/components/targets/TargetOverview";

/**
 * A pass that concluded nothing must not report three zeroes.
 *
 * `halt()` in the add-on reconciler returns the result struct untouched —
 * `bound`, `queued` and `stale` are still at their zero values because nothing
 * was ever measured. The card rendered them anyway: "0 accounts managed · 0
 * fixes waiting to be sent · 0 accounts missing", which is what a healthy,
 * fully-converged system also looks like. The two states that most need
 * telling apart were displayed identically, and the flag that distinguishes
 * them was missing from the UI's own type.
 */

vi.mock("@/lib/queries/useTargetSystemHealth", () => ({
  useTargetSystemHealth: () => ({
    data: undefined,
    isLoading: false,
    error: null,
  }),
}));

// Every helper lives INSIDE the factory: `vi.mock` is hoisted above every
// const in the file, so a top-level one is not yet initialised when it runs.
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
    useTargetHealth: idle,
    useTargetInventory: idle,
    useResolveLogFinding: inert,
    useResolveBindingConflict: inert,
    useSetLifecycle: inert,
    useReleaseBinding: inert,
    useAdoptAccount: inert,
    useReconcileTarget: () => ({
      ...inert(),
      data: {
        target: "truenas",
        bound: 0,
        queued: 0,
        current: false,
        halted: true,
        reason: "target unreachable",
      },
    }),
  };
});

vi.mock("@/components/targets/MappingManagement", () => ({
  MappingManagement: () => null,
}));
vi.mock("@/components/targets/PeopleOnTarget", () => ({
  PeopleOnTarget: () => null,
}));
vi.mock("@/components/targets/DormantAccounts", () => ({
  DormantAccounts: () => null,
}));
vi.mock("@/components/targets/MergeFindings", () => ({
  MergeFindings: () => null,
}));

function renderTarget() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={client}>
      <TargetOverview target="truenas" />
    </QueryClientProvider>,
  );
}

describe("a reconcile pass that halted", () => {
  it("says nothing was concluded, and does not report its unmeasured zeroes", () => {
    renderTarget();

    expect(
      screen.getByText(/stopped before it concluded anything/i),
    ).toBeInTheDocument();
    expect(screen.getAllByText(/target unreachable/i).length).toBeGreaterThan(
      0,
    );

    // The exact sentence a healthy run produces. A halted pass must never be
    // able to render it.
    expect(screen.queryByText(/0 accounts managed/)).toBeNull();
    expect(screen.queryByText(/fixes waiting to be sent/)).toBeNull();
  });
});
