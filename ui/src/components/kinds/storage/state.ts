import { accountField } from "@/lib/fields";
import { isStorage } from "@/lib/nav";
import type { MyTargetView } from "@/lib/queries/useMyStorage";

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
  const storage = view.storage;
  if (storage?.needs_new_password) return "restored";
  if (view.credential.needs_re_enrolment) return "reenrol";
  if (storage?.needs_password || (!storage && !view.credential.set)) return "needs-password";
  if (storage?.places_readable && (storage.places ?? []).length === 0) return "opens-nothing";
  // The target's own answer about this account is missing: a read that failed,
  // not an account that is fine. Offline is its own sentence ("ready", below).
  if (!storage && view.health !== "offline") return "unread";
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
