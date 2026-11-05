import type {
  Change,
  Claim,
  ClaimKind,
  Lands,
  Profile,
  ProjectRole,
  Reach,
  RoleMap,
} from "@/lib/queries/useClaimShape";
import { sortByName } from "@/lib/sort";

/**
 * Words and small reads for the claims part (A-14). Nothing here shapes a
 * token: every value shown comes from the backend's explain step. These only
 * say it, and say what a change will do. Sentences mark code with backticks;
 * `<Ticks>` draws them in mono.
 */

export const LANDS_LABEL: Record<Lands, string> = {
  both: "The access token and userinfo",
  access_token: "The access token",
  userinfo: "The ID token and userinfo",
};

export const READS_LABEL: Record<string, string> = {
  access_token: "the access token",
  id_token: "the ID token",
  userinfo: "userinfo",
};

/** `custom_claims` › `groups` as one string. */
export function pathLabel(path: string[]): string {
  return path.join(" › ");
}

/** A value as it reads on one line: `["a", "b"]`, `{"k": true}`. */
export function literal(value: unknown): string {
  if (Array.isArray(value)) return `[${value.map(literal).join(", ")}]`;
  if (value && typeof value === "object") {
    return `{${Object.entries(value as Record<string, unknown>)
      .map(([k, v]) => `${JSON.stringify(k)}: ${literal(v)}`)
      .join(", ")}}`;
  }
  return JSON.stringify(value ?? null);
}

/** The value a token holds at a claim's path, or undefined. */
export function valueAt(token: Record<string, unknown> | null | undefined, path: string[]): unknown {
  let at: unknown = token;
  for (const seg of path) {
    if (!at || typeof at !== "object" || Array.isArray(at)) return undefined;
    at = (at as Record<string, unknown>)[seg];
  }
  return at;
}

/** One top-level line of a token, as TokenBlock and Diff draw it. */
export interface TokenLine {
  key: string;
  text: string;
  mark: " " | "+" | "−";
}

export function tokenLines(token: Record<string, unknown> | null | undefined): TokenLine[] {
  return Object.entries(token ?? {}).map(([key, v]) => ({ key, text: `"${key}": ${literal(v)}`, mark: " " }));
}

/**
 * The draft preview's lines: each top-level key the same, or − what it was and
 * + what it becomes. The marks carry the meaning, so it never rests on colour.
 */
export function diffLines(
  now: Record<string, unknown> | null | undefined,
  draft: Record<string, unknown> | null | undefined,
): TokenLine[] {
  const was = new Map(tokenLines(now).map((l) => [l.key, l]));
  const out: TokenLine[] = [];
  const seen = new Set<string>();
  for (const line of tokenLines(draft)) {
    seen.add(line.key);
    const old = was.get(line.key);
    if (old && old.text === line.text) out.push(line);
    else {
      if (old) out.push({ ...old, mark: "−" });
      out.push({ ...line, mark: "+" });
    }
  }
  was.forEach((line, key) => {
    if (!seen.has(key)) out.push({ ...line, mark: "−" });
  });
  return out;
}

export function sameProfile(a: Profile | null | undefined, b: Profile | null | undefined): boolean {
  return JSON.stringify(a ?? null) === JSON.stringify(b ?? null);
}

export function cloneProfile(p: Profile): Profile {
  return JSON.parse(JSON.stringify(p)) as Profile;
}

// ---------------------------------------------------------------------------
// Roles: which this app sees, in its own words.

export interface RoleWord {
  role: ProjectRole;
  /** The word the app sees; "" means Syndra's key. */
  as: string;
  included: boolean;
  /** Not in the shape's map: a role added to the project later. */
  added: boolean;
}

export function roleWords(profile: Profile, roles: ProjectRole[]): RoleWord[] {
  const map = new Map((profile.roles ?? []).map((r) => [r.key, r]));
  return roles.map((role) => {
    const m = map.get(role.key);
    return m
      ? { role, as: m.as ?? "", included: m.included, added: false }
      : { role, as: "", included: profile.include_new, added: true };
  });
}

/**
 * Whether a role missing from a saved shape is worth calling "new". Only once
 * the shape names roles of its own: a shape that never chose any (the seeded
 * Actions v1 shape) would otherwise call every role new.
 */
export const namesRoles = (saved: Profile | null | undefined) => (saved?.roles?.length ?? 0) > 0;

/** The word a role is sent as: its own, or Syndra's key. */
export const wordOf = (r: { key: string; as?: string }) => r.as || r.key;

/** Writes one role's row into the map, mapping it if it was added later. */
export function setRole(profile: Profile, key: string, patch: Partial<RoleMap>): Profile {
  const roles = [...(profile.roles ?? [])];
  const i = roles.findIndex((r) => r.key === key);
  const base: RoleMap = i >= 0 ? roles[i] : { key, included: profile.include_new };
  const next = { ...base, ...patch };
  if (!next.as) delete next.as;
  if (i >= 0) roles[i] = next;
  else roles.push(next);
  return { ...profile, roles };
}

