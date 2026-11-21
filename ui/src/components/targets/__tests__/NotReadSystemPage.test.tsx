// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import ConnectedSystemsPage from "@/app/system/targets/page";
import { TargetOverview } from "@/components/targets/TargetOverview";
import { UiViewProvider } from "@/lib/ui-view";
import type { TargetSummary } from "@/lib/queries/useTargets";
import { asRead, NONE } from "@/test-utils/wire";

/**
 * Design §13.6's guard for the system pages: with the add-on down, every add-on
 * read is `none` (503 TARGET_NOT_READ, or the health route's `read: none`), and
 * no consumer draws a 0, an empty state or a date from it. Each says why, in
 * §13.1's words, with what to check.
 */

const WHY = /Syndra can't reach TrueNAS's add-on\. Check that its container is running\./;
const EMPTY = /Nothing outside Syndra\.|Nobody yet\.|Nothing raised|No longer needed 0|Nothing waiting/;
// A read's date in any of the product's forms: a month, a clock time, an age.
const DATE = /\b(Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)\b|\b\d{1,2}:\d{2}\b|\bago\b|last state seen|last known/i;

const roster: TargetSummary[] = [
  {
    target: "truenas",
    name: "TrueNAS",
    name_from: "operator",
    registered: true,
    auth_mode: "derived",
    callable: true,
    operations: [],
    circuit_open: false,
    status: "addon_unreachable",
    health: { state: "addon_unreachable", cause: "no_answer", recent: ["missed", "missed", "missed"] },
    project: { id: "p1", name: "Makerspace" },
    kind: "storage",
  },
];

const state = { none: true };

vi.mock("@/lib/queries/useTargetSystemHealth", () => ({
  useTargetSystemHealth: () => ({ data: asRead({ target: "truenas", read: NONE }), isLoading: false, isError: false, error: null }),
}));
vi.mock("@/lib/queries/useTargets", async () => {
  const actual = await vi.importActual<typeof import("@/lib/queries/useTargets")>("@/lib/queries/useTargets");
  const inert = () => ({ mutate: vi.fn(), mutateAsync: vi.fn(), isPending: false, error: null, data: undefined, reset: vi.fn() });
  const none = () => ({ data: asRead({ read: NONE }), isLoading: false, isError: false, error: null, refetch: vi.fn() });
  return {
    ...actual,
    useTargets: () => ({ data: roster, isLoading: false, error: null, refetch: vi.fn() }),
    useTargetHealth: () => ({
      data: actual.healthReplyOf({ read: NONE, status: "addon_unreachable", health: roster[0].health }),
      isLoading: false,
      isError: false,
      error: null,
    }),
    useTargetHealthAgain: () => ({ press: async () => {}, pending: false, error: null }),
    useTargetInventory: none,
    useTargetInventoryAgain: () => ({ press: async () => {}, pending: false, error: null }),
    useTargetHomes: none,
    useTargetSettings: none,
    useMergeFindings: () => ({ data: [], isLoading: false, isError: false, error: null }),
    useUnconfirmedOperations: () => ({ data: [], isLoading: false, error: null }),
    useResolveLogFinding: inert,
    useReconcileTarget: inert,
    useSetLifecycle: inert,
    useReleaseBinding: inert,
    useAdoptAccount: inert,
    useSetTargetSetting: inert,
  };
});
vi.mock("@/components/targets/MappingManagement", () => ({ MappingManagement: () => null }));
vi.mock("@/components/targets/DormantAccounts", () => ({ DormantAccounts: () => null }));
vi.mock("@/components/targets/MergeFindings", () => ({ MergeFindings: () => null }));
vi.mock("@/components/usage/TargetStats", () => ({ TargetStats: () => null }));
vi.mock("@/lib/queries/useFieldSchema", async (orig) => (await import("@/lib/__fixtures__/field-schema-mock")).withRosterNames(orig));

const wrap = (ui: React.ReactNode) =>
  render(
    <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
      <UiViewProvider isOperator initialView="advanced">
        {ui}
      </UiViewProvider>
    </QueryClientProvider>,
  );

beforeEach(() => {
  state.none = true;
});

// Syndra's own records stand outside the read (design §13.3) and keep their counts, 0 included:
// what waits on you, and what the accepted manifest says it can do.
const OWN = /^(Waiting on you|What runs here|What it can do)/;
const where = (e: Element) => e.closest("a,section,[data-card-row],.card")?.textContent ?? "";

function expectNoFalseReading() {
  expect(screen.queryAllByText("0").map(where).filter((w) => !OWN.test(w))).toEqual([]);
  expect(Array.from(document.querySelectorAll("[data-testid=card-count]")).map(where).filter((w) => !OWN.test(w))).toEqual([]);
  expect(screen.queryAllByText(EMPTY)).toHaveLength(0);
  expect(document.body.textContent).not.toMatch(DATE);
}

describe("a system whose add-on is down", () => {
  it("its page draws no count, no empty state and no date, and says why", () => {
    wrap(<TargetOverview target="truenas" />);
    expectNoFalseReading();
    // The account list, what the system reports, and the fault Notice each say it.
    expect(screen.getAllByText(WHY).length).toBeGreaterThanOrEqual(3);
    // The Accounts chip carries no count.
    expect(screen.getAllByRole("link", { name: /^Accounts/ }).every((a) => !/\d/.test(a.textContent ?? ""))).toBe(true);
  });

  it("its row on Connected systems has no count, and says it was not read", () => {
    wrap(<ConnectedSystemsPage />);
    expectNoFalseReading();
    expect(screen.getAllByText("Not read").length).toBeGreaterThan(0);
    expect(screen.getByText(WHY)).toBeInTheDocument();
  });
});
