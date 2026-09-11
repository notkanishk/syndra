// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import PendingChangesPage from "@/app/governance/pending/page";
import type { PendingRow } from "@/lib/queries/usePropagation";
import type { BundleRow } from "@/lib/queries/useBundles";
import type { MappingRuleRow } from "@/lib/queries/useMappingRules";

const pending = vi.hoisted(() => ({ data: [] as PendingRow[] }));
const summary = vi.hoisted(() => ({ reachable: true }));
const bundles = vi.hoisted(() => ({ data: [] as BundleRow[] }));
const rules = vi.hoisted(() => ({ data: [] as MappingRuleRow[] }));

vi.mock("@/lib/queries/usePropagation", () => ({
  usePendingPropagations: () => ({ ...pending, isLoading: false, error: null, refetch: () => {} }),
  useDrainPropagations: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));

vi.mock("@/lib/queries/useBundles", () => ({
  useBundles: () => ({ ...bundles, isLoading: false, error: null }),
}));

vi.mock("@/lib/queries/useMappingRules", () => ({
  useMappingRules: () => ({ ...rules, isLoading: false, error: null }),
}));

vi.mock("@/lib/queries/useGovernance", () => ({
  useGovernanceSummary: () => ({
    data: { pending_propagation: { count: 0, zitadel_reachable: summary.reachable } },
  }),
}));

function row(overrides: Partial<PendingRow> = {}): PendingRow {
  return {
    id: "o1",
    op_type: "add",
    user_id: "u1",
    project_id: "p1",
    role_keys: ["trained"],
    source: "rule",
    source_ref: "rule-1111-2222",
    cascade_id: "cas-8841-aaaa",
    status: "pending",
    attempts: 0,
    created_at: "2026-07-31T09:42:00Z",
    ...overrides,
  } as PendingRow;
}

function renderPending() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <PendingChangesPage />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  pending.data = [];
  summary.reachable = true;
  bundles.data = [];
  rules.data = [];
});

describe("Pending changes", () => {
  it("says two writes belong to one cascade rather than leaving them looking unrelated", () => {
    pending.data = [row({ id: "o1" }), row({ id: "o2", role_keys: ["door"] })];
    renderPending();
    expect(
      screen.getByText(/These 2 changes come from one edit/),
    ).toBeInTheDocument();
    expect(screen.getByText(/They are sent together or not at all\./)).toBeInTheDocument();
  });

  it("does not claim a lone write is a cascade", () => {
    pending.data = [row()];
    renderPending();
    expect(screen.queryByText(/share cascade/)).not.toBeInTheDocument();
  });

  it("keeps writes from different events apart", () => {
    pending.data = [
      row({ id: "o1", cascade_id: "cas-aaaa" }),
      row({ id: "o2", cascade_id: "cas-bbbb" }),
    ];
    renderPending();
    // Two separate single-write groups, so neither gets the shared-cascade line.
    expect(screen.queryByText(/share cascade/)).not.toBeInTheDocument();
  });

  it("explains a disabled confirm in visible copy, not a tooltip", () => {
    summary.reachable = false;
    pending.data = [row()];
    renderPending();

    expect(screen.getByText(/Zitadel is not answering/)).toBeInTheDocument();
    expect(
      screen.getByText(/Sending is paused/),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /^Send / })).toBeDisabled();
  });
});

/**
 * Caused by: a name, never a bare handle standing in for one.
 */
describe("Pending changes · caused by", () => {
  it("names the bundle, with the handle kept alongside for tracing", () => {
    bundles.data = [{ id: "bundle-1111-2222", name: "Woodshop access" }];
    pending.data = [row({ source: "bundle", source_ref: "bundle-1111-2222" })];
    renderPending();

    expect(screen.getByText("Woodshop access")).toBeInTheDocument();
    expect(screen.getByText(/b_bund/)).toBeInTheDocument();
  });

  it("names the rule by its mapping when the bundle/rule list has not loaded", () => {
    rules.data = [
      {
        id: "rule-1111-2222",
        source_project: "p1",
        source_role: "trained",
        target_project: "p2",
        target_role: "door",
        holder_count: 3,
        created_at: "2026-01-01T00:00:00Z",
      },
    ];
    pending.data = [row({ source: "rule", source_ref: "rule-1111-2222" })];
    renderPending();

    expect(screen.getByText("trained → door")).toBeInTheDocument();
  });

  it("never leaves the handle as the only label, even before the lookup resolves", () => {
    pending.data = [row({ source: "bundle", source_ref: "bundle-9999-0000" })];
    renderPending();

    expect(screen.getByText("Bundle")).toBeInTheDocument();
    expect(screen.getByText(/b_bund/)).toBeInTheDocument();
  });

  it("links a c_ handle to Change history", () => {
    pending.data = [row({ cascade_id: "cas-8841-aaaa" })];
    renderPending();

    const link = screen.getByRole("link", { name: /Change history/i });
    expect(link).toHaveAttribute("href", "/operations/cascades?cascade=cas-8841-aaaa");
  });
});
