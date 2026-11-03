"use client";

import { Fragment, type ReactNode } from "react";

import { Mono } from "@/components/ui/Badge";

/**
 * A sentence whose `code` spans are mono. The claims part's words are written
 * with backticks once, in `lib/claims.ts`, and drawn here.
 */
export function Ticks({ text }: { text: string }) {
  return (
    <>
      {text.split("`").map((part, i) =>
        i % 2 ? (
          <Mono key={i} className="[overflow-wrap:anywhere]">
            {part}
          </Mono>
        ) : (
          <Fragment key={i}>{part}</Fragment>
        ),
      )}
    </>
  );
}

export interface TokenBlockLine {
  key: string;
  text: string;
  /** " " the same; "+" added by a draft; "−" gone in it. The mark, not the colour, says which. */
  mark?: " " | "+" | "−";
  /** Zitadel's own line (`sub`, `email`): faded, because Syndra doesn't send it. */
  theirs?: boolean;
}

/**
 * The token as Zitadel receives it from Syndra (A-11, A-14 §1): mono on
 * surface-0, one line per top-level claim, in the shape's order. A draft
 * marks each line in a gutter (+ − or blank) so a difference never rests on
 * colour alone. `raw` is debug mode's form: plain JSON, wrapping anywhere,
 * never scrolling sideways.
 */
export function TokenBlock({
  lines,
  raw,
  comment,
  empty,
  className = "",
}: {
  lines?: TokenBlockLine[];
  /** Raw JSON, pretty-printed as-is. */
  raw?: unknown;
  /** A faint `//` line above the claims. */
  comment?: string;
  /** Said when there are no lines: `{}` and why. */
  empty?: ReactNode;
  className?: string;
}) {
  const frame = `rounded-inner border border-line bg-surface-0 px-[18px] py-3.5 font-mono text-[13px] leading-[1.8] ${className}`;
  if (raw !== undefined) {
    return (
      <pre className={`${frame} whitespace-pre-wrap [overflow-wrap:anywhere] text-ink`}>
        {JSON.stringify(raw, null, 2)}
      </pre>
    );
  }
  const marked = (lines ?? []).some((l) => l.mark && l.mark !== " ");
  return (
    <div className={frame} role="group" aria-label="Token">
      {comment && <div className="text-faint">{`// ${comment}`}</div>}
      <div className="text-muted">{"{"}</div>
      {(lines ?? []).length === 0 && <div className="pl-4 text-muted">{empty ?? null}</div>}
      {(lines ?? []).map((line, i) => {
        const tone = line.mark === "−" ? "text-muted line-through decoration-1" : line.theirs ? "text-faint" : "text-ink";
        const gutter = line.mark === "+" ? "text-accent-text" : "text-danger-text";
        return (
          <div key={`${line.mark ?? " "}${line.key}${i}`} className="flex gap-2">
            {marked && (
              <span aria-hidden className={`w-3 flex-none select-none font-semibold ${gutter}`}>
                {line.mark === " " ? "" : line.mark}
              </span>
            )}
            {marked && line.mark && line.mark !== " " && (
              <span className="sr-only">{line.mark === "+" ? "Added: " : "Removed: "}</span>
            )}
            <span className={`min-w-0 [overflow-wrap:anywhere] ${marked ? "" : "pl-4"} ${tone}`}>
              {line.text}
              {i < (lines ?? []).length - 1 ? "," : ""}
            </span>
          </div>
        );
      })}
      <div className="text-muted">{"}"}</div>
    </div>
  );
}
