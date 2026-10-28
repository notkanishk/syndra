// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import PeoplePage from "@/app/users/page";
import type { UserListEntry } from "@/lib/queries/useUsers";
import { gapSettled } from "@/test-utils/leaving";
import { setMediaQuery } from "@/test-utils/media";
import { motionInTests } from "@/test-utils/motion";

const users = vi.hoisted(() => ({ data: [] as UserListEntry[], isPlaceholderData: false }));
const nav = vi.hoisted(() => ({ url: "/users", replaced: [] as string[] }));
const roleMembers = vi.hoisted(() => ({ data: undefined as { members: Array<{ user: { id: string } }> } | undefined }));

vi.mock("@/lib/queries/useUsers", () => ({
  useUsers: () => ({ ...users, isLoading: false, error: null, refetch: () => {} }),
}));

vi.mock("@/lib/queries/useProjects", () => ({
  useProjects: () => ({
    data: [{ project: { id: "pLaser", name: "Laser Lab", roles: [{ key: "trained", label: "Trained" }] } }],
  }),
}));

vi.mock("@/lib/queries/useRoleMembers", () => ({
  useRoleMembers: () => ({ data: roleMembers.data }),
}));

// The page reads its filters from the URL, so the router stand-in is a plain
// query string the tests set before rendering.
vi.mock("next/navigation", () => ({
  useSearchParams: () => new URLSearchParams(nav.url.split("?")[1] ?? ""),
  useRouter: () => ({
    replace: (href: string) => {
      nav.replaced.push(href);
      nav.url = href;
    },
  }),
}));

function person(overrides: Partial<UserListEntry> = {}): UserListEntry {
  return {
    user: {
      id: "u1",
      name: "Tomas Beck",
      email: "tomas@example.org",
      title: "Student Staff",
      team: "Fabrication",
      status: "active",
      avatar: "TB",
    },
    bundle_count: 1,
    bundle_names: ["Lab Tech"],
    effective_role_count: 6,
    project_count: 3,
    key_projects: ["Laser Lab"],
    key_project_ids: ["pLaser"],
    expiring_count: 0,
    open_request_count: 0,
    unexplained_count: 0,
    ...overrides,
  } as UserListEntry;
}

function renderPeople() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <PeoplePage />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  users.data = [person()];
  nav.url = "/users";
  nav.replaced = [];
  roleMembers.data = undefined;
});

