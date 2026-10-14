import { STATUS_TONE, type StatusTone } from "@/components/ui/Badge";
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
        word: "Offline",
        line: `Since ${since}`,
        sentence: `Since ${since}: 3 checks in a row went unanswered. Changes for ${name} wait until it answers.`,
        look: `Look at ${name}.`,
        pips: true,
      };
    }
    case "waiting_to_retry":
      return {
        tone: "warn",
        word: "Waiting to retry",
        sentence: `5 calls in a row failed, so it stopped calling and tries again by itself every 30 seconds. ${name} still answers Syndra's checks, so look at the Syndra server. Changes for ${name} wait, and nothing is lost.`,
        pips: false,
      };
    case "checking": {
      const started = formatClock(health?.started_at);
      return {
        tone: "neutral",
        word: "Checking",
        line: `No check since restart, ${started}`,
        sentence: `Syndra restarted at ${started} and hasn't heard from ${name} since. This clears at its first answer, within about 90 seconds.`,
        pips: true,
      };
    }
    default: {
      const run = missedRun(recent);
      if (run > 0) {
        const at = formatClock(health?.missed_since);
        return {
          tone: "healthy",
          word: "Answering",
          line: `Missed ${run} check${run === 1 ? "" : "s"}, ${at}`,
          sentence: `Missed its last check${run === 1 ? "" : "s"}, ${run === 1 ? "at" : "from"} ${at}. Syndra checks every 30 seconds and calls it offline after 3 in a row. Changes still go out.`,
          pips: true,
        };
      }
      return { tone: "healthy", word: "Answering", pips: false };
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
      {slots.map((s, i) => (
        <span
          key={i}
          aria-hidden
          className={`h-1.5 w-1.5 rounded-pill ${
            s === "answered" ? "bg-healthy" : s === "missed" ? "border border-warn" : "border border-line-strong"
          }`}
        />
      ))}
    </span>
  );
}

/** Dot, word, and the pips when they say something. */
export function ReadingWord({ reading, recent }: { reading: Reading; recent: HealthState["recent"] }) {
  return (
    <span className="inline-flex items-center gap-2">
      <span aria-hidden className={`h-2 w-2 flex-none rounded-pill ${STATUS_TONE[reading.tone].dot}`} />
      <span className={STATUS_TONE[reading.tone].label}>{reading.word}</span>
      {reading.pips && <CheckPips recent={recent} />}
    </span>
  );
}
