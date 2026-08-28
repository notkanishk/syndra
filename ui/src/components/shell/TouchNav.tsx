"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useCallback, useEffect, useRef, useState } from "react";

import { BADGE_TONE, DOT_TONE, loudestTone } from "@/components/shell/navTones";
import { VIEW_EXPLANATION, VIEW_HINT } from "@/components/shell/ViewSwitch";
import { useDialogFocusTrap } from "@/components/ui/Modal";
import { leafMatches, navFor, targetNav, type IndicatorKey, type NavLeaf } from "@/lib/nav";
import { useIndicators } from "@/lib/queries/useIndicators";
import { useTargets } from "@/lib/queries/useTargets";
import {
  destinationMatches,
  destinationsWantingAttention,
  navShape,
  tonesInPlay,
  touchDestinations,
  type TouchDestination,
} from "@/lib/touch-nav";
import { useUiView } from "@/lib/ui-view";

/**
 * Navigation for a thumb.
 *
 * Rendered beside the rail and hidden above the tablet breakpoint, rather than
 * swapped in by JavaScript. Both exist in the tree at every width, which costs
 * one hidden subtree and buys three things: no flash of the wrong navigation
 * before hydration, no hydration mismatch to design around, and — the one that
 * actually decides it — no remount of the rail when a device rotates. The
 * rail's badges are guarded by `useFlashOnChange`'s `settled` flag, and that
 * guard resets on mount, so a rail that unmounted and came back would flash
 * every non-zero badge as though the numbers had just changed.
 *
 * The shape is decided by the destination count, not by the audience, so a
 * deployment that registers a fourth add-on cannot quietly produce a five-tab
 * bar. See `lib/touch-nav.ts`.
 */
export function TouchNav() {
  const pathname = usePathname();
  const { audience, isOperator } = useUiView();
  const { data: indicators, isPlaceholderData } = useIndicators(isOperator);
  const { data: targets } = useTargets();

  const entries =
    audience === "advanced"
      ? targetNav((targets ?? []).map((t) => t.target))
      : navFor(audience);
  const destinations = touchDestinations(entries);
  const counts = isPlaceholderData ? undefined : indicators;

  return (
    <nav
      aria-label="Main navigation"
      // `tablet:hidden` and not a JS branch: the rail takes over at the same
      // width, and one of the two is always the only one visible.
      className="flex-none border-t border-line bg-rail px-1.5 pb-[env(safe-area-inset-bottom)] pt-2 tablet:hidden"
    >
      {navShape(destinations) === "tabs" ? (
        <TabBar destinations={destinations} pathname={pathname} counts={counts} />
      ) : (
        <SheetTrigger destinations={destinations} pathname={pathname} counts={counts} />
      )}
    </nav>
  );
}

function TabBar({
  destinations,
  pathname,
  counts,
}: {
  destinations: TouchDestination[];
  pathname: string;
  counts: Partial<Record<IndicatorKey, number>> | undefined;
}) {
  return (
    // Inset rather than edge-to-edge: at four tabs the corners of a 390px
    // screen are the furthest points from a thumb's arc, and the two tabs that
    // land there are the two nobody taps.
    <div className="mx-auto flex w-full max-w-[420px] items-stretch gap-1 px-1.5 pb-2">
      {destinations.map((destination) => (
        <Tab
          key={destination.href}
          destination={destination}
          active={destinationMatches(destination, pathname)}
          counts={counts}
        />
      ))}
    </div>
  );
}

