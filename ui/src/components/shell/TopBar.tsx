"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";

import { AccountSheet } from "@/components/shell/AccountSheet";
import { OPEN_GO_TO } from "@/components/shell/GoToHost";
import { SyndraMark } from "@/components/shell/SyndraMark";
import { ThemeToggle } from "@/components/shell/ThemeToggle";
import { ViewSwitch } from "@/components/shell/ViewSwitch";
import { notePage } from "@/lib/motion";
import { usePageCrumb } from "@/lib/page-crumb";
import { crumbsFor } from "@/lib/nav";
import type { SessionUser } from "@/lib/session";
import { useUiView } from "@/lib/ui-view";

/**
 * The 66px header: where you are on the left, what you're looking at on the
 * right. The view switch sits beside the account chip and away from the rail,
 * so it reads as "what I'm looking at" rather than a destination.
 */
export function TopBar({ session }: { session: SessionUser }) {
  const pathname = usePathname();
  // In render, not an effect: the page's parts render after this in the same
  // pass and ask whether they are its first paint (a Notice, A-08 §5).
  notePage(pathname);
  const { audience, isOperator } = useUiView();
  const { crumb } = usePageCrumb();

  const crumbs = crumbsFor(pathname, audience);
  const trail = crumb ? [...crumbs, { label: crumb }] : crumbs;

  return (
    <header className="flex h-[66px] flex-none items-center gap-2.5 border-b border-line px-4 tablet:gap-4 tablet:px-[26px]">
      {/* The mark has a seat on a phone (A-20 P6): the rail that holds it is
          hidden there, and the one-shot bloom lights whichever is on screen. */}
      <span className="flex-none tablet:hidden">
        <SyndraMark size={16} welcome />
      </span>
      <nav aria-label="Where you are" className="min-w-0 truncate text-[14.5px]">
        {trail.length === 0 ? (
          <span className="font-semibold text-ink">Syndra</span>
        ) : (
          trail.map((entry, index) => {
            const last = index === trail.length - 1;
            return (
              <span key={`${entry.label}-${index}`}>
                {index > 0 && <span className="text-faint"> / </span>}
                {last ? (
                  <span className="font-semibold text-ink">{entry.label}</span>
                ) : "href" in entry && entry.href ? (
                  // A breadcrumb trail is links and separators, not a
                  // sentence, so 2.5.8's inline exemption does not reach it.
                  // This measured 18px on every detail route.
                  <Link
                    href={entry.href}
                    className="inline-flex min-h-[var(--tap)] items-center text-muted link-hover desktop:min-h-6"
                  >
                    {entry.label}
                  </Link>
                ) : (
                  <span className="text-muted">{entry.label}</span>
                )}
              </span>
            );
          })
        )}
      </nav>

      <span className="flex-1" />

      {/* Go to, by touch (A-20 P6): a button, not a tab — Basic keeps its four.
          Operators only; a member's three pages need no jump. It asks the one
          Go to that GoToHost mounts, so nothing is duplicated. */}
      {isOperator && (
        <button
          type="button"
          onClick={() => window.dispatchEvent(new Event(OPEN_GO_TO))}
          aria-label="Go to a person, a project, an app, or a page"
          className="inline-flex min-h-[var(--tap)] flex-none items-center rounded-pill px-3 text-[14.5px] font-semibold text-ink control press-scale tablet:hidden"
        >
          Go to
        </button>
      )}

      {/* Advanced reaches the switch at the top of its nav sheet, so on a
          phone it is not also here — the same control in two places at once is
          two answers to "where does this live". Basic has no sheet, so the
          header is the only home it has. */}
      <span className={audience === "advanced" ? "hidden tablet:flex" : "flex"}>
        <ViewSwitch />
      </span>

      {/* Appearance is reachable from the account sheet at every width. The
          header keeps its one-tap toggle only where there is room for one. */}
      <span className="hidden tablet:flex">
        <ThemeToggle />
      </span>

      <AccountSheet session={session} />
    </header>
  );
}
