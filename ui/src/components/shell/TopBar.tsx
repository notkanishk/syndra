"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";

import { AccountSheet } from "@/components/shell/AccountSheet";
import { ThemeToggle } from "@/components/shell/ThemeToggle";
import { ViewSwitch } from "@/components/shell/ViewSwitch";
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
  const { audience } = useUiView();
  const { crumb } = usePageCrumb();

  const crumbs = crumbsFor(pathname, audience);
  const trail = crumb ? [...crumbs, { label: crumb }] : crumbs;

  return (
    <header className="flex h-[66px] flex-none items-center gap-2.5 border-b border-line px-4 tablet:gap-4 tablet:px-[26px]">
      <nav aria-label="Breadcrumb" className="min-w-0 truncate text-[14.5px]">
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
                    className="inline-flex min-h-11 items-center text-muted hover:text-ink desktop:min-h-6"
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
