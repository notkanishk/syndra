import type { Limits } from "@/components/usage/MetricBlock";
import { storageLimits } from "@/components/kinds/storage/limits";
import type { MyTargetView } from "@/lib/queries/useMyStorage";

/** A kind that can say a person's observed limits (the same registry shape as MemberTargetCard's); a kind with none draws no meter. */
const KIND_LIMITS: Record<string, (view: MyTargetView) => Limits> = { storage: storageLimits };

export const observedLimits = (view: MyTargetView): Limits => KIND_LIMITS[view.kind ?? ""]?.(view) ?? {};
