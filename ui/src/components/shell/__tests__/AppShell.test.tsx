// @vitest-environment jsdom
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { act, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { AppShell } from "@/components/shell/AppShell";
import { resetDoorForTests } from "@/lib/motion/door";
import type { SessionUser } from "@/lib/session";
import { setMediaQuery } from "@/test-utils/media";
import { motionInTests } from "@/test-utils/motion";

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


// A-24 §3.2, Q4: the first page of a sign-in opens on the door, and you walk through it, once.
describe("walking through the door", () => {
  const signedIn = (role: "admin" | "user" = "admin", signedInAt = "2026-11-19T08:00:00.000Z") =>
    ({ id: "u1", role, signedInAt }) as SessionUser;
  const shell = (session: SessionUser) =>
    render(
      <AppShell session={session}>
        <p>page</p>
      </AppShell>,
    );
  const door = () => document.querySelector(".door-through");
  // jsdom has no AnimationEvent, so React listens for the prefixed name.
  const animationEnd = (el: Element) => act(() => void fireEvent(el, new Event("webkitAnimationEnd", { bubbles: true })));
  beforeEach(() => {
    window.localStorage.clear();
    resetDoorForTests();
    motionInTests();
  });
  afterEach(() => vi.restoreAllMocks());

  it("opens on the sign-in's first page, whatever page it is, over a page that is there and tints in under it", () => {
    const { container } = shell(signedIn("user"));
    expect(door()).toHaveAttribute("aria-hidden");
    expect(door()!.querySelectorAll(".door-through-room, .door-through-light, .door-through-arch")).toHaveLength(3);
    // The page is under it from the first frame, waiting for the reveal; nothing of it is held back.
    expect(container.querySelector("#app-root")).toHaveAttribute("data-door");
    expect(screen.getByText("page")).toBeInTheDocument();
    // Outside the page, so a dialog's inert never reaches it and its fade never hides it.
    expect(container.querySelector("#app-root .door-through")).toBeNull();
  });

  it("is gone when its light has dissolved, and the page is the page again", () => {
    const { container } = shell(signedIn());
    animationEnd(door()!.querySelector(".door-through-arch")!);
    expect(door()).not.toBeNull();
    animationEnd(door()!.querySelector(".door-through-light")!);
    expect(door()).toBeNull();
    expect(container.querySelector("#app-root")).not.toHaveAttribute("data-door");
  });

  it("is once a sign-in: not on a reload, and again for the next sign-in", () => {
    shell(signedIn()).unmount();
    resetDoorForTests(); // a reload
    shell(signedIn()).unmount();
    expect(door()).toBeNull();
    resetDoorForTests();
    shell(signedIn("admin", "2026-11-20T08:00:00.000Z"));
    expect(door()).not.toBeNull();
  });

  it("asks once a page load, so a second run of the effect can't open it twice", () => {
    shell(signedIn()).unmount();
    window.localStorage.clear(); // even if the device forgot
    shell(signedIn());
    expect(door()).toBeNull();
  });

  it("shows nothing extra when this device can't remember, rather than opening every time", () => {
    vi.spyOn(window, "localStorage", "get").mockImplementation(() => {
      throw new Error("blocked");
    });
    shell(signedIn());
    expect(door()).toBeNull();
  });

  it("is no layer under reduced motion: the page simply appears, and the arrival is spent", () => {
    setMediaQuery("(prefers-reduced-motion: reduce)", true);
    const { container } = shell(signedIn());
    expect(door()).toBeNull();
    expect(container.querySelector("#app-root")).not.toHaveAttribute("data-door");
    setMediaQuery("(prefers-reduced-motion: reduce)", false);
    resetDoorForTests();
    shell(signedIn());
    expect(door()).toBeNull();
  });

  it("has no door without a sign-in instant to keep it to", () => {
    shell({ id: "u1", role: "admin" } as SessionUser);
    expect(door()).toBeNull();
  });
});
