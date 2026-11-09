import { describe, expect, it } from "vitest";

import { check6, verdictOf, type CheckContext } from "@/components/apps/claim-words";
import type { StatusTone } from "@/components/ui/Badge";
import {
  ADD_MENU,
  addMuted,
  breakingLine,
  claimLines,
  compareKeys,
  diffLines,
  fromTemplate,
  literal,
  roleWords,
  rolesIndex,
  rolesState,
  rowProblems,
  sentAs,
  setRole,
  understandLine,
  namesRoles,
} from "@/lib/claims";
import type { Profile } from "@/lib/queries/useClaimShape";

const roles = [
  { key: "staff", label: "Staff", description: "" },
  { key: "admin", label: "Admins", description: "" },
  { key: "guest", label: "Guests", description: "" },
];

describe("the draft preview's lines", () => {
  it("marks a changed claim − then +, keeps the same as it was, and a gone one −", () => {
    const lines = diffLines(
      { groups: ["netbird-staff", "netbird-admins"], keep: true, gone: 1 },
      { groups: ["network-staff", "netbird-admins"], keep: true, joins: "x" },
    );
    expect(lines.map((l) => `${l.mark}${l.key}`)).toEqual(["−groups", "+groups", " keep", "+joins", "−gone"]);
    expect(lines[0].text).toBe('"groups": ["netbird-staff", "netbird-admins"]');
  });

  it("says nothing changed when the tokens match", () => {
    expect(diffLines({ roles: [] }, { roles: [] }).every((l) => l.mark === " ")).toBe(true);
  });

  it("reads a value on one line, objects included", () => {
    expect(literal({ custom_claims: { groups: ["a"], ok: true } })).toBe('{"custom_claims": {"groups": ["a"], "ok": true}}');
  });
});

describe("a draft's own row checks", () => {
  const p = (claims: Profile["claims"]): Profile => ({ claims, include_new: true, lands: "both" });

  it("refuses a repeated name and a leaf that is another row's parent", () => {
    const out = rowProblems(
      p([
        { path: ["groups"], kind: "roles" },
        { path: ["groups"], kind: "bundles" },
        { path: ["custom_claims"], kind: "attribute", source: "email" },
        { path: ["custom_claims", "groups"], kind: "roles" },
      ]),
      { project: "Network" },
    );
    expect(out[0]).toBeNull();
    expect(out[1]?.message).toBe("`groups` is sent by another row too.");
    expect(out[2]?.message).toBe("`custom_claims` is already an object here, holding `groups`.");
    expect(out.every((x) => !x || x.blocksPreview)).toBe(true);
  });

  it("says which a fixed value lacks, a name or a value, and lets a nameless one stop the preview", () => {
    const out = rowProblems(
      p([
        { path: [""], kind: "static", value: "" },
        { path: ["org"], kind: "static", value: "" },
        { path: ["tags"], kind: "static", value: [] },
        { path: ["flag"], kind: "static", value: false },
        { path: ["n"], kind: "static", value: 0 },
      ]),
      { project: "Network" },
    );
    expect(out.map((x) => x?.message ?? null)).toEqual(["Name it and give it a value.", "Give it a value.", "Give it a value.", null, null]);
    expect(out[0]?.blocksPreview).toBe(true);
    expect(out[1]?.blocksPreview).toBe(false);
  });

  it("refuses a top-level key Zitadel never lets an app set, as the shaper's Validate does", () => {
    const out = rowProblems(
      p([
        { path: ["sub"], kind: "static", value: "x" },
        { path: ["urn:zitadel:iam:org:id"], kind: "static", value: "x" },
        { path: ["custom_claims", "sub"], kind: "static", value: "x" },
        { path: ["email"], kind: "attribute", source: "email" },
      ]),
      { project: "Network" },
    );
    expect(out[0]?.message).toBe("`sub` is Zitadel's own claim, so Zitadel drops this one. Name it something else, or put it inside an object.");
    expect(out[1]?.blocksPreview).toBe(true);
    expect(out.slice(2)).toEqual([null, null]);
  });

  it("keeps a dotted name one key", () => {
    expect(rowProblems(p([{ path: ["syndra.laser.roles"], kind: "roles" }]), { project: "Laser" })).toEqual([null]);
  });

  it("names a copied role that isn't here, and lets the preview run without that row", () => {
    const [problem] = rowProblems(p([{ path: ["is_admin"], kind: "if_role", roles: [] }]), {
      project: "Audio",
      missing: { is_admin: "Admin" },
    });
    expect(problem).toEqual({ message: "Admin isn't in Audio. Choose a role from Audio, or remove this row.", blocksPreview: false });
  });
});

