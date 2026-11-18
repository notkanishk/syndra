import { StatusSeat } from "@/components/ui/StatusSeat";
import { standingWords } from "@/lib/account-standing";
import type { Standing } from "@/lib/queries/useTargets";

/**
 * An account's standing, as the add-on reported it: the state, its own note,
 * and the next step. One component for the member card, the account list and
 * anywhere else a standing is read, so no screen words it differently. The
 * step goes through StatusSeat's own column, so it starts at the word.
 *
 * `step={false}` draws the state and its note only, for a table whose status is
 * a narrow column (D-1 §5 #39): the step is a sentence, and a sentence in a
 * 240px seat is five lines. That table says it with `StandingStep`, on the
 * row's detail line.
 */
export function StandingLine({
  standing,
  system,
  audience,
  step = true,
}: {
  standing: Standing;
  system: string;
  audience: "member" | "operator";
  step?: boolean;
}) {
  const w = standingWords(standing, system, audience);
  return (
    <span className="flex min-w-0 flex-col [overflow-wrap:anywhere]" data-standing>
      <StatusSeat
        tone={w.tone}
        word={w.state}
        note={w.note}
        next={step ? <span className="block text-[13px] leading-[1.5] text-muted [text-wrap:pretty]">{w.next}</span> : undefined}
      />
    </span>
  );
}

/** The standing's next step alone, in the same words, for a row's detail line. */
export function StandingStep({ standing, system, audience }: { standing: Standing; system: string; audience: "member" | "operator" }) {
  return <span data-standing-step>{standingWords(standing, system, audience).next}</span>;
}
