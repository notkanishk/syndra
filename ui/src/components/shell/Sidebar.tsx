"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";

import { useTargets } from "@/lib/queries/useTargets";
import { useIndicators, type Indicators } from "@/lib/queries/useIndicators";
import { leafMatches, navFor, targetNav, type NavLeaf } from "@/lib/nav";
// Shared with the tab bar, because the rail and the tab bar are two renderings
// of one tree and a count that is danger in one cannot be accent in the other.
import { BADGE_TONE } from "@/components/shell/navTones";
import { useUiView } from "@/lib/ui-view";
import { useFlashOnChange } from "@/lib/useFlashOnChange";
import { SyndraMark } from "./SyndraMark";

/**
 * The navigation rail. 252px, its own background, one border on the right.
 *
 * Two properties this component exists to guarantee:
 *
 *   1. Structure never moves. Every row is rendered from a static tree; only
 *      badge values change. A count going 0 → 12 must never shift the item
 *      under the operator's cursor.
 *   2. A section with nothing in it keeps its seat and shows a hollow zero.
 *      Disappearing rows teach people not to trust the rail.
 */
export default function Sidebar() {
  const pathname = usePathname();
  const { audience, isOperator } = useUiView();
  const { data: indicators, isPlaceholderData } = useIndicators(isOperator);

  // The roster is deployment configuration, so a target row appears because the
  // deployment registered one — never because this operator can see something.
  // The rail renders without it and gains the rows when it arrives: a structure
  // that waited for a fetch would be a rail that moves in response to a poll.
  const { data: targets } = useTargets();
  const entries =
    audience === "advanced"
      ? targetNav((targets ?? []).map((t) => t.target))
      : navFor(audience);
  const viewLabel =
    audience === "member" ? "Member" : audience === "advanced" ? "Advanced" : "Basic";

  return (
    // Hidden below tablet rather than unmounted. It keeps polling there, which
    // is deliberate — the tab bar reads the same query, so one poll serves both
    // — and it means a rotation does not remount the rail and reset the
    // `settled` guard that stops every badge flashing on arrival.
    <div className="hidden w-[252px] flex-none flex-col gap-5 overflow-y-auto border-r border-line bg-rail px-3 pb-[22px] pt-5 tablet:flex">
      <div className="flex items-center gap-2.5 px-2">
        <SyndraMark />
        <span className="font-display text-[18px] font-semibold tracking-[-0.01em]">Syndra</span>
        <span className="flex-1" />
        <span
          title={`${viewLabel} view`}
          className="rounded-pill border border-line-strong px-2 py-0.5 text-[12.5px] text-faint"
        >
          {viewLabel}
        </span>
      </div>

      <nav className="flex flex-col gap-[3px]" aria-label="Main navigation">
        {entries.map((entry) =>
          entry.kind === "leaf" ? (
            <NavRow
              key={entry.href}
              item={entry}
              pathname={pathname}
              indicators={indicators}
              settled={!isPlaceholderData}
              nested={false}
            />
          ) : (
            <div key={entry.label} className="contents">
              {/* A parent with children is a section label, not a link. */}
              <div className="px-3 pb-[5px] pt-4">
                <span className="type-nav-group">{entry.label}</span>
              </div>
              {entry.children.map((child) => (
                <NavRow
                  key={child.href}
                  item={child}
                  pathname={pathname}
                  indicators={indicators}
                  settled={!isPlaceholderData}
                  nested
                />
              ))}
            </div>
          ),
        )}
      </nav>
    </div>
  );
}


function NavRow({
  item,
  pathname,
  indicators,
  settled,
  nested,
}: {
  item: NavLeaf;
  pathname: string;
  indicators?: Indicators;
  /** False while the rail is showing the query's placeholder zeros. */
  settled: boolean;
  nested: boolean;
}) {
  const active = leafMatches(item, pathname);
  const count = item.indicator ? Number(indicators?.[item.indicator] ?? 0) : undefined;
  // The rail is polled, so this is where a value most often changes while the
  // operator is reading something else. The ROW washes; the count itself only
  // lifts into place. Nothing counts up or ticks.
  //
  // `settled` is what stops every nonzero badge flashing on page load: until
  // the first payload lands these counts are the query's placeholder zeros,
  // and a real 12 arriving over a fabricated 0 is an arrival, not a change.
  const changed = useFlashOnChange(count, settled);

  return (
    <Link
      href={item.href}
      aria-current={active ? "page" : undefined}
      // `min-h-11` up to desktop: the rail appears at the tablet breakpoint and
      // a tablet is still a thumb, which is the whole argument
      // touch-targets.test.tsx makes. Its own links were 38-43px there.
      className={`flex min-h-11 items-center gap-[9px] rounded-nav text-[14.5px] motion-tint desktop:min-h-0 ${
        changed ? "flash " : ""
      }${
        nested ? "py-2 pl-[21px] pr-3" : "px-3 py-[9px]"
      } ${
        active
          ? "bg-accent-soft font-semibold text-accent-text"
          : `${nested ? "text-muted" : "text-ink/[.74]"} hover:bg-[var(--hover)]`
      }`}
    >
      {!nested && (
        <span
          aria-hidden
          className={`h-1.5 w-1.5 flex-none rounded-pill ${
            active ? "bg-accent" : "bg-ink/20"
          }`}
        />
      )}
      <span className="flex-1 truncate">{item.label}</span>
      {count !== undefined &&
        (count > 0 ? (
          <span
            className={`rounded-pill px-2 py-0.5 text-[12.5px] font-semibold ${
              changed ? "flash-value " : ""
            }${BADGE_TONE[item.tone ?? "accent"]}`}
          >
            {count}
          </span>
        ) : (
          // Hollow zero: the row keeps its seat and says so, rather than
          // vanishing and moving everything below it.
          <span className="rounded-pill border border-line-strong px-2 py-0.5 text-[12.5px] text-label">
            0
          </span>
        ))}
    </Link>
  );
}
