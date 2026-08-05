// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import Sidebar from "@/components/shell/Sidebar";
import type { Indicators } from "@/lib/queries/useIndicators";

const pathname = vi.hoisted(() => ({ value: "/" }));
const view = vi.hoisted(() => ({ audience: "basic" as string, isOperator: true }));
const indicators = vi.hoisted(() => ({ data: undefined as Indicators | undefined }));

vi.mock("next/navigation", () => ({ usePathname: () => pathname.value }));
vi.mock("@/lib/ui-view", () => ({
  useUiView: () => ({ ...view, view: "basic", setView: () => {}, revealInAdvanced: () => {} }),
}));
vi.mock("@/lib/queries/useIndicators", () => ({
  useIndicators: () => indicators,
}));

function renderRail() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <Sidebar />
    </QueryClientProvider>,
  );
}

function railOrder(): string[] {
  return within(screen.getByRole("navigation", { name: "Primary" }))
    .getAllByRole("link")
    .map((link) => link.textContent?.replace(/\d+$/, "").trim() ?? "");
}

beforeEach(() => {
  pathname.value = "/";
  view.audience = "basic";
  view.isOperator = true;
  indicators.data = undefined;
});

describe("the rail's stable structure", () => {
  // The prohibition: the previous sidebar injected a Drift section at the top
  // when its count went non-zero, pushing every other item down under the
  // operator's cursor mid-click.
  it("does not move a single row when counts go from zero to non-zero", () => {
    indicators.data = {
      pending_requests: 0,
      expiring_grants: 0,
      pending_propagation: 0,
      drift: 0,
      zitadel_reachable: true,
    };
    view.audience = "advanced";
    const { unmount } = renderRail();
    const quiet = railOrder();
    unmount();

    indicators.data = {
      pending_requests: 3,
      expiring_grants: 1,
      pending_propagation: 2,
      drift: 12,
      zitadel_reachable: false,
    };
    renderRail();
    expect(railOrder()).toEqual(quiet);
  });

  it("keeps a zero-count row in its seat with a hollow zero", () => {
    indicators.data = {
      pending_requests: 0,
      expiring_grants: 0,
      pending_propagation: 0,
      drift: 0,
      zitadel_reachable: true,
    };
    view.audience = "advanced";
    renderRail();

    const drift = screen.getByRole("link", { name: /Unexplained access/ });
    expect(drift).toBeInTheDocument();
    expect(drift.textContent).toContain("0");
  });

  it("renders Advanced as an append: Basic's rows, in order, then more", () => {
    const basicRail = renderRail();
    const basic = railOrder();
    expect(screen.getByText("Basic")).toBeInTheDocument();
    basicRail.unmount();

    view.audience = "advanced";
    renderRail();
    const advanced = railOrder();
    expect(advanced.slice(0, basic.length)).toEqual(basic);
    expect(advanced).toContain("Unexplained access");
  });

  it("shows a member two destinations and never the operator rows", () => {
    view.audience = "member";
    view.isOperator = false;
    renderRail();

    expect(railOrder()).toEqual(["My access", "Requests"]);
    expect(screen.queryByText("Bundles")).toBeNull();
    expect(screen.getByText("Member")).toBeInTheDocument();
  });
});

describe("the rail's active row", () => {
  it("marks the current page and nothing else", () => {
    pathname.value = "/users/u_2f81";
    renderRail();

    const people = screen.getByRole("link", { name: "People" });
    expect(people).toHaveAttribute("aria-current", "page");
    expect(screen.getByRole("link", { name: "Home" })).not.toHaveAttribute("aria-current");
  });

  it("marks Roles, not Projects, on a role detail route", () => {
    pathname.value = "/projects/pLaser/roles/trained";
    renderRail();

    expect(screen.getByRole("link", { name: "Roles" })).toHaveAttribute("aria-current", "page");
    expect(screen.getByRole("link", { name: "Projects" })).not.toHaveAttribute("aria-current");
  });
});
