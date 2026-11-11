import { describe, expect, it } from "vitest";

import { TRUENAS_SCHEMA } from "@/lib/__fixtures__/truenas-schema";
import {
  accountField,
  fieldNoun,
  fieldsByRole,
  formatLimit,
  isLifecycleField,
  limitFields,
  limitWordsFor,
  mappingFields,
  roleOf,
  type EntitlementField,
} from "@/lib/fields";

// A door-access system: names that are nothing like TrueNAS's, same roles.
const ACCESS: EntitlementField[] = [
  { name: "policy", role: "membership", label: "Access policy" },
  { name: "active", role: "account" },
  { name: "after_hours", role: "channel", label: "After-hours entry" },
  { name: "guests", role: "limit", label: "Guests a day", unit: "count", unlimited: "-1" },
];

describe("fields are read by role", () => {
  it("groups a schema by role, every role present", () => {
    const by = fieldsByRole(ACCESS);
    expect(by.membership.map((f) => f.name)).toEqual(["policy"]);
    expect(by.channel.map((f) => f.name)).toEqual(["after_hours"]);
    expect(by.flag).toEqual([]);
    expect(fieldsByRole(undefined).account).toEqual([]);
  });

  it("finds the account whatever it is called", () => {
    expect(accountField(ACCESS)?.name).toBe("active");
    expect(accountField(TRUENAS_SCHEMA)?.name).toBe("enabled");
    expect(accountField([])).toBeUndefined();
  });

  it("lists limits and says which fields the resolver derives", () => {
    expect(limitFields(ACCESS).map((f) => f.name)).toEqual(["guests"]);
    expect(isLifecycleField(ACCESS, "active")).toBe(true);
    expect(isLifecycleField(ACCESS, "after_hours")).toBe(true);
    expect(isLifecycleField(ACCESS, "policy")).toBe(false);
    expect(isLifecycleField(ACCESS, "not_declared")).toBe(false);
    expect(roleOf(ACCESS, "guests")).toBe("limit");
  });

  it("finds a role mapping's fields without naming any", () => {
    expect(mappingFields(ACCESS)).toEqual({ membership: "policy", flag: undefined, limit: "guests" });
    expect(mappingFields(TRUENAS_SCHEMA)).toEqual({ membership: "group", flag: "home", limit: "home_quota" });
  });

  it("says a field in the add-on's label, lowercase, and a flag by what it gives", () => {
    expect(fieldNoun(ACCESS[0])).toBe("access policy");
    expect(fieldNoun(TRUENAS_SCHEMA[1])).toBe("home folder");
    expect(fieldNoun(undefined)).toBe("setting");
  });
});

describe("formatLimit", () => {
  it("says each unit in its own words", () => {
    expect(formatLimit("5368709120", "bytes")).toBe("5 GB");
    expect(formatLimit(1.5 * 1024 ** 3, "bytes")).toBe("1.5 GB");
    expect(formatLimit(10 * 1024 ** 2, "bytes")).toBe("10 MB");
    expect(formatLimit("512", "bytes")).toBe("512 bytes");
    expect(formatLimit(5400, "seconds")).toBe("90 minutes");
    expect(formatLimit(5461, "seconds")).toBe("5461 seconds");
    expect(formatLimit(7200, "seconds")).toBe("2 hours");
    expect(formatLimit(60, "seconds")).toBe("1 minute");
    expect(formatLimit(1, "items")).toBe("1 item");
    expect(formatLimit(3, "items")).toBe("3 items");
    expect(formatLimit(250, "grams")).toBe("250 g");
    expect(formatLimit(1500, "grams")).toBe("1.5 kg");
    expect(formatLimit(4, "count")).toBe("4");
  });

  it("returns what is not a number as it came", () => {
    expect(formatLimit("lots", "bytes")).toBe("lots");
  });

  it("says no limit for the field's own unlimited value", () => {
    const quota = TRUENAS_SCHEMA[2];
    expect(limitWordsFor(quota, "0")).toBe("no limit");
    expect(limitWordsFor(quota, undefined)).toBe("no limit");
    expect(limitWordsFor(quota, "21474836480")).toBe("20 GB");
    expect(limitWordsFor(ACCESS[3], "-1")).toBe("no limit");
    expect(limitWordsFor(ACCESS[3], "0")).toBe("0");
  });
});