describe("People index", () => {
  // A-07 §30: People never includes them, in the list or in the count.
  it("leaves a service account out of the list and the count, marked or machine", () => {
    users.data = [
      person(),
      person({ user: { ...person().user, id: "svc", name: "netbird", title: "", team: "", kind: "machine" } }),
      person({ user: { ...person().user, id: "svc2", name: "Ingestion Station", service_account: true } }),
    ];
    renderPeople();
    expect(screen.getByText("Tomas Beck")).toBeInTheDocument();
    expect(screen.queryByText("netbird")).not.toBeInTheDocument();
    expect(screen.queryByText("Ingestion Station")).not.toBeInTheDocument();
    expect(screen.queryByText("Service account")).not.toBeInTheDocument();
    expect(screen.getByText("1 person")).toBeInTheDocument();
  });

  // A-07 §26: a search that lands on one says so, and points at where it lives.
  it("offers the service account a search matched, linking to its own page", () => {
    nav.url = "/users?q=netb";
    users.data = [
      person(),
      person({ user: { ...person().user, id: "svc", name: "netbird", kind: "machine" } }),
    ];
    renderPeople();
    expect(screen.getByRole("link", { name: "netbird is a service account" })).toHaveAttribute(
      "href",
      "/system/service-accounts/svc",
    );
  });

  it("says nothing about service accounts when the search is empty or matches none", () => {
    users.data = [person({ user: { ...person().user, id: "svc", name: "netbird", kind: "machine" } })];
    renderPeople();
    expect(screen.queryByText(/is a service account/)).not.toBeInTheDocument();
  });

  // Home's "and 26 more →" lands here (A-07 §33).
  it("honours ?bundle=none: a chip in force, newest joined first, undated last", () => {
    nav.url = "/users?bundle=none";
    const bare = { bundle_names: [], bundle_count: 0 };
    users.data = [
      person({ ...bare, user: { ...person().user, id: "a", name: "Old Hand", joined: "2026-01-05T00:00:00Z" } }),
      person({ ...bare, user: { ...person().user, id: "b", name: "Undated" } }),
      person({ ...bare, user: { ...person().user, id: "c", name: "Newcomer", joined: "2026-10-20T00:00:00Z" } }),
      person({ user: { ...person().user, id: "d", name: "Bundled" } }),
    ];
    renderPeople();
    expect(screen.getByText("No bundle")).toBeInTheDocument();
    expect(screen.queryByText("Bundled")).not.toBeInTheDocument();
    const order = screen.getAllByRole("link").map((link) => link.textContent ?? "");
    const at = (name: string) => order.findIndex((text) => text.includes(name));
    expect(at("Newcomer")).toBeLessThan(at("Old Hand"));
    expect(at("Old Hand")).toBeLessThan(at("Undated"));

    fireEvent.click(screen.getByRole("button", { name: /Clear filter: no bundle/ }));
    expect(nav.replaced.at(-1)).toBe("/users");
  });

  it("gives Select's bundle verb its one name", () => {
    nav.url = "/users?bulk=1";
    renderPeople();
    fireEvent.click(screen.getByRole("checkbox", { name: "Select Tomas Beck" }));
    expect(screen.getByRole("button", { name: "Give a bundle" })).toBeInTheDocument();
    expect(screen.queryByText("Add to a bundle")).not.toBeInTheDocument();
  });

  it("keeps a machine account out of 'no access yet', which is for people who arrived", () => {
    nav.url = "/users?attention=no-access";
    users.data = [
      person({ user: { ...person().user, id: "svc", name: "netbird", kind: "machine" }, effective_role_count: 0 }),
      person({ user: { ...person().user, id: "u2", name: "Ines Ward" }, effective_role_count: 0 }),
    ];
    renderPeople();
    expect(screen.getByText("Ines Ward")).toBeInTheDocument();
    expect(screen.queryByText("netbird")).not.toBeInTheDocument();
  });

  it("carries what a person needs, not just what they have", () => {
    users.data = [
      person({ expiring_count: 1, soonest_expiry: new Date(Date.now() + 2 * 86400_000).toISOString() }),
    ];
    renderPeople();
    expect(screen.getByText("Ends in 2 days")).toBeInTheDocument();
  });

  it("shows the most serious signal when a person has more than one", () => {
    users.data = [person({ expiring_count: 1, open_request_count: 1, unexplained_count: 1 })];
    renderPeople();
    // Unexplained access outranks a deadline, which outranks a request.
    expect(screen.getByText("Unexplained role")).toBeInTheDocument();
    expect(screen.queryByText(/open request/)).not.toBeInTheDocument();
  });

  it("renders nothing in Needs attention when a row is clear", () => {
    // A lone dash under the access text was the only thing in the column at
    // phone width — worse than nothing there at all.
    renderPeople();
    expect(screen.queryByText("—")).not.toBeInTheDocument();
  });

  it("names bundles instead of counting them", () => {
    users.data = [person({ bundle_names: ["Lab Tech", "Studio Member"] })];
    renderPeople();
    expect(screen.getByText("Lab Tech")).toBeInTheDocument();
    expect(screen.getByText("Studio Member")).toBeInTheDocument();
  });

  it("says it hasn't been checked before Zitadel has ever been read", () => {
    renderPeople();
    expect(screen.getByText("Not checked yet")).toBeInTheDocument();
  });

  it("says how much access somebody has in words, off what Zitadel observed", () => {
    users.data = [
      person({
        observation: { read_at: "2026-09-10T00:00:00Z", current: true, truncated: false },
        observed_role_count: 6,
        observed_project_count: 3,
      }),
    ];
    renderPeople();
    expect(screen.getByText("6 roles across 3 projects")).toBeInTheDocument();
  });

  it("distinguishes recorded-but-not-in-Zitadel from genuinely nothing", () => {
    users.data = [
      person({
        observation: { read_at: "2026-09-10T00:00:00Z", current: true, truncated: false },
        observed_role_count: 0,
        effective_role_count: 4,
      }),
      person({
        user: { ...person().user, id: "u2", name: "Nina Roy" },
        observation: { read_at: "2026-09-10T00:00:00Z", current: true, truncated: false },
        observed_role_count: 0,
        effective_role_count: 0,
      }),
    ];
    renderPeople();
    expect(screen.getByText("Not in Zitadel yet")).toBeInTheDocument();
    expect(screen.getByText("Nothing yet")).toBeInTheDocument();
  });

  it("keeps a departed account visible, at reduced contrast", () => {
    users.data = [person({ user: { ...person().user, status: "departed" } })];
    renderPeople();
    const row = screen.getByRole("link", { name: /Tomas Beck/ });
    expect(row.className).toContain("opacity-60");
  });

  it("tells people they can search by role key", () => {
    renderPeople();
    expect(
      screen.getByPlaceholderText("Search name, email or role key…"),
    ).toBeInTheDocument();
  });

  it("paginates explicitly rather than scrolling forever", () => {
    users.data = Array.from({ length: 60 }, (_, index) =>
      person({ user: { ...person().user, id: `u${index}`, name: `Person ${index}` } }),
    );
    renderPeople();
    expect(screen.getByText("Showing 10 of 60")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Show 25 more" })).toBeInTheDocument();
  });

  it("counts how many people have access expiring, in the header", () => {
    users.data = [person({ expiring_count: 1 }), person({ user: { ...person().user, id: "u2" } })];
    renderPeople();
    expect(screen.getByText(/1 with access ending inside 30 days/)).toBeInTheDocument();
  });
});

describe("People index — filters come from the URL", () => {
  it("narrows to a project by id, so a link survives a rename", () => {
    users.data = [
      person(),
      person({
        user: { ...person().user, id: "u2", name: "Nina Roy" },
        key_projects: ["Wood Shop"],
        key_project_ids: ["pWood"],
      }),
    ];
    nav.url = "/users?project=pLaser";
    renderPeople();
    expect(screen.getByText("Tomas Beck")).toBeInTheDocument();
    expect(screen.queryByText("Nina Roy")).not.toBeInTheDocument();
  });

  it("narrows to an attention cohort that no column exposes on its own", () => {
    users.data = [
      person({ effective_role_count: 0, project_count: 0 }),
      person({ user: { ...person().user, id: "u2", name: "Nina Roy" } }),
    ];
    nav.url = "/users?attention=no-access";
    renderPeople();
    expect(screen.getByText("Tomas Beck")).toBeInTheDocument();
    expect(screen.queryByText("Nina Roy")).not.toBeInTheDocument();
  });

  it("counts a departed person as work only while they still hold roles", () => {
    users.data = [
      person({ user: { ...person().user, status: "departed" } }),
      person({
        user: { ...person().user, id: "u2", name: "Nina Roy", status: "departed" },
        effective_role_count: 0,
      }),
    ];
    nav.url = "/users?attention=departed";
    renderPeople();
    expect(screen.getByText("Tomas Beck")).toBeInTheDocument();
    // Cleanly offboarded — not work, and listing it would bury the ones that are.
    expect(screen.queryByText("Nina Roy")).not.toBeInTheDocument();
  });

  it("takes role membership from the role endpoint, not from the people list", () => {
    users.data = [
      person(),
      person({ user: { ...person().user, id: "u2", name: "Nina Roy" } }),
    ];
    roleMembers.data = { members: [{ user: { id: "u1" } }] };
    nav.url = "/users?project=pLaser&role=trained";
    renderPeople();
    expect(screen.getByText("Tomas Beck")).toBeInTheDocument();
    expect(screen.queryByText("Nina Roy")).not.toBeInTheDocument();
  });

  it("shows everyone rather than nobody while role membership is still loading", () => {
    users.data = [person(), person({ user: { ...person().user, id: "u2", name: "Nina Roy" } })];
    roleMembers.data = undefined;
    nav.url = "/users?project=pLaser&role=trained";
    renderPeople();
    // An empty list would read as "nobody holds this", which is a different
    // and much worse claim than "still loading".
    expect(screen.getByText("Tomas Beck")).toBeInTheDocument();
    expect(screen.getByText("Nina Roy")).toBeInTheDocument();
  });
});

describe("People index — bulk mode", () => {
  it("shows no checkboxes at all until bulk mode is on", () => {
    renderPeople();
    expect(screen.queryByRole("checkbox")).not.toBeInTheDocument();
    expect(screen.queryByRole("region", { name: "Bulk actions" })).not.toBeInTheDocument();
  });

  it("keeps the row a link out of bulk mode and a control inside it", () => {
    renderPeople();
    expect(screen.getByRole("link", { name: /Tomas Beck/ })).toBeInTheDocument();

    cleanup();
    nav.url = "/users?bulk=1";
    renderPeople();
    expect(screen.getByRole("checkbox", { name: "Select Tomas Beck" })).toBeInTheDocument();
    // The name is still a way to reach the person — the rest of the row selects.
    expect(screen.getByRole("link", { name: "Tomas Beck" })).toBeInTheDocument();
  });

  it("selects every row matching the filter, not just the rendered page", async () => {
    users.data = Array.from({ length: 60 }, (_, index) =>
      person({ user: { ...person().user, id: `u${index}`, name: `Person ${index}` } }),
    );
    nav.url = "/users?bulk=1";
    renderPeople();

    // Both numbers, in words: what ticking this selects, and how many exist
    // beyond the filter. A bare "Select all" is the ambiguity worth removing.
    fireEvent.click(screen.getByRole("checkbox", { name: /Select these 60 people/ }));

    // ...and says so, with a way back to just the visible page: selecting 60
    // when you meant 10 is dozens of people's access.
    expect(await screen.findByText(/All 60 people selected/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /select only the 10 people shown/ })).toBeInTheDocument();
  });

  it("won't select every match while a new search is still being read", () => {
    users.data = Array.from({ length: 60 }, (_, index) =>
      person({ user: { ...person().user, id: `u${index}`, name: `Person ${index}` } }),
    );
    // The rows on screen still answer the last search.
    users.isPlaceholderData = true;
    nav.url = "/users?bulk=1";
    renderPeople();

    fireEvent.click(screen.getByRole("checkbox", { name: /Select these 60 people/ }));
    expect(screen.queryByText(/All 60 people selected/)).toBeNull();
    users.isPlaceholderData = false;
  });

  // The name is the largest thing in a row and the obvious place a thumb
  // lands. With a mouse it is a precise escape hatch out of the mode; with a
  // thumb it is a trap, so below the desktop breakpoint the whole row selects.
  it("keeps the name a link for a mouse and not for a thumb", async () => {
    users.data = [person()];
    nav.url = "/users?bulk=1";
    setMediaQuery("(max-width: 67.49rem)", false);
    const { unmount } = renderPeople();
    expect(await screen.findByRole("link", { name: /Tomas Beck/ })).toBeTruthy();
    unmount();

    setMediaQuery("(max-width: 67.49rem)", true);
    renderPeople();
    await waitFor(() => expect(screen.queryByRole("link", { name: /Tomas Beck/ })).toBeNull());
  });

  it("drops the selection when the filter changes underneath it", async () => {
    users.data = [person(), person({ user: { ...person().user, id: "u2", name: "Nina Roy" } })];
    nav.url = "/users?bulk=1";
    const { rerender } = renderPeople();

    fireEvent.click(screen.getByRole("checkbox", { name: "Select Tomas Beck" }));
    expect(await screen.findByText(/1 person selected/)).toBeInTheDocument();

    // A filter change re-aims the action at a different set of people, so the
    // pending selection cannot survive it.
    nav.url = "/users?bulk=1&attention=expiring";
    rerender(
      <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
        <PeoplePage />
      </QueryClientProvider>,
    );
    await waitFor(() => expect(screen.queryByText(/1 person selected/)).not.toBeInTheDocument());
  });

  it("names the filter in the selection bar rather than only counting", async () => {
    nav.url = "/users?bulk=1&project=pLaser";
    renderPeople();
    fireEvent.click(screen.getByRole("checkbox", { name: "Select Tomas Beck" }));
    expect(await screen.findByText(/in Laser Lab/)).toBeInTheDocument();
  });
});

