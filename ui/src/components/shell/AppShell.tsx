"use client";

import Sidebar from "@/components/shell/Sidebar";
import { TopBar } from "@/components/shell/TopBar";
import { TouchNav } from "@/components/shell/TouchNav";
import { DegradedBanner } from "@/components/states/DegradedBanner";
import { OfflineBanner } from "@/components/states/OfflineBanner";
import { PageCrumbProvider } from "@/lib/page-crumb";
import type { SessionUser } from "@/lib/session";
import type { UiView } from "@/lib/nav";
import { UiViewProvider } from "@/lib/ui-view";

/**
 * The operator shell.
 *
 *   ┌─ 252px rail ─┬───────────── content ─────────────┐
 *   │ own bg       │ top bar 66px, border-bottom       │
 *   │ border-right │  breadcrumb …  [switch] [account] │
 *   │              ├───────────────────────────────────┤
 *   │              │ padding 30px 26px 36px            │
 *   └──────────────┴───────────────────────────────────┘
 *
 * The scroll container carries an id because a scoped jump from Basic into
 * Advanced scrolls THIS element rather than calling scrollIntoView, which
 * would also move the page sideways in a narrow viewport. It stays the
 * scroller at every width for the same reason: moving scrolling to the body on
 * a phone would make that jump a silent no-op rather than an error.
 *
 * Below the tablet breakpoint the same tree stacks instead:
 *
 *   ┌───────────────────────────────────┐
 *   │ top bar                           │
 *   ├───────────────────────────────────┤
 *   │ content, 16px gutters             │
 *   ├───────────────────────────────────┤
 *   │ tab bar / go-to bar, safe area    │
 *   └───────────────────────────────────┘
 *
 * `h-dvh` and not `h-screen`: `100vh` on a phone is the viewport with the URL
 * bar retracted, so the bottom of a `h-screen overflow-hidden` shell sits
 * permanently under it — and the bottom of this shell is where every primary
 * action lives.
 */
export function AppShell({
  session,
  initialView,
  children,
}: {
  session: SessionUser;
  /** The operator's stored Basic/Advanced choice, read from its cookie by the server. */
  initialView?: UiView;
  children: React.ReactNode;
}) {
  return (
    <UiViewProvider isOperator={session.role === "admin"} selfId={session.id} initialView={initialView}>
      <PageCrumbProvider>
        {/* `app-root` is what a dialog makes inert (Modal): overlays portal to <body>, outside it. */}
        <div id="app-root" className="flex h-dvh flex-col overflow-hidden bg-canvas tablet:flex-row">
          {/* First in the tab order, off-screen until focused: a keyboard user
              reaches the page without tabbing through the whole rail first. */}
          <a
            href="#main"
            className="sr-only focus:not-sr-only focus:fixed focus:left-4 focus:top-4 focus:z-[70] focus:rounded-pill focus:border focus:border-line-strong focus:bg-surface-2 focus:px-4 focus:py-2 focus:text-[14px] focus:font-semibold focus:text-ink"
          >
            Skip to content
          </a>
          <Sidebar />
          {/* `min-h-0` on both, and it is load-bearing rather than tidy. A flex
              item defaults to `min-height: auto`, which refuses to shrink below
              its content — so `flex-1` never constrained the scroller, it grew
              to the height of the page inside it, `overflow-y-auto` never had
              anything to scroll, and the shell's own `overflow-hidden` clipped
              the remainder. On a phone that meant any route taller than the
              viewport could not be scrolled at all and the tab bar sat below
              the fold: on /roles it was at y=4037 of an 844px screen. Every
              sweep before this one measured horizontal overflow, which is
              zero, and never asked whether the page could scroll. */}
          <div className="flex min-h-0 min-w-0 flex-1 flex-col">
            <TopBar session={session} />
            <div id="app-scroll" className="min-h-0 flex-1 overflow-y-auto">
              {/* Notices, not a band under the top bar (6a): every dysfunction
                  is said in the content column, above the page it qualifies,
                  and pushes the page down rather than covering it.

                  Offline is above degraded because it qualifies it: with no
                  network the mode read cannot refresh either, so a degraded
                  Notice underneath is reporting a state nobody can currently
                  confirm. */}
              {/* tabIndex -1 so the skip link's jump moves focus here, not only the scroll. */}
              <main id="main" tabIndex={-1} className="px-4 pb-9 pt-5 tablet:px-[26px] tablet:pt-[30px]">
                <OfflineBanner />
                <DegradedBanner />
                {children}
              </main>
            </div>
          </div>
          <TouchNav />
        </div>
      </PageCrumbProvider>
    </UiViewProvider>
  );
}
