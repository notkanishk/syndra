// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import PendingChangesPage from "@/app/governance/pending/page";
import { NextStep } from "@/components/ui/NextStep";
import type { PendingRow } from "@/lib/queries/usePropagation";

/**
 * A-19 P3: "Waiting for you to send →" lands on you. The link carries the
 * person, Pending changes opens narrowed to them with the way out attached,
 * and the names on it are links back to the people.
 */

const pending = vi.hoisted(() => ({ data: [] as PendingRow[] }));
const nav = vi.hoisted(() => ({ search: "" }));

vi.mock("next/navigation", () => ({ useSearchParams: () => new URLSearchParams(nav.search) }));

vi.mock("@/lib/queries/useNameResolver", () => {
  const people: Record<string, string> = { u1: "Ada Lovelace", u2: "Priya Nair" };
  const miss = { value: undefined, resolved: true } as const;
  return {
    useNameResolver: () => ({
      resolveUser: (id: string) => (people[id] ? { value: { display_name: people[id], email: "" }, resolved: true } : miss),
      resolveProject: () => miss,
      resolveRole: () => miss,
      resolveBundle: () => miss,
    }),
  };
});
vi.mock("@/lib/queries/usePropagation", () => ({
  usePendingPropagations: () => ({ ...pending, isLoading: false, error: null, refetch: () => {} }),
  useDrainPropagations: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));
vi.mock("@/lib/queries/useBundles", () => ({ useBundles: () => ({ data: [], isLoading: false, error: null }) }));
vi.mock("@/lib/queries/useMappingRules", () => ({ useMappingRules: () => ({ data: [], isLoading: false, error: null }) }));
vi.mock("@/lib/queries/useGovernance", () => ({
  useGovernanceSummary: () => ({ data: { pending_propagation: { count: 0, zitadel_reachable: true } } }),
}));

const row = (id: string, user_id: string, extra: Partial<PendingRow> = {}) =>
  ({
    id,
    op_type: "add",
    user_id,
    project_id: "p1",
    role_keys: ["trained"],
    source: "direct",
    source_ref: "",
    cascade_id: `c-${id}`,
    status: "pending",
    attempts: 0,
    created_at: "2026-07-31T09:42:00Z",
    ...extra,
  }) as PendingRow;

function renderPending() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <PendingChangesPage />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  pending.data = [row("o1", "u1"), row("o2", "u2"), row("o3", "u2")];
  nav.search = "";
  window.history.replaceState(null, "", "/governance/pending");
});

describe("Waiting for you to send", () => {
  it("carries the person, and stays the whole queue with none", () => {
    const { rerender } = render(<NextStep person="u 1" />);
    expect(screen.getByRole("link")).toHaveAttribute("href", "/governance/pending?person=u%201");
    rerender(<NextStep />);
    expect(screen.getByRole("link")).toHaveAttribute("href", "/governance/pending");
    rerender(<NextStep href="/x" person="u1" />);
    expect(screen.getByRole("link")).toHaveAttribute("href", "/x");
  });
});

describe("Pending changes opened for a person", () => {
  it("shows the whole queue with no person asked for, and no chip", () => {
    renderPending();
    expect(screen.queryByRole("button", { name: "Show everyone's changes" })).not.toBeInTheDocument();
    expect(screen.getAllByRole("link", { name: "Ada Lovelace" })).toHaveLength(1);
    expect(screen.getAllByRole("link", { name: "Priya Nair" })).toHaveLength(2);
  });

  it("narrows to that person under an in-force chip, and says Send still sends all", () => {
    nav.search = "person=u2";
    renderPending();
    expect(screen.queryByText("Ada Lovelace", { selector: "a *, a" })).not.toBeInTheDocument();
    expect(screen.getAllByRole("link", { name: "Priya Nair" })).toHaveLength(2);
    expect(screen.getByRole("button", { name: "Show everyone's changes" })).toBeInTheDocument();
    expect(screen.getByText("2 of the 3 changes match. Sending sends all 3, in order.")).toBeInTheDocument();
  });

  it("clears with the chip's own ✕: everyone returns and the address is clean", () => {
    vi.useFakeTimers();
    try {
      nav.search = "person=u2";
      window.history.replaceState(null, "", "/governance/pending?person=u2");
      renderPending();
      fireEvent.click(screen.getByRole("button", { name: "Show everyone's changes" }));
      // The chip tints out first, then clears.
      vi.advanceTimersByTime(500);
    } finally {
      vi.useRealTimers();
    }
    expect(screen.getByRole("link", { name: "Ada Lovelace" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Show everyone's changes" })).not.toBeInTheDocument();
    expect(window.location.search).toBe("");
  });

  it("says so when the person has nothing waiting, and the way out is still there", () => {
    nav.search = "person=nobody";
    renderPending();
    expect(screen.getByText("Nothing is waiting for you to send for this person.")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Show everyone's changes" })).toBeInTheDocument();
  });

  it("links the names to the people", () => {
    renderPending();
    expect(screen.getAllByRole("link", { name: "Ada Lovelace" })[0]).toHaveAttribute("href", "/users/u1");
  });
});
