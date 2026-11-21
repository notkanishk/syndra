// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

import { HomeLocationRow } from "@/components/kinds/storage/HomeLocation";
import { PersonActivity } from "@/components/people/PersonActivity";
import { DormantAccounts } from "@/components/targets/DormantAccounts";
import { MappingManagement } from "@/components/targets/MappingManagement";
import { ApiError } from "@/lib/api-client";
import { notReadOf } from "@/lib/queries/read";

/**
 * Design §13.6, on every list this slice owns: a read with nothing to give (a 503
 * TARGET_NOT_READ, as the hooks hand it over) draws §13.1's sentence and its next
 * step, and never a 0, an empty state or a date.
 */
const SENTENCE = "Syndra can't reach TrueNAS's add-on. Check that its container is running. Syndra keeps trying.";
const none = notReadOf(
  new ApiError(503, { error: "TARGET_NOT_READ", message: SENTENCE, details: { reason: "addon_unreachable", cause: "no_answer" }, retry_after: 30 }),
)!;
const query = { data: none, isLoading: false, isFetching: false, isError: false, error: null, refetch: () => {} };

vi.mock("@/lib/queries/useTargets", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/queries/useTargets")>()),
  useTargets: () => ({ data: [{ target: "truenas", name: "TrueNAS", kind: "storage" }], isLoading: false, error: null }),
  useTargetSettings: () => query,
  useTargetReach: () => query,
  useTargetReachAgain: () => ({ press: async () => {}, pending: false, error: null }),
}));
vi.mock("@/lib/queries/useDormant", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/queries/useDormant")>()),
  useDormantAccounts: () => query,
  useSweepDormant: () => ({ mutate: () => {}, isPending: false, error: null }),
}));
vi.mock("@/lib/queries/useTargetActivity", () => ({
  useTargetActivity: () => query,
  useTargetActivityAgain: () => ({ press: async () => {}, pending: false, error: null }),
}));
vi.mock("@/lib/queries/useAudit", () => ({
  useAuditEntries: () => ({ data: [], isLoading: false, error: null, refetch: () => {} }),
}));
vi.mock("@/lib/queries/useMappings", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/queries/useMappings")>()),
  useMappings: () => ({ data: [], isLoading: false, error: null, refetch: () => {} }),
  useMappingHistory: () => ({ data: { target: "truenas", current_version: 0, unpublished_changes: [], unpublished: false, versions: [] }, isLoading: false, error: null }),
}));
vi.mock("@/lib/queries/useFieldSchema", async () => (await import("@/lib/__fixtures__/field-schema-mock")).fieldSchemaMock);
vi.mock("@/components/names", () => ({ UserName: () => null }));
vi.mock("@/components/audit/TraceCell", () => ({ TraceCell: () => null }));

const wrap = (node: ReactNode) =>
  render(<QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>{node}</QueryClientProvider>);

/** No count, no empty state, no date: the three things a read that did not happen must never draw. */
function expectNothingFromANonRead() {
  expect(screen.getAllByText(SENTENCE).length).toBeGreaterThan(0);
  expect(screen.queryByTestId("card-count")).toBeNull();
  expect(document.body.textContent).not.toMatch(/\b0\b/);
  expect(document.body.textContent).not.toMatch(/Nothing to tidy|No recorded activity|Not chosen yet|Opens nothing|lists nothing/);
  expect(document.body.textContent).not.toMatch(/\b(was read|read from|ago|\d\d:\d\d|1 Jan)\b/i);
}

describe("a list with nothing to give", () => {
  it.each([
    ["the dormant list", () => <DormantAccounts target="truenas" />],
    ["a person's activity on a system", () => <PersonActivity userId="u-cena" name="John Cena" />],
    ["where home folders go", () => <HomeLocationRow target="truenas" lastKnown={false} expected />],
    ["what each group opens", () => <MappingManagement target="truenas" />],
  ])("%s says why, and draws no count, empty state or date", (_name, node) => {
    wrap(node());
    // What each group opens sits above the mappings, which are Syndra's own (an empty mapping list is real).
    if (_name !== "what each group opens") expectNothingFromANonRead();
    else {
      expect(screen.getByText(SENTENCE)).toBeInTheDocument();
      expect(document.body.textContent).not.toMatch(/folder permissions|Opens nothing/);
    }
  });
});
