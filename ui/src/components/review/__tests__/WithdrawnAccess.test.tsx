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

const NAMES: Record<string, string> = { u1: "Iyo Sky", u2: "Jimmy Uso", u3: "Tiffany Stratton" };
const PROJECTS: Record<string, string> = { pWood: "Wood shop" };

vi.mock("@/lib/queries/useFieldSchema", async (orig) => (await import("@/lib/__fixtures__/field-schema-mock")).withRosterNames(orig));
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

// The endpoint's own shape: a page of rows, and the totals it counted itself.
type Row = ReturnType<typeof row>;
function listing(revocations: Row[], total?: { spent: number; queued: number }) {
  const spent = revocations.filter((r) => r.spent).length;
  return {
    revocations,
    summary: { spent: total?.spent ?? spent, queued: total?.queued ?? revocations.length - spent, oldest_age_seconds: 0 },
    escalated: false,
  };
}

let payload: unknown = listing([]);
// A system's standing findings, for the one read the page makes per system (7t-6).
let findings: Array<{ subject_id: string; outcome: string }> = [];

vi.mock("@/lib/api-client", async () => {
  const actual = await vi.importActual<typeof import("@/lib/api-client")>("@/lib/api-client");
  return {
    ...actual,
    request: vi.fn(async (path: string) => (path.endsWith("/merge-findings") ? { findings } : payload)),
  };
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
  payload = listing([row("r1", false)]);
});

describe("what has not gone away yet", () => {
  it("keeps both sections, in one order, whatever either holds", async () => {
    await renderPage();
    expect(sectionTitles(), "terminal first, always").toEqual(["Given up", "Still on the way"]);
    // The empty one is present, and says what its emptiness means: the good news on this page.
    expect(screen.getByText(/Nothing has been given up on/)).toBeInTheDocument();
  });

  it("arrives its two sections on first paint, as every page's cards do (S11)", async () => {
    await renderPage();
    for (const section of document.querySelectorAll("section")) {
      expect(section.parentElement?.classList.contains("arrive-list")).toBe(true);
    }
  });

  it("keeps its read line when nothing is unfinished, as a sentence with a full stop", async () => {
    payload = listing([]);
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(
      <QueryClientProvider client={client}>
        <WithdrawnAccess />
      </QueryClientProvider>,
    );
    expect(await screen.findByText("Everyone whose access was revoked is out.")).toBeInTheDocument();
    expect(screen.getByText(/This list/)).toBeInTheDocument();
  });

  it("does not insert a card above the queue when one goes terminal", async () => {
    payload = listing([row("r1", false), row("r2", true, { user_id: "u2" })]);
    await renderPage();
    expect(sectionTitles()).toEqual(["Given up", "Still on the way"]);
    expect(screen.queryByText(/Nothing has been given up on/)).toBeNull();
  });
});

describe("what happens next sits in one seat (D-1)", () => {
  it("is the same wide seat on a row still being tried and on one given up, and its heading cell matches", async () => {
    payload = listing([row("r1", false), row("r2", true, { user_id: "u2" })]);
    await renderPage();
    const seats = Array.from(document.querySelectorAll<HTMLElement>("[data-action-seat]"));
    expect(seats).toHaveLength(2);
    expect(seats.map((seat) => seat.getAttribute("data-seat"))).toEqual(["wide", "wide"]);
    // The row still being tried says how many tries so far; the given-up one points where to act.
    // Given up comes first, always.
    expect(within(seats[0]).getByRole("link")).toBeInTheDocument();
    expect(seats[1].textContent).toMatch(/Tried 3 times/);
    for (const heading of Array.from(document.querySelectorAll('.row-seat[aria-hidden="true"]'))) {
      expect(heading.getAttribute("data-seat")).toBe("wide");
    }
  });
});