/** A role whose word another included role already uses ("Also Admins' word."). */
export function sharedWordOwner(words: RoleWord[], index: number): RoleWord | undefined {
  const me = words[index];
  if (!me.included) return undefined;
  const w = wordOf({ key: me.role.key, as: me.as });
  return words.slice(0, index).find((o) => o.included && wordOf({ key: o.role.key, as: o.as }) === w);
}

// ---------------------------------------------------------------------------
// Claim kinds, in the Carries select's groups (§4).

export const ATTRIBUTE_LABEL: Record<string, string> = {
  given_name: "Given name",
  family_name: "Family name",
  name: "Full name",
  username: "Username",
  email: "Email",
  is_service_account: "Is a service account",
  title: "Title",
  team: "Team",
  role_count: "How many roles they hold",
  compiled_at: "When their claims were built",
  user_id: "Their Zitadel id",
  project_id: "The project's id",
};

export const KIND_LABEL: Record<ClaimKind, string> = {
  roles: "Roles",
  attribute: "",
  static: "A fixed value",
  if_role: "Yes if they hold a role",
  bundles: "Bundles they hold",
  role_ends: "Each role's end date",
};

// ---------------------------------------------------------------------------
// What a change will do (§3, §7). One Diff from the backend feeds the draft
// preview's sentence, the before-line, the save summary and History.

export interface SayContext {
  app: string;
  roles: ProjectRole[];
  reach?: Reach | null;
  templateId?: string;
  /** Why a saved shape would not reach the app yet, when something outside it stops that. Said instead of "next sign-in". */
  heldBack?: string;
}

const labelOf = (roles: ProjectRole[], key?: string) => roles.find((r) => r.key === key)?.label ?? key ?? "";

function people(n: number | undefined): string {
  return n === undefined ? "People" : n === 1 ? "1 person" : `${n} people`;
}

/** History's and the summary's plain line for one change. */
export function changeLine(c: Change, ctx: SayContext): string {
  const role = labelOf(ctx.roles, c.role);
  const path = c.path ? `\`${pathLabel(c.path)}\`` : "";
  switch (c.kind) {
    case "rename":
      return `${role} is now \`${c.to}\`, was \`${c.from}\`.`;
    case "remove":
      return c.role ? `${role} is left out. It was sent as \`${c.from}\`.` : `${path} is no longer sent.`;
    case "add":
      return c.role ? `${role} is now sent, as \`${c.to}\`.` : `${path} is sent.`;
    case "retype":
      return `${path} is sent differently.`;
    case "move":
      return `${path} moves to \`${pathLabel(c.new_path ?? [])}\`.`;
    case "lands":
      return `The claims now land in ${(LANDS_LABEL[c.to as Lands] ?? c.to ?? "").toLowerCase()}.`;
    case "include_new":
      return c.to === "true" ? "Roles added later are included." : "Roles added later are left out.";
  }
}

/** What the app will do about one breaking change, per its template. */
export function consequence(c: Change, ctx: SayContext): string {
  const n = c.role ? ctx.reach?.by_role[c.role] : undefined;
  const who = people(n);
  if (!c.role) return `${ctx.app} no longer finds ${c.path ? `\`${pathLabel(c.path)}\`` : "its claims"} where it looks.`;
  switch (ctx.templateId) {
    case "netbird":
      return c.kind === "rename"
        ? `${who} move from \`${c.from}\` to a new group, \`${c.to}\`, at their next sign-in.`
        : `${who} leave \`${c.from}\`.`;
    case "gitlab":
      return `If \`gitlab.rb\` names \`${c.from}\` in \`admin_groups\`, every admin who signs in is demoted. If it's in \`required_groups\`, they can't sign in. Change \`gitlab.rb\` in the same step.`;
    case "home-assistant":
      return "New people get no admin until `roles.admin` matches.";
    case "supabase-oidc":
    case "supabase-keycloak":
      return "Row-level security that checks the old word stops matching.";
    default:
      return c.kind === "rename"
        ? `${ctx.app} will see \`${c.to}\` instead of \`${c.from}\`.`
        : `${ctx.app} no longer sees \`${c.from}\`.`;
  }
}

/** The before-line for breaking changes: "2 changes NetBird will act on. …" */
export function breakingLine(breaking: Change[], ctx: SayContext): string {
  const head = `${breaking.length === 1 ? "1 change" : `${breaking.length} changes`} ${ctx.app} will act on.`;
  return [head, ...breaking.map((c) => consequence(c, ctx))].join(" ");
}

/** The I-understand line: "I understand NetBird will move 12 people to a new group." */
export function understandLine(breaking: Change[], ctx: SayContext): string {
  const renamed = breaking.filter((c) => c.kind === "rename" && c.role);
  if (ctx.templateId === "netbird" && renamed.length > 0) {
    const n = renamed.reduce((sum, c) => sum + (ctx.reach?.by_role[c.role!] ?? 0), 0);
    return ctx.reach
      ? `I understand ${ctx.app} will move ${people(n).toLowerCase()} to a new group.`
      : `I understand ${ctx.app} will move people to a new group.`;
  }
  return `I understand ${ctx.app} will act on ${breaking.length === 1 ? "this change" : "these changes"}.`;
}

