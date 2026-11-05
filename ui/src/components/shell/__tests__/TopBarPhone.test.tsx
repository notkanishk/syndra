// @vitest-environment jsdom
import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { OPEN_GO_TO } from "@/components/shell/GoToHost";
import { TopBar } from "@/components/shell/TopBar";
import type { SessionUser } from "@/lib/session";

const view = vi.hoisted(() => ({ audience: "basic" as string, isOperator: true }));

vi.mock("next/navigation", () => ({ usePathname: () => "/users" }));
vi.mock("@/lib/page-crumb", () => ({ usePageCrumb: () => ({ crumb: null }) }));
vi.mock("@/lib/ui-view", () => ({
  useUiView: () => ({
    audience: view.audience,
    view: view.audience === "advanced" ? "advanced" : "basic",
    isOperator: view.isOperator,
    setView: () => {},
  }),
}));
vi.mock("@/components/shell/AccountSheet", () => ({ AccountSheet: () => <span>account</span> }));
vi.mock("@/components/shell/ThemeToggle", () => ({ ThemeToggle: () => <span>theme</span> }));
vi.mock("@/components/shell/GoToHost", () => ({ OPEN_GO_TO: "syndra:open-go-to" }));

const session = { id: "u1", name: "Sana Example", email: "sana@example.edu" } as SessionUser;

beforeEach(() => {
  view.audience = "basic";
  view.isOperator = true;
});

/** A-20 P6: the phone's bar holds the mark at left and Go to at right, and no fifth tab. */
describe("the phone's top bar", () => {
  const mark = (c: HTMLElement) => c.querySelector(".syndra-mark");
  const goTo = () => screen.queryByRole("button", { name: /^Go to/ });

  it.each(["basic", "advanced"])("shows the mark and Go to to an operator in %s", (audience) => {
    view.audience = audience;
    const { container } = render(<TopBar session={session} />);
    expect(mark(container)).toBeTruthy();
    expect(goTo()).toBeTruthy();
    // Phone only: both leave at the tablet breakpoint, where the rail holds the mark.
    expect(mark(container)!.parentElement!.className).toMatch(/tablet:hidden/);
    expect(goTo()!.className).toMatch(/tablet:hidden/);
  });

  it("shows a member the mark, and no Go to (three pages need no jump)", () => {
    view.audience = "member";
    view.isOperator = false;
    const { container } = render(<TopBar session={session} />);
    expect(mark(container)).toBeTruthy();
    expect(goTo()).toBeNull();
  });

  it("asks the one Go to to open, by event", () => {
    render(<TopBar session={session} />);
    const asked = vi.fn();
    window.addEventListener(OPEN_GO_TO, asked);
    fireEvent.click(goTo()!);
    window.removeEventListener(OPEN_GO_TO, asked);
    expect(asked).toHaveBeenCalledTimes(1);
  });

  it("is a button with the tap floor by token, never a tab", () => {
    render(<TopBar session={session} />);
    expect(goTo()!.className).toMatch(/min-h-\[var\(--tap\)\]/);
    expect(screen.queryByRole("tab")).toBeNull();
  });
});
