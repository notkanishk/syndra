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

describe("an automatic retry, worded by how it went", () => {
  it("says it was sent again when something went out", () => {
    expect(describeAction("propagation.retried", nameOf, { outcome: "retrying" }).verb).toMatch(/^Sent a change again on its own/);
  });

  it("says nothing went out, and Syndra will try again, when it was not sent", () => {
    const said = describeAction("propagation.retried", nameOf, { outcome: "not_sent", tries_spent: 1 });
    for (const words of [said.verb, said.template, said.passive]) {
      expect(words).toMatch(/nothing went out/);
      expect(words).toMatch(/Syndra will try again/);
      expect(words).not.toMatch(/\bSent\b|sent again/i);
    }
  });
});
