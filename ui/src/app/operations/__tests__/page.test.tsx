// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import EventActivityPage from "@/app/operations/page";
import type { OnboardingTriggerRow, WebhookEventRow } from "@/lib/queries/useOperations";
import { formatShortDate } from "@/lib/format";

/**
 * Rows used to show only "13:33:18" with no date anywhere near it, so a
 * timeline spanning several days read as one long out-of-order minute.
 * Grouping by day heading is what makes "newest first" legible to the eye
 * instead of only true in the data.
 */

const state = vi.hoisted(() => ({
  events: [] as WebhookEventRow[],
  triggers: [] as OnboardingTriggerRow[],
}));

vi.mock("@/lib/queries/useOperations", () => ({
  useWebhookEvents: () => ({ data: state.events, isLoading: false, error: null, refetch: () => {} }),
  useOnboardingTriggers: () => ({
    data: state.triggers,
    isLoading: false,
    error: null,
    refetch: () => {},
  }),
}));

vi.mock("@/components/names", () => ({
  UserName: () => <span>Someone</span>,
  ProjectName: () => <span>A project</span>,
  BundleName: () => <span>A bundle</span>,
}));

function webhook(overrides: Partial<WebhookEventRow> = {}): WebhookEventRow {
  return {
    id: "w1",
    event_type: "grant.created",
    user_id: "u1",
    source_project: "p1",
    idempotency_key: "k1",
    status: "processed",
    created_at: "2026-09-05T09:00:00Z",
    ...overrides,
  };
}

function renderPage() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <EventActivityPage />
    </QueryClientProvider>,
  );
}

describe("Incoming events — grouped by day", () => {
  it("gives each calendar day its own heading, newest first", () => {
    state.events = [
      webhook({ id: "w1", created_at: "2026-09-06T08:00:00Z" }),
      webhook({ id: "w2", created_at: "2026-09-05T09:00:00Z" }),
    ];
    renderPage();

    const newer = screen.getByText(formatShortDate("2026-09-06T08:00:00Z"));
    const older = screen.getByText(formatShortDate("2026-09-05T09:00:00Z"));
    // Newest day's heading precedes the older one in document order.
    expect(newer.compareDocumentPosition(older) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  });

  it("does not repeat a day's heading for a second row on the same day", () => {
    state.events = [
      webhook({ id: "w1", created_at: "2026-09-05T09:00:00Z" }),
      webhook({ id: "w2", created_at: "2026-09-05T15:00:00Z" }),
    ];
    renderPage();

    expect(screen.getAllByText(formatShortDate("2026-09-05T09:00:00Z"))).toHaveLength(1);
  });
});
