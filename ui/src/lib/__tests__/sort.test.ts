import { describe, expect, it } from "vitest";

import { compareNames, sortAccounts, sortByName, sortNames } from "@/lib/sort";

describe("sortByName", () => {
  it("ignores case: a lower-case name sits among the rest, not after Z", () => {
    expect(sortNames(["mkauth", "Kanishk Singh", "claude code", "Admin Makerspace", "netbird"])).toEqual([
      "Admin Makerspace",
      "claude code",
      "Kanishk Singh",
      "mkauth",
      "netbird",
    ]);
  });

  it("ignores accents: É sorts with E, not after z", () => {
    expect(sortNames(["Zoë", "Émile", "Eva", "Alia"])).toEqual(["Alia", "Émile", "Eva", "Zoë"]);
  });

  it("reads numbers as numbers: Lab 2 before Lab 10", () => {
    expect(sortNames(["Lab 10", "Lab 2", "Lab 1"])).toEqual(["Lab 1", "Lab 2", "Lab 10"]);
  });

  it("breaks a tie on the id, whatever order the ties arrived in", () => {
    const a = [{ id: "b", name: "Lab" }, { id: "a", name: "lab" }, { id: "c", name: "Annex" }];
    const b = [a[1], a[2], a[0]];
    expect(sortByName(a, (x) => x.name).map((x) => x.id)).toEqual(["c", "a", "b"]);
    expect(sortByName(b, (x) => x.name).map((x) => x.id)).toEqual(["c", "a", "b"]);
  });

  it("takes a custom id for rows that carry none", () => {
    const rows = [{ key: "z", label: "Same" }, { key: "a", label: "Same" }];
    expect(sortByName(rows, (r) => r.label, (r) => r.key).map((r) => r.key)).toEqual(["a", "z"]);
  });

  it("never mutates what it was given", () => {
    const names = ["b", "a"];
    sortNames(names);
    expect(names).toEqual(["b", "a"]);
  });

  it("compareNames treats case and accent variants as equal", () => {
    expect(compareNames("ada", "ADA")).toBe(0);
    expect(compareNames("resume", "résumé")).toBe(0);
  });
});

describe("sortAccounts", () => {
  it("puts people A to Z, then service accounts A to Z", () => {
    const accounts = [
      { id: "1", name: "netbird", service_account: true },
      { id: "2", name: "Kanishk Singh" },
      { id: "3", name: "claude code", service_account: true },
      { id: "4", name: "Admin Makerspace", service_account: false },
      { id: "5", name: "Alia Ahuja" },
    ];
    expect(sortAccounts(accounts).map((a) => a.name)).toEqual([
      "Admin Makerspace",
      "Alia Ahuja",
      "Kanishk Singh",
      "claude code",
      "netbird",
    ]);
  });
});
