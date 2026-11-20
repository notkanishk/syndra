// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, fireEvent, render, screen, within } from "@testing-library/react";
import { renderToString } from "react-dom/server";
import { beforeEach, describe, expect, it, vi } from "vitest";

import Sidebar from "@/components/shell/Sidebar";
import { GlideIndicator, SETTLE_MS } from "@/lib/motion";
import { motionInTests } from "@/test-utils/motion";
import { block, colour, css, over, ratio, type RGBA } from "./cssTokens";

// A-25 moment 4, your place in the rail (owner, 20 Nov 2026: "do glide with light", at 6%): the pill
// glides with the glide indicator's movement, shaped as the row is, and a faint light travels under
// it; only when the pointer took you there.

const pathname = vi.hoisted(() => ({ value: "/" }));
const view = vi.hoisted(() => ({ audience: "basic" }));
vi.mock("next/navigation", () => ({ usePathname: () => pathname.value }));
vi.mock("@/lib/ui-view", () => ({
  useUiView: () => ({ audience: view.audience, isOperator: true, view: view.audience, setView: () => {}, revealInAdvanced: () => {} }),
}));
vi.mock("@/lib/queries/useTargets", () => ({ useTargets: () => ({ data: [] }) }));
vi.mock("@/lib/queries/useMySystems", () => ({ useMySystems: () => ({ data: [] }) }));
vi.mock("@/lib/queries/useIndicators", async (original) => ({
  ...(await original<typeof import("@/lib/queries/useIndicators")>()),
  useIndicators: () => ({ data: undefined, isPlaceholderData: true }),
}));

const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
const rail = () => (
  <QueryClientProvider client={client}>
    <Sidebar />
  </QueryClientProvider>
);
const nav = () => screen.getByRole("navigation", { name: "Main navigation" });
const pill = () => nav().querySelector<HTMLElement>(".place-pill");
const light = () => nav().querySelector<HTMLElement>("[data-place-light]");
const link = (name: string) => within(nav()).getByRole("link", { name: new RegExp(`^${name}`) });

beforeEach(() => {
  pathname.value = "/";
  view.audience = "basic";
  // A test of the motion itself: the run is still by default (reduced motion lands it at once).
  motionInTests();
});

describe("your place travels only when the pointer took you there", () => {
  it("lands at once on a first paint, a key and Back, and glides after a press on one of the rail's rows", () => {
    const view = render(rail());
    // First paint: there, at once.
    expect(pill()).not.toBeNull();
    expect(pill()!.className).not.toMatch(/indicator-glide/);
    // A press on People, then the path changes: the pill and its light glide.
    fireEvent.click(link("People"), { detail: 1 });
    pathname.value = "/users";
    view.rerender(rail());
    expect(pill()!.className).toMatch(/indicator-glide/);
    expect(light()!.className).toMatch(/place-light-glide/);
    // Back, or a key (Go to, Enter on a row): no press, so it lands at once.
    pathname.value = "/";
    view.rerender(rail());
    expect(pill()!.className).not.toMatch(/indicator-glide/);
    expect(light()!.className).not.toMatch(/place-light-glide/);
    fireEvent.click(link("Projects"));
    pathname.value = "/projects";
    view.rerender(rail());
    expect(pill()!.className).not.toMatch(/indicator-glide/);
  });

  it("does not travel under reduced motion, even after a press", () => {
    motionInTests(false);
    const view = render(rail());
    fireEvent.click(link("People"), { detail: 1 });
    pathname.value = "/users";
    view.rerender(rail());
    expect(pill()!.className).not.toMatch(/indicator-glide/);
    expect(light()!.className).not.toMatch(/place-light-glide/);
  });

  it("is a press on the rail's own rows only: a press elsewhere moves nothing", () => {
    const view = render(
      <>
        {rail()}
        <button type="button">People elsewhere</button>
      </>,
    );
    fireEvent.click(screen.getByText("People elsewhere"), { detail: 1 });
    pathname.value = "/users";
    view.rerender(
      <>
        {rail()}
        <button type="button">People elsewhere</button>
      </>,
    );
    expect(pill()!.className).not.toMatch(/indicator-glide/);
  });
});