function Tab({
  destination,
  active,
  counts,
}: {
  destination: TouchDestination;
  active: boolean;
  counts: Partial<Record<IndicatorKey, number>> | undefined;
}) {
  const count = destination.indicator ? Number(counts?.[destination.indicator] ?? 0) : undefined;
  // A collapsed group shows a dot in its loudest tone. Never a rolled-up
  // number: the destinations it hides are counted in different units.
  const groupTone = destination.children ? loudestTone(tonesInPlay(destination, counts)) : null;

  return (
    <Link
      href={destination.href}
      aria-current={active ? "page" : undefined}
      className={`relative flex min-h-[52px] flex-1 flex-col items-center justify-center gap-[5px] rounded-[13px] px-1 motion-press ${
        active ? "bg-accent-soft" : ""
      }`}
    >
      <span
        aria-hidden
        className={`h-[7px] w-[7px] rounded-pill ${active ? "bg-accent-text" : "bg-ink/30"}`}
      />
      <span
        className={`max-w-full truncate text-[12.5px] ${
          active ? "font-semibold text-accent-text" : "text-muted"
        }`}
      >
        {destination.label}
      </span>

      {count !== undefined && count > 0 && (
        <span
          className={`absolute right-2 top-1 min-w-[20px] rounded-pill px-1.5 text-center text-[12.5px] font-bold leading-[20px] ${
            BADGE_TONE[destination.tone ?? "accent"]
          }`}
        >
          {count}
        </span>
      )}
      {groupTone && (
        <span
          aria-hidden
          className={`absolute right-3 top-2 h-[7px] w-[7px] rounded-pill ${DOT_TONE[groupTone]}`}
        />
      )}
      {/* Colour never carries meaning alone. The dot above is decoration; this
          is the sentence a screen reader gets. */}
      {groupTone && <span className="sr-only">Needs attention</span>}
    </Link>
  );
}

/**
 * Advanced's entry to the rail, as a single bar.
 *
 * It reports one dot in the loudest tone present and a count of DESTINATIONS
 * wanting attention. Not items: three unexplained findings plus eleven
 * expiring grants plus three holds due is seventeen of nothing — three
 * different kinds of work, no single action that reduces the number, and an
 * operator who cannot tell from it where to go.
 */
function SheetTrigger({
  destinations,
  pathname,
  counts,
}: {
  destinations: TouchDestination[];
  pathname: string;
  counts: Partial<Record<IndicatorKey, number>> | undefined;
}) {
  const [open, setOpen] = useState(false);
  const here = destinations.find((destination) => destinationMatches(destination, pathname));
  const wanting = destinationsWantingAttention(destinations, counts);
  const tone = loudestTone(
    destinations.flatMap((destination) =>
      destination.children ? tonesInPlay(destination, counts) : [destination.tone],
    ),
  );

  return (
    <div className="px-2 pb-2">
      <button
        type="button"
        onClick={() => setOpen(true)}
        aria-haspopup="dialog"
        aria-expanded={open}
        className="flex min-h-[44px] w-full items-center gap-2.5 rounded-pill bg-tint-2 px-4 text-[13.5px] font-semibold text-ink motion-press"
      >
        <span aria-hidden className="text-faint">
          •••
        </span>
        <span className="flex-1 truncate text-left">{here?.label ?? "Go to"}</span>
        {wanting > 0 && tone && (
          <>
            <span aria-hidden className={`h-[7px] w-[7px] rounded-pill ${DOT_TONE[tone]}`} />
            {/* One string rather than interpolated fragments: the dot is
                decoration and this sentence is the whole of what a screen
                reader — or a test — is given. */}
            <span className="text-[12.5px] font-semibold text-muted">
              {wanting === 1 ? "1 place needs attention" : `${wanting} places need attention`}
            </span>
          </>
        )}
      </button>

      {open && (
        <NavSheet
          destinations={destinations}
          pathname={pathname}
          counts={counts}
          onClose={() => setOpen(false)}
        />
      )}
    </div>
  );
}

