import type { StatusTone } from "@/components/ui/Badge";
import { formatList } from "@/lib/format";
import { DRAIN_UNKNOWN_NEXT, NOTHING_TO_DO, SENDS_ITSELF_NEXT, type ActionOutcome, type AutoRetry } from "@/lib/outcome";
import type { BulkOutcome, BulkPlan } from "@/lib/queries/useBulkGrants";
import { RETRYING, refusedNext, retryLine, retryNextStep, type RetryState } from "@/lib/retry";
import { sortByName, sortNames } from "@/lib/sort";
import { refusalCause, waitIsChosen } from "@/lib/wait-reason";

/**
 * A plan grouped by how each change travels (A-21 §Q1).
 *
 * The reader asks a plan two things: after I press Apply, is anything still
 * mine to do; and what happens to each person. The folds answer the first,
 * the rows the second. People, not writes: one row per person, under the fold
 * of the step left for them, so a person with any role that waits sits under
 * the wait fold and their row says which wait and which go now.
 *
 * Pure: `RehearsalDialog` and `PlanReview` call these and draw nothing of
 * their own, so the folds, the before-line, the button and the result are one
 * reading of the same rows.
 */

export type FoldKey = "go" | "wait" | "unanswered" | "refused" | "left";

/**
 * The folds in their fixed order, each with its word before Apply and after
 * (the tense crossing). Two exist only after: nothing has failed to answer or
 * been refused before anything was sent.
 */
export const GROUPS: ReadonlyArray<{ key: FoldKey; before?: string; after: string }> = [
  { key: "go", before: "Straight away", after: "Done" },
  { key: "wait", before: "Will wait for you to send", after: "Waiting for you to send" },
  { key: "unanswered", after: "Zitadel didn't answer" },
  { key: "refused", after: "Zitadel refused" },
  { key: "left", before: "Left as they are", after: "Left as they were" },
];

export interface PlanGroup {
  key: FoldKey;
  heading: string;
  /** "1 person" / "4 people", in the plan's own noun. */
  count: string;
  dot?: StatusTone;
  /** Done's dot blooms once when every written row reached Zitadel. */
  bloom?: boolean;
  /** After Apply, the wait fold's heading is the link to Pending changes. */
  link?: boolean;
  /** The faint line under the heading: why they wait, or when Syndra tries again. */
  note?: string;
  rows: BulkOutcome[];
}

/** The applyBlocked reason, and the before-line, when no row acts. */
export const NOTHING_TO_APPLY = "Nothing here would change, so there is nothing to apply.";

/** The step a result with rows Zitadel refused leaves (N1): each row names its own. */
export const REFUSED_ROWS_NEXT = "Follow the step on each row Zitadel refused, below.";

/**
 * The operations whose queued rows drain themselves.
 *
 * The backend's rule is on the OUTBOX ROW (`op_type = 'revoke' OR
 * withdraws_only`), and this set is the client's honest reading of which plans
 * produce only such rows. `delete_mapping` only takes access away, so its rows
 * drain on the background runner. `rollback_mappings` restores a SET, so it
 * both grants and revokes and keeps the safe "go and send it" reading:
 * under-claiming sends somebody to a queue that turns out to be empty, while
 * over-claiming tells them a door is locked when it is open.
 */
export const REVOKING_OPS: ReadonlySet<BulkPlan["op"]> = new Set<BulkPlan["op"]>([
  "remove_role",
  "remove_bundle",
  "delete_mapping",
]);

/**
 * Whether a queued row goes on its own (re-audit R2): true when Syndra sends
 * it without anyone (a retry still trying, a revocation), false when it waits
 * for Send, null when that can't be known yet (the indicators unread, N3).
 * The server's own retry state says it whatever the indicators say. With the
 * background drain off (`auto` false) nothing goes by itself.
 */
export function goesItself(o: BulkOutcome, op: BulkPlan["op"], auto: AutoRetry = true): boolean | null {
  if (o.wait_reason === RETRYING) return o.retry ? o.retry.state === "retrying" : auto;
  return waitIsChosen(o.wait_reason) || !(REVOKING_OPS.has(op) || o.sends_on_its_own) ? false : auto;
}

/**
 * The last sentence of a row's consequence is the backend's: "Cancels a change
 * waiting for you to send: the removal of … queued 12 Oct (reason)."
 * (`services.DescribeCancels`). The plan says it once more under the rows, and
 * splits it from whatever else the row said. An older backend said "waiting to
 * be sent"; both are read.
 */
const CANCELS =
  /(?:^|\s)Cancels (a change|(\d+) changes) waiting\s+(?:to\s+be\s+sent|for\s+you\s+to\s+send): ([\s\S]*?)\.?$/;

