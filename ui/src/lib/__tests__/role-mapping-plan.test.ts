import { describe, expect, it } from "vitest";

import { bytesToGb, gbToBytes, homeCount, limitWords, mergePlans, opsFor, rowsOf } from "@/lib/role-mapping-plan";
import type { MappingRehearsal, RoleMapping } from "@/lib/queries/useMappings";

const row = (field: string, value: string): RoleMapping =>
  ({ id: `m-${field}`, target: "truenas", project_id: "p", role_key: "lead", field, value, created_by: "op" }) as RoleMapping;

const plan = (outcomes: Array<[string, string, string]>, planId?: string): MappingRehearsal =>
  ({
    op: "create_mapping",
    plan_id: planId,
    applied: false,
    outcomes: outcomes.map(([user_id, effect, detail]) => ({ user_id, name: user_id, email: "", effect, detail })),
    summary: { total: outcomes.length, apply: 0, no_change: 0, blocked: 0, failed: 0, succeeded: 0, queued: 0 },
  }) as MappingRehearsal;

describe("what a role change needs", () => {
  it("creates a role's group, home and limit together", () => {
    const ops = opsFor({}, { group: "fab", home: true, quota: "21474836480" });
    expect(ops.map((o) => `${o.kind} ${o.field}`)).toEqual(["create group", "create home", "create home_quota"]);
  });

  it("changes only what differs, and takes a home away with its limit", () => {
    const current = rowsOf([row("group", "fab"), row("home", "true"), row("home_quota", "1")], "p", "lead");
    expect(opsFor(current, { group: "fab", home: true, quota: "1" })).toEqual([]);
    expect(opsFor(current, { group: "fab", home: true, quota: "0" }).map((o) => `${o.kind} ${o.field}`)).toEqual(["edit home_quota"]);
    expect(opsFor(current, { group: "fab", home: false, quota: null }).map((o) => `${o.kind} ${o.field}`)).toEqual([
      "delete home",
      "delete home_quota",
    ]);
  });

  it("removes every row of a role when the role is removed", () => {
    const current = rowsOf([row("group", "fab"), row("home", "true")], "p", "lead");
    expect(opsFor(current, null).map((o) => `${o.kind} ${o.field}`)).toEqual(["delete group", "delete home"]);
  });
});

describe("one review from several rehearsals", () => {
  it("names each person once, with every change and the strongest effect", () => {
    const parts = [
      { op: { kind: "create" as const, field: "group", value: "fab" }, plan: plan([["u1", "apply", "joins fab"], ["u2", "no_change", ""]], "plan_g") },
      { op: { kind: "create" as const, field: "home", value: "true" }, plan: plan([["u1", "apply", "gets a home folder"], ["u2", "apply", "gets a home folder"], ["u3", "no_change", ""]], "plan_h") },
    ];
    const merged = mergePlans(parts);
    expect(merged.plan_id).toBe("plan_g");
    expect(merged.outcomes.map((o) => [o.user_id, o.effect, o.detail])).toEqual([
      ["u1", "apply", "joins fab; gets a home folder"],
      ["u2", "apply", "gets a home folder"],
      ["u3", "no_change", ""],
    ]);
    expect(merged.summary).toMatchObject({ total: 3, apply: 2, no_change: 1 });
    expect(homeCount(parts)).toEqual({ makes: 2, already: 1 });
  });
});

describe("limits in words", () => {
  it("reads GB the way the dialog takes it", () => {
    expect(gbToBytes("20")).toBe("21474836480");
    expect(bytesToGb("21474836480")).toBe("20");
    expect(limitWords("21474836480")).toBe("up to 20 GB");
    expect(limitWords("0")).toBe("no limit");
    expect(gbToBytes("0")).toBe("");
  });
});
