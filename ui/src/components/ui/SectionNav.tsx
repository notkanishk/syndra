"use client";

import { useEffect, useState, type ReactNode } from "react";

import { Chip, Count, StatusDot, type StatusTone } from "@/components/ui/Badge";
import { GlideIndicator, glideTo, useGlideIndicator } from "@/lib/motion";

/**
 * Getting to a section on a page that has two or more (RULES, grouped lists).
 *
 * One interface, two pieces that share it: `ReadLine` is the line under the
 * filter bar — how current the list is on the left, one chip per section at the
 * right end — and `SectionPill` is the same sections floating at the top centre
 * once that line has scrolled away. Both take the same `sections` and the same
 * `onOpen`, and both read their positions from the DOM by section `id`, so the
 * caller draws its headings and nothing else.
 *
 * `onOpen` is how a folded section opens on the way (it is called first, then
 * the page glides there). It receives the section it was given, so a caller can
 * keep its own key beside the id.
 *
 * Drawn only for two or more sections: one has nowhere to take you.
 */
export type NavSection = {
  /** The id of the heading's element; the page scrolls to it. */
  id: string;
  /** The short name on the chip. */
  label: string;
  /** Absent for a section that is one thing, not a list (a connected system). */
  count?: number;
  /** A dot before the name; none when omitted. */
  tone?: StatusTone;
};

/** The chips' own id: where the pill reads "the read line has gone". */
const CHIPS_ID = "section-chips";

/** Opens a folded section on the way, then takes the reader to it. */
function go<T extends NavSection>(section: T, onOpen?: (section: T) => void) {
  onOpen?.(section);
  // After the open has rendered, or there is no place to land.
  setTimeout(() => {
    const target = document.getElementById(section.id);
    if (!target) return;
    glideTo(target);
  }, 0);
}

/**
 * The read line. `children` is its left: usually a `ReadFreshness`, whatever
 * says how current the list is. The chips are at the right end, dot, short
 * name, count, ↓.
 */
export function ReadLine<T extends NavSection>({
  sections,
  onOpen,
  label,
  children,
  className = "",
}: {
  sections: T[];
  onOpen?: (section: T) => void;
  /** Names the chips for a screen reader: "Sections of drift". */
  label: string;
  children?: ReactNode;
  className?: string;
}) {
  return (
    <div className={`flex flex-wrap items-center gap-x-3.5 gap-y-2 ${className}`}>
      {children}
      {sections.length > 1 && (
        <nav id={CHIPS_ID} aria-label={label} className="ml-auto flex flex-wrap gap-2">
          {sections.map((section) => (
            <a
              key={section.id}
              href={`#${section.id}`}
              onClick={(event) => {
                event.preventDefault();
                go(section, onOpen);
              }}
              className="group inline-flex min-h-[var(--tap)] items-center rounded-pill control press-scale desktop:min-h-0"
            >
              <Chip className="gap-2 motion-tint group-hover:bg-tint-3">
                {section.tone && <StatusDot tone={section.tone} />}
                {section.label}
                {section.count !== undefined && (
                  <span className="text-faint">
                    <Count n={section.count} />
                  </span>
                )}
                <span aria-hidden className="text-fine text-faint">
                  ↓
                </span>
              </Chip>
            </a>
          ))}
        </nav>
      )}
    </div>
  );
}

/**
 * The same sections as a pill at the top centre, once the read line has
 * scrolled away. The one you are in is filled; the others point up or down to
 * where they are. Put it at the top of the sections' own container, which is
 * what it sticks within; `gap` is that container's row gap, which the pill
 * gives back so it takes no room of its own. A CSS length, so a container whose
 * gap is a token passes the token (`var(--region-gap)`) and the two can never
 * disagree, at any width.
 */
