// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import Sidebar from "@/components/shell/Sidebar";
import { UiViewProvider } from "@/lib/ui-view";

/**
 * `/requests` is the one route both MEMBER_NAV and the operator navs point at,
 * because it serves a queue to operators and a submission form to members from
 * the same URL. The rail must not read that shared href as a signal to switch
 * audience — audience comes from the session, never from the route. Unlike
 * Sidebar.test.tsx, `useUiView` is NOT mocked here: this exercises the real
 * session → UiViewProvider → audience wiring the other file bypasses.
 */

const pathname = vi.hoisted(() => ({ value: "/" }));
vi.mock("next/navigation", () => ({ usePathname: () => pathname.value }));
vi.mock("@/lib/queries/useTargets", () => ({ useTargets: () => ({ data: [] }) }));
vi.mock("@/lib/queries/useIndicators", () => ({
  useIndicators: () => ({ data: undefined, isPlaceholderData: true }),
}));

function renderRail(isOperator: boolean) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <UiViewProvider isOperator={isOperator}>
        <Sidebar />
      </UiViewProvider>
    </QueryClientProvider>,
  );
}

describe("the shared /requests route does not bend the rail", () => {
  it("keeps the operator rail for an operator on /requests", () => {
    pathname.value = "/requests";
    renderRail(true);

    const rail = screen.getByRole("navigation", { name: "Main navigation" });
    expect(within(rail).getByRole("link", { name: "Home" })).toBeTruthy();
    expect(within(rail).queryByText("My access")).toBeNull();
    expect(within(rail).queryByText("Network storage")).toBeNull();
  });

  it("keeps the member rail for a member on /requests", () => {
    pathname.value = "/requests";
    renderRail(false);

    const rail = screen.getByRole("navigation", { name: "Main navigation" });
    expect(within(rail).getByRole("link", { name: "My access" })).toBeTruthy();
    expect(within(rail).queryByText("Home")).toBeNull();
  });
});
