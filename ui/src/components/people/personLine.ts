import { orderedSources } from "@/components/access/AccessSource";
import { ENDING_WITHIN_DAYS, inDays } from "@/components/member/headerLine";
import { daysUntil } from "@/lib/format";
import { observedHolding } from "@/lib/people-filters";
import type { UserAccessView } from "@/lib/queries/useUsers";
import { sortByName } from "@/lib/sort";
import { holds, onItsWay } from "@/lib/standing";

/**
 * What the line says while there is no count to say: loading, or a read of
 * Zitadel that is missing, failed or cut short. The page's own read warning
 * says which; this never claims a number it doesn't have.
 */
export const CHECKING = "Checking what Zitadel shows.";

/**
 * The line that knows, on a person's page (A-24 §9, Q10), in place of the lede
 * that said the same thing for everyone. My access's ladder (`headerLine`), in
 * an operator's words: what they hold, then the one thing worth knowing.
 *
 * The count is `observedHolding`, the one judgement the People row uses, so
 * the page and the row can never disagree; its projects and its ladder read
 * the same read's standings (the answer each row's standing words come from),
 * never Syndra's records alone. An end date is the record's, said only for a
 * role Zitadel shows held and that one source alone gives: a role a bundle or
 * rule also gives does not end when its direct grant does (`orderedSources`,
 * as the page's held-twice note decides it).
 */
export function personLine(
  view: UserAccessView | undefined,
  names: { role: (projectId: string, roleKey: string) => string },
  now: Date = new Date(),
): string {
  const count = view ? observedHolding(view) : undefined;
  if (!view || count === undefined) return CHECKING;

  const roles = (view.projects ?? []).flatMap((p) =>
    Object.entries(p.standings ?? {}).map(([key, standing]) => ({
      project: p,
      key,
      standing,
      label: `${names.role(p.project_id, key)} in ${p.project_name}`,
      sources: orderedSources([...p.source_roles, ...p.derived_roles].find((r) => r.role_key === key)?.reasons).length,
    })),
  );
  const held = roles.filter((r) => holds(r.standing));
  const places = Array.from(new Set(held.map((r) => r.project.project_name)));
  const holding =
    count === 0
      ? "Holds no roles."
      : places.length === 1
        ? `Holds ${count} ${count === 1 ? "role" : "roles"} in ${places[0]}.`
        : places.length > 1
          ? `Holds ${count} roles across ${places.length} projects.`
          : `Holds ${count} ${count === 1 ? "role" : "roles"}.`;

  // The one thing worth knowing, in My access's order: an end soon, then what was given and is not
  // in Zitadel, then what is on its way. Days are counted as My access counts them (`daysUntil`).
  const ending = sortByName(
    held.filter((r) => r.sources <= 1),
    (r) => r.label,
    (r) => `${r.project.project_id}:${r.key}`,
  )
    .map((r) => {
      const holding = r.project.holdings?.[r.key];
      return { ...r, days: holding?.kind === "until" ? daysUntil(holding.until, now) : null };
    })
    .filter((r): r is typeof r & { days: number } => r.days !== null && r.days >= 0 && r.days <= ENDING_WITHIN_DAYS)
    .sort((a, b) => a.days - b.days);
  if (ending.length) return `${holding} ${ending[0].label} ends ${inDays(ending[0].days)}.`;

  // "Not in Zitadel" is the standing's own word (lib/standing.ts), said as a row says it.
  const missing = roles.filter((r) => r.standing === "not_held");
  if (missing.length === 1) return `${holding} Not in Zitadel: ${missing[0].label}.`;
  if (missing.length > 1) return `${holding} Not in Zitadel: ${missing.length} roles they were given.`;

  const coming = roles.filter((r) => onItsWay(r.standing));
  if (coming.length === 1) return `${holding} ${coming[0].label} is on its way to Zitadel.`;
  if (coming.length > 1) return `${holding} ${coming.length} roles are on their way to Zitadel.`;

  return holding;
}
