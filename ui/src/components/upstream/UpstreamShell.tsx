"use client";

import Link from "next/link";

import { ActionOutcome } from "@/components/ui/ActionOutcome";
import { ArrowLink } from "@/components/ui/ArrowLink";
import { Button } from "@/components/ui/Button";
import { Checkbox } from "@/components/ui/Choice";
import { ModalFooter } from "@/components/ui/Modal";
import { PageHeader } from "@/components/ui/PageHeader";
import { formatList } from "@/lib/format";
import { OUTCOME_LABEL, REVOKE_SENDS_ITSELF_WORD, succeeded, type ActionOutcome as Outcome } from "@/lib/outcome";
import { sortNames } from "@/lib/sort";

/**
 * The frame every upstream-inspection page shares.
 *
 * These pages read the identity provider directly rather than Syndra's model
 * of it, which is the whole point of them and also the thing most likely to
 * confuse somebody who wandered in from Access. So each one says what it is
 * before it says anything else, and links to where the same question is
 * answered in Syndra's own terms. What a write skips is said beside the write
 * (`DirectWriteWarning`), never as a standing line under the title.
 */
export function UpstreamShell({
  title,
  lede,
  syndraHref,
  syndraLabel,
  children,
}: {
  title: string;
  lede: string;
  syndraHref: string;
  syndraLabel: string;
  children: React.ReactNode;
}) {
  return (
    <div className="flex flex-col gap-4">
      <PageHeader
        eyebrow={
          <Link
            href="/zitadel"
            className="inline-flex min-h-[var(--tap)] items-center font-semibold link-hover desktop:min-h-0"
          >
            Zitadel
          </Link>
        }
        title={title}
        lede={
          <>
            {lede} <ArrowLink href={syndraHref}>{syndraLabel}</ArrowLink>
          </>
        }
      />
      {children}
    </div>
  );
}

/**
 * A direct write's end, in the sheet (A-17 G4): the act's own word on success,
 * the outcome's own label ("Failed", "Refused") when it did not go through.
 * The sheet becomes this; it never closes onto the row.
 */
export function WriteResult({
  outcome,
  word,
  neutral = OUTCOME_LABEL.no_change,
  className = "mx-6 mb-1",
}: {
  outcome: Outcome;
  /** The act, past tense: "Given", "Revoked", "Replaced". */
  word: string;
  /** The word for a neutral end (`no_change`): "Nothing revoked", "Not known". */
  neutral?: string;
  className?: string;
}) {
  const shown =
    outcome.kind === "applied"
      ? word
      : outcome.kind === "no_change"
        ? neutral
        : outcome.sendsItself
          ? REVOKE_SENDS_ITSELF_WORD
          : OUTCOME_LABEL[outcome.kind];
  return <ActionOutcome outcome={outcome} word={shown} className={className} />;
}

/**
 * When a change made here reaches Zitadel (C3). Every give, replace and
 * removal goes straight away unless a role waits for the operator (N14 B,
 * migration 000078), so the caption names the roles that wait, once it knows
 * them. `waiting` is undefined while the list of roles that wait is unread.
 */
export function sendsWhen(waiting: string[] | undefined): string {
  if (waiting === undefined) {
    return "It is sent to Zitadel straight away, unless a role waits for you: that change waits in Pending changes until you send it.";
  }
  if (waiting.length === 0) return "It is sent to Zitadel straight away.";
  const one = waiting.length === 1;
  return `It is sent to Zitadel straight away, except ${formatList(sortNames(waiting))}: ${
    one ? "that role always waits" : "those roles always wait"
  } for you, so ${one ? "its change waits" : "their changes wait"} in Pending changes until you send ${one ? "it" : "them"}.`;
}

/** The footer of a direct-write sheet: Cancel and the act, then after the result one outline Close. */
export function WriteFooter({
  outcome,
  onClose,
  children,
}: {
  outcome: Outcome | null;
  onClose: () => void;
  /** The act (accent, or dangerConfirm). */
  children: React.ReactNode;
}) {
  // Anything but a refusal or a failure happened, or may have: no second press.
  const done = succeeded(outcome);
  return (
    <ModalFooter cancel={done ? undefined : { onClick: onClose }}>
      {done ? (
        <Button variant="outline" onClick={onClose}>
          Close
        </Button>
      ) : (
        children
      )}
    </ModalFooter>
  );
}

/**
 * The standing caveat on anything that writes here. Rendered next to the
 * control rather than once at the top: a warning three scroll-lengths above
 * the button it applies to is a warning nobody reads.
 */
export function DirectWriteWarning({
  what,
  sends,
  acknowledged,
  onAcknowledge,
}: {
  what: string;
  /** Traced only: when it reaches Zitadel ({@link sendsWhen}). */
  sends?: string;
  /**
   * Supply both to gate the action on a tick, which every write on these
   * screens does. These are the least undoable actions in the product — no
   * rehearsal, no cascade preview, no reason — and the ladder's own rule
   * is that the rung is set by what cannot be undone rather than by how many
   * rows are touched. A paragraph above an armed button is not a rung.
   */
  acknowledged?: boolean;
  onAcknowledge?: (next: boolean) => void;
}) {
  // The sentence names what is *missing*, not a quantity. Everywhere else the
  // tick counts what changes; here the count is beside the point and the
  // absence is the entire risk.
  const tick = onAcknowledge ? (
    <label className="mt-2 flex min-h-[var(--tap)] cursor-pointer items-center gap-3 text-[13.5px] text-ink">
      <span className="flex size-[var(--tap)] shrink-0 items-center justify-center desktop:h-auto desktop:w-auto">
        <Checkbox checked={Boolean(acknowledged)} onChange={(event) => onAcknowledge(event.target.checked)} />
      </span>
      <span>
        {/* A record IS written (C3), so the tick names only what is missing. */}
        I understand this changes Zitadel now, with no preview and no reason.
      </span>
    </label>
  ) : null;

  // Every write on these screens is traced: an explanation, so a caption, not a box.
  return (
    <div>
      <p className="max-w-[80ch] text-[13.5px] leading-[1.55] text-muted">
        <strong className="font-semibold text-ink">This goes through Syndra’s record.</strong> {what} The
        grant, an audit line and the change are written together, so it is explained afterwards.{" "}
        {sends ?? sendsWhen(undefined)}
      </p>
      {tick}
    </div>
  );
}
