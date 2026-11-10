import { describe, expect, it } from "vitest";

import { holds, onItsWay, standingTone, standingWords, type RoleStanding } from "@/lib/standing";

const ALL: Array<RoleStanding | null> = ["held", "sent", "waiting", "removing", "not_held", null];

describe("standingWords", () => {
  it("says the operator's word for every standing", () => {
    expect(ALL.map((s) => standingWords(s, "operator"))).toEqual([
      "In Zitadel",
      "Sent, not seen back yet",
      "Not sent to Zitadel yet",
      "Being removed",
      "Not in Zitadel",
      "Couldn’t check Zitadel",
    ]);
  });

  it("says the member's word for every standing, and nothing for a held role", () => {
    expect(ALL.map((s) => standingWords(s, "member"))).toEqual([
      null,
      "On its way",
      "On its way",
      "Being removed",
      "Not there yet",
      "Couldn’t check",
    ]);
  });

  it("reads a missing standing as not checked", () => {
    expect(standingWords(undefined, "member")).toBe("Couldn’t check");
  });
});

describe("holds / onItsWay", () => {
  it("a role being removed is still held; sent and waiting are on their way, not held", () => {
    expect(ALL.filter(holds)).toEqual(["held", "removing"]);
    expect(ALL.filter(onItsWay)).toEqual(["sent", "waiting"]);
  });
});

describe("standingTone", () => {
  it("warns only when the reader is needed", () => {
    expect(standingTone("not_held")).toBe("warn");
    expect(standingTone("removing")).toBe("warn");
    for (const s of ["held", "sent", "waiting", null] as const) expect(standingTone(s)).toBe("quiet");
  });
});
