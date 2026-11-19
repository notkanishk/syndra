import type { ReactNode } from "react";

import { StatusDot, type StatusTone } from "@/components/ui/Badge";
import { Cells, type Columns } from "@/components/ui/Card";
import { FoldChevron } from "@/components/ui/FoldChevron";
import { Unfold } from "@/lib/motion";

/**
 * The columns, which debug mode's headings draw too. On a phone the verdict
 * moves up beside the time (`order`), and the row stacks.
 */
export const SIGN_IN_COLS: Columns = [
  "order-1 w-[28px] flex-none",
  "order-1 w-[96px] flex-none",
  "order-3 flex w-full min-w-0 items-center gap-2.5 tablet:w-auto tablet:min-w-[140px] tablet:flex-[1_1_0%]",
  "order-3 w-full tablet:w-[120px] tablet:flex-none",
  "order-3 min-w-[140px] flex-[2_1_0%]",
  "order-2 ml-auto tablet:order-3 tablet:ml-0 tablet:w-[120px] tablet:flex-none tablet:text-right",
];

/**
 * One captured sign-in in debug mode's live list (A-14 §6): chevron · Time ·
 * Person · Asked for · Syndra answered · Checks. The chevron opens it in place
 * (Unfold) to show its body. On a phone the row stacks: time and verdict on
 * one line, then the person, then what was asked.
 */
export function SignInRow({
  time,
  person,
  asked,
  answered,
  verdict,
  open,
  onToggle,
  children,
}: {
  time: string;
  person: ReactNode;
  asked: string;
  answered: ReactNode;
  verdict: { tone: StatusTone; word: string };
  open: boolean;
  onToggle: () => void;
  children: ReactNode;
}) {
  const check = (
    <span className="inline-flex items-center justify-end gap-2 text-small font-semibold">
      <StatusDot tone={verdict.tone} />
      {verdict.word}
    </span>
  );
  return (
    <div className="row-divider motion-tint">
      <div className="row-cols flex flex-wrap items-center gap-y-1 px-5 py-2.5">
        <Cells cols={SIGN_IN_COLS}>
        <button
          type="button"
          aria-expanded={open}
          aria-label={open ? `Close the sign-in at ${time}` : `Open the sign-in at ${time}`}
          onClick={onToggle}
          className="rounded-pill control press-scale"
        >
          <FoldChevron open={open} />
        </button>
        <span className="type-mono text-small">{time}</span>
        {person}
        <span className="text-body text-muted">{asked}</span>
        <span className="text-small [overflow-wrap:anywhere]">{answered}</span>
        {check}
        </Cells>
      </div>
      <Unfold open={open}>
        <div className="flex flex-col gap-[22px] px-[22px] pb-6 pt-2 tablet:pl-[clamp(22px,4.3vw,62px)]">{children}</div>
      </Unfold>
    </div>
  );
}
