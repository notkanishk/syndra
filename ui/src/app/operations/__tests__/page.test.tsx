// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

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
  eventsError: null as unknown,
  triggersError: null as unknown,
  names: { u1: "Meera Pillai", u2: "Kabir Shah", p1: "Laser cutter", b1: "New members" } as Record<
    string,
    string
  >,
}));

// A failed read keeps whatever the last good read left, as react-query does.
vi.mock("@/lib/queries/useOperations", () => ({
  LOG_LIMIT: 200,
  useWebhookEvents: () => ({
    data: state.eventsError && !state.events.length ? undefined : state.events,
    isLoading: false,
    error: state.eventsError,
    dataUpdatedAt: Date.now(),
    refetch: () => {},
  }),
  useOnboardingTriggers: () => ({
    data: state.triggersError && !state.triggers.length ? undefined : state.triggers,
    isLoading: false,
    error: state.triggersError,
    dataUpdatedAt: Date.now(),
    refetch: () => {},
  }),
}));

vi.mock("@/components/names", () => ({
  UserName: ({ id }: { id: string }) => <span>{state.names[id] ?? "Someone"}</span>,
  ProjectName: ({ id }: { id: string }) => <span>{state.names[id] ?? "A project"}</span>,
  BundleName: ({ id }: { id: string }) => <span>{state.names[id] ?? "A bundle"}</span>,
}));

// Search reads the same names the rows show, so it needs the resolver to agree.
vi.mock("@/lib/queries/useNameResolver", () => ({
  useNameResolver: () => ({
    resolveUser: (id: string) => ({
      value: state.names[id] ? { display_name: state.names[id] } : undefined,
      resolved: true,
    }),
    resolveProject: (id: string) => ({
      value: state.names[id] ? { name: state.names[id] } : undefined,
      resolved: true,
    }),
    resolveRole: () => ({ value: undefined, resolved: true }),
    resolveBundle: (id: string) => ({
      value: state.names[id] ? { name: state.names[id] } : undefined,
      resolved: true,
    }),
  }),
}));

function trigger(overrides: Partial<OnboardingTriggerRow> = {}): OnboardingTriggerRow {
  return {
    id: "t1",
    user_id: "u2",
    source: "webhook",
    idempotency_key: "tk1",
    status: "completed",
    bundle_id: "b1",
    created_at: "2026-09-05T10:00:00Z",
    ...overrides,
  };
}

function webhook(overrides: Partial<WebhookEventRow> = {}): WebhookEventRow {
  return {
    id: "w1",
    event_type: "grant_added",
    user_id: "u1",
    source_project: "p1",
    idempotency_key: "k1",
    status: "processed",
    created_at: "2026-09-05T09:00:00Z",
    ...overrides,
  };
}

beforeEach(() => {
  state.events = [];
  state.triggers = [];
  state.eventsError = null;
  state.triggersError = null;
});

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

describe("Incoming events — plain titles and outcomes", () => {
  it("titles each row in words, never by its event key", () => {
    state.events = [
      webhook({ id: "w1", event_type: "grant_added", created_at: "2026-09-05T09:00:00Z" }),
      webhook({ id: "w2", event_type: "user_deactivated", created_at: "2026-09-05T08:00:00Z" }),
      webhook({ id: "w3", event_type: "something_new", created_at: "2026-09-05T07:00:00Z" }),
    ];
    state.triggers = [trigger({ created_at: "2026-09-05T06:00:00Z" })];
    renderPage();

    expect(screen.getByText("Access given in Zitadel")).toBeTruthy();
    expect(screen.getByText("Person deactivated in Zitadel")).toBeTruthy();
    expect(screen.getByText("Person joined")).toBeTruthy();
    // An event this page has no words for still gets a plain title, not its key.
    expect(screen.getByText("Another event from Zitadel")).toBeTruthy();
    expect(screen.queryByText("something_new")).toBeNull();
    expect(screen.queryByText("zitadel.grant_added", { selector: "span:not(.type-mono)" })).toBeNull();
  });

  it("says who and what in the sentence, naming the bundle a new person was given", () => {
    state.events = [webhook({ id: "w1", event_type: "grant_added", role_key: "operator" })];
    state.triggers = [trigger()];
    renderPage();

    expect(screen.getByText(/was given/).textContent).toContain("Meera Pillai");
    expect(screen.getByText(/joined\. Syndra gave them/).textContent).toContain("New members");
  });

  it.each([
    ["processed", "Done"],
    ["pending", "Waiting"],
    ["failed", "Failed"],
    ["dropped_enrichment_incomplete", "Not acted on"],
  ])("shows a %s event as its own outcome word, %s", (status, word) => {
    state.events = [webhook({ status, error_message: status === "failed" ? "role is not in the catalog" : undefined })];
    renderPage();

    const row = screen.getByText("Access given in Zitadel").closest("[data-event-row]") as HTMLElement;
    expect(within(row).getAllByText(word).length).toBeGreaterThan(0);
  });

  it("gives a failed row its reason, and a welcome with no default bundle its own outcome", () => {
    state.events = [webhook({ id: "w1", status: "failed", error_message: "role wood.lead is not in the catalog" })];
    state.triggers = [trigger({ status: "unconfigured", bundle_id: undefined })];
    renderPage();

    expect(screen.getByText(/role wood\.lead is not in the catalog/)).toBeTruthy();
    const joined = screen.getByText("Person joined").closest("[data-event-row]") as HTMLElement;
    expect(within(joined).getAllByText("Not acted on").length).toBeGreaterThan(0);
    expect(within(joined).queryByText("Failed")).toBeNull();
    expect(joined.textContent).toContain("no default bundle for new members is set");
  });
});