export function splitCancels(consequence: string | undefined): { rest: string; count: number; parts: string; line: string } | null {
  const match = consequence ? CANCELS.exec(consequence) : null;
  if (!consequence || !match) return null;
  const count = match[2] ? Number(match[2]) : 1;
  const parts = match[3].replace(/ \(([^()]*)\)/g, " “$1”");
  return {
    rest: consequence.slice(0, match.index).trim(),
    count,
    parts,
    line: `Cancels ${count === 1 ? "a change that is" : `${count} changes that are`} waiting for you to send: ${parts}.`,
  };
}

/** How many queued changes the row cancels. */
export const cancelsOf = (o: BulkOutcome): number => splitCancels(o.consequence)?.count ?? 0;

/** A row that only cancels a waiting change and sends nothing (A-21 §2.4): its words say the whole story. */
export const onlyCancels = (o: BulkOutcome): boolean => o.delivery === "none" && cancelsOf(o) > 0;

const acts = (o: BulkOutcome) => o.effect !== "no_change" && o.effect !== "blocked";

/**
 * Whether the plan can be grouped: every row that goes ahead says how it
 * travels. A plan where some do and some don't is drawn flat, so no op can
 * half-claim "Straight away" (A-21 §2.9); the ops that don't go through the
 * planner (mappings, entitlements, marking drift, declining) are flat today.
 */
export function groupable(plan: BulkPlan): boolean {
  const rows = plan.outcomes ?? [];
  return rows.length > 0 && rows.filter(acts).every((o) => Boolean(o.delivery));
}

/** The fold a row sits in, before or after Apply. Every (effect, delivery) pair lands in exactly one. */
export function foldOf(o: BulkOutcome, plan: Pick<BulkPlan, "op" | "applied">, auto: AutoRetry = true): FoldKey {
  switch (o.effect) {
    case "no_change":
    case "blocked":
      return "left";
    case "failed":
      return "refused";
    case "queued":
      return !plan.applied || goesItself(o, plan.op, auto) === false ? "wait" : "unanswered";
    case "applied":
      // A rule's role that waits is the person's step left (A-21 §4.2): the
      // act went, and they still sit under the wait fold.
      return o.rule_waiting ? "wait" : "go";
    case "apply":
      return o.delivery === "waits_for_you" ? "wait" : "go";
  }
}

const count = (n: number, noun: [string, string]) => `${n} ${n === 1 ? noun[0] : noun[1]}`;

/** "Because Trained in Laser Lab always waits for you." over the fold's waiting roles, alphabetical. */
function causeLine(rows: BulkOutcome[]): string | undefined {
  const roles = sortNames([...new Set(rows.flatMap((o) => o.waits ?? []))]);
  if (roles.length === 0) return undefined;
  return `Because ${formatList(roles)} always ${roles.length === 1 ? "waits" : "wait"} for you.`;
}

/** The fold's first retry, which says when Syndra tries next. */
const firstRetry = (rows: BulkOutcome[]): RetryState | undefined => rows.find((o) => o.retry)?.retry;

/**
 * The plan's folds, in their fixed order, each row once and alphabetical
 * inside its fold. A fold with nobody in it is not drawn. Null when the plan
 * can't be grouped ({@link groupable}): it is drawn flat.
 */
export function groupPlan(
  plan: BulkPlan,
  noun: [string, string] = ["person", "people"],
  auto: AutoRetry = true,
  system: string = "Zitadel",
): PlanGroup[] | null {
  if (!groupable(plan)) return null;
  const by = new Map<FoldKey, BulkOutcome[]>();
  for (const o of plan.outcomes) {
    const key = foldOf(o, plan, auto);
    by.set(key, [...(by.get(key) ?? []), o]);
  }
  const reached = !["wait", "unanswered", "refused"].some((k) => by.has(k as FoldKey));
  return GROUPS.flatMap(({ key, before, after }) => {
    const rows = by.get(key);
    const heading = plan.applied ? after.replace("Zitadel", system) : before;
    if (!rows || !heading) return [];
    const group: PlanGroup = {
      key,
      heading,
      count: count(rows.length, noun),
      rows: sortByName(rows, (o) => o.name || o.email || "", (o) => o.user_id),
    };
    switch (key) {
      case "go":
        if (plan.applied) Object.assign(group, { dot: "healthy", bloom: reached });
        break;
      case "wait":
        Object.assign(group, plan.applied ? { link: true } : { dot: "warn" }, { note: causeLine(rows) });
        break;
      case "unanswered": {
        const retry = firstRetry(rows);
        Object.assign(group, { dot: "warn", note: retry ? retryLine(retry, system, rows.length === 1 ? "it" : "them") : undefined });
        break;
      }
      case "refused":
        group.dot = "danger";
        break;
    }
    return [group];
  });
}

