// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import ExpiringAccessPage from "@/app/review/expiring-access/page";
import type { ExpiringGrantRow } from "@/lib/queries/useExpiringAccess";

/**
 * route-expiring.dc.html's standing rules for a list with sections: search and
 * Filters in the bar, headings that sort (and gather), two cards that fold,
 * chips on the read line and the floating pill.
 */
const state = vi.hoisted(() => ({
  rows: [] as ExpiringGrantRow[],
  search: "",
  // When set, a window reads its own rows: the 30-day list is not the 14-day list plus nothing.
  byWindow: null as Record<number, ExpiringGrantRow[]> | null,
}));

const NAMES: Record<string, string> = { u1: "Meera Pillai", u2: "Arjun Das", u3: "Tomas Weber", priya: "Priya Nair" };
const PROJECTS: Record<string, string> = { p1: "Electronics bench", p2: "Wood shop" };

vi.mock("next/navigation", () => ({
  useSearchParams: () => new URLSearchParams(state.search),
  useRouter: () => ({ replace: vi.fn() }),
}));

vi.mock("@/lib/queries/useNameResolver", () => ({
  useNameResolver: () => ({
    resolveUser: (id: string) => ({ value: NAMES[id] ? { display_name: NAMES[id] } : undefined, resolved: true }),
    resolveProject: (id: string) => ({ value: PROJECTS[id] ? { name: PROJECTS[id] } : undefined, resolved: true }),
  }),
}));

vi.mock("@/lib/queries/useExpiringAccess", () => ({
  useExpiringGrants: (days: number) => ({
    data: state.byWindow?.[days] ?? state.rows,
    isLoading: false,
    error: null,
    refetch: vi.fn(),
    dataUpdatedAt: Date.now(),
  }),
  useAcknowledgeExpiry: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useClearExpiryAcknowledgement: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));

vi.mock("@/lib/queries/useUsers", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/queries/useUsers")>()),
  useCreateGrant: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));

const days = (n: number) => new Date(Date.now() + n * 86_400_000).toISOString();

function grant(over: Partial<ExpiringGrantRow> = {}): ExpiringGrantRow {
  return {
    id: "g1",
    user_id: "u1",
    project_id: "p1",
    role_key: "operator",
    granted_by: "priya",
    reason: "",
    expires_at: days(5),
    created_at: "2026-06-01T12:00:00Z",
    ...over,
  };
}

const ACK = { by: "priya", at: "2026-08-04T09:00:00Z" };

/** Three undecided (soonest: Meera) and one let lapse. */
function mixed() {
  return [
    grant({ id: "a", user_id: "u1", project_id: "p1", role_key: "operator", expires_at: days(2) }),
    grant({ id: "b", user_id: "u2", project_id: "p2", role_key: "member", expires_at: days(6) }),
    grant({ id: "c", user_id: "u3", project_id: "p2", role_key: "member", expires_at: days(9) }),
    grant({ id: "d", user_id: "u2", project_id: "p1", role_key: "operator", expires_at: days(4), acknowledged: ACK }),
  ];
}

function renderPage() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <ExpiringAccessPage />
    </QueryClientProvider>,
  );
}

const UNDECIDED = "Undecided";
const LAPSE = "Let lapse";
const peopleIn = (card: HTMLElement) =>
  Array.from(card.querySelectorAll("a[href^='/users/']")).map((a) => a.textContent);

beforeEach(() => {
  state.rows = mixed();
  state.search = "";
  state.byWindow = null;
});