/** "From the Custom template, 5 changes since." (shape only: roles never count). */
export function fromTemplate(name: string, changes: number): string {
  return `From the ${name} template${changes === 0 ? "" : `, ${changes === 1 ? "1 change" : `${changes} changes`} since`}.`;
}

// ---------------------------------------------------------------------------
// The list of claims (A-16 §1): one line per top-level key, the key beside
// what it carries in words. The roles claim comes first, the rest A to Z, and
// A to Z inside an object. The shaper writes the token in the same order
// (`ordered` in internal/claims), so the list and the token read across.

/** A to Z for claim keys, case aside: byte for byte the shaper's `keyLess` (keys are ASCII). */
export function compareKeys(a: string, b: string): number {
  const x = a.toLowerCase();
  const y = b.toLowerCase();
  if (x !== y) return x < y ? -1 : 1;
  return a < b ? -1 : a > b ? 1 : 0;
}

/** The roles claim's row: the first top-level roles row, else the first nested one, else -1 (the shaper's `RolesKey`). */
export function rolesIndex(p: Profile): number {
  const top = p.claims.findIndex((c) => c.kind === "roles" && c.path.length === 1);
  return top >= 0 ? top : p.claims.findIndex((c) => c.kind === "roles");
}

export interface ClaimLine {
  key: string;
  /** The rows under this key, in saved order. The first is the line's id: it holds while the key is retyped. */
  rows: number[];
  words: string;
  /** The faint line: the roles' state, "New in this draft.", or what an object's roles row is. */
  sub?: string;
  /** A roles claim sent as text keeps its warning on its line, in both views. */
  warn?: string;
  /** The roles claim's line, first and the hero. */
  roles: boolean;
  object: boolean;
}

const joinWith = (xs: string[], word: string) =>
  xs.length <= 1 ? (xs[0] ?? "") : `${xs.slice(0, -1).join(", ")} ${word} ${xs[xs.length - 1]}`;

const TEXT_FORMAT: Record<string, string> = { csv: "As text, with commas", space_delimited: "As text, with spaces" };

/** What a roles claim's line says it carries. */
export const rolesCarry = (c: Claim, project: string) =>
  `The roles each person holds in ${project}, as ${(c.format ?? "array") === "array" ? "a list" : "text"}`;

/**
 * The roles line's faint line (A-16 §2): the state, so the common case never
 * needs opening. Named roles are the table's order, A to Z by name.
 */
export function rolesState(p: Profile, roles: ProjectRole[], ctx: { app: string }): string {
  const words = sortByName(roleWords(p, roles), (w) => w.role.label || w.role.key, (w) => w.role.key);
  const n = words.length;
  if (n === 0) return "No roles yet.";
  const renamed = words.filter((w) => w.included && w.as && w.as !== w.role.key);
  const out = words.filter((w) => !w.included);
  const all = n === 1 ? "Its one role" : n === 2 ? "Both roles" : `All ${n} roles`;
  if (out.length === 0 && renamed.length === 0) return `${all}, in Syndra's words.`;
  if (out.length === 0 && renamed.length === n) return `${all}, in ${ctx.app}'s words.`;
  if (out.length === n) return n === 1 ? "Its one role is left out." : `None of the ${n} roles. All left out.`;
  const head = out.length === 0 ? `${all}.` : `${n - out.length} of ${n} roles.`;
  const name = (w: RoleWord) => w.role.label || w.role.key;
  const tail =
    renamed.length + out.length <= 2
      ? [...renamed.map((w) => `${name(w)} as \`${w.as}\``), ...out.map((w) => `${name(w)} left out`)].join("; ")
      : [renamed.length ? `${renamed.length} renamed` : "", out.length ? `${out.length} left out` : ""].filter(Boolean).join(", ");
  return `${head} ${tail}.`;
}

function carriesWords(c: Claim, roles: ProjectRole[], project: string): string {
  switch (c.kind) {
    case "roles":
      return rolesCarry(c, project);
    case "static":
      return `The same for everyone: \`${literal(c.value)}\``;
    case "if_role":
      return c.roles?.length
        ? `\`true\` if they hold ${joinWith(c.roles.map((k) => labelOf(roles, k)), "or")}, otherwise \`false\``
        : "`false` for everyone, until it has a role";
    case "bundles":
    case "role_ends":
      return KIND_LABEL[c.kind];
    default:
      return ATTRIBUTE_LABEL[c.source ?? ""] ?? c.source ?? "";
  }
}

/**
 * The list: one line per top-level key, the roles claim's first, the rest A
 * to Z. `saved` is what's saved, so a line it lacks says "New in this draft.".
 */