describe("why a given-up revocation stopped", () => {
  // The same helper as Pending changes (`stoppedLine`): a stored reason is never printed bare.
  it("says it in a sentence, never the stored text of an out-of-retries or a refusal alone", async () => {
    payload = listing([
      row("r1", true, { user_id: "u1", attempts: 5, last_error: "out of retries after 5 attempts: Zitadel could not be reached" }),
      row("r2", true, { user_id: "u2", wait_reason: "refused:not_found", last_error: "zitadel api 404" }),
    ]);
    await renderPage();
    expect(screen.getByText("Zitadel didn't answer after 5 tries, so Syndra stopped sending it.")).toBeInTheDocument();
    expect(screen.getByText("Zitadel refused it: it no longer has this person, project or role, so Syndra stopped sending it.")).toBeInTheDocument();
    expect(screen.queryByText(/out of retries/)).toBeNull();
    expect(screen.queryByText(/zitadel api 404/)).toBeNull();
  });

  // 7t-6: a revocation the system blocked on the person's binding conflict carries its wait reason,
  // so while that conflict stands it is sent where it is decided, not to the person's account; once
  // it is decided or closed, the row has its ordinary link.
  it("sends a revocation blocked by a binding conflict to Waiting on you while the conflict stands", async () => {
    const said = "The account rey, recorded for this person, is gone, and its number is now held by a different account, kofi.";
    findings = [{ subject_id: "u1", outcome: "binding_conflict" }, { subject_id: "u2", outcome: "conflict" }];
    payload = listing([
      row("r1", true, { user_id: "u1", target: "truenas", project_id: "", role_keys: [], reason: "Remove from the members share", wait_reason: "binding_conflict", last_error: said }),
      row("r2", true, { user_id: "u2", target: "truenas", project_id: "", role_keys: [], reason: "Remove from the members share", wait_reason: "target_refused", last_error: "It is held." }),
    ]);
    await renderPage();
    expect(screen.getByText(`TrueNAS refused it: ${said} Syndra stopped sending it.`)).toBeInTheDocument();
    const decide = await screen.findAllByRole("link", { name: "Decide it under Waiting on you" });
    expect(decide).toHaveLength(1);
    expect(decide[0]).toHaveAttribute("href", "/system/targets/truenas#waiting");
    // The plain refusal keeps its own step.
    expect(screen.getByRole("link", { name: /Check Jimmy’s account on TrueNAS/ })).toBeInTheDocument();
    findings = [];
  });

  it("gives a decided binding conflict's revocation its ordinary link", async () => {
    findings = [];
    payload = listing([
      row("r1", true, { user_id: "u1", target: "truenas", project_id: "", role_keys: [], reason: "Remove from the members share", wait_reason: "binding_conflict", last_error: "Held." }),
    ]);
    await renderPage();
    expect(await screen.findByRole("link", { name: /Check Iyo’s account on TrueNAS/ })).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "Decide it under Waiting on you" })).toBeNull();
  });
});

