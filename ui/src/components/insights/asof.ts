import type { Panel } from "@/components/insights/AnswerPanel";
import { dayLabel, dayLong } from "@/components/insights/days";
import type { Model } from "@/components/insights/model";
import type { InsightsAsOf } from "@/lib/queries/useInsights";

const people = (n: number) => `${n} ${n === 1 ? "person" : "people"}`;
const MOST = 3;

/**
 * People holding each role that day, by map node. The backend keys them
 * "project_id:role_key"; the map knows roles by node id.
 */
export function pastHeld(m: Model, rolePeople: Record<string, number>): Map<string, number> {
  const held = new Map<string, number>();
  for (const [key, n] of Object.entries(rolePeople)) {
    const cut = key.indexOf(":");
    const id = cut > 0 ? m.roleId(key.slice(0, cut), key.slice(cut + 1)) : null;
    if (id) held.set(id, n);
  }
  return held;
}

/**
 * What the chosen date says: how far it is from today, in the audit log's
 * count, and the roles whose people number changed. The counts are Zitadel's
 * as Syndra last read them that day, never recomputed.
 */
export function asOfPanel(
  asked: string,
  read: InsightsAsOf,
  today: { model: Model | null; held: Map<string, number> },
  then: Map<string, number>,
): Panel {
  if (read.kind === "none") {
    return {
      tone: "neutral",
      eyebrow: `As of ${dayLabel(asked)}`,
      title: read.first_day ? `No picture for ${dayLabel(asked)}` : "No earlier dates yet",
      lines: [
        {
          tone: "ink",
          text: read.first_day
            ? `Syndra keeps one picture a day, starting ${dayLong(read.first_day)}. There's none for that date.`
            : "Syndra keeps one picture a day, starting today. Come back tomorrow for the first earlier date.",
        },
      ],
    };
  }

  const changed = Array.from(then, ([id, was]) => ({ id, was, now: today.held.get(id) }))
    .filter((r): r is { id: string; was: number; now: number } => r.now != null && r.now !== r.was)
    .sort((a, b) => Math.abs(b.now - b.was) - Math.abs(a.now - a.was));

  return {
    tone: "accent",
    eyebrow: `As of ${dayLabel(asked)}`,
    title: `${read.changes_since} ${read.changes_since === 1 ? "change" : "changes"} since`,
    meta: "Read from the audit log",
    lines: [
      ...(read.day !== asked
        ? [{ tone: "accent" as const, text: `The nearest picture is from ${dayLabel(read.day)}.` }]
        : []),
      ...changed.slice(0, MOST).map((r) => ({
        tone: "accent" as const,
        lead: today.model?.nameOf(r.id) ?? "A role",
        text: `reached ${people(r.was)}. It reaches ${r.now} now.`,
      })),
    ],
    open: { label: "Open the audit log", href: "/audit" },
  };
}
