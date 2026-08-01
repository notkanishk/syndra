"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";

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
    <header className="flex h-[66px] flex-none items-center gap-4 border-b border-line px-[26px]">
      <nav aria-label="Breadcrumb" className="min-w-0 truncate text-[14.5px]">
        {trail.length === 0 ? (
          <span className="font-semibold text-ink">MkAuth</span>
        ) : (
          trail.map((entry, index) => {
            const last = index === trail.length - 1;
            return (
              <span key={`${entry.label}-${index}`}>
                {index > 0 && <span className="text-faint"> / </span>}
                {last ? (
                  <span className="font-semibold text-ink">{entry.label}</span>
                ) : "href" in entry && entry.href ? (
                  <Link href={entry.href} className="text-muted hover:text-ink">
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

      <ViewSwitch />
      <ThemeToggle />

      <span className="flex items-center gap-[9px]">
        <span
          aria-hidden
          className="avatar-fill flex h-[30px] w-[30px] items-center justify-center rounded-pill text-[11px] font-semibold text-ink/70"
        >
          {session.avatar}
        </span>
        {/* Email is the fallback, never the id: if every naming source came up
            empty, an address still identifies a human. */}
        <span className="text-[13.5px] text-muted">{session.name || session.email}</span>
      </span>

      <form action="/auth/logout" method="post">
        <button
          type="submit"
          className="rounded-pill border border-line-strong px-3 py-1.5 text-[13px] font-semibold text-muted transition-colors hover:text-ink"
        >
          Sign out
        </button>
      </form>
    </header>
  );
}
