import { describe, expect, it } from "vitest";

import { check5, checkWords, type CheckContext } from "@/components/apps/claim-words";
import type { CheckState, Explanation } from "@/lib/queries/useClaimShape";

const ctx: CheckContext = {
  app: "NetBird",
  project: "Network",
  person: "Ada",
  roles: [],
  profile: { claims: [{ path: ["groups"], kind: "roles" }], include_new: true, lands: "both" },
  templates: [],
};

function explanation(states: CheckState[], facts: Record<string, unknown>[] = []): Explanation {
  return {
    user_id: "ada",
    checks: states.map((state, i) => ({ state, facts: facts[i] ?? {} })),
    token: { groups: ["netbird-staff"] },
    read_at: new Date().toISOString(),
  };
}

describe("claim words, with a token's roles from Zitadel (ADR-004)", () => {
  it("says a person not built yet still gets their roles", () => {
    const e = explanation(["pass", "pass", "warn", "pass", "warn", "none"], [{}, { roles: ["staff"] }]);
    expect(checkWords(e, ctx)[2].faint).toEqual(["Syndra builds them at their next change. Until then Zitadel gets their roles from Syndra, and nothing else."]);
    expect(check5(e, ctx, false).sentence).toBe("Syndra answers with `groups`, their roles alone for now.");
  });

  it("never reads an unread grants list as no roles", () => {
    const e = explanation(["pass", "stale", "pass", "pass", "pass", "none"], [{}, { roles: [], unread: true }]);
    const c2 = checkWords(e, ctx)[1];
    expect(c2.sentence).toBe("Syndra couldn't read Ada's roles from Zitadel just now.");
    expect(c2.tone).toBe("warn");
  });
});
