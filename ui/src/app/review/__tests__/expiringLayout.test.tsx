// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import ExpiringAccessPage from "@/app/review/expiring-access/page";
import type { ExpiringGrantRow } from "@/lib/queries/useExpiringAccess";

const state = vi.hoisted(() => ({ rows: [] as ExpiringGrantRow[], extend: vi.fn() }));

vi.mock("next/navigation", () => ({
  useSearchParams: () => new URLSearchParams(""),
  useRouter: () => ({ replace: vi.fn() }),
}));

vi.mock("@/lib/queries/useExpiringAccess", () => ({
  useExpiringGrants: () => ({ data: state.rows, isLoading: false, error: null, refetch: vi.fn() }),
  useAcknowledgeExpiry: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useClearExpiryAcknowledgement: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));

vi.mock("@/lib/queries/useUsers", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/queries/useUsers")>()),
  useCreateGrant: () => ({ mutateAsync: state.extend, isPending: false }),
}));

const days = (n: number) => new Date(Date.now() + n * 86_400_000).toISOString();

function grant(over: Partial<ExpiringGrantRow> = {}): ExpiringGrantRow {
  return {
    id: "g1",
    user_id: "u1",
    project_id: "pLaser",
    role_key: "trained",
    granted_by: "priya",
    reason: "",
    expires_at: days(5),
    created_at: "2026-06-01T12:00:00Z",
    ...over,
  };
}

function renderPage() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <ExpiringAccessPage />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  state.rows = [grant()];
  state.extend = vi.fn().mockResolvedValue({ outbox_id: "ob1", status: "applied" });
});

/**
 * route-expiring.dc.html, reconciled with A-04 §4 (the window stays 14 | 30, so the board's
 * single 30 days does not apply) and verdict 96b (Close, never Done).
 */
describe("Expiring access, laid out as its board", () => {
  it("says what the page does once, in the lede, and carries no explainer cards", () => {
    renderPage();
    expect(document.body.textContent).toMatch(/Extend gives 90 more days/);
    expect(document.body.textContent).toMatch(/Let it lapse changes nothing/);
    expect(screen.queryByText(/When a row comes back/)).toBeNull();
    expect(screen.queryByText(/does, and what it doesn/)).toBeNull();
  });

  it("an empty window is one card with the healthy line and no column headings", () => {
    state.rows = [];
    renderPage();
    expect(screen.getByText("Nothing ends in the next 14 days.")).toBeTruthy();
    expect(screen.queryByText("Who")).toBeNull();
    expect(screen.queryByText("Ends in")).toBeNull();
  });

  it("a row says who gave it and when, how long is left, and carries no State column or Direct badge", () => {
    renderPage();
    expect(screen.getByText("Ends in")).toBeTruthy();
    expect(screen.getByText("Given by")).toBeTruthy();
    expect(screen.getAllByText("trained").length).toBeGreaterThan(0);
    expect(document.body.textContent).toMatch(/5 days left/);
    expect(screen.queryByText("Direct")).toBeNull();
    expect(screen.queryByText("State")).toBeNull();
    expect(document.body.textContent).not.toMatch(/Undecided — ends/);
  });

  it("a decided row is not dimmed and its decision sits under the role", () => {
    state.rows = [grant({ acknowledged: { by: "priya", at: "2026-08-04T09:00:00Z", note: "Cohort ends" } })];
    const { container } = renderPage();
    expect(container.querySelector(".opacity-\\[\\.72\\]")).toBeNull();
    expect(document.body.textContent).toMatch(/let it lapse on .*Cohort ends/i);
  });

  it("Select sits in the filter bar beside the window, not in the page header", () => {
    renderPage();
    const select = screen.getByRole("button", { name: "Select" });
    expect(select.parentElement).toContainElement(screen.getByRole("radiogroup", { name: "How far ahead" }));
  });

  it("shows ten undecided rows at a time", () => {
    state.rows = Array.from({ length: 12 }, (_, i) => grant({ id: `g${i}`, user_id: `u${i}` }));
    renderPage();
    expect(screen.getAllByRole("button", { name: "Extend 90 days" })).toHaveLength(10);
    fireEvent.click(screen.getByRole("button", { name: "Show 2 more" }));
    expect(screen.getAllByRole("button", { name: "Extend 90 days" })).toHaveLength(12);
  });

  it("the Undecided section goes when everything is decided, and Let lapse stays", () => {
    state.rows = [grant({ acknowledged: { by: "priya", at: "2026-08-04T09:00:00Z" } })];
    renderPage();
    expect(screen.queryByText("Undecided")).toBeNull();
    expect(screen.getByText("Let lapse")).toBeTruthy();
  });

  it("Extend reports in the seat its buttons held", async () => {
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: "Extend 90 days" }));
    await waitFor(() => expect(screen.getByRole("status")).toHaveTextContent(/Extended by 90 days/));
    expect(screen.queryByRole("button", { name: "Extend 90 days" })).toBeNull();
    expect(screen.queryByRole("button", { name: "Let it lapse" })).toBeNull();
  });

  it("a failed Extend keeps its buttons so it can be tried again", async () => {
    state.extend = vi.fn().mockRejectedValue(new Error("nope"));
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: "Extend 90 days" }));
    await waitFor(() => expect(screen.getByRole("alert")).toBeTruthy());
    expect(screen.getByRole("button", { name: "Extend 90 days" })).toBeEnabled();
  });
});