describe("roles and their words", () => {
  const profile: Profile = { claims: [], roles: [{ key: "staff", as: "netbird-staff", included: true }], include_new: false, lands: "both" };

  it("follows roles added later for a role the shape never mapped", () => {
    const w = roleWords(profile, roles);
    expect(w.map((x) => [x.role.key, x.included, x.added])).toEqual([
      ["staff", true, false],
      ["admin", false, true],
      ["guest", false, true],
    ]);
  });

  it("maps a role on its first edit, and an emptied word means Syndra's key", () => {
    const next = setRole(setRole(profile, "admin", { included: true }), "staff", { as: "" });
    expect(next.roles).toEqual([
      { key: "staff", included: true },
      { key: "admin", included: true },
    ]);
  });
});

describe("what NetBird will do", () => {
  const ctx = { app: "NetBird", roles, templateId: "netbird", reach: { by_role: { staff: 12, guest: 4 }, people: 23 } };
  const changes = [
    { kind: "rename" as const, role: "staff", from: "netbird-staff", to: "network-staff" },
    { kind: "remove" as const, role: "guest", from: "netbird-guests" },
  ];

  it("says the before-line with the template's consequence and the counts", () => {
    expect(breakingLine(changes, ctx)).toBe(
      "2 changes NetBird will act on. 12 people move from `netbird-staff` to a new group, `network-staff`, at their next sign-in. 4 people leave `netbird-guests`.",
    );
  });

  it("asks the I-understand line in those words, without counts when Zitadel was never read", () => {
    expect(understandLine(changes, ctx)).toBe("I understand NetBird will move 12 people to a new group.");
    expect(understandLine(changes, { ...ctx, reach: null })).toBe("I understand NetBird will move people to a new group.");
  });

  it("says a Custom app's rename plainly, with no app-specific consequence", () => {
    expect(breakingLine([{ kind: "rename", role: "staff", from: "stf", to: "staff" }], { app: "Inventory", roles, templateId: undefined })).toBe(
      "1 change Inventory will act on. Inventory will see `staff` instead of `stf`.",
    );
  });
});

describe("Sent as", () => {
  const token = { custom_claims: { groups: ["analyst"], given_name: "Charlotte" }, roles: [] };

  it("shows a nested row's whole object, and its sibling says it's inside the same one", () => {
    expect(sentAs({ path: ["custom_claims", "groups"], kind: "roles" }, token, "Charlotte Flair", { app: "Supabase" })).toBe(
      'Sent as `{"custom_claims": {"groups": ["analyst"], "given_name": "Charlotte"}}` for Charlotte Flair, with `"groups": []` for anyone who holds none.',
    );
    expect(sentAs({ path: ["custom_claims", "given_name"], kind: "attribute", source: "given_name" }, token, "Charlotte Flair", { app: "Supabase", sameObject: true })).toBe(
      'Sent as `"Charlotte"` for Charlotte Flair, inside the same object.',
    );
  });

  it("always sends a roles row, empty as []", () => {
    expect(sentAs({ path: ["roles"], kind: "roles" }, token, "Liv Morgan", { app: "Inventory" })).toBe(
      "Sent as `[]` for Liv Morgan, and `[]` for anyone who holds none.",
    );
  });
});

