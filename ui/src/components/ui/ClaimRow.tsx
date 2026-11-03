import type { ReactNode } from "react";

import { Button } from "@/components/ui/Button";
import { Ticks } from "@/components/ui/TokenBlock";

/**
 * One claim, in the one row form every kind shares (A-14 §4).
 *
 * Line 1: Claim (mono, with "Inside" before it when nested) · Carries · Remove.
 * Line 2: only for a kind that needs it (a fixed value's type, the roles an
 * if-role row checks, a roles row's text format). Line 3, on every row: the
 * exact literal it is sent as. An error is danger text under the row; a
 * warning is warn text.
 */
export function ClaimRow({
  inside,
  name,
  carries,
  onRemove,
  removeLabel,
  second,
  sentAs,
  warn,
  error,
  footer,
}: {
  /** The parent object's name field, for a nested row. */
  inside?: ReactNode;
  name: ReactNode;
  carries: ReactNode;
  onRemove?: () => void;
  removeLabel: string;
  second?: ReactNode;
  /** "Sent as `…` for Ada Lovelace." Backticks are mono. */
  sentAs?: string;
  warn?: string;
  error?: string;
  /** Under the Sent as line: "Put it inside another claim". */
  footer?: ReactNode;
}) {
  return (
    <div className="flex flex-col gap-1.5 px-[22px] pb-3 pt-2">
      <div className="flex flex-wrap items-center gap-2">
        {inside && <span className="flex flex-[0_1_140px] items-center gap-2">{inside}<span aria-hidden className="text-[15px] text-faint">›</span></span>}
        <span className="min-w-[140px] flex-[1_1_140px]">{name}</span>
        <span className="min-w-[190px] flex-[1_1_190px]">{carries}</span>
        {onRemove && (
          <span className="whitespace-nowrap">
            <Button variant="ghost" size="sm" aria-label={removeLabel} onClick={onRemove}>
              Remove
            </Button>
          </span>
        )}
      </div>
      {second && <div className="flex flex-wrap items-center gap-2 text-[13.5px] text-muted">{second}</div>}
      {sentAs && (
        <span className="text-[12.5px] leading-[1.5] text-faint [overflow-wrap:anywhere]">
          <Ticks text={sentAs} />
        </span>
      )}
      {warn && (
        <p className="text-[13px] leading-[1.5] text-warn-text [text-wrap:pretty]">
          <Ticks text={warn} />
        </p>
      )}
      {error && (
        <p role="alert" className="text-[13px] leading-[1.5] text-danger-text [text-wrap:pretty]">
          <Ticks text={error} />
        </p>
      )}
      {footer && <span className="-ml-2.5 inline-flex">{footer}</span>}
    </div>
  );
}
