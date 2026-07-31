"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";

import { useIndicators, type Indicators } from "@/lib/queries/useIndicators";
import { leafMatches, navFor, type BadgeTone, type NavLeaf } from "@/lib/nav";
import { useUiView } from "@/lib/ui-view";

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
  const { data: indicators } = useIndicators(isOperator);

  const entries = navFor(audience);
  const viewLabel =
    audience === "member" ? "Member" : audience === "advanced" ? "Advanced" : "Basic";

  return (
    <div className="flex w-[252px] flex-none flex-col gap-5 overflow-y-auto border-r border-line bg-rail px-3 pb-[22px] pt-5">
      <div className="flex items-center gap-2.5 px-2">
        <span className="flex h-7 w-7 items-center justify-center rounded-[9px] bg-accent font-display text-[15px] font-bold text-accent-ink">
          m
        </span>
        <span className="font-display text-[18px] font-semibold tracking-[-0.01em]">MkAuth</span>
        <span className="flex-1" />
        <span className="rounded-pill border border-line-strong px-2 py-0.5 text-[11px] text-faint">
          {viewLabel}
        </span>
      </div>

      <nav className="flex flex-col gap-[3px]" aria-label="Primary">
        {entries.map((entry) =>
          entry.kind === "leaf" ? (
            <NavRow
              key={entry.href}
              item={entry}
              pathname={pathname}
              indicators={indicators}
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

const BADGE_TONE: Record<BadgeTone, string> = {
  accent: "bg-accent text-accent-ink",
  warn: "bg-warn text-warn-ink",
  danger: "bg-danger text-danger-ink",
};

function NavRow({
  item,
  pathname,
  indicators,
  nested,
}: {
  item: NavLeaf;
  pathname: string;
  indicators?: Indicators;
  nested: boolean;
}) {
  const active = leafMatches(item, pathname);
  const count = item.indicator ? Number(indicators?.[item.indicator] ?? 0) : undefined;

  return (
    <Link
      href={item.href}
      aria-current={active ? "page" : undefined}
      className={`flex items-center gap-[9px] rounded-nav text-[14.5px] transition-colors duration-150 ${
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
            className={`rounded-pill px-2 py-0.5 text-[11.5px] font-semibold ${
              BADGE_TONE[item.tone ?? "accent"]
            }`}
          >
            {count}
          </span>
        ) : (
          // Hollow zero: the row keeps its seat and says so, rather than
          // vanishing and moving everything below it.
          <span className="rounded-pill border border-line-strong px-2 py-0.5 text-[11.5px] text-label">
            0
          </span>
        ))}
    </Link>
  );
}