describe("what a click is, and how long a glide lasts", () => {
  it("is a plain primary click: a cmd-click or a middle click, then a key's arrival on that path, does not glide", () => {
    const view = render(rail());
    fireEvent.click(link("People"), { detail: 1, metaKey: true });
    fireEvent.click(link("People"), { detail: 1, button: 1 });
    pathname.value = "/users";
    view.rerender(rail());
    expect(pill()!.className).not.toMatch(/indicator-glide/);
  });

  it("glides once: after the settle, a later re-measure lands it at once", () => {
    vi.useFakeTimers();
    try {
      const view = render(rail());
      fireEvent.click(link("People"), { detail: 1 });
      pathname.value = "/users";
      view.rerender(rail());
      expect(pill()!.className).toMatch(/indicator-glide/);
      act(() => {
        vi.advanceTimersByTime(SETTLE_MS);
      });
      expect(pill()!.className).not.toMatch(/indicator-glide/);
      expect(light()!.className).not.toMatch(/place-light-glide/);
    } finally {
      vi.useRealTimers();
    }
  });
});

describe("a place that folds away", () => {
  it("leaves no pill on any row when Advanced turns to Basic under it (its row folds, the next would take its seat)", () => {
    // jsdom runs no animations; give Unfold an engine that does, so the row really folds (it is
    // there, closing) rather than leaving at once.
    const engine = Element.prototype as unknown as { getAnimations?: () => Animation[] };
    const had = "getAnimations" in engine;
    engine.getAnimations = () => [];
    try {
      pathname.value = "/bundles";
      view.audience = "advanced";
      const shell = render(rail());
      expect(pill()).not.toBeNull();
      view.audience = "basic";
      shell.rerender(rail());
      // The row is folding: it has no place, so there is no pill and no light anywhere.
      expect(nav().querySelector('.unfold[data-phase="closing"] a[aria-current="page"]')).not.toBeNull();
      expect(pill()).toBeNull();
      expect(light()).toBeNull();
    } finally {
      if (!had) delete engine.getAnimations;
    }
  });
});

describe("the first HTML", () => {
  it("draws no pill and keeps the row's own fill, until the page is measured on the client", () => {
    const html = renderToString(rail());
    expect(html).not.toMatch(/place-pill|data-place-light/);
    expect(html).toMatch(/aria-current="page"[^>]*class="[^"]*bg-accent-soft/);
  });
});

describe("the pill is one box, shaped as its row", () => {
  it("is drawn with the row's own corners, never round caps, and the row gives up its own fill once it is there", () => {
    render(rail());
    expect(pill()).not.toHaveClass("rounded-pill");
    expect(link("Home")).not.toHaveClass("bg-accent-soft");
    expect(link("Home")).toHaveClass("relative");
    const { container } = render(<GlideIndicator box={{ left: 12, top: 67, width: 227, height: 39, radius: "12px" }} kind="fill" />);
    const box = container.querySelector<HTMLElement>("[data-indicator]")!;
    expect(box.style.borderRadius).toBe("12px");
    expect(box).not.toHaveClass("rounded-pill");
    expect(box.style.transform).toBe("translate(12px, 67px)");
  });
});

describe("its light", () => {
  const ramp = block(":root {\n    --violet-600");
  const themes = { dark: { ...ramp, ...block(':root,\n  [data-theme="dark"] {') }, light: { ...ramp, ...block('[data-theme="light"] {') } };
  const rule = /\.place-light \{([^}]*)\}/.exec(css)?.[1] ?? "";

  it("is the accent at 6% of the room's weight, falling to nothing on the six eased stops", () => {
    expect(rule).toMatch(/--pool-k: calc\(6% \* var\(--hour-weight\)\);/);
    expect(rule).toMatch(/background: radial-gradient\(closest-side, var\(--pool-stops\)\);/);
    expect(rule).toMatch(/z-index: -1;/);
  });

  it("never takes the place's word under 4.5:1: the bold violet word on its pill over the pool's centre, in both rooms", () => {
    const k = Number(/calc\((\d+)% \* var\(--hour-weight\)\)/.exec(rule)![1]) / 100;
    const failures: string[] = [];
    for (const [theme, vars] of Object.entries(themes)) {
      const accent = colour(vars, "--accent");
      const lit = over([accent[0], accent[1], accent[2], (accent[3] ?? 1) * k * Number(vars["--hour-weight"])] as RGBA, colour(vars, "--rail"));
      const pill = over(colour(vars, "--accent-soft"), lit);
      const r = ratio(colour(vars, "--accent-text"), pill);
      // A-25 measured 4.72 in the dark room and 5.51 on paper.
      if (r < 4.5) failures.push(`${theme}: ${r.toFixed(2)}`);
    }
    expect(failures).toEqual([]);
  });

  it("is gone, with the moving pill, under forced colours, where the place takes the system's Highlight", () => {
    expect(css).toMatch(/@media \(forced-colors: active\) \{\s*\.place-light,\s*\.place-pill \{\s*display: none;/);
    expect(css).toMatch(/&\[aria-current="page"\] \{\s*forced-color-adjust: none;\s*background-color: Highlight;/);
  });
});
