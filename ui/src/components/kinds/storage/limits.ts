import { limitFields } from "@/lib/fields";
import type { Limits } from "@/components/usage/MetricBlock";
import type { MyTargetView } from "@/lib/queries/useMyStorage";

/**
 * The storage kind's observed limits, for a statistics meter. A meter is drawn
 * over the limit Syndra READ (`quota_bytes` on the home place), never over a
 * limit it computed, and the field it answers for is the schema's own limit
 * field (no name written here). No place read means no entry: "not read", which
 * the meter says, and is not the same as 0, "no limit".
 */
export function storageLimits(view: MyTargetView): Limits {
  const [field] = limitFields(view.entitlement_schema);
  const home = view.storage?.places?.find((p) => p.home);
  if (!field || !home?.usage) return {};
  return { [field.name]: home.usage.quota_bytes ?? 0 };
}