// "Add people to this role" sent the role as a filter, so People listed only
// the people who already held it: the ones nobody needed to add.
describe("People · choosing who gets a role", () => {
  it("lists everyone, holders included, and says what the pick is for", () => {
    users.data = [person({ user: { ...person().user, id: "u1", name: "Holds it" } }), person({ user: { ...person().user, id: "u2", name: "Needs it" }, key_project_ids: [] })];
    roleMembers.data = { members: [{ user: { id: "u1" } }] };
    nav.url = "/users?bulk=1&give_project=pLaser&give_role=trained";
    renderPeople();
    expect(screen.getByText("Holds it")).toBeInTheDocument();
    expect(screen.getByText("Needs it")).toBeInTheDocument();
    expect(screen.getByText(/Pick who should get/)).toBeInTheDocument();
  });
});

// The board (route-users, undated, with RULES of 9 and 17 Oct over it): the team
// rides in the subtitle, not a column; headings sort; the list says how fresh it
// is; the attention cell says what and when; nothing a decision rests on is cut.
describe("People · the board", () => {
  const headings = () => {
    const row = document.querySelector("[class*='tracking-[0.09em]']") as HTMLElement;
    return Array.from(row.children).map((cell) => (cell.textContent ?? "").replace(/[↑↓]/g, "").trim());
  };
  const named = (name: string, over: Partial<UserListEntry> = {}) =>
    person({ user: { ...person().user, id: name, name }, ...over });

  it("reads Person, Bundles, Access, Attention, with no Team column", () => {
    renderPeople();
    expect(headings()).toEqual(["Person", "Bundles", "Access", "Attention"]);
  });

  it("puts title and team under the name, and the email only when there is no title", () => {
    users.data = [
      named("Aanya Rao", { user: { ...person().user, id: "a", name: "Aanya Rao", title: "Research assistant", team: "Mechanical Engineering" } }),
      named("Lena Brandt", { user: { ...person().user, id: "l", name: "Lena Brandt", title: "", team: "", email: "lena@example.edu" } }),
    ];
    renderPeople();
    expect(screen.getByText("Research assistant · Mechanical Engineering")).toBeInTheDocument();
    expect(screen.getByText("lena@example.edu")).toBeInTheDocument();
    expect(screen.queryByText("tomas@example.org")).not.toBeInTheDocument();
  });

  it("sorts on Person, Bundles and Attention, the Person arrow up by default", () => {
    users.data = [
      named("Zed", { bundle_names: ["Alpha"], unexplained_count: 0 }),
      named("Amy", { bundle_names: ["Zulu"], expiring_count: 1, soonest_expiry: new Date(Date.now() + 2 * 86400_000).toISOString() }),
      named("Bo", { bundle_names: ["Mid"], unexplained_count: 2 }),
    ];
    renderPeople();
    const order = () => screen.getAllByRole("link").map((a) => a.textContent ?? "").map((t) => ["Zed", "Amy", "Bo"].find((n) => t.includes(n)));
    expect(screen.getByRole("button", { name: "Sort by person" })).toHaveAttribute("data-sort", "asc");
    expect(order()).toEqual(["Amy", "Bo", "Zed"]);
    fireEvent.click(screen.getByRole("button", { name: "Sort by person" }));
    expect(order()).toEqual(["Zed", "Bo", "Amy"]);
    fireEvent.click(screen.getByRole("button", { name: "Sort by bundles" }));
    expect(order()).toEqual(["Zed", "Bo", "Amy"]);
    fireEvent.click(screen.getByRole("button", { name: "Sort by attention" }));
    expect(screen.getByRole("button", { name: "Sort by attention" })).toHaveAttribute("data-sort", "asc");
    // The first press puts the people who need you on top: unexplained, then
    // the deadline, then the clear row.
    expect(order()).toEqual(["Bo", "Amy", "Zed"]);
  });

  it("says how fresh the list is", () => {
    users.data = [
      person({ observation: { read_at: new Date().toISOString(), current: true, truncated: false } }),
    ];
    renderPeople();
    expect(screen.getByText("The people list was read just now")).toBeInTheDocument();
  });

  it("names the attention in words with the day it falls", () => {
    users.data = [
      named("One", { expiring_count: 1, soonest_expiry: new Date(Date.now() + 2 * 86400_000).toISOString() }),
      named("Two", { unexplained_count: 3 }),
      named("Three", { open_request_count: 1 }),
      named("Four", { open_request_count: 2 }),
    ];
    renderPeople();
    expect(screen.getByText("Ends in 2 days")).toBeInTheDocument();
    expect(screen.getByText("3 unexplained roles")).toBeInTheDocument();
    expect(screen.getByText("Open request")).toBeInTheDocument();
    expect(screen.getByText("2 open requests")).toBeInTheDocument();
  });

  it("never cuts the Access sentence, and wraps attention", () => {
    users.data = [
      person({
        observation: { read_at: "2026-09-10T00:00:00Z", current: true, truncated: false },
        observed_role_count: 6,
        observed_project_count: 4,
        unexplained_count: 1,
      }),
    ];
    renderPeople();
    expect(screen.getByText("6 roles across 4 projects").className).not.toMatch(/truncate/);
    expect(screen.getByText("Unexplained role").className).not.toMatch(/truncate|whitespace-nowrap/);
  });

  // Seen live: typing in the search dropped 10 rows to 2 by removing every row and adding two,
  // because the new read was a loading list. The rows that match are the same rows (A-05 §2.10).
  describe("a search narrowing the list", () => {
    it("holds the rows that still match, fades the rest where they were, and settles the gap after", () => {
      motionInTests();
      const named = (id: string, name: string) => person({ user: { ...person().user, id, name } });
      const everyone = [named("a", "Ada Lin"), named("b", "Bo Chen"), named("c", "Cy Ward")];
      users.data = everyone;
      const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
      const page = () => (
        <QueryClientProvider client={client}>
          <PeoplePage />
        </QueryClientProvider>
      );
      const { rerender } = render(page());
      const ada = screen.getByText("Ada Lin");

      // The query is typed first; the read for it lands after.
      nav.url = "/users?q=ada";
      rerender(page());
      users.data = [everyone[0]];
      rerender(page());

      expect(screen.getByText("Ada Lin")).toBe(ada);
      expect(ada.closest("[data-leaving]")).toBeNull();
      expect(screen.getByText("Bo Chen").closest("[data-leaving]")).not.toBeNull();
      expect(screen.getByText("Cy Ward").closest("[data-leaving]")).not.toBeNull();
      expect(screen.getByText("Bo Chen").closest("[data-closing]")).not.toBeNull();

      // Their gaps have settled and they are gone. Clearing it brings them back: the gap
      // opens, then they tint in. The rows that stayed hold still.
      gapSettled(screen.getByText("Bo Chen").closest("[data-closing]")!);
      gapSettled(screen.getByText("Cy Ward").closest("[data-closing]")!);
      expect(screen.queryByText("Bo Chen")).toBeNull();
      nav.url = "/users";
      rerender(page());
      users.data = everyone;
      rerender(page());
      expect(screen.getByText("Ada Lin")).toBe(ada);
      expect(screen.getByText("Bo Chen").closest("[data-opens]")).not.toBeNull();
      expect(ada.closest("[data-opens]")).toBeNull();
      expect(ada.closest("[data-fresh]")).toBeNull();
    });
  });

  const SEARCH = "Search people by name, email or role key";

  // Back/Forward and a nav click change the URL under the box.
  it("re-syncs the search box when the URL's search changes from outside, and keeps what is being typed through its own echo", () => {
    vi.useFakeTimers();
    try {
      const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
      const page = () => (
        <QueryClientProvider client={client}>
          <PeoplePage />
        </QueryClientProvider>
      );
      const { rerender } = render(page());
      const box = screen.getByLabelText(SEARCH) as HTMLInputElement;

      fireEvent.change(box, { target: { value: "ab" } });
      act(() => void vi.advanceTimersByTime(300));
      expect(nav.replaced.at(-1)).toContain("q=ab");
      // More typing lands before the page's own write comes back through the URL.
      fireEvent.change(box, { target: { value: "abc" } });
      rerender(page());
      expect(box.value).toBe("abc");

      // Back to somebody else's search.
      nav.url = "/users?q=zed";
      rerender(page());
      expect(box.value).toBe("zed");
    } finally {
      vi.useRealTimers();
    }
  });

  describe("Esc in bulk mode", () => {
    it("leaves bulk mode when nothing else took the key", () => {
      nav.url = "/users?bulk=1";
      renderPeople();
      fireEvent.keyDown(document.body, { key: "Escape" });
      expect(nav.replaced.at(-1)).toBe("/users");
    });

    it("closes an open bulk dialog without also dropping the selection", () => {
      nav.url = "/users?bulk=1";
      renderPeople();
      fireEvent.click(screen.getByRole("checkbox", { name: "Select Tomas Beck" }));
      fireEvent.click(screen.getByRole("button", { name: "Give a bundle" }));
      expect(screen.getByRole("dialog")).toBeInTheDocument();
      const before = nav.replaced.length;

      fireEvent.keyDown(document.body, { key: "Escape" });
      expect(screen.queryByRole("dialog")).toBeNull();
      expect(nav.replaced.length).toBe(before);
    });

    it("ignores an Esc something already handled", () => {
      nav.url = "/users?bulk=1";
      renderPeople();
      const swallow = (event: KeyboardEvent) => event.preventDefault();
      document.addEventListener("keydown", swallow);
      fireEvent.keyDown(document.body, { key: "Escape" });
      document.removeEventListener("keydown", swallow);
      expect(nav.replaced).toEqual([]);
    });
  });
});
