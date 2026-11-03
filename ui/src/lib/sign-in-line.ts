import { formatShortDate, formatSignIn } from "@/lib/format";
import type { PersonSignIns } from "@/lib/queries/useSignIns";

const DAY = 24 * 60 * 60 * 1000;

/**
 * A project's last-sign-in line on a person's page (A-10 §6), or null for none.
 *
 * - "Last sign-in 10 min ago, to Fab queue": the app used last is named; a row
 *   from before apps were kept names none.
 * - "No sign-in to its app in 90 days" ("its apps" for two or more), or
 *   "since 3 Sept" when Syndra began reading sign-ins less than 90 days ago.
 * - null for a project with no app, or while sign-ins aren't arriving at all:
 *   never say what hasn't been read.
 */
export function projectSignInLine(
  projectId: string,
  apps: Array<{ id: string; name: string; project_id: string }>,
  signIns: PersonSignIns | undefined,
  now: Date = new Date(),
): string | null {
  const own = apps.filter((a) => a.project_id === projectId);
  if (own.length === 0 || !signIns?.receiving) return null;
  const last = signIns.usage
    .filter((r) => r.kind === "sign_in" && r.project_id === projectId)
    .sort((a, b) => b.last_at.localeCompare(a.last_at))[0];
  if (last) {
    const app = own.find((a) => a.id === last.app_id);
    return `Last sign-in ${formatSignIn(last.last_at, now)}${app ? `, to ${app.name}` : ""}`;
  }
  const its = own.length === 1 ? "its app" : "its apps";
  const since = signIns.receiving_since ? new Date(signIns.receiving_since) : null;
  if (since && now.getTime() - since.getTime() < 90 * DAY) {
    return `No sign-in to ${its} since ${formatShortDate(signIns.receiving_since, now)}`;
  }
  return `No sign-in to ${its} in 90 days`;
}

/**
 * The app page's meta (A-10 §7): "9 people in 7 days · last sign-in 10 min
 * ago", "Nobody in 7 days · last sign-in 2 Oct", "No sign-in yet", or null
 * while sign-ins aren't arriving.
 */
export function appSignInMeta(
  usage: { receiving: boolean; people_7d?: number; last_at?: string | null } | undefined,
  now: Date = new Date(),
): string | null {
  if (!usage?.receiving) return null;
  if (!usage.last_at) return "No sign-in yet";
  const n = usage.people_7d ?? 0;
  const people = n === 0 ? "Nobody in 7 days" : `${n} ${n === 1 ? "person" : "people"} in 7 days`;
  return `${people} · last sign-in ${formatSignIn(usage.last_at, now)}`;
}