describe("the list of claims (A-16 §1–§2)", () => {
  const ctx = { app: "NetBird", project: "Network" };
  const base: Profile = { claims: [{ path: ["groups"], kind: "roles", format: "array" }], roles: [], include_new: true, lands: "both" };
  const mapped = (as: Record<string, string>, out: string[] = []): Profile => ({
    ...base,
    include_new: false,
    roles: roles.map((r) => ({ key: r.key, ...(as[r.key] ? { as: as[r.key] } : {}), included: !out.includes(r.key) })),
  });
  const state = (p: Profile, rs = roles) => rolesState(p, rs, ctx);

  it("says the roles' state on the roles line, naming up to two changes, A to Z by name", () => {
    expect(state(base)).toBe("All 3 roles, in Syndra's words.");
    expect(state(base, roles.slice(0, 2))).toBe("Both roles, in Syndra's words.");
    expect(state(base, roles.slice(0, 1))).toBe("Its one role, in Syndra's words.");
    expect(state(mapped({ staff: "s", admin: "a", guest: "g" }))).toBe("All 3 roles, in NetBird's words.");
    expect(state(mapped({ staff: "netbird-staff" }, ["guest"]))).toBe("2 of 3 roles. Staff as `netbird-staff`; Guests left out.");
    expect(state(mapped({ staff: "netbird-staff" }))).toBe("All 3 roles. Staff as `netbird-staff`.");
    expect(state(mapped({ staff: "s", admin: "a" }, ["guest"]))).toBe("2 of 3 roles. 2 renamed, 1 left out.");
    expect(state(mapped({}, ["staff", "admin", "guest"]))).toBe("None of the 3 roles. All left out.");
    // A word that is the key itself is Syndra's word.
    expect(state(mapped({ staff: "staff" }))).toBe("All 3 roles, in Syndra's words.");
  });

  it("puts the roles claim first and the rest A to Z, an object's keys A to Z too", () => {
    const p: Profile = {
      ...base,
      claims: [
        { path: ["zeta"], kind: "static", value: 1 },
        { path: ["custom_claims", "name"], kind: "attribute", source: "name" },
        { path: ["custom_claims", "groups"], kind: "roles" },
        { path: ["custom_claims", "given_name"], kind: "attribute", source: "given_name" },
        { path: ["Is_admin"], kind: "if_role", roles: ["admin", "staff"] },
        { path: ["groups"], kind: "roles" },
      ],
    };
    const lines = claimLines(p, roles, ctx);
    expect(lines.map((l) => `${l.key}: ${l.words}`)).toEqual([
      "groups: The roles each person holds in Network, as a list",
      "custom_claims: An object holding `given_name`, `groups` and `name`",
      "Is_admin: `true` if they hold Admins or Staff, otherwise `false`",
      "zeta: The same for everyone: `1`",
    ]);
    expect(lines[0]).toMatchObject({ roles: true, sub: "All 3 roles, in Syndra's words.", rows: [5] });
    expect(lines[1]).toMatchObject({ object: true, sub: "Its `groups` is the same list of roles.", rows: [1, 2, 3] });
    expect(lines.slice(1).every((l) => !l.roles)).toBe(true);
  });

  it("makes a nested roles claim's object the roles line, and with none every line is A to Z", () => {
    const nested = claimLines({ ...base, claims: [{ path: ["email"], kind: "attribute", source: "email" }, { path: ["custom_claims", "groups"], kind: "roles" }] }, roles, ctx);
    expect(nested.map((l) => [l.key, l.roles])).toEqual([["custom_claims", true], ["email", false]]);
    const none = claimLines({ ...base, claims: [{ path: ["b"], kind: "bundles" }, { path: ["a"], kind: "role_ends" }] }, roles, ctx);
    expect(none.map((l) => l.key)).toEqual(["a", "b"]);
    expect(rolesIndex({ ...base, claims: [{ path: ["a"], kind: "bundles" }] })).toBe(-1);
  });

  it("marks a line the saved shape lacks as new, and keeps a text-format warning on its line", () => {
    const draft: Profile = { ...base, claims: [...base.claims, { path: ["given_name"], kind: "attribute", source: "given_name" }] };
    expect(claimLines(draft, roles, { ...ctx, saved: base })[1].sub).toBe("New in this draft.");
    expect(claimLines(draft, roles, ctx)[1].sub).toBeUndefined();
    const text = claimLines({ ...base, claims: [{ path: ["custom_claims", "groups"], kind: "roles", format: "csv" }] }, roles, ctx);
    expect(text[0].warn).toBe("As text, with commas. No app Syndra has a template for reads roles as text.");
    expect(claimLines(base, roles, ctx)[0].warn).toBeUndefined();
  });

  it("orders keys as the shaper does: case aside, then byte order", () => {
    expect(["zeta", "Beta", "alpha", "beta"].sort(compareKeys)).toEqual(["alpha", "Beta", "beta", "zeta"]);
  });

  it("names the template and the changes since in one line", () => {
    expect(fromTemplate("Custom", 5)).toBe("From the Custom template, 5 changes since.");
    expect(fromTemplate("NetBird", 1)).toBe("From the NetBird template, 1 change since.");
    expect(fromTemplate("NetBird", 0)).toBe("From the NetBird template.");
  });
});

