// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { PersonActivity } from "@/components/people/PersonActivity";
import type { TargetActivity } from "@/lib/queries/useTargetActivity";

const state = vi.hoisted(() => ({
  activity: null as TargetActivity | null,
}));

vi.mock("@/lib/queries/useAudit", () => ({
  useAuditEntries: () => ({ data: [], isLoading: false, error: null, refetch: () => {} }),
}));

vi.mock("@/lib/queries/useTargets", () => ({
  useTargets: () => ({ data: [{ target: "truenas" }], isLoading: false, error: null }),
}));

vi.mock("@/lib/queries/useTargetActivity", () => ({
  useTargetActivity: () => ({ data: state.activity, isLoading: false, isError: false }),
}));

vi.mock("@/components/names", () => ({ UserName: () => null }));
vi.mock("@/components/audit/TraceCell", () => ({ TraceCell: () => null }));

function renderTab() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <PersonActivity userId="u1" name="Ada" />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  state.activity = null;
});

/**
 * Syndra's ledger and the target's audit log are two claims, and the surface
 * has to keep them apart. Syndra's feed says what Syndra did; the target's says
 * what the account did, including everything that happened with no involvement
 * from Syndra — which is the category a merged feed would hide.
 */
describe("the target's own log, beside Syndra's", () => {
  it("says which source it is reading", async () => {
    state.activity = { target: "truenas", subject: "u1", readable: true, events: [] };
    renderTab();

    await waitFor(() =>
      expect(screen.getByText(/audit log, not from Syndra/i)).toBeTruthy(),
    );
  });

  // The distinction the card exists for. An unreadable log and a quiet one are
  // the same empty list otherwise, and they are opposite answers.
  it("refuses to render an unreadable log as no activity", async () => {
    state.activity = {
      target: "truenas",
      subject: "u1",
      readable: false,
      detail: "target unreachable",
    };
    renderTab();

    await waitFor(() =>
      expect(screen.getByText(/not a claim that nothing\s+happened/i)).toBeTruthy(),
    );
    expect(screen.queryByText(/No recorded activity/i)).toBeNull();
  });

  // A short list on a target whose shares are half unaudited is not a quiet
  // week, and the operator cannot tell without being told.
  it("names the shares nothing was watching", async () => {
    state.activity = {
      target: "truenas",
      subject: "u1",
      readable: true,
      events: [],
      unaudited_shares: ["scratch"],
    };
    renderTab();

    await waitFor(() => expect(screen.getByText(/Auditing is off on scratch/)).toBeTruthy());
  });

  it("marks a refused access apart from one that succeeded", async () => {
    state.activity = {
      target: "truenas",
      subject: "u1",
      readable: true,
      events: [
        { at: "2026-08-20T10:00:00Z", event: "CONNECT", share: "lab", success: true },
        { at: "2026-08-20T11:00:00Z", event: "CONNECT", share: "vault", success: false },
      ],
    };
    renderTab();

    await waitFor(() => expect(screen.getAllByText("CONNECT")).toHaveLength(2));
    expect(screen.getAllByText("Refused")).toHaveLength(1);
  });
});
