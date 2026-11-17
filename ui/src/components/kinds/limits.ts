import { STORAGE_USAGE_STEP } from "@/components/kinds/storage/captions";
import { drawsHomeMonth, storageLimitField, storageLimits } from "@/components/kinds/storage/limits";
import type { MyTargetView } from "@/lib/queries/useMyStorage";
import type { StatsMetric } from "@/lib/queries/useStats";
import type { Limits } from "@/lib/usage";

/**
 * What a kind says about a system's statistics, where the generic usage card cannot: a person's
 * observed limits, the metric its own card draws under its meter, and its words for the way to
 * Insights. A kind with none of these has the card's generic words and draws no meter.
 */

/** A kind that can say a person's observed limits (the same registry shape as MemberTargetCard's). */
const KIND_LIMITS: Record<string, (view: MyTargetView) => Limits> = { storage: storageLimits };

export const observedLimits = (view: MyTargetView): Limits => KIND_LIMITS[view.kind ?? ""]?.(view) ?? {};

/**
 * The limit field a kind's card draws a person's month over, when that card will draw it for this
 * view (`operator`: the person page's card, else the member's own); undefined when it will not.
 */
const KIND_METERED: Record<string, (view: MyTargetView, operator: boolean) => string | undefined> = {
  storage: (view, operator) => (drawsHomeMonth(view, operator) ? storageLimitField(view) : undefined),
};

/**
 * The metric a kind's card draws under its own meter (A-23 R1), when it draws it: the level whose
 * `against` is that meter's field. The one answer both sides ask, so the month is drawn in the kind's
 * card exactly when the usage card leaves it out: never twice, never nowhere.
 */
export function meteredMetric(view: MyTargetView, operator: boolean): StatsMetric | undefined {
  const field = KIND_METERED[view.kind ?? ""]?.(view, operator);
  if (field === undefined || !view.usage?.reported) return undefined;
  return view.usage.metrics.find((m) => m.instrument === "gauge" && m.against === field);
}

/**
 * The totals card's step to Insights, Usage in a kind's own words (the owner's ruling of 18 Nov 2026:
 * storage says "each share", which only the storage kind may say); else the card's generic words.
 */
const KIND_USAGE_STEP: Record<string, string> = { storage: STORAGE_USAGE_STEP };

export const usageStep = (kind: string | undefined): string | undefined => KIND_USAGE_STEP[kind ?? ""];