function NavSheet({
  destinations,
  pathname,
  counts,
  onClose,
}: {
  destinations: TouchDestination[];
  pathname: string;
  counts: Partial<Record<IndicatorKey, number>> | undefined;
  onClose: () => void;
}) {
  const panelRef = useRef<HTMLDivElement>(null);
  const { view, isOperator, setView } = useUiView();

  // Held in a ref for the same reason the focus trap holds its own: `onClose`
  // is a fresh closure every render, and this component re-renders on the
  // indicator poll every 30 seconds. With it in the dependency list the effect
  // below tore down and re-ran on that timer, pushing a new history entry each
  // time — a sheet left open for five minutes buried the screen behind it
  // under ten dead entries.
  const closeRef = useRef(onClose);
  useEffect(() => {
    closeRef.current = onClose;
  });

  // A sheet is a level of history: the system back gesture closes it before it
  // leaves the screen. Without this, back from an open sheet abandons the
  // screen behind it and the sheet's own dismissal is never reachable.
  useEffect(() => {
    window.history.pushState({ syndraSheet: true }, "");
    const onPop = () => closeRef.current();
    window.addEventListener("popstate", onPop);
    return () => window.removeEventListener("popstate", onPop);
  }, []);

  // Dismissing goes back rather than closing directly, so the entry pushed
  // above is spent rather than left behind. Closing the sheet by grabber and
  // then pressing back used to do nothing at all: the first press only ate the
  // entry the sheet never cleaned up.
  //
  // Picking a destination is the exception and keeps calling `onClose`: the
  // navigation pushes its own entry on top, so back from the new screen lands
  // on this one — and racing `history.back()` against a Next.js push would
  // undo the navigation the tap asked for.
  const dismiss = useCallback(() => window.history.back(), []);

  // Never busy: a nav sheet runs nothing, so nothing can make it undismissable.
  useDialogFocusTrap(panelRef, true, false, dismiss);

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-label="Go to a page"
      className="settle-scrim fixed inset-0 z-50 flex items-end bg-black/70"
      onClick={(event) => {
        if (event.target === event.currentTarget) dismiss();
      }}
    >
      <div
        ref={panelRef}
        className="settle-in max-h-[86dvh] w-full overflow-y-auto rounded-t-[24px] border-t border-line-strong bg-rail px-3.5 pb-[max(24px,env(safe-area-inset-bottom))] pt-3"
      >
        <button
          type="button"
          onClick={dismiss}
          // The same words Modal's grabber answers to. Two sheets whose
          // handles have different names are two sheets to anything querying
          // by accessible name, a screen reader included.
          aria-label="Close this sheet"
          // Out of the tab order, for the reason `focusableIn` documents: as
          // the panel's first focusable element it took the focus the trap
          // gives on open, so the nav sheet opened with the cursor on "close
          // it" rather than on where the operator came to go. Esc, the scrim
          // and the back gesture all still dismiss.
          tabIndex={-1}
          // 44px of target around a 4px bar. The negative margins hold the bar
          // where it was — measured at 390, not derived: the panel's own
          // border pushes everything a pixel that arithmetic on the padding
          // alone does not see, and a first attempt computed that way put the
          // bar 6px low.
          className="-mb-[1px] -mt-[11px] mx-auto flex h-11 w-full max-w-[120px] items-center justify-center"
        >
          <span aria-hidden className="h-1 w-[38px] rounded-pill bg-ink/20" />
        </button>

        {isOperator && (
          // Two states, both labelled, so the current one is unmistakable.
          // Switching reveals in place and never navigates, so the sheet stays
          // open around it — the operator is choosing what this screen shows,
          // not where to go.
          <div role="group" aria-label="Basic or Advanced view" className="mb-3 flex rounded-pill bg-tint-1 p-1">
            {(["basic", "advanced"] as const).map((option) => (
              <button
                key={option}
                type="button"
                onClick={() => setView(option)}
                aria-pressed={view === option}
                title={VIEW_HINT[option]}
                className={`min-h-[44px] flex-1 rounded-pill text-[13.5px] capitalize motion-press ${
                  // Dense, not bright: the label is 13.5px and the bright
                  // accent fails AA under 18.5px.
                  view === option ? "bg-accent-dense font-semibold text-accent-ink" : "text-muted"
                }`}
              >
                {option}
              </button>
            ))}
          </div>
        )}
        {isOperator && (
          <p className="mb-3 px-3.5 text-[12.5px] leading-[1.5] text-faint">{VIEW_EXPLANATION}</p>
        )}

        <div className="flex flex-col gap-0.5">
          {destinations.map((destination) => (
            <SheetSection
              key={destination.href}
              destination={destination}
              pathname={pathname}
              counts={counts}
              onPick={onClose}
            />
          ))}
        </div>
      </div>
    </div>
  );
}

