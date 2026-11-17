import { limitFields } from "@/lib/fields";
import { drawnUsage } from "@/components/kinds/storage/state";
import type { MyTargetView } from "@/lib/queries/useMyStorage";
import type { Limits } from "@/lib/usage";

/**
 * The storage kind's limit field: the schema's one limit in bytes (no name written here). Only a
 * limit in bytes can meter a byte count, and only when the schema has exactly one: with two, which
 * is the home's would be a guess, so none is made.
 */
export function storageLimitField(view: MyTargetView): string | undefined {
  const fields = limitFields(view.entitlement_schema).filter((f) => f.unit === "bytes");
  return fields.length === 1 ? fields[0].name : undefined;
}

/**
 * The storage kind's observed limits, for a statistics band. A band is scaled to the limit Syndra
 * READ (`quota_bytes` on the home place), never to a limit it computed. No place read means no
 * entry: "not read", which is not the same as 0, "no limit". The usage is the one the storage card
 * draws: a place the person can change, and nothing while the system is offline.
 */
export function storageLimits(view: MyTargetView): Limits {
  const field = storageLimitField(view);
  const home = view.storage?.places?.find((p) => p.home);
  const usage = home && drawnUsage(home, view.health === "offline");
  if (field === undefined || !usage) return {};
  return { [field]: usage.quota_bytes ?? 0 };
}
