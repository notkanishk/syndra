import React from "react";

/**
 * Count and status pills. Tone follows the semantic palette and nothing else:
 * neutral for a kind, accent for work, warn for a deadline, danger for
 * something that already went wrong.
 *
 * `hollow` renders the zero form — a count that is currently nothing still
 * occupies its seat rather than disappearing.
 */

type Tone = "neutral" | "accent" | "warn" | "danger" | "dangerSoft" | "warnSoft";

const TONES: Record<Tone, string> = {
  neutral: "bg-tint-2 text-ink/[.82]",
  accent: "bg-accent-dense text-accent-ink",
  warn: "bg-warn text-warn-ink",
  danger: "bg-danger text-danger-ink",
  // A label, not an alarm. `danger` is the solid fill a count wears when the
  // count itself is the bad news; this is for a pill that NAMES something
  // dangerous — a safety role group — beside prose that is already saying so.
  // The drift queue was rendering it inline with these exact values and no
  // name, which is how the same pill ended up bold in one branch and not in
  // the next.
  dangerSoft: "bg-danger-soft text-danger-text",
  // The same argument one tone up: a refused event in an activity list is
  // worth finding, and it is not an alarm going off.
  warnSoft: "bg-warn-soft text-warn-text",
};

export function Badge({
  tone = "neutral",
  hollow = false,
  className = "",
  children,
  ...props
}: React.HTMLAttributes<HTMLSpanElement> & { tone?: Tone; hollow?: boolean }) {
  return (
    <span
      className={`inline-flex items-center rounded-pill px-2.5 py-0.5 text-[12.5px] font-semibold ${
        hollow ? "border border-line-strong text-label" : TONES[tone]
      } ${className}`}
      {...props}
    >
      {children}
    </span>
  );
}

/** A neutral outline pill — bundle membership, an app served, a role group. */
export function Chip({
  className = "",
  children,
  ...props
}: React.HTMLAttributes<HTMLSpanElement>) {
  return (
    <span
      className={`inline-flex items-center rounded-pill bg-tint-2 px-3.5 py-1.5 text-[14px] font-semibold ${className}`}
      {...props}
    >
      {children}
    </span>
  );
}

/** A role key, grant id or claim name. Always mono, never the body face. */
export function Mono({
  className = "",
  children,
  ...props
}: React.HTMLAttributes<HTMLSpanElement>) {
  return (
    <span className={`type-mono ${className}`} {...props}>
      {children}
    </span>
  );
}

/**
 * The tone dot that precedes a status word.
 *
 * Colour never carries a reading on its own here — a word in green and a word
 * in amber are the same word to an operator who cannot tell them apart, and
 * they are the same word in a screenshot printed in grey. The dot gives the
 * reading a second channel: position and presence, which survive both.
 *
 * Shared so the target page's health readings and the add-on index's
 * reachability column are one idiom. They were two.
 */
export const STATUS_TONE = {
  healthy: { dot: "bg-healthy", label: "text-ink" },
  // A reading that is a runtime fact and not a state of health: a target that
  // is registered and has not published a manifest yet.
  //
  // Amber was wrong here and had shipped that way. Amber is a deadline or a
  // broken assumption, and an add-on that has not answered yet is the ordinary
  // first minute of its life — spending the colour on it teaches an operator
  // that the colour does not mean anything, which is the only thing that makes
  // amber useful anywhere else. Lime is worse: nothing has been read, so
  // nothing is healthy.
  neutral: { dot: "bg-faint", label: "text-muted" },
  accent: { dot: "bg-accent", label: "text-accent-text" },
  warn: { dot: "bg-warn", label: "text-warn-text" },
  danger: { dot: "bg-danger", label: "text-danger-text" },
} as const;

export type StatusTone = keyof typeof STATUS_TONE;

export function StatusDot({ tone, className = "" }: { tone: StatusTone; className?: string }) {
  return (
    <span
      aria-hidden
      className={`size-1.5 shrink-0 rounded-pill ${STATUS_TONE[tone].dot} ${className}`}
    />
  );
}

/**
 * The count in a region heading. One seat, three things it can say.
 *
 * A filled number, a hollow zero, or an em dash when the read failed. It holds
 * its place at zero — a region that vanished with its count would be structure
 * moving in response to data, on the page where an operator most needs to know
 * that a quiet region is quiet rather than missing.
 *
 * The em dash is the reason this is not just `Badge`. A failed read and an
 * empty one are different facts and a `0` for the first is a lie: it says the
 * region is empty when nobody could look.
 */
export function CountChip({ n }: { n: number | null | undefined }) {
  if (n === null || n === undefined) {
    return (
      <Badge hollow aria-label="count could not be read">
        <span aria-hidden>—</span>
      </Badge>
    );
  }
  return n === 0 ? <Badge hollow>0</Badge> : <Badge tone="accent">{n}</Badge>;
}
