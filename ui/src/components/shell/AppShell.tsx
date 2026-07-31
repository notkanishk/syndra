"use client";

import Sidebar from "@/components/shell/Sidebar";
import { TopBar } from "@/components/shell/TopBar";
import { DegradedBanner } from "@/components/states/DegradedBanner";
import { PageCrumbProvider } from "@/lib/page-crumb";
import type { SessionUser } from "@/lib/session";
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
 * would also move the page sideways in a narrow viewport.
 */
export function AppShell({
  session,
  children,
}: {
  session: SessionUser;
  children: React.ReactNode;
}) {
  return (
    <UiViewProvider isOperator={session.role === "admin"}>
      <PageCrumbProvider>
        <div className="flex h-screen overflow-hidden bg-canvas">
          <Sidebar />
          <div className="flex min-w-0 flex-1 flex-col">
            <TopBar session={session} />
            <div id="app-scroll" className="flex-1 overflow-y-auto">
              <DegradedBanner />
              <main className="px-[26px] pb-9 pt-[30px]">{children}</main>
            </div>
          </div>
        </div>
      </PageCrumbProvider>
    </UiViewProvider>
  );
}
