// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import ChangeHistoryPage from "@/app/operations/cascades/page";
import type { CascadeGroupRow } from "@/lib/queries/useConfirmationMode";

/**
 * `status=applied` is Syndra's own record of having sent a write. It is not a
 * Zitadel-side fact until `confirmed_at` says a post-write read of Zitadel
 * agreed — collapsing the two used to render "3 sent … all of them went
 * through" for cascades nobody had verified landed anywhere.
 */

vi.mock("next/navigation", () => ({
  useSearchParams: () => new URLSearchParams(),
  useRouter: () => ({ replace: () => {} }),
}));

vi.mock("@/components/names", () => ({
  UserName: ({ id }: { id: string }) => <span>{id}</span>,
  ProjectName: ({ id }: { id: string }) => <span>{id}</span>,
}));

let groups: CascadeGroupRow[] = [];

vi.mock("@/lib/queries/useConfirmationMode", () => ({
  useCascadeGroups: () => ({ data: groups, isLoading: false, error: null, refetch: () => {} }),
}));

function group(overrides: Partial<CascadeGroupRow>): CascadeGroupRow {
  return {
    cascade_id: "c1",
    source: "bundle",
    source_ref: "b1",
    applied: 0,
    waiting: 0,
    failed: 0,
    confirmed: 0,
    user_ids: ["u1"],
    writes: [
      {
        id: "w1",
        op_type: "add",
        user_id: "u1",
        project_id: "p1",
        role_keys: ["maker"],
        source: "bundle",
        status: "applied",
      },
    ],
    started_at: "2026-09-01T00:00:00Z",
    ...overrides,
  };
}

function renderPage() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <ChangeHistoryPage />
    </QueryClientProvider>,
  );
}

describe("Change history: sent vs. confirmed", () => {
  it("says confirmed by Zitadel when every applied write has been read back", () => {
    groups = [group({ applied: 3, confirmed: 3 })];
    renderPage();

    expect(screen.getByText("3 confirmed by Zitadel")).toBeInTheDocument();
    expect(screen.getByText(/all of them went through/)).toBeInTheDocument();
  });

  it("distinguishes sent from confirmed when read-back is still pending", () => {
    groups = [group({ applied: 3, confirmed: 1 })];
    renderPage();

    expect(screen.getByText("3 sent, 1 confirmed")).toBeInTheDocument();
    expect(
      screen.getByText(/Sent; Zitadel has not yet been read back for 2\./),
    ).toBeInTheDocument();
    expect(screen.queryByText(/all of them went through/)).not.toBeInTheDocument();
  });
});