describe("the board's anatomy", () => {
  beforeEach(() => {
    payload = listing([
        row("r1", true, { user_id: "u1", last_error: "Zitadel says this person no longer exists there.", cascade_id: "c-abc123" }),
        row("r2", false, { user_id: "u3", attempts: 2, age_seconds: 240, created_at: "2026-10-20T08:00:00Z" }),
        row("r3", false, { user_id: "u2", attempts: 0, target: "truenas", project_id: "", role_keys: [], reason: "Remove from the members share" }),
    ]);
  });

  it("opens with a meta line, search, a read line and one chip per section", async () => {
    await renderPage();
    expect(screen.getByText("1 given up")).toBeInTheDocument();
    expect(screen.getByText("2 on the way")).toBeInTheDocument();
    expect(screen.getByText("3 people")).toBeInTheDocument();
    expect(screen.getByRole("searchbox", { name: /Search unfinished revocations/ })).toBeInTheDocument();
    expect(screen.getByText(/This list/)).toBeInTheDocument();
    const chips = within(screen.getByRole("navigation", { name: "Sections of unfinished revocations" }));
    expect(chips.getByText("Given up")).toBeInTheDocument();
    expect(chips.getByText("Still on the way")).toBeInTheDocument();
    // Every chip wears a dot: the queued one is neutral, not toneless (A-17 §2.19).
    expect(chips.getAllByRole("link").map((chip) => chip.querySelectorAll("[data-dot]").length)).toEqual([1, 1]);
  });

  it("counts what is outstanding from the endpoint's totals, not from the page it was sent", async () => {
    // The page is capped at 200 rows; the header must not read the cap as the whole queue.
    payload = listing([row("r1", true), row("r2", false)], { spent: 40, queued: 310 });
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(
      <QueryClientProvider client={client}>
        <WithdrawnAccess />
      </QueryClientProvider>,
    );
    expect(await screen.findByText("40 given up")).toBeInTheDocument();
    expect(screen.getByText("310 on the way")).toBeInTheDocument();
    // A capped page cannot say how many people are behind the rest.
    expect(screen.queryByText(/\d (person|people)$/)).toBeNull();
  });

  it("says nothing about 'Tries again by itself' on every row, and a hint on neither card", async () => {
    await renderPage();
    expect(screen.queryByText(/Tries again by itself/)).toBeNull();
    expect(screen.queryByText(/Syndra keeps trying by itself/)).toBeNull();
    expect(screen.queryByText(/They still have it until somebody ends it/)).toBeNull();
  });

  it("is a table with the board's headings and the retry column", async () => {
    await renderPage();
    for (const name of ["Sort by who", "Sort by what was revoked", "Sort by where", "Sort by revoked"]) {
      expect(screen.getAllByRole("button", { name }).length).toBe(2);
    }
    // A given-up row names the way forward, and says why it stopped. Its record
    // is gone, so not the person's page (nothing left to revoke there): Drift,
    // where the access Zitadel still holds is raised (re-audit R11).
    const link = screen.getByRole("link", { name: /Revoke it from Drift/ });
    expect(link.getAttribute("href")).toBe("/governance/drift");
    expect(screen.queryByRole("link", { name: /Revoke again on/ })).toBeNull();
    expect(screen.getByText("Zitadel refused it, so Syndra stopped sending it.")).toBeInTheDocument();
    // A queued row says how many tries, and claims neither a limit nor a next time.
    expect(screen.getByText("Tried 2 times")).toBeInTheDocument();
    expect(screen.getByText("Not tried yet")).toBeInTheDocument();
    // The change that set it off is a link in words, never its raw id.
    expect(screen.getByRole("link", { name: /Change history/ }).getAttribute("href")).toBe("/operations/cascades?cascade=c-abc123");
    expect(screen.queryByText(/c-abc123/)).toBeNull();
    expect(screen.queryByText(/Next try/)).toBeNull();
    expect(screen.queryByText(/ of 8/)).toBeNull();
    // Names and roles, never the raw ids; an add-on row says its reason.
    expect(screen.getByText("Remove from the members share")).toBeInTheDocument();
    expect(screen.queryByText("u1")).toBeNull();
  });

  it("search narrows the rows and a section with none says so, not 'nothing given up'", async () => {
    await renderPage();
    fireEvent.change(screen.getByRole("searchbox", { name: /Search unfinished revocations/ }), {
      target: { value: "tiffany" },
    });
    expect(screen.queryByText("Iyo Sky")).toBeNull();
    expect(screen.getByText("Tiffany Stratton")).toBeInTheDocument();
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
    expect(within(given).queryByText("Iyo Sky")).toBeNull();
    fireEvent.click(
      within(screen.getByRole("navigation", { name: "Sections of unfinished revocations" })).getByText("Given up"),
    );
    expect(within(given).getByText("Iyo Sky")).toBeInTheDocument();
  });
});
