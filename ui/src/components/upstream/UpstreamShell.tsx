"use client";

import Link from "next/link";

/**
 * The frame every upstream-inspection page shares.
 *
 * These pages read the identity provider directly rather than Syndra's model
 * of it, which is the whole point of them and also the thing most likely to
 * confuse somebody who wandered in from Access. So each one says what it is
 * before it says anything else, and links back to where the same question is
 * answered in Syndra's own terms.
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
    <div className="flex flex-col gap-[18px]">
      <div>
        <Link
          href="/zitadel"
          // Standalone navigation, not a link in a sentence: this is the
          // back affordance on these two screens, and on a phone it is what a
          // thumb reaches for first.
          className="inline-flex min-h-[44px] items-center text-[13.5px] font-semibold text-accent-text motion-tint hover:brightness-110"
        >
          ← Identity provider
        </Link>
        <h1 className="mt-2 type-page-title">{title}</h1>
        {/* A standing line, not only per-action ceremony. A warning that
            appears beside each button is met at the moment an operator has
            already decided, which teaches them to dismiss it. A line under the
            title tells them where they are before they decide anything. Those
            are two different jobs and both are wanted. */}
        <p className="mt-1.5 text-[13.5px] font-semibold text-warn-text">
          Changes here go straight to Zitadel.
        </p>
        <p className="mt-2 max-w-[80ch] text-[14.5px] leading-[1.55] text-muted">
          {lede}{" "}
          <Link href={syndraHref} className="font-semibold text-accent-text">
            {syndraLabel} →
          </Link>
        </p>
      </div>
      {children}
    </div>
  );
}

/**
 * The standing caveat on anything that writes here. Rendered next to the
 * control rather than once at the top: a warning three scroll-lengths above
 * the button it applies to is a warning nobody reads.
 */
export function DirectWriteWarning({
  what,
  acknowledged,
  onAcknowledge,
}: {
  what: string;
  /**
   * Supply both to gate the action on a tick, which every write on these
   * screens does. These are the least undoable actions in the product — no
   * rehearsal, no cascade preview, no ledger row — and the ladder's own rule
   * is that the rung is set by what cannot be undone rather than by how many
   * rows are touched. A paragraph above an armed button is not a rung.
   */
  acknowledged?: boolean;
  onAcknowledge?: (next: boolean) => void;
}) {
  return (
    <div className="danger-note px-4 py-3 text-[13.5px] leading-[1.55] text-muted">
      <strong className="font-semibold text-danger-text">This writes straight to the provider.</strong>{" "}
      {what} Syndra records nothing: no ledger row, no audit entry, no cascade. The next cache
      compile may overwrite it, and the drift sweep will report it as unexplained access. Use the
      equivalent action inside Syndra unless that is genuinely not an option.
      {onAcknowledge ? (
        // The sentence names what is *missing*, not a quantity. Everywhere
        // else the tick counts what changes; here the count is beside the
        // point and the absence is the entire risk.
        <label className="mt-3 flex min-h-[44px] cursor-pointer items-center gap-3 text-[13.5px] text-ink">
          <span className="flex h-11 w-11 shrink-0 items-center justify-center desktop:h-auto desktop:w-auto">
            <input
              type="checkbox"
              className="h-6 w-6 accent-[var(--accent)] desktop:h-4 desktop:w-4"
              checked={Boolean(acknowledged)}
              onChange={(event) => onAcknowledge(event.target.checked)}
            />
          </span>
          <span>I understand this writes to Zitadel now, with no plan and no record in Syndra.</span>
        </label>
      ) : null}
    </div>
  );
}