export function claimLines(
  p: Profile,
  roles: ProjectRole[],
  ctx: { app: string; project: string; saved?: Profile | null },
): ClaimLine[] {
  const hero = rolesIndex(p);
  const savedKeys = ctx.saved ? new Set(ctx.saved.claims.map((c) => c.path[0])) : null;
  // A top-level row is a line of its own, even while its name repeats another's
  // (its editor says so); the rows inside one object are one line.
  const groups = new Map<string, number[]>();
  p.claims.forEach((c, i) => {
    const g = c.path.length === 1 ? `row:${i}` : `obj:${c.path[0]}`;
    groups.set(g, [...(groups.get(g) ?? []), i]);
  });
  const lines = Array.from(groups.values(), (rows): ClaimLine => {
    const claims = rows.map((i) => p.claims[i]);
    const key = claims[0].path[0];
    const top = claims[0].path.length === 1 ? claims[0] : undefined;
    const text = claims.find((c) => c.kind === "roles" && (c.format ?? "array") !== "array");
    const inner = Array.from(new Set(claims.map((c) => c.path[1]))).sort(compareKeys);
    const isRoles = rows.includes(hero);
    const nestedRoles = claims.find((c) => c.kind === "roles" && c.path.length > 1);
    return {
      key,
      rows,
      words: top ? carriesWords(top, roles, ctx.project) : `An object holding ${joinWith(inner.map((k) => `\`${k}\``), "and")}`,
      sub: isRoles
        ? rolesState(p, roles, ctx)
        : savedKeys && !savedKeys.has(key)
          ? "New in this draft."
          : nestedRoles
            ? `Its \`${nestedRoles.path[nestedRoles.path.length - 1]}\` is the same list of roles.`
            : undefined,
      warn: text ? `${TEXT_FORMAT[text.format!]}. No app Syndra has a template for reads roles as text.` : undefined,
      roles: isRoles,
      object: !top,
    };
  });
  return lines.sort((a, b) => Number(b.roles) - Number(a.roles) || compareKeys(a.key, b.key));
}

// ---------------------------------------------------------------------------
// Adding a claim (A-16 §3, option a): a menu of finished claims, named the way
// apps expect, A to Z within each of the Carries groups.

/** Keys Zitadel sends itself (A-15's token line). Greyed at the top level of the menu, choosable inside an object. */
export const ZITADEL_SENDS = ["email", "name", "preferred_username"];

/**
 * Keys Zitadel never lets an action set (the shaper's `ZitadelOwns`): its
 * registered claims and its own `urn:zitadel:iam` namespace. Refused at the top level.
 */
export const zitadelOwns = (key: string) =>
  ["sub", "aud", "iss", "exp", "iat", "nbf", "jti", "client_id"].includes(key) || key.startsWith("urn:zitadel:iam");

export interface AddEntry {
  /** "{app}" stands for the app's name. */
  label: string;
  /** The conventional key a finished entry gets. */
  key?: string;
  claim?: Omit<Claim, "path">;
  /** Needs one choice first: the roles (the role picker), or a name and value (its editor, opened). */
  then?: "roles" | "value";
}

export const ADD_MENU: Array<{ group: string; entries: AddEntry[] }> = [
  {
    group: "From the person",
    entries: [
      { label: "Bundles they hold", key: "bundles", claim: { kind: "bundles" } },
      { label: "Each role's end date", key: "role_ends", claim: { kind: "role_ends" } },
      { label: "Email", key: "email", claim: { kind: "attribute", source: "email" } },
      { label: "Family name", key: "family_name", claim: { kind: "attribute", source: "family_name" } },
      { label: "Full name", key: "name", claim: { kind: "attribute", source: "name" } },
      { label: "Given name", key: "given_name", claim: { kind: "attribute", source: "given_name" } },
      { label: "Is a service account", key: "is_service_account", claim: { kind: "attribute", source: "is_service_account" } },
      // `groups` is the default (owner, 4 Nov): five of the six templates that send roles use it.
      { label: "Roles {app} sees", key: "groups", claim: { kind: "roles", format: "array" } },
      { label: "Username", key: "preferred_username", claim: { kind: "attribute", source: "username" } },
      { label: "Yes if they hold a role", then: "roles" },
    ],
  },
  { group: "The same for everyone", entries: [{ label: "A fixed value", then: "value" }] },
];

/**
 * Why an entry can't be added here, said in place, or undefined. At the top
 * level: Zitadel's own keys, a key already sent, and a second roles claim.
 * Inside an object only its own keys are taken.
 */
export function addMuted(e: AddEntry, taken: ReadonlySet<string>, at: { inside: boolean; rolesKey?: string }): string | undefined {
  if (!e.key) return undefined;
  if (!at.inside && ZITADEL_SENDS.includes(e.key)) return `Zitadel sends \`${e.key}\``;
  if (!at.inside && e.claim?.kind === "roles" && at.rolesKey) return `Sent as \`${at.rolesKey}\``;
  return taken.has(e.key) ? `Sent as \`${e.key}\`` : undefined;
}