function SheetSection({
  destination,
  pathname,
  counts,
  onPick,
}: {
  destination: TouchDestination;
  pathname: string;
  counts: Partial<Record<IndicatorKey, number>> | undefined;
  onPick: () => void;
}) {
  const here = destinationMatches(destination, pathname);

  if (!destination.children) {
    return (
      <SheetRow
        href={destination.href}
        label={destination.label}
        active={here}
        indicator={destination.indicator}
        tone={destination.tone}
        counts={counts}
        onPick={onPick}
      />
    );
  }

  // Only the current section is expanded. A sheet listing every child of every
  // group is the rail again, at half the width and under a thumb.
  const tone = loudestTone(tonesInPlay(destination, counts));
  if (!here) {
    return (
      <SheetRow
        href={destination.href}
        label={destination.label}
        active={false}
        counts={counts}
        onPick={onPick}
        dot={tone}
      />
    );
  }

  return (
    <div className="flex flex-col gap-0.5">
      <div className="px-3.5 pb-1 pt-3">
        <span className="type-nav-group">{destination.label}</span>
      </div>
      {destination.children.map((child) => (
        <SheetRow
          key={child.href}
          href={child.href}
          label={child.label}
          active={leafMatches(child, pathname)}
          indicator={child.indicator}
          tone={child.tone}
          counts={counts}
          onPick={onPick}
          nested
        />
      ))}
    </div>
  );
}

function SheetRow({
  href,
  label,
  active,
  indicator,
  tone,
  counts,
  onPick,
  nested = false,
  dot = null,
}: {
  href: string;
  label: string;
  active: boolean;
  indicator?: NavLeaf["indicator"];
  tone?: NavLeaf["tone"];
  counts: Partial<Record<IndicatorKey, number>> | undefined;
  onPick: () => void;
  nested?: boolean;
  dot?: ReturnType<typeof loudestTone>;
}) {
  const count = indicator ? Number(counts?.[indicator] ?? 0) : undefined;

  return (
    <Link
      href={href}
      onClick={onPick}
      aria-current={active ? "page" : undefined}
      className={`flex min-h-[44px] items-center gap-2.5 rounded-[12px] text-[15px] motion-press ${
        nested ? "pl-7 pr-3.5" : "px-3.5"
      } ${active ? "bg-accent-soft font-semibold text-accent-text" : "text-ink/[.82]"}`}
    >
      <span className="flex-1 truncate">{label}</span>

      {dot && (
        <>
          <span aria-hidden className={`h-[7px] w-[7px] rounded-pill ${DOT_TONE[dot]}`} />
          <span className="sr-only">Needs attention</span>
        </>
      )}

      {count !== undefined &&
        (count > 0 ? (
          <span
            className={`min-w-[20px] rounded-pill px-1.5 text-center text-[12.5px] font-bold leading-[20px] ${
              BADGE_TONE[tone ?? "accent"]
            }`}
          >
            {count}
          </span>
        ) : (
          // The hollow zero, same as the rail: the row keeps its seat and says
          // there is nothing, rather than vanishing and moving what is below it
          // under a thumb already on its way down.
          <span className="min-w-[20px] rounded-pill border border-line-strong px-1.5 text-center text-[12.5px] leading-[18px] text-label">
            0
          </span>
        ))}
    </Link>
  );
}