describe("the add menu (A-16 §3)", () => {
  const entry = (label: string) => ADD_MENU.flatMap((g) => g.entries).find((e) => e.label === label)!;

  it("greys Zitadel's own keys at the top level only, saying so", () => {
    for (const [label, key] of [["Email", "email"], ["Full name", "name"], ["Username", "preferred_username"]]) {
      expect(entry(label).key).toBe(key);
      expect(addMuted(entry(label), new Set(), { inside: false })).toBe(`Zitadel sends \`${key}\``);
      expect(addMuted(entry(label), new Set(), { inside: true })).toBeUndefined();
    }
    // given_name and family_name reach the access token, which Zitadel doesn't fill with them.
    expect(addMuted(entry("Given name"), new Set(), { inside: false })).toBeUndefined();
  });

  it("mutes a key already sent and a second roles claim, and names the roles claim groups by default", () => {
    expect(addMuted(entry("Given name"), new Set(["given_name"]), { inside: false })).toBe("Sent as `given_name`");
    expect(addMuted(entry("Roles {app} sees"), new Set(["roles"]), { inside: false, rolesKey: "roles" })).toBe("Sent as `roles`");
    expect(entry("Roles {app} sees")).toMatchObject({ key: "groups", claim: { kind: "roles", format: "array" } });
    expect(addMuted(entry("Roles {app} sees"), new Set(), { inside: true, rolesKey: "groups" })).toBeUndefined();
    expect(addMuted(entry("Yes if they hold a role"), new Set(["x"]), { inside: false })).toBeUndefined();
  });

  it("lists every group A to Z", () => {
    for (const g of ADD_MENU) {
      const labels = g.entries.map((e) => e.label);
      expect(labels).toEqual([...labels].sort((a, b) => a.localeCompare(b)));
    }
  });
});

describe("the preview's verdict (A-15 §5)", () => {
  const t = (...tones: StatusTone[]) => tones.map((tone) => ({ tone }));
  it("says what passed, failed or needs a look, on the draft when there is one", () => {
    expect(verdictOf(t("healthy", "healthy", "neutral", "healthy", "healthy"), false)).toEqual({ tone: "healthy", text: "All five checks pass" });
    expect(verdictOf(t("healthy", "healthy", "healthy", "healthy", "healthy"), true).text).toBe("All five checks pass on this draft");
    expect(verdictOf(t("danger", "healthy", "healthy", "healthy", "healthy"), false)).toEqual({ tone: "danger", text: "Check 1 fails" });
    expect(verdictOf(t("danger", "healthy", "healthy", "warn", "danger"), false)).toEqual({ tone: "danger", text: "Checks 1 and 5 fail" });
    expect(verdictOf(t("healthy", "healthy", "healthy", "warn", "healthy"), false)).toEqual({ tone: "warn", text: "Check 4 needs a look" });
    expect(verdictOf(t("healthy", "healthy", "healthy", "healthy", "healthy", "healthy"), false).text).toBe("All six checks pass");
    expect(verdictOf(t("healthy", "healthy", "healthy", "healthy", "healthy", "danger"), false).text).toBe("Check 6 fails");
  });
});

