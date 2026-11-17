import { StatusSeat } from "@/components/ui/StatusSeat";
import { standingWords } from "@/lib/account-standing";
import type { Standing } from "@/lib/queries/useTargets";

/**
 * An account's standing, as the add-on reported it: the state, its own note,
 * and the next step. One component for the member card, the account list and
 * anywhere else a standing is read, so no screen words it differently. The
 * step goes through StatusSeat's own column, so it starts at the word.
 */
export function StandingLine({ standing, system, audience }: { standing: Standing; system: string; audience: "member" | "operator" }) {
  const w = standingWords(standing, system, audience);
  return (
    <span className="flex min-w-0 flex-col [overflow-wrap:anywhere]" data-standing>
      <StatusSeat
        tone={w.tone}
        word={w.state}
        note={w.note}
        next={<span className="block text-[13px] leading-[1.5] text-muted [text-wrap:pretty]">{w.next}</span>}
      />
    </span>
  );
}
