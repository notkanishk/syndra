import { describe, expect, it } from "vitest";

import { describeAction } from "@/lib/audit-vocabulary";

/**
 * `revocation.landed` (design §13.4) is worded by the system its details name: Zitadel's is a role
 * that no longer signs them in, an add-on's is access on that system, named. Never "Revoked access",
 * because a replace that only took something away lands under the same line.
 */
const nameOf = (target: string) => ({ truenas: "TrueNAS" })[target] ?? target;

describe("a withdrawal that reached its system", () => {
  it("names the add-on's system it landed on", () => {
    const said = describeAction("revocation.landed", nameOf, { target: "truenas", recorded_at: "x", landed_at: "y" });
    expect(said.verb).toBe("A revocation reached TrueNAS: it is in force there now");
    expect(said.template).toContain("{}");
  });

  it("says sign-in for Zitadel", () => {
    expect(describeAction("revocation.landed", nameOf, { target: "zitadel" }).verb).toMatch(/the role no longer signs them in/);
  });

  it.each([["truenas"], ["zitadel"]])("never says Revoked access (%s)", (target) => {
    const said = describeAction("revocation.landed", nameOf, { target });
    for (const words of [said.verb, said.template, said.passive]) expect(words).not.toMatch(/Revoked access/i);
  });
});
