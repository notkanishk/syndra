// Display helpers for role and project references. The backend speaks in
// `project_id:role_key` pairs; the UI surfaces those to non-technical staff
// who recognize project names better than internal keys. This module
// centralizes the human-readable formatting so every view does it the same way.

/**
 * How a role reads in a SENTENCE — a toast, a dialog lede, a warning banner.
 *
 * A role is never named alone. `admin` in Printing Lab and `admin` in Metal
 * Shop are two different roles, so a sentence that says "now holds admin" has
 * not said which grant was written. The project is not decoration here; it is
 * half of the identity.
 *
 * Prose gets the role's human name ("Trained operator"), not its key: a
 * sentence is read, and `laser_trained` is not a word. Rows want the opposite
 * — see <RoleRef/>, which shows the key in monospace because a table is
 * scanned for identifiers rather than read.
 */
export function roleLabel(projectName: string, roleKey: string, roleDisplayName?: string): string {
  return `${projectName} / ${roleDisplayName || humanizeKey(roleKey)}`;
}

/**
 * Convert a snake/kebab/lower role key into a friendlier label. Used as a
 * fallback when the project catalog doesn't carry an explicit `label` for
 * the role.
 */
export function humanizeKey(key: string): string {
  if (!key) return "";
  return key
    .replace(/[_-]+/g, " ")
    .replace(/\b\w/g, (c) => c.toUpperCase());
}

/**
 * How long a request asked for, as the decider reads it.
 *
 * Deliberately says "for N days" rather than a date: the expiry is computed
 * when the request is approved, not when it is made, so a date here would be
 * wrong by however long the request sat in the queue.
 *
 * Absent or zero is not "unspecified" — the backend reads it as no expiry at
 * all, so it is the one value that must never render as blank.
 */
export function describeDuration(days: number | null | undefined): string {
  if (!days || days < 1) return "no end date";
  return `for ${days} ${days === 1 ? "day" : "days"}`;
}

export type UrgencyTone = "critical" | "warning" | "neutral" | "expired";

/**
 * Compute a human countdown ("in 3 days", "in 2 hours", "expired") plus a
 * semantic tone the UI can use to color-code urgency. Returns `null` for an
 * absent expiry (permanent grants).
 */
export function describeExpiry(expiresAt: string | null | undefined, now: Date = new Date()):
  | { countdown: string; tone: UrgencyTone; daysLeft: number }
  | null {
  if (!expiresAt) return null;
  const target = new Date(expiresAt).getTime();
  if (Number.isNaN(target)) return null;
  const ms = target - now.getTime();
  const daysLeft = Math.ceil(ms / (1000 * 60 * 60 * 24));

  if (ms <= 0) {
    return { countdown: "expired", tone: "expired", daysLeft };
  }
  if (daysLeft <= 1) {
    const hours = Math.max(1, Math.round(ms / (1000 * 60 * 60)));
    return {
      countdown: `expires in ${hours} hour${hours === 1 ? "" : "s"}`,
      tone: "critical",
      daysLeft,
    };
  }
  if (daysLeft <= 7) {
    return {
      countdown: `expires in ${daysLeft} day${daysLeft === 1 ? "" : "s"}`,
      tone: "critical",
      daysLeft,
    };
  }
  if (daysLeft <= 14) {
    return { countdown: `expires in ${daysLeft} days`, tone: "warning", daysLeft };
  }
  return { countdown: `expires in ${daysLeft} days`, tone: "neutral", daysLeft };
}

/**
 * "4h ago" / "2d ago" — the relative stamp list rows use. Deliberately coarse:
 * on a work queue the difference between 4 and 5 hours changes nothing, and a
 * precise timestamp invites reading it as an SLA.
 */
export function formatRelative(iso: string | null | undefined, now: Date = new Date()): string {
  if (!iso) return "—";
  const then = new Date(iso).getTime();
  if (Number.isNaN(then)) return "—";

  const seconds = Math.max(0, Math.round((now.getTime() - then) / 1000));
  if (seconds < 60) return "just now";
  const minutes = Math.round(seconds / 60);
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.round(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.round(hours / 24);
  if (days < 30) return `${days}d ago`;
  return formatShortDate(iso);
}

/**
 * Dates are formatted in a FIXED locale, not the ambient one.
 *
 * A date rendered on the server in one locale and re-rendered in the browser in
 * another is a hydration mismatch: React discards the subtree and rebuilds it,
 * and the operator sees the value flicker. en-GB also happens to be the form
 * the design speaks in — "2 Aug", not "Aug 2".
 */
const DATE_LOCALE = "en-GB";

/** "2 Aug" — the form a deadline is spoken in. */
export function formatShortDate(iso: string | null | undefined): string {
  if (!iso) return "—";
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return "—";
  return date.toLocaleDateString(DATE_LOCALE, { day: "numeric", month: "short" });
}

/**
 * "14:32" — the time of day, for a feed already grouped under its date. Fixed
 * to a 24-hour clock so a row never renders "2:05" ambiguously between morning
 * and afternoon in an audit trail.
 */
export function formatClock(iso: string | null | undefined): string {
  if (!iso) return "—";
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return "—";
  return date.toLocaleTimeString(DATE_LOCALE, { hour: "2-digit", minute: "2-digit", hour12: false });
}

/** "18 Dec 2026" — the form a resolved expiry date is confirmed in. */
export function formatLongDate(iso: string | null | undefined): string {
  if (!iso) return "—";
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return "—";
  return date.toLocaleDateString(DATE_LOCALE, { day: "numeric", month: "short", year: "numeric" });
}

/** "Friday 31 July" — the line under a greeting. */
export function formatWeekday(date: Date = new Date()): string {
  return date.toLocaleDateString(DATE_LOCALE, {
    weekday: "long",
    day: "numeric",
    month: "long",
  });
}

/** Whole days until an expiry, or null when there isn't one. */
export function daysUntil(iso: string | null | undefined, now: Date = new Date()): number | null {
  if (!iso) return null;
  const target = new Date(iso).getTime();
  if (Number.isNaN(target)) return null;
  return Math.ceil((target - now.getTime()) / (1000 * 60 * 60 * 24));
}

/**
 * Bytes, in the units the division actually produces.
 *
 * Divides by 1024 and says so. Two copies of this used to live in components
 * and they disagreed: one divided by 1024 and labelled the result KB/MB/GB,
 * which names a quantity 2.4% smaller than the one it printed by the time it
 * reaches terabytes. Binary steps get binary names.
 */
export function formatBytes(bytes: number): string {
  const units = ["B", "KiB", "MiB", "GiB", "TiB", "PiB"];
  let value = bytes;
  let unit = 0;
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024;
    unit += 1;
  }
  // One decimal place above bytes; nobody needs "1.0 B".
  return unit === 0 ? `${Math.round(value)} B` : `${value.toFixed(1)} ${units[unit]}`;
}

/**
 * Names in a sentence, with the conjunction people actually write.
 *
 * `join(", ")` gives "gitlab_data, main", which reads as a truncated list. The
 * JSX side of this problem is separate — a component interleaving <Mono> can't
 * use a string helper — but the failure is the same one, and it has already
 * shipped once as two share names run together into one.
 */
export function formatList(items: string[]): string {
  if (items.length <= 1) return items[0] ?? "";
  if (items.length === 2) return `${items[0]} and ${items[1]}`;
  return `${items.slice(0, -1).join(", ")} and ${items[items.length - 1]}`;
}
