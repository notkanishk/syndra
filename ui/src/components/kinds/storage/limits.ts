import { limitFields } from "@/lib/fields";
import type { Limits } from "@/components/usage/MetricBlock";
import { drawnUsage } from "@/components/kinds/storage/state";
import type { MyTargetView } from "@/lib/queries/useMyStorage";

/**
 * The storage kind's observed limits, for a statistics meter. A meter is drawn
 * over the limit Syndra READ (`quota_bytes` on the home place), never over a
 * limit it computed, and the field it answers for is the schema's own limit
 * field (no name written here). No place read means no entry: "not read", which
 * the meter says, and is not the same as 0, "no limit".
 *
 * Only a limit in bytes can meter a byte count, and only when the schema has
 * exactly one (with two, which is the home's would be a guess, so none is
 * made). The usage is the one the storage card draws: a place the person can
 * change, and nothing while the system is offline.
 */
export function storageLimits(view: MyTargetView): Limits {
  const fields = limitFields(view.entitlement_schema).filter((f) => f.unit === "bytes");
  const home = view.storage?.places?.find((p) => p.home);
  const usage = home && drawnUsage(home, view.health === "offline");
  if (fields.length !== 1 || !usage) return {};
  return { [fields[0].name]: usage.quota_bytes ?? 0 };
}