/** Where the app reads the roles claim's name, from its template's "Set in …" (A-16 §2). */
export const NAME_SET_IN: Record<string, string> = {
  netbird: "NetBird reads the name set in its Settings › Groups › JWT claim.",
  gitlab: "GitLab reads the name set as `groups_attribute` in `gitlab.rb`.",
  "home-assistant": "Home Assistant reads the name set as `claims.groups`.",
};

/** How a roles claim is sent, beside its Sent as switch. */
export const SENT_AS_WORDS: Record<string, string> = {
  array: "A list, and `[]` for anyone who holds none.",
  csv: "Text, with commas.",
  space_delimited: "Text, with spaces.",
};

/** Lands in's ID token option says this on hover and focus (A-16 §1), so it costs no line at rest. */
export const ID_TOKEN_TITLE = "One choice: the ID token carries them only while User Info inside ID Token is on.";

/** "the access token and userinfo", for "In … ." */
export const landsIn = (lands: Lands) => {
  const l = LANDS_LABEL[lands];
  return l[0].toLowerCase() + l.slice(1);
};

/** One line of a template sheet; `warn` is something the app will act on (amber dot). */
export type SheetLine = string | { warn: string };

export interface TemplateSheet {
  /** The short name its Start button uses: "Start from Supabase, custom OIDC". */
  short: string;
  lede: string;
  /** The chooser row's one description. */
  description: string;
  /** The chooser's three unfolded lines. */
  chooser: string[];
  sections: { title: string; lines: SheetLine[] }[];
}

