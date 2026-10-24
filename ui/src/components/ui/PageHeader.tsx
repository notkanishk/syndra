"use client";

import { useEffect, useLayoutEffect, useState } from "react";

import { ARRIVE_SETTLED_MS, returnedToThisPage } from "@/components/ui/ArriveList";
import { TINT_MS } from "@/lib/motion";

/**
 * A screen's first paint (S11), for a header that is not a `PageHeader`: the
 * greeting on Home, the member's home, a screen that stands alone. Spread
 * `className` and `data-settled` onto the first child of the page; what follows
 * it arrives (`page-head` in globals.css). Only when you navigate there:
 * coming Back, or once it has run, the page is simply there.
 */
export function usePageHead(): { className: string; "data-settled": string | undefined } {
  const [settled, setSettled] = useState(false);
  // After mount rather than in the initial state: the server cannot know about
  // a Back, and a different first render is a hydration mismatch.
  useLayoutEffect(() => {
    if (returnedToThisPage()) setSettled(true);
  }, []);
  useEffect(() => {
    if (settled) return;
    const timer = setTimeout(() => setSettled(true), TINT_MS + ARRIVE_SETTLED_MS);
    return () => clearTimeout(timer);
  }, [settled]);
  return { className: "page-head", "data-settled": settled ? "" : undefined };
}

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
    <div
      className={`${head.className} flex flex-wrap items-end gap-4 ${className}`}
      data-settled={head["data-settled"]}
    >
      {/* `flex-auto` (basis: auto), not `flex-1` (basis: 0%) — with a 0%
          basis this block reports zero size to the wrap decision, so `actions`
          claims the whole line and this text is squeezed into whatever sliver
          is left, wrapping a two-word name onto two lines with the rest of
          the row still empty. `flex-auto` lets its real content size count,
          so `actions` wraps below once the two genuinely don't fit together. */}
      <div className="min-w-0 flex-auto">
        {eyebrow && <div className="mb-1 text-[13.5px] text-faint">{eyebrow}</div>}
        <h1 className="type-page-title">{title}</h1>
        {meta && <div className="mt-1.5 text-[14.5px] text-muted">{meta}</div>}
        {lede && (
          <p className="mt-2 max-w-[72ch] text-[14.5px] leading-[1.6] text-muted">{lede}</p>
        )}
      </div>
      {actions && <div className="flex flex-wrap items-center gap-2.5">{actions}</div>}
    </div>
  );
}

/** Dot separator for a metadata row. */
export function MetaDot() {
  return <span aria-hidden className="inline-block h-[3px] w-[3px] rounded-pill bg-ink/30" />;
}

/** A metadata row: children separated by 3px dots. */
export function MetaRow({ children }: { children: React.ReactNode[] }) {
  const items = children.filter(Boolean);
  return (
    <span className="flex flex-wrap items-center gap-3 text-[14.5px] text-muted">
      {items.map((item, index) => (
        <span key={index} className="flex items-center gap-3">
          {index > 0 && <MetaDot />}
          {item}
        </span>
      ))}
    </span>
  );
}
