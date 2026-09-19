import { describe, expect, it } from "vitest";

import {
  EMPTY_DRIFT_FILTERS,
  applyDriftFilters,
  driftHref,
  driftRequest,
  hasAnyDriftFilter,
  originOf,
  parseDriftFilters,
} from "@/lib/drift-filters";
import type { DriftTriageItem } from "@/lib/queries/useDrift";

const NOW = new Date("2026-09-19T12:00:00Z");

function row(over: Partial<DriftTriageItem>): DriftTriageItem {
  return {
    id: "d1",
    target: "zitadel",
    user_id: "u1",
    project_id: "p1",
    role_keys: ["community"],
    drift_type: "target_only",
    detection_source: "reconciliation_sweep",
    detected_at: "2026-09-19T09:00:00Z",
    role_in_catalogue: true,
    role_catalogue_applies: true,
    user_is_service_account: false,
    other_items_for_user: 0,
    ...over,
  } as DriftTriageItem;
}

describe("parseDriftFilters", () => {
  // A hand-edited or stale URL must degrade to "no filter", never to an empty
  // list that reads as "nothing matches".
  it("ignores values it does not recognise", () => {
    const filters = parseDriftFilters(new URLSearchParams("origin=whatever&age=someday"));
    expect(filters.origin).toBe("");
    expect(filters.age).toBe("");
  });

  it("reads every filter out of the query string", () => {
    const filters = parseDriftFilters(
      new URLSearchParams("project=p1&user=u1&source=webhook&origin=named&role=community&age=week"),
    );
    expect(filters).toEqual({
      project: "p1",
      user: "u1",
      source: "webhook",
      origin: "named",
      role: "community",
      age: "week",
    });
  });
});

// Two go to the backend and four are applied here. Sending one of the
// client-side four as a request param would silently mean "of whatever the
// server chose to return", which is a worse lie than not offering it.
describe("driftRequest", () => {
  it("sends only what the endpoint actually filters on", () => {
    const req = driftRequest({
      project: "p1",
      user: "u1",
      source: "webhook",
      origin: "named",
      role: "community",
      age: "week",
    });
    expect(req).toEqual({ project_id: "p1", source: "webhook" });
  });

  // The endpoint DOES accept user_id, and asking it is the bug: the person
  // dropdown is built from the people in the queue, so a server-narrowed queue
  // leaves that list holding only the name already chosen.
  it("never asks the server to narrow to one person", () => {
    const req = driftRequest({ ...EMPTY_DRIFT_FILTERS, user: "u1" });
    expect(req).not.toHaveProperty("user_id");
  });

  it("omits an empty filter rather than sending a blank", () => {
    expect(driftRequest(EMPTY_DRIFT_FILTERS)).toEqual({
      project_id: undefined,
      source: undefined,
    });
  });
});

describe("originOf", () => {
  it("prefers a named actor over any weaker claim", () => {
    expect(originOf(row({ upstream_actor: "sai", event_possibly_missed: true }))).toBe("named");
  });

  it("tells a missing event apart from an unattributable one", () => {
    expect(originOf(row({ event_possibly_missed: true }))).toBe("missed");
    expect(originOf(row({ attribution_unavailable: true }))).toBe("unattributable");
  });

  it("claims nothing when nothing is known", () => {
    expect(originOf(row({}))).toBeNull();
  });
});

describe("applyDriftFilters", () => {
  it("narrows by origin", () => {
    const rows = [row({ id: "a", upstream_actor: "sai" }), row({ id: "b" })];
    expect(applyDriftFilters(rows, { ...EMPTY_DRIFT_FILTERS, origin: "named" }, NOW)).toHaveLength(1);
  });

  it("narrows by role key", () => {
    const rows = [row({ id: "a", role_keys: ["community"] }), row({ id: "b", role_keys: ["kiln"] })];
    const kept = applyDriftFilters(rows, { ...EMPTY_DRIFT_FILTERS, role: "kiln" }, NOW);
    expect(kept.map((r) => r.id)).toEqual(["b"]);
  });

  it("narrows by age, by local day for today", () => {
    const rows = [
      row({ id: "today", detected_at: "2026-09-19T09:00:00Z" }),
      row({ id: "week", detected_at: "2026-09-15T09:00:00Z" }),
      row({ id: "old", detected_at: "2026-08-01T09:00:00Z" }),
    ];
    const ids = (age: "today" | "week" | "month") =>
      applyDriftFilters(rows, { ...EMPTY_DRIFT_FILTERS, age }, NOW).map((r) => r.id);
    expect(ids("today")).toEqual(["today"]);
    expect(ids("week")).toEqual(["today", "week"]);
    expect(ids("month")).toEqual(["old"]);
  });

  it("keeps everything when nothing is set", () => {
    const rows = [row({ id: "a" }), row({ id: "b" })];
    expect(applyDriftFilters(rows, EMPTY_DRIFT_FILTERS, NOW)).toHaveLength(2);
  });

  // A row with an unparseable date must not vanish from an unfiltered queue,
  // and must not be claimed as matching an age it cannot be checked against.
  it("drops an undateable row only when age is actually being asked", () => {
    const rows = [row({ id: "bad", detected_at: "not-a-date" })];
    expect(applyDriftFilters(rows, EMPTY_DRIFT_FILTERS, NOW)).toHaveLength(1);
    expect(applyDriftFilters(rows, { ...EMPTY_DRIFT_FILTERS, age: "today" }, NOW)).toHaveLength(0);
  });
});

// The bug this `except` argument exists to prevent: a dropdown whose options
// are narrowed by the value already selected offers exactly that one value,
// and the only way back is the "Any" row.
describe("options for a filter's own control", () => {
  const rows = [
    row({ id: "a", user_id: "u1", role_keys: ["community"] }),
    row({ id: "b", user_id: "u2", role_keys: ["kiln"] }),
  ];

  it("keeps every person in hand once one is chosen", () => {
    const chosen = { ...EMPTY_DRIFT_FILTERS, user: "u1" };
    expect(applyDriftFilters(rows, chosen, NOW).map((r) => r.id)).toEqual(["a"]);
    // ...but the list the CONTROL is built from still holds both.
    const forControl = applyDriftFilters(rows, chosen, NOW, "user");
    expect(forControl.map((r) => r.user_id)).toEqual(["u1", "u2"]);
  });

  it("still applies the other filters to those options", () => {
    const chosen = { ...EMPTY_DRIFT_FILTERS, user: "u1", role: "kiln" };
    const forControl = applyDriftFilters(rows, chosen, NOW, "user");
    expect(forControl.map((r) => r.user_id)).toEqual(["u2"]);
  });
});

describe("driftHref", () => {
  it("builds a link another screen can send somebody", () => {
    expect(driftHref({ project: "p1" })).toBe("/governance/drift?project=p1");
    expect(driftHref({})).toBe("/governance/drift");
  });
});

describe("hasAnyDriftFilter", () => {
  it("is false only when nothing is set", () => {
    expect(hasAnyDriftFilter(EMPTY_DRIFT_FILTERS)).toBe(false);
    expect(hasAnyDriftFilter({ ...EMPTY_DRIFT_FILTERS, age: "today" })).toBe(true);
  });
});