/** Each template's words (A-14 §2), true to `docs/research/2026-11-03-app-claims.md`. */
export const TEMPLATE_SHEET: Record<string, TemplateSheet> = {
  netbird: {
    short: "NetBird",
    lede: "For self-hosted NetBird. Each role you include becomes a NetBird group.",
    description: "Each role you include becomes a NetBird group.",
    chooser: [
      "Sends `groups`, a list of the roles you include, in NetBird's words.",
      "Lands in the access token and userinfo, so NetBird finds it either way.",
      "A rename makes a new NetBird group and moves people out of the old one.",
    ],
    sections: [
      { title: "What it sends", lines: ["`groups`: a list of the roles you include, in NetBird's words, such as `netbird-staff`. Always a list; `[]` when someone holds none.", "Nothing else. NetBird reads who someone is from Zitadel's own `sub`, `email`, `name` and `preferred_username`."] },
      { title: "Where it lands", lines: ["The access token and userinfo. NetBird reads its access token, and its built-in Dex reads the ID token with userinfo, so both ways work."] },
      { title: "What NetBird does with it", lines: ["Each value becomes a NetBird group.", "On every sign-in, NetBird adds the person to the groups in the claim and takes them out of any group it made from a token that the claim no longer names.", { warn: "A rename makes a new group and moves people out of the old one. Policies on the old group stop matching them." }, "A group you made by hand in NetBird with the same name isn't joined."] },
      { title: "What no claim can do", lines: ["Set NetBird's owner, admin or user role. Syndra found no claim that sets them; they are set in NetBird."] },
      { title: "Set in NetBird", lines: ["Settings › Groups: turn on Enable JWT group sync, and set JWT claim to `groups`.", { warn: "JWT allow groups turns away anyone in none of them. Never save one you aren't in." }] },
      { title: "Set in Zitadel", lines: ["Auth token type: JWT. A Bearer token carries no claims.", "User Info inside ID Token: on."] },
    ],
  },
  gitlab: {
    short: "GitLab",
    lede: "For self-managed GitLab signing in with OpenID Connect. Its group lists decide who may sign in and who is an admin.",
    description: "Matches the group lists in GitLab's gitlab.rb.",
    chooser: [
      "Sends `groups`, a list in the words your `gitlab.rb` uses.",
      "Lands in the ID token and userinfo. GitLab merges the two.",
      "Group sync is GitLab Premium or Ultimate. On the free edition these settings do nothing.",
    ],
    sections: [
      { title: "What it sends", lines: ["`groups`: a list in the words your `gitlab.rb` uses, such as `gitlab-users`, `gitlab-admins` and `gitlab-external`. Always a list; `[]` when someone holds none.", "Nothing else. GitLab takes `name`, `email`, `preferred_username`, `given_name` and `family_name` from Zitadel's own claims."] },
      { title: "Where it lands", lines: ["The ID token and userinfo. GitLab merges them, and the ID token wins where they differ."] },
      { title: "What GitLab does with it", lines: [{ warn: "If `required_groups` is set, anyone without a match can't sign in." }, { warn: "If `admin_groups` is set, GitLab makes and unmakes admins on every sign-in, including admins made by hand. `auditor_groups` and `external_groups` work the same way, and external cancels admin." }, { warn: "A missing or renamed claim demotes every admin and can lock everyone out." }] },
      { title: "What no claim can do", lines: ["Do anything on GitLab's free edition. Group sync is Premium or Ultimate only."] },
      { title: "Set in GitLab", lines: ["Under `args.client_options.gitlab`: `groups_attribute: groups`, and the four lists, naming these words exactly.", "Renaming is optional: GitLab's lists can name Syndra's keys directly. If you rename, change `gitlab.rb` in the same step."] },
      { title: "Set in Zitadel", lines: ["User Info inside ID Token: on. GitLab doesn't read the access token."] },
    ],
  },
  "supabase-oidc": {
    short: "Supabase, custom OIDC",
    lede: "For Supabase Auth's custom OIDC provider (custom:). It keeps one object, custom_claims, and drops other keys it doesn't know.",
    description: "Supabase Auth's custom: provider. It keeps only the custom_claims object.",
    chooser: [
      "Sends `custom_claims` › `groups`: a list inside one object, always, `[]` when empty.",
      "Lands in the ID token, the only one Supabase reads.",
      "It lands in `user_metadata`, which the person can edit, so it isn't safe for row-level security as it arrives.",
    ],
    sections: [
      { title: "What it sends", lines: ["`custom_claims` › `groups`: a list of the roles you include, inside one object. Sent as `{\"custom_claims\": {\"groups\": [\"analyst\"]}}`.", "Always sent, with `[]` when someone holds none.", "`given_name`, `family_name` and `name` already reach `user_metadata` on their own. Put them inside `custom_claims` only if your code reads them there."] },
      { title: "Where it lands", lines: ["The ID token, and userinfo too. Supabase reads only the ID token when there is one, and Zitadel always sends one."] },
      { title: "What Supabase does with it", lines: ["It keeps the top-level `custom_claims` object and drops other keys it doesn't know. A top-level `groups` would be lost unless it is on `custom_claims_allowlist`.", { warn: "It rewrites the person's metadata on every sign-in, key by key, and a missing key keeps its old value. That's why the claim is always sent, even empty." }, "Read it as `auth.jwt()->'user_metadata'->'custom_claims'->'groups'`."] },
      { title: "What no claim can do", lines: [{ warn: "Make this safe for row-level security as it arrives. It lands in `user_metadata`, which the signed-in person can edit. Copy it into `app_metadata` with a Custom Access Token Hook first." }, "Refresh groups without a new sign-in through Zitadel. Supabase's own refresh doesn't fetch them."] },
      { title: "Set in Supabase", lines: ["A `custom:` provider with `provider_type: \"oidc\"`. Needs Supabase Auth v2.196.0 or later, or self-hosted 0.8.1.", "Which one is yours? A `custom:` provider is this one; `GOTRUE_EXTERNAL_KEYCLOAK_*` settings mean the Keycloak one."] },
      { title: "Set in Zitadel", lines: ["User Info inside ID Token: on."] },
    ],
  },
  "supabase-keycloak": {
    short: "Supabase, Keycloak",
    lede: "For Supabase Auth's Keycloak provider. It reads only userinfo, and copies every key it doesn't know into custom_claims.",
    description: "Supabase Auth's Keycloak provider. It reads only userinfo.",
    chooser: [
      "Sends `groups`, a top-level list, always, `[]` when empty.",
      "Lands in userinfo, the only place this provider reads.",
      "It asks Keycloak's paths, so Zitadel needs a rewrite in front of it.",
    ],
    sections: [
      { title: "What it sends", lines: ["`groups`: a top-level list of the roles you include. Supabase copies every key it doesn't know into `custom_claims`, so it arrives as `user_metadata.custom_claims.groups`.", "Always sent, with `[]` when someone holds none."] },
      { title: "Where it lands", lines: ["Userinfo. This provider reads only userinfo."] },
      { title: "What Supabase does with it", lines: [{ warn: "It rewrites the person's metadata on every sign-in, key by key, and a missing key keeps its old value. That's why the claim is always sent, even empty." }] },
      { title: "What no claim can do", lines: ["Point this provider at Zitadel by itself. It asks Keycloak's own paths, so Zitadel needs a rewrite in front of it.", { warn: "Make this safe for row-level security as it arrives. It lands in `user_metadata`, which the signed-in person can edit. Copy it into `app_metadata` with a Custom Access Token Hook first." }] },
      { title: "Set in Supabase", lines: ["The `GOTRUE_EXTERNAL_KEYCLOAK_*` settings.", "Which one is yours? Those settings mean this one; a `custom:` provider is the custom OIDC one."] },
      { title: "Set in Zitadel", lines: ["Nothing beyond the app itself: userinfo always carries these claims."] },
    ],
  },
  "home-assistant": {
    short: "Home Assistant",
    lede: "For Home Assistant with the hass-oidc-auth integration. Its groups decide who is an administrator, once.",
    description: "For the hass-oidc-auth integration.",
    chooser: [
      "Sends `groups`, a list. Name the roles `admins` and `users`, Home Assistant's defaults.",
      "Lands in the ID token and userinfo; userinfo fills in what's missing.",
      "Home Assistant sets a role only when someone's user is first made.",
    ],
    sections: [
      { title: "What it sends", lines: ["`groups`: a list of the roles you include. Name them `admins` and `users`, Home Assistant's defaults, or set `roles.admin` and `roles.user` to Syndra's words.", "Always a list. Anything else is read as no groups."] },
      { title: "Where it lands", lines: ["The ID token and userinfo. Home Assistant reads the ID token, and userinfo fills in what's missing."] },
      { title: "What Home Assistant does with it", lines: ["In `admins`: an administrator.", "In `users`, or when no user group is set: a user.", { warn: "If `roles.user` is set, anyone in neither group is turned away." }, { warn: "Renaming the admin role without changing `roles.admin` means new people silently get no admin." }] },
      { title: "What no claim can do", lines: [{ warn: "Change someone's role once their Home Assistant user exists. Home Assistant sets it only when the user is first made. Promote or demote existing people by hand in Home Assistant." }] },
      { title: "Set in Home Assistant", lines: ["The `auth_oidc` integration (hass-oidc-auth). Home Assistant has no OIDC sign-in of its own.", "`claims.groups: groups`, which is already its default."] },
      { title: "Set in Zitadel", lines: ["User Info inside ID Token: on."] },
    ],
  },
  "sign-in-only": {
    short: "Sign-in only",
    lede: "For apps with no groups at all. Syndra adds nothing; who may sign in is decided in Zitadel.",
    description: "For Vaultwarden, Kan and Navidrome. Who may sign in is Zitadel's role check.",
    chooser: [
      "Sends nothing of its own. Zitadel's standard claims do the work.",
      "Who may sign in is Zitadel's role check for the project, not a claim.",
    ],
    sections: [
      { title: "What it sends", lines: ["Nothing of its own. Zitadel's standard `email`, `email_verified`, `name`, `preferred_username` and `picture` do the work."] },
      { title: "What no claim can do", lines: [{ warn: "Decide who may sign in. Vaultwarden and Kan have no groups. Turn on Zitadel's role check for the project, and give the role." }] },
      { title: "Vaultwarden", lines: ["Needs `email` and `email_verified`. Without them, sign-up is blocked.", "Set `SSO_AUDIENCE_TRUSTED` to the project's id: Zitadel puts it in the ID token's audience."] },
      { title: "Kan", lines: ["Reads `email` and `name` from the ID token, or from userinfo when the ID token has no `email`."] },
      { title: "Navidrome", lines: ["It has no OIDC of its own. It signs in through a proxy, which needs a stable `preferred_username` or `email`. If the proxy checks a group, use Custom instead."] },
      { title: "Set in Zitadel", lines: ["The project's role check: on."] },
    ],
  },
  custom: {
    short: "Custom",
    lede: "For in-house apps: SmartDB, Community Slicer, Inventory, Audio-Dash, Printer Config and Syndra itself. No template knows what they read.",
    description: "Every role, in Syndra's words. For in-house apps.",
    chooser: [
      "Sends `roles`, every role in Syndra's words, as a list.",
      "Lands in both places. Roles added later are included.",
      "No template knows what your app reads, so its own code has to read `roles`.",
    ],
    sections: [
      { title: "What it sends", lines: ["`roles`: every role in Syndra's words, as a list; `[]` when someone holds none.", "Add claims for anything else your app needs."] },
      { title: "Where it lands", lines: ["Both: the access token, and userinfo with the ID token."] },
      { title: "Roles added later", lines: ["Included."] },
      { title: "What no claim can do", lines: ["Tell Syndra what your app reads. Its own code has to read what this shape sends."] },
    ],
  },
};

