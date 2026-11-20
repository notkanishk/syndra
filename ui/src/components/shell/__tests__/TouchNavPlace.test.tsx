// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { TouchNav } from "@/components/shell/TouchNav";
import { motionInTests } from "@/test-utils/motion";

// A-25 moment 4 on the phone: the tab bar marks your place as the rail does, one box shaped as the
// tab, gliding across with its light only when a tap on one of the bar's own tabs took you there.

const pathname = vi.hoisted(() => ({ value: "/" }));
vi.mock("next/navigation", () => ({ usePathname: () => pathname.value }));
vi.mock("@/lib/ui-view", () => ({
  useUiView: () => ({ audience: "basic", isOperator: true, view: "basic", setView: () => {}, revealInAdvanced: () => {} }),
}));
vi.mock("next/link", () => ({
  default: ({ href, replace: _replace, children, ...rest }: React.ComponentProps<"a"> & { replace?: boolean }) => (
    <a href={href} {...rest}>
      {children}
    </a>
  ),
}));
vi.mock("@/lib/queries/useTargets", () => ({ useTargets: () => ({ data: [] }) }));
vi.mock("@/lib/queries/useIndicators", () => ({ useIndicatorsRead: () => ({ data: undefined, isPlaceholderData: true }) }));
vi.mock("@/lib/queries/useMySystems", () => ({ useMySystems: () => ({ data: [], isLoading: false }) }));

const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
const bar = () => (
  <QueryClientProvider client={client}>
    <TouchNav />
  </QueryClientProvider>
);
const nav = () => screen.getByRole("navigation", { name: "Main navigation" });
const pill = () => nav().querySelector<HTMLElement>('[data-indicator="box"]');
const light = () => nav().querySelector<HTMLElement>('[data-place-light="bar"]');
const tab = (name: string) => within(nav()).getByRole("link", { name: new RegExp(`^${name}`) });

beforeEach(() => {
  pathname.value = "/";
  motionInTests();
});

describe("your place in the tab bar", () => {
  it("is one box shaped as its tab, never round caps, and the tab gives up its own fill once it is there", () => {
    render(bar());
    expect(pill()).not.toBeNull();
    expect(pill()).not.toHaveClass("rounded-pill");
    expect(tab("Home")).not.toHaveClass("bg-accent-soft");
    // The bar is the room its light lives in, under everything in it.
    expect(nav()).toHaveAttribute("data-place-room");
    expect(nav()).toHaveClass("relative", "isolate");
  });

  it("glides only after a tap on one of the bar's tabs; a key, Back and a first paint land it at once", () => {
    const view = render(bar());
    expect(pill()!.className).not.toMatch(/indicator-glide/);
    fireEvent.pointerDown(tab("People"));
    pathname.value = "/users";
    view.rerender(bar());
    expect(pill()!.className).toMatch(/indicator-glide/);
    expect(light()!.className).toMatch(/place-light-glide/);
    // Back: no tap.
    pathname.value = "/";
    view.rerender(bar());
    expect(pill()!.className).not.toMatch(/indicator-glide/);
    // A key (Enter on a tab) clicks with no pointer press.
    fireEvent.click(tab("Requests"));
    pathname.value = "/requests";
    view.rerender(bar());
    expect(pill()!.className).not.toMatch(/indicator-glide/);
  });
});
