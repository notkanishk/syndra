import React from "react";

/**
 * Count and status pills. Tone follows the semantic palette and nothing else:
 * neutral for a kind, accent for work, warn for a deadline, danger for
 * something that already went wrong.
 *
 * `hollow` renders the zero form — a count that is currently nothing still
 * occupies its seat rather than disappearing.
 */

export type BadgeTone = Tone;
type Tone = "neutral" | "accent" | "warn" | "danger" | "dangerSoft" | "warnSoft";

const TONES: Record<Tone, string> = {
  neutral: "bg-tint-2 text-ink",
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

/**
 * A chosen thing you can put back (A-17 G15): one shape for every picker's
 * "Chosen" row. The chip is 36px with the chosen tint; its × reaches 44px on
 * touch by extending past it, invisibly.
 */
export function ChosenChip({
  children,
  onRemove,
  removeLabel,
  disabled = false,
}: {
  children: React.ReactNode;
  onRemove: () => void;
  /** "Remove Laser cutter": says what goes. */
  removeLabel: string;
  disabled?: boolean;
}) {
  return (
    <span className="chip-in inline-flex h-9 items-center gap-1 rounded-pill border border-accent-line bg-accent-soft pl-3 pr-1 text-[13px] font-semibold">
      {children}
      <button
        type="button"
        disabled={disabled}
        onClick={onRemove}
        aria-label={removeLabel}
        className="group -my-1 -mr-2 flex min-h-[var(--tap)] min-w-[var(--tap)] items-center justify-center desktop:m-0 desktop:min-h-0 desktop:min-w-0"
      >
        <span
          aria-hidden
          className="flex size-7 items-center justify-center rounded-pill text-muted motion-tint group-hover:bg-[var(--hover)]"
        >
          ×
        </span>
      </button>
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
  healthy: { dot: "bg-healthy", edge: "border-healthy", label: "text-ink" },
  // A reading that is a runtime fact and not a state of health: a target that
  // is registered and has not published a manifest yet.
  //
  // Amber was wrong here and had shipped that way. Amber is a deadline or a
  // broken assumption, and an add-on that has not answered yet is the ordinary
  // first minute of its life — spending the colour on it teaches an operator
  // that the colour does not mean anything, which is the only thing that makes
  // amber useful anywhere else. Lime is worse: nothing has been read, so
  // nothing is healthy.
  neutral: { dot: "bg-faint", edge: "border-faint", label: "text-muted" },
  accent: { dot: "bg-accent", edge: "border-accent", label: "text-accent-text" },
  warn: { dot: "bg-warn", edge: "border-warn", label: "text-warn-text" },
  danger: { dot: "bg-danger", edge: "border-danger", label: "text-danger-text" },
} as const;

export type StatusTone = keyof typeof STATUS_TONE;

export function StatusDot({
  tone,
  bloom = false,
  ring = false,
  className = "",
}: {
  tone: StatusTone;
  /** One ring of the dot's own colour, once: only for something that just went well because of the reader (A-05). */
  bloom?: boolean;
  /**
   * Not a fresh read (A-13 §1.4): a 1.5px ring in the tone's own colour, no
   * fill. The colour still says what the system said; the ring says it is not
   * being said now. Every dot inside a `Card stale` rings by itself (the
   * edge colour is always set, so the card's rule only has to empty it).
   */
  ring?: boolean;
  className?: string;
}) {
  const t = STATUS_TONE[tone];
  return (
    <span
      aria-hidden
      data-dot
      className={`size-1.5 shrink-0 rounded-pill motion-tint ${t.edge} ${ring ? "border-[1.5px]" : t.dot} ${bloom ? "bloom" : ""} ${className}`}
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
