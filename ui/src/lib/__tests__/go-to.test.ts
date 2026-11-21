import { describe, expect, it } from "vitest";

import { GO_TO_SHOWN, flatItems, fold, goTo, goToNamed, goToRows, offeredScope, rankName, sigilScope, type GoToSource } from "@/lib/go-to";
import { ADVANCED_NAV, BASIC_NAV, memberNav } from "@/lib/nav";

const user = (id: string, name: string, extra: object = {}) =>
  ({ id, name, email: `${id}@example.edu`, title: "", team: "Fabrication", status: "active", avatar: "", ...extra }) as never;

const source = (over: Partial<GoToSource> = {}): GoToSource => ({
  nav: ADVANCED_NAV,
  names: true,
  bundles: true,
  people: [user("u3", "Prince Nana"), user("u1", "Primo Colon"), user("s1", "Printer Bot", { service_account: true }), user("u2", "Damian Priest")],
  projects: [
    { id: "p2", name: "Print Lab", kind: "", description: "", roles: [{ key: "trained", label: "Trained", description: "" }] },
    { id: "p1", name: "Laser Lab", kind: "", description: "", roles: [] },
  ],
  apps: [{ id: "a1", name: "Print Queue", project_id: "p2", description: "", consumer: "oidc", claim_name: "", format_type: "" }],
  bundleList: [{ id: "b1", name: "Printing basics" }, { id: "b0", name: "Community" }] as never,
  ...over,
});

describe("Go to", () => {
  it("lists pages only while nothing is typed", () => {
    expect(goTo(source(), "").map((g) => g.group)).toEqual(["Pages"]);
  });

  it("puts the groups in a fixed order and each group A to Z, never by score", () => {
    const groups = goTo(source(), "pri");
    expect(groups.map((g) => g.group)).toEqual(["People", "Projects", "Apps", "Bundles"]);
    const people = groups.find((g) => g.group === "People")!.items.map((i) => i.name);
    // People A to Z, service accounts after them under the same group.
    expect(people).toEqual(["Damian Priest", "Primo Colon", "Prince Nana", "Printer Bot"]);
    // Pages come first and are A to Z too.
    const [pages, ...rest] = goTo(source(), "s");
    expect(pages.group).toBe("Pages");
    expect(pages.items.map((i) => i.name)).toEqual([...pages.items.map((i) => i.name)].sort((a, b) => a.localeCompare(b)));
    expect(rest.length).toBeGreaterThan(0);
  });

  it("matches case and accents aside", () => {
    const groups = goTo(source({ people: [user("u9", "José Álvarez")] }), "JOSE alv");
    expect(flatItems(groups).map((i) => i.name)).toEqual(["José Álvarez"]);
  });

  it("leaves a group with no match out, and says nothing when nothing matches", () => {
    expect(goTo(source(), "zzzz")).toEqual([]);
    expect(goTo(source(), "laser").map((g) => g.group)).toEqual(["Projects"]);
  });

  it("finds roles under their project", () => {
    const [roles] = goTo(source(), "trained");
    expect(roles.group).toBe("Roles");
    expect(roles.items[0]).toMatchObject({ descriptor: "Print Lab", href: "/projects/p2/roles/trained" });
  });

  it("follows the view: Advanced pages and Bundles appear only in Advanced", () => {
    const basic = goTo(source({ nav: BASIC_NAV, bundles: false }), "pending");
    expect(basic).toEqual([]);
    expect(goTo(source({ nav: BASIC_NAV, bundles: false }), "print").map((g) => g.group)).not.toContain("Bundles");
    const advanced = goTo(source(), "pending");
    expect(advanced[0].items.map((i) => i.name)).toContain("Pending changes");
  });

  it("gives a member their own pages and no names", () => {
    const groups = goTo(source({ nav: memberNav([]), names: false }), "a");
    expect(groups.map((g) => g.group)).toEqual(["Pages"]);
    expect(flatItems(groups).map((i) => i.name)).toEqual(["My access", "Network storage"]);
  });

  it("links each kind to its page", () => {
    const hrefs = Object.fromEntries(flatItems(goTo(source(), "pri")).map((i) => [i.key, i.href]));
    expect(hrefs["person:u3"]).toBe("/users/u3");
    expect(hrefs["project:p2"]).toBe("/projects/p2");
    expect(hrefs["app:a1"]).toBe("/applications/a1");
    expect(hrefs["bundle:b1"]).toBe("/bundles/b1");
  });
});

