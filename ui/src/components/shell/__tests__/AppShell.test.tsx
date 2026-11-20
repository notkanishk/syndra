// @vitest-environment jsdom
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { AppShell } from "@/components/shell/AppShell";
import type { SessionUser } from "@/lib/session";

vi.mock("@/components/shell/Sidebar", () => ({ default: () => <a href="#rail">rail</a> }));
vi.mock("@/components/shell/TopBar", () => ({ TopBar: () => <header>top</header> }));
vi.mock("@/components/shell/PlaceHost", () => ({ PlaceHost: () => null }));
vi.mock("@/components/shell/IntentPrefetchHost", () => ({ IntentPrefetchHost: () => null }));
vi.mock("@/components/shell/TouchNav", () => ({ TouchNav: () => null }));
vi.mock("@/components/shell/GoToHost", () => ({ GoToHost: () => null }));
vi.mock("@/components/states/DegradedBanner", () => ({ DegradedBanner: () => null }));
vi.mock("@/components/states/OfflineBanner", () => ({ OfflineBanner: () => null }));
vi.mock("@/lib/page-crumb", () => ({ PageCrumbProvider: ({ children }: { children: React.ReactNode }) => children }));
vi.mock("@/lib/ui-view", () => ({ UiViewProvider: ({ children }: { children: React.ReactNode }) => children }));
const route = vi.hoisted(() => ({ path: "/" }));
vi.mock("next/navigation", async (original) => ({
  ...(await original<typeof import("next/navigation")>()),
  usePathname: () => route.path,
}));

describe("AppShell skip link", () => {
  it("is the first focusable thing and points at a focusable main", () => {
    render(
      <AppShell session={{ id: "u1", role: "admin" } as SessionUser}>
        <p>page</p>
      </AppShell>,
    );
    const link = screen.getByRole("link", { name: "Skip to content" });
    expect(link).toHaveAttribute("href", "#main");
    // Document order: the link comes before the rail's first link.
    const links = screen.getAllByRole("link");
    expect(links[0]).toBe(link);
    // A target that cannot take focus makes the jump scroll without moving the cursor.
    const main = screen.getByRole("main");
    expect(main).toHaveAttribute("id", "main");
    expect(main).toHaveAttribute("tabindex", "-1");
  });
});

describe("AppShell scrolling", () => {
  // The window must never scroll and push the top bar off. `overflow-hidden` is still a scroll
  // container (focus, find and anchors can move it); `overflow-clip` is not. The one scroller is
  // #app-scroll, and it is the containing block of what is positioned inside it.
  it("clips the shell rather than hiding its overflow, and scrolls only in #app-scroll", () => {
    const { container } = render(
      <AppShell session={{ id: "u1", role: "admin" } as SessionUser}>
        <p>page</p>
      </AppShell>,
    );
    const root = container.querySelector("#app-root")!;
    expect(root).toHaveClass("overflow-clip", "relative");
    expect(root).not.toHaveClass("overflow-hidden");
    const scroller = container.querySelector("#app-scroll")!;
    expect(scroller).toHaveClass("relative", "overflow-y-auto");
  });

  // The sticky section band covers the top of the scroller; a link to an anchor or a focus that
  // scrolls must land below it (WCAG 2.4.11), on pages that have a band and only those.
  it("keeps a jump below the section band, on a page that has one", () => {
    const css = readFileSync(resolve(__dirname, "../../../app/globals.css"), "utf8");
    expect(css).toMatch(/#app-scroll:has\(\[data-section-band\]\)\s*\{\s*scroll-padding-top:\s*var\(--section-band\);/);
    const { container } = render(
      <AppShell session={{ id: "u1", role: "admin" } as SessionUser}>
        <p>page</p>
      </AppShell>,
    );
    // Not on the scroller itself: a page without a band is not shifted.
    expect(container.querySelector("#app-scroll")!.className).not.toMatch(/scroll-p/);
  });
});


describe("AppShell, the light under your hand (A-25 moment 2)", () => {
  // Home and People first: the shell's main carries the scope there, and nowhere else yet.
  it("marks main on Home and People, and on no other page", () => {
    const routes = [["/", true], ["/users", true], ["/users/u1", true], ["/requests", false], ["/roles", false], ["/me", false]] as const;
    for (const [path, on] of routes) {
      route.path = path;
      const { container, unmount } = render(
        <AppShell session={{ id: "u1", role: "admin" } as SessionUser}>
          <p>page</p>
        </AppShell>,
      );
      expect(container.querySelector("main")!.hasAttribute("data-hand"), path).toBe(on);
      unmount();
    }
  });
});
