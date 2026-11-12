/**
 * The navigation contract.
 *
 * Structure never moves. Only badge values change. A section with nothing in
 * it renders in place with a hollow `0` — it never disappears, and nothing
 * ever appears above it. The previous sidebar injected a Drift section at the
 * top when its count went non-zero, pushing every other item down under the
 * operator's cursor; that is prohibited here, and this file is why it can't
 * come back: the arrays are static and the counts are looked up by key.
 *
 * Switching Basic → Advanced APPENDS sections. It never reorders or renames
 * what was already there.
 */

import { sortByName } from "@/lib/sort";

/** What the storage page is called, said once. */
export const STORAGE_LABEL = "Network storage";

export type UiView = "basic" | "advanced";
export type Audience = UiView | "member";

/** Keys of GET /api/v1/governance/indicators. */
export type IndicatorKey =
  | "pending_requests"
  | "expiring_grants"
  | "pending_propagation"
  | "drift"
  | "unconfirmed_revocations"
  | "holds_due"
  /** Read by a leaf's `retries` pill, never as a leaf's own count. */
  | "retrying"
  | "gave_up";

/**
 * Badge tone follows the semantic palette and nothing else: Requests and
 * Pending changes are work (accent), Expiring access is a deadline (warn),
 * Drift is something that already went wrong (danger).
 *
 * Unfinished revocations is danger too, and for a sharper reason than drift:
 * drift is access that appeared without an explanation, and this is access
 * somebody revoked that is still there.
 */
export type BadgeTone = "accent" | "warn" | "danger";

export interface NavLeaf {
  kind: "leaf";
  label: string;
  href: string;
  indicator?: IndicatorKey;
  tone?: BadgeTone;
  /**
   * Routes beyond `href` that belong to this row — detail pages, mostly.
   * A pattern rather than a prefix because /projects/{id} is Projects while
   * /projects/{id}/roles/{key} is Roles, and a prefix cannot tell them apart.
   */
  pattern?: RegExp;
  /** Set by `operatorNav`: this row is in Advanced and not in Basic. */
  advancedOnly?: boolean;
  /**
   * This row also carries the retry pill (`retryBadge`): amber with the count
   * while Syndra sends changes again on its own, danger once it gave up on
   * one nobody has marked seen. Static, like every seat here; only its value
   * moves. Home carries it so Basic, which has no Pending changes row, is
   * always in the know (owner, 5 Nov 2026).
   */
  retries?: boolean;
  /**
   * A seat for rows drawn from the registry, never a row itself: `memberNav`
   * puts one row per connected system in its place. Its `pattern` is what
   * makes those rows reachable from the static tree, so the middleware (no
   * fetch at the Edge) admits them without knowing them. Its own `href` is no
   * page: it is not reachable, not a route and never a crumb.
   */
  family?: "systems";
}

/**
 * A connected system as a rail draws it, from GET /targets (operators) or
 * GET /me/systems (anyone's own rows). `name` is the backend's (one source);
 * `kind` is absent until Syndra has read the system, and GET /me/systems never
 * carries one without it (not read is operators-only).
 */
export interface NavSystem {
  target: string;
  name: string;
  kind?: string;
}

export interface NavGroup {
  kind: "group";
  /** A section label, not a link. */
  label: string;
  children: NavLeaf[];
  /** Set by `operatorNav`: this whole section is in Advanced and not in Basic. */
  advancedOnly?: boolean;
}

export type NavEntry = NavLeaf | NavGroup;

const leaf = (
  label: string,
  href: string,
  extra: Partial<Omit<NavLeaf, "kind" | "label" | "href">> = {},
): NavLeaf => ({ kind: "leaf", label, href, ...extra });

const group = (label: string, children: NavLeaf[]): NavGroup => ({
  kind: "group",
  label,
  children,
});

/** Basic — the everyday surface. Four destinations. */
export const BASIC_NAV: NavEntry[] = [
  leaf("Home", "/", { retries: true }),
  leaf("People", "/users", { pattern: /^\/users(\/|$)/ }),
  group("Access", [
    leaf("Projects", "/projects", { pattern: /^\/projects(\/[^/]+)?$/ }),
    leaf("Roles", "/roles", { pattern: /^(\/roles(\/|$)|\/projects\/[^/]+\/roles\/)/ }),
    leaf("Apps", "/applications", { pattern: /^\/applications(\/|$)/ }),
  ]),
  leaf("Requests", "/requests", { indicator: "pending_requests", tone: "accent" }),
];