/** The chooser's meta line: "`groups` · access token and userinfo". */
export const TEMPLATE_META: Record<string, string> = {
  netbird: "`groups` · access token and userinfo",
  gitlab: "`groups` · ID token and userinfo",
  "supabase-oidc": "`custom_claims.groups` · ID token and userinfo",
  "supabase-keycloak": "`groups` · userinfo",
  "home-assistant": "`groups` · ID token and userinfo",
  "sign-in-only": "nothing of its own",
  custom: "`roles` · both",
};

/** Which apps a template is for, past its own name (the Suggested note). */
export const TEMPLATE_FOR: Record<string, RegExp> = {
  "sign-in-only": /vaultwarden|kan\b|navidrome/i,
};

/** The "Sent as …" for one claim row, from the preview token. */
export function sentAs(
  c: Claim,
  token: Record<string, unknown> | null | undefined,
  person: string,
  opts: { app: string; sameObject?: boolean },
): string {
  const nested = c.path.length > 1;
  // A nested row's first line shows the whole object; its siblings say so.
  const shown = nested && !opts.sameObject ? { [c.path[0]]: valueAt(token, [c.path[0]]) } : valueAt(token, c.path);
  const v = shown === undefined ? "nothing" : `\`${literal(shown)}\``;
  const tail = nested && opts.sameObject ? ", inside the same object" : "";
  switch (c.kind) {
    case "static":
      return `Sent as ${v} for everyone${tail}.`;
    case "roles": {
      if (c.format && c.format !== "array") return `Sent as ${v} for ${person}${tail}.`;
      const leaf = c.path[c.path.length - 1];
      return nested && !opts.sameObject
        ? `Sent as ${v} for ${person}, with \`"${leaf}": []\` for anyone who holds none.`
        : `Sent as ${v} for ${person}${tail}, and \`[]\` for anyone who holds none.`;
    }
    case "if_role":
      return `Sent as ${v} for ${person}${tail}. Otherwise \`false\`.`;
    case "role_ends":
      return `Sent as ${v} for ${person}${tail}. Roles with no end are left out.`;
    case "bundles":
      return `Sent as ${v} for ${person}${tail}. Bundle names, so a rename reaches ${opts.app}.`;
    default:
      return c.source === "is_service_account"
        ? `Sent as ${v} for ${person}${tail}. Service accounts get \`true\`.`
        : `Sent as ${v} for ${person}${tail}.`;
  }
}

