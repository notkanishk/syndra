import { humanizeKey } from "@/lib/format";
import type { UserListEntry } from "@/lib/queries/useUsers";

/**
 * The People filter model.
 *
 * Filters live in the URL rather than in component state, and that is the whole
 * point of this file: it makes every count elsewhere in the product a link.
 * "3 unexplained" on Home, "12 people hold this" on a role, a bundle chip on
 * someone's record — each becomes a href into People already narrowed to
 * exactly that set, and the resulting view is shareable and bookmarkable
 * instead of being a state an operator has to reconstruct by hand.
 *
 * Kept as plain functions over plain data so the semantics are testable without
 * mounting a page or faking a router.
 */

/**
 * The one thing about a person that might need you. Mirrors the "Needs
 * attention" column, plus two cohorts that only make sense as filters:
 * `no-access` (arrived, never set up) and `departed` (left, never cleaned up).
 */
export type Attention =
  | "expiring"
  | "unexplained"
  | "requests"
  | "no-access"
  | "departed";

export const ATTENTION_VALUES: readonly Attention[] = [
  "expiring",
  "unexplained",
  "requests",
  "no-access",
  "departed",
];

export const ATTENTION_LABELS: Record<Attention, string> = {
  expiring: "Expiring access",
  unexplained: "Drift",
  requests: "Open requests",
  "no-access": "No access yet",
  departed: "Departed, still has access",
};

export interface PeopleFilters {
  q: string;
  /** Project **id**, not name — a link must survive a rename. */
  project: string;
  /**
   * Role key, only meaningful alongside `project`. Membership is resolved
   * through the role-members endpoint rather than guessed from the people
   * list, so "who holds this role" means the same thing on both screens.
   */
  role: string;
  /** Bundle name — what the chips on a person's record already carry. */
  bundle: string;
  /**
   * Bundle version, only meaningful alongside `bundle`. Narrows "in the Lab
   * Tech bundle" to "on v2 of it" — the question that only exists once bundles
   * are versioned, and the one that finds the people an earlier publish left
   * behind.
   *
   * A string because it comes from and goes back to the URL; compared against
   * the row's pinned version by value.
   */
  version: string;
  attention: Attention | "";
}

export const EMPTY_FILTERS: PeopleFilters = {
  q: "",
  project: "",
  role: "",
  bundle: "",
  version: "",
  attention: "",
};

function isAttention(value: string): value is Attention {
  return (ATTENTION_VALUES as readonly string[]).includes(value);
}

/** Reads filters out of a query string. Unknown values degrade to unset. */
export function parseFilters(params: URLSearchParams): PeopleFilters {
  const attention = params.get("attention") ?? "";
  return {
    q: params.get("q") ?? "",
    project: params.get("project") ?? "",
    role: params.get("role") ?? "",
    bundle: params.get("bundle") ?? "",
    // A version with no bundle narrows nothing — "everyone on v2" spans
    // unrelated bundles and is not a question anybody asks. Dropped rather
    // than honoured, so a hand-edited URL degrades to the bundle-less view
    // instead of an empty one.
    version: params.get("bundle") ? (params.get("version") ?? "") : "",
    attention: isAttention(attention) ? attention : "",
  };
}

/**
 * Serialises filters back to a query string, omitting empties so a cleared
 * filter leaves no trace in the URL — `/users` and `/users?q=&project=` are the
 * same view and should share one address.
 */
export function serializeFilters(filters: PeopleFilters, extra: Record<string, string> = {}): string {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries({ ...filters, ...extra })) {
    if (value) params.set(key, value);
  }
  const query = params.toString();
  return query ? `?${query}` : "";
}

/** A href into People, pre-narrowed. The link form every other screen uses. */
export function peopleHref(filters: Partial<PeopleFilters>, extra: Record<string, string> = {}): string {
  return `/users${serializeFilters({ ...EMPTY_FILTERS, ...filters }, extra)}`;
}

export function hasAnyFilter(filters: PeopleFilters): boolean {
  return Boolean(
    filters.q || filters.project || filters.role || filters.bundle || filters.version || filters.attention,
  );
}

export function isDeparted(status: string | undefined): boolean {
  const value = (status ?? "").toLowerCase();
  return value === "departed" || value === "inactive" || value === "alumni" || value === "deactivated";
}

function matchesAttention(entry: UserListEntry, attention: Attention): boolean {
  switch (attention) {
    case "expiring":
      return entry.expiring_count > 0;
    case "unexplained":
      return entry.unexplained_count > 0;
    case "requests":
      return entry.open_request_count > 0;
    case "no-access":
      return entry.effective_role_count === 0;
    case "departed":
      // Only departed people who still hold something: a cleanly offboarded
      // account is not work, and listing it as such would bury the ones that are.
      return isDeparted(entry.user.status) && entry.effective_role_count > 0;
  }
}

/**
 * Applies every set filter. `q` is handled server-side by the users query, so
 * it is deliberately NOT re-applied here — doing so would double-filter with
 * different matching rules and make the count disagree with the list.
 *
 * `roleHolders` is the id set from the role-members endpoint when a role filter
 * is active. `null` means "not loaded yet", and the rows are left unfiltered
 * rather than emptied: briefly showing too many people is recoverable, briefly
 * showing none reads as "nobody holds this" and is not.
 */
export function applyFilters(
  rows: UserListEntry[],
  filters: PeopleFilters,
  roleHolders: ReadonlySet<string> | null = null,
): UserListEntry[] {
  return rows.filter((entry) => {
    if (filters.project && !(entry.key_project_ids ?? []).includes(filters.project)) return false;
    if (filters.role && roleHolders && !roleHolders.has(entry.user.id)) return false;
    if (filters.bundle && !(entry.bundle_names ?? []).includes(filters.bundle)) return false;
    if (filters.version && String(entry.bundle_versions?.[filters.bundle] ?? "") !== filters.version) {
      return false;
    }
    if (filters.attention && !matchesAttention(entry, filters.attention)) return false;
    return true;
  });
}

/**
 * Describes the active filters in one sentence, for the selection bar and the
 * empty state. A bulk action that says "all 214 people matching your filter"
 * without naming the filter is asking an operator to trust a number they can't
 * check.
 */
export function describeFilters(filters: PeopleFilters, projectName?: string): string {
  const parts: string[] = [];
  if (filters.q) parts.push(`matching “${filters.q}”`);
  if (filters.project) parts.push(`in ${projectName || "the selected project"}`);
  // A key is not a word; the humanised form is the fallback prose already uses.
  if (filters.role) parts.push(`holding ${humanizeKey(filters.role)}`);
  if (filters.bundle) {
    parts.push(
      filters.version
        ? `on v${filters.version} of the ${filters.bundle} bundle`
        : `in the ${filters.bundle} bundle`,
    );
  }
  if (filters.attention) parts.push(`with ${ATTENTION_LABELS[filters.attention].toLowerCase()}`);
  if (parts.length === 0) return "";
  if (parts.length === 1) return parts[0];
  return `${parts.slice(0, -1).join(", ")} and ${parts[parts.length - 1]}`;
}