/** Advanced — everything in Basic, plus the machine that acts on everyone. */
export const ADVANCED_NAV: NavEntry[] = [
  ...BASIC_NAV,
  leaf("Bundles", "/bundles", { pattern: /^\/bundles(\/|$)/ }),
  // N18, 3a: the things that give access at top level; a Changes group for
  // what Syndra sends, what it sent, and which roles wait for you.
  leaf("Automatic rules", "/policies"),
  leaf("Insights", "/insights", { pattern: /^\/(insights|graph)(\/|$)/ }),
  group("Changes", [
    leaf("Pending changes", "/governance/pending", {
      indicator: "pending_propagation",
      tone: "accent",
      retries: true,
    }),
    leaf("Change history", "/operations/cascades"),
    leaf("Roles that wait", "/changes/roles-that-wait", {
      pattern: /^(\/changes\/roles-that-wait|\/automation\/settings)(\/|$)/,
    }),
  ]),
  group("Review", [
    leaf("Drift", "/governance/drift", { indicator: "drift", tone: "danger" }),
    leaf("Unfinished revocations", "/governance/unconfirmed-revocations", {
      indicator: "unconfirmed_revocations",
      tone: "danger",
    }),
    leaf("Expiring access", "/review/expiring-access", {
      indicator: "expiring_grants",
      tone: "warn",
    }),
    // Beside Expiring access, never inside it. Inaction means the opposite
    // thing in each — an expiring grant lapses if ignored, a hold stays in
    // force — and one list would sit "do nothing and access ends" next to "do
    // nothing and access stays blocked".
    leaf("Holds due", "/review/holds", { indicator: "holds_due", tone: "warn" }),
    leaf("Audit", "/audit"),
  ]),
  group("System", [
    leaf("Zitadel", "/zitadel"),
    // The target plane's own row, present on every deployment INCLUDING one
    // that has registered no add-on at all.
    //
    // It used to appear only alongside a target, so a deployment with none had
    // no add-on anywhere in the product — not a row, not a page, not a
    // sentence. An operator who had just shipped the add-on platform went
    // looking for it and found the rail unchanged, which reads as "it did not
    // ship" rather than "it is not configured here". Those are different facts
    // and only one of them was true.
    //
    // Static, so `crumbsFor` can find it and so the seat is held whether or not
    // this deployment ever registers a target — the same rule as every other
    // row in this file. The per-target rows are appended after it by
    // `targetNav`, which is deployment configuration and may legitimately vary.
    leaf("Connected systems", "/system/targets", { pattern: /^\/system\/targets$/ }),
    // How systems reach Zitadel, so it sits with them: `targetNav` slices the
    // first two rows, so this lands after the targets and before Incoming events.
    leaf("Service accounts", "/system/service-accounts"),
    // The LLDAP bridge's row. Gone with the bridge: it named a service that no
    // longer exists, and a nav entry for a deleted subsystem is worse than a
    // missing one — an operator clicks it before they read anything.
    // Exact: a prefix would light this row on Change history (/operations/cascades) too.
    leaf("Incoming events", "/operations", { pattern: /^\/operations$/ }),
  ]),
  // Last, and a leaf of its own: settings that belong to the whole makerspace
  // rather than to one connected system (the term dates, today). Appended, so
  // nothing above it moves.
  leaf("Makerspace settings", "/settings"),
];

/**
 * The per-target System rows, derived from DEPLOYMENT CONFIGURATION.
 *
 * Not from data, and the difference is the whole IA rule. An operator on a
 * deployment running a TrueNAS add-on sees the TrueNAS row whether or not it
 * currently answers, whether or not anybody is bound to it, and whether or not
 * this operator can read a single account on it. A row that appeared when the
 * first person was provisioned would be structure moving in response to data,
 * which is exactly what this file exists to prevent.
 *
 * `GET /api/v1/targets` is the source, and it lists what was registered — never
 * what is reachable. Reachability belongs on the page, where it can be
 * explained.
 */
