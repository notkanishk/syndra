import type { ReactNode } from "react";

import { StatusDot, type StatusTone } from "@/components/ui/Badge";
import { Unfold } from "@/components/ui/Unfold";

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
    <span className="inline-flex items-center justify-end gap-2 text-[13.5px] font-semibold">
      <StatusDot tone={verdict.tone} />
      {verdict.word}
    </span>
  );
  return (
    <div className="row-divider motion-tint">
      <div className="flex flex-wrap items-center gap-x-[18px] gap-y-1 px-5 py-2.5">
        <button
          type="button"
          aria-expanded={open}
          aria-label={open ? `Close the sign-in at ${time}` : `Open the sign-in at ${time}`}
          onClick={onToggle}
          className="flex size-7 flex-none items-center justify-center rounded-pill border border-line-strong text-[12.5px] text-muted control press-scale"
        >
          <span aria-hidden className={`inline-block motion-tint ${open ? "rotate-90" : ""}`}>
            ›
          </span>
        </button>
        <span className="type-mono w-[96px] flex-none text-[13px]">{time}</span>
        <span className="ml-auto tablet:hidden">{check}</span>
        <span className="flex w-full min-w-0 items-center gap-2.5 tablet:w-[210px] tablet:flex-none">{person}</span>
        <span className="w-full text-[14px] text-muted tablet:w-[120px] tablet:flex-none">{asked}</span>
        <span className="min-w-[150px] flex-1 text-[13.5px] [overflow-wrap:anywhere]">{answered}</span>
        <span className="hidden w-[120px] flex-none text-right tablet:block">{check}</span>
      </div>
      <Unfold open={open}>
        <div className="flex flex-col gap-[22px] px-[22px] pb-6 pt-2 tablet:pl-[clamp(22px,4.3vw,62px)]">{children}</div>
      </Unfold>
    </div>
  );
}