describe("namesRoles", () => {
  it("is false for a shape that never chose roles, so no role reads as new", () => {
    expect(namesRoles(null)).toBe(false);
    expect(namesRoles({ include_new: true, roles: [] } as unknown as Profile)).toBe(false);
  });
  it("is true once the shape names a role of its own", () => {
    expect(namesRoles({ include_new: true, roles: [{ key: "staff", included: true }] } as unknown as Profile)).toBe(true);
  });
});

describe("check 6: Zitadel reached Syndra for this app recently (S3)", () => {
  const ctx = { app: "NetBird", project: "Network", person: "Charlotte", roles: [], profile: { claims: [], include_new: false, lands: "both" }, templates: [] } as unknown as CheckContext;
  const at = new Date().toISOString();
  const e = (state: string, facts: Record<string, unknown> = {}) =>
    ({ user_id: "charlotte", read_at: at, token: null, checks: [0, 1, 2, 3, 4].map(() => ({ state: "pass", facts: {} })).concat([{ state, facts }]) }) as never;

  it("passes when called in the last day and served", () => {
    const [c] = check6(e("pass", { last_call_at: at, last_outcome: "served", last_served_at: at }), ctx);
    expect(c.tone).toBe("healthy");
    expect(c.sentence).toBe("Zitadel reached Syndra for NetBird recently.");
    expect(c.faint?.[0]).toMatch(/^Last called at \d\d:\d\d today, and Syndra served its claims\.$/);
  });

  it("needs a look when the last answer carried no claims, and says why plainly", () => {
    const [c] = check6(e("warn", { last_call_at: at, last_outcome: "sig_mismatch" }), ctx);
    expect(c.tone).toBe("warn");
    expect(c.sentence).toBe("Zitadel reached Syndra for NetBird, but the last answer carried no claims.");
    expect(c.faint?.[0]).toMatch(/: signed with a key Syndra doesn't hold\.$/);
  });

  it("fails when people signed in after the last call", () => {
    const [c] = check6(e("fail", { last_call_at: "2026-11-01T08:00:00Z", last_sign_in_at: at }), ctx);
    expect(c.tone).toBe("danger");
    expect(c.sentence).toBe("Zitadel stopped reaching Syndra for NetBird.");
    expect(c.faint?.[0]).toMatch(/People signed in after it, the last at \d\d:\d\d today, and their tokens went out without Syndra's claims\./);
  });

  it("fails when Zitadel never called for it", () => {
    const [c] = check6(e("fail", { last_call_at: null, last_sign_in_at: at }), ctx);
    expect(c.sentence).toBe("Zitadel has never called Syndra for NetBird.");
  });

  it("has nothing to tell when nobody signed in lately", () => {
    const [c] = check6(e("none"), ctx);
    expect(c.tone).toBe("neutral");
    expect(c.sentence).toBe("Nobody has signed in to NetBird lately, so there is no call to look at.");
  });

  it("says it couldn't read the record, as a ring, when telemetry is unread", () => {
    const [c] = check6(e("stale", { unread: true }), ctx);
    expect(c).toMatchObject({ tone: "neutral", ring: true });
    expect(c.sentence).toBe("Syndra couldn't read its record of Zitadel's calls for NetBird.");
  });

  it("invents no sixth check for an answer that has only five", () => {
    expect(check6({ user_id: "charlotte", read_at: at, token: null, checks: [] } as never, ctx)).toEqual([]);
  });
});
