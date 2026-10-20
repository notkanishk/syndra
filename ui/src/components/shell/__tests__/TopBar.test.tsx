// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { TopBar } from "@/components/shell/TopBar";
import type { SessionUser } from "@/lib/session";

const view = vi.hoisted(() => ({ audience: "basic" as string }));

vi.mock("next/navigation", () => ({ usePathname: () => "/users" }));
vi.mock("@/lib/page-crumb", () => ({ usePageCrumb: () => ({ crumb: null }) }));
vi.mock("@/lib/ui-view", () => ({
  useUiView: () => ({
    audience: view.audience,
    view: view.audience === "advanced" ? "advanced" : "basic",
    isOperator: true,
    setView: () => {},
  }),
}));
vi.mock("@/components/shell/AccountSheet", () => ({ AccountSheet: () => <span>account</span> }));
vi.mock("@/components/shell/ThemeToggle", () => ({ ThemeToggle: () => <span>theme</span> }));

const session = { id: "u1", name: "Sai Example", email: "sai@example.edu" } as SessionUser;

beforeEach(() => {
  view.audience = "basic";
});

/**
 * The phone header (CONSENSUS #21, Locked; MOBILE.md). Basic has no nav sheet,
 * so the header is the only home the view switch has and it shows at every
 * width. Advanced reaches the same switch at the top of its nav sheet, so below
 * the tablet breakpoint it is not also here: one control in two places is two
 * answers to "where does this live". The boards draw the Advanced phone, which
 * is why they show only the avatar.
 */
describe("the view switch in the header", () => {
  const seat = () => screen.getByRole("radiogroup", { name: "Basic or Advanced view" }).closest("span.flex, span.hidden");

  it("shows at every width in Basic", () => {
    render(<TopBar session={session} />);

    expect(seat()?.className).not.toMatch(/\bhidden\b/);
  });

  it("is left for the nav sheet below the tablet breakpoint in Advanced", () => {
    view.audience = "advanced";
    render(<TopBar session={session} />);

    expect(seat()?.className).toMatch(/\bhidden\b/);
    expect(seat()?.className).toMatch(/tablet:flex/);
  });
});
