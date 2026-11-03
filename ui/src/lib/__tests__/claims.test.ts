import { describe, expect, it } from "vitest";

import {
  breakingLine,
  diffLines,
  literal,
  roleWords,
  rowProblems,
  sentAs,
  setRole,
  understandLine,
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
  const token = { custom_claims: { groups: ["analyst"], given_name: "Ada" }, roles: [] };

  it("shows a nested row's whole object, and its sibling says it's inside the same one", () => {
    expect(sentAs({ path: ["custom_claims", "groups"], kind: "roles" }, token, "Ada Lovelace", { app: "Supabase" })).toBe(
      'Sent as `{"custom_claims": {"groups": ["analyst"], "given_name": "Ada"}}` for Ada Lovelace, with `"groups": []` for anyone who holds none.',
    );
    expect(sentAs({ path: ["custom_claims", "given_name"], kind: "attribute", source: "given_name" }, token, "Ada Lovelace", { app: "Supabase", sameObject: true })).toBe(
      'Sent as `"Ada"` for Ada Lovelace, inside the same object.',
    );
  });

  it("always sends a roles row, empty as []", () => {
    expect(sentAs({ path: ["roles"], kind: "roles" }, token, "Rohan Mehta", { app: "Inventory" })).toBe(
      "Sent as `[]` for Rohan Mehta, and `[]` for anyone who holds none.",
    );
  });
});
