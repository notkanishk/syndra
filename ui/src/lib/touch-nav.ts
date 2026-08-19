import {
  leafMatches,
  type BadgeTone,
  type IndicatorKey,
  type NavEntry,
  type NavLeaf,
} from "@/lib/nav";

/**
 * The rail's tree, as a thumb has to meet it.
 *
 * `lib/nav.ts` stays the only declaration of structure — this module derives
 * from it and adds nothing. A destination that appeared here and not there
 * would be the same class of bug as the member storage row that existed in the
 * rail and not in the middleware.
 *
 * The one judgement made here is what a GROUP becomes. A group is a section
 * label with children, and a tab bar has no room for a section label: so the
 * group becomes one destination that lands on its first child and keeps the
 * group's own name. Renaming the tab after the child would make the tree's own
 * word — "Access" — unreachable, and it is the word that explains why Projects,
 * Roles and Apps sit together.
 */
export interface TouchDestination {
  /** The group's label, or the leaf's. Never the child's. */
  label: string;
  /** Where the tab goes: the leaf, or the group's first child. */
  href: string;
  /** Present only for a group. */
  children?: NavLeaf[];
  /** Counted indicator, for a leaf that has one. */
  indicator?: IndicatorKey;
  tone?: BadgeTone;
}

export function touchDestinations(entries: NavEntry[]): TouchDestination[] {
  return entries.map((entry) =>
    entry.kind === "leaf"
      ? { label: entry.label, href: entry.href, indicator: entry.indicator, tone: entry.tone }
      : {
          label: entry.label,
          href: entry.children[0]?.href ?? "/",
          children: entry.children,
        },
  );
}

/** True when the current path belongs to this destination or any of its children. */
export function destinationMatches(destination: TouchDestination, pathname: string): boolean {
  if (destination.children) {
    return destination.children.some((child) => leafMatches(child, pathname));
  }
  return leafMatches({ kind: "leaf", label: destination.label, href: destination.href }, pathname);
}

/**
 * Four or fewer destinations become a tab bar; more become a sheet.
 *
 * Count-driven rather than audience-driven, so a deployment that registers a
 * fourth add-on cannot silently produce a five-tab bar. There is deliberately
 * no "More" tab: a fifth slot holding leftovers quietly rewrites the rail's own
 * rule into "four things and a drawer".
 */
export const TAB_BAR_LIMIT = 4;

export function navShape(destinations: TouchDestination[]): "tabs" | "sheet" {
  return destinations.length <= TAB_BAR_LIMIT ? "tabs" : "sheet";
}

/**
 * How many destinations want attention — never how many items are outstanding.
 *
 * The distinction is the point. Summing gives a number spanning several kinds
 * of work that no single action can reduce; counting destinations tells an
 * operator how many places they have to go, which is a question the nav can
 * actually answer.
 */
export function destinationsWantingAttention(
  destinations: TouchDestination[],
  counts: Partial<Record<IndicatorKey, number>> | undefined,
): number {
  return destinations.filter((destination) => wantsAttention(destination, counts)).length;
}

function wantsAttention(
  destination: TouchDestination,
  counts: Partial<Record<IndicatorKey, number>> | undefined,
): boolean {
  const indicators = destination.children
    ? destination.children.map((child) => child.indicator)
    : [destination.indicator];
  return indicators.some((key) => (key ? Number(counts?.[key] ?? 0) > 0 : false));
}

/** The tones a destination is currently carrying, for the dot that stands in. */
export function tonesInPlay(
  destination: TouchDestination,
  counts: Partial<Record<IndicatorKey, number>> | undefined,
): (BadgeTone | undefined)[] {
  const leaves = destination.children ?? [];
  return leaves
    .filter((leaf) => (leaf.indicator ? Number(counts?.[leaf.indicator] ?? 0) > 0 : false))
    .map((leaf) => leaf.tone);
}
