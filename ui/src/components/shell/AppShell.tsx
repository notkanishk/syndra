"use client";

import { GoToHost } from "@/components/shell/GoToHost";
import { KeysHost } from "@/components/shell/KeysHost";
import { IntentPrefetchHost } from "@/components/shell/IntentPrefetchHost";
import { PlaceHost } from "@/components/shell/PlaceHost";
import Sidebar from "@/components/shell/Sidebar";
import { TopBar } from "@/components/shell/TopBar";
import { TouchNav } from "@/components/shell/TouchNav";
import { ShellNoticesProvider } from "@/components/shell/ShellNotices";
import { PageCrumbProvider } from "@/lib/page-crumb";
import type { SessionUser } from "@/lib/session";
import type { UiView } from "@/lib/nav";
import { UiViewProvider } from "@/lib/ui-view";

/**
 * The operator shell.
 *
 *   ┌─ 252px rail ─┬───────────── content ─────────────┐
 *   │ own bg       │ top bar --bar, border-bottom      │
 *   │ border-right │  breadcrumb …  [switch] [account] │
 *   │              ├───────────────────────────────────┤
 *   │              │ --page-top, --page-x, 36px        │
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
 * bar retracted, so the bottom of a `h-screen overflow-clip` shell sits
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
        {/* `app-root` is what a dialog makes inert (Modal): overlays portal to <body>, outside it.
            `relative` so `overflow-clip` also clips absolutely positioned descendants. */}
        <div id="app-root" className="relative flex h-dvh flex-col overflow-clip bg-canvas tablet:flex-row">
          {/* First in the tab order, off-screen until focused: a keyboard user
              reaches the page without tabbing through the whole rail first. */}
          <a
            href="#main"
            className="sr-only focus:not-sr-only focus:fixed focus:left-4 focus:top-4 focus:z-(--layer-popover) focus:rounded-pill focus:border focus:border-line-strong focus:bg-surface-2 focus:px-4 focus:py-2 focus:text-[14px] focus:font-semibold focus:text-ink"
          >
            Skip to content
          </a>
          <Sidebar />
          {/* `min-h-0` on both, and it is load-bearing rather than tidy. A flex
              item defaults to `min-height: auto`, which refuses to shrink below
              its content — so `flex-1` never constrained the scroller, it grew
              to the height of the page inside it, `overflow-y-auto` never had
              anything to scroll, and the shell's own `overflow-clip` clipped
              the remainder. On a phone that meant any route taller than the
              viewport could not be scrolled at all and the tab bar sat below
              the fold: on /roles it was at y=4037 of an 844px screen. Every
              sweep before this one measured horizontal overflow, which is
              zero, and never asked whether the page could scroll. */}
          {/* `@container`: the page frame's side inset (`--page-x`, D-1 §3) is read in `cqw` of this
              column, by main, the top bar's row and the section band alike, so all three share one frame. */}
          <div className="@container flex min-h-0 min-w-0 flex-1 flex-col">
            <TopBar session={session} />
            {/* `relative`: the scroller is the containing block of everything absolutely positioned inside it
                (the `sr-only` live regions, popovers), so none of it sits outside the scroller where
                the shell could be scrolled to reach it. `overflow-clip` on the shell above is the other
                half: unlike `overflow-hidden` it is never a scroll container, so no focus, find or
                animation can scroll the window and push the top bar off. The app scrolls here, only.
                A page with a sticky section band gets `scroll-padding-top` for it (globals.css), so a
                link to an anchor or a focus that scrolls lands below it (WCAG 2.4.11). */}
            <div id="app-scroll" className="relative min-h-0 flex-1 overflow-y-auto">
              {/* Notices, not a band under the top bar (6a): every dysfunction
                  is said in the content column and pushes the page down
                  rather than covering it. They sit under the page's own
                  header (A-20 P3, G7), in the seat `ShellNotices` holds. */}
              {/* tabIndex -1 so the skip link's jump moves focus here, not only the scroll. */}
              <main id="main" tabIndex={-1} className="px-4 pb-9 pt-5 tablet:px-(--page-x) tablet:pt-(--page-top)">
                <ShellNoticesProvider>{children}</ShellNoticesProvider>
              </main>
            </div>
          </div>
          <TouchNav />
          <GoToHost />
          <KeysHost />
          <PlaceHost />
          <IntentPrefetchHost />
        </div>
      </PageCrumbProvider>
    </UiViewProvider>
  );
}
