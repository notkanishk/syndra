import { limitFields } from "@/lib/fields";
import { drawnUsage, stateOf, storageOf } from "@/components/kinds/storage/state";
import type { MyTargetView, PersonTargetView } from "@/lib/queries/useMyStorage";
import type { Limits } from "@/lib/usage";

/** The person page draws a home folder's own usage only for a home that exists or is given; otherwise its limit lines stand in. */
export const drawsHome = ({ home }: PersonTargetView): boolean => Boolean(home?.home || home?.gives_home || home?.own);

/**
 * Whether the storage card draws the home folder's month under its meter (A-23 R1) for this view:
 * the one answer the card and the usage card both ask, so the month is said in exactly one of them.
 *
 * - The member's own card (`MyStorage`) draws it in the home place's block: an account that is
 *   theirs, whose places were read, with a home place among them (not "unread", not "opens nothing").
 * - The person page's card (`PersonStorage`, `operator`) draws it in the Home folder row: an account,
 *   no setup step, a read storage (now or the add-on's kept copy), and a home that is drawn.
 */
export function drawsHomeMonth(view: MyTargetView, operator: boolean): boolean {
  if (operator) {
    const person = view as PersonTargetView;
    return !person.setup && Boolean(person.account) && Boolean(storageOf(person)) && drawsHome(person);
  }
  const state = stateOf(view);
  if (state === "none" || state === "held" || state === "waiting" || state === "unread" || state === "opens-nothing" || !view.account) return false;
  const storage = storageOf(view);
  return Boolean(storage?.places_readable && storage.places?.some((p) => p.home));
}

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
 * draws: a place the person can change, and nothing from a read that has no data.
 */
export function storageLimits(view: MyTargetView): Limits {
  const field = storageLimitField(view);
  const home = storageOf(view)?.places?.find((p) => p.home);
  const usage = home && drawnUsage(home);
  if (field === undefined || !usage) return {};
  return { [field]: usage.quota_bytes ?? 0 };
}