export function clockOf(iso: string | undefined | null): string {
  if (!iso) return "";
  const d = new Date(iso);
  return d.toLocaleTimeString("en-GB", { hour: "2-digit", minute: "2-digit", hour12: false });
}

export function minutesLeft(iso: string, now = Date.now()): number {
  return Math.max(0, Math.ceil((new Date(iso).getTime() - now) / 60000));
}

// ---------------------------------------------------------------------------
// A draft's own checks, before the backend's: what stops a preview or a save.

export const pathKey = (path: string[]) => path.join("\u0000");

/** Each row's problem, by row index. Only `blocksPreview` ones stop the preview. */
export function rowProblems(
  p: Profile,
  ctx: { project: string; missing?: Record<string, string> },
): Array<{ message: string; blocksPreview: boolean } | null> {
  const keys = p.claims.map((c) => pathKey(c.path));
  return p.claims.map((c, i) => {
    const valueless = c.kind === "static" && (c.value === "" || c.value === undefined || (Array.isArray(c.value) && c.value.length === 0));
    const nameless = c.path.some((seg) => !seg.trim());
    // A fixed value lands opened, unnamed and empty (A-16 §3): the line says which is missing.
    if (nameless && valueless) return { message: "Name it and give it a value.", blocksPreview: true };
    if (nameless) return { message: "Name this claim.", blocksPreview: true };
    if (zitadelOwns(c.path[0])) {
      return { message: `\`${c.path[0]}\` is Zitadel's own claim, so Zitadel drops this one. Name it something else, or put it inside an object.`, blocksPreview: true };
    }
    if (valueless) return { message: "Give it a value.", blocksPreview: false };
    if (keys.indexOf(keys[i]) !== i) return { message: `\`${pathLabel(c.path)}\` is sent by another row too.`, blocksPreview: true };
    const child = p.claims.find((o) => o.path.length > c.path.length && c.path.every((seg, j) => o.path[j] === seg));
    if (child) {
      return { message: `\`${pathLabel(c.path)}\` is already an object here, holding \`${child.path[c.path.length]}\`.`, blocksPreview: true };
    }
    if (c.kind === "if_role" && !c.roles?.length) {
      const was = ctx.missing?.[pathKey(c.path)];
      return {
        message: was
          ? `${was} isn't in ${ctx.project}. Choose a role from ${ctx.project}, or remove this row.`
          : "Choose a role, or remove this row.",
        blocksPreview: false,
      };
    }
    return null;
  });
}

/**
 * What one call to the claims action came to, in a few plain words (S3): the
 * backend's outcome classes, said so they follow a colon.
 */
export const OUTCOME_PHRASE: Record<string, string> = {
  served: "served",
  served_lkg: "served from Syndra's last copy",
  served_welcome: "served the welcome roles at a first sign-in",
  served_roles_only: "served their roles, before the rest of their claims were built",
  degraded_no_facts: "their access wasn't built yet",
  degraded_shape_unreadable: "the app's shape couldn't be read",
  degraded_malformed: "their stored access couldn't be read",
  no_grants: "the token named no project",
  contract_fault: "Syndra's answer failed its own check",
  refused_cross_project: "it asked for another project's access",
  not_landing: "the shape sends nothing at that step",
  unknown_function: "it came from a step Syndra doesn't know",
  sig_missing: "unsigned, so not from Zitadel",
  sig_stale: "its signature was too old",
  sig_mismatch: "signed with a key Syndra doesn't hold",
  bad_request: "Syndra couldn't read the call",
  deadline_exceeded: "Syndra took too long to answer",
  panic: "Syndra hit an error",
};

export const outcomePhrase = (o: string | undefined) => (o ? (OUTCOME_PHRASE[o] ?? o) : "");
