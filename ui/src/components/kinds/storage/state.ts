import { accountField } from "@/lib/fields";
import { isStorage } from "@/lib/nav";
import type { MyTargetView, StoragePlace } from "@/lib/queries/useMyStorage";

/**
 * Where a member is on a storage system, as the storage card's board states
 * name it. The card draws from it, and so does anything else that speaks of
 * a storage step (the member page's header line), so the two never disagree.
 */
export type State =
  | "none"
  | "held"
  | "waiting"
  | "needs-password"
  | "reenrol"
  | "restored"
  | "ready"
  | "unread"
  | "opens-nothing"
  | "nohost";

/**
 * The pause on the account itself, found by the schema's account ROLE, never
 * by a field's name: another storage add-on may call it anything.
 */
export function isHold(view: MyTargetView, s: { field: string }): boolean {
  return s.field === accountField(view.entitlement_schema)?.name;
}

/** Where this member is, as one of the board's states. */
export function stateOf(view: MyTargetView): State {
  const suspended = view.suspended ?? [];
  if (!view.entitled) {
    return suspended.some((s) => isHold(view, s)) ? "held" : "none";
  }
  if (!view.account) return "waiting";
  // Asked first (design §13.6): a system that cannot be read says so, and nobody
  // is told to set a password on it (the form would be refused, nothing sent).
  if (!view.read || view.read.state === "none") return "unread";
  const storage = view.read.data.storage;
  if (storage?.needs_new_password) return "restored";
  if (view.credential.needs_re_enrolment) return "reenrol";
  if (storage?.needs_password || (!storage && !view.credential.set)) return "needs-password";
  if (storage?.places_readable && (storage.places ?? []).length === 0) return "opens-nothing";
  // The read answered without the storage status: what the account opens was not told.
  if (!storage) return "unread";
  if (!view.connection) return "nohost";
  return "ready";
}

/** The read this module draws itself, from `view.storage`; any other read goes to `Reads`. */
export const OWN_READS = ["storage.status"];

export const PASSWORD_STEP = new Set<State>(["needs-password", "reenrol", "restored"]);

/**
 * A storage account that is theirs and wants a password set: the storage
 * card's password step, and only for a system of the storage kind. A door or
 * a laser has no password a member sets, so it never asks for one.
 */
export function needsPassword(view: MyTargetView): boolean {
  return isStorage(view) && PASSWORD_STEP.has(stateOf(view));
}

/** The monitor holds the system down (either outage, design §13.2): nothing can be sent to it now. */
export const down = (view: MyTargetView) => view.health === "addon_unreachable" || view.health === "app_unreachable";

/** The add-on's storage status, only under a read that has data (design §13.6). */
export const storageOf = (view: MyTargetView) => (view.read && view.read.state !== "none" ? view.read.data.storage : undefined);

/** The usage a member's card draws for one place: only a place they can change. */
export function drawnUsage(place: StoragePlace): StoragePlace["usage"] {
  return place.access === "change" ? place.usage : undefined;
}

/**
 * Whether a card shows any usage at all. Pass the usage figures the card
 * DRAWS, and none for a place or folder it does not: the unread-usage notice
 * says "the usage above is what was read" only when some is above it.
 */
export function usageShown(drawn: ReadonlyArray<unknown>): boolean {
  return drawn.some((u) => u != null);
}
