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
 * 240px seat is five lines. That table says it on the row's detail line, in
 * `rowStep`'s words (`lib/account-standing.ts`), which are silent for a healthy
 * row with nothing to do (D-1 Q5).
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
        next={step ? <span className="block text-small leading-[1.5] text-muted [text-wrap:pretty]">{w.next}</span> : undefined}
      />
    </span>
  );
}