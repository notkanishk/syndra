// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { AppShell } from "@/components/shell/AppShell";
import type { SessionUser } from "@/lib/session";

vi.mock("@/components/shell/Sidebar", () => ({ default: () => <a href="#rail">rail</a> }));
vi.mock("@/components/shell/TopBar", () => ({ TopBar: () => <header>top</header> }));
vi.mock("@/components/shell/TouchNav", () => ({ TouchNav: () => null }));
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