export function targetNav(targets: readonly NavSystem[]): NavEntry[] {
  if (targets.length === 0) return ADVANCED_NAV;
  return ADVANCED_NAV.map((entry) => {
    if (entry.kind !== "group" || entry.label !== "System") return entry;
    return group("System", [
      // Zitadel, then Connected systems — the two static rows — and
      // each registered target under the index it belongs to.
      ...entry.children.slice(0, 2),
      ...targets.map(({ target, name }) =>
        leaf(name, `/system/targets/${target}`, {
          pattern: new RegExp(`^/system/targets/${target}(/|$)`),
        }),
      ),
      ...entry.children.slice(2),
    ]);
  });
}

/**
 * What an operator's rail and sheet draw, whichever view is on: Advanced's
 * whole tree, in Advanced's order, each part Basic does not have marked
 * `advancedOnly`. The rail wraps those parts in an Unfold that follows the
 * view (A-08 §3), so a row both views share is the same row in both and
 * never remounts. Derived from the two lists above, never restated: a row
 * added to `ADVANCED_NAV` is Advanced-only until it is added to Basic.
 *
 * A group Basic does not have is marked as a whole (its rows follow it); in a
 * group both views have, each row is judged on its own.
 */
export function operatorNav(targets: readonly NavSystem[]): NavEntry[] {
  const basicLeaves = new Set(navLeaves(BASIC_NAV).map((entry) => entry.href));
  const basicGroups = new Set(BASIC_NAV.flatMap((entry) => (entry.kind === "group" ? [entry.label] : [])));
  const mark = (entry: NavLeaf): NavLeaf => ({ ...entry, advancedOnly: !basicLeaves.has(entry.href) });
  return targetNav(targets).map((entry) =>
    entry.kind === "leaf"
      ? mark(entry)
      : { ...entry, advancedOnly: !basicGroups.has(entry.label), children: entry.children.map(mark) },
  );
}

/**
 * A system's id title-cased: the fallback for a row drawn before any roster
 * has loaded, and the backend's own last resort (handlers.titleCased).
 *
 * Never a map of product names. A system's name is a registration attribute
 * the backend shapes once (`name` on both rosters: the operator's name, else
 * the manifest's product, else this); a map here was a second place to name a
 * system, and the day a second laser cutter shipped it would have called both
 * the same. `useSystemNames` is how a surface reads the real one.
 */
export function targetLabel(target: string): string {
  return target.charAt(0).toUpperCase() + target.slice(1);
}

/**
 * Member — three destinations, then one row per connected system that is not
 * storage. No Home: a member's landing IS their access, so a separate landing
 * would be an empty room in front of the only room. The view switch is not
 * rendered for members at all.
 *
 * This comment said "two" until the storage row shipped, which is the same
 * drift that put the row in the rail and not in the middleware — a developer
 * reads the sentence before they trust the array, and the count is quoted
 * elsewhere as a fact about the product.
 */
export const MEMBER_NAV: NavEntry[] = [
  leaf("My access", "/"),
  leaf("Requests", "/requests"),
  /**
   * Present for every member, always, whatever they can reach.
   *
   * Gating this on entitlement would make the rail move as somebody's roles
   * change — the one thing this file forbids — and it would also be the wrong
   * answer to the question a member without access is asking, which is "can I
   * get storage?". The page answers that; a missing row does not.
   */
  leaf(STORAGE_LABEL, "/storage"),
  /**
   * The family seat (addon-contract 3.10): last, so nothing above it ever
   * moves. `memberNav` draws one row here per system Syndra has read whose kind
   * is not storage (storage folds into the row above), alphabetical by name.
   * A system not read yet has no row and no member page: only operators see it
   * (owner, 11 Nov). No system, no row: no placeholder.
   */
  leaf("Connected systems", "/systems", { family: "systems", pattern: /^\/systems\/[^/]+$/ }),
];

/** The kind that folds into Network storage rather than drawing a row. */
export const STORAGE_KIND = "storage";

/** A system of the storage kind: Network storage's, never a row of its own. */
export const isStorage = (system: { kind?: string }) => system.kind === STORAGE_KIND;

/**
 * The systems that get a row of their own, alphabetical (lib/sort): the one
 * rule, for the member's rail and an operator's account sheet alike. The
 * member reads (GET /me/systems, /me/targets) never carry a system not read
 * yet, so kind is known here; a kindless row is still never drawn.
 */
export function memberSystems<T extends NavSystem>(systems: readonly T[]): T[] {
  return sortByName(
    systems.filter((s) => s.kind && !isStorage(s)),
    (s) => s.name,
    (s) => s.target,
  );
}

