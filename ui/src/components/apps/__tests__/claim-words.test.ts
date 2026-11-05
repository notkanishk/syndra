import { describe, expect, it } from "vitest";

import { check5, checkWords, heldBackLine, listingNotice, type CheckContext } from "@/components/apps/claim-words";
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
    expect(checkWords(e, ctx)[2].faint).toEqual(["Syndra builds them at their next change. Until then it sends only what Zitadel already knows about them."]);
    expect(check5(e, ctx, false).sentence).toBe("Syndra answers with `groups` for now.");
  });

  it("never reads an unread grants list as no roles", () => {
    const e = explanation(["pass", "stale", "pass", "pass", "pass", "none"], [{}, { roles: [], unread: true }]);
    const c2 = checkWords(e, ctx)[1];
    expect(c2.sentence).toBe("Syndra couldn't read Ada's roles from Zitadel just now.");
    expect(c2.tone).toBe("warn");
  });
});

describe("before Actions v1 goes off", () => {
  it("says a person Zitadel lists no project for gets nothing, as the token does", () => {
    const e = explanation(["pass", "none", "none", "pass", "none", "pass"], [{}, { roles: [] }, {}, {}, { unlisted: true }]);
    const c5 = check5(e, ctx, false);
    expect(c5.sentence).toBe("Zitadel doesn't list Network in Ada's token, so Syndra adds nothing.");
    expect(c5.tone).toBe("neutral");
  });

  it("says when Zitadel names no app, so the project default goes out instead", () => {
    const named = explanation(["pass", "pass", "pass", "pass", "pass", "pass"], [{}, {}, {}, { source: "app", version: 2 }]);
    expect(checkWords(named, ctx)[3].sentence).toBe("What NetBird is told applies, version 2.");

    const unnamed = explanation(["pass", "pass", "pass", "warn", "pass", "pass"], [
      {},
      {},
      {},
      { source: "app", version: 2, client_id_missing: ["function/preaccesstoken"] },
    ]);
    const c4 = checkWords(unnamed, ctx)[3];
    expect(c4.tone).toBe("warn");
    expect(c4.sentence).toBe("Zitadel doesn't say which app is signing in, so NetBird gets Network's default, not this shape.");
    expect(c4.faint?.[0]).toContain("the access token");
    expect(c4.faint?.[0]).toContain("version 4.0");
  });

  describe("the project-not-listed notice", () => {
    const links = { project: "https://zitadel.example.org/ui/console/projects/pNet" };

    it("shows when the roles aren't asserted and nothing was served lately, with the fix in Zitadel", () => {
      const e = explanation(["fail", "pass", "pass", "pass", "fail", "none"], [{ project_role_assertion: false }]);
      const n = listingNotice(e, { ...ctx, links });
      expect(n?.title).toBe("NetBird gets nothing from Syndra.");
      expect(n?.body).toContain('turn on "Assert roles on authentication" for Network');
      expect(n?.link?.href).toBe(links.project);
    });

    it("stays away when a recent call was served: the app asks for role scopes", () => {
      const e = explanation(["fail", "pass", "pass", "pass", "fail", "pass"], [{ project_role_assertion: false }]);
      expect(listingNotice(e, ctx)).toBeNull();
    });

    it("names the extra audience scope when the last call spanned projects", () => {
      const e = explanation(["pass", "pass", "pass", "pass", "pass", "warn"], [
        { project_role_assertion: true },
        {},
        {},
        {},
        {},
        { last_outcome: "refused_cross_project" },
      ]);
      expect(listingNotice(e, ctx)?.body).toContain("urn:zitadel:iam:org:project:id:…:aud");
    });

    it("stays away when Syndra could not read its record of calls: unread proves nothing", () => {
      const e = explanation(["fail", "pass", "pass", "pass", "fail", "stale"], [{ project_role_assertion: false }]);
      expect(listingNotice(e, ctx)).toBeNull();
    });

    it("says nothing when the roles are asserted, or the shape sends nothing of its own", () => {
      expect(listingNotice(explanation(["pass", "pass", "pass", "pass", "pass", "pass"], [{ project_role_assertion: true }]), ctx)).toBeNull();
      const signInOnly = { ...ctx, profile: { ...ctx.profile, claims: [] } };
      expect(listingNotice(explanation(["warn", "pass", "pass", "pass", "none", "none"], [{ project_role_assertion: false }]), signInOnly)).toBeNull();
      expect(listingNotice(undefined, ctx)).toBeNull();
    });
  });

  it("says people with access to the project are the ones it lists, not only people with a role", () => {
    const e = explanation(["pass", "none", "none", "pass", "none", "pass"], [{}, { roles: [] }, {}, {}, { unlisted: true }]);
    expect(check5(e, ctx, false).faint?.[0]).toBe("Zitadel lists Network only for people who have been given access to it.");
  });

  describe("what holds a saved shape back", () => {
    it("is the app keeping the project's default while Zitadel names no app", () => {
      const e = explanation(["pass", "pass", "pass", "warn", "pass", "pass"], [{}, {}, {}, { source: "app", client_id_missing: ["function/preuserinfo"] }]);
      expect(heldBackLine(e, ctx)).toBe("Zitadel doesn't say which app is signing in, so NetBird keeps getting Network's default until Zitadel is upgraded.");
    });

    it("is the app getting nothing while Zitadel lists no project", () => {
      const e = explanation(["fail", "pass", "pass", "pass", "fail", "none"], [{ project_role_assertion: false }]);
      expect(heldBackLine(e, ctx)).toContain("NetBird gets nothing from Syndra until that is fixed");
    });

    it("is nothing when the shape reaches the app, or nothing is known", () => {
      expect(heldBackLine(explanation(["pass", "pass", "pass", "pass", "pass", "pass"], [{ project_role_assertion: true }]), ctx)).toBeNull();
      expect(heldBackLine(undefined, ctx)).toBeNull();
    });
  });
});