describe("Expiring access — the read line", () => {
  it("says it is Syndra's own list, and names each card in a chip with its count", () => {
    renderPage();
    expect(screen.getByText(/Syndra’s own list was read just now/)).toBeInTheDocument();
    const chips = screen.getByRole("navigation", { name: "Sections of expiring access" });
    expect(within(chips).getByRole("link", { name: /Undecided\s*3/ })).toBeInTheDocument();
    expect(within(chips).getByRole("link", { name: /Let lapse\s*1/ })).toBeInTheDocument();
  });

  it("has a chip that opens a folded card", () => {
    renderPage();
    const lapse = screen.getByRole("region", { name: LAPSE });
    fireEvent.click(within(lapse).getByRole("button", { name: LAPSE }));
    expect(within(lapse).queryByText("Arjun Das")).toBeNull();

    fireEvent.click(
      within(screen.getByRole("navigation", { name: "Sections of expiring access" })).getByRole("link", {
        name: /Let lapse/,
      }),
    );
    expect(within(lapse).getByText("Arjun Das")).toBeInTheDocument();
  });

  it("has a floating pill for the same two cards", () => {
    renderPage();
    const pill = document.querySelector('nav[aria-label="Sections of expiring access (while scrolling)"]')!;
    expect(pill).toHaveAttribute("aria-hidden", "true");
    expect(within(pill as HTMLElement).getAllByRole("link", { hidden: true })).toHaveLength(2);
  });

  it("draws no chips when only one card has rows", () => {
    state.rows = [grant()];
    renderPage();
    expect(screen.queryByRole("navigation", { name: "Sections of expiring access" })).toBeNull();
    expect(screen.getByText(/Syndra’s own list/)).toBeInTheDocument();
  });
});

describe("Expiring access — folding", () => {
  it("folds a card to its heading, leaving the other alone", () => {
    renderPage();
    const undecided = screen.getByRole("region", { name: UNDECIDED });
    const heading = within(undecided).getByRole("button", { name: UNDECIDED });
    expect(heading).toHaveAttribute("aria-expanded", "true");

    fireEvent.click(heading);
    expect(heading).toHaveAttribute("aria-expanded", "false");
    expect(within(undecided).queryByText("Meera Pillai")).toBeNull();
    expect(within(screen.getByRole("region", { name: LAPSE })).getByText("Arjun Das")).toBeInTheDocument();
  });
});

describe("Expiring access — search", () => {
  const box = () => screen.getByRole("textbox", { name: "Search expiring access by person or role" });

  it("finds a person by name across both cards, and the lede counts what matched", () => {
    renderPage();
    fireEvent.change(box(), { target: { value: "arjun" } });

    expect(within(screen.getByRole("region", { name: UNDECIDED })).getByText("Arjun Das")).toBeInTheDocument();
    expect(within(screen.getByRole("region", { name: LAPSE })).getByText("Arjun Das")).toBeInTheDocument();
    expect(screen.queryByText("Meera Pillai")).toBeNull();
    expect(screen.getByText(/2 roles matching · 1 person/)).toBeInTheDocument();
  });

  it("finds a role by its project and by its key", () => {
    renderPage();
    fireEvent.change(box(), { target: { value: "wood" } });
    expect(screen.queryByText("Meera Pillai")).toBeNull();
    expect(screen.getByText("Tomas Weber")).toBeInTheDocument();

    fireEvent.change(box(), { target: { value: "operator" } });
    expect(screen.getByText("Meera Pillai")).toBeInTheDocument();
    expect(screen.queryByText("Tomas Weber")).toBeNull();
  });

  it("never says every one is decided about a search that matches nothing", () => {
    renderPage();
    fireEvent.change(box(), { target: { value: "zzz" } });
    expect(screen.queryByText(/every one decided/)).toBeNull();
    expect(screen.getByText("Nothing that ends matches those filters.")).toBeInTheDocument();
  });

  it("says it of the whole window, and only when the whole window is decided", () => {
    state.rows = mixed().map((row) => ({ ...row, acknowledged: ACK }));
    renderPage();
    expect(screen.getByText(/every one decided/)).toBeInTheDocument();
  });

  it("does not say it of a search that happens to match only decided rows", () => {
    // Arjun's Electronics bench row is let lapse; his Wood shop row is not, and neither are others.
    state.rows = mixed().filter((row) => row.id !== "b");
    renderPage();
    fireEvent.change(box(), { target: { value: "arjun" } });
    expect(screen.queryByText(/every one decided/)).toBeNull();
  });

  it("says nothing matches, and clearing is the way back", () => {
    renderPage();
    fireEvent.change(box(), { target: { value: "zzz" } });
    expect(screen.getByText("Nothing that ends matches those filters.")).toBeInTheDocument();
    expect(screen.queryByRole("region", { name: UNDECIDED })).toBeNull();

    fireEvent.click(screen.getByRole("button", { name: "Clear filters" }));
    expect(screen.getByText("Meera Pillai")).toBeInTheDocument();
  });

  it("does not call the queue decided because a search left no undecided row", () => {
    renderPage();
    fireEvent.change(box(), { target: { value: "operator" } });
    // Only Arjun's decided Electronics row and Meera's undecided one match; narrow to the decided one.
    fireEvent.change(box(), { target: { value: "arjun das" } });
    expect(screen.queryByText(/Nothing here is waiting on a decision/)).toBeNull();
  });

  it("moves the amber to the soonest row that is still shown", () => {
    renderPage();
    const amber = () => document.querySelectorAll(".bg-warn-soft").length;
    expect(amber()).toBe(1);
    fireEvent.change(box(), { target: { value: "tomas" } });
    expect(amber()).toBe(1);
  });
});

