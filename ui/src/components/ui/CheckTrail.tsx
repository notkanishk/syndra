import type { ReactNode } from "react";

import { StatusDot, type StatusTone } from "@/components/ui/Badge";
import { Ticks } from "@/components/ui/TokenBlock";

export interface TrailCheck {
  tone: StatusTone;
  /** The check's sentence; backticks are mono. */
  sentence: string;
  /** Faint lines under it: why, or what to do. */
  faint?: string[];
  /** The way on, a violet link ending in →. */
  link?: { label: string; href: string };
  /** Anything drawn under the check (check 5's token). */
  children?: ReactNode;
  /** A ring, not a dot: the check reads Zitadel's last answer, not a fresh one. */
  ring?: boolean;
}

/**
 * The five-check trail (A-11, A-14 §1): one dot and one sentence a check, in
 * the order a token is made, each step joined to the next by a quiet line.
 * `compact` is debug mode's form: details only where a check didn't pass.
 */
export function CheckTrail({ checks, compact = false }: { checks: TrailCheck[]; compact?: boolean }) {
  return (
    <ol className="flex flex-col" aria-label="Checks">
      {checks.map((check, i) => {
        const details = !compact || check.tone !== "healthy";
        return (
          <li key={i} className="grid grid-cols-[14px_1fr] gap-x-3">
            <span className="flex flex-col items-center">
              <StatusDot tone={check.tone} ring={check.ring} className="mt-[8px]" />
              {i < checks.length - 1 && <span aria-hidden className="mt-1.5 w-px flex-1 bg-line" />}
            </span>
            <div className={`flex min-w-0 flex-col gap-1 ${i < checks.length - 1 ? "pb-3.5" : ""}`}>
              <span className="text-[14px] leading-[1.5] text-ink [text-wrap:pretty]">
                <span className="sr-only">{`Check ${i + 1}, ${TONE_WORD[check.tone]}: `}</span>
                <Ticks text={check.sentence} />
              </span>
              {details &&
                check.faint?.map((line) => (
                  <span key={line} className="text-[13px] leading-[1.5] text-faint [text-wrap:pretty]">
                    <Ticks text={line} />
                  </span>
                ))}
              {details && check.link && (
                <a href={check.link.href} target="_blank" rel="noreferrer" className="text-[13.5px] font-semibold text-accent-text link-hover">
                  {check.link.label} →
                </a>
              )}
              {check.children && <div className="mt-1.5">{check.children}</div>}
            </div>
          </li>
        );
      })}
    </ol>
  );
}

const TONE_WORD: Record<StatusTone, string> = {
  healthy: "passes",
  neutral: "nothing to check",
  accent: "noted",
  warn: "needs a look",
  danger: "fails",
};
