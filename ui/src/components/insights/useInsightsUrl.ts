"use client";

import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useCallback } from "react";

import { isDay, today } from "@/components/insights/days";
import { fromParams } from "@/components/insights/question";
import type { TrendPeriod } from "@/lib/queries/useInsights";

export type Finding = "drift" | "send" | "single" | "unused" | "overlap";
export const FINDINGS: Finding[] = ["drift", "send", "single", "unused", "overlap"];
export const MAX_PINS = 3;
/**
 * What the usage lens and its Trends card are called. Sign-ins are the only
 * source today, so the name says so; the day a badge source connects this
 * becomes "Recent use" (A-06 §7). One constant, so it flips in one place.
 */
export const RECENT_USE = "Recent sign-ins";
export type Lens = "people" | "use" | "ending" | "group";
export const LENSES: Array<{ value: Lens; label: string }> = [
  { value: "people", label: "People" },
  { value: "use", label: RECENT_USE },
  { value: "ending", label: "Ending soon" },
  { value: "group", label: "A group" },
];
/** One window across the product, 14 first: Expiring access, Home and this lens (A-06 §7). */
export const ENDING_DAYS = [14, 30];

/**
 * Everything Insights remembers is in the link: the tab, the question (`q`
 * and the parts it names), Map or List, the pins, the finding and the lens (`lens`, with `days` or `group` where it needs one). Nothing here is component state, so
 * a link opened on a desktop is the map as it was left on a phone.
 */
export function useInsightsUrl() {
  const router = useRouter();
  const pathname = usePathname();
  const params = useSearchParams();
  const raw = params.toString();

  // ponytail: ids are joined with commas; a node id with a comma would split. Real ids never carry one.
  const pins = Array.from(new Set((params.get("pin") ?? "").split(",").filter(Boolean))).slice(0, MAX_PINS);
  const lens = LENSES.find((l) => l.value === params.get("lens"))?.value ?? "people";
  const days = ENDING_DAYS.find((d) => String(d) === params.get("days")) ?? ENDING_DAYS[0];
  // left, new_term or team:<name>; anything else is the default.
  const g = params.get("group") ?? "";
  const group = g === "new_term" || g.startsWith("team:") ? g : "left";
  const finding = FINDINGS.find((f) => f === params.get("finding")) ?? null;

  /** One call changes everything it names, so a click is one history entry. */
  const set = useCallback(
    (changes: Record<string, string | null>) => {
      const next = new URLSearchParams(raw);
      for (const [key, value] of Object.entries(changes)) {
        if (value) next.set(key, value);
        else next.delete(key);
      }
      const qs = next.toString();
      router.replace(qs ? `${pathname}?${qs}` : pathname, { scroll: false });
    },
    [raw, pathname, router],
  );

  const period = (["30d", "year"] as const).find((p) => p === params.get("period")) ?? ("term" as const);
  // A date in the link is only a date if it is one, and not one that has not happened.
  const asof = isDay(params.get("asof")) && params.get("asof")! <= today() ? params.get("asof") : null;

  return {
    period: period as TrendPeriod,
    asof,
    tab: params.get("tab") === "trends" ? ("trends" as const) : ("map" as const),
    question: fromParams((k) => params.get(k)),
    view: params.get("view") === "list" ? ("list" as const) : ("map" as const),
    pins,
    finding,
    lens,
    days,
    group,
    set,
  };
}

export type InsightsUrl = ReturnType<typeof useInsightsUrl>;
