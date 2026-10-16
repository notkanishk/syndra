import type { Panel } from "@/components/insights/AnswerPanel";
import { dayLabel, dayOf } from "@/components/insights/days";
import type { Model } from "@/components/insights/model";
import type { InsightsLens, InsightsUsage } from "@/lib/queries/useInsights";
import { RECENT_USE } from "@/components/insights/useInsightsUrl";

/** What a lens says: the people it counts per role node, and the sentences. */
export interface LensRead {
  panel: Panel;
  /** People per role node. These are the roles the map lights. */
  counts: Map<string, number>;
  /** What the number on a lit role counts, after "4 people". */
  unit: string;
  /** The quiet legend sentence under the map. */
  note?: string;
}

interface Read<T> {
  data?: T;
  isError: boolean;
}

const SHOWN = 5;
const people = (n: number) => `${n} ${n === 1 ? "person" : "people"}`;
const roles = (n: number) => `${n} ${n === 1 ? "role" : "roles"}`;

type Named = { project_id: string; role_key: string; project_name?: string; role_label?: string };

/** A role as a person says it. The map's own name wins; the lens's names cover a role the map has not drawn. */
export function roleName(m: Model, r: Named): string {
  const id = m.roleId(r.project_id, r.role_key);
  if (id) return m.nameOf(id);
  return [r.project_name, r.role_label].filter(Boolean).join(" · ") || "A role no longer on the map";
}

function tally(m: Model, rows: Array<Named & { people: number }>): Map<string, number> {
  const counts = new Map<string, number>();
  for (const r of rows) {
    const id = m.roleId(r.project_id, r.role_key);
    if (id && r.people > 0) counts.set(id, r.people);
  }
  return counts;
}

const sentence = (eyebrow: string, title: string, text: string, tone: Panel["tone"] = "neutral"): LensRead => ({
  counts: new Map(),
  unit: "",
  panel: { tone, eyebrow, title, lines: [{ tone: "ink", text }] },
});

const more = (n: number) => (n > 0 ? [{ tone: "ink" as const, text: `And ${roles(n)} more.` }] : []);
const NOT_READ = "Syndra hasn't read who holds what from Zitadel yet.";

export function usageRead(u: InsightsUsage, m: Model): LensRead {
  const eyebrow = `${RECENT_USE} · ${u.window_days} days`;
  if (!u.sources.sign_in.receiving) return sentence(eyebrow, "Not showing yet", "This fills in once sign-ins arrive.");
  if (!u.holders_observed) return sentence(eyebrow, "Not read yet", NOT_READ);
  const rows = [...u.held_not_used].sort((a, b) => b.people - a.people);
  if (rows.length === 0) return sentence(eyebrow, `Every role someone holds has been used in ${u.window_days} days`, "Nothing is held without being used.");
  return {
    counts: tally(m, rows),
    unit: "held but not used",
    note: `Lit roles are held by people who haven't used them in ${u.window_days} days. The number is how many.`,
    panel: {
      tone: "neutral",
      eyebrow,
      title: `${roles(rows.length)} held but not used in ${u.window_days} days`,
      lines: [
        ...rows.slice(0, SHOWN).map((r) => ({
          tone: "ink" as const,
          lead: roleName(m, r),
          text: `is held by ${people(r.people)}, and none has used it.`,
        })),
        ...more(rows.length - SHOWN),
      ],
    },
  };
}

export function endingRead(d: InsightsLens, days: number, m: Model): LensRead {
  const eyebrow = `Ending soon · next ${days} days`;
  const open = { label: "Open Expiring access", href: `/review/expiring-access?within=${days}` };
  if (!d.holders_observed) return sentence(eyebrow, "Not read yet", `${NOT_READ} Ends appear here once it has.`, "warn");
  if (d.roles.length === 0) return sentence(eyebrow, `Nothing ends in the next ${days} days`, "No access has an end date in this time.");
  const rows = [...d.roles].sort((a, b) => (a.earliest_end ?? "").localeCompare(b.earliest_end ?? "") || b.people - a.people);
  const first = rows.find((r) => r.earliest_end)?.earliest_end;
  return {
    counts: tally(m, rows),
    unit: `lose it within ${days} days`,
    note: `Lit roles have access ending within ${days} days. The number is how many people lose it.`,
    panel: {
      tone: "warn",
      eyebrow,
      title: `Access ends in ${roles(rows.length)}`,
      meta: first ? `The first ends on ${dayLabel(dayOf(new Date(first)))}` : undefined,
      lines: [
        ...rows.slice(0, SHOWN).map((r) => ({
          tone: "warn" as const,
          lead: roleName(m, r),
          text: `ends for ${people(r.people)}${r.earliest_end ? `, the first on ${dayLabel(dayOf(new Date(r.earliest_end)))}` : ""}.`,
        })),
        ...more(rows.length - SHOWN),
      ],
      open,
    },
  };
}

const teamOf = (group: string) => (group.startsWith("team:") ? group.slice(5).trim() : null);

export function groupTitle(group: string): string {
  const team = teamOf(group);
  return group === "left" ? "People who left the makerspace" : group === "new_term" ? "People new this term" : `Team ${team}`;
}

export function groupRead(d: InsightsLens, group: string, m: Model): LensRead {
  const left = group === "left";
  const title = groupTitle(group);
  const size = d.size ?? 0;
  if (size === 0) return sentence("A group", title, "Nobody is in this group right now.");
  const rows = [...d.roles].sort((a, b) => b.people - a.people);
  const holding = d.holders_observed ? ` · ${left ? "still holding" : "holding"} ${roles(rows.length)}` : "";
  return {
    counts: tally(m, rows),
    unit: "in this group",
    note: `Lit roles are held by people in this group. The number is how many.`,
    panel: {
      tone: left ? "warn" : "accent",
      eyebrow: "A group",
      title,
      meta: `${people(size)}${holding}`,
      lines: !d.holders_observed
        ? [{ tone: "ink", text: `${NOT_READ} What they hold appears here once it has.` }]
        : rows.length === 0
          ? [{ tone: "ink", text: "None of them holds a role." }]
          : [
              ...rows.slice(0, SHOWN).map((r) => ({
                tone: left ? ("warn" as const) : ("accent" as const),
                lead: roleName(m, r),
                text: `${r.people} of ${size} ${left ? "still hold it" : "hold it"}.`,
              })),
              ...more(rows.length - SHOWN),
            ],
      open: left ? { label: "Open People", href: "/users" } : undefined,
    },
  };
}

export const unread = (eyebrow: string): LensRead =>
  sentence(eyebrow, "Couldn't read this", "Syndra couldn't read it just now. Nothing was changed.", "warn");

/**
 * The lens's answer once its read has come back, or null while it is coming.
 * `group` is the whole link value: left, new_term or team:<name>.
 */
export function readLens(
  m: Model,
  lens: "use" | "ending" | "group",
  s: { usage: Read<InsightsUsage>; lens: Read<InsightsLens>; days: number; group: string },
): LensRead | null {
  if (lens === "use") return s.usage.data ? usageRead(s.usage.data, m) : s.usage.isError ? unread(RECENT_USE) : null;
  if (lens === "ending") return s.lens.data ? endingRead(s.lens.data, s.days, m) : s.lens.isError ? unread("Ending soon") : null;
  if (teamOf(s.group) === "") return sentence("A group", "Which team?", "Type a team name to see what it holds.");
  return s.lens.data ? groupRead(s.lens.data, s.group, m) : s.lens.isError ? unread("A group") : null;
}