// A-07 FINAL §2 item 8: the pills are the filter; there is no Filters panel and no project select.
describe("Expiring access — no Filters panel", () => {
  it("narrows by how far ahead and by search, and by nothing else", () => {
    renderPage();
    expect(screen.queryByRole("button", { name: /Filters/ })).toBeNull();
    expect(screen.queryByRole("combobox", { name: "Filter by project" })).toBeNull();
    expect(screen.getByRole("radio", { name: "14 days" })).toBeInTheDocument();
    expect(screen.getByRole("radio", { name: "30 days" })).toBeInTheDocument();
  });
});

// A-07 §16: moving between sections matters most while selecting across them.
describe("Expiring access — chips in Select mode", () => {
  it("keeps the section chips visible while selecting", () => {
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: /^Select/ }));
    expect(screen.getByRole("navigation", { name: "Sections of expiring access" })).toBeInTheDocument();
  });
});

describe("Expiring access — sort", () => {
  it("starts with the soonest first, the arrow on Ends in", () => {
    renderPage();
    expect(peopleIn(screen.getByRole("region", { name: UNDECIDED }))).toEqual([
      "Meera Pillai",
      "Arjun Das",
      "Tomas Weber",
    ]);
    const ends = within(screen.getByRole("region", { name: UNDECIDED })).getByRole("button", { name: "Sort by ends in" });
    expect(ends.querySelector("span")).toHaveClass("opacity-100");
  });

  it("reverses Ends in on a second press", () => {
    renderPage();
    const card = screen.getByRole("region", { name: UNDECIDED });
    fireEvent.click(within(card).getByRole("button", { name: "Sort by ends in" }));
    expect(peopleIn(card)).toEqual(["Tomas Weber", "Arjun Das", "Meera Pillai"]);
  });

  it("orders by who, with no runs", () => {
    renderPage();
    const card = screen.getByRole("region", { name: UNDECIDED });
    fireEvent.click(within(card).getByRole("button", { name: "Sort by who" }));
    expect(peopleIn(card)).toEqual(["Arjun Das", "Meera Pillai", "Tomas Weber"]);
    expect(within(card).queryAllByTestId("run-label")).toHaveLength(0);
  });

  it("gathers equal roles under a counted label, soonest first within each", () => {
    renderPage();
    const card = screen.getByRole("region", { name: UNDECIDED });
    fireEvent.click(within(card).getByRole("button", { name: "Sort by what they can get into" }));

    expect(within(card).getAllByTestId("run-label").map((el) => el.textContent)).toEqual([
      "Electronics bench · Operator1",
      "Wood shop · Member2",
    ]);
    expect(peopleIn(card)).toEqual(["Meera Pillai", "Arjun Das", "Tomas Weber"]);
  });

  it("keeps the soonest row amber whatever the order", () => {
    renderPage();
    const card = screen.getByRole("region", { name: UNDECIDED });
    fireEvent.click(within(card).getByRole("button", { name: "Sort by who" }));
    const amber = card.querySelector(".bg-warn-soft") as HTMLElement;
    expect(amber).toHaveTextContent("Meera Pillai");
  });
});