const changes = (n: number) => `${n} ${n === 1 ? "change" : "changes"}`;

/**
 * How many changes a row sends now and how many wait, counted from its roles
 * (browser pass, 8 Nov). A row's delivery is "waits_for_you" when ANY of its
 * roles waits, so counting rows by delivery called a bundle of three roles,
 * one of which waits, three changes that wait. A row with no role lists (a
 * single act, an older backend) is one change of its delivery.
 */
export function travels(o: Pick<BulkOutcome, "delivery" | "waits" | "goes">): { now: number; wait: number } {
  const now = o.goes?.length ?? 0;
  const wait = o.waits?.length ?? 0;
  if (now + wait > 0) return { now, wait };
  return { now: o.delivery === "straight_away" ? 1 : 0, wait: o.delivery === "waits_for_you" ? 1 : 0 };
}

/**
 * The footer's before-line (A-21 §2.5): whether this finishes now or waits, by
 * changes, never by people (a person with one role that waits still has the
 * rest sent now). Always the same shape; it replaces the retired note that
 * counted the rows the headings now carry. Undefined when there is nothing to
 * say (a flat plan, or one where nothing reaches Zitadel or waits).
 */
export function beforeLine(plan: BulkPlan, system: string = "Zitadel"): string | undefined {
  if (plan.applied) return undefined;
  if (plan.summary?.apply === 0) return NOTHING_TO_APPLY;
  if (!groupable(plan)) return undefined;
  const going = plan.outcomes.filter((o) => o.effect === "apply");
  let sends = 0;
  let waits = 0;
  for (const o of going) {
    const t = travels(o);
    sends += t.now;
    waits += t.wait;
  }
  const cancelOnly = going.filter(onlyCancels).length;
  const said: string[] = [];
  if (sends > 0 && waits === 0) said.push(`Every change goes to ${system} when you apply.`);
  else if (sends === 0 && waits > 0)
    said.push(`Every change waits for you to send. Nothing reaches ${system} until you press Send on Pending changes.`);
  else if (sends > 0) said.push(`Apply sends ${changes(sends)} to ${system} now. ${waits} ${waits === 1 ? "waits" : "wait"} for you to send.`);
  if (cancelOnly === 1) said.push("For 1, this only cancels a change that's waiting; nothing is sent.");
  else if (cancelOnly > 1) said.push(`For ${cancelOnly}, this only cancels changes that are waiting; nothing is sent.`);
  return said.join(" ") || undefined;
}

/**
 * The Apply button's words: the number, and when the plan cancels, that too,
 * in the cancel step's own shape ("Cancel 2 and apply to 12 people").
 */
export function applyLabel(plan: BulkPlan, noun: [string, string]): string {
  const n = plan.summary?.apply;
  // A count that did not arrive is not a count: never "Apply to undefined people".
  if (typeof n !== "number") return "Apply the change";
  const cancels = (plan.outcomes ?? []).reduce((k, o) => k + cancelsOf(o), 0);
  // Nobody to apply to: the button is held, and the line under it says why.
  if (n === 0) return "Apply";
  return cancels > 0 ? `Cancel ${cancels} and apply to ${count(n, noun)}` : `Apply to ${count(n, noun)}`;
}

/** The result block's word: the act, in the past tense, counting people written. */
function resultWord(op: BulkPlan["op"], written: number, total: number, noun: [string, string]): string | undefined {
  if (written === 0) return undefined;
  const who = written === total ? count(written, noun) : `${written} of ${count(total, noun)}`;
  switch (op) {
    case "assign_role":
    case "assign_bundle":
      return `Given to ${who}`;
    case "remove_role":
    case "remove_bundle":
      return `Revoked from ${who}`;
    case "extend":
      return `Extended for ${who}`;
    default:
      return undefined;
  }
}

/**
 * What an applied grouped plan says (A-21 §2.6): one clause per fold that has
 * people in it, in the fold order, and one step. Null for a flat plan, which
 * keeps the dialog's own result.
 */