describe("fold", () => {
  it("drops case and accents, so a name is found however it is typed", () => {
    expect(fold("  Émile Zoë Ångström ")).toBe("emile zoe angstrom");
  });

});

describe("Go to's one list (A-26 §0)", () => {
  it("at rest holds the last place only; asked for more, five, and what needs you last when something waits", () => {
    const lately = ["/users/u1", "/applications", "/users/u2"];
    expect(goToRows({ ...source(), lately }, "", null).map((r) => [r.name, r.tag])).toEqual([["Primo Colon", "Lately"]]);
    const more = goToRows({ ...source(), lately, waiting: 2 }, "", null, true);
    expect(more.map((r) => r.name)).toEqual(["Primo Colon", "Apps", "Damian Priest", "Pending changes"]);
    expect(more.at(-1)).toMatchObject({ tag: "Needs you", descriptor: "2 changes are waiting for you to send" });
    // Nothing waiting, or a view with no such page: no line.
    expect(goToRows({ ...source(), lately, waiting: 0 }, "", null, true).some((r) => r.tag === "Needs you")).toBe(false);
    expect(goToRows({ ...source({ nav: BASIC_NAV }), lately, waiting: 2 }, "", null, true).some((r) => r.tag === "Needs you")).toBe(false);
  });

  it("looking everywhere, lists the best match first, seven at most, each with its kind", () => {
    const rows = goToRows(source(), "pri", null);
    // Equal matches keep the catalogue's order: people (service accounts after them), projects, apps, bundles.
    expect(rows.map((r) => r.name)).toEqual(["Primo Colon", "Prince Nana", "Printer Bot", "Print Lab", "Print Queue", "Printing basics", "Damian Priest"]);
    expect(rows.map((r) => r.tag)).toEqual(["Person", "Person", "Service account", "Project", "App", "Bundle", "Person"]);
    expect(goToRows(source(), "p", null)).toHaveLength(GO_TO_SHOWN);
    // A whole name before its start, its start before a word inside it.
    expect(goToRows(source(), "print lab", null)[0].name).toBe("Print Lab");
  });

  it("in a scope, lists every one of that kind that matches, A to Z", () => {
    // People A to Z, then service accounts A to Z, as every picker does.
    expect(goToRows(source(), "", "people").map((r) => r.name)).toEqual(["Damian Priest", "Primo Colon", "Prince Nana", "Printer Bot"]);
    expect(goToRows(source(), "pri", "people").map((r) => r.name)).toEqual(["Damian Priest", "Primo Colon", "Prince Nana", "Printer Bot"]);
    expect(goToRows(source(), "", "roles").map((r) => r.name)).toEqual(["Trained"]);
    expect(goToRows(source(), "rules", "pages").map((r) => r.name)).toEqual(["Automatic rules"]);
  });

  it("offers a kind's scope from two letters of its word, and reads a scope's first key", () => {
    expect(offeredScope("p")).toBeNull();
    expect(offeredScope("pe")).toBe("people");
    expect(offeredScope("Peo")).toBe("people");
    expect(offeredScope("person")).toBe("people");
    expect(offeredScope("rol")).toBe("roles");
    expect(offeredScope("pag")).toBe("pages");
    expect(offeredScope("pri")).toBeNull();
    expect([sigilScope("@"), sigilScope("#la"), sigilScope("/"), sigilScope("a@")]).toEqual(["people", "roles", "pages", null]);
  });

  it("under an answer, lists the places its question names, best first, three at most", () => {
    expect(goToNamed(source(), [])).toEqual([]);
    expect(goToNamed(source(), ["Trained", "Print Lab"]).map((r) => [r.name, r.tag])).toEqual([
      ["Print Lab", "Project"],
      ["Trained", "Role"],
    ]); // equally good matches keep the list's own order
    // A name that starts many: three at most.
    expect(goToNamed(source(), ["Pri"]).map((r) => r.name)).toEqual(["Primo Colon", "Prince Nana", "Printer Bot"]);
  });

  it("reads a name one way for Go to and for Insights' questions", () => {
    expect(rankName("Print Lab", "print lab")).toBe(0);
    expect(rankName("Print Lab", "pri")).toBe(1);
    expect(rankName("Damian Priest", "pri")).toBe(2);
    expect(rankName("Damian Priest", "x")).toBeNull();
    expect(rankName("Élan Vital", "elan")).toBe(1);
  });
});
