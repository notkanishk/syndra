import { ENDING_WITHIN_DAYS, inDays } from "@/components/member/headerLine";
import type { UserAccessView } from "@/lib/queries/useUsers";
import { sortByName } from "@/lib/sort";
import { holds, onItsWay } from "@/lib/standing";
import { daysUntil } from "@/lib/format";

/**
 * The line that knows, on a person's page (A-24 §9, Q10), in place of the lede
 * that said the same thing for everyone. My access's ladder (`headerLine`), in
 * an operator's words: what they hold, then the one thing worth knowing.
 *
 * Everything it counts is the person's standings from Syndra's one read of
 * Zitadel (`standings`, the same answer each row's standing words come from),
 * never Syndra's records alone. When that read is missing, failed or cut
 * short, the line says so and counts nothing: it says less, never something
 * untrue. An end date is the record's (the holding's `until`), said only for a
 * role Zitadel shows held, as My access does.
 */
export function personLine(
  view: UserAccessView | undefined,
  names: { role: (projectId: string, roleKey: string) => string },
  now: Date = new Date(),
): string {
  if (!view) return "Checking what they hold.";
  const read = view.observation;
  if (!read?.read_at) return "Zitadel hasn't answered yet, so what they hold isn't counted.";
  if (!read.current) return "Zitadel couldn't be read just now, so what they hold isn't counted.";
  if (read.truncated) return "Zitadel's answer was cut short, so what they hold isn't counted.";

  const roles = (view.projects ?? []).flatMap((p) =>
    Object.entries(p.standings ?? {}).map(([key, standing]) => ({
      project: p,
      key,
      standing,
      label: `${names.role(p.project_id, key)} in ${p.project_name}`,
    })),
  );
  const held = roles.filter((r) => holds(r.standing));
  const places = new Set(held.map((r) => r.project.project_id));
  const count =
    held.length === 0
      ? "Holds no roles."
      : places.size === 1
        ? `Holds ${held.length} ${held.length === 1 ? "role" : "roles"} in ${held[0].project.project_name}.`
        : `Holds ${held.length} roles across ${places.size} projects.`;

  // The one thing worth knowing, in My access's order: an end soon, then what was given and is not
  // in Zitadel, then what is on its way.
  // A to Z first, then by days: the sort is stable, so a tie on days reads alphabetically.
  // Days counted as My access counts them (`daysUntil`, the makerspace's days).
  const ending = sortByName(held, (r) => r.label, (r) => `${r.project.project_id}:${r.key}`)
    .map((r) => ({ ...r, holding: r.project.holdings?.[r.key] }))
    .map((r) => ({ ...r, days: r.holding?.kind === "until" ? daysUntil(r.holding.until, now) : null }))
    .filter((r): r is typeof r & { days: number } => r.days !== null && r.days >= 0 && r.days <= ENDING_WITHIN_DAYS)
    .sort((a, b) => a.days - b.days);
  if (ending.length) return `${count} ${ending[0].label} ends ${inDays(ending[0].days)}.`;

  const missing = roles.filter((r) => r.standing === "not_held");
  if (missing.length === 1) return `${count} ${missing[0].label} isn't in Zitadel yet.`;
  if (missing.length > 1) return `${count} ${missing.length} roles they were given aren't in Zitadel yet.`;

  const coming = roles.filter((r) => onItsWay(r.standing));
  if (coming.length === 1) return `${count} ${coming[0].label} is on its way to Zitadel.`;
  if (coming.length > 1) return `${count} ${coming.length} roles are on their way to Zitadel.`;

  return count;
}
