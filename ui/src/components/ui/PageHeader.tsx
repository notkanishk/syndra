"use client";

import { useEffect, useLayoutEffect, useState } from "react";

import { ShellNotices } from "@/components/shell/ShellNotices";
import { ARRIVE_SETTLED_MS, quietVisit, returnedToThisPage } from "@/lib/motion";
import { TINT_MS } from "@/lib/motion";

/**
 * A screen's first paint (S11), for a header that is not a `PageHeader`: the
 * greeting on Home, the member's home, a screen that stands alone. Spread
 * `className` and `data-settled` onto the first child of the page; what follows
 * it arrives (`page-head` in globals.css). Only the first time you navigate
 * there in a session (A-24 Q14): a later visit, or one made by key, is quiet,
 * the header tinting in over content that is simply there; coming Back, or once
 * it has run, the page is simply there.
 */
export function usePageHead(): {
  className: string;
  "data-settled": string | undefined;
  "data-quiet": string | undefined;
} {
  const [settled, setSettled] = useState(false);
  const [quiet, setQuiet] = useState(false);
  // After mount rather than in the initial state: the server cannot know about
  // a Back, and a different first render is a hydration mismatch.
  useLayoutEffect(() => {
    if (returnedToThisPage()) setSettled(true);
    else if (quietVisit()) {
      setSettled(true);
      setQuiet(true);
    }
  }, []);
  useEffect(() => {
    if (settled) return;
    const timer = setTimeout(() => setSettled(true), TINT_MS + ARRIVE_SETTLED_MS);
    return () => clearTimeout(timer);
  }, [settled]);
  return { className: "page-head", "data-settled": settled ? "" : undefined, "data-quiet": quiet ? "" : undefined };
}

/**
 * Header → content, in px (RULES 9b, A-17 G5). PageHeader owns it: a page
 * stacks its content at `CONTENT_GAP` (`flex flex-col gap-4`) and never sets
 * its own header gap. Owner ruled 28 (A-17 Q5, 5 Nov): the calmer page.
 */
export const HEADER_TO_CONTENT = 28;
/** The page's own stack, card to card and card to notice: `gap-4`. */
export const CONTENT_GAP = 16;
/** For a page head that is not a PageHeader (Home, the member's home): the same space under it. */
export const BELOW_HEADER = { marginBottom: HEADER_TO_CONTENT - CONTENT_GAP } as const;

/**
 * The page's own title block. The breadcrumb says where you are; this says
 * what you are looking at, with its actions on the right.
 *
 * `lede` is the page's purpose, in one or two sentences: what it shows, when
 * you would come here, and — for a queue — what happens if you do nothing
 * (plain-language-copy §4). It used to be smuggled through `meta`, so a page
 * carried a count there, or a sentence, or nothing, and eleven pages had no
 * sentence saying what they were for. The two are separate props so that a
 * count never displaces the sentence, and so the guard can see which pages
 * still lack one.
 *
 * It is also the screen's first paint (S11): it tints in, then what follows
 * it arrives (`page-head` in globals.css), and only when you navigate to the
 * page: coming Back to it, it is simply there.
 */
export function PageHeader({
  eyebrow,
  title,
  lede,
  meta,
  actions,
  className = "",
}: {
  /** Quiet line above the title — a parent project, a category. */
  eyebrow?: React.ReactNode;
  title: React.ReactNode;
  /** What this page is for. One or two sentences; every page has one. */
  lede?: React.ReactNode;
  /** Metadata row beneath: email · title · team · id, dot-separated. Never a sentence. */
  meta?: React.ReactNode;
  actions?: React.ReactNode;
  className?: string;
}) {
  const head = usePageHead();

  return (
    // `className` sizes the root, the box a parent row lays out (the person
    // page's `w-full flex-1` beside its avatar). On the inner row it left the
    // root as wide as its text, so the actions sat wherever the text ended:
    // the right seat (D-1) is the root's full width, the text block growing
    // into it (`flex-auto`) and the actions at its end.
    <div {...head} className={`${head.className} ${className}`} style={BELOW_HEADER}>
      <div className="flex flex-wrap items-end gap-4">
      {/* `flex-auto` (basis: auto), not `flex-1` (basis: 0%) — with a 0%
          basis this block reports zero size to the wrap decision, so `actions`
          claims the whole line and this text is squeezed into whatever sliver
          is left, wrapping a two-word name onto two lines with the rest of
          the row still empty. `flex-auto` lets its real content size count,
          so `actions` wraps below once the two genuinely don't fit together. */}
      <div className="min-w-0 flex-auto">
        {eyebrow && <div className="mb-1 text-small text-faint">{eyebrow}</div>}
        <h1 className="type-page-title">{title}</h1>
        {meta && (
          <div data-prose className="mt-1.5 text-body text-muted">
            {meta}
          </div>
        )}
        {lede && (
          <p data-prose className="measure mt-2 text-body text-muted">
            {lede}
          </p>
        )}
      </div>
      {actions && <div className="flex flex-wrap items-center gap-2.5">{actions}</div>}
      </div>
      {/* The shell's Notices, 16 under the lede (A-20 P3, G7). */}
      <ShellNotices />
    </div>
  );
}

/** Dot separator for a metadata row. */
export function MetaDot() {
  return <span aria-hidden className="inline-block h-[3px] w-[3px] rounded-pill bg-ink/30" />;
}

/**
 * A metadata row: children separated by 3px dots.
 *
 * Every item carries its dot, the row sits one dot and gap (15px) to the left,
 * and the wrapper clips: whichever item starts a line, the first or one that
 * wrapped on a phone, has its dot in the clipped strip. A dot only ever sits
 * between two items. The 3px padding keeps focus rings inside the clip.
 */
export function MetaRow({ children }: { children: React.ReactNode[] }) {
  const items = children.filter(Boolean);
  return (
    <span className="-m-[3px] block overflow-hidden p-[3px]">
      <span className="-ml-[15px] flex flex-wrap items-center gap-3 text-body text-muted">
        {items.map((item, index) => (
          <span key={index} className="flex items-center gap-3">
            <MetaDot />
            {item}
          </span>
        ))}
      </span>
    </span>
  );
}