export function resultOutcome(
  plan: BulkPlan,
  noun: [string, string] = ["person", "people"],
  auto: AutoRetry = true,
  system: string = "Zitadel",
): { outcome: ActionOutcome; word?: string } | null {
  const groups = plan.applied ? groupPlan(plan, noun, auto, system) : null;
  if (!groups) return null;
  const rows = (key: FoldKey) => groups.find((g) => g.key === key)?.rows ?? [];
  const done = rows("go").length;
  const wait = rows("wait").length;
  const unanswered = rows("unanswered");
  const refused = rows("refused").length;
  const written = done + wait + unanswered.length;
  const unsureRows = unanswered.some((o) => goesItself(o, plan.op, auto) !== true);

  // The clauses count changes, not people (browser pass, 8 Nov): a person in
  // the wait fold for one role had the rest sent straight away, and "Nothing
  // has reached Zitadel yet" said otherwise. Each row's roles say which went
  // ({@link travels}); a row that says nothing of its roles is one change.
  let inZitadel = 0;
  let waiting = 0;
  let again = 0;
  let unsure = 0;
  let didntGo = 0;
  for (const o of rows("go")) {
    const t = travels(o);
    inZitadel += Math.max(1, t.now + t.wait);
  }
  for (const o of rows("wait")) {
    const t = travels(o);
    const listed = (o.goes?.length ?? 0) + (o.waits?.length ?? 0) > 0;
    if (o.effect === "applied") {
      // Its own act took effect; what a rule gave with it waits (N4). Without
      // role lists nothing says the act sent anything (an adoption sends
      // nothing), so only the rule's waiting roles are counted.
      inZitadel += listed ? t.now : 0;
      waiting += listed ? t.wait : (o.rule_waiting ?? 1);
    } else if (listed && t.wait > 0) {
      waiting += t.wait;
      // The roles sent with it went, unless the send left them to a retry.
      // A halt with no retry counts its own unsent roles (`unsent`).
      if (!o.retry) {
        const left = Math.min(o.unsent ?? 0, t.now);
        inZitadel += t.now - left;
        didntGo += left;
      }
      else if (o.retry.state === "retrying") again += t.now;
      else unsure += t.now;
    } else {
      // A send held for you (halted): every change on the row waits.
      waiting += Math.max(1, t.now);
    }
  }
  for (const o of unanswered) {
    const t = travels(o);
    const n = Math.max(1, t.now + t.wait);
    if (goesItself(o, plan.op, auto) === true) again += n;
    else unsure += n;
  }

  const settled = again === 0 && unsure === 0 && didntGo === 0 && refused === 0;
  const said: string[] = [];
  if (inZitadel > 0)
    said.push(
      waiting === 0 && settled && inZitadel > 1 ? `All ${inZitadel} are in ${system} now.` : `${inZitadel} ${inZitadel === 1 ? "is" : "are"} in ${system} now.`,
    );
  if (waiting > 0) {
    said.push(
      inZitadel === 0 && settled
        ? `${waiting === 1 ? "It waits" : `All ${waiting} wait`} for you to send. Nothing has reached ${system} yet.`
        : `${waiting} ${waiting === 1 ? "waits" : "wait"} for you to send.`,
    );
  }
  if (again === 1) said.push(`1 isn't in ${system} yet; Syndra sends it again on its own.`);
  else if (again > 1) said.push(`${again} aren't in ${system} yet; Syndra sends them again on its own.`);
  if (didntGo > 0) said.push(`${didntGo} didn't go to ${system}.`);
  if (unsure > 0) said.push(`${unsure} ${unsure === 1 ? "isn't" : "aren't"} in ${system} yet.`);
  if (refused === 1) said.push(`${system} refused 1; the row says why.`);
  else if (refused > 1) said.push(`${system} refused ${refused}; each row says why.`);
  if (said.length === 0) said.push("Nobody's access changed.");
  const message = said.join(" ");
  const word = resultWord(plan.op, written, written + refused, noun);

  if (refused > 0) return { outcome: { kind: "failed", recorded: true, message, next: REFUSED_ROWS_NEXT.replace("Zitadel", system) }, word };
  if (wait > 0) return { outcome: { kind: "queued", message }, word };
  if (unanswered.length > 0) {
    const retry = firstRetry(unanswered);
    const it = unanswered.length === 1 ? "it" : "them";
    const next = unsureRows ? DRAIN_UNKNOWN_NEXT : retry ? retryNextStep(retry, { it }) : SENDS_ITSELF_NEXT;
    return { outcome: { kind: "queued", message, sendsItself: true, next }, word };
  }
  return { outcome: { kind: "applied", message, next: NOTHING_TO_DO }, word: word ?? (written === 0 ? "Done" : undefined) };
}

/**
 * A refused row's seat (A-21 §2.6, fold 4): what Zitadel said and the one true
 * step, from the row's wait reason. The row keeps its plan words above it.
 */
export function refusedSeat(o: BulkOutcome, op: BulkPlan["op"], system: string = "Zitadel"): string {
  const reason = o.wait_reason ?? "refused:other";
  return `${system} refused this: ${refusalCause(reason)}. ${refusedNext(reason, { revocation: REVOKING_OPS.has(op) })}`;
}