/** One row per system of `memberSystems`, each to its own page. */
const systemRows = (systems: readonly NavSystem[]): NavLeaf[] =>
  memberSystems(systems).map((s) => leaf(s.name, `/systems/${s.target}`));

/**
 * The member's rail: `MEMBER_NAV` with its family seat replaced by one row per
 * system (`memberSystems`). Rows are a registration, never an entitlement: a
 * member none of whose roles reaches the system still has the row, and the
 * page says so with a next step.
 */
export function memberNav(systems: readonly NavSystem[]): NavEntry[] {
  const rows = systemRows(systems);
  return MEMBER_NAV.flatMap((entry) => (entry.kind === "leaf" && entry.family ? rows : [entry]));
}

/**
 * An operator's own pages, reached from the Account sheet (A-03 §4, B).
 *
 * An operator is a person too: their own access, asked for like anyone's, and
 * a storage password and folders of their own. The rail is about running the
 * makerspace. Everything about you sits
 * behind the account (shell-account board), so that is where these live for
 * an operator. A member already has the same page in their rail, and one row
 * per job means the sheet doesn't offer it to them twice.
 */
export const ACCOUNT_NAV: NavLeaf[] = [leaf("My access", "/me"), leaf(STORAGE_LABEL, "/storage")];

/**
 * The account sheet's rows: `ACCOUNT_NAV`, then the operator's own systems
 * that are not storage, from the same GET /me/systems a member's rail reads
 * (`memberSystems`), so an operator's own laser account has a way in.
 */
export function accountNav(systems: readonly NavSystem[]): NavLeaf[] {
  return [...ACCOUNT_NAV, ...systemRows(systems)];
}

export function navFor(audience: Audience): NavEntry[] {
  if (audience === "member") return MEMBER_NAV;
  return audience === "advanced" ? ADVANCED_NAV : BASIC_NAV;
}

/** True when `pathname` belongs to this nav row. */
export function leafMatches(item: NavLeaf, pathname: string): boolean {
  if (item.family) return Boolean(item.pattern?.test(pathname));
  if (pathname === item.href) return true;
  if (item.pattern) return item.pattern.test(pathname);
  return item.href !== "/" && pathname.startsWith(`${item.href}/`);
}

/**
 * True when `href` goes to a page only Advanced's rail has a row for. Judged by
 * the two lists above, never restated: a route Basic's rail claims is Basic's,
 * one only Advanced's rail claims is Advanced-only. A connected system's own
 * pages are dynamic rows `targetNav` adds, so the System group is named here.
 * Query and hash never decide it (A-20 P10).
 */
