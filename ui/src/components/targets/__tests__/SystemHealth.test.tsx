// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { TargetOverview } from "@/components/targets/TargetOverview";
import type { TargetSystemHealth } from "@/lib/queries/useTargetSystemHealth";

/**
 * What the TARGET says about itself.
 *
 * `health.get` was declared, implemented, dispatched and policy'd from the day
 * the platform landed, and nothing called it — so "What it can do" listed a
 * capability with nothing behind it while the questions it answers went
 * unanswered. These assert the two distinctions the card exists to make: a
 * source that could not be read is not a source that reported nothing, and a
 * stopped `cifs` is the explanation for "my drive vanished" that no other read
 * in Syndra can give.
 */

const state: { health: TargetSystemHealth | undefined; loading: boolean } = {
  health: undefined,
  loading: false,
};

vi.mock("@/lib/queries/useTargetSystemHealth", () => ({
  useTargetSystemHealth: () => ({
    data: state.health,
    isLoading: state.loading,
    isError: false,
  }),
}));

// Everything else on the page is another component's subject. Declared inside
// the factory because `vi.mock` is hoisted above every const in this file.
vi.mock("@/lib/queries/useTargets", () => {
  const idle = () => ({ data: undefined, isLoading: false, isError: false, error: null, refetch: vi.fn() });
  const inert = () => ({ mutate: vi.fn(), mutateAsync: vi.fn(), isPending: false, error: null, data: undefined, reset: vi.fn() });
  return {
    useTargets: () => ({ ...idle(), data: [] }),
    useMergeFindings: () => ({ ...idle(), data: [] }),
    useTargetHealth: idle,
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

describe("what the target reports", () => {
  beforeEach(() => {
    state.health = undefined;
    state.loading = false;
  });

  it("says nothing at all while it is still asking", () => {
    state.loading = true;
    renderTarget();
    // Four calls to the NAS take a moment, and a card that flashes "nothing
    // raised" before the answer arrives has said something false.
    expect(screen.queryByText(/What the target reports/)).toBeNull();
  });

  it("names a failing disk the target reported", () => {
    state.health = {
      target: "truenas",
      readable: true,
      system: { hostname: "truenas", version: "25.10.5" },
      alerts: [
        {
          level: "WARNING",
          klass: "SMARTUncorrectedErrors",
          text: "1 uncorrectable errors reported for sde (SERIAL0000).",
          dismissed: false,
        },
      ],
      pools: [
        {
          name: "pool0",
          status: "ONLINE",
          healthy: true,
          warning: false,
          free_bytes: 38978043379712,
          allocated_bytes: 999512211456,
          size_bytes: 39977555591168,
        },
      ],
      services: [{ service: "cifs", state: "RUNNING", enable: true }],
    };
    renderTarget();

    expect(screen.getByText(/uncorrectable errors reported for sde/)).toBeInTheDocument();
    expect(screen.getByText(/pool0/)).toBeInTheDocument();
    // Binary steps, binary names.
    expect(screen.getByText(/930\.9 GiB of 36\.4 TiB used/)).toBeInTheDocument();
  });

  it("does not claim health for a report it could not read", () => {
    state.health = { target: "truenas", readable: false, detail: "no route to host" };
    renderTarget();

    expect(screen.getByText(/not a report that nothing is wrong/i)).toBeInTheDocument();
    expect(screen.queryByText(/Nothing raised/)).toBeNull();
  });

  it("names the source that failed rather than implying it was empty", () => {
    state.health = {
      target: "truenas",
      readable: true,
      pools: [
        {
          name: "tank",
          status: "ONLINE",
          healthy: true,
          warning: false,
          free_bytes: 1,
          allocated_bytes: 1,
          size_bytes: 2,
        },
      ],
      services: [{ service: "cifs", state: "RUNNING", enable: true }],
      degraded: ["alerts"],
    };
    renderTarget();

    expect(screen.getByText(/alerts could not be read/)).toBeInTheDocument();
    // Three sources answered and one did not; that is not "nothing raised".
    expect(screen.queryByText(/Nothing raised/)).toBeNull();
  });

  it("explains a stopped sharing service in terms of what it costs", () => {
    state.health = {
      target: "truenas",
      readable: true,
      pools: [],
      services: [
        { service: "cifs", state: "STOPPED", enable: true },
        { service: "nfs", state: "RUNNING", enable: true },
      ],
    };
    renderTarget();

    expect(screen.getByText(/File sharing \(SMB\) is not running/)).toBeInTheDocument();
    expect(screen.getByText(/Nobody can open their shares while it is stopped/)).toBeInTheDocument();
    // Set to start on boot and not running means it stopped by itself, which
    // is a different problem from somebody having turned it off.
    expect(screen.getByText(/it stopped by itself/)).toBeInTheDocument();
  });

  it("says so plainly when the target reported nothing wrong", () => {
    state.health = {
      target: "truenas",
      readable: true,
      alerts: [],
      pools: [
        {
          name: "tank",
          status: "ONLINE",
          healthy: true,
          warning: false,
          free_bytes: 1,
          allocated_bytes: 1,
          size_bytes: 2,
        },
      ],
      services: [{ service: "cifs", state: "RUNNING", enable: true }],
    };
    renderTarget();

    expect(screen.getByText(/Nothing raised/)).toBeInTheDocument();
  });

  it("ignores an alert somebody already dismissed on the target", () => {
    state.health = {
      target: "truenas",
      readable: true,
      alerts: [{ level: "WARNING", klass: "HasUpdate", text: "An update is available.", dismissed: true }],
      pools: [],
      services: [],
    };
    renderTarget();

    expect(screen.queryByText(/An update is available/)).toBeNull();
    expect(screen.getByText(/Nothing raised/)).toBeInTheDocument();
  });
});
