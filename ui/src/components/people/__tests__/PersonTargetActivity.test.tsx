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

    // Coverage of THIS person, not the share's switch. TrueNAS scopes SMB
    // auditing by group, so a share can be audited and still record nothing
    // for them — and "auditing is off" would send an operator to a setting
    // that is already on.
    await waitFor(() =>
      expect(screen.getByText(/Auditing on scratch does not cover/)).toBeTruthy(),
    );
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

/**
 * The context that makes a repeated refusal actionable.
 *
 * The add-on returns an address and the target's own status token, and the row
 * used to drop both. On the live NAS that meant 553 rows over a week rendering
 * as 553 identical "Refused" pills — one verb, one outcome, and nothing to tell
 * one from another. The two dropped fields are the only things that did.
 */
describe("a refusal an operator can act on", () => {
  it("names where it came from and what the target said", async () => {
    state.activity = {
      target: "truenas",
      subject: "u1",
      readable: true,
      events: [
        {
          at: "2026-08-23T10:45:31Z",
          event: "AUTHENTICATION",
          success: false,
          address: "192.0.2.77",
          detail: "NT_STATUS_NO_SUCH_USER",
        },
      ],
    };
    renderTab();

    await waitFor(() => expect(screen.getByText("AUTHENTICATION")).toBeInTheDocument());
    // The address is the only thing distinguishing one refusal from the next.
    expect(screen.getByText("192.0.2.77")).toBeInTheDocument();
    // The target's own token, not a translation of it: this is the string an
    // operator searches for.
    expect(screen.getByText("NT_STATUS_NO_SUCH_USER")).toBeInTheDocument();
    expect(screen.getByText("Refused")).toBeInTheDocument();
  });

  it("tells two refusals apart when everything but the source differs", async () => {
    state.activity = {
      target: "truenas",
      subject: "u1",
      readable: true,
      events: [
        { at: "2026-08-23T10:45:31Z", event: "AUTHENTICATION", success: false, address: "192.0.2.77", detail: "NT_STATUS_NO_SUCH_USER" },
        { at: "2026-08-23T10:45:32Z", event: "AUTHENTICATION", success: false, address: "192.0.2.72", detail: "NT_STATUS_NO_SUCH_USER" },
      ],
    };
    renderTab();

    await waitFor(() => expect(screen.getByText("192.0.2.77")).toBeInTheDocument());
    expect(screen.getByText("192.0.2.72")).toBeInTheDocument();
  });

  it("renders an event that carries neither, because most targets will not", async () => {
    state.activity = {
      target: "truenas",
      subject: "u1",
      readable: true,
      events: [{ at: "2026-08-23T10:45:31Z", event: "CONNECT", share: "main", success: true }],
    };
    renderTab();

    await waitFor(() => expect(screen.getByText("CONNECT")).toBeInTheDocument());
    expect(screen.getByText(/main/)).toBeInTheDocument();
    expect(screen.queryByText("Refused")).toBeNull();
  });
});
