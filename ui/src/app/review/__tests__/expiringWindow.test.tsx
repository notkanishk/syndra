// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import ExpiringAccessPage from "@/app/review/expiring-access/page";

const nav = vi.hoisted(() => ({ search: "", replace: vi.fn(), askedFor: [] as number[] }));

vi.mock("next/navigation", () => ({
  useSearchParams: () => new URLSearchParams(nav.search),
  useRouter: () => ({ replace: nav.replace }),
}));

vi.mock("@/lib/queries/useExpiringAccess", () => ({
  useExpiringGrants: (days: number) => {
    nav.askedFor.push(days);
    return { data: [], isLoading: false, error: null, refetch: vi.fn() };
  },
  useAcknowledgeExpiry: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useClearExpiryAcknowledgement: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));

function renderPage() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <ExpiringAccessPage />
    </QueryClientProvider>,
  );
}

const pills = () => within(screen.getByRole("radiogroup", { name: "How far ahead" }));

beforeEach(() => {
  nav.search = "";
  nav.replace.mockClear();
  nav.askedFor.length = 0;
});

/**
 * One window across the product (A-04 §4, A-06 §7): 14 | 30 days, 14 first.
 * The number on Home's tile and the rail is the number of rows you land on.
 */
describe("the expiring window", () => {
  it("opens on 14 days with no link to say so, which is what the rail opens", () => {
    renderPage();
    expect(pills().getAllByRole("radio").map((r) => r.textContent)).toEqual(["14 days", "30 days"]);
    expect(pills().getByRole("radio", { name: "14 days" })).toBeChecked();
    expect(nav.askedFor.at(-1)).toBe(14);
    expect(screen.getByText(/end in the next 14 days/)).toBeInTheDocument();
  });

  it("opens on 30 from ?within=30, and on 14 from Home's ?within=14", () => {
    nav.search = "within=30";
    const { unmount } = renderPage();
    expect(pills().getByRole("radio", { name: "30 days" })).toBeChecked();
    expect(nav.askedFor.at(-1)).toBe(30);
    unmount();

    nav.search = "within=14";
    renderPage();
    expect(pills().getByRole("radio", { name: "14 days" })).toBeChecked();
  });

  it("a link naming 7 or 90 (or nonsense) lands on 14", () => {
    for (const within of ["7", "90", "nonsense"]) {
      nav.search = `within=${within}`;
      const { unmount } = renderPage();
      expect(pills().getByRole("radio", { name: "14 days" })).toBeChecked();
      expect(nav.askedFor.at(-1)).toBe(14);
      unmount();
    }
  });

  it("keeps the choice in the link: 30 is named, 14 is the clean link", () => {
    const { unmount } = renderPage();
    fireEvent.click(pills().getByRole("radio", { name: "30 days" }));
    expect(nav.replace).toHaveBeenLastCalledWith("/review/expiring-access?within=30", { scroll: false });
    unmount();

    nav.search = "within=30";
    renderPage();
    fireEvent.click(pills().getByRole("radio", { name: "14 days" }));
    expect(nav.replace).toHaveBeenLastCalledWith("/review/expiring-access", { scroll: false });
  });
});
