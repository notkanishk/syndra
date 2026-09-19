import type { DriftTriageItem } from "@/lib/queries/useDrift";

/**
 * The Drift queue's filters, in the URL.
 *
 * They used to be two `useState` calls, which meant a filtered queue had no
 * link: an operator who narrowed to one project and sent it to a colleague sent
 * them the whole backlog, and a refresh threw the narrowing away. The People
 * page solved this already (`people-filters.ts`); this is the same shape, so
 * any screen can link into a pre-filtered Drift queue the way Home links into
 * pre-filtered People.
 *
 * Two of these are request parameters the backend already accepts. Four are
 * applied here, over the rows that came back — three because they ask about
 * fields the endpoint does not filter on, and one (person) because asking the
 * server would break the control that sets it. Which is which is stated on each
 * one: a filter that silently meant "of the first page" would be a worse lie
 * than no filter at all.
 */
export interface DriftFilters {
  /** Server: `project_id`. */
  project: string;
  /**
   * Client, deliberately, although the endpoint accepts `user_id`.
   *
   * The person dropdown is built from the people who appear in the queue. Ask
   * the SERVER to narrow to one person and the queue comes back holding only
   * that person — so the dropdown rebuilds from their rows alone and collapses
   * to the one name already chosen, with no way back except "Anyone". Filtering
   * here keeps every name in hand. The queue is a triage backlog of tens of
   * rows, not a page of thousands; it is all in the browser already.
   */
  user: string;
  /** Server: `source` — webhook, or the scheduled sweep. */
  source: string;
  /** Client: what is known about where this came from. */
  origin: "" | "named" | "unattributable" | "missed";
  /** Client: one role key. */
  role: string;
  /** Client: how long it has been sitting there. */
  age: "" | "today" | "week" | "month";
}

export const EMPTY_DRIFT_FILTERS: DriftFilters = {
  project: "",
  user: "",
  source: "",
  origin: "",
  role: "",
  age: "",
};

/**
 * What can be said about where a row came from.
 *
 * This is the closest thing the queue has to "why can't Syndra explain it",
 * and it is deliberately not called that. A PENDING drift row has no recorded
 * explanation by construction — if a bundle, rule or grant explained it, the
 * sweep would never have raised it, or would have retracted it already. So the
 * honest axis is not "explained how" but "how much do we know about who did
 * it", which is exactly what an operator needs to decide whether they can
 * attribute it or must judge it blind.
 */
export const ORIGIN_LABELS: Record<Exclude<DriftFilters["origin"], "">, string> = {
  named: "Somebody is named",
  unattributable: "Nobody can be named",
  missed: "The event probably went missing",
};

export function originOf(item: DriftTriageItem): Exclude<DriftFilters["origin"], ""> | null {
  if (item.upstream_actor) return "named";
  if (item.event_possibly_missed) return "missed";
  if (item.attribution_unavailable) return "unattributable";
  return null;
}

export const AGE_LABELS: Record<Exclude<DriftFilters["age"], "">, string> = {
  today: "Found today",
  week: "Within a week",
  month: "Older than a month",
};

function matchesAge(detectedAt: string, age: DriftFilters["age"], now: Date): boolean {
  if (!age) return true;
  const found = new Date(detectedAt);
  if (Number.isNaN(found.getTime())) return false;
  const days = (now.getTime() - found.getTime()) / 86_400_000;
  if (age === "today") return found.toDateString() === now.toDateString();
  if (age === "week") return days <= 7;
  return days > 30;
}

/** The two the backend filters on, as the query hook wants them. */
export function driftRequest(filters: DriftFilters): {
  project_id?: string;
  source?: string;
} {
  return {
    project_id: filters.project || undefined,
    source: filters.source || undefined,
  };
}

/**
 * The four this page applies itself, over what the request returned.
 *
 * `except` drops one filter from the pass — that is how a dropdown's own
 * options are built. A list of people narrowed by the person already chosen
 * has exactly one name in it, which is the bug this argument exists to
 * prevent: every other filter still applies, so the options stay relevant,
 * but never so relevant that they exclude the alternatives.
 */
export function applyDriftFilters(
  items: DriftTriageItem[],
  filters: DriftFilters,
  now: Date = new Date(),
  except?: keyof DriftFilters,
): DriftTriageItem[] {
  const on = (key: keyof DriftFilters) => key !== except && Boolean(filters[key]);
  return items.filter((item) => {
    if (on("user") && item.user_id !== filters.user) return false;
    if (on("origin") && originOf(item) !== filters.origin) return false;
    if (on("role") && !item.role_keys.includes(filters.role)) return false;
    if (on("age") && !matchesAge(item.detected_at, filters.age, now)) return false;
    return true;
  });
}

export function hasAnyDriftFilter(filters: DriftFilters): boolean {
  return Object.values(filters).some(Boolean);
}

export function parseDriftFilters(params: URLSearchParams): DriftFilters {
  const origin = params.get("origin") ?? "";
  const age = params.get("age") ?? "";
  return {
    project: params.get("project") ?? "",
    user: params.get("user") ?? "",
    source: params.get("source") ?? "",
    // A hand-edited or stale URL degrades to "no filter", never to an empty
    // list that reads as "nothing matches".
    origin: origin in ORIGIN_LABELS ? (origin as DriftFilters["origin"]) : "",
    role: params.get("role") ?? "",
    age: age in AGE_LABELS ? (age as DriftFilters["age"]) : "",
  };
}

export function serializeDriftFilters(
  filters: DriftFilters,
  extra: Record<string, string> = {},
): string {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries({ ...filters, ...extra })) {
    if (value) params.set(key, value);
  }
  const query = params.toString();
  return query ? `?${query}` : "";
}

/** A link into the Drift queue, already narrowed. */
export function driftHref(filters: Partial<DriftFilters>): string {
  return `/governance/drift${serializeDriftFilters({ ...EMPTY_DRIFT_FILTERS, ...filters })}`;
}
