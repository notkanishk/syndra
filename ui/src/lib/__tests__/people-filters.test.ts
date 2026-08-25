import { describe, expect, it } from "vitest";

import {
  applyFilters,
  describeFilters,
  EMPTY_FILTERS,
  hasAnyFilter,
  parseFilters,
  peopleHref,
  serializeFilters,
} from "@/lib/people-filters";
import type { UserListEntry } from "@/lib/queries/useUsers";

function person(overrides: Partial<UserListEntry> = {}): UserListEntry {
  return {
    user: {
      id: "u1",
      name: "Ada Lovelace",
      email: "ada@example.edu",
      title: "",
      team: "",
      status: "active",
      avatar: "AL",
    },
    bundle_count: 0,
    bundle_names: [],
    effective_role_count: 3,
    project_count: 1,
    key_projects: ["Laser Lab"],
    key_project_ids: ["pLaser"],
    expiring_count: 0,
    open_request_count: 0,
    unexplained_count: 0,
    ...overrides,
  } as UserListEntry;
}

describe("parseFilters", () => {
  it("ignores an attention value it doesn't recognise", () => {
    // A hand-edited or stale URL must degrade to "no filter", never to an
    // empty list that reads as "nobody matches".
    const filters = parseFilters(new URLSearchParams("attention=everything"));
    expect(filters.attention).toBe("");
  });

  it("reads every filter out of the query string", () => {
    const filters = parseFilters(
      new URLSearchParams("q=ada&project=pLaser&role=trained&bundle=Safety&attention=expiring"),
    );
    expect(filters).toEqual({
      q: "ada",
      project: "pLaser",
      role: "trained",
      bundle: "Safety",
      version: "",
      attention: "expiring",
    });
  });
});

describe("serializeFilters", () => {
  it("omits empty values so a cleared filter leaves no trace", () => {
    expect(serializeFilters({ ...EMPTY_FILTERS, q: "ada" })).toBe("?q=ada");
    expect(serializeFilters(EMPTY_FILTERS)).toBe("");
  });

  it("round-trips through parseFilters", () => {
    const original = {
      q: "ada",
      project: "pLaser",
      role: "trained",
      bundle: "Safety",
      version: "2",
      attention: "expiring" as const,
    };
    const query = serializeFilters(original);
    expect(parseFilters(new URLSearchParams(query.slice(1)))).toEqual(original);
  });

  it("carries extras like bulk mode alongside the filters", () => {
    expect(peopleHref({ project: "pLaser", role: "trained" }, { bulk: "1" })).toBe(
      "/users?project=pLaser&role=trained&bulk=1",
    );
  });
});

describe("applyFilters", () => {
  it("matches projects by id, so a rename doesn't break a saved link", () => {
    const rows = [person(), person({ key_project_ids: ["pWood"], key_projects: ["Wood Shop"] })];
    expect(applyFilters(rows, { ...EMPTY_FILTERS, project: "pLaser" })).toHaveLength(1);
    // The display name is deliberately NOT a match key.
    expect(applyFilters(rows, { ...EMPTY_FILTERS, project: "Laser Lab" })).toHaveLength(0);
  });

  it("leaves rows alone while role membership is still loading", () => {
    const rows = [person(), person({ user: { ...person().user, id: "u2" } })];
    // null means "not loaded". Filtering to nothing would claim nobody holds
    // the role, which is a different and much worse statement.
    expect(applyFilters(rows, { ...EMPTY_FILTERS, role: "trained" }, null)).toHaveLength(2);
    expect(applyFilters(rows, { ...EMPTY_FILTERS, role: "trained" }, new Set(["u1"]))).toHaveLength(1);
  });

  it("does not re-apply the text query the server already ran", () => {
    // Double-filtering with different matching rules would make the header
    // count disagree with the list underneath it.
    const rows = [person()];
    expect(applyFilters(rows, { ...EMPTY_FILTERS, q: "nothing matches this" })).toHaveLength(1);
  });

  it("treats 'departed' as work only when roles are still held", () => {
    const rows = [
      person({ user: { ...person().user, status: "departed" }, effective_role_count: 2 }),
      person({ user: { ...person().user, id: "u2", status: "departed" }, effective_role_count: 0 }),
    ];
    const kept = applyFilters(rows, { ...EMPTY_FILTERS, attention: "departed" });
    expect(kept).toHaveLength(1);
    expect(kept[0].user.id).toBe("u1");
  });

  it("finds people with no access at all", () => {
    const rows = [person({ effective_role_count: 0 }), person({ user: { ...person().user, id: "u2" } })];
    const kept = applyFilters(rows, { ...EMPTY_FILTERS, attention: "no-access" });
    expect(kept).toHaveLength(1);
    expect(kept[0].user.id).toBe("u1");
  });

  it("stacks filters rather than picking one", () => {
    const rows = [
      person({ expiring_count: 1 }),
      person({ user: { ...person().user, id: "u2" }, key_project_ids: ["pWood"] , expiring_count: 1 }),
      person({ user: { ...person().user, id: "u3" } }),
    ];
    const kept = applyFilters(rows, { ...EMPTY_FILTERS, project: "pLaser", attention: "expiring" });
    expect(kept.map((entry) => entry.user.id)).toEqual(["u1"]);
  });
});

describe("describeFilters", () => {
  it("names the filter so a bulk action can state its own scope", () => {
    // A bar that says "all 214 selected" without naming the filter asks an
    // operator to trust a number they cannot check.
    expect(describeFilters({ ...EMPTY_FILTERS, project: "pLaser" }, "Laser Lab")).toBe("in Laser Lab");
    expect(
      describeFilters({ ...EMPTY_FILTERS, q: "ada", attention: "expiring" }),
    ).toBe("matching “ada” and with access expiring");
  });

  it("says nothing when nothing is filtered", () => {
    expect(describeFilters(EMPTY_FILTERS)).toBe("");
    expect(hasAnyFilter(EMPTY_FILTERS)).toBe(false);
  });
});

describe("the version dimension", () => {
  it("narrows a bundle to the people on one version of it", () => {
    const onV2 = person({
      user: { ...person().user, id: "u1" },
      bundle_names: ["Safety"],
      bundle_versions: { Safety: 2 },
    });
    const onV4 = person({
      user: { ...person().user, id: "u2" },
      bundle_names: ["Safety"],
      bundle_versions: { Safety: 4 },
    });

    const both = applyFilters([onV2, onV4], { ...EMPTY_FILTERS, bundle: "Safety" });
    expect(both).toHaveLength(2);

    const stragglers = applyFilters([onV2, onV4], {
      ...EMPTY_FILTERS,
      bundle: "Safety",
      version: "2",
    });
    expect(stragglers.map((r) => r.user.id)).toEqual(["u1"]);
  });

  // A version with no bundle spans unrelated bundles and answers nothing, so a
  // hand-edited URL degrades to the bundle-less view rather than an empty one.
  it("ignores a version with no bundle", () => {
    expect(parseFilters(new URLSearchParams("version=2")).version).toBe("");
  });

  it("says which version it narrowed to", () => {
    expect(describeFilters({ ...EMPTY_FILTERS, bundle: "Safety", version: "2" })).toBe(
      "on v2 of the Safety bundle",
    );
    expect(describeFilters({ ...EMPTY_FILTERS, bundle: "Safety" })).toBe("in the Safety bundle");
  });
});
