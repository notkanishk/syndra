// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { WithdrawnAccess } from "@/components/review/WithdrawnAccess";

/**
 * route-revocations.dc.html: a meta line, search and Filters, a read line with a chip per
 * section, two folding tables (Given up first, always), and a retry column.
 *
 * Which section is empty IS the answer: they used to render only when non-empty, so a revocation
 * going terminal inserted a red card above the queue somebody was reading. Structure moving in
 * response to data, on the page whose whole job is to say what has not happened yet.
 */

const NAMES: Record<string, string> = { u1: "Sai Khurana", u2: "Tomas Weber", u3: "Omar Haddad" };
const PROJECTS: Record<string, string> = { pWood: "Wood shop" };

vi.mock("@/lib/queries/useNameResolver", () => ({
  useNameResolver: () => ({
    resolveUser: (id: string) => ({ value: NAMES[id] ? { display_name: NAMES[id] } : undefined, resolved: true }),
    resolveProject: (id: string) => ({ value: PROJECTS[id] ? { name: PROJECTS[id] } : undefined, resolved: true }),
  }),
}));

function row(id: string, spent: boolean, over: Record<string, unknown> = {}) {
  return {
    id,
    op_type: "revoke",
    user_id: "u1",
    project_id: "pWood",
    role_keys: ["wood_member"],
    status: spent ? "failed" : "pending",
    attempts: spent ? 9 : 3,
    created_at: "2026-08-20T00:00:00Z",
    target: "zitadel",
    age_seconds: 4000,
    spent,
    ...over,
  };
}

let payload: unknown = { revocations: [] };

vi.mock("@/lib/api-client", async () => {
  const actual = await vi.importActual<typeof import("@/lib/api-client")>("@/lib/api-client");
  return { ...actual, request: vi.fn(async () => payload) };
});

async function renderPage() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <WithdrawnAccess />
    </QueryClientProvider>,
  );
  await screen.findByText(/\d (person|people)$/);
}

const sectionTitles = () =>
  Array.from(document.querySelectorAll("section")).map((el) => el.querySelector('[id$="-title"]')!.textContent);

beforeEach(() => {
  payload = { revocations: [row("r1", false)] };
});

describe("what has not gone away yet", () => {
  it("keeps both sections, in one order, whatever either holds", async () => {
    await renderPage();
    expect(sectionTitles(), "terminal first, always").toEqual(["Given up", "Still on the way"]);
    // The empty one is present, and says what its emptiness means: the good news on this page.
    expect(screen.getByText(/Nothing has been given up on/)).toBeInTheDocument();
  });

  it("does not insert a card above the queue when one goes terminal", async () => {
    payload = { revocations: [row("r1", false), row("r2", true, { user_id: "u2" })] };
    await renderPage();
    expect(sectionTitles()).toEqual(["Given up", "Still on the way"]);
    expect(screen.queryByText(/Nothing has been given up on/)).toBeNull();
  });
});

describe("the board's anatomy", () => {
  beforeEach(() => {
    payload = {
      revocations: [
        row("r1", true, { user_id: "u1", last_error: "Zitadel says this person no longer exists there.", cascade_id: "c-abc123" }),
        row("r2", false, { user_id: "u3", attempts: 2, age_seconds: 240, created_at: "2026-10-20T08:00:00Z" }),
        row("r3", false, { user_id: "u2", attempts: 0, target: "truenas", project_id: "", role_keys: [], reason: "Remove from the members share" }),
      ],
    };
  });

  it("opens with a meta line, search, a read line and one chip per section", async () => {
    await renderPage();
    expect(screen.getByText("1 given up")).toBeInTheDocument();
    expect(screen.getByText("2 on the way")).toBeInTheDocument();
    expect(screen.getByText("3 people")).toBeInTheDocument();
    expect(screen.getByRole("textbox", { name: /Search unfinished revocations/ })).toBeInTheDocument();
    expect(screen.getByText(/Syndra.s send queue/)).toBeInTheDocument();
    const chips = within(screen.getByRole("navigation", { name: "Sections of unfinished revocations" }));
    expect(chips.getByText("Given up")).toBeInTheDocument();
    expect(chips.getByText("Still on the way")).toBeInTheDocument();
  });

  it("puts the first card 12px of padding below the intro, not against it", async () => {
    await renderPage();
    const heading = screen.getByRole("heading", { name: "Unfinished revocations" });
    expect(heading.closest("div.pb-3")).not.toBeNull();
  });

  it("is a table with the board's headings and the retry column", async () => {
    await renderPage();
    for (const name of ["Sort by who", "Sort by what was revoked", "Sort by where", "Sort by revoked"]) {
      expect(screen.getAllByRole("button", { name }).length).toBe(2);
    }
    // A given-up row names the way forward on that person's page, and says why it stopped.
    const link = screen.getByRole("link", { name: /Revoke again on Sai.s page/ });
    expect(link.getAttribute("href")).toBe("/users/u1");
    expect(screen.getByText("Zitadel says this person no longer exists there.")).toBeInTheDocument();
    // A queued row says how many tries, and claims neither a limit nor a next time.
    expect(screen.getByText("Tried 2 times")).toBeInTheDocument();
    expect(screen.getByText("Not tried yet")).toBeInTheDocument();
    expect(screen.queryByText(/Next try/)).toBeNull();
    expect(screen.queryByText(/ of 8/)).toBeNull();
    // Names and roles, never the raw ids; an add-on row says its reason.
    expect(screen.getByText("Remove from the members share")).toBeInTheDocument();
    expect(screen.queryByText("u1")).toBeNull();
  });

  it("search narrows the rows and a section with none says so, not 'nothing given up'", async () => {
    await renderPage();
    fireEvent.change(screen.getByRole("textbox", { name: /Search unfinished revocations/ }), {
      target: { value: "omar" },
    });
    expect(screen.queryByText("Sai Khurana")).toBeNull();
    expect(screen.getByText("Omar Haddad")).toBeInTheDocument();
    expect(screen.getByText("Nothing here matches.")).toBeInTheDocument();
    expect(sectionTitles()).toEqual(["Given up", "Still on the way"]);
  });

  it("sorting by where gathers equal systems under a counted label", async () => {
    await renderPage();
    fireEvent.click(within(document.getElementById("on-the-way")!).getByRole("button", { name: "Sort by where" }));
    const labels = within(document.getElementById("on-the-way")!)
      .getAllByTestId("run-label")
      .map((el) => el.textContent);
    expect(labels).toEqual(["TrueNAS1", "Zitadel1"]);
  });

  it("folds a section to its heading, and a chip opens it again", async () => {
    await renderPage();
    const given = document.getElementById("given-up")!;
    fireEvent.click(within(given).getByRole("button", { name: /Given up/ }));
    expect(within(given).queryByText("Sai Khurana")).toBeNull();
    fireEvent.click(
      within(screen.getByRole("navigation", { name: "Sections of unfinished revocations" })).getByText("Given up"),
    );
    expect(within(given).getByText("Sai Khurana")).toBeInTheDocument();
  });
});
