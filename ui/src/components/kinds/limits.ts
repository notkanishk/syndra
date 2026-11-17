import { storageLimitField, storageLimits } from "@/components/kinds/storage/limits";
import type { MyTargetView } from "@/lib/queries/useMyStorage";
import type { StatsMetric } from "@/lib/queries/useStats";
import type { Limits } from "@/lib/usage";

/** A kind that can say a person's observed limits (the same registry shape as MemberTargetCard's); a kind with none draws no meter. */
const KIND_LIMITS: Record<string, (view: MyTargetView) => Limits> = { storage: storageLimits };

export const observedLimits = (view: MyTargetView): Limits => KIND_LIMITS[view.kind ?? ""]?.(view) ?? {};

/** The limit field a kind draws its own meter over (storage: the home folder's), named by the schema. */
const KIND_METERED: Record<string, (view: MyTargetView) => string | undefined> = { storage: storageLimitField };

/**
 * The metric a kind draws under its own meter (A-23 R1): the level whose `against` is the field that
 * meter is over. The kind's card draws its month there, and the usage card leaves it out, so it is
 * said once. None for a kind with no meter of its own.
 */
export function meteredMetric(view: MyTargetView): StatsMetric | undefined {
  const field = KIND_METERED[view.kind ?? ""]?.(view);
  if (field === undefined || !view.usage?.reported) return undefined;
  return view.usage.metrics.find((m) => m.instrument === "gauge" && m.against === field);
}
