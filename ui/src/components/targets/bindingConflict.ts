/**
 * A binding conflict finding (`binding_conflict`, addon-contract 7t): a person's
 * recorded account names no account, because another one holds its name or its
 * number. The add-on says what happened and what to do (`detail`, `next`) and
 * those words are shown as written; this file holds only Syndra's own sentences
 * around them, which the copy guide (design.md §16) quotes and
 * `bindingConflictCopyGuide.test.ts` keeps in step.
 */

/** The add-on's report, as the finding carries it in `theirs` (common/binding_conflict.json). */
export interface BindingConflictReport {
  reason?: string;
  username?: string;
  account_id?: string;
  holders?: { username: string; account_id: string }[];
  adoptable?: boolean;
  /** The subject already holding the account (`claimed_twice`): a person's id, never a label. */
  bound_to?: string;
  detail?: string;
  next?: string;
}

/** What `theirs` says, read defensively: it is the add-on's text, so anything may be missing. */
export function conflictReport(theirs: unknown): BindingConflictReport {
  if (typeof theirs !== "object" || theirs === null || Array.isArray(theirs)) return {};
  const t = theirs as Record<string, unknown>;
  const text = (v: unknown) => (typeof v === "string" && v.trim() ? v : undefined);
  const holders = Array.isArray(t.holders)
    ? t.holders.flatMap((h) => {
        const one = h as Record<string, unknown> | null;
        const username = text(one?.username);
        return username ? [{ username, account_id: text(one?.account_id) ?? "" }] : [];
      })
    : undefined;
  return {
    reason: text(t.reason),
    username: text(t.username),
    account_id: text(t.account_id),
    holders,
    adoptable: t.adoptable === true,
    bound_to: text(t.bound_to),
    detail: text(t.detail),
    next: text(t.next),
  };
}

/** Said when the add-on's report has no sentence (an older record). */
export const conflictDetailFallback = (system: string) =>
  `This person's account on ${system} can't be matched to one account.`;
export const CONFLICT_NEXT_FALLBACK = "Stop tracking it; the next change creates a new account for them.";

/** `claimed_twice`: the other person, named by the row beside it. */
export const OTHER_PERSON = "The other person is";

/** `ambiguous`: every account that holds the name or number. */
export const holdersLine = (system: string, names: string[]) =>
  `Accounts on ${system} that hold it: ${names.join(", ")}.`;

/** What the one action does, in the sentence next to the button that does it. */
export const stopTrackingConsequence = (system: string) =>
  `Syndra stops tracking this person's account on ${system}. Nothing on ${system} is changed.`;
// What to do next, adopt the holder or not, is the add-on's `next`: this line adds only what stopping
// does to the system, so nothing is said twice and adoptability is never guessed from `reason`.

/** The one action's label: the system page's own control for `account.release` says it the same way. */
export const STOP_TRACKING = "Stop tracking";
