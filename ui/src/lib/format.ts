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

/**
 * How long an ask is for, in the words people use for it: "a week", "a month",
 * else "N days". The dialog offers a week, a month and the end of term, so the
 * common lengths read as the choice that was made rather than as arithmetic.
 */
export function lengthWords(days: number): string {
  if (days === 7) return "a week";
  if (days === 14) return "two weeks";
  if (days >= 28 && days <= 31) return "a month";
  return `${days} ${days === 1 ? "day" : "days"}`;
}

/**
 * A pending ask's length, as the person who asked reads it (A-06 §2): the
 * length only, never a date. Approving starts it from the moment it is
 * approved, so a date here would be wrong by however long the ask waits.
 */
export function askLengthForMember(days: number | null | undefined): string {
  if (!days || days < 1) return "With no end date.";
  return `For ${lengthWords(days)}, from when it's approved.`;
}

/**
 * The same ask, as the person deciding reads it: the length and the date that
 * approving it now would end it. "For a month · until 17 Nov". Approval starts
 * it now, so that date is what Approve will do.
 */
export function askLengthForOperator(
  days: number | null | undefined,
  now: Date = new Date(),
): string {
  if (!days || days < 1) return "With no end date";
  const end = new Date(now.getTime() + days * 24 * 60 * 60 * 1000);
  return `For ${lengthWords(days)} · until ${formatShortDate(end.toISOString(), now)}`;
}

export type UrgencyTone = "critical" | "warning" | "neutral" | "expired";

/**
 * Compute a human countdown ("ends in 3 days", "ends in 2 hours", "ended") plus a
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
    return { countdown: "ended", tone: "expired", daysLeft };
  }
  if (daysLeft <= 1) {
    const hours = Math.max(1, Math.round(ms / (1000 * 60 * 60)));
    return {
      countdown: `ends in ${hours} hour${hours === 1 ? "" : "s"}`,
      tone: "critical",
      daysLeft,
    };
  }
  if (daysLeft <= 7) {
    return {
      countdown: `ends in ${daysLeft} day${daysLeft === 1 ? "" : "s"}`,
      tone: "critical",
      daysLeft,
    };
  }
  if (daysLeft <= 14) {
    return { countdown: `ends in ${daysLeft} days`, tone: "warning", daysLeft };
  }
  return { countdown: `ends in ${daysLeft} days`, tone: "neutral", daysLeft };
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
  if (minutes < 60) return `${minutes} min ago`;
  const hours = Math.round(minutes / 60);
  if (hours < 24) return `${hours} ${hours === 1 ? "hour" : "hours"} ago`;
  const days = Math.round(hours / 24);
  if (days < 30) return `${days} ${days === 1 ? "day" : "days"} ago`;
  return formatShortDate(iso, now);
}

/**
 * "2h ago": the short stamp a note seat has room for (A-04 §10). Past a month it
 * is the date, the year only when it isn't this year.
 */
export function formatAgoShort(iso: string | null | undefined, now: Date = new Date()): string {
  if (!iso) return "";
  const then = new Date(iso).getTime();
  if (Number.isNaN(then)) return "";
  const minutes = Math.max(0, Math.round((now.getTime() - then) / 60_000));
  if (minutes < 1) return "just now";
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.round(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.round(hours / 24);
  if (days < 30) return `${days}d ago`;
  return formatShortDate(iso, now);
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

/**
 * "2 Aug" — the form a deadline is spoken in. The year appears only when the
 * date is not in this year ("2 Aug 2027"): a year on every date is noise, and
 * a missing year on a date a year away is a false statement (A-04 §7.9).
 */
export function formatShortDate(
  iso: string | null | undefined,
  now: Date = new Date(),
): string {
  if (!iso) return "—";
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return "—";
  return date.toLocaleDateString(DATE_LOCALE, {
    day: "numeric",
    month: "short",
    ...(date.getFullYear() === now.getFullYear() ? {} : { year: "numeric" }),
  });
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

/**
 * "14:32" when the moment is today, otherwise "10 Sept 14:32". For a feed that
 * is NOT grouped under its date: a bare clock beside a day-old entry reads as
 * today, which is a false statement about when something happened.
 */
export function formatWhen(iso: string | null | undefined, now: Date = new Date()): string {
  if (!iso) return "—";
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return "—";
  const clock = formatClock(iso);
  return date.toDateString() === now.toDateString() ? clock : `${formatShortDate(iso, now)} ${clock}`;
}

/** "Friday 31 July" — the line under a greeting. */
export function formatWeekday(date: Date = new Date()): string {
  // Short month, matching formatShortDate everywhere else. "Saturday 19
  // September" beside a feed of "18 Sept" rows is the same product speaking
  // two dialects on one screen.
  return date.toLocaleDateString(DATE_LOCALE, {
    weekday: "long",
    day: "numeric",
    month: "short",
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

/**
 * The confirmed half of a Recorded/Confirmed count pair — what a role's or
 * project's holder count looks like next to what the observation store backs
 * up of it (`one-truth-many-checks`).
 *
 * `confirmed` is `undefined`, not zero, when the org has never been checked —
 * that must read as "not confirmed", never as a confirmed absence.
 */
export function confirmedNote(recorded: number, confirmed: number | undefined): string {
  if (confirmed === undefined) return "not confirmed";
  if (confirmed === recorded) return "confirmed";
  return `${confirmed} confirmed`;
}

/**
 * Who someone is, in a picker (People everywhere 1): "title · team", read
 * before the email under it. Team is free-text Zitadel metadata, so either
 * half may be missing; an empty line is "", never "—" — the em dash belongs
 * to People's own Team column, where a seat holds its place.
 */
export function personLine(user: { title?: string | null; team?: string | null }): string {
  return [user.title, user.team]
    .map((part) => part?.trim())
    .filter(Boolean)
    .join(" · ");
}

/**
 * Why the last pass concluded nothing, in words.
 *
 * The reason is a closed vocabulary the backend writes to a row
 * (`db/target_reconciliation.go`), and this card printed the value itself —
 * `target_unreachable`, in the smallest grey type on the dashboard, under a
 * sentence written for somebody who has never read the schema. The whole point
 * of the entry is that the operator's next move differs by reason: there is
 * nothing to reach for, or there is something to fix.
 *
 * An unrecognised value falls through unchanged rather than being swallowed. A
 * reason nobody has worded yet is still more use than no reason at all, and it
 * shows up here rather than disappearing quietly.
 */
export function whyNotChecked(reason: string): string {
  switch (reason) {
    case "target_unreachable":
      return "It did not answer when Syndra last tried.";
    case "read_refused":
      return "It answered and would not serve the read — something to fix rather than wait out.";
    case "read_stale":
      return "It served an older copy of itself, which cannot be compared against what Syndra expects.";
    case "read_truncated":
      return "The read hit its safety limit, so what is missing cannot be told from what is absent.";
    case "findings_unrecorded":
      return "The pass ran and its findings could not be written down.";
    default:
      return reason;
  }
}

/**
 * Some add-ons report `product_version` already carrying the product name
 * ("TrueNAS-25.10.5"), which doubled the name wherever it was shown beside
 * one ("TrueNAS TrueNAS-25.10.5"). Stripped once, here, so every caller gets
 * a plain version number.
 */
export function versionLabel(name: string, productVersion?: string): string {
  if (!productVersion) return "";
  const stripped = productVersion.replace(new RegExp(`^${name}[-\\s]*`, "i"), "");
  return stripped || productVersion;
}
