// @vitest-environment jsdom
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
  // scrolls must land below it (WCAG 2.4.11), from the same value the band and glideTo use.
  it("keeps a native jump or a focus below the section band", () => {
    const { container } = render(
      <AppShell session={{ id: "u1", role: "admin" } as SessionUser}>
        <p>page</p>
      </AppShell>,
    );
    expect(container.querySelector("#app-scroll")).toHaveClass("scroll-pt-(--section-band)");
  });
});

