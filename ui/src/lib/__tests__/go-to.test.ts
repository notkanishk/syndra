import { describe, expect, it } from "vitest";

import { flatItems, fold, goTo, type GoToSource } from "@/lib/go-to";
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
