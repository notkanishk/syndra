// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { TargetOverview } from "@/components/targets/TargetOverview";
import type { TargetSystemHealth } from "@/lib/queries/useTargetSystemHealth";
import { expectFullStrength } from "@/test-utils/fullStrength";
import type { WireRead } from "@/lib/queries/read";
import { asRead, NONE, readNow } from "@/test-utils/wire";

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

type Report = Omit<TargetSystemHealth, "read"> & { read?: WireRead };
const state: { health: Report | undefined; loading: boolean; reachable: boolean | undefined } = {
  health: undefined,
  loading: false,
  reachable: undefined,
};

// The report was last read at 09:41, local time.
const READ_AT = new Date(2026, 10, 3, 9, 41).getTime();

vi.mock("@/lib/queries/useFieldSchema", async (orig) => (await import("@/lib/__fixtures__/field-schema-mock")).withRosterNames(orig));
vi.mock("@/lib/queries/useTargetSystemHealth", () => ({
  useTargetSystemHealth: () => ({
    data: state.health && asRead(state.health),
    dataUpdatedAt: READ_AT,
    isLoading: state.loading,
    isError: false,
  }),
}));

// Everything else on the page is another component's subject. Declared inside
// the factory because `vi.mock` is hoisted above every const in this file.
vi.mock("@/lib/queries/useTargets", async () => {
  const { healthReplyOf } = await vi.importActual<typeof import("@/lib/queries/useTargets")>("@/lib/queries/useTargets");
  const idle = () => ({ data: undefined, isLoading: false, isError: false, error: null, refetch: vi.fn() });
  const inert = () => ({ mutate: vi.fn(), mutateAsync: vi.fn(), isPending: false, error: null, data: undefined, reset: vi.fn() });
  return {
    useTargets: () => ({ ...idle(), data: [] }),
    useUnconfirmedOperations: () => ({ data: [], isLoading: false, error: null }),
    useMergeFindings: () => ({ ...idle(), data: [] }),
    useTargetHealthAgain: () => ({ press: async () => {} }),
    useTargetHealth: () => ({
      ...idle(),
      data:
        state.reachable === undefined
          ? undefined
          : healthReplyOf({ read: state.reachable ? readNow() : NONE, reachable: state.reachable, lifecycle: "active" }),
    }),
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

describe("what the target reports", () => {
  beforeEach(() => {
    state.health = undefined;
    state.loading = false;
    state.reachable = undefined;
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

  // D-1, from staging (18 Nov, 1481px): the sentence was a detail line capped at the measure, so it
  // fitted beside a short title on the half-width card and sat there, squeezing the title to a word a
  // line ("A / disk / reports / errors…"). jsdom draws no layout, so this holds the structure the
  // layout comes from; design-system.test.ts holds the CSS (the detail line is never capped by a
  // max-width, so it always takes a line of its own).
  it("gives each reading's title the row's first line and its sentence the line under it", () => {
    state.health = {
      target: "truenas",
      system: { hostname: "truenas", version: "25.10.5" },
      alerts: [
        {
          level: "CRITICAL",
          klass: "SMARTUncorrectedErrors",
          text: "Device: /dev/sdc [SAT], 8 Offline uncorrectable sectors.",
          at: "2026-11-17T22:04:00Z",
          dismissed: false,
        },
      ],
      pools: [
        { name: "raid-z1-slow", status: "DEGRADED", healthy: false, warning: false, free_bytes: 1, allocated_bytes: 2, size_bytes: 3 },
      ],
      services: [{ service: "cifs", state: "RUNNING", enable: true }],
    };
    renderTarget();

    for (const title of ["A disk reports errors it could not correct", "Storage pool raid-z1-slow"]) {
      const words = screen.getByText(title);
      const row = words.closest("[data-card-row]") as HTMLElement;
      const detail = row.querySelector("[data-row-detail]") as HTMLElement;
      expect(detail, title).toHaveClass("row-detail");
      expect(detail).not.toContainElement(words);
      // The title's part flexes; it is never a fixed-width label column beside the sentence.
      const part = row.firstElementChild as HTMLElement;
      expect(part).toContainElement(words);
      expect(part.className).toMatch(/(^|\s)flex-1(\s|$)/);
      expect(part.className).not.toMatch(/(^|\s)w-[[(]/);
    }
    expect(screen.getByText(/8 Offline uncorrectable sectors/).closest("[data-row-detail]")).not.toBeNull();
  });

  // Nothing to give (503 TARGET_NOT_READ): why, in §13.1's words, never "nothing raised" and no date.
  it("does not claim health for a report it could not read, and says why", () => {
    state.health = { target: "truenas", read: { state: "none", reason: "addon_unreachable", cause: "timeout" } };
    renderTarget();

    expect(screen.getByText(/TrueNAS's add-on started answering and stopped\. Syndra keeps trying\./)).toBeInTheDocument();
    expect(screen.queryByText(/Nothing raised/)).toBeNull();
    expect(screen.queryByText(/Last known|All read|Partly read/)).toBeNull();
  });

  it("names the source that failed rather than implying it was empty", () => {
    state.health = {
      target: "truenas",
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
      alerts: [{ level: "WARNING", klass: "HasUpdate", text: "An update is available.", dismissed: true }],
      pools: [],
      services: [],
    };
    renderTarget();

    expect(screen.queryByText(/An update is available/)).toBeNull();
    expect(screen.getByText(/Nothing raised/)).toBeInTheDocument();
  });

  // A-13 (d): while the system isn't answering, the card is its last read. It
  // recedes to the page, its dots ring in their own colour, its age is said
  // once in its note, and nothing in it is dimmed.
  describe("while the system isn't answering", () => {
    const report: Report = {
      target: "truenas",
      alerts: [{ level: "WARNING", klass: "SMARTUncorrectedErrors", text: "1 uncorrectable error on sde.", dismissed: false }],
      pools: [
        { name: "tank", status: "ONLINE", healthy: true, warning: false, free_bytes: 1, allocated_bytes: 1, size_bytes: 2 },
      ],
      services: [],
    };
    const card = () => screen.getByText("What TrueNAS reports").closest(".card") as HTMLElement;

    it("recedes, rings its dots, says its age once, and dims nothing", () => {
      state.health = report;
      state.reachable = false;
      renderTarget();

      expect(card()).toHaveAttribute("data-stale");
      expect(card()).toHaveAttribute("data-recedes");
      expectFullStrength(card());
      expect(screen.getAllByText(/Last known · 09:41/)).toHaveLength(1);
      // The pool still reads healthy and the alert still reads amber: a ring
      // of each dot's own colour, never all amber.
      const dots = Array.from(card().querySelectorAll("[data-dot]"));
      expect(dots.some((d) => d.className.includes("text-healthy"))).toBe(true);
      expect(dots.some((d) => d.className.includes("text-warn"))).toBe(true);
    });

    it("rises back, filled, once it answers", () => {
      state.health = report;
      state.reachable = true;
      renderTarget();

      expect(card()).not.toHaveAttribute("data-stale");
      // Still marked as a card that can recede, so the change back is a tint.
      expect(card()).toHaveAttribute("data-recedes");
      expect(screen.queryByText(/Last known/)).toBeNull();
      expect(card().querySelector('[data-dot].bg-healthy')).not.toBeNull();
    });
  });
});
