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
  | "drift";

/**
 * Badge tone follows the semantic palette and nothing else: Requests and
 * Pending changes are work (accent), Expiring access is a deadline (warn),
 * Unexplained access is something that already went wrong (danger).
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
  leaf("Today", "/"),
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
    leaf("Expiring access", "/review/expiring-access", {
      indicator: "expiring_grants",
      tone: "warn",
    }),
    leaf("Audit", "/audit"),
  ]),
  group("System", [
    leaf("Identity provider", "/zitadel"),
    leaf("Hardware sync", "/system/hardware-sync"),
    leaf("Event activity", "/operations"),
  ]),
];

/**
 * Member — two destinations, and that is deliberate. No Today: a work queue
 * for someone with no queue is an empty room. The view switch is not rendered
 * for members at all.
 */
export const MEMBER_NAV: NavEntry[] = [
  leaf("My access", "/"),
  leaf("Requests", "/requests"),
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
 * Every route a member may reach. Anything else is not rendered and not
 * reachable for them — the backend 403s the underlying reads regardless, and
 * an affordance that will fail is worse than no affordance.
 */
export const MEMBER_ROUTES = ["/", "/requests"];

export function memberMayVisit(pathname: string): boolean {
  return MEMBER_ROUTES.includes(pathname);
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