export function SectionPill<T extends NavSection>({
  sections,
  onOpen,
  label,
  gap = "16px",
}: {
  sections: T[];
  onOpen?: (section: T) => void;
  label: string;
  gap?: string;
}) {
  const [spy, setSpy] = useState<{ active: string; away: boolean }>({ active: "", away: false });
  const ids = sections.map((section) => section.id).join(" ");
  useEffect(() => {
    if (!ids.includes(" ")) return;
    const read = () => {
      const rest = document.getElementById(CHIPS_ID);
      const container = document.getElementById("app-scroll");
      const top = container ? container.getBoundingClientRect().top : 0;
      let active = ids.split(" ")[0];
      for (const id of ids.split(" ")) {
        const el = document.getElementById(id);
        if (el && el.getBoundingClientRect().top - top < 120) active = id;
      }
      const away = rest ? rest.getBoundingClientRect().bottom - top < 0 : false;
      setSpy((cur) => (cur.active === active && cur.away === away ? cur : { active, away }));
    };
    read();
    document.addEventListener("scroll", read, true);
    return () => document.removeEventListener("scroll", read, true);
  }, [ids]);

  const { ref: pillRef, box } = useGlideIndicator(spy.active);
  if (sections.length < 2) return null;
  const at = Math.max(0, sections.findIndex((section) => section.id === spy.active));
  return (
    <div className="sticky top-0 z-(--layer-bar) h-0" style={{ marginBottom: `calc(-1 * ${gap})` }}>
      {/* The band behind the pill: the page's own ground, full width of the column, so text that
          scrolls up never shows through beside the pill. --section-band is the room a page with this band keeps
          above what a jump lands on (the scroller's scroll-padding-top in globals.css, which glideTo reads
          as the browser does), so a section it lands on starts just under the band. */}
      <div
        aria-hidden
        data-section-band
        className={`absolute -inset-x-4 top-0 h-(--section-band) border-b border-line bg-canvas motion-tint tablet:-inset-x-(--page-x) ${
          spy.away ? "opacity-100" : "pointer-events-none opacity-0"
        }`}
      />
      <nav
        ref={pillRef}
        aria-label={`${label} (while scrolling)`}
        aria-hidden={!spy.away}
        className={`absolute left-1/2 top-3 flex max-w-[calc(100%-24px)] -translate-x-1/2 gap-0.5 overflow-x-auto whitespace-nowrap rounded-pill border border-line-strong bg-surface-1 p-1 shadow-dialog motion-tint ${
          // Drops in 10px on settle once the read line has gone; leaves on tint,
          // where it stands (opacity only).
          spy.away ? "drop-in opacity-100" : "pointer-events-none opacity-0"
        }`}
      >
        {/* The current section's fill is the one shared indicator: it glides, the links only tint. */}
        <GlideIndicator box={box} kind="fill" className="bg-surface-2" />
        {sections.map((section, index) => {
          const here = index === at;
          return (
            <a
              key={section.id}
              href={`#${section.id}`}
              tabIndex={spy.away ? 0 : -1}
              aria-current={here ? "location" : undefined}
              onClick={(event) => {
                event.preventDefault();
                go(section, onOpen);
              }}
              className={`relative inline-flex min-h-9 items-center gap-2 rounded-pill px-3.5 text-small font-semibold control press-scale ${
                // Until the indicator is measured (first paint) the current link wears its own fill.
                here ? `${box ? "" : "bg-surface-2 "}text-ink` : "text-muted hover:bg-[var(--hover)] hover:text-ink"
              }`}
            >
              {section.tone && <StatusDot tone={section.tone} />}
              {section.label}
              {section.count !== undefined && (
                <span className="font-medium text-faint">
                  <Count n={section.count} />
                </span>
              )}
              {/* Every section keeps its arrow's seat, empty for the one you are in, so the pill holds its
                  width as you scroll (D-1 §5 #35): an arrow on every section but the current one used to
                  change the pill's width at every section boundary. */}
              <span aria-hidden data-arrow-seat className="inline-block w-3 text-center text-fine text-faint">
                {here ? "" : index < at ? "↑" : "↓"}
              </span>
            </a>
          );
        })}
      </nav>
    </div>
  );
}
