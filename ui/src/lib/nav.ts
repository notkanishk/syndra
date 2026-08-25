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

export type UiView = "basic" | "advanced";
export type Audience = UiView | "member";

/** Keys of GET /api/v1/governance/indicators. */
export type IndicatorKey =
  | "pending_requests"
  | "expiring_grants"
  | "pending_propagation"
  | "drift"
  | "unconfirmed_revocations"
  | "holds_due";

/**
 * Badge tone follows the semantic palette and nothing else: Requests and
 * Pending changes are work (accent), Expiring access is a deadline (warn),
 * Unexplained access is something that already went wrong (danger).
 *
 * Withdrawn access is danger too, and for a sharper reason than drift: drift is
 * access that appeared without an explanation, and this is access somebody
 * decided to take away that is still there.
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
}

export interface NavGroup {
  kind: "group";
  /** A section label, not a link. */
  label: string;
  children: NavLeaf[];
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
  leaf("Home", "/"),
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
  group("Automation", [
    leaf("Automatic rules", "/policies"),
    leaf("Pending changes", "/governance/pending", {
      indicator: "pending_propagation",
      tone: "accent",
    }),
    leaf("Change history", "/operations/cascades"),
    leaf("Access map", "/graph"),
    leaf("Settings", "/automation/settings"),
  ]),
  group("Review", [
    leaf("Unexplained access", "/governance/drift", { indicator: "drift", tone: "danger" }),
    leaf("Withdrawn access", "/governance/unconfirmed-revocations", {
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
    leaf("Identity provider", "/zitadel"),
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
    // The LLDAP bridge's row. Gone with the bridge: it named a service that no
    // longer exists, and a nav entry for a deleted subsystem is worse than a
    // missing one — an operator clicks it before they read anything.
    leaf("Event activity", "/operations"),
  ]),
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
export function targetNav(targets: string[]): NavEntry[] {
  if (targets.length === 0) return ADVANCED_NAV;
  return ADVANCED_NAV.map((entry) => {
    if (entry.kind !== "group" || entry.label !== "System") return entry;
    return group("System", [
      // Identity provider, then Connected systems — the two static rows — and
      // each registered target under the index it belongs to.
      ...entry.children.slice(0, 2),
      ...targets.map((target) =>
        leaf(targetLabel(target), `/system/targets/${target}`, {
          pattern: new RegExp(`^/system/targets/${target}(/|$)`),
        }),
      ),
      ...entry.children.slice(2),
    ]);
  });
}

/**
 * A target's display name. Title-cased from its id, because the id is
 * deployment configuration an operator wrote and a mapping table here would be
 * a second place to add a target — which is how the day a UniFi add-on ships,
 * its row is called `unifi` on one screen and `UniFi Access` on another.
 */
export function targetLabel(target: string): string {
  const known: Record<string, string> = { truenas: "TrueNAS", unifi: "UniFi Access" };
  if (known[target]) return known[target];
  return target.charAt(0).toUpperCase() + target.slice(1);
}

/**
 * Member — three destinations. No Home: a member's landing IS their access, so
 * a separate landing would be an empty room in front of the only room. The view
 * switch is not rendered for members at all.
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
  leaf("Network storage", "/storage"),
];

export function navFor(audience: Audience): NavEntry[] {
  if (audience === "member") return MEMBER_NAV;
  return audience === "advanced" ? ADVANCED_NAV : BASIC_NAV;
}

/** True when `pathname` belongs to this nav row. */
export function leafMatches(item: NavLeaf, pathname: string): boolean {
  if (pathname === item.href) return true;
  if (item.pattern) return item.pattern.test(pathname);
  return item.href !== "/" && pathname.startsWith(`${item.href}/`);
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
export const MEMBER_ROUTES = navLeaves(MEMBER_NAV).map((leaf) => leaf.href);

/**
 * Reachability is `leafMatches` over the member's own leaves, so the rule the
 * middleware enforces is exactly the rule the rail highlights. It follows a
 * leaf's `pattern` if it ever gains one — `/projects/{id}/roles/{key}` is why
 * that field exists, and a member leaf with a detail route would need it.
 * Otherwise a sub-path belongs to its parent, so `/storage/{target}` is
 * reachable the day it exists; `/` matches exactly, or it admits everything.
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
export function crumbsFor(pathname: string, audience: Audience): Crumb[] {
  const entries = navFor(audience);

  for (const entry of entries) {
    if (entry.kind === "leaf") {
      if (entry.href === pathname) return [{ label: entry.label }];
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
    for (const child of candidates) {
      if (child.href !== pathname && leafMatches(child, pathname)) {
        const parent: Crumb[] = entry.kind === "group" ? [{ label: entry.label }] : [];
        return [...parent, { label: child.label, href: child.href }];
      }
    }
  }

  return [];
}