export function isAdvancedOnly(href: string): boolean {
  const path = href.split(/[?#]/)[0];
  if (!path.startsWith("/")) return false;
  if (navLeaves(BASIC_NAV).some((entry) => leafMatches(entry, path))) return false;
  return /^\/system(\/|$)/.test(path) || navLeaves(ADVANCED_NAV).some((entry) => leafMatches(entry, path));
}

/** Flattens a nav tree to its leaves, preserving order. */
export function navLeaves(entries: NavEntry[]): NavLeaf[] {
  return entries.flatMap((entry) => (entry.kind === "leaf" ? [entry] : entry.children));
}

/**
 * Every route a member may reach — **derived from the rail they are shown**,
 * never restated. Anything else is not rendered and not reachable for them:
 * the backend 403s the underlying reads regardless, and an affordance that
 * will fail is worse than no affordance.
 *
 * The derivation is the point. A hand-written copy of `MEMBER_NAV` is what
 * this already was in `middleware.ts`, and the storage row shipped into one
 * and not the other — every member who tapped it was redirected off their own
 * page. A second copy here would be the same bug waiting on the next row, and
 * an equality test only catches it when somebody runs the tests. A row added
 * to `MEMBER_NAV` is reachable in the same commit.
 *
 * An allowlist rather than a denylist on purpose — a new operator route is
 * protected the moment it exists, instead of being exposed until somebody
 * remembers to add it here.
 */
export const MEMBER_ROUTES = navLeaves(MEMBER_NAV)
  .filter((leaf) => !leaf.family)
  .map((leaf) => leaf.href);

/**
 * Reachability is `leafMatches` over the member's own leaves, so the rule the
 * middleware enforces is exactly the rule the rail highlights. It follows a
 * leaf's `pattern` if it ever gains one — `/projects/{id}/roles/{key}` is why
 * that field exists, and a member leaf with a detail route would need it.
 * Otherwise a sub-path belongs to its parent; `/` matches exactly, or it
 * admits everything. The family seat's pattern admits every
 * `/systems/{target}` with no data, so the rows `memberNav` draws are
 * reachable the moment they are drawn (and `/systems` itself, no page, is not).
 */
export function memberMayVisit(pathname: string): boolean {
  return navLeaves(MEMBER_NAV).some((leaf) => leafMatches(leaf, pathname));
}

export interface Crumb {
  label: string;
  href?: string;
}

/**
 * Breadcrumb for a route, derived from the same tree the rail renders so the
 * two can never disagree. Detail pages append their own trailing crumb once
 * the name resolves (see useCrumb) rather than flashing a raw id.
 */
export function crumbsFor(
  pathname: string,
  audience: Audience,
  nameOf: (target: string) => string | undefined = targetLabel,
): Crumb[] {
  // A target's own routes, resolved from the PATH rather than from the roster.
  //
  // The per-target rows are appended by `targetNav` from deployment
  // configuration, so they are not in the static tree this function walks and a
  // target page has never had a breadcrumb. `targetLabel` turns an id into a
  // name with no lookup, so the chain is derivable here without the roster and
  // without making this function async or data-dependent.
  //
  // The target stays a LINK on its sub-routes, because the likeliest move after
  // opening a target's mappings is going back to the page you opened them from.
  //
  // Not gated on `audience === "advanced"` any more: the target's own routes
  // live under System, an Advanced-only section, but the crumb names the PAGE,
  // not the sidebar — an operator who followed a link there in Basic still
  // needs to know where they are.
  // `nameOf` is the roster's name when the caller has it (TopBar), else the
  // id title-cased; undefined while the roster is being read, and then the
  // target is not crumbed yet, so the id is never drawn and then replaced.
  const onTarget = /^\/system\/targets\/([^/]+)(\/.*)?$/.exec(pathname);
  if (onTarget && audience !== "member") {
    const [, target, rest] = onTarget;
    const name = nameOf(target);
    if (name === undefined) return [{ label: "System" }];
    const self: Crumb = { label: name, href: `/system/targets/${target}` };
    return rest ? [{ label: "System" }, self] : [{ label: "System" }, { label: self.label }];
  }

  // A family row (`/systems/{target}`) has no crumb from here: the seat is
  // never one, and the page names itself with `useCrumb` once its read lands,
  // so a system a member does not meet is never named and a name never
  // replaces an id.

  const own = findCrumb(navFor(audience), pathname);
  if (own.length > 0) return own;

  // The view toggle governs the SIDEBAR, not the page's own name. Basic is a
  // subset of Advanced's tree (`ADVANCED_NAV = [...BASIC_NAV, ...]`), so an
  // operator in Basic who reaches an Advanced-only page (a link, a bookmark, a
  // direct URL) still landed somewhere real — looking it up in the full
  // operator nav is what keeps the crumb the page's own name instead of the
  // "Syndra" fallback below. A member's tree is not a subset of anything here,
  // so this stays out of that case.
  if (audience !== "member") {
    const operator = findCrumb(ADVANCED_NAV, pathname);
    return operator.length > 0 ? operator : findCrumb(ACCOUNT_NAV, pathname);
  }

  return [];
}

function findCrumb(entries: NavEntry[], pathname: string): Crumb[] {
  for (const entry of entries) {
    if (entry.kind === "leaf") {
      if (entry.href === pathname && !entry.family) return [{ label: entry.label }];
      continue;
    }
    for (const child of entry.children) {
      if (child.href === pathname) {
        return [{ label: entry.label }, { label: child.label, href: child.href }];
      }
    }
  }

  // Detail routes: keep the parent context so the crumb reads as a place in
  // the product rather than a URL.
  for (const entry of entries) {
    const candidates = entry.kind === "leaf" ? [entry] : entry.children;
    for (const child of candidates.filter((c) => !c.family)) {
      if (child.href !== pathname && leafMatches(child, pathname)) {
        const parent: Crumb[] = entry.kind === "group" ? [{ label: entry.label }] : [];
        return [...parent, { label: child.label, href: child.href }];
      }
    }
  }

  return [];
}
