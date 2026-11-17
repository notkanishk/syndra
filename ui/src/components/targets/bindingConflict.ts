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

/** The default action's label: the system page's own control for `account.release` says it the same way. */
export const STOP_TRACKING = "Stop tracking";

/**
 * The other answers (addon-contract 7t-7, owner ruling 17 Nov 2026). Each label is the
 * confirm button's, and each `says` is the sentence beside it: what happens on the system
 * and in Syndra, true of what the backend does.
 */
export const ADOPT_IT = "It's theirs: adopt it";
// Adoptable means bound to nobody and not the add-on's own, never untouched: so it says what they get.
export const adoptConsequence = (system: string, holder: string, reason?: string) =>
  `Syndra makes ${holder} this person's account on ${system}. They get it as it is now: everything already on it, and its password. From now on Syndra manages it, and Syndra's next change for them sets its groups and sharing. ${
    reason === "name_held" ? `Their old account, if it is still on ${system} under another name, stays there, and Syndra no longer manages it. ` : ""
  }If ${holder} has changed on ${system} since it was checked, nothing is adopted.`;

/** Claimed twice: the account stays one person's, and Syndra stops tracking it for the other. */
export const keepFor = (person: string) => `Keep it for ${person}`;
export const keepConsequence = (system: string, holder: string, keeper: string, released: string) =>
  `${holder} stays ${keeper}'s account on ${system}, and Syndra stops tracking it for ${released}. Nothing on ${system} is changed. ${start(released)}'s next change sets up an account for them on ${system}, or, if their name there is taken, stops and asks you in Pending changes.`;

/** A fresh read of the system: the same pass as Check now. An action, never a decision. */
export const CHECK_AGAIN = "Check again";
export const checkAgainConsequence = (system: string) =>
  `Syndra reads ${system} again now, the same check as Check now. Nothing on ${system} is changed, and any fix it finds waits in Pending changes. Once the accounts are fixed on ${system}, this closes by itself.`;
// Inside the add-on's cooldown it says what every cooldown says (cooldownLine, components/results/copy.ts).
export const stillReported = (system: string) =>
  `Checked just now: ${system} still reports this. Fix it on ${system}, then check again, or decide it another way.`;
export const checkStopped = (said: string) =>
  `The check stopped before it concluded anything. ${said} Check again once that is fixed.`;

/** A check that answered, before the list has been read again. */
export const CHECKED_UPDATING = "Checked. Updating\u2026";

/**
 * A decided adoption or keep: the add-on acted when it was chosen, and the row closes at the next
 * check. If the answer said it didn't finish (Syndra's own records), this finishes it, on the same
 * route, safe to repeat.
 */
export const FINISH_AGAIN_NOTE = "If Syndra said this didn't finish, finish it here. Nothing already done is done twice.";
export const FINISH_REASON = "Finishing a choice that did not complete";
export const finishLabel = (decision: string) => (decision === "adopted" ? "Finish: adopt it" : "Finish: keep it for them");

/** What a decided conflict is waiting for, in place of "waiting for {system} to match". */
export const closesAtNextCheck = (system: string) => `closes when Syndra next checks ${system}`;

const start = (s: string) => s.charAt(0).toUpperCase() + s.slice(1);

export type ConflictAnswer = "unbound" | "adopted" | "kept" | "check";
export interface ConflictChoice {
  answer: ConflictAnswer;
  label: string;
  says: string;
}

/**
 * The answers a conflict takes, safest first: the first is preselected. Stop tracking is the
 * default; Check again comes first only for `ambiguous`, whose fix is on the system itself, and is
 * offered on every conflict. Adoption only where the add-on marked the holder adoptable (the backend
 * refuses it anywhere else); keeping it for one of two people only for `claimed_twice` with the
 * other person named, where keeping it for the other one is this person's Stop tracking.
 * `person` and `other` are names as shown, never ids.
 */
export function conflictChoices(c: BindingConflictReport, system: string, person: string, other: string): ConflictChoice[] {
  const stop: ConflictChoice = { answer: "unbound", label: STOP_TRACKING, says: stopTrackingConsequence(system) };
  const check: ConflictChoice = { answer: "check", label: CHECK_AGAIN, says: checkAgainConsequence(system) };
  const holder = c.username ?? "The account";
  switch (c.reason) {
    case "ambiguous":
      return [check, stop];
    case "claimed_twice":
      return c.bound_to
        ? [
            { answer: "unbound", label: keepFor(other), says: keepConsequence(system, holder, other, person) },
            { answer: "kept", label: keepFor(person), says: keepConsequence(system, holder, person, other) },
            check,
          ]
        : [stop, check];
    case "name_held":
    case "number_held":
      return c.adoptable && c.username
        ? [stop, { answer: "adopted", label: ADOPT_IT, says: adoptConsequence(system, c.username, c.reason) }, check]
        : [stop, check];
    default:
      return [stop, check];
  }
}
