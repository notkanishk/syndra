// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import AuditPage from "@/app/audit/page";

const log = vi.hoisted(() => ({
  entries: [] as Array<{ id: string; actor_id: string; target_id: string; action: string; resource_id: string; created_at: string }>,
  hasNextPage: false,
  fetchNextPage: vi.fn(),
}));

vi.mock("next/navigation", () => ({
  useSearchParams: () => new URLSearchParams(""),
  useRouter: () => ({ replace: () => {} }),
}));
vi.mock("@/lib/queries/useAudit", () => ({
  useAuditPages: () => ({
    data: { pages: [log.entries] },
    isLoading: false,
    error: null,
    refetch: () => {},
    hasNextPage: log.hasNextPage,
    isFetchingNextPage: false,
    fetchNextPage: log.fetchNextPage,
  }),
}));
vi.mock("@/lib/queries/useNameResolver", () => ({
  useNameResolver: () => ({ resolveUser: () => ({ value: null }) }),
}));
vi.mock("@/components/names", () => ({ UserName: () => <span>someone</span> }));
vi.mock("@/components/audit/EventLine", () => ({ EventSentence: () => <span>did something</span> }));
vi.mock("@/components/audit/TraceCell", () => ({ TraceCell: () => <span /> }));

const entry = (i: number) => ({
  id: `a${i}`,
  actor_id: "u1",
  target_id: "u2",
  action: "grant.create",
  resource_id: "r",
  created_at: new Date(Date.now() - i * 60_000).toISOString(),
});
const rowsOnScreen = () => screen.getAllByText("did something").length;

describe("Audit pages through one pagination row", () => {
  beforeEach(() => {
    log.fetchNextPage.mockClear();
    // The 7-day window is the default; entries a minute apart all sit inside it.
    log.entries = Array.from({ length: 30 }, (_, i) => entry(i));
    log.hasNextPage = true;
  });
  afterEach(cleanup);

  it("shows ten, and Show 25 more past what is loaded asks the server for the next page", () => {
    render(<AuditPage />);
    expect(rowsOnScreen()).toBe(10);
    expect(screen.getByText("Showing 10")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Show all/ })).toBeNull();

    fireEvent.click(screen.getByRole("button", { name: "Show 25 more" }));
    expect(rowsOnScreen()).toBe(30);
    expect(log.fetchNextPage).toHaveBeenCalledOnce();
  });

  it("does not ask the server while the loaded rows cover the page", () => {
    log.entries = Array.from({ length: 60 }, (_, i) => entry(i));
    render(<AuditPage />);
    fireEvent.click(screen.getByRole("button", { name: "Show 25 more" }));
    expect(rowsOnScreen()).toBe(35);
    expect(log.fetchNextPage).not.toHaveBeenCalled();
  });

  it("states the end of the log, with the date it reaches back to", () => {
    log.hasNextPage = false;
    log.entries = Array.from({ length: 12 }, (_, i) => entry(i));
    render(<AuditPage />);
    fireEvent.click(screen.getByRole("button", { name: "Show 25 more" }));
    expect(screen.getByText(/That's everything since/)).toBeInTheDocument();
    expect(screen.getByText("Showing all 12")).toBeInTheDocument();
  });
});