describe("Incoming events — search", () => {
  it("narrows to the person, project or bundle typed, by the names on screen", () => {
    state.events = [
      webhook({ id: "w1", user_id: "u1", event_type: "grant_added" }),
      webhook({ id: "w2", user_id: "u2", event_type: "grant_removed" }),
    ];
    state.triggers = [];
    renderPage();
    const box = screen.getByRole("searchbox", { name: /search events/i });

    fireEvent.change(box, { target: { value: "kabir" } });
    expect(screen.queryByText("Access given in Zitadel")).toBeNull();
    expect(screen.getByText("Access removed in Zitadel")).toBeTruthy();

    // A project's name finds its events; a bundle's name finds the people who were given it.
    fireEvent.change(box, { target: { value: "laser" } });
    expect(screen.getAllByText(/Access (given|removed) in Zitadel/)).toHaveLength(2);
  });

  it("says nothing matched, how many events there are, and offers the way back", () => {
    state.events = [webhook({ id: "w1" })];
    state.triggers = [];
    renderPage();

    fireEvent.change(screen.getByRole("searchbox", { name: /search events/i }), {
      target: { value: "zzz" },
    });
    expect(screen.getByText("No events match those filters.")).toBeTruthy();
    expect(screen.getByText("1 event is in the timeline.")).toBeTruthy();

    fireEvent.click(screen.getByRole("button", { name: "Clear filters" }));
    expect(screen.getByText("Access given in Zitadel")).toBeTruthy();
  });
});

describe("Incoming events — states", () => {
  it("says nothing has happened yet when there is nothing, with no search to offer", () => {
    state.events = [];
    state.triggers = [];
    renderPage();

    expect(screen.getByText("Nothing has happened yet.")).toBeTruthy();
    expect(screen.queryByText("No events match those filters.")).toBeNull();
  });

  it("draws no table headings over an empty card, and does over rows", () => {
    state.events = [];
    state.triggers = [];
    const { unmount } = renderPage();
    expect(screen.queryAllByRole("columnheader")).toHaveLength(0);
    expect(screen.queryByRole("button", { name: "Sort by event" })).toBeNull();
    unmount();

    state.events = [webhook({ id: "w1" })];
    renderPage();
    expect(screen.getByRole("button", { name: "Sort by event" })).toBeTruthy();
  });

  it("names the stream it could not read and keeps the rows of the one it could", () => {
    state.events = [webhook({ id: "w1" })];
    state.triggers = [];
    state.triggersError = new Error("down");
    renderPage();

    expect(screen.getByText("People joining could not be read.")).toBeTruthy();
    expect(screen.getByText("Access given in Zitadel")).toBeTruthy();
  });
});

describe("Incoming events — ten rows, then more; sort on the headings", () => {
  it("shows ten rows and offers the rest", () => {
    state.events = Array.from({ length: 14 }, (_, i) =>
      webhook({ id: `w${i}`, created_at: `2026-09-05T${String(i + 1).padStart(2, "0")}:00:00Z` }),
    );
    state.triggers = [];
    renderPage();

    expect(screen.getAllByText("Access given in Zitadel")).toHaveLength(10);
    fireEvent.click(screen.getByRole("button", { name: "Show 4 more" }));
    expect(screen.getAllByText("Access given in Zitadel")).toHaveLength(14);
  });

  it("gathers equal events under a labelled run with their count when sorted by event", () => {
    state.events = [
      webhook({ id: "w1", event_type: "grant_removed", created_at: "2026-09-05T09:00:00Z" }),
      webhook({ id: "w2", event_type: "grant_added", created_at: "2026-09-05T10:00:00Z" }),
      webhook({ id: "w3", event_type: "grant_added", created_at: "2026-09-04T10:00:00Z" }),
    ];
    state.triggers = [];
    renderPage();

    fireEvent.click(screen.getByRole("button", { name: "Sort by event" }));
    const runs = screen.getAllByTestId("run-label").map((el) => el.textContent);
    expect(runs).toEqual(["Access given in Zitadel2", "Access removed in Zitadel1"]);
  });
});
