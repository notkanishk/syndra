import { STATUS_TONE, StatusDot, type StatusTone } from "@/components/ui/Badge";
import { formatClock } from "@/lib/format";
import type { HealthState, TargetStatus } from "@/lib/queries/useTargets";

/**
 * How a connected system is doing, in A-01's words, for the Connected systems
 * row and the system page alike. The backend ranks the readings (`status`), so
 * no screen can send an operator to the Syndra server while TrueNAS is down.
 *
 * "Paused" is only ever a person's choice; Syndra's own stops say what's
 * happening.
 */
export interface Reading {
  tone: StatusTone;
  /** The status word beside the dot. */
  word: string;
  /** The faint line under it on a row. */
  line?: string;
  /** The sentence on the system page. */
  sentence?: string;
  /** Where to look, when the reading is a fault. */
  look?: string;
  /** Show the last three checks. */
  pips: boolean;
}

/**
 * One word per state, for every system in the System group (A-20 P5): the
 * Connected systems row, a system's page, Zitadel's page and a member's own
 * storage card all say these and no others.
 */
export const SYSTEM_WORDS = {
  answering: "Answering",
  notAnswering: "Not answering",
  offline: "Offline",
  waitingToRetry: "Waiting to retry",
  notConnected: "Not connected",
} as const;

/** Misses in a row at which a system is Offline rather than merely Not answering. */
export const OFFLINE_AFTER = 3;

/**
 * Zitadel's reading, in the same words. Its health endpoint answers or it does
 * not, so "missed" is a poll that did not come back; `misses` is the run of
 * those the page has seen. One or two is Not answering, with the pips; three is
 * Offline and Syndra keeps checking.
 */
export function zitadelReading({
  status,
  misses,
}: {
  status?: string;
  misses: number;
}): { tone: StatusTone; word: string; offline: boolean } {
  if (status === "ok") return { tone: "healthy", word: SYSTEM_WORDS.answering, offline: false };
  if (status === "disabled") return { tone: "warn", word: SYSTEM_WORDS.notConnected, offline: false };
  if (misses >= OFFLINE_AFTER) return { tone: "warn", word: SYSTEM_WORDS.offline, offline: true };
  return { tone: "warn", word: SYSTEM_WORDS.notAnswering, offline: false };
}

/** Trailing misses in the last three checks: the current run. */
function missedRun(recent: HealthState["recent"]): number {
  let n = 0;
  for (let i = recent.length - 1; i >= 0 && recent[i] === "missed"; i--) n++;
  return n;
}

export function targetReading({
  status,
  health,
  name,
  transportError,
}: {
  status?: TargetStatus;
  health?: HealthState;
  name: string;
  transportError?: string;
}): Reading {
  const recent = health?.recent ?? [];
  switch (status) {
    case "cannot_connect":
      return {
        tone: "danger",
        word: "Cannot connect",
        sentence: `Syndra cannot read the key it signs requests with${transportError ? `: ${transportError}` : ""}.`,
        look: `Look at the Syndra server, not at ${name}.`,
        pips: false,
      };
    case "offline": {
      const since = formatClock(health?.reachable_since);
      return {
        tone: "warn",
        word: SYSTEM_WORDS.offline,
        line: `Since ${since}`,
        sentence: `Since ${since}: 3 checks in a row went unanswered. Changes for ${name} wait until it answers.`,
        look: `Look at ${name}.`,
        pips: true,
      };
    }
    case "waiting_to_retry":
      return {
        tone: "warn",
        word: SYSTEM_WORDS.waitingToRetry,
        sentence: `5 calls in a row failed, so it stopped calling and tries again by itself every 30 seconds. ${name} still answers Syndra's checks, so look at the Syndra server. Changes for ${name} wait, and nothing is lost.`,
        pips: false,
      };
    case "checking": {
      // The API omits a start time it does not have: no check yet, and no clock to name.
      const started = health?.started_at ? formatClock(health.started_at) : null;
      return {
        tone: "neutral",
        word: "Checking",
        line: started ? `No check since restart, ${started}` : "Not checked yet",
        sentence: `${started ? `Syndra restarted at ${started} and hasn't` : "Syndra hasn't"} heard from ${name} since. This clears at its first answer, within about 90 seconds.`,
        pips: true,
      };
    }
    default: {
      const run = missedRun(recent);
      if (run > 0) {
        const at = formatClock(health?.missed_since);
        return {
          tone: "healthy",
          word: SYSTEM_WORDS.answering,
          line: `Missed ${run} check${run === 1 ? "" : "s"}, ${at}`,
          sentence: `Missed its last check${run === 1 ? "" : "s"}, ${run === 1 ? "at" : "from"} ${at}. Syndra checks every 30 seconds and calls it offline after 3 in a row. Changes still go out.`,
          pips: true,
        };
      }
      return { tone: "healthy", word: SYSTEM_WORDS.answering, pips: false };
    }
  }
}

/**
 * The last three checks, oldest first: lime filled answered, amber ring
 * missed, grey ring no check yet. A ring means not a fresh read (86c).
 */
export function CheckPips({ recent }: { recent: HealthState["recent"] }) {
  const slots: Array<"answered" | "missed" | "none"> = [...recent.slice(-3)];
  while (slots.length < 3) slots.push("none");
  const said = slots.map((s) => (s === "none" ? "no check yet" : s)).join(", ");
  return (
    <span role="img" aria-label={`Last 3 checks: ${said}`} className="inline-flex items-center gap-[3px]">
      {slots.map((s, i) =>
        s === "answered" ? (
          <StatusDot key={i} tone="healthy" />
        ) : (
          <StatusDot key={i} tone={s === "missed" ? "warn" : "neutral"} ring />
        ),
      )}
    </span>
  );
}

/** Dot, word, and the pips when they say something. */
export function ReadingWord({ reading, recent }: { reading: Reading; recent: HealthState["recent"] }) {
  return (
    <span className="inline-flex items-center gap-2">
      <StatusDot tone={reading.tone} />
      <span className={STATUS_TONE[reading.tone].label}>{reading.word}</span>
      {reading.pips && <CheckPips recent={recent} />}
    </span>
  );
}
