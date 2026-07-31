// Display helpers for role and project references. The backend speaks in
// `project_id:role_key` pairs; the UI surfaces those to non-technical staff
// who recognize project names better than internal keys. This module
// centralizes the human-readable formatting so every view does it the same way.

interface ProjectInfo {
  id: string;
  name: string;
  roles?: Array<{ key: string; label?: string }>;
}

/**
 * Format a project_id + role_key pair into a human label, with the raw pair
 * available as a secondary monospace tag for power users. Returns an object
 * so callers can render the parts independently — `label` typically reads
 * like "{Project Name} · {Role Label}" and `raw` is `{project_id}:{role_key}`.
 */
export function formatRoleRef(
  projectId: string,
  roleKey: string,
  projects: ProjectInfo[],
): { label: string; raw: string } {
  const project = projects.find((p) => p.id === projectId);
  const projectLabel = project?.name ?? projectId;
  const role = project?.roles?.find((r) => r.key === roleKey);
  const roleLabel = role?.label ?? humanizeKey(roleKey);
  return {
    label: `${projectLabel} · ${roleLabel}`,
    raw: `${projectId}:${roleKey}`,
  };
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
